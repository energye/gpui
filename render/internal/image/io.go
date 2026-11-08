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
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/webp" // also registers WebP decoder for image.Decode
)

// I/O errors.
var (
	// ErrUnsupportedFormat is returned when no registered decoder claims the
	// bytes — either the container is unknown, or it is recognised
	// (AVIF/HEIF) but has no pixel decoder wired in this build.
	ErrUnsupportedFormat = errors.New("image: unsupported format")

	// ErrEmptyData is returned when image data is empty.
	ErrEmptyData = errors.New("image: empty data")
)

// DecodeOptions controls decode-time normalisation. The zero value is the
// default behaviour, which is how the image is meant to be viewed: any recorded
// EXIF orientation is applied to the raster.
type DecodeOptions struct {
	// PreserveOrientation returns the stored raster exactly as encoded. The
	// recorded orientation remains queryable through
	// ImageOrientationOfBytes and ImageOrientationOfStream.
	PreserveOrientation bool
}

// bufioHeaderSize covers the bounded header walk that finds EXIF orientation,
// so one peek serves both format detection and the orientation probe.
const bufioHeaderSize = maxHeaderScan

// LoadPNG loads a PNG image from the given file path.
func LoadPNG(path string) (*ImageBuf, error) {
	return loadFile(path, DecodeOptions{}, EncodingPNG)
}

// LoadJPEG loads a JPEG image from the given file path.
func LoadJPEG(path string) (*ImageBuf, error) {
	return loadFile(path, DecodeOptions{}, EncodingJPEG)
}

// LoadWebP loads a WebP image from the given file path.
func LoadWebP(path string) (*ImageBuf, error) {
	return loadFile(path, DecodeOptions{}, EncodingWebP)
}

// LoadImage loads an image from the given file path, identifying the container
// from its leading bytes. The extension is not consulted, so a mislabelled file
// still decodes to what it actually contains. Recorded EXIF orientation is
// applied by default, matching how the reference frameworks present images.
func LoadImage(path string) (*ImageBuf, error) {
	return LoadImageWithOptions(path, DecodeOptions{})
}

// LoadImageWithOptions loads an image from a file path with explicit options.
func LoadImageWithOptions(path string, opts DecodeOptions) (*ImageBuf, error) {
	return loadFile(path, opts, EncodingUnknown)
}

// LoadImageFromBytes loads an image from encoded bytes, identifying the
// container from the leading bytes.
func LoadImageFromBytes(data []byte, opts DecodeOptions) (*ImageBuf, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}
	return Decode(bytes.NewReader(data), opts)
}

// Decode decodes an image from the given reader, identifying the container from
// the leading bytes. Only a bounded header window is inspected before decoding.
func Decode(r io.Reader, opts DecodeOptions) (*ImageBuf, error) {
	if r == nil {
		return nil, ErrEmptyData
	}
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReaderSize(r, bufioHeaderSize)
	}
	// The window is peeked, not consumed: the decoders below still see the
	// whole stream, and orientation has to be read before the header is eaten.
	window := peekWindow(br, bufioHeaderSize)
	format := SniffEncoding(window)
	if format == EncodingUnknown {
		return nil, fmt.Errorf("%w: unrecognised container", ErrUnsupportedFormat)
	}
	if err := format.ErrDecodable(); err != nil {
		return nil, err
	}
	var ori ImageOrientation
	if !opts.PreserveOrientation {
		ori, _ = ImageOrientationOfBytes(window)
	}
	img, err := decodeByFormat(br, format, window)
	if err != nil {
		return nil, err
	}
	return FromStdImage(orientImage(img, ori)), nil
}

// loadFile opens a path and decodes it. wantFormat, when not EncodingUnknown,
// pins the container so a format-specific loader fails loudly instead of
// silently falling back to another decoder.
func loadFile(path string, opts DecodeOptions, wantFormat Encoding) (*ImageBuf, error) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("image: open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	br := bufio.NewReaderSize(f, bufioHeaderSize)
	if wantFormat != EncodingUnknown && SniffEncoding(peekWindow(br, 12)) != wantFormat {
		return nil, fmt.Errorf("image: %s: asked for %s, content is %s", path, wantFormat,
			SniffEncoding(peekWindow(br, 12)))
	}
	return Decode(br, opts)
}

// decodeByFormat dispatches to the decoder registered for a sniffed container.
// window is the already-peeked header (it costs nothing: peek does not consume),
// needed to tell an animated WebP apart from a still one before the stream is
// handed to a decoder that would choke on ANMF chunks.
func decodeByFormat(r io.Reader, format Encoding, window []byte) (image.Image, error) {
	switch format {
	case EncodingPNG:
		img, err := png.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("image: decode PNG: %w", err)
		}
		return img, nil
	case EncodingJPEG:
		img, err := jpeg.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("image: decode JPEG: %w", err)
		}
		return img, nil
	case EncodingGIF:
		// Static door for a GIF: the first frame, which is what a single-frame
		// consumer expects as the poster. Animation goes through
		// LoadAnimatedImageFromBytes.
		g, err := gif.DecodeAll(r)
		if err != nil {
			return nil, fmt.Errorf("image: decode GIF: %w", err)
		}
		if len(g.Image) == 0 {
			return nil, fmt.Errorf("%w: GIF has no frames", ErrUnsupportedFormat)
		}
		return g.Image[0], nil
	case EncodingWebP:
		if webPAnimated(window) {
			// The frame table must be walked before frame 0 can be drawn, so
			// the stream is buffered here; animated WebP files carry the whole
			// frame list in one RIFF blob anyway.
			data, err := io.ReadAll(r)
			if err != nil {
				return nil, fmt.Errorf("image: read animated WebP: %w", err)
			}
			a, err := openWebPAnim(data)
			if err != nil {
				return nil, err
			}
			defer a.Dispose()
			if err := a.DecodeFrame(0); err != nil {
				return nil, err
			}
			return a.Image().ToStdImage(), nil
		}
		img, err := webp.Decode(r)
		if err != nil {
			return nil, fmt.Errorf("image: decode WebP: %w", err)
		}
		return img, nil
	default:
		return nil, fmt.Errorf("%w: %s has no decoder wired in", ErrUnsupportedFormat, format)
	}
}

// webPAnimated reports whether the VP8X chunk in the header window sets the
// animation bit. A missing or unparsed VP8X is treated as "not animated".
func webPAnimated(window []byte) bool {
	if len(window) < 12 || !bytes.HasPrefix(window, []byte("RIFF")) || !bytes.Equal(window[8:12], []byte("WEBP")) {
		return false
	}
	i := 12
	for i+8 <= len(window) {
		fcc := string(window[i : i+4])
		size := int(binary.LittleEndian.Uint32(window[i+4 : i+8]))
		if size < 0 || i+8+size > len(window) {
			return false
		}
		if fcc == "VP8X" {
			return size >= 1 && window[i+8]&0x02 != 0
		}
		i += 8 + size + (size & 1)
	}
	return false
}

// DecodePNG decodes a PNG image from the given reader, leaving any recorded
// orientation unapplied.
func DecodePNG(r io.Reader) (*ImageBuf, error) {
	img, err := png.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("image: decode PNG: %w", err)
	}
	return FromStdImage(img), nil
}

// DecodeJPEG decodes a JPEG image from the given reader, leaving any recorded
// orientation unapplied.
func DecodeJPEG(r io.Reader) (*ImageBuf, error) {
	img, err := jpeg.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("image: decode JPEG: %w", err)
	}
	return FromStdImage(img), nil
}

// DecodeWebP decodes a WebP image from the given reader, leaving any recorded
// orientation unapplied.
func DecodeWebP(r io.Reader) (*ImageBuf, error) {
	img, err := webp.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("image: decode WebP: %w", err)
	}
	return FromStdImage(img), nil
}

// SavePNG saves the image as a PNG file.
func (b *ImageBuf) SavePNG(path string) error {
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("image: create file: %w", err)
	}

	if err := b.EncodePNG(f); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}

// SaveJPEG saves the image as a JPEG file with the given quality (1-100).
func (b *ImageBuf) SaveJPEG(path string, quality int) error {
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("image: create file: %w", err)
	}

	if err := b.EncodeJPEG(f, quality); err != nil {
		_ = f.Close()
		return err
	}

	return f.Close()
}

// EncodePNG encodes the image as PNG to the given writer.
func (b *ImageBuf) EncodePNG(w io.Writer) error {
	img := b.ToStdImage()
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("image: encode PNG: %w", err)
	}
	return nil
}

// EncodeJPEG encodes the image as JPEG to the given writer.
func (b *ImageBuf) EncodeJPEG(w io.Writer, quality int) error {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}

	img := b.ToStdImage()
	if err := jpeg.Encode(w, img, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("image: encode JPEG: %w", err)
	}
	return nil
}

// FromStdImage creates an ImageBuf from a standard library image.Image.
// The resulting ImageBuf will be in RGBA8 format.
func FromStdImage(img image.Image) *ImageBuf {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// *image.RGBA pixels are PREMULTIPLIED (stdlib contract) — store them in
	// the premul format so blitters treat the bytes correctly. The straight
	// FormatRGBA8 here used to double-multiply every semi-transparent RGBA
	// drawn through ImageBufFromImage (the draw legs premultiply "straight"
	// data again). The generic path below has the same trap: image.Image's
	// At() is premul 16-bit, so it lands in the premul format too.
	premul, _ := NewImageBuf(width, height, FormatRGBAPremul)
	buf := premul

	// Fast path for RGBA images
	if rgba, ok := img.(*image.RGBA); ok {
		// Direct copy if stride matches
		if rgba.Stride == buf.Stride() {
			copy(buf.Data(), rgba.Pix)
			return buf
		}
		// Row-by-row copy for different strides
		for y := range height {
			srcStart := y * rgba.Stride
			copy(buf.RowBytes(y), rgba.Pix[srcStart:srcStart+width*4])
		}
		return buf
	}

	// Fast path for NRGBA images
	if nrgba, ok := img.(*image.NRGBA); ok {
		if nrgba.Stride == buf.Stride() {
			copy(buf.Data(), nrgba.Pix)
			return buf
		}
		for y := range height {
			srcStart := y * nrgba.Stride
			copy(buf.RowBytes(y), nrgba.Pix[srcStart:srcStart+width*4])
		}
		return buf
	}

	// Generic slow path for any image type: At() is premul 16-bit — store
	// the 8-bit premul bytes verbatim in the premul buffer.
	for y := range height {
		for x := range width {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			r, g, b, a := c.RGBA()
			// RGBA() returns 16-bit values, scale to 8-bit
			// Right shift by 8 guarantees result fits in uint8
			i := y*buf.Stride() + x*4
			d := buf.Data()
			d[i], d[i+1], d[i+2], d[i+3] = byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8)
		}
	}

	return buf
}

// ToStdImage converts the ImageBuf to a standard library image.Image.
// Returns *image.RGBA for most formats, *image.Gray for grayscale.
func (b *ImageBuf) ToStdImage() image.Image {
	rect := image.Rect(0, 0, b.width, b.height)

	switch b.format {
	case FormatGray8:
		gray := image.NewGray(rect)
		for y := range b.height {
			row := b.RowBytes(y)
			copy(gray.Pix[y*gray.Stride:], row)
		}
		return gray

	case FormatGray16:
		gray16 := image.NewGray16(rect)
		for y := range b.height {
			row := b.RowBytes(y)
			dstStart := y * gray16.Stride
			for x := range b.width {
				// Gray16 in image package is big-endian
				gray16.Pix[dstStart+x*2] = row[x*2+1] // high byte
				gray16.Pix[dstStart+x*2+1] = row[x*2] // low byte
			}
		}
		return gray16

	case FormatRGBA8:
		// RGBA8 is non-premultiplied, use NRGBA
		nrgba := image.NewNRGBA(rect)
		if b.stride == nrgba.Stride {
			copy(nrgba.Pix, b.data)
		} else {
			for y := range b.height {
				copy(nrgba.Pix[y*nrgba.Stride:], b.RowBytes(y))
			}
		}
		return nrgba

	case FormatRGBAPremul:
		// RGBAPremul is premultiplied, use RGBA
		rgba := image.NewRGBA(rect)
		if b.stride == rgba.Stride {
			copy(rgba.Pix, b.data)
		} else {
			for y := range b.height {
				copy(rgba.Pix[y*rgba.Stride:], b.RowBytes(y))
			}
		}
		return rgba

	case FormatBGRA8:
		// Convert BGRA to NRGBA (non-premultiplied)
		nrgba := image.NewNRGBA(rect)
		for y := range b.height {
			row := b.RowBytes(y)
			dstStart := y * nrgba.Stride
			for x := range b.width {
				srcOff := x * 4
				dstOff := dstStart + x*4
				nrgba.Pix[dstOff] = row[srcOff+2]   // R <- B
				nrgba.Pix[dstOff+1] = row[srcOff+1] // G <- G
				nrgba.Pix[dstOff+2] = row[srcOff]   // B <- R
				nrgba.Pix[dstOff+3] = row[srcOff+3] // A <- A
			}
		}
		return nrgba

	case FormatBGRAPremul:
		// Convert BGRA to RGBA (premultiplied)
		rgba := image.NewRGBA(rect)
		for y := range b.height {
			row := b.RowBytes(y)
			dstStart := y * rgba.Stride
			for x := range b.width {
				srcOff := x * 4
				dstOff := dstStart + x*4
				rgba.Pix[dstOff] = row[srcOff+2]   // R <- B
				rgba.Pix[dstOff+1] = row[srcOff+1] // G <- G
				rgba.Pix[dstOff+2] = row[srcOff]   // B <- R
				rgba.Pix[dstOff+3] = row[srcOff+3] // A <- A
			}
		}
		return rgba

	case FormatRGB8:
		// Expand to NRGBA (opaque)
		nrgba := image.NewNRGBA(rect)
		for y := range b.height {
			row := b.RowBytes(y)
			dstStart := y * nrgba.Stride
			for x := range b.width {
				srcOff := x * 3
				dstOff := dstStart + x*4
				nrgba.Pix[dstOff] = row[srcOff]
				nrgba.Pix[dstOff+1] = row[srcOff+1]
				nrgba.Pix[dstOff+2] = row[srcOff+2]
				nrgba.Pix[dstOff+3] = 255 // Opaque
			}
		}
		return nrgba

	default:
		// Fallback using GetRGBA - assume non-premultiplied
		nrgba := image.NewNRGBA(rect)
		for y := range b.height {
			for x := range b.width {
				r, g, bl, a := b.GetRGBA(x, y)
				off := y*nrgba.Stride + x*4
				nrgba.Pix[off] = r
				nrgba.Pix[off+1] = g
				nrgba.Pix[off+2] = bl
				nrgba.Pix[off+3] = a
			}
		}
		return nrgba
	}
}

// EncodeToBytes encodes the image to PNG format and returns the bytes.
func (b *ImageBuf) EncodeToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := b.EncodePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// EncodeToJPEGBytes encodes the image to JPEG format and returns the bytes.
func (b *ImageBuf) EncodeToJPEGBytes(quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := b.EncodeJPEG(&buf, quality); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
