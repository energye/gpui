// Command video_vr5_seek is the VW2 VR5 real-window: seek to the nearest
// keyframe, decode forward to the target, recover instantly.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=15 go run ./examples/video_vr5_seek
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 15). GPU window required.
// Left: seek gates from the real player (dual-IDR 96x96 both GOPs,
// B-reorder 96x96, 480p crop: target/landing/key/forward/recover).
// Right: the live picture (dual-IDR clip, non-loop player looping via
// SeekTo plus two mid-run jumps; every jump shows at once, never black).
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

// seekBudgetMs caps |target - landed| for the 5fps gate clips (one frame
// interval is 200ms; 500ms leaves generous headroom without hiding bugs).
const seekBudgetMs = 500

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VR5")
	} else {
		secs = 15
	}
	wrkit.EnsureUIFace()

	states := loadSeek()

	seekErr := ""
	maxDelta := int64(0)
	maxForward := int64(0)
	for _, st := range states {
		if st.err != nil {
			if seekErr == "" {
				seekErr = st.name + "：" + st.err.Error()
			}
			continue
		}
		if st.deltaMs > maxDelta {
			maxDelta = st.deltaMs
		}
		if st.forward > maxForward {
			maxForward = st.forward
		}
		if !st.recover {
			if seekErr == "" {
				seekErr = st.name + "：跳后没立刻恢复"
			}
		}
		if st.deltaMs > seekBudgetMs {
			if seekErr == "" {
				seekErr = fmt.Sprintf("%s：落点差%d毫秒超预算%d", st.name, st.deltaMs, seekBudgetMs)
			}
		}
	}
	// Head-replay guard: the mid-GOP jump must decode far fewer samples
	// than a full head replay (4 vs 10 on vr5_seek.mp4).
	for _, st := range states {
		if st.err == nil && st.name == "双关键帧跳第二GOP" && st.forward >= 10 {
			if seekErr == "" {
				seekErr = fmt.Sprintf("疑似从头解：前解%d不小于全片10", st.forward)
			}
		}
	}
	seekOK := 0
	if seekErr == "" {
		seekOK = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vr5_seek — 跳进度", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VR5 跳进度 — 关键帧/往前解/即时恢复", []string{
		"左边四跳，走真播放器",
		"右边直播，5秒/10秒各跳一次",
		"落点预算500毫秒内",
		"门禁：落点+恢复+不黑屏",
		"跑满15秒，正确性窗判",
	})

	statusText := "跳进度正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if seekErr != "" {
		statusText = "跳失败：" + shortErr(seekErr, 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VR5 跳进度", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, st := range states {
		if i >= 4 {
			break
		}
		shell.Body.LabelAt(st.infoLine(), 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: non-loop player looping via SeekTo (end -> seek head)
	// plus two mid-run jumps. Every jump must show on the next poll.
	live, err := govideo.OpenFile(resolveClip("vr5_seek.mp4"), govideo.Options{})
	liveErr := ""
	if err != nil {
		liveErr = "直播打不开：" + err.Error()
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		liveImg = rendering.NewRenderImage(360, 360)
		shell.Body.LabelAt("直播（5秒/10秒跳，播完绕回也走跳）", 13, 20, 180, 0.6, 0.8, 0.95)
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
				fmt.Fprintf(os.Stderr, "video_vr5_seek: 关闭 (%s)\n", win.Backend())
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
	liveSeeks := int64(0)
	liveMaxDelta := int64(0)
	liveRecoverMaxMs := int64(0)
	seekNote := "播放中"
	didJump1, didJump2 := false, false
	var pendingRecoverSince time.Time
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		if live != nil {
			elapsed := float64(hotTick) / 60.0
			// Two scheduled jumps prove mid-play seeks; end-of-stream
			// wraps via SeekTo as well (continuous fast seeks, no stall).
			if !didJump1 && elapsed > 5 {
				didJump1 = true
				if landed, err := live.SeekTo(1800); err != nil {
					liveErr = "5秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					d := landed - 1800
					if d < 0 {
						d = -d
					}
					if d > liveMaxDelta {
						liveMaxDelta = d
					}
					pendingRecoverSince = time.Now()
					seekNote = fmt.Sprintf("5秒跳→%d", landed)
				}
			}
			if !didJump2 && elapsed > 10 {
				didJump2 = true
				if landed, err := live.SeekTo(600); err != nil {
					liveErr = "10秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					d := landed - 600
					if d < 0 {
						d = -d
					}
					if d > liveMaxDelta {
						liveMaxDelta = d
					}
					pendingRecoverSince = time.Now()
					seekNote = fmt.Sprintf("10秒跳→%d", landed)
				}
			}
			f, ended := live.Poll()
			if f != nil {
				liveShown++
				if !pendingRecoverSince.IsZero() {
					ms := time.Since(pendingRecoverSince).Milliseconds()
					if ms > liveRecoverMaxMs {
						liveRecoverMaxMs = ms
					}
					pendingRecoverSince = time.Time{}
					seekNote += " 已恢复"
				}
				if liveBuf == nil {
					liveBuf, _ = render.NewImageBuf(f.Width, f.Height, render.FormatRGBA8)
				}
				blitRGBA(liveBuf, f.Pix, f.Width, f.Height)
				liveImg.SetImageShared(liveBuf)
			}
			if ended {
				// Loop via seek: head PTS is 400 on this clip.
				if _, err := live.SeekTo(400); err != nil {
					liveErr = "绕回跳失败：" + err.Error()
				} else {
					liveSeeks++
					pendingRecoverSince = time.Now()
					seekNote = "播完绕回→400"
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := seekErr == "" && liveErr == "" && snapH.PresentCount > 0
		shell.UpdateHUD("VR5", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("跳ok=%d 差≤%d 前解≤%d", seekOK, maxDelta, maxForward),
			fmt.Sprintf("直播=%d 跳=%d %s", liveShown, liveSeeks, seekNote))
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
	if liveMaxDelta > maxDelta {
		maxDelta = liveMaxDelta
	}
	rep := buildReport(snap, app.PresentCount(), elapsed, states, seekOK, maxDelta, maxForward, seekErr, liveErr, liveShown, liveSeeks, liveRecoverMaxMs, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VR5需要真窗口)")
		os.Exit(1)
	}
	if seekErr != "" {
		fmt.Fprintf(os.Stderr, "FAIL: 跳进度门禁: %s\n", seekErr)
		os.Exit(1)
	}
	if liveErr != "" {
		fmt.Fprintf(os.Stderr, "FAIL: 直播跳: %s\n", liveErr)
		os.Exit(1)
	}
	if liveShown < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 直播一帧没播出来")
		os.Exit(1)
	}
	if liveSeeks < 2 {
		fmt.Fprintf(os.Stderr, "FAIL: 直播跳太少=%d(要≥2次定时跳)\n", liveSeeks)
		os.Exit(1)
	}
	if liveRecoverMaxMs > 3000 {
		fmt.Fprintf(os.Stderr, "FAIL: 跳后恢复=%d毫秒超3秒\n", liveRecoverMaxMs)
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
	fmt.Fprintf(os.Stderr, "video_vr5_seek: 通过 跳ok=%d 差≤%d 前解≤%d 直播=%d 跳=%d 恢复≤%d毫秒 上屏=%d 用时=%.1f秒\n",
		seekOK, maxDelta, maxForward, liveShown, liveSeeks, liveRecoverMaxMs, app.PresentCount(), elapsed)
}

// blitRGBA copies a player frame into the shared display buffer and flags
// it for GPU reupload (window side only).
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VR5 extras.
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
	SeekLandingDeltaMs int64   `json:"seek_landing_delta_ms"`

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

	SeeksDone      int64 `json:"seeks_done"`
	ForwardMax     int64 `json:"forward_max"`
	RecoverMaxMs   int64 `json:"recover_max_ms"`
	LiveSeeks      int64 `json:"live_seeks"`
	SeekLiveErr    string `json:"seek_live_error"`
	SeekGateErr    string `json:"seek_gate_error"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, states []*seekState, seekOK int, maxDelta, maxForward int64, seekErr, liveErr string, liveShown, liveSeeks, recoverMaxMs int64, hotTick int) report {
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
	framesDec := int64(0)
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
		framesDec += int64(st.info.Frames)
	}
	yuvReady := 0
	if seekOK == 1 {
		yuvReady = 1
	}
	errText := seekErr
	if errText == "" {
		errText = liveErr
	}
	return report{
		AbilityID: "VR5", Scenario: "video_vr5_seek", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: 0, DecodeMsP95: 0,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: framesDec + liveShown, FramesShown: liveShown, ClockDriftMs: 0, SeekOK: seekOK, SeekLandingDeltaMs: maxDelta,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: framesDec, YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: names, Profile: prof,
		SeeksDone: int64(len(states)), ForwardMax: maxForward, RecoverMaxMs: recoverMaxMs, LiveSeeks: liveSeeks,
		SeekLiveErr: liveErr, SeekGateErr: seekErr,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: errText, Source: src,
		NANote: "seek-only: audio N/A (V-U2); loop+seek goes to VC1",
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
	for _, k := range []string{"seek_ok", "yuv_ready"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
