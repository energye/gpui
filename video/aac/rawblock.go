package aac

import (
	"fmt"
)

// cpeResult holds one decoded channel pair.
type cpeResult struct {
	left   *singleResult
	right  *singleResult
	common bool
}

// decodeCPE decodes one channel_pair_element. commonWindow selects shared
// ICS info. samplingIdx/frameLen select tables; random is the noise RNG.
func decodeCPE(r *bitReaderMSB, samplingIdx, frameLen int, prevSeq [2]int, prevKb [2]bool, random *uint32) (*cpeResult, [2]int, [2]bool, error) {
	cw, err := r.read1()
	if err != nil {
		return nil, prevSeq, prevKb, err
	}
	common := cw == 1
	var ics0, ics1 icsInfo
	var msPresent int
	var msMask []bool
	var maxSfbSte int
	if common {
		ics0, err = parseICSInfo(r, samplingIdx, frameLen, prevSeq[0], prevKb[0])
		if err != nil {
			return nil, prevSeq, prevKb, err
		}
		// ch1 inherits, keeping its prev overlap shape and sequence.
		ics1 = ics0
		ics1.useKbWindow[1] = prevKb[1]
		ics1.windowSeq[1] = prevSeq[1]
		// LC must not have predictor; parseICSInfo already rejects.
		ms, err := r.read(2)
		if err != nil {
			return nil, prevSeq, prevKb, err
		}
		if ms == 3 {
			return nil, prevSeq, prevKb, fmt.Errorf("%w: ms 3", ErrBadADTS)
		}
		msPresent = ms
		maxSfbSte = ics0.maxSfb
		if msPresent == 1 {
			n := ics0.numGroups * ics0.maxSfb
			msMask = make([]bool, n)
			for i := 0; i < n; i++ {
				b, err := r.read1()
				if err != nil {
					return nil, prevSeq, prevKb, err
				}
				msMask[i] = b == 1
			}
		} else if msPresent == 2 {
			n := ics0.numGroups * ics0.maxSfb
			msMask = make([]bool, n)
			for i := range msMask {
				msMask[i] = true
			}
		}
		left, err := decodeSingle(r, &ics0, samplingIdx, frameLen, random)
		if err != nil {
			return nil, prevSeq, prevKb, err
		}
		right, err := decodeSingle(r, &ics1, samplingIdx, frameLen, random)
		if err != nil {
			return nil, prevSeq, prevKb, err
		}
		// M/S butterflies.
		if msPresent != 0 {
			applyMidSide(left.coef, right.coef, left.bt, right.bt, &ics0, msMask, maxSfbSte)
		}
		// Intensity stereo (right 14/15 rebuilt from left).
		applyIntensity(left.coef, right, &ics0, msMask, msPresent != 0)
		// TNS per channel (after stereo, before IMDCT).
		if left.hasTNS {
			applyTNSFilter(left.coef, &ics0, &left.tns, samplingIdx)
		}
		if right.hasTNS {
			applyTNSFilter(right.coef, &ics1, &right.tns, samplingIdx)
		}
		newSeq := [2]int{ics0.windowSeq[0], ics1.windowSeq[0]}
		newKb := [2]bool{ics0.useKbWindow[0], ics1.useKbWindow[0]}
		return &cpeResult{left: left, right: right, common: true}, newSeq, newKb, nil
	}
	// Independent windows.
	ics0, err = parseICSInfo(r, samplingIdx, frameLen, prevSeq[0], prevKb[0])
	if err != nil {
		return nil, prevSeq, prevKb, err
	}
	ics1, err = parseICSInfo(r, samplingIdx, frameLen, prevSeq[1], prevKb[1])
	if err != nil {
		return nil, prevSeq, prevKb, err
	}
	left, err := decodeSingle(r, &ics0, samplingIdx, frameLen, random)
	if err != nil {
		return nil, prevSeq, prevKb, err
	}
	right, err := decodeSingle(r, &ics1, samplingIdx, frameLen, random)
	if err != nil {
		return nil, prevSeq, prevKb, err
	}
	applyIntensity(left.coef, right, &ics1, nil, false)
	if left.hasTNS {
		applyTNSFilter(left.coef, &ics0, &left.tns, samplingIdx)
	}
	if right.hasTNS {
		applyTNSFilter(right.coef, &ics1, &right.tns, samplingIdx)
	}
	newSeq := [2]int{ics0.windowSeq[0], ics1.windowSeq[0]}
	newKb := [2]bool{ics0.useKbWindow[0], ics1.useKbWindow[0]}
	return &cpeResult{left: left, right: right}, newSeq, newKb, nil
}

// applyMidSide peers butterflies_float: L=(M+S), R=(M-S)? ffmpeg does
// v1+=v2 in place (L=M+S, R=M-S via temp). Replicate: newL=L+R, newR=L-R.
// Bands where either side is noise/intensity (>= NOISE_BT) are skipped,
// peers the band_type guard in apply_mid_side_stereo.
func applyMidSide(left, right []float64, leftBt, rightBt []int, ics *icsInfo, mask []bool, maxSfbSte int) {
	off := ics.swbOffset
	for g := 0; g < ics.numGroups; g++ {
		for sfb := 0; sfb < maxSfbSte && sfb < ics.maxSfb; sfb++ {
			idx := g*ics.maxSfb + sfb
			if idx >= len(mask) || !mask[idx] {
				continue
			}
			if idx >= len(leftBt) || idx >= len(rightBt) ||
				leftBt[idx] >= btNoise || rightBt[idx] >= btNoise {
				continue
			}
			start := int(off[sfb])
			end := int(off[sfb+1])
			for group := 0; group < ics.groupLen[g]; group++ {
				base := 0
				if ics.windowSeq[0] == 2 {
					// Short: locate window base.
					// winOff accumulates previous groups.
					winBase := 0
					for gg := 0; gg < g; gg++ {
						winBase += ics.groupLen[gg] * 128
					}
					base = winBase + group*128
				}
				for k := start; k < end; k++ {
					l := left[base+k]
					rr := right[base+k]
					left[base+k] = l + rr
					right[base+k] = l - rr
				}
			}
		}
	}
}

// applyIntensity rebuilds right intensity bands from left.
func applyIntensity(left []float64, right *singleResult, ics *icsInfo, mask []bool, hasMS bool) {
	off := ics.swbOffset
	for g := 0; g < ics.numGroups; g++ {
		for sfb := 0; sfb < ics.maxSfb; sfb++ {
			idx := g*ics.maxSfb + sfb
			b := right.bt[idx]
			if b != btInt && b != btInt2 {
				continue
			}
			c := -1.0
			if b == btInt {
				c = 1.0
			}
			if hasMS && idx < len(mask) && mask[idx] {
				c = -c
			}
			scale := c * right.sf[idx]
			start := int(off[sfb])
			end := int(off[sfb+1])
			for group := 0; group < ics.groupLen[g]; group++ {
				base := 0
				if ics.windowSeq[0] == 2 {
					winBase := 0
					for gg := 0; gg < g; gg++ {
						winBase += ics.groupLen[gg] * 128
					}
					base = winBase + group*128
				}
				for k := start; k < end; k++ {
					right.coef[base+k] = left[base+k] * scale
				}
			}
		}
	}
}
