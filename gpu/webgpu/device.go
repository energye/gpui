//go:build !(js && wasm)

package webgpu

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

// Device represents a logical GPU device.
// On the wgpu-native backend, this wraps rwgpu Device.
type Device struct {
	r        *rwgpu.Device
	instance *Instance // stored for PopErrorScope which needs Instance handle
	queue    *Queue
	features Features
	limits   Limits
	released bool
}

// Queue returns the device's command queue.
func (d *Device) Queue() *Queue {
	return d.queue
}

// Features returns the device's enabled features.
func (d *Device) Features() Features {
	return d.features
}

// Limits returns the device's resource limits.
func (d *Device) Limits() Limits {
	return d.limits
}

// CreateBuffer creates a GPU buffer.
func (d *Device) CreateBuffer(desc *hal.BufferDescriptor) (*Buffer, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: buffer descriptor is nil")
	}
	rb, err := d.r.CreateBuffer(&rwgpu.BufferDescriptor{
		Label:            desc.Label,
		Size:             desc.Size,
		Usage:            desc.Usage,
		MappedAtCreation: desc.MappedAtCreation,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create buffer: %w", err)
	}

	return &Buffer{r: rb, device: d}, nil
}

// CreateTexture creates a GPU texture.
func (d *Device) CreateTexture(desc *hal.TextureDescriptor) (*Texture, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: texture descriptor is nil")
	}
	rt, err := d.r.CreateTexture(&rwgpu.TextureDescriptor{
		Label:         desc.Label,
		Usage:         desc.Usage,
		Dimension:     desc.Dimension,
		Size:          rwgpu.Extent3D{Width: desc.Size.Width, Height: desc.Size.Height, DepthOrArrayLayers: desc.Size.DepthOrArrayLayers},
		Format:        desc.Format,
		MipLevelCount: desc.MipLevelCount,
		SampleCount:   desc.SampleCount,
		ViewFormats:   desc.ViewFormats,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create texture: %w", err)
	}

	return &Texture{r: rt, device: d, format: desc.Format}, nil
}

// CreateTextureView creates a view into a texture.
// In rwgpu, CreateView is a method on Texture, not Device.
func (d *Device) CreateTextureView(texture *Texture, desc *hal.TextureViewDescriptor) (*TextureView, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if texture == nil || texture.r == nil {
		return nil, fmt.Errorf("wgpu: texture is nil")
	}

	var rDesc *rwgpu.TextureViewDescriptor
	if desc != nil {
		// webgpu.h: omitted counts use UINT32_MAX (WGPU_*_COUNT_UNDEFINED).
		const countUndefined = ^uint32(0)
		mipLevelCount := desc.MipLevelCount
		if mipLevelCount == 0 {
			mipLevelCount = countUndefined
		}
		arrayLayerCount := desc.ArrayLayerCount
		if arrayLayerCount == 0 {
			arrayLayerCount = countUndefined
		}
		rDesc = &rwgpu.TextureViewDescriptor{
			Label:           desc.Label,
			Format:          desc.Format,
			Dimension:       desc.Dimension,
			Aspect:          rwgpu.TextureAspect(desc.Aspect),
			BaseMipLevel:    desc.BaseMipLevel,
			MipLevelCount:   mipLevelCount,
			BaseArrayLayer:  desc.BaseArrayLayer,
			ArrayLayerCount: arrayLayerCount,
		}
	}

	rv, err := texture.r.CreateView(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create texture view: %w", err)
	}

	return &TextureView{r: rv, device: d, texture: texture}, nil
}

// CreateSampler creates a texture sampler.
func (d *Device) CreateSampler(desc *hal.SamplerDescriptor) (*Sampler, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	var rDesc *rwgpu.SamplerDescriptor
	if desc != nil {
		rDesc = &rwgpu.SamplerDescriptor{
			Label:        desc.Label,
			AddressModeU: desc.AddressModeU,
			AddressModeV: desc.AddressModeV,
			AddressModeW: desc.AddressModeW,
			MagFilter:    desc.MagFilter,
			MinFilter:    desc.MinFilter,
			MipmapFilter: desc.MipmapFilter,
			LodMinClamp:  desc.LodMinClamp,
			LodMaxClamp:  desc.LodMaxClamp,
			Compare:      desc.Compare,
			Anisotropy:   desc.Anisotropy,
		}
	}

	rs, err := d.r.CreateSampler(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create sampler: %w", err)
	}

	return &Sampler{r: rs, device: d}, nil
}

// CreateShaderModule creates a shader module.
func (d *Device) CreateShaderModule(desc *hal.ShaderModuleDescriptor) (*ShaderModule, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: shader module descriptor is nil")
	}
	var rm *rwgpu.ShaderModule
	var err error

	switch {
	case desc.WGSL != "":
		rm, err = d.r.CreateShaderModuleWGSL(desc.WGSL)
	case len(desc.SPIRV) > 0:
		rm, err = d.r.CreateShaderModuleSPIRV(desc.Label, desc.SPIRV)
	default:
		return nil, fmt.Errorf("wgpu: shader module descriptor has no source")
	}

	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create shader module: %w", err)
	}

	return &ShaderModule{r: rm, device: d}, nil
}

// CreateBindGroupLayout creates a bind group layout.
func (d *Device) CreateBindGroupLayout(desc *hal.BindGroupLayoutDescriptor) (*BindGroupLayout, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: bind group layout descriptor is nil")
	}
	n := len(desc.Entries)
	var stack [8]rwgpu.BindGroupLayoutEntry
	var rEntries []rwgpu.BindGroupLayoutEntry
	if n <= len(stack) {
		rEntries = stack[:n]
	} else {
		rEntries = make([]rwgpu.BindGroupLayoutEntry, n)
	}
	for i, e := range desc.Entries {
		rEntries[i] = convertBindGroupLayoutEntry(e)
	}

	rl, err := d.r.CreateBindGroupLayout(&rwgpu.BindGroupLayoutDescriptor{
		Label:   desc.Label,
		Entries: rEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create bind group layout: %w", err)
	}

	return &BindGroupLayout{r: rl, device: d}, nil
}

// CreatePipelineLayout creates a pipeline layout.
func (d *Device) CreatePipelineLayout(desc *hal.PipelineLayoutDescriptor) (*PipelineLayout, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: pipeline layout descriptor is nil")
	}
	n := len(desc.BindGroupLayouts)
	var stack [8]*rwgpu.BindGroupLayout
	var rLayouts []*rwgpu.BindGroupLayout
	if n <= len(stack) {
		rLayouts = stack[:n]
	} else {
		rLayouts = make([]*rwgpu.BindGroupLayout, n)
	}
	for i, l := range desc.BindGroupLayouts {
		if l == nil {
			continue
		}
		if wl, ok := l.(*BindGroupLayout); ok && wl != nil {
			rLayouts[i] = wl.r
		}
	}

	rl, err := d.r.CreatePipelineLayout(&rwgpu.PipelineLayoutDescriptor{
		Label:            desc.Label,
		BindGroupLayouts: rLayouts,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create pipeline layout: %w", err)
	}

	return &PipelineLayout{r: rl, device: d}, nil
}

// CreateBindGroup creates a bind group.
// Takes hal.BindGroupDescriptor (片5, hal 以 webgpu 扁平形为准);
// internal unpack of hal interfaces to concrete handles.
func (d *Device) CreateBindGroup(desc *hal.BindGroupDescriptor) (*BindGroup, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: bind group descriptor is nil")
	}
	var rLayout *rwgpu.BindGroupLayout
	if desc.Layout != nil {
		if wl, ok := desc.Layout.(*BindGroupLayout); ok && wl != nil {
			rLayout = wl.r
		}
	}

	n := len(desc.Entries)
	var stack [8]rwgpu.BindGroupEntry
	var rEntries []rwgpu.BindGroupEntry
	if n <= len(stack) {
		rEntries = stack[:n]
	} else {
		rEntries = make([]rwgpu.BindGroupEntry, n)
	}
	for i, e := range desc.Entries {
		rEntries[i] = convertBindGroupEntry(e)
	}

	rg, err := d.r.CreateBindGroup(&rwgpu.BindGroupDescriptor{
		Label:   desc.Label,
		Layout:  rLayout,
		Entries: rEntries,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create bind group: %w", err)
	}

	return &BindGroup{r: rg, device: d}, nil
}

// CreateRenderPipeline creates a render pipeline.
func (d *Device) CreateRenderPipeline(desc *hal.RenderPipelineDescriptor) (*RenderPipeline, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: render pipeline descriptor is nil")
	}
	// R7.6: convert via pooled scratch (common ≤4 VB / ≤16 attrs / ≤4 targets).
	sc := acquireRPLConvertScratch()
	rDesc, keepAlive := convertRenderPipelineDescInto(sc, desc)
	rp, err := d.r.CreateRenderPipeline(rDesc)
	runtime.KeepAlive(keepAlive)
	runtime.KeepAlive(sc)
	releaseRPLConvertScratch(sc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create render pipeline: %w", err)
	}

	return &RenderPipeline{r: rp, device: d}, nil
}

// CreateComputePipeline creates a compute pipeline.
func (d *Device) CreateComputePipeline(desc *hal.ComputePipelineDescriptor) (*ComputePipeline, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: compute pipeline descriptor is nil")
	}
	var rLayout *rwgpu.PipelineLayout
	if desc.Layout != nil {
		if wl, ok := desc.Layout.(*PipelineLayout); ok && wl != nil {
			rLayout = wl.r
		}
	}
	var rModule *rwgpu.ShaderModule
	if desc.Module != nil {
		if wm, ok := desc.Module.(*ShaderModule); ok && wm != nil {
			rModule = wm.r
		}
	}

	rp, err := d.r.CreateComputePipeline(&rwgpu.ComputePipelineDescriptor{
		Label:      desc.Label,
		Layout:     rLayout,
		Module:     rModule,
		EntryPoint: desc.EntryPoint,
	})
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create compute pipeline: %w", err)
	}

	return &ComputePipeline{r: rp, device: d}, nil
}

// CreateCommandEncoder creates a command encoder for recording GPU commands.
func (d *Device) CreateCommandEncoder(desc *hal.CommandEncoderDescriptor) (*CommandEncoder, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	var rDesc *rwgpu.CommandEncoderDescriptor
	if desc != nil {
		rDesc = &rwgpu.CommandEncoderDescriptor{
			Label: desc.Label,
		}
	}

	re, err := d.r.CreateCommandEncoder(rDesc)
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create command encoder: %w", err)
	}

	return &CommandEncoder{r: re, device: d}, nil
}

// CreateFence creates a GPU synchronization fence.
// On the wgpu-native backend, fences are not exposed by wgpu-native.
// Returns a no-op fence for API compatibility.
// Implements hal.Device (returns hal.Fence interface).
func (d *Device) CreateFence() (hal.Fence, error) {
	if err := prepareDeviceCall(d); err != nil {
		return nil, err
	}
	return &Fence{}, nil
}

// DestroyFence destroys a fence.
// On the wgpu-native backend, fences are no-ops — this is a no-op.
// Implements hal.Device (takes hal.Fence interface, internal unpack).
//
// Deprecated: Use Fence.Release() instead.
func (d *Device) DestroyFence(fence hal.Fence) {
	if wf, ok := fence.(*Fence); ok && wf != nil {
		wf.Release()
	}
}

// ResetFence resets a fence to the unsignaled state.
// On the wgpu-native backend, fences are no-ops — this always succeeds.
// Implements hal.Device (takes hal.Fence interface, internal unpack).
func (d *Device) ResetFence(fence hal.Fence) error {
	if d.released {
		return ErrReleased
	}
	wf, ok := fence.(*Fence)
	if !ok || wf == nil || wf.released {
		return ErrReleased
	}
	return nil
}

// GetFenceStatus returns true if the fence is signaled (non-blocking).
// On the wgpu-native backend, fences are no-ops — always reports signaled.
// Implements hal.Device (takes hal.Fence interface, internal unpack).
func (d *Device) GetFenceStatus(fence hal.Fence) (bool, error) {
	if d.released {
		return false, ErrReleased
	}
	wf, ok := fence.(*Fence)
	if !ok || wf == nil || wf.released {
		return false, ErrReleased
	}
	return true, nil
}

// WaitForFence waits for a fence to reach the specified value.
// On the wgpu-native backend, fences are no-ops — polls the device and returns immediately.
// Implements hal.Device (takes hal.Fence interface, internal unpack).
func (d *Device) WaitForFence(fence hal.Fence, _ uint64, _ time.Duration) (bool, error) {
	if d.released {
		return false, ErrReleased
	}
	wf, ok := fence.(*Fence)
	if !ok || wf == nil || wf.released {
		return false, ErrReleased
	}
	// Poll device to ensure GPU work has progressed.
	d.r.Poll(true)
	return true, nil
}

// PushErrorScope pushes a new error scope onto the device's error scope stack.
// Implements hal.Device (takes hal.ErrorFilter).
func (d *Device) PushErrorScope(filter hal.ErrorFilter) {
	if d.r != nil {
		d.r.PushErrorScope(rwgpu.ErrorFilter(filter)) //nolint:gosec // G115: ErrorFilter values are small enum constants that fit uint32
	}
}

// PopErrorScope pops the most recently pushed error scope.
// Returns the captured error, or nil if no error occurred.
// Implements hal.Device (returns *hal.GPUError).
func (d *Device) PopErrorScope() *hal.GPUError {
	if d.r == nil || d.instance == nil || d.instance.r == nil {
		return nil
	}
	errType, message, err := d.r.PopErrorScopeAsync(d.instance.r)
	if err != nil {
		return nil //nolint:nilerr // PopErrorScope returns *hal.GPUError not error; infrastructure failure = no captured error
	}
	if errType == rwgpu.ErrorTypeNoError {
		return nil
	}
	return &hal.GPUError{
		Type:    convertErrorType(errType),
		Message: message,
	}
}

// WaitIdle waits for all GPU work to complete.
func (d *Device) WaitIdle() error {
	if err := prepareDeviceCall(d); err != nil {
		return err
	}
	// Poll with wait=true blocks until all work completes.
	d.r.Poll(true)
	return nil
}

// Poll drives the per-device pending-map triage loop and pumps instance
// callbacks when available (device-lost / map async). Prefer calling Poll
// (or Instance.ProcessEvents) once per frame before Swapchain.BeginFrame.
func (d *Device) Poll(pollType PollType) bool {
	if d == nil || d.r == nil {
		return false
	}
	if d.instance != nil {
		d.instance.ProcessEvents()
	}
	return d.r.Poll(pollType == PollWait)
}

// FlushCallbacks pumps pending wgpu callbacks and folds Uncaptured/DeviceLost
// into sticky IsLost (Skia abandon signal). Safe on nil / released / lost devices.
func (d *Device) FlushCallbacks() {
	if d == nil || d.released {
		return
	}
	if d.instance != nil {
		d.instance.ProcessEvents()
	}
	if d.r != nil {
		d.r.SyncLostState()
		if !d.r.IsLost() {
			_ = d.r.Poll(false)
		}
	}
}

// SyncLostState pumps callbacks and folds pending lost signals into sticky IsLost.
// Safe on nil / released / lost devices.
func (d *Device) SyncLostState() {
	if d == nil || d.released || d.r == nil {
		return
	}
	if d.instance != nil {
		d.instance.ProcessEvents()
	}
	d.r.SyncLostState()
}

// MarkLost sets sticky device-lost on this facade device (same as native callback).
// Safe on nil. Used by tests and recovery injection.
func (d *Device) MarkLost() {
	if d == nil || d.r == nil {
		return
	}
	d.r.MarkLost()
}

// Destroy marks the device sticky-lost and drops the queue ref (Skia/Flutter abandon).
// After Destroy, IsLost is true and create/submit return hal.ErrDeviceLost.
//
// Does NOT call wgpuDeviceDestroy: on current libwgpu_native that API leaves the
// adapter in a state where later RequestDevice/CreateTexture report
// "Not enough memory left" (reproduced in isolation). Dropping the last device
// ref via Release() reclaims VRAM correctly; AutoRecover uses Release only.
// Safe on nil / already destroyed / released.
func (d *Device) Destroy() {
	if d == nil || d.released {
		return
	}
	// Drop queue while parent is still "not lost" so native QueueRelease runs
	// (skip-on-lost would otherwise pin the device refcount / VRAM).
	if d.queue != nil {
		d.queue.Release()
		d.queue = nil
	}
	if d.r != nil {
		d.r.MarkLost()
	}
}

// IsLost reports whether this device handle was marked lost by WGPUDeviceLostCallback.
// Per-device only; recreate via RequestDevice (and swapchain auto-recover) to continue.
func (d *Device) IsLost() bool {
	if d == nil || d.r == nil {
		return false
	}
	return d.r.IsLost()
}

// FreeCommandBuffer releases a command buffer after GPU work that used it has
// completed (or after WaitIdle). wgpu-native command buffers hold device
// resources; leaving them unreleased prevents Device.Release from reclaiming
// VRAM and causes subsequent CreateTexture failures under ResetAccelerator.
func (d *Device) FreeCommandBuffer(cb hal.CommandBuffer) {
	if cb == nil {
		return
	}
	wcb, ok := cb.(*CommandBuffer)
	if !ok || wcb == nil {
		return
	}
	wcb.Release()
}

// HalDevice returns nil on wgpu-native backend. There is no HAL layer.
func (d *Device) HalDevice() any { return nil }

// DestroyBuffer implements hal.Device: unwraps hal.Buffer.
func (d *Device) DestroyBuffer(buffer hal.Buffer) {
	if wb, ok := buffer.(*Buffer); ok {
		wb.Release()
	}
}

// DestroyTexture implements hal.Device: unwraps hal.Texture.
func (d *Device) DestroyTexture(texture hal.Texture) {
	if wt, ok := texture.(*Texture); ok {
		wt.Release()
	}
}

// DestroyTextureView implements hal.Device: unwraps hal.TextureView.
func (d *Device) DestroyTextureView(view hal.TextureView) {
	if wv, ok := view.(*TextureView); ok {
		wv.Release()
	}
}

// DestroySampler implements hal.Device: unwraps hal.Sampler.
func (d *Device) DestroySampler(sampler hal.Sampler) {
	if ws, ok := sampler.(*Sampler); ok {
		ws.Release()
	}
}

// DestroyBindGroupLayout implements hal.Device: unwraps hal.BindGroupLayout.
func (d *Device) DestroyBindGroupLayout(layout hal.BindGroupLayout) {
	if wl, ok := layout.(*BindGroupLayout); ok {
		wl.Release()
	}
}

// DestroyBindGroup implements hal.Device: unwraps hal.BindGroup.
func (d *Device) DestroyBindGroup(group hal.BindGroup) {
	if wg, ok := group.(*BindGroup); ok {
		wg.Release()
	}
}

// DestroyPipelineLayout implements hal.Device: unwraps hal.PipelineLayout.
func (d *Device) DestroyPipelineLayout(layout hal.PipelineLayout) {
	if wl, ok := layout.(*PipelineLayout); ok {
		wl.Release()
	}
}

// DestroyShaderModule implements hal.Device: unwraps hal.ShaderModule.
func (d *Device) DestroyShaderModule(module hal.ShaderModule) {
	if wm, ok := module.(*ShaderModule); ok {
		wm.Release()
	}
}

// DestroyRenderPipeline implements hal.Device: unwraps hal.RenderPipeline.
func (d *Device) DestroyRenderPipeline(pipeline hal.RenderPipeline) {
	if wp, ok := pipeline.(*RenderPipeline); ok {
		wp.Release()
	}
}

// DestroyComputePipeline implements hal.Device: unwraps hal.ComputePipeline.
func (d *Device) DestroyComputePipeline(pipeline hal.ComputePipeline) {
	if wp, ok := pipeline.(*ComputePipeline); ok {
		wp.Release()
	}
}

// MapBuffer implements hal.Device: blocking map via Buffer.Map.
func (d *Device) MapBuffer(buffer hal.Buffer, offset, size uint64) (hal.BufferMapping, error) {
	wb, ok := buffer.(*Buffer)
	if !ok {
		return hal.BufferMapping{}, hal.ErrInvalidMapRange
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := wb.Map(ctx, MapModeRead, offset, size); err != nil {
		return hal.BufferMapping{}, err
	}
	mr, err := wb.MappedRange(offset, size)
	if err != nil {
		return hal.BufferMapping{}, err
	}
	data := mr.BytesMut()
	if len(data) == 0 {
		return hal.BufferMapping{}, hal.ErrInvalidMapRange
	}
	return hal.BufferMapping{Ptr: unsafe.Pointer(&data[0]), IsCoherent: true}, nil //nolint:gosec // ADR-018 opaque handle
}

// UnmapBuffer implements hal.Device: unwraps hal.Buffer.
func (d *Device) UnmapBuffer(buffer hal.Buffer) error {
	wb, ok := buffer.(*Buffer)
	if !ok {
		return hal.ErrInvalidMapRange
	}
	return wb.Unmap()
}

// Release releases the device and all associated resources.
func (d *Device) Release() {
	if d == nil || d.released {
		return
	}
	d.released = true
	// Drop queue ref first (wgpuDeviceGetQueue holds a separate refcount).
	// Without this, DeviceRelease may not free the native device and a later
	// RequestDevice + CreateTexture can OOM ("Not enough memory left").
	if d.queue != nil {
		d.queue.Release()
		d.queue = nil
	}
	if d.r != nil {
		d.r.Release()
		d.r = nil
	}
}

// --- Descriptor conversion helpers ---

// convertBindGroupLayoutEntry converts a gputypes.BindGroupLayoutEntry to rwgpu.
func convertBindGroupLayoutEntry(e BindGroupLayoutEntry) rwgpu.BindGroupLayoutEntry {
	re := rwgpu.BindGroupLayoutEntry{
		Binding:    e.Binding,
		Visibility: e.Visibility,
	}
	if e.Buffer != nil {
		re.Buffer = &rwgpu.BufferBindingLayout{
			Type:             e.Buffer.Type,
			HasDynamicOffset: e.Buffer.HasDynamicOffset,
			MinBindingSize:   e.Buffer.MinBindingSize,
		}
	}
	if e.Sampler != nil {
		re.Sampler = &rwgpu.SamplerBindingLayout{
			Type: e.Sampler.Type,
		}
	}
	if e.Texture != nil {
		re.Texture = &rwgpu.TextureBindingLayout{
			SampleType:    e.Texture.SampleType,
			ViewDimension: e.Texture.ViewDimension,
			Multisampled:  e.Texture.Multisampled,
		}
	}
	if e.StorageTexture != nil {
		re.StorageTexture = &rwgpu.StorageTextureBindingLayout{
			Access:        e.StorageTexture.Access,
			Format:        e.StorageTexture.Format,
			ViewDimension: e.StorageTexture.ViewDimension,
		}
	}
	return re
}

// convertBindGroupEntry converts a hal.BindGroupEntry to rwgpu.BindGroupEntry.
// Internal unpack: hal interfaces hold *Buffer/*Sampler/*TextureView concretes.
func convertBindGroupEntry(e hal.BindGroupEntry) rwgpu.BindGroupEntry {
	re := rwgpu.BindGroupEntry{
		Binding: e.Binding,
		Offset:  e.Offset,
		Size:    e.Size,
	}
	if e.Buffer != nil {
		if wb, ok := e.Buffer.(*Buffer); ok && wb != nil {
			re.Buffer = wb.r
		}
	}
	if e.Sampler != nil {
		if ws, ok := e.Sampler.(*Sampler); ok && ws != nil {
			re.Sampler = ws.r
		}
	}
	if e.TextureView != nil {
		if wv, ok := e.TextureView.(*TextureView); ok && wv != nil {
			re.TextureView = wv.r
		}
	}
	return re
}

// R7.6: pooled scratch for CreateRenderPipeline descriptor conversion.
// Covers common render shapes (≤4 vertex buffers, ≤16 attrs each, ≤4 color targets).
type rplConvertScratch struct {
	layouts   [4]rwgpu.VertexBufferLayout
	attrStore [4][16]rwgpu.VertexAttribute
	attrKeep  [4][]rwgpu.VertexAttribute
	targets   [4]rwgpu.ColorTargetState
	blends    [4]rwgpu.BlendState
	fragment  rwgpu.FragmentState
	depth     rwgpu.DepthStencilState
	rDesc     rwgpu.RenderPipelineDescriptor
}

var rplConvertPool = sync.Pool{New: func() any { return new(rplConvertScratch) }}

func acquireRPLConvertScratch() *rplConvertScratch {
	return rplConvertPool.Get().(*rplConvertScratch)
}

func releaseRPLConvertScratch(sc *rplConvertScratch) {
	if sc == nil {
		return
	}
	sc.rDesc = rwgpu.RenderPipelineDescriptor{}
	sc.fragment = rwgpu.FragmentState{}
	sc.depth = rwgpu.DepthStencilState{}
	for i := range sc.attrKeep {
		sc.attrKeep[i] = nil
	}
	rplConvertPool.Put(sc)
}

// convertRenderPipelineDesc converts hal RenderPipelineDescriptor to rwgpu.
// Heap-owning fallback for callers outside CreateRenderPipeline.
func convertRenderPipelineDesc(desc *hal.RenderPipelineDescriptor) (*rwgpu.RenderPipelineDescriptor, [][]rwgpu.VertexAttribute) {
	sc := acquireRPLConvertScratch()
	rDesc, keep := convertRenderPipelineDescInto(sc, desc)
	owned := make([][]rwgpu.VertexAttribute, len(keep))
	for i, a := range keep {
		if len(a) == 0 {
			continue
		}
		cp := make([]rwgpu.VertexAttribute, len(a))
		copy(cp, a)
		owned[i] = cp
	}
	outBufs := make([]rwgpu.VertexBufferLayout, len(rDesc.Vertex.Buffers))
	copy(outBufs, rDesc.Vertex.Buffers)
	for i := range outBufs {
		if len(owned[i]) > 0 {
			outBufs[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&owned[i][0])) //nolint:gosec
			outBufs[i].AttributeCount = uintptr(len(owned[i]))
		}
	}
	out := *rDesc
	out.Vertex.Buffers = outBufs
	if rDesc.Fragment != nil {
		frag := *rDesc.Fragment
		if len(frag.Targets) > 0 {
			ts := make([]rwgpu.ColorTargetState, len(frag.Targets))
			copy(ts, frag.Targets)
			for i := range ts {
				if ts[i].Blend != nil {
					b := *ts[i].Blend
					ts[i].Blend = &b
				}
			}
			frag.Targets = ts
		}
		out.Fragment = &frag
	}
	if rDesc.DepthStencil != nil {
		ds := *rDesc.DepthStencil
		out.DepthStencil = &ds
	}
	releaseRPLConvertScratch(sc)
	return &out, owned
}

// convertRenderPipelineDescInto fills sc and returns pointers into it.
// Caller must runtime.KeepAlive(sc) until after native CreateRenderPipeline returns.
func convertRenderPipelineDescInto(sc *rplConvertScratch, desc *hal.RenderPipelineDescriptor) (*rwgpu.RenderPipelineDescriptor, [][]rwgpu.VertexAttribute) {
	sc.rDesc = rwgpu.RenderPipelineDescriptor{Label: desc.Label}
	if desc.Layout != nil {
		if wl, ok := desc.Layout.(*PipelineLayout); ok && wl != nil {
			sc.rDesc.Layout = wl.r
		}
	}

	sc.rDesc.Vertex = rwgpu.VertexState{EntryPoint: desc.Vertex.EntryPoint}
	if desc.Vertex.Module != nil {
		if wm, ok := desc.Vertex.Module.(*ShaderModule); ok && wm != nil {
			sc.rDesc.Vertex.Module = wm.r
		}
	}
	bufs, keepAlive := convertVertexBufferLayoutsInto(sc, desc.Vertex.Buffers)
	sc.rDesc.Vertex.Buffers = bufs

	sc.rDesc.Primitive = rwgpu.PrimitiveState{
		Topology:       desc.Primitive.Topology,
		FrontFace:      desc.Primitive.FrontFace,
		CullMode:       desc.Primitive.CullMode,
		UnclippedDepth: desc.Primitive.UnclippedDepth,
	}
	if desc.Primitive.StripIndexFormat != nil {
		sc.rDesc.Primitive.StripIndexFormat = *desc.Primitive.StripIndexFormat
	}

	if desc.DepthStencil != nil {
		convertDepthStencilStateInto(&sc.depth, desc.DepthStencil)
		sc.rDesc.DepthStencil = &sc.depth
	}

	sc.rDesc.Multisample = rwgpu.MultisampleState{
		Count:                  desc.Multisample.Count,
		Mask:                   uint32(desc.Multisample.Mask), //nolint:gosec
		AlphaToCoverageEnabled: desc.Multisample.AlphaToCoverageEnabled,
	}

	if desc.Fragment != nil {
		convertFragmentStateInto(sc, desc.Fragment)
		sc.rDesc.Fragment = &sc.fragment
	}
	return &sc.rDesc, keepAlive
}

// convertVertexBufferLayouts converts vertex buffer layouts (heap-owning).
func convertVertexBufferLayouts(layouts []VertexBufferLayout) ([]rwgpu.VertexBufferLayout, [][]rwgpu.VertexAttribute) {
	sc := acquireRPLConvertScratch()
	bufs, keep := convertVertexBufferLayoutsInto(sc, layouts)
	outBufs := make([]rwgpu.VertexBufferLayout, len(bufs))
	copy(outBufs, bufs)
	owned := make([][]rwgpu.VertexAttribute, len(keep))
	for i, a := range keep {
		if len(a) == 0 {
			continue
		}
		cp := make([]rwgpu.VertexAttribute, len(a))
		copy(cp, a)
		owned[i] = cp
		if len(cp) > 0 {
			outBufs[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&cp[0])) //nolint:gosec
			outBufs[i].AttributeCount = uintptr(len(cp))
		}
	}
	releaseRPLConvertScratch(sc)
	return outBufs, owned
}

func convertVertexBufferLayoutsInto(sc *rplConvertScratch, layouts []VertexBufferLayout) ([]rwgpu.VertexBufferLayout, [][]rwgpu.VertexAttribute) {
	n := len(layouts)
	if n == 0 {
		return nil, nil
	}
	useStack := n <= len(sc.layouts)
	if useStack {
		for _, l := range layouts {
			if len(l.Attributes) > len(sc.attrStore[0]) {
				useStack = false
				break
			}
		}
	}
	var result []rwgpu.VertexBufferLayout
	var keepAlive [][]rwgpu.VertexAttribute
	if useStack {
		result = sc.layouts[:n]
		keepAlive = sc.attrKeep[:n]
	} else {
		result = make([]rwgpu.VertexBufferLayout, n)
		keepAlive = make([][]rwgpu.VertexAttribute, n)
	}
	for i, l := range layouts {
		na := len(l.Attributes)
		var attrs []rwgpu.VertexAttribute
		if useStack {
			attrs = sc.attrStore[i][:na]
		} else {
			attrs = make([]rwgpu.VertexAttribute, na)
		}
		for j, a := range l.Attributes {
			attrs[j] = rwgpu.VertexAttribute{
				Format:         a.Format,
				Offset:         a.Offset,
				ShaderLocation: a.ShaderLocation,
			}
		}
		keepAlive[i] = attrs
		result[i] = rwgpu.VertexBufferLayout{
			ArrayStride:    l.ArrayStride,
			StepMode:       l.StepMode,
			AttributeCount: uintptr(na),
		}
		if na > 0 {
			result[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&attrs[0])) //nolint:gosec
		}
	}
	return result, keepAlive
}

// convertDepthStencilState converts depth-stencil state (heap).
func convertDepthStencilState(ds *hal.DepthStencilState) *rwgpu.DepthStencilState {
	out := &rwgpu.DepthStencilState{}
	convertDepthStencilStateInto(out, ds)
	return out
}

func convertDepthStencilStateInto(out *rwgpu.DepthStencilState, ds *hal.DepthStencilState) {
	*out = rwgpu.DepthStencilState{
		Format:              ds.Format,
		DepthWriteEnabled:   ds.DepthWriteEnabled,
		DepthCompare:        ds.DepthCompare,
		StencilReadMask:     ds.StencilReadMask,
		StencilWriteMask:    ds.StencilWriteMask,
		DepthBias:           ds.DepthBias,
		DepthBiasSlopeScale: ds.DepthBiasSlopeScale,
		DepthBiasClamp:      ds.DepthBiasClamp,
		StencilFront: rwgpu.StencilFaceState{
			Compare:     ds.StencilFront.Compare,
			FailOp:      rwgpu.StencilOperation(ds.StencilFront.FailOp),
			DepthFailOp: rwgpu.StencilOperation(ds.StencilFront.DepthFailOp),
			PassOp:      rwgpu.StencilOperation(ds.StencilFront.PassOp),
		},
		StencilBack: rwgpu.StencilFaceState{
			Compare:     ds.StencilBack.Compare,
			FailOp:      rwgpu.StencilOperation(ds.StencilBack.FailOp),
			DepthFailOp: rwgpu.StencilOperation(ds.StencilBack.DepthFailOp),
			PassOp:      rwgpu.StencilOperation(ds.StencilBack.PassOp),
		},
	}
}

// convertFragmentState converts fragment state (heap-owning).
func convertFragmentState(fs *hal.FragmentState) *rwgpu.FragmentState {
	sc := acquireRPLConvertScratch()
	convertFragmentStateInto(sc, fs)
	out := sc.fragment
	if len(out.Targets) > 0 {
		ts := make([]rwgpu.ColorTargetState, len(out.Targets))
		copy(ts, out.Targets)
		for i := range ts {
			if ts[i].Blend != nil {
				b := *ts[i].Blend
				ts[i].Blend = &b
			}
		}
		out.Targets = ts
	}
	releaseRPLConvertScratch(sc)
	return &out
}

func convertFragmentStateInto(sc *rplConvertScratch, fs *hal.FragmentState) {
	sc.fragment = rwgpu.FragmentState{EntryPoint: fs.EntryPoint}
	if fs.Module != nil {
		if wm, ok := fs.Module.(*ShaderModule); ok && wm != nil {
			sc.fragment.Module = wm.r
		}
	}
	n := len(fs.Targets)
	if n == 0 {
		sc.fragment.Targets = nil
		return
	}
	useStack := n <= len(sc.targets)
	var targets []rwgpu.ColorTargetState
	if useStack {
		targets = sc.targets[:n]
	} else {
		targets = make([]rwgpu.ColorTargetState, n)
	}
	for i, t := range fs.Targets {
		ct := rwgpu.ColorTargetState{
			Format:    t.Format,
			WriteMask: t.WriteMask,
		}
		if t.Blend != nil {
			bl := rwgpu.BlendState{
				Color: rwgpu.BlendComponent{
					SrcFactor: t.Blend.Color.SrcFactor,
					DstFactor: t.Blend.Color.DstFactor,
					Operation: t.Blend.Color.Operation,
				},
				Alpha: rwgpu.BlendComponent{
					SrcFactor: t.Blend.Alpha.SrcFactor,
					DstFactor: t.Blend.Alpha.DstFactor,
					Operation: t.Blend.Alpha.Operation,
				},
			}
			if useStack {
				sc.blends[i] = bl
				ct.Blend = &sc.blends[i]
			} else {
				b := bl
				ct.Blend = &b
			}
		}
		targets[i] = ct
	}
	sc.fragment.Targets = targets
}

// convertErrorType maps rwgpu ErrorType to hal.ErrorFilter.
func convertErrorType(et rwgpu.ErrorType) hal.ErrorFilter {
	switch et {
	case rwgpu.ErrorTypeValidation:
		return hal.ErrorFilterValidation
	case rwgpu.ErrorTypeOutOfMemory:
		return hal.ErrorFilterOutOfMemory
	case rwgpu.ErrorTypeInternal:
		return hal.ErrorFilterInternal
	default:
		return hal.ErrorFilterInternal
	}
}
