//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package image

import (
	"bufio"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/webp"
)

// LoadAnimatedImage opens a multi-frame container from a path and returns a
// compositor. Supported: GIF, APNG, animated WebP (lossy frames with an ALPH
// chunk are recognised and rejected rather than silently drawn opaque).
//
// A single-frame file is an error: use LoadImage for those.
func LoadAnimatedImage(path string) (*AnimatedImage, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("image: open file: %w", err)
	}
	return LoadAnimatedImageFromBytes(data, DecodeOptions{})
}

// LoadAnimatedImageFromBytes opens a multi-frame container from encoded bytes.
func LoadAnimatedImageFromBytes(data []byte, opts DecodeOptions) (*AnimatedImage, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}
	enc := SniffEncoding(data)
	if err := enc.ErrDecodable(); err != nil {
		return nil, err
	}
	switch enc {
	case EncodingGIF:
		return openGIFAnim(data)
	case EncodingPNG:
		return openAPNGAnim(data)
	case EncodingWebP:
		return openWebPAnim(data)
	default:
		return nil, fmt.Errorf("%w: %s is not an animated container", ErrUnsupportedFormat, enc)
	}
}

// DecodeConfig reports dimensions without decoding pixels, routing by the
// sniffed container.
func DecodeConfig(r io.Reader) (image.Config, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, 12)
	}
	enc := SniffEncoding(peekWindow(br, 12))
	if enc == EncodingUnknown {
		return image.Config{}, fmt.Errorf("%w: unrecognised container", ErrUnsupportedFormat)
	}
	if err := enc.ErrDecodable(); err != nil {
		return image.Config{}, err
	}
	switch enc {
	case EncodingPNG:
		return png.DecodeConfig(br)
	case EncodingJPEG:
		return jpeg.DecodeConfig(br)
	case EncodingGIF:
		return gif.DecodeConfig(br)
	case EncodingWebP:
		return webp.DecodeConfig(br)
	default:
		return image.Config{}, fmt.Errorf("%w: %s has no decoder wired in", ErrUnsupportedFormat, enc)
	}
}
