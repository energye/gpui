package rwgpu

import (
	"unsafe"

	"github.com/energye/gpui/gpu/types"
)

// surfaceDescriptor is the native structure for surface creation.
type surfaceDescriptor struct {
	nextInChain uintptr    // Pointer to platform-specific source
	label       StringView // 16 bytes
}

// surfaceConfigurationWire is the FFI-compatible structure for configuring a surface.
// Uses uint32 for format (converted from gputypes) and uint64 for usage.
type surfaceConfigurationWire struct {
	nextInChain     uintptr // 8 bytes
	device          uintptr // 8 bytes (WGPUDevice handle)
	format          uint32  // 4 bytes (converted from gputypes.TextureFormat)
	_pad1           [4]byte // 4 bytes padding
	usage           uint64  // 8 bytes (TextureUsage as uint64)
	width           uint32  // 4 bytes
	height          uint32  // 4 bytes
	viewFormatCount uintptr // 8 bytes (size_t)
	viewFormats     uintptr // 8 bytes (pointer)
	alphaMode       uint32  // 4 bytes (CompositeAlphaMode)
	presentMode     uint32  // 4 bytes (PresentMode)
}

// surfaceTexture is the native structure returned by GetCurrentTexture.
type surfaceTexture struct {
	nextInChain uintptr                        // 8 bytes
	texture     uintptr                        // 8 bytes (WGPUTexture)
	status      SurfaceGetCurrentTextureStatus // 4 bytes
	_pad        [4]byte                        // 4 bytes padding
}

// surfaceCapabilitiesWire is the FFI-compatible structure for WGPUSurfaceCapabilities.
// Matches C struct layout from wgpu-native v27.
type surfaceCapabilitiesWire struct {
	nextInChain      uintptr // 8 bytes (WGPUChainedStructOut*)
	usages           uint64  // 8 bytes (WGPUTextureUsage bitflags)
	formatCount      uintptr // 8 bytes (size_t)
	formats          uintptr // 8 bytes (WGPUTextureFormat* - pointer to array)
	presentModeCount uintptr // 8 bytes (size_t)
	presentModes     uintptr // 8 bytes (WGPUPresentMode* - pointer to array)
	alphaModeCount   uintptr // 8 bytes (size_t)
	alphaModes       uintptr // 8 bytes (WGPUCompositeAlphaMode* - pointer to array)
}

// SurfaceConfiguration describes how to configure a surface.
// Note: the Device field is deprecated — pass the device as a separate argument to Configure.
// It remains here for backward compatibility; if non-nil it takes precedence over the explicit arg.
type SurfaceConfiguration struct {
	// Device is deprecated: pass the device to Configure() directly instead.
	// Kept for backward compatibility. If non-nil, overrides the explicit device argument.
	Device      *Device
	Format      types.TextureFormat
	Usage       types.TextureUsage
	Width       uint32
	Height      uint32
	AlphaMode   types.CompositeAlphaMode
	PresentMode types.PresentMode
}

// SurfaceTexture holds the result of GetCurrentTexture.
type SurfaceTexture struct {
	Texture *Texture
	Status  SurfaceGetCurrentTextureStatus
}

// SurfaceCapabilities describes the capabilities of a surface for presentation.
// Returned by Surface.GetCapabilities() to query supported formats, present modes, etc.
type SurfaceCapabilities struct {
	Usages       types.TextureUsage
	Formats      []types.TextureFormat
	PresentModes []types.PresentMode
	AlphaModes   []types.CompositeAlphaMode
}

// Error values for surface operations.
// These are sentinel errors for programmatic error handling via errors.Is().
var (
	ErrSurfaceNeedsReconfigure = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface needs reconfigure"}
	ErrSurfaceLost             = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface lost"}
	ErrSurfaceTimeout          = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface texture timeout"}
	// ErrSurfaceOccluded is returned on macOS Metal when the window is minimized or fully covered.
	// Applications should skip rendering for the current frame and try again when unoccluded.
	// New in wgpu-native v29.
	ErrSurfaceOccluded = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface occluded (window minimized or covered)"}
	// ErrSurfaceOutOfMemory is kept for backward compatibility.
	// Deprecated: In v29 this is reported as generic Error status.
	ErrSurfaceOutOfMemory = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "out of memory"}
	// ErrSurfaceDeviceLost is kept for backward compatibility.
	// Deprecated: In v29 this is reported as generic Error status.
	ErrSurfaceDeviceLost = &WGPUError{Op: "Surface.GetCurrentTexture", Message: "device lost"}
)

// Configure configures the surface for rendering.
// The device argument specifies which logical device to use for the surface.
// If config.Device is also set (deprecated usage), it takes precedence over the device arg.
// Returns nil on success. Soft-error native reports failures via Device ErrorSink
// (Uncaptured/DeviceLost); this method folds them into the returned error.
// Enum values are converted from gputypes to wgpu-native values before FFI call.
func (s *Surface) Configure(device *Device, config *SurfaceConfiguration) error {
	if s == nil || s.handle == 0 {
		return &WGPUError{Op: "Surface.Configure", Message: "surface is nil or released"}
	}
	if config == nil {
		return &WGPUError{Op: "Surface.Configure", Message: "configuration is nil"}
	}
	if config.Width == 0 || config.Height == 0 {
		return &WGPUError{Op: "Surface.Configure", Message: "extent must be non-zero"}
	}

	// config.Device takes precedence (backward compat) over the device argument.
	dev := device
	if config.Device != nil {
		dev = config.Device
	}
	if dev == nil || dev.handle == 0 {
		return &WGPUError{Op: "Surface.Configure", Message: "device is nil or released"}
	}

	// Refuse Configure after sticky device-lost.
	if err := refuseIfLost("Surface.Configure", dev.handle); err != nil {
		s.abandoned = true
		return err
	}
	if dev.IsLost() {
		s.abandoned = true
		return ErrDeviceLost
	}
	if _, msg, owner := PeekLastUncapturedError(); looksLikeDeviceLost(msg) {
		// Only refuse if this device owns the sticky message (or owner unknown).
		if owner == 0 || owner == dev.handle {
			noteLostMessage(dev.handle, msg)
			_, _ = LastUncapturedError()
			s.abandoned = true
			return ErrDeviceLost
		}
	}
	if err := checkInit(); err != nil {
		return err
	}

	gpuMu.Lock()
	defer gpuMu.Unlock()

	nativeConfig := surfaceConfigurationWire{
		nextInChain:     0,
		device:          dev.handle,
		format:          uint32(config.Format),
		usage:           uint64(config.Usage),
		width:           config.Width,
		height:          config.Height,
		viewFormatCount: 0,
		viewFormats:     0,
		alphaMode:       uint32(config.AlphaMode),
		presentMode:     uint32(config.PresentMode),
	}

	_, _ = LastUncapturedError() // attribute post-call errors to this op
	procSurfaceConfigure.Call(   //nolint:errcheck
		s.handle,
		uintptr(unsafe.Pointer(&nativeConfig)),
	)
	if typ, msg := LastUncapturedError(); msg != "" {
		if looksLikeDeviceLost(msg) {
			noteLostMessage(dev.handle, msg)
			s.abandoned = true
			return ErrDeviceLost
		}
		return &WGPUError{Op: "Surface.Configure", Type: typ, Message: msg}
	}
	s.device = dev.handle
	s.deviceRef = dev
	// Healthy configure clears abandon so a recovered device can present again.
	s.abandoned = false
	return nil
}

// ConfigureLegacy configures the surface using only the config struct (legacy API).
// Deprecated: use Configure(device, config) instead.
func (s *Surface) ConfigureLegacy(config *SurfaceConfiguration) {
	_ = s.Configure(nil, config)
}

// Unconfigure removes the surface configuration.
// Defense order: nil/zero handle → abandoned/lost (skip native) → init → native.
// Nil-safe, zero-handle-safe, and idempotent w.r.t. Go-side device binding.
func (s *Surface) Unconfigure() {
	if s == nil || s.handle == 0 {
		return
	}
	lost := s.abandoned || isOwnerDeviceLost(s.device) || (s.deviceRef != nil && s.deviceRef.IsLost())
	if lost {
		s.abandoned = true
	}
	s.device = 0
	s.deviceRef = nil
	// Skip native Unconfigure when lost: force-Unconfigure + Configure on the
	// *same* surface SIGSEGVs in this libwgpu_native. Recover recreates Surface.
	if lost {
		return
	}
	if checkInit() != nil {
		return
	}
	unconfigureNativeHandle(s.handle, false, func(h uintptr) {
		procSurfaceUnconfigure.Call(h) //nolint:errcheck
	})
}

// surfaceParentLost reports whether this surface's parent device is sticky-lost
// and absorbs pending uncaptured "Parent device is lost" for THIS device only.
// Multi-window: uncaptured from device A must not poison surface on device B.
// Must run before any purego surface call that aborts when parent is lost.
// On true, also sets s.abandoned so later Unconfigure/Release skip native.
func (s *Surface) surfaceParentLost() bool {
	if s == nil {
		return false
	}
	if s.abandoned {
		return true
	}
	if s.deviceRef != nil && s.deviceRef.IsLost() {
		s.abandoned = true
		return true
	}
	if isOwnerDeviceLost(s.device) {
		s.abandoned = true
		return true
	}
	// Pending uncaptured — only absorb if attributed to this surface's device.
	_, msg, owner := PeekLastUncapturedError()
	if !looksLikeDeviceLost(msg) {
		return false
	}
	// owner==0: unknown attribution; only apply if we have a parent handle match
	// via consume+mark of this surface's device alone (never mark others).
	if owner != 0 && s.device != 0 && owner != s.device {
		return false // message belongs to another window's device
	}
	_, _ = LastUncapturedError() // consume sticky slot
	if s.deviceRef != nil {
		s.deviceRef.MarkLost()
	} else if s.device != 0 {
		markDeviceLost(s.device)
	}
	s.abandoned = true
	return true
}

// GetCurrentTexture gets the current texture to render to.
// Returns the texture, a suboptimal flag (true if the surface needs reconfiguration
// but is still usable this frame), and any error. This matches the gogpu/wgpu API.
//
// Soft-error native (patched rwgpu): parent-device-lost and not-configured return
// status=Error (and may fire Uncaptured/DeviceLost). Go maps status + uncaptured
// into structured errors. Still refuse native when sticky-lost / probe fail-closed.
func (s *Surface) GetCurrentTexture() (*SurfaceTexture, bool, error) {
	if s == nil || s.handle == 0 {
		return nil, false, &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface is nil or released"}
	}
	if s.surfaceParentLost() {
		return nil, false, ErrSurfaceDeviceLost
	}
	if err := checkInit(); err != nil {
		return nil, false, err
	}

	gpuMu.Lock()
	defer gpuMu.Unlock()

	// Skia model: sticky lost / pending uncaptured → refuse; else call native
	// (soft .so returns status/error, does not SIGABRT).
	if s.deviceRef != nil {
		pumpInstanceEvents(s.deviceRef)
		if s.deviceRef.IsLost() || s.deviceRef.absorbLostUncapturedLocked() {
			s.abandoned = true
			return nil, false, ErrSurfaceDeviceLost
		}
	} else if s.device != 0 && isOwnerDeviceLost(s.device) {
		s.abandoned = true
		return nil, false, ErrSurfaceDeviceLost
	}
	if s.surfaceParentLost() {
		return nil, false, ErrSurfaceDeviceLost
	}

	var surfTex surfaceTexture
	_, _ = LastUncapturedError()       // attribute post-call errors to this acquire
	procSurfaceGetCurrentTexture.Call( //nolint:errcheck
		s.handle,
		uintptr(unsafe.Pointer(&surfTex)),
	)

	// Soft native ErrorSink may fire DeviceLostCallback and/or Uncaptured during Call.
	// DeviceLost goes to DeviceLost handler (sticky IsLost), not always LastUncapturedError.
	uncapturedTyp, uncapturedMsg := LastUncapturedError()
	if (s.deviceRef != nil && s.deviceRef.IsLost()) || isOwnerDeviceLost(s.device) ||
		(uncapturedMsg != "" && looksLikeDeviceLost(uncapturedMsg)) {
		if uncapturedMsg != "" && looksLikeDeviceLost(uncapturedMsg) {
			noteLostMessage(s.device, uncapturedMsg)
		}
		s.abandoned = true
		if surfTex.texture != 0 {
			h := surfTex.texture
			releaseNativeHandle(&h, true, nil)
			surfTex.texture = 0
		}
		return nil, false, ErrSurfaceDeviceLost
	}

	result := &SurfaceTexture{
		Texture: &Texture{handle: surfTex.texture, device: s.device},
		Status:  surfTex.status,
	}

	// On non-success statuses, native may still fill a texture pointer.
	// That SurfaceOutput MUST be dropped (Release) before Configure/re-acquire.
	dropTex := func() {
		if result.Texture != nil && result.Texture.handle != 0 {
			result.Texture.Release()
			result.Texture = nil
		}
	}

	switch surfTex.status {
	case SurfaceGetCurrentTextureStatusSuccessOptimal:
		return result, false, nil
	case SurfaceGetCurrentTextureStatusSuccessSuboptimal:
		return result, true, nil
	case SurfaceGetCurrentTextureStatusOutdated:
		dropTex()
		return nil, false, ErrSurfaceNeedsReconfigure
	case SurfaceGetCurrentTextureStatusLost:
		dropTex()
		return nil, false, ErrSurfaceLost
	case SurfaceGetCurrentTextureStatusTimeout:
		dropTex()
		return nil, false, ErrSurfaceTimeout
	case NativeSurfaceGetCurrentTextureStatusOccluded:
		dropTex()
		return nil, false, ErrSurfaceOccluded
	case SurfaceGetCurrentTextureStatusError:
		// Soft native: not configured / parent device lost / other fatal-turned-soft.
		dropTex()
		if (s.deviceRef != nil && s.deviceRef.IsLost()) || isOwnerDeviceLost(s.device) {
			s.abandoned = true
			return nil, false, ErrSurfaceDeviceLost
		}
		if uncapturedMsg != "" {
			if looksLikeDeviceLost(uncapturedMsg) {
				noteLostMessage(s.device, uncapturedMsg)
				s.abandoned = true
				return nil, false, ErrSurfaceDeviceLost
			}
			return nil, false, &WGPUError{Op: "Surface.GetCurrentTexture", Type: uncapturedTyp, Message: uncapturedMsg}
		}
		return nil, false, &WGPUError{Op: "Surface.GetCurrentTexture", Message: "surface get current texture error"}
	default:
		dropTex()
		if uncapturedMsg != "" {
			return nil, false, &WGPUError{Op: "Surface.GetCurrentTexture", Type: uncapturedTyp, Message: uncapturedMsg}
		}
		return nil, false, &WGPUError{Op: "Surface.GetCurrentTexture", Message: "failed to get surface texture"}
	}
}

// Present presents the current frame to the surface.
// The texture argument is accepted for API compatibility with gogpu/wgpu but
// is unused in the FFI implementation (wgpuSurfacePresent takes no texture arg).
// Returns nil on success.
func (s *Surface) Present(texture ...*SurfaceTexture) error {
	if s == nil || s.handle == 0 {
		return &WGPUError{Op: "Surface.Present", Message: "surface is nil or released"}
	}
	if s.surfaceParentLost() {
		return ErrSurfaceDeviceLost
	}
	if err := checkInit(); err != nil {
		return err
	}
	gpuMu.Lock()
	defer gpuMu.Unlock()
	// Lighter than full canary: BeginFrame already probed. Only pump + sticky check
	// so Present does not double WriteBuffer cost every frame.
	if s.deviceRef != nil {
		pumpInstanceEvents(s.deviceRef)
		if s.deviceRef.absorbLostUncapturedLocked() {
			s.abandoned = true
			return ErrSurfaceDeviceLost
		}
	}
	if s.surfaceParentLost() {
		return ErrSurfaceDeviceLost
	}
	_, _ = LastUncapturedError()
	status, _, _ := procSurfacePresent.Call(s.handle)
	if typ, msg := LastUncapturedError(); msg != "" {
		if looksLikeDeviceLost(msg) {
			noteLostMessage(s.device, msg)
			s.abandoned = true
			return ErrSurfaceDeviceLost
		}
		return &WGPUError{Op: "Surface.Present", Type: typ, Message: msg}
	}
	if WGPUStatus(status) == WGPUStatusError {
		return &WGPUError{Op: "Surface.Present", Message: "wgpuSurfacePresent returned Error"}
	}
	_ = texture
	return nil
}

// Release releases the surface.
// Nil-safe and idempotent. Skips native release when the parent device is lost
// or the surface was abandoned after parent-lost refuse.
func (s *Surface) Release() {
	if s == nil {
		return
	}
	lost := s.abandoned || isOwnerDeviceLost(s.device) || (s.deviceRef != nil && s.deviceRef.IsLost())
	if lost {
		s.abandoned = true
	}
	s.device = 0
	s.deviceRef = nil
	releaseNativeHandle(&s.handle, lost, func(h uintptr) {
		procSurfaceRelease.Call(h) //nolint:errcheck
	})
}

// Handle returns the underlying handle. For advanced use only.
func (s *Surface) Handle() uintptr { return s.handle }

// GetCapabilities queries the surface capabilities for the given adapter.
// This determines which texture formats, present modes, and alpha modes are supported.
// The caller must provide a valid adapter that will be used with this surface.
// Defense order: nil/zero handle → init → native.
func (s *Surface) GetCapabilities(adapter *Adapter) (*SurfaceCapabilities, error) {
	if s == nil || s.handle == 0 {
		return nil, &WGPUError{Op: "Surface.GetCapabilities", Message: "surface is nil"}
	}
	if adapter == nil || adapter.handle == 0 {
		return nil, &WGPUError{Op: "Surface.GetCapabilities", Message: "adapter is nil"}
	}
	if err := checkInit(); err != nil {
		return nil, err
	}

	// Call wgpuSurfaceGetCapabilities
	var wire surfaceCapabilitiesWire
	procSurfaceGetCapabilities.Call( //nolint:errcheck
		s.handle,
		adapter.handle,
		uintptr(unsafe.Pointer(&wire)),
	)

	// Convert wire struct to Go struct
	caps := &SurfaceCapabilities{
		Usages: types.TextureUsage(wire.usages),
	}

	// Convert formats array
	if wire.formatCount > 0 && wire.formats != 0 {
		rawFormats := unsafe.Slice((*uint32)(ptrFromUintptr(wire.formats)), wire.formatCount)
		caps.Formats = make([]types.TextureFormat, len(rawFormats))
		for i, f := range rawFormats {
			caps.Formats[i] = types.TextureFormat(f)
		}
	}

	// Convert present modes array
	if wire.presentModeCount > 0 && wire.presentModes != 0 {
		rawPresentModes := unsafe.Slice((*uint32)(ptrFromUintptr(wire.presentModes)), wire.presentModeCount)
		caps.PresentModes = make([]types.PresentMode, len(rawPresentModes))
		for i, pm := range rawPresentModes {
			caps.PresentModes[i] = types.PresentMode(pm)
		}
	}

	// Convert alpha modes array
	if wire.alphaModeCount > 0 && wire.alphaModes != 0 {
		rawAlphaModes := unsafe.Slice((*uint32)(ptrFromUintptr(wire.alphaModes)), wire.alphaModeCount)
		caps.AlphaModes = make([]types.CompositeAlphaMode, len(rawAlphaModes))
		for i, am := range rawAlphaModes {
			caps.AlphaModes[i] = types.CompositeAlphaMode(am)
		}
	}

	// Free C memory allocated by wgpu-native
	procSurfaceCapabilitiesFreeMembers.Call(uintptr(unsafe.Pointer(&wire))) //nolint:errcheck

	return caps, nil
}
