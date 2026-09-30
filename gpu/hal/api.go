//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !(js && wasm)

package hal

import (
	"image"
	"time"
	"unsafe"

	gputypes "github.com/energye/gpui/gpu/types"
)

// BufferMapping describes a CPU-visible mapping of a GPU buffer.
//
// Returned by Device.MapBuffer. The memory remains valid until the
// corresponding Device.UnmapBuffer call. Callers must ensure the GPU
// is not writing to the mapped region during CPU access — this is the
// caller's responsibility (core coordinates this via submission fences).
type BufferMapping struct {
	// Ptr is the host-visible pointer to the start of the mapped range.
	// Never nil on success.
	Ptr unsafe.Pointer

	// IsCoherent indicates whether the underlying memory is coherent
	// with GPU writes without explicit flush/invalidate.
	//   - true → DX12 (always), Metal Shared storage, coherent Vulkan memory
	//   - false → non-coherent Vulkan memory; caller must invalidate before
	//             reading GPU-written data and flush after writing
	//
	// When false, core is responsible for calling FlushMappedRanges /
	// InvalidateMappedRanges at the appropriate times.
	IsCoherent bool
}

// Backend identifies a graphics backend implementation.
// Backends are registered globally and provide factory methods for instances.
type Backend interface {
	// Variant returns the backend type identifier.
	Variant() gputypes.Backend

	// CreateInstance creates a new GPU instance with the given configuration.
	// Returns an error if instance creation fails (e.g., drivers not available).
	CreateInstance(desc *InstanceDescriptor) (Instance, error)
}

// Instance is the entry point for GPU operations.
// An instance manages adapter enumeration and surface creation.
type Instance interface {
	// CreateSurface creates a rendering surface from a typed raw platform target.
	// The target's native objects remain caller-owned and must outlive the Surface.
	CreateSurface(target SurfaceTarget) (Surface, error)

	// EnumerateAdapters enumerates available physical GPUs.
	// surfaceHint is optional - if provided, only adapters compatible with
	// the surface are returned.
	EnumerateAdapters(surfaceHint Surface) []ExposedAdapter

	// RequestAdapter requests a GPU adapter matching the options.
	// If opts is nil, the best available adapter is returned.
	RequestAdapter(opts *RequestAdapterOptions) (Adapter, error)

	// ProcessEvents pumps pending async callbacks (device-lost, map async, etc.).
	// Synchronous backends (noop/gles/metal) are no-ops.
	ProcessEvents()

	// Release releases the instance.
	// All adapters and surfaces created from this instance must be destroyed first.
	Release()
}

// ExposedAdapter bundles an adapter with its capabilities.
// This is returned by Instance.EnumerateAdapters.
type ExposedAdapter struct {
	// Adapter is the physical GPU.
	Adapter Adapter

	// Info contains adapter metadata.
	Info gputypes.AdapterInfo

	// Features are the supported optional features.
	Features gputypes.Features

	// Capabilities contains detailed capability information.
	Capabilities Capabilities
}

// Adapter represents a physical GPU.
// Adapters are enumerated from instances and provide capability queries.
type Adapter interface {
	// Open opens a logical device with the requested features and limits.
	// Returns an error if the adapter cannot support the requested configuration.
	Open(features gputypes.Features, limits gputypes.Limits) (OpenDevice, error)

	// TextureFormatCapabilities returns capabilities for a specific texture format.
	TextureFormatCapabilities(format gputypes.TextureFormat) TextureFormatCapabilities

	// Info returns adapter metadata.
	Info() gputypes.AdapterInfo

	// Features returns supported features.
	Features() gputypes.Features

	// Limits returns the adapter's resource limits.
	Limits() gputypes.Limits

	// RequestDevice creates a logical device from this adapter.
	// If desc is nil, default features and limits are used.
	// Queue is accessible via the returned Device.Queue().
	RequestDevice(desc *DeviceDescriptor) (Device, error)

	// GetSurfaceCapabilities returns capabilities for a specific surface.
	// Returns nil if the adapter is not compatible with the surface.
	GetSurfaceCapabilities(surface Surface) *SurfaceCapabilities

	// Release releases the adapter.
	// Any devices created from this adapter must be destroyed first.
	Release()
}

// OpenDevice is returned when Adapter.Open succeeds.
// It bundles the device and queue together since they're created atomically.
type OpenDevice struct {
	// Device is the logical GPU device.
	Device Device

	// Queue is the device's command queue.
	Queue Queue
}

// PollType selects the blocking behavior of Device.Poll.
// PollType is canonical here since 7f.
type PollType uint8

const (
	PollPoll PollType = iota
	PollWait
)

// Device represents a logical GPU device.
// Devices are used to create resources and command encoders.
type Device interface {
	// Queue returns the device's command queue.
	Queue() Queue

	// Features returns the device's enabled features.
	Features() gputypes.Features

	// Limits returns the device's resource limits.
	Limits() gputypes.Limits

	// CreateBuffer creates a GPU buffer.
	CreateBuffer(desc *BufferDescriptor) (Buffer, error)

	// DestroyBuffer destroys a GPU buffer.
	DestroyBuffer(buffer Buffer)

	// MapBuffer establishes a CPU-visible mapping for the given byte range
	// of a host-visible buffer.
	//
	// The buffer must have been created with BufferUsageMapRead or
	// BufferUsageMapWrite (or have been created with MappedAtCreation: true,
	// in which case backends may return the existing mapping directly).
	//
	// Thread-safety contract: the caller (core) must ensure the GPU is not
	// writing to the mapped range while the mapping is active. Typically
	// this means core has observed a submission fence reaching completion
	// before calling MapBuffer.
	//
	// Some backends (Vulkan, Metal Shared, DX12 UPLOAD/READBACK, software)
	// keep buffers persistently mapped; MapBuffer for those backends returns
	// the cached pointer and is cheap. Backends without persistent mappings
	// call the native map API on each call.
	//
	// Returns ErrInvalidMapRange if offset+size exceeds the buffer size or
	// the buffer has no host-visible memory.
	MapBuffer(buffer Buffer, offset, size uint64) (BufferMapping, error)

	// UnmapBuffer releases a CPU-visible mapping established by MapBuffer.
	//
	// For backends with persistent mappings this is a no-op that simply
	// returns nil. For backends without persistent mappings the native
	// unmap API is called.
	//
	// Calling UnmapBuffer on a buffer that is not mapped is undefined —
	// core guarantees the state machine never calls this on an unmapped
	// buffer.
	UnmapBuffer(buffer Buffer) error

	// CreateTexture creates a GPU texture.
	CreateTexture(desc *TextureDescriptor) (Texture, error)

	// DestroyTexture destroys a GPU texture.
	DestroyTexture(texture Texture)

	// CreateTextureView creates a view into a texture.
	CreateTextureView(texture Texture, desc *TextureViewDescriptor) (TextureView, error)

	// DestroyTextureView destroys a texture view.
	DestroyTextureView(view TextureView)

	// CreateSampler creates a texture sampler.
	CreateSampler(desc *SamplerDescriptor) (Sampler, error)

	// DestroySampler destroys a sampler.
	DestroySampler(sampler Sampler)

	// CreateBindGroupLayout creates a bind group layout.
	CreateBindGroupLayout(desc *BindGroupLayoutDescriptor) (BindGroupLayout, error)

	// DestroyBindGroupLayout destroys a bind group layout.
	DestroyBindGroupLayout(layout BindGroupLayout)

	// CreateBindGroup creates a bind group.
	CreateBindGroup(desc *BindGroupDescriptor) (BindGroup, error)

	// DestroyBindGroup destroys a bind group.
	DestroyBindGroup(group BindGroup)

	// CreatePipelineLayout creates a pipeline layout.
	CreatePipelineLayout(desc *PipelineLayoutDescriptor) (PipelineLayout, error)

	// DestroyPipelineLayout destroys a pipeline layout.
	DestroyPipelineLayout(layout PipelineLayout)

	// CreateShaderModule creates a shader module.
	CreateShaderModule(desc *ShaderModuleDescriptor) (ShaderModule, error)

	// DestroyShaderModule destroys a shader module.
	DestroyShaderModule(module ShaderModule)

	// CreateRenderPipeline creates a render pipeline.
	CreateRenderPipeline(desc *RenderPipelineDescriptor) (RenderPipeline, error)

	// DestroyRenderPipeline destroys a render pipeline.
	DestroyRenderPipeline(pipeline RenderPipeline)

	// CreateComputePipeline creates a compute pipeline.
	CreateComputePipeline(desc *ComputePipelineDescriptor) (ComputePipeline, error)

	// DestroyComputePipeline destroys a compute pipeline.
	DestroyComputePipeline(pipeline ComputePipeline)

	// CreateQuerySet creates a query set for timestamp or occlusion queries.
	// Returns ErrTimestampsNotSupported if the backend does not support the query type.
	CreateQuerySet(desc *QuerySetDescriptor) (QuerySet, error)

	// DestroyQuerySet destroys a query set.
	DestroyQuerySet(querySet QuerySet)

	// CreateCommandEncoder creates a command encoder.
	CreateCommandEncoder(desc *CommandEncoderDescriptor) (CommandEncoder, error)

	// CreateRenderBundleEncoder creates a render bundle encoder.
	// Render bundles are pre-recorded command sequences that can be replayed
	// multiple times for better performance.
	CreateRenderBundleEncoder(desc *RenderBundleEncoderDescriptor) (RenderBundleEncoder, error)

	// DestroyRenderBundle destroys a render bundle.
	DestroyRenderBundle(bundle RenderBundle)

	// FreeCommandBuffer returns a command buffer to the command pool.
	// This must be called after the GPU has finished using the command buffer.
	// The command buffer handle becomes invalid after this call.
	FreeCommandBuffer(cmdBuffer CommandBuffer)

	// CreateFence creates a synchronization fence.
	CreateFence() (Fence, error)

	// DestroyFence destroys a fence.
	DestroyFence(fence Fence)

	// WaitForFence waits for a fence to reach the specified value.
	// Returns true if the fence reached the value, false if timeout.
	// Returns ErrDeviceLost if the device is lost.
	WaitForFence(fence Fence, value uint64, timeout time.Duration) (bool, error)

	// ResetFence resets a fence to the unsignaled state.
	// The fence must not be in use by the GPU.
	ResetFence(fence Fence) error

	// GetFenceStatus returns true if the fence is signaled (non-blocking).
	// This is used for polling completion without blocking.
	GetFenceStatus(fence Fence) (bool, error)

	// WaitIdle waits for all GPU work to complete.
	// Call this before destroying resources to ensure the GPU is not using them.
	WaitIdle() error

	// Poll drives pending work and pumps callbacks.
	Poll(pollType PollType) bool

	// IsLost reports whether the device was marked lost.
	IsLost() bool

	// FlushCallbacks pumps pending callbacks and folds lost signals.
	FlushCallbacks()

	// PushErrorScope pushes a new error scope onto the device's error scope stack.
	PushErrorScope(filter ErrorFilter)

	// PopErrorScope pops the most recently pushed error scope.
	PopErrorScope() *GPUError

	// CreateAccelerationStructure creates an acceleration structure (BLAS or TLAS).
	// Requires FeatureRayQuery. Returns ErrUnsupported if RT is not available.
	CreateAccelerationStructure(desc *AccelerationStructureDescriptor) (AccelerationStructure, error)

	// DestroyAccelerationStructure destroys an acceleration structure.
	DestroyAccelerationStructure(as AccelerationStructure)

	// GetAccelerationStructureBuildSizes returns the sizes needed for building an AS.
	GetAccelerationStructureBuildSizes(desc *GetAccelerationStructureBuildSizesDescriptor) AccelerationStructureBuildSizes

	// GetAccelerationStructureDeviceAddress returns the GPU device address of an AS.
	// Used for TLAS instance buffer population (BlasAddress field).
	GetAccelerationStructureDeviceAddress(as AccelerationStructure) uint64

	// TlasInstanceToBytes converts a TlasInstance to the backend-specific
	// packed byte representation (64 bytes for Vulkan/DX12/Metal).
	TlasInstanceToBytes(instance TlasInstance) []byte

	// Release releases the device.
	// All resources created from this device must be destroyed first.
	Release()
}

// Queue handles command submission and presentation.
// Queues are typically thread-safe (backend-specific).
type Queue interface {
	// Submit submits command buffers to the GPU for execution.
	// Returns a monotonically increasing submission index that can be used
	// with Poll to determine when the GPU has finished the work.
	// The HAL manages its own internal fences/synchronization.
	Submit(commandBuffers ...CommandBuffer) (submissionIndex uint64, err error)

	// Poll returns the highest submission index known to be completed
	// by the GPU. Non-blocking. Returns 0 if no submissions have completed.
	// Backend divergence: webgpu always returns 0
	// ; gles returns Fence.GetLatest
	// (or the submission index when queueless); metal returns the
	// GPU-callback-driven completedIndex; noop returns the submission
	// index (synchronous, submitted == completed).
	Poll() uint64

	// LastSubmissionIndex returns the most recent submission index.
	// Backend divergence: webgpu always returns 0
	// ; gles/metal/noop return their own
	// monotonically increasing counter.
	LastSubmissionIndex() uint64

	// WriteBuffer writes data to a buffer immediately.
	// This is a convenience method that creates a staging buffer internally.
	// Returns an error if the buffer is invalid or the write fails.
	WriteBuffer(buffer Buffer, offset uint64, data []byte) error

	// WriteTexture writes data to a texture immediately.
	// This is a convenience method that creates a staging buffer internally.
	// Returns an error if any step in the upload pipeline fails (VK-003).
	WriteTexture(dst *ImageCopyTexture, data []byte, layout *ImageDataLayout, size *Extent3D) error

	// Present presents a surface texture to the screen.
	// The texture must have been acquired via Surface.AcquireTexture.
	// After this call, the texture is consumed and must not be used.
	//
	// damageRects is an optional list of rectangles (physical pixels, top-left
	// origin) indicating which regions of the surface changed this frame.
	// When nil or empty, the entire surface is presented — EXACT same code
	// path as a call without damage. Backends that support damage rects
	// (DX12 FLIP_SEQUENTIAL, Vulkan VK_KHR_incremental_present, GLES
	// EGL_KHR_swap_buffers_with_damage, software partial blit) use them
	// as compositor hints. Backends without support accept and ignore them.
	Present(surface Surface, texture SurfaceTexture, damageRects []image.Rectangle) error

	// GetTimestampPeriod returns the timestamp period in nanoseconds.
	// Used to convert timestamp query results to real time.
	GetTimestampPeriod() float32

	// SupportsCommandBufferCopies reports whether this queue uses command-buffer-based
	// copy operations (true for DX12, Vulkan, Metal) or direct API calls (false for
	// GLES, Software). When false, PendingWrites passes WriteBuffer/WriteTexture
	// directly to the HAL without batching.
	SupportsCommandBufferCopies() bool

	// SetSwapchainSuppressed temporarily disables swapchain semaphore binding
	// for subsequent Submit calls. Used for offscreen renders (e.g., RepaintBoundary)
	// that must not consume acquire/present semaphores intended for the compositor
	// submit. Call with true before offscreen submits, false after.
	//
	// Without this, the first Submit per frame hijacks swapchain
	// semaphores even when rendering to an offscreen texture, causing the compositor
	// submit to run without synchronization (race condition -> flickering).
	//
	// Only meaningful on Vulkan — other backends are no-ops (DX12/Metal/GLES/Software
	// have different synchronization models that don't exhibit this issue).
	//
	// Thread-safe: acquires the queue mutex internally.
	// Zero allocations: two pointer writes + one bool write.
	//
	// Callers must restore state with SetSwapchainSuppressed(false) after offscreen
	// submits complete. Use defer for safety:
	//
	//   queue.SetSwapchainSuppressed(true)
	//   defer queue.SetSwapchainSuppressed(false)
	//   queue.Submit(offscreenCmds...)
	//
	// Precedent: save/restore pattern in webgpu Queue.WriteTexture
	SetSwapchainSuppressed(suppressed bool)
}

// MaxStagingBufferSizer is an optional interface implemented by HAL devices
// that can report the maximum safe staging buffer allocation size.
//
// For Vulkan, this returns min(64MB, maxMemoryAllocationSize) from
// VkPhysicalDeviceMaintenance3Properties. Without this limit, staging belt
// allocations can silently fail when they exceed the driver's maximum
// allocation size, leading to SIGSEGV.
//
// Backends that do not implement this interface default to 64MB
// (stagingBeltMaxOversizedSize), which is safe for DX12, Metal, and GLES.
type MaxStagingBufferSizer interface {
	// MaxStagingBufferSize returns the maximum size in bytes for a single
	// staging buffer allocation. Returns 0 to use the default (64MB).
	MaxStagingBufferSize() uint64
}
