//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"

	intImage "github.com/energye/gpui/render/internal/image"
)

// ImageBuf is a public alias for internal ImageBuf.
// It represents a memory-efficient image buffer with support for multiple
// pixel formats and lazy premultiplication.
type ImageBuf = intImage.ImageBuf

// InterpolationMode defines how texture sampling is performed when drawing images.
type InterpolationMode = intImage.InterpolationMode

// Image interpolation modes.
const (
	// InterpNearest selects the closest pixel (no interpolation).
	// Fast but produces blocky results when scaling.
	InterpNearest = intImage.InterpNearest

	// InterpBilinear performs linear interpolation between 4 neighboring pixels.
	// Good balance between quality and performance.
	InterpBilinear = intImage.InterpBilinear

	// InterpBicubic performs cubic interpolation using a 4x4 pixel neighborhood.
	// Highest quality but slower than bilinear.
	InterpBicubic = intImage.InterpBicubic
)

// ImageFormat represents a pixel storage format.
type ImageFormat = intImage.Format

// Pixel formats.
const (
	// FormatGray8 is 8-bit grayscale (1 byte per pixel).
	FormatGray8 = intImage.FormatGray8

	// FormatGray16 is 16-bit grayscale (2 bytes per pixel).
	FormatGray16 = intImage.FormatGray16

	// FormatRGB8 is 24-bit RGB (3 bytes per pixel, no alpha).
	FormatRGB8 = intImage.FormatRGB8

	// FormatRGBA8 is 32-bit RGBA in sRGB color space (4 bytes per pixel).
	// This is the standard format for most operations.
	FormatRGBA8 = intImage.FormatRGBA8

	// FormatRGBAPremul is 32-bit RGBA with premultiplied alpha (4 bytes per pixel).
	// Used for correct alpha blending operations.
	FormatRGBAPremul = intImage.FormatRGBAPremul

	// FormatBGRA8 is 32-bit BGRA in sRGB color space (4 bytes per pixel).
	// Common on Windows and some GPU formats.
	FormatBGRA8 = intImage.FormatBGRA8

	// FormatBGRAPremul is 32-bit BGRA with premultiplied alpha (4 bytes per pixel).
	FormatBGRAPremul = intImage.FormatBGRAPremul
)

// Paint-level BlendMode type lives in blendmode.go (alias of internal/image).
// Constants are re-exported here next to DrawImage for discoverability.

// Blend modes (canonical paint API — see blendmode.go).
const (
	// BlendNormal performs standard alpha blending (source over destination).
	BlendNormal = intImage.BlendNormal

	// BlendMultiply multiplies source and destination colors.
	// Result is always darker or equal. Formula: dst * src
	BlendMultiply = intImage.BlendMultiply

	// BlendScreen performs inverse multiply for lighter results.
	// Formula: 1 - (1-dst) * (1-src)
	BlendScreen = intImage.BlendScreen

	// BlendOverlay combines multiply and screen based on destination brightness.
	// Dark areas are multiplied, bright areas are screened.
	BlendOverlay = intImage.BlendOverlay

	// BlendHue is a non-separable HSL-style blend (B.04).
	BlendHue = intImage.BlendHue
	// BlendSaturation is a non-separable HSL-style blend (B.04).
	BlendSaturation = intImage.BlendSaturation
	// BlendColor is a non-separable HSL-style blend (B.04).
	BlendColor = intImage.BlendColor
	// BlendLuminosity is a non-separable HSL-style blend (B.04).
	BlendLuminosity = intImage.BlendLuminosity

	// BlendClear is Porter-Duff Clear (B.02): result is transparent black.
	BlendClear = intImage.BlendClear
	// BlendCopy is Porter-Duff Src/Copy (B.02): result is source.
	BlendCopy = intImage.BlendCopy
	// BlendPlus is Porter-Duff Plus (B.02 / B.07): clamped source+destination.
	BlendPlus = intImage.BlendPlus
	// BlendModulate multiplies source*destination.
	BlendModulate = intImage.BlendModulate
	// BlendDestinationOut is Porter-Duff DstOut (B.02).
	BlendDestinationOut = intImage.BlendDestinationOut
	// BlendSourceAtop is Porter-Duff SrcAtop (B.02).
	BlendSourceAtop = intImage.BlendSourceAtop
	// BlendXor is Porter-Duff Xor (B.02).
	BlendXor = intImage.BlendXor
	// BlendDestinationOver is Porter-Duff DstOver (B.02).
	BlendDestinationOver = intImage.BlendDestinationOver
	// BlendSourceIn is Porter-Duff SrcIn (B.02).
	BlendSourceIn = intImage.BlendSourceIn
	// BlendSourceOut is Porter-Duff SrcOut (B.02).
	BlendSourceOut = intImage.BlendSourceOut
	// BlendDestinationIn is Porter-Duff DstIn (B.02).
	BlendDestinationIn = intImage.BlendDestinationIn
	// BlendDestinationAtop is Porter-Duff DstAtop (B.02).
	BlendDestinationAtop = intImage.BlendDestinationAtop

	// Separable advanced modes (B.03 extended).
	BlendDarken     = intImage.BlendDarken
	BlendLighten    = intImage.BlendLighten
	BlendColorDodge = intImage.BlendColorDodge
	BlendColorBurn  = intImage.BlendColorBurn
	BlendHardLight  = intImage.BlendHardLight
	BlendSoftLight  = intImage.BlendSoftLight
	BlendDifference = intImage.BlendDifference
	BlendExclusion  = intImage.BlendExclusion
)

// DrawImageOptions specifies parameters for drawing an image.
type DrawImageOptions struct {
	// X, Y specify the top-left corner where the image will be drawn.
	X, Y float64

	// DstWidth and DstHeight specify the dimensions to scale the image to.
	// If zero, the source dimensions are used (possibly from SrcRect).
	DstWidth  float64
	DstHeight float64

	// SrcRect defines the source rectangle to sample from.
	// If nil, the entire source image is used.
	SrcRect *image.Rectangle

	// Interpolation specifies the interpolation mode for sampling.
	// Default is InterpBilinear.
	Interpolation InterpolationMode

	// Opacity controls the overall transparency of the source image (0.0 to 1.0).
	// 1.0 means fully opaque, 0.0 means fully transparent.
	// Default is 1.0.
	Opacity float64

	// UseMipmaps enables mipmap sampling when the image is drawn smaller than
	// its native size (I.04). Currently applied on the CPU image path.
	UseMipmaps bool

	// BlendMode specifies how to blend source and destination pixels.
	// Default is BlendNormal.
	BlendMode BlendMode
}
