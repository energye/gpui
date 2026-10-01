//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Surface and SurfaceTexture are defined in platform-specific files (resource_windows.go, resource_linux.go)

// Buffer implements hal.Buffer for OpenGL.
type Buffer struct {
	id     uint32 // GL buffer object ID
	target uint32 // GL_ARRAY_BUFFER, GL_UNIFORM_BUFFER, etc.
	size   uint64
	usage  gputypes.BufferUsage
	label  string
	glCtx  *gl.Context
	ctx    *AdapterContext // owning context for self-locked Destroy (deferred-release safe)
	mapped []byte          // For mapped buffers
	data   []byte          // CPU-side storage for readback (populated by CopyTextureToBuffer)
}

// Size returns the buffer size in bytes.
func (b *Buffer) Size() uint64 { return b.size }

// Usage returns the buffer's usage flags.
func (b *Buffer) Usage() gputypes.BufferUsage { return b.usage }

// Label returns the buffer's debug label.
func (b *Buffer) Label() string { return b.label }

// withGL runs fn with a current GL context: self-locked via the owning
// AdapterContext when present, else the stored context directly. Shared by
// every GL-object Destroy below — a delete with no current context is a
// silent no-op on NVIDIA while the ledger already refunded, so every
// resize leaks until OOM. Reports whether fn ran: false means no context
// available or the bind failed (TryLock, not Lock: the old Lock handed
// back a usable-looking context on bind failure, so deletes ran into the
// void). Callers restore ids AND ledger slots on false for retry.
func withGL(ctx *AdapterContext, fallback *gl.Context, fn func(glCtx *gl.Context)) (ran bool) {
	if ctx != nil {
		glCtx, err := ctx.TryLock()
		if err != nil {
			return false
		}
		defer ctx.Unlock()
		if glCtx != nil {
			fn(glCtx)
			return true
		}
		return false
	}
	if fallback != nil {
		fn(fallback)
		return true
	}
	return false
}

// Destroy releases the buffer.
// Self-locked via the owning AdapterContext: deferred-release queues
// (pendingTexRetire/pendingBufRetire, offscreen/stencil pools) drain without
// holding the GL lock, and a delete with no current context is a silent
// no-op on NVIDIA — ledger forgets but driver keeps the memory, so every
// resize leaks until OOM. Locking here makes direct Destroy safe anywhere;
// Device.DestroyBuffer delegates here without pre-locking (no nesting).
// Bind failure leaves the id AND the ledger slot in place for the next
// attempt (same retry contract as Sampler.Destroy).
func (b *Buffer) Destroy() {
	if b == nil {
		return
	}
	if b.id == 0 {
		return
	}
	id := b.id
	b.id = 0
	hal.VramForget(vramBufferHandle(id))
	if !withGL(b.ctx, b.glCtx, func(glCtx *gl.Context) {
		glCtx.DeleteBuffers(id)
	}) && b.ctx != nil {
		// Bind failed: restore id and ledger slot so a later Destroy
		// retries instead of leaking driver memory.
		b.id = id
		hal.VramAdd(vramBufferHandle(id), b.size)
	}
}

// NativeHandle returns the GL buffer object ID.
func (b *Buffer) NativeHandle() uintptr { return uintptr(b.id) }

// Texture implements hal.Texture for OpenGL.
type Texture struct {
	id          uint32 // GL texture object ID
	target      uint32 // GL_TEXTURE_2D, GL_TEXTURE_2D_MULTISAMPLE, etc.
	format      gputypes.TextureFormat
	dimension   gputypes.TextureDimension
	size        hal.Extent3D
	mipLevels   uint32
	sampleCount uint32 // 1 for regular textures, >1 for MSAA
	fbo         uint32 // GL framebuffer object ID (0 = no FBO created)
	glCtx       *gl.Context
	ctx         *AdapterContext // owning context for self-locked Destroy (deferred-release safe)
}

// CurrentUsage returns 0 — GLES has no explicit resource state tracking.
func (t *Texture) CurrentUsage() gputypes.TextureUsage { return 0 }
func (t *Texture) AddPendingRef()                      {}
func (t *Texture) DecPendingRef()                      {}

// Format returns the texture format.
func (t *Texture) Format() gputypes.TextureFormat { return t.format }

// Destroy releases the texture and any associated framebuffer object.
// Self-locked via the owning AdapterContext for the same reason as
// Buffer.Destroy: resize churn retires depth/stencil textures through
// pending queues that drain with no context current; without the lock the
// driver keeps the storage while the ledger already refunded it — each size
// change adds VRAM until OOM. Device.DestroyTexture delegates here without
// pre-locking (no nesting).
func (t *Texture) Destroy() {
	if t == nil {
		return
	}
	fbo, id := t.fbo, t.id
	if fbo == 0 && id == 0 {
		return
	}
	t.fbo, t.id = 0, 0
	if id != 0 {
		hal.VramForget(vramTextureHandle(id))
	}
	// Ledger estimate for the restore path below: same inputs as
	// CreateTexture (device_linux.go/device.go), so refund and re-charge
	// agree to the byte. Non-MSAA textures were charged with their
	// recorded sampleCount (min 1).
	sc := t.sampleCount
	if sc == 0 {
		sc = 1
	}
	need := hal.VramTextureBytes(
		gputypes.Extent3D{Width: t.size.Width, Height: t.size.Height, DepthOrArrayLayers: t.size.DepthOrArrayLayers},
		t.size.DepthOrArrayLayers,
		t.mipLevels, sc, t.format)
	if !withGL(t.ctx, t.glCtx, func(glCtx *gl.Context) {
		if fbo != 0 {
			glCtx.DeleteFramebuffers(fbo)
		}
		if id != 0 {
			glCtx.DeleteTextures(id)
		}
	}) && t.ctx != nil && id != 0 {
		// Bind failed: restore id/fbo and ledger slot so a later Destroy
		// retries instead of leaking driver memory.
		t.fbo, t.id = fbo, id
		hal.VramAdd(vramTextureHandle(id), need)
	}
}

// NativeHandle returns the GL texture object ID.
func (t *Texture) NativeHandle() uintptr { return uintptr(t.id) }

// TextureView implements hal.TextureView for OpenGL.
type TextureView struct {
	texture    *Texture
	aspect     gputypes.TextureAspect
	baseMip    uint32
	mipCount   uint32
	baseLayer  uint32
	layerCount uint32
	isSurface  bool            // true for default framebuffer (surface texture)
	surfaceTex *SurfaceTexture // non-nil only when isSurface is true
}

// Destroy is a no-op for texture views in OpenGL.
func (v *TextureView) Destroy() {}

// Texture returns the parent texture.
func (v *TextureView) Texture() hal.Texture {
	if v.texture == nil {
		return nil
	}
	return v.texture
}

// NativeHandle returns the underlying texture's GL object ID.
func (v *TextureView) NativeHandle() uintptr {
	if v.texture != nil {
		return uintptr(v.texture.id)
	}
	return 0
}

// Sampler implements hal.Sampler for OpenGL using GL sampler objects (GL 3.3+).
type Sampler struct {
	id    uint32 // GL sampler object ID (0 if sampler objects not supported)
	glCtx *gl.Context
	ctx   *AdapterContext // owning context for self-locked Destroy (same as Buffer/Texture)
}

// Destroy releases the GL sampler object.
// Self-locked via the owning AdapterContext (same contract as
// Buffer.Destroy): a delete with no current context is a silent no-op while
// the ledger already refunded, so bind failure leaves the ids AND the
// ledger slot in place for the next attempt. Device.DestroySampler
// delegates here without pre-locking (no nesting).
func (s *Sampler) Destroy() {
	if s == nil {
		return
	}
	id := s.id
	if id != 0 {
		hal.VramForget(vramSamplerHandle(id))
	}
	s.id = 0
	if id == 0 {
		return
	}
	if !withGL(s.ctx, s.glCtx, func(glCtx *gl.Context) {
		glCtx.DeleteSamplers(id)
	}) {
		if s.ctx == nil {
			return
		}
		// Bind failed: restore id and ledger slot so a later Destroy
		// retries instead of leaking driver memory.
		s.id = id
		hal.VramAdd(vramSamplerHandle(id), hal.VramSamplerBytes)
	}
}

// NativeHandle returns the GL sampler object ID.
func (s *Sampler) NativeHandle() uintptr { return uintptr(s.id) }

// ShaderModule implements hal.ShaderModule for OpenGL.
type ShaderModule struct {
	vertexID   uint32 // GL shader object ID for vertex
	fragmentID uint32 // GL shader object ID for fragment
	computeID  uint32 // GL shader object ID for compute
	wgsl       string
	spirv      []uint32
	glCtx      *gl.Context
	ctx        *AdapterContext // owning context for self-locked Destroy (same as Buffer)
}

// Destroy releases the shader module.
// Same retry contract as Buffer.Destroy: bind failure restores ids AND any
// ledger slot via withGL accounting below (shaders carry no ledger charge;
// only the GL ids are restored for retry).
func (m *ShaderModule) Destroy() {
	if m == nil {
		return
	}
	vid, fid, cid := m.vertexID, m.fragmentID, m.computeID
	if vid == 0 && fid == 0 && cid == 0 {
		return
	}
	m.vertexID, m.fragmentID, m.computeID = 0, 0, 0
	if !withGL(m.ctx, m.glCtx, func(glCtx *gl.Context) {
		if vid != 0 {
			glCtx.DeleteShader(vid)
		}
		if fid != 0 {
			glCtx.DeleteShader(fid)
		}
		if cid != 0 {
			glCtx.DeleteShader(cid)
		}
	}) && m.ctx != nil {
		m.vertexID, m.fragmentID, m.computeID = vid, fid, cid
	}
}

// BindGroupLayout implements hal.BindGroupLayout for OpenGL.
type BindGroupLayout struct {
	entries []gputypes.BindGroupLayoutEntry
}

// Destroy is a no-op for bind group layouts.
func (l *BindGroupLayout) Destroy() {}

// BindGroup implements hal.BindGroup for OpenGL.
// Entries hold the canonical hal flat shape directly:
// CreateBindGroup stores desc.Entries verbatim, Execute unpacks to *Buffer/
// *TextureView/*Sampler internally (gpu unpack pattern, not red-line assert).
type BindGroup struct {
	layout  *BindGroupLayout
	entries []hal.BindGroupEntry
}

// Destroy is a no-op for bind groups.
func (g *BindGroup) Destroy() {}

// BindGroupLayoutInfo stores per-group binding-to-slot mapping computed at
// PipelineLayout creation time.
// Each entry in BindingToSlot is indexed by the WGSL binding number and maps
// to the sequential GL slot index for that resource type. 0xFF means unused.
type BindGroupLayoutInfo struct {
	BindingToSlot []uint8 // indexed by binding number, 0xFF = unused
}

// PipelineLayout implements hal.PipelineLayout for OpenGL.
// Stores pre-computed per-type sequential binding indices.
type PipelineLayout struct {
	bindGroupLayouts []*BindGroupLayout
	// groupInfos stores per-group binding-to-slot tables computed from per-type
	// sequential counters (samplers, textures, images, uniform buffers, storage buffers).
	groupInfos []BindGroupLayoutInfo
	// Computed simultaneously with groupInfos in CreatePipelineLayout.
	bindingMap map[glsl.BindingMapKey]uint8
}

// Destroy is a no-op for pipeline layouts.
func (l *PipelineLayout) Destroy() {}

// ColorTargetDesc describes per-render-target blend and write mask state.
type ColorTargetDesc struct {
	Blend     *gputypes.BlendState
	WriteMask gputypes.ColorWriteMask
}

// RenderPipeline implements hal.RenderPipeline for OpenGL.
type RenderPipeline struct {
	programID uint32 // GL program object ID
	layout    *PipelineLayout
	glCtx     *gl.Context
	ctx       *AdapterContext // owning context for self-locked Destroy (same as Buffer)

	// Pipeline state
	primitiveTopology gputypes.PrimitiveTopology
	cullMode          gputypes.CullMode
	frontFace         gputypes.FrontFace
	depthStencil      *hal.DepthStencilState
	multisample       gputypes.MultisampleState

	// Per-target blend and write mask state for MRT.
	// When len == 1, uniform blend/mask is applied globally.
	// When len > 1, indexed GL calls are used if available (GLES 3.2 / GL 4.0).
	colorTargets []ColorTargetDesc

	// Vertex buffer layouts from the pipeline descriptor.
	// OpenGL requires explicit glVertexAttribPointer calls to configure
	// how vertex data is interpreted. This is stored here so that
	// SetVertexBuffer can configure attributes using the pipeline's layout.
	vertexBuffers []gputypes.VertexBufferLayout

	// samplerBindMap maps texture unit indices to sampler unit indices.
	// When binding textures, the associated sampler must be bound to the SAME
	// texture unit (not the sampler's own WGSL binding).
	samplerBindMap [maxTextureSlots]int8 // -1 = no sampler, otherwise = sampler glBinding
}

const maxTextureSlots = 32

// Destroy releases the render pipeline.
// Same retry contract as Buffer.Destroy: bind failure restores programID
// AND the ledger slot so a later Destroy retries instead of leaking.
func (p *RenderPipeline) Destroy() {
	if p == nil {
		return
	}
	id := p.programID
	if id == 0 {
		return
	}
	p.programID = 0
	hal.VramForget(vramProgramHandle(id))
	if !withGL(p.ctx, p.glCtx, func(glCtx *gl.Context) {
		glCtx.DeleteProgram(id)
	}) && p.ctx != nil {
		p.programID = id
		hal.VramAdd(vramProgramHandle(id), hal.VramPipelineBytes)
	}
}

// ComputePipeline implements hal.ComputePipeline for OpenGL.
type ComputePipeline struct {
	programID uint32
	layout    *PipelineLayout
	glCtx     *gl.Context
	ctx       *AdapterContext // owning context for self-locked Destroy (same as Buffer)
}

// Destroy releases the compute pipeline.
// Same retry contract as RenderPipeline.Destroy.
func (p *ComputePipeline) Destroy() {
	if p == nil {
		return
	}
	id := p.programID
	if id == 0 {
		return
	}
	p.programID = 0
	hal.VramForget(vramProgramHandle(id))
	if !withGL(p.ctx, p.glCtx, func(glCtx *gl.Context) {
		glCtx.DeleteProgram(id)
	}) && p.ctx != nil {
		p.programID = id
		hal.VramAdd(vramProgramHandle(id), hal.VramPipelineBytes)
	}
}

// glFence holds a GL sync object paired with its submission value.
type glFence struct {
	sync  uintptr // GL sync object handle from glFenceSync
	value uint64  // submission index this fence was signaled with
}

// Fence implements hal.Fence using GL sync objects (glFenceSync).
// Tracks pending GL sync objects and polls their completion status.
//
// Threading contract: every method below issues GL calls on the calling
// thread, so the caller must hold the AdapterContext lock (which pins the
// goroutine and makes the context current) — Queue.Submit holds it for
// Signal/Maintain, Queue.Poll and Device.WaitForFence/GetFenceStatus take
// it around GetLatest/Wait/Reset. Destroy/Maintain self-lock via Fence.ctx
// like any other GL object. The only lock-free path is the counter
// fallback (nil glCtx or no fence-sync support), which touches no GL.
type Fence struct {
	lastCompleted atomic.Uint64 // highest known completed value
	pending       []glFence     // GL sync objects awaiting completion
	glCtx         *gl.Context
	ctx           *AdapterContext // owning context for self-locked Destroy/Maintain (same as Buffer)
}

// NewFence creates a new fence. Callers that own an AdapterContext should
// set Fence.ctx (e.g. Device.CreateFence does) so Destroy/Reset can
// self-lock and retry on bind failure; queue-owned fences are covered by
// the Submit/Poll lock instead.
func NewFence(glCtx *gl.Context) *Fence {
	return &Fence{
		glCtx: glCtx,
	}
}

// Signal inserts a GL fence sync object into the command stream at the given value.
// Must be called on the GL thread BEFORE glFlush — the flush sends both the
// preceding commands and this fence to the GPU together.
// Returns an error if glFenceSync fails (typically OOM).
func (f *Fence) Signal(value uint64) error {
	if f.glCtx == nil || !f.glCtx.SupportsFenceSync() {
		f.lastCompleted.Store(value)
		return nil
	}
	sync := f.glCtx.FenceSync(gl.SYNC_GPU_COMMANDS_COMPLETE, 0)
	if sync == 0 {
		return fmt.Errorf("gles: glFenceSync failed (OOM)")
	}
	f.pending = append(f.pending, glFence{sync: sync, value: value})
	return nil
}

// GetLatest polls pending sync objects and returns the highest completed value.
func (f *Fence) GetLatest() uint64 {
	maxValue := f.lastCompleted.Load()

	if f.glCtx == nil || !f.glCtx.SupportsFenceSync() {
		return maxValue
	}

	for _, gf := range f.pending {
		if gf.value <= maxValue {
			continue // already known complete
		}
		status := f.glCtx.GetSyncStatus(gf.sync)
		if status == gl.SIGNALED {
			maxValue = gf.value
		} else {
			// Anything after the first unsignalled is guaranteed unsignalled.
			break
		}
	}

	// Cache the latest value to avoid redundant queries.
	for {
		old := f.lastCompleted.Load()
		if maxValue <= old {
			break
		}
		if f.lastCompleted.CompareAndSwap(old, maxValue) {
			break
		}
	}

	return maxValue
}

// Maintain cleans up completed sync objects.
func (f *Fence) Maintain() {
	if f.glCtx == nil || !f.glCtx.SupportsFenceSync() {
		return
	}

	latest := f.GetLatest()

	// Delete completed sync objects.
	n := 0
	for _, gf := range f.pending {
		if gf.value <= latest {
			f.glCtx.DeleteSync(gf.sync)
		} else {
			f.pending[n] = gf
			n++
		}
	}
	f.pending = f.pending[:n]
}

// Wait waits for the fence to reach the specified value with timeout.
func (f *Fence) Wait(waitValue uint64, timeout time.Duration) bool {
	if f.lastCompleted.Load() >= waitValue {
		return true
	}

	if f.glCtx == nil || !f.glCtx.SupportsFenceSync() {
		return f.lastCompleted.Load() >= waitValue
	}

	// Find a matching pending fence.
	var target *glFence
	for i := range f.pending {
		if f.pending[i].value >= waitValue {
			target = &f.pending[i]
			break
		}
	}
	if target == nil {
		return false // value not yet signaled
	}

	// Convert timeout to nanoseconds for glClientWaitSync.
	timeoutNS := uint64(timeout.Nanoseconds())
	if timeoutNS > uint64(^uint32(0)) {
		timeoutNS = uint64(^uint32(0)) // cap to max u32 for safety
	}

	status := f.glCtx.ClientWaitSync(target.sync, gl.SYNC_FLUSH_COMMANDS_BIT, timeoutNS)

	signaled := status == gl.ALREADY_SIGNALED || status == gl.CONDITION_SATISFIED
	if signaled {
		// Update last completed.
		for {
			old := f.lastCompleted.Load()
			if waitValue <= old {
				break
			}
			if f.lastCompleted.CompareAndSwap(old, waitValue) {
				break
			}
		}
	}
	return signaled
}

// GetValue returns the current fence value (non-blocking poll).
func (f *Fence) GetValue() uint64 {
	return f.GetLatest()
}

// Reset resets the fence to the unsignaled state.
// Same retry contract as Buffer.Destroy: bind failure restores the pending
// sync list so a later Reset retries instead of leaking sync objects.
func (f *Fence) Reset() {
	if f == nil {
		return
	}
	pending := f.pending
	f.lastCompleted.Store(0)
	f.pending = f.pending[:0]
	if len(pending) == 0 {
		return
	}
	if !withGL(f.ctx, f.glCtx, func(glCtx *gl.Context) {
		for _, gf := range pending {
			glCtx.DeleteSync(gf.sync)
		}
	}) && f.ctx != nil {
		f.pending = append(f.pending, pending...)
	}
}

// Destroy releases all fence resources.
// Same retry contract as Reset: bind failure restores pending so a later
// Destroy retries instead of leaking sync objects.
func (f *Fence) Destroy() {
	if f == nil {
		return
	}
	pending := f.pending
	f.pending = nil
	if len(pending) == 0 {
		return
	}
	if !withGL(f.ctx, f.glCtx, func(glCtx *gl.Context) {
		for _, gf := range pending {
			glCtx.DeleteSync(gf.sync)
		}
	}) && f.ctx != nil {
		f.pending = pending
	}
}

// QuerySet implements hal.QuerySet for OpenGL.
// Stores GL query object IDs and the target type (GL_TIMESTAMP or GL_ANY_SAMPLES_PASSED).
type QuerySet struct {
	queries []uint32 // GL query object IDs
	target  uint32   // GL_TIMESTAMP or GL_ANY_SAMPLES_PASSED_CONSERVATIVE
	glCtx   *gl.Context
	ctx     *AdapterContext // owning context for self-locked Destroy (same as Buffer)
}

// Target returns the GL query target type (GL_TIMESTAMP or GL_ANY_SAMPLES_PASSED).
func (q *QuerySet) Target() uint32 { return q.target }

// Destroy releases all GL query objects.
// Self-locked via the owning AdapterContext (same contract as
// Buffer.Destroy): queries carry no ledger charge, only the ids are
// restored for retry on bind failure.
func (q *QuerySet) Destroy() {
	if q == nil {
		return
	}
	ids := q.queries
	if len(ids) == 0 {
		return
	}
	q.queries = nil
	if !withGL(q.ctx, q.glCtx, func(glCtx *gl.Context) {
		glCtx.DeleteQueries(int32(len(ids)), &ids[0])
	}) && q.ctx != nil {
		q.queries = ids
	}
}

// NativeHandle returns 0 (no single native handle for a query set).
func (q *QuerySet) NativeHandle() uintptr { return 0 }
