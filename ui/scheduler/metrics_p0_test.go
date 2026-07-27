package scheduler_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

func TestMetrics_PaintCountAndPercentiles(t *testing.T) {
	s := scheduler.New().Metrics()
	s.SetPaintCount(42)
	s.SetLayoutCount(7)
	s.SetVSyncSource("fallback")
	base := time.Now()
	for i := 0; i < 20; i++ {
		s.NoteFrameInterval(base.Add(time.Duration(i*16) * time.Millisecond))
	}
	snap := s.Snapshot()
	if snap.PaintCount != 42 || snap.LayoutCount != 7 {
		t.Fatalf("counts paint=%d layout=%d", snap.PaintCount, snap.LayoutCount)
	}
	if snap.VSyncSource != "fallback" {
		t.Fatalf("vsync=%q", snap.VSyncSource)
	}
	if snap.P50FrameIntervalMs <= 0 || snap.P99FrameIntervalMs <= 0 {
		t.Fatalf("percentiles p50=%v p99=%v", snap.P50FrameIntervalMs, snap.P99FrameIntervalMs)
	}
	// steady 16ms → p50 near 16
	if snap.P50FrameIntervalMs < 10 || snap.P50FrameIntervalMs > 25 {
		t.Fatalf("p50 unexpected %v", snap.P50FrameIntervalMs)
	}
}
