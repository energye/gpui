package scheduler

import (
	"encoding/json"
	"sync"
	"time"
)

// HitchThresholdMs counts a frame interval as a hitch when it exceeds this
// wall-time gap (≈2× 60 Hz vsync). Used for soak / jank observation.
const HitchThresholdMs = 33.4

// intervalRingCap is the sample window for p50/p99 frame intervals.
const intervalRingCap = 256

// FrameMetrics is the L1 observability snapshot (F14).
// JSON field names are stable for baseline tooling.
type FrameMetrics struct {
	// Counts
	FrameCount   int64 `json:"frame_count"`
	PresentCount int64 `json:"present_count"`
	MissedVSync  int64 `json:"missed_vsync"`
	HitchCount   int64 `json:"hitch_count"` // intervals > HitchThresholdMs

	// HitchRatePerMin is hitch_count / wall minutes between first and last
	// NoteFrameInterval (0 until at least one interval sample exists).
	HitchRatePerMin float64 `json:"hitch_rate_per_min,omitempty"`

	// Pipeline
	PipelineDepth int `json:"pipeline_depth"`
	PipelineMax   int `json:"pipeline_max"`

	// Timing (milliseconds)
	LastFrameIntervalMs float64 `json:"last_frame_interval_ms"`
	MaxFrameIntervalMs  float64 `json:"max_frame_interval_ms"`
	AvgFrameIntervalMs  float64 `json:"avg_frame_interval_ms"`
	P50FrameIntervalMs  float64 `json:"frame_interval_p50_ms,omitempty"`
	P95FrameIntervalMs  float64 `json:"frame_interval_p95_ms,omitempty"`
	P99FrameIntervalMs  float64 `json:"frame_interval_p99_ms,omitempty"`
	LastBuildMs         float64 `json:"frame_build_ms"`
	LastRasterMs        float64 `json:"frame_raster_ms"`

	// Layout/paint flush counters (cumulative; wired by PipelineApp)
	LayoutCount         int64 `json:"layout_count"`
	PaintCount          int64 `json:"paint_count"`
	RasterLayerCount    int64 `json:"raster_layer_count"`
	CompositeLayerCount int64 `json:"composite_layer_count"`

	// Present locality (序13 dirty-rect path; 0/empty when unavailable).
	// DamageAreaPx is physical-pixel area of the last present's FrameDamage union.
	DamageAreaPx   int64  `json:"damage_area_px,omitempty"`
	PresentMode    string `json:"present_mode,omitempty"` // full|damage_union|damage_multi|idle
	DamageAreaLast int64  `json:"damage_area_last_px,omitempty"`

	// VSyncSource is "true" | "fallback" | "" (unknown).
	VSyncSource string `json:"vsync_source,omitempty"`

	// Process resource samples (Wave P0 closeout; Linux /proc; 0 = unavailable).
	RSSStartKB      int64   `json:"rss_start_kb,omitempty"`
	RSSEndKB        int64   `json:"rss_end_kb,omitempty"`
	RSSPeakKB       int64   `json:"rss_peak_kb,omitempty"`
	RSSAfterCloseKB int64   `json:"rss_after_close_kb,omitempty"`
	CPUPctAvg       float64 `json:"cpu_pct_avg,omitempty"`

	// CPUUIPct / CPURasterPct are **path work-share proxies**, not OS-thread DevTools %.
	// They split cumulative NoteBuildMs vs NoteRasterMs wall work:
	//   share = pathSum / (buildSum+rasterSum) * 100  (sums to ~100 when both sides fed)
	// When cpu_pct_avg > 0, values are further scaled: processCPU * share
	// so UI+Raster ≈ process CPU and still diverge when work is one-sided.
	// Zero/omitted when no build/raster notes yet (honest unavailable).
	CPUUIPct     float64 `json:"cpu_ui_pct,omitempty"`
	CPURasterPct float64 `json:"cpu_raster_pct,omitempty"`
}

// MetricsStore is a concurrency-safe metrics accumulator.
type MetricsStore struct {
	mu          sync.Mutex
	m           FrameMetrics
	firstAt     time.Time // wall clock of first NoteFrameInterval
	lastAt      time.Time
	intervalSum float64
	intervalN   int64
	ring        [intervalRingCap]float64
	ringN       int
	ringI       int

	// Cumulative UI-build and Raster-present wall times (ms) for path CPU split.
	buildSumMs  float64
	rasterSumMs float64
}

// Snapshot returns a copy of current metrics (includes p50/p95/p99, hitch rate, path CPU).
func (s *MetricsStore) Snapshot() FrameMetrics {
	if s == nil {
		return FrameMetrics{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.m
	out.P50FrameIntervalMs, out.P95FrameIntervalMs, out.P99FrameIntervalMs = s.percentilesLocked()
	out.HitchRatePerMin = s.hitchRatePerMinLocked()
	out.CPUUIPct, out.CPURasterPct = s.pathCPULocked()
	return out
}

// JSON returns metrics as JSON bytes.
func (s *MetricsStore) JSON() ([]byte, error) {
	m := s.Snapshot()
	return json.Marshal(m)
}

// NoteFrameInterval records wall time since previous frame note.
// Tracks max/avg interval, hitch count, and a ring for p50/p95/p99.
func (s *MetricsStore) NoteFrameInterval(now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.firstAt.IsZero() {
		s.firstAt = now
	}
	if !s.lastAt.IsZero() {
		ms := now.Sub(s.lastAt).Seconds() * 1000
		s.m.LastFrameIntervalMs = ms
		if ms > s.m.MaxFrameIntervalMs {
			s.m.MaxFrameIntervalMs = ms
		}
		s.intervalSum += ms
		s.intervalN++
		s.m.AvgFrameIntervalMs = s.intervalSum / float64(s.intervalN)
		if ms > HitchThresholdMs {
			s.m.HitchCount++
		}
		s.ring[s.ringI%intervalRingCap] = ms
		s.ringI++
		if s.ringN < intervalRingCap {
			s.ringN++
		}
	}
	s.lastAt = now
	s.m.FrameCount++
}

// NotePresent increments present counter.
func (s *MetricsStore) NotePresent() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PresentCount++
	s.mu.Unlock()
}

// NotePresentOutcome records present mode + damage area for the last frame (M-DAMAGE-AREA).
// mode should be render.PresentMode.String(); areaPx is physical dirty union area.
func (s *MetricsStore) NotePresentOutcome(mode string, areaPx int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PresentCount++
	s.m.PresentMode = mode
	s.m.DamageAreaPx = areaPx
	s.m.DamageAreaLast = areaPx
	s.mu.Unlock()
}

// NoteMissedVSync increments missed vsync counter.
func (s *MetricsStore) NoteMissedVSync() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.MissedVSync++
	s.mu.Unlock()
}

// SetPipeline records current and max pipeline depth.
func (s *MetricsStore) SetPipeline(depth, max int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PipelineDepth = depth
	s.m.PipelineMax = max
	s.mu.Unlock()
}

// NoteBuildMs records UI frame build duration and accumulates path work for cpu_ui_pct.
func (s *MetricsStore) NoteBuildMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastBuildMs = ms
	if ms > 0 {
		s.buildSumMs += ms
	}
	s.mu.Unlock()
}

// NoteRasterMs records raster duration and accumulates path work for cpu_raster_pct.
func (s *MetricsStore) NoteRasterMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastRasterMs = ms
	if ms > 0 {
		s.rasterSumMs += ms
	}
	s.mu.Unlock()
}

// pathCPULocked returns UI/Raster CPU proxies from cumulative build/raster ms.
// Caller holds s.mu.
func (s *MetricsStore) pathCPULocked() (ui, raster float64) {
	total := s.buildSumMs + s.rasterSumMs
	if total <= 1e-12 {
		return 0, 0
	}
	uiShare := s.buildSumMs / total
	raShare := s.rasterSumMs / total
	if s.m.CPUPctAvg > 0 {
		// Attribute process CPU across paths by work share (sums ≈ process %).
		return s.m.CPUPctAvg * uiShare, s.m.CPUPctAvg * raShare
	}
	// No process sample: report pure work-share percentages (sum ≈ 100).
	return uiShare * 100, raShare * 100
}

// BuildSumMs returns cumulative UI build wall ms (tests / diagnostics).
func (s *MetricsStore) BuildSumMs() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buildSumMs
}

// RasterSumMs returns cumulative raster wall ms (tests / diagnostics).
func (s *MetricsStore) RasterSumMs() float64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rasterSumMs
}

// SetRasterLayerCount records dirty-layer re-raster count for this frame (F02).
func (s *MetricsStore) SetRasterLayerCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.RasterLayerCount = n
	s.mu.Unlock()
}

// SetLayoutCount records layout flush count snapshot.
func (s *MetricsStore) SetLayoutCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LayoutCount = n
	s.mu.Unlock()
}

// SetPaintCount records paint flush count snapshot.
func (s *MetricsStore) SetPaintCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PaintCount = n
	s.mu.Unlock()
}

// SetVSyncSource records whether pacing uses true vsync or software fallback.
func (s *MetricsStore) SetVSyncSource(src string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.VSyncSource = src
	s.mu.Unlock()
}

// SetProcessStats records RSS (KiB) and average process CPU% for JSON baselines.
func (s *MetricsStore) SetProcessStats(startKB, endKB, peakKB, afterCloseKB int64, cpuPctAvg float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.RSSStartKB = startKB
	s.m.RSSEndKB = endKB
	s.m.RSSPeakKB = peakKB
	s.m.RSSAfterCloseKB = afterCloseKB
	s.m.CPUPctAvg = cpuPctAvg
	s.mu.Unlock()
}

// percentilesLocked returns p50, p95, and p99 of the interval ring. Caller holds s.mu.
func (s *MetricsStore) percentilesLocked() (p50, p95, p99 float64) {
	n := s.ringN
	if n == 0 {
		return 0, 0, 0
	}
	tmp := make([]float64, n)
	// ring is filled [0,n) until full, then ringI wraps; always copy last n samples.
	if n < intervalRingCap {
		copy(tmp, s.ring[:n])
	} else {
		// oldest is at ringI % cap when full
		start := s.ringI % intervalRingCap
		for i := 0; i < n; i++ {
			tmp[i] = s.ring[(start+i)%intervalRingCap]
		}
	}
	// insertion sort (n≤256)
	for i := 1; i < n; i++ {
		v := tmp[i]
		j := i
		for j > 0 && tmp[j-1] > v {
			tmp[j] = tmp[j-1]
			j--
		}
		tmp[j] = v
	}
	p50 = tmp[(n-1)*50/100]
	p95 = tmp[(n-1)*95/100]
	p99 = tmp[(n-1)*99/100]
	return p50, p95, p99
}

// hitchRatePerMinLocked is hitch_count / elapsed minutes (first→last NoteFrameInterval).
// Zero when wall span is missing or negligible. Caller holds s.mu.
func (s *MetricsStore) hitchRatePerMinLocked() float64 {
	if s.m.HitchCount <= 0 || s.firstAt.IsZero() || s.lastAt.IsZero() {
		return 0
	}
	elapsed := s.lastAt.Sub(s.firstAt).Minutes()
	if elapsed <= 1e-9 {
		// Sub-millisecond span: treat as "not enough wall time" rather than inf.
		return 0
	}
	return float64(s.m.HitchCount) / elapsed
}
