package platform

import (
	"sync"
	"time"
)

// Headless is an in-memory Host for layout/hit tests and CI (no OS window).
// Supports gogpu-style WaitEvents / WakeUp for demand-driven frame tests.
type Headless struct {
	mu sync.Mutex
	cv *sync.Cond

	width, height int
	scale         float64
	caps          Caps

	queue   []Event
	redraws int
	closed  bool
	focused bool
	wake    int // WakeUp generations

	// Last SetIMEPosition (tests / C3).
	imeX, imeY float64
	imePosN    int

	// In-memory clipboard (CapClipboard).
	clip *MemoryClipboard

	// Last cursor (CursorHost; tests).
	lastCursor CursorKind
	hasCursor  bool
}

// NewHeadless creates a headless host with the given logical size.
func NewHeadless(width, height int) *Headless {
	if width <= 0 {
		width = 800
	}
	if height <= 0 {
		height = 600
	}
	h := &Headless{
		width:   width,
		height:  height,
		scale:   1,
		caps:    HeadlessCaps,
		focused: true,
		clip:    NewMemoryClipboard(),
	}
	h.cv = sync.NewCond(&h.mu)
	return h
}

// Clipboard implements ClipboardProvider.
func (h *Headless) Clipboard() Clipboard {
	if h == nil {
		return nil
	}
	return h.clip
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

// PumpEvents implements Host (non-blocking).
func (h *Headless) PumpEvents() []Event {
	return h.WaitEvents(0)
}

// WaitEvents implements Host.
func (h *Headless) WaitEvents(timeout time.Duration) []Event {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil
	}
	if len(h.queue) > 0 {
		return h.takeQueueLocked()
	}
	if timeout == 0 {
		return nil
	}
	if timeout > 0 {
		// Timed wait via channel + cond (avoid holding lock during sleep incorrectly).
		timer := time.AfterFunc(timeout, func() { h.WakeUp() })
		defer timer.Stop()
		startWake := h.wake
		for len(h.queue) == 0 && !h.closed && h.wake == startWake {
			h.cv.Wait()
		}
	} else {
		// Block until queue, WakeUp, or close.
		for len(h.queue) == 0 && !h.closed {
			startWake := h.wake
			h.cv.Wait()
			if h.wake != startWake && len(h.queue) == 0 {
				// Pure wakeup without events — return empty (scheduler re-checks).
				return nil
			}
		}
	}
	if h.closed {
		return nil
	}
	return h.takeQueueLocked()
}

func (h *Headless) takeQueueLocked() []Event {
	if len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

// WakeUp implements Host.
func (h *Headless) WakeUp() {
	h.mu.Lock()
	h.wake++
	h.mu.Unlock()
	h.cv.Broadcast()
}

// RequestRedraw implements Host.
func (h *Headless) RequestRedraw() {
	h.mu.Lock()
	h.redraws++
	h.queue = append(h.queue, Event{Type: EventRedraw})
	h.mu.Unlock()
	h.WakeUp()
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
	h.closed = true
	h.mu.Unlock()
	h.WakeUp()
	return nil
}

// Inject enqueues a synthetic event (tests).
func (h *Headless) Inject(ev Event) {
	h.mu.Lock()
	h.queue = append(h.queue, ev)
	h.mu.Unlock()
	h.WakeUp()
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
	h.width, h.height = width, height
	h.queue = append(h.queue, Event{Type: EventResize, Width: width, Height: height})
	h.mu.Unlock()
	h.WakeUp()
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

// SetCursor implements CursorHost (records kind for tests).
func (h *Headless) SetCursor(kind CursorKind) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.lastCursor = kind
	h.hasCursor = true
	h.mu.Unlock()
}

// LastCursor returns the last SetCursor kind.
func (h *Headless) LastCursor() (CursorKind, bool) {
	if h == nil {
		return CursorDefault, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastCursor, h.hasCursor
}

var (
	_ Host          = (*Headless)(nil)
	_ IMEPositioner = (*Headless)(nil)
	_ CursorHost    = (*Headless)(nil)
)
