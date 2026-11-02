//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// kit anim-image 真窗：动图组件（kit.BuildAnimatedImage）的原子验证窗。
//
// 只用组件公开 API 与控制动词（Play/Pause/SeekFrame/SetSpeed），组件行为一概
// 不在示例里实现。三张卡各摆一份循环档位不同的演示数据（无限循环 / 额外一轮 /
// 播完即停），覆盖容器的三种循环真值；键盘是控制面板（空格=全体播放/暂停，
// s=全体回到第 0 帧，x=全体 1x/2x 倍速切换）。
//
// -auto-only：按秒表跑 5s，键盘动作脚本化注入，stdout 出 JSON，门禁判定失败
// exit 1；金图在冻结帧位（全体 Pause + Seek(0)）后比对。
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
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH = 1280.0, 800.0
	// 素材正本（批 1b 网格片）：真窗运行目录是仓库根。
	sampleDir = "render/testdata/imgfmt"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	region *rect
	desc   string
}

type ticker struct{ on func(dt float64) }

func (t *ticker) Tick(dt float64) bool {
	if t.on != nil {
		t.on(dt)
	}
	return true
}

var pixelResult = map[string]bool{}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "G5-anim-image")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui kit anim-image — G5 paced playback", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	ctl := win.Controls()

	ctx := kit.DefaultScopeCtx()
	// 演示数据：三种循环档（容器真值），组件不改写循环语义。
	cards := map[string]*demoCard{}
	for _, spec := range []struct {
		key, file, title string
	}{
		{"keep", "gif_keep.gif", "loop forever (repetition<0)"},
		{"prev", "gif_restore_prev.gif", "restore-previous, 2 plays"},
		{"once", "gif_once.gif", "play once (repetition=0)"},
	} {
		anim, err := render.LoadAnimatedImage(filepath.Join(sampleDir, spec.file))
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: animated sample:", spec.file, err)
			os.Exit(1)
		}
		cards[spec.key] = newDemoCard(ctx, spec.title, anim, len(cards))
	}

	shell := wrkit.NewShell(winW, winH, "G5 anim-image — paced playback", []string{
		"left: keep (loop forever)",
		"mid: restore-previous (2 plays)",
		"right: once (single play)",
		"keys: space=play/pause s=seek0 x=speed",
		"freeze: auto-only pauses + seeks 0 before snapshot",
	})

	for _, c := range []*demoCard{cards["keep"], cards["prev"], cards["once"]} {
		shell.Body.Place(c.panel, c.slotX, 60)
	}

	snapDir := filepath.Join("examples", "kit", "anim-image", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	// 逻辑探针：运行期内 keep 卡推进过、once 卡播完自停（组件契约的可观察面）。
	keepMaxFrame, oncePlaying, onceFrame := 0, false, -1
	frozen := false
	started := false
	speed2x := false
	freezeAt := float64(secs) - 1.0

	geoValid := false
	var goldenRects []rect
	// 像素探针 = 动图着墨存在：在卡画面矩形内数暖色像素（素材帧的红块/黄条），
	// 区域计数比单点采样稳（帧内容位置固定、底色与卡底板接近）。
	probeRects := map[string]*rect{}
	resolveGeometry := func() {
		if geoValid {
			return
		}
		bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
		for key, c := range cards {
			x, y := c.panel.Offset().X, c.panel.Offset().Y
			probeRects[key] = &rect{x: bx + x, y: by + y + titleOff, w: cardW, h: cardImgH}
		}
		goldenRects = []rect{
			{x: 0, y: 0, w: winW, h: 48},
			{shell.Legend.X, shell.Legend.Y, shell.Legend.W, shell.Legend.H},
		}
		for _, c := range cards {
			x, y := c.panel.Offset().X, c.panel.Offset().Y
			goldenRects = append(goldenRects, rect{x: bx + x, y: by + y, w: cardW, h: titleOff + cardImgH + 30})
		}
		geoValid = true
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, "showcase_anim_image.png")
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose:
				fmt.Fprintf(os.Stderr, "kit anim-image: close (%s)\n", win.Backend())
			case platform.EventKey:
				// 控制面板：键盘 → 组件控制动词（不实现任何组件行为）；
				// 人工窗把收到的真控制动词打进日志并改标题显示状态。
				switch string(ev.Rune) {
				case " ":
					toggleAll(cards)
					fmt.Fprintln(os.Stderr, "kit anim-image: key space → play/pause all")
				case "s":
					seekAll(cards, 0)
					fmt.Fprintln(os.Stderr, "kit anim-image: key s → seek all to frame 0")
				case "x":
					speed2x = !speed2x
					v := 1.0
					if speed2x {
						v = 2.0
					}
					for _, c := range cards {
						c.box.Inst.SetSpeed(v)
					}
					fmt.Fprintf(os.Stderr, "kit anim-image: key x → speed %vx\n", int(v))
				}
				if ctl != nil {
					kf := cards["keep"].box.Inst
					sp := 1
					if speed2x {
						sp = 2
					}
					ctl.SetTitle(fmt.Sprintf("anim-image playing=%v keep=%d once=%d speed=%dx",
						kf.Playing(), kf.FrameIndex(), cards["once"].box.Inst.FrameIndex(), sp))
				}
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	var elapsed float64
	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
		elapsed += dt
		// 脚本化自检：启动即播放（一次性——反复 Play 会把播完自停的卡复活，
		// once 卡的末帧自停断言就永远等不到）。
		if !started && !frozen && elapsed >= 0.2 {
			started = true
			for _, c := range cards {
				c.box.Inst.Play()
			}
		}
		// 冻结后画面定格，探针读数不再更新（once 的末帧自停断言取冻结前的值）。
		if !frozen {
			if keep := cards["keep"].box.Inst.FrameIndex(); keep > keepMaxFrame {
				keepMaxFrame = keep
			}
			onceInst := cards["once"].box.Inst
			oncePlaying, onceFrame = onceInst.Playing(), onceInst.FrameIndex()
		}
		// 冻结帧位：auto-only 快照前全体 Pause + Seek(0)，金图与像素探针确定。
		if secsSet && !frozen && elapsed >= freezeAt {
			frozen = true
			for _, c := range cards {
				c.box.Inst.Pause()
				if err := c.box.Inst.SeekFrame(0); err != nil {
					fmt.Fprintln(os.Stderr, "freeze seek:", err)
				}
			}
		}
		app.ScheduleFrame()
		proc.Sample()
		shell.NoteHUDTick(dt)
		snapH := app.Metrics().Snapshot()
		gateOK := snapH.BoundarySkip >= 0 && snapH.PaintCount > 0
		shell.UpdateHUD("G5-anim-image", wrkit.PhaseSteady, app, gateOK,
			fmt.Sprintf("paint=%d presents=%d keep_frame=%d once_playing=%v",
				snapH.PaintCount, app.PresentCount(), keepMaxFrame, oncePlaying),
			fmt.Sprintf("speed2x=%v frozen=%v t=%.1f", speed2x, frozen, elapsed))
	}})
	app.Scheduler().SetMode(scheduler.ModePersistent)

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: open:", err)
		os.Exit(1)
	}
	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: run:", err)
		os.Exit(1)
	}
	proc.Stop()
	app.Close()
	win.Close()
	proc.NoteAfterClose()
	proc.Apply(app.Metrics())

	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	// 容器真值要在释放前抓（Release 后节点回到无动画态）。
	keepFrames, keepLoop := cards["keep"].box.Inst.Info()
	// 释放组件借宿主装上的解码器（资源谁建谁放）。
	for _, c := range cards {
		c.box.Inst.Release()
	}

	resolveGeometry()
	finalChecks := []pixelCheck{
		{region: probeRects["keep"], desc: "keep_card_picture"},
		{region: probeRects["prev"], desc: "prev_card_picture"},
		{region: probeRects["once"], desc: "once_card_picture"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, "showcase_anim_image.png")), finalChecks, pixelResult)
	scriptedOK, scriptedTotal := 0, len(pixelResult)
	for _, ok := range pixelResult {
		if ok {
			scriptedOK++
		}
	}
	// 金图：冻结帧位（各卡第 0 帧）后比对。
	goldenDiffPct, goldenTotalPx, goldenSeeded, goldenErr := wrkit.GoldenGateW(
		filepath.Join(snapDir, "showcase_anim_image.png"),
		filepath.Join(snapDir, "showcase_anim_image_base.png"),
		winW, toGoldenRects(goldenRects))
	if goldenErr != nil {
		fmt.Fprintln(os.Stderr, "golden:", goldenErr)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "G5-anim-image",
		Scenario:      "kit_anim_image",
		Snap:          snap,
		PresentCount:  app.PresentCount(),
		ElapsedSec:    elapsedSec,
		SurfaceAreaPx: winW * winH,
		Warmup:        true,
		Extra: map[string]any{
			"keep_frames":           keepFrames,
			"keep_loop":             keepLoop,
			"keep_max_frame":        keepMaxFrame,
			"once_final_playing":    oncePlaying,
			"once_final_frame":      onceFrame,
			"frozen_for_golden":     frozen,
			"pixel_golden_diff_pct": goldenDiffPct,
			"pixel_golden_total_px": goldenTotalPx,
			"pixel_golden_seeded":   goldenSeeded,
			"scripted_ok":           scriptedOK,
			"scripted_total":        scriptedTotal,
			"client_px":             fmt.Sprintf("%.0fx%.0f", winW, winH),
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if err := wrgate.EvaluateGates(report, wrgate.GateOptions{
		MinPresents:            1,
		RequireFullPaintPolicy: true,
		RequirePersistentFPS:   true,
		MinFPSWall:             55,
		MaxP95Ms:               22,
	}); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	if !secsSet {
		// 常驻人工窗：不带 -auto-only / -run-seconds。上面已出汇总 JSON；
		// 门禁与像素/金图断言是 auto-only 的活，人工窗以人为观察为准。
		fmt.Fprintf(os.Stderr, "kit anim-image: manual session done presents=%d keep_frame=%d once_frame=%d elapsed=%.1fs\n",
			app.PresentCount(), keepMaxFrame, onceFrame, elapsedSec)
		return
	}
	if snap.VSyncSource == "" {
		fmt.Fprintln(os.Stderr, "FAIL: vsync_source missing")
		os.Exit(1)
	}
	if keepMaxFrame < 1 {
		fmt.Fprintf(os.Stderr, "FAIL: keep card never advanced (frame max=%d)\n", keepMaxFrame)
		os.Exit(1)
	}
	if oncePlaying {
		fmt.Fprintln(os.Stderr, "FAIL: once card must stop itself after a single play")
		os.Exit(1)
	}
	if onceFrame != 1 {
		fmt.Fprintf(os.Stderr, "FAIL: once card must rest on its last frame (1), got %d\n", onceFrame)
		os.Exit(1)
	}
	if scriptedTotal != 3 || scriptedOK != scriptedTotal {
		fmt.Fprintf(os.Stderr, "FAIL: scripted=%d/%d want 3/3 (pixel probes)\n", scriptedOK, scriptedTotal)
		os.Exit(1)
	}
	if !goldenSeeded && goldenDiffPct != 0 {
		fmt.Fprintf(os.Stderr, "FAIL: pixel_golden_diff_pct=%.4f want 0 over %d px\n", goldenDiffPct, goldenTotalPx)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "kit anim-image: OK presents=%d scripted=%d/%d golden=%.4f%% keep_frame=%d elapsed=%.1fs\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiffPct, keepMaxFrame, elapsedSec)
}

const (
	titleOff  = 26.0
	cardW     = 240.0
	cardImgH  = 180.0
	cardSlotW = 60.0
)

// demoCard 是一张演示卡：卡底板 + 标题 + 组件宿主。卡与标题是窗的摆法；
// 行为全部在组件里。
type demoCard struct {
	panel *rendering.AbsoluteBox
	box   *kit.AnimatedImageBox
	slotX float64
}

func newDemoCard(ctx kit.ScopeCtx, title string, anim *render.AnimatedImage, slot int) *demoCard {
	w, h := anim.Canvas()
	scale := cardImgH / float64(h)
	imgW := float64(w) * scale
	box := kit.BuildAnimatedImage(ctx, anim, kit.DefaultAnimatedImageProps(imgW, cardImgH))

	panel := rendering.NewAbsoluteBox(cardW, titleOff+cardImgH+30)
	panel.Background = &rendering.Color{R: 0.12, G: 0.13, B: 0.16, A: 1}
	panel.SetDebugName("g5-anim-card")
	panel.SetRepaintBoundary(true)
	panel.Place(wrkit.Label(title, 11, 0.62, 0.72, 0.85), 8, 7)
	panel.Place(box.Node, (cardW-imgW)/2, titleOff)
	return &demoCard{panel: panel, box: box, slotX: cardSlotW + float64(slot)*(cardW+cardSlotW)}
}

// toggleAll / seekAll 是控制动词的扇出：示例只调组件公开 API。
func toggleAll(cards map[string]*demoCard) {
	for _, c := range cards {
		if c.box.Inst.Playing() {
			c.box.Inst.Pause()
		} else {
			c.box.Inst.Play()
		}
	}
}

func seekAll(cards map[string]*demoCard, i int) {
	for _, c := range cards {
		_ = c.box.Inst.SeekFrame(i)
	}
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		// 动图着墨存在：画面矩形内数暖色像素（素材帧的红块 R 高 G 低、黄条 R/G 高
		// B 低）；≥150 个即认定画面画上了（块面积远超阈值，空白卡为 0）。
		n := 0
		if img != nil && c.region != nil {
			x0, y0 := int(c.region.x*dpr), int(c.region.y*dpr)
			x1, y1 := int((c.region.x+c.region.w)*dpr), int((c.region.y+c.region.h)*dpr)
			for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
				for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
					r32, g32, b32, _ := img.At(px, py).RGBA()
					r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
					if (r > 0.5 && g < 0.45) || (r > 0.6 && g > 0.6 && b < 0.45) {
						n++
					}
				}
			}
		}
		ok := n >= 150
		fmt.Fprintf(os.Stderr, "kit anim-image: pixel %-20s warm_px=%d (want >=150) ok=%v\n", c.desc, n, ok)
		result[c.desc] = ok
	}
}

func toGoldenRects(rs []rect) []wrkit.GoldenRect {
	out := make([]wrkit.GoldenRect, 0, len(rs))
	for _, r := range rs {
		out = append(out, wrkit.GoldenRect{X: r.x, Y: r.y, W: r.w, H: r.h})
	}
	return out
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
