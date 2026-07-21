//go:build !darwin || nouiplatform

package platform

import (
	"fmt"
	"time"
)

// DarwinHost is unavailable on this OS (compile stub).
type DarwinHost struct{}

// DarwinOptions configures NewDarwinHost.
type DarwinOptions struct {
	Width, Height int
	Title         string
	Scale         float64
}

// NewDarwinHost returns an error outside Darwin builds.
func NewDarwinHost(opts DarwinOptions) (*DarwinHost, error) {
	return nil, fmt.Errorf("platform: Darwin host not available on this OS")
}

func (h *DarwinHost) Caps() Caps                               { return 0 }
func (h *DarwinHost) Size() (int, int)                         { return 0, 0 }
func (h *DarwinHost) ScaleFactor() float64                     { return 1 }
func (h *DarwinHost) PumpEvents() []Event                      { return nil }
func (h *DarwinHost) WaitEvents(timeout time.Duration) []Event { return nil }
func (h *DarwinHost) WakeUp()                                  {}
func (h *DarwinHost) RequestRedraw()                           {}
func (h *DarwinHost) Close() error                             { return nil }
func (h *DarwinHost) Display() uintptr                         { return 0 }
func (h *DarwinHost) Window() uintptr                          { return 0 }
func (h *DarwinHost) Flush()                                   {}
func (h *DarwinHost) Inject(ev Event)                          {}
