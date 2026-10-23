package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Palette shared by the scene and the analytic pixel oracle.
var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colTitle  = [3]float64{0.80, 0.86, 0.94}
	colSub    = [3]float64{0.64, 0.72, 0.84}
	colBorder = [3]float64{0.32, 0.36, 0.44}
)

// Source atlas colors (left 16x16 quadrants, right 16x16 magenta).
var (
	srcRed     = [3]float64{230.0 / 255, 26.0 / 255, 26.0 / 255}
	srcGreen   = [3]float64{26.0 / 255, 204.0 / 255, 26.0 / 255}
	srcBlue    = [3]float64{26.0 / 255, 51.0 / 255, 230.0 / 255}
	srcYellow  = [3]float64{242.0 / 255, 204.0 / 255, 26.0 / 255}
	srcMagenta = [3]float64{240.0 / 255, 20.0 / 255, 240.0 / 255}

	checkerRed  = [3]float64{0.9, 0.2, 0.1}
	checkerBlue = [3]float64{0.1, 0.3, 0.9}
	solidGreen  = [3]float64{0.2, 0.9, 0.3}
)

// ---- card geometry (card-local) ----

const (
	// A
	aRectX, aRectY, aRectSize       = 20.0, 44.0, 160.0
	aPreviewX, aPreviewY, aPreviewW = 210.0, 62.0, 96.0

	// B: three sub-rects
	bRectY, bRectH          = 48.0, 116.0
	b1X, b2X, b3X           = 20.0, 150.0, 280.0
	bRectW                  = 118.0
	b3CX, b3CY              = 340.0, 106.0
	b3ImageQuadrantCenterQ  = 4.0 // image quadrant centers at 4 / 12
	b3ImageQuadrantCenterQ2 = 12.0

	// C: ring
	cCX, cCY, cR, cWidth = 100.0, 120.0, 60.0, 24.0

	// D: two sub-rects
	d1X, d1Y, d1W, d1H = 20.0, 48.0, 160.0, 116.0
	d2X, dY, d2W, d2H  = 210.0, 48.0, 180.0, 116.0
)

// ---------------------------------------------------------------------------
// Source atlas + custom pattern
// ---------------------------------------------------------------------------

var (
	patternSource    *render.ImageBuf
	patternSourceSHA string
)

// loadPatternSource reads the committed 32x16 atlas. Keeping it in testdata
// (instead of generating it in a loop here) follows the repo data rule and
// makes the golden traceable to one binary asset.
func loadPatternSource() (*render.ImageBuf, error) {
	path := filepath.Join("examples", "ui_render_base_imagepattern", "testdata", "pattern_atlas.png")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	patternSourceSHA = hex.EncodeToString(sum[:])[:16]
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 16 {
		return nil, fmt.Errorf("%s: %dx%d, want 32x16", path, b.Dx(), b.Dy())
	}
	return render.ImageBufFromImage(img), nil
}

// checkerPattern is the user-defined legacy Pattern used by the D card and
// the Brush/Paint conversion probes. Origin is the first checker cell.
type checkerPattern struct {
	ox, oy float64
}

func (p *checkerPattern) ColorAt(x, y float64) render.RGBA {
	cx := int((x - p.ox) / 16)
	cy := int((y - p.oy) / 16)
	if (cx+cy)%2 == 0 {
		return rgbaOf(checkerRed)
	}
	return rgbaOf(checkerBlue)
}

func rgbaOf(c [3]float64) render.RGBA {
	return render.RGBA{R: c[0], G: c[1], B: c[2], A: 1}
}

func nearRGBA(c render.RGBA, want [3]float64, tol float64) bool {
	return math.Abs(c.R-want[0]) <= tol &&
		math.Abs(c.G-want[1]) <= tol &&
		math.Abs(c.B-want[2]) <= tol &&
		math.Abs(c.A-1) <= tol
}

func overBG(c [3]float64, alpha float64) [3]float64 {
	return [3]float64{
		c[0]*alpha + colCardBG[0]*(1-alpha),
		c[1]*alpha + colCardBG[1]*(1-alpha),
		c[2]*alpha + colCardBG[2]*(1-alpha),
	}
}

// b3Matrix maps image-space to device-space for the rotated B3 pattern.
// The inverse is what ImagePattern stores; the pixel oracle computes the
// forward from the same angle so it does not reuse the inverse under test.
func b3Matrix(ox, oy float64) render.Matrix {
	return render.Translate(ox+b3CX, oy+b3CY).
		Multiply(render.Rotate(0.3)).
		Multiply(render.Scale(2, 2))
}

func b3Local(ix, iy float64) (float64, float64) {
	pt := b3Matrix(0, 0).TransformPoint(render.Pt(ix, iy))
	return pt.X, pt.Y
}

// ---------------------------------------------------------------------------
// Logic probes
// ---------------------------------------------------------------------------

var patternProbeKeys = []string{
	"imagepattern_kind",
	"imagepattern_tile",
	"imagepattern_srcregion",
	"imagepattern_anchor",
	"imagepattern_scale",
	"imagepattern_opacity",
	"imagepattern_clamp",
	"imagepattern_transform",
	"imagepattern_source_roundtrip",
	"imagepattern_nil",
	"solid_pattern",
	"brush_from_solid",
	"brush_from_custom",
	"pattern_from_solid",
	"pattern_from_custom",
	"paint_solid",
	"paint_custom",
	"paint_pattern_fallback",
}

func runPatternLogic() (ran, failed []string) {
	add := func(key string, ok bool) {
		ran = append(ran, key)
		if !ok {
			failed = append(failed, key)
		}
	}
	ip := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	src := ip.Source()
	add("imagepattern_kind", src.Image == patternSource &&
		src.SrcX == 0 && src.SrcY == 0 && src.SrcW == 16 && src.SrcH == 16 &&
		src.Inverse == render.Identity() && src.Opacity == 1 && !src.ClampToSource)

	add("imagepattern_tile",
		nearRGBA(ip.ColorAt(4, 4), srcRed, 1e-6) &&
			nearRGBA(ip.ColorAt(12, 4), srcGreen, 1e-6) &&
			nearRGBA(ip.ColorAt(4, 12), srcBlue, 1e-6) &&
			nearRGBA(ip.ColorAt(12, 12), srcYellow, 1e-6) &&
			nearRGBA(ip.ColorAt(20, 4), srcRed, 1e-6))

	right := render.NewImagePattern(patternSource, 16, 0, 16, 16)
	add("imagepattern_srcregion",
		nearRGBA(right.ColorAt(0, 0), srcMagenta, 1e-6) &&
			nearRGBA(right.ColorAt(8, 8), srcMagenta, 1e-6) &&
			nearRGBA(right.ColorAt(16, 0), srcMagenta, 1e-6))

	pa := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	pa.SetAnchor(40, 20)
	add("imagepattern_anchor",
		nearRGBA(pa.ColorAt(44, 24), srcRed, 1e-6) &&
			nearRGBA(pa.ColorAt(52, 24), srcGreen, 1e-6))

	pa.SetScale(2, 2)
	add("imagepattern_scale",
		nearRGBA(pa.ColorAt(44, 24), srcRed, 1e-6) &&
			nearRGBA(pa.ColorAt(56, 24), srcGreen, 1e-6))

	pa.SetOpacity(0.5)
	c := pa.ColorAt(44, 24)
	add("imagepattern_opacity",
		math.Abs(c.R-srcRed[0]) <= 1e-6 && math.Abs(c.G-srcRed[1]) <= 1e-6 &&
			math.Abs(c.B-srcRed[2]) <= 1e-6 && math.Abs(c.A-0.5) <= 1e-9)
	pa.SetOpacity(1)

	pa.SetClamp(true)
	out := pa.ColorAt(80, 24)
	pa.SetClamp(false)
	in := pa.ColorAt(80, 24)
	add("imagepattern_clamp", out.A == 0 && nearRGBA(in, srcRed, 1e-6))

	pt := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	m := render.Translate(100, 100).Multiply(render.Rotate(0.3)).Multiply(render.Scale(2, 2))
	pt.SetTransform(m)
	transformOK := true
	for _, q := range []struct {
		ix, iy float64
		want   [3]float64
	}{
		{4, 4, srcRed}, {12, 4, srcGreen}, {4, 12, srcBlue}, {12, 12, srcYellow},
	} {
		dev := m.TransformPoint(render.Pt(q.ix, q.iy))
		if !nearRGBA(pt.ColorAt(dev.X, dev.Y), q.want, 1e-6) {
			transformOK = false
		}
	}
	add("imagepattern_transform", transformOK)

	// A pattern rebuilt from its own description must sample the same texels:
	// this is what lets a display list replay a pattern bit-exactly, so the
	// inverse travels as stored instead of being inverted a second time.
	back := render.NewImagePatternFromSource(pt.Source())
	backOK := true
	for _, q := range []render.Point{{X: 120, Y: 90}, {X: 210, Y: 140}, {X: 305, Y: 260}, {X: 40, Y: 40}} {
		if back.ColorAt(q.X, q.Y) != pt.ColorAt(q.X, q.Y) {
			backOK = false
		}
	}
	add("imagepattern_source_roundtrip", backOK && back.Source() == pt.Source())

	np := render.NewImagePattern(nil, 0, 0, 16, 16)
	nc := np.ColorAt(1, 1)
	add("imagepattern_nil", nc == (render.RGBA{}))

	sp := render.NewSolidPattern(rgbaOf(solidGreen))
	add("solid_pattern", nearRGBA(sp.ColorAt(3, 4), solidGreen, 1e-9))

	b := render.BrushFromPattern(render.NewSolidPattern(rgbaOf(srcRed)))
	_, solidBrush := b.(render.SolidBrush)
	add("brush_from_solid", solidBrush && nearRGBA(b.ColorAt(0, 0), srcRed, 1e-9))

	chk := &checkerPattern{}
	cb := render.BrushFromPattern(chk)
	_, customBrush := cb.(render.CustomBrush)
	add("brush_from_custom", customBrush &&
		nearRGBA(cb.ColorAt(4, 4), checkerRed, 1e-9) &&
		nearRGBA(cb.ColorAt(20, 4), checkerBlue, 1e-9))

	ps := render.PatternFromBrush(render.Solid(rgbaOf(srcBlue)))
	_, solidPattern := ps.(*render.SolidPattern)
	add("pattern_from_solid", solidPattern && nearRGBA(ps.ColorAt(0, 0), srcBlue, 1e-9))

	pcb := render.PatternFromBrush(render.NewCustomBrush(func(x, y float64) render.RGBA {
		return chk.ColorAt(x, y)
	}))
	add("pattern_from_custom", nearRGBA(pcb.ColorAt(4, 4), checkerRed, 1e-9) &&
		nearRGBA(pcb.ColorAt(20, 4), checkerBlue, 1e-9))

	paint := render.NewPaint()
	start := paint.ColorAt(0, 0)
	paint.SetBrush(render.Solid(rgbaOf(srcRed)))
	sc, solid := paint.SolidColor()
	gb := paint.GetBrush()
	_, gbSolid := gb.(render.SolidBrush)
	add("paint_solid", start == render.Black && solid &&
		nearRGBA(sc, srcRed, 1e-9) && gbSolid && nearRGBA(gb.ColorAt(0, 0), srcRed, 1e-9))

	paint.SetBrush(render.Checkerboard(render.White, render.Black, 8))
	cc := paint.ColorAt(1, 1)
	_, paintSolid := paint.SolidColor()
	pcb2 := paint.GetBrush()
	add("paint_custom", cc == render.White && !paintSolid && paint.Pattern != nil &&
		pcb2.ColorAt(1, 1) == cc)

	fallback := &render.Paint{Pattern: chk}
	fc := fallback.ColorAt(4, 4)
	_, fbSolid := fallback.SolidColor()
	fg := fallback.GetBrush()
	add("paint_pattern_fallback", !fbSolid &&
		nearRGBA(fc, checkerRed, 1e-9) &&
		fg.ColorAt(4, 4) == fc)

	return ran, failed
}

// ---------------------------------------------------------------------------
// Pixel probes
// ---------------------------------------------------------------------------

type pixProbe struct {
	key  string
	desc string
	card int
	rect rect
	want [3]float64
	tol  float64
}

// patternPixProbes uses the same card geometry and transform as the paint
// callbacks. The oracle is the atlas quadrant / checker formula / card BG,
// not the Pattern implementation under test.
func patternPixProbes() []pixProbe {
	tol := 8.0 / 255
	out := []pixProbe{
		// A: 16x16 crop tiled from the rect origin.
		{key: "A_tile_tl", desc: "atlas crop red", card: 0, rect: rect{x: 24, y: 48, w: 1, h: 1}, want: srcRed, tol: tol},
		{key: "A_tile_tr", desc: "atlas crop green", card: 0, rect: rect{x: 32, y: 48, w: 1, h: 1}, want: srcGreen, tol: tol},
		{key: "A_tile_bl", desc: "atlas crop blue", card: 0, rect: rect{x: 24, y: 56, w: 1, h: 1}, want: srcBlue, tol: tol},
		{key: "A_tile_br", desc: "atlas crop yellow", card: 0, rect: rect{x: 32, y: 56, w: 1, h: 1}, want: srcYellow, tol: tol},
		{key: "A_repeat_1", desc: "tile 1 repeats red", card: 0, rect: rect{x: 40, y: 48, w: 1, h: 1}, want: srcRed, tol: tol},
		{key: "A_repeat_2", desc: "tile 1 repeats yellow", card: 0, rect: rect{x: 44, y: 56, w: 1, h: 1}, want: srcYellow, tol: tol},

		// B1: anchor 20,48 + scale 2 + opacity .5, tiled.
		{key: "B1_red", desc: "scaled red at alpha .5", card: 1, rect: rect{x: 24, y: 52, w: 1, h: 1}, want: overBG(srcRed, 0.5), tol: tol},
		{key: "B1_green", desc: "scaled green at alpha .5", card: 1, rect: rect{x: 40, y: 52, w: 1, h: 1}, want: overBG(srcGreen, 0.5), tol: tol},
		{key: "B1_blue", desc: "scaled blue at alpha .5", card: 1, rect: rect{x: 24, y: 72, w: 1, h: 1}, want: overBG(srcBlue, 0.5), tol: tol},
		{key: "B1_repeat", desc: "scaled tile repeats", card: 1, rect: rect{x: 84, y: 52, w: 1, h: 1}, want: overBG(srcRed, 0.5), tol: tol},

		// B2: clamp=true, only the first 32x32 tile; the rest stays card BG.
		{key: "B2_inside", desc: "clamp first tile red", card: 1, rect: rect{x: 154, y: 52, w: 1, h: 1}, want: srcRed, tol: tol},
		{key: "B2_outside", desc: "clamp OOB is card BG", card: 1, rect: rect{x: 210, y: 52, w: 1, h: 1}, want: colCardBG, tol: tol},

		// C: SetStrokePattern on a non-rect ring, phase anchored at card origin.
		{key: "C_top_red", desc: "ring top red quadrant", card: 2, rect: rect{x: 100, y: 52, w: 1, h: 1}, want: srcRed, tol: tol},
		{key: "C_top_blue", desc: "ring top blue quadrant", card: 2, rect: rect{x: 100, y: 60, w: 1, h: 1}, want: srcBlue, tol: tol},
		{key: "C_left_blue", desc: "ring left blue quadrant", card: 2, rect: rect{x: 32, y: 120, w: 1, h: 1}, want: srcBlue, tol: tol},
		{key: "C_left_yellow", desc: "ring left yellow quadrant", card: 2, rect: rect{x: 40, y: 120, w: 1, h: 1}, want: srcYellow, tol: tol},

		// D1: custom Pattern (checker 16px), origin at the rect top-left.
		{key: "D_checker_red", desc: "custom checker red", card: 3, rect: rect{x: 24, y: 52, w: 1, h: 1}, want: checkerRed, tol: tol},
		{key: "D_checker_blue", desc: "custom checker blue", card: 3, rect: rect{x: 40, y: 52, w: 1, h: 1}, want: checkerBlue, tol: tol},
		{key: "D_checker_blue_y", desc: "custom checker blue (y+16)", card: 3, rect: rect{x: 24, y: 68, w: 1, h: 1}, want: checkerBlue, tol: tol},
		{key: "D_checker_red_xy", desc: "custom checker red (x+16,y+16)", card: 3, rect: rect{x: 40, y: 68, w: 1, h: 1}, want: checkerRed, tol: tol},

		// D2: NewSolidPattern uses the inline solid fast path.
		{key: "D_solid", desc: "solid pattern green", card: 3, rect: rect{x: 220, y: 60, w: 1, h: 1}, want: solidGreen, tol: tol},
	}

	// B3: rotated + scaled pattern (field-masked GPU path). The sample
	// positions are the forward-mapped quadrant centers; the expected color
	// is the quadrant itself.
	for _, q := range []struct {
		key    string
		ix, iy float64
		want   [3]float64
	}{
		{"B3_red", b3ImageQuadrantCenterQ, b3ImageQuadrantCenterQ, srcRed},
		{"B3_green", b3ImageQuadrantCenterQ2, b3ImageQuadrantCenterQ, srcGreen},
		{"B3_blue", b3ImageQuadrantCenterQ, b3ImageQuadrantCenterQ2, srcBlue},
		{"B3_yellow", b3ImageQuadrantCenterQ2, b3ImageQuadrantCenterQ2, srcYellow},
	} {
		x, y := b3Local(q.ix, q.iy)
		out = append(out, pixProbe{
			key: q.key, desc: "rotated pattern quadrant", card: 1,
			rect: rect{x: x, y: y, w: 1, h: 1}, want: q.want, tol: tol,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// Scene
// ---------------------------------------------------------------------------

func buildPatternScene(shell *wrkit.ShellChrome) {
	cardWithBoundary := func(i int, title string, boundary bool, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.SetRepaintBoundary(boundary)
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, colTitle[0], colTitle[1], colTitle[2]),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	cardWithBoundary(0, "A  NewImagePattern + SetFillPattern", true, paintPatternTile)
	cardWithBoundary(1, "B  anchor / scale / opacity / clamp / transform", true, paintPatternTransform)
	cardWithBoundary(2, "C  SetStrokePattern — ring (non-rect)", true, paintPatternStroke)
	cardWithBoundary(3, "D  custom Pattern + solid Pattern + Paint", true, paintPatternCustom)

	label := func(i int, s string, size, x, y float64) {
		shell.Body.Place(wrkit.Label(s, size, colSub[0], colSub[1], colSub[2]),
			cardAt[i][0]+x, cardAt[i][1]+y)
	}

	// A
	label(0, "atlas 32×16", 10, 210, 44)
	label(0, "tile 16 px", 10, 210, 166)
	label(0, "16×16 crop", 10, 210, 184)

	// B
	label(1, "tile · s2 · α.5", 10, 20, 30)
	label(1, "clamp · OOB bg", 10, 150, 30)
	label(1, "rot 17° · s2", 10, 280, 30)

	// C
	label(2, "SetStrokePattern", 11, 190, 52)
	label(2, "ring r=60 · width 24", 10, 190, 74)
	label(2, "non-rect stroke path", 10, 190, 96)
	label(2, "phase = card local", 10, 190, 118)

	// D
	label(3, "custom Pattern · checker 16", 10, 20, 30)
	label(3, "NewSolidPattern · solid", 10, 210, 30)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func paintPatternTile(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	p := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	p.SetAnchor(ox+aRectX, oy+aRectY)
	dc.SetFillPattern(p)
	dc.DrawRectangle(ox+aRectX, oy+aRectY, aRectSize, aRectSize)
	_ = dc.Fill()

	srcRect := image.Rect(0, 0, 16, 16)
	dc.DrawImageEx(patternSource, render.DrawImageOptions{
		X: ox + aPreviewX, Y: oy + aPreviewY,
		DstWidth: aPreviewW, DstHeight: aPreviewW,
		SrcRect:       &srcRect,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	})
}

func paintPatternTransform(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	// B1: anchor + scale + opacity, tiled.
	p1 := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	p1.SetAnchor(ox+b1X, oy+bRectY)
	p1.SetScale(2, 2)
	p1.SetOpacity(0.5)
	p1.SetClamp(false)
	dc.SetFillPattern(p1)
	dc.DrawRectangle(ox+b1X, oy+bRectY, bRectW, bRectH)
	_ = dc.Fill()

	// B2: clamp=true, only the first 32x32 tile is painted.
	p2 := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	p2.SetAnchor(ox+b2X, oy+bRectY)
	p2.SetScale(2, 2)
	p2.SetOpacity(1)
	p2.SetClamp(true)
	dc.SetFillPattern(p2)
	dc.DrawRectangle(ox+b2X, oy+bRectY, bRectW, bRectH)
	_ = dc.Fill()

	// B3: full affine rotate+scale; non-axis-aligned so the N2 field mask
	// samples by inverse. The rect stays axis-aligned; the pattern rotates.
	p3 := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	p3.SetTransform(b3Matrix(ox, oy))
	dc.SetFillPattern(p3)
	dc.DrawRectangle(ox+b3X, oy+bRectY, bRectW, bRectH)
	_ = dc.Fill()

	// Thin solid borders make the clamp / rotated sub-rects legible.
	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.SetLineWidth(1)
	for _, x := range []float64{b1X, b2X, b3X} {
		dc.DrawRectangle(ox+x, oy+bRectY, bRectW, bRectH)
		_ = dc.Stroke()
	}
}

func paintPatternStroke(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	p := render.NewImagePattern(patternSource, 0, 0, 16, 16)
	p.SetAnchor(ox, oy)
	dc.SetStrokePattern(p)
	dc.SetLineWidth(cWidth)
	dc.DrawCircle(ox+cCX, oy+cCY, cR)
	_ = dc.Stroke()
}

func paintPatternCustom(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)

	chk := &checkerPattern{ox: ox + d1X, oy: oy + d1Y}
	dc.SetFillPattern(chk)
	dc.DrawRectangle(ox+d1X, oy+d1Y, d1W, d1H)
	_ = dc.Fill()

	dc.SetFillPattern(render.NewSolidPattern(rgbaOf(solidGreen)))
	dc.DrawRectangle(ox+d2X, oy+dY, d2W, d2H)
	_ = dc.Fill()
}
