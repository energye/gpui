// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build darwin && !(js && wasm)

package metal

import (
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Adapter implements hal.Adapter for Metal.
type Adapter struct {
	instance              *Instance
	raw                   ID   // id<MTLDevice>
	formatDepth24Stencil8 bool // true if Depth24UnormStencil8 supported (Intel-era AMD only)
	info                  gputypes.AdapterInfo
	features              gputypes.Features
	limits                gputypes.Limits
}

// mapTextureFormat converts a WebGPU texture format to Metal pixel format,
// accounting for device capabilities (e.g. Depth24 support on Apple Silicon).
func (a *Adapter) mapTextureFormat(format gputypes.TextureFormat) MTLPixelFormat {
	switch format {
	case gputypes.TextureFormatDepth24Plus:
		if a.formatDepth24Stencil8 {
			return MTLPixelFormatDepth24UnormStencil8
		}
		return MTLPixelFormatDepth32Float
	case gputypes.TextureFormatDepth24PlusStencil8:
		if a.formatDepth24Stencil8 {
			return MTLPixelFormatDepth24UnormStencil8
		}
		return MTLPixelFormatDepth32FloatStencil8
	default:
		return textureFormatToMTL(format)
	}
}

// Open opens a logical device with the requested features and limits.
func (a *Adapter) Open(features gputypes.Features, limits gputypes.Limits) (hal.OpenDevice, error) {
	device, err := newDevice(a)
	if err != nil {
		return hal.OpenDevice{}, err
	}

	queue := &Queue{
		device:       device,
		commandQueue: device.commandQueue,
	}

	// Initialize frame semaphore for CPU-ahead-of-GPU throttling.
	// Uses a buffered channel of size maxFramesInFlight pre-filled with tokens.
	// Each Submit() consumes a token; the GPU's addCompletedHandler: returns it.
	// If block support is unavailable, frameSemaphore stays nil (no throttling).
	if symNSConcreteGlobalBlock != 0 {
		queue.frameSemaphore = make(chan struct{}, maxFramesInFlight)
		for i := 0; i < maxFramesInFlight; i++ {
			queue.frameSemaphore <- struct{}{}
		}
	}

	// Back-reference so Device.WaitIdle can drain the frame semaphore.
	device.queue = queue
	device.features = features
	device.limits = limits

	hal.Logger().Debug("metal: adapter opened",
		"maxFramesInFlight", maxFramesInFlight,
		"blockSupport", symNSConcreteGlobalBlock != 0,
	)

	return hal.OpenDevice{
		Device: device,
		Queue:  queue,
	}, nil
}

// TextureFormatCapabilities returns capabilities for a specific texture format.
func (a *Adapter) TextureFormatCapabilities(format gputypes.TextureFormat) hal.TextureFormatCapabilities {
	flags := hal.TextureFormatCapabilitySampled

	// Most common formats support all operations on Metal
	switch format {
	case gputypes.TextureFormatRGBA8Unorm,
		gputypes.TextureFormatRGBA8UnormSrgb,
		gputypes.TextureFormatBGRA8Unorm,
		gputypes.TextureFormatBGRA8UnormSrgb,
		gputypes.TextureFormatRGBA16Float,
		gputypes.TextureFormatRGBA32Float:
		flags |= hal.TextureFormatCapabilityStorage |
			hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityBlendable |
			hal.TextureFormatCapabilityMultisample |
			hal.TextureFormatCapabilityMultisampleResolve

	case gputypes.TextureFormatDepth32Float,
		gputypes.TextureFormatDepth16Unorm:
		flags |= hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityMultisample

	case gputypes.TextureFormatDepth24PlusStencil8,
		gputypes.TextureFormatDepth32FloatStencil8:
		flags |= hal.TextureFormatCapabilityRenderAttachment
	}

	return hal.TextureFormatCapabilities{
		Flags: flags,
	}
}

// Info returns adapter metadata cached at enumeration.
func (a *Adapter) Info() gputypes.AdapterInfo { return a.info }

// Features returns enumerated features.
func (a *Adapter) Features() gputypes.Features { return a.features }

// Limits returns enumerated limits.
func (a *Adapter) Limits() gputypes.Limits { return a.limits }

// RequestDevice opens a device with the requested features and limits.
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

// GetSurfaceCapabilities returns capabilities for a specific surface.
// Matches webgpu Adapter.GetSurfaceCapabilities name.
func (a *Adapter) GetSurfaceCapabilities(surface hal.Surface) *hal.SurfaceCapabilities {
	if surface == nil {
		return nil
	}

	return &hal.SurfaceCapabilities{
		Formats: []gputypes.TextureFormat{
			gputypes.TextureFormatBGRA8Unorm,
			gputypes.TextureFormatBGRA8UnormSrgb,
			gputypes.TextureFormatRGBA8Unorm,
			gputypes.TextureFormatRGBA8UnormSrgb,
			gputypes.TextureFormatRGBA16Float,
		},
		PresentModes: []gputypes.PresentMode{
			hal.PresentModeFifo,
			hal.PresentModeImmediate,
			hal.PresentModeMailbox,
		},
		AlphaModes: []gputypes.CompositeAlphaMode{
			hal.CompositeAlphaModeOpaque,
			hal.CompositeAlphaModePremultiplied,
		},
	}
}

// Destroy releases the adapter.
func (a *Adapter) Destroy() {
	if a.raw != 0 {
		Release(a.raw)
		a.raw = 0
	}
}
