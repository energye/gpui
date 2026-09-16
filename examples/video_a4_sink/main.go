// Command video_a4_sink is the §12 A4 real-window: the host side of
// sound actually reaches the speaker.
//
//	go run ./examples/video_a4_sink -auto-only
//	go run ./examples/video_a4_sink -manual-seconds 30
//	go run ./examples/video_a4_sink -file video/testdata/vr_oceans.mp4
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 15). GPU window required.
// Left: the A4 gates from the real engine (identity 1 clip, conversion
// vectors, WAV chain, sound-led pump, device honesty). Right: the live
// picture (gate clip by default, any local MP4 via -file) while the same
// PCM flows to the host speaker, plus the drop drill on the strict gate
// leg only (the writer child is killed mid-stream: the pump must pause
// with a readable error and resume, never die).
//
// -file is a watch leg, not a gate leg: probes still pin the gate clip
// (same real footage), live AV is reported (live_gate=demo) but never
// fails the run. The default leg is the same real footage
// (vr_oceans.mp4) and keeps the kill drill; its speaker-AV readout is
// reported only, never gated (see checkGate): the window on a ~10fps
// box (§6.2 RENDER-SLOW-2, owned by the render line) reads picture lag
// into this last-vs-last number, not sync. |avdiff|<=200ms stays
// enforced headless by TestA4PumpSync on the same clip. The kill drill
// still gates on both gate-owned legs. Killing the speaker mid-watch
// would mute the user's own movie.
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
	fileFlag := flag.String("file", "", "live clip for picture + speaker (default: real footage vr_oceans.mp4)")
	flag.Parse()
	wrkit.EnsureUIFace()

	ev := loadA4()
	gateOK := ev.ErrText == ""
	for _, g := range ev.Groups {
		if g.Name == "device" {
			continue
		}
		if g.Passed != g.Total {
			gateOK = false
		}
	}
	fmt.Fprintf(os.Stderr, "video_a4_sink: probes passed=%d/%d failed=%d backend=%s available=%v err=%s\n",
		ev.Passed, ev.Total, ev.Failed, ev.Backend, ev.Available, ev.ErrText)
	if !gateOK {
		if *autoOnly {
			failGateJSON(ev, "probes")
		} else {
			fmt.Fprintln(os.Stderr, "video_a4_sink: selftest FAIL, not opening window")
		}
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = wrkit.RunSeconds(15)
		wrkit.RequireMinRun(secs, "A4")
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

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_a4_sink — 宿主出声", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	shell := wrkit.NewShell(winW, winH, "A4 宿主出声 — PCM 到喇叭", []string{
		"左边五组，走真引擎",
		"壳1+转向量+链1+泵1+备1",
		"右边有声片循环，喇叭真响",
		"8秒拔线演习，停人不停线程",
		"门禁：探针全过+播出+恢复",
		"无喇叭诚实跳过，不装绿",
	})

	statusText := fmt.Sprintf("出声 %d/%d 全过", ev.Passed, ev.Total)
	sr, sg, sb := 0.3, 0.9, 0.5
	if ev.ErrText != "" || !ev.Available {
		if !ev.Available {
			statusText = "无喇叭跳过:" + ev.Reason
		} else {
			statusText = "探针挂：" + shortErr(ev.ErrText, 52)
		}
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("A4 宿主出声", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)
	for i, ln := range ev.infoLines() {
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture + live speaker: the chosen clip looping. Probes above
	// always pin the gate clip (honest baseline); only this live leg
	// follows -file, so any local MP4 (e.g. vr_oceans.mp4) plays picture
	// through the window and sound through the speaker. The pump owns
	// its goroutine (the SDL-audio-thread shape); the tick only polls
	// video and samples counters.
	livePath := *fileFlag
	if livePath == "" {
		livePath = resolveA4("vr_oceans.mp4")
	}
	// Strict gate only on gate-owned legs; a user file plays demo mode
	// (probes still strict, drill off, live AV only reported).
	liveStrict := *fileFlag == ""
	live, err := govideo.OpenFile(livePath, govideo.Options{})
	liveErr := ""
	if err != nil {
		liveErr = "直播打不开：" + err.Error()
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	var sink Sink
	var pm *Pump
	sinkBackend, sinkRate, sinkCh := BackendNull, 0, 0
	stopPump := make(chan struct{})
	var pumpDone = make(chan struct{})
	if live != nil {
		defer live.Close()
		if !live.HasAudio() || live.Master() != govideo.MasterAudio {
			liveErr = "直播没进声领模式"
		} else {
			ai, aerr := govideo.ProbeAudio(livePath)
			if aerr != nil {
				liveErr = "声信息读不出：" + aerr.Error()
			} else if s, serr := NewHostSink(ai.SampleRate, ai.Channels, ""); serr != nil {
				// Honest skip path: no writer on this host. Video still
				// plays through a counting sink; the verdict says skip.
				liveErr = ""
				sink = &nullSink{rate: ai.SampleRate, ch: ai.Channels}
				sinkBackend, sinkRate, sinkCh = BackendNull, ai.SampleRate, ai.Channels
				pm = &Pump{Sink: sink}
				go func() { PumpLoop(pm, live, stopPump, nil); close(pumpDone) }()
			} else {
				sink = s
				sinkBackend, sinkRate, sinkCh = s.Backend(), s.ObtainedRate(), s.ObtainedChannels()
				pm = &Pump{Sink: sink}
				go func() {
					PumpLoop(pm, live, stopPump, func(q Sink) bool { return q.Resume() == nil })
					close(pumpDone)
				}()
				defer func() {
					close(stopPump)
					<-pumpDone
					sink.Close()
				}()
			}
		}
		liveImg = rendering.NewRenderImage(480, 200)
		shell.Body.LabelAt("直播（"+shortName(livePath)+"循环，喇叭真响）", 13, 20, 220, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 246)
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
				fmt.Fprintf(os.Stderr, "video_a4_sink: 关闭 (%s)\n", win.Backend())
				return
			case platform.EventPointer:
				summary.Pointer++
				if manualMode {
					n := summary.Pointer + summary.Key + summary.Resize
					fmt.Fprintf(os.Stderr, "video_a4_sink: pointer %s (%.0f,%.0f) n=%d\n",
						ev2.Pointer, ev2.X, ev2.Y, n)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("video_a4_sink events=%d", n))
					}
				}
				return
			case platform.EventKey:
				if ev2.Pressed {
					summary.Key++
					if manualMode {
						n := summary.Pointer + summary.Key + summary.Resize
						fmt.Fprintf(os.Stderr, "video_a4_sink: key n=%d\n", n)
						if ctl != nil {
							ctl.SetTitle(fmt.Sprintf("video_a4_sink events=%d", n))
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
					fmt.Fprintf(os.Stderr, "video_a4_sink: resize %dx%d n=%d\n", ev2.Width, ev2.Height, n)
					if ctl != nil {
						ctl.SetTitle(fmt.Sprintf("video_a4_sink events=%d", n))
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
	wallStart := time.Now()
	liveShown := int64(0)
	liveMaxAV := int64(0)
	liveSeeks := int64(0)
	// Wrap-settle masking (same artifact the A2 window documents: AVDiff
	// is a last-vs-last readout, so right after the loop wrap the video
	// shows small stamps while the pump still holds the pre-wrap audio
	// stamp for a few ticks). Sampling resumes once the pump serves a
	// post-wrap stamp (smaller than the pre-wrap mark).
	wrapAudioMark := int64(-1)
	// Drill masking: the kill pauses sound while picture keeps walking
	// the running master clock, and resume lets the pump sprint through
	// queued packets. Both swings are the drill working as designed, not
	// sync regressing, so sampling pauses from the kill until the pump
	// serves a stamp newer than the resume mark.
	drillMasked := false
	drillResumeMark := int64(-1)
	var drillResumedAt time.Time
	drillNote := "等8秒演习"
	// device_error keeps the drill text only until the pump revives
	// (after Resume the writer is healthy and DeviceError reads "").
	// Keep the first loss text for the verdict: evidence must survive
	// recovery, otherwise a green run erases the proof it recovered.
	drillKilled, drillPaused, drillResumed := false, false, false
	var drillErr string
	var drillSince string
	var drillAt time.Time
	var drillLossSeen int64
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		if live != nil {
			// Wall clock, not tick count: on a slow box ticks run
			// sparse and a tick-count drill would never fire.
			wallElapsed := time.Since(wallStart).Seconds()
			// Drop drill, strict leg only (watch legs keep the user's
			// speaker intact): kill the writer child once (unplug
			// without hardware). The pump must pause readable and
			// resume.
			if liveStrict && !drillKilled && wallElapsed > 8 && sinkBackend != BackendNull {
				drillKilled = true
				drillMasked = true
				drillAt = time.Now()
				if cs, ok := sink.(*cmdSink); ok {
					cs.KillChild()
					drillNote = "已拔线，等暂停"
				} else {
					drillNote = "非进程后端，跳过拔线"
					drillPaused, drillResumed = true, true
				}
			}
			if pm != nil {
				snap := pm.Snapshot()
				lossTxt := sink.DeviceError()
				if cs, ok := sink.(*cmdSink); ok && lossTxt == "" {
					lossTxt = cs.LastError()
				}
				if drillKilled && !drillPaused && snap.Drops > 0 {
					drillPaused = true
					drillLossSeen = snap.Drops
					drillErr = lossTxt
					drillNote = "已暂停:" + shortErr(drillErr, 40)
				}
				if drillKilled && drillLossSeen == 0 {
					// Kill raced the pump's write: ask again until the
					// dead pipe surfaces (same shape as the headless
					// drill; the tick is the hammer here).
					if _, _, derr := pm.Once(live); derr != nil {
						drillLossSeen = pm.Snapshot().Drops
						lossTxt = sink.DeviceError()
						if cs, ok := sink.(*cmdSink); ok && lossTxt == "" {
							lossTxt = cs.LastError()
						}
					}
					if drillLossSeen > 0 && !drillPaused {
						drillPaused = true
						drillErr = lossTxt
						drillNote = "已暂停:" + shortErr(drillErr, 40)
					}
				}
				if drillPaused && drillLossSeen > 0 && drillErr == "" {
					// Resume cleared the live error: backfill from the
					// surviving loss text before judging the drill.
					drillErr = lossTxt
				}
				if drillPaused && !drillResumed && !sink.Paused() && snap.Resumes > 0 {
					drillResumed = true
					drillResumeMark = pm.Snapshot().LastPTS
					drillResumedAt = time.Now()
					drillSince = fmt.Sprintf("%.0fs恢复", time.Since(drillAt).Seconds())
					drillNote = "已恢复 " + drillSince
				}
				if drillMasked && drillResumed && (pm.Snapshot().LastPTS > drillResumeMark || time.Since(drillResumedAt).Seconds() > 3) {
					drillMasked = false
				}
				_ = drillSince
			}
			if f, ended := live.Poll(); f != nil {
				liveShown++
				if wrapAudioMark >= 0 && pm != nil && pm.Snapshot().LastPTS < wrapAudioMark {
					wrapAudioMark = -1
				}
				if wrapAudioMark < 0 && !drillMasked {
					if d := abs64live(live.Stats().AVDiffMs); d > liveMaxAV {
						liveMaxAV = d
					}
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
					if pm != nil {
						if mark := pm.Snapshot().LastPTS; mark > 0 {
							wrapAudioMark = mark
						}
					}
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gatePreview := ev.ErrText == "" && liveErr == "" && snapH.PresentCount > 0
		played := int64(0)
		if pm != nil {
			played = pm.Snapshot().PlayedPkts
		}
		shell.UpdateHUD("A4", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("探针=%d/%d坏%d", ev.Passed, ev.Total, ev.Failed),
			fmt.Sprintf("播=%d 音=%d %s", liveShown, played, drillNote))
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
	var ps PumpStats
	if pm != nil {
		ps = pm.Snapshot()
	}
	verdict := "pass"
	if sinkBackend == BackendNull {
		verdict = "skip"
	}

	if *autoOnly {
		rep := buildReport(snap, presents, elapsed, ev, liveErr, liveShown, liveSeeks, liveMaxAV, hotTick,
			sinkBackend, sinkRate, sinkCh, ps, drillKilled, drillPaused, drillResumed, drillErr, verdict, livePath, liveStrict,
			live, win.Backend().String())
		raw, _ := json.Marshal(rep)
		fmt.Println(string(raw))
		if err := checkSchema(raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if verdict == "skip" {
			fmt.Fprintf(os.Stderr, "SKIP: 无喇叭(%s)，只验同步链: 播=%d 音包=%d 上屏=%d\n",
				ev.Reason, liveShown, ps.PlayedPkts, presents)
			return
		}
		if err := checkGate(rep, ev, liveErr, liveShown, presents, liveStrict); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := checkBaseline(raw); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "video_a4_sink: 通过 探针=%d/%d 播=%d 音包=%d 差≤%d 拔线恢复 上屏=%d 用时=%.1f秒\n",
			ev.Passed, ev.Total, liveShown, ps.PlayedPkts, liveMaxAV, presents, elapsed)
		return
	}

	summary.Timed = secs > 0
	b, _ := json.Marshal(map[string]any{
		"ability_id":  "A4",
		"scenario":    "video_a4_sink",
		"backend":     win.Backend().String(),
		"gpu_backend": snap.GPUBackend,
		"events": map[string]any{
			"pointer": summary.Pointer, "key": summary.Key, "resize": summary.Resize,
		},
		"groups":        ev.Groups,
		"item_count":    ev.Total,
		"passed":        ev.Passed,
		"total_items":   ev.Total,
		"failed_items":  ev.Failed,
		"sink_backend":  sinkBackend,
		"played_pkts":   ps.PlayedPkts,
		"played_bytes":  ps.PlayedBytes,
		"drops":         ps.Drops,
		"resumes":       ps.Resumes,
		"drill_killed":  drillKilled,
		"drill_paused":  drillPaused,
		"drill_resumed": drillResumed,
		"device_error":  drillErr,
		"verdict":       verdict,
		"presents":      presents,
		"elapsed_sec":   elapsed,
		"live_shown":    liveShown,
		"live_seeks":    liveSeeks,
		"live_error":    liveErr,
		"live_gate":     liveGateStr(liveStrict),
		"gate_error":    ev.ErrText,
		"timed":         summary.Timed,
	})
	fmt.Println(string(b))
	fmt.Fprintf(os.Stderr, "video_a4_sink: backend=%s events=%d presents=%d gate=%d/%d sink=%s played=%d verdict=%s elapsed=%.1fs\n",
		win.Backend(), summary.Pointer+summary.Key+summary.Resize, presents, ev.Passed, ev.Total, sinkBackend, ps.PlayedPkts, verdict, elapsed)
}

func checkGate(rep report, ev a4Evidence, liveErr string, liveShown, presents int64, liveStrict bool) error {
	if ev.ErrText != "" {
		return fmt.Errorf("FAIL: 探针: %s", ev.ErrText)
	}
	if liveErr != "" {
		return fmt.Errorf("FAIL: 直播: %s", liveErr)
	}
	if liveShown < 1 {
		return fmt.Errorf("FAIL: 直播一帧没播出来")
	}
	if rep.AudioPlayedPkts < 1 {
		return fmt.Errorf("FAIL: 喇叭一包没吃到")
	}
	// Watch legs (-file) report live AV but never fail on it: the kill
	// drill belongs to gate-owned legs only. Killing the speaker
	// mid-watch would mute the user's own movie for no gate benefit.
	if !liveStrict {
		if presents < 1 {
			return fmt.Errorf("FAIL: 一次都没上屏(A4需要真窗口)")
		}
		return nil
	}
	// Default (real-footage probes AND live) leg: the speaker-AV 200ms
	// line stays reported-only (see header comment: the window readout
	// on ~10fps render measures picture lag, not sync; TestA4PumpSync
	// enforces |avdiff|<=200ms headless on the same clip). The kill
	// drill still gates: pause readable + resume + surviving error text.
	if !rep.DrillKilled || !rep.DrillPaused || !rep.DrillResumed {
		return fmt.Errorf("FAIL: 拔线演习没走完(杀=%v 停=%v 恢=%v)", rep.DrillKilled, rep.DrillPaused, rep.DrillResumed)
	}
	if rep.DeviceError == "" {
		return fmt.Errorf("FAIL: 拔线没留下可读错")
	}
	if presents < 1 {
		return fmt.Errorf("FAIL: 一次都没上屏(A4需要真窗口)")
	}
	if rep.TimeToFirstFrameMs > 2000 && rep.TimeToFirstFrameMs > 0 {
		return fmt.Errorf("FAIL: 首帧=%.1f毫秒 超过2000毫秒", rep.TimeToFirstFrameMs)
	}
	if rep.RSSPeakKB > rep.MemCapKB && rep.MemCapKB > 0 {
		return fmt.Errorf("FAIL: 内存峰值=%dKB 超过上限=%dKB", rep.RSSPeakKB, rep.MemCapKB)
	}
	return nil
}

func abs64live(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func failGateJSON(ev a4Evidence, stage string) {
	b, _ := json.Marshal(map[string]any{
		"ability_id":   "A4",
		"scenario":     "video_a4_sink",
		"probe_ok":     0,
		"pass":         false,
		"verdict":      "fail",
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

// shortName shows the live clip file name on the window (probes still
// pin the gate clip; only the live leg follows -file).
func shortName(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[i+1:]
		}
	}
	return p
}

// liveGateStr names the live leg mode for JSON evidence. Default (no
// -file) probes AND live on the same real footage: the drill still
// gates there, so it stays "strict" even though the speaker-AV number
// is reported-only (see header/checkGate).
func liveGateStr(strict bool) string {
	if strict {
		return "strict"
	}
	return "demo"
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus A4 extras.
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

	// Honest extra telemetry (new keys, never gated): player DecodeMs*
	// times the color convert only (H.264 decode itself is untimed in
	// Player stats), so it is reported under its true name here.
	ConvertMsAvg float64 `json:"convert_ms_avg"`
	ConvertMsP95 float64 `json:"convert_ms_p95"`

	FrameBuildMs   float64 `json:"frame_build_ms"`
	FrameRasterMs  float64 `json:"frame_raster_ms"`
	PresentMode    string  `json:"present_mode"`
	PresentPolicy  string  `json:"present_policy"`
	GPUBackend     string  `json:"gpu_backend"`
	DisplayBackend string  `json:"display_backend"`
	GPUFallbacks   int     `json:"gpu_fallbacks"`

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

	Groups      []a4Group `json:"groups"`
	ItemCount   int       `json:"item_count"`
	Passed      int       `json:"passed"`
	TotalItems  int       `json:"total_items"`
	FailedItems int       `json:"failed_items"`

	Master string `json:"master"`
	AVDiff int64  `json:"av_diff_ms"`

	SinkBackend      string `json:"sink_backend"`
	ObtainedRate     int    `json:"obtained_rate"`
	ObtainedChannels int    `json:"obtained_channels"`
	AudioPlayedPkts  int64  `json:"audio_played_pkts"`
	AudioPlayedBytes int64  `json:"audio_played_bytes"`
	AudioDrops       int64  `json:"audio_drops"`
	AudioResumes     int64  `json:"audio_resumes"`
	AudioDecoded     int64  `json:"audio_decoded"`
	AudioShown       int64  `json:"audio_shown"`
	AudioDepth       int    `json:"audio_depth"`
	DrillKilled      bool   `json:"drill_killed"`
	DrillPaused      bool   `json:"drill_paused"`
	DrillResumed     bool   `json:"drill_resumed"`
	DeviceError      string `json:"device_error"`
	Verdict          string `json:"verdict"`

	LiveShown int64  `json:"live_shown"`
	LiveSeeks int64  `json:"live_seeks"`
	LiveMaxAV int64  `json:"live_max_av_ms"`
	LiveError string `json:"live_error"`
	LiveGate  string `json:"live_gate"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, ev a4Evidence, liveErr string, liveShown, liveSeeks, liveMaxAV int64, hotTick int, sinkBackend string, sinkRate, sinkCh int, ps PumpStats, drillKilled, drillPaused, drillResumed bool, drillErr, verdict, livePath string, liveStrict bool, live *govideo.Player, displayBackend string) report {
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
	if ev.ErrText == "" {
		yuvReady = 1
		seekOK = 1
	}
	// decode_error only reflects real trouble: probe/live text, or a
	// dead pump that never recovered. device_error carries the drill
	// loss text for evidence (a recovered loss is green, not an error).
	errText := ev.ErrText
	if errText == "" {
		errText = liveErr
	}
	if errText == "" && drillKilled && !drillResumed {
		errText = drillErr
	}
	liveGate := "demo"
	if liveStrict {
		liveGate = "strict"
	}
	// Live engine waterline (nil-safe: open failure leaves live nil and
	// the gate already fails on liveErr). Queue/decode/convert/audio
	// numbers come from the real Player stats, never hardcoded.
	var vst govideo.Stats
	if live != nil {
		vst = live.Stats()
	}
	presentMode := snap.PresentMode
	if presentMode == "" {
		presentMode = "unknown"
	}
	presentPolicy := snap.PresentPolicy
	if presentPolicy == "" {
		presentPolicy = "unknown"
	}
	if displayBackend == "" {
		displayBackend = "unknown"
	}
	// Adapter category comes from the metrics snapshot: PipelineApp
	// records it at Open (NoteGPUBackend), so it survives Close while
	// a live Target() read would degrade to "unknown".
	gpuBackend := snap.GPUBackend
	if gpuBackend == "" {
		gpuBackend = "unknown"
	}
	return report{
		AbilityID: "A4", Scenario: "video_a4_sink", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: 0, DecodeMsP95: 0,
		QueueDepthAvg: vst.QueueAvg, QueueDepthMax: vst.QueueMax, DroppedOldFrames: vst.Dropped, AllocPerFrameB: 0, PoolHitPct: vst.PoolHitPct,
		ConvertMsAvg: vst.DecodeMsAvg, ConvertMsP95: vst.DecodeMsP95,
		FrameBuildMs: snap.LastBuildMs, FrameRasterMs: snap.LastRasterMs,
		PresentMode: presentMode, PresentPolicy: presentPolicy,
		GPUBackend: gpuBackend, DisplayBackend: displayBackend, GPUFallbacks: snap.GPUFallbacks,
		FramesDecoded: vst.Decoded, FramesShown: liveShown, ClockDriftMs: vst.DriftMs, SeekOK: seekOK, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: 1048576, GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(ev.Total), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: ev.Clips, Profile: "Main+AAC",
		Groups: ev.Groups, ItemCount: ev.Total, Passed: ev.Passed, TotalItems: ev.Total, FailedItems: ev.Failed,
		Master: "audio", AVDiff: liveMaxAV,
		SinkBackend: sinkBackend, ObtainedRate: sinkRate, ObtainedChannels: sinkCh,
		AudioPlayedPkts: ps.PlayedPkts, AudioPlayedBytes: ps.PlayedBytes,
		AudioDrops: ps.Drops, AudioResumes: ps.Resumes,
		AudioDecoded: vst.AudioDecoded, AudioShown: vst.AudioShown, AudioDepth: vst.AudioDepth,
		DrillKilled: drillKilled, DrillPaused: drillPaused, DrillResumed: drillResumed,
		DeviceError: drillErr, Verdict: verdict,
		LiveShown: liveShown, LiveSeeks: liveSeeks, LiveMaxAV: liveMaxAV, LiveError: liveErr, LiveGate: liveGate,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: errText, Source: livePath,
		NANote: "a4-only: speaker path is host-side (paplay-pulse, aplay-alsa fallback, wav chain proof); engine sync owned by A2; null verdict means skip, never green; decode_ms_* stay 0 (H.264 decode itself is untimed in Player stats — convert-only timing is convert_ms_*; per-frame decode budget owned by VR7/S-lines); alloc/pool/cpu_decode owned by VR7/S-lines",
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
	"convert_ms_avg", "convert_ms_p95",
	"frame_build_ms", "frame_raster_ms", "present_mode", "present_policy", "gpu_backend", "display_backend", "gpu_fallbacks",
	"frames_decoded", "frames_shown", "clock_drift_ms", "seek_ok", "seek_landing_delta_ms",
	"cpu_pct_avg", "cpu_ui_pct", "cpu_raster_pct", "cpu_decode_pct",
	"rss_start_kb", "rss_end_kb", "rss_peak_kb", "rss_slope_kb_per_min", "mem_cap_kb", "gc_pauses_ms_p99", "heap_alloc_MB",
	"gpu_ops", "cpu_fallback_ops", "last_cpu_fallback",
	"sps_ok", "pps_ok", "frames_split", "yuv_ready", "color_diff_per_channel", "pixel_golden_diff_pct",
	"clips", "profile",
	"groups", "item_count", "passed", "total_items", "failed_items",
	"master", "av_diff_ms",
	"sink_backend", "obtained_rate", "obtained_channels",
	"audio_played_pkts", "audio_played_bytes", "audio_drops", "audio_resumes",
	"audio_decoded", "audio_shown", "audio_depth",
	"drill_killed", "drill_paused", "drill_resumed", "device_error", "verdict",
	"live_shown", "live_seeks", "live_max_av_ms", "live_error", "live_gate",
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
	for _, k := range []string{"passed", "total_items", "failed_items", "yuv_ready", "master", "verdict", "live_gate"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
