package aac

import "fmt"

// swbOffset returns the scalefactor-band offsets for long windows.
// frameLen is 1024 or 960; samplingIdx is 0..12.
func swbOffsetLong(frameLen, samplingIdx int) ([]uint16, error) {
	if samplingIdx < 0 || samplingIdx > 12 {
		return nil, fmt.Errorf("%w: sampling idx %d", ErrBadASC, samplingIdx)
	}
	if frameLen == 1024 {
		switch {
		case samplingIdx <= 1:
			return swb_swb_offset_1024_96[:], nil
		case samplingIdx == 2:
			return swb_swb_offset_1024_64[:], nil
		case samplingIdx <= 4:
			return swb_swb_offset_1024_48[:], nil
		case samplingIdx == 5:
			return swb_swb_offset_1024_32[:], nil
		case samplingIdx <= 7:
			return swb_swb_offset_1024_24[:], nil
		case samplingIdx <= 10:
			return swb_swb_offset_1024_16[:], nil
		default:
			return swb_swb_offset_1024_8[:], nil
		}
	}
	if frameLen == 960 {
		switch {
		case samplingIdx <= 1:
			return swb_swb_offset_960_96[:], nil
		case samplingIdx == 2:
			return swb_swb_offset_960_64[:], nil
		case samplingIdx <= 4:
			return swb_swb_offset_960_48[:], nil
		case samplingIdx == 5:
			return swb_swb_offset_960_48[:], nil
		case samplingIdx <= 7:
			return swb_swb_offset_960_24[:], nil
		case samplingIdx <= 10:
			return swb_swb_offset_960_16[:], nil
		default:
			return swb_swb_offset_960_8[:], nil
		}
	}
	return nil, fmt.Errorf("%w: frame len %d", ErrUnsupported, frameLen)
}

// swbOffsetShort returns offsets for eight-short windows.
func swbOffsetShort(frameLen, samplingIdx int) ([]uint16, error) {
	if samplingIdx < 0 || samplingIdx > 12 {
		return nil, fmt.Errorf("%w: sampling idx %d", ErrBadASC, samplingIdx)
	}
	if frameLen == 1024 {
		switch {
		case samplingIdx <= 2:
			return swb_swb_offset_128_96[:], nil
		case samplingIdx <= 5:
			return swb_swb_offset_128_48[:], nil
		case samplingIdx <= 7:
			return swb_swb_offset_128_24[:], nil
		case samplingIdx <= 10:
			return swb_swb_offset_128_16[:], nil
		default:
			return swb_swb_offset_128_8[:], nil
		}
	}
	if frameLen == 960 {
		switch {
		case samplingIdx <= 2:
			return swb_swb_offset_120_96[:], nil
		case samplingIdx <= 5:
			return swb_swb_offset_120_48[:], nil
		case samplingIdx <= 7:
			return swb_swb_offset_120_24[:], nil
		case samplingIdx <= 10:
			return swb_swb_offset_120_16[:], nil
		default:
			return swb_swb_offset_120_8[:], nil
		}
	}
	return nil, fmt.Errorf("%w: frame len %d", ErrUnsupported, frameLen)
}

// numSwbLong returns the band count for long windows.
func numSwbLong(frameLen, samplingIdx int) (int, error) {
	if samplingIdx < 0 || samplingIdx > 12 {
		return 0, fmt.Errorf("%w: sampling idx %d", ErrBadASC, samplingIdx)
	}
	if frameLen == 1024 {
		return int(numSwb_num_swb_1024[samplingIdx]), nil
	}
	if frameLen == 960 {
		return int(numSwb_num_swb_960[samplingIdx]), nil
	}
	return 0, fmt.Errorf("%w: frame len %d", ErrUnsupported, frameLen)
}

// numSwbShort returns the band count for short windows.
func numSwbShort(frameLen, samplingIdx int) (int, error) {
	if samplingIdx < 0 || samplingIdx > 12 {
		return 0, fmt.Errorf("%w: sampling idx %d", ErrBadASC, samplingIdx)
	}
	if frameLen == 1024 {
		return int(numSwb_num_swb_128[samplingIdx]), nil
	}
	if frameLen == 960 {
		// 960 short uses 120 tables; count table for 96/120 family:
		// ff_aac_num_swb_96 covers 96-length; use 120 count via offsets.
		// Fall back to offset length - 1.
		off, err := swbOffsetShort(frameLen, samplingIdx)
		if err != nil {
			return 0, err
		}
		return len(off) - 1, nil
	}
	return 0, fmt.Errorf("%w: frame len %d", ErrUnsupported, frameLen)
}
