//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	_ "embed"
	"fmt"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
)

//go:embed shaders/video_yuv.wgsl
var videoYUVShaderSource string

// Video YUV convert runs on the TexturedQuadPipeline as extra variants:
// the same vertex layout, uniform block, and clip group as image quads,
// only the fragment work differs (NV12 sample + BT.601). One WGSL source
// serves both backends, so the two stay symmetric by construction.

// ensureYUVBase compiles the YUV shader and builds its bind group layout
// (uniform + Y texture + UV texture + sampler) plus pipeline layout.
// Idempotent; nil-safe like ensureBase.
func (p *TexturedQuadPipeline) ensureYUVBase() error {
	if p == nil || p.device == nil {
		return fmt.Errorf("video yuv: nil pipeline or device")
	}
	if p.yuvShader != nil && p.yuvLayout != nil && p.yuvPipeLayout != nil {
		return nil
	}
	if videoYUVShaderSource == "" {
		return fmt.Errorf("video_yuv shader source is empty")
	}
	shader, err := p.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "video_yuv_shader",
		WGSL:  videoYUVShaderSource,
	})
	if err != nil {
		return fmt.Errorf("compile video_yuv shader: %w", err)
	}
	p.yuvShader = shader

	layout, err := p.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "video_yuv_bind_layout",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer:     &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: imageUniformSize},
			},
			{
				Binding:    1,
				Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType:    types.TextureSampleTypeFloat,
					ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding:    2,
				Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType:    types.TextureSampleTypeFloat,
					ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding:    3,
				Visibility: types.ShaderStageFragment,
				Sampler:    &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("create video_yuv bind layout: %w", err)
	}
	p.yuvLayout = layout

	bgLayouts := []hal.BindGroupLayout{p.yuvLayout}
	hasClip := p.clipBindLayout != nil
	if hasClip {
		bgLayouts = append(bgLayouts, p.clipBindLayout)
	}
	pipeLayout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "video_yuv_pipe_layout",
		BindGroupLayouts: bgLayouts,
	})
	if err != nil {
		return fmt.Errorf("create video_yuv pipe layout: %w", err)
	}
	p.yuvPipeLayoutHasClip = hasClip
	p.yuvPipeLayout = pipeLayout
	return nil
}

// ensureYUVPipelines builds the stencil + depth-clip YUV variants.
// Same depth/stencil state as the image counterparts; only the fragment
// module differs. Lazily compiled once so a missing variant never
// silently drops a video draw.
func (p *TexturedQuadPipeline) ensureYUVPipelines() error {
	if p == nil {
		return fmt.Errorf("video yuv: nil pipeline")
	}
	if p.yuvPipeStencil != nil && p.yuvPipeDepthClip != nil && (p.clipBindLayout == nil || p.yuvPipeLayoutHasClip) {
		return nil
	}
	p.destroyYUVMiddle()
	if err := p.ensureYUVBase(); err != nil {
		return err
	}
	premulBlend := types.BlendStatePremultiplied()
	if p.yuvPipeStencil == nil {
		pipe, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
			Label:  "video_yuv_pipeline_with_stencil",
			Layout: p.yuvPipeLayout,
			Vertex: hal.VertexState{
				Module:     p.yuvShader,
				EntryPoint: shaderEntryVS,
				Buffers:    imageVertexLayout(),
			},
			Fragment: &hal.FragmentState{
				Module:     p.yuvShader,
				EntryPoint: shaderEntryFS,
				Targets: []types.ColorTargetState{
					{
						Format:    types.TextureFormatBGRA8Unorm,
						Blend:     &premulBlend,
						WriteMask: types.ColorWriteMaskAll,
					},
				},
			},
			DepthStencil: stencilPassthroughDepthStencil(),
			Primitive:    triangleListPrimitive(),
			Multisample:  multisampleState(p.sampleCount),
		})
		if err != nil {
			return fmt.Errorf("create video yuv stencil pipeline: %w", err)
		}
		p.yuvPipeStencil = pipe
	}
	if p.yuvPipeDepthClip == nil {
		pipe, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
			Label:  "video_yuv_pipeline_depth_clip",
			Layout: p.yuvPipeLayout,
			Vertex: hal.VertexState{
				Module:     p.yuvShader,
				EntryPoint: shaderEntryVS,
				Buffers:    imageVertexLayout(),
			},
			Fragment: &hal.FragmentState{
				Module:     p.yuvShader,
				EntryPoint: shaderEntryFS,
				Targets: []types.ColorTargetState{
					{
						Format:    types.TextureFormatBGRA8Unorm,
						Blend:     &premulBlend,
						WriteMask: types.ColorWriteMaskAll,
					},
				},
			},
			DepthStencil: depthClipDepthStencil(),
			Primitive:    triangleListPrimitive(),
			Multisample:  multisampleState(p.sampleCount),
		})
		if err != nil {
			return fmt.Errorf("create video yuv depth clip pipeline: %w", err)
		}
		p.yuvPipeDepthClip = pipe
	}
	return nil
}

// ensureYUVBlitPipeline builds the non-MSAA YUV variant for the
// compositor fast path. Same shape as ensureBlitPipeline.
func (p *TexturedQuadPipeline) ensureYUVBlitPipeline() error {
	if p == nil {
		return fmt.Errorf("video yuv: nil pipeline")
	}
	if err := p.ensureYUVBase(); err != nil {
		return err
	}
	wantClip := p.clipBindLayout != nil
	if p.yuvPipeBlit != nil && p.yuvBlitLayoutHasClip == wantClip {
		return nil
	}
	if p.yuvPipeBlit != nil {
		p.yuvPipeBlit.Destroy()
		p.yuvPipeBlit = nil
	}
	if p.yuvBlitLayout != nil {
		p.yuvBlitLayout.Destroy()
		p.yuvBlitLayout = nil
	}
	layouts := []hal.BindGroupLayout{p.yuvLayout}
	if wantClip {
		layouts = append(layouts, p.clipBindLayout)
	}
	layout, err := p.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "video_yuv_blit_layout",
		BindGroupLayouts: layouts,
	})
	if err != nil {
		return fmt.Errorf("create video yuv blit layout: %w", err)
	}
	p.yuvBlitLayout = layout
	p.yuvBlitLayoutHasClip = wantClip

	premulBlend := types.BlendStatePremultiplied()
	pipe, err := p.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "video_yuv_blit_pipeline",
		Layout: p.yuvBlitLayout,
		Vertex: hal.VertexState{
			Module:     p.yuvShader,
			EntryPoint: shaderEntryVS,
			Buffers:    imageVertexLayout(),
		},
		Fragment: &hal.FragmentState{
			Module:     p.yuvShader,
			EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{
				{
					Format:    types.TextureFormatBGRA8Unorm,
					Blend:     &premulBlend,
					WriteMask: types.ColorWriteMaskAll,
				},
			},
		},
		Primitive: triangleListPrimitive(),
		Multisample: types.MultisampleState{
			Count: 1,
			Mask:  0xFFFFFFFF,
		},
	})
	if err != nil {
		return fmt.Errorf("create video yuv blit pipeline: %w", err)
	}
	p.yuvPipeBlit = pipe
	return nil
}

// yuvPipelineForDraw selects the YUV pipeline for one draw call.
// Nil means the variants are not ready; the caller keeps the CPU
// fallback instead of silently dropping the frame.
func (p *TexturedQuadPipeline) yuvPipelineForDraw(useDepthClip, useBlit bool) hal.RenderPipeline {
	if p == nil {
		return nil
	}
	if useBlit {
		return p.yuvPipeBlit
	}
	if useDepthClip && p.yuvPipeDepthClip != nil {
		return p.yuvPipeDepthClip
	}
	return p.yuvPipeStencil
}

// destroyYUVMiddle releases the YUV pipe layout (kept separate so a clip
// change rebuilds layout + pipelines together).
func (p *TexturedQuadPipeline) destroyYUVMiddle() {
	if p == nil || p.device == nil {
		return
	}
	if p.yuvPipeStencil != nil {
		p.yuvPipeStencil.Destroy()
		p.yuvPipeStencil = nil
	}
	if p.yuvPipeDepthClip != nil {
		p.yuvPipeDepthClip.Destroy()
		p.yuvPipeDepthClip = nil
	}
	if p.yuvPipeLayout != nil {
		p.yuvPipeLayout.Destroy()
		p.yuvPipeLayout = nil
		p.yuvPipeLayoutHasClip = false
	}
}

// destroyYUV releases all YUV pipeline resources.
func (p *TexturedQuadPipeline) destroyYUV() {
	if p == nil || p.device == nil {
		return
	}
	p.destroyYUVMiddle()
	if p.yuvPipeBlit != nil {
		p.yuvPipeBlit.Destroy()
		p.yuvPipeBlit = nil
	}
	if p.yuvBlitLayout != nil {
		p.yuvBlitLayout.Destroy()
		p.yuvBlitLayout = nil
	}
	if p.yuvLayout != nil {
		p.yuvLayout.Destroy()
		p.yuvLayout = nil
	}
	if p.yuvShader != nil {
		p.yuvShader.Destroy()
		p.yuvShader = nil
	}
}
