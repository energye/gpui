//go:build linux

package platform

import (
	"testing"
	"unsafe"
)

// TestWaylandCSDHoverEnter is a regression test for the CSD hover SIGSEGV
// (mouse entering the title bar / borders crashed in native code with
// addr=0x21/0x35/0xa0). Root cause: applyCursorImage dereferenced
// wl_cursor.images once instead of twice (images is array of pointers), so
// the attached wl_buffer was garbage and wl_proxy_marshal_array_flags
// crashed. Driving the enter/motion callbacks in-process with a fake serial
// reproduces the exact pointer path without a real mouse.
func TestWaylandCSDHoverEnter(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	if w.ptr == nil {
		t.Skipf("no pointer bound on this compositor")
	}

	// Title-bar caption (non-button zone): hit = csdActMove.
	enter := func(surface uintptr, x, y float64) {
		t.Helper()
		serial := uintptr(1000)
		wlPtrEnterCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, serial, surface,
			uintptr(int32(x*256)), uintptr(int32(y*256)))
	}
	// Entering plain content (the window interior) must NOT arm a cursor:
	// setCursor must never hand the compositor an empty cursor surface (that
	// makes the pointer invisible — the "no mouse in window" regression).
	enter(w.surface, 200, 150)
	if w.csd.curName != "" {
		t.Fatalf("content enter armed cursor name %q — set_cursor must stay untouched", w.csd.curName)
	}

	enter(w.csd.topSurface, 500, 16)  // middle of the caption
	enter(w.csd.topSurface, 960, 16)  // near the buttons (but not on them)
	enter(w.csd.left.surf, 2, 100)    // left border
	enter(w.csd.right.surf, 2, 100)   // right border
	enter(w.csd.bottom.surf, 300, 2)  // bottom border
	enter(w.csd.topSurface, 979, 16)  // minimize button
	enter(w.csd.topSurface, 1024, 16) // maximize button
	enter(w.csd.topSurface, 1068, 16) // close button
	// Leave + re-enter (hover clear path).
	wlPtrLeaveCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1001, w.csd.topSurface)
	enter(w.csd.topSurface, 500, 16)
}
