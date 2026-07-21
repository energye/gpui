// Per-font Indic positional classes from GSUB feature coverages (ENGINE_GAPS G1.c).
//
// Static Unicode tables (indicConsonantPos) cover common scripts. Real fonts encode
// below/post-base consonants under blwf / pstf / rkrf / vatu coverage.
// We harvest those coverages so final reordering can bucket glyph IDs correctly
// when the static table is incomplete or font-specific.
//
// This is not full Uniscribe OT class machinery (no per-lookup match-class
// rewriting); it only refines final below/post consonant buckets and base
// skipping after cmap when a glyph is listed in those features.
package text

import "encoding/binary"

// indicFontPosClasses maps glyph ID → static-compatible consonant position
// derived from font GSUB feature coverages.
type indicFontPosClasses struct {
	// pos is sparse: only glyphs listed under positional GSUB features.
	pos map[uint16]consPos
}

// buildIndicFontPosClasses collects coverage glyphs from blwf / pstf / rkrf / vatu
// features for the given script/lang. Returns nil when GSUB is missing or empty.
func buildIndicFontPosClasses(g *gsubTable, scriptTag, langTag [4]byte) *indicFontPosClasses {
	if g == nil {
		return nil
	}
	out := &indicFontPosClasses{pos: make(map[uint16]consPos)}
	add := func(tag [4]byte, p consPos) {
		for _, gid := range collectFeatureCoveredGlyphs(g, scriptTag, langTag, tag) {
			// Prefer below over post if a glyph appears in both (unusual).
			if cur, ok := out.pos[gid]; ok && cur == consPosBelow {
				continue
			}
			out.pos[gid] = p
		}
	}
	add(tag4('b', 'l', 'w', 'f'), consPosBelow)
	add(tag4('r', 'k', 'r', 'f'), consPosBelow) // rakar forms
	add(tag4('v', 'a', 't', 'u'), consPosBelow) // vattu
	add(tag4('p', 's', 't', 'f'), consPosPost)

	if len(out.pos) == 0 {
		return nil
	}
	return out
}

// consonantPosFor returns font-driven position when known, else static Unicode class.
func (f *indicFontPosClasses) consonantPosFor(gid uint16, r rune) consPos {
	if f != nil {
		if p, ok := f.pos[gid]; ok {
			return p
		}
	}
	return indicConsonantPos(r)
}

// collectFeatureCoveredGlyphs returns glyph IDs listed in any Coverage of lookups
// attached to the given feature tag (LangSys-ordered, script/lang selected).
func collectFeatureCoveredGlyphs(g *gsubTable, scriptTag, langTag, featureTag [4]byte) []uint16 {
	if g == nil {
		return nil
	}
	indices := collectLookupIndicesOrdered(g.scripts, g.features, scriptTag, langTag, [][4]byte{featureTag})
	seen := make(map[uint16]bool)
	var out []uint16
	for _, li := range indices {
		if int(li) >= len(g.lookups) {
			continue
		}
		for _, gid := range lookupCoveredGlyphs(&g.lookups[li]) {
			if !seen[gid] {
				seen[gid] = true
				out = append(out, gid)
			}
		}
	}
	return out
}

// lookupCoveredGlyphs walks a GSUB lookup (incl. extension) and unions Coverage glyphs.
func lookupCoveredGlyphs(lu *otLookup) []uint16 {
	if lu == nil {
		return nil
	}
	var out []uint16
	seen := make(map[uint16]bool)
	addCov := func(cov otCoverage) {
		if cov == nil {
			return
		}
		for _, gid := range coverageGlyphIDs(cov) {
			if !seen[gid] {
				seen[gid] = true
				out = append(out, gid)
			}
		}
	}
	for _, st := range lu.subtables {
		if lu.lookupType == 7 { // extension
			extType, payload := parseGSUBExtension(st)
			if payload == nil {
				continue
			}
			addCov(subtablePrimaryCoverage(extType, payload))
			continue
		}
		addCov(subtablePrimaryCoverage(lu.lookupType, st))
	}
	return out
}

// parseGSUBExtension returns (extensionLookupType, subtableData) for Type 7.
func parseGSUBExtension(data []byte) (uint16, []byte) {
	if len(data) < 8 {
		return 0, nil
	}
	// format uint16, extensionLookupType uint16, extensionOffset Offset32
	extType := binary.BigEndian.Uint16(data[2:4])
	off := int(binary.BigEndian.Uint32(data[4:8]))
	if off <= 0 || off >= len(data) {
		return 0, nil
	}
	return extType, data[off:]
}

// subtablePrimaryCoverage returns the main Coverage of common GSUB subtables
// (Types 1–6, 8): format at 0, coverageOffset at 2.
func subtablePrimaryCoverage(lookupType uint16, data []byte) otCoverage {
	if len(data) < 4 {
		return nil
	}
	switch lookupType {
	case 1, 2, 3, 4, 5, 6, 8:
		// Format 2 context may differ; primary coverage offset still at 2 for
		// format 1/2/3 of most types. Best-effort parse.
		covOff := int(binary.BigEndian.Uint16(data[2:4]))
		if covOff <= 0 || covOff >= len(data) {
			return nil
		}
		return parseCoverage(data[covOff:])
	default:
		return nil
	}
}

// coverageGlyphIDs enumerates glyph IDs in a coverage table.
func coverageGlyphIDs(cov otCoverage) []uint16 {
	switch c := cov.(type) {
	case *coverageFormat1:
		out := make([]uint16, len(c.glyphs))
		copy(out, c.glyphs)
		return out
	case *coverageFormat2:
		var out []uint16
		for _, r := range c.ranges {
			for g := r.startGlyphID; g <= r.endGlyphID; g++ {
				out = append(out, g)
				if g == 0xFFFF {
					break
				}
			}
		}
		return out
	default:
		return nil
	}
}
