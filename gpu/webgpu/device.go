//go:build !(js && wasm)

package webgpu

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

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
func (d *Device) CreateBuffer(desc *BufferDescriptor) (*Buffer, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateTexture(desc *TextureDescriptor) (*Texture, error) {
	if d == nil {
		return nil, fmt.Errorf("wgpu: device is nil")
	}
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateTextureView(texture *Texture, desc *TextureViewDescriptor) (*TextureView, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateSampler(desc *SamplerDescriptor) (*Sampler, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateShaderModule(desc *ShaderModuleDescriptor) (*ShaderModule, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateBindGroupLayout(desc *BindGroupLayoutDescriptor) (*BindGroupLayout, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreatePipelineLayout(desc *PipelineLayoutDescriptor) (*PipelineLayout, error) {
	if d.released {
		return nil, ErrReleased
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
		if l != nil {
			rLayouts[i] = l.r
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
func (d *Device) CreateBindGroup(desc *BindGroupDescriptor) (*BindGroup, error) {
	if d.released {
		return nil, ErrReleased
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: bind group descriptor is nil")
	}
	var rLayout *rwgpu.BindGroupLayout
	if desc.Layout != nil {
		rLayout = desc.Layout.r
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
func (d *Device) CreateRenderPipeline(desc *RenderPipelineDescriptor) (*RenderPipeline, error) {
	if d.released {
		return nil, ErrReleased
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
		return nil, fmt.Erro
	if err != nil {
		return nil, fmt.Errorf("wgpu: failed to create render pipeline: %w", err)
	}

	return &RenderPipeline{r: rp, device: d}, nil
}

// CreateComputePipeline creates a compute pipeline.
func (d *Device) CreateComputePipeline(desc *ComputePipelineDescriptor) (*ComputePipeline, error) {
	if d.released {
		return nil, ErrReleased
	}
	if desc == nil {
		return nil, fmt.Errorf("wgpu: compute pipeline descriptor is nil")
	}
	var rLayout *rwgpu.PipelineLayout
	if desc.Layout != nil {
		rLayout = desc.Layout.r
	}
	var rModule *rwgpu.ShaderModule
	if desc.Module != nil {
		rModule = desc.Module.r
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
func (d *Device) CreateCommandEncoder(desc *CommandEncoderDescriptor) (*CommandEncoder, error) {
	if d.released {
		return nil, ErrReleased
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
func (d *Device) CreateFence() (*Fence, error) {
	if d.released {
		return nil, ErrReleased
	}
	return &Fence{}, nil
}

// DestroyFence destroys a fence.
// On the wgpu-native backend, fences are no-ops — this is a no-op.
//
// Deprecated: Use Fence.Release() instead.
func (d *Device) DestroyFence(f *Fence) {
	if f != nil {
		f.Release()
	}
}

// ResetFence resets a fence to the unsignaled state.
// On the wgpu-native backend, fences are no-ops — this always succeeds.
func (d *Device) ResetFence(f *Fence) error {
	if d.released {
		return ErrReleased
	}
	if f == nil || f.released {
		return ErrReleased
	}
	return nil
}

// GetFenceStatus returns true if the fence is signaled (non-blocking).
// On the wgpu-native backend, fences are no-ops — always reports signaled.
func (d *Device) GetFenceStatus(f *Fence) (bool, error) {
	if d.released {
		return false, ErrReleased
	}
	if f == nil || f.released {
		return false, ErrReleased
	}
	return true, nil
}

// WaitForFence waits for a fence to reach the specified value.
// On the wgpu-native backend, fences are no-ops — polls the device and returns immediately.
func (d *Device) WaitForFence(f *Fence, _ uint64, _ time.Duration) (bool, error) {
	if d.released {
		return false, ErrReleased
	}
	if f == nil || f.released {
		return false, ErrReleased
	}
	// Poll device to ensure GPU work has progressed.
	d.r.Poll(true)
	return true, nil
}

// PushErrorScope pushes a new error scope onto the device's error scope stack.
func (d *Device) PushErrorScope(filter ErrorFilter) {
	if d.r != nil {
		d.r.PushErrorScope(rwgpu.ErrorFilter(filter)) //nolint:gosec // G115: ErrorFilter values are small enum constants that fit uint32
	}
}

// PopErrorScope pops the most recently pushed error scope.
// Returns the captured error, or nil if no error occurred.
func (d *Device) PopErrorScope() *GPUError {
	if d.r == nil || d.instance == nil || d.instance.r == nil {
		return nil
	}
	errType, message, err := d.r.PopErrorScopeAsync(d.instance.r)
	if err != nil {
		return nil //nolint:nilerr // PopErrorScope returns *GPUError not error; infrastructure failure = no captured error
	}
	if errType == rwgpu.ErrorTypeNoError {
		return nil
	}
	return &GPUError{
		Type:    convertErrorType(errType),
		Message: message,
	}
}

// WaitIdle waits for all GPU work to complete.
func (d *Device) WaitIdle() error {
	if d.released {
		return ErrReleased
	}
	// Poll with wait=true blocks until all work completes.
	d.r.Poll(true)
	return nil
}

// Poll drives the per-device pending-map triage loop.
func (d *Device) Poll(pollType PollType) bool {
	if d == nil || d.r == nil {
		return false
	}
	return d.r.Poll(pollType == PollWait)
}

// FreeCommandBuffer releases a command buffer after GPU work that used it has
// completed (or after WaitIdle). wgpu-native command buffers hold device
// resources; leaving them unreleased prevents Device.Release from reclaiming
// VRAM and causes subsequent CreateTexture failures under ResetAccelerator.
func (d *Device) FreeCommandBuffer(cb *CommandBuffer) {
	if cb == nil {
		return
	}
	cb.Release()
}

// HalDevice returns nil on wgpu-native backend. There is no HAL layer.
func (d *Device) HalDevice() any { return nil }

// Release releases the device and all associated resources.
func (d *Device) Release() {
	if d.released {
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

// convertBindGroupEntry converts a BindGroupEntry to rwgpu.BindGroupEntry.
func convertBindGroupEntry(e BindGroupEntry) rwgpu.BindGroupEntry {
	re := rwgpu.BindGroupEntry{
		Binding: e.Binding,
		Offset:  e.Offset,
		Size:    e.Size,
	}
	if e.Buffer != nil {
		re.Buffer = e.Buffer.r
	}
	if e.Sampler != nil {
		re.Sampler = e.Sampler.r
	}
	if e.TextureView != nil {
		re.TextureView = e.TextureView.r
	}
	return re
derPipeline descriptor conversion.
// Covers the common render shapes (≤4 vertex buffers, ≤16 attrs each, ≤4 color targets).
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
	// Drop pointers into heap keepAlive / modules so pooled slots don't pin GPU objects.
	sc.rDesc = rwgpu.RenderPipelineDescriptor{}
	sc.fragment = rwgpu.FragmentState{}
	sc.depth = rwgpu.DepthStencilState{}
	for i := range sc.attrKeep {
		sc.attrKeep[i] = nil
	}
	rplConvertPool.Put(sc)
}

// convertRenderPipelineDesc converts ou
 
}
	sc := acquireRPLConvertScratch()
	// NOTE: caller must KeepAlive the returned keepAlive; scratch is NOT returned
	// here for API compatibility — heap-copy keepAlive attrs only.
	rDesc, keep := convertRenderPipelineDescInto(sc, desc)
	// Detach attr slices from scratch storage into owned heap so sc can be released.
	owned := make([][]rwgpu.VertexAttribute, len(keep))
	for i, a := range keep {
		if len(a) == 0 {
			continue
		}
		cp := make([]rwgpu.VertexAttribute, len(a))
		copy(cp, a)
		owned[i] = cp
		if len(cp) > 0 {
			rDesc.Vertex.Buffers[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&cp[0])) //nolint:gosec
			rDesc.Vertex.Buffers[i].AttributeCount = uintptr(len(cp))
		}
	}
	// Deep-copy buffers slice if it aliases scratch.
	if len(rDesc.Vertex.Buffers) > 0 {
		bufs := make([]rwgpu.VertexBufferLayout, len(rDesc.Vertex.Buffers))
		copy(bufs, rDesc.Vertex.Buffers)
		rDesc.Vertex.Buffers = bufs
	}
	if rDesc.Fragment != nil {
		frag := *rDesc.Fragment
		if len(frag.Targets) > 0 {
			ts := make([]rwgpu.ColorTargetState, len(frag.Targets))
			copy(ts, frag.Targets)
			// re-point blends if they alias scratch blends
			for i := range ts {
				if ts[i].Blend != nil {
					b := *ts[i].Blend
					ts[i].Blend = &b
				}
			}
			frag.Targets = ts
		}
		rDesc.Fragment = &frag
	}
	if rDesc.DepthStencil != nil {
		ds := *rDesc.DepthStencil
		rDesc.DepthStencil = &ds
	}
	// rDesc itself may alias sc.rDesc
	out := *rDesc
	releaseRPLConvertScratch(sc)
	return &out, owned
}

// convertRenderPipelineDescInto fills sc and returns pointers into it.
// Caller must runtime.KeepAlive(sc) until after the native CreateRenderPipeline returns.
func convertRenderPipelineDescInto(sc *rplConvertScratch, desc *RenderPipelineDescriptor) (*rwgpu.RenderPipelineDescriptor, [][]rwgpu.VertexAttribute) {
	sc.rDesc = rwgpu.RenderPipelineDescriptor{
// convertRenderPipelineDesc converts our RenderPipelineDescriptor to rwgpu.
func convertRenderPipelineDesc(desc *RenderPipelineDescriptor) (*rwgpu.RenderPipelineDescriptor, [][]rwgpu.VertexAttribute) {
	}
		sc.rDesc.Layout = desc.Layout.r

	if desc.Layout != nil {
	sc.rDesc.Vertex = rwgpu.VertexState{

	// Vertex state.
	rDesc.Vertex = rwgpu.VertexState{
		sc.rDesc.Vertex.Module = desc.Vertex.Module.r
	}
	bufs, keepAlive := convertVertexBufferLayoutsInto(sc, desc.Vertex.Buffers)
	sc.rDesc.Vertex.Buffers = bufs
		rDesc.Vertex.Module = desc.Vertex.Module.r
	sc.rDesc.Primitive = rwgpu.PrimitiveState{

	// Primitive state (topology/front/cull converted to native wire inside rwgpu).
	rDesc.Primitive = rwgpu.PrimitiveState{
		Topology:       desc.Primitive.Topology,
		FrontFace:      desc.Primitive.FrontFace,
		CullMode:       desc.Primitive.CullMode,
		sc.rDesc.Primitive.StripIndexFormat = *desc.Primitive.StripIndexFormat
	}
	if desc.Primitive.StripIndexFormat != nil {
	}
		convertDepthStencilStateInto(&sc.depth, desc.DepthStencil)
		sc.rDesc.DepthStencil = &sc.depth
	// Depth-stencil state.
	if desc.DepthStencil != nil {
	sc.rDesc.Multisample = rwgpu.MultisampleState{

		Mask:                   uint32(desc.Multisample.Mask), //nolint:gosec
	rDesc.Multisample = rwgpu.MultisampleState{
		Count:                  desc.Multisample.Count,
		Mask:                   uint32(desc.Multisample.Mask), //nolint:gosec // mask truncation is intentional (WebGPU spec: 32-bit)
	}
		convertFragmentStateInto(sc, desc.Fragment)
		sc.rDesc.Fragment = &sc.fragment
	// Fragment state.
	if desc.Fragment != nil {
	return &sc.rDesc, keepAlive
	}

// convertVertexBufferLayouts converts vertex buffer layouts (heap fallback).

	sc := acquireRPLConvertScratch()
	bufs, keep := convertVertexBufferLayoutsInto(sc, layouts)
	// Own the result so scratch can be released.
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

func convertVertexBufferLayouts(layouts []VertexBufferLayout) ([]rwgpu.VertexBufferLayout, [][]rwgpu.VertexAttribute) {
		var attrs []rwgpu.VertexAttribute
		na := len(l.Attributes)
		if useStack {
			attrs = sc.attrStore[i][:na]
		} else {
			attrs = make([]rwgpu.VertexAttribute, na)
		}
	keepAlive := make([][]rwgpu.VertexAttribute, len(layouts))
	for i, l := range layouts {
		attrs := make([]rwgpu.VertexAttribute, len(l.Attributes))
		for j, a := range l.Attributes {
			attrs[j] = rwgpu.VertexAttribute{
				Format:         a.Format,
				Offset:         a.Offset,
				ShaderLocation: a.ShaderLocation,
			}
		}
		keepAlive[i] = attrs
			AttributeCount: uintptr(na),
			ArrayStride:    l.ArrayStride,
		if na > 0 {
			result[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&attrs[0])) //nolint:gosec
		}
		if len(attrs) > 0 {
			result[i].Attributes = (*rwgpu.VertexAttribute)(unsafe.Pointer(&attrs[0])) //nolint:gosec // G103: FFI interop requires unsafe pointer to C-style attribute array
		}
	}
// convertDepthStencilState converts depth-stencil state (heap).
}
	out := &rwgpu.DepthStencilState{}
	convertDepthStencilStateInto(out, ds)
	return out
}

func convertDepthStencilStateInto(out *rwgpu.DepthStencilState, ds *DepthStencilState) {
	*out = rwgpu.DepthStencilState{
// convertDepthStencilState converts depth-stencil state.
func convertDepthStencilState(ds *DepthStencilState) *rwgpu.DepthStencilState {
	return &rwgpu.DepthStencilState{
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
// convertFragmentState converts fragment state (heap fallback).
}
	sc := acquireRPLConvertScratch()
	convertFragmentStateInto(sc, fs)
	// Own copy
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

func convertFragmentStateInto(sc *rplConvertScratch, fs *FragmentState) {
	sc.fragment = rwgpu.FragmentState{
// convertFragmentState converts fragment state.
func convertFragmentState(fs *FragmentState) *rwgpu.FragmentState {
	result := &rwgpu.FragmentState{
		sc.fragment.Module = fs.Module.r
	}
	n := len(fs.Targets)
	if n == 0 {
		sc.fragment.Targets = nil
		return
	}
	var targets []rwgpu.ColorTargetState
	useStack := n <= len(sc.targets)
	if useStack {
		targets = sc.targets[:n]
	} else {
		targets = make([]rwgpu.ColorTargetState, n)
	}
	}

	result.Targets = make([]rwgpu.ColorTargetState, len(fs.Targets))
	for i, t := range fs.Targets {
		ct := rwgpu.ColorTargetState{
			Format:    t.Format,
			if useStack {
				sc.blends[i] = rwgpu.BlendState{
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
				ct.Blend = &sc.blends[i]
			} else {
				ct.Blend = &rwgpu.BlendState{
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
			}
		}
		targets[i] = ct
	}
	sc.fragment.Targets = targets
		result.Targets[i] = ct
	}
	return result
}

// convertErrorType maps rwgpu ErrorType to our ErrorFilter.
func convertErrorType(et rwgpu.ErrorType) ErrorFilter {
	switch et {
	case rwgpu.ErrorTypeValidation:
		return ErrorFilterValidation
	case rwgpu.ErrorTypeOutOfMemory:
		return ErrorFilterOutOfMemory
	case rwgpu.ErrorTypeInternal:
		return ErrorFilterInternal
	default:
		return ErrorFilterInternal
	}
}
