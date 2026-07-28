package rendering

import (
	"time"

	"github.com/energye/gpui/ui/gestures"
	"github.com/energye/gpui/ui/platform"
)

// Scrollable is a pointer→scroll adapter for RenderViewport.
//
// # Nested scroll competition (P5b)
//
// Default rules (Flutter-like minimum set):
//
//  1. Deepest scrollable handles the gesture first (hit path / Parent chain).
//  2. Child not at edge: only the child ScrollOffset changes; parent stays put.
//  3. Child at edge and drag continues in the overscroll direction: residual
//     delta is transferred to Parent (no double-apply of the absorbed part).
//  4. Wheel: deepest consumes first; residual bubbles to Parent when BubbleWheel
//     is true (default).
//
// Arena note: when multiple Scrollables join a GestureArena, only one pan wins.
// Nested handoff does **not** re-open the arena mid-gesture; it uses the Parent
// chain residual after the winner applies ScrollBy. Wire Parent for nesting;
// JoinPointer path should list deepest first for fair arena entry order.
//
// Non-goals: multi-axis simultaneous competition, overscroll glow, platform
// pixel-perfect parity.
//
// P5a: drag uses gestures.PanGestureRecognizer. Wheel bypasses the arena.
type Scrollable struct {
	Viewport *RenderViewport

	// Parent is the outer scrollable for edge handoff (nil = root).
	Parent *Scrollable

	// TransferAtEdge enables drag residual handoff to Parent (default true).
	// Set false to pin scrolling to this viewport only.
	TransferAtEdge bool

	// BubbleWheel enables wheel residual handoff to Parent (default true).
	BubbleWheel bool

	// TouchSlop overrides pan slop when > 0 (logical px).
	TouchSlop float64

	pan *gestures.PanGestureRecognizer
	mgr *gestures.GestureArenaManager

	// LayoutDuringDrag counts layout flushes observed by tests (external).

	// lastPan sample for fling velocity (scroll-space py/s).
	lastPanAt time.Time
	lastPanDY float64 // finger dy of last update
	velY      float64 // estimated scroll velocity (px/s), finger→scroll already negated in apply
}

// NewScrollable wraps a viewport with default nested-handoff enabled.
func NewScrollable(vp *RenderViewport) *Scrollable {
	s := &Scrollable{
		Viewport:       vp,
		mgr:            gestures.NewManager(),
		TransferAtEdge: true,
		BubbleWheel:    true,
	}
	s.pan = gestures.NewPan()
	s.wirePan()
	return s
}

// SetParent links an outer scrollable for residual handoff.
func (s *Scrollable) SetParent(parent *Scrollable) {
	if s == nil {
		return
	}
	s.Parent = parent
}

func (s *Scrollable) wirePan() {
	if s == nil || s.pan == nil {
		return
	}
	if s.TouchSlop > 0 {
		s.pan.TouchSlop = s.TouchSlop
	}
	s.pan.OnPanUpdate = func(e gestures.PointerEvent, dx, dy float64) {
		// Finger dy>0 (down) → content follows → scrollY decreases → ScrollBy(0,-dy).
		now := time.Now()
		if !s.lastPanAt.IsZero() {
			dt := now.Sub(s.lastPanAt).Seconds()
			if dt > 1e-4 && dt < 0.1 {
				// Scroll-space velocity: applyFingerDrag uses -dy.
				s.velY = (-dy) / dt
			}
		}
		s.lastPanAt = now
		s.lastPanDY = dy
		s.applyFingerDrag(dx, dy)
	}
	s.pan.OnPanEnd = func(e gestures.PointerEvent) {
		if s.Viewport != nil && s.Viewport.Physics != nil && s.velY != 0 {
			s.Viewport.Fling(s.velY)
		}
		s.lastPanAt = time.Time{}
		s.velY = 0
	}
}

// applyFingerDrag converts finger delta to scroll and hands residual to Parent.
func (s *Scrollable) applyFingerDrag(dx, dy float64) {
	if s == nil {
		return
	}
	// Intended scroll delta:
	s.applyScrollDelta(-dx, -dy)
}

// applyScrollDelta applies ScrollBy(dx,dy) with clamp; residual goes to Parent
// when TransferAtEdge is set.
func (s *Scrollable) applyScrollDelta(dx, dy float64) {
	if s == nil || s.Viewport == nil {
		return
	}
	before := s.Viewport.ScrollOffset()
	s.Viewport.ScrollBy(dx, dy)
	after := s.Viewport.ScrollOffset()
	appliedX := after.X - before.X
	appliedY := after.Y - before.Y
	resX := dx - appliedX
	resY := dy - appliedY
	if (resX != 0 || resY != 0) && s.TransferAtEdge && s.Parent != nil {
		s.Parent.applyScrollDelta(resX, resY)
	}
}

// applyWheel applies wheel deltas (platform ScrollX/Y) with optional bubble.
func (s *Scrollable) applyWheel(scrollX, scrollY float64) {
	if s == nil || s.Viewport == nil {
		return
	}
	// Same convention as pre-P5b HandlePointer: ScrollBy(-scrollX, -scrollY).
	dx, dy := -scrollX, -scrollY
	before := s.Viewport.ScrollOffset()
	s.Viewport.ScrollBy(dx, dy)
	after := s.Viewport.ScrollOffset()
	resX := dx - (after.X - before.X)
	resY := dy - (after.Y - before.Y)
	if (resX != 0 || resY != 0) && s.BubbleWheel && s.Parent != nil {
		// Parent receives residual as ScrollBy (already in scroll space).
		s.Parent.applyScrollDelta(resX, resY)
	}
}

// AtMinY reports scrollY is at the top (cannot scroll further "up" content-wise:
// decreasing scrollY is blocked).
func (s *Scrollable) AtMinY() bool {
	if s == nil || s.Viewport == nil {
		return true
	}
	return s.Viewport.ScrollOffset().Y <= 1e-6
}

// AtMaxY reports scrollY is at the bottom clamp.
func (s *Scrollable) AtMaxY() bool {
	if s == nil || s.Viewport == nil {
		return true
	}
	max := s.Viewport.MaxScrollY()
	if max < 0 {
		return false
	}
	return s.Viewport.ScrollOffset().Y >= max-1e-6
}

// Pan returns the underlying pan recognizer (for multi-recognizer arenas).
func (s *Scrollable) Pan() *gestures.PanGestureRecognizer {
	if s == nil {
		return nil
	}
	return s.pan
}

// JoinPointer implements gestures.PointerTarget (deepest-first path).
func (s *Scrollable) JoinPointer(m *gestures.GestureArenaManager, pointerID int, e gestures.PointerEvent) {
	if s == nil || s.pan == nil || m == nil {
		return
	}
	if s.TouchSlop > 0 {
		s.pan.TouchSlop = s.TouchSlop
	}
	// Fresh pan per down when joining external arena.
	s.pan = gestures.NewPan()
	s.wirePan()
	a := m.Arena(pointerID)
	a.Add(s.pan)
	s.pan.AddPointer(pointerID, e)
}

// HandlePointer updates scroll from pointer events (logical coords).
// Returns true if the event was consumed.
func (s *Scrollable) HandlePointer(ev platform.Event) bool {
	if s == nil || s.Viewport == nil || ev.Type != platform.EventPointer {
		return false
	}
	e := gestures.FromPlatform(ev)
	switch e.Kind {
	case platform.PointerScroll:
		s.applyWheel(e.ScrollX, e.ScrollY)
		return true
	case platform.PointerDown:
		if s.mgr == nil {
			s.mgr = gestures.NewManager()
		}
		s.pan = gestures.NewPan()
		s.wirePan()
		id := e.EffectivePointerID()
		a := s.mgr.Arena(id)
		a.Add(s.pan)
		s.pan.AddPointer(id, e)
		s.mgr.Route(e)
		s.mgr.CloseArena(id)
		return true
	case platform.PointerMove, platform.PointerUp:
		if s.mgr == nil {
			return false
		}
		s.mgr.Route(e)
		return true
	default:
		return false
	}
}

// ApplyScrollDeltaForTest exposes applyScrollDelta for nested unit tests.
func (s *Scrollable) ApplyScrollDeltaForTest(dx, dy float64) {
	s.applyScrollDelta(dx, dy)
}

// HandlePointerLatest applies only the last move in a batch (coalesce).
func (s *Scrollable) HandlePointerLatest(evs []platform.Event) {
	if s == nil {
		return
	}
	var lastMove *platform.Event
	for i := range evs {
		ev := &evs[i]
		if ev.Type == platform.EventPointer && ev.Pointer == platform.PointerMove {
			lastMove = ev
			continue
		}
		if lastMove != nil {
			s.HandlePointer(*lastMove)
			lastMove = nil
		}
		s.HandlePointer(*ev)
	}
	if lastMove != nil {
		s.HandlePointer(*lastMove)
	}
}
