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
	"sync"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// recycleCommandList returns a consumed command slice backing to the pool.
// Headers must already be cleared (all nil): Submit nils each header as it
// releases the command, Destroy callers have no live references.
func recycleCommandList(cmds []Command) {
	if cmds == nil || cap(cmds) > maxRecycledCommandList {
		return
	}
	for i := range cmds {
		cmds[i] = nil
	}
	commandListPool.Put(cmds[:0])
}

// acquireCommandList takes a recycled command slice backing.
// acquireCommandList takes a recycled command slice backing.
func acquireCommandList() []Command {
	return commandListPool.Get().([]Command) //nolint:forcetypeassert
}

// CommandEncoder implements hal.CommandEncoder for OpenGL.
// Platform-specific fields are defined in command_<platform>.go files.
type CommandEncoder struct {
	glCtx           *gl.Context
	commands        []Command
	label           string
	vao             uint32 // persistent VAO from Device for Core Profile
	maxTextureUnits int32  // Hardware limit passed from Device
}

// Hot-command pools: the per-draw command structs below are recorded every
// frame (hundreds per submit) and must not heap-allocate per record.
// Stdlib parity: sync.Pool per-request objects (net/http, encoding/json
// buffer pools). Each pool holds *T with all fields caller-initialized at
// acquire — acquire helpers take every field, so no stale payload can leak
// across frames. Release happens once, in Queue.Submit after Execute (and in
// DiscardEncoding for never-submitted records); CommandBuffer.Destroy only
// drops headers. Same-OS-thread only (GL context affinity).
var (
	drawCommandPool             = sync.Pool{New: func() any { return &DrawCommand{} }}
	drawIndexedCommandPool      = sync.Pool{New: func() any { return &DrawIndexedCommand{} }}
	setBindGroupCommandPool     = sync.Pool{New: func() any { return &SetBindGroupCommand{} }}
	setVertexBufferCommandPool  = sync.Pool{New: func() any { return &SetVertexBufferCommand{} }}
	useProgramCommandPool       = sync.Pool{New: func() any { return &UseProgramCommand{} }}
	setPipelineStateCommandPool = sync.Pool{New: func() any { return &SetPipelineStateCommand{} }}
)

// acquireDrawCommand takes a pooled DrawCommand with all fields set.
// acquireDrawCommand takes a pooled DrawCommand with all fields set.
func acquireDrawCommand(vertexCount, instanceCount, firstVertex, firstInstance uint32, topology gputypes.PrimitiveTopology) *DrawCommand {
	c := drawCommandPool.Get().(*DrawCommand) //nolint:forcetypeassert
	c.vertexCount = vertexCount
	c.instanceCount = instanceCount
	c.firstVertex = firstVertex
	c.firstInstance = firstInstance
	c.topology = topology
	return c
}

// acquireDrawIndexedCommand takes a pooled DrawIndexedCommand with all fields set.
// acquireDrawIndexedCommand takes a pooled DrawIndexedCommand with all fields set.
func acquireDrawIndexedCommand(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32, indexFormat gputypes.IndexFormat, topology gputypes.PrimitiveTopology) *DrawIndexedCommand {
	c := drawIndexedCommandPool.Get().(*DrawIndexedCommand) //nolint:forcetypeassert
	c.indexCount = indexCount
	c.instanceCount = instanceCount
	c.firstIndex = firstIndex
	c.baseVertex = baseVertex
	c.firstInstance = firstInstance
	c.indexFormat = indexFormat
	c.topology = topology
	return c
}

// acquireSetBindGroupCommand takes a pooled SetBindGroupCommand with all fields set.
// acquireSetBindGroupCommand takes a pooled SetBindGroupCommand with all fields set.
func acquireSetBindGroupCommand(index uint32, group *BindGroup, offsets []uint32, maxUnits int32, infos []BindGroupLayoutInfo, smap *[maxTextureSlots]int8) *SetBindGroupCommand {
	c := setBindGroupCommandPool.Get().(*SetBindGroupCommand) //nolint:forcetypeassert
	c.index = index
	c.group = group
	c.dynamicOffsets = offsets
	c.maxTextureUnits = maxUnits
	c.groupInfos = infos
	c.samplerBindMap = smap
	return c
}

// acquireSetVertexBufferCommand takes a pooled SetVertexBufferCommand with all fields set.
// acquireSetVertexBufferCommand takes a pooled SetVertexBufferCommand with all fields set.
func acquireSetVertexBufferCommand(slot uint32, buffer *Buffer, offset uint64, layout *gputypes.VertexBufferLayout) *SetVertexBufferCommand {
	c := setVertexBufferCommandPool.Get().(*SetVertexBufferCommand) //nolint:forcetypeassert
	c.slot = slot
	c.buffer = buffer
	c.offset = offset
	c.layout = layout
	return c
}

// acquireUseProgramCommand takes a pooled UseProgramCommand with all fields set.
// acquireUseProgramCommand takes a pooled UseProgramCommand with all fields set.
func acquireUseProgramCommand(programID uint32) *UseProgramCommand {
	c := useProgramCommandPool.Get().(*UseProgramCommand) //nolint:forcetypeassert
	c.programID = programID
	return c
}

// acquireSetPipelineStateCommand takes a pooled SetPipelineStateCommand with all fields set.
// acquireSetPipelineStateCommand takes a pooled SetPipelineStateCommand with all fields set.
func acquireSetPipelineStateCommand(topology gputypes.PrimitiveTopology, cullMode gputypes.CullMode, frontFace gputypes.FrontFace, depthStencil *hal.DepthStencilState, colorTargets []ColorTargetDesc, stencilRef uint32) *SetPipelineStateCommand {
	c := setPipelineStateCommandPool.Get().(*SetPipelineStateCommand) //nolint:forcetypeassert
	c.topology = topology
	c.cullMode = cullMode
	c.frontFace = frontFace
	c.depthStencil = depthStencil
	c.colorTargets = colorTargets
	c.stencilRef = stencilRef
	return c
}

// releasePooledCommand returns a hot-path command to its pool after Execute.
// Unknown (cold-path) types are dropped for the GC: pooling covers only the
// six per-draw types above. Field clearing is unnecessary — every acquire
// helper overwrites all fields — except reference fields, which are cleared
// here so a pooled struct never pins buffers/groups/pipelines across frames.
// releasePooledCommand returns a hot-path command to its pool after Execute.
// Unknown (cold-path) types are dropped for the GC: pooling covers only the
// six per-draw types above. Field clearing is unnecessary — every acquire
// helper overwrites all fields — except reference fields, which are cleared
// here so a pooled struct never pins buffers/groups/pipelines across frames.
func releasePooledCommand(cmd Command) {
	switch c := cmd.(type) {
	case *DrawCommand:
		drawCommandPool.Put(c)
	case *DrawIndexedCommand:
		drawIndexedCommandPool.Put(c)
	case *SetBindGroupCommand:
		c.group = nil
		c.dynamicOffsets = nil
		c.groupInfos = nil
		c.samplerBindMap = nil
		setBindGroupCommandPool.Put(c)
	case *SetVertexBufferCommand:
		c.buffer = nil
		c.layout = nil
		setVertexBufferCommandPool.Put(c)
	case *UseProgramCommand:
		useProgramCommandPool.Put(c)
	case *SetPipelineStateCommand:
		c.depthStencil = nil
		c.colorTargets = nil
		setPipelineStateCommandPool.Put(c)
	}
}

// CommandEncoder pool: per-frame recording reuses backing instead of
// regrowing the command slice every frame. Stdlib parity: sync.Pool for
// per-request scratch (net/http bufio reuse) — Get/Put own the object while
// checked out; Submit/Finish returns it. Same-OS-thread only (GL context
// affinity): take/return run on the raster/encode thread, no cross-thread
// sharing while recording.
var commandEncoderPool = sync.Pool{
	New: func() any { return &CommandEncoder{} },
}

// acquireCommandEncoder takes a pooled encoder and resets it for recording.
// The backing (commands slice) is reused — either the encoder's own kept
// backing or a command-list pool take — so steady frames stay alloc-free.
// acquireCommandEncoder takes a pooled encoder and resets it for recording.
// The backing (commands slice) is reused — either the encoder's own kept
// backing or a command-list pool take — so steady frames stay alloc-free.
func acquireCommandEncoder() *CommandEncoder {
	e := commandEncoderPool.Get().(*CommandEncoder) //nolint:forcetypeassert
	if e.commands == nil {
		e.commands = acquireCommandList()
	} else {
		e.commands = e.commands[:0]
	}
	return e
}

// releaseCommandEncoder returns an encoder after Finish/Discard: clears the
// headers (interface words only — pointed-to payloads belong to the frame
// and are released with their own owners) and keeps the backing.
// releaseCommandEncoder returns an encoder after Finish/Discard: clears the
// headers (interface words only — pointed-to payloads belong to the frame
// and are released with their own owners) and keeps the backing.
func releaseCommandEncoder(e *CommandEncoder) {
	if e == nil {
		return
	}
	for i := range e.commands {
		e.commands[i] = nil
	}
	e.commands = e.commands[:0]
	e.label = ""
	e.glCtx = nil
	e.vao = 0
	e.maxTextureUnits = 0
	commandEncoderPool.Put(e)
}

// BeginEncoding begins command recording.
// BeginEncoding begins command recording.
func (e *CommandEncoder) BeginEncoding(label string) error {
	e.label = label
	e.commands = e.commands[:0]
	return nil
}

// EndEncoding finishes command recording and returns a command buffer.
// EndEncoding finishes command recording and returns a command buffer.
func (e *CommandEncoder) EndEncoding() (hal.CommandBuffer, error) {
	cmdBuf := &CommandBuffer{
		commands: e.commands,
	}
	e.commands = nil
	releaseCommandEncoder(e)
	return cmdBuf, nil
}

// Finish implements hal.CommandEncoder: same as EndEncoding.
// Finish implements hal.CommandEncoder: same as EndEncoding.
func (e *CommandEncoder) Finish() (hal.CommandBuffer, error) {
	return e.EndEncoding()
}

// DiscardEncoding discards the encoder.
// DiscardEncoding discards the encoder.
func (e *CommandEncoder) DiscardEncoding() {
	e.commands = e.commands[:0]
	releaseCommandEncoder(e)
}

// ResetAll resets command buffers for reuse.
// ResetAll resets command buffers for reuse.
func (e *CommandEncoder) ResetAll(_ []hal.CommandBuffer) {
	// No-op for OpenGL
}

// Destroy is a no-op for OpenGL (no persistent GPU resources owned by encoder).
// Destroy is a no-op for OpenGL (no persistent GPU resources owned by encoder).
func (e *CommandEncoder) Destroy() {}

// TransitionBuffers emits memory barriers for buffer state transitions.
// GLES only needs explicit barriers when transitioning FROM storage write
// (compute shader output) to any other usage (vertex, uniform, draw).
// TransitionBuffers emits memory barriers for buffer state transitions.
// GLES only needs explicit barriers when transitioning FROM storage write
// (compute shader output) to any other usage (vertex, uniform, draw).
func (e *CommandEncoder) TransitionBuffers(barriers []hal.BufferBarrier) {
	var bits uint32
	for _, bar := range barriers {
		if bar.Usage.OldUsage&gputypes.BufferUsageStorage != 0 {
			bits |= gl.SHADER_STORAGE_BARRIER_BIT | gl.BUFFER_UPDATE_BARRIER_BIT |
				gl.VERTEX_ATTRIB_ARRAY_BARRIER_BIT | gl.ELEMENT_ARRAY_BARRIER_BIT |
				gl.UNIFORM_BARRIER_BIT | gl.COMMAND_BARRIER_BIT
		}
	}
	if bits != 0 {
		e.commands = append(e.commands, &MemoryBarrierCommand{barriers: bits})
	}
}

// TransitionTextures emits memory barriers for texture state transitions.
// GLES only needs explicit barriers when transitioning FROM storage write
// to any other usage (texture fetch, framebuffer attachment).
// TransitionTextures emits memory barriers for texture state transitions.
// GLES only needs explicit barriers when transitioning FROM storage write
// to any other usage (texture fetch, framebuffer attachment).
func (e *CommandEncoder) TransitionTextures(barriers []hal.TextureBarrier) {
	var bits uint32
	for _, bar := range barriers {
		if bar.Usage.OldUsage&gputypes.TextureUsageStorageBinding != 0 {
			bits |= gl.TEXTURE_FETCH_BARRIER_BIT | gl.SHADER_IMAGE_ACCESS_BARRIER_BIT |
				gl.FRAMEBUFFER_BARRIER_BIT | gl.TEXTURE_UPDATE_BARRIER_BIT |
				gl.PIXEL_BUFFER_BARRIER_BIT
		}
	}
	if bits != 0 {
		e.commands = append(e.commands, &MemoryBarrierCommand{barriers: bits})
	}
}

// ClearBuffer clears a buffer region to zero.
// ClearBuffer clears a buffer region to zero.
func (e *CommandEncoder) ClearBuffer(buffer hal.Buffer, offset, size uint64) {
	buf, ok := buffer.(*Buffer)
	if !ok {
		return
	}
	e.commands = append(e.commands, &ClearBufferCommand{
		buffer: buf,
		offset: offset,
		size:   size,
	})
}

// CopyBufferToBuffer copies data between buffers.
// CopyBufferToBuffer copies data between buffers.
func (e *CommandEncoder) CopyBufferToBuffer(src hal.Buffer, srcOffset uint64, dst hal.Buffer, dstOffset uint64, size uint64) {
	srcBuf, srcOk := src.(*Buffer)
	dstBuf, dstOk := dst.(*Buffer)
	if !srcOk || !dstOk {
		return
	}

	e.commands = append(e.commands, &CopyBufferCommand{
		srcID:     srcBuf.id,
		srcOffset: srcOffset,
		dstID:     dstBuf.id,
		dstOffset: dstOffset,
		size:      size,
	})
}

// CopyBufferToTexture copies buffer data to a texture via PBO (pixel unpack buffer).
// Binds the source buffer as GL_PIXEL_UNPACK_BUFFER, then calls glTexSubImage2D
// with offset 0 so that GL reads pixel data from the bound PBO.
// CopyBufferToTexture copies buffer data to a texture via PBO (pixel unpack buffer).
// Binds the source buffer as GL_PIXEL_UNPACK_BUFFER, then calls glTexSubImage2D
// with offset 0 so that GL reads pixel data from the bound PBO.
func (e *CommandEncoder) CopyBufferToTexture(src hal.Buffer, dst hal.Texture, regions []hal.BufferTextureCopy) {
	srcBuf, srcOK := src.(*Buffer)
	dstTex, dstOK := dst.(*Texture)
	if !srcOK || !dstOK || srcBuf == nil || dstTex == nil {
		return
	}

	for _, region := range regions {
		e.commands = append(e.commands, &CopyBufferToTextureCommand{
			srcBuffer: srcBuf,
			dstTex:    dstTex,
			origin:    [3]uint32{region.TextureBase.Origin.X, region.TextureBase.Origin.Y, region.TextureBase.Origin.Z},
			copySize:  [3]uint32{region.Size.Width, region.Size.Height, region.Size.DepthOrArrayLayers},
			bufOffset: region.BufferLayout.Offset,
		})
	}
}

// CopyTextureToBuffer copies texture data to a buffer via FBO + glReadPixels.
// For each region, it binds the source texture's FBO and reads pixels into the
// destination buffer's CPU-side data slice.
// CopyTextureToBuffer copies texture data to a buffer via FBO + glReadPixels.
// For each region, it binds the source texture's FBO and reads pixels into the
// destination buffer's CPU-side data slice.
func (e *CommandEncoder) CopyTextureToBuffer(src hal.Texture, dst hal.Buffer, regions []hal.BufferTextureCopy) {
	srcTex, srcOK := src.(*Texture)
	dstBuf, dstOK := dst.(*Buffer)
	if !srcOK || !dstOK {
		return
	}

	for _, region := range regions {
		e.commands = append(e.commands, &CopyTextureToBufferCommand{
			glCtx:       e.glCtx,
			srcTexture:  srcTex,
			dstBuffer:   dstBuf,
			srcOrigin:   [3]uint32{region.TextureBase.Origin.X, region.TextureBase.Origin.Y, region.TextureBase.Origin.Z},
			copySize:    [3]uint32{region.Size.Width, region.Size.Height, region.Size.DepthOrArrayLayers},
			dstOffset:   region.BufferLayout.Offset,
			bytesPerRow: region.BufferLayout.BytesPerRow,
		})
	}
}

// CopyTextureToTexture copies between textures using FBO read + glCopyTexSubImage2D.
// Attaches the source texture to the read framebuffer, then copies pixels into the
// destination texture. This works on GL 3.0+ / ES 3.0+ without glCopyImageSubData.
// CopyTextureToTexture copies between textures using FBO read + glCopyTexSubImage2D.
// Attaches the source texture to the read framebuffer, then copies pixels into the
// destination texture. This works on GL 3.0+ / ES 3.0+ without glCopyImageSubData.
func (e *CommandEncoder) CopyTextureToTexture(src, dst hal.Texture, regions []hal.TextureCopy) {
	srcTex, srcOK := src.(*Texture)
	dstTex, dstOK := dst.(*Texture)
	if !srcOK || !dstOK || srcTex == nil || dstTex == nil {
		return
	}

	for _, region := range regions {
		e.commands = append(e.commands, &CopyTextureToTextureCommand{
			srcTex:    srcTex,
			dstTex:    dstTex,
			srcOrigin: [3]uint32{region.Source.Origin.X, region.Source.Origin.Y, region.Source.Origin.Z},
			dstOrigin: [3]uint32{region.Destination.Origin.X, region.Destination.Origin.Y, region.Destination.Origin.Z},
			copySize:  [3]uint32{region.Size.Width, region.Size.Height, region.Size.DepthOrArrayLayers},
			srcMip:    region.Source.MipLevel,
			dstMip:    region.Destination.MipLevel,
		})
	}
}

// ResolveQuerySet copies query results from a query set into a destination buffer.
// Each result is a uint64 (8 bytes) written starting at destinationOffset.
// Uses glGetQueryObjectui64v to read results, then glBufferSubData to write them.
// Nil or foreign query sets/buffers are ignored (nil-safe).
// ResolveQuerySet copies query results from a query set into a destination buffer.
// Each result is a uint64 (8 bytes) written starting at destinationOffset.
// Uses glGetQueryObjectui64v to read results, then glBufferSubData to write them.
// Nil or foreign query sets/buffers are ignored (nil-safe).
func (e *CommandEncoder) ResolveQuerySet(querySet hal.QuerySet, firstQuery, queryCount uint32, destination hal.Buffer, destinationOffset uint64) {
	qs, qsOK := querySet.(*QuerySet)
	dstBuf, bufOK := destination.(*Buffer)
	if !qsOK || !bufOK || qs == nil || dstBuf == nil {
		return
	}

	e.commands = append(e.commands, &ResolveQuerySetCommand{
		querySet:   qs,
		firstQuery: firstQuery,
		queryCount: queryCount,
		dstBuffer:  dstBuf,
		dstOffset:  destinationOffset,
	})
}

// BuildAccelerationStructures is a no-op (GLES has no ray tracing).
// BuildAccelerationStructures is a no-op (GLES has no ray tracing).
func (e *CommandEncoder) BuildAccelerationStructures(_ []hal.BuildAccelerationStructureDescriptor) {}

// PlaceAccelerationStructureBarrier is a no-op (GLES has no ray tracing).
// PlaceAccelerationStructureBarrier is a no-op (GLES has no ray tracing).
func (e *CommandEncoder) PlaceAccelerationStructureBarrier(_ hal.AccelerationStructureBarrier) {}

// CopyAccelerationStructure is a no-op (GLES has no ray tracing).
// CopyAccelerationStructure is a no-op (GLES has no ray tracing).
func (e *CommandEncoder) CopyAccelerationStructure(_, _ hal.AccelerationStructure, _ gputypes.AccelerationStructureCopyMode) {
}

// ReadAccelerationStructureCompactSize is a no-op (GLES has no ray tracing).
// ReadAccelerationStructureCompactSize is a no-op (GLES has no ray tracing).
func (e *CommandEncoder) ReadAccelerationStructureCompactSize(_ hal.AccelerationStructure, _ hal.Buffer, _ uint64) {
}

// BeginRenderPass begins a render pass.
// BeginRenderPass begins a render pass.
func (e *CommandEncoder) BeginRenderPass(desc *hal.RenderPassDescriptor) (hal.RenderPassEncoder, error) {
	rpe := &RenderPassEncoder{
		encoder: e,
		desc:    desc,
	}

	// Bind the persistent VAO. Core Profile requires a VAO to be bound for any
	// vertex attribute or draw call. Re-binding at pass start ensures it is
	// active even if external code (or a previous pass) unbound it.
	if e.vao != 0 {
		e.commands = append(e.commands, &BindVAOCommand{vao: e.vao})
	}

	// Bind the correct framebuffer and set viewport.
	if desc != nil && len(desc.ColorAttachments) > 0 {
		e.setupColorAttachment(desc, rpe)
	}

	// Set draw buffers for MRT.
	if desc != nil && len(desc.ColorAttachments) > 0 {
		e.commands = append(e.commands, &SetDrawColorBuffersCommand{
			count: len(desc.ColorAttachments),
		})
	}

	// Record per-buffer clear commands. Uses glClearBufferfv for per-target
	// clearing instead of global glClearColor+glClear(GL_COLOR_BUFFER_BIT).
	if desc != nil {
		for i, ca := range desc.ColorAttachments {
			if ca.LoadOp == gputypes.LoadOpClear {
				clearColor := ca.ClearValue
				e.commands = append(e.commands, &ClearColorBufferCommand{
					drawBuffer: int32(i),
					color: [4]float32{
						float32(clearColor.R),
						float32(clearColor.G),
						float32(clearColor.B),
						float32(clearColor.A),
					},
				})
			}
		}

		if desc.DepthStencilAttachment != nil {
			dsa := desc.DepthStencilAttachment
			if dsa.DepthLoadOp == gputypes.LoadOpClear {
				e.commands = append(e.commands, &ClearDepthCommand{
					depth: float64(dsa.DepthClearValue),
				})
			}
			if dsa.StencilLoadOp == gputypes.LoadOpClear {
				e.commands = append(e.commands, &ClearStencilCommand{
					stencil: int32(dsa.StencilClearValue),
				})
			}
		}

		// Emit beginning-of-pass timestamp if requested.
		if desc.TimestampWrites != nil {
			e.emitTimestamp(desc.TimestampWrites.QuerySet, desc.TimestampWrites.BeginningOfPassWriteIndex)
			rpe.endTimestampQuerySet = desc.TimestampWrites.QuerySet
			rpe.endTimestampIndex = desc.TimestampWrites.EndOfPassWriteIndex
		}
	}

	return rpe, nil
}

// setupColorAttachment configures framebuffer, viewport, and MSAA resolve for
// all color attachments of a render pass. For surface targets only attachment[0]
// is used (surfaces are always single-target). For offscreen targets, all
// attachments are bound to GL_COLOR_ATTACHMENT0..N.
// setupColorAttachment configures framebuffer, viewport, and MSAA resolve for
// all color attachments of a render pass. For surface targets only attachment[0]
// is used (surfaces are always single-target). For offscreen targets, all
// attachments are bound to GL_COLOR_ATTACHMENT0..N.
func (e *CommandEncoder) setupColorAttachment(desc *hal.RenderPassDescriptor, rpe *RenderPassEncoder) {
	ca := desc.ColorAttachments[0]
	tv, ok := ca.View.(*TextureView)
	if !ok {
		return
	}

	if tv.isSurface {
		e.setupSurfaceTarget(desc, tv, rpe)
		return
	}

	if tv.texture == nil {
		return
	}

	e.setupOffscreenTarget(desc, tv, rpe)
}

// setupSurfaceTarget binds the Surface's swapchain offscreen framebuffer and
// sets viewport to surface dimensions. The swapchain FBO is a persistent GL
// framebuffer owned by the Surface (allocated in Surface.Configure). User
// render passes target this FBO — never the default framebuffer (FBO 0) —
// because the scene is intentionally rendered upside-down via the in-shader
// Y-flip (WriterFlagAdjustCoordinateSpace). Queue.Present performs an explicit
// Y-flipping glBlitFramebuffer from this FBO to FBO 0 before SwapBuffers.
// setupSurfaceTarget binds the Surface's swapchain offscreen framebuffer and
// sets viewport to surface dimensions. The swapchain FBO is a persistent GL
// framebuffer owned by the Surface (allocated in Surface.Configure). User
// render passes target this FBO — never the default framebuffer (FBO 0) —
// because the scene is intentionally rendered upside-down via the in-shader
// Y-flip (WriterFlagAdjustCoordinateSpace). Queue.Present performs an explicit
// Y-flipping glBlitFramebuffer from this FBO to FBO 0 before SwapBuffers.
func (e *CommandEncoder) setupSurfaceTarget(desc *hal.RenderPassDescriptor, tv *TextureView, rpe *RenderPassEncoder) {
	if tv.surfaceTex != nil && tv.surfaceTex.surface != nil {
		surf := tv.surfaceTex.surface
		e.commands = append(e.commands, &BindSurfaceFramebufferCommand{surface: surf})
		if surf.config != nil {
			cfg := surf.config
			rpe.fbHeight = cfg.Height
			e.commands = append(e.commands, &SetViewportCommand{
				width:    float32(cfg.Width),
				height:   float32(cfg.Height),
				maxDepth: 1,
			})
		}
		// Attach depth/stencil only when it covers the color target (helper
		// keeps the surface and offscreen paths in one place).
		if desc.DepthStencilAttachment != nil {
			if dsView, ok := desc.DepthStencilAttachment.View.(*TextureView); ok && dsView.texture != nil {
				if depthSizeMatches(dsView.texture, surf.fboWidth, surf.fboHeight) {
					e.commands = append(e.commands, &AttachDepthStencilToFBOCommand{
						depthTexture: dsView.texture,
					})
				}
			}
		}
		return
	}
	// Fallback: no surface texture (shouldn't normally happen for isSurface=true
	// views, but preserves prior behavior for tests / degenerate cases).
	e.commands = append(e.commands, &BindFramebufferCommand{fbo: 0})
}

// depthSizeMatches reports whether a depth/stencil texture covers a w×h
// color target. A degraded 1x1 depth serving a full-size target must not
// attach: the pair reports FBO-complete yet silently drops all color draws
// (probe-verified 64x64 color + 1x1 depth clears to zero), so drawing runs
// without depth and depth returns on the next full-size probe. Both the
// surface path (target = swapchain FBO size) and the offscreen path
// (target = color texture size) share it.
// setupOffscreenTarget configures an offscreen FBO with all color attachments,
// depth/stencil attachment, and MSAA resolve.
func (e *CommandEncoder) setupOffscreenTarget(
	desc *hal.RenderPassDescriptor,
	tv *TextureView,
	rpe *RenderPassEncoder,
) {
	e.commands = append(e.commands, &EnsureOffscreenFBOCommand{texture: tv.texture})

	// Attach additional color textures (attachments 1..N) to the FBO.
	// Attachment 0 is already bound by EnsureOffscreenFBOCommand.
	for i := 1; i < len(desc.ColorAttachments); i++ {
		ca := desc.ColorAttachments[i]
		atv, ok := ca.View.(*TextureView)
		if !ok || atv.texture == nil {
			continue
		}
		e.commands = append(e.commands, &AttachColorCommand{
			attachmentIndex: uint32(i),
			texture:         atv.texture,
		})
	}

	// Attach depth/stencil only when it covers the color target (twin of the
	// surface path above).
	if desc.DepthStencilAttachment != nil {
		if dsView, ok := desc.DepthStencilAttachment.View.(*TextureView); ok && dsView.texture != nil {
			if tv.texture != nil && depthSizeMatches(dsView.texture, tv.texture.size.Width, tv.texture.size.Height) {
				e.commands = append(e.commands, &AttachDepthStencilCommand{
					colorTexture: tv.texture,
					depthTexture: dsView.texture,
				})
			}
		}
	}

	rpe.fbHeight = tv.texture.size.Height
	e.commands = append(e.commands, &SetViewportCommand{
		width:    float32(tv.texture.size.Width),
		height:   float32(tv.texture.size.Height),
		maxDepth: 1,
	})

	// Record MSAA resolve target if present (attachment 0 only).
	ca := desc.ColorAttachments[0]
	if resolveView, ok := ca.ResolveTarget.(*TextureView); ok && ca.ResolveTarget != nil {
		if resolveView.texture != nil {
			rpe.msaaTexture = tv.texture
			rpe.resolveTexture = resolveView.texture
		} else if resolveView.isSurface {
			rpe.msaaTexture = tv.texture
			rpe.resolveToSurface = true
		}
	}
}

// BeginComputePass begins a compute pass.
// BeginComputePass begins a compute pass.
func (e *CommandEncoder) BeginComputePass(desc *hal.ComputePassDescriptor) (hal.ComputePassEncoder, error) {
	cpe := &ComputePassEncoder{
		encoder: e,
	}

	// Emit beginning-of-pass timestamp if requested.
	if desc != nil && desc.TimestampWrites != nil {
		e.emitTimestamp(desc.TimestampWrites.QuerySet, desc.TimestampWrites.BeginningOfPassWriteIndex)
		cpe.endTimestampQuerySet = desc.TimestampWrites.QuerySet
		cpe.endTimestampIndex = desc.TimestampWrites.EndOfPassWriteIndex
	}

	return cpe, nil
}

// emitTimestamp records a glQueryCounter command for a timestamp write index.
// Used for render/compute pass beginning/end timestamp queries.
// emitTimestamp records a glQueryCounter command for a timestamp write index.
// Used for render/compute pass beginning/end timestamp queries.
func (e *CommandEncoder) emitTimestamp(querySet hal.QuerySet, index *uint32) {
	if index == nil {
		return
	}
	qs, ok := querySet.(*QuerySet)
	if !ok || qs == nil {
		return
	}
	idx := *index
	if int(idx) >= len(qs.queries) {
		return
	}
	e.commands = append(e.commands, &TimestampQueryCommand{query: qs.queries[idx]})
}

// RenderPassEncoder implements hal.RenderPassEncoder for OpenGL.
type RenderPassEncoder struct {
	encoder       *CommandEncoder
	desc          *hal.RenderPassDescriptor
	pipeline      *RenderPipeline
	vertexBuffers []*Buffer
	vertexOffsets []uint64
	indexBuffer   *Buffer
	indexFormat   gputypes.IndexFormat
	stencilRef    uint32
	fbHeight      uint32 // Framebuffer height for MSAA resolve blit Y-flip

	// MSAA resolve state: set during BeginRenderPass when ResolveTarget is present.
	msaaTexture      *Texture // The MSAA color texture (source for resolve)
	resolveTexture   *Texture // The single-sample resolve target (nil when resolveToSurface)
	resolveToSurface bool     // True when resolve target is the default framebuffer (FBO 0)

	// End-of-pass timestamp write (deferred to End()).
	endTimestampQuerySet hal.QuerySet
	endTimestampIndex    *uint32

	// B2 录制侧去重（webgpu renderpass.go 对等规则）：完全重复才跳过，
	// 像素不可能变。模板/混合只在值变时下发，共享状态语义不动。
	// 三组：当前管线；槽位绑定（组/顶点/索引）；纯值状态（视口/裁剪/混合/模板）。
	curPipeline *RenderPipeline
	boundGroups [4]glBindGroupEntry
	boundVerts  [8]glBoundVertEntry
	indexEntry  glIndexEntry
	stateVals   glStateVals
}

// glBindGroupEntry 记住一个槽位的绑定（webgpu boundGroupEntry 对等）。
// 管线也记：同组在不同管线下烘出的采样映射不同，换管线必须重发。
type glBindGroupEntry struct {
	set     bool
	group   *BindGroup
	pipe    *RenderPipeline
	offsets []uint32
}

// glBoundVertEntry 记住一个槽位的顶点缓冲（webgpu boundVertEntry + layout 对等）。
type glBoundVertEntry struct {
	set    bool
	buf    *Buffer
	offset uint64
	layout *gputypes.VertexBufferLayout
}

// glIndexEntry 记住索引缓冲。
type glIndexEntry struct {
	set    bool
	buf    *Buffer
	format gputypes.IndexFormat
	offset uint64
}

// glStateVals 纯值状态去重（webgpu viewport/scissor/stencil/blend 对等）。
// 管线/布局引用放在槽位键里（换管线重发），这里只记值。
type glStateVals struct {
	viewport   [6]float32
	viewportOK bool
	scissor    [4]uint32
	scissorOK  bool
	blend      [4]float32
	blendOK    bool
	stencilRef uint32
	stencilDS  *hal.DepthStencilState
	stencilOK  bool
}
