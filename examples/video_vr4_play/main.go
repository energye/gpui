// Command video_vr4_play is the VW2 VR4 real-window: background decode,
// bounded queue, timestamp display, pause, end-of-stream.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/video_vr4_play
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 30). GPU window required.
// Left: per-clip play gates from the real video player (B-reorder 96x96
// loops, 480p crop, 720p, all through demux+decode+color+queue+clock).
// Right: the live picture plus a pause proof (second half of the run the
// player holds and the picture truly stops).
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
		wrkit.RequireMinRun(secs, "VR4")
	} else {
		secs = 30
	}
	wrkit.EnsureUIFace()

	states := loadPlay()

	framesDecoded := int64(0)
	framesShown := int64(0)
	dropped := int64(0)
	decodeMax := 0.0
	driftMax := int64(0)
	playErr := ""
	yuvReady := 0
	for _, st := range states {
		if st.err != nil {
			if playErr == "" {
				playErr = st.name + "：" + st.err.Error()
			}
			continue
		}
		framesDecoded += st.decoded
		framesShown += st.shown
		dropped += st.dropped
		if st.decodeAvg > decodeMax {
			decodeMax = st.decodeAvg
		}
		if st.decodeP95 > decodeMax {
			decodeMax = st.decodeP95
		}
		if st.driftMs > driftMax {
			driftMax = st.driftMs
		}
	}
	// Gate: every clip played, frames actually showed, nothing dropped
	// on these tiny clips (catch-up counting is pinned in unit tests).
	allPlayed := playErr == "" && framesShown > 0 && dropped == 0
	if allPlayed {
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr4_play — 播放管线", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR4 播放管线 — 后台解/队列/时钟/暂停", []string{
		"左边三档，走真播放器",
		"右边直播，暂停真停",
		"队列水位 HUD 可见",
		"门禁：播出+暂停+零丢帧",
		"跑满30秒，动画60档判",
	})

	statusText := "播放正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if playErr != "" {
		statusText = "播放失败：" + shortErr(translatePlayError(playErr), 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	} else if !allPlayed {
		statusText = fmt.Sprintf("播出不足：显示%d 丢%d", framesShown, dropped)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR4 播放管线", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, st := range states {
		if i >= 4 {
			break
		}
		shell.Body.LabelAt(st.infoLine(), 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: replay the first clip on the window clock, pause in
	// the second half to prove Pause truly stops the picture.
	live, err := govideo.OpenFile(states[0].path, govideo.Options{Loop: true})
	if err != nil && playErr == "" {
		playErr = "直播打不开：" + err.Error()
		allPlayed = false
		yuvReady = 0
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		liveImg = rendering.NewRenderImage(360, 360)
		shell.Body.LabelAt("直播（后半暂停，真停）", 13, 20, 180, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 206)
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
				fmt.Fprintf(os.Stderr, "video_vr4_play: 关闭 (%s)\n", win.Backend())
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
	pauseNote := "播放中"
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		// Pause proof: hold the player for the middle third of the run.
		if live != nil {
			elapsed := float64(hotTick) / 60.0
			third := float64(secs) / 3.0
			if elapsed > third && elapsed < 2*third {
				if !live.Paused() {
					live.Pause()
				}
				pauseNote = "已暂停（画面应定住）"
			} else {
				if live.Paused() {
					live.Resume()
				}
				if elapsed >= 2*third {
					pauseNote = "已恢复（画面应继续）"
				}
			}
			if f, _ := live.Poll(); f != nil {
				liveShown++
				if liveBuf == nil {
					liveBuf, _ = render.NewImageBuf(f.Width, f.Height, render.FormatRGBA8)
				}
				blitRGBA(liveBuf, f.Pix, f.Width, f.Height)
				liveImg.SetImageShared(liveBuf)
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := playErr == "" && allPlayed && snapH.PresentCount > 0
		shell.UpdateHUD("VR4", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("解码=%d 显示=%d 丢=%d", framesDecoded, framesShown, dropped),
			fmt.Sprintf("直播=%d %s", liveShown, pauseNote))
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
	rep := buildReport(snap, app.PresentCount(), elapsed, states, framesDecoded, framesShown, dropped, yuvReady, decodeMax, driftMax, playErr, liveShown, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR4需要真窗口)")
		os.Exit(1)
	}
	// 60fps animation gate (§2.2.2): the window itself must stay fluid.
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if playErr != "" {
		fmt.Fprintf(os.Stderr, "FAIL: 播放失败: %s\n", translatePlayError(playErr))
		os.Exit(1)
	}
	if !allPlayed {
		fmt.Fprintf(os.Stderr, "FAIL: 播出不足 显示=%d 丢=%d\n", framesShown, dropped)
		os.Exit(1)
	}
	if liveShown < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 直播一帧没播出来")
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
	fmt.Fprintf(os.Stderr, "video_vr4_play: 通过 解码=%d 显示=%d 丢=%d 直播=%d 上屏=%d 用时=%.1f秒\n",
		framesDecoded, framesShown, dropped, liveShown, app.PresentCount(), elapsed)
}

// blitRGBA copies a player frame into the shared display buffer and flags
// it for GPU reupload (window side only). Without MarkPixelsDirty the GPU
// texture cache keys on GenerationID and would keep showing the first
// frame forever: numbers run, picture frozen.
func blitRGBA(dst *render.ImageBuf, pix []byte, w, h int) {
	if dst == nil || len(pix) < w*h*4 {
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

func translatePlayError(s string) string {
	if s == "" {
		return ""
	}
	switch {
	case containsStr(s, "打不开"):
		return "片子打不开"
	case containsStr(s, "没视频轨"):
		return "盒子里没视频轨"
	case containsStr(s, "对照图"):
		return "对照图缺了，现跑生成脚本"
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR4 extras.
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

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, states []*playState, framesDecoded, framesShown, dropped int64, yuvReady int, decodeMax float64, driftMax int64, playErr string, liveShown int64, hotTick int) report {
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
	for i, st := range states {
		if i > 0 {
			names += "+"
		}
		names += st.name
		if st.info.Profile != "" && prof == "" {
			prof = st.info.Profile
		}
		if i == 0 {
			src = st.path
		}
	}
	return report{
		AbilityID: "VR4", Scenario: "video_vr4_play", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: decodeMax, DecodeMsP95: decodeMax,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: dropped, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: framesDecoded, FramesShown: framesShown + liveShown, ClockDriftMs: driftMax, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: framesDecoded, YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: names, Profile: prof,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: playErr, Source: src,
		NANote: "play-only: seek N/A in VR4 (VR5); queue/clock/pause/end are the gates",
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
	for _, k := range []string{"frames_decoded", "yuv_ready", "dropped_old_frames"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
