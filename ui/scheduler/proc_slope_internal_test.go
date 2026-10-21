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

// Steady-state M-RSS-SLOPE semantics: the warmup
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

// A 5s window cannot separate the startup ramp from a leak: the raw field is
// startup-inclusive, and the steady verdict must be explicitly invalid rather
// than reporting the ramp as a leak.
func TestRSSSlopeSteady_ShortRunInvalid(t *testing.T) {
	// 46MB → 264MB through 4s, then flat to 5s (same shape as a short window).
	tr := trackerWithSeries(5, func(t float64) int64 {
		if t < 4 {
			return 46000 + int64(218000*t/4)
		}
		return 264000
	})
	slope, warm, valid := tr.RSSSlopeSteadyKBPerMin()
	if valid {
		t.Fatalf("5s run must not be a steady leak verdict (slope=%.0f)", slope)
	}
	if warm != rssSteadyMinWarmupSec {
		t.Fatalf("warmup=%v want %v", warm, rssSteadyMinWarmupSec)
	}
	// The raw field stays honest: it still includes the ramp.
	if raw := tr.RSSSlopeKBPerMin(); raw < 100000 {
		t.Fatalf("raw 5s slope=%.0f want startup-inclusive >=100000", raw)
	}
}

// The documented minimum verdict length is 30s: max(25%,15s) warmup plus at
// least 15s of post-warmup samples. 29s must stay invalid.
func TestRSSSlopeSteady_30sBoundary(t *testing.T) {
	for _, tc := range []struct {
		elapsed   float64
		wantValid bool
	}{
		{30, true},
		{29, false},
	} {
		tr := trackerWithSeries(tc.elapsed, func(t float64) int64 {
			return 100000 + int64(2000*t)
		})
		slope, warm, valid := tr.RSSSlopeSteadyKBPerMin()
		if valid != tc.wantValid {
			t.Fatalf("%.0fs valid=%v want %v (slope=%.0f warm=%.0f)", tc.elapsed, valid, tc.wantValid, slope, warm)
		}
		if warm != rssSteadyMinWarmupSec {
			t.Fatalf("%.0fs warmup=%v want %v", tc.elapsed, warm, rssSteadyMinWarmupSec)
		}
		if tc.wantValid && math.Abs(slope-120000) > 12000 {
			t.Fatalf("30s leak slope=%.0f KB/min want ~120000", slope)
		}
	}
}

// A 60s run with a 10s ramp and a small steady creep: the steady verdict is
// valid and folds the ramp, while a real leak keeps its slope.
func TestRSSSlopeSteady_60sRampFolds(t *testing.T) {
	tr := trackerWithSeries(60, func(t float64) int64 {
		if t < 10 {
			return 46000 + int64(264000*t/10)
		}
		return 310000 + int64(85*(t-10))
	})
	slope, warm, valid := tr.RSSSlopeSteadyKBPerMin()
	if !valid {
		t.Fatal("60s run must be a valid steady verdict")
	}
	if warm != 15 {
		t.Fatalf("warmup=%v want 15", warm)
	}
	if math.Abs(slope) > 20000 {
		t.Fatalf("steady slope=%.0f KB/min want <20000", slope)
	}
}

func TestRSSSlopeSteady_LeakKeepsTrueSlope(t *testing.T) {
	tr := trackerWithSeries(60, func(t float64) int64 {
		return 100000 + int64(5000*t)
	})
	slope, _, valid := tr.RSSSlopeSteadyKBPerMin()
	if !valid {
		t.Fatal("60s leak must be valid")
	}
	if math.Abs(slope-300000) > 30000 {
		t.Fatalf("steady leak slope=%.0f KB/min want ~300000±10%%", slope)
	}
}

func TestRSSSlopeSteady_MidRunLeakStillCaught(t *testing.T) {
	tr := trackerWithSeries(60, func(t float64) int64 {
		if t < 30 {
			return 310000
		}
		return 310000 + int64(5000*(t-30))
	})
	slope, _, valid := tr.RSSSlopeSteadyKBPerMin()
	if !valid {
		t.Fatal("60s mid-run leak must be valid")
	}
	if slope < 100000 {
		t.Fatalf("steady mid-run leak slope=%.0f KB/min want >=100000", slope)
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
