//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rwgpu

import (
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/energye/gpui/gpu/types"
)

// LastUncapturedError returns and clears the most recent uncaptured device error.
func LastUncapturedError() (ErrorType, string) {
	lastUncapturedMu.Lock()
	defer lastUncapturedMu.Unlock()
	t, m := lastUncapturedTyp, lastUncapturedMsg
	lastUncapturedTyp = 0
	lastUncapturedMsg = ""
	lastUncapturedDevice = 0
	return t, m
}

// PeekLastUncapturedError returns the sticky uncaptured error without clearing.
// The second return is the native device handle that reported it (0 if unknown).
// PeekLastUncapturedError returns the sticky uncaptured error without clearing.
// The second return is the native device handle that reported it (0 if unknown).
func PeekLastUncapturedError() (typ ErrorType, msg string, deviceHandle uintptr) {
	lastUncapturedMu.Lock()
	defer lastUncapturedMu.Unlock()
	return lastUncapturedTyp, lastUncapturedMsg, lastUncapturedDevice
}

// allocDeviceSlot reserves a userdata slot before RequestDevice so DeviceLost /
// Uncaptured callbacks can identify this logical device even before the native
// handle exists. Call bindDeviceSlot after RequestDevice succeeds.
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
	//
	// Per-device userdata slot: multi-window apps each RequestDevice with their
	// own slot so DeviceLost / uncaptured lost marks only that Device.
	deviceSlot := allocDeviceSlot()
	uncapturedErrorCallbackOnce.Do(initUncapturedErrorCallback)
	deviceLostCallbackOnce.Do(initDeviceLostCallback)
	var reqLimitsWire limitsWire // kept alive for the duration of the FFI call
	wire := deviceDescriptorWire{
		DeviceLostCallbackInfo: DeviceLostCallbackInfo{
			// AllowSpontaneous: mark lost as soon as native decides, not only
			// on the next ProcessEvents. GetCurrentTexture aborts if we race.
			Mode:      CallbackModeAllowSpontaneous,
			Callback:  deviceLostCallbackPtr,
			Userdata1: deviceSlot,
		},
		UncapturedErrorCallbackInfo: UncapturedErrorCallbackInfo{
			Callback:  uncapturedErrorCallbackPtr,
			Userdata1: deviceSlot,
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
		freeDeviceSlot(deviceSlot)
		return nil, err
	}
	if err := waitForFuture(a.instance, future, "RequestDevice"); err != nil {
		deviceRequestsMu.Lock()
		delete(deviceRequests, reqID)
		deviceRequestsMu.Unlock()
		freeDeviceSlot(deviceSlot)
		return nil, err
	}

	select {
	case <-req.done:
		if req.status != RequestDeviceStatusSuccess {
			msg := req.message
			if msg == "" {
				msg = "device request failed"
			}
			freeDeviceSlot(deviceSlot)
			return nil, &WGPUError{Op: "RequestDevice", Message: msg}
		}
		if req.device != nil {
			req.device.limits = fetchDeviceLimits(req.device.handle)
			req.device.instance = a.instance
			// Primary route: userdata slot; secondary: native handle map.
			bindDeviceSlot(deviceSlot, req.device)
			registerLiveDevice(req.device)
			// path). Eager alloc on every RequestDevice exhausted VRAM in unit
			// suites that create many short-lived devices.
		} else {
			freeDeviceSlot(deviceSlot)
		}
		return req.device, nil
	default:
		freeDeviceSlot(deviceSlot)
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
// Queue returns the default queue for the device.
// Returns nil when the device is nil/released or the sticky device-lost fuse is set
// (avoids purego Call on a lost device handle).
func (d *Device) Queue() *Queue {
	if d == nil || d.handle == 0 {
		return nil
	}
	if d.IsLost() {
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

// absorbLostUncapturedLocked consumes a pending uncaptured lost message for this
// device and marks sticky lost. Returns true if the device is now lost.
// Caller should hold gpuMu when folding with surface acquire.
// Destroy forces native device teardown and sticky-lost state (Skia/Flutter
// abandon-context). Sticky is force-marked so public APIs refuse with
// ErrDeviceLost even if DeviceLostCallback is not delivered. Idempotent and nil-safe.
func (d *Device) Destroy() {
	if d == nil {
		return
	}
	// Sticky abandon so concurrent ops refuse with ErrDeviceLost.
	d.MarkLost()

	if d.handle == 0 {
		return
	}
	h := d.handle
	// Keep lostDeviceHandles[h] (set by MarkLost) after we clear d.handle.
	unregisterLiveDevice(d)

	if checkInit() == nil && procDeviceDestroy != nil {
		gpuMu.Lock()
		if nativeCallHook != nil {
			nativeCallHook("device_destroy")
		}
		// Intentional native Destroy even though sticky-lost: this is the force path.
		procDeviceDestroy.Call(h) //nolint:errcheck
		// Best-effort delivery of DeviceLost (often a no-op on this build).
		if d.instance != 0 && procInstanceProcessEvents != nil {
			procInstanceProcessEvents.Call(d.instance) //nolint:errcheck
		}
		// Reclaim the device ref after Destroy (Release last ref).
		if procDeviceRelease != nil {
			procDeviceRelease.Call(h) //nolint:errcheck
		}
		gpuMu.Unlock()
	}
	untrackResource(h)
	d.handle = 0
}

// Poll polls the device for completed work.
// If wait is true, blocks until there is work to process.
// Returns true if the queue is empty.
// This is a wgpu-native extension.
// After device-lost, returns true without calling native (queue treated as empty).
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
	_, _ = LastUncapturedError()
	result, _, _ := procDevicePoll.Call(d.handle, waitArg, 0)
	// Soft native: poll errors report via ErrorSink then return false.
	if _, msg := LastUncapturedError(); looksLikeDeviceLost(msg) {
		d.MarkLost()
	}
	return result != 0
}

// Release releases the device resources.
// Nil-safe and idempotent. When the device is already lost, only Go-side
// state is cleared — native DeviceRelease is skipped when lost.
// Release releases the device resources.
// Nil-safe and idempotent. When the device is already lost, only Go-side
// state is cleared — native DeviceRelease is skipped when lost.
func (d *Device) Release() {
	if d == nil {
		return
	}
	lost := d.IsLost()
	if d.handle != 0 {
		// Preserve sticky lost map entry for this handle before unregister.
		if lost {
			lostDeviceHandles.Store(d.handle, struct{}{})
		}
		unregisterLiveDevice(d)
	}
	releaseNativeHandle(&d.handle, lost, func(h uintptr) {
		procDeviceRelease.Call(h) //nolint:errcheck
	})
}

// Release releases the queue resources.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
// Release releases the queue resources.
// Nil-safe and idempotent. Skips native release when the parent device is lost.
func (q *Queue) Release() {
	if q == nil {
		return
	}
	lost := isOwnerDeviceLost(q.device)
	releaseNativeHandle(&q.handle, lost, func(h uintptr) {
		procQueueRelease.Call(h) //nolint:errcheck
	})
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

// CreateDepthTextureErr creates a depth texture and reports the native error
// instead of swallowing it, so callers under VRAM pressure can tell OOM
// (degrade/retry) from misuse (fail fast) instead of blind-retesting nil.
// CreateDepthTextureErr creates a depth texture and reports the native error
// instead of swallowing it, so callers under VRAM pressure can tell OOM
// (degrade/retry) from misuse (fail fast) instead of blind-retesting nil.
func (d *Device) CreateDepthTextureErr(width, height uint32, format types.TextureFormat) (*Texture, error) {
	desc := TextureDescriptor{
		Usage:         types.TextureUsageRenderAttachment,
		Dimension:     types.TextureDimension2D,
		Size:          types.Extent3D{Width: width, Height: height, DepthOrArrayLayers: 1},
		Format:        format,
		MipLevelCount: 1,
		SampleCount:   1,
	}
	return d.CreateTexture(&desc)
}

// CreateDepthTexture creates a depth texture with the specified dimensions and format.
// This is a convenience function for creating depth buffers for render passes.
// Returns nil on error (use CreateDepthTextureErr for full error handling).
// The ledger charge in CreateTexture is best-effort: on failure this refunds
// by handle is impossible (no handle), so the check-then-create gap can
// strand at most one texture estimate (~4MB at 1080p) until process exit —
// negligible against the 768MB budget and far cheaper than a ledger mutex
// around the native call.
// CreateDepthTexture creates a depth texture with the specified dimensions and format.
// This is a convenience function for creating depth buffers for render passes.
// Returns nil on error (use CreateDepthTextureErr for full error handling).
// The ledger charge in CreateTexture is best-effort: on failure this refunds
// by handle is impossible (no handle), so the check-then-create gap can
// strand at most one texture estimate (~4MB at 1080p) until process exit —
// negligible against the 768MB budget and far cheaper than a ledger mutex
// around the native call.
func (d *Device) CreateDepthTexture(width, height uint32, format types.TextureFormat) *Texture {
	t, _ := d.CreateDepthTextureErr(width, height, format)
	return t
}

// Limits returns the resource limits of this device.
//
// Limits are cached at device creation time and returned by value.
// No FFI call is made. Returns zero-value Limits if the device is nil.
// Limits returns the resource limits of this device.
//
// Limits are cached at device creation time and returned by value.
// No FFI call is made. Returns zero-value Limits if the device is nil.
func (d *Device) Limits() Limits {
	if d == nil || d.handle == 0 {
		return Limits{}
	}
	return d.limits
}

// Features retrieves all features enabled on this device.
// Returns a slice of FeatureName values.
// Defense order: nil/zero handle → lost → init → native.
// Features retrieves all features enabled on this device.
// Returns a slice of FeatureName values.
// Defense order: nil/zero handle → lost → init → native.
func (d *Device) Features() []FeatureName {
	if d == nil || d.handle == 0 {
		return nil
	}
	if refuseIfLost("Device.Features", d.handle) != nil {
		return nil
	}
	if checkInit() != nil {
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
// Defense order: nil/zero handle → lost → init → native.
// HasFeature checks if the device has a specific feature enabled.
// Defense order: nil/zero handle → lost → init → native.
func (d *Device) HasFeature(feature FeatureName) bool {
	if d == nil || d.handle == 0 {
		return false
	}
	if refuseIfLost("Device.HasFeature", d.handle) != nil {
		return false
	}
	if checkInit() != nil {
		return false
	}

	result, _, _ := procDeviceHasFeature.Call(
		d.handle,
		uintptr(feature),
	)

	return Bool(result) == True
}

// pumpInstanceEvents runs wgpuInstanceProcessEvents (delivers DeviceLost callbacks).
// pumpInstanceEvents runs wgpuInstanceProcessEvents (delivers DeviceLost callbacks).
func pumpInstanceEvents(d *Device) {
	if d == nil || d.instance == 0 || procInstanceProcessEvents == nil {
		return
	}
	procInstanceProcessEvents.Call(d.instance) //nolint:errcheck
}
