package embedder

import (
	"strings"
	"testing"
)

// TestFaultOOM_EnvParsing locks the GPUI_FAULT_OOM spec grammar: empty is
// inert, "after:count" arms, garbage degrades to inert (never crashes a
// production window over a typo).
func TestFaultOOM_EnvParsing(t *testing.T) {
	if f := newFaultOOMFromEnvWith(""); f.armed {
		t.Fatal("empty spec must stay disarmed")
	}
	f := newFaultOOMFromEnvWith("60:3")
	if !f.armed || f.after != 60 || f.count != 3 {
		t.Fatalf("armed=%v after=%d count=%d", f.armed, f.after, f.count)
	}
	if f := newFaultOOMFromEnvWith("garbage"); f.armed {
		t.Fatal("garbage spec must degrade to disarmed")
	}
	if f := newFaultOOMFromEnvWith("5:0"); f.armed {
		t.Fatal("non-positive count must degrade to disarmed")
	}
}

// TestFaultOOM_InjectStreak locks the injection window: before `after` no
// error, exactly `count` consecutive OOM errors, then none.
func TestFaultOOM_InjectStreak(t *testing.T) {
	f := newFaultOOMFromEnvWith("10:3")
	for i := int64(0); i < 10; i++ {
		if err := f.injectOOM(i); err != nil {
			t.Fatalf("submit %d injected early: %v", i, err)
		}
	}
	for i := int64(10); i < 13; i++ {
		err := f.injectOOM(i)
		if err == nil {
			t.Fatalf("submit %d missed injection", i)
		}
		if !strings.Contains(err.Error(), "out of memory") {
			t.Fatalf("injected error not OOM-class: %v", err)
		}
	}
	for i := int64(13); i < 20; i++ {
		if err := f.injectOOM(i); err != nil {
			t.Fatalf("submit %d injected past budget: %v", i, err)
		}
	}
}

// TestFaultOOM_FeedsRealExitChain is the X12 truth test (#11): the injected
// OOM errors fed through oomExit.Note must latch the human-readable reason
// at exactly the threshold, so Run exits instead of black-looping.
func TestFaultOOM_FeedsRealExitChain(t *testing.T) {
	f := newFaultOOMFromEnvWith("0:3")
	var oom oomExit
	exits := 0
	for i := int64(0); i < 5; i++ {
		err := f.injectOOM(i)
		if oom.note(err) {
			exits++
		}
	}
	if exits != 1 {
		t.Fatalf("exit fired %d times, want exactly 1 at threshold", exits)
	}
	err := oom.runErr()
	if err == nil {
		t.Fatal("RunErr must return the latched reason")
	}
	if !strings.Contains(err.Error(), "GPU out of memory persists across 3 frames") {
		t.Fatalf("latched reason off-spec: %v", err)
	}
}
