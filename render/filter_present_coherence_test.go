package render_test

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// Regression: intermediate FlushGPU then more draws then mild blur must not wipe
// already-flushed content (D105/D140 class).
func TestFilterGraph_AfterMidFlushKeepsContent(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 160
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.2, 0.5, 0.9)
	dc.DrawRectangle(50, 40, 100, 24)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	dc.SetRGB(0.9, 0.25, 0.3)
	dc.DrawRectangle(w-50, 8, 40, 20)
	_ = dc.Fill()
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.3})
	r, g, b, a := p1Sample(dc, 70, 50)
	if a < 10 || b < 60 {
		t.Fatalf("header wiped after mid-flush+filter rgba=%d,%d,%d,%d", r, g, b, a)
	}
	r2, g2, b2, _ := p1Sample(dc, w-30, 18)
	if r2 < 100 {
		t.Fatalf("badge missing rgba=%d,%d,%d", r2, g2, b2)
	}
}

// Regression: draws after ApplyImageFilterGraph must appear in Image() (D133).
func TestFilterGraph_DrawAfterFilterVisible(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 200, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.2, 0.55, 0.9)
	dc.DrawRectangle(20, 20, 100, 60)
	_ = dc.Fill()
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.4})
	dc.SetRGB(0.98, 0.8, 0.1)
	dc.DrawRectangle(120, 70, 60, 30)
	_ = dc.Fill()
	r, g, b, a := p1Sample(dc, 140, 85)
	if a < 10 || !(r > 140 && g > 90 && b < 120) {
		t.Fatalf("post-filter yellow badge missing rgba=%d,%d,%d,%d", r, g, b, a)
	}
	r2, g2, b2, a2 := p1Sample(dc, 50, 40)
	if a2 < 10 || b2 < 40 {
		t.Fatalf("filtered body missing rgba=%d,%d,%d,%d", r2, g2, b2, a2)
	}
}

// Regression: PresentFrameDamageRects content must be visible via Image() (D152).
func TestPresentDamage_ImageMatchesView(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 200, 120
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRectangle(30, 30, 50, 30)
	_ = dc.Fill()
	view, rel := dc.CreateOffscreenTexture(w, h)
	if rel == nil || view.IsNil() {
		t.Skip("no offscreen")
	}
	defer rel()
	rects := []image.Rectangle{image.Rect(30, 30, 80, 60)}
	if err := dc.PresentFrameDamageRects(view, uint32(w), uint32(h), rects, func() error { return nil }); err != nil {
		t.Fatalf("present: %v", err)
	}
	r, g, b, a := p1Sample(dc, 50, 45)
	if r > 240 && g > 240 && b > 240 {
		t.Fatalf("Image() still white after present rgba=%d,%d,%d,%d", r, g, b, a)
	}
	if r < 100 || g > 200 {
		t.Fatalf("red damage missing rgba=%d,%d,%d,%d", r, g, b, a)
	}
}

// Regression: advanced blend (Multiply) mid-flushes pending into pixmap then
// queues more draws; ApplyImageFilterGraph must not drop the pre-blend surface
// (D140 kitchen sink lattice wipe).
func TestFilterGraph_MultiplyThenFilterKeepsContent(t *testing.T) {
	p1RequireGPU(t)
	const w, h = 280, 200
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ClearWithColor(render.White)
	dc.ClipRect(10, 10, w-20, h-20)
	for col := 0; col < 4; col++ {
		x := 40 + float64(col)*50
		dc.SetBlendMode(render.BlendNormal)
		dc.SetRGB(0.25+float64(col)*0.05, 0.35, 0.75)
		dc.DrawRoundedRectangle(x, 40, 42, 30, 5)
		_ = dc.Fill()
		if col%3 == 0 {
			dc.SetBlendMode(render.BlendMultiply)
			dc.SetRGBA(1, 0.7, 0.4, 1)
			dc.DrawCircle(x+21, 55, 10)
			_ = dc.Fill()
			dc.SetBlendMode(render.BlendNormal)
		}
	}
	dc.ResetClip()
	dc.SetRGB(0.9, 0.3, 0.35)
	dc.DrawRoundedRectangle(160, 90, 60, 24, 6)
	_ = dc.Fill()
	dc.ApplyImageFilterGraph(render.ImageFilterNode{Kind: render.ImageFilterBlur, Radius: 0.2})
	r, g, b, a := p1Sample(dc, 55, 50)
	if a < 10 || b < 40 {
		t.Fatalf("cell wiped after multiply+filter rgba=%d,%d,%d,%d", r, g, b, a)
	}
	r2, _, _, _ := p1Sample(dc, 180, 100)
	if r2 < 100 {
		t.Fatalf("post-multiply accent missing r=%d", r2)
	}
}
