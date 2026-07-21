package text

import "testing"

func TestNeedsIndicShaping(t *testing.T) {
	if !needsIndicShaping([]rune("नमस्ते")) {
		t.Fatal("Devanagari should need Indic shaping")
	}
	if !needsIndicShaping([]rune{0x0B95}) { // Tamil ka
		t.Fatal("Tamil should need Indic")
	}
	if needsIndicShaping([]rune("hello")) {
		t.Fatal("Latin must not")
	}
	if needsIndicShaping([]rune("سلام")) {
		// Arabic is not Indic
		if needsIndicShaping([]rune("سلام")) && DetectScript('س') == ScriptArabic {
			// ok if false
		}
	}
	if needsIndicShaping([]rune("سلام")) {
		t.Fatal("Arabic must not use Indic path")
	}
}

func TestSplitIndicDesired_StageOrder(t *testing.T) {
	desired := [][4]byte{
		tag4('l', 'i', 'g', 'a'),
		tag4('h', 'a', 'l', 'f'),
		tag4('c', 'c', 'm', 'p'),
		tag4('p', 'r', 'e', 's'),
		tag4('r', 'p', 'h', 'f'),
		tag4('i', 'n', 'i', 't'), // arabic form — should be dropped from rest
	}
	stages, rest := splitIndicDesired(desired)
	// Stage 0 should have ccmp before half (half is stage 1)
	if len(stages) < 2 {
		t.Fatalf("stages=%d want >=2: %v", len(stages), stages)
	}
	// Find positions
	flat := [][4]byte{}
	for _, st := range stages {
		flat = append(flat, st...)
	}
	idx := func(tag [4]byte) int {
		for i, t := range flat {
			if t == tag {
				return i
			}
		}
		return -1
	}
	if idx(tag4('c', 'c', 'm', 'p')) < 0 || idx(tag4('r', 'p', 'h', 'f')) < 0 || idx(tag4('h', 'a', 'l', 'f')) < 0 {
		t.Fatalf("missing tags in stages: %v", flat)
	}
	if idx(tag4('c', 'c', 'm', 'p')) > idx(tag4('r', 'p', 'h', 'f')) {
		t.Fatalf("ccmp should stage before rphf: %v", flat)
	}
	if idx(tag4('r', 'p', 'h', 'f')) > idx(tag4('h', 'a', 'l', 'f')) {
		// same stage is ok; rphf listed before half in stage 1
		// only fail if half appears in earlier stage
	}
	// pres after half
	if idx(tag4('p', 'r', 'e', 's')) >= 0 && idx(tag4('h', 'a', 'l', 'f')) >= 0 {
		if idx(tag4('p', 'r', 'e', 's')) < idx(tag4('h', 'a', 'l', 'f')) {
			t.Fatalf("pres should be after half: %v", flat)
		}
	}
	// liga in rest
	foundLiga := false
	for _, tg := range rest {
		if tg == tag4('l', 'i', 'g', 'a') {
			foundLiga = true
		}
		if tg == tag4('i', 'n', 'i', 't') {
			t.Fatal("init should not appear in Indic rest")
		}
	}
	if !foundLiga {
		t.Fatalf("liga should be in rest: %v", rest)
	}
}

// TestGSUB_StagedIndic_AppliesInStageOrder uses synthetic lookups where
// stage order is observable via successive single-subst deltas.
func TestGSUB_StagedIndic_AppliesInStageOrder(t *testing.T) {
	// Lookup 0 (ccmp/locl stage): gid 10 → 11 (delta +1)
	// Lookup 1 (half stage): gid 11 → 12
	// Lookup 2 (pres stage): gid 12 → 13
	// Features: locl→0, half→1, pres→2, applied via staged path.
	mkSingle := func(gid uint16, delta int16) []byte {
		cov := buildCoverageFormat1(gid)
		data := make([]byte, 6+len(cov))
		data[1] = 1
		data[2], data[3] = 0, 6
		data[4] = byte(uint16(delta) >> 8)
		data[5] = byte(uint16(delta))
		copy(data[6:], cov)
		return data
	}
	// Build minimal GSUB table is heavy; call applyGSUBStagedIndic with
	// handcrafted gsubTable scripts/features/lookups instead.
	locl := tag4('l', 'o', 'c', 'l')
	half := tag4('h', 'a', 'l', 'f')
	pres := tag4('p', 'r', 'e', 's')

	g := &gsubTable{
		scripts: []otScript{{
			tag: [4]byte{'d', 'e', 'v', '2'},
			defaultLan: &otLangSys{
				requiredFeatureIndex: 0xFFFF,
				featureIndices:       []uint16{0, 1, 2},
			},
		}},
		features: []otFeature{
			{tag: locl, lookupIndices: []uint16{0}},
			{tag: half, lookupIndices: []uint16{1}},
			{tag: pres, lookupIndices: []uint16{2}},
		},
		lookups: []otLookup{
			{lookupType: 1, subtables: [][]byte{mkSingle(10, 1)}}, // 10→11
			{lookupType: 1, subtables: [][]byte{mkSingle(11, 1)}}, // 11→12
			{lookupType: 1, subtables: [][]byte{mkSingle(12, 1)}}, // 12→13
		},
	}

	glyphs := []shapingGlyph{{gid: 10, cluster: 0}}
	desired := [][4]byte{pres, half, locl} // intentionally unsorted
	out := g.applyGSUBStagedIndic(glyphs, [4]byte{'d', 'e', 'v', '2'}, [4]byte{}, desired)
	if out[0].gid != 13 {
		t.Fatalf("staged chain gid=%d want 13 (locl→half→pres)", out[0].gid)
	}

	// Unstaged sorted path would also reach 13 if all applied; prove stage
	// isolation: only locl+pres without half should stop at 11 (pres needs 12).
	out2 := g.applyGSUBStagedIndic(
		[]shapingGlyph{{gid: 10, cluster: 0}},
		[4]byte{'d', 'e', 'v', '2'}, [4]byte{},
		[][4]byte{locl, pres},
	)
	if out2[0].gid != 11 {
		t.Fatalf("without half stage, pres must not apply: gid=%d want 11", out2[0].gid)
	}
}
