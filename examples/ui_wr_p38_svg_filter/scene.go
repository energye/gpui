package main

import (
	"fmt"
	"image/color"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid (1460×800 window, body 1164 wide — same geometry as the
// other P3.8 windows). Each card bakes ONE SVG document at 1:1.
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

// 4b-1 新原语演示卡：feMorphology 异向扩缩 + feTile/feImage。
const morphSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <filter id="dil6" x="-20%" y="-20%" width="140%" height="140%">
      <feMorphology operator="dilate" radius="6"/>
    </filter>
    <filter id="ero4" x="-20%" y="-20%" width="140%" height="140%">
      <feMorphology operator="erode" radius="4"/>
    </filter>
  </defs>
  <rect x="40" y="96" width="100" height="60" fill="#4caf50"/>
  <rect x="40" y="96" width="100" height="60" fill="#4caf50" filter="url(#dil6)" transform="translate(190,0)"/>
  <rect x="40" y="96" width="100" height="60" fill="#4caf50" filter="url(#ero4)" transform="translate(380,0)"/>
  <text x="40" y="70" fill="#9aa4b0" font-size="14">reference</text>
  <text x="230" y="70" fill="#9aa4b0" font-size="14">dilate 6</text>
  <text x="420" y="70" fill="#9aa4b0" font-size="14">erode 4</text>
</svg>`

const blurSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <filter id="b4" x="-20%" y="-20%" width="140%" height="140%">
      <feGaussianBlur stdDeviation="6"/>
    </filter>
    <filter id="b12" x="-30%" y="-30%" width="160%" height="160%">
      <feGaussianBlur stdDeviation="12"/>
    </filter>
  </defs>
  <rect x="40" y="40" width="180" height="80" fill="#e24a4a"/>
  <rect x="300" y="40" width="180" height="80" fill="#4a90e2" filter="url(#b4)"/>
  <rect x="150" y="150" width="200" height="60" fill="#3ccf7a" filter="url(#b12)"/>
</svg>`

const dropshadowSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <filter id="ds1" x="-30%" y="-30%" width="160%" height="160%">
      <feDropShadow dx="8" dy="8" stdDeviation="3" flood-color="#000000"/>
    </filter>
    <filter id="ds2" x="-30%" y="-30%" width="160%" height="160%">
      <feDropShadow dx="-6" dy="-6" stdDeviation="0" flood-color="#c05df0"/>
    </filter>
  </defs>
  <rect x="40" y="40" width="160" height="80" fill="#4a90e2" filter="url(#ds1)"/>
  <rect x="300" y="120" width="160" height="80" fill="#f0c000" filter="url(#ds2)"/>
</svg>`

const colormatrixSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <filter id="gray" x="-10%" y="-10%" width="120%" height="120%">
      <feColorMatrix type="saturate" values="0"/>
    </filter>
    <filter id="half" x="-10%" y="-10%" width="120%" height="120%">
      <feColorMatrix type="matrix" values="1 0 0 0 0  0 1 0 0 0  0 0 1 0 0  0 0 0 0.5 0"/>
    </filter>
  </defs>
  <rect x="40" y="40" width="180" height="60" fill="#e24a4a"/>
  <rect x="40" y="40" width="180" height="60" fill="#e24a4a" filter="url(#gray)" transform="translate(0,70)"/>
  <rect x="40" y="180" width="180" height="40" fill="#4a90e2" filter="url(#half)"/>
</svg>`

const projMergeSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <filter id="proj" x="-20%" y="-20%" width="150%" height="150%">
      <feGaussianBlur in="SourceAlpha" stdDeviation="3" result="b"/>
      <feOffset in="b" dx="10" dy="10" result="off"/>
      <feFlood flood-color="#000000" result="f"/>
      <feComposite in="f" in2="off" operator="in" result="shadow"/>
      <feMerge>
        <feMergeNode in="shadow"/>
        <feMergeNode in="SourceGraphic"/>
      </feMerge>
    </filter>
  </defs>
  <rect x="60" y="50" width="120" height="60" fill="#3ccf7a" filter="url(#proj)"/>
  <rect x="280" y="50" width="120" height="60" fill="#c05df0" filter="url(#proj)"/>
  <rect x="440" y="220" width="50" height="12" fill="#00ff00"/>
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
		srcs := [4]string{blurSVG, dropshadowSVG, colormatrixSVG, projMergeSVG}
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

	card(0, "1 feGaussianBlur σ=6/12（对比无滤镜）")
	card(1, "2 feDropShadow dx/dy 正负偏移+颜色")
	card(2, "3 feColorMatrix saturate(0)+半透明 alpha（transform 生效）")
	card(3, "4 投影链 blur+offset+flood+composite+merge")
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}
