package rwgpu

import (
	"strings"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
)

// RequestDeviceCallbackInfo holds callback configuration for RequestDevice.
type RequestDeviceCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Mode        CallbackMode
	Callback    uintptr // Function pointer
	Userdata1   uintptr
	Userdata2   uintptr
}

// deviceRequest holds state for an async device request.
type deviceRequest struct {
	done    chan struct{}
	device  *Device
	status  RequestDeviceStatus
	message string
}

var (
	// deviceRequests is the global registry for pending device requests.
	// Protected by deviceRequestsMu for concurrent access.
	deviceRequests   = make(map[uintptr]*deviceRequest)
	deviceRequestsMu sync.Mutex
	deviceRequestID  uintptr

	// deviceCallbackPtr is the callback function pointer (created once).
	// Protected by deviceCallbackOnce for concurrent initialization.
	deviceCallbackPtr  uintptr
	deviceCallbackOnce sync.Once
)

// deviceCallbackHandler is the Go function called by native code via purego.NewCallback.
// Signature: void(status uint32, device uintptr, message StringView, userdata1 uintptr, userdata2 uintptr)
func deviceCallbackHandler(status uintptr, device uintptr, messageData uintptr, messageLength uintptr, userdata1, userdata2 uintptr) uintptr {
	msg := callbackStringView(messageData, messageLength)

	// Find and complete the request
	deviceRequestsMu.Lock()
	req, ok := deviceRequests[userdata1]
	if ok {
		delete(deviceRequests, userdata1)
	}
	deviceRequestsMu.Unlock()

	if ok && req != nil {
		req.status = RequestDeviceStatus(status)
		if device != 0 {
			trackResource(device, "Device")
			req.device = &Device{handle: device}
		}
		req.message = msg
		close(req.done)
	}
	return 0
}

// initDeviceCallback creates the C callback function pointer using purego.
func initDeviceCallback() {
	deviceCallbackPtr = purego.NewCallback(deviceCallbackHandler)
}

// Non-panicking device lifecycle callbacks. wgpu-native's default uncaptured
// handler aborts the process on Validation/OOM ("Not enough memory left"),
// which turns transient VRAM pressure into hard SIGABRT. We record the last
// error so CreateTexture/etc. can return a Go error (null handle path).
//
// Device-lost is sticky per handle: once the native device is lost, further
// wgpuSurfaceGetCurrentTexture / Configure calls panic in Rust with
// "Parent device is lost". Callers must check IsLost() and skip native ops.

var (
	uncapturedErrorCallbackPtr  uintptr
	uncapturedErrorCallbackOnce sync.Once
	deviceLostCallbackPtr       uintptr
	deviceLostCallbackOnce      sync.Once

	lastUncapturedMu  sync.Mutex
	lastUncapturedMsg string
	lastUncapturedTyp ErrorType

	// liveDevices maps native WGPUDevice handle → *Device so DeviceLostCallback
	// can set d.lost on the correct Go object (callback only provides the handle).
	// Not a "lost set" — only registration for routing the official callback.
	liveDevices sync.Map // map[uintptr]*Device
)

// LastUncapturedError returns and clears the most recent uncaptured device error.
func LastUncapturedError() (ErrorType, string) {
	lastUncapturedMu.Lock()
	defer lastUncapturedMu.Unlock()
	t, m := lastUncapturedTyp, lastUncapturedMsg
	lastUncapturedTyp = 0
	lastUncapturedMsg = ""
	return t, m
}

// registerLiveDevice records handle → *Device for DeviceLostCallback routing.
func registerLiveDevice(d *Device) {
	if d == nil || d.handle == 0 {
		return
	}
	liveDevices.Store(d.handle, d)
}

// unregisterLiveDevice removes a device from callback routing (on Release).
func unregisterLiveDevice(d *Device) {
	if d == nil || d.handle == 0 {
		return
	}
	liveDevices.Delete(d.handle)
}

// markDeviceLost sets Device.lost for the registered handle (DeviceLostCallback).
func markDeviceLost(handle uintptr) {
	if handle == 0 {
		return
	}
	if v, ok := liveDevices.Load(handle); ok {
		if d, ok := v.(*Device); ok && d != nil {
			d.lost.Store(true)
		}
	}
}

// IsDeviceHandleLost reports lost state for a registered native handle.
func IsDeviceHandleLost(handle uintptr) bool {
	if handle == 0 {
		return false
	}
	if v, ok := liveDevices.Load(handle); ok {
		if d, ok := v.(*Device); ok && d != nil {
			return d.lost.Load()
		}
	}
	return false
}

// IsLost reports DeviceLostCallback state for this device. Safe on nil.
func (d *Device) IsLost() bool {
	return d != nil && d.lost.Load()
}

// uncapturedErrorHandler records validation/OOM/etc. Does not mark device lost.
func uncapturedErrorHandler(devicePtr, errType, messageData, messageLength, _, _ uintptr) uintptr {
	_ = devicePtr
	msg := callbackStringView(messageData, messageLength)
	lastUncapturedMu.Lock()
	lastUncapturedTyp = ErrorType(errType)
	lastUncapturedMsg = msg
	lastUncapturedMu.Unlock()
	return 0
}

func initUncapturedErrorCallback() {
	uncapturedErrorCallbackPtr = purego.NewCallback(uncapturedErrorHandler)
}

// deviceLostHandler is WGPUDeviceLostCallback — sole writer of Device.lost.
func deviceLostHandler(devicePtr, reason, messageData, messageLength, userdata1, userdata2 uintptr) uintptr {
	_, _, _, _ = reason, messageData, messageLength, userdata1
	_ = userdata2
	markDeviceLost(deviceHandleFromCallbackArg(devicePtr))
	return 0
}

// deviceHandleFromCallbackArg extracts a WGPUDevice handle from the C callback's
// first argument. webgpu.h passes WGPUDevice const* (pointer-to-handle); some
// purego/ABI paths may pass the handle value directly.
// Returns 0 when the pointed-to device is null (e.g. FailedCreation) — does not
// mark any other device lost.
func deviceHandleFromCallbackArg(devicePtr uintptr) uintptr {
	if devicePtr == 0 {
		return 0
	}
	h := *(*uintptr)(ptrFromUintptr(devicePtr))
	if h != 0 {
		return h
	}
	// ABI fallback: treat arg as the handle itself when non-null.
	return devicePtr
}

func looksLikeDeviceLost(msg string) bool {
	if msg == "" {
		return false
	}
	// Match common wgpu / Vulkan / DX12 / Metal phrasings (log + uncaptured).
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "device lost") ||
		strings.Contains(lower, "device_lost") ||
		strings.Contains(lower, "parent device is lost") ||
		strings.Contains(lower, "device is lost") ||
		strings.Contains(lower, "lost the device") ||
		strings.Contains(lower, "dxgienum::device_removed") ||
		strings.Contains(lower, "device_removed") ||
		strings.Contains(lower, "vk_error_device_lost")
}

func initDeviceLostCallback() {
	deviceLostCallbackPtr = purego.NewCallback(deviceLostHandler)
}

// RequestDevice requests a GPU device from the adapter.
// This is a synchronous wrapper that blocks until the device is available.
func (a *Adapter) RequestDevice(options *DeviceDescriptor) (*Device, error) {
	if err := checkInit(); err != nil {
		return nil, err
	}
	if a == nil || a.handle == 0 {
		return nil, &WGPUError{Op: "RequestDevice", Message: "adapter is nil or released"}
	}

	// Initialize callback once
	deviceCallbackOnce.Do(initDeviceCallback)

	// Create request state
	req := &deviceRequest{
		done: make(chan struct{}),
	}

	// Register request
	deviceRequestsMu.Lock()
	deviceRequestID++
	reqID := deviceRequestID
	deviceRequests[reqID] = req
	deviceRequestsMu.Unlock()

	// Convert Go-idiomatic descriptor to wire format. Always attach non-panicking
	// uncaptured/device-lost callbacks so VRAM pressure returns Go errors.
	uncapturedErrorCallbackOnce.Do(initUncapturedErrorCallback)
	deviceLostCallbackOnce.Do(initDeviceLostCallback)
	var reqLimitsWire limitsWire // kept alive for the duration of the FFI call
	wire := deviceDescriptorWire{
		DeviceLostCallbackInfo: DeviceLostCallbackInfo{
			Mode:     CallbackModeAllowProcessEvents, // delivered in ProcessEvents
			Callback: deviceLostCallbackPtr,
		},
		UncapturedErrorCallbackInfo: UncapturedErrorCallbackInfo{
			Callback: uncapturedErrorCallbackPtr,
		},
	}
	if options != nil {
		wire.Label = stringToStringView(options.Label)
		if len(options.RequiredFeatures) > 0 {
			wire.RequiredFeatureCount = uintptr(len(options.RequiredFeatures))
			wire.RequiredFeatures = uintptr(unsafe.Pointer(&options.RequiredFeatures[0]))
		}
		if options.RequiredLimits != nil {
			reqLimitsWire = limitsToWire(options.RequiredLimits)
			wire.RequiredLimits = uintptr(unsafe.Pointer(&reqLimitsWire))
		}
	}
	optionsPtr := uintptr(unsafe.Pointer(&wire))
	_ = reqLimitsWire // ensure not optimised away before the call below

	// Prepare callback info
	callbackInfo := RequestDeviceCallbackInfo{
		NextInChain: 0,
		Mode:        CallbackModeWaitAnyOnly,
		Callback:    deviceCallbackPtr,
		Userdata1:   reqID,
		Userdata2:   0,
	}

	future, err := callAdapterRequestDevice(a.handle, optionsPtr, &callbackInfo)
	if err != nil {
		return nil, err
	}
	if err := waitForFuture(a.instance, future, "RequestDevice"); err != nil {
		deviceRequestsMu.Lock()
		delete(deviceRequests, reqID)
		deviceRequestsMu.Unlock()
		return nil, err
	}

	select {
	case <-req.done:
		if req.status != RequestDeviceStatusSuccess {
			msg := req.message
			if msg == "" {
				msg = "device request failed"
			}
			return nil, &WGPUError{Op: "RequestDevice", Message: msg}
		}
		if req.device != nil {
			req.device.limits = fetchDeviceLimits(req.device.handle)
			req.device.instance = a.instance
			// Route future DeviceLostCallback to this *Device via handle.
			registerLiveDevice(req.device)
		}
		return req.device, nil
	default:
		return nil, &WGPUError{Op: "RequestDevice", Message: "future completed without invoking callback"}
	}
}

func callAdapterRequestDevice(adapter uintptr, options uintptr, callbackInfo *RequestDeviceCallbackInfo) (Future, error) {
	proc, ok := procAdapterRequestDevice.(*unixProc)
	if !ok {
		future, _, err := procAdapterRequestDevice.Call(
			adapter,
			options,
			uintptr(unsafe.Pointer(callbackInfo)),
		)
		return Future{ID: uint64(future)}, err
	}
	if proc.fnPtr == 0 {
		return Future{}, &WGPUError{Op: "RequestDevice", Message: "wgpuAdapterRequestDevice symbol is missing"}
	}

	var requestDevice func(uintptr, uintptr, RequestDeviceCallbackInfo) Future
	purego.RegisterFunc(&requestDevice, proc.fnPtr)
	return requestDevice(adapter, options, *callbackInfo), nil
}

// fetchDeviceLimits calls wgpuDeviceGetLimits and converts the wire struct to public Limits.
// Returns zero-value Limits on failure (non-fatal: limits remain valid defaults).
func fetchDeviceLimits(handle uintptr) Limits {
	var wire limitsWire
	status, _, _ := procDeviceGetLimits.Call(
		handle,
		uintptr(unsafe.Pointer(&wire)),
	)
	if WGPUStatus(status) != WGPUStatusSuccess {
		return Limits{}
	}
	return limitsFromWire(&wire)
}

// Queue returns the default queue for the device.
// Returns nil when the device is nil/released or the sticky device-lost fuse is set
// (avoids purego Call on a lost device handle).
func (d *Device) Queue() *Queue {
	if d == nil || d.handle == 0 {
		return nil
	}
	if refuseIfLost("Device.Queue", d.handle) != nil {
		return nil
	}
	if checkInit() != nil {
		return nil
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	handle, _, _ := procDeviceGetQueue.Call(d.handle)
	if handle == 0 {
		return nil
	}
	trackResource(handle, "Queue")
	return &Queue{handle: handle, device: d.handle}
}

// Poll polls the device for completed work.
// If wait is true, blocks until there is work to process.
// Returns true if the queue is empty.
// This is a wgpu-native extension.
// After device-lost, returns true without calling native (queue treated as empty).
func (d *Device) Poll(wait bool) bool {
	if d == nil || d.handle == 0 {
		return true
	}
	if refuseIfLost("Device.Poll", d.handle) != nil {
		return true
	}
	if checkInit() != nil {
		return true
	}
	var waitArg uintptr
	if wait {
		waitArg = 1
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	result, _, _ := procDevicePoll.Call(d.handle, waitArg, 0)
	return result != 0
}

// Release releases the device resources.
func (d *Device) Release() {
	if d == nil {
		return
	}
	if d.handle != 0 {
		unregisterLiveDevice(d)
		untrackResource(d.handle)
		procDeviceRelease.Call(d.handle) //nolint:errcheck
		d.handle = 0
	}
}

// Release releases the queue resources.
func (q *Queue) Release() {
	if q.handle != 0 {
		untrackResource(q.handle)
		procQueueRelease.Call(q.handle) //nolint:errcheck
		q.handle = 0
	}
}

// DeviceLostCallbackInfo configures the device-lost callback.
type DeviceLostCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Mode        CallbackMode
	Callback    uintptr // Function pointer
	Userdata1   uintptr
	Userdata2   uintptr
}

// UncapturedErrorCallbackInfo configures the uncaptured-error callback.
type UncapturedErrorCallbackInfo struct {
	NextInChain uintptr // *ChainedStruct
	Callback    uintptr // Function pointer
	Userdata1   uintptr
	Userdata2   uintptr
}

// DeviceDescriptor configures device creation.
// Matches the gogpu/wgpu API for cross-project compatibility.
type DeviceDescriptor struct {
	// Label is an optional debug label for the device.
	Label string
	// RequiredFeatures lists GPU features that the device must support.
	RequiredFeatures []FeatureName
	// RequiredLimits, if non-nil, specifies minimum resource limits the device must meet.
	// Pass nil to use the adapter's default limits.
	RequiredLimits *Limits
}

// limitsToWire converts public Limits to the FFI-compatible limitsWire struct.
// Used when passing required limits to wgpuAdapterRequestDevice.
func limitsToWire(l *Limits) limitsWire {
	if l == nil {
		return limitsWire{}
	}
	return limitsWire{
		MaxTextureDimension1D:                     l.MaxTextureDimension1D,
		MaxTextureDimension2D:                     l.MaxTextureDimension2D,
		MaxTextureDimension3D:                     l.MaxTextureDimension3D,
		MaxTextureArrayLayers:                     l.MaxTextureArrayLayers,
		MaxBindGroups:                             l.MaxBindGroups,
		MaxBindGroupsPlusVertexBuffers:            l.MaxBindGroupsPlusVertexBuffers,
		MaxBindingsPerBindGroup:                   l.MaxBindingsPerBindGroup,
		MaxDynamicUniformBuffersPerPipelineLayout: l.MaxDynamicUniformBuffersPerPipelineLayout,
		MaxDynamicStorageBuffersPerPipelineLayout: l.MaxDynamicStorageBuffersPerPipelineLayout,
		MaxSampledTexturesPerShaderStage:          l.MaxSampledTexturesPerShaderStage,
		MaxSamplersPerShaderStage:                 l.MaxSamplersPerShaderStage,
		MaxStorageBuffersPerShaderStage:           l.MaxStorageBuffersPerShaderStage,
		MaxStorageTexturesPerShaderStage:          l.MaxStorageTexturesPerShaderStage,
		MaxUniformBuffersPerShaderStage:           l.MaxUniformBuffersPerShaderStage,
		MaxUniformBufferBindingSize:               l.MaxUniformBufferBindingSize,
		MaxStorageBufferBindingSize:               l.MaxStorageBufferBindingSize,
		MinUniformBufferOffsetAlignment:           l.MinUniformBufferOffsetAlignment,
		MinStorageBufferOffsetAlignment:           l.MinStorageBufferOffsetAlignment,
		MaxVertexBuffers:                          l.MaxVertexBuffers,
		MaxBufferSize:                             l.MaxBufferSize,
		MaxVertexAttributes:                       l.MaxVertexAttributes,
		MaxVertexBufferArrayStride:                l.MaxVertexBufferArrayStride,
		MaxInterStageShaderVariables:              l.MaxInterStageShaderVariables,
		MaxColorAttachments:                       l.MaxColorAttachments,
		MaxColorAttachmentBytesPerSample:          l.MaxColorAttachmentBytesPerSample,
		MaxComputeWorkgroupStorageSize:            l.MaxComputeWorkgroupStorageSize,
		MaxComputeInvocationsPerWorkgroup:         l.MaxComputeInvocationsPerWorkgroup,
		MaxComputeWorkgroupSizeX:                  l.MaxComputeWorkgroupSizeX,
		MaxComputeWorkgroupSizeY:                  l.MaxComputeWorkgroupSizeY,
		MaxComputeWorkgroupSizeZ:                  l.MaxComputeWorkgroupSizeZ,
		MaxComputeWorkgroupsPerDimension:          l.MaxComputeWorkgroupsPerDimension,
	}
}

// deviceDescriptorWire is the FFI-compatible C-layout struct for wgpuAdapterRequestDevice.
// v29: Added Label, RequiredFeatureCount, RequiredFeatures, RequiredLimits,
// DefaultQueue, DeviceLostCallbackInfo, UncapturedErrorCallbackInfo fields.
type deviceDescriptorWire struct {
	NextInChain                 uintptr // *ChainedStruct
	Label                       StringView
	RequiredFeatureCount        uintptr // size_t
	RequiredFeatures            uintptr // *FeatureName (const)
	RequiredLimits              uintptr // *Limits (const, nullable)
	DefaultQueue                QueueDescriptor
	DeviceLostCallbackInfo      DeviceLostCallbackInfo
	UncapturedErrorCallbackInfo UncapturedErrorCallbackInfo
}

// QueueDescriptor configures queue creation.
type QueueDescriptor struct {
	NextInChain uintptr // *ChainedStruct
	Label       StringView
}

// CreateDepthTexture creates a depth texture with the specified dimensions and format.
// This is a convenience function for creating depth buffers for render passes.
// Returns nil on error (use CreateTexture directly for full error handling).
func (d *Device) CreateDepthTexture(width, height uint32, format types.TextureFormat) *Texture {
	desc := TextureDescriptor{
		Usage:         types.TextureUsageRenderAttachment,
		Dimension:     types.TextureDimension2D,
		Size:          types.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1},
		Format:        format,
		MipLevelCount: 1,
		SampleCount:   1,
	}

	t, _ := d.CreateTexture(&desc)
	return t
}

// Limits returns the resource limits of this device.
//
// Limits are cached at device creation time and returned by value.
// No FFI call is made. Returns zero-value Limits if the device is nil.
// This matches the gogpu/wgpu API signature for cross-project compatibility.
func (d *Device) Limits() Limits {
	if d == nil || d.handle == 0 {
		return Limits{}
	}
	return d.limits
}

// Features retrieves all features enabled on this device.
// Returns a slice of FeatureName values.
func (d *Device) Features() []FeatureName {
	mustInit()
	if d == nil || d.handle == 0 {
		return nil
	}

	// Call wgpuDeviceGetFeatures to populate SupportedFeatures struct
	var supported SupportedFeatures
	procDeviceGetFeatures.Call( //nolint:errcheck
		d.handle,
		uintptr(unsafe.Pointer(&supported)),
	)

	if supported.FeatureCount == 0 || supported.Features == 0 {
		return nil
	}

	// Convert C array to Go slice
	featuresPtr := (*FeatureName)(ptrFromUintptr(supported.Features))
	features := unsafe.Slice(featuresPtr, supported.FeatureCount)

	// Copy to new slice (don't keep pointer to C memory)
	result := make([]FeatureName, supported.FeatureCount)
	copy(result, features)

	// Free C-allocated memory (pass pointer to struct, not individual fields)
	procSupportedFeaturesFreeMembers.Call(uintptr(unsafe.Pointer(&supported))) //nolint:errcheck

	return result
}

// HasFeature checks if the device has a specific feature enabled.
func (d *Device) HasFeature(feature FeatureName) bool {
	mustInit()
	if d == nil || d.handle == 0 {
		return false
	}

	result, _, _ := procDeviceHasFeature.Call(
		d.handle,
		uintptr(feature),
	)

	return Bool(result) == True
}

// pumpInstanceEvents runs wgpuInstanceProcessEvents (delivers DeviceLost callbacks).
func pumpInstanceEvents(d *Device) {
	if d == nil || d.instance == 0 || procInstanceProcessEvents == nil {
		return
	}
	procInstanceProcessEvents.Call(d.instance) //nolint:errcheck
}
