//go:build darwin && !nouiplatform

package platform

import (
	"fmt"
	"time"
)

// DarwinHost is an M6 stub host for macOS. Real AppKit/Metal surface adapter
// is deferred; API surface matches LinuxHost/WindowsHost.
// CapClipboard uses pbcopy/pbpaste + memory fallback (cross-platform SPI).
type DarwinHost struct {
	width, height int
	scale         float64
	title         string
	queue         []Event
	closed        bool
	redraws       int
	clip          Clipboard
}

// DarwinOptions configures NewDarwinHost.
type DarwinOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewDarwinHost returns a stub host (synthetic events only).
func NewDarwinHost(opts DarwinOptions) (*DarwinHost, error) {
	if opts.Width <= 0 {
		opts.Width = 800
	}
	if opts.Height <= 0 {
		opts.Height = 600
	}
	if opts.Title == "" {
		opts.Title = "gpui"
	}
	if opts.Scale <= 0 {
		opts.Scale = 2 // Retina default hint
	}
	return &DarwinHost{
		width: opts.Width, height: opts.Height,
		scale: opts.Scale, title: opts.Title,
		clip: NewSystemClipboard(),
	}, nil
}

// Caps implements Host.
// CapClipboard is set (pbcopy/pbpaste + memory fallback).
func (h *DarwinHost) Caps() Caps {
	return CapWindow | CapPointer | CapKeyboard | CapTextInput | CapPresent | CapCursor | CapClipboard
}

// Clipboard implements ClipboardProvider.
func (h *DarwinHost) Clipboard() Clipboard {
	if h == nil {
		return nil
	}
	if h.clip == nil {
		h.clip = NewSystemClipboard()
	}
	return h.clip
}

// Size implements Host.
func (h *DarwinHost) Size() (int, int) { return h.width, h.height }

// ScaleFactor implements Host.
func (h *DarwinHost) ScaleFactor() float64 {
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// PumpEvents implements Host.
func (h *DarwinHost) PumpEvents() []Event {
	return h.WaitEvents(0)
}

// WaitEvents implements Host (stub: queue + optional sleep).
func (h *DarwinHost) WaitEvents(timeout time.Duration) []Event {
	if h == nil || h.closed {
		return nil
	}
	if len(h.queue) > 0 {
		out := h.queue
		h.queue = nil
		return out
	}
	if timeout > 0 {
		time.Sleep(timeout)
		if len(h.queue) > 0 {
			out := h.queue
			h.queue = nil
			return out
		}
	}
	return nil
}

// WakeUp implements Host (no waiter on stub).
func (h *DarwinHost) WakeUp() {}

// RequestRedraw implements Host.
func (h *DarwinHost) RequestRedraw() {
	h.redraws++
	h.queue = append(h.queue, Event{Type: EventRedraw})
	h.WakeUp()
}

// Close implements Host.
func (h *DarwinHost) Close() error {
	h.closed = true
	return nil
}

// Inject enqueues synthetic events.
func (h *DarwinHost) Inject(ev Event) {
	h.queue = append(h.queue, ev)
}

func (h *DarwinHost) Display() uintptr { return 0 }
func (h *DarwinHost) Window() uintptr  { return 0 }
func (h *DarwinHost) Flush()           {}

// ErrDarwinGPUNotWired is returned when real Metal present is requested.
var ErrDarwinGPUNotWired = fmt.Errorf("platform/darwin: GPU present not wired (M6 stub; use Linux for PresentFrame*)")

var (
	_ Host          = (*DarwinHost)(nil)
	_ NativeHandles = (*DarwinHost)(nil)
)
