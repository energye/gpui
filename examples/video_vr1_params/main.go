// Command video_vr1_params is the VW0 VR1 real-window: H.264 params + split.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/video_vr1_params
//
// Window: 1200x800. RUN_SECONDS>=5. GPU window required.
// Left: parameter panel from real video/h264 parsing (out-of-band avcC
// plus in-band NALUs). Right: NALU histogram + cut frames.
// Bottom: missing-parameter stream must error readably, no panic.
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/video/h264"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 800

type paramsResult struct {
	source      string
	packing     string
	profile     string
	profileIDC  uint8
	level       string
	levelIDC    uint8
	levelOK     bool
	width       uint32
	height      uint32
	entropy     string
	cabac       bool
	spsTotal    int
	ppsTotal    int
	spsInband   int
	ppsInband   int
	nalTotal    int
	hist        map[int]int
	frames      []h264.Frame
	idrFrames   int
	profilesHit map[uint8]bool
	err         error
	parseMs     float64
}

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR1")
	} else {
		secs = 5
	}
	wrkit.EnsureUIFace()

	res := loadParams(os.Getenv("VIDEO_MP4_PATH"))

	badErrStr := ""
	badOK := false
	func() {
		defer func() {
			if recover() != nil {
				badErrStr = "解析时崩了"
				badOK = false
			}
		}()
		// 切片指向从没见过的参数集，必须检出，不能乱切。
		ps := h264.NewParamSets()
		if _, _, err := ps.RequireForSlice(0); err != nil {
			badErrStr = translateH264Error(err.Error())
			badOK = true
		} else {
			badErrStr = "坏流居然通过了，没报对"
			badOK = false
		}
	}()

	spsOK := 0
	if res.spsTotal > 0 && res.err == nil {
		spsOK = 1
	}
	ppsOK := 0
	if res.ppsTotal > 0 && res.err == nil {
		ppsOK = 1
	}
	framesSplit := len(res.frames)

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr1_params — H.264参数与切分", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR1 H.264参数与切分 — 档位/等级/切帧", []string{
		"左边是参数面板，用真解析结果",
		"右边是类型统计加切出的帧",
		"缺参数的坏流必须报可读错",
		"门禁：参数对=1 切帧>=1 档位全认",
		"跑满5秒，至少上屏1次，输出同步源",
	})

	statusText := "解析成功"
	sr, sg, sb := 0.3, 0.9, 0.5
	if res.err != nil {
		statusText = "解析失败：" + shortErr(translateH264Error(res.err.Error()), 64)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR1 H.264参数与切分", 22, 20, 12, 0.92, 0.94, 0.98)
	statusLabel := shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range res.infoLines() {
		if i >= 14 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	barX, barY, barW := 640.0, 110.0, 300.0
	shell.Body.LabelAt("类型统计(个数)", 13, barX, 76, 0.6, 0.8, 0.95)
	order := []int{h264.NALSPS, h264.NALPPS, h264.NALSliceIDR, h264.NALSliceNonIDR, h264.NALSei, h264.NALAUD}
	maxN := 1
	for _, t := range order {
		if res.hist[t] > maxN {
			maxN = res.hist[t]
		}
	}
	for i, t := range order {
		n := res.hist[t]
		y := barY + float64(i)*30
		shell.Body.LabelAt(fmt.Sprintf("%s %d个", h264.TypeNameCN(t), n), 11, barX, y, 0.65, 0.75, 0.85)
		w := 0.0
		if n > 0 {
			w = float64(n) / float64(maxN) * (barW - 130)
		}
		if w > 0 {
			shell.Body.ColorAt(w, 14, barX+110, y-2, 0.3, 0.7, 0.95, 1, false)
		}
	}
	fy := barY + float64(len(order))*30 + 12
	shell.Body.LabelAt(fmt.Sprintf("切出 %d 帧(其中IDR %d帧)", framesSplit, res.idrFrames), 13, barX, fy, 0.6, 0.85, 0.6)
	for i := 0; i < len(res.frames) && i < 4; i++ {
		f := res.frames[i]
		kind := "非IDR"
		if f.IsIDR {
			kind = "IDR"
		}
		shell.Body.LabelAt(fmt.Sprintf("第%d帧 %s 切片%d %d字节", i+1, kind, f.SliceCount, f.SizeBytes), 11, barX, fy+28+float64(i)*20, 0.65, 0.75, 0.85)
	}

	badR, badG, badB := 0.45, 0.85, 0.55
	badPrefix := "缺参数坏流正常报错："
	if !badOK {
		badR, badG, badB = 0.95, 0.4, 0.35
		badPrefix = "缺参数坏流没报对："
	}
	shell.Body.LabelAt(badPrefix+shortErr(badErrStr, 80), 12, 20, shell.Body.H-56, badR, badG, badB)
	shell.Body.LabelAt("来源："+shortErr(res.source, 96), 11, 20, shell.Body.H-30, 0.55, 0.65, 0.75)

	hot := rendering.NewRenderColorBox(90, 90, 0.3, 0.7, 0.95, 1)
	hot.SetRepaintBoundary(true)
	hotX, hotY := 640.0, 560.0
	shell.Body.Place(hot, hotX, hotY)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "video_vr1_params: 关闭 (%s)\n", win.Backend())
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
		gatePreview := res.err == nil && spsOK == 1 && ppsOK == 1 && framesSplit >= 1 && badOK
		shell.UpdateHUD("VR1", phaseCN(phase), app, gatePreview && snapH.PresentCount > 0,
			fmt.Sprintf("参数=%d/%d 切帧=%d", spsOK, ppsOK, framesSplit),
			fmt.Sprintf("解析=%.2f毫秒 %s", res.parseMs, shortErr(res.source, 36)))
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
	rep := buildReport(snap, app.PresentCount(), elapsed, res, spsOK, ppsOK, framesSplit, badOK, badErrStr, hotTick, statusLabel != nil)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR1需要真窗口)")
		os.Exit(1)
	}
	if res.err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: 参数解析失败: %s\n", translateH264Error(res.err.Error()))
		os.Exit(1)
	}
	if spsOK != 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 参数集不对(sps_ok=0)")
		os.Exit(1)
	}
	if ppsOK != 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 图参数不对(pps_ok=0)")
		os.Exit(1)
	}
	if framesSplit < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一帧都没切出来")
		os.Exit(1)
	}
	if !res.levelOK {
		fmt.Fprintf(os.Stderr, "FAIL: 等级不支持: %s\n", res.level)
		os.Exit(1)
	}
	if !badOK {
		fmt.Fprintf(os.Stderr, "FAIL: 缺参数坏流没报对: %s\n", badErrStr)
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
	fmt.Fprintf(os.Stderr, "video_vr1_params: 通过 参数=%d/%d 切帧=%d 档位=%s 等级=%s 上屏=%d 用时=%.1f秒\n",
		spsOK, ppsOK, framesSplit, res.profile, res.level, app.PresentCount(), elapsed)
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

func translateH264Error(s string) string {
	if s == "" {
		return ""
	}
	switch {
	case containsStr(s, "slice groups"):
		return "条带组是老抗丢包件，本阶段不支持"
	case containsStr(s, "data partitioning"):
		return "数据分区是老抗丢包件，本阶段不支持"
	case containsStr(s, "unsupported NAL"):
		return "这个NAL类型本阶段不支持"
	case containsStr(s, "never seen"), containsStr(s, "needs a PPS"), containsStr(s, "needs an SPS"):
		return "切片要用的参数集没见过，切不了"
	case containsStr(s, "bad SPS"), containsStr(s, "bad PPS"):
		return "参数集坏了，读不懂"
	case containsStr(s, "bad avcC"), containsStr(s, "no parameter sets"):
		return "盒子里的参数区坏了"
	case containsStr(s, "bad NAL"), containsStr(s, "no NAL"):
		return "NAL单元坏了或找不到"
	case containsStr(s, "slice header"):
		return "切片头坏了，帧边界定不下来"
	case containsStr(s, "truncated"):
		return "数据被截断"
	case containsStr(s, "unsupported level"):
		return "等级超范围"
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR1 extras.
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
	Profile             string  `json:"profile"`
	Level               string  `json:"level"`
	NALTotal            int     `json:"nal_total"`
	IDRFrames           int     `json:"idr_frames"`
	Packing             string  `json:"packing"`
	Entropy             string  `json:"entropy"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	BadFileOK    bool   `json:"bad_file_ok"`
	BadFileError string `json:"bad_file_error"`
	DemuxError   string `json:"demux_error"`
	Source       string `json:"source"`
	NANote       string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, res *paramsResult, spsOK, ppsOK, framesSplit int, badOK bool, badErr string, hotTick int, _ bool) report {
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
	if res.err != nil {
		demuxErrStr = res.err.Error()
	}
	_ = hotTick
	return report{
		AbilityID: "VR1", Scenario: "video_vr1_params", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: res.parseMs, DecodeMsP95: res.parseMs,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: framesSplit, FramesShown: presents, ClockDriftMs: 0, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 524288, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: spsOK, PPSOk: ppsOK, FramesSplit: framesSplit, YUVReady: 0, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Profile: res.profile, Level: res.level, NALTotal: res.nalTotal, IDRFrames: res.idrFrames, Packing: res.packing, Entropy: res.entropy,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		BadFileOK: badOK, BadFileError: badErr, DemuxError: demuxErrStr, Source: res.source,
		NANote: "params-only: queue/clock/seek/yuv/color/golden N/A in VR1; sps/pps/frames/profile/level are the gates",
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
	"profile", "level", "nal_total", "idr_frames", "packing", "entropy",
	"time_to_first_frame_ms", "present_count", "paint_count", "bad_file_ok", "bad_file_error",
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
	for _, k := range []string{"sps_ok", "pps_ok", "frames_split", "profile", "level"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
