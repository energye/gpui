package main

import (
	"image"
	"image/color"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
	contentW     = cardW - 2*contentX0 // 406
	contentH     = cardH - contentY0 - 12.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Image slots inside a card's content box.
const (
	halfW = 199.0
	halfX = halfW + 8.0 // 207
	topH  = 112.0
	zoomY = 120.0
	zoomW = 96.0
	zoomH = 64.0
	zoomS = 4.0 // nearest-neighbour zoom factor
)

// Palette shared by the scene and the analytic pixel oracle.
var (
	colCardBG  = [3]float64{0.10, 0.11, 0.13}
	colBorder  = [3]float64{0.32, 0.36, 0.44}
	colSolid   = [3]float64{0.40, 0.40, 0.40}
	colDiffAmp = 64.0
)

// ---------------------------------------------------------------------------
// Baked offscreen images
// ---------------------------------------------------------------------------

type bakedImage struct {
	buf          *render.ImageBuf
	std          image.Image
	unique       int
	gpuOps       int
	cpuFallbacks int
}

var (
	bakeOnce sync.Once

	imgAOff, imgAOn                          *bakedImage
	imgAZoomOff, imgAZoomOn                  *bakedImage
	imgADiff                                 *bakedImage
	imgBOff, imgBOn                          *bakedImage
	imgCOff, imgCOn                          *bakedImage
	imgDOff, imgDOn, imgDZoomOff, imgDZoomOn *bakedImage
	imgDDiff                                 *bakedImage

	ditherGPUOps       int64
	ditherCPUFallbacks int64
)

func linearBrush(w, h int) *render.LinearGradientBrush {
	return render.NewLinearGradientBrush(0, 0, 0, float64(h)).
		AddColorStop(0, render.RGBA{R: 0.20, G: 0.22, B: 0.27, A: 1}).
		AddColorStop(1, render.RGBA{R: 0.30, G: 0.33, B: 0.39, A: 1})
}

func radialBrush(w, h int) *render.RadialGradientBrush {
	return render.NewRadialGradientBrush(float64(w)*0.5, float64(h)*0.5, 0, float64(minInt(w, h))*0.52).
		AddColorStop(0, render.RGBA{R: 0.86, G: 0.90, B: 0.98, A: 1}).
		AddColorStop(0.45, render.RGBA{R: 0.24, G: 0.55, B: 0.86, A: 1}).
		AddColorStop(1, render.RGBA{R: 0.06, G: 0.09, B: 0.18, A: 1})
}

func sweepBrush(w, h int) *render.SweepGradientBrush {
	return render.NewSweepGradientBrush(float64(w)*0.5, float64(h)*0.5, 0).
		AddColorStop(0, render.Red).
		AddColorStop(0.25, render.Yellow).
		AddColorStop(0.5, render.Green).
		AddColorStop(0.75, render.Blue).
		AddColorStop(1, render.Red)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// oracleColor is the independent base-color model for a source image pixel
// center. It drives both the paint (through the same brush builders) and the
// point probes, so dither is the only extra term in the on/off comparison.
func oracleColor(kind string, x, y float64, w, h int) render.RGBA {
	switch kind {
	case "linear":
		return linearBrush(w, h).ColorAt(x, y)
	case "radial":
		return radialBrush(w, h).ColorAt(x, y)
	case "sweep":
		return sweepBrush(w, h).ColorAt(x, y)
	case "solid":
		return render.RGBA{R: colSolid[0], G: colSolid[1], B: colSolid[2], A: 1}
	}
	return render.RGBA{}
}

func drawBakedShape(dc *render.Context, kind string, w, h int) {
	switch kind {
	case "linear":
		dc.SetFillBrush(linearBrush(w, h))
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
	case "radial":
		dc.SetFillBrush(radialBrush(w, h))
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
	case "sweep":
		dc.SetFillBrush(sweepBrush(w, h))
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
	case "solid":
		dc.SetRGBA(colSolid[0], colSolid[1], colSolid[2], 1)
		dc.DrawRectangle(0, 0, float64(w), float64(h))
		_ = dc.Fill()
	}
}

func bakeImage(kind string, dither bool, w, h int) *bakedImage {
	dc := render.NewContext(w, h)
	drawBakedShape(dc, kind, w, h)
	if dither {
		dc.SetDither(true)
	}
	std := dc.Image()
	stats := dc.RenderPathStats()
	out := &bakedImage{
		buf:          render.ImageBufFromImage(std),
		std:          std,
		unique:       countUnique(std, std.Bounds()),
		gpuOps:       stats.GPUOps,
		cpuFallbacks: stats.CPUFallbackOps,
	}
	dc.Close()
	return out
}

// bakeDiffPanel turns the absolute on/off difference of the two 24x16 zoom
// images into an amplified 24x16 pattern; it is displayed with nearest 4x so
// the Bayer 4x4 cells are visible to the eye.
func bakeDiffPanel(off, on *bakedImage) *bakedImage {
	if off == nil || on == nil {
		return nil
	}
	b := off.std.Bounds()
	rgba := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r0, g0, b0, _ := off.std.At(x, y).RGBA()
			r1, g1, b1, _ := on.std.At(x, y).RGBA()
			amp := func(a, c uint32) uint8 {
				d := int(a>>8) - int(c>>8)
				if d < 0 {
					d = -d
				}
				v := d * int(colDiffAmp)
				if v > 255 {
					v = 255
				}
				return uint8(v)
			}
			rgba.SetRGBA(x, y, color.RGBA{
				R: amp(r0, r1),
				G: amp(g0, g1),
				B: amp(b0, b1),
				A: 255,
			})
		}
	}
	return &bakedImage{
		buf:    render.ImageBufFromImage(rgba),
		std:    rgba,
		unique: countUnique(rgba, rgba.Bounds()),
	}
}

func ensureBaked() {
	bakeOnce.Do(func() {
		imgAOff = bakeImage("linear", false, int(halfW), int(topH))
		imgAOn = bakeImage("linear", true, int(halfW), int(topH))
		imgAZoomOff = bakeImage("linear", false, 24, 16)
		imgAZoomOn = bakeImage("linear", true, 24, 16)
		imgADiff = bakeDiffPanel(imgAZoomOff, imgAZoomOn)
		imgBOff = bakeImage("radial", false, int(halfW), int(contentH))
		imgBOn = bakeImage("radial", true, int(halfW), int(contentH))
		imgCOff = bakeImage("sweep", false, int(halfW), int(contentH))
		imgCOn = bakeImage("sweep", true, int(halfW), int(contentH))
		imgDOff = bakeImage("solid", false, int(halfW), int(topH))
		imgDOn = bakeImage("solid", true, int(halfW), int(topH))
		imgDZoomOff = bakeImage("solid", false, 24, 16)
		imgDZoomOn = bakeImage("solid", true, 24, 16)
		imgDDiff = bakeDiffPanel(imgDZoomOff, imgDZoomOn)
		for _, b := range []*bakedImage{
			imgAOff, imgAOn, imgAZoomOff, imgAZoomOn, imgADiff,
			imgBOff, imgBOn, imgCOff, imgCOn,
			imgDOff, imgDOn, imgDZoomOff, imgDZoomOn, imgDDiff,
		} {
			if b == nil {
				continue
			}
			ditherGPUOps += int64(b.gpuOps)
			ditherCPUFallbacks += int64(b.cpuFallbacks)
		}
	})
}

func countUnique(img image.Image, r image.Rectangle) int {
	if img == nil {
		return 0
	}
	r = r.Intersect(img.Bounds())
	seen := map[[3]uint8]struct{}{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			seen[[3]uint8{uint8(cr >> 8), uint8(cg >> 8), uint8(cb >> 8)}] = struct{}{}
		}
	}
	return len(seen)
}

// ---------------------------------------------------------------------------
// Painting
// ---------------------------------------------------------------------------

func buildDitherScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A 线性渐变：off / on / 4× 放大 / 差异×64", paintA)
	card(1, "B 径向渐变：off / on", paintB)
	card(2, "C 扫掠渐变：off / on", paintC)
	card(3, "D 纯色：off / on（整张 pixmap 后处理，限界）", paintD)
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

func drawImage(pc *rendering.PaintContext, b *bakedImage, x, y, w, h float64, nearest bool) {
	if pc == nil || pc.DC == nil || b == nil || b.buf == nil {
		return
	}
	ax, ay := pc.Abs(x, y)
	if nearest {
		pc.DC.DrawImageEx(b.buf, render.DrawImageOptions{
			X: ax, Y: ay, DstWidth: w, DstHeight: h, Interpolation: render.InterpNearest,
		})
		return
	}
	pc.DC.DrawImage(b.buf, ax, ay)
}

func paintA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0

	slotFrame(dc, cx, cy, halfW, topH)
	slotFrame(dc, cx+halfX, cy, halfW, topH)
	drawImage(pc, imgAOff, contentX0, contentY0, halfW, topH, false)
	drawImage(pc, imgAOn, contentX0+halfX, contentY0, halfW, topH, false)

	slotFrame(dc, cx, cy+zoomY, zoomW, zoomH)
	slotFrame(dc, cx+halfX, cy+zoomY, zoomW, zoomH)
	drawImage(pc, imgAZoomOff, contentX0, contentY0+zoomY, zoomW, zoomH, true)
	drawImage(pc, imgAZoomOn, contentX0+halfX, contentY0+zoomY, zoomW, zoomH, true)

	slotFrame(dc, cx+310, cy+zoomY, zoomW, zoomH)
	drawImage(pc, imgADiff, contentX0+310, contentY0+zoomY, zoomW, zoomH, true)
}

func paintB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	slotFrame(dc, cx, cy, halfW, contentH)
	slotFrame(dc, cx+halfX, cy, halfW, contentH)
	drawImage(pc, imgBOff, contentX0, contentY0, halfW, contentH, false)
	drawImage(pc, imgBOn, contentX0+halfX, contentY0, halfW, contentH, false)
}

func paintC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	slotFrame(dc, cx, cy, halfW, contentH)
	slotFrame(dc, cx+halfX, cy, halfW, contentH)
	drawImage(pc, imgCOff, contentX0, contentY0, halfW, contentH, false)
	drawImage(pc, imgCOn, contentX0+halfX, contentY0, halfW, contentH, false)
}

func paintD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	cx, cy := ox+contentX0, oy+contentY0
	slotFrame(dc, cx, cy, halfW, topH)
	slotFrame(dc, cx+halfX, cy, halfW, topH)
	drawImage(pc, imgDOff, contentX0, contentY0, halfW, topH, false)
	drawImage(pc, imgDOn, contentX0+halfX, contentY0, halfW, topH, false)

	slotFrame(dc, cx, cy+zoomY, zoomW, zoomH)
	slotFrame(dc, cx+halfX, cy+zoomY, zoomW, zoomH)
	drawImage(pc, imgDZoomOff, contentX0, contentY0+zoomY, zoomW, zoomH, true)
	drawImage(pc, imgDZoomOn, contentX0+halfX, contentY0+zoomY, zoomW, zoomH, true)

	slotFrame(dc, cx+310, cy+zoomY, zoomW, zoomH)
	drawImage(pc, imgDDiff, contentX0+310, contentY0+zoomY, zoomW, zoomH, true)
}
