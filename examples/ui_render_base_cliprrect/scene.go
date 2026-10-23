package main

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/examples/wrkit"
	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// Cards: 2x2 grid inside the shell body panel (body-relative coords).
const (
	cardW, cardH = 430.0, 232.0
	contentX0    = 12.0
	contentY0    = 36.0
)

var cardAt = [4][2]float64{{12, 44}, {462, 44}, {12, 296}, {462, 296}}

// Clip and draw geometry in card-content coordinates. The draw rects are
// deliberately larger than the clips so a missing rounded clip shows image
// or text in the corner cut-outs; the probes read these same constants.
type clipSpec struct {
	x, y, w, h, r float64
}

type dstSpec struct {
	x, y, w, h float64
}

var (
	// A: zero-readback GPU texture under a rounded clip (textured_quad).
	aClip = clipSpec{x: 108, y: 28, w: 184, h: 146, r: 38}
	aDst  = dstSpec{x: 84, y: 8, w: 232, h: 188}

	// B: ImageBuf under a rounded clip (image pipeline).
	bClip = clipSpec{x: 28, y: 28, w: 184, h: 146, r: 38}
	bDst  = dstSpec{x: 4, y: 8, w: 232, h: 188}

	// C: glyph runs under a rounded clip (text/glyph pipeline).
	cClip = clipSpec{x: 16, y: 26, w: 390, h: 150, r: 36}

	// D: Push/Pop restores the clip; left is rounded, right is unclipped.
	dClip  = clipSpec{x: 12, y: 28, w: 170, h: 146, r: 32}
	dDst   = dstSpec{x: 0, y: 10, w: 194, h: 180}
	dRight = dstSpec{x: 216, y: 28, w: 176, h: 150}
)

const textLine = "RRECT CLIP TEXT MMMM"

// Palette. Pixel and logic probes read these same values; the source texture
// is inverted on the GPU, so its expected color is 1-colQuad*.
var (
	colCardBG = render.RGBA{R: 0.10, G: 0.11, B: 0.13, A: 1}
	colQuadTL = render.RGBA{R: 0.18, G: 0.32, B: 0.82, A: 1}
	colQuadTR = render.RGBA{R: 0.82, G: 0.24, B: 0.18, A: 1}
	colQuadBL = render.RGBA{R: 0.20, G: 0.72, B: 0.26, A: 1}
	colQuadBR = render.RGBA{R: 0.88, G: 0.78, B: 0.20, A: 1}
	colText   = render.RGBA{R: 0.98, G: 0.98, B: 0.98, A: 1}
)

type baked struct {
	quad *render.ImageBuf
	face text.Face

	texCtx  *render.Context
	texView gpucontext.TextureView
	texW    int
	texH    int
	texOK   bool
	texGPU  int
	texCPU  int
	bakeErr string
}

var (
	bakeOnce sync.Once
	bakedRes baked
	liveTex  atomic.Int64
)

func ensureBaked() *baked {
	bakeOnce.Do(func() {
		b := &bakedRes
		img, err := makeQuadImage(200, 160)
		if err != nil {
			b.bakeErr = "image: " + err.Error()
			return
		}
		b.quad = img

		face, _, err := text.LoadDefaultFace(40)
		if err != nil {
			b.bakeErr = "font: " + err.Error()
			return
		}
		b.face = face

		// The source is rendered and inverted on the GPU, then published as a
		// texture view. Card A samples that view through DrawGPUTexture every
		// frame: no readback and no CPU upload on the present route.
		tc := render.NewContext(200, 160)
		drawQuadrants(tc, 200, 160)
		tc.ResetRenderPathStats()
		tc.ApplyInvert()
		stats := tc.RenderPathStats()
		b.texGPU, b.texCPU = stats.GPUOps, stats.CPUFallbackOps
		view, w, h, ok := tc.GPUFilterTexture()
		if !ok || view.IsNil() || w != 200 || h != 160 {
			tc.Close()
			b.bakeErr = fmt.Sprintf("GPUFilterTexture: ok=%v nil=%v size=%dx%d", ok, view.IsNil(), w, h)
			return
		}
		b.texCtx, b.texView, b.texW, b.texH, b.texOK = tc, view, w, h, true
	})
	return &bakedRes
}

func closeBaked() {
	b := &bakedRes
	if b.texCtx != nil {
		b.texCtx.Close()
		b.texCtx = nil
	}
	b.texView = gpucontext.TextureView{}
	b.texOK = false
}

func makeQuadImage(w, h int) (*render.ImageBuf, error) {
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil {
		return nil, err
	}
	cols := [4]render.RGBA{colQuadTL, colQuadTR, colQuadBL, colQuadBR}
	hw, hh := w/2, h/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := 0
			if x >= hw {
				i++
			}
			if y >= hh {
				i += 2
			}
			c := cols[i]
			if err := img.SetRGBA(x, y, to8(c.R), to8(c.G), to8(c.B), to8(c.A)); err != nil {
				return nil, err
			}
		}
	}
	return img, nil
}

func to8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

func drawQuadrants(dc *render.Context, w, h int) {
	hw, hh := float64(w/2), float64(h/2)
	quad := func(x, y, ww, hh float64, c render.RGBA) {
		dc.SetRGBA(c.R, c.G, c.B, c.A)
		dc.DrawRectangle(x, y, ww, hh)
		_ = dc.Fill()
	}
	quad(0, 0, hw, hh, colQuadTL)
	quad(hw, 0, float64(w)-hw, hh, colQuadTR)
	quad(0, hh, hw, float64(h)-hh, colQuadBL)
	quad(hw, hh, float64(w)-hw, float64(h)-hh, colQuadBR)
}

func drawQuadImage(pc *rendering.PaintContext, dst dstSpec) {
	b := ensureBaked()
	if pc == nil || pc.DC == nil || b.quad == nil {
		return
	}
	ax, ay := pc.Abs(dst.x, dst.y)
	pc.DC.DrawImageEx(b.quad, render.DrawImageOptions{
		X: ax, Y: ay, DstWidth: dst.w, DstHeight: dst.h,
		Interpolation: render.InterpNearest,
	})
}

func cardBG(dc render.Canvas, ox, oy, w, h float64) {
	dc.SetRGBA(colCardBG.R, colCardBG.G, colCardBG.B, colCardBG.A)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
}

func buildClipScene(shell *wrkit.ShellChrome) {
	card := func(i int, title string, paint func(*rendering.PaintContext, rendering.Size)) {
		box := rendering.NewRenderBox()
		box.FixedWidth, box.FixedHeight = cardW, cardH
		box.OnPaint = paint
		shell.Body.Place(box, cardAt[i][0], cardAt[i][1])
		shell.Body.Place(wrkit.Label(title, 12, 0.80, 0.86, 0.94),
			cardAt[i][0]+12, cardAt[i][1]+10)
	}
	card(0, "A DrawGPUTexture · ClipRoundRect：圆角外必须透出卡底", paintA)
	card(1, "B DrawImageEx · ClipRoundRect：图片四角被圆角裁掉", paintB)
	card(2, "C DrawString · ClipRoundRect：字形越界被圆角裁掉", paintC)
	card(3, "D Push/Pop：左圆角裁图 / 右同图未裁剪（证明恢复）", paintD)
}

func paintA(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	b := ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if !b.texOK {
		dc.SetRGBA(0.35, 0.06, 0.08, 1)
		dc.DrawRectangle(ox+aClip.x, oy+aClip.y, aClip.w, aClip.h)
		_ = dc.Fill()
		return
	}
	dc.Push()
	dc.ClipRoundRect(ox+aClip.x, oy+aClip.y, aClip.w, aClip.h, aClip.r)
	if rendering.DrawGPUTexture(pc, b.texView, ox+aDst.x, oy+aDst.y, int(aDst.w), int(aDst.h)) {
		liveTex.Add(1)
	}
	dc.Pop()
}

func paintB(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	dc.Push()
	dc.ClipRoundRect(ox+bClip.x, oy+bClip.y, bClip.w, bClip.h, bClip.r)
	drawQuadImage(pc, bDst)
	dc.Pop()
}

func paintC(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	b := ensureBaked()
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	if b.face == nil {
		return
	}
	dc.Push()
	dc.ClipRoundRect(ox+cClip.x, oy+cClip.y, cClip.w, cClip.h, cClip.r)
	drawTextRows(dc, ox, oy, b.face)
	dc.Pop()
}

func paintD(pc *rendering.PaintContext, sz rendering.Size) {
	if pc == nil || pc.DC == nil {
		return
	}
	dc, ox, oy := pc.DC, pc.OriginX, pc.OriginY
	cardBG(dc, ox, oy, sz.Width, sz.Height)
	dc.Push()
	dc.ClipRoundRect(ox+dClip.x, oy+dClip.y, dClip.w, dClip.h, dClip.r)
	drawQuadImage(pc, dDst)
	dc.Pop()
	drawQuadImage(pc, dRight)
}
