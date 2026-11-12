//go:build linux && !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
	rendgpu "github.com/energye/gpui/render/gpu"
)

// P3.1b GL 腿：公开 Context 的 sRGB 目标语义必须与 native 逐值一致。
// gles 的 sRGB 附件只有在 GL_FRAMEBUFFER_SRGB 开启时才做线性混合/编码；
// 默认 Unorm 目标必须保持关闭（同绘制留在编码域）。
func TestP31bContextSrgbOffscreenGL(t *testing.T) {
	// Register the reset before setupGLDevice so it runs last (LIFO): the
	// temporary GL provider is abandoned first, then a fresh accelerator is
	// registered so later tests in the same process are not left without a
	// GPU context (K6 global-pollution class).
	t.Cleanup(func() { _ = rendgpu.ResetAccelerator() })
	if err := setupGLDevice(t); err != nil {
		t.Skipf("GL unavailable: %v", err)
	}

	single, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8UnormSrgb, false)
	p31bCheckColor(t, "GL sRGB single fill", single, [3]int{128, 64, 26})
	if stats.GPUOps != 1 || stats.CPUFallbackOps != 0 {
		t.Fatalf("GL sRGB single fill stats=%+v, want GPUOps=1 CPUFallbackOps=0", stats)
	}

	srgb, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8UnormSrgb, true)
	p31bCheckColor(t, "GL sRGB linear Plus", srgb, [3]int{141, 89, 70})
	if stats.GPUOps != 2 || stats.CPUFallbackOps != 0 {
		t.Fatalf("GL sRGB linear Plus stats=%+v, want GPUOps=2 CPUFallbackOps=0", stats)
	}
	if stats.LinearFixedFunctionFallbacks != 0 {
		t.Fatalf("GL sRGB linear Plus counted %d fixed-function linear fallbacks, want 0", stats.LinearFixedFunctionFallbacks)
	}

	unorm, stats := p31bContextReadCenter(t, types.TextureFormatRGBA8Unorm, true)
	p31bCheckColor(t, "GL Unorm encoded Plus", unorm, [3]int{192, 128, 89})
	if stats.GPUOps != 2 || stats.CPUFallbackOps != 0 {
		t.Fatalf("GL Unorm linear Plus stats=%+v, want GPUOps=2 CPUFallbackOps=0", stats)
	}
}
