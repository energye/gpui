//go:build !(js && wasm)

package hal

import gputypes "github.com/energye/gpui/gpu/types"

// CommandEncoder records GPU commands.
//
// Lifecycle: After EndEncoding, the encoder is in "closed" state. After
// ResetAll (with completed command buffers), the encoder is ready for a
// new BeginEncoding cycle. This enables encoder pooling at the wgpu core
// level, matching Rust wgpu-core's CommandAllocator pattern.
type CommandEncoder interface {
	// BeginEncoding begins command recording with an optional label.
	BeginEncoding(label string) error

	// EndEncoding finishes command recording and returns a command buffer.
	// The encoder enters "closed" state but retains its internal resources
	// (e.g., DX12 allocator, Vulkan command pool). Call ResetAll after GPU
	// completion to prepare for the next BeginEncoding cycle.
	EndEncoding() (CommandBuffer, error)

	// DiscardEncoding discards the encoder without creating a command buffer.
	// Use this to cancel encoding that encountered errors.
	DiscardEncoding()

	// ResetAll resets the encoder and associated command buffers for reuse.
	// Must be called after the GPU has completed all commands from previous
	// EndEncoding calls. After ResetAll, the encoder is ready for BeginEncoding.
	// Not all backends need this (GLES/Software/Metal/Noop are no-ops).
	ResetAll(commandBuffers []CommandBuffer)

	// Destroy releases all GPU resources owned by this encoder.
	// Must be called when the encoder is no longer needed (e.g., during
	// device shutdown or encoder pool cleanup). The encoder must not be
	// in recording state when Destroy is called.
	Destroy()

	// TransitionBuffers transitions buffer states for synchronization.
	// This is required on some backends (Vulkan, DX12) but no-op on others (Metal).
	TransitionBuffers(barriers []BufferBarrier)

	// TransitionTextures transitions texture states for synchronization.
	// This is required on some backends (Vulkan, DX12) but no-op on others (Metal).
	TransitionTextures(barriers []TextureBarrier)

	// ClearBuffer clears a buffer region to zero.
	ClearBuffer(buffer Buffer, offset, size uint64)

	// CopyBufferToBuffer copies data between buffers.
	// Matches webgpu CommandEncoder.CopyBufferToBuffer flat five-arg shape
	// (gpu/webgpu/encoder.go:106).
	CopyBufferToBuffer(src Buffer, srcOffset uint64, dst Buffer, dstOffset uint64, size uint64)

	// CopyBufferToTexture copies data from a buffer to a texture.
	CopyBufferToTexture(src Buffer, dst Texture, regions []BufferTextureCopy)

	// CopyTextureToBuffer copies data from a texture to a buffer.
	CopyTextureToBuffer(src Texture, dst Buffer, regions []BufferTextureCopy)

	// CopyTextureToTexture copies data between textures.
	CopyTextureToTexture(src, dst Texture, regions []TextureCopy)

	// ResolveQuerySet copies query results from a query set into a buffer.
	// firstQuery is the index of the first query to resolve.
	// queryCount is the number of queries to resolve.
	// destination is the buffer to write results to.
	// destinationOffset is the byte offset into the destination buffer.
	// Each timestamp result is a uint64 (8 bytes).
	ResolveQuerySet(querySet QuerySet, firstQuery, queryCount uint32, destination Buffer, destinationOffset uint64)

	// BeginRenderPass begins a render pass.
	// Returns a render pass encoder for recording draw commands.
	// Matches webgpu CommandEncoder.BeginRenderPass (gpu/webgpu/encoder.go:23).
	BeginRenderPass(desc *RenderPassDescriptor) (RenderPassEncoder, error)

	// BeginComputePass begins a compute pass.
	// Returns a compute pass encoder for recording dispatch commands.
	// Matches webgpu CommandEncoder.BeginComputePass (gpu/webgpu/encoder.go:86).
	BeginComputePass(desc *ComputePassDescriptor) (ComputePassEncoder, error)

	// BuildAccelerationStructures builds one or more acceleration structures.
	// Batched to match Vulkan vkCmdBuildAccelerationStructuresKHR.
	// No-op on backends without RT support.
	BuildAccelerationStructures(descriptors []BuildAccelerationStructureDescriptor)

	// PlaceAccelerationStructureBarrier inserts an AS memory barrier.
	PlaceAccelerationStructureBarrier(barrier AccelerationStructureBarrier)

	// CopyAccelerationStructure copies or compacts an AS.
	CopyAccelerationStructure(src, dst AccelerationStructure, copyMode gputypes.AccelerationStructureCopyMode)

	// ReadAccelerationStructureCompactSize reads the post-compact size of an AS
	// into the given buffer. Used for the compaction state machine.
	ReadAccelerationStructureCompactSize(as AccelerationStructure, buffer Buffer, offset uint64)
}

// RenderPassEncoder records render commands within a render pass.
// Render passes define rendering targets and operations.
type RenderPassEncoder interface {
	// End finishes the render pass.
	// After this call, the encoder cannot be used again.
	// Matches webgpu RenderPassEncoder.End (gpu/webgpu/renderpass.go:117).
	End() error

	// SetPipeline sets the active render pipeline.
	SetPipeline(pipeline RenderPipeline)

	// SetBindGroup sets a bind group for the given index.
	// offsets are dynamic offsets for dynamic uniform/storage buffers.
	SetBindGroup(index uint32, group BindGroup, offsets []uint32)

	// SetVertexBuffer sets a vertex buffer for the given slot.
	SetVertexBuffer(slot uint32, buffer Buffer, offset uint64)

	// SetIndexBuffer sets the index buffer.
	SetIndexBuffer(buffer Buffer, format gputypes.IndexFormat, offset uint64)

	// SetViewport sets the viewport transformation.
	// Matches webgpu RenderPassEncoder.SetViewport flat six-arg shape
	// (gpu/webgpu/renderpass.go:60).
	SetViewport(x, y, width, height, minDepth, maxDepth float32)

	// SetScissorRect sets the scissor rectangle for clipping.
	// Matches webgpu RenderPassEncoder.SetScissorRect flat four-arg shape
	// (gpu/webgpu/renderpass.go:65).
	SetScissorRect(x, y, width, height uint32)

	// SetBlendConstant sets the blend constant color.
	SetBlendConstant(color *gputypes.Color)

	// SetStencilReference sets the stencil reference value.
	SetStencilReference(reference uint32)

	// Draw draws primitives.
	// Matches webgpu RenderPassEncoder.Draw flat four-arg shape
	// (gpu/webgpu/renderpass.go:87).
	Draw(vertexCount, instanceCount, firstVertex, firstInstance uint32)

	// DrawIndexed draws indexed primitives.
	// Matches webgpu RenderPassEncoder.DrawIndexed flat five-arg shape
	// (gpu/webgpu/renderpass.go:92).
	DrawIndexed(indexCount, instanceCount, firstIndex uint32, baseVertex int32, firstInstance uint32)

	// DrawIndirect draws primitives with GPU-generated parameters.
	// Matches webgpu RenderPassEncoder.DrawIndirect two-arg shape
	// (gpu/webgpu/renderpass.go:98).
	DrawIndirect(buffer Buffer, offset uint64)

	// DrawIndexedIndirect draws indexed primitives with GPU-generated parameters.
	// Matches webgpu RenderPassEncoder.DrawIndexedIndirect two-arg shape
	// (gpu/webgpu/renderpass.go:106).
	DrawIndexedIndirect(buffer Buffer, offset uint64)

	// DrawIndirectCount draws primitives using a GPU count buffer (Vulkan 1.2+).
	// countBuffer holds a single uint32 draw count at countOffset.
	DrawIndirectCount(buffer Buffer, offset uint64, countBuffer Buffer, countOffset uint64, maxDrawCount uint32)

	// DrawIndexedIndirectCount draws indexed primitives using a GPU count buffer.
	DrawIndexedIndirectCount(buffer Buffer, offset uint64, countBuffer Buffer, countOffset uint64, maxDrawCount uint32)

	// ExecuteBundle executes a pre-recorded render bundle.
	// Bundles are an optimization for repeated draw calls.
	ExecuteBundle(bundle RenderBundle)
}

// ComputePassEncoder records compute commands within a compute pass.
type ComputePassEncoder interface {
	// End finishes the compute pass.
	// After this call, the encoder cannot be used again.
	// Matches webgpu ComputePassEncoder.End (gpu/webgpu/computepass.go:46).
	End() error

	// SetPipeline sets the active compute pipeline.
	SetPipeline(pipeline ComputePipeline)

	// SetBindGroup sets a bind group for the given index.
	// offsets are dynamic offsets for dynamic uniform/storage buffers.
	SetBindGroup(index uint32, group BindGroup, offsets []uint32)

	// Dispatch dispatches compute work.
	// x, y, z are the number of workgroups to dispatch in each dimension.
	Dispatch(x, y, z uint32)

	// DispatchIndirect dispatches compute work with GPU-generated parameters.
	// buffer contains DispatchIndirectArgs at the given offset.
	DispatchIndirect(buffer Buffer, offset uint64)
}

// RenderBundle is a pre-recorded set of render commands.
// Bundles can be executed multiple times for better performance.
type RenderBundle interface {
	Resource
}

// RenderBundleEncoder records commands into a render bundle.
// The recorded commands can be replayed multiple times via ExecuteBundle.
type RenderBundleEncoder interface {
	// SetPipeline sets the active render pipeline.
	SetPipeline(pipeline RenderPipeline)

	// SetBindGroup sets a bind group for the given index.
	SetBindGroup(index uint32, group BindGroup, offsets []uint32)

	// SetVertexBuffer sets a vertex buffer for the given slot.
	SetVertexBuffer(slot uint32, buffer Buffer, offset uint64)

	// SetIndexBuffer sets the index buffer.
	SetIndexBuffer(buffer Buffer, format gputypes.IndexFormat, offset uint64)

	// Draw draws primitives.
	Draw(args gputypes.DrawArgs)

	// DrawIndexed draws indexed primitives.
	DrawIndexed(args gputypes.DrawIndexedArgs)

	// Finish finalizes the bundle and returns it.
	// The encoder cannot be used after this call.
	Finish() RenderBundle
}

// BufferBarrier defines a buffer state transition.
type BufferBarrier struct {
	Buffer Buffer
	Usage  BufferUsageTransition
}

// TextureBarrier defines a texture state transition.
type TextureBarrier struct {
	Texture Texture
	Range   TextureRange
	Usage   TextureUsageTransition
}

// BufferUsageTransition defines a buffer usage state transition.
type BufferUsageTransition struct {
	OldUsage gputypes.BufferUsage
	NewUsage gputypes.BufferUsage
}

// TextureUsageTransition defines a texture usage state transition.
type TextureUsageTransition struct {
	OldUsage gputypes.TextureUsage
	NewUsage gputypes.TextureUsage
}

// TextureRange specifies a range of texture subresources.
type TextureRange struct {
	// Aspect specifies which aspect of the texture (color, depth, stencil).
	Aspect gputypes.TextureAspect

	// BaseMipLevel is the first mip level in the range.
	BaseMipLevel uint32

	// MipLevelCount is the number of mip levels (0 means all remaining levels).
	MipLevelCount uint32

	// BaseArrayLayer is the first array layer in the range.
	BaseArrayLayer uint32

	// ArrayLayerCount is the number of array layers (0 means all remaining layers).
	ArrayLayerCount uint32
}

// BufferTextureCopy defines a buffer-texture copy region.
type BufferTextureCopy struct {
	BufferLayout ImageDataLayout
	TextureBase  ImageCopyTexture
	Size         Extent3D
}

// TextureCopy defines a texture-to-texture copy region.
// Matches webgpu TextureCopy field names (gpu/webgpu/descriptor_browser.go:270).
type TextureCopy struct {
	Source      ImageCopyTexture
	Destination ImageCopyTexture
	Size        Extent3D
}

// ImageDataLayout describes the layout of image data in a buffer.
type ImageDataLayout struct {
	// Offset is the offset in bytes from the start of the buffer.
	Offset uint64

	// BytesPerRow is the stride in bytes between rows of the image.
	// Must be a multiple of 256 for texture copies.
	// Can be 0 for single-row images.
	BytesPerRow uint32

	// RowsPerImage is the number of rows per image slice.
	// Only needed for 3D textures.
	// Can be 0 to use the image height.
	RowsPerImage uint32
}

// ImageCopyTexture specifies a texture location for copying.
type ImageCopyTexture struct {
	// Texture is the texture to copy to/from.
	Texture Texture

	// MipLevel is the mip level to copy.
	MipLevel uint32

	// Origin is the starting point of the copy.
	Origin Origin3D

	// Aspect specifies which aspect to copy (color, depth, stencil).
	Aspect gputypes.TextureAspect
}

// Origin3D is a 3D origin point.
type Origin3D struct {
	X uint32
	Y uint32
	Z uint32
}

// Extent3D is a 3D extent.
type Extent3D struct {
	Width              uint32
	Height             uint32
	DepthOrArrayLayers uint32
}
