// GSUB Lookup Type 5 (Contextual), Type 6 (Chaining Contextual), Type 8 (Reverse).
//
// ENGINE_GAPS G1.c — complex OT shaping. Nested substitutions resolve through
// the existing Type 1–4 (and Type 7 extension) implementations via the parent
// gsubTable lookup list.
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gsub
//
// Formats:
//   - Type 5 Format 1/2/3
//   - Type 6 Format 1/2/3
//   - Type 8 Format 1 (single reverse chaining)
//
// Lookup flags IgnoreMarks/Base/Ligature, MarkAttachmentType, and
// MarkFilteringSet are honored via GDEF (see gdef.go / otLookup.markFilterSet).
package text

import "encoding/binary"

// substLookupRecord maps an input-sequence index to a nested GSUB lookup.
type substLookupRecord struct {
	sequenceIndex   uint16
	lookupListIndex uint16
}

func parseSubstLookupRecords(data []byte, offset, count int) ([]substLookupRecord, bool) {
	need := offset + count*4
	if count < 0 || need > len(data) {
		return nil, false
	}
	out := make([]substLookupRecord, count)
	for i := range count {
		off := offset + i*4
		out[i] = substLookupRecord{
			sequenceIndex:   binary.BigEndian.Uint16(data[off : off+2]),
			lookupListIndex: binary.BigEndian.Uint16(data[off+2 : off+4]),
		}
	}
	return out, true
}

// applyNestedLookup applies lookups[li] only starting at glyph position pos.
// Nested contextual lookups must not re-scan earlier glyphs.
func (g *gsubTable) applyNestedLookup(li uint16, glyphs []shapingGlyph, pos int) []shapingGlyph {
	if g == nil || int(li) >= len(g.lookups) || pos < 0 || pos >= len(glyphs) {
		return glyphs
	}
	return g.applyLookupAt(glyphs, &g.lookups[li], pos)
}

// applyLookupAt applies a single lookup only for matches that begin at pos.
func (g *gsubTable) applyLookupAt(glyphs []shapingGlyph, lu *otLookup, pos int) []shapingGlyph {
	lookupType := lu.lookupType
	if lookupType == 7 {
		// Extension: unwrap and apply extension type at pos.
		for _, st := range lu.subtables {
			if len(st) < 8 {
				continue
			}
			if binary.BigEndian.Uint16(st[0:2]) != 1 {
				continue
			}
			extType := binary.BigEndian.Uint16(st[2:4])
			extOffset := binary.BigEndian.Uint32(st[4:8])
			if int(extOffset) >= len(st) {
				continue
			}
			glyphs = g.applySubtableAt(extType, st[extOffset:], glyphs, pos)
		}
		return glyphs
	}
	for _, st := range lu.subtables {
		glyphs = g.applySubtableAt(lookupType, st, glyphs, pos)
	}
	return glyphs
}

// applySubtableAt applies one subtable restricted to position pos.
func (g *gsubTable) applySubtableAt(lookupType uint16, data []byte, glyphs []shapingGlyph, pos int) []shapingGlyph {
	if pos < 0 || pos >= len(glyphs) {
		return glyphs
	}
	switch lookupType {
	case 1:
		// Single subst — only transform glyphs[pos].
		one := []shapingGlyph{glyphs[pos]}
		one = applySingleSubst(data, one)
		glyphs[pos] = one[0]
		return glyphs
	case 2:
		// Multiple subst — only expand glyphs[pos].
		one := []shapingGlyph{glyphs[pos]}
		one = applyMultipleSubst(data, one)
		return sliceReplace(glyphs, pos, 1, one)
	case 3:
		one := []shapingGlyph{glyphs[pos]}
		one = applyAlternateSubst(data, one)
		glyphs[pos] = one[0]
		return glyphs
	case 4:
		return applyLigatureSubstAt(data, glyphs, pos)
	case 5:
		// Nested contextual: apply only on suffix, then rejoin.
		suffix := append([]shapingGlyph(nil), glyphs[pos:]...)
		suffix = g.applyContextualSubst(data, suffix)
		return sliceReplace(glyphs, pos, len(glyphs)-pos, suffix)
	case 6:
		suffix := append([]shapingGlyph(nil), glyphs[pos:]...)
		suffix = g.applyChainedContextualSubst(data, suffix)
		return sliceReplace(glyphs, pos, len(glyphs)-pos, suffix)
	case 8:
		// Reverse chain needs neighbors — apply on a copy, write only pos.
		tmp := append([]shapingGlyph(nil), glyphs...)
		tmp = g.applyReverseChainingSubst(data, tmp)
		glyphs[pos] = tmp[pos]
		return glyphs
	default:
		return glyphs
	}
}

// applySubstRecords applies nested lookups and returns the new glyph slice.
// delta tracks length changes so subsequent sequenceIndex values stay correct.
func (g *gsubTable) applySubstRecords(glyphs []shapingGlyph, inputStart int, recs []substLookupRecord) []shapingGlyph {
	delta := 0
	for _, rec := range recs {
		pos := inputStart + int(rec.sequenceIndex) + delta
		if pos < 0 || pos >= len(glyphs) {
			continue
		}
		before := len(glyphs)
		glyphs = g.applyNestedLookup(rec.lookupListIndex, glyphs, pos)
		delta += len(glyphs) - before
	}
	return glyphs
}

// applyContextualSubst is GSUB Lookup Type 5.
func (g *gsubTable) applyContextualSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 2 {
		return glyphs
	}
	switch binary.BigEndian.Uint16(data[0:2]) {
	case 1:
		return g.applyContextFormat1(data, glyphs)
	case 2:
		return g.applyContextFormat2(data, glyphs)
	case 3:
		return g.applyContextFormat3(data, glyphs)
	default:
		return glyphs
	}
}

// applyChainedContextualSubst is GSUB Lookup Type 6.
func (g *gsubTable) applyChainedContextualSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 2 {
		return glyphs
	}
	switch binary.BigEndian.Uint16(data[0:2]) {
	case 1:
		return g.applyChainFormat1(data, glyphs)
	case 2:
		return g.applyChainFormat2(data, glyphs)
	case 3:
		return g.applyChainFormat3(data, glyphs)
	default:
		return glyphs
	}
}

// applyReverseChainingSubst is GSUB Lookup Type 8 Format 1.
func (g *gsubTable) applyReverseChainingSubst(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 10 {
		return glyphs
	}
	if binary.BigEndian.Uint16(data[0:2]) != 1 {
		return glyphs
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	if covOff >= len(data) {
		return glyphs
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return glyphs
	}
	backtrackCount := int(binary.BigEndian.Uint16(data[4:6]))
	pos := 6
	if pos+backtrackCount*2 > len(data) {
		return glyphs
	}
	backtrackCov := make([]otCoverage, backtrackCount)
	for i := range backtrackCount {
		off := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		if off >= len(data) {
			return glyphs
		}
		backtrackCov[i] = parseCoverage(data[off:])
		if backtrackCov[i] == nil {
			return glyphs
		}
	}
	if pos+2 > len(data) {
		return glyphs
	}
	lookaheadCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	if pos+lookaheadCount*2 > len(data) {
		return glyphs
	}
	lookaheadCov := make([]otCoverage, lookaheadCount)
	for i := range lookaheadCount {
		off := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		pos += 2
		if off >= len(data) {
			return glyphs
		}
		lookaheadCov[i] = parseCoverage(data[off:])
		if lookaheadCov[i] == nil {
			return glyphs
		}
	}
	if pos+2 > len(data) {
		return glyphs
	}
	glyphCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2
	if pos+glyphCount*2 > len(data) {
		return glyphs
	}
	substitutes := make([]uint16, glyphCount)
	for i := range glyphCount {
		substitutes[i] = binary.BigEndian.Uint16(data[pos : pos+2])
		pos += 2
	}

	for i := len(glyphs) - 1; i >= 0; i-- {
		covIdx, ok := cov.contains(glyphs[i].gid)
		if !ok || covIdx >= len(substitutes) {
			continue
		}
		if !matchCoverageBacktrack(glyphs, i, backtrackCov) {
			continue
		}
		if !matchCoverageLookahead(glyphs, i, 1, lookaheadCov) {
			continue
		}
		glyphs[i].gid = substitutes[covIdx]
	}
	return glyphs
}

// --- Type 5 Format 1 ---

func (g *gsubTable) applyContextFormat1(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	ruleSetCount := int(binary.BigEndian.Uint16(data[4:6]))
	if covOff >= len(data) || len(data) < 6+ruleSetCount*2 {
		return glyphs
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return glyphs
	}

	i := 0
	for i < len(glyphs) {
		setIdx, ok := cov.contains(glyphs[i].gid)
		if !ok || setIdx >= ruleSetCount {
			i++
			continue
		}
		setOff := int(binary.BigEndian.Uint16(data[6+setIdx*2 : 6+setIdx*2+2]))
		if setOff >= len(data) {
			i++
			continue
		}
		var matched bool
		var inputLen int
		glyphs, matched, inputLen = g.tryContextRuleSetGlyph(data[setOff:], glyphs, i)
		if matched {
			if inputLen <= 1 {
				i++
			}
			// else stay — buffer may have changed around i
			continue
		}
		i++
	}
	return glyphs
}

func (g *gsubTable) tryContextRuleSetGlyph(data []byte, glyphs []shapingGlyph, pos int) ([]shapingGlyph, bool, int) {
	if len(data) < 2 {
		return glyphs, false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return glyphs, false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		rule := data[ruleOff:]
		if len(rule) < 4 {
			continue
		}
		glyphCount := int(binary.BigEndian.Uint16(rule[0:2]))
		substCount := int(binary.BigEndian.Uint16(rule[2:4]))
		if glyphCount < 1 {
			continue
		}
		extra := glyphCount - 1
		if len(rule) < 4+extra*2+substCount*4 {
			continue
		}
		if pos+glyphCount > len(glyphs) {
			continue
		}
		match := true
		for j := range extra {
			want := binary.BigEndian.Uint16(rule[4+j*2 : 4+j*2+2])
			if glyphs[pos+1+j].gid != want {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		recs, ok := parseSubstLookupRecords(rule, 4+extra*2, substCount)
		if !ok {
			continue
		}
		glyphs = g.applySubstRecords(glyphs, pos, recs)
		return glyphs, true, glyphCount
	}
	return glyphs, false, 0
}

// --- Type 5 Format 2 ---

func (g *gsubTable) applyContextFormat2(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 8 {
		return glyphs
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	classDefOff := int(binary.BigEndian.Uint16(data[4:6]))
	classSetCount := int(binary.BigEndian.Uint16(data[6:8]))
	if covOff >= len(data) || classDefOff >= len(data) || len(data) < 8+classSetCount*2 {
		return glyphs
	}
	cov := parseCoverage(data[covOff:])
	cd := parseClassDef(data[classDefOff:])
	if cov == nil || cd == nil {
		return glyphs
	}

	i := 0
	for i < len(glyphs) {
		if _, ok := cov.contains(glyphs[i].gid); !ok {
			i++
			continue
		}
		class0 := int(cd.classOf(glyphs[i].gid))
		if class0 >= classSetCount {
			i++
			continue
		}
		setOff := int(binary.BigEndian.Uint16(data[8+class0*2 : 8+class0*2+2]))
		if setOff == 0 || setOff >= len(data) {
			i++
			continue
		}
		var matched bool
		var inputLen int
		glyphs, matched, inputLen = g.tryContextClassRuleSet(data[setOff:], glyphs, i, cd)
		if matched {
			if inputLen <= 1 {
				i++
			}
			continue
		}
		i++
	}
	return glyphs
}

func (g *gsubTable) tryContextClassRuleSet(data []byte, glyphs []shapingGlyph, pos int, cd otClassDef) ([]shapingGlyph, bool, int) {
	if len(data) < 2 {
		return glyphs, false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return glyphs, false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		rule := data[ruleOff:]
		if len(rule) < 4 {
			continue
		}
		glyphCount := int(binary.BigEndian.Uint16(rule[0:2]))
		substCount := int(binary.BigEndian.Uint16(rule[2:4]))
		if glyphCount < 1 {
			continue
		}
		extra := glyphCount - 1
		if len(rule) < 4+extra*2+substCount*4 {
			continue
		}
		if pos+glyphCount > len(glyphs) {
			continue
		}
		match := true
		for j := range extra {
			wantClass := binary.BigEndian.Uint16(rule[4+j*2 : 4+j*2+2])
			if cd.classOf(glyphs[pos+1+j].gid) != wantClass {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		recs, ok := parseSubstLookupRecords(rule, 4+extra*2, substCount)
		if !ok {
			continue
		}
		glyphs = g.applySubstRecords(glyphs, pos, recs)
		return glyphs, true, glyphCount
	}
	return glyphs, false, 0
}

// --- Type 5 Format 3 ---

func (g *gsubTable) applyContextFormat3(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	glyphCount := int(binary.BigEndian.Uint16(data[2:4]))
	substCount := int(binary.BigEndian.Uint16(data[4:6]))
	if glyphCount < 1 || len(data) < 6+glyphCount*2+substCount*4 {
		return glyphs
	}
	coverages := make([]otCoverage, glyphCount)
	for i := range glyphCount {
		off := int(binary.BigEndian.Uint16(data[6+i*2 : 6+i*2+2]))
		if off >= len(data) {
			return glyphs
		}
		coverages[i] = parseCoverage(data[off:])
		if coverages[i] == nil {
			return glyphs
		}
	}
	recs, ok := parseSubstLookupRecords(data, 6+glyphCount*2, substCount)
	if !ok {
		return glyphs
	}

	i := 0
	for i+glyphCount <= len(glyphs) {
		if matchCoverageSequence(glyphs, i, coverages) {
			before := len(glyphs)
			glyphs = g.applySubstRecords(glyphs, i, recs)
			if glyphCount <= 1 || len(glyphs) == before {
				i++
			}
			continue
		}
		i++
	}
	return glyphs
}

// --- Type 6 Format 1 ---

func (g *gsubTable) applyChainFormat1(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 6 {
		return glyphs
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	setCount := int(binary.BigEndian.Uint16(data[4:6]))
	if covOff >= len(data) || len(data) < 6+setCount*2 {
		return glyphs
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return glyphs
	}

	i := 0
	for i < len(glyphs) {
		setIdx, ok := cov.contains(glyphs[i].gid)
		if !ok || setIdx >= setCount {
			i++
			continue
		}
		setOff := int(binary.BigEndian.Uint16(data[6+setIdx*2 : 6+setIdx*2+2]))
		if setOff >= len(data) {
			i++
			continue
		}
		var matched bool
		var inputLen int
		glyphs, matched, inputLen = g.tryChainRuleSetGlyph(data[setOff:], glyphs, i)
		if matched {
			if inputLen <= 1 {
				i++
			}
			continue
		}
		i++
	}
	return glyphs
}

func (g *gsubTable) tryChainRuleSetGlyph(data []byte, glyphs []shapingGlyph, pos int) ([]shapingGlyph, bool, int) {
	if len(data) < 2 {
		return glyphs, false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return glyphs, false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		var ok bool
		var inputLen int
		glyphs, ok, inputLen = g.matchAndApplyChainRuleGlyph(data[ruleOff:], glyphs, pos)
		if ok {
			return glyphs, true, inputLen
		}
	}
	return glyphs, false, 0
}

func (g *gsubTable) matchAndApplyChainRuleGlyph(rule []byte, glyphs []shapingGlyph, pos int) ([]shapingGlyph, bool, int) {
	if len(rule) < 2 {
		return glyphs, false, 0
	}
	p := 0
	backtrackCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(rule) {
		return glyphs, false, 0
	}
	backtrack := make([]uint16, backtrackCount)
	for i := range backtrackCount {
		backtrack[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	inputCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if inputCount < 1 {
		return glyphs, false, 0
	}
	extra := inputCount - 1
	if p+extra*2 > len(rule) {
		return glyphs, false, 0
	}
	inputExtra := make([]uint16, extra)
	for i := range extra {
		inputExtra[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	lookaheadCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(rule) {
		return glyphs, false, 0
	}
	lookahead := make([]uint16, lookaheadCount)
	for i := range lookaheadCount {
		lookahead[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	substCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	recs, ok := parseSubstLookupRecords(rule, p, substCount)
	if !ok {
		return glyphs, false, 0
	}

	if pos < backtrackCount {
		return glyphs, false, 0
	}
	for i := range backtrackCount {
		if glyphs[pos-1-i].gid != backtrack[i] {
			return glyphs, false, 0
		}
	}
	if pos+inputCount > len(glyphs) {
		return glyphs, false, 0
	}
	for i := range extra {
		if glyphs[pos+1+i].gid != inputExtra[i] {
			return glyphs, false, 0
		}
	}
	lookStart := pos + inputCount
	if lookStart+lookaheadCount > len(glyphs) {
		return glyphs, false, 0
	}
	for i := range lookaheadCount {
		if glyphs[lookStart+i].gid != lookahead[i] {
			return glyphs, false, 0
		}
	}

	glyphs = g.applySubstRecords(glyphs, pos, recs)
	return glyphs, true, inputCount
}

// --- Type 6 Format 2 ---

func (g *gsubTable) applyChainFormat2(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 12 {
		return glyphs
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	backCDOff := int(binary.BigEndian.Uint16(data[4:6]))
	inputCDOff := int(binary.BigEndian.Uint16(data[6:8]))
	lookCDOff := int(binary.BigEndian.Uint16(data[8:10]))
	setCount := int(binary.BigEndian.Uint16(data[10:12]))
	if covOff >= len(data) || backCDOff >= len(data) || inputCDOff >= len(data) ||
		lookCDOff >= len(data) || len(data) < 12+setCount*2 {
		return glyphs
	}
	cov := parseCoverage(data[covOff:])
	backCD := parseClassDef(data[backCDOff:])
	inputCD := parseClassDef(data[inputCDOff:])
	lookCD := parseClassDef(data[lookCDOff:])
	if cov == nil || backCD == nil || inputCD == nil || lookCD == nil {
		return glyphs
	}

	i := 0
	for i < len(glyphs) {
		if _, ok := cov.contains(glyphs[i].gid); !ok {
			i++
			continue
		}
		class0 := int(inputCD.classOf(glyphs[i].gid))
		if class0 >= setCount {
			i++
			continue
		}
		setOff := int(binary.BigEndian.Uint16(data[12+class0*2 : 12+class0*2+2]))
		if setOff == 0 || setOff >= len(data) {
			i++
			continue
		}
		var matched bool
		var inputLen int
		glyphs, matched, inputLen = g.tryChainClassRuleSet(data[setOff:], glyphs, i, backCD, inputCD, lookCD)
		if matched {
			if inputLen <= 1 {
				i++
			}
			continue
		}
		i++
	}
	return glyphs
}

func (g *gsubTable) tryChainClassRuleSet(
	data []byte,
	glyphs []shapingGlyph,
	pos int,
	backCD, inputCD, lookCD otClassDef,
) ([]shapingGlyph, bool, int) {
	if len(data) < 2 {
		return glyphs, false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return glyphs, false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		var ok bool
		var inputLen int
		glyphs, ok, inputLen = g.matchAndApplyChainRuleClass(data[ruleOff:], glyphs, pos, backCD, inputCD, lookCD)
		if ok {
			return glyphs, true, inputLen
		}
	}
	return glyphs, false, 0
}

func (g *gsubTable) matchAndApplyChainRuleClass(
	rule []byte,
	glyphs []shapingGlyph,
	pos int,
	backCD, inputCD, lookCD otClassDef,
) ([]shapingGlyph, bool, int) {
	if len(rule) < 2 {
		return glyphs, false, 0
	}
	p := 0
	backtrackCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(rule) {
		return glyphs, false, 0
	}
	backClasses := make([]uint16, backtrackCount)
	for i := range backtrackCount {
		backClasses[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	inputCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if inputCount < 1 {
		return glyphs, false, 0
	}
	extra := inputCount - 1
	if p+extra*2 > len(rule) {
		return glyphs, false, 0
	}
	inputExtra := make([]uint16, extra)
	for i := range extra {
		inputExtra[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	lookaheadCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(rule) {
		return glyphs, false, 0
	}
	lookClasses := make([]uint16, lookaheadCount)
	for i := range lookaheadCount {
		lookClasses[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return glyphs, false, 0
	}
	substCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	recs, ok := parseSubstLookupRecords(rule, p, substCount)
	if !ok {
		return glyphs, false, 0
	}

	if pos < backtrackCount {
		return glyphs, false, 0
	}
	for i := range backtrackCount {
		if backCD.classOf(glyphs[pos-1-i].gid) != backClasses[i] {
			return glyphs, false, 0
		}
	}
	if pos+inputCount > len(glyphs) {
		return glyphs, false, 0
	}
	for i := range extra {
		if inputCD.classOf(glyphs[pos+1+i].gid) != inputExtra[i] {
			return glyphs, false, 0
		}
	}
	lookStart := pos + inputCount
	if lookStart+lookaheadCount > len(glyphs) {
		return glyphs, false, 0
	}
	for i := range lookaheadCount {
		if lookCD.classOf(glyphs[lookStart+i].gid) != lookClasses[i] {
			return glyphs, false, 0
		}
	}

	glyphs = g.applySubstRecords(glyphs, pos, recs)
	return glyphs, true, inputCount
}

// --- Type 6 Format 3 ---

func (g *gsubTable) applyChainFormat3(data []byte, glyphs []shapingGlyph) []shapingGlyph {
	if len(data) < 4 {
		return glyphs
	}
	p := 2
	backtrackCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(data) {
		return glyphs
	}
	backCov := make([]otCoverage, backtrackCount)
	for i := range backtrackCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return glyphs
		}
		backCov[i] = parseCoverage(data[off:])
		if backCov[i] == nil {
			return glyphs
		}
	}
	if p+2 > len(data) {
		return glyphs
	}
	inputCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if inputCount < 1 || p+inputCount*2 > len(data) {
		return glyphs
	}
	inputCov := make([]otCoverage, inputCount)
	for i := range inputCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return glyphs
		}
		inputCov[i] = parseCoverage(data[off:])
		if inputCov[i] == nil {
			return glyphs
		}
	}
	if p+2 > len(data) {
		return glyphs
	}
	lookaheadCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(data) {
		return glyphs
	}
	lookCov := make([]otCoverage, lookaheadCount)
	for i := range lookaheadCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return glyphs
		}
		lookCov[i] = parseCoverage(data[off:])
		if lookCov[i] == nil {
			return glyphs
		}
	}
	if p+2 > len(data) {
		return glyphs
	}
	substCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	recs, ok := parseSubstLookupRecords(data, p, substCount)
	if !ok {
		return glyphs
	}

	i := 0
	for i < len(glyphs) {
		if i+inputCount > len(glyphs) {
			break
		}
		if !matchCoverageSequence(glyphs, i, inputCov) ||
			!matchCoverageBacktrack(glyphs, i, backCov) ||
			!matchCoverageLookahead(glyphs, i, inputCount, lookCov) {
			i++
			continue
		}
		before := len(glyphs)
		glyphs = g.applySubstRecords(glyphs, i, recs)
		if inputCount <= 1 || len(glyphs) == before {
			i++
		}
	}
	return glyphs
}

func matchCoverageSequence(glyphs []shapingGlyph, pos int, coverages []otCoverage) bool {
	if pos+len(coverages) > len(glyphs) {
		return false
	}
	for i, cov := range coverages {
		if cov == nil {
			return false
		}
		if _, ok := cov.contains(glyphs[pos+i].gid); !ok {
			return false
		}
	}
	return true
}

func matchCoverageBacktrack(glyphs []shapingGlyph, pos int, coverages []otCoverage) bool {
	if pos < len(coverages) {
		return false
	}
	for i, cov := range coverages {
		if cov == nil {
			return false
		}
		if _, ok := cov.contains(glyphs[pos-1-i].gid); !ok {
			return false
		}
	}
	return true
}

func matchCoverageLookahead(glyphs []shapingGlyph, inputStart, inputLen int, coverages []otCoverage) bool {
	start := inputStart + inputLen
	if start+len(coverages) > len(glyphs) {
		return false
	}
	for i, cov := range coverages {
		if cov == nil {
			return false
		}
		if _, ok := cov.contains(glyphs[start+i].gid); !ok {
			return false
		}
	}
	return true
}
