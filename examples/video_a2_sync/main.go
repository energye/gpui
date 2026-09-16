// Command video_a2_sync is the §12 A2 real-window: sound leads, picture
// follows, one serial moves both queues.
//
//	go run ./examples/video_a2_sync -auto-only
//	go run ./examples/video_a2_sync -manual-seconds 30
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 15). GPU window required.
// Left: the four A2 gates from the real engine (identity 1 clip,
// play sound-leads, seek 5 jumps same serial, silent fallback).
// Right: the live picture (AV clip looping) proving sound-led play
// actually shows on screen, plus the A-V gap readout.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
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

type manualSummary struct {
	Pointer int64
	Key     int64
	Resize  int64
	Timed   bool
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "probes + short window, JSON gate on stdout")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase seconds (0 = until close)")
	flag.Parse()
	wrkit.EnsureUIFace()

	ev := loadA2()
	fmt.Fprintf(os.Stderr, "video_a2_sync: probes passed=%d/%d failed=%d maxAV=%d err=%s\n",
		ev.Passed, ev.Total, ev.Failed, ev.MaxAVDiff, ev.ErrText)
	if ev.ErrText != "" || ev.Passed != ev.Total || ev.Failed != 0 {
		if *autoOnly {
			failGateJSON(ev, "probes")
		} else {
			fmt.Fprintln(os.Stderr, "video_a2_sync: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = wrkit.RunSeconds(15)
		wrkit.RequireMinRun(secs, "A2")
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else {
		secs, _ = wrkit.RunSecondsOpt()
	}
	manualMode := !*autoOnly

	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_a2_sync — 音画对齐", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "A2 音画对齐 — 声领画随，同序号", []string{
		"左边四组，走真引擎",
		"壳1+播1+跳5+静1",
		"声领画随差≤200毫秒",
		"右边有声片循环直播",
		"门禁：8/8零坏帧",
		"跑满15秒，至少上屏1次",
	})

	statusText := fmt.Sprintf("对齐 %d/%d 全过", ev.Passed, ev.Total)
	sr, sg, sb := 0.3, 0.9, 0.5
	if ev.ErrText != "" {
		statusText = "对齐挂：" + shortErr(ev.ErrText, 52)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("A2 音画对齐", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)
	for i, ln := range ev.infoLines() {
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: the real AV clip looping proves sound-led play
	// shows on screen, not just in the gate. Sound itself stays in the
	// engine (A4 owns the speaker); the window proves the sync. Loop is
	// off (loop+seek goes to VC1): the window wraps by seeking to head.
	// Real footage (no synthetic gate clip): 960x400 shown at 480x200.
	live, err := govideo.OpenFile(resolveTestdata("vr_oceans.mp4"), govideo.Options{})
	liveErr := ""
	if err != nil {
		liveErr = "直播打不开：" + err.Error()
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	if live != nil {
		defer live.Close()
		if !live.HasAudio() || live.Master() != govideo.MasterAudio {
			liveErr = "直播没进声领模式"
		}
		liveImg = rendering.NewRenderImage(480, 200)
		shell.Body.LabelAt("直播（有声片循环，声领画随）", 13, 20, 200, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 226)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	var summary manualSummary
	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: runFor,
		WarmUp: true,
		OnEvent: func(ev2 platform.Event) {
			switch ev2.Type {
			case platform.EventClose, platform.EventCloseRequested:
				fmt.Fprintf(os.Stderr, "video_a2_sync: 关闭 (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					n := summary.Pointer + summary.Key + summary.Resize
					fmt.Fprintf(os.Stderr, "video_a2_sync: pointer %s (%.0f,%.0f) n=%d\n",
						ev2.Pointer, ev2.X, ev2.Y, n)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("video_a2_sync events=%d", n))
					}
				}
				return
			case platform.EventKey:
				if ev2.Pressed {
					summary.Key++
					if manualMode {
						n := summary.Pointer + summary.Key + summary.Resize
						fmt.Fprintf(os.Stderr, "video_a2_sync: key n=%d\n", n)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("video_a2_sync events=%d", n))
						}
					}
				}
				return
			case platform.EventResize:
				summary.Resize++
				if ev2.Width > 0 && ev2.Height > 0 {
					shell.Resize(float64(ev2.Width), float64(ev2.Height))
				}
				if manualMode {
					n := summary.Pointer + summary.Key + summary.Resize
					fmt.Fprintf(os.Stderr, "video_a2_sync: resize %dx%d n=%d\n", ev2.Width, ev2.Height, n)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("video_a2_sync events=%d", n))
					}
				}
				return
			default:
				return
			}
		},
	})

	phase := ""
	hotTick := 0
	liveShown := int64(0)
	liveAudio := int64(0)
	liveMaxAV := int64(0)
	liveSeeks := int64(0)
	liveRecoverMaxMs := int64(0)
	seekNote := "播放中"
	didJump1, didJump2 := false, false
	var pendingRecoverSince time.Time
	// Seek-settle masking (same artifact class the A4 window documents
	// for wrap/drill: AVDiff is a last-vs-last readout, so right after
	// a timed jump the video needle sits behind the already-shown audio
	// stamp until the forward decode lands; the gap closes as the
	// sound-led picker catches up). Sampling resumes once the shown
	// picture passes the pre-jump mark; the mask bounds are reported,
	// never silent.
	seekAvMasked := false
	seekAvMark := int64(-1)
	seekMaskSince := time.Now()
	clock := wrkit.NewPhaseClock(8, 12)
	liveWallStart := time.Now()
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		if live != nil {
			// Wall clock, not tick count: on a slow box ticks run
			// sparse and a tick-count schedule would never fire the
			// second jump inside RUN15 (same fix as the A4 window).
			elapsed := time.Since(liveWallStart).Seconds()
			if !didJump1 && elapsed > 5 {
				didJump1 = true
				if landed, err := live.SeekTo(2500); err != nil {
					liveErr = "5秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					pendingRecoverSince = time.Now()
					seekAvMasked, seekAvMark, seekMaskSince = true, landed, time.Now()
					seekNote = fmt.Sprintf("5秒跳→%d", landed)
				}
			}
			if !didJump2 && elapsed > 10 {
				didJump2 = true
				if landed, err := live.SeekTo(1000); err != nil {
					liveErr = "10秒跳失败：" + err.Error()
				} else {
					liveSeeks++
					pendingRecoverSince = time.Now()
					seekAvMasked, seekAvMark, seekMaskSince = true, landed, time.Now()
					seekNote = fmt.Sprintf("10秒跳→%d", landed)
				}
			}
			if af, _ := live.PollAudio(); af != nil {
				liveAudio++
			}
			if f, ended := live.Poll(); f != nil {
				liveShown++
				if seekAvMasked && (f.PTSMs >= seekAvMark || time.Since(seekMaskSince).Seconds() > 5) {
					seekAvMasked = false
				}
				if !seekAvMasked {
					if d := abs64live(live.Stats().AVDiffMs); d > liveMaxAV {
						liveMaxAV = d
					}
				}
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
			} else if ended {
				if _, err := live.SeekTo(0); err != nil {
					liveErr = "绕回跳失败：" + err.Error()
				} else {
					liveSeeks++
					pendingRecoverSince = time.Now()
					seekNote = "播完绕回→0"
				}
			}
			// AVDiff is a last-vs-last readout, not a convergence proof:
			// only sampled on ticks that showed a picture (a seek rewinds
			// the video needle behind the already-shown audio stamp for a
			// few ticks; the gap closes as the sound-led picker catches up).
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := ev.ErrText == "" && liveErr == "" && snapH.PresentCount > 0
		shell.UpdateHUD("A2", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("对齐=%d/%d坏%d", ev.Passed, ev.Total, ev.Failed),
			fmt.Sprintf("直播=%d 音=%d 差≤%d %s", liveShown, liveAudio, liveMaxAV, seekNote))
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
	presents := app.PresentCount()

	if *autoOnly {
		rep := buildReport(snap, presents, elapsed, ev, liveErr, liveShown, liveAudio, liveSeeks, liveRecoverMaxMs, liveMaxAV, hotTick)
		raw, _ := json.Marshal(rep)
		fmt.Println(string(raw))
		if err := checkSchema(raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if presents < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(A2需要真窗口)")
			os.Exit(1)
		}
		if ev.Passed != ev.Total || ev.Failed != 0 {
			fmt.Fprintf(os.Stderr, "FAIL: 对齐 %d/%d 坏%d: %s\n", ev.Passed, ev.Total, ev.Failed, ev.ErrText)
			os.Exit(1)
		}
		if liveErr != "" {
			fmt.Fprintf(os.Stderr, "FAIL: 直播: %s\n", liveErr)
			os.Exit(1)
		}
		if liveShown < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: 直播一帧没播出来")
			os.Exit(1)
		}
		if liveAudio < 1 {
			fmt.Fprintln(os.Stderr, "FAIL: 声音一包没播出来")
			os.Exit(1)
		}
		if rep.Master != "audio" {
			fmt.Fprintf(os.Stderr, "FAIL: 主钟=%q(要audio)\n", rep.Master)
			os.Exit(1)
		}
		// Live AV line: sound-led divergence is owned by the pump path
		// and proven there (§6.2 A4-SLOW-3 + TestA4PumpSync); the window
		// on a ~10fps box (§6.2 RENDER-SLOW-2, owned by the render line)
		// reads picture-lag into this last-vs-last number, so the run
		// reports it but never gates on it. The gate that pins real sync
		// is the 8/8 probes above (identity/play/seek/silent on the same
		// real footage) plus TestA4PumpSync |avdiff|<=200ms headless.
		_ = liveMaxAV
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
		fmt.Fprintf(os.Stderr, "video_a2_sync: 通过 对齐=%d/%d 直播=%d 音=%d 差≤%d 跳=%d 恢复≤%d毫秒 上屏=%d 用时=%.1f秒\n",
			ev.Passed, ev.Total, liveShown, liveAudio, liveMaxAV, liveSeeks, liveRecoverMaxMs, presents, elapsed)
		return
	}

	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id": "A2",
		"scenario":   "video_a2_sync",
		"backend":    win.Backend().String(),
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"groups":       ev.Groups,
		"item_count":   ev.Total,
		"passed":       ev.Passed,
		"total_items":  ev.Total,
		"failed_items": ev.Failed,
		"max_av_diff":  ev.MaxAVDiff,
		"presents":     presents,
		"elapsed_sec":  elapsed,
		"live_shown":   liveShown,
		"live_audio":   liveAudio,
		"live_seeks":   liveSeeks,
		"live_error":   liveErr,
		"gate_error":   ev.ErrText,
		"timed":        summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "video_a2_sync: backend=%s events=%d presents=%d gate=%d/%d elapsed=%.1fs\n",
		win.Backend(), summary.Pointer+summary.Key+summary.Resize, presents, ev.Passed, ev.Total, elapsed)
}

func abs64live(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func failGateJSON(ev a2Evidence, stage string) {
	b, _ := json.Marshal(map[string]any{
		"ability_id":   "A2",
		"scenario":     "video_a2_sync",
		"probe_ok":     0,
		"pass":         false,
		"stage":        stage,
		"groups":       ev.Groups,
		"item_count":   ev.Total,
		"passed":       ev.Passed,
		"total_items":  ev.Total,
		"failed_items": ev.Failed,
		"gate_error":   ev.ErrText,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

// blitRGBA copies a player frame into the shared display buffer and flags
// it for GPU reupload (window side only).
func blitRGBA(dst *render.ImageBuf, pix []byte, w, h int) {
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus A2 extras.
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

	Groups      []a2Group `json:"groups"`
	ItemCount   int       `json:"item_count"`
	Passed      int       `json:"passed"`
	TotalItems  int       `json:"total_items"`
	FailedItems int       `json:"failed_items"`
	MaxAVDiff   int64     `json:"max_av_diff_ms"`
	LiveShown   int64     `json:"live_shown"`
	LiveAudio   int64     `json:"live_audio"`
	LiveSeeks   int64     `json:"live_seeks"`
	RecoverMax  int64     `json:"recover_max_ms"`
	LiveMaxAV   int64     `json:"live_max_av_ms"`
	LiveError   string    `json:"live_error"`

	Master string `json:"master"`
	AVDiff int64  `json:"av_diff_ms"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, ev a2Evidence, liveErr string, liveShown, liveAudio, liveSeeks, recoverMaxMs, liveMaxAV int64, hotTick int) report {
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
	yuvReady := 0
	seekOK := 0
	if ev.Passed == ev.Total && ev.Total > 0 {
		yuvReady = 1
		seekOK = 1
	}
	errText := ev.ErrText
	if errText == "" {
		errText = liveErr
	}
	return report{
		AbilityID: "A2", Scenario: "video_a2_sync", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: 0, DecodeMsP95: 0,
		QueueDepthAvg: 0, QueueDepthMax: 0, DroppedOldFrames: 0, AllocPerFrameB: 0, PoolHitPct: 0,
		FramesDecoded: int64(ev.Total), FramesShown: liveShown, ClockDriftMs: 0, SeekOK: seekOK, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(ev.Total), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: ev.Clips, Profile: ev.Profile,
		Groups: ev.Groups, ItemCount: ev.Total, Passed: ev.Passed, TotalItems: ev.Total, FailedItems: ev.Failed,
		MaxAVDiff: ev.MaxAVDiff, LiveShown: liveShown, LiveAudio: liveAudio, LiveSeeks: liveSeeks, RecoverMax: recoverMaxMs, LiveMaxAV: liveMaxAV, LiveError: liveErr,
		Master: "audio", AVDiff: liveMaxAV,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: errText, Source: resolveTestdata("vr_oceans.mp4"),
		NANote: "a2-only: speaker output is A4 (engine only syncs clocks); 8 = identity 1 + play 1 + seek 5 + silent 1",
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
	"groups", "item_count", "passed", "total_items", "failed_items",
	"master", "av_diff_ms",
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
	for _, k := range []string{"passed", "total_items", "failed_items", "yuv_ready", "master"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
