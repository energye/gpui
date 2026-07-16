//go:build linux && !nogpu

package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"

	"github.com/energye/gpui/render"
)

type metricsWriter struct {
	f  *os.File
	w  *csv.Writer
	ok bool
}

func openMetrics(path string) *metricsWriter {
	if path == "" {
		return &metricsWriter{}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("metrics file: %v", err)
		return &metricsWriter{}
	}
	w := csv.NewWriter(f)
	st, _ := f.Stat()
	if st != nil && st.Size() == 0 {
		_ = w.Write([]string{
			"scenario", "t_s", "frame", "fps_ema", "fps_avg", "work_ms", "cpu_pct",
			"rss_kb", "gpu_ops", "cpu_fb", "last_fb", "presents", "reconfig", "active",
		})
		w.Flush()
	}
	return &metricsWriter{f: f, w: w, ok: true}
}

func (m *metricsWriter) write(scenario string, t float64, frame int, fpsEMA, fpsAvg, workMS, cpuPct float64, rss int64, gpuOps, cpuFb int, lastFb string, presents, reconfig int, active string) {
	if m == nil || !m.ok {
		return
	}
	_ = m.w.Write([]string{
		scenario,
		fmt.Sprintf("%.2f", t),
		strconv.Itoa(frame),
		fmt.Sprintf("%.2f", fpsEMA),
		fmt.Sprintf("%.2f", fpsAvg),
		fmt.Sprintf("%.2f", workMS),
		fmt.Sprintf("%.1f", cpuPct),
		strconv.FormatInt(rss, 10),
		strconv.Itoa(gpuOps),
		strconv.Itoa(cpuFb),
		lastFb,
		strconv.Itoa(presents),
		strconv.Itoa(reconfig),
		active,
	})
	m.w.Flush()
}

func (m *metricsWriter) close() {
	if m != nil && m.f != nil {
		_ = m.f.Close()
	}
}

type runResult struct {
	Scenario         string  `json:"scenario"`
	Name             string  `json:"name"`
	Seconds          float64 `json:"seconds"`
	Frames           int     `json:"frames"`
	FPSEma           float64 `json:"fps_ema"`
	FPSAvg           float64 `json:"fps_avg"`
	CPUAvg           float64 `json:"cpu_avg"`
	RSSStartKB       int64   `json:"rss_start_kb"`
	RSSEndKB         int64   `json:"rss_end_kb"`
	RSSDeltaKB       int64   `json:"rss_delta_kb"`
	RSSSteadyDeltaKB int64   `json:"rss_steady_delta_kb"`
	GPUOps           int     `json:"gpu_ops"`
	CPUFallback      int     `json:"cpu_fallback_ops"`
	LastFB           string  `json:"last_fb"`
	Presents         int     `json:"presents"`
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
	line := fmt.Sprintf("scenario=%s name=%s seconds=%.1f frames=%d fps_ema=%.1f fps_avg=%.1f cpu_avg=%.0f rss_start_kb=%d rss_end_kb=%d rss_delta_kb=%d rss_steady_delta_kb=%d cpu_fb=%d gpu_ops=%d status=%s exit=%s\n",
		r.Scenario, r.Name, r.Seconds, r.Frames, r.FPSEma, r.FPSAvg, r.CPUAvg, r.RSSStartKB, r.RSSEndKB, r.RSSDeltaKB, r.RSSSteadyDeltaKB, r.CPUFallback, r.GPUOps, r.Status, r.ExitReason)
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
	if !r.AllowLowFPS {
		// Plan gate G-FPS is STEADY-state: fps_ema (late frames), not full-run avg
		// which is dragged by pipeline/font/atlas warmup in the first seconds.
		lo := float64(targetFPS) - 5 // 55 at 60fps
		if r.FPSEma < lo {
			return "FAIL", fmt.Sprintf("fps_low_steady ema=%.1f avg=%.1f want_ema>=%.0f", r.FPSEma, r.FPSAvg, lo)
		}
		// Catch broken pacing (busy-spin / catch-up) that inflates ema while avg lags.
		if r.FPSEma > float64(targetFPS)+15 {
			return "FAIL", fmt.Sprintf("fps_runaway ema=%.1f avg=%.1f", r.FPSEma, r.FPSAvg)
		}
		// Full-run avg must not collapse (warmup-tolerant: target-12).
		avgLo := float64(targetFPS) - 12 // 48 at 60fps
		if r.FPSAvg < avgLo {
			return "FAIL", fmt.Sprintf("fps_low_avg ema=%.1f avg=%.1f want_avg>=%.0f", r.FPSEma, r.FPSAvg, avgLo)
		}
	}
	// Steady RSS slope hard cap (~512MB over the steady window) — leak guard.
	if r.RSSSteadyDeltaKB > 512*1024 {
		return "FAIL", fmt.Sprintf("rss_steady_delta_kb=%d", r.RSSSteadyDeltaKB)
	}
	return "PASS", ""
}

func drawDensityField(dc *render.Context, w, h int, t float64, frame, density int) {
	if density <= 0 || dc == nil {
		return
	}
	fw, fh := float64(w), float64(h)
	n := density
	if n > 2500 {
		n = 2500
	}
	cols := int(math.Sqrt(float64(n))) + 1
	rows := (n + cols - 1) / cols
	cellW := fw / float64(cols)
	cellH := fh / float64(rows)
	r := math.Min(cellW, cellH) * 0.28
	if r < 1.5 {
		r = 1.5
	}
	for i := 0; i < n; i++ {
		cx := (float64(i%cols) + 0.5) * cellW
		cy := (float64(i/cols) + 0.5) * cellH
		phase := t*1.7 + float64(i)*0.015
		ox := cx + math.Sin(phase)*cellW*0.15
		oy := cy + math.Cos(phase*0.9)*cellH*0.15
		hr, hg, hb := hsv(math.Mod(float64(i)*0.0017+t*0.02, 1), 0.65, 0.95)
		dc.SetRGBA(hr, hg, hb, 0.55)
		if i%5 == frame%5 {
			dc.DrawCircle(ox, oy, r)
			_ = dc.Fill()
		} else if i%7 == 0 {
			dc.SetLineWidth(1)
			dc.DrawLine(ox-r, oy, ox+r, oy)
			_ = dc.Stroke()
		}
	}
}
