// Package wrgate builds ENGINE_UI_WIDGET_RENDER §2.2 metrics reports and FAIL gates
// for ui_wr_* real-window examples (W0+).
//
// Pure helpers are unit-tested without a display; examples wire PipelineApp + ProcessTracker.
package wrgate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/energye/gpui/ui/scheduler"
)

// Report is the §2.2 JSON shell emitted by every ui_wr_* window example.
type Report struct {
	AbilityID     string  `json:"ability_id"`
	Scenario      string  `json:"scenario"`
	PresentPolicy string  `json:"present_policy"`
	TargetHz      float64 `json:"target_hz"`
	// FPSWall is present_count/elapsed_sec (includes open/close overhead — often low).
	FPSWall float64 `json:"fps_wall"`
	// FPSInterval is 1000/interval_avg_ms when samples exist (honest steady-frame rate).
	FPSInterval      float64 `json:"fps_interval,omitempty"`
	IntervalAvgMs    float64 `json:"interval_avg_ms"`
	IntervalP50Ms    float64 `json:"interval_p50_ms"`
	IntervalP95Ms    float64 `json:"interval_p95_ms"`
	IntervalP99Ms    float64 `json:"interval_p99_ms"`
	HitchCount       int64   `json:"hitch_count"`
	HitchRatePerMin  float64 `json:"hitch_rate_per_min"`
	VSyncSource      string  `json:"vsync_source"`
	FrameBuildMs     float64 `json:"frame_build_ms"`
	FrameRasterMs    float64 `json:"frame_raster_ms"`
	PipelineDepth    int     `json:"pipeline_depth"`
	PipelineMax      int     `json:"pipeline_max"`
	LayoutCount      int64   `json:"layout_count"`
	PaintCount       int64   `json:"paint_count"`
	DamageAreaPx     int64   `json:"damage_area_px"`
	DamageRatio      float64 `json:"damage_ratio"`
	PresentMode      string  `json:"present_mode"`
	PresentCount     int64   `json:"present_count"`
	FrameCount       int64   `json:"frame_count"`
	CPUPctAvg        float64 `json:"cpu_pct_avg"`
	CPUUIPct         float64 `json:"cpu_ui_pct"`
	CPURasterPct     float64 `json:"cpu_raster_pct"`
	CPUUnavailable   bool    `json:"cpu_unavailable,omitempty"`
	RSSStartKB       int64   `json:"rss_start_kb"`
	RSSEndKB         int64   `json:"rss_end_kb"`
	RSSPeakKB        int64   `json:"rss_peak_kb"`
	RSSSlopeKBPerMin float64 `json:"rss_slope_kb_per_min"`
	RSSAfterCloseKB  int64   `json:"rss_after_close_kb,omitempty"`
	RSSUnavailable   bool    `json:"rss_unavailable,omitempty"`
	GPUOps           int64   `json:"gpu_ops"`
	CPUFallbackOps   int64   `json:"cpu_fallback_ops"`
	FrameFlushes     int64   `json:"frame_flushes,omitempty"`
	LastCPUFallback  string  `json:"last_cpu_fallback,omitempty"`
	Warmup           bool    `json:"warmup"`
	ElapsedSec       float64 `json:"elapsed_sec"`
	// W1 boundary cache (from MetricsStore; also mirrored in ability_extra).
	BoundaryRerecord int64 `json:"boundary_rerecord"`
	BoundarySkip     int64 `json:"boundary_skip"`
	BoundaryCount    int64 `json:"boundary_count,omitempty"`
	BoundaryMaxDepth int64 `json:"boundary_max_depth,omitempty"`
	// AbilityExtra holds R-specific keys (optional).
	AbilityExtra map[string]any `json:"ability_extra,omitempty"`
}

// BuildInput is everything needed to assemble a Report from a finished run.
type BuildInput struct {
	AbilityID     string
	Scenario      string
	Snap          scheduler.FrameMetrics
	PresentCount  int64
	ElapsedSec    float64
	SurfaceAreaPx int64 // logical w*h; 0 → damage_ratio 0
	Warmup        bool
	Extra         map[string]any
}

// BuildReport maps a metrics snapshot into the §2.2 shell.
func BuildReport(in BuildInput) Report {
	el := in.ElapsedSec
	if el < 0.001 {
		el = 0.001
	}
	fpsWall := float64(in.PresentCount) / el
	var fpsInterval float64
	if in.Snap.AvgFrameIntervalMs > 1e-6 {
		fpsInterval = 1000.0 / in.Snap.AvgFrameIntervalMs
	}
	var dmgRatio float64
	if in.SurfaceAreaPx > 0 && in.Snap.DamageAreaPx > 0 {
		dmgRatio = float64(in.Snap.DamageAreaPx) / float64(in.SurfaceAreaPx)
	}
	pol := in.Snap.PresentPolicy
	if pol == "" {
		pol = scheduler.PresentPolicyFullPaint
	}
	r := Report{
		AbilityID:        in.AbilityID,
		Scenario:         in.Scenario,
		PresentPolicy:    pol,
		TargetHz:         60,
		FPSWall:          fpsWall,
		FPSInterval:      fpsInterval,
		IntervalAvgMs:    in.Snap.AvgFrameIntervalMs,
		IntervalP50Ms:    in.Snap.P50FrameIntervalMs,
		IntervalP95Ms:    in.Snap.P95FrameIntervalMs,
		IntervalP99Ms:    in.Snap.P99FrameIntervalMs,
		HitchCount:       in.Snap.HitchCount,
		HitchRatePerMin:  in.Snap.HitchRatePerMin,
		VSyncSource:      in.Snap.VSyncSource,
		FrameBuildMs:     in.Snap.LastBuildMs,
		FrameRasterMs:    in.Snap.LastRasterMs,
		PipelineDepth:    in.Snap.PipelineDepth,
		PipelineMax:      in.Snap.PipelineMax,
		LayoutCount:      in.Snap.LayoutCount,
		PaintCount:       in.Snap.PaintCount,
		DamageAreaPx:     in.Snap.DamageAreaPx,
		DamageRatio:      dmgRatio,
		PresentMode:      in.Snap.PresentMode,
		PresentCount:     in.PresentCount,
		FrameCount:       in.Snap.FrameCount,
		CPUPctAvg:        in.Snap.CPUPctAvg,
		CPUUIPct:         in.Snap.CPUUIPct,
		CPURasterPct:     in.Snap.CPURasterPct,
		RSSStartKB:       in.Snap.RSSStartKB,
		RSSEndKB:         in.Snap.RSSEndKB,
		RSSPeakKB:        in.Snap.RSSPeakKB,
		RSSSlopeKBPerMin: in.Snap.RSSSlopeKBPerMin,
		RSSAfterCloseKB:  in.Snap.RSSAfterCloseKB,
		GPUOps:           in.Snap.GPUOps,
		CPUFallbackOps:   in.Snap.CPUFallbackOps,
		FrameFlushes:     in.Snap.FrameFlushes,
		LastCPUFallback:  in.Snap.LastCPUFallbackReason,
		Warmup:           in.Warmup,
		ElapsedSec:       el,
		BoundaryRerecord: in.Snap.BoundaryRerecord,
		BoundarySkip:     in.Snap.BoundarySkip,
		BoundaryCount:    in.Snap.BoundaryCount,
		BoundaryMaxDepth: in.Snap.BoundaryMaxDepth,
		AbilityExtra:     in.Extra,
	}
	if r.RSSStartKB == 0 && r.RSSEndKB == 0 && r.RSSPeakKB == 0 {
		r.RSSUnavailable = true
	}
	if r.CPUPctAvg == 0 && r.CPUUIPct == 0 && r.CPURasterPct == 0 && in.Snap.LastBuildMs == 0 && in.Snap.LastRasterMs == 0 {
		// May still be legitimate zeros; mark unavailable only when no process sample path.
		// Leave cpu_unavailable false if path proxies exist from notes.
	}
	if r.VSyncSource == "" {
		r.VSyncSource = "unknown"
	}
	return r
}

// Marshal formats the report as compact JSON.
func Marshal(r Report) ([]byte, error) {
	return json.Marshal(r)
}

// RequiredSchemaKeys are always required in emitted JSON (R12 / C0 schema gate).
var RequiredSchemaKeys = []string{
	"ability_id", "scenario", "present_policy", "target_hz", "fps_wall",
	"interval_p50_ms", "interval_p95_ms", "hitch_count", "hitch_rate_per_min", "vsync_source",
	"frame_build_ms", "frame_raster_ms", "layout_count", "paint_count",
	"damage_area_px", "damage_ratio", "present_mode", "present_count", "frame_count",
	"cpu_pct_avg", "cpu_ui_pct", "cpu_raster_pct",
	"rss_start_kb", "rss_end_kb", "rss_peak_kb", "rss_slope_kb_per_min",
	"gpu_ops", "cpu_fallback_ops", "elapsed_sec",
}

// CheckSchema ensures the marshaled JSON contains required keys (string search).
func CheckSchema(jsonBytes []byte) error {
	s := string(jsonBytes)
	var missing []string
	for _, k := range RequiredSchemaKeys {
		needle := `"` + k + `"`
		if !strings.Contains(s, needle) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("FAIL: metrics schema missing keys: %s", strings.Join(missing, ", "))
	}
	return nil
}

// GateOptions configures FAIL thresholds for a window run.
type GateOptions struct {
	// MinPresents fails when present_count < MinPresents (default 1).
	MinPresents int64
	// RequireFullPaintPolicy fails when present_policy != full_paint.
	RequireFullPaintPolicy bool
	// RequirePersistentFPS applies fps_wall >= MinFPSWall (default 55) when ElapsedSec >= MinFPSElapsed.
	RequirePersistentFPS bool
	MinFPSWall           float64
	MinFPSElapsed        float64 // default 2s — short smokes skip FPS hard gate
	// MaxP95Ms when >0 fails if interval_p95_ms > MaxP95Ms (animation/scroll).
	MaxP95Ms float64
	// SchemaOnly only checks schema (R12); skips present/FPS gates when true.
	SchemaOnly bool
	// MinBoundarySkip fails when boundary_skip < N (R3; 0 = off).
	MinBoundarySkip int64
	// MinBoundaryRerecord fails when boundary_rerecord < N (dirty path must record; 0 = off).
	MinBoundaryRerecord int64
	// MinBoundaryCount fails when boundary_count < N (R3b; 0 = off).
	MinBoundaryCount int64
	// MinBoundaryMaxDepth fails when boundary_max_depth < N (R3b nest; 0 = off).
	MinBoundaryMaxDepth int64
}

// EvaluateGates returns a FAIL error or nil. Pure: no I/O.
func EvaluateGates(r Report, opt GateOptions) error {
	b, err := Marshal(r)
	if err != nil {
		return fmt.Errorf("FAIL: marshal metrics: %w", err)
	}
	if err := CheckSchema(b); err != nil {
		return err
	}
	if opt.SchemaOnly {
		return nil
	}
	minP := opt.MinPresents
	if minP <= 0 {
		minP = 1
	}
	if r.PresentCount < minP {
		return fmt.Errorf("FAIL: present_count=%d want >=%d (no frames after open)", r.PresentCount, minP)
	}
	if opt.RequireFullPaintPolicy {
		if r.PresentPolicy != scheduler.PresentPolicyFullPaint {
			return fmt.Errorf("FAIL: present_policy=%q want %q", r.PresentPolicy, scheduler.PresentPolicyFullPaint)
		}
	}
	minEl := opt.MinFPSElapsed
	if minEl <= 0 {
		minEl = 5 // U16: default align with min RUN_SECONDS
	}
	minFPS := opt.MinFPSWall
	if minFPS <= 0 {
		minFPS = 55
	}
	if opt.RequirePersistentFPS && r.ElapsedSec >= minEl {
		// Prefer interval-derived FPS (steady frame pace); wall FPS includes teardown.
		fps := r.FPSInterval
		if fps <= 0 {
			fps = r.FPSWall
		}
		if fps < minFPS {
			return fmt.Errorf("FAIL: fps=%.2f (interval=%.2f wall=%.2f) < %.2f (60Hz-class gate)",
				fps, r.FPSInterval, r.FPSWall, minFPS)
		}
		if opt.MaxP95Ms > 0 && r.IntervalP95Ms > opt.MaxP95Ms && r.IntervalP95Ms > 0 {
			return fmt.Errorf("FAIL: interval_p95_ms=%.2f > %.2f", r.IntervalP95Ms, opt.MaxP95Ms)
		}
	}
	if opt.MinBoundarySkip > 0 && r.BoundarySkip < opt.MinBoundarySkip {
		return fmt.Errorf("FAIL: boundary_skip=%d want >=%d (clean boundary must Replay)", r.BoundarySkip, opt.MinBoundarySkip)
	}
	if opt.MinBoundaryRerecord > 0 && r.BoundaryRerecord < opt.MinBoundaryRerecord {
		return fmt.Errorf("FAIL: boundary_rerecord=%d want >=%d (dirty boundary must re-record)", r.BoundaryRerecord, opt.MinBoundaryRerecord)
	}
	if opt.MinBoundaryCount > 0 && r.BoundaryCount < opt.MinBoundaryCount {
		return fmt.Errorf("FAIL: boundary_count=%d want >=%d", r.BoundaryCount, opt.MinBoundaryCount)
	}
	if opt.MinBoundaryMaxDepth > 0 && r.BoundaryMaxDepth < opt.MinBoundaryMaxDepth {
		return fmt.Errorf("FAIL: boundary_max_depth=%d want >=%d", r.BoundaryMaxDepth, opt.MinBoundaryMaxDepth)
	}
	return nil
}
