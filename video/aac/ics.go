package aac

import (
	"fmt"
	"math"
)

// Band types (ISO 14496-3 Table 4.6 / aac.h).
const (
	btZero      = 0
	btFirstPair = 5
	btEsc       = 11
	btReserved  = 12
	btNoise     = 13
	btInt2      = 14
	btInt       = 15
)

// icsInfo holds one channel's stream side info.
type icsInfo struct {
	windowSeq   [2]int
	useKbWindow [2]bool
	numGroups   int
	groupLen    [8]int
	maxSfb      int
	numWindows  int
	swbOffset   []uint16
	numSwb      int
	predictor   bool
}

// singleChannel holds one SCE's payload.
type singleChannel struct {
	ics        icsInfo
	globalGain int
	bandType   []int
	sfo        []int
	sf         []float64
	coef       []float64
	tnsPresent bool
}

func parseICSInfo(r *bitReaderMSB, samplingIdx, frameLen int, prevSeq int, prevKb bool) (icsInfo, error) {
	var ics icsInfo
	if err := ensureHuff(); err != nil {
		return ics, err
	}
	rsv, err := r.read1()
	if err != nil {
		return ics, err
	}
	if rsv != 0 {
		// ffmpeg warns but continues unless err_recognition; we continue.
	}
	ics.windowSeq[1] = prevSeq
	ws, err := r.read(2)
	if err != nil {
		return ics, err
	}
	ics.windowSeq[0] = ws
	kb, err := r.read1()
	if err != nil {
		return ics, err
	}
	ics.useKbWindow[1] = prevKb
	ics.useKbWindow[0] = kb == 1
	ics.numGroups = 1
	ics.groupLen[0] = 1
	if ws == 2 { // EIGHT_SHORT
		maxSfb, err := r.read(4)
		if err != nil {
			return ics, err
		}
		ics.maxSfb = maxSfb
		ics.numWindows = 8
		ics.numGroups = 1
		ics.groupLen[0] = 1
		for i := 0; i < 7; i++ {
			b, err := r.read1()
			if err != nil {
				return ics, err
			}
			if b == 1 {
				ics.groupLen[ics.numGroups-1]++
			} else {
				ics.numGroups++
				ics.groupLen[ics.numGroups-1] = 1
			}
		}
		off, err := swbOffsetShort(frameLen, samplingIdx)
		if err != nil {
			return ics, err
		}
		ics.swbOffset = off
		n, err := numSwbShort(frameLen, samplingIdx)
		if err != nil {
			return ics, err
		}
		ics.numSwb = n
	} else {
		maxSfb, err := r.read(6)
		if err != nil {
			return ics, err
		}
		ics.maxSfb = maxSfb
		ics.numWindows = 1
		off, err := swbOffsetLong(frameLen, samplingIdx)
		if err != nil {
			return ics, err
		}
		ics.swbOffset = off
		n, err := numSwbLong(frameLen, samplingIdx)
		if err != nil {
			return ics, err
		}
		ics.numSwb = n
		pred, err := r.read1()
		if err != nil {
			return ics, err
		}
		ics.predictor = pred == 1
		if ics.predictor {
			return ics, fmt.Errorf("aac: predictor present: %w", ErrUnsupported)
		}
	}
	if ics.maxSfb > ics.numSwb {
		return ics, fmt.Errorf("%w: max_sfb %d > num_swb %d", ErrBadADTS, ics.maxSfb, ics.numSwb)
	}
	return ics, nil
}

func decodeBandTypes(r *bitReaderMSB, ics *icsInfo) ([]int, error) {
	bt := make([]int, ics.numGroups*ics.maxSfb)
	bits := 5
	if ics.windowSeq[0] == 2 {
		bits = 3
	}
	for g := 0; g < ics.numGroups; g++ {
		k := 0
		for k < ics.maxSfb {
			sectType, err := r.read(4)
			if err != nil {
				return nil, err
			}
			if sectType == btReserved {
				return nil, fmt.Errorf("%w: reserved band type", ErrBadADTS)
			}
			sectEnd := k
			for {
				incr, err := r.read(bits)
				if err != nil {
					return nil, err
				}
				sectEnd += incr
				if sectEnd > ics.maxSfb {
					return nil, fmt.Errorf("%w: bands %d > max %d", ErrBadADTS, sectEnd, ics.maxSfb)
				}
				if incr != (1<<bits)-1 {
					break
				}
			}
			for ; k < sectEnd; k++ {
				bt[g*ics.maxSfb+k] = sectType
			}
		}
	}
	return bt, nil
}

func decodeScalefactors(r *bitReaderMSB, ics *icsInfo, bt []int, globalGain int) ([]int, error) {
	sfo := make([]int, ics.numGroups*ics.maxSfb)
	offset := [3]int{globalGain, globalGain - 90, 0}
	noiseFlag := true
	for g := 0; g < ics.numGroups; g++ {
		for sfb := 0; sfb < ics.maxSfb; sfb++ {
			idx := g*ics.maxSfb + sfb
			switch bt[idx] {
			case btZero:
				sfo[idx] = 0
			case btInt, btInt2:
				sym, err := sfTree.decode(r)
				if err != nil {
					return nil, err
				}
				offset[2] += sym - 60
				if offset[2] < -155 || offset[2] > 100 {
					// ffmpeg clips with warning; clip.
					if offset[2] < -155 {
						offset[2] = -155
					} else {
						offset[2] = 100
					}
				}
				sfo[idx] = offset[2] - 100
			case btNoise:
				var diff int
				if noiseFlag {
					v, err := r.read(9)
					if err != nil {
						return nil, err
					}
					diff = v - 256
					noiseFlag = false
				} else {
					sym, err := sfTree.decode(r)
					if err != nil {
						return nil, err
					}
					diff = sym - 60
				}
				offset[1] += diff
				if offset[1] < -100 {
					offset[1] = -100
				} else if offset[1] > 155 {
					offset[1] = 155
				}
				sfo[idx] = offset[1]
			default:
				sym, err := sfTree.decode(r)
				if err != nil {
					return nil, err
				}
				offset[0] += sym - 60
				if offset[0] > 255 {
					return nil, fmt.Errorf("%w: scalefactor %d", ErrBadADTS, offset[0])
				}
				sfo[idx] = offset[0] - 100
			}
		}
	}
	return sfo, nil
}

// dequantScalefactors peers dequant_scalefactors (float): normal and
// noise share -2^(sfo/4); intensity uses +2^((-sfo-100)/4); zero is 0.
func dequantScalefactors(bt, sfo []int) []float64 {
	sf := make([]float64, len(bt))
	for i, b := range bt {
		switch b {
		case btZero:
			sf[i] = 0
		case btInt, btInt2:
			sf[i] = math.Pow(2, float64(-sfo[i]-100)/4)
		case btNoise:
			sf[i] = -math.Pow(2, float64(sfo[i])/4)
		default:
			sf[i] = -math.Pow(2, float64(sfo[i])/4)
		}
	}
	return sf
}
