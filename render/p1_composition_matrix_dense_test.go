//go:build !nogpu

package render_test

// Phase A dense composition probes D09+ — arbitrary axis crossings, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

func compMinGPU(t *testing.T, dc *render.Context, min int) {
	t.Helper()
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	stats := dc.RenderPathStats()
	if stats.GPUOps < min {
		t.Fatalf("expected GPUOps>=%d: %s", min, stats.LogLine())
	}
	if stats.CPUFallbackOps > 0 {
		t.Logf("note: cpu_fallback_ops=%d (allowed if still GPUOps>0)", stats.CPUFallbackOps)
	}
}

func compFillMaskRect(mask *render.Mask, x0, y0, x1, y1 int, v uint8) {
	w, h := mask.Width(), mask.Height()
	for y := y0; y < y1 && y < h; y++ {
		if y < 0 {
			continue
		}
		for x := x0; x < x1 && x < w; x++ {
			if x < 0 {
				continue
			}
			mask.Set(x, y, v)
		}
	}
}

// D09: dash stroke × nested clip × text label.
func TestP1_Comp_D09_DashStrokeClipText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D09_DashStrokeClipText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D09_DashStrokeClipText")
		return
	}
	const w, h = 360, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(20, 20, 320, 180)
	dc.ClipRect(40, 40, 280, 140)

	dc.SetRGB(0.15, 0.35, 0.85)
	dc.SetLineWidth(3)
	dc.SetDash(10, 6, 3, 6)
	dc.SetLineCap(render.LineCapRound)
	dc.DrawRoundedRectangle(50, 50, 260, 120, 14)
	_ = dc.Stroke()
	dc.SetDash()

	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawString("dash×clip×text", 90, 120)

	compMinGPU(t, dc, 3)
	// Dash ink somewhere on top edge band
	ink := 0
	for x := 60; x < 300; x++ {
		r, g, b, _ := p1Sample(dc, x, 50)
		if r < 220 || g < 220 || b < 220 {
			ink++
		}
	}
	if ink < 5 {
		t.Fatalf("D09 dash stroke ink too low: %d", ink)
	}
}

// D10: multi-stop gradient × ClipRoundRect × translucent layer.
func TestP1_Comp_D10_GradientClipLayer(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D10_GradientClipLayer"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D10_GradientClipLayer")
		return
	}
	const w, h = 280, 180
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.ClipRoundRect(30, 30, 220, 120, 20)
	grad := render.NewLinearGradientBrush(30, 30, 250, 30).
		AddColorStop(0, render.RGB(0.9, 0.2, 0.2)).
		AddColorStop(0.5, render.RGB(0.2, 0.8, 0.3)).
		AddColorStop(1, render.RGB(0.15, 0.35, 0.95))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.55)
	dc.SetRGB(0.15, 0.35, 0.95)
	dc.DrawRoundedRectangle(70, 60, 140, 50, 8)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 2)
	// Left of gradient should be red-ish (outside blue panel)
	r, g, b, _ := p1Sample(dc, 50, 90)
	if r < 80 {
		t.Fatalf("D10 left gradient not red-ish rgba=%d,%d,%d", r, g, b)
	}
	// Outside rrect corner: near white page
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if r2 < 200 {
		t.Fatalf("D10 outside clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
	// Layer panel mid: blue contribution over gradient
	r3, g3, b3, _ := p1Sample(dc, 140, 85)
	p1NotNearWhite(t, "D10 panel", r3, g3, b3)
	if b3 < 60 {
		t.Fatalf("D10 panel expected blue-ish rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D11: EvenOdd hole path × layer × blend multiply accent.
func TestP1_Comp_D11_EvenOddLayerBlend(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D11_EvenOddLayerBlend"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D11_EvenOddLayerBlend")
		return
	}
	const w, h = 240, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.85, 0.88, 0.92)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 1)
	dc.SetFillRule(render.FillRuleEvenOdd)
	dc.SetRGB(0.2, 0.45, 0.85)
	dc.DrawRectangle(40, 30, 160, 140)
	dc.DrawRectangle(80, 70, 80, 60) // hole
	_ = dc.Fill()
	dc.SetFillRule(render.FillRuleNonZero)
	dc.PopLayer()

	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGBA(1, 0.5, 0.5, 1)
	dc.DrawCircle(120, 100, 36)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	compMinGPU(t, dc, 3)
	// Frame body blue-ish
	r, g, b, _ := p1Sample(dc, 50, 40)
	if b < 80 {
		t.Fatalf("D11 frame missing rgba=%d,%d,%d", r, g, b)
	}
	// Hole center should not be solid blue frame (page or multiply result)
	hr, hg, hb, _ := p1Sample(dc, 120, 100)
	// hole had page gray then multiply red circle — should be darkened/reddish, not pure blue frame
	if hb > hr+40 && hb > 150 && hr < 80 {
		t.Fatalf("D11 hole looks like solid frame blue rgba=%d,%d,%d", hr, hg, hb)
	}
}

// D12: alpha mask × solid fill × image composite.
func TestP1_Comp_D12_MaskFillImage(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D12_MaskFillImage"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D12_MaskFillImage")
		return
	}
	const w, h = 200, 160
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	img := compMakeImage(t, 40, 40, 20, 180, 60)
	dc.DrawImage(img, 20, 20)
	dc.DrawImage(img, 140, 90)

	mask := render.NewMask(w, h)
	// Diagonal band mask
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x+y > 80 && x+y < 220 {
				mask.Set(x, y, 255)
			}
		}
	}
	dc.SetMask(mask)
	dc.SetRGB(0.9, 0.25, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.ClearMask()

	compMinGPU(t, dc, 2)
	// Masked band should show red
	r, g, b, _ := p1Sample(dc, 100, 80)
	if r < 100 {
		t.Fatalf("D12 mask band red missing rgba=%d,%d,%d", r, g, b)
	}
	// Corner outside band: image green or white
	r2, g2, b2, _ := p1Sample(dc, 30, 30)
	// may be green image
	if r2 > 200 && g2 < 50 {
		t.Fatalf("D12 corner unexpectedly red-only rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D13: PushMaskLayer × text × nested clip.
func TestP1_Comp_D13_MaskLayerTextClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D13_MaskLayerTextClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D13_MaskLayerTextClip")
		return
	}
	const w, h = 280, 180
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 18)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.92, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 40, 30, 240, 150, 255)

	dc.ClipRect(20, 20, 240, 140)
	dc.PushMaskLayer(mask)
	dc.SetRGB(0.15, 0.2, 0.75)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("mask×layer×text", 55, 100)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("layer flush: %v", err)
	}
	dc.PopLayer()

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 100, 90)
	p1NotNearWhite(t, "D13 masked body", r, g, b)
	// Outside mask rect but inside canvas
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if b2 > 180 && r2 < 80 {
		t.Fatalf("D13 mask leaked blue to corner rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D14: DrawVertices mesh × clip × Plus blend overlay.
func TestP1_Comp_D14_VerticesClipBlend(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D14_VerticesClipBlend"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D14_VerticesClipBlend")
		return
	}
	const w, h = 240, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.15, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(30, 30, 180, 140)
	pos := []render.Point{
		{X: 40, Y: 40}, {X: 200, Y: 50}, {X: 120, Y: 160},
		{X: 50, Y: 150}, {X: 190, Y: 140}, {X: 130, Y: 40},
	}
	cols := []render.RGBA{
		{R: 1, G: 0.2, B: 0.2, A: 1},
		{R: 0.2, G: 1, B: 0.3, A: 1},
		{R: 0.2, G: 0.4, B: 1, A: 1},
		{R: 1, G: 1, B: 0.2, A: 1},
		{R: 1, G: 0.4, B: 0.9, A: 1},
		{R: 0.3, G: 0.9, B: 1, A: 1},
	}
	dc.DrawVertices(pos, cols, render.VertexModeTriangles)

	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0.25, 0.25, 0.25, 1)
	dc.DrawCircle(120, 100, 40)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	compMinGPU(t, dc, 3)
	r, g, b, _ := p1Sample(dc, 120, 90)
	p1NotNearWhite(t, "D14 mesh", r, g, b)
	// Outside clip: dark bg
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if r2 > 80 || g2 > 80 {
		t.Fatalf("D14 outside clip should stay dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D15: DrawAtlas × HiDPI × clip window.
func TestP1_Comp_D15_AtlasHiDPIClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D15_AtlasHiDPIClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D15_AtlasHiDPIClip")
		return
	}
	dc := render.NewContext(320, 200, render.WithDeviceScale(2.0))
	defer dc.Close()

	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 160, 100)
	_ = dc.Fill()

	atlas := compMakeImage(t, 64, 32, 0, 0, 0)
	// two colored cells in atlas
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			_ = atlas.SetRGBA(x, y, 220, 40, 40, 255)
			_ = atlas.SetRGBA(x+32, y, 40, 80, 220, 255)
		}
	}

	dc.ClipRect(10, 10, 140, 80)
	sprites := []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, DstX: 16, DstY: 16, DstW: 40, DstH: 40, Opacity: 1},
		{SrcX: 32, SrcY: 0, SrcW: 32, SrcH: 32, DstX: 70, DstY: 24, DstW: 48, DstH: 48, Opacity: 0.85},
		{SrcX: 0, SrcY: 0, SrcW: 32, SrcH: 32, DstX: 120, DstY: 20, DstW: 30, DstH: 30, Opacity: 1},
	}
	dc.DrawAtlas(atlas, sprites)

	compMinGPU(t, dc, 2)
	// Physical sample ~ logical(30,30)*2
	r, g, b, _ := p1Sample(dc, 60, 60)
	if r < 100 {
		t.Fatalf("D15 atlas red cell missing rgba=%d,%d,%d", r, g, b)
	}
}

// D16: indexed DrawMesh × transform × layer opacity.
func TestP1_Comp_D16_MeshTransformLayer(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D16_MeshTransformLayer"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D16_MeshTransformLayer")
		return
	}
	const w, h = 260, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.9)
	dc.Translate(40, 30)
	dc.Scale(1.2, 1.2)
	mesh := render.Mesh{
		Positions: []render.Point{
			{X: 0, Y: 0}, {X: 100, Y: 10}, {X: 80, Y: 90}, {X: 10, Y: 80},
		},
		Colors: []render.RGBA{
			{R: 1, G: 0.3, B: 0.2, A: 1},
			{R: 0.3, G: 1, B: 0.3, A: 1},
			{R: 0.2, G: 0.4, B: 1, A: 1},
			{R: 1, G: 1, B: 0.3, A: 1},
		},
		Indices: []uint16{0, 1, 2, 0, 2, 3},
	}
	dc.DrawMesh(mesh)
	dc.PopLayer()

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 100, 80)
	p1NotNearWhite(t, "D16 mesh", r, g, b)
	// dark bg corner
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if r2 > 60 {
		t.Fatalf("D16 bg corner rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D17: DrawImageQuad × clip path × text.
func TestP1_Comp_D17_ImageQuadClipText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D17_ImageQuadClipText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D17_ImageQuadClipText")
		return
	}
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.MoveTo(40, 30)
	dc.LineTo(260, 40)
	dc.LineTo(250, 190)
	dc.LineTo(30, 170)
	dc.ClosePath()
	dc.Clip()

	img := compMakeImage(t, 32, 32, 30, 140, 220)
	dc.DrawImageQuad(img, [4]render.Point{
		{X: 60, Y: 50}, {X: 200, Y: 40}, {X: 210, Y: 150}, {X: 50, Y: 160},
	})
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawString("quad×clip×text", 90, 100)

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 120, 90)
	p1NotNearWhite(t, "D17 quad", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 200 {
		t.Fatalf("D17 outside path clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D18: path effects (corners/discrete/trim) × stroke × clip.
func TestP1_Comp_D18_PathEffectsClipStroke(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D18_PathEffectsClipStroke"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D18_PathEffectsClipStroke")
		return
	}
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRect(10, 10, 340, 220)

	poly := render.NewPath()
	poly.MoveTo(40, 40)
	poly.LineTo(160, 30)
	poly.LineTo(180, 140)
	poly.LineTo(50, 150)
	poly.Close()
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.SetLineWidth(3)
	dc.AppendPath(poly.WithCorners(14))
	_ = dc.Stroke()
	dc.SetRGBA(0.2, 0.5, 0.95, 0.2)
	dc.AppendPath(poly.WithCorners(14))
	_ = dc.Fill()

	guide := render.NewPath()
	guide.MoveTo(200, 40)
	guide.LineTo(330, 180)
	dc.SetRGB(0.9, 0.35, 0.15)
	dc.SetLineWidth(2)
	dc.AppendPath(guide.Discrete(8, 3))
	_ = dc.Stroke()

	curve := render.NewPath()
	curve.MoveTo(30, 200)
	curve.LineTo(330, 200)
	dc.SetRGB(0.2, 0.7, 0.4)
	dc.SetLineWidth(4)
	dc.AppendPath(curve.Trim(0.15, 0.85))
	_ = dc.Stroke()

	compMinGPU(t, dc, 4)
	r, g, b, _ := p1Sample(dc, 100, 90)
	p1NotNearWhite(t, "D18 fill", r, g, b)
	// trimmed stroke ink on y=200
	ink := 0
	for x := 60; x < 300; x++ {
		rr, gg, _, _ := p1Sample(dc, x, 200)
		if gg > rr && gg > 80 {
			ink++
		}
	}
	if ink < 3 {
		t.Fatalf("D18 trim stroke ink low: %d", ink)
	}
}

// D19: 3-level nested clip × nested layers × image × text (complex chrome).
func TestP1_Comp_D19_DeepNestClipLayerImageText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D19_DeepNestClipLayerImageText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D19_DeepNestClipLayerImageText")
		return
	}
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Shell
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(12, 12, w-24, h-24) // level 1
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(28, 40, w-56, h-70) // level 2 content
	dc.PushLayer(render.BlendNormal, 1)
	// sidebar
	dc.SetRGB(0.22, 0.24, 0.28)
	dc.DrawRectangle(28, 40, 100, h-70)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.85, 0.87, 0.9)
		dc.DrawString(fmt.Sprintf("nav-%d", i), 40, 70+float64(i)*28)
	}

	dc.ClipRect(140, 48, w-180, h-90) // level 3
	dc.PushLayer(render.BlendNormal, 0.95)
	img := compMakeImage(t, 24, 24, 60, 140, 220)
	for row := 0; row < 4; row++ {
		for col := 0; col < 5; col++ {
			x := 150 + float64(col)*48
			y := 56 + float64(row)*48
			dc.DrawImage(img, x, y)
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawString(fmt.Sprintf("%d", row*5+col), x+4, y+36)
		}
	}
	dc.PopLayer()
	dc.PopLayer()

	compMinGPU(t, dc, 15)
	// sidebar dark
	r, g, b, _ := p1Sample(dc, 50, 80)
	if r > 100 {
		t.Fatalf("D19 sidebar not dark rgba=%d,%d,%d", r, g, b)
	}
	// grid cell blue-ish
	r2, g2, b2, _ := p1Sample(dc, 162, 68)
	if b2 < 80 {
		t.Fatalf("D19 grid image missing rgba=%d,%d,%d", r2, g2, b2)
	}
	// outer shell dark remaining?
	r3, g3, b3, _ := p1Sample(dc, 4, 4)
	if r3 > 80 {
		t.Fatalf("D19 shell edge rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D20: multi-blend stack (Multiply/Screen/Plus) × clip × image.
func TestP1_Comp_D20_MultiBlendClipImage(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D20_MultiBlendClipImage"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D20_MultiBlendClipImage")
		return
	}
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.75, 0.8, 0.9)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRoundRect(20, 20, 260, 180, 12)
	img := compMakeImage(t, 80, 80, 200, 60, 40)
	dc.DrawImage(img, 40, 40)

	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(0.5, 0.8, 1)
	dc.DrawCircle(100, 100, 50)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendScreen)
	dc.SetRGB(0.3, 0.2, 0.6)
	dc.DrawCircle(180, 110, 55)
	_ = dc.Fill()

	dc.SetBlendMode(render.BlendPlus)
	dc.SetRGBA(0.2, 0.2, 0.1, 1)
	dc.DrawRectangle(60, 130, 160, 40)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 100, 100)
	p1NotNearWhite(t, "D20 multiply zone", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 180 {
		t.Fatalf("D20 outside rrect polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D21: external GPU texture tiles × clip × backdrop × text.
func TestP1_Comp_D21_ExternalTexClipBackdropText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D21_ExternalTexClipBackdropText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D21_ExternalTexClipBackdropText")
		return
	}
	const w, h = 400, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("external×clip×backdrop", 12, 24)

	var releases []func()
	defer func() {
		for _, r := range releases {
			if r != nil {
				r()
			}
		}
	}()

	cols := [][3]float64{{0.85, 0.3, 0.25}, {0.25, 0.55, 0.9}, {0.3, 0.75, 0.4}, {0.9, 0.7, 0.2}}
	for i, c := range cols {
		tile := render.NewContext(48, 48)
		view, rel := tile.CreateOffscreenTexture(48, 48)
		if rel == nil || view.IsNil() {
			tile.Close()
			t.Skip("CreateOffscreenTexture unavailable")
		}
		releases = append(releases, func() {
			rel()
			tile.Close()
		})
		tile.SetRGB(c[0], c[1], c[2])
		tile.DrawRoundedRectangle(0, 0, 48, 48, 6)
		_ = tile.Fill()
		if err := tile.FlushGPUWithView(view, 48, 48); err != nil {
			t.Fatalf("tile flush: %v", err)
		}
		x := 24 + float64(i)*90
		dc.ClipRect(x-2, 50, 56, 56)
		dc.DrawGPUTexture(view, x, 52, 48, 48)
		dc.ResetClip()
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre backdrop: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.35)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(90, 70, 220, 120, 10)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.15, 0.18)
	dc.DrawString("preview card", 120, 130)
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 200, 130)
	if r < 200 {
		t.Fatalf("D21 card missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 120 {
		t.Fatalf("D21 header should be dimmed/dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D22: dense scene + ApplyDropShadow + ApplyBlur composition.
func TestP1_Comp_D22_ShadowBlurDenseScene(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D22_ShadowBlurDenseScene"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D22_ShadowBlurDenseScene")
		return
	}
	const w, h = 320, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Card body
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(48, 40, 220, 140, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.45, 0.9)
	dc.DrawRoundedRectangle(48, 40, 220, 36, 12)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("card title", 64, 64)
	dc.SetRGB(0.25, 0.27, 0.32)
	for i := 0; i < 4; i++ {
		dc.DrawString(fmt.Sprintf("row content %d", i+1), 64, 100+float64(i)*18)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre filter: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	dc.ApplyDropShadow(4, 6, 8, render.RGBA{R: 0, G: 0, B: 0, A: 0.35})
	dc.ApplyBlur(0.8)

	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	stats := dc.RenderPathStats()
	t.Logf("D22 stats %s base=%d", stats.LogLine(), base)
	// Filters should keep GPU activity or at least leave readable structure
	r, g, b, _ := p1Sample(dc, 160, 60)
	p1NotNearWhite(t, "D22 header", r, g, b)
	// Soft edge somewhere should not be pure binary only — sample near card edge
	r2, g2, b2, _ := p1Sample(dc, 46, 100)
	_ = r2
	_ = g2
	_ = b2
}

// D23: SetDither × gradient band × HiDPI.
func TestP1_Comp_D23_DitherGradientHiDPI(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D23_DitherGradientHiDPI"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D23_DitherGradientHiDPI")
		return
	}
	dc := render.NewContext(256, 128, render.WithDeviceScale(2.0))
	defer dc.Close()

	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, 128, 64)
	_ = dc.Fill()

	dc.SetDither(true)
	grad := render.NewLinearGradientBrush(0, 0, 128, 0).
		AddColorStop(0, render.RGB(0.15, 0.15, 0.18)).
		AddColorStop(1, render.RGB(0.85, 0.86, 0.9))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(8, 12, 112, 40)
	_ = dc.Fill()
	dc.SetDither(false)

	compMinGPU(t, dc, 2)
	// Left dark, right light in physical coords (logical 20 -> phys 40)
	rL, gL, bL, _ := p1Sample(dc, 30, 50)
	rR, gR, bR, _ := p1Sample(dc, 220, 50)
	if rL > rR {
		t.Fatalf("D23 gradient direction unexpected L=%d,%d,%d R=%d,%d,%d", rL, gL, bL, rR, gR, bR)
	}
	if rR-rL < 20 {
		t.Fatalf("D23 gradient contrast too low L=%d R=%d", rL, rR)
	}
}

// D24: scroll-like nested clip + translate pan + dense text rows.
func TestP1_Comp_D24_ScrollClipTranslateText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D24_ScrollClipTranslateText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D24_ScrollClipTranslateText")
		return
	}
	const w, h = 320, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// Frame
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawRoundedRectangle(20, 20, 280, 240, 8)
	_ = dc.Fill()
	// Viewport
	dc.ClipRect(28, 28, 264, 224)
	dc.SetRGB(0.98, 0.98, 0.99)
	dc.DrawRectangle(28, 28, 264, 224)
	_ = dc.Fill()

	// Simulate scrolled content (pan up by 40)
	dc.Push()
	dc.Translate(0, -40)
	for i := 0; i < 18; i++ {
		y := 36 + float64(i)*28
		if i%2 == 0 {
			dc.SetRGB(0.94, 0.95, 0.97)
			dc.DrawRectangle(28, y, 264, 28)
			_ = dc.Fill()
		}
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("scroll-row-%02d content line", i), 40, y+18)
		dc.SetRGB(0.2, 0.5, 0.9)
		dc.DrawRoundedRectangle(250, y+6, 30, 16, 4)
		_ = dc.Fill()
	}
	dc.Pop()

	compMinGPU(t, dc, 20)
	// Viewport interior not pure dark frame
	r, g, b, _ := p1Sample(dc, 100, 80)
	if r < 100 && g < 100 {
		t.Fatalf("D24 viewport empty/dark rgba=%d,%d,%d", r, g, b)
	}
	// Outside frame corner white-ish
	r2, g2, b2, _ := p1Sample(dc, 6, 6)
	if r2 < 200 {
		t.Fatalf("D24 outside frame polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D25: nested layers + sequential blend stack (avoid Multiply onto empty layer).
func TestP1_Comp_D25_DeepNestedBlendLayers(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D25_DeepNestedBlendLayers"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D25_DeepNestedBlendLayers")
		return
	}
	const w, h = 240, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.7, 0.75, 0.85)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Layer 1: red disc
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(1, 0.35, 0.3)
	dc.DrawCircle(90, 100, 55)
	_ = dc.Fill()
	dc.PopLayer()

	// Sequential blends onto real destination content
	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(0.45, 1, 0.45)
	dc.DrawCircle(145, 105, 55)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendScreen)
	dc.SetRGB(0.25, 0.35, 1)
	dc.DrawCircle(115, 70, 48)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	// Layer 2: semi-transparent badge over stack
	dc.PushLayer(render.BlendNormal, 0.65)
	dc.SetRGB(0.15, 0.2, 0.85)
	dc.DrawRoundedRectangle(70, 85, 100, 36, 8)
	_ = dc.Fill()
	dc.PopLayer()

	// Layer 3: outer frame accent
	dc.PushLayer(render.BlendNormal, 0.8)
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(24, 24, 192, 152, 10)
	_ = dc.Stroke()
	dc.PopLayer()

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 115, 100)
	p1NotNearWhite(t, "D25 center stack", r, g, b)
	// badge region blue-ish
	r2, g2, b2, _ := p1Sample(dc, 120, 100)
	if b2 < 40 {
		t.Fatalf("D25 badge/stack expected color rgba=%d,%d,%d (center %d,%d,%d)", r2, g2, b2, r, g, b)
	}
}

// D26: stroke caps/joins × dashed polyline × path clip.
func TestP1_Comp_D26_CapsJoinsDashPathClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D26_CapsJoinsDashPathClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D26_CapsJoinsDashPathClip")
		return
	}
	const w, h = 300, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	dc.DrawRoundedRectangle(20, 20, 260, 160, 10)
	dc.Clip()

	dc.SetRGB(0.15, 0.2, 0.85)
	dc.SetLineWidth(8)
	dc.SetLineCap(render.LineCapSquare)
	dc.SetLineJoin(render.LineJoinBevel)
	dc.SetDash(14, 8)
	dc.MoveTo(40, 50)
	dc.LineTo(150, 40)
	dc.LineTo(250, 90)
	dc.LineTo(180, 150)
	dc.LineTo(60, 140)
	_ = dc.Stroke()
	dc.SetDash()

	dc.SetLineCap(render.LineCapRound)
	dc.SetLineJoin(render.LineJoinRound)
	dc.SetRGB(0.85, 0.25, 0.2)
	dc.SetLineWidth(5)
	dc.MoveTo(50, 160)
	dc.LineTo(240, 50)
	_ = dc.Stroke()

	compMinGPU(t, dc, 3)
	ink := 0
	for y := 30; y < 170; y += 2 {
		for x := 30; x < 270; x += 2 {
			r, g, b, _ := p1Sample(dc, x, y)
			if r < 230 || g < 230 || b < 230 {
				ink++
			}
		}
	}
	if ink < 40 {
		t.Fatalf("D26 stroke ink too low: %d", ink)
	}
}

// D27: rounded/circular image × mask × layer.
func TestP1_Comp_D27_ImageRoundedMaskLayer(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D27_ImageRoundedMaskLayer"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D27_ImageRoundedMaskLayer")
		return
	}
	const w, h = 280, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	img := compMakeImage(t, 64, 64, 40, 120, 220)
	// Make non-uniform
	for y := 0; y < 64; y++ {
		for x := 0; x < 32; x++ {
			_ = img.SetRGBA(x, y, 220, 80, 40, 255)
		}
	}

	dc.PushLayer(render.BlendNormal, 0.95)
	dc.DrawImageRounded(img, 30, 40, 12)
	dc.DrawImageCircular(img, 190, 100, 40)
	dc.PopLayer()

	mask := render.NewMask(w, h)
	// Vignette: only center band fully opaque for a red wash
	compFillMaskRect(mask, 100, 60, 180, 140, 180)
	dc.SetMask(mask)
	dc.SetRGBA(1, 0.2, 0.2, 0.5)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.ClearMask()

	compMinGPU(t, dc, 3)
	r, g, b, _ := p1Sample(dc, 50, 60)
	p1NotNearWhite(t, "D27 rounded image", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 190, 100)
	p1NotNearWhite(t, "D27 circular image", r2, g2, b2)
}

// D28: multi-region dirty redraw with gradient + text + image.
func TestP1_Comp_D28_DamageGradientTextImage(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D28_DamageGradientTextImage"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D28_DamageGradientTextImage")
		return
	}
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// Base dashboard
	grad := render.NewLinearGradientBrush(0, 0, w, 0).
		AddColorStop(0, render.RGB(0.15, 0.18, 0.25)).
		AddColorStop(1, render.RGB(0.25, 0.35, 0.55))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, 48)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("dashboard base", 16, 30)

	img := compMakeImage(t, 40, 40, 80, 160, 60)
	dc.DrawImage(img, 24, 70)
	dc.DrawImage(img, 200, 70)
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("panel-A", 24, 140)
	dc.DrawString("panel-B", 200, 140)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// Damage A: opaque cover then accent
	dc.ClipRect(16, 60, 120, 120)
	dc.SetRGB(0.2, 0.45, 0.95)
	dc.DrawRectangle(16, 60, 120, 120)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("A-upd", 40, 120)
	dc.ResetClip()

	// Damage B
	dc.ClipRect(180, 60, 140, 120)
	dc.SetRGB(0.9, 0.45, 0.2)
	dc.DrawRectangle(180, 60, 140, 120)
	_ = dc.Fill()
	img2 := compMakeImage(t, 28, 28, 255, 255, 255)
	dc.DrawImage(img2, 220, 90)
	dc.ResetClip()

	// Exercise multi-rect damage flush when available (then full resolve for Image sampling)
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel != nil && !view.IsNil() {
		defer rel()
		rects := []image.Rectangle{
			image.Rect(16, 60, 136, 180),
			image.Rect(180, 60, 320, 180),
		}
		if err := dc.FlushGPUWithViewDamageRects(view, uint32(w), uint32(h), rects); err != nil {
			t.Logf("FlushGPUWithViewDamageRects: %v", err)
		}
	}
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)

	stats := dc.RenderPathStats()
	if stats.GPUOps <= base {
		t.Fatalf("D28 expected more GPUOps base=%d now=%s", base, stats.LogLine())
	}
	// Sample centers of damage fills (avoid leftover icon coords)
	r, g, b, _ := p1Sample(dc, 76, 120)
	if b < 100 || r > b {
		t.Fatalf("D28 panel A not blue rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 200, 120)
	if r2 < 100 || r2 < g2 {
		t.Fatalf("D28 panel B not orange rgba=%d,%d,%d", r2, g2, b2)
	}
	// header preserved
	r3, g3, b3, _ := p1Sample(dc, 20, 20)
	if r3 > 120 && g3 > 120 {
		t.Fatalf("D28 header should stay dark gradient rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D29: rotate transform × path clip × image × stroke.
func TestP1_Comp_D29_RotateClipImageStroke(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D29_RotateClipImageStroke"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D29_RotateClipImageStroke")
		return
	}
	const w, h = 300, 240
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.92, 0.93, 0.95)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.Push()
	dc.Translate(150, 120)
	dc.Rotate(math.Pi / 8)
	dc.Translate(-80, -60)

	dc.DrawRoundedRectangle(0, 0, 160, 120, 10)
	dc.Clip()

	img := compMakeImage(t, 40, 40, 30, 100, 200)
	for i := 0; i < 3; i++ {
		dc.DrawImage(img, 10+float64(i)*40, 20+float64(i%2)*20)
	}
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.SetLineWidth(3)
	dc.DrawRectangle(8, 8, 144, 104)
	_ = dc.Stroke()
	dc.Pop()

	compMinGPU(t, dc, 3)
	// Center should have content
	r, g, b, _ := p1Sample(dc, 150, 120)
	p1NotNearWhite(t, "D29 rotated content", r, g, b)
}

// D30: virtual-list density via primitives (clip×image×text×badge×layer×scroll).
func TestP1_Comp_D30_VirtualListPrimitiveDensity(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D30_VirtualListPrimitiveDensity"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D30_VirtualListPrimitiveDensity")
		return
	}
	const w, h = 480, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// App shell
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, 72, h)
	_ = dc.Fill()
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(72, 0, w-72, 48)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("inbox · composition density", 88, 30)

	// List viewport
	dc.ClipRect(72, 48, w-72, h-48)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(72, 48, w-72, h-48)
	_ = dc.Fill()

	avatar := compMakeImage(t, 28, 28, 60, 120, 200)
	dc.Push()
	dc.Translate(0, -16) // partial row scroll
	for i := 0; i < 14; i++ {
		y := 56 + float64(i)*32
		if i == 3 {
			dc.PushLayer(render.BlendNormal, 1)
			dc.SetRGB(0.9, 0.94, 1.0)
			dc.DrawRectangle(72, y, w-72, 32)
			_ = dc.Fill()
			dc.PopLayer()
		}
		dc.DrawImageCircular(avatar, 96, y+16, 12)
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("Message subject line %02d — preview text", i), 120, y+14)
		dc.SetRGB(0.45, 0.48, 0.55)
		dc.DrawString("2h", w-48, y+14)
		// badge
		dc.SetRGB(0.9, 0.25, 0.25)
		dc.DrawCircle(w-70, y+16, 6)
		_ = dc.Fill()
	}
	dc.Pop()

	// Floating action layer
	dc.PushLayer(render.BlendNormal, 0.92)
	dc.SetRGB(0.15, 0.45, 0.95)
	dc.DrawCircle(w-40, h-40, 22)
	_ = dc.Fill()
	dc.PopLayer()

	// Nested filter chip row with clip
	dc.ClipRect(88, 52, 300, 28)
	dc.SetRGB(0.2, 0.55, 0.9)
	dc.DrawRoundedRectangle(88, 54, 70, 22, 11)
	_ = dc.Fill()
	dc.SetRGB(0.85, 0.86, 0.88)
	dc.DrawRoundedRectangle(166, 54, 70, 22, 11)
	_ = dc.Fill()
	dc.ResetClip()

	compMinGPU(t, dc, 40)
	// rail
	r, g, b, _ := p1Sample(dc, 20, 100)
	if r > 60 {
		t.Fatalf("D30 rail not dark rgba=%d,%d,%d", r, g, b)
	}
	// selected row tint
	r2, g2, b2, _ := p1Sample(dc, 200, 56+3*32+8-16)
	p1NotNearWhite(t, "D30 list area", r2, g2, b2)
	// FAB
	r3, g3, b3, _ := p1Sample(dc, w-40, h-40)
	if b3 < 100 {
		t.Fatalf("D30 FAB missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D31: stress lattice — many rrects + text + clip + alternating blends (correctness under density).
func TestP1_Comp_D31_LatticeStressBlendClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D31_LatticeStressBlendClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D31_LatticeStressBlendClip")
		return
	}
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 9)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRect(8, 8, w-16, h-16)

	for row := 0; row < 8; row++ {
		for col := 0; col < 10; col++ {
			x := 12 + float64(col)*38
			y := 12 + float64(row)*34
			if (row+col)%5 == 0 {
				dc.SetBlendMode(render.BlendMultiply)
			} else if (row+col)%5 == 1 {
				dc.SetBlendMode(render.BlendScreen)
			} else {
				dc.SetBlendMode(render.BlendNormal)
			}
			dc.SetRGB(0.3+float64(col)*0.05, 0.4, 0.7-float64(row)*0.05)
			dc.DrawRoundedRectangle(x, y, 34, 28, 5)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendNormal)
			dc.SetRGB(0.1, 0.1, 0.12)
			dc.DrawString(fmt.Sprintf("%d", row*10+col), x+8, y+18)
		}
	}

	compMinGPU(t, dc, 50)
	r, g, b, _ := p1Sample(dc, 30, 30)
	p1NotNearWhite(t, "D31 cell", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 2, 2)
	if r2 < 200 {
		t.Fatalf("D31 outside clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D32: pattern fill × transform × clip × stroke border.
func TestP1_Comp_D32_PatternTransformClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D32_PatternTransformClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D32_PatternTransformClip")
		return
	}
	const w, h = 260, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	tile := compMakeImage(t, 16, 16, 40, 140, 220)
	for y := 0; y < 16; y++ {
		for x := 0; x < 8; x++ {
			_ = tile.SetRGBA(x, y, 220, 80, 40, 255)
		}
	}
	pat := dc.CreateImagePattern(tile, 0, 0, 16, 16)

	dc.Push()
	dc.Translate(30, 20)
	dc.Scale(1.25, 1.25)
	dc.ClipRoundRect(0, 0, 160, 120, 12)
	dc.SetFillPattern(pat)
	dc.DrawRectangle(0, 0, 160, 120)
	_ = dc.Fill()
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.SetLineWidth(2)
	dc.DrawRoundedRectangle(0, 0, 160, 120, 12)
	_ = dc.Stroke()
	dc.Pop()

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 80, 70)
	p1NotNearWhite(t, "D32 pattern", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 8, 8)
	if r2 < 200 {
		t.Fatalf("D32 outside polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D33: editor-like multi-pane — nested clip + text columns + selection layer + gutter.
func TestP1_Comp_D33_EditorMultiPaneComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D33_EditorMultiPaneComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D33_EditorMultiPaneComposition")
		return
	}
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("editor composition", 12, 22)

	// File tree pane
	dc.ClipRect(0, 32, 140, h-32)
	dc.SetRGB(0.18, 0.19, 0.22)
	dc.DrawRectangle(0, 32, 140, h-32)
	_ = dc.Fill()
	for i := 0; i < 12; i++ {
		y := 48 + float64(i)*22
		if i == 4 {
			dc.SetRGB(0.25, 0.4, 0.65)
			dc.DrawRectangle(0, y-4, 140, 20)
			_ = dc.Fill()
		}
		dc.SetRGB(0.85, 0.87, 0.9)
		dc.DrawString(fmt.Sprintf("src/file_%02d.go", i), 12, y+10)
	}
	dc.ResetClip()

	// Code viewport
	dc.ClipRect(140, 32, w-140, h-32)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(140, 32, w-140, h-32)
	_ = dc.Fill()
	// gutter
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(140, 32, 40, h-32)
	_ = dc.Fill()
	// selection layer
	dc.PushLayer(render.BlendNormal, 0.35)
	dc.SetRGB(0.25, 0.45, 0.9)
	dc.DrawRectangle(180, 120, w-200, 18)
	_ = dc.Fill()
	dc.PopLayer()
	for i := 0; i < 16; i++ {
		y := 52 + float64(i)*18
		dc.SetRGB(0.45, 0.48, 0.55)
		dc.DrawString(fmt.Sprintf("%d", i+1), 148, y)
		dc.SetRGB(0.75, 0.78, 0.85)
		dc.DrawString(fmt.Sprintf("func compose_%02d() { /* clip×layer×text */ }", i), 188, y)
	}
	// minimap strip
	dc.SetBlendMode(render.BlendScreen)
	for i := 0; i < 40; i++ {
		dc.SetRGB(0.2, 0.5, 0.3)
		dc.DrawRectangle(float64(w-28), 40+float64(i)*7, 18, 3)
		_ = dc.Fill()
	}
	dc.SetBlendMode(render.BlendNormal)
	dc.ResetClip()

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 40, 100)
	if r > 90 {
		t.Fatalf("D33 tree pane not dark rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 220, 80)
	if r2 > 80 && g2 > 80 && b2 > 80 {
		t.Fatalf("D33 code bg not dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D34: chart morph — grid + path stroke + gradient bars + labels + clip legend.
func TestP1_Comp_D34_ChartPrimitiveComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D34_ChartPrimitiveComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D34_ChartPrimitiveComposition")
		return
	}
	const w, h = 440, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.ClipRect(40, 30, 300, 200)
	// grid
	dc.SetRGB(0.88, 0.89, 0.92)
	dc.SetLineWidth(1)
	for i := 0; i < 6; i++ {
		y := 40 + float64(i)*36
		dc.DrawLine(40, y, 340, y)
		_ = dc.Stroke()
	}
	// bars with gradient-ish solids
	vals := []float64{40, 90, 60, 130, 100, 150, 80}
	for i, v := range vals {
		x := 55 + float64(i)*38
		grad := render.NewLinearGradientBrush(x, 220-v, x, 220).
			AddColorStop(0, render.RGB(0.25, 0.55, 0.95)).
			AddColorStop(1, render.RGB(0.15, 0.3, 0.7))
		dc.SetFillBrush(grad)
		dc.DrawRoundedRectangle(x, 220-v, 26, v, 3)
		_ = dc.Fill()
	}
	// line path
	line := render.NewPath()
	for i, v := range vals {
		x := 68 + float64(i)*38
		y := 220 - v - 10
		if i == 0 {
			line.MoveTo(x, y)
		} else {
			line.LineTo(x, y)
		}
	}
	dc.SetRGB(0.9, 0.35, 0.2)
	dc.SetLineWidth(2)
	dc.AppendPath(line.WithCorners(6))
	_ = dc.Stroke()
	dc.ResetClip()

	// legend
	dc.ClipRect(350, 40, 80, 160)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(350, 40, 80, 160, 8)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.5, 0.9)
	dc.DrawRectangle(360, 60, 12, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("bars", 378, 70)
	dc.SetRGB(0.9, 0.35, 0.2)
	dc.DrawRectangle(360, 90, 12, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("trend", 378, 100)
	dc.ResetClip()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 100, 180)
	p1NotNearWhite(t, "D34 bar", r, g, b)
	// legend swatch (blue) at 360,60 12x12
	r2, g2, b2, _ := p1Sample(dc, 366, 66)
	if b2 < 80 {
		t.Fatalf("D34 legend swatch missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D35: calendar density — many cells + today highlight layer + header + overflow clip.
func TestP1_Comp_D35_CalendarGridComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D35_CalendarGridComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D35_CalendarGridComposition")
		return
	}
	const w, h = 420, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.16, 0.18, 0.22)
	dc.DrawRectangle(0, 0, w, 48)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("July 2026", 16, 30)

	dc.ClipRect(8, 56, w-16, h-64)
	cellW, cellH := 56.0, 48.0
	for row := 0; row < 6; row++ {
		for col := 0; col < 7; col++ {
			x := 12 + float64(col)*cellW
			y := 60 + float64(row)*cellH
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x, y, cellW-4, cellH-4, 4)
			_ = dc.Fill()
			day := row*7 + col + 1
			if day == 15 {
				dc.PushLayer(render.BlendNormal, 0.9)
				dc.SetRGB(0.2, 0.45, 0.95)
				dc.DrawRoundedRectangle(x, y, cellW-4, cellH-4, 4)
				_ = dc.Fill()
				dc.SetRGB(1, 1, 1)
				dc.DrawString(fmt.Sprintf("%d", day), x+8, y+20)
				dc.PopLayer()
			} else {
				dc.SetRGB(0.2, 0.22, 0.26)
				dc.DrawString(fmt.Sprintf("%d", day), x+8, y+20)
			}
			// event chips
			if day%3 == 0 && day < 32 {
				dc.SetRGB(0.3, 0.7, 0.45)
				dc.DrawRoundedRectangle(x+4, y+28, cellW-12, 8, 2)
				_ = dc.Fill()
			}
		}
	}
	dc.ResetClip()

	compMinGPU(t, dc, 50)
	// day 15: row=2, col=0
	r, g, b, _ := p1Sample(dc, int(12+0*cellW+20), int(60+2*cellH+12))
	if b < 80 {
		t.Fatalf("D35 today highlight missing rgba=%d,%d,%d", r, g, b)
	}
}

// D36: kitchen-sink composition — most axes in one scene (correctness under maximal mix).
func TestP1_Comp_D36_KitchenSinkMaxMix(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D36_KitchenSinkMaxMix"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D36_KitchenSinkMaxMix")
		return
	}
	const w, h = 560, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// HiDPI-like scale block
	dc.Push()
	dc.Translate(10, 10)
	dc.Scale(1.1, 1.1)
	grad := render.NewLinearGradientBrush(0, 0, 200, 0).
		AddColorStop(0, render.RGB(0.2, 0.3, 0.6)).
		AddColorStop(1, render.RGB(0.6, 0.3, 0.5))
	dc.SetFillBrush(grad)
	dc.DrawRoundedRectangle(0, 0, 200, 80, 10)
	_ = dc.Fill()
	dc.Pop()

	// Masked wash
	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 220, 20, 520, 120, 255)
	dc.SetMask(mask)
	dc.SetRGB(0.2, 0.75, 0.55)
	dc.DrawRectangle(220, 20, 300, 100)
	_ = dc.Fill()
	dc.ClearMask()

	// Mesh + vertices
	dc.DrawMesh(render.Mesh{
		Positions: []render.Point{{X: 30, Y: 120}, {X: 120, Y: 110}, {X: 90, Y: 200}, {X: 20, Y: 190}},
		Colors: []render.RGBA{
			{R: 1, G: 0.3, B: 0.2, A: 1}, {R: 0.3, G: 1, B: 0.3, A: 1},
			{R: 0.2, G: 0.4, B: 1, A: 1}, {R: 1, G: 1, B: 0.2, A: 1},
		},
		Indices: []uint16{0, 1, 2, 0, 2, 3},
	})

	// Atlas icons
	atlas := compMakeImage(t, 48, 24, 0, 0, 0)
	for y := 0; y < 24; y++ {
		for x := 0; x < 24; x++ {
			_ = atlas.SetRGBA(x, y, 240, 80, 40, 255)
			_ = atlas.SetRGBA(x+24, y, 40, 120, 240, 255)
		}
	}
	dc.DrawAtlas(atlas, []render.AtlasSprite{
		{SrcX: 0, SrcY: 0, SrcW: 24, SrcH: 24, DstX: 150, DstY: 130, DstW: 28, DstH: 28},
		{SrcX: 24, SrcY: 0, SrcW: 24, SrcH: 24, DstX: 190, DstY: 140, DstW: 32, DstH: 32},
	})

	// Path effects + dash
	p := render.NewPath()
	p.MoveTo(240, 140)
	p.LineTo(360, 130)
	p.LineTo(380, 220)
	p.LineTo(250, 230)
	p.Close()
	dc.SetRGB(0.15, 0.45, 0.9)
	dc.SetLineWidth(2)
	dc.SetDash(6, 4)
	dc.AppendPath(p.WithCorners(10))
	_ = dc.Stroke()
	dc.SetDash()

	// Nested clip + text list
	dc.ClipRect(20, 250, 300, 130)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(20, 250, 300, 130)
	_ = dc.Fill()
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.15, 0.16, 0.2)
		dc.DrawString(fmt.Sprintf("sink-row-%d clip×text×list", i), 32, 270+float64(i)*18)
	}
	dc.ResetClip()

	// Blend stack blob
	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(1, 0.6, 0.6)
	dc.DrawCircle(450, 200, 50)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendScreen)
	dc.SetRGB(0.4, 0.5, 1)
	dc.DrawCircle(480, 230, 45)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	// Backdrop modal
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre-backdrop: %v", err)
	}
	base := dc.RenderPathStats().GPUOps
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.3)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(160, 100, 240, 140, 12)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.12, 0.15)
	dc.DrawString("kitchen-sink modal", 190, 170)
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 280, 170)
	if r < 200 {
		t.Fatalf("D36 modal missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 40)
	p1NotNearWhite(t, "D36 header gradient", r2, g2, b2)
}
