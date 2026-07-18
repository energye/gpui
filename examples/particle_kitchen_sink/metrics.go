//go:build linux && !nogpu

package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

type runResult struct {
	Tier             string  `json:"tier"`
	ProbeID          string  `json:"probe_id"`
	ProbeClass       string  `json:"probe_class"`
	Name             string  `json:"name"`
	Seconds          float64 `json:"seconds"`
	Frames           int     `json:"frames"`
	FPSEma           float64 `json:"fps_ema"`
	FPSAvg           float64 `json:"fps_avg"`
	FPSMin           float64 `json:"fps_min"`
	FPSMax           float64 `json:"fps_max"`
	FPSJitter        float64 `json:"fps_jitter"`
	LowFPSRatio      float64 `json:"low_fps_ratio"`
	CPUAvg           float64 `json:"cpu_avg"`
	RSSStartKB       int64   `json:"rss_start_kb"`
	RSSEndKB         int64   `json:"rss_end_kb"`
	RSSSteadyDeltaKB int64   `json:"rss_steady_delta_kb"`
	GPUOps           int     `json:"gpu_ops"`
	CPUFallback      int     `json:"cpu_fallback_ops"`
	LastFB           string  `json:"last_fb"`
	Presents         int     `json:"presents"`
	PresentErrors    int     `json:"present_errors"` // total (resize+steady)
	PresentErrResize int     `json:"present_errors_resize"`
	PresentErrSteady int     `json:"present_errors_steady"`
	LastPresentErr   string  `json:"last_present_err,omitempty"`
	ResizeEvents     int     `json:"resize_events"`
	RecoverFails     int     `json:"resize_recover_fails"`
	ParticleN        int     `json:"particle_n"`
	MinParticleN     int     `json:"min_particle_n"`
	Region           float64 `json:"region"`
	EnableSolid      bool    `json:"enable_solid"`
	EnableBlend      bool    `json:"enable_blend"`
	EnableGlow       bool    `json:"enable_glow"`
	EnableMesh       bool    `json:"enable_mesh"`
	EnableAtlas      bool    `json:"enable_atlas"`
	EnableText       bool    `json:"enable_text"`
	EnableLayer      bool    `json:"enable_layer"`
	EnableTrails     bool    `json:"enable_trails"`
	PerCircleBlend   bool    `json:"per_circle_blend"`
	ResizeOscillate  bool    `json:"resize_oscillate"`
	PathSubmitHeavy  bool    `json:"path_submit_heavy"`
	MultiLayer       int     `json:"multi_layer"`
	AltClear         bool    `json:"alt_clear"`
	GrowN            bool    `json:"grow_n"`
	MaxCPUPct        float64 `json:"max_cpu_pct"`
	MaxJitter        float64 `json:"max_jitter"`
	BlendCircles     int     `json:"blend_circles"`
	ContentOK        bool    `json:"content_ok"`
	ContentNote      string  `json:"content_note,omitempty"`
	PixelOK          bool    `json:"pixel_ok"`
	PixelNote        string  `json:"pixel_note,omitempty"`
	PixelSamples     string  `json:"pixel_samples,omitempty"`
	StageSigOK       bool    `json:"stage_sig_ok"`
	StageSigNote     string  `json:"stage_sig_note,omitempty"`
	SigSamples       int     `json:"sig_samples"`
	SigFails         int     `json:"sig_fails"`
	SigFailRatio     float64 `json:"sig_fail_ratio"`
	ProbeOK          bool    `json:"probe_ok"`
	ProbeNote        string  `json:"probe_note,omitempty"`
	Status           string  `json:"status"`
	FailReason       string  `json:"fail_reason,omitempty"`
	// Diagnostics: non-empty warnings that did not alone fail the run.
	Warnings    []string `json:"warnings,omitempty"`
	AllowLowFPS bool     `json:"allow_low_fps"`
	ExitReason  string   `json:"exit_reason"`
	Features    string   `json:"features"`
	BisectHint  string   `json:"bisect_hint,omitempty"`
	Expect      string   `json:"expect,omitempty"`
}

func writeResult(path string, r runResult) {
	if path == "" {
		return
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
	line := fmt.Sprintf("probe=%s class=%s status=%s fps_ema=%.1f fps_avg=%.1f fps_jit=%.1f cpu=%.0f cpu_fb=%d present_err=%d steady_err=%d resize_err=%d n=%d content=%v pixel=%v reason=%s\n",
		r.ProbeID, r.ProbeClass, r.Status, r.FPSEma, r.FPSAvg, r.FPSJitter, r.CPUAvg, r.CPUFallback,
		r.PresentErrors, r.PresentErrSteady, r.PresentErrResize, r.ParticleN, r.ContentOK, r.PixelOK, r.FailReason)
	_ = os.WriteFile(path+".line", []byte(line), 0o644)
}

// rssSteadyDelta estimates post-warmup RSS climb (KB).
// Same shape as render.memAssertSteadyRSS: drop first 20%, then late-third − early-third.
func rssSteadyDelta(samples []int64) int64 {
	n := len(samples)
	if n < 30 {
		return 0
	}
	start := n / 5
	steady := samples[start:]
	if len(steady) < 9 {
		return 0
	}
	third := len(steady) / 3
	var a, b float64
	for i := 0; i < third; i++ {
		a += float64(steady[i])
	}
	for i := len(steady) - third; i < len(steady); i++ {
		b += float64(steady[i])
	}
	return int64((b / float64(third)) - (a / float64(third)))
}

func judgeResult(r runResult, targetFPS int) (status, reason string) {
	if r.Frames < 30 {
		return "FAIL", "too_few_frames"
	}
	if r.CPUFallback > 0 {
		return "FAIL", fmt.Sprintf("cpu_fallback_ops=%d last=%s", r.CPUFallback, r.LastFB)
	}
	if r.GPUOps <= 0 {
		return "FAIL", "gpu_ops=0"
	}
	// Steady present errors always hard-fail (broken present path).
	if r.PresentErrSteady > 0 {
		return "FAIL", fmt.Sprintf("present_errors_steady=%d last=%s", r.PresentErrSteady, r.LastPresentErr)
	}
	// Resize path: fail if we never recover after a resize event, or too many resize errs.
	if r.RecoverFails > 0 {
		return "FAIL", fmt.Sprintf("resize_recover_fails=%d last=%s", r.RecoverFails, r.LastPresentErr)
	}
	if r.ResizeEvents > 0 && r.PresentErrResize > r.ResizeEvents*3 {
		// more than ~3 grace errors per resize is still a signal
		return "FAIL", fmt.Sprintf("present_errors_resize=%d events=%d last=%s", r.PresentErrResize, r.ResizeEvents, r.LastPresentErr)
	}
	// Non-resize runs: any present error is fail.
	if !r.ResizeOscillate && r.PresentErrors > 0 {
		return "FAIL", fmt.Sprintf("present_errors=%d last=%s", r.PresentErrors, r.LastPresentErr)
	}
	if !r.ProbeOK {
		return "FAIL", "probe_fail:" + r.ProbeNote
	}
	if !r.ContentOK {
		return "FAIL", "content_fail:" + r.ContentNote
	}
	// Pixel fingerprint / stage signature: empty raster is always a hard fail.
	if !r.PixelOK {
		return "FAIL", "pixel_fail:" + r.PixelNote
	}
	if !r.StageSigOK {
		return "FAIL", "stage_sig_fail:" + r.StageSigNote
	}
	// Intermittent empty/wrong content across the run (flicker / dropouts).
	if r.SigSamples >= 4 && r.SigFailRatio > 0.15 {
		return "FAIL", fmt.Sprintf("intermittent_content fails=%d/%d ratio=%.2f last=%s",
			r.SigFails, r.SigSamples, r.SigFailRatio, r.StageSigNote)
	}
	if r.MinParticleN > 0 && r.ParticleN < r.MinParticleN {
		return "FAIL", fmt.Sprintf("content_gutted n=%d min=%d", r.ParticleN, r.MinParticleN)
	}

	if r.ProbeClass == string(classTrap) && r.PerCircleBlend {
		if r.FPSEma < 10 || r.FPSAvg < 8 {
			return "FAIL", fmt.Sprintf("trap_hot_path_still_slow ema=%.1f (per-circle blend ~1fps regression)", r.FPSEma)
		}
	} else if !r.AllowLowFPS {
		lo := float64(targetFPS) - 5
		if r.FPSEma < lo {
			return "FAIL", fmt.Sprintf("fps_low_steady ema=%.1f want>=%.0f", r.FPSEma, lo)
		}
		avgLo := float64(targetFPS) - 12
		if r.FPSAvg < avgLo {
			return "FAIL", fmt.Sprintf("fps_low_avg avg=%.1f want>=%.0f", r.FPSAvg, avgLo)
		}
		// Gate: moderate hitch is a regression. Stress dig: severe hitch still fails
		// even when EMA looks "green" (classic flicker/filter export spikes).
		if r.ProbeClass == string(classGate) && r.LowFPSRatio > 0.12 && r.FPSMin > 0 && r.FPSMin < lo-15 {
			return "FAIL", fmt.Sprintf("fps_hitch_ratio=%.2f min=%.1f jit=%.1f", r.LowFPSRatio, r.FPSMin, r.FPSJitter)
		}
		if r.ProbeClass == string(classStress) && r.LowFPSRatio > 0.25 && r.FPSMin > 0 && r.FPSMin < lo-20 {
			return "FAIL", fmt.Sprintf("fps_hitch_ratio=%.2f min=%.1f jit=%.1f", r.LowFPSRatio, r.FPSMin, r.FPSJitter)
		}
	}

	// CPU budget diagnostic (opt-in per probe).
	if r.MaxCPUPct > 0 && r.CPUAvg > r.MaxCPUPct {
		return "FAIL", fmt.Sprintf("cpu_over_budget avg=%.1f max=%.0f", r.CPUAvg, r.MaxCPUPct)
	}
	// FPS stability dig (opt-in): span of steady inst fps.
	if r.MaxJitter > 0 && r.FPSJitter > r.MaxJitter {
		return "FAIL", fmt.Sprintf("fps_jitter_high span=%.1f max=%.0f min=%.1f maxf=%.1f", r.FPSJitter, r.MaxJitter, r.FPSMin, r.FPSMax)
	}

	if r.RSSSteadyDeltaKB > 512*1024 {
		return "FAIL", fmt.Sprintf("rss_steady_delta_kb=%d", r.RSSSteadyDeltaKB)
	}
	if (r.ProbeID == "P_MEM_SOAK" || r.ProbeID == "P_MEM_LONG" || r.GrowN) && r.RSSSteadyDeltaKB > 128*1024 {
		return "FAIL", fmt.Sprintf("mem_rss_delta_kb=%d", r.RSSSteadyDeltaKB)
	}
	return "PASS", ""
}

// collectWarnings adds non-fatal diagnostic signals into r.Warnings.
func collectWarnings(r *runResult, targetFPS int) {
	if r == nil {
		return
	}
	var w []string
	if r.PresentErrResize > 0 {
		w = append(w, fmt.Sprintf("resize_present_glitch=%d", r.PresentErrResize))
	}
	if r.CPUAvg > 90 && r.MaxCPUPct <= 0 {
		w = append(w, fmt.Sprintf("cpu_high=%.0f", r.CPUAvg))
	}
	if r.AllowLowFPS && r.FPSEma > 0 && r.FPSEma < float64(targetFPS)-15 {
		w = append(w, fmt.Sprintf("fps_stress_low=%.1f", r.FPSEma))
	}
	if r.LowFPSRatio > 0.05 {
		w = append(w, fmt.Sprintf("hitch_ratio=%.2f", r.LowFPSRatio))
	}
	if r.FPSJitter > 25 {
		w = append(w, fmt.Sprintf("fps_jitter=%.1f", r.FPSJitter))
	}
	if r.RSSSteadyDeltaKB > 16*1024 {
		w = append(w, fmt.Sprintf("rss_climb_kb=%d", r.RSSSteadyDeltaKB))
	}
	if r.SigSamples >= 4 && r.SigFailRatio > 0.05 {
		w = append(w, fmt.Sprintf("intermittent_sig=%.2f", r.SigFailRatio))
	}
	r.Warnings = w
}

func fpsSpan(min, max float64) float64 {
	if min <= 0 || max <= 0 {
		return 0
	}
	return math.Max(0, max-min)
}

func joinWarn(w []string) string {
	return strings.Join(w, ";")
}
