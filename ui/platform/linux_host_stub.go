//go:build !linux || nouiplatform

package platform

import "fmt"

// LinuxHost is unavailable on this platform (stub for compile).
type LinuxHost struct{}

// LinuxOptions configures NewLinuxHost.
type LinuxOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewLinuxHost returns an error on non-Linux builds (use WindowsHost/DarwinHost/Headless).
func NewLinuxHost(opts LinuxOptions) (*LinuxHost, error) {
	return nil, fmt.Errorf("platform: Linux host not available on this OS (use NewHost / Headless / Windows / Darwin stubs)")
}

func (h *LinuxHost) Caps() Caps           { return 0 }
func (h *LinuxHost) Size() (int, int)     { return 0, 0 }
func (h *LinuxHost) ScaleFactor() float64 { return 1 }
func (h *LinuxHost) PumpEvents() []Event  { return nil }
func (h *LinuxHost) RequestRedraw()       {}
func (h *LinuxHost) Close() error         { return nil }
func (h *LinuxHost) Display() uintptr     { return 0 }
func (h *LinuxHost) Window() uintptr      { return 0 }
func (h *LinuxHost) Flush()               {}
