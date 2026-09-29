// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package gles

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles/egl"
	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// vendorUnknown is the placeholder vendor name used when the actual GPU vendor
// cannot be determined (e.g., no surface available during adapter enumeration).
const vendorUnknown = "Unknown"

// driverOpenGL is the AdapterInfo.Driver string for GLES backends.
const driverOpenGL = "OpenGL"

// Backend implements hal.Backend for OpenGL ES / OpenGL 3.3+ on Linux.
type Backend struct{}

// NewBackend returns a GLES backend instance.
func NewBackend() Backend { return Backend{} }

// Variant returns the backend type identifier.
func (Backend) Variant() gputypes.Backend {
	return gputypes.BackendGL
}

// CreateInstance creates a new OpenGL instance with an optional EGL context.
// Attempts to create an EGL context at instance level (Rust wgpu-hal egl.rs:846
// parity) for adapter enumeration without a surface. Uses surfaceless/pbuffer
// context — same role as Windows hidden 1×1 HWND (v0.28.6).
//
// On Wayland, this may fail (EGL needs wl_display*) — that's OK, CreateSurface
// provides the proper context later. On X11/headless, this succeeds.
func (Backend) CreateInstance(_ *hal.InstanceDescriptor) (hal.Instance, error) {
	// PRIME selection (GPUI_POWER=high): EGL resolves the render node at
	// display bring-up, so DRI_PRIME=1 must be in the env before the first
	// EGL call in this process. Set it here when the caller asked for
	// discrete; do not override an explicit user value, do not touch it
	// for low/default (Mesa default is the integrated node).
	// P2-0 vendor pinning rides the same hook: __EGL_VENDOR_LIBRARY_FILENAMES
	// narrows libglvnd arbitration to the policy vendor before first use.
	applyPrimeEnvForPower()
	applyVendorPinForPower()
	if err := egl.Init(); err != nil {
		return nil, fmt.Errorf("gles: failed to initialize EGL: %w", err)
	}

	// Try to create instance-level EGL context (Rust wgpu-hal parity).
	// Skip on Wayland: surfaceless context would create GL objects (VAO, FBO) that
	// are invisible to the Surface's windowed context (GL objects not shared between
	// EGL contexts). Device/Queue must use the SAME context as the window surface.
	// On X11/headless, instance context IS the presentation context — safe to create.
	if egl.DetectWindowKind() == egl.WindowKindWayland {
		hal.Logger().Info("gles: skipping instance context on Wayland (surface provides context)")
		return &Instance{}, nil
	}

	config := egl.DefaultContextConfig()
	config.GLES = false
	eglCtx, err := egl.NewContext(config)
	if err != nil {
		hal.Logger().Info("gles: instance context unavailable (expected on Wayland)", "err", err)
		return &Instance{}, nil
	}

	if err := eglCtx.MakeCurrent(); err != nil {
		eglCtx.Destroy()
		hal.Logger().Warn("gles: instance context MakeCurrent failed", "err", err)
		return &Instance{}, nil
	}

	glCtx := &gl.Context{}
	if err := glCtx.Load(egl.GetGLProcAddress); err != nil {
		eglCtx.Destroy()
		hal.Logger().Warn("gles: instance GL load failed", "err", err)
		return &Instance{}, nil
	}

	hal.Logger().Info("gles: instance created with EGL context",
		"version", glCtx.GetString(gl.VERSION),
		"renderer", glCtx.GetString(gl.RENDERER))

	// Unbind right after init: a current context stays pinned to this
	// goroutine's OS thread and later Locks on other threads fail with
	// EGL_BAD_ACCESS (0x3002), turning later GL calls into silent no-ops.
	_ = egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	return &Instance{ctx: NewAdapterContext(eglCtx, glCtx, true)}, nil
}

// Instance implements hal.Instance for the OpenGL backend on Linux.
// ctx is non-nil when an instance-level EGL context was created successfully
// (X11/headless). On Wayland it may be nil — CreateSurface provides a
// Surface-owned AdapterContext when a window handle is available.
type Instance struct {
	ctx *AdapterContext
	// Shared X11 context: the Queue binds one context, so every window
	// surface must live on the same EGL display — first X11 window opens
	// Display* and creates the context, later windows add window surfaces
	// on it. Wayland surfaces keep per-surface contexts (per wl_display).
	x11 *x11Shared
}

type x11Shared struct {
	ctx     *AdapterContext
	display uintptr
}

// CreateSurface creates an OpenGL surface from window handles.
// On Linux: displayHandle and windowHandle are platform-specific.
// For X11: displayHandle is X11 Display*, windowHandle is Window.
// For Wayland: displayHandle is wl_display*, windowHandle is wl_surface*.
//
// When Instance has a pre-created EGL context (X11/headless), the context is
// SHARED — Surface only creates an EGL window surface for presentation. This
// matches the Windows pattern where Instance owns AdapterContext and Surface
// is lightweight (just HWND + reference to shared ctx).
//
// When Instance has no context (Wayland — no wl_display* at init), CreateSurface
// creates a new EGL context with the caller's displayHandle and wraps it in a
// Surface-owned AdapterContext (intentional Wayland divergence).
func (i *Instance) CreateSurface(target hal.SurfaceTarget) (hal.Surface, error) {
	var targetWindowKind egl.WindowKind
	switch target.Kind {
	case hal.SurfaceTargetXlibWindow:
		targetWindowKind = egl.WindowKindX11
	case hal.SurfaceTargetWaylandSurface:
		targetWindowKind = egl.WindowKindWayland
	default:
		return nil, fmt.Errorf("gles: %w: got %s, backend requires Xlib window or Wayland surface", hal.ErrUnsupportedSurfaceTarget, target.Kind)
	}
	displayHandle, windowHandle := target.DisplayHandle, target.WindowHandle

	// X11 shares one instance-owned context across windows (see x11 field).
	if targetWindowKind == egl.WindowKindX11 {
		return i.createX11Surface(displayHandle, windowHandle)
	}

	// Path A: share Instance AdapterContext (headless context matches window system).
	// Do NOT share if Instance context is surfaceless (headless/Wayland fallback)
	// and Surface needs a window — the EGL display won't support eglCreateWindowSurface.
	if i.ctx != nil && i.ctx.EGL() != nil && i.ctx.GL() != nil && i.ctx.EGL().WindowKind() == targetWindowKind {
		hal.Logger().Info("gles: surface sharing Instance AdapterContext")
		glCtx := i.ctx.Lock()
		version := glCtx.GetString(gl.VERSION)
		renderer := glCtx.GetString(gl.RENDERER)
		i.ctx.Unlock()
		return &Surface{
			displayHandle: displayHandle,
			windowHandle:  windowHandle,
			ctx:           i.ctx,
			eglDisplay:    i.ctx.EGL().Display(),
			ownsContext:   false,
			version:       version,
			renderer:      renderer,
		}, nil
	}

	// Path B: create new context (Wayland — Instance had no wl_display* at init).
	// Try desktop GL first, fall back to GLES 3.0 — Mesa Wayland EGL may only
	// expose EGL_OPENGL_ES3_BIT configs, not EGL_OPENGL_BIT. Found by @lkmavi (PR #215).
	config := egl.DefaultContextConfig()
	config.NativeDisplay = displayHandle
	config.WindowKind = &targetWindowKind
	eglCtx, err := egl.NewContext(config)
	if err != nil {
		config.GLES = true
		config.GLVersionMajor = 3
		config.GLVersionMinor = 0
		config.CoreProfile = false
		eglCtx, err = egl.NewContext(config)
	}
	if err != nil {
		return nil, fmt.Errorf("gles: failed to create EGL context (tried desktop GL and GLES 3.0): %w", err)
	}

	if err := eglCtx.MakeCurrent(); err != nil {
		eglCtx.Destroy()
		return nil, fmt.Errorf("gles: failed to make context current: %w", err)
	}

	glCtx := &gl.Context{}
	if err := glCtx.Load(egl.GetGLProcAddress, config.GLES); err != nil {
		eglCtx.Destroy()
		return nil, fmt.Errorf("gles: failed to load GL functions: %w", err)
	}

	version := glCtx.GetString(gl.VERSION)
	renderer := glCtx.GetString(gl.RENDERER)
	hal.Logger().Info("gles: surface created with owned AdapterContext",
		"version", version, "renderer", renderer, "gles", config.GLES)

	// Same unbind as CreateInstance: don't pin the new context to the
	// creation thread.
	_ = egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)

	return &Surface{
		displayHandle: displayHandle,
		windowHandle:  windowHandle,
		ctx:           NewAdapterContext(eglCtx, glCtx, true),
		eglDisplay:    eglCtx.Display(),
		ownsContext:   true,
		version:       version,
		renderer:      renderer,
	}, nil
}

// createX11Surface shares one instance-owned X11 context across windows.
func (i *Instance) createX11Surface(displayHandle, windowHandle uintptr) (hal.Surface, error) {
	sharedOK := i.x11 != nil && i.x11.ctx != nil && i.x11.ctx.EGL() != nil && i.x11.ctx.GL() != nil
	if !sharedOK {
		// Fresh share on the caller's display: each XOpenDisplay connection
		// needs its own EGL display, so the first window's Display* is kept.
		config := egl.DefaultContextConfig()
		config.GLES = false
		config.NativeDisplay = displayHandle
		x11kind := egl.WindowKindX11
		config.WindowKind = &x11kind
		eglCtx, err := egl.NewContext(config)
		if err != nil {
			return nil, fmt.Errorf("gles: failed to create shared X11 EGL context: %w", err)
		}
		if err := eglCtx.MakeCurrent(); err != nil {
			eglCtx.Destroy()
			return nil, fmt.Errorf("gles: failed to make shared X11 context current: %w", err)
		}
		glCtx := &gl.Context{}
		if err := glCtx.Load(egl.GetGLProcAddress, config.GLES); err != nil {
			eglCtx.Destroy()
			return nil, fmt.Errorf("gles: failed to load GL functions: %w", err)
		}
		hal.Logger().Info("gles: shared X11 context created",
			"version", glCtx.GetString(gl.VERSION),
			"renderer", glCtx.GetString(gl.RENDERER))
		// Same unbind as CreateInstance: don't pin the context to the
		// creation thread.
		_ = egl.MakeCurrent(eglCtx.Display(), egl.NoSurface, egl.NoSurface, egl.NoContext)
		i.x11 = &x11Shared{ctx: NewAdapterContext(eglCtx, glCtx, true)}
	}

	glCtx := i.x11.ctx.Lock()
	version := glCtx.GetString(gl.VERSION)
	renderer := glCtx.GetString(gl.RENDERER)
	i.x11.ctx.Unlock()
	return &Surface{
		displayHandle: displayHandle,
		windowHandle:  windowHandle,
		ctx:           i.x11.ctx,
		eglDisplay:    i.x11.ctx.EGL().Display(),
		ownsContext:   false,
		version:       version,
		renderer:      renderer,
	}, nil
}

// EnumerateAdapters returns available OpenGL adapters.
// Uses surface context (preferred), instance context (X11/headless), or placeholder.
func (i *Instance) EnumerateAdapters(surfaceHint hal.Surface) []hal.ExposedAdapter {
	// Priority 1: surface provides the best context (has window handle)
	if surface, ok := surfaceHint.(*Surface); ok {
		return []hal.ExposedAdapter{surface.GetAdapterInfo()}
	}

	// Priority 2: instance-level AdapterContext (created in CreateInstance via pbuffer/surfaceless)
	if i.ctx != nil && i.ctx.GL() != nil {
		return []hal.ExposedAdapter{
			makeAdapterFromContext(i.ctx),
		}
	}

	// Priority 3: no context available (Wayland without surface hint)
	// Return placeholder — Open() has nil guard from PR #210.
	return []hal.ExposedAdapter{placeholderExposedAdapter("OpenGL 3.3+ / ES 3.0+ (no context — use RequestAdapterWithSurface)")}
}

// placeholderExposedAdapter returns a non-openable adapter when no EGL context exists.
func placeholderExposedAdapter(driverInfo string) hal.ExposedAdapter {
	return hal.ExposedAdapter{
		Adapter: &Adapter{},
		Info: gputypes.AdapterInfo{
			Name:       "OpenGL Adapter",
			Vendor:     vendorUnknown,
			DeviceType: gputypes.DeviceTypeOther,
			Driver:     driverOpenGL,
			DriverInfo: driverInfo,
			Backend:    gputypes.BackendGL,
		},
		Capabilities: hal.Capabilities{
			Limits: gputypes.DefaultLimits(),
			AlignmentsMask: hal.Alignments{
				BufferCopyOffset: 4,
				BufferCopyPitch:  256,
			},
		},
	}
}

// makeAdapterFromContext creates an ExposedAdapter using a live AdapterContext.
func makeAdapterFromContext(ctx *AdapterContext) hal.ExposedAdapter {
	glCtx := ctx.Lock()
	defer ctx.Unlock()

	version := glCtx.GetString(gl.VERSION)
	renderer := glCtx.GetString(gl.RENDERER)
	vendor := glCtx.GetString(gl.VENDOR)

	return hal.ExposedAdapter{
		Adapter: &Adapter{
			ctx:      ctx,
			version:  version,
			renderer: renderer,
		},
		Info: gputypes.AdapterInfo{
			Name:       renderer,
			Vendor:     vendor,
			DeviceType: gputypes.DeviceTypeIntegratedGPU,
			Driver:     driverOpenGL,
			DriverInfo: version,
			Backend:    gputypes.BackendGL,
		},
		Capabilities: hal.Capabilities{
			Limits: gputypes.DefaultLimits(),
			AlignmentsMask: hal.Alignments{
				BufferCopyOffset: 4,
				BufferCopyPitch:  256,
			},
		},
	}
}

// Release releases the instance resources.
// Matches webgpu Instance.Release (7b Device.Destroy→Release precedent).
func (i *Instance) Release() {
	if i.ctx != nil {
		i.ctx.Destroy()
		i.ctx = nil
	}
	if i.x11 != nil {
		if i.x11.ctx != nil {
			i.x11.ctx.Destroy()
			i.x11.ctx = nil
		}
		i.x11 = nil
	}
}

// RequestAdapter returns the first enumerated adapter.
// Matches webgpu Instance.RequestAdapter shape; CompatibleSurface is
// forwarded as the enumerate hint (Wayland deferred enumeration).
func (i *Instance) RequestAdapter(opts *hal.RequestAdapterOptions) (hal.Adapter, error) {
	var hint hal.Surface
	if opts != nil {
		hint = opts.CompatibleSurface
	}
	adapters := i.EnumerateAdapters(hint)
	if len(adapters) == 0 {
		return nil, fmt.Errorf("gles: no adapters available")
	}
	return adapters[0].Adapter, nil
}

// ProcessEvents is a no-op for GLES (synchronous, no async callbacks).
func (i *Instance) ProcessEvents() {}
