//go:build windows

package platform

import "fmt"

// win32Backend is the reserved Win32 backend (HWND). Implementation lands
// with the Win/mac platform milestone; the interface is registered so the
// registry routing is already exercised.
type win32Backend struct{}

func init() { Register(PlatformWin32, &win32Backend{}) }

func (b *win32Backend) Kind() PlatformKind { return PlatformWin32 }

// Create returns a clear "not implemented yet" error (reserved interface).
func (b *win32Backend) Create(opts Options) (*Window, error) {
	return nil, fmt.Errorf("win32: backend not implemented yet (reserved)")
}

// Adopt returns a clear error (reserved interface).
func (b *win32Backend) Adopt(ns NativeSurface) (*Window, error) {
	return nil, fmt.Errorf("win32: Adopt not implemented yet (reserved)")
}
