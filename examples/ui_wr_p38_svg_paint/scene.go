package main

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 3x2 grid (1460×800 window, body 1164 wide — same geometry as
// ui_wr_p38_svg_doc). Each card bakes ONE SVG document at 1:1.
const (
	cardW, cardH = 376.0, 300.0
	innerW       = 344.0
	innerH       = 252.0
	innerX       = (cardW - innerW) / 2
	innerY       = 40.0
)

var cardAt = [6][2]float64{
	{12, 8}, {400, 8}, {788, 8},
	{12, 316}, {400, 316}, {788, 316},
}

// --- Card sources (mirrors of the batch-2 unit-test samples) ---

const gradSpreadSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <linearGradient id="pad" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="90" y2="0">
      <stop offset="0" stop-color="#e24a4a"/><stop offset="1" stop-color="#4a90e2"/>
    </linearGradient>
    <linearGradient id="rep" href="#pad" spreadMethod="repeat"/>
    <linearGradient id="ref" href="#pad" spreadMethod="reflect"/>
  </defs>
  <rect x="10"  y="16" width="324" height="60" fill="url(#pad)"/>
  <rect x="10"  y="96" width="324" height="60" fill="url(#rep)"/>
  <rect x="10"  y="176" width="324" height="60" fill="url(#ref)"/>
</svg>`

const radialFocalSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <radialGradient id="spot" gradientUnits="userSpaceOnUse" cx="172" cy="126" r="150" fx="110" fy="90">
      <stop offset="0" stop-color="#ffffff"/><stop offset="1" stop-color="#000000"/>
    </radialGradient>
  </defs>
  <rect x="0" y="0" width="344" height="252" fill="url(#spot)"/>
</svg>`

const gradTransformSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <linearGradient id="ramp" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="344" y2="0">
      <stop offset="0" stop-color="#e24a4a"/><stop offset="1" stop-color="#4a90e2"/>
    </linearGradient>
    <linearGradient id="tilted" href="#ramp" gradientTransform="rotate(90 172 126)"/>
  </defs>
  <rect x="10" y="16" width="324" height="100" fill="url(#ramp)"/>
  <rect x="10" y="136" width="324" height="100" fill="none" stroke="url(#tilted)" stroke-width="16"/>
</svg>`

const clipSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <clipPath id="two" clipPathUnits="userSpaceOnUse">
      <rect x="10" y="16" width="150" height="100"/>
      <rect x="120" y="16" width="150" height="100"/>
    </clipPath>
    <clipPath id="ring" clipPathUnits="userSpaceOnUse">
      <path d="M40,150 H304 V236 H40 Z M110,168 V218 H234 V168 Z"/>
    </clipPath>
  </defs>
  <rect x="10" y="16" width="324" height="100" fill="#4a90e2" clip-path="url(#two)"/>
  <rect x="40" y="150" width="264" height="86" fill="#e24a4a" clip-path="url(#ring)"/>
</svg>`

const maskSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <mask id="lum" maskUnits="userSpaceOnUse" x="10" y="16" width="324" height="220">
      <rect x="10"  y="16" width="108" height="220" fill="#ffffff"/>
      <rect x="118" y="16" width="108" height="220" fill="#000000"/>
      <rect x="226" y="16" width="108" height="220" fill="#808080"/>
    </mask>
  </defs>
  <rect x="10" y="16" width="324" height="220" fill="#4a90e2" mask="url(#lum)"/>
</svg>`

const patternImageSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="344" height="252" viewBox="0 0 344 252">
  <rect x="0" y="0" width="344" height="252" fill="#171a20"/>
  <defs>
    <!-- 内容坐标按 tile 原点归零（x/y=10/16 只定义 tile 摆放；旧样本把内容也写成
         10/16..26/32，整块落在 16×16 tile 外，resvg 同样是空白——P3.8 批②归因）。 -->
    <pattern id="chk" patternUnits="userSpaceOnUse" x="10" y="16" width="16" height="16">
      <rect x="0" y="0" width="8" height="8" fill="#e24a4a"/>
      <rect x="8" y="0" width="8" height="8" fill="#171a20"/>
      <rect x="0" y="8" width="8" height="8" fill="#171a20"/>
      <rect x="8" y="8" width="8" height="8" fill="#4a90e2"/>
    </pattern>
  </defs>
  <rect x="10" y="16" width="324" height="120" fill="url(#chk)"/>
  <image x="82" y="166" width="180" height="70" href="__DATA_URI__"/>
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
		// Image card payload: the same 4x2 sample PNG as the unit tests,
		// read from testdata at bake time (working dir = repo root).
		raw, err := os.ReadFile("render/svg/testdata/p38/img_rgba.png")
		dataURI := ""
		if err == nil {
			dataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
		}
		srcs := [6]string{gradSpreadSVG, radialFocalSVG, gradTransformSVG, clipSVG, maskSVG,
			strings.Replace(patternImageSVG, "__DATA_URI__", dataURI, 1)}
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

	card(0, "1 线性渐变 pad/repeat/reflect 三带")
	card(1, "2 径向渐变焦点 fx,fy（热点偏左上）")
	card(2, "3 渐变填充+渐变描边+gradientTransform")
	card(3, "4 clipPath 并集（上）+ winding 挖洞环（下）")
	card(4, "5 mask luminance 白显/黑隐/灰半")
	card(5, "6 pattern tile 平铺 + image data URI")
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}
