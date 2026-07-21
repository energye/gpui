// GSUB table parser and glyph substitution engine.
//
// Implements OpenType GSUB (Glyph Substitution) table parsing and application.
// Supported lookup types:
//
//   - Type 1: Single substitution (one-to-one glyph replacement)
//   - Type 2: Multiple substitution (one-to-many expansion)
//   - Type 3: Alternate substitution (one-of-many selection)
//   - Type 4: Ligature substitution (many-to-one contraction, e.g. fi, ffi)
//   - Type 5: Contextual substitution (ENGINE_GAPS G1.c)
//   - Type 6: Chaining contextual substitution (ENGINE_GAPS G1.c)
//   - Type 7: Extension substitution (wrapper for above types in large fonts)
//   - Type 8: Reverse chaining single substitution (ENGINE_GAPS G1.c)
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gsub
//
// This file is part of Phase 5 (ADR-048: Pure Go Font Stack).
package text

import "encoding/binary"

// gsubTable holds parsed GSUB data ready for substitution.
type gsubTable struct {
	scripts  []otScript
	features []otFeature
	lookups  []otLookup
	gdef     *gdefTable // optional; enables IgnoreMarks/Base/Ligature flags
}

// parseGSUB parses the raw GSUB table data.
func parseGSUB(data []byte) *gsubTable {
	hdr, ok := parseOTLayoutHeader(data)
	if !ok {
		return nil
	}

	var g gsubTable
	if int(hdr.scriptListOffset) < len(data) {
		g.scripts = parseScriptList(data[hdr.scriptListOffset:])
	}
	if int(hdr.featureListOffset) < len(data) {
		g.features = parseFeatureList(data[hdr.featureListOffset:])
	}
	if int(hdr.lookupListOffset) < len(data) {
		g.lookups = parseLookupList(data[hdr.lookupListOffset:])
	}
	return &g
}

// applyGSUB applies GSUB substitutions to a glyph buffer.
// scriptTag and langTag select the language system. desiredTags selects
// which features to apply (e.g. "liga", "smcp").
//
// The glyph buffer is modified in-place (glyphs may be added or removed).
// Returns the resulting glyph buffer.
func (g *gsubTable) applyGSUB(
	glyphs []shapingGlyph,
	scriptTag, langTag [4]byte,
	desiredTags [][4]byte,
) []shapingGlyph {
	return g.applyGSUBFeatures(glyphs, scriptTag, langTag, desiredTags, nil)
}

// applyGSUBFeatures applies only the given feature tags. If mask is non-nil,
// Type 1 single substitutions only touch glyphs where mask[i] is true
// (used for Arabic isol/init/medi/fina positional forms).
func (g *gsubTable) applyGSUBFeatures(
	glyphs []shapingGlyph,
	scriptTag, langTag [4]byte,
	desiredTags [][4]byte,
	mask []bool,
) []shapingGlyph {
	if g == nil || len(desiredTags) == 0 {
		return glyphs
	}
	// Collect features in LangSys order (not sorted by lookup index) so that
	// required + listed features keep relative ordering for complex scripts.
	lookupIndices := collectLookupIndicesOrdered(g.scripts, g.features, scriptTag, langTag, desiredTags)
	for _, li := range lookupIndices {
		if int(li) >= len(g.lookups) {
			continue
		}
		glyphs = g.applyLookupMasked(&g.lookups[li], glyphs, mask)
	}
	return glyphs
}

// applyGSUBStagedArabic applies general features, then isol/fina/medi/init
// with per-glyph form masks derived from joining analysis (ENGINE_GAPS G1.c).
func (g *gsubTable) applyGSUBStagedArabic(
	glyphs []shapingGlyph,
	runes []rune,
	scriptTag, langTag [4]byte,
	desiredTags [][4]byte,
) []shapingGlyph {
	if g == nil {
		return glyphs
	}
	formTags := map[[4]byte]bool{
		{'i', 's', 'o', 'l'}: true,
		{'i', 'n', 'i', 't'}: true,
		{'m', 'e', 'd', 'i'}: true,
		{'f', 'i', 'n', 'a'}: true,
	}
	var general, forms [][4]byte
	for _, t := range desiredTags {
		if formTags[t] {
			forms = append(forms, t)
		} else {
			general = append(general, t)
		}
	}

	// 1) ccmp/locl/rlig/calt/liga… without form features
	glyphs = g.applyGSUBFeatures(glyphs, scriptTag, langTag, general, nil)

	if !needsArabicJoining(runes) || len(forms) == 0 {
		// Still apply form tags unmasked if caller only wanted them without arab text.
		if len(forms) > 0 && !needsArabicJoining(runes) {
			glyphs = g.applyGSUBFeatures(glyphs, scriptTag, langTag, forms, nil)
		}
		return glyphs
	}

	// Align forms to glyphs via cluster index → rune form.
	runeForms := computePresentationForms(runes)
	// OT Arabic feature order: isol, fina, medi, init (common practice).
	order := []presentationForm{formIsol, formFina, formMedi, formInit}
	for _, pf := range order {
		tag := formFeatureTag(pf)
		want := false
		for _, t := range forms {
			if t == tag {
				want = true
				break
			}
		}
		if !want {
			continue
		}
		mask := make([]bool, len(glyphs))
		for i, gl := range glyphs {
			if gl.cluster >= 0 && gl.cluster < len(runeForms) && runeForms[gl.cluster] == pf {
				mask[i] = true
			}
		}
		glyphs = g.applyGSUBFeatures(glyphs, scriptTag, langTag, [][4]byte{tag}, mask)
	}
	return glyphs
}

// shapingGlyph represents a glyph being shaped with its cluster index.
type shapingGlyph struct {
	gid     uint16
	cluster int // source character index
}

// applyLookup applies a single GSUB lookup to the glyph buffer.
func (g *gsubTable) applyLookup(lu *otLookup, glyphs []shapingGlyph) []shapingGlyph {
	return g.applyLookupMasked(lu, glyphs, nil)
}

func (g *gsubTable) applyLookupMasked(lu *otLookup, glyphs []shapingGlyph, mask []bool) []shapingGlyph {
	lookupType := lu.lookupType
	flag := lu.lookupFlag
	mfs := lu.markFilterSet

	// Extension substitution (Type 7) wraps another lookup type.
	if lookupType == 7 {
		return g.applyExtensionLookupMasked(lu, glyphs, mask)
	}

	for _, st := range lu.subtables {
		glyphs = g.applySubtableMasked(lookupType, st, glyphs, flag, mfs, mask)
	}
	return glyphs
}

// applyExtensionLookup handles GSUB Lookup Type 7 (Extension Substitution).
// Each subtable contains a pointer to the actual substitution subtable.
func (g *gsubTable) applyExtensionLookup(lu *otLookup, glyphs []shapingGlyph) []shapingGlyph {
	return g.applyExtensionLookupMasked(lu, glyphs, nil)
}

func (g *gsubTable) applyExtensionLookupMasked(lu *otLookup, glyphs []shapingGlyph, mask []bool) []shapingGlyph {
	for _, st := range lu.subtables {
		if len(st) < 8 {
			continue
		}
		format := binary.BigEndian.Uint16(st[0:2])
		if format != 1 {
			continue
		}
		extType := binary.BigEndian.Uint16(st[2:4])
		extOffset := binary.BigEndian.Uint32(st[4:8])
		if int(extOffset) >= len(st) {
			continue
		}
		// Extension subtables inherit the outer lookup flag.
		glyphs = g.applySubtableMasked(extType, st[extOffset:], glyphs, lu.lookupFlag, lu.markFilterSet, mask)
	}
	return glyphs
}

// applySubtable dispatches to the correct substitution function.
func (g *gsubTable) applySubtable(lookupType uint16, data []byte, glyphs []shapingGlyph, flag uint16) []shapingGlyph {
	return g.applySubtableMasked(lookupType, data, glyphs, flag, -1, nil)
}

func (g *gsubTable) applySubtableMasked(lookupType uint16, data []byte, glyphs []shapingGlyph, flag uint16, markFilterSet int, mask []bool) []shapingGlyph {
	switch lookupType {
	case 1:
		return applySingleSubstMasked(data, glyphs, mask)
	case 2:
		return applyMultipleSubst(data, glyphs)
	case 3:
		return applyAlternateSubst(data, glyphs)
	case 4:
		return applyLigatureSubstFlag(data, glyphs, flag, g.gdef, markFilterSet)
	case 5:
		return g.applyContextualSubst(data, glyphs)
	case 6:
		return g.applyChainedContextualSubst(data, glyphs)
	case 8:
		return g.applyReverseChainingSubst(data, glyphs)
	default:
		return glyphs
	}
}

// --- Lookup Type 1: Single Substitution ---

// applySingleSubst applies single substitution (one glyph → one glyph).
func applySingleSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	return applySingleSubstMasked(data, glyphs, nil)
}

// applySingleSubstMasked is Type 1 single subst; when mask != nil only mask[i]
// positions are eligible (Arabic presentation-form features).
func applySingleSubstMasked(data []byte, glyphs []shapingGlyph, mask []bool) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	format := binary.BigEndian.Uint16(data[0:2])
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return glyphs
	}

	allow := func(i int) bool {
		if mask == nil {
			return true
		}
		return i < len(mask) && mask[i]
	}

	switch format {
	case 1:
		// Format 1: add delta to covered glyphs.
		delta := int16(binary.BigEndian.Uint16(data[4:6]))
		for i := range glyphs {
			if !allow(i) {
				continue
			}
			if _, ok := cov.contains(glyphs[i].gid); ok {
				glyphs[i].gid = uint16(int32(glyphs[i].gid) + int32(delta))
			}
		}
	case 2:
		// Format 2: substitute from array.
		glyphCount := int(binary.BigEndian.Uint16(data[4:6]))
		if len(data) < 6+glyphCount*2 {
			return glyphs
		}
		for i := range glyphs {
			if !allow(i) {
				continue
			}
			idx, ok := cov.contains(glyphs[i].gid)
			if ok && idx < glyphCount {
				glyphs[i].gid = binary.BigEndian.Uint16(data[6+idx*2 : 6+idx*2+2])
			}
		}
	}
	return glyphs
}

// --- Lookup Type 2: Multiple Substitution ---

// applyMultipleSubst replaces one glyph with a sequence of glyphs.
func applyMultipleSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	format := binary.BigEndian.Uint16(data[0:2])
	if format != 1 {
		return glyphs
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return glyphs
	}
	seqCount := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 6+seqCount*2 {
		return glyphs
	}

	// Process from right to left so index shifts do not affect earlier glyphs.
	for i := len(glyphs) - 1; i >= 0; i-- {
		idx, ok := cov.contains(glyphs[i].gid)
		if !ok || idx >= seqCount {
			continue
		}
		seqOffset := int(binary.BigEndian.Uint16(data[6+idx*2 : 6+idx*2+2]))
		if seqOffset >= len(data) {
			continue
		}
		seq := data[seqOffset:]
		if len(seq) < 2 {
			continue
		}
		subCount := int(binary.BigEndian.Uint16(seq[0:2]))
		if len(seq) < 2+subCount*2 || subCount == 0 {
			continue
		}

		cluster := glyphs[i].cluster
		replacement := make([]shapingGlyph, subCount)
		for j := range subCount {
			replacement[j] = shapingGlyph{
				gid:     binary.BigEndian.Uint16(seq[2+j*2 : 2+j*2+2]),
				cluster: cluster,
			}
		}

		// Replace glyphs[i] with replacement.
		glyphs = sliceReplace(glyphs, i, 1, replacement)
	}
	return glyphs
}

// --- Lookup Type 3: Alternate Substitution ---

// applyAlternateSubst replaces a glyph with an alternate form.
// Always selects the first alternate (index 0). Full alternate selection
// would require user-facing API to choose which alternate.
func applyAlternateSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	format := binary.BigEndian.Uint16(data[0:2])
	if format != 1 {
		return glyphs
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return glyphs
	}
	altSetCount := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 6+altSetCount*2 {
		return glyphs
	}

	for i := range glyphs {
		idx, ok := cov.contains(glyphs[i].gid)
		if !ok || idx >= altSetCount {
			continue
		}
		altSetOffset := int(binary.BigEndian.Uint16(data[6+idx*2 : 6+idx*2+2]))
		if altSetOffset >= len(data) {
			continue
		}
		altSet := data[altSetOffset:]
		if len(altSet) < 4 {
			continue
		}
		altCount := int(binary.BigEndian.Uint16(altSet[0:2]))
		if altCount > 0 && len(altSet) >= 2+altCount*2 {
			// Select first alternate.
			glyphs[i].gid = binary.BigEndian.Uint16(altSet[2:4])
		}
	}
	return glyphs
}

// --- Lookup Type 4: Ligature Substitution ---

// applyLigatureSubst applies ligature substitution (many glyphs → one glyph).
// For example, 'f' + 'i' → 'fi' ligature glyph.
func applyLigatureSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	return applyLigatureSubstFlag(data, glyphs, 0, nil, -1)
}

// applyLigatureSubstFlag is applyLigatureSubst with GSUB lookup flags and GDEF.
// When IgnoreMarks / MarkFilteringSet is set, marks between components are
// skipped for matching and retained in the buffer after the ligature.
func applyLigatureSubstFlag(data []byte, glyphs []shapingGlyph, flag uint16, gdef *gdefTable, markFilterSet int) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	format := binary.BigEndian.Uint16(data[0:2])
	if format != 1 {
		return glyphs
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return glyphs
	}
	ligSetCount := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 6+ligSetCount*2 {
		return glyphs
	}

	// Process left-to-right; on successful match remove consumed glyphs
	// and advance past the ligature.
	i := 0
	for i < len(glyphs) {
		idx, ok := cov.contains(glyphs[i].gid)
		if !ok || idx >= ligSetCount {
			i++
			continue
		}
		ligSetOffset := int(binary.BigEndian.Uint16(data[6+idx*2 : 6+idx*2+2]))
		if ligSetOffset >= len(data) {
			i++
			continue
		}

		// Skip starting glyph if ignored by flag (rare for lig first component).
		if gdef != nil && gdef.ignoredByLookupFlag(glyphs[i].gid, flag, markFilterSet) {
			i++
			continue
		}
		matchedComps, consumed := tryLigatureSetFlag(data[ligSetOffset:], glyphs, i, flag, gdef, markFilterSet)
		if matchedComps > 0 {
			// Replace first component with ligature; remove only matched
			// non-ignored components after it. Ignored marks between
			// components stay in the buffer (after the ligature glyph).
			// Simple path when no skips: contiguous removal.
			if consumed == matchedComps {
				glyphs = sliceReplace(glyphs, i+1, matchedComps-1, nil)
			} else {
				// Compact: keep glyphs[i] as lig, drop matched component
				// positions after i, keep ignored.
				glyphs = compactLigatureMatch(glyphs, i, matchedComps, flag, gdef, markFilterSet)
			}
			i++
		} else {
			i++
		}
	}
	return glyphs
}

// tryLigatureSet is the no-flag wrapper used by tests and applyLigatureSubstAt.
func tryLigatureSet(data []byte, glyphs []shapingGlyph, pos int) int {
	matched, consumed := tryLigatureSetFlag(data, glyphs, pos, 0, nil, -1)
	if matched == 0 {
		return 0
	}
	return consumed
}

// tryLigatureSetFlag tries all ligatures in a LigatureSet starting at pos.
// On match, glyphs[pos].gid becomes the ligature. Returns (componentCount, bufferConsumed).
// componentCount is OpenType component count; bufferConsumed is how many
// buffer slots from pos were walked (includes ignored marks between components).
func tryLigatureSetFlag(data []byte, glyphs []shapingGlyph, pos int, flag uint16, gdef *gdefTable, markFilterSet int) (matchedComps int, consumed int) {
	if len(data) < 2 {
		return 0, 0
	}
	ligCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ligCount*2 {
		return 0, 0
	}

	for li := range ligCount {
		ligOffset := int(binary.BigEndian.Uint16(data[2+li*2 : 2+li*2+2]))
		if ligOffset >= len(data) {
			continue
		}
		ligData := data[ligOffset:]
		if len(ligData) < 4 {
			continue
		}
		ligGlyph := binary.BigEndian.Uint16(ligData[0:2])
		compCount := int(binary.BigEndian.Uint16(ligData[2:4]))
		if compCount < 2 {
			continue
		}
		numExtra := compCount - 1
		if len(ligData) < 4+numExtra*2 {
			continue
		}
		want := make([]uint16, numExtra)
		for j := range numExtra {
			want[j] = binary.BigEndian.Uint16(ligData[4+j*2 : 4+j*2+2])
		}
		// With no ignore flags, require contiguous match (fast path).
		useSkip := gdef != nil && (flag&(lookupFlagIgnoreBaseGlyphs|lookupFlagIgnoreLigatures|lookupFlagIgnoreMarks|lookupFlagUseMarkFilteringSet) != 0 ||
			(flag>>8)&0xFF != 0)
		if !useSkip {
			if pos+compCount > len(glyphs) {
				continue
			}
			ok := true
			for j := range numExtra {
				if glyphs[pos+1+j].gid != want[j] {
					ok = false
					break
				}
			}
			if ok {
				glyphs[pos].gid = ligGlyph
				return compCount, compCount
			}
			continue
		}
		// Skip-ignored matching for remaining components.
		walked := matchSequenceSkipIgnored(glyphs, pos, want, flag, gdef, markFilterSet)
		if walked > 0 {
			glyphs[pos].gid = ligGlyph
			return compCount, walked
		}
	}
	return 0, 0
}

// compactLigatureMatch rebuilds the glyph slice after a ligature match that
// skipped ignored glyphs: keep lig at pos, drop other matched components,
// retain ignored glyphs that were between components (place them after lig).
func compactLigatureMatch(glyphs []shapingGlyph, pos, matchedComps int, flag uint16, gdef *gdefTable, markFilterSet int) []shapingGlyph {
	// Collect indices of matched components (non-ignored), starting at pos.
	compIdx := make([]int, 0, matchedComps)
	compIdx = append(compIdx, pos)
	j := pos + 1
	for len(compIdx) < matchedComps {
		j = nextMatchIndex(glyphs, j, flag, gdef, markFilterSet)
		if j < 0 {
			break
		}
		compIdx = append(compIdx, j)
		j++
	}
	if len(compIdx) < matchedComps {
		// Fallback: contiguous remove
		return sliceReplace(glyphs, pos+1, matchedComps-1, nil)
	}
	end := compIdx[len(compIdx)-1] + 1
	// Marks between pos and end that are not in compIdx stay.
	drop := make(map[int]bool, len(compIdx)-1)
	for _, ix := range compIdx[1:] {
		drop[ix] = true
	}
	out := make([]shapingGlyph, 0, len(glyphs)-(matchedComps-1))
	out = append(out, glyphs[:pos+1]...) // includes lig at pos
	for k := pos + 1; k < end; k++ {
		if drop[k] {
			continue
		}
		out = append(out, glyphs[k])
	}
	out = append(out, glyphs[end:]...)
	return out
}

// applyLigatureSubstAt tries ligature substitution only starting at pos.
// Used by contextual nested lookups so later glyphs are not rescanned.
func applyLigatureSubstAt(data []byte, glyphs []shapingGlyph, pos int) []shapingGlyph {
	if pos < 0 || pos >= len(glyphs) || len(data) < 6 {
		return glyphs
	}
	format := binary.BigEndian.Uint16(data[0:2])
	if format != 1 {
		return glyphs
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return glyphs
	}
	ligSetCount := int(binary.BigEndian.Uint16(data[4:6]))
	if len(data) < 6+ligSetCount*2 {
		return glyphs
	}
	idx, ok := cov.contains(glyphs[pos].gid)
	if !ok || idx >= ligSetCount {
		return glyphs
	}
	ligSetOffset := int(binary.BigEndian.Uint16(data[6+idx*2 : 6+idx*2+2]))
	if ligSetOffset >= len(data) {
		return glyphs
	}
	matched := tryLigatureSet(data[ligSetOffset:], glyphs, pos)
	if matched > 0 {
		return sliceReplace(glyphs, pos+1, matched-1, nil)
	}
	return glyphs
}

// --- Slice helpers ---

// sliceReplace replaces glyphs[pos:pos+removeCount] with replacement.
// If replacement is nil/empty, it is a pure deletion.
func sliceReplace(glyphs []shapingGlyph, pos, removeCount int, replacement []shapingGlyph) []shapingGlyph {
	end := pos + removeCount
	if end > len(glyphs) {
		end = len(glyphs)
	}

	// Calculate new length.
	newLen := len(glyphs) - (end - pos) + len(replacement)
	if newLen <= 0 {
		return glyphs[:0]
	}

	// If we are shrinking, shift left and truncate.
	if len(replacement) <= end-pos {
		copy(glyphs[pos:], replacement)
		copy(glyphs[pos+len(replacement):], glyphs[end:])
		return glyphs[:newLen]
	}

	// Expanding: need to grow.
	result := make([]shapingGlyph, newLen)
	copy(result, glyphs[:pos])
	copy(result[pos:], replacement)
	copy(result[pos+len(replacement):], glyphs[end:])
	return result
}
