//go:build windows && !nouiplatform

package platform

import (
	"fmt"
	"time"
)

// WindowsHost is an M6 stub host. Real Win32/WinUI adapter lands when GPU
// surface bootstrap for Windows is productized; API matches LinuxHost.
// CapClipboard uses PowerShell Get/Set-Clipboard + memory fallback (cross-platform SPI).
type WindowsHost struct {
	width, height int
	scale         float64
	title         string
	queue         []Event
	closed        bool
	redraws       int
	clip          Clipboard
}

// WindowsOptions configures NewWindowsHost.
type WindowsOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewWindowsHost returns a stub host that accepts synthetic events (tests).
// It does not open a real HWND yet — use Headless for CI; Linux for GPU present.
func NewWindowsHost(opts WindowsOptions) (*WindowsHost, error) {
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
		opts.Scale = 1
	}
	return &WindowsHost{
		width: opts.Width, height: opts.Height,
		scale: opts.Scale, title: opts.Title,
		clip: NewSystemClipboard(),
	}, nil
}

// Caps implements Host — window/input/present claimed; real GPU later.
// CapClipboard is set (PowerShell/clip.exe + memory fallback).
func (h *WindowsHost) Caps() Caps {
	return CapWindow | CapPointer | CapKeyboard | CapTextInput | CapPresent | CapCursor | CapClipboard
}

// Clipboard implements ClipboardProvider.
func (h *WindowsHost) Clipboard() Clipboard {
	if h == nil {
		return nil
	}
	if h.clip == nil {
		h.clip = NewSystemClipboard()
	}
	return h.clip
}

// Size implements Host.
func (h *WindowsHost) Size() (int, int) { return h.width, h.height }

// ScaleFactor implements Host.
func (h *WindowsHost) ScaleFactor() float64 {
	if h.scale <= 0 {
		return 1
	}
	return h.scale
}

// PumpEvents implements Host.
func (h *WindowsHost) PumpEvents() []Event {
	return h.WaitEvents(0)
}

// WaitEvents implements Host (stub: queue + optional sleep).
func (h *WindowsHost) WaitEvents(timeout time.Duration) []Event {
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
func (h *WindowsHost) WakeUp() {}

// RequestRedraw implements Host.
func (h *WindowsHost) RequestRedraw() {
	h.redraws++
	h.queue = append(h.queue, Event{Type: EventRedraw})
	h.WakeUp()
}

// Close implements Host.
func (h *WindowsHost) Close() error {
	h.closed = true
	return nil
}

// Inject enqueues synthetic events (parity with Headless for unit tests).
func (h *WindowsHost) Inject(ev Event) {
	h.queue = append(h.queue, ev)
}

// NativeHandles: HWND not yet available.
func (h *WindowsHost) Display() uintptr { return 0 }
func (h *WindowsHost) Window() uintptr  { return 0 }
func (h *WindowsHost) Flush()           {}

// ErrWindowsGPUNotWired is returned by OpenPresent when real swapchain is needed.
var ErrWindowsGPUNotWired = fmt.Errorf("platform/windows: GPU present not wired (M6 stub; use Linux for PresentFrame*)")

var (
	_ Host          = (*WindowsHost)(nil)
	_ NativeHandles = (*WindowsHost)(nil)
)
