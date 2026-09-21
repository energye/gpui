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
		if !vertical {
			p[(ey+line)*stride+ex-1] = uint8(clip3((2*p1+p0+q1+2)>>2, 0, 255))
			p[(ey+line)*stride+ex] = uint8(clip3((2*q1+q0+p1+2)>>2, 0, 255))
		} else {
			p[(ey-1)*stride+ex+line] = uint8(clip3((2*p1+p0+q1+2)>>2, 0, 255))
			p[(ey)*stride+ex+line] = uint8(clip3((2*q1+q0+p1+2)>>2, 0, 255))
		}
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
	qpAt := func(mbx, mby int) int32 {
		if mbx < 0 {
			mbx = 0
		}
		if mby < 0 {
			mby = 0
		}
		if mbx >= mbW {
			mbx = mbW - 1
		}
		if mby >= mbH {
			mby = mbH - 1
		}
		return qps[mby*mbW+mbx]
	}
	// Per-macroblock order like the reference: vertical edges (left
	// then internal) followed by horizontal edges (top then internal),
	// in MB raster order. Edges share one picture, so order matters.
	for mby := 0; mby < mbH; mby++ {
		for mbx := 0; mbx < mbW; mbx++ {
			addr := mby*mbW + mbx
			is8 := addr >= 0 && addr < len(mbT8) && mbT8[addr]
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
					qp := (qpAt(mbx, mby) + qpAt(qx, qy) + 1) >> 1
					alpha, beta := filterAlphaBeta(qp, fA[mby*mbW+mbx], fB[mby*mbW+mbx])
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
					var bSedge [4]int
					for seg := 0; seg < 4; seg++ {
						var sx, sy int
						if !vertical {
							sx, sy = ex, ey+seg*4
						} else {
							sx, sy = ex+seg*4, ey
						}
						var bS int
						if inFast {
							bS = interBSInterior(mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
						} else {
							bS = interBS(mbIntra, nnzY, mvX, mvY, refIdx, refList, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, mvX1, mvY1, refIdx1, refList1, useM)
						}
						bSedge[seg] = bS
						if bS == 0 {
							continue
						}
						tc := filterTC(qp, fA[mby*mbW+mbx], bS)
						// S1 dispatch: closed gates skip (scalar would
						// filter nothing), the arch kernel filters
						// directly in the picture, chroma stays scalar
						// (2 lines are too narrow to win back setup).
						if alpha == 0 || beta == 0 {
							continue
						}
						if !deblockLumaFast(pic.Y, W, sx, sy, vertical, bS, alpha, beta, tc) {
							filterLumaEdge(pic.Y, W, sx, sy, vertical, bS, alpha, beta, tc)
						}
					}
					// Chroma edges sit on even luma edges only (4:2:0: every
					// second one), with thresholds from averaged chroma QP.
					// Cb and Cr average separately: the second plane has its
					// own offset.
					if e%2 == 0 {
						qpcCb := (ChromaQP(qpAt(mbx, mby), cOff0) + ChromaQP(qpAt(qx, qy), cOff0) + 1) >> 1
						qpcCr := (ChromaQP(qpAt(mbx, mby), cOff1) + ChromaQP(qpAt(qx, qy), cOff1) + 1) >> 1
						alphaCb, betaCb := filterAlphaBeta(qpcCb, fA[mby*mbW+mbx], fB[mby*mbW+mbx])
						alphaCr, betaCr := filterAlphaBeta(qpcCr, fA[mby*mbW+mbx], fB[mby*mbW+mbx])
						for seg := 0; seg < 4; seg++ {
							// Chroma strength reuses the luma edge result:
							// identical derivation inputs, so no second
							// interBS call is needed.
							bS := bSedge[seg]
							if bS < 1 {
								continue
							}
							tcCb := filterTC(qpcCb, fA[mby*mbW+mbx], bS)
							tcCr := filterTC(qpcCr, fA[mby*mbW+mbx], bS)
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
								filterChromaIntraEdge(pic.Cb, W/2, cx, cy, vertical, alphaCb, betaCb)
								filterChromaIntraEdge(pic.Cr, W/2, cx, cy, vertical, alphaCr, betaCr)
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
			if bRefsDiffer(mvX, mvY, refIdx, mvX1, mvY1, refIdx1, pi, qi) {
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

// interBSInterior is interBS for interior edges: identical results with
// no bounds checks and no closures. Callers must check edgeInterior
// first and pass full-length arrays.
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
	if t8count(nnzY, stride, mbT8, mbW, mbH, pBX, pBY, pMB) > 0 ||
		t8count(nnzY, stride, mbT8, mbW, mbH, qBX, qBY, qMB) > 0 {
		return 2
	}
	if useM != nil {
		if bRefsDifferInterior(mvX, mvY, refIdx, mvX1, mvY1, refIdx1, pi, qi) {
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

// bRefsDifferInterior is bRefsDiffer for in-range indexes: no bounds
// checks, no closures. Shape mirrors bRefsDiffer exactly, including the
// cross-list mirror pairing.
//
// S1b-C: closure-free lane. The old neg1/ge4 closures allocated per
// call in spirit (inlined, but blocking optimization); inline compares
// keep identical semantics with no call overhead.
func bRefsDifferInterior(mvX, mvY []int16, refIdx []int8, mvX1, mvY1 []int16, refIdx1 []int8, pi, qi int) bool {
	// NOTE: negative indexes are all "unavailable" (-1, -2 sentinels):
	// normalize like the safe path's raw() instead of comparing raw.
	r0p := int(refIdx[pi])
	if r0p < 0 {
		r0p = -1
	}
	r0q := int(refIdx[qi])
	if r0q < 0 {
		r0q = -1
	}
	v := r0p != r0q
	if !v && r0p != -1 {
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
		r1p := int(refIdx1[pi])
		if r1p < 0 {
			r1p = -1
		}
		r1q := int(refIdx1[qi])
		if r1q < 0 {
			r1q = -1
		}
		v = r1p != r1q
		if !v && r1p != -1 {
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
			if r0p != r1q || r1p != r0q {
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

// bRefsDiffer compares one inter edge across both reference lists with
// raw index semantics (unavailable reads -1, like the reference;
// callers zero unused-list motion, so cross-list compares stay
// deterministic): list-0 pictures-or-motion, then list 1, then the
// cross-list pairing when one side's lists mirror the other's.
func bRefsDiffer(mvX, mvY []int16, refIdx []int8, mvX1, mvY1 []int16, refIdx1 []int8, pi, qi int) bool {
	raw := func(ref []int8, i int) int {
		if ref == nil || i < 0 || i >= len(ref) || ref[i] < 0 {
			return -1
		}
		return int(ref[i])
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
	r0p, r0q := raw(refIdx, pi), raw(refIdx, qi)
	r1p, r1q := raw(refIdx1, pi), raw(refIdx1, qi)
	v := r0p != r0q
	if !v && r0p != -1 {
		v = mvGe4(mv0p[0], mv0p[1], mv0q[0], mv0q[1])
	}
	if !v {
		v = r1p != r1q
		if !v && r1p != -1 {
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
