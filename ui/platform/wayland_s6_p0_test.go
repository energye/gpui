//go:build linux

package platform

import (
	"testing"
	"unsafe"
)

// S6-P0 Wayland input/scale tests. The compositor-gated paths (bind, seat
// caps, output enumeration) need WAYLAND_DISPLAY and are covered by the B
// layer real-window suite; the decode/state logic below runs everywhere by
// driving the protocol callbacks directly with crafted proxies.

func wlFixed(v float64) uintptr { return uintptr(int32(v * 256)) }

func wlDrainPtr(w *wlWin) []Event {
	w.ptrMu.Lock()
	defer w.ptrMu.Unlock()
	out := append([]Event(nil), w.ptrEvents...)
	w.ptrEvents = nil
	return out
}

// wlTestWin registers a hand-built window the way production does (wlByPtr):
// objects reachable from a global must be treated as shared, so direct Go
// calls into uintptr-aliased callbacks observe every write — exactly what
// the compositor-driven path guarantees via C dispatch barriers.
func wlTestWin(t *testing.T, w *wlWin) {
	t.Helper()
	w.selfPtr = uintptr(unsafe.Pointer(w))
	wlMu.Lock()
	wlByPtr[w.selfPtr] = w
	wlMu.Unlock()
	t.Cleanup(func() {
		wlMu.Lock()
		delete(wlByPtr, w.selfPtr)
		wlMu.Unlock()
	})
}

func TestWaylandLeaveStampsPosition(t *testing.T) {
	w := &wlWin{}
	w.surface = 0x1001
	wlTestWin(t, w)
	st := &wlPointerState{win: w}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	// Mirror production wiring (seat caps path: w.ptr = w.bindPointer()):
	// the state must be reachable from the window, which itself is
	// registered in wlByPtr — otherwise the optimizer treats st as a purely
	// local value and may keep its fields cached across the uintptr-aliased
	// callbacks below.
	w.ptr = st

	wlPtrEnterCB(st.selfPtr, 0, 5, w.surface, wlFixed(100), wlFixed(50))
	wlPtrMotionCB(st.selfPtr, 0, 0, wlFixed(120), wlFixed(60))
	first := wlDrainPtr(w)
	if len(first) != 2 || first[0].Pointer != PointerEnter || first[0].X != 100 || first[0].Y != 50 {
		t.Fatalf("enter = %v", first)
	}
	if first[1].Pointer != PointerMove || first[1].X != 120 || first[1].Y != 60 {
		t.Fatalf("move = %v", first)
	}
	wlPtrLeaveCB(st.selfPtr, 0, 6, w.surface)
	st.resolveDeferredLeave()
	// Leave carries no coordinates on the wire — the last known position.
	got := wlDrainPtr(w)
	if len(got) != 1 || got[0].Pointer != PointerLeave || got[0].X != 120 || got[0].Y != 60 {
		t.Fatalf("leave = %v, want Leave(120,60)", got)
	}
	// Enter-then-leave without motion stamps the enter position.
	w2 := &wlWin{}
	w2.surface = 0x1002
	st2 := &wlPointerState{win: w2}
	st2.selfPtr = uintptr(unsafe.Pointer(st2))
	w2.ptr = st2
	wlPtrEnterCB(st2.selfPtr, 0, 7, w2.surface, wlFixed(10), wlFixed(20))
	if got := wlDrainPtr(w2); len(got) != 1 {
		t.Fatalf("enter = %v", got)
	}
	wlPtrLeaveCB(st2.selfPtr, 0, 8, w2.surface)
	st2.resolveDeferredLeave()
	got2 := wlDrainPtr(w2)
	if len(got2) != 1 || got2[0].Pointer != PointerLeave || got2[0].X != 10 || got2[0].Y != 20 {
		t.Fatalf("enter-leave = %v", got2)
	}
}

func TestWaylandStateChangedDerived(t *testing.T) {
	w := &wlWin{}
	wlTestWin(t, w)
	states := func() []Event {
		w.focusMu.Lock()
		defer w.focusMu.Unlock()
		var out []Event
		for _, e := range w.focusEvents {
			if e.Type == EventStateChanged {
				out = append(out, e)
			}
		}
		w.focusEvents = nil
		return out
	}
	cfg := func(s ...uint32) {
		arr := uintptr(0) // empty states array
		if len(s) > 0 {
			arr = wlStatesArr(s...)
		}
		wlTopConfigure(w.selfPtr, 0, 800, 600, arr)
	}
	// Initial configure: all-false triple, nothing to report.
	cfg()
	if got := states(); len(got) != 0 {
		t.Fatalf("initial configure emitted %v", got)
	}
	// Compositor maximizes: exactly one event.
	cfg(1)
	got := states()
	if len(got) != 1 || !got[0].Maximized || got[0].Minimized || got[0].Fullscreen {
		t.Fatalf("maximize = %v", got)
	}
	// Repeating the same configure stays quiet.
	cfg(1)
	if got := states(); len(got) != 0 {
		t.Fatalf("repeat configure emitted %v", got)
	}
	// Fullscreen joins: one event carrying both.
	cfg(1, 2)
	got = states()
	if len(got) != 1 || !got[0].Maximized || !got[0].Fullscreen {
		t.Fatalf("fullscreen = %v", got)
	}
	// States cleared: one all-false event.
	cfg()
	got = states()
	if len(got) != 1 || got[0].Maximized || got[0].Fullscreen || got[0].Minimized {
		t.Fatalf("restore = %v", got)
	}
	// Controller minimize has no configure behind it: the optimistic write
	// reports immediately, and a repeat stays quiet.
	c := &waylandController{h: &wlHost{win: w}}
	c.Minimize()
	got = states()
	if len(got) != 1 || !got[0].Minimized {
		t.Fatalf("minimize = %v", got)
	}
	c.Minimize()
	if got := states(); len(got) != 0 {
		t.Fatalf("repeat minimize emitted %v", got)
	}
	// Activated configure reverses the optimistic minimize: one event.
	cfg(4)
	got = states()
	if len(got) != 1 || got[0].Minimized {
		t.Fatalf("activated restore = %v", got)
	}
	if c.IsMinimized() {
		t.Fatal("IsMinimized()=true after activated configure")
	}
	// Controller maximize reports once; the compositor confirm is quiet.
	c.Maximize()
	got = states()
	if len(got) != 1 || !got[0].Maximized {
		t.Fatalf("controller maximize = %v", got)
	}
	cfg(1)
	if got := states(); len(got) != 0 {
		t.Fatalf("confirm configure emitted %v", got)
	}
	c.Unmaximize()
	got = states()
	if len(got) != 1 || got[0].Maximized {
		t.Fatalf("controller unmaximize = %v", got)
	}
	// Controller fullscreen round-trips the same way.
	c.SetFullscreen(true)
	got = states()
	if len(got) != 1 || !got[0].Fullscreen {
		t.Fatalf("controller fullscreen = %v", got)
	}
	c.SetFullscreen(false)
	got = states()
	if len(got) != 1 || got[0].Fullscreen {
		t.Fatalf("controller unfullscreen = %v", got)
	}
}

func TestWaylandRepeatSetsBit(t *testing.T) {
	// P0 "Repeat 必带": synthesized client-side repeats must carry Repeat,
	// while the initial press path leaves it clear (shared keysymEvent).
	w := &wlWin{}
	wlTestWin(t, w)
	drainKeys := func() []Event {
		w.keyMu.Lock()
		defer w.keyMu.Unlock()
		out := append([]Event(nil), w.keyEvents...)
		w.keyEvents = nil
		return out
	}
	st := &wlKeyboardState{win: w}
	defer st.cancelRepeat()
	if ev := st.keysymEvent(0x61, true); ev.Repeat {
		t.Fatalf("initial press carries Repeat: %+v", ev)
	}
	st.heldKC, st.heldKS = 38, 0x61
	st.fireRepeat()
	defer st.cancelRepeat()
	got := drainKeys()
	if len(got) != 1 || got[0].Type != EventKey || !got[0].Pressed || !got[0].Repeat {
		t.Fatalf("synthesized repeat = %v, want one pressed Repeat key", got)
	}
}

func TestWaylandOutputMapsInit(t *testing.T) {
	// Regression: bindOutputs on the real create path wrote outputsByProxy
	// before it was made, panicking against a live compositor (callback
	// tests hand-build their tables and never caught it).
	w := &wlWin{}
	w.ensureOutputMaps()
	if w.outputs == nil || w.enteredOutputs == nil || w.outputsByProxy == nil {
		t.Fatalf("tables still nil after ensure: %+v", w)
	}
	// Idempotent: populated tables are kept, not replaced.
	w.outputs[7] = &wlOutputState{name: 7}
	w.ensureOutputMaps()
	if w.outputs[7] == nil || w.outputs[7].name != 7 {
		t.Fatal("ensureOutputMaps replaced a populated table")
	}
	// Nil receiver is a no-op, not a panic.
	var nilWin *wlWin
	nilWin.ensureOutputMaps()
}

func TestWaylandTouchCallbacks(t *testing.T) {
	w := &wlWin{}
	w.surface = 0x2002
	st := &wlTouchState{win: w, points: make(map[uint32]wlTouchPoint)}
	st.selfPtr = uintptr(unsafe.Pointer(st))
	self := st.selfPtr

	wlTouchDownCB(self, 0, 1, 0, w.surface, 2, wlFixed(30), wlFixed(40))
	wlTouchDownCB(self, 0, 1, 0, 0x9999, 3, wlFixed(1), wlFixed(1)) // foreign — ignored
	wlTouchMotionCB(self, 0, 0, 2, wlFixed(35), wlFixed(45))
	wlTouchMotionCB(self, 0, 0, 9, wlFixed(0), wlFixed(0)) // unknown id — ignored
	wlTouchUpCB(self, 0, 0, 0, 2)
	wlTouchUpCB(self, 0, 0, 0, 9) // unknown id — ignored
	wlTouchDownCB(self, 0, 0, 0, w.surface, 4, wlFixed(7), wlFixed(8))
	wlTouchCancelCB(self, 0, 4)
	got := wlDrainPtr(w)
	want := []Event{
		{Type: EventTouch, Pointer: PointerDown, TouchID: 3, X: 30, Y: 40},
		{Type: EventTouch, Pointer: PointerMove, TouchID: 3, X: 35, Y: 45},
		{Type: EventTouch, Pointer: PointerUp, TouchID: 3, X: 35, Y: 45},
		{Type: EventTouch, Pointer: PointerDown, TouchID: 5, X: 7, Y: 8},
		{Type: EventTouch, Pointer: PointerCancel, TouchID: 5, X: 7, Y: 8},
	}
	if len(got) != len(want) {
		t.Fatalf("events = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Type != want[i].Type || got[i].Pointer != want[i].Pointer ||
			got[i].TouchID != want[i].TouchID || got[i].X != want[i].X || got[i].Y != want[i].Y {
			t.Fatalf("event %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if len(st.points) != 0 {
		t.Fatalf("live touches leaked: %v", st.points)
	}
}

func TestWaylandOutputScale(t *testing.T) {
	w := &wlWin{
		outputs:        map[uint32]*wlOutputState{7: {scale: 1}},
		outputsByProxy: map[uintptr]uint32{0x3003: 7},
		enteredOutputs: map[uint32]bool{},
	}
	h := &wlHost{win: w, scale: 1}
	w.hostRef = h
	w.outputs[7].win = w
	ost := w.outputs[7]
	ost.selfPtr = uintptr(unsafe.Pointer(ost))

	scales := func() []float64 {
		w.focusMu.Lock()
		defer w.focusMu.Unlock()
		var out []float64
		for _, e := range w.focusEvents {
			if e.Type == EventScale {
				out = append(out, e.Scale)
			}
		}
		w.focusEvents = nil
		return out
	}
	// Scale advertised while the surface is on no output: stored, silent.
	wlOutputScaleCB(ost.selfPtr, 0, 2)
	if ost.scale != 2 {
		t.Fatalf("output scale = %d, want 2", ost.scale)
	}
	if h.ScaleFactor() != 1 || len(scales()) != 0 {
		t.Fatalf("unentered scale must not surface: host=%v", h.ScaleFactor())
	}
	// Register for winFrom (surface enter/leave resolve the window by data).
	w.selfPtr = uintptr(unsafe.Pointer(w))
	wlMu.Lock()
	wlByPtr[w.selfPtr] = w
	wlMu.Unlock()
	defer func() {
		wlMu.Lock()
		delete(wlByPtr, w.selfPtr)
		wlMu.Unlock()
	}()
	// Entering the 2x output flips the host scale + queues EventScale.
	wlSurfaceEnterCB(w.selfPtr, w.surface, 0x3003)
	if h.ScaleFactor() != 2 {
		t.Fatalf("host scale = %v, want 2", h.ScaleFactor())
	}
	if s := scales(); len(s) != 1 || s[0] != 2 {
		t.Fatalf("scale events = %v, want [2]", s)
	}
	// Unknown outputs are ignored.
	wlSurfaceEnterCB(w.selfPtr, w.surface, 0x9999)
	if h.ScaleFactor() != 2 || len(scales()) != 0 {
		t.Fatalf("unknown output must not change scale")
	}
	// Leaving drops back to 1.
	wlSurfaceLeaveCB(w.selfPtr, w.surface, 0x3003)
	if h.ScaleFactor() != 1 {
		t.Fatalf("host scale = %v, want 1", h.ScaleFactor())
	}
	if s := scales(); len(s) != 1 || s[0] != 1 {
		t.Fatalf("scale events = %v, want [1]", s)
	}
}
