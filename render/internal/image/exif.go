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
	"fmt"
	"image"
)

// ImageOrientation is the EXIF/TIFF orientation of the stored pixels, i.e.
// where the visual top-left of the scene sits inside the encoded raster. It is
// reported before any transform is applied, and applying it is what turns the
// stored raster into the intended view.
//
// Values follow the EXIF Orientation tag (0x0112) and Skia's SkEncodedOrigin:
// 1..8 are the eight corner/axis permutations, 0 means "no orientation
// recorded". Anything else is reported as OrientationUnspecified.
type ImageOrientation uint8

const (
	OrientationUnspecified ImageOrientation = 0
	OrientationTopLeft     ImageOrientation = 1
	OrientationTopRight    ImageOrientation = 2
	OrientationBottomRight ImageOrientation = 3
	OrientationBottomLeft  ImageOrientation = 4
	OrientationLeftTop     ImageOrientation = 5
	OrientationRightTop    ImageOrientation = 6
	OrientationRightBottom ImageOrientation = 7
	OrientationLeftBottom  ImageOrientation = 8
)

// String names the orientation for probes and error text.
func (o ImageOrientation) String() string {
	switch o {
	case OrientationTopLeft:
		return "top-left"
	case OrientationTopRight:
		return "top-right"
	case OrientationBottomRight:
		return "bottom-right"
	case OrientationBottomLeft:
		return "bottom-left"
	case OrientationLeftTop:
		return "left-top"
	case OrientationRightTop:
		return "right-top"
	case OrientationRightBottom:
		return "right-bottom"
	case OrientationLeftBottom:
		return "left-bottom"
	default:
		return "unspecified"
	}
}

// SwapsWidthHeight reports whether applying this orientation transposes the
// raster (orientations 5..8), so callers sizing buffers before decode can
// allocate the displayed dimensions.
func (o ImageOrientation) SwapsWidthHeight() bool {
	return o >= OrientationLeftTop && o <= OrientationLeftBottom
}

// exifOrientationTag is the TIFF Orientation tag.
const exifOrientationTag = 0x0112

// maxHeaderScan bounds how far a header walk reads while looking for an EXIF
// block. Encoders place APP1/eXIf/EXIF before the pixel data, and any file that
// buries it deeper is treated as having no orientation rather than being read
// end to end twice.
const maxHeaderScan = 1 << 16

// ImageOrientationOfBytes reports the orientation recorded in an encoded image
// without decoding any pixels. Formats with no orientation field (GIF) or no
// EXIF block return OrientationUnspecified with a nil error.
func ImageOrientationOfBytes(data []byte) (ImageOrientation, error) {
	switch SniffEncoding(data) {
	case EncodingJPEG:
		return jpegOrientation(data)
	case EncodingPNG:
		return pngOrientation(data)
	case EncodingWebP:
		return webpOrientation(data)
	default:
		return OrientationUnspecified, nil
	}
}

// ImageOrientationOfStream reads only a bounded header window from br and
// reports the recorded orientation. Nothing is consumed: callers that reuse the
// reader can keep going from the start.
func ImageOrientationOfStream(br *bufio.Reader) (ImageOrientation, error) {
	switch SniffEncoding(peekWindow(br, 12)) {
	case EncodingJPEG, EncodingPNG, EncodingWebP:
		return ImageOrientationOfBytes(peekWindow(br, maxHeaderScan))
	default:
		return OrientationUnspecified, nil
	}
}

// peekWindow returns up to max leading bytes without consuming them. A short
// window (small file, or a buffer smaller than max) is returned as-is: header
// walks degrade to "no orientation recorded", never to a decode failure.
func peekWindow(br *bufio.Reader, max int) []byte {
	head, err := br.Peek(max)
	if err != nil && len(head) == 0 {
		return nil
	}
	return head
}

// jpegOrientation walks the marker segments looking for APP1/Exif. Measured on
// this repo's own samples: APP0/JFIF comes first, so position cannot be assumed.
func jpegOrientation(data []byte) (ImageOrientation, error) {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return OrientationUnspecified, nil
	}
	i := 2
	limit := len(data)
	if limit > maxHeaderScan {
		limit = maxHeaderScan
	}
	for i+4 <= limit {
		if data[i] != 0xFF {
			return OrientationUnspecified, nil // lost alignment: treat as unoriented
		}
		marker := data[i+1]
		if marker == 0xD8 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			i += 2
			continue
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		if segLen < 2 {
			return OrientationUnspecified, nil
		}
		body := data[i+4 : i+2+segLen] // segment payload (length counts itself + payload)
		if marker == 0xDA {
			break // start of scan: no EXIF ahead
		}
		if marker == 0xE1 {
			if ori, ok := orientationFromExifApp1(body); ok {
				return ori, nil
			}
		}
		i += 2 + segLen
	}
	return OrientationUnspecified, nil
}

// pngOrientation walks PNG chunks for the eXIf chunk (PNG third edition), which
// carries a bare TIFF stream.
func pngOrientation(data []byte) (ImageOrientation, error) {
	if len(data) < 8 || !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return OrientationUnspecified, nil
	}
	i := 8
	limit := len(data)
	if limit > maxHeaderScan {
		limit = maxHeaderScan
	}
	for i+12 <= limit {
		size := int(binary.BigEndian.Uint32(data[i : i+4]))
		if size < 0 || i+12+size > len(data) {
			return OrientationUnspecified, nil
		}
		typ := data[i+4 : i+8]
		body := data[i+8 : i+8+size]
		switch string(typ) {
		case "eXIf":
			return orientationFromTIFF(body)
		case "IDAT", "IEND":
			return OrientationUnspecified, nil // eXIf must precede the image data
		}
		i += 12 + size
	}
	return OrientationUnspecified, nil
}

// webpOrientation walks RIFF chunks for the EXIF chunk. WebP stores the TIFF
// stream without the JPEG-style "Exif\0\0" prefix, but both forms are accepted.
func webpOrientation(data []byte) (ImageOrientation, error) {
	if len(data) < 12 || !bytes.HasPrefix(data, []byte("RIFF")) || !bytes.Equal(data[8:12], []byte("WEBP")) {
		return OrientationUnspecified, nil
	}
	i := 12
	limit := len(data)
	if limit > maxHeaderScan {
		limit = maxHeaderScan
	}
	for i+8 <= limit {
		fourcc := string(data[i : i+4])
		size := int(binary.LittleEndian.Uint32(data[i+4 : i+8]))
		if size < 0 || i+8+size > len(data) {
			return OrientationUnspecified, nil
		}
		body := data[i+8 : i+8+size]
		if fourcc == "EXIF" {
			if ori, ok := orientationFromExifApp1(body); ok {
				return ori, nil
			}
			return orientationFromTIFF(body)
		}
		i += 8 + size + (size & 1)
	}
	return OrientationUnspecified, nil
}

// orientationFromExifApp1 handles the JPEG APP1 payload, which starts with the
// "Exif\0\0" identifier before the TIFF stream.
func orientationFromExifApp1(body []byte) (ImageOrientation, bool) {
	if bytes.HasPrefix(body, []byte("Exif\x00\x00")) {
		ori, err := orientationFromTIFF(body[6:])
		if err != nil {
			return OrientationUnspecified, false
		}
		return ori, ori != OrientationUnspecified
	}
	return OrientationUnspecified, false
}

// orientationFromTIFF parses just enough of the TIFF IFD0 to read tag 0x0112.
func orientationFromTIFF(tiff []byte) (ImageOrientation, error) {
	if len(tiff) < 8 {
		return OrientationUnspecified, fmt.Errorf("image: exif TIFF too short")
	}
	var order binary.ByteOrder
	switch {
	case bytes.Equal(tiff[:2], []byte("II")):
		order = binary.LittleEndian
	case bytes.Equal(tiff[:2], []byte("MM")):
		order = binary.BigEndian
	default:
		return OrientationUnspecified, fmt.Errorf("image: exif TIFF byte order")
	}
	if order.Uint16(tiff[2:4]) != 42 {
		return OrientationUnspecified, fmt.Errorf("image: exif TIFF magic")
	}
	ifdOff := int(order.Uint32(tiff[4:8]))
	if ifdOff < 8 || ifdOff+2 > len(tiff) {
		return OrientationUnspecified, fmt.Errorf("image: exif IFD offset")
	}
	count := int(order.Uint16(tiff[ifdOff : ifdOff+2]))
	for k := 0; k < count; k++ {
		ent := ifdOff + 2 + k*12
		if ent+12 > len(tiff) {
			return OrientationUnspecified, fmt.Errorf("image: exif IFD entry runs past the block")
		}
		tag := order.Uint16(tiff[ent : ent+2])
		if tag != exifOrientationTag {
			continue
		}
		// Orientation is a single SHORT stored inline in the value field.
		val := order.Uint16(tiff[ent+8 : ent+10])
		if val > 8 {
			return OrientationUnspecified, nil
		}
		return ImageOrientation(val), nil
	}
	return OrientationUnspecified, nil
}

// orientImage returns img with the recorded orientation applied so the raster
// matches the intended view. Orientations 0 and 1 are already correct and come
// back unchanged. The mapping below is the EXIF tag definition, expressed as the
// stored coordinate feeding each display pixel:
//
//	1 (x, y)                   5 (y, x)
//	2 (sw-1-x, y)              6 (y, sh-1-x)
//	3 (sw-1-x, sh-1-y)         7 (sw-1-y, sh-1-x)
//	4 (x, sh-1-y)              8 (sw-1-y, x)
//
// 5..8 transpose the raster, which is why SwapsWidthHeight exists: a caller
// sizing a buffer before decode needs the displayed extent.
func orientImage(img image.Image, ori ImageOrientation) image.Image {
	if ori <= OrientationTopLeft || ori > OrientationLeftBottom {
		return img
	}
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	dw, dh := sw, sh
	if ori.SwapsWidthHeight() {
		dw, dh = sh, sw
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			sx, sy := storedCoords(ori, x, y, sw, sh)
			dst.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return dst
}

// storedCoords maps display pixel (x, y) back into the stored raster of size
// (sw, sh).
func storedCoords(ori ImageOrientation, x, y, sw, sh int) (int, int) {
	switch ori {
	case OrientationTopRight:
		return sw - 1 - x, y
	case OrientationBottomRight:
		return sw - 1 - x, sh - 1 - y
	case OrientationBottomLeft:
		return x, sh - 1 - y
	case OrientationLeftTop:
		return y, x
	case OrientationRightTop:
		return y, sh - 1 - x
	case OrientationRightBottom:
		return sw - 1 - y, sh - 1 - x
	case OrientationLeftBottom:
		return sw - 1 - y, x
	default:
		return x, y
	}
}
