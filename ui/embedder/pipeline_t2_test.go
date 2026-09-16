package embedder

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

// T2 D14 single-side budget: the shared template never mutates; each frame's
// clone is independent, so consecutive frames never share mutable ops/area.
func TestCloneFrameBudget_Independent(t *testing.T) {
	shared := &rendering.SaveLayerBudget{MaxOps: 1, MaxArea: 1000}
	if cloneFrameBudget(nil) != nil {
		t.Fatal("nil template must clone to nil")
	}
	a := cloneFrameBudget(shared)
	b := cloneFrameBudget(shared)
	if a == shared || b == shared || a == b {
		t.Fatal("clones must be fresh objects")
	}
	if !a.Allow(10, 10) {
		t.Fatal("first Allow on fresh clone must pass")
	}
	if a.Allow(10, 10) {
		t.Fatal("second Allow must hit MaxOps=1")
	}
	if !b.Allow(10, 10) {
		t.Fatal("sibling clone must be unaffected by a's consumption")
	}
	// Spending clones never writes back to the template.
	c := cloneFrameBudget(shared)
	if !c.Allow(10, 10) {
		t.Fatal("template must still allow after clones spent theirs")
	}
}

// T2 G9/D5: input stream numbers flow into frames; completion publishes the
// receipt; latency math counts completed minus input plus one.
func TestPipelineApp_InputLatencyFrames(t *testing.T) {
	var nilApp *PipelineApp
	nilApp.NoteInputEvent() // nil-safe
	if nilApp.InputLatencyFrames() != 0 || nilApp.CompletedFrameID() != 0 || nilApp.SubmittedCount() != 0 {
		t.Fatal("nil app must report zeros")
	}
	if p, q := nilApp.PendingCoalesceDrops(); p != 0 || q != 0 {
		t.Fatal("nil app must report zero drops")
	}

	a := &PipelineApp{}
	if got := a.InputLatencyFrames(); got != 0 {
		t.Fatalf("no input yet, latency=%d want 0", got)
	}
	a.frameID.Store(10)
	a.NoteInputEvent()
	a.NoteInputEvent()
	if got := a.InputLatencyFrames(); got != 0 {
		t.Fatalf("no completion yet, latency=%d want 0", got)
	}
	// Input arrived before frame 11; completing 11 means 1 frame of latency.
	a.completedFrame.Store(11)
	if got := a.InputLatencyFrames(); got != 1 {
		t.Fatalf("latency=%d want 1", got)
	}
	a.completedFrame.Store(12)
	if got := a.InputLatencyFrames(); got != 2 {
		t.Fatalf("latency=%d want 2 (T2 door: point-click stays within 2)", got)
	}
	if got := a.CompletedFrameID(); got != 12 {
		t.Fatalf("completed=%d want 12", got)
	}
}

// T2 honesty counters: submissions and coalesce drops accumulate monotonically.
func TestPipelineApp_SubmitCounters(t *testing.T) {
	a := &PipelineApp{}
	a.submitted.Add(3)
	a.coalesceDrop.Add(1)
	a.preDrop.Add(2)
	if got := a.SubmittedCount(); got != 3 {
		t.Fatalf("submitted=%d want 3", got)
	}
	if p, q := a.PendingCoalesceDrops(); p != 1 || q != 2 {
		t.Fatalf("drops=(%d,%d) want (1,2)", p, q)
	}
}
