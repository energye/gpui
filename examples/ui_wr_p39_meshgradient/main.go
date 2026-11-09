// Command ui_wr_p39_meshgradient is the P3.9 capability real window: colored
// Bezier patches (mesh gradient) in the Skia drawPatch shape.
//
// Four cards:
//
//	A  2x2 Coons patch grid (3x3 shared vertices): 12-control-point patches.
//	B  3x3 Coons patch grid (4x4 shared vertices): denser grid.
//	C  2x2 bicubic grid: 16-control-point tensor-product patches (PDF type 7),
//	   with the 4x4 control lattice drawn on the first patch.
//	D  corner roles (TL/TR/BR/BL) + translucent corner alpha over a checker.
//
// The patch tessellation lives in render (render/mesh_patch.go); this window
// only builds patch data through the public render API. Gate mode freezes the
// picture and asserts logic probes, semantic pixel probes and a zero-tolerance
// golden over the shell content (legend + body).
//
//	export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
//	RUN_SECONDS=6 go run ./examples/ui_wr_p39_meshgradient -auto-only
//	GPUI_ACCEPT_GOLDEN=1 RUN_SECONDS=6 go run ./examples/ui_wr_p39_meshgradient -auto-only
//	go run ./examples/ui_wr_p39_meshgradient                      # manual, stays open
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
	windowTitle = "gpui ui_wr_p39_meshgradient — P3.9 mesh gradient（2×2/3×3 patch）"
)

// ── patch data ────────────────────────────────────────────────────────────

type node struct {
	P render.Point
	C render.RGBA
}

var breakMode = os.Getenv("GPUI_P39_BREAK")

// cardOrigin captures the origin the painter actually received; probes map
// card-local geometry through the same numbers.
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

func lerpPt(a, b render.Point, t float64) render.Point {
	return render.Pt(a.X+(b.X-a.X)*t, a.Y+(b.Y-a.Y)*t)
}

// edgeControl returns the cubic control point at fraction t along a->b plus a
// symmetric bulge derived from the edge midpoint. Reversing a/b produces the
// same pair of points at 1/3 and 2/3, so adjacent patches share the edge
// exactly (G0/G1-ish patch grid).
func edgeControl(a, b render.Point, t float64) render.Point {
	mid := lerpPt(a, b, 0.5)
	dx := 7 * math.Cos(mid.Y*0.035+mid.X*0.011)
	dy := 7 * math.Sin(mid.X*0.035+mid.Y*0.013)
	p := lerpPt(a, b, t)
	return render.Pt(p.X+dx, p.Y+dy)
}

func coonsPatch(tl, tr, br, bl node) render.MeshPatch {
	pts := []render.Point{
		tl.P,
		edgeControl(tl.P, tr.P, 1.0/3.0), edgeControl(tl.P, tr.P, 2.0/3.0), tr.P,
		edgeControl(tr.P, br.P, 1.0/3.0), edgeControl(tr.P, br.P, 2.0/3.0), br.P,
		edgeControl(bl.P, br.P, 2.0/3.0), edgeControl(bl.P, br.P, 1.0/3.0), bl.P,
		edgeControl(tl.P, bl.P, 2.0/3.0), edgeControl(tl.P, bl.P, 1.0/3.0),
	}
	return render.MeshPatch{
		Style:  render.MeshPatchCoons,
		Points: pts,
		Colors: [4]render.RGBA{tl.C, tr.C, br.C, bl.C},
	}
}

func bilinearPoint(tl, tr, br, bl render.Point, u, v float64) render.Point {
	top := lerpPt(tl, tr, u)
	bottom := lerpPt(bl, br, u)
	return lerpPt(top, bottom, v)
}

func bicubicPatch(tl, tr, br, bl node, bulge float64) render.MeshPatch {
	p := make([]render.Point, 16)
	// row 0: top edge
	p[0] = tl.P
	p[1] = edgeControl(tl.P, tr.P, 1.0/3.0)
	p[2] = edgeControl(tl.P, tr.P, 2.0/3.0)
	p[3] = tr.P
	// row 1: v=1/3, left edge / interior / right edge
	p[4] = edgeControl(tl.P, bl.P, 1.0/3.0)
	p[5] = bilinearPoint(tl.P, tr.P, br.P, bl.P, 1.0/3.0, 1.0/3.0)
	p[6] = bilinearPoint(tl.P, tr.P, br.P, bl.P, 2.0/3.0, 1.0/3.0)
	p[7] = edgeControl(tr.P, br.P, 1.0/3.0)
	// row 2: v=2/3
	p[8] = edgeControl(tl.P, bl.P, 2.0/3.0)
	p[9] = bilinearPoint(tl.P, tr.P, br.P, bl.P, 1.0/3.0, 2.0/3.0)
	p[10] = bilinearPoint(tl.P, tr.P, br.P, bl.P, 2.0/3.0, 2.0/3.0)
	p[11] = edgeControl(tr.P, br.P, 2.0/3.0)
	// row 3: bottom edge
	p[12] = bl.P
	p[13] = edgeControl(bl.P, br.P, 1.0/3.0)
	p[14] = edgeControl(bl.P, br.P, 2.0/3.0)
	p[15] = br.P
	// The four interior control points are what makes the surface bicubic;
	// boundary points stay shared with the neighboring patches.
	for _, i := range []int{5, 6, 9, 10} {
		p[i].Y += bulge
	}
	return render.MeshPatch{
		Style:  render.MeshPatchBicubic,
		Points: p,
		Colors: [4]render.RGBA{tl.C, tr.C, br.C, bl.C},
	}
}

func makeNodes(n int, ox, oy, stepX, stepY float64, hueBase float64) [][]node {
	nodes := make([][]node, n+1)
	for r := 0; r <= n; r++ {
		nodes[r] = make([]node, n+1)
		for c := 0; c <= n; c++ {
			x := ox + float64(c)*stepX + 5*math.Cos(float64(r)*1.1+float64(c)*0.7)
			y := oy + float64(r)*stepY + 9*math.Sin(float64(c)*1.3+float64(r)*0.9)
			nodes[r][c] = node{
				P: render.Pt(x, y),
				C: hueColor(hueBase + float64(r*(n+1)+c)/float64((n+1)*(n+1))),
			}
		}
	}
	return nodes
}

func coonsGrid(nodes [][]node, n int) []render.MeshPatch {
	out := make([]render.MeshPatch, 0, n*n)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			out = append(out, coonsPatch(nodes[r][c], nodes[r][c+1], nodes[r+1][c+1], nodes[r+1][c]))
		}
	}
	return out
}

func bicubicGrid(nodes [][]node, n int, bulge float64) []render.MeshPatch {
	out := make([]render.MeshPatch, 0, n*n)
	for r := 0; r < n; r++ {
		for c := 0; c < n; c++ {
			out = append(out, bicubicPatch(nodes[r][c], nodes[r][c+1], nodes[r+1][c+1], nodes[r+1][c], bulge))
		}
	}
	return out
}

type sceneData struct {
	nodesA, nodesB, nodesC       [][]node
	patchesA, patchesB, patchesC []render.MeshPatch
	patchD, alphaD               render.MeshPatch
}

func buildScene() sceneData {
	nodesA := makeNodes(2, 68, 22, 150, 125, 0.02)
	nodesB := makeNodes(3, 38, 24, 120, 84, 0.10)
	nodesC := makeNodes(2, 68, 30, 150, 125, 0.55)

	tlD := node{P: render.Pt(30, 40), C: render.RGBA{R: 0.95, G: 0.15, B: 0.12, A: 1}}
	trD := node{P: render.Pt(205, 34), C: render.RGBA{R: 0.15, G: 0.85, B: 0.30, A: 1}}
	brD := node{P: render.Pt(210, 270), C: render.RGBA{R: 0.20, G: 0.35, B: 0.95, A: 1}}
	blD := node{P: render.Pt(34, 265), C: render.RGBA{R: 0.95, G: 0.85, B: 0.15, A: 1}}

	tlA := node{P: render.Pt(235, 40), C: render.RGBA{R: 0.95, G: 0.15, B: 0.12, A: 0.50}}
	trA := node{P: render.Pt(405, 34), C: render.RGBA{R: 0.15, G: 0.85, B: 0.30, A: 0.35}}
	brA := node{P: render.Pt(410, 270), C: render.RGBA{R: 0.20, G: 0.35, B: 0.95, A: 0.75}}
	blA := node{P: render.Pt(239, 265), C: render.RGBA{R: 1.0, G: 1.0, B: 1.0, A: 1.0}}

	return sceneData{
		nodesA: nodesA, nodesB: nodesB, nodesC: nodesC,
		patchesA: coonsGrid(nodesA, 2),
		patchesB: coonsGrid(nodesB, 3),
		patchesC: bicubicGrid(nodesC, 2, -16),
		patchD:   coonsPatch(tlD, trD, brD, blD),
		alphaD:   coonsPatch(tlA, trA, brA, blA),
	}
}

var scene = buildScene()

func maybeBreak(p render.MeshPatch) render.MeshPatch {
	if breakMode == "swap-corners" {
		p.Colors[0], p.Colors[2] = p.Colors[2], p.Colors[0]
	}
	return p
}

// ── drawing ───────────────────────────────────────────────────────────────

func drawPatch(pc *rendering.PaintContext, p render.MeshPatch, card int) {
	if pc == nil || pc.DC == nil {
		return
	}
	p = maybeBreak(p)
	if dc, ok := pc.LiveContext(); ok {
		dc.Push()
		dc.Translate(pc.OriginX, pc.OriginY)
		res, err := dc.DrawMeshPatchEx(p, render.VertDrawOptions{})
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
	pc.DC.DrawMeshPatch(p)
	pc.DC.Pop()
}

func paintCardA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[0] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[0] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	for _, p := range scene.patchesA {
		drawPatch(pc, p, 0)
	}
	for r := range scene.nodesA {
		for c := range scene.nodesA[r] {
			n := scene.nodesA[r][c]
			rendering.FillCircle(pc, n.P.X, n.P.Y, 3, 0.95, 0.95, 1.0, 0.90)
		}
	}
}

func paintCardB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[1] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[1] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	for _, p := range scene.patchesB {
		drawPatch(pc, p, 1)
	}
}

func paintCardC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[2] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[2] = patchStats{}
	rendering.FillRect(pc, 0, 0, sz.Width, sz.Height, 0.070, 0.080, 0.100, 1)
	for _, p := range scene.patchesC {
		drawPatch(pc, p, 2)
	}
	// Draw the 4x4 control lattice of the first patch: corners larger, edge
	// and interior control points smaller.
	for i, p := range scene.patchesC[0].Points {
		r := 2.0
		if i == 0 || i == 3 || i == 12 || i == 15 {
			r = 4.0
		}
		rendering.FillCircle(pc, p.X, p.Y, r, 0.98, 0.98, 1.0, 0.95)
	}
}

func checkerColor(x, y float64) render.RGBA {
	const cell = 52.0
	cx := int(math.Floor((x - 8) / cell))
	cy := int(math.Floor((y - 8) / 50.0))
	if (cx+cy)%2 == 0 {
		return render.RGBA{R: 0.160, G: 0.170, B: 0.200, A: 1}
	}
	return render.RGBA{R: 0.225, G: 0.235, B: 0.270, A: 1}
}

func paintCardD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	cardOrigin[3] = render.Pt(pc.OriginX, pc.OriginY)
	cardStats[3] = patchStats{}
	for r := 0; r < 6; r++ {
		for c := 0; c < 8; c++ {
			col := checkerColor(8+float64(c)*52, 8+float64(r)*50)
			rendering.FillRect(pc, 8+float64(c)*52, 8+float64(r)*50, 52, 50, col.R, col.G, col.B, 1)
		}
	}
	drawPatch(pc, scene.patchD, 3)
	drawPatch(pc, scene.alphaD, 3)
	rendering.StrokeRect(pc, 8, 8, sz.Width-16, sz.Height-16, 1, 0.45, 0.50, 0.60, 0.9)
}

// ── pure logic probes ─────────────────────────────────────────────────────

type logicResult struct {
	name string
	ok   bool
	det  string
	num  map[string]float64
}

func expectedLOD(p render.MeshPatch) (int, int) {
	var top, bottom, left, right [4]render.Point
	if p.Style == render.MeshPatchBicubic {
		top = [4]render.Point{p.Points[0], p.Points[1], p.Points[2], p.Points[3]}
		right = [4]render.Point{p.Points[3], p.Points[7], p.Points[11], p.Points[15]}
		bottom = [4]render.Point{p.Points[12], p.Points[13], p.Points[14], p.Points[15]}
		left = [4]render.Point{p.Points[0], p.Points[4], p.Points[8], p.Points[12]}
	} else {
		top = [4]render.Point{p.Points[0], p.Points[1], p.Points[2], p.Points[3]}
		right = [4]render.Point{p.Points[3], p.Points[4], p.Points[5], p.Points[6]}
		bottom = [4]render.Point{p.Points[9], p.Points[8], p.Points[7], p.Points[6]}
		left = [4]render.Point{p.Points[0], p.Points[11], p.Points[10], p.Points[9]}
	}
	arc := func(q [4]render.Point) float64 {
		l := 0.0
		for i := 0; i < 3; i++ {
			l += math.Hypot(q[i+1].X-q[i].X, q[i+1].Y-q[i].Y)
		}
		return l
	}
	lodX := int(math.Max(arc(top), arc(bottom)) / 10)
	lodY := int(math.Max(arc(left), arc(right)) / 10)
	if lodX < 8 {
		lodX = 8
	}
	if lodY < 8 {
		lodY = 8
	}
	if (lodX+1)*(lodY+1) > 10000 || lodX > 200 || lodY > 200 {
		sum := lodX + lodY
		lodX = int(float64(lodX) / float64(sum) * 200)
		lodY = int(float64(lodY) / float64(sum) * 200)
		if lodX < 1 {
			lodX = 1
		}
		if lodY < 1 {
			lodY = 1
		}
	}
	return lodX, lodY
}

func sumTriangles(ps []render.MeshPatch) int {
	n := 0
	for _, p := range ps {
		lx, ly := expectedLOD(p)
		n += lx * ly * 2
	}
	return n
}

func samePoint(a, b render.Point) bool {
	return math.Abs(a.X-b.X) <= 1e-9 && math.Abs(a.Y-b.Y) <= 1e-9
}

func runLogicProbes() []logicResult {
	var out []logicResult
	add := func(name string, ok bool, det string, num map[string]float64) {
		out = append(out, logicResult{name: name, ok: ok, det: det, num: num})
	}

	add("patch_counts", len(scene.patchesA) == 4 && len(scene.patchesB) == 9 && len(scene.patchesC) == 4,
		fmt.Sprintf("2x2=%d 3x3=%d bicubic=%d", len(scene.patchesA), len(scene.patchesB), len(scene.patchesC)),
		map[string]float64{"patches_2x2": float64(len(scene.patchesA)), "patches_3x3": float64(len(scene.patchesB)), "patches_bicubic": float64(len(scene.patchesC))})

	sharedOK, sharedN := true, 0
	checkGrid := func(nodes [][]node, patches []render.MeshPatch, n int) {
		idx := 0
		for r := 0; r < n; r++ {
			for c := 0; c < n; c++ {
				p := patches[idx]
				idx++
				if !samePoint(p.Points[0], nodes[r][c].P) || !samePoint(p.Points[3], nodes[r][c+1].P) ||
					!samePoint(p.Points[6], nodes[r+1][c+1].P) || !samePoint(p.Points[9], nodes[r+1][c].P) ||
					p.Colors[0] != nodes[r][c].C || p.Colors[1] != nodes[r][c+1].C ||
					p.Colors[2] != nodes[r+1][c+1].C || p.Colors[3] != nodes[r+1][c].C {
					sharedOK = false
				}
				sharedN += 4
			}
		}
	}
	checkGrid(scene.nodesA, scene.patchesA, 2)
	checkGrid(scene.nodesB, scene.patchesB, 3)
	add("coons_grid_corner_roles", sharedOK, fmt.Sprintf("%d corners TL/TR/BR/BL match the shared node grid", sharedN),
		map[string]float64{"corners_checked": float64(sharedN)})

	bicubicOK := len(scene.patchesC) == 4
	for _, p := range scene.patchesC {
		if p.Style != render.MeshPatchBicubic || len(p.Points) != 16 {
			bicubicOK = false
		}
	}
	add("bicubic_lattice_16", bicubicOK, "4 patches x 16 control points (4x4 lattice)", nil)

	lodOK := true
	for _, p := range append(append([]render.MeshPatch{}, scene.patchesA...), scene.patchesB...) {
		lx, ly := expectedLOD(p)
		if lx < 8 || ly < 8 || lx*ly*6 > 60000 {
			lodOK = false
		}
	}
	add("coons_lod_bounds", lodOK, "all patch LODs >=8 and index count <=60000", nil)

	// Alpha center color: premultiplied bilerp of the four corner roles.
	center := alphaCenter(scene.alphaD)
	add("alpha_center_distinct", center.A > 0.1 && center.A < 0.95,
		fmt.Sprintf("alpha patch center A=%.3f (premul bilerp)", center.A),
		map[string]float64{"alpha_center": center.A})

	add("break_inactive", breakMode == "",
		fmt.Sprintf("GPUI_P39_BREAK=%q (negative control must be run with it set)", breakMode), nil)

	return out
}

func alphaCenter(p render.MeshPatch) render.RGBA {
	tl, tr, br, bl := p.Colors[0], p.Colors[1], p.Colors[2], p.Colors[3]
	pm := func(c render.RGBA) render.RGBA {
		return render.RGBA{R: c.R * c.A, G: c.G * c.A, B: c.B * c.A, A: c.A}
	}
	a := lerpRGBA(pm(tl), pm(tr), 0.5)
	b := lerpRGBA(pm(bl), pm(br), 0.5)
	out := lerpRGBA(a, b, 0.5)
	if out.A <= 0 {
		return render.RGBA{}
	}
	return render.RGBA{R: out.R / out.A, G: out.G / out.A, B: out.B / out.A, A: out.A}
}

func lerpRGBA(a, b render.RGBA, t float64) render.RGBA {
	return render.RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func over(src, dst render.RGBA) render.RGBA {
	sa := src.A
	da := dst.A * (1 - sa)
	a := sa + da
	if a <= 0 {
		return render.RGBA{}
	}
	return render.RGBA{
		R: (src.R*sa + dst.R*da) / a,
		G: (src.G*sa + dst.G*da) / a,
		B: (src.B*sa + dst.B*da) / a,
		A: a,
	}
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
		wrkit.RequireMinRun(secs, "P39-mesh-gradient")
	} else if *manualSeconds > 0 {
		secs = int(*manualSeconds)
	}
	wrkit.EnsureUIFace()

	logic := runLogicProbes()
	logicOK, logicTotal := 0, len(logic)
	for _, l := range logic {
		fmt.Fprintf(os.Stderr, "ui_wr_p39_meshgradient: logic %-28s ok=%v %s\n", l.name, l.ok, l.det)
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

	shell := wrkit.NewShell(winW, winH, "P3.9 mesh gradient — 2×2 / 3×3 patch（Coons + bicubic）", []string{
		"A 2×2 Coons：4 patch / 9 顶点",
		"B 3×3 Coons：9 patch / 16 顶点",
		"C 2×2 bicubic：16 控制点",
		"D 角角色 + 半透明 alpha",
		"白点=共享角；C 白点=控制点",
		"门禁：逻辑 + 像素 + 金图",
		"负向：GPUI_P39_BREAK=swap-corners",
	})

	type cardSpec struct {
		x, y, w, h float64
		title      string
		paint      func(*rendering.PaintContext, rendering.Size)
	}
	specs := []cardSpec{
		{8, 8, 436, 300, "A 2×2 Coons patch 网格", paintCardA},
		{460, 8, 436, 300, "B 3×3 Coons patch 网格", paintCardB},
		{8, 320, 436, 320, "C 2×2 bicubic（16 控制点）", paintCardC},
		{460, 320, 436, 320, "D 角角色 + 半透明 alpha", paintCardD},
	}
	for _, cs := range specs {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cs.w, cs.h
		paint := cs.paint
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
			paint(pc, sz)
		}
		shell.Body.Place(box, cs.x, cs.y)
		shell.Body.LabelAt(cs.title, 11, cs.x+8, cs.y+6, 0.86, 0.90, 0.96)
	}

	snapDir := filepath.Join("examples", "ui_wr_p39_meshgradient", "testdata")
	_ = os.MkdirAll(snapDir, 0o755)
	snapPath := filepath.Join(snapDir, "p39_meshgradient_final.png")
	baseName := "p39_meshgradient_final.png"
	if os.Getenv("GPUI_BACKEND") == "go" {
		baseName = "p39_meshgradient_final_go.png"
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
		shell.UpdateHUD("P39-mesh-gradient", wrkit.PhaseSteady, app, snapH.PaintCount > 0,
			fmt.Sprintf("paint=%d presents=%d", snapH.PaintCount, app.PresentCount()),
			"cards A/B/C/D · logic 6 · pixel 16")
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

	// ── render-level probes: the GPU path must have tessellated each card ──
	wantTris := [4]int{
		sumTriangles(scene.patchesA),
		sumTriangles(scene.patchesB),
		sumTriangles(scene.patchesC),
		sumTriangles([]render.MeshPatch{scene.patchD, scene.alphaD}),
	}
	statsOK := true
	for i := range cardStats {
		if cardStats[i].triangles != wantTris[i] || cardStats[i].degraded || len(cardStats[i].errs) > 0 {
			statsOK = false
			fmt.Fprintf(os.Stderr, "ui_wr_p39_meshgradient: card %d triangles=%d want=%d degraded=%v errs=%v\n",
				i, cardStats[i].triangles, wantTris[i], cardStats[i].degraded, cardStats[i].errs)
		}
	}

	// ── pixel probes in window-logical coordinates ──
	local := func(card int, x, y float64) (float64, float64) {
		return cardOrigin[card].X + x, cardOrigin[card].Y + y
	}
	mkProbe := func(name string, card int, x, y float64, c render.RGBA, tol uint8) wrkit.PixelProbe {
		ax, ay := local(card, x, y)
		return wrkit.PixelProbe{Name: name, X: ax, Y: ay, R: c.R, G: c.G, B: c.B, Tol: tol}
	}

	aTL := scene.nodesA[0][0]
	aTR := scene.nodesA[0][2]
	aBR := scene.nodesA[2][2]
	aBL := scene.nodesA[2][0]
	aC := scene.nodesA[1][1]
	bC := scene.nodesB[2][2]
	cCenter := scene.patchesC[0]
	cCorner := scene.patchesC[0].Points[0]
	cCenterPt := bicubicSurfacePoint(cCenter, 0.5, 0.5)
	dCorner := scene.patchD.Points
	alphaCorner := scene.alphaD.Points[0]
	alphaCenterColor := alphaCenter(scene.alphaD)
	alphaCenterPt := coonsSurfacePoint(scene.alphaD, 0.5, 0.5)

	probes := []wrkit.PixelProbe{
		mkProbe("A_tl_corner", 0, aTL.P.X+6, aTL.P.Y+6, aTL.C, 30),
		mkProbe("A_tr_corner", 0, aTR.P.X-6, aTR.P.Y+6, aTR.C, 30),
		mkProbe("A_br_corner", 0, aBR.P.X-6, aBR.P.Y-6, aBR.C, 30),
		mkProbe("A_bl_corner", 0, aBL.P.X+6, aBL.P.Y-6, aBL.C, 30),
		mkProbe("A_center_node", 0, aC.P.X+5, aC.P.Y+5, aC.C, 30),
		mkProbe("B_center_node", 1, bC.P.X-5, bC.P.Y-5, bC.C, 30),
		mkProbe("C_patch_center", 2, cCenterPt.X, cCenterPt.Y,
			avg4(cCenter.Colors), 35),
		mkProbe("C_corner", 2, cCorner.X+6, cCorner.Y+6, cCenter.Colors[0], 30),
		mkProbe("D_tl_corner", 3, dCorner[0].X+6, dCorner[0].Y+6, scene.patchD.Colors[0], 30),
		mkProbe("D_tr_corner", 3, dCorner[3].X-6, dCorner[3].Y+6, scene.patchD.Colors[1], 30),
		mkProbe("D_br_corner", 3, dCorner[6].X-6, dCorner[6].Y-6, scene.patchD.Colors[2], 30),
		mkProbe("D_bl_corner", 3, dCorner[9].X+6, dCorner[9].Y-6, scene.patchD.Colors[3], 30),
		mkProbe("D_alpha_tl", 3, alphaCorner.X+6, alphaCorner.Y+6,
			over(scene.alphaD.Colors[0], checkerColor(alphaCorner.X+6, alphaCorner.Y+6)), 40),
		mkProbe("D_alpha_center", 3, alphaCenterPt.X, alphaCenterPt.Y,
			over(alphaCenterColor, checkerColor(alphaCenterPt.X, alphaCenterPt.Y)), 45),
	}
	// Seam probes: 2px inside each side of a shared patch edge; a gap would
	// show the card background instead of patch ink.
	seamPt := coonsSurfacePoint(scene.patchesA[0], 1.0, 0.5)
	seamColor := lerpRGBA(scene.nodesA[0][1].C, scene.nodesA[1][1].C, 0.5)
	probes = append(probes,
		mkProbe("A_seam_left", 0, seamPt.X-1.5, seamPt.Y, seamColor, 45),
		mkProbe("A_seam_right", 0, seamPt.X+1.5, seamPt.Y, seamColor, 45),
	)

	var pixelErr error
	for _, pr := range probes {
		if err := wrkit.CheckPixelProbes(snapPath, winW, []wrkit.PixelProbe{pr}); err != nil {
			fmt.Fprintf(os.Stderr, "ui_wr_p39_meshgradient: pixel %-24s FAIL %v\n", pr.Name, err)
			if pixelErr == nil {
				pixelErr = err
			}
		} else {
			fmt.Fprintf(os.Stderr, "ui_wr_p39_meshgradient: pixel %-24s ok\n", pr.Name)
		}
	}

	goldenDiff, goldenPx, seeded, goldenErr := wrkit.GoldenGate(snapPath, basePath, wrkit.ShellContentRects())

	extra := map[string]any{
		"fps_gate": "off_correctness", "slope_gate": "off",
		"apis":            "MeshPatch{Coons 12, Bicubic 16}, Context.DrawMeshPatch/DrawMeshPatchEx, Canvas.DrawMeshPatch, recording.CmdDrawMeshPatch",
		"logic_probes_ok": logicOK, "logic_probes_total": logicTotal, "logic_failed": logicFail,
		"render_stats_ok":       statsOK,
		"card_triangles":        []int{cardStats[0].triangles, cardStats[1].triangles, cardStats[2].triangles, cardStats[3].triangles},
		"card_triangles_want":   []int{wantTris[0], wantTris[1], wantTris[2], wantTris[3]},
		"pixel_golden_diff_pct": goldenDiff, "pixel_golden_total_px": goldenPx,
		"pixel_golden_first_run": seeded,
		"break_mode":             breakMode,
	}
	for i, l := range logic {
		extra[fmt.Sprintf("logic_%02d_%s", i+1, l.name)] = l.ok
	}
	report := wrgate.BuildReport(wrgate.BuildInput{
		AbilityID: "P39-mesh-gradient", Scenario: "ui_wr_p39_meshgradient", Backend: win.Backend().String(),
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
		fail("render stats: card triangle counts/degrade mismatch")
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
	fmt.Fprintf(os.Stderr, "ui_wr_p39_meshgradient: OK presents=%d logic=%d/%d golden=%.4f%% seeded=%v gate=%v elapsed=%.1fs\n",
		app.PresentCount(), logicOK, logicTotal, goldenDiff, seeded, *autoOnly, elapsed)
}

func cubicPt(p [4]render.Point, t float64) render.Point {
	mt := 1 - t
	b0 := mt * mt * mt
	b1 := 3 * t * mt * mt
	b2 := 3 * t * t * mt
	b3 := t * t * t
	return render.Pt(
		b0*p[0].X+b1*p[1].X+b2*p[2].X+b3*p[3].X,
		b0*p[0].Y+b1*p[1].Y+b2*p[2].Y+b3*p[3].Y,
	)
}

func coonsSurfacePoint(p render.MeshPatch, u, v float64) render.Point {
	top := cubicPt([4]render.Point{p.Points[0], p.Points[1], p.Points[2], p.Points[3]}, u)
	bottom := cubicPt([4]render.Point{p.Points[9], p.Points[8], p.Points[7], p.Points[6]}, u)
	left := cubicPt([4]render.Point{p.Points[0], p.Points[11], p.Points[10], p.Points[9]}, v)
	right := cubicPt([4]render.Point{p.Points[3], p.Points[4], p.Points[5], p.Points[6]}, v)
	s0 := lerpPt(top, bottom, v)
	s1 := lerpPt(left, right, u)
	s2 := lerpPt(lerpPt(p.Points[0], p.Points[3], u), lerpPt(p.Points[9], p.Points[6], u), v)
	return render.Pt(s0.X+s1.X-s2.X, s0.Y+s1.Y-s2.Y)
}

func bicubicSurfacePoint(p render.MeshPatch, u, v float64) render.Point {
	bu := [4]float64{(1 - u) * (1 - u) * (1 - u), 3 * u * (1 - u) * (1 - u), 3 * u * u * (1 - u), u * u * u}
	bv := [4]float64{(1 - v) * (1 - v) * (1 - v), 3 * v * (1 - v) * (1 - v), 3 * v * v * (1 - v), v * v * v}
	var x, y float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			w := bv[i] * bu[j]
			x += w * p.Points[i*4+j].X
			y += w * p.Points[i*4+j].Y
		}
	}
	return render.Pt(x, y)
}

func avg4(c [4]render.RGBA) render.RGBA {
	return render.RGBA{
		R: (c[0].R + c[1].R + c[2].R + c[3].R) / 4,
		G: (c[0].G + c[1].G + c[2].G + c[3].G) / 4,
		B: (c[0].B + c[1].B + c[2].B + c[3].B) / 4,
		A: 1,
	}
}

type tickFunc struct{ on func(dt float64) }

func (t *tickFunc) Tick(dt float64) bool { t.on(dt); return true }
