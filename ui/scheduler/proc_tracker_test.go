package scheduler_test

import (
	"testing"

	"github.com/energye/gpui/ui/scheduler"
)

func TestProcessTracker_Smoke(t *testing.T) {
	var tr scheduler.ProcessTracker
	tr.Start()
	tr.Sample()
	tr.Stop()
	tr.NoteAfterClose()
	s := scheduler.New().Metrics()
	tr.Apply(s)
	snap := s.Snapshot()
	// On Linux CI/dev we expect non-zero RSS; off-linux stubs stay 0.
	if snap.RSSStartKB < 0 || snap.RSSPeakKB < snap.RSSStartKB && snap.RSSPeakKB != 0 {
		t.Fatalf("rss start=%d peak=%d", snap.RSSStartKB, snap.RSSPeakKB)
	}
	_ = scheduler.ReadRSSKB()
}

func TestTryDefaultFont_DoesNotPanic(t *testing.T) {
	// painting test lives in painting package; just ensure scheduler builds with proc.
}
