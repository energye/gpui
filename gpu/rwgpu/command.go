package rwgpu

import (
	"runtime"
	"unsafe"

	"github.com/energye/gpui/gpu/types"
)

// CommandEncoderDescriptor describes a command encoder to create.
type CommandEncoderDescriptor struct {
	Label string
}

// commandEncoderDescriptorWire is the FFI-compatible C-layout struct for wgpu-native.
// nextInChain(8)+label(16) = 24 bytes.
type commandEncoderDescriptorWire struct {
	NextInChain uintptr    // *ChainedStruct
	Label       StringView // 16 bytes
}

// CommandBufferDescriptor describes a command buffer.
type CommandBufferDescriptor struct {
	NextInChain uintptr // *ChainedStruct
	Label       StringView
}

// ComputePassTimestampWrites is a deprecated alias for PassTimestampWrites.
// Deprecated: Use PassTimestampWrites. Renamed in wgpu-native v29.
type ComputePassTimestampWrites = PassTimestampWrites

// computePassDescriptorWire is the native FFI structure for ComputePassDescriptor.
// v29: timestampWrites field is *WGPUPassTimestampWrites (unified, not separate ComputePassTimestampWrites).
type computePassDescriptorWire struct {
	nextInChain     uintptr    // 8 bytes
	label           StringView // 16 bytes
	timestampWrites uintptr    // 8 bytes (*passTimestampWrites, nullable)
}

// ComputePassDescriptor describes a compute pass (user-facing API).
type ComputePassDescriptor struct {
	Label           string
	TimestampWrites *PassTimestampWrites // optional; use PassTimestampWrites (was ComputePassTimestampWrites)
}

// CreateCommandEncoder creates a command encoder.
// Returns an error if the FFI call fails or the device is nil.
func (d *Device) CreateCommandEncoder(desc *CommandEncoderDescriptor) (*CommandEncoder, error) {
	if err := prepareDeviceCall("CreateCommandEncoder", d); err != nil {
		return nil, err
	}
	var descPtr uintptr
	var wire commandEncoderDescriptorWire
	if desc != nil {
		wire = commandEncoderDescriptorWire{
			Label: stringToStringView(desc.Label),
		}
		descPtr = uintptr(unsafe.Pointer(&wire))
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _, _ := procDeviceCreateCommandEncoder.Call(
		d.handle,
		descPtr,
	)
	runtime.KeepAlive(wire)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateCommandEncoder", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "CommandEncoder")
	return &CommandEncoder{handle: handle, device: d.handle}, nil
}

// BeginComputePass begins a compute pass.
// Returns an error if the FFI call fails or the encoder is nil.
func (enc *CommandEncoder) BeginComputePass(desc *ComputePassDescriptor) (*ComputePassEncoder, error) {
	if enc == nil || enc.handle == 0 {
		return nil, &WGPUError{Op: "BeginComputePass", Message: "encoder is nil or released"}
	}
	if err := refuseIfLost("BeginComputePass", enc.device); err != nil {
		return nil, err
	}
	if err := checkInit(); err != nil {
		return nil, err
	}

	var wireDesc computePassDescriptorWire
	var wireTimestamp passTimestampWrites
	var descPtr uintptr

	if desc != nil {
		wireDesc.nextInChain = 0
		if desc.Label != "" {
			labelBytes := []byte(desc.Label)
			wireDesc.label = StringView{
				Data:   uintptr(unsafe.Pointer(&labelBytes[0])),
				Length: uintptr(len(labelBytes)),
			}
		} else {
			wireDesc.label = EmptyStringView()
		}
		if desc.TimestampWrites != nil {
			wireTimestamp = passTimestampWrites{
				nextInChain:               0,
				querySet:                  desc.TimestampWrites.QuerySet.handle,
				beginningOfPassWriteIndex: desc.TimestampWrites.BeginningOfPassWriteIndex,
				endOfPassWriteIndex:       desc.TimestampWrites.EndOfPassWriteIndex,
			}
			wireDesc.timestampWrites = uintptr(unsafe.Pointer(&wireTimestamp))
		}
		descPtr = uintptr(unsafe.Pointer(&wireDesc))
	}

	handle, _, _ := procCommandEncoderBeginComputePass.Call(
		enc.handle,
		descPtr,
	)
	if handle == 0 {
		return nil, &WGPUError{Op: "BeginComputePass", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "ComputePassEncoder")
	return &ComputePassEncoder{handle: handle, device: enc.device}, nil
}

// CopyBufferToBuffer copies data between buffers.
func (enc *CommandEncoder) CopyBufferToBuffer(src *Buffer, srcOffset uint64, dst *Buffer, dstOffset uint64, size uint64) {
	if enc == nil || enc.handle == 0 || src == nil || src.handle == 0 || dst == nil || dst.handle == 0 {
		return
	}
	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderCopyBufferToBuffer.Call( //nolint:errcheck
		enc.handle,
		src.handle,
		uintptr(srcOffset),
		dst.handle,
		uintptr(dstOffset),
		uintptr(size),
	)
}

// ClearBuffer clears a region of a buffer to zeros.
// size = 0 means clear from offset to end of buffer.
func (enc *CommandEncoder) ClearBuffer(buffer *Buffer, offset, size uint64) {
	if enc == nil || enc.handle == 0 || buffer == nil || buffer.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderClearBuffer.Call( //nolint:errcheck
		enc.handle,
		buffer.handle,
		uintptr(offset),
		uintptr(size),
	)
}

// InsertDebugMarker inserts a single debug marker label.
// This is useful for GPU debugging tools to identify specific command points.
func (enc *CommandEncoder) InsertDebugMarker(markerLabel string) {
	if enc == nil || enc.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	labelBytes := []byte(markerLabel)
	if len(labelBytes) == 0 {
		return
	}
	label := StringView{
		Data:   uintptr(unsafe.Pointer(&labelBytes[0])),
		Length: uintptr(len(labelBytes)),
	}
	callHandleStringView(procCommandEncoderInsertDebugMarker, enc.handle, &label)
}

// PushDebugGroup begins a labeled debug group.
// Use PopDebugGroup to end the group. Groups can be nested.
func (enc *CommandEncoder) PushDebugGroup(groupLabel string) {
	if enc == nil || enc.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	labelBytes := []byte(groupLabel)
	if len(labelBytes) == 0 {
		return
	}
	label := StringView{
		Data:   uintptr(unsafe.Pointer(&labelBytes[0])),
		Length: uintptr(len(labelBytes)),
	}
	callHandleStringView(procCommandEncoderPushDebugGroup, enc.handle, &label)
}

// PopDebugGroup ends the current debug group.
// Must match a preceding PushDebugGroup call.
func (enc *CommandEncoder) PopDebugGroup() {
	if enc == nil || enc.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderPopDebugGroup.Call(enc.handle) //nolint:errcheck
}

// CopyBufferToTexture copies data from a buffer to a texture using low-level wire types.
// Errors are reported via Device error scopes, not as return values.
func (enc *CommandEncoder) CopyBufferToTexture(source *TexelCopyBufferInfo, destination *TexelCopyTextureInfo, copySize *types.Extent3D) {
	if enc == nil || enc.handle == 0 || source == nil || destination == nil || copySize == nil {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderCopyBufferToTexture.Call( //nolint:errcheck
		enc.handle,
		uintptr(unsafe.Pointer(source)),
		uintptr(unsafe.Pointer(destination)),
		uintptr(unsafe.Pointer(copySize)),
	)
}

// CopyTextureToBuffer copies data from a texture to a buffer.
// Accepts gogpu/wgpu-compatible types: src *Texture, dst *Buffer, regions []BufferTextureCopy.
// Each region specifies the buffer layout, texture subresource origin, and copy extent.
// Errors are reported via Device error scopes, not as return values.
func (enc *CommandEncoder) CopyTextureToBuffer(src *Texture, dst *Buffer, regions []BufferTextureCopy) {
	if enc == nil || enc.handle == 0 || src == nil || dst == nil || len(regions) == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	for i := range regions {
		r := &regions[i]
		srcWire := r.TextureBase.toWire()
		dstWire := TexelCopyBufferInfo{
			Layout: TexelCopyBufferLayout{
				Offset:       r.BufferLayout.Offset,
				BytesPerRow:  r.BufferLayout.BytesPerRow,
				RowsPerImage: r.BufferLayout.RowsPerImage,
			},
			Buffer: dst.handle,
		}
		size := r.Size
		procCommandEncoderCopyTextureToBuffer.Call( //nolint:errcheck
			enc.handle,
			uintptr(unsafe.Pointer(&srcWire)),
			uintptr(unsafe.Pointer(&dstWire)),
			uintptr(unsafe.Pointer(&size)),
		)
	}
}

// CopyTextureToBufferRaw copies data from a texture to a buffer using low-level wire types.
// Prefer [CopyTextureToBuffer] for new code.
func (enc *CommandEncoder) CopyTextureToBufferRaw(source *TexelCopyTextureInfo, destination *TexelCopyBufferInfo, copySize *types.Extent3D) {
	if enc == nil || enc.handle == 0 || source == nil || destination == nil || copySize == nil {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderCopyTextureToBuffer.Call( //nolint:errcheck
		enc.handle,
		uintptr(unsafe.Pointer(source)),
		uintptr(unsafe.Pointer(destination)),
		uintptr(unsafe.Pointer(copySize)),
	)
}

// CopyTextureToTexture copies data from one texture to another.
// Accepts gogpu/wgpu-compatible types: src *Texture, dst *Texture, regions []TextureCopy.
// Each region specifies the source and destination subresource origins and copy extent.
// Errors are reported via Device error scopes, not as return values.
func (enc *CommandEncoder) CopyTextureToTexture(src, dst *Texture, regions []TextureCopy) {
	if enc == nil || enc.handle == 0 || src == nil || dst == nil || len(regions) == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	for i := range regions {
		r := &regions[i]
		srcWire := r.Source.toWire()
		dstWire := r.Destination.toWire()
		size := r.Size
		procCommandEncoderCopyTextureToTexture.Call( //nolint:errcheck
			enc.handle,
			uintptr(unsafe.Pointer(&srcWire)),
			uintptr(unsafe.Pointer(&dstWire)),
			uintptr(unsafe.Pointer(&size)),
		)
	}
}

// CopyTextureToTextureRaw copies data from one texture to another using low-level wire types.
// Prefer [CopyTextureToTexture] for new code.
func (enc *CommandEncoder) CopyTextureToTextureRaw(source *TexelCopyTextureInfo, destination *TexelCopyTextureInfo, copySize *types.Extent3D) {
	if enc == nil || enc.handle == 0 || source == nil || destination == nil || copySize == nil {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderCopyTextureToTexture.Call( //nolint:errcheck
		enc.handle,
		uintptr(unsafe.Pointer(source)),
		uintptr(unsafe.Pointer(destination)),
		uintptr(unsafe.Pointer(copySize)),
	)
}

// Finish finishes recording and returns a command buffer.
// The optional desc argument allows setting a label; pass nothing for defaults.
// This variadic signature matches the gogpu/wgpu API for compatibility.
// Returns an error if the FFI call fails or the encoder is nil.
func (enc *CommandEncoder) Finish(desc ...*CommandBufferDescriptor) (*CommandBuffer, error) {
	if enc == nil || enc.handle == 0 {
		return nil, &WGPUError{Op: "CommandEncoder.Finish", Message: "encoder is nil or released"}
	}
	if err := refuseIfLost("CommandEncoder.Finish", enc.device); err != nil {
		return nil, err
	}
	if err := checkInit(); err != nil {
		return nil, err
	}
	var descPtr uintptr
	if len(desc) > 0 && desc[0] != nil {
		descPtr = uintptr(unsafe.Pointer(desc[0]))
	}
	handle, _ := call2(procCommandEncoderFinish, enc.handle, descPtr)
	if len(desc) > 0 {
		runtime.KeepAlive(desc[0])
	}
	if handle == 0 {
		return nil, &WGPUError{Op: "CommandEncoder.Finish", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "CommandBuffer")
	return &CommandBuffer{handle: handle, device: enc.device}, nil
}

// Release releases the command encoder.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (enc *CommandEncoder) Release() {
	if enc == nil {
		return
	}
	releaseNativeHandle(&enc.handle, isOwnerDeviceLost(enc.device), func(h uintptr) {
		call1(procCommandEncoderRelease, h)
	})
}

// WriteTimestamp writes a timestamp to a query.
// Note: This is a wgpu-native extension. Prefer pass-level timestamps
// via RenderPassTimestampWrites or ComputePassTimestampWrites when possible.
func (enc *CommandEncoder) WriteTimestamp(querySet *QuerySet, queryIndex uint32) {
	if enc == nil || enc.handle == 0 || querySet == nil || querySet.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderWriteTimestamp.Call( //nolint:errcheck
		enc.handle,
		querySet.handle,
		uintptr(queryIndex),
	)
}

// ResolveQuerySet resolves query results to a buffer.
// The buffer must have BufferUsageQueryResolve usage.
func (enc *CommandEncoder) ResolveQuerySet(querySet *QuerySet, firstQuery, queryCount uint32, destination *Buffer, destinationOffset uint64) {
	if enc == nil || enc.handle == 0 || querySet == nil || querySet.handle == 0 || destination == nil || destination.handle == 0 {
		return
	}

	if isOwnerDeviceLost(enc.device) || checkInit() != nil {
		return
	}
	procCommandEncoderResolveQuerySet.Call( //nolint:errcheck
		enc.handle,
		querySet.handle,
		uintptr(firstQuery),
		uintptr(queryCount),
		destination.handle,
		uintptr(destinationOffset),
	)
}

// Handle returns the underlying handle.
func (enc *CommandEncoder) Handle() uintptr { return enc.handle }

// SetPipeline sets the compute pipeline.
func (cpe *ComputePassEncoder) SetPipeline(pipeline *ComputePipeline) {
	if cpe == nil || cpe.handle == 0 || pipeline == nil || pipeline.handle == 0 {
		return
	}

	if isOwnerDeviceLost(cpe.device) || checkInit() != nil {
		return
	}
	procComputePassEncoderSetPipeline.Call( //nolint:errcheck
		cpe.handle,
		pipeline.handle,
	)
}

// SetBindGroup sets a bind group.
func (cpe *ComputePassEncoder) SetBindGroup(groupIndex uint32, group *BindGroup, dynamicOffsets []uint32) {
	if cpe == nil || cpe.handle == 0 || group == nil || group.handle == 0 {
		return
	}

	if isOwnerDeviceLost(cpe.device) || checkInit() != nil {
		return
	}
	var offsetsPtr uintptr
	offsetCount := uintptr(0)
	if len(dynamicOffsets) > 0 {
		offsetsPtr = uintptr(unsafe.Pointer(&dynamicOffsets[0]))
		offsetCount = uintptr(len(dynamicOffsets))
	}
	procComputePassEncoderSetBindGroup.Call( //nolint:errcheck
		cpe.handle,
		uintptr(groupIndex),
		group.handle,
		offsetCount,
		offsetsPtr,
	)
}

// DispatchWorkgroups dispatches compute work.
func (cpe *ComputePassEncoder) DispatchWorkgroups(x, y, z uint32) {
	if cpe == nil || cpe.handle == 0 {
		return
	}

	if isOwnerDeviceLost(cpe.device) || checkInit() != nil {
		return
	}
	procComputePassEncoderDispatchWorkgroups.Call( //nolint:errcheck
		cpe.handle,
		uintptr(x),
		uintptr(y),
		uintptr(z),
	)
}

// DispatchWorkgroupsIndirect dispatches compute work using parameters from a GPU buffer.
// indirectBuffer must contain a DispatchIndirectArgs structure:
//   - workgroupCountX (uint32)
//   - workgroupCountY (uint32)
//   - workgroupCountZ (uint32)
func (cpe *ComputePassEncoder) DispatchWorkgroupsIndirect(indirectBuffer *Buffer, indirectOffset uint64) {
	if cpe == nil || cpe.handle == 0 || indirectBuffer == nil || indirectBuffer.handle == 0 {
		return
	}

	if isOwnerDeviceLost(cpe.device) || checkInit() != nil {
		return
	}
	procComputePassEncoderDispatchWorkgroupsIndirect.Call( //nolint:errcheck
		cpe.handle,
		indirectBuffer.handle,
		uintptr(indirectOffset),
	)
}

// End ends the compute pass.
func (cpe *ComputePassEncoder) End() {
	if cpe == nil || cpe.handle == 0 {
		return
	}

	if isOwnerDeviceLost(cpe.device) || checkInit() != nil {
		return
	}
	call1(procComputePassEncoderEnd, cpe.handle)
}

// Release releases the compute pass encoder.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (cpe *ComputePassEncoder) Release() {
	if cpe == nil {
		return
	}
	releaseNativeHandle(&cpe.handle, isOwnerDeviceLost(cpe.device), func(h uintptr) {
		procComputePassEncoderRelease.Call(h) //nolint:errcheck
	})
}

// Handle returns the underlying handle.
func (cpe *ComputePassEncoder) Handle() uintptr { return cpe.handle }

// Submit submits command buffers for execution.
// Returns the submission index (uint64) and nil on success. The submission
// index can be used with Device.Poll to track when work completes.
// Matches gogpu/wgpu Queue.Submit(commands ...*CommandBuffer) (uint64, error).
func (q *Queue) Submit(commands ...*CommandBuffer) (uint64, error) {
	if err := gateQueue("Queue.Submit", q); err != nil {
		return 0, err
	}
	if len(commands) == 0 {
		return 0, nil
	}
	if err := checkInit(); err != nil {
		return 0, err
	}
	if cap(q.submitHandles) < len(commands) {
		q.submitHandles = make([]uintptr, len(commands))
	} else {
		q.submitHandles = q.submitHandles[:len(commands)]
	}
	handles := q.submitHandles
	for i, cmd := range commands {
		if cmd != nil {
			handles[i] = cmd.handle
		} else {
			handles[i] = 0
		}
	}
	// wgpuQueueSubmitForIndex is a wgpu-native extension that returns WGPUSubmissionIndex (uint64).
	// This enables callers to poll for GPU completion of a specific submission.
	gpuMu.Lock()
	defer gpuMu.Unlock()
	_, _ = LastUncapturedError() // attribute post-call errors to this submit
	submissionIndex, _ := call3(procQueueSubmitForIndex, q.handle, uintptr(len(handles)), uintptr(unsafe.Pointer(&handles[0])))
	runtime.KeepAlive(handles)
	runtime.KeepAlive(commands)
	if typ, msg := LastUncapturedError(); msg != "" {
		// Fold device-lost uncaptured into sticky fuse before next surface acquire.
		if looksLikeDeviceLost(msg) {
			noteLostMessage(q.device, msg)
			return 0, ErrDeviceLost
		}
		return 0, &WGPUError{Op: "Queue.Submit", Type: typ, Message: msg}
	}
	return uint64(submissionIndex), nil
}

// Release releases the command buffer.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (cb *CommandBuffer) Release() {
	if cb == nil {
		return
	}
	releaseNativeHandle(&cb.handle, isOwnerDeviceLost(cb.device), func(h uintptr) {
		call1(procCommandBufferRelease, h)
	})
}

// Handle returns the underlying handle.
func (cb *CommandBuffer) Handle() uintptr { return cb.handle }
