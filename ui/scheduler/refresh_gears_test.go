package scheduler

import (
	"testing"
	"time"
)

// G期三档核验：60/120/140Hz 上报各有各的周期，学来的漂移覆盖不了上报，
// 换屏重报就跟过去，没上报才走学来的兜底。

// 三档上报 → 三档周期：60→16.67ms，120→8.33ms，140→7.14ms。
func TestGears_SeededBoundary(t *testing.T) {
	for _, tc := range []struct {
		hz   float64
		want time.Duration
	}{
		{60, 16666666 * time.Nanosecond},
		{59.88, 16700267 * time.Nanosecond},
		{120, 8333333 * time.Nanosecond},
		{140, 7142857 * time.Nanosecond},
	} {
		s := New()
		s.SeedDisplayRefreshHz(tc.hz)
		s.mu.Lock()
		got := s.boundaryPeriodLocked()
		s.mu.Unlock()
		if d := got - tc.want; d < -2*time.Microsecond || d > 2*time.Microsecond {
			t.Fatalf("hz=%v: boundary=%v want %v", tc.hz, got, tc.want)
		}
	}
}

// 上报赢过学来的漂移：种子 60Hz，合成器通知 16.9ms 稳来 40 次，
// 周期必须还是上报的 16.67ms（H 修的就是这 0.2ms/帧漂移）。
func TestGears_ReportedWinsOverLearnedDrift(t *testing.T) {
	s := New()
	s.SeedDisplayRefreshHz(60)
	for i := 0; i < 40; i++ {
		s.learnCompositorPeriod(16900 * time.Microsecond)
	}
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	learned := s.displayPeriod
	s.mu.Unlock()
	if learned <= 0 {
		t.Fatal("steady 16.9ms input did not learn (test setup broken)")
	}
	want := time.Second / 60
	if d := got - want; d < -2*time.Microsecond || d > 2*time.Microsecond {
		t.Fatalf("boundary=%v want reported %v (learned=%v must not win)", got, want, learned)
	}
}

// 硬件 vblank 学来的也不覆盖上报：种子 120Hz，DRM 量到 8.5ms，周期还是 8.33ms。
func TestGears_ReportedWinsOverDRMLearn(t *testing.T) {
	s := New()
	s.SeedDisplayRefreshHz(120)
	s.learnDisplayPeriod(8500 * time.Microsecond)
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	s.mu.Unlock()
	want := time.Second / 120
	if d := got - want; d < -2*time.Microsecond || d > 2*time.Microsecond {
		t.Fatalf("boundary=%v want reported %v", got, want)
	}
}

// 换屏重报就跟过去：60 → 120，周期从 16.67ms 切到 8.33ms。
func TestGears_ReseedFollowsOutputChange(t *testing.T) {
	s := New()
	s.SeedDisplayRefreshHz(60)
	s.SeedDisplayRefreshHz(120)
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	s.mu.Unlock()
	want := time.Second / 120
	if d := got - want; d < -2*time.Microsecond || d > 2*time.Microsecond {
		t.Fatalf("boundary=%v want %v after reseed to 120Hz", got, want)
	}
}

// 没上报才走学来的兜底：不播种，120Hz 稳通知 40 次，周期约 8.33ms。
func TestGears_UnseededFallsBackToLearned(t *testing.T) {
	s := New()
	for i := 0; i < 40; i++ {
		s.learnCompositorPeriod(8330 * time.Microsecond)
	}
	s.mu.Lock()
	got := s.boundaryPeriodLocked()
	s.mu.Unlock()
	if got < 8*time.Millisecond || got > 9*time.Millisecond {
		t.Fatalf("boundary=%v want ~8.33ms fallback with no seed", got)
	}
}

type dtRecorder struct{ dts []float64 }

func (t *dtRecorder) Tick(dt float64) bool {
	t.dts = append(t.dts, dt)
	return true
}

// Tick 步长跟着档位走：120Hz 种子下动画每步约 8.33ms，不是墙钟间隙。
func TestGears_TickDtFollowsGear(t *testing.T) {
	s := New()
	s.SeedDisplayRefreshHz(120)
	rec := &dtRecorder{}
	s.Tickers().Add(rec)
	s.Tick()
	if len(rec.dts) != 1 {
		t.Fatalf("ticks=%d want 1", len(rec.dts))
	}
	want := float64(time.Second) / 120 / float64(time.Second)
	if d := rec.dts[0] - want; d < -0.0005 || d > 0.0005 {
		t.Fatalf("dt=%v want ~%v (120Hz step)", rec.dts[0], want)
	}
}
