//go:build !nogpu

package render_test

// Phase A mega composition probes D59+ — denser multi-axis scenes, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D59: wrapped/anchored/stroked text × clip × layer card stack.
func TestP1_Comp_D59_RichTextClipLayerStack(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D59_RichTextClipLayerStack"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D59_RichTextClipLayerStack")
		return
	}
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// Article column with wrap
	dc.ClipRect(20, 20, 240, 260)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(20, 20, 240, 260)
	_ = dc.Fill()
	dc.SetRGB(0.12, 0.14, 0.18)
	body := "Composition probes validate arbitrary axis crossings: clip, layer, blend, text wrapping, and stroke outlines under real GPU paths."
	dc.DrawStringWrapped(body, 32, 36, 0, 0, 210, 1.25, render.AlignLeft)
	dc.SetRGB(0.15, 0.45, 0.9)
	dc.SetLineWidth(1.2)
	dc.StrokeString("Outlined title", 32, 200)
	dc.ResetClip()

	// Side cards with anchored labels
	for i := 0; i < 3; i++ {
		y := 30 + float64(i)*85
		dc.PushLayer(render.BlendNormal, 0.92)
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(280, y, 120, 70, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.28)
		dc.DrawStringAnchored(fmt.Sprintf("card-%d", i+1), 340, y+35, 0.5, 0.5)
		dc.PopLayer()
	}

	compMinGPU(t, dc, 6)
	// text ink in article
	ink := 0
	for y := 40; y < 120; y += 3 {
		for x := 40; x < 220; x += 3 {
			r, g, b, _ := p1Sample(dc, x, y)
			if r < 230 || g < 230 || b < 230 {
				ink++
			}
		}
	}
	if ink < 30 {
		t.Fatalf("D59 wrapped text ink low: %d", ink)
	}
	r, g, b, _ := p1Sample(dc, 340, 65)
	p1NotNearWhite(t, "D59 card", r, g, b)
}

// D60: Difference/ColorBurn/Exclusion blend stack × image × clip.
func TestP1_Comp_D60_DiffBurnExclusionBlendStack(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D60_DiffBurnExclusionBlendStack"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D60_DiffBurnExclusionBlendStack")
		return
	}
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.75, 0.35, 0.3)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	img := compMakeImage(t, 64, 64, 40, 160, 220)
	dc.DrawImage(img, 30, 30)

	dc.ClipRoundRect(20, 20, 260, 180, 12)
	dc.SetBlendMode(render.BlendDifference)
	dc.SetRGB(0.3, 0.8, 0.4)
	dc.DrawCircle(100, 100, 55)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendColorBurn)
	dc.SetRGB(0.9, 0.7, 0.2)
	dc.DrawCircle(180, 110, 50)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendExclusion)
	dc.SetRGBA(0.2, 0.3, 0.9, 1)
	dc.DrawRectangle(60, 140, 180, 40)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
	dc.ResetClip()

	compMinGPU(t, dc, 5)
	r, g, b, _ := p1Sample(dc, 100, 100)
	p1NotNearWhite(t, "D60 difference zone", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 180, 110)
	p1NotNearWhite(t, "D60 burn zone", r2, g2, b2)
}

// D61: AA on/off geometry × dash offset × miter joins × clip.
func TestP1_Comp_D61_AADashOffsetMiterClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D61_AADashOffsetMiterClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D61_AADashOffsetMiterClip")
		return
	}
	const w, h = 340, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRect(10, 10, 320, 200)

	dc.SetAntiAlias(true)
	dc.SetRGB(0.15, 0.45, 0.9)
	dc.SetLineWidth(6)
	dc.SetLineJoin(render.LineJoinMiter)
	dc.SetLineCap(render.LineCapButt)
	dc.SetDash(12, 8)
	dc.SetDashOffset(4)
	dc.MoveTo(30, 40)
	dc.LineTo(160, 30)
	dc.LineTo(200, 120)
	dc.LineTo(80, 160)
	_ = dc.Stroke()

	dc.SetAntiAlias(false)
	dc.SetRGB(0.9, 0.3, 0.2)
	dc.SetDashOffset(0)
	dc.SetDash(8, 6)
	dc.SetLineWidth(4)
	dc.MoveTo(220, 40)
	dc.LineTo(300, 50)
	dc.LineTo(280, 160)
	dc.LineTo(210, 140)
	_ = dc.Stroke()
	dc.SetDash()
	dc.SetAntiAlias(true)
	dc.ResetClip()

	compMinGPU(t, dc, 2)
	ink := 0
	for y := 20; y < 180; y += 2 {
		for x := 20; x < 320; x += 2 {
			r, g, b, _ := p1Sample(dc, x, y)
			if r < 230 || g < 230 || b < 230 {
				ink++
			}
		}
	}
	if ink < 80 {
		t.Fatalf("D61 stroke ink low: %d", ink)
	}
}

// D62: Resize mid-frame then recompose multi-panel scene.
func TestP1_Comp_D62_ResizeRecomposePanels(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D62_ResizeRecomposePanels"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D62_ResizeRecomposePanels")
		return
	}
	dc := render.NewContext(200, 160)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, 200, 160)
	dc.SetRGB(0.2, 0.5, 0.9)
	dc.DrawRectangle(10, 10, 80, 60)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("pre-resize: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	if err := dc.Resize(360, 240); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	// Full recompose at new size
	p1White(dc, 360, 240)
	for i := 0; i < 4; i++ {
		x := 16 + float64(i%2)*170
		y := 20 + float64(i/2)*100
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(x, y, 150, 85, 8)
		_ = dc.Fill()
		dc.SetRGB(0.15+float64(i)*0.1, 0.35, 0.75)
		dc.DrawRectangle(x, y, 150, 24)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(fmt.Sprintf("panel %d after resize", i+1), x+10, y+16)
	}
	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 40, 30)
	if b < 80 {
		t.Fatalf("D62 post-resize panel missing rgba=%d,%d,%d", r, g, b)
	}
	if dc.Width() != 360 || dc.Height() != 240 {
		t.Fatalf("D62 size want 360x240 got %dx%d", dc.Width(), dc.Height())
	}
}

// D63: FrameDamage tracking + single-rect damage present path.
func TestP1_Comp_D63_FrameDamageSingleRectPresent(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D63_FrameDamageSingleRectPresent"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D63_FrameDamageSingleRectPresent")
		return
	}
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.4, 0.85)
	dc.DrawRoundedRectangle(20, 20, 120, 80, 8)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base: %v", err)
	}

	dc.ResetFrameDamage()
	dc.SetRGB(0.9, 0.35, 0.2)
	dc.DrawRoundedRectangle(160, 40, 130, 90, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.7, 0.4)
	dc.DrawCircle(100, 150, 30)
	_ = dc.Fill()
	rects := dc.FrameDamage()
	if len(rects) == 0 {
		t.Fatalf("D63 expected FrameDamage rects after draws")
	}
	uni := dc.FrameDamageUnion()
	if uni.Empty() {
		t.Fatalf("D63 FrameDamageUnion empty")
	}
	t.Logf("D63 damage n=%d union=%v", len(rects), uni)

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	if err := dc.PresentFrameDamage(view, uint32(w), uint32(h), uni, func() error { return nil }); err != nil {
		t.Fatalf("PresentFrameDamage: %v", err)
	}
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	r, g, b, _ := p1Sample(dc, 200, 80)
	if r < 100 {
		t.Fatalf("D63 orange panel missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 100, 150)
	if g2 < 80 {
		t.Fatalf("D63 green circle missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D64: mask-from-alpha snapshot × PushMaskLayer × image × text.
func TestP1_Comp_D64_MaskFromAlphaLayerImageText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D64_MaskFromAlphaLayerImageText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D64_MaskFromAlphaLayerImageText")
		return
	}
	const w, h = 280, 200
	// Build alpha source: transparent outside, opaque circle (true A8 mask).
	// Opaque black/white RGB fills both have A=255 and would yield a full mask.
	src := render.NewContext(w, h)
	src.Clear()
	src.SetRGBA(1, 1, 1, 1)
	src.DrawCircle(140, 100, 70)
	_ = src.Fill()
	if err := src.FlushGPU(); err != nil {
		src.Close()
		t.Fatalf("mask src: %v", err)
	}
	mask := render.NewMaskFromAlpha(src.Image())
	if mask.At(140, 100) < 200 || mask.At(10, 10) > 20 {
		src.Close()
		t.Fatalf("D64 mask not circular alpha center=%d corner=%d", mask.At(140, 100), mask.At(10, 10))
	}
	src.Close()

	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 14)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// colorful base
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.2+float64(i)*0.08, 0.4, 0.85-float64(i)*0.05)
		dc.DrawRectangle(float64(i)*35, 0, 35, h)
		_ = dc.Fill()
	}
	img := compMakeImage(t, 40, 40, 220, 80, 40)
	dc.DrawImage(img, 40, 40)
	dc.DrawImage(img, 200, 120)

	dc.PushMaskLayer(mask)
	dc.SetRGB(0.1, 0.1, 0.12)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("alpha-mask", 95, 105)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("layer: %v", err)
	}
	dc.PopLayer()

	compMinGPU(t, dc, 3)
	// Center (inside circle mask) should be dark overlay (~25,25,31)
	r, g, b, _ := p1Sample(dc, 140, 100)
	if r > 60 || g > 60 || b > 60 {
		t.Fatalf("D64 masked center expected dark rgba=%d,%d,%d", r, g, b)
	}
	// Corner outside circle: first stripe remains (not full-frame dark layer)
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 40 && g2 < 40 && b2 < 45 {
		t.Fatalf("D64 corner should not be fully masked dark rgba=%d,%d,%d", r2, g2, b2)
	}
	// Sanity: corner must differ from the dark overlay so full-surface mask fails.
	dr, dg, db := int(r2)-int(r), int(g2)-int(g), int(b2)-int(b)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if dr < 8 && dg < 8 && db < 8 {
		t.Fatalf("D64 corner matches center dark — mask likely full-surface rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D65: infinite-canvas pan/zoom — CTM stack × grid × nodes × connectors.
func TestP1_Comp_D65_InfiniteCanvasPanZoom(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D65_InfiniteCanvasPanZoom"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D65_InfiniteCanvasPanZoom")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// viewport chrome
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawRectangle(0, 0, w, 32)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("canvas pan/zoom composition", 12, 22)

	dc.ClipRect(0, 32, w, h-32)
	dc.Push()
	dc.Translate(40, 20) // pan
	dc.Scale(1.25, 1.25) // zoom
	// grid
	dc.SetRGB(0.22, 0.24, 0.28)
	dc.SetLineWidth(1)
	for i := 0; i < 16; i++ {
		dc.DrawLine(float64(i)*40, 0, float64(i)*40, 400)
		_ = dc.Stroke()
		dc.DrawLine(0, float64(i)*40, 500, float64(i)*40)
		_ = dc.Stroke()
	}
	// nodes
	nodes := [][2]float64{{40, 40}, {180, 60}, {120, 160}, {260, 140}}
	for i, n := range nodes {
		dc.SetRGB(0.25, 0.45, 0.85)
		dc.DrawRoundedRectangle(n[0], n[1], 90, 48, 8)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(fmt.Sprintf("N%d", i+1), n[0]+30, n[1]+28)
	}
	// connectors
	dc.SetRGB(0.7, 0.75, 0.85)
	dc.SetLineWidth(2)
	dc.SetDash(6, 4)
	dc.DrawLine(130, 64, 180, 84)
	_ = dc.Stroke()
	dc.DrawLine(85, 88, 120, 160)
	_ = dc.Stroke()
	dc.DrawLine(210, 108, 260, 150)
	_ = dc.Stroke()
	dc.SetDash()
	dc.Pop()
	dc.ResetClip()

	// minimap
	dc.PushLayer(render.BlendNormal, 0.9)
	dc.SetRGB(0.18, 0.2, 0.24)
	dc.DrawRoundedRectangle(w-110, h-90, 96, 72, 6)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.55, 0.95)
	dc.DrawRectangle(w-95, h-70, 30, 20)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 120, 100)
	p1NotNearWhite(t, "D65 canvas", r, g, b)
}

// D66: chat density — bubbles × avatars × clip list × composer layer.
func TestP1_Comp_D66_ChatBubbleComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D66_ChatBubbleComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D66_ChatBubbleComposition")
		return
	}
	const w, h = 400, 360
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
	dc.DrawString("chat composition", 12, 26)

	avatar := compMakeImage(t, 28, 28, 60, 140, 220)
	dc.ClipRect(0, 40, w, h-100)
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(0, 40, w, h-100)
	_ = dc.Fill()
	msgs := []struct {
		me   bool
		text string
	}{
		{false, "Hey, can we compose clip×layer×text?"},
		{true, "Yes — bubbles are just rrects + wrap."},
		{false, "Add avatar images and timestamps too."},
		{true, "And a sticky composer bar at bottom."},
		{false, "Perfect for arbitrary UI density."},
	}
	y := 56.0
	for _, m := range msgs {
		if m.me {
			dc.SetRGB(0.2, 0.5, 0.95)
			dc.DrawRoundedRectangle(120, y, 250, 44, 12)
			_ = dc.Fill()
			dc.SetRGB(1, 1, 1)
			dc.DrawStringWrapped(m.text, 132, y+8, 0, 0, 220, 1.1, render.AlignLeft)
		} else {
			dc.DrawImageCircular(avatar, 28, y+20, 14)
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(50, y, 250, 44, 12)
			_ = dc.Fill()
			dc.SetRGB(0.15, 0.16, 0.2)
			dc.DrawStringWrapped(m.text, 62, y+8, 0, 0, 220, 1.1, render.AlignLeft)
		}
		y += 56
	}
	dc.ResetClip()

	// composer
	dc.PushLayer(render.BlendNormal, 0.98)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, h-60, w, 60)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(12, h-48, w-100, 36, 18)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(w-76, h-48, 64, 36, 8)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 20)
	// second message is "me" blue bubble at y=56+56=112
	r, g, b, _ := p1Sample(dc, 200, 125)
	if b < 80 || r > b {
		t.Fatalf("D66 me bubble not blue rgba=%d,%d,%d", r, g, b)
	}
	// peer bubble white card with text ink
	ink := 0
	for y := 60; y < 95; y++ {
		for x := 60; x < 280; x += 2 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 240 || gg < 240 || bb < 240 {
				ink++
			}
		}
	}
	if ink < 5 {
		t.Fatalf("D66 peer bubble ink low: %d", ink)
	}
	r2, g2, b2, _ := p1Sample(dc, w-40, h-30)
	if b2 < 80 {
		t.Fatalf("D66 send btn missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D67: gantt chart — rows × bars × today line × clip header.
func TestP1_Comp_D67_GanttChartComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D67_GanttChartComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D67_GanttChartComposition")
		return
	}
	const w, h = 520, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("gantt composition", 12, 24)

	// left labels freeze
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 36, 120, h-36)
	_ = dc.Fill()
	tasks := []string{"Design", "ABI bind", "Facade", "Render", "Compose", "Perf"}
	for i, name := range tasks {
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(name, 12, 60+float64(i)*40)
	}

	dc.ClipRect(120, 36, w-120, h-36)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(120, 36, w-120, h-36)
	_ = dc.Fill()
	// grid weeks
	dc.SetRGB(0.9, 0.91, 0.93)
	for i := 0; i < 10; i++ {
		x := 120 + float64(i)*40
		dc.DrawLine(x, 36, x, h)
		_ = dc.Stroke()
	}
	bars := []struct {
		row, x, w float64
		r, g, b   float64
	}{
		{0, 140, 80, 0.2, 0.5, 0.95},
		{1, 180, 100, 0.3, 0.7, 0.4},
		{2, 220, 90, 0.9, 0.55, 0.2},
		{3, 250, 120, 0.55, 0.35, 0.9},
		{4, 300, 70, 0.2, 0.65, 0.75},
		{5, 340, 60, 0.9, 0.3, 0.35},
	}
	for _, b := range bars {
		y := 48 + b.row*40
		dc.SetRGB(b.r, b.g, b.b)
		dc.DrawRoundedRectangle(b.x, y, b.w, 22, 4)
		_ = dc.Fill()
	}
	// today line
	dc.SetRGB(0.95, 0.25, 0.25)
	dc.SetLineWidth(2)
	dc.DrawLine(280, 36, 280, h)
	_ = dc.Stroke()
	dc.ResetClip()

	compMinGPU(t, dc, 20)
	r, g, b, _ := p1Sample(dc, 200, 55)
	p1NotNearWhite(t, "D67 bar", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 40, 60)
	p1NotNearWhite(t, "D67 label col", r2, g2, b2)
}

// D68: heatmap grid × color scale × tooltip layer × clip.
func TestP1_Comp_D68_HeatmapTooltipComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D68_HeatmapTooltipComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D68_HeatmapTooltipComposition")
		return
	}
	const w, h = 360, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("heatmap composition", 12, 20)

	dc.ClipRect(20, 36, 240, 220)
	for row := 0; row < 10; row++ {
		for col := 0; col < 10; col++ {
			v := float64((row*3+col*7)%10) / 9
			dc.SetRGB(0.15+v*0.75, 0.2+v*0.2, 0.85-v*0.5)
			dc.DrawRectangle(20+float64(col)*24, 36+float64(row)*22, 22, 20)
			_ = dc.Fill()
		}
	}
	dc.ResetClip()

	// scale legend
	for i := 0; i < 8; i++ {
		v := float64(i) / 7
		dc.SetRGB(0.15+v*0.75, 0.2+v*0.2, 0.85-v*0.5)
		dc.DrawRectangle(280, 40+float64(i)*22, 18, 18)
		_ = dc.Fill()
	}

	// tooltip
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRoundedRectangle(140, 100, 110, 48, 6)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("cell 3,4 = 0.72", 150, 128)
	dc.PopLayer()

	compMinGPU(t, dc, 80)
	r, g, b, _ := p1Sample(dc, 40, 50)
	p1NotNearWhite(t, "D68 heat cell", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 180, 120)
	if r2 > 80 {
		t.Fatalf("D68 tooltip should be dark rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D69: multi-modal stack — 2 backdrops + nested cards + focus ring.
func TestP1_Comp_D69_MultiModalStackComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D69_MultiModalStackComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D69_MultiModalStackComposition")
		return
	}
	const w, h = 420, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// page
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.94, 0.95, 0.97)
		dc.DrawRoundedRectangle(24, 24+float64(i)*40, w-48, 32, 6)
		_ = dc.Fill()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("page: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// modal 1
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.35)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(60, 50, 300, 200, 12)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("First modal", 80, 90)
	dc.PopLayer()

	// modal 2 on top
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.25)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(100, 90, 220, 140, 10)
	_ = dc.Fill()
	// focus ring
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.SetLineWidth(3)
	dc.DrawRoundedRectangle(100, 90, 220, 140, 10)
	_ = dc.Stroke()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("Nested confirm", 130, 150)
	dc.SetRGB(0.2, 0.55, 0.9)
	dc.DrawRoundedRectangle(140, 180, 80, 28, 6)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, base+1)
	r, g, b, _ := p1Sample(dc, 210, 150)
	if r < 200 {
		t.Fatalf("D69 nested modal missing rgba=%d,%d,%d", r, g, b)
	}
	// dimmed page edge
	r2, g2, b2, _ := p1Sample(dc, 20, 20)
	if r2 > 180 && g2 > 180 {
		t.Fatalf("D69 expected dimmed page rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D70: map-like tile grid × markers × path route × popup.
func TestP1_Comp_D70_MapTilesRoutePopup(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D70_MapTilesRoutePopup"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D70_MapTilesRoutePopup")
		return
	}
	const w, h = 400, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// tiles
	for row := 0; row < 6; row++ {
		for col := 0; col < 8; col++ {
			shade := 0.55 + 0.08*float64((row+col)%3)
			dc.SetRGB(shade*0.7, shade*0.85, shade*0.65)
			dc.DrawRectangle(float64(col)*50, float64(row)*50, 50, 50)
			_ = dc.Fill()
			dc.SetRGB(shade*0.5, shade*0.6, shade*0.45)
			dc.SetLineWidth(1)
			dc.DrawRectangle(float64(col)*50, float64(row)*50, 50, 50)
			_ = dc.Stroke()
		}
	}
	// route
	route := render.NewPath()
	route.MoveTo(40, 250)
	route.LineTo(120, 180)
	route.LineTo(200, 200)
	route.LineTo(300, 80)
	route.LineTo(360, 120)
	dc.SetRGB(0.15, 0.45, 0.95)
	dc.SetLineWidth(4)
	dc.SetLineCap(render.LineCapRound)
	dc.SetLineJoin(render.LineJoinRound)
	dc.AppendPath(route.WithCorners(12))
	_ = dc.Stroke()

	// markers
	for _, p := range [][2]float64{{40, 250}, {200, 200}, {360, 120}} {
		dc.SetRGB(0.9, 0.25, 0.25)
		dc.DrawCircle(p[0], p[1], 7)
		_ = dc.Fill()
	}

	// popup
	dc.PushLayer(render.BlendNormal, 0.96)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(210, 40, 150, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("POI: render hub", 222, 70)
	dc.DrawString("eta 12 min", 222, 92)
	dc.PopLayer()

	compMinGPU(t, dc, 40)
	r, g, b, _ := p1Sample(dc, 120, 180)
	p1NotNearWhite(t, "D70 route/tiles", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 260, 70)
	if r2 < 200 {
		t.Fatalf("D70 popup missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D71: code diff view — line gutters × add/del layers × clip × text.
func TestP1_Comp_D71_CodeDiffComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D71_CodeDiffComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D71_CodeDiffComposition")
		return
	}
	const w, h = 480, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("diff composition", 12, 22)

	dc.ClipRect(0, 32, w, h-32)
	lines := []struct {
		kind byte // ' ', '+', '-'
		text string
	}{
		{' ', "func compose() {"},
		{'-', "  oldPath := clip.Rect()"},
		{'+', "  newPath := clip.RoundRect()"},
		{'+', "  layer.Push(BlendNormal, 0.9)"},
		{' ', "  drawPrimitives()"},
		{'-', "  // TODO: remove"},
		{'+', "  flushGPU()"},
		{' ', "}"},
		{' ', ""},
		{'+', " // more coverage"},
		{' ', " return nil"},
	}
	for i, ln := range lines {
		y := 40 + float64(i)*22
		switch ln.kind {
		case '+':
			dc.SetRGB(0.12, 0.28, 0.16)
			dc.DrawRectangle(0, y-4, w, 22)
			_ = dc.Fill()
			dc.SetRGB(0.45, 0.9, 0.55)
		case '-':
			dc.SetRGB(0.32, 0.14, 0.14)
			dc.DrawRectangle(0, y-4, w, 22)
			_ = dc.Fill()
			dc.SetRGB(0.95, 0.5, 0.5)
		default:
			dc.SetRGB(0.75, 0.78, 0.85)
		}
		dc.SetRGB(0.45, 0.48, 0.55)
		dc.DrawString(fmt.Sprintf("%2d", i+1), 8, y+10)
		if ln.kind == '+' {
			dc.SetRGB(0.45, 0.9, 0.55)
		} else if ln.kind == '-' {
			dc.SetRGB(0.95, 0.5, 0.5)
		} else {
			dc.SetRGB(0.8, 0.82, 0.88)
		}
		prefix := "  "
		if ln.kind != ' ' {
			prefix = string(ln.kind) + " "
		}
		dc.DrawString(prefix+ln.text, 40, y+10)
	}
	dc.ResetClip()

	compMinGPU(t, dc, 15)
	// added line greenish bg
	r, g, b, _ := p1Sample(dc, 100, 40+2*22)
	if g < 40 {
		t.Fatalf("D71 add line bg missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 100, 40+1*22)
	if r2 < 40 {
		t.Fatalf("D71 del line bg missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D72: bicubic image scale × clip path × stroke pattern border.
func TestP1_Comp_D72_BicubicImagePathClipPattern(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D72_BicubicImagePathClipPattern"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D72_BicubicImagePathClipPattern")
		return
	}
	const w, h = 300, 220
	dc := render.NewContext(w, h)
	defer dc.Close()

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	img := compMakeImage(t, 32, 32, 0, 0, 0)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			_ = img.SetRGBA(x, y, uint8(x*7), uint8(y*7), 180, 255)
		}
	}

	dc.MoveTo(40, 30)
	dc.LineTo(250, 40)
	dc.LineTo(240, 180)
	dc.LineTo(50, 170)
	dc.ClosePath()
	dc.Clip()

	// solid underlay so scaled image/blend is visible even if filter is soft
	dc.SetRGB(0.15, 0.45, 0.85)
	dc.DrawRectangle(40, 30, 210, 150)
	_ = dc.Fill()
	dc.DrawImageEx(img, render.DrawImageOptions{
		X: 50, Y: 40, DstWidth: 180, DstHeight: 120,
		Interpolation: render.InterpBicubic,
		Opacity:       0.85,
		BlendMode:     render.BlendNormal,
	})

	tile := compMakeImage(t, 6, 6, 220, 60, 40)
	dc.SetStrokePattern(dc.CreateImagePattern(tile, 0, 0, 6, 6))
	dc.SetLineWidth(5)
	dc.DrawRectangle(50, 40, 180, 130)
	_ = dc.Stroke()

	compMinGPU(t, dc, 2)
	r, g, b, _ := p1Sample(dc, 120, 100)
	p1NotNearWhite(t, "D72 clipped content", r, g, b)
	if b < 40 {
		t.Fatalf("D72 expected blue-ish underlay/image rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 200 {
		t.Fatalf("D72 outside path clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D73: dock layout — activity bar × side × editor × panel × status.
func TestP1_Comp_D73_IDEDockLayoutComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D73_IDEDockLayoutComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D73_IDEDockLayoutComposition")
		return
	}
	const w, h = 560, 360
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	// activity bar
	dc.SetRGB(0.14, 0.15, 0.18)
	dc.DrawRectangle(0, 0, 48, h-24)
	_ = dc.Fill()
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.3, 0.35, 0.45)
		dc.DrawRoundedRectangle(10, 16+float64(i)*48, 28, 28, 6)
		_ = dc.Fill()
	}
	// side bar
	dc.ClipRect(48, 0, 160, h-24)
	dc.SetRGB(0.18, 0.19, 0.22)
	dc.DrawRectangle(48, 0, 160, h-24)
	_ = dc.Fill()
	for i := 0; i < 10; i++ {
		dc.SetRGB(0.8, 0.82, 0.86)
		dc.DrawString(fmt.Sprintf("explorer %d", i), 60, 28+float64(i)*28)
	}
	dc.ResetClip()
	// editor
	dc.ClipRect(208, 0, w-208, h-120)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(208, 0, w-208, h-120)
	_ = dc.Fill()
	for i := 0; i < 12; i++ {
		dc.SetRGB(0.55, 0.6, 0.7)
		dc.DrawString(fmt.Sprintf("%2d  render.Compose(axis_%d)", i+1, i), 220, 24+float64(i)*18)
	}
	dc.ResetClip()
	// bottom panel
	dc.SetRGB(0.16, 0.17, 0.2)
	dc.DrawRectangle(208, h-120, w-208, 96)
	_ = dc.Fill()
	dc.SetRGB(0.75, 0.8, 0.5)
	dc.DrawString("terminal $ go test ./render -run Comp", 220, h-80)
	// status
	dc.SetRGB(0.15, 0.45, 0.85)
	dc.DrawRectangle(0, h-24, w, 24)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Ln 12, Col 4  |  GPU ops composition", 12, h-8)

	compMinGPU(t, dc, 25)
	// pure activity bar chrome (avoid icon hit)
	r, g, b, _ := p1Sample(dc, 4, 8)
	if r > 70 {
		t.Fatalf("D73 activity bar not dark rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 300, h-12)
	if b2 < 80 {
		t.Fatalf("D73 status missing rgba=%d,%d,%d", r2, g2, b2)
	}
	// editor dark (gap between text lines)
	r3, g3, b3, _ := p1Sample(dc, 500, 10)
	if r3 > 80 {
		t.Fatalf("D73 editor bg not dark rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D74: filter+mask+blend+text mega — many axes in one correctness scene.
func TestP1_Comp_D74_FilterMaskBlendTextMega(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D74_FilterMaskBlendTextMega"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D74_FilterMaskBlendTextMega")
		return
	}
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
	grad := render.NewLinearGradientBrush(0, 0, w, 0).
		AddColorStop(0, render.RGB(0.2, 0.3, 0.7)).
		AddColorStop(1, render.RGB(0.7, 0.3, 0.5))
	dc.SetFillBrush(grad)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	img := compMakeImage(t, 48, 48, 40, 200, 80)
	dc.DrawImageEx(img, render.DrawImageOptions{X: 30, Y: 30, DstWidth: 80, DstHeight: 80, Opacity: 0.85})

	mask := render.NewMask(w, h)
	compFillMaskRect(mask, 120, 40, 320, 200, 255)
	dc.SetMask(mask)
	dc.SetBlendMode(render.BlendSoftLight)
	dc.SetRGB(1, 0.8, 0.2)
	dc.DrawCircle(220, 120, 70)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)
	dc.ClearMask()

	dc.PushLayer(render.BlendNormal, 0.8)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(40, 160, 200, 60, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawStringWrapped("mask×blend×layer×text mega", 50, 175, 0, 0, 180, 1.15, render.AlignLeft)
	dc.PopLayer()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.5})
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)

	r, g, b, _ := p1Sample(dc, 60, 50)
	p1NotNearWhite(t, "D74 image", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 100, 180)
	p1NotNearWhite(t, "D74 card", r2, g2, b2)
}

// D75: stress dashboard — KPI cards × sparkline paths × table × live badge damage.
func TestP1_Comp_D75_DashboardKPISparklineTable(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D75_DashboardKPISparklineTable"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D75_DashboardKPISparklineTable")
		return
	}
	const w, h = 560, 380
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("dashboard mega composition", 16, 24)

	// KPI cards
	for i := 0; i < 4; i++ {
		x := 16 + float64(i)*135
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 40, 124, 80, 10)
		_ = dc.Fill()
		dc.SetRGB(0.45, 0.48, 0.55)
		dc.DrawString(fmt.Sprintf("KPI-%d", i+1), x+12, 60)
		dc.SetRGB(0.12, 0.14, 0.18)
		dc.DrawString(fmt.Sprintf("%d.%d k", 12+i, i*3), x+12, 88)
		// sparkline
		sp := render.NewPath()
		for s := 0; s < 12; s++ {
			px := x + 10 + float64(s)*9
			py := 100 - 8*math.Sin(float64(s+i)*0.7)
			if s == 0 {
				sp.MoveTo(px, py)
			} else {
				sp.LineTo(px, py)
			}
		}
		dc.SetRGB(0.2, 0.55, 0.95)
		dc.SetLineWidth(2)
		dc.AppendPath(sp)
		_ = dc.Stroke()
	}

	// table
	dc.ClipRect(16, 140, w-32, 200)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(16, 140, w-32, 200)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawRectangle(16, 140, w-32, 28)
	_ = dc.Fill()
	for c, name := range []string{"Name", "Region", "Value", "Delta", "Status"} {
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawString(name, 28+float64(c)*100, 158)
	}
	for r := 0; r < 7; r++ {
		y := 176 + float64(r)*22
		if r%2 == 0 {
			dc.SetRGB(0.97, 0.98, 0.99)
			dc.DrawRectangle(16, y-6, w-32, 22)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("row-%02d", r+1), 28, y+8)
		dc.DrawString([]string{"APAC", "EMEA", "AMER"}[r%3], 128, y+8)
		dc.DrawString(fmt.Sprintf("%.1f", 10.5+float64(r)*1.7), 228, y+8)
		dc.SetRGB(0.2, 0.65, 0.35)
		dc.DrawString("+2.1%", 328, y+8)
		dc.SetRGB(0.2, 0.55, 0.9)
		dc.DrawRoundedRectangle(420, y-2, 50, 16, 8)
		_ = dc.Fill()
	}
	dc.ResetClip()

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base: %v", err)
	}
	base := dc.RenderPathStats().GPUOps
	// live badge damage
	dc.ClipRect(500, 12, 40, 20)
	dc.SetRGB(0.9, 0.25, 0.25)
	dc.DrawRoundedRectangle(500, 12, 40, 18, 9)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("LIVE", 506, 25)
	dc.ResetClip()
	compMinGPU(t, dc, base+1)

	// KPI title text ink
	ink := 0
	for x := 28; x < 120; x++ {
		r, g, b, _ := p1Sample(dc, x, 56)
		if r < 220 || g < 220 || b < 220 {
			ink++
		}
	}
	if ink < 3 {
		t.Fatalf("D75 KPI text ink low: %d", ink)
	}
	// table header band
	r, g, b, _ := p1Sample(dc, 40, 150)
	if r < 200 {
		t.Fatalf("D75 table header expected light rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 520, 20)
	if r2 < 100 {
		t.Fatalf("D75 LIVE badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}
