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

package webgpu

// x11WindowSize is unavailable off Linux; callers fall back to the applied
// swapchain size (the configured surface stays authoritative on Wayland /
// Windows / macOS, where the app is told the size instead of probing it).
func x11WindowSize(display, window uintptr) (w, h int, ok bool) {
	return 0, 0, false
}
