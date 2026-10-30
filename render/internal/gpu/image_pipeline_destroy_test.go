//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// destroyPipeline is the single release point for every GPU object the
// textured-quad pipeline lazily creates. Any field the release path forgets
// leaks once per render.Context create/close — the conformance per-round VRAM
// delta (P2.4c K1) caught the bicubic pair exactly this way.
func TestTexturedQuadPipeline_DestroyReleasesAllOwnedResources(t *testing.T) {
	dev := newNoopTestDevice(t)
	p := NewTexturedQuadPipeline(dev, dev.Queue(), 1)

	if err := p.ensurePipelineWithStencil(); err != nil {
		t.Fatalf("ensurePipelineWithStencil: %v", err)
	}
	if err := p.ensureDepthClipPipeline(); err != nil {
		t.Fatalf("ensureDepthClipPipeline: %v", err)
	}
	if err := p.ensureBicubicPipelines(); err != nil {
		t.Fatalf("ensureBicubicPipelines: %v", err)
	}
	if err := p.ensureBlitPipeline(); err != nil {
		t.Fatalf("ensureBlitPipeline: %v", err)
	}
	if err := p.ensureYUVPipelines(); err != nil {
		t.Fatalf("ensureYUVPipelines: %v", err)
	}
	if err := p.ensureYUVBlitPipeline(); err != nil {
		t.Fatalf("ensureYUVBlitPipeline: %v", err)
	}
	if err := p.EnsureBlendPipeline(render.BlendCopy); err != nil {
		t.Fatalf("EnsureBlendPipeline: %v", err)
	}

	// Preconditions: every lazily-created field is live; otherwise the
	// post-Destroy nil checks below would pass vacuously.
	live := map[string]bool{
		"pipelineWithStencil":          p.pipelineWithStencil != nil,
		"pipelineWithDepthClip":        p.pipelineWithDepthClip != nil,
		"pipelineWithBicubic":          p.pipelineWithBicubic != nil,
		"pipelineWithBicubicDepthClip": p.pipelineWithBicubicDepthClip != nil,
		"blitPipeline":                 p.blitPipeline != nil,
		"blitLayout":                   p.blitLayout != nil,
		"pipeLayout":                   p.pipeLayout != nil,
		"uniformLayout":                p.uniformLayout != nil,
		"shader":                       p.shader != nil,
		"bicubicShader":                p.bicubicShader != nil,
		"sampler":                      p.sampler != nil,
		"nearestSampler":               p.nearestSampler != nil,
		"yuvShader":                    p.yuvShader != nil,
		"yuvLayout":                    p.yuvLayout != nil,
		"yuvPipeLayout":                p.yuvPipeLayout != nil,
		"yuvPipeStencil":               p.yuvPipeStencil != nil,
		"yuvPipeDepthClip":             p.yuvPipeDepthClip != nil,
		"yuvPipeBlit":                  p.yuvPipeBlit != nil,
		"yuvBlitLayout":                p.yuvBlitLayout != nil,
		"blendPipelines":               len(p.blendPipelines) > 0,
		"blendDepthClipPipelines":      len(p.blendDepthClipPipelines) > 0,
	}
	for name, ok := range live {
		if !ok {
			t.Fatalf("%s not created; post-Destroy assertions would be vacuous", name)
		}
	}

	p.Destroy()

	after := map[string]bool{
		"pipelineWithStencil":          p.pipelineWithStencil == nil,
		"pipelineWithDepthClip":        p.pipelineWithDepthClip == nil,
		"pipelineWithBicubic":          p.pipelineWithBicubic == nil,
		"pipelineWithBicubicDepthClip": p.pipelineWithBicubicDepthClip == nil,
		"blitPipeline":                 p.blitPipeline == nil,
		"blitLayout":                   p.blitLayout == nil,
		"pipeLayout":                   p.pipeLayout == nil,
		"uniformLayout":                p.uniformLayout == nil,
		"shader":                       p.shader == nil,
		"bicubicShader":                p.bicubicShader == nil,
		"sampler":                      p.sampler == nil,
		"nearestSampler":               p.nearestSampler == nil,
		"yuvShader":                    p.yuvShader == nil,
		"yuvLayout":                    p.yuvLayout == nil,
		"yuvPipeLayout":                p.yuvPipeLayout == nil,
		"yuvPipeStencil":               p.yuvPipeStencil == nil,
		"yuvPipeDepthClip":             p.yuvPipeDepthClip == nil,
		"yuvPipeBlit":                  p.yuvPipeBlit == nil,
		"yuvBlitLayout":                p.yuvBlitLayout == nil,
		"blendPipelines":               len(p.blendPipelines) == 0,
		"blendDepthClipPipelines":      len(p.blendDepthClipPipelines) == 0,
	}
	for name, ok := range after {
		if !ok {
			t.Errorf("%s still held after Destroy", name)
		}
	}
}
