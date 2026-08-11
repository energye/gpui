package gestures

import "github.com/energye/gpui/ui/input"

// PointerTarget can join an arena on pointer down (deepest-first path order).
type PointerTarget interface {
	// JoinPointer is called for each hit-test path entry from leaf to root.
	JoinPointer(m *GestureArenaManager, pointerID int, e PointerEvent)
}

// Dispatcher routes platform pointer events through a GestureArenaManager.
//
// Down: call JoinPointer on each path target (deepest first), then CloseArena.
// Move/Up: Route to arena (optionally coalesce moves).
// Scroll: never enters the arena; use OnScroll.
type Dispatcher struct {
	Manager *GestureArenaManager
	// OnScroll is invoked for PointerScroll (bypasses arena).
	OnScroll func(ScrollDelta)
}

// NewDispatcher creates a dispatcher with a fresh manager.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{Manager: NewManager()}
}

// HandleDown starts an arena and lets path targets join (deepest-first).
func (d *Dispatcher) HandleDown(e PointerEvent, path []PointerTarget) {
	if d == nil || d.Manager == nil {
		return
	}
	id := e.ID
	_ = d.Manager.Arena(id) // ensure open
	for _, t := range path {
		if t != nil {
			t.JoinPointer(d.Manager, id, e)
		}
	}
	// Also deliver down to recognizers that joined.
	d.Manager.Route(e)
	d.Manager.CloseArena(id)
}

// HandleEvent routes a non-down pointer event (move/up). Prefer HandleBatch for moves.
func (d *Dispatcher) HandleEvent(e PointerEvent) {
	if d == nil || d.Manager == nil {
		return
	}
	if e.Kind == input.PointerScroll {
		ApplyScrollWheel(e, d.OnScroll)
		return
	}
	if e.Kind == input.PointerDown {
		// Without path, still open empty arena — callers should use HandleDown.
		id := e.ID
		_ = d.Manager.Arena(id)
		d.Manager.Route(e)
		d.Manager.CloseArena(id)
		return
	}
	d.Manager.Route(e)
}

// HandleBatch processes a list of events, coalescing consecutive PointerMove
// samples to the latest point (P4 S6 / P5 A.1).
func (d *Dispatcher) HandleBatch(evs []PointerEvent, downPath func(e PointerEvent) []PointerTarget) {
	if d == nil {
		return
	}
	var lastMove *PointerEvent
	flushMove := func() {
		if lastMove != nil {
			d.HandleEvent(*lastMove)
			lastMove = nil
		}
	}
	for i := range evs {
		e := evs[i]
		switch e.Kind {
		case input.PointerMove:
			// keep only latest
			cp := e
			lastMove = &cp
		case input.PointerScroll:
			flushMove()
			ApplyScrollWheel(e, d.OnScroll)
		case input.PointerDown:
			flushMove()
			var path []PointerTarget
			if downPath != nil {
				path = downPath(e)
			}
			d.HandleDown(e, path)
		default:
			flushMove()
			d.HandleEvent(e)
		}
	}
	flushMove()
}

// RecognizerTarget wraps a GestureRecognizer as a PointerTarget that joins the arena.
type RecognizerTarget struct {
	Rec GestureRecognizer
}

// JoinPointer implements PointerTarget.
func (t *RecognizerTarget) JoinPointer(m *GestureArenaManager, pointerID int, e PointerEvent) {
	if t == nil || t.Rec == nil || m == nil {
		return
	}
	a := m.Arena(pointerID)
	a.Add(t.Rec)
	t.Rec.AddPointer(pointerID, e)
}
