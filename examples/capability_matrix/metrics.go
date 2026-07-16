//go:build linux && !nogpu

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type runResult struct {
	Scenario         string  `json:"scenario"`
	Name             string  `json:"name"`
	MatrixIDs        string  `json:"matrix_ids"`
	Seconds          float64 `json:"seconds"`
	Frames           int     `json:"frames"`
	FPSEma           float64 `json:"fps_ema"`
	FPSAvg           float64 `json:"fps_avg"`
	CPUAvg           float64 `json:"cpu_avg"`
	RSSStartKB       int64   `json:"rss_start_kb"`
	RSSEndKB         int64   `json:"rss_end_kb"`
	RSSSteadyDeltaKB int64   `json:"rss_steady_delta_kb"`
	GPUOps           int     `json:"gpu_ops"`
	CPUFallback      int     `json:"cpu_fallback_ops"`
	LastFB           string  `json:"last_fb"`
	Presents         int     `json:"presents"`
	ProbeOK          bool    `json:"probe_ok"`
	ProbeNote        string  `json:"probe_note,omitempty"`
	Status           string  `json:"status"`
	FailReason       string  `json:"fail_reason,omitempty"`
	AllowLowFPS      bool    `json:"allow_low_fps"`
	ExitReason       string  `json:"exit_reason"`
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
	line := fmt.Sprintf("scenario=%s status=%s fps_ema=%.1f fps_avg=%.1f cpu=%.0f cpu_fb=%d gpu_ops=%d probe=%v reason=%s\n",
		r.Scenario, r.Status, r.FPSEma, r.FPSAvg, r.CPUAvg, r.CPUFallback, r.GPUOps, r.ProbeOK, r.FailReason)
	_ = os.WriteFile(path+".line", []byte(line), 0o644)
}

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
	if !r.ProbeOK {
		return "FAIL", "probe_fail:" + r.ProbeNote
	}
	if !r.AllowLowFPS {
		lo := float64(targetFPS) - 5
		if r.FPSEma < lo {
			return "FAIL", fmt.Sprintf("fps_low_steady ema=%.1f want>=%.0f", r.FPSEma, lo)
		}
		avgLo := float64(targetFPS) - 12
		if r.FPSAvg < avgLo {
			return "FAIL", fmt.Sprintf("fps_low_avg avg=%.1f want>=%.0f", r.FPSAvg, avgLo)
		}
	}
	if r.RSSSteadyDeltaKB > 512*1024 {
		return "FAIL", fmt.Sprintf("rss_steady_delta_kb=%d", r.RSSSteadyDeltaKB)
	}
	return "PASS", ""
}
