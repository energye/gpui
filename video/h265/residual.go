package h265

import (
	"fmt"
)

// Residual syntax for one intra transform block (levels + signs,
// stored pre-dequant for the pixel stage).
// Peer (read-only): hevc/cabac.c ff_hevc_hls_residual_coding
// (last-pos, subset loop, greater1/2, remaining, signs) +
// last_significant_coeff_xy_prefix/suffix + sig/group/level helpers +
// bypass/unary/remaining/scan tables. Only bin order and context
// indices travel; dequant/transform stay in the pixel stage.

// residual parses one TU's coefficients and reports them in raster
// order (trafo_size x trafo_size). TU (x0,y0) rides in top-left units
// for the luma plane; chroma callers pre-shift (as the peer does).
func (p *synParser) residual(x0, y0, log2t, scan, cIdx int) ([]int16, error) {
	size := 1 << log2t
	out := make([]int16, size*size)
	if p.traceTU != nil {
		*p.traceTU = append(*p.traceTU, fmt.Sprintf("resid(%d,%d,%d,c%d)@%d", x0, y0, size, cIdx, p.cab.consumedBits()))
	}
	// Last significant position: prefix CABAC + suffix bypass.
	lx, ly, err := p.lastPos(cIdx, log2t)
	if err != nil {
		return nil, err
	}
	if scan == ScanVert {
		lx, ly = ly, lx
	}
	if lx >= size || ly >= size {
		return nil, fmt.Errorf("%w: last pos %d,%d in %d", ErrBadSlice, lx, ly, size)
	}
	cgLastX, cgLastY := lx>>2, ly>>2
	var cgXTab, cgYTab, offXTab, offYTab []uint8
	var numCoeff int
	switch scan {
	case ScanDiag:
		lastXC, lastYC := lx&3, ly&3
		numCoeff = int(diagScan4x4Inv[lastYC][lastXC])
		switch {
		case size == 4:
			cgXTab, cgYTab = []uint8{0}, []uint8{0}
		case size == 8:
			numCoeff += int(diagScan2x2Inv[cgLastY][cgLastX]) << 4
			cgXTab, cgYTab = diagScan2x2X[:], diagScan2x2Y[:]
		case size == 16:
			numCoeff += int(diagScan4x4Inv[cgLastY][cgLastX]) << 4
			cgXTab, cgYTab = diagScan4x4X[:], diagScan4x4Y[:]
		default: // 32
			numCoeff += int(diagScan8x8Inv[cgLastY][cgLastX]) << 4
			cgXTab, cgYTab = diagScan8x8X[:], diagScan8x8Y[:]
		}
		offXTab, offYTab = diagScan4x4X[:], diagScan4x4Y[:]
	case ScanHoriz:
		cgXTab, cgYTab = horizScan2x2X[:], horizScan2x2Y[:]
		offXTab, offYTab = horizScan4x4X[:], horizScan4x4Y[:]
		numCoeff = int(horizScan8x8Inv[ly][lx])
	default: // ScanVert: transposed horizontal tables
		cgXTab, cgYTab = horizScan2x2Y[:], horizScan2x2X[:]
		offXTab, offYTab = horizScan4x4Y[:], horizScan4x4X[:]
		numCoeff = int(horizScan8x8Inv[lx][ly])
	}
	numCoeff++
	lastSubset := (numCoeff - 1) >> 4
	nCG := size >> 2
	group := make([][]bool, nCG)
	for i := range group {
		group[i] = make([]bool, nCG)
	}
	greater1Ctx := 1
	for i := lastSubset; i >= 0; i-- {
		xcg, ycg := int(cgXTab[i]), int(cgYTab[i])
		if i < lastSubset && i > 0 {
			c := 0
			if xcg < nCG-1 && group[xcg+1][ycg] {
				c++
			}
			if ycg < nCG-1 && group[xcg][ycg+1] {
				c++
			}
			inc := 0
			if c > 0 {
				inc = 1
			}
			if cIdx > 0 {
				inc += 2
			}
			v, err := p.bin(ctxSignificantCoeffGroupFlag + inc)
			if err != nil {
				return nil, err
			}
			group[xcg][ycg] = v != 0
		} else {
			group[xcg][ycg] = (xcg == cgLastX && ycg == cgLastY) || (xcg == 0 && ycg == 0)
		}
		lastPos := numCoeff - (i << 4) - 1
		var nEnd int
		var idxList []int
		if i == lastSubset {
			nEnd = lastPos - 1
			idxList = []int{lastPos}
		} else {
			nEnd = 15
		}
		prevSig := 0
		if xcg < nCG-1 && group[xcg+1][ycg] {
			prevSig |= 1
		}
		if ycg < nCG-1 && group[xcg][ycg+1] {
			prevSig |= 2
		}
		if group[xcg][ycg] && nEnd >= 0 {
			off := p.sigOffset(scan, cIdx, log2t, xcg, ycg, prevSig)
			nb0 := len(idxList)
			implicit := i < lastSubset && i > 0
			for n := nEnd; n > 0; n-- {
				v, err := p.bin(ctxSignificantCoeffFlag + int(sigCtxIdxMap[p.sigMapRow(scan, log2t, prevSig)+n]) + off)
				if err != nil {
					return nil, err
				}
				if v != 0 {
					idxList = append(idxList, n)
				}
			}
			if len(idxList) != nb0 {
				implicit = false
			}
			if !implicit {
				var scf int
				if i == 0 {
					if cIdx == 0 {
						scf = 0
					} else {
						scf = 27
					}
				} else {
					scf = 2 + off
				}
				v, err := p.bin(ctxSignificantCoeffFlag + scf)
				if err != nil {
					return nil, err
				}
				if v != 0 {
					idxList = append(idxList, 0)
				}
			} else {
				idxList = append(idxList, 0)
			}
		}
		nEnd = len(idxList)
		if nEnd == 0 {
			continue
		}
		// Levels: first 8 positions use greater1/2 contexts.
		ctxSet := 0
		if i > 0 && cIdx == 0 {
			ctxSet = 2
		}
		if i != lastSubset {
			// Peer bumps the set when the previous subset saw a
			// greater1 (greater1Ctx drained to 0); nonzero keeps it.
			if greater1Ctx == 0 {
				ctxSet++
			}
		}
		greater1Ctx = 1
		gtMask := 0
		g1 := make([]int, 0, 8)
		nn := nEnd
		if nn > 8 {
			nn = 8
		}
		for m := 0; m < nn; m++ {
			inc := (ctxSet << 2) + greater1Ctx
			off := 0
			if cIdx > 0 {
				off = 16
			}
			v, err := p.bin(ctxCoeffAbsLevelGreater1Flag + inc + off)
			if err != nil {
				return nil, err
			}
			g1 = append(g1, v)
			if v != 0 {
				gtMask |= 1 << uint(m)
			}
			if v != 0 {
				greater1Ctx = 0
			} else if greater1Ctx > 0 && greater1Ctx < 3 {
				greater1Ctx++
			}
		}
		firstG1 := -1
		for m := 0; m < len(g1); m++ {
			if gtMask&(1<<uint(m)) != 0 {
				firstG1 = m
				break
			}
		}
		// Reverse idxList to scan order (list built last-first).
		for a, b := 0, len(idxList)-1; a < b; a, b = a+1, b-1 {
			idxList[a], idxList[b] = idxList[b], idxList[a]
		}
		firstNZ, lastNZ := idxList[0], idxList[len(idxList)-1]
		signHidden := lastNZ-firstNZ >= 4
		if p.pps.SignHiding && signHidden {
			// kept for the pixel stage's sign recovery
		}
		if firstG1 >= 0 {
			off := 0
			if cIdx > 0 {
				off = 4
			}
			v, err := p.bin(ctxCoeffAbsLevelGreater2Flag + ctxSet + off)
			if err != nil {
				return nil, err
			}
			g1[firstG1] += v
		}
		var signs uint16
		nSigns := len(idxList)
		if p.pps.SignHiding && signHidden {
			nSigns--
		}
		for k := 0; k < nSigns; k++ {
			signs = (signs << 1) | uint16(p.bypass())
		}
		signs <<= uint16(16 - nSigns)
		rice := 0
		sumAbs := 0
		for m, n := range idxList {
			var level int32 = 1
			if m < len(g1) {
				level += int32(g1[m])
				if level == int32((map[bool]int{true: 3, false: 2}[m == firstG1])) {
					rem, err := p.remaining(rice)
					if err != nil {
						return nil, err
					}
					level += int32(rem)
					rice = p.bumpRice(rice, int(level))
				}
			} else {
				rem, err := p.remaining(rice)
				if err != nil {
					return nil, err
				}
				level += int32(rem)
				rice = p.bumpRice(rice, int(level))
			}
			if p.pps.SignHiding && signHidden {
				sumAbs += int(level)
				if n == firstNZ && sumAbs&1 != 0 {
					level = -level
				}
			}
			sign := -int32(signs >> 15)
			level = (level ^ sign) - sign
			signs <<= 1
			xc, yc := (xcg<<2)+int(offXTab[n]), (ycg<<2)+int(offYTab[n])
			out[yc*size+xc] = int16(level)
			_ = x0
			_ = y0
		}
	}
	return out, nil
}

// lastPos reads the last-significant X/Y (both prefixes first, then
// both suffixes; suffix order matters on the wire).
func (p *synParser) lastPos(cIdx, log2t int) (int, int, error) {
	max := (log2t << 1) - 1
	var ctxOff, ctxShift int
	if cIdx == 0 {
		ctxOff = 3*(log2t-2) + ((log2t - 1) >> 2)
		ctxShift = (log2t + 1) >> 2
	} else {
		ctxOff = 15
		ctxShift = log2t - 2
	}
	var lx int
	for lx < max {
		v, err := p.bin(ctxLastSignificantCoeffXPrefix + (lx >> ctxShift) + ctxOff)
		if err != nil {
			return 0, 0, err
		}
		if v == 0 {
			break
		}
		lx++
	}
	var ly int
	for ly < max {
		v, err := p.bin(ctxLastSignificantCoeffYPrefix + (ly >> ctxShift) + ctxOff)
		if err != nil {
			return 0, 0, err
		}
		if v == 0 {
			break
		}
		ly++
	}
	if lx > 3 {
		s, err := p.lastSuffix(lx)
		if err != nil {
			return 0, 0, err
		}
		lx = (1<<((lx>>1)-1))*(2+(lx&1)) + s
	}
	if ly > 3 {
		s, err := p.lastSuffix(ly)
		if err != nil {
			return 0, 0, err
		}
		ly = (1<<((ly>>1)-1))*(2+(ly&1)) + s
	}
	return lx, ly, nil
}

// lastSuffix reads the bypass suffix for a large last position.
func (p *synParser) lastSuffix(prefix int) (int, error) {
	length := (prefix >> 1) - 1
	v := p.bypass()
	for i := 1; i < length; i++ {
		v = (v << 1) | p.bypass()
	}
	return v, nil
}

// sigMapRow selects the ctx_idx_map row for a subset position.
func (p *synParser) sigMapRow(scan, log2t, prevSig int) int {
	base := 0
	switch scan {
	case ScanHoriz:
		base = 80
	case ScanVert:
		base = 160
	}
	if log2t == 2 {
		return base
	}
	return base + (prevSig+1)*16
}

// sigOffset derives the significance context offset.
func (p *synParser) sigOffset(scan, cIdx, log2t, xcg, ycg, prevSig int) int {
	_ = prevSig
	if cIdx != 0 {
		if log2t == 2 {
			return 27
		}
		if log2t == 3 {
			return 27 + 9
		}
		return 27 + 12
	}
	if log2t == 2 {
		return 0
	}
	off := 0
	if xcg > 0 || ycg > 0 {
		off += 3
	}
	if log2t == 3 {
		if scan == ScanDiag {
			off += 9
		} else {
			off += 15
		}
	} else {
		off += 21
	}
	return off
}

// remaining reads coeff_abs_level_remaining with the rice parameter.
// It returns the base value; the caller bumps rice from the full level.
// The return also carries the updated rice for the next coefficient:
// levels above 3<<rice bump it (max 4).
func (p *synParser) remaining(rice int) (int, error) {
	prefix := 0
	for prefix < 31 {
		if p.bypass() == 0 {
			break
		}
		prefix++
	}
	// Peer rejects CABAC_MAX_BIN or k>22 (prefix-3+rice>16+6).
	if prefix == 31 || (prefix-3)+rice > 22 {
		return 0, fmt.Errorf("%w: remaining prefix %d rice %d", ErrBadSlice, prefix, rice)
	}
	var suffix int
	var k int
	if prefix < 3 {
		for i := 0; i < rice; i++ {
			suffix = (suffix << 1) | p.bypass()
		}
		base := (prefix << uint(rice)) + suffix
		return base, nil
	}
	pm3 := prefix - 3
	k = pm3 + rice
	v, err := p.bypassN(k)
	if err != nil {
		return 0, err
	}
	suffix = int(v)
	base := (((1 << uint(pm3)) + 3 - 1) << uint(rice)) + suffix
	return base, nil
}

// bumpRice mirrors the peer: full level above 3<<rice bumps (max 4).
func (p *synParser) bumpRice(rice, level int) int {
	if level > (3<<uint(rice)) && rice < 4 {
		return rice + 1
	}
	return rice
}
