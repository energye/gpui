//go:build !linux

package platform

import "errors"

// ErrNoDRMVBlank is returned when no usable DRM vblank path is available.
var ErrNoDRMVBlank = errors.New("platform: DRM vblank not supported on this OS")

// InitDRMVBlank is a no-op stub off Linux.
func InitDRMVBlank() error { return ErrNoDRMVBlank }

// WaitDRMVBlank always fails off Linux.
func WaitDRMVBlank() error { return ErrNoDRMVBlank }

// HasDRMVBlank is always false off Linux.
func HasDRMVBlank() bool { return false }
