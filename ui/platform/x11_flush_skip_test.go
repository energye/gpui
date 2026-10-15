package platform

import "testing"

// Steady 60fps sends no X requests from the event thread: flushXIfNeeded
// must be a Go-side no-op (no XNextRequest trampoline per wake). A dirty
// flag set by an event-thread X write flushes exactly once.
func TestX11_FlushSkipOnUnchangedSerial(t *testing.T) {
	calls := 0
	st := &x11State{
		flush: func() { calls++ },
	}
	st.flushXIfNeeded()
	if calls != 0 {
		t.Fatalf("clean calls = %d, want 0 (skip)", calls)
	}
	st.flushDirty = true
	st.flushXIfNeeded()
	if calls != 1 {
		t.Fatalf("dirty calls = %d, want 1", calls)
	}
	st.flushXIfNeeded()
	if calls != 1 {
		t.Fatalf("after flush calls = %d, want 1 (flag cleared)", calls)
	}
	// Nil-safe: must not panic.
	var nilSt *x11State
	nilSt.flushXIfNeeded()
	(&x11State{}).flushXIfNeeded()
}
