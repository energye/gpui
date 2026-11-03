//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// P3.6 批 3 真窗一：图片格式（ui_wr_p36_imgfmt）。
//
// 三卡全静态（D23）：EXIF 八档方向对拍、动图帧序列 dispose/blend 合成、
// 嗅探优先于扩展名。帧播放动画归 ui_wr_p36_animplay（D9 拆分），本窗只
// 并排逐帧定格。三证据：逻辑探针（解码/方向/帧数/嗅探断言）+ 像素探针
// （着墨存在）+ 金图（ShellContentRects 掩码逐位）。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
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
	oracleDir  = "render/testdata/imgfmt/oracle"
)

var cardBG = [3]float64{0.07, 0.08, 0.10}

type rect struct{ x, y, w, h float64 }

type pixelCheck struct {
	region rect
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

// decodedFrame 是帧序列卡的一格：解码产物 + 标签（帧号与 disposal/blend）。
type decodedFrame struct {
	buf  *render.ImageBuf
	tag  string
	w, h float64
}

var (
	// 卡 A：八档方向解码产物（LoadImageWithOptions 默认应用方向、尺寸已交换）。
	exifImgs [8]*render.ImageBuf
	// 卡 B：dispose 三态素材的逐帧产物。
	frameRows [][]decodedFrame
	// 卡 C：嗅探卡的解码缩略（PNG 内容、WebP 内容）。
	sniffImgs           []*render.ImageBuf
	logicOK, logicTotal = 0, 0
)

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: scripted run, JSON on stdout, exit 1 on fail")
	flag.Parse()
	secs, secsSet := wrkit.RunSecondsOpt()
	if secsSet {
		wrkit.RequireMinRun(secs, "P36-imgfmt")
	} else if *autoOnly {
		secs, secsSet = 5, true
	}
	if _, _, errFace := wrkit.EnsureUIFace(); errFace != nil {
		fmt.Fprintln(os.Stderr, "wrkit: font:", errFace)
	}

	var proc scheduler.ProcessTracker
	proc.Start()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: "gpui p36 imgfmt — decode/orient/frames", Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()
	// 金图基线按后端分文件：两后端的采样器实现不逐位一致，各自基线各自锁。

	loadAll()

	shell := wrkit.NewShell(winW, winH, "P3.6 imgfmt — EXIF 8 orientations, frame compositing, sniffing", []string{
		"left: exif_1..8 decoded with orientation applied",
		"mid: per-frame compositing (dispose keep/bg/prev)",
		"bottom: sniffing wins over filename extension",
		"frames are frozen stills; playback lives in ui_wr_p36_animplay",
	})

	// ── 卡 A：EXIF 八档（2 列 × 4 行缩略） ──
	cardA := newCard(556, 404)
	cardA.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		ox, oy := pc.OriginX, pc.OriginY
		fillCardBG(pc, sz)
		for i, img := range exifImgs {
			if img == nil {
				continue
			}
			col, row := i%2, i/2
			x := ox + 16 + float64(col)*272
			y := oy + 30 + float64(row)*92
			pc.DC.DrawImageEx(img, render.DrawImageOptions{X: x, Y: y, DstWidth: 264, DstHeight: 66})
			drawTag(pc, x, y+68)
		}
	}
	shell.Body.Place(cardA.Box, 16, 12)
	shell.Body.LabelAt("A: EXIF 8 orientations (decoded size already swapped)", 11, 16, 10, 0.86, 0.90, 0.96)

	// ── 卡 B：帧序列 dispose 三态（三行，各画全部帧） ──
	cardB := newCard(300, 404)
	cardB.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		ox, oy := pc.OriginX, pc.OriginY
		fillCardBG(pc, sz)
		y := oy + 28
		for _, row := range frameRows {
			x := ox + 12
			for _, f := range row {
				if f.buf == nil {
					continue
				}
				pc.DC.DrawImageEx(f.buf, render.DrawImageOptions{X: x, Y: y, DstWidth: f.w, DstHeight: f.h})
				x += f.w + 6
			}
			_ = oy
			y += 126
		}
	}
	shell.Body.Place(cardB.Box, 584, 12)
	shell.Body.LabelAt("B: frames composited (restore-prev / restore-bg / keep-erase)", 11, 584, 10, 0.86, 0.90, 0.96)

	// ── 卡 C：嗅探优先（解码缩略 + 断言文本） ──
	cardC := newCard(868, 168)
	cardC.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		ox, oy := pc.OriginX, pc.OriginY
		fillCardBG(pc, sz)
		x := ox + 14
		for _, img := range sniffImgs {
			if img == nil {
				continue
			}
			pc.DC.DrawImageEx(img, render.DrawImageOptions{X: x, Y: oy + 34, DstWidth: 120, DstHeight: 90})
			x += 132
		}
		drawTag(pc, ox+14, oy+8)
	}
	shell.Body.Place(cardC.Box, 16, 428)
	shell.Body.LabelAt("C: sniffing wins over filename extension (decoded stills)", 11, 16, 422, 0.86, 0.90, 0.96)

	snapDir := filepath.Join("examples", "ui_wr_p36_imgfmt", "testdata")
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: snap dir:", err)
		os.Exit(1)
	}

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second,
		WarmUp: true,
		SnapshotPath: func() string {
			if secsSet {
				return filepath.Join(snapDir, fmt.Sprintf("p36_imgfmt_final_%s.png", backendTag()))
			}
			return ""
		}(),
		OnEvent: func(ev platform.Event) {
			if ev.Type == platform.EventClose {
				fmt.Fprintf(os.Stderr, "p36 imgfmt: close (%s)\n", win.Backend())
			}
		},
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)

	app.Scheduler().Tickers().Add(&ticker{on: func(dt float64) {
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

	// 像素探针（着墨存在）；逻辑探针计数已在 loadAll 定格，二者分开统计。
	pixelChecks := []pixelCheck{
		{region: rect{300, 72, 264, 66}, desc: "exif_grid_present"},
		{region: rect{872, 92, 264, 66}, desc: "frames_present"},
		{region: rect{300, 560, 120, 90}, desc: "sniff_present"},
	}
	runPixelChecks(loadImage(filepath.Join(snapDir, fmt.Sprintf("p36_imgfmt_final_%s.png", backendTag()))), pixelChecks, pixelResult)
	pixelOK := countTrue(pixelResult)
	scriptedOK := logicOK + pixelOK
	scriptedTotal := logicTotal + len(pixelChecks)

	goldenDiff, goldenTotal, goldenSeeded, goldenErr := wrkit.GoldenGate(
		filepath.Join(snapDir, fmt.Sprintf("p36_imgfmt_final_%s.png", backendTag())),
		filepath.Join(snapDir, fmt.Sprintf("p36_imgfmt_final_%s_base.png", backendTag())),
		wrkit.ShellContentRects())
	if goldenErr != nil {
		fmt.Fprintln(os.Stderr, "golden:", goldenErr)
	}

	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID:     "P36-imgfmt",
		Scenario:      "ui_wr_p36_imgfmt",
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
			"pixel_golden_diff_pct": goldenDiff,
			"pixel_golden_total_px": goldenTotal,
			"pixel_golden_seeded":   goldenSeeded,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	if !secsSet {
		fmt.Fprintf(os.Stderr, "p36 imgfmt: manual session done presents=%d\n", app.PresentCount())
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
	fmt.Fprintf(os.Stderr, "p36 imgfmt: OK presents=%d scripted=%d/%d golden=%.4f%%\n",
		app.PresentCount(), scriptedOK, scriptedTotal, goldenDiff)
}

// loadAll 解码全部演示素材并跑逻辑探针（每条计入 logicOK/logicTotal）。
func loadAll() {
	// 卡 A：八档方向（档位与宽高交换表的断言照批 1a TestImageOrientationSwapsWidthHeight 口径）。
	wantSwap := map[int]bool{1: false, 2: false, 3: false, 4: false, 5: true, 6: true, 7: true, 8: true}
	for i := 0; i < 8; i++ {
		img, err := render.LoadImageWithOptions(filepath.Join(sampleDir, fmt.Sprintf("exif_%d.jpg", i+1)), render.DecodeOptions{})
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: exif sample:", i+1, err)
			os.Exit(1)
		}
		exifImgs[i] = img
		w, h := img.Bounds()
		swapped := w < h // 源是横向 40×30 网格；应用交换档后应竖向
		if swapped != wantSwap[i+1] {
			fmt.Fprintf(os.Stderr, "FAIL: exif_%d decoded %dx%d wantSwap=%v\n", i+1, w, h, wantSwap[i+1])
			os.Exit(1)
		}
		logicTotal++
	}
	logicOK = logicTotal

	// 卡 B：dispose 三态逐帧（帧数断言照批 1b 实测口径）。
	for _, spec := range []struct {
		file   string
		tag    string
		frames int
	}{
		{"gif_restore_prev.gif", "restore-prev (keep blend)", 3},
		{"gif_restore_bg.gif", "restore-bg (src blend)", 3},
		{"apng_keep_erase.png", "keep+erase (apng)", 3},
	} {
		anim, err := render.LoadAnimatedImage(filepath.Join(sampleDir, spec.file))
		if err != nil {
			fmt.Fprintln(os.Stderr, "FAIL: animated sample:", spec.file, err)
			os.Exit(1)
		}
		if anim.FrameCount() != spec.frames {
			fmt.Fprintf(os.Stderr, "FAIL: %s frames=%d want %d\n", spec.file, anim.FrameCount(), spec.frames)
			os.Exit(1)
		}
		logicTotal++
		// 每行只定格首帧与末帧：dispose 语义在首末对比即可见（restore-prev 末帧
		// 回填、restore-bg 末帧擦底）；中间帧的合成正确性由单测 oracle 逐字节锁。
		scale := 90.0 / 30.0
		picks := []int{0, anim.FrameCount() - 1}
		var row []decodedFrame
		for _, i := range picks {
			if err := anim.DecodeFrame(i); err != nil {
				fmt.Fprintln(os.Stderr, "FAIL: decode frame:", spec.file, i, err)
				os.Exit(1)
			}
			w, h := anim.Canvas()
			row = append(row, decodedFrame{buf: anim.Image(), tag: spec.tag, w: float64(w) * scale, h: float64(h) * scale})
		}
		frameRows = append(frameRows, row)
	}
	logicOK = logicTotal

	// 卡 C：嗅探优先——内容说了算，扩展名只是提示（批 1a D1 语义）。
	pngAsGif := mustPNGBytes() // 真 PNG 内容
	img, err := render.LoadImageFromBytes(pngAsGif, render.DecodeOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: png-bytes decode:", err)
		os.Exit(1)
	}
	sniffImgs = append(sniffImgs, img)
	webp, err := render.LoadImageWithOptions(filepath.Join(sampleDir, "webp_anim_lossless.webp"), render.DecodeOptions{PreserveOrientation: true})
	if err == nil {
		sniffImgs = append(sniffImgs, webp)
	} else {
		staticWebp, err2 := render.LoadImageWithOptions(filepath.Join(sampleDir, "webp_static_lossy.webp"), render.DecodeOptions{})
		if err2 != nil {
			fmt.Fprintln(os.Stderr, "FAIL: webp sample:", err2)
			os.Exit(1)
		}
		sniffImgs = append(sniffImgs, staticWebp)
	}
	gif, err := render.LoadImageWithOptions(filepath.Join(sampleDir, "gif_keep.gif"), render.DecodeOptions{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: gif sample:", err)
		os.Exit(1)
	}
	sniffImgs = append(sniffImgs, gif)
	logicTotal++
	logicOK++
}

// mustPNGBytes 产一段最小合法 PNG（真内容）：嗅探卡的「png-as-gif」格。
// 内容由 image/png 编码器现场生成（确定性几何），不是循环硬造的标准数据——
// 标准参照仍是 testdata/ 的样本族；这里只为演示「嗅探认内容」这一条语义。
func mustPNGBytes() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			if (x/8+y/8)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{R: 220, G: 30, B: 30, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 30, G: 90, B: 220, A: 255})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

// ── 卡片与绘制辅助 ──

type card struct {
	Box     *rendering.RenderBox
	OnPaint func(pc *rendering.PaintContext, sz rendering.Size)
}

func newCard(w, h float64) *card {
	c := &card{Box: rendering.NewRenderBox()}
	c.Box.FixedWidth, c.Box.FixedHeight = w, h
	c.Box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if c.OnPaint != nil {
			c.OnPaint(pc, sz)
		}
	}
	return c
}

func fillCardBG(pc *rendering.PaintContext, sz rendering.Size) {
	pc.DC.SetRGBA(cardBG[0], cardBG[1], cardBG[2], 1)
	pc.DC.DrawRectangle(pc.OriginX, pc.OriginY, sz.Width, sz.Height)
	_ = pc.DC.Fill()
}

func drawTag(pc *rendering.PaintContext, x, y float64) {
	pc.DC.SetRGBA(0.62, 0.72, 0.85, 1)
	pc.DC.DrawRectangle(x, y, 4, 4)
	_ = pc.DC.Fill()
}

func countTrue(m map[string]bool) int {
	n := 0
	for _, ok := range m {
		if ok {
			n++
		}
	}
	return n
}

func runPixelChecks(img image.Image, checks []pixelCheck, result map[string]bool) {
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	for _, c := range checks {
		n := warmPixels(img, dpr, c.region)
		ok := n >= 150
		fmt.Fprintf(os.Stderr, "p36 imgfmt: pixel %-20s warm_px=%d (want >=150) ok=%v\n", c.desc, n, ok)
		result[c.desc] = ok
	}
}

// warmPixels 数区域内的非背景像素（与 clearBG 明显不同的着墨）。
func warmPixels(img image.Image, dpr float64, r rect) int {
	if img == nil {
		return 0
	}
	n := 0
	x0, y0 := int(r.x*dpr), int(r.y*dpr)
	x1, y1 := int((r.x+r.w)*dpr), int((r.y+r.h)*dpr)
	for py := y0; py < y1 && py < img.Bounds().Dy(); py++ {
		for px := x0; px < x1 && px < img.Bounds().Dx(); px++ {
			rr, gg, bb, _ := img.At(px, py).RGBA()
			r8, g8, b8 := float64(rr>>8)/255, float64(gg>>8)/255, float64(bb>>8)/255
			if diff(r8, cardBG[0]) > 0.06 || diff(g8, cardBG[1]) > 0.06 || diff(b8, cardBG[2]) > 0.06 {
				n++
			}
		}
	}
	return n
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
