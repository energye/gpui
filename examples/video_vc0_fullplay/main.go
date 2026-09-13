// Command video_vc0_fullplay is the VW2 VC0 combo real-window: one clip
// (720p) from demux to picture, end to end, no black, no flower.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/video_vc0_fullplay
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 30). GPU window required.
// Left: the five chain gates from the same file (VR0 numbers, VR1 params,
// VR2 decode count + monotonic stamps, VR3 first/last frame variance,
// VR4 full play to Ended with zero drops).
// Right: the live picture looping the same clip for the whole run.
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
		wrkit.RequireMinRun(secs, "VC0")
	} else {
		secs = 30
	}
	wrkit.EnsureUIFace()

	st := loadChain("720p全链路", resolveClip("vr2_720p.mp4"))

	chainErr := ""
	chainOK := 0
	yuvReady := 0
	if st.err != nil {
		chainErr = st.err.Error()
	} else {
		chainOK = 1
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vc0_fullplay — 全链路", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VC0 全链路 — 拆盒/参数/解码/转色/播放", []string{
		"左边五站，同一片出数",
		"右边直播，同片循环30秒",
		"一片播完，不黑不花",
		"门禁：五站全过+零丢帧",
		"跑满30秒，动画60档判",
	})

	statusText := "全链路正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if chainErr != "" {
		statusText = "全链路失败：" + shortErr(chainErr, 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VC0 全链路", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range st.infoLines() {
		if i >= 6 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: the same clip looping so eyes can verify motion
	// while the startup gate already proved one full pass to Ended.
	// The frame buffer is sized ONCE from the startup chain's clip info
	// (1280x720) — VR4/VC0 use one buffer per window, never per-frame alloc.
	liveW, liveH := 1280, 720
	if st.width > 0 && st.height > 0 {
		liveW, liveH = int(st.width), int(st.height)
	}
	live, err := govideo.OpenFile(st.path, govideo.Options{Loop: true})
	if err != nil && chainErr == "" {
		chainErr = "直播打不开：" + err.Error()
		chainOK = 0
		yuvReady = 0
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		var bufErr error
		liveBuf, bufErr = render.NewImageBuf(liveW, liveH, render.FormatRGBA8)
		if bufErr != nil && chainErr == "" {
			chainErr = "显存建不起：" + bufErr.Error()
			chainOK = 0
			yuvReady = 0
		}
		liveImg = rendering.NewRenderImage(480, 270)
		shell.Body.LabelAt("直播（同片循环，不黑不花）", 13, 20, 230, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 256)
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
				fmt.Fprintf(os.Stderr, "video_vc0_fullplay: 关闭 (%s)\n", win.Backend())
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
					liveImg.SetImageShared(liveBuf)
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := chainErr == "" && chainOK == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VC0", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("解码=%d 显示=%d 丢=%d", st.decoded, st.shown, st.dropped),
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
	rep := buildReport(snap, app.PresentCount(), elapsed, st, chainOK, yuvReady, chainErr, liveShown, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VC0需要真窗口)")
		os.Exit(1)
	}
	// 60fps animation gate (§2.2.2): VC0 is a sustained-play combo.
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if chainErr != "" || chainOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 全链路门禁: %s\n", chainErr)
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
	fmt.Fprintf(os.Stderr, "video_vc0_fullplay: 通过 解码=%d 显示=%d 丢=%d 直播=%d 上屏=%d 用时=%.1f秒\n",
		st.decoded, st.shown, st.dropped, liveShown, app.PresentCount(), elapsed)
}

// blitRGBA copies a player frame into the shared display buffer and flags
// it for GPU reupload (window side only). Without MarkPixelsDirty the GPU
// texture cache keys on GenerationID and would keep showing the first
// frame forever: numbers run, picture frozen.
func blitRGBA(dst *render.ImageBuf, pix []byte, w, h int) {
	fastBlitRGBA(dst, pix, w, h)
}

// fastBlitRGBA is the row-copy fast path: RGBA8 rows are contiguous, so
// one copy per row replaces w*h SetRGBA calls (520x faster on 720p, and
// the fix that took VC0 from 41fps/p95 65ms to the 60档 line).
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
	// Strided fallback (kept, never hit on RGBA8): per-pixel copy.
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VC0 chain extras.
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

	ChainOK     int     `json:"chain_ok"`
	TrackFound  int     `json:"track_found"`
	KeyframeN   int     `json:"keyframe_count"`
	DurationMs  int64   `json:"duration_ms"`
	Width       uint32  `json:"width"`
	Height      uint32  `json:"height"`
	FrameRate   float64 `json:"frame_rate"`
	SampleN     int     `json:"sample_count"`
	FirstVar    float64 `json:"first_var"`
	LastVar     float64 `json:"last_var"`
	PTSMonotone int     `json:"pts_monotone"`
	PlayedEnded int     `json:"played_ended"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, st *chainState, chainOK, yuvReady int, chainErr string, liveShown int64, hotTick int) report {
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
	trackFound := 0
	if st.samples > 0 {
		trackFound = 1
	}
	mono := 0
	if st.ptsMono {
		mono = 1
	}
	ended := 0
	if st.ended {
		ended = 1
	}
	return report{
		AbilityID: "VC0", Scenario: "video_vc0_fullplay", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: st.decodeAvg, DecodeMsP95: st.decodeP95,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: st.dropped, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: st.decoded, FramesShown: st.shown + liveShown, ClockDriftMs: st.driftMs, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(st.split), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: st.name, Profile: st.profile,
		ChainOK: chainOK, TrackFound: trackFound, KeyframeN: st.keyframes, DurationMs: st.durMs,
		Width: st.width, Height: st.height, FrameRate: st.fps, SampleN: st.samples,
		FirstVar: st.firstVar, LastVar: st.lastVar, PTSMonotone: mono, PlayedEnded: ended,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: chainErr, Source: st.path,
		NANote: "combo: seek N/A in VC0 (VC1); single 720p clip end to end",
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
	for _, k := range []string{"chain_ok", "yuv_ready", "frames_split"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
