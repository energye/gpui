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

// filterChromaEdge filters 2 lines across a chroma edge (bS>=2 only).
func filterChromaEdge(p []uint8, stride int, ex, ey int, vertical bool, bS int, alpha, beta, tc int) {
	if bS < 2 {
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
func DeblockPicture(pic *Picture, qps []int32, fIDC []uint32, fA, fB []int32, mbW, mbH int, cOff0, cOff1 int32) {
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
			for _, vertical := range []bool{false, true} {
				for e := 0; e < 4; e++ {
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
					bS := 3
					if edgeMB {
						bS = 4
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
					tc := filterTC(qp, fA[mby*mbW+mbx], bS)
					for seg := 0; seg < 4; seg++ {
						var sx, sy int
						if !vertical {
							sx, sy = ex, ey+seg*4
						} else {
							sx, sy = ex+seg*4, ey
						}
						filterLumaEdge(pic.Y, W, sx, sy, vertical, bS, alpha, beta, tc)
					}
					// Chroma edges sit on even luma edges only (4:2:0: every
					// second one), with thresholds from averaged chroma QP.
					if e%2 == 0 {
						qpc := (ChromaQP(qpAt(mbx, mby), cOff0) + ChromaQP(qpAt(qx, qy), cOff0) + 1) >> 1
						alphaC, betaC := filterAlphaBeta(qpc, fA[mby*mbW+mbx], fB[mby*mbW+mbx])
						tcC := filterTC(qpc, fA[mby*mbW+mbx], bS)
						for seg := 0; seg < 4; seg++ {
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
								filterChromaIntraEdge(pic.Cb, W/2, cx, cy, vertical, alphaC, betaC)
								filterChromaIntraEdge(pic.Cr, W/2, cx, cy, vertical, alphaC, betaC)
								continue
							}
							filterChromaEdge(pic.Cb, W/2, cx, cy, vertical, bS, alphaC, betaC, tcC)
							filterChromaEdge(pic.Cr, W/2, cx, cy, vertical, bS, alphaC, betaC, tcC)
						}
					}
				}
			}
		}
	}
}
