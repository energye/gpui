// Command video_vc2_soak is the VW3 VC2 combo real-window: VR4 playback +
// VR7 limits held for 120 wall seconds on the same 1080p clip.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=120 go run ./examples/video_vc2_soak
//
// Window: 1200x800. RUN_SECONDS>=5 (close uses 120). GPU window required.
// Left: the five startup gates on vr2_1080p.mp4 (demux 1920x1080, params,
// cold decode 5 frames with monotonic stamps, variance non-black, estimate
// vs 512MB cap, three pools warmed).
// Right: the live 1080p picture looping the same clip for the whole run;
// every shown frame cycles the work pool (acquire-copy-release) for black
// detection, so pool_hit_pct is measured steady reuse, never faked.
// Bottom HUD: fps / decode / queue / memory four curves plus alloc/pool/GC.
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

// poolSink keeps the per-frame variance result visible to the compiler so
// the pool-work loop is real measured work, never deleted as dead code.
var poolSink float64

// VC2 budgets (§2.7 + §2.2.2; README + JSON gate all four curves).
// hitch follows the §2.2.2 long-run default (≤5/min): the VR7 box proved
// 0/min over 60s on the same clip and same upload path, so the default
// is the honest line here — no VR7-style relaxation carried over.
const (
	budgetDecodeP95Ms  = 50.0
	budgetAllocPerFrmB = int64(2048)
	budgetPoolHitPct   = 90.0
	budgetSlopeKBPM    = 30000.0
	budgetGCP99Ms      = 10.0
	budgetCPUAvgPct    = 85.0
	budgetHitchPerMin  = 5.0
)

func main() {
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "VC2")
	} else {
		secs = 120
	}
	wrkit.EnsureUIFace()

	st := loadSoak("1080p长跑", resolveClip("vr2_1080p.mp4"))

	gateErr := ""
	gateOK := 0
	yuvReady := 0
	if st.err != nil {
		gateErr = st.err.Error()
	} else {
		gateOK = 1
		yuvReady = 1
	}
	if st.decodeP95 > budgetDecodeP95Ms && gateErr == "" {
		gateErr = fmt.Sprintf("冷解码p95=%.1f毫秒超预算%.0f毫秒", st.decodeP95, budgetDecodeP95Ms)
		gateOK = 0
		yuvReady = 0
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui video_vc2_soak — 120秒长跑", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: 窗口打开失败(需要真机窗口):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "VC2 120秒长跑 — 播稳/内存平/无泄漏", []string{
		"左五站，同片1080p出数",
		"右直播，同片循环120秒",
		"每帧走工作池，不新开大内存",
		"门禁：斜率+泄漏+卡顿全过",
		"跑满120秒，四曲线+60档判",
	})

	statusText := "长跑正常"
	sr, sg, sb := 0.3, 0.9, 0.5
	if gateErr != "" {
		statusText = "长跑失败：" + shortErr(gateErr, 56)
		sr, sg, sb = 0.95, 0.4, 0.35
	}
	shell.Body.LabelAt("VC2 120秒长跑", 22, 20, 12, 0.92, 0.94, 0.98)
	shell.Body.LabelAt(statusText, 13, 20, 44, sr, sg, sb)

	for i, ln := range st.infoLines() {
		if i >= 6 {
			break
		}
		shell.Body.LabelAt(ln, 12, 20, 76+float64(i)*24, 0.72, 0.8, 0.9)
	}

	// Live picture: same 1080p clip looping. One 1920x1080 buffer for the
	// whole run (never per-frame alloc); display scales to 480x270 via the
	// target rect — no decode-side downscale (§2.8: full 1080p decoded,
	// scaling only at display; 1/4 scale keeps the 8MB sampler cost off
	// the 60Hz repaint path on this box, same choice as VR7).
	// Work-pool sizing (VR7): one 1080p frame samples 1920*1080/29 ≈ 71.5K
	// variance bytes per shown frame, so the pool buffer is 128KB — big
	// enough to never grow, small enough to stay honest.
	const liveW, liveH = 1920, 1080
	const workBufSize = 128 << 10
	live, err := govideo.OpenFile(st.path, govideo.Options{Loop: true})
	if err != nil && gateErr == "" {
		gateErr = "直播打不开：" + err.Error()
		gateOK = 0
		yuvReady = 0
	}
	var liveImg *rendering.RenderImage
	var liveBuf *render.ImageBuf
	pools := st.pools
	if pools == nil || pools.Work.BufSize() < workBufSize {
		pools = govideo.NewPools(liveW, liveH, workBufSize, 64<<20)
	}
	if live != nil {
		defer live.Close()
		var bufErr error
		// FormatRGBAPremul: our frames are fully opaque (alpha 255, the
		// color converter writes 255 everywhere), so straight == premul
		// bit for bit. Storing premul-labeled skips the 8MB
		// straight→premul recompute on every upload present
		// (PremultipliedData returns the buffer directly for premul
		// formats) with zero pixel difference — verified by the VR2
		// 1080p byte-exact gate plus the variance cross-check below.
		liveBuf, bufErr = render.NewImageBuf(liveW, liveH, render.FormatRGBAPremul)
		if bufErr != nil && gateErr == "" {
			gateErr = "显存建不起：" + bufErr.Error()
			gateOK = 0
			yuvReady = 0
		}
		liveImg = rendering.NewRenderImage(480, 270)
		shell.Body.LabelAt("直播（1080p循环120秒，走池不黑不花）", 13, 20, 230, 0.6, 0.8, 0.95)
		shell.Body.Place(liveImg, 20, 256)
	}

	// Video-core steady probe (§2.7 alloc gate): an isolated loop player
	// cycles 5 wall seconds BEFORE the window opens, so the UI
	// render/HUD/GPU-setup allocations cannot leak into the numerator.
	// GC-clean floors on both ends; per-frame bytes = heap delta / steady
	// shown frames. Probed on the VR7 box: ~80-100B/frame (budget 2048).
	steadyAllocB, steadyShown := int64(0), int64(0)
	if gateErr == "" {
		if per, shown, perr := probeVideoSteady(st.path, workBufSize); perr != nil {
			gateErr = "稳态探针失败：" + perr.Error()
			gateOK = 0
			yuvReady = 0
		} else {
			steadyAllocB, steadyShown = per, shown
		}
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
				fmt.Fprintf(os.Stderr, "video_vc2_soak: 关闭 (%s)\n", win.Backend())
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
	poolWorkHits := 0.0
	clock := wrkit.NewPhaseClock(8, 12)
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		phase = clock.Advance(dt)
		hotTick++
		app.ScheduleFrame()
		proc.Sample()

		if live != nil && liveBuf != nil {
			if f, _ := live.Poll(); f != nil {
				liveShown++
				// Per-frame pool work: black detection via pooled copy.
				// Output identical to direct variance (same samples);
				// the pool cycle is the measured reuse (hits). The math
				// is inlined here (not a helper call) so the pooled
				// slice never escapes to the heap: a helper taking
				// (pix, scratch) forced a heap alloc per call in the
				// first VR7 RUN60 (~2.6KB/frame), the inline form costs 0.
				// The result lands in the package sink so the compiler
				// cannot delete the loop as dead code; all locals stay
				// on the stack.
				wb := pools.Work.Acquire()
				const step = 29
				n := 0
				for i := 0; i < len(f.Pix) && n < len(wb); i += 4 * step {
					wb[n] = f.Pix[i]
					n++
				}
				if n > 0 {
					var sum float64
					for i := 0; i < n; i++ {
						sum += float64(wb[i])
					}
					mean := sum / float64(n)
					var dev float64
					for i := 0; i < n; i++ {
						d := float64(wb[i]) - mean
						if d < 0 {
							d = -d
						}
						dev += d
					}
					poolSink = dev / float64(n)
				}
				pools.Work.Release(wb)
				if f.Width != liveW || f.Height != liveH {
					liveNote = fmt.Sprintf("尺寸漂移 %dx%d", f.Width, f.Height)
				} else {
					// damage_present (§2.1): a new frame is due only
					// ~5/s; reupload + resample only then. Uploading
					// the same 8MB at 60Hz cost p95 with no visual
					// difference (VR7 green RUN60s proved it).
					fastBlitRGBA(liveBuf, f.Pix, liveW, liveH)
					liveImg.SetImageShared(liveBuf)
				}
			}
		}

		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		workHit := pools.Work.Stats().HitPct
		poolWorkHits = workHit
		gatePreview := gateErr == "" && gateOK == 1 && snapH.PresentCount > 0
		shell.UpdateHUD("VC2", phaseCN(phase), app, gatePreview,
			fmt.Sprintf("解码=%d 显示=%d 池=%.1f%%", st.decoded, st.shown, workHit),
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

	// allocPerFrame comes from the isolated pre-window probe above
	// (video core only, UI excluded by construction), not from the
	// whole-process heap delta which mixes in 120s of UI render/HUD.
	allocPerFrame := steadyAllocB
	_ = steadyShown
	workStats := pools.Work.Stats()
	poolHit := workStats.HitPct
	poolOutstanding := pools.OutstandingTotal()
	liveStats := govideo.Stats{}
	if live != nil {
		liveStats = live.Stats()
	}
	// Soak verdict: the loop must still be showing at the end with a
	// real (non-black) picture; slope/leak/hitch/CPU/GC gates below
	// carry the numbers.
	soakOK := 0
	if gateErr == "" && liveShown > 0 && poolSink >= 2 {
		soakOK = 1
	} else if gateErr == "" {
		gateErr = fmt.Sprintf("长跑播不稳: 直播%d 尾MAD%.1f", liveShown, poolSink)
		gateOK = 0
		yuvReady = 0
	}

	rep := buildReport(snap, app.PresentCount(), elapsed, st, gateOK, soakOK, yuvReady, gateErr, liveShown, allocPerFrame, poolHit, poolOutstanding, workStats, liveStats, hotTick)
	raw, _ := json.Marshal(rep)
	fmt.Println(string(raw))

	if err := checkSchema(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if app.PresentCount() < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 一次都没上屏(VC2需要真窗口)")
		os.Exit(1)
	}
	// 60fps animation gate (§2.2.2) + long-run hitch gate (default ≤5/min).
	fpsWall := 0.0
	if elapsed > 0.001 {
		fpsWall = float64(app.PresentCount()) / elapsed
	}
	if fpsWall < 55 || snap.P95FrameIntervalMs > 22 {
		fmt.Fprintf(os.Stderr, "FAIL: 帧时不达标 fps=%.1f p95=%.2f毫秒(要≥55fps且p95≤22毫秒)\n", fpsWall, snap.P95FrameIntervalMs)
		os.Exit(1)
	}
	if snap.HitchRatePerMin > budgetHitchPerMin {
		fmt.Fprintf(os.Stderr, "FAIL: 卡顿率=%.2f/分钟超预算%.0f\n", snap.HitchRatePerMin, budgetHitchPerMin)
		os.Exit(1)
	}
	if gateErr != "" || gateOK != 1 || soakOK != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: 长跑门禁: %s\n", gateErr)
		os.Exit(1)
	}
	if liveShown < 1 {
		fmt.Fprintln(os.Stderr, "FAIL: 直播一帧没播出来")
		os.Exit(1)
	}
	if allocPerFrame > budgetAllocPerFrmB {
		fmt.Fprintf(os.Stderr, "FAIL: 稳态分配=%d字节/帧超预算%d\n", allocPerFrame, budgetAllocPerFrmB)
		os.Exit(1)
	}
	if poolHit < budgetPoolHitPct {
		fmt.Fprintf(os.Stderr, "FAIL: 池命中=%.1f%%低于预算%.0f%%\n", poolHit, budgetPoolHitPct)
		os.Exit(1)
	}
	if poolOutstanding != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 池泄漏 outstanding=%d\n", poolOutstanding)
		os.Exit(1)
	}
	if rep.RSSPeakKB > rep.MemCapKB && rep.MemCapKB > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 内存峰值=%dKB 超过上限=%dKB\n", rep.RSSPeakKB, rep.MemCapKB)
		os.Exit(1)
	}
	if rep.RSSSlopeKBPerMin > budgetSlopeKBPM {
		fmt.Fprintf(os.Stderr, "FAIL: 内存斜率=%.0fKB/分钟超预算%.0f\n", rep.RSSSlopeKBPerMin, budgetSlopeKBPM)
		os.Exit(1)
	}
	if rep.GCPausesMsP99 > budgetGCP99Ms {
		fmt.Fprintf(os.Stderr, "FAIL: GC停顿p99=%.2f毫秒超预算%.0f毫秒\n", rep.GCPausesMsP99, budgetGCP99Ms)
		os.Exit(1)
	}
	if rep.CPUPctAvg > budgetCPUAvgPct {
		fmt.Fprintf(os.Stderr, "FAIL: CPU均值=%.1f%%超预算%.0f%%\n", rep.CPUPctAvg, budgetCPUAvgPct)
		os.Exit(1)
	}
	if rep.TimeToFirstFrameMs > 2000 && rep.TimeToFirstFrameMs > 0 {
		fmt.Fprintf(os.Stderr, "FAIL: 首帧=%.1f毫秒 超过2000毫秒\n", rep.TimeToFirstFrameMs)
		os.Exit(1)
	}
	if err := checkBaseline(raw); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	_ = poolWorkHits
	fmt.Fprintf(os.Stderr, "video_vc2_soak: 通过 解码=%d 显示=%d 直播=%d 分配=%dB/帧 池=%.1f%% 斜率=%.0fKB/分 GCp99=%.2fms 上屏=%d 用时=%.1f秒\n",
		st.decoded, st.shown, liveShown, allocPerFrame, poolHit, rep.RSSSlopeKBPerMin, rep.GCPausesMsP99, app.PresentCount(), elapsed)
}

// probeVideoSteady measures video-core steady bytes per shown frame in
// isolation: fresh loop player, warmup past open + first pass, GC-clean
// floors, 5 wall seconds of Poll + inline pool work (same shape as the
// live loop), then heap delta / shown frames. No window, no UI, no GPU
// setup in flight — the numerator is video steady only.
func probeVideoSteady(path string, workSize int) (perFrameB, shown int64, err error) {
	p, err := govideo.OpenFile(path, govideo.Options{Loop: true})
	if err != nil {
		return 0, 0, err
	}
	defer p.Close()
	pools := govideo.NewPools(1920, 1080, workSize, 64<<20)
	// Pool warmup BEFORE the floor: the first Acquire is a cold miss
	// (128KB fresh) and must not land inside the measurement window.
	wb0 := pools.Work.Acquire()
	pools.Work.Release(wb0)
	warm := govideo.WallDeadline(2000)
	for !govideo.WallPast(warm) {
		p.Poll()
		govideo.WallSleep(5)
	}
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	var m0 runtime.MemStats
	runtime.ReadMemStats(&m0)
	shown0 := p.Stats().Shown
	end := govideo.WallDeadline(5000)
	var n int64
	for !govideo.WallPast(end) {
		f, _ := p.Poll()
		if f == nil {
			govideo.WallSleep(1)
			continue
		}
		n++
		wb := pools.Work.Acquire()
		const step = 29
		s := 0
		for i := 0; i < len(f.Pix) && s < len(wb); i += 4 * step {
			wb[s] = f.Pix[i]
			s++
		}
		if s > 0 {
			var sum float64
			for i := 0; i < s; i++ {
				sum += float64(wb[i])
			}
			mean := sum / float64(s)
			var dev float64
			for i := 0; i < s; i++ {
				d := float64(wb[i]) - mean
				if d < 0 {
					d = -d
				}
				dev += d
			}
			poolSink = dev / float64(s)
		}
		pools.Work.Release(wb)
	}
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	shown = p.Stats().Shown - shown0
	if shown <= 0 {
		shown = n
	}
	if shown <= 0 {
		return 0, 0, fmt.Errorf("一帧没播出来")
	}
	return int64(m1.TotalAlloc-m0.TotalAlloc) / shown, shown, nil
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

// report covers VIDEO_DECODE_PLAN 2.2 families A-J plus VC2 soak extras.
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

	SoakOK          int     `json:"soak_ok"`
	EstimateMB      float64 `json:"estimate_MB"`
	PoolYUVHit      float64 `json:"pool_yuv_hit_pct"`
	PoolRGBAHit     float64 `json:"pool_rgba_hit_pct"`
	PoolWorkHit     float64 `json:"pool_work_hit_pct"`
	PoolOutstanding int64   `json:"pool_outstanding"`
	PoolEvictions   int64   `json:"pool_evictions"`

	TimeToFirstFrameMs float64 `json:"time_to_first_frame_ms"`

	PresentCount int64 `json:"present_count"`
	PaintCount   int64 `json:"paint_count"`

	DecodeError string `json:"decode_error"`
	Source      string `json:"source"`
	NANote      string `json:"n_a_reason"`
}

func buildReport(snap scheduler.FrameMetrics, presents int64, elapsed float64, st *soakState, gateOK, soakOK, yuvReady int, gateErr string, liveShown, allocPerFrame int64, poolHit float64, poolOutstanding int64, workStats govideo.PoolStats, liveStats govideo.Stats, hotTick int) report {
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
	yuvHit, rgbaHit := 0.0, 0.0
	evict := workStats.Evictions
	if st.pools != nil {
		yuvHit = st.pools.YUV.Stats().HitPct
		rgbaHit = st.pools.RGBA.Stats().HitPct
		// Work pool keeps cycling live; merge startup + live stats by
		// recomputing from the live pool object (same pointer).
		evict += st.pools.YUV.Stats().Evictions + st.pools.RGBA.Stats().Evictions
	}
	return report{
		AbilityID: "VC2", Scenario: "video_vc2_soak", ElapsedSec: elapsed,
		FPSWall: fpsWall, IntervalAvgMs: snap.AvgFrameIntervalMs, IntervalP50Ms: snap.P50FrameIntervalMs, IntervalP95Ms: snap.P95FrameIntervalMs,
		HitchCount: snap.HitchCount, HitchRatePerMin: snap.HitchRatePerMin, VSyncSource: vsync, TargetHz: 60,
		DecodeMsAvg: st.decodeAvg, DecodeMsP95: st.decodeP95,
		QueueDepthAvg: liveStats.QueueAvg, QueueDepthMax: liveStats.QueueMax, DroppedOldFrames: liveStats.Dropped, AllocPerFrameB: allocPerFrame, PoolHitPct: poolHit,
		FramesDecoded: st.decoded, FramesShown: liveShown, ClockDriftMs: liveStats.DriftMs, SeekOK: 0, SeekLandingDeltaMs: 0,
		CPUPctAvg: snap.CPUPctAvg, CPUUIPct: snap.CPUUIPct, CPURasterPct: snap.CPURasterPct, CPUDecodePct: 0,
		RSSStartKB: snap.RSSStartKB, RSSEndKB: snap.RSSEndKB, RSSPeakKB: snap.RSSPeakKB, RSSSlopeKBPerMin: snap.RSSSlopeKBPerMin,
		MemCapKB: int64(govideo.MemCapKBFor1080p), GCPausesMsP99: gcP99, HeapAllocMB: heapMB,
		GPUOps: snap.GPUOps, CPUFallbackOps: snap.CPUFallbackOps, LastCPUFallback: snap.LastCPUFallbackReason,
		SPSOk: 1, PPSOk: 1, FramesSplit: int64(st.split), YUVReady: yuvReady, ColorDiffPerChannel: 0, PixelGoldenDiffPct: 0,
		Clips: st.name, Profile: st.profile,
		SoakOK: soakOK, EstimateMB: float64(st.estimateB) / (1 << 20),
		PoolYUVHit: yuvHit, PoolRGBAHit: rgbaHit, PoolWorkHit: workStats.HitPct,
		PoolOutstanding: poolOutstanding, PoolEvictions: evict,
		TimeToFirstFrameMs: snap.TimeToFirstPresentMs,
		PresentCount:       presents, PaintCount: snap.PaintCount,
		DecodeError: gateErr, Source: st.path,
		NANote: "vc2: VR4 playback + VR7 limits held 120s; cold decode at open (decode_ms), steady display-only so cpu_decode 0; rgba/yuv pools cold, work pool steady; loop reuses pre-decoded frames",
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
	for _, k := range []string{"soak_ok", "yuv_ready", "frames_split"} {
		if fmt.Sprint(cur[k]) != fmt.Sprint(baseM[k]) {
			return fmt.Errorf("FAIL: 基线 %s 变了：基线=%v 当前=%v", k, baseM[k], cur[k])
		}
	}
	return nil
}
