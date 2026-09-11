//go:build linux

package platform

import (
	"encoding/binary"
	"math"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// S6-P0 X11 backend reporting tests (real X server, no WM needed). Each test
// opens its own window; assertions pump the real event queue (synthetic
// client messages for close/buttons/keys, direct property writes for state).

func x11P0Handles(t *testing.T, win *Window) (*x11Host, *x11State, *x11CtlLib) {
	t.Helper()
	h, ok := win.Host().(*x11Host)
	if !ok || h.st == nil {
		t.Fatalf("not an x11 host")
	}
	lib := ctlLib.open()
	if !lib.ok() {
		t.Skipf("x11 ctl lib unavailable")
	}
	return h, h.st, lib
}

// x11Collect drains until cond is true or the deadline passes, returning
// everything seen.
func x11Collect(h *x11Host, timeout time.Duration, cond func([]Event) bool) []Event {
	var got []Event
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		got = append(got, h.WaitEvents(50*time.Millisecond)...)
		if cond != nil && cond(got) {
			return got
		}
	}
	return got
}

func x11HasType(got []Event, ty EventType) *Event {
	for i := range got {
		if got[i].Type == ty {
			return &got[i]
		}
	}
	return nil
}

// x11SendEvent delivers a crafted native event to our own window.
func x11SendEvent(st *x11State, lib *x11CtlLib, ev []byte) {
	if lib.sendEvent == nil {
		return
	}
	lib.sendEvent(st.display, st.window, 0, 0, &ev[0])
	st.flush()
}

func x11ClientMessage(st *x11State, msgType uintptr, data0 uint64) []byte {
	ev := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&ev[0])) = 33 // ClientMessage
	*(*uintptr)(unsafe.Pointer(&ev[32])) = st.window
	*(*uintptr)(unsafe.Pointer(&ev[40])) = msgType
	*(*int32)(unsafe.Pointer(&ev[48])) = 32 // format
	*(*uint64)(unsafe.Pointer(&ev[56])) = data0
	return ev
}

func TestX11CloseRequestedSplit(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)

	protos := append([]byte("WM_PROTOCOLS"), 0)
	msgType := lib.internAtom(st.display, &protos[0], 0)
	if msgType == 0 || st.wmDelete == 0 {
		t.Skipf("WM atoms unavailable")
	}
	x11SendEvent(st, lib, x11ClientMessage(st, msgType, uint64(st.wmDelete)))
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return x11HasType(ev, EventCloseRequested) != nil
	})
	if e := x11HasType(got, EventCloseRequested); e == nil {
		t.Fatalf("WM_DELETE produced no CloseRequested; saw %v", evTypes(got))
	}
	if e := x11HasType(got, EventClose); e != nil {
		t.Fatalf("WM_DELETE must not report destroyed; saw %+v", *e)
	}
}

func TestX11FocusWriteback(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)
	_ = st
	ctl, ok := win.Controls().(*x11Controller)
	if !ok {
		t.Skipf("no x11 controller")
	}
	if lib.setInputFocus == nil {
		t.Skipf("XSetInputFocus unavailable")
	}
	// Wait until the server reports the window viewable: focusing during
	// Mutter's reparent/map dance is a fatal BadMatch. The gate drains the
	// queue, so a spontaneous early FocusIn is recorded here — focusing an
	// already-focused window is a server no-op with no new event.
	x11WaitViewable(t, h, st, lib)
	preFocused := false
	for _, e := range h.WaitEvents(0) {
		if e.Type == EventFocus && e.Focused {
			preFocused = true
		}
	}
	if err := ctl.Focus(); err != nil {
		t.Fatalf("Focus() = %v", err)
	}
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventFocus && e.Focused {
				return true
			}
		}
		return false
	})
	found := preFocused
	for _, e := range got {
		if e.Type == EventFocus && e.Focused {
			found = true
		}
	}
	if !found {
		t.Fatalf("no Focus event after Focus() (nor before); saw %v", evTypes(got))
	}
	if !ctl.IsFocused() {
		t.Fatal("IsFocused()=false after FocusIn (writeback missing)")
	}
}

func x11Crossing(st *x11State, typ int32, x, y int32) []byte {
	ev := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&ev[0])) = typ // 7 enter / 8 leave
	*(*uintptr)(unsafe.Pointer(&ev[32])) = st.window
	*(*int32)(unsafe.Pointer(&ev[64])) = x
	*(*int32)(unsafe.Pointer(&ev[68])) = y
	return ev
}

func TestX11EnterLeaveCoords(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)

	x11SendEvent(st, lib, x11Crossing(st, 7, 30, 40))
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventPointer && e.Pointer == PointerEnter {
				return true
			}
		}
		return false
	})
	var enter *Event
	for i := range got {
		if got[i].Type == EventPointer && got[i].Pointer == PointerEnter {
			enter = &got[i]
		}
	}
	if enter == nil {
		t.Fatalf("no PointerEnter; saw %v", evTypes(got))
	}
	if enter.X != 30 || enter.Y != 40 {
		t.Fatalf("enter pos = (%.0f,%.0f), want (30,40)", enter.X, enter.Y)
	}
	x11SendEvent(st, lib, x11Crossing(st, 8, 11, 22))
	got = x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventPointer && e.Pointer == PointerLeave {
				return true
			}
		}
		return false
	})
	var leave *Event
	for i := range got {
		if got[i].Type == EventPointer && got[i].Pointer == PointerLeave {
			leave = &got[i]
		}
	}
	if leave == nil {
		t.Fatalf("no PointerLeave; saw %v", evTypes(got))
	}
	if leave.X != 11 || leave.Y != 22 {
		t.Fatalf("leave pos = (%.0f,%.0f), want (11,22)", leave.X, leave.Y)
	}
}

func TestX11HideShowEmits(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)
	ctl, ok := win.Controls().(*x11Controller)
	if !ok {
		t.Skipf("no x11 controller")
	}
	// Show/hide is meaningless on a window the compositor never mapped (our
	// unmap would be a silent server no-op and every witness below would
	// time out): skip early instead of failing late.
	x11WaitViewable(t, h, st, lib)
	// Settle the compositor's initial manage burst first: hiding mid-manage
	// races its reparent/map dance (genuine server maps, not our Show).
	// Production hides long-managed windows; Visible=false covers create.
	settle := func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			if len(h.WaitEvents(100*time.Millisecond)) == 0 {
				return
			}
		}
	}
	settle()
	if err := ctl.Hide(); err != nil {
		t.Fatalf("Hide() = %v", err)
	}
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventHidden && e.Hidden {
				return true
			}
		}
		return false
	})
	if x11HasType(got, EventHidden) == nil {
		t.Fatalf("Hide produced no EventHidden{true}; saw %v", evTypes(got))
	}
	if ctl.IsVisible() {
		t.Fatal("IsVisible()=true after Hide()")
	}
	// A redundant Hide stays fully silent (signal-flagged, settled window).
	// A live compositor may interpose a genuine re-map of its own between
	// our calls (Hidden{false} clearing our signal): re-establish with a
	// fresh Hide and re-soak, bounded. A Hidden{true} in the soak is always
	// ours and always fatal.
	quiet := false
	for tries := 0; tries < 3 && !quiet; tries++ {
		if tries > 0 {
			t.Logf("re-establishing hidden state (try %d)", tries)
			x11Drain(h)
			if !x11HideFresh(h, ctl) {
				continue
			}
		}
		if err := ctl.Hide(); err != nil {
			t.Fatalf("redundant Hide() = %v", err)
		}
		quiet = true
		for _, e := range x11Collect(h, 400*time.Millisecond, nil) {
			if e.Type != EventHidden {
				continue
			}
			if e.Hidden {
				t.Fatalf("redundant Hide emitted %+v", e)
			}
			quiet = false // someone re-mapped us; re-establish below
		}
	}
	if !quiet {
		t.Fatal("compositor kept re-mapping the window; Hide silence unverifiable")
	}
	if err := ctl.Show(); err != nil {
		t.Fatalf("Show() = %v", err)
	}
	got = x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventHidden && !e.Hidden {
				return true
			}
		}
		return false
	})
	if e := x11HasType(got, EventHidden); e == nil || e.Hidden {
		t.Fatalf("Show produced no EventHidden{false}; saw %v", evTypes(got))
	}
	if !ctl.IsVisible() {
		t.Fatal("IsVisible()=false after Show()")
	}
	// Show's own MapNotify must not double-report (signal already cleared).
	for _, e := range x11Collect(h, 300*time.Millisecond, nil) {
		if e.Type == EventHidden {
			t.Fatalf("Show double-reported %+v", e)
		}
	}
	// External re-map of a hidden window (bypassing controller Show)
	// surfaces as Hidden{false} too. Wait until the pump has PROCESSED the
	// server's UnmapNotify for this Hide: IsVisible is a client flag flipped
	// synchronously inside Hide and proves nothing about the server — the
	// synthetic re-map below carries a later serial, so the causality gate
	// (event serial >= hideReq) holds exactly when the real unmap was
	// witnessed first. (Show clears the witness, so it cannot be stale from
	// an earlier cycle.)
	//
	// Establish hidden + server-witnessed-unmap, healing compositor
	// interpositions (bounded): the synthetic re-map below passes the
	// causality gate (event serial >= hideReq) exactly when the real unmap
	// for THIS Hide was witnessed. A try that loses the mapped state
	// server-side re-maps via Show first; Show's own push is drained so it
	// cannot pollute the later expects.
	witnessed := false
	for tries := 0; tries < 3 && !witnessed; tries++ {
		if tries > 0 {
			t.Logf("re-establishing mapped state (try %d)", tries)
			x11Drain(h)
			if err := ctl.Show(); err != nil {
				t.Fatalf("re-Show() = %v", err)
			}
			x11Drain(h)
		}
		if err := ctl.Hide(); err != nil {
			t.Fatalf("Hide() = %v", err)
		}
		x11Collect(h, 2*time.Second, func(ev []Event) bool {
			return x11HasType(ev, EventHidden) != nil
		})
		witnessed = x11WaitSoft(h, 1500*time.Millisecond, func() bool { return st.hideUnmapped })
	}
	if !witnessed {
		t.Fatal("server never witnessed our UnmapNotify after 3 tries; external re-map untestable")
	}
	mapEv := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&mapEv[0])) = 19 // MapNotify
	*(*uintptr)(unsafe.Pointer(&mapEv[32])) = st.window
	x11SendEvent(st, lib, mapEv)
	got = x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventHidden && !e.Hidden {
				return true
			}
		}
		return false
	})
	if e := x11HasType(got, EventHidden); e == nil || e.Hidden {
		t.Fatalf("external re-map produced no EventHidden{false}; saw %v", evTypes(got))
	}
	// Minimize never emits Hidden (no confusion between the paths).
	ctl.Minimize()
	for _, e := range x11Collect(h, 400*time.Millisecond, nil) {
		if e.Type == EventHidden {
			t.Fatalf("Minimize emitted Hidden: %+v", e)
		}
	}
}

func x11Button(st *x11State, typ int32, btn uint, x, y int32) []byte {
	ev := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&ev[0])) = typ // 4 press / 5 release
	*(*uintptr)(unsafe.Pointer(&ev[32])) = st.window
	*(*int32)(unsafe.Pointer(&ev[64])) = x
	*(*int32)(unsafe.Pointer(&ev[68])) = y
	*(*uint)(unsafe.Pointer(&ev[84])) = btn
	return ev
}

func TestX11TiltScrollSource(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)

	x11SendEvent(st, lib, x11Button(st, 4, 6, 5, 5))
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventPointer && e.Pointer == PointerScroll && e.ScrollX != 0 {
				return true
			}
		}
		return false
	})
	var sc *Event
	for i := range got {
		if got[i].Type == EventPointer && got[i].Pointer == PointerScroll && got[i].ScrollX != 0 {
			sc = &got[i]
		}
	}
	if sc == nil {
		t.Fatalf("button 6 press produced no horizontal scroll; saw %v", evTypes(got))
	}
	if sc.ScrollX != -1 {
		t.Fatalf("button 6 ScrollX = %v, want -1", sc.ScrollX)
	}
	// The release stays an ordinary Up (one tilt ticks once).
	x11SendEvent(st, lib, x11Button(st, 5, 7, 5, 5))
	got = x11Collect(h, 2*time.Second, func(ev []Event) bool {
		for _, e := range ev {
			if e.Type == EventPointer && e.Pointer == PointerUp && e.Button == 7 {
				return true
			}
		}
		return false
	})
	found := false
	for _, e := range got {
		if e.Type == EventPointer && e.Pointer == PointerUp && e.Button == 7 {
			found = true
		}
		if e.Type == EventPointer && e.Pointer == PointerScroll && e.ScrollX != 0 {
			t.Fatalf("button 7 release must not scroll: %+v", e)
		}
	}
	if !found {
		t.Fatalf("button 7 release lost; saw %v", evTypes(got))
	}
}

func x11Key(st *x11State, typ int32, keycode uint, tm uint64) []byte {
	ev := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&ev[0])) = typ // 2 press / 3 release
	*(*uintptr)(unsafe.Pointer(&ev[32])) = st.window
	*(*uint64)(unsafe.Pointer(&ev[56])) = tm
	*(*uint)(unsafe.Pointer(&ev[84])) = keycode
	return ev
}

func x11KeyEvents(got []Event) []Event {
	var out []Event
	for _, e := range got {
		if e.Type == EventKey {
			out = append(out, e)
		}
	}
	return out
}

func TestX11KeyRepeat(t *testing.T) {
	// NOTE: keys round-trip the live input method (fcitx5/ibus verdict decides
	// delivery), so do not run a nested compositor concurrently — its Xwayland
	// presence perturbs the outer engine into swallowing synthetic keys.
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)

	// Auto-repeat pair: release+press sharing keycode and timestamp collapses
	// into one Repeat press with no release.
	x11SendEvent(st, lib, x11Key(st, 3, 38, 0x5000))
	x11SendEvent(st, lib, x11Key(st, 2, 38, 0x5000))
	got := x11KeyEvents(x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return len(x11KeyEvents(ev)) >= 1
	}))
	// Drain a little more to catch a leaked release arriving late.
	time.Sleep(200 * time.Millisecond)
	got = append(got, x11KeyEvents(h.WaitEvents(0))...)
	if len(got) != 1 {
		t.Fatalf("repeat pair produced %d key events, want 1: %v", len(got), got)
	}
	if !got[0].Pressed || !got[0].Repeat {
		t.Fatalf("repeat press = %+v, want Pressed+Repeat", got[0])
	}
	// Plain pair (different timestamps) passes through untouched.
	x11SendEvent(st, lib, x11Key(st, 3, 38, 0x6000))
	x11SendEvent(st, lib, x11Key(st, 2, 38, 0x6001))
	got = x11KeyEvents(x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return len(x11KeyEvents(ev)) >= 2
	}))
	if len(got) != 2 {
		t.Fatalf("plain pair produced %d key events, want 2: %v", len(got), got)
	}
	if got[0].Pressed || got[0].Repeat {
		t.Fatalf("plain release = %+v", got[0])
	}
	if !got[1].Pressed || got[1].Repeat {
		t.Fatalf("plain press = %+v", got[1])
	}
}

func TestX11StateReadback(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)
	if lib.changeProperty == nil || lib.internAtom == nil {
		t.Skipf("property write unavailable")
	}
	if st.atNetState == 0 || st.atWMState == 0 {
		t.Skipf("state atoms unresolved")
	}
	atomName := func(s string) uintptr {
		b := append([]byte(s), 0)
		return lib.internAtom(st.display, &b[0], 0)
	}
	atomType, cardinalType := atomName("ATOM"), atomName("CARDINAL")
	setProp := func(prop, typ uintptr, vals []uint64) {
		var p unsafe.Pointer
		if len(vals) > 0 {
			p = unsafe.Pointer(&vals[0])
		} else {
			var dummy byte
			p = unsafe.Pointer(&dummy)
		}
		lib.changeProperty(st.display, st.window, prop, typ, 32, 0, p, len(vals))
		st.flush()
	}
	waitState := func(wantMin, wantMax, wantFull bool) Event {
		got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
			for _, e := range ev {
				if e.Type == EventStateChanged {
					return true
				}
			}
			return false
		})
		for _, e := range got {
			if e.Type == EventStateChanged {
				if e.Minimized != wantMin || e.Maximized != wantMax || e.Fullscreen != wantFull {
					t.Fatalf("state = min=%v max=%v full=%v, want %v %v %v",
						e.Minimized, e.Maximized, e.Fullscreen, wantMin, wantMax, wantFull)
				}
				return e
			}
		}
		t.Fatalf("no EventStateChanged (want min=%v max=%v full=%v); saw %v",
			wantMin, wantMax, wantFull, evTypes(got))
		return Event{}
	}
	// Maximize via the property (as a WM would): both atoms → maximized.
	setProp(st.atNetState, atomType, []uint64{uint64(st.atMaxV), uint64(st.atMaxH)})
	waitState(false, true, false)
	ctl, ok := win.Controls().(*x11Controller)
	if !ok {
		t.Skipf("no x11 controller")
	}
	if !ctl.IsMaximized() {
		t.Fatal("IsMaximized()=false after WM-state maximize (readback missing)")
	}
	// Clearing the atoms un-maximizes.
	setProp(st.atNetState, atomType, nil)
	waitState(false, false, false)
	if ctl.IsMaximized() {
		t.Fatal("IsMaximized()=true after atoms cleared")
	}
	// WM_STATE Iconic → minimized; Normal → restored.
	setProp(st.atWMState, cardinalType, []uint64{3, 0})
	waitState(true, false, false)
	setProp(st.atWMState, cardinalType, []uint64{1, 0})
	waitState(false, false, false)
	// An unrelated property change stays quiet.
	setProp(st.atNetName, atomType, []uint64{1})
	soak := x11Collect(h, 400*time.Millisecond, nil)
	for _, e := range soak {
		if e.Type == EventStateChanged {
			t.Fatalf("unrelated property emitted state: %+v", e)
		}
	}
}

func TestXI2ParseTouch(t *testing.T) {
	// No display needed: the parser is verified against the XIDeviceEvent
	// layout (XInput2.h) with crafted bytes.
	mkXI := func(detail uint32, x, y float64) []byte {
		buf := make([]byte, 120)
		binary.LittleEndian.PutUint32(buf[56:], detail)
		binary.LittleEndian.PutUint64(buf[104:], math.Float64bits(x))
		binary.LittleEndian.PutUint64(buf[112:], math.Float64bits(y))
		return buf
	}
	down, ok := parseXITouch(mkXI(5, 30.5, 40.25), xiTouchBegin)
	if !ok || down.Type != EventTouch || down.Pointer != PointerDown {
		t.Fatalf("begin = %+v ok=%v", down, ok)
	}
	if down.TouchID != 6 || down.X != 30.5 || down.Y != 40.25 {
		t.Fatalf("begin payload = id=%d (%.2f,%.2f), want id=6 (30.5,40.25)", down.TouchID, down.X, down.Y)
	}
	move, ok := parseXITouch(mkXI(5, 31, 41), xiTouchUpdate)
	if !ok || move.Pointer != PointerMove || move.TouchID != 6 {
		t.Fatalf("update = %+v ok=%v", move, ok)
	}
	up, ok := parseXITouch(mkXI(5, 0, 0), xiTouchEnd)
	if !ok || up.Pointer != PointerUp {
		t.Fatalf("end = %+v ok=%v", up, ok)
	}
	if _, ok := parseXITouch(mkXI(0, 0, 0), 99); ok {
		t.Fatal("unknown evtype must not parse")
	}
	if _, ok := parseXITouch(make([]byte, 64), xiTouchBegin); ok {
		t.Fatal("short buffer must not parse")
	}
}

func TestXI2ProbeGating(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, _ := x11P0Handles(t, win)
	// The Create-time probe result must be self-consistent: selected only
	// when the extension opcode resolved.
	if st.xiTouch != (st.xiMajor != 0) {
		t.Fatalf("xiTouch=%v xiMajor=%d inconsistent", st.xiTouch, st.xiMajor)
	}
	// Without touch hardware the pump stays quiet (no spurious touches).
	soak := x11Collect(h, 400*time.Millisecond, nil)
	for _, e := range soak {
		if e.Type == EventTouch {
			t.Fatalf("spurious touch without hardware: %+v", e)
		}
	}
	t.Logf("xi2 touch selected=%v (no hardware here; live-device verification needs a touchscreen)", st.xiTouch)
}

func x11ScaleEvents(got []Event) []Event {
	var out []Event
	for _, e := range got {
		if e.Type == EventScale {
			out = append(out, e)
		}
	}
	return out
}

func TestX11ParseDPI(t *testing.T) {
	// No display needed: the RESOURCE_MANAGER parser is verified directly.
	if v := x11ParseDPI("Xft.dpi:\t96\nfoo: 1\n"); v != 96 {
		t.Fatalf("tab form = %v, want 96", v)
	}
	if v := x11ParseDPI("foo: 1\nXft.dpi: 192.0\n"); v != 192 {
		t.Fatalf("fractional form = %v, want 192", v)
	}
	if v := x11ParseDPI(""); v != 0 {
		t.Fatalf("empty = %v, want 0", v)
	}
	if v := x11ParseDPI("Xft.hinting: 1\nXft.antialias: 1\n"); v != 0 {
		t.Fatalf("no dpi key = %v, want 0", v)
	}
	if v := x11ParseDPI("NotXft.dpi: 200\n"); v != 0 {
		t.Fatalf("mid-line match = %v, want 0 (line-start only)", v)
	}
	if v := x11ScaleFromDPI(192); v != 2 {
		t.Fatalf("192dpi = %v, want 2", v)
	}
	if v := x11ScaleFromDPI(96); v != 1 {
		t.Fatalf("96dpi = %v, want 1", v)
	}
	for _, bad := range []float64{0, -5, 96 * 4 * 2} {
		if v := x11ScaleFromDPI(bad); v != 1 {
			t.Fatalf("dpi %v = %v, want fallback 1", bad, v)
		}
	}
}

func TestX11ScaleRandRNotify(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)
	if !st.rrOK || st.rrBase == 0 {
		t.Skipf("RandR unavailable")
	}
	// The live reader must honor its range contract on any machine.
	live := x11ScaleSource(st)
	if live < 1 || live > 4 {
		t.Fatalf("live scale = %v, want within [1,4]", live)
	}
	seam := 2.0
	if live == 2 {
		seam = 1.5
	}
	// A screen-change notice with an unchanged scale stays fully quiet.
	notify := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&notify[0])) = int32(st.rrBase)
	for _, e := range x11ScaleEvents(x11Collect(h, 400*time.Millisecond, nil)) {
		t.Fatalf("unchanged scale emitted %+v", e)
	}
	x11SendEvent(st, lib, notify)
	for _, e := range x11ScaleEvents(x11Collect(h, 400*time.Millisecond, nil)) {
		t.Fatalf("unchanged scale emitted %+v", e)
	}
	// Pin the reader (never touches the live root resources): the same
	// notice shape now reports exactly one EventScale.
	prev := x11ScaleSource
	x11ScaleSource = func(*x11State) float64 { return seam }
	t.Cleanup(func() { x11ScaleSource = prev })
	x11SendEvent(st, lib, notify)
	got := x11ScaleEvents(x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return len(x11ScaleEvents(ev)) >= 1
	}))
	time.Sleep(200 * time.Millisecond)
	got = append(got, x11ScaleEvents(h.WaitEvents(0))...)
	if len(got) != 1 || got[0].Scale != seam {
		t.Fatalf("RandR notify produced %v, want one EventScale{%v}", got, seam)
	}
	if s := h.ScaleFactor(); s != seam {
		t.Fatalf("ScaleFactor()=%v after scale event, want %v", s, seam)
	}
	// Restoring the live reader reports the way back down (no stuck flag).
	x11ScaleSource = prev
	x11SendEvent(st, lib, notify)
	got = x11ScaleEvents(x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return len(x11ScaleEvents(ev)) >= 1
	}))
	if len(got) != 1 || got[0].Scale != live {
		t.Fatalf("restore notify produced %v, want one EventScale{%v}", got, live)
	}
	if s := h.ScaleFactor(); s != live {
		t.Fatalf("ScaleFactor()=%v after restore, want %v", s, live)
	}
}

func TestX11ScaleResourceManagerNotify(t *testing.T) {
	win := openTestX11(t)
	defer win.Close()
	h, st, lib := x11P0Handles(t, win)
	if st.atResManager == 0 || st.root == 0 {
		t.Skipf("RESOURCE_MANAGER atom unresolved")
	}
	live := x11ScaleSource(st)
	seam := 2.0
	if live == 2 {
		seam = 1.5
	}
	prev := x11ScaleSource
	x11ScaleSource = func(*x11State) float64 { return seam }
	t.Cleanup(func() { x11ScaleSource = prev })
	prop := make([]byte, 128)
	*(*int32)(unsafe.Pointer(&prop[0])) = xPropertyNotify
	*(*uintptr)(unsafe.Pointer(&prop[32])) = st.root
	*(*uintptr)(unsafe.Pointer(&prop[40])) = st.atResManager
	x11SendEvent(st, lib, prop)
	got := x11ScaleEvents(x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return len(x11ScaleEvents(ev)) >= 1
	}))
	if len(got) != 1 || got[0].Scale != seam {
		t.Fatalf("RESOURCE_MANAGER notify produced %v, want one EventScale{%v}", got, seam)
	}
	// Same atom on our own window (not the root) is not a desktop-scale
	// write: restore the live value first so any emission would stand out,
	// then stay quiet.
	x11ScaleSource = prev
	*(*uintptr)(unsafe.Pointer(&prop[32])) = st.window
	x11SendEvent(st, lib, prop)
	for _, e := range x11ScaleEvents(x11Collect(h, 400*time.Millisecond, nil)) {
		t.Fatalf("non-root RESOURCE_MANAGER emitted %+v", e)
	}
	// Back to the pinned scale for a clean handoff (Close must not leak it
	// into the next test before Cleanup runs — Cleanup restores anyway).
	if s := h.ScaleFactor(); s != seam {
		t.Fatalf("ScaleFactor()=%v, want pinned %v", s, seam)
	}
}

// x11AttrGet queries XWindowAttributes (test-only binding for server-truth
// gates the client flags cannot give).
var (
	x11AttrOnce sync.Once
	x11AttrLib  uintptr
	x11AttrGet  func(dpy, win uintptr, attr *byte) int
)

func x11AttrOK() bool {
	x11AttrOnce.Do(func() {
		lib, err := purego.Dlopen("libX11.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		if _, err := purego.Dlsym(lib, "XGetWindowAttributes"); err != nil {
			return
		}
		purego.RegisterLibFunc(&x11AttrGet, lib, "XGetWindowAttributes")
		x11AttrLib = lib
	})
	return x11AttrLib != 0 && x11AttrGet != nil
}

// x11MapStateOff = 92: XWindowAttributes.map_state on linux amd64
// (x@0 y@4 w@8 h@12 border@16 depth@20 visual@24 root@32 class@40
// bit_grav@44 win_grav@48 backing@52 planes@56 pixel@64 save_under@72
// colormap@80 map_installed@88 map_state@92). Validated live: a mapped
// window reads 2 (IsViewable), a hidden one 0 (IsUnmapped).
const x11MapStateOff = 92

const x11IsViewable = 2

func x11MapState(st *x11State) (int32, bool) {
	if st == nil || st.display == 0 || st.window == 0 || !x11AttrOK() {
		return 0, false
	}
	var attr [256]byte
	if x11AttrGet(st.display, st.window, &attr[0]) == 0 {
		return 0, false
	}
	return *(*int32)(unsafe.Pointer(&attr[x11MapStateOff])), true
}

// x11WaitViewable pumps until the server reports the window viewable before
// the caller touches focus or visibility. Two pre-existing live-desktop
// hazards motivate the gate: (1) focusing during Mutter's reparent/map dance
// makes X_SetInputFocus fail with BadMatch — fatal via the Xlib default
// handler (the process dies, no retry possible) — so the gate must precede
// Focus, not follow it; (2) hiding a window the compositor never mapped is
// a silent server no-op, so the UnmapNotify witness below would wait forever.
// Queue contents drained here are the caller's to account for (a spontaneous
// early FocusIn is real focus).
func x11WaitViewable(t *testing.T, h *x11Host, st *x11State, lib *x11CtlLib) {
	t.Helper()
	if !x11AttrOK() {
		t.Skipf("XGetWindowAttributes unavailable")
	}
	if x11WaitSeen(h, st, 3*time.Second) {
		return
	}
	// One re-map: rapid successive test windows can outrun the compositor
	// (focus-stealing throttle drops the first map); re-issuing the request
	// is harmless. Anything beyond that is the desktop's decision, not ours.
	if lib != nil && lib.ok() && lib.mapWindow != nil && st != nil && st.flush != nil {
		t.Logf("re-issuing map request (compositor may have dropped the first)")
		lib.mapWindow(st.display, st.window)
		st.flush()
		if x11WaitSeen(h, st, 2*time.Second) {
			return
		}
	}
	// Never viewable: proceeding would kill the test binary with BadMatch
	// (focus) or wedge the witness below (hide), so skip with the server's
	// own word attached instead.
	if ms, ok := x11MapState(st); ok {
		t.Skipf("window never became viewable (map_state=%d)", ms)
	}
	t.Skipf("window never became viewable (query failed)")
}

func x11WaitSeen(h *x11Host, st *x11State, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		x11Drain(h)
		if ms, ok := x11MapState(st); ok && ms == x11IsViewable {
			return true
		}
		h.WaitEvents(100 * time.Millisecond)
	}
	return false
}

// x11Drain swallows everything currently queued.
func x11Drain(h *x11Host) {
	for _, e := range h.WaitEvents(0) {
		_ = e
	}
}

// x11WaitSoft pumps until cond holds or timeout passes, reporting instead of
// failing (retry loops arbitrate; only the exhausted loop fails the test).
func x11WaitSoft(h *x11Host, timeout time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		h.WaitEvents(100 * time.Millisecond)
	}
	return cond()
}

// x11HideFresh hides and waits for the controller's own Hidden{true} push,
// reporting instead of failing (the push is controller-local; a miss usually
// means the queue was polluted by a compositor interposition — retry).
func x11HideFresh(h *x11Host, ctl *x11Controller) bool {
	if err := ctl.Hide(); err != nil {
		return false
	}
	got := x11Collect(h, 2*time.Second, func(ev []Event) bool {
		return x11HasType(ev, EventHidden) != nil
	})
	e := x11HasType(got, EventHidden)
	return e != nil && e.Hidden
}

func evTypes(got []Event) []string {
	var out []string
	for _, e := range got {
		out = append(out, e.Type.String())
	}
	return out
}
