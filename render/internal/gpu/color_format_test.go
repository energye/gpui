//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// M5 HDR 全链 P1（evidence/M5/assessment-design.md §5）：
// 管线颜色目标格式参数化。8bit 零值 = 历史 BGRA8（行为零变化）；
// float 目标把 owner 的 colorFormat 设成 RGBA16Float 后，同一批生产管线
// 必须能按该格式建出来（Skia/Impeller 的 pipeline 键含目标格式同形态）。
//
// 本钉只证明「建管线」这一层；16F 面的绘制/混合语义由 P0 底座探针
// （hdr_16f_probe_test.go）证明，端到端真窗绘制归 P4。
func TestP1ColorFormatParameterization(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()

	if got := colorFormatOr(types.TextureFormatUndefined); got != types.TextureFormatBGRA8Unorm {
		t.Fatalf("zero colorFormat = %v, want BGRA8 (8bit behavior must stay default)", got)
	}
	if got := colorFormatOr(types.TextureFormatRGBA16Float); got != types.TextureFormatRGBA16Float {
		t.Fatalf("explicit colorFormat = %v, want RGBA16Float", got)
	}

	// SDF: the shape fast path used by circles/rrects.
	sdf := NewSDFRenderPipeline(device, queue, 1)
	clip, _ := testSharedLayouts(t, device, sdf)
	sdf.colorFormat = types.TextureFormatRGBA16Float
	if err := sdf.createPipeline(); err != nil {
		t.Fatalf("SDF pipeline with RGBA16Float target: %v", err)
	}
	if sdf.pipeline == nil {
		t.Fatal("SDF pipeline with RGBA16Float target: nil pipeline")
	}
	sdf.Destroy()

	// Image quads: the textured-quad family (9 descriptors incl. blit/blend).
	img := NewTexturedQuadPipeline(device, queue, 1)
	img.colorFormat = types.TextureFormatRGBA16Float
	if err := img.ensurePipelineWithStencil(); err != nil {
		t.Fatalf("image pipeline with RGBA16Float target: %v", err)
	}
	if img.pipelineWithStencil == nil {
		t.Fatal("image pipeline with RGBA16Float target: nil pipeline")
	}
	img.Destroy()

	// MSDF text: the glyph path.
	txt := NewMSDFTextPipeline(device, queue, 1)
	txt.SetClipBindLayout(clip)
	txt.colorFormat = types.TextureFormatRGBA16Float
	if err := txt.ensurePipelineWithStencil(); err != nil {
		t.Fatalf("text pipeline with RGBA16Float target: %v", err)
	}
	if txt.pipelineWithStencil == nil {
		t.Fatal("text pipeline with RGBA16Float target: nil pipeline")
	}
	txt.Destroy()

	// Control leg: the zero value still builds the historical BGRA8 pipelines.
	ctrl := NewSDFRenderPipeline(device, queue, 1)
	testSharedLayouts(t, device, ctrl)
	if ctrl.colorFormat != types.TextureFormatUndefined {
		t.Fatalf("fresh pipeline colorFormat = %v, want zero (BGRA8 default)", ctrl.colorFormat)
	}
	if err := ctrl.createPipeline(); err != nil {
		t.Fatalf("SDF pipeline with default (BGRA8) target: %v", err)
	}
	ctrl.Destroy()
}
