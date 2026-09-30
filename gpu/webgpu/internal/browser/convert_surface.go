//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package browser

import "github.com/energye/gpui/gpu/types"

// CompositeAlphaModeToJS converts a gputypes.CompositeAlphaMode to the WebGPU JS
// canvas alpha mode string.
//
// Browser WebGPU only supports "opaque" and "premultiplied". PostMultiplied and
// Inherit are not valid on the web. Auto and Opaque
// both map to "opaque".
//
// See: https://www.w3.org/TR/webgpu/#enumdef-gpucanvasalphamode
func CompositeAlphaModeToJS(mode types.CompositeAlphaMode) string {
	switch mode {
	case types.CompositeAlphaModePremultiplied:
		return "premultiplied"
	default:
		// Auto, Opaque, Unpremultiplied, Inherit all fall back to opaque.
		return "opaque" //nolint:goconst // intentional literal in enum-to-string conversion
	}
}

// PresentModeToJS converts a gputypes.PresentMode to the WebGPU JS present mode string.
//
// Browser WebGPU does not expose present mode control; the browser always uses
// FIFO (VSync).
// We return "fifo" for all modes since the browser ignores it anyway.
func PresentModeToJS(mode types.PresentMode) string {
	// Browser WebGPU only supports FIFO. The configure() call does not even
	// accept a presentMode field -- the browser auto-presents with VSync.
	_ = mode
	return "fifo" //nolint:goconst // intentional literal; browser only supports FIFO
}

// TextureFormatFromJS converts a WebGPU JS texture format string to the
// corresponding gputypes.TextureFormat. Returns TextureFormatUndefined if
// the string is not recognized.
//
// This is the reverse of TextureFormatToJS and is needed for parsing the
// preferred canvas format returned by navigator.gpu.getPreferredCanvasFormat().
func TextureFormatFromJS(s string) types.TextureFormat {
	f, ok := textureFormatFromJSMap[s]
	if ok {
		return f
	}
	return types.TextureFormatUndefined
}

// textureFormatFromJSMap is the reverse mapping of textureFormatMap.
// Built at init time from textureFormatMap.
var textureFormatFromJSMap map[string]types.TextureFormat

func init() {
	textureFormatFromJSMap = make(map[string]types.TextureFormat, len(textureFormatMap))
	for k, v := range textureFormatMap {
		textureFormatFromJSMap[v] = k
	}
}
