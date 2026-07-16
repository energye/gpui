//go:build !nogpu

package gpu

import (
	_ "embed"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
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

	shader, err := sr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "cover_textured_linear",
		WGSL:  coverTexturedLinearShaderSource,
	})
	if err != nil {
		return fmt.Errorf("textured cover shader: %w", err)
	}

	bgl0, err := sr.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
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
		shader.Release()
		return err
	}

	pipeLay, err := sr.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label: "textured_cover_lay",
		BindGroupLayouts: []*webgpu.BindGroupLayout{
			bgl0, clipLay, maskLay,
		},
	})
	if err != nil {
		bgl0.Release()
		shader.Release()
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

	pipe, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "textured_cover_pipeline",
		Layout: pipeLay,
		Vertex: webgpu.VertexState{Module: shader, EntryPoint: shaderEntryVS, Buffers: vbl},
		Fragment: &webgpu.FragmentState{
			Module: shader, EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatBGRA8Unorm, Blend: &premul, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
			DepthCompare: types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilReadMask: 0xFF, StencilWriteMask: 0xFF,
		},
		Multisample: ms,
		Primitive:   prim,
	})
	if err != nil {
		pipeLay.Release()
		bgl0.Release()
		shader.Release()
		return fmt.Errorf("textured cover pipe: %w", err)
	}

	samp, err := sr.device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "textured_cover_ramp_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeLinear, MinFilter: types.FilterModeLinear,
		MipmapFilter: types.MipmapFilterModeNearest, Anisotropy: 1,
	})
	if err != nil {
		pipe.Release()
		pipeLay.Release()
		bgl0.Release()
		shader.Release()
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
		b.texturedCoverBG.Release()
		b.texturedCoverBG = nil
	}
	if b.rampView != nil {
		b.rampView.Release()
		b.rampView = nil
	}
	if b.rampTex != nil {
		b.rampTex.Release()
		b.rampTex = nil
	}

	n := cmd.RampN
	rampTex, err := sr.device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "session_tex_cover_ramp",
		Size:          webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return err
	}
	rampView, err := sr.device.CreateTextureView(rampTex, &webgpu.TextureViewDescriptor{
		Label: "session_tex_cover_ramp_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		rampTex.Release()
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
		&webgpu.ImageCopyTexture{Texture: rampTex, MipLevel: 0},
		rampUp,
		&webgpu.ImageDataLayout{BytesPerRow: rbpr, RowsPerImage: 1},
		&webgpu.Extent3D{Width: uint32(n), Height: 1, DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		rampView.Release()
		rampTex.Release()
		return err
	}

	// Cover uniform for textured path (48 bytes; solid path used 32).
	uni := encodeTexturedCoverUniform(w, h, cmd)
	if b.coverUniBuf == nil || b.coverUniCap < texturedCoverUniformSize {
		if b.coverUniBuf != nil {
			b.coverUniBuf.Release()
			b.coverUniBuf = nil
		}
		// Solid coverBindGroup referenced old uni — invalidate.
		if b.coverBindGroup != nil {
			b.coverBindGroup.Release()
			b.coverBindGroup = nil
		}
		ub, err := sr.device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "tex_cover_uni", Size: texturedCoverUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			rampView.Release()
			rampTex.Release()
			return err
		}
		b.coverUniBuf = ub
		b.coverUniCap = texturedCoverUniformSize
	}
	if err := sr.queue.WriteBuffer(b.coverUniBuf, 0, uni); err != nil {
		rampView.Release()
		rampTex.Release()
		return err
	}

	bg, err := sr.device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "session_tex_cover_bg",
		Layout: sr.texturedCoverBGL0,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: b.coverUniBuf, Offset: 0, Size: texturedCoverUniformSize},
			{Binding: 1, TextureView: rampView},
			{Binding: 2, Sampler: sr.texturedCoverSampler},
		},
	})
	if err != nil {
		rampView.Release()
		rampTex.Release()
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
		tess := NewFanTessellator()
		tess.TessellatePath(path)
		fanVerts = tess.Vertices()
		if len(fanVerts) == 0 {
			return nil
		}
		coverQuad = tess.CoverQuad()
	}
	if len(fanVerts) == 0 {
		return nil
	}

	// Own ramp copy — command may outlive caller stack.
	rampCopy := make([]byte, rampN*4)
	copy(rampCopy, ramp[:rampN*4])

	cmd := StencilPathCommand{
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

	shader, err := sr.device.CreateShaderModule(&webgpu.ShaderModuleDescriptor{
		Label: "cover_textured_pattern",
		WGSL:  coverTexturedPatternShaderSource,
	})
	if err != nil {
		return fmt.Errorf("pattern cover shader: %w", err)
	}
	bgl0, err := sr.device.CreateBindGroupLayout(&webgpu.BindGroupLayoutDescriptor{
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
		shader.Release()
		return err
	}
	pipeLay, err := sr.device.CreatePipelineLayout(&webgpu.PipelineLayoutDescriptor{
		Label:            "pattern_cover_lay",
		BindGroupLayouts: []*webgpu.BindGroupLayout{bgl0, clipLay, maskLay},
	})
	if err != nil {
		bgl0.Release()
		shader.Release()
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
	pipe, err := sr.device.CreateRenderPipeline(&webgpu.RenderPipelineDescriptor{
		Label:  "pattern_cover_pipeline",
		Layout: pipeLay,
		Vertex: webgpu.VertexState{Module: shader, EntryPoint: shaderEntryVS, Buffers: vbl},
		Fragment: &webgpu.FragmentState{
			Module: shader, EntryPoint: shaderEntryFS,
			Targets: []types.ColorTargetState{{
				Format: types.TextureFormatBGRA8Unorm, Blend: &premul, WriteMask: types.ColorWriteMaskAll,
			}},
		},
		DepthStencil: &webgpu.DepthStencilState{
			Format: types.TextureFormatDepth24PlusStencil8, DepthWriteEnabled: false,
			DepthCompare: types.CompareFunctionAlways,
			StencilFront: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilBack: webgpu.StencilFaceState{
				Compare: types.CompareFunctionNotEqual, FailOp: webgpu.StencilOperationKeep,
				DepthFailOp: webgpu.StencilOperationKeep, PassOp: webgpu.StencilOperationZero,
			},
			StencilReadMask: 0xFF, StencilWriteMask: 0xFF,
		},
		Multisample: ms,
		Primitive:   prim,
	})
	if err != nil {
		pipeLay.Release()
		bgl0.Release()
		shader.Release()
		return fmt.Errorf("pattern cover pipe: %w", err)
	}
	samp, err := sr.device.CreateSampler(&webgpu.SamplerDescriptor{
		Label:        "pattern_cover_samp",
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		AddressModeW: types.AddressModeClampToEdge,
		MagFilter:    types.FilterModeNearest, MinFilter: types.FilterModeNearest,
		MipmapFilter: types.MipmapFilterModeNearest, Anisotropy: 1,
	})
	if err != nil {
		pipe.Release()
		pipeLay.Release()
		bgl0.Release()
		shader.Release()
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
		b.texturedCoverBG.Release()
		b.texturedCoverBG = nil
	}
	if b.rampView != nil {
		b.rampView.Release()
		b.rampView = nil
	}
	if b.rampTex != nil {
		b.rampTex.Release()
		b.rampTex = nil
	}

	srcW, srcH := cmd.PatW, cmd.PatH
	patTex, err := sr.device.CreateTexture(&webgpu.TextureDescriptor{
		Label:         "session_pat_cover_src",
		Size:          webgpu.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
		MipLevelCount: 1, SampleCount: 1, Dimension: types.TextureDimension2D,
		Format: types.TextureFormatRGBA8Unorm,
		Usage:  types.TextureUsageTextureBinding | types.TextureUsageCopyDst,
	})
	if err != nil {
		return err
	}
	patView, err := sr.device.CreateTextureView(patTex, &webgpu.TextureViewDescriptor{
		Label: "session_pat_cover_src_view", Format: types.TextureFormatRGBA8Unorm,
		Dimension: types.TextureViewDimension2D, Aspect: types.TextureAspectAll, MipLevelCount: 1,
	})
	if err != nil {
		patTex.Release()
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
		&webgpu.ImageCopyTexture{Texture: patTex, MipLevel: 0},
		up,
		&webgpu.ImageDataLayout{BytesPerRow: bpr, RowsPerImage: uint32(srcH)},              //nolint:gosec
		&webgpu.Extent3D{Width: uint32(srcW), Height: uint32(srcH), DepthOrArrayLayers: 1}, //nolint:gosec
	); err != nil {
		patView.Release()
		patTex.Release()
		return err
	}

	uni := encodePatternCoverUniform(w, h, cmd)
	if b.coverUniBuf == nil || b.coverUniCap < patternCoverUniformSize {
		if b.coverUniBuf != nil {
			b.coverUniBuf.Release()
			b.coverUniBuf = nil
		}
		if b.coverBindGroup != nil {
			b.coverBindGroup.Release()
			b.coverBindGroup = nil
		}
		ub, err := sr.device.CreateBuffer(&webgpu.BufferDescriptor{
			Label: "pat_cover_uni", Size: patternCoverUniformSize,
			Usage: types.BufferUsageUniform | types.BufferUsageCopyDst,
		})
		if err != nil {
			patView.Release()
			patTex.Release()
			return err
		}
		b.coverUniBuf = ub
		b.coverUniCap = patternCoverUniformSize
	}
	if err := sr.queue.WriteBuffer(b.coverUniBuf, 0, uni); err != nil {
		patView.Release()
		patTex.Release()
		return err
	}
	bg, err := sr.device.CreateBindGroup(&webgpu.BindGroupDescriptor{
		Label:  "session_pat_cover_bg",
		Layout: sr.patternCoverBGL0,
		Entries: []webgpu.BindGroupEntry{
			{Binding: 0, Buffer: b.coverUniBuf, Offset: 0, Size: patternCoverUniformSize},
			{Binding: 1, TextureView: patView},
			{Binding: 2, Sampler: sr.patternCoverSampler},
		},
	})
	if err != nil {
		patView.Release()
		patTex.Release()
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
		tess := NewFanTessellator()
		tess.TessellatePath(path)
		fanVerts = tess.Vertices()
		if len(fanVerts) == 0 {
			return nil
		}
		coverQuad = tess.CoverQuad()
	}
	if len(fanVerts) == 0 {
		return nil
	}

	tileCopy := make([]byte, srcW*srcH*4)
	copy(tileCopy, tile[:srcW*srcH*4])

	cmd := StencilPathCommand{
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
