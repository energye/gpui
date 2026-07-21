// GPOS table parser and glyph positioning engine.
//
// Implements OpenType GPOS (Glyph Positioning) table parsing and application.
// Supported lookup types:
//
//   - Type 1: Single adjustment (position a single glyph)
//   - Type 2: Pair adjustment (kerning: position two adjacent glyphs)
//   - Type 3: Cursive attachment (ENGINE_GAPS G1.c)
//   - Type 4: Mark-to-base (ENGINE_GAPS G1.c)
//   - Type 5: Mark-to-ligature (ENGINE_GAPS G1.c)
//   - Type 6: Mark-to-mark (ENGINE_GAPS G1.c)
//   - Type 7: Contextual positioning (ENGINE_GAPS G1.c)
//   - Type 8: Chained contextual positioning (ENGINE_GAPS G1.c)
//   - Type 9: Extension positioning (wrapper for above types in large fonts)
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gpos
//
// This file is part of Phase 5 (ADR-048: Pure Go Font Stack).
package text

import "encoding/binary"

// gposTable holds parsed GPOS data ready for positioning.
type gposTable struct {
	scripts  []otScript
	features []otFeature
	lookups  []otLookup
	gdef     *gdefTable // optional; IgnoreMarks/Base/Ligature for matching
}

// parseGPOS parses the raw GPOS table data.
func parseGPOS(data []byte) *gposTable {
	hdr, ok := parseOTLayoutHeader(data)
	if !ok {
		return nil
	}

	var g gposTable
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

// gposAdjustment holds the positioning adjustments for a glyph.
type gposAdjustment struct {
	xPlacement int16
	yPlacement int16
	xAdvance   int16
	yAdvance   int16
}

// applyGPOS applies GPOS positioning to a glyph buffer.
// Returns per-glyph adjustments in font units.
// metrics supplies hmtx advances for mark attachment (may be zero-valued).
func (g *gposTable) applyGPOS(
	glyphs []shapingGlyph,
	scriptTag, langTag [4]byte,
	desiredTags [][4]byte,
	metrics gposMetrics,
) []gposAdjustment {
	adjustments := make([]gposAdjustment, len(glyphs))

	lookupIndices := collectLookupIndices(g.scripts, g.features, scriptTag, langTag, desiredTags)
	for _, li := range lookupIndices {
		if int(li) >= len(g.lookups) {
			continue
		}
		g.applyLookup(&g.lookups[li], glyphs, adjustments, metrics)
	}
	return adjustments
}

// applyLookup applies a single GPOS lookup.
func (g *gposTable) applyLookup(lu *otLookup, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics) {
	lookupType := lu.lookupType
	flag := lu.lookupFlag
	mfs := lu.markFilterSet

	// Extension positioning (Type 9) wraps another lookup type.
	if lookupType == 9 {
		g.applyExtensionLookup(lu, glyphs, adj, metrics)
		return
	}

	for _, st := range lu.subtables {
		g.applySubtable(lookupType, st, glyphs, adj, metrics, flag, mfs)
	}
}

// applyExtensionLookup handles GPOS Lookup Type 9 (Extension Positioning).
func (g *gposTable) applyExtensionLookup(lu *otLookup, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics) {
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
		g.applySubtable(extType, st[extOffset:], glyphs, adj, metrics, lu.lookupFlag, lu.markFilterSet)
	}
}

// applySubtable dispatches to the correct positioning function.
func (g *gposTable) applySubtable(lookupType uint16, data []byte, glyphs []shapingGlyph, adj []gposAdjustment, metrics gposMetrics, flag uint16, markFilterSet int) {
	switch lookupType {
	case 1:
		applySinglePos(data, glyphs, adj)
	case 2:
		applyPairPosFlag(data, glyphs, adj, flag, g.gdef, markFilterSet)
	case 3:
		applyCursivePos(data, glyphs, adj)
	case 4:
		applyMarkToBasePos(data, glyphs, adj, metrics)
	case 5:
		applyMarkToLigPos(data, glyphs, adj, metrics)
	case 6:
		applyMarkToMarkPos(data, glyphs, adj, metrics)
	case 7:
		g.applyContextualPos(data, glyphs, adj, metrics)
	case 8:
		g.applyChainedPos(data, glyphs, adj, metrics)
	default:
	}
}

// --- Lookup Type 1: Single Adjustment ---

// applySinglePos applies single glyph position adjustment.
func applySinglePos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment) {
	if len(data) < 6 {
		return
	}
	format := binary.BigEndian.Uint16(data[0:2])
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	if covOffset >= len(data) {
		return
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return
	}
	valueFormat := binary.BigEndian.Uint16(data[4:6])

	switch format {
	case 1:
		// Format 1: same ValueRecord for all covered glyphs.
		vr, _ := parseValueRecord(data, 6, valueFormat)
		for i := range glyphs {
			if _, ok := cov.contains(glyphs[i].gid); ok {
				adj[i].xPlacement += vr.xPlacement
				adj[i].yPlacement += vr.yPlacement
				adj[i].xAdvance += vr.xAdvance
				adj[i].yAdvance += vr.yAdvance
			}
		}
	case 2:
		// Format 2: array of ValueRecords, one per covered glyph.
		vrSize := valueRecordSize(valueFormat)
		valCount := int(binary.BigEndian.Uint16(data[6:8]))
		for i := range glyphs {
			idx, ok := cov.contains(glyphs[i].gid)
			if !ok || idx >= valCount {
				continue
			}
			vr, _ := parseValueRecord(data, 8+idx*vrSize, valueFormat)
			adj[i].xPlacement += vr.xPlacement
			adj[i].yPlacement += vr.yPlacement
			adj[i].xAdvance += vr.xAdvance
			adj[i].yAdvance += vr.yAdvance
		}
	}
}

// --- Lookup Type 2: Pair Adjustment (Kerning) ---

// applyPairPos applies pair positioning (kerning) to adjacent glyph pairs.
func applyPairPos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment) {
	applyPairPosFlag(data, glyphs, adj, 0, nil, -1)
}

// applyPairPosFlag is applyPairPos with lookup flags (IgnoreMarks / MarkFilteringSet).
func applyPairPosFlag(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, flag uint16, gdef *gdefTable, markFilterSet int) {
	if len(data) < 2 {
		return
	}
	format := binary.BigEndian.Uint16(data[0:2])
	switch format {
	case 1:
		applyPairPosFormat1Flag(data, glyphs, adj, flag, gdef, markFilterSet)
	case 2:
		applyPairPosFormat2Flag(data, glyphs, adj, flag, gdef, markFilterSet)
	}
}

// applyPairPosFormat1 applies pair adjustment using specific glyph pairs.
func applyPairPosFormat1(data []byte, glyphs []shapingGlyph, adj []gposAdjustment) {
	applyPairPosFormat1Flag(data, glyphs, adj, 0, nil, -1)
}

func applyPairPosFormat1Flag(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, flag uint16, gdef *gdefTable, markFilterSet int) {
	if len(data) < 10 {
		return
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	valueFormat1 := binary.BigEndian.Uint16(data[4:6])
	valueFormat2 := binary.BigEndian.Uint16(data[6:8])
	pairSetCount := int(binary.BigEndian.Uint16(data[8:10]))
	if len(data) < 10+pairSetCount*2 || covOffset >= len(data) {
		return
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return
	}
	vr1Size := valueRecordSize(valueFormat1)
	vr2Size := valueRecordSize(valueFormat2)

	for i := 0; i < len(glyphs); i++ {
		idx, ok := cov.contains(glyphs[i].gid)
		if !ok || idx >= pairSetCount {
			continue
		}
		psOffset := int(binary.BigEndian.Uint16(data[10+idx*2 : 10+idx*2+2]))
		if psOffset >= len(data) {
			continue
		}
		ps := data[psOffset:]
		if len(ps) < 2 {
			continue
		}
		pairValueCount := int(binary.BigEndian.Uint16(ps[0:2]))
		recordSize := 2 + vr1Size + vr2Size // uint16 secondGlyph + vr1 + vr2

		// Second of pair: next non-ignored glyph when lookup flags say so.
		j := i + 1
		if gdef != nil && (flag&(lookupFlagIgnoreBaseGlyphs|lookupFlagIgnoreLigatures|lookupFlagIgnoreMarks|lookupFlagUseMarkFilteringSet) != 0 || (flag>>8)&0xFF != 0) {
			j = nextMatchIndex(glyphs, i+1, flag, gdef, markFilterSet)
			if j < 0 {
				continue
			}
		} else if j >= len(glyphs) {
			continue
		}
		secondGID := glyphs[j].gid
		// Binary search for secondGlyph in the PairSet.
		found := searchPairSet(ps[2:], pairValueCount, recordSize, secondGID)
		if found < 0 {
			continue
		}
		recordOff := 2 + found*recordSize

		// Parse value records.
		vr1, n1 := parseValueRecord(ps, recordOff+2, valueFormat1)
		adj[i].xPlacement += vr1.xPlacement
		adj[i].yPlacement += vr1.yPlacement
		adj[i].xAdvance += vr1.xAdvance
		adj[i].yAdvance += vr1.yAdvance

		if valueFormat2 != 0 {
			vr2, _ := parseValueRecord(ps, recordOff+2+n1, valueFormat2)
			adj[j].xPlacement += vr2.xPlacement
			adj[j].yPlacement += vr2.yPlacement
			adj[j].xAdvance += vr2.xAdvance
			adj[j].yAdvance += vr2.yAdvance
		}
	}
}

// searchPairSet binary-searches for secondGlyph in a PairSet's PairValueRecords.
// The records start at data[0] and each is recordSize bytes. The secondGlyph
// is the first uint16 in each record.
func searchPairSet(data []byte, count, recordSize int, secondGlyph uint16) int {
	lo, hi := 0, count-1
	for lo <= hi {
		mid := (lo + hi) / 2
		off := mid * recordSize
		if off+2 > len(data) {
			return -1
		}
		gid := binary.BigEndian.Uint16(data[off : off+2])
		if gid == secondGlyph {
			return mid
		}
		if gid < secondGlyph {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return -1
}

// applyPairPosFormat2 applies pair adjustment using glyph class pairs.
func applyPairPosFormat2(data []byte, glyphs []shapingGlyph, adj []gposAdjustment) {
	applyPairPosFormat2Flag(data, glyphs, adj, 0, nil, -1)
}

func applyPairPosFormat2Flag(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, flag uint16, gdef *gdefTable, markFilterSet int) {
	if len(data) < 16 {
		return
	}
	covOffset := int(binary.BigEndian.Uint16(data[2:4]))
	valueFormat1 := binary.BigEndian.Uint16(data[4:6])
	valueFormat2 := binary.BigEndian.Uint16(data[6:8])
	classDef1Offset := int(binary.BigEndian.Uint16(data[8:10]))
	classDef2Offset := int(binary.BigEndian.Uint16(data[10:12]))
	class1Count := int(binary.BigEndian.Uint16(data[12:14]))
	class2Count := int(binary.BigEndian.Uint16(data[14:16]))

	if covOffset >= len(data) || classDef1Offset >= len(data) || classDef2Offset >= len(data) {
		return
	}
	cov := parseCoverage(data[covOffset:])
	if cov == nil {
		return
	}
	cd1 := parseClassDef(data[classDef1Offset:])
	cd2 := parseClassDef(data[classDef2Offset:])
	if cd1 == nil || cd2 == nil {
		return
	}

	vr1Size := valueRecordSize(valueFormat1)
	vr2Size := valueRecordSize(valueFormat2)
	recordSize := vr1Size + vr2Size
	arrayStart := 16

	for i := 0; i < len(glyphs); i++ {
		if _, ok := cov.contains(glyphs[i].gid); !ok {
			continue
		}
		j := i + 1
		if gdef != nil && (flag&(lookupFlagIgnoreBaseGlyphs|lookupFlagIgnoreLigatures|lookupFlagIgnoreMarks|lookupFlagUseMarkFilteringSet) != 0 || (flag>>8)&0xFF != 0) {
			j = nextMatchIndex(glyphs, i+1, flag, gdef, markFilterSet)
			if j < 0 {
				continue
			}
		} else if j >= len(glyphs) {
			continue
		}
		c1 := int(cd1.classOf(glyphs[i].gid))
		c2 := int(cd2.classOf(glyphs[j].gid))
		if c1 >= class1Count || c2 >= class2Count {
			continue
		}

		recordIndex := c1*class2Count + c2
		recordOff := arrayStart + recordIndex*recordSize

		vr1, n1 := parseValueRecord(data, recordOff, valueFormat1)
		adj[i].xPlacement += vr1.xPlacement
		adj[i].yPlacement += vr1.yPlacement
		adj[i].xAdvance += vr1.xAdvance
		adj[i].yAdvance += vr1.yAdvance

		if valueFormat2 != 0 {
			vr2, _ := parseValueRecord(data, recordOff+n1, valueFormat2)
			adj[j].xPlacement += vr2.xPlacement
			adj[j].yPlacement += vr2.yPlacement
			adj[j].xAdvance += vr2.xAdvance
			adj[j].yAdvance += vr2.yAdvance
		}
	}
}
