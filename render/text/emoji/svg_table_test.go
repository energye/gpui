package emoji

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"testing"
)

// buildSVGTable assembles an SVG table from records (OT 1.9.1 layout:
// version(2) + listOffset(4) + reserved(4) → numEntries(2) +
// records{start(2) end(2) offset(4, list-relative) length(4)} + documents).
// Record r[2] selects the document by index into docs; document offsets are
// computed from the physical layout (always non-zero, per spec).
func buildSVGTable(t *testing.T, docs map[int][]byte, records [][4]uint32) []byte {
	t.Helper()
	listStart := uint32(10)
	listLen := uint32(2 + len(records)*12)

	// Assign each distinct document a slot after the record array.
	slots := make(map[int]uint32) // doc index → list-relative offset
	var layout [][]byte
	next := listLen
	for _, r := range records {
		idx := int(r[2])
		if _, ok := slots[idx]; ok {
			continue
		}
		doc := docs[idx]
		if doc == nil {
			t.Fatalf("record references missing doc %d", idx)
		}
		slots[idx] = next
		layout = append(layout, doc)
		next += uint32(len(doc))
	}

	buf := &bytes.Buffer{}
	writeU16(buf, 0)              // version 0
	writeU32(buf, listStart)      // svgDocumentListOffset
	buf.Write([]byte{0, 0, 0, 0}) // reserved
	writeU16(buf, uint16(len(records)))
	for _, r := range records { // start end offset length
		writeU16(buf, uint16(r[0]))
		writeU16(buf, uint16(r[1]))
		writeU32(buf, slots[int(r[2])])
		writeU32(buf, uint32(len(docs[int(r[2])])))
	}
	for _, doc := range layout {
		buf.Write(doc)
	}
	return buf.Bytes()
}

func writeU16(buf *bytes.Buffer, v uint16) {
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], v)
	buf.Write(b[:])
}

func writeU32(buf *bytes.Buffer, v uint32) {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	buf.Write(b[:])
}

const svgDocA = `<svg xmlns="http://www.w3.org/2000/svg"><g id="glyph1"><rect width="100" height="100" fill="red"/></g></svg>`

func TestSVGTable_PlainDocuments(t *testing.T) {
	doc := []byte(svgDocA)
	table := buildSVGTable(t, map[int][]byte{0: doc}, [][4]uint32{
		{1, 1, 0, 0},
		{5, 7, 0, 0}, // shared document, discontinuous range
	})
	p, err := NewSVGTableParser(table)
	if err != nil {
		t.Fatalf("NewSVGTableParser() error = %v", err)
	}
	if !p.HasGlyph(1) || !p.HasGlyph(5) || !p.HasGlyph(7) {
		t.Error("HasGlyph() = false for covered gids, want true")
	}
	if p.HasGlyph(2) || p.HasGlyph(0) || p.HasGlyph(8) {
		t.Error("HasGlyph() = true for uncovered gids, want false")
	}
	got, err := p.GlyphDocument(6)
	if err != nil {
		t.Fatalf("GlyphDocument(6) error = %v", err)
	}
	if string(got) != svgDocA {
		t.Errorf("GlyphDocument(6) = %q, want %q", got, svgDocA)
	}
	if _, err := p.GlyphDocument(2); err == nil {
		t.Error("GlyphDocument(2) error = nil, want not-found error")
	}
}

func TestSVGTable_GzipDocument(t *testing.T) {
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write([]byte(svgDocA)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	table := buildSVGTable(t, map[int][]byte{0: gz.Bytes()}, [][4]uint32{
		{3, 3, 0, 0},
	})
	p, err := NewSVGTableParser(table)
	if err != nil {
		t.Fatalf("NewSVGTableParser() error = %v", err)
	}
	got, err := p.GlyphDocument(3)
	if err != nil {
		t.Fatalf("GlyphDocument(3) error = %v", err)
	}
	if string(got) != svgDocA {
		t.Errorf("decompressed document = %q, want %q", got, svgDocA)
	}
}

func TestSVGTable_InvalidTables(t *testing.T) {
	cases := []struct {
		name  string
		table []byte
	}{
		{"empty", nil},
		{"truncated header", []byte{0, 0, 0, 0, 0}},
		{"bad version", func() []byte {
			tt := buildSVGTable(t, map[int][]byte{0: []byte(svgDocA)}, [][4]uint32{{1, 1, 0, 0}})
			binary.BigEndian.PutUint16(tt[0:2], 1)
			return tt
		}()},
		{"zero list offset", func() []byte {
			tt := buildSVGTable(t, map[int][]byte{0: []byte(svgDocA)}, [][4]uint32{{1, 1, 0, 0}})
			binary.BigEndian.PutUint32(tt[2:6], 0)
			return tt
		}()},
		{"record beyond data", func() []byte {
			tt := buildSVGTable(t, map[int][]byte{0: []byte(svgDocA)}, [][4]uint32{{1, 1, 0, 0}})
			binary.BigEndian.PutUint32(tt[10+2+8:10+2+12], 1<<20) // docLength huge
			return tt
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSVGTableParser(tc.table); err == nil {
				t.Errorf("NewSVGTableParser(%s) error = nil, want error", tc.name)
			}
		})
	}
}

func TestSVGTable_UnsortedRecords(t *testing.T) {
	doc := []byte(svgDocA)
	table := buildSVGTable(t, map[int][]byte{0: doc}, [][4]uint32{
		{5, 7, 0, 0},
		{1, 1, 0, 0}, // out of order
	})
	if _, err := NewSVGTableParser(table); err == nil {
		t.Error("NewSVGTableParser(unsorted) error = nil, want error")
	}
}
