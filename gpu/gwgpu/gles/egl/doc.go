//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

// Package egl provides EGL (EGL) context management for OpenGL ES on Linux.
//
// EGL is the native interface for OpenGL ES on Linux, supporting:
//   - X11 displays (via EGL_PLATFORM_X11_KHR)
//   - Wayland displays (via EGL_PLATFORM_WAYLAND_KHR)
//   - Surfaceless contexts (via EGL_PLATFORM_SURFACELESS_MESA)
//
// This implementation uses gpu/gwgpu/ffishim (purego-backed, no CGO,
// no external goffi dependency) for Pure Go FFI.
package egl
