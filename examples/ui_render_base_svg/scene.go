package main

import (
	"fmt"
	"image"
	"image/color"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	iconSize     = 180.0
	iconX        = (cardW - iconSize) / 2
	iconY        = 36.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// The four SVGs deliberately cover the paths that used to be weak:
//
//	A filled circle (baseline),
//	B root fill="none" + stroke-only (inheritance),
//	C nested <g transform> (transform traversal),
//	D color override (theming, including keeping "none" transparent).
const (
	svgFilled = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
  <circle cx="8" cy="8" r="6" fill="#E24A4A"/>
</svg>`

	svgStroke = `<svg xmlns="http://www.w3.org/2000/svg" width="100%" height="100%" viewBox="0 0 16 16" fill="none">
  <circle cx="8" cy="8" r="6" stroke="#4A90E2" stroke-width="2"/>
</svg>`

	svgNested = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
  <g fill="#3CCF7A" transform="translate(2 2)">
    <rect x="1" y="1" width="4" height="4"/>
    <rect x="7" y="7" width="4" height="4"/>
  </g>
</svg>`

	svgOverride = `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 16 16">
  <circle cx="8" cy="8" r="6" fill="#FFFFFF"/>
</svg>`
)

var (
	colCardBG   = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
	colFilled   = render.RGBA{R: 0.886, G: 0.290, B: 0.290, A: 1}
	colStroke   = render.RGBA{R: 0.290, G: 0.565, B: 0.886, A: 1}
	colNested   = render.RGBA{R: 0.235, G: 0.812, B: 0.478, A: 1}
	colOverride = render.RGBA{R: 0.753, G: 0.376, B: 1.0, A: 1}
)

type baked struct {
	icons  [4]*render.ImageBuf
	err    string
	closed bool
}

var (
	bakeOnce sync.Once
	bakedRes baked
)

func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		specs := []struct {
			src    string
			color  bool
			target render.RGBA
		}{
			{src: svgFilled},
			{src: svgStroke},
			{src: svgNested},
			{src: svgOverride, color: true, target: colOverride},
		}
		for i, sp := range specs {
			var img *image.RGBA
			var err error
			if sp.color {
				img, err = svg.RenderWithColor([]byte(sp.src), 256, 256, imageColor(sp.target))
			} else {
				img, err = svg.Render([]byte(sp.src), 256, 256)
			}
			if err != nil {
				b.err = fmt.Sprintf("card %d: %v", i, err)
				return
			}
			b.icons[i] = render.ImageBufFromImage(img)
			if b.icons[i] == nil {
				b.err = fmt.Sprintf("card %d: ImageBufFromImage returned nil", i)
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
	for i := range b.icons {
		if b.icons[i] != nil {
			b.icons[i].Dispose()
			b.icons[i] = nil
		}
	}
	b.closed = true
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG.R, colCardBG.G, colCardBG.B, colCardBG.A)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func buildSVGScene(shell *wrkit.ShellChrome) {
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
			if b.err != "" || b.icons[i] == nil {
				dc.SetRGBA(0.35, 0.06, 0.08, 1)
				dc.DrawRectangle(ox+iconX, oy+iconY, iconSize, iconSize)
				_ = dc.Fill()
				return
			}
			rendering.DrawImageBuf(pc, b.icons[i], iconX, iconY, iconSize, iconSize)
		}
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}

	card(0, "A 填充圆：SVG parse → raster → GPU image draw")
	card(1, "B root fill=none + stroke：继承 none，圆心必须透明")
	card(2, "C <g transform>：两块绿方块跟随平移")
	card(3, "D RenderWithColor：白色填充被主题紫色替换")
}
