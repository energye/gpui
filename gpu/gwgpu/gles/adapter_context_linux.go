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
	"syscall"

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

	boundValid     bool
	boundIsPbuffer bool
	boundSurf      egl.EGLSurface
	boundTid       int

	// targetAdjust is the internal target coordinate adjustment block (see
	// target_adjust.go). Per GL context, so it lives here rather than on a
	// Device: several devices may share one context.
	targetAdjust targetAdjustState
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

// currentTid returns the Linux thread id for the binding cache.
// Gettid is a raw syscall (~tens of ns) against eglMakeCurrent FFI + driver
// sync (~us). Only called with mu held and the goroutine pinned.
func currentTid() int {
	return syscall.Gettid()
}

// invalidateLocked clears the binding cache; caller holds c.mu (all binds
// run serialized under it, so the record is always exact).
func (c *AdapterContext) invalidateLocked() {
	c.boundValid = false
}

// InvalidateBinding drops the binding cache (surface teardown paths that run
// without holding c.mu). A stale entry could otherwise fast-hit a recycled
// surface handle; the next bind then remakes. Never blocks frame work: it
// only takes the short mutex, never waits on GL.
func (c *AdapterContext) InvalidateBinding() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.boundValid = false
	c.mu.Unlock()
}

// bindSurfaceLocked makes the context current on surf (fast path: same OS
// thread already bound to the same surface). Caller holds c.mu and the
// OSThread pin. Records the binding on success, clears it on failure.
func (c *AdapterContext) bindSurfaceLocked(surf egl.EGLSurface) error {
	if c.eglCtx == nil {
		c.boundValid = false
		return fmt.Errorf("gles: AdapterContext: nil egl context")
	}
	if tid := currentTid(); c.boundValid && !c.boundIsPbuffer && c.boundSurf == surf && c.boundTid == tid {
		return nil
	}
	if err := c.eglCtx.MakeCurrentSurface(surf); err != nil {
		c.invalidateLocked()
		return err
	}
	c.boundValid = true
	c.boundIsPbuffer = false
	c.boundSurf = surf
	c.boundTid = currentTid()
	return nil
}

// bindPbufferLocked makes the context current on the pbuffer/surfaceless
// surface (fast path: same OS thread already has ANY binding — resource
// creation and uploads do not depend on the draw surface). Caller holds c.mu
// and the OSThread pin. Records the binding on success, clears on failure.
func (c *AdapterContext) bindPbufferLocked() error {
	if c.eglCtx == nil {
		c.boundValid = false
		return fmt.Errorf("gles: AdapterContext: nil egl context")
	}
	if tid := currentTid(); c.boundValid && c.boundTid == tid {
		return nil
	}
	if err := c.eglCtx.MakeCurrent(); err != nil {
		c.invalidateLocked()
		return err
	}
	c.boundValid = true
	c.boundIsPbuffer = true
	c.boundTid = currentTid()
	return nil
}

// TryLock is Lock with a usable error: non-nil gl + nil error means ready,
// nil gl + LockMakeCurrentErr means the context did not bind (typically
// EGL_BAD_ACCESS 0x3002: another thread holds it, or the display went
// away). Callers stop instead of issuing GL calls into the void.
func (c *AdapterContext) TryLock() (*gl.Context, error) {
	c.mu.Lock()
	runtime.LockOSThread()
	if err := c.bindPbufferLocked(); err != nil {
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
	if err := c.bindPbufferLocked(); err != nil {
		slog.Error("gles: AdapterContext.Lock MakeCurrent failed", "err", err)
	}
	return c.gl
}

// TryLockForSurface is the TryLock twin for a window EGLSurface: non-nil gl
// + nil error means ready, nil gl + LockMakeCurrentErr means the bind
// failed (dead surface, BadAlloc 0x3003, display lost). Callers stop
// instead of issuing GL calls into the void — the old LockForSurface
// logged and still returned gl, so Present blitted and swapped unbound
// (transparent stall).
func (c *AdapterContext) TryLockForSurface(surf egl.EGLSurface) (*gl.Context, error) {
	c.mu.Lock()
	runtime.LockOSThread()
	if err := c.bindSurfaceLocked(surf); err != nil {
		c.mu.Unlock()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("%w: %v", LockMakeCurrentErr, err)
	}
	return c.gl, nil
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
	if err := c.bindSurfaceLocked(surf); err != nil {
		slog.Error("gles: AdapterContext.LockForSurface MakeCurrent failed",
			"err", err,
			"surface", fmt.Sprintf("0x%x", surf))
	}
	return c.gl
}

// Unlock unmakes the GL context current, unpins the goroutine from the OS
// thread, and releases the mutex.
//
// The unmake is unconditional: the mutex guarantees this Lock/Unlock pair
// owns the bind, so no GetCurrentContext query is needed to guard against
// double-unmake (pprof: the query cost 250ms/2.32s on spin_min — every
// CreateCommandEncoder/Submit/Present Unlock paid one FFI round trip).
// Unlock stays safe before egl.Init (unit tests, teardown) via the nil check.
func (c *AdapterContext) Unlock() {
	// Check eglCtx first so Unlock is safe before egl.Init (unit tests, teardown).
	if c.eglCtx != nil {
		if egl.MakeCurrent(c.eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext) == egl.False {
			slog.Error("gles: AdapterContext.Unlock UnmakeCurrent failed",
				"err", fmt.Sprintf("0x%x", egl.GetError()))
		}
	}
	c.boundValid = false
	runtime.UnlockOSThread()
	c.mu.Unlock()
}

// UnlockKeepBinding releases the mutex and unpins the goroutine but keeps
// the GL context current on this OS thread (skips the UnmakeCurrent FFI).
// Exactness comes from the binding cache above: the next bind on the same
// thread fast-hits only when thread + surface-kind + surface all match, any
// other-thread bind overwrites the record, and every Unmake/failure/teardown
// clears it — so a kept binding can never be mistaken for another one.
// Frame-end paths (Submit/SubmitAndPresent) keep; all other callers keep Unlock.
func (c *AdapterContext) UnlockKeepBinding() {
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

// allocTargetAdjustStore creates the (empty) 16-byte store of the target
// adjustment buffer. glBufferData's data parameter is uintptr in this context
// wrapper and unsafe.Pointer in the WGL one, so this single call cannot be
// shared (twin: adapter_context.go).
func allocTargetAdjustStore(glCtx *gl.Context, ubo uint32) {
	glCtx.BindBuffer(gl.UNIFORM_BUFFER, ubo)
	glCtx.BufferData(gl.UNIFORM_BUFFER, targetAdjustSize, 0, gl.DYNAMIC_DRAW)
}

// Destroy deletes the EGL context when this AdapterContext owns it.
// Takes the mutex so Destroy cannot race with Lock/Unlock on another goroutine.
func (c *AdapterContext) Destroy() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boundValid = false
	// The target adjustment buffer dies with the context (GL objects belong to
	// their context), so only the stale id has to be dropped: reusing it in a
	// recreated context would bind a recycled buffer name.
	c.targetAdjust = targetAdjustState{}
	if c.owns && c.eglCtx != nil {
		c.eglCtx.Destroy()
		c.eglCtx = nil
		c.gl = nil
	}
}
