//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gles

import (
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// depthSizeMatches reports whether a depth/stencil texture covers a w×h
// color target. A degraded 1x1 depth serving a full-size target must not
// attach: the pair reports FBO-complete yet silently drops all color draws
// (probe-verified 64x64 color + 1x1 depth clears to zero), so drawing runs
// without depth and depth returns on the next full-size probe. Both the
// surface path (target = swapchain FBO size) and the offscreen path
// (target = color texture size) share it.
func depthSizeMatches(tex *Texture, w, h uint32) bool {
	if tex == nil {
		return false
	}
	return tex.size.Width == w && tex.size.Height == h
}

// setupOffscreenTarget configures an offscreen FBO with all color attachments,
// depth/stencil attachment, and MSAA resolve.
func glOffsetsEqual(a, b []uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// resolveTargetSurface extracts the *Surface owning the resolve target (if the
// resolve target is a surface texture view). Returns nil otherwise.
// resolveTargetSurface extracts the *Surface owning the resolve target (if the
// resolve target is a surface texture view). Returns nil otherwise.
func (e *RenderPassEncoder) resolveTargetSurface() *Surface {
	if len(e.desc.ColorAttachments) == 0 {
		return nil
	}
	rv, ok := e.desc.ColorAttachments[0].ResolveTarget.(*TextureView)
	if !ok || rv == nil || rv.surfaceTex == nil {
		return nil
	}
	return rv.surfaceTex.surface
}

// emitMSAAResolve appends the appropriate MSAAResolveCommand when the pass has
// an MSAA resolve target. Must only be called when e.msaaTexture != nil.
// emitMSAAResolve appends the appropriate MSAAResolveCommand when the pass has
// an MSAA resolve target. Must only be called when e.msaaTexture != nil.
func (e *RenderPassEncoder) emitMSAAResolve() {
	w := int32(e.msaaTexture.size.Width)
	h := int32(e.msaaTexture.size.Height)
	if e.resolveToSurface {
		// Resolve into the Surface's swapchain offscreen FBO (not FBO 0).
		// The Y-flip happens once at Present time (Queue.Present), not here.
		e.encoder.commands = append(e.encoder.commands, &MSAAResolveCommand{
			msaaTexture:      e.msaaTexture,
			resolveToSurface: true,
			surface:          e.resolveTargetSurface(),
			width:            w,
			height:           h,
		})
		return
	}
	if e.resolveTexture != nil {
		// Resolve into an offscreen texture FBO.
		e.encoder.commands = append(e.encoder.commands, &MSAAResolveCommand{
			msaaTexture:    e.msaaTexture,
			resolveTexture: e.resolveTexture,
			width:          w,
			height:         h,
		})
	}
}

// End finishes the render pass.
// If MSAA resolve is needed, blits the MSAA FBO to the resolve target FBO.
// If the pass was rendering to an offscreen FBO, rebinds the default framebuffer
// so subsequent operations do not accidentally target the offscreen texture.
// End finishes the render pass.
// If MSAA resolve is needed, blits the MSAA FBO to the resolve target FBO.
// If the pass was rendering to an offscreen FBO, rebinds the default framebuffer
// so subsequent operations do not accidentally target the offscreen texture.
func (e *RenderPassEncoder) End() error {
	if e.msaaTexture != nil {
		e.emitMSAAResolve()
	}

	// Emit end-of-pass timestamp if requested.
	if e.endTimestampIndex != nil {
		e.encoder.emitTimestamp(e.endTimestampQuerySet, e.endTimestampIndex)
	}

	// Check if we were rendering to an offscreen target.
	if e.desc != nil && len(e.desc.ColorAttachments) > 0 {
		if tv, ok := e.desc.ColorAttachments[0].View.(*TextureView); ok {
			if !tv.isSurface && tv.texture != nil {
				e.encoder.commands = append(e.encoder.commands, &BindFramebufferCommand{fbo: 0})
			}
		}
	}
	return nil
}

// SetPipeline sets the render pipeline. Same pointer twice emits once
// (webgpu SetPipeline 对等：顶点重打也是同一布局，跳过无损).
// SetPipeline sets the render pipeline. Same pointer twice emits once
// (webgpu SetPipeline 对等：顶点重打也是同一布局，跳过无损).
func (e *RenderPassEncoder) SetPipeline(pipeline hal.RenderPipeline) {
	p, ok := pipeline.(*RenderPipeline)
	if !ok {
		return
	}
	if p == e.curPipeline {
		return
	}
	e.curPipeline = p
	e.pipeline = p
	e.encoder.commands = append(e.encoder.commands,
		acquireUseProgramCommand(p.programID),
		acquireSetPipelineStateCommand(
			p.primitiveTopology, p.cullMode, p.frontFace,
			p.depthStencil, p.colorTargets, e.stencilRef,
		))
	// Vertex setup follows the consuming pipeline: tiers bind the buffer
	// before switching pipelines, so re-emit with this pipeline's layout.
	for slot, buf := range e.vertexBuffers {
		if buf == nil {
			continue
		}
		var layout *gputypes.VertexBufferLayout
		if slot < len(p.vertexBuffers) {
			layout = &p.vertexBuffers[slot]
		}
		var off uint64
		if slot < len(e.vertexOffsets) {
			off = e.vertexOffsets[slot]
		}
		e.encoder.commands = append(e.encoder.commands, acquireSetVertexBufferCommand(
			uint32(slot), buf, off, layout))
	}
}

// SetBindGroup sets a bind group. Exact duplicates on the same index emit
// once (webgpu SetBindGroup 对等）；显式空组永远下发，不记缓存.
// SetBindGroup sets a bind group. Exact duplicates on the same index emit
// once (webgpu SetBindGroup 对等）；显式空组永远下发，不记缓存.
func (e *RenderPassEncoder) SetBindGroup(index uint32, group hal.BindGroup, offsets []uint32) {
	bg, ok := group.(*BindGroup)
	if !ok {
		return
	}
	if index < uint32(len(e.boundGroups)) {
		if bg == nil {
			e.boundGroups[index] = glBindGroupEntry{}
		} else if prev := &e.boundGroups[index]; prev.set && prev.group == bg && prev.pipe == e.pipeline && glOffsetsEqual(prev.offsets, offsets) {
			return
		} else {
			e.boundGroups[index] = glBindGroupEntry{set: true, group: bg, pipe: e.pipeline}
			if len(offsets) != 0 {
				e.boundGroups[index].offsets = append(e.boundGroups[index].offsets[:0], offsets...)
			}
		}
	}
	var samplerMap *[maxTextureSlots]int8
	var groupInfos []BindGroupLayoutInfo
	if e.pipeline != nil {
		samplerMap = &e.pipeline.samplerBindMap
		if e.pipeline.layout != nil {
			groupInfos = e.pipeline.layout.groupInfos
		}
	}
	e.encoder.commands = append(e.encoder.commands, acquireSetBindGroupCommand(
		index, bg, offsets, e.encoder.maxTextureUnits, groupInfos, samplerMap))
}

// SetVertexBuffer sets a vertex buffer and configures vertex attributes.
// In OpenGL, vertex attribute configuration (glVertexAttribPointer +
// glEnableVertexAttribArray) must be done explicitly. The layout is taken
// from the currently bound render pipeline's vertex buffer descriptors.
// The offset is remembered so a later SetPipeline can re-emit the setup
// with the new pipeline's layout (see SetPipeline).
// SetVertexBuffer sets a vertex buffer and configures vertex attributes.
// In OpenGL, vertex attribute configuration (glVertexAttribPointer +
// glEnableVertexAttribArray) must be done explicitly. The layout is taken
// from the currently bound render pipeline's vertex buffer descriptors.
// The offset is remembered so a later SetPipeline can re-emit the setup
// with the new pipeline's layout (see SetPipeline).
func (e *RenderPassEncoder) SetVertexBuffer(slot uint32, buffer hal.Buffer, offset uint64) {
	buf, ok := buffer.(*Buffer)
	if !ok {
		return
	}

	// Grow slice if needed
	for len(e.vertexBuffers) <= int(slot) {
		e.vertexBuffers = append(e.vertexBuffers, nil)
	}
	for len(e.vertexOffsets) <= int(slot) {
		e.vertexOffsets = append(e.vertexOffsets, 0)
	}
	e.vertexBuffers[slot] = buf
	e.vertexOffsets[slot] = offset

	// Get vertex layout from the current pipeline for this slot.
	var layout *gputypes.VertexBufferLayout
	if e.pipeline != nil && int(slot) < len(e.pipeline.vertexBuffers) {
		layout = &e.pipeline.vertexBuffers[slot]
	}

	// 同槽位同缓冲同偏移同布局只发一次（布局跟着管线走，管线变则布局变，不误跳）.
	if slot < uint32(len(e.boundVerts)) {
		if prev := &e.boundVerts[slot]; prev.set && prev.buf == buf && prev.offset == offset && prev.layout == layout {
			return
		}
		e.boundVerts[slot] = glBoundVertEntry{set: true, buf: buf, offset: offset, layout: layout}
	}

	e.encoder.commands = append(e.encoder.commands, acquireSetVertexBufferCommand(
		slot, buf, offset, layout))
}

// SetIndexBuffer sets the index buffer. Same buffer+format+offset emits once.
// SetIndexBuffer sets the index buffer. Same buffer+format+offset emits once.
func (e *RenderPassEncoder) SetIndexBuffer(buffer hal.Buffer, format gputypes.IndexFormat, offset uint64) {
	buf, ok := buffer.(*Buffer)
	if !ok {
		return
	}
	if e.indexEntry.set && e.indexEntry.buf == buf && e.indexEntry.format == format && e.indexEntry.offset == offset {
		return
	}
	e.indexEntry = glIndexEntry{set: true, buf: buf, format: format, offset: offset}
	e.indexBuffer = buf
	e.indexFormat = format

	e.encoder.commands = append(e.encoder.commands, &SetIndexBufferCommand{
		buffer: buf,
		format: format,
		offset: offset,
	})
}

// SetViewport sets the viewport. Identical values emit once.
// SetViewport sets the viewport. Identical values emit once.
func (e *RenderPassEncoder) SetViewport(x, y, width, height, minDepth, maxDepth float32) {
	vp := [6]float32{x, y, width, height, minDepth, maxDepth}
	if e.stateVals.viewportOK && e.stateVals.viewport == vp {
		return
	}
	e.stateVals.viewport, e.stateVals.viewportOK = vp, true
	e.encoder.commands = append(e.encoder.commands, &SetViewportCommand{
		x: x, y: y, width: width, height: height,
		minDepth: minDepth, maxDepth: maxDepth,
	})
}

// SetScissorRect sets the scissor rectangle.
// With ADJUST_COORDINATE_SPACE, no Y-flip is needed — coordinates pass through directly.
// Identical values emit once.
// SetScissorRect sets the scissor rectangle.
// With ADJUST_COORDINATE_SPACE, no Y-flip is needed — coordinates pass through directly.
// Identical values emit once.
func (e *RenderPassEncoder) SetScissorRect(x, y, width, height uint32) {
	box := [4]uint32{x, y, width, height}
	if e.stateVals.scissorOK && e.stateVals.scissor == box {
		return
	}
	e.stateVals.scissor, e.stateVals.scissorOK = box, true
	e.encoder.commands = append(e.encoder.commands, &SetScissorCommand{
		x: x, y: y, width: width, height: height,
	})
}

// SetBlendConstant sets the blend constant. Identical values emit once.
// SetBlendConstant sets the blend constant. Identical values emit once.
func (e *RenderPassEncoder) SetBlendConstant(color *gputypes.Color) {
	c := [4]float32{float32(color.R), float32(color.G), float32(color.B), float32(color.A)}
	if e.stateVals.blendOK && e.stateVals.blend == c {
		return
	}
	e.stateVals.blend, e.stateVals.blendOK = c, true
	e.encoder.commands = append(e.encoder.commands, &SetBlendConstantCommand{
		r: float32(color.R),
		g: float32(color.G),
		b: float32(color.B),
		a: float32(color.A),
	})
}

// SetStencilReference sets the stencil reference value. Same ref on the same
// depth-stencil state emits once; ref 或管线变都照发，模板语义不动。
// SetStencilReference sets the stencil reference value. Same ref on the same
// depth-stencil state emits once; ref 或管线变都照发，模板语义不动。
func (e *RenderPassEncoder) SetStencilReference(ref uint32) {
	var ds *hal.DepthStencilState
	if e.pipeline != nil {
		ds = e.pipeline.depthStencil
	}
	e.stencilRef = ref
	if e.stateVals.stencilOK && e.stateVals.stencilRef == ref && e.stateVals.stencilDS == ds {
		return
	}
	e.stateVals.stencilRef, e.stateVals.stencilDS, e.stateVals.stencilOK = ref, ds, true
	e.encoder.commands = append(e.encoder.commands, &SetStencilRefCommand{
		ref:          ref,
		depthStencil: ds,
	})
}

// Draw draws primitives.
// Draw draws primitives.
func (e *RenderPassEncoder) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	topology := gputypes.PrimitiveTopologyTriangleList // default
	if e.pipeline != nil {
		topology = e.pipeline.primitiveTopology
	}
	e.encoder.commands = append(e.encoder.commands, acquireDrawCommand(
		vertexCount, instanceCount, firstVertex, firstInstance, topology))
}

// DrawIndexed draws indexed primitives.
// DrawIndexed draws indexed primitives.
func (e *RenderPassEncoder) DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	topology := gputypes.PrimitiveTopologyTriangleList // default
	if e.pipeline != nil {
		topology = e.pipeline.primitiveTopology
	}
	e.encoder.commands = append(e.encoder.commands, acquireDrawIndexedCommand(
		indexCount, instanceCount, firstIndex, baseVertex, firstInstance,
		e.indexFormat, topology))
}

// DrawIndirect draws a single indirect record.
// DrawIndirect draws a single indirect record.
func (e *RenderPassEncoder) DrawIndirect(buffer hal.Buffer, offset uint64) {
	_ = buffer
	_ = offset
}

// DrawIndexedIndirect draws a single indexed indirect record.
// DrawIndexedIndirect draws a single indexed indirect record.
func (e *RenderPassEncoder) DrawIndexedIndirect(buffer hal.Buffer, offset uint64) {
	_ = buffer
	_ = offset
}

// DrawIndirectCount is not supported by the GLES backend.
// DrawIndirectCount is not supported by the GLES backend.
func (e *RenderPassEncoder) DrawIndirectCount(_ hal.Buffer, _ uint64, _ hal.Buffer, _ uint64, _ uint32) {
}

// DrawIndexedIndirectCount is not supported by the GLES backend.
// DrawIndexedIndirectCount is not supported by the GLES backend.
func (e *RenderPassEncoder) DrawIndexedIndirectCount(_ hal.Buffer, _ uint64, _ hal.Buffer, _ uint64, _ uint32) {
}

// ExecuteBundle executes a pre-recorded render bundle.
// Note: Render bundles are not natively supported in OpenGL.
// OpenGL uses display lists (deprecated) or VAO/VBO state caching.
// This is a no-op - bundles are expanded inline in the command stream.
// Nil bundles are ignored (nil-safe).
// ExecuteBundle executes a pre-recorded render bundle.
// Note: Render bundles are not natively supported in OpenGL.
// OpenGL uses display lists (deprecated) or VAO/VBO state caching.
// This is a no-op - bundles are expanded inline in the command stream.
// Nil bundles are ignored (nil-safe).
func (e *RenderPassEncoder) ExecuteBundle(bundle hal.RenderBundle) {
	_ = bundle
}

// ComputePassEncoder implements hal.ComputePassEncoder for OpenGL.
type ComputePassEncoder struct {
	encoder  *CommandEncoder
	pipeline *ComputePipeline

	// End-of-pass timestamp write (deferred to End()).
	endTimestampQuerySet hal.QuerySet
	endTimestampIndex    *uint32
}

// End finishes the compute pass.
