package main

import (
	"fmt"
	"image"
	"image/color"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/examples/wrkit"
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Image slots inside a card's content box. The upper row shows the source and
// the filtered result side by side; the lower row shows 4x nearest-neighbour
// zooms plus an amplified difference panel.
const (
	halfW = 199.0
	halfX = 207.0
	topH  = 130.0

	zoomY = 138.0
	zoomW = 96.0
	zoomH = 48.0
	zoomS = 4.0
)

var (
	colCardBG = [3]float64{0.10, 0.11, 0.13}
	colBorder = [3]float64{0.32, 0.36, 0.44}
)

// Content palette. The probes derive their expectations from the source image
// drawn into the same card, so these constants are only the input to the bake.
var (
	colA0 = render.RGBA{R: 1.00, G: 0.36, B: 0.12, A: 1}
	colA1 = render.RGBA{R: 0.12, G: 0.78, B: 0.66, A: 1}
	colA2 = render.RGBA{R: 0.22, G: 0.42, B: 0.95, A: 1}
	colA3 = render.RGBA{R: 0.90, G: 0.90, B: 0.90, A: 1}

	colBBG    = render.RGBA{R: 0.05, G: 0.08, B: 0.14, A: 1}
	colBBand  = render.RGBA{R: 0.92, G: 0.76, B: 0.18, A: 1}
	colBBar   = render.RGBA{R: 0.16, G: 0.82, B: 0.62, A: 1}
	colCTop   = render.RGBA{R: 0.90, G: 0.86, B: 0.20, A: 1}
	colCLeft  = render.RGBA{R: 0.82, G: 0.22, B: 0.12, A: 1}
	colCRight = render.RGBA{R: 0.12, G: 0.68, B: 0.82, A: 1}
	colDBase  = render.RGBA{R: 0.05, G: 0.06, B: 0.10, A: 1}
	colDBand  = render.RGBA{R: 0.95, G: 0.35, B: 0.10, A: 1}
	colDStrip = render.RGBA{R: 0.10, G: 0.85, B: 0.75, A: 1}
)

// zoomSrc is the 24x12 source region magnified 4x in the lower row.
var zoomSrc = [4]image.Rectangle{
	image.Rect(20, 26, 44, 38),
	image.Rect(20, 36, 44, 48),
	image.Rect(20, 16, 44, 28),
	image.Rect(20, 34, 44, 46),
}

type bakedImage struct {
	buf *render.ImageBuf
	std image.Image
	w   int
	h   int
}

type filterCardBaked struct {
	src  *bakedImage
	res  *bakedImage
	diff *bakedImage

	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once

	cards [4]*filterCardBaked

	// Card D publishes a GPU filter view. The view is drawn live into the
	// window every frame (zero readback); the readback copy only feeds the
	// zoom / diff panels.
	filterDView        gpucontext.TextureView
	filterDW, filterDH int
	filterDOK          bool
	filterDBeforeOK    bool
	filterDCtx         *render.Context
	filterDRefImg      image.Image
	dLivePaints        atomic.Int64
)

func drawContent(dc *render.Context, kind int) {
	if dc == nil {
		return
	}
	switch kind {
	case 0: // invert: four flat bands (opaque base, integer seams)
		dc.ClearWithColor(colA0)
		bands := []struct {
			y0, y1 float64
			c      render.RGBA
		}{
			{0, 32, colA0}, {32, 65, colA1}, {65, 97, colA2}, {97, 130, colA3},
		}
		for _, b := range bands {
			dc.SetRGBA(b.c.R, b.c.G, b.c.B, b.c.A)
			dc.DrawRectangle(0, b.y0, halfW, b.y1-b.y0)
			_ = dc.Fill()
		}
	case 1: // blurXY(0,4): horizontal band + vertical bar
		dc.SetRGBA(colBBG.R, colBBG.G, colBBG.B, 1)
		dc.DrawRectangle(0, 0, halfW, topH)
		_ = dc.Fill()
		dc.SetRGBA(colBBand.R, colBBand.G, colBBand.B, 1)
		dc.DrawRectangle(0, 44, halfW, 42)
		_ = dc.Fill()
		dc.SetRGBA(colBBar.R, colBBar.G, colBBar.B, 1)
		dc.DrawRectangle(140, 0, 30, topH)
		_ = dc.Fill()
	case 2: // graph: BlurXY(0,4) -> Grayscale -> Invert
		dc.ClearWithColor(colCTop)
		dc.SetRGBA(colCTop.R, colCTop.G, colCTop.B, 1)
		dc.DrawRectangle(0, 0, halfW, 26)
		_ = dc.Fill()
		dc.SetRGBA(colCLeft.R, colCLeft.G, colCLeft.B, 1)
		dc.DrawRectangle(0, 26, 100, topH-26)
		_ = dc.Fill()
		dc.SetRGBA(colCRight.R, colCRight.G, colCRight.B, 1)
		dc.DrawRectangle(100, 26, halfW-100, topH-26)
		_ = dc.Fill()
	case 3: // GPU texture: opaque base + band + stripe
		dc.ClearWithColor(colDBase)
		dc.SetRGBA(colDBand.R, colDBand.G, colDBand.B, 1)
		dc.DrawRectangle(0, 44, halfW, 42)
		_ = dc.Fill()
		dc.SetRGBA(colDStrip.R, colDStrip.G, colDStrip.B, 1)
		dc.DrawRectangle(100, 90, 20, topH-90)
		_ = dc.Fill()
	}
}

func bakeDrawn(w, h int, kind int) *bakedImage {
	dc := render.NewContext(w, h)
	drawContent(dc, kind)
	img := dc.Image()
	dc.Close()
	return &bakedImage{buf: render.ImageBufFromImage(img), std: img, w: w, h: h}
}

func bakeFiltered(w, h int, kind int, apply func(*render.Context)) (*bakedImage, int, int) {
	dc := render.NewContext(w, h)
	drawContent(dc, kind)
	dc.ResetRenderPathStats()
	apply(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	dc.Close()
	return &bakedImage{buf: render.ImageBufFromImage(img), std: img, w: w, h: h},
		stats.GPUOps, stats.CPUFallbackOps
}

// bakeDiff amplifies the absolute per-channel difference between two images.
// The caller samples a 24x12 region of it, so the whole image is kept.
func bakeDiff(a, b image.Image, amp float64) *bakedImage {
	if a == nil || b == nil {
		return nil
	}
	ra, rb := a.Bounds(), b.Bounds()
	w, h := ra.Dx(), ra.Dy()
	if rb.Dx() < w {
		w = rb.Dx()
	}
	if rb.Dy() < h {
		h = rb.Dy()
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, _ := a.At(ra.Min.X+x, ra.Min.Y+y).RGBA()
			r1, g1, b1, _ := b.At(rb.Min.X+x, rb.Min.Y+y).RGBA()
			amp1 := func(p, q uint32) uint8 {
				d := int(p>>8) - int(q>>8)
				if d < 0 {
					d = -d
				}
				v := float64(d) * amp
				if v > 255 {
					v = 255
				}
				return uint8(v + 0.5)
			}
			rgba.SetRGBA(x, y, color.RGBA{R: amp1(r0, r1), G: amp1(g0, g1), B: amp1(b0, b1), A: 255})
		}
	}
	return &bakedImage{buf: render.ImageBufFromImage(rgba), std: rgba, w: w, h: h}
}

// warmGPU performs one real draw+readback so the lazy accelerator (and the
// filter-graph hooks registered by it) is live before the first filter runs.
func warmGPU() {
	dc := render.NewContext(8, 8)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	_ = dc.Image()
	dc.Close()
}

func ensureBaked() {
	bakeOnce.Do(func() {
		warmGPU()
		w, h := int(halfW), int(topH)

		cards[0] = &filterCardBaked{src: bakeDrawn(w, h, 0)}
		cards[0].res, cards[0].gpuOps, cards[0].cpuFallbacks = bakeFiltered(w, h, 0, func(dc *render.Context) {
			dc.ApplyInvert()
		})
		cards[0].diff = bakeDiff(cards[0].src.std, cards[0].res.std, 32)

		cards[1] = &filterCardBaked{src: bakeDrawn(w, h, 1)}
		cards[1].res, cards[1].gpuOps, cards[1].cpuFallbacks = bakeFiltered(w, h, 1, func(dc *render.Context) {
			dc.ApplyBlurXY(0, 4)
		})
		cards[1].diff = bakeDiff(cards[1].src.std, cards[1].res.std, 32)

		cards[2] = &filterCardBaked{src: bakeDrawn(w, h, 2)}
		cards[2].res, cards[2].gpuOps, cards[2].cpuFallbacks = bakeFiltered(w, h, 2, func(dc *render.Context) {
			dc.ApplyImageFilterGraph(
				render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 0, RadiusY: 4},
				render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
				render.ImageFilterNode{Kind: render.ImageFilterInvert},
			)
		})
		cards[2].diff = bakeDiff(cards[2].src.std, cards[2].res.std, 32)

		bakeD(w, h)
	})
}

func bakeD(w, h int) {
	cards[3] = &filterCardBaked{src: bakeDrawn(w, h, 3)}

	// CPU reference: same content and blur with the GPU graph disabled. It
	// feeds the diff panel and the window's GPU-vs-CPU assertion.
	ref := render.NewContext(w, h)
	drawContent(ref, 3)
	restore := render.DisableGPUFilterGraphForTest()
	ref.ApplyBlurXY(0, 6)
	restore()
	filterDRefImg = ref.Image()
	ref.Close()

	dc := render.NewContext(w, h)
	drawContent(dc, 3)
	_, _, _, filterDBeforeOK = dc.GPUFilterTexture()
	dc.ResetRenderPathStats()
	dc.ApplyBlurXY(0, 6)
	stats := dc.RenderPathStats()
	cards[3].gpuOps = stats.GPUOps
	cards[3].cpuFallbacks = stats.CPUFallbackOps

	view, vw, vh, ok := dc.GPUFilterTexture()
	if !ok || view.IsNil() || vw != w || vh != h {
		dc.Close()
		return
	}
	filterDView, filterDW, filterDH, filterDOK = view, vw, vh, true
	filterDCtx = dc

	// One readback for the zoom / diff panels; the live card still draws the
	// published view itself, so the present path never needs this copy.
	dst := render.NewContext(w, h)
	dst.Clear()
	dst.DrawGPUTexture(view, 0, 0, w, h)
	res := dst.Image()
	dst.Close()
	cards[3].res = &bakedImage{buf: render.ImageBufFromImage(res), std: res, w: w, h: h}
	if filterDRefImg != nil {
		cards[3].diff = bakeDiff(filterDRefImg, cards[3].res.std, 64)
	}
}

func closeFilterScene() {
	if filterDCtx != nil {
		filterDCtx.Close()
		filterDCtx = nil
	}
	filterDView = gpucontext.TextureView{}
	filterDOK = false
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildFilterScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A 反色 ApplyInvert：源 / 结果（逐像素 255−src）", paintA)
	card(1, "B 纵向模糊 ApplyBlurXY(0,4)：上下软化 / 左右仍锐", paintB)
	card(2, "C 图链 BlurY→Grayscale→Invert：结果=255−luma", paintC)
	card(3, "D GPUFilterTexture：发布视图直绘（零读回）/ 底保不透明", paintD)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func slotFrame(dc render.Canvas, x, y, w, h float64) {
	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.DrawRectangle(x, y, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(x, y+h-1, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(x, y, 1, h)
	_ = dc.Fill()
	dc.DrawRectangle(x+w-1, y, 1, h)
	_ = dc.Fill()
}

func drawImageAt(pc *rendering.PaintContext, b *bakedImage, x, y, w, h float64, src *image.Rectangle) {
	if pc == nil || pc.DC == nil || b == nil || b.buf == nil {
		return
	}
	ax, ay := pc.Abs(x, y)
	if src == nil {
		pc.DC.DrawImageEx(b.buf, render.DrawImageOptions{
			X: ax, Y: ay, DstWidth: w, DstHeight: h, Interpolation: render.InterpBilinear,
		})
		return
	}
	r := *src
	pc.DC.DrawImageEx(b.buf, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: w, DstHeight: h, SrcRect: &r, Interpolation: render.InterpNearest,
	})
}

func paintCardBase(pc *rendering.PaintContext, sz rendering.Size) {
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	slotFrame(dc, cx, cy, halfW, topH)
	slotFrame(dc, cx+halfX, cy, halfW, topH)
	slotFrame(dc, cx, cy+zoomY, zoomW, zoomH)
	slotFrame(dc, cx+halfX, cy+zoomY, zoomW, zoomH)
	slotFrame(dc, cx+310, cy+zoomY, zoomW, zoomH)
}

func paintAB(pc *rendering.PaintContext, sz rendering.Size, idx int) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	paintCardBase(pc, sz)
	c := cards[idx]
	if c == nil {
		return
	}
	zr := zoomSrc[idx]
	drawImageAt(pc, c.src, contentX0, contentY0, halfW, topH, nil)
	drawImageAt(pc, c.res, contentX0+halfX, contentY0, halfW, topH, nil)
	drawImageAt(pc, c.src, contentX0, contentY0+zoomY, zoomW, zoomH, &zr)
	drawImageAt(pc, c.res, contentX0+halfX, contentY0+zoomY, zoomW, zoomH, &zr)
	drawImageAt(pc, c.diff, contentX0+310, contentY0+zoomY, zoomW, zoomH, &zr)
}

func paintA(pc *rendering.PaintContext, sz rendering.Size) { paintAB(pc, sz, 0) }
func paintB(pc *rendering.PaintContext, sz rendering.Size) { paintAB(pc, sz, 1) }
func paintC(pc *rendering.PaintContext, sz rendering.Size) { paintAB(pc, sz, 2) }

func paintD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	paintCardBase(pc, sz)
	c := cards[3]
	if c == nil {
		return
	}
	zr := zoomSrc[3]
	drawImageAt(pc, c.src, contentX0, contentY0, halfW, topH, nil)

	// Live zero-readback publish: the window samples the filter view directly,
	// exactly the production present route. If publishing failed, the slot
	// stays at the card background and the probes report it.
	if filterDOK && !filterDView.IsNil() {
		ax, ay := pc.Abs(contentX0+halfX, contentY0)
		if rendering.DrawGPUTexture(pc, filterDView, ax, ay, filterDW, filterDH) {
			dLivePaints.Add(1)
		}
	} else if pc.DC != nil {
		dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
		dc.SetRGBA(0.35, 0.06, 0.08, 1)
		dc.DrawRectangle(ox+contentX0+halfX, oy+contentY0, halfW, topH)
		_ = dc.Fill()
	}

	drawImageAt(pc, c.src, contentX0, contentY0+zoomY, zoomW, zoomH, &zr)
	drawImageAt(pc, c.res, contentX0+halfX, contentY0+zoomY, zoomW, zoomH, &zr)
	drawImageAt(pc, c.diff, contentX0+310, contentY0+zoomY, zoomW, zoomH, &zr)
}

func filterSceneSummary() string {
	if cards[0] == nil || cards[3] == nil {
		return "not baked"
	}
	return fmt.Sprintf("A gpu=%d cpu=%d | B gpu=%d cpu=%d | C gpu=%d cpu=%d | D gpu=%d cpu=%d tex=%v",
		cards[0].gpuOps, cards[0].cpuFallbacks,
		cards[1].gpuOps, cards[1].cpuFallbacks,
		cards[2].gpuOps, cards[2].cpuFallbacks,
		cards[3].gpuOps, cards[3].cpuFallbacks, filterDOK)
}
