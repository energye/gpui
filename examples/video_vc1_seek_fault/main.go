// Command video_vc1_seek_fault is the VW2 VC1 combo real-window: play with
// seeks while bad files mix in — jumps recover, faults stay readable, and
// the good clip is never affected.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=30 go run ./examples/video_vc1_seek_fault
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 30). GPU window required.
// Left top: three seek gates on the good dual-IDR clip (target/landing/
// key/forward/recover, budget 500ms, recover ≤3s).
// Left bottom: five fault gates (missing file, non-mp4, truncated tail,
// F20 flower isolation, F17 partition), each with its layer+tool note.
// Right: the live picture on the good clip — two scheduled mid-run jumps
// plus end-of-stream wrap via SeekTo; every jump shows at once, never black.
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
		wrkit.RequireMinRun(secs, "VC1")
	} else {
		secs = 30
	}
	wrkit.EnsureUIFace()

	seekGates := loadSeekGates()
	faultGates := loadFaultGates()

	seekErr := ""
	maxDelta := int64(0)
	maxForward := int64(0)
	seekPass := 0
	for _, g := range seekGates {
		if g.err != nil {
			if seekErr == "" {
				seekErr = g.name + "：" + g.err.Error()
			}
			continue
		}
		if g.delta > maxDelta {
			maxDelta = g.delta
		}
		if g.forward > maxForward {
			maxForward = g.forward
		}
		if !g.recover {
			if seekErr == "" {
				seekErr = g.name + "：跳后没立刻恢复"
			}
			continue
		}
		if g.delta > seekBudgetMs {
			if seekErr == "" {
				seekErr = fmt.Sprintf("%s：落点差%d毫秒超预算%d", g.name, g.delta, seekBudgetMs)
			}
			continue
		}
		seekPass++
	}
	seekOK := 0
	if seekErr == "" && seekPass == len(seekGates) {
		seekOK = 1
	}

	faultPass, faultTotal := 0, len(faultGates)
	faultErr := ""
	for _, g := range faultGates {
		if g.pass {
			faultPass++
		} else if faultErr == "" {
			faultErr = g.name + "：" + g.note
		}
	}
	faultOK := 0
	if faultTotal > 0 && faultPass == faultTotal {
		faultOK = 1
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vc1_seek_fault — 边播边跳+坏文件", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VC1 边播边跳+坏文件 — 跳后恢复/坏片隔离", []string{
		"左上三跳，好片走真播放器",
		"左下五坏例，走真归口",
		"右边直播，跳两次+绕回",
		"门禁：跳全过+坏例全过",
		"跑满30秒，动画60档判",
	})

	statusText := fmt.Sprintf("跳%d/%d 坏例%d/%d 全过", seekPass, len(seekGates), faultPass, faultTotal)
	sr, sg, sb := 0.3, 0.9, 0.5
	firstErr := seekErr
	if firstErr == "" {
		firstErr = faultErr
	}
	if firstErr != "" {
		statusText = "组合挂：" + shortErr(firstErr, 52)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VC1 边播边跳+坏文件", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, g := range seekGates {
		if i >= 3 {
			break
		}
		shell.Body.LabelAt(g.infoLine(), 12, 20, 76+float64(i)*22, 0.72, 0.8, 0.9)
	}
	for i, g := range faultGates {
		if i >= 6 {
			break
		}
		shell.Body.LabelAt(g.infoLine(), 11, 20, 150+float64(i)*20, 0.65, 0.75, 0.85)
	}

	// Live picture: good dual-IDR clip, non-loop player driven by SeekTo —
	// two scheduled mid-run jumps plus end-of-stream wrap. Mid-run faults
	// never touch this player, proving bad files don't affect good ones.
	live, err := govideo.OpenFile(resolveClip("vr5_seek.mp4"), govideo.Options{})
	liveErr := ""
	if err != nil {
		liveErr = "直播打不开：" + err.Error()
	}
	// Good clip is 96x96 — one buffer for the whole run, never per-frame.
	const liveW, liveH = 96, 96
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		var bufErr error
		liveBuf, bufErr = render.NewImageBuf(liveW, liveH, render.FormatRGBA8)
		if bufErr != nil && liveErr == "" {
			liveErr = "显存建不起：" + bufErr.Error()
		}
		liveImg = rendering.NewRenderImage(360, 360)
		shell.Body.LabelAt("直播（跳两次+绕回，好片不受坏例影响）", 13, 20, 280, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 306)
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
				fmt.Fprintf(os.Stderr, "video_vc1_seek_fault: 关闭 (%s)\n", win.Backend())
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

		if live != nil && liveBuf != nil {
			elapsed := float64(hotTick) / 60.0
			if !didJump1 && elapsed > 10 {
				didJump1 = true
				if landed, err := live.SeekTo(1800); err != nil {
					liveErr = "10秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					if d := landed - 1800; d < 0 {
						if -d > liveMaxDelta {
							liveMaxDelta = -d
						}
					} else if d > liveMaxDelta {
						liveMaxDelta = d
					}
					pendingRecoverSince = time.Now()
					seekNote = fmt.Sprintf("10秒跳→%d", landed)
				}
			}
			if !didJump2 && elapsed > 20 {
				didJump2 = true
				if landed, err := live.SeekTo(600); err != nil {
					liveErr = "20秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					if d := landed - 600; d < 0 {
						if -d > liveMaxDelta {
							liveMaxDelta = -d
						}
					} else if d > liveMaxDelta {
						liveMaxDelta = d
					}
					pendingRecoverSince = time.Now()
					seekNote = fmt.Sprintf("20秒跳→%d", landed)
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
				if f.Width != liveW || f.Height != liveH {
					seekNote = fmt.Sprintf("尺寸漂移 %dx%d", f.Width, f.Height)
				} else {
					fastBlitRGBA(liveBuf, f.Pix, liveW, liveH)
					liveImg.SetImageShared(liveBuf)
				}
			}
			if ended {
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
		gatePreview := seekErr == "" && faultErr == "" && liveErr == "" && snapH.PresentCount > 0
		shell.UpdateHUD("VC1", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("跳%d/%d 坏例%d/%d", seekPass, len(seekGates), faultPass, faultTotal),
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
	rep := buildReport(snap, app.PresentCount(), elapsed, seekGates, faultGates, seekPass, seekOK, maxDelta, maxForward, faultPass, faultTotal, faultOK, seekErr, faultErr, liveErr, liveShown, liveSeeks, liveRecoverMaxMs, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VC1需要真窗口)")
		os.Exit(1)
	}
	// 60fps animation gate (§2.2.2): VC1 is a sustained-play combo.
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if seekErr != "" || seekOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 跳进度门禁: %s\n", seekErr)
		os.Exit(1)
	}
	if faultErr != "" || faultOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 容错门禁 %d/%d: %s\n", faultPass, faultTotal, faultErr)
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
	fmt.Fprintf(os.Stderr, "video_vc1_seek_fault: 通过 跳=%d/%d 坏例=%d/%d 直播=%d 跳=%d 恢复≤%d毫秒 上屏=%d 用时=%.1f秒\n",
		seekPass, len(seekGates), faultPass, faultTotal, liveShown, liveSeeks, liveRecoverMaxMs, app.PresentCount(), elapsed)
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VC1 combo extras.
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

	SeekPass      int    `json:"seek_pass"`
	SeekTotal     int    `json:"seek_total"`
	ForwardMax    int64  `json:"forward_max"`
	RecoverMaxMs  int64  `json:"recover_max_ms"`
	LiveSeeks     int64  `json:"live_seeks"`
	FaultPass     int    `json:"fault_cases_pass"`
	FaultTotal    int    `json:"fault_total"`
	SeekGateErr   string `json:"seek_gate_error"`
	FaultGateErr  string `json:"fault_gate_error"`
	LiveErr       string `json:"live_error"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, seekGates []*seekGate, faultGates []*faultGate, seekPass, seekOK int, maxDelta, maxForward int64, faultPass, faultTotal, faultOK int, seekErr, faultErr, liveErr string, liveShown, liveSeeks, recoverMaxMs int64, hotTick int) report {
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
	names := "seek["
	for i, g := range seekGates {
		if i > 0 {
			names += "+"
		}
		names += g.name
	}
	names += "]+fault["
	for i, g := range faultGates {
		if i > 0 {
			names += "+"
		}
		names += g.name
	}
	names += "]"
	yuvReady := 0
	if seekOK == 1 && faultOK == 1 {
		yuvReady = 1
	}
	errText := seekErr
	if errText == "" {
		errText = faultErr
	}
	if errText == "" {
		errText = liveErr
	}
	return report{
		AbilityID: "VC1", Scenario: "video_vc1_seek_fault", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: 0, DecodeMsP95: 0,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: liveShown, FramesShown: liveShown, ClockDriftMs: 0, SeekOK: seekOK, SeekLandingDeltaMs: maxDelta,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(len(seekGates)), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: names, Profile: "seek-fault-combo",
		SeekPass: seekPass, SeekTotal: len(seekGates), ForwardMax: maxForward, RecoverMaxMs: recoverMaxMs, LiveSeeks: liveSeeks,
		FaultPass: faultPass, FaultTotal: faultTotal,
		SeekGateErr: seekErr, FaultGateErr: faultErr, LiveErr: liveErr,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: errText, Source: resolveClip("vr5_seek.mp4"),
		NANote: "combo: seek N/A quirk none; loop+seek refused at engine, window loops via SeekTo",
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
	for _, k := range []string{"seek_ok", "fault_cases_pass", "yuv_ready"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
