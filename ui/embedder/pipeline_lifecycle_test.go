package embedder

import (
	"testing"

	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

func newLifecycleApp(t *testing.T) *PipelineApp {
	t.Helper()
	host := platform.NewStubHost(64, 64)
	root := rendering.NewAbsoluteBox(64, 64)
	return NewPipelineApp(host, root, PipelineOptions{})
}

// TestHandleLifecycle_Occluded drives the unified occluded pair end to end:
// platform event → FromPlatform → handleLifecycle, asserting the same
// behavior the old platform-typed branch had (flag + frame demand).
func TestHandleLifecycle_Occluded(t *testing.T) {
	a := newLifecycleApp(t)
	a.ScheduleFrame()
	if !a.sched.Pending() {
		t.Fatal("ScheduleFrame must raise demand")
	}
	occ := input.FromPlatform(platform.Event{Type: platform.EventOccluded, Occluded: true}, input.Modifiers{})
	if !a.handleLifecycle(occ) {
		t.Fatal("KindOccluded must be consumed")
	}
	if !a.occluded.Load() {
		t.Fatal("occluded flag not latched")
	}
	if a.sched.Pending() {
		t.Fatal("occluded must clear pending demand")
	}
	vis := input.FromPlatform(platform.Event{Type: platform.EventOccluded}, input.Modifiers{})
	if !a.handleLifecycle(vis) {
		t.Fatal("visible must be consumed")
	}
	if a.occluded.Load() {
		t.Fatal("occluded flag not released")
	}
	if !a.sched.Pending() {
		t.Fatal("visible must resume demand")
	}
}

// TestHandleLifecycle_Hidden parks headless: hide latches the flag without a
// live loop or present target. Show needs a real window (step 5 真窗).
func TestHandleLifecycle_Hidden(t *testing.T) {
	a := newLifecycleApp(t)
	hid := input.FromPlatform(platform.Event{Type: platform.EventHidden, Hidden: true}, input.Modifiers{})
	if !a.handleLifecycle(hid) {
		t.Fatal("KindHidden must be consumed")
	}
	if !a.occluded.Load() {
		t.Fatal("hidden must latch occluded")
	}
}

// TestHandleLifecycle_FramePresented feeds pacing without demand effects.
func TestHandleLifecycle_FramePresented(t *testing.T) {
	a := newLifecycleApp(t)
	fp := input.FromPlatform(platform.Event{Type: platform.EventFramePresented}, input.Modifiers{})
	if !a.handleLifecycle(fp) {
		t.Fatal("KindFramePresented must be consumed")
	}
}

// TestHandleLifecycle_RejectsOthers pins the trio boundary: dedicated kinds
// belong to the router, not here.
func TestHandleLifecycle_RejectsOthers(t *testing.T) {
	a := newLifecycleApp(t)
	for _, ev := range []input.Event{
		{Kind: input.KindPointer},
		{Kind: input.KindResize, Width: 64, Height: 64, Scale: 1},
		{Kind: input.KindNone},
	} {
		if a.handleLifecycle(ev) {
			t.Fatalf("%s must not be consumed here", ev.Kind)
		}
	}
	if a.occluded.Load() {
		t.Fatal("rejected events must not touch the flag")
	}
}
