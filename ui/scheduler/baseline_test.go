package scheduler_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

func TestMetrics_RSSSlope_FromProcessTracker(t *testing.T) {
	var tr scheduler.ProcessTracker
	tr.Start() // sets started=true
	// Force known RSS endpoints via exported fields (proc may be stub/noisy).
	tr.StartRSSKB = 1000
	tr.EndRSSKB = 1600
	tr.PeakRSSKB = 1600
	// Simulate ~60s wall so slope = 600 KB / 1 min = 600.
	tr.ElapsedSec = 60

	s := scheduler.New().Metrics()
	slope := tr.RSSSlopeKBPerMin()
	if slope < 599 || slope > 601 {
		t.Fatalf("RSSSlopeKBPerMin=%v want ~600 (600KB / 1min)", slope)
	}
	tr.Apply(s)
	snap := s.Snapshot()
	if snap.RSSSlopeKBPerMin < 599 || snap.RSSSlopeKBPerMin > 601 {
		t.Fatalf("json slope=%v want ~600", snap.RSSSlopeKBPerMin)
	}
	if snap.RSSElapsedSec != 60 {
		t.Fatalf("rss_elapsed_sec=%v want 60", snap.RSSElapsedSec)
	}
	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if !containsAll(string(b), `"rss_slope_kb_per_min"`) {
		t.Fatalf("JSON missing slope key: %s", b)
	}
}

func TestMetrics_RSSSlope_ZeroWhenNoElapsed(t *testing.T) {
	var tr scheduler.ProcessTracker
	// Not started → 0
	if tr.RSSSlopeKBPerMin() != 0 {
		t.Fatal("unstarted slope must be 0")
	}
	s := scheduler.New().Metrics()
	s.SetRSSSlope(12, 0)
	if s.Snapshot().RSSSlopeKBPerMin != 0 {
		t.Fatal("elapsedSec≤0 must clear slope")
	}
}

func TestMetrics_NoteGPUPathStats_JSON(t *testing.T) {
	s := scheduler.New().Metrics()
	s.NoteGPUPathStats(100, 3, 2, "no_accelerator")
	snap := s.Snapshot()
	if snap.GPUOps != 100 || snap.CPUFallbackOps != 3 || snap.FrameFlushes != 2 {
		t.Fatalf("gpu stats %+v", snap)
	}
	if snap.LastCPUFallbackReason != "no_accelerator" {
		t.Fatalf("reason=%q", snap.LastCPUFallbackReason)
	}
	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, key := range []string{`"gpu_ops"`, `"cpu_fallback_ops"`, `"frame_flushes"`, `"last_cpu_fallback"`} {
		if !containsAll(js, key) {
			t.Fatalf("JSON missing %s: %s", key, js)
		}
	}
}

func TestCompareToBaseline_PassAndFail(t *testing.T) {
	base := scheduler.FrameMetrics{
		HitchRatePerMin:    1,
		P95FrameIntervalMs: 16,
		RSSSlopeKBPerMin:   100,
		CPUFallbackOps:     0,
		HitchCount:         2,
	}
	curOK := base
	curOK.HitchRatePerMin = 1.5 // within +5
	curOK.P95FrameIntervalMs = 18
	tol := scheduler.DefaultBaselineTolerance()
	d := scheduler.CompareToBaseline(curOK, base, tol)
	if !d.OK {
		t.Fatalf("expected pass, failures=%v", d.Failures)
	}

	curBad := base
	curBad.HitchRatePerMin = 20 // +19 > 5
	curBad.CPUFallbackOps = 100
	d2 := scheduler.CompareToBaseline(curBad, base, tol)
	if d2.OK {
		t.Fatal("expected fail")
	}
	if len(d2.Failures) < 1 {
		t.Fatal("want failure messages")
	}
	// Improvement always passes even if large negative.
	curBetter := base
	curBetter.HitchRatePerMin = 0
	curBetter.P95FrameIntervalMs = 10
	d3 := scheduler.CompareToBaseline(curBetter, base, tol)
	if !d3.OK {
		t.Fatalf("improvement must pass: %v", d3.Failures)
	}
}

func TestBaseline_SaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")
	m := scheduler.FrameMetrics{
		FrameCount:         100,
		HitchCount:         2,
		HitchRatePerMin:    1.5,
		P95FrameIntervalMs: 17.2,
		RSSSlopeKBPerMin:   50,
		GPUOps:             900,
		CPUFallbackOps:     1,
		VSyncSource:        "fallback",
	}
	if err := scheduler.SaveBaseline(path, m); err != nil {
		t.Fatal(err)
	}
	got, err := scheduler.LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.FrameCount != m.FrameCount || got.GPUOps != m.GPUOps {
		t.Fatalf("roundtrip %+v vs %+v", got, m)
	}
	if !scheduler.NearlyEqual(got.P95FrameIntervalMs, m.P95FrameIntervalMs, 1e-9) {
		t.Fatalf("p95 %v vs %v", got.P95FrameIntervalMs, m.P95FrameIntervalMs)
	}
	// Missing file errors.
	if _, err := scheduler.LoadBaseline(filepath.Join(dir, "nope.json")); err == nil {
		t.Fatal("want error for missing file")
	}
	_ = os.Remove(path)
}

func TestProcessTracker_ElapsedAndSlopeSmoke(t *testing.T) {
	var tr scheduler.ProcessTracker
	tr.Start()
	time.Sleep(20 * time.Millisecond)
	tr.Sample()
	tr.Stop()
	if tr.ElapsedSec <= 0 {
		t.Fatal("ElapsedSec must advance after Sample")
	}
	// On Linux RSS usually non-zero; slope may be 0 if unchanged — just ensure Apply is safe.
	s := scheduler.New().Metrics()
	tr.Apply(s)
	snap := s.Snapshot()
	if snap.RSSElapsedSec <= 0 {
		t.Fatal("Apply must set rss_elapsed_sec")
	}
}
