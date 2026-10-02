//go:build !nogpu

package gpu

import (
	"strings"
	"testing"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

// The YUV convert shader must carry the same BT.601 numbers as the CPU
// fallback oracle (render nv12ToRGBA): 1.164/1.596/-0.391/-0.813/2.018
// with the 16/128 level offsets. One WGSL source serves both backends.
func TestVideoYUVShaderMatchesCPUOracle(t *testing.T) {
	if videoYUVShaderSource == "" {
		t.Fatal("video_yuv shader source is empty")
	}
	for _, want := range []string{"1.164", "1.596", "0.391", "0.813", "2.018", "16.0", "128.0"} {
		if !strings.Contains(videoYUVShaderSource, want) {
			t.Fatalf("video_yuv shader missing BT.601 constant %s", want)
		}
	}
	// Dual-texture binding shape (same as the production dual-tex blend
	// path): two texture_2d plus one sampler.
	for _, want := range []string{
		"y_texture: texture_2d<f32>",
		"uv_texture: texture_2d<f32>",
		"video_sampler: sampler",
	} {
		if !strings.Contains(videoYUVShaderSource, want) {
			t.Fatalf("video_yuv shader missing binding %s", want)
		}
	}
	// Same ortho vertex layout + clip group as textured quads.
	for _, want := range []string{"ImageUniforms", "clip: ClipParams", "vs_main", "fs_main"} {
		if !strings.Contains(videoYUVShaderSource, want) {
			t.Fatalf("video_yuv shader missing %s", want)
		}
	}
}

func TestQueueGPUVideoPlanesDraw_Enqueues(t *testing.T) {
	rc := &GPURenderContext{shared: NewGPUShared()}
	target := makeTestTarget(600, 400)
	y := gpucontext.NewTextureView(unsafe.Pointer(new(int)))
	uv := gpucontext.NewTextureView(unsafe.Pointer(new(int)))

	if !rc.QueueGPUVideoPlanesDraw(target, y, uv, 64, 36,
		0, 0, 32, 18, 1.0, 600, 400) {
		t.Fatal("QueueGPUVideoPlanesDraw = false, want true")
	}
	if len(rc.pendingGPUTextureCommands) != 1 {
		t.Fatalf("pending = %d, want 1", len(rc.pendingGPUTextureCommands))
	}
	cmd := rc.pendingGPUTextureCommands[0]
	if !cmd.IsYUV {
		t.Fatal("IsYUV = false, want true")
	}
	if cmd.View.IsNil() || cmd.UVView.IsNil() {
		t.Fatal("Y/UV views nil, want both set")
	}
	// Nil views and bad sizes fail closed without queueing.
	if rc.QueueGPUVideoPlanesDraw(target, gpucontext.TextureView{}, uv, 64, 36, 0, 0, 32, 18, 1.0, 600, 400) {
		t.Fatal("nil Y view = true, want false")
	}
	if rc.QueueGPUVideoPlanesDraw(target, y, uv, 0, 36, 0, 0, 32, 18, 1.0, 600, 400) {
		t.Fatal("zero width = true, want false")
	}
	var nilRC *GPURenderContext
	if nilRC.QueueGPUVideoPlanesDraw(target, y, uv, 64, 36, 0, 0, 32, 18, 1.0, 600, 400) {
		t.Fatal("nil context = true, want false")
	}
	if len(rc.pendingGPUTextureCommands) != 1 {
		t.Fatalf("pending after rejects = %d, want 1", len(rc.pendingGPUTextureCommands))
	}
}

func TestCanMergeGPUTextureDraw_YUVNeverMerges(t *testing.T) {
	yuv := &GPUTextureDrawCommand{Opacity: 1, ViewportWidth: 600, ViewportHeight: 400, IsYUV: true}
	lin := &GPUTextureDrawCommand{Opacity: 1, ViewportWidth: 600, ViewportHeight: 400}
	if canMergeGPUTextureDraw(yuv, yuv) {
		t.Fatal("YUV+YUV merges, want no merge (one draw per video quad)")
	}
	if canMergeGPUTextureDraw(yuv, lin) || canMergeGPUTextureDraw(lin, yuv) {
		t.Fatal("YUV+linear merges, want no merge (different pipelines)")
	}
}

// YUV pipeline variants compile on a real device; without one the test
// skips honestly (no fake green).
func TestVideoYUVPipelines_Compile(t *testing.T) {
	device, queue, cleanup := createNativeDevice(t)
	defer cleanup()
	p := NewTexturedQuadPipeline(device, queue, 1)
	defer p.Destroy()
	if err := p.ensureYUVPipelines(); err != nil {
		t.Fatalf("ensureYUVPipelines: %v", err)
	}
	if p.yuvPipeStencil == nil || p.yuvPipeDepthClip == nil {
		t.Fatal("YUV stencil/depth-clip pipelines nil after ensure")
	}
	if err := p.ensureYUVBlitPipeline(); err != nil {
		t.Fatalf("ensureYUVBlitPipeline: %v", err)
	}
	if p.yuvPipeBlit == nil {
		t.Fatal("YUV blit pipeline nil after ensure")
	}
	if got := p.yuvPipelineForDraw(false, false); got == nil {
		t.Fatal("yuvPipelineForDraw stencil = nil, want pipeline")
	}
	if got := p.yuvPipelineForDraw(false, true); got == nil {
		t.Fatal("yuvPipelineForDraw blit = nil, want pipeline")
	}
	// Without built variants the selector is nil: the caller keeps the
	// CPU fallback instead of silently dropping the frame.
	q := NewTexturedQuadPipeline(device, queue, 1)
	defer q.Destroy()
	if got := q.yuvPipelineForDraw(false, false); got != nil {
		t.Fatal("yuvPipelineForDraw before ensure != nil, want nil")
	}
}

// DrawVideoPlanes without a GPU session fails closed like DrawVideoSlot.
func TestDrawVideoPlanes_FailClosedWithoutGPU(t *testing.T) {
	dc := render.NewContext(64, 64)
	if dc.DrawVideoPlanes(nil, render.VideoDrawOptions{}) {
		t.Fatal("DrawVideoPlanes(nil) = true, want false")
	}
}
