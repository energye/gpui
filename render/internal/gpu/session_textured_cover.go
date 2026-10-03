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
	"encoding/binary"
	"fmt"
	"math"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

//go:embed shaders/cover_textured_linear.wgsl
var coverTexturedLinearShaderSource string

//go:embed shaders/cover_textured_pattern.wgsl
var coverTexturedPatternShaderSource string

// texturedCoverUniformSize: viewport(2)+pad(2)+p0(2)+p1(2)+inv_len2+t_min+inv_span+mode = 12 floats = 48 bytes.
const texturedCoverUniformSize = 48

// ensureTexturedCoverPipeline creates the session-inline textured cover pipeline
// (group0: uni+ramp+samp, group1: clip, group2: mask) matching solid cover contract.
func (sr *StencilRenderer) ensureTexturedCoverPipeline() error {
	if sr == nil {
		return fmt.Errorf("nil stencil renderer")
	}
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if sr.texturedCoverPipeline != nil {
		return nil
	}
	// Need solid cover layout pieces first (clip/mask BGLs).
	if sr.nonZeroCoverPipeline == nil {
		if err := sr.createPipelines(); err != nil {
			return err
		}
	}
	if sr.clipBindLayout == nil && sr.defaultClipBindLayout == nil {
		return fmt.Errorf("textured cover: no clip layout")
	}
	clipLay := sr.clipBindLayout
	if clipLay == nil {
		clipLay = sr.defaultClipBindLayout
	}
	maskLay := sr.coverPipeMaskLayout
	if maskLay == nil {
		maskLay = sr.maskBindLayout
	}
	if maskLay == nil {
		if err := sr.ensureNoMaskBindGroup(); err != nil {
			return err
		}
		maskLay = sr.maskBindLayout
	}
	if maskLay == nil {
		return fmt.Errorf("textured cover: no mask layout")
	}

	shader, err := sr.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "cover_textured_linear",
		WGSL:  coverTexturedLinearShaderSource,
	})
	if err != nil {
		return fmt.Errorf("textured cover shader: %w", err)
	}

	bgl0, err := sr.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "textured_cover_g0",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding: 0, Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: texturedCoverUniformSize},
			},
			{
				Binding: 1, Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding: 2, Visibility: types.ShaderStageFragment,
				Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		shader.Destroy()
		return err
	}

	pipeLay, err := sr.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label: "textured_cover_lay",
		BindGroupLayouts: []hal.BindGroupLayout{
			bgl0, clipLay, maskLay,
		},
	})
	if err != nil {
		bgl0.Destroy()
		shader.Destroy()
		return err
	}

	vbl := []types.VertexBufferLayout{{
		ArrayStride: 8,
		StepMode:    types.VertexStepModeVertex,
		Attributes: []types.VertexAttribute{{
			Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0,
		}},
	}}
	premul := types.BlendStatePremultiplied()
	ms := multisampleState(sr.sampleCount)
	prim := triangleListPrimitive()

	pipe, err := sr.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "textured_cover_pipeline",
		Layout: pipeLay,
		Vertex: hal.VertexState{Module: shader, EntryPoint: shaderEntryVS, Buffers: vbl},
		Fragment: &hal.FragmentState{
			Module: shader, EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatBGRA8Unorm, Blend: &premul, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		DepthStencil: &hal.DepthStencilState{
			Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
			DepthCompare: types.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep, PassOp: types.StencilOperationZero,
			},
			StencilBack: hal.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep, PassOp: types.StencilOperationZero,
			},
			StencilReadMask: 0xFF, StencilWriteMask: 0xFF,
		},
		Multisample: ms,
		Primitive:   prim,
	})
	if err != nil {
		pipeLay.Destroy()
		bgl0.Destroy()
		shader.Destroy()
		return fmt.Errorf("textured cover pipe: %w", err)
	}

	samp, err := sr.device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "textured_cover_ramp_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeNearest, Anisotropy: 1,
	})
	if err != nil {
		pipe.Destroy()
		pipeLay.Destroy()
		bgl0.Destroy()
		shader.Destroy()
		return err
	}

	sr.texturedCoverShader = shader
	sr.texturedCoverBGL0 = bgl0
	sr.texturedCoverPipeLay = pipeLay
	sr.texturedCoverPipeline = pipe
	sr.texturedCoverSampler = samp
	return nil
}

func encodeTexturedCoverUniform(w, h uint32, cmd *StencilPathCommand) []byte {
	buf := make([]byte, texturedCoverUniformSize)
	put := func(i int, v float32) {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	put(0, float32(w))
	put(1, float32(h))
	put(2, 0)
	put(3, 0)
	put(4, cmd.TexP0X)
	put(5, cmd.TexP0Y)
	put(6, cmd.TexP1X)
	put(7, cmd.TexP1Y)
	put(8, cmd.TexInvLen2)
	put(9, cmd.TexTMin)
	put(10, cmd.TexInvSpan)
	put(11, cmd.TexMode)
	return buf
}

// updateTexturedCoverResources attaches ramp texture + textured cover bind group.
func (sr *StencilRenderer) updateTexturedCoverResources(b *stencilCoverBuffers, w, h uint32, cmd *StencilPathCommand) error {
	if cmd == nil || cmd.RampN <= 0 || len(cmd.Ramp) < cmd.RampN*4 {
		b.isTextured = false
		return nil
	}
	if err := sr.ensureTexturedCoverPipeline(); err != nil {
		return err
	}

	// Release previous textured resources.
	if b.texturedCoverBG != nil {
		b.texturedCoverBG.Destroy()
		b.texturedCoverBG = nil
	}
	if b.rampView != nil {
		b.rampView.Destroy()
		b.rampView = nil
	}
	if b.rampTex != nil {
		b.rampTex.Destroy()
		b.rampTex = nil
	}

	n := cmd.RampN
	rampTex, err := sr.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "session_tex_cover_ramp",
		Size:          hal.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return err
	}
	rampView, err := sr.device.CreateTextureView(rampTex, &hal.TextureViewDescriptor{
		Label: "session_tex_cover_ramp_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		rampTex.Destroy()
		return err
	}
	rbpr := alignTextureBytesPerRow(uint32(n * 4)) //nolint:gosec
	rampUp := cmd.Ramp
	if rbpr != uint32(n*4) { //nolint:gosec
		padded := make([]byte, int(rbpr))
		copy(padded, cmd.Ramp[:n*4])
		rampUp = padded
	}
	if err := sr.queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: rampTex, MipLevel: 0},
		rampUp,
		&hal.ImageDataLayout{BytesPerRow: rbpr, RowsPerImage: 1},
		&hal.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		rampView.Destroy()
		rampTex.Destroy()
		return err
	}

	// Cover uniform for textured path (48 bytes; solid path used 32).
	uni := encodeTexturedCoverUniform(w, h, cmd)
	if b.coverUniBuf == nil || b.coverUniCap < texturedCoverUniformSize {
		if b.coverUniBuf != nil {
			b.coverUniBuf.Destroy()
			b.coverUniBuf = nil
		}
		// Solid coverBindGroup referenced old uni — invalidate.
		if b.coverBindGroup != nil {
			b.coverBindGroup.Destroy()
			b.coverBindGroup = nil
		}
		ub, err := sr.device.CreateBuffer(&hal.BufferDescriptor{
			Label: "tex_cover_uni", Size: texturedCoverUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			rampView.Destroy()
			rampTex.Destroy()
			return err
		}
		b.coverUniBuf = ub
		b.coverUniCap = texturedCoverUniformSize
	}
	if err := sr.queue.WriteBuffer(b.coverUniBuf, 0, uni); err != nil {
		rampView.Destroy()
		rampTex.Destroy()
		return err
	}

	bg, err := sr.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "session_tex_cover_bg",
		Layout: sr.texturedCoverBGL0,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: b.coverUniBuf, Offset: 0, Size: texturedCoverUniformSize},
			{Binding: 1, TextureView: rampView},
			{Binding: 2, Sampler: sr.texturedCoverSampler},
		},
	})
	if err != nil {
		rampView.Destroy()
		rampTex.Destroy()
		return err
	}

	b.rampTex = rampTex
	b.rampView = rampView
	b.texturedCoverBG = bg
	b.isTextured = true
	b.isPattern = false
	return nil
}

// queueSessionTexturedCover enqueues a device-space stencil-then-textured-cover
// path into the session pass (true session-inline, no offscreen result).
func (rc *GPURenderContext) queueSessionTexturedCover(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	ramp []byte,
	rampN int,
	p0x, p0y, p1x, p1y, invLen2, tMin, invSpan, mode float32,
) error {
	if rc == nil || path == nil || paint == nil || rampN < 1 || len(ramp) < rampN*4 {
		return render.ErrFallbackToCPU
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	var fanVerts []float32
	var coverQuad [12]float32
	aaOff := !rc.antiAlias
	fr := paint.FillRule
	if cache := rc.shared.PathGeomCache(); cache != nil {
		if v, cq, ok := cache.GetOrTessellate(path, fr, aaOff); ok {
			fanVerts, coverQuad = v, cq
		}
	}
	if fanVerts == nil {
		// Pooled tessellator: queued command outlives the call, copy out.
		tess := acquireFanTessellator()
		tess.TessellatePath(path)
		fv := tess.Vertices()
		if len(fv) == 0 {
			releaseFanTessellator(tess)
			return nil
		}
		fanVerts = append([]float32(nil), fv...)
		coverQuad = tess.CoverQuad()
		releaseFanTessellator(tess)
	}
	if len(fanVerts) == 0 {
		return nil
	}

	// Own ramp copy — command may outlive caller stack.
	rampCopy := make([]byte, rampN*4)
	copy(rampCopy, ramp[:rampN*4])

	cmd := StencilPathCommand{
		Matrix:    render.Identity(), // baked device-space (pattern/textured, F2 fallback)
		Vertices:  fanVerts,
		CoverQuad: coverQuad,
		Color:     [4]float32{0, 0, 0, 0}, // unused when textured
		FillRule:  paint.FillRule,
		BlendMode: paint.BlendMode,
		Ramp:      rampCopy,
		RampN:     rampN,
		TexP0X:    p0x, TexP0Y: p0y,
		TexP1X: p1x, TexP1Y: p1y,
		TexInvLen2: invLen2,
		TexTMin:    tMin,
		TexInvSpan: invSpan,
		TexMode:    mode,
	}
	rc.QueueStencil(target, cmd)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	rc.markBrushSessionInline()
	return nil
}

// patternCoverUniformSize: viewport(2)+pad(2)+inv0(4)+inv1(4)+pat(2)+opacity+clamp = 16 floats = 64 bytes.
const patternCoverUniformSize = 64

func (sr *StencilRenderer) ensurePatternCoverPipeline() error {
	if sr == nil {
		return fmt.Errorf("nil stencil renderer")
	}
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if sr.patternCoverPipeline != nil {
		return nil
	}
	if sr.nonZeroCoverPipeline == nil {
		if err := sr.createPipelines(); err != nil {
			return err
		}
	}
	clipLay := sr.clipBindLayout
	if clipLay == nil {
		clipLay = sr.defaultClipBindLayout
	}
	if clipLay == nil {
		return fmt.Errorf("pattern cover: no clip layout")
	}
	maskLay := sr.coverPipeMaskLayout
	if maskLay == nil {
		maskLay = sr.maskBindLayout
	}
	if maskLay == nil {
		if err := sr.ensureNoMaskBindGroup(); err != nil {
			return err
		}
		maskLay = sr.maskBindLayout
	}
	if maskLay == nil {
		return fmt.Errorf("pattern cover: no mask layout")
	}

	shader, err := sr.device.CreateShaderModule(&hal.ShaderModuleDescriptor{
		Label: "cover_textured_pattern",
		WGSL:  coverTexturedPatternShaderSource,
	})
	if err != nil {
		return fmt.Errorf("pattern cover shader: %w", err)
	}
	bgl0, err := sr.device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{
		Label: "pattern_cover_g0",
		Entries: []types.BindGroupLayoutEntry{
			{
				Binding: 0, Visibility: types.ShaderStageVertex | types.ShaderStageFragment,
				Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: patternCoverUniformSize},
			},
			{
				Binding: 1, Visibility: types.ShaderStageFragment,
				Texture: &types.TextureBindingLayout{
					SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D,
				},
			},
			{
				Binding: 2, Visibility: types.ShaderStageFragment,
				Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering},
			},
		},
	})
	if err != nil {
		shader.Destroy()
		return err
	}
	pipeLay, err := sr.device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{
		Label:            "pattern_cover_lay",
		BindGroupLayouts: []hal.BindGroupLayout{bgl0, clipLay, maskLay},
	})
	if err != nil {
		bgl0.Destroy()
		shader.Destroy()
		return err
	}
	vbl := []types.VertexBufferLayout{{
		ArrayStride: 8,
		StepMode:    types.VertexStepModeVertex,
		Attributes: []types.VertexAttribute{{
			Format: types.VertexFormatFloat32x2, Offset: 0, ShaderLocation: 0,
		}},
	}}
	premul := types.BlendStatePremultiplied()
	ms := multisampleState(sr.sampleCount)
	prim := triangleListPrimitive()
	pipe, err := sr.device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label:  "pattern_cover_pipeline",
		Layout: pipeLay,
		Vertex: hal.VertexState{Module: shader, EntryPoint: shaderEntryVS, Buffers: vbl},
		Fragment: &hal.FragmentState{
			Module: shader, EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatBGRA8Unorm, Blend: &premul, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		DepthStencil: &hal.DepthStencilState{
			Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
			DepthCompare: types.CompareFunctionAlways,
			StencilFront: hal.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep, PassOp: types.StencilOperationZero,
			},
			StencilBack: hal.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: types.StencilOperationKeep,
				DepthFailOp: types.StencilOperationKeep, PassOp: types.StencilOperationZero,
			},
			StencilReadMask: 0xFF, StencilWriteMask: 0xFF,
		},
		Multisample: ms,
		Primitive:   prim,
	})
	if err != nil {
		pipeLay.Destroy()
		bgl0.Destroy()
		shader.Destroy()
		return fmt.Errorf("pattern cover pipe: %w", err)
	}
	samp, err := sr.device.CreateSampler(&hal.SamplerDescriptor{
		Label:        "pattern_cover_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest, MinFilter: types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest, Anisotropy: 1,
	})
	if err != nil {
		pipe.Destroy()
		pipeLay.Destroy()
		bgl0.Destroy()
		shader.Destroy()
		return err
	}
	sr.patternCoverShader = shader
	sr.patternCoverBGL0 = bgl0
	sr.patternCoverPipeLay = pipeLay
	sr.patternCoverPipeline = pipe
	sr.patternCoverSampler = samp
	return nil
}

func encodePatternCoverUniform(w, h uint32, cmd *StencilPathCommand) []byte {
	buf := make([]byte, patternCoverUniformSize)
	put := func(i int, v float32) {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	put(0, float32(w))
	put(1, float32(h))
	put(2, 0)
	put(3, 0)
	put(4, cmd.PatInvA)
	put(5, cmd.PatInvB)
	put(6, cmd.PatInvC)
	put(7, 0)
	put(8, cmd.PatInvD)
	put(9, cmd.PatInvE)
	put(10, cmd.PatInvF)
	put(11, 0)
	put(12, float32(cmd.PatW))
	put(13, float32(cmd.PatH))
	put(14, cmd.PatOpacity)
	put(15, cmd.PatClamp)
	return buf
}

// updatePatternCoverResources attaches pattern tile + cover bind group.
func (sr *StencilRenderer) updatePatternCoverResources(b *stencilCoverBuffers, w, h uint32, cmd *StencilPathCommand) error {
	if cmd == nil || cmd.PatW <= 0 || cmd.PatH <= 0 || len(cmd.PatTile) < cmd.PatW*cmd.PatH*4 {
		b.isPattern = false
		return nil
	}
	if err := sr.ensurePatternCoverPipeline(); err != nil {
		return err
	}
	if b.texturedCoverBG != nil {
		b.texturedCoverBG.Destroy()
		b.texturedCoverBG = nil
	}
	if b.rampView != nil {
		b.rampView.Destroy()
		b.rampView = nil
	}
	if b.rampTex != nil {
		b.rampTex.Destroy()
		b.rampTex = nil
	}

	srcW, srcH := cmd.PatW, cmd.PatH
	patTex, err := sr.device.CreateTexture(&hal.TextureDescriptor{
		Label:         "session_pat_cover_src",
		Size:          hal.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return err
	}
	patView, err := sr.device.CreateTextureView(patTex, &hal.TextureViewDescriptor{
		Label: "session_pat_cover_src_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		patTex.Destroy()
		return err
	}
	bpr := alignTextureBytesPerRow(uint32(srcW * 4)) //nolint:gosec
	up := cmd.PatTile
	if bpr != uint32(srcW*4) { //nolint:gosec
		padded := make([]byte, int(bpr)*srcH)
		rowBytes := srcW * 4
		for y := 0; y < srcH; y++ {
			copy(padded[y*int(bpr):y*int(bpr)+rowBytes], cmd.PatTile[y*rowBytes:(y+1)*rowBytes])
		}
		up = padded
	}
	if err := sr.queue.WriteTexture(
		&hal.ImageCopyTexture{Texture: patTex, MipLevel: 0},
		up,
		&hal.ImageDataLayout{BytesPerRow: bpr, RowsPerImage: uint32(srcH)},              //nolint:gosec
		&hal.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		patView.Destroy()
		patTex.Destroy()
		return err
	}

	uni := encodePatternCoverUniform(w, h, cmd)
	if b.coverUniBuf == nil || b.coverUniCap < patternCoverUniformSize {
		if b.coverUniBuf != nil {
			b.coverUniBuf.Destroy()
			b.coverUniBuf = nil
		}
		if b.coverBindGroup != nil {
			b.coverBindGroup.Destroy()
			b.coverBindGroup = nil
		}
		ub, err := sr.device.CreateBuffer(&hal.BufferDescriptor{
			Label: "pat_cover_uni", Size: patternCoverUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			patView.Destroy()
			patTex.Destroy()
			return err
		}
		b.coverUniBuf = ub
		b.coverUniCap = patternCoverUniformSize
	}
	if err := sr.queue.WriteBuffer(b.coverUniBuf, 0, uni); err != nil {
		patView.Destroy()
		patTex.Destroy()
		return err
	}
	bg, err := sr.device.CreateBindGroup(&hal.BindGroupDescriptor{
		Label:  "session_pat_cover_bg",
		Layout: sr.patternCoverBGL0,
		Entries: []hal.BindGroupEntry{
			{Binding: 0, Buffer: b.coverUniBuf, Offset: 0, Size: patternCoverUniformSize},
			{Binding: 1, TextureView: patView},
			{Binding: 2, Sampler: sr.patternCoverSampler},
		},
	})
	if err != nil {
		patView.Destroy()
		patTex.Destroy()
		return err
	}
	b.rampTex = patTex
	b.rampView = patView
	b.texturedCoverBG = bg
	b.isPattern = true
	b.isTextured = false
	return nil
}

// queueSessionPatternCover enqueues device-space stencil + ImagePattern cover
// into the main session pass (N2 session-inline).
func (rc *GPURenderContext) queueSessionPatternCover(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	tile []byte,
	srcW, srcH int,
	invA, invB, invC, invD, invE, invF float32,
	opacity, clampMode float32,
) error {
	if rc == nil || path == nil || paint == nil || srcW <= 0 || srcH <= 0 || len(tile) < srcW*srcH*4 {
		return render.ErrFallbackToCPU
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	var fanVerts []float32
	var coverQuad [12]float32
	aaOff := !rc.antiAlias
	fr := paint.FillRule
	if cache := rc.shared.PathGeomCache(); cache != nil {
		if v, cq, ok := cache.GetOrTessellate(path, fr, aaOff); ok {
			fanVerts, coverQuad = v, cq
		}
	}
	if fanVerts == nil {
		// Pooled tessellator: queued command outlives the call, copy out.
		tess := acquireFanTessellator()
		tess.TessellatePath(path)
		fv := tess.Vertices()
		if len(fv) == 0 {
			releaseFanTessellator(tess)
			return nil
		}
		fanVerts = append([]float32(nil), fv...)
		coverQuad = tess.CoverQuad()
		releaseFanTessellator(tess)
	}
	if len(fanVerts) == 0 {
		return nil
	}

	tileCopy := make([]byte, srcW*srcH*4)
	copy(tileCopy, tile[:srcW*srcH*4])

	cmd := StencilPathCommand{
		Matrix:     render.Identity(), // baked device-space (pattern/textured, F2 fallback)
		Vertices:   fanVerts,
		CoverQuad:  coverQuad,
		Color:      [4]float32{0, 0, 0, 0},
		FillRule:   paint.FillRule,
		BlendMode:  paint.BlendMode,
		PatTile:    tileCopy,
		PatW:       srcW,
		PatH:       srcH,
		PatInvA:    invA,
		PatInvB:    invB,
		PatInvC:    invC,
		PatInvD:    invD,
		PatInvE:    invE,
		PatInvF:    invF,
		PatOpacity: opacity,
		PatClamp:   clampMode,
	}
	rc.QueueStencil(target, cmd)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	rc.markBrushSessionInline()
	return nil
}
