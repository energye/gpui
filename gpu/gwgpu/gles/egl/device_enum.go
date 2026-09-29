//go:build linux && !(js && wasm)

package egl

import (
	"strings"
	"unsafe"

	ffi "github.com/energye/gpui/gpu/gwgpu/ffishim"
	types "github.com/energye/gpui/gpu/gwgpu/ffishim"
)

// Device enumeration (EGL_EXT_device_enumeration +
// EGL_EXT_device_query + EGL_EXT_platform_device): lets callers name a
// GPU before opening a display, the same "enumerate then pick" shape as
// the WebGPU RequestAdapter path. Visible-window present still requires
// a display the driver accepts (on this Xwayland box the NVIDIA EGL
// driver refuses Xwayland pixmaps, so high-power GL windows stay
// unreachable); offscreen/surfaceless work runs on the picked device.

// PlatformDeviceEXT is EGL_PLATFORM_DEVICE_EXT (0x313F).
const PlatformDeviceEXT EGLEnum = 0x313F

// DeviceInfo describes one EGL device.
type DeviceInfo struct {
	// Device is the opaque EGLDeviceEXT handle.
	Device uintptr
	// Name is the driver-reported device string (may be empty).
	Name string
	// IsNVIDIA reports an NVIDIA vendor/renderer string.
	IsNVIDIA bool
	// IsMesa reports a Mesa/Intel/AMD string (integrated on this box).
	IsMesa bool
}

// DeviceEnumerationSupported reports whether the loader exposes device
// enumeration (client extensions). Presence only — see QueryDevices.
func DeviceEnumerationSupported() bool {
	for _, ext := range splitExtensions(QueryString(NoDisplay, Extensions)) {
		if ext == "EGL_EXT_device_enumeration" {
			return true
		}
	}
	return false
}

// FindDiscreteDevice returns the first NVIDIA device, or 0 when
// enumeration is unavailable or none matches. Visible windows still
// present through the platform display; the picked device drives
// offscreen/surfaceless work.
func FindDiscreteDevice() uintptr {
	for _, d := range QueryDevices() {
		if d.IsNVIDIA {
			return d.Device
		}
	}
	return 0
}

// PlatformDisplayForDevice opens an EGLDisplay on an enumerated device
// (EGL_PLATFORM_DEVICE_EXT). 0/empty device falls back to NoDisplay so
// callers keep the platform path.
func PlatformDisplayForDevice(dev uintptr) EGLDisplay {
	if dev == 0 || symEglGetPlatformDisplay == nil {
		return NoDisplay
	}
	return GetPlatformDisplay(PlatformDeviceEXT, dev, nil)
}

// QueryDevices lists EGL devices. Empty (not an error) on drivers where
// enumeration is unsupported or the call cannot marshal (see ABI note).
// No process may die probing GPUs; falling back to the platform display
// is the correct answer on such drivers.
//
// ABI note: eglQueryDevicesEXT resolves via eglGetProcAddress (GetSymbol
// on the loader segfaults — 2026-09 regression), and its
// (EGLint, EGLDevice*, EGLint*) triple must go through the ffishim
// RegisterFunc trampoline (CallTrampoline): the SyscallN/cgocall path
// carries stale al/XMM state that this loader dispatch reads as the
// count register (ok=0 under SyscallN, ok=1 under trampoline, same bytes
// — verified 2026-09). The trampoline shape is covered by the ffishim
// three-arg shape test; if it ever goes red, this stays empty.
func QueryDevices() []DeviceInfo {
	queryAddr := deviceProcResolver("eglQueryDevicesEXT")
	if queryAddr == 0 {
		return nil
	}
	var cifQuery types.CallInterface
	if err := ffi.PrepareCallInterface(&cifQuery, types.DefaultCall,
		types.UInt32TypeDescriptor, // EGLBoolean
		[]*types.TypeDescriptor{
			types.UInt32TypeDescriptor,  // EGLint max_devices
			types.PointerTypeDescriptor, // EGLDeviceEXT *devices
			types.PointerTypeDescriptor, // EGLint *num_devices
		}); err != nil {
		return nil
	}
	queryFn := u2p(queryAddr)
	// avalue holds pointers to argument values (see Initialize): the two
	// pointer args travel as uintptr slots holding the address (or 0).
	callQuery := func(max uint32, devsPtr, numPtr uintptr) (uint32, error) {
		var ok uint32
		devArg := devsPtr
		numArg := numPtr
		err := ffi.CallTrampoline(&cifQuery, queryFn, unsafe.Pointer(&ok),
			[]unsafe.Pointer{unsafe.Pointer(&max), unsafe.Pointer(&devArg), unsafe.Pointer(&numArg)})
		return ok, err
	}
	var num int32
	maxZero := uint32(0)
	ok, err := callQuery(maxZero, 0, uintptr(unsafe.Pointer(&num)))
	if err != nil {
		return nil
	}
	if ok == 0 || num <= 0 {
		return nil
	}
	if num > maxEnumDevices {
		num = maxEnumDevices
	}
	devs := make([]uintptr, num)
	max := uint32(num)
	ok = 0
	num = 0
	if ok, err = callQuery(max, uintptr(unsafe.Pointer(&devs[0])), uintptr(unsafe.Pointer(&num))); err != nil {
		return nil
	}
	if ok == 0 || num <= 0 {
		return nil
	}
	out := make([]DeviceInfo, 0, num)
	for _, d := range devs[:num] {
		if d == 0 {
			continue
		}
		out = append(out, classifyDevice(d))
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// maxEnumDevices caps the enumeration buffer: drivers report a handful
// of GPUs; an absurd count means the call mis-marshalled.
const maxEnumDevices = 16

// drmDeviceFileEXT is EGL_DRM_DEVICE_FILE_EXT (0x3233): per-device DRM
// node path (e.g. /dev/dri/card0), the stable identity string.
const drmDeviceFileEXT EGLInt = 0x3233

// deviceProcResolver resolves extension entry points. A variable (not a
// direct call) so tests can force the proc-missing path without hardware.
var deviceProcResolver = GetProcAddress

// classifyDevice names one device and marks its vendor family. The name
// prefers the DRM node path; classification reads per-device extension
// markers. Every string query is fail-soft: a device with no readable
// strings still counts (handle valid, names empty).
func classifyDevice(dev uintptr) DeviceInfo {
	info := DeviceInfo{Device: dev}
	strAddr := deviceProcResolver("eglQueryDeviceStringEXT")
	if strAddr == 0 {
		return info
	}
	info.Name = queryDeviceString(strAddr, dev, drmDeviceFileEXT)
	exts := queryDeviceString(strAddr, dev, Extensions)
	lower := strings.ToLower(exts + " " + info.Name)
	switch {
	case strings.Contains(lower, "nvidia") || strings.Contains(lower, "nv_device_cuda"):
		info.IsNVIDIA = true
	case strings.Contains(lower, "mesa") || strings.Contains(lower, "dri") ||
		strings.Contains(lower, "intel") || strings.Contains(lower, "amd") ||
		strings.Contains(lower, "radeon"):
		info.IsMesa = true
	}
	return info
}

// queryDeviceString calls eglQueryDeviceStringEXT(device, name) and
// returns "" when the address is nil, the call fails, or the driver
// returns NULL.
func queryDeviceString(strAddr uintptr, dev uintptr, name EGLInt) string {
	var cif types.CallInterface
	if err := ffi.PrepareCallInterface(&cif, types.DefaultCall,
		types.PointerTypeDescriptor, // const char *
		[]*types.TypeDescriptor{
			types.PointerTypeDescriptor, // EGLDeviceEXT device
			types.UInt32TypeDescriptor,  // EGLint name
		}); err != nil {
		return ""
	}
	devArg := dev
	nameArg := uint32(name)
	var cstr uintptr
	if err := ffi.CallTrampoline(&cif, u2p(strAddr), unsafe.Pointer(&cstr),
		[]unsafe.Pointer{unsafe.Pointer(&devArg), unsafe.Pointer(&nameArg)}); err != nil {
		return ""
	}
	if cstr == 0 {
		return ""
	}
	return goString(cstr)
}

func splitExtensions(s string) []string {
	var out []string
	start := -1
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ' ' || s[i] == '\t' || s[i] == '\n' {
			if start >= 0 {
				out = append(out, s[start:i])
				start = -1
			}
		} else if start < 0 {
			start = i
		}
	}
	return out
}
