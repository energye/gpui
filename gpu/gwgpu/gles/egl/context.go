// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build linux && !(js && wasm)

package egl

import (
	"fmt"
	"strings"
	"unsafe"
)

// Context wraps an EGL rendering context with its display, config, and surface.
type Context struct {
	display      EGLDisplay
	config       EGLConfig
	context      EGLContext
	pbuffer      EGLSurface
	windowKind   WindowKind
	displayOwner *DisplayOwner // owns native display connection (X11); closed after eglTerminate
}

// ContextConfig holds configuration options for creating an EGL context.
type ContextConfig struct {
	// GLVersionMajor is the major OpenGL version (e.g., 3 for OpenGL 3.3).
	GLVersionMajor int
	// GLVersionMinor is the minor OpenGL version (e.g., 3 for OpenGL 3.3).
	GLVersionMinor int
	// CoreProfile requests a core profile context (vs compatibility).
	CoreProfile bool
	// Debug enables debug context with validation.
	Debug bool
	// GLES requests OpenGL ES instead of desktop OpenGL.
	GLES bool
	// Surfaceless creates a context without a surface (headless rendering).
	Surfaceless bool
	// NativeDisplay is the native display handle to use for EGL display creation.
	// On Wayland: must be the app's wl_display* — passing 0 causes EGL to open
	// a second connection, which makes wl_surface proxies mismatched on configure.
	// On X11: the X11 Display*. Zero uses the default display.
	NativeDisplay uintptr
	// AllowSoftwareConfigs lifts the P2-0 hardware-caveat filter: configs
	// marked SLOW/NON_CONFORMANT become eligible. Default false (a software
	// rasterizer must never shadow a real GPU on the shared display); set it
	// only when the caller explicitly picked a software device display
	// (P2-2 ForceFallback materialize), where slow is the honest answer.
	AllowSoftwareConfigs bool
	// WindowKind selects the native window system explicitly. Nil preserves
	// automatic environment-based detection and keeps ContextConfig's zero value
	// independent of WindowKind's numeric constants.
	WindowKind *WindowKind
}

// DefaultContextConfig returns a sensible default context configuration.
// Creates an OpenGL 3.3 core profile context.
func DefaultContextConfig() ContextConfig {
	return ContextConfig{
		GLVersionMajor: 3,
		GLVersionMinor: 3,
		CoreProfile:    true,
		Debug:          false,
		GLES:           false,
		Surfaceless:    false,
	}
}

// NewContext creates a new EGL context. An explicit WindowKind selects that
// platform directly; otherwise it detects X11, Wayland, or Surfaceless.
//
// PRIME note: EGL on X11 follows DRI_PRIME for the render node. Callers that
// want the discrete GPU set DRI_PRIME=1 in the process env before the first
// EGL display init (Mesa resolves the node at display bring-up, not per
// context). No EGL code change needed here; selection lives one layer up
// (adapter policy), this comment only pins the mechanism.
func NewContext(config ContextConfig) (*Context, error) {
	// Get an EGL display for the explicit or detected platform.
	// displayOwner (non-nil for X11) keeps the native display connection alive.
	windowKind := selectWindowKind(config.WindowKind, DetectWindowKind)
	display, windowKind, displayOwner, err := getEGLDisplayForKind(config.NativeDisplay, windowKind)
	if err != nil {
		return nil, fmt.Errorf("failed to get EGL display: %w", err)
	}
	return newContextOnDisplay(display, windowKind, displayOwner, config)
}

// NewContextOnDisplay creates an EGL context on an already-opened EGL display
// (e.g. PlatformDisplayForDevice for a picked GPU from device enumeration).
// Takes over the display: Destroy terminates it, and error paths terminate
// it too — same ownership as NewContext. The display is reported as
// surfaceless: device displays have no window system, so the context must
// never be mistaken for a shareable window context (P2-3 owns that split).
func NewContextOnDisplay(display EGLDisplay, config ContextConfig) (*Context, error) {
	if display == NoDisplay {
		return nil, fmt.Errorf("no display")
	}
	return newContextOnDisplay(display, WindowKindSurfaceless, nil, config)
}

// newContextOnDisplay runs the shared Initialize→BindAPI→chooseConfig→
// createContext→pbuffer flow on a known display.
func newContextOnDisplay(display EGLDisplay, windowKind WindowKind, displayOwner *DisplayOwner, config ContextConfig) (*Context, error) {

	// closeOwner is a helper to close the display owner on error paths.
	closeOwner := func() {
		if displayOwner != nil {
			displayOwner.Close()
		}
	}

	// Initialize EGL
	var major, minor EGLInt
	if Initialize(display, &major, &minor) == False {
		closeOwner()
		return nil, fmt.Errorf("eglInitialize failed: error 0x%x", GetError())
	}

	// Bind OpenGL or OpenGL ES API
	api := OpenGLAPI
	if config.GLES {
		api = OpenGLESAPI
	}
	if BindAPI(api) == False {
		Terminate(display)
		closeOwner()
		return nil, fmt.Errorf("eglBindAPI failed: error 0x%x", GetError())
	}

	// Choose EGL frame buffer configuration
	eglConfig, err := chooseEGLConfig(display, config)
	if err != nil {
		Terminate(display)
		closeOwner()
		return nil, fmt.Errorf("failed to choose EGL config: %w", err)
	}

	// Create EGL context
	eglContext := createEGLContext(display, eglConfig, config)
	if eglContext == NoContext {
		Terminate(display)
		closeOwner()
		return nil, fmt.Errorf("eglCreateContext failed: error 0x%x", GetError())
	}

	// Surfaceless context: EGL 1.5+ or EGL_KHR_surfaceless_context allows
	// MakeCurrent with EGL_NO_SURFACE. Skip pbuffer creation in that case.
	// Fallback to 1×1 pbuffer for older drivers.
	// Matches Rust wgpu-hal egl.rs:735-758.
	hasSurfaceless := (major > 1 || (major == 1 && minor >= 5))
	if !hasSurfaceless {
		displayExts := QueryString(display, Extensions)
		hasSurfaceless = strings.Contains(displayExts, "EGL_KHR_surfaceless_context")
	}

	var pbuffer EGLSurface
	if hasSurfaceless {
		pbuffer = NoSurface
	} else {
		pbuffer = createPbufferSurface(display, eglConfig)
		if pbuffer == NoSurface {
			DestroyContext(display, eglContext)
			Terminate(display)
			closeOwner()
			return nil, fmt.Errorf("eglCreatePbufferSurface failed and no surfaceless support: error 0x%x", GetError())
		}
	}

	return &Context{
		display:      display,
		config:       eglConfig,
		context:      eglContext,
		pbuffer:      pbuffer,
		windowKind:   windowKind,
		displayOwner: displayOwner,
	}, nil
}

func selectWindowKind(requested *WindowKind, detect func() WindowKind) WindowKind {
	if requested != nil {
		return *requested
	}
	return detect()
}

// chooseEGLConfig selects an appropriate EGL frame buffer configuration.
func chooseEGLConfig(display EGLDisplay, config ContextConfig) (EGLConfig, error) {
	// Determine renderable type
	var renderableType EGLInt
	if config.GLES {
		switch {
		case config.GLVersionMajor >= 3:
			renderableType = OpenGLES3Bit
		case config.GLVersionMajor >= 2:
			renderableType = OpenGLES2Bit
		default:
			renderableType = OpenGLESBit
		}
	} else {
		renderableType = OpenGLBit
	}

	// Tiered config selection (Rust wgpu-hal egl.rs:218-293).
	// Rust's top tier = WindowBit alone (never combined with PbufferBit).
	// Mesa Wayland EGL does NOT support PbufferBit — any tier requiring it
	// returns 0 configs. WindowBit alone = 48 configs on Mesa Wayland.
	// Caveat: prefer hardware configs first (skia Ganesh GR_GL_CONFIG..
	// style: SLOW_CONFIG means software/less-direct). Tiers that only
	// yield EGL_SLOW_CONFIG fall through to the next tier, so a software
	// rasterizer never shadows a real GPU (PRIME dGPU vs llvmpipe).
	tiers := []EGLInt{
		WindowBit | PbufferBit, // Tier 2: X11/headless (window + pbuffer)
		WindowBit,              // Tier 1: Wayland (no pbuffer support in Mesa)
		PbufferBit,             // Tier 0: surfaceless/CI fallback
	}

	baseAttribs := []EGLInt{
		RenderableType, renderableType,
		RedSize, 8,
		GreenSize, 8,
		BlueSize, 8,
		AlphaSize, 8,
		DepthSize, 24,
		StencilSize, 8,
	}

	for _, surfaceType := range tiers {
		attribs := make([]EGLInt, 0, len(baseAttribs)+5)
		attribs = append(attribs, SurfaceType, surfaceType)
		attribs = append(attribs, baseAttribs...)
		// Prefer hardware rasterization: skip tiers whose best config is
		// EGL_SLOW_CONFIG (software rasterizer), so llvmpipe never shadows
		// a real GPU on hybrid boxes (PRIME dGPU / iGPU present).
		// Hard-fail the tier instead of DontCare filtering here: probing a
		// cold eglChooseConfig on some drivers (observed: NVIDIA 580 X11,
		// uninitialized display) returns 0 configs with 0x3001 for ANY
		// attribute set, so filtering must not turn a driver quirk into
		// "no configs". The caveat check below inspects the winner.
		// AllowSoftwareConfigs (P2-2 software-device path only) skips the
		// winner check: on an explicitly picked software display, slow is
		// the honest answer, not a shadow.
		attribs = append(attribs, ConfigCaveat, DontCare)
		attribs = append(attribs, None)

		var eglConfig EGLConfig
		var numConfigs EGLInt
		if ChooseConfig(display, &attribs[0], &eglConfig, 1, &numConfigs) == False {
			continue
		}
		if numConfigs > 0 && (config.AllowSoftwareConfigs || configCaveatIsHardware(display, eglConfig)) {
			return eglConfig, nil
		}
	}

	return 0, fmt.Errorf("no suitable EGL configs found (tried window+pbuffer and pbuffer-only)")
}

// configCaveatIsHardware reports whether a config is hardware-accelerated
// (caveat is neither SLOW nor NON_CONFORMANT). Query failures fail open:
// rejecting a config we cannot inspect is worse than trying it.
func configCaveatIsHardware(display EGLDisplay, config EGLConfig) bool {
	var caveat EGLInt
	if GetConfigAttrib(display, config, ConfigCaveat, &caveat) == False {
		return true
	}
	return caveat != SlowConfig && caveat != NonConformantConfig
}

// createEGLContext creates an EGL rendering context.
func createEGLContext(display EGLDisplay, config EGLConfig, cfg ContextConfig) EGLContext {
	var attribs []EGLInt

	// Set OpenGL version
	attribs = append(attribs,
		ContextMajorVersion, EGLInt(cfg.GLVersionMajor),
		ContextMinorVersion, EGLInt(cfg.GLVersionMinor),
	)

	// Set profile (core vs compatibility) — desktop OpenGL only.
	// EGL_CONTEXT_OPENGL_PROFILE_MASK is invalid for GLES; some drivers reject it.
	if cfg.CoreProfile && !cfg.GLES {
		attribs = append(attribs,
			ContextOpenGLProfileMask, ContextOpenGLCoreProfileBit,
		)
	}

	// Enable debug context if requested
	if cfg.Debug {
		attribs = append(attribs,
			ContextFlagsKHR, ContextOpenGLDebugBitKHR,
		)
	}

	// Terminate attribute list
	attribs = append(attribs, None)

	return CreateContext(display, config, NoContext, &attribs[0])
}

// createPbufferSurface creates a minimal pbuffer surface for the context.
func createPbufferSurface(display EGLDisplay, config EGLConfig) EGLSurface {
	attribs := []EGLInt{
		Width, 16,
		Height, 16,
		None,
	}
	return CreatePbufferSurface(display, config, &attribs[0])
}

// MakeCurrent makes this context current on the pbuffer (headless rendering).
func (c *Context) MakeCurrent() error {
	if MakeCurrent(c.display, c.pbuffer, c.pbuffer, c.context) == False {
		return fmt.Errorf("eglMakeCurrent failed: error 0x%x", GetError())
	}
	return nil
}

// MakeCurrentSurface makes this context current on a window surface (for Present).
func (c *Context) MakeCurrentSurface(surface EGLSurface) error {
	if MakeCurrent(c.display, surface, surface, c.context) == False {
		return fmt.Errorf("eglMakeCurrent(surface) failed: error 0x%x", GetError())
	}
	return nil
}

// CreateWindowSurface creates an EGL window surface for presentation.
// The surface shares this context's display and config.
func (c *Context) CreateWindowSurface(nativeWindow uintptr) (EGLSurface, error) {
	attribs := []EGLInt{None}
	surface := CreateWindowSurface(c.display, c.config, EGLNativeWindowType(nativeWindow), &attribs[0])
	if surface == NoSurface {
		return NoSurface, fmt.Errorf("eglCreateWindowSurface failed: error 0x%x", GetError())
	}
	return surface, nil
}

// Display returns the EGL display handle.
func (c *Context) Display() EGLDisplay { return c.display }

// Config returns the EGL config handle.
func (c *Context) Config() EGLConfig { return c.config }

// Destroy releases the context and its associated resources.
// Order matters: EGL resources first, then the native display connection.
// Closing the native display (e.g. XCloseDisplay) before eglTerminate
// would cause EGL to access a freed connection.
func (c *Context) Destroy() {
	if c.context != NoContext {
		// Unbind context first
		_ = MakeCurrent(c.display, NoSurface, NoSurface, NoContext)
		DestroyContext(c.display, c.context)
		c.context = NoContext
	}
	if c.pbuffer != NoSurface {
		DestroySurface(c.display, c.pbuffer)
		c.pbuffer = NoSurface
	}
	if c.display != NoDisplay {
		Terminate(c.display)
		c.display = NoDisplay
	}
	// Close native display connection AFTER eglTerminate.
	// For X11 this calls XCloseDisplay; for other platforms displayOwner is nil.
	if c.displayOwner != nil {
		c.displayOwner.Close()
		c.displayOwner = nil
	}
}

// EGLContext returns the EGL context handle.
func (c *Context) EGLContext() EGLContext {
	return c.context
}

// Pbuffer returns the pbuffer surface.
func (c *Context) Pbuffer() EGLSurface {
	return c.pbuffer
}

// WindowKind returns the detected window system type.
func (c *Context) WindowKind() WindowKind {
	return c.windowKind
}

// GetGLProcAddress returns the address of an OpenGL function.
// It uses eglGetProcAddress to load both core and extension functions.
// Returns unsafe.Pointer for the GL entry-point table.
func GetGLProcAddress(name string) unsafe.Pointer {
	return u2p(GetProcAddress(name))
}
