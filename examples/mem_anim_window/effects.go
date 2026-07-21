//go:build linux && !nogpu

package main

import (
	"math"

	"github.com/energye/gpui/render"
)

// effectRT is a reusable CPU offscreen for continuous Skia-style effects.
//
// Why offscreen (not full-surface Apply* / PushLayer on the present Context):
//  1. ApplyBlur on 800×600 is ~20–30ms on iGPU → frame hitch = perceived flash.
//  2. Sparse one-frame Apply*/PushLayer changes pixels only that frame → flash.
//  3. PushLayer on the window Context composites into the CPU parent pixmap, while
//     PresentFrameFull only flushes the GPU command stream — layer pixels never
//     reliably re-enter the present path.
//
// Skia pattern: saveLayer / ImageFilter on a bounded layer RT, then composite.
// Here: small NewContext → real API → ExportImageBuf → DrawImage every frame.
//
// Under multi-module (S12 lite): recompute is interleaved across frames while
// DrawImage of the last result still runs every frame (retained layer pattern).
// Modules stay continuously visible; APIs still execute on a rolling schedule.
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

// publish exports pixels. retained=true keeps a stable GenerationID between
// recomputes so DrawImage hits the GPU image cache (Skia retained layer).
// retained=false marks ephemeral (gen=0) for single-module full-rate stress
// (S10) where content changes every frame and cache would thrash anyway.
func (e *effectRT) publish(retained bool) *render.ImageBuf {
	if e == nil || e.dc == nil {
		return nil
	}
	if !e.dc.ExportImageBuf(&e.img) {
		return nil
	}
	if !retained {
		e.img.MarkEphemeral()
	}
	return e.img
}

func (e *effectRT) last() *render.ImageBuf {
	if e == nil {
		return nil
	}
	return e.img
}

func (e *effectRT) close() {
	if e == nil {
		return
	}
	if e.dc != nil {
		_ = e.dc.Close()
		e.dc = nil
	}
	e.img = nil
	e.closed = true
}

// dropGPU drops per-context GPU session + cached ImageBuf GPU upload state
// without destroying the logical offscreen (recreated lazily via ensure).
// Must run on device-lost abandon/recover (1GB cards OOM if old sessions pin VRAM).
func (e *effectRT) dropGPU() {
	if e == nil {
		return
	}
	if e.dc != nil {
		e.dc.DropGPURenderContext()
	}
	e.img = nil
}

// closeAllEffectRTs frees every package-level effect offscreen (filter/layer/blend…).
func closeAllEffectRTs() {
	for _, e := range []*effectRT{
		&gFilterBlurRT, &gFilterShadowRT, &gFilterGrayRT,
		&gLayerRT, &gBackdropRT, &gBlendRT,
		&gAdvBlendRT, &gGradientRT,
	} {
		e.close()
	}
}

// shouldRecompute decides whether an effect RT should rebuild this frame.
//
//	no cache          → always rebuild
//	period > 1        → throttle to frame%period == slot (caller opted into cadence;
//	                    used by S15/S16 gradient/advblend so non-lite still stays ≥55fps)
//	period ≤ 1, !lite → every frame (S10 filter stress)
//	period ≤ 1, lite  → same as period=1 (always); lite callers pass period≥3
func shouldRecompute(hasCache bool, lite bool, frame, slot, period int) bool {
	if !hasCache {
		return true
	}
	if period < 1 {
		period = 1
	}
	if period > 1 {
		return frame%period == slot%period
	}
	if !lite {
		return true
	}
	return frame%period == slot%period
}

// Shared RTs (one process = one scenario).
var (
	gFilterBlurRT   effectRT
	gFilterShadowRT effectRT
	gFilterGrayRT   effectRT
	gLayerRT        effectRT
	gBackdropRT     effectRT
	gBlendRT        effectRT
)

func drawFilterPanel(dc *render.Context, fw, fh, t float64, lite bool, frame int) {
	// Fixed RT size: resizing mid-run looks like "rect jumped" and hitches.
	tw, th := 112, 64
	gap := 8.0
	baseX := fw * 0.62
	baseY := fh * 0.40
	// Under lite (S12): recompute one tile per frame; always DrawImage all tiles.
	period := 1
	if lite {
		period = 3
	}

	// Tile 0: Gaussian blur (F.01)
	{
		if shouldRecompute(gFilterBlurRT.last() != nil, lite, frame, 0, period) {
			rt := gFilterBlurRT.ensure(tw, th)
			rt.Clear()
			r, g, b := hsv(math.Mod(t*0.12, 1), 0.55, 0.45)
			rt.SetRGB(r, g, b)
			rt.DrawRectangle(0, 0, float64(tw), float64(th))
			_ = rt.Fill()
			cr, cg, cb := hsv(math.Mod(t*0.12+0.15, 1), 0.85, 1)
			rt.SetRGBA(cr, cg, cb, 0.95)
			cx := float64(tw)*0.5 + 6*math.Sin(t*1.7)
			cy := float64(th) * 0.5
			rt.DrawCircle(cx, cy, 14)
			_ = rt.Fill()
			rt.SetRGBA(1, 1, 1, 0.55)
			rt.DrawCircle(cx-10, cy-6, 7)
			_ = rt.Fill()
			radius := 1.2
			if lite {
				radius = 0.9
			}
			rt.ApplyBlur(radius)
			_ = gFilterBlurRT.publish(lite)
		}
		if img := gFilterBlurRT.last(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: baseX, Y: baseY, Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}

	// Tile 1: drop shadow (F.02)
	{
		if shouldRecompute(gFilterShadowRT.last() != nil, lite, frame, 1, period) {
			rt := gFilterShadowRT.ensure(tw, th)
			rt.Clear()
			rt.SetRGBA(0.12, 0.14, 0.22, 1)
			rt.DrawRectangle(0, 0, float64(tw), float64(th))
			_ = rt.Fill()
			rt.SetRGBA(0.95, 0.75, 0.25, 1)
			rt.DrawRectangle(18, 14, float64(tw)-36, float64(th)-28)
			_ = rt.Fill()
			rt.SetRGBA(0.3, 0.85, 1.0, 1)
			rt.DrawCircle(float64(tw)*0.72, float64(th)*0.5, 10+2*math.Sin(t))
			_ = rt.Fill()
			blur := 1.6
			if lite {
				blur = 1.1
			}
			rt.ApplyDropShadow(2.5, 3.0, blur, render.RGBA{R: 0, G: 0, B: 0, A: 0.55})
			_ = gFilterShadowRT.publish(lite)
		}
		if img := gFilterShadowRT.last(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: baseX, Y: baseY + float64(th) + gap, Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}

	// Tile 2: grayscale (F.04). Under lite skip third tile (S10 keeps all three).
	if !lite {
		if shouldRecompute(gFilterGrayRT.last() != nil, false, frame, 2, 1) {
			rt := gFilterGrayRT.ensure(tw, th)
			rt.Clear()
			for i := 0; i < 4; i++ {
				rr, gg, bb := hsv(math.Mod(t*0.08+float64(i)*0.2, 1), 0.9, 1)
				rt.SetRGB(rr, gg, bb)
				rt.DrawRectangle(float64(i)*float64(tw)/4, 0, float64(tw)/4+1, float64(th))
				_ = rt.Fill()
			}
			rt.SetRGBA(1, 1, 1, 0.85)
			rt.DrawCircle(float64(tw)*0.5, float64(th)*0.5, 12)
			_ = rt.Fill()
			rt.ApplyGrayscale()
			_ = gFilterGrayRT.publish(false)
		}
		if img := gFilterGrayRT.last(); img != nil {
			dc.DrawImageEx(img, render.DrawImageOptions{
				X: baseX, Y: baseY + 2*(float64(th)+gap), Opacity: 1, Interpolation: render.InterpBilinear,
			})
		}
	}
}

func drawLayerStack(dc *render.Context, fw, fh, t float64, frame int, lite bool) {
	tw, th := 150, 84
	x := fw * 0.36
	y := fh * 0.40
	alpha := 0.72 + 0.18*math.Sin(t*1.3)
	period := 1
	if lite {
		period = 3
	}
	if shouldRecompute(gLayerRT.last() != nil, lite, frame, 0, period) {
		rt := gLayerRT.ensure(tw, th)
		rt.Clear()
		rt.PushLayer(render.BlendNormal, alpha)
		rt.SetRGBA(0.18, 0.32, 0.55, 0.95)
		rt.DrawRectangle(4, 4, float64(tw)-8, float64(th)-8)
		_ = rt.Fill()
		rt.SetRGBA(0.95, 0.98, 1.0, 0.9)
		rt.DrawRectangle(14, 16, float64(tw)*0.55, 8)
		_ = rt.Fill()
		rt.SetRGBA(0.35, 0.9, 1.0, 1)
		rt.DrawCircle(float64(tw)-28, float64(th)*0.55, 9+2*math.Sin(t*2))
		_ = rt.Fill()
		rt.SetRGBA(1, 0.5, 0.2, 0.35)
		rt.DrawRectangle(12, float64(th)*0.55, float64(tw)*0.4, 18)
		_ = rt.Fill()
		rt.PopLayer()
		_ = gLayerRT.publish(lite)
	}
	if img := gLayerRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: x, Y: y, Opacity: 1, Interpolation: render.InterpBilinear,
		})
	}
}

func drawBackdropCard(dc *render.Context, fw, fh, t float64, lite bool, frame int) {
	// Fixed RT size — size must NOT jump when adaptive lite engages.
	tw, th := 180, 96
	orbs := 4
	if lite {
		orbs = 3
	}
	x := fw * 0.52
	y := fh * 0.58
	alpha := 0.72 + 0.12*math.Sin(t)
	period := 1
	if lite {
		period = 3
	}
	if shouldRecompute(gBackdropRT.last() != nil, lite, frame, 1, period) {
		rt := gBackdropRT.ensure(tw, th)
		r, g, b := hsv(math.Mod(0.55+t*0.04, 1), 0.45, 0.35)
		rt.SetRGB(r, g, b)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		for i := 0; i < orbs; i++ {
			rr, gg, bb := hsv(math.Mod(t*0.1+float64(i)*0.13, 1), 0.8, 1)
			rt.SetRGBA(rr, gg, bb, 0.75)
			step := float64(tw - 36)
			if orbs > 1 {
				step = float64(tw-36) / float64(orbs-1)
			}
			cx := 18 + float64(i)*step + 5*math.Sin(t+float64(i))
			rt.DrawCircle(cx, 26+float64(i%3)*14, 10)
			_ = rt.Fill()
		}
		rt.PushBackdropLayer(render.BlendNormal, alpha)
		rt.SetRGBA(0.10, 0.12, 0.18, 0.42)
		rt.DrawRectangle(10, 10, float64(tw)-20, float64(th)-20)
		_ = rt.Fill()
		rt.SetRGBA(1, 1, 1, 0.90)
		rt.DrawRectangle(22, 28, float64(tw)*0.42, 8)
		_ = rt.Fill()
		rt.SetRGBA(0.45, 0.85, 1.0, 0.95)
		rt.DrawCircle(float64(tw)-34, float64(th)*0.55, 11)
		_ = rt.Fill()
		rt.PopLayer()
		_ = gBackdropRT.publish(lite)
	}
	if img := gBackdropRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: x, Y: y, Opacity: 1, Interpolation: render.InterpBilinear,
		})
	}
}

func drawBlendShapes(dc *render.Context, fw, fh, t float64, lite bool, frame int) {
	// Plus (GPU fixed-function) on present path every frame — bright additive halo.
	r := 34 + 8*math.Sin(t)
	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0.55, 0.28, 0.08, 0.65)
	dc.DrawCircle(fw*0.72, fh*0.52, r)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	// Offscreen Multiply/Screen over an explicit checker GRID (not solid gray).
	// User-facing S11 name is 网格/混合/变换 — the panel must show a grid + blend.
	// Fixed RT size; recompute interleaved under lite, DrawImage every frame.
	tw, th := 168, 104
	period := 1
	if lite {
		period = 3
	}
	if shouldRecompute(gBlendRT.last() != nil, lite, frame, 0, period) {
		rt := gBlendRT.ensure(tw, th)
		// Checker grid base (high contrast so blend modes are obvious).
		cell := 14.0
		for y := 0.0; y < float64(th); y += cell {
			for x := 0.0; x < float64(tw); x += cell {
				ix, iy := int(x/cell), int(y/cell)
				if (ix+iy)%2 == 0 {
					rt.SetRGB(0.94, 0.94, 0.97)
				} else {
					rt.SetRGB(0.28, 0.30, 0.40)
				}
				rt.DrawRectangle(x, y, cell, cell)
				_ = rt.Fill()
			}
		}
		// Explicit grid lines (the "网格" inside the square).
		rt.SetRGBA(0.15, 0.95, 0.45, 0.95)
		rt.SetLineWidth(1.25)
		for x := 0.0; x <= float64(tw)+0.5; x += cell * 2 {
			rt.DrawLine(x, 0, x, float64(th))
			_ = rt.Stroke()
		}
		for y := 0.0; y <= float64(th)+0.5; y += cell * 2 {
			rt.DrawLine(0, y, float64(tw), y)
			_ = rt.Stroke()
		}
		// Multiply orange — darkens checker through the circle.
		rt.SetBlendMode(render.BlendMultiply)
		rt.SetRGBA(1.0, 0.22, 0.08, 0.92)
		rt.DrawCircle(58, 56, 34+4*math.Sin(t))
		_ = rt.Fill()
		// Screen cyan — brightens through the circle.
		rt.SetBlendMode(render.BlendScreen)
		rt.SetRGBA(0.08, 0.45, 1.0, 0.88)
		rt.DrawCircle(118, 56, 32+4*math.Cos(t*1.1))
		_ = rt.Fill()
		// Normal yellow core for an unambiguous third blob.
		rt.SetBlendMode(render.BlendNormal)
		rt.SetRGBA(1.0, 0.92, 0.15, 0.9)
		rt.DrawCircle(88, 52, 12+2*math.Sin(t*1.7))
		_ = rt.Fill()
		// Border so the panel reads as a card, not "missing content".
		rt.SetRGBA(1, 1, 1, 0.85)
		rt.SetLineWidth(2)
		rt.DrawRectangle(1, 1, float64(tw-2), float64(th-2))
		_ = rt.Stroke()
		_ = gBlendRT.publish(lite)
	}
	if img := gBlendRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: fw * 0.58, Y: fh * 0.42, Opacity: 1, Interpolation: render.InterpNearest,
		})
	}
}
