//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// mkcolorfont generates the P3.8 batch-5 color-font testdata files by
// splicing color tables into a copy of goregular.ttf (real skeleton tables,
// no glyf surgery: color data keys on the existing gids of 'A' and 'B').
//
// Outputs (next to this directory's parent, i.e. render/text/testdata/):
//
//	p38_color_sbix.ttf   — sbix strike (ppem 32) on gid('A')
//	p38_color_svg.ttf    — "SVG " table: 'A' plain doc, 'B' gzip doc with
//	                       currentColor + var(--color0, fallback)
//	p38_color_colrv1.ttf — COLR v1 paint graph on 'A' (gradient base +
//	                       translated green 'B') + CPAL + ClipList
//	p38_color_colrv0.ttf — COLR v0 layers on 'A' + CPAL
//	p38_color_cbdt.ttf   — CBDT/CBLC bitmap on 'A'
//
// Run: go run ./render/text/testdata/mkcolorfont  (repo root cwd)
// The generator self-verifies: each output is re-parsed through the text
// package and the expected glyph types are asserted before exiting.
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/render/text/emoji"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mkcolorfont:", err)
		os.Exit(1)
	}
}

func run() error {
	raw, err := os.ReadFile("render/text/testdata/goregular.ttf")
	if err != nil {
		return err
	}
	tables, err := parseTableDirectory(raw)
	if err != nil {
		return fmt.Errorf("goregular: %w", err)
	}
	probe, err := text.NewFontSource(raw)
	if err != nil {
		return fmt.Errorf("probe font: %w", err)
	}
	parsed := probe.Parsed()
	gA := parsed.GlyphIndex('A')
	gB := parsed.GlyphIndex('B')
	upem := parsed.UnitsPerEm()
	fmt.Printf("goregular: gid(A)=%d gid(B)=%d upem=%d numGlyphs=%d\n", gA, gB, upem, parsed.NumGlyphs())

	const (
		sbixPath = "render/text/testdata/p38_color_sbix.ttf"
		svgPath  = "render/text/testdata/p38_color_svg.ttf"
		v1Path   = "render/text/testdata/p38_color_colrv1.ttf"
		v0Path   = "render/text/testdata/p38_color_colrv0.ttf"
		cbdtPath = "render/text/testdata/p38_color_cbdt.ttf"
	)
	numGlyphs := parsed.NumGlyphs()

	// sbix: one strike ppem 32, gid('A') → red square PNG.
	sbix, err := buildSbix(gA, numGlyphs, testPNG(32, color.RGBA{R: 220, G: 40, B: 40, A: 255}), 32)
	if err != nil {
		return err
	}
	if err := emit(sbixPath, tables, map[string][]byte{"sbix": sbix}); err != nil {
		return err
	}

	// SVG table: 'A' plain doc (hardcoded colors), 'B' gzip doc
	// (currentColor + var(--color0, fallback)).
	docA := []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg"><g id="glyph%d"><circle cx="500" cy="-400" r="400" fill="#e24a4a"/><rect x="100" y="-700" width="200" height="200" fill="#4a90e2"/></g></svg>`, gA))
	docB := []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg"><g id="glyph%d"><rect x="0" y="-800" width="900" height="900" fill="currentColor"/><rect x="100" y="-700" width="300" height="300" fill="var(--color0, #3ccf7a)"/></g></svg>`, gB))
	var gzB bytes.Buffer
	gz := gzip.NewWriter(&gzB)
	if _, err := gz.Write(docB); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	svgTable := buildSVGTable([]svgDoc{
		{start: gA, end: gA, doc: docA},
		{start: gB, end: gB, doc: gzB.Bytes()},
	})
	if err := emit(svgPath, tables, map[string][]byte{"SVG ": svgTable}); err != nil {
		return err
	}

	// COLR v1: base glyph 'A' — gradient 'A' + translated green 'B',
	// ClipList covering both, CPAL palette 0..3.
	cpal := buildCPAL([][4]uint8{
		{0, 0, 226, 255},    // 0: red   (BGRA)
		{0, 226, 60, 255},   // 1: green
		{226, 144, 74, 255}, // 2: blue
		{60, 192, 240, 255}, // 3: yellow
	})
	colrv1 := buildCOLRv1(gA, gB, upem)
	if err := emit(v1Path, tables, map[string][]byte{"COLR": colrv1, "CPAL": cpal}); err != nil {
		return err
	}

	// COLR v0: base glyph 'A' = [gid A red, gid B green].
	colrv0 := buildCOLRv0(gA, gB)
	if err := emit(v0Path, tables, map[string][]byte{"COLR": colrv0, "CPAL": cpal}); err != nil {
		return err
	}

	// CBDT/CBLC: gid('A') → 32×32 blue square.
	cbdt, cblc, err := buildCBDT(gA, 32, testPNG(32, color.RGBA{R: 40, G: 80, B: 220, A: 255}))
	if err != nil {
		return err
	}
	if err := emit(cbdtPath, tables, map[string][]byte{"CBDT": cbdt, "CBLC": cblc}); err != nil {
		return err
	}

	return verify(map[string]struct {
		path  string
		check func(text.ParsedFont) error
	}{
		"sbix":   {sbixPath, func(p text.ParsedFont) error { return wantType(p, 'A', text.GlyphTypeBitmap) }},
		"svg":    {svgPath, func(p text.ParsedFont) error { return wantType(p, 'B', text.GlyphTypeSVG) }},
		"colrv1": {v1Path, func(p text.ParsedFont) error { return wantType(p, 'A', text.GlyphTypeCOLR) }},
		"colrv0": {v0Path, func(p text.ParsedFont) error { return wantType(p, 'A', text.GlyphTypeCOLR) }},
		"cbdt":   {cbdtPath, func(p text.ParsedFont) error { return wantType(p, 'A', text.GlyphTypeBitmap) }},
	})
}

func wantType(p text.ParsedFont, r rune, want text.GlyphType) error {
	cf, ok := p.(text.ColorFont)
	if !ok {
		return fmt.Errorf("font is not a ColorFont")
	}
	gid := p.GlyphIndex(r)
	got := cf.GlyphType(gid)
	if got != want {
		return fmt.Errorf("glyph %c: type=%v want %v", r, got, want)
	}
	return nil
}

func verify(entries map[string]struct {
	path  string
	check func(text.ParsedFont) error
}) error {
	for name, e := range entries {
		src, err := text.NewFontSourceFromFile(e.path)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := e.check(src.Parsed()); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		fmt.Printf("%s: ok (%s)\n", name, e.path)
	}
	return nil
}

// --- font file assembly (table-directory surgery) ---

func parseTableDirectory(data []byte) (map[string][]byte, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("font too short")
	}
	num := int(binary.BigEndian.Uint16(data[4:6]))
	out := make(map[string][]byte, num)
	for i := 0; i < num; i++ {
		rec := 12 + i*16
		if rec+16 > len(data) {
			return nil, fmt.Errorf("record %d beyond data", i)
		}
		tag := string(data[rec : rec+4])
		off := int(binary.BigEndian.Uint32(data[rec+8 : rec+12]))
		length := int(binary.BigEndian.Uint32(data[rec+12 : rec+16]))
		if off+length > len(data) {
			return nil, fmt.Errorf("table %q beyond data", tag)
		}
		out[tag] = data[off : off+length]
	}
	return out, nil
}

func emit(path string, base map[string][]byte, extra map[string][]byte) error {
	all := make(map[string][]byte, len(base)+len(extra))
	for k, v := range base {
		all[k] = v
	}
	for k, v := range extra {
		all[k] = v
	}
	tags := make([]string, 0, len(all))
	for k := range all {
		tags = append(tags, k)
	}
	for i := 1; i < len(tags); i++ {
		for j := i; j > 0 && tags[j] < tags[j-1]; j-- {
			tags[j], tags[j-1] = tags[j-1], tags[j]
		}
	}
	num := len(tags)
	headLen := 12 + num*16
	var body bytes.Buffer
	offsets := make(map[string]uint32, num)
	for _, tag := range tags {
		off := headLen + body.Len()
		offsets[tag] = uint32(off)
		body.Write(all[tag])
		for body.Len()%4 != 0 {
			body.WriteByte(0)
		}
	}
	out := &bytes.Buffer{}
	binary.Write(out, binary.BigEndian, uint32(0x00010000))
	binary.Write(out, binary.BigEndian, uint16(num))
	binary.Write(out, binary.BigEndian, uint16(0)) // searchRange (parser ignores)
	binary.Write(out, binary.BigEndian, uint16(0))
	binary.Write(out, binary.BigEndian, uint16(0))
	for _, tag := range tags {
		out.WriteString(tag)
		binary.Write(out, binary.BigEndian, uint32(0)) // checksum (parser ignores)
		binary.Write(out, binary.BigEndian, offsets[tag])
		binary.Write(out, binary.BigEndian, uint32(len(all[tag])))
	}
	out.Write(body.Bytes())
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// --- color table builders (same layouts the unit tests pin) ---

func testPNG(size int, fill color.RGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := fill
			if x < 4 && y < 4 {
				c = color.RGBA{R: 60, G: 220, B: 60, A: 255} // orientation marker
			}
			img.SetNRGBA(x, y, color.NRGBA(c))
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func buildSbix(gid uint16, numGlyphs int, png []byte, ppem uint16) ([]byte, error) {
	entry := make([]byte, 8+len(png))
	binary.BigEndian.PutUint16(entry[0:2], 0)  // originOffsetX
	binary.BigEndian.PutUint16(entry[2:4], 32) // originOffsetY: ink above baseline
	copy(entry[4:8], "png ")
	copy(entry[8:], png)
	header := 8 + 4
	strikeOffsets := uint32(header)
	offsetsEnd := header + 4 + (numGlyphs+1)*4
	entryOff := uint32(offsetsEnd) - strikeOffsets

	buf := make([]byte, offsetsEnd+len(entry))
	binary.BigEndian.PutUint16(buf[0:2], 1) // version
	binary.BigEndian.PutUint32(buf[4:8], 1) // numStrikes
	binary.BigEndian.PutUint32(buf[8:12], strikeOffsets)
	binary.BigEndian.PutUint16(buf[strikeOffsets:strikeOffsets+2], ppem)
	binary.BigEndian.PutUint16(buf[strikeOffsets+2:strikeOffsets+4], 72)
	arr := strikeOffsets + 4
	for i := 0; i <= numGlyphs; i++ {
		var v uint32
		if uint16(i) == gid {
			v = entryOff
		} else if uint16(i) == gid+1 {
			v = entryOff + uint32(len(entry))
		}
		binary.BigEndian.PutUint32(buf[int(arr)+i*4:int(arr)+i*4+4], v)
	}
	copy(buf[offsetsEnd:], entry)
	return buf, nil
}

type svgDoc struct {
	start, end uint16
	doc        []byte
}

func buildSVGTable(docs []svgDoc) []byte {
	listLen := 2 + len(docs)*12
	listStart := uint32(10)
	buf := &bytes.Buffer{}
	write := binary.Write
	write(buf, binary.BigEndian, uint16(0)) // version
	write(buf, binary.BigEndian, listStart)
	write(buf, binary.BigEndian, uint32(0)) // reserved
	write(buf, binary.BigEndian, uint16(len(docs)))
	// Document slots follow the record array; offsets are list-relative.
	slots := make([]uint32, len(docs))
	next := uint32(listLen)
	for i, d := range docs {
		slots[i] = next
		next += uint32(len(d.doc))
	}
	for i, d := range docs {
		write(buf, binary.BigEndian, d.start)
		write(buf, binary.BigEndian, d.end)
		write(buf, binary.BigEndian, slots[i])
		write(buf, binary.BigEndian, uint32(len(d.doc)))
	}
	for _, d := range docs {
		buf.Write(d.doc)
	}
	return buf.Bytes()
}

func buildCPAL(entries [][4]uint8) []byte {
	buf := &bytes.Buffer{}
	write := binary.Write
	write(buf, binary.BigEndian, uint16(0))            // version
	write(buf, binary.BigEndian, uint16(len(entries))) // numEntries
	write(buf, binary.BigEndian, uint16(1))            // numPalettes
	write(buf, binary.BigEndian, uint16(len(entries))) // numColorRecords
	write(buf, binary.BigEndian, uint32(14))           // colorRecordsOffset
	write(buf, binary.BigEndian, uint16(0))            // palette 0 offset
	for _, c := range entries {
		buf.Write(c[:])
	}
	return buf.Bytes()
}

// buildCOLRv1 lays out: header(34) | BaseGlyphList | LayerList | ClipList |
// paints. 'A' root: PaintColrLayers[ PaintGlyph(A)→LinearGradient(red→blue),
// PaintTranslate(dx,dy)→PaintGlyph(B)→Solid(green) ]; ClipList covers the
// union (font units, y-up). Paint child offsets are Offset24 (3 bytes) —
// the same layout discipline the emoji package's unit tests pin.
func buildCOLRv1(gA, gB uint16, upem int) []byte {
	type patch struct {
		pos   int
		value uint32
		size  int // 3 or 4
	}
	buf := &bytes.Buffer{}
	var patches []patch
	w := binary.Write
	put32 := func(pos int, v uint32) { binary.BigEndian.PutUint32(buf.Bytes()[pos:pos+4], v) }
	put24 := func(pos int, v uint32) {
		buf.Bytes()[pos] = byte(v >> 16)
		buf.Bytes()[pos+1] = byte(v >> 8)
		buf.Bytes()[pos+2] = byte(v)
	}
	slot24 := func() int { // 3-byte rel slot anchored at the paint start
		pos := buf.Len()
		buf.Write([]byte{0, 0, 0})
		return pos
	}
	f14 := func(v float64) { w(buf, binary.BigEndian, uint16(int16(v*16384))) }

	// Header placeholder (34 bytes) — offsets patched at the end.
	buf.Write(make([]byte, 34))
	base := uint32(34)

	// BaseGlyphList.
	bgBase := uint32(buf.Len())
	w(buf, binary.BigEndian, uint32(1)) // numBaseGlyphPaintRecords
	w(buf, binary.BigEndian, gA)
	bgPaintSlot := buf.Len()
	w(buf, binary.BigEndian, uint32(0))
	patches = append(patches, patch{bgPaintSlot, 0, 4})

	// LayerList.
	llBase := uint32(buf.Len())
	w(buf, binary.BigEndian, uint32(2))
	e0 := buf.Len()
	w(buf, binary.BigEndian, uint32(0))
	e1 := buf.Len()
	w(buf, binary.BigEndian, uint32(0))
	patches = append(patches, patch{e0, 0, 4}, patch{e1, 0, 4})

	// ClipList: one box covering the union.
	clBase := uint32(buf.Len())
	buf.WriteByte(1) // format
	w(buf, binary.BigEndian, uint32(1))
	w(buf, binary.BigEndian, gA)
	w(buf, binary.BigEndian, gA)
	boxSlot := buf.Len()
	buf.Write([]byte{0, 0, 0})
	patches = append(patches, patch{boxSlot, 0, 3})
	boxBase := uint32(buf.Len())
	buf.WriteByte(1) // ClipBoxFormat1
	for _, v := range []int16{-100, int16(-upem), int16(2 * upem), int16(upem)} {
		w(buf, binary.BigEndian, uint16(v))
	}

	// Paints (forward layout: parents before children where links point
	// forward; the parser accepts any direction, siblings first).
	rootPos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintColrLayers)
	buf.WriteByte(2)
	w(buf, binary.BigEndian, uint32(0))

	layer0Pos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintGlyph)
	g0Slot := slot24()
	w(buf, binary.BigEndian, gA)

	gradPos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintLinearGradient)
	lineSlot := slot24()
	w(buf, binary.BigEndian, uint16(0))             // x0
	w(buf, binary.BigEndian, uint16(int16(-upem)))  // y0
	w(buf, binary.BigEndian, uint16(0))             // x1
	w(buf, binary.BigEndian, uint16(int16(upem)))   // y1
	w(buf, binary.BigEndian, uint16(int16(upem/2))) // x2
	w(buf, binary.BigEndian, uint16(int16(-upem)))  // y2
	linePos := uint32(buf.Len())
	buf.WriteByte(emoji.ExtendPad)
	w(buf, binary.BigEndian, uint16(2))
	w(buf, binary.BigEndian, uint16(0)) // stop0 offset 0
	w(buf, binary.BigEndian, uint16(0)) // palette 0 (red)
	f14(1.0)
	w(buf, binary.BigEndian, uint16(16384)) // stop1 offset 1
	w(buf, binary.BigEndian, uint16(2))     // palette 2 (blue)
	f14(1.0)

	layer1Pos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintTranslate)
	tSlot := slot24()
	w(buf, binary.BigEndian, uint16(int16(upem/4)))  // dx
	w(buf, binary.BigEndian, uint16(int16(-upem/4))) // dy

	glyph1Pos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintGlyph)
	g1Slot := slot24()
	w(buf, binary.BigEndian, gB)

	solidPos := uint32(buf.Len())
	buf.WriteByte(emoji.PaintSolid)
	w(buf, binary.BigEndian, uint16(1)) // palette 1 (green)
	f14(1.0)

	// Resolve relative offsets.
	for i := range patches {
		patches[i].value = 0 // placeholder; set below
	}
	put24(boxSlot, boxBase-clBase)
	put32(bgPaintSlot, rootPos-bgBase)
	put32(e0, layer0Pos-llBase)
	put32(e1, layer1Pos-llBase)
	put24(g0Slot, gradPos-layer0Pos)
	put24(lineSlot, linePos-gradPos)
	put24(tSlot, glyph1Pos-layer1Pos)
	put24(g1Slot, solidPos-glyph1Pos)

	// Header fields.
	put32(14, base)
	put32(18, llBase)
	put32(22, clBase)
	buf.Bytes()[1] = 1 // version 1
	_ = patches
	return buf.Bytes()
}

// buildCOLRv0: base 'A' = [gid A palette0, gid B palette1].
func buildCOLRv0(gA, gB uint16) []byte {
	buf := &bytes.Buffer{}
	w := binary.Write
	w(buf, binary.BigEndian, uint16(0)) // version
	w(buf, binary.BigEndian, uint16(1)) // numBaseGlyphRecords
	w(buf, binary.BigEndian, uint32(14))
	w(buf, binary.BigEndian, uint32(20))
	w(buf, binary.BigEndian, uint16(2))
	// BaseGlyphRecord
	w(buf, binary.BigEndian, gA)
	w(buf, binary.BigEndian, uint16(0)) // firstLayer
	w(buf, binary.BigEndian, uint16(2)) // numLayers
	// LayerRecords
	w(buf, binary.BigEndian, gA)
	w(buf, binary.BigEndian, uint16(0))
	w(buf, binary.BigEndian, gB)
	w(buf, binary.BigEndian, uint16(1))
	return buf.Bytes()
}

func buildCBDT(gid uint16, size int, png []byte) (cbdt, cblc []byte, err error) {
	// CBDT: dataLen(4) + PNG — imageFormat 19 carries metrics in CBLC.
	cbdt = make([]byte, 4+len(png))
	binary.BigEndian.PutUint32(cbdt[0:4], uint32(len(png)))
	copy(cbdt[4:], png)

	// CBLC: version 3.0, one BitmapSize record (48B at offset 8), an
	// IndexSubtableArray entry (8B at 56), and a format-2 index subtable
	// (constant metrics, 20B at 64): header(8) + imageSize(4) + bigMetrics(8).
	cblc = make([]byte, 84)
	binary.BigEndian.PutUint16(cblc[0:2], 3) // majorVersion
	binary.BigEndian.PutUint32(cblc[4:8], 1) // numSizes
	// BitmapSize record (at 8).
	binary.BigEndian.PutUint32(cblc[8:12], 56)  // indexSubtableListOffset
	binary.BigEndian.PutUint32(cblc[12:16], 28) // list size (8 + 20)
	binary.BigEndian.PutUint32(cblc[16:20], 1)  // numberOfIndexSubtables
	// record+40/42: start/end glyph; +44/45/46: ppemX/ppemY/bitDepth.
	binary.BigEndian.PutUint16(cblc[48:50], gid)
	binary.BigEndian.PutUint16(cblc[50:52], gid)
	cblc[52], cblc[53], cblc[54] = uint8(size), uint8(size), 32
	// IndexSubtableArray (at 56): first/last glyph + additionalOffset 8.
	binary.BigEndian.PutUint16(cblc[56:58], gid)
	binary.BigEndian.PutUint16(cblc[58:60], gid)
	binary.BigEndian.PutUint32(cblc[60:64], 8)
	// Index subtable (at 64): format 2, imageFormat 19, imageDataOffset 0.
	binary.BigEndian.PutUint16(cblc[64:66], 2)
	binary.BigEndian.PutUint16(cblc[66:68], 19)
	binary.BigEndian.PutUint32(cblc[68:72], 0)
	binary.BigEndian.PutUint32(cblc[72:76], uint32(4+len(png))) // imageSize
	// bigGlyphMetrics (at 76): height, width, horiBearingX, horiBearingY,
	// horiAdvance, vertBearingX, vertBearingY, vertAdvance.
	cblc[76] = uint8(size)
	cblc[77] = uint8(size)
	cblc[78] = 0
	cblc[79] = uint8(size) // bitmap top sits `size` px above the baseline
	cblc[80] = uint8(size)
	return cbdt, cblc, nil
}
