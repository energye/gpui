//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// Wayland CSD interaction-behavior tests (GTK4-aligned window capabilities):
// release-inside button semantics, right-click caption window menu, fixed-size
// windows showing no resize grips, and window-level pointer coordinates.
// Real-window tests: skipped with a reason when no compositor / no CSD.

// TestWaylandCSDButtonReleaseInside: the close/minimize/maximize buttons arm
// on press and fire on release-inside only (GTK4); pressing a button and
// releasing elsewhere cancels. A press on the caption (move) never leaks to
// the content layer.
func TestWaylandCSDButtonReleaseInside(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	wTop := csd.top.w
	closeX := wTop - csdButtonW
	y := float64(csdTitleBarHeight) / 2

	// Press on close, release elsewhere (over the caption) → cancelled.
	hit := csd.hitTest(csd.topSurface, float64(closeX)+10, y)
	if hit.act != csdActClose {
		t.Fatalf("close-button hit = %d, want csdActClose", hit.act)
	}
	csd.onButtonPress(w.seat, 900, hit)
	if !csd.state.Close.Pressed {
		t.Error("close button must show pressed after press")
	}
	csd.onButtonRelease(csd.topSurface, float64(closeX-100), y) // over caption
	if csd.state.Close.Pressed {
		t.Error("pressed state must clear on release")
	}
	if csd.closeRequested {
		t.Error("close must NOT fire when released outside the button (GTK4 release-inside)")
	}

	// Press on close, release on close → fires closeRequested.
	csd.onButtonPress(w.seat, 901, hit)
	csd.onButtonRelease(csd.topSurface, float64(closeX)+10, y)
	if !csd.closeRequested {
		t.Error("close must fire on release-inside (GTK4 release-inside)")
	}
	csd.closeRequested = false

	// Minimize: same release-inside contract.
	hit = csd.hitTest(csd.topSurface, float64(closeX-2*csdButtonW), y)
	if hit.act != csdActMinimize {
		t.Fatalf("minimize hit = %d, want csdActMinimize", hit.act)
	}
	csd.onButtonPress(w.seat, 902, hit)
	csd.onButtonRelease(csd.topSurface, float64(closeX-100), y) // cancel
	if w.minimized {
		t.Error("minimize must not fire on cancel")
	}
}

// TestWaylandCSDRightCaptionMenu: a right press on the caption opens the
// window menu (show_window_menu) and is consumed; right press on the content
// is forwarded (context menus belong to the app). The request marshals
// without error.
func TestWaylandCSDRightCaptionMenu(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd

	// Caption (non-button zone) right-press → consumed + menu request.
	consumed := csd.onRightPress(w.seat, 910, csd.topSurface, 100, 16)
	if !consumed {
		t.Error("right-press on caption must be consumed by the chrome")
	}
	// Button zone right-press → consumed (chrome owns the bar) but no menu.
	consumed = csd.onRightPress(w.seat, 911, csd.topSurface, float64(csd.top.w-20), 16)
	if !consumed {
		t.Error("right-press on the button zone must be consumed")
	}
	// Border surface right-press → not consumed (nothing to do).
	if csd.onRightPress(w.seat, 912, csd.left.surf, 2, 100) {
		t.Error("right-press on the border must not be consumed")
	}
	// Content right-press → not consumed (the app owns content context menus).
	if csd.onRightPress(w.seat, 913, w.surface, 100, 100) {
		t.Error("right-press on content must not be consumed")
	}
}

// TestWaylandCSDLockedNoResize: a size-locked window (SetSize min==max clamp
// or SetResizable(false)) shows no resize grips; lifting the clamp restores
// them (GTK4 parity: fixed-size windows have no resize affordances).
func TestWaylandCSDLockedNoResize(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd

	// Unlocked: the title-bar flank and border strips are resize grips.
	if hit := csd.hitTest(csd.topSurface, 2, 16); hit.act != csdActResize {
		t.Fatalf("unlocked flank hit = %d, want resize", hit.act)
	}
	if hit := csd.hitTest(csd.left.surf, 2, 100); hit.act != csdActResize {
		t.Fatalf("unlocked left border hit = %d, want resize", hit.act)
	}

	// SetSize clamps min==max → locked: no resize grips anywhere.
	ctl := win.Controls()
	ctl.SetSize(500, 400)
	if hit := csd.hitTest(csd.topSurface, 2, 16); hit.act == csdActResize {
		t.Error("locked window: flank must not be a resize grip")
	}
	if hit := csd.hitTest(csd.left.surf, 2, 100); hit.act == csdActResize {
		t.Error("locked window: border must not be a resize grip")
	}
	// Caption + buttons still work.
	if hit := csd.hitTest(csd.topSurface, 200, 16); hit.act != csdActMove {
		t.Errorf("locked window caption = %d, want move", hit.act)
	}

	// SetResizable(true) lifts the clamp → resize grips return.
	ctl.SetResizable(true)
	if hit := csd.hitTest(csd.left.surf, 2, 100); hit.act != csdActResize {
		t.Error("unlocked border must be a resize grip again")
	}
}

// TestWaylandCSDCoordTranslate: decoration-surface pointer coords translate
// into the content coordinate space (negative y over the title bar) — the
// app sees one continuous space over the whole window.
func TestWaylandCSDCoordTranslate(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	cw, ch := csd.stateW()

	// Title bar: content coords are (x - border, y - titleBarHeight) — the
	// caption sits above the content, so y is negative.
	if x, y := csd.appCoords(csd.topSurface, csdBorderThick+10, 16); x != 10 || y != 16-csdTitleBarHeight {
		t.Errorf("top translate = (%.0f,%.0f), want (%d,%d)", x, y, 10, 16-csdTitleBarHeight)
	}
	// Left border strip.
	if x, y := csd.appCoords(csd.left.surf, 2, 100); x != 2-csdBorderThick || y != 100 {
		t.Errorf("left translate = (%.0f,%.0f), want (%d,%d)", x, y, 2-csdBorderThick, 100)
	}
	// Right border strip: content x = surface x + content width.
	if x, y := csd.appCoords(csd.right.surf, 2, 100); x != float64(2+cw) || y != 100 {
		t.Errorf("right translate = (%.0f,%.0f), want (%d,%d)", x, y, 2+cw, 100)
	}
	// Bottom strip: content y = surface y + content height.
	if x, y := csd.appCoords(csd.bottom.surf, 100, 2); x != 100-csdBorderThick || y != float64(2+ch) {
		t.Errorf("bottom translate = (%.0f,%.0f), want (%d,%d)", x, y, 100-csdBorderThick, 2+ch)
	}
	// Content surface: identity.
	if x, y := csd.appCoords(w.surface, 33, 44); x != 33 || y != 44 {
		t.Errorf("content translate = (%.0f,%.0f), want (33,44)", x, y)
	}
}

// TestWaylandPointerWindowLevel: enter/leave are window-level (content + CSD
// chrome is ONE surface): crossing content↔title bar reports motion with
// translated coords, not enter/leave pairs; leaving the window reports one
// PointerLeave; a press on the content reaches the app, a press on the
// chrome never does.
func TestWaylandPointerWindowLevel(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	if w.ptr == nil {
		t.Skipf("no pointer bound on this compositor")
	}
	drain := func() {
		t.Helper()
		for {
			if evs := win.Host().WaitEvents(50 * time.Millisecond); len(evs) == 0 {
				return
			}
		}
	}
	drain()
	ptr := uintptr(unsafe.Pointer(w.ptr))
	fixed := func(v float64) uintptr { return uintptr(int32(v * 256)) }

	// Enter content from outside → PointerEnter (content coords).
	wlPtrEnterCB(ptr, w.ptr.ptr, 4000, w.surface, fixed(200), fixed(150))
	got := drainPointer(win, PointerEnter)
	if !got {
		t.Fatal("enter from outside must push PointerEnter")
	}

	// Cross content → title bar: NOT an enter/leave pair — a Move with
	// translated (negative-y) coords.
	wlPtrLeaveCB(ptr, w.ptr.ptr, 4001, w.surface)
	wlPtrEnterCB(ptr, w.ptr.ptr, 4002, w.csd.topSurface, fixed(100), fixed(16))
	evs := win.Host().WaitEvents(100 * time.Millisecond)
	for _, ev := range evs {
		if ev.Type == EventPointer && (ev.Pointer == PointerEnter || ev.Pointer == PointerLeave) {
			t.Errorf("internal crossing must not push enter/leave, got %v", ev.Pointer)
		}
		if ev.Type == EventPointer && ev.Pointer == PointerMove {
			if ev.Y >= 0 {
				t.Errorf("title-bar move y = %.0f, want < 0 (translated coords)", ev.Y)
			}
		}
	}

	// Leave the window → PointerLeave (resolved via the foreign enter).
	wlPtrLeaveCB(ptr, w.ptr.ptr, 4003, w.csd.topSurface)
	wlPtrEnterCB(ptr, w.ptr.ptr, 4004, 0x12345, fixed(0), fixed(0)) // foreign surface
	if !drainPointer(win, PointerLeave) {
		t.Error("window leave must push PointerLeave")
	}
}

// drainPointer waits up to ~2s for a pointer event of kind k.
func drainPointer(win *Window, k PointerKind) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, ev := range win.Host().WaitEvents(100 * time.Millisecond) {
			if ev.Type == EventPointer && ev.Pointer == k {
				return true
			}
		}
	}
	return false
}
