package main

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
	contentW     = cardW - 2*contentX0
	contentH     = cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette. Both the scene and the pixel oracle read these constants, so a
// palette edit can never desync the drawn pixels from the asserted ones.
var (
	colCardBG     = [3]float64{0.10, 0.11, 0.13}
	colSlotBG     = [3]float64{0.145, 0.160, 0.190}
	colSlotBorder = [3]float64{0.32, 0.36, 0.44}
	colVR         = [3]float64{0.96, 0.22, 0.20}
	colVG         = [3]float64{0.20, 0.86, 0.30}
	colVB         = [3]float64{0.28, 0.44, 0.98}
	colVY         = [3]float64{0.95, 0.84, 0.22}
	colSolid      = [3]float64{0.98, 0.72, 0.18}
	colRot        = [3]float64{0.18, 0.82, 0.86}
	colSingle     = [3]float64{0.55, 0.88, 0.35}
	colMismatch   = [3]float64{0.85, 0.36, 0.86}
)

// ---- card geometry (content-local) ----

var (
	// A: Gouraud triangle via DrawVerticesEx + solid triangle via DrawVertices.
	aTri      = [3]render.Point{{X: 24, Y: 178}, {X: 150, Y: 178}, {X: 87, Y: 58}}
	aTriCols  = [3][3]float64{colVR, colVG, colVB}
	aSolidTri = [3]render.Point{{X: 230, Y: 178}, {X: 356, Y: 178}, {X: 293, Y: 58}}

	// B: colored fan + the same fan under a rotated CTM, solid cyan.
	bFan     = [4]render.Point{{X: 30, Y: 60}, {X: 170, Y: 60}, {X: 170, Y: 176}, {X: 30, Y: 176}}
	bFanCols = [4][3]float64{colVR, colVG, colVB, colVY}
	bCenter  = render.Point{X: 300, Y: 110}
	bHW      = 60.0
	bHH      = 45.0
	bDeg     = 25.0
	bLocal   = [4]render.Point{{X: -bHW, Y: -bHH}, {X: bHW, Y: -bHH}, {X: bHW, Y: bHH}, {X: -bHW, Y: bHH}}

	// C: indexed quad via DrawMeshEx, the same quad expanded via DrawVertices,
	// and a single 3-index triangle (6-byte index upload alignment pin).
	cQuad   = [4]render.Point{{X: 24, Y: 44}, {X: 148, Y: 44}, {X: 148, Y: 150}, {X: 24, Y: 150}}
	cShiftX = 196.0
	cSingle = [3]render.Point{{X: 198, Y: 180}, {X: 286, Y: 180}, {X: 242, Y: 150}}

	// D: three slots. Left = short indices (skipped), middle = mismatched
	// colors (solid fallback), right = bad index (Ex rejects).
	dSlotX = [3]float64{16, 154, 292}
	dSlotY = 44.0
	dSlotW = 120.0
	dSlotH = 124.0
	dShort = [4]render.Point{{X: 26, Y: 62}, {X: 126, Y: 62}, {X: 126, Y: 150}, {X: 26, Y: 150}}
	dBad   = [4]render.Point{{X: 302, Y: 62}, {X: 402, Y: 62}, {X: 402, Y: 150}, {X: 302, Y: 150}}
)

var quadIndices = []uint16{0, 1, 2, 0, 2, 3}

// drawDiagnostics captures Ex results from the real window paint so the JSON
// records the GPU-side contract (no degrade, exact triangle counts, error).
var (
	diagMu sync.Mutex
	diag   = map[string]string{}
)

func noteDraw(key string, res render.VertDrawResult, err error) {
	diagMu.Lock()
	defer diagMu.Unlock()
	if err != nil {
		diag[key] = "err=" + err.Error()
		return
	}
	diag[key] = fmt.Sprintf("triangles=%d skipped=%v degraded=%v reason=%q",
		res.Triangles, res.Skipped, res.Degraded, res.Reason)
}

func drawDiagnostics() map[string]string {
	diagMu.Lock()
	defer diagMu.Unlock()
	out := make(map[string]string, len(diag))
	for k, v := range diag {
		out[k] = v
	}
	return out
}

func rgbaOf(c [3]float64) render.RGBA {
	return render.RGBA{R: c[0], G: c[1], B: c[2], A: 1}
}

func offsetPts(pts []render.Point, dx, dy float64) []render.Point {
	out := make([]render.Point, len(pts))
	for i, p := range pts {
		out[i] = render.Point{X: p.X + dx, Y: p.Y + dy}
	}
	return out
}

func meshColors(cs [][3]float64) []render.RGBA {
	out := make([]render.RGBA, len(cs))
	for i, c := range cs {
		out[i] = rgbaOf(c)
	}
	return out
}

func expandedQuad(pts [4]render.Point, cols [4][3]float64, dx float64) ([]render.Point, []render.RGBA) {
	p := offsetPts(pts[:], dx, 0)
	c := meshColors(cols[:])
	return []render.Point{p[0], p[1], p[2], p[0], p[2], p[3]},
		[]render.RGBA{c[0], c[1], c[2], c[0], c[2], c[3]}
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildVerticesScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A DrawVertices：Gouraud 三角形 / 纯色回退 / Ex 诊断", paintVertsA)
	card(1, "B TriangleFan + CTM：四色扇面 / 旋转 25° 纯色扇面", paintVertsB)
	card(2, "C DrawMesh：左 indexed = 右 expanded / 下 3 索引单三角", paintVertsC)
	card(3, "D 边界：短索引不画 / 颜色不匹配实心 / 坏索引拒绝", paintVertsD)
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func paintVertsA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	// Gouraud: Ex validates then draws through the same GPU mesh queue.
	res, err := dc.DrawVerticesEx(
		offsetPts(aTri[:], cx, cy),
		meshColors(aTriCols[:]),
		render.VertexModeTriangles, render.VertDrawOptions{})
	noteDraw("verts_gouraud", res, err)

	// Solid fallback: nil vertex colors use the current paint.
	dc.SetRGBA(colSolid[0], colSolid[1], colSolid[2], 1)
	dc.DrawVertices(offsetPts(aSolidTri[:], cx, cy), nil, render.VertexModeTriangles)
}

func paintVertsB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.DrawVertices(offsetPts(bFan[:], cx, cy), meshColors(bFanCols[:]),
		render.VertexModeTriangleFan)

	// Same fan, solid, under a rotated CTM. The rotate probe region below
	// proves this is not merely a translated copy of the unrotated quad.
	dc.Push()
	dc.Translate(cx+bCenter.X, cy+bCenter.Y)
	dc.Rotate(bDeg * math.Pi / 180)
	dc.SetRGBA(colRot[0], colRot[1], colRot[2], 1)
	dc.DrawVertices(bLocal[:], nil, render.VertexModeTriangleFan)
	dc.Pop()
}

func paintVertsC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	idx := render.Mesh{
		Positions: offsetPts(cQuad[:], cx, cy),
		Colors:    meshColors([][3]float64{colVR, colVG, colVB, colVY}),
		Indices:   quadIndices,
	}
	res, err := dc.DrawMeshEx(idx, render.VertDrawOptions{})
	noteDraw("mesh_indexed", res, err)

	expPos, expCol := expandedQuad(cQuad, [4][3]float64{colVR, colVG, colVB, colVY}, cShiftX)
	dc.DrawVertices(offsetPts(expPos, cx, cy), expCol, render.VertexModeTriangles)

	// Three indices = 6 bytes: the old upload failed WebGPU's 4-byte
	// Queue.WriteBuffer alignment rule. Still visible here as a pin.
	dc.SetRGBA(colSingle[0], colSingle[1], colSingle[2], 1)
	dc.DrawMesh(render.Mesh{
		Positions: offsetPts(cSingle[:], cx, cy),
		Indices:   []uint16{0, 1, 2},
	})
}

func paintVertsD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	for i := range dSlotX {
		x, y := cx+dSlotX[i], cy+dSlotY
		dc.SetRGBA(colSlotBG[0], colSlotBG[1], colSlotBG[2], 1)
		dc.DrawRectangle(x, y, dSlotW, dSlotH)
		_ = dc.Fill()
		dc.SetRGBA(colSlotBorder[0], colSlotBorder[1], colSlotBorder[2], 1)
		dc.DrawRectangle(x, y, dSlotW, 2)
		_ = dc.Fill()
		dc.DrawRectangle(x, y+dSlotH-2, dSlotW, 2)
		_ = dc.Fill()
		dc.DrawRectangle(x, y, 2, dSlotH)
		_ = dc.Fill()
		dc.DrawRectangle(x+dSlotW-2, y, 2, dSlotH)
		_ = dc.Fill()
	}

	// Slot 0: non-empty Indices with fewer than three entries draws nothing.
	dc.DrawMesh(render.Mesh{
		Positions: offsetPts(dShort[:], cx, cy),
		Colors:    meshColors([][3]float64{colVR, colVG, colVB, colVY}),
		Indices:   []uint16{0},
	})

	// Slot 1: mismatched color count falls back to the solid paint.
	dc.SetRGBA(colMismatch[0], colMismatch[1], colMismatch[2], 1)
	dc.DrawMesh(render.Mesh{
		Positions: offsetPts(dShort[:], cx+138, cy),
		Colors:    []render.RGBA{{R: 1, A: 1}, {G: 1, A: 1}},
		Indices:   quadIndices,
	})

	// Slot 2: Ex validates every index and must not queue anything.
	res, err := dc.DrawMeshEx(render.Mesh{
		Positions: offsetPts(dBad[:], cx, cy),
		Colors:    meshColors([][3]float64{colVR, colVG, colVB, colVY}),
		Indices:   []uint16{0, 1, 9},
	}, render.VertDrawOptions{})
	noteDraw("mesh_bad", res, err)
}

// ---------------------------------------------------------------------------
// Point pixel probes (analytic oracle).
// ---------------------------------------------------------------------------

type band struct {
	ch       int
	min, max float64
}

type probeSpec struct {
	card  int
	key   string
	desc  string
	pt    render.Point
	want  func(ox, oy, dpr float64, px, py int) (float64, float64, float64)
	bands []band
}

const tolSolid = 23.0 / 255

var probeSpecs []probeSpec

func solid(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
	return func(float64, float64, float64, int, int) (float64, float64, float64) {
		return c[0], c[1], c[2]
	}
}

func baryColor(p0, p1, p2 render.Point, c0, c1, c2 render.RGBA, q render.Point) (float64, float64, float64, bool) {
	den := (p1.Y-p2.Y)*(p0.X-p2.X) + (p2.X-p1.X)*(p0.Y-p2.Y)
	if math.Abs(den) < 1e-12 {
		return 0, 0, 0, false
	}
	w0 := ((p1.Y-p2.Y)*(q.X-p2.X) + (p2.X-p1.X)*(q.Y-p2.Y)) / den
	w1 := ((p2.Y-p0.Y)*(q.X-p2.X) + (p0.X-p2.X)*(q.Y-p2.Y)) / den
	w2 := 1 - w0 - w1
	if w0 < -1e-9 || w1 < -1e-9 || w2 < -1e-9 {
		return 0, 0, 0, false
	}
	return w0*c0.R + w1*c1.R + w2*c2.R,
		w0*c0.G + w1*c1.G + w2*c2.G,
		w0*c0.B + w1*c1.B + w2*c2.B, true
}

func triWant(pts [3]render.Point, cols [3][3]float64) func(ox, oy, dpr float64, px, py int) (float64, float64, float64) {
	return func(ox, oy, dpr float64, px, py int) (float64, float64, float64) {
		q := render.Point{X: (float64(px)+0.5)/dpr - ox, Y: (float64(py)+0.5)/dpr - oy}
		r, g, b, ok := baryColor(pts[0], pts[1], pts[2],
			rgbaOf(cols[0]), rgbaOf(cols[1]), rgbaOf(cols[2]), q)
		if !ok {
			return -1, -1, -1
		}
		return r, g, b
	}
}

func fanWant(pts [4]render.Point, cols [4][3]float64) func(ox, oy, dpr float64, px, py int) (float64, float64, float64) {
	return func(ox, oy, dpr float64, px, py int) (float64, float64, float64) {
		q := render.Point{X: (float64(px)+0.5)/dpr - ox, Y: (float64(py)+0.5)/dpr - oy}
		for _, tri := range [3][3]int{{0, 1, 2}, {0, 2, 3}} {
			if r, g, b, ok := baryColor(pts[tri[0]], pts[tri[1]], pts[tri[2]],
				rgbaOf(cols[tri[0]]), rgbaOf(cols[tri[1]]), rgbaOf(cols[tri[2]]), q); ok {
				return r, g, b
			}
		}
		return -1, -1, -1
	}
}

func buildProbeSpecs() {
	probeSpecs = nil
	add := func(card int, key, desc string, x, y float64,
		want func(ox, oy, dpr float64, px, py int) (float64, float64, float64), bands ...band) {
		probeSpecs = append(probeSpecs, probeSpec{
			card: card, key: key, desc: desc,
			pt: render.Point{X: x, Y: y}, want: want, bands: bands,
		})
	}
	addSolid := func(card int, key, desc string, x, y float64, c [3]float64) {
		add(card, key, desc, x, y, solid(c))
	}
	redBand := []band{{ch: 0, min: 0.55, max: 1.0}, {ch: 1, min: 0, max: 0.45}, {ch: 2, min: 0, max: 0.45}}
	greenBand := []band{{ch: 0, min: 0, max: 0.45}, {ch: 1, min: 0.55, max: 1.0}, {ch: 2, min: 0, max: 0.45}}
	blueBand := []band{{ch: 0, min: 0, max: 0.45}, {ch: 1, min: 0, max: 0.45}, {ch: 2, min: 0.55, max: 1.0}}
	yellowBand := []band{{ch: 0, min: 0.55, max: 1.0}, {ch: 1, min: 0.55, max: 1.0}, {ch: 2, min: 0, max: 0.35}}

	// A: Gouraud interpolation near each vertex, solid fallback, card ground.
	add(0, "a_tri_red", "Gouraud red vertex", 40, 166, triWant(aTri, aTriCols), redBand...)
	add(0, "a_tri_green", "Gouraud green vertex", 136, 166, triWant(aTri, aTriCols), greenBand...)
	add(0, "a_tri_blue", "Gouraud blue vertex", 87, 66, triWant(aTri, aTriCols), blueBand...)
	addSolid(0, "a_solid_amber", "nil colors use solid paint", 293, 155, colSolid)
	addSolid(0, "a_card_bg", "card ground between triangles", 205, 70, colCardBG)

	// B: fan corner dominance, rotated CTM center, and the unrotated-only
	// corner that must stay background when the CTM rotation is applied.
	add(1, "b_fan_red", "fan red corner", 42, 72, fanWant(bFan, bFanCols), redBand...)
	add(1, "b_fan_green", "fan green corner", 158, 72, fanWant(bFan, bFanCols), greenBand...)
	add(1, "b_fan_blue", "fan blue corner", 158, 168, fanWant(bFan, bFanCols), blueBand...)
	add(1, "b_fan_yellow", "fan yellow corner", 50, 166, fanWant(bFan, bFanCols), yellowBand...)
	addSolid(1, "b_rot_center", "rotated fan center", 300, 110, colRot)
	addSolid(1, "b_rot_out", "unrotated corner is empty after CTM", 245, 70, colCardBG)

	// C: indexed and expanded quads share the same palette; the single
	// 3-index triangle is the alignment pin.
	add(2, "c_idx_red", "indexed quad red corner", 36, 56, fanWant([4]render.Point{cQuad[0], cQuad[1], cQuad[2], cQuad[3]},
		[4][3]float64{colVR, colVG, colVB, colVY}), redBand...)
	cRight := [4]render.Point{
		{X: cQuad[0].X + cShiftX, Y: cQuad[0].Y},
		{X: cQuad[1].X + cShiftX, Y: cQuad[1].Y},
		{X: cQuad[2].X + cShiftX, Y: cQuad[2].Y},
		{X: cQuad[3].X + cShiftX, Y: cQuad[3].Y},
	}
	add(2, "c_exp_red", "expanded quad red corner", 232, 56, fanWant(cRight,
		[4][3]float64{colVR, colVG, colVB, colVY}), redBand...)
	addSolid(2, "c_single_green", "3-index single triangle", 242, 170, colSingle)

	// D: empty slots stay slot-background; the mismatch slot is solid magenta.
	addSolid(3, "d_short_bg", "short indices draw nothing", 76, 104, colSlotBG)
	addSolid(3, "d_mismatch", "mismatched colors solid fallback", 214, 104, colMismatch)
	addSolid(3, "d_bad_bg", "bad index draws nothing", 352, 104, colSlotBG)
}

// ---------------------------------------------------------------------------
// Logic probes (CPU offscreen child): value-level contracts.
// ---------------------------------------------------------------------------

var probeDetail string

func setDetail(format string, args ...any) { probeDetail = fmt.Sprintf(format, args...) }

func probeCtxOpaque(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

func sampleRGBA(dc *render.Context, x, y int) (r, g, b, a float64) {
	img := dc.Image()
	r32, g32, b32, a32 := img.At(x, y).RGBA()
	return float64(r32>>8) / 255, float64(g32>>8) / 255, float64(b32>>8) / 255, float64(a32>>8) / 255
}

func inkCount(dc *render.Context, w, h int) int {
	n := 0
	img := dc.Image()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 < 250 || g>>8 < 250 || b>>8 < 250 {
				n++
			}
		}
	}
	return n
}

func imageDiff(a, b *render.Context, tol int) (changed, max int) {
	ia, ib := a.Image(), b.Image()
	w, h := ia.Bounds().Dx(), ia.Bounds().Dy()
	if ib.Bounds().Dx() != w || ib.Bounds().Dy() != h {
		return w * h, 255
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, aa := ia.At(x, y).RGBA()
			br, bg, bb, ba := ib.At(x, y).RGBA()
			d := absInt(int(ar>>8) - int(br>>8))
			if v := absInt(int(ag>>8) - int(bg>>8)); v > d {
				d = v
			}
			if v := absInt(int(ab>>8) - int(bb>>8)); v > d {
				d = v
			}
			if v := absInt(int(aa>>8) - int(ba>>8)); v > d {
				d = v
			}
			if d > tol {
				changed++
			}
			if d > max {
				max = d
			}
		}
	}
	return changed, max
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func meshQuad() ([]render.Point, []render.RGBA) {
	pos := []render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 56, Y: 56}, {X: 8, Y: 56}}
	col := meshColors([][3]float64{colVR, colVG, colVB, colVY})
	return pos, col
}

func runLogicProbes() (probes, failed []string) {
	check := func(key string, pass bool) {
		probes = append(probes, key)
		if !pass {
			failed = append(failed, key)
			if probeDetail != "" {
				fmt.Fprintf(os.Stderr, "logic %s FAILED: %s\n", key, probeDetail)
			}
		}
		probeDetail = ""
	}

	// 1) Gouraud triangle: each corner keeps its own channel; not an average.
	{
		dc := probeCtxOpaque(64, 64)
		pts := []render.Point{{X: 8, Y: 56}, {X: 56, Y: 56}, {X: 32, Y: 8}}
		dc.DrawVertices(pts, meshColors([][3]float64{colVR, colVG, colVB}), render.VertexModeTriangles)
		r, g, b, _ := sampleRGBA(dc, 12, 50)
		gr, gg, gb, _ := sampleRGBA(dc, 52, 50)
		br, bg, bb, _ := sampleRGBA(dc, 32, 18)
		setDetail("left=(%.3f,%.3f,%.3f) right=(%.3f,%.3f,%.3f) top=(%.3f,%.3f,%.3f)",
			r, g, b, gr, gg, gb, br, bg, bb)
		check("verts_gradient",
			r > 0.55 && g < 0.5 && b < 0.5 &&
				gg > 0.55 && gr < 0.5 && gb < 0.5 &&
				bb > 0.55 && br < 0.5 && bg < 0.5)
		dc.Close()
	}

	// 2) Ex fan reports the real triangle count and the CPU degrade marker.
	{
		dc := probeCtxOpaque(64, 64)
		pos, col := meshQuad()
		res, err := dc.DrawVerticesEx(pos, col, render.VertexModeTriangleFan, render.VertDrawOptions{})
		setDetail("res=%+v err=%v", res, err)
		check("verts_ex_fan_triangles",
			err == nil && !res.Skipped && res.Triangles == 2 && res.Degraded && res.Reason == render.VertCPUFallbackReason)
		dc.Close()
	}

	// 3) Empty/short vertex lists skip cleanly with no ink.
	{
		ok := true
		for _, pts := range [][]render.Point{nil, {{X: 2, Y: 2}, {X: 9, Y: 2}}} {
			dc := probeCtxOpaque(64, 64)
			res, err := dc.DrawVerticesEx(pts, nil, render.VertexModeTriangles, render.VertDrawOptions{})
			if err != nil || !res.Skipped || res.Triangles != 0 || inkCount(dc, 64, 64) != 0 {
				ok = false
				setDetail("pts=%d res=%+v err=%v ink=%d", len(pts), res, err, inkCount(dc, 64, 64))
			}
			dc.Close()
		}
		check("verts_ex_empty_skipped", ok)
	}

	// 4) NaN position is rejected before any rasterization.
	{
		dc := probeCtxOpaque(64, 64)
		pts := []render.Point{{X: 8, Y: 8}, {X: math.NaN(), Y: 20}, {X: 30, Y: 9}}
		res, err := dc.DrawVerticesEx(pts, nil, render.VertexModeTriangles, render.VertDrawOptions{})
		setDetail("res=%+v err=%v ink=%d", res, err, inkCount(dc, 64, 64))
		check("verts_ex_nonfinite_pos",
			errors.Is(err, render.ErrVertsNonFinite) && inkCount(dc, 64, 64) == 0)
		dc.Close()
	}

	// 5) Non-finite vertex color is rejected too.
	{
		dc := probeCtxOpaque(64, 64)
		pts := []render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 32, Y: 56}}
		cols := []render.RGBA{{R: 1, A: 1}, {G: math.Inf(1), A: 1}, {B: 1, A: 1}}
		res, err := dc.DrawVerticesEx(pts, cols, render.VertexModeTriangles, render.VertDrawOptions{})
		setDetail("res=%+v err=%v ink=%d", res, err, inkCount(dc, 64, 64))
		check("verts_ex_nonfinite_color",
			errors.Is(err, render.ErrVertsNonFinite) && inkCount(dc, 64, 64) == 0)
		dc.Close()
	}

	// 6) Fan hub is a real mix: every channel present and no corner color.
	{
		dc := probeCtxOpaque(64, 64)
		pos, col := meshQuad()
		dc.DrawVertices(pos, col, render.VertexModeTriangleFan)
		r, g, b, a := sampleRGBA(dc, 24, 28)
		v := [3]float64{r, g, b}
		mix := true
		for _, c := range [][3]float64{colVR, colVG, colVB, colVY} {
			near := math.Abs(v[0]-c[0]) < 0.06 && math.Abs(v[1]-c[1]) < 0.06 && math.Abs(v[2]-c[2]) < 0.06
			if near {
				mix = false
			}
		}
		setDetail("hub=(%.3f,%.3f,%.3f) a=%.3f distinct=%v", r, g, b, a, mix)
		check("verts_fan_hub_mix",
			a > 0.99 && r > 0.10 && g > 0.08 && b > 0.10 && mix)
		dc.Close()
	}

	// 7) Nil colors use the solid paint; Ex still reports one triangle.
	{
		dc := probeCtxOpaque(64, 64)
		dc.SetRGBA(0.2, 0.9, 0.3, 1)
		pts := []render.Point{{X: 8, Y: 56}, {X: 56, Y: 56}, {X: 32, Y: 8}}
		res, err := dc.DrawVerticesEx(pts, nil, render.VertexModeTriangles, render.VertDrawOptions{})
		r, g, b, _ := sampleRGBA(dc, 32, 40)
		setDetail("res=%+v err=%v center=(%.3f,%.3f,%.3f)", res, err, r, g, b)
		check("verts_solid_paint",
			err == nil && res.Triangles == 1 && math.Abs(r-0.2) < 2.0/255 && math.Abs(g-0.9) < 2.0/255 && math.Abs(b-0.3) < 2.0/255)
		dc.Close()
	}

	// 8) Uniform per-vertex colors take the same AA Fill path as solid paint.
	{
		dcA := probeCtxOpaque(64, 64)
		dcB := probeCtxOpaque(64, 64)
		pts := []render.Point{{X: 8, Y: 56}, {X: 56, Y: 56}, {X: 32, Y: 8}}
		uni := meshColors([][3]float64{{0.2, 0.9, 0.3}, {0.2, 0.9, 0.3}, {0.2, 0.9, 0.3}})
		dcA.DrawVertices(pts, uni, render.VertexModeTriangles)
		dcB.SetRGBA(0.2, 0.9, 0.3, 1)
		dcB.DrawVertices(pts, nil, render.VertexModeTriangles)
		changed, max := imageDiff(dcA, dcB, 0)
		setDetail("changed=%d max=%d", changed, max)
		check("verts_uniform_matches_solid", changed == 0)
		dcA.Close()
		dcB.Close()
	}

	// 9) CTM path matches pre-transformed positions, and differs from no CTM.
	{
		local := []render.Point{{X: 10, Y: 40}, {X: 50, Y: 40}, {X: 30, Y: 12}}
		dcCtm := probeCtxOpaque(64, 64)
		dcCtm.Push()
		dcCtm.Translate(6, 7)
		dcCtm.Rotate(0.25)
		m := dcCtm.GetTransform()
		dcCtm.Pop()
		dcCtm.Push()
		dcCtm.Translate(6, 7)
		dcCtm.Rotate(0.25)
		dcCtm.SetRGBA(0.9, 0.4, 0.2, 1)
		dcCtm.DrawVertices(local, nil, render.VertexModeTriangles)
		dcCtm.Pop()

		pre := make([]render.Point, len(local))
		for i, p := range local {
			pre[i] = m.TransformPoint(p)
		}
		dcPre := probeCtxOpaque(64, 64)
		dcPre.SetRGBA(0.9, 0.4, 0.2, 1)
		dcPre.DrawVertices(pre, nil, render.VertexModeTriangles)

		dcPlain := probeCtxOpaque(64, 64)
		dcPlain.SetRGBA(0.9, 0.4, 0.2, 1)
		dcPlain.DrawVertices(local, nil, render.VertexModeTriangles)

		changed, max := imageDiff(dcCtm, dcPre, 1)
		plainChanged, _ := imageDiff(dcCtm, dcPlain, 2)
		setDetail("ctm-vs-pre changed=%d max=%d ctm-vs-plain=%d", changed, max, plainChanged)
		check("verts_ctm_matches_pretransformed", changed <= 20 && max <= 1 && plainChanged > 100)
		dcCtm.Close()
		dcPre.Close()
		dcPlain.Close()
	}

	// 10) Per-vertex alpha interpolates and composites over the destination.
	{
		dc := probeCtxOpaque(64, 64)
		pts := []render.Point{{X: 8, Y: 56}, {X: 56, Y: 56}, {X: 32, Y: 8}}
		cols := []render.RGBA{
			{R: 1, A: 1},
			{R: 1, A: 0},
			{R: 1, A: 1},
		}
		dc.DrawVertices(pts, cols, render.VertexModeTriangles)
		r, g, b, _ := sampleRGBA(dc, 32, 40)
		tr, tg, tb, _ := sampleRGBA(dc, 52, 50)
		setDetail("mid=(%.3f,%.3f,%.3f) right=(%.3f,%.3f,%.3f)", r, g, b, tr, tg, tb)
		check("verts_vertex_alpha_blends",
			r > 0.95 && g > 0.10 && g < 0.60 && b > 0.10 && b < 0.60 &&
				tr > 0.95 && tg > 0.60 && tb > 0.60)
		dc.Close()
	}

	// 11) Indexed Ex reports two triangles and degrades on CPU.
	{
		dc := probeCtxOpaque(64, 64)
		pos, col := meshQuad()
		res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: quadIndices},
			render.VertDrawOptions{})
		setDetail("res=%+v err=%v", res, err)
		check("mesh_ex_indexed_triangles",
			err == nil && !res.Skipped && res.Triangles == 2 && res.Degraded && res.Reason == render.VertCPUFallbackReason)
		dc.Close()
	}

	// 12) Short index lists are indexed meshes with no complete triangle.
	{
		ok := true
		pos, col := meshQuad()
		for _, idx := range [][]uint16{{0}, {0, 1}} {
			dc := probeCtxOpaque(64, 64)
			dc.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: idx})
			res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: idx}, render.VertDrawOptions{})
			if err != nil || !res.Skipped || res.Triangles != 0 || inkCount(dc, 64, 64) != 0 {
				ok = false
				setDetail("idx=%v res=%+v err=%v ink=%d", idx, res, err, inkCount(dc, 64, 64))
			}
			dc.Close()
		}
		check("mesh_short_indices_skip", ok)
	}

	// 13) Ex rejects bad indices in a complete group and in a trailing group.
	{
		ok := true
		pos, col := meshQuad()
		for _, idx := range [][]uint16{{0, 1, 9}, {0, 1, 2, 9}, {9}} {
			dc := probeCtxOpaque(64, 64)
			res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: idx}, render.VertDrawOptions{})
			if !errors.Is(err, render.ErrVertsBadIndex) || inkCount(dc, 64, 64) != 0 {
				ok = false
				setDetail("idx=%v res=%+v err=%v ink=%d", idx, res, err, inkCount(dc, 64, 64))
			}
			dc.Close()
		}
		check("mesh_bad_index_rejected", ok)
	}

	// 14) Old DrawMesh skips only groups with an out-of-range index.
	{
		pos, col := meshQuad()
		dcBad := probeCtxOpaque(64, 64)
		dcBad.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: []uint16{0, 1, 2, 1, 2, 9}})
		dcGood := probeCtxOpaque(64, 64)
		dcGood.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: []uint16{0, 1, 2}})
		changed, max := imageDiff(dcBad, dcGood, 0)
		setDetail("changed=%d max=%d", changed, max)
		check("mesh_old_bad_index_skips", changed == 0)
		dcBad.Close()
		dcGood.Close()
	}

	// 15) Mismatched color count is a solid fallback, not a dropped mesh.
	{
		dc := probeCtxOpaque(64, 64)
		dc.SetRGBA(0.85, 0.36, 0.86, 1)
		pos, _ := meshQuad()
		res, err := dc.DrawMeshEx(render.Mesh{
			Positions: pos,
			Colors:    []render.RGBA{{R: 1, A: 1}},
			Indices:   quadIndices,
		}, render.VertDrawOptions{})
		r, g, b, _ := sampleRGBA(dc, 40, 20)
		setDetail("res=%+v err=%v center=(%.3f,%.3f,%.3f)", res, err, r, g, b)
		check("mesh_mismatched_colors_solid",
			err == nil && !res.Skipped && res.Triangles == 2 && res.Degraded &&
				math.Abs(r-0.85) < 2.0/255 && math.Abs(g-0.36) < 2.0/255 && math.Abs(b-0.86) < 2.0/255)
		dc.Close()
	}

	// 16) A complete trailing group is ignored, not treated as an error.
	{
		dc := probeCtxOpaque(64, 64)
		pos, col := meshQuad()
		res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col,
			Indices: []uint16{0, 1, 2, 0}}, render.VertDrawOptions{})
		setDetail("res=%+v err=%v ink=%d", res, err, inkCount(dc, 64, 64))
		check("mesh_partial_tail",
			err == nil && !res.Skipped && res.Triangles == 1 && res.Degraded && inkCount(dc, 64, 64) > 0)
		dc.Close()
	}

	// 17) Nil/empty indices keep the historical triangle-list interpretation.
	{
		pos, col := meshQuad()
		pos = append(pos, pos[0], pos[2])
		col = append(col, col[0], col[2])
		ok := true
		for _, idx := range [][]uint16{nil, {}} {
			dc := probeCtxOpaque(64, 64)
			res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: idx}, render.VertDrawOptions{})
			if err != nil || res.Skipped || res.Triangles != 2 || !res.Degraded {
				ok = false
				setDetail("idx=%v res=%+v err=%v", idx, res, err)
			}
			dc.Close()
		}
		check("mesh_nil_indices_triangle_list", ok)
	}

	// 18) Single indexed triangle (3 indices / 6 bytes) draws.
	{
		dc := probeCtxOpaque(64, 64)
		pos, col := meshQuad()
		res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col,
			Indices: []uint16{0, 1, 2}}, render.VertDrawOptions{})
		setDetail("res=%+v err=%v ink=%d", res, err, inkCount(dc, 64, 64))
		check("mesh_single_triangle",
			err == nil && !res.Skipped && res.Triangles == 1 && res.Degraded && inkCount(dc, 64, 64) > 0)
		dc.Close()
	}

	// 19) Indexed and CPU-expanded rendering agree bit for bit.
	{
		pos, col := meshQuad()
		expPos := []render.Point{pos[0], pos[1], pos[2], pos[0], pos[2], pos[3]}
		expCol := []render.RGBA{col[0], col[1], col[2], col[0], col[2], col[3]}
		dcIdx := probeCtxOpaque(64, 64)
		dcIdx.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: quadIndices})
		dcExp := probeCtxOpaque(64, 64)
		dcExp.DrawVertices(expPos, expCol, render.VertexModeTriangles)
		changed, max := imageDiff(dcIdx, dcExp, 0)
		setDetail("changed=%d max=%d", changed, max)
		check("mesh_indexed_matches_expanded", changed == 0)
		dcIdx.Close()
		dcExp.Close()
	}

	return probes, failed
}

// ---------------------------------------------------------------------------
// Region pixel probes on the GPU snapshot.
// ---------------------------------------------------------------------------

func runLowPixelChecks(img image.Image, originX, originY, dpr float64) map[string]bool {
	res := map[string]bool{}
	if img == nil {
		return res
	}
	px := func(card int, x float64) int {
		return int((originX + cardAt[card][0] + contentX0 + x) * dpr)
	}
	py := func(card int, y float64) int {
		return int((originY + cardAt[card][1] + contentY0 + y) * dpr)
	}
	painted := func(card int, x, y, w, h float64) int {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		n := 0
		for yy := y0; yy < y1; yy++ {
			for xx := x0; xx < x1; xx++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if !valid {
					continue
				}
				if math.Abs(r-colCardBG[0]) > 0.06 || math.Abs(g-colCardBG[1]) > 0.06 ||
					math.Abs(b-colCardBG[2]) > 0.06 {
					n++
				}
			}
		}
		return n
	}
	countNear := func(card int, x, y, w, h float64, c [3]float64, tol float64) int {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		n := 0
		for yy := y0; yy < y1; yy++ {
			for xx := x0; xx < x1; xx++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if valid && math.Abs(r-c[0]) <= tol && math.Abs(g-c[1]) <= tol && math.Abs(b-c[2]) <= tol {
					n++
				}
			}
		}
		return n
	}
	mean := func(card int, x, y, w, h float64) (float64, float64, float64, int) {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		var sr, sg, sb float64
		n := 0
		for yy := y0; yy < y1; yy++ {
			for xx := x0; xx < x1; xx++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if valid {
					sr += r
					sg += g
					sb += b
					n++
				}
			}
		}
		if n == 0 {
			return 0, 0, 0, 0
		}
		return sr / float64(n), sg / float64(n), sb / float64(n), n
	}
	patchEqual := func(card int, ax, ay, bx, by, w, h float64) (bool, int) {
		ax0, ay0 := px(card, ax), py(card, ay)
		bx0, by0 := px(card, bx), py(card, by)
		n := 0
		for yy := 0; yy < int(h*dpr); yy++ {
			for xx := 0; xx < int(w*dpr); xx++ {
				ar, ag, ab, aa := img.At(ax0+xx, ay0+yy).RGBA()
				br, bg, bb, ba := img.At(bx0+xx, by0+yy).RGBA()
				n++
				if ar != br || ag != bg || ab != bb || aa != ba {
					return false, n
				}
			}
		}
		return true, n
	}

	// A: three corner bands keep their own channel (true gradient), and the
	// solid triangle is the paint color across its interior.
	{
		lr, lg, lb, ln := mean(0, 28, 138, 30, 34)
		rr, rg, rb, rn := mean(0, 116, 138, 30, 34)
		tr, tg, tb, tn := mean(0, 72, 62, 28, 24)
		res["a_gouraud_gradient"] = ln > 100 && rn > 100 && tn > 100 &&
			lr > lg+0.18 && lr > lb+0.18 &&
			rg > rr+0.18 && rg > rb+0.18 &&
			tb > tr+0.18 && tb > tg+0.18
		res["a_solid_amber_region"] = countNear(0, 240, 70, 110, 100, colSolid, 0.06) > 2000
		res["a_triangles_painted"] = painted(0, 20, 54, 340, 128) > 7000
	}

	// B: both fans paint; the rotated one is cyan at its center and the
	// unrotated top-left corner stays background (a translate-only bug would
	// paint there).
	{
		res["b_fan_painted"] = painted(1, 26, 58, 148, 122) > 6000
		res["b_rotated_painted"] = countNear(1, 226, 43, 148, 134, colRot, 0.06) > 2500
		res["b_rotated_absent_unrotated"] = painted(1, 242, 66, 6, 8) == 0
	}

	// C: the indexed patch is pixel-identical to the expanded patch, and the
	// single 3-index triangle is present.
	{
		same, compared := patchEqual(2, cQuad[0].X, cQuad[0].Y, cQuad[0].X+cShiftX, cQuad[0].Y, 124, 106)
		res["c_indexed_expanded_same"] = same && compared > 10000
		res["c_indexed_painted"] = painted(2, cQuad[0].X, cQuad[0].Y, 124, 106) > 10000
		res["c_single_green_region"] = countNear(2, 210, 156, 64, 26, colSingle, 0.06) > 300
	}

	// D: empty slots keep the slot background, the mismatch slot is solid
	// magenta, and no rejected mesh leaks into either empty slot.
	{
		area := 96 * 80
		res["d_short_empty"] = countNear(3, 30, 66, 96, 80, colSlotBG, 0.03) > int(float64(area)*0.97)
		res["d_bad_empty"] = countNear(3, 306, 66, 96, 80, colSlotBG, 0.03) > int(float64(area)*0.97)
		res["d_mismatch_solid"] = countNear(3, 164, 62, 100, 88, colMismatch, 0.06) > 6000
	}

	return res
}
