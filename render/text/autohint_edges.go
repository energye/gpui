package text

import (
	"sort"
)

// Edge detection, blue edge matching, and grid-fitting for the auto-hinter.
//
// An "edge" groups nearby segments at similar positions into a single
// hinting feature. Edges are then linked to form stems, matched to
// blue zones, and grid-fitted.
//
// References:
//   - FreeType aflatin.c:2154 af_latin_hints_compute_edges
//   - FreeType aflatin.c:2529 af_latin_hints_compute_blue_edges
//   - FreeType aflatin.c:4220 af_latin_hint_edges
//   - skrifa topo/edges.rs compute_edges
//   - skrifa hint/edges.rs hint_edges

// hintEdge represents an edge — a group of segments at similar positions.
// opos and pos are in 26.6 fixed-point (1 unit = 1/64 pixel), matching
// skrifa topo/mod.rs Edge { fpos: i16, opos: i32, pos: i32 }.
type hintEdge struct {
	// fpos is the original (unscaled font unit) position.
	// Stored as float32 for compatibility with segment positions (which
	// are averaged font-unit coordinates). Compared against point.fx/fy.
	fpos float32

	// opos is the original scaled pixel position, 26.6 fixed-point.
	// Matches skrifa Edge.opos: i32 (26.6).
	opos int32

	// pos is the current (possibly grid-fitted) position, 26.6 fixed-point.
	// Matches skrifa Edge.pos: i32 (26.6).
	pos int32

	// dir is the edge direction.
	dir hintDirection

	// flags for edge classification.
	flags uint32

	// linkIdx is the index of the linked edge (stem partner), or -1.
	linkIdx int16

	// serifIdx is the index of the serif edge, or -1.
	serifIdx int16

	// blueEdge points to the blue zone this edge is aligned to, or nil.
	blueEdge *scaledWidth

	// scale is a cached interpolation scale factor (16.16) for
	// align_strong_points. Matches skrifa Edge.scale: i32.
	// Zero means not yet computed.
	scale int32

	// segments belonging to this edge.
	segmentIndices []int
}

// computeEdgeDistThreshold computes the edge distance threshold for grouping
// segments into edges. Default and CJK use different computations.
//
// See skrifa topo/edges.rs:49-64.
func computeEdgeDistThreshold(axis *scaledAxisMetrics, group scriptGroup) float32 {
	if axis.scale <= 0 {
		edt := axis.edgeDistThreshold
		if edt > 0.25 {
			edt = 0.25
		}
		return edt
	}
	if group == scriptGroupDefault {
		scaledThreshold := axis.edgeDistThreshold
		if scaledThreshold > 0.25 {
			scaledThreshold = 0.25
		}
		return float32(float64(scaledThreshold) / axis.scale)
	}
	// CJK: different computation.
	scaled := axis.edgeDistThreshold
	if scaled > 0.25 {
		return float32(0.25 / axis.scale)
	}
	return float32(float64(axis.edgeDistThreshold) / axis.scale)
}

// computeEdges groups segments into edges.
// Segments with similar positions (within the edge distance threshold)
// and same direction are merged into a single edge.
//
// The group parameter controls CJK-specific behavior:
//   - CJK: no segment filtering (height/width/direction), different edge distance
//     threshold computation, no break on first match (pick closest), skip
//     directionless segment pass.
//   - Default: existing behavior (segment filtering, break on first match).
//
// See FreeType aflatin.c:2154 af_latin_hints_compute_edges.
// See skrifa topo/edges.rs compute_edges.
//
//nolint:gocognit,gocyclo,cyclop,funlen // FreeType aflatin.c port — algorithmic complexity is inherent
func computeEdges(segments []hintSegment, axis *scaledAxisMetrics, dim hintDimension, group scriptGroup, topToBottom bool) []*hintEdge {
	if len(segments) == 0 {
		return nil
	}

	// Edge distance threshold: segment positions may be in font units (contour
	// path) or scaled pixels (legacy path). The threshold must match.
	//
	// Default (skrifa edges.rs:49-55):
	//   edge_distance_threshold = fixed_div(min(threshold*scale, 16), scale)
	// CJK (skrifa edges.rs:57-64):
	//   if threshold*scale > 16: fixed_div(16, scale), else: threshold (unscaled)
	edgeDistThreshold := computeEdgeDistThreshold(axis, group)

	// Segment length threshold: ignore segments shorter than 1px
	// (horizontal dimension only — helps with serif fonts).
	// Skrifa: fixed_div(64, y_scale) — converts 1px to font units.
	var segLenThreshold float32
	if dim == dimHorizontal && axis.scale > 0 {
		segLenThreshold = float32(1.0 / axis.scale)
	}

	// Segment width threshold: ignore segments wider than 0.5px.
	// Skrifa: fixed_div(32, scale) — converts 0.5px to font units.
	var segWidthThreshold float32
	if axis.scale > 0 {
		segWidthThreshold = float32(0.5 / axis.scale)
	}

	var edges []*hintEdge

	// First pass: create edges from segments.
	for si, seg := range segments {
		// Default: skip directionless segments (and short/wide ones).
		// CJK: include ALL directional segments, no height/width filtering.
		// Matches skrifa edges.rs:71-86 (the @deprecated filter is Default-only).
		if group == scriptGroupDefault {
			if seg.dir == dirNone || seg.height < segLenThreshold || seg.delta > segWidthThreshold {
				continue
			}
			// Ignore serif edges that are smaller than 1.5 pixels.
			if seg.serifIdx >= 0 && 2*seg.height < 3*segLenThreshold {
				continue
			}
		}

		// Look for an existing edge at a nearby position with same direction.
		bestDist := float32(1e10)
		bestEdgeIdx := -1
		for ei, edge := range edges {
			dist := seg.pos - edge.fpos
			if dist < 0 {
				dist = -dist
			}
			if dist < edgeDistThreshold && edge.dir == seg.dir && dist < bestDist {
				if group == scriptGroupDefault {
					bestEdgeIdx = ei
					break // Default: first match wins
				}
				// CJK: check whether all linked segments of the candidate edge
				// can make a single edge (FreeType afcjk.c:1073-1091).
				// If this segment has a link, its link must be close to the
				// links of the segments already attached to the candidate edge,
				// otherwise the edge cannot be shared.
				if seg.linkIdx >= 0 {
					linkOk := true
					for _, existingIdx := range edge.segmentIndices {
						existing := segments[existingIdx]
						if existing.linkIdx < 0 {
							continue
						}
						dist2 := segLinkDist(segments, int(seg.linkIdx), int(existing.linkIdx))
						if dist2 >= edgeDistThreshold {
							linkOk = false
							break
						}
					}
					if !linkOk {
						continue
					}
				}
				// CJK: don't break, pick closest.
				bestDist = dist
				bestEdgeIdx = ei
			}
		}

		if bestEdgeIdx < 0 {
			// Create new edge. Scale font-unit position to 26.6 pixel coordinates.
			opos := f26dot6FromFloat(float64(seg.pos))
			if axis.scale > 0 {
				opos = f26dot6FromFloat(float64(seg.pos) * axis.scale)
			}
			edge := &hintEdge{
				fpos:     seg.pos,
				opos:     opos,
				pos:      opos,
				dir:      seg.dir,
				linkIdx:  -1,
				serifIdx: -1,
			}
			edge.segmentIndices = append(edge.segmentIndices, si)
			edges = append(edges, edge)
			segments[si].edgeIdx = int16(len(edges) - 1)
		} else {
			// Add segment to existing edge.
			found := edges[bestEdgeIdx]
			found.segmentIndices = append(found.segmentIndices, si)
			segments[si].edgeIdx = int16(edgeIndex(edges, found))
		}
	}

	// Sort edges by position, matching FreeType af_axis_hints_new_edge
	// (afhints.c:197-253) and skrifa Axis::insert_edge (topo/mod.rs): ascending
	// fpos, or descending for hint_top_to_bottom scripts (Devanagari, Bengali,
	// Gurmukhi, Gothic, Mongolian — afhints.c:257-268). Within equal fpos, a
	// new edge with the minor direction keeps shifting left past same-position
	// edges while one with the major direction stops, so minor-direction edges
	// come first in reverse detection order, then major-direction edges in
	// detection order. Major dirs: HORZ=UP, VERT=LEFT (afhints.c:942-948).
	majorDir := dirUp
	if dim == dimVertical {
		majorDir = dirLeft
	}
	edgeOrder := make([]int, len(edges))
	for i := range edges {
		edgeOrder[i] = i
	}
	sort.SliceStable(edgeOrder, func(i, j int) bool {
		a, b := edges[edgeOrder[i]], edges[edgeOrder[j]]
		if a.fpos != b.fpos {
			if topToBottom {
				return a.fpos > b.fpos
			}
			return a.fpos < b.fpos
		}
		am := a.dir == majorDir
		bm := b.dir == majorDir
		if am != bm {
			// Minor-direction edges precede major-direction ones.
			return bm
		}
		if am {
			// Major direction: detection order.
			return edgeOrder[i] < edgeOrder[j]
		}
		// Minor direction: reverse detection order.
		return edgeOrder[i] > edgeOrder[j]
	})
	sorted := make([]*hintEdge, len(edges))
	for i, oi := range edgeOrder {
		sorted[i] = edges[oi]
	}
	edges = sorted

	// Update segment->edge mappings after sort.
	for ei, edge := range edges {
		for _, si := range edge.segmentIndices {
			segments[si].edgeIdx = int16(ei)
		}
	}

	// Directionless segment pass (Default only).
	// CJK skips this entirely.
	if group == scriptGroupDefault {
		for si, seg := range segments {
			if seg.dir != dirNone {
				continue
			}
			for ei, edge := range edges {
				dist := seg.pos - edge.fpos
				if dist < 0 {
					dist = -dist
				}
				if dist < edgeDistThreshold {
					edges[ei].segmentIndices = append(edges[ei].segmentIndices, si)
					segments[si].edgeIdx = int16(ei)
					break
				}
			}
		}
	}

	// Second pass: compute edge properties from their segments.
	for _, edge := range edges {
		roundCount := 0
		straightCount := 0

		for _, si := range edge.segmentIndices {
			seg := &segments[si]

			if (seg.flags & edgeFlagRound) != 0 {
				roundCount++
			} else {
				straightCount++
			}

			// Link edges based on segment links.
			// FreeType af_cjk_hints_compute_edges (afcjk.c:1229-1245): when the
			// edge already has a link, only switch to the new candidate if the
			// segment pair is strictly closer than the edge pair (seg_delta <
			// edge_delta). This keeps the first stem link (e.g. 528->680) from
			// being clobbered by a later serif segment link (560->712).
			if seg.linkIdx >= 0 {
				linkedSeg := &segments[seg.linkIdx]
				if linkedSeg.edgeIdx >= 0 {
					newLink := int(linkedSeg.edgeIdx)
					if edge.linkIdx >= 0 {
						edgeDelta := edge.fpos - edges[edge.linkIdx].fpos
						if edgeDelta < 0 {
							edgeDelta = -edgeDelta
						}
						segDelta := seg.pos - linkedSeg.pos
						if segDelta < 0 {
							segDelta = -segDelta
						}
						if segDelta < edgeDelta {
							edge.linkIdx = int16(newLink)
						}
					} else {
						edge.linkIdx = int16(newLink)
					}
				}
			}
			if seg.serifIdx >= 0 {
				serifSeg := &segments[seg.serifIdx]
				if serifSeg.edgeIdx >= 0 {
					newSerif := int(serifSeg.edgeIdx)
					if edge.serifIdx >= 0 {
						edgeDelta := edge.fpos - edges[edge.serifIdx].fpos
						if edgeDelta < 0 {
							edgeDelta = -edgeDelta
						}
						segDelta := seg.pos - serifSeg.pos
						if segDelta < 0 {
							segDelta = -segDelta
						}
						if segDelta < edgeDelta {
							edge.serifIdx = int16(newSerif)
						}
					} else {
						edge.serifIdx = int16(newSerif)
					}
				}
			}
		}

		// Set round/straight flags.
		edge.flags = 0
		if roundCount > 0 && roundCount >= straightCount {
			edge.flags |= edgeFlagRound
		}

		// Prefer link over serif.
		if edge.serifIdx >= 0 && edge.linkIdx >= 0 {
			edge.serifIdx = -1
		}
	}

	return edges
}

// edgeIndex returns the index of an edge in the edges slice.
// segLinkDist returns the absolute distance between the linked segments
// of two segments. Matches FreeType AF_SEGMENT_DIST(link, link1) in
// afcjk.c af_cjk_hints_compute_edges.
func segLinkDist(segments []hintSegment, linkIdx1, linkIdx2 int) float32 {
	if linkIdx1 < 0 || linkIdx2 < 0 || linkIdx1 >= len(segments) || linkIdx2 >= len(segments) {
		return float32(1e10)
	}
	d := segments[linkIdx1].pos - segments[linkIdx2].pos
	if d < 0 {
		d = -d
	}
	return d
}

func edgeIndex(edges []*hintEdge, target *hintEdge) int {
	for i, e := range edges {
		if e == target {
			return i
		}
	}
	return -1
}

// computeBlueEdges matches edges to blue zones.
// For each edge, find the closest blue zone and link them.
//
// The comparison is done in font units (edge.fpos vs unscaled blue position),
// and the distance is then scaled for threshold comparison. This matches
// skrifa topo/edges.rs:compute_blue_edges which compares fpos to unscaled
// blue positions.
//
// The group parameter controls CJK-specific behavior:
//   - CJK: picks whichever of position/overshoot is closer to edge.fpos
//     (instead of Default's round-edge overshoot check).
//   - Default: existing behavior.
//
// See FreeType aflatin.c:2529 af_latin_hints_compute_blue_edges.
// See skrifa topo/edges.rs:compute_blue_edges.
//
//nolint:gocognit,nestif // FreeType/skrifa port — algorithmic complexity is inherent
func computeBlueEdges(edges []*hintEdge, axis *scaledAxisMetrics, group scriptGroup) {
	// Initial threshold: UPM/40 scaled, capped at 0.5px = 32 in 26.6.
	// Matches skrifa: fixed_mul(scale.units_per_em / 40, axis_scale).min(64/2)
	initialThreshold := int32(32) // 0.5px in 26.6
	if axis.unitsPerEm > 0 {
		scaledThreshold := fixedMul26dot6(int32(axis.unitsPerEm/40), axis.scale16dot16)
		if scaledThreshold < initialThreshold {
			initialThreshold = scaledThreshold
		}
	}

	// Major direction for blue zone matching.
	// TrueType outlines are clockwise (Y-up), so:
	//   - H-axis (vertical stems): major = dirDown
	//   - V-axis (horizontal stems): major = dirRight
	// However, our segment detection follows the counter-clockwise convention
	// (matching FreeType/skrifa default orientation None):
	//   - H-axis: major = dirUp
	//   - V-axis: major = dirLeft
	//
	// See skrifa topo/mod.rs:96-101: Axis::reset major_dir selection.
	majorDir := axis.majorDir

	for _, edge := range edges {
		var bestBlue *scaledWidth
		bestDist := initialThreshold

		for bi := range axis.blues {
			blue := &axis.blues[bi]
			if !blue.isActive {
				continue
			}

			// For top blue zones, check edges going against the major direction.
			// For bottom blue zones, check edges going in the major direction.
			// Skrifa: is_top ^ is_major_dir (XOR — match when they differ).
			isTopBlue := blue.flags.isTopLike()
			isMajorDir := edge.dir == majorDir

			// Top zones match non-major edges, bottom zones match major edges.
			if isTopBlue == isMajorDir {
				continue
			}

			// Select reference position and matching blue.
			// Default: always use position; CJK: pick closer of position/overshoot.
			var refPos int32
			var matchingBlue *scaledWidth
			if group == scriptGroupDefault {
				refPos = blue.unscaledRef
				matchingBlue = &blue.reference
			} else {
				// CJK: pick whichever is closer to edge.
				// skrifa: (edge.fpos as i32 - unscaled_blue.position).abs()
				edgeFposI32 := int32(edge.fpos)
				distPos := edgeFposI32 - blue.unscaledRef
				if distPos < 0 {
					distPos = -distPos
				}
				distShoot := edgeFposI32 - blue.unscaledShoot
				if distShoot < 0 {
					distShoot = -distShoot
				}
				if distPos > distShoot {
					refPos = blue.unscaledShoot
					matchingBlue = &blue.overshoot
				} else {
					refPos = blue.unscaledRef
					matchingBlue = &blue.reference
				}
			}

			// Compare edge.fpos (font units) to reference position,
			// then scale the distance for threshold comparison.
			// Skrifa: fixed_mul((edge.fpos as i32 - ref_pos).abs(), axis_scale)
			edgeDist := int32(edge.fpos) - refPos
			if edgeDist < 0 {
				edgeDist = -edgeDist
			}
			dist := fixedMul26dot6(edgeDist, axis.scale16dot16)
			if dist < bestDist {
				bestDist = dist
				bestBlue = matchingBlue
			}

			// For Default group round edges, also compare to unscaled overshoot.
			// CJK already handled position/overshoot selection above.
			if group == scriptGroupDefault {
				if (edge.flags&edgeFlagRound) != 0 && dist != 0 {
					isUnderRef := int32(edge.fpos) < refPos
					if isTopBlue != isUnderRef {
						shootDist := int32(edge.fpos) - blue.unscaledShoot
						if shootDist < 0 {
							shootDist = -shootDist
						}
						sDist := fixedMul26dot6(shootDist, axis.scale16dot16)
						if sDist < bestDist {
							bestDist = sDist
							bestBlue = &blue.overshoot
						}
					}
				}
			}
		}

		if bestBlue != nil {
			edge.blueEdge = bestBlue
		}
	}
}

// hintEdges performs the main grid-fitting of edges.
// The algorithm proceeds in three passes:
//  1. Align edges to blue zones
//  2. Align stem edges (linked pairs)
//  3. Align remaining edges (serifs, singles)
//
// The group parameter controls CJK-specific behavior in stem alignment
// and remaining edge alignment.
//
// See FreeType aflatin.c:4220 af_latin_hint_edges.
// See skrifa hint/edges.rs hint_edges.
func hintEdges(edges []*hintEdge, axis *scaledAxisMetrics, group scriptGroup, topToBottom bool, dim hintDimension) {
	if len(edges) == 0 {
		return
	}

	// Pass 1: Blue zone alignment.
	anchorIdx := alignEdgesToBlues(edges, axis, group)

	// Pass 2: Stem edges.
	serifCount := 0
	anchorIdx = alignStemEdges(edges, axis, anchorIdx, &serifCount, group, topToBottom, dim)

	// Pass 2.5: Symmetry correction for m-shaped glyphs (FreeType
	// af_cjk_hint_edges, afcjk.c:2078-2116). CJK glyphs with exactly 6 or 12
	// vertical edges whose three stems are evenly spaced (< 8/64px apart in
	// span) get their third stem and trailing serifs shifted so the glyph
	// stays symmetric about the second stem.
	if group != scriptGroupDefault && dim == dimHorizontal {
		applyMSymmetryCJK(edges)
	}

	// Pass 3: Remaining edges (serifs, singles).
	if serifCount > 0 || anchorIdx < 0 {
		alignRemainingEdges(edges, anchorIdx, group, topToBottom)
	}
}

// alignEdgesToBlues aligns edges linked to blue zones.
// Returns the index of the first anchor edge, or -1.
// All positions (edge.pos, blue.fitted) are in 26.6 fixed-point.
//
// See FreeType aflatin.c:4250-4340.
func alignEdgesToBlues(edges []*hintEdge, axis *scaledAxisMetrics, group scriptGroup) int {
	_ = group // Used by caller for dim check; CJK allows blues on both axes.
	anchorIdx := -1

	for i, edge := range edges {
		if (edge.flags & edgeFlagDone) != 0 {
			continue
		}

		// FreeType aflatin.c:3028-3112. The blue zone may sit on this edge
		// OR on its stem partner. FT scans edges in order and treats the
		// scanned edge as the anchor (`anchor = edge`, line 3111) even when
		// the partner holds the blue zone. Using the wrong anchor changes
		// subsequent SERIF_LINK2 offsets, so mirror the flip logic.
		blueIdx := -1
		if edge.blueEdge != nil {
			blueIdx = i
		} else if edge.linkIdx >= 0 {
			link := edges[edge.linkIdx]
			if link.blueEdge != nil {
				blueIdx = int(edge.linkIdx)
			}
		}
		if blueIdx < 0 {
			continue
		}

		blueEdge := edges[blueIdx]
		blue := blueEdge.blueEdge
		blueEdge.pos = blue.fitted // 26.6 fixed-point
		blueEdge.flags |= edgeFlagDone

		// Align the stem partner if it does not carry its own blue zone.
		if blueIdx == i {
			if edge.linkIdx >= 0 {
				link := edges[edge.linkIdx]
				if link.blueEdge == nil {
					alignLinkedEdge(edges, axis, i, int(edge.linkIdx), group)
					link.flags |= edgeFlagDone
				}
			}
		} else {
			alignLinkedEdge(edges, axis, blueIdx, i, group)
			edge.flags |= edgeFlagDone
		}

		if anchorIdx < 0 {
			anchorIdx = i
		}
	}

	return anchorIdx
}

// alignStemEdges aligns stem edges (linked pairs) to the pixel grid.
// Uses computeStemWidth for consistent stem widths.
// All positions are in 26.6 fixed-point.
//
// The group parameter controls CJK-specific behavior:
//   - CJK: skips stems too close together (< 1px), uses hintNormalStemCJK.
//   - Default: existing behavior.
//
// See FreeType aflatin.c:4344-4570.
// See skrifa hint/edges.rs align_stem_edges.
//
//nolint:gocognit,nestif // FreeType aflatin.c port — algorithmic complexity is inherent
func alignStemEdges(edges []*hintEdge, axis *scaledAxisMetrics, anchorIdx int, serifCount *int, group scriptGroup, topToBottom bool, dim hintDimension) int {
	var lastStemPos int32 = -1000000 // sentinel
	var delta int32

	for i, edge := range edges {
		if (edge.flags & edgeFlagDone) != 0 {
			continue
		}

		if edge.linkIdx < 0 {
			*serifCount++
			continue
		}

		edge2Idx := int(edge.linkIdx)
		edge2 := edges[edge2Idx]

		// CJK: skip stems that are too close (< 1px = 64 in 26.6).
		if group != scriptGroupDefault {
			if lastStemPos > -1000000 {
				if edge.pos < lastStemPos+64 || edge2.pos < lastStemPos+64 {
					*serifCount++
					continue
				}
			}
		}

		if edge2.blueEdge != nil {
			// Edge2 already positioned by blue zone.
			alignLinkedEdge(edges, axis, edge2Idx, i, group)
			edge.flags |= edgeFlagDone
			continue
		}

		if group == scriptGroupDefault {
			// Default stem alignment.
			orgLen := edge2.opos - edge.opos
			curLen := computeStemWidth(axis, orgLen, edge.flags, edge2.flags)

			if anchorIdx < 0 {
				positionFirstStem(edge, edge2, orgLen, curLen)
				anchorIdx = i
			} else {
				anchor := edges[anchorIdx]
				positionSubsequentStem(edge, edge2, anchor, orgLen, curLen)
			}

			edge.flags |= edgeFlagDone
			edge2.flags |= edgeFlagDone

			// Bound check. Matches skrifa edges.rs adjust_link (LinkDir::Prev);
			// top_to_bottom reverses the order check (edges.rs:454).
			if i > 0 {
				orderBroken := edge.pos < edges[i-1].pos
				if topToBottom {
					orderBroken = edge.pos > edges[i-1].pos
				}
				if orderBroken {
					if edge.linkIdx >= 0 {
						linkPos := edges[edge.linkIdx].pos
						d := linkPos - edges[i-1].pos
						if d < 0 {
							d = -d
						}
						if d > 16 {
							edge.pos = edges[i-1].pos
						}
					}
				}
			}
		} else {
			// CJK stem alignment.
			if edge2Idx < i {
				// Edge2 comes before edge — align edge via linked edge.
				lastStemPos = edge.pos
				edge.flags |= edgeFlagDone
				alignLinkedEdge(edges, axis, edge2Idx, i, group)
				continue
			}

			stemDelta := hintNormalStemCJK(edges, axis, i, edge2Idx, delta, dim)
			// Only accumulate delta for the first stem on the HORIZONTAL axis.
			// FreeType af_cjk_hint_edges (afcjk.c:1963-1966): the return value
			// of af_cjk_hint_normal_stem is assigned to delta only when
			// `dim != AF_DIMENSION_VERT && !anchor`. For the VERTICAL axis the
			// return value is discarded, so delta stays 0 for all stems.
			if dim == dimHorizontal && anchorIdx < 0 {
				delta = stemDelta
			}
			anchorIdx = i
			edge.flags |= edgeFlagDone
			edges[edge2Idx].flags |= edgeFlagDone
			lastStemPos = edges[edge2Idx].pos
		}
	}

	return anchorIdx
}

// positionFirstStem positions the first (anchor) stem.
// For narrow stems (<1.5px = 96 in 26.6), center-align. Otherwise, round.
// All values in 26.6 fixed-point.
//
// See FreeType aflatin.c:4378-4445.
// See skrifa hint/edges.rs align_stem_edges (anchor_ix.is_none() branch).
//
//nolint:nestif // FreeType aflatin.c port — algorithmic complexity is inherent
func positionFirstStem(edge, edge2 *hintEdge, orgLen, curLen int32) {
	if curLen < 96 { // < 1.5px in 26.6
		orgCenter := edge.opos + (orgLen >> 1)
		curPos := f26dot6Round(orgCenter)

		var uOff, dOff int32
		if curLen <= 64 { // <= 1.0px
			uOff = 32 // 0.5px = 32/64
			dOff = 32
		} else {
			uOff = 38 // 38/64 = 0.59375px
			dOff = 26 // 26/64 = 0.40625px
		}

		delta1 := orgCenter - (curPos - uOff)
		if delta1 < 0 {
			delta1 = -delta1
		}
		delta2 := orgCenter - (curPos + dOff)
		if delta2 < 0 {
			delta2 = -delta2
		}

		if delta1 < delta2 {
			curPos -= uOff
		} else {
			curPos += dOff
		}

		edge.pos = curPos - curLen/2
		edge2.pos = edge.pos + curLen
	} else {
		edge.pos = f26dot6Round(edge.opos)
		edge2.pos = edge.pos + curLen
	}
}

// positionSubsequentStem positions a stem relative to the anchor.
// All values in 26.6 fixed-point.
//
// See FreeType aflatin.c:4447-4540.
// See skrifa hint/edges.rs align_stem_edges (anchor_ix.is_some() branch).
//
//nolint:nestif // FreeType aflatin.c port — algorithmic complexity is inherent
func positionSubsequentStem(edge, edge2, anchor *hintEdge, orgLen, curLen int32) {
	orgPos := anchor.pos + (edge.opos - anchor.opos)
	orgCenter := orgPos + (orgLen >> 1)

	if edge2.flags&edgeFlagDone != 0 {
		// Edge2 already done (e.g., blue zone) — adjust edge.
		edge.pos = edge2.pos - curLen
		return
	}

	if curLen < 96 { // < 1.5px in 26.6
		curPos := f26dot6Round(orgCenter)

		var uOff, dOff int32
		if curLen <= 64 { // <= 1.0px
			uOff = 32
			dOff = 32
		} else {
			uOff = 38
			dOff = 26
		}

		delta1 := orgCenter - (curPos - uOff)
		if delta1 < 0 {
			delta1 = -delta1
		}
		delta2 := orgCenter - (curPos + dOff)
		if delta2 < 0 {
			delta2 = -delta2
		}

		if delta1 < delta2 {
			curPos -= uOff
		} else {
			curPos += dOff
		}

		edge.pos = curPos - curLen/2
		edge2.pos = curPos + curLen/2
	} else {
		curPos1 := f26dot6Round(orgPos)
		delta1 := curPos1 + (curLen >> 1) - orgCenter
		if delta1 < 0 {
			delta1 = -delta1
		}

		curPos2 := f26dot6Round(orgPos+orgLen) - curLen
		delta2 := curPos2 + (curLen >> 1) - orgCenter
		if delta2 < 0 {
			delta2 = -delta2
		}

		if delta1 < delta2 {
			edge.pos = curPos1
		} else {
			edge.pos = curPos2
		}
		edge2.pos = edge.pos + curLen
	}
}

// alignLinkedEdge aligns one stem edge relative to another.
// Uses computeStemWidth (Default) or computeStemWidthCJK for the fitted width.
// All values in 26.6.
//
// See FreeType aflatin.c:4161 af_latin_align_linked_edge.
// See skrifa hint/edges.rs align_linked_edge.
func alignLinkedEdge(edges []*hintEdge, axis *scaledAxisMetrics, baseIdx, stemIdx int, group scriptGroup) {
	base := edges[baseIdx]
	stem := edges[stemIdx]

	dist := stem.opos - base.opos
	baseDelta := base.pos - base.opos
	var fittedWidth int32
	if group == scriptGroupCJK {
		fittedWidth = computeStemWidthCJK(axis, dist)
	} else {
		_ = baseDelta // Default doesn't use baseDelta in smooth hinting
		fittedWidth = computeStemWidth(axis, dist, base.flags, stem.flags)
	}
	stem.pos = base.pos + fittedWidth
}

// alignRemainingEdges handles serifs and single-segment edges.
// All positions in 26.6 fixed-point.
//
// CJK has a simpler 2-pass approach:
//  1. Align edges with serif references
//  2. Interpolate between completed bounding edges
//
// See FreeType aflatin.c:4635-4830.
// See skrifa hint/edges.rs align_remaining_edges.
//
// applyMSymmetryCJK applies FreeType's lowercase-m symmetry correction
// (af_cjk_hint_edges, afcjk.c:2078-2116): for glyphs with exactly 6 or 12
// vertical edges where the three stems are consecutive-linked edges with
// nearly equal spacing (span < 8 in 26.6 = 1/8px), the third stem is shifted
// so that it is symmetric about the second stem.
//
//	n_edges == 6:  edge1 = edges[0], edge2 = edges[2], edge3 = edges[4]
//	n_edges == 12: edge1 = edges[1], edge2 = edges[5], edge3 = edges[9]
//
// The delta is the deviation of edge3 from the symmetric position:
// delta = edge3.pos - (2*edge2.pos - edge1.pos). edge3 and its link are
// moved by -delta; for 12 edges, edges[8] and edges[11] (serifs) follow.
//
//nolint:gocognit // FreeType aflatin.c port — algorithmic complexity is inherent
func applyMSymmetryCJK(edges []*hintEdge) {
	n := len(edges)
	if n != 6 && n != 12 {
		return
	}

	var e1, e2, e3 int
	if n == 6 {
		e1, e2, e3 = 0, 2, 4
	} else {
		e1, e2, e3 = 1, 5, 9
	}

	dist1 := edges[e2].opos - edges[e1].opos
	dist2 := edges[e3].opos - edges[e2].opos
	span := dist1 - dist2
	if span < 0 {
		span = -span
	}

	if edges[e1].linkIdx != int16(e1+1) ||
		edges[e2].linkIdx != int16(e2+1) ||
		edges[e3].linkIdx != int16(e3+1) ||
		span >= 8 {
		return
	}

	delta := edges[e3].pos - (2*edges[e2].pos - edges[e1].pos)
	edges[e3].pos -= delta
	if edges[e3].linkIdx >= 0 {
		edges[edges[e3].linkIdx].pos -= delta
	}

	if n == 12 {
		edges[8].pos -= delta
		edges[11].pos -= delta
	}

	edges[e3].flags |= edgeFlagDone
	if edges[e3].linkIdx >= 0 {
		edges[edges[e3].linkIdx].flags |= edgeFlagDone
	}
}

func alignRemainingEdges(edges []*hintEdge, anchorIdx int, group scriptGroup, topToBottom bool) {
	if group != scriptGroupDefault {
		alignRemainingEdgesCJK(edges)
		return
	}

	for i, edge := range edges {
		if (edge.flags & edgeFlagDone) != 0 {
			continue
		}

		if edge.serifIdx >= 0 { //nolint:gocritic,nestif // FreeType aflatin.c port — conditional chain on edge type
			serifEdge := edges[edge.serifIdx]
			delta := serifEdge.opos - edge.opos
			if delta < 0 {
				delta = -delta
			}

			if delta < 80 { //nolint:gocritic // FreeType aflatin.c port — value range if-else chain // 1.25px = 80 in 26.6
				// Align serif: shift by same amount as base.
				edge.pos = serifEdge.pos + (edge.opos - serifEdge.opos)
			} else if anchorIdx < 0 {
				edge.pos = f26dot6Round(edge.opos)
				anchorIdx = i
			} else {
				// Interpolate between nearest done edges.
				edge.pos = interpolateEdge(edges, i, anchorIdx)
			}
		} else if anchorIdx < 0 {
			edge.pos = f26dot6Round(edge.opos)
			anchorIdx = i
		} else {
			// Single edge: interpolate.
			edge.pos = interpolateEdge(edges, i, anchorIdx)
		}

		edge.flags |= edgeFlagDone

		// Bound checks. 0.25px = 16 in 26.6. Matches skrifa edges.rs
		// adjust_link: top_to_bottom reverses both order checks (edges.rs:454).
		if i > 0 {
			orderBroken := edge.pos < edges[i-1].pos
			if topToBottom {
				orderBroken = edge.pos > edges[i-1].pos
			}
			if orderBroken {
				if edge.linkIdx >= 0 {
					linkPos := edges[edge.linkIdx].pos
					d := linkPos - edges[i-1].pos
					if d < 0 {
						d = -d
					}
					if d > 16 {
						edge.pos = edges[i-1].pos
					}
				}
			}
		}
		if i+1 < len(edges) && (edges[i+1].flags&edgeFlagDone) != 0 {
			orderBroken := edge.pos > edges[i+1].pos
			if topToBottom {
				orderBroken = edge.pos < edges[i+1].pos
			}
			if orderBroken {
				if edge.linkIdx >= 0 {
					linkPos := edges[edge.linkIdx].pos
					d := linkPos - edges[i-1].pos
					if d < 0 {
						d = -d
					}
					if d > 16 {
						edge.pos = edges[i+1].pos
					}
				}
			}
		}
	}
}

// alignRemainingEdgesCJK handles serif and single-segment edges for CJK.
// Two-pass approach matching skrifa hint/edges.rs align_remaining_edges (CJK branch):
//  1. First pass: align edges with serif references
//  2. Second pass: interpolate between completed bounding edges
//
// See skrifa hint/edges.rs:588-635 (CJK branch of align_remaining_edges).
// See FreeType afcjk.c:2119.
func alignRemainingEdgesCJK(edges []*hintEdge) {
	serifCount := 0

	// Pass 1: align edges with serif references.
	for _, edge := range edges {
		if (edge.flags & edgeFlagDone) != 0 {
			continue
		}
		if edge.serifIdx >= 0 {
			serifEdge := edges[edge.serifIdx]
			edge.pos = serifEdge.pos + (edge.opos - serifEdge.opos)
			edge.flags |= edgeFlagDone
		} else {
			serifCount++
		}
	}

	if serifCount == 0 {
		return
	}

	// Pass 2: interpolate remaining edges between completed bounding edges.
	for i, edge := range edges {
		if (edge.flags & edgeFlagDone) != 0 {
			continue
		}

		beforeIdx, afterIdx := findBoundingDoneEdges(edges, i)

		switch {
		case beforeIdx >= 0 && afterIdx < 0:
			// Only before: align to before.
			before := edges[beforeIdx]
			edge.pos = before.pos + (edge.opos - before.opos)
		case beforeIdx < 0 && afterIdx >= 0:
			// Only after: align to after.
			after := edges[afterIdx]
			edge.pos = after.pos + (edge.opos - after.opos)
		case beforeIdx >= 0 && afterIdx >= 0:
			// Both: interpolate.
			before := edges[beforeIdx]
			after := edges[afterIdx]
			if after.fpos == before.fpos {
				edge.pos = before.pos
			} else {
				edge.pos = before.pos + fixedMulDiv26dot6(
					int32(edge.fpos)-int32(before.fpos),
					after.pos-before.pos,
					int32(after.fpos)-int32(before.fpos),
				)
			}
		}
	}
}

// findBoundingDoneEdges returns indices of the nearest "done" edges
// before and after the given index. Returns -1 if not found.
func findBoundingDoneEdges(edges []*hintEdge, idx int) (int, int) {
	beforeIdx := -1
	for j := idx - 1; j >= 0; j-- {
		if (edges[j].flags & edgeFlagDone) != 0 {
			beforeIdx = j
			break
		}
	}
	afterIdx := -1
	for j := idx + 1; j < len(edges); j++ {
		if (edges[j].flags & edgeFlagDone) != 0 {
			afterIdx = j
			break
		}
	}
	return beforeIdx, afterIdx
}

// hintNormalStemCJK adjusts both edges of a CJK stem and returns the delta.
// This is the CJK-specific stem placement algorithm that grid-aligns stems
// while preserving the center position.
//
// See FreeType afcjk.c:1678 af_cjk_hint_normal_stem.
// See skrifa hint/edges.rs:947-1050 hint_normal_stem_cjk.
//
//nolint:gocognit,gocyclo,cyclop,nestif // FreeType afcjk.c port — algorithmic complexity is inherent
func hintNormalStemCJK(edges []*hintEdge, axis *scaledAxisMetrics, edgeIdx, edge2Idx int, anchor int32, dim hintDimension) int32 {
	edge := edges[edgeIdx]
	edge2 := edges[edge2Idx]

	// do_stem_adjust is off in FT light mode (afcjk.c:1419-1422): the stem
	// width is NOT quantized and the delta is clamped to
	// AF_LIGHT_MODE_MAX_DELTA_ABS (14/64 px). The stem-width threshold also
	// changes (afcjk.c:1661-1671): MAX_HORZ_GAP=9 for the vertical dimension,
	// MAX_VERT_GAP=15 for the horizontal dimension.
	doStemAdjust := axis.doStemAdjust
	var thresholdDelta int32
	if !doStemAdjust {
		maxGap := int32(15) // AF_LIGHT_MODE_MAX_VERT_GAP (horizontal dimension)
		if dim == dimVertical {
			maxGap = 9 // AF_LIGHT_MODE_MAX_HORZ_GAP (vertical dimension)
		}
		if (edge.flags&edgeFlagRound) != 0 && (edge2.flags&edgeFlagRound) != 0 {
			thresholdDelta = maxGap
		} else {
			thresholdDelta = maxGap / 3
		}
	}
	threshold := int32(64) - thresholdDelta

	originalLen := edge2.opos - edge.opos
	var curLen int32
	if !doStemAdjust {
		curLen = originalLen // FT light: af_cjk_compute_stem_width returns width unchanged
	} else {
		curLen = computeStemWidthCJK(axis, originalLen)
	}

	originalCenter := (edge.opos+edge2.opos)/2 + anchor
	curPos1 := originalCenter - curLen/2
	curPos2 := curPos1 + curLen

	const maxDeltaAbs = 14

	finish := func(delta int32) int32 {
		if !doStemAdjust {
			if delta > maxDeltaAbs {
				delta = maxDeltaAbs
			} else if delta < -maxDeltaAbs {
				delta = -maxDeltaAbs
			}
		}
		adjustment := curPos1 + delta
		if edge.opos < edge2.opos {
			edges[edgeIdx].pos = adjustment
			edges[edge2Idx].pos = adjustment + curLen
		} else {
			edges[edge2Idx].pos = adjustment
			edges[edgeIdx].pos = adjustment + curLen
		}
		return delta
	}

	dOff1 := curPos1 - f26dot6Floor(curPos1)
	dOff2 := curPos2 - f26dot6Floor(curPos2)
	var delta int32

	if dOff1 == 0 || dOff2 == 0 {
		return finish(delta)
	}

	uOff1 := int32(64) - dOff1
	uOff2 := int32(64) - dOff2

	if curLen <= threshold {
		if dOff2 < curLen {
			if uOff1 <= dOff2 {
				delta = uOff1
			} else {
				delta = -dOff2
			}
		}
		return finish(delta)
	}

	if threshold < 64 &&
		(dOff1 >= threshold || uOff1 >= threshold || dOff2 >= threshold || uOff2 >= threshold) {
		return finish(delta)
	}

	offset := curLen & 63
	if offset < 32 {
		if uOff1 <= offset || dOff2 <= offset {
			return finish(delta)
		}
	} else {
		offset = 64 - threshold
	}

	dOff1 = threshold - uOff1
	uOff1 -= offset
	uOff2 = threshold - dOff2
	dOff2 -= offset

	if dOff1 <= uOff1 {
		uOff1 = -dOff1
	}
	if dOff2 <= uOff2 {
		uOff2 = -dOff2
	}

	if abs32(uOff1) <= abs32(uOff2) {
		delta = uOff1
	} else {
		delta = uOff2
	}

	return finish(delta)
}

// abs32 returns the absolute value of an int32.
func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

// fixedMulDiv26dot6 computes a * b / c with rounding (not truncation).
// Matches skrifa Fixed::mul_div (font-types/src/fixed.rs:155):
// uses absolute values, adds half-divisor for rounding, applies sign.
func fixedMulDiv26dot6(a, b, c int32) int32 {
	if c == 0 {
		return 0
	}
	sign := int32(1)
	au := int64(a)
	if a < 0 {
		au = -au
		sign = -sign
	}
	bu := int64(b)
	if b < 0 {
		bu = -bu
		sign = -sign
	}
	cu := int64(c)
	if c < 0 {
		cu = -cu
		sign = -sign
	}
	result := int32((au*bu + cu/2) / cu)
	if sign < 0 {
		return -result
	}
	return result
}

// interpolateEdge interpolates an edge's position between two neighboring
// "done" edges, or shifts it relative to the anchor. All values in 26.6.
func interpolateEdge(edges []*hintEdge, edgeIdx, anchorIdx int) int32 {
	edge := edges[edgeIdx]

	// Find nearest done edge before and after.
	beforeIdx := -1
	afterIdx := -1

	for j := edgeIdx - 1; j >= 0; j-- {
		if (edges[j].flags & edgeFlagDone) != 0 {
			beforeIdx = j
			break
		}
	}
	for j := edgeIdx + 1; j < len(edges); j++ {
		if (edges[j].flags & edgeFlagDone) != 0 {
			afterIdx = j
			break
		}
	}

	if beforeIdx >= 0 && afterIdx >= 0 {
		before := edges[beforeIdx]
		after := edges[afterIdx]
		denom := after.opos - before.opos
		if denom != 0 {
			// Fixed-point interpolation: before.pos + (edge.opos - before.opos) * (after.pos - before.pos) / denom
			num := int64(edge.opos-before.opos) * int64(after.pos-before.pos)
			return before.pos + int32(num/int64(denom))
		}
		return before.pos
	}

	// Fall back to anchor-relative positioning.
	// FreeType SERIF_LINK2 (aflatin.c:3477-3479): the offset is snapped to a
	// quarter-pixel grid, not rounded to whole pixels:
	//   edge->pos = anchor->pos + ( ( edge->opos - anchor->opos + 16 ) & ~31 );
	anchor := edges[anchorIdx]
	return anchor.pos + int32((edge.opos-anchor.opos+16)&^31)
}

// Note: old float32 fixedDiv/fixedMul removed — the pipeline now uses
// integer 26.6 arithmetic via fixedMul26dot6/fixedDiv26dot6 in autohint.go.
