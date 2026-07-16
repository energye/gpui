//go:build linux && !nogpu

package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
)

// Small reusable offscreen RT (Skia saveLayer / filter bounded layer pattern).
// Composite path: ExportImageBuf → DrawImage on present (pixel-correct).
// Note: FlushGPUWithView→DrawGPUTexture is NOT used here yet — offscreen RT
// content often lives on the pixmap (advanced blend / filters / image base),
// and view-only resolve produced empty/wrong window content.
type effectRT struct {
	dc     *render.Context
	img    *render.ImageBuf
	w, h   int
	closed bool
}

func (e *effectRT) ensure(w, h int) *render.Context {
	if w < 8 {
		w = 8
	}
	if h < 8 {
		h = 8
	}
	if e.dc != nil && e.w == w && e.h == h && !e.closed {
		return e.dc
	}
	if e.dc != nil {
		_ = e.dc.Close()
		e.dc = nil
	}
	e.dc = render.NewContext(w, h)
	e.w, e.h = w, h
	e.img = nil
	e.closed = false
	return e.dc
}

// blitTo composites this RT onto present with correct pixel content.
func (e *effectRT) blitTo(present *render.Context, x, y float64) bool {
	return e.blitToInterp(present, x, y, render.InterpBilinear)
}

func (e *effectRT) blitToInterp(present *render.Context, x, y float64, interp render.InterpolationMode) bool {
	if e == nil || e.dc == nil || present == nil {
		return false
	}
	if img := e.publish(); img != nil {
		present.DrawImageEx(img, render.DrawImageOptions{
			X: x, Y: y, Opacity: 1, Interpolation: interp,
		})
		return true
	}
	return false
}

// presentCached redraws last exported image without rebuild.
func (e *effectRT) presentCached(present *render.Context, x, y float64, interp render.InterpolationMode) bool {
	if e == nil || present == nil || e.img == nil {
		return false
	}
	present.DrawImageEx(e.img, render.DrawImageOptions{
		X: x, Y: y, Opacity: 1, Interpolation: interp,
	})
	return true
}

func (e *effectRT) hasCached() bool {
	return e != nil && e.img != nil
}

func (e *effectRT) publish() *render.ImageBuf {
	if e == nil || e.dc == nil {
		return nil
	}
	if !e.dc.ExportImageBuf(&e.img) {
		return nil
	}
	return e.img
}

func (e *effectRT) last() *render.ImageBuf {
	if e == nil {
		return nil
	}
	return e.img
}

var (
	gLayerRT    effectRT
	gFilterBlur effectRT
	gFilterShad effectRT
	gFilterGray effectRT
	gBackdropRT effectRT
	gAdvBlendRT effectRT
	gTextLCDRT  effectRT
	gMaskRT     effectRT
	gGradRT     effectRT
	gBlendRT    effectRT
	gPDBoardRT  effectRT
	gClipDiffRT effectRT
	gGradTileRT effectRT
	gImageAdvRT effectRT
	gTextAdvRT  effectRT
	gNineSrc    *render.ImageBuf
	gCheckerImg *render.ImageBuf
	gBlendBG    *render.ImageBuf
	gSoftMask   *render.Mask
	gMeshPos    []render.Point
	gMeshCol    []render.RGBA
	gMeshIdx    []uint16
)

func ensureChecker() *render.ImageBuf {
	if gCheckerImg != nil {
		return gCheckerImg
	}
	img, err := render.NewImageBuf(48, 48, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			on := ((x/8)+(y/8))%2 == 0
			if on {
				_ = img.SetRGBA(x, y, 40, 160, 255, 255)
			} else {
				_ = img.SetRGBA(x, y, 255, 120, 40, 255)
			}
		}
	}
	gCheckerImg = img
	return gCheckerImg
}

func ensureBlendBG(w, h int) *render.ImageBuf {
	if gBlendBG != nil {
		bw, bh := gBlendBG.Bounds()
		if bw == w && bh == h {
			return gBlendBG
		}
	}
	img, err := render.NewImageBuf(w, h, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	// Slate + light checker, baked once (opaque) for correct Export path.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if ((x/28)+(y/28))%2 == 0 {
				_ = img.SetRGBA(x, y, 64, 89, 140, 255)
			} else {
				_ = img.SetRGBA(x, y, 200, 205, 225, 255)
			}
		}
	}
	gBlendBG = img
	return gBlendBG
}

func ensureSoftMask() *render.Mask {
	if gSoftMask != nil {
		return gSoftMask
	}
	m := render.NewMask(128, 128)
	if m == nil {
		return nil
	}
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			dx := float64(x-64) / 64
			dy := float64(y-64) / 64
			d := math.Sqrt(dx*dx + dy*dy)
			a := 0.0
			if d < 1 {
				a = (1 - d) * 255
			}
			m.Set(x, y, uint8(a))
		}
	}
	gSoftMask = m
	return gSoftMask
}

// drawCapability renders one capability family for the given scenario.
// Returns a short Chinese note of expected on-screen content.
func drawCapability(dc *render.Context, fonts fontPack, kind string, fw, fh, t float64, frame int, pix []byte) string {
	if dc == nil {
		return ""
	}
	// Shared dark stage (except damage which preserves background intentionally).
	if kind != "damage" {
		dc.SetRGB(0.07, 0.08, 0.11)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
	}

	switch kind {
	case "clear":
		r, g, b := hsv(math.Mod(t*0.05, 1), 0.45, 0.22)
		dc.SetRGB(r, g, b)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		dc.SetRGBA(0.3, 0.85, 1, 0.9)
		cx := fw*0.2 + math.Mod(t*80, fw*0.6)
		dc.DrawCircle(cx, fh*0.5, 28)
		_ = dc.Fill()
		return "清屏色相变化 + 横向移动圆（S.03/S.04）"

	case "xform":
		cx, cy := fw*0.5, fh*0.52
		dc.Push()
		dc.Translate(cx, cy)
		dc.Rotate(t * 0.9)
		dc.Scale(1+0.15*math.Sin(t*1.5), 1+0.15*math.Cos(t*1.3))
		dc.SetRGBA(1, 0.55, 0.2, 0.95)
		dc.DrawRoundedRectangle(-70, -40, 140, 80, 12)
		_ = dc.Fill()
		dc.SetRGBA(0.2, 0.9, 1, 0.9)
		dc.SetLineWidth(3)
		dc.DrawRectangle(-50, -25, 100, 50)
		_ = dc.Stroke()
		dc.Pop()
		dc.Push()
		dc.Translate(fw*0.2, fh*0.25)
		dc.Rotate(-t * 1.2)
		dc.SetRGBA(0.6, 1, 0.4, 0.85)
		dc.DrawCircle(0, 0, 22)
		_ = dc.Fill()
		dc.Pop()
		return "中心旋转缩放方块 + 左上独立旋转圆（T.01/T.02）"

	case "path":
		cx, cy := fw*0.5, fh*0.5
		dc.SetRGBA(0.35, 0.75, 1, 0.9)
		dc.SetLineWidth(4)
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineJoin(render.LineJoinRound)
		dc.NewSubPath()
		dc.MoveTo(cx-100, cy)
		for i := 0; i <= 40; i++ {
			u := float64(i) / 40
			x := cx - 100 + u*200
			y := cy + 40*math.Sin(u*math.Pi*3+t*2)
			dc.LineTo(x, y)
		}
		_ = dc.Stroke()
		// Caps showcase
		dc.SetLineWidth(10)
		dc.SetLineCap(render.LineCapButt)
		dc.SetRGBA(1, 0.4, 0.3, 0.95)
		dc.DrawLine(fw*0.15, fh*0.75, fw*0.35, fh*0.75)
		_ = dc.Stroke()
		dc.SetLineCap(render.LineCapRound)
		dc.SetRGBA(0.4, 1, 0.45, 0.95)
		dc.DrawLine(fw*0.4, fh*0.75, fw*0.6, fh*0.75)
		_ = dc.Stroke()
		dc.SetLineCap(render.LineCapSquare)
		dc.SetRGBA(1, 0.85, 0.3, 0.95)
		dc.DrawLine(fw*0.65, fh*0.75, fw*0.85, fh*0.75)
		_ = dc.Stroke()
		// Fill star
		dc.SetRGBA(0.9, 0.5, 1, 0.75)
		dc.NewSubPath()
		n := 5
		for i := 0; i < n*2; i++ {
			a := float64(i)/float64(n*2)*2*math.Pi - math.Pi/2 + t*0.3
			r := 50.0
			if i%2 == 1 {
				r = 22
			}
			x, y := cx+120+math.Cos(a)*r, cy-20+math.Sin(a)*r
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		dc.ClosePath()
		_ = dc.Fill()
		return "波浪描边 + Cap 三线 + 旋转星形填充（H/G/P）"

	case "dash":
		dc.SetRGBA(0.4, 0.9, 1, 0.95)
		dc.SetLineWidth(0) // hairline (P.04)
		dc.SetDash(8, 6)
		dc.SetDashOffset(t * 20)
		dc.NewSubPath()
		dc.MoveTo(fw*0.1, fh*0.3)
		dc.CubicTo(fw*0.3, fh*0.1, fw*0.7, fh*0.5, fw*0.9, fh*0.25)
		_ = dc.Stroke()
		dc.SetLineWidth(2.5)
		dc.SetDash(12, 8, 2, 8)
		dc.SetDashOffset(-t * 30)
		dc.SetRGBA(1, 0.6, 0.2, 0.95)
		dc.DrawCircle(fw*0.5, fh*0.6, 70+10*math.Sin(t*2))
		_ = dc.Stroke()
		dc.ClearDash()
		return "Hairline 虚线贝塞尔 + 脉动虚线圆（P.04/E.01）"

	case "clip":
		for i := 0; i < 12; i++ {
			r, g, b := hsv(float64(i)/12+t*0.02, 0.55, 0.85)
			dc.SetRGB(r, g, b)
			dc.DrawRectangle(float64(i)*fw/12, 0, fw/12+1, fh)
			_ = dc.Fill()
		}
		dc.Push()
		dc.ClipRoundRect(fw*0.2, fh*0.2, fw*0.6, fh*0.55, 28)
		dc.SetRGBA(0.05, 0.05, 0.08, 0.55)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		dc.SetRGBA(1, 1, 1, 0.95)
		dc.DrawCircle(fw*0.5+40*math.Sin(t), fh*0.48, 50)
		_ = dc.Fill()
		dc.Pop()
		dc.Push()
		dc.ClipRect(fw*0.72, fh*0.12, fw*0.22, fh*0.22)
		dc.SetRGBA(0, 0, 0, 0.5)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		dc.SetRGBA(1, 0.3, 0.4, 0.95)
		dc.DrawCircle(fw*0.83+15*math.Cos(t*2), fh*0.23, 30)
		_ = dc.Fill()
		dc.Pop()
		return "条纹底 + 圆角裁剪窗 + 矩形裁剪窗（C.01/C.02）"

	case "grad":
		// Bounded RT: full-window multi-stop gradients are ColorAt-heavy (CPU).
		// Skia pattern: paint shader on layer-sized RT, composite with DrawImage.
		// Recompute every 2 frames; DrawImage every frame keeps present smooth.
		tw, th := 240, 150
		if frame%3 != 0 && gGradRT.hasCached() {
			_ = gGradRT.presentCached(dc, fw*0.5-float64(tw)/2, fh*0.45-float64(th)/2, render.InterpBilinear)
			dc.SetRGBA(1, 0.7, 0.2, 0.9)
			dc.DrawCircle(fw*0.12+30*math.Sin(t*2), fh*0.85, 10)
			_ = dc.Fill()
			return "线性/径向/扫描渐变 + 图像 pattern（D.01–D.03/D.05）"
		}
		rt := gGradRT.ensure(tw, th)
		rt.Clear()
		rt.SetRGB(0.08, 0.09, 0.12)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		lin := render.NewLinearGradientBrush(12, 12, float64(tw)-12, 56).
			AddColorStop(0, render.RGBA{R: 1, G: 0.3, B: 0.2, A: 1}).
			AddColorStop(0.5, render.RGBA{R: 1, G: 0.9, B: 0.2, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.2, G: 0.8, B: 1, A: 1})
		rt.SetFillBrush(lin)
		rt.DrawRoundedRectangle(12, 12, float64(tw-24), 48, 10)
		_ = rt.Fill()
		rcx, rcy, rr := 90.0, 140.0, 52.0
		rad := render.NewRadialGradientBrush(rcx, rcy, 0, rr).
			AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
			AddColorStop(0.5, render.RGBA{R: 0.4, G: 0.7, B: 1, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.1, G: 0.1, B: 0.25, A: 1})
		rt.SetFillBrush(rad)
		rt.DrawCircle(rcx, rcy, rr)
		_ = rt.Fill()
		scx, scy := 250.0, 140.0
		sw := render.NewSweepGradientBrush(scx, scy, t*0.7).
			AddColorStop(0, render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1}).
			AddColorStop(0.33, render.RGBA{R: 0.2, G: 1, B: 0.4, A: 1}).
			AddColorStop(0.66, render.RGBA{R: 0.25, G: 0.45, B: 1, A: 1}).
			AddColorStop(1, render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1})
		rt.SetFillBrush(sw)
		rt.DrawCircle(scx, scy, 48)
		_ = rt.Fill()
		if tile := ensureChecker(); tile != nil {
			pat := rt.CreateImagePattern(tile, 0, 0, tile.Width(), tile.Height())
			if pat != nil {
				rt.SetFillPattern(pat)
				rt.DrawRoundedRectangle(12, float64(th)-44, float64(tw-24), 32, 6)
				_ = rt.Fill()
			}
		}
		rt.SetRGB(1, 1, 1)
		_ = gGradRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.45-float64(th)/2)
		dc.SetRGBA(1, 0.7, 0.2, 0.9)
		dc.DrawCircle(fw*0.12+30*math.Sin(t*2), fh*0.85, 10)
		_ = dc.Fill()
		return "线性/径向/扫描渐变 + 图像 pattern（D.01–D.03/D.05）"

	case "blend":
		// C07: small offscreen RT + ExportImageBuf→DrawImage (pixel-correct).
		// Solid base + 4 light checker cells only (cheap) + 3 blend shapes.
		// No FlushGPUWithView RT composite (was empty/wrong). No full-window
		// advanced blends (present-path was ~30fps). Bounded export wins.
		tw, th := 240, 150
		rt := gBlendRT.ensure(tw, th)
		rt.Clear()
		rt.SetRGB(0.25, 0.35, 0.55)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		// 2x2 checker accents so blend contrast is visible
		cw, ch := float64(tw)/2, float64(th)/2
		rt.SetRGBA(0.92, 0.92, 0.96, 0.35)
		rt.DrawRectangle(0, 0, cw, ch)
		_ = rt.Fill()
		rt.DrawRectangle(cw, ch, cw, ch)
		_ = rt.Fill()
		rt.SetBlendMode(render.BlendMultiply)
		rt.SetRGBA(1, 0.45, 0.1, 0.9)
		rt.DrawCircle(float64(tw)*0.35+12*math.Sin(t), float64(th)*0.5, 36)
		_ = rt.Fill()
		rt.SetBlendMode(render.BlendScreen)
		rt.SetRGBA(0.2, 0.55, 1, 0.85)
		rt.DrawCircle(float64(tw)*0.58+12*math.Cos(t), float64(th)*0.5, 36)
		_ = rt.Fill()
		rt.SetBlendMode(render.BlendOverlay)
		rt.SetRGBA(0.9, 0.9, 0.3, 0.7)
		rt.DrawRoundedRectangle(float64(tw)*0.36, float64(th)*0.34+7*math.Sin(t*1.5), 70, 44, 8)
		_ = rt.Fill()
		rt.SetBlendMode(render.BlendNormal)
		_ = gBlendRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2)
		dc.SetRGBA(1, 0.7, 0.2, 0.9)
		dc.DrawCircle(fw*0.1+18*math.Sin(t*2), fh*0.88, 9)
		_ = dc.Fill()
		return "浅格底上 Multiply橙 / Screen蓝 / Overlay黄 叠加圆（Export 合成，无隔帧闪）"

	case "layer":
		// Background circles on present surface
		dc.SetRGBA(0.3, 0.6, 1, 0.9)
		dc.DrawCircle(fw*0.4+60*math.Sin(t), fh*0.5, 70)
		_ = dc.Fill()
		dc.SetRGBA(1, 0.5, 0.2, 0.9)
		dc.DrawCircle(fw*0.6+60*math.Cos(t), fh*0.5, 70)
		_ = dc.Fill()
		// Translucent layer on offscreen RT then DrawImage (L.02/L.03)
		tw, th := 320, 180
		rt := gLayerRT.ensure(tw, th)
		rt.Clear()
		rt.SetRGBA(0, 0, 0, 0)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		rt.PushLayer(render.BlendNormal, 0.45)
		rt.SetRGBA(0.2, 1, 0.6, 0.95)
		rt.DrawRoundedRectangle(16, 16, float64(tw-32), float64(th-32), 16)
		_ = rt.Fill()
		rt.SetRGBA(1, 1, 1, 0.95)
		rt.SetLineWidth(2)
		rt.DrawRoundedRectangle(16, 16, float64(tw-32), float64(th-32), 16)
		_ = rt.Stroke()
		rt.PopLayer()
		_ = gLayerRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.5-float64(th)/2)
		return "双圆背景 + 半透明 PushLayer 卡片（L.02/L.03）"

	case "image":
		if img := ensureChecker(); img != nil {
			// DrawImage (I.01)
			dc.DrawImage(img, fw*0.1, fh*0.15)
			// DrawImageEx scale + opacity (I.02/I.05)
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: fw * 0.35, Y: fh * 0.2, DstWidth: 120, DstHeight: 120,
				Opacity: 0.9, Interpolation: render.InterpBilinear,
			})
			// rotated via CTM (I.06)
			dc.Push()
			dc.Translate(fw*0.7, fh*0.35)
			dc.Rotate(t * 0.6)
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: -40, Y: -40, DstWidth: 80, DstHeight: 80,
				Opacity: 1, Interpolation: render.InterpNearest,
			})
			dc.Pop()
		}
		// WritePixels (S.07)
		const pw, ph = 64, 48
		need := pw * ph * 4
		if len(pix) >= need {
			for y := 0; y < ph; y++ {
				for x := 0; x < pw; x++ {
					i := (y*pw + x) * 4
					on := ((x/8)+(y/8)+int(t*2))%2 == 0
					if on {
						pix[i+0], pix[i+1], pix[i+2], pix[i+3] = 40, 180, 255, 255
					} else {
						pix[i+0], pix[i+1], pix[i+2], pix[i+3] = 255, 140, 40, 255
					}
				}
			}
			px := int(fw) - pw - 16
			py := int(fh) - ph - 48
			if px < 0 {
				px = 0
			}
			if py < 0 {
				py = 0
			}
			dc.WritePixels(px, py, pw, ph, pix[:need])
		}
		return "棋盘贴图/缩放/旋转 + WritePixels（I.01/S.07）"

	case "text":
		ensureFontPack(dc, fonts, 22)
		dc.SetRGBA(0.95, 0.96, 1, 1)
		dc.DrawString("中英文混排 Text · CJK Fallback", fw*0.08, fh*0.28)
		dc.SetTextDecoration(render.TextDecorationUnderline)
		dc.SetRGBA(0.4, 0.9, 1, 1)
		dc.DrawString("Underline 装饰线 sample AaBb123", fw*0.08, fh*0.38)
		dc.SetTextDecoration(0)
		ensureFontPack(dc, fonts, 16)
		dc.SetRGBA(0.85, 0.88, 0.95, 0.95)
		dc.DrawString("对标 Skia drawString / typeface / decoration（X.01/X.02/X.06/X.08）", fw*0.08, fh*0.5)
		// Moving accent line for continuous present
		dc.SetRGBA(1, 0.55, 0.2, 0.9)
		dc.SetLineWidth(2)
		x0 := fw*0.08 + math.Mod(t*40, fw*0.6)
		dc.DrawLine(x0, fh*0.58, x0+80, fh*0.58)
		_ = dc.Stroke()
		return "中英文本 + 下划线装饰（X.*）"

	case "filter":
		// Three small offscreen filter tiles (F.01/F.02/F.04)
		drawFilterTiles(dc, fw, fh, t, frame)
		return "模糊 / 投影 / 灰度 滤镜瓦片（F.01/F.02/F.04）"

	case "mesh":
		// Triangle mesh is piecewise-linear: residual "锯齿" ≈ under-tessellation
		// on curvature. Use denser grid + single low-frequency wave (no multi-lobe
		// interference) + mild amp so each facet spans a tiny dy.
		const cols, rows = 72, 42
		nVert := (cols + 1) * (rows + 1)
		nIdx := cols * rows * 6
		if cap(gMeshPos) < nVert {
			gMeshPos = make([]render.Point, nVert)
			gMeshCol = make([]render.RGBA, nVert)
		} else {
			gMeshPos = gMeshPos[:nVert]
			gMeshCol = gMeshCol[:nVert]
		}
		if cap(gMeshIdx) < nIdx {
			gMeshIdx = make([]uint16, nIdx)
		} else {
			gMeshIdx = gMeshIdx[:nIdx]
		}
		// Build indices once (topology fixed).
		if len(gMeshIdx) == nIdx {
			// always rewrite — cheap vs draw
		}
		ox, oy := fw*0.06, fh*0.16
		spanW, spanH := fw*0.88, fh*0.62
		cw, ch := spanW/float64(cols), spanH/float64(rows)
		// Amplitude ≈ 1.1 cells vertically → local slope gentle after 72-wide samples.
		amp := ch * 1.15
		if amp < 5 {
			amp = 5
		}
		if amp > 12 {
			amp = 12
		}
		// One traveling sine across width (~0.9 cycle); phase by row keeps bands parallel.
		k := math.Pi * 0.9
		vi := 0
		for j := 0; j <= rows; j++ {
			fy := float64(j) / float64(rows)
			// Tiny row lag (not a second frequency lobe).
			rowPhase := fy * 0.55
			for i := 0; i <= cols; i++ {
				fx := float64(i) / float64(cols)
				// Pure low-freq: sin only (smooth C∞). No secondary ridge.
				wave := amp * math.Sin(t*0.95+fx*k+rowPhase)
				gMeshPos[vi] = render.Point{
					X: ox + float64(i)*cw,
					Y: oy + float64(j)*ch + wave,
				}
				// Smooth color field (no high-freq sin on G — avoids "faceted color").
				gMeshCol[vi] = render.RGBA{
					R: 0.10 + 0.90*fx,
					G: 0.28 + 0.62*fy,
					B: 0.98 - 0.60*fy,
					A: 0.97,
				}
				vi++
			}
		}
		ii := 0
		for j := 0; j < rows; j++ {
			for i := 0; i < cols; i++ {
				i0 := uint16(j*(cols+1) + i)
				i1 := i0 + 1
				i2 := i0 + uint16(cols+1)
				i3 := i2 + 1
				gMeshIdx[ii+0] = i0
				gMeshIdx[ii+1] = i1
				gMeshIdx[ii+2] = i2
				gMeshIdx[ii+3] = i1
				gMeshIdx[ii+4] = i3
				gMeshIdx[ii+5] = i2
				ii += 6
			}
		}
		dc.SetAntiAlias(true)
		dc.DrawMesh(render.Mesh{Positions: gMeshPos, Colors: gMeshCol, Indices: gMeshIdx})
		// Soft guide polylines on top/mid/bottom — same verts as mesh (exact surface).
		dc.SetRGBA(1, 1, 1, 0.28)
		dc.SetLineWidth(1.15)
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineJoin(render.LineJoinRound)
		dc.SetAntiAlias(true)
		for _, j := range []int{0, rows / 2, rows} {
			base := j * (cols + 1)
			dc.NewSubPath()
			dc.MoveTo(gMeshPos[base].X, gMeshPos[base].Y)
			for i := 1; i <= cols; i++ {
				dc.LineTo(gMeshPos[base+i].X, gMeshPos[base+i].Y)
			}
			_ = dc.Stroke()
		}
		return "高密度彩色网格平滑起伏（V.01/V.03）"

	case "evenodd":
		// EvenOdd ring with hole
		cx, cy := fw*0.35, fh*0.5
		dc.SetFillRule(render.FillRuleEvenOdd)
		dc.SetRGBA(0.3, 0.85, 1, 0.9)
		dc.DrawCircle(cx, cy, 90)
		dc.DrawCircle(cx, cy, 45)
		_ = dc.Fill()
		// NonZero solid comparison
		dc.SetFillRule(render.FillRuleNonZero)
		dc.SetRGBA(1, 0.55, 0.25, 0.9)
		dc.DrawCircle(fw*0.68, cy, 90)
		dc.DrawCircle(fw*0.68, cy, 45)
		_ = dc.Fill()
		// labels via simple strokes
		dc.SetRGBA(1, 1, 1, 0.7)
		dc.SetLineWidth(1)
		dc.DrawLine(cx-40, cy+110, cx+40, cy+110)
		_ = dc.Stroke()
		return "左 EvenOdd 空心环 / 右 NonZero 实心（H.03）"

	case "mask":
		// Content under mask on offscreen
		tw, th := 200, 200
		rt := gMaskRT.ensure(tw, th)
		rt.Clear()
		// colorful base
		for i := 0; i < 8; i++ {
			r, g, b := hsv(float64(i)/8+t*0.05, 0.7, 0.95)
			rt.SetRGB(r, g, b)
			rt.DrawRectangle(float64(i)*float64(tw)/8, 0, float64(tw)/8+1, float64(th))
			_ = rt.Fill()
		}
		rt.SetRGBA(1, 1, 1, 0.9)
		rt.DrawCircle(float64(tw)/2+20*math.Sin(t), float64(th)/2, 40)
		_ = rt.Fill()
		if m := ensureSoftMask(); m != nil {
			rt.PushMaskLayer(m)
			// Extra content only inside mask
			rt.SetRGBA(0.1, 0.1, 0.15, 0.35)
			rt.DrawRectangle(0, 0, float64(tw), float64(th))
			_ = rt.Fill()
			rt.SetRGBA(1, 0.9, 0.2, 0.95)
			rt.DrawRoundedRectangle(40, 70, 120, 50, 10)
			_ = rt.Fill()
			rt.PopLayer()
		}
		_ = gMaskRT.blitTo(dc, fw*0.5-100, fh*0.5-100)
		// Unmasked reference stripes on side
		for i := 0; i < 4; i++ {
			r, g, b := hsv(float64(i)/4, 0.5, 0.6)
			dc.SetRGB(r, g, b)
			dc.DrawRectangle(fw*0.08, fh*0.2+float64(i)*40, 40, 36)
			_ = dc.Fill()
		}
		return "圆形 alpha PushMaskLayer 蒙版内容（L.06）"

	case "backdrop":
		// Animated base on present surface
		for i := 0; i < 10; i++ {
			r, g, b := hsv(float64(i)/10+t*0.03, 0.65, 0.9)
			dc.SetRGB(r, g, b)
			dc.DrawCircle(fw*0.15+float64(i)*fw*0.08, fh*0.5+30*math.Sin(t+float64(i)), 36)
			_ = dc.Fill()
		}
		// Backdrop card on bounded RT: snapshot-style frosted panel
		tw, th := 360, 200
		rt := gBackdropRT.ensure(tw, th)
		rt.Clear()
		// Fake parent content stripes so backdrop sample is visible
		for i := 0; i < 12; i++ {
			r, g, b := hsv(float64(i)/12+t*0.04, 0.6, 0.85)
			rt.SetRGB(r, g, b)
			rt.DrawRectangle(float64(i)*float64(tw)/12, 0, float64(tw)/12+1, float64(th))
			_ = rt.Fill()
		}
		rt.PushBackdropLayer(render.BlendNormal, 0.75)
		rt.SetRGBA(0.95, 0.97, 1, 0.35)
		rt.DrawRoundedRectangle(20, 20, float64(tw-40), float64(th-40), 18)
		_ = rt.Fill()
		rt.SetRGBA(1, 1, 1, 0.9)
		rt.SetLineWidth(1.5)
		rt.DrawRoundedRectangle(20, 20, float64(tw-40), float64(th-40), 18)
		_ = rt.Stroke()
		rt.PopLayer()
		_ = gBackdropRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.5-float64(th)/2)
		return "动态底 + Backdrop 半透明卡片（L.05）"

	case "damage":
		// Static base once-ish: dark + guide rect; animated only in damage region
		// Caller still clears via present path; we paint full stage lightly then animate dirty zone.
		dc.SetRGB(0.08, 0.09, 0.12)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		// Static chrome
		dc.SetRGBA(0.25, 0.28, 0.35, 1)
		dc.DrawRoundedRectangle(fw*0.08, fh*0.12, fw*0.84, fh*0.7, 12)
		_ = dc.Fill()
		dc.SetRGBA(0.5, 0.55, 0.65, 0.9)
		dc.SetLineWidth(2)
		dc.DrawRoundedRectangle(fw*0.08, fh*0.12, fw*0.84, fh*0.7, 12)
		_ = dc.Stroke()
		// Dirty animated region (center card)
		dx, dy, dw, dh := fw*0.35, fh*0.35, fw*0.3, fh*0.25
		r, g, b := hsv(math.Mod(t*0.15, 1), 0.55, 0.85)
		dc.SetRGB(r, g, b)
		dc.DrawRoundedRectangle(dx, dy, dw, dh, 10)
		_ = dc.Fill()
		dc.SetRGBA(1, 1, 1, 0.95)
		dc.DrawCircle(dx+dw/2+20*math.Sin(t*3), dy+dh/2, 18)
		_ = dc.Fill()
		return "中心 damage 区动画，外侧静态（S.09）"

	case "advblend":
		drawAdvBlend(dc, fw, fh, t, frame)
		return "高级混合模式网格 SoftLight/Diff/…（B.03/B.04）"

	case "textlcd":
		drawTextLCD(dc, fonts, fw, fh, t, frame)
		return "LCD/GlyphMask/Aliased 文本对照（X.04/X.05）"

	case "rrect":
		// G.06 independent XY radii
		specs := []struct {
			rx, ry float64
			label  string
		}{
			{4, 4, "4/4"},
			{8, 24, "8/24"},
			{24, 8, "24/8"},
			{40, 20, "40/20"},
		}
		for i, s := range specs {
			x := fw*0.08 + float64(i)*(fw*0.22)
			y := fh*0.3 + 8*math.Sin(t+float64(i))
			w, h := fw*0.18, fh*0.32
			dc.SetRGBA(0.25+float64(i)*0.15, 0.65, 1-float64(i)*0.12, 0.92)
			dc.DrawRoundedRectangleXY(x, y, w, h, s.rx, s.ry)
			_ = dc.Fill()
			dc.SetRGBA(1, 1, 1, 0.75)
			dc.SetLineWidth(1.5)
			dc.DrawRoundedRectangleXY(x, y, w, h, s.rx, s.ry)
			_ = dc.Stroke()
		}
		return "独立 XY 圆角半径 rrect 家族（G.06）"

	case "porterduff":
		drawPorterDuffBoard(dc, fw, fh, t, frame)
		return "PorterDuff 板 Clear/Copy/Plus/DstOut/Xor/Modulate…（B.02）"
	case "clipdiff":
		drawClipPathDiff(dc, fw, fh, t, frame)
		return "path clip + Difference 镂空（C.03/C.06/C.04）"
	case "gradtile":
		drawGradientTileLocal(dc, fw, fh, t, frame)
		return "渐变 Repeat/Reflect + pattern 局部矩阵（D.04/D.06）"
	case "imageadv":
		drawImageAdvanced(dc, fw, fh, t, frame)
		return "mip/opacity/旋转/九宫格（I.04–I.07）"
	case "textadv":
		drawTextAdvanced(dc, fonts, fw, fh, t, frame)
		return "MultiFace 混排 / atlas 复用 / emoji 探针（X.03/X.09–X.11）"
	case "composite":
		// Multi-capability light composite (not full stress)
		// Grad base strip
		lin := render.NewLinearGradientBrush(0, 0, fw, 0).
			AddColorStop(0, render.RGBA{R: 0.12, G: 0.15, B: 0.28, A: 1}).
			AddColorStop(0.5, render.RGBA{R: 0.18, G: 0.22, B: 0.35, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.1, G: 0.12, B: 0.2, A: 1})
		dc.SetFillBrush(lin)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		// Transform block
		dc.Push()
		dc.Translate(fw*0.25, fh*0.4)
		dc.Rotate(t * 0.5)
		dc.SetRGBA(1, 0.5, 0.2, 0.9)
		dc.DrawRoundedRectangle(-50, -30, 100, 60, 10)
		_ = dc.Fill()
		dc.Pop()
		// Clip circle content
		dc.Push()
		dc.ClipRect(fw*0.45, fh*0.25, fw*0.25, fh*0.35)
		dc.SetRGBA(0.3, 0.8, 1, 0.85)
		dc.DrawCircle(fw*0.57+15*math.Sin(t), fh*0.42, 40)
		_ = dc.Fill()
		dc.Pop()
		// Image
		if img := ensureChecker(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: fw * 0.75, Y: fh * 0.28, DstWidth: 64, DstHeight: 64,
				Opacity: 0.95, Interpolation: render.InterpBilinear,
			})
		}
		// Small mesh
		positions := []render.Point{
			{X: fw * 0.12, Y: fh * 0.7}, {X: fw * 0.22, Y: fh * 0.62}, {X: fw * 0.32, Y: fh * 0.72},
			{X: fw * 0.14, Y: fh * 0.82}, {X: fw * 0.24, Y: fh * 0.78}, {X: fw * 0.34, Y: fh * 0.85},
		}
		colors := []render.RGBA{
			{R: 1, G: 0.3, B: 0.3, A: 0.9}, {R: 0.3, G: 1, B: 0.4, A: 0.9}, {R: 0.3, G: 0.5, B: 1, A: 0.9},
			{R: 1, G: 0.8, B: 0.2, A: 0.9}, {R: 0.8, G: 0.3, B: 1, A: 0.9}, {R: 0.2, G: 0.9, B: 0.9, A: 0.9},
		}
		dc.DrawMesh(render.Mesh{Positions: positions, Colors: colors})
		// Text
		ensureFontPack(dc, fonts, 16)
		dc.SetRGBA(0.95, 0.96, 1, 1)
		dc.DrawString("合成压测 Composite · 渐变/变换/裁剪/贴图/网格/文本", fw*0.08, fh*0.18)
		return "多能力同屏合成（S/T/P/G/C/D/L/I/X/V）"

	default:
		dc.SetRGB(0.5, 0.2, 0.2)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		return "未知场景 kind=" + kind
	}
}

func drawFilterTiles(dc *render.Context, fw, fh, t float64, frame int) {
	// Small RTs. On odd frames present last results (still GPU DrawImage).
	if frame%2 == 1 && gFilterBlur.hasCached() && gFilterShad.hasCached() && gFilterGray.hasCached() {
		_ = gFilterBlur.presentCached(dc, fw*0.12, fh*0.38, render.InterpBilinear)
		_ = gFilterShad.presentCached(dc, fw*0.42, fh*0.38, render.InterpBilinear)
		_ = gFilterGray.presentCached(dc, fw*0.72, fh*0.38, render.InterpBilinear)
		return
	}
	// Blur tile
	{
		tw, th := 112, 72
		rt := gFilterBlur.ensure(tw, th)
		rt.Clear()
		rt.SetRGB(0.12, 0.14, 0.2)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		rt.SetRGBA(0.3, 0.9, 1, 0.95)
		rt.DrawCircle(float64(tw)/2+8*math.Sin(t*2), float64(th)/2, 18)
		_ = rt.Fill()
		// Fixed modest radius (avoid large kernel spikes)
		rt.ApplyBlur(4)
		if img := gFilterBlur.publish(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: fw * 0.12, Y: fh * 0.38, Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}
	// Drop shadow tile
	{
		tw, th := 112, 72
		rt := gFilterShad.ensure(tw, th)
		rt.Clear()
		rt.SetRGB(0.14, 0.12, 0.1)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		rt.SetRGBA(1, 0.85, 0.3, 0.95)
		rt.DrawRoundedRectangle(24, 18, 64, 36, 6)
		_ = rt.Fill()
		rt.ApplyDropShadow(3, 4, 4, render.RGBA{R: 0, G: 0, B: 0, A: 0.6})
		if img := gFilterShad.publish(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: fw * 0.42, Y: fh * 0.38, Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}
	// Grayscale tile
	{
		tw, th := 112, 72
		rt := gFilterGray.ensure(tw, th)
		rt.Clear()
		for i := 0; i < 6; i++ {
			r, g, b := hsv(float64(i)/6+t*0.05, 0.8, 0.95)
			rt.SetRGB(r, g, b)
			rt.DrawRectangle(float64(i)*float64(tw)/6, 0, float64(tw)/6+1, float64(th))
			_ = rt.Fill()
		}
		rt.ApplyGrayscale()
		if img := gFilterGray.publish(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: fw * 0.72, Y: fh * 0.38, Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}
	_ = frame
}

func drawAdvBlend(dc *render.Context, fw, fh, t float64, frame int) {
	// Offscreen RT + ExportImageBuf→DrawImage (pixel-correct for advanced blends).
	// Keep RT modest: 7 separable modes still visible, export cost bounded.
	tw, th := 200, 120
	rt := gAdvBlendRT.ensure(tw, th)
	rt.Clear()
	// 2 band base (cheaper than 4 full-width fills, still shows blend contrast)
	rt.SetRGB(0.18, 0.22, 0.40)
	rt.DrawRectangle(0, 0, float64(tw), float64(th)/2)
	_ = rt.Fill()
	rt.SetRGB(0.45, 0.28, 0.20)
	rt.DrawRectangle(0, float64(th)/2, float64(tw), float64(th)/2)
	_ = rt.Fill()
	modes := []struct {
		mode render.BlendMode
		col  render.RGBA
	}{
		{render.BlendMultiply, render.RGBA{R: 1, G: 0.25, B: 0.1, A: 0.9}},
		{render.BlendScreen, render.RGBA{R: 0.15, G: 0.55, B: 1, A: 0.9}},
		{render.BlendOverlay, render.RGBA{R: 1, G: 0.85, B: 0.15, A: 0.85}},
		{render.BlendSoftLight, render.RGBA{R: 0.4, G: 0.8, B: 1, A: 0.85}},
		{render.BlendDifference, render.RGBA{R: 0.9, G: 0.9, B: 0.2, A: 0.8}},
		{render.BlendPlus, render.RGBA{R: 0.6, G: 0.3, B: 0.1, A: 0.7}},
		{render.BlendModulate, render.RGBA{R: 0.95, G: 0.75, B: 0.55, A: 0.95}},
	}
	cols := 4
	cellW := float64(tw) / float64(cols)
	cellH := float64(th) / 2
	for i, m := range modes {
		col := i % cols
		row := i / cols
		cx := cellW*(float64(col)+0.5) + 2.5*math.Sin(t+float64(i)*0.4)
		cy := cellH*(float64(row)+0.5) + 2.0*math.Cos(t*1.1+float64(i)*0.3)
		r := math.Min(cellW, cellH) * 0.30
		rt.SetBlendMode(m.mode)
		rt.SetRGBA(m.col.R, m.col.G, m.col.B, m.col.A)
		rt.DrawCircle(cx, cy, r)
		_ = rt.Fill()
	}
	rt.SetBlendMode(render.BlendNormal)
	_ = gAdvBlendRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.5-float64(th)/2)
	// Continuous motion marker outside RT (cheap present-path).
	dc.SetRGBA(1, 0.75, 0.25, 0.95)
	dc.DrawCircle(fw*0.1+16*math.Sin(t*2), fh*0.88, 8)
	_ = dc.Fill()
	_ = frame
}

func drawTextLCD(dc *render.Context, fonts fontPack, fw, fh, t float64, frame int) {
	tw, th := 420, 180
	// Text shaping is CPU-side; present retained bitmap on odd frames.
	if frame%2 == 1 && gTextLCDRT.hasCached() {
		_ = gTextLCDRT.presentCached(dc, fw*0.5-float64(tw)/2, fh*0.5-float64(th)/2, render.InterpNearest)
		return
	}
	rt := gTextLCDRT.ensure(tw, th)
	rt.Clear()
	rt.SetRGB(0.95, 0.95, 0.97) // light bg for LCD visibility
	rt.DrawRectangle(0, 0, float64(tw), float64(th))
	_ = rt.Fill()
	ensureFontPack(rt, fonts, 18)
	modes := []struct {
		mode render.TextMode
		lcd  render.LCDLayout
		name string
		y    float64
	}{
		{render.TextModeGlyphMask, render.LCDLayoutNone, "GlyphMask", 36},
		{render.TextModeGlyphMask, render.LCDLayoutRGB, "GlyphLCD-RGB", 72},
		{render.TextModeAuto, render.LCDLayoutNone, "Auto", 108},
		{render.TextModeAliased, render.LCDLayoutNone, "Aliased", 144},
	}
	for _, m := range modes {
		rt.SetTextMode(m.mode)
		rt.SetLCDLayout(m.lcd)
		rt.SetRGB(0.1, 0.12, 0.16)
		rt.DrawString(fmt.Sprintf("%s  LCD Aa Bb 123  中文混排 %.1f", m.name, t), 16, m.y)
	}
	rt.SetLCDLayout(render.LCDLayoutNone)
	rt.SetTextMode(render.TextModeAuto)
	rt.SetRGB(0.2, 0.25, 0.35)
	ensureFontPack(rt, fonts, 14)
	rt.DrawString("对标 Skia edging / subpixel / LCD layout（X.04/X.05）", 16, 190)
	_ = gTextLCDRT.blitToInterp(dc, fw*0.5-float64(tw)/2, fh*0.5-float64(th)/2, render.InterpNearest)
	_ = frame
}

func ensureNineSrc() *render.ImageBuf {
	if gNineSrc != nil {
		return gNineSrc
	}
	img, err := render.NewImageBuf(48, 48, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	// 9-patch style: dark borders, bright center cross.
	for y := 0; y < 48; y++ {
		for x := 0; x < 48; x++ {
			edge := x < 8 || x >= 40 || y < 8 || y >= 40
			if edge {
				_ = img.SetRGBA(x, y, 30, 90, 200, 255)
			} else if (x > 18 && x < 30) || (y > 18 && y < 30) {
				_ = img.SetRGBA(x, y, 255, 200, 60, 255)
			} else {
				_ = img.SetRGBA(x, y, 60, 180, 120, 255)
			}
		}
	}
	gNineSrc = img
	return gNineSrc
}

// drawPorterDuffBoard — C21 / B.02 fixed-function Porter-Duff grid.
func drawPorterDuffBoard(dc *render.Context, fw, fh, t float64, frame int) {
	tw, th := 420, 220
	if frame%2 == 1 && gPDBoardRT.hasCached() {
		_ = gPDBoardRT.presentCached(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2, render.InterpBilinear)
		dc.SetRGBA(1, 0.85, 0.2, 0.95)
		dc.DrawCircle(fw*0.1+20*math.Sin(t*2), fh*0.88, 9)
		_ = dc.Fill()
		return
	}
	rt := gPDBoardRT.ensure(tw, th)
	// banded destination
	for i := 0; i < 6; i++ {
		u := float64(i) / 5
		rt.SetRGB(0.15+0.1*u, 0.2+0.15*(1-u), 0.45+0.2*u)
		rt.DrawRectangle(0, float64(i)*float64(th)/6, float64(tw), float64(th)/6+1)
		_ = rt.Fill()
	}
	modes := []struct {
		mode render.BlendMode
		name string
		col  render.RGBA
	}{
		{render.BlendClear, "Clear", render.RGBA{R: 1, G: 1, B: 1, A: 1}},
		{render.BlendCopy, "Copy", render.RGBA{R: 1, G: 0.2, B: 0.2, A: 0.85}},
		{render.BlendPlus, "Plus", render.RGBA{R: 0.9, G: 0.7, B: 0.1, A: 0.7}},
		{render.BlendDestinationOut, "DstOut", render.RGBA{R: 1, G: 1, B: 1, A: 0.75}},
		{render.BlendSourceAtop, "SrcAtop", render.RGBA{R: 0.2, G: 1, B: 0.4, A: 0.85}},
		{render.BlendXor, "Xor", render.RGBA{R: 0.3, G: 0.6, B: 1, A: 0.9}},
		{render.BlendDestinationOver, "DstOver", render.RGBA{R: 1, G: 0.4, B: 0.8, A: 0.7}},
		{render.BlendSourceIn, "SrcIn", render.RGBA{R: 1, G: 0.9, B: 0.2, A: 0.9}},
		{render.BlendSourceOut, "SrcOut", render.RGBA{R: 0.4, G: 0.9, B: 1, A: 0.85}},
		{render.BlendDestinationIn, "DstIn", render.RGBA{R: 1, G: 0.5, B: 0.2, A: 0.6}},
		{render.BlendDestinationAtop, "DstAtop", render.RGBA{R: 0.7, G: 0.3, B: 1, A: 0.8}},
		{render.BlendModulate, "Modulate", render.RGBA{R: 0.95, G: 0.85, B: 0.6, A: 0.95}},
	}
	cols, rows := 4, 3
	cellW := float64(tw) / float64(cols)
	cellH := float64(th) / float64(rows)
	for i, m := range modes {
		col := i % cols
		row := i / cols
		cx := cellW*(float64(col)+0.5) + 4*math.Sin(t+float64(i)*0.35)
		cy := cellH*(float64(row)+0.5) + 3*math.Cos(t*1.1+float64(i)*0.25)
		r := math.Min(cellW, cellH) * 0.28
		// label bar under cell (normal blend)
		rt.SetBlendMode(render.BlendNormal)
		rt.SetRGBA(0, 0, 0, 0.35)
		rt.DrawRectangle(cellW*float64(col)+4, cellH*float64(row)+cellH-18, cellW-8, 14)
		_ = rt.Fill()
		// mode sample
		rt.SetBlendMode(m.mode)
		rt.SetRGBA(m.col.R, m.col.G, m.col.B, m.col.A)
		rt.DrawCircle(cx, cy, r)
		_ = rt.Fill()
	}
	rt.SetBlendMode(render.BlendNormal)
	_ = gPDBoardRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2)
	// live marker outside RT so present always moves
	dc.SetRGBA(1, 0.85, 0.2, 0.95)
	dc.DrawCircle(fw*0.1+20*math.Sin(t*2), fh*0.88, 9)
	_ = dc.Fill()
	_ = frame
}

func drawClipPathDiff(dc *render.Context, fw, fh, t float64, frame int) {
	// C22: draw on present path (no ExportImageBuf). Fixed clip geometry;
	// only content inside clips animates — smooth, no alternate-frame jump.
	// Matrix: C.03 path clip, C.06 Difference, C.04 stack via Push/Pop.
	tw, th := 380.0, 200.0
	ox := fw*0.5 - tw/2
	oy := fh*0.48 - th/2

	// cheap panel base (2 fills, not 10 stripes)
	dc.SetRGB(0.12, 0.16, 0.28)
	dc.DrawRectangle(ox, oy, tw, th)
	_ = dc.Fill()
	dc.SetRGB(0.18, 0.22, 0.34)
	dc.DrawRectangle(ox, oy, tw/2, th)
	_ = dc.Fill()

	// Left: fixed circle path clip (C.03)
	dc.Push()
	dc.DrawCircle(ox+100, oy+th*0.52, 68)
	dc.Clip()
	dc.SetRGB(0.15, 0.45, 0.75)
	dc.DrawRectangle(ox+20, oy+24, 170, 150)
	_ = dc.Fill()
	dc.SetRGBA(1, 0.55, 0.15, 0.95)
	dc.DrawCircle(ox+100+18*math.Sin(t*1.8), oy+th*0.52+12*math.Cos(t*1.5), 16)
	_ = dc.Fill()
	dc.SetRGBA(0.3, 1, 0.55, 0.9)
	dc.DrawRoundedRectangle(ox+58+10*math.Cos(t*1.2), oy+58, 55, 32, 8)
	_ = dc.Fill()
	dc.Pop()

	// Right: fixed rect clip + fixed Difference hole (C.06)
	dc.Push()
	dc.ClipRect(ox+210, oy+22, 150, 156)
	dc.DrawCircle(ox+285, oy+th*0.52, 34)
	dc.ClipPathOp(render.ClipOpDifference)
	// solid animated color (cheaper than multi-stop gradient each frame)
	u := 0.5 + 0.5*math.Sin(t*0.9)
	dc.SetRGB(0.85+0.1*u, 0.35+0.25*(1-u), 0.25+0.45*u)
	dc.DrawRectangle(ox+210, oy+22, 150, 156)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 0.92)
	dc.DrawCircle(ox+240+12*math.Cos(t*1.6), oy+52+6*math.Sin(t*1.3), 10)
	_ = dc.Fill()
	dc.Pop()

	// continuous marker
	dc.SetRGBA(1, 0.75, 0.25, 0.95)
	dc.DrawCircle(fw*0.1+18*math.Sin(t*2), fh*0.88, 9)
	_ = dc.Fill()
	_ = frame
	_ = gClipDiffRT
}

// drawGradientTileLocal — C23 D.04 extend + D.06 local matrix pattern.
func drawGradientTileLocal(dc *render.Context, fw, fh, t float64, frame int) {
	tw, th := 460, 200
	rt := gGradTileRT.ensure(tw, th)
	rt.Clear()
	rt.SetRGB(0.08, 0.09, 0.12)
	rt.DrawRectangle(0, 0, float64(tw), float64(th))
	_ = rt.Fill()

	// short linear gradient (stops span ~80px) with ExtendRepeat over wide rect
	rep := render.NewLinearGradientBrush(20, 24, 100, 24).
		AddColorStop(0, render.RGBA{R: 1, G: 0.25, B: 0.2, A: 1}).
		AddColorStop(0.5, render.RGBA{R: 1, G: 0.9, B: 0.2, A: 1}).
		AddColorStop(1, render.RGBA{R: 0.2, G: 0.7, B: 1, A: 1}).
		SetExtend(render.ExtendRepeat)
	rt.SetFillBrush(rep)
	rt.DrawRoundedRectangle(16, 16, 200, 70, 10)
	_ = rt.Fill()

	// ExtendReflect band
	ref := render.NewLinearGradientBrush(240, 24, 320, 24).
		AddColorStop(0, render.RGBA{R: 0.3, G: 1, B: 0.5, A: 1}).
		AddColorStop(1, render.RGBA{R: 0.4, G: 0.3, B: 1, A: 1}).
		SetExtend(render.ExtendReflect)
	rt.SetFillBrush(ref)
	rt.DrawRoundedRectangle(236, 16, 200, 70, 10)
	_ = rt.Fill()

	// pattern local matrix (rotate + scale)
	if tile := ensureChecker(); tile != nil {
		pat := rt.CreateImagePattern(tile, 0, 0, tile.Width(), tile.Height())
		if ip, ok := pat.(*render.ImagePattern); ok && ip != nil {
			sx := 0.55 + 0.15*math.Sin(t)
			sy := 0.55 + 0.15*math.Cos(t*0.9)
			m := render.Translate(80, 140).
				Multiply(render.Rotate(t * 0.4)).
				Multiply(render.Scale(sx, sy)).
				Multiply(render.Translate(-24, -24))
			ip.SetTransform(m)
			ip.SetOpacity(0.95)
			rt.SetFillPattern(ip)
			rt.DrawRoundedRectangle(16, 100, 420, 84, 12)
			_ = rt.Fill()
		}
	}
	_ = gGradTileRT.blitTo(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2)
	dc.SetRGBA(1, 0.6, 0.2, 0.9)
	dc.DrawCircle(fw*0.88, fh*0.85+8*math.Sin(t*2), 10)
	_ = dc.Fill()
	_ = frame
}

// drawImageAdvanced — C24 I.04–I.07
func drawImageAdvanced(dc *render.Context, fw, fh, t float64, frame int) {
	img := ensureChecker()
	if img == nil {
		return
	}
	// I.04 mip/small scale
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: fw*0.1 + 6*math.Sin(t), Y: fh * 0.28,
		DstWidth: 28, DstHeight: 28,
		Opacity: 1, Interpolation: render.InterpBilinear, UseMipmaps: true,
	})
	// slightly larger mip sample
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: fw * 0.2, Y: fh * 0.26,
		DstWidth: 48, DstHeight: 48,
		Opacity: 1, Interpolation: render.InterpBilinear, UseMipmaps: true,
	})
	// I.05 opacity
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: fw * 0.35, Y: fh * 0.25,
		DstWidth: 90, DstHeight: 90,
		Opacity: 0.45 + 0.35*math.Abs(math.Sin(t)), Interpolation: render.InterpBilinear,
	})
	// I.06 rotate (CTM)
	dc.Push()
	dc.Translate(fw*0.62, fh*0.38)
	dc.Rotate(t * 0.7)
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: -40, Y: -40, DstWidth: 80, DstHeight: 80,
		Opacity: 0.95, Interpolation: render.InterpBilinear,
	})
	dc.Pop()
	// I.07 nine-patch
	if nine := ensureNineSrc(); nine != nil {
		cx := 8 + int(4*math.Sin(t*0.5))
		cy := 8 + int(4*math.Cos(t*0.5))
		center := image.Rect(cx, cy, 48-cx, 48-cy)
		dw := 160 + 30*math.Sin(t*0.8)
		dh := 100 + 20*math.Cos(t*0.9)
		dc.DrawImageNine(nine, center, fw*0.12, fh*0.58, dw, dh)
	}
	// label chips
	dc.SetRGBA(0, 0, 0, 0.4)
	dc.DrawRoundedRectangle(fw*0.08, fh*0.16, 420, 26, 6)
	_ = dc.Fill()
	_ = frame
	_ = gImageAdvRT
}

// drawTextAdvanced — C25 shaping / multiface / atlas reuse / emoji probe.
func drawTextAdvanced(dc *render.Context, fonts fontPack, fw, fh, t float64, frame int) {
	tw, th := 520, 220
	// retain on odd frames to reduce shape cost while still presenting GPU image
	if frame%2 == 1 && gTextAdvRT.hasCached() {
		_ = gTextAdvRT.presentCached(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2, render.InterpNearest)
		dc.SetRGBA(1, 0.75, 0.2, 0.9)
		dc.DrawCircle(fw*0.1+16*math.Sin(t*2), fh*0.88, 8)
		_ = dc.Fill()
		return
	}
	rt := gTextAdvRT.ensure(tw, th)
	rt.Clear()
	rt.SetRGB(0.1, 0.11, 0.14)
	rt.DrawRectangle(0, 0, float64(tw), float64(th))
	_ = rt.Fill()

	// MultiFace mixed (X.03/X.06)
	ensureFontPack(rt, fonts, 20)
	rt.SetRGBA(0.95, 0.96, 1, 1)
	rt.DrawString(fmt.Sprintf("AaBb 中文混排 shaping  t=%.1f", t), 16, 36)
	// repeated same string → glyph atlas reuse (X.11)
	ensureFontPack(rt, fonts, 15)
	rt.SetRGBA(0.75, 0.85, 1, 1)
	for i := 0; i < 6; i++ {
		rt.DrawString("atlas reuse · 复用字形 Aa中", 16, 64+float64(i)*18)
	}
	// variation attempt (X.09) — best-effort; fall back to normal face
	if fonts.latin != "" {
		_ = rt.LoadFontFace(fonts.latin, 18)
	}
	rt.SetRGBA(1, 0.85, 0.4, 1)
	rt.DrawString("variable/weight probe (face load)", 16, 185)
	// emoji / color font probe (X.10) — may tofu without emoji font; present must stay stable
	ensureFontPack(rt, fonts, 22)
	rt.SetRGBA(1, 0.95, 0.9, 1)
	rt.DrawString("emoji probe: 😀🚀✨  (no color-font → tofu ok)", 16, 210)

	_ = gTextAdvRT.blitToInterp(dc, fw*0.5-float64(tw)/2, fh*0.48-float64(th)/2, render.InterpNearest)
	dc.SetRGBA(1, 0.75, 0.2, 0.9)
	dc.DrawCircle(fw*0.1+16*math.Sin(t*2), fh*0.88, 8)
	_ = dc.Fill()
}

// probeCapability: path-stats gate after draw (GPU-first).
func probeCapability(dc *render.Context, kind string) (bool, string) {
	if dc == nil {
		return false, "nil_context"
	}
	st := dc.RenderPathStats()
	if st.CPUFallbackOps > 0 {
		return false, "cpu_fb:" + st.LastCPUFallbackReason
	}
	if st.GPUOps <= 0 {
		return false, "gpu_ops=0"
	}
	return true, fmt.Sprintf("gpu_ops=%d kind=%s", st.GPUOps, kind)
}

// damageRect returns the dirty region for C16 PresentFrameDamage.
func damageRect(fw, fh float64) (x, y, w, h int) {
	dx := int(fw * 0.35)
	dy := int(fh * 0.35)
	dw := int(fw * 0.3)
	dh := int(fh * 0.25)
	if dw < 32 {
		dw = 32
	}
	if dh < 32 {
		dh = 32
	}
	return dx, dy, dw, dh
}
