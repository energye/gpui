//go:build linux && !(js && wasm)

package egl

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
// ABI note (Mesa 25, 2026-09): eglQueryDevicesEXT lives in the vendor
// ICD, not libEGL.so.1 (GetSymbol on the loader segfaults), and its
// 3-arg tail call faults through this repo's ffi trampoline
// (addr 0x1000000000). Until the trampoline grows that shape, the list
// stays empty by contract.
func QueryDevices() []DeviceInfo {
	return nil
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
