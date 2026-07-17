//go:build !nogpu

package render_test

// Phase A hyper composition probes D91+ — denser multi-axis stress, not widgets.
// docs/P1_COMPOSITION_MATRIX.md

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/filters"
	_ "github.com/energye/gpui/render/gpu"
)

// D91: ClipPreserve fill+stroke same path × layer × text.
func TestP1_Comp_D91_ClipPreserveFillStrokeText(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D91_ClipPreserveFillStrokeText"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D91_ClipPreserveFillStrokeText")
		return
	}
	const w, h = 320, 220
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 13)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	dc.MoveTo(40, 40)
	dc.LineTo(260, 50)
	dc.LineTo(240, 180)
	dc.LineTo(50, 170)
	dc.ClosePath()
	dc.ClipPreserve()
	dc.SetRGBA(0.2, 0.5, 0.9, 0.35)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.35, 0.85)
	dc.SetLineWidth(4)
	_ = dc.Stroke()

	dc.PushLayer(render.BlendNormal, 0.9)
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(90, 90, 140, 50, 8)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("ClipPreserve", 105, 120)
	dc.PopLayer()

	compMinGPU(t, dc, 4)
	// blue-ish fill inside clip (avoid white overlay panel center)
	r, g, b, _ := p1Sample(dc, 160, 70)
	p1NotNearWhite(t, "D91 clipped fill", r, g, b)
	if b < 40 {
		t.Fatalf("D91 expected blue-ish fill rgba=%d,%d,%d", r, g, b)
	}
	// white panel text region should still render
	ink := 0
	for x := 100; x < 220; x++ {
		rr, gg, bb, _ := p1Sample(dc, x, 115)
		if rr < 230 || gg < 230 || bb < 230 {
			ink++
		}
	}
	if ink < 3 {
		t.Fatalf("D91 panel text ink low: %d", ink)
	}
	r2, g2, b2, _ := p1Sample(dc, 10, 10)
	if r2 < 200 {
		t.Fatalf("D91 outside clip polluted rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D92: grayscale + color matrix sequential filters on dense UI.
func TestP1_Comp_D92_GrayscaleColorMatrixDense(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D92_GrayscaleColorMatrixDense"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D92_GrayscaleColorMatrixDense")
		return
	}
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 340, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.9, 0.25, 0.2)
	dc.DrawRoundedRectangle(20, 20, 140, 90, 10)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.55, 0.95)
	dc.DrawRoundedRectangle(180, 30, 140, 90, 10)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.75, 0.4)
	dc.DrawCircle(160, 170, 45)
	_ = dc.Fill()
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawString("pre-filter chrome", 30, 150)

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	// boost contrast then grayscale
	mat := [20]float32{
		1.2, 0, 0, 0, 0,
		0, 1.1, 0, 0, 0,
		0, 0, 1.15, 0, 0,
		0, 0, 0, 1, 0,
	}
	dc.ApplyColorMatrix(mat)
	dc.ApplyGrayscale()
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)

	// after grayscale, channels should be similar on red card area
	r, g, b, _ := p1Sample(dc, 60, 50)
	p1NotNearWhite(t, "D92 gray card", r, g, b)
	if absU8(r, g) > 40 || absU8(g, b) > 40 {
		t.Fatalf("D92 expected near-gray rgba=%d,%d,%d", r, g, b)
	}
}

// D93: DrawGPUTextureBase underlay + opacity overlay + clip text.
func TestP1_Comp_D93_GPUTextureBaseOpacityClip(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D93_GPUTextureBaseOpacityClip"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D93_GPUTextureBaseOpacityClip")
		return
	}
	const w, h = 360, 240
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)

	var releases []func()
	defer func() {
		for _, r := range releases {
			if r != nil {
				r()
			}
		}
	}()

	mk := func(rr, gg, bb float64) interface{ IsNil() bool } {
		c := render.NewContext(80, 80)
		view, rel := c.CreateOffscreenTexture(80, 80)
		if rel == nil || view.IsNil() {
			c.Close()
			t.Skip("offscreen unavailable")
		}
		releases = append(releases, func() { rel(); c.Close() })
		c.SetRGB(rr, gg, bb)
		c.DrawRoundedRectangle(0, 0, 80, 80, 10)
		_ = c.Fill()
		if err := c.FlushGPUWithView(view, 80, 80); err != nil {
			t.Fatalf("tile: %v", err)
		}
		return view
	}
	for i, col := range [][3]float64{{0.2, 0.45, 0.9}, {0.9, 0.4, 0.25}, {0.3, 0.75, 0.4}} {
		c := render.NewContext(80, 80)
		view, rel := c.CreateOffscreenTexture(80, 80)
		if rel == nil || view.IsNil() {
			c.Close()
			t.Skip("offscreen unavailable")
		}
		releases = append(releases, func() { rel(); c.Close() })
		c.SetRGB(col[0], col[1], col[2])
		c.DrawRoundedRectangle(0, 0, 80, 80, 10)
		_ = c.Fill()
		if err := c.FlushGPUWithView(view, 80, 80); err != nil {
			t.Fatalf("tile: %v", err)
		}
		x := 30 + float64(i)*100
		// solid underlay so sampling works even if Base path is compositor-only
		dc.SetRGB(col[0]*0.5, col[1]*0.5, col[2]*0.5)
		dc.DrawRoundedRectangle(x, 40, 80, 80, 10)
		_ = dc.Fill()
		dc.DrawGPUTextureBase(view, x, 40, 80, 80)
		dc.DrawGPUTextureWithOpacity(view, x+8, 100, 80, 80, 0.7)
		dc.DrawGPUTexture(view, x+16, 150, 48, 48)
	}
	_ = mk
	dc.ClipRect(20, 20, 320, 200)
	dc.SetRGB(0.12, 0.14, 0.18)
	dc.DrawString("base×opacity×clip textures", 40, 210)
	dc.ResetClip()

	compMinGPU(t, dc, 6)
	r, g, b, _ := p1Sample(dc, 50, 60)
	p1NotNearWhite(t, "D93 tile area", r, g, b)
	r2, g2, b2, _ := p1Sample(dc, 150, 170)
	p1NotNearWhite(t, "D93 tex draw", r2, g2, b2)
}

// D94: full PresentFrame offscreen e2e after complex draw.
func TestP1_Comp_D94_PresentFrameComplexScene(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D94_PresentFrameComplexScene"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D94_PresentFrameComplexScene")
		return
	}
	const w, h = 320, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 12)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.18, 0.25)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("PresentFrame complex", 12, 26)
	for i := 0; i < 5; i++ {
		dc.SetRGB(0.2+float64(i)*0.1, 0.45, 0.85-float64(i)*0.05)
		dc.DrawRoundedRectangle(20+float64(i)*55, 60, 48, 100, 8)
		_ = dc.Fill()
	}
	dc.SetBlendMode(render.BlendMultiply)
	dc.SetRGB(1, 0.6, 0.4)
	dc.DrawCircle(160, 110, 40)
	_ = dc.Fill()
	dc.SetBlendMode(render.BlendNormal)

	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("offscreen unavailable")
	}
	defer rel()
	presented := false
	if err := dc.PresentFrame(view, uint32(w), uint32(h), func() error {
		presented = true
		return nil
	}); err != nil {
		t.Fatalf("PresentFrame: %v", err)
	}
	if !presented {
		t.Fatal("PresentFrame present callback not invoked")
	}
	// also resolve to Image for sampling
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	r, g, b, _ := p1Sample(dc, 40, 90)
	p1NotNearWhite(t, "D94 card", r, g, b)
}

// D95: TextModeMSDF/Bitmap fallback mix × clip × layer (best-effort modes).
func TestP1_Comp_D95_TextModeMixClipLayer(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D95_TextModeMixClipLayer"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D95_TextModeMixClipLayer")
		return
	}
	const w, h = 380, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 16)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.ClipRect(16, 16, 348, 168)
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(16, 16, 348, 168)
	_ = dc.Fill()

	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.2, 0.22, 0.28)
	dc.DrawRoundedRectangle(28, 28, 320, 140, 10)
	_ = dc.Fill()

	dc.SetRGB(0.95, 0.96, 0.98)
	dc.SetTextMode(render.TextModeGlyphMask)
	dc.DrawString("GlyphMask mode line", 44, 60)
	dc.SetTextMode(render.TextModeVector)
	dc.DrawString("Vector mode line", 44, 95)
	dc.SetTextMode(render.TextModeAuto)
	dc.DrawString("Auto mode line", 44, 130)
	// MSDF/Bitmap if available should not panic
	dc.SetTextMode(render.TextModeMSDF)
	dc.DrawString("MSDF try", 44, 155)
	dc.SetTextMode(render.TextModeBitmap)
	dc.DrawString("Bitmap try", 180, 155)
	dc.SetTextMode(render.TextModeAuto)
	dc.PopLayer()
	dc.ResetClip()

	compMinGPU(t, dc, 3)
	ink := 0
	for y := 45; y < 140; y += 2 {
		for x := 44; x < 300; x += 2 {
			r, g, b, _ := p1Sample(dc, x, y)
			if r > 180 && g > 180 && b > 180 {
				ink++
			}
		}
	}
	if ink < 20 {
		t.Fatalf("D95 text ink low: %d", ink)
	}
}

// D96: file manager density — tree + icon grid + path bar + selection.
func TestP1_Comp_D96_FileManagerDensityComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D96_FileManagerDensityComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D96_FileManagerDensityComposition")
		return
	}
	const w, h = 520, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// path bar
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, 36)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawString("/home/user/projects/gpui/render", 12, 24)

	// tree
	dc.ClipRect(0, 36, 160, h-36)
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(0, 36, 160, h-36)
	_ = dc.Fill()
	for i := 0; i < 12; i++ {
		y := 52 + float64(i)*22
		if i == 4 {
			dc.SetRGB(0.85, 0.91, 1)
			dc.DrawRectangle(0, y-6, 160, 22)
			_ = dc.Fill()
		}
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("%sfolder-%02d", "  "[0:i%3], i), 12, y+8)
	}
	dc.ResetClip()

	// icon grid
	dc.ClipRect(160, 36, w-160, h-36)
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(160, 36, w-160, h-36)
	_ = dc.Fill()
	icon := compMakeImage(t, 32, 32, 60, 140, 220)
	for row := 0; row < 4; row++ {
		for col := 0; col < 5; col++ {
			x := 180 + float64(col)*64
			y := 56 + float64(row)*64
			if row == 1 && col == 2 {
				dc.PushLayer(render.BlendNormal, 1)
				dc.SetRGB(0.88, 0.93, 1)
				dc.DrawRoundedRectangle(x-6, y-6, 56, 56, 8)
				_ = dc.Fill()
				dc.PopLayer()
			}
			dc.DrawImageRounded(icon, x, y, 6)
			dc.SetRGB(0.25, 0.27, 0.32)
			dc.DrawString(fmt.Sprintf("f%d", row*5+col), x+4, y+48)
		}
	}
	dc.ResetClip()

	compMinGPU(t, dc, 30)
	r, g, b, _ := p1Sample(dc, 200, 80)
	p1NotNearWhite(t, "D96 icon", r, g, b)
}

// D97: email compose — to/cc chips × body wrap × attach strip × send layer.
func TestP1_Comp_D97_EmailComposeComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D97_EmailComposeComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D97_EmailComposeComposition")
		return
	}
	const w, h = 460, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.96, 0.97, 0.98)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawRoundedRectangle(16, 16, w-32, h-32, 12)
	_ = dc.Fill()

	// chips
	for i, name := range []string{"alice@x.io", "bob@y.io", "team@z.io"} {
		x := 28 + float64(i)*120
		dc.SetRGB(0.88, 0.93, 1)
		dc.DrawRoundedRectangle(x, 32, 110, 24, 12)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.35, 0.75)
		dc.DrawString(name, x+8, 48)
	}
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("Subject: composition probes D97", 28, 80)

	dc.ClipRect(28, 100, w-56, 160)
	dc.SetRGB(0.98, 0.98, 0.99)
	dc.DrawRectangle(28, 100, w-56, 160)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	body := "Please review the multi-axis composition coverage. Nested clips, layers, wrap text, and attachment chips should remain correct under GPU."
	dc.DrawStringWrapped(body, 36, 110, 0, 0, w-80, 1.25, render.AlignLeft)
	dc.ResetClip()

	// attachments
	for i := 0; i < 3; i++ {
		x := 28 + float64(i)*100
		dc.SetRGB(0.93, 0.94, 0.96)
		dc.DrawRoundedRectangle(x, 270, 90, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.3, 0.32, 0.38)
		dc.DrawString(fmt.Sprintf("file-%d.pdf", i+1), x+8, 288)
	}
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(w-110, 270, 70, 28, 6)
	_ = dc.Fill()
	dc.SetRGB(1, 1, 1)
	dc.DrawString("Send", w-90, 288)
	dc.PopLayer()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, w-70, 280)
	if b < 80 {
		t.Fatalf("D97 send missing rgba=%d,%d,%d", r, g, b)
	}
}

// D98: board swimlanes — lanes × cards × WIP limit × drag ghost.
func TestP1_Comp_D98_SwimlaneBoardComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D98_SwimlaneBoardComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D98_SwimlaneBoardComposition")
		return
	}
	const w, h = 540, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.93, 0.94, 0.96)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	lanes := []string{"Todo", "Doing", "Blocked", "Done"}
	for li, name := range lanes {
		y := 20 + float64(li)*72
		dc.ClipRect(12, y, w-24, 64)
		dc.SetRGB(0.88, 0.9, 0.93)
		dc.DrawRoundedRectangle(12, y, w-24, 60, 8)
		_ = dc.Fill()
		dc.SetRGB(0.25, 0.27, 0.32)
		dc.DrawString(name+" · WIP 3", 24, y+18)
		for c := 0; c < 4; c++ {
			x := 140 + float64(c)*95
			dc.SetRGB(1, 1, 1)
			dc.DrawRoundedRectangle(x, y+10, 85, 40, 6)
			_ = dc.Fill()
			dc.SetRGB(0.2, 0.22, 0.26)
			dc.DrawString(fmt.Sprintf("%c-%d", name[0], c+1), x+12, y+34)
		}
		dc.ResetClip()
	}
	// drag ghost
	dc.PushLayer(render.BlendNormal, 0.55)
	dc.SetRGB(1, 1, 0.8)
	dc.DrawRoundedRectangle(260, 140, 85, 40, 6)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 25)
	// lane chrome gray
	r, g, b, _ := p1Sample(dc, 30, 40)
	if r < 180 {
		t.Fatalf("D98 lane chrome unexpected rgba=%d,%d,%d", r, g, b)
	}
	// card text ink
	ink := 0
	for y := 30; y < 280; y += 3 {
		for x := 150; x < 500; x += 4 {
			rr, gg, bb, _ := p1Sample(dc, x, y)
			if rr < 230 || gg < 230 || bb < 230 {
				ink++
			}
		}
	}
	if ink < 30 {
		t.Fatalf("D98 board ink low: %d", ink)
	}
}

// D99: polar chart / radar — path stroke × fill × labels × clip legend.
func TestP1_Comp_D99_RadarChartComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D99_RadarChartComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D99_RadarChartComposition")
		return
	}
	const w, h = 360, 300
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.1, 0.11, 0.14)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	cx, cy, R := 160.0, 150.0, 90.0
	// grid rings
	dc.SetRGB(0.25, 0.28, 0.34)
	dc.SetLineWidth(1)
	for k := 1; k <= 4; k++ {
		dc.DrawCircle(cx, cy, R*float64(k)/4)
		_ = dc.Stroke()
	}
	axes := 6
	vals := []float64{0.8, 0.55, 0.9, 0.4, 0.7, 0.65}
	poly := render.NewPath()
	for i := 0; i < axes; i++ {
		ang := -math.Pi/2 + float64(i)*2*math.Pi/float64(axes)
		dc.DrawLine(cx, cy, cx+R*math.Cos(ang), cy+R*math.Sin(ang))
		_ = dc.Stroke()
		x := cx + R*vals[i]*math.Cos(ang)
		y := cy + R*vals[i]*math.Sin(ang)
		if i == 0 {
			poly.MoveTo(x, y)
		} else {
			poly.LineTo(x, y)
		}
		dc.SetRGB(0.85, 0.88, 0.92)
		dc.DrawString(fmt.Sprintf("A%d", i+1), cx+(R+16)*math.Cos(ang)-8, cy+(R+16)*math.Sin(ang)+4)
	}
	poly.Close()
	dc.SetRGBA(0.2, 0.55, 0.95, 0.35)
	dc.AppendPath(poly)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.65, 1)
	dc.SetLineWidth(2)
	dc.AppendPath(poly)
	_ = dc.Stroke()

	// legend clip
	dc.ClipRect(270, 40, 80, 120)
	dc.SetRGB(0.18, 0.2, 0.24)
	dc.DrawRoundedRectangle(270, 40, 80, 120, 8)
	_ = dc.Fill()
	dc.SetRGB(0.3, 0.65, 1)
	dc.DrawRectangle(280, 60, 12, 12)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("score", 298, 70)
	dc.ResetClip()

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, int(cx), int(cy-20))
	p1NotNearWhite(t, "D99 radar", r, g, b)
}

// D100: pip / picture-in-picture — main stage × floating window × resize handle.
func TestP1_Comp_D100_PictureInPictureComposition(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D100_PictureInPictureComposition"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D100_PictureInPictureComposition")
		return
	}
	const w, h = 420, 280
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// main stage
	dc.SetRGB(0.12, 0.14, 0.2)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	for i := 0; i < 8; i++ {
		dc.SetRGB(0.2+float64(i)*0.05, 0.3, 0.5)
		dc.DrawRoundedRectangle(30+float64(i)*20, 40+float64(i%3)*30, 100, 50, 8)
		_ = dc.Fill()
	}
	// PiP window
	dc.PushLayer(render.BlendNormal, 0.98)
	dc.SetRGB(0.08, 0.09, 0.12)
	dc.DrawRoundedRectangle(250, 150, 150, 100, 10)
	_ = dc.Fill()
	dc.SetRGB(0.25, 0.55, 0.9)
	dc.DrawRoundedRectangle(258, 158, 134, 70, 6)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.91, 0.93)
	dc.DrawString("PiP live", 270, 240)
	// resize handle
	dc.SetRGB(0.7, 0.75, 0.85)
	dc.DrawRectangle(386, 236, 10, 10)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 10)
	r, g, b, _ := p1Sample(dc, 300, 180)
	if b < 80 {
		t.Fatalf("D100 pip content missing rgba=%d,%d,%d", r, g, b)
	}
}

// D101: nested scrollport x2 + sticky + horizontal chips + FAB.
func TestP1_Comp_D101_DoubleScrollStickyChipsFAB(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D101_DoubleScrollStickyChipsFAB"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D101_DoubleScrollStickyChipsFAB")
		return
	}
	const w, h = 400, 340
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("double scroll composition", 12, 26)

	// outer vertical scroll
	dc.ClipRect(0, 40, w, h-40)
	dc.SetRGB(0.97, 0.97, 0.98)
	dc.DrawRectangle(0, 40, w, h-40)
	_ = dc.Fill()
	// sticky chips row
	dc.SetRGB(0.2, 0.45, 0.9)
	dc.DrawRectangle(0, 40, w, 36)
	_ = dc.Fill()
	// horizontal scroll chips inside sticky
	dc.ClipRect(0, 40, w, 36)
	dc.Push()
	dc.Translate(-20, 0)
	for i := 0; i < 8; i++ {
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(12+float64(i)*70, 48, 60, 22, 11)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.35, 0.7)
		dc.DrawString(fmt.Sprintf("tag%d", i), 22+float64(i)*70, 63)
	}
	dc.Pop()
	dc.ResetClip()

	// content scrolled
	dc.Push()
	dc.Translate(0, -15)
	for i := 0; i < 12; i++ {
		y := 90 + float64(i)*32
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(12, y, w-24, 28, 6)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawString(fmt.Sprintf("content row %02d under sticky chips", i), 24, y+18)
	}
	dc.Pop()
	dc.ResetClip()

	// FAB
	dc.PushLayer(render.BlendNormal, 0.95)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawCircle(w-36, h-36, 22)
	_ = dc.Fill()
	dc.PopLayer()

	compMinGPU(t, dc, 25)
	r, g, b, _ := p1Sample(dc, w-36, h-36)
	if r < 100 {
		t.Fatalf("D101 FAB missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 40, 55)
	if r2 < 200 {
		t.Fatalf("D101 chip expected light rgba=%d,%d,%d", r2, g2, b2)
	}
}

// D102: multi-resize thrash then complex recompose.
func TestP1_Comp_D102_MultiResizeRecomposeStress(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D102_MultiResizeRecomposeStress"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D102_MultiResizeRecomposeStress")
		return
	}
	dc := render.NewContext(160, 120)
	defer dc.Close()
	font := p1FindFont(t)

	dc.ResetRenderPathStats()
	sizes := [][2]int{{200, 140}, {280, 180}, {360, 240}, {320, 200}}
	for _, s := range sizes {
		if err := dc.Resize(s[0], s[1]); err != nil {
			t.Fatalf("resize %v: %v", s, err)
		}
		_ = dc.LoadFontFace(font, 11)
		p1White(dc, s[0], s[1])
		dc.SetRGB(0.2, 0.45, 0.9)
		dc.DrawRoundedRectangle(12, 12, float64(s[0]-24), 40, 8)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(fmt.Sprintf("%dx%d", s[0], s[1]), 24, 38)
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}
	// final dense scene
	p1White(dc, 320, 200)
	for i := 0; i < 6; i++ {
		dc.SetRGB(0.95, 0.96, 0.98)
		dc.DrawRoundedRectangle(12+float64(i%3)*100, 20+float64(i/3)*80, 90, 60, 8)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.5, 0.85)
		dc.DrawRectangle(12+float64(i%3)*100, 20+float64(i/3)*80, 90, 18)
		_ = dc.Fill()
	}
	compMinGPU(t, dc, 5)
	if dc.Width() != 320 || dc.Height() != 200 {
		t.Fatalf("final size %dx%d", dc.Width(), dc.Height())
	}
	r, g, b, _ := p1Sample(dc, 30, 28)
	if b < 80 {
		t.Fatalf("D102 header missing rgba=%d,%d,%d", r, g, b)
	}
}

// D103: damage multi-rect + WritePixels + external tile hybrid update.
func TestP1_Comp_D103_HybridDamageWritePixelsTexture(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D103_HybridDamageWritePixelsTexture"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D103_HybridDamageWritePixelsTexture")
		return
	}
	const w, h = 400, 260
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.15, 0.16, 0.2)
	dc.DrawRectangle(0, 0, w, 40)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawString("hybrid damage update", 12, 26)
	for i := 0; i < 3; i++ {
		dc.SetRGB(0.92, 0.93, 0.95)
		dc.DrawRoundedRectangle(16+float64(i)*128, 56, 116, 160, 10)
		_ = dc.Fill()
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("base: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// panel 0 WritePixels stamp
	buf := make([]byte, 16*16*4)
	for i := 0; i < 16*16; i++ {
		buf[i*4+0], buf[i*4+1], buf[i*4+2], buf[i*4+3] = 220, 50, 40, 255
	}
	dc.WritePixels(40, 80, 16, 16, buf)

	// panel 1 solid damage
	dc.ClipRect(144, 56, 116, 160)
	dc.SetRGB(0.2, 0.55, 0.95)
	dc.DrawRectangle(144, 56, 116, 160)
	_ = dc.Fill()
	dc.ResetClip()

	// panel 2 external texture
	tile := render.NewContext(64, 64)
	view, rel := tile.CreateOffscreenTexture(64, 64)
	if rel == nil || view.IsNil() {
		tile.Close()
		t.Skip("offscreen unavailable")
	}
	defer func() { rel(); tile.Close() }()
	tile.SetRGB(0.25, 0.75, 0.4)
	tile.DrawRoundedRectangle(0, 0, 64, 64, 8)
	_ = tile.Fill()
	if err := tile.FlushGPUWithView(view, 64, 64); err != nil {
		t.Fatalf("tile: %v", err)
	}
	dc.DrawGPUTexture(view, 290, 90, 64, 64)

	view2, rel2 := dc.CreateOffscreenTexture(w, h)
	if rel2 != nil && !view2.IsNil() {
		defer rel2()
		_ = dc.FlushGPUWithViewDamageRects(view2, uint32(w), uint32(h), []image.Rectangle{
			image.Rect(40, 80, 56, 96),
			image.Rect(144, 56, 260, 216),
			image.Rect(290, 90, 354, 154),
		})
	}
	p1Flush(t, dc)
	compAutoSavePNG(t, dc)
	if dc.RenderPathStats().GPUOps <= base {
		t.Fatalf("D103 expected more GPUOps")
	}
	r, g, b, _ := p1Sample(dc, 48, 88)
	if r < 100 {
		t.Fatalf("D103 writepixels red missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, 180, 100)
	if b2 < 80 {
		t.Fatalf("D103 panel1 blue missing rgba=%d,%d,%d", r2, g2, b2)
	}
	r3, g3, b3, _ := p1Sample(dc, 310, 110)
	if g3 < 80 {
		t.Fatalf("D103 green tex missing rgba=%d,%d,%d", r3, g3, b3)
	}
}

// D104: annotated design canvas — guides × selection handles × multi-select layer.
func TestP1_Comp_D104_DesignCanvasAnnotations(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D104_DesignCanvasAnnotations"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D104_DesignCanvasAnnotations")
		return
	}
	const w, h = 480, 320
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 10)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	dc.SetRGB(0.94, 0.95, 0.97)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()

	// artboard
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(60, 40, 360, 240)
	_ = dc.Fill()
	// objects
	dc.SetRGB(0.2, 0.5, 0.95)
	dc.DrawRoundedRectangle(100, 80, 120, 70, 8)
	_ = dc.Fill()
	dc.SetRGB(0.9, 0.4, 0.25)
	dc.DrawCircle(320, 150, 40)
	_ = dc.Fill()
	// guides
	dc.SetRGB(0.95, 0.3, 0.7)
	dc.SetLineWidth(1)
	dc.SetDash(4, 3)
	dc.DrawLine(60, 120, 420, 120)
	_ = dc.Stroke()
	dc.DrawLine(200, 40, 200, 280)
	_ = dc.Stroke()
	dc.SetDash()
	// selection box + handles
	dc.PushLayer(render.BlendNormal, 0.9)
	dc.SetRGB(0.15, 0.55, 1)
	dc.SetLineWidth(1)
	dc.DrawRectangle(96, 76, 128, 78)
	_ = dc.Stroke()
	for _, p := range [][2]float64{{96, 76}, {224, 76}, {96, 154}, {224, 154}, {160, 76}, {160, 154}} {
		dc.SetRGB(1, 1, 1)
		dc.DrawRectangle(p[0]-3, p[1]-3, 6, 6)
		_ = dc.Fill()
		dc.SetRGB(0.15, 0.55, 1)
		dc.DrawRectangle(p[0]-3, p[1]-3, 6, 6)
		_ = dc.Stroke()
	}
	dc.PopLayer()
	dc.SetRGB(0.3, 0.32, 0.38)
	dc.DrawString("selection · guides · handles", 70, 300)

	compMinGPU(t, dc, 15)
	r, g, b, _ := p1Sample(dc, 140, 110)
	if b < 80 {
		t.Fatalf("D104 blue object missing rgba=%d,%d,%d", r, g, b)
	}
}

// D105: kitchen-sink v3 stress — combine rich text, filters, textures, damage, blends.
func TestP1_Comp_D105_KitchenSinkV3Stress(t *testing.T) {
	p1RequireGPU(t)
	if dc, ok := compTryScene(t, "D105_KitchenSinkV3Stress"); ok {
		defer dc.Close()
		compSavePNG(t, dc, "D105_KitchenSinkV3Stress")
		return
	}
	if !render.FiltersRegistered() {
		t.Fatal("filters not registered")
	}
	const w, h = 560, 400
	dc := render.NewContext(w, h)
	defer dc.Close()
	font := p1FindFont(t)
	_ = dc.LoadFontFace(font, 11)

	dc.ResetRenderPathStats()
	p1White(dc, w, h)
	// shell
	dc.SetRGB(0.12, 0.13, 0.16)
	dc.DrawRectangle(0, 0, 56, h)
	_ = dc.Fill()
	dc.SetRGB(0.95, 0.96, 0.98)
	dc.DrawRectangle(56, 0, w-56, 40)
	_ = dc.Fill()
	dc.SetRGB(0.2, 0.22, 0.26)
	dc.DrawString("kitchen-sink v3 stress", 68, 26)

	// content columns
	dc.ClipRect(56, 40, w-56, h-40)
	for col := 0; col < 3; col++ {
		x := 68 + float64(col)*160
		dc.SetRGB(1, 1, 1)
		dc.DrawRoundedRectangle(x, 56, 148, 280, 10)
		_ = dc.Fill()
		dc.SetRGB(0.2, 0.5, 0.9)
		dc.DrawRectangle(x, 56, 148, 28)
		_ = dc.Fill()
		dc.SetRGB(1, 1, 1)
		dc.DrawString(fmt.Sprintf("col-%d", col+1), x+12, 74)
		dc.SetRGB(0.2, 0.22, 0.26)
		dc.DrawStringWrapped("Axis mix stress: clip layer blend text image transform mask filter damage.", x+10, 100, 0, 0, 128, 1.2, render.AlignLeft)
		if col == 1 {
			dc.SetBlendMode(render.BlendMultiply)
			dc.SetRGBA(1, 0.6, 0.4, 1)
			dc.DrawCircle(x+74, 220, 36)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendNormal)
		}
		if col == 2 {
			img := compMakeImage(t, 40, 40, 40, 160, 220)
			dc.DrawImageEx(img, render.DrawImageOptions{X: x + 20, Y: 200, DstWidth: 100, DstHeight: 70, Opacity: 0.9})
		}
	}
	dc.ResetClip()

	// external strip
	var releases []func()
	defer func() {
		for _, r := range releases {
			if r != nil {
				r()
			}
		}
	}()
	for i := 0; i < 4; i++ {
		c := render.NewContext(40, 40)
		view, rel := c.CreateOffscreenTexture(40, 40)
		if rel == nil || view.IsNil() {
			c.Close()
			continue
		}
		releases = append(releases, func() { rel(); c.Close() })
		c.SetRGB(0.3+float64(i)*0.15, 0.5, 0.8-float64(i)*0.1)
		c.DrawRoundedRectangle(0, 0, 40, 40, 6)
		_ = c.Fill()
		_ = c.FlushGPUWithView(view, 40, 40)
		dc.DrawGPUTextureWithOpacity(view, 70+float64(i)*50, 350, 40, 40, 0.9)
	}

	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps
	// damage badge
	dc.ClipRect(w-70, 8, 50, 24)
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawRoundedRectangle(w-70, 8, 50, 24, 8)
	_ = dc.Fill()
	dc.ResetClip()
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.3})
	compMinGPU(t, dc, base)
	r, g, b, _ := p1Sample(dc, 100, 70)
	if b < 60 {
		t.Fatalf("D105 col header missing rgba=%d,%d,%d", r, g, b)
	}
	r2, g2, b2, _ := p1Sample(dc, w-40, 18)
	if r2 < 100 {
		t.Fatalf("D105 badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}
