package scheduler_test

import (
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

func TestMetrics_PresentPolicy_JSONKey(t *testing.T) {
	s := scheduler.New().Metrics()
	if s.PresentPolicy() != "" {
		t.Fatalf("fresh store policy=%q want empty", s.PresentPolicy())
	}
	s.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	if s.PresentPolicy() != scheduler.PresentPolicyFullPaint {
		t.Fatalf("policy=%q", s.PresentPolicy())
	}
	snap := s.Snapshot()
	if snap.PresentPolicy != scheduler.PresentPolicyFullPaint {
		t.Fatalf("snapshot policy=%q", snap.PresentPolicy)
	}
	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !containsAll(js, `"present_policy"`) || !containsAll(js, `full_paint`) {
		t.Fatalf("JSON missing present_policy full_paint: %s", js)
	}
}

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
	if snap.P50FrameIntervalMs <= 0 || snap.P95FrameIntervalMs <= 0 || snap.P99FrameIntervalMs <= 0 {
		t.Fatalf("percentiles p50=%v p95=%v p99=%v", snap.P50FrameIntervalMs, snap.P95FrameIntervalMs, snap.P99FrameIntervalMs)
	}
	// steady 16ms → p50 near 16
	if snap.P50FrameIntervalMs < 10 || snap.P50FrameIntervalMs > 25 {
		t.Fatalf("p50 unexpected %v", snap.P50FrameIntervalMs)
	}
	if snap.P95FrameIntervalMs < snap.P50FrameIntervalMs-0.01 {
		t.Fatalf("p95=%v < p50=%v", snap.P95FrameIntervalMs, snap.P50FrameIntervalMs)
	}
	if snap.P99FrameIntervalMs < snap.P95FrameIntervalMs-0.01 {
		t.Fatalf("p99=%v < p95=%v", snap.P99FrameIntervalMs, snap.P95FrameIntervalMs)
	}
}

func TestMetrics_P95BetweenP50AndP99(t *testing.T) {
	s := scheduler.New().Metrics()
	base := time.Now()
	// Build a skewed distribution: mostly 10ms, some 20ms, few 40ms, one 80ms.
	// NoteFrameInterval needs pairs: call at cumulative times.
	tAt := base
	s.NoteFrameInterval(tAt)
	add := func(ms int) {
		tAt = tAt.Add(time.Duration(ms) * time.Millisecond)
		s.NoteFrameInterval(tAt)
	}
	for i := 0; i < 80; i++ {
		add(10)
	}
	for i := 0; i < 15; i++ {
		add(20)
	}
	for i := 0; i < 4; i++ {
		add(40)
	}
	add(80)

	snap := s.Snapshot()
	if snap.P50FrameIntervalMs <= 0 || snap.P95FrameIntervalMs <= 0 || snap.P99FrameIntervalMs <= 0 {
		t.Fatalf("zero percentile: %+v", snap)
	}
	if !(snap.P50FrameIntervalMs <= snap.P95FrameIntervalMs+0.01 && snap.P95FrameIntervalMs <= snap.P99FrameIntervalMs+0.01) {
		t.Fatalf("order p50=%v p95=%v p99=%v", snap.P50FrameIntervalMs, snap.P95FrameIntervalMs, snap.P99FrameIntervalMs)
	}
	// p50 should be near the dense 10ms cluster
	if snap.P50FrameIntervalMs < 8 || snap.P50FrameIntervalMs > 22 {
		t.Fatalf("p50=%v want ~10–20", snap.P50FrameIntervalMs)
	}
	// p95 should be clearly above p50 given the tail
	if snap.P95FrameIntervalMs < snap.P50FrameIntervalMs {
		t.Fatalf("p95=%v not above p50", snap.P95FrameIntervalMs)
	}

	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !containsAll(js, `"frame_interval_p95_ms"`) {
		t.Fatalf("JSON missing p95 key: %s", js)
	}
}

func TestMetrics_HitchRatePerMin(t *testing.T) {
	s := scheduler.New().Metrics()
	base := time.Now()
	s.NoteFrameInterval(base)
	// 30s later: clear hitch (>33.4ms)
	s.NoteFrameInterval(base.Add(50 * time.Millisecond))
	// Advance wall span to 30 seconds total from first note
	s.NoteFrameInterval(base.Add(30 * time.Second))

	snap := s.Snapshot()
	if snap.HitchCount < 1 {
		t.Fatalf("hitch_count=%d want ≥1", snap.HitchCount)
	}
	// ~30s wall → 0.5 min; 1 hitch → ~2 per minute (exact depends on hitch count from 50ms + maybe second interval)
	if snap.HitchRatePerMin <= 0 {
		t.Fatalf("hitch_rate_per_min=%v want >0 (hitches=%d)", snap.HitchRatePerMin, snap.HitchCount)
	}
	// Sanity: rate = count / minutes ≈ count / 0.5
	minRate := float64(snap.HitchCount) / 0.6 // allow clock skew band
	maxRate := float64(snap.HitchCount) / 0.4
	if snap.HitchRatePerMin < minRate*0.5 || snap.HitchRatePerMin > maxRate*2 {
		// loose band — primary gate is >0 with positive hitch+elapsed
		t.Logf("hitch_rate_per_min=%v count=%d (informational band)", snap.HitchRatePerMin, snap.HitchCount)
	}

	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(b), `"hitch_rate_per_min"`) {
		t.Fatalf("JSON missing hitch_rate_per_min: %s", b)
	}
}

func TestMetrics_HitchRateZeroWithoutHitch(t *testing.T) {
	s := scheduler.New().Metrics()
	base := time.Now()
	for i := 0; i < 10; i++ {
		s.NoteFrameInterval(base.Add(time.Duration(i*16) * time.Millisecond))
	}
	snap := s.Snapshot()
	if snap.HitchCount != 0 {
		t.Fatalf("hitch_count=%d", snap.HitchCount)
	}
	if snap.HitchRatePerMin != 0 {
		t.Fatalf("rate=%v want 0", snap.HitchRatePerMin)
	}
}

func containsAll(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestMetrics_PathCPU_DivergesWhenOneSided(t *testing.T) {
	s := scheduler.New().Metrics()
	// Only UI build work → cpu_ui high, cpu_raster ~0 (work-share mode, no process CPU).
	for i := 0; i < 10; i++ {
		s.NoteBuildMs(5)
	}
	snap := s.Snapshot()
	if snap.CPUUIPct < 99 {
		t.Fatalf("UI-only: cpu_ui_pct=%v want ~100", snap.CPUUIPct)
	}
	if snap.CPURasterPct > 1 {
		t.Fatalf("UI-only: cpu_raster_pct=%v want ~0", snap.CPURasterPct)
	}
	if s.BuildSumMs() < 49 || s.RasterSumMs() != 0 {
		t.Fatalf("sums build=%v raster=%v", s.BuildSumMs(), s.RasterSumMs())
	}

	// Feed heavy raster → both non-zero and raster dominates.
	for i := 0; i < 10; i++ {
		s.NoteRasterMs(20)
	}
	snap = s.Snapshot()
	if snap.CPUUIPct <= 0 || snap.CPURasterPct <= 0 {
		t.Fatalf("both sides: ui=%v raster=%v", snap.CPUUIPct, snap.CPURasterPct)
	}
	if snap.CPURasterPct <= snap.CPUUIPct {
		t.Fatalf("raster should dominate: ui=%v raster=%v", snap.CPUUIPct, snap.CPURasterPct)
	}
	// Shares should sum to ~100 in work-share mode.
	sum := snap.CPUUIPct + snap.CPURasterPct
	if sum < 99 || sum > 101 {
		t.Fatalf("work-share sum=%v want ~100", sum)
	}

	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !containsAll(js, `"cpu_ui_pct"`) || !containsAll(js, `"cpu_raster_pct"`) {
		t.Fatalf("JSON missing path CPU keys: %s", js)
	}
}

func TestMetrics_PathCPU_ScalesWithProcessCPU(t *testing.T) {
	s := scheduler.New().Metrics()
	s.NoteBuildMs(10)
	s.NoteRasterMs(30)                // 25% UI / 75% raster of work
	s.SetProcessStats(0, 0, 0, 0, 40) // process 40%
	snap := s.Snapshot()
	// Expect ~10 and ~30 (40 * 0.25 / 0.75)
	if snap.CPUUIPct < 8 || snap.CPUUIPct > 12 {
		t.Fatalf("cpu_ui_pct=%v want ~10", snap.CPUUIPct)
	}
	if snap.CPURasterPct < 28 || snap.CPURasterPct > 32 {
		t.Fatalf("cpu_raster_pct=%v want ~30", snap.CPURasterPct)
	}
	if snap.CPUUIPct == snap.CPURasterPct {
		t.Fatal("UI and Raster must diverge under uneven work")
	}
}

func TestMetrics_PathCPU_ZeroWithoutNotes(t *testing.T) {
	s := scheduler.New().Metrics()
	snap := s.Snapshot()
	if snap.CPUUIPct != 0 || snap.CPURasterPct != 0 {
		t.Fatalf("unavailable must be zero: ui=%v raster=%v", snap.CPUUIPct, snap.CPURasterPct)
	}
}
