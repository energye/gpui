//go:build darwin && !nouiplatform

package platform

import "fmt"

// DarwinHost is an M6 stub host for macOS. Real AppKit/Metal surface adapter
// is deferred; API surface matches LinuxHost/WindowsHost.
type DarwinHost struct {
	width, height int
	scale         float64
	title         string
	queue         []Event
	closed        bool
	redraws       int
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
	}, nil
}

// Caps implements Host.
func (h *DarwinHost) Caps() Caps {
	return CapWindow | CapPointer | CapKeyboard | CapTextInput | CapPresent | CapCursor
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
	if h == nil || h.closed || len(h.queue) == 0 {
		return nil
	}
	out := h.queue
	h.queue = nil
	return out
}

// RequestRedraw implements Host.
func (h *DarwinHost) RequestRedraw() {
	h.redraws++
	h.queue = append(h.queue, Event{Type: EventRedraw})
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
