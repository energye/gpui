// Copyright 2025 The GoGPU Authors
// SPDX-License-Identifier: MIT

//go:build darwin && !(js && wasm)

package metal

import (
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu/hal"
)

// Adapter implements hal.Adapter for Metal.
type Adapter struct {
	instance              *Instance
	raw                   ID   // id<MTLDevice>
	formatDepth24Stencil8 bool // true if Depth24UnormStencil8 supported (Intel-era AMD only)
}

// mapTextureFormat converts a WebGPU texture format to Metal pixel format,
// accounting for device capabilities (e.g. Depth24 support on Apple Silicon).
func (a *Adapter) mapTextureFormat(format types.TextureFormat) MTLPixelFormat {
	switch format {
	case types.TextureFormatDepth24Plus:
		if a.formatDepth24Stencil8 {
			return MTLPixelFormatDepth24UnormStencil8
		}
		return MTLPixelFormatDepth32Float
	case types.TextureFormatDepth24PlusStencil8:
		if a.formatDepth24Stencil8 {
			return MTLPixelFormatDepth24UnormStencil8
		}
		return MTLPixelFormatDepth32FloatStencil8
	default:
		return textureFormatToMTL(format)
	}
}

// Open opens a logical device with the requested features and limits.
func (a *Adapter) Open(features types.Features, limits types.Limits) (hal.OpenDevice, error) {
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
func (a *Adapter) TextureFormatCapabilities(format types.TextureFormat) hal.TextureFormatCapabilities {
	flags := hal.TextureFormatCapabilitySampled

	// Most common formats support all operations on Metal
	switch format {
	case types.TextureFormatRGBA8Unorm,
		types.TextureFormatRGBA8UnormSrgb,
		types.TextureFormatBGRA8Unorm,
		types.TextureFormatBGRA8UnormSrgb,
		types.TextureFormatRGBA16Float,
		types.TextureFormatRGBA32Float:
		flags |= hal.TextureFormatCapabilityStorage |
			hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityBlendable |
			hal.TextureFormatCapabilityMultisample |
			hal.TextureFormatCapabilityMultisampleResolve

	case types.TextureFormatDepth32Float,
		types.TextureFormatDepth16Unorm:
		flags |= hal.TextureFormatCapabilityRenderAttachment |
			hal.TextureFormatCapabilityMultisample

	case types.TextureFormatDepth24PlusStencil8,
		types.TextureFormatDepth32FloatStencil8:
		flags |= hal.TextureFormatCapabilityRenderAttachment
	}

	return hal.TextureFormatCapabilities{
		Flags: flags,
	}
}

// SurfaceCapabilities returns capabilities for a specific surface.
func (a *Adapter) SurfaceCapabilities(surface hal.Surface) *hal.SurfaceCapabilities {
	if surface == nil {
		return nil
	}

	return &hal.SurfaceCapabilities{
		Formats: []types.TextureFormat{
			types.TextureFormatBGRA8Unorm,
			types.TextureFormatBGRA8UnormSrgb,
			types.TextureFormatRGBA8Unorm,
			types.TextureFormatRGBA8UnormSrgb,
			types.TextureFormatRGBA16Float,
		},
		PresentModes: []hal.PresentMode{
			hal.PresentModeFifo,
			hal.PresentModeImmediate,
			hal.PresentModeMailbox,
		},
		AlphaModes: []hal.CompositeAlphaMode{
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
