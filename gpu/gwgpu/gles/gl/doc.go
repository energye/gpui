//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

// Package gl provides low-level OpenGL function bindings using syscall.
//
// This package is used internally by the GLES HAL backend.
// It loads OpenGL functions at runtime via wglGetProcAddress (Windows)
// or platform-specific loaders.
//
// # Usage
//
// The Context type holds function pointers loaded at runtime:
//
//	ctx := &gl.Context{}
//	ctx.Load(wgl.GetGLProcAddress)
//	ctx.ClearColor(0.2, 0.3, 0.3, 1.0)
//	ctx.Clear(gl.COLOR_BUFFER_BIT)
package gl
