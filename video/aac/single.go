package aac

import (
	"fmt"
	"math"
)

// singleResult is one decoded channel's spectra plus side data.
type singleResult struct {
	coef   []float64 // 1024 spectral lines
	ics    icsInfo
	bt     []int
	sfo    []int
	sf     []float64
	tns    tnsInfo
	hasTNS bool
}

// decodeSingle decodes global_gain onwards for one channel.
// ics is already parsed (shared or independent). random is the decoder
// noise RNG. SamplingIdx/frameLen select tables.
func decodeSingle(r *bitReaderMSB, ics *icsInfo, samplingIdx, frameLen int, random *uint32) (*singleResult, error) {
	gg, err := r.read(8)
	if err != nil {
		return nil, err
	}
	bt, err := decodeBandTypes(r, ics)
	if err != nil {
		return nil, err
	}
	sfo, err := decodeScalefactors(r, ics, bt, gg)
	if err != nil {
		return nil, err
	}
	sf := dequantScalefactors(bt, sfo)
	// Bitstream order peers ff_aac_decode_ics: pulse, TNS and gain
	// control come BEFORE the spectral data; pulse applies to the
	// decoded coefficients afterwards. Decoding spectrum first (the
	// old order here) misaligns every packet that carries pulse/TNS.
	var pulsePos [4]int
	var pulseAmp [4]int
	numPulse := 0
	pulsePresent, err := r.read1()
	if err != nil {
		return nil, err
	}
	if pulsePresent == 1 {
		if ics.windowSeq[0] == 2 {
			return nil, fmt.Errorf("%w: pulse in short", ErrBadADTS)
		}
		np, err := r.read(2)
		if err != nil {
			return nil, err
		}
		numPulse = np + 1
		startSfb, err := r.read(6)
		if err != nil {
			return nil, err
		}
		if startSfb >= ics.numSwb {
			return nil, fmt.Errorf("%w: pulse sfb %d", ErrBadADTS, startSfb)
		}
		off := int(ics.swbOffset[startSfb])
		top := int(ics.swbOffset[ics.numSwb])
		prevPos := off
		for i := 0; i < numPulse; i++ {
			posOff, err := r.read(5)
			if err != nil {
				return nil, err
			}
			amp, err := r.read(4)
			if err != nil {
				return nil, err
			}
			// ffmpeg decode_pulses: pos[0] is offset-based,
			// later ones accumulate on the previous position.
			pos := off + posOff
			if i > 0 {
				pos = prevPos + posOff
			}
			prevPos = pos
			if pos >= top {
				return nil, fmt.Errorf("%w: pulse pos %d", ErrBadADTS, pos)
			}
			pulsePos[i] = pos
			pulseAmp[i] = amp
		}
	}
	// TNS present flag.
	tnsFlag, err := r.read1()
	if err != nil {
		return nil, err
	}
	var tns tnsInfo
	hasTNS := false
	if tnsFlag == 1 {
		t, err := parseTNS(r, ics, AOTLC)
		if err != nil {
			return nil, err
		}
		tns = t
		hasTNS = t.present
	}
	// Gain control (SSR only).
	gainFlag, err := r.read1()
	if err != nil {
		return nil, err
	}
	if gainFlag == 1 {
		return nil, fmt.Errorf("%w: gain control", ErrUnsupported)
	}
	numLines := 1024
	coef := make([]float64, numLines)
	// Zero beyond max_sfb per window.
	c := 1024 / ics.numWindows
	if c < 1 {
		c = 1024
	}
	for g := 0; g < ics.numWindows; g++ {
		base := g * 128
		if ics.windowSeq[0] == 2 {
			// Short: each window has 128 lines.
			for i := int(ics.swbOffset[len(ics.swbOffset)-1]); i < 128; i++ {
				coef[base+i] = 0
			}
		} else {
			_ = base
		}
	}
	if ics.windowSeq[0] != 2 {
		for i := int(ics.swbOffset[ics.maxSfb]); i < c; i++ {
			coef[i] = 0
		}
	}
	// Spectrum per group/band.
	idx := 0
	winOff := 0
	for g := 0; g < ics.numGroups; g++ {
		gLen := ics.groupLen[g]
		for sfb := 0; sfb < ics.maxSfb; sfb, idx = sfb+1, idx+1 {
			b := bt[g*ics.maxSfb+sfb]
			off := int(ics.swbOffset[sfb])
			length := int(ics.swbOffset[sfb+1]) - off
			switch b {
			case btZero, btInt, btInt2:
				if b == btZero {
					for w := 0; w < gLen; w++ {
						base := winOff + w*128
						if ics.windowSeq[0] != 2 {
							base = 0
						}
						for k := 0; k < length; k++ {
							coef[base+off+k] = 0
						}
					}
				} else {
					// Intensity: spectra zero here; stereo stage
					// rebuilds right from left.
					for w := 0; w < gLen; w++ {
						base := winOff + w*128
						if ics.windowSeq[0] != 2 {
							base = 0
						}
						for k := 0; k < length; k++ {
							coef[base+off+k] = 0
						}
					}
				}
			case btNoise:
				for w := 0; w < gLen; w++ {
					base := winOff + w*128
					if ics.windowSeq[0] != 2 {
						base = 0
					}
					// Generate pseudo-random noise peers the float
					// path: int32 LCG values as float, scaled by
					// sf/sqrt(energy).
					tmp := make([]float64, length)
					var energy float64
					for k := 0; k < length; k++ {
						*random = *random*1664525 + 1013904223
						v := float64(int32(*random))
						tmp[k] = v
						energy += v * v
					}
					scale := sf[idx] / math.Sqrt(energy)
					for k := 0; k < length; k++ {
						coef[base+off+k] = tmp[k] * scale
					}
				}
			default:
				// Books 1..11.
				book := b
				if book < 1 || book > 11 {
					return nil, fmt.Errorf("%w: band book %d", ErrBadADTS, book)
				}
				// Grouping shares scalefactors, not spectra: each window in
				// the group carries its own Huffman data (ffmpeg consumes
				// fresh VLC words per group window).
				for w := 0; w < gLen; w++ {
					base := winOff + w*128
					if ics.windowSeq[0] != 2 {
						base = 0
					}
					if err := decodeSpectrumBlock(r, book, coef, base+off, length, sf[idx], random); err != nil {
						return nil, err
					}
				}
			}
		}
		winOff += gLen * 128
		if ics.windowSeq[0] != 2 {
			winOff = 0
		}
	}
	// Pulse applies after spectrum (ffmpeg applies it inside
	// decode_spectrum_and_dequant from the stored pulse data).
	for i := 0; i < numPulse; i++ {
		pos := pulsePos[i]
		amp := pulseAmp[i]
		// Find band for scale.
		band := -1
		for s := 0; s < ics.maxSfb; s++ {
			if pos >= int(ics.swbOffset[s]) && pos < int(ics.swbOffset[s+1]) {
				band = s
				break
			}
		}
		if band < 0 || bt[band] == btNoise || sf[band] == 0 {
			continue
		}
		co := coef[pos]
		if co != 0 {
			// Peers the float path of ffmpeg's pulse block:
			// mag grows by amp either way; negatives stay
			// negative after the 4/3 reconstruction.
			q := co / sf[band]
			neg := q < 0
			aq := math.Abs(q)
			mag := aq/math.Sqrt(math.Sqrt(aq)) + float64(amp)
			nv := math.Cbrt(mag) * mag * sf[band]
			if neg {
				nv = -nv
			}
			coef[pos] = nv
		} else {
			coef[pos] = -float64(amp)
		}
	}
	return &singleResult{coef: coef, ics: *ics, bt: bt, sfo: sfo, sf: sf, tns: tns, hasTNS: hasTNS}, nil
}

// applyPulseAdjust is unused (pulse is inline above).
