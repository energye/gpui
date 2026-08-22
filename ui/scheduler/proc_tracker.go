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

	// Decimated RSS series (≥rssSampleGapSec apart) for the least-squares
	// M-RSS-SLOPE fit. Bounded: a 300s soak holds ~600 points.
	rssT       []float64
	rssV       []int64
	lastStored float64
}

// rssSampleGapSec paces series storage (Sample may be called every frame).
const rssSampleGapSec = 0.5

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
	t.rssT = t.rssT[:0]
	t.rssV = t.rssV[:0]
	t.lastStored = 0
	t.rssT = append(t.rssT, 0)
	t.rssV = append(t.rssV, rss)
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
		// Store decimated series points for the slope fit.
		if t.ElapsedSec-t.lastStored >= rssSampleGapSec {
			t.lastStored = t.ElapsedSec
			t.rssT = append(t.rssT, t.ElapsedSec)
			t.rssV = append(t.rssV, rss)
		}
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

// RSSSlopeKBPerMin returns the least-squares RSS trend over the **steady-state**
// segment of the decimated sample series (samples after 25% of elapsed wall),
// in KB/min (M-RSS-SLOPE). The E-family question is "does it leak": heap and
// GPU-driver equilibrium is established during warmup, so that quarter is
// discarded; a plateau folds to ~0 while a sustained leak — including one that
// starts mid-run — keeps its true slope. The previous two-point
// (End-Start)/elapsed estimator conflated the one-time startup ramp with
// ongoing growth. Falls back to the two-point estimate when fewer than 2
// steady-state points exist. Zero when not started, no wall span, or stub
// /proc (non-Linux) — honest unavailable.
func (t *ProcessTracker) RSSSlopeKBPerMin() float64 {
	if t == nil || !t.started || t.ElapsedSec <= 1e-9 {
		return 0
	}
	if t.StartRSSKB == 0 && t.EndRSSKB == 0 {
		return 0 // stub /proc (non-Linux) — honest unavailable
	}
	cut := t.ElapsedSec / 4
	ts := make([]float64, 0, len(t.rssT))
	vs := make([]int64, 0, len(t.rssV))
	for i := range t.rssV {
		if t.rssT[i] >= cut {
			ts = append(ts, t.rssT[i])
			vs = append(vs, t.rssV[i])
		}
	}
	if len(vs) < 2 {
		minutes := t.ElapsedSec / 60.0
		if minutes <= 1e-12 {
			return 0
		}
		return float64(t.EndRSSKB-t.StartRSSKB) / minutes
	}
	return rssSlopeLSQ(ts, vs)
}

// rssSlopeLSQ is the least-squares slope of the (t, v) series in KB/min.
func rssSlopeLSQ(ts []float64, vs []int64) float64 {
	n := len(vs)
	if n < 2 {
		return 0
	}
	var mt, mv float64
	for i := range vs {
		mt += ts[i]
		mv += float64(vs[i])
	}
	mt /= float64(n)
	mv /= float64(n)
	var num, den float64
	for i := range vs {
		dt := ts[i] - mt
		num += dt * (float64(vs[i]) - mv)
		den += dt * dt
	}
	if den <= 1e-9 {
		return 0
	}
	return num / den * 60.0 // KB per second → KB per minute
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
