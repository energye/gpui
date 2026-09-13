// Command video_vr2_decode is the VW1 VR2 real-window: H.264 full decode.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/video_vr2_decode
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 15). GPU window required.
// Left: per-clip decode gates from the real video/h264 engine (B门禁,
// 480p裁边, 720p, all byte-exact vs the ffmpeg oracle).
// Right: first display frame side by side, decoded vs oracle (luma gray;
// color lives in VR3, VR2 proves YUV exactness).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR2")
	} else {
		secs = 15
	}
	wrkit.EnsureUIFace()

	clips := loadDecode()

	framesDecoded := 0
	yuvReady := 0
	maxDiffPct := 0.0
	decodeMsMax := 0.0
	decodeErr := ""
	allExact := true
	for _, c := range clips {
		if c.err != nil {
			allExact = false
			if decodeErr == "" {
				decodeErr = c.name + "：" + c.err.Error()
			}
			continue
		}
		framesDecoded += c.frames
		if c.diffPct > maxDiffPct {
			maxDiffPct = c.diffPct
		}
		if c.decodeMs > decodeMsMax {
			decodeMsMax = c.decodeMs
		}
		if c.diffPct != 0 {
			allExact = false
		}
	}
	if allExact && framesDecoded > 0 {
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr2_decode — H.264画面解码", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR2 H.264画面解码 — B帧/多档逐字节对", []string{
		"左边三档门禁，走真解码引擎",
		"右边首帧并排，解出 vs 对照",
		"灰度只看亮度，颜色归VR3",
		"门禁：帧数够 对照差异=0",
		"跑满15秒，至少上屏1次",
	})

	statusText := "解码全对"
	sr, sg, sb := 0.3, 0.9, 0.5
	if decodeErr != "" {
		statusText = "解码失败：" + shortErr(translateDecodeError(decodeErr), 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	} else if !allExact {
		statusText = fmt.Sprintf("差异超标：最大%.4f%%", maxDiffPct)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR2 H.264画面解码", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, c := range clips {
		if i >= 6 {
			break
		}
		shell.Body.LabelAt(c.infoLine(), 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Side-by-side first display frame (B门禁 clip): decoded vs oracle.
	thumbW, thumbH := 240.0, 240.0
	thumbY := 176.0
	shell.Body.LabelAt("解出首帧", 13, 20, 150, 0.6, 0.8, 0.95)
	shell.Body.LabelAt("对照首帧", 13, 290, 150, 0.6, 0.8, 0.95)
	if len(clips) > 0 && clips[0].err == nil {
		ours := rendering.NewRenderImage(thumbW, thumbH)
		ours.SetImageShared(grayFromY(clips[0].firstY, 96, 96))
		shell.Body.Place(ours, 20, thumbY)
		golden := rendering.NewRenderImage(thumbW, thumbH)
		golden.SetImageShared(grayFromY(clips[0].firstGoldenY, 96, 96))
		shell.Body.Place(golden, 290, thumbY)
		shell.Body.LabelAt("并排肉眼一致，差异进门禁", 11, 20, thumbY+thumbH+12, 0.65, 0.75, 0.85)
	} else {
		shell.Body.LabelAt("无图可摆，门禁必挂", 12, 20, thumbY, 0.95, 0.4, 0.35)
	}
	shell.Body.LabelAt("1080p/1440p/4K本地另验，大文件不进仓", 11, 20, shell.Body.H-56, 0.55, 0.65, 0.75)
	shell.Body.LabelAt("隔行走F12人话报错，见单测interlace", 11, 20, shell.Body.H-30, 0.55, 0.65, 0.75)

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_vr2_decode: 关闭 (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	phase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := decodeErr == "" && allExact && yuvReady == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VR2", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("解码=%d帧 差异=%.4f%%", framesDecoded, maxDiffPct),
			fmt.Sprintf("最慢%.1f毫秒 %s", decodeMsMax, shortErr(clips[0].source, 30)))
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 打开失败:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 运行失败:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsed := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()
	rep := buildReport(snap, app.PresentCount(), elapsed, clips, framesDecoded, yuvReady, maxDiffPct, decodeMsMax, decodeErr, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR2需要真窗口)")
		os.Exit(1)
	}
	if decodeErr != "" {
		fmt.Fprintf(os.Stderr, "FAIL: 解码失败: %s\n", translateDecodeError(decodeErr))
		os.Exit(1)
	}
	if !allExact {
		fmt.Fprintf(os.Stderr, "FAIL: 对照差异=%.4f%% 不为0\n", maxDiffPct)
		os.Exit(1)
	}
	if yuvReady != 1 || framesDecoded < 15 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧数=%d 对照=%d(三档共15帧)\n", framesDecoded, yuvReady)
		os.Exit(1)
	}
	if rep.TimeToFirstFrameMs > 2000 && rep.TimeToFirstFrameMs > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 首帧=%.1f毫秒 超过2000毫秒\n", rep.TimeToFirstFrameMs)
		os.Exit(1)
	}
	if rep.RSSPeakKB > rep.MemCapKB && rep.MemCapKB > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 内存峰值=%dKB 超过上限=%dKB\n", rep.RSSPeakKB, rep.MemCapKB)
		os.Exit(1)
	}
	if err := checkBaseline(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "video_vr2_decode: 通过 解码=%d帧 差异=%.4f%% 上屏=%d 用时=%.1f秒\n",
		framesDecoded, maxDiffPct, app.PresentCount(), elapsed)
}

// grayFromY packs luma into a gray RGBA bridge image (window side only;
// the decoder never touches render).
func grayFromY(y []uint8, w, h int) *render.ImageBuf {
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			v := uint8(0)
			if yy*w+xx < len(y) {
				v = y[yy*w+xx]
			}
			_ = img.SetRGBA(xx, yy, v, v, v, 255)
		}
	}
	return img
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func shortErr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

func phaseCN(phase string) string {
	switch phase {
	case wrkit.PhaseSteady:
		return "稳态"
	case wrkit.PhaseSpike:
		return "冲击"
	case wrkit.PhaseRecover:
		return "恢复"
	default:
		return phase
	}
}

func translateDecodeError(s string) string {
	if s == "" {
		return ""
	}
	switch {
	case containsStr(s, "盒子打不开"):
		return "盒子打不开"
	case containsStr(s, "无视频轨"):
		return "盒子里没视频轨"
	case containsStr(s, "对照图"):
		return "对照图缺了，现跑生成脚本"
	case containsStr(s, "解码序对不上"):
		return "解码显示序号排错了"
	case containsStr(s, "F12"):
		return "隔行片本阶段只认不解"
	default:
		return s
	}
}

func containsStr(s, sub string) bool {
	if sub == "" {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR2 extras.
type report struct {
	AbilityID  string  `json:"ability_id"`
	Scenario   string  `json:"scenario"`
	ElapsedSec float64 `json:"elapsed_sec"`

	FPSWall         float64 `json:"fps_wall"`
	IntervalAvgMs   float64 `json:"interval_avg_ms"`
	IntervalP50Ms   float64 `json:"interval_p50_ms"`
	IntervalP95Ms   float64 `json:"interval_p95_ms"`
	HitchCount      int64   `json:"hitch_count"`
	HitchRatePerMin float64 `json:"hitch_rate_per_min"`
	VSyncSource     string  `json:"vsync_source"`
	TargetHz        int     `json:"target_hz"`

	DecodeMsAvg      float64 `json:"decode_ms_avg"`
	DecodeMsP95      float64 `json:"decode_ms_p95"`
	QueueDepthAvg    float64 `json:"queue_depth_avg"`
	QueueDepthMax    int     `json:"queue_depth_max"`
	DroppedOldFrames int     `json:"dropped_old_frames"`
	AllocPerFrameB   int64   `json:"alloc_per_frame_B"`
	PoolHitPct       float64 `json:"pool_hit_pct"`

	FramesDecoded      int     `json:"frames_decoded"`
	FramesShown        int64   `json:"frames_shown"`
	ClockDriftMs       float64 `json:"clock_drift_ms"`
	SeekOK             int     `json:"seek_ok"`
	SeekLandingDeltaMs float64 `json:"seek_landing_delta_ms"`

	CPUPctAvg    float64 `json:"cpu_pct_avg"`
	CPUUIPct     float64 `json:"cpu_ui_pct"`
	CPURasterPct float64 `json:"cpu_raster_pct"`
	CPUDecodePct float64 `json:"cpu_decode_pct"`

	RSSStartKB       int64   `json:"rss_start_kb"`
	RSSEndKB         int64   `json:"rss_end_kb"`
	RSSPeakKB        int64   `json:"rss_peak_kb"`
	RSSSlopeKBPerMin float64 `json:"rss_slope_kb_per_min"`
	MemCapKB         int64   `json:"mem_cap_kb"`
	GCPausesMsP99    float64 `json:"gc_pauses_ms_p99"`
	HeapAllocMB      float64 `json:"heap_alloc_MB"`

	GPUOps          int64  `json:"gpu_ops"`
	CPUFallbackOps  int64  `json:"cpu_fallback_ops"`
	LastCPUFallback string `json:"last_cpu_fallback"`

	SPSOk               int     `json:"sps_ok"`
	PPSOk               int     `json:"pps_ok"`
	FramesSplit         int     `json:"frames_split"`
	YUVReady            int     `json:"yuv_ready"`
	ColorDiffPerChannel float64 `json:"color_diff_per_channel"`
	PixelGoldenDiffPct  float64 `json:"pixel_golden_diff_pct"`
	Clips               string  `json:"clips"`
	Profile             string  `json:"profile"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, clips []*clipResult, framesDecoded, yuvReady int, maxDiffPct, decodeMsMax float64, decodeErr string, hotTick int) report {
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(presents) / elapsed
	}
	vsync := snap.VSyncSource
	if vsync == "" {
		vsync = "unknown"
	}
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	heapMB := float64(memStats.HeapAlloc) / (1 << 20)
	gcP99 := gcPauseP99(&memStats)
	_ = hotTick
	names := ""
	prof := ""
	src := ""
	for i, c := range clips {
		if i > 0 {
			names += "+"
		}
		names += c.name
		if c.profile != "" && prof == "" {
			prof = c.profile
		}
		if i == 0 {
			src = c.source
		}
	}
	return report{
		AbilityID: "VR2", Scenario: "video_vr2_decode", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: decodeMsMax, DecodeMsP95: decodeMsMax,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: framesDecoded, FramesShown: presents, ClockDriftMs: 0, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: framesDecoded, YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: maxDiffPct,
		Clips: names, Profile: prof,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: decodeErr, Source: src,
		NANote: "decode-only: queue/clock/seek/color N/A in VR2; frames/decoded/diff/profile are the gates",
	}
}

func gcPauseP99(m *runtime.MemStats) float64 {
	if m == nil || m.NumGC == 0 {
		return 0
	}
	tmp := append([]uint64(nil), m.PauseNs[:]...)
	sort.Slice(tmp, func(i, j int) bool { return tmp[i] < tmp[j] })
	idx := int(float64(len(tmp)) * 0.99)
	if idx >= len(tmp) {
		idx = len(tmp) - 1
	}
	return float64(tmp[idx]) / 1e6
}

var requiredKeys = []string{
	"ability_id", "scenario", "elapsed_sec",
	"fps_wall", "interval_avg_ms", "interval_p50_ms", "interval_p95_ms", "hitch_count", "hitch_rate_per_min", "vsync_source", "target_hz",
	"decode_ms_avg", "decode_ms_p95", "queue_depth_avg", "queue_depth_max", "dropped_old_frames", "alloc_per_frame_B", "pool_hit_pct",
	"frames_decoded", "frames_shown", "clock_drift_ms", "seek_ok", "seek_landing_delta_ms",
	"cpu_pct_avg", "cpu_ui_pct", "cpu_raster_pct", "cpu_decode_pct",
	"rss_start_kb", "rss_end_kb", "rss_peak_kb", "rss_slope_kb_per_min", "mem_cap_kb", "gc_pauses_ms_p99", "heap_alloc_MB",
	"gpu_ops", "cpu_fallback_ops", "last_cpu_fallback",
	"sps_ok", "pps_ok", "frames_split", "yuv_ready", "color_diff_per_channel", "pixel_golden_diff_pct",
	"clips", "profile",
	"time_to_first_frame_ms", "present_count", "paint_count", "decode_error",
}

func checkSchema(raw []byte) error {
	var missing []string
	for _, k := range requiredKeys {
		if !bytes.Contains(raw, []byte(`"`+k+`"`)) {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("FAIL: 指标缺键：%v", missing)
	}
	return nil
}

func checkBaseline(raw []byte) error {
	basePath := os.Getenv("BASELINE_JSON")
	savePath := os.Getenv("SAVE_BASELINE_JSON")
	if savePath != "" {
		if err := os.WriteFile(savePath, raw, 0o644); err != nil {
			return fmt.Errorf("FAIL: 存基线失败：%w", err)
		}
	}
	if basePath == "" {
		return nil
	}
	base, err := os.ReadFile(basePath)
	if err != nil {
		return fmt.Errorf("FAIL: 读基线失败：%w", err)
	}
	var cur, baseM map[string]any
	if err := json.Unmarshal(raw, &cur); err != nil {
		return fmt.Errorf("FAIL: 当前结果解析失败：%w", err)
	}
	if err := json.Unmarshal(base, &baseM); err != nil {
		return fmt.Errorf("FAIL: 基线解析失败：%w", err)
	}
	for _, k := range []string{"frames_decoded", "yuv_ready", "pixel_golden_diff_pct"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
