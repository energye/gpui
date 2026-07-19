package rwgpu

import (
	"errors"
	"strings"
	"sync"
)

// gpuMu serializes Device / Surface native entry points that are unsafe for
// concurrent use. wgpu-native device and surface objects are not free-threaded;
// concurrent Configure / GetCurrentTexture / Present can corrupt state and abort.
var gpuMu sync.Mutex

// LockGPU acquires the process-global GPU serialization lock.
// Prefer WithGPU for scoped use. Must be paired with UnlockGPU.
func LockGPU() { gpuMu.Lock() }

// UnlockGPU releases the process-global GPU serialization lock.
func UnlockGPU() { gpuMu.Unlock() }

// WithGPU runs fn while holding the global GPU lock.
func WithGPU(fn func()) {
	gpuMu.Lock()
	defer gpuMu.Unlock()
	fn()
}

// ErrorClass is a structured classification of GPU operation failures.
// Prefer errors.Is against sentinels; ClassifyError is for grading
// (retry vs fatal) without ad-hoc string matching.
type ErrorClass int

const (
	// ErrorClassUnknown is an unclassified or opaque error.
	ErrorClassUnknown ErrorClass = iota
	// ErrorClassDeviceLost is a sticky, fatal device failure.
	ErrorClassDeviceLost
	// ErrorClassInvalidHandle is a nil/released/zero native handle.
	ErrorClassInvalidHandle
	// ErrorClassSurfaceInvalid is surface lost/outdated/unconfigured/occluded.
	ErrorClassSurfaceInvalid
	// ErrorClassOutOfMemory is GPU VRAM/system memory pressure.
	ErrorClassOutOfMemory
	// ErrorClassValidation is a WebGPU validation error.
	ErrorClassValidation
)

// String returns a short name for the error class.
func (c ErrorClass) String() string {
	switch c {
	case ErrorClassDeviceLost:
		return "device_lost"
	case ErrorClassInvalidHandle:
		return "invalid_handle"
	case ErrorClassSurfaceInvalid:
		return "surface_invalid"
	case ErrorClassOutOfMemory:
		return "out_of_memory"
	case ErrorClassValidation:
		return "validation"
	default:
		return "unknown"
	}
}

// Structured sentinels (in addition to ErrDeviceLost / ErrSurface* / ErrOutOfMemory).
var (
	// ErrInvalidHandle is returned when a nil, released, or zero handle is used.
	ErrInvalidHandle = &WGPUError{Type: ErrorTypeUnknown, Message: "invalid handle"}
	// ErrSurfaceInvalid is a generic surface-not-usable sentinel (lost/outdated/nil).
	ErrSurfaceInvalid = &WGPUError{Type: ErrorTypeUnknown, Message: "surface invalid"}
)

// ClassifyError maps err to a structured ErrorClass using sentinel matching
// first, then conservative message fallbacks for wrapped native text.
func ClassifyError(err error) ErrorClass {
	if err == nil {
		return ErrorClassUnknown
	}
	if errors.Is(err, ErrDeviceLost) || errors.Is(err, ErrSurfaceDeviceLost) {
		return ErrorClassDeviceLost
	}
	if errors.Is(err, ErrOutOfMemory) || errors.Is(err, ErrSurfaceOutOfMemory) {
		return ErrorClassOutOfMemory
	}
	if errors.Is(err, ErrInvalidHandle) {
		return ErrorClassInvalidHandle
	}
	if errors.Is(err, ErrSurfaceLost) || errors.Is(err, ErrSurfaceNeedsReconfigure) ||
		errors.Is(err, ErrSurfaceOccluded) || errors.Is(err, ErrSurfaceTimeout) ||
		errors.Is(err, ErrSurfaceInvalid) {
		return ErrorClassSurfaceInvalid
	}
	if errors.Is(err, ErrValidation) {
		return ErrorClassValidation
	}
	var we *WGPUError
	if errors.As(err, &we) {
		switch we.Type {
		case ErrorTypeOutOfMemory:
			return ErrorClassOutOfMemory
		case ErrorTypeValidation:
			if looksLikeDeviceLost(we.Message) {
				return ErrorClassDeviceLost
			}
			if looksLikeInvalidHandle(we.Message) {
				return ErrorClassInvalidHandle
			}
			return ErrorClassValidation
		}
		if looksLikeDeviceLost(we.Message) {
			return ErrorClassDeviceLost
		}
		if looksLikeInvalidHandle(we.Message) {
			return ErrorClassInvalidHandle
		}
		if looksLikeSurfaceInvalid(we.Message) {
			return ErrorClassSurfaceInvalid
		}
		if looksLikeOOM(we.Message) {
			return ErrorClassOutOfMemory
		}
	}
	msg := err.Error()
	if looksLikeDeviceLost(msg) {
		return ErrorClassDeviceLost
	}
	if looksLikeInvalidHandle(msg) {
		return ErrorClassInvalidHandle
	}
	if looksLikeSurfaceInvalid(msg) {
		return ErrorClassSurfaceInvalid
	}
	if looksLikeOOM(msg) {
		return ErrorClassOutOfMemory
	}
	return ErrorClassUnknown
}

func looksLikeInvalidHandle(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "invalid handle") ||
		strings.Contains(lower, "nil or released") ||
		strings.Contains(lower, "is nil") ||
		strings.Contains(lower, "null handle") ||
		strings.Contains(lower, "already released") ||
		strings.Contains(lower, "resource already released")
}

func looksLikeSurfaceInvalid(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "surface lost") ||
		strings.Contains(lower, "surface outdated") ||
		strings.Contains(lower, "needs reconfigure") ||
		strings.Contains(lower, "surface occluded") ||
		strings.Contains(lower, "surface invalid") ||
		strings.Contains(lower, "not configured")
}

func looksLikeOOM(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "out of memory") ||
		strings.Contains(lower, "not enough memory")
}

// refuseIfLost returns ErrDeviceLost when the device handle's Device.lost is set.
func refuseIfLost(op string, deviceHandle uintptr) error {
	_ = op
	if deviceHandle != 0 && IsDeviceHandleLost(deviceHandle) {
		return ErrDeviceLost
	}
	return nil
}

// gateDevice validates device handle and per-handle lost state before purego Call.
// Does not load the native library — safe after device-lost without init.
func gateDevice(op string, d *Device) error {
	if d == nil || d.handle == 0 {
		return &WGPUError{Op: op, Message: "device is nil or released"}
	}
	return refuseIfLost(op, d.handle)
}

// gateQueue validates queue handle and parent device lost state.
func gateQueue(op string, q *Queue) error {
	if q == nil || q.handle == 0 {
		return &WGPUError{Op: op, Message: "queue is nil or released"}
	}
	return refuseIfLost(op, q.device)
}

// prepareDeviceCall is gateDevice + checkInit for error-returning Device methods.
func prepareDeviceCall(op string, d *Device) error {
	if err := gateDevice(op, d); err != nil {
		return err
	}
	return checkInit()
}

// prepareQueueCall is gateQueue + checkInit for error-returning Queue methods.
func prepareQueueCall(op string, q *Queue) error {
	if err := gateQueue(op, q); err != nil {
		return err
	}
	return checkInit()
}

// nativeCallHook, when non-nil, is invoked immediately before a native
// release/destroy/unconfigure Call. Tests use it to assert skip-on-lost.
// Production code leaves this nil.
var nativeCallHook func(kind string)

// isOwnerDeviceLost reports whether the parent device handle is sticky-lost.
func isOwnerDeviceLost(deviceHandle uintptr) bool {
	return deviceHandle != 0 && IsDeviceHandleLost(deviceHandle)
}

// releaseNativeHandle clears *handle (nil/zero-safe, idempotent), untracks the
// resource, and calls nativeRelease unless lost is true. When the parent device
// is lost, only Go-side state is cleared — native release on a lost device can
// SIGABRT inside wgpu-native.
func releaseNativeHandle(handle *uintptr, lost bool, nativeRelease func(h uintptr)) {
	if handle == nil || *handle == 0 {
		return
	}
	h := *handle
	*handle = 0
	untrackResource(h)
	if lost {
		return
	}
	if nativeCallHook != nil {
		nativeCallHook("release")
	}
	if nativeRelease != nil {
		nativeRelease(h)
	}
}

// destroyAndReleaseNativeHandle is releaseNativeHandle plus an optional native
// Destroy before Release. When lost, both native calls are skipped.
func destroyAndReleaseNativeHandle(handle *uintptr, lost bool, destroy, release func(h uintptr)) {
	if handle == nil || *handle == 0 {
		return
	}
	h := *handle
	*handle = 0
	untrackResource(h)
	if lost {
		return
	}
	if nativeCallHook != nil {
		nativeCallHook("destroy")
	}
	if destroy != nil {
		destroy(h)
	}
	if release != nil {
		release(h)
	}
}

// unconfigureNativeHandle runs native unconfigure unless lost. Does not clear
// the surface handle (Unconfigure leaves the Surface object alive).
func unconfigureNativeHandle(surfaceHandle uintptr, lost bool, nativeUnconfigure func(h uintptr)) {
	if surfaceHandle == 0 || lost {
		return
	}
	if nativeCallHook != nil {
		nativeCallHook("unconfigure")
	}
	if nativeUnconfigure != nil {
		nativeUnconfigure(surfaceHandle)
	}
}
