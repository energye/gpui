// Command video_vr3_color is the VW1 VR3 real-window: YUV to RGBA color.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/video_vr3_color
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 5). GPU window required.
// Left: vector gates from the real video/color engine (grey ramp, RGB
// patches, skin, both ranges, 601+709). Right: ours-vs-theory patches plus
// one real decoded I frame in color.
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
	"github.com/energye/gpui/video/color"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

// VR3 pass line (§12.1, ffmpeg-anchored, no self-set numbers): vectors stay
// byte-exact (maxDiff==0); real frames equal the committed baseline md5s in
// video/testdata/vr3_ffmpeg.json, whose ffmpeg gap is R/B P99<=2, G P99<=3
// (peer: libswscale/yuv2rgb.c + output.c + swscale.c, see README).
const vectorExact = 0

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR3")
	} else {
		secs = 5
	}
	wrkit.EnsureUIFace()

	gate := loadGate()

	yuvReady := 0
	colorErr := ""
	if gate.err != nil {
		colorErr = gate.err.Error()
	} else if gate.parityErr != "" {
		colorErr = gate.parityErr
	} else if gate.maxDiff == vectorExact && gate.parityOk && gate.frames > 0 {
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr3_color — YUV转RGBA颜色", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR3 YUV转RGBA颜色 — 色卡灰阶真帧", []string{
		"左边门禁，走真转色引擎",
		"右边色块，转出 vs 理论",
		"灰阶肤色三原色全覆盖",
		"门禁：向量零差异+3片对等 真帧≥1",
		"跑满5秒，至少上屏1次",
	})

	statusText := "转色全对"
	sr, sg, sb := 0.3, 0.9, 0.5
	if colorErr != "" {
		statusText = "转色失败：" + shortErr(translateColorError(colorErr), 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	} else if gate.maxDiff != vectorExact {
		statusText = fmt.Sprintf("向量漂移：最大差%d(要0)", gate.maxDiff)
		sr, sg, sb = 0.95, 0.4, 0.35
	} else if !gate.parityOk {
		statusText = "对等失败：" + shortErr(gate.parityErr, 48)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR3 YUV转RGBA颜色", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range gate.infoLines() {
		if i >= 4 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Ours-vs-theory patches (solid vectors show first-pixel color).
	patchY := 180.0
	shell.Body.LabelAt("转出 vs 理论", 13, 20, patchY-26, 0.6, 0.8, 0.95)
	px := 20.0
	for _, key := range []string{"red patch", "green patch", "blue patch", "skin patch, limited", "grey mid, limited"} {
		vr := gate.patch(key)
		if vr == nil {
			continue
		}
		r1, g1, b1 := vr.ours[0], vr.ours[1], vr.ours[2]
		r2, g2, b2 := vr.want[0], vr.want[1], vr.want[2]
		o := rendering.NewRenderImage(64, 40)
		o.SetImageShared(solidImage(r1, g1, b1, 64, 40))
		shell.Body.Place(o, px, patchY)
		t := rendering.NewRenderImage(64, 40)
		t.SetImageShared(solidImage(r2, g2, b2, 64, 40))
		shell.Body.Place(t, px, patchY+46)
		px += 76
	}
	shell.Body.LabelAt("上=转出 下=理论", 11, 20, patchY+92, 0.65, 0.75, 0.85)

	// Grey ramp strip (ours over theory).
	if ramp := gate.patch("grey ramp"); ramp != nil {
		ry := patchY + 120.0
		shell.Body.LabelAt("灰阶条：上=转出 下=理论", 13, 20, ry-26, 0.6, 0.8, 0.95)
		ro := rendering.NewRenderImage(360, 22)
		ro.SetImageShared(stripImage(ramp.ours, ramp.w, ramp.h, 360, 22))
		shell.Body.Place(ro, 20, ry)
		rt := rendering.NewRenderImage(360, 22)
		rt.SetImageShared(stripImage(ramp.want, ramp.w, ramp.h, 360, 22))
		shell.Body.Place(rt, 20, ry+28)
	}

	// Real decoded I frame in color.
	fx := 560.0
	shell.Body.LabelAt("真解码I帧转色", 13, fx, patchY-26, 0.6, 0.8, 0.95)
	if gate.realFrame != nil {
		fr := rendering.NewRenderImage(240, 240)
		fr.SetImageShared(frameImage(gate.realFrame))
		shell.Body.Place(fr, fx, patchY)
		shell.Body.LabelAt(fmt.Sprintf("%dx%d %s", gate.realW, gate.realH, gate.profile), 11, fx, patchY+252, 0.65, 0.75, 0.85)
	} else {
		shell.Body.LabelAt("无图可摆，门禁必挂", 12, fx, patchY, 0.95, 0.4, 0.35)
	}
	shell.Body.LabelAt("R/B P99≤2 G P99≤3 对等ffmpeg，见README", 11, 20, shell.Body.H-56, 0.55, 0.65, 0.75)
	shell.Body.LabelAt("不支持的矩阵/采样报人话错，见单测", 11, 20, shell.Body.H-30, 0.55, 0.65, 0.75)

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_vr3_color: 关闭 (%s)\n", win.Backend())
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
		gatePreview := colorErr == "" && yuvReady == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VR3", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("向量=%d组 差%d 对等%s", len(gate.vectors), gate.maxDiff, gate.parityLine),
			fmt.Sprintf("真帧解码%.1f毫秒", gate.decodeMs))
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
	rep := buildReport(snap, app.PresentCount(), elapsed, gate, yuvReady, colorErr, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR3需要真窗口)")
		os.Exit(1)
	}
	if colorErr != "" {
		fmt.Fprintf(os.Stderr, "FAIL: 转色失败: %s\n", translateColorError(colorErr))
		os.Exit(1)
	}
	if gate.maxDiff != vectorExact {
		fmt.Fprintf(os.Stderr, "FAIL: 向量最大差=%d(要逐字节零差异)\n", gate.maxDiff)
		os.Exit(1)
	}
	if !gate.parityOk {
		fmt.Fprintf(os.Stderr, "FAIL: 对等失败: %s\n", gate.parityErr)
		os.Exit(1)
	}
	if yuvReady != 1 || gate.frames < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 真帧=%d 对照=%d(至少1帧)\n", gate.frames, yuvReady)
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
	fmt.Fprintf(os.Stderr, "video_vr3_color: 通过 向量=%d组 差%d 对等%s 上屏=%d 用时=%.1f秒\n",
		len(gate.vectors), gate.maxDiff, gate.parityLine, app.PresentCount(), elapsed)
}

// solidImage packs one RGB triple into a bridge image (window side only;
// the converter never touches render).
func solidImage(r, g, b uint8, w, h int) *render.ImageBuf {
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			_ = img.SetRGBA(xx, yy, r, g, b, 255)
		}
	}
	return img
}

// stripImage scales a small RGBA vector row-major into a display strip.
func stripImage(pix []uint8, sw, sh, w, h int) *render.ImageBuf {
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	if sw <= 0 || sh <= 0 || len(pix) < sw*sh*4 {
		return img
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			si := (yy*sh/h*sw + xx*sw/w) * 4
			_ = img.SetRGBA(xx, yy, pix[si], pix[si+1], pix[si+2], 255)
		}
	}
	return img
}

// frameImage bridges a converted frame to the display (window side only;
// the converter never touches render).
func frameImage(f *color.Frame) *render.ImageBuf {
	if f == nil || f.Width <= 0 || f.Height <= 0 || len(f.Pix) < f.Width*f.Height*4 {
		return nil
	}
	img, err := render.NewImageBuf(f.Width, f.Height, render.FormatRGBA8)
	if err != nil {
		return nil
	}
	for yy := 0; yy < f.Height; yy++ {
		for xx := 0; xx < f.Width; xx++ {
			o := (yy*f.Width + xx) * 4
			_ = img.SetRGBA(xx, yy, f.Pix[o], f.Pix[o+1], f.Pix[o+2], 255)
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

func translateColorError(s string) string {
	if s == "" {
		return ""
	}
	switch {
	case containsStr(s, "盒子打不开"):
		return "盒子打不开"
	case containsStr(s, "没视频轨"):
		return "盒子里没视频轨"
	case containsStr(s, "向量表"):
		return "向量表缺了，检查testdata"
	case containsStr(s, "unsupported colour matrix"):
		return "这个颜色矩阵本期不支持"
	case containsStr(s, "unsupported sampling"):
		return "这种采样格式本期不支持"
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR3 extras.
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

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, gate *colorGate, yuvReady int, colorErr string, hotTick int) report {
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
	decodeMs := 0.0
	if gate != nil {
		decodeMs = gate.decodeMs + gate.convertMs
	}
	return report{
		AbilityID: "VR3", Scenario: "video_vr3_color", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: decodeMs, DecodeMsP95: decodeMs,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: gate.frames, FramesShown: presents, ClockDriftMs: 0, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: gate.frames, YUVReady: yuvReady, ColorDiffPerChannel: float64(gate.maxDiff), PixelGoldenDiffPct: gate.diffPct,
		Clips: fmt.Sprintf("vectors=%d parity=%s", len(gate.vectors), gate.parityLine), Profile: gate.profile,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: colorErr, Source: "video/testdata/vr3_ffmpeg.json+video/color/testdata/vr3_vectors.json",
		NANote: "color-only: queue/clock/seek N/A in VR3; vectors exact + 3-clip rgba md5 parity (R/B P99<=2 G P99<=3) are the gates",
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
	for _, k := range []string{"frames_decoded", "yuv_ready", "color_diff_per_channel"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
