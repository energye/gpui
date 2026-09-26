//go:build !(js && wasm)

package noop

import (
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Adapter implements hal.Adapter for the noop backend.
type Adapter struct{}

// Open creates a noop device with the requested features and limits.
// Always succeeds and returns a device/queue pair.
func (a *Adapter) Open(features gputypes.Features, limits gputypes.Limits) (hal.OpenDevice, error) {
	queue := &Queue{}
	device := &Device{queue: queue, features: features, limits: limits}
	return hal.OpenDevice{
		Device: device,
		Queue:  queue,
	}, nil
}

// TextureFormatCapabilities returns default capabilities for all formats.
func (a *Adapter) TextureFormatCapabilities(_ gputypes.TextureFormat) hal.TextureFormatCapabilities {
	return hal.TextureFormatCapabilities{
		Flags: hal.TextureFormatCapabilitySampled |
			hal.TextureFormatCapabilityStorage |
			hal.TextureFormatCapabilityStorageReadWrite |
			hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityBlendable |
			hal.TextureFormatCapabilityMultisample |
			hal.TextureFormatCapabilityMultisampleResolve,
	}
}

// Info returns noop adapter metadata.
func (a *Adapter) Info() gputypes.AdapterInfo {
	return gputypes.AdapterInfo{
		Name:       "Noop Adapter",
		Vendor:     "GoGPU",
		DeviceType: gputypes.DeviceTypeOther,
		Driver:     "noop-1.0",
		DriverInfo: "No-operation backend for testing",
		Backend:    gputypes.BackendEmpty,
	}
}

// Features returns no features (noop backend).
func (a *Adapter) Features() gputypes.Features { return 0 }

// Limits returns default limits.
func (a *Adapter) Limits() gputypes.Limits { return gputypes.DefaultLimits() }

// RequestDevice opens a noop device with the requested features and limits.
// Queue is accessible via the returned Device.Queue().
func (a *Adapter) RequestDevice(desc *hal.DeviceDescriptor) (hal.Device, error) {
	var features gputypes.Features
	var limits gputypes.Limits = gputypes.DefaultLimits()
	if desc != nil {
		features = desc.RequiredFeatures
		if desc.RequiredLimits != (gputypes.Limits{}) {
			limits = desc.RequiredLimits
		}
	}
	opened, err := a.Open(features, limits)
	if err != nil {
		return nil, err
	}
	return opened.Device, nil
}

// GetSurfaceCapabilities returns default surface capabilities.
// Matches webgpu Adapter.GetSurfaceCapabilities name.
func (a *Adapter) GetSurfaceCapabilities(_ hal.Surface) *hal.SurfaceCapabilities {
	return &hal.SurfaceCapabilities{
		Formats: []gputypes.TextureFormat{
			gputypes.TextureFormatBGRA8Unorm,
			gputypes.TextureFormatRGBA8Unorm,
		},
		PresentModes: []gputypes.PresentMode{
			hal.PresentModeImmediate,
			hal.PresentModeMailbox,
			hal.PresentModeFifo,
			hal.PresentModeFifoRelaxed,
		},
		AlphaModes: []gputypes.CompositeAlphaMode{
			hal.CompositeAlphaModeOpaque,
			hal.CompositeAlphaModePremultiplied,
			hal.CompositeAlphaModeUnpremultiplied,
			hal.CompositeAlphaModeInherit,
		},
	}
}

// Destroy is a no-op for the noop adapter.
func (a *Adapter) Destroy() {}
