package h264

// Deblocking filter (spec 8.7 facts: alpha/beta/tc0 tables).
// VR2a pictures are all-intra: boundary strength is 4 on macroblock
// edges and 3 on internal 4x4 edges. Inter strengths arrive with VR2b.

var deblockAlpha = [52]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	4, 4, 5, 6, 7, 8, 9, 10, 12, 13, 15, 17, 20, 22,
	25, 28, 32, 36, 40, 45, 50, 56, 63, 71,
	80, 90, 101, 113, 127, 144, 162, 182, 203, 226, 255, 255,
}

var deblockBeta = [52]uint8{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	2, 2, 2, 3, 3, 3, 3, 4, 4, 4, 6, 6, 7, 7,
	8, 8, 9, 9, 10, 10, 11, 11, 12, 12,
	13, 13, 14, 14, 15, 15, 16, 16, 17, 17, 18, 18,
}

var deblockTC0 = [52][4]int{
	{-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0},
	{-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0},
	{-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0},
	{-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0}, {-1, 0, 0, 0},
	{-1, 0, 0, 0}, {-1, 0, 0, 1}, {-1, 0, 0, 1}, {-1, 0, 0, 1},
	{-1, 0, 0, 1}, {-1, 0, 1, 1}, {-1, 0, 1, 1}, {-1, 1, 1, 1},
	{-1, 1, 1, 1}, {-1, 1, 1, 1}, {-1, 1, 1, 1}, {-1, 1, 1, 2},
	{-1, 1, 1, 2}, {-1, 1, 1, 2}, {-1, 1, 1, 2}, {-1, 1, 2, 3},
	{-1, 1, 2, 3}, {-1, 2, 2, 3}, {-1, 2, 2, 4}, {-1, 2, 3, 4},
	{-1, 2, 3, 4}, {-1, 3, 3, 5}, {-1, 3, 4, 6}, {-1, 3, 4, 6},
	{-1, 4, 5, 7}, {-1, 4, 5, 8}, {-1, 4, 6, 9}, {-1, 5, 7, 10},
	{-1, 6, 8, 11}, {-1, 6, 8, 13}, {-1, 7, 10, 14}, {-1, 8, 11, 16},
	{-1, 9, 12, 18}, {-1, 10, 13, 20}, {-1, 11, 15, 23}, {-1, 13, 17, 25},
}

func clipIdx(v, lo, hi int32) int {
	if v < lo {
		return int(lo)
	}
	if v > hi {
		return int(hi)
	}
	return int(v)
}

func filterAlphaBeta(qp, aOff, bOff int32) (alpha, beta int) {
	ia := clipIdx(qp+aOff, 0, 51)
	ib := clipIdx(qp+bOff, 0, 51)
	alpha = int(deblockAlpha[ia])
	beta = int(deblockBeta[ib])
	if qp+aOff < 0 {
		alpha = 0
	}
	if qp+aOff > 51 {
		alpha = 255
	}
	if qp+bOff < 0 {
		beta = 0
	}
	if qp+bOff > 51 {
		beta = 18
	}
	return alpha, beta
}

func filterTC(qp, aOff int32, bS int) int {
	if bS <= 0 {
		return -1
	}
	if bS > 3 {
		bS = 3
	}
	ia := clipIdx(qp+aOff, 0, 51)
	return deblockTC0[ia][bS]
}

// filterLumaEdge filters 4 lines across a luma edge. For a vertical edge
// (vertical=false) the edge runs down at x=ex starting at y=ey; for a
// horizontal edge it runs right at y=ey starting at x=ex.
func filterLumaEdge(p []uint8, stride int, ex, ey int, vertical bool, bS int, alpha, beta, tc int) {
	if bS <= 0 {
		return
	}
	get := func(x, y int) int { return int(p[y*stride+x]) }
	put := func(x, y, v int) {
		if v < 0 {
			v = 0
		}
		if v > 255 {
			v = 255
		}
		p[y*stride+x] = uint8(v)
	}
	for line := 0; line < 4; line++ {
		var px, py [8]int
		for k := -4; k < 4; k++ {
			if !vertical {
				px[k+4], py[k+4] = ex+k, ey+line
			} else {
				px[k+4], py[k+4] = ex+line, ey+k
			}
		}
		p3, p2, p1, p0 := get(px[0], py[0]), get(px[1], py[1]), get(px[2], py[2]), get(px[3], py[3])
		q0, q1, q2, q3 := get(px[4], py[4]), get(px[5], py[5]), get(px[6], py[6]), get(px[7], py[7])
		if abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta {
			continue
		}
		if bS < 4 {
			tcLine := tc
			ap := abs(p2-p0) < beta
			aq := abs(q2-q0) < beta
			if ap {
				tcLine++
			}
			if aq {
				tcLine++
			}
			delta := clip3((((q0-p0)<<2)+(p1-q1)+4)>>3, -tcLine, tcLine)
			put(px[3], py[3], p0+delta)
			put(px[4], py[4], q0-delta)
			if ap {
				put(px[2], py[2], p1+clip3((p2+((p0+q0+1)>>1)-2*p1)>>1, -tc, tc))
			}
			if aq {
				put(px[5], py[5], q1+clip3((q2+((p0+q0+1)>>1)-2*q1)>>1, -tc, tc))
			}
			continue
		}
		ap := abs(p2-p0) < beta
		aq := abs(q2-q0) < beta
		if abs(p0-q0) < (alpha>>2)+2 {
			if ap {
				put(px[3], py[3], (p2+2*p1+2*p0+2*q0+q1+4)>>3)
				put(px[2], py[2], (p2+p1+p0+q0+2)>>2)
				put(px[1], py[1], (2*p3+3*p2+p1+p0+q0+4)>>3)
			} else {
				put(px[3], py[3], (2*p1+p0+q1+2)>>2)
			}
			if aq {
				put(px[4], py[4], (p1+2*p0+2*q0+2*q1+q2+4)>>3)
				put(px[5], py[5], (p0+q0+q1+q2+2)>>2)
				put(px[6], py[6], (2*q3+3*q2+q1+q0+p0+4)>>3)
			} else {
				put(px[4], py[4], (2*q1+q0+p1+2)>>2)
			}
		} else {
			put(px[3], py[3], (2*p1+p0+q1+2)>>2)
			put(px[4], py[4], (2*q1+q0+p1+2)>>2)
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clip3(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// filterChromaEdge filters 2 lines across a chroma edge (bS>=1; the
// caller sends bS 4 to the intra path instead).
func filterChromaEdge(p []uint8, stride int, ex, ey int, vertical bool, bS int, alpha, beta, tc int) {
	if bS < 1 {
		return
	}
	tc++
	for line := 0; line < 2; line++ {
		var p1, p0, q0, q1 int
		if !vertical {
			p1, p0 = int(p[(ey+line)*stride+ex-2]), int(p[(ey+line)*stride+ex-1])
			q0, q1 = int(p[(ey+line)*stride+ex]), int(p[(ey+line)*stride+ex+1])
		} else {
			p1, p0 = int(p[(ey-2)*stride+ex+line]), int(p[(ey-1)*stride+ex+line])
			q0, q1 = int(p[(ey)*stride+ex+line]), int(p[(ey+1)*stride+ex+line])
		}
		if abs(p0-q0) >= alpha || abs(p1-p0) >= beta || abs(q1-q0) >= beta {
			continue
		}
		delta := clip3((((q0-p0)<<2)+(p1-q1)+4)>>3, -tc, tc)
		if !vertical {
			p[(ey+line)*stride+ex-1] = uint8(clip3(p0+delta, 0, 255))
			p[(ey+line)*stride+ex] = uint8(clip3(q0-delta, 0, 255))
		} else {
			p[(ey-1)*stride+ex+line] = uint8(clip3(p0+delta, 0, 255))
			p[(ey)*stride+ex+line] = uint8(clip3(q0-delta, 0, 255))
		}
	}
}

// filterChromaIntraEdge filters 2 lines across a chroma MB edge (bS 4):
// fixed taps without tc clipping.
func filterChromaIntraEdge(p []uint8, stride int, ex, ey int, vertical bool, alpha, beta int) {
	if vertical {
		filterChromaIntraEdgeV(p, stride, ex, ey, alpha, beta)
		return
	}
	filterChromaIntraEdgeH(p, stride, ex, ey, alpha, beta)
}

// S1b-CIE direction-hoisted intra edge (bit-identical pixels): the old
// loop branched on vertical per line (2x), called abs 3x and clip3 2x
// per line. vertical is edge-constant, so each direction gets its own
// unrolled body: abs inlined (no call), and the final clip3 is dropped
// (no-op: (2*255+255+255+2)>>2==255, (0+0+0+2)>>2==0, so the value
// never leaves 0..255). Same taps, same order, same skips.
func filterChromaIntraEdgeH(p []uint8, stride int, ex, ey int, alpha, beta int) {
	base0 := ey*stride + ex
	p1 := int(p[base0-2])
	p0 := int(p[base0-1])
	q0 := int(p[base0])
	q1 := int(p[base0+1])
	if d := p0 - q0; d < 0 {
		if -d >= alpha {
			goto line1
		}
	} else if d >= alpha {
		goto line1
	}
	if d := p1 - p0; d < 0 {
		if -d >= beta {
			goto line1
		}
	} else if d >= beta {
		goto line1
	}
	if d := q1 - q0; d < 0 {
		if -d < beta {
			p[base0-1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
			p[base0] = uint8((2*q1 + q0 + p1 + 2) >> 2)
		}
	} else if d < beta {
		p[base0-1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
		p[base0] = uint8((2*q1 + q0 + p1 + 2) >> 2)
	}
line1:
	base1 := base0 + stride
	p1 = int(p[base1-2])
	p0 = int(p[base1-1])
	q0 = int(p[base1])
	q1 = int(p[base1+1])
	if d := p0 - q0; d < 0 {
		if -d >= alpha {
			return
		}
	} else if d >= alpha {
		return
	}
	if d := p1 - p0; d < 0 {
		if -d >= beta {
			return
		}
	} else if d >= beta {
		return
	}
	if d := q1 - q0; d < 0 {
		if -d < beta {
			p[base1-1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
			p[base1] = uint8((2*q1 + q0 + p1 + 2) >> 2)
		}
	} else if d < beta {
		p[base1-1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
		p[base1] = uint8((2*q1 + q0 + p1 + 2) >> 2)
	}
}

// S1b-CIE vertical twin: columns ex/ex+1, rows ey-2..ey+1, same
// inlining and no-op-clip removal as the horizontal body above.
func filterChromaIntraEdgeV(p []uint8, stride int, ex, ey int, alpha, beta int) {
	r0 := (ey-2)*stride + ex
	r1 := r0 + stride
	r2 := r1 + stride
	r3 := r2 + stride
	p1 := int(p[r0])
	p0 := int(p[r1])
	q0 := int(p[r2])
	q1 := int(p[r3])
	if d := p0 - q0; d < 0 {
		if -d >= alpha {
			goto col1
		}
	} else if d >= alpha {
		goto col1
	}
	if d := p1 - p0; d < 0 {
		if -d >= beta {
			goto col1
		}
	} else if d >= beta {
		goto col1
	}
	if d := q1 - q0; d < 0 {
		if -d < beta {
			p[r1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
			p[r2] = uint8((2*q1 + q0 + p1 + 2) >> 2)
		}
	} else if d < beta {
		p[r1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
		p[r2] = uint8((2*q1 + q0 + p1 + 2) >> 2)
	}
col1:
	r0++
	r1++
	r2++
	r3++
	p1 = int(p[r0])
	p0 = int(p[r1])
	q0 = int(p[r2])
	q1 = int(p[r3])
	if d := p0 - q0; d < 0 {
		if -d >= alpha {
			return
		}
	} else if d >= alpha {
		return
	}
	if d := p1 - p0; d < 0 {
		if -d >= beta {
			return
		}
	} else if d >= beta {
		return
	}
	if d := q1 - q0; d < 0 {
		if -d < beta {
			p[r1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
			p[r2] = uint8((2*q1 + q0 + p1 + 2) >> 2)
		}
	} else if d < beta {
		p[r1] = uint8((2*p1 + p0 + q1 + 2) >> 2)
		p[r2] = uint8((2*q1 + q0 + p1 + 2) >> 2)
	}
}

// DeblockPicture filters all 4x4 edges of a decoded picture.
// qps/fIDC/fA/fB carry per-macroblock QP and filter switches.
// mbIntra/nnzY/mv/ref carry inter info for boundary strength; nil mbIntra
// keeps the VR2a all-intra strengths (4 on MB edges, 3 inside).
// refList resolves reference indices to pictures: aliased indices into
// the same picture count as one reference (no strength from the index
// alone). A nil list keeps plain index comparison.
func DeblockPicture(pic *Picture, qps []int32, fIDC []uint32, fA, fB []int32, mbW, mbH int, cOff0, cOff1 int32, mbIntra []bool, nnzY []int8, mvX, mvY []int16, refIdx []int8, refList []*Picture, mbT8 []bool, mvX1, mvY1 []int16, refIdx1 []int8, refList1 []*Picture, useM []uint8, uniSkip []bool, uniDM [][2]bDirectMV) {
	if pic == nil {
		return
	}
	W := int(pic.Width)
	// S1b-AD qpAt inlined at its 3 call sites below (closure call +
	// 4 clamp branches each, ~30ms flat per 90f profile): qC is always
	// in range (addr itself); qL/qT clamp only on the first column/row
	// (then they equal qC, same as the closure). No closure, two
	// predictable branches per macroblock.
	// Per-macroblock order like the reference: vertical edges (left
	// then internal) followed by horizontal edges (top then internal),
	// in MB raster order. Edges share one picture, so order matters.
	for mby := 0; mby < mbH; mby++ {
		for mbx := 0; mbx < mbW; mbx++ {
			addr := mby*mbW + mbx
			is8 := addr >= 0 && addr < len(mbT8) && mbT8[addr]
			// idc==1 skips every edge of this MB (same verdict as the
			// per-edge gate below, which has no side effects): skip
			// before computing any thresholds.
			if fIDC[addr] == 1 {
				continue
			}
			// S1b-AD lazy thresholds: the old code evaluated all 3
			// cases (C/L/T) x (luma + chroma Cb/Cr) per macroblock up
			// front (~19 table lookups: filterAlphaBeta + ChromaQP,
			// ~110ms flat per 90f profile), but near-half the edges
			// are all-zero strength and skip before any threshold is
			// read (S1-Z census: 47% on 1080p), plus closed-gate and
			// uniform-skip edges. So only q's (plain loads) and fa/fb
			// stay hoisted; every filterAlphaBeta/ChromaQP runs on
			// demand after the bS verdict, at most one case per edge
			// (e==0 takes L/T, internal edges share one cached C).
			// Same values, same order of reads — pure scheduling.
			qC := qps[addr]
			qL := qC
			if mbx > 0 {
				qL = qps[addr-1]
			}
			qT := qC
			if mby > 0 {
				qT = qps[addr-mbW]
			}
			fa := fA[mby*mbW+mbx]
			fb := fB[mby*mbW+mbx]
			var alphaC, betaC int
			var haveC bool
			var qpcCbC, qpcCrC int32
			var alphaCbC, betaCbC, alphaCrC, betaCrC int
			var haveChromaC bool
			// NOTE: plain index loop, not `range []bool{...}` — the
			// slice literal allocates per macroblock (5k+ allocs per
			// 1536x864 frame straight into GC).
			for vi := 0; vi < 2; vi++ {
				vertical := vi == 1
				for e := 0; e < 4; e++ {
					// 8x8 transform skips internal 4x4 edges (1 and 3);
					// MB edge (0) and 8x8 boundary (2) still filter.
					if is8 && (e == 1 || e == 3) {
						continue
					}
					var ex, ey int
					var edgeMB bool
					if !vertical {
						ex = mbx*16 + e*4
						ey = mby * 16
						edgeMB = e == 0
					} else {
						ex = mbx * 16
						ey = mby*16 + e*4
						edgeMB = e == 0
					}
					if edgeMB && (mbx == 0 && !vertical || mby == 0 && vertical) {
						continue
					}
					idc := fIDC[mby*mbW+mbx]
					if idc == 1 || idc == 2 && !edgeMB {
						continue
					}
					var qx, qy int
					if !vertical {
						qx, qy = mbx, mby
						if e == 0 {
							qx = mbx - 1
						}
					} else {
						qx, qy = mbx, mby
						if e == 0 {
							qy = mby - 1
						}
					}
					// S1b-AD: luma thresholds resolve below, after the
					// bS verdict (zero edges never pay for them).
					var qp int32
					var alpha, beta int
					// Interior edges skip per-segment bounds checks (same
					// results, no branches). Border edges keep the safe path.
					inFast := edgeInterior(ex, ey, vertical, mbW, mbH) &&
						mbIntra != nil && nnzY != nil && mvX != nil && mvY != nil &&
						refIdx != nil && mbT8 != nil &&
						(useM == nil || (mvX1 != nil && mvY1 != nil && refIdx1 != nil))
					// S1b-E uniform-edge shortcut: both sides decoded as
					// skip-uniform this frame (uniSkip, recorded at motion
					// store time with cbp==0 so nnz is zero by
					// construction) with identical motion — every segment
					// verdict is 0 (non-intra, no coefficients, equal refs
					// and zero motion difference on both lists, cross-list
					// mirror included), so skip verdicts, filtering and
					// chroma (bSedge stays 0) together. Anything else
					// keeps the per-segment path with identical output.
					// Internal edges (e>0) sit inside one macroblock:
					// that block alone decides. Nil arrays (tests) stay
					// on the slow path.
					if inFast && uniSkip != nil && uniDM != nil {
						pMB, qMB := mby*mbW+mbx, qy*mbW+qx
						hit := false
						if e == 0 {
							hit = pMB >= 0 && qMB >= 0 && pMB < len(uniSkip) && qMB < len(uniSkip) &&
								pMB < len(uniDM) && qMB < len(uniDM) &&
								uniSkip[pMB] && uniSkip[qMB] && uniDM[pMB] == uniDM[qMB]
						} else if pMB >= 0 && pMB < len(uniSkip) && uniSkip[pMB] {
							hit = true
						}
						if hit {
							continue
						}
					}
					// Whole-edge batch (ffmpeg filter_mb_edgev pattern:
					// bS[4] for the edge, one gate, then filter): bS feeds
					// luma and chroma, so it is always computed; the gate
					// below skips tc + kernel setup for all 4 segments at
					// once. Closed gates write nothing (every filter path
					// gates each line on alpha/beta first), so skipping the
					// whole edge equals skipping per segment, bit for bit.
					// Pure Go so amd64/arm64/fallback share one path.
					var bSedge [4]int
					var segX, segY [4]int
					for seg := 0; seg < 4; seg++ {
						if !vertical {
							segX[seg], segY[seg] = ex, ey+seg*4
						} else {
							segX[seg], segY[seg] = ex+seg*4, ey
						}
					}
					if inFast {
						// S1-D edge batch: one call verdicts all 4
						// segments (divisions + MB-intra verdict hoisted;
						// identical outputs, see interBSEdge).
						interBSEdge(&bSedge, mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, ex, ey, vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
					} else {
						for seg := 0; seg < 4; seg++ {
							bSedge[seg] = interBS(mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, segX[seg], segY[seg], vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
						}
					}
					// S1-Z zero-edge skip: all four strengths zero means no
					// filter on the edge writes anything, luma or chroma.
					// deblockLumaEdge16 reports true untouched for zero
					// edges (see its doc), the per-segment luma path skips
					// bS==0 segments, the chroma batched entry reports true
					// untouched for all-zero bS (see its doc), and the
					// per-segment chroma path skips bS<1 segments, so the
					// tc loops, kernel setups and both filter calls below
					// are dead work. Census 2026-09-22: zero edges are 47%
					// of counted luma edges on 1080p (57% on 2K), all with
					// open gates (gated=0%), i.e. half the edges pay full
					// dispatch for zero output. The stats note replicates
					// the original call site exactly (it only fires when
					// the luma gate below is open; closed-gate zero edges
					// were never counted). Zero-alloc: one OR-chain, one
					// predictable branch (skip areas cluster spatially).
					if bSedge[0]|bSedge[1]|bSedge[2]|bSedge[3] == 0 {
						// S1b-AD: zero edges skip threshold work in
						// production; the census (test-only,
						// deblockStatsOn) resolves the same case the
						// eager code read, so its counts stay exact.
						if deblockStatsOn {
							if e == 0 {
								if vertical {
									qp = (qC + qT + 1) >> 1
								} else {
									qp = (qC + qL + 1) >> 1
								}
								alpha, beta = filterAlphaBeta(qp, fa, fb)
							} else {
								if !haveC {
									alphaC, betaC = filterAlphaBeta(qC, fa, fb)
									haveC = true
								}
								qp, alpha, beta = qC, alphaC, betaC
							}
							if alpha != 0 && beta != 0 {
								edge16Note(edge16Zero, true)
							}
						}
						continue
					}
					// S1b-AD lazy resolve: same per-case values the
					// hoisted tables held (e==0 takes the L/T neighbour
					// average, internal edges share one cached C).
					if e == 0 {
						if vertical {
							qp = (qC + qT + 1) >> 1
						} else {
							qp = (qC + qL + 1) >> 1
						}
						alpha, beta = filterAlphaBeta(qp, fa, fb)
					} else {
						if !haveC {
							alphaC, betaC = filterAlphaBeta(qC, fa, fb)
							haveC = true
						}
						qp, alpha, beta = qC, alphaC, betaC
					}
					if alpha != 0 && beta != 0 {
						var tcEdge [4]int
						for seg := 0; seg < 4; seg++ {
							tcEdge[seg] = filterTC(qp, fa, bSedge[seg])
						}
						// S1-E16 whole-edge call (ffmpeg filter_mb_edgev
						// shape): one asm call filters all 16 lines;
						// uniform weak edges run the per-segment-tc kernel
						// and uniform strong (all-4, e.g. intra) runs the
						// strong kernel. Mixed, scalar-forced and
						// non-amd64 edges report false and run the
						// per-segment path below with identical output;
						// chroma stays scalar per segment (2 lines are too
						// narrow to win back setup).
						taken := deblockLumaEdge16(pic.Y, W, ex, ey, vertical, bSedge, tcEdge, alpha, beta)
						if deblockStatsOn {
							edge16Note(classifyEdge16(bSedge, alpha, beta), taken)
						}
						if !taken {
							for seg := 0; seg < 4; seg++ {
								bS := bSedge[seg]
								if bS == 0 {
									continue
								}
								tc := tcEdge[seg]
								// S1 dispatch: the arch kernel filters
								// directly in the picture.
								if !deblockLumaFast(pic.Y, W, segX[seg], segY[seg], vertical, bS, alpha, beta, tc) {
									filterLumaEdge(pic.Y, W, segX[seg], segY[seg], vertical, bS, alpha, beta, tc)
								}
							}
						}
					}
					// Chroma edges sit on even luma edges only (4:2:0: every
					// second one), with thresholds from averaged chroma QP.
					// Cb and Cr average separately: the second plane has its
					// own offset.
					if e%2 == 0 {
						// S1b-AD lazy resolve, chroma (same per-case
						// values the hoisted tables held; e==2 shares
						// one cached C). Reached only with non-zero bS
						// (zero edges continued above), so at most one
						// case's ChromaQP + AlphaBeta runs per edge.
						var qpcCb, qpcCr int32
						var alphaCb, betaCb, alphaCr, betaCr int
						if e == 0 {
							if vertical {
								qpcCb = (ChromaQP(qC, cOff0) + ChromaQP(qT, cOff0) + 1) >> 1
								qpcCr = (ChromaQP(qC, cOff1) + ChromaQP(qT, cOff1) + 1) >> 1
							} else {
								qpcCb = (ChromaQP(qC, cOff0) + ChromaQP(qL, cOff0) + 1) >> 1
								qpcCr = (ChromaQP(qC, cOff1) + ChromaQP(qL, cOff1) + 1) >> 1
							}
							alphaCb, betaCb = filterAlphaBeta(qpcCb, fa, fb)
							alphaCr, betaCr = filterAlphaBeta(qpcCr, fa, fb)
						} else {
							if !haveChromaC {
								qpcCbC = ChromaQP(qC, cOff0)
								qpcCrC = ChromaQP(qC, cOff1)
								alphaCbC, betaCbC = filterAlphaBeta(qpcCbC, fa, fb)
								alphaCrC, betaCrC = filterAlphaBeta(qpcCrC, fa, fb)
								haveChromaC = true
							}
							qpcCb, qpcCr = qpcCbC, qpcCrC
							alphaCb, betaCb = alphaCbC, betaCbC
							alphaCr, betaCr = alphaCrC, betaCrC
						}
						// Whole-edge chroma gate (same pattern as luma): both
						// planes gate every line on alpha/beta first, so two
						// closed gates mean no writes on any segment.
						chromaOpen := (alphaCb != 0 && betaCb != 0) || (alphaCr != 0 && betaCr != 0)
						// S1-D chroma whole-edge call: one asm call per
						// plane filters all 8 lines with per-segment tc.
						// Weak edges only (no bS==4: the intra formula
						// differs); intra, mixed, scalar-forced and
						// non-amd64 edges run the per-segment path below
						// with identical output. The picture-border skip
						// (e==0 on the outer row/column) is edge-wide:
						// every segment shares mbx/mby.
						batched := false
						if chromaOpen {
							weakOnly := true
							for seg := 0; seg < 4; seg++ {
								if bSedge[seg] == 4 {
									weakOnly = false
									break
								}
							}
							border := e == 0 && ((!vertical && mbx == 0) || (vertical && mby == 0))
							if weakOnly && !border {
								var tcCb, tcCr [4]int
								for seg := 0; seg < 4; seg++ {
									if bSedge[seg] < 1 {
										tcCb[seg], tcCr[seg] = -1, -1
									} else {
										tcCb[seg] = filterTC(qpcCb, fa, bSedge[seg]) + 1
										tcCr[seg] = filterTC(qpcCr, fa, bSedge[seg]) + 1
									}
								}
								var cx, cy int
								if !vertical {
									cx, cy = mbx*8+e*2, mby*8
								} else {
									cx, cy = mbx*8, mby*8+e*2
								}
								cbOK := alphaCb == 0 || betaCb == 0 ||
									chromaDebEdge16(pic.Cb, W/2, cx, cy, vertical, bSedge, tcCb, alphaCb, betaCb)
								crOK := alphaCr == 0 || betaCr == 0 ||
									chromaDebEdge16(pic.Cr, W/2, cx, cy, vertical, bSedge, tcCr, alphaCr, betaCr)
								batched = cbOK && crOK
							}
						}
						if batched {
							continue
						}
						for seg := 0; seg < 4 && chromaOpen; seg++ {
							// Chroma strength reuses the luma edge result:
							// identical derivation inputs, so no second
							// interBS call is needed.
							bS := bSedge[seg]
							if bS < 1 {
								continue
							}
							tcCb := filterTC(qpcCb, fa, bS)
							tcCr := filterTC(qpcCr, fa, bS)
							var cx, cy int
							if !vertical {
								cx, cy = mbx*8+e*2, mby*8+seg*2
								if e == 0 && mbx == 0 {
									continue
								}
							} else {
								cx, cy = mbx*8+seg*2, mby*8+e*2
								if e == 0 && mby == 0 {
									continue
								}
							}
							if bS == 4 {
								// S1b-CIE: direction hoisted once for
								// both planes (was one branch per
								// line per plane inside the kernel).
								if vertical {
									filterChromaIntraEdgeV(pic.Cb, W/2, cx, cy, alphaCb, betaCb)
									filterChromaIntraEdgeV(pic.Cr, W/2, cx, cy, alphaCr, betaCr)
								} else {
									filterChromaIntraEdgeH(pic.Cb, W/2, cx, cy, alphaCb, betaCb)
									filterChromaIntraEdgeH(pic.Cr, W/2, cx, cy, alphaCr, betaCr)
								}
								continue
							}
							filterChromaEdge(pic.Cb, W/2, cx, cy, vertical, bS, alphaCb, betaCb, tcCb)
							filterChromaEdge(pic.Cr, W/2, cx, cy, vertical, bS, alphaCr, betaCr, tcCr)
						}
					}
				}
			}
		}
	}
}

// t8count reads deblock presence for one 4x4 slot: the whole-8x8 sum
// from the first slot when its macroblock uses the 8x8 transform,
// the slot's own count otherwise. Macroblock origins sit on even 4x4
// columns, so clearing the low bit stays inside the block.
func t8count(nnzY []int8, stride int, mbT8 []bool, mbW, mbH, bx, by, mb int) int {
	i := by*stride + bx
	if mb < 0 || mb >= mbW*mbH || mb >= len(mbT8) || !mbT8[mb] {
		if i < 0 || i >= len(nnzY) {
			return 0
		}
		return int(nnzY[i])
	}
	si := (by&^1)*stride + (bx &^ 1)
	if si < 0 || si >= len(nnzY) {
		return 0
	}
	return int(nnzY[si])
}

// interBS derives one 4-line edge strength. sx,sy is the edge origin in
// luma pixels; vertical=false means a vertical edge at x=sx.
// Same-picture references through different indices compare equal;
// without a list the raw indices compare (single-reference clips).
// The trailing B parameters (second-list motion, both lists, usage mask)
// switch on two-list comparison; nil keeps the single-list P behaviour.
func interBS(mbIntra []bool, nnzY []int8, mvX, mvY []int16, refIdx []int8, refList []*Picture, mbW, mbH int, sx, sy int, vertical bool, edgeMB bool, mbT8 []bool, mvX1, mvY1 []int16, refIdx1 []int8, refList1 []*Picture, useM []uint8) int {
	if mbIntra == nil {
		if edgeMB {
			return 4
		}
		return 3
	}
	var pBX, pBY, qBX, qBY int
	if !vertical {
		pBX, pBY = (sx-1)/4, sy/4
		qBX, qBY = sx/4, sy/4
	} else {
		pBX, pBY = sx/4, (sy-1)/4
		qBX, qBY = sx/4, sy/4
	}
	stride := mbW * 4
	if pBX < 0 || pBY < 0 || qBX < 0 || qBY < 0 || pBX >= stride || qBX >= stride {
		if edgeMB {
			return 4
		}
		return 3
	}
	pMB := (pBY/4)*mbW + pBX/4
	qMB := (qBY/4)*mbW + qBX/4
	if pMB < 0 || qMB < 0 || pMB >= mbW*mbH || qMB >= mbW*mbH {
		if edgeMB {
			return 4
		}
		return 3
	}
	if mbIntra[pMB] || mbIntra[qMB] {
		if edgeMB {
			return 4
		}
		return 3
	}
	pi, qi := pBY*stride+pBX, qBY*stride+qBX
	if pi < 0 || qi < 0 || pi >= len(nnzY) || qi >= len(nnzY) {
		if edgeMB {
			return 4
		}
		return 3
	}
	// Strength counts whole-8x8 presence for 8x8-transform blocks (the
	// first 4x4 slot of each 8x8 holds the block sum) and per-block
	// counts otherwise, mirroring the reference per-8x8 coded flags.
	if t8count(nnzY, stride, mbT8, mbW, mbH, pBX, pBY, pMB) > 0 ||
		t8count(nnzY, stride, mbT8, mbW, mbH, qBX, qBY, qMB) > 0 {
		return 2
	}
	if refIdx != nil {
		if useM != nil {
			if bRefsDiffer(mvX, mvY, refIdx, mvX1, mvY1, refIdx1, refList, refList1, pi, qi) {
				return 1
			}
		} else if refList != nil {
			// refIdx entries are int8 indices; negative marks
			// unavailable (intra/cleared) slots.
			refOf := func(i int) *Picture {
				idx := int(refIdx[i])
				if idx < 0 || idx >= len(refList) {
					return nil
				}
				return refList[idx]
			}
			if refOf(pi) != refOf(qi) {
				return 1
			}
		} else if refIdx[pi] != refIdx[qi] {
			return 1
		}
	}
	if useM != nil {
		return 0
	}
	if mvX != nil {
		dx := int(mvX[pi]) - int(mvX[qi])
		dy := int(mvY[pi]) - int(mvY[qi])
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		if dx >= 4 || dy >= 4 {
			return 1
		}
	}
	return 0
}

// edgeInterior reports whether all four 16-pixel segments of an edge
// stay clear of the picture border, so per-segment bounds checks can be
// skipped. ex,ey is the edge origin in luma pixels; coded size is exact
// multiples of 16 by construction. Horizontal edges filter rows ey..ey+15
// starting at column ex-1 (p-side tap); vertical edges filter columns
// ex..ex+15 starting at row ey-1 — the minus-one side is what the guard
// covers.
func edgeInterior(ex, ey int, vertical bool, mbW, mbH int) bool {
	if vertical {
		return ey >= 4 && ey+16 <= mbH*16 && ex >= 0 && ex+16 <= mbW*16
	}
	return ex >= 4 && ex+16 <= mbW*16 && ey >= 0 && ey+16 <= mbH*16
}

// interBSEdge verdicts all four 4-line segments of one 16-pixel edge.
// Exact batch of 4x interBSInterior: identical outputs segment by
// segment. The DeblockPicture loop hands MB-aligned origins (ex/ey are
// multiples of 4: ex=mbx*16+e*4 / ey=mby*16, or the transpose), so the
// four segments step exactly one 4x4 block each and share one MB pair:
// divisions and the MB-intra verdict run once, per-segment block work
// (t8count, ref/mv compares) keeps the Interior body verbatim via
// running indexes (no multiplies, no divisions per segment).
// Non-aligned origins fall back to 4x Interior calls (one predictable
// branch per edge; no caller passes those today).
// Precondition (same as interBSInterior): edgeInterior checked,
// full-length arrays.
func interBSEdge(bSedge *[4]int, mbIntra []bool, nnzY []int8, mvX, mvY []int16, refIdx []int8, refList []*Picture, mbW, mbH int, ex, ey int, vertical bool, edgeMB bool, mbT8 []bool, mvX1, mvY1 []int16, refIdx1 []int8, refList1 []*Picture, useM []uint8) {
	if ex&3 != 0 || ey&3 != 0 {
		for seg := 0; seg < 4; seg++ {
			sx, sy := ex, ey+seg*4
			if vertical {
				sx, sy = ex+seg*4, ey
			}
			bSedge[seg] = interBSInterior(mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
		}
		return
	}
	stride := mbW * 4
	var pBX0, pBY0, qBX0, qBY0 int
	// S1b-W div-to-shift: all dividends are non-negative here
	// (fast path guarantees edgeInterior: horizontal needs ex>=4
	// and ey>=0, vertical needs ey>=4 and ex>=0, so ex-1>=3 and
	// ey-1>=3 where subtracted), and for x>=0, x/4 == x>>2 exactly
	// (truncation and floor agree). Same values, no DIV sequence.
	if !vertical {
		pBX0, pBY0 = (ex-1)>>2, ey>>2
		qBX0, qBY0 = ex>>2, ey>>2
	} else {
		pBX0, pBY0 = ex>>2, (ey-1)>>2
		qBX0, qBY0 = ex>>2, ey>>2
	}
	// One MB pair for all four segments (origins MB-aligned, steps of
	// one block stay inside the pair): a single intra verdict decides
	// all four, exactly like 4x Interior returning 4/3 each.
	pMB := (pBY0>>2)*mbW + (pBX0 >> 2)
	qMB := (qBY0>>2)*mbW + (qBX0 >> 2)
	if mbIntra[pMB] || mbIntra[qMB] {
		v := 3
		if edgeMB {
			v = 4
		}
		bSedge[0], bSedge[1], bSedge[2], bSedge[3] = v, v, v, v
		return
	}
	pi0, qi0 := pBY0*stride+pBX0, qBY0*stride+qBX0
	step := stride
	if vertical {
		step = 1
	}
	// S1b-S segment-inline: the per-segment call overhead (19 args,
	// call + bounds recheck per segment) was ~130ms flat per 90f
	// profile — a third of this function's total. The loop below runs
	// the interBSSegment body verbatim per segment (t8count, B-ref
	// compare, ref/mv compares), same order, same values; the helper
	// stays as the shared tail for the non-aligned entry so the two
	// cannot drift apart (gated by TestS1InterBSEdgeMatchesSegments).
	nnzn := len(nnzY)
	pT8 := pMB >= 0 && pMB < mbW*mbH && pMB < len(mbT8) && mbT8[pMB]
	qT8 := qMB >= 0 && qMB < mbW*mbH && qMB < len(mbT8) && mbT8[qMB]
	useB := useM != nil
	hasList := refList != nil
	l0n := len(refList)
	l1n := len(refList1)
	// S1b-V running indices: pi/qi were recomputed per segment as
	// pi0+seg*step (a multiply each, ~30ms flat per 90f profile).
	// Precompute the four offsets once (2 multiplies total) and add
	// per segment; values bit-identical to the old form (same start,
	// same step, 4 steps). Offset table (not running vars) because
	// the body below uses continue per verdict — a trailing
	// pi+=step would be skipped by every continue and drift.
	o1, o2, o3 := step, step*2, step*3
	off := [4]int{0, o1, o2, o3}
	// S1b-AA edge-level validation (one predictable branch per edge):
	// the fast caller guarantees edgeInterior + full-length arrays,
	// so all four pi/qi land in [0,nnzn) and pMB/qMB in range; the
	// loops below then run without per-segment bounds checks
	// (identical values — the checks never fired). Anything off
	// (border probes in tests) falls back to 4x checked Interior
	// calls with identical verdicts, so the contract holds.
	nmb := len(mbT8)
	needP, needQ := pi0+3*step, qi0+3*step
	if pMB < 0 || pMB >= nmb || qMB < 0 || qMB >= nmb ||
		pi0 < 0 || qi0 < 0 || needP >= nnzn || needQ >= nnzn ||
		needP >= len(mvX) || needQ >= len(mvX) ||
		needP >= len(mvY) || needQ >= len(mvY) ||
		needP >= len(refIdx) || needQ >= len(refIdx) ||
		(useB && (needP >= len(mvX1) || needQ >= len(mvX1) ||
			needP >= len(mvY1) || needQ >= len(mvY1) ||
			needP >= len(refIdx1) || needQ >= len(refIdx1))) {
		for seg := 0; seg < 4; seg++ {
			sx, sy := ex, ey+seg*4
			if vertical {
				sx, sy = ex+seg*4, ey
			}
			bSedge[seg] = interBSInterior(mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
		}
		return
	}
	// S1b-AA mode-split loops: useB/hasList are edge-constant, so
	// each edge runs exactly one loop with no per-segment mode
	// branches. Bodies are the verbatim S1b-S inline (t8count, B-ref
	// compare, ref/mv compares), only the bounds checks dropped per
	// the validation above; the helpers stay as shared tails.
	if useB {
		for seg := 0; seg < 4; seg++ {
			pi, qi := pi0+off[seg], qi0+off[seg]
			var pv, qv int
			// S1b-AC lazy coords: pBX/pBY/qBX/qBY are only used
			// on the t8 path (8x8 transform merges 2x2 nnz slots).
			// The common non-t8 path reads nnzY[pi]/nnzY[qi]
			// directly, so the per-segment vertical branch + 4
			// assigns are dead work there. Compute coords only
			// inside the t8 else, same values as before.
			if !pT8 {
				pv = int(nnzY[pi])
			} else {
				var pBX, pBY int
				if !vertical {
					pBX, pBY = pBX0, pBY0+seg
				} else {
					pBX, pBY = pBX0+seg, pBY0
				}
				pv = int(nnzY[(pBY&^1)*stride+(pBX&^1)])
			}
			if pv > 0 {
				bSedge[seg] = 2
				continue
			}
			if !qT8 {
				qv = int(nnzY[qi])
			} else {
				var qBX, qBY int
				if !vertical {
					qBX, qBY = qBX0, qBY0+seg
				} else {
					qBX, qBY = qBX0+seg, qBY0
				}
				qv = int(nnzY[(qBY&^1)*stride+(qBX&^1)])
			}
			if qv > 0 {
				bSedge[seg] = 2
				continue
			}
			// S1b-AB B-ref unchecked inline: edge validation above
			// guarantees pi/qi in range for all motion/ref arrays
			// (short arrays fell back to checked Interior), so the
			// per-segment pi/qi range + nil checks in
			// bRefsDifferInterior never fire. Only the value guards
			// stay: negative ref index (unused list slot) and list
			// index past a short/duplicate-reordered list. Verbatim
			// the bRefsDifferInterior verdicts; the helper stays the
			// shared tail for the fallback path (gated by
			// TestS1InterBSEdgeMatchesSegments).
			var p0p, p0q, p1p, p1q *Picture
			if rp := refIdx[pi]; rp >= 0 {
				if idx := int(rp); idx < l0n {
					p0p = refList[idx]
				}
			}
			if rq := refIdx[qi]; rq >= 0 {
				if idx := int(rq); idx < l0n {
					p0q = refList[idx]
				}
			}
			if rp := refIdx1[pi]; rp >= 0 {
				if idx := int(rp); idx < l1n {
					p1p = refList1[idx]
				}
			}
			if rq := refIdx1[qi]; rq >= 0 {
				if idx := int(rq); idx < l1n {
					p1q = refList1[idx]
				}
			}
			v := p0p != p0q
			if !v && p0p != nil {
				// S1b-AC branchless threshold (same as hasList lane).
				dx := int(mvX[pi]) - int(mvX[qi])
				dy := int(mvY[pi]) - int(mvY[qi])
				v = dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4
			}
			if v {
				bSedge[seg] = 1
				continue
			}
			v = p1p != p1q
			if !v && p1p != nil {
				dx := int(mvX1[pi]) - int(mvX1[qi])
				dy := int(mvY1[pi]) - int(mvY1[qi])
				v = dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4
			}
			if !v {
				bSedge[seg] = 0
				continue
			}
			if p0p != p1q || p1p != p0q {
				bSedge[seg] = 1
				continue
			}
			dx := int(mvX[pi]) - int(mvX1[qi])
			dy := int(mvY[pi]) - int(mvY1[qi])
			if dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4 {
				bSedge[seg] = 1
				continue
			}
			dx = int(mvX1[pi]) - int(mvX[qi])
			dy = int(mvY1[pi]) - int(mvY[qi])
			if dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4 {
				bSedge[seg] = 1
			} else {
				bSedge[seg] = 0
			}
		}
		return
	}
	if hasList {
		for seg := 0; seg < 4; seg++ {
			pi, qi := pi0+off[seg], qi0+off[seg]
			var pv, qv int
			// S1b-AC lazy coords (same as useB lane above).
			if !pT8 {
				pv = int(nnzY[pi])
			} else {
				var pBX, pBY int
				if !vertical {
					pBX, pBY = pBX0, pBY0+seg
				} else {
					pBX, pBY = pBX0+seg, pBY0
				}
				pv = int(nnzY[(pBY&^1)*stride+(pBX&^1)])
			}
			if pv > 0 {
				bSedge[seg] = 2
				continue
			}
			if !qT8 {
				qv = int(nnzY[qi])
			} else {
				var qBX, qBY int
				if !vertical {
					qBX, qBY = qBX0, qBY0+seg
				} else {
					qBX, qBY = qBX0+seg, qBY0
				}
				qv = int(nnzY[(qBY&^1)*stride+(qBX&^1)])
			}
			if qv > 0 {
				bSedge[seg] = 2
				continue
			}
			var pp, qq *Picture
			if rp := refIdx[pi]; rp >= 0 && int(rp) < l0n {
				pp = refList[rp]
			}
			if rq := refIdx[qi]; rq >= 0 && int(rq) < l0n {
				qq = refList[rq]
			}
			if pp != qq {
				bSedge[seg] = 1
				continue
			}
			// S1b-AC branchless threshold: abs(dx)>=4 is exactly
			// dx>=4||dx<=-4 (same for dy), so the two abs branches
			// go away with identical verdicts.
			dx := int(mvX[pi]) - int(mvX[qi])
			dy := int(mvY[pi]) - int(mvY[qi])
			if dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4 {
				bSedge[seg] = 1
				continue
			}
			bSedge[seg] = 0
		}
		return
	}
	for seg := 0; seg < 4; seg++ {
		pi, qi := pi0+off[seg], qi0+off[seg]
		var pv, qv int
		// S1b-AC lazy coords (same as the two lanes above).
		if !pT8 {
			pv = int(nnzY[pi])
		} else {
			var pBX, pBY int
			if !vertical {
				pBX, pBY = pBX0, pBY0+seg
			} else {
				pBX, pBY = pBX0+seg, pBY0
			}
			pv = int(nnzY[(pBY&^1)*stride+(pBX&^1)])
		}
		if pv > 0 {
			bSedge[seg] = 2
			continue
		}
		if !qT8 {
			qv = int(nnzY[qi])
		} else {
			var qBX, qBY int
			if !vertical {
				qBX, qBY = qBX0, qBY0+seg
			} else {
				qBX, qBY = qBX0+seg, qBY0
			}
			qv = int(nnzY[(qBY&^1)*stride+(qBX&^1)])
		}
		if qv > 0 {
			bSedge[seg] = 2
			continue
		}
		if refIdx[pi] != refIdx[qi] {
			bSedge[seg] = 1
			continue
		}
		// S1b-AC branchless threshold (same as hasList lane above).
		dx := int(mvX[pi]) - int(mvX[qi])
		dy := int(mvY[pi]) - int(mvY[qi])
		if dx >= 4 || dx <= -4 || dy >= 4 || dy <= -4 {
			bSedge[seg] = 1
			continue
		}
		bSedge[seg] = 0
	}
}

// interBSSegment is the per-segment tail of interBSInterior after the
// coordinate divisions and the MB-intra verdict: t8count, ref/mv
// compares. Byte-for-byte the same body; factored so the edge batch
// and the per-segment entry share it instead of drifting apart.
func interBSSegment(nnzY []int8, mvX, mvY []int16, refIdx []int8, refList []*Picture, mbW, mbH int, pBX, pBY, qBX, qBY, pMB, qMB, pi, qi, stride int, mbT8 []bool, mvX1, mvY1 []int16, refIdx1 []int8, refList1 []*Picture, useM []uint8) int {
	if t8count(nnzY, stride, mbT8, mbW, mbH, pBX, pBY, pMB) > 0 ||
		t8count(nnzY, stride, mbT8, mbW, mbH, qBX, qBY, qMB) > 0 {
		return 2
	}
	if useM != nil {
		if bRefsDifferInterior(mvX, mvY, refIdx, mvX1, mvY1, refIdx1, refList, refList1, pi, qi) {
			return 1
		}
		return 0
	}
	if refList != nil {
		// Same-picture-different-index compares EQUAL (reference
		// reordering duplicates one picture at two indexes): compare
		// pictures, not raw indexes — exactly like the safe path.
		var pp, qq *Picture
		if rp := refIdx[pi]; rp >= 0 && int(rp) < len(refList) {
			pp = refList[rp]
		}
		if rq := refIdx[qi]; rq >= 0 && int(rq) < len(refList) {
			qq = refList[rq]
		}
		if pp != qq {
			return 1
		}
	} else if refIdx[pi] != refIdx[qi] {
		return 1
	}
	dx := int(mvX[pi]) - int(mvX[qi])
	if dx < 0 {
		dx = -dx
	}
	dy := int(mvY[pi]) - int(mvY[qi])
	if dy < 0 {
		dy = -dy
	}
	if dx >= 4 || dy >= 4 {
		return 1
	}
	return 0
}

// interBSInterior is interBS for interior edges: identical results with
// no bounds checks and no closures. Callers must check edgeInterior
// first and pass full-length arrays. Thin entry over interBSSegment
// (same coordinates, same intra verdict, shared tail); the edge batch
// calls the batch entry instead.
func interBSInterior(mbIntra []bool, nnzY []int8, mvX, mvY []int16, refIdx []int8, refList []*Picture, mbW, mbH int, sx, sy int, vertical bool, edgeMB bool, mbT8 []bool, mvX1, mvY1 []int16, refIdx1 []int8, refList1 []*Picture, useM []uint8) int {
	var pBX, pBY, qBX, qBY int
	if !vertical {
		pBX, pBY = (sx-1)/4, sy/4
		qBX, qBY = sx/4, sy/4
	} else {
		pBX, pBY = sx/4, (sy-1)/4
		qBX, qBY = sx/4, sy/4
	}
	stride := mbW * 4
	pMB := (pBY/4)*mbW + pBX/4
	qMB := (qBY/4)*mbW + qBX/4
	if mbIntra[pMB] || mbIntra[qMB] {
		if edgeMB {
			return 4
		}
		return 3
	}
	pi, qi := pBY*stride+pBX, qBY*stride+qBX
	return interBSSegment(nnzY, mvX, mvY, refIdx, refList, mbW, mbH, pBX, pBY, qBX, qBY, pMB, qMB, pi, qi, stride, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
}

// bRefsDifferInterior is bRefsDiffer for in-range indexes: no bounds
// checks, no closures. Shape mirrors bRefsDiffer exactly, including the
// cross-list mirror pairing.
//
// Reference identity compares pictures, not bare list indexes: list 0
// index 0 and list 1 index 0 are different pictures, and the reference
// decoder's filter caches carry the same picture identity
// (h264_slice.c fill_filter_caches_inter ref2frm, read by
// h264_loopfilter.c check_mv). Bare-index compare reports the B-skip
// edge equal when it is not, dropping a bS=1 filter.
//
// S1b-C: closure-free lane. The old neg1/ge4 closures allocated per
// call in spirit (inlined, but blocking optimization); inline compares
// keep identical semantics with no call overhead.
func bRefsDifferInterior(mvX, mvY []int16, refIdx []int8, mvX1, mvY1 []int16, refIdx1 []int8, refList, refList1 []*Picture, pi, qi int) bool {
	// S1b-R at-inline (bit-identical to 4x the at closure, no calls):
	// the closure ran 4 guarded loads per segment through a call each
	// (~80ms flat per 90f profile for the body alone); explicit blocks
	// let the call overhead vanish and the compiler CSE the repeated
	// refIdx[pi]/refIdx[qi] loads. Same nil/negative/range verdicts.
	var p0p, p0q, p1p, p1q *Picture
	if refIdx != nil {
		if pi >= 0 && pi < len(refIdx) && refIdx[pi] >= 0 {
			if idx := int(refIdx[pi]); idx >= 0 && idx < len(refList) {
				p0p = refList[idx]
			}
		}
		if qi >= 0 && qi < len(refIdx) && refIdx[qi] >= 0 {
			if idx := int(refIdx[qi]); idx >= 0 && idx < len(refList) {
				p0q = refList[idx]
			}
		}
	}
	if refIdx1 != nil {
		if pi >= 0 && pi < len(refIdx1) && refIdx1[pi] >= 0 {
			if idx := int(refIdx1[pi]); idx >= 0 && idx < len(refList1) {
				p1p = refList1[idx]
			}
		}
		if qi >= 0 && qi < len(refIdx1) && refIdx1[qi] >= 0 {
			if idx := int(refIdx1[qi]); idx >= 0 && idx < len(refList1) {
				p1q = refList1[idx]
			}
		}
	}
	v := p0p != p0q
	if !v && p0p != nil {
		dx := int(mvX[pi]) - int(mvX[qi])
		if dx < 0 {
			dx = -dx
		}
		dy := int(mvY[pi]) - int(mvY[qi])
		if dy < 0 {
			dy = -dy
		}
		v = dx >= 4 || dy >= 4
	}
	if !v {
		v = p1p != p1q
		if !v && p1p != nil {
			dx := int(mvX1[pi]) - int(mvX1[qi])
			if dx < 0 {
				dx = -dx
			}
			dy := int(mvY1[pi]) - int(mvY1[qi])
			if dy < 0 {
				dy = -dy
			}
			v = dx >= 4 || dy >= 4
		}
		if v {
			if p0p != p1q || p1p != p0q {
				return true
			}
			dx := int(mvX[pi]) - int(mvX1[qi])
			if dx < 0 {
				dx = -dx
			}
			dy := int(mvY[pi]) - int(mvY1[qi])
			if dy < 0 {
				dy = -dy
			}
			if dx >= 4 || dy >= 4 {
				return true
			}
			dx = int(mvX1[pi]) - int(mvX[qi])
			if dx < 0 {
				dx = -dx
			}
			dy = int(mvY1[pi]) - int(mvY[qi])
			if dy < 0 {
				dy = -dy
			}
			return dx >= 4 || dy >= 4
		}
	}
	return v
}

// ge4 reports quarter-pel motion differences of 1 luma sample or more.
func ge4(ax, ay, bx, by int16) bool {
	dx, dy := int(ax)-int(bx), int(ay)-int(by)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	return dx >= 4 || dy >= 4
}

// bRefsDiffer compares one inter edge across both reference lists by
// picture identity (nil for unused slots, like the reference's
// LIST_NOT_USED; callers zero unused-list motion, so cross-list
// compares stay deterministic): list-0 pictures-or-motion, then list 1,
// then the cross-list pairing when one side's lists mirror the other's.
// Same peer as bRefsDifferInterior above.
func bRefsDiffer(mvX, mvY []int16, refIdx []int8, mvX1, mvY1 []int16, refIdx1 []int8, refList, refList1 []*Picture, pi, qi int) bool {
	at := func(list []*Picture, ref []int8, i int) *Picture {
		if ref == nil || i < 0 || i >= len(ref) || ref[i] < 0 {
			return nil
		}
		if idx := int(ref[i]); idx >= 0 && idx < len(list) {
			return list[idx]
		}
		return nil
	}
	mvGe4 := func(ax, ay, bx, by int16) bool {
		dx, dy := int(ax)-int(bx), int(ay)-int(by)
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		return dx >= 4 || dy >= 4
	}
	mv0p, mv0q := [2]int16{}, [2]int16{}
	mv1p, mv1q := [2]int16{}, [2]int16{}
	if mvX != nil && pi < len(mvX) && qi < len(mvX) {
		mv0p = [2]int16{mvX[pi], mvY[pi]}
		mv0q = [2]int16{mvX[qi], mvY[qi]}
	}
	if mvX1 != nil && pi < len(mvX1) && qi < len(mvX1) {
		mv1p = [2]int16{mvX1[pi], mvY1[pi]}
		mv1q = [2]int16{mvX1[qi], mvY1[qi]}
	}
	r0p, r0q := at(refList, refIdx, pi), at(refList, refIdx, qi)
	r1p, r1q := at(refList1, refIdx1, pi), at(refList1, refIdx1, qi)
	v := r0p != r0q
	if !v && r0p != nil {
		v = mvGe4(mv0p[0], mv0p[1], mv0q[0], mv0q[1])
	}
	if !v {
		v = r1p != r1q
		if !v && r1p != nil {
			v = mvGe4(mv1p[0], mv1p[1], mv1q[0], mv1q[1])
		}
		if v {
			if r0p != r1q || r1p != r0q {
				return true
			}
			return mvGe4(mv0p[0], mv0p[1], mv1q[0], mv1q[1]) ||
				mvGe4(mv1p[0], mv1p[1], mv0q[0], mv0q[1])
		}
	}
	return v
}
