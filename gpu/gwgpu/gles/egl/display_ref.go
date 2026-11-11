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

package egl

import "sync"

// EGLDisplay handles are not stable per native display: Mesa returns a
// distinct handle for each eglGetPlatformDisplay call, even when every call
// uses the same wl_display*. Terminating each handle independently invalidates
// the shared native display behind the others (the C7 close-time SIGSEGV:
// 8 per-surface Wayland contexts → 8 handles → 8 eglTerminate calls). Key the
// lifetime by the native display instead; for X11/headless keep the first
// handle and terminate it once at the last release. Wayland's display is
// borrowed from the host, so it is never terminated here (see releaseDisplay).
// The owner (when EGL created the native X11 connection) is closed in the same
// last-release step.
type displayKey struct {
	kind   WindowKind
	native uintptr
}

type displayRef struct {
	refs    int
	owner   *DisplayOwner
	display EGLDisplay // first handle registered for this native display
}

var (
	displayRefMu sync.Mutex
	displayRefs  = map[displayKey]*displayRef{}

	// Test seams: keep the refcount unit-testable without a real EGL display.
	terminateDisplayFn  = Terminate
	closeDisplayOwnerFn = func(o *DisplayOwner) { o.Close() }
)

// retainDisplay adds one context claim on the native display. display is the
// context's EGL handle (only the first is kept for the final terminate);
// owner may be nil for borrowed native displays (Wayland) or supplied X11
// displays.
func retainDisplay(key displayKey, display EGLDisplay, owner *DisplayOwner) {
	displayRefMu.Lock()
	ref := displayRefs[key]
	if ref == nil {
		ref = &displayRef{display: display}
		displayRefs[key] = ref
	}
	ref.refs++
	if owner != nil && ref.owner == nil {
		ref.owner = owner
	}
	displayRefMu.Unlock()
}

// releaseDisplay drops one context claim. The last claim terminates the first
// EGLDisplay handle registered for the native display (X11/headless) and
// closes the native connection owner (if any); a borrowed Wayland display is
// left to the host/process exit. Unknown keys are a no-op: Destroy may run
// twice on partial teardown paths.
func releaseDisplay(key displayKey) {
	displayRefMu.Lock()
	ref := displayRefs[key]
	if ref == nil {
		displayRefMu.Unlock()
		return
	}
	ref.refs--
	if ref.refs > 0 {
		displayRefMu.Unlock()
		return
	}
	delete(displayRefs, key)
	display, owner := ref.display, ref.owner
	displayRefMu.Unlock()

	if key.kind == WindowKindWayland {
		// Wayland's EGL display is borrowed from the host's wl_display. The
		// host may close that connection before the last GPU context is
		// released; eglTerminate then walks freed Mesa state (C7 close-time
		// SIGSEGV). Contexts/pbuffers are already destroyed above; leave the
		// EGL display to the host/process exit instead of terminating a
		// borrowed connection we no longer own.
		return
	}
	terminateDisplayFn(display)
	if owner != nil {
		closeDisplayOwnerFn(owner)
	}
}
