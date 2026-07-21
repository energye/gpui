// GPOS Lookup Type 3 (Cursive), Type 4 (MarkToBase), Type 6 (MarkToMark).
//
// ENGINE_GAPS G1.c — mark attachment / cursive positioning for complex scripts.
// Anchor Format 1–3 coordinates supported; device tables ignored.
//
// Reference: https://learn.microsoft.com/en-us/typography/opentype/spec/gpos
package text

import "encoding/binary"

// otAnchor is a design-unit anchor position.
type otAnchor struct {
	x  int16
	y  int16
	ok bool
}

func parseAnchor(data []byte, offset int) otAnchor {
	if offset < 0 || offset+6 > len(data) {
		return otAnchor{}
	}
	format := binary.BigEndian.Uint16(data[offset : offset+2])
	if format < 1 || format > 3 {
		return otAnchor{}
	}
	return otAnchor{
		x:  int16(binary.BigEndian.Uint16(data[offset+2 : offset+4])),
		y:  int16(binary.BigEndian.Uint16(data[offset+4 : offset+6])),
		ok: true,
	}
}

// gposMetrics supplies advances for mark attachment math.
type gposMetrics struct {
	hmtxAdv     []uint16
	numHMetrics int
}

func (m gposMetrics) advance(gid uint16) int32 {
	if m.hmtxAdv == nil {
		return 0
	}
	return int32(hmtxAdvance(m.hmtxAdv, m.numHMetrics, gid))
}

// applyCursivePos is GPOS Lookup Type 3 Format 1.
func applyCursivePos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment) {
	if len(data) < 6 || len(glyphs) < 2 {
		return
	}
	if binary.BigEndian.Uint16(data[0:2]) != 1 {
		return
	}
	covOff := int(binary.BigEndian.Uint16(data[2:4]))
	entryExitCount := int(binary.BigEndian.Uint16(data[4:6]))
	if covOff >= len(data) || len(data) < 6+entryExitCount*4 {
		return
	}
	cov := parseCoverage(data[covOff:])
	if cov == nil {
		return
	}

	for i := 0; i+1 < len(glyphs); i++ {
		idx0, ok0 := cov.contains(glyphs[i].gid)
		idx1, ok1 := cov.contains(glyphs[i+1].gid)
		if !ok0 || !ok1 || idx0 >= entryExitCount || idx1 >= entryExitCount {
			continue
		}
		rec0 := 6 + idx0*4
		rec1 := 6 + idx1*4
		exitOff := int(binary.BigEndian.Uint16(data[rec0+2 : rec0+4]))
		entryOff := int(binary.BigEndian.Uint16(data[rec1 : rec1+2]))
		if exitOff == 0 || entryOff == 0 {
			continue
		}
		exit := parseAnchor(data, exitOff)
		entry := parseAnchor(data, entryOff)
		if !exit.ok || !entry.ok {
			continue
		}
		// LTR cursive: attach entry of i+1 to exit of i (incl. prior placement).
		adj[i+1].xPlacement += exit.x - entry.x + adj[i].xPlacement
		adj[i+1].yPlacement += exit.y - entry.y + adj[i].yPlacement
	}
}

// applyMarkToBasePos is GPOS Lookup Type 4 Format 1.
func applyMarkToBasePos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, m gposMetrics) {
	applyMarkAttach(data, glyphs, adj, m, false)
}

// applyMarkToMarkPos is GPOS Lookup Type 6 Format 1.
func applyMarkToMarkPos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, m gposMetrics) {
	applyMarkAttach(data, glyphs, adj, m, true)
}

// applyMarkAttach shares MarkToBase / MarkToMark structure.
// markToMark=true: base coverage is also marks; search only previous mark2.
func applyMarkAttach(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, m gposMetrics, markToMark bool) {
	if len(data) < 12 {
		return
	}
	if binary.BigEndian.Uint16(data[0:2]) != 1 {
		return
	}
	markCovOff := int(binary.BigEndian.Uint16(data[2:4]))
	baseCovOff := int(binary.BigEndian.Uint16(data[4:6]))
	classCount := int(binary.BigEndian.Uint16(data[6:8]))
	markArrayOff := int(binary.BigEndian.Uint16(data[8:10]))
	baseArrayOff := int(binary.BigEndian.Uint16(data[10:12]))
	if markCovOff >= len(data) || baseCovOff >= len(data) ||
		markArrayOff >= len(data) || baseArrayOff >= len(data) || classCount < 1 {
		return
	}
	markCov := parseCoverage(data[markCovOff:])
	baseCov := parseCoverage(data[baseCovOff:])
	if markCov == nil || baseCov == nil {
		return
	}
	ma := data[markArrayOff:]
	if len(ma) < 2 {
		return
	}
	markCount := int(binary.BigEndian.Uint16(ma[0:2]))
	if len(ma) < 2+markCount*4 {
		return
	}
	ba := data[baseArrayOff:]
	if len(ba) < 2 {
		return
	}
	baseCount := int(binary.BigEndian.Uint16(ba[0:2]))
	baseRecSize := classCount * 2
	if len(ba) < 2+baseCount*baseRecSize {
		return
	}

	for i := range glyphs {
		markIdx, isMark := markCov.contains(glyphs[i].gid)
		if !isMark || markIdx >= markCount {
			continue
		}
		basePos := -1
		for j := i - 1; j >= 0; j-- {
			if _, ok := baseCov.contains(glyphs[j].gid); ok {
				basePos = j
				break
			}
			if !markToMark {
				// MarkToBase: skip intervening marks; stop on other glyphs.
				if _, ok := markCov.contains(glyphs[j].gid); ok {
					continue
				}
				break
			}
		}
		if basePos < 0 {
			continue
		}
		baseIdx, ok := baseCov.contains(glyphs[basePos].gid)
		if !ok || baseIdx >= baseCount {
			continue
		}
		mrec := 2 + markIdx*4
		markClass := int(binary.BigEndian.Uint16(ma[mrec : mrec+2]))
		markAnchorOff := int(binary.BigEndian.Uint16(ma[mrec+2 : mrec+4]))
		if markClass >= classCount || markAnchorOff == 0 {
			continue
		}
		markAnchor := parseAnchor(ma, markAnchorOff)
		if !markAnchor.ok {
			continue
		}
		brec := 2 + baseIdx*baseRecSize + markClass*2
		baseAnchorOff := int(binary.BigEndian.Uint16(ba[brec : brec+2]))
		if baseAnchorOff == 0 {
			continue
		}
		baseAnchor := parseAnchor(ba, baseAnchorOff)
		if !baseAnchor.ok {
			continue
		}

		// mark.x = base.x + base.place + baseAnchor.x - markAnchor.x
		// pen at mark = base.x + sum(advances base..mark-1)
		// => xPlacement = base.place + baseAnchor.x - markAnchor.x - sumAdv
		var sumAdv int32
		for j := basePos; j < i; j++ {
			sumAdv += m.advance(glyphs[j].gid)
			sumAdv += int32(adj[j].xAdvance)
		}
		adj[i].xPlacement += adj[basePos].xPlacement + baseAnchor.x - markAnchor.x - int16(sumAdv)
		adj[i].yPlacement += adj[basePos].yPlacement + baseAnchor.y - markAnchor.y
	}
}

// applyMarkToLigPos is GPOS Lookup Type 5 Format 1 (mark-to-ligature).
//
// Without per-glyph ligature component indices (GDEF/buffer), marks attach to
// the last ligature component that defines an anchor for the mark class —
// a common desktop approximation for trailing marks on fi/ffi-style ligatures.
func applyMarkToLigPos(data []byte, glyphs []shapingGlyph, adj []gposAdjustment, m gposMetrics) {
	if len(data) < 12 {
		return
	}
	if binary.BigEndian.Uint16(data[0:2]) != 1 {
		return
	}
	markCovOff := int(binary.BigEndian.Uint16(data[2:4]))
	ligCovOff := int(binary.BigEndian.Uint16(data[4:6]))
	classCount := int(binary.BigEndian.Uint16(data[6:8]))
	markArrayOff := int(binary.BigEndian.Uint16(data[8:10]))
	ligArrayOff := int(binary.BigEndian.Uint16(data[10:12]))
	if markCovOff >= len(data) || ligCovOff >= len(data) ||
		markArrayOff >= len(data) || ligArrayOff >= len(data) || classCount < 1 {
		return
	}
	markCov := parseCoverage(data[markCovOff:])
	ligCov := parseCoverage(data[ligCovOff:])
	if markCov == nil || ligCov == nil {
		return
	}
	ma := data[markArrayOff:]
	if len(ma) < 2 {
		return
	}
	markCount := int(binary.BigEndian.Uint16(ma[0:2]))
	if len(ma) < 2+markCount*4 {
		return
	}
	la := data[ligArrayOff:]
	if len(la) < 2 {
		return
	}
	ligCount := int(binary.BigEndian.Uint16(la[0:2]))
	if len(la) < 2+ligCount*2 {
		return
	}

	for i := range glyphs {
		markIdx, isMark := markCov.contains(glyphs[i].gid)
		if !isMark || markIdx >= markCount {
			continue
		}
		// Find preceding ligature glyph.
		ligPos := -1
		for j := i - 1; j >= 0; j-- {
			if _, ok := ligCov.contains(glyphs[j].gid); ok {
				ligPos = j
				break
			}
			if _, ok := markCov.contains(glyphs[j].gid); ok {
				continue // skip intervening marks
			}
			break
		}
		if ligPos < 0 {
			continue
		}
		ligIdx, ok := ligCov.contains(glyphs[ligPos].gid)
		if !ok || ligIdx >= ligCount {
			continue
		}
		attachOff := int(binary.BigEndian.Uint16(la[2+ligIdx*2 : 2+ligIdx*2+2]))
		if attachOff == 0 || attachOff >= len(la) {
			continue
		}
		attach := la[attachOff:]
		if len(attach) < 2 {
			continue
		}
		componentCount := int(binary.BigEndian.Uint16(attach[0:2]))
		if componentCount < 1 || len(attach) < 2+componentCount*classCount*2 {
			continue
		}

		mrec := 2 + markIdx*4
		markClass := int(binary.BigEndian.Uint16(ma[mrec : mrec+2]))
		markAnchorOff := int(binary.BigEndian.Uint16(ma[mrec+2 : mrec+4]))
		if markClass >= classCount || markAnchorOff == 0 {
			continue
		}
		markAnchor := parseAnchor(ma, markAnchorOff)
		if !markAnchor.ok {
			continue
		}

		// Prefer last component with a defined anchor for this class.
		var baseAnchor otAnchor
		found := false
		for c := componentCount - 1; c >= 0; c-- {
			recOff := 2 + c*classCount*2 + markClass*2
			aOff := int(binary.BigEndian.Uint16(attach[recOff : recOff+2]))
			if aOff == 0 {
				continue
			}
			baseAnchor = parseAnchor(attach, aOff)
			if baseAnchor.ok {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		var sumAdv int32
		for j := ligPos; j < i; j++ {
			sumAdv += m.advance(glyphs[j].gid)
			sumAdv += int32(adj[j].xAdvance)
		}
		adj[i].xPlacement += adj[ligPos].xPlacement + baseAnchor.x - markAnchor.x - int16(sumAdv)
		adj[i].yPlacement += adj[ligPos].yPlacement + baseAnchor.y - markAnchor.y
	}
}
