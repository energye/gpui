package main

import (
	"fmt"
	"image/color"
	"os"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid (1460×800 window, body 1164 wide — same geometry as the
// other P3.8 windows). AHEM is registered via svg.SetFontResolver at bake
// time; every glyph is a solid em-square so anchors are exact.
const (
	cardW, cardH = 560.0, 300.0
	innerW       = 528.0
	innerH       = 252.0
	innerX       = (cardW - innerW) / 2
	innerY       = 40.0
)

var cardAt = [4][2]float64{
	{12, 8}, {592, 8},
	{12, 316}, {592, 316},
}

const textAnchorSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <text x="20" y="60" font-family="Ahem" font-size="24" fill="#e24a4a">ab</text>
  <text x="264" y="130" font-family="Ahem" font-size="24" fill="#4a90e2" text-anchor="middle">cd</text>
  <text x="508" y="200" font-family="Ahem" font-size="24" fill="#3ccf7a" text-anchor="end">ef</text>
</svg>`

const tspanSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <text x="20" y="60" font-family="Ahem" font-size="24" fill="#e24a4a">a<tspan dx="30" fill="#3ccf7a">b</tspan><tspan x="300" dy="40" fill="#4a90e2">c</tspan></text>
  <text x="20" y="180" font-family="Ahem" font-size="24" fill="#f0c000" letter-spacing="12">hi</text>
</svg>`

const textPaintSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <linearGradient id="rg" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="240" y2="0">
      <stop offset="0" stop-color="#e24a4a"/><stop offset="1" stop-color="#4a90e2"/>
    </linearGradient>
    <clipPath id="half" clipPathUnits="userSpaceOnUse">
      <rect x="0" y="0" width="120" height="252"/>
    </clipPath>
    __MASK__
  </defs>
  <text x="20" y="100" font-family="Ahem" font-size="24" fill="url(#rg)">abcdefgh</text>
  <text x="20" y="100" font-family="Ahem" font-size="24" fill="#3ccf7a" clip-path="url(#half)">abcd</text>
  <text x="20" y="180" font-family="Ahem" font-size="24" fill="#c05df0" mask="url(#lum)">abcd</text>
</svg>`

const maskSnippet = `<mask id="lum" maskUnits="userSpaceOnUse" x="0" y="0" width="528" height="252">
    <rect x="20" y="140" width="264" height="80" fill="#ffffff"/>
    <rect x="284" y="140" width="244" height="80" fill="#000000"/>
  </mask>`

const textRotateSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <text x="120" y="90" font-family="Ahem" font-size="24" fill="#e24a4a" rotate="45 45">ab</text>
  <text x="120" y="190" font-family="Ahem" font-size="24" fill="#4a90e2" letter-spacing="18">ab</text>
  <text x="330" y="190" font-family="No-Such-Family" font-size="24" fill="#e24a4a">zz</text>
  <rect x="330" y="210" width="60" height="12" fill="#00ff00"/>
</svg>`

var (
	colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
)

type baked struct {
	icons  [4]*render.ImageBuf
	docs   [4]*svg.Document
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
		raw, err := os.ReadFile("render/svg/testdata/p38/ahem.ttf")
		if err != nil {
			b.err = "ahem.ttf: " + err.Error()
			return
		}
		ahem, err := text.NewFontSource(raw)
		if err != nil {
			b.err = "ahem parse: " + err.Error()
			return
		}
		svg.SetFontResolver(func(family string, weight text.FontWeight, italic bool) *text.FontSource {
			_ = weight
			_ = italic
			if family == "Ahem" {
				return ahem
			}
			return nil
		})
		textPaintResolved := replaceMask(textPaintSVG, maskSnippet)
		_ = textPaintResolved
		srcs := [4]string{textAnchorSVG, tspanSVG, textPaintResolved, textRotateSVG}
		for i, srcText := range srcs {
			doc, err := svg.Parse([]byte(srcText))
			if err != nil {
				b.err = fmt.Sprintf("card %d: %v", i, err)
				return
			}
			b.docs[i] = doc
			img := doc.Render(int(innerW), int(innerH))
			if img == nil {
				b.err = fmt.Sprintf("card %d: render nil", i)
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

func replaceMask(s, mask string) string {
	return swapOnce(s, "__MASK__", mask)
}

func swapOnce(s, old, new string) string {
	i := index(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
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
				dc.DrawRectangle(ox+innerX, oy+innerY, innerW, innerH)
				_ = dc.Fill()
				return
			}
			rendering.DrawImageBuf(pc, b.icons[i], innerX, innerY, innerW, innerH)
		}
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}

	card(0, "1 text-anchor start/middle/end（AHEM 方块）")
	card(1, "2 tspan dx/x,dy + letter-spacing")
	card(2, "3 文字 paint 管线：渐变+clip+mask")
	card(3, "4 rotate 逐字符 + spacing + 缺字体拒绝（右下绿条）")
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}
