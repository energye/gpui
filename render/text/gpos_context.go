// GPOS Lookup Type 7 (Contextual) and Type 8 (Chained Contextual).
//
// ENGINE_GAPS G1.c — remaining GPOS context/chaining positioning.
// Nested lookups resolve through Type 1–6 (and Type 9 extension) via the
// parent gposTable lookup list. Glyph length is not changed by positioning.
//
// Formats: Type 7/8 Format 1 (glyph rules), 2 (class), 3 (coverage).
// PosLookupRecord layout matches SubstLookupRecord (sequenceIndex + lookupListIndex).
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gpos
package text

import "encoding/binary"

// posLookupRecord maps an input-sequence index to a nested GPOS lookup.
// Wire format is identical to GSUB SubstLookupRecord.
type posLookupRecord = substLookupRecord

func parsePosLookupRecords(data []byte, offset, count int) ([]posLookupRecord, bool) {
	return parseSubstLookupRecords(data, offset, count)
}

// applyNestedPos applies lookups[li] only at glyph position pos.
func (g *gposTable) applyNestedPos(li uint16, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int) {
	if g == nil || int(li) >= len(g.lookups) || pos < 0 || pos >= len(glyphs) {
		return
	}
	g.applyLookupAt(glyphs, adj, metrics, &g.lookups[li], pos)
}

// applyLookupAt applies a GPOS lookup only for matches that begin at pos.
func (g *gposTable) applyLookupAt(glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, lu *otLookup, pos int) {
	lookupType := lu.lookupType
	if lookupType == 9 {
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
			g.applySubtableAt(extType, st[extOffset:], glyphs, adj, metrics, pos)
		}
		return
	}
	for _, st := range lu.subtables {
		g.applySubtableAt(lookupType, st, glyphs, adj, metrics, pos)
	}
}

// applySubtableAt applies one GPOS subtable restricted to position pos.
// For multi-glyph lookups (pair/cursive), only pairs/links starting at pos fire.
func (g *gposTable) applySubtableAt(lookupType uint16, data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int) {
	if pos < 0 || pos >= len(glyphs) {
		return
	}
	switch lookupType {
	case 1:
		// Single pos — only glyphs[pos].
		one := []shapingGlyph{glyphs[pos]}
		oneAdj := []gposAdjustment{{}}
		applySinglePos(data, one, oneAdj)
		adj[pos].xPlacement += oneAdj[0].xPlacement
		adj[pos].yPlacement += oneAdj[0].yPlacement
		adj[pos].xAdvance += oneAdj[0].xAdvance
		adj[pos].yAdvance += oneAdj[0].yAdvance
	case 2:
		// Pair pos — only the pair (pos, pos+1).
		if pos+1 >= len(glyphs) {
			return
		}
		pair := []shapingGlyph{glyphs[pos], glyphs[pos+1]}
		pairAdj := []gposAdjustment{{}, {}}
		applyPairPos(data, pair, pairAdj)
		adj[pos].xPlacement += pairAdj[0].xPlacement
		adj[pos].yPlacement += pairAdj[0].yPlacement
		adj[pos].xAdvance += pairAdj[0].xAdvance
		adj[pos].yAdvance += pairAdj[0].yAdvance
		adj[pos+1].xPlacement += pairAdj[1].xPlacement
		adj[pos+1].yPlacement += pairAdj[1].yPlacement
		adj[pos+1].xAdvance += pairAdj[1].xAdvance
		adj[pos+1].yAdvance += pairAdj[1].yAdvance
	case 3:
		// Cursive — only link pos → pos+1.
		if pos+1 >= len(glyphs) {
			return
		}
		pair := []shapingGlyph{glyphs[pos], glyphs[pos+1]}
		pairAdj := []gposAdjustment{adj[pos], adj[pos+1]} // seed prior placement
		// Work on a copy of current adj for the two glyphs.
		tmp := []gposAdjustment{adj[pos], adj[pos+1]}
		applyCursivePos(data, pair, tmp)
		// applyCursivePos adds to tmp[1] relative to tmp[0]; merge deltas.
		adj[pos] = tmp[0]
		adj[pos+1] = tmp[1]
		_ = pairAdj
	case 4:
		// Mark-to-base: run full table but only keep adj[pos] delta.
		before := adj[pos]
		tmp := make([]gposAdjustment, len(adj))
		copy(tmp, adj)
		applyMarkToBasePos(data, glyphs, tmp, metrics)
		// Only apply change at pos (mark).
		adj[pos].xPlacement += tmp[pos].xPlacement - before.xPlacement
		adj[pos].yPlacement += tmp[pos].yPlacement - before.yPlacement
		adj[pos].xAdvance += tmp[pos].xAdvance - before.xAdvance
		adj[pos].yAdvance += tmp[pos].yAdvance - before.yAdvance
	case 5:
		before := adj[pos]
		tmp := make([]gposAdjustment, len(adj))
		copy(tmp, adj)
		applyMarkToLigPos(data, glyphs, tmp, metrics)
		adj[pos].xPlacement += tmp[pos].xPlacement - before.xPlacement
		adj[pos].yPlacement += tmp[pos].yPlacement - before.yPlacement
		adj[pos].xAdvance += tmp[pos].xAdvance - before.xAdvance
		adj[pos].yAdvance += tmp[pos].yAdvance - before.yAdvance
	case 6:
		before := adj[pos]
		tmp := make([]gposAdjustment, len(adj))
		copy(tmp, adj)
		applyMarkToMarkPos(data, glyphs, tmp, metrics)
		adj[pos].xPlacement += tmp[pos].xPlacement - before.xPlacement
		adj[pos].yPlacement += tmp[pos].yPlacement - before.yPlacement
		adj[pos].xAdvance += tmp[pos].xAdvance - before.xAdvance
		adj[pos].yAdvance += tmp[pos].yAdvance - before.yAdvance
	case 7:
		// Nested contextual: apply on suffix only for match starts.
		g.applyContextualPosOnRange(data, glyphs, adj, metrics, pos, len(glyphs))
	case 8:
		g.applyChainedPosOnRange(data, glyphs, adj, metrics, pos, len(glyphs))
	default:
		// ignore
	}
}

func (g *gposTable) applyPosRecords(glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, inputStart int, recs []posLookupRecord) {
	for _, rec := range recs {
		pos := inputStart + int(rec.sequenceIndex)
		if pos < 0 || pos >= len(glyphs) {
			continue
		}
		g.applyNestedPos(rec.lookupListIndex, glyphs, adj, metrics, pos)
	}
}

// --- Type 7 Contextual Positioning ---

func (g *gposTable) applyContextualPos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics) {
	g.applyContextualPosOnRange(data, glyphs, adj, metrics, 0, len(glyphs))
}

func (g *gposTable) applyContextualPosOnRange(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 2 || start < 0 || start >= end {
		return
	}
	switch binary.BigEndian.Uint16(data[0:2]) {
	case 1:
		g.applyPosContextFormat1(data, glyphs, adj, metrics, start, end)
	case 2:
		g.applyPosContextFormat2(data, glyphs, adj, metrics, start, end)
	case 3:
		g.applyPosContextFormat3(data, glyphs, adj, metrics, start, end)
	}
}

func (g *gposTable) applyPosContextFormat1(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 6 {
		return
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	ruleSetCount := int(binary.BigEndian.Uint16(data[4:6]))
	if covOff >= len(data) || len(data) < 6+ruleSetCount*2 {
		return
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return
	}
	i := start
	for i < end && i < len(glyphs) {
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
		if matched, inputLen := g.tryPosRuleSetGlyph(data[setOff:], glyphs, adj, metrics, i); matched {
			if inputLen <= 1 {
				i++
			} else {
				i += inputLen
			}
			continue
		}
		i++
	}
}

func (g *gposTable) tryPosRuleSetGlyph(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int) (bool, int) {
	if len(data) < 2 {
		return false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return false, 0
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
		posCount := int(binary.BigEndian.Uint16(rule[2:4]))
		if glyphCount < 1 {
			continue
		}
		extra := glyphCount - 1
		if len(rule) < 4+extra*2+posCount*4 {
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
		recs, ok := parsePosLookupRecords(rule, 4+extra*2, posCount)
		if !ok {
			continue
		}
		g.applyPosRecords(glyphs, adj, metrics, pos, recs)
		return true, glyphCount
	}
	return false, 0
}

func (g *gposTable) applyPosContextFormat2(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 8 {
		return
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	classDefOff := int(binary.BigEndian.Uint16(data[4:6]))
	classSetCount := int(binary.BigEndian.Uint16(data[6:8]))
	if covOff >= len(data) || classDefOff >= len(data) || len(data) < 8+classSetCount*2 {
		return
	}
	cov := parseCoverage(data[covOff:])
	cd := parseClassDef(data[classDefOff:])
	if cov == nil || cd == nil {
		return
	}
	i := start
	for i < end && i < len(glyphs) {
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
		if matched, inputLen := g.tryPosClassRuleSet(data[setOff:], glyphs, adj, metrics, i, cd); matched {
			if inputLen <= 1 {
				i++
			} else {
				i += inputLen
			}
			continue
		}
		i++
	}
}

func (g *gposTable) tryPosClassRuleSet(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int, cd otClassDef) (bool, int) {
	if len(data) < 2 {
		return false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return false, 0
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
		posCount := int(binary.BigEndian.Uint16(rule[2:4]))
		if glyphCount < 1 {
			continue
		}
		extra := glyphCount - 1
		if len(rule) < 4+extra*2+posCount*4 {
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
		recs, ok := parsePosLookupRecords(rule, 4+extra*2, posCount)
		if !ok {
			continue
		}
		g.applyPosRecords(glyphs, adj, metrics, pos, recs)
		return true, glyphCount
	}
	return false, 0
}

func (g *gposTable) applyPosContextFormat3(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 6 {
		return
	}
	glyphCount := int(binary.BigEndian.Uint16(data[2:4]))
	posCount := int(binary.BigEndian.Uint16(data[4:6]))
	if glyphCount < 1 || len(data) < 6+glyphCount*2+posCount*4 {
		return
	}
	coverages := make([]otCoverage, glyphCount)
	for i := range glyphCount {
		off := int(binary.BigEndian.Uint16(data[6+i*2 : 6+i*2+2]))
		if off >= len(data) {
			return
		}
		coverages[i] = parseCoverage(data[off:])
		if coverages[i] == nil {
			return
		}
	}
	recs, ok := parsePosLookupRecords(data, 6+glyphCount*2, posCount)
	if !ok {
		return
	}
	i := start
	for i+glyphCount <= end && i+glyphCount <= len(glyphs) {
		if matchCoverageSequence(glyphs, i, coverages) {
			g.applyPosRecords(glyphs, adj, metrics, i, recs)
			i += glyphCount
			continue
		}
		i++
	}
}

// --- Type 8 Chained Contextual Positioning ---

func (g *gposTable) applyChainedPos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics) {
	g.applyChainedPosOnRange(data, glyphs, adj, metrics, 0, len(glyphs))
}

func (g *gposTable) applyChainedPosOnRange(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 2 || start < 0 || start >= end {
		return
	}
	switch binary.BigEndian.Uint16(data[0:2]) {
	case 1:
		g.applyPosChainFormat1(data, glyphs, adj, metrics, start, end)
	case 2:
		g.applyPosChainFormat2(data, glyphs, adj, metrics, start, end)
	case 3:
		g.applyPosChainFormat3(data, glyphs, adj, metrics, start, end)
	}
}

func (g *gposTable) applyPosChainFormat1(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 6 {
		return
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	setCount := int(binary.BigEndian.Uint16(data[4:6]))
	if covOff >= len(data) || len(data) < 6+setCount*2 {
		return
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return
	}
	i := start
	for i < end && i < len(glyphs) {
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
		if matched, inputLen := g.tryPosChainRuleSetGlyph(data[setOff:], glyphs, adj, metrics, i); matched {
			if inputLen <= 1 {
				i++
			} else {
				i += inputLen
			}
			continue
		}
		i++
	}
}

func (g *gposTable) tryPosChainRuleSetGlyph(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int) (bool, int) {
	if len(data) < 2 {
		return false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		if ok, inputLen := g.matchAndApplyPosChainRuleGlyph(data[ruleOff:], glyphs, adj, metrics, pos); ok {
			return true, inputLen
		}
	}
	return false, 0
}

func (g *gposTable) matchAndApplyPosChainRuleGlyph(rule []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, pos int) (bool, int) {
	if len(rule) < 2 {
		return false, 0
	}
	p := 0
	backtrackCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(rule) {
		return false, 0
	}
	backtrack := make([]uint16, backtrackCount)
	for i := range backtrackCount {
		backtrack[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	inputCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if inputCount < 1 {
		return false, 0
	}
	extra := inputCount - 1
	if p+extra*2 > len(rule) {
		return false, 0
	}
	inputExtra := make([]uint16, extra)
	for i := range extra {
		inputExtra[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	lookaheadCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(rule) {
		return false, 0
	}
	lookahead := make([]uint16, lookaheadCount)
	for i := range lookaheadCount {
		lookahead[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	posCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	recs, ok := parsePosLookupRecords(rule, p, posCount)
	if !ok {
		return false, 0
	}

	if pos < backtrackCount {
		return false, 0
	}
	for i := range backtrackCount {
		if glyphs[pos-1-i].gid != backtrack[i] {
			return false, 0
		}
	}
	if pos+inputCount > len(glyphs) {
		return false, 0
	}
	for i := range extra {
		if glyphs[pos+1+i].gid != inputExtra[i] {
			return false, 0
		}
	}
	lookStart := pos + inputCount
	if lookStart+lookaheadCount > len(glyphs) {
		return false, 0
	}
	for i := range lookaheadCount {
		if glyphs[lookStart+i].gid != lookahead[i] {
			return false, 0
		}
	}
	g.applyPosRecords(glyphs, adj, metrics, pos, recs)
	return true, inputCount
}

func (g *gposTable) applyPosChainFormat2(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 12 {
		return
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	backCDOff := int(binary.BigEndian.Uint16(data[4:6]))
	inputCDOff := int(binary.BigEndian.Uint16(data[6:8]))
	lookCDOff := int(binary.BigEndian.Uint16(data[8:10]))
	setCount := int(binary.BigEndian.Uint16(data[10:12]))
	if covOff >= len(data) || backCDOff >= len(data) || inputCDOff >= len(data) ||
		lookCDOff >= len(data) || len(data) < 12+setCount*2 {
		return
	}
	cov := parseCoverage(data[covOff:])
	backCD := parseClassDef(data[backCDOff:])
	inputCD := parseClassDef(data[inputCDOff:])
	lookCD := parseClassDef(data[lookCDOff:])
	if cov == nil || backCD == nil || inputCD == nil || lookCD == nil {
		return
	}
	i := start
	for i < end && i < len(glyphs) {
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
		if matched, inputLen := g.tryPosChainClassRuleSet(data[setOff:], glyphs, adj, metrics, i, backCD, inputCD, lookCD); matched {
			if inputLen <= 1 {
				i++
			} else {
				i += inputLen
			}
			continue
		}
		i++
	}
}

func (g *gposTable) tryPosChainClassRuleSet(
	data []byte,
	glyphs []shapingGlyph,
	adj []gposAdjustment,
	metrics gposMetrics,
	pos int,
	backCD, inputCD, lookCD otClassDef,
) (bool, int) {
	if len(data) < 2 {
		return false, 0
	}
	ruleCount := int(binary.BigEndian.Uint16(data[0:2]))
	if len(data) < 2+ruleCount*2 {
		return false, 0
	}
	for ri := range ruleCount {
		ruleOff := int(binary.BigEndian.Uint16(data[2+ri*2 : 2+ri*2+2]))
		if ruleOff >= len(data) {
			continue
		}
		if ok, inputLen := g.matchAndApplyPosChainRuleClass(data[ruleOff:], glyphs, adj, metrics, pos, backCD, inputCD, lookCD); ok {
			return true, inputLen
		}
	}
	return false, 0
}

func (g *gposTable) matchAndApplyPosChainRuleClass(
	rule []byte,
	glyphs []shapingGlyph,
	adj []gposAdjustment,
	metrics gposMetrics,
	pos int,
	backCD, inputCD, lookCD otClassDef,
) (bool, int) {
	if len(rule) < 2 {
		return false, 0
	}
	p := 0
	backtrackCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(rule) {
		return false, 0
	}
	backClasses := make([]uint16, backtrackCount)
	for i := range backtrackCount {
		backClasses[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	inputCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if inputCount < 1 {
		return false, 0
	}
	extra := inputCount - 1
	if p+extra*2 > len(rule) {
		return false, 0
	}
	inputExtra := make([]uint16, extra)
	for i := range extra {
		inputExtra[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	lookaheadCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(rule) {
		return false, 0
	}
	lookClasses := make([]uint16, lookaheadCount)
	for i := range lookaheadCount {
		lookClasses[i] = binary.BigEndian.Uint16(rule[p : p+2])
		p += 2
	}
	if p+2 > len(rule) {
		return false, 0
	}
	posCount := int(binary.BigEndian.Uint16(rule[p : p+2]))
	p += 2
	recs, ok := parsePosLookupRecords(rule, p, posCount)
	if !ok {
		return false, 0
	}

	if pos < backtrackCount {
		return false, 0
	}
	for i := range backtrackCount {
		if backCD.classOf(glyphs[pos-1-i].gid) != backClasses[i] {
			return false, 0
		}
	}
	if pos+inputCount > len(glyphs) {
		return false, 0
	}
	for i := range extra {
		if inputCD.classOf(glyphs[pos+1+i].gid) != inputExtra[i] {
			return false, 0
		}
	}
	lookStart := pos + inputCount
	if lookStart+lookaheadCount > len(glyphs) {
		return false, 0
	}
	for i := range lookaheadCount {
		if lookCD.classOf(glyphs[lookStart+i].gid) != lookClasses[i] {
			return false, 0
		}
	}
	g.applyPosRecords(glyphs, adj, metrics, pos, recs)
	return true, inputCount
}

func (g *gposTable) applyPosChainFormat3(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, start, end int) {
	if len(data) < 4 {
		return
	}
	p := 2
	backtrackCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if p+backtrackCount*2 > len(data) {
		return
	}
	backCov := make([]otCoverage, backtrackCount)
	for i := range backtrackCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return
		}
		backCov[i] = parseCoverage(data[off:])
		if backCov[i] == nil {
			return
		}
	}
	if p+2 > len(data) {
		return
	}
	inputCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if inputCount < 1 || p+inputCount*2 > len(data) {
		return
	}
	inputCov := make([]otCoverage, inputCount)
	for i := range inputCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return
		}
		inputCov[i] = parseCoverage(data[off:])
		if inputCov[i] == nil {
			return
		}
	}
	if p+2 > len(data) {
		return
	}
	lookaheadCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	if p+lookaheadCount*2 > len(data) {
		return
	}
	lookCov := make([]otCoverage, lookaheadCount)
	for i := range lookaheadCount {
		off := int(binary.BigEndian.Uint16(data[p : p+2]))
		p += 2
		if off >= len(data) {
			return
		}
		lookCov[i] = parseCoverage(data[off:])
		if lookCov[i] == nil {
			return
		}
	}
	if p+2 > len(data) {
		return
	}
	posCount := int(binary.BigEndian.Uint16(data[p : p+2]))
	p += 2
	recs, ok := parsePosLookupRecords(data, p, posCount)
	if !ok {
		return
	}

	i := start
	for i < end && i < len(glyphs) {
		if i+inputCount > len(glyphs) {
			break
		}
		if !matchCoverageSequence(glyphs, i, inputCov) ||
			!matchCoverageBacktrack(glyphs, i, backCov) ||
			!matchCoverageLookahead(glyphs, i, inputCount, lookCov) {
			i++
			continue
		}
		g.applyPosRecords(glyphs, adj, metrics, i, recs)
		if inputCount <= 1 {
			i++
		} else {
			i += inputCount
		}
	}
}
