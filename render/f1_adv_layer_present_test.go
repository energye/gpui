package render_test

import (
	"os"
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// F1: advanced PushLayer must composite on Present-style FlushGPUWithView.
func TestF1_AdvancedLayerPresentViewMultiply(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture unavailable")
	}
	defer release()

	dc.BeginFrame()
	dc.SetRGB(1, 1, 1)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()

	dc.PushLayer(render.BlendMultiply, 1.0)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 48, 48)
	_ = dc.Fill()
	dc.PopLayer()

	if err := dc.FlushGPUWithView(view, uint32(w), uint32(h)); err != nil {
		t.Fatalf("FlushGPUWithView: %v", err)
	}

	type readbacker interface {
		ReadbackViewRGBA(view gpucontext.TextureView, ww, hh int) ([]byte, error)
	}
	raw := dc.GPURenderContext()
	rb, ok := raw.(readbacker)
	if !ok || rb == nil {
		t.Fatal("ReadbackViewRGBA unavailable")
	}
	rgba, err := rb.ReadbackViewRGBA(view, w, h)
	if err != nil || len(rgba) < w*h*4 {
		t.Fatalf("readback: err=%v len=%d", err, len(rgba))
	}
	i := (32*w + 32) * 4
	r, g, b, a := rgba[i], rgba[i+1], rgba[i+2], rgba[i+3]
	t.Logf("center rgba=%d,%d,%d,%d stats=%s", r, g, b, a, dc.RenderPathStats().LogLine())
	// Multiply red over white ≈ red
	if r < 100 {
		t.Fatalf("expected red-ish after multiply layer present, got %d,%d,%d,%d", r, g, b, a)
	}
	if g > 60 || b > 60 {
		t.Fatalf("unexpected green/blue: %d,%d,%d,%d", r, g, b, a)
	}
}

func TestF1_AdvancedLayerPresentViewScreen(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()

	view, release := dc.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture unavailable")
	}
	defer release()

	dc.BeginFrame()
	dc.SetRGB(0, 0, 0)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()

	dc.PushLayer(render.BlendScreen, 1.0)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(8, 8, 48, 48)
	_ = dc.Fill()
	dc.PopLayer()

	if err := dc.FlushGPUWithView(view, uint32(w), uint32(h)); err != nil {
		t.Fatalf("FlushGPUWithView: %v", err)
	}

	type readbacker interface {
		ReadbackViewRGBA(view gpucontext.TextureView, ww, hh int) ([]byte, error)
	}
	rb, ok := dc.GPURenderContext().(readbacker)
	if !ok || rb == nil {
		t.Fatal("ReadbackViewRGBA unavailable")
	}
	rgba, err := rb.ReadbackViewRGBA(view, w, h)
	if err != nil || len(rgba) < w*h*4 {
		t.Fatalf("readback: err=%v len=%d", err, len(rgba))
	}
	i := (32*w + 32) * 4
	r, g, b, a := rgba[i], rgba[i+1], rgba[i+2], rgba[i+3]
	t.Logf("screen center rgba=%d,%d,%d,%d", r, g, b, a)
	if r < 200 || g > 40 || b > 40 {
		t.Fatalf("expected red screen result, got %d,%d,%d,%d", r, g, b, a)
	}
}

// HUD/FPS text drawn after advanced PopLayer must survive present-path
// frameScratch encode → dual-tex resolve → surface blit (LoadOpLoad).
func TestF1_AdvancedLayerPresentView_HUDText(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 200, 120
	dc := render.NewContext(w, h)
	defer dc.Close()

	font := firstExistingFont(
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
	)
	if font == "" {
		t.Skip("no latin font")
	}
	if err := dc.LoadFontFace(font, 18); err != nil {
		t.Fatalf("LoadFontFace: %v", err)
	}

	view, release := dc.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		t.Fatal("CreateOffscreenTexture unavailable")
	}
	defer release()

	dc.BeginFrame()
	dc.SetRGB(0.07, 0.08, 0.11)
	dc.DrawRectangle(0, 0, float64(w), float64(h))
	_ = dc.Fill()

	// Stage + clip like particle kitchen sink.
	dc.SetRGB(0.82, 0.84, 0.88)
	dc.DrawRectangle(20, 40, 160, 60)
	_ = dc.Fill()
	dc.Push()
	dc.ClipRect(20, 40, 160, 60)
	dc.PushLayer(render.BlendScreen, 1.0)
	dc.SetRGBA(1, 0.15, 0.15, 0.9)
	dc.DrawCircle(80, 70, 22)
	_ = dc.Fill()
	dc.PopLayer()
	dc.PushLayer(render.BlendMultiply, 1.0)
	dc.SetRGBA(0.15, 1, 0.2, 0.85)
	dc.DrawCircle(120, 70, 22)
	_ = dc.Fill()
	dc.PopLayer()
	dc.Pop()

	// HUD bar + text outside stage clip (FPS region).
	dc.SetRGBA(0.05, 0.06, 0.09, 0.9)
	dc.DrawRectangle(0, 0, float64(w), 28)
	_ = dc.Fill()
	dc.SetRGBA(0.95, 0.97, 1, 1)
	dc.DrawString("FPS 60.0 HUD", 8, 20)

	if err := dc.FlushGPUWithView(view, uint32(w), uint32(h)); err != nil {
		t.Fatalf("FlushGPUWithView: %v", err)
	}

	type readbacker interface {
		ReadbackViewRGBA(view gpucontext.TextureView, ww, hh int) ([]byte, error)
	}
	rb, ok := dc.GPURenderContext().(readbacker)
	if !ok || rb == nil {
		t.Fatal("ReadbackViewRGBA unavailable")
	}
	rgba, err := rb.ReadbackViewRGBA(view, w, h)
	if err != nil || len(rgba) < w*h*4 {
		t.Fatalf("readback: err=%v len=%d", err, len(rgba))
	}

	// HUD strip should contain bright glyph pixels, not only the dark bar.
	bright := 0
	maxL := 0
	for y := 4; y < 26; y++ {
		for x := 6; x < 160; x++ {
			i := (y*w + x) * 4
			l := (int(rgba[i]) + int(rgba[i+1]) + int(rgba[i+2])) / 3
			if l > maxL {
				maxL = l
			}
			if l > 160 {
				bright++
			}
		}
	}
	t.Logf("HUD bright=%d maxL=%d stats=%s", bright, maxL, dc.RenderPathStats().LogLine())
	if bright < 30 || maxL < 180 {
		t.Fatalf("HUD/FPS text missing after advanced present: bright=%d maxL=%d", bright, maxL)
	}
}

func firstExistingFont(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}
