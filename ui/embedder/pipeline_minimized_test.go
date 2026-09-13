package embedder

import (
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
)

func stateChangedInput(min, max, full bool) input.Event {
	return input.FromPlatform(platform.Event{
		Type:       platform.EventStateChanged,
		Minimized:  min,
		Maximized:  max,
		Fullscreen: full,
	}, input.Modifiers{})
}

// TestHandleLifecycle_MinimizedLatch pins the R17 stop: user iconify latches
// the minimized flag and clears frame demand; restore releases and resumes.
func TestHandleLifecycle_MinimizedLatch(t *testing.T) {
	a := newLifecycleApp(t)
	if a.Minimized() || a.Occluded() {
		t.Fatal("fresh app must report visible")
	}
	a.ScheduleFrame()
	if !a.sched.Pending() {
		t.Fatal("ScheduleFrame must raise demand")
	}
	if !a.handleLifecycle(stateChangedInput(true, false, false)) {
		t.Fatal("KindStateChanged must be consumed")
	}
	if !a.Minimized() {
		t.Fatal("minimized flag not latched")
	}
	if a.Occluded() {
		t.Fatal("minimized must not touch the occluded latch")
	}
	if a.sched.Pending() {
		t.Fatal("minimized must clear pending demand")
	}
	// Repeat iconify: transition guard keeps it a no-op (no demand raised).
	if !a.handleLifecycle(stateChangedInput(true, false, false)) {
		t.Fatal("repeated KindStateChanged must be consumed")
	}
	if a.sched.Pending() {
		t.Fatal("repeated iconify must not raise demand")
	}
	if !a.handleLifecycle(stateChangedInput(false, false, false)) {
		t.Fatal("restore must be consumed")
	}
	if a.Minimized() {
		t.Fatal("restore must release the minimized flag")
	}
	if !a.sched.Pending() {
		t.Fatal("restore must resume demand")
	}
}

// TestHandleLifecycle_StateChangedNonMinimized pins the transition guard: a
// maximize/fullscreen-only StateChanged raises no frame demand.
func TestHandleLifecycle_StateChangedNonMinimized(t *testing.T) {
	a := newLifecycleApp(t)
	if a.sched.Pending() {
		t.Fatal("fresh app must have no demand")
	}
	if !a.handleLifecycle(stateChangedInput(false, true, false)) {
		t.Fatal("maximize StateChanged must be consumed")
	}
	if a.Minimized() || a.Occluded() {
		t.Fatal("maximize must not latch any stop flag")
	}
	if a.sched.Pending() {
		t.Fatal("maximize-only StateChanged must not raise demand")
	}
}

// TestHandleLifecycle_OccludedMinimizedIndependent pins the two-flag split:
// restoring one stop while the other still holds must not resume rendering.
func TestHandleLifecycle_OccludedMinimizedIndependent(t *testing.T) {
	a := newLifecycleApp(t)
	occ := input.FromPlatform(platform.Event{Type: platform.EventOccluded, Occluded: true}, input.Modifiers{})
	if !a.handleLifecycle(occ) {
		t.Fatal("KindOccluded must be consumed")
	}
	// A StateChanged restore for a window that was never minimized must
	// not clear the obscured stop (e.g. maximize while covered).
	if !a.handleLifecycle(stateChangedInput(false, true, false)) {
		t.Fatal("KindStateChanged must be consumed")
	}
	if !a.Occluded() {
		t.Fatal("unrelated StateChanged must not clear the occluded latch")
	}
	if a.sched.Pending() {
		t.Fatal("occluded stop must hold while obscured")
	}
}
