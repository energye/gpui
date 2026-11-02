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
