//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package webgpu

import (
	"sync"
	"time"
	"unsafe"

	"github.com/energye/gpui/gpu/hal"
	rwgpu "github.com/energye/gpui/gpu/rwgpu"
)

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
// Takes hal.PollType (canonical, 片7a以 hal 为准).
// Poll drives the per-device pending-map triage loop and pumps instance
// callbacks when available (device-lost / map async). Prefer calling Poll
// (or Instance.ProcessEvents) once per frame before Swapchain.BeginFrame.
// Takes hal.PollType (canonical, 片7a以 hal 为准).
func (d *Device) Poll(pollType hal.PollType) bool {
	if d == nil || d.r == nil {
		return false
	}
	if d.instance != nil {
		d.instance.ProcessEvents()
	}
	return d.r.Poll(pollType == hal.PollWait)
}

// FlushCallbacks pumps pending wgpu callbacks and folds Uncaptured/DeviceLost
// into sticky IsLost. Safe on nil / released / lost devices.
// FlushCallbacks pumps pending wgpu callbacks and folds Uncaptured/DeviceLost
// into sticky IsLost. Safe on nil / released / lost devices.
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
// TlasInstanceToBytes implements hal.Device: no ray tracing, returns nil.
func (d *Device) TlasInstanceToBytes(_ hal.TlasInstance) []byte { return nil }

// --- Descriptor conversion helpers ---

// convertBindGroupLayoutEntry converts a gputypes.BindGroupLayoutEntry to rwgpu.
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

// pooled scratch for CreateRenderPipeline descriptor conversion.
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

var _ hal.Device = (*Device)(nil)
