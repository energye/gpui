//go:build !nogpu

package render_test

// Phase A extreme composition probes D37+ — multi-axis stress, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D37: colorful scene × grayscale/invert filters × clip residual check.
func TestP1_Comp_D37_FilterGraphColorOpsClip(t *testing.T) {
	p1RequireGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 320, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.2, 0.2)
	dc.DrawRectangle(20, 20, 120, 80)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.8, 0.3)
	dc.DrawRectangle(160, 30, 120, 90)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.35, 0.95)
	dc.DrawCircle(120, 150, 45)
	_ = dc.Fill()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre-filter flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterGrayscale},
	)
	// Then invert only a clipped region by redrawing inverted panel? Full invert:
	// sample grayscale mid before invert isn't available; invert whole surface
	dc.ApplyInvert()

	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("D37 stats %s base=%d", stats.LogLine(), base)
	// After grayscale+invert, pure white bg becomes black-ish or dark
	r, g, b, _ := p1Sample(dc, 8, 8)
	// inverted white ≈ black
	if r > 80 && g > 80 && b > 80 {
		t.Fatalf("D37 expected inverted bg dark rgba=%d,%d,%d", r, g, b)
	}
	// Former red region after gray+invert should still be non-uniform vs bg
	r2, g2, b2, _ := p1Sample(dc, 60, 50)
	if absU8(r2, r)+absU8(g2, g)+absU8(b2, b) < 10 {
		t.Fatalf("D37 shape collapsed into bg rgba=%d,%d,%d bg=%d,%d,%d", r2, g2, b2, r, g, b)
	}
}

// D38: DrawImageEx (src rect, opacity, blend, scale) × nested clip × text.
func TestP1_Comp_D38_DrawImageExClipText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	atlas := compMakeImage(t, 64, 64, 0, 0, 0)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if x < 32 {
				_ = atlas.SetRGBA(x, y, 220, 50, 40, 255)
			} else {
				_ = atlas.SetRGBA(x, y, 40, 90, 220, 255)
			}
		}
	}

	dc.ClipRect(24, 24, 312, 192)
	srcLeft := image.Rect(0, 0, 32, 64)
	srcRight := image.Rect(32, 0, 64, 64)
	dc.DrawImageEx(atlas, render.DrawImageOptions{
		X: 40, Y: 40, DstWidth: 100, DstHeight: 100,
		SrcRect: &srcLeft, Opacity: 0.9, BlendMode: render.BlendNormal,
		Interpolation: render.InterpNearest,
	})
	dc.DrawImageEx(atlas, render.DrawImageOptions{
		X: 160, Y: 50, DstWidth: 120, DstHeight: 80,
		SrcRect: &srcRight, Opacity: 0.7, BlendMode: render.BlendMultiply,
		Interpolation: render.InterpBilinear,
	})
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawString("DrawImageEx×clip×text", 50, 180)
	dc.ResetClip()

	compMinGPU(t, dc, 3)
	r, g, b, _ := p1Sample(dc, 70, 70)
	if r < 80 {
		t.Fatalf("D38 left crop red missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if r2 < 200 {
		t.Fatalf("D38 outside clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D39: radial + sweep gradients × layer × clip panels.
func TestP1_Comp_D39_RadialSweepGradientPanels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 240
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.ClipRoundRect(20, 20, 170, 200, 16)
	rad := render.NewRadialGradientBrush(100, 120, 0, 90).
		AddColorStop(0, render.RGB(1, 1, 1)).
		AddColorStop(0.5, render.RGB(0.3, 0.6, 1)).
		AddColorStop(1, render.RGB(0.05, 0.1, 0.35))
	dc.SetFillBrush(rad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.ResetClip()

	dc.ClipRoundRect(210, 20, 170, 200, 16)
	sw := render.NewSweepGradientBrush(295, 120, 0).
		AddColorStop(0, render.RGB(0.9, 0.2, 0.2)).
		AddColorStop(0.33, render.RGB(0.2, 0.9, 0.3)).
		AddColorStop(0.66, render.RGB(0.2, 0.4, 0.95)).
		AddColorStop(1, render.RGB(0.9, 0.2, 0.2))
	dc.SetFillBrush(sw)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.5)
	dc.SetRGB(0, 0, 0)
	dc.DrawRoundedRectangle(80, 90, 240, 50, 8)
	_ = dc.Fill()
	dc.PopLayer()
	dc.ResetClip()

	compMinGPU(t, dc, 3)
	// Radial stop0 is white at the focus; sample mid-ring (not center, not black bar).
	r, g, b, _ := p1Sample(dc, 100, 60)
	p1NotNearWhite(t, "D39 radial mid-ring", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 295, 80)
	p1NotNearWhite(t, "D39 sweep", r2, g2, b2)
}

// D40: advanced blend stack Hue/Overlay/SoftLight × image × text.
func TestP1_Comp_D40_AdvancedBlendImageText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// Base photo-like bands
	dc.SetRGB(0.85, 0.35, 0.25)
	dc.DrawRectangle(0, 0, w, h/2)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.45, 0.85)
	dc.DrawRectangle(0, h/2, w, h/2)
	_ = dc.Fill()

	img := compMakeImage(t, 80, 80, 40, 200, 80)
	dc.DrawImage(img, 40, 40)

	dc.SetBlendMode(render.BlendOverlay)
	dc.SetRGBA(0.9, 0.9, 0.2, 1)
	dc.DrawCircle(100, 80, 40)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendSoftLight)
	dc.SetRGBA(0.2, 0.9, 0.9, 1)
	dc.DrawCircle(200, 120, 50)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendHue)
	dc.SetRGBA(0.8, 0.2, 0.8, 1)
	dc.DrawRectangle(60, 140, 180, 50)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	dc.SetRGB(1, 1, 1)
	dc.DrawString("overlay×soft×hue", 70, 170)

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "D40 overlay zone", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 200, 120)
	p1NotNearWhite(t, "D40 softlight zone", r2, g2, b2)
}

// D41: fill pattern + stroke pattern × transform × clip.
func TestP1_Comp_D41_FillStrokePatternTransform(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	fillTile := compMakeImage(t, 12, 12, 30, 120, 200)
	strokeTile := compMakeImage(t, 8, 8, 220, 80, 30)
	for y := 0; y < 12; y++ {
		for x := 0; x < 6; x++ {
			_ = fillTile.SetRGBA(x, y, 200, 60, 40, 255)
		}
	}

	dc.Push()
	dc.Translate(40, 30)
	dc.Rotate(0.15)
	dc.ClipRoundRect(0, 0, 180, 140, 12)
	dc.SetFillPattern(dc.CreateImagePattern(fillTile, 0, 0, 12, 12))
	dc.DrawRectangle(0, 0, 180, 140)
	_ = dc.Fill()
	dc.SetStrokePattern(dc.CreateImagePattern(strokeTile, 0, 0, 8, 8))
	dc.SetLineWidth(6)
	dc.DrawRoundedRectangle(10, 10, 160, 120, 10)
	_ = dc.Stroke()
	dc.Pop()

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 100, 90)
	p1NotNearWhite(t, "D41 patterned fill", r, g, b)
}

// D42: anisotropic blur + drop shadow filter graph on dense card stack.
func TestP1_Comp_D42_BlurXYShadowGraphDense(t *testing.T) {
	p1RequireGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 360, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	for i := 0; i < 4; i++ {
		x := 30 + float64(i)*20
		y := 30 + float64(i)*18
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, y, 220, 120, 10)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.45, 0.9)
		dc.DrawRectangle(x, y, 220, 28)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(fmt.Sprintf("stack card %d", i), x+12, y+20)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	dc.ApplyImageFilterGraph(
		render.ImageFilterNode{
			Kind:        render.ImageFilterDropShadow,
			OffsetX:     3,
			OffsetY:     5,
			ShadowBlur:  6,
			ShadowColor: render.RGBA{R: 0, G: 0, B: 0, A: 0.4},
		},
		render.ImageFilterNode{Kind: render.ImageFilterBlurXY, RadiusX: 1.2, RadiusY: 0.4},
	)
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	r, g, b, _ := p1Sample(dc, 120, 50)
	p1NotNearWhite(t, "D42 card header after filters", r, g, b)
}

// D43: InvertMask × fill × nested layer × text.
func TestP1_Comp_D43_InvertMaskLayerText(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 260, 180
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.55, 0.35)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 40, 30, 220, 150, 255)
	dc.SetMask(mask)
	dc.InvertMask() // now outside band is opaque

	dc.PushLayer(render.BlendNormal, 0.85)
	dc.SetRGB(0.9, 0.25, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("invert-mask", 70, 100)
	dc.PopLayer()
	dc.ClearMask()

	compMinGPU(t, dc, 2)
	// Corner should be red-ish (inverted mask opaque outside center)
	r, g, b, _ := p1Sample(dc, 10, 10)
	if r < 80 {
		t.Fatalf("D43 inverted outside expected red wash rgba=%d,%d,%d", r, g, b)
	}
	// Center band was masked off → green base remains more
	r2, g2, b2, _ := p1Sample(dc, 130, 90)
	if g2 < 40 {
		t.Fatalf("D43 center expected green base rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D44: multi offscreen GPU textures + opacity + multi-rect damage flush.
func TestP1_Comp_D44_ExternalOpacityDamageRects(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("external opacity × damage", 12, 26)

	var releases []func()
	defer func() {
		for _, r := range releases {
			if r != nil {
				r()
			}
		}
	}()

	for i := 0; i < 6; i++ {
		tile := render.NewContext(40, 40)
		view, rel := tile.CreateOffscreenTexture(40, 40)
		if rel == nil || view.IsNil() {
			tile.Close()
			t.Skip("offscreen unavailable")
		}
		releases = append(releases, func() { rel(); tile.Close() })
		tile.SetRGB(0.2+float64(i)*0.1, 0.4, 0.9-float64(i)*0.08)
		tile.DrawRoundedRectangle(0, 0, 40, 40, 6)
		_ = tile.Fill()
		if err := tile.FlushGPUWithView(view, 40, 40); err != nil {
			t.Fatalf("tile: %v", err)
		}
		x := 24 + float64(i%3)*130
		y := 60 + float64(i/3)*90
		op := float32(0.45 + float32(i)*0.08)
		dc.DrawGPUTextureWithOpacity(view, x, y, 40, 40, op)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("t%d", i), x, y+55)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// Dirty update two tiles
	dc.ClipRect(24, 60, 50, 50)
	dc.SetRGB(0.95, 0.3, 0.2)
	dc.DrawRectangle(24, 60, 50, 50)
	_ = dc.Fill()
	dc.ResetClip()
	dc.ClipRect(284, 150, 50, 50)
	dc.SetRGB(0.2, 0.85, 0.4)
	dc.DrawRectangle(284, 150, 50, 50)
	_ = dc.Fill()
	dc.ResetClip()

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel != nil && !view.IsNil() {
		defer rel()
		_ = dc.FlushGPUWithViewDamageRects(view, uint32(w), uint32(h), []image.Rectangle{
			image.Rect(24, 60, 74, 110),
			image.Rect(284, 150, 334, 200),
		})
	}
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	if dc.RenderPathStats().GPUOps <= base {
		t.Fatalf("D44 expected more GPUOps")
	}
	r, g, b, _ := p1Sample(dc, 40, 80)
	if r < 100 {
		t.Fatalf("D44 dirty A not red rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 300, 170)
	if g2 < 100 {
		t.Fatalf("D44 dirty B not green rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D45: WritePixels badges + retained panels + clip text.
func TestP1_Comp_D45_WritePixelsRetainedPanels(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 400, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	for i := 0; i < 3; i++ {
		x := 16 + float64(i)*128
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(x, 24, 116, 192, 10)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("panel %d", i+1), x+16, 50)
		for r := 0; r < 6; r++ {
			dc.SetRGB(0.85, 0.87, 0.9)
			dc.DrawRectangle(x+12, 70+float64(r)*22, 90, 12)
			_ = dc.Fill()
		}
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// WritePixels notification badges
	mkBadge := func(rr, gg, bb uint8) []byte {
		buf := make([]byte, 10*10*4)
		for i := 0; i < 100; i++ {
			buf[i*4+0], buf[i*4+1], buf[i*4+2], buf[i*4+3] = rr, gg, bb, 255
		}
		return buf
	}
	dc.WritePixels(110, 30, 10, 10, mkBadge(220, 40, 40))
	dc.WritePixels(238, 30, 10, 10, mkBadge(40, 160, 80))
	dc.WritePixels(366, 30, 10, 10, mkBadge(40, 90, 220))

	// Retained update panel 2 body only
	dc.ClipRect(144, 64, 116, 140)
	dc.SetRGB(0.88, 0.93, 1.0)
	dc.DrawRectangle(144, 64, 116, 140)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.35, 0.85)
	dc.DrawString("updated", 160, 120)
	dc.ResetClip()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 114, 34)
	if r < 150 {
		t.Fatalf("D45 red badge missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 180, 100)
	if b2 < 100 {
		t.Fatalf("D45 panel2 update missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D46: filter graph chain blur→shadow→matrix on multi-primitive scene.
func TestP1_Comp_D46_FilterGraphMultiNodeScene(t *testing.T) {
	p1RequireGPU(t)
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 300, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(40, 30, 140, 100, 12)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.4, 0.2)
	dc.DrawCircle(210, 100, 50)
	_ = dc.Fill()
	img := compMakeImage(t, 32, 32, 40, 180, 60)
	dc.DrawImage(img, 50, 140)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// slight contrast matrix
	mat := [20]float32{
		1.1, 0, 0, 0, 0.02,
		0, 1.05, 0, 0, 0,
		0, 0, 1.15, 0, 0,
		0, 0, 0, 1, 0,
	}
	dc.ApplyImageFilterGraph(
		render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.6},
		render.ImageFilterNode{Kind: render.ImageFilterColorMatrix, Matrix: mat},
	)
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	r, g, b, _ := p1Sample(dc, 90, 70)
	p1NotNearWhite(t, "D46 filtered rect", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 210, 100)
	p1NotNearWhite(t, "D46 filtered circle", r2, g2, b2)
}

// D47: kanban board density — columns × cards × badges × clips × layers.
func TestP1_Comp_D47_KanbanPrimitiveDensity(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("kanban composition", 16, 24)

	cols := []string{"Backlog", "Doing", "Review", "Done"}
	for c, name := range cols {
		x := 16 + float64(c)*135
		dc.ClipRect(x, 40, 128, h-56)
		dc.SetRGB(0.88, 0.89, 0.92)
		dc.DrawRoundedRectangle(x, 40, 124, h-60, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(name, x+12, 62)
		for i := 0; i < 5; i++ {
			y := 80 + float64(i)*50
			dc.PushLayer(render.BlendNormal, 0.98)
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x+8, y, 108, 42, 6)
			_ = dc.Fill()
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString(fmt.Sprintf("%s-%d", name[:1], i+1), x+16, y+18)
			// priority badge
			dc.SetRGB(0.9, 0.35+float64(c)*0.1, 0.25)
			dc.DrawCircle(x+100, y+14, 6)
			_ = dc.Fill()
			dc.PopLayer()
		}
		dc.ResetClip()
	}

	// drag ghost layer
	dc.PushLayer(render.BlendNormal, 0.55)
	dc.SetRGB(1, 1, 0.85)
	dc.DrawRoundedRectangle(180, 160, 108, 42, 6)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 40)
	// Cards are intentionally near-white; sample column chrome + priority badge.
	r, g, b, _ := p1Sample(dc, 30, 50)
	p1NotNearWhite(t, "D47 column chrome", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 116, 94)
	p1NotNearWhite(t, "D47 priority badge", r2, g2, b2)
}

// D48: nested scroll + sticky header + modal backdrop (app shell).
func TestP1_Comp_D48_NestedScrollStickyModal(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// App frame
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, 64, h)
	_ = dc.Fill()
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(64, 0, w-64, 44)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("shell · sticky × scroll × modal", 80, 28)

	// Outer scroll viewport
	dc.ClipRect(64, 44, w-64, h-44)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(64, 44, w-64, h-44)
	_ = dc.Fill()

	// Sticky section header simulated: draw content scrolled, then sticky on top
	dc.Push()
	dc.Translate(0, -30)
	for i := 0; i < 12; i++ {
		y := 60 + float64(i)*36
		dc.SetRGB(0.94+float64(i%2)*0.03, 0.95, 0.97)
		dc.DrawRectangle(64, y, w-64, 36)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("row item %02d with nested details", i), 80, y+22)
	}
	dc.Pop()
	// sticky bar
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawRectangle(64, 44, w-64, 28)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("STICKY FILTERS", 80, 62)
	dc.ResetClip()

	// Modal
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre modal: %v", err)
	}
	base := dc.RenderPathStats().GPUOps
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.4)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(120, 80, 260, 180, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Confirm nested action", 150, 130)
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(150, 190, 90, 32, 6)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 250, 140)
	if r < 200 {
		t.Fatalf("D48 modal card missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 20, 100)
	if r2 > 60 {
		t.Fatalf("D48 rail should stay dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D49: HiDPI complex chrome — scale 2 app with hairline, text, images, clips.
func TestP1_Comp_D49_HiDPIAppChromeDensity(t *testing.T) {
	p1RequireGPU(t)
	// physical 640x400, logical 320x200
	dc := render.NewContext(640, 400, render.WithDeviceScale(2.0))
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	const lw, lh = 320.0, 200.0
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, lw, lh)
	_ = dc.Fill()

	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, lw, 28)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("HiDPI chrome", 8, 18)

	dc.ClipRect(8, 36, 140, 150)
	img := compMakeImage(t, 24, 24, 50, 130, 220)
	for i := 0; i < 8; i++ {
		y := 40 + float64(i)*18
		dc.DrawImage(img, 12, y)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("item %d", i), 42, y+14)
	}
	dc.ResetClip()

	dc.ClipRoundRect(160, 40, 148, 140, 10)
	grad := render.NewLinearGradientBrush(160, 40, 308, 40).
		AddColorStop(0, render.RGB(0.2, 0.4, 0.9)).
		AddColorStop(1, render.RGB(0.9, 0.3, 0.5))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(160, 40, 148, 140)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(0)
	dc.DrawLine(170, 50, 300, 160)
	_ = dc.Stroke()
	dc.ResetClip()

	compMinGPU(t, dc, 8)
	// physical samples (DPR=2): logical header band maps to y≈0..56
	dark := 0
	for x := 10; x < 200; x += 4 {
		r, g, b, _ := p1Sample(dc, x, 24)
		if r < 140 && g < 140 && b < 150 {
			dark++
		}
	}
	if dark < 5 {
		t.Fatalf("D49 header dark samples too low: %d", dark)
	}
	r2, g2, b2, _ := p1Sample(dc, 400, 120) // gradient panel phys
	p1NotNearWhite(t, "D49 gradient panel", r2, g2, b2)
	// list icon phys ~ logical(12+12,40+12)*2
	r3, g3, b3, _ := p1Sample(dc, 48, 100)
	p1NotNearWhite(t, "D49 list area", r3, g3, b3)
}

// D50: multi CTM stack rotate+scale+translate × mesh × text × path clip.
func TestP1_Comp_D50_MultiCTMMeshTextClip(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.12, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.Push()
	dc.Translate(180, 140)
	dc.Rotate(math.Pi / 7)
	dc.Scale(1.15, 0.95)
	dc.Translate(-80, -60)

	dc.DrawRoundedRectangle(0, 0, 160, 120, 12)
	dc.Clip()

	mesh := render.Mesh{
		Positions: []render.Point{{X: 10, Y: 10}, {X: 150, Y: 20}, {X: 140, Y: 110}, {X: 15, Y: 100}},
		Colors: []render.RGBA{
			{R: 1, G: 0.3, B: 0.2, A: 1}, {R: 0.3, G: 1, B: 0.4, A: 1},
			{R: 0.2, G: 0.5, B: 1, A: 1}, {R: 1, G: 0.9, B: 0.2, A: 1},
		},
		Indices: []uint16{0, 1, 2, 0, 2, 3},
	}
	dc.DrawMesh(mesh)
	dc.SetRGB(1, 1, 1)
	dc.DrawString("CTM×mesh×clip", 30, 70)
	dc.SetRGB(1, 1, 1)
	dc.SetLineWidth(2)
	dc.SetDash(5, 3)
	dc.DrawRectangle(8, 8, 144, 104)
	_ = dc.Stroke()
	dc.SetDash()
	dc.Pop()

	compMinGPU(t, dc, 3)
	r, g, b, _ := p1Sample(dc, 180, 140)
	p1NotNearWhite(t, "D50 center", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 50 {
		t.Fatalf("D50 bg should stay dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D51: spreadsheet density — grid lines × cells × selection layer × freeze clip.
func TestP1_Comp_D51_SpreadsheetGridComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 500, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// freeze panes: header row + col
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, 28)
	_ = dc.Fill()
	dc.DrawRectangle(0, 28, 48, h-28)
	_ = dc.Fill()
	for c := 0; c < 10; c++ {
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("%c", 'A'+c), 60+float64(c)*42, 18)
	}
	for r := 0; r < 12; r++ {
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("%d", r+1), 16, 48+float64(r)*22)
	}

	dc.ClipRect(48, 28, w-48, h-28)
	for r := 0; r < 12; r++ {
		for c := 0; c < 10; c++ {
			x := 48 + float64(c)*42
			y := 28 + float64(r)*22
			if (r+c)%2 == 0 {
				dc.SetRGB(0.97, 0.98, 0.99)
				dc.DrawRectangle(x, y, 42, 22)
				_ = dc.Fill()
			}
			dc.SetRGB(0.25, 0.27, 0.32)
			dc.DrawString(fmt.Sprintf("%d", (r+1)*(c+1)), x+8, y+15)
		}
	}
	// selection
	dc.PushLayer(render.BlendNormal, 0.35)
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRectangle(48+42*2, 28+22*3, 42*3, 22*2)
	_ = dc.Fill()
	dc.PopLayer()
	// grid hairlines
	dc.SetRGB(0.8, 0.82, 0.85)
	dc.SetLineWidth(0)
	for c := 0; c <= 10; c++ {
		x := 48 + float64(c)*42
		dc.DrawLine(x, 28, x, h)
		_ = dc.Stroke()
	}
	for r := 0; r <= 12; r++ {
		y := 28 + float64(r)*22
		dc.DrawLine(48, y, w, y)
		_ = dc.Stroke()
	}
	dc.ResetClip()

	compMinGPU(t, dc, 50)
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "D51 cell area", r, g, b)
	// frozen header
	r2, g2, b2, _ := p1Sample(dc, 20, 14)
	if r2 < 180 {
		t.Fatalf("D51 freeze header expected light rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D52: media timeline — filmstrip quads + playhead + waveform path + backdrop scrubber.
func TestP1_Comp_D52_MediaTimelineComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("timeline composition", 12, 22)

	// filmstrip
	for i := 0; i < 8; i++ {
		img := compMakeImage(t, 20, 16, uint8(30+i*20), uint8(100+i*10), uint8(200-i*15))
		x := 20 + float64(i)*60
		dc.DrawImageQuad(img, [4]render.Point{
			{X: x, Y: 40}, {X: x + 50, Y: 42}, {X: x + 48, Y: 100}, {X: x - 2, Y: 98},
		})
	}

	// waveform
	wave := render.NewPath()
	wave.MoveTo(20, 180)
	for x := 20; x < w-20; x += 6 {
		amp := 20 + 15*math.Sin(float64(x)*0.08) + 8*math.Sin(float64(x)*0.21)
		wave.LineTo(float64(x), 180-amp)
	}
	for x := w - 20; x >= 20; x -= 6 {
		amp := 20 + 15*math.Sin(float64(x)*0.08) + 8*math.Sin(float64(x)*0.21)
		wave.LineTo(float64(x), 180+amp*0.4)
	}
	wave.Close()
	dc.SetRGBA(0.2, 0.7, 0.55, 0.7)
	dc.AppendPath(wave)
	_ = dc.Fill()

	// playhead
	dc.SetRGB(0.95, 0.35, 0.3)
	dc.SetLineWidth(2)
	dc.DrawLine(200, 36, 200, 250)
	_ = dc.Stroke()

	// scrubber backdrop chip
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps
	dc.PushBackdropLayer(render.BlendNormal, 0.9)
	dc.SetRGBA(0, 0, 0, 0.35)
	dc.DrawRoundedRectangle(160, 230, 200, 40, 8)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("00:12 / 03:45", 210, 255)
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 50, 70)
	p1NotNearWhite(t, "D52 frame", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 200, 180)
	p1NotNearWhite(t, "D52 waveform/playhead", r2, g2, b2)
}

// D53: form/wizard multi-step — sections × validation marks × clip × layer overlay.
func TestP1_Comp_D53_FormWizardComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 440, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// steps
	for i, label := range []string{"Account", "Profile", "Review"} {
		x := 40 + float64(i)*120
		if i == 1 {
			dc.SetRGB(0.2, 0.5, 0.95)
		} else {
			dc.SetRGB(0.75, 0.78, 0.82)
		}
		dc.DrawCircle(x, 36, 14)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(label, x-20, 64)
		if i < 2 {
			dc.SetRGB(0.8, 0.82, 0.85)
			dc.SetLineWidth(2)
			dc.DrawLine(x+16, 36, x+100, 36)
			_ = dc.Stroke()
		}
	}

	// form card
	dc.ClipRoundRect(40, 90, 360, 190, 12)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(40, 90, 360, 190)
	_ = dc.Fill()
	fields := []string{"Display name", "Email", "Timezone", "Bio"}
	for i, f := range fields {
		y := 110 + float64(i)*40
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawString(f, 56, y)
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRoundedRectangle(56, y+6, 300, 26, 4)
		_ = dc.Fill()
		if i == 1 {
			// error state
			dc.SetRGB(0.9, 0.25, 0.25)
			dc.SetLineWidth(1)
			dc.DrawRoundedRectangle(56.5, y+6.5, 299, 25, 4)
			_ = dc.Stroke()
			dc.DrawString("invalid email", 56, y+40)
		}
	}
	dc.ResetClip()

	// help popover layer
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRoundedRectangle(280, 70, 140, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("field help tip", 292, 110)
	dc.PopLayer()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 160, 36)
	if b < 80 {
		t.Fatalf("D53 active step missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 100, 140)
	p1NotNearWhite(t, "D53 form field", r2, g2, b2)
}

// D54: tree + breadcrumb + split view density.
func TestP1_Comp_D54_TreeSplitViewComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 500, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// breadcrumb
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawString("root / projects / gpui / render", 12, 22)

	// tree
	dc.ClipRect(0, 32, 180, h-32)
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(0, 32, 180, h-32)
	_ = dc.Fill()
	indent := []int{0, 1, 1, 2, 2, 1, 0, 1, 2, 2, 2, 1}
	for i, ind := range indent {
		y := 48 + float64(i)*22
		if i == 3 {
			dc.SetRGB(0.85, 0.91, 1.0)
			dc.DrawRectangle(0, y-6, 180, 22)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("%snode-%02d", strings.Repeat("  ", ind), i), 8+float64(ind)*10, y+8)
	}
	dc.ResetClip()

	// split preview
	dc.ClipRect(180, 32, w-180, h-32)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(180, 32, w-180, h-32)
	_ = dc.Fill()
	dc.SetRGB(0.75, 0.78, 0.85)
	for i := 0; i < 14; i++ {
		dc.DrawString(fmt.Sprintf("%3d  | preview line content %d", i+1, i), 196, 56+float64(i)*18)
	}
	// overlay find bar
	dc.PushLayer(render.BlendNormal, 0.92)
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawRoundedRectangle(200, 40, 240, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("Find in file…", 212, 58)
	dc.PopLayer()
	dc.ResetClip()

	compMinGPU(t, dc, 20)
	// selected tree row (i==3): y=48+3*22=114 → sample y=112
	r, g, b, _ := p1Sample(dc, 40, 112)
	if b < 100 {
		t.Fatalf("D54 selected tree row missing tint rgba=%d,%d,%d", r, g, b)
	}
	// text ink scan in tree
	ink := 0
	for y := 50; y < 200; y += 4 {
		for x := 10; x < 160; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 200 || gg < 200 || bb < 200 {
				ink++
			}
		}
	}
	if ink < 20 {
		t.Fatalf("D54 tree ink too low: %d", ink)
	}
	r2, g2, b2, _ := p1Sample(dc, 300, 100)
	if r2 > 80 {
		t.Fatalf("D54 preview should be dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D55: cascader multi-column panels with hover layer + clip.
func TestP1_Comp_D55_CascaderColumnsComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 420, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("region selector", 16, 24)

	for col := 0; col < 3; col++ {
		x := 16 + float64(col)*130
		dc.ClipRect(x, 40, 124, 200)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 40, 120, 196, 8)
		_ = dc.Fill()
		dc.SetRGB(0.85, 0.86, 0.88)
		dc.SetLineWidth(1)
		dc.DrawRoundedRectangle(x+0.5, 40.5, 119, 195, 8)
		_ = dc.Stroke()
		for i := 0; i < 8; i++ {
			y := 52 + float64(i)*22
			if col == 1 && i == 2 {
				dc.PushLayer(render.BlendNormal, 1)
				dc.SetRGB(0.9, 0.94, 1)
				dc.DrawRectangle(x+4, y-4, 112, 22)
				_ = dc.Fill()
				dc.PopLayer()
			}
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString(fmt.Sprintf("L%d-item-%d", col+1, i), x+12, y+10)
		}
		dc.ResetClip()
	}

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 180, 100)
	p1NotNearWhite(t, "D55 hover row", r, g, b)
}

// D56: notification stack + toast layers + badge mesh accents.
func TestP1_Comp_D56_NotificationStackComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 360, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// page content
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRoundedRectangle(20, 20+float64(i)*40, w-40, 32, 6)
		_ = dc.Fill()
	}

	// stacked toasts
	for i := 0; i < 4; i++ {
		y := 40 + float64(i)*48
		dc.PushLayer(render.BlendNormal, 0.95-float64(i)*0.05)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(80, y, 220, 40, 8)
		_ = dc.Fill()
		// accent
		cols := [][3]float64{{0.2, 0.7, 0.4}, {0.2, 0.5, 0.95}, {0.95, 0.6, 0.2}, {0.9, 0.3, 0.3}}
		c := cols[i]
		dc.SetRGB(c[0], c[1], c[2])
		dc.DrawRectangle(80, y, 6, 40)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("toast message #%d", i+1), 96, y+24)
		dc.PopLayer()
	}

	// bell badge via vertices
	dc.DrawVertices([]render.Point{
		{X: 320, Y: 24}, {X: 340, Y: 24}, {X: 330, Y: 44},
	}, []render.RGBA{
		{R: 0.9, G: 0.2, B: 0.2, A: 1}, {R: 0.9, G: 0.2, B: 0.2, A: 1}, {R: 0.9, G: 0.2, B: 0.2, A: 1},
	}, render.VertexModeTriangles)

	compMinGPU(t, dc, 10)
	// Toast faces are white; sample the colored accent strip on toast #0.
	r, g, b, _ := p1Sample(dc, 83, 60)
	p1NotNearWhite(t, "D56 toast accent", r, g, b)
}

// D57: dual scroll transfer lists with move button and selection.
func TestP1_Comp_D57_TransferDualListComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 480, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	drawList := func(x float64, selected int) {
		dc.ClipRect(x, 40, 170, 220)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 40, 166, 216, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString("items", x+12, 60)
		for i := 0; i < 10; i++ {
			y := 76 + float64(i)*18
			if i == selected {
				dc.SetRGB(0.88, 0.93, 1)
				dc.DrawRectangle(x+4, y-4, 158, 18)
				_ = dc.Fill()
			}
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString(fmt.Sprintf("entry-%02d", i), x+16, y+8)
		}
		dc.ResetClip()
	}
	drawList(24, 2)
	drawList(286, 5)

	// transfer buttons
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(210, 120, 50, 28, 4)
	_ = dc.Fill()
	dc.DrawRoundedRectangle(210, 160, 50, 28, 4)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString(">", 228, 138)
	dc.DrawString("<", 228, 178)

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 235, 130)
	if b < 80 {
		t.Fatalf("D57 transfer btn missing rgba=%d,%d,%d", r, g, b)
	}
}

// D58: color picker morph — hue sweep + SV square + alpha bar + preview.
func TestP1_Comp_D58_ColorPickerComposition(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 320, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("color picker", 16, 22)

	// SV square via mesh-ish strips
	for y := 0; y < 140; y += 4 {
		for x := 0; x < 140; x += 4 {
			s := float64(x) / 140
			v := 1 - float64(y)/140
			dc.SetRGB(v, v*(1-s*0.5), v*(1-s))
			dc.DrawRectangle(20+float64(x), 40+float64(y), 4, 4)
			_ = dc.Fill()
		}
	}
	// hue bar sweep gradient
	dc.ClipRect(180, 40, 24, 140)
	sw := render.NewSweepGradientBrush(192, 110, 0).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(0.16, render.RGB(1, 1, 0)).
		AddColorStop(0.33, render.RGB(0, 1, 0)).
		AddColorStop(0.5, render.RGB(0, 1, 1)).
		AddColorStop(0.66, render.RGB(0, 0, 1)).
		AddColorStop(0.83, render.RGB(1, 0, 1)).
		AddColorStop(1, render.RGB(1, 0, 0))
	// linear hue as multi-stop instead if sweep looks circular - use linear
	hg := render.NewLinearGradientBrush(180, 40, 180, 180).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(0.2, render.RGB(1, 1, 0)).
		AddColorStop(0.4, render.RGB(0, 1, 0)).
		AddColorStop(0.6, render.RGB(0, 1, 1)).
		AddColorStop(0.8, render.RGB(0, 0, 1)).
		AddColorStop(1, render.RGB(1, 0, 1))
	_ = sw
	dc.SetFillBrush(hg)
	dc.DrawRectangle(180, 40, 24, 140)
	_ = dc.Fill()
	dc.ResetClip()

	// alpha bar
	dc.SetRGB(0.85, 0.85, 0.85)
	dc.DrawRectangle(20, 200, 140, 16)
	_ = dc.Fill()
	dc.SetRGBA(0.2, 0.4, 0.9, 0.5)
	dc.DrawRectangle(20, 200, 140, 16)
	_ = dc.Fill()

	// preview
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawRoundedRectangle(220, 40, 70, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("#3377E6", 220, 130)

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 30, 45)
	p1NotNearWhite(t, "D58 SV", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 255, 70)
	if b2 < 80 {
		t.Fatalf("D58 preview missing rgba=%d,%d,%d", r2, g2, b2)
	}
}
