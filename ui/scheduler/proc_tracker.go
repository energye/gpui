package scheduler

import "time"

// ProcessTracker samples process RSS and coarse CPU% over a run (Wave P0 closeout).
// Not a substitute for per-thread DevTools; good enough for example soak gates.
type ProcessTracker struct {
	StartRSSKB   int64
	EndRSSKB     int64
	PeakRSSKB    int64
	AfterCloseKB int64
	CPUPctAvg    float64
	SampleCount  int
	// ElapsedSec is wall time from Start to last Sample/Stop (for M-RSS-SLOPE).
	ElapsedSec  float64
	cpuJiffies0 float64
	wall0       time.Time
	cpuSumPct   float64
	cpuSamples  int
	started     bool
}

// Start records baseline RSS and wall clock (and CPU jiffies when available).
func (t *ProcessTracker) Start() {
	if t == nil {
		return
	}
	rss := ReadRSSKB()
	t.StartRSSKB = rss
	t.PeakRSSKB = rss
	t.EndRSSKB = rss
	t.wall0 = time.Now()
	t.ElapsedSec = 0
	if j, ok := readCPUTime(); ok {
		t.cpuJiffies0 = j
	}
	t.started = true
	t.SampleCount = 1
}

// Sample updates peak RSS, elapsed wall, and running average process CPU%.
func (t *ProcessTracker) Sample() {
	if t == nil || !t.started {
		return
	}
	rss := ReadRSSKB()
	t.EndRSSKB = rss
	if rss > t.PeakRSSKB {
		t.PeakRSSKB = rss
	}
	t.SampleCount++
	if !t.wall0.IsZero() {
		t.ElapsedSec = time.Since(t.wall0).Seconds()
	}
	if j, ok := readCPUTime(); ok && !t.wall0.IsZero() {
		elapsed := t.ElapsedSec
		if elapsed > 0.05 {
			// process CPU% over whole window since Start (not interval delta) —
			// simpler and stable for short demos.
			hz := clockTicks()
			pct := ((j - t.cpuJiffies0) / hz) / elapsed * 100
			if pct < 0 {
				pct = 0
			}
			t.CPUPctAvg = pct
			t.cpuSumPct = pct
			t.cpuSamples = 1
		}
	}
}

// Stop finalizes end RSS (call before Close of GPU if measuring run RSS).
func (t *ProcessTracker) Stop() {
	if t == nil {
		return
	}
	t.Sample()
}

// NoteAfterClose records RSS after app/window teardown (release gate).
func (t *ProcessTracker) NoteAfterClose() {
	if t == nil {
		return
	}
	t.AfterCloseKB = ReadRSSKB()
}

// RSSSlopeKBPerMin returns (End-Start)/elapsed_minutes (M-RSS-SLOPE).
// Zero when not started, no wall span, or RSS samples unavailable (both 0).
func (t *ProcessTracker) RSSSlopeKBPerMin() float64 {
	if t == nil || !t.started || t.ElapsedSec <= 1e-9 {
		return 0
	}
	if t.StartRSSKB == 0 && t.EndRSSKB == 0 {
		return 0 // stub /proc (non-Linux) — honest unavailable
	}
	minutes := t.ElapsedSec / 60.0
	if minutes <= 1e-12 {
		return 0
	}
	return float64(t.EndRSSKB-t.StartRSSKB) / minutes
}

// Apply copies tracker fields into the metrics store for JSON export,
// including automatic rss_slope_kb_per_min.
func (t *ProcessTracker) Apply(s *MetricsStore) {
	if t == nil || s == nil {
		return
	}
	s.SetProcessStats(t.StartRSSKB, t.EndRSSKB, t.PeakRSSKB, t.AfterCloseKB, t.CPUPctAvg)
	s.SetRSSSlope(t.RSSSlopeKBPerMin(), t.ElapsedSec)
}
