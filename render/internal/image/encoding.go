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
	"bytes"
	"fmt"
)

// Encoding names an encoded image container, identified by its magic bytes.
// It is deliberately separate from Format, which describes the decoded pixel
// layout. P3.6 batch 1a: content sniffing is the primary key for choosing a
// decoder, and a file extension is only ever a hint.
type Encoding int

const (
	// EncodingUnknown means no registered decoder claims these bytes.
	EncodingUnknown Encoding = iota
	EncodingPNG
	EncodingJPEG
	EncodingGIF
	EncodingWebP
	// EncodingAVIF and EncodingHEIF are detected so the loader can name the
	// container in its error. Pixel decoding for both is deferred: the module
	// set has no AV1/HEVC entropy decoder (ledger P3.6 decision D1).
	EncodingAVIF
	EncodingHEIF
)

// String names the encoding for error messages and probe output.
func (e Encoding) String() string {
	switch e {
	case EncodingPNG:
		return "png"
	case EncodingJPEG:
		return "jpeg"
	case EncodingGIF:
		return "gif"
	case EncodingWebP:
		return "webp"
	case EncodingAVIF:
		return "avif"
	case EncodingHEIF:
		return "heif"
	default:
		return "unknown"
	}
}

// heifBrands are the ISO-BMFF major brands carrying HEVC image items, plus the
// HEIF meta families. Brand matching alone cannot always separate AVIF from
// HEIF, so the compatible-brand list is consulted as well.
var heifBrands = map[string]bool{
	"heic": true, "heix": true, "hevc": true, "hevx": true,
	"heim": true, "heis": true, "mif1": true, "msf1": true,
}

var avifBrands = map[string]bool{
	"avif": true, "avis": true,
}

// SniffEncoding identifies the container from its leading bytes. Every
// registered format decides within the first 12 bytes except the ISO-BMFF
// brand window, so callers may hand it a bounded header peek.
func SniffEncoding(data []byte) Encoding {
	switch {
	case len(data) >= 8 && bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")):
		return EncodingPNG
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return EncodingJPEG
	case len(data) >= 6 && bytes.HasPrefix(data, []byte("GIF8")):
		return EncodingGIF
	case len(data) >= 12 && bytes.HasPrefix(data, []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return EncodingWebP
	case len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")):
		return sniffISOBrands(data)
	default:
		return EncodingUnknown
	}
}

// sniffISOBrands separates AVIF from HEIF by major brand, then by the
// compatible-brand list inside the ftyp box.
func sniffISOBrands(data []byte) Encoding {
	major := string(data[8:12])
	if avifBrands[major] {
		return EncodingAVIF
	}
	if heifBrands[major] {
		return EncodingHEIF
	}
	for off := 16; off+4 <= len(data); off += 4 {
		brand := string(data[off : off+4])
		if avifBrands[brand] {
			return EncodingAVIF
		}
		if heifBrands[brand] {
			return EncodingHEIF
		}
	}
	return EncodingUnknown
}

// ErrDecodable returns a named error when the container is recognised but has
// no pixel decoder in this build, and nil when pixels can be produced.
func (e Encoding) ErrDecodable() error {
	switch e {
	case EncodingAVIF, EncodingHEIF:
		return fmt.Errorf("%w: %s has no AV1/HEVC decoder wired (ledger P3.6 decision D1)",
			ErrUnsupportedFormat, e)
	default:
		return nil
	}
}

// IsDecodable reports whether pixels can be produced for this encoding.
func (e Encoding) IsDecodable() bool {
	return e != EncodingUnknown && e != EncodingAVIF && e != EncodingHEIF
}
