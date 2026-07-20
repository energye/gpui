package rwgpu

import (
	"context"
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
)

// MapMode specifies the mapping mode for MapAsync.
type MapMode uint64

const (
	// MapModeNone indicates no mapping mode (default).
	MapModeNone MapMode = 0x0000000000000000
	// MapModeRead maps the buffer for reading via GetMappedRange.
	MapModeRead MapMode = 0x0000000000000001
	// MapModeWrite maps the buffer for writing via GetMappedRange.
	MapModeWrite MapMode = 0x0000000000000002
)

// MapAsyncStatus is the status returned by MapAsync callback.
type MapAsyncStatus uint32

const (
	// MapAsyncStatusSuccess indicates the buffer was successfully mapped.
	MapAsyncStatusSuccess MapAsyncStatus = 0x00000001
	// MapAsyncStatusCallbackCancelled indicates the callback was cancelled.
	MapAsyncStatusCallbackCancelled MapAsyncStatus = 0x00000002
	// MapAsyncStatusError indicates a mapping error occurred.
	MapAsyncStatusError MapAsyncStatus = 0x00000003
	// MapAsyncStatusAborted indicates the mapping was aborted (e.g., buffer destroyed).
	MapAsyncStatusAborted MapAsyncStatus = 0x00000004

	// Deprecated: use MapAsyncStatusCallbackCancelled.
	MapAsyncStatusInstanceDropped = MapAsyncStatusCallbackCancelled
)

// BufferMapCallbackInfo holds callback configuration for MapAsync.
type BufferMapCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Mode        CallbackMode
	Callback    uintptr // Function pointer
	Userdata1   uintptr
	Userdata2   uintptr
}

// mapRequest holds state for an async map request.
type mapRequest struct {
	done    chan struct{}
	status  MapAsyncStatus
	message string
}

var (
	// mapRequests is the global registry for pending map requests.
	// Protected by mapRequestsMu for concurrent access.
	mapRequests   = make(map[uintptr]*mapRequest)
	mapRequestsMu sync.Mutex
	mapRequestID  uintptr

	// mapCallbackPtr is the callback function pointer (created once).
	// Protected by mapCallbackOnce for concurrent initialization.
	mapCallbackPtr  uintptr
	mapCallbackOnce sync.Once
)

// mapCallbackHandler is the Go function called by native code via purego.NewCallback.
// Signature: void(status uint32, message StringView, userdata1 uintptr, userdata2 uintptr)
func mapCallbackHandler(status uintptr, messageData uintptr, messageLength uintptr, userdata1, userdata2 uintptr) uintptr {
	msg := callbackStringView(messageData, messageLength)

	// Find and complete the request
	mapRequestsMu.Lock()
	req, ok := mapRequests[userdata1]
	if ok {
		delete(mapRequests, userdata1)
	}
	mapRequestsMu.Unlock()

	if ok && req != nil {
		req.status = MapAsyncStatus(status)
		req.message = msg
		close(req.done)
	}
	return 0
}

// initMapCallback creates the C callback function pointer using purego.
func initMapCallback() {
	mapCallbackPtr = purego.NewCallback(mapCallbackHandler)
}

// BufferDescriptor describes a GPU buffer to create.
type BufferDescriptor struct {
	Label            string            // Buffer label for debugging
	Usage            types.BufferUsage // How the buffer will be used
	Size             uint64            // Size in bytes
	MappedAtCreation bool              // If true, buffer is mapped when created
}

// bufferDescriptorWire is the FFI-compatible C-layout struct for wgpu-native.
// CRITICAL: layout must match WGPUBufferDescriptor exactly.
// nextInChain(8)+label(16)+usage(8)+size(8)+mappedAtCreation(4)+pad(4) = 48 bytes.
type bufferDescriptorWire struct {
	NextInChain      uintptr           // *ChainedStruct
	Label            StringView        // Buffer label for debugging
	Usage            types.BufferUsage // How the buffer will be used
	Size             uint64            // Size in bytes
	MappedAtCreation Bool              // If true, buffer is mapped when created
	_pad             [4]byte           //nolint:unused // padding for FFI alignment
}

// CreateBuffer creates a new GPU buffer.
// Returns an error if the FFI call fails or the device/descriptor is nil.
func (d *Device) CreateBuffer(desc *BufferDescriptor) (*Buffer, error) {
	if err := prepareDeviceCall("CreateBuffer", d); err != nil {
		return nil, err
	}
	if desc == nil {
		return nil, &WGPUError{Op: "CreateBuffer", Message: "descriptor is nil"}
	}
	wire := bufferDescriptorWire{
		Label:            stringToStringView(desc.Label),
		Usage:            desc.Usage,
		Size:             desc.Size,
		MappedAtCreation: boolToWGPU(desc.MappedAtCreation),
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _, _ := procDeviceCreateBuffer.Call(
		d.handle,
		uintptr(unsafe.Pointer(&wire)),
	)
	runtime.KeepAlive(wire)
	runtime.KeepAlive(desc)
	if handle == 0 {
		return nil, &WGPUError{Op: "CreateBuffer", Message: "wgpu returned null handle"}
	}
	trackResource(handle, "Buffer")
	mapState := BufferMapStateUnmapped
	if desc.MappedAtCreation {
		mapState = BufferMapStateMapped
	}
	return &Buffer{handle: handle, device: d, mapState: mapState}, nil
}

// GetMappedRange returns a pointer to the mapped buffer data.
// The buffer must be mapped (either via MapAsync or MappedAtCreation).
// offset and size specify the range to access.
// Returns nil if the buffer is not mapped or the range is invalid.
func (b *Buffer) GetMappedRange(offset, size uint64) unsafe.Pointer {
	if b == nil || b.handle == 0 {
		return nil
	}
	if b.device != nil && b.device.IsLost() {
		return nil
	}
	if checkInit() != nil {
		return nil
	}
	_, _ = LastUncapturedError()
	ptr, _, _ := procBufferGetMappedRange.Call(
		b.handle,
		uintptr(offset),
		uintptr(size),
	)
	// Soft native: null + Uncaptured/DeviceLost instead of panic.
	if _, msg := LastUncapturedError(); looksLikeDeviceLost(msg) && b.device != nil {
		b.device.MarkLost()
	}
	if ptr == 0 {
		return nil
	}
	return ptrFromUintptr(ptr)
}

// Unmap unmaps the buffer, making the mapped memory inaccessible.
// For buffers created with MappedAtCreation, this commits the data to the GPU.
// Returns nil on success. Matches gogpu/wgpu Buffer.Unmap() error signature.
func (b *Buffer) Unmap() error {
	if b == nil || b.handle == 0 {
		return nil
	}
	if b.device != nil && b.device.IsLost() {
		b.mapState = BufferMapStateUnmapped
		return nil
	}
	if checkInit() != nil {
		return nil
	}
	procBufferUnmap.Call(b.handle) //nolint:errcheck
	b.mapState = BufferMapStateUnmapped
	// wgpu-native returns void for wgpuBufferUnmap; always nil per WebGPU spec.
	return nil
}

// Size returns the size of the buffer in bytes.
func (b *Buffer) Size() uint64 {
	if b == nil || b.handle == 0 {
		return 0
	}
	if b.device != nil && b.device.IsLost() {
		return 0
	}
	if checkInit() != nil {
		return 0
	}
	size, _, _ := procBufferGetSize.Call(b.handle)
	return uint64(size)
}

// MapAsyncBlocking maps a buffer for reading or writing, blocking until complete.
// Deprecated: Use [Buffer.Map] for blocking mapping or [Buffer.MapAsync] for non-blocking.
// This method is retained for backward compatibility.
func (b *Buffer) MapAsyncBlocking(device *Device, mode MapMode, offset, size uint64) error {
	if device != nil && b.device == nil {
		b.device = device
	}
	return b.Map(context.Background(), mode, offset, size)
}

// Destroy destroys the buffer, making it invalid.
// After Destroy the handle is nulled so subsequent ops cannot call native with
// a wild pointer. Idempotent; a following Release is a no-op.
// When the parent device is lost, only Go-side state is cleared.
func (b *Buffer) Destroy() {
	if b == nil {
		return
	}
	lost := b.device != nil && b.device.IsLost()
	destroyAndReleaseNativeHandle(&b.handle, lost,
		func(h uintptr) { procBufferDestroy.Call(h) }, //nolint:errcheck
		func(h uintptr) { procBufferRelease.Call(h) }, //nolint:errcheck
	)
	b.mapState = BufferMapStateUnmapped
}

// Release releases the buffer reference.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (b *Buffer) Release() {
	if b == nil {
		return
	}
	lost := b.device != nil && b.device.IsLost()
	releaseNativeHandle(&b.handle, lost, func(h uintptr) {
		procBufferRelease.Call(h) //nolint:errcheck
	})
	b.mapState = BufferMapStateUnmapped
}

// WriteBuffer writes data to a buffer.
// Returns nil on success. On this libwgpu_native.so, a lost parent yields a soft
// uncaptured "Parent device is lost" (not a fatal abort). We fold that into
// sticky device-lost and return ErrDeviceLost so surface acquire can refuse.
func (q *Queue) WriteBuffer(buffer *Buffer, offset uint64, data []byte) error {
	if err := gateQueue("Queue.WriteBuffer", q); err != nil {
		return err
	}
	if buffer != nil && buffer.device != nil && buffer.device.IsLost() {
		return ErrDeviceLost
	}
	if buffer == nil || buffer.handle == 0 || len(data) == 0 {
		return nil
	}
	if err := checkInit(); err != nil {
		return err
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	_, _ = LastUncapturedError() // attribute post-call errors to this write
	call5(procQueueWriteBuffer, q.handle, buffer.handle, uintptr(offset), uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	runtime.KeepAlive(data)
	runtime.KeepAlive(buffer)
	if typ, msg := LastUncapturedError(); msg != "" {
		if looksLikeDeviceLost(msg) {
			if buffer.device != nil {
				buffer.device.MarkLost()
			} else if q.device != 0 {
				markDeviceLost(q.device)
			}
			return ErrDeviceLost
		}
		return &WGPUError{Op: "Queue.WriteBuffer", Type: typ, Message: msg}
	}
	return nil
}

// WriteBufferTyped writes typed data to a buffer.
// The data pointer should point to the first element, size is total byte size.
func (q *Queue) WriteBufferRaw(buffer *Buffer, offset uint64, data unsafe.Pointer, size uint64) {
	if gateQueue("Queue.WriteBufferRaw", q) != nil {
		return
	}
	if buffer == nil || buffer.handle == 0 || size == 0 {
		return
	}
	if checkInit() != nil {
		return
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	call5(procQueueWriteBuffer, q.handle, buffer.handle, uintptr(offset), uintptr(data), uintptr(size))
	// Caller retains data; keep buffer object live across the FFI boundary.
	runtime.KeepAlive(buffer)
	runtime.KeepAlive(data)
}

// Usage returns the usage flags of this buffer.
func (b *Buffer) Usage() types.BufferUsage {
	if b == nil || b.handle == 0 {
		return types.BufferUsageNone
	}
	if b.device != nil && b.device.IsLost() {
		return types.BufferUsageNone
	}
	if checkInit() != nil {
		return types.BufferUsageNone
	}
	usage, _, _ := procBufferGetUsage.Call(b.handle)
	return types.BufferUsage(usage)
}

// MapState returns the current mapping state of this buffer.
func (b *Buffer) MapState() BufferMapState {
	if b == nil || b.handle == 0 {
		return BufferMapStateUnmapped
	}
	return b.mapState
}

// Handle returns the underlying handle. For advanced use only.
func (b *Buffer) Handle() uintptr { return b.handle }
