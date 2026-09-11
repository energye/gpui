// Command video_vr0_demux is the VW0 VR0 real-window: MP4 demux.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/video_vr0_demux
//
// Window: 1200x800. RUN_SECONDS>=5. GPU window required.
// Left: file info panel from a real video/mp4 parse.
// Right: keyframe position bar by PTS. Bottom: bad-box readable error.
package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/video/mp4"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR0")
	} else {
		secs = 5
	}
	wrkit.EnsureUIFace()

	srcPath := os.Getenv("VIDEO_MP4_PATH")
	goodBytes, srcDesc := goodMP4Bytes(srcPath)
	t0parse := time.Now()
	movie, parseErr := mp4.Parse(goodBytes)
	decodeMs := float64(time.Since(t0parse).Microseconds()) / 1000.0

	badBytes := badMP4Bytes(goodBytes)
	badErrStr := ""
	badOK := false
	func() {
		defer func() {
			if recover() != nil {
				badErrStr = "panic on bad input"
				badOK = false
			}
		}()
		if _, err := mp4.Parse(badBytes); err != nil {
			badErrStr = err.Error()
			badOK = true
		} else {
			badErrStr = "bad input parsed without error"
			badOK = false
		}
	}()

	trackFound := 0
	keyframeCount := 0
	var durationMs int64
	var width, height uint32
	var frameRate float64
	var sampleCount int
	var codec, brand string
	var timescale uint32
	var rotation int
	var avcCLen int
	var paspH, paspV uint32
	var hasEditList bool
	var keyframes []mp4.Keyframe
	if parseErr == nil && movie != nil && movie.Video != nil {
		v := movie.Video
		trackFound = 1
		keyframeCount = len(v.Keyframes)
		durationMs = v.DurationMs
		width = v.Width
		height = v.Height
		frameRate = v.FrameRate
		sampleCount = v.SampleCount
		codec = v.Codec
		brand = movie.MajorBrand
		timescale = v.Timescale
		rotation = v.Rotation
		avcCLen = len(v.AVCConfig)
		paspH = v.PixelAspectH
		paspV = v.PixelAspectV
		hasEditList = v.HasEditList
		keyframes = append([]mp4.Keyframe(nil), v.Keyframes...)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr0_demux — MP4拆盒", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR0 MP4拆盒 — 视频轨/尺寸/帧率/时长/关键帧", []string{
		"左边是文件信息，用真解析结果",
		"右边是关键帧位置条，按显示时间排",
		"坏盒子必须报可读错，不能崩",
		"门禁：视频轨=1、关键帧>=1、时长>0",
		"跑满5秒，至少上屏1次，输出同步源",
	})

	statusText := "拆盒成功"
	sr, sg, sb := 0.3, 0.9, 0.5
	if parseErr != nil {
		statusText = "拆盒失败：" + shortErr(translateDemuxError(parseErr.Error()), 72)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR0 MP4拆盒", 22, 20, 12, 0.92, 0.94, 0.98)
	statusLabel := shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	infoLines := buildInfoLines(srcDesc, brand, codec, width, height, rotation, timescale, durationMs, frameRate, sampleCount, keyframeCount, avcCLen, paspH, paspV, hasEditList, decodeMs, len(goodBytes), parseErr)
	for i, ln := range infoLines {
		if i >= 14 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	barX, barY, barW := 640.0, 110.0, 300.0
	shell.Body.LabelAt("关键帧(按显示时间排)", 13, barX, 76, 0.6, 0.8, 0.95)
	shell.Body.ColorAt(barW, 26, barX, barY, 0.16, 0.18, 0.22, 1, false)
	shownKeys := keyframes
	if len(shownKeys) > 12 {
		shownKeys = shownKeys[:12]
	}
	denom := durationMs
	if denom <= 0 {
		denom = 1
	}
	for i, k := range shownKeys {
		x := barX + float64(k.PTSMs)/float64(denom)*barW
		if x < barX {
			x = barX
		}
		if x > barX+barW-6 {
			x = barX + barW - 6
		}
		shell.Body.ColorAt(6, 26, x, barY, 0.3, 0.85, 0.5, 1, false)
		if i < 4 {
			shell.Body.LabelAt(fmt.Sprintf("第%d个 采样%d %d毫秒", i+1, k.SampleNumber, k.PTSMs), 11, barX, barY+34+float64(i)*20, 0.65, 0.75, 0.85)
		}
	}
	shell.Body.LabelAt(fmt.Sprintf("0毫秒 ... %d毫秒 (共%d个)", durationMs, keyframeCount), 11, barX, barY+34+4*20, 0.55, 0.65, 0.75)
	if parseErr == nil && keyframeCount == 0 {
		shell.Body.LabelAt("没有关键帧(缺同步表表示全是关键帧，否则判失败)", 11, barX, barY+130, 0.95, 0.5, 0.4)
	}

	badColorR, badColorG, badColorB := 0.45, 0.85, 0.55
	badPrefix := "坏盒子正常报错："
	if !badOK {
		badColorR, badColorG, badColorB = 0.95, 0.4, 0.35
		badPrefix = "坏盒子没报对："
	}
	shownBad := translateDemuxError(badErrStr)
	shell.Body.LabelAt(badPrefix+shortErr(shownBad, 96), 12, 20, shell.Body.H-56, badColorR, badColorG, badColorB)
	shell.Body.LabelAt("来源："+shortErr(translateSource(srcDesc), 96), 11, 20, shell.Body.H-30, 0.55, 0.65, 0.75)

	hot := rendering.NewRenderColorBox(90, 90, 0.3, 0.7, 0.95, 1)
	hot.SetRepaintBoundary(true)
	hotX, hotY := 640.0, 320.0
	shell.Body.Place(hot, hotX, hotY)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_vr0_demux: 关闭 (%s)\n", win.Backend())
			case platform.EventResize:
				if ev.Width > 0 && ev.Height > 0 {
					shell.Resize(float64(ev.Width), float64(ev.Height))
				}
			}
		},
	})

	phase := ""
	hotTick := 0
	clock := wrkit.NewPhaseClock(2, 4)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		switch phase {
		case wrkit.PhaseSteady:
			hot.R, hot.G, hot.B = 0.3, 0.7, 0.95
			hotX = 640
		case wrkit.PhaseSpike:
			hot.R, hot.G, hot.B = 1.0, 0.85, 0.2
			hotX = 720
		default:
			hot.R, hot.G, hot.B = 0.4, 0.9, 0.5
			hotX = 680
		}
		shell.Body.Box.Place(hot, hotX, hotY)
		hot.MarkNeedsPaint()
		app.ScheduleFrame()
		proc.Sample()

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := parseErr == nil && trackFound == 1 && keyframeCount >= 1 && durationMs > 0 && badOK
		shell.UpdateHUD("VR0", phaseCN(phase), app, gatePreview && snapH.PresentCount > 0,
			fmt.Sprintf("视频轨=%d 关键帧=%d 时长=%d毫秒", trackFound, keyframeCount, durationMs),
			fmt.Sprintf("拆盒=%.2f毫秒 %s", decodeMs, shortErr(translateSource(srcDesc), 40)))
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
	rep := buildReport(snap, app.PresentCount(), elapsed, decodeMs, trackFound, keyframeCount, durationMs, width, height, frameRate, sampleCount, codec, badOK, badErrStr, parseErr, srcDesc, hotTick, statusLabel != nil)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR0需要真窗口)")
		os.Exit(1)
	}
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "FAIL: 好文件拆盒失败: %v\n", translateDemuxError(parseErr.Error()))
		os.Exit(1)
	}
	if trackFound != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 视频轨=%d 想要1\n", trackFound)
		os.Exit(1)
	}
	if keyframeCount < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 关键帧数=%d 想要>=1\n", keyframeCount)
		os.Exit(1)
	}
	if durationMs <= 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 时长=%d毫秒 想要>0\n", durationMs)
		os.Exit(1)
	}
	if !badOK {
		fmt.Fprintf(os.Stderr, "FAIL: 坏文件没报对: %q\n", translateDemuxError(badErrStr))
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
	fmt.Fprintf(os.Stderr, "video_vr0_demux: 通过 视频轨=%d 关键帧=%d 时长=%d毫秒 上屏=%d 用时=%.1f秒 拆盒=%.2f毫秒\n",
		trackFound, keyframeCount, durationMs, app.PresentCount(), elapsed, decodeMs)
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

func shortErr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func buildInfoLines(src, brand, codec string, w, h uint32, rot int, ts uint32, durMs int64, fps float64, samples, keys int, avcC int, paspH, paspV uint32, hasElst bool, decodeMs float64, fileBytes int, parseErr error) []string {
	elst := "无"
	if hasElst {
		elst = "有"
	}
	errLine := "解析=成功"
	if parseErr != nil {
		errLine = "解析=" + shortErr(translateDemuxError(parseErr.Error()), 48)
	}
	return []string{
		"来源：" + shortErr(translateSource(src), 64),
		fmt.Sprintf("品牌=%s 编码=%s", brand, codec),
		fmt.Sprintf("画面=%dx%d 旋转=%d度 编码尺寸=%dx%d", w, h, rot, w, h),
		fmt.Sprintf("时间刻度=%d 时长=%d毫秒 帧率=%.2f", ts, durMs, fps),
		fmt.Sprintf("采样数=%d 关键帧数=%d", samples, keys),
		fmt.Sprintf("参数集=%d字节 像素比=%d/%d 编辑表=%s", avcC, paspH, paspV, elst),
		fmt.Sprintf("拆盒耗时=%.2f毫秒 文件=%d字节", decodeMs, fileBytes),
		errLine,
	}
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

func translateDemuxError(s string) string {
	if s == "" {
		return ""
	}
	switch {
	case containsAny(s, []string{"truncated", "extends past end", "too small", "short "}):
		return "文件被截断，盒子长度超过文件尾"
	case containsAny(s, []string{"missing moov", "no ftyp/moov"}):
		return "找不到电影盒，文件头缺失"
	case containsAny(s, []string{"no video track", "no tracks", "without codec"}):
		return "找不到视频轨"
	case containsAny(s, []string{"fragmented", "moof"}):
		return "碎片文件，本阶段不支持"
	case containsAny(s, []string{"bad box", "bad size", "small size", "overruns"}):
		return "盒子损坏，长度不对"
	case containsAny(s, []string{"inconsistent", "no sample table", "missing stco", "missing stsc", "zero samples"}):
		return "采样表对不上，文件索引坏了"
	case containsAny(s, []string{"unsupported", "stz2"}):
		return "用了本阶段不支持的写法"
	case containsAny(s, []string{"panic"}):
		return "解析时崩了"
	case containsAny(s, []string{"bad input parsed without error"}):
		return "坏文件居然通过了，没报对"
	default:
		return s
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func translateSource(src string) string {
	if src == "" {
		return ""
	}
	if strings.HasPrefix(src, "file:") {
		return "文件：" + strings.TrimPrefix(src, "file:")
	}
	if strings.HasPrefix(src, "synthetic:") {
		rest := strings.TrimPrefix(src, "synthetic:")
		out := rest
		out = strings.ReplaceAll(out, "samples", "个采样")
		out = strings.ReplaceAll(out, "fps", "帧每秒")
		out = strings.ReplaceAll(out, "keyframes", "个关键帧")
		return "合成片：" + out
	}
	return src
}

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR0 extras.
// Required fields use plain tags so zero values still marshal.
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
	TrackFound          int     `json:"track_found"`
	KeyframeCount       int     `json:"keyframe_count"`
	DurationMs          int64   `json:"duration_ms"`
	Width               uint32  `json:"width"`
	Height              uint32  `json:"height"`
	FrameRate           float64 `json:"frame_rate"`
	SampleCount         int     `json:"sample_count"`
	Codec               string  `json:"codec"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	BadFileOK    bool   `json:"bad_file_ok"`
	BadFileError string `json:"bad_file_error"`
	DemuxError   string `json:"demux_error"`
	Source       string `json:"source"`
	NANote       string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed, decodeMs float64, trackFound, keyCount int, durMs int64, w, h uint32, fps float64, samples int, codec string, badOK bool, badErr string, demuxErr error, src string, hotTick int, _ bool) report {
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
	demuxErrStr := ""
	if demuxErr != nil {
		demuxErrStr = demuxErr.Error()
	}
	_ = hotTick
	return report{
		AbilityID: "VR0", Scenario: "video_vr0_demux", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: decodeMs, DecodeMsP95: decodeMs,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: samples, FramesShown: presents, ClockDriftMs: 0, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 524288, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 0, PPSOk: 0, FramesSplit: 0, YUVReady: 0, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		TrackFound: trackFound, KeyframeCount: keyCount, DurationMs: durMs, Width: w, Height: h, FrameRate: fps, SampleCount: samples, Codec: codec,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		BadFileOK: badOK, BadFileError: badErr, DemuxError: demuxErrStr, Source: src,
		NANote: "demux-only: queue/clock/seek/sps/pps/yuv/color/golden N/A in VR0; queue=0 pool=0 seek_ok=0 sps/pps/yuv=0",
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
	"track_found", "keyframe_count", "duration_ms", "width", "height", "frame_rate", "sample_count", "codec",
	"time_to_first_frame_ms", "present_count", "paint_count", "bad_file_ok", "bad_file_error",
}

func checkSchema(raw []byte) error {
	s := string(raw)
	var missing []string
	for _, k := range requiredKeys {
		if !bytes.Contains(raw, []byte(`"`+k+`"`)) {
			missing = append(missing, k)
		}
	}
	_ = s
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
	for _, k := range []string{"track_found", "keyframe_count", "duration_ms", "codec"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}

// Synthetic MP4 builders (same box rules as video/mp4 tests).

func mkBox(typ string, payload []byte) []byte {
	size := uint32(8 + len(payload))
	buf := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(buf[0:], size)
	copy(buf[4:], typ)
	copy(buf[8:], payload)
	return buf
}

func mkFullBox(typ string, version byte, flags uint32, body []byte) []byte {
	payload := make([]byte, 4+len(body))
	payload[0] = version
	payload[1] = byte(flags >> 16)
	payload[2] = byte(flags >> 8)
	payload[3] = byte(flags)
	copy(payload[4:], body)
	return mkBox(typ, payload)
}

func goodMP4Bytes(path string) ([]byte, string) {
	if path != "" {
		if b, err := os.ReadFile(path); err == nil && len(b) >= 8 {
			return b, "文件：" + path
		}
	}
	const samples = 120
	const timescale = uint32(90000)
	const delta = uint32(3000)
	sizes := make([]uint32, samples)
	for i := range sizes {
		sizes[i] = uint32(120 + (i*37)%180)
	}
	const chunks = 12
	perChunk := samples / chunks
	chunkOffs := make([]uint32, chunks)
	base := uint32(4096)
	off := base
	idx := 0
	for c := 0; c < chunks; c++ {
		chunkOffs[c] = off
		for k := 0; k < perChunk; k++ {
			off += sizes[idx]
			idx++
		}
	}
	var keys []uint32
	for s := 1; s <= samples; s += 30 {
		keys = append(keys, uint32(s))
	}
	ftypPayload := make([]byte, 16)
	copy(ftypPayload[0:], "isom")
	copy(ftypPayload[8:], "isom")
	copy(ftypPayload[12:], "mp41")
	ftyp := mkBox("ftyp", ftypPayload)

	mvhdBody := make([]byte, 100)
	binary.BigEndian.PutUint32(mvhdBody[8:], 1000)
	binary.BigEndian.PutUint32(mvhdBody[12:], 4000)
	mvhd := mkFullBox("mvhd", 0, 0, mvhdBody)

	tkhdBody := make([]byte, 80)
	binary.BigEndian.PutUint32(tkhdBody[8:], 1)
	binary.BigEndian.PutUint32(tkhdBody[16:], 4000)
	binary.BigEndian.PutUint32(tkhdBody[36:], 0x00010000)
	binary.BigEndian.PutUint32(tkhdBody[52:], 0x00010000)
	binary.BigEndian.PutUint32(tkhdBody[68:], 0x40000000)
	binary.BigEndian.PutUint32(tkhdBody[72:], 1280<<16)
	binary.BigEndian.PutUint32(tkhdBody[76:], 720<<16)
	tkhd := mkFullBox("tkhd", 0, 7, tkhdBody)

	mdhdBody := make([]byte, 20)
	binary.BigEndian.PutUint32(mdhdBody[8:], timescale)
	binary.BigEndian.PutUint32(mdhdBody[12:], uint32(samples)*delta)
	mdhd := mkFullBox("mdhd", 0, 0, mdhdBody)

	hdlrBody := make([]byte, 25)
	copy(hdlrBody[4:], "vide")
	copy(hdlrBody[20:], "test\x00")
	hdlr := mkFullBox("hdlr", 0, 0, hdlrBody)

	avc1Entry := make([]byte, 86)
	copy(avc1Entry[4:], "avc1")
	binary.BigEndian.PutUint16(avc1Entry[14:], 1)
	binary.BigEndian.PutUint16(avc1Entry[32:], 1280)
	binary.BigEndian.PutUint16(avc1Entry[34:], 720)
	binary.BigEndian.PutUint32(avc1Entry[36:], 0x00480000)
	binary.BigEndian.PutUint32(avc1Entry[40:], 0x00480000)
	binary.BigEndian.PutUint16(avc1Entry[48:], 1)
	binary.BigEndian.PutUint16(avc1Entry[82:], 0x0018)
	binary.BigEndian.PutUint16(avc1Entry[84:], 0xFFFF)
	avcc := mkBox("avcC", []byte{0x01, 0x64, 0x00, 0x1E, 0xFF, 0xE1, 0x00, 0x0A, 0x01, 0x02, 0x03})
	paspPayload := make([]byte, 8)
	binary.BigEndian.PutUint32(paspPayload[0:], 1)
	binary.BigEndian.PutUint32(paspPayload[4:], 1)
	pasp := mkBox("pasp", paspPayload)
	avc1Entry = append(avc1Entry, avcc...)
	avc1Entry = append(avc1Entry, pasp...)
	binary.BigEndian.PutUint32(avc1Entry[0:], uint32(len(avc1Entry)))
	stsdBody := make([]byte, 4)
	binary.BigEndian.PutUint32(stsdBody[0:], 1)
	stsdBody = append(stsdBody, avc1Entry...)
	stsd := mkFullBox("stsd", 0, 0, stsdBody)

	sttsBody := make([]byte, 12)
	binary.BigEndian.PutUint32(sttsBody[0:], 1)
	binary.BigEndian.PutUint32(sttsBody[4:], uint32(samples))
	binary.BigEndian.PutUint32(sttsBody[8:], delta)
	stts := mkFullBox("stts", 0, 0, sttsBody)

	stscBody := make([]byte, 16)
	binary.BigEndian.PutUint32(stscBody[0:], 1)
	binary.BigEndian.PutUint32(stscBody[4:], 1)
	binary.BigEndian.PutUint32(stscBody[8:], uint32(perChunk))
	binary.BigEndian.PutUint32(stscBody[12:], 1)
	stsc := mkFullBox("stsc", 0, 0, stscBody)

	stszBody := make([]byte, 8+len(sizes)*4)
	binary.BigEndian.PutUint32(stszBody[4:], uint32(len(sizes)))
	for i, s := range sizes {
		binary.BigEndian.PutUint32(stszBody[8+i*4:], s)
	}
	stsz := mkFullBox("stsz", 0, 0, stszBody)

	stcoBody := make([]byte, 4+len(chunkOffs)*4)
	binary.BigEndian.PutUint32(stcoBody[0:], uint32(len(chunkOffs)))
	for i, o := range chunkOffs {
		binary.BigEndian.PutUint32(stcoBody[4+i*4:], o)
	}
	stco := mkFullBox("stco", 0, 0, stcoBody)

	stssBody := make([]byte, 4+len(keys)*4)
	binary.BigEndian.PutUint32(stssBody[0:], uint32(len(keys)))
	for i, k := range keys {
		binary.BigEndian.PutUint32(stssBody[4+i*4:], k)
	}
	stss := mkFullBox("stss", 0, 0, stssBody)

	stbl := mkBox("stbl", bytes.Join([][]byte{stsd, stts, stsc, stsz, stco, stss}, nil))
	vmhd := mkFullBox("vmhd", 0, 1, make([]byte, 8))
	dinf := mkBox("dinf", []byte{})
	minf := mkBox("minf", bytes.Join([][]byte{vmhd, dinf, stbl}, nil))
	mdia := mkBox("mdia", bytes.Join([][]byte{mdhd, hdlr, minf}, nil))
	trak := mkBox("trak", bytes.Join([][]byte{tkhd, mdia}, nil))
	moov := mkBox("moov", bytes.Join([][]byte{mvhd, trak}, nil))
	mdat := mkBox("mdat", make([]byte, 2048))
	return bytes.Join([][]byte{ftyp, moov, mdat}, nil), "合成片：120个采样/30帧每秒/1280x720/4个关键帧"
}

func badMP4Bytes(good []byte) []byte {
	if len(good) > 100 {
		return append([]byte(nil), good[:len(good)-100]...)
	}
	return []byte("12345678junkjunkjunkjunk")
}
