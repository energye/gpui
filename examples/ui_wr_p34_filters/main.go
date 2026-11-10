// Command ui_wr_p34_filters is the P3.4 filter-family capability real window
// (PLAN §5.1 / evidence/P3.4): every ImageFilter node plus the paint-level
// MaskFilter gets its own card, all baked through the real render.Context.
//
// Cards (4x4): blur, blurXY, dropShadow, colorMatrix / grayscale, invert,
// erode, dilate / offset, alphaThreshold, lighting, tile / bloom,
// displacement, merge, paint MaskFilter.
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=5 go run ./examples/ui_wr_p34_filters -auto-only
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
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"

	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

const (
	winW, winH   = 1400, 900
	closeSeconds = 5
	cardW        = 264.0
	cardH        = 150.0
	windowTitle  = "gpui ui_wr_p34_filters — P3.4 滤镜家族 15 节点 + MaskFilter"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var cardAt = [16][2]float64{
	{8, 52}, {280, 52}, {552, 52}, {824, 52},
	{8, 210}, {280, 210}, {552, 210}, {824, 210},
	{8, 368}, {280, 368}, {552, 368}, {824, 368},
	{8, 526}, {280, 526}, {552, 526}, {824, 526},
}

type p34Card struct {
	img          *render.ImageBuf
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once
	cards    [16]*p34Card
)

func bakeCard(w, h int, draw func(*render.Context)) *p34Card {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &p34Card{img: render.ImageBufFromImage(img), gpuOps: stats.GPUOps, cpuFallbacks: stats.CPUFallbackOps}
}

func darkBG(dc *render.Context) {
	dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
}

func clearBG4(dc *render.Context) { dc.ClearWithColor(render.RGBA{}) }

func fillRect(dc *render.Context, x, y, w, h float64) {
	dc.DrawRectangle(x, y, w, h)
	_ = dc.Fill()
}

// nodeDraws mirrors render/p34_filter_nodes_test.go one-to-one: the window and
// the unit matrix must exercise the same geometry so a green means the same
// thing on both carriers.
func nodeDraws() [16]func(*render.Context) {
	return [16]func(*render.Context){
		func(dc *render.Context) { // 0 blur
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 40, 50, 60, 60)
			dc.ApplyBlur(8)
		},
		func(dc *render.Context) { // 1 blurXY
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 40, 70, 160, 20)
			dc.ApplyBlurXY(0, 10)
		},
		func(dc *render.Context) { // 2 dropShadow
			clearBG4(dc)
			dc.SetRGBA(1, 0.2, 0.2, 1)
			fillRect(dc, 50, 40, 70, 70)
			dc.ApplyDropShadow(20, 20, 4, render.RGBA{A: 0.8})
		},
		func(dc *render.Context) { // 3 colorMatrix
			darkBG(dc)
			dc.SetRGBA(1, 0, 0, 1)
			fillRect(dc, 50, 50, 60, 60)
			m := [20]float32{
				1, 0, 0, 0, 0,
				0, 1, 0, 0, 0,
				0, 0, 1, 0, 0,
				0, 0, 0, 1, 0,
			}
			m[0], m[2] = 0, 1
			m[10], m[12] = 1, 0
			dc.ApplyColorMatrix(m)
		},
		func(dc *render.Context) { // 4 grayscale
			darkBG(dc)
			dc.SetRGBA(1, 0, 0, 1)
			fillRect(dc, 50, 50, 60, 60)
			dc.ApplyGrayscale()
		},
		func(dc *render.Context) { // 5 invert
			darkBG(dc)
			dc.SetRGBA(0, 0, 1, 1)
			fillRect(dc, 50, 50, 60, 60)
			dc.ApplyInvert()
		},
		func(dc *render.Context) { // 6 erode
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 60, 50, 60, 60)
			dc.ApplyErode(5)
		},
		func(dc *render.Context) { // 7 dilate
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 80, 50, 60, 60)
			dc.ApplyDilate(5)
		},
		func(dc *render.Context) { // 8 offset
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 40, 60, 50, 50)
			dc.ApplyOffset(120, 0)
		},
		func(dc *render.Context) { // 9 alphaThreshold
			clearBG4(dc)
			dc.SetRGBA(1, 1, 1, 0.5)
			fillRect(dc, 40, 50, 60, 60)
			dc.SetRGBA(1, 1, 1, 0.95)
			fillRect(dc, 160, 50, 60, 60)
			dc.ApplyAlphaThreshold(180)
		},
		func(dc *render.Context) { // 10 lighting
			clearBG4(dc)
			dc.SetRGBA(1, 1, 1, 1)
			dc.DrawCircle(144, 80, 45)
			_ = dc.Fill()
			dc.ApplyLighting([3]float32{-30, -40, 60}, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 1, 2)
		},
		func(dc *render.Context) { // 11 tile
			clearBG4(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 20, 20, 20, 20)
			dc.ApplyTile([4]float32{10, 10, 50, 50})
		},
		func(dc *render.Context) { // 12 bloom
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 120, 60, 48, 48)
			dc.ApplyBloom(render.BloomParams{Threshold: 0.6, Sigma: 6, Intensity: 0.8})
		},
		func(dc *render.Context) { // 13 displacement
			darkBG(dc)
			dc.SetRGBA(1, 1, 1, 1)
			fillRect(dc, 60, 60, 60, 60)
			m := image.NewRGBA(image.Rect(0, 0, int(cardW), int(cardH)))
			for i := 0; i < len(m.Pix); i += 4 {
				m.Pix[i], m.Pix[i+3] = 255, 255
			}
			dc.ApplyDisplacementMap(0, 0, 40, render.ImageBufFromImage(m))
		},
		func(dc *render.Context) { // 14 merge
			clearBG4(dc)
			dc.SetRGBA(0.2, 1, 0.4, 1)
			fillRect(dc, 30, 50, 90, 90)
			dc.ApplyMerge(
				[]render.ImageFilterNode{{Kind: render.ImageFilterOffset, IntOffsetX: 140}},
				[]render.ImageFilterNode{{Kind: render.ImageFilterInvert}},
			)
		},
		func(dc *render.Context) { // 15 paint MaskFilter
			darkBG(dc)
			dc.SetRGBA(1, 0.6, 0.2, 1)
			dc.SetPaintMaskFilter(4)
			fillRect(dc, 40, 50, 120, 100)
			dc.SetPaintMaskFilter(0)
			dc.SetRGBA(1, 0.6, 0.2, 1)
			fillRect(dc, 190, 55, 60, 80)
		},
	}
}

var nodeTitles = [16]string{
	"0 Blur 高斯模糊 r=8",
	"1 BlurXY 异向模糊 (0,10)",
	"2 DropShadow (20,20)+blur4",
	"3 ColorMatrix R/B 换道",
	"4 Grayscale 灰度",
	"5 Invert 反色",
	"6 Erode 腐蚀 r=5",
	"7 Dilate 膨胀 r=5",
	"8 Offset 整像素位移 (120,0)",
	"9 AlphaThreshold 阈值 180",
	"10 Lighting 点光源高光",
	"11 Tile 平铺（4×4 格）",
	"12 Bloom 泛光 σ6",
	"13 Displacement 位移图 +20px",
	"14 Merge 双链 over（offset+invert）",
	"15 Paint.MaskFilter 羽化 vs 硬边",
}

func ensureBaked() {
	bakeOnce.Do(func() {
		draws := nodeDraws()
		for i := range draws {
			draw := draws[i]
			cards[i] = bakeCard(int(cardW), int(cardH), draw)
		}
	})
}

func buildScene(shell *wrkit.ShellChrome) {
	for i := range cards {
		idx := i
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
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
		shell.Body.Place(box, cardAt[idx][0], cardAt[idx][1])
		shell.Body.Place(wrkit.Label(nodeTitles[idx], 10, 0.85, 0.9, 0.95), cardAt[idx][0]+8, cardAt[idx][1]+5)
	}
}

// ── probes ────────────────────────────────────────────────────────────────

type probe struct {
	key  string
	card int
	box  rect
	want func(r, g, b, a int) bool
}

func probes() []probe {
	return []probe{
		{"p34_blur_core", 0, rect{60, 70, 20, 20}, func(r, g, b, a int) bool { return r > 170 }},
		{"p34_blur_edge", 0, rect{44, 70, 8, 20}, func(r, g, b, a int) bool { return r > 60 && r < 230 }},
		{"p34_blurxy_smear", 1, rect{110, 58, 20, 8}, func(r, g, b, a int) bool { return r > 40 && r < 220 }},
		{"p34_blurxy_no_x", 1, rect{20, 72, 12, 16}, func(r, g, b, a int) bool { return r < 40 }},
		{"p34_shadow_content", 2, rect{70, 60, 20, 20}, func(r, g, b, a int) bool { return r > 150 }},
		{"p34_shadow_band", 2, rect{126, 88, 10, 14}, func(r, g, b, a int) bool { return r < 30 }},
		{"p34_matrix_swapped", 3, rect{70, 70, 20, 20}, func(r, g, b, a int) bool { return b > 150 && r < 80 }},
		{"p34_gray_equal", 4, rect{70, 70, 20, 20}, func(r, g, b, a int) bool {
			return abs(r-g) <= 4 && abs(g-b) <= 4 && r > 30 && r < 90
		}},
		{"p34_invert_yellow", 5, rect{70, 70, 20, 20}, func(r, g, b, a int) bool { return r > 200 && g > 200 && b < 70 }},
		{"p34_erode_corner", 6, rect{60, 52, 5, 6}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_erode_body", 6, rect{80, 70, 20, 20}, func(r, g, b, a int) bool { return r > 170 }},
		{"p34_dilate_grown", 7, rect{78, 52, 10, 6}, func(r, g, b, a int) bool { return r > 150 }},
		{"p34_dilate_far", 7, rect{20, 20, 12, 12}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_offset_moved", 8, rect{160, 70, 30, 20}, func(r, g, b, a int) bool { return r > 150 }},
		{"p34_offset_vacated", 8, rect{50, 70, 20, 20}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_alpha_low_zeroed", 9, rect{60, 70, 20, 20}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_alpha_high_kept", 9, rect{180, 70, 20, 20}, func(r, g, b, a int) bool { return a > 220 }},
		{"p34_light_lit_side", 10, rect{112, 62, 18, 16}, func(r, g, b, a int) bool { return r > 40 }},
		{"p34_light_dark_side", 10, rect{160, 92, 18, 16}, func(r, g, b, a int) bool { return r > 20 }},
		{"p34_tile_cell", 11, rect{104, 104, 10, 10}, func(r, g, b, a int) bool { return r > 200 }},
		{"p34_tile_gap", 11, rect{84, 84, 10, 10}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_bloom_core", 12, rect{130, 75, 20, 15}, func(r, g, b, a int) bool { return r > 200 }},
		{"p34_bloom_halo", 12, rect{106, 76, 10, 18}, func(r, g, b, a int) bool { return r > 36 }},
		{"p34_disp_shifted", 13, rect{46, 46, 10, 10}, func(r, g, b, a int) bool { return r > 150 }},
		{"p34_disp_vacated", 13, rect{112, 112, 12, 12}, func(r, g, b, a int) bool { return r < 60 }},
		{"p34_merge_offset", 14, rect{175, 70, 25, 30}, func(r, g, b, a int) bool { return r < 120 && g > 150 && b > 60 }},
		{"p34_merge_invert", 14, rect{50, 65, 40, 30}, func(r, g, b, a int) bool { return r > 130 && b > 130 }},
		{"p34_mask_soft", 15, rect{34, 90, 6, 20}, func(r, g, b, a int) bool { return r > 60 && r < 230 }},
		{"p34_mask_hard", 15, rect{184, 90, 4, 20}, func(r, g, b, a int) bool { return r < 60 }},
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode")
	flag.Parse()
	gateMode := *autoOnly
	secs, secsSet := wrkit.RunSecondsOpt()
	if !secsSet {
		secs = closeSeconds
	} else {
		wrkit.RequireMinRun(secs, "P34")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.4 滤镜家族 — 15 个 ImageFilter 节点 + Paint.MaskFilter", []string{
		"上排: Blur / BlurXY / DropShadow / ColorMatrix",
		"第二排: Grayscale / Invert / Erode / Dilate",
		"第三排: Offset / AlphaThreshold / Lighting / Tile",
		"下排: Bloom / Displacement / Merge / MaskFilter",
		"门禁: 29 个区域探针 + 金图逐位（按后端分基线）",
	})
	buildScene(shell)

	snapDir := filepath.Join("examples", "ui_wr_p34_filters", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p34_final.png")

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
	boxAvg := func(b probe) (r, g, bch, a int) {
		if img == nil {
			return 0, 0, 0, 0
		}
		x0 := int((bx + cardAt[b.card][0] + b.box.x) * dpr)
		y0 := int((by + cardAt[b.card][1] + b.box.y) * dpr)
		x1 := int((bx + cardAt[b.card][0] + b.box.x + b.box.w) * dpr)
		y1 := int((by + cardAt[b.card][1] + b.box.y + b.box.h) * dpr)
		var sr, sg, sb, sa, n int
		bounds := img.Bounds()
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if x < bounds.Min.X || x >= bounds.Max.X || y < bounds.Min.Y || y >= bounds.Max.Y {
					continue
				}
				pr, pg, pb, pa := img.At(x, y).RGBA()
				sr += int(pr >> 8)
				sg += int(pg >> 8)
				sb += int(pb >> 8)
				sa += int(pa >> 8)
				n++
			}
		}
		if n == 0 {
			return 0, 0, 0, 0
		}
		return sr / n, sg / n, sb / n, sa / n
	}

	pixelOK, pixelTotal := 0, 0
	for _, p := range probes() {
		r, g, b, a := boxAvg(p)
		ok := p.want(r, g, b, a)
		fmt.Fprintf(os.Stderr, "ui_wr_p34_filters: pixel %-22s ok=%v avg=(%d,%d,%d,%d)\n", p.key, ok, r, g, b, a)
		if ok {
			pixelOK++
		}
		pixelTotal++
	}

	goldenRects := []rect{{x: 0, y: 0, w: winW, h: 40}}
	for _, ca := range cardAt {
		goldenRects = append(goldenRects, rect{x: bx + ca[0], y: by + ca[1], w: cardW, h: cardH})
	}
	goldenDiffPct, goldenTotalPx, goldenSeeded, goldenMissing := evaluateGolden(snapPath, goldenRects, dpr)

	route := ""
	for i, c := range cards {
		if c == nil {
			continue
		}
		route += fmt.Sprintf("card%d:gpu_ops=%d,cpu_fb=%d;", i, c.gpuOps, c.cpuFallbacks)
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P34-filters", Scenario: "ui_wr_p34_filters", Backend: win.Backend().String(),
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: elapsedSec,
		SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"fps_gate": "off_correctness", "slope_gate": "off",
			"apis":            "ApplyBlur,ApplyBlurXY,ApplyDropShadow,ApplyColorMatrix,ApplyGrayscale,ApplyInvert,ApplyErode,ApplyDilate,ApplyOffset,ApplyAlphaThreshold,ApplyLighting,ApplyTile,ApplyBloom,ApplyDisplacementMap,ApplyMerge,Paint.MaskFilterRadius",
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
		fail("golden baseline missing (%s) — re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit", goldenPath(snapPath))
	} else if goldenSeeded {
		fail("golden seeded from this run — not a pass; review + commit the baseline, then re-run")
	} else if goldenDiffPct != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiffPct, goldenTotalPx)
	}
	if gateMode {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p34_filters: OK presents=%d pixel=%d/%d golden=%.4f%% gate=%v elapsed=%.1fs\n",
		app.PresentCount(), pixelOK, pixelTotal, goldenDiffPct, gateMode, elapsedSec)
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

func goldenPath(snapPath string) string {
	name := "p34_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		name = "p34_final_go.png"
	}
	return filepath.Join(filepath.Dir(snapPath), "goldens", name)
}

func evaluateGolden(snapPath string, rects []rect, dpr float64) (diffPct float64, totalPx int64, seeded bool, missing bool) {
	gp := goldenPath(snapPath)
	cur := loadImage(snapPath)
	if cur == nil {
		return 100, 0, false, true
	}
	golden := loadImage(gp)
	// fail-closed（对齐 wrkit.GoldenGate）：缺基线且没显式 GPUI_ACCEPT_GOLDEN=1 ⇒ missing=true 判红。
	if golden == nil && os.Getenv("GPUI_ACCEPT_GOLDEN") != "1" {
		return 100, 0, false, true
	}
	if golden == nil {
		_ = os.MkdirAll(filepath.Dir(gp), 0o755)
		out, err := os.Create(gp)
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
