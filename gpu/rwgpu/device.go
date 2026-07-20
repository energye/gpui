package rwgpu

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
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

	lastUncapturedMu     sync.Mutex
	lastUncapturedMsg    string
	lastUncapturedTyp    ErrorType
	lastUncapturedDevice uintptr // native handle of the device that reported it (0=unknown)

	// liveDevices maps native WGPUDevice handle → *Device (secondary route).
	liveDevices sync.Map // map[uintptr]*Device

	// deviceSlots maps callback Userdata1 slot id → *Device.
	// Multi-window: each Device has its own slot; lost marks only that device.
	deviceSlots   sync.Map // map[uintptr]*Device
	deviceSlotSeq atomic.Uint64

	// lostDeviceHandles: sticky lost native handles after Device.Release.
	lostDeviceHandles sync.Map // map[uintptr]struct{}
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
func PeekLastUncapturedError() (typ ErrorType, msg string, deviceHandle uintptr) {
	lastUncapturedMu.Lock()
	defer lastUncapturedMu.Unlock()
	return lastUncapturedTyp, lastUncapturedMsg, lastUncapturedDevice
}

// allocDeviceSlot reserves a userdata slot before RequestDevice so DeviceLost /
// Uncaptured callbacks can identify this logical device even before the native
// handle exists. Call bindDeviceSlot after RequestDevice succeeds.
func allocDeviceSlot() uintptr {
	id := deviceSlotSeq.Add(1)
	if id == 0 {
		id = deviceSlotSeq.Add(1)
	}
	slot := uintptr(id)
	// Placeholder until bindDeviceSlot; prevents free-slot reuse races.
	deviceSlots.Store(slot, (*Device)(nil))
	return slot
}

// bindDeviceSlot associates a userdata slot with the created *Device.
func bindDeviceSlot(slot uintptr, d *Device) {
	if slot == 0 || d == nil {
		return
	}
	d.callbackUserdata = slot
	deviceSlots.Store(slot, d)
}

// freeDeviceSlot drops userdata routing for a released or failed device.
func freeDeviceSlot(slot uintptr) {
	if slot == 0 {
		return
	}
	deviceSlots.Delete(slot)
}

// deviceFromUserdata resolves the *Device registered for a callback Userdata1.
func deviceFromUserdata(userdata1 uintptr) *Device {
	if userdata1 == 0 {
		return nil
	}
	if v, ok := deviceSlots.Load(userdata1); ok {
		if d, ok := v.(*Device); ok {
			return d // may be nil while RequestDevice is in flight
		}
	}
	return nil
}

// registerLiveDevice records handle → *Device for handle-based lost routing.
// A newly registered device is healthy: any stale sticky lost mark for this
// handle is dropped (tests reuse fake handles; native handles are unique).
func registerLiveDevice(d *Device) {
	if d == nil || d.handle == 0 {
		return
	}
	lostDeviceHandles.Delete(d.handle)
	d.lost.Store(false)
	liveDevices.Store(d.handle, d)
	if d.callbackUserdata != 0 {
		deviceSlots.Store(d.callbackUserdata, d)
	}
}

// unregisterLiveDevice removes a device from callback routing (on Release).
func unregisterLiveDevice(d *Device) {
	if d == nil {
		return
	}
	if d.handle != 0 {
		liveDevices.Delete(d.handle)
	}
	if d.callbackUserdata != 0 {
		freeDeviceSlot(d.callbackUserdata)
		d.callbackUserdata = 0
	}
}

// markDeviceLost sets Device.lost for the registered handle only (per-device).
func markDeviceLost(handle uintptr) {
	if handle == 0 {
		return
	}
	lostDeviceHandles.Store(handle, struct{}{})
	if v, ok := liveDevices.Load(handle); ok {
		if d, ok := v.(*Device); ok && d != nil {
			d.lost.Store(true)
		}
	}
}

// markDeviceObjectLost marks one *Device sticky-lost (userdata primary path).
func markDeviceObjectLost(d *Device) {
	if d == nil {
		return
	}
	d.lost.Store(true)
	if d.handle != 0 {
		lostDeviceHandles.Store(d.handle, struct{}{})
	}
}

// markDeviceLostFromCallback routes a native device-lost signal onto exactly
// one Device when possible:
//  1. userdata1 slot (preferred — multi-window / multi-device)
//  2. native device handle from callback arg
//
// Never marks unrelated live devices.
func markDeviceLostFromCallback(devicePtr, userdata1 uintptr) {
	if d := deviceFromUserdata(userdata1); d != nil {
		markDeviceObjectLost(d)
		return
	}
	h := deviceHandleFromCallbackArg(devicePtr)
	if h != 0 {
		markDeviceLost(h)
		return
	}
	// purego may pass WGPUDevice by value.
	if devicePtr != 0 {
		if _, ok := liveDevices.Load(devicePtr); ok {
			markDeviceLost(devicePtr)
		}
	}
	// Unresolved: do NOT mark all devices (would poison other windows).
}

// noteLostMessage marks sticky lost for one known device handle only.
// If deviceHandle is 0, no device is marked (caller must pass the owner).
func noteLostMessage(deviceHandle uintptr, msg string) {
	if !looksLikeDeviceLost(msg) || deviceHandle == 0 {
		return
	}
	markDeviceLost(deviceHandle)
}

// IsDeviceHandleLost reports sticky lost state for a native device handle.
// Survives Device.Release (liveDevices unregister) so child Release/Destroy
// still skip native after the parent Device object is gone.
func IsDeviceHandleLost(handle uintptr) bool {
	if handle == 0 {
		return false
	}
	if _, ok := lostDeviceHandles.Load(handle); ok {
		return true
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

// MarkLost sets sticky device-lost state (same effect as DeviceLostCallback /
// uncaptured "Parent device is lost"). Safe on nil. Used by facade tests and
// recovery paths that inject lost without a native callback.
func (d *Device) MarkLost() {
	if d == nil {
		return
	}
	d.lost.Store(true)
	if d.handle != 0 {
		lostDeviceHandles.Store(d.handle, struct{}{})
	}
}

// uncapturedErrorHandler records validation/OOM/etc. When the message looks
// like device-lost (e.g. "Parent device is lost"), also marks the owning
// device sticky-lost so subsequent public calls refuse with ErrDeviceLost
// instead of treating the device as healthy.
//
// Critical: wgpuSurfaceGetCurrentTexture panics in Rust when parent is lost.
// We must mark sticky on the FIRST uncaptured lost message so the next
// GetCurrentTexture refuses before purego Call.
func uncapturedErrorHandler(devicePtr, errType, messageData, messageLength, userdata1, _ uintptr) uintptr {
	msg := callbackStringView(messageData, messageLength)
	// Resolve owning device for sticky storage + multi-device isolation.
	ownerHandle := uintptr(0)
	if d := deviceFromUserdata(userdata1); d != nil && d.handle != 0 {
		ownerHandle = d.handle
	} else {
		ownerHandle = deviceHandleFromCallbackArg(devicePtr)
	}
	// Always-on: soaks can see whether native delivered uncaptured into Go
	// (including non-lost validation) before GCT / submit paths.
	log.Printf("rwgpu: UncapturedCallback ENTER type=%d msg=%q devicePtr=%#x owner=%#x userdata1=%d",
		errType, msg, devicePtr, ownerHandle, userdata1)
	lastUncapturedMu.Lock()
	lastUncapturedTyp = ErrorType(errType)
	lastUncapturedMsg = msg
	lastUncapturedDevice = ownerHandle
	lastUncapturedMu.Unlock()
	if looksLikeDeviceLost(msg) {
		log.Printf("rwgpu: Uncaptured looksLikeDeviceLost type=%d msg=%q owner=%#x userdata1=%d",
			errType, msg, ownerHandle, userdata1)
		// Userdata first — which window/device owns this error.
		markDeviceLostFromCallback(devicePtr, userdata1)
		log.Printf("rwgpu: UncapturedCallback DONE sticky_lost=true owner=%#x", ownerHandle)
	}
	return 0
}

func initUncapturedErrorCallback() {
	uncapturedErrorCallbackPtr = purego.NewCallback(uncapturedErrorHandler)
}

// deviceLostEnterHook, when non-nil, is invoked at the start of deviceLostHandler.
// Tests use it to count native → Go DeviceLost deliveries. Production leaves nil.
var deviceLostEnterHook func()

// deviceLostHandler is WGPUDeviceLostCallback — primary writer of Device.lost.
// userdata1 is the per-device slot allocated at RequestDevice (multi-window safe).
// Always logs ENTER so soaks can see whether native delivered into Go before GCT abort.
func deviceLostHandler(devicePtr, reason, messageData, messageLength, userdata1, userdata2 uintptr) uintptr {
	_ = userdata2
	if deviceLostEnterHook != nil {
		deviceLostEnterHook()
	}
	msg := callbackStringView(messageData, messageLength)
	h := deviceHandleFromCallbackArg(devicePtr)
	log.Printf("rwgpu: DeviceLostCallback ENTER reason=%d msg=%q devicePtr=%#x handle=%#x userdata1=%d",
		reason, msg, devicePtr, h, userdata1)
	markDeviceLostFromCallback(devicePtr, userdata1)
	d := deviceFromUserdata(userdata1)
	lost := d != nil && d.IsLost()
	if !lost && h != 0 {
		lost = IsDeviceHandleLost(h)
	}
	log.Printf("rwgpu: DeviceLostCallback DONE sticky_lost=%v routed_device=%v", lost, d != nil)
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
			// lostCanary is allocated lazily on first SyncLostState (surface present
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

// deviceProbeResult is the outcome of a soft health probe before fatal surface ops.
type deviceProbeResult int

const (
	// deviceProbeAlive: soft ops succeeded; GetCurrentTexture may proceed.
	deviceProbeAlive deviceProbeResult = iota
	// deviceProbeLost: sticky lost set; refuse surface ops with ErrSurfaceDeviceLost.
	deviceProbeLost
	// deviceProbeUnavailable: cannot confirm health (OOM / null handles) — skip
	// frame without calling fatal GetCurrentTexture. Does not mark sticky lost
	// unless uncaptured already said so.
	deviceProbeUnavailable
)

// ensureLostCanaryLocked allocates a 4-byte COPY_DST buffer for soft probe.
// Caller must hold gpuMu. Best-effort: failure leaves lostCanary nil.
func (d *Device) ensureLostCanaryLocked() {
	if d == nil || d.handle == 0 || d.lostCanary != nil || d.IsLost() {
		return
	}
	if procDeviceCreateBuffer == nil {
		return
	}
	// COPY_BUFFER_ALIGNMENT is 4; WriteBuffer size must be a multiple of 4.
	wire := bufferDescriptorWire{
		Label:            stringToStringView("gpui-device-lost-canary"),
		Usage:            BufferUsageCopyDst,
		Size:             4,
		MappedAtCreation: False,
	}
	_, _ = LastUncapturedError()
	handle, _, _ := procDeviceCreateBuffer.Call(
		d.handle,
		uintptr(unsafe.Pointer(&wire)),
	)
	// Uncaptured may be async on some builds — pump before inspecting.
	pumpInstanceEvents(d)
	if handle == 0 {
		return
	}
	trackResource(handle, "Buffer")
	d.lostCanary = &Buffer{handle: handle, device: d, mapState: BufferMapStateUnmapped}
}

// absorbLostUncapturedLocked consumes a pending uncaptured lost message for this
// device and marks sticky lost. Returns true if the device is now lost.
func (d *Device) absorbLostUncapturedLocked() bool {
	if d == nil {
		return false
	}
	if d.IsLost() {
		return true
	}
	_, msg, owner := PeekLastUncapturedError()
	if !looksLikeDeviceLost(msg) {
		return d.IsLost()
	}
	// Only absorb when attributed to this device (or unknown owner).
	if owner != 0 && d.handle != 0 && owner != d.handle {
		return false
	}
	_, _ = LastUncapturedError()
	markDeviceObjectLost(d)
	return true
}

// dropLostCanaryLocked clears the canary Go handle (native skip if lost).
func (d *Device) dropLostCanaryLocked() {
	if d == nil || d.lostCanary == nil {
		return
	}
	c := d.lostCanary
	d.lostCanary = nil
	if d.IsLost() {
		c.handle = 0
		c.device = nil
		return
	}
	// Healthy drop uses native release (canary may be recreated next probe).
	releaseNativeHandle(&c.handle, false, func(h uintptr) {
		if procBufferDestroy != nil {
			procBufferDestroy.Call(h) //nolint:errcheck
		}
		if procBufferRelease != nil {
			procBufferRelease.Call(h) //nolint:errcheck
		}
	})
	c.device = nil
}

// probeDeviceForSurfaceLocked runs soft-only ops to detect parent-device-lost
// before fatal wgpuSurfaceGetCurrentTexture. Caller must hold gpuMu.
//
// Fail-closed: PopErrorScope failure / missing canary → unavailable (never Alive).
// Soft probes can still false-negative under occlusion TDR; GetCurrentTexture uses
// a SIGABRT shield (gct_guard_linux.c) as the last line of defense.
func (d *Device) probeDeviceForSurfaceLocked() deviceProbeResult {
	if d == nil || d.handle == 0 {
		return deviceProbeUnavailable
	}
	if d.IsLost() {
		return deviceProbeLost
	}
	if procDeviceGetQueue == nil || procQueueWriteBuffer == nil {
		return deviceProbeUnavailable
	}

	pumpInstanceEvents(d)
	if d.IsLost() || d.absorbLostUncapturedLocked() {
		return deviceProbeLost
	}

	// Lazy canary (not per-frame CreateBuffer — that path stressed the 1GB GPU).
	d.ensureLostCanaryLocked()
	if d.IsLost() || d.absorbLostUncapturedLocked() {
		d.dropLostCanaryLocked()
		return deviceProbeLost
	}
	if d.lostCanary == nil || d.lostCanary.handle == 0 {
		return deviceProbeUnavailable
	}

	qh, _, _ := procDeviceGetQueue.Call(d.handle)
	pumpInstanceEvents(d)
	if d.IsLost() || d.absorbLostUncapturedLocked() {
		if qh != 0 {
			procQueueRelease.Call(qh) //nolint:errcheck
		}
		return deviceProbeLost
	}
	if qh == 0 {
		markDeviceObjectLost(d)
		return deviceProbeLost
	}

	pushedScope := false
	if procDevicePushErrorScope != nil {
		procDevicePushErrorScope.Call(d.handle, uintptr(ErrorFilterValidation)) //nolint:errcheck
		pushedScope = true
	}

	_, _ = LastUncapturedError()
	var probe [4]byte
	call5(procQueueWriteBuffer, qh, d.lostCanary.handle, 0,
		uintptr(unsafe.Pointer(&probe[0])), 4)
	procQueueRelease.Call(qh) //nolint:errcheck

	pumpInstanceEvents(d)
	if procDevicePoll != nil {
		procDevicePoll.Call(d.handle, 0, 0)
		pumpInstanceEvents(d)
	}

	if pushedScope && procDevicePopErrorScope != nil && d.instance != 0 {
		errType, msg, err := d.popErrorScopeLocked()
		if err != nil {
			// Fail-closed: never treat pop failure as Alive (prior SIGABRT path).
			return deviceProbeUnavailable
		}
		if looksLikeDeviceLost(msg) {
			markDeviceObjectLost(d)
			d.dropLostCanaryLocked()
			return deviceProbeLost
		}
		if errType != ErrorTypeNoError && errType != 0 && msg != "" {
			d.dropLostCanaryLocked()
			// Transient validation — skip GCT this frame.
			return deviceProbeUnavailable
		}
	} else if pushedScope {
		return deviceProbeUnavailable
	}

	if d.IsLost() || d.absorbLostUncapturedLocked() {
		d.dropLostCanaryLocked()
		return deviceProbeLost
	}
	return deviceProbeAlive
}

// popErrorScopeLocked pops one error scope while gpuMu is already held.
// Uses WaitAnyOnly; does not take gpuMu again.
func (d *Device) popErrorScopeLocked() (ErrorType, string, error) {
	if d == nil || d.handle == 0 || d.instance == 0 {
		return ErrorTypeNoError, "", &WGPUError{Op: "popErrorScopeLocked", Message: "device/instance unavailable"}
	}
	errorScopeCallbackOnce.Do(initErrorScopeCallback)
	result := &errorScopeResult{done: make(chan struct{})}
	errorScopeResultsMu.Lock()
	errorScopeResultID++
	resultID := errorScopeResultID
	errorScopeResults[resultID] = result
	errorScopeResultsMu.Unlock()

	callbackInfo := popErrorScopeCallbackInfo{
		mode:      CallbackModeWaitAnyOnly,
		callback:  errorScopeCallbackPtr,
		userdata1: resultID,
	}
	future, err := callDevicePopErrorScope(d.handle, &callbackInfo)
	if err != nil {
		errorScopeResultsMu.Lock()
		delete(errorScopeResults, resultID)
		errorScopeResultsMu.Unlock()
		return ErrorTypeNoError, "", err
	}
	if err := waitForFuture(d.instance, future, "popErrorScopeLocked"); err != nil {
		errorScopeResultsMu.Lock()
		delete(errorScopeResults, resultID)
		errorScopeResultsMu.Unlock()
		return ErrorTypeNoError, "", err
	}
	select {
	case <-result.done:
		if result.status != PopErrorScopeStatusSuccess {
			return ErrorTypeNoError, "", &WGPUError{
				Op:      "popErrorScopeLocked",
				Message: fmt.Sprintf("pop status %d", result.status),
			}
		}
		return result.errType, result.message, nil
	default:
		return ErrorTypeNoError, "", &WGPUError{Op: "popErrorScopeLocked", Message: "callback not invoked"}
	}
}

// SyncLostState pumps DeviceLost callbacks and runs the soft probe canary.
// On this libwgpu_native.so, DeviceLost often never enters Go and
// GetCurrentTexture aborts if parent is already lost — the probe converts that
// into sticky IsLost before surface acquire.
//
// Safe and idempotent on nil / already-lost devices. Must not false-positive a
// healthy device (covered by TestSyncLostState_IdempotentOnHealthyDevice).
func (d *Device) SyncLostState() {
	if d == nil || d.IsLost() {
		return
	}
	if checkInit() != nil {
		return
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	_ = d.probeDeviceForSurfaceLocked()
}

// syncLostStateLocked is kept for call sites that only need sticky update.
func (d *Device) syncLostStateLocked() {
	_ = d.probeDeviceForSurfaceLocked()
}

// Destroy forces native device teardown and sticky-lost state (Skia/Flutter
// abandon-context equivalent for tests and intentional reclaim).
//
// This .so often does not deliver DeviceLostCallback after Destroy; sticky is
// force-marked so public APIs refuse with ErrDeviceLost and surface acquire
// never reaches fatal GetCurrentTexture. Idempotent and nil-safe.
func (d *Device) Destroy() {
	if d == nil {
		return
	}
	// Free canary with native release while device is still healthy (before
	// sticky-lost). Releasing after MarkLost skips BufferRelease and leaks VRAM,
	// which causes subsequent RequestDevice "Not enough memory left" in tests.
	if d.lostCanary != nil {
		c := d.lostCanary
		d.lostCanary = nil
		if !d.IsLost() {
			c.Release()
		} else {
			// Already lost: clear Go handle only.
			c.handle = 0
			c.device = nil
		}
	}

	// Sticky so concurrent GetCurrentTexture refuses immediately.
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
// Nil-safe and idempotent. When the device is already lost, only Go-side
// state is cleared — native DeviceRelease on a lost device can SIGABRT.
func (d *Device) Release() {
	if d == nil {
		return
	}
	lost := d.IsLost()
	if d.lostCanary != nil {
		d.lostCanary.Release()
		d.lostCanary = nil
	}
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
func pumpInstanceEvents(d *Device) {
	if d == nil || d.instance == 0 || procInstanceProcessEvents == nil {
		return
	}
	procInstanceProcessEvents.Call(d.instance) //nolint:errcheck
}
