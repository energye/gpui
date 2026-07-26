package platform

import (
	"sync"
	"time"
)

// StubHost is an in-memory Host for unit tests (no OS window).
type StubHost struct {
	mu     sync.Mutex
	ns     NativeSurface
	w, h   int
	scale  float64
	wake   chan struct{}
	events []Event
}

// NewStubHost creates a test host with the given logical size.
func NewStubHost(w, h int) *StubHost {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return &StubHost{
		w:     w,
		h:     h,
		scale: 1,
		wake:  make(chan struct{}, 1),
		ns:    NativeSurface{Kind: PlatformX11, Display: 1, Window: 1}, // non-zero placeholders
	}
}

// SetNativeSurface overrides handles (tests).
func (h *StubHost) SetNativeSurface(ns NativeSurface) {
	h.mu.Lock()
	h.ns = ns
	h.mu.Unlock()
}

// Push injects events for the next WaitEvents.
func (h *StubHost) Push(ev ...Event) {
	h.mu.Lock()
	h.events = append(h.events, ev...)
	h.mu.Unlock()
	h.WakeUp()
}

// NativeSurface implements Host.
func (h *StubHost) NativeSurface() NativeSurface {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ns
}

// Size implements Host.
func (h *StubHost) Size() (int, int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.w, h.h
}

// ScaleFactor implements Host.
func (h *StubHost) ScaleFactor() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// SetSize updates logical size.
func (h *StubHost) SetSize(w, hgt int) {
	h.mu.Lock()
	h.w, h.h = w, hgt
	h.mu.Unlock()
}

// WaitEvents implements Host.
func (h *StubHost) WaitEvents(timeout time.Duration) []Event {
	h.mu.Lock()
	if len(h.events) > 0 {
		out := h.events
		h.events = nil
		h.mu.Unlock()
		return out
	}
	h.mu.Unlock()

	if timeout == 0 {
		return nil
	}
	if timeout < 0 {
		<-h.wake
	} else {
		select {
		case <-h.wake:
		case <-time.After(timeout):
		}
	}
	h.mu.Lock()
	out := h.events
	h.events = nil
	h.mu.Unlock()
	if out == nil {
		return []Event{{Type: EventWake}}
	}
	return out
}

// WakeUp implements Host.
func (h *StubHost) WakeUp() {
	select {
	case h.wake <- struct{}{}:
	default:
	}
}
