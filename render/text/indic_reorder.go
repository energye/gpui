// Indic syllable reordering (ENGINE_GAPS G1.c lightweight subset).
//
// Implements a Devanagari-oriented initial + final reorder sufficient for
// common Ra+Halant (reph), multi-consonant base selection, and pre-base matras
// across major Indic scripts.
// Matra classification covers pre / above / below / post for Devanagari and
// common peer-script pre-base signs. Not a full Uniscribe/HarfBuzz Indic
// engine (no Khmer / Myanmar / font-driven below-base classification).
//
// Pipeline position (OwnShaper):
//  1. initial reorder on logical runes (reph after base for feature application)
//  2. cmap → glyphs (clusters = original source indices)
//  3. staged GSUB / GPOS
//  4. final reorder on glyphs by cluster (pre | base-group | below | above | post | reph)
//
// Reference: Microsoft OpenType Devanagari shaping / “dev2” model (simplified).
package text

// Devanagari code points used by the lightweight classifier.
const (
	devaRa     = 0x0930
	devaHalant = 0x094D
	devaNukta  = 0x093C
	devaIMatra = 0x093F // pre-base matra "ि"
)

type indicCat uint8

const (
	icOther indicCat = iota
	icConsonant
	icVirama
	icNukta
	icPreBaseMatra
	icAboveMatra
	icBelowMatra
	icPostMatra // remaining dependent vowels (post-base / default matra)
	icIndependentVowel
	icZWJ
	icZWNJ
)

// icMatra is true for any dependent vowel class (pre/above/below/post).
func (c indicCat) isMatra() bool {
	return c == icPreBaseMatra || c == icAboveMatra || c == icBelowMatra || c == icPostMatra
}

func indicCategory(r rune) indicCat {
	switch r {
	case 0x200D:
		return icZWJ
	case 0x200C:
		return icZWNJ
	case devaHalant:
		return icVirama
	case devaNukta:
		return icNukta
	}
	if isIndicPreBaseMatra(r) {
		return icPreBaseMatra
	}
	if isIndicAboveMatra(r) {
		return icAboveMatra
	}
	if isIndicBelowMatra(r) {
		return icBelowMatra
	}
	// Dependent vowels (post-base / default) Devanagari block remainder
	if r >= 0x093E && r <= 0x094C || r == 0x094F || r == 0x0955 || r == 0x0956 || r == 0x0957 {
		return icPostMatra
	}
	// Peer-script dependent vowels (rough range; pre/above/below already classified)
	if isIndicDependentVowelBlock(r) {
		return icPostMatra
	}
	// Independent vowels
	if r >= 0x0904 && r <= 0x0914 || r == 0x0960 || r == 0x0961 {
		return icIndependentVowel
	}
	// Consonants (incl. Ra)
	if r >= 0x0915 && r <= 0x0939 || r >= 0x0958 && r <= 0x095F || r == 0x0979 || r == 0x097A {
		return icConsonant
	}
	// Other Indic scripts: treat general letter-like as consonant-ish for syllable breaks
	if DetectScript(r) == ScriptDevanagari || (r >= 0x0900 && r <= 0x097F) {
		// Remaining marks → matra-like / other
		if r >= 0x0900 && r <= 0x0903 {
			return icPostMatra // candrabindu, anusvara, visarga — post
		}
		return icOther
	}
	// Peer Indic consonants (Bengali…Malayalam letter ranges, coarse)
	if isIndicPeerConsonant(r) {
		return icConsonant
	}
	if isIndicPeerVirama(r) {
		return icVirama
	}
	return icOther
}

// isIndicPreBaseMatra lists common left-side (pre-base) matras used by final reorder.
// Devanagari + major North/South Indic scripts that place I/E/AI before the base.
func isIndicPreBaseMatra(r rune) bool {
	switch r {
	// Devanagari
	case 0x093F, // ि
		0x094E: // ॎ prishthamatra e
		return true
	// Bengali
	case 0x09BF, // ি
		0x09C7, // ে
		0x09C8: // ৈ
		return true
	// Gurmukhi
	case 0x0A3F: // ਿ
		return true
	// Gujarati
	case 0x0ABF: // િ
		return true
	// Oriya (E/AI are pre-base in Oriya model)
	case 0x0B47, // େ
		0x0B48: // ୈ
		return true
	// Tamil
	case 0x0BC6, // ெ
		0x0BC7, // ே
		0x0BC8: // ை
		return true
	// Malayalam
	case 0x0D46, // െ
		0x0D47, // േ
		0x0D48: // ൈ
		return true
	// Kannada (E/AI pre-base component of two-part matras)
	case 0x0CC6, // ೆ
		0x0CC7, // ೇ
		0x0CC8: // ೈ
		return true
	// Telugu (E/AI)
	case 0x0C46, // ె
		0x0C47, // ే
		0x0C48: // ై
		return true
	}
	return false
}

// isIndicAboveMatra: top-placed dependent vowels (Devanagari-focused + common peers).
func isIndicAboveMatra(r rune) bool {
	switch r {
	// Devanagari e/ai/candra e/o
	case 0x0945, 0x0946, 0x0947, 0x0948, 0x0949:
		return true
	// Bengali candrabindu-style vowels already post; I is pre. U+09C0 is post.
	// Gujarati e/ai
	case 0x0AC5, 0x0AC7, 0x0AC8, 0x0AC9:
		return true
	// Gurmukhi e/ai
	case 0x0A47, 0x0A48:
		return true
	// Oriya i (above)
	case 0x0B3F:
		return true
		// Tamil / Malayalam above signs are limited; skip rare
	}
	return false
}

// isIndicBelowMatra: bottom-placed dependent vowels.
func isIndicBelowMatra(r rune) bool {
	switch r {
	// Devanagari u/uu/vocalic r/rr/l/ll
	case 0x0941, 0x0942, 0x0943, 0x0944, 0x0962, 0x0963:
		return true
	// Bengali u/uu/vocalic r
	case 0x09C1, 0x09C2, 0x09C3, 0x09C4:
		return true
	// Gurmukhi u/uu
	case 0x0A41, 0x0A42:
		return true
	// Gujarati u/uu/vocalic r
	case 0x0AC1, 0x0AC2, 0x0AC3, 0x0AC4:
		return true
	// Oriya u/uu
	case 0x0B41, 0x0B42, 0x0B43:
		return true
	}
	return false
}

func isIndicDependentVowelBlock(r rune) bool {
	// Coarse dependent-vowel ranges for peer scripts (when not pre/above/below).
	switch {
	case r >= 0x09BE && r <= 0x09CC:
		return true
	case r >= 0x0A3E && r <= 0x0A4C:
		return true
	case r >= 0x0ABE && r <= 0x0ACC:
		return true
	case r >= 0x0B3E && r <= 0x0B4C:
		return true
	case r >= 0x0BBE && r <= 0x0BCC:
		return true
	case r >= 0x0C3E && r <= 0x0C4C:
		return true
	case r >= 0x0CBE && r <= 0x0CCC:
		return true
	case r >= 0x0D3E && r <= 0x0D4C:
		return true
	}
	return false
}

func isIndicPeerConsonant(r rune) bool {
	switch {
	case r >= 0x0995 && r <= 0x09B9: // Bengali
		return true
	case r >= 0x0A15 && r <= 0x0A39: // Gurmukhi
		return true
	case r >= 0x0A95 && r <= 0x0AB9: // Gujarati
		return true
	case r >= 0x0B15 && r <= 0x0B39: // Oriya
		return true
	case r >= 0x0B95 && r <= 0x0BB9: // Tamil
		return true
	case r >= 0x0C15 && r <= 0x0C39: // Telugu
		return true
	case r >= 0x0C95 && r <= 0x0CB9: // Kannada
		return true
	case r >= 0x0D15 && r <= 0x0D39: // Malayalam
		return true
	}
	return false
}

func isIndicPeerVirama(r rune) bool {
	switch r {
	case 0x09CD, 0x0A4D, 0x0ACD, 0x0B4D, 0x0BCD, 0x0C4D, 0x0CCD, 0x0D4D:
		return true
	}
	return false
}

// indicUnit is one logical character with its original source index.
type indicUnit struct {
	r    rune
	orig int
}

// reorderIndicInitial moves Ra+Halant that form a reph to just after the base
// consonant within each syllable (classic initial reordering for reph).
// Returns reordered units (orig indices preserved for clustering).
func reorderIndicInitial(runes []rune) []indicUnit {
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	if len(units) == 0 {
		return units
	}
	sylls := splitIndicSyllables(units)
	out := make([]indicUnit, 0, len(units))
	for _, syl := range sylls {
		out = append(out, reorderSyllableInitial(syl)...)
	}
	return out
}

// reorderIndicFinalGlyphs applies final reordering using original rune categories
// via glyph.cluster. Pre-base matras move before base; reph (Ra+Halant clusters)
// moves to the end of the syllable.
func reorderIndicFinalGlyphs(glyphs []shapingGlyph, runes []rune) []shapingGlyph {
	if len(glyphs) == 0 || len(runes) == 0 {
		return glyphs
	}
	// Build units from runes, find syllables in original logical order.
	units := make([]indicUnit, len(runes))
	for i, r := range runes {
		units[i] = indicUnit{r: r, orig: i}
	}
	// Syllable boundaries on original indices
	sylls := splitIndicSyllables(units)

	// Map cluster → syllable index
	clusterSyl := make(map[int]int, len(runes))
	for si, syl := range sylls {
		for _, u := range syl {
			clusterSyl[u.orig] = si
		}
	}

	// Group glyphs by syllable (stable)
	type bucket struct {
		gs []shapingGlyph
	}
	buckets := make([]bucket, len(sylls))
	var other []shapingGlyph
	for _, g := range glyphs {
		si, ok := clusterSyl[g.cluster]
		if !ok {
			other = append(other, g)
			continue
		}
		buckets[si].gs = append(buckets[si].gs, g)
	}

	out := make([]shapingGlyph, 0, len(glyphs))
	for si, syl := range sylls {
		out = append(out, reorderSyllableFinalGlyphs(buckets[si].gs, syl)...)
	}
	out = append(out, other...)
	return out
}

func splitIndicSyllables(units []indicUnit) [][]indicUnit {
	var sylls [][]indicUnit
	i := 0
	for i < len(units) {
		start := i
		cat := indicCategory(units[i].r)
		// Standalone other (space, Latin) — single unit syllables
		if cat == icOther || cat == icIndependentVowel {
			// Independent vowel + following matras
			i++
			for i < len(units) {
				c := indicCategory(units[i].r)
				if c.isMatra() || c == icNukta {
					i++
					continue
				}
				break
			}
			sylls = append(sylls, units[start:i])
			continue
		}
		// Consonant-based syllable
		if cat == icConsonant || cat == icVirama {
			// Consume: (Ra Halant)? C (Nukta)? (Halant C (Nukta)?)* matras...
			i++
			for i < len(units) {
				c := indicCategory(units[i].r)
				switch c {
				case icNukta, icZWJ, icZWNJ:
					i++
				case icVirama:
					// Halant + optional ZWJ/ZWNJ + consonant continues syllable
					i++
					if i < len(units) && (indicCategory(units[i].r) == icZWJ || indicCategory(units[i].r) == icZWNJ) {
						i++
					}
					if i < len(units) && indicCategory(units[i].r) == icConsonant {
						i++
						continue
					}
					// dangling virama — stay in syllable
				case icConsonant:
					// only after virama handled above; bare C starts new syllable
					goto done
				case icPreBaseMatra, icAboveMatra, icBelowMatra, icPostMatra:
					i++
					for i < len(units) {
						c2 := indicCategory(units[i].r)
						if c2.isMatra() || c2 == icNukta {
							i++
							continue
						}
						break
					}
					goto done
				default:
					goto done
				}
			}
		done:
			sylls = append(sylls, units[start:i])
			continue
		}
		// Fallback single
		sylls = append(sylls, units[start:start+1])
		i++
	}
	return sylls
}

func reorderSyllableInitial(syl []indicUnit) []indicUnit {
	if len(syl) < 3 {
		return syl
	}
	// Detect leading Reph: Ra + Halant + Consonant...
	if syl[0].r != devaRa || indicCategory(syl[1].r) != icVirama {
		return syl
	}
	// Multi-consonant: base is the last consonant before matras (not the reph Ra).
	base := findIndicBaseIndex(syl)
	if base < 0 {
		return syl
	}
	// Reph is already after base if base is the only post-reph consonant at index 2
	// and Ra+Halant still lead — only reorder when reph is still leading.
	if base < 2 {
		return syl
	}
	// Move Ra+Halant to immediately after base (+ optional nukta after base)
	endBase := base + 1
	if endBase < len(syl) && indicCategory(syl[endBase].r) == icNukta {
		endBase++
	}
	// out: syl[2:endBase] + Ra + Halant + syl[endBase:]
	out := make([]indicUnit, 0, len(syl))
	out = append(out, syl[2:endBase]...)
	out = append(out, syl[0], syl[1])
	out = append(out, syl[endBase:]...)
	return out
}

// findIndicBaseIndex returns the index of the base consonant within a syllable.
//
// Simplified Uniscribe/dev2 rule without font OT classes: the base is the last
// consonant before any matra. Leading reph (Ra+Halant) is not a base candidate.
// Returns -1 if no consonant base exists.
func findIndicBaseIndex(syl []indicUnit) int {
	if len(syl) == 0 {
		return -1
	}
	// Limit search to the pre-matra portion of the syllable.
	end := len(syl)
	for i, u := range syl {
		if indicCategory(u.r).isMatra() {
			end = i
			break
		}
	}
	start := 0
	// Skip leading reph Ra + Halant so base is chosen among following consonants.
	if end >= 2 && syl[0].r == devaRa && indicCategory(syl[1].r) == icVirama {
		start = 2
	}
	base := -1
	for i := start; i < end; i++ {
		if indicCategory(syl[i].r) == icConsonant {
			base = i
		}
	}
	return base
}

func reorderSyllableFinalGlyphs(gs []shapingGlyph, syl []indicUnit) []shapingGlyph {
	if len(gs) <= 1 || len(syl) == 0 {
		return gs
	}
	rephClusters := map[int]bool{}
	preBaseClusters := map[int]bool{}
	belowClusters := map[int]bool{}
	aboveClusters := map[int]bool{}
	postClusters := map[int]bool{}

	// Leading reph only (original logical Ra+Halant at syllable start).
	if len(syl) >= 2 && syl[0].r == devaRa && indicCategory(syl[1].r) == icVirama {
		// Only treat as reph if a following base consonant exists in the syllable.
		if findIndicBaseIndex(syl) >= 0 {
			rephClusters[syl[0].orig] = true
			rephClusters[syl[1].orig] = true
		}
	}
	for i := 0; i < len(syl); i++ {
		switch indicCategory(syl[i].r) {
		case icPreBaseMatra:
			preBaseClusters[syl[i].orig] = true
		case icBelowMatra:
			belowClusters[syl[i].orig] = true
		case icAboveMatra:
			aboveClusters[syl[i].orig] = true
		case icPostMatra:
			postClusters[syl[i].orig] = true
		}
	}
	if len(preBaseClusters) == 0 && len(rephClusters) == 0 &&
		len(belowClusters) == 0 && len(aboveClusters) == 0 && len(postClusters) == 0 {
		return gs
	}
	// Final visual buckets (dev2-simplified):
	//   pre-base matras | base/half group | below | above | post | reph
	var pre, mid, below, above, post, reph []shapingGlyph
	for _, g := range gs {
		switch {
		case preBaseClusters[g.cluster]:
			pre = append(pre, g)
		case rephClusters[g.cluster]:
			reph = append(reph, g)
		case belowClusters[g.cluster]:
			below = append(below, g)
		case aboveClusters[g.cluster]:
			above = append(above, g)
		case postClusters[g.cluster]:
			post = append(post, g)
		default:
			mid = append(mid, g)
		}
	}
	out := make([]shapingGlyph, 0, len(gs))
	out = append(out, pre...)
	out = append(out, mid...)
	out = append(out, below...)
	out = append(out, above...)
	out = append(out, post...)
	out = append(out, reph...)
	return out
}

func indicUnitsToRunes(units []indicUnit) []rune {
	out := make([]rune, len(units))
	for i, u := range units {
		out[i] = u.r
	}
	return out
}
