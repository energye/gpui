package main

import (
	"image"
	"image/png"
	"os"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/svg"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Two side-by-side panels: our 1:1 bake of testdata/showcase.svg and the
// resvg 0.42 reference PNG generated from the same file.
// 批 4g：烘焙分辨率=面板显示分辨率（570×356）——1:1 呈现零缩放采样，
// 消除 1024→570 单级 bilinear 的共同混叠（两侧对称模糊）。参照由 resvg
// 以同尺寸重出（--width 570 --height 356），两栏同口径。
const (
	panelW  = 570.0
	panelH  = 357.0
	docW    = 570.0
	docH    = 357.0
	panelY0 = 70.0
)

var panelAt = [2][2]float64{
	{12, panelY0},
	{600, panelY0},
}

// resourcesDir is the base for external image hrefs (batch 4c-6 API).
const resourcesDir = "examples/ui_wr_p38_svg_showcase/testdata"

var (
	bakeOnce sync.Once
	bakedRes baked
)

type baked struct {
	doc    *svg.Document
	img    *render.ImageBuf
	ref    *render.ImageBuf
	grade  float64
	err    string
	closed bool
}

var colPanelBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}

// colPanelBGf is the float form for pixel-probe expectations.
var colPanelBGf = [3]float64{0.10, 0.11, 0.13}

// parseShowcase parses the standard stress document (single source of truth
// with the resvg reference run).
func parseShowcase() (*svg.Document, error) {
	data, err := os.ReadFile(filepathOfShowcase)
	if err != nil {
		return nil, err
	}
	return svg.ParseWithConfig(data, svg.ParseConfig{ResourcesDir: resourcesDir})
}

const filepathOfShowcase = "examples/ui_wr_p38_svg_showcase/testdata/showcase.svg"

// ensureBaked renders our side and loads the reference PNG once.
func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		// 文字能力接线：svg 不做系统字体扫描，宿主经 SetFontResolver 提供
		// 家族（与 ui_wr_p38_svg_text 窗同口径）。参照渲染用 --use-font-file
		// ahem.ttf 钉死，这里同样钉 Ahem（批 4d：此前未挂 resolver，整篇
		// 文字被 fail-closed 丢弃）。
		ahemRaw, err := os.ReadFile("render/svg/testdata/p38/ahem.ttf")
		if err != nil {
			b.err = "ahem.ttf: " + err.Error()
			return
		}
		ahem, err := text.NewFontSource(ahemRaw)
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
		doc, err := parseShowcase()
		if err != nil {
			b.err = "parse: " + err.Error()
			return
		}
		b.doc = doc
		img := doc.Render(int(docW), int(docH))
		if img == nil {
			b.err = "render nil"
			return
		}
		b.img = render.ImageBufFromImage(img)
		if b.img == nil {
			b.err = "ImageBufFromImage nil"
			return
		}
		rf, err := os.Open(filepathOfRef)
		if err != nil {
			b.err = "reference: " + err.Error()
			return
		}
		defer rf.Close()
		ref, err := png.Decode(rf)
		if err != nil {
			b.err = "reference decode: " + err.Error()
			return
		}
		b.ref = render.ImageBufFromImage(ref)
		if b.ref == nil {
			b.err = "reference ImageBuf nil"
			return
		}
		b.grade = gridGrade(img, ref)
	})
	return &bakedRes
}

const filepathOfRef = "examples/ui_wr_p38_svg_showcase/testdata/showcase_ref.png"

func closeBaked() {
	b := &bakedRes
	if b.closed {
		return
	}
	if b.img != nil {
		b.img.Dispose()
		b.img = nil
	}
	if b.ref != nil {
		b.ref.Dispose()
		b.ref = nil
	}
	b.closed = true
}

// gridGrade is the conformance-style 48×48 mean per-channel delta.
func gridGrade(got, want image.Image) float64 {
	const gridN = 48
	b := want.Bounds()
	bw, bh := b.Dx(), b.Dy()
	sum := 0.0
	n := 0
	for gy := 0; gy < gridN; gy++ {
		for gx := 0; gx < gridN; gx++ {
			x0, x1 := gx*bw/gridN, (gx+1)*bw/gridN
			y0, y1 := gy*bh/gridN, (gy+1)*bh/gridN
			var gs, ws [4]float64
			cnt := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					gr, gg, gb, ga := got.At(x, y).RGBA()
					wr, wg, wb, wa := want.At(x, y).RGBA()
					gs[0] += float64(gr >> 8)
					gs[1] += float64(gg >> 8)
					gs[2] += float64(gb >> 8)
					gs[3] += float64(ga >> 8)
					ws[0] += float64(wr >> 8)
					ws[1] += float64(wg >> 8)
					ws[2] += float64(wb >> 8)
					ws[3] += float64(wa >> 8)
					cnt++
				}
			}
			if cnt == 0 {
				continue
			}
			for c := 0; c < 4; c++ {
				d := gs[c]/float64(cnt) - ws[c]/float64(cnt)
				if d < 0 {
					d = -d
				}
				sum += d
				n++
			}
		}
	}
	return sum / float64(n)
}

func buildSVGScene(shell *wrkit.ShellChrome) {
	panel := func(i int, title string, drawRef bool) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = panelW, panelH
		box.OnPaint = func(pc *rendering.PaintContext, sz rendering.Size) {
			if pc == nil || pc.DC == nil {
				return
			}
			dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
			dc.SetRGBA(colPanelBG.R, colPanelBG.G, colPanelBG.B, colPanelBG.A)
			dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
			_ = dc.Fill()
			b := ensureBaked()
			if b.err != "" {
				dc.SetRGBA(0.35, 0.06, 0.08, 1)
				dc.DrawRectangle(ox, oy, sz.Width, sz.Height)
				_ = dc.Fill()
				return
			}
			if drawRef {
				rendering.DrawImageBuf(pc, b.ref, 0, 0, panelW, panelH)
			} else {
				rendering.DrawImageBuf(pc, b.img, 0, 0, panelW, panelH)
			}
		}
		shell.Body.Place(box, panelAt[i][0], panelAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			panelAt[i][0], panelAt[i][1]-20)
	}

	panel(0, "本引擎渲染 (570×357 1:1)", false)
	panel(1, "resvg 0.42 参照 (同文件 Ahem 钉死)", true)
}
