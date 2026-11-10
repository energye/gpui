package scheduler_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

// TestMetrics_HitchPauseExcludesVerificationStalls pins the C4/P2.6 engine
// contract: verification readbacks (U21 snapshot double-shot) stall the raster
// frame past HitchThresholdMs, and the soak/jank budget must not charge those
// stalls to animation frames. PauseHitchCount/ResumeHitchCount scope the
// exclusion; the first interval noted after the final resume straddles the
// paused activity and is skipped exactly once.
func TestMetrics_HitchPauseExcludesVerificationStalls(t *testing.T) {
	s := scheduler.New().Metrics()
	cur := time.Now()
	note := func(ms int) {
		cur = cur.Add(time.Duration(ms) * time.Millisecond)
		s.NoteFrameInterval(cur)
	}

	// Baseline: one jank-sized stall counts.
	note(16)
	note(60)
	if got := s.Snapshot().HitchCount; got != 1 {
		t.Fatalf("baseline hitch_count=%d want 1", got)
	}

	// While paused: readback-sized stalls are not jank.
	s.PauseHitchCount()
	note(100)
	s.PauseHitchCount() // nesting is allowed (drain may recurse)
	note(90)
	s.ResumeHitchCount()
	if got := s.Snapshot().HitchCount; got != 1 {
		t.Fatalf("paused hitches counted: got %d want 1", got)
	}

	// Resume: the first post-resume interval straddles the paused activity
	// (drain runs before the present-completion sample) and skips once.
	s.ResumeHitchCount()
	note(80)
	if got := s.Snapshot().HitchCount; got != 1 {
		t.Fatalf("straddling interval counted: got %d want 1", got)
	}

	// Normal observation resumes: a further stall counts again.
	note(70)
	if got := s.Snapshot().HitchCount; got != 2 {
		t.Fatalf("post-resume stall not counted: got %d want 2", got)
	}

	// Unbalanced resume is a no-op (must not arm a spurious skip).
	s.ResumeHitchCount()
	s.ResumeHitchCount()
	note(16)
	note(75)
	if got := s.Snapshot().HitchCount; got != 3 {
		t.Fatalf("unbalanced resume changed counting: got %d want 3", got)
	}
}

// TestMetrics_HitchPauseKeepsIntervalHonest pins that pausing only scopes the
// jank counter: the stall still lands in the interval ring (max shows it), so
// the observability surface stays truthful while the budget stays meaningful.
// (Rare-stall percentile indexing is engine-existing behavior, out of scope.)
func TestMetrics_HitchPauseKeepsIntervalHonest(t *testing.T) {
	s := scheduler.New().Metrics()
	cur := time.Now()
	note := func(ms int) {
		cur = cur.Add(time.Duration(ms) * time.Millisecond)
		s.NoteFrameInterval(cur)
	}

	note(16)
	s.PauseHitchCount()
	note(100)
	s.ResumeHitchCount()
	note(16)

	snap := s.Snapshot()
	if snap.MaxFrameIntervalMs < 100 {
		t.Fatalf("max=%v want >=100 (stall must stay observable)", snap.MaxFrameIntervalMs)
	}
	if snap.AvgFrameIntervalMs < 40 {
		t.Fatalf("avg=%v want >=40 (stall must stay in interval stats)", snap.AvgFrameIntervalMs)
	}
	if got := snap.HitchCount; got != 0 {
		t.Fatalf("hitch_count=%d want 0", got)
	}
}
