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
	"context"
	"log/slog"
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Destroy releases the command buffer resources.
func (c *CommandBuffer) Destroy() {
	recycleCommandList(c.commands)
	c.commands = nil
}

// commandListPool recycles recorded-command slice backings across submits.
// Rationale: EndEncoding hands the encoder's slice to the buffer, so without
// a pool the backing regrows every frame (profile: Draw/SetBindGroup/SetPipe
// append lines dominate steady alloc). Stdlib parity: sync.Pool scratch.
// Same backing never aliases two live buffers: recycle happens after Execute
// consumed every header (Submit nils as it releases) or in Destroy.
var commandListPool = sync.Pool{
	New: func() any { return make([]Command, 0, 256) },
}

// maxRecycledCommandList caps pooled backing so an outlier frame (huge
// one-off record) can't pin unbounded memory across frames.
const maxRecycledCommandList = 4096

// recycleCommandList returns a consumed command slice backing to the pool.
// Headers must already be cleared (all nil): Submit nils each header as it
// releases the command, Destroy callers have no live references.
func (c *MemoryBarrierCommand) Execute(ctx *gl.Context, st *glExecState) {
	ctx.MemoryBarrier(c.barriers)
}

// ClearBufferCommand clears a buffer region.
type ClearBufferCommand struct {
	buffer *Buffer
	offset uint64
	size   uint64
}

func (c *ClearBufferCommand) Execute(_ *gl.Context, _ *glExecState) {
	// Note: glClearBufferSubData requires GL 4.3+ / GLES 3.1+.
	// For older versions, map buffer and memset, or use compute shader.
}

// BindVAOCommand binds a vertex array object.
type BindVAOCommand struct {
	vao uint32
}

func (c *BindVAOCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.bindVAO(c.vao) {
		ctx.BindVertexArray(c.vao)
	}
}

// BindFramebufferCommand binds a framebuffer object.
type BindFramebufferCommand struct {
	fbo uint32
}

func (c *BindFramebufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.bindFramebuffer(c.fbo) {
		ctx.BindFramebuffer(gl.FRAMEBUFFER, c.fbo)
	}
}

// BindSurfaceFramebufferCommand binds the Surface's swapchain offscreen
// framebuffer. Reads surface.swapchainFBO at execute time so reconfigure
// (e.g. window resize) between encode and submit is handled correctly.
type BindSurfaceFramebufferCommand struct {
	surface *Surface
}

func (c *BindSurfaceFramebufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.surface == nil {
		if st.bindFramebuffer(0) {
			ctx.BindFramebuffer(gl.FRAMEBUFFER, 0)
		}
		return
	}
	if st.bindFramebuffer(c.surface.swapchainFBO) {
		ctx.BindFramebuffer(gl.FRAMEBUFFER, c.surface.swapchainFBO)
	}
}

// EnsureOffscreenFBOCommand lazily creates a framebuffer object for an offscreen
// texture and binds it. If the texture already has an FBO, it simply binds it.
type EnsureOffscreenFBOCommand struct {
	texture *Texture
}

func (c *EnsureOffscreenFBOCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.texture.fbo == 0 {
		// Create FBO.
		fbo := ctx.GenFramebuffers(1)
		ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		// Attach the color texture as COLOR_ATTACHMENT0.
		// Use the texture's actual target (GL_TEXTURE_2D or GL_TEXTURE_2D_MULTISAMPLE).
		ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, c.texture.target, c.texture.id, 0)
		// Verify completeness.
		status := ctx.CheckFramebufferStatus(gl.FRAMEBUFFER)
		if status != gl.FRAMEBUFFER_COMPLETE {
			// FBO incomplete — delete and fall back to default framebuffer.
			ctx.DeleteFramebuffers(fbo)
			ctx.BindFramebuffer(gl.FRAMEBUFFER, 0)
			st.bindFramebuffer(0)
			return
		}
		c.texture.fbo = fbo
		st.bindFramebuffer(fbo)
	} else {
		if st.bindFramebuffer(c.texture.fbo) {
			ctx.BindFramebuffer(gl.FRAMEBUFFER, c.texture.fbo)
		}
	}
}

// AttachDepthStencilCommand attaches a depth/stencil texture to the currently
// bound FBO (the one associated with the color texture). This must be recorded
// after EnsureOffscreenFBOCommand so the FBO is already bound.
type AttachDepthStencilCommand struct {
	colorTexture *Texture // used to verify the FBO exists
	depthTexture *Texture
}

func (c *AttachDepthStencilCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.colorTexture.fbo == 0 {
		return
	}
	attachment := depthStencilAttachmentPoint(c.depthTexture.format)
	ctx.FramebufferTexture2D(gl.FRAMEBUFFER, attachment, c.depthTexture.target, c.depthTexture.id, 0)
}

// depthStencilAttachmentPoint returns the GL attachment point for a depth/stencil
// texture format.
// depthStencilAttachmentPoint returns the GL attachment point for a depth/stencil
// texture format.
func depthStencilAttachmentPoint(format gputypes.TextureFormat) uint32 {
	switch format {
	case gputypes.TextureFormatDepth24PlusStencil8, gputypes.TextureFormatDepth32FloatStencil8:
		return gl.DEPTH_STENCIL_ATTACHMENT
	case gputypes.TextureFormatStencil8:
		return gl.STENCIL_ATTACHMENT
	default:
		return gl.DEPTH_ATTACHMENT
	}
}

// AttachDepthStencilToFBOCommand attaches a depth/stencil texture to the
// currently bound FBO. Unlike AttachDepthStencilCommand, this does not
// reference a color texture — it operates on whatever FBO is currently bound
// (typically the surface swapchain FBO).
//
// The attachment point is chosen by texture format:
//   - depth-only → GL_DEPTH_ATTACHMENT
//   - stencil-only → GL_STENCIL_ATTACHMENT
//   - depth+stencil → GL_DEPTH_STENCIL_ATTACHMENT
type AttachDepthStencilToFBOCommand struct {
	depthTexture *Texture
}

func (c *AttachDepthStencilToFBOCommand) Execute(ctx *gl.Context, st *glExecState) {
	attachment := depthStencilAttachmentPoint(c.depthTexture.format)
	ctx.FramebufferTexture2D(gl.FRAMEBUFFER, attachment, c.depthTexture.target, c.depthTexture.id, 0)
}

// MSAAResolveCommand resolves an MSAA framebuffer to a single-sample framebuffer
// using glBlitFramebuffer. This is recorded at render pass End() when a
// ResolveTarget is specified in the color attachment.
//
// When resolveToSurface is true, the destination is the Surface's swapchain
// offscreen FBO (NOT FBO 0). No Y-flip is applied here — the Y-flip is the
// sole responsibility of Queue.Present, which blits the swapchain FBO to FBO 0
// with srcY0=height, srcY1=0 before SwapBuffers.
type MSAAResolveCommand struct {
	msaaTexture      *Texture // MSAA source texture (SampleCount > 1)
	resolveTexture   *Texture // Single-sample resolve target (nil when resolveToSurface)
	resolveToSurface bool     // True to resolve into the Surface's swapchain FBO
	surface          *Surface // Surface owning the swapchain FBO (resolveToSurface only)
	width, height    int32
}

func (c *MSAAResolveCommand) Execute(ctx *gl.Context, st *glExecState) {
	// Disable scissor test before blit — glBlitFramebuffer respects GL_SCISSOR_TEST
	// on the draw framebuffer. Without this, only the last scissor rect's pixels
	// are copied, leaving the rest of the surface black.
	if st.setScissorTest(false) {
		ctx.Disable(gl.SCISSOR_TEST)
	}

	// Bind MSAA FBO as read source.
	if st.bindRead(c.msaaTexture.fbo) {
		ctx.BindFramebuffer(gl.READ_FRAMEBUFFER, c.msaaTexture.fbo)
	}

	// Bind the draw target.
	if c.resolveToSurface {
		// Resolve into the Surface's swapchain offscreen FBO. The Y-flip is
		// performed at Present time, not here, so the resolve is a straight
		// (non-flipped) MSAA resolve.
		var drawFBO uint32
		if c.surface != nil {
			drawFBO = c.surface.swapchainFBO
		}
		if st.bindDraw(drawFBO) {
			ctx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, drawFBO)
		}
	} else {
		if !c.ensureResolveFBO(ctx) {
			return
		}
		st.bindDraw(c.resolveTexture.fbo)
	}

	// Straight MSAA resolve — no Y-flip. Both source and destination use the
	// same upside-down orientation; Queue.Present un-flips at SwapBuffers time.
	ctx.BlitFramebuffer(
		0, 0, c.width, c.height,
		0, 0, c.width, c.height,
		gl.COLOR_BUFFER_BIT, gl.NEAREST,
	)

	// Restore default framebuffer binding.
	if st.bindRead(0) {
		ctx.BindFramebuffer(gl.READ_FRAMEBUFFER, 0)
	}
	if st.bindDraw(0) {
		ctx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, 0)
	}
}

// ensureResolveFBO lazily creates the resolve FBO and binds it as draw target.
// Returns false if the FBO creation fails.
// ensureResolveFBO lazily creates the resolve FBO and binds it as draw target.
// Returns false if the FBO creation fails.
func (c *MSAAResolveCommand) ensureResolveFBO(ctx *gl.Context) bool {
	if c.resolveTexture.fbo == 0 {
		fbo := ctx.GenFramebuffers(1)
		ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0,
			c.resolveTexture.target, c.resolveTexture.id, 0)
		if ctx.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
			ctx.DeleteFramebuffers(fbo)
			return false
		}
		c.resolveTexture.fbo = fbo
	}
	ctx.BindFramebuffer(gl.DRAW_FRAMEBUFFER, c.resolveTexture.fbo)
	return true
}

// AttachColorCommand attaches a color texture to a specific FBO attachment point.
// Used for MRT (Multiple Render Targets) to bind attachments 1..N.
// Attachment 0 is bound by EnsureOffscreenFBOCommand.
type AttachColorCommand struct {
	attachmentIndex uint32 // 0-based index (attachment point = GL_COLOR_ATTACHMENT0 + index)
	texture         *Texture
}

func (c *AttachColorCommand) Execute(ctx *gl.Context, st *glExecState) {
	ctx.FramebufferTexture2D(gl.FRAMEBUFFER,
		gl.COLOR_ATTACHMENT0+c.attachmentIndex,
		c.texture.target, c.texture.id, 0)
}

// SetDrawColorBuffersCommand configures the list of draw buffers for MRT output.
type SetDrawColorBuffersCommand struct {
	count int // number of color attachments
}

func (c *SetDrawColorBuffersCommand) Execute(ctx *gl.Context, st *glExecState) {
	bufs := make([]uint32, c.count)
	for i := range bufs {
		bufs[i] = gl.COLOR_ATTACHMENT0 + uint32(i)
	}
	ctx.DrawBuffers(bufs)
}

// ClearColorBufferCommand clears a specific color draw buffer using glClearBufferfv.
// This replaces the old global glClearColor+glClear approach for MRT correctness:
// each color attachment can have a different clear value.
type ClearColorBufferCommand struct {
	drawBuffer int32      // 0-based draw buffer index
	color      [4]float32 // RGBA clear value
}

func (c *ClearColorBufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setScissorTest(false) {
		ctx.Disable(gl.SCISSOR_TEST) // Ensure clear covers full framebuffer (not clipped by stale scissor)
	}
	// Temporarily enable all color writes so the clear takes effect even if a
	// previous pipeline masked some channels.
	if st.setColorMask(true, true, true, true) {
		ctx.ColorMask(true, true, true, true)
	}
	ctx.ClearBufferfv(gl.COLOR, c.drawBuffer, &c.color)
}

// ClearColorCommand clears a color attachment using the legacy global clear path.
// Retained for backward compatibility with tests and single-target code paths.
// For MRT, use ClearColorBufferCommand instead (per-buffer via glClearBufferfv).
type ClearColorCommand struct {
	r, g, b, a float32
}

func (c *ClearColorCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setScissorTest(false) {
		ctx.Disable(gl.SCISSOR_TEST) // Ensure clear covers full framebuffer (not clipped by stale scissor)
	}
	ctx.ClearColor(c.r, c.g, c.b, c.a)
	ctx.Clear(gl.COLOR_BUFFER_BIT)
}

// ClearDepthCommand clears the depth buffer.
type ClearDepthCommand struct {
	depth float64
}

func (c *ClearDepthCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setScissorTest(false) {
		ctx.Disable(gl.SCISSOR_TEST)
	}
	ctx.DepthMask(true)
	st.markDepthMask(true)
	ctx.ClearDepth(c.depth)
	ctx.Clear(gl.DEPTH_BUFFER_BIT)
}

// ClearStencilCommand clears the stencil buffer.
type ClearStencilCommand struct {
	stencil int32
}

func (c *ClearStencilCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setScissorTest(false) {
		ctx.Disable(gl.SCISSOR_TEST)
	}
	// Ensure stencil write mask allows the clear to take effect.
	ctx.StencilMaskSeparate(gl.FRONT_AND_BACK, 0xFF)
	st.markStencilWMask(true, 0xFF)
	st.markStencilWMask(false, 0xFF)
	ctx.Clear(gl.STENCIL_BUFFER_BIT)
}

// UseProgramCommand activates a shader program.
type UseProgramCommand struct {
	programID uint32
}

func (c *UseProgramCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.useProgram(c.programID) {
		ctx.UseProgram(c.programID)
	}
}

// SetPipelineStateCommand sets pipeline state (culling, depth, stencil, blending, color mask).
type SetPipelineStateCommand struct {
	topology     gputypes.PrimitiveTopology
	cullMode     gputypes.CullMode
	frontFace    gputypes.FrontFace
	depthStencil *hal.DepthStencilState
	colorTargets []ColorTargetDesc // per-target blend/write-mask for MRT
	stencilRef   uint32
}

// cullFaceGL maps a CullMode to its GL face enum (only read when culling on).
// cullFaceGL maps a CullMode to its GL face enum (only read when culling on).
func cullFaceGL(m gputypes.CullMode) uint32 {
	if m == gputypes.CullModeFront {
		return gl.FRONT
	}
	return gl.BACK
}

func (c *SetPipelineStateCommand) Execute(ctx *gl.Context, st *glExecState) {
	// Culling
	enabled := c.cullMode != gputypes.CullModeNone
	toggle, face := st.setCull(enabled, cullFaceGL(c.cullMode))
	if toggle {
		if enabled {
			ctx.Enable(gl.CULL_FACE)
		} else {
			ctx.Disable(gl.CULL_FACE)
		}
	}
	if enabled && face {
		ctx.CullFace(cullFaceGL(c.cullMode))
	}

	// Front face — swapped CW↔CCW to compensate for the Y-flip from
	// ADJUST_COORDINATE_SPACE. The negation of gl_Position.y reverses triangle
	// winding order, so we swap the front face to keep the same visibility.
	wantFront := uint32(gl.CW)
	if c.frontFace == gputypes.FrontFaceCW {
		wantFront = gl.CCW
	}
	if st.setFrontFace(wantFront) {
		ctx.FrontFace(wantFront)
	}

	// Depth and stencil
	c.applyDepthStencilState(ctx, st)

	// Color targets (blend + write mask).
	//   - If targets differ, use per-draw-buffer indexed calls (GLES 3.2 / GL 4.0).
	//     Fallback: apply target[0] globally when indexed functions unavailable.
	c.applyColorTargets(ctx, st)
}

// applyColorTargets sets blend and write-mask state per render target.
// applyColorTargets sets blend and write-mask state per render target.
func (c *SetPipelineStateCommand) applyColorTargets(ctx *gl.Context, st *glExecState) {
	if len(c.colorTargets) == 0 {
		// No color targets — disable blending, allow all color writes.
		if t, _, _ := st.setBlend(false, 0, 0, 0, 0, 0, 0); t {
			ctx.Disable(gl.BLEND)
		}
		if st.setColorMask(true, true, true, true) {
			ctx.ColorMask(true, true, true, true)
		}
		return
	}

	// Single target or all targets identical: use global (non-indexed) calls.
	ct := c.colorTargets[0]
	r := ct.WriteMask&gputypes.ColorWriteMaskRed != 0
	g := ct.WriteMask&gputypes.ColorWriteMaskGreen != 0
	b := ct.WriteMask&gputypes.ColorWriteMaskBlue != 0
	a := ct.WriteMask&gputypes.ColorWriteMaskAlpha != 0
	if st.setColorMask(r, g, b, a) {
		ctx.ColorMask(r, g, b, a)
	}
	if ct.Blend != nil {
		srcRGB := blendFactorToGL(ct.Blend.Color.SrcFactor)
		dstRGB := blendFactorToGL(ct.Blend.Color.DstFactor)
		srcA := blendFactorToGL(ct.Blend.Alpha.SrcFactor)
		dstA := blendFactorToGL(ct.Blend.Alpha.DstFactor)
		eqRGB := blendOperationToGL(ct.Blend.Color.Operation)
		eqA := blendOperationToGL(ct.Blend.Alpha.Operation)
		t, f, e := st.setBlend(true, srcRGB, dstRGB, srcA, dstA, eqRGB, eqA)
		if t {
			ctx.Enable(gl.BLEND)
		}
		if f {
			ctx.BlendFuncSeparate(srcRGB, dstRGB, srcA, dstA)
		}
		if e {
			ctx.BlendEquationSeparate(eqRGB, eqA)
		}
	} else {
		if t, _, _ := st.setBlend(false, 0, 0, 0, 0, 0, 0); t {
			ctx.Disable(gl.BLEND)
		}
	}
}

// applyDepthStencilState configures GL depth test and stencil test from pipeline state.
// applyDepthStencilState configures GL depth test and stencil test from pipeline state.
func (c *SetPipelineStateCommand) applyDepthStencilState(ctx *gl.Context, st *glExecState) {
	if c.depthStencil == nil {
		if t, _, _ := st.setDepth(false, false, 0); t {
			ctx.Disable(gl.DEPTH_TEST)
		}
		if st.setStencilTest(false) {
			ctx.Disable(gl.STENCIL_TEST)
		}
		return
	}

	// Depth test
	depthOn := c.depthStencil.DepthWriteEnabled || c.depthStencil.DepthCompare != gputypes.CompareFunctionAlways
	depthFn := compareFunctionToGL(c.depthStencil.DepthCompare)
	if t, m, f := st.setDepth(depthOn, c.depthStencil.DepthWriteEnabled, depthFn); depthOn {
		if t {
			ctx.Enable(gl.DEPTH_TEST)
		}
		if m {
			ctx.DepthMask(c.depthStencil.DepthWriteEnabled)
		}
		if f {
			ctx.DepthFunc(depthFn)
		}
	} else if t {
		ctx.Disable(gl.DEPTH_TEST)
	}

	// Stencil test
	hasStencilOps := c.depthStencil.StencilFront.PassOp != hal.StencilOperationKeep ||
		c.depthStencil.StencilFront.FailOp != hal.StencilOperationKeep ||
		c.depthStencil.StencilBack.PassOp != hal.StencilOperationKeep ||
		c.depthStencil.StencilBack.FailOp != hal.StencilOperationKeep ||
		c.depthStencil.StencilFront.Compare != gputypes.CompareFunctionAlways ||
		c.depthStencil.StencilBack.Compare != gputypes.CompareFunctionAlways

	if !hasStencilOps && c.depthStencil.StencilWriteMask == 0 {
		if st.setStencilTest(false) {
			ctx.Disable(gl.STENCIL_TEST)
		}
		return
	}

	if st.setStencilTest(true) {
		ctx.Enable(gl.STENCIL_TEST)
	}
	ref := int32(c.stencilRef)

	if st.setStencilFunc(true, compareFunctionToGL(c.depthStencil.StencilFront.Compare),
		ref, c.depthStencil.StencilReadMask) {
		ctx.StencilFuncSeparate(gl.FRONT,
			compareFunctionToGL(c.depthStencil.StencilFront.Compare),
			ref, c.depthStencil.StencilReadMask)
	}
	if st.setStencilFunc(false, compareFunctionToGL(c.depthStencil.StencilBack.Compare),
		ref, c.depthStencil.StencilReadMask) {
		ctx.StencilFuncSeparate(gl.BACK,
			compareFunctionToGL(c.depthStencil.StencilBack.Compare),
			ref, c.depthStencil.StencilReadMask)
	}

	if st.setStencilOp(true,
		stencilOpToGL(c.depthStencil.StencilFront.FailOp),
		stencilOpToGL(c.depthStencil.StencilFront.DepthFailOp),
		stencilOpToGL(c.depthStencil.StencilFront.PassOp)) {
		ctx.StencilOpSeparate(gl.FRONT,
			stencilOpToGL(c.depthStencil.StencilFront.FailOp),
			stencilOpToGL(c.depthStencil.StencilFront.DepthFailOp),
			stencilOpToGL(c.depthStencil.StencilFront.PassOp))
	}
	if st.setStencilOp(false,
		stencilOpToGL(c.depthStencil.StencilBack.FailOp),
		stencilOpToGL(c.depthStencil.StencilBack.DepthFailOp),
		stencilOpToGL(c.depthStencil.StencilBack.PassOp)) {
		ctx.StencilOpSeparate(gl.BACK,
			stencilOpToGL(c.depthStencil.StencilBack.FailOp),
			stencilOpToGL(c.depthStencil.StencilBack.DepthFailOp),
			stencilOpToGL(c.depthStencil.StencilBack.PassOp))
	}

	if st.setStencilMask(true, c.depthStencil.StencilWriteMask) {
		ctx.StencilMaskSeparate(gl.FRONT, c.depthStencil.StencilWriteMask)
	}
	if st.setStencilMask(false, c.depthStencil.StencilWriteMask) {
		ctx.StencilMaskSeparate(gl.BACK, c.depthStencil.StencilWriteMask)
	}
	st.markStencilApplied()
}

// SetBindGroupCommand binds resources.
type SetBindGroupCommand struct {
	index           uint32
	group           *BindGroup
	dynamicOffsets  []uint32
	maxTextureUnits int32 // Hardware limit for validation
	// groupInfos stores per-group binding-to-slot tables from PipelineLayout.
	// Used to look up the GL slot for each binding instead of the old group*16+binding formula.
	groupInfos []BindGroupLayoutInfo
	// samplerBindMap maps texture unit → sampler unit for combined sampler2D.
	// When non-nil, sampler is bound to the texture's unit instead of its own WGSL binding.
	samplerBindMap *[maxTextureSlots]int8
}

func (c *SetBindGroupCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.group == nil {
		return
	}

	dynamicIdx := 0
	for _, entry := range c.group.entries {
		// Look up the GL slot index from the pre-computed per-type sequential
		// binding table (computed in CreatePipelineLayout). This replaces the old
		glBinding := c.lookupSlot(entry.Binding)
		if glBinding == 0xFF {
			continue // binding not mapped in pipeline layout
		}

		// unpack hal entries to concrete gles types (gpu unpack pattern).
		switch {
		case entry.Buffer != nil:
			buf, ok := entry.Buffer.(*Buffer)
			if !ok || buf == nil || buf.id == 0 {
				continue
			}
			bufID := buf.id
			offset := int(entry.Offset)
			size := int(entry.Size)

			// Determine GL buffer target and apply dynamic offset from layout entry.
			// Storage buffers use GL_SHADER_STORAGE_BUFFER, uniform buffers use GL_UNIFORM_BUFFER.
			target, dynOff := c.resolveBufferTarget(entry.Binding, &dynamicIdx)
			offset += dynOff

			if size > 0 {
				if st.bindIndexed(target, glBinding, bufID, offset, size) {
					ctx.BindBufferRange(target, glBinding, bufID, offset, size)
				}
			} else if st.bindIndexed(target, glBinding, bufID, 0, 0) {
				ctx.BindBufferBase(target, glBinding, bufID)
			}

		case entry.TextureView != nil:
			tv, ok := entry.TextureView.(*TextureView)
			if !ok || tv == nil || tv.texture == nil || tv.texture.id == 0 {
				continue
			}
			texID := tv.texture.id
			// Validate texture unit index against hardware limit.
			// Without this check, textures silently fail to bind when
			// glBinding >= GL_MAX_TEXTURE_IMAGE_UNITS (typically 8 on Intel).
			if c.maxTextureUnits > 0 && int32(glBinding) >= c.maxTextureUnits {
				if l := hal.Logger(); l != nil {
					l.Warn("GLES texture unit overflow: binding exceeds hardware limit",
						slog.Uint64("glBinding", uint64(glBinding)),
						slog.Int64("maxTextureUnits", int64(c.maxTextureUnits)),
						slog.Uint64("group", uint64(c.index)),
						slog.Uint64("binding", uint64(entry.Binding)),
					)
				}
				continue
			}
			if st.activeTexture(gl.TEXTURE0 + glBinding) {
				ctx.ActiveTexture(gl.TEXTURE0 + glBinding)
			}
			if st.bindTexture(glBinding, texID) {
				ctx.BindTexture(gl.TEXTURE_2D, texID)
			}

		case entry.Sampler != nil:
			s, ok := entry.Sampler.(*Sampler)
			if !ok || s == nil || s.id == 0 {
				continue
			}
			samplerID := s.id
			// Determine the correct texture unit for this sampler.
			// SamplerBindMap provides this mapping.
			bindUnit := c.resolveSamplerUnit(glBinding)
			if hal.Logger().Enabled(context.Background(), slog.LevelDebug) {
				hal.Logger().Debug("gles: binding sampler",
					"samplerBinding", glBinding,
					"textureUnit", bindUnit,
					"samplerID", samplerID,
				)
			}
			if st.bindSampler(bindUnit, samplerID) {
				ctx.BindSampler(bindUnit, samplerID)
			}
		}
	}
}

// lookupSlot returns the GL slot index for a binding number using the
// pre-computed GroupInfos from PipelineLayout. Returns 0xFF if not mapped.
// lookupSlot returns the GL slot index for a binding number using the
// pre-computed GroupInfos from PipelineLayout. Returns 0xFF if not mapped.
func (c *SetBindGroupCommand) lookupSlot(binding uint32) uint32 {
	if int(c.index) < len(c.groupInfos) {
		info := c.groupInfos[c.index]
		if int(binding) < len(info.BindingToSlot) {
			return uint32(info.BindingToSlot[binding])
		}
	}
	return 0xFF
}

// resolveSamplerUnit finds the texture unit for a sampler via SamplerBindMap.
// resolveSamplerUnit finds the texture unit for a sampler via SamplerBindMap.
func (c *SetBindGroupCommand) resolveSamplerUnit(glBinding uint32) uint32 {
	if c.samplerBindMap == nil {
		return glBinding
	}
	for texUnit := range c.samplerBindMap {
		if c.samplerBindMap[texUnit] == int8(glBinding) {
			return uint32(texUnit)
		}
	}
	return glBinding
}

// resolveBufferTarget determines the GL buffer target (UNIFORM_BUFFER or SHADER_STORAGE_BUFFER)
// and dynamic offset for a binding number by looking up the bind group layout entry.
// Returns the GL target and the dynamic offset to apply (0 if none).
// resolveBufferTarget determines the GL buffer target (UNIFORM_BUFFER or SHADER_STORAGE_BUFFER)
// and dynamic offset for a binding number by looking up the bind group layout entry.
// Returns the GL target and the dynamic offset to apply (0 if none).
func (c *SetBindGroupCommand) resolveBufferTarget(binding uint32, dynamicIdx *int) (uint32, int) {
	target := uint32(gl.UNIFORM_BUFFER)
	dynOffset := 0
	if c.group.layout == nil {
		return target, dynOffset
	}
	for _, le := range c.group.layout.entries {
		if le.Binding != binding || le.Buffer == nil {
			continue
		}
		if le.Buffer.Type == gputypes.BufferBindingTypeStorage ||
			le.Buffer.Type == gputypes.BufferBindingTypeReadOnlyStorage {
			target = gl.SHADER_STORAGE_BUFFER
		}
		if le.Buffer.HasDynamicOffset && c.dynamicOffsets != nil && *dynamicIdx < len(c.dynamicOffsets) {
			dynOffset = int(c.dynamicOffsets[*dynamicIdx])
			*dynamicIdx++
		}
		break
	}
	return target, dynOffset
}

// SetVertexBufferCommand binds a vertex buffer and configures vertex attributes.
// In OpenGL, vertex attributes must be configured explicitly via
// glVertexAttribPointer + glEnableVertexAttribArray. The layout describes
// how vertex data is interpreted (attribute locations, formats, strides).
type SetVertexBufferCommand struct {
	slot   uint32
	buffer *Buffer
	offset uint64
	layout *gputypes.VertexBufferLayout // from the render pipeline descriptor
}

func (c *SetVertexBufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.bindArray(c.buffer.id) {
		ctx.BindBuffer(gl.ARRAY_BUFFER, c.buffer.id)
	}

	// Configure vertex attributes from the pipeline's vertex layout.
	if c.layout == nil {
		return
	}
	stride := int32(c.layout.ArrayStride)
	// Determine divisor from step mode: 0=per-vertex, 1=per-instance.
	var divisor uint32
	if c.layout.StepMode == gputypes.VertexStepModeInstance {
		divisor = 1
	}
	for _, attr := range c.layout.Attributes {
		loc := attr.ShaderLocation
		size, typ, normalized := vertexFormatToGL(attr.Format)
		attrOffset := uintptr(c.offset) + uintptr(attr.Offset)
		en, ptr, div := st.setAttrib(loc, attribState{
			enabled: true, size: size, typ: typ, normalized: normalized,
			stride: stride, offset: attrOffset, buf: c.buffer.id, divisor: divisor,
		})
		// The buffer above is bound (or already was); the pointer captures it.
		if en {
			ctx.EnableVertexAttribArray(loc)
		}
		if ptr {
			ctx.VertexAttribPointer(loc, size, typ, normalized, stride, attrOffset)
		}
		// Set the instance divisor. Per-vertex attributes get divisor=0 (reset),
		// per-instance attributes get divisor=1. Without this, instanced rendering
		// reads the same instance data for all instances.
		if div {
			ctx.VertexAttribDivisor(loc, divisor)
		}
	}
}

// SetIndexBufferCommand binds an index buffer.
type SetIndexBufferCommand struct {
	buffer *Buffer
	format gputypes.IndexFormat
	offset uint64
}

func (c *SetIndexBufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.bindElement(c.buffer.id) {
		ctx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, c.buffer.id)
	}
}

// SetViewportCommand sets the viewport.
type SetViewportCommand struct {
	x, y, width, height float32
	minDepth, maxDepth  float32
}

func (c *SetViewportCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setViewport(int32(c.x), int32(c.y), int32(c.width), int32(c.height), float64(c.minDepth), float64(c.maxDepth)) {
		ctx.Viewport(int32(c.x), int32(c.y), int32(c.width), int32(c.height))
		ctx.DepthRange(float64(c.minDepth), float64(c.maxDepth))
	}
}

// SetScissorCommand sets the scissor rectangle.
// With ADJUST_COORDINATE_SPACE enabled, the scene is rendered upside-down in GL.
// The scissor coordinates are passed through directly (no Y-flip needed) because
// the scissor operates in the same flipped coordinate space as the rendered content.
type SetScissorCommand struct {
	x, y, width, height uint32
}

func (c *SetScissorCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setScissorTest(true) {
		ctx.Enable(gl.SCISSOR_TEST)
	}
	// No Y-flip: ADJUST_COORDINATE_SPACE flips the scene in the vertex shader,
	// so GL pixel Y=0 corresponds to the top of the scene.
	// The scissor rect in WebGPU coords maps directly to GL coords.
	if st.setScissor(int32(c.x), int32(c.y), int32(c.width), int32(c.height)) {
		ctx.Scissor(int32(c.x), int32(c.y), int32(c.width), int32(c.height))
	}
}

// SetBlendConstantCommand sets blend constant.
type SetBlendConstantCommand struct {
	r, g, b, a float32
}

func (c *SetBlendConstantCommand) Execute(ctx *gl.Context, st *glExecState) {
	if st.setBlendColor(c.r, c.g, c.b, c.a) {
		ctx.BlendColor(c.r, c.g, c.b, c.a)
	}
}

// SetStencilRefCommand updates the stencil reference value.
// This re-applies glStencilFuncSeparate with the new reference while
// keeping the compare function and read mask from the current pipeline.
type SetStencilRefCommand struct {
	ref          uint32
	depthStencil *hal.DepthStencilState
}

func (c *SetStencilRefCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.depthStencil == nil {
		return
	}
	ref := int32(c.ref)
	if st.setStencilFunc(true, compareFunctionToGL(c.depthStencil.StencilFront.Compare),
		ref, c.depthStencil.StencilReadMask) {
		ctx.StencilFuncSeparate(gl.FRONT,
			compareFunctionToGL(c.depthStencil.StencilFront.Compare),
			ref, c.depthStencil.StencilReadMask)
	}
	if st.setStencilFunc(false, compareFunctionToGL(c.depthStencil.StencilBack.Compare),
		ref, c.depthStencil.StencilReadMask) {
		ctx.StencilFuncSeparate(gl.BACK,
			compareFunctionToGL(c.depthStencil.StencilBack.Compare),
			ref, c.depthStencil.StencilReadMask)
	}
}

// DrawCommand executes a non-indexed draw.
type DrawCommand struct {
	vertexCount, instanceCount uint32
	firstVertex, firstInstance uint32
	topology                   gputypes.PrimitiveTopology
}

func (c *DrawCommand) Execute(ctx *gl.Context, st *glExecState) {
	mode := primitiveTopologyToGL(c.topology)
	if c.instanceCount <= 1 {
		ctx.DrawArrays(mode, int32(c.firstVertex), int32(c.vertexCount))
	} else {
		ctx.DrawArraysInstanced(mode, int32(c.firstVertex), int32(c.vertexCount), int32(c.instanceCount))
	}
}

// DrawIndexedCommand executes an indexed draw.
type DrawIndexedCommand struct {
	indexCount, instanceCount uint32
	firstIndex                uint32
	baseVertex                int32
	firstInstance             uint32
	indexFormat               gputypes.IndexFormat
	topology                  gputypes.PrimitiveTopology
}

func (c *DrawIndexedCommand) Execute(ctx *gl.Context, st *glExecState) {
	indexType := uint32(gl.UNSIGNED_SHORT)
	indexSize := uintptr(2)
	if c.indexFormat == gputypes.IndexFormatUint32 {
		indexType = gl.UNSIGNED_INT
		indexSize = 4
	}

	offset := uintptr(c.firstIndex) * indexSize
	mode := primitiveTopologyToGL(c.topology)

	if c.instanceCount <= 1 {
		ctx.DrawElements(mode, int32(c.indexCount), indexType, offset)
	} else {
		ctx.DrawElementsInstanced(mode, int32(c.indexCount), indexType, offset, int32(c.instanceCount))
	}
}

// CopyBufferCommand copies between buffers.
type CopyBufferCommand struct {
	srcID, dstID         uint32
	srcOffset, dstOffset uint64
	size                 uint64
}

func (c *CopyBufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	ctx.BindBuffer(gl.COPY_READ_BUFFER, c.srcID)
	ctx.BindBuffer(gl.COPY_WRITE_BUFFER, c.dstID)
	// glCopyBufferSubData would go here
	ctx.BindBuffer(gl.COPY_READ_BUFFER, 0)
	ctx.BindBuffer(gl.COPY_WRITE_BUFFER, 0)
}

// DispatchCommand dispatches compute work.
type DispatchCommand struct {
	x, y, z uint32
}

// Execute dispatches compute work and inserts a memory barrier.
// Execute dispatches compute work and inserts a memory barrier.
func (c *DispatchCommand) Execute(ctx *gl.Context, st *glExecState) {
	ctx.DispatchCompute(c.x, c.y, c.z)
	// VERTEX_ATTRIB_ARRAY_BARRIER_BIT is required when compute writes an SSBO that
	// is later read as a vertex buffer (e.g. particles ping-pong). Without it,
	// vertex fetch reads stale pre-compute data.
	ctx.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT | gl.VERTEX_ATTRIB_ARRAY_BARRIER_BIT | gl.BUFFER_UPDATE_BARRIER_BIT)
}

// DispatchIndirectCommand dispatches compute work with GPU-generated parameters.
type DispatchIndirectCommand struct {
	buffer *Buffer
	offset uint64
}

// Execute dispatches compute work from indirect buffer and inserts a memory barrier.
// Execute dispatches compute work from indirect buffer and inserts a memory barrier.
func (c *DispatchIndirectCommand) Execute(ctx *gl.Context, st *glExecState) {
	// Bind the buffer containing dispatch parameters
	ctx.BindBuffer(gl.DISPATCH_INDIRECT_BUFFER, c.buffer.id)
	// Dispatch with parameters from the buffer at the given offset
	ctx.DispatchComputeIndirect(uintptr(c.offset))
	// Unbind the indirect buffer
	ctx.BindBuffer(gl.DISPATCH_INDIRECT_BUFFER, 0)
	ctx.MemoryBarrier(gl.SHADER_STORAGE_BARRIER_BIT | gl.VERTEX_ATTRIB_ARRAY_BARRIER_BIT | gl.BUFFER_UPDATE_BARRIER_BIT)
}

// CopyTextureToBufferCommand reads pixels from a texture's FBO into a buffer's
// CPU-side data slice. This is the standard GLES readback path since GLES lacks
// glGetTexImage. The approach: bind texture FBO -> glReadPixels -> copy to buffer.
type CopyTextureToBufferCommand struct {
	glCtx       *gl.Context
	srcTexture  *Texture
	dstBuffer   *Buffer
	srcOrigin   [3]uint32 // x, y, z
	copySize    [3]uint32 // width, height, depthOrArrayLayers
	dstOffset   uint64
	bytesPerRow uint32
}

// Execute reads pixels from the source texture's FBO into the destination buffer.
// Execute reads pixels from the source texture's FBO into the destination buffer.
func (c *CopyTextureToBufferCommand) Execute(ctx *gl.Context, st *glExecState) {
	width := int32(c.copySize[0])
	height := int32(c.copySize[1])
	if width == 0 || height == 0 {
		return
	}

	// RGBA8 readback: 4 bytes per pixel.
	bpp := uint32(4)
	rowBytes := uint32(width) * bpp
	// Row stride follows the hal BufferTextureCopy contract (WebGPU
	// copyTextureToBuffer parity): buffer rows sit BytesPerRow apart and
	// staging rows are 256B-padded. MapBuffer's fast path needs
	// len(data)==buf.size, so round strided regions up to the buffer size
	// (exact-size buffers are unaffected).
	stride := c.bytesPerRow
	if stride < rowBytes {
		stride = rowBytes
	}
	totalBytes := c.dstOffset + uint64(stride)*uint64(height-1) + uint64(rowBytes)
	if totalBytes < c.dstBuffer.size {
		totalBytes = c.dstBuffer.size
	}

	// Ensure destination buffer has enough CPU-side storage.
	requiredSize := totalBytes
	if uint64(len(c.dstBuffer.data)) < requiredSize {
		newData := make([]byte, requiredSize)
		copy(newData, c.dstBuffer.data)
		c.dstBuffer.data = newData
	}

	// Save the current FBO binding so we can restore it after the read.
	var prevFBO int32
	ctx.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)

	// Ensure the source texture has an FBO. Create one lazily if needed.
	if c.srcTexture.fbo == 0 {
		fbo := ctx.GenFramebuffers(1)
		ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
		ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, c.srcTexture.target, c.srcTexture.id, 0)
		status := ctx.CheckFramebufferStatus(gl.FRAMEBUFFER)
		if status != gl.FRAMEBUFFER_COMPLETE {
			ctx.DeleteFramebuffers(fbo)
			ctx.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
			return
		}
		c.srcTexture.fbo = fbo
	} else {
		ctx.BindFramebuffer(gl.FRAMEBUFFER, c.srcTexture.fbo)
	}

	// Set tight pixel packing (no row alignment padding).
	ctx.PixelStorei(gl.PACK_ALIGNMENT, 1)

	// Read pixels from the bound FBO into a temporary CPU buffer.
	// PACK_ALIGNMENT is 1 above, so GL packs rows tightly.
	tmpBuf := make([]byte, uint64(rowBytes)*uint64(height))
	ctx.ReadPixels(
		int32(c.srcOrigin[0]), int32(c.srcOrigin[1]),
		width, height,
		gl.BGRA, gl.UNSIGNED_BYTE,
		unsafe.Pointer(&tmpBuf[0]),
	)

	// Copy the pixel data into the destination buffer's CPU-side storage.
	// No row flip: session shaders use WriterFlagAdjustCoordinateSpace
	// (negated gl_Position.y), so the scene already lands with FBO bottom
	// row == logical top row; glReadPixels row 0 is the caller's row 0.
	for row := int32(0); row < height; row++ {
		srcStart := uint64(row) * uint64(rowBytes)
		dstStart := c.dstOffset + uint64(row)*uint64(stride)
		copy(c.dstBuffer.data[dstStart:dstStart+uint64(rowBytes)], tmpBuf[srcStart:srcStart+uint64(rowBytes)])
	}

	// Restore the previous FBO binding.
	ctx.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
}

// CopyBufferToTextureCommand copies buffer data to a texture using a pixel unpack
// buffer. Binds the source GL buffer as GL_PIXEL_UNPACK_BUFFER, then calls
// glTexSubImage2D with offset=0 so GL reads from the bound PBO.
type CopyBufferToTextureCommand struct {
	srcBuffer *Buffer
	dstTex    *Texture
	origin    [3]uint32 // x, y, z
	copySize  [3]uint32 // width, height, depthOrArrayLayers
	bufOffset uint64
}

func (c *CopyBufferToTextureCommand) Execute(ctx *gl.Context, st *glExecState) {
	width := int32(c.copySize[0])
	height := int32(c.copySize[1])
	if width == 0 || height == 0 {
		return
	}

	_, format, dataType := textureFormatToGL(c.dstTex.format)

	// Bind source buffer as pixel unpack buffer (PBO).
	ctx.BindBuffer(gl.PIXEL_UNPACK_BUFFER, c.srcBuffer.id)

	// Bind destination texture.
	ctx.BindTexture(c.dstTex.target, c.dstTex.id)

	// Set pixel alignment to 1 for formats whose rows may not be 4-byte aligned.
	ctx.PixelStorei(gl.UNPACK_ALIGNMENT, 1)
	ctx.PixelStorei(gl.UNPACK_ROW_LENGTH, 0) // use default (tightly packed)

	// Upload from PBO. When a PBO is bound, the pixels pointer is interpreted
	// as a byte offset into the bound buffer.
	offset := u2p(uintptr(c.bufOffset))
	ctx.TexSubImage2D(c.dstTex.target, 0,
		int32(c.origin[0]), int32(c.origin[1]),
		width, height,
		format, dataType, offset)

	// Restore defaults.
	ctx.PixelStorei(gl.UNPACK_ALIGNMENT, 4)
	ctx.BindBuffer(gl.PIXEL_UNPACK_BUFFER, 0)
	ctx.BindTexture(c.dstTex.target, 0)
}

// CopyTextureToTextureCommand copies pixels between textures using an FBO.
// Attaches the source texture to GL_READ_FRAMEBUFFER, then copies into the
// destination via glCopyTexSubImage2D.
type CopyTextureToTextureCommand struct {
	srcTex    *Texture
	dstTex    *Texture
	srcOrigin [3]uint32 // x, y, z
	dstOrigin [3]uint32 // x, y, z
	copySize  [3]uint32 // width, height, depthOrArrayLayers
	srcMip    uint32
	dstMip    uint32
}

func (c *CopyTextureToTextureCommand) Execute(ctx *gl.Context, st *glExecState) {
	width := int32(c.copySize[0])
	height := int32(c.copySize[1])
	if width == 0 || height == 0 {
		return
	}

	// Save current FBO binding.
	var prevFBO int32
	ctx.GetIntegerv(gl.FRAMEBUFFER_BINDING, &prevFBO)

	// Create a temporary FBO for reading from the source texture.
	readFBO := ctx.GenFramebuffers(1)
	ctx.BindFramebuffer(gl.READ_FRAMEBUFFER, readFBO)
	ctx.FramebufferTexture2D(gl.READ_FRAMEBUFFER, gl.COLOR_ATTACHMENT0,
		c.srcTex.target, c.srcTex.id, int32(c.srcMip))

	// Bind destination texture and copy pixels from the read framebuffer.
	ctx.BindTexture(c.dstTex.target, c.dstTex.id)
	ctx.CopyTexSubImage2D(c.dstTex.target, int32(c.dstMip),
		int32(c.dstOrigin[0]), int32(c.dstOrigin[1]),
		int32(c.srcOrigin[0]), int32(c.srcOrigin[1]),
		width, height)
	ctx.BindTexture(c.dstTex.target, 0)

	// Clean up temporary FBO.
	ctx.DeleteFramebuffers(readFBO)
	ctx.BindFramebuffer(gl.FRAMEBUFFER, uint32(prevFBO))
}

// ResolveQuerySetCommand reads query results via glGetQueryObjectui64v and
// writes them into the destination buffer via glBufferSubData.
// Uses the CPU fallback path (no QUERY_BUFFER) to stay compatible with
// GLES 3.0+ / GL 3.3+. Each result is a uint64 (8 bytes).
type ResolveQuerySetCommand struct {
	querySet   *QuerySet
	firstQuery uint32
	queryCount uint32
	dstBuffer  *Buffer
	dstOffset  uint64
}

func (c *ResolveQuerySetCommand) Execute(ctx *gl.Context, st *glExecState) {
	if c.querySet == nil || len(c.querySet.queries) == 0 {
		return
	}

	// Read query results into a temporary slice.
	results := make([]uint64, c.queryCount)
	for i := uint32(0); i < c.queryCount; i++ {
		idx := c.firstQuery + i
		if int(idx) >= len(c.querySet.queries) {
			break
		}
		ctx.GetQueryObjectui64v(c.querySet.queries[idx], gl.QUERY_RESULT, &results[i])
	}

	// Write results into the destination GL buffer via glBufferSubData.
	if c.dstBuffer.id != 0 {
		queryData := unsafe.Slice((*byte)(unsafe.Pointer(&results[0])), len(results)*8)
		ctx.BindBuffer(c.dstBuffer.target, c.dstBuffer.id)
		ctx.BufferSubData(c.dstBuffer.target, int(c.dstOffset), len(queryData),
			unsafe.Pointer(&queryData[0]))
		ctx.BindBuffer(c.dstBuffer.target, 0)
	}
}

// TimestampQueryCommand records a timestamp via glQueryCounter.
// Used internally by render/compute pass timestamp writes.
type TimestampQueryCommand struct {
	query uint32 // GL query object ID
}

func (c *TimestampQueryCommand) Execute(ctx *gl.Context, st *glExecState) {
	ctx.QueryCounter(c.query, gl.TIMESTAMP)
}

// vertexFormatToGL converts a WebGPU vertex format to GL component count and type.
// vertexFormatToGL converts a WebGPU vertex format to GL component count and type.
func vertexFormatToGL(format gputypes.VertexFormat) (size int32, typ uint32, normalized bool) {
	switch format {
	case gputypes.VertexFormatFloat32:
		return 1, gl.FLOAT, false
	case gputypes.VertexFormatFloat32x2:
		return 2, gl.FLOAT, false
	case gputypes.VertexFormatFloat32x3:
		return 3, gl.FLOAT, false
	case gputypes.VertexFormatFloat32x4:
		return 4, gl.FLOAT, false
	case gputypes.VertexFormatUint8x2:
		return 2, gl.UNSIGNED_BYTE, false
	case gputypes.VertexFormatUint8x4:
		return 4, gl.UNSIGNED_BYTE, false
	case gputypes.VertexFormatUnorm8x2:
		return 2, gl.UNSIGNED_BYTE, true
	case gputypes.VertexFormatUnorm8x4:
		return 4, gl.UNSIGNED_BYTE, true
	case gputypes.VertexFormatSint8x2:
		return 2, gl.BYTE, false
	case gputypes.VertexFormatSint8x4:
		return 4, gl.BYTE, false
	case gputypes.VertexFormatSnorm8x2:
		return 2, gl.BYTE, true
	case gputypes.VertexFormatSnorm8x4:
		return 4, gl.BYTE, true
	case gputypes.VertexFormatUint16x2:
		return 2, gl.UNSIGNED_SHORT, false
	case gputypes.VertexFormatUint16x4:
		return 4, gl.UNSIGNED_SHORT, false
	case gputypes.VertexFormatUnorm16x2:
		return 2, gl.UNSIGNED_SHORT, true
	case gputypes.VertexFormatUnorm16x4:
		return 4, gl.UNSIGNED_SHORT, true
	case gputypes.VertexFormatSint16x2:
		return 2, gl.SHORT, false
	case gputypes.VertexFormatSint16x4:
		return 4, gl.SHORT, false
	case gputypes.VertexFormatSnorm16x2:
		return 2, gl.SHORT, true
	case gputypes.VertexFormatSnorm16x4:
		return 4, gl.SHORT, true
	case gputypes.VertexFormatUint32:
		return 1, gl.UNSIGNED_INT, false
	case gputypes.VertexFormatUint32x2:
		return 2, gl.UNSIGNED_INT, false
	case gputypes.VertexFormatUint32x3:
		return 3, gl.UNSIGNED_INT, false
	case gputypes.VertexFormatUint32x4:
		return 4, gl.UNSIGNED_INT, false
	case gputypes.VertexFormatSint32:
		return 1, gl.INT, false
	case gputypes.VertexFormatSint32x2:
		return 2, gl.INT, false
	case gputypes.VertexFormatSint32x3:
		return 3, gl.INT, false
	case gputypes.VertexFormatSint32x4:
		return 4, gl.INT, false
	default:
		return 4, gl.FLOAT, false
	}
}

// stencilOpToGL converts a HAL stencil operation to the corresponding GL constant.
// stencilOpToGL converts a HAL stencil operation to the corresponding GL constant.
func stencilOpToGL(op gputypes.StencilOperation) uint32 {
	switch op {
	case hal.StencilOperationKeep:
		return gl.KEEP
	case hal.StencilOperationZero:
		return gl.ZERO
	case hal.StencilOperationReplace:
		return gl.REPLACE
	case hal.StencilOperationInvert:
		return gl.INVERT
	case hal.StencilOperationIncrementClamp:
		return gl.INCR
	case hal.StencilOperationDecrementClamp:
		return gl.DECR
	case hal.StencilOperationIncrementWrap:
		return gl.INCR_WRAP
	case hal.StencilOperationDecrementWrap:
		return gl.DECR_WRAP
	default:
		return gl.KEEP
	}
}

// compareFunctionToGL converts compare function to GL constant.
// compareFunctionToGL converts compare function to GL constant.
func compareFunctionToGL(fn gputypes.CompareFunction) uint32 {
	switch fn {
	case gputypes.CompareFunctionNever:
		return gl.NEVER
	case gputypes.CompareFunctionLess:
		return gl.LESS
	case gputypes.CompareFunctionEqual:
		return gl.EQUAL
	case gputypes.CompareFunctionLessEqual:
		return gl.LEQUAL
	case gputypes.CompareFunctionGreater:
		return gl.GREATER
	case gputypes.CompareFunctionNotEqual:
		return gl.NOTEQUAL
	case gputypes.CompareFunctionGreaterEqual:
		return gl.GEQUAL
	case gputypes.CompareFunctionAlways:
		return gl.ALWAYS
	default:
		return gl.ALWAYS
	}
}

// blendFactorToGL converts a WebGPU blend factor to the corresponding GL constant.
// blendFactorToGL converts a WebGPU blend factor to the corresponding GL constant.
func blendFactorToGL(f gputypes.BlendFactor) uint32 {
	switch f {
	case gputypes.BlendFactorZero:
		return gl.ZERO
	case gputypes.BlendFactorOne:
		return gl.ONE
	case gputypes.BlendFactorSrc:
		return gl.SRC_COLOR
	case gputypes.BlendFactorOneMinusSrc:
		return gl.ONE_MINUS_SRC_COLOR
	case gputypes.BlendFactorSrcAlpha:
		return gl.SRC_ALPHA
	case gputypes.BlendFactorOneMinusSrcAlpha:
		return gl.ONE_MINUS_SRC_ALPHA
	case gputypes.BlendFactorDst:
		return gl.DST_COLOR
	case gputypes.BlendFactorOneMinusDst:
		return gl.ONE_MINUS_DST_COLOR
	case gputypes.BlendFactorDstAlpha:
		return gl.DST_ALPHA
	case gputypes.BlendFactorOneMinusDstAlpha:
		return gl.ONE_MINUS_DST_ALPHA
	case gputypes.BlendFactorSrcAlphaSaturated:
		return gl.SRC_ALPHA_SATURATE
	case gputypes.BlendFactorConstant:
		return gl.CONSTANT_COLOR
	case gputypes.BlendFactorOneMinusConstant:
		return gl.ONE_MINUS_CONSTANT_COLOR
	default:
		return gl.ONE
	}
}

// blendOperationToGL converts a WebGPU blend operation to the corresponding GL constant.
// blendOperationToGL converts a WebGPU blend operation to the corresponding GL constant.
func blendOperationToGL(op gputypes.BlendOperation) uint32 {
	switch op {
	case gputypes.BlendOperationAdd:
		return gl.FUNC_ADD
	case gputypes.BlendOperationSubtract:
		return gl.FUNC_SUBTRACT
	case gputypes.BlendOperationReverseSubtract:
		return gl.FUNC_REVERSE_SUBTRACT
	case gputypes.BlendOperationMin:
		return gl.MIN
	case gputypes.BlendOperationMax:
		return gl.MAX
	default:
		return gl.FUNC_ADD
	}
}

// primitiveTopologyToGL converts a WebGPU primitive topology to the corresponding GL constant.
// primitiveTopologyToGL converts a WebGPU primitive topology to the corresponding GL constant.
func primitiveTopologyToGL(topology gputypes.PrimitiveTopology) uint32 {
	switch topology {
	case gputypes.PrimitiveTopologyPointList:
		return gl.POINTS
	case gputypes.PrimitiveTopologyLineList:
		return gl.LINES
	case gputypes.PrimitiveTopologyLineStrip:
		return gl.LINE_STRIP
	case gputypes.PrimitiveTopologyTriangleList:
		return gl.TRIANGLES
	case gputypes.PrimitiveTopologyTriangleStrip:
		return gl.TRIANGLE_STRIP
	default:
		return gl.TRIANGLES
	}
}
