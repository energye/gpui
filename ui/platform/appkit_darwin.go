//go:build darwin

package platform

import "fmt"

// appkitBackend is the reserved AppKit backend (NSView*/CAMetalLayer*).
// Implementation lands with the Win/mac platform milestone.
type appkitBackend struct{}

func init() { Register(PlatformAppKit, &appkitBackend{}) }

func (b *appkitBackend) Kind() PlatformKind { return PlatformAppKit }

// Create returns a clear "not implemented yet" error (reserved interface).
func (b *appkitBackend) Create(opts Options) (*Window, error) {
	return nil, fmt.Errorf("appkit: backend not implemented yet (reserved)")
}

// Adopt returns a clear error (reserved interface).
func (b *appkitBackend) Adopt(ns NativeSurface) (*Window, error) {
	return nil, fmt.Errorf("appkit: Adopt not implemented yet (reserved)")
}
