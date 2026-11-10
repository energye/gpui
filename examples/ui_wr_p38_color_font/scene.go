package main

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid (1460×800 window, same geometry as the other P3.8
// windows). Each card draws live text through the standard pipeline
// (Canvas.SetFont + DrawString) with one generated color font — the text
// dispatch routes color glyphs to ColorRasterCache (CPU leg) or the GPU
// color engine automatically; both flatten the four formats the same way.
const (
	cardW, cardH = 560.0, 300.0
	innerW       = 528.0
	innerH       = 252.0
	innerX       = (cardW - innerW) / 2
	innerY       = 40.0
	glyphSize    = 96.0
	stripW       = 1140.0
	stripH       = 120.0
)

var cardAt = [4][2]float64{
	{12, 8}, {592, 8},
	{12, 316}, {592, 316},
}

var (
	colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
	colTextFG = render.RGBA{R: 0.9, G: 0.92, B: 0.95, A: 1}
)

type baked struct {
	varFaces [3]text.Face
	faces    [4]text.Face
	err      string
	closed   bool
}

var (
	bakeOnce sync.Once
	bakedRes baked
)

// cardFontFiles: one generated color font per card (mkcolorfont output).
var cardFontFiles = [4]string{
	"render/text/testdata/p38_color_cbdt.ttf",
	"render/text/testdata/p38_color_sbix.ttf",
	"render/text/testdata/p38_color_svg.ttf",
	"render/text/testdata/p38_color_colrv1.ttf",
}

var cardTitles = [4]string{
	"1 CBDT/CBLC 位图（蓝方块+绿标记）",
	"2 sbix strike（红方块，strike 缩放）",
	"3 SVG 表（'A' 自色 / 'B' currentColor+调色板）",
	"4 COLRv1 paint graph（渐变 A + 平移绿 B + ClipList）",
}

func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		// Color glyph pixel production lives in the render layers; install
		// both hooks once (explicit, idempotent).
		svg.InstallSVGTableRenderer()
		render.InstallCOLRv1Renderer()
		if src, err := text.NewFontSourceFromFile("render/text/testdata/p44_colrv1_var.ttf"); err == nil {
			for k, v := range []float32{-90, 0, 90} {
				b.varFaces[k] = src.Face(80, text.WithVariations(text.NewFontVariation("SWPS", v)))
			}
		} else {
			b.err = "p44 variable font: " + err.Error()
			return
		}
		for i, path := range cardFontFiles {
			src, err := text.NewFontSourceFromFile(path)
			if err != nil {
				b.err = fmt.Sprintf("%s: %v", path, err)
				return
			}
			b.faces[i] = src.Face(glyphSize)
			if b.faces[i] == nil {
				b.err = fmt.Sprintf("%s: nil face", path)
				return
			}
		}
	})
	return &bakedRes
}

func closeBaked() {
	b := &bakedRes
	if b.closed {
		return
	}
	b.closed = true
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG.R, colCardBG.G, colCardBG.B, colCardBG.A)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func buildColorFontScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
			cardBG(dc, ox, oy, sz.Width, sz.Height)
			b := ensureBaked()
			if b.err != "" || b.faces[i] == nil {
				dc.SetRGBA(0.35, 0.06, 0.08, 1)
				dc.DrawRectangle(ox+innerX, oy+innerY, innerW, innerH)
				_ = dc.Fill()
				return
			}
			// Live text through the standard pipeline: SetFont + DrawString
			// routes color glyphs through ColorRasterCache / GPU color atlas.
			dc.SetFont(b.faces[i])
			dc.SetRGBA(colTextFG.R, colTextFG.G, colTextFG.B, colTextFG.A)
			// Baseline near the card middle; color glyphs span roughly one
			// em above it. 'A' exercises the primary format, 'B' the
			// currentColor/palette variants (SVG card) and the second paint
			// layer (COLRv1 card).
			dc.DrawString("A", ox+innerX+40, oy+innerY+150)
			dc.DrawString("B", ox+innerX+200, oy+innerY+150)
		}
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}

	for i := range cardAt {
		card(i, cardTitles[i])
	}

	// 卡 5（P4.4）：COLRv1 变量轴——同一字形在 SWPS=-90/0/+90 三个实例下。
	strip := rendering.NewRenderBox()
	strip.FixedWidth, strip.FixedHeight = stripW, stripH
	strip.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		cardBG(dc, ox, oy, sz.Width, sz.Height)
		b := ensureBaked()
		if b.err != "" {
			return
		}
		for k := 0; k < 3; k++ {
			if b.varFaces[k] == nil {
				continue
			}
			dc.DrawShapedColorGlyphs([]text.ShapedGlyph{{GID: 12}}, b.varFaces[k],
				ox+40+float64(k)*380, oy+stripH-30)
		}
	}
	shell.Body.Place(strip, 12, 624)
	shell.Body.Place(wrkit.Label("5 COLRv1 变量轴（同一字形 SWPS=-90 / 0 / +90，P4.4）", 12, 0.80, 0.86, 0.94), 12+12, 624+8)
}
