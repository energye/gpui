//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// Command represents a recorded GL command.
type Command interface {
	Execute(ctx *gl.Context, st *glExecState)
}

// CommandBuffer holds recorded commands for later execution.
type CommandBuffer struct {
	commands []Command
	// windowSurface is set when this buffer renders into a Surface's window
	// default framebuffer (default_fb_linux.go). FBO 0 is a drawable only for
	// the EGL surface the context is bound to, while the pbuffer binding the
	// plain Submit path uses leaves it without one: Queue.Submit binds the
	// window before running such a buffer.
	windowSurface *Surface
}
