//go:build linux

package platform

import (
	"testing"
	"unsafe"
)

// wlStatesArr builds an in-memory wl_array (struct wl_array: size@+0,
// alloc@+8, data@+16 on amd64) holding the given xdg_toplevel state enum
// values (1=maximized, 2=fullscreen, 3=resizing, 4=activated, 5..8=tiled).
// The buffer must stay alive for the duration of the wlTopConfigure call.
func wlStatesArr(states ...uint32) uintptr {
	arr := make([]uint32, len(states))
	copy(arr, states)
	buf := make([]byte, 24)
	*(*uint64)(unsafe.Pointer(&buf[0])) = uint64(len(arr) * 4)
	*(*uintptr)(unsafe.Pointer(&buf[16])) = uintptr(unsafe.Pointer(&arr[0]))
	return uintptr(unsafe.Pointer(&buf[0]))
}

// anchorWin registers a bare wlWin (nil lib → all CSD paint paths no-op)
// for direct wlTopConfigure calls.
func anchorWin(w0, h0 int) *wlWin {
	w := &wlWin{width: w0, height: h0}
	w.csd = &wlCSD{win: w, visible: true}
	w.selfPtr = uintptr(unsafe.Pointer(w))
	wlMu.Lock()
	wlByPtr[w.selfPtr] = w
	wlMu.Unlock()
	return w
}

func anchorCleanup(w *wlWin) {
	wlMu.Lock()
	delete(wlByPtr, w.selfPtr)
	wlMu.Unlock()
}

// TestWlTopConfigure_MaximizedContentHeightAnchor guards the switch-away/
// back regression: mutter alternates the maximized configure between the
// full work-area height (content + title bar, must shrink) and the content
// height itself (once the declared window geometry took effect, must NOT
// shrink again — blind subtraction shrank the window bottom by 32px).
func TestWlTopConfigure_MaximizedContentHeightAnchor(t *testing.T) {
	w := anchorWin(400, 400)
	defer anchorCleanup(w)

	// 1. First maximized configure: full work area (1868x1053) → shrink.
	wlTopConfigure(w.selfPtr, 0, 1868, 1053, wlStatesArr(1, 4, 5)) // max+act+tiled
	if w.width != 1868 || w.height != 1021 {
		t.Fatalf("initial maximize: got %dx%d, want 1868x1021 (1053 - 32 title bar)", w.width, w.height)
	}
	if w.maxContentH != 1021 {
		t.Fatalf("maxContentH=%d, want 1021", w.maxContentH)
	}

	// 2. Content-height configure (geometry took effect): must NOT shrink
	// again (regression: produced 1868x989 on focus toggle).
	w.resized = false // simulate poll() consuming the step-1 resize flag
	wlTopConfigure(w.selfPtr, 0, 1868, 1021, wlStatesArr(1, 4, 5))
	if w.width != 1868 || w.height != 1021 {
		t.Fatalf("content-height configure: got %dx%d, want 1868x1021 (keep, no double shrink)", w.width, w.height)
	}
	if w.resized {
		t.Fatalf("content-height configure must not flag resized (dedup keeps the size)")
	}
	if w.maxContentH != 1021 {
		t.Fatalf("maxContentH=%d, want 1021", w.maxContentH)
	}

	// 3. Full-work-area configure again: shrink back to the anchor.
	wlTopConfigure(w.selfPtr, 0, 1868, 1053, wlStatesArr(1, 4, 5))
	if w.width != 1868 || w.height != 1021 {
		t.Fatalf("full-work-area configure: got %dx%d, want 1868x1021", w.width, w.height)
	}

	// 4. Work-area change (different width = different monitor): re-anchor —
	// the new full-work-area height (693) must shrink to 661, not compare
	// against the stale 1021 anchor.
	wlTopConfigure(w.selfPtr, 0, 1280, 693, wlStatesArr(1, 4, 5))
	if w.width != 1280 || w.height != 661 {
		t.Fatalf("work-area change: got %dx%d, want 1280x661 (693 - 32)", w.width, w.height)
	}
	if w.maxContentH != 661 {
		t.Fatalf("maxContentH=%d, want 661 (re-anchored)", w.maxContentH)
	}

	// 5. Unmaximize: anchor resets.
	wlTopConfigure(w.selfPtr, 0, 400, 400, wlStatesArr(4)) // act only
	if w.width != 400 || w.height != 400 {
		t.Fatalf("unmaximize: got %dx%d, want 400x400", w.width, w.height)
	}
	if w.maxContentH != 0 {
		t.Fatalf("maxContentH=%d, want 0 after unmaximize", w.maxContentH)
	}
}

// TestWlTopConfigure_MaximizeRestoreNoContentHeight: maximize → restore →
// maximize again must shrink correctly on the second maximize (the anchor
// was reset by the restore configure).
func TestWlTopConfigure_MaximizeRestoreNoContentHeight(t *testing.T) {
	w := anchorWin(400, 400)
	defer anchorCleanup(w)

	wlTopConfigure(w.selfPtr, 0, 1868, 1053, wlStatesArr(1, 4, 5))
	if w.height != 1021 {
		t.Fatalf("maximize: height=%d, want 1021", w.height)
	}
	wlTopConfigure(w.selfPtr, 0, 400, 400, wlStatesArr(4))
	if w.height != 400 || w.maxContentH != 0 {
		t.Fatalf("restore: height=%d maxContentH=%d, want 400/0", w.height, w.maxContentH)
	}
	wlTopConfigure(w.selfPtr, 0, 1868, 1053, wlStatesArr(1, 4, 5))
	if w.height != 1021 {
		t.Fatalf("second maximize: height=%d, want 1021", w.height)
	}
}

// TestWlTopConfigure_MaximizedFullscreenNoShrink: fullscreen keeps the full
// configure size (no chrome shown) and resets the anchor.
func TestWlTopConfigure_MaximizedFullscreenNoShrink(t *testing.T) {
	w := anchorWin(400, 400)
	defer anchorCleanup(w)

	wlTopConfigure(w.selfPtr, 0, 1868, 1053, wlStatesArr(1, 4, 5))
	if w.height != 1021 {
		t.Fatalf("maximize: height=%d, want 1021", w.height)
	}
	// Fullscreen (state 2, no maximized): full size kept, anchor reset.
	wlTopConfigure(w.selfPtr, 0, 1920, 1080, wlStatesArr(2, 4))
	if w.height != 1080 {
		t.Fatalf("fullscreen: height=%d, want 1080 (no shrink)", w.height)
	}
	if w.maxContentH != 0 {
		t.Fatalf("maxContentH=%d, want 0 after fullscreen", w.maxContentH)
	}
}
