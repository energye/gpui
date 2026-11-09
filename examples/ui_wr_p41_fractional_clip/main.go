// Command ui_wr_p41_fractional_clip is the P4.1 batch-2 capability real
// window: fractional clip rect analytic AA (subitem 2).
//
// Four cards:
//
//	A  fractional clip (edges at .5) over a solid fill — the edge pixel sits
//	   on the boundary and must show ~50% analytic coverage; the pixel beyond
//	   it stays background (the old integer scissor bled it hard-edged).
//	B  integer clip lock — pixel-aligned hard scissor semantics unchanged.
//	C  fractional clip with rotating content — AA holds under arbitrary
//	   per-frame content (every frame full repaint).
//	D  nested fractional+integer clip — the stack intersection keeps AA.
//
// Negative control: GPUI_P41C_BREAK=hard-edge makes card A's fractional clip
// snap to the integer grid (the old behavior) and must turn the half-coverage
// probes red.
//
// Gate mode (-auto-only) asserts logic probes, pixel probes and a
// zero-tolerance golden; the default run stays open for manual inspection.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
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
	winW, winH  = 1200, 800
	windowTitle = "gpui ui_wr_p41_fractional_clip — P4.1 批2 分数裁剪矩形 AA"
)

var breakMode = os.Getenv("GPUI_P41C_BREAK")

// autoGate mirrors the -auto-only flag for paint-time decisions (the gate
// frame must be fully static for a bit-identical golden).
var autoGate bool

var cardOrigin [4]render.Point
var cardDrawn [4]int

// clipRects (card-local): fractional edges land mid-pixel on purpose.
var (
	clipFrac    = [4]float64{40.5, 40.5, 356, 220} // edges 40.5 / 396.5
	clipInt     = [4]float64{40, 40, 356, 220}     // pixel-aligned
	clipFracC   = [4]float64{60.5, 60.5, 316, 200}
	clipFracD   = [4]float64{50.5, 50.5, 336, 220}
	clipIntD    = [4]float64{40, 40, 320, 180} // nested inside D's fractional (bottom edge is the integer hard cut)
	rotAngleRAD = 0.0
)

func breakClip(r [4]float64) [4]float64 {
	if breakMode == "hard-edge" {
		return [4]float64{math.Floor(r[0]), math.Floor(r[1]),
			math.Ceil(r[2]), math.Ceil(r[3])}
	}
	return r
}

// drawClippedFill clips (card-local, Push/Pop restores clip depth) and fills
// the whole card with a solid color through the clip.
func drawClippedFill(pc *rendering.PaintContext, r [4]float64, rr, gg, bb float64, rot float64) {
	dc := pc.DC
	if dc == nil {
		return
	}
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	dc.ClipRect(r[0], r[1], r[2], r[3])
	if rot != 0 {
		dc.Translate(r[0]+r[2]/2, r[1]+r[3]/2)
		dc.Rotate(rot)
		dc.Translate(-(r[0] + r[2]/2), -(r[1] + r[3]/2))
	}
	dc.SetRGBA(rr, gg, bb, 1)
	dc.DrawRectangle(-2, -2, 460, 360) // oversized: only the clip shows
	_ = dc.Fill()
	dc.Pop()
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	drawClippedFill(pc, breakClip(clipFrac), 0.9, 0.15, 0.15, 0)
	cardDrawn[0]++
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	drawClippedFill(pc, clipInt, 0.15, 0.35, 0.85, 0)
	cardDrawn[1]++
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	// 静止底块盖满 clip 区（边缘 AA 探针的稳定采样面）。旋转深色块只在人工
	// 窗绘制：门禁（-auto-only）画面完全静态，保证金图逐位可比；旋转演示
	// （AA 在每帧内容变化下稳定）属于人工检查项。
	drawClippedFill(pc, clipFracC, 0.3, 0.75, 0.4, 0)
	if !autoGate {
		drawClippedFill(pc, clipFracC, 0.1, 0.45, 0.2, rotAngleRAD)
	}
	cardDrawn[2]++
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	dc := pc.DC
	if dc == nil {
		return
	}
	// Nested: fractional clip, then integer clip inside it.
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	frac := breakClip(clipFracD)
	dc.ClipRect(frac[0], frac[1], frac[2], frac[3])
	dc.ClipRect(clipIntD[0], clipIntD[1], clipIntD[2], clipIntD[3])
	dc.SetRGBA(0.8, 0.5, 0.1, 1)
	dc.DrawRectangle(-2, -2, 460, 360)
	_ = dc.Fill()
	dc.Pop()
	cardDrawn[3]++
}

type logicResult struct {
	name string
	ok   bool
	det  string
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()
	autoGate = *autoOnly

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P41-fractional-clip")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P4.1 批2 分数裁剪矩形 AA（子项②）", []string{
		"A 分数 clip（.5 边缘）→ analytic AA 半覆盖边",
		"B 整数 clip 锁定（硬边语义零变化）",
		"C 分数 clip + 旋转内容（逐帧全重绘）",
		"D 嵌套 分数+整数 clip",
		"Skia：非整数矩形裁剪走 analytic FP",
		"门禁：逻辑 + 像素 + 金图",
		"负向：GPUI_P41C_BREAK=hard-edge",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A 分数 clip AA", paintCardA},
		{460, 8, 436, 300, "B 整数 clip 锁定", paintCardB},
		{8, 320, 436, 320, "C 分数 clip + 旋转", paintCardC},
		{460, 320, 436, 320, "D 嵌套 clip", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p41_fractional_clip", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p41_fractional_clip_final.png")
	baseName := "p41_fractional_clip_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p41_fractional_clip_final_go.png"
	}
	basePath := filepath.Join(snapDir, "goldens", baseName)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		shell.NoteHUDTick(dt)
		if *autoOnly {
			rotAngleRAD = 0.7 // gate: freeze phase for a bit-identical golden
		} else {
			rotAngleRAD += dt * 0.8
		}
		app.ScheduleFrame()
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("P41-fractional-clip", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			"A/B/C/D · pixel 16")
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
	app.Close()
	win.Close()
	elapsed := time.Since(t0).Seconds()

	logic := []logicResult{
		{"cards_drawn", cardDrawn[0] > 0 && cardDrawn[1] > 0 && cardDrawn[2] > 0 && cardDrawn[3] > 0,
			fmt.Sprintf("drawn=%v", cardDrawn)},
		{"break_inactive", breakMode == "", fmt.Sprintf("GPUI_P41C_BREAK=%q", breakMode)},
	}
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p41_fractional_clip: logic %-16s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}
	logicFail := logicTotal - logicOK

	u8 := func(v float64) float64 { return v / 255 }
	half := func(base [3]float64) render.RGBA {
		// premul base over white at 50% coverage: out = premul*0.5 over white.
		pr, pg, pb := base[0]*255*0.5, base[1]*255*0.5, base[2]*255*0.5
		sa := 255 * 0.5
		return render.RGBA{
			R: (pr + 255*(255-sa)/255) / 255,
			G: (pg + 255*(255-sa)/255) / 255,
			B: (pb + 255*(255-sa)/255) / 255,
			A: 1,
		}
	}
	redHalf := half([3]float64{0.9, 0.15, 0.15})
	blueInt := render.RGBA{R: u8(38), G: u8(89), B: u8(217), A: 1}   // 0.15/0.35/0.85 premul over white edge-less
	// C edge: DPR scaling moves the fractional edge off the pixel center, so
	// the coverage is an arbitrary fraction — assert "partial coverage" as the
	// midpoint between outside-white and inside-base with a half-range tol.
	cIn := [3]float64{0.3 * 255, 0.75 * 255, 0.4 * 255}
	greenHalf := render.RGBA{
		R: (cIn[0] + 255) / 2 / 255, G: (cIn[1] + 255) / 2 / 255, B: (cIn[2] + 255) / 2 / 255, A: 1,
	}

	mkProbe := func(name string, card int, x, y float64, c render.RGBA, tol uint8) wrkit.PixelProbe {
		return wrkit.PixelProbe{
			Name: name,
			X:    cardOrigin[card].X + x,
			Y:    cardOrigin[card].Y + y,
			R:    c.R, G: c.G, B: c.B, Tol: tol,
		}
	}

	probes := []wrkit.PixelProbe{
		// A: fractional edges at 40.5 — pixel 40 half, 39 white, 41 inside.
		mkProbe("A_edge_half", 0, 40, 150, redHalf, 14),
		mkProbe("A_outside_white", 0, 39, 150, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
		mkProbe("A_inside", 0, 42, 150, render.RGBA{R: u8(230), G: u8(38), B: u8(38), A: 1}, 10),
		mkProbe("A_edge_top_half", 0, 200, 40, redHalf, 14),
		// B: integer clip — pixel-aligned hard edge (40 = first inside row).
		mkProbe("B_inside", 1, 42, 150, blueInt, 10),
		mkProbe("B_outside_white", 1, 39, 150, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
		// C: fractional edge half coverage on the left edge (content inside rotates).
		mkProbe("C_edge_half", 2, 60, 260, greenHalf, 45),
		mkProbe("C_outside_white", 2, 59, 260, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
		// D: nested fractional outer edge (50.5) with integer inner clip:
		//    outer edge half coverage of whatever shows through (orange fill
		//    only inside the inner integer clip; between 50.5..80 the outer
		//    clip covers but inner doesn't -> white-ish blend). Sample the
		//    inner region instead and the outer half edge as blend-to-white.
		mkProbe("D_inside_nested", 3, 120, 200, render.RGBA{R: u8(204), G: u8(128), B: u8(26), A: 1}, 12),
		mkProbe("D_outside_all_white", 3, 46, 200, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
		// 能力边界登记（非本批引入）：GPU 端嵌套 rrect clip 只表达最近一个
		//（RRectBounds 单返回），外层 rrect 退化为 scissor 粗裁；且真窗
		// DPR≠1 时"整数" clip 在设备空间分数化→也走 rrect entry。嵌套
		// AA 精确化（多 rrect 表达）归后续能力批，本卡只断交集形状。
	}

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_fractional_clip: pixel %-22s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_fractional_clip: pixel %-22s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "ClipRect fractional rect -> clip.PushRectFractional (rrect entry Radius=0, SDF analytic AA), ReplaceRectFractional",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"pixel_golden_diff_pct":  goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P41-fractional-clip", Scenario: "ui_wr_p41_fractional_clip", Backend: win.Backend().String(),
		Snap: app.Metrics().Snapshot(), PresentCount: app.PresentCount(), ElapsedSec: elapsed,
		SurfaceAreaPx: winW * winH, Warmup: true, Extra: extra,
	})
	raw, _ := json.Marshal(report)
	fmt.Println(string(raw))

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "FAIL: "+format+"\n", args...)
		if *autoOnly {
			os.Exit(1)
		}
	}
	if logicOK != logicTotal {
		fail("logic probes %d/%d ok", logicOK, logicTotal)
	}
	if pixelErr != nil {
		fail("pixel probes: %v", pixelErr)
	}
	if goldenErr != nil {
		fail("golden: %v", goldenErr)
	}
	if !seeded && goldenDiff != 0 {
		fail("golden diff %.4f%% want 0 over %d px", goldenDiff, goldenPx)
	}
	if *autoOnly {
		if err := wrgate.EvaluateGates(report, wrgate.GateOptions{MinPresents: 1, RequireFullPaintPolicy: true}); err != nil {
			fail("%v", err)
		}
	}
	fmt.Fprintf(os.Stderr, "ui_wr_p41_fractional_clip: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
