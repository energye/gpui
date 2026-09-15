package aac

import (
	"fmt"
)

// frameOut holds one decoded raw_data_block's PCM (per channel).
type frameOut struct {
	channels [][]float64 // per channel 1024 samples (long) or 1024 (short mapped)
}

// decoderState holds per-decoder overlap and history.
type decoderState struct {
	prevSeq [2]int
	prevKb  [2]bool
	overlap [2][]float64
	random  uint32
	init    bool
}

func newDecoderState() *decoderState {
	return &decoderState{
		random:  0x1f2e3d4c,
		overlap: [2][]float64{make([]float64, 1024), make([]float64, 1024)},
	}
}

// decodeRawBlock parses one raw_data_block (entire packet) for stereo LC.
// It peers decode_frame_ga (aacdec.c) for element loop; SCE/CPE/DSE/FIL
// are wired, others report honest errors.
func decodeRawBlock(r *bitReaderMSB, cfg *Config, st *decoderState) (*frameOut, error) {
	samplingIdx := cfg.SamplingIndex
	frameLen := cfg.FrameLength
	if frameLen != 1024 {
		return nil, fmt.Errorf("aac: frame %d pending: %w", frameLen, ErrUnsupported)
	}
	var left, right []float64
	var icsL, icsR icsInfo
	channels := cfg.Channels
	if channels != 2 {
		return nil, fmt.Errorf("aac: %d ch pending: %w", channels, ErrUnsupported)
	}
	// Element loop.
	gotAudio := false
	for {
		if r.left() < 3 {
			return nil, fmt.Errorf("%w: element header", ErrTruncated)
		}
		elemType, err := r.read(3)
		if err != nil {
			return nil, err
		}
		if elemType == 7 { // END
			break
		}
		if elemType == 6 { // FIL: the 4 bits after the type ARE the
			// byte count (no instance tag). Peers ffmpeg's
			// TYPE_FIL branch (elem_id doubles as count).
			if err := skipFIL(r); err != nil {
				return nil, err
			}
			if r.left() < 3 {
				continue
			}
			continue
		}
		elemID, err := r.read(4)
		if err != nil {
			return nil, err
		}
		_ = elemID
		switch elemType {
		case 0: // SCE (mono; stereo clips should not hit, but support)
			ics, err := parseICSInfo(r, samplingIdx, frameLen, st.prevSeq[0], st.prevKb[0])
			if err != nil {
				return nil, err
			}
			single, err := decodeSingle(r, &ics, samplingIdx, frameLen, &st.random)
			if err != nil {
				return nil, err
			}
			pcm, newOverlap, err := imdctChannel(single.coef, st.overlap[0], &ics)
			if err != nil {
				return nil, err
			}
			left = pcm
			st.overlap[0] = newOverlap
			st.prevSeq[0] = ics.windowSeq[0]
			st.prevKb[0] = ics.useKbWindow[0]
			icsL = ics
			_ = icsR
			gotAudio = true
		case 1: // CPE
			cpe, newSeq, newKb, err := decodeCPE(r, samplingIdx, frameLen, st.prevSeq, st.prevKb, &st.random)
			if err != nil {
				return nil, err
			}
			pcmL, ovL, err := imdctChannel(cpe.left.coef, st.overlap[0], &cpe.left.ics)
			if err != nil {
				return nil, err
			}
			pcmR, ovR, err := imdctChannel(cpe.right.coef, st.overlap[1], &cpe.right.ics)
			if err != nil {
				return nil, err
			}
			left, right = pcmL, pcmR
			st.overlap[0], st.overlap[1] = ovL, ovR
			st.prevSeq, st.prevKb = newSeq, newKb
			icsL = cpe.left.ics
			icsR = cpe.right.ics
			gotAudio = true
		case 4: // DSE: skip
			if err := skipDSE(r); err != nil {
				return nil, err
			}
		case 6: // FIL (unreachable: handled above before elemID)
			return nil, fmt.Errorf("%w: internal FIL", ErrBadADTS)
		case 2, 3, 5:
			return nil, fmt.Errorf("aac: element %d pending: %w", elemType, ErrUnsupported)
		default:
			return nil, fmt.Errorf("%w: element %d", ErrBadADTS, elemType)
		}
		if r.left() < 3 {
			// Trailing bits must still end with END; loop will catch.
			continue
		}
	}
	if !gotAudio {
		return nil, fmt.Errorf("%w: no audio element", ErrBadADTS)
	}
	if left == nil || right == nil {
		return nil, fmt.Errorf("%w: stereo incomplete", ErrBadADTS)
	}
	_ = icsL
	_ = icsR
	return &frameOut{channels: [][]float64{left, right}}, nil
}

func skipDSE(r *bitReaderMSB) error {
	align, err := r.read(1)
	if err != nil {
		return err
	}
	count, err := r.read(8)
	if err != nil {
		return err
	}
	if count == 255 {
		extra, err := r.read(8)
		if err != nil {
			return err
		}
		count += extra
	}
	if align == 1 {
		// Byte-align from current position.
		skip := (8 - (r.pos % 8)) % 8
		if err := r.skip(skip); err != nil {
			return err
		}
	}
	return r.skip(count * 8)
}

func skipFIL(r *bitReaderMSB) error {
	count, err := r.read(4)
	if err != nil {
		return err
	}
	if count == 15 {
		extra, err := r.read(8)
		if err != nil {
			return err
		}
		count += extra - 1
	}
	return r.skip(count * 8)
}

// imdctChannel runs IMDCT+windowing for one channel's spectra. ics
// carries both window sequences and shapes; saved is the overlap state,
// updated in place. All four sequences are supported.
func imdctChannel(coef, saved []float64, ics *icsInfo) ([]float64, []float64, error) {
	return imdctAndWindowing1024(coef, saved, ics), saved, nil
}
