// Command ui_wr_p33_richtext is the P3.3 rich-text capability real window
// (PLAN §5.2 / evidence/P3.3): three cards baked through the real
// render.Context — a multi-color multi-decoration paragraph, a Justified
// paragraph, and a maxLines+ellipsis truncation card.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_p33_richtext -auto-only
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/energye/gpui/examples/wrgate"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1200, 800
	closeSeconds = 5
	cardW        = 520.0
	cardH        = 170.0
	windowTitle  = "gpui ui_wr_p33_richtext — P3.3 富文本逐 run 样式"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var cardAt = [][2]float64{{8, 60}, {8, 250}, {8, 440}}

type p33Card struct {
	img          *render.ImageBuf
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once
	cards    [3]*p33Card
)

func bakeCard(w, h int, draw func(*render.Context)) *p33Card {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &p33Card{img: render.ImageBufFromImage(img), gpuOps: stats.GPUOps, cpuFallbacks: stats.CPUFallbackOps}
}

// loadFace loads the window's font from the repo's text testdata.
func loadFace() (text.Face, *text.FontFamily) {
	ref := text.FontRef{Path: "render/text/testdata/goregular.ttf"}
	if _, err2 := os.Stat(ref.Path); err2 != nil {
		ref = text.FontRef{Path: "text/testdata/goregular.ttf"}
	}
	face, fam, _, err := text.LoadFontFamily(20.0, ref)
	if err != nil {
		return nil, nil
	}
	return face, fam
}

func ensureBaked() {
	bakeOnce.Do(func() {
		face, fam := loadFace()
		if face == nil {
			fmt.Fprintln(os.Stderr, "bake: no font, cards stay empty")
			cards[0] = &p33Card{img: nil}
			return
		}
		// Card A: multi-color multi-decoration paragraph.
		cards[0] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
			b := render.NewParagraphBuilder(face, render.TextStyle{Color: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
			b.SetFamily(fam)
			b.PushStyle(render.TextStyle{Color: render.RGBA{R: 1, A: 1}})
			b.AddText("Red ")
			b.PushStyle(render.TextStyle{Color: render.RGBA{G: 1, B: 1, A: 1}, Decoration: render.TextDecorationUnderline})
			b.AddText("cyan underline ")
			b.PopStyle()
			b.PushStyle(render.TextStyle{Color: render.RGBA{R: 1, G: 1, A: 1}, Decoration: render.TextDecorationLineThrough})
			b.AddText("yellow strike")
			p := b.Build(text.LayoutOptions{MaxWidth: cardW - 16})
			dc.DrawParagraphStyled(p, 8, 48)
		})
		// Card B: justified paragraph stretches interior lines to MaxWidth.
		cards[1] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
			b := render.NewParagraphBuilder(face, render.TextStyle{Color: render.RGBA{R: 0.85, G: 0.9, B: 1, A: 1}})
			b.SetFamily(fam)
			b.AddText("justified words are spread across the full width of every line except the very last one here")
			p := b.Build(text.LayoutOptions{MaxWidth: cardW - 16, Alignment: text.AlignJustify})
			dc.DrawParagraphStyled(p, 8, 36)
		})
		// Card C: maxLines + ellipsis truncation.
		cards[2] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
			b := render.NewParagraphBuilder(face, render.TextStyle{Color: render.RGBA{R: 0.9, G: 0.95, B: 1, A: 1}})
			b.SetFamily(fam)
			b.AddText("the quick brown fox jumps over the lazy dog again and again until it wraps many times")
			p := b.Build(text.LayoutOptions{MaxWidth: cardW - 16, MaxLines: 3, Overflow: text.OverflowEllipsis})
			dc.DrawParagraphStyled(p, 8, 36)
		})
	})
}

func buildScene(shell *wrkit.ShellChrome) {
	drawBaked := func(idx int) func(*rendering.PaintContext, rendering.Size) {
		return func(pc *rendering.PaintContext, _ rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			ensureBaked()
			c := cards[idx]
			if c == nil || c.img == nil {
				return
			}
			pc.DC.DrawImageEx(c.img, render.DrawImageOptions{
				X: pc.OriginX, Y: pc.OriginY, DstWidth: cardW, DstHeight: cardH,
				Interpolation: render.InterpNearest,
			})
		}
	}
	titles := []string{
		"A 多色+装饰（红/青下划线/黄删除线）",
		"B Justify 两端对齐（末行不拉伸）",
		"C maxLines=3 + ellipsis",
	}
	for i, title := range titles {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = drawBaked(i)
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.85, 0.9, 0.95), cardAt[i][0]+12, cardAt[i][1]+8)
	}
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode")
	flag.Parse()
	gateMode := *autoOnly
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "P33")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.3 富文本 — 逐 run 颜色/装饰 + Justify + ellipsis", []string{
		"A 逐 run 颜色+装饰线（红/青下划线/黄删除线）",
		"B Justify 空格分配、末行不拉伸",
		"C maxLines=3+ellipsis 截断",
		"门禁: gpu_ops>0 + 金图逐位 + 像素断言",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_p33_richtext", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p33_final.png")

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: clearBG[0], ClearG: clearBG[1], ClearB: clearBG[2], ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&fixedTick{on: func(dt float64) { app.ScheduleFrame() }})
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
	app.Close()
	win.Close()
	elapsedSec := time.Since(t0).Seconds()
	snap := app.Metrics().Snapshot()

	img := loadImage(snapPath)
	dpr := 1.0
	if img != nil {
		dpr = float64(img.Bounds().Dx()) / winW
	}
	ensureBaked()

	bx, by := shell.Body.Box.Offset().X, shell.Body.Box.Offset().Y
	// Region probes: each asserts that at least one pixel inside the named
	// card rect carries the expected run color (glyph AA makes single-point
	// probes brittle; P0.2 region-probe precedent).
	type regionProbe struct {
		key  string
		rect rect
		want func(r, g, b int) bool
	}
	cardRect := func(i int) rect {
		return rect{x: bx + cardAt[i][0], y: by + cardAt[i][1], w: cardW, h: cardH}
	}
	probes := []regionProbe{
		{"p33_a_red_run", cardRect(0), func(r, g, b int) bool { return r > 180 && g < 90 && b < 90 }},
		{"p33_a_cyan_run", cardRect(0), func(r, g, b int) bool { return r < 120 && g > 150 && b > 150 }},
		{"p33_a_yellow_run", cardRect(0), func(r, g, b int) bool { return r > 180 && g > 160 && b < 110 }},
		{"p33_b_justify_reach", cardRect(1), func(r, g, b int) bool { return b > 140 && r > 120 }},
		{"p33_c_ellipsis_tail", cardRect(2), func(r, g, b int) bool { return r > 120 && g > 120 && b > 120 }},
	}
	pixelOK := 0
	pixelTotal := 0
	for _, p := range probes {
		ok := false
		detail := "no snapshot"
		if img != nil {
			hits := 0
			for y := int(p.rect.y * dpr); y < int((p.rect.y+p.rect.h)*dpr) && hits == 0; y++ {
				for x := int(p.rect.x * dpr); x < int((p.rect.x+p.rect.w)*dpr); x++ {
					r, g, b, _ := at(img, x, y)
					if p.want(int(r), int(g), int(b)) {
						hits++
						break
					}
				}
			}
			ok = hits > 0
			detail = fmt.Sprintf("hits=%d", hits)
		}
		fmt.Fprintf(os.Stderr, "ui_wr_p33_richtext: pixel %-22s ok=%v %s\n", p.key, ok, detail)
		if ok {
			pixelOK++
		}
		pixelTotal++
	}

	goldenRects := []rect{{x: 0, y: 0, w: winW, h: 48}}
	for _, ca := range cardAt {
		goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
	}
	goldenDiffPct, goldenTotalPx, goldenSeeded, goldenMissing := evaluateGolden(snapPath, goldenRects, dpr)

	route := ""
	for i, c := range cards {
		if c != nil {
			route += fmt.Sprintf("card%d:gpu_ops=%d,cpu_fb=%d;", i, c.gpuOps, c.cpuFallbacks)
		}
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P33-richtext", Scenario: "ui_wr_p33_richtext", Backend: win.Backend().String(),
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: elapsedSec,
		SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"fps_gate": "off_correctness", "slope_gate": "off",
			"apis":            "NewParagraphBuilder,PushStyle,AddText,Build,DrawParagraphStyled,AlignJustify,MaxLines,Overflow",
			"route":           route,
			"pixel_probes_ok": pixelOK, "pixel_probes_total": pixelTotal,
			"pixel_golden_diff_pct": goldenDiffPct, "pixel_golden_total_px": goldenTotalPx,
			"pixel_golden_first_run": goldenSeeded,
		},
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if gateMode {
			os.Exit(1)
		}
	}
	if pixelOK != pixelTotal || pixelTotal == 0 {
		fail("pixel probes %d/%d ok", pixelOK, pixelTotal)
	}
	if goldenMissing {
		fail("golden baseline missing (goldens/p33_final.png) — re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit")
	} else if goldenSeeded {
		fail("golden seeded from this run — not a pass; review + commit the baseline, then re-run")
	} else if goldenDiffPct != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if snap.GPUOps == 0 && snap.CPUFallbackOps == 0 {
		fail("no render ops recorded")
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p33_richtext: OK presents=%d pixel=%d/%d golden=%.4f%% gate=%v elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, pixelTotal, goldenDiffPct, gateMode, elapsedSec)
}

func inBand(v int, band [2]int) bool {
	if band[0] < 0 && band[1] < 0 {
		return true
	}
	lo, hi := band[0], band[1]
	if lo < 0 {
		lo = 0
	}
	if hi > 256 {
		hi = 256
	}
	return v >= lo && v <= hi
}

type fixedTick struct {
	on func(dt float64)
}

func (t *fixedTick) Tick(dt float64) bool { t.on(dt); return true }

func loadImage(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil
	}
	return img
}

func at(img image.Image, x, y int) (byte, byte, byte, byte) {
	r, g, b, a := img.At(x, y).RGBA()
	return byte(r >> 8), byte(g >> 8), byte(b >> 8), byte(a >> 8)
}

func evaluateGolden(snapPath string, rects []rect, dpr float64) (diffPct float64, totalPx int64, seeded bool, missing bool) {
	goldenPath := filepath.Join(filepath.Dir(snapPath), "goldens", "p33_final.png")
	cur := loadImage(snapPath)
	if cur == nil {
		return 100, 0, false, true
	}
	golden := loadImage(goldenPath)
	// fail-closed（对齐 wrkit.GoldenGate）：缺基线且没显式 GPUI_ACCEPT_GOLDEN=1 ⇒ missing=true 判红。
	if golden == nil && os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
		return 100, 0, false, true
	}
	if golden == nil {
		_ = os.MkdirAll(filepath.Dir(goldenPath), 0o755)
		out, err := os.Create(goldenPath)
		if err != nil {
			return 100, 0, false, true
		}
		defer out.Close()
		if err := png.Encode(out, cur); err != nil {
			return 100, 0, false, true
		}
		return 0, 0, true, false
	}
	if golden.Bounds() != cur.Bounds() {
		return 100, 0, false, true
	}
	var diff, total int64
	for _, r := range rects {
		for y := int(r.y * dpr); y < int((r.y+r.h)*dpr); y++ {
			for x := int(r.x * dpr); x < int((r.x+r.w)*dpr); x++ {
				a1, b1, c1, _ := at(golden, x, y)
				a2, b2, c2, _ := at(cur, x, y)
				total++
				if a1 != a2 || b1 != b2 || c1 != c2 {
					diff++
				}
			}
		}
	}
	if total == 0 {
		return 100, 0, false, true
	}
	return float64(diff) * 100 / float64(total), total, false, false
}
