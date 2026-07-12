package browser

import (
	"testing"

	"github.com/energye/gpui/gpu/types"
)

// TestCompositeAlphaModeToJS verifies all composite alpha mode mappings.
func TestCompositeAlphaModeToJS(t *testing.T) {
	tests := []struct {
		mode types.CompositeAlphaMode
		want string
	}{
		{types.CompositeAlphaModeAuto, "opaque"},
		{types.CompositeAlphaModeOpaque, "opaque"},
		{types.CompositeAlphaModePremultiplied, "premultiplied"},
		{types.CompositeAlphaModeUnpremultiplied, "opaque"}, // not supported on web, falls back
		{types.CompositeAlphaModeInherit, "opaque"},         // not supported on web, falls back
	}
	for _, tc := range tests {
		got := CompositeAlphaModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("CompositeAlphaModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestPresentModeToJS verifies that all present modes return "fifo" on browser.
func TestPresentModeToJS(t *testing.T) {
	tests := []struct {
		mode types.PresentMode
		want string
	}{
		{types.PresentModeFifo, "fifo"},
		{types.PresentModeFifoRelaxed, "fifo"},
		{types.PresentModeImmediate, "fifo"},
		{types.PresentModeMailbox, "fifo"},
		{types.PresentModeUndefined, "fifo"},
	}
	for _, tc := range tests {
		got := PresentModeToJS(tc.mode)
		if got != tc.want {
			t.Errorf("PresentModeToJS(%v) = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

// TestTextureFormatFromJS verifies the reverse mapping from JS strings to Go format constants.
func TestTextureFormatFromJS(t *testing.T) {
	tests := []struct {
		jsStr string
		want  types.TextureFormat
	}{
		// Canvas preferred formats (the primary use case)
		{"bgra8unorm", types.TextureFormatBGRA8Unorm},
		{"rgba8unorm", types.TextureFormatRGBA8Unorm},
		{"rgba16float", types.TextureFormatRGBA16Float},

		// Spot check other common formats
		{"r8unorm", types.TextureFormatR8Unorm},
		{"depth32float", types.TextureFormatDepth32Float},
		{"bc1-rgba-unorm", types.TextureFormatBC1RGBAUnorm},

		// Unknown returns Undefined
		{"", types.TextureFormatUndefined},
		{"nonexistent-format", types.TextureFormatUndefined},
	}
	for _, tc := range tests {
		got := TextureFormatFromJS(tc.jsStr)
		if got != tc.want {
			t.Errorf("TextureFormatFromJS(%q) = %v, want %v", tc.jsStr, got, tc.want)
		}
	}
}

// TestTextureFormatRoundTrip verifies that every format in the forward map
// can be recovered by the reverse map, and vice versa.
func TestTextureFormatRoundTrip(t *testing.T) {
	for goFmt, jsStr := range textureFormatMap {
		recovered := TextureFormatFromJS(jsStr)
		if recovered != goFmt {
			t.Errorf("round trip failed: TextureFormatToJS(%v) = %q, TextureFormatFromJS(%q) = %v, want %v",
				goFmt, jsStr, jsStr, recovered, goFmt)
		}
	}
}
