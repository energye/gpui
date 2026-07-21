//go:build !windows || nouiplatform

package platform

import "fmt"

// WindowsHost is unavailable on this OS (compile stub).
type WindowsHost struct{}

// WindowsOptions configures NewWindowsHost.
type WindowsOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewWindowsHost returns an error outside Windows builds.
func NewWindowsHost(opts WindowsOptions) (*WindowsHost, error) {
	return nil, fmt.Errorf("platform: Windows host not available on this OS")
}

func (h *WindowsHost) Caps() Caps           { return 0 }
func (h *WindowsHost) Size() (int, int)     { return 0, 0 }
func (h *WindowsHost) ScaleFactor() float64 { return 1 }
func (h *WindowsHost) PumpEvents() []Event  { return nil }
func (h *WindowsHost) RequestRedraw()       {}
func (h *WindowsHost) Close() error         { return nil }
func (h *WindowsHost) Display() uintptr     { return 0 }
func (h *WindowsHost) Window() uintptr      { return 0 }
func (h *WindowsHost) Flush()               {}
func (h *WindowsHost) Inject(ev Event)      {}
