//go:build linux && !(js && wasm)

package egl

import "testing"

// TestDisplayRef_Lifecycle pins the C7 fix: Wayland returns a distinct
// EGLDisplay handle for each per-surface context even though they all share
// one wl_display. Lifetime is keyed by the native display, and only the first
// handle is terminated once, when the last context releases it; the native
// connection owner is closed in the same step.
func TestDisplayRef_Lifecycle(t *testing.T) {
	oldTerminate, oldClose := terminateDisplayFn, closeDisplayOwnerFn
	t.Cleanup(func() {
		terminateDisplayFn, closeDisplayOwnerFn = oldTerminate, oldClose
	})
	var terminations int
	var terminatedAt EGLDisplay
	terminateDisplayFn = func(d EGLDisplay) EGLBoolean {
		terminations++
		terminatedAt = d
		return True
	}
	var ownerCloses int
	closeDisplayOwnerFn = func(*DisplayOwner) { ownerCloses++ }

	key := displayKey{kind: WindowKindX11, native: 0xc7c7c7}
	first := EGLDisplay(0x1001)
	second := EGLDisplay(0x1002)
	owner := &DisplayOwner{}
	retainDisplay(key, first, owner)
	retainDisplay(key, second, nil) // same native display, different EGL handle
	retainDisplay(key, first, nil)

	releaseDisplay(key)
	if terminations != 0 || ownerCloses != 0 {
		t.Fatalf("first release must keep display alive: terminate=%d close=%d", terminations, ownerCloses)
	}
	releaseDisplay(key)
	if terminations != 0 || ownerCloses != 0 {
		t.Fatalf("second release must keep display alive: terminate=%d close=%d", terminations, ownerCloses)
	}
	releaseDisplay(key)
	if terminations != 1 || ownerCloses != 1 {
		t.Fatalf("last release must terminate once and close owner: terminate=%d close=%d", terminations, ownerCloses)
	}
	if terminatedAt != first {
		t.Fatalf("must terminate the first registered handle %#x, got %#x", first, terminatedAt)
	}
	// Unknown / already released keys are a no-op (double Destroy paths).
	releaseDisplay(key)
	if terminations != 1 || ownerCloses != 1 {
		t.Fatalf("released key must be a no-op: terminate=%d close=%d", terminations, ownerCloses)
	}
}

func TestDisplayRef_ContextDestroyReleasesOnce(t *testing.T) {
	oldTerminate, oldClose := terminateDisplayFn, closeDisplayOwnerFn
	t.Cleanup(func() {
		terminateDisplayFn, closeDisplayOwnerFn = oldTerminate, oldClose
	})
	var terminations int
	terminateDisplayFn = func(EGLDisplay) EGLBoolean {
		terminations++
		return True
	}
	closeDisplayOwnerFn = func(*DisplayOwner) {}

	key := displayKey{kind: WindowKindX11, native: 0x4d5e6f}
	display := EGLDisplay(0x2001)
	retainDisplay(key, display, nil)
	ctx := &Context{display: display, displayKey: key}
	ctx.Destroy()
	ctx.Destroy() // idempotent: display already NoDisplay
	if terminations != 1 {
		t.Fatalf("Context.Destroy must release the native display exactly once, got %d", terminations)
	}
}

// TestDisplayRef_WaylandBorrowedDisplayIsNotTerminated: a Wayland EGL display
// is borrowed from the host's wl_display. The host may close that connection
// before the last GPU context is released; terminating it then crashes inside
// Mesa. The refcount still groups the native display, but the final release
// must only destroy contexts (already handled by Context.Destroy), not
// eglTerminate the borrowed display.
func TestDisplayRef_WaylandBorrowedDisplayIsNotTerminated(t *testing.T) {
	oldTerminate, oldClose := terminateDisplayFn, closeDisplayOwnerFn
	t.Cleanup(func() {
		terminateDisplayFn, closeDisplayOwnerFn = oldTerminate, oldClose
	})
	var terminations, ownerCloses int
	terminateDisplayFn = func(EGLDisplay) EGLBoolean {
		terminations++
		return True
	}
	closeDisplayOwnerFn = func(*DisplayOwner) { ownerCloses++ }

	key := displayKey{kind: WindowKindWayland, native: 0x777777}
	retainDisplay(key, EGLDisplay(0x3001), nil)
	retainDisplay(key, EGLDisplay(0x3002), nil)
	releaseDisplay(key)
	releaseDisplay(key)
	if terminations != 0 || ownerCloses != 0 {
		t.Fatalf("borrowed Wayland display must not terminate/close: terminate=%d close=%d", terminations, ownerCloses)
	}
}
