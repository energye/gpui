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

package gles

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// AdapterContext wraps an EGL/GL context with mutex-protected MakeCurrent switching.
// Shared by Instance → Adapter → Device → Queue (X11/headless), or owned by Surface
// on Wayland (intentional context-per-surface divergence — no wl_display* at Instance init).
type AdapterContext struct {
	mu     sync.Mutex
	eglCtx *egl.Context
	gl     *gl.Context
	owns   bool // true → Destroy() destroys eglCtx
}

// NewAdapterContext wraps an already-created EGL context and GL function table.
// owns=true means Destroy() will call eglCtx.Destroy(); owns=false for borrowed refs.
func NewAdapterContext(eglCtx *egl.Context, glCtx *gl.Context, owns bool) *AdapterContext {
	return &AdapterContext{
		eglCtx: eglCtx,
		gl:     glCtx,
		owns:   owns,
	}
}

// LockMakeCurrentErr is returned when the EGL context cannot be made
// current. Callers must stop: GL calls with no current context are silent
// no-ops (0 handles, FALSE status, empty info logs).
var LockMakeCurrentErr = fmt.Errorf("gles: AdapterContext: MakeCurrent failed")

// TryLock is Lock with a usable error: nil gl + nil error means ready,
// nil gl + LockMakeCurrentErr means the context did not bind (typically
// EGL_BAD_ACCESS 0x3002: another thread holds it, or the display went
// away). Callers stop instead of issuing GL calls into the void.
func (c *AdapterContext) TryLock() (*gl.Context, error) {
	c.mu.Lock()
	runtime.LockOSThread()

	if c.eglCtx == nil {
		c.mu.Unlock()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("gles: AdapterContext: nil egl context")
	}
	if err := c.eglCtx.MakeCurrent(); err != nil {
		c.mu.Unlock()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("%w: %v", LockMakeCurrentErr, err)
	}
	return c.gl, nil
}

// Lock acquires the mutex, pins the goroutine to the current OS thread, and
// makes the GL context current on the pbuffer / surfaceless draw surface.
//
// A bind failure is logged; callers that need a usable context use TryLock
// and stop on error instead of running GL calls with nothing current.
func (c *AdapterContext) Lock() *gl.Context {
	c.mu.Lock()
	runtime.LockOSThread()

	if c.eglCtx == nil {
		slog.Error("gles: AdapterContext.Lock: nil egl context")
		return c.gl
	}
	if err := c.eglCtx.MakeCurrent(); err != nil {
		slog.Error("gles: AdapterContext.Lock MakeCurrent failed", "err", err)
	}
	return c.gl
}

// LockForSurface acquires the mutex, pins the goroutine to the current OS thread,
// and makes the GL context current on the given window EGLSurface.
func (c *AdapterContext) LockForSurface(surf egl.EGLSurface) *gl.Context {
	c.mu.Lock()
	runtime.LockOSThread()

	if c.eglCtx == nil {
		slog.Error("gles: AdapterContext.LockForSurface: nil egl context")
		return c.gl
	}
	if err := c.eglCtx.MakeCurrentSurface(surf); err != nil {
		slog.Error("gles: AdapterContext.LockForSurface MakeCurrent failed",
			"err", err,
			"surface", fmt.Sprintf("0x%x", surf))
	}
	return c.gl
}

// Unlock unmakes the GL context current, unpins the goroutine from the OS
// thread, and releases the mutex.
//
// Guards against double-unmake: only calls eglMakeCurrent(NO_SURFACE, NO_CONTEXT)
// if a context is actually current on this thread.
func (c *AdapterContext) Unlock() {
	// Check eglCtx first so Unlock is safe before egl.Init (unit tests, teardown).
	if c.eglCtx != nil && egl.GetCurrentContext() != egl.NoContext {
		if egl.MakeCurrent(c.eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext) == egl.False {
			slog.Error("gles: AdapterContext.Unlock UnmakeCurrent failed",
				"err", fmt.Sprintf("0x%x", egl.GetError()))
		}
	}
	runtime.UnlockOSThread()
	c.mu.Unlock()
}

// GL returns the GL function table.
// Safe to read without Lock; GL calls on the returned context require Lock.
func (c *AdapterContext) GL() *gl.Context {
	return c.gl
}

// EGL returns the underlying EGL context wrapper.
func (c *AdapterContext) EGL() *egl.Context {
	return c.eglCtx
}

// Destroy deletes the EGL context when this AdapterContext owns it.
// Takes the mutex so Destroy cannot race with Lock/Unlock on another goroutine.
func (c *AdapterContext) Destroy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.owns && c.eglCtx != nil {
		c.eglCtx.Destroy()
		c.eglCtx = nil
		c.gl = nil
	}
}
