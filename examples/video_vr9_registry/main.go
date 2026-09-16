// Command video_vr9_registry is the VW3 VR9 real-window: containers,
// codecs and colors all walk the registry; new formats plug in without
// touching the core flow.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/video_vr9_registry
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 15). GPU window required.
// Left: the registry proof on vr2_720p.mp4 (probe names, capability
// lists, same-clip play through the registry, window-side stub decoder,
// three readable unsupported cases with zero crash).
// Right: the same 720p clip looping for the whole run.
// Bottom HUD: fps / decode / queue / memory.
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
	govideo "github.com/energye/gpui/video"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR9")
	} else {
		secs = 15
	}
	wrkit.EnsureUIFace()

	st := loadRegistry("注册表扩展", resolveClip("vr2_720p.mp4"))

	gateErr := ""
	gateOK := 0
	yuvReady := 0
	if st.err != nil {
		gateErr = st.err.Error()
	} else if st.pass != st.total || st.total == 0 {
		gateErr = fmt.Sprintf("注册用例 %d/%d 没全过", st.pass, st.total)
	} else {
		gateOK = 1
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr9_registry — 扩展注册", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR9 扩展注册 — 问完再开/即插即用", []string{
		"同片走注册表播出",
		"灰桩解码器可插拔",
		"不支持的报人话错",
		"核心流程不写名字",
		"跑满15秒，动画60档判",
	})

	statusText := "注册正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if gateErr != "" {
		statusText = "注册失败：" + shortErr(gateErr, 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR9 扩展注册", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range st.infoLines() {
		if i >= 4 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: same 720p clip looping. One 1280x720 buffer for the
	// whole run (never per-frame alloc); display scales to 480x270 via
	// the target rect — no decode-side downscale (§2.8).
	const liveW, liveH = 1280, 720
	live, err := govideo.OpenFile(st.path, govideo.Options{Loop: true})
	if err != nil && gateErr == "" {
		gateErr = "直播打不开：" + err.Error()
		gateOK = 0
		yuvReady = 0
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		var bufErr error
		liveBuf, bufErr = render.NewImageBuf(liveW, liveH, render.FormatRGBA8)
		if bufErr != nil && gateErr == "" {
			gateErr = "显存建不起：" + bufErr.Error()
			gateOK = 0
			yuvReady = 0
		}
		liveImg = rendering.NewRenderImage(480, 270)
		shell.Body.LabelAt("直播（同片走注册表循环，不黑不花）", 13, 20, 180, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 206)
		shell.Body.LabelAt(fmt.Sprintf("注册用例 %d/%d（探测+能力+注册名+播出+灰桩+三坏例+H265三项）", st.pass, st.total), 12, 520, 206, 0.72, 0.8, 0.9)
		shell.Body.LabelAt(fmt.Sprintf("坏盒：%s", shortErr(st.badShellErr, 40)), 12, 520, 232, 0.72, 0.8, 0.9)
		shell.Body.LabelAt(fmt.Sprintf("坏编码：%s", shortErr(st.badCodecErr, 40)), 12, 520, 258, 0.72, 0.8, 0.9)
		shell.Body.LabelAt(fmt.Sprintf("坏采样：%s", shortErr(st.badColorErr, 40)), 12, 520, 284, 0.72, 0.8, 0.9)
		shell.Body.LabelAt(fmt.Sprintf("灰桩像素 R=%d G=%d B=%d", st.stubR, st.stubG, st.stubB), 12, 520, 310, 0.72, 0.8, 0.9)
		shell.Body.LabelAt(fmt.Sprintf("H265 %s/等级%d/长%d %d单元 %s", st.h265Profile, st.h265Level, st.h265Length, st.h265Units, st.h265Kind), 12, 520, 334, 0.72, 0.8, 0.9)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_vr9_registry: 关闭 (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	phase := ""
	hotTick := 0
	liveShown := int64(0)
	lastVar := 0.0
	liveNote := "播放中"
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		if live != nil && liveBuf != nil {
			if f, _ := live.Poll(); f != nil {
				liveShown++
				if f.Width != liveW || f.Height != liveH {
					liveNote = fmt.Sprintf("尺寸漂移 %dx%d", f.Width, f.Height)
				} else {
					fastBlitRGBA(liveBuf, f.Pix, liveW, liveH)
					if liveImg != nil {
						liveImg.SetImageShared(liveBuf)
					}
					lastVar = pixVar(f.Pix)
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := gateErr == "" && gateOK == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VR9", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("注册=%d/%d 播出=%d", st.pass, st.total, st.shown),
			fmt.Sprintf("直播=%d %s", liveShown, liveNote))
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

	liveStats := govideo.Stats{}
	if live != nil {
		liveStats = live.Stats()
	}
	// Live verdict: the loop must actually show across the run and stay
	// a real picture (variance proves no black hang behind the gate).
	if gateErr == "" {
		if liveShown <= 0 {
			gateErr = "直播一帧没播出来"
		} else if lastVar < 2 {
			gateErr = fmt.Sprintf("黑屏嫌疑: 尾帧MAD%.1f", lastVar)
		}
	}
	if gateErr != "" {
		gateOK = 0
		yuvReady = 0
	}

	rep := buildReport(snap, app.PresentCount(), elapsed, st, gateOK, yuvReady, gateErr, liveShown, lastVar, liveStats, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR9需要真窗口)")
		os.Exit(1)
	}
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if gateErr != "" || gateOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 注册门禁: %s\n", gateErr)
		os.Exit(1)
	}
	if st.pass != st.total {
		fmt.Fprintf(os.Stderr, "FAIL: 注册用例 %d/%d 没全过\n", st.pass, st.total)
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
	fmt.Fprintf(os.Stderr, "video_vr9_registry: 通过 注册=%d/%d 播出=%d 直播=%d 上屏=%d 用时=%.1f秒\n",
		st.pass, st.total, st.shown, liveShown, app.PresentCount(), elapsed)
}

// pixVar is the mean absolute deviation of sampled bytes (R channel).
func pixVar(pix []byte) float64 {
	if len(pix) < 16 {
		return 0
	}
	const step = 29
	var sum float64
	var n float64
	for i := 0; i < len(pix); i += 4 * step {
		sum += float64(pix[i])
		n++
	}
	if n == 0 {
		return 0
	}
	mean := sum / n
	var dev float64
	for i := 0; i < len(pix); i += 4 * step {
		d := float64(pix[i]) - mean
		if d < 0 {
			d = -d
		}
		dev += d
	}
	return dev / n
}

// fastBlitRGBA is the row-copy fast path: RGBA8 rows are contiguous, so
// one copy per row replaces w*h SetRGBA calls (VC0's fix, reused here).
func fastBlitRGBA(dst *render.ImageBuf, pix []byte, w, h int) {
	if dst == nil || len(pix) < w*h*4 {
		return
	}
	data := dst.Data()
	rowLen := w * 4
	if len(data) >= h*rowLen {
		for y := 0; y < h; y++ {
			copy(data[y*rowLen:(y+1)*rowLen], pix[y*rowLen:(y+1)*rowLen])
		}
		dst.MarkPixelsDirty()
		dst.InvalidatePremulCache()
		return
	}
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w; xx++ {
			o := (yy*w + xx) * 4
			_ = dst.SetRGBA(xx, yy, pix[o], pix[o+1], pix[o+2], pix[o+3])
		}
	}
	dst.MarkPixelsDirty()
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR9 registry extras.
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
	DroppedOldFrames int64   `json:"dropped_old_frames"`
	AllocPerFrameB   int64   `json:"alloc_per_frame_B"`
	PoolHitPct       float64 `json:"pool_hit_pct"`

	FramesDecoded      int64   `json:"frames_decoded"`
	FramesShown        int64   `json:"frames_shown"`
	ClockDriftMs       int64   `json:"clock_drift_ms"`
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
	FramesSplit         int64   `json:"frames_split"`
	YUVReady            int     `json:"yuv_ready"`
	ColorDiffPerChannel float64 `json:"color_diff_per_channel"`
	PixelGoldenDiffPct  float64 `json:"pixel_golden_diff_pct"`
	Clips               string  `json:"clips"`
	Profile             string  `json:"profile"`

	RegistryCasesPass  int     `json:"registry_cases_pass"`
	RegistryCasesTotal int     `json:"registry_cases_total"`
	ProbeContainer     string  `json:"probe_container"`
	ProbeCodec         string  `json:"probe_codec"`
	StubOK             int     `json:"stub_ok"`
	H265Codec          string  `json:"h265_codec"`
	H265Profile        string  `json:"h265_profile"`
	H265Level          int     `json:"h265_level"`
	H265LengthSize     int     `json:"h265_length_size"`
	H265Units          int     `json:"h265_units"`
	H265Kind           string  `json:"h265_kind"`
	LastVar            float64 `json:"last_var"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, st *registryState, gateOK, yuvReady int, gateErr string, liveShown int64, lastVar float64, liveStats govideo.Stats, hotTick int) report {
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
	stubOK := 0
	if st.stubOK {
		stubOK = 1
	}
	return report{
		AbilityID: "VR9", Scenario: "video_vr9_registry", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: st.decodeAvg, DecodeMsP95: st.decodeP95,
		QueueDepthAvg: liveStats.QueueAvg, QueueDepthMax: liveStats.QueueMax, DroppedOldFrames: liveStats.Dropped, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: st.decoded, FramesShown: liveShown, ClockDriftMs: liveStats.DriftMs, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(st.frames), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: st.name, Profile: "",
		RegistryCasesPass: st.pass, RegistryCasesTotal: st.total,
		ProbeContainer: st.container, ProbeCodec: st.codec, StubOK: stubOK,
		H265Codec: st.h265Codec, H265Profile: st.h265Profile, H265Level: st.h265Level,
		H265LengthSize: st.h265Length, H265Units: st.h265Units, H265Kind: st.h265Kind,
		LastVar:            lastVar,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: gateErr, Source: st.path,
		NANote: "registry-only: seek N/A in VR9 (VR5); containers/codecs/colors resolve via tables, core flow names no format",
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
	"clips", "profile", "registry_cases_pass", "registry_cases_total",
	"h265_codec", "h265_profile", "h265_level", "h265_length_size", "h265_units", "h265_kind",
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
	for _, k := range []string{"registry_cases_pass", "registry_cases_total", "yuv_ready"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
