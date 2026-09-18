package scheduler_test

import (
	"testing"

	"github.com/energye/gpui/ui/scheduler"
)

// TestPresentModeAge tracks the R3-6 age companion: real presents stamp the
// mode time, idle preserves value without refreshing it, snapshots report a
// non-negative age.
func TestPresentModeAge(t *testing.T) {
	m := scheduler.New().Metrics()
	if got := m.Snapshot().PresentModeAgeSec; got != 0 {
		t.Fatalf("fresh age=%v want 0", got)
	}
	m.NotePresentOutcome("damage_multi", 100)
	snap := m.Snapshot()
	if snap.PresentMode != "damage_multi" {
		t.Fatalf("mode=%q want damage_multi", snap.PresentMode)
	}
	if snap.PresentModeAgeSec < 0 {
		t.Fatalf("age=%v want >=0", snap.PresentModeAgeSec)
	}
	m.NotePresentOutcome("idle", 0)
	snap2 := m.Snapshot()
	if snap2.PresentMode != "damage_multi" {
		t.Fatalf("idle must preserve mode, got %q", snap2.PresentMode)
	}
	if snap2.PresentModeAgeSec < snap.PresentModeAgeSec {
		t.Fatal("idle must not refresh the age")
	}
}
