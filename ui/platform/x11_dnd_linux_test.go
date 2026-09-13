//go:build linux

package platform

import (
	"testing"
	"time"
	"unsafe"
)

// Two-window XDND対拖: source sends Enter/Position/Drop via real ClientMessage,
// target must emit DragEnter/DragOver/Drop(files). Skipped without DISPLAY.

func x11DndOpenPair(t *testing.T) (target, source *Window, th, sh *x11Host, ts, ss *x11State) {
	t.Helper()
	if !HasX11Display() {
		t.Skipf("xdnd test skipped: DISPLAY not set")
	}
	tw, err := Open(Options{Width: 400, Height: 300, Title: "gpui xdnd target", Backend: DisplayX11})
	if err != nil {
		t.Fatalf("open target: %v", err)
	}
	sw, err := Open(Options{Width: 400, Height: 300, Title: "gpui xdnd source", Backend: DisplayX11})
	if err != nil {
		tw.Close()
		t.Fatalf("open source: %v", err)
	}
	t.Cleanup(func() { tw.Close(); sw.Close() })
	th, ok := tw.Host().(*x11Host)
	if !ok || th.st == nil {
		t.Fatalf("target not x11")
	}
	shHost, ok2 := sw.Host().(*x11Host)
	if !ok2 || shHost.st == nil {
		t.Fatalf("source not x11")
	}
	sh = shHost
	ts, ss = th.st, sh.st
	if ts.atXdndEnter == 0 || ts.atXdndPosition == 0 || ts.atXdndLeave == 0 || ts.atXdndDrop == 0 {
		t.Skipf("xdnd atoms unresolved")
	}
	if ss.atXdndEnter == 0 || ss.atTextUriList == 0 {
		t.Skipf("source xdnd atoms unresolved")
	}
	// Drain create-time noise on both.
	th.WaitEvents(0)
	sh.WaitEvents(0)
	return tw, sw, th, sh, ts, ss
}

func x11DndCollectBoth(th, sh *x11Host, timeout time.Duration) []Event {
	var got []Event
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		// Pump the source FIRST so SelectionRequest(XdndSelection) is
		// served before the target drains: the convert reply must be in
		// the target queue when it drains, or the TARGETS step stalls a
		// full round-trip (MIME phase 2 serial queue).
		_ = sh.WaitEvents(0)
		got = append(got, th.WaitEvents(50*time.Millisecond)...)
	}
	return got
}

func x11DndHas(got []Event, ty EventType) *Event {
	for i := range got {
		if got[i].Type == ty {
			return &got[i]
		}
	}
	return nil
}

// x11DndEnter2Types crafts an XdndEnter ClientMessage offering two inline
// type atoms (XDND packs up to 3 inline; more go via XdndTypeList).
func x11DndEnter2Types(ss *x11State, targetWin, t1, t2 uintptr) []byte {
	ev := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&ev[0])) = int32(xClientMessage)
	*(*uintptr)(unsafe.Pointer(&ev[32])) = targetWin
	*(*uintptr)(unsafe.Pointer(&ev[40])) = ss.atXdndEnter
	*(*int32)(unsafe.Pointer(&ev[48])) = 32
	*(*uint64)(unsafe.Pointer(&ev[56])) = uint64(ss.window)
	*(*uint64)(unsafe.Pointer(&ev[64])) = uint64(5 << 24)
	*(*uint64)(unsafe.Pointer(&ev[72])) = uint64(t1)
	*(*uint64)(unsafe.Pointer(&ev[80])) = uint64(t2)
	return ev
}

// x11DndSendBuf delivers a crafted ClientMessage buffer to the target
// window via the source connection.
func x11DndSendBuf(ss *x11State, targetWin uintptr, ev []byte) {
	lib := ctlLib.open()
	if ss == nil || ss.display == 0 || targetWin == 0 || !lib.ok() || lib.sendEvent == nil {
		return
	}
	lib.sendEvent(ss.display, targetWin, 0, 0, &ev[0])
	if ss.flush != nil {
		ss.flush()
	}
}

// x11DndInsideRoot returns a root position inside the target window (50,50
// window-local) so Position maps to a plausible in-window DragOver/Drop.
// Falls back to (100,100) when the position query is unavailable.
func x11DndInsideRoot(t *testing.T, target *Window, ts *x11State) (int, int) {
	t.Helper()
	if target != nil {
		if ctl := target.Controls(); ctl != nil {
			if x, y, ok := ctl.Position(); ok {
				return x + 50, y + 50
			}
		}
	}
	return 100, 100
}

func TestX11DnDEnterOverLeave(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	// Enter with a single inline type (text/uri-list).
	x11DndSendClient(ss, ts.window, ss.atXdndEnter,
		uint64(ss.window), uint64(5<<24), uint64(ss.atTextUriList), 0, 0)
	got := x11DndCollectBoth(th, sh, 2*time.Second)
	enter := x11DndHas(got, EventDragEnter)
	if enter == nil {
		t.Fatalf("no DragEnter; saw %v", evTypes(got))
	}
	found := false
	for _, m := range enter.MIMETypes {
		if m == mimeURIList {
			found = true
		}
	}
	if !found {
		t.Fatalf("enter mimes = %v, want text/uri-list", enter.MIMETypes)
	}
	// Position inside the window → window-local via Translate.
	rx, ry := x11DndInsideRoot(t, tw, ts)
	x11DndSendClient(ss, ts.window, ss.atXdndPosition,
		uint64(ss.window), 0, uint64((rx<<16)|ry), 0, uint64(ss.atXdndActionCopy))
	got = x11DndCollectBoth(th, sh, 2*time.Second)
	over := x11DndHas(got, EventDragOver)
	if over == nil {
		t.Fatalf("no DragOver; saw %v", evTypes(got))
	}
	if over.X == 0 && over.Y == 0 {
		t.Logf("note: over at (0,0) — translate fallback, still a valid position event")
	}
	// Leave clears the drag.
	x11DndSendClient(ss, ts.window, ss.atXdndLeave, uint64(ss.window), 0, 0, 0, 0)
	got = x11DndCollectBoth(th, sh, 2*time.Second)
	if leave := x11DndHas(got, EventDragLeave); leave == nil {
		t.Fatalf("no DragLeave; saw %v", evTypes(got))
	}
	// Stale Leave stays quiet.
	soak := x11DndCollectBoth(th, sh, 400*time.Millisecond)
	for _, e := range soak {
		if e.Type == EventDragEnter || e.Type == EventDragOver || e.Type == EventDragLeave || e.Type == EventDrop {
			t.Fatalf("stale leave emitted %+v", e)
		}
	}
}

func TestX11DnDDropFiles(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	ss.dndMu.Lock()
	ss.dndSrcData = "file:///tmp/a\nfile:///tmp/b\n"
	ss.dndMu.Unlock()
	x11DndOwnSelection(ss)

	x11DndSendClient(ss, ts.window, ss.atXdndEnter,
		uint64(ss.window), uint64(5<<24), uint64(ss.atTextUriList), 0, 0)
	// Wait for Enter so the target has the mime list before Position/Drop.
	x11DndCollectBoth(th, sh, 2*time.Second)
	rx, ry := x11DndInsideRoot(t, tw, ts)
	x11DndSendClient(ss, ts.window, ss.atXdndPosition,
		uint64(ss.window), 0, uint64((rx<<16)|ry), 0, uint64(ss.atXdndActionCopy))
	x11DndCollectBoth(th, sh, 2*time.Second)
	x11DndSendClient(ss, ts.window, ss.atXdndDrop, uint64(ss.window), 0, 0, 0, 0)
	drop := x11DndWaitDrop(th, sh, 5*time.Second)
	if drop == nil {
		t.Fatalf("no EventDrop after XDND file drop")
	}
	if len(drop.Files) != 2 || drop.Files[0] != "/tmp/a" || drop.Files[1] != "/tmp/b" {
		t.Fatalf("drop files = %v, want [/tmp/a /tmp/b]", drop.Files)
	}
	if _, ok := drop.DropData[mimeURIList]; !ok {
		t.Fatalf("drop data missing raw %q: %q", mimeURIList, drop.DropData)
	}
}

func x11DndWaitDrop(th, sh *x11Host, timeout time.Duration) *Event {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_ = sh.WaitEvents(0)
		for _, e := range th.WaitEvents(50 * time.Millisecond) {
			if e.Type == EventDrop {
				c := e
				return &c
			}
		}
	}
	return nil
}

func TestX11DnDNonURIListNoDrop(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	// Offer TEXT with no servable payload: TARGETS answers empty, the
	// target must swallow the Drop (no EventDrop).
	textAtom := ss.atUTF8
	if textAtom == 0 {
		t.Skipf("UTF8 atom unresolved")
	}
	x11DndSendClient(ss, ts.window, ss.atXdndEnter,
		uint64(ss.window), uint64(5<<24), uint64(textAtom), 0, 0)
	x11DndCollectBoth(th, sh, 2*time.Second)
	rx, ry := x11DndInsideRoot(t, tw, ts)
	x11DndSendClient(ss, ts.window, ss.atXdndPosition,
		uint64(ss.window), 0, uint64((rx<<16)|ry), 0, uint64(ss.atXdndActionCopy))
	x11DndCollectBoth(th, sh, 2*time.Second)
	x11DndSendClient(ss, ts.window, ss.atXdndDrop, uint64(ss.window), 0, 0, 0, 0)
	got := x11DndCollectBoth(th, sh, 1*time.Second)
	for _, e := range got {
		if e.Type == EventDrop {
			t.Fatalf("non-uri-list drop must be swallowed, got %+v", e)
		}
	}
}

func TestX11DnDTextPlainFillsData(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	// Offer UTF8_STRING only (no uri-list): MIME phase 2 must convert it
	// and report a Drop with Data (previously swallowed, Data恒空).
	textAtom := ss.atUTF8
	if textAtom == 0 {
		t.Skipf("UTF8 atom unresolved")
	}
	ss.dndMu.Lock()
	ss.dndSrcMimeData = map[string]string{"UTF8_STRING": "hello dnd"}
	ss.dndMu.Unlock()
	x11DndOwnSelection(ss)
	x11DndSendClient(ss, ts.window, ss.atXdndEnter,
		uint64(ss.window), uint64(5<<24), uint64(textAtom), 0, 0)
	x11DndCollectBoth(th, sh, 2*time.Second)
	rx, ry := x11DndInsideRoot(t, tw, ts)
	x11DndSendClient(ss, ts.window, ss.atXdndPosition,
		uint64(ss.window), 0, uint64((rx<<16)|ry), 0, uint64(ss.atXdndActionCopy))
	x11DndCollectBoth(th, sh, 2*time.Second)
	x11DndSendClient(ss, ts.window, ss.atXdndDrop, uint64(ss.window), 0, 0, 0, 0)
	drop := x11DndWaitDrop(th, sh, 5*time.Second)
	if drop == nil {
		t.Fatal("no EventDrop after text-only drop (want Data)")
	}
	if len(drop.Files) != 0 {
		t.Fatalf("text drop files = %v, want none", drop.Files)
	}
	if string(drop.DropData["UTF8_STRING"]) != "hello dnd" {
		t.Fatalf("text drop data = %q, want %q", drop.DropData, "hello dnd")
	}
}

func TestX11DnDFilesPlusTextData(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	// Offer uri-list + UTF8_STRING: one Drop carries both Files and Data.
	// The Enter carries both inline atoms (XDND packs up to 3); TARGETS
	// then confirms the pair and the serial queue fetches both.
	ss.dndMu.Lock()
	ss.dndSrcData = "file:///tmp/a\n"
	ss.dndSrcMimeData = map[string]string{"UTF8_STRING": "note"}
	ss.dndMu.Unlock()
	x11DndOwnSelection(ss)
	enterBuf := x11DndEnter2Types(ss, ts.window, ss.atTextUriList, ss.atUTF8)
	x11DndSendBuf(ss, ts.window, enterBuf)
	x11DndCollectBoth(th, sh, 2*time.Second)
	rx, ry := x11DndInsideRoot(t, tw, ts)
	x11DndSendClient(ss, ts.window, ss.atXdndPosition,
		uint64(ss.window), 0, uint64((rx<<16)|ry), 0, uint64(ss.atXdndActionCopy))
	x11DndCollectBoth(th, sh, 2*time.Second)
	x11DndSendClient(ss, ts.window, ss.atXdndDrop, uint64(ss.window), 0, 0, 0, 0)
	drop := x11DndWaitDrop(th, sh, 8*time.Second)
	if drop == nil {
		t.Fatal("no EventDrop after files+text drop")
	}
	if len(drop.Files) != 1 || drop.Files[0] != "/tmp/a" {
		t.Fatalf("drop files = %v, want [/tmp/a]", drop.Files)
	}
	if string(drop.DropData["UTF8_STRING"]) != "note" {
		t.Fatalf("drop data = %q, want %q", drop.DropData, "note")
	}
	if _, ok := drop.DropData[mimeURIList]; !ok {
		t.Fatalf("drop data missing raw %q: %q", mimeURIList, drop.DropData)
	}
}

// Outbound source via the public API (S6-P1 item 4): the source window's
// controller drops Files + Data onto the explicit target; the target must
// see Enter/Over plus one Drop carrying both.
func TestX11DnDSourceStartDragTo(t *testing.T) {
	tw, sw, th, sh, _, _ := x11DndOpenPair(t)

	sctl, ok := sw.Controls().(*x11Controller)
	if !ok {
		t.Fatal("source has no x11 controller")
	}
	offer := DragOffer{
		Files: []string{"/tmp/a", "/tmp/b"},
		Data:  map[string][]byte{"UTF8_STRING": []byte("via source")},
	}
	if err := sctl.StartDragTo(tw, offer); err != nil {
		t.Fatalf("StartDragTo: %v", err)
	}
	got := x11DndCollectBoth(th, sh, 3*time.Second)
	if x11DndHas(got, EventDragEnter) == nil {
		t.Fatalf("no DragEnter after StartDragTo; saw %v", evTypes(got))
	}
	if x11DndHas(got, EventDragOver) == nil {
		t.Fatalf("no DragOver after StartDragTo; saw %v", evTypes(got))
	}
	// The Enter/Position/Drop trio is sent back-to-back, so the whole
	// chain may already have completed inside the collect above.
	drop := x11DndHas(got, EventDrop)
	if drop == nil {
		c := x11DndWaitDrop(th, sh, 8*time.Second)
		if c == nil {
			t.Fatal("no EventDrop after StartDragTo")
		}
		drop = c
	}
	if len(drop.Files) != 2 || drop.Files[0] != "/tmp/a" || drop.Files[1] != "/tmp/b" {
		t.Fatalf("drop files = %v, want [/tmp/a /tmp/b]", drop.Files)
	}
	if string(drop.DropData["UTF8_STRING"]) != "via source" {
		t.Fatalf("drop data = %q, want %q", drop.DropData, "via source")
	}
	if _, ok := drop.DropData[mimeURIList]; !ok {
		t.Fatalf("drop data missing raw %q: %q", mimeURIList, drop.DropData)
	}
}

// Empty offers and nil targets are rejected before touching the server.
func TestX11DnDSourceStartDragToRejects(t *testing.T) {
	tw, sw, th, sh, _, _ := x11DndOpenPair(t)

	sctl, ok := sw.Controls().(*x11Controller)
	if !ok {
		t.Fatal("source has no x11 controller")
	}
	if err := sctl.StartDragTo(tw, DragOffer{}); err == nil {
		t.Fatal("empty offer must fail")
	}
	if err := sctl.StartDragTo(nil, DragOffer{Files: []string{"/tmp/a"}}); err == nil {
		t.Fatal("nil target must fail")
	}
	if err := sctl.StartDragTo(sw, DragOffer{Data: map[string][]byte{"x/y": {}}}); err == nil {
		t.Fatal("empty payload bytes must fail")
	}
	soak := x11DndCollectBoth(th, sh, 400*time.Millisecond)
	for _, e := range soak {
		if e.Type == EventDrop {
			t.Fatalf("rejected drag must not drop, got %+v", e)
		}
	}
}

// Nil-safe: no window behind the controller reports an error, never panics
// (runs headless, no DISPLAY needed).
func TestX11DnDSourceNilSafe(t *testing.T) {
	var nilCtl *x11Controller
	if err := nilCtl.StartDrag(DragOffer{Files: []string{"/tmp/a"}}); err == nil {
		t.Fatal("nil controller StartDrag must fail")
	}
	if err := nilCtl.StartDragTo(nil, DragOffer{}); err == nil {
		t.Fatal("nil controller StartDragTo must fail")
	}
	if x11WindowOf(nil) != 0 {
		t.Fatal("x11WindowOf(nil) must be 0")
	}
	if x11DndCheckAware(nil, 1234) {
		t.Fatal("checkAware(nil) must be false")
	}
	if x11DndCheckAware(&x11State{}, 0) {
		t.Fatal("checkAware(win 0) must be false")
	}
	x11DndClearSource(nil) // must not panic
	payload := map[string][]byte{mimeURIList: []byte("file:///tmp/a\n")}
	st := &x11State{}
	x11DndStageSource(st, payload)
	if st.dndSrcData != "file:///tmp/a\n" {
		t.Fatalf("staged uri-list = %q", st.dndSrcData)
	}
	x11DndClearSource(st)
	if st.dndSrcData != "" || st.dndSrcMimeData != nil {
		t.Fatalf("cleared source = %q %v", st.dndSrcData, st.dndSrcMimeData)
	}
	if got := (DragOffer{Data: map[string][]byte{"b/b": {1}, mimeURIList: {2}, "a/a": {3}, "": {4}, "e/e": {}}}).MIMETypes(); len(got) != 3 ||
		got[0] != mimeURIList || got[1] != "a/a" || got[2] != "b/b" {
		t.Fatalf("ordered mimes = %v", got)
	}
}

// Windows advertise XdndAware at Create; window 0 never does.
func TestX11DnDAwareProbe(t *testing.T) {
	_, _, _, _, ts, _ := x11DndOpenPair(t)

	if !x11DndCheckAware(ts, ts.window) {
		t.Fatal("own window must be XdndAware")
	}
	if x11DndCheckAware(ts, 0) {
		t.Fatal("window 0 must not be XdndAware")
	}
}
