package rendering

import (
	"sync"
	"time"
)

// PipelineOwner flushes layout and paint for a root RenderObject (Flutter subset).
type PipelineOwner struct {
	mu sync.Mutex

	root RenderObject

	// Counters for tests / metrics (reset with ResetCounts).
	LayoutCount int64
	PaintCount  int64

	layoutDirty bool
	paintDirty  bool

	// boundaryCache is the process-lifetime Picture cache for RepaintBoundary
	// nodes on this tree (W1 R3). Shared across presents so skip is observable.
	boundaryCache *BoundaryCache

	// blinkers are widgets with framework-owned blink state (caret blink)
	// that need dt while focused. Widgets self-report on focus transitions;
	// the embedder pumps them via TickBlink on the UI thread. blinkCtl (when
	// set by the embedder) registers/unregisters the pump ticker as the set
	// transitions between empty and non-empty, so a tree with no blinkers
	// costs nothing.
	blinkers map[Blinkable]struct{}
	blinkCtl func(active bool)
}

// Blinkable is a widget whose blink phase advances on UI-thread dt.
// BlinkTick reports whether the visible state toggled (the caller dirties
// via the widget's own MarkNeedsPaint; frames come from dirtiness, never
// from ticker aliveness).
type Blinkable interface {
	BlinkTick(dt float64) bool
}

// BlinkDeadliner is an optional deadline expression for Blinkables: how
// long until the next visible toggle (the event loop's sleep deadline).
// ok=false means no deadline (unfocused or steady state).
type BlinkDeadliner interface {
	NextBlinkIn() (time.Duration, bool)
}

// SetBlinkTickerCtl installs the embedder callback that (de)registers the
// blink pump ticker. Nil clears it (unit-built owners).
func (o *PipelineOwner) SetBlinkTickerCtl(fn func(active bool)) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.blinkCtl = fn
	o.mu.Unlock()
}

// NoteBlinkable registers b for blink dt (idempotent). Notifies the
// embedder when the set becomes non-empty.
func (o *PipelineOwner) NoteBlinkable(b Blinkable) {
	if o == nil || b == nil {
		return
	}
	var ctl func(active bool)
	var notify bool
	o.mu.Lock()
	if o.blinkers == nil {
		o.blinkers = make(map[Blinkable]struct{})
	}
	if _, ok := o.blinkers[b]; !ok {
		notify = len(o.blinkers) == 0
		o.blinkers[b] = struct{}{}
		ctl = o.blinkCtl
	}
	o.mu.Unlock()
	if notify && ctl != nil {
		ctl(true)
	}
}

// DropBlinkable removes b from blink dt (idempotent). Notifies the
// embedder when the set becomes empty.
func (o *PipelineOwner) DropBlinkable(b Blinkable) {
	if o == nil || b == nil {
		return
	}
	var ctl func(active bool)
	var notify bool
	o.mu.Lock()
	if _, ok := o.blinkers[b]; ok {
		delete(o.blinkers, b)
		notify = len(o.blinkers) == 0
		ctl = o.blinkCtl
	}
	o.mu.Unlock()
	if notify && ctl != nil {
		ctl(false)
	}
}

// BlinkActive reports whether any blinkable is registered.
func (o *PipelineOwner) BlinkActive() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.blinkers) > 0
}

// NextBlinkWake reports the minimum time until any registered blinkable's
// next visible toggle. False when none is registered or none reports one;
// the loop then waits on events instead of holding a pacing cadence.
func (o *PipelineOwner) NextBlinkWake() (time.Duration, bool) {
	if o == nil {
		return 0, false
	}
	o.mu.Lock()
	list := make([]Blinkable, 0, len(o.blinkers))
	for b := range o.blinkers {
		if b != nil {
			list = append(list, b)
		}
	}
	o.mu.Unlock()
	var min time.Duration
	found := false
	for _, b := range list {
		dl, ok := b.(BlinkDeadliner)
		if !ok {
			continue
		}
		d, ok := dl.NextBlinkIn()
		if !ok {
			continue
		}
		if !found || d < min {
			min, found = d, true
		}
	}
	return min, found
}

// TickBlink advances every registered blinkable on the UI thread and
// reports whether any toggled. Callbacks run without the owner lock held
// (widgets dirty themselves, which re-enters the owner).
func (o *PipelineOwner) TickBlink(dt float64) bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	if len(o.blinkers) == 0 {
		o.mu.Unlock()
		return false
	}
	list := make([]Blinkable, 0, len(o.blinkers))
	for b := range o.blinkers {
		if b != nil {
			list = append(list, b)
		}
	}
	o.mu.Unlock()
	toggled := false
	for _, b := range list {
		if b.BlinkTick(dt) {
			toggled = true
		}
	}
	return toggled
}

// NewPipelineOwner creates an owner with optional root.
func NewPipelineOwner(root RenderObject) *PipelineOwner {
	o := &PipelineOwner{
		root:          root,
		boundaryCache: NewBoundaryCache(),
	}
	if root != nil {
		attachOwner(root, o)
	}
	return o
}

// BoundaryCache returns the tree's Picture-backed RepaintBoundary cache (never nil
// for a non-nil owner). Survives across frames so clean boundaries can skip.
func (o *PipelineOwner) BoundaryCache() *BoundaryCache {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.boundaryCache == nil {
		o.boundaryCache = NewBoundaryCache()
	}
	return o.boundaryCache
}

// SetRoot replaces the root and attaches owner.
func (o *PipelineOwner) SetRoot(root RenderObject) {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.root = root
	o.layoutDirty = true
	o.paintDirty = true
	o.mu.Unlock()
	if root != nil {
		attachOwner(root, o)
	}
}

// Root returns the current root.
func (o *PipelineOwner) Root() RenderObject {
	if o == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.root
}

func (o *PipelineOwner) noteLayoutDirty() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.layoutDirty = true
	o.mu.Unlock()
}

func (o *PipelineOwner) notePaintDirty() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.paintDirty = true
	o.mu.Unlock()
}

// ResetCounts zeroes layout/paint counters.
func (o *PipelineOwner) ResetCounts() {
	if o == nil {
		return
	}
	o.mu.Lock()
	o.LayoutCount = 0
	o.PaintCount = 0
	o.mu.Unlock()
}

// FlushLayout lays out the root under tight viewport constraints when dirty
// or when force is true. Returns whether layout work ran.
// A second flush with no dirty flags is a no-op (layout_count unchanged).
func (o *PipelineOwner) FlushLayout(viewport Size, force bool) bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	root := o.root
	dirty := o.layoutDirty || force
	o.mu.Unlock()
	if root == nil {
		return false
	}
	if !force && !dirty && !root.NeedsLayout() {
		return false
	}
	_ = root.Layout(Tight(viewport.Width, viewport.Height))
	o.mu.Lock()
	o.LayoutCount++
	o.layoutDirty = false
	o.mu.Unlock()
	return true
}

// FlushPaint paints the root when paint-dirty or force. pc must be non-nil.
// When force is false and the tree is only partially dirty, CompositeOnly is set
// so clean subtrees are skipped (P2 retained paint).
// Returns whether paint ran.
func (o *PipelineOwner) FlushPaint(pc *PaintContext, force bool) bool {
	if o == nil || pc == nil {
		return false
	}
	o.mu.Lock()
	root := o.root
	dirty := o.paintDirty || force
	o.mu.Unlock()
	if root == nil {
		return false
	}
	if !dirty && !root.NeedsPaint() && !SubtreeNeedsPaint(root) && !force {
		return false
	}
	if !force {
		pc.CompositeOnly = true
	} else {
		pc.CompositeOnly = false
	}
	root.Paint(pc)
	o.mu.Lock()
	o.PaintCount++
	o.paintDirty = false
	o.mu.Unlock()
	return true
}

// UpdateCompositingBits runs F04 propagation from the root.
func (o *PipelineOwner) UpdateCompositingBits() {
	if o == nil || o.root == nil {
		return
	}
	if b, ok := baseOf(o.root); ok {
		b.UpdateCompositingBits()
	}
}

// ConsumeNeedsPaint clears all paint-dirty flags without drawing. Used by the
// retained textured-composite path (W2 R4): content is captured into the layer
// tree (BuildLayerTree records leaf display lists) and rasterized to cached
// textures, so no live FlushPaint runs and the needs-paint marks must not keep
// scheduling frames forever.
func (o *PipelineOwner) ConsumeNeedsPaint() {
	if o == nil || o.root == nil {
		return
	}
	o.mu.Lock()
	o.paintDirty = false
	o.mu.Unlock()
	var walk func(n RenderObject)
	walk = func(n RenderObject) {
		if n == nil {
			return
		}
		if b, ok := baseOf(n); ok {
			b.clearPaintDirty()
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(o.root)
}

// NeedsFrame reports layout or paint dirty.
func (o *PipelineOwner) NeedsFrame() bool {
	if o == nil {
		return false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.layoutDirty || o.paintDirty {
		return true
	}
	if o.root != nil && (o.root.NeedsLayout() || o.root.NeedsPaint()) {
		return true
	}
	return false
}

func attachOwner(n RenderObject, o *PipelineOwner) {
	if n == nil {
		return
	}
	if b, ok := baseOf(n); ok {
		b.SetOwner(o)
	}
	for _, c := range n.Children() {
		attachOwner(c, o)
	}
}
