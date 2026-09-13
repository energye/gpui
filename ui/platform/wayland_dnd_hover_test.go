//go:build linux

package platform

import (
	"runtime"
	"sync"
	"testing"
	"unsafe"
)

// Headless tests for the Wayland DnD hover reporting (item 1): the
// wl_data_device enter/motion/leave callbacks must queue EventDragEnter,
// EventDragOver, EventDragLeave onto w.dndEvents (the same queue the Drop
// path uses), mirroring the X11 XDND semantics (always report enter,
// motion-without-enter synthesizes enter, stray leaves stay quiet).

var (
	hoverKeepAliveMu sync.Mutex
	hoverKeepAlive   = map[uintptr]*wlDataDeviceState{}
)

func newHoverState(t *testing.T) (*wlDataDeviceState, uintptr) {
	t.Helper()
	st := &wlDataDeviceState{win: &wlWin{}}
	self := uintptr(unsafe.Pointer(st))
	st.selfPtr = self
	hoverKeepAliveMu.Lock()
	hoverKeepAlive[self] = st
	hoverKeepAliveMu.Unlock()
	w := st.win
	w.selfPtr = uintptr(unsafe.Pointer(w))
	wlMu.Lock()
	wlByPtr[w.selfPtr] = w
	wlMu.Unlock()
	t.Cleanup(func() {
		hoverKeepAliveMu.Lock()
		delete(hoverKeepAlive, self)
		hoverKeepAliveMu.Unlock()
		wlMu.Lock()
		delete(wlByPtr, w.selfPtr)
		wlMu.Unlock()
		runtime.KeepAlive(st)
	})
	return st, self
}

// wlFixed lives in wayland_s6_p0_test.go (shared fixed-point helper).

func drainDnD(t *testing.T, st *wlDataDeviceState) []Event {
	t.Helper()
	w := st.win
	w.dndMu.Lock()
	defer w.dndMu.Unlock()
	out := append([]Event(nil), w.dndEvents...)
	w.dndEvents = nil
	return out
}

func announceMime(t *testing.T, self uintptr, offer uint, mime string) {
	t.Helper()
	b := append([]byte(mime), 0)
	wlOfferMimeCB(self, uintptr(offer), uintptr(unsafe.Pointer(&b[0])))
}

// Enter → mime → motion → leave produces the full trio with coordinates and
// the growing mime snapshot.
func TestWlDDHover_EnterMotionLeave(t *testing.T) {
	st, self := newHoverState(t)
	wlDDEnterCB(self, 0, 0, 0, wlFixed(100.5), wlFixed(50.25), 7)
	evs := drainDnD(t, st)
	if len(evs) != 1 || evs[0].Type != EventDragEnter {
		t.Fatalf("enter events = %+v, want one drag-enter", evs)
	}
	if evs[0].X != 100.5 || evs[0].Y != 50.25 {
		t.Fatalf("enter pos = (%.2f,%.2f), want (100.50,50.25)", evs[0].X, evs[0].Y)
	}
	if len(evs[0].MIMETypes) != 0 {
		t.Fatalf("enter mimes = %v, want empty (offers announce async)", evs[0].MIMETypes)
	}
	announceMime(t, self, 7, "text/uri-list")
	wlDDMotionCB(self, 0, 0, wlFixed(120), wlFixed(60))
	evs = drainDnD(t, st)
	if len(evs) != 1 || evs[0].Type != EventDragOver {
		t.Fatalf("motion events = %+v, want one drag-over", evs)
	}
	if evs[0].X != 120 || evs[0].Y != 60 {
		t.Fatalf("over pos = (%.1f,%.1f), want (120,60)", evs[0].X, evs[0].Y)
	}
	if len(evs[0].MIMETypes) != 1 || evs[0].MIMETypes[0] != "text/uri-list" {
		t.Fatalf("over mimes = %v, want [text/uri-list]", evs[0].MIMETypes)
	}
	wlDDLeaveCB(self, 0)
	evs = drainDnD(t, st)
	if len(evs) != 1 || evs[0].Type != EventDragLeave {
		t.Fatalf("leave events = %+v, want one drag-leave", evs)
	}
	if st.dragInFlight || st.dragOffer != 0 {
		t.Fatal("leave must clear the in-flight drag state")
	}
}

// Motion without a prior enter synthesizes the Enter first (X11 rule).
func TestWlDDHover_MotionWithoutEnter(t *testing.T) {
	st, self := newHoverState(t)
	wlDDMotionCB(self, 0, 0, wlFixed(10), wlFixed(20))
	evs := drainDnD(t, st)
	if len(evs) != 2 || evs[0].Type != EventDragEnter || evs[1].Type != EventDragOver {
		t.Fatalf("events = %+v, want synthesized enter + over", evs)
	}
}

// A leave with no drag inside stays quiet (X11 rule).
func TestWlDDHover_StrayLeaveQuiet(t *testing.T) {
	st, self := newHoverState(t)
	wlDDLeaveCB(self, 0)
	if evs := drainDnD(t, st); len(evs) != 0 {
		t.Fatalf("stray leave events = %+v, want quiet", evs)
	}
}

// A second enter without leave replaces the offer: the old one is queued
// for destruction and a fresh Enter is reported.
func TestWlDDHover_EnterReplacesOffer(t *testing.T) {
	st, self := newHoverState(t)
	wlDDEnterCB(self, 0, 0, 0, wlFixed(1), wlFixed(1), 7)
	drainDnD(t, st)
	wlDDEnterCB(self, 0, 0, 0, wlFixed(2), wlFixed(2), 9)
	evs := drainDnD(t, st)
	if len(evs) != 1 || evs[0].Type != EventDragEnter {
		t.Fatalf("re-enter events = %+v, want one drag-enter", evs)
	}
	if st.dragOffer != 9 {
		t.Fatalf("dragOffer = %d, want 9", st.dragOffer)
	}
	if len(st.pendingDestroys) != 1 {
		t.Fatalf("pendingDestroys = %d, want the retired offer queued", len(st.pendingDestroys))
	}
}

func TestWlDndFetchOrderURIListFirst(t *testing.T) {
	st, _ := newHoverState(t)
	_ = st
	seen := wlDndOrderForTest([]string{"text/plain", "text/uri-list", "image/png"})
	if len(seen) != 3 || seen[0] != mimeURIList {
		t.Fatalf("order = %v, want uri-list first", seen)
	}
	dup := wlDndOrderForTest([]string{mimeURIList, mimeURIList, "text/plain", ""})
	if len(dup) != 2 || dup[0] != mimeURIList || dup[1] != "text/plain" {
		t.Fatalf("dedup order = %v, want [uri-list text/plain]", dup)
	}
}

func TestWlDndFetchCapsDropOversize(t *testing.T) {
	big := make([]byte, wlDndMaxTypeBytes+1)
	if wlDndAcceptForTest(len(big), 0) {
		t.Fatal("oversize single must be rejected")
	}
	if !wlDndAcceptForTest(3, 0) {
		t.Fatal("small payload must be accepted")
	}
	if wlDndAcceptForTest(3, wlDndMaxTotalBytes) {
		t.Fatal("payload breaching the total cap must be rejected")
	}
}
