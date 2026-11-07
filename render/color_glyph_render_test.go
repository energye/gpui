package render

import (
	"image/color"
	"math"
	"testing"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

// loadAHEM loads the AHEM block font (every glyph = one solid em-box
// square) for pixel-exact COLRv1 assertions.
func loadAHEM(t *testing.T) text.ParsedFont {
	t.Helper()
	src, err := text.NewFontSourceFromFile("text/testdata/ahem.ttf")
	if err != nil {
		t.Fatalf("NewFontSourceFromFile(ahem) error = %v", err)
	}
	return src.Parsed()
}

// ahemBox returns the design-space (y-up) bounding box of AHEM glyph 'A'.
func ahemBox(t *testing.T, font text.ParsedFont) (gid uint16, upem float64, box Rect) {
	t.Helper()
	gid = font.GlyphIndex('A')
	if gid == 0 {
		t.Fatal("AHEM lacks glyph 'A'")
	}
	upem = float64(font.UnitsPerEm())
	outline, err := (&text.OutlineExtractor{}).ExtractOutline(font, text.GlyphID(gid), upem)
	if err != nil {
		t.Fatalf("ExtractOutline error = %v", err)
	}
	// ExtractOutline is y-down (ink above the baseline is negative); flip
	// into the COLRv1 y-up design space.
	box = Rect{
		Min: Point{X: outline.Bounds.MinX, Y: -outline.Bounds.MaxY},
		Max: Point{X: outline.Bounds.MaxX, Y: -outline.Bounds.MinY},
	}
	return gid, upem, box
}

// testPalette: 0=red, 1=green, 2=blue.
var testPalette = []emoji.Color{
	{R: 255, G: 0, B: 0, A: 255},
	{R: 0, G: 255, B: 0, A: 255},
	{R: 0, G: 0, B: 255, A: 255},
}

func TestCOLRv1Render_SolidLayers(t *testing.T) {
	InstallCOLRv1Renderer()
	font := loadAHEM(t)
	gid, upem, box := ahemBox(t, font)
	const ppem = 100.0

	// Two layers: red square at origin; green square translated +400 units
	// right and −400 down (design y-up: dy negative = down).
	green := &emoji.PaintNode{
		Format:       emoji.PaintSolid,
		PaletteIndex: 1,
		Alpha:        1,
	}
	glyphGreen := &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid, Child: green}
	moved := &emoji.PaintNode{
		Format: emoji.PaintTranslate, DX: 400, DY: -400,
		XX: 1, YY: 1, Child: glyphGreen,
	}
	red := &emoji.PaintNode{Format: emoji.PaintSolid, PaletteIndex: 0, Alpha: 1}
	glyphRed := &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid, Child: red}
	root := &emoji.PaintNode{
		Format:   emoji.PaintColrLayers,
		Children: []emoji.PaintRef{{Node: glyphRed}, {Node: moved}},
	}
	g := &emoji.COLRGlyph{GlyphID: gid, Version: 1, Paint: root}

	img, ox, oy, err := renderCOLRv1WithPalette(font, g, upem, ppem, testPalette, color.RGBA{A: 255})
	_ = ox
	_ = oy
	if err != nil {
		t.Fatalf("renderCOLRv1Glyph error = %v", err)
	}
	s := ppem / upem
	// Design→pixel: px=(x−xmin)s, py=(ymax−y)s over the union bounds
	// (red box ∪ green box translated by (400,−400)).
	uxmin := math.Min(box.Min.X, box.Min.X+400)
	uxmax := math.Max(box.Max.X, box.Max.X+400)
	uymin := math.Min(box.Min.Y, box.Min.Y-400)
	uymax := math.Max(box.Max.Y, box.Max.Y-400)
	toPx := func(x, y float64) (int, int) {
		return int((x - uxmin) * s), int((uymax - y) * s)
	}
	// Sample inside the red-only band: x in (0,400) design, y in (−200,200)
	// design (the green box covers y ≤ −200 after its −400 shift).
	rx, ry := toPx(200, 0)
	r, gr, b, a := img.At(rx, ry).RGBA()
	if r>>8 != 255 || gr>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("red-only region = (%d,%d,%d,%d), want opaque red", r>>8, gr>>8, b>>8, a>>8)
	}
	// Sample inside the overlap: translated box ∩ original box.
	ox0 := math.Max(box.Min.X, box.Min.X+400)
	ox1 := math.Min(box.Max.X, box.Max.X+400)
	oy0 := math.Max(box.Min.Y, box.Min.Y-400)
	oy1 := math.Min(box.Max.Y, box.Max.Y-400)
	cx, cy := toPx((ox0+ox1)/2, (oy0+oy1)/2)
	r, gr, b, a = img.At(cx, cy).RGBA()
	if r>>8 != 0 || gr>>8 != 255 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("overlap region = (%d,%d,%d,%d), want opaque green (top layer)", r>>8, gr>>8, b>>8, a>>8)
	}
	// Ink bounds equal the union box mapped (crop to ink).
	if img.Bounds().Dx() != int(math.Ceil((uxmax-uxmin)*s)) || img.Bounds().Dy() != int(math.Ceil((uymax-uymin)*s)) {
		t.Errorf("cropped size = %v, want union box at scale %v", img.Bounds(), s)
	}
}

func TestCOLRv1Render_ClipList(t *testing.T) {
	InstallCOLRv1Renderer()
	font := loadAHEM(t)
	gid, upem, box := ahemBox(t, font)
	const ppem = 100.0

	red := &emoji.PaintNode{Format: emoji.PaintSolid, PaletteIndex: 0, Alpha: 1}
	root := &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid, Child: red}
	// Clip box covers only the top-left quarter of the glyph box.
	clip := &emoji.ClipBox{
		XMin: box.Min.X, XMax: (box.Min.X + box.Max.X) / 2,
		YMin: (box.Min.Y + box.Max.Y) / 2, YMax: box.Max.Y,
	}
	g := &emoji.COLRGlyph{GlyphID: gid, Version: 1, Paint: root, Clip: clip}

	img, _, _, err := renderCOLRv1WithPalette(font, g, upem, ppem, testPalette, color.RGBA{A: 255})
	if err != nil {
		t.Fatalf("renderCOLRv1Glyph error = %v", err)
	}
	s := ppem / upem
	wantW := int(math.Ceil((clip.XMax - clip.XMin) * s))
	wantH := int(math.Ceil((clip.YMax - clip.YMin) * s))
	if img.Bounds().Dx() != wantW || img.Bounds().Dy() != wantH {
		t.Errorf("clipped size = %v, want ~%dx%d", img.Bounds(), wantW, wantH)
	}
	cx, cy := img.Bounds().Dx()/2, img.Bounds().Dy()/2
	r, gr, b, a := img.At(cx, cy).RGBA()
	if r>>8 != 255 || gr>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("clip interior = (%d,%d,%d,%d), want opaque red", r>>8, gr>>8, b>>8, a>>8)
	}
}

func TestCOLRv1Render_CompositeDestOut(t *testing.T) {
	InstallCOLRv1Renderer()
	font := loadAHEM(t)
	gid, upem, box := ahemBox(t, font)
	const ppem = 100.0

	// Backdrop: full red square. Source: green square scaled to the middle
	// half (PaintScale 0.5 around the box center). DestOut: red with a
	// green-square hole (source area cleared from backdrop).
	srcScale, srcChild := scaleAroundCenterNodes(box, 0.5)
	_ = srcChild
	greenBox := &emoji.PaintNode{
		Format: emoji.PaintScaleAroundCenter,
		XX: 0.5, YY: 0.5,
		DX: (box.Min.X+box.Max.X)/2*0.5, DY: (box.Min.Y+box.Max.Y)/2*0.5,
		Child: &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid,
			Child: &emoji.PaintNode{Format: emoji.PaintSolid, PaletteIndex: 1, Alpha: 1}},
	}
	_ = srcScale
	backdrop := &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid,
		Child: &emoji.PaintNode{Format: emoji.PaintSolid, PaletteIndex: 0, Alpha: 1}}
	root := &emoji.PaintNode{
		Format:   emoji.PaintComposite,
		Mode:     emoji.CompositeDestOut,
		Source:   greenBox,
		Backdrop: backdrop,
	}
	g := &emoji.COLRGlyph{GlyphID: gid, Version: 1, Paint: root}

	img, _, _, err := renderCOLRv1WithPalette(font, g, upem, ppem, testPalette, color.RGBA{A: 255})
	if err != nil {
		t.Fatalf("renderCOLRv1Glyph error = %v", err)
	}
	s := ppem / upem
	toPx := func(x, y float64) (int, int) {
		return int((x - box.Min.X) * s), int((box.Max.Y - y) * s)
	}
	// Center: inside the scaled green square → hole (transparent).
	hx, hy := toPx((box.Min.X+box.Max.X)/2, (box.Min.Y+box.Max.Y)/2)
	_, _, _, a := img.At(hx, hy).RGBA()
	if a>>8 != 0 {
		t.Errorf("composite center alpha = %d, want 0 (punched out)", a>>8)
	}
	// Corner region: outside the scaled square → red survives.
	cxp, cyp := toPx(box.Min.X+(box.Max.X-box.Min.X)*0.1, box.Min.Y+(box.Max.Y-box.Min.Y)*0.1)
	r, gr, b, a := img.At(cxp, cyp).RGBA()
	if r>>8 != 255 || gr>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("composite corner = (%d,%d,%d,%d), want opaque red", r>>8, gr>>8, b>>8, a>>8)
	}
}

func scaleAroundCenterNodes(box Rect, f float64) (*emoji.PaintNode, *emoji.PaintNode) {
	cx := (box.Min.X + box.Max.X) / 2
	cy := (box.Min.Y + box.Max.Y) / 2
	return &emoji.PaintNode{
		Format: emoji.PaintScaleAroundCenter,
		XX:     f, YY: f,
		DX: cx - f*cx, DY: cy - f*cy,
	}, nil
}

func TestCOLRv1Render_LinearGradient(t *testing.T) {
	InstallCOLRv1Renderer()
	font := loadAHEM(t)
	gid, upem, box := ahemBox(t, font)
	const ppem = 100.0

	// Vertical gradient in design space (y-up): red at the bottom (p0),
	// blue at the top (p1), p2 offset to the right (non-parallel rotation
	// point). AHEM box interior samples near the bottom/top edges.
	line := emoji.ColorLine{
		Stops: []emoji.ColorStop{
			{Offset: 0, PaletteIndex: 0, Alpha: 1}, // red at p0 (bottom)
			{Offset: 1, PaletteIndex: 2, Alpha: 1}, // blue at p1 (top)
		},
	}
	grad := &emoji.PaintNode{
		Format: emoji.PaintLinearGradient,
		Line:   line,
		X0:     box.Min.X, Y0: box.Min.Y,
		X1: box.Min.X, Y1: box.Max.Y,
		X2: box.Max.X, Y2: box.Min.Y,
	}
	root := &emoji.PaintNode{Format: emoji.PaintGlyph, GlyphID: gid, Child: grad}
	g := &emoji.COLRGlyph{GlyphID: gid, Version: 1, Paint: root}

	img, _, _, err := renderCOLRv1WithPalette(font, g, upem, ppem, testPalette, color.RGBA{A: 255})
	if err != nil {
		t.Fatalf("renderCOLRv1Glyph error = %v", err)
	}
	s := ppem / upem
	cx := img.Bounds().Dx() / 2
	topY := int((box.Max.Y - box.Max.Y) + (box.Max.Y-(box.Max.Y-(box.Max.Y-box.Min.Y)*0.1))*s*0)
	_ = topY
	sampleY := func(designY float64) int {
		return int((box.Max.Y - designY) * s)
	}
	rT, gT, bT, _ := img.At(cx, sampleY(box.Max.Y-(box.Max.Y-box.Min.Y)*0.05)).RGBA()
	rB, gB, bB, _ := img.At(cx, sampleY(box.Min.Y+(box.Max.Y-box.Min.Y)*0.05)).RGBA()
	// Top ≈ blue, bottom ≈ red (near-stop samples, allow ramp slack).
	if !(bT > rT && bT > 128 && gT < 128) {
		t.Errorf("gradient top = (%d,%d,%d), want blue-dominant", rT>>8, gT>>8, bT>>8)
	}
	if !(rB > bB && rB > 128 && gB < 128) {
		t.Errorf("gradient bottom = (%d,%d,%d), want red-dominant", rB>>8, gB>>8, bB>>8)
	}
	// Middle: t=0.5 interpolated in LINEAR-LIGHT RGB per the CPAL spec's
	// Interpolation of Colors rule — sRGB-encoded midpoint ≈ 0.5^(1/2.2)·255 ≈ 188.
	rM, _, bM, _ := img.At(cx, sampleY((box.Min.Y+box.Max.Y)/2)).RGBA()
	if math.Abs(float64(rM>>8)-188) > 12 || math.Abs(float64(bM>>8)-188) > 12 {
		t.Errorf("gradient middle = (%d,%d,%d), want linear-light half mix (~188)", rM>>8, 0, bM>>8)
	}
}

func TestCOLRv1Render_UnboundedFails(t *testing.T) {
	InstallCOLRv1Renderer()
	font := loadAHEM(t)
	_, upem, _ := ahemBox(t, font)
	// Root solid with no PaintGlyph and no clip box: unbounded per spec.
	root := &emoji.PaintNode{Format: emoji.PaintSolid, PaletteIndex: 0, Alpha: 1}
	g := &emoji.COLRGlyph{GlyphID: 1, Version: 1, Paint: root}
	if _, _, _, err := renderCOLRv1WithPalette(font, g, upem, 100, testPalette, color.RGBA{A: 255}); err == nil {
		t.Error("unbounded glyph error = nil, want error")
	}
}
