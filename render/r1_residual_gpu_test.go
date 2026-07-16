//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// Round-1 residual CPU→GPU fixes (docs/GPU_FIRST_ROUTING.md §7.1).

func TestR1_PushMaskLayerGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 48, 48
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	mask := render.NewMask(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x < w/2 {
				mask.Set(x, y, 255)
			} else {
				mask.Set(x, y, 0)
			}
		}
	}
	dc.PushMaskLayer(mask)
	dc.SetRGB(1, 0, 0)
	dc.DrawRectangle(0, 0, w, h)
	_ = dc.Fill()
	dc.SetRGB(0, 0, 1)
	dc.DrawCircle(24, 24, 14)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R1 mask %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("expected GPUOps>0 for mask-layer content")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	lr, lg, lb, _ := sampleRGBA(dc, 10, 24)
	rr, rg, rb, _ := sampleRGBA(dc, 40, 24)
	t.Logf("left=%d,%d,%d right=%d,%d,%d", lr, lg, lb, rr, rg, rb)
	if lr > 240 && lg > 240 && lb > 240 {
		t.Fatalf("left should show masked content")
	}
	if rr < 240 || rg < 240 || rb < 240 {
		t.Fatalf("right should stay white, got %d,%d,%d", rr, rg, rb)
	}
}

func TestR1_BackdropSeedGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.15, 0.35, 0.9)
	dc.DrawRectangle(8, 8, 48, 48)
	_ = dc.Fill()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("parent flush: %v", err)
	}
	base := dc.RenderPathStats().GPUOps

	// Backdrop + partial dim (not full cover) so snapshot seed is required.
	dc.PushBackdropLayer(render.BlendNormal, 1)
	dc.SetRGBA(0, 0, 0, 0.5)
	dc.DrawRectangle(16, 16, 32, 32)
	_ = dc.Fill()
	dc.PopLayer()
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("backdrop flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R1 backdrop %s base=%d", st.LogLine(), base)
	if st.GPUOps <= base {
		t.Fatalf("expected additional GPUOps after backdrop draw")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	// Center of dimmed region should keep some blue from seeded backdrop.
	r, g, b, _ := sampleRGBA(dc, 32, 32)
	t.Logf("center=%d,%d,%d", r, g, b)
	if b < 40 {
		t.Fatalf("lost backdrop blue: %d,%d,%d", r, g, b)
	}
	// Outside dim rect but inside blue parent should stay bright blue-ish.
	or, og, ob, _ := sampleRGBA(dc, 12, 12)
	t.Logf("outer blue=%d,%d,%d", or, og, ob)
	if ob < 100 {
		t.Fatalf("outer blue destroyed: %d,%d,%d", or, og, ob)
	}
}

func TestR1_ImageMipmapUsesGPU(t *testing.T) {
	requireNativeGPU(t)
	const w, h = 64, 64
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	img, err := render.NewImageBuf(32, 32, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// Fill red via raw data if available.
	data := img.Data()
	for i := 0; i+3 < len(data); i += 4 {
		data[i+0] = 255
		data[i+1] = 0
		data[i+2] = 0
		data[i+3] = 255
	}
	img.NotifyPixelsChanged()

	dc.DrawImageEx(img, render.DrawImageOptions{
		X: 8, Y: 8, DstWidth: 16, DstHeight: 16,
		Interpolation: render.InterpBilinear,
		UseMipmaps:    true,
		Opacity:       1,
		BlendMode:     render.BlendNormal,
	})
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("FlushGPU: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R1 mipmap %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("UseMipmaps should use GPU bilinear path")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s (mipmap must not force CPU)", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
}

func TestR2_GradientStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(128, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)

	grad := render.NewLinearGradientBrush(0, 0, 128, 0).
		AddColorStop(0, render.RGB(1, 0, 0)).
		AddColorStop(1, render.RGB(0, 0, 1))
	dc.SetStrokeBrush(grad)
	dc.SetLineWidth(8)
	dc.MoveTo(10, 32)
	dc.LineTo(118, 32)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R2 grad stroke %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("gradient stroke must produce GPUOps")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
	// Mid-line should not be pure white.
	r, g, b, _ := sampleRGBA(dc, 64, 32)
	t.Logf("mid=%d,%d,%d", r, g, b)
	if r > 250 && g > 250 && b > 250 {
		t.Fatalf("expected inked gradient stroke")
	}
}

func TestR3_DashedCircleStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(80, 80)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0.1, 0.2, 0.9)
	dc.SetLineWidth(3)
	dc.SetDash(6, 4)
	dc.DrawCircle(40, 40, 28)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R3 dash circle %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("dashed circle stroke must be GPU")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
}

func TestR3_ThinStrokeGPU(t *testing.T) {
	requireNativeGPU(t)
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.ClearWithColor(render.White)
	dc.SetRGB(0, 0, 0)
	dc.SetLineWidth(1) // thin: was hard CPU via StrokeShape SDF reject
	dc.DrawRectangle(8, 8, 48, 48)
	if err := dc.Stroke(); err != nil {
		t.Fatalf("stroke: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	st := dc.RenderPathStats()
	t.Logf("R3 thin %s", st.LogLine())
	if st.GPUOps == 0 {
		t.Fatalf("thin stroke must be GPU via expand")
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("cpu_fb=%d last=%s", st.CPUFallbackOps, st.LastCPUFallbackReason)
	}
}
