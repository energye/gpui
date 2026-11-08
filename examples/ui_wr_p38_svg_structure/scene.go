package main

import (
	"encoding/base64"
	"fmt"
	"image/color"
	"strings"
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

// 批 4c 卡 1：switch 条件处理（systemLanguage 命中/requiredFeatures 失败/
// display:none 选中不回退/条件失败连子树隐藏）。
const switchSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <switch id="sw1">
    <rect id="sw1_hit" x="30" y="60" width="140" height="70" fill="#3ccf7a" systemLanguage="en"/>
    <rect id="sw1_fallback" x="30" y="60" width="140" height="70" fill="#e24a4a"/>
  </switch>
  <text x="30" y="45" fill="#9aa4b0" font-size="13">systemLanguage=en 命中</text>
  <switch id="sw2">
    <rect id="sw2_hidden" x="200" y="60" width="140" height="70" fill="#e24a4a" display="none"/>
    <rect id="sw2_after" x="200" y="60" width="140" height="70" fill="#e24a4a"/>
  </switch>
  <text x="200" y="45" fill="#9aa4b0" font-size="13">选中 display:none → 空（不回退）</text>
  <g id="sw3_cond" systemLanguage="fr">
    <rect id="sw3_red" x="370" y="60" width="130" height="70" fill="#e24a4a"/>
  </g>
  <rect id="sw3_green" x="370" y="150" width="130" height="70" fill="#3ccf7a"/>
  <text x="370" y="45" fill="#9aa4b0" font-size="13">fr 不命中 → 整组隐藏</text>
</svg>`

// 批 4c 卡 2：CSS（#id/[attr]/子代组合器/specificity/!important/type 校验）。
const cssSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <style>
    #css_id { fill: #3ccf7a }
    [data-k] { fill: #4a90e2 }
    g.wrap &gt; rect { fill: #e2b84a }
    #css_spec { fill: #3ccf7a }
    .cls-spec { fill: #e24a4a }
    #css_imp { fill: #e24a4a }
    #css_imp { fill: #3ccf7a !important }
  </style>
  <style type="nope-css">
    .never { fill: #e24a4a }
  </style>
  <rect id="css_id" x="30" y="60" width="140" height="60"/>
  <rect id="css_attr" x="200" y="60" width="140" height="60" data-k="1"/>
  <g class="wrap"><rect id="css_child" x="370" y="60" width="130" height="60"/></g>
  <rect id="css_spec" x="30" y="160" width="140" height="60" class="cls-spec"/>
  <rect id="css_imp" x="200" y="160" width="140" height="60"/>
  <rect id="css_type" x="370" y="160" width="130" height="60" fill="#e2b84a" class="never"/>
  <text x="30" y="45" fill="#9aa4b0" font-size="13">#id / [attr] / g&gt;rect</text>
  <text x="30" y="245" fill="#9aa4b0" font-size="13">specificity: id 胜 class（类规则在后）</text>
  <text x="200" y="245" fill="#9aa4b0" font-size="13">!important 忽略→后者胜</text>
  <text x="370" y="245" fill="#9aa4b0" font-size="13">type=nope-css 整表跳过</text>
</svg>`

// 批 4c 卡 3：image 数据源（外链 ResourcesDir/SVG 图像源/auto 等比/显式 0 跳过）。
const imageSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <image id="img_ext" x="30" y="50" width="126" href="res63.png"/>
  <image id="img_svg" x="200" y="50" width="110" height="110" href="__SVG64__"/>
  <image id="img_zero" x="360" y="50" width="130" height="0" href="res63.png"/>
  <text x="30" y="40" fill="#9aa4b0" font-size="13">外链 ResourcesDir（width=126 → 高等比）</text>
  <text x="200" y="40" fill="#9aa4b0" font-size="13">SVG 图像源（data URI 子文档）</text>
  <text x="360" y="40" fill="#9aa4b0" font-size="13">height=0 → 跳过</text>
  <text x="30" y="200" fill="#9aa4b0" font-size="13">auto 高 = 61/63×126 ≈ 122</text>
</svg>`

// 批 4c 卡 4：use/symbol opacity 组层 + transform-origin + pattern 新语义。
const useOriginSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="528" height="252" viewBox="0 0 528 252">
  <rect x="0" y="0" width="528" height="252" fill="#171a20"/>
  <defs>
    <symbol id="sym_g">
      <rect id="sym_g_rect" x="0" y="0" width="120" height="70" fill="#3ccf7a"/>
    </symbol>
    <symbol id="sym_r" opacity="0.5">
      <rect id="sym_r_rect" x="0" y="0" width="120" height="70" fill="#e24a4a"/>
    </symbol>
    <pattern id="patt1" patternUnits="userSpaceOnUse" x="330" y="60" width="60" height="60"
             patternContentUnits="objectBoundingBox">
      <rect x="0" y="0" width="0.1666" height="0.1666" fill="#808080"/>
      <rect x="0.1666" y="0.1666" width="0.1666" height="0.1666" fill="#3ccf7a"/>
    </pattern>
  </defs>
  <use id="use_op" href="#sym_g" x="30" y="60" width="120" height="70" opacity="0.5"/>
  <use id="use_sym_op" href="#sym_r" x="180" y="60" width="120" height="70"/>
  <rect id="origin_rect" x="230" y="100" width="60" height="60" fill="#4a90e2"
        transform="rotate(45)" transform-origin="center"/>
  <rect id="patt_rect" x="320" y="50" width="180" height="180" fill="url(#patt1)"/>
  <text x="30" y="45" fill="#9aa4b0" font-size="13">use opacity=0.5 组层</text>
  <text x="180" y="45" fill="#9aa4b0" font-size="13">symbol opacity=0.5</text>
  <text x="180" y="200" fill="#9aa4b0" font-size="13">rotate(45) 绕 viewBox center</text>
  <text x="320" y="245" fill="#9aa4b0" font-size="13">pattern: oBB 内容 + x/y 相位</text>
</svg>`

// svgImageURI builds the data URI for the nested-SVG image source.
func svgImageURI() string {
	doc := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"><rect width="40" height="40" fill="#e2884a"/><circle cx="20" cy="20" r="13" fill="#ffffff"/></svg>`
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(doc))
}

var (
	bakeOnce sync.Once
	bakedRes baked
)

// colCardBG is the card body backdrop (dark, same as the other P3.8 windows).
var colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}

type baked struct {
	docs   [4]*svg.Document
	icons  [4]*render.ImageBuf
	err    string
	closed bool
}

// resourcesDir is the base for external image hrefs (batch 4c-6 API).
const resourcesDir = "examples/ui_wr_p38_svg_structure/testdata"

// cardSources returns the four card SVG sources with the runtime data-URI
// placeholder expanded.
func cardSources() [4]string {
	return [4]string{
		switchSVG,
		cssSVG,
		strings.ReplaceAll(imageSVG, "__SVG64__", svgImageURI()),
		useOriginSVG,
	}
}

// parseCards parses the four cards with the batch 4c-6 config.
func parseCards() ([4]*svg.Document, error) {
	var docs [4]*svg.Document
	srcs := cardSources()
	for i, srcText := range srcs {
		doc, err := svg.ParseWithConfig([]byte(srcText), svg.ParseConfig{
			ResourcesDir: resourcesDir,
		})
		if err != nil {
			return docs, fmt.Errorf("card %d: %w", i, err)
		}
		docs[i] = doc
	}
	return docs, nil
}

// ensureBaked bakes the four cards at 1:1.
func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		docs, err := parseCards()
		if err != nil {
			b.err = err.Error()
			return
		}
		b.docs = docs
		for i, doc := range docs {
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

	card(0, "1 switch 条件处理（systemLanguage/display:none/组隐藏）")
	card(1, "2 CSS：#id/[attr]/组合器/specificity/!important/type")
	card(2, "3 image：外链 ResourcesDir/SVG 图像源/auto/零尺寸")
	card(3, "4 use·symbol opacity 组层 + transform-origin + pattern")
}

func imageColor(c render.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8(c.R*255 + 0.5),
		G: uint8(c.G*255 + 0.5),
		B: uint8(c.B*255 + 0.5),
		A: 255,
	}
}
