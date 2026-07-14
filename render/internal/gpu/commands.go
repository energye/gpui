//go:build !nogpu

package gpu

import (
	"github.com/energye/gpui/gpu/webgpu"
)

// CommandEncoder is a legacy lightweight command-recording helper kept for
// older unit tests.
//
// Runtime rendering should use CoreCommandEncoder, which wraps webgpu objects
// and submits real commands through rwgpu/wgpu-native. This type intentionally
// preserves the old stub ID API so existing pure state-machine tests do not
// require a native GPU device.
type CommandEncoder struct {
	device  *webgpu.Device
	encoder StubCommandEncoderID

	// State tracking
	hasActivePass bool
	passCount     int
}

// StubCommandEncoderID is a legacy logical marker for command-encoder tests.
type StubCommandEncoderID uint64

// StubCommandBufferID is a legacy logical marker for command-buffer tests.
type StubCommandBufferID uint64

// NewCommandEncoder creates a legacy test command encoder.
func NewCommandEncoder(device *webgpu.Device) *CommandEncoder {
	return &CommandEncoder{
		device:  device,
		encoder: StubCommandEncoderID(1),
	}
}

// BeginRenderPass begins a new render pass targeting the specified texture.
// If clearTarget is true, the texture is cleared to transparent before drawing.
func (e *CommandEncoder) BeginRenderPass(target *GPUTexture, clearTarget bool) *RenderPass {
	if e.hasActivePass {
		// Can't begin a new pass while one is active
		return nil
	}

	e.hasActivePass = true
	e.passCount++

	return &RenderPass{
		encoder: e,
		//nolint:gosec // passCount is incremented sequentially, overflow not possible in practice
		pass:   StubRenderPassID(e.passCount),
		target: target,
	}
}

// BeginComputePass begins a new compute pass.
func (e *CommandEncoder) BeginComputePass() *ComputePass {
	if e.hasActivePass {
		return nil
	}

	e.hasActivePass = true
	e.passCount++

	return &ComputePass{
		encoder: e,
		//nolint:gosec // passCount is incremented sequentially, overflow not possible in practice
		pass: StubComputePassID(e.passCount),
	}
}

// CopyTextureToTexture records a legacy logical copy operation.
func (e *CommandEncoder) CopyTextureToTexture(src, dst *GPUTexture, width, height int) {
	if e.hasActivePass {
		// Can't copy while a pass is active
		return
	}

	_ = src
	_ = dst
	_ = width
	_ = height
}

// CopyTextureToBuffer records a legacy logical readback copy operation.
func (e *CommandEncoder) CopyTextureToBuffer(src *GPUTexture, dst StubBufferID, bytesPerRow uint32) {
	if e.hasActivePass {
		return
	}

	_ = src
	_ = dst
	_ = bytesPerRow
}

// Finish completes the legacy command encoder and returns its logical command
// buffer marker.
func (e *CommandEncoder) Finish() StubCommandBufferID {
	return StubCommandBufferID(1)
}

// PassCount returns the number of passes recorded.
func (e *CommandEncoder) PassCount() int {
	return e.passCount
}

// endPass is called by passes when they end.
func (e *CommandEncoder) endPass() {
	e.hasActivePass = false
}

// RenderPass represents an active render pass for draw commands.
// Draw commands can only be issued while a render pass is active.
type RenderPass struct {
	encoder *CommandEncoder
	pass    StubRenderPassID
	target  *GPUTexture

	// State
	pipelineBound bool
	bindGroupSet  bool
}

// StubRenderPassID is a legacy logical marker for render-pass tests.
type StubRenderPassID uint64

// SetPipeline marks a legacy render pipeline as bound.
func (p *RenderPass) SetPipeline(pipeline StubPipelineID) {
	_ = pipeline
	p.pipelineBound = true
}

// SetBindGroup marks a legacy bind group as set.
func (p *RenderPass) SetBindGroup(index uint32, bindGroup StubBindGroupID) {
	_ = index
	_ = bindGroup
	p.bindGroupSet = true
}

// SetVertexBuffer records a legacy vertex buffer binding.
func (p *RenderPass) SetVertexBuffer(slot uint32, buffer StubBufferID) {
	_ = slot
	_ = buffer
}

// SetIndexBuffer records a legacy index buffer binding.
func (p *RenderPass) SetIndexBuffer(buffer StubBufferID, format IndexFormat) {
	_ = buffer
	_ = format
}

// Draw issues a non-indexed draw call.
// vertexCount: number of vertices to draw
// instanceCount: number of instances to draw
// firstVertex: offset into the vertex buffer
// firstInstance: instance ID offset
func (p *RenderPass) Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32) {
	if !p.pipelineBound {
		return
	}

	_ = vertexCount
	_ = instanceCount
	_ = firstVertex
	_ = firstInstance
}

// DrawIndexed issues an indexed draw call.
func (p *RenderPass) DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32) {
	if !p.pipelineBound {
		return
	}

	_ = indexCount
	_ = instanceCount
	_ = firstIndex
	_ = baseVertex
	_ = firstInstance
}

// DrawFullScreenTriangle is a convenience method for drawing a full-screen triangle.
// This is commonly used for post-processing effects and texture blits.
// Uses 3 vertices with no instance or offset.
func (p *RenderPass) DrawFullScreenTriangle() {
	p.Draw(3, 1, 0, 0)
}

// End finishes the render pass.
// No more draw calls can be issued after this.
func (p *RenderPass) End() {
	p.encoder.endPass()
}

// Target returns the render target texture.
func (p *RenderPass) Target() *GPUTexture {
	return p.target
}

// ComputePass represents an active compute pass for dispatch commands.
type ComputePass struct {
	encoder *CommandEncoder
	pass    StubComputePassID

	// State
	pipelineBound bool
	bindGroupSet  bool
}

// StubComputePassID is a legacy logical marker for compute-pass tests.
type StubComputePassID uint64

// SetPipeline marks a legacy compute pipeline as bound.
func (p *ComputePass) SetPipeline(pipeline StubComputePipelineID) {
	_ = pipeline
	p.pipelineBound = true
}

// SetBindGroup marks a legacy compute bind group as set.
func (p *ComputePass) SetBindGroup(index uint32, bindGroup StubBindGroupID) {
	_ = index
	_ = bindGroup
	p.bindGroupSet = true
}

// DispatchWorkgroups dispatches compute work.
// workgroupCountX/Y/Z: number of workgroups in each dimension
func (p *ComputePass) DispatchWorkgroups(workgroupCountX, workgroupCountY, workgroupCountZ uint32) {
	if !p.pipelineBound {
		return
	}

	_ = workgroupCountX
	_ = workgroupCountY
	_ = workgroupCountZ
}

// DispatchWorkgroupsForSize calculates and dispatches workgroups for a given work size.
// workSize: total number of work items
// workgroupSize: number of items per workgroup (typically 64 or 256)
func (p *ComputePass) DispatchWorkgroupsForSize(workSize, workgroupSize uint32) {
	if workgroupSize == 0 {
		workgroupSize = 64
	}
	workgroups := (workSize + workgroupSize - 1) / workgroupSize
	p.DispatchWorkgroups(workgroups, 1, 1)
}

// End finishes the compute pass.
func (p *ComputePass) End() {
	p.encoder.endPass()
}

// IndexFormat specifies the format of index buffer elements.
type IndexFormat uint32

const (
	// IndexFormatUint16 uses 16-bit unsigned integers.
	IndexFormatUint16 IndexFormat = 0

	// IndexFormatUint32 uses 32-bit unsigned integers.
	IndexFormatUint32 IndexFormat = 1
)

// CommandBuffer represents a finished command buffer ready for submission.
type CommandBuffer struct {
	id StubCommandBufferID
}

// NewCommandBuffer wraps a command buffer ID.
func NewCommandBuffer(id StubCommandBufferID) *CommandBuffer {
	return &CommandBuffer{id: id}
}

// ID returns the underlying command buffer ID.
func (b *CommandBuffer) ID() StubCommandBufferID {
	return b.id
}

// QueueSubmitter is a legacy logical queue helper for tests.
type QueueSubmitter struct {
	queue *webgpu.Queue
}

// NewQueueSubmitter creates a legacy logical queue helper.
func NewQueueSubmitter(queue *webgpu.Queue) *QueueSubmitter {
	return &QueueSubmitter{queue: queue}
}

// Submit records logical command buffer submission.
func (s *QueueSubmitter) Submit(buffers ...*CommandBuffer) {
	if len(buffers) == 0 {
		return
	}

	_ = buffers
}

// WriteBuffer records a legacy logical buffer write.
func (s *QueueSubmitter) WriteBuffer(buffer StubBufferID, offset uint64, data []byte) {
	_ = buffer
	_ = offset
	_ = data
}

// WriteTexture records a legacy logical texture write.
func (s *QueueSubmitter) WriteTexture(texture *GPUTexture, data []byte) {
	_ = texture
	_ = data
}

// RenderCommandBuilder provides a fluent API for building render commands.
type RenderCommandBuilder struct {
	encoder *CommandEncoder
	pass    *RenderPass
}

// NewRenderCommandBuilder creates a new render command builder.
func NewRenderCommandBuilder(device *webgpu.Device, target *GPUTexture, clearTarget bool) *RenderCommandBuilder {
	encoder := NewCommandEncoder(device)
	pass := encoder.BeginRenderPass(target, clearTarget)

	return &RenderCommandBuilder{
		encoder: encoder,
		pass:    pass,
	}
}

// SetPipeline sets the render pipeline.
func (b *RenderCommandBuilder) SetPipeline(pipeline StubPipelineID) *RenderCommandBuilder {
	b.pass.SetPipeline(pipeline)
	return b
}

// SetBindGroup sets a bind group.
func (b *RenderCommandBuilder) SetBindGroup(index uint32, bindGroup StubBindGroupID) *RenderCommandBuilder {
	b.pass.SetBindGroup(index, bindGroup)
	return b
}

// Draw issues a draw call.
func (b *RenderCommandBuilder) Draw(vertexCount, instanceCount uint32) *RenderCommandBuilder {
	b.pass.Draw(vertexCount, instanceCount, 0, 0)
	return b
}

// DrawFullScreen draws a full-screen triangle.
func (b *RenderCommandBuilder) DrawFullScreen() *RenderCommandBuilder {
	b.pass.DrawFullScreenTriangle()
	return b
}

// Finish ends the pass and returns the command buffer.
func (b *RenderCommandBuilder) Finish() StubCommandBufferID {
	b.pass.End()
	return b.encoder.Finish()
}

// ComputeCommandBuilder provides a fluent API for building compute commands.
type ComputeCommandBuilder struct {
	encoder *CommandEncoder
	pass    *ComputePass
}

// NewComputeCommandBuilder creates a new compute command builder.
func NewComputeCommandBuilder(device *webgpu.Device) *ComputeCommandBuilder {
	encoder := NewCommandEncoder(device)
	pass := encoder.BeginComputePass()

	return &ComputeCommandBuilder{
		encoder: encoder,
		pass:    pass,
	}
}

// SetPipeline sets the compute pipeline.
func (b *ComputeCommandBuilder) SetPipeline(pipeline StubComputePipelineID) *ComputeCommandBuilder {
	b.pass.SetPipeline(pipeline)
	return b
}

// SetBindGroup sets a bind group.
func (b *ComputeCommandBuilder) SetBindGroup(index uint32, bindGroup StubBindGroupID) *ComputeCommandBuilder {
	b.pass.SetBindGroup(index, bindGroup)
	return b
}

// Dispatch dispatches workgroups.
func (b *ComputeCommandBuilder) Dispatch(x, y, z uint32) *ComputeCommandBuilder {
	b.pass.DispatchWorkgroups(x, y, z)
	return b
}

// DispatchForSize calculates and dispatches for a work size.
func (b *ComputeCommandBuilder) DispatchForSize(size, groupSize uint32) *ComputeCommandBuilder {
	b.pass.DispatchWorkgroupsForSize(size, groupSize)
	return b
}

// Finish ends the pass and returns the command buffer.
func (b *ComputeCommandBuilder) Finish() StubCommandBufferID {
	b.pass.End()
	return b.encoder.Finish()
}
