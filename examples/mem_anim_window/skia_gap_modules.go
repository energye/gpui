//go:build linux && !nogpu

package main

import (
	"fmt"
	"image"
	"math"

	"github.com/energye/gpui/render"
)

// Skia-gap modules for S15–S21. These exercise render APIs that S01–S14 only
// covered thinly or not at all in the continuous window soak path.

var (
	gAdvBlendRT   effectRT
	gGradientRT   effectRT
	gTextLCDRT    effectRT
	gScrollRT     effectRT
	gPatternTile  *render.ImageBuf
	gDamageBooted bool
)

func ensurePatternTile() *render.ImageBuf {
	if gPatternTile != nil {
		return gPatternTile
	}
	img, err := render.NewImageBuf(16, 16, render.FormatRGBA8)
	if err != nil || img == nil {
		return nil
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			on := ((x/4)+(y/4))%2 == 0
			if on {
				_ = img.SetRGBA(x, y, 70, 190, 255, 255)
			} else {
				_ = img.SetRGBA(x, y, 255, 140, 60, 255)
			}
		}
	}
	gPatternTile = img
	return gPatternTile
}

// drawGradientPattern: real Linear/Radial/Sweep brushes + ImagePattern.
// CPU brush ColorAt is expensive on large rects → bounded offscreen RT (Skia
// saveLayer style), then DrawImage every frame (same pattern as filter/layer).
func drawGradientPattern(dc *render.Context, fw, fh, t float64, lite bool, frame int) {
	tw, th := 200, 140
	// Gradients are CPU ColorAt-heavy: recompute every 4 frames; DrawImage every frame.
	period := 4
	if lite {
		period = 6
	}
	if shouldRecompute(gGradientRT.last() != nil, lite, frame, 0, period) {
		rt := gGradientRT.ensure(tw, th)
		rt.Clear()
		rt.SetRGBA(0.08, 0.09, 0.14, 1)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()

		// Linear multi-stop
		c0 := render.RGBA{R: 0.95, G: 0.25 + 0.2*math.Sin(t), B: 0.35, A: 1}
		c1 := render.RGBA{R: 0.2, G: 0.75, B: 0.95, A: 1}
		c2 := render.RGBA{R: 0.95, G: 0.85, B: 0.2, A: 1}
		lin := render.NewLinearGradientBrush(12, 12, float64(tw)-12, 12).
			AddColorStop(0, c0).AddColorStop(0.5, c1).AddColorStop(1, c2)
		rt.SetFillBrush(lin)
		rt.DrawRoundedRectangle(12, 12, float64(tw)-24, 40, 8)
		_ = rt.Fill()
		rt.SetRGB(1, 1, 1)

		// Radial
		rcx, rcy := 70.0, 120.0
		rr := 42.0 + 4*math.Sin(t*1.3)
		rad := render.NewRadialGradientBrush(rcx, rcy, 0, rr).
			AddColorStop(0, render.RGBA{R: 1, G: 1, B: 1, A: 1}).
			AddColorStop(0.55, render.RGBA{R: 0.3, G: 0.85, B: 1, A: 1}).
			AddColorStop(1, render.RGBA{R: 0.05, G: 0.08, B: 0.15, A: 1})
		rt.SetFillBrush(rad)
		rt.DrawCircle(rcx, rcy, rr)
		_ = rt.Fill()
		rt.SetRGB(1, 1, 1)

		// Sweep
		scx, scy := 200.0, 120.0
		sw := render.NewSweepGradientBrush(scx, scy, t*0.7).
			AddColorStop(0, render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1}).
			AddColorStop(0.33, render.RGBA{R: 0.2, G: 1, B: 0.4, A: 1}).
			AddColorStop(0.66, render.RGBA{R: 0.25, G: 0.45, B: 1, A: 1}).
			AddColorStop(1, render.RGBA{R: 1, G: 0.2, B: 0.3, A: 1})
		rt.SetFillBrush(sw)
		rt.DrawCircle(scx, scy, 40)
		_ = rt.Fill()
		rt.SetRGB(1, 1, 1)

		// Image pattern strip
		if tile := ensurePatternTile(); tile != nil {
			pat := rt.CreateImagePattern(tile, 0, 0, tile.Width(), tile.Height())
			if pat != nil {
				rt.SetFillPattern(pat)
				rt.DrawRoundedRectangle(12, float64(th)-48, float64(tw)-24, 32, 6)
				_ = rt.Fill()
				rt.SetRGB(1, 1, 1)
			}
		}
		rt.SetRGBA(1, 1, 1, 0.85)
		rt.SetLineWidth(2)
		rt.DrawRectangle(1, 1, float64(tw-2), float64(th-2))
		_ = rt.Stroke()
		_ = gGradientRT.publish(true) // retained: stable gen for GPU image cache
	}
	if img := gGradientRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: fw * 0.08, Y: fh * 0.28, Opacity: 1, Interpolation: render.InterpBilinear,
		})
	}
}

// drawAdvancedBlendPanel: broader blend-mode set on a retained offscreen RT.
func drawAdvancedBlendPanel(dc *render.Context, fw, fh, t float64, lite bool, frame int) {
	tw, th := 240, 128
	period := 3
	if lite {
		period = 6
	}
	modes := []struct {
		mode render.BlendMode
		name string
		col  render.RGBA
	}{
		{render.BlendMultiply, "Mul", render.RGBA{R: 1, G: 0.25, B: 0.1, A: 0.9}},
		{render.BlendScreen, "Scr", render.RGBA{R: 0.15, G: 0.55, B: 1, A: 0.9}},
		{render.BlendOverlay, "Ovl", render.RGBA{R: 1, G: 0.85, B: 0.15, A: 0.85}},
		{render.BlendDarken, "Drk", render.RGBA{R: 0.2, G: 0.9, B: 0.4, A: 0.85}},
		{render.BlendLighten, "Ltn", render.RGBA{R: 0.95, G: 0.4, B: 0.95, A: 0.85}},
		{render.BlendHardLight, "Hrd", render.RGBA{R: 1, G: 0.5, B: 0.2, A: 0.85}},
		{render.BlendSoftLight, "Sft", render.RGBA{R: 0.4, G: 0.8, B: 1, A: 0.85}},
		{render.BlendDifference, "Dif", render.RGBA{R: 0.9, G: 0.9, B: 0.2, A: 0.8}},
		{render.BlendExclusion, "Exc", render.RGBA{R: 0.3, G: 1, B: 0.7, A: 0.8}},
		{render.BlendColorDodge, "Ddg", render.RGBA{R: 1, G: 0.3, B: 0.5, A: 0.75}},
		{render.BlendColorBurn, "Brn", render.RGBA{R: 0.4, G: 0.3, B: 1, A: 0.75}},
		{render.BlendPlus, "Plus", render.RGBA{R: 0.6, G: 0.3, B: 0.1, A: 0.7}},
	}
	if shouldRecompute(gAdvBlendRT.last() != nil, lite, frame, 0, period) {
		rt := gAdvBlendRT.ensure(tw, th)
		// Coarse vertical bands (not 1px rows): same visual intent, far fewer draws.
		// Per-row Fill previously issued ~128 GPU ops every recompute and tanked S16 FPS.
		const bands = 16
		bandH := float64(th) / float64(bands)
		for y := 0; y < bands; y++ {
			u := float64(y) / float64(bands-1)
			rt.SetRGB(0.15+0.55*u, 0.18+0.25*(1-u), 0.35+0.4*u)
			rt.DrawRectangle(0, float64(y)*bandH, float64(tw), bandH+0.5)
			_ = rt.Fill()
		}
		// Vertical stripes
		for i := 0; i < 8; i++ {
			x := float64(i) * float64(tw) / 8
			if i%2 == 0 {
				rt.SetRGBA(0.95, 0.95, 0.98, 0.35)
			} else {
				rt.SetRGBA(0.05, 0.06, 0.1, 0.35)
			}
			rt.DrawRectangle(x, 0, float64(tw)/8, float64(th))
			_ = rt.Fill()
		}
		n := 10
		if lite {
			n = 6
		}
		if n > len(modes) {
			n = len(modes)
		}
		cols := 4
		cellW := float64(tw) / float64(cols)
		cellH := float64(th) / math.Ceil(float64(n)/float64(cols))
		for i := 0; i < n; i++ {
			m := modes[i]
			col := i % cols
			row := i / cols
			cx := cellW*(float64(col)+0.5) + 4*math.Sin(t+float64(i)*0.4)
			cy := cellH*(float64(row)+0.5) + 3*math.Cos(t*1.1+float64(i)*0.3)
			r := math.Min(cellW, cellH)*0.32 + 2*math.Sin(t+float64(i))
			rt.SetBlendMode(m.mode)
			rt.SetRGBA(m.col.R, m.col.G, m.col.B, m.col.A)
			rt.DrawCircle(cx, cy, r)
			_ = rt.Fill()
		}
		rt.SetBlendMode(render.BlendNormal)
		rt.SetRGBA(1, 1, 1, 0.9)
		rt.SetLineWidth(2)
		rt.DrawRectangle(1, 1, float64(tw-2), float64(th-2))
		_ = rt.Stroke()
		_ = gAdvBlendRT.publish(true)
	}
	if img := gAdvBlendRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: fw * 0.48, Y: fh * 0.30, Opacity: 1, Interpolation: render.InterpNearest,
		})
	}
	// Present-path Plus halo (GPU fixed-function) continuous.
	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0.5, 0.25, 0.05, 0.55)
	dc.DrawCircle(fw*0.55, fh*0.72, 28+6*math.Sin(t))
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
}

// drawRRectEvenOdd: ClipRoundRect + FillRuleEvenOdd + nested clip stack.
func drawRRectEvenOdd(dc *render.Context, fw, fh, t float64, lite bool) {
	ox, oy := fw*0.12, fh*0.22
	w, h := fw*0.36, fh*0.42
	if w < 200 {
		w = 200
	}
	if h < 160 {
		h = 160
	}
	// Outer plate
	dc.SetRGBA(0.1, 0.11, 0.16, 0.9)
	dc.DrawRoundedRectangle(ox-8, oy-8, w+16, h+16, 14)
	_ = dc.Fill()

	// ClipRoundRect window with animated content inside
	dc.Push()
	rad := 18 + 4*math.Sin(t)
	dc.ClipRoundRect(ox, oy, w, h, rad)
	// Content stripes (must be clipped to rrect)
	nStripes := 10
	if lite {
		nStripes = 5
	}
	for i := 0; i < nStripes; i++ {
		den := float64(nStripes - 1)
		if den < 1 {
			den = 1
		}
		u := float64(i) / den
		r, g, b := hsv(math.Mod(u+t*0.08, 1), 0.7, 0.95)
		dc.SetRGBA(r, g, b, 0.85)
		yy := oy + float64(i)*(h/10) + 6*math.Sin(t+float64(i)*0.5)
		dc.DrawRectangle(ox, yy, w, h/10+2)
		_ = dc.Fill()
	}
	// Nested rect clip strip
	dc.ClipRect(ox+w*0.15, oy+h*0.25, w*0.7, h*0.35)
	dc.SetRGBA(0.05, 0.05, 0.08, 0.55)
	dc.DrawRectangle(ox, oy, w, h)
	_ = dc.Fill()
	dc.SetRGBA(1, 1, 1, 0.9)
	dc.DrawCircle(ox+w*0.5, oy+h*0.42, 22+4*math.Sin(t*1.6))
	_ = dc.Fill()
	dc.ResetClip()
	dc.Pop()

	// Even-odd star (H.03-class) continuous
	cx, cy := ox+w*0.5, oy+h+70
	if cy > fh-40 {
		cy = fh - 50
	}
	dc.SetFillRule(render.FillRuleEvenOdd)
	dc.SetRGBA(0.95, 0.75, 0.2, 0.9)
	star(dc, cx, cy, 36+4*math.Sin(t), 16, 5, t*0.4)
	_ = dc.Fill()
	dc.SetFillRule(render.FillRuleNonZero)
	// Stroke outline with miter join stress
	dc.SetRGBA(1, 1, 1, 0.85)
	dc.SetLineWidth(2.5)
	dc.SetLineJoin(render.LineJoinMiter)
	dc.SetMiterLimit(4)
	dc.SetLineCap(render.LineCapSquare)
	star(dc, cx, cy, 36+4*math.Sin(t), 16, 5, t*0.4)
	_ = dc.Stroke()
	dc.SetLineJoin(render.LineJoinRound)
	dc.SetLineCap(render.LineCapRound)

	// Complex cubic ribbon under clip-none (path quality)
	if !lite {
		dc.SetRGBA(0.4, 0.9, 1, 0.7)
		dc.SetLineWidth(2)
		dc.SetDash(8, 5, 2, 5)
		dc.SetDashOffset(math.Floor(math.Mod(t*20, 20)))
		p := render.NewPath()
		p.MoveTo(ox, oy+h+20)
		p.CubicTo(ox+w*0.25, oy+h+60, ox+w*0.75, oy+h-10, ox+w, oy+h+30)
		p.QuadraticTo(ox+w*0.5, oy+h+50, ox, oy+h+20)
		dc.SetPath(p)
		_ = dc.Stroke()
		dc.ClearDash()
	}
}

func star(dc *render.Context, cx, cy, rOuter, rInner float64, points int, rot float64) {
	if points < 3 {
		points = 5
	}
	dc.NewSubPath()
	for i := 0; i < points*2; i++ {
		ang := rot + float64(i)*math.Pi/float64(points) - math.Pi/2
		r := rOuter
		if i%2 == 1 {
			r = rInner
		}
		x := cx + math.Cos(ang)*r
		y := cy + math.Sin(ang)*r
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	dc.ClosePath()
}

// drawTextLCDShape: LCD layout + multiple TextMode strips + wrap/underline.
// Text shaping is expensive → recompute on small retained RT (Skia saveLayer),
// DrawImage every frame. Still runs real SetLCDLayout/SetTextMode/DrawString
// on recompute frames; mixed GlyphMask+LCD batches exercise the pipeline fix.
func drawTextLCDShape(dc *render.Context, fonts fontPack, fw, fh, t float64, frame int, lite bool) {
	if !fonts.ok {
		return
	}
	tw, th := 380, 220
	period := 4
	if lite {
		period = 6
	}
	if shouldRecompute(gTextLCDRT.last() != nil, lite, frame, 0, period) {
		rt := gTextLCDRT.ensure(tw, th)
		rt.Clear()
		rt.SetRGBA(0.07, 0.08, 0.12, 1)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()

		modes := []struct {
			mode render.TextMode
			name string
			lcd  render.LCDLayout
		}{
			{render.TextModeGlyphMask, "GlyphMask", render.LCDLayoutNone},
			{render.TextModeGlyphMask, "GlyphLCD-RGB", render.LCDLayoutRGB},
			{render.TextModeAuto, "Auto", render.LCDLayoutNone},
			{render.TextModeAliased, "Aliased", render.LCDLayoutNone},
		}
		if lite {
			modes = modes[:3]
		}
		lineH := 34.0
		px, py := 10.0, 12.0
		pw := float64(tw) - 20
		// Prefer latin face for mode rows (cheap); one CJK sample below.
		face := fonts.latin
		if face == "" {
			face = fonts.sans
		}
		for i, m := range modes {
			y := py + float64(i)*lineH
			rt.SetLCDLayout(m.lcd)
			rt.SetTextMode(m.mode)
			rt.SetRGB(0.96, 0.97, 0.99)
			rt.DrawRectangle(px, y, pw*0.62, 26)
			_ = rt.Fill()
			rt.SetRGB(0.08, 0.1, 0.14)
			ensureFont(rt, face, 14)
			rt.DrawString(fmt.Sprintf("%s  LCD Aa Bb 123", m.name), px+6, y+18)
			rt.SetRGB(0.12, 0.14, 0.2)
			rt.DrawRectangle(px+pw*0.64, y, pw*0.34, 26)
			_ = rt.Fill()
			rt.SetRGB(0.95, 0.97, 1)
			rt.DrawString(m.name, px+pw*0.64+6, y+18)
		}
		rt.SetTextMode(render.TextModeAuto)
		rt.SetLCDLayout(render.LCDLayoutNone)

		// One CJK line + wrap (stable string) for shaping coverage
		ensureFont(rt, fonts.sans, 13)
		rt.SetRGB(0.85, 0.9, 0.98)
		body := "对标Skia文本：中英混排 shaping、换行、装饰线、TextMode/LCD。"
		wy := py + float64(len(modes))*lineH + 8
		rt.DrawStringWrapped(body, px, wy, 0, 0, pw, 1.2, render.AlignLeft)
		ensureFont(rt, fonts.sans, 15)
		rt.SetTextDecoration(render.TextDecorationUnderline)
		rt.SetRGB(0.4, 1, 0.7)
		rt.DrawString("下划线 Underline", px, wy+52)
		rt.SetTextDecoration(render.TextDecoration(0))
		ensureFont(rt, face, 11)
		rt.SetRGB(0.7, 0.75, 0.85)
		rt.DrawString("small 11px", px, wy+78)
		ensureFont(rt, face, 20)
		rt.SetRGB(1, 0.85, 0.35)
		rt.DrawString("Large 20px", px+110, wy+78)
		ensureFont(rt, fonts.sans, 13)
		rt.SetRGBA(1, 1, 1, 0.85)
		rt.SetLineWidth(2)
		rt.DrawRectangle(1, 1, float64(tw-2), float64(th-2))
		_ = rt.Stroke()
		_ = gTextLCDRT.publish(true) // retained gen for DrawImage cache
	}
	if img := gTextLCDRT.last(); img != nil {
		// slight bob so panel is visibly live without reshaping every frame
		bob := 2 * math.Sin(t)
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: fw * 0.08, Y: fh*0.20 + bob, Opacity: 1, Interpolation: render.InterpNearest,
		})
	}
	_ = frame
}

// drawScrollModalUI: list scroll + modal overlay via retained RT for row text cost.
func drawScrollModalUI(dc *render.Context, fonts fontPack, fw, fh, t float64, lite bool) {
	tw, th := 320, 280
	period := 2
	if lite {
		period = 3
	}
	// encode scroll phase into recompute so motion is visible
	if shouldRecompute(gScrollRT.last() != nil, lite, int(t*30), 0, period) || gScrollRT.last() == nil {
		rt := gScrollRT.ensure(tw, th)
		rt.Clear()
		// list panel
		rt.SetRGBA(0.09, 0.1, 0.14, 1)
		rt.DrawRoundedRectangle(0, 0, 170, float64(th), 8)
		_ = rt.Fill()
		rt.SetRGBA(0.2, 0.55, 1, 1)
		rt.DrawRectangle(0, 0, 170, 24)
		_ = rt.Fill()
		face := fonts.latin
		if face == "" {
			face = fonts.sans
		}
		if fonts.ok {
			ensureFont(rt, face, 12)
			rt.SetRGB(1, 1, 1)
			rt.DrawString("Scroll List", 8, 16)
		}
		rowH := 22.0
		nRows := 18
		if lite {
			nRows = 12
		}
		scroll := math.Mod(t*48, float64(nRows)*rowH*0.5)
		rt.Push()
		rt.ClipRect(2, 26, 166, float64(th)-30)
		for i := 0; i < nRows; i++ {
			y := 28 + float64(i)*rowH - scroll
			if y+rowH < 26 || y > float64(th) {
				continue
			}
			if i%2 == 0 {
				rt.SetRGBA(0.14, 0.16, 0.22, 1)
			} else {
				rt.SetRGBA(0.11, 0.12, 0.17, 1)
			}
			rt.DrawRectangle(4, y, 162, rowH-2)
			_ = rt.Fill()
			r, g, b := hsv(float64(i)*0.07, 0.7, 0.95)
			rt.SetRGBA(r, g, b, 1)
			rt.DrawRectangle(4, y, 3, rowH-2)
			_ = rt.Fill()
			if fonts.ok {
				ensureFont(rt, face, 11)
				rt.SetRGB(0.9, 0.92, 0.96)
				rt.DrawString(fmt.Sprintf("row %02d item-%d", i+1, i*3+1), 12, y+14)
			}
		}
		rt.ResetClip()
		rt.Pop()

		// modal dim over right side of RT only (left list stays readable)
		rt.SetRGBA(0, 0, 0, 0.4)
		rt.DrawRectangle(0, 0, float64(tw), float64(th))
		_ = rt.Fill()
		// modal card
		mx, my, mw, mh := 90.0, 50.0, 210.0, 150.0
		rt.SetRGBA(0, 0, 0, 0.3)
		rt.DrawRoundedRectangle(mx+3, my+4, mw, mh, 10)
		_ = rt.Fill()
		rt.SetRGBA(0.12, 0.14, 0.2, 1)
		rt.DrawRoundedRectangle(mx, my, mw, mh, 10)
		_ = rt.Fill()
		rt.SetRGBA(0.25, 0.65, 1, 1)
		rt.DrawRectangle(mx, my, mw, 28)
		_ = rt.Fill()
		if fonts.ok {
			ensureFont(rt, face, 13)
			rt.SetRGB(1, 1, 1)
			rt.DrawString("Modal Dialog", mx+10, my+18)
			ensureFont(rt, fonts.sans, 12)
			rt.SetRGB(0.85, 0.9, 0.96)
			rt.DrawStringWrapped("列表滚动+遮罩+圆角卡 控件层未实现", mx+10, my+40, 0, 0, mw-20, 1.2, render.AlignLeft)
		}
		rt.SetRGBA(0.2, 0.55, 1, 1)
		rt.DrawRoundedRectangle(mx+mw-140, my+mh-36, 58, 24, 5)
		_ = rt.Fill()
		rt.SetRGBA(0.25, 0.28, 0.35, 1)
		rt.DrawRoundedRectangle(mx+mw-72, my+mh-36, 58, 24, 5)
		_ = rt.Fill()
		if fonts.ok {
			ensureFont(rt, fonts.sans, 11)
			rt.SetRGB(1, 1, 1)
			rt.DrawString("确定", mx+mw-128, my+mh-20)
			rt.DrawString("取消", mx+mw-60, my+mh-20)
		}
		_ = gScrollRT.publish(true)
	}
	if img := gScrollRT.last(); img != nil {
		dc.DrawImageEx(img, render.DrawImageOptions{
			X: fw * 0.12, Y: fh * 0.18, Opacity: 1, Interpolation: render.InterpNearest,
		})
	}
}

// drawDamagePartialScene: retained UI chrome + only dirty scroll band each frame.
// bootstrap=true draws full chrome (first frames / resize).
func drawDamagePartialScene(dc *render.Context, fonts fontPack, fw, fh, t float64, frame int, bootstrap bool) {
	// Scroll band metrics (logical)
	bandX, bandY := int(fw*0.1), int(fh*0.28)
	bandW, bandH := int(fw*0.8), int(fh*0.36)
	if bandW < 200 {
		bandW = 200
	}
	if bandH < 120 {
		bandH = 120
	}

	if bootstrap || !gDamageBooted {
		dc.SetRGB(0.07, 0.08, 0.11)
		dc.DrawRectangle(0, 0, fw, fh)
		_ = dc.Fill()
		// Static chrome
		dc.SetRGBA(0.12, 0.14, 0.2, 0.98)
		dc.DrawRoundedRectangle(fw*0.08, fh*0.12, fw*0.84, fh*0.12, 8)
		_ = dc.Fill()
		if fonts.ok {
			ensureFont(dc, fonts.sans, 16)
			dc.SetRGB(0.9, 0.95, 1)
			dc.DrawString("S19 局部 Damage Present — 仅中间条带每帧重绘", fw*0.1, fh*0.19)
		}
		// Side panels static
		dc.SetRGBA(0.1, 0.12, 0.18, 0.95)
		dc.DrawRoundedRectangle(fw*0.08, fh*0.68, fw*0.35, fh*0.18, 8)
		_ = dc.Fill()
		dc.DrawRoundedRectangle(fw*0.48, fh*0.68, fw*0.44, fh*0.18, 8)
		_ = dc.Fill()
		if fonts.ok {
			ensureFont(dc, fonts.sans, 13)
			dc.SetRGB(0.7, 0.8, 0.9)
			dc.DrawString("静态左侧栏（不应每帧闪）", fw*0.1, fh*0.76)
			dc.DrawString("静态右侧栏", fw*0.5, fh*0.76)
		}
		dc.MarkFullRedraw()
		gDamageBooted = true
	}

	// Dirty band only
	dc.SetRGB(0.09, 0.1, 0.14)
	dc.DrawRectangle(float64(bandX), float64(bandY), float64(bandW), float64(bandH))
	_ = dc.Fill()
	// Moving content inside band
	dc.Push()
	dc.ClipRect(float64(bandX), float64(bandY), float64(bandW), float64(bandH))
	scroll := math.Mod(t*60, 200)
	for i := 0; i < 8; i++ {
		y := float64(bandY) + float64(i)*30 - scroll
		r, g, b := hsv(math.Mod(float64(i)*0.08+t*0.05, 1), 0.65, 0.95)
		dc.SetRGBA(r, g, b, 0.9)
		dc.DrawRoundedRectangle(float64(bandX)+8, y, float64(bandW)-16, 24, 4)
		_ = dc.Fill()
		if fonts.ok {
			face := fonts.latin
			if face == "" {
				face = fonts.sans
			}
			ensureFont(dc, face, 11)
			dc.SetRGB(0.05, 0.06, 0.08)
			dc.DrawString(fmt.Sprintf("dmg-row %02d", i), float64(bandX)+16, y+15)
		}
	}
	// cursor pip
	dc.SetRGBA(1, 1, 1, 0.9)
	dc.DrawCircle(float64(bandX+bandW)-24, float64(bandY)+float64(bandH)*0.5+10*math.Sin(t*3), 8)
	_ = dc.Fill()
	dc.ResetClip()
	dc.Pop()

	dc.Invalidate(image.Rect(bandX, bandY, bandX+bandW, bandY+bandH))
}

func resetDamageBootstrap() {
	gDamageBooted = false
}
