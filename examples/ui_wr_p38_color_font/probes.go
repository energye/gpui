package main

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

type probeResult struct {
	Key    string
	OK     bool
	Detail string
}

// 全局快照引用（main 里 loadImage 后注入，供探针采样）。
var (
	snapImage image.Image
	bxGlobal  float64
	byGlobal  float64
	dprGlobal float64
)

var probeKeys = []string{
	"fonts_load_five",
	"glyph_types_four",
	"raster_cbdt",
	"raster_sbix",
	"raster_colrv0",
	"raster_svg_wired",
	"raster_colrv1_wired",
	"svg_flags",
	"colrv1_graph",
	"colrv0_layers",
	"colrv1_var_axes",
}

var (
	colBlueGlyph  = [3]float64{40.0 / 255, 80.0 / 255, 220.0 / 255}
	colRedGlyph   = [3]float64{220.0 / 255, 40.0 / 255, 40.0 / 255}
	colGreenGlyph = [3]float64{60.0 / 255, 220.0 / 255, 60.0 / 255}
	colSVGRed     = [3]float64{0xe2 / 255.0, 0x4a / 255.0, 0x4a / 255.0}
	colSVGGreen   = [3]float64{0x3c / 255.0, 0xcf / 255.0, 0x7a / 255.0}
	colV0Red      = [3]float64{226.0 / 255, 0, 0}          // generated CPAL entry 0
	colV0Green    = [3]float64{60.0 / 255, 226.0 / 255, 0} // generated CPAL entry 1
)

func nearRGB(got, want [3]float64, tol float64) bool {
	for i := range got {
		if math.Abs(got[i]-want[i]) > tol {
			return false
		}
	}
	return true
}

// probeFonts: the four card fonts plus the COLRv0 font (no card of its
// own — the v0 flatten path is covered at probe level).
var probeFonts = []string{
	"render/text/testdata/p38_color_cbdt.ttf",
	"render/text/testdata/p38_color_sbix.ttf",
	"render/text/testdata/p38_color_svg.ttf",
	"render/text/testdata/p38_color_colrv1.ttf",
	"render/text/testdata/p38_color_colrv0.ttf",
	"render/text/testdata/p44_colrv1_var.ttf",
}

func runLogicProbes() []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}

	// The probe subprocess runs CPU-only; install the pixel-production
	// hooks explicitly (same as the window bake).
	svg.InstallSVGTableRenderer()
	render.InstallCOLRv1Renderer()

	var fonts [6]text.ParsedFont
	for i, path := range probeFonts {
		src, err := text.NewFontSourceFromFile(path)
		if err != nil {
			add("fonts_load_five", false, path+": "+err.Error())
			for _, k := range probeKeys[1:] {
				add(k, false, "font load failed")
			}
			return out
		}
		fonts[i] = src.Parsed()
	}
	add("fonts_load_five", true, "cbdt/sbix/svg/colrv1/colrv0")

	wantTypes := [4]text.GlyphType{text.GlyphTypeBitmap, text.GlyphTypeBitmap, text.GlyphTypeSVG, text.GlyphTypeCOLR}
	typesOK := true
	detail := ""
	for i := 0; i < 4; i++ {
		cf, isCF := fonts[i].(text.ColorFont)
		if !isCF {
			typesOK = false
			detail = fmt.Sprintf("card %d font is not ColorFont", i)
			break
		}
		gid := fonts[i].GlyphIndex('A')
		if got := cf.GlyphType(gid); got != wantTypes[i] {
			typesOK = false
			detail = fmt.Sprintf("card %d glyph A: %v want %v", i, got, wantTypes[i])
			break
		}
	}
	add("glyph_types_four", typesOK, detail)

	cache := text.NewColorRasterCache(16)
	fg := color.RGBA{R: 255, G: 255, B: 255, A: 255}

	// CBDT: blue bitmap with a green orientation marker.
	if img, err := cache.Image(fonts[0], fonts[0].GlyphIndex('A'), 96, 0, fg); err != nil {
		add("raster_cbdt", false, err.Error())
	} else if hasInk(img) && regionHas(img, colBlueGlyph, 0.12) && regionHas(img, colGreenGlyph, 0.12) {
		add("raster_cbdt", true, "blue bitmap + green marker")
	} else {
		add("raster_cbdt", false, "missing ink")
	}

	// sbix: red bitmap.
	if img, err := cache.Image(fonts[1], fonts[1].GlyphIndex('A'), 96, 0, fg); err != nil {
		add("raster_sbix", false, err.Error())
	} else if hasInk(img) && regionHas(img, colRedGlyph, 0.12) {
		add("raster_sbix", true, "red bitmap inked")
	} else {
		add("raster_sbix", false, "no red ink")
	}

	// COLRv0: flattened layers (red A + green B → both colors present).
	if img, err := cache.Image(fonts[4], fonts[4].GlyphIndex('A'), 96, 0, fg); err != nil {
		add("raster_colrv0", false, err.Error())
	} else if hasInk(img) && regionHas(img, colV0Red, 0.08) && regionHas(img, colV0Green, 0.08) {
		add("raster_colrv0", true, "v0 layers flattened")
	} else {
		add("raster_colrv0", false, "missing layer ink")
	}

	// SVG through the installed renderer: 'A' self-colored red circle.
	if img, err := cache.Image(fonts[2], fonts[2].GlyphIndex('A'), 96, 0, fg); err != nil {
		add("raster_svg_wired", false, err.Error())
	} else if hasInk(img) && regionHas(img, colSVGRed, 0.12) {
		add("raster_svg_wired", true, "svg doc rendered red")
	} else {
		add("raster_svg_wired", false, "no red ink from svg doc")
	}

	// COLRv1 through the installed renderer: gradient A + translated
	// green B (region-scan for the green layer and either gradient end).
	if img, err := cache.Image(fonts[3], fonts[3].GlyphIndex('A'), 96, 0, fg); err != nil {
		add("raster_colrv1_wired", false, err.Error())
	} else if hasInk(img) && regionHas(img, colV0Green, 0.08) {
		add("raster_colrv1_wired", true, "paint graph inked (green layer present)")
	} else {
		add("raster_colrv1_wired", false, "no green layer ink")
	}

	// SVG doc flags: 'A' self-colored, 'B' currentColor+palette.
	cf2 := fonts[2].(text.ColorFont)
	gA, errA := cf2.SVGGlyph(fonts[2].GlyphIndex('A'))
	gB, errB := cf2.SVGGlyph(fonts[2].GlyphIndex('B'))
	add("svg_flags", errA == nil && errB == nil && !gA.UsesCurrentColor && !gA.UsesPalette && gB.UsesCurrentColor && gB.UsesPalette,
		fmt.Sprintf("A(cur=%v,pal=%v) B(cur=%v,pal=%v) errs=%v/%v",
			gA.UsesCurrentColor, gA.UsesPalette, gB.UsesCurrentColor, gB.UsesPalette, errA, errB))

	// COLRv1 graph: 2-layer root + ClipList box.
	cf4 := fonts[3].(text.ColorFont)
	cg, err := cf4.COLRGlyph(fonts[3].GlyphIndex('A'), 0)
	layers := 0
	hasClip := false
	rootOK := false
	if err == nil && cg != nil && cg.Paint != nil {
		rootOK = cg.Paint.Format == emojiPaintColrLayers
		layers = len(cg.Paint.Children)
		hasClip = cg.Clip != nil
	}
	add("colrv1_graph", rootOK && layers == 2 && hasClip,
		fmt.Sprintf("err=%v root=%v layers=%d clip=%v", err, rootOK, layers, hasClip))

	// COLRv0 layers on the colrv0 font.
	cf0 := fonts[4].(text.ColorFont)
	cg0, errG := cf0.COLRGlyph(fonts[4].GlyphIndex('A'), 0)
	v0Layers := 0
	if cg0 != nil {
		v0Layers = len(cg0.Layers)
	}
	add("colrv0_layers", errG == nil && cg0 != nil && cg0.Paint == nil && v0Layers == 2,
		fmt.Sprintf("err=%v layers=%d", errG, v0Layers))

	// P4.4: the same COLRv1 glyph at SWPS = -90 / 0 / +90 must produce three
	// different paint graphs (Var* deltas applied end to end).
	if vf, ok := fonts[5].(interface {
		COLRGlyphVar(uint16, int, []text.FontVariation) (*emoji.COLRGlyph, error)
	}); ok {
		var sum func(*emoji.PaintNode) float64
		sum = func(n *emoji.PaintNode) float64 {
			if n == nil {
				return 0
			}
			v := n.Alpha + n.X0 + n.Y0 + n.X1 + n.Y1 + n.X2 + n.Y2 + n.R0 + n.R1 +
				n.CenterX + n.CenterY + n.StartAngleDeg + n.EndAngleDeg +
				n.XX + n.YX + n.XY + n.YY + n.DX + n.DY + float64(n.GlyphID)
			for _, st := range n.Line.Stops {
				v += st.Offset + st.Alpha
			}
			for _, c := range n.Children {
				v += sum(c.Node)
			}
			return v + sum(n.Child) + sum(n.Source) + sum(n.Backdrop)
		}
		var sums [3]float64
		okAll := true
		for i, ax := range []float32{-90, 0, 90} {
			g, err := vf.COLRGlyphVar(12, 0, []text.FontVariation{text.NewFontVariation("SWPS", ax)})
			if err != nil || g == nil || g.Paint == nil {
				okAll = false
				break
			}
			sums[i] = sum(g.Paint)
		}
		distinct := okAll && sums[0] != sums[1] && sums[1] != sums[2] && sums[0] != sums[2]
		add("colrv1_var_axes", distinct, fmt.Sprintf("sums=%.3f/%.3f/%.3f", sums[0], sums[1], sums[2]))
	} else {
		add("colrv1_var_axes", false, "COLRGlyphVar unavailable")
	}

	return out
}

// emojiPaintColrLayers mirrors emoji.PaintColrLayers without importing the
// emoji package here (the window surface only needs the constant).
const emojiPaintColrLayers = 1

// hasInk reports any non-transparent pixel.
func hasInk(img *text.ColorRasterImage) bool {
	if img == nil || img.Pix == nil {
		return false
	}
	b := img.Pix.Bounds()
	for y := 0; y < b.Dy(); y++ {
		row := img.Pix.Pix[y*img.Pix.Stride:]
		for x := 0; x < b.Dx(); x++ {
			if row[x*4+3] > 0 {
				return true
			}
		}
	}
	return false
}

// regionHas reports any pixel near the wanted color (placement-tolerant).
func regionHas(img *text.ColorRasterImage, want [3]float64, tol float64) bool {
	b := img.Pix.Bounds()
	for y := 0; y < b.Dy(); y++ {
		row := img.Pix.Pix[y*img.Pix.Stride:]
		for x := 0; x < b.Dx(); x++ {
			off := x * 4
			if row[off+3] < 200 {
				continue
			}
			got := [3]float64{
				float64(row[off]) / 255,
				float64(row[off+1]) / 255,
				float64(row[off+2]) / 255,
			}
			if nearRGB(got, want, tol) {
				return true
			}
		}
	}
	return false
}

// --- pixel probes on the window snapshot ---

func runPixelProbes(img image.Image, bx, by, dpr float64) []probeResult {
	var out []probeResult
	add := func(key string, ok bool, detail string) {
		out = append(out, probeResult{Key: key, OK: ok, Detail: detail})
	}
	keys := []string{
		"pixel_cbdt_blue", "pixel_cbdt_marker",
		"pixel_sbix_red",
		"pixel_svg_selfcolor", "pixel_svg_palette",
		"pixel_colrv1_green", "pixel_colrv1_gradient",
	}
	if img == nil {
		for _, k := range keys {
			add(k, false, "snapshot unavailable")
		}
		return out
	}

	// Glyphs render with face size 96 from the card baseline positions in
	// scene.go: 'A' at inner (40,150), 'B' at (200,150). Rasters sit
	// baseline-relative; exact placement varies by format — the probes
	// scan regions for a near-want pixel, not single points.
	regionHasColorTol := func(key string, card int, x0, y0, x1, y1 float64, want [3]float64, tol float64) {
		ax0 := bx + cardAt[card][0] + innerX + x0
		ay0 := by + cardAt[card][1] + innerY + y0
		ax1 := bx + cardAt[card][0] + innerX + x1
		ay1 := by + cardAt[card][1] + innerY + y1
		found, samples := regionInSnapshot(img, ax0, ay0, ax1, ay1, want, dpr, tol)
		add(key, found, fmt.Sprintf("samples=%d want=%v", samples, want))
	}
	regionHasColor := func(key string, card int, x0, y0, x1, y1 float64, want [3]float64) {
		regionHasColorTol(key, card, x0, y0, x1, y1, want, 0.09)
	}

	// 卡 1 CBDT：蓝方块本体 + 绿标记角。
	regionHasColor("pixel_cbdt_blue", 0, 30, 50, 150, 160, colBlueGlyph)
	regionHasColor("pixel_cbdt_marker", 0, 30, 40, 80, 90, colGreenGlyph)
	// 卡 2 sbix：红方块。
	regionHasColor("pixel_sbix_red", 1, 30, 50, 150, 160, colRedGlyph)
	// 卡 3 SVG：'A' 自色红圆；'B' currentColor 白 + 调色板 fallback 绿。
	regionHasColor("pixel_svg_selfcolor", 2, 30, 50, 150, 160, colSVGRed)
	regionHasColor("pixel_svg_palette", 2, 190, 50, 330, 160, colSVGGreen)
	// 卡 4 COLRv1：绿 B 层；渐变 A 的蓝端。
	regionHasColorTol("pixel_colrv1_green", 3, 30, 40, 340, 160, colV0Green, 0.2)
	// Generated CPAL entry 2 = blue (74,144,226): the gradient's top end.
	gradientBlue := [3]float64{74.0 / 255, 144.0 / 255, 226.0 / 255}
	regionHasColorTol("pixel_colrv1_gradient", 3, 30, 40, 180, 160, gradientBlue, 0.2)
	return out
}

// regionInSnapshot scans a card region (card-local inner coordinates) for
// a pixel near want; returns hit + sampled count.
func regionInSnapshot(img image.Image, ax0, ay0, ax1, ay1 float64, want [3]float64, dpr float64, tol float64) (bool, int) {
	if img == nil {
		return false, 0
	}
	px0, py0 := int(math.Round(ax0*dpr)), int(math.Round(ay0*dpr))
	px1, py1 := int(math.Round(ax1*dpr)), int(math.Round(ay1*dpr))
	samples := 0
	for py := py0; py <= py1; py++ {
		for px := px0; px <= px1; px++ {
			got, ok := pixelAt(img, px, py)
			if !ok {
				continue
			}
			samples++
			if nearRGB(got, want, tol) {
				return true, samples
			}
		}
	}
	return false, samples
}

func pixelAt(img image.Image, x, y int) ([3]float64, bool) {
	if img == nil {
		return [3]float64{}, false
	}
	b := img.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return [3]float64{}, false
	}
	r, g, bl, _ := img.At(x, y).RGBA()
	return [3]float64{float64(r>>8) / 255, float64(g>>8) / 255, float64(bl>>8) / 255}, true
}
