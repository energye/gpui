//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package emoji

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"io"
)

// SVG table format errors (OpenType 1.9.1 SVG table).
var (
	// ErrNoSVGTable indicates the font doesn't have an SVG table.
	ErrNoSVGTable = errors.New("emoji: font has no SVG table")

	// ErrInvalidSVGData indicates the SVG table data is malformed.
	ErrInvalidSVGData = errors.New("emoji: invalid SVG table data")

	// ErrGlyphNotInSVG indicates the glyph has no SVG document.
	ErrGlyphNotInSVG = errors.New("emoji: glyph not found in SVG table")
)

// svgDocRecord is one SVGDocumentRecord: a glyph ID range and where its
// document lives. svgDocOffset is relative to the start of the
// SVGDocumentList (not the table start).
type svgDocRecord struct {
	startGlyphID uint16
	endGlyphID   uint16
	docOffset    uint32
	docLength    uint32
}

// SVGTableParser parses the OpenType SVG table (OT 1.9.1): a document list
// mapping glyph ID ranges to UTF-8 (optionally gzip-encoded) SVG documents.
// Rendering of a document is not done here — the consumer resolves the
// element with id "glyph<N>" per the table's use-semantics contract.
type SVGTableParser struct {
	data       []byte
	listOffset uint32 // SVGDocumentList, relative to table start
	records    []svgDocRecord
}

// NewSVGTableParser creates a parser from the raw "SVG " table data.
// Structural violations (bad version, out-of-order records, records or
// documents beyond the table) fail closed with ErrInvalidSVGData.
func NewSVGTableParser(data []byte) (*SVGTableParser, error) {
	if len(data) == 0 {
		return nil, ErrNoSVGTable
	}
	if len(data) < 10 {
		return nil, ErrInvalidSVGData
	}
	version := binary.BigEndian.Uint16(data[0:2])
	if version != 0 {
		return nil, ErrInvalidSVGData
	}
	listOffset := binary.BigEndian.Uint32(data[2:6])
	if listOffset == 0 || int(listOffset)+2 > len(data) {
		return nil, ErrInvalidSVGData
	}

	p := &SVGTableParser{data: data, listOffset: listOffset}
	list := data[listOffset:]
	numEntries := binary.BigEndian.Uint16(list[0:2])
	recordsEnd := 2 + int(numEntries)*12
	if recordsEnd > len(list) {
		return nil, ErrInvalidSVGData
	}

	prevEnd := int64(-1)
	for i := 0; i < int(numEntries); i++ {
		pos := 2 + i*12
		rec := svgDocRecord{
			startGlyphID: binary.BigEndian.Uint16(list[pos : pos+2]),
			endGlyphID:   binary.BigEndian.Uint16(list[pos+2 : pos+4]),
			docOffset:    binary.BigEndian.Uint32(list[pos+4 : pos+8]),
			docLength:    binary.BigEndian.Uint32(list[pos+8 : pos+12]),
		}
		if rec.endGlyphID < rec.startGlyphID {
			return nil, ErrInvalidSVGData
		}
		if int64(rec.startGlyphID) <= prevEnd {
			// Records must be sorted in increasing startGlyphID order with
			// disjoint ranges (OT 1.9.1 SVG table).
			return nil, ErrInvalidSVGData
		}
		prevEnd = int64(rec.endGlyphID)
		if rec.docOffset == 0 || rec.docLength == 0 {
			return nil, ErrInvalidSVGData
		}
		// Document bounds relative to the SVGDocumentList start.
		if int64(rec.docOffset)+int64(rec.docLength) > int64(len(list)) {
			return nil, ErrInvalidSVGData
		}
		p.records = append(p.records, rec)
	}
	if len(p.records) == 0 {
		return nil, ErrInvalidSVGData
	}
	return p, nil
}

// HasGlyph reports whether the glyph is covered by a document record.
func (p *SVGTableParser) HasGlyph(glyphID uint16) bool {
	return p.findRecord(glyphID) != nil
}

// GlyphDocument returns the decoded (decompressed) UTF-8 SVG document for
// the glyph. Documents may be shared by several records; each call returns
// a fresh copy safe for the caller to retain.
func (p *SVGTableParser) GlyphDocument(glyphID uint16) ([]byte, error) {
	rec := p.findRecord(glyphID)
	if rec == nil {
		return nil, ErrGlyphNotInSVG
	}
	list := p.data[p.listOffset:]
	encoded := list[rec.docOffset : rec.docOffset+rec.docLength]
	if len(encoded) >= 3 && encoded[0] == 0x1F && encoded[1] == 0x8B && encoded[2] == 0x08 {
		zr, err := gzip.NewReader(bytes.NewReader(encoded))
		if err != nil {
			return nil, ErrInvalidSVGData
		}
		defer zr.Close()
		decoded, err := io.ReadAll(io.LimitReader(zr, maxSVGDocSize))
		if err != nil {
			return nil, ErrInvalidSVGData
		}
		return decoded, nil
	}
	out := make([]byte, len(encoded))
	copy(out, encoded)
	return out, nil
}

// findRecord finds the record covering glyphID (records are sorted and
// disjoint, so a linear scan is fine for the record count sizes seen in
// fonts; documents are the large part of the table, not the record list).
func (p *SVGTableParser) findRecord(glyphID uint16) *svgDocRecord {
	for i := range p.records {
		if glyphID >= p.records[i].startGlyphID && glyphID <= p.records[i].endGlyphID {
			return &p.records[i]
		}
	}
	return nil
}

// maxSVGDocSize bounds gzip bomb decompression: real glyph documents are a
// few KB; 1 MiB is far beyond any legitimate color glyph.
const maxSVGDocSize = 1 << 20

// SVGGlyph is the decoded SVG document for one SVG-table color glyph.
type SVGGlyph struct {
	// GlyphID is the glyph this document describes.
	GlyphID uint16

	// Doc is the decompressed UTF-8 SVG document. The element with
	// id "glyph<N>" is the glyph description (use-semantics rendering).
	Doc []byte

	// UsesCurrentColor is true when the document references the
	// currentColor keyword (the raster depends on the text color).
	UsesCurrentColor bool

	// UsesPalette is true when the document references CPAL palette
	// variables via var(--colorN, fallback).
	UsesPalette bool
}
