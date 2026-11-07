package main

import (
	"bytes"
	"fmt"
	"image/color"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 3x2 grid inside the shell body panel (body-relative coords).
// Each card bakes ONE SVG document at 1:1 (bake size == viewBox), so SVG
// user units map to card-local pixels and probe coordinates are shared
// between the SVG sources and the pixel probes.
const (
	cardW, cardH = 376.0, 300.0
	innerW       = 344.0 // card inner area (viewBox == bake size == 1:1)
	innerH       = 252.0
	innerX       = (cardW - innerW) / 2 // 16
	innerY       = 40.0
)

// 1460×800 窗：左轨 284 + body 1164 宽；三列 376+12 间距恰好 1164，
// 两行 300+12 = 616 ≤ body 高 656。
var cardAt = [6][2]float64{
	{12, 8}, {400, 8}, {788, 8},
	{12, 316}, {400, 316}, {788, 316},
}

// --- Card 1/2: preserveAspectRatio nine-value grid (meet / slice) ---
// One <svg> per card: 9 nested viewports (3x3), each viewBox "0 0 20 20"
// with a quadrant pattern (TL red / TR green / BL blue / bg dark). meet
// shows alignment, slice shows cover + clip.

const cellW, cellH = 108.0, 72.0
var cellX = [3]float64{0, 118, 236}
var cellY = [3]float64{0, 90, 180}

var parNames = [3][3]string{ // [row][col] → align token (row: YMin/YMid/YMax)
	{"xMinYMin", "xMidYMin", "xMaxYMin"},
	{"xMinYMid", "xMidYMid", "xMaxYMid"},
	{"xMinYMax", "xMidYMax", "xMaxYMax"},
}

func parGridSVG(slice bool) string {
	mode := "meet"
	if slice {
		mode = "slice"
	}
	b := &bytes.Buffer{}
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">`)
	b.WriteString(`<rect x="0" y="0" width="344" height="252" fill="#171a20"/>`)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			fmt.Fprintf(b, `<svg x="%g" y="%g" width="%g" height="%g" viewBox="0 0 20 20" preserveAspectRatio="%s %s">`,
				cellX[c], cellY[r], cellW, cellH, parNames[r][c], mode)
			b.WriteString(`<rect x="0" y="0" width="20" height="20" fill="#20242c"/>`)
			b.WriteString(`<rect x="0" y="0" width="10" height="10" fill="#e24a4a"/>`)
			b.WriteString(`<rect x="10" y="0" width="10" height="10" fill="#3ccf7a"/>`)
			b.WriteString(`<rect x="0" y="10" width="10" height="10" fill="#4a90e2"/>`)
			b.WriteString(`</svg>`)
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// --- Card 3: unit system bars (1:1 px mapping, absolute/em/ex/% order) ---

const unitsSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <rect x="12" y="14" width="1in"  height="14" fill="#e24a4a"/>
  <rect x="12" y="38" width="96px" height="14" fill="#4a90e2"/>
  <rect x="12" y="62" width="72pt" height="14" fill="#3ccf7a"/>
  <rect x="12" y="86" width="6pc"  height="14" fill="#f0c000"/>
  <rect x="12" y="110" width="25.4mm" height="14" fill="#c05df0"/>
  <g font-size="20">
    <rect x="12" y="140" width="2em" height="16" fill="#e28b4a"/>
    <rect x="12" y="164" width="4ex" height="16" fill="#4ad0e2"/>
  </g>
  <rect x="12" y="192" width="50%" height="16" fill="#9ae24a"/>
  <circle cx="300" cy="200" r="10%" fill="#e24ad0"/>
</svg>`

// --- Card 4: inheritance tree + style attr + class + currentColor ---

const inheritSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <style>.warm { fill: #f0a000; }</style>
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <g fill="#e24a4a" fill-opacity="1" stroke="#4a90e2" stroke-width="3" stroke-dasharray="8 4" color="#3ccf7a">
    <g stroke-width="1.5">
      <rect x="14" y="20" width="90" height="60"/>
      <rect x="118" y="20" width="90" height="60" fill="currentColor"/>
      <rect x="222" y="20" width="90" height="60" fill="#20242c" style="fill:#f0c000"/>
      <rect x="14" y="100" width="90" height="60" fill-opacity="0.5"/>
      <rect class="warm" x="118" y="100" width="90" height="60"/>
    </g>
  </g>
</svg>`

// --- Card 5: use/symbol instantiation + dangling paint refusal ---

const useSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <rect id="chip" x="0" y="0" width="12" height="12" fill="#123456"/>
    <symbol id="box" viewBox="0 0 20 10" preserveAspectRatio="xMinYMin meet">
      <rect x="0" y="0" width="20" height="10" fill="#e24a4a"/>
      <use href="#chip" x="4" y="2"/>
    </symbol>
  </defs>
  <use href="#box" x="12" y="18" width="160" height="80"/>
  <use href="#chip" x="220" y="24"/>
  <rect x="12" y="130" width="90" height="60" fill="url(#missing-paint)"/>
  <rect x="118" y="130" width="90" height="60" fill="#3ccf7a"/>
</svg>`

// --- Card 6: nested svg viewports (xMaxYMax meet / xMidYMid slice) ---

const nestedSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <svg x="12" y="20" width="240" height="80" viewBox="0 0 20 10" preserveAspectRatio="xMaxYMax meet">
    <rect x="0" y="0" width="20" height="10" fill="#4a90e2"/>
  </svg>
  <svg x="12" y="140" width="240" height="80" viewBox="0 0 40 10" preserveAspectRatio="xMidYMid slice">
    <rect x="0" y="0" width="40" height="10" fill="#3ccf7a"/>
  </svg>
</svg>`

var (
	colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
)

type baked struct {
	icons  [6]*render.ImageBuf
	docs   [6]*svg.Document
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
		srcs := [6]string{parGridSVG(false), parGridSVG(true), unitsSVG, inheritSVG, useSVG, nestedSVG}
		for i, src := range srcs {
			doc, err := svg.Parse([]byte(src))
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

	card(0, "1 pAR meet 九值：等比缩放按九点对齐")
	card(1, "2 pAR slice 九值：cover 铺满 + 出界裁剪")
	card(2, "3 单位系统：in/px/pt/pc/mm/em/ex/%")
	card(3, "4 继承+style/class/currentColor")
	card(4, "5 use/symbol 引用 + 悬空 url 拒绝（左下空）")
	card(5, "6 嵌套 <svg> 视口：xMaxYMax meet / xMidYMid slice")
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}
