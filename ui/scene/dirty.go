// Package scene dynamic dirty layer (capability 8.3, P2, S37/W5).
//
// Frozen 2026-09-15 (capability 8.3, P2, S37/W5): DirtyRect, DirtyCode,
// DirtyError, DirtyCodeOf, MaxDirtyRects, DirtyStats, DirtyLayer,
// NewDirtyLayer, NewSpriteDirtyLayer. Additive changes only.
//
// Dynamic world partial update ("move what is dirty"): the static scene
// stays retained, the dynamic layer records only the union of each moved
// sprite's old and new boxes, and the sprite layer updates independently
// of the static layer. More than MaxDirtyRects dirty boxes in one frame
// falls back to a full repaint and reports no per-rect list, mirroring
// render Scene (maxDirtyRects = 16): len > 16 clears rects, DirtyRects is
// nil, NeedsFullRedraw is true. Dirty boxes are clipped to the layer
// bounds; empty and non-finite inputs are ignored quietly, never an error.
// A nil *DirtyLayer never panics: marks are no-ops, getters park at zero.
package scene

import (
	"errors"
	"math"
)

// MaxDirtyRects caps per-frame dirty boxes before a full repaint.
// Mirrors render Scene maxDirtyRects = 16 (len > 16 falls back to full).
const MaxDirtyRects = 16

// DirtyCode classifies a dirty-layer construction failure.
type DirtyCode string

const (
	// DirtyCodeInvalidArg reports an empty, zero-size, or non-finite argument.
	DirtyCodeInvalidArg DirtyCode = "invalid-arg"
)

// DirtyError is the ui/scene dirty-layer error: what kind, which operation.
type DirtyError struct {
	Code DirtyCode
	Op   string
	ID   string
}

// Error renders `op "id": code`.
func (e *DirtyError) Error() string {
	op := e.Op
	if op == "" {
		op = "scene.dirty"
	}
	if e.ID != "" {
		return op + " \"" + e.ID + "\": " + string(e.Code)
	}
	return op + ": " + string(e.Code)
}

// DirtyCodeOf classifies err: the DirtyCode of a *DirtyError, or "" for
// nil and foreign errors.
func DirtyCodeOf(err error) DirtyCode {
	if err == nil {
		return ""
	}
	var e *DirtyError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func dirtyInvalidArg(op, id string) *DirtyError {
	return &DirtyError{Code: DirtyCodeInvalidArg, Op: op, ID: id}
}

// DirtyRect is one repaint box in layer-local coordinates (Y-down).
type DirtyRect struct {
	X, Y, W, H float64
}

// IsEmpty reports whether the rect has no area.
func (r DirtyRect) IsEmpty() bool { return r.W <= 0 || r.H <= 0 }

// Area returns W*H, or 0 for an empty rect.
func (r DirtyRect) Area() float64 {
	if r.IsEmpty() {
		return 0
	}
	return r.W * r.H
}

// Union returns the smallest box holding both. Empty side is ignored.
func (r DirtyRect) Union(o DirtyRect) DirtyRect {
	if r.IsEmpty() {
		return o
	}
	if o.IsEmpty() {
		return r
	}
	x1 := math.Min(r.X, o.X)
	y1 := math.Min(r.Y, o.Y)
	x2 := math.Max(r.X+r.W, o.X+o.W)
	y2 := math.Max(r.Y+r.H, o.Y+o.H)
	return DirtyRect{x1, y1, x2 - x1, y2 - y1}
}

// Intersect returns the overlap box, or false when there is none.
func (r DirtyRect) Intersect(o DirtyRect) (DirtyRect, bool) {
	x1 := math.Max(r.X, o.X)
	y1 := math.Max(r.Y, o.Y)
	x2 := math.Min(r.X+r.W, o.X+o.W)
	y2 := math.Min(r.Y+r.H, o.Y+o.H)
	if x2 <= x1 || y2 <= y1 {
		return DirtyRect{}, false
	}
	return DirtyRect{x1, y1, x2 - x1, y2 - y1}, true
}

func finiteDirty(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func finiteDirtyRect(r DirtyRect) bool {
	return finiteDirty(r.X) && finiteDirty(r.Y) && finiteDirty(r.W) && finiteDirty(r.H)
}

// DirtyStats reports per-layer frame accounting. Frames counts Clear calls,
// TotalRects sums kept dirty boxes over non-full frames, FullFallbacks counts
// frames that fell back to full repaint, MaxRects is the largest kept count.
type DirtyStats struct {
	Frames        int
	TotalRects    int
	FullFallbacks int
	MaxRects      int
}

// DirtyLayer is one dynamic repaint unit: a bounded rect list plus a
// full-repaint fallback. Sprite layers (NewSpriteDirtyLayer) update
// independently of static layers: the caller marks only moved sprites and
// never touches the static layer, so a still background keeps zero dirty
// boxes while the car moves. Implements Layer without touching the retained
// picture / boundary cache path.
type DirtyLayer struct {
	id     uint64
	bounds DirtyRect
	rects  []DirtyRect
	full   bool
	sprite bool
	kids   []Layer
	stats  DirtyStats
}

// NewDirtyLayer builds a dynamic layer over bounds (x,y,w,h).
// Non-finite coordinates or W <= 0 or H <= 0 return invalid-arg.
func NewDirtyLayer(x, y, w, h float64) (*DirtyLayer, error) {
	const op = "scene.NewDirtyLayer"
	if !finiteDirty(x) || !finiteDirty(y) || !finiteDirty(w) || !finiteDirty(h) {
		return nil, dirtyInvalidArg(op, "bounds")
	}
	if w <= 0 || h <= 0 {
		return nil, dirtyInvalidArg(op, "bounds")
	}
	return &DirtyLayer{id: NextLayerID(), bounds: DirtyRect{x, y, w, h}}, nil
}

// NewSpriteDirtyLayer builds the independent sprite update layer.
// Same validation as NewDirtyLayer; Kind reports "dirty_sprite".
func NewSpriteDirtyLayer(x, y, w, h float64) (*DirtyLayer, error) {
	const op = "scene.NewSpriteDirtyLayer"
	l, err := NewDirtyLayer(x, y, w, h)
	if err != nil {
		return nil, err
	}
	l.sprite = true
	_ = op
	return l, nil
}

// LayerID returns the stable dirty identity (0 on nil).
func (l *DirtyLayer) LayerID() uint64 {
	if l == nil {
		return 0
	}
	return l.id
}

// Children returns child layers in paint order.
func (l *DirtyLayer) Children() []Layer {
	if l == nil {
		return nil
	}
	return l.kids
}

// Kind is "dirty" for dynamic layers, "dirty_sprite" for sprite layers.
func (l *DirtyLayer) Kind() string {
	if l != nil && l.sprite {
		return "dirty_sprite"
	}
	return "dirty"
}

// Add appends a child layer.
func (l *DirtyLayer) Add(child Layer) {
	if l == nil || child == nil {
		return
	}
	l.kids = append(l.kids, child)
}

// Bounds returns the layer bounds.
func (l *DirtyLayer) Bounds() DirtyRect {
	if l == nil {
		return DirtyRect{}
	}
	return l.bounds
}

// IsSprite reports whether this is the independent sprite layer.
func (l *DirtyLayer) IsSprite() bool { return l != nil && l.sprite }

// NeedsFull reports whether the frame must repaint everything.
func (l *DirtyLayer) NeedsFull() bool { return l != nil && l.full }

// DirtyCount returns kept dirty boxes (0 when full or nil).
func (l *DirtyLayer) DirtyCount() int {
	if l == nil || l.full {
		return 0
	}
	return len(l.rects)
}

// DirtyRects returns a copy of the kept boxes, or nil when full.
// The caller may mutate the result; the layer keeps no alias.
func (l *DirtyLayer) DirtyRects() []DirtyRect {
	if l == nil || l.full || len(l.rects) == 0 {
		return nil
	}
	out := make([]DirtyRect, len(l.rects))
	copy(out, l.rects)
	return out
}

// DirtyArea sums kept box areas (0 when full).
func (l *DirtyLayer) DirtyArea() float64 {
	if l == nil || l.full {
		return 0
	}
	var total float64
	for _, r := range l.rects {
		total += r.Area()
	}
	return total
}

// Coverage returns DirtyArea over bounds area (0 when full or nil).
// Observed only; it never forces a fallback by itself.
func (l *DirtyLayer) Coverage() float64 {
	if l == nil || l.full {
		return 0
	}
	ba := l.bounds.Area()
	if ba <= 0 {
		return 0
	}
	return l.DirtyArea() / ba
}

// Stats returns a copy of the frame accounting.
func (l *DirtyLayer) Stats() DirtyStats {
	if l == nil {
		return DirtyStats{}
	}
	return l.stats
}

// MarkFull forces a full repaint: rects are dropped, NeedsFull is true.
func (l *DirtyLayer) MarkFull() {
	if l == nil || l.full {
		return
	}
	l.full = true
	l.rects = nil
}

// Clear ends the frame: records accounting, then resets rects and full.
// Every Clear counts one frame, including still frames with zero boxes.
func (l *DirtyLayer) Clear() {
	if l == nil {
		return
	}
	l.stats.Frames++
	if l.full {
		l.stats.FullFallbacks++
	} else {
		l.stats.TotalRects += len(l.rects)
		if len(l.rects) > l.stats.MaxRects {
			l.stats.MaxRects = len(l.rects)
		}
	}
	l.rects = nil
	l.full = false
}

// MarkDirty clips r to the layer bounds and keeps it.
// Empty, non-finite, or fully outside boxes are ignored quietly.
// True means one box was kept. A nil layer or a full layer keeps nothing.
func (l *DirtyLayer) MarkDirty(r DirtyRect) bool {
	if l == nil || l.full {
		return false
	}
	if !finiteDirtyRect(r) || r.IsEmpty() {
		return false
	}
	clipped, ok := r.Intersect(l.bounds)
	if !ok {
		return false
	}
	l.rects = append(l.rects, clipped)
	if len(l.rects) > MaxDirtyRects {
		l.full = true
		l.rects = nil
		return false
	}
	return true
}

// MarkMoved dirties the union of a sprite's old and new boxes so both the
// trail and the arrival repaint (chase-car semantics). Identical boxes mean
// no motion and keep nothing. Either side empty means spawn/despawn: the
// non-empty side alone is dirtied. Fully outside or non-finite input keeps
// nothing. True means one box was kept.
func (l *DirtyLayer) MarkMoved(from, to DirtyRect) bool {
	if l == nil || l.full {
		return false
	}
	if !finiteDirtyRect(from) || !finiteDirtyRect(to) {
		return false
	}
	if from.IsEmpty() && to.IsEmpty() {
		return false
	}
	if from.IsEmpty() {
		return l.MarkDirty(to)
	}
	if to.IsEmpty() {
		return l.MarkDirty(from)
	}
	if from == to {
		return false
	}
	return l.MarkDirty(from.Union(to))
}
