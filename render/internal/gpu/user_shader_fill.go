//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"fmt"
	"image"
	"math"
	"os"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// fillShaderAsImage evaluates a user ShaderEffect over the painted bounds and
// composites the result through the standard coverage chain (P3.2): pass 1
// renders the effect into an offscreen premul RGBA RT; its pixels then ride
// the same R8-mask modulation path fillMaskedAsImage uses, so clip/mask/blend
// semantics match every other fill.
func (rc *GPURenderContext) fillShaderAsImage(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	effect := paint.Shader
	if effect == nil {
		return nil
	}
	// Lazy device init, same shape as the sibling GPU paths (L2-P0-2: a
	// shader paint as the process's first GPU op used to find device==nil,
	// fall to CPU, and paint the ambient solid).
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(bb.Min.X) - 1
		y0 := int(bb.Min.Y) - 1
		x1 := int(bb.Max.X) + 1
		y1 := int(bb.Max.Y) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 || bw*bh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Pass 1: run the user shader over the bounds. The effect returns
	// premultiplied color (ShaderEffect contract: the engine premultiplies a
	// straight result, so users may emit either; the chain below is fed
	// straight). Un-premultiply here because the image-draw composite leg
	// multiplies uploaded alpha into RGB at draw time (L2-P1-4 calibration:
	// premul-fed data gets alpha squared — (0.5,0,0,0.5) landed as (0.25,0,0)
	// contribution, read back as washed gray).
	srcRegion, err := rc.runUserShaderPass(device, queue, effect, bw, bh)
	if err != nil {
		fmt.Fprintln(os.Stderr, "[user_shader] pass1 err:", err)
		return render.ErrFallbackToCPU
	}
	premulToStraightRGBA(srcRegion)

	// Coverage: rasterize the path opaque; the alpha channel is the AA
	// coverage the mask chain multiplies with (same basis as
	// fillMaskedAsImage's CPU-raster source).
	localPath := path
	if bounds.Min.X != 0 || bounds.Min.Y != 0 {
		localPath = path.Transform(render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)))
	}
	pm := render.NewPixmap(bw, bh)
	sr := render.NewSoftwareRenderer(bw, bh)
	sr.SetAntiAlias(rc.antiAlias)
	local := paint.Clone()
	local.Shader = nil
	local.BlendMode = render.BlendNormal
	local.MaskCoverage = nil
	local.ClipCoverage = nil
	if err := sr.Fill(pm, localPath, local); err != nil {
		return render.ErrFallbackToCPU
	}
	pm.NotifyPixelsChanged()

	// Path coverage comes from the opaque raster's alpha channel (same basis
	// as fillMaskedAsImage's CPU-raster source); an active Mask plane or
	// MaskCoverage multiplies on top.
	maskRegion := make([]byte, bw*bh)
	srcFull := pm.Data()
	any := false
	for row := 0; row < bh; row++ {
		py := bounds.Min.Y + row
		for col := 0; col < bw; col++ {
			px := bounds.Min.X + col
			m := srcFull[(row*bw+col)*4+3]
			if plane, pw, ph, ok := rc.shared.MaskPlane(); ok && pw == tw && ph == th {
				m = uint8(uint16(m) * uint16(plane[py*tw+px]) / 255)
			} else if maskFn := paint.MaskCoverage; maskFn != nil {
				m = uint8(uint16(m) * uint16(maskFn(px, py)) / 255)
			}
			maskRegion[row*bw+col] = m
			if srcRegion[row*bw*4+col*4+3] != 0 && m != 0 {
				any = true
			}
		}
	}
	if !any {
		return nil
	}

	var pixelData []byte
	if bw*bh <= 128*1024 {
		maskPreMulRGBAInPlace(srcRegion, maskRegion)
		pixelData = srcRegion
	} else {
		pixelData, err = maskR8Modulate(device, queue, &rc.shared.maskR8, srcRegion, maskRegion, bw, bh)
		if err != nil {
			return render.ErrFallbackToCPU
		}
	}

	fx0 := float32(bounds.Min.X)
	fy0 := float32(bounds.Min.Y)
	fx1 := float32(bounds.Max.X)
	fy1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, pixelData, 0, nil, render.MipmapNone, bw, bh, bw*4,
		fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
		1.0, uint32(tw), uint32(th), 0, 0, 1, 1, false, false)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// premulToStraightRGBA converts premultiplied bytes to straight in place
// (c = c*255/a, round-half-up; a==0 leaves rgb untouched, they are masked
// out downstream anyway).
func premulToStraightRGBA(data []byte) {
	for i := 0; i+3 < len(data); i += 4 {
		a := uint16(data[i+3])
		if a == 0 || a == 255 {
			continue
		}
		data[i] = uint8((uint16(data[i])*255 + a/2) / a)
		data[i+1] = uint8((uint16(data[i+1])*255 + a/2) / a)
		data[i+2] = uint8((uint16(data[i+2])*255 + a/2) / a)
	}
}

// runUserShaderPass renders the effect into a scratch BGRA8 RT and maps it
// back as premul RGBA bytes (the readback helper swizzles BGRA→RGBA, matching
// every other offscreen chain). Per-call scratch texture; pooling into
// res.Cache is a registered follow-up.
func (rc *GPURenderContext) runUserShaderPass(device hal.Device, queue hal.Queue, effect *render.ShaderEffect, bw, bh int) ([]byte, error) {
	pipe, err := rc.ensureUserShaderPipeline(device, queue, effect)
	if err != nil {
		return nil, err
	}
	tex, view, err := dualTexCreateTexFmt(device, queue, "user_shader_rt", bw, bh, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc, types.TextureFormatBGRA8Unorm)
	if err != nil {
		return nil, err
	}
	defer tex.Destroy()
	defer view.Destroy()

	uniforms := effect.Uniforms()
	if len(uniforms) > 0 {
		if err := queue.WriteBuffer(pipe.uniform, 0, uniforms); err != nil {
			return nil, err
		}
	}

	// Bind entries mirror the layout exactly (L2-P1-3): the uniform slot is
	// bound at the layout's full size, and a declared image slot falls back
	// to the cached 1x1 black placeholder when no image is bound.
	entries := []hal.BindGroupEntry{{Binding: 0, Buffer: pipe.uniform, Offset: 0, Size: uint64(pipe.uniformSize)}}
	if pipe.hasTex {
		var imgView hal.TextureView
		var samp hal.Sampler
		if img := effect.Image(); img != nil {
			data := img.PremultipliedData()
			imgTex, view, terr := dualTexCreateTex(device, queue, "user_shader_img", img.Width(), img.Height(), data,
				types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
			if terr != nil {
				return nil, terr
			}
			defer imgTex.Destroy()
			defer view.Destroy()
			s, serr := device.CreateSampler(&hal.SamplerDescriptor{MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear})
			if serr != nil {
				return nil, serr
			}
			defer s.Destroy()
			imgView, samp = view, s
		} else {
			c := &rc.shared.userShader
			c.mu.Lock()
			berr := c.ensureBlackBindings(device, queue)
			imgView, samp = c.blackView, c.blackSamp
			c.mu.Unlock()
			if berr != nil {
				return nil, berr
			}
		}
		entries = append(entries,
			hal.BindGroupEntry{Binding: 1, TextureView: imgView},
			hal.BindGroupEntry{Binding: 2, Sampler: samp},
		)
	}
	bg, err := device.CreateBindGroup(&hal.BindGroupDescriptor{Layout: pipe.layoutBindGroupLayout(), Entries: entries})
	if err != nil {
		return nil, err
	}
	defer bg.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "user_shader_enc"})
	if err != nil {
		return nil, err
	}
	pass, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{
		Label: "user_shader_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View:       view,
			LoadOp:     types.LoadOpClear,
			StoreOp:    types.StoreOpStore,
			ClearValue: types.Color{R: 0, G: 0, B: 0, A: 0},
		}},
	})
	if err != nil {
		return nil, err
	}
	pass.SetPipeline(pipe.pipeline)
	pass.SetBindGroup(0, bg, nil)
	pass.Draw(3, 1, 0, 0)
	pass.End()
	cmd, err := enc.Finish()
	if err != nil {
		return nil, err
	}
	if _, err := queue.Submit(cmd); err != nil {
		return nil, err
	}
	_ = device.WaitIdle()
	return readTextureViewRegionRGBA(device, queue, packView(view), image.Rect(0, 0, bw, bh), bw, bh)
}

// fillPaintFilteredAsImage renders a paint that carries P3.4 paint-level
// filters: content raster (brush/shader over the path) → MaskFilter blur
// (soften edges) → ImageFilter graph → mask/clip composite → blit. It is the
// filtered-paint twin of fillMaskedAsImage and rides the same R8 modulation
// and QueueImageDraw plumbing.
func (rc *GPURenderContext) fillPaintFilteredAsImage(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	if paint == nil {
		return render.ErrFallbackToCPU
	}
	hasImageFilter := len(paint.ImageFilter) > 0
	hasMaskBlur := paint.MaskFilterRadius > 0
	if !hasImageFilter && !hasMaskBlur {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(bb.Min.X) - 1
		y0 := int(bb.Min.Y) - 1
		x1 := int(bb.Max.X) + 1
		y1 := int(bb.Max.Y) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	// Skia mask-filter semantics: the blurred coverage extends beyond the
	// geometry — expand the staging bounds by the blur pad so the outward
	// halo is not clipped at the shape bounds.
	if paint.MaskFilterRadius > 0 {
		pad := int(math.Ceil(3 * paint.MaskFilterRadius))
		bounds = image.Rect(bounds.Min.X-pad, bounds.Min.Y-pad, bounds.Max.X+pad, bounds.Max.Y+pad).
			Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 || bw*bh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	localPath := path
	if bounds.Min.X != 0 || bounds.Min.Y != 0 {
		localPath = path.Transform(render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)))
	}
	pm := render.NewPixmap(bw, bh)
	sr := render.NewSoftwareRenderer(bw, bh)
	sr.SetAntiAlias(rc.antiAlias)
	local := paint.Clone()
	local.ImageFilter = nil
	local.MaskFilterRadius = 0
	local.MaskCoverage = nil
	local.ClipCoverage = nil
	if err := sr.Fill(pm, localPath, local); err != nil {
		return render.ErrFallbackToCPU
	}
	pm.NotifyPixelsChanged()

	content := make([]byte, bw*bh*4)
	copy(content, pm.Data())

	// MaskFilter: soften the content edges (premul blur keeps the storage
	// invariant; the mask-chain A8 preprocessing refinement is registered).
	if hasMaskBlur {
		// The blur runs through the registered filter graph (GPU first via
		// the pixel hook, CPU fallback inside); premul blur keeps storage
		// invariant. The mask-chain A8 preprocessing refinement is a
		// registered follow-up.
		if !c_tryPaintFilterGraph(rc, content, bw, bh, []render.ImageFilterNode{
			{Kind: render.ImageFilterBlurXY, RadiusX: paint.MaskFilterRadius, RadiusY: paint.MaskFilterRadius},
		}) {
			return render.ErrFallbackToCPU
		}
	}

	// ImageFilter graph: GPU hook first, CPU sequential fallback.
	if hasImageFilter {
		if !c_tryPaintFilterGraph(rc, content, bw, bh, paint.ImageFilter) {
			return render.ErrFallbackToCPU
		}
	}

	maskRegion := make([]byte, bw*bh)
	any := false
	for row := 0; row < bh; row++ {
		py := bounds.Min.Y + row
		for col := 0; col < bw; col++ {
			px := bounds.Min.X + col
			var m uint8 = 255
			if plane, pw, ph, ok := rc.shared.MaskPlane(); ok && pw == tw && ph == th {
				m = plane[py*tw+px]
			} else if maskFn := paint.MaskCoverage; maskFn != nil {
				m = maskFn(px, py)
			}
			maskRegion[row*bw+col] = m
			if content[row*bw*4+col*4+3] != 0 && m != 0 {
				any = true
			}
		}
	}
	if !any {
		return nil
	}
	if bw*bh <= 128*1024 {
		maskPreMulRGBAInPlace(content, maskRegion)
	} else {
		out, err := maskR8Modulate(rc.shared.device, rc.shared.queue, &rc.shared.maskR8, content, maskRegion, bw, bh)
		if err != nil {
			return render.ErrFallbackToCPU
		}
		content = out
	}

	fx0 := float32(bounds.Min.X)
	fy0 := float32(bounds.Min.Y)
	fx1 := float32(bounds.Max.X)
	fy1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, content, 0, nil, render.MipmapNone, bw, bh, bw*4,
		fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
		1.0, uint32(tw), uint32(th), 0, 0, 1, 1, false, false)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// c_tryPaintFilterGraph runs a filter node sequence over a premul RGBA
// buffer through the registered GPU hooks (pixel read-back variant), with
// the CPU sequential fallback handled by the caller's buffer walk. It
// bridges to render's registered gpuFilterGraphApply via the shared hook
// table (the same hooks ApplyImageFilterGraph uses).
func c_tryPaintFilterGraph(rc *GPURenderContext, content []byte, bw, bh int, nodes []render.ImageFilterNode) bool {
	hooks := render.RegisteredGPUFilterGraphHooks()
	if hooks.Apply == nil {
		return false
	}
	out, err := hooks.Apply(content, bw, bh, nodes)
	if err != nil {
		return false
	}
	copy(content, out)
	return true
}
