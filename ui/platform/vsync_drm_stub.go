//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

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
