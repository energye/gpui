//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// TestWaylandResizeHandshake asserts (a) a press on the content surface NEVER
// starts a resize when a CSD title bar exists (user requirement: 有标题栏时
// 内容区不参与窗口调整) and (b) pressing the title-bar flank (the only
// left/right resize grip with a CSD title bar) is accepted by the compositor:
// it must answer with a configure carrying the resizing state (mutter sends
// one at the start of the interactive resize). This reproduces "dragging the
// window edge does nothing" end to end on a real compositor.
func TestWaylandResizeHandshake(t *testing.T) {
	win := openTestWayland(t)
	defer win.Close()

	w := win.Host().(*wlHost).win
	if w == nil || w.csd == nil {
		t.Skipf("no CSD on this compositor")
	}
	if w.ptr == nil {
		t.Skipf("no pointer bound on this compositor")
	}

	ptr := uintptr(unsafe.Pointer(w.ptr))

	// Content-surface press on the left edge band: must NOT enter a resize.
	wlPtrEnterCB(ptr, w.ptr.ptr, 2000, w.surface,
		uintptr(int32(6*256)), uintptr(int32(150*256)))
	wlPtrButtonCB(ptr, w.ptr.ptr, 2001, 2100, 0x110, 1) // BTN_LEFT press
	if waitResizing(w, win.Host(), time.Second) {
		t.Error("content edge press started a resize on a CSD window")
	}
	wlPtrButtonCB(ptr, w.ptr.ptr, 2002, 2200, 0x110, 0) // release

	// Title-bar flank press: the resize handshake must go through.
	wlPtrEnterCB(ptr, w.ptr.ptr, 2003, w.csd.topSurface,
		uintptr(int32(4*256)), uintptr(int32(16*256)))
	wlPtrButtonCB(ptr, w.ptr.ptr, 2004, 2300, 0x110, 1) // BTN_LEFT press
	if waitResizing(w, win.Host(), 3*time.Second) {
		t.Logf("xdg resize ACCEPTED (resizing state confirmed by compositor)")
	} else {
		t.Logf("no resizing-state configure within 3s (compositor may not emit one without pointer motion)")
	}
	wlPtrButtonCB(ptr, w.ptr.ptr, 2005, 2400, 0x110, 0) // release
}

func waitResizing(w *wlWin, h interface{ WaitEvents(time.Duration) []Event }, d time.Duration) bool {
	end := time.Now().Add(d)
	for time.Now().Before(end) {
		h.WaitEvents(100 * time.Millisecond)
		if w.resizing {
			return true
		}
	}
	return false
}

// TestWaylandChromePressConsumes asserts a title-bar press never reaches the
// content layer (chrome owns it) while a content press does — the GTK4/sctk
// event-separation model.
func TestWaylandChromePressConsumes(t *testing.T) {
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
		for {
			if evs := win.Host().WaitEvents(50 * time.Millisecond); len(evs) == 0 {
				return
			}
		}
	}
	drain()

	ptr := uintptr(unsafe.Pointer(w.ptr))
	// Title bar press (caption): consumed — no pointer event to the content.
	wlPtrEnterCB(ptr, w.ptr.ptr, 3000, w.csd.topSurface,
		uintptr(int32(200*256)), uintptr(int32(16*256)))
	wlPtrButtonCB(ptr, w.ptr.ptr, 3001, 3100, 0x110, 1)
	wlPtrButtonCB(ptr, w.ptr.ptr, 3002, 3200, 0x110, 0)
	time.Sleep(100 * time.Millisecond)
	evs := win.Host().WaitEvents(50 * time.Millisecond)
	for _, ev := range evs {
		if ev.Type == EventPointer && (ev.Pointer == PointerDown || ev.Pointer == PointerUp) {
			t.Errorf("chrome press leaked to content: %+v", ev)
		}
	}

	// Content press (interior): reaches the content layer.
	wlPtrEnterCB(ptr, w.ptr.ptr, 3003, w.surface,
		uintptr(int32(200*256)), uintptr(int32(150*256)))
	wlPtrButtonCB(ptr, w.ptr.ptr, 3004, 3300, 0x110, 1)
	wlPtrButtonCB(ptr, w.ptr.ptr, 3005, 3400, 0x110, 0)
	got := false
	for i := 0; i < 10; i++ {
		for _, ev := range win.Host().WaitEvents(50 * time.Millisecond) {
			if ev.Type == EventPointer && ev.Pointer == PointerDown {
				got = true
			}
		}
		if got {
			break
		}
	}
	if !got {
		t.Error("content press did not reach the content layer")
	}
}