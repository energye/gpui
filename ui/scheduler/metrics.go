//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package scheduler

import (
	"encoding/json"
	"sort"
	"sync"
	"time"
)

// HitchThresholdMs counts a frame interval as a hitch when it exceeds this
// wall-time gap (≈2× 60 Hz vsync). Used for soak / jank observation.
const HitchThresholdMs = 33.4

// intervalRingCap is the sample window for p50/p99 frame intervals.
const intervalRingCap = 256

// percentilePool reuses the 256-sample sort buffer for Snapshot percentiles.
// Snapshot runs per tick (60Hz+) in animation windows; allocating a fresh
// slice each time costs ~250KB/s of pure GC churn for identical results.
var percentilePool = sync.Pool{New: func() any { return make([]float64, intervalRingCap) }}

// Present-policy names for Metrics JSON.
const (
	PresentPolicyFullPaint = "full_paint"
	PresentPolicyRetained  = "retained"
	PresentPolicyHybrid    = "hybrid"
)

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
	AvgFrameIntervalMs  float64 `json:"interval_avg_ms"`
	P50FrameIntervalMs  float64 `json:"interval_p50_ms,omitempty"`
	P95FrameIntervalMs  float64 `json:"interval_p95_ms,omitempty"`
	P99FrameIntervalMs  float64 `json:"interval_p99_ms,omitempty"`
	// FPSWall is present_count / wall seconds between the first and last
	// interval note (A-family contract, same definition as wrgate's
	// fps_wall: includes open/close overhead — often low). 0/omitted until
	// both a present and an interval span exist.
	FPSWall float64 `json:"fps_wall,omitempty"`
	// TargetHz is the display refresh rate the pacer was seeded with
	// (SeedDisplayRefreshHz); 0/omitted when the host never reported one.
	TargetHz     float64 `json:"target_hz,omitempty"`
	LastBuildMs  float64 `json:"frame_build_ms"`
	LastRasterMs float64 `json:"frame_raster_ms"`
	// F期干活计时独立：Flush(干活)与present(等显示器)分开记。
	// LastFlushMs = GPU干活耗时(重录+画画)；LastPresentWaitMs = 等显示器耗时。
	// 6ms口径看 LastFlushMs，不被 Fifo 等待带偏。
	LastFlushMs       float64 `json:"frame_flush_ms,omitempty"`
	LastPresentWaitMs float64 `json:"frame_present_wait_ms,omitempty"`
	// LastAcquireWaitMs = BeginFrame 等空闲缓冲(Fifo 背压)。F期三段尺子
	// 之三：干活(Flush) / 等缓冲(Acquire) / 等显示(PresentWait)。
	LastAcquireWaitMs float64 `json:"frame_acquire_wait_ms,omitempty"`

	// Frame-stage distributions (ms) over the recent-frame ring (8192):
	// build/raster are UI-side wall work, flush = GPU work, present_wait =
	// Fifo wait, acquire_wait = buffer backpressure. Percentiles are written
	// at Snapshot time (pure observation — no frame-path behaviour change;
	// hitch 专项慢帧归属用：慢帧落在哪一段就是铁证).
	BuildMsP50Ms         float64 `json:"frame_build_p50_ms,omitempty"`
	BuildMsP95Ms         float64 `json:"frame_build_p95_ms,omitempty"`
	BuildMsMaxMs         float64 `json:"frame_build_max_ms,omitempty"`
	BuildMsSamples       int64   `json:"frame_build_samples,omitempty"`
	RasterMsP50Ms        float64 `json:"frame_raster_p50_ms,omitempty"`
	RasterMsP95Ms        float64 `json:"frame_raster_p95_ms,omitempty"`
	RasterMsMaxMs        float64 `json:"frame_raster_max_ms,omitempty"`
	RasterMsSamples      int64   `json:"frame_raster_samples,omitempty"`
	FlushMsP50Ms         float64 `json:"frame_flush_p50_ms,omitempty"`
	FlushMsP95Ms         float64 `json:"frame_flush_p95_ms,omitempty"`
	FlushMsMaxMs         float64 `json:"frame_flush_max_ms,omitempty"`
	FlushMsSamples       int64   `json:"frame_flush_samples,omitempty"`
	PresentWaitMsP50Ms   float64 `json:"frame_present_wait_p50_ms,omitempty"`
	PresentWaitMsP95Ms   float64 `json:"frame_present_wait_p95_ms,omitempty"`
	PresentWaitMsMaxMs   float64 `json:"frame_present_wait_max_ms,omitempty"`
	PresentWaitMsSamples int64   `json:"frame_present_wait_samples,omitempty"`
	AcquireWaitMsP50Ms   float64 `json:"frame_acquire_wait_p50_ms,omitempty"`
	AcquireWaitMsP95Ms   float64 `json:"frame_acquire_wait_p95_ms,omitempty"`
	AcquireWaitMsMaxMs   float64 `json:"frame_acquire_wait_max_ms,omitempty"`
	AcquireWaitMsSamples int64   `json:"frame_acquire_wait_samples,omitempty"`

	// Layout/paint flush counters (cumulative; wired by PipelineApp)
	LayoutCount         int64 `json:"layout_count"`
	PaintCount          int64 `json:"paint_count"`
	PaintVisits         int64 `json:"paint_visits,omitempty"` // last frame node visits
	RasterLayerCount    int64 `json:"raster_layer_count"`
	CompositeLayerCount int64 `json:"composite_layer_count"`
	FilterLayerCount    int64 `json:"filter_layer_count"` // color/image filter layers applied last composite

	// Present locality (序13 dirty-rect path; 0/empty when unavailable).
	// DamageAreaPx is physical-pixel area of the last present's FrameDamage union.
	DamageAreaPx int64  `json:"damage_area_px,omitempty"`
	PresentMode  string `json:"present_mode,omitempty"` // full|damage_union|damage_multi (idle inputs preserve last real)
	// PresentModeAgeSec is seconds since the last REAL (non-idle) present
	// set the mode (R3-6: idle frames preserve the value, so readers need
	// its age to tell fresh steady-state from hours-stale residue).
	PresentModeAgeSec float64 `json:"present_mode_age_s,omitempty"`
	// PresentModeAtNs is the internal timestamp behind PresentModeAgeSec
	// (UnixNano, never marshalled).
	PresentModeAtNs int64 `json:"-"`
	DamageAreaLast  int64 `json:"damage_area_last_px,omitempty"`

	// DirtyLayerIDs is the last frame's dirty layer/boundary id list (R4b; the
	// C-family dirty_layer_ids). Two distant hot spots → two ids. Empty/omitted
	// when the window path does not collect them. DamageMultiFrames is the
	// cumulative count of damage_multi presents (multi-rect independent scissors).
	DirtyLayerIDs     []uint64 `json:"dirty_layer_ids,omitempty"`
	DamageMultiFrames int64    `json:"damage_multi_frames,omitempty"`

	// PresentPolicy is the window paint/present strategy name.
	// Values: PresentPolicyFullPaint | PresentPolicyRetained | PresentPolicyHybrid.
	PresentPolicy string `json:"present_policy,omitempty"`

	// VSyncSource is "true" | "fallback" | "" (unknown).
	VSyncSource string `json:"vsync_source,omitempty"`

	// Process resource samples (Wave P0 closeout; Linux /proc; 0 = unavailable).
	RSSStartKB      int64 `json:"rss_start_kb,omitempty"`
	RSSEndKB        int64 `json:"rss_end_kb,omitempty"`
	RSSPeakKB       int64 `json:"rss_peak_kb,omitempty"`
	RSSAfterCloseKB int64 `json:"rss_after_close_kb,omitempty"`
	// RSSSlopeKBPerMin is the raw least-squares RSS trend over samples after
	// 25% of elapsed (M-RSS-SLOPE). Positive = growth. On a short run the
	// one-time startup ramp is included, so this is NOT a leak verdict; use
	// RSSSlopeSteadyKBPerMin for that. Zero when wall span is
	// missing/negligible or RSS samples unavailable.
	RSSSlopeKBPerMin float64 `json:"rss_slope_kb_per_min,omitempty"`
	// RSSSlopeSteadyKBPerMin is the least-squares trend after
	// max(25% elapsed, 15s) warmup. Only meaningful when RSSSlopeValid;
	// valid requires >=15s of post-warmup samples (a >=30s run).
	RSSSlopeSteadyKBPerMin float64 `json:"rss_slope_steady_kb_per_min,omitempty"`
	// RSSSlopeValid reports whether the steady slope has enough post-warmup
	// span to be a leak verdict. false = cannot conclude (run too short or
	// RSS unavailable), not "no leak".
	RSSSlopeValid bool `json:"rss_slope_valid"`
	// RSSWarmupSec is the warmup discarded by the steady fit; reported even
	// when invalid so short runs explain why.
	RSSWarmupSec float64 `json:"rss_warmup_sec,omitempty"`
	// RSSElapsedSec is wall seconds used for slope (honest denominator).
	RSSElapsedSec float64 `json:"rss_elapsed_sec,omitempty"`
	CPUPctAvg     float64 `json:"cpu_pct_avg,omitempty"`

	// CPUUIPct / CPURasterPct are **path work-share proxies**, not OS-thread DevTools %.
	// They split cumulative NoteBuildMs vs NoteRasterMs wall work:
	//   share = pathSum / (buildSum+rasterSum) * 100 (sums to ~100 when both sides fed)
	// When cpu_pct_avg > 0, values are further scaled: processCPU * share
	// so UI+Raster ≈ process CPU and still diverge when work is one-sided.
	// Zero/omitted when no build/raster notes yet (honest unavailable).
	CPUUIPct     float64 `json:"cpu_ui_pct,omitempty"`
	CPURasterPct float64 `json:"cpu_raster_pct,omitempty"`

	// GPU path routing (from render.Context.RenderPathStats; M-GPU-* UI JSON).
	// GPUOps / CPUFallbackOps are cumulative on the present Context for the run
	// (not reset each frame). FrameFlushes is last-frame (BeginFrame clears it).
	GPUOps                int64  `json:"gpu_ops,omitempty"`
	CPUFallbackOps        int64  `json:"cpu_fallback_ops,omitempty"`
	FrameFlushes          int64  `json:"frame_flushes,omitempty"`
	LastCPUFallbackReason string `json:"last_cpu_fallback,omitempty"`
	// StagedWriteOverlaps is the backend upload guard from
	// render.Context.RenderPathStats (cumulative per process): uploads a drain
	// collapsed onto bytes an earlier staged write of the same window already
	// covered. A healthy run keeps it 0; non-zero = an upload window reused a
	// buffer region and earlier command buffers read later bytes.
	StagedWriteOverlaps int64 `json:"staged_write_overlaps,omitempty"`

	// GPUBackend is the actual adapter category behind the window
	// (discrete/integrated/software/gl), reported once at open from
	// PresentTarget.GPUBackend (multiwindow 1.3 downgrade chain).
	GPUBackend string `json:"gpu_backend,omitempty"`
	// GPUFallbacks counts open-time downgrade levels retried before the
	// window opened (0 = first level succeeded). Always emitted (zero is
	// the honest common case, not "unavailable").
	GPUFallbacks int `json:"gpu_fallbacks"`

	// Boundary cache counters.
	BoundaryRerecord int64 `json:"boundary_rerecord,omitempty"`
	BoundarySkip     int64 `json:"boundary_skip,omitempty"`
	// BoundaryCount is last reported tree boundary count (R3b; optional).
	BoundaryCount int64 `json:"boundary_count,omitempty"`
	// BoundaryMaxDepth is max nesting depth of repaint boundaries (R3b).
	BoundaryMaxDepth int64 `json:"boundary_max_depth,omitempty"`
	// Shell partitioning of the boundary caches (R21 shell/content layering;
	// cumulative across NoteShellBoundaryFrame calls). A scrolling body must
	// keep ShellRerecord == 0 while the shell Picture cache Replays.
	ShellRerecord int64 `json:"shell_rerecord,omitempty"`
	ShellSkip     int64 `json:"shell_skip,omitempty"`

	// OSPresentPartial / OSPresentFull are the cumulative OS-present counters
	// sampled from the swapchain (hal.Swapchain.PresentStats) after each real
	// present: partial = OS damage/present protocols used (EGL
	// SwapBuffersWithDamage on the Go GL backend), full = whole-surface swaps.
	// Backends without an OS damage-present protocol report zero partials
	// honestly (wgpu-native deliberately ignores damage; Windows WGL has none)
	// — dirty region ④ narrowing is measured against os_present_partial.
	OSPresentPartial uint64 `json:"os_present_partial,omitempty"`
	OSPresentFull    uint64 `json:"os_present_full,omitempty"`
	// TextureRecordBytes is the cumulative GPU texture bytes written by
	// boundary-layer re-records (record full-surface + recordLocal bounds,
	// w*h*4 each) — the honest proxy for retained-path texture upload cost;
	// dirty region ② upload narrowing is measured against this.
	TextureRecordBytes int64 `json:"texture_record_bytes,omitempty"`

	SaveLayerAllow  int64 `json:"savelayer_allow,omitempty"`
	SaveLayerReject int64 `json:"savelayer_reject,omitempty"`

	// PictureOpCount is last-frame display-list op count.
	PictureOpCount int64 `json:"picture_op_count,omitempty"`

	// Cache budget observability: CacheEntries is the current
	// combined entry count of the two layer caches (boundary Picture cache +
	// retained-path texture LRU); CacheEvictions is the cumulative count of
	// entries dropped by the explicit budgets / capacity LRU / generational
	// sweep. Sampled per frame by PipelineApp.
	CacheEntries   int64 `json:"cache_entries,omitempty"`
	CacheEvictions int64 `json:"cache_evictions,omitempty"`
	// BudgetRefusals is the cumulative count of EnsureCapacity growth
	// refusals under multi-window budget pressure.
	BudgetRefusals int64 `json:"budget_refusals,omitempty"`

	// MeasureCacheHit is cumulative text-measure cache hits since last reset
	// . MeasureCacheMiss is the miss counterpart for hits≥miss proof.
	MeasureCacheHit  int64 `json:"measure_cache_hit"`
	MeasureCacheMiss int64 `json:"measure_cache_miss"`

	// Virtual-list virtualization.
	// BindCount/ItemCount describe the most recently rebound VirtualList: the
	// R7 gate is bind_count ≪ item_count (only viewport cells mounted).
	// Omitted while no VirtualList has bound (windows without lists).
	BindCount int64 `json:"bind_count,omitempty"`
	ItemCount int64 `json:"item_count,omitempty"`
	// ScrollRerecord is the cumulative count of cells freshly mounted because
	// they scrolled into the viewport (R7b/C3 scroll reuse); kept cells replay
	// their cached Picture and are not counted.
	ScrollRerecord int64 `json:"scroll_rerecord,omitempty"`

	// H-family first-present observation.
	// Warmup is true when the Open-time warm-up full paint actually ran before
	// the first loop present. TimeToFirstPresentMs is wall ms from Open to the
	// first present completing. FirstPresentPaintCount is the pipe paint count
	// at that moment (>0 proves the first frame has content, not a black/empty
	// boot). All three are set once by SetFirstPresent; zero/empty when never.
	Warmup                 bool    `json:"warmup,omitempty"`
	TimeToFirstPresentMs   float64 `json:"time_to_first_present_ms,omitempty"`
	FirstPresentPaintCount int64   `json:"first_present_paint_count,omitempty"`
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
	targetHz    float64 // display refresh seeded by SeedDisplayRefreshHz

	// Hitch pause (C4/P2.6): verification readbacks (snapshot drain) stall the
	// raster frame past HitchThresholdMs, which is verification activity, not
	// animation jank. depth>0 excludes stalls from HitchCount; after the
	// depth returns to 0, skipNextHitch excludes the one interval that
	// straddles the paused activity (the drain site samples the frame
	// interval right after resuming).
	hitchPauseDepth int
	skipNextHitch   bool

	// Cumulative UI-build and Raster-present wall times (ms) for path CPU split.
	buildSumMs  float64
	rasterSumMs float64

	// Frame-stage duration rings (ms) — see FrameMetrics distribution fields.
	buildDist, rasterDist, flushDist, presentDist, acquireDist stageDist
}

// stageDist collects per-frame stage durations (ms) into a fixed ring so the
// end-of-run Snapshot can report p50/p95/max without adding cost to the frame
// path (a ring write per note; sort deferred to Snapshot). Pure observation.
type stageDist struct {
	ring [8192]float64
	seen int64
	next int
}

func (d *stageDist) add(ms float64) {
	if ms < 0 {
		ms = 0
	}
	d.ring[d.next] = ms
	d.next = (d.next + 1) % len(d.ring)
	if d.seen < int64(len(d.ring)) {
		d.seen++
	}
}

// fill writes p50/p95/max (ms) over collected samples into the stats fields.
func (d *stageDist) fill(samples *int64, p50, p95, max *float64) {
	*samples = d.seen
	if d.seen == 0 {
		return
	}
	c := make([]float64, d.seen)
	copy(c, d.ring[:d.seen])
	sort.Slice(c, func(i, j int) bool { return c[i] < c[j] })
	*p50 = c[len(c)/2]
	*p95 = c[min(int(float64(len(c))*0.95), len(c)-1)]
	*max = c[len(c)-1]
}

// StageDistributionsLocked fills the per-stage distribution fields of out.
// Caller must hold s.mu.
func (s *MetricsStore) stageDistributionsLocked(out *FrameMetrics) {
	s.buildDist.fill(&out.BuildMsSamples, &out.BuildMsP50Ms, &out.BuildMsP95Ms, &out.BuildMsMaxMs)
	s.rasterDist.fill(&out.RasterMsSamples, &out.RasterMsP50Ms, &out.RasterMsP95Ms, &out.RasterMsMaxMs)
	s.flushDist.fill(&out.FlushMsSamples, &out.FlushMsP50Ms, &out.FlushMsP95Ms, &out.FlushMsMaxMs)
	s.presentDist.fill(&out.PresentWaitMsSamples, &out.PresentWaitMsP50Ms, &out.PresentWaitMsP95Ms, &out.PresentWaitMsMaxMs)
	s.acquireDist.fill(&out.AcquireWaitMsSamples, &out.AcquireWaitMsP50Ms, &out.AcquireWaitMsP95Ms, &out.AcquireWaitMsMaxMs)
}

// PauseHitchCount suspends jank counting for a scoped verification activity
// (snapshot readback drain). Nesting is allowed; every pause must pair with a
// ResumeHitchCount. Interval percentiles and max stay recorded — only
// HitchCount is scoped.
func (s *MetricsStore) PauseHitchCount() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.hitchPauseDepth++
	s.mu.Unlock()
}

// ResumeHitchCount ends one PauseHitchCount scope. When the outermost scope
// ends, the first subsequent NoteFrameInterval is treated as straddling the
// paused activity and skips hitch counting exactly once (a fast readback has
// nothing to skip; a slow one lands in that interval).
func (s *MetricsStore) ResumeHitchCount() {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.hitchPauseDepth > 0 {
		s.hitchPauseDepth--
		if s.hitchPauseDepth == 0 {
			s.skipNextHitch = true
		}
	}
	s.mu.Unlock()
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
	out.TargetHz = s.targetHz
	s.stageDistributionsLocked(&out)
	if s.m.PresentCount > 0 && !s.firstAt.IsZero() && s.lastAt.After(s.firstAt) {
		out.FPSWall = float64(s.m.PresentCount) / s.lastAt.Sub(s.firstAt).Seconds()
	}
	if out.PresentModeAtNs > 0 {
		out.PresentModeAgeSec = time.Since(time.Unix(0, out.PresentModeAtNs)).Seconds()
	}
	return out
}

// setTargetHz records the display refresh rate for the metrics JSON
// (target_hz A-family field). Same package as FrameScheduler, which forwards
// from SeedDisplayRefreshHz — no separate public knob.
func (s *MetricsStore) setTargetHz(hz float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.targetHz = hz
	s.mu.Unlock()
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
		// C4/P2.6: verification stalls (snapshot drain pause) are not animation
		// jank; the first interval after the outermost resume straddles the
		// paused activity (drain and the present-completion sample are
		// adjacent in one raster job) and skips exactly once.
		if ms > HitchThresholdMs && s.hitchPauseDepth == 0 && !s.skipNextHitch {
			s.m.HitchCount++
		}
		if s.hitchPauseDepth == 0 {
			s.skipNextHitch = false
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
// Does NOT bump PresentCount: the raster loop already counted this frame via
// NotePresent, so counting here too double-books every frame (spin_min:
// PresentCount read 2x FrameCount). Idle presents carry no drawing: they
// still count as presents but preserve the last real mode + area so
// JSON/gates observe the true steady-state (damage_union/damage_multi)
// instead of event-noise zeros.
func (s *MetricsStore) NotePresentOutcome(mode string, areaPx int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	// Idle presents carry no drawing and are not the retained steady-state
	// mode (e.g. an X11 Expose-triggered frame with nothing dirty). Keep the
	// last real present mode + area so JSON/gates observe the true retained
	// incremental outcome (damage_union/damage_multi) instead of event-noise.
	if mode != "idle" {
		s.m.PresentMode = mode
		s.m.DamageAreaPx = areaPx
		s.m.DamageAreaLast = areaPx
		s.m.PresentModeAtNs = time.Now().UnixNano()
	}
	s.mu.Unlock()
}

// NoteDamageMultiFrame increments the cumulative damage_multi present count (R4b).
func (s *MetricsStore) NoteDamageMultiFrame() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.DamageMultiFrames++
	s.mu.Unlock()
}

// SetDirtyLayerIDs records the last frame's dirty layer/boundary id list (R4b).
func (s *MetricsStore) SetDirtyLayerIDs(ids []uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if ids == nil {
		s.m.DirtyLayerIDs = nil
	} else {
		s.m.DirtyLayerIDs = append([]uint64(nil), ids...)
	}
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
	s.buildDist.add(ms)
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
	s.rasterDist.add(ms)
	s.mu.Unlock()
}

// NoteFlushMs records GPU work duration (Flush, no present wait).
// 6ms口径看这个数。
func (s *MetricsStore) NoteFlushMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastFlushMs = ms
	s.flushDist.add(ms)
	s.mu.Unlock()
}

// NotePresentWaitMs records present-wait duration (Fifo wait for display).
// 等的耗时记这里，不掺进干活。
func (s *MetricsStore) NotePresentWaitMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastPresentWaitMs = ms
	s.presentDist.add(ms)
	s.mu.Unlock()
}

// NoteAcquireWaitMs records BeginFrame acquire-wait duration.
// 等空闲缓冲的耗时。
func (s *MetricsStore) NoteAcquireWaitMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastAcquireWaitMs = ms
	s.acquireDist.add(ms)
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

// SetFilterLayerCount records filter layers applied in the last composite.
func (s *MetricsStore) SetFilterLayerCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.FilterLayerCount = n
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

// SetPaintVisits records the last frame's node paint visits.
func (s *MetricsStore) SetPaintVisits(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PaintVisits = n
	s.mu.Unlock()
}

// SetVirtualBind samples the most recently rebound VirtualList window:
// bind = currently mounted cells, itemCount = logical row count.
func (s *MetricsStore) SetVirtualBind(bind, itemCount int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.BindCount = bind
	s.m.ItemCount = itemCount
	s.mu.Unlock()
}

// SetScrollRerecord records the cumulative scroll fresh-mount total (R7b/C3).
func (s *MetricsStore) SetScrollRerecord(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.ScrollRerecord = n
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

// SetPresentPolicy records the window present/paint strategy (present_policy JSON).
// Empty policy is ignored; use PresentPolicyFullPaint / Retained / Hybrid.
func (s *MetricsStore) SetPresentPolicy(policy string) {
	if s == nil || policy == "" {
		return
	}
	s.mu.Lock()
	s.m.PresentPolicy = policy
	s.mu.Unlock()
}

// PresentPolicy returns the current present_policy string (may be empty if unset).
func (s *MetricsStore) PresentPolicy() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m.PresentPolicy
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

// SetRSSSlope records M-RSS-SLOPE (KB/min) and the wall seconds used as denominator.
// slope may be negative (RSS dropped). elapsedSec ≤0 clears slope to 0.
func (s *MetricsStore) SetRSSSlope(slopeKBPerMin, elapsedSec float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if elapsedSec <= 1e-9 {
		s.m.RSSSlopeKBPerMin = 0
		s.m.RSSElapsedSec = 0
	} else {
		s.m.RSSSlopeKBPerMin = slopeKBPerMin
		s.m.RSSElapsedSec = elapsedSec
	}
	s.mu.Unlock()
}

// SetRSSSlopeSteady records M-RSS-SLOPE-STEADY (KB/min), the warmup actually
// discarded, and whether the result is valid as a leak verdict. When valid is
// false the steady slope is cleared but the warmup is kept so short-run JSON
// explains why (e.g. rss_warmup_sec=15, rss_slope_valid=false on a 5s window).
func (s *MetricsStore) SetRSSSlopeSteady(slopeKBPerMin, warmupSec float64, valid bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.RSSSlopeValid = valid
	s.m.RSSWarmupSec = warmupSec
	if valid {
		s.m.RSSSlopeSteadyKBPerMin = slopeKBPerMin
	} else {
		s.m.RSSSlopeSteadyKBPerMin = 0
	}
	s.mu.Unlock()
}

// NoteGPUPathStats records render path routing counters into UI JSON (M-GPU-SUBMIT /
// M-GPU-FALLBACK / M-GPU-FALLBACK-N). Values are typically cumulative Context stats
// snapshotted after Present (gpu_ops, cpu_fallback_ops) plus last-frame flushes.
func (s *MetricsStore) NoteGPUPathStats(gpuOps, cpuFallbackOps, frameFlushes int, lastFallbackReason string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.GPUOps = int64(gpuOps)
	s.m.CPUFallbackOps = int64(cpuFallbackOps)
	s.m.FrameFlushes = int64(frameFlushes)
	s.m.LastCPUFallbackReason = lastFallbackReason
	s.mu.Unlock()
}

// NoteStagedWriteOverlaps records the backend upload guard (cumulative).
// Called with the same RenderPathStats snapshot as NoteGPUPathStats.
func (s *MetricsStore) NoteStagedWriteOverlaps(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.StagedWriteOverlaps = n
	s.mu.Unlock()
}

// NoteGPUBackend records the actual adapter category + open-time downgrade
// count once per window (multiwindow 1.3). Called by embedder Open after the
// present target opens.
func (s *MetricsStore) NoteGPUBackend(backend string, fallbacks int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.GPUBackend = backend
	s.m.GPUFallbacks = fallbacks
	s.mu.Unlock()
}

// NoteBoundaryFrame accumulates per-frame RepaintBoundary cache stats.
// rerecord/skip are typically that frame's FrameRerecord/FrameSkip from BoundaryCache.
func (s *MetricsStore) NoteBoundaryFrame(rerecord, skip int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.BoundaryRerecord += rerecord
	s.m.BoundarySkip += skip
	s.mu.Unlock()
}

// NoteShellBoundaryFrame accumulates the shell-tagged boundary cache stats
// . A scrolling body must keep shellRerecord == 0
// while its shell Picture cache Replays.
func (s *MetricsStore) NoteShellBoundaryFrame(shellRerecord, shellSkip int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.ShellRerecord += shellRerecord
	s.m.ShellSkip += shellSkip
	s.mu.Unlock()
}

// NoteSaveLayer accumulates per-frame SaveLayer budget outcomes.
// allow/reject are that frame's accepted / budget-refused SaveLayer pushes.
func (s *MetricsStore) NoteSaveLayer(allow, reject int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.SaveLayerAllow += allow
	s.m.SaveLayerReject += reject
	s.mu.Unlock()
}

// SetCacheBudget records the R14 cache-budget observability sample:
// entries is the current combined boundary-Picture + texture-LRU entry count,
// evictions the cumulative count of budget/LRU/sweep drops.
func (s *MetricsStore) SetCacheBudget(entries, evictions int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.CacheEntries = entries
	s.m.CacheEvictions = evictions
	s.mu.Unlock()
}

// SetOSPresentStats stores the cumulative OS-present counters sampled from
// the swapchain after a real present (dirty region ④ evidence). Backends
// without an OS damage-present protocol keep partial at 0.
func (s *MetricsStore) SetOSPresentStats(partial, full uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.OSPresentPartial = partial
	s.m.OSPresentFull = full
	s.mu.Unlock()
}

// SetTextureRecordBytes stores the cumulative re-record texture byte total
// (dirty region ② upload-cost proxy). Sampled per frame by PipelineApp.
func (s *MetricsStore) SetTextureRecordBytes(b int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.TextureRecordBytes = b
	s.mu.Unlock()
}

// SetBudgetRefusals records the cumulative EnsureCapacity growth refusal
// count. Sampled per frame by PipelineApp.
func (s *MetricsStore) SetBudgetRefusals(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.BudgetRefusals = n
	s.mu.Unlock()
}

// SetBoundaryDiscovery records R3b compositing-bits / boundary walk counts.
func (s *MetricsStore) SetBoundaryDiscovery(count, maxDepth int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.BoundaryCount = int64(count)
	s.m.BoundaryMaxDepth = int64(maxDepth)
	s.mu.Unlock()
}

// SetPictureOpCount records last-frame display-list op count.
func (s *MetricsStore) SetPictureOpCount(n int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PictureOpCount = int64(n)
	s.mu.Unlock()
}

// SetMeasureCacheStats records cumulative text measure hit/miss.
func (s *MetricsStore) SetMeasureCacheStats(hits, misses int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.MeasureCacheHit = hits
	s.m.MeasureCacheMiss = misses
	s.mu.Unlock()
}

// SetFirstPresent records the H-family first-present observation:
// warmup = Open-time warm-up full paint ran; ms = wall ms from Open to first
// present completing; paintCount = pipe paint count at that moment (>0 =
// first frame has content). Call exactly once at the first present.
func (s *MetricsStore) SetFirstPresent(ms float64, warmup bool, paintCount int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m.Warmup = warmup
	s.m.TimeToFirstPresentMs = ms
	s.m.FirstPresentPaintCount = paintCount
}

// percentilesLocked returns p50, p95, and p99 of the interval ring. Caller holds s.mu.
func (s *MetricsStore) percentilesLocked() (p50, p95, p99 float64) {
	n := s.ringN
	if n == 0 {
		return 0, 0, 0
	}
	tmp := percentilePool.Get().([]float64)[:n]
	defer percentilePool.Put(tmp[:intervalRingCap])
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
