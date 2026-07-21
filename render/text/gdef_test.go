package text

import (
	"encoding/binary"
	"testing"
)

// buildClassDefFormat1 builds ClassDef format 1 for consecutive glyphs.
func buildClassDefFormat1(startGID uint16, classes []uint16) []byte {
	out := make([]byte, 6+2*len(classes))
	out[1] = 1 // format
	binary.BigEndian.PutUint16(out[2:4], startGID)
	binary.BigEndian.PutUint16(out[4:6], uint16(len(classes)))
	for i, c := range classes {
		binary.BigEndian.PutUint16(out[6+i*2:8+i*2], c)
	}
	return out
}

func putU16(b []byte, v uint16) []byte {
	return append(b, byte(v>>8), byte(v))
}

// buildClassDefFormat2 builds ClassDef format 2 from (start,end,class) triples.
func buildClassDefFormat2(ranges [][3]uint16) []byte {
	out := []byte{0, 2} // format 2
	out = putU16(out, uint16(len(ranges)))
	for _, r := range ranges {
		out = putU16(out, r[0])
		out = putU16(out, r[1])
		out = putU16(out, r[2])
	}
	return out
}

func TestGDEF_ParseAndIgnoreMarks(t *testing.T) {
	cd := buildClassDefFormat1(10, []uint16{gdefClassBase, gdefClassMark, gdefClassBase})
	header := make([]byte, 12)
	header[1] = 1 // major
	binary.BigEndian.PutUint16(header[4:6], 12)
	data := append(header, cd...)
	gdef := parseGDEF(data)
	if gdef == nil || gdef.classDef == nil {
		t.Fatal("parseGDEF failed")
	}
	if !gdef.isMark(11) || !gdef.isBase(10) {
		t.Fatalf("classes base10=%d mark11=%d", gdef.classOf(10), gdef.classOf(11))
	}
	if !gdef.ignoredByLookupFlag(11, lookupFlagIgnoreMarks, -1) {
		t.Fatal("IgnoreMarks should ignore mark 11")
	}
	if gdef.ignoredByLookupFlag(10, lookupFlagIgnoreMarks, -1) {
		t.Fatal("IgnoreMarks should not ignore base 10")
	}
}

// TestGSUB_Ligature_IgnoreMarks: f + mark + i → fi ligature, mark retained.
func TestGSUB_Ligature_IgnoreMarks(t *testing.T) {
	data := []byte{
		0, 1, // format 1
		0, 18, // coverageOffset
		0, 1, // ligSetCount
		0, 8, // ligSetOffset
		0, 1, // ligCount
		0, 4, // ligOffset
		0, 100, // ligGlyph
		0, 2, // compCount
		0, 11, // component
		// Coverage at 18:
		0, 1,
		0, 1,
		0, 10,
	}
	cd2 := buildClassDefFormat2([][3]uint16{
		{10, 10, gdefClassBase},
		{11, 11, gdefClassBase},
		{99, 99, gdefClassMark},
	})
	glyphs := []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 99, cluster: 1},
		{gid: 11, cluster: 2},
	}
	// No flag → no match (11 not immediately after 10)
	out := applyLigatureSubstFlag(data, append([]shapingGlyph(nil), glyphs...), 0, nil, -1)
	if len(out) != 3 {
		t.Fatalf("no-flag should not ligate across mark: len=%d", len(out))
	}

	gdef := &gdefTable{classDef: parseClassDef(cd2)}
	out2 := applyLigatureSubstFlag(data, append([]shapingGlyph(nil), glyphs...), lookupFlagIgnoreMarks, gdef, -1)
	// Expect: lig 100 at 0, mark 99 kept → [100, 99]
	if len(out2) != 2 {
		t.Fatalf("IgnoreMarks ligature len=%d want 2: %+v", len(out2), out2)
	}
	if out2[0].gid != 100 {
		t.Fatalf("lig gid=%d want 100", out2[0].gid)
	}
	if out2[1].gid != 99 {
		t.Fatalf("mark retained gid=%d want 99", out2[1].gid)
	}
}

func TestNextMatchIndex_SkipMarks(t *testing.T) {
	cd2 := buildClassDefFormat2([][3]uint16{
		{1, 1, gdefClassBase},
		{2, 2, gdefClassMark},
	})
	gdef := &gdefTable{classDef: parseClassDef(cd2)}
	glyphs := []shapingGlyph{{gid: 1}, {gid: 2}, {gid: 1}}
	j := nextMatchIndex(glyphs, 1, lookupFlagIgnoreMarks, gdef, -1)
	if j != 2 {
		t.Fatalf("nextMatchIndex=%d want 2", j)
	}
}

func TestGDEF_MarkAttachmentType(t *testing.T) {
	// Glyph class: 20,21 marks; MarkAttachClass: 20→class1, 21→class2
	// ClassDef format 2 ranges must be sorted by glyph ID (end) for binary search.
	glyphCD := buildClassDefFormat2([][3]uint16{
		{10, 10, gdefClassBase},
		{20, 21, gdefClassMark},
	})
	markCD := buildClassDefFormat2([][3]uint16{
		{20, 20, 1},
		{21, 21, 2},
	})
	// GDEF header 12 bytes + glyphCD + markCD
	// glyphClass at 12, markAttach at 12+len(glyphCD)
	header := make([]byte, 12)
	header[1] = 1
	binary.BigEndian.PutUint16(header[4:6], 12)
	markOff := 12 + len(glyphCD)
	binary.BigEndian.PutUint16(header[10:12], uint16(markOff))
	data := append(header, glyphCD...)
	data = append(data, markCD...)
	gdef := parseGDEF(data)
	if gdef == nil || gdef.markAttachCD == nil {
		t.Fatal("expected markAttachCD")
	}
	// Flag MarkAttachmentType=1 (bits 8-15 = 1): ignore marks not class 1
	flag := uint16(1 << 8)
	if gdef.ignoredByLookupFlag(20, flag, -1) {
		t.Fatal("mark class1 should not be ignored")
	}
	if !gdef.ignoredByLookupFlag(21, flag, -1) {
		t.Fatal("mark class2 should be ignored when type=1")
	}
	if gdef.ignoredByLookupFlag(10, flag, -1) {
		t.Fatal("base should not be ignored by MarkAttachmentType alone")
	}
}

// TestGDEF_MarkFilteringSet: UseMarkFilteringSet (flag bit 4) ignores marks not
// listed in GDEF MarkGlyphSets[setIndex].
func TestGDEF_MarkFilteringSet(t *testing.T) {
	// Glyph classes: 10 base, 20 mark (in set 0), 21 mark (not in set 0)
	glyphCD := buildClassDefFormat2([][3]uint16{
		{10, 10, gdefClassBase},
		{20, 21, gdefClassMark},
	})
	// Coverage format 1: only glyph 20
	cov := buildCoverageFormat1(20)
	// MarkGlyphSets table at start of sets blob:
	// format=1, count=1, Offset32 coverageOffset=8 (after 4+4 header)
	sets := make([]byte, 8+len(cov))
	sets[1] = 1 // format
	binary.BigEndian.PutUint16(sets[2:4], 1)
	binary.BigEndian.PutUint32(sets[4:8], 8)
	copy(sets[8:], cov)

	// GDEF 1.2 header: 14 bytes (major/minor/glyphClass/attach/ligCaret/markAttach/markGlyphSets)
	header := make([]byte, 14)
	header[1] = 1 // major
	header[3] = 2 // minor ≥ 2 for MarkGlyphSetsDef
	binary.BigEndian.PutUint16(header[4:6], 14)
	markSetsOff := 14 + len(glyphCD)
	binary.BigEndian.PutUint16(header[12:14], uint16(markSetsOff))

	data := append(header, glyphCD...)
	data = append(data, sets...)
	gdef := parseGDEF(data)
	if gdef == nil || len(gdef.markGlyphSets) != 1 {
		t.Fatalf("expected 1 mark glyph set, got gdef=%v sets=%d", gdef != nil, len(gdef.markGlyphSets))
	}
	if !gdef.markInFilteringSet(20, 0) || gdef.markInFilteringSet(21, 0) {
		t.Fatal("set 0 should contain 20 only")
	}

	flag := lookupFlagUseMarkFilteringSet
	// Mark 20 is in set → not ignored; mark 21 not in set → ignored
	if gdef.ignoredByLookupFlag(20, flag, 0) {
		t.Fatal("mark 20 in filtering set should not be ignored")
	}
	if !gdef.ignoredByLookupFlag(21, flag, 0) {
		t.Fatal("mark 21 not in filtering set should be ignored")
	}
	if gdef.ignoredByLookupFlag(10, flag, 0) {
		t.Fatal("base should not be ignored by MarkFilteringSet")
	}
	// Without UseMarkFilteringSet bit, set index is irrelevant
	if gdef.ignoredByLookupFlag(21, 0, 0) {
		t.Fatal("no flag → mark 21 not ignored")
	}
}

// TestParseLookup_MarkFilteringSet reads MarkFilteringSet uint16 after subtable offsets.
func TestParseLookup_MarkFilteringSet(t *testing.T) {
	// Lookup: type=1, flag=UseMarkFilteringSet, subtableCount=1, stOff=10, markFilterSet=3
	// header 6 + 2 (subtable) + 2 (filter) = 10; subtable starts at 10 (empty ok for parse)
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 1) // type
	binary.BigEndian.PutUint16(data[2:4], lookupFlagUseMarkFilteringSet)
	binary.BigEndian.PutUint16(data[4:6], 1) // subtableCount
	binary.BigEndian.PutUint16(data[6:8], 10)
	binary.BigEndian.PutUint16(data[8:10], 3) // markFilterSet
	lu := parseLookupTable(data)
	if lu.markFilterSet != 3 {
		t.Fatalf("markFilterSet=%d want 3", lu.markFilterSet)
	}
	if lu.lookupFlag&lookupFlagUseMarkFilteringSet == 0 {
		t.Fatal("flag bit missing")
	}
	// Without flag, markFilterSet stays -1
	data2 := make([]byte, 8)
	binary.BigEndian.PutUint16(data2[0:2], 1)
	binary.BigEndian.PutUint16(data2[4:6], 1)
	binary.BigEndian.PutUint16(data2[6:8], 8)
	lu2 := parseLookupTable(data2)
	if lu2.markFilterSet != -1 {
		t.Fatalf("default markFilterSet=%d want -1", lu2.markFilterSet)
	}
}
