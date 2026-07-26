package rendering

import (
	"sync"

	"github.com/energye/gpui/ui/painting"
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
}

// NewPipelineOwner creates an owner with optional root.
func NewPipelineOwner(root RenderObject) *PipelineOwner {
	o := &PipelineOwner{root: root}
	if root != nil {
		attachOwner(root, o)
	}
	return o
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
func (o *PipelineOwner) FlushPaint(pc *painting.Context, force bool) bool {
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
