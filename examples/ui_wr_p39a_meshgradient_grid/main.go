// Command ui_wr_p39a_meshgradient_grid is the P3.9a capability real window:
// high-level mesh gradient grids (rows×columns + per-vertex tangents + color
// model), built on the P3.9 Coons patch kernel.
//
// Four cards:
//
//	A  1×1 grid: Bilinear vs Catmull-Rom color.
//	B  2×2 grid: automatic C1 tangents.
//	C  3×3 grid: automatic tangents + one explicit zero (sharp) center tangent.
//	D  4×4 grid: Catmull-Rom color over 25 vertices.
//
// The grid is tessellated by render into one indexed Mesh; this window only
// builds grid data through the public render API. Gate mode asserts logic
// probes, semantic pixel probes and a zero-tolerance golden.
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
	windowTitle = "gpui ui_wr_p39a_meshgradient_grid — P3.9a 高层 mesh gradient（切向量/颜色模型）"
)

var breakMode = os.Getenv("GPUI_P39A_BREAK")

var cardOrigin [4]render.Point

type patchStats struct {
	triangles int
	degraded  bool
	errs      []string
}

var cardStats [4]patchStats

func hueColor(h float64) render.RGBA {
	h = math.Mod(h, 1)
	if h < 0 {
		h++
	}
	s, v := 0.82, 1.0
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h*6, 2)-1))
	m := v - c
	var r, g, b float64
	switch int(h*6) % 6 {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return render.RGBA{R: r + m, G: g + m, B: b + m, A: 1}
}

func makeGrid(rows, cols int, x0, y0, dx, dy, wave float64, hueBase float64) ([]render.MeshGradientVertex, []render.RGBA) {
	vs := make([]render.MeshGradientVertex, 0, (rows+1)*(cols+1))
	colors := make([]render.RGBA, 0, (rows+1)*(cols+1))
	for r := 0; r <= rows; r++ {
		for c := 0; c <= cols; c++ {
			x := x0 + float64(c)*dx + wave*math.Cos(float64(r)*1.1+float64(c)*0.7)
			y := y0 + float64(r)*dy + wave*math.Sin(float64(c)*1.3+float64(r)*0.9)
			col := hueColor(hueBase + float64(r*(cols+1)+c)/float64((rows+1)*(cols+1)))
			vs = append(vs, render.MeshGradientVertex{Pos: render.Pt(x, y), Color: col})
			colors = append(colors, col)
		}
	}
	return vs, colors
}

func mustGrid(rows, cols int, vs []render.MeshGradientVertex, model render.MeshGradientColorModel) *render.MeshGradient {
	g, err := render.NewMeshGradient(rows, cols, vs)
	if err != nil {
		panic(err)
	}
	g.ColorModel = model
	return g
}

type sceneData struct {
	aBilinear, aCatmull, b, c, d *render.MeshGradient
}

func buildScene() sceneData {
	// A: 1x1, four corners.
	aPoints := []render.Point{render.Pt(20, 45), render.Pt(200, 45), render.Pt(20, 270), render.Pt(200, 270)}
	aColors := []render.RGBA{
		{R: 0.95, G: 0.15, B: 0.12, A: 1},
		{R: 0.15, G: 0.85, B: 0.30, A: 1},
		{R: 0.95, G: 0.85, B: 0.15, A: 1},
		{R: 0.20, G: 0.35, B: 0.95, A: 1},
	}
	av := make([]render.MeshGradientVertex, 4)
	for i := range av {
		av[i] = render.MeshGradientVertex{Pos: aPoints[i], Color: aColors[i]}
	}
	aBil := mustGrid(1, 1, cloneVerts(av), render.MeshGradientBilinear)
	catVerts := cloneVerts(av)
	for i := range catVerts {
		catVerts[i].Pos.X += 210
	}
	aCat := mustGrid(1, 1, catVerts, render.MeshGradientCatmullRom)

	// B: 2x2 auto tangents.
	bVs, _ := makeGrid(2, 2, 40, 40, 140, 110, 7, 0.05)

	// C: 3x3 auto tangents + explicit sharp center.
	cVs, _ := makeGrid(3, 3, 30, 40, 100, 85, 6, 0.35)
	center := 1*4 + 1
	zero := render.Point{}
	cVs[center].Left = &zero
	cVs[center].Top = &zero
	cVs[center].Right = &zero
	cVs[center].Bottom = &zero

	// D: 4x4 Catmull-Rom.
	dVs, _ := makeGrid(4, 4, 25, 40, 78, 62, 5, 0.60)

	return sceneData{
		aBilinear: aBil, aCatmull: aCat,
		b: mustGrid(2, 2, cloneVerts(bVs), render.MeshGradientBilinear),
		c: mustGrid(3, 3, cloneVerts(cVs), render.MeshGradientBilinear),
		d: mustGrid(4, 4, cloneVerts(dVs), render.MeshGradientCatmullRom),
	}
}

func cloneVerts(in []render.MeshGradientVertex) []render.MeshGradientVertex {
	out := make([]render.MeshGradientVertex, len(in))
	for i, v := range in {
		out[i] = v
		if v.Left != nil {
			p := *v.Left
			out[i].Left = &p
		}
		if v.Top != nil {
			p := *v.Top
			out[i].Top = &p
		}
		if v.Right != nil {
			p := *v.Right
			out[i].Right = &p
		}
		if v.Bottom != nil {
			p := *v.Bottom
			out[i].Bottom = &p
		}
	}
	return out
}

var scene = buildScene()

func maybeBreak(g *render.MeshGradient) *render.MeshGradient {
	if g == nil || breakMode == "" {
		return g
	}
	out := g.Clone()
	if breakMode == "zero-tangents" {
		zero := render.Point{}
		for i := range out.Vertices {
			out.Vertices[i].Left = &zero
			out.Vertices[i].Top = &zero
			out.Vertices[i].Right = &zero
			out.Vertices[i].Bottom = &zero
		}
	} else if breakMode == "swap-color-model" {
		out.ColorModel = render.MeshGradientBilinear
	}
	return out
}

// ── drawing ───────────────────────────────────────────────────────────────

func drawGradient(pc *rendering.PaintContext, g *render.MeshGradient, card int) {
	if pc == nil || pc.DC == nil || g == nil {
		return
	}
	g = maybeBreak(g)
	if dc, ok := pc.LiveContext(); ok {
		dc.Push()
		dc.Translate(pc.OriginX, pc.OriginY)
		res, err := dc.DrawMeshGradientEx(g, render.VertDrawOptions{})
		dc.Pop()
		cardStats[card].triangles += res.Triangles
		if res.Degraded {
			cardStats[card].degraded = true
		}
		if err != nil {
			cardStats[card].errs = append(cardStats[card].errs, err.Error())
		}
		return
	}
	pc.DC.Push()
	pc.DC.Translate(pc.OriginX, pc.OriginY)
	pc.DC.DrawMeshGradient(g)
	pc.DC.Pop()
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[0] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	drawGradient(pc, scene.aBilinear, 0)
	drawGradient(pc, scene.aCatmull, 0)
	rendering.StrokeLine(pc, 214, 35, 214, 280, 1, 0.35, 0.40, 0.50, 0.8)
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[1] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	drawGradient(pc, scene.b, 1)
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[2] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	drawGradient(pc, scene.c, 2)
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[3] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	drawGradient(pc, scene.d, 3)
}

// ── probe helpers (independent expected values) ───────────────────────────

func premul(c render.RGBA) [4]float64 {
	a := clamp01(c.A)
	return [4]float64{clamp01(c.R) * a, clamp01(c.G) * a, clamp01(c.B) * a, a}
}

func unpremul(v [4]float64) render.RGBA {
	if v[3] <= 0 {
		return render.RGBA{}
	}
	return render.RGBA{
		R: clamp01(v[0] / v[3]), G: clamp01(v[1] / v[3]),
		B: clamp01(v[2] / v[3]), A: clamp01(v[3]),
	}
}

func lerp4(a, b [4]float64, t float64) [4]float64 {
	var o [4]float64
	for i := range o {
		o[i] = a[i] + (b[i]-a[i])*t
	}
	return o
}

func bilerpRGBA(tl, tr, br, bl render.RGBA, u, v float64) render.RGBA {
	top := lerp4(premul(tl), premul(tr), u)
	bottom := lerp4(premul(bl), premul(br), u)
	return unpremul(lerp4(top, bottom, v))
}

func catmullW(t float64) [4]float64 {
	t2 := t * t
	t3 := t2 * t
	return [4]float64{
		-0.5*t3 + t2 - 0.5*t,
		1.5*t3 - 2.5*t2 + 1,
		-1.5*t3 + 2*t2 + 0.5*t,
		0.5*t3 - 0.5*t2,
	}
}

func probeColor(g *render.MeshGradient, r, c int, u, v float64) render.RGBA {
	stride := g.Cols + 1
	if g.ColorModel == render.MeshGradientBilinear {
		itl := r*stride + c
		return bilerpRGBA(g.Vertices[itl].Color, g.Vertices[itl+1].Color,
			g.Vertices[itl+stride+1].Color, g.Vertices[itl+stride].Color, u, v)
	}
	wu := catmullW(u)
	wv := catmullW(v)
	var rows [4][4]float64
	for i := 0; i < 4; i++ {
		rr := r - 1 + i
		if rr < 0 {
			rr = 0
		} else if rr > g.Rows {
			rr = g.Rows
		}
		var acc [4]float64
		for j := 0; j < 4; j++ {
			cc := c - 1 + j
			if cc < 0 {
				cc = 0
			} else if cc > g.Cols {
				cc = g.Cols
			}
			pm := premul(g.Vertices[rr*stride+cc].Color)
			for k := 0; k < 4; k++ {
				acc[k] += pm[k] * wu[j]
			}
		}
		rows[i] = acc
	}
	var out [4]float64
	for i := 0; i < 4; i++ {
		for k := 0; k < 4; k++ {
			out[k] += rows[i][k] * wv[i]
		}
	}
	return unpremul(out)
}

// probePoint returns the Coons surface center (u=v=0.5 equals the average of
// the four cell corners for any tangents). For edge probes it falls back to
// bilinear corner interpolation, which is exact at the shared vertices.
func probePoint(g *render.MeshGradient, r, c int, u, v float64) render.Point {
	stride := g.Cols + 1
	itl := r*stride + c
	tl := g.Vertices[itl].Pos
	tr := g.Vertices[itl+1].Pos
	bl := g.Vertices[itl+stride].Pos
	br := g.Vertices[itl+stride+1].Pos
	top := render.Pt(tl.X+(tr.X-tl.X)*u, tl.Y+(tr.Y-tl.Y)*u)
	bottom := render.Pt(bl.X+(br.X-bl.X)*u, bl.Y+(br.Y-bl.Y)*u)
	return render.Pt(top.X+(bottom.X-top.X)*v, top.Y+(bottom.Y-top.Y)*v)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ── logic probes ──────────────────────────────────────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
}

func runLogicProbes() []logicResult {
	var out []logicResult
	add := func(name string, ok bool, det string) { out = append(out, logicResult{name, ok, det}) }
	add("card_shapes",
		scene.aBilinear.Rows == 1 && scene.aBilinear.Cols == 1 && len(scene.aBilinear.Vertices) == 4 &&
			scene.b.Rows == 2 && scene.b.Cols == 2 && len(scene.b.Vertices) == 9 &&
			scene.c.Rows == 3 && scene.c.Cols == 3 && len(scene.c.Vertices) == 16 &&
			scene.d.Rows == 4 && scene.d.Cols == 4 && len(scene.d.Vertices) == 25,
		"1x1=4, 2x2=9, 3x3=16, 4x4=25 vertices")
	center := 1*4 + 1
	sharp := scene.c.Vertices[center].Left != nil && scene.c.Vertices[center].Top != nil &&
		scene.c.Vertices[center].Right != nil && scene.c.Vertices[center].Bottom != nil
	add("manual_sharp_tangent", sharp, "3x3 center vertex has explicit zero tangents")
	add("break_inactive", breakMode == "", fmt.Sprintf("GPUI_P39A_BREAK=%q", breakMode))
	return out
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
		wrkit.RequireMinRun(secs, "P39a-mesh-gradient-grid")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p39a_meshgradient_grid: logic %-24s ok=%v %s\n", l.name, l.ok, l.det)
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

	shell := wrkit.NewShell(winW, winH, "P3.9a mesh gradient 高层网格 — 切向量 / Bilinear / CatmullRom", []string{
		"A 1×1：左 Bilinear / 右 CatmullRom",
		"B 2×2：自动 C1 切向量",
		"C 3×3：自动 + 中心尖点切向量",
		"D 4×4：CatmullRom 颜色",
		"网格 = rows×cols 个 patch",
		"门禁：逻辑 + 像素 + 金图",
		"负向：GPUI_P39A_BREAK=zero-tangents",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A 1×1 Bilinear vs CatmullRom", paintCardA},
		{460, 8, 436, 300, "B 2×2 自动切向量", paintCardB},
		{8, 320, 436, 320, "C 3×3 自动 + 尖点", paintCardC},
		{460, 320, 436, 320, "D 4×4 CatmullRom", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) { paint(pc, sz) }
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p39a_meshgradient_grid", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p39a_meshgradient_grid_final.png")
	baseName := "p39a_meshgradient_grid_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p39a_meshgradient_grid_final_go.png"
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
		shell.UpdateHUD("P39a-mesh-gradient-grid", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			"1x1/2x2/3x3/4x4 · logic 3 · pixel 9")
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

	statsOK := true
	for i := range cardStats {
		if cardStats[i].triangles <= 0 || cardStats[i].degraded || len(cardStats[i].errs) > 0 {
			statsOK = false
			fmt.Fprintf(os.Stderr, "ui_wr_p39a_meshgradient_grid: card %d triangles=%d degraded=%v errs=%v\n",
				i, cardStats[i].triangles, cardStats[i].degraded, cardStats[i].errs)
		}
	}

	local := func(card int, x, y float64) (float64, float64) {
		return cardOrigin[card].X + x, cardOrigin[card].Y + y
	}
	mkProbe := func(name string, card int, x, y float64, c render.RGBA, tol uint8) wrkit.PixelProbe {
		ax, ay := local(card, x, y)
		return wrkit.PixelProbe{Name: name, X: ax, Y: ay, R: c.R, G: c.G, B: c.B, Tol: tol}
	}

	// A centers.
	pa := probePoint(scene.aBilinear, 0, 0, 0.5, 0.5)
	pb := probePoint(scene.aCatmull, 0, 0, 0.5, 0.5)
	probes := []wrkit.PixelProbe{
		mkProbe("A_bilinear_center", 0, pa.X, pa.Y, probeColor(scene.aBilinear, 0, 0, 0.5, 0.5), 35),
		mkProbe("A_catmull_center", 0, pb.X, pb.Y, probeColor(scene.aCatmull, 0, 0, 0.5, 0.5), 40),
	}
	// B cell center and shared center vertex.
	pb1 := probePoint(scene.b, 0, 0, 0.5, 0.5)
	probes = append(probes,
		mkProbe("B_cell_center", 1, pb1.X, pb1.Y, probeColor(scene.b, 0, 0, 0.5, 0.5), 35),
		mkProbe("B_shared_vertex", 1, scene.b.Vertices[4].Pos.X, scene.b.Vertices[4].Pos.Y, scene.b.Vertices[4].Color, 30),
	)
	// C center cell and D center cell.
	pc1 := probePoint(scene.c, 1, 1, 0.5, 0.5)
	pd1 := probePoint(scene.d, 2, 2, 0.5, 0.5)
	probes = append(probes,
		mkProbe("C_center_cell", 2, pc1.X, pc1.Y, probeColor(scene.c, 1, 1, 0.5, 0.5), 35),
		mkProbe("D_catmull_center", 3, pd1.X, pd1.Y, probeColor(scene.d, 2, 2, 0.5, 0.5), 40),
	)
	// Card backgrounds.
	probes = append(probes,
		mkProbe("A_background", 0, 420, 285, render.RGBA{R: 0.07, G: 0.08, B: 0.10, A: 1}, 12),
		mkProbe("B_background", 1, 420, 285, render.RGBA{R: 0.07, G: 0.08, B: 0.10, A: 1}, 12),
	)

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p39a_meshgradient_grid: pixel %-24s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p39a_meshgradient_grid: pixel %-24s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "MeshGradient{rows,cols,tangents,ColorModel}, Context.DrawMeshGradient/DrawMeshGradientEx, Canvas.DrawMeshGradient, recording.CmdDrawMeshGradient",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"render_stats_ok":       statsOK,
		"card_triangles":        []int{cardStats[0].triangles, cardStats[1].triangles, cardStats[2].triangles, cardStats[3].triangles},
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P39a-mesh-gradient-grid", Scenario: "ui_wr_p39a_meshgradient_grid", Backend: win.Backend().String(),
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
		fail("render stats: card triangles/degrade mismatch")
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
	fmt.Fprintf(os.Stderr, "ui_wr_p39a_meshgradient_grid: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
