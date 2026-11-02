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
	"io"

	intImage "github.com/energye/gpui/render/internal/image"
)

// ImageEncoding names an encoded image container, identified from the leading
// bytes of the data rather than from a file extension. It describes what the
// bytes are; whether pixels can be produced is a separate question
// (ImageEncoding.IsDecodable).
type ImageEncoding = intImage.Encoding

const (
	ImageEncodingUnknown = intImage.EncodingUnknown
	ImageEncodingPNG     = intImage.EncodingPNG
	ImageEncodingJPEG    = intImage.EncodingJPEG
	ImageEncodingGIF     = intImage.EncodingGIF
	ImageEncodingWebP    = intImage.EncodingWebP
	// ImageEncodingAVIF and ImageEncodingHEIF are recognised but not decoded
	// in this build: no AV1/HEVC entropy decoder is wired in (P3.6 decision D1).
	ImageEncodingAVIF = intImage.EncodingAVIF
	ImageEncodingHEIF = intImage.EncodingHEIF
)

// ImageOrientation is the EXIF/TIFF orientation recorded for stored pixels:
// where the visual top-left of the scene sits inside the encoded raster. Values
// follow the EXIF Orientation tag (0x0112) and Skia's SkEncodedOrigin, so 1..8
// are the eight corner/axis permutations and 0 means nothing was recorded.
type ImageOrientation = intImage.ImageOrientation

const (
	OrientationUnspecified ImageOrientation = intImage.OrientationUnspecified
	OrientationTopLeft     ImageOrientation = intImage.OrientationTopLeft
	OrientationTopRight    ImageOrientation = intImage.OrientationTopRight
	OrientationBottomRight ImageOrientation = intImage.OrientationBottomRight
	OrientationBottomLeft  ImageOrientation = intImage.OrientationBottomLeft
	OrientationLeftTop     ImageOrientation = intImage.OrientationLeftTop
	OrientationRightTop    ImageOrientation = intImage.OrientationRightTop
	OrientationRightBottom ImageOrientation = intImage.OrientationRightBottom
	OrientationLeftBottom  ImageOrientation = intImage.OrientationLeftBottom
)

// DecodeOptions controls decode-time normalisation. The zero value is the
// default behaviour: any recorded EXIF orientation is applied to the raster, so
// the returned ImageBuf is what the image is meant to look like.
type DecodeOptions = intImage.DecodeOptions

// Decoding errors. Both are sentinel values; use errors.Is.
var (
	// ErrUnsupportedFormat reports that no decoder claims the bytes — either
	// the container is unrecognised, or it is known (AVIF/HEIF) but has no
	// pixel decoder in this build.
	ErrUnsupportedFormat = intImage.ErrUnsupportedFormat

	// ErrEmptyData reports empty image input.
	ErrEmptyData = intImage.ErrEmptyData
)

// AnimatedImage decodes a multi-frame container (GIF, APNG, animated WebP) and
// composites frames onto one reused ImageBuf, so advancing a frame rewrites the
// same GPU texture instead of allocating one per frame.
type AnimatedImage = intImage.AnimatedImage

// FrameInfo is one frame's playback and compositing metadata.
type FrameInfo = intImage.FrameInfo

// FrameBlend says how a frame's pixels meet the canvas underneath.
type FrameBlend = intImage.FrameBlend

// FrameDisposal says what happens to a frame's rectangle once it is off screen.
type FrameDisposal = intImage.FrameDisposal

const (
	FrameBlendSrc     = intImage.FrameBlendSrc
	FrameBlendSrcOver = intImage.FrameBlendSrcOver

	FrameDisposalKeep            = intImage.FrameDisposalKeep
	FrameDisposalRestoreBGColor  = intImage.FrameDisposalRestoreBGColor
	FrameDisposalRestorePrevious = intImage.FrameDisposalRestorePrevious

	// NoRequiredFrame means a frame can be decoded without any earlier frame.
	NoRequiredFrame = intImage.NoRequiredFrame
)

// LoadAnimatedImage opens a multi-frame container from a path.
func LoadAnimatedImage(path string) (*AnimatedImage, error) {
	return intImage.LoadAnimatedImage(path)
}

// LoadAnimatedImageFromBytes opens a multi-frame container from encoded bytes.
func LoadAnimatedImageFromBytes(data []byte, opts DecodeOptions) (*AnimatedImage, error) {
	return intImage.LoadAnimatedImageFromBytes(data, opts)
}

// DecodeImage decodes from a stream, identifying the container from its leading
// bytes. It is the streaming twin of LoadImageFromBytes and the single door the
// UI layer's async decode pool goes through.
func DecodeImage(r io.Reader, opts DecodeOptions) (*ImageBuf, error) {
	return intImage.Decode(r, opts)
}

// DecodeImageConfig reports the encoded image's stored-raster dimensions without
// decoding pixels, so callers can route or size buffers before paying for the
// raster. It does not apply orientation: when ImageOrientationOfBytes returns
// an orientation whose SwapsWidthHeight is true, the displayed size transposes
// the reported one.
func DecodeImageConfig(r io.Reader) (image.Config, error) {
	return intImage.DecodeConfig(r)
}
