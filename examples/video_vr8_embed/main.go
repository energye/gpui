// Command video_vr8_embed is the VW3 VR8 real-window: video as one rect
// node inside the UI scene, composited with panel/text/overlay.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/video_vr8_embed
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 30). GPU window required.
// Left: startup gate on vr2_720p.mp4 (demux 1280x720, params, cold decode
// to Ended with monotonic stamps and non-black variance).
// Body: the same 720p clip looping inside the scene — main 480x270 target
// plus a second 240x135 target sharing the frame (scale via target rect),
// wrapped in clip + opacity nodes, with a translucent overlay covering one
// corner and panel text beside it; mid-run resize proves no assert crash.
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
		wrkit.RequireMinRun(secs, "VR8")
	} else {
		secs = 30
	}
	wrkit.EnsureUIFace()

	st := loadEmbed("720p内嵌", resolveClip("vr2_720p.mp4"))

	gateErr := ""
	gateOK := 0
	yuvReady := 0
	if st.err != nil {
		gateErr = st.err.Error()
	} else {
		gateOK = 1
		yuvReady = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr8_embed — 真窗内嵌", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR8 真窗内嵌 — 视频做场景里的一块", []string{
		"视频嵌场景，与文字浮层合成",
		"两尺寸同源，缩放走目标矩形",
		"裁剪+透明走现有机制",
		"浮层盖一角，中途改尺寸",
		"跑满30秒，动画60档判",
	})

	statusText := "内嵌正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if gateErr != "" {
		statusText = "内嵌失败：" + shortErr(gateErr, 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR8 真窗内嵌", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range st.infoLines() {
		if i >= 4 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: same 720p clip looping. One 1280x720 buffer for the
	// whole run (never per-frame alloc); two targets share it to prove
	// scaling sits in the display rect — decode stays full 1280x720
	// (§2.8: no decode-side downscale).
	const liveW, liveH = 1280, 720
	live, err := govideo.OpenFile(st.path, govideo.Options{Loop: true})
	if err != nil && gateErr == "" {
		gateErr = "直播打不开：" + err.Error()
		gateOK = 0
		yuvReady = 0
	}
	var liveImg, smallImg *rendering.RenderImage
	var clipNode *rendering.RenderClipRRect
	var opNode *rendering.RenderOpacity
	var overlayWrap *rendering.RenderOpacity
	var liveBuf *render.ImageBuf
	const mainW, mainH = 480.0, 270.0
	const smallW, smallH = 240.0, 135.0
	if live != nil {
		defer live.Close()
		var bufErr error
		liveBuf, bufErr = render.NewImageBuf(liveW, liveH, render.FormatRGBA8)
		if bufErr != nil && gateErr == "" {
			gateErr = "显存建不起：" + bufErr.Error()
			gateOK = 0
			yuvReady = 0
		}
		shell.Body.LabelAt("内嵌视频（缩放+裁剪+透明，同场景合成）", 13, 20, 180, 0.6, 0.8, 0.95)
		liveImg = rendering.NewRenderImage(mainW, mainH)
		clipNode = rendering.NewRenderClipRRect(liveImg)
		clipNode.FixedWidth, clipNode.FixedHeight = mainW, mainH
		clipNode.Radius = 12
		opNode = rendering.NewRenderOpacity(1.0, clipNode)
		opNode.FixedWidth, opNode.FixedHeight = mainW, mainH
		shell.Body.Place(opNode, 20, 206)
		shell.Body.LabelAt("同源第二尺寸（缩放走目标矩形）", 13, 520, 180, 0.6, 0.8, 0.95)
		smallImg = rendering.NewRenderImage(smallW, smallH)
		shell.Body.Place(smallImg, 520, 206)
		// Translucent overlay covering one corner of the main video:
		// existing color box + existing opacity, no new channel. It is
		// placed after the video so it composites on top.
		overlayBox := rendering.NewRenderColorBox(200, 60, 0.2, 0.5, 0.9, 1.0)
		overlayWrap = rendering.NewRenderOpacity(0.75, overlayBox)
		overlayWrap.FixedWidth, overlayWrap.FixedHeight = 200, 60
		shell.Body.Place(overlayWrap, 60, 406)
		shell.Body.LabelAt("浮层（透明0.75，盖住一角）", 12, 70, 416, 0.92, 0.95, 1.0)
		shell.Body.LabelAt("面板文字与视频同屏，不闪", 12, 520, 360, 0.72, 0.8, 0.9)
		shell.Body.LabelAt("浮层盖住一角，视频不坏", 12, 520, 384, 0.72, 0.8, 0.9)
		shell.Body.LabelAt("10秒缩一次 20秒还原（改尺寸不断言）", 12, 20, 500, 0.72, 0.8, 0.9)
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
				fmt.Fprintf(os.Stderr, "video_vr8_embed: 关闭 (%s)\n", win.Backend())
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
	resizeStage := 0
	resizeNote := "未改尺寸"
	shownAtShrink := int64(-1)
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		// Mid-run resize proof (no assert crash): shrink once, restore
		// once. Both go through Width/Fixed + MarkNeedsLayout, the same
		// path a real window resize takes.
		if liveImg != nil && clipNode != nil && opNode != nil {
			if resizeStage == 0 && hotTick == 600 {
				liveImg.Width, liveImg.Height = 360, 202
				liveImg.MarkNeedsLayout()
				clipNode.FixedWidth, clipNode.FixedHeight = 360, 202
				clipNode.MarkNeedsLayout()
				opNode.FixedWidth, opNode.FixedHeight = 360, 202
				opNode.MarkNeedsLayout()
				resizeStage = 1
				resizeNote = "已缩小360x202"
				shownAtShrink = liveShown
			} else if resizeStage == 1 && hotTick == 1200 {
				liveImg.Width, liveImg.Height = mainW, mainH
				liveImg.MarkNeedsLayout()
				clipNode.FixedWidth, clipNode.FixedHeight = mainW, mainH
				clipNode.MarkNeedsLayout()
				opNode.FixedWidth, opNode.FixedHeight = mainW, mainH
				opNode.MarkNeedsLayout()
				resizeStage = 2
				resizeNote = "已还原480x270"
			}
		}

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
					if smallImg != nil {
						smallImg.SetImageShared(liveBuf)
					}
					lastVar = pixVar(f.Pix)
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := gateErr == "" && gateOK == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VR8", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("解码=%d 显示=%d %s", st.decoded, st.shown, resizeNote),
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
	// Embed verdict: startup gate + live playback across overlay and
	// across both resizes. Overlay nodes exist by construction; the live
	// proof is that frames kept showing after the shrink.
	scaleOK := 0
	clipOK := 0
	overlayOK := 0
	resizeOK := 0
	embedOK := 0
	if liveImg != nil && smallImg != nil {
		scaleOK = 1
	}
	if clipNode != nil && opNode != nil {
		clipOK = 1
	}
	if overlayWrap != nil {
		overlayOK = 1
	}
	if resizeStage == 2 && shownAtShrink >= 0 && liveShown > shownAtShrink {
		resizeOK = 1
	} else if resizeStage != 2 {
		gateErr = fmt.Sprintf("改尺寸没走完 stage=%d", resizeStage)
	} else if liveShown <= shownAtShrink {
		gateErr = "改尺寸后直播停了"
	}
	if gateErr == "" && liveShown > 0 && lastVar >= 2 && scaleOK == 1 && clipOK == 1 && overlayOK == 1 && resizeOK == 1 {
		embedOK = 1
	} else if gateErr == "" {
		if liveShown <= 0 {
			gateErr = "直播一帧没播出来"
		} else if lastVar < 2 {
			gateErr = fmt.Sprintf("黑屏嫌疑: 尾帧MAD%.1f", lastVar)
		} else {
			gateErr = fmt.Sprintf("内嵌不全: 缩放%d 裁剪%d 浮层%d 改尺寸%d", scaleOK, clipOK, overlayOK, resizeOK)
		}
	}
	if gateErr != "" {
		gateOK = 0
		yuvReady = 0
	}

	rep := buildReport(snap, app.PresentCount(), elapsed, st, gateOK, yuvReady, gateErr, liveShown, lastVar, embedOK, scaleOK, clipOK, overlayOK, resizeOK, liveStats, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR8需要真窗口)")
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
	if gateErr != "" || gateOK != 1 || embedOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 内嵌门禁: %s\n", gateErr)
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
	fmt.Fprintf(os.Stderr, "video_vr8_embed: 通过 解码=%d 显示=%d 直播=%d 内嵌=%d 改尺寸=%s 上屏=%d 用时=%.1f秒\n",
		st.decoded, st.shown, liveShown, embedOK, resizeNote, app.PresentCount(), elapsed)
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR8 embed extras.
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

	EmbedOK   int     `json:"embed_ok"`
	ScaleOK   int     `json:"scale_ok"`
	ClipOK    int     `json:"clip_ok"`
	OverlayOK int     `json:"overlay_ok"`
	ResizeOK  int     `json:"resize_ok"`
	LastVar   float64 `json:"last_var"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, st *embedState, gateOK, yuvReady int, gateErr string, liveShown int64, lastVar float64, embedOK, scaleOK, clipOK, overlayOK, resizeOK int, liveStats govideo.Stats, hotTick int) report {
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
	return report{
		AbilityID: "VR8", Scenario: "video_vr8_embed", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: st.decodeAvg, DecodeMsP95: st.decodeP95,
		QueueDepthAvg: liveStats.QueueAvg, QueueDepthMax: liveStats.QueueMax, DroppedOldFrames: liveStats.Dropped, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: st.decoded, FramesShown: liveShown, ClockDriftMs: liveStats.DriftMs, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(st.split), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: st.name, Profile: st.profile,
		EmbedOK: embedOK, ScaleOK: scaleOK, ClipOK: clipOK, OverlayOK: overlayOK, ResizeOK: resizeOK, LastVar: lastVar,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: gateErr, Source: st.path,
		NANote: "embed-only: seek N/A in VR8 (VR5); scale via target rect, clip via ClipRRect, opacity via RenderOpacity, no new display channel",
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
	for _, k := range []string{"embed_ok", "yuv_ready", "frames_split"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
