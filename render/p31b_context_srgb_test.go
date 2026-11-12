//go:build !nogpu

package render_test

import (
	"strings"
	"testing"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"

	_ "github.com/energye/gpui/render/gpu"
)

// P3.1b-2b: the public Context path must preserve the opt-in sRGB target
// semantics, not only the low-level GPURenderContext unit fixture.
//
// Contract: render.RGBA colors are sRGB-encoded. On an sRGB target the shader
// decodes them to linear light (encode_out) and the fixed-function hardware
// encodes the blend result back to sRGB. A single opaque fill is therefore an
// identity round-trip (128,64,26); the discriminator is a linear-light Plus
// blend: 0.5 + 0.25 in linear light = 141, whereas the Unorm target keeps the
// encoded-domain result 192.
func TestP31bContextSrgbOffscreen(t *testing.T) {
	// Single opaque fill: sRGB-encoded input round-trips through the sRGB
	// attachment unchanged. A raw 188 here would mean the shader skipped the
	// sRGB->linear decode and the target encoding double-applied.
	single, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8UnormSrgb, false)
	p31bCheckColor(t, "public sRGB single fill", single, [3]int{128, 64, 26})
	if stats.GPUOps != 1 || stats.CPUFallbackOps != 0 {
		t.Fatalf("public sRGB single fill stats=%+v, want GPUOps=1 CPUFallbackOps=0", stats)
	}

	// Linear-light Plus on the sRGB target: 0.5+0.25 in linear light -> 141.
	srgb, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8UnormSrgb, true)
	p31bCheckColor(t, "public sRGB linear Plus", srgb, [3]int{141, 89, 70})
	if stats.GPUOps != 2 || stats.CPUFallbackOps != 0 {
		t.Fatalf("public sRGB linear Plus stats=%+v, want GPUOps=2 CPUFallbackOps=0", stats)
	}
	if stats.LinearFixedFunctionFallbacks != 0 {
		t.Fatalf("public sRGB linear Plus counted %d fixed-function linear fallbacks, want 0 (hardware blends in linear light)",
			stats.LinearFixedFunctionFallbacks)
	}

	// Unorm control: the same draw stays in the encoded domain (192), proving
	// the public path selects the target format rather than always using one
	// global pipeline set. The linear request is a real degradation here and
	// must be visible in the telemetry.
	unorm, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8Unorm, true)
	p31bCheckColor(t, "public Unorm encoded Plus", unorm, [3]int{192, 128, 89})
	if stats.GPUOps != 2 || stats.CPUFallbackOps != 0 {
		t.Fatalf("public Unorm linear Plus stats=%+v, want GPUOps=2 CPUFallbackOps=0", stats)
	}
}

// p31bContextReadCenter runs the public Context path (queue Fill(s), then
// bind an offscreen target at Flush) and returns the center RGBA bytes.
func p31bContextReadCenter(t *testing.T, format types.TextureFormat, plus bool) ([4]byte, render.RenderPathStats) {
	t.Helper()
	dc := render.NewContext(64, 64)
	defer dc.Close()
	// The fallback counter is process-wide; tests in this package share the
	// device. Materialize the GPU context and measure this leg as a delta so
	// earlier tests cannot poison it.
	_ = dc.GPURenderContext()
	lfBefore := dc.RenderPathStats().LinearFixedFunctionFallbacks
	dc.SetRGBA(0.5, 0.25, 0.1, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill base (%v): %v", format, err)
	}
	if plus {
		dc.SetRGBA(0.25, 0.25, 0.25, 1)
		dc.SetBlendMode(render.BlendPlus)
		dc.SetBlendSpace(render.BlendSpaceLinear)
		dc.DrawRectangle(0, 0, 64, 64)
		if err := dc.Fill(); err != nil {
			t.Fatalf("Fill plus (%v): %v", format, err)
		}
	}
	stats := dc.RenderPathStats()
	view, release, err := dc.CreateOffscreenTextureFormat(64, 64, format)
	if err != nil {
		t.Fatalf("CreateOffscreenTextureFormat(%v): %v", format, err)
	}
	defer release()
	if err := dc.FlushGPUWithView(view, 64, 64); err != nil {
		t.Fatalf("FlushGPUWithView(%v): %v", format, err)
	}
	// The degradation counter is resolved at Flush time against the real
	// target format, so read the stats after the flush.
	stats = dc.RenderPathStats()
	stats.LinearFixedFunctionFallbacks -= lfBefore
	t.Logf("P3.1b context leg format=%v plus=%v lfBefore=%d stats=%+v", format, plus, lfBefore, stats)
	rb, ok := dc.GPURenderContext().(interface {
		ReadbackViewStraightRGBA(view gpucontext.TextureView, w, h int) ([]byte, error)
	})
	if !ok {
		t.Skip("ReadbackViewStraightRGBA unavailable")
	}
	rgba, err := rb.ReadbackViewStraightRGBA(view, 64, 64)
	if err != nil {
		t.Fatalf("readback (%v): %v", format, err)
	}
	center := (32*64 + 32) * 4
	var got [4]byte
	copy(got[:], rgba[center:center+4])
	return got, stats
}

// p31bCheckColor asserts an RGB triple within the P3.1b ±3/255 tolerance.
func p31bCheckColor(t *testing.T, name string, got [4]byte, want [3]int) {
	t.Helper()
	for i := 0; i < 3; i++ {
		if d := int(got[i]) - want[i]; d < -3 || d > 3 {
			t.Fatalf("%s center=%v, want %v±3", name, got, want)
		}
	}
}

// TestP31bContextSrgbCPUFallbackPolicy pins the linear-target CPU fallback
// policy: an opt-in sRGB target is a GPU-only surface. With no GPU context
// (GOGPU_RENDER_MODE=cpu / WithCPUOnly) the target creation must fail closed
// with the explicit "GPU context unavailable" error instead of handing back a
// target that no CPU pass can populate (silent black = fake green).
func TestP31bContextSrgbCPUFallbackPolicy(t *testing.T) {
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := render.NewContext(64, 64)
	defer dc.Close()
	dc.SetRGBA(0.5, 0.25, 0.1, 1)
	dc.DrawRectangle(0, 0, 64, 64)
	if err := dc.Fill(); err != nil {
		t.Fatalf("Fill: %v", err)
	}
	view, release, err := dc.CreateOffscreenTextureFormat(64, 64, types.TextureFormatRGBA8UnormSrgb)
	if err == nil {
		if release != nil {
			release()
		}
		t.Fatalf("cpu-only CreateOffscreenTextureFormat returned view=%v, want fail-closed error", view)
	}
	if !strings.Contains(err.Error(), "GPU context unavailable") {
		t.Fatalf("cpu-only sRGB target error = %q, want GPU context unavailable", err)
	}
}
