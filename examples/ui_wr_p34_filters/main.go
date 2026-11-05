// Command ui_wr_p34_filters is the P3.4 filter-family capability real window
// (PLAN §5.1 / evidence/P3.4): three cards baked through the real
// render.Context — morphology duality (erode vs dilate), the merge
// orchestration (N layers over), and the paint-level MaskFilter edge
// softening.
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
	winW, winH   = 1200, 800
	closeSeconds = 5
	cardW        = 360.0
	cardH        = 200.0
	windowTitle  = "gpui ui_wr_p34_filters — P3.4 滤镜家族"
)

var clearBG = [3]float64{0.08, 0.09, 0.11}

type rect struct{ x, y, w, h float64 }

var cardAt = [][2]float64{{8, 60}, {392, 60}, {8, 290}}

type p34Card struct {
	img          *render.ImageBuf
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once
	cards    [3]*p34Card
)

func bakeCard(w, h int, draw func(*render.Context)) *p34Card {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &p34Card{img: render.ImageBufFromImage(img), gpuOps: stats.GPUOps, cpuFallbacks: stats.CPUFallbackOps}
}

func ensureBaked() {
	bakeOnce.Do(func() {
		// Card A: morphology duality — dilate grows the white block, erode
		// shrinks it (two shapes side by side in one card).
		cards[0] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
			dc.SetRGBA(1, 1, 1, 1)
			dc.DrawRectangle(20, 30, 60, 60)
			_ = dc.Fill()
			dc.ApplyDilate(3)
			dc.SetRGBA(1, 1, 1, 1)
			dc.DrawRectangle(230, 60, 60, 60)
			_ = dc.Fill()
			dc.ApplyErode(3)
		})
		// Card B: merge on a TRANSPARENT base (layers must carry alpha for
		// over-compositing to be visible): a cyan block, layer 1 = offset
		// copy (+140px), layer 2 = invert (premul invert keeps alpha, so the
		// block turns magenta and empty space stays transparent).
		cards[1] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.SetRGBA(0.2, 1, 0.4, 1)
			dc.DrawRectangle(30, 50, 90, 90)
			_ = dc.Fill()
			dc.ApplyMerge(
				[]render.ImageFilterNode{{Kind: render.ImageFilterOffset, IntOffsetX: 140}},
				[]render.ImageFilterNode{{Kind: render.ImageFilterInvert}},
			)
		})
		// Card C: paint-level MaskFilter softens a sharp rectangle's edges.
		cards[2] = bakeCard(int(cardW), int(cardH), func(dc *render.Context) {
			dc.ClearWithColor(render.RGBA{R: 0.10, G: 0.11, B: 0.14, A: 1})
			dc.SetRGBA(1, 0.6, 0.2, 1)
			dc.SetPaintMaskFilter(4)
			dc.DrawRectangle(40, 50, 120, 100)
			_ = dc.Fill()
			dc.SetPaintMaskFilter(0)
			dc.SetRGBA(1, 0.6, 0.2, 1)
			dc.DrawRectangle(230, 60, 80, 80)
			_ = dc.Fill()
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
		"A 形态学对偶：Dilate 扩张（左）/ Erode 收缩（右）",
		"B Merge 双层：offset 副本 + invert 反色 over 基面",
		"C Paint.MaskFilter 羽化（左）vs 硬边（右）",
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
		wrkit.RequireMinRun(secs, "P34")
	}

	win, err := platform.Open(platform.Options{Width: winW, Height: winH,
		Title: windowTitle, Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P3.4 滤镜家族 — 形态学/Merge/Paint 级 MaskFilter", []string{
		"A Dilate 扩张 vs Erode 收缩（对偶可见）",
		"B Merge：offset 副本+invert 反色 over 基面",
		"C Paint.MaskFilter 羽化 vs 硬边",
		"门禁: 像素区域断言 + 金图逐位",
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
	cardRect := func(i int) rect {
		return rect{x: bx + cardAt[i][0], y: by + cardAt[i][1], w: cardW, h: cardH}
	}
	// Region probes: each asserts a color signature inside the named card.
	type regionProbe struct {
		key  string
		rect rect
		want func(r, g, b int) bool
	}
	probes := []regionProbe{
		// A left: dilated white block reaches x≈17 (originally 20).
		{"p34_a_dilate_grown", rect{x: cardRect(0).x + 14, y: cardRect(0).y + 55, w: 8, h: 20}, func(r, g, b int) bool { return r > 180 && g > 180 && b > 180 }},
		// A right: eroded block corners consumed (probe the old corner — now background).
		{"p34_a_erode_shrunk", rect{x: cardRect(0).x + 230, y: cardRect(0).y + 62, w: 8, h: 8}, func(r, g, b int) bool { return r < 60 && g < 60 && b < 70 }},
		// B: the offset cyan copy exists at x+140 (cyan = layer 1 over layer 2's transparent space).
		{"p34_b_offset_copy", rect{x: cardRect(1).x + 165, y: cardRect(1).y + 90, w: 40, h: 30}, func(r, g, b int) bool { return r < 120 && g > 150 && b > 60 }},
		// B: the inverted layer turned the original block magenta in place.
		{"p34_b_inverted", rect{x: cardRect(1).x + 30, y: cardRect(1).y + 55, w: 60, h: 60}, func(r, g, b int) bool { return r > 130 && b > 130 }},
		// C left: feathered edge — intermediate alpha right at the old boundary.
		{"p34_c_soft_edge", rect{x: cardRect(2).x + 36, y: cardRect(2).y + 95, w: 8, h: 20}, func(r, g, b int) bool { return r > 90 && r < 230 }},
		// C right: hard edge stays hard (just outside is background).
		{"p34_c_hard_edge", rect{x: cardRect(2).x + 226, y: cardRect(2).y + 95, w: 6, h: 20}, func(r, g, b int) bool { return r < 60 && g < 60 && b < 70 }},
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
		fmt.Fprintf(os.Stderr, "ui_wr_p34_filters: pixel %-22s ok=%v %s\n", p.key, ok, detail)
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
		AbilityID: "P34-filters", Scenario: "ui_wr_p34_filters", Backend: win.Backend().String(),
		Snap: snap, PresentCount: app.PresentCount(), ElapsedSec: elapsedSec,
		SurfaceAreaPx: winW * winH, Warmup: true,
		Extra: map[string]any{
			"fps_gate": "off_correctness", "slope_gate": "off",
			"apis":            "ApplyErode,ApplyDilate,ApplyOffset,ApplyMerge,Paint.MaskFilterRadius",
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
		fail("golden baseline missing (goldens/p34_final.png) — re-run with GPUI_ACCEPT_GOLDEN=1 to seed, then review + commit")
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

func evaluateGolden(snapPath string, rects []rect, dpr float64) (diffPct float64, totalPx int64, seeded bool, missing bool) {
	goldenPath := filepath.Join(filepath.Dir(snapPath), "goldens", "p34_final.png")
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
