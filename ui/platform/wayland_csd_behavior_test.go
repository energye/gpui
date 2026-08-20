//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// TestWaylandCSDResizeFillsBeforeCommit: resizeSurface must swap the shm
// buffer WITHOUT an empty gap — the new buffer is filled with the title-bar
// content BEFORE it is attached/committed, and the old buffer/pool survive
// until the new one is committed. Regression for "title-bar text blinks and
// jumps position while resizing": the old order detached (attach NULL) and
// destroyed the old buffer first, leaving the subsurface without a buffer
// (applied immediately, desync) until paint() re-committed.
func TestWaylandCSDResizeFillsBeforeCommit(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	top := csd.top
	if top == nil || top.buffer == 0 || top.data == nil {
		t.Skipf("CSD top surface not initialized")
	}
	oldBuf := top.buffer

	csd.resize(800, 600)

	if top.w != 800 || top.h != csdTitleBarHeight {
		t.Fatalf("top after resize = %dx%d, want 800x%d", top.w, top.h, csdTitleBarHeight)
	}
	if top.buffer == 0 || top.data == nil {
		t.Fatalf("resize lost the top buffer/data")
	}
	if top.buffer == oldBuf {
		t.Fatalf("resize must allocate a NEW buffer, still holding the old one")
	}
	// The new buffer must already carry the drawn title bar (not blank/garbage
	// at commit time): the title glyphs are bright (#DFE1E5) on the #2B2D30 bg.
	found := false
	for y := 0; y < top.h && !found; y++ {
		for x := 0; x < top.w; x++ {
			o := (y*top.w + x) * 4
			b, g, r, a := top.data[o], top.data[o+1], top.data[o+2], top.data[o+3]
			if a == 0xFF && r > 180 && g > 180 && b > 180 {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("top buffer after resize has no drawn title glyphs (blank before commit?)")
	}
}

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
// app sees one continuous space over the whole window. Layout: top at
// (0,-32), left/right at (-4,-32)/(cw,-32), bottom at (-4,ch).
func TestWaylandCSDCoordTranslate(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	cw, ch := csd.stateW()

	// Title bar: content coords are (x, y - titleBarHeight) — the caption
	// sits above the content, so y is negative.
	if x, y := csd.appCoords(csd.topSurface, 10, 16); x != 10 || y != 16-csdTitleBarHeight {
		t.Errorf("top translate = (%.0f,%.0f), want (%d,%d)", x, y, 10, 16-csdTitleBarHeight)
	}
	// Left border strip: content coords = (x - border, y - titleBarHeight)
	// — the border now spans the full window height including the bar.
	if x, y := csd.appCoords(csd.left.surf, 2, 100); x != 2-csdBorderThick || y != 100-csdTitleBarHeight {
		t.Errorf("left translate = (%.0f,%.0f), want (%d,%d)", x, y, 2-csdBorderThick, 100-csdTitleBarHeight)
	}
	// Right border strip: content x = surface x + content width.
	if x, y := csd.appCoords(csd.right.surf, 2, 100); x != float64(2+cw) || y != 100-csdTitleBarHeight {
		t.Errorf("right translate = (%.0f,%.0f), want (%d,%d)", x, y, 2+cw, 100-csdTitleBarHeight)
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

// TestWaylandCSDHiddenChromeResizeZones: when the chrome is hidden
// (setVisible(false) — server-side decoration negotiation) the content
// edges take over the full 8-zone resize mapping: the title-bar's
// top-left/top-right corners and top edge become the window's own
// corners/edge (user requirement), plus left/right/bottom edges and
// bottom corners. With the chrome visible the content never resizes.
func TestWaylandCSDHiddenChromeResizeZones(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	cw, ch := csd.stateW()
	fw, fh := float64(cw), float64(ch)

	// Chrome visible: content never participates in resize.
	if hit := csd.hitTest(w.surface, 2, 2); hit.act != csdActNone {
		t.Fatalf("visible chrome: content corner must not resize, got act=%d", hit.act)
	}

	// Chrome hidden: full 8-zone mapping on the content edges.
	csd.setVisible(false)
	zones := []struct {
		x, y float64
		edge int
	}{
		{1, 1, resizeTopLeft},               // 标题栏左上角 → 窗口左上角
		{fw - 2, 1, resizeTopRight},         // 标题栏右上角 → 窗口右上角
		{fw / 2, 1, resizeTop},              // 标题栏上边 → 窗口上边
		{1, fh - 2, resizeBottomLeft},       // 窗口左下角
		{fw - 2, fh - 2, resizeBottomRight}, // 窗口右下角
		{1, fh / 2, resizeLeft},             // 窗口左边
		{fw - 2, fh / 2, resizeRight},       // 窗口右边
		{fw / 2, fh - 1, resizeBottom},      // 窗口下边
	}
	for _, z := range zones {
		hit := csd.hitTest(w.surface, z.x, z.y)
		if hit.act != csdActResize || hit.edge != z.edge {
			t.Errorf("hidden chrome: hit(%.0f,%.0f) = act=%d edge=%d, want resize edge=%d",
				z.x, z.y, hit.act, hit.edge, z.edge)
		}
	}
	// Middle of the content stays app-owned (no resize).
	if hit := csd.hitTest(w.surface, fw/2, fh/2); hit.act != csdActNone {
		t.Errorf("hidden chrome: content middle must not resize, got act=%d", hit.act)
	}

	// Chrome back: content zones are inert again.
	csd.setVisible(true)
	if hit := csd.hitTest(w.surface, 1, 1); hit.act != csdActNone {
		t.Error("visible chrome again: content corner must not resize")
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

// TestWaylandCSDCursorApply: the resize-zone cursor path actually applies —
// setCursor lazily creates the cursor surface/theme and ends with curName
// set (applyCursorImage succeeded). Regression for "resize zones never show
// a resize cursor": ensureCursorSurface had no callers, so cursorSurf/
// cursorTheme stayed 0 and every applyCursorImage returned false.
func TestWaylandCSDCursorApply(t *testing.T) {
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

	csd.setCursor(900, csdHit{act: csdActResize, edge: resizeLeft})
	if csd.curName != "sb_h_double_arrow" {
		t.Fatalf("resize-left cursor = %q, want sb_h_double_arrow (theme/surface init failed?)", csd.curName)
	}
	csd.setCursor(901, csdHit{act: csdActResize, edge: resizeBottomRight})
	if csd.curName != "bottom_right_corner" && csd.curName != "nwse-resize" && csd.curName != "top_left_corner" {
		t.Fatalf("resize-BR cursor = %q, want bottom_right_corner, nwse-resize or top_left_corner (classic themes use the per-corner bottom name)", csd.curName)
	}
	// Back to the default arrow. mutter treats set_cursor(NULL) as "hide
	// the pointer" (empty surface = invisible), so the restore must
	// re-apply the theme's default "left_ptr" explicitly.
	csd.setCursor(902, csdHit{})
	if csd.curName != "left_ptr" {
		t.Errorf("default cursor = %q, want left_ptr (set_cursor NULL would hide the pointer)", csd.curName)
	}
}

// TestWaylandCSDCursorHotspot: applyCursorImage stores the cursor image's
// real hotspot, and setCursor passes it to wl_pointer.set_cursor. A zero
// hotspot is a real-world invisibility bug: mutter refreshes the cursor
// sprite texture on hotspot change and on damage-carrying commits — with
// hotspot (0,0) and no damage the sprite never gets its texture and the
// pointer stays invisible over the window.
func TestWaylandCSDCursorHotspot(t *testing.T) {
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

	csd.setCursor(910, csdHit{act: csdActResize, edge: resizeLeft})
	if csd.curName != "sb_h_double_arrow" {
		t.Fatalf("resize-left cursor = %q, want sb_h_double_arrow", csd.curName)
	}
	if csd.cursorHotX == 0 && csd.cursorHotY == 0 {
		t.Errorf("sb_h_double_arrow hotspot = (%d,%d), want non-zero (mutter shows no cursor for 0,0)", csd.cursorHotX, csd.cursorHotY)
	}
	if csd.cursorHotX == 0 || csd.cursorHotY == 0 {
		t.Errorf("sb_h_double_arrow hotspot = (%d,%d), want both axes non-zero", csd.cursorHotX, csd.cursorHotY)
	}
	// Corners use the standard diagonal double-arrow name first, falling
	// back to the classic name when the theme lacks it — either way the
	// applied cursor must be one of the candidates and carry a real
	// hotspot (the "unrecognized arrow at the corners" regression: Yaru's
	// top_left_corner is a single arrow; nwse-resize is the double arrow).
	csd.setCursor(911, csdHit{act: csdActResize, edge: resizeTopLeft})
	if csd.curName != "nwse-resize" && csd.curName != "top_left_corner" {
		t.Fatalf("top-left corner cursor = %q, want nwse-resize or top_left_corner", csd.curName)
	}
	if csd.cursorHotX == 0 || csd.cursorHotY == 0 {
		t.Errorf("corner cursor hotspot = (%d,%d), want both axes non-zero", csd.cursorHotX, csd.cursorHotY)
	}
	csd.setCursor(912, csdHit{act: csdActResize, edge: resizeTopRight})
	if csd.curName != "nesw-resize" && csd.curName != "top_right_corner" {
		t.Fatalf("top-right corner cursor = %q, want nesw-resize or top_right_corner", csd.curName)
	}
	// Bottom corners must resolve to their OWN classic name (bottom_left/
	// bottom_right_corner) — classic themes lack nwse/nesw, and falling back
	// to the top-corner names made the bottom corners show the top-corner
	// arrows ("bottom corners look like top corners").
	csd.setCursor(913, csdHit{act: csdActResize, edge: resizeBottomLeft})
	if csd.curName != "nesw-resize" && csd.curName != "bottom_left_corner" && csd.curName != "top_right_corner" {
		t.Fatalf("bottom-left corner cursor = %q, want nesw-resize, bottom_left_corner or top_right_corner", csd.curName)
	}
	csd.setCursor(914, csdHit{act: csdActResize, edge: resizeBottomRight})
	if csd.curName != "nwse-resize" && csd.curName != "bottom_right_corner" && csd.curName != "top_left_corner" {
		t.Fatalf("bottom-right corner cursor = %q, want nwse-resize, bottom_right_corner or top_left_corner", csd.curName)
	}
	// The cursor image's wl_buffer must come from wl_cursor_image_get_buffer
	// (the struct does not store it). A garbage value sent to attach makes
	// the compositor kill the connection ("invalid arguments for
	// wl_surface@N.attach") — verify the attached proxy carries a plausible
	// wire id (wl_proxy = { wl_object { interface*, id, version } ... }, id
	// at offset 8 on amd64; object ids are small, heap pointers are not).
	if csd.cursorBuf == 0 {
		t.Errorf("applyCursorImage attached no buffer")
	} else {
		// The wl_buffer proxy's wire id lives at offset 16 in this
		// libwayland build (verified empirically against the known-good CSD
		// shm buffer: same interface pointer at +0, small odd client-side
		// id at +16). A garbage value here is what killed the connection on
		// mutter ("invalid arguments for wl_surface@N.attach").
		id := *(*uint32)(unsafe.Pointer(csd.cursorBuf + 16))
		if id == 0 || id > 1<<20 {
			t.Errorf("cursor buffer wire id = %d (0x%x), want small wl object id", id, id)
		}
	}
}

// TestWaylandCSDTopRightCornerResizeHit: the title-bar top-right corner must
// be a resize grip (diagonal resize cursor), not the close button. The button
// zone used to win (x ≥ w-csdButtonW checked before the corner grip), so the
// corner never showed the diagonal arrow — only the close-button cursor.
// GTK4 parity: the title-bar top strip is a resize grip; the buttons sit
// below it.
func TestWaylandCSDTopRightCornerResizeHit(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	csd := w.csd
	if csd.top == nil {
		t.Skipf("no title-bar surface")
	}
	wTop := float64(csd.top.w)

	corner := csd.hitTest(csd.topSurface, wTop-1, 1)
	if corner.act != csdActResize || corner.edge != resizeTopRight {
		t.Fatalf("top-right corner hit = act=%d edge=%d, want resizeTopRight", corner.act, corner.edge)
	}
	// Just below the grip the close button still wins.
	btn := csd.hitTest(csd.topSurface, wTop-1, float64(csdCornerGrip+4))
	if btn.act != csdActClose {
		t.Fatalf("below-grip hit = act=%d, want close button", btn.act)
	}
	// Maximized/fullscreen/locked → no resize affordances: the corner is the
	// close button again.
	csd.state.Maximized = true
	mx := csd.hitTest(csd.topSurface, wTop-1, 1)
	csd.state.Maximized = false
	if mx.act != csdActClose {
		t.Fatalf("maximized top-right corner = act=%d, want close", mx.act)
	}
	// Top-left corner stays a resize grip.
	tl := csd.hitTest(csd.topSurface, 1, 1)
	if tl.act != csdActResize || tl.edge != resizeTopLeft {
		t.Fatalf("top-left corner hit = act=%d edge=%d, want resizeTopLeft", tl.act, tl.edge)
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
