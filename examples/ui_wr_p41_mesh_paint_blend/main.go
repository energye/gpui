// Command ui_wr_p41_mesh_paint_blend is the P4.1 batch-1 capability real
// window: vertex-mesh paint alpha/blend wiring (subitem 3).
//
// Four cards, each pinning one wired behavior (expected colors follow the
// Skia contract: paint alpha is applied after the primitive color blend —
// premul-linear, so vertex-level scaling is equivalent):
//
//	A  Gouraud mesh (R/G/B vertices) × solid paint alpha 0.5 over white
//	   → centroid premul(85,85,85)×0.5 over white = (170,170,170).
//	B  Gouraud mesh × Paint.BlendMode=BlendPlus over opaque blue
//	   → premul s+d clamped = (85,85,255).
//	C  Gouraud mesh × Paint.BlendMode=BlendMultiply over opaque gray
//	   → layer path (advanced): s×d premul = (43,43,85).
//	D  solid mesh (no vertex colors) × paint alpha 0.5 (regression lock)
//	   → premul(127.5,0,0,127.5) over white = (255,128,128).
//
// Negative controls: GPUI_P41_BREAK=no-alpha (paint alpha forced to 1 on the
// alpha cards) and GPUI_P41_BREAK=normal-blend (B/C forced to BlendNormal)
// must turn the pixel probes red, then revert green.
//
// Gate mode (-auto-only) asserts logic probes, pixel probes and a
// zero-tolerance golden; the default run stays open for manual inspection.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	windowTitle = "gpui ui_wr_p41_mesh_paint_blend — P4.1 批1 顶点网格 paint alpha/blend"
)

var breakMode = os.Getenv("GPUI_P41_BREAK")

var cardOrigin [4]render.Point

type cardStats struct {
	drawn     int
	degraded  bool
	errs      []string
	blendName string
	alphaVal  float64
}

var cards [4]cardStats

// cardA-D triangle (card-local): centroid weight is exactly 1/3 per vertex
// for any triangle, so the probe expectation is the analytic premul mix.
var meshTri = [3]render.Point{{X: 40, Y: 250}, {X: 400, Y: 250}, {X: 220, Y: 50}}
var triCenter = render.Point{X: 220, Y: 183}

var vcRGB = []render.RGBA{
	{R: 1, A: 1}, {G: 1, A: 1}, {B: 1, A: 1},
}

func breakBlend(mode render.BlendMode) render.BlendMode {
	if breakMode == "normal-blend" {
		return render.BlendNormal
	}
	return mode
}

func breakAlpha(a float64) float64 {
	if breakMode == "no-alpha" {
		return 1
	}
	return a
}

// drawGouraudMesh paints the shared triangle with per-vertex colors and the
// requested paint alpha/blend; returns the effective blend mode name.
func drawGouraudMesh(pc *rendering.PaintContext, alpha float64, mode render.BlendMode) string {
	if pc == nil || pc.DC == nil {
		return ""
	}
	alpha = breakAlpha(alpha)
	mode = breakBlend(mode)
	// Draw through the Canvas interface: live frames hand OnPaint a Recorder
	// (recordFullFrame → presentRecordedFrame), so LiveContext() aborts and the
	// mesh disappears (white/blue/gray backgrounds only). pc.DC is the same
	// live Context on the snapshot path and the recorder on live frames — the
	// P4.1 batch4 live-frame contract used by the other p41 windows.
	dc := pc.DC
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	dc.SetRGBA(1, 1, 1, alpha)
	dc.SetBlendMode(mode)
	dc.DrawVertices(meshTri[:], vcRGB, render.VertexModeTriangles)
	dc.SetBlendMode(render.BlendNormal)
	dc.Pop()
	cards[lastCard].blendName = mode.String()
	cards[lastCard].alphaVal = alpha
	cards[lastCard].drawn++
	return mode.String()
}

// drawSolidMesh paints the shared triangle with no vertex colors (solid paint
// alpha through the solid-copy path).
func drawSolidMesh(pc *rendering.PaintContext, alpha float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	alpha = breakAlpha(alpha)
	// Same contract as drawGouraudMesh: record through Canvas, never assert a
	// live Context inside OnPaint.
	dc := pc.DC
	dc.Push()
	dc.Translate(pc.OriginX, pc.OriginY)
	dc.SetRGBA(1, 0, 0, alpha)
	dc.DrawMesh(render.Mesh{Positions: meshTri[:]})
	dc.SetBlendMode(render.BlendNormal)
	dc.Pop()
	cards[lastCard].alphaVal = alpha
	cards[lastCard].drawn++
}

// lastCard tracks which card is painting (set by each paintCardX).
var lastCard int

// ── logic probes ──────────────────────────────────────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
}

func runLogicProbes() []logicResult {
	var out []logicResult
	add := func(name string, ok bool, det string) { out = append(out, logicResult{name, ok, det}) }
	add("mesh_shape", len(meshTri) == 3 && len(vcRGB) == 3, "3 vertices / 3 colors (true Gouraud, non-uniform)")
	add("break_inactive", breakMode == "", fmt.Sprintf("GPUI_P41_BREAK=%q", breakMode))
	return out
}

// runCardLogicProbes runs after the first painted frame: they assert the
// per-card paint state the draw helpers recorded.
func runCardLogicProbes() []logicResult {
	var out []logicResult
	add := func(name string, ok bool, det string) { out = append(out, logicResult{name, ok, det}) }
	add("card_alpha_a", cards[0].drawn > 0 && cards[0].alphaVal == 0.5, fmt.Sprintf("A alpha=%v drawn=%d", cards[0].alphaVal, cards[0].drawn))
	add("card_blend_b", cards[1].drawn > 0 && cards[1].blendName == render.BlendPlus.String(), fmt.Sprintf("B blend=%s", cards[1].blendName))
	add("card_blend_c", cards[2].drawn > 0 && cards[2].blendName == render.BlendMultiply.String(), fmt.Sprintf("C blend=%s", cards[2].blendName))
	add("card_alpha_d", cards[3].drawn > 0 && cards[3].alphaVal == 0.5, fmt.Sprintf("D alpha=%v drawn=%d", cards[3].alphaVal, cards[3].drawn))
	return out
}

// ── drawing ───────────────────────────────────────────────────────────────

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	lastCard = 0
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	cards[0] = cardStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.98, 0.98, 0.98, 1)
	drawGouraudMesh(pc, 0.5, render.BlendNormal)
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	lastCard = 1
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	cards[1] = cardStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0, 0, 1, 1)
	drawGouraudMesh(pc, 1, render.BlendPlus)
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	lastCard = 2
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	cards[2] = cardStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.502, 0.502, 0.502, 1)
	drawGouraudMesh(pc, 1, render.BlendMultiply)
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	lastCard = 3
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	cards[3] = cardStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 1, 1, 1, 1)
	drawSolidMesh(pc, 0.5)
}

// ── main ──────────────────────────────────────────────────────────────────

func main() {
	autoOnly := flag.Bool("auto-only", false, "gate mode: freeze, snapshot + probes + golden")
	manualSeconds := flag.Float64("manual-seconds", 0, "manual mode: auto-close after N seconds (0 = stay until closed)")
	flag.Parse()

	secs, secsSet := wrkit.RunSecondsOpt()
	if *autoOnly {
		secs = 6
	} else if secsSet {
		wrkit.RequireMinRun(secs, "P41-mesh-paint-blend")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: logic %-18s ok=%v %s\n", l.name, l.ok, l.det)
		if l.ok {
			logicOK++
		}
	}
	logicFail := logicTotal - logicOK

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: windowTitle,
		Decorations: true, Resizable: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL: window open (needs_gpu_window):", err)
		os.Exit(1)
	}
	host := win.Host()

	shell := wrkit.NewShell(winW, winH, "P4.1 批1 顶点网格 paint alpha/blend 接线", []string{
		"A Gouraud ×paint alpha 0.5（白底）",
		"B Gouraud ×BlendPlus（蓝底）",
		"C Gouraud ×BlendMultiply（灰底，层封装）",
		"D 实色无顶点色 ×alpha 0.5（锁定）",
		"Skia 口径：paint alpha 在 primitive blend 后乘",
		"门禁：逻辑 + 像素 + 金图",
		"负向：GPUI_P41_BREAK=no-alpha / normal-blend",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A Gouraud × alpha 0.5", paintCardA},
		{460, 8, 436, 300, "B Gouraud × BlendPlus", paintCardB},
		{8, 320, 436, 320, "C Gouraud × BlendMultiply", paintCardC},
		{460, 320, 436, 320, "D solid × alpha 0.5", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p41_mesh_paint_blend", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p41_mesh_paint_blend_final.png")
	baseName := "p41_mesh_paint_blend_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p41_mesh_paint_blend_final_go.png"
	}
	basePath := filepath.Join(snapDir, "goldens", baseName)

	app := embedder.NewPipelineApp(host, shell.Root, embedder.PipelineOptions{
		ClearR: 0.08, ClearG: 0.09, ClearB: 0.11, ClearA: 1,
		RunFor: time.Duration(secs) * time.Second, WarmUp: true, SnapshotPath: snapPath,
	})
	app.SetPresentPolicy(scheduler.PresentPolicyFullPaint)
	app.Scheduler().Tickers().Add(&tickFunc{on: func(dt float64) {
		shell.NoteHUDTick(dt)
		app.ScheduleFrame()
		snapH := app.Metrics().Snapshot()
		shell.UpdateHUD("P41-mesh-paint-blend", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			"A/B/C/D · logic 6 · pixel 8")
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

	// Card-state logic probes run after painting (they read per-card draw state).
	cardLogic := runCardLogicProbes()
	for _, l := range cardLogic {
		fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: logic %-18s ok=%v %s\n", l.name, l.ok, l.det)
		logic = append(logic, l)
		if l.ok {
			logicOK++
		} else {
			logicFail++
		}
	}
	logicTotal += len(cardLogic)

	statsOK := true
	for i := range cards {
		if cards[i].drawn == 0 || cards[i].degraded || len(cards[i].errs) > 0 {
			statsOK = false
			fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: card %d drawn=%d degraded=%v errs=%v\n",
				i, cards[i].drawn, cards[i].degraded, cards[i].errs)
		}
	}

	mkProbe := func(name string, card int, x, y float64, c render.RGBA, tol uint8) wrkit.PixelProbe {
		return wrkit.PixelProbe{
			Name: name,
			X:    cardOrigin[card].X + x,
			Y:    cardOrigin[card].Y + y,
			R:    c.R, G: c.G, B: c.B, Tol: tol,
		}
	}
	u8 := func(v float64) float64 { return v / 255 }

	probes := []wrkit.PixelProbe{
		// A: premul(85,85,85,255)×0.5 over white → (170,170,170,255).
		mkProbe("A_alpha_centroid", 0, triCenter.X, triCenter.Y,
			render.RGBA{R: u8(170), G: u8(170), B: u8(170), A: 1}, 12),
		mkProbe("A_outside_white", 0, 430, 290, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
		// B: premul s(85,85,85,255)+d(0,0,255,255) clamped → (85,85,255,255).
		mkProbe("B_plus_centroid", 1, triCenter.X, triCenter.Y,
			render.RGBA{R: u8(85), G: u8(85), B: 1, A: 1}, 12),
		mkProbe("B_outside_blue", 1, 430, 290, render.RGBA{R: 0, G: 0, B: 1, A: 1}, 6),
		// C: advanced Multiply via layer path → premul s×d over gray
		// (128,128,128,255) = (42.6,42.6,42.6,255).
		mkProbe("C_multiply_centroid", 2, triCenter.X, triCenter.Y,
			render.RGBA{R: u8(43), G: u8(43), B: u8(43), A: 1}, 12),
		mkProbe("C_outside_gray", 2, 430, 310, render.RGBA{R: u8(128), G: u8(128), B: u8(128), A: 1}, 8),
		// D: premul(127.5,0,0,127.5) over white → (255,128,128,255).
		mkProbe("D_solid_alpha", 3, triCenter.X, triCenter.Y,
			render.RGBA{R: 1, G: u8(128), B: u8(128), A: 1}, 12),
		mkProbe("D_outside_white", 3, 430, 310, render.RGBA{R: 1, G: 1, B: 1, A: 1}, 6),
	}

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: pixel %-22s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: pixel %-22s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "DrawVertices/DrawMesh paint alpha wiring, Paint.BlendMode on mesh (layer path for non-normal), compositePremulPorterDuff, PopLayer Porter-Duff materialize",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"render_stats_ok":       statsOK,
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P41-mesh-paint-blend", Scenario: "ui_wr_p41_mesh_paint_blend", Backend: win.Backend().String(),
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
	if !statsOK {
		fail("render stats: card drawn/degrade mismatch")
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
	fmt.Fprintf(os.Stderr, "ui_wr_p41_mesh_paint_blend: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
