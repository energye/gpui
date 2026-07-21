// GDEF table — glyph class definition for lookup flag filtering.
//
// ENGINE_GAPS G1.c: IgnoreBase/Ligature/Marks, MarkAttachmentType, and
// MarkFilteringSet (GDEF MarkGlyphSets + LookupFlag bit 4).
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gdef
package text

import "encoding/binary"

// OpenType GDEF glyph classes.
const (
	gdefClassBase      uint16 = 1
	gdefClassLigature  uint16 = 2
	gdefClassMark      uint16 = 3
	gdefClassComponent uint16 = 4
)

// Lookup flag bits (GSUB/GPOS LookupTable.lookupFlag).
const (
	lookupFlagRightToLeft         uint16 = 0x0001
	lookupFlagIgnoreBaseGlyphs    uint16 = 0x0002
	lookupFlagIgnoreLigatures     uint16 = 0x0004
	lookupFlagIgnoreMarks         uint16 = 0x0008
	lookupFlagUseMarkFilteringSet uint16 = 0x0010
	// bits 8–15: MarkAttachmentType
)

// gdefTable holds the subset of GDEF we need for lookup flags.
type gdefTable struct {
	classDef      otClassDef   // GlyphClassDef; nil if missing
	markAttachCD  otClassDef   // MarkAttachClassDef; nil if missing
	markGlyphSets []otCoverage // MarkGlyphSets coverages; nil/empty if missing
}

// parseGDEF parses raw GDEF table data. Returns nil if unusable.
func parseGDEF(data []byte) *gdefTable {
	// Version 1.0 header (12 bytes):
	//   major, minor, glyphClassDef, attachList, ligCaretList, markAttachClassDef
	// 1.2+ adds markGlyphSetsDef at offset 12 (Offset16).
	if len(data) < 6 {
		return nil
	}
	major := binary.BigEndian.Uint16(data[0:2])
	if major != 1 {
		return nil
	}
	minor := binary.BigEndian.Uint16(data[2:4])
	g := &gdefTable{}
	off := int(binary.BigEndian.Uint16(data[4:6]))
	if off > 0 && off < len(data) {
		g.classDef = parseClassDef(data[off:])
	}
	if len(data) >= 12 {
		moff := int(binary.BigEndian.Uint16(data[10:12]))
		if moff > 0 && moff < len(data) {
			g.markAttachCD = parseClassDef(data[moff:])
		}
	}
	// MarkGlyphSetsDef: present when minor >= 2 and offset at 12 is non-zero.
	if minor >= 2 && len(data) >= 14 {
		sOff := int(binary.BigEndian.Uint16(data[12:14]))
		if sOff > 0 && sOff < len(data) {
			g.markGlyphSets = parseMarkGlyphSets(data[sOff:])
		}
	}
	return g
}

// parseMarkGlyphSets parses MarkGlyphSetsTable (format 1).
//
//	uint16 format
//	uint16 markGlyphSetCount
//	Offset32 coverageOffset[markGlyphSetCount]  (from beginning of MarkGlyphSetsTable)
func parseMarkGlyphSets(data []byte) []otCoverage {
	if len(data) < 4 {
		return nil
	}
	format := binary.BigEndian.Uint16(data[0:2])
	if format != 1 {
		return nil
	}
	count := int(binary.BigEndian.Uint16(data[2:4]))
	if count < 0 || len(data) < 4+count*4 {
		return nil
	}
	sets := make([]otCoverage, count)
	for i := range count {
		off := int(binary.BigEndian.Uint32(data[4+i*4 : 8+i*4]))
		if off <= 0 || off >= len(data) {
			continue
		}
		sets[i] = parseCoverage(data[off:])
	}
	return sets
}

func (g *gdefTable) classOf(gid uint16) uint16 {
	if g == nil || g.classDef == nil {
		return 0
	}
	return g.classDef.classOf(gid)
}

func (g *gdefTable) isMark(gid uint16) bool {
	return g.classOf(gid) == gdefClassMark
}

func (g *gdefTable) isBase(gid uint16) bool {
	return g.classOf(gid) == gdefClassBase
}

func (g *gdefTable) isLigature(gid uint16) bool {
	return g.classOf(gid) == gdefClassLigature
}

// markInFilteringSet reports whether gid is listed in MarkGlyphSets[setIndex].
func (g *gdefTable) markInFilteringSet(gid uint16, setIndex int) bool {
	if g == nil || setIndex < 0 || setIndex >= len(g.markGlyphSets) {
		return false
	}
	cov := g.markGlyphSets[setIndex]
	if cov == nil {
		return false
	}
	_, ok := cov.contains(gid)
	return ok
}

// ignoredByLookupFlag reports whether gid should be skipped when matching.
// markFilterSet is the Lookup's MarkFilteringSet index, or -1 if unused.
func (g *gdefTable) ignoredByLookupFlag(gid uint16, flag uint16, markFilterSet int) bool {
	if g == nil {
		return false
	}
	c := g.classOf(gid)
	if flag&lookupFlagIgnoreMarks != 0 && c == gdefClassMark {
		return true
	}
	if flag&lookupFlagIgnoreBaseGlyphs != 0 && c == gdefClassBase {
		return true
	}
	if flag&lookupFlagIgnoreLigatures != 0 && c == gdefClassLigature {
		return true
	}
	// MarkAttachmentType (bits 8–15)
	markType := (flag >> 8) & 0xFF
	if markType != 0 && c == gdefClassMark {
		if g.markAttachCD == nil {
			return false
		}
		ac := g.markAttachCD.classOf(gid)
		if ac != markType {
			return true
		}
	}
	// MarkFilteringSet: marks NOT in the selected set are ignored.
	if flag&lookupFlagUseMarkFilteringSet != 0 && c == gdefClassMark {
		if !g.markInFilteringSet(gid, markFilterSet) {
			return true
		}
	}
	return false
}

// nextMatchIndex returns the next glyph index >= i that is not ignored by flag.
// Returns -1 if none. markFilterSet is -1 when UseMarkFilteringSet is clear.
func nextMatchIndex(glyphs []shapingGlyph, i int, flag uint16, gdef *gdefTable, markFilterSet int) int {
	for i < len(glyphs) {
		if gdef == nil || !gdef.ignoredByLookupFlag(glyphs[i].gid, flag, markFilterSet) {
			return i
		}
		i++
	}
	return -1
}

// matchSequenceSkipIgnored checks remaining components after pos, skipping
// ignored glyphs. Returns buffer walk length from pos, or 0 on no match.
func matchSequenceSkipIgnored(
	glyphs []shapingGlyph,
	pos int,
	wantGIDs []uint16,
	flag uint16,
	gdef *gdefTable,
	markFilterSet int,
) int {
	if pos < 0 || pos >= len(glyphs) {
		return 0
	}
	j := pos + 1
	for _, want := range wantGIDs {
		j = nextMatchIndex(glyphs, j, flag, gdef, markFilterSet)
		if j < 0 {
			return 0
		}
		if glyphs[j].gid != want {
			return 0
		}
		j++
	}
	return j - pos
}
