package platform

// Real-window resize-sync tests. Interactive resize on X11/XWayland is
// driven by _NET_WM_SYNC_REQUEST (the Flutter/Skia/GTK4 model): the
// compositor sends a sync request per drag step — live size in l[1]/l[2],
// serial in l[3]/l[4] — and waits for the counter to pass the serial before
// showing the painted frame. Without the protocol (gogpu reference model)
// XWayland/mutter keeps the X geometry frozen for the whole drag and only
// stretches the stale buffer: the app gets no ConfigureNotify, renders
// nothing, and content freezes until pause/release (measured twice on this
// machine). These run against the real X server and are skipped without
// DISPLAY.

import (
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Test-only X bindings (not part of the platform API).
var (
	testXInternAtom func(dpy uintptr, name *byte, onlyIfExists int) uintptr
	testXSendEvent  func(dpy uintptr, win uintptr, propagate int, eventMask int64, ev *byte) int
	testXFlush      func(dpy uintptr) int
	testXGetProp    func(dpy uintptr, win uintptr, property uintptr, longOffset, longLength int64,
		deleteProp int, reqType uintptr, actualType *uintptr, actualFormat *int, nItems *uint64,
		bytesAfter *uint64, prop *unsafe.Pointer) int
	testXFree func(data *byte) int
)

func bindTestX11(l *x11Lib) {
	if testXSendEvent != nil {
		return
	}
	purego.RegisterLibFunc(&testXInternAtom, l.lib, "XInternAtom")
	purego.RegisterLibFunc(&testXSendEvent, l.lib, "XSendEvent")
	purego.RegisterLibFunc(&testXFlush, l.lib, "XFlush")
	purego.RegisterLibFunc(&testXGetProp, l.lib, "XGetWindowProperty")
	purego.RegisterLibFunc(&testXFree, l.lib, "XFree")
}

func w32(b []byte, off int, v uint64) {
	for i := 0; i < 8; i++ {
		b[off+i] = byte(v >> (8 * i))
	}
}

// TestX11RealWindow_ResizeSyncProtocol: the window MUST advertise
// _NET_WM_SYNC_REQUEST in WM_PROTOCOLS and carry a zero-initialized
// _NET_WM_SYNC_REQUEST_COUNTER property. Declaring the protocol makes the
// compositor drive an interactive resize drag with sync requests — each
// carries the live size (l[1]/l[2]) and a serial the counter must pass —
// instead of stretching a frozen buffer for the whole drag. Without it
// (gogpu reference model) XWayland/mutter keeps the X geometry frozen while
// the mouse moves: the app gets no ConfigureNotify, renders nothing, and
// content appears to freeze until pause/release. Flutter/Skia/GTK4 all
// declare this protocol for live resize on X11/XWayland.
func TestX11RealWindow_ResizeSyncProtocol(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()

	host, ok := win.Host().(*x11Host)
	if !ok {
		t.Fatalf("host is %T, want *x11Host", win.Host())
	}
	st := host.st
	bindTestX11(host.lib)

	name := append([]byte("WM_PROTOCOLS"), 0)
	atProtocols := testXInternAtom(st.display, &name[0], 1)
	if atProtocols == 0 {
		t.Skip("WM_PROTOCOLS atom unavailable")
	}
	var (
		actualType uintptr
		actualFmt  int
		nItems     uint64
		bytesAfter uint64
		prop       unsafe.Pointer
	)
	rc := testXGetProp(st.display, st.window, atProtocols, 0, 1024, 0, 0,
		&actualType, &actualFmt, &nItems, &bytesAfter, &prop)
	if rc != 0 || prop == nil {
		t.Fatalf("XGetWindowProperty(WM_PROTOCOLS) rc=%d prop=%v", rc, prop)
	}
	defer testXFree((*byte)(prop))
	atoms := unsafe.Slice((*uintptr)(prop), int(nItems))
	found := false
	for _, a := range atoms {
		if a == st.atSyncReq {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("WM_PROTOCOLS does not advertise _NET_WM_SYNC_REQUEST (%#x): XWayland drag freezes content", st.atSyncReq)
	}

	// The counter property must exist and start at 0 (spec requirement;
	// mutter checks the property before treating the window as sync-capable).
	if st.atSyncCounter == 0 {
		t.Fatal("atSyncCounter atom unresolved")
	}
	var (
		ctType uintptr
		ctFmt  int
		ctN    uint64
		ctRest uint64
		ctProp unsafe.Pointer
	)
	rc = testXGetProp(st.display, st.window, st.atSyncCounter, 0, 4, 0, 0,
		&ctType, &ctFmt, &ctN, &ctRest, &ctProp)
	if rc != 0 || ctProp == nil {
		t.Fatalf("XGetWindowProperty(_NET_WM_SYNC_REQUEST_COUNTER) rc=%d prop=%v", rc, ctProp)
	}
	defer testXFree((*byte)(ctProp))
	if ctFmt != 32 {
		t.Fatalf("counter format = %d, want 32 (long[2] = 64-bit)", ctFmt)
	}
	words := unsafe.Slice((*uint64)(ctProp), int(ctN))
	if len(words) == 0 || words[0] != 0 {
		t.Fatalf("counter initial value = %v, want 0 (spec: counter starts at 0)", words)
	}
}

// TestX11RealWindow_SyncRequestAdvancesCounter: a compositor sync request
// (live resize drag step) must surface EventResize with the request's new
// size and EventResizeSync, record the serial, and NotifyFrameDrawn must
// advance the counter past that serial — the painted frame unblocks the
// compositor's next drag step. Guards the l[3]/l[4] serial layout (the
// height-as-serial bug froze the compositor permanently).
func TestX11RealWindow_SyncRequestAdvancesCounter(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()

	host, ok := win.Host().(*x11Host)
	if !ok {
		t.Fatalf("host is %T, want *x11Host", win.Host())
	}
	st := host.st
	bindTestX11(host.lib)
	if st.atSyncReq == 0 {
		t.Fatal("atSyncReq atom unresolved; resize sync not declared")
	}

	const (
		serial  = 123456789
		wantW   = 700
		wantH   = 500
	)
	// XClientMessageEvent (x86-64): data.l[] at 56, 8 bytes each on x64.
	var ev [112]byte
	ev[0] = 33 // ClientMessage
	w32(ev[:], 32, uint64(st.window))        // window
	w32(ev[:], 40, uint64(st.atSyncReq))     // message_type
	w32(ev[:], 48, 32)                       // format
	w32(ev[:], 56, uint64(st.atSyncReq))     // l[0] = atom
	w32(ev[:], 64, wantW)                    // l[1] = width
	w32(ev[:], 72, wantH)                    // l[2] = height
	w32(ev[:], 80, serial)                   // l[3] = serial low 32 (flags high 32 = 0)
	w32(ev[:], 88, 0)                        // l[4] = serial high 32
	if rc := testXSendEvent(st.display, st.window, 0, 1<<17 /*StructureNotifyMask*/, &ev[0]); rc == 0 {
		t.Fatalf("XSendEvent(sync request) failed rc=%d", rc)
	}
	testXFlush(st.display)

	deadline := time.Now().Add(2 * time.Second)
	syncSeen := false
	for time.Now().Before(deadline) {
		for _, e := range host.WaitEvents(100 * time.Millisecond) {
			switch e.Type {
			case EventResizeSync:
				syncSeen = true
			case EventResize:
				if e.Width == wantW && e.Height == wantH {
					host.NotifyFrameDrawn() // the painted frame advances the counter
					st.mu.Lock()
					cnt := st.syncCounter
					pend := st.pendingSync
					st.mu.Unlock()
					if !syncSeen {
						t.Fatalf("EventResize before EventResizeSync (serial not recorded) pend=%d", pend)
					}
					if cnt < serial+1 {
						t.Fatalf("counter = %d after NotifyFrameDrawn, want >= %d (serial+1)", cnt, serial+1)
					}
					if pend != 0 {
						t.Fatalf("pendingSync = %d after frame, want 0 (consumed)", pend)
					}
					return
				}
			}
		}
	}
	t.Fatal("sync request never surfaced EventResizeSize/EventResize with the request geometry")
}

// TestX11RealWindow_ConfigNotifyDrivesResize: a compositor-sent
// XConfigureEvent (as during a live drag) surfaces EventResize with the new
// geometry — the geometry signal path that keeps content following the drag.
func TestX11RealWindow_ConfigNotifyDrivesResize(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()

	host, ok := win.Host().(*x11Host)
	if !ok {
		t.Fatalf("host is %T, want *x11Host", win.Host())
	}
	st := host.st
	bindTestX11(host.lib)

	// XConfigureEvent (x86-64): type=22, display@24, event@32, window@40,
	// x@48, y@52, width@56, height@60, border@64, above@72, override@80.
	var ev [112]byte
	ev[0] = 22
	w32(ev[:], 32, uint64(st.window))
	w32(ev[:], 40, uint64(st.window))
	w32(ev[:], 56, 700)
	w32(ev[:], 60, 500)
	if rc := testXSendEvent(st.display, st.window, 0, 1<<17 /*StructureNotifyMask*/, &ev[0]); rc == 0 {
		t.Fatalf("XSendEvent(configure) failed rc=%d", rc)
	}
	testXFlush(st.display)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range host.WaitEvents(100 * time.Millisecond) {
			if e.Type == EventResize && e.Width == 700 && e.Height == 500 {
				return
			}
		}
	}
	t.Fatal("EventResize(700x500) from ConfigureNotify never surfaced")
}
