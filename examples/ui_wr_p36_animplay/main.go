//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2024
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.6 批 3 真窗二：动图播放与消费链（ui_wr_p36_animplay，D9 第三扇）。
//
// 用 render/rendering 层原语直驱（RenderImage.SetAnimatedImage + PipelineOwner
// ticker；kit 组件的生产窗是 examples/kit/anim-image，本窗验批 1c 消费链）：
//   - 连播期间 imageCache 对账（D20 转发口）：Entries 恒 1、Creates 恒平、
//     Uploads 随帧涨、RetiredGens 归还、VramLive 不持续涨（换代即归还）；
//   - 两消费者：镜像节点 SetImageShared 同一 buf，画面跟帧不冻结（批 1c
//     「第二消费者冻结」缺陷的真窗回归面）；
//   - 图案填充采样场：动图 buf 做 ImagePattern 源，逐帧跟手。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1200.0, 800.0
	sampleDir  = "render/testdata/imgfmt"
)

var cardBG = [3]float64{0.07, 0.08, 0.10}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

var pixelResult = map[string]bool{}

// playbackDriver 是窗侧的播放驱动器（照 kit 状态机形状：Tick 返回「是否继续
// 驱动」，自持时间轴）。它是窗的摆法，不是组件实现。
type playbackDriver struct {
	anim    *render.AnimatedImage
	node    *rendering.RenderImage
	elapsed float64
	idx     int
}

func (d *playbackDriver) Tick(dt float64) bool {
	if d == nil || d.node == nil || d.anim == nil || d.anim.Disposed() {
		return false
	}
	d.elapsed += dt
	for guard := d.anim.FrameCount(); guard > 0; guard-- {
		info, err := d.anim.FrameInfo(d.idx)
		if err != nil {
			return false
		}
		if d.elapsed < info.Duration.Seconds() {
			return true
		}
		if d.idx+1 < d.anim.FrameCount() {
			if !d.node.AdvanceFrame() {
				return false
			}
			d.idx++
			d.elapsed -= info.Duration.Seconds()
			continue
		}
		// 无限循环素材：回第 0 帧继续。
		if err := d.node.SeekFrame(0); err != nil {
			return false
		}
		d.idx = 0
		d.elapsed -= info.Duration.Seconds()
	}
	return true
}

// statsTrack 是对账卡的一次全程采样轨迹（只记不可变量与单调性）。
type statsTrack struct {
	sawData         bool
	entriesMax      int
	createsMax      uint64
	uploadsFirst    uint64
	uploadsLast     uint64
	uploadsMonotone bool
	retiredLast     uint64
	watchedMax      int
	vramFirst       float64
	vramMax         float64
	vramMin         float64
	vramSamples     int
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "P36-animplay")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui p36 animplay — playback + cache ledger", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	// 金图基线按后端分文件：两后端的采样器实现不逐位一致，各自基线各自锁。
	ctl := win.Controls()

	anim, err := render.LoadAnimatedImage(filepath.Join(sampleDir, "gif_keep.gif"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: animated sample:", err)
		os.Exit(1)
	}

	// 主播放节点：接管解码器；镜像节点：SetImageShared 同一 buf（批 1c 的
	// 「一 buf 多节点」通路——第二消费者不得冻结）。
	mainNode := rendering.NewRenderImage(240, 180)
	mainNode.SetAnimatedImage(anim)
	mirrorNode := rendering.NewRenderImage(240, 180)
	mirrorNode.SetImageShared(anim.Image())
	driver := &playbackDriver{anim: anim, node: mainNode}

	// 图案填充：同一动图 buf 做 ImagePattern 源（逐帧换代 → 采样场键变 → 跟手）。
	frameW, frameH := anim.Canvas()
	pattern := render.NewImagePattern(anim.Image(), 0, 0, frameW, frameH)

	shell := wrkit.NewShell(winW, winH, "P3.6 animplay — playback, cache ledger, two consumers, pattern", []string{
		"left: playback (RenderImage + owner ticker)",
		"mid: mirror consumer (SetImageShared, same buf)",
		"right: image-pattern fill of the same animated buf",
		"bottom: cache ledger (entries/creates/uploads/retired/vram)",
		"golden mask covers picture cards only; ledger numbers move by design",
	})

	// ── 卡 A：主播放 ──
	cardA := rendering.NewRenderBox()
	cardA.FixedWidth, cardA.FixedHeight = 300, 236
	cardA.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		fillCardBG(pc, sz)
	}
	shell.Body.Place(cardA, 16, 14)
	shell.Body.Place(mainNode, 46, 40)
	shell.Body.LabelAt("A: playback (owner ticker drives frames)", 11, 16, 16, 0.86, 0.90, 0.96)

	// ── 卡 C：镜像消费者 ──
	cardC := rendering.NewRenderBox()
	cardC.FixedWidth, cardC.FixedHeight = 300, 236
	cardC.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		fillCardBG(pc, sz)
	}
	shell.Body.Place(cardC, 332, 14)
	shell.Body.Place(mirrorNode, 362, 40)
	shell.Body.LabelAt("C: mirror consumer (SetImageShared, same buf)", 11, 332, 16, 0.86, 0.90, 0.96)

	// ── 卡 D：图案填充 ──
	cardD := rendering.NewRenderBox()
	cardD.FixedWidth, cardD.FixedHeight = 252, 236
	cardD.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		fillCardBG(pc, sz)
		if pc.DC == nil {
			return
		}
		pc.DC.SetFillPattern(pattern)
		pc.DC.DrawRectangle(pc.OriginX+10, pc.OriginY+34, sz.Width-20, sz.Height-48)
		_ = pc.DC.Fill()
		pc.DC.SetFillPattern(render.NewSolidPattern(render.RGBA{R: 0.07, G: 0.08, B: 0.10, A: 1}))
	}
	shell.Body.Place(cardD, 648, 14)
	shell.Body.LabelAt("D: image-pattern of the animated buf", 11, 648, 16, 0.86, 0.90, 0.96)

	// ── 卡 B：对账（数字随帧移动，金图掩码排除本卡） ──
	var track statsTrack
	var ledgerLines []string
	cardB := rendering.NewRenderBox()
	cardB.FixedWidth, cardB.FixedHeight = 884, 132
	cardB.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		fillCardBG(pc, sz)
		for i, line := range ledgerLines {
			pc.DC.DrawString(line, pc.OriginX+14, pc.OriginY+26+float64(i)*24)
		}
	}
	shell.Body.Place(cardB, 16, 262)
	shell.Body.LabelAt("B: image-cache ledger (ImageCacheSnapshot + render.Vram*)", 11, 16, 256, 0.86, 0.90, 0.96)

	snapDir := filepath.Join("examples", "ui_wr_p36_animplay", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	frameBytes := float64(frameW * frameH * 4)
	var elapsed float64
	var frozen, started bool
	var keepMaxFrame int
	freezeAt := float64(secs) - 1.0

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, fmt.Sprintf("p36_animplay_final_%s.png", backendTag()))
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "p36 animplay: close (%s)\n", win.Backend())
			case platform.EventKey:
				if string(ev.Rune) == " " {
					if driver.idx >= 0 && elapsed > 0 {
						_ = driver.node.SeekFrame(0)
					}
				}
				if ctl != nil {
					ctl.SetTitle(fmt.Sprintf("animplay frame=%d uploads-ledger in HUD", driver.idx))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// 挂树后把播放驱动注册进本棵树的 ticker 注册表（一次）。
		if !started && mainNode.Owner() != nil {
			started = true
			mainNode.Owner().AddTicker(driver)
		}
		// 对账采样（冻结后停更，断言取冻结前的全程轨迹）。
		if !frozen {
			if st, ok := render.ImageCacheSnapshot(); ok {
				track.sawData = true
				if st.Entries > track.entriesMax {
					track.entriesMax = st.Entries
				}
				if st.Creates > track.createsMax {
					track.createsMax = st.Creates
				}
				if track.vramSamples == 0 {
					track.uploadsFirst = st.Uploads
					track.vramFirst = float64(render.VramLiveBytes())
					track.vramMin, track.vramMax = track.vramFirst, track.vramFirst
					track.uploadsMonotone = true
				} else if st.Uploads < track.uploadsLast {
					track.uploadsMonotone = false
				}
				track.uploadsLast = st.Uploads
				track.retiredLast = st.RetiredGens
				if st.Watched > track.watchedMax {
					track.watchedMax = st.Watched
				}
				v := float64(render.VramLiveBytes())
				if v > track.vramMax {
					track.vramMax = v
				}
				if v < track.vramMin {
					track.vramMin = v
				}
				track.vramSamples++
				if track.vramSamples%30 == 0 {
					ledgerLines = []string{
						fmt.Sprintf("entries=%d (want ==1: one buf, one key) creates=%d (flat: no churn)",
							track.entriesMax, track.createsMax),
						fmt.Sprintf("uploads=%d (grows per frame) retired_gens=%d watched=%d (reclaim)",
							track.uploadsLast, track.retiredLast, track.watchedMax),
						fmt.Sprintf("vram live %.1f KB (max-min %.1f KB over %d samples)",
							track.vramMax/1024, (track.vramMax-track.vramMin)/1024, track.vramSamples),
					}
				}
			}
			if f := mainNode.AnimatedFrameIndex(); f > keepMaxFrame {
				keepMaxFrame = f
			}
		}
		// 冻结帧位：auto 快照前停驱动并 Seek 回帧 0（金图确定性）。
		if secsSet && !frozen && elapsed >= freezeAt {
			frozen = true
			_ = mainNode.SeekFrame(0)
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	snap := app.Metrics().Snapshot()

	// 释放节点持有的解码器（镜像节点只共享 buf，释放归主节点）。
	mainNode.SetAnimatedImage(nil)

	// 逻辑探针（消费链对账，全部来自全程采样轨迹）。
	probes := map[string]bool{
		"ledger_seen":       track.sawData,
		"playback_advanced": keepMaxFrame >= 2,
		"entries_stable":    track.sawData && track.entriesMax == 1,
		// creates 有界（== uploads，一传一建不失控）。批 1c 登记「不接分桶池」：
		// 换号即退役销毁、新号新建是当前真实行为，池化复用后此值才会平坦——
		// 对账卡如实断言现状，不冒充未交付的优化（见 batch3-design §5）。
		"creates_bounded":  track.sawData && track.createsMax == track.uploadsLast,
		"uploads_growth":   track.sawData && track.uploadsLast >= track.uploadsFirst+15,
		"uploads_monotone": track.sawData && track.uploadsMonotone,
		"retired_reclaim":  track.sawData && track.retiredLast >= 15,
		"watched_sources":  track.sawData && track.watchedMax >= 1,
		"vram_steady":      track.sawData && track.vramSamples > 60 && (track.vramMax-track.vramMin) <= frameBytes*40,
	}
	logicOK, logicTotal := 0, len(probes)
	for name, ok := range probes {
		fmt.Fprintf(os.Stderr, "p36 animplay: logic %-18s ok=%v\n", name, ok)
		if ok {
			logicOK++
		}
	}

	// 像素探针：三张画面卡各自着墨存在；镜像卡与主卡同帧（冻结后都停帧 0）。
	snapImg := loadImage(filepath.Join(snapDir, fmt.Sprintf("p36_animplay_final_%s.png", backendTag())))
	// region = 窗口逻辑坐标（Body 原点 284,60 + 卡内摆放）。
	runPixelChecks(snapImg, []pixelCheck{
		{region: rect{330, 100, 240, 180}, desc: "main_picture"},
		{region: rect{646, 100, 240, 180}, desc: "mirror_picture"},
		{region: rect{942, 108, 226, 172}, desc: "pattern_picture"},
	}, pixelResult)
	pixelOK := 0
	for _, ok := range pixelResult {
		if ok {
			pixelOK++
		}
	}
	scriptedOK, scriptedTotal := logicOK+pixelOK, logicTotal+len(pixelResult)

	// 金图：只掩三张画面卡（对账数字随帧移动是设计内行为）。
	goldenDiff, goldenTotal, goldenSeeded, goldenErr := wrkit.GoldenGateW(
		filepath.Join(snapDir, fmt.Sprintf("p36_animplay_final_%s.png", backendTag())),
		filepath.Join(snapDir, fmt.Sprintf("p36_animplay_final_%s_base.png", backendTag())),
		winW, []wrkit.GoldenRect{
			{X: 330, Y: 54, W: 240, H: 180},
			{X: 646, Y: 54, W: 240, H: 180},
			{X: 662, Y: 302, W: 232, H: 180},
		})
	if goldenErr != nil {
		fmt.Fprintln(os.Stderr, "golden:", goldenErr)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "P36-animplay",
		Scenario:      "ui_wr_p36_animplay",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    5,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"logic_probes_ok":       logicOK,
			"logic_probes_total":    logicTotal,
			"scripted_ok":           scriptedOK,
			"scripted_total":        scriptedTotal,
			"keep_max_frame":        keepMaxFrame,
			"entries_max":           track.entriesMax,
			"creates_max":           track.createsMax,
			"uploads_last":          track.uploadsLast,
			"retired_last":          track.retiredLast,
			"watched_max":           track.watchedMax,
			"frozen_for_golden":     frozen,
			"pixel_golden_diff_pct": goldenDiff,
			"pixel_golden_total_px": goldenTotal,
			"pixel_golden_seeded":   goldenSeeded,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if !secsSet {
		fmt.Fprintf(os.Stderr, "p36 animplay: manual session done presents=%d\n", app.PresentCount())
		return
	}
	if goldenErr != nil {
		os.Exit(1)
	}
	if !goldenSeeded && goldenDiff != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: golden diff=%.4f%% over %d px\n", goldenDiff, goldenTotal)
		os.Exit(1)
	}
	if scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d (logic+pixel)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "p36 animplay: OK presents=%d scripted=%d/%d golden=%.4f%% keep_frame=%d uploads=%d\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiff, keepMaxFrame, track.uploadsLast)
}

// ── 绘制辅助（与 imgfmt 窗同形状） ──

func fillCardBG(pc *rendering.PaintContext, sz rendering.Size) {
	pc.DC.SetRGBA(cardBG[0], cardBG[1], cardBG[2], 1)
	pc.DC.DrawRectangle(pc.OriginX, pc.OriginY, sz.Width, sz.Height)
	_ = pc.DC.Fill()
}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	region rect
	desc   string
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		n := 0
		if img != nil {
			x0, y0 := int(c.region.x*dpr), int(c.region.y*dpr)
			x1, y1 := int((c.region.x+c.region.w)*dpr), int((c.region.y+c.region.h)*dpr)
			for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
				for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
					rr, gg, bb, _ := img.At(px, py).RGBA()
					r8, g8, b8 := float64(rr>>8)/255, float64(gg>>8)/255, float64(bb>>8)/255
					if diff(r8, cardBG[0]) > 0.06 || diff(g8, cardBG[1]) > 0.06 || diff(b8, cardBG[2]) > 0.06 {
						n++
					}
				}
			}
		}
		ok := n >= 150
		fmt.Fprintf(os.Stderr, "p36 animplay: pixel %-18s warm_px=%d (want >=150) ok=%v\n", c.desc, n, ok)
		result[c.desc] = ok
	}
}

func diff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks: %v\n", err)
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pixel checks decode: %v\n", err)
		return nil
	}
	return img
}

// backendTag 返回当前 GPU 后端标签（金图基线按后端分文件）。
func backendTag() string {
	be := os.Getenv("GPUI_BACKEND")
	if be == "" {
		be = "native"
	}
	return be
}
