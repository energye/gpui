package text

import "testing"

func TestIndicInitial_RephMovesAfterBase(t *testing.T) {
	// Ra + Halant + Ka → Ka + Ra + Halant (initial)
	runes := []rune{devaRa, devaHalant, 0x0915} // र् + क
	units := reorderIndicInitial(runes)
	got := indicUnitsToRunes(units)
	want := []rune{0x0915, devaRa, devaHalant}
	if string(got) != string(want) {
		t.Fatalf("initial reph: got %U want %U", got, want)
	}
	// clusters preserve original indices
	if units[0].orig != 2 || units[1].orig != 0 || units[2].orig != 1 {
		t.Fatalf("orig indices=%v want 2,0,1", []int{units[0].orig, units[1].orig, units[2].orig})
	}
}

func TestIndicInitial_NoRephWithoutBase(t *testing.T) {
	runes := []rune{devaRa, devaHalant} // incomplete
	units := reorderIndicInitial(runes)
	if string(indicUnitsToRunes(units)) != string(runes) {
		t.Fatalf("should not reorder incomplete reph")
	}
}

func TestIndicFinal_PreBaseMatraBeforeBase(t *testing.T) {
	// Logical: Ka + pre-base i matra (often stored after base in encoded text?
	// In Unicode, matra follows base: क + ि
	// Final reorder: matra before base for visual.
	runes := []rune{0x0915, devaIMatra} // क ि
	// glyphs as after cmap (logical order = runes)
	gs := []shapingGlyph{
		{gid: 100, cluster: 0}, // Ka
		{gid: 200, cluster: 1}, // matra
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	if out[0].cluster != 1 || out[1].cluster != 0 {
		t.Fatalf("pre-base final: clusters=%d,%d want matra then base", out[0].cluster, out[1].cluster)
	}
}

func TestIndicFinal_RephToEnd(t *testing.T) {
	// After initial reorder runes are Ka, Ra, Halant with orig 2,0,1
	// Glyphs follow that order with clusters = orig
	runes := []rune{devaRa, devaHalant, 0x0915}
	units := reorderIndicInitial(runes)
	gs := make([]shapingGlyph, len(units))
	for i, u := range units {
		gs[i] = shapingGlyph{gid: uint16(100 + i), cluster: u.orig}
	}
	// Final: reph (clusters 0,1) to end → base first then reph
	out := reorderIndicFinalGlyphs(gs, runes)
	// base cluster 2 first, then reph 0,1
	if out[0].cluster != 2 {
		t.Fatalf("base should lead: first cluster=%d", out[0].cluster)
	}
	if out[1].cluster != 0 || out[2].cluster != 1 {
		t.Fatalf("reph at end: clusters=%d,%d,%d", out[0].cluster, out[1].cluster, out[2].cluster)
	}
}

func TestIndicSyllableSplit_Simple(t *testing.T) {
	runes := []rune{0x0915, 0x0947, 0x0020, 0x0915} // के क
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	sylls := splitIndicSyllables(units)
	if len(sylls) != 3 {
		t.Fatalf("syllables=%d want 3 (के / space / क)", len(sylls))
	}
}

func TestIndicCategory_Basics(t *testing.T) {
	if indicCategory(devaRa) != icConsonant {
		t.Fatal("Ra consonant")
	}
	if indicCategory(devaHalant) != icVirama {
		t.Fatal("halant")
	}
	if indicCategory(devaIMatra) != icPreBaseMatra {
		t.Fatal("pre-base matra")
	}
}

func TestIndicCategory_MatraClasses(t *testing.T) {
	// Devanagari position classes
	if indicCategory(0x093F) != icPreBaseMatra { // ि
		t.Fatal("deva i pre")
	}
	if indicCategory(0x0947) != icAboveMatra { // े
		t.Fatal("deva e above")
	}
	if indicCategory(0x0941) != icBelowMatra { // ु
		t.Fatal("deva u below")
	}
	if indicCategory(0x093E) != icPostMatra { // ा
		t.Fatal("deva aa post")
	}
	if !indicCategory(0x0947).isMatra() || !indicCategory(0x093E).isMatra() {
		t.Fatal("isMatra covers above/post")
	}
	// Peer pre-base (Bengali/Gujarati/Tamil)
	for _, r := range []rune{0x09BF, 0x0ABF, 0x0BC6, 0x0D47} {
		if indicCategory(r) != icPreBaseMatra {
			t.Fatalf("%U want pre-base", r)
		}
	}
}

func TestIndicFinal_PeerPreBaseMatra(t *testing.T) {
	// Bengali: Ka + pre-base i → final visual matra before base
	runes := []rune{0x0995, 0x09BF} // ক ি
	gs := []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 20, cluster: 1},
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	if out[0].cluster != 1 || out[1].cluster != 0 {
		t.Fatalf("bengali pre-base: clusters=%d,%d", out[0].cluster, out[1].cluster)
	}
}

func TestIndicFinal_AboveMatraStaysWithBase(t *testing.T) {
	// Above/below matras are not pre-base: stay after base in final order.
	runes := []rune{0x0915, 0x0947} // के
	gs := []shapingGlyph{
		{gid: 10, cluster: 0},
		{gid: 20, cluster: 1},
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	if out[0].cluster != 0 || out[1].cluster != 1 {
		t.Fatalf("above matra should not pre-reorder: %d,%d", out[0].cluster, out[1].cluster)
	}
}

func TestIndicFindBase_MultiConsonant(t *testing.T) {
	// Ra + Halant + Ka + Halant + Ta → base is Ta (last consonant)
	runes := []rune{devaRa, devaHalant, 0x0915, devaHalant, 0x0924} // र्क्त
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	base := findIndicBaseIndex(units)
	if base != 4 {
		t.Fatalf("base index=%d want 4 (Ta)", base)
	}
	// Without reph: Ka + Halant + Ta → base Ta
	units2 := []indicUnit{{r: 0x0915, orig: 0}, {r: devaHalant, orig: 1}, {r: 0x0924, orig: 2}}
	if findIndicBaseIndex(units2) != 2 {
		t.Fatalf("cluster base=%d want 2", findIndicBaseIndex(units2))
	}
}

func TestIndicFindBase_SkipBelowBaseRa(t *testing.T) {
	// Ka + Halant + Ra → base is Ka (Ra is below-base / rakar, not base)
	units := []indicUnit{
		{r: 0x0915, orig: 0}, // Ka
		{r: devaHalant, orig: 1},
		{r: devaRa, orig: 2},
	}
	if findIndicBaseIndex(units) != 0 {
		t.Fatalf("base=%d want 0 (Ka, skip below Ra)", findIndicBaseIndex(units))
	}
	if indicConsonantPos(devaRa) != consPosBelow {
		t.Fatal("Ra should be below-base class")
	}
	if indicConsonantPos(0x0915) != consPosBase {
		t.Fatal("Ka base class")
	}
}

func TestIndicFinal_BelowBaseConsonantBucket(t *testing.T) {
	// Logical: Ka + Halant + Ra (rakar). Final: Ka then Halant+Ra in below bucket.
	runes := []rune{0x0915, devaHalant, devaRa}
	gs := []shapingGlyph{
		{gid: 1, cluster: 0}, // Ka
		{gid: 2, cluster: 1}, // Halant
		{gid: 3, cluster: 2}, // Ra
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	// base mid first, then below (halant+ra)
	if out[0].cluster != 0 {
		t.Fatalf("base first: %v", clustersOf(out))
	}
	if out[1].cluster != 1 || out[2].cluster != 2 {
		t.Fatalf("below cluster order: %v", clustersOf(out))
	}
}

func TestIndicConsonantPos_PeerScripts(t *testing.T) {
	if indicConsonantPos(0x09B0) != consPosBelow { // Bengali Ra
		t.Fatal("bengali ra")
	}
	if indicConsonantPos(0x0C30) != consPosPost { // Telugu Ra
		t.Fatal("telugu ra post")
	}
	if indicConsonantPos(0x0915) != consPosBase {
		t.Fatal("ka base")
	}
}

func TestIndicInitial_RephAfterMultiConsonantBase(t *testing.T) {
	// र् + क + ् + त → क + ् + त + र + ्  (reph after base Ta)
	runes := []rune{devaRa, devaHalant, 0x0915, devaHalant, 0x0924}
	units := reorderIndicInitial(runes)
	got := indicUnitsToRunes(units)
	want := []rune{0x0915, devaHalant, 0x0924, devaRa, devaHalant}
	if string(got) != string(want) {
		t.Fatalf("multi reph initial: got %U want %U", got, want)
	}
	// orig: Ka=2, Halant=3, Ta=4, Ra=0, Halant=1
	if units[0].orig != 2 || units[2].orig != 4 || units[3].orig != 0 {
		t.Fatalf("orig=%v", []int{units[0].orig, units[1].orig, units[2].orig, units[3].orig, units[4].orig})
	}
}

func TestIndicFinal_MatraBuckets(t *testing.T) {
	// Ka + pre-i + below-u + above-e + post-aa
	// Logical Unicode often stores matras after base in various orders; we place by class.
	runes := []rune{0x0915, 0x093F, 0x0941, 0x0947, 0x093E} // क ि ु े ा
	gs := []shapingGlyph{
		{gid: 1, cluster: 0}, // Ka
		{gid: 2, cluster: 1}, // pre i
		{gid: 3, cluster: 2}, // below u
		{gid: 4, cluster: 3}, // above e
		{gid: 5, cluster: 4}, // post aa
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	// Expected clusters: pre(1) | base(0) | below(2) | above(3) | post(4)
	want := []int{1, 0, 2, 3, 4}
	for i, c := range want {
		if out[i].cluster != c {
			t.Fatalf("pos %d cluster=%d want %d (full=%v)", i, out[i].cluster, c, clustersOf(out))
		}
	}
}

func TestIndicFinal_RephAfterMatras(t *testing.T) {
	// Reph + Ka + pre-i → final: pre | base | reph
	runes := []rune{devaRa, devaHalant, 0x0915, devaIMatra}
	units := reorderIndicInitial(runes)
	gs := make([]shapingGlyph, len(units))
	for i, u := range units {
		gs[i] = shapingGlyph{gid: uint16(10 + i), cluster: u.orig}
	}
	// After initial units are Ka, Ra, Halant, i_matra with orig 2,0,1,3
	out := reorderIndicFinalGlyphs(gs, runes)
	// pre (3), base (2), reph (0,1)
	if len(out) != 4 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0].cluster != 3 || out[1].cluster != 2 {
		t.Fatalf("pre+base: %v", clustersOf(out))
	}
	if out[2].cluster != 0 || out[3].cluster != 1 {
		t.Fatalf("reph end: %v", clustersOf(out))
	}
}

func TestIndicSyllableSplit_ConsonantCluster(t *testing.T) {
	// क + ् + त + ि  one syllable
	runes := []rune{0x0915, devaHalant, 0x0924, devaIMatra}
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	sylls := splitIndicSyllables(units)
	if len(sylls) != 1 {
		t.Fatalf("syllables=%d want 1", len(sylls))
	}
	if len(sylls[0]) != 4 {
		t.Fatalf("syll len=%d", len(sylls[0]))
	}
}

func clustersOf(gs []shapingGlyph) []int {
	out := make([]int, len(gs))
	for i, g := range gs {
		out[i] = g.cluster
	}
	return out
}

func TestKhmerCategory_PreBaseAndCoeng(t *testing.T) {
	if indicCategory(0x1780) != icConsonant { // ក
		t.Fatal("khmer ka")
	}
	if indicCategory(khmerCoeng) != icVirama {
		t.Fatal("coeng")
	}
	if indicCategory(0x17C1) != icPreBaseMatra { // េ
		t.Fatal("khmer e pre")
	}
	if indicCategory(0x17B6) != icPostMatra { // ា
		t.Fatal("khmer aa post")
	}
}

func TestKhmerFinal_PreBaseBeforeBase(t *testing.T) {
	// Ka + pre-base e → visual e then Ka
	runes := []rune{0x1780, 0x17C1} // ក េ
	gs := []shapingGlyph{{gid: 1, cluster: 0}, {gid: 2, cluster: 1}}
	out := reorderIndicFinalGlyphs(gs, runes)
	if out[0].cluster != 1 || out[1].cluster != 0 {
		t.Fatalf("khmer pre-base: %v", clustersOf(out))
	}
}

func TestKhmerSyllable_CoengCluster(t *testing.T) {
	// Ka + Coeng + Kho → one syllable
	runes := []rune{0x1780, khmerCoeng, 0x1781}
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	sylls := splitIndicSyllables(units)
	if len(sylls) != 1 || len(sylls[0]) != 3 {
		t.Fatalf("sylls=%d len0=%v", len(sylls), len(sylls[0]))
	}
	if findIndicBaseIndex(units) != 2 {
		t.Fatalf("base=%d want last consonant (kho)", findIndicBaseIndex(units))
	}
}

func TestMyanmarCategory_PreBaseE(t *testing.T) {
	if indicCategory(0x1000) != icConsonant { // က
		t.Fatal("myan ka")
	}
	if indicCategory(myanE) != icPreBaseMatra {
		t.Fatal("myan e pre")
	}
	if indicCategory(myanVirama) != icVirama {
		t.Fatal("myan virama")
	}
}

func TestMyanmarFinal_PreBaseE(t *testing.T) {
	// Ka + e → e before Ka
	runes := []rune{0x1000, myanE}
	gs := []shapingGlyph{{gid: 1, cluster: 0}, {gid: 2, cluster: 1}}
	out := reorderIndicFinalGlyphs(gs, runes)
	if out[0].cluster != 1 || out[1].cluster != 0 {
		t.Fatalf("myanmar pre-base: %v", clustersOf(out))
	}
}

func TestMyanmarInitial_KinziAfterBase(t *testing.T) {
	// Kinzi + Ka → Ka + kinzi (Nga Asat Virama)
	// U+1004 U+103A U+1039 U+1000
	runes := []rune{myanNga, myanAsat, myanVirama, 0x1000}
	units := reorderIndicInitial(runes)
	got := indicUnitsToRunes(units)
	want := []rune{0x1000, myanNga, myanAsat, myanVirama}
	if string(got) != string(want) {
		t.Fatalf("kinzi initial: got %U want %U", got, want)
	}
}

func TestMyanmarFinal_KinziToEnd(t *testing.T) {
	runes := []rune{myanNga, myanAsat, myanVirama, 0x1000, myanE}
	units := reorderIndicInitial(runes)
	gs := make([]shapingGlyph, len(units))
	for i, u := range units {
		gs[i] = shapingGlyph{gid: uint16(10 + i), cluster: u.orig}
	}
	out := reorderIndicFinalGlyphs(gs, runes)
	// pre e (4), base (3), kinzi (0,1,2)
	if len(out) != 5 {
		t.Fatalf("len=%d %v", len(out), clustersOf(out))
	}
	if out[0].cluster != 4 {
		t.Fatalf("pre first: %v", clustersOf(out))
	}
	if out[1].cluster != 3 {
		t.Fatalf("base second: %v", clustersOf(out))
	}
	if out[2].cluster != 0 || out[3].cluster != 1 || out[4].cluster != 2 {
		t.Fatalf("kinzi end: %v", clustersOf(out))
	}
}

func TestNeedsIndic_KhmerMyanmar(t *testing.T) {
	if !needsIndicShaping([]rune{0x1780, 0x17C1}) {
		t.Fatal("khmer should need indic staging")
	}
	if !needsIndicShaping([]rune{0x1000, myanE}) {
		t.Fatal("myanmar should need indic staging")
	}
}
