package scheduler

import (
	"math"
	"testing"
)

// white-box tracker with a synthetic decimated series (no /proc dependency).
func trackerWithSeries(elapsed float64, series func(t float64) int64) *ProcessTracker {
	tr := &ProcessTracker{started: true}
	tr.ElapsedSec = elapsed
	tr.StartRSSKB = series(0)
	tr.EndRSSKB = series(elapsed)
	for t := 0.0; t <= elapsed; t += 1.0 {
		tr.rssT = append(tr.rssT, t)
		tr.rssV = append(tr.rssV, series(t))
	}
	return tr
}

// Steady-state M-RSS-SLOPE semantics (R7 RSS gate honesty): the warmup
// equilibrium ramp is discarded; a plateau folds to ~0.
func TestRSSSlope_SteadyStateFoldsWarmupRamp(t *testing.T) {
	// 60s run: 46MB→310MB over the first 10s, then ~85KB/s creep.
	tr := trackerWithSeries(60, func(t float64) int64 {
		if t < 10 {
			return 46000 + int64(264000*t/10)
		}
		return 310000 + int64(85*(t-10))
	})
	got := math.Abs(tr.RSSSlopeKBPerMin())
	// Two-point (end-start)/elapsed would report ~266000 KB/min for this curve.
	if got > 20000 {
		t.Fatalf("steady-state slope=%.0f KB/min want <20000", got)
	}
}

func TestRSSSlope_LeakKeepsTrueSlope(t *testing.T) {
	// Sustained 5MB/s leak across the whole run keeps its ~300000 KB/min slope.
	tr := trackerWithSeries(60, func(t float64) int64 {
		return 100000 + int64(5000*t)
	})
	if got := tr.RSSSlopeKBPerMin(); math.Abs(got-300000) > 30000 {
		t.Fatalf("leak slope=%.0f KB/min want ~300000±10%%", got)
	}
}

func TestRSSSlope_MidRunLeakStillCaught(t *testing.T) {
	// Leak starting at t=30s: the steady-state segment [15,60] must still
	// report a large slope (diluted but far above any plateau).
	tr := trackerWithSeries(60, func(t float64) int64 {
		if t < 30 {
			return 310000
		}
		return 310000 + int64(5000*(t-30))
	})
	got := tr.RSSSlopeKBPerMin()
	if got < 100000 {
		t.Fatalf("mid-run leak slope=%.0f KB/min want >=100000", got)
	}
}

// Least-squares math edge cases.
func TestRSSSlopeLSQ_Degenerate(t *testing.T) {
	if got := rssSlopeLSQ([]float64{1}, []int64{100}); got != 0 {
		t.Fatalf("single point slope=%v want 0", got)
	}
	if got := rssSlopeLSQ([]float64{1, 1, 1}, []int64{100, 100, 100}); got != 0 {
		t.Fatalf("zero-variance t slope=%v want 0", got)
	}
}
