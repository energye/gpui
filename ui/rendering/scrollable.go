package rendering

import "github.com/energye/gpui/ui/platform"

// Scrollable is a thin pointer→scroll adapter for RenderViewport (S6 MVP).
// Not a full GestureArena (P5). Uses latest pointer sample per HandlePointer call.
type Scrollable struct {
	Viewport *RenderViewport

	dragging bool
	lastY    float64
	// LayoutDuringDrag counts layout flushes observed by tests (external).
}

// NewScrollable wraps a viewport.
func NewScrollable(vp *RenderViewport) *Scrollable {
	return &Scrollable{Viewport: vp}
}

// HandlePointer updates scroll from pointer events (logical coords).
// Returns true if the event was consumed.
func (s *Scrollable) HandlePointer(ev platform.Event) bool {
	if s == nil || s.Viewport == nil || ev.Type != platform.EventPointer {
		return false
	}
	switch ev.Pointer {
	case platform.PointerDown:
		s.dragging = true
		s.lastY = ev.Y
		return true
	case platform.PointerUp:
		s.dragging = false
		return true
	case platform.PointerMove:
		if !s.dragging {
			return false
		}
		// Finger moves down → content follows → scrollY decreases?
		// Convention: drag content up (finger moves up, dy negative in Y-down) increases scrollY.
		// delta screen: dy = ev.Y - lastY; content should move by dy → scrollY -= dy
		dy := ev.Y - s.lastY
		s.lastY = ev.Y
		s.Viewport.ScrollBy(0, -dy)
		return true
	case platform.PointerScroll:
		// ScrollY: positive typically means wheel up / content down depending on platform.
		// Apply as ScrollBy(0, -ScrollY) so wheel "down" increases scrollY.
		s.Viewport.ScrollBy(-ev.ScrollX, -ev.ScrollY)
		return true
	default:
		return false
	}
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
		// Flush pending move before non-move
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
