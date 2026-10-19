package main

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
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
	contentH     = cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// ---------------------------------------------------------------------------
// Palette + demo geometry. Pixel probes read these same constants, so a
// palette edit can never desync the drawn pixels from the asserted ones.
// ---------------------------------------------------------------------------

var (
	colCardBG = [3]float64{0.12, 0.135, 0.165}
	colPanel  = [3]float64{0.17, 0.19, 0.235}

	colMeasureBox  = [3]float64{0.24, 0.28, 0.36}
	colMeasureMark = [3]float64{1.00, 0.63, 0.20}
	colAnchorMark  = [3]float64{0.98, 0.82, 0.25}
	colAnchorText  = [3]float64{0.55, 0.78, 1.00}
	colMeasureText = [3]float64{0.92, 0.95, 1.00}
	colLabel       = [3]float64{0.76, 0.82, 0.90}

	colWrapText  = [3]float64{0.72, 0.88, 1.00}
	colWrapRule  = [3]float64{0.30, 0.95, 0.75}
	colWrapLimit = [3]float64{1.00, 0.30, 0.35}

	colModeText = [3]float64{0.92, 0.95, 1.00}
	colModeRule = [3]float64{0.20, 0.23, 0.29}

	colLCDPanel = [3]float64{0.96, 0.96, 0.93}
	colLCDInk   = [3]float64{0.02, 0.02, 0.02}
)

const (
	aText      = "Measure\nmultiline\nblock"
	aLineSpace = 1.2
	aBoxX      = 18.0
	aBoxY      = 42.0
	aBoxPad    = 5.0
	aWrapWidth = 160.0
	aAnchorW   = 110.0

	bSrc      = "The quick brown fox jumps over the lazy dog"
	bWidth    = 180.0
	bLeftX    = 20.0
	bBaseY    = 42.0
	bLineStep = 20.0
	bRow2Base = 142.0
)

type anchorSpec struct {
	x, y   float64
	ax, ay float64
	align  render.Align
}

var anchorSpecs = [3]anchorSpec{
	{x: 236, y: 42, ax: 0, ay: 0, align: render.AlignLeft},
	{x: 300, y: 104, ax: 0.5, ay: 0.5, align: render.AlignCenter},
	{x: 365, y: 166, ax: 1, ay: 1, align: render.AlignRight},
}

type modeRow struct {
	mode     render.TextMode
	label    string
	baseline float64
}

var modeRows = []modeRow{
	{render.TextModeAuto, "Auto", 32},
	{render.TextModeMSDF, "MSDF", 60},
	{render.TextModeVector, "Vector", 88},
	{render.TextModeBitmap, "Bitmap", 116},
	{render.TextModeGlyphMask, "Glyph", 144},
	{render.TextModeAliased, "Alias", 172},
}

type lcdRow struct {
	layout render.LCDLayout
	label  string
	panelY float64
}

var lcdRows = []lcdRow{
	{render.LCDLayoutNone, "None", 16},
	{render.LCDLayoutRGB, "RGB", 62},
	{render.LCDLayoutBGR, "BGR", 108},
}

const (
	lcdPanelX = 64.0
	lcdPanelW = 330.0
	lcdPanelH = 38.0
	lcdTextX  = 80.0
)

// ---------------------------------------------------------------------------
// Text layout: faces + geometry measured through the same APIs the cards use.
// ---------------------------------------------------------------------------

type layoutData struct {
	fontPath string
	face12   text.Face
	face16   text.Face
	face20   text.Face
	face24   text.Face

	aW, aH float64
	aLines int

	bLines       []string
	bLineWidths  []float64
	bRow2Text    string
	bRow2Width   float64
	bRow2LineW   float64
	bRow2LimitX  float64
	bRow2BaseY   float64
	bRow2RuleEnd float64
}

var lay = &layoutData{}

func buildTextLayout() error {
	var err error
	lay.face12, lay.fontPath, err = rendering.TryLoadDefaultFace(12)
	if err != nil {
		return fmt.Errorf("default face @12: %w", err)
	}
	if lay.face16, _, err = rendering.TryLoadDefaultFace(16); err != nil {
		return fmt.Errorf("default face @16: %w", err)
	}
	if lay.face20, _, err = rendering.TryLoadDefaultFace(20); err != nil {
		return fmt.Errorf("default face @20: %w", err)
	}
	if lay.face24, _, err = rendering.TryLoadDefaultFace(24); err != nil {
		return fmt.Errorf("default face @24: %w", err)
	}

	// Card A: measure the exact block the painter draws.
	dc := render.NewContext(256, 180)
	defer dc.Close()
	dc.SetFont(lay.face16)
	lay.aW, lay.aH = dc.MeasureMultilineString(aText, aLineSpace)
	lay.aLines = len(splitLines(aText))

	// Card B: wrap through the public API; underline widths are measured from
	// the returned line text, so a leaked trailing space extends the rule.
	dc.SetFont(lay.face16)
	lay.bLines = dc.WordWrap(bSrc, bWidth)
	lay.bLineWidths = make([]float64, len(lay.bLines))
	for i, line := range lay.bLines {
		lay.bLineWidths[i], _ = dc.MeasureString(line)
	}

	word := "word"
	wordW, _ := dc.MeasureString(word)
	spaceW, _ := dc.MeasureString(" ")
	// Deliberately half a space too narrow for the separator: the visible word
	// fits, the trailing space does not. This is the regression case.
	lay.bRow2Width = wordW + spaceW*0.25
	lay.bRow2Text = dc.WordWrap("word ", lay.bRow2Width)[0]
	lay.bRow2LineW, _ = dc.MeasureString(lay.bRow2Text)
	lay.bRow2LimitX = bLeftX + lay.bRow2Width
	lay.bRow2BaseY = bRow2Base
	lay.bRow2RuleEnd = bLeftX + lay.bRow2LineW

	return nil
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.Split(s, "\n")
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

const tolSolid = 8.0 / 255

var probeSpecs []probeSpec

func solid(c [3]float64) func(float64, float64, float64, int, int) (float64, float64, float64) {
	return func(float64, float64, float64, int, int) (float64, float64, float64) {
		return c[0], c[1], c[2]
	}
}

func buildProbeSpecs() {
	probeSpecs = nil
	add := func(card int, key, desc string, pt render.Point, c [3]float64) {
		probeSpecs = append(probeSpecs, probeSpec{card: card, key: key, desc: desc, pt: pt, want: solid(c)})
	}

	// A: measured box corners + three anchors.
	boxW, boxH := lay.aW+2*aBoxPad, lay.aH+2*aBoxPad
	add(0, "measure_box_tl", "MeasureMultilineString box top-left", render.Point{X: aBoxX, Y: aBoxY}, colMeasureMark)
	add(0, "measure_box_right", "measured width top-right marker", render.Point{X: aBoxX + boxW, Y: aBoxY}, colMeasureMark)
	add(0, "measure_box_bottom", "measured height bottom-left marker", render.Point{X: aBoxX, Y: aBoxY + boxH}, colMeasureMark)
	for i, a := range anchorSpecs {
		add(0, fmt.Sprintf("anchor_%d", i), fmt.Sprintf("DrawStringWrapped anchor %.1f/%.1f", a.ax, a.ay),
			render.Point{X: a.x, Y: a.y}, colAnchorMark)
	}

	// B: wrap limit + underline rules.
	add(1, "wrap_limit", "180px wrap limit line", render.Point{X: bLeftX + bWidth, Y: 30}, colWrapLimit)
	if len(lay.bLines) > 0 {
		add(1, "wrap_rule_first", "first wrapped line rule", render.Point{X: bLeftX + 8, Y: bBaseY + 4}, colWrapRule)
	}
	if len(lay.bLines) > 1 {
		add(1, "wrap_rule_second", "second wrapped line rule", render.Point{X: bLeftX + 8, Y: bBaseY + bLineStep + 4}, colWrapRule)
	}
	add(1, "wrap_row2_rule", "word rule starts after no trailing space", render.Point{X: bLeftX + 8, Y: lay.bRow2BaseY + 4}, colWrapRule)
	add(1, "wrap_row2_limit", "half-space-short limit line", render.Point{X: lay.bRow2LimitX, Y: lay.bRow2BaseY - 18}, colWrapLimit)
	add(1, "wrap_row2_beyond", "no rule past the 50% space limit", render.Point{X: lay.bRow2LimitX + 4, Y: lay.bRow2BaseY + 4}, colCardBG)
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildTextLowScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A MeasureMultilineString + DrawStringWrapped 三锚点", paintMeasure)
	card(1, "B WordWrap：行尾空格不占行宽 + 每行实测标尺", paintWrap)
	card(2, "C SetTextMode：Auto / MSDF / Vector / Bitmap / GlyphMask / Aliased", paintModes)
	card(3, "D SetLCDLayout：None / RGB / BGR 同上下文交错", paintLCD)
}

func cardBG(dc *render.Context, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func fillSquare(dc *render.Context, x, y, arm float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x-arm/2, y-arm/2, arm, arm)
	_ = dc.Fill()
}

func fillPlus(dc *render.Context, x, y, arm float64, c [3]float64) {
	dc.SetRGBA(c[0], c[1], c[2], 1)
	dc.DrawRectangle(x-arm, y-1, arm*2, 2)
	_ = dc.Fill()
	dc.DrawRectangle(x-1, y-arm, 2, arm*2)
	_ = dc.Fill()
	fillSquare(dc, x, y, 5, c)
}

func paintMeasure(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	boxW, boxH := lay.aW+2*aBoxPad, lay.aH+2*aBoxPad

	dc.SetRGBA(colPanel[0], colPanel[1], colPanel[2], 1)
	dc.DrawRectangle(cx+aBoxX, cy+aBoxY, boxW, boxH)
	_ = dc.Fill()
	dc.SetRGBA(colMeasureBox[0], colMeasureBox[1], colMeasureBox[2], 1)
	dc.SetLineWidth(1.5)
	dc.DrawRectangle(cx+aBoxX, cy+aBoxY, boxW, boxH)
	_ = dc.Stroke()

	dc.SetTextMode(render.TextModeAuto)
	dc.SetFont(lay.face16)
	dc.SetRGBA(colMeasureText[0], colMeasureText[1], colMeasureText[2], 1)
	dc.DrawStringWrapped(aText, cx+aBoxX+aBoxPad, cy+aBoxY+aBoxPad, 0, 0, aWrapWidth, aLineSpace, render.AlignLeft)
	dc.SetFont(lay.face12)
	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(fmt.Sprintf("w=%.1f  h=%.1f  lines=%d", lay.aW, lay.aH, lay.aLines),
		cx+aBoxX, cy+aBoxY+boxH+15)

	// Four corner markers make the measured box legible without relying on a
	// hairline surviving DPR scaling.
	fillSquare(dc, cx+aBoxX, cy+aBoxY, 5, colMeasureMark)
	fillSquare(dc, cx+aBoxX+boxW, cy+aBoxY, 5, colMeasureMark)
	fillSquare(dc, cx+aBoxX, cy+aBoxY+boxH, 5, colMeasureMark)
	fillSquare(dc, cx+aBoxX+boxW, cy+aBoxY+boxH, 5, colMeasureMark)

	dc.SetFont(lay.face24)
	for _, a := range anchorSpecs {
		dc.SetRGBA(colAnchorText[0], colAnchorText[1], colAnchorText[2], 1)
		dc.DrawStringWrapped("Ag", cx+a.x, cy+a.y, a.ax, a.ay, aAnchorW, 1.0, a.align)
		fillPlus(dc, cx+a.x, cy+a.y, 9, colAnchorMark)
	}
	dc.SetTextMode(render.TextModeAuto)
}

func paintWrap(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetTextMode(render.TextModeAuto)
	dc.SetFont(lay.face16)
	dc.SetRGBA(colWrapText[0], colWrapText[1], colWrapText[2], 1)
	for i, line := range lay.bLines {
		baseline := cy + bBaseY + float64(i)*bLineStep
		dc.DrawString(line, cx+bLeftX, baseline)
	}
	// Underline every returned line with its measured width. A trailing space
	// that survived WordWrap would extend the rule past the limit.
	dc.SetRGBA(colWrapRule[0], colWrapRule[1], colWrapRule[2], 1)
	for i, w := range lay.bLineWidths {
		y := cy + bBaseY + float64(i)*bLineStep + 3
		dc.DrawRectangle(cx+bLeftX, y, w, 3)
		_ = dc.Fill()
	}

	// Limit line for the paragraph.
	dc.SetRGBA(colWrapLimit[0], colWrapLimit[1], colWrapLimit[2], 1)
	dc.DrawRectangle(cx+bLeftX+bWidth-1, cy+22, 3, 78)
	_ = dc.Fill()

	// Second row: word + separator at a width where the space does not fit.
	dc.SetRGBA(colWrapText[0], colWrapText[1], colWrapText[2], 1)
	dc.DrawString(lay.bRow2Text, cx+bLeftX, cy+lay.bRow2BaseY)
	dc.SetRGBA(colWrapRule[0], colWrapRule[1], colWrapRule[2], 1)
	dc.DrawRectangle(cx+bLeftX, cy+lay.bRow2BaseY+3, lay.bRow2LineW, 3)
	_ = dc.Fill()
	dc.SetRGBA(colWrapLimit[0], colWrapLimit[1], colWrapLimit[2], 1)
	dc.DrawRectangle(cx+lay.bRow2LimitX-1, cy+lay.bRow2BaseY-20, 3, 28)
	_ = dc.Fill()

	dc.SetFont(lay.face12)
	dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
	dc.DrawString(fmt.Sprintf("limit %.1fpx; line %.1fpx", lay.bRow2Width, lay.bRow2LineW),
		cx+bLeftX, cy+lay.bRow2BaseY-24)
}

func paintModes(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	dc.SetFont(lay.face12)
	for _, row := range modeRows {
		dc.SetRGBA(colModeRule[0], colModeRule[1], colModeRule[2], 1)
		dc.DrawRectangle(cx+12, cy+row.baseline+5, contentW-24, 1)
		_ = dc.Fill()
		dc.SetTextMode(render.TextModeAuto)
		dc.SetRGBA(colLabel[0], colLabel[1], colLabel[2], 1)
		dc.DrawString(row.label, cx+14, cy+row.baseline)
	}

	dc.SetFont(lay.face20)
	for _, row := range modeRows {
		dc.SetTextMode(row.mode)
		dc.SetRGBA(colModeText[0], colModeText[1], colModeText[2], 1)
		dc.DrawString("AgHij", cx+96, cy+row.baseline)
	}
	dc.SetTextMode(render.TextModeAuto)
}

func paintLCD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	for _, row := range lcdRows {
		dc.SetRGBA(colLCDPanel[0], colLCDPanel[1], colLCDPanel[2], 1)
		dc.DrawRectangle(cx+lcdPanelX, cy+row.panelY, lcdPanelW, lcdPanelH)
		_ = dc.Fill()

		dc.SetTextMode(render.TextModeAuto)
		dc.SetFont(lay.face12)
		dc.SetRGBA(colMeasureText[0], colMeasureText[1], colMeasureText[2], 1)
		dc.DrawString(row.label, cx+14, cy+row.panelY+24)

		dc.SetFont(lay.face24)
		dc.SetTextMode(render.TextModeGlyphMask)
		dc.SetLCDLayout(row.layout)
		dc.SetRGBA(colLCDInk[0], colLCDInk[1], colLCDInk[2], 1)
		dc.DrawString("AgHij", cx+lcdTextX, cy+row.panelY+28)
	}
	dc.SetLCDLayout(render.LCDLayoutNone)
	dc.SetTextMode(render.TextModeAuto)
}

// ---------------------------------------------------------------------------
// Logic probes (CPU offscreen child): value-level contracts.
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

	face20, path, err := rendering.TryLoadDefaultFace(20)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: font: %v\n", err)
		return nil, nil
	}
	face16, _, err := rendering.TryLoadDefaultFace(16)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logic: font @16: %v\n", err)
		return nil, nil
	}
	_ = path

	// 1) Width is the longest visual line, not the whole unwrapped string.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		s := "Hello\nWide line\nx"
		w, _ := dc.MeasureMultilineString(s, 1.25)
		var wantW float64
		for _, line := range splitLines(s) {
			if lw, _ := dc.MeasureString(line); lw > wantW {
				wantW = lw
			}
		}
		setDetail("width=%.4f want=%.4f", w, wantW)
		check("measure_width_max_line", w == wantW)
		_ = dc.Close()
	}

	// 2) Height is ascent + (n-1)*lineHeight*spacing + descent.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		s := "Hello\nWide line\nx"
		_, h := dc.MeasureMultilineString(s, 1.25)
		m := face20.Metrics()
		want := float64(len(splitLines(s))-1)*m.LineHeight()*1.25 + m.Ascent + m.Descent
		setDetail("height=%.4f want=%.4f", h, want)
		check("measure_height_visual_block", h == want)
		_ = dc.Close()
	}

	// 3) Single line has no gap, so spacing does not change its height.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		_, h1 := dc.MeasureMultilineString("Hello", 1)
		_, h2 := dc.MeasureMultilineString("Hello", 3)
		m := face20.Metrics()
		setDetail("h1=%.4f h2=%.4f want=%.4f", h1, h2, m.Ascent+m.Descent)
		check("measure_single_line_spacing_invariant", h1 == h2 && h1 == m.Ascent+m.Descent)
		_ = dc.Close()
	}

	// 4) Recorder and Context expose the same measurement.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		rec := recording.NewRecorder(400, 200)
		rec.SetFont(face20)
		s := "Hello\r\nWide line"
		cw, ch := dc.MeasureMultilineString(s, 1.3)
		rw, rh := rec.MeasureMultilineString(s, 1.3)
		setDetail("context=(%.4f,%.4f) recorder=(%.4f,%.4f)", cw, ch, rw, rh)
		check("measure_recorder_parity", cw == rw && ch == rh)
		_ = dc.Close()
	}

	// 5) WordWrap drops a separator that triggered the break.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		lines := dc.WordWrap("word ", 50)
		fits := len(lines) == 1 && lines[0] == "word"
		if fits {
			w, _ := dc.MeasureString(lines[0])
			fits = w <= 50
		}
		setDetail("lines=%q", lines)
		check("wrap_trims_trailing_space", fits)
		_ = dc.Close()
	}

	// 6) Every wrapped line fits the requested width and loses edge spaces.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		lines := dc.WordWrap("Hello world", 80)
		ok := len(lines) == 2
		for _, line := range lines {
			if line != strings.TrimSpace(line) {
				ok = false
			}
			if w, _ := dc.MeasureString(line); w > 80 {
				ok = false
			}
		}
		setDetail("lines=%q", lines)
		check("wrap_lines_fit_width", ok)
		_ = dc.Close()
	}

	// 7) Wrapping preserves every word except separators at line edges.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		src := "The quick brown fox jumps over"
		lines := dc.WordWrap(src, 120)
		got := strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
		want := strings.Join(strings.Fields(src), " ")
		setDetail("got=%q want=%q lines=%q", got, want, lines)
		check("wrap_words_preserved", got == want)
		_ = dc.Close()
	}

	// 8) A long word is preserved whole (overflow is allowed, truncation is not).
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face20)
		long := "supercalifragilisticexpialidocious"
		lines := dc.WordWrap(long, 80)
		setDetail("lines=%q", lines)
		check("wrap_long_word_preserved", len(lines) == 1 && lines[0] == long)
		_ = dc.Close()
	}

	// 9) CRLF byte ranges address the original string, not the normalized copy.
	{
		src := "a\r\nb\r\nc"
		results := text.WrapText(src, face16, 1000, text.WrapWord)
		ok := len(results) == 3
		for _, r := range results {
			if r.Start < 0 || r.End > len(src) || src[r.Start:r.End] != r.Text {
				ok = false
			}
		}
		setDetail("results=%+v", results)
		check("wrap_crlf_original_offsets", ok)
	}

	// 10) Blank paragraphs keep their position and an empty range.
	{
		src := "a\r\n\r\nb"
		results := text.WrapText(src, face16, 1000, text.WrapWord)
		ok := len(results) == 3 && results[1].Text == "" && results[1].Start == results[1].End
		if ok {
			ok = src[results[0].Start:results[0].End] == "a" && src[results[2].Start:results[2].End] == "b"
		}
		setDetail("results=%+v", results)
		check("wrap_blank_line_offsets", ok)
	}

	// 11) Recorder WordWrap matches Context WordWrap.
	{
		dc := probeCtxOpaque(400, 200)
		dc.SetFont(face16)
		rec := recording.NewRecorder(400, 200)
		rec.SetFont(face16)
		a := dc.WordWrap("Hello world this is a test", 80)
		b := rec.WordWrap("Hello world this is a test", 80)
		setDetail("context=%q recorder=%q", a, b)
		check("wrap_recorder_parity", strings.Join(a, "|") == strings.Join(b, "|"))
		_ = dc.Close()
	}

	// 12) DrawStringWrapped anchors the wrap box: top-left and right-bottom.
	{
		dc := probeCtxOpaque(260, 180)
		dc.SetFont(face20)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringWrapped("Ag", 60, 40, 0, 0, 120, 1.0, render.AlignLeft)
		minX, minY, _, _, ok := inkBounds(dc, 260, 180)
		setDetail("bbox min=(%d,%d)", minX, minY)
		check("drawwrapped_top_left_anchor", ok && minX >= 60 && minX <= 66 && minY >= 40 && minY <= 47)
		_ = dc.Close()
	}
	{
		dc := probeCtxOpaque(300, 220)
		dc.SetFont(face20)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringWrapped("Ag", 220, 160, 1, 1, 120, 1.0, render.AlignRight)
		_, _, maxX, maxY, ok := inkBounds(dc, 300, 220)
		setDetail("bbox max=(%d,%d)", maxX, maxY)
		check("drawwrapped_right_bottom_anchor", ok && maxX >= 214 && maxX <= 221 && maxY >= 154 && maxY <= 161)
		_ = dc.Close()
	}

	// 13) Center anchor with center alignment puts ink around the cross.
	{
		dc := probeCtxOpaque(300, 220)
		dc.SetFont(face20)
		dc.SetRGB(0, 0, 0)
		dc.DrawStringWrapped("Ag", 150, 110, 0.5, 0.5, 140, 1.0, render.AlignCenter)
		minX, minY, maxX, maxY, ok := inkBounds(dc, 300, 220)
		cx, cy := float64(minX+maxX)/2, float64(minY+maxY)/2
		setDetail("bbox=%d,%d..%d,%d center=(%.1f,%.1f)", minX, minY, maxX, maxY, cx, cy)
		check("drawwrapped_center_anchor", ok && math.Abs(cx-150) <= 4 && math.Abs(cy-110) <= 4)
		_ = dc.Close()
	}

	// 14) Every TextMode has visible ink; Auto must not silently drop text.
	{
		dc := probeCtxOpaque(260, 80)
		dc.SetFont(face20)
		dc.SetRGB(0, 0, 0)
		ok := true
		var counts []int
		for _, mode := range []render.TextMode{render.TextModeAuto, render.TextModeMSDF, render.TextModeVector, render.TextModeBitmap, render.TextModeGlyphMask, render.TextModeAliased} {
			dc.ClearWithColor(render.White)
			dc.SetTextMode(mode)
			dc.DrawString("AgHij", 12, 50)
			n := countInk(dc, 260, 80)
			counts = append(counts, n)
			if n < 20 {
				ok = false
			}
		}
		dc.SetTextMode(render.TextModeAuto)
		setDetail("ink counts=%v", counts)
		check("textmode_all_modes_render", ok)
		_ = dc.Close()
	}

	// 15) LCD layouts are distinct enum states; interleaving must not panic.
	{
		dc := probeCtxOpaque(260, 80)
		dc.SetFont(face20)
		dc.SetRGB(0, 0, 0)
		for _, layout := range []render.LCDLayout{render.LCDLayoutNone, render.LCDLayoutRGB, render.LCDLayoutBGR} {
			dc.SetLCDLayout(layout)
			dc.DrawString("Ag", 12, 50)
		}
		setDetail("none=%d rgb=%d bgr=%d", render.LCDLayoutNone, render.LCDLayoutRGB, render.LCDLayoutBGR)
		check("lcd_layouts_distinct", render.LCDLayoutNone != render.LCDLayoutRGB &&
			render.LCDLayoutRGB != render.LCDLayoutBGR && render.LCDLayoutNone != render.LCDLayoutBGR)
		_ = dc.Close()
	}

	return probes, failed
}

// ---------------------------------------------------------------------------
// Region pixel probes on the GPU snapshot.
// ---------------------------------------------------------------------------

func runLowPixelChecks(img image.Image, originX, originY, dpr float64) map[string]bool {
	out := map[string]bool{}
	if img == nil {
		return out
	}
	cardBG := colCardBG
	near := func(r, g, b float64, c [3]float64, tol float64) bool {
		return math.Abs(r-c[0]) <= tol && math.Abs(g-c[1]) <= tol && math.Abs(b-c[2]) <= tol
	}
	rectStats := func(card int, x0, y0, x1, y1 float64) (ink, bg, colored, total int) {
		ca := cardAt[card]
		ox := originX + ca[0] + contentX0
		oy := originY + ca[1] + contentY0
		px0, py0 := int((ox+x0)*dpr), int((oy+y0)*dpr)
		px1, py1 := int((ox+x1)*dpr), int((oy+y1)*dpr)
		for y := py0; y < py1 && y < img.Bounds().Dy(); y++ {
			for x := px0; x < px1 && x < img.Bounds().Dx(); x++ {
				r32, g32, b32, _ := img.At(x, y).RGBA()
				r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
				total++
				if near(r, g, b, cardBG, 0.045) {
					bg++
				} else {
					ink++
				}
				mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
				if mx-mn > 8.0/255 {
					colored++
				}
			}
		}
		return
	}

	// Card A: the measured box contains DrawStringWrapped ink.
	{
		boxW, boxH := lay.aW+2*aBoxPad, lay.aH+2*aBoxPad
		ink, _, _, total := rectStats(0, aBoxX+aBoxPad, aBoxY+aBoxPad, aBoxX+aBoxPad+lay.aW, aBoxY+aBoxPad+lay.aH)
		out["pixel_measure_box_has_text"] = ink > total/50 && total > 0
		out["pixel_measure_box_edges"] = boxW > 0 && boxH > 0
	}

	// Card C: every mode row has ink, and MSDF is not a solid rectangle.
	modeInk := map[string]int{}
	modeBG := map[string]int{}
	for _, row := range modeRows {
		ink, bg, _, total := rectStats(2, 92, row.baseline-20, 200, row.baseline+3)
		modeInk[row.label] = ink
		modeBG[row.label] = bg
		out["pixel_mode_"+strings.ToLower(row.label)+"_ink"] = ink > 15 && total > 0
	}
	if total := (200 - 92) * 23; total > 0 {
		out["pixel_mode_msdf_not_solid"] = modeInk["MSDF"] > 15 && modeBG["MSDF"]*100/total > 25
	}

	// Card D: None is grayscale; RGB/BGR are colored and differ; all have ink.
	rowStats := func(row lcdRow) (ink, colored int) {
		ca := cardAt[3]
		ox := originX + ca[0] + contentX0 + lcdTextX
		oy := originY + ca[1] + contentY0 + row.panelY
		px0, py0 := int(ox*dpr), int((oy+3)*dpr)
		px1, py1 := int((ox+270)*dpr), int((oy+lcdPanelH-3)*dpr)
		for y := py0; y < py1 && y < img.Bounds().Dy(); y++ {
			for x := px0; x < px1 && x < img.Bounds().Dx(); x++ {
				r32, g32, b32, _ := img.At(x, y).RGBA()
				r, g, b := float64(r32>>8)/255, float64(g32>>8)/255, float64(b32>>8)/255
				if r < 0.70 || g < 0.70 || b < 0.70 {
					ink++
				}
				mx, mn := math.Max(r, math.Max(g, b)), math.Min(r, math.Min(g, b))
				if mx-mn > 0.04 {
					colored++
				}
			}
		}
		return
	}
	var lcd [3]lcdRow
	copy(lcd[:], lcdRows)
	inkNone, coloredNone := rowStats(lcd[0])
	inkRGB, coloredRGB := rowStats(lcd[1])
	inkBGR, coloredBGR := rowStats(lcd[2])
	out["pixel_lcd_none_ink"] = inkNone > 30
	out["pixel_lcd_none_grayscale"] = coloredNone <= 2
	out["pixel_lcd_rgb_ink"] = inkRGB > 30
	out["pixel_lcd_rgb_colored"] = coloredRGB > 8
	out["pixel_lcd_bgr_ink"] = inkBGR > 30
	out["pixel_lcd_bgr_colored"] = coloredBGR > 8
	out["pixel_lcd_rgb_bgr_distinct"] = lcdRegionsDiffer(img, originX, originY, dpr)
	return out
}

func lcdRegionsDiffer(img image.Image, originX, originY, dpr float64) bool {
	diff := 0
	for _, pair := range [][2]lcdRow{{lcdRows[1], lcdRows[2]}} {
		a, b := pair[0], pair[1]
		ca := cardAt[3]
		ox := originX + ca[0] + contentX0 + lcdTextX
		oyA := originY + ca[1] + contentY0 + a.panelY
		oyB := originY + ca[1] + contentY0 + b.panelY
		px0, px1 := int(ox*dpr), int((ox+270)*dpr)
		py0, py1 := 3, int(lcdPanelH-3)
		for y := py0; y < py1; y++ {
			for x := px0; x < px1; x++ {
				ar, ag, ab, _ := img.At(x, int((oyA+float64(y))*dpr)).RGBA()
				br, bg, bb, _ := img.At(x, int((oyB+float64(y))*dpr)).RGBA()
				if channelDelta8(ar, br) > 2 || channelDelta8(ag, bg) > 2 || channelDelta8(ab, bb) > 2 {
					diff++
				}
			}
		}
	}
	return diff > 20
}

func channelDelta8(a, b uint32) int {
	d := int(a>>8) - int(b>>8)
	if d < 0 {
		return -d
	}
	return d
}
