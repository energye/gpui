//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build js && wasm

package browser

import (
	"syscall/js"

	"github.com/energye/gpui/gpu/types"
)

// BuildSurfaceConfiguration constructs a JS GPUCanvasConfiguration object.
//
// The returned object is passed to GPUCanvasContext.configure(). Fields match
// the WebGPU spec GPUCanvasConfiguration dictionary:
//   - device: GPUDevice
//   - format: GPUTextureFormat string
//   - usage: GPUTextureUsageFlags (default: RENDER_ATTACHMENT)
//   - alphaMode: GPUCanvasAlphaMode ("opaque" or "premultiplied")
//   - viewFormats: sequence<GPUTextureFormat>
//
// Note: the spec does not include width/height/presentMode in the configure()
// call. Canvas dimensions are set separately via canvas.width/canvas.height.
func BuildSurfaceConfiguration(
	deviceRef js.Value,
	format types.TextureFormat,
	usage types.TextureUsage,
	alphaMode types.CompositeAlphaMode,
	viewFormats []types.TextureFormat,
) js.Value {
	config := newJSObject()
	config.Set("device", deviceRef)
	config.Set("format", TextureFormatToJS(format))

	// Usage defaults to RENDER_ATTACHMENT in the spec, but we set it explicitly
	// for clarity.
	config.Set("usage", float64(usage))

	config.Set("alphaMode", CompositeAlphaModeToJS(alphaMode))

	if len(viewFormats) > 0 {
		arr := newJSArray()
		for _, vf := range viewFormats {
			arr.Call("push", TextureFormatToJS(vf))
		}
		config.Set("viewFormats", arr)
	}

	return config
}
