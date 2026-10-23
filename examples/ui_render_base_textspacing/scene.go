package main

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"

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
	contentX0    = 12.0
	contentY0    = 36.0
	contentW     = cardW - 2*contentX0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

var (
	colCardBG = [3]float64{0.10, 0.11, 0.13}
	colPanel  = [3]float64{0.14, 0.16, 0.19}
	colTextA  = [3]float64{0.93, 0.95, 0.98}
	colTextB  = [3]float64{0.55, 0.88, 0.78}
	colTextC  = [3]float64{0.95, 0.85, 0.45}
	colTextD  = [3]float64{0.75, 0.82, 1.00}
	colLabel  = [3]float64{0.65, 0.70, 0.78}
	colRule   = [3]float64{0.95, 0.66, 0.24}
	colTick   = [3]float64{0.18, 0.82, 0.86}
)

// ---------------------------------------------------------------------------
// Scene data (built once; both the widgets and the pixel oracle read it).
// ---------------------------------------------------------------------------

type spacingRow struct {
	label   string
	text    string
	ls      float64
	node    *rendering.RenderText
	x, y    float64
	width   float64 // TextLayout line width (the painted advance)
	measure float64 // RenderText.MeasureWidth for the same text
	rulerY  float64 // content-relative rule top
}

type spacingColumn struct {
	label      string
	node       *rendering.RenderText
	x, y       float64
	lines      []string
	lineWidths []float64
	lineStep   float64
}

type layoutData struct {
	fontPath string
	cjkPath  string

	face12 text.Face
	face16 text.Face
	face20 text.Face
	face22 text.Face
	face24 text.Face
	cjk16  text.Face
	cjk20  text.Face

	aRows []*spacingRow
	bRows []*spacingRow

	cPlain  *spacingColumn
	cSpaced *spacingColumn

	dPos *spacingRow
	dNeg *spacingRow
}

var lay = &layoutData{}

func loadCJKFace(points float64) (text.Face, string, error) {
	var lastErr error
	for _, path := range text.SystemFontCandidates(text.FontRoleCJK) {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		face, desc, err := rendering.LoadFaceByFamily(path, points)
		if err == nil && face != nil {
			return face, desc, nil
		}
		if err != nil {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no CJK font candidate on disk")
	}
	return nil, "", lastErr
}

func textNode(s string, face text.Face, size float64, c [3]float64) *rendering.RenderText {
	t := rendering.NewRenderText(s)
	t.SetFace(face)
	t.SetFontSize(size)
	t.R, t.G, t.B, t.A = c[0], c[1], c[2], 1
	return t
}

func lineWidth(t *rendering.RenderText) float64 {
	if t == nil {
		return 0
	}
	l := t.TextLayout()
	if l == nil || l.LineCount() == 0 {
		return 0
	}
	_, _, w, _, ok := l.Line(0)
	if !ok {
		return 0
	}
	return w
}

func makeRow(label, s string, ls float64, face text.Face, size, x, y float64, c [3]float64) *spacingRow {
	t := textNode(s, face, size, c)
	t.SetLetterSpacing(ls)
	_ = t.Layout(rendering.Loose(400, 80))
	return &spacingRow{
		label: label, text: s, ls: ls, node: t, x: x, y: y,
		width: lineWidth(t), measure: t.MeasureWidth(s), rulerY: y + 30,
	}
}

func makeColumn(label, s string, ls float64, face text.Face, x, y float64) *spacingColumn {
	t := textNode(s, face, 16, colTextC)
	t.SetMaxWidth(130)
	t.SetLetterSpacing(ls)
	_ = t.Layout(rendering.Loose(130, 300))
	l := t.TextLayout()
	col := &spacingColumn{label: label, node: t, x: x, y: y, lineStep: t.LineHeight()}
	if l != nil {
		for i := 0; i < l.LineCount(); i++ {
			start, end, w, _, ok := l.Line(i)
			if !ok {
				continue
			}
			col.lines = append(col.lines, l.Text[start:end])
			col.lineWidths = append(col.lineWidths, w)
		}
	}
	return col
}

func buildTextSpacingLayout() error {
	var err error
	if lay.face12, lay.fontPath, err = rendering.TryLoadDefaultFace(12); err != nil {
		return fmt.Errorf("default face @12: %w", err)
	}
	if lay.face16, _, err = rendering.TryLoadDefaultFace(16); err != nil {
		return fmt.Errorf("default face @16: %w", err)
	}
	if lay.face20, _, err = rendering.TryLoadDefaultFace(20); err != nil {
		return fmt.Errorf("default face @20: %w", err)
	}
	if lay.face22, _, err = rendering.TryLoadDefaultFace(22); err != nil {
		return fmt.Errorf("default face @22: %w", err)
	}
	if lay.face24, _, err = rendering.TryLoadDefaultFace(24); err != nil {
		return fmt.Errorf("default face @24: %w", err)
	}
	if lay.cjk16, lay.cjkPath, err = loadCJKFace(16); err != nil {
		return fmt.Errorf("CJK face @16: %w", err)
	}
	if lay.cjk20, _, err = loadCJKFace(20); err != nil {
		return fmt.Errorf("CJK face @22: %w", err)
	}

	// A: one Latin cluster row per spacing value. "H" has no kerning and no
	// trailing gap, so the ruler is exactly base + (clusters-1)*spacing.
	aSpecs := []struct {
		label string
		ls    float64
	}{
		{"ls -4", -4}, {"ls 0", 0}, {"ls +5", 5}, {"ls +10", 10},
	}
	for i, s := range aSpecs {
		r := makeRow(s.label, "HHHHHH", s.ls, lay.face22, 22, 96, 38+float64(i)*40, colTextA)
		lay.aRows = append(lay.aRows, r)
	}

	// B: CJK and Arabic (RTL). The rule follows the shaped line width; for
	// Arabic that is the painted advance, not the unshaped rune sum.
	lay.bRows = append(lay.bRows,
		makeRow("CJK 0", "间距测试", 0, lay.cjk20, 20, 96, 38, colTextB),
		makeRow("CJK +10", "间距测试", 10, lay.cjk20, 20, 96, 78, colTextB),
		makeRow("RTL 0", "مرحبا", 0, lay.face20, 20, 96, 118, colTextB),
		makeRow("RTL +10", "مرحبا", 10, lay.face20, 20, 96, 158, colTextB),
	)
	for _, r := range lay.bRows {
		r.rulerY = r.y + 30
	}

	// C: the same 24 Han clusters at the same 130px budget. Spacing makes the
	// wrap break earlier (3 lines → 5 lines), which is visible without labels.
	const cSrc = "间距测试字距换行演示文本内容长度对比多行回绕效果"
	lay.cPlain = makeColumn("ls 0", cSrc, 0, lay.cjk16, 20, 50)
	lay.cSpaced = makeColumn("ls +10", cSrc, 10, lay.cjk16, 244, 50)

	// D: deliberately warm the unspaced layout cache, then set spacing. The
	// painted result and both rulers must come from the rebuilt +8 layout.
	pos := textNode("HHHHHH", lay.face24, 24, colTextD)
	_ = pos.Layout(rendering.Loose(400, 80))
	pos.SetLetterSpacing(8)
	_ = pos.Layout(rendering.Loose(400, 80))
	lay.dPos = &spacingRow{
		label: "warm 0 → +8", text: "HHHHHH", ls: 8, node: pos, x: 96, y: 46,
		width: lineWidth(pos), measure: pos.MeasureWidth("HHHHHH"), rulerY: 80,
	}
	lay.dNeg = makeRow("ls -5", "HHHHHH", -5, lay.face24, 24, 96, 128, colTextD)
	lay.dNeg.rulerY = lay.dNeg.y + 34

	return nil
}

// ---------------------------------------------------------------------------
// Pixel probe specs (point probes). Region probes live in runLowPixelChecks.
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

func buildProbeSpecs() {
	probeSpecs = nil
	add := func(card int, key, desc string, x, y float64, c [3]float64) {
		probeSpecs = append(probeSpecs, probeSpec{
			card: card, key: key, desc: desc,
			pt: render.Point{X: x, Y: y}, want: solid(c),
		})
	}

	// A: each row's measured advance ends at the amber tick.
	for i, r := range lay.aRows {
		add(0, fmt.Sprintf("a_tick_%d", i), fmt.Sprintf("%s end tick", r.label),
			r.x+r.width, r.rulerY+1, colTick)
		add(0, fmt.Sprintf("a_rule_%d", i), fmt.Sprintf("%s width rule", r.label),
			r.x+4, r.rulerY+1, colRule)
	}

	// B: CJK/RTL rules and end ticks come from the shaped layout.
	for i, r := range lay.bRows {
		add(1, fmt.Sprintf("b_tick_%d", i), fmt.Sprintf("%s end tick", r.label),
			r.x+r.width, r.rulerY+1, colTick)
		add(1, fmt.Sprintf("b_rule_%d", i), fmt.Sprintf("%s width rule", r.label),
			r.x+4, r.rulerY+1, colRule)
	}

	// C: the spaced column has a fourth line; the plain column does not.
	plain4Y := lay.cPlain.y + 3*lay.cPlain.lineStep + 19
	add(2, "c_plain_rule_1", "plain first line rule", lay.cPlain.x+4, lay.cPlain.y+19, colRule)
	add(2, "c_spaced_rule_1", "spaced first line rule", lay.cSpaced.x+4, lay.cSpaced.y+19, colRule)
	add(2, "c_spaced_rule_4", "spaced fourth line rule", lay.cSpaced.x+4, plain4Y, colRule)
	add(2, "c_plain_4th_bg", "plain has no fourth line", lay.cPlain.x+4, plain4Y, colCardBG)
	add(2, "c_plain_limit", "plain 130px limit", lay.cPlain.x+130, 100, colTick)
	add(2, "c_spaced_limit", "spaced 130px limit", lay.cSpaced.x+130, 100, colTick)

	// D: measure and layout rulers must end at the same x on the +8 row.
	add(3, "d_measure_tick", "MeasureWidth end tick",
		lay.dPos.x+lay.dPos.measure, lay.dPos.y+34, colRule)
	add(3, "d_layout_tick", "TextLayout end tick",
		lay.dPos.x+lay.dPos.width, lay.dPos.y+42, colTick)
	add(3, "d_measure_rule", "MeasureWidth rule", lay.dPos.x+4, lay.dPos.y+35, colRule)
	add(3, "d_layout_rule", "TextLayout rule", lay.dPos.x+4, lay.dPos.y+43, colTick)
	add(3, "d_negative_tick", "negative spacing end tick",
		lay.dNeg.x+lay.dNeg.width, lay.dNeg.y+35, colRule)
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildTextSpacingScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A SetLetterSpacing：单行 -4 / 0 / +5 / +10", paintSpacingA)
	card(1, "B CJK / RTL：簇间隙不破形", paintSpacingB)
	card(2, "C 回绕：同宽 130px，字距改变断行", paintSpacingC)
	card(3, "D MeasureWidth = TextLayout = 绘制（缓存预热后改字距）", paintSpacingD)

	placeRow := func(card int, r *spacingRow) {
		shell.Body.Place(r.node,
			cardAt[card][0]+contentX0+r.x,
			cardAt[card][1]+contentY0+r.y)
	}
	for _, r := range lay.aRows {
		placeRow(0, r)
	}
	for _, r := range lay.bRows {
		placeRow(1, r)
	}
	placeRow(3, lay.dPos)
	placeRow(3, lay.dNeg)

	placeColumn := func(col *spacingColumn) {
		shell.Body.Place(col.node,
			cardAt[2][0]+contentX0+col.x,
			cardAt[2][1]+contentY0+col.y)
	}
	placeColumn(lay.cPlain)
	placeColumn(lay.cSpaced)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func drawRule(dc render.Canvas, x, y, w float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x, y, w, 2)
	_ = dc.Fill()
}

func drawTick(dc render.Canvas, x, y float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x-1.5, y-5, 3, 10)
	_ = dc.Fill()
}

func paintSpacingA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetFont(lay.face12)
	for _, r := range lay.aRows {
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(r.label, cx+14, cy+r.y+24)
		drawRule(dc, cx+r.x, cy+r.rulerY, r.width, colRule)
		drawTick(dc, cx+r.x+r.width, cy+r.rulerY+1, colTick)
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(fmt.Sprintf("w=%.1f", r.width), cx+r.x+r.width+8, cy+r.y+27)
	}
}

func paintSpacingB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetFont(lay.face12)
	for _, r := range lay.bRows {
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(r.label, cx+14, cy+r.y+24)
		drawRule(dc, cx+r.x, cy+r.rulerY, r.width, colRule)
		drawTick(dc, cx+r.x+r.width, cy+r.rulerY+1, colTick)
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(fmt.Sprintf("w=%.1f", r.width), cx+r.x+r.width+8, cy+r.y+27)
	}
}

func paintSpacingC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetFont(lay.face12)
	for _, col := range []*spacingColumn{lay.cPlain, lay.cSpaced} {
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(fmt.Sprintf("%s · %d lines", col.label, len(col.lineWidths)),
			cx+col.x, cy+30)
		dc.SetRGBA(colTick[0], colTick[1], colTick[2], 1)
		dc.DrawRectangle(cx+col.x+130, cy+40, 2, 150)
		_ = dc.Fill()
		for i, w := range col.lineWidths {
			y := cy + col.y + float64(i)*col.lineStep + 18
			drawRule(dc, cx+col.x, y, w, colRule)
			drawTick(dc, cx+col.x+w, y+1, colTick)
		}
	}
}

func paintSpacingD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetFont(lay.face12)
	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(lay.dPos.label, cx+14, cy+lay.dPos.y+24)
	dc.SetRGBA(colRule[0], colRule[1], colRule[2], 1)
	dc.DrawString("measure", cx+14, cy+lay.dPos.y+38)
	dc.SetRGBA(colTick[0], colTick[1], colTick[2], 1)
	dc.DrawString("layout", cx+14, cy+lay.dPos.y+51)

	drawRule(dc, cx+lay.dPos.x, cy+lay.dPos.y+34, lay.dPos.measure, colRule)
	drawTick(dc, cx+lay.dPos.x+lay.dPos.measure, cy+lay.dPos.y+34, colRule)
	drawRule(dc, cx+lay.dPos.x, cy+lay.dPos.y+42, lay.dPos.width, colTick)
	drawTick(dc, cx+lay.dPos.x+lay.dPos.width, cy+lay.dPos.y+42, colTick)
	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(fmt.Sprintf("w=%.1f", lay.dPos.width),
		cx+lay.dPos.x+lay.dPos.width+8, cy+lay.dPos.y+48)

	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(lay.dNeg.label, cx+14, cy+lay.dNeg.y+24)
	drawRule(dc, cx+lay.dNeg.x, cy+lay.dNeg.y+34, lay.dNeg.width, colRule)
	drawTick(dc, cx+lay.dNeg.x+lay.dNeg.width, cy+lay.dNeg.y+34, colRule)
	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(fmt.Sprintf("w=%.1f", lay.dNeg.width),
		cx+lay.dNeg.x+lay.dNeg.width+8, cy+lay.dNeg.y+30)
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

func paintInkBounds(face text.Face, s string, ls float64) (minX, maxX int, ok bool) {
	t := textNode(s, face, 20, [3]float64{0, 0, 0})
	t.SetLetterSpacing(ls)
	_ = t.Layout(rendering.Loose(500, 80))
	dc := probeCtxOpaque(500, 80)
	defer dc.Close()
	t.Paint(rendering.NewPaintContext(dc, 1))
	minX, _, maxX, _, ok = inkBounds(dc, 500, 80)
	return
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
	closeTo := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

	face16, path, err := rendering.TryLoadDefaultFace(16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: default font: %v\n", err)
		return nil, nil
	}
	face20, _, err := rendering.TryLoadDefaultFace(20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: default font @20: %v\n", err)
		return nil, nil
	}
	cjk16, _, err := loadCJKFace(16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: CJK font: %v\n", err)
		return nil, nil
	}
	_ = path

	newText := func(s string, face text.Face, size, ls float64) *rendering.RenderText {
		t := textNode(s, face, size, [3]float64{0, 0, 0})
		t.SetLetterSpacing(ls)
		_ = t.Layout(rendering.Loose(600, 120))
		return t
	}
	width := func(t *rendering.RenderText) float64 { return lineWidth(t) }

	// 1) Zero spacing is the plain layout, not an approximation.
	{
		plain := newText("HHHHHH", face20, 20, 0)
		zero := newText("HHHHHH", face20, 20, 0)
		setDetail("plain=%.4f zero=%.4f", width(plain), width(zero))
		check("spacing_zero_matches_plain", width(plain) == width(zero))
	}

	// 2/3) The width bump is exactly (clusters-1)*spacing for positive and
	// negative values — no trailing phantom and no clamp at zero.
	{
		plain := width(newText("HHHHHH", face20, 20, 0))
		pos := width(newText("HHHHHH", face20, 20, 6))
		neg := width(newText("HHHHHH", face20, 20, -4))
		setDetail("plain=%.4f pos=%.4f (want +30) neg=%.4f (want -20)", plain, pos, neg)
		check("spacing_positive_bump_exact", closeTo(pos-plain, 30))
		check("spacing_negative_bump_exact", closeTo(neg-plain, -20))
	}

	// 4) Spaced measure and layout share one shaped builder for Latin, Han
	// and RTL (clusters, ligatures and bidi reorder included).
	{
		ok := true
		var got []string
		for _, tc := range []struct {
			s    string
			face text.Face
		}{
			{"HHHHHH", face20}, {"间距测试", cjk16}, {"مرحبا", face20},
		} {
			t := newText(tc.s, tc.face, 20, 10)
			w, m := width(t), t.MeasureWidth(tc.s)
			got = append(got, fmt.Sprintf("%q w=%.4f m=%.4f", tc.s, w, m))
			if !closeTo(w, m) {
				ok = false
			}
		}
		setDetail("%s", strings.Join(got, "; "))
		check("spacing_measure_equals_layout", ok)
	}

	// 5) One cluster has no gap and therefore no trailing phantom.
	{
		w0 := width(newText("H", face20, 20, 0))
		w12 := width(newText("H", face20, 20, 12))
		setDetail("w0=%.4f w12=%.4f", w0, w12)
		check("spacing_single_cluster_no_trailing", closeTo(w0, w12))
	}

	// 6) Han clusters: 4 clusters → 3 gaps; the shaped pen of the last
	// cluster moves by exactly 30px at +10.
	{
		plainT := newText("间距测试", cjk16, 20, 0)
		spacedT := newText("间距测试", cjk16, 20, 10)
		plain, spaced := width(plainT), width(spacedT)
		pg, sg := plainT.TextLayout().LineGlyphs(0), spacedT.TextLayout().LineGlyphs(0)
		penDelta := 0.0
		if len(pg) > 0 && len(sg) > 0 {
			penDelta = (sg[len(sg)-1].X - sg[0].X) - (pg[len(pg)-1].X - pg[0].X)
		}
		setDetail("plain=%.4f spaced=%.4f penDelta=%.4f", plain, spaced, penDelta)
		check("spacing_cjk_cluster_bump", closeTo(spaced-plain, 30) && closeTo(penDelta, 30))
	}

	// 7) RTL: visual reorder must not swallow gaps; all Arabic glyphs are real
	// (GID != 0), and the spaced measure equals the shaped layout.
	{
		plainT := newText("مرحبا", face20, 20, 0)
		spacedT := newText("مرحبا", face20, 20, 10)
		plain, spaced := width(plainT), width(spacedT)
		glyphs := spacedT.TextLayout().LineGlyphs(0)
		allReal := len(glyphs) == 5
		for _, g := range glyphs {
			if g.GID == 0 {
				allReal = false
			}
		}
		setDetail("plain=%.4f spaced=%.4f bump=%.4f glyphs=%d real=%v measure=%.4f",
			plain, spaced, spaced-plain, len(glyphs), allReal, spacedT.MeasureWidth("مرحبا"))
		check("spacing_rtl_bump_and_glyphs",
			closeTo(spaced-plain, 40) && allReal && closeTo(spacedT.MeasureWidth("مرحبا"), spaced))
	}

	// 8) Wrap budget: the same Han paragraph goes 3 → 5 lines at +10, every
	// line stays within 130px, and no separator whitespace is introduced.
	{
		const src = "间距测试字距换行演示文本内容长度对比多行回绕效果"
		mk := func(ls float64) *rendering.RenderText {
			t := textNode(src, cjk16, 16, [3]float64{0, 0, 0})
			t.SetMaxWidth(130)
			t.SetLetterSpacing(ls)
			_ = t.Layout(rendering.Loose(130, 300))
			return t
		}
		plain, spaced := mk(0), mk(10)
		pl, sl := plain.TextLayout(), spaced.TextLayout()
		fit := sl != nil && sl.LineCount() > 0
		if fit {
			for i := 0; i < sl.LineCount(); i++ {
				_, _, w, _, ok := sl.Line(i)
				if !ok || w > 130.0001 {
					fit = false
				}
			}
		}
		setDetail("plainLines=%d spacedLines=%d fit=%v", pl.LineCount(), sl.LineCount(), fit)
		check("spacing_wrap_breaks_earlier",
			pl != nil && sl != nil && pl.LineCount() == 3 && sl.LineCount() == 5 && fit)
	}

	// 9) Negative spacing lets more clusters fit the same budget.
	{
		const src = "间距测试字距换行演示文本内容长度对比多行回绕效果"
		mk := func(ls float64) *rendering.RenderText {
			t := textNode(src, cjk16, 16, [3]float64{0, 0, 0})
			t.SetMaxWidth(130)
			t.SetLetterSpacing(ls)
			_ = t.Layout(rendering.Loose(130, 300))
			return t
		}
		plain, neg := mk(0), mk(-3)
		lp, ln := plain.TextLayout(), neg.TextLayout()
		ps, pe, pw, _, pok := lp.Line(0)
		ns, ne, nw, _, nok := ln.Line(0)
		more := nok && pok && (ne-ns) > (pe-ps) && nw <= 130.0001
		setDetail("plain0=%q neg0=%q widths=%.3f/%.3f", lp.Text[ps:pe], ln.Text[ns:ne], pw, nw)
		check("spacing_negative_fits_more", more && ln.LineCount() == 3)
	}

	// 10) Letter spacing is horizontal: multi-line height is unchanged.
	{
		a := newText("HH\nHH", face20, 20, 0)
		b := newText("HH\nHH", face20, 20, 8)
		sa := a.Layout(rendering.Loose(400, 400))
		sb := b.Layout(rendering.Loose(400, 400))
		setDetail("h0=%.4f h8=%.4f lines=%d/%d", sa.Height, sb.Height,
			a.TextLayout().LineCount(), b.TextLayout().LineCount())
		check("spacing_multiline_height_invariant",
			sa.Height == sb.Height && a.TextLayout().LineCount() == 2 && b.TextLayout().LineCount() == 2)
	}

	// 11) SetLetterSpacing invalidates the measure cache even when the new
	// spacing lands in the same 0.01 key bucket as the old one.
	{
		t := newText("HHHHHH", face20, 20, 0)
		w0 := t.MeasureWidth("HHHHHH")
		t.SetLetterSpacing(6)
		w1 := t.MeasureWidth("HHHHHH")
		want := width(t)
		setDetail("w0=%.4f w1=%.4f want=%.4f", w0, w1, want)
		check("spacing_measure_cache_updates", w1 > w0 && closeTo(w1, want))
	}

	// 12) A spacing change rebuilds the cached TextLayout generation.
	{
		t := newText("HHHHHH", face20, 20, 0)
		g0 := t.TextLayout().Generation
		t.SetLetterSpacing(6)
		g1 := t.TextLayout().Generation
		setDetail("generation %d -> %d", g0, g1)
		check("spacing_layout_generation_updates", g0 != g1 && closeTo(width(t), width(newText("HHHHHH", face20, 20, 6))))
	}

	// 13/14) Ink follows the layout: positive extends right only, negative
	// tightens right only; the first cluster never gets a leading gap.
	{
		plainMin, plainMax, pok := paintInkBounds(face20, "HHHHHH", 0)
		posMin, posMax, ok := paintInkBounds(face20, "HHHHHH", 10)
		negMin, negMax, nok := paintInkBounds(face20, "HHHHHH", -5)
		setDetail("plain=(%d,%d) pos=(%d,%d) neg=(%d,%d)", plainMin, plainMax, posMin, posMax, negMin, negMax)
		check("spacing_paint_positive_extends",
			pok && ok && posMin == plainMin && posMax > plainMax+20)
		check("spacing_paint_negative_tightens",
			pok && nok && negMin == plainMin && negMax < plainMax-10)
	}

	// 15) The parent boundary fingerprint includes spacing; a changed value
	// must re-record, not replay stale unspaced pixels.
	{
		cache := rendering.NewBoundaryCache()
		root := rendering.NewAbsoluteBox(200, 60)
		root.SetRepaintBoundary(true)
		child := textNode("HHHHHH", face16, 16, [3]float64{0, 0, 0})
		root.Place(child, 10, 10)
		root.Layout(rendering.Tight(200, 60))
		paint := func() {
			dc := render.NewContext(200, 60)
			defer dc.Close()
			dc.BeginFrame()
			dc.ClearWithColor(render.White)
			pc := rendering.NewPaintContext(dc, 1)
			pc.BoundaryCache, pc.UseBoundaryCache = cache, true
			cache.BeginFrame()
			root.Paint(pc)
		}
		paint()
		st0 := cache.Stats()
		rr0, skip0 := st0.FrameRerecord, st0.FrameSkip
		paint()
		st1 := cache.Stats()
		rr1, skip1 := st1.FrameRerecord, st1.FrameSkip
		child.SetLetterSpacing(6)
		paint()
		rr2 := cache.Stats().FrameRerecord
		setDetail("first=%d/%d clean=%d/%d afterSpacing=%d", rr0, skip0, rr1, skip1, rr2)
		check("spacing_boundary_cache_invalidates",
			rr0 == 1 && skip0 == 0 && rr1 == 0 && skip1 == 1 && rr2 > 0)
	}

	// 16) Warm unspaced cache, then change spacing before the first paint:
	// the layout must be rebuilt with spacing, not reused.
	{
		t := newText("HHHHHH", face20, 20, 0)
		w0 := width(t)
		t.SetLetterSpacing(8)
		g1 := t.TextLayout().Generation
		w1 := width(t)
		setDetail("warm=%.4f after=%.4f want=%.4f gen=%d", w0, w1, w0+40, g1)
		check("spacing_warm_cache_set_spacing", closeTo(w1, w0+40) && g1 > 0)
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
	ink := func(card int, x, y, w, h float64) int {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		n := 0
		for yy := y0; yy < y1; yy++ {
			for xx := x0; xx < x1; xx++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if !valid {
					continue
				}
				if 0.299*r+0.587*g+0.114*b > 0.45 {
					n++
				}
			}
		}
		return n
	}
	firstInkX := func(card int, x, y, w, h float64) int {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		best := -1
		for xx := x0; xx < x1; xx++ {
			for yy := y0; yy < y1; yy++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if valid && 0.299*r+0.587*g+0.114*b > 0.45 {
					if best < 0 || xx < best {
						best = xx
					}
					break
				}
			}
		}
		return best
	}
	maxInkX := func(card int, x, y, w, h float64) int {
		x0, y0 := px(card, x), py(card, y)
		x1, y1 := px(card, x+w), py(card, y+h)
		best := -1
		for xx := x1 - 1; xx >= x0; xx-- {
			for yy := y0; yy < y1; yy++ {
				r, g, b, valid := readPixel(img, xx, yy)
				if valid && 0.299*r+0.587*g+0.114*b > 0.45 {
					if xx > best {
						best = xx
					}
					break
				}
			}
		}
		return best
	}

	// A: first glyph ink starts at the same x for every spacing; the right
	// edge grows monotonically with positive spacing and shrinks with negative.
	{
		base := firstInkX(0, lay.aRows[0].x-2, lay.aRows[0].y, lay.aRows[0].width+2, 24)
		aligned := base >= 0
		for _, r := range lay.aRows {
			if got := firstInkX(0, r.x-2, r.y, r.width+2, 24); got != base {
				aligned = false
			}
		}
		negMax := maxInkX(0, lay.aRows[0].x, lay.aRows[0].y, lay.aRows[0].width, 24)
		zeroMax := maxInkX(0, lay.aRows[1].x, lay.aRows[1].y, lay.aRows[1].width, 24)
		pos5Max := maxInkX(0, lay.aRows[2].x, lay.aRows[2].y, lay.aRows[2].width, 24)
		pos10Max := maxInkX(0, lay.aRows[3].x, lay.aRows[3].y, lay.aRows[3].width, 24)
		res["a_first_ink_aligned"] = aligned
		res["a_widths_grow"] = negMax < zeroMax && zeroMax < pos5Max && pos5Max < pos10Max && pos10Max > negMax+30
	}

	// B: both scripts paint real ink, and +10 moves the right edge outward.
	{
		cjk0, cjk10 := lay.bRows[0], lay.bRows[1]
		rtl0, rtl10 := lay.bRows[2], lay.bRows[3]
		res["b_cjk_ink"] = ink(1, cjk0.x-2, cjk0.y, cjk0.width+2, 25) > 40
		res["b_rtl_ink"] = ink(1, rtl0.x-2, rtl0.y, rtl0.width+2, 30) > 40
		cjkGrow := maxInkX(1, cjk10.x, cjk10.y, cjk10.width, 25) > maxInkX(1, cjk0.x, cjk0.y, cjk0.width, 25)+20
		rtlGrow := maxInkX(1, rtl10.x, rtl10.y, rtl10.width, 30) > maxInkX(1, rtl0.x, rtl0.y, rtl0.width, 30)+30
		res["b_widths_grow"] = cjkGrow && rtlGrow
	}

	// C: the spaced column has a fourth line; the plain column's fourth slot
	// is empty, and neither column paints past its 130px budget.
	{
		fourthY := lay.cSpaced.y + 3*lay.cSpaced.lineStep
		bandY := fourthY + 4
		bandH := lay.cSpaced.lineStep - 6
		res["c_spaced_fourth_ink"] = ink(2, lay.cSpaced.x, bandY, 125, bandH) > 20
		res["c_plain_fourth_empty"] = ink(2, lay.cPlain.x, bandY, 125, lay.cPlain.lineStep-6) < 3
		res["c_no_ink_beyond_limit"] =
			ink(2, lay.cPlain.x+132, 45, 40, 140) == 0 &&
				ink(2, lay.cSpaced.x+132, 45, 40, 140) == 0
	}

	// D: negative spacing shortens the right edge; the +8 row has ink.
	{
		posMax := maxInkX(3, lay.dPos.x, lay.dPos.y, lay.dPos.width, 24)
		negMax := maxInkX(3, lay.dNeg.x, lay.dNeg.y, lay.dNeg.width, 24)
		res["d_positive_ink"] = ink(3, lay.dPos.x-2, lay.dPos.y, lay.dPos.width+2, 24) > 40
		res["d_negative_shorter"] = negMax >= 0 && posMax > negMax+30
	}

	return res
}
