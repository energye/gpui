//go:build linux

package platform

import (
	"testing"
	"unsafe"
)

func newRefreshTestWin() (*wlWin, *wlHost) {
	w := &wlWin{}
	w.ensureOutputMaps()
	h := &wlHost{win: w}
	w.hostRef = h
	return w, h
}

func modeCB(st *wlOutputState, flags, refresh uintptr) {
	wlOutputModeCB(uintptr(unsafe.Pointer(st)), 0, flags, 0, 0, refresh)
}

// Only the current-mode event counts; refresh is mHz → Hz.
func TestWlOutputModeRefresh(t *testing.T) {
	w, h := newRefreshTestWin()
	st := &wlOutputState{win: w, name: 7, scale: 1}
	w.outputs[7] = st

	modeCB(st, 0x2, 120000) // non-current → ignored
	if got := h.DisplayRefreshHz(); got != 0 {
		t.Fatalf("non-current mode: got %v want 0", got)
	}
	modeCB(st, 0x1, 59940) // current 59.94Hz
	if got := h.DisplayRefreshHz(); got < 59.93 || got > 59.95 {
		t.Fatalf("current mode: got %v want 59.94", got)
	}
}

// Entered outputs win (max); pre-enter falls back to all bound outputs.
func TestWlOutputRefreshEnterFallback(t *testing.T) {
	w, h := newRefreshTestWin()
	s1 := &wlOutputState{win: w, name: 1, scale: 1}
	s2 := &wlOutputState{win: w, name: 2, scale: 1}
	w.outputs[1], w.outputs[2] = s1, s2
	modeCB(s1, 0x1, 60000)
	modeCB(s2, 0x1, 120000)

	// Pre-enter: max over all bound outputs.
	if got := h.DisplayRefreshHz(); got < 119.99 || got > 120.01 {
		t.Fatalf("pre-enter: got %v want 120", got)
	}
	// Entered 60Hz output only → 60.
	w.enteredOutputs[1] = true
	w.evaluateRefresh()
	if got := h.DisplayRefreshHz(); got < 59.99 || got > 60.01 {
		t.Fatalf("entered 60Hz: got %v want 60", got)
	}
	// Leave → back to fallback.
	delete(w.enteredOutputs, 1)
	w.evaluateRefresh()
	if got := h.DisplayRefreshHz(); got < 119.99 || got > 120.01 {
		t.Fatalf("after leave: got %v want 120", got)
	}
}
