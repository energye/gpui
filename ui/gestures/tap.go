package gestures

import (
	"math"

	"github.com/energye/gpui/ui/platform"
)

// TapGestureRecognizer wins if pointer up occurs within kTouchSlop of down.
// Exceeding slop (or losing the arena) rejects without firing OnTap.
type TapGestureRecognizer struct {
	baseRecognizer

	// OnTap is invoked once when the tap wins and pointer goes up within slop.
	OnTap func(e PointerEvent)

	// TouchSlop overrides kTouchSlop when > 0.
	TouchSlop float64

	downX, downY float64
	tracking     bool
	// wonAccept means arena accepted us (may still wait for Up to fire OnTap).
	wonAccept bool
	fired     bool
}

// NewTap creates a tap recognizer.
func NewTap() *TapGestureRecognizer {
	return &TapGestureRecognizer{}
}

func (t *TapGestureRecognizer) slop() float64 {
	if t != nil && t.TouchSlop > 0 {
		return t.TouchSlop
	}
	return kTouchSlop
}

// AddPointer implements GestureRecognizer.
func (t *TapGestureRecognizer) AddPointer(pointerID int, e PointerEvent) {
	if t == nil || t.disposed {
		return
	}
	if e.Kind != platform.PointerDown {
		return
	}
	t.tracking = true
	t.downX, t.downY = e.X, e.Y
	t.wonAccept = false
	t.fired = false
	t.accepted = false
	t.rejected = false
	if t.a == nil && pointerID != 0 {
		// Arena should Add us externally; if already set, fine.
	}
	_ = pointerID
}

// HandleEvent implements GestureRecognizer.
func (t *TapGestureRecognizer) HandleEvent(e PointerEvent) {
	if t == nil || t.isDead() || !t.tracking {
		return
	}
	switch e.Kind {
	case platform.PointerMove:
		if dist(t.downX, t.downY, e.X, e.Y) > t.slop() {
			t.rejectSelf()
		}
	case platform.PointerUp:
		if dist(t.downX, t.downY, e.X, e.Y) > t.slop() {
			t.rejectSelf()
			return
		}
		// Within slop: claim victory then fire.
		if !t.accepted && !t.rejected {
			if t.a != nil {
				t.a.Accept(t)
			} else {
				t.Accept()
			}
		}
		if t.accepted && !t.fired {
			t.fired = true
			if t.OnTap != nil {
				t.OnTap(e)
			}
		}
		t.tracking = false
	}
}

// Accept implements GestureRecognizer.
func (t *TapGestureRecognizer) Accept() {
	if t == nil || t.rejected {
		return
	}
	t.accepted = true
	t.wonAccept = true
}

// Reject implements GestureRecognizer.
func (t *TapGestureRecognizer) Reject() {
	if t == nil {
		return
	}
	t.rejected = true
	t.accepted = false
	t.tracking = false
}

// Dispose implements GestureRecognizer.
func (t *TapGestureRecognizer) Dispose() {
	if t == nil {
		return
	}
	t.disposed = true
	t.tracking = false
	t.OnTap = nil
	t.a = nil
}

func (t *TapGestureRecognizer) rejectSelf() {
	if t == nil || t.rejected {
		return
	}
	if t.a != nil {
		t.a.Reject(t)
	} else {
		t.Reject()
	}
}

func dist(x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	return math.Hypot(dx, dy)
}

var _ GestureRecognizer = (*TapGestureRecognizer)(nil)
var _ arenaClient = (*TapGestureRecognizer)(nil)
