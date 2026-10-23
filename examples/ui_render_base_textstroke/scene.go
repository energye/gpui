package main

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// ---------------------------------------------------------------------------
// House layout: 2×2 cards inside the shell body panel (body-relative coords).
// ---------------------------------------------------------------------------

const (
	cardW, cardH = 430.0, 232.0
	// Content region inside each card: the dark panel carries the title on
	// top, the capability demo lives in the inset area.
	contentX0, contentY0 = 12.0, 36.0
	contentW, contentH   = cardW - 2*contentX0, cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// ---------------------------------------------------------------------------
// Palette + demo geometry. Pixel probes read these same constants, so a
// palette edit can never desync the drawn pixels from the asserted ones.
// ---------------------------------------------------------------------------

const (
	glyphSize  = 64.0
	anchorSize = 30.0
	fracA      = 20.2
	fracB      = 20.8
	fracString = "MMMMMMMMMMMMMMMM"
)

var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}

	// A: three StrokeString line widths.
	colStrokeW4  = [3]float64{0.20, 0.85, 0.75}
	colStrokeW8  = [3]float64{0.95, 0.65, 0.22}
	colStrokeW16 = [3]float64{0.92, 0.36, 0.60}

	// B: anchored stroke text + anchor crosshair.
	colAnchorMark = [3]float64{0.95, 0.72, 0.25}
	colAnchorText = [3]float64{0.35, 0.65, 1.00}

	// C: TextPath -> fill / stroke / DrawString reference.
	colPathFill   = [3]float64{0.25, 0.85, 0.95}
	colPathStroke = [3]float64{0.75, 0.50, 1.00}
	colDrawRef    = [3]float64{0.95, 0.95, 0.60}

	// D: rotated outline copy + fractional-size rows.
	colUpright = [3]float64{0.95, 0.55, 0.18}
	colRotated = [3]float64{0.55, 0.90, 0.30}
	colFracA   = [3]float64{0.60, 0.78, 1.00}
	colFracB   = [3]float64{1.00, 0.62, 0.78}
)

// ---- card geometry (content-local) ----

const (
	glyphBaseline = 118.0
	aO1X          = 20.0
	aO2X          = 150.0
	aO3X          = 280.0

	cFillX   = 20.0
	cStrokeX = 150.0
	cDrawX   = 280.0

	dIX        = 60.0
	dIBaseline = 110.0
	fracRow1Y  = 158.0
	fracRow2Y  = 180.0
)

var anchorAt = [3][2]float64{
	{70, 60},   // (0, 0): text hangs right/down from the anchor
	{203, 100}, // (0.5, 0.5): text centered on the anchor
	{336, 150}, // (1, 1): text ends at the anchor
}

// ---------------------------------------------------------------------------
// Text layout: faces + geometry measured through the same APIs the cards use.
// Both the painter and the pixel-probe oracle read these numbers, so a font
// substitution can move both together (no hardcoded glyph pixels).
// ---------------------------------------------------------------------------

type strokeItem struct {
	x, baseline, width float64
	col                [3]float64
	bounds             image.Rectangle
}

type layoutData struct {
	fontPath  string
	face64    text.Face
	face30    text.Face
	faceFracA text.Face
	faceFracB text.Face

	a      [3]strokeItem
	cFill  strokeItem
	cStrk  strokeItem
	cDraw  strokeItem
	iBnds  image.Rectangle
	iUp    render.Point
	iCent  render.Point
	iTip   render.Point
	iEmpty render.Point
}

var lay = &layoutData{}

// measurePathBounds returns TextPath's bounds for s at (x, baseline).
func measurePathBounds(face text.Face, s string, x, baseline float64) image.Rectangle {
	dc := render.NewContext(int(contentW), int(contentH))
	defer dc.Close()
	dc.SetFont(face)
	p := dc.TextPath(s, x, baseline)
	if p == nil {
		return image.Rectangle{}
	}
	return p.Bounds()
}

// inkMask renders fn into a fresh content-sized context and returns the set of
// painted pixels (any alpha > 0). Used to derive card D's probe points from the
// same drawing code the card runs, instead of assuming glyph pixels.
func inkMask(fn func(*render.Context)) map[[2]int]bool {
	dc := render.NewContext(int(contentW), int(contentH))
	defer dc.Close()
	fn(dc)
	img := dc.Image()
	m := map[[2]int]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				m[[2]int{x, y}] = true
			}
		}
	}
	return m
}

// buildTextLayout loads the demo font and measures every glyph-dependent probe
// point. Returns an error when no usable font is available (single FontSource,
// required by StrokeString/TextPath).
func buildTextLayout() error {
	face64, path, err := rendering.TryLoadDefaultFace(glyphSize)
	if err != nil {
		return fmt.Errorf("default face: %w", err)
	}
	face30, _, err := rendering.TryLoadDefaultFace(anchorSize)
	if err != nil {
		return fmt.Errorf("default face @%v: %w", anchorSize, err)
	}
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		return fmt.Errorf("font source %s: %w", path, err)
	}
	lay.fontPath = path
	lay.face64 = face64
	lay.face30 = face30
	lay.faceFracA = src.Face(fracA)
	lay.faceFracB = src.Face(fracB)

	// A: three "O" strokes at widths 4/8/16.
	lay.a = [3]strokeItem{
		{x: aO1X, baseline: glyphBaseline, width: 4, col: colStrokeW4},
		{x: aO2X, baseline: glyphBaseline, width: 8, col: colStrokeW8},
		{x: aO3X, baseline: glyphBaseline, width: 16, col: colStrokeW16},
	}
	for i := range lay.a {
		lay.a[i].bounds = measurePathBounds(face64, "O", lay.a[i].x, lay.a[i].baseline)
		if lay.a[i].bounds.Empty() {
			return fmt.Errorf("card A: no TextPath bounds for %q", "O")
		}
	}

	// C: TextPath fill / stroke / DrawString reference.
	lay.cFill = strokeItem{x: cFillX, baseline: glyphBaseline, width: 0, col: colPathFill}
	lay.cStrk = strokeItem{x: cStrokeX, baseline: glyphBaseline, width: 6, col: colPathStroke}
	lay.cDraw = strokeItem{x: cDrawX, baseline: glyphBaseline, width: 0, col: colDrawRef}
	lay.cFill.bounds = measurePathBounds(face64, "O", lay.cFill.x, lay.cFill.baseline)
	lay.cStrk.bounds = measurePathBounds(face64, "O", lay.cStrk.x, lay.cStrk.baseline)
	lay.cDraw.bounds = measurePathBounds(face64, "O", lay.cDraw.x, lay.cDraw.baseline)
	if lay.cFill.bounds.Empty() || lay.cStrk.bounds.Empty() || lay.cDraw.bounds.Empty() {
		return fmt.Errorf("card C: no TextPath bounds for %q", "O")
	}

	// D: upright "I" plus a 90°-rotated copy about the glyph's bbox center.
	lay.iBnds = measurePathBounds(face64, "I", dIX, dIBaseline)
	if lay.iBnds.Empty() {
		return fmt.Errorf("card D: no TextPath bounds for %q", "I")
	}
	up := inkMask(func(dc *render.Context) { drawCardDInk(dc, -contentX0, -contentY0, false) })
	both := inkMask(func(dc *render.Context) { drawCardDInk(dc, -contentX0, -contentY0, true) })
	if len(up) == 0 || len(both) == 0 {
		return fmt.Errorf("card D: no ink in the offscreen reference")
	}
	// Upright sample: topmost painted pixel, nudged down/right into the stem.
	var upPt render.Point
	best := 1 << 30
	for p := range up {
		if p[1] < best || (p[1] == best && p[0] < int(upPt.X)) {
			best = p[1]
			upPt = render.Pt(float64(p[0]), float64(p[1]+3))
		}
	}
	// Tip sample: farthest-right pixel painted only by the rotated copy,
	// nudged left so AA never enters the probe.
	var tipPt render.Point
	lastX := -1 << 30
	for p := range both {
		if up[p] {
			continue
		}
		if p[0] > lastX {
			lastX = p[0]
			tipPt = render.Pt(float64(p[0]), float64(p[1]))
		}
	}
	if lastX == -1<<30 {
		return fmt.Errorf("card D: the rotated copy painted no extra ink")
	}
	tipPt.X -= 3
	// Overlap sample: the glyph bbox center (both copies paint it; the rotated
	// copy is drawn last, so its color must win).
	centPt := render.Pt(
		float64(lay.iBnds.Min.X+lay.iBnds.Max.X)/2,
		float64(lay.iBnds.Min.Y+lay.iBnds.Max.Y)/2,
	)
	// Empty sample: right of the rotated bar and above the fractional rows.
	emptyPt := render.Pt(float64(lay.iBnds.Max.X)+26, float64(lay.iBnds.Min.Y)+4)
	for d := 0; d < 40 && both[[2]int{int(emptyPt.X), int(emptyPt.Y)}]; d++ {
		emptyPt.Y -= 2
	}
	if p := [2]int{int(upPt.X), int(upPt.Y)}; !up[p] {
		return fmt.Errorf("card D: upright sample %v is not painted in the reference", upPt)
	}
	if p := [2]int{int(centPt.X), int(centPt.Y)}; !both[p] {
		return fmt.Errorf("card D: overlap sample %v is not painted in the reference", centPt)
	}
	if p := [2]int{int(tipPt.X), int(tipPt.Y)}; !both[p] {
		return fmt.Errorf("card D: rotated-only sample %v is not painted in the reference", tipPt)
	}
	if p := [2]int{int(emptyPt.X), int(emptyPt.Y)}; both[p] {
		return fmt.Errorf("card D: empty sample %v is painted in the reference", emptyPt)
	}
	lay.iUp, lay.iCent, lay.iTip, lay.iEmpty = upPt, centPt, tipPt, emptyPt
	return nil
}

// ---------------------------------------------------------------------------
// Pixel probe spec (end-of-run, on the GPU snapshot).
// ---------------------------------------------------------------------------

// band is an independent channel bound on the sampled pixel (0..1 scale), on
// top of the oracle match.
type band struct {
	ch       int
	min, max float64
}

// b8 builds a band in byte units for readability.
func b8(ch int, lo, hi byte) band {
	return band{ch: ch, min: float64(lo) / 255, max: float64(hi) / 255}
}

// probeSpec is one end-of-run pixel probe: card index, content-local sample
// point and the expected-color oracle (palette constants).
type probeSpec struct {
	key, desc string
	card      int
	pt        render.Point
	want      func(ox, oy, dpr float64, devPX, devPY int) (r, g, b float64)
	bands     []band
}

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

// autoBands brackets the expected color per channel: an independent sanity
// bound on top of the exact-color oracle.
func autoBands(c [3]float64, spread float64) []band {
	return []band{
		{ch: 0, min: c[0] - spread, max: c[0] + spread},
		{ch: 1, min: c[1] - spread, max: c[1] + spread},
		{ch: 2, min: c[2] - spread, max: c[2] + spread},
	}
}

func buildProbeSpecs() {
	probeSpecs = nil
	solid := func(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
		return func(float64, float64, float64, int, int) (float64, float64, float64) {
			return c[0], c[1], c[2]
		}
	}
	add := func(key, desc string, card int, pt render.Point, want [3]float64) {
		probeSpecs = append(probeSpecs, probeSpec{key, desc, card, pt, solid(want), autoBands(want, 0.1)})
	}

	for _, it := range lay.a {
		cy := float64(it.bounds.Min.Y+it.bounds.Max.Y) / 2
		cx := float64(it.bounds.Min.X+it.bounds.Max.X) / 2
		add(fmt.Sprintf("a_w%d_ring", int(it.width)),
			fmt.Sprintf("A 线宽 %d：轮廓外缘内 1px 为描边色（描边心在轮廓上）", int(it.width)),
			0, render.Pt(float64(it.bounds.Min.X+1), cy), it.col)
		add(fmt.Sprintf("a_w%d_center", int(it.width)),
			fmt.Sprintf("A 线宽 %d：字形中空（StrokeString 只描边不填充）", int(it.width)),
			0, render.Pt(cx, cy), colCardBG)
		add(fmt.Sprintf("a_w%d_out", int(it.width)),
			fmt.Sprintf("A 线宽 %d：半宽+4px 之外无墨（线宽不越界）", int(it.width)),
			0, render.Pt(float64(it.bounds.Min.X)-it.width/2-4, cy), colCardBG)
	}

	for i, a := range anchorAt {
		add(fmt.Sprintf("b_anchor%d", i),
			fmt.Sprintf("B 锚点 %v 十字标记中心 = 琥珀", i),
			1, render.Pt(a[0], a[1]), colAnchorMark)
	}

	cy := float64(lay.cFill.bounds.Min.Y+lay.cFill.bounds.Max.Y) / 2
	cx := float64(lay.cFill.bounds.Min.X+lay.cFill.bounds.Max.X) / 2
	add("c_fill_ring", "C FillPath(TextPath)：外缘内 1px 为填充色", 2,
		render.Pt(float64(lay.cFill.bounds.Min.X+1), cy), colPathFill)
	add("c_fill_counter", "C FillPath(TextPath)：字碗中空（孔洞未填）", 2,
		render.Pt(cx, cy), colCardBG)
	cy2 := float64(lay.cStrk.bounds.Min.Y+lay.cStrk.bounds.Max.Y) / 2
	cx2 := float64(lay.cStrk.bounds.Min.X+lay.cStrk.bounds.Max.X) / 2
	add("c_stroke_band", "C StrokePath(TextPath) 线宽 6：外缘内 1px 为描边色", 2,
		render.Pt(float64(lay.cStrk.bounds.Min.X+1), cy2), colPathStroke)
	add("c_stroke_counter", "C StrokePath(TextPath)：字碗中空（只描边）", 2,
		render.Pt(cx2, cy2), colCardBG)
	cy3 := float64(lay.cDraw.bounds.Min.Y+lay.cDraw.bounds.Max.Y) / 2
	add("c_draw_ring", "C DrawString(Vector) 对照：外缘内 1px 为参考色", 2,
		render.Pt(float64(lay.cDraw.bounds.Min.X+1), cy3), colDrawRef)

	add("d_up_stem", "D 直立 FillPath(TextPath)：字干上部为直立色", 3, lay.iUp, colUpright)
	add("d_rot_tip", "D 旋转 90° 副本：仅在旋转份出现的墨为旋转色", 3, lay.iTip, colRotated)
	add("d_overlap", "D 重叠处（后画者胜）：旋转色压过直立色", 3, lay.iCent, colRotated)
	add("d_empty", "D 旋转条右侧空白：无墨（旋转未把墨留在原位）", 3, lay.iEmpty, colCardBG)
}

// ---------------------------------------------------------------------------
// Logic probes (value-level, CPU offscreen, child process).
// ---------------------------------------------------------------------------

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

func near3(r, g, b, wr, wg, wb, tol float64) bool {
	return math.Abs(r-wr) <= tol && math.Abs(g-wg) <= tol && math.Abs(b-wb) <= tol
}

// inkBounds returns the bbox of pixels differing from the white background.
func inkBounds(dc *render.Context, w, h int) (minX, minY, maxX, maxY int, ok bool) {
	minX, minY, maxX, maxY = w, h, -1, -1
	img := dc.Image()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return minX, minY, maxX, maxY, maxX >= minX
}

func countInk(dc *render.Context, w, h int) int {
	n := 0
	img := dc.Image()
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
				n++
			}
		}
	}
	return n
}

var probeDetail string

func setDetail(format string, args ...any) { probeDetail = fmt.Sprintf(format, args...) }

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
	const tol = 3.0 / 255

	face64, path, err := rendering.TryLoadDefaultFace(glyphSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: font: %v\n", err)
		return nil, nil
	}
	src, err := text.NewFontSourceFromFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: font source: %v\n", err)
		return nil, nil
	}
	probeInk := func(size float64, draw func(*render.Context, text.Face)) (int, image.Rectangle) {
		dc := render.NewContext(300, 160)
		defer dc.Close()
		dc.ClearWithColor(render.White)
		draw(dc, src.Face(size))
		minX, minY, maxX, maxY, _ := inkBounds(dc, 300, 160)
		return countInk(dc, 300, 160), image.Rect(minX, minY, maxX+1, maxY+1)
	}

	// 1) StrokeString paints the ring and leaves the glyph counter empty:
	// outline stroke, never a fill.
	{
		dc := probeCtxOpaque(300, 160)
		dc.SetFont(face64)
		dc.SetRGB(0, 0, 0)
		dc.SetLineWidth(6)
		dc.StrokeString("O", 40, 120)
		b := measurePathBoundsIn(dc, "O", 40, 120)
		cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
		cr, cg, cb, _ := sampleRGBA(dc, cx, cy)
		rr, rg, rb, _ := sampleRGBA(dc, b.Min.X+1, cy)
		setDetail("ring=(%.3f,%.3f,%.3f) counter=(%.3f,%.3f,%.3f) bounds=%v", rr, rg, rb, cr, cg, cb, b)
		check("stroke_ring_without_fill", near3(cr, cg, cb, 1, 1, 1, tol) && rr < 0.2 && rg < 0.2 && rb < 0.2)
		_ = dc.Close()
	}
	// 2) Ink grows with the line width (a width-16 stroke covers more than 2×
	// the width-4 stroke).
	{
		ink4, _ := probeInk(glyphSize, func(dc *render.Context, f text.Face) {
			dc.SetFont(f)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(4)
			dc.StrokeString("O", 40, 120)
		})
		ink16, _ := probeInk(glyphSize, func(dc *render.Context, f text.Face) {
			dc.SetFont(f)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(16)
			dc.StrokeString("O", 40, 120)
		})
		setDetail("ink(w=4)=%d ink(w=16)=%d", ink4, ink16)
		check("stroke_width_scales_ink", ink4 > 0 && ink16 > ink4*3/2)
	}
	// 3) The stroke uses the current color (not the fill color / black).
	{
		dc := probeCtxOpaque(300, 160)
		dc.SetFont(face64)
		dc.SetRGB(colStrokeW8[0], colStrokeW8[1], colStrokeW8[2])
		dc.SetLineWidth(8)
		dc.StrokeString("O", 40, 120)
		b := measurePathBoundsIn(dc, "O", 40, 120)
		cy := (b.Min.Y + b.Max.Y) / 2
		r, g, bl, _ := sampleRGBA(dc, b.Min.X+1, cy)
		setDetail("got=(%.3f,%.3f,%.3f) want=(%.3f,%.3f,%.3f)", r, g, bl, colStrokeW8[0], colStrokeW8[1], colStrokeW8[2])
		check("stroke_uses_current_color", near3(r, g, bl, colStrokeW8[0], colStrokeW8[1], colStrokeW8[2], tol))
		_ = dc.Close()
	}
	// 4) StrokeString ignores the text mode (outlines are the draw plan).
	{
		inkFor := func(mode render.TextMode) map[[2]int]bool {
			dc := probeCtxOpaque(300, 160)
			defer dc.Close()
			dc.SetTextMode(mode)
			dc.SetFont(face64)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(6)
			dc.StrokeString("Ag", 40, 120)
			m := map[[2]int]bool{}
			img := dc.Image()
			for y := 0; y < 160; y++ {
				for x := 0; x < 300; x++ {
					r, g, b, _ := img.At(x, y).RGBA()
					if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
						m[[2]int{x, y}] = true
					}
				}
			}
			return m
		}
		vec := inkFor(render.TextModeVector)
		mask := inkFor(render.TextModeGlyphMask)
		same := len(vec) == len(mask) && len(vec) > 0
		if same {
			for p := range vec {
				if !mask[p] {
					same = false
					break
				}
			}
		}
		setDetail("vector px=%d glyphmask px=%d equal=%v", len(vec), len(mask), same)
		check("stroke_ignores_text_mode", same)
	}
	// 5) StrokeString preserves the caller's path (Stroke() still strokes it).
	{
		dc := probeCtxOpaque(300, 160)
		dc.SetFont(face64)
		dc.SetRGB(0, 0, 0)
		dc.SetLineWidth(2)
		dc.DrawRectangle(20, 20, 260, 120)
		dc.StrokeString("O", 40, 120)
		_ = dc.Stroke()
		// The rectangle's top edge must be stroked (y ≈ 20).
		r, g, b, _ := sampleRGBA(dc, 150, 20)
		setDetail("rect top edge sample=(%.3f,%.3f,%.3f) want dark", r, g, b)
		check("stroke_keeps_user_path", r < 0.5 && g < 0.5 && b < 0.5)
		_ = dc.Close()
	}
	// 6) Stroke damage covers the painted outline: the path bounds alone
	// under-report by half the line width, and a damage-tight layer composite
	// would clip the outer edge (2026-10-19 fix).
	{
		dc := render.NewContext(64, 64)
		defer dc.Close()
		dc.SetDamageTracking(true)
		dc.ResetFrameDamage()
		dc.SetRGB(1, 0, 0)
		dc.SetLineWidth(12)
		dc.DrawRectangle(20, 20, 24, 24)
		_ = dc.Stroke()
		d := dc.FrameDamageUnion()
		setDetail("damage=%v want cover 14..50", d)
		check("stroke_damage_covers_width",
			d.Min.X <= 14 && d.Min.Y <= 14 && d.Max.X >= 50 && d.Max.Y >= 50)
	}
	// 7) Anchored stroke text: (0,0) puts the ink's left/top at the anchor.
	{
		_, bb := probeInk(anchorSize, func(dc *render.Context, f text.Face) {
			dc.SetFont(f)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(2)
			dc.StrokeStringAnchored("Ag", 80, 90, 0, 0)
		})
		setDetail("bbox=%v want left/top ≈ (80,90)", bb)
		check("anchored_left_top", math.Abs(float64(bb.Min.X)-80) <= 3 && math.Abs(float64(bb.Min.Y)-90) <= 3)
	}
	// 8) (0.5,0.5) centers the ink on the anchor.
	{
		_, bb := probeInk(anchorSize, func(dc *render.Context, f text.Face) {
			dc.SetFont(f)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(2)
			dc.StrokeStringAnchored("Ag", 150, 90, 0.5, 0.5)
		})
		setDetail("bbox=%v center=(%d,%d) want (150,90)", bb, (bb.Min.X+bb.Max.X)/2, (bb.Min.Y+bb.Max.Y)/2)
		check("anchored_center", math.Abs(float64((bb.Min.X+bb.Max.X)/2)-150) <= 3 && math.Abs(float64((bb.Min.Y+bb.Max.Y)/2)-90) <= 3)
	}
	// 9) (1,1) puts the ink's right/bottom at the anchor.
	{
		_, bb := probeInk(anchorSize, func(dc *render.Context, f text.Face) {
			dc.SetFont(f)
			dc.SetRGB(0, 0, 0)
			dc.SetLineWidth(2)
			dc.StrokeStringAnchored("Ag", 220, 90, 1, 1)
		})
		setDetail("bbox=%v want right/bottom ≈ (220,90)", bb)
		check("anchored_right_bottom", math.Abs(float64(bb.Max.X)-220) <= 3 && math.Abs(float64(bb.Max.Y)-90) <= 3)
	}
	// 10) TextPath -> FillPath: the ring fills, the counter stays hollow
	// (the outline winding is carried through the path replay).
	{
		dc := probeCtxOpaque(300, 160)
		dc.SetFont(face64)
		dc.SetRGB(0, 0, 0)
		p := dc.TextPath("O", 40, 120)
		if p == nil {
			setDetail("TextPath returned nil")
			check("textpath_fill_hollow_counter", false)
		} else {
			if err := dc.FillPath(p); err != nil {
				setDetail("FillPath: %v", err)
				check("textpath_fill_hollow_counter", false)
			} else {
				b := p.Bounds()
				cx, cy := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
				r, g, bl, _ := sampleRGBA(dc, b.Min.X+2, cy)
				cr, cg, cb, _ := sampleRGBA(dc, cx, cy)
				setDetail("ring=(%.3f,%.3f,%.3f) counter=(%.3f,%.3f,%.3f)", r, g, bl, cr, cg, cb)
				check("textpath_fill_hollow_counter", r < 0.3 && g < 0.3 && bl < 0.3 && cr > 0.95 && cg > 0.95 && cb > 0.95)
			}
		}
		_ = dc.Close()
	}
	// 11) TextPath filled == DrawString(vector): same outlines, same place.
	{
		bPath := func() image.Rectangle {
			dc := probeCtxOpaque(300, 160)
			defer dc.Close()
			dc.SetFont(face64)
			dc.SetRGB(0, 0, 0)
			_ = dc.FillPath(dc.TextPath("O", 40, 120))
			minX, minY, maxX, maxY, _ := inkBounds(dc, 300, 160)
			return image.Rect(minX, minY, maxX+1, maxY+1)
		}()
		bDraw := func() image.Rectangle {
			dc := probeCtxOpaque(300, 160)
			defer dc.Close()
			dc.SetTextMode(render.TextModeVector)
			dc.SetFont(face64)
			dc.SetRGB(0, 0, 0)
			dc.DrawString("O", 40, 120)
			minX, minY, maxX, maxY, _ := inkBounds(dc, 300, 160)
			return image.Rect(minX, minY, maxX+1, maxY+1)
		}()
		close := func(a, b int) bool {
			d := a - b
			if d < 0 {
				d = -d
			}
			return d <= 1
		}
		setDetail("path=%v draw=%v", bPath, bDraw)
		check("textpath_matches_drawstring",
			close(bPath.Min.X, bDraw.Min.X) && close(bPath.Min.Y, bDraw.Min.Y) &&
				close(bPath.Max.X, bDraw.Max.X) && close(bPath.Max.Y, bDraw.Max.Y))
	}
	// 12) TextPath output follows the current transform.
	{
		bboxOf := func(rot bool) image.Rectangle {
			dc := probeCtxOpaque(300, 160)
			defer dc.Close()
			dc.SetFont(face64)
			dc.SetRGB(0, 0, 0)
			if rot {
				dc.Push()
				dc.Translate(80, 80)
				dc.Rotate(math.Pi / 2)
				dc.Translate(-80, -80)
			}
			_ = dc.FillPath(dc.TextPath("I", 75, 120))
			if rot {
				dc.Pop()
			}
			minX, minY, maxX, maxY, _ := inkBounds(dc, 300, 160)
			return image.Rect(minX, minY, maxX+1, maxY+1)
		}
		up, r90 := bboxOf(false), bboxOf(true)
		setDetail("up=%v rot=%v", up, r90)
		check("textpath_follows_transform",
			up.Dx() > 0 && up.Dy() > 0 && r90.Dx() > 0 && r90.Dy() > 0 &&
				math.Abs(float64(r90.Dx()-up.Dy())) <= 3 && math.Abs(float64(r90.Dy()-up.Dx())) <= 3)
	}
	// 13) Fractional ppem sizes keep distinct outlines: the 26.6 cache key
	// fix (2026-10-19). The larger size's ink span must visibly exceed the
	// smaller one (the old int16 key made both spans identical).
	{
		spanAt := func(size float64) float64 {
			dc := render.NewContext(300, 80)
			defer dc.Close()
			dc.SetFont(src.Face(size))
			p := dc.TextPath("MMMM", 20, 50)
			if p == nil {
				return -1
			}
			minX, maxX := 1e9, -1e9
			p.Iterate(func(_ render.PathVerb, coords []float64) {
				for i := 0; i+1 < len(coords); i += 2 {
					if coords[i] < minX {
						minX = coords[i]
					}
					if coords[i] > maxX {
						maxX = coords[i]
					}
				}
			})
			return maxX - minX
		}
		s1, s2 := spanAt(fracA), spanAt(fracB)
		setDetail("span(%.1f)=%.3f span(%.1f)=%.3f want +1.5..3.5", fracA, s1, fracB, s2)
		check("textpath_fractional_sizes_distinct", s1 > 0 && (s2-s1) > 1.5 && (s2-s1) < 3.5)
	}
	// 14) No face -> no path (TextPath contract).
	{
		dc := render.NewContext(80, 40)
		defer dc.Close()
		p := dc.TextPath("A", 10, 30)
		setDetail("path=%v want nil", p)
		check("textpath_no_face_nil", p == nil)
	}

	return probes, failed
}

// measurePathBoundsIn returns TextPath's bounds on an existing context (the
// logic probes need the same face/state they just drew with).
func measurePathBoundsIn(dc *render.Context, s string, x, y float64) image.Rectangle {
	p := dc.TextPath(s, x, y)
	if p == nil {
		return image.Rectangle{}
	}
	return p.Bounds()
}

// ---------------------------------------------------------------------------
// Scene.
// ---------------------------------------------------------------------------

func buildTextStrokeScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A StrokeString 线宽 4/8/16（只描边，不填充）", paintStrokeWidths)
	card(1, "B StrokeStringAnchored 锚点 (0,0)/(0.5,0.5)/(1,1)", paintAnchors)
	card(2, "C TextPath → FillPath / StrokePath / DrawString 对照", paintTextPath)
	card(3, "D TextPath 变换（90° 旋转）+ 分数字号 20.2/20.8", paintTransform)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func fillPlus(dc render.Canvas, x, y, arm float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x-arm, y-1, arm*2, 2)
	_ = dc.Fill()
	dc.DrawRectangle(x-1, y-arm, 2, arm*2)
	_ = dc.Fill()
	dc.DrawRectangle(x-2, y-2, 4, 4)
	_ = dc.Fill()
}

func paintStrokeWidths(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	dc.SetFont(lay.face64)
	for _, it := range lay.a {
		dc.SetRGBA(it.col[0], it.col[1], it.col[2], 1)
		dc.SetLineWidth(it.width)
		dc.StrokeString("O", cx+it.x, cy+it.baseline)
	}
}

func paintAnchors(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	dc.SetFont(lay.face30)
	dc.SetRGBA(colAnchorText[0], colAnchorText[1], colAnchorText[2], 1)
	dc.SetLineWidth(2)
	for i, a := range anchorAt {
		ax, ay := 0.0, 0.0
		switch i {
		case 1:
			ax, ay = 0.5, 0.5
		case 2:
			ax, ay = 1, 1
		}
		dc.StrokeStringAnchored("Ag", cx+a[0], cy+a[1], ax, ay)
	}
	for _, a := range anchorAt {
		fillPlus(dc, cx+a[0], cy+a[1], 9, colAnchorMark)
	}
}

func paintTextPath(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	// TextPath extracts outlines through the context's own glyph cache, so this
	// card only exists on the live context; a recording walk refuses and counts.
	dc, ok := pc.RequireLiveContext()
	if !ok {
		return
	}
	ox, oy := pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	dc.SetFont(lay.face64)

	dc.SetRGBA(colPathFill[0], colPathFill[1], colPathFill[2], 1)
	_ = dc.FillPath(dc.TextPath("O", cx+lay.cFill.x, cy+lay.cFill.baseline))

	dc.SetRGBA(colPathStroke[0], colPathStroke[1], colPathStroke[2], 1)
	dc.SetLineWidth(lay.cStrk.width)
	_ = dc.StrokePath(dc.TextPath("O", cx+lay.cStrk.x, cy+lay.cStrk.baseline))

	dc.SetTextMode(render.TextModeVector)
	dc.SetRGBA(colDrawRef[0], colDrawRef[1], colDrawRef[2], 1)
	dc.DrawString("O", cx+lay.cDraw.x, cy+lay.cDraw.baseline)
	dc.SetTextMode(render.TextModeAuto)
}

// drawCardDInk paints card D's glyphs (no card background) at the given
// origin. Shared by the paint callback and the offscreen probe-point oracle.
func drawCardDInk(dc *render.Context, ox, oy float64, rotated bool) {
	cx, cy := ox+contentX0, oy+contentY0
	dc.SetFont(lay.face64)
	dc.SetRGBA(colUpright[0], colUpright[1], colUpright[2], 1)
	_ = dc.FillPath(dc.TextPath("I", cx+dIX, cy+dIBaseline))
	if !rotated {
		return
	}
	// Second copy rotated 90° about the glyph bbox center: the outline path is
	// replayed through the current transform (Skia skia_path + canvas matrix).
	b := lay.iBnds
	mcx, mcy := float64(b.Min.X+b.Max.X)/2, float64(b.Min.Y+b.Max.Y)/2
	dc.Push()
	dc.Translate(cx+mcx, cy+mcy)
	dc.Rotate(math.Pi / 2)
	dc.Translate(-(cx + mcx), -(cy + mcy))
	dc.SetRGBA(colRotated[0], colRotated[1], colRotated[2], 1)
	_ = dc.FillPath(dc.TextPath("I", cx+dIX, cy+dIBaseline))
	dc.Pop()
}

func paintTransform(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	// Card D is drawn from extracted outlines: live-only, counted refusal.
	if live, ok := pc.RequireLiveContext(); ok {
		drawCardDInk(live, ox, oy, true)
	}

	// Fractional ppem rows: two sizes that used to share one cached outline
	// (int16 key) and now stay distinct (26.6 fixed point).
	cx, cy := ox+contentX0, oy+contentY0
	dc.SetFont(lay.faceFracA)
	dc.SetRGBA(colFracA[0], colFracA[1], colFracA[2], 1)
	dc.DrawString(fracString, cx+16, cy+fracRow1Y)
	dc.SetFont(lay.faceFracB)
	dc.SetRGBA(colFracB[0], colFracB[1], colFracB[2], 1)
	dc.DrawString(fracString, cx+16, cy+fracRow2Y)
}
