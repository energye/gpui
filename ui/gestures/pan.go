package gestures

import "github.com/energye/gpui/ui/input"

// PanGestureRecognizer wins when movement exceeds touch slop, then tracks drag.
//
// OnPanUpdate receives the latest sample and delta since the previous sample
// (after accept). Coalesce moves externally (Dispatcher) for "latest point only".
type PanGestureRecognizer struct {
	baseRecognizer

	OnPanStart  func(e PointerEvent)
	OnPanUpdate func(e PointerEvent, dx, dy float64)
	OnPanEnd    func(e PointerEvent)

	// TouchSlop overrides kTouchSlop when > 0.
	TouchSlop float64

	downX, downY float64
	lastX, lastY float64
	tracking     bool
	started      bool // OnPanStart fired
}

// NewPan creates a pan recognizer.
func NewPan() *PanGestureRecognizer {
	return &PanGestureRecognizer{}
}

func (p *PanGestureRecognizer) slop() float64 {
	if p != nil && p.TouchSlop > 0 {
		return p.TouchSlop
	}
	return kTouchSlop
}

// AddPointer implements GestureRecognizer.
func (p *PanGestureRecognizer) AddPointer(pointerID int, e PointerEvent) {
	if p == nil || p.disposed {
		return
	}
	if e.Kind != input.PointerDown {
		return
	}
	p.tracking = true
	p.started = false
	p.accepted = false
	p.rejected = false
	p.downX, p.downY = e.X, e.Y
	p.lastX, p.lastY = e.X, e.Y
	_ = pointerID
}

// HandleEvent implements GestureRecognizer.
func (p *PanGestureRecognizer) HandleEvent(e PointerEvent) {
	if p == nil || p.isDead() || !p.tracking {
		return
	}
	switch e.Kind {
	case input.PointerMove:
		if !p.accepted {
			if dist(p.downX, p.downY, e.X, e.Y) > p.slop() {
				if p.a != nil {
					p.a.Accept(p)
				} else {
					p.Accept()
				}
			} else {
				return
			}
		}
		if p.accepted {
			if !p.started {
				p.started = true
				if p.OnPanStart != nil {
					p.OnPanStart(e)
				}
			}
			dx := e.X - p.lastX
			dy := e.Y - p.lastY
			p.lastX, p.lastY = e.X, e.Y
			if p.OnPanUpdate != nil {
				p.OnPanUpdate(e, dx, dy)
			}
		}
	case input.PointerUp:
		if p.accepted {
			if p.OnPanEnd != nil {
				p.OnPanEnd(e)
			}
		} else {
			// Never exceeded slop — lose to tap.
			if p.a != nil {
				p.a.Reject(p)
			} else {
				p.Reject()
			}
		}
		p.tracking = false
	}
}

// Accept implements GestureRecognizer.
func (p *PanGestureRecognizer) Accept() {
	if p == nil || p.rejected {
		return
	}
	p.accepted = true
}

// Reject implements GestureRecognizer.
func (p *PanGestureRecognizer) Reject() {
	if p == nil {
		return
	}
	p.rejected = true
	p.accepted = false
	p.tracking = false
	p.started = false
}

// Dispose implements GestureRecognizer.
func (p *PanGestureRecognizer) Dispose() {
	if p == nil {
		return
	}
	p.disposed = true
	p.tracking = false
	p.OnPanStart = nil
	p.OnPanUpdate = nil
	p.OnPanEnd = nil
	p.a = nil
}

var _ GestureRecognizer = (*PanGestureRecognizer)(nil)
var _ arenaClient = (*PanGestureRecognizer)(nil)
