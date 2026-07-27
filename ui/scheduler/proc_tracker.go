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
	cpuJiffies0  float64
	wall0        time.Time
	cpuSumPct    float64
	cpuSamples   int
	started      bool
}

// Start records baseline RSS and CPU jiffies.
func (t *ProcessTracker) Start() {
	if t == nil {
		return
	}
	rss := ReadRSSKB()
	t.StartRSSKB = rss
	t.PeakRSSKB = rss
	t.EndRSSKB = rss
	if j, ok := readCPUTime(); ok {
		t.cpuJiffies0 = j
		t.wall0 = time.Now()
	}
	t.started = true
	t.SampleCount = 1
}

// Sample updates peak RSS and running average process CPU%.
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
	if j, ok := readCPUTime(); ok && !t.wall0.IsZero() {
		elapsed := time.Since(t.wall0).Seconds()
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

// Apply copies tracker fields into the metrics store for JSON export.
func (t *ProcessTracker) Apply(s *MetricsStore) {
	if t == nil || s == nil {
		return
	}
	s.SetProcessStats(t.StartRSSKB, t.EndRSSKB, t.PeakRSSKB, t.AfterCloseKB, t.CPUPctAvg)
}
