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

	// Last SetIMEPosition (tests / C3).
	imeX, imeY float64
	imePosN    int
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

// InjectScroll enqueues a scroll event at (x,y).
func (h *Headless) InjectScroll(x, y, dx, dy float64) {
	h.Inject(Event{Type: EventScroll, X: x, Y: y, ScrollDX: dx, ScrollDY: dy})
}

// InjectText enqueues committed text input.
func (h *Headless) InjectText(text string) {
	h.Inject(Event{Type: EventText, Text: text})
}

// InjectKey enqueues a key down/up.
func (h *Headless) InjectKey(key string, down bool) {
	h.Inject(Event{Type: EventKey, Key: key, Down: down})
}

// InjectIME enqueues an IME composition update (requires CapIME — set on Headless).
// End=true clears preedit; non-empty Text with End may be committed by EditableText.
func (h *Headless) InjectIME(text string, end bool) {
	h.Inject(Event{Type: EventIME, IMEText: text, IMEEnd: end})
}

// SetIMEPosition implements IMEPositioner for tests (records last caret request).
func (h *Headless) SetIMEPosition(x, y float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.imeX, h.imeY = x, y
	h.imePosN++
}

// LastIMEPosition returns the last SetIMEPosition arguments and call count.
func (h *Headless) LastIMEPosition() (x, y float64, n int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.imeX, h.imeY, h.imePosN
}

// var _ Host = (*Headless)(nil) at bottom of file after methods — compile check.
var (
	_ Host          = (*Headless)(nil)
	_ IMEPositioner = (*Headless)(nil)
)
