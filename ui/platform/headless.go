package platform

import "sync"

// Headless is an in-memory Host for layout/hit tests and CI (no OS window).
type Headless struct {
	mu sync.Mutex

	width, height int
	scale         float64
	caps          Caps

	queue   []Event
	redraws int
	closed  bool
	focused bool
}

// NewHeadless creates a headless host with the given logical size.
func NewHeadless(width, height int) *Headless {
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	return &Headless{
		width:   width,
		height:  height,
		scale:   1,
		caps:    HeadlessCaps,
		focused: true,
	}
}

// Caps implements Host.
func (h *Headless) Caps() Caps { return h.caps }

// Size implements Host.
func (h *Headless) Size() (int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.width, h.height
}

// ScaleFactor implements Host.
func (h *Headless) ScaleFactor() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// SetScale sets the DPI scale factor.
func (h *Headless) SetScale(s float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.scale = s
}

// PumpEvents implements Host.
func (h *Headless) PumpEvents() []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

// RequestRedraw implements Host.
func (h *Headless) RequestRedraw() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.redraws++
	h.queue = append(h.queue, Event{Type: EventRedraw})
}

// RedrawCount returns how many times RequestRedraw was called.
func (h *Headless) RedrawCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.redraws
}

// Close implements Host.
func (h *Headless) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	return nil
}

// Inject enqueues a synthetic event (tests).
func (h *Headless) Inject(ev Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.queue = append(h.queue, ev)
}

// InjectPointer is a convenience for pointer events.
func (h *Headless) InjectPointer(kind PointerKind, x, y float64, btn PointerBtn) {
	h.Inject(Event{Type: EventPointer, Pointer: kind, X: x, Y: y, Button: btn})
}

// InjectClick synthesizes left-button down+up at (x,y).
func (h *Headless) InjectClick(x, y float64) {
	h.InjectPointer(PointerDown, x, y, BtnLeft)
	h.InjectPointer(PointerUp, x, y, BtnLeft)
}

// Resize changes the client size and enqueues a resize event.
func (h *Headless) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.width, h.height = width, height
	h.queue = append(h.queue, Event{Type: EventResize, Width: width, Height: height})
}

// var _ Host = (*Headless)(nil) at bottom of file after methods — compile check.
var _ Host = (*Headless)(nil)
