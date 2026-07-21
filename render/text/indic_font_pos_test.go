package text

import (
	"encoding/binary"
	"testing"
)

// buildLookupType1Coverage builds a minimal GSUB Type 1 Format 1 subtable
// covering the given glyphs (delta 0 — coverage harvest only).
func buildLookupType1Coverage(glyphs ...uint16) []byte {
	cov := buildCoverageFormat1(glyphs...)
	// format=1, covOffset=6, delta=0, then coverage
	out := make([]byte, 6+len(cov))
	out[1] = 1
	binary.BigEndian.PutUint16(out[2:4], 6)
	// delta 0
	copy(out[6:], cov)
	return out
}

func TestBuildIndicFontPosClasses_BlwfPstf(t *testing.T) {
	// Synthetic GSUB: one script DFLT, feature blwf lookup0, pstf lookup1.
	blwfST := buildLookupType1Coverage(50, 51)
	pstfST := buildLookupType1Coverage(60)

	// Lookup list: two lookups type1
	// Each lookup: type=1, flag=0, subtableCount=1, stOff=8, then subtable at 8
	mkLookup := func(st []byte) []byte {
		lu := make([]byte, 8+len(st))
		binary.BigEndian.PutUint16(lu[0:2], 1) // type
		binary.BigEndian.PutUint16(lu[4:6], 1) // subtableCount
		binary.BigEndian.PutUint16(lu[6:8], 8) // st offset
		copy(lu[8:], st)
		return lu
	}
	lu0 := mkLookup(blwfST)
	lu1 := mkLookup(pstfST)

	// LookupList: count=2, off0=6, off1=6+len(lu0)
	ll := make([]byte, 6+len(lu0)+len(lu1))
	binary.BigEndian.PutUint16(ll[0:2], 2)
	binary.BigEndian.PutUint16(ll[2:4], 6)
	binary.BigEndian.PutUint16(ll[4:6], uint16(6+len(lu0)))
	copy(ll[6:], lu0)
	copy(ll[6+len(lu0):], lu1)

	// FeatureList: count=2
	// records at 2: tag+offset each 6 bytes → offsets relative to FeatureList start
	// Feature0 body after records: 2+2*6=14
	feat0Body := []byte{0, 0, 0, 1, 0, 0} // lookup 0
	feat1Body := []byte{0, 0, 0, 1, 0, 1} // lookup 1
	fl := make([]byte, 2+12+len(feat0Body)+len(feat1Body))
	binary.BigEndian.PutUint16(fl[0:2], 2)
	copy(fl[2:6], []byte("blwf"))
	binary.BigEndian.PutUint16(fl[6:8], 14)
	copy(fl[8:12], []byte("pstf"))
	binary.BigEndian.PutUint16(fl[12:14], uint16(14+len(feat0Body)))
	copy(fl[14:], feat0Body)
	copy(fl[14+len(feat0Body):], feat1Body)

	// ScriptList: DFLT → defaultLangSys with both features
	// LangSys: lookupOrder=0, req=0xFFFF, featureCount=2, feat indices 0,1
	ls := []byte{0, 0, 0xFF, 0xFF, 0, 2, 0, 0, 0, 1}
	// Script table: defaultLangSys offset=4, langSysCount=0
	script := make([]byte, 4+len(ls))
	binary.BigEndian.PutUint16(script[0:2], 4)
	copy(script[4:], ls)
	// ScriptList: count=1, tag DFLT, offset=8
	sl := make([]byte, 8+len(script))
	binary.BigEndian.PutUint16(sl[0:2], 1)
	copy(sl[2:6], []byte("DFLT"))
	binary.BigEndian.PutUint16(sl[6:8], 8)
	copy(sl[8:], script)

	// GSUB header: version 1.0, scriptList, featureList, lookupList
	// offsets relative to GSUB start; header 10 bytes
	headerSize := 10
	gsub := make([]byte, headerSize+len(sl)+len(fl)+len(ll))
	binary.BigEndian.PutUint16(gsub[0:2], 1) // major
	binary.BigEndian.PutUint16(gsub[2:4], 0)
	binary.BigEndian.PutUint16(gsub[4:6], uint16(headerSize))
	binary.BigEndian.PutUint16(gsub[6:8], uint16(headerSize+len(sl)))
	binary.BigEndian.PutUint16(gsub[8:10], uint16(headerSize+len(sl)+len(fl)))
	copy(gsub[headerSize:], sl)
	copy(gsub[headerSize+len(sl):], fl)
	copy(gsub[headerSize+len(sl)+len(fl):], ll)

	g := parseGSUB(gsub)
	if g == nil {
		t.Fatal("parseGSUB failed")
	}
	dflt := [4]byte{'D', 'F', 'L', 'T'}
	fp := buildIndicFontPosClasses(g, dflt, dflt)
	if fp == nil {
		t.Fatal("expected font pos classes")
	}
	if fp.consonantPosFor(50, 0) != consPosBelow || fp.consonantPosFor(51, 0) != consPosBelow {
		t.Fatalf("blwf: pos map=%v", fp.pos)
	}
	if fp.consonantPosFor(60, 0) != consPosPost {
		t.Fatalf("pstf: pos map=%v", fp.pos)
	}
	// Unknown glyph falls back to static (Ka = base)
	if fp.consonantPosFor(999, 0x0915) != consPosBase {
		t.Fatal("fallback static")
	}
}

func TestBuildIndicFontPosClasses_RkrfVatu(t *testing.T) {
	// Same synthetic layout as BlwfPstf but tags rkrf/vatu → below.
	st := buildLookupType1Coverage(70)
	mkLookup := func(sub []byte) []byte {
		lu := make([]byte, 8+len(sub))
		binary.BigEndian.PutUint16(lu[0:2], 1)
		binary.BigEndian.PutUint16(lu[4:6], 1)
		binary.BigEndian.PutUint16(lu[6:8], 8)
		copy(lu[8:], sub)
		return lu
	}
	lu0 := mkLookup(st)
	ll := make([]byte, 4+len(lu0))
	binary.BigEndian.PutUint16(ll[0:2], 1)
	binary.BigEndian.PutUint16(ll[2:4], 4)
	copy(ll[4:], lu0)

	featBody := []byte{0, 0, 0, 1, 0, 0}
	// FeatureList with both rkrf and vatu pointing to same lookup
	fl := make([]byte, 2+12+len(featBody)*2)
	binary.BigEndian.PutUint16(fl[0:2], 2)
	copy(fl[2:6], []byte("rkrf"))
	binary.BigEndian.PutUint16(fl[6:8], 14)
	copy(fl[8:12], []byte("vatu"))
	binary.BigEndian.PutUint16(fl[12:14], uint16(14+len(featBody)))
	copy(fl[14:], featBody)
	copy(fl[14+len(featBody):], featBody)

	ls := []byte{0, 0, 0xFF, 0xFF, 0, 2, 0, 0, 0, 1}
	script := make([]byte, 4+len(ls))
	binary.BigEndian.PutUint16(script[0:2], 4)
	copy(script[4:], ls)
	sl := make([]byte, 8+len(script))
	binary.BigEndian.PutUint16(sl[0:2], 1)
	copy(sl[2:6], []byte("DFLT"))
	binary.BigEndian.PutUint16(sl[6:8], 8)
	copy(sl[8:], script)

	headerSize := 10
	gsub := make([]byte, headerSize+len(sl)+len(fl)+len(ll))
	binary.BigEndian.PutUint16(gsub[0:2], 1)
	binary.BigEndian.PutUint16(gsub[4:6], uint16(headerSize))
	binary.BigEndian.PutUint16(gsub[6:8], uint16(headerSize+len(sl)))
	binary.BigEndian.PutUint16(gsub[8:10], uint16(headerSize+len(sl)+len(fl)))
	copy(gsub[headerSize:], sl)
	copy(gsub[headerSize+len(sl):], fl)
	copy(gsub[headerSize+len(sl)+len(fl):], ll)

	g := parseGSUB(gsub)
	if g == nil {
		t.Fatal("parseGSUB")
	}
	dflt := [4]byte{'D', 'F', 'L', 'T'}
	fp := buildIndicFontPosClasses(g, dflt, dflt)
	if fp == nil || fp.consonantPosFor(70, 0) != consPosBelow {
		t.Fatalf("rkrf/vatu want below: %+v", fp)
	}
}

func TestReorderFinal_FontPosOverridesStatic(t *testing.T) {
	// Font marks glyph for cluster of Ta as below even though Ta is static base.
	// Syllable: Ka + Halant + Ta with Ta font-pos below → below bucket after base.
	runes := []rune{0x0915, devaHalant, 0x0924}
	gs := []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 11, cluster: 1},
		{gid: 12, cluster: 2}, // Ta mapped as below by font
	}
	fp := &indicFontPosClasses{pos: map[uint16]consPos{12: consPosBelow}}
	out := reorderIndicFinalGlyphsFont(gs, runes, fp)
	if out[0].cluster != 0 {
		t.Fatalf("base first: %v", clustersOf(out))
	}
	// virama + ta in below
	if out[1].cluster != 1 || out[2].cluster != 2 {
		t.Fatalf("below: %v", clustersOf(out))
	}
}

func TestFindBase_FontPosSkipsCoveredConsonant(t *testing.T) {
	// Ka + Halant + X where X is static base but font marks its glyph as below.
	// Base selection must prefer Ka when font pos is supplied.
	syl := []indicUnit{
		{r: 0x0915, orig: 0},
		{r: devaHalant, orig: 1},
		{r: 0x0924, orig: 2}, // Ta — static base
	}
	gids := []uint16{10, 11, 12}
	// Without font: base is Ta (index 2)
	if findIndicBaseIndexWithFont(syl, gids, nil) != 2 {
		t.Fatalf("static base want 2 got %d", findIndicBaseIndexWithFont(syl, gids, nil))
	}
	fp := &indicFontPosClasses{pos: map[uint16]consPos{12: consPosBelow}}
	if findIndicBaseIndexWithFont(syl, gids, fp) != 0 {
		t.Fatalf("font pos should skip Ta: base=%d", findIndicBaseIndexWithFont(syl, gids, fp))
	}
}
