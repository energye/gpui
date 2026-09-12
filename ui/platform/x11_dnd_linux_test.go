//go:build linux

package platform

import (
	"testing"
	"time"
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
		got = append(got, th.WaitEvents(50*time.Millisecond)...)
		// Pump the source so SelectionRequest(XdndSelection) is served;
		// source Status/Finished replies are ignored by the pump.
		_ = sh.WaitEvents(0)
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
	var drop *Event
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range th.WaitEvents(50 * time.Millisecond) {
			if e.Type == EventDrop {
				c := e
				drop = &c
			}
		}
		_ = sh.WaitEvents(0)
		if drop != nil {
			break
		}
	}
	if drop == nil {
		t.Fatalf("no EventDrop after XDND file drop")
	}
	if len(drop.Files) != 2 || drop.Files[0] != "/tmp/a" || drop.Files[1] != "/tmp/b" {
		t.Fatalf("drop files = %v, want [/tmp/a /tmp/b]", drop.Files)
	}
}

func TestX11DnDNonURIListNoDrop(t *testing.T) {
	tw, _, th, sh, ts, ss := x11DndOpenPair(t)

	// Offer TEXT (no uri-list): target must reject (Status) and swallow Drop.
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
