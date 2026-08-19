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
	// Entering plain content (the window interior) re-applies the default
	// arrow. mutter treats set_cursor(NULL) as "hide the pointer" (empty
	// surface = invisible), so content enter must land on the theme's
	// "left_ptr" — the "no mouse in window" regression.
	enter(w.surface, 200, 150)
	if w.csd.curName != "left_ptr" {
		t.Fatalf("content enter cursor = %q, want left_ptr (pointer must stay visible)", w.csd.curName)
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

// TestWaylandCSDHoverDirectButtonEnter is a regression test for "moving the
// mouse straight onto a title-bar button from OUTSIDE the window shows no
// hover feedback; only entering from another spot inside the window works".
// The compositor reports the direct entry as a window enter whose surface is
// the title-bar subsurface (not the content surface), and onHover must set
// the button state in that path exactly like the internal-crossing path.
func TestWaylandCSDHoverDirectButtonEnter(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	if w.ptr == nil {
		t.Skipf("no pointer bound on this compositor")
	}
	csd := w.csd
	if csd.top == nil || csd.top.w < csdButtonW*3 {
		t.Skipf("title bar too narrow for three buttons: w=%d", csd.top.w)
	}
	topW := float64(csd.top.w)
	closeX := topW - csdButtonW
	maxX := closeX - csdButtonW
	minX := maxX - csdButtonW

	enter := func(surface uintptr, x, y float64) {
		t.Helper()
		wlPtrEnterCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1000, surface,
			uintptr(int32(x*256)), uintptr(int32(y*256)))
	}
	hovered := func() bool {
		return csd.state.Close.Hovered || csd.state.Maximize.Hovered || csd.state.Minimize.Hovered
	}

	// 1. Direct entry from outside the window straight onto the CLOSE
	// button (no preceding leave — the pointer was never over the window).
	enter(csd.topSurface, closeX+10, 16)
	if !csd.state.Close.Hovered || hovered() != (csd.state.Close.Hovered) {
		t.Fatalf("direct close-button enter: Close.Hovered=%v want true (hovered=%v)",
			csd.state.Close.Hovered, hovered())
	}
	if csd.state.Minimize.Hovered || csd.state.Maximize.Hovered {
		t.Fatalf("direct close enter armed wrong button min=%v max=%v",
			csd.state.Minimize.Hovered, csd.state.Maximize.Hovered)
	}

	// 2. Direct entry onto MINIMIZE from outside — state must switch.
	enter(csd.topSurface, minX+10, 16)
	if !csd.state.Minimize.Hovered || csd.state.Close.Hovered || csd.state.Maximize.Hovered {
		t.Fatalf("direct minimize enter: min=%v close=%v max=%v",
			csd.state.Minimize.Hovered, csd.state.Close.Hovered, csd.state.Maximize.Hovered)
	}

	// 3. Direct entry onto MAXIMIZE.
	enter(csd.topSurface, maxX+10, 16)
	if !csd.state.Maximize.Hovered || csd.state.Close.Hovered || csd.state.Minimize.Hovered {
		t.Fatalf("direct maximize enter: max=%v close=%v min=%v",
			csd.state.Maximize.Hovered, csd.state.Close.Hovered, csd.state.Minimize.Hovered)
	}

	// 4. Internal crossing path (content -> button) still works: the
	// "from another spot inside the window" case the user reports as OK.
	enter(w.surface, 200, 150)
	enter(csd.topSurface, closeX+10, 16)
	if !csd.state.Close.Hovered {
		t.Fatalf("crossing close-button enter: Close.Hovered=%v want true", csd.state.Close.Hovered)
	}

	// 5. Leaving the window clears all hover.
	wlPtrLeaveCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1001, csd.topSurface)
	wlPtrEnterCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1002, 0, 0, 0) // foreign surface
	if hovered() {
		t.Fatalf("hover not cleared after leave: %+v", csd.state)
	}
}

// TestWaylandCSDHoverBetweenButtonsMotion: moving the pointer BETWEEN the
// title-bar buttons (minimize → maximize → close, no leave/enter) must move
// the hover highlight. Regression for "no hover effect when entering the
// buttons from another button; only entering from outside works": the old
// repaint condition compared any-hover-vs-any-hover, so button→button moves
// (still "some button hovered") never repainted and the highlight stayed
// stuck on the previous button. Pixel-verified on the real compositor.
func TestWaylandCSDHoverBetweenButtonsMotion(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	if w.ptr == nil {
		t.Skipf("no pointer bound on this compositor")
	}
	csd := w.csd
	if csd.top == nil || csd.top.data == nil || csd.top.w < csdButtonW*3 {
		t.Skipf("title bar too narrow or no shm buffer")
	}
	csd.state.Focused = true
	csd.repaintTitle()

	topW := csd.top.w
	minX := topW - 3*csdButtonW
	maxX := topW - 2*csdButtonW
	closeX := topW - csdButtonW
	cy := 3 // just inside the button's top-left corner: glyph-free fill area
	pxAt := func(btnX int) [4]byte {
		off := (cy*topW + (btnX + 3)) * 4
		return [4]byte{csd.top.data[off], csd.top.data[off+1], csd.top.data[off+2], csd.top.data[off+3]}
	}
	enter := func(surface uintptr, x, y float64) {
		t.Helper()
		wlPtrEnterCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1000, surface,
			uintptr(int32(x*256)), uintptr(int32(y*256)))
	}
	motion := func(x, y float64) {
		t.Helper()
		wlPtrMotionCB(uintptr(unsafe.Pointer(w.ptr)), w.ptr.ptr, 1001,
			uintptr(int32(x*256)), uintptr(int32(y*256)))
	}

	// Enter directly on MINIMIZE → its button fills with the hover color.
	enter(csd.topSurface, float64(minX+10), 16)
	if !csd.state.Minimize.Hovered {
		t.Fatalf("enter minimize: Minimize.Hovered=%v want true", csd.state.Minimize.Hovered)
	}
	if got := pxAt(minX); got != csdColorBtnHover {
		t.Fatalf("minimize hover pixel = %v, want %v", got, csdColorBtnHover)
	}

	// Motion to MAXIMIZE (button→button, no leave/enter pair): the
	// highlight must follow — minimize clears, maximize fills.
	motion(float64(maxX+10), 16)
	if csd.state.Minimize.Hovered || !csd.state.Maximize.Hovered {
		t.Fatalf("motion to maximize: min=%v max=%v", csd.state.Minimize.Hovered, csd.state.Maximize.Hovered)
	}
	if got := pxAt(minX); got != csdColorBgFocus {
		t.Fatalf("minimize pixel after motion = %v, want cleared bg %v", got, csdColorBgFocus)
	}
	if got := pxAt(maxX); got != csdColorBtnHover {
		t.Fatalf("maximize pixel after motion = %v, want hover %v", got, csdColorBtnHover)
	}

	// And on to CLOSE (uses the red close-hover fill).
	motion(float64(closeX+10), 16)
	if csd.state.Maximize.Hovered || !csd.state.Close.Hovered {
		t.Fatalf("motion to close: max=%v close=%v", csd.state.Maximize.Hovered, csd.state.Close.Hovered)
	}
	if got := pxAt(maxX); got != csdColorBgFocus {
		t.Fatalf("maximize pixel after motion = %v, want cleared bg %v", got, csdColorBgFocus)
	}
	if got := pxAt(closeX); got != csdColorCloseHover {
		t.Fatalf("close pixel after motion = %v, want close-hover %v", got, csdColorCloseHover)
	}
}

// TestWaylandCSDHoverRepaintsButton verifies on a REAL compositor window
// that setting a button hover state + repaintTitle actually changes the
// title-bar pixels (the hover visual the user sees). Reads the shm buffer
// the title-bar subsurface attaches — the same buffer the compositor
// samples — so it proves the whole hover chain (hit → state → repaint →
// pixels) without needing a physical pointer.
func TestWaylandCSDHoverRepaintsButton(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	if csd.top == nil || csd.top.data == nil || csd.top.w < csdButtonW*3 {
		t.Skipf("title bar too narrow or no shm buffer")
	}
	// Force focused + baseline repaint so the initial pixels are the focus
	// background, not whatever the compositor focus state happens to be.
	csd.state.Focused = true
	csd.repaintTitle()

	topW := csd.top.w
	// Sample just inside the button's top-left corner: the glyph (X / bar /
	// square) is centered, so the corner pixels are background/hover fill
	// only — the glyph would mask the fill change.
	closeX := topW - csdButtonW
	cx := closeX + 3
	cy := 3
	off := (cy*topW + cx) * 4
	px := func() [4]byte {
		return [4]byte{csd.top.data[off], csd.top.data[off+1], csd.top.data[off+2], csd.top.data[off+3]}
	}

	if got := px(); got != csdColorBgFocus {
		t.Fatalf("baseline close-button pixel = %v, want focus bg %v", got, csdColorBgFocus)
	}

	// Hover the close button → the button background must switch to the
	// close-hover color in the very buffer the compositor samples.
	csd.state.Close.Hovered = true
	csd.repaintTitle()
	if got := px(); got != csdColorCloseHover {
		t.Fatalf("hover close-button pixel = %v, want close-hover %v", got, csdColorCloseHover)
	}

	// Hover cleared → back to background.
	csd.state.Close.Hovered = false
	csd.repaintTitle()
	if got := px(); got != csdColorBgFocus {
		t.Fatalf("unhover close-button pixel = %v, want focus bg %v", got, csdColorBgFocus)
	}

	// Minimize hover uses the neutral button hover color.
	csd.state.Minimize.Hovered = true
	csd.repaintTitle()
	minX := topW - 3*csdButtonW
	minOff := (cy*topW + (minX + 3)) * 4
	if got := [4]byte{csd.top.data[minOff], csd.top.data[minOff+1], csd.top.data[minOff+2], csd.top.data[minOff+3]}; got != csdColorBtnHover {
		t.Fatalf("hover minimize-button pixel = %v, want btn-hover %v", got, csdColorBtnHover)
	}
}
