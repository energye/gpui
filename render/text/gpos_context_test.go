package text

import "testing"

// buildSinglePosFormat1 builds GPOS Type 1 Format 1: coverage + valueFormat xAdvance.
// All covered glyphs get xAdvance delta.
func buildSinglePosFormat1(gid uint16, xAdv int16) []byte {
	cov := buildCoverageFormat1(gid)
	// format=1, covOff=6+ later, valueFormat=0x0004 (xAdvance), value, then cov
	// header: format(2) covOff(2) valueFormat(2) xAdvance(2) = 8, then cov
	header := []byte{
		0, 1, // format 1
		0, 8, // coverageOffset = 8
		0, 4, // valueFormat = X_ADVANCE
		byte(uint16(xAdv) >> 8), byte(uint16(xAdv)),
	}
	return append(header, cov...)
}

// TestGPOS_ContextFormat3_NestedSinglePos: when sequence [10,11] matches,
// nested Type 1 adds +50 xAdvance to glyph 10 (sequenceIndex 0).
func TestGPOS_ContextFormat3_NestedSinglePos(t *testing.T) {
	nested := buildSinglePosFormat1(10, 50)

	cov0 := buildCoverageFormat1(10)
	cov1 := buildCoverageFormat1(11)

	// Context Format 3:
	// format=3, glyphCount=2, posCount=1
	// coverageOffset[0], coverageOffset[1]
	// PosLookupRecord{seq=0, lookup=0}
	// then cov0, cov1
	header := []byte{
		0, 3, // format 3
		0, 2, // glyphCount
		0, 1, // posCount
		0, 14, // coverageOffset[0]
		0, 0, // coverageOffset[1] filled
		0, 0, // sequenceIndex
		0, 0, // lookupListIndex
	}
	cov1Off := 14 + len(cov0)
	header[8] = byte(cov1Off >> 8)
	header[9] = byte(cov1Off)
	ctx := append(header, cov0...)
	ctx = append(ctx, cov1...)

	g := &gposTable{
		lookups: []otLookup{
			{lookupType: 1, subtables: [][]byte{nested}},
			{lookupType: 7, subtables: [][]byte{ctx}},
		},
	}

	glyphs := []shapingGlyph{
		{gid: 5}, {gid: 10}, {gid: 11}, {gid: 20},
	}
	adj := make([]gposAdjustment, len(glyphs))
	g.applyLookup(&g.lookups[1], glyphs, adj, gposMetrics{})

	if adj[0].xAdvance != 0 || adj[3].xAdvance != 0 {
		t.Fatalf("unrelated glyphs got advance: %+v", adj)
	}
	if adj[1].xAdvance != 50 {
		t.Fatalf("contextual pos: xAdvance=%d want 50", adj[1].xAdvance)
	}
	if adj[2].xAdvance != 0 {
		t.Fatalf("second input glyph should not get nested pos unless recorded: %d", adj[2].xAdvance)
	}
}

// TestGPOS_ChainFormat3_Lookahead: input [10] with lookahead [11] → +30 xAdvance on 10.
func TestGPOS_ChainFormat3_Lookahead(t *testing.T) {
	nested := buildSinglePosFormat1(10, 30)
	inputCov := buildCoverageFormat1(10)
	lookCov := buildCoverageFormat1(11)

	// Chain Format 3 header (same as GSUB chain format 3):
	// format backtrackCount=0 inputCount=1 inputOff lookaheadCount=1 lookOff
	// posCount=1 seq=0 lookup=0
	header := make([]byte, 18)
	header[1] = 3
	header[5] = 1 // inputCount
	header[6], header[7] = 0, 18
	header[9] = 1 // lookaheadCount
	lookOff := 18 + len(inputCov)
	header[10] = byte(lookOff >> 8)
	header[11] = byte(lookOff)
	header[13] = 1 // posCount
	chain := append(header, inputCov...)
	chain = append(chain, lookCov...)

	g := &gposTable{
		lookups: []otLookup{
			{lookupType: 1, subtables: [][]byte{nested}},
			{lookupType: 8, subtables: [][]byte{chain}},
		},
	}

	adj := make([]gposAdjustment, 2)
	g.applyLookup(&g.lookups[1], []shapingGlyph{{gid: 10}, {gid: 11}}, adj, gposMetrics{})
	if adj[0].xAdvance != 30 {
		t.Fatalf("chain lookahead: xAdvance=%d want 30", adj[0].xAdvance)
	}

	adj2 := make([]gposAdjustment, 2)
	g.applyLookup(&g.lookups[1], []shapingGlyph{{gid: 10}, {gid: 12}}, adj2, gposMetrics{})
	if adj2[0].xAdvance != 0 {
		t.Fatalf("without lookahead should not fire: %d", adj2[0].xAdvance)
	}
}
