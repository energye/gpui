//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build linux && !(js && wasm)

package gles

import (
	"fmt"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
)

// Adapter implements hal.Adapter for OpenGL on Linux.
// Holds a shared *AdapterContext (owned by Instance on X11/headless, or by
// Surface on Wayland).
type Adapter struct {
	ctx           *AdapterContext
	displayHandle uintptr
	windowHandle  uintptr
	version       string
	renderer      string
	// from (0 = default display / surface context, no device). Info-only
	// enumeration entries carry it without a context; materialized adapters
	// carry both.
	eglDevice uintptr

	// caps holds the probed adapter capabilities (extensions, features,
	// limits, MSAA support). Populated by queryAdapterCapabilities during
	// adapter enumeration.
	caps AdapterCapabilities
}

// Open creates a logical device with the requested features and limits.
func (a *Adapter) Open(features gputypes.Features, limits gputypes.Limits) (hal.OpenDevice, error) {
	// EnumerateAdapters(nil) path returns an adapter with nil ctx because no
	// EGL context can be created without a display/window handle. Return a
	// descriptive error instead of a nil pointer dereference at GenVertexArrays.
	if a.ctx == nil || a.ctx.GL() == nil {
		return hal.OpenDevice{}, fmt.Errorf("gles: adapter has no GL context — pass a surface hint to CreateSurface before RequestDevice")
	}

	glCtx := a.ctx.Lock()
	defer a.ctx.Unlock()

	// VAO is created lazily in CreateCommandEncoder — ensures it's allocated
	// on the window surface (after Configure), not on the pbuffer (during Open).
	vao := uint32(0)

	// Query hardware texture unit limit for binding validation.
	var maxTexUnits int32
	glCtx.GetIntegerv(gl.MAX_TEXTURE_IMAGE_UNITS, &maxTexUnits)
	if maxTexUnits <= 0 {
		maxTexUnits = 8 // Conservative default
	}

	vendor := glCtx.GetString(gl.VENDOR)

	hal.Logger().Info("gles: device opened",
		"vendor", vendor,
		"version", a.version,
		"renderer", a.renderer,
		"maxTextureUnits", maxTexUnits,
	)

	glslVer := GLSLVersionToNaga(a.caps.GLSLVersion, a.caps.IsES)

	device := &Device{
		ctx:                 a.ctx,
		displayHandle:       a.displayHandle,
		windowHandle:        a.windowHandle,
		vao:                 vao,
		maxTextureUnits:     maxTexUnits,
		maxMSAA:             a.caps.MaxMSAASamples,
		features:            features,
		limits:              limits,
		glslVersion:         glslVer,
		shaderBindingLayout: glslVer.SupportsExplicitLocations(),
	}

	queue := &Queue{
		ctx:   a.ctx,
		fence: NewFence(glCtx),
	}
	device.queue = queue

	return hal.OpenDevice{
		Device: device,
		Queue:  queue,
	}, nil
}

// TextureFormatCapabilities returns capabilities for a texture format.
// Uses probed GL extension and MSAA information for accurate per-format detection.
func (a *Adapter) TextureFormatCapabilities(format gputypes.TextureFormat) hal.TextureFormatCapabilities {
	return queryTextureFormatCapabilities(format, a.caps.Features, a.caps.MaxMSAASamples, a.caps.Extensions)
}

// Info returns adapter metadata built from probed GL strings.
func (a *Adapter) Info() gputypes.AdapterInfo {
	return gputypes.AdapterInfo{
		Name:       a.renderer,
		Vendor:     a.caps.Vendor,
		VendorID:   a.caps.VendorID,
		DeviceType: a.caps.DeviceType,
		Driver:     a.version,
		DriverInfo: a.version,
		Backend:    gputypes.BackendGL,
	}
}

// Features returns probed GL features.
func (a *Adapter) Features() gputypes.Features { return a.caps.Features }

// Limits returns probed GL limits.
func (a *Adapter) Limits() gputypes.Limits { return a.caps.Limits }

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

// GetSurfaceCapabilities returns surface capabilities.
func (a *Adapter) GetSurfaceCapabilities(_ hal.Surface) *hal.SurfaceCapabilities {
	return &hal.SurfaceCapabilities{
		Formats: []gputypes.TextureFormat{
			gputypes.TextureFormatBGRA8Unorm,
			gputypes.TextureFormatRGBA8Unorm,
			gputypes.TextureFormatBGRA8UnormSrgb,
			gputypes.TextureFormatRGBA8UnormSrgb,
		},
		PresentModes: []gputypes.PresentMode{
			hal.PresentModeFifo,      // VSync on
			hal.PresentModeImmediate, // VSync off (if supported)
		},
		AlphaModes: []gputypes.CompositeAlphaMode{
			hal.CompositeAlphaModeOpaque,
			hal.CompositeAlphaModePremultiplied,
		},
	}
}

// Release releases the adapter.
func (a *Adapter) Release() {
	// Adapter doesn't own the GL context
}
