package main

import (
	"fmt"
	"image"
	"math"
	"sync"

	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Window layout: four narrow cards + one wide paragraph card.
const (
	cardW, cardH         = 430.0, 196.0
	wideCardW            = 880.0
	contentX0, contentY0 = 12.0, 34.0
	contentW, contentH   = 406.0, 150.0
	wideContentW         = 856.0
)

var cardAt = [5][2]float64{
	{12, 40},  // A DrawShadow
	{462, 40}, // B DrawColor
	{12, 248}, // C DrawPaint
	{462, 248},
	{12, 456}, // E DrawParagraph (wide)
}

var cardSize = [5][2]float64{
	{cardW, cardH},
	{cardW, cardH},
	{cardW, cardH},
	{cardW, cardH},
	{wideCardW, cardH},
}

var (
	colCardBG = [3]float64{0.10, 0.11, 0.13}
	colBorder = [3]float64{0.32, 0.36, 0.44}

	lightBG = render.RGBA{R: 0.94, G: 0.955, B: 0.975, A: 1}
	inkDark = render.RGBA{R: 0.08, G: 0.10, B: 0.16, A: 1}
)

type bakedCard struct {
	img *render.ImageBuf
	std image.Image
	w   int
	h   int

	gpuOps       int
	cpuFallbacks int
}

var (
	miscBakeOnce sync.Once
	miscCards    [5]*bakedCard
)

func warmMiscGPU() {
	dc := render.NewContext(8, 8)
	dc.ClearWithColor(render.RGBA{R: 1, A: 1})
	dc.SetRGB(0, 1, 0)
	dc.DrawRectangle(0, 0, 8, 8)
	_ = dc.Fill()
	_ = dc.Image()
	_ = dc.Close()
}

func bakeMiscCard(w, h int, draw func(*render.Context)) *bakedCard {
	dc := render.NewContext(w, h)
	draw(dc)
	stats := dc.RenderPathStats()
	img := dc.Image()
	_ = dc.Close()
	return &bakedCard{
		img:          render.ImageBufFromImage(img),
		std:          img,
		w:            w,
		h:            h,
		gpuOps:       stats.GPUOps,
		cpuFallbacks: stats.CPUFallbackOps,
	}
}

// ---------------------------------------------------------------------------
// Card A: DrawShadow
// ---------------------------------------------------------------------------

func shadowRectPath(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x, y)
	p.LineTo(x+w, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
	return p
}

func shadowCirclePath(cx, cy, r float64) *render.Path {
	p := render.NewPath()
	const n = 48
	for i := 0; i < n; i++ {
		a := float64(i) * 2 * math.Pi / n
		x, y := cx+math.Cos(a)*r, cy+math.Sin(a)*r
		if i == 0 {
			p.MoveTo(x, y)
		} else {
			p.LineTo(x, y)
		}
	}
	p.Close()
	return p
}

func shadowTrianglePath(x, y, w, h float64) *render.Path {
	p := render.NewPath()
	p.MoveTo(x+w/2, y)
	p.LineTo(x+w, y+h)
	p.LineTo(x, y+h)
	p.Close()
	return p
}

func drawShadowCard(dc *render.Context) {
	dc.ClearWithColor(lightBG)

	// Three silhouettes at increasing elevation. DrawShadow paints only the
	// soft, offset shadow; the colored occluders are drawn separately.
	rectPath := shadowRectPath(22, 18, 82, 48)
	dc.DrawShadow(rectPath, render.RGBA{A: 1}, 8, false)
	dc.SetRGBA(0.23, 0.51, 0.96, 1)
	dc.DrawPath(rectPath)
	_ = dc.Fill()

	circlePath := shadowCirclePath(178, 50, 28)
	dc.DrawShadow(circlePath, render.RGBA{A: 1}, 16, false)
	dc.SetRGBA(0.96, 0.45, 0.09, 1)
	dc.DrawPath(circlePath)
	_ = dc.Fill()

	triPath := shadowTrianglePath(250, 18, 72, 50)
	dc.DrawShadow(triPath, render.RGBA{A: 1}, 24, false)
	dc.SetRGBA(0.14, 0.72, 0.42, 1)
	dc.DrawPath(triPath)
	_ = dc.Fill()

	// A small annotation strip makes the shadow direction obvious to the eye.
	dc.SetRGBA(0.55, 0.58, 0.64, 1)
	dc.DrawRectangle(18, 136, 370, 1)
	_ = dc.Fill()
}

// ---------------------------------------------------------------------------
// Card B: DrawColor
// ---------------------------------------------------------------------------

func drawColorCard(dc *render.Context) {
	dc.ClearWithColor(lightBG)
	bands := []render.RGBA{
		{R: 1, G: 0.12, B: 0.10, A: 1},
		{R: 0.10, G: 0.88, B: 0.28, A: 1},
		{R: 0.12, G: 0.35, B: 0.98, A: 1},
		{R: 0.98, G: 0.82, B: 0.10, A: 1},
	}
	xs := []float64{0, 102, 204, 305}
	for i, c := range bands {
		w := 102.0
		if i == 3 {
			w = contentW - 305
		}
		dc.SetRGBA(c.R, c.G, c.B, 1)
		dc.DrawRectangle(xs[i], 0, w, contentH)
		_ = dc.Fill()
	}

	// The current paint must be ignored by DrawColor.
	dc.SetRGBA(0.3, 0.8, 0.5, 0.2)
	dc.ClipRect(0, 9, contentW, 40)
	dc.DrawColor(render.RGBA{R: 0.9, G: 0.9, B: 0.9, A: 1}, render.BlendMultiply)
	dc.ResetClip()

	dc.ClipRect(0, 101, contentW, 40)
	dc.DrawColor(render.RGBA{R: 0.2, G: 0.3, B: 0.8, A: 1}, render.BlendScreen)
	dc.ResetClip()

	// A rounded clip proves DrawColor paints the clip region, not a rectangle.
	dc.ClipRoundRect(326, 55, 64, 40, 18)
	dc.DrawColor(render.RGBA{R: 1, G: 1, B: 1, A: 1}, render.BlendNormal)
	dc.ResetClip()
}

// ---------------------------------------------------------------------------
// Card C: DrawPaint
// ---------------------------------------------------------------------------

func drawPaintCard(dc *render.Context) {
	dc.ClearWithColor(lightBG)

	// A current path is left behind deliberately; DrawPaint must not consume it.
	dc.ClipRoundRect(16, 16, 174, 118, 22)
	dc.DrawRectangle(40, 52, 24, 24)
	g1 := render.NewLinearGradientBrush(16, 0, 190, 0)
	g1.AddColorStop(0, render.RGBA{R: 0.95, G: 0.14, B: 0.18, A: 1})
	g1.AddColorStop(0.52, render.RGBA{R: 0.48, G: 0.18, B: 0.92, A: 1})
	g1.AddColorStop(1, render.RGBA{R: 0.10, G: 0.38, B: 0.98, A: 1})
	dc.SetFillBrush(g1)
	dc.DrawPaint()
	dc.SetRGBA(0.10, 0.84, 0.34, 1)
	_ = dc.Fill() // the pre-DrawPaint path must still be alive
	dc.ResetClip()

	dc.ClipRect(208, 16, 182, 118)
	g2 := render.NewLinearGradientBrush(208, 16, 390, 134)
	g2.AddColorStop(0, render.RGBA{R: 1, G: 0.90, B: 0.15, A: 1})
	g2.AddColorStop(1, render.RGBA{R: 0.06, G: 0.60, B: 0.62, A: 1})
	dc.SetFillBrush(g2)
	dc.DrawPaint()
	dc.ResetClip()
}

// ---------------------------------------------------------------------------
// Card D: ClipBounds
// ---------------------------------------------------------------------------

func drawBoundsFrame(dc *render.Context, b image.Rectangle, col render.RGBA, width float64) {
	dc.SetRGBA(col.R, col.G, col.B, col.A)
	x0, y0 := float64(b.Min.X), float64(b.Min.Y)
	x1, y1 := float64(b.Max.X), float64(b.Max.Y)
	dc.DrawRectangle(x0, y0, x1-x0, width)
	_ = dc.Fill()
	dc.DrawRectangle(x0, y1-width, x1-x0, width)
	_ = dc.Fill()
	dc.DrawRectangle(x0, y0, width, y1-y0)
	_ = dc.Fill()
	dc.DrawRectangle(x1-width, y0, width, y1-y0)
	_ = dc.Fill()
}

func drawClipBoundsCard(dc *render.Context) {
	dc.ClearWithColor(lightBG)

	// Panel 1: a plain rectangle clip.
	dc.ClipRect(10, 20, 80, 60)
	dc.SetRGBA(0.98, 0.55, 0.12, 1)
	dc.DrawRectangle(0, 0, contentW, contentH)
	_ = dc.Fill()
	drawBoundsFrame(dc, dc.ClipBounds(), render.RGBA{R: 0.06, G: 0.25, B: 0.75, A: 1}, 2)
	dc.ResetClip()

	// Panel 2: nested rectangles; ClipBounds returns the intersection.
	dc.ClipRect(110, 20, 80, 60)
	dc.SetRGBA(0.98, 0.82, 0.18, 1)
	dc.DrawRectangle(0, 0, contentW, contentH)
	_ = dc.Fill()
	outer := dc.ClipBounds()
	dc.ClipRect(130, 35, 40, 30)
	dc.SetRGBA(0.84, 0.12, 0.55, 1)
	dc.DrawRectangle(0, 0, contentW, contentH)
	_ = dc.Fill()
	inner := dc.ClipBounds()
	drawBoundsFrame(dc, outer, render.RGBA{R: 0.06, G: 0.25, B: 0.75, A: 1}, 2)
	drawBoundsFrame(dc, inner, render.RGBA{R: 0.12, G: 0.66, B: 0.24, A: 1}, 2)
	dc.ResetClip()

	// Panel 3: a rotated path clip. ClipBounds is the conservative local AABB.
	dc.Push()
	dc.Translate(282, 62)
	dc.Rotate(20 * math.Pi / 180)
	p := shadowRectPath(-28, -28, 56, 56)
	dc.DrawPath(p)
	dc.Clip()
	dc.SetRGBA(0.10, 0.72, 0.78, 1)
	dc.DrawRectangle(0, 0, contentW, contentH)
	_ = dc.Fill()
	drawBoundsFrame(dc, dc.ClipBounds(), render.RGBA{R: 0.72, G: 0.10, B: 0.18, A: 1}, 2)
	dc.Pop()
}

// ---------------------------------------------------------------------------
// Card E: DrawParagraph
// ---------------------------------------------------------------------------

func drawParagraphCard(dc *render.Context) {
	dc.ClearWithColor(lightBG)
	face := wrkit.FaceAt(18)
	if face == nil {
		dc.SetRGBA(0.8, 0.1, 0.1, 1)
		dc.DrawRectangle(20, 20, 200, 20)
		_ = dc.Fill()
		return
	}
	left := text.LayoutText(
		"DrawParagraph reuses shaped runs and\npaints every line at its baseline.\nWrap and alignment stay in layout.",
		face,
		text.LayoutOptions{MaxWidth: 420, LineSpacing: 1.25},
	)
	right := text.LayoutText(
		"一次排版，多次绘制；\n中英混排共享同一入口。\n右侧段落验证对齐与多行基线。",
		face,
		text.LayoutOptions{MaxWidth: 380, LineSpacing: 1.25, Alignment: text.AlignRight},
	)
	// Avoid a blank right column if the CJK fallback chain is unavailable.
	if right == nil || len(right.Lines) == 0 {
		right = text.LayoutText(
			"One layout, many draws.\nA second paragraph.\nRight-aligned baseline.",
			face,
			text.LayoutOptions{MaxWidth: 380, LineSpacing: 1.25, Alignment: text.AlignRight},
		)
	}
	dc.SetRGBA(inkDark.R, inkDark.G, inkDark.B, 1)
	dc.DrawParagraph(left, 20, 18)
	dc.SetRGBA(0.54, 0.12, 0.16, 1)
	dc.DrawParagraph(right, 452, 18)
}

// ---------------------------------------------------------------------------
// Bake + paint
// ---------------------------------------------------------------------------

func ensureMiscBaked() {
	miscBakeOnce.Do(func() {
		warmMiscGPU()
		miscCards[0] = bakeMiscCard(int(contentW), int(contentH), drawShadowCard)
		miscCards[1] = bakeMiscCard(int(contentW), int(contentH), drawColorCard)
		miscCards[2] = bakeMiscCard(int(contentW), int(contentH), drawPaintCard)
		miscCards[3] = bakeMiscCard(int(contentW), int(contentH), drawClipBoundsCard)
		miscCards[4] = bakeMiscCard(int(wideContentW), int(contentH), drawParagraphCard)
	})
}

func buildMiscScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardSize[i][0], cardSize[i][1]
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A DrawShadow · elevation 8/16/24，只画投影不画遮挡体", paintMiscA)
	card(1, "B DrawColor · Multiply / Screen / Normal（当前画笔被忽略）", paintMiscB)
	card(2, "C DrawPaint · 渐变填满裁剪区，路径状态保留", paintMiscC)
	card(3, "D ClipBounds · 矩形 / 嵌套 / 旋转路径的本地保守界", paintMiscD)
	card(4, "E DrawParagraph · 预排版多行 / 中英混排 / 对齐", paintMiscE)
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG[0], colCardBG[1], colCardBG[2], 1)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
	dc.SetRGBA(colBorder[0], colBorder[1], colBorder[2], 1)
	dc.DrawRectangle(ox, oy, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(ox, oy+h-1, w, 1)
	_ = dc.Fill()
	dc.DrawRectangle(ox, oy, 1, h)
	_ = dc.Fill()
	dc.DrawRectangle(ox+w-1, oy, 1, h)
	_ = dc.Fill()
}

func drawBakedCard(pc *rendering.PaintContext, idx int) {
	if pc == nil || pc.DC == nil {
		return
	}
	ensureMiscBaked()
	ox, oy := pc.OriginX, pc.OriginY
	w, h := cardSize[idx][0], cardSize[idx][1]
	cardBG(pc.DC, ox, oy, w, h)
	c := miscCards[idx]
	if c == nil || c.img == nil {
		return
	}
	cw, ch := contentW, contentH
	if idx == 4 {
		cw = wideContentW
	}
	ax, ay := pc.Abs(contentX0, contentY0)
	pc.DC.DrawImageEx(c.img, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: cw, DstHeight: ch, Interpolation: render.InterpNearest,
	})
}

func paintMiscA(pc *rendering.PaintContext, _ rendering.Size) { drawBakedCard(pc, 0) }
func paintMiscB(pc *rendering.PaintContext, _ rendering.Size) { drawBakedCard(pc, 1) }
func paintMiscC(pc *rendering.PaintContext, _ rendering.Size) { drawBakedCard(pc, 2) }
func paintMiscD(pc *rendering.PaintContext, _ rendering.Size) { drawBakedCard(pc, 3) }
func paintMiscE(pc *rendering.PaintContext, _ rendering.Size) { drawBakedCard(pc, 4) }

func miscSceneSummary() string {
	if miscCards[0] == nil || miscCards[4] == nil {
		return "not baked"
	}
	return fmt.Sprintf("A gpu=%d/%d B gpu=%d/%d C gpu=%d/%d D gpu=%d/%d E gpu=%d/%d",
		miscCards[0].gpuOps, miscCards[0].cpuFallbacks,
		miscCards[1].gpuOps, miscCards[1].cpuFallbacks,
		miscCards[2].gpuOps, miscCards[2].cpuFallbacks,
		miscCards[3].gpuOps, miscCards[3].cpuFallbacks,
		miscCards[4].gpuOps, miscCards[4].cpuFallbacks)
}
