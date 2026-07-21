package text

import (
	"os"
	"path/filepath"
	"testing"
)

// buildCoverageFormat1 builds a Coverage format 1 table for the given glyphs
// (must be sorted ascending).
func buildCoverageFormat1(glyphs ...uint16) []byte {
	out := make([]byte, 4+2*len(glyphs))
	out[0], out[1] = 0, 1 // format 1
	out[2] = byte(len(glyphs) >> 8)
	out[3] = byte(len(glyphs))
	for i, g := range glyphs {
		out[4+i*2] = byte(g >> 8)
		out[4+i*2+1] = byte(g)
	}
	return out
}

// TestGSUB_ContextFormat3_NestedSingle applies Type 5 Format 3: when coverage
// sequence [10,11] matches, nested Type 1 replaces glyph 10 → 100.
func TestGSUB_ContextFormat3_NestedSingle(t *testing.T) {
	// Nested lookup 0: single subst format 1, coverage=[10], delta=+90 → 100
	nestedCov := buildCoverageFormat1(10)
	nested := make([]byte, 6+len(nestedCov))
	nested[1] = 1 // format 1
	nested[2], nested[3] = 0, 6
	nested[4], nested[5] = 0, 90 // delta +90
	copy(nested[6:], nestedCov)

	// Input coverages for sequence of 2 glyphs.
	cov0 := buildCoverageFormat1(10)
	cov1 := buildCoverageFormat1(11)

	// Context Format 3 layout:
	// format=3, glyphCount=2, substCount=1
	// coverageOffset[0], coverageOffset[1]
	// SubstLookupRecord{sequenceIndex=0, lookupListIndex=0}
	// then cov0, cov1
	//
	// offsets relative to start of context subtable:
	// header 6 + 2*2 cov offs + 4 record = 14; cov0 at 14; cov1 at 14+len(cov0)
	header := []byte{
		0, 3, // format 3
		0, 2, // glyphCount
		0, 1, // substCount
		0, 14, // coverageOffset[0]
		0, 0, // coverageOffset[1] filled below
		0, 0, // sequenceIndex
		0, 0, // lookupListIndex
	}
	cov1Off := 14 + len(cov0)
	header[8] = byte(cov1Off >> 8)
	header[9] = byte(cov1Off)
	ctx := append(header, cov0...)
	ctx = append(ctx, cov1...)

	g := &gsubTable{
		lookups: []otLookup{
			{lookupType: 1, subtables: [][]byte{nested}},
			{lookupType: 5, subtables: [][]byte{ctx}},
		},
	}

	glyphs := []shapingGlyph{
		{gid: 5, cluster: 0},
		{gid: 10, cluster: 1},
		{gid: 11, cluster: 2},
		{gid: 20, cluster: 3},
	}
	// Apply contextual lookup (index 1) only.
	out := g.applyLookup(&g.lookups[1], glyphs)
	if len(out) != 4 {
		t.Fatalf("len=%d want 4: %+v", len(out), out)
	}
	if out[0].gid != 5 || out[3].gid != 20 {
		t.Fatalf("unrelated glyphs changed: %+v", out)
	}
	if out[1].gid != 100 {
		t.Fatalf("contextual single subst: gid=%d want 100", out[1].gid)
	}
	if out[2].gid != 11 {
		t.Fatalf("second input glyph should stay 11, got %d", out[2].gid)
	}
}

// TestGSUB_ChainFormat3_Lookahead applies Type 6 Format 3: input [10] with
// lookahead [11] triggers single subst 10→100.
func TestGSUB_ChainFormat3_Lookahead(t *testing.T) {
	nestedCov := buildCoverageFormat1(10)
	nested := make([]byte, 6+len(nestedCov))
	nested[1] = 1
	nested[2], nested[3] = 0, 6
	nested[4], nested[5] = 0, 90
	copy(nested[6:], nestedCov)

	inputCov := buildCoverageFormat1(10)
	lookCov := buildCoverageFormat1(11)

	// Chain Format 3:
	// format=3
	// backtrackCount=0
	// inputCount=1, inputCoverageOffset[0]
	// lookaheadCount=1, lookaheadCoverageOffset[0]
	// substCount=1, record(seq=0, lookup=0)
	// then tables
	//
	// header size: 2 + 2 + (0) + 2 + 2 + 2 + 2 + 2 + 4 = 18
	// layout:
	// [0] format
	// [2] backtrackCount=0
	// [4] inputCount=1
	// [6] inputCovOff
	// [8] lookaheadCount=1
	// [10] lookCovOff
	// [12] substCount=1
	// [14] seqIndex=0
	// [16] lookup=0
	// [18] inputCov
	// ... lookCov
	header := make([]byte, 18)
	header[1] = 3 // format
	// backtrackCount already 0
	header[5] = 1 // inputCount
	header[6], header[7] = 0, 18
	header[9] = 1 // lookaheadCount
	// lookCovOff filled later
	header[13] = 1 // substCount
	// seq/lookup already 0
	lookOff := 18 + len(inputCov)
	header[10] = byte(lookOff >> 8)
	header[11] = byte(lookOff)
	chain := append(header, inputCov...)
	chain = append(chain, lookCov...)

	g := &gsubTable{
		lookups: []otLookup{
			{lookupType: 1, subtables: [][]byte{nested}},
			{lookupType: 6, subtables: [][]byte{chain}},
		},
	}

	// Should match: 10 then 11
	out := g.applyLookup(&g.lookups[1], []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 11, cluster: 1},
	})
	if out[0].gid != 100 {
		t.Fatalf("chain lookahead: gid=%d want 100", out[0].gid)
	}

	// No match without lookahead 11
	out2 := g.applyLookup(&g.lookups[1], []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 12, cluster: 1},
	})
	if out2[0].gid != 10 {
		t.Fatalf("no lookahead should not subst: gid=%d", out2[0].gid)
	}
}

// TestGSUB_ReverseChainFormat1 replaces covered glyph when backtrack matches.
func TestGSUB_ReverseChainFormat1(t *testing.T) {
	// coverage=[10] → substitute 100
	// backtrack coverage=[20] (glyph immediately before)
	// no lookahead
	cov := buildCoverageFormat1(10)
	back := buildCoverageFormat1(20)
	// layout:
	// format1, covOff, backtrackCount=1, backOff, lookaheadCount=0, glyphCount=1, substitute=100
	// then cov, back
	// header: 2+2+2+2+2+2+2 = 14, cov at 14, back at 14+len(cov)
	header := make([]byte, 14)
	header[1] = 1
	header[2], header[3] = 0, 14 // cov
	header[5] = 1                // backtrackCount
	backOff := 14 + len(cov)
	header[6] = byte(backOff >> 8)
	header[7] = byte(backOff)
	// lookaheadCount=0 at [8:10]
	header[11] = 1 // glyphCount
	header[12], header[13] = 0, 100
	data := append(header, cov...)
	data = append(data, back...)

	g := &gsubTable{}
	out := g.applyReverseChainingSubst(data, []shapingGlyph{
		{gid: 20, cluster: 0},
		{gid: 10, cluster: 1},
	})
	if out[1].gid != 100 {
		t.Fatalf("reverse chain: gid=%d want 100", out[1].gid)
	}
	// without backtrack
	out2 := g.applyReverseChainingSubst(data, []shapingGlyph{
		{gid: 30, cluster: 0},
		{gid: 10, cluster: 1},
	})
	if out2[1].gid != 10 {
		t.Fatalf("reverse without backtrack should not fire: %d", out2[1].gid)
	}
}

// TestGSUB_Type6_RealFont_Smoke ensures fonts known to carry Type 6 lookups
// still shape without panic and produce non-empty output after G1.c wiring.
func TestGSUB_Type6_RealFont_Smoke(t *testing.T) {
	paths := []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}
	var fontPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			fontPath = p
			break
		}
	}
	if fontPath == "" {
		t.Skip("no system font with Type 6 GSUB")
	}

	opts := []SourceOption{}
	if filepath.Ext(fontPath) == ".ttc" {
		opts = append(opts, WithCollectionIndex(0))
	}
	src, err := NewFontSourceFromFile(fontPath, opts...)
	if err != nil {
		t.Fatal(err)
	}
	face := src.Face(24)
	shaper := NewOwnShaper()
	// Latin + CJK samples exercise ccmp/liga and any chain rules present.
	samples := []string{"office", "ffi", "中文", "A.V"}
	for _, s := range samples {
		glyphs := shaper.Shape(s, face)
		if s != "" && len(glyphs) == 0 {
			t.Fatalf("Shape(%q) empty on %s", s, fontPath)
		}
		t.Logf("%s Shape(%q)=%d glyphs", filepath.Base(fontPath), s, len(glyphs))
	}
}
