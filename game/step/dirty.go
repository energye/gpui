// Dynamic dirty tracker (capability 8.3, P2, S37/W5).
//
// Frozen 2026-09-15 (capability 8.3, P2, S37/W5): MaxDirtyRects,
// DirtyStats, DirtyTracker, NewDirtyTracker, NewSpriteDirtyTracker,
// DirtyForMove. Additive changes only.
//
// Game-side twin of ui/scene DirtyLayer: pure core numbers, no render
// import. The loop marks each moved sprite's old-plus-new union, leaves
// still sprites unmarked so the sprite layer updates independently, and
// falls back to a full repaint when more than MaxDirtyRects boxes gather
// in one frame (same len > 16 rule as render Scene and ui/scene). Boxes
// are clipped to the tracker bounds; empty and non-finite inputs are
// ignored quietly. A nil *DirtyTracker never panics.
package step

import (
	"math"

	"github.com/energye/gpui/game/core"
)

// MaxDirtyRects caps per-frame dirty boxes before a full repaint.
// Must equal ui/scene MaxDirtyRects (len > 16 falls back to full).
const MaxDirtyRects = 16

// DirtyStats reports per-tracker frame accounting. Frames counts Clear
// calls, TotalRects sums kept boxes over non-full frames, FullFallbacks
// counts frames that fell back to full repaint, MaxRects is the largest
// kept count.
type DirtyStats struct {
	Frames        int
	TotalRects    int
	FullFallbacks int
	MaxRects      int
}

// DirtyTracker is one dynamic repaint unit over core.Rect bounds.
// Sprite trackers (NewSpriteDirtyTracker) update independently of the
// static world: only moved sprites are marked, still ground keeps zero
// boxes. Not safe for concurrent use; a nil tracker never panics.
type DirtyTracker struct {
	bounds core.Rect
	rects  []core.Rect
	full   bool
	sprite bool
	stats  DirtyStats
}

func finiteDirtyFloat(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func finiteDirtyRect(r core.Rect) bool {
	return finiteDirtyFloat(r.X) && finiteDirtyFloat(r.Y) &&
		finiteDirtyFloat(r.W) && finiteDirtyFloat(r.H)
}

// NewDirtyTracker builds a dynamic tracker over bounds.
// Non-finite coordinates or W <= 0 or H <= 0 return invalid-arg.
func NewDirtyTracker(bounds core.Rect) (*DirtyTracker, error) {
	const op = "step.NewDirtyTracker"
	if !finiteDirtyRect(bounds) {
		return nil, core.InvalidArg(op, "bounds")
	}
	if bounds.W <= 0 || bounds.H <= 0 {
		return nil, core.InvalidArg(op, "bounds")
	}
	return &DirtyTracker{bounds: bounds}, nil
}

// NewSpriteDirtyTracker builds the independent sprite update tracker.
// Same validation as NewDirtyTracker.
func NewSpriteDirtyTracker(bounds core.Rect) (*DirtyTracker, error) {
	const op = "step.NewSpriteDirtyTracker"
	t, err := NewDirtyTracker(bounds)
	if err != nil {
		return nil, err
	}
	t.sprite = true
	_ = op
	return t, nil
}

// Bounds returns the tracker bounds.
func (t *DirtyTracker) Bounds() core.Rect {
	if t == nil {
		return core.Rect{}
	}
	return t.bounds
}

// IsSprite reports whether this is the independent sprite tracker.
func (t *DirtyTracker) IsSprite() bool { return t != nil && t.sprite }

// NeedsFull reports whether the frame must repaint everything.
func (t *DirtyTracker) NeedsFull() bool { return t != nil && t.full }

// DirtyCount returns kept dirty boxes (0 when full or nil).
func (t *DirtyTracker) DirtyCount() int {
	if t == nil || t.full {
		return 0
	}
	return len(t.rects)
}

// DirtyRects returns a copy of the kept boxes, or nil when full or empty.
// The caller may mutate the result; the tracker keeps no alias.
func (t *DirtyTracker) DirtyRects() []core.Rect {
	if t == nil || t.full || len(t.rects) == 0 {
		return nil
	}
	out := make([]core.Rect, len(t.rects))
	copy(out, t.rects)
	return out
}

// DirtyArea sums kept box areas (0 when full).
func (t *DirtyTracker) DirtyArea() float64 {
	if t == nil || t.full {
		return 0
	}
	var total float64
	for _, r := range t.rects {
		total += r.Area()
	}
	return total
}

// Coverage returns DirtyArea over bounds area (0 when full or nil).
// Observed only; it never forces a fallback by itself.
func (t *DirtyTracker) Coverage() float64 {
	if t == nil || t.full {
		return 0
	}
	ba := t.bounds.Area()
	if ba <= 0 {
		return 0
	}
	return t.DirtyArea() / ba
}

// Stats returns a copy of the frame accounting.
func (t *DirtyTracker) Stats() DirtyStats {
	if t == nil {
		return DirtyStats{}
	}
	return t.stats
}

// MarkFull forces a full repaint: rects are dropped, NeedsFull is true.
func (t *DirtyTracker) MarkFull() {
	if t == nil || t.full {
		return
	}
	t.full = true
	t.rects = nil
}

// Clear ends the frame: records accounting, then resets rects and full.
// Every Clear counts one frame, including still frames with zero boxes.
func (t *DirtyTracker) Clear() {
	if t == nil {
		return
	}
	t.stats.Frames++
	if t.full {
		t.stats.FullFallbacks++
	} else {
		t.stats.TotalRects += len(t.rects)
		if len(t.rects) > t.stats.MaxRects {
			t.stats.MaxRects = len(t.rects)
		}
	}
	t.rects = nil
	t.full = false
}

// MarkDirty clips r to the tracker bounds and keeps it.
// Empty, non-finite, or fully outside boxes are ignored quietly.
// True means one box was kept. A nil tracker or a full tracker keeps nothing.
func (t *DirtyTracker) MarkDirty(r core.Rect) bool {
	if t == nil || t.full {
		return false
	}
	if !finiteDirtyRect(r) || r.IsEmpty() {
		return false
	}
	clipped, ok := r.Intersection(t.bounds)
	if !ok {
		return false
	}
	t.rects = append(t.rects, clipped)
	if len(t.rects) > MaxDirtyRects {
		t.full = true
		t.rects = nil
		return false
	}
	return true
}

// DirtyForMove is the pure chase-car union: the box covering both the old
// and the new sprite boxes so the trail and the arrival both repaint.
// Either side empty means spawn/despawn: the non-empty side alone is the
// answer. Identical boxes mean no motion (false). Non-finite or fully
// empty input is false. Bounds clipping is left to MarkDirty/MarkMoved.
func DirtyForMove(from, to core.Rect) (core.Rect, bool) {
	if !finiteDirtyRect(from) || !finiteDirtyRect(to) {
		return core.Rect{}, false
	}
	if from.IsEmpty() && to.IsEmpty() {
		return core.Rect{}, false
	}
	if from.IsEmpty() {
		return to, true
	}
	if to.IsEmpty() {
		return from, true
	}
	if from == to {
		return core.Rect{}, false
	}
	return from.Union(to), true
}

// MarkMoved dirties the union of a sprite's old and new boxes (see
// DirtyForMove), clipped to the tracker bounds. True means one box kept.
func (t *DirtyTracker) MarkMoved(from, to core.Rect) bool {
	if t == nil || t.full {
		return false
	}
	u, ok := DirtyForMove(from, to)
	if !ok {
		return false
	}
	return t.MarkDirty(u)
}
