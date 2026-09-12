package h264

import "fmt"

// Scaling lists (F9): per-list quantization weights parsed from the
// sequence/picture sets and resolved per slice. Positions follow the
// bitstream order: 0..5 are 4x4 (intra-Y/Cb/Cr, inter-Y/Cb/Cr),
// 6..7 are 8x8 luma (intra-Y, inter-Y); 4:4:4-only 8..11 are consumed
// and dropped. Only the numbers travel; fallback chains mirror the
// reference decoder's decode_scaling_matrices flow.

// Default (JVT) 4x4 lists in raster order: intra then inter.
var defaultScaling4 = [2][16]uint8{
	{6, 13, 20, 28, 13, 20, 28, 32, 20, 28, 32, 37, 28, 32, 37, 42},
	{10, 14, 20, 24, 14, 20, 24, 27, 20, 24, 27, 30, 24, 27, 30, 34},
}

// Default (JVT) 8x8 lists in raster order: intra then inter.
var defaultScaling8 = [2][64]uint8{
	{
		6, 10, 13, 16, 18, 23, 25, 27,
		10, 11, 16, 18, 23, 25, 27, 29,
		13, 16, 18, 23, 25, 27, 29, 31,
		16, 18, 23, 25, 27, 29, 31, 33,
		18, 23, 25, 27, 29, 31, 33, 36,
		23, 25, 27, 29, 31, 33, 36, 38,
		25, 27, 29, 31, 33, 36, 38, 40,
		27, 29, 31, 33, 36, 38, 40, 42,
	},
	{
		9, 13, 15, 17, 19, 21, 22, 24,
		13, 13, 17, 19, 21, 22, 24, 25,
		15, 17, 19, 21, 22, 24, 25, 27,
		17, 19, 21, 22, 24, 25, 27, 28,
		19, 21, 22, 24, 25, 27, 28, 30,
		21, 22, 24, 25, 27, 28, 30, 32,
		22, 24, 25, 27, 28, 30, 32, 33,
		24, 25, 27, 28, 30, 32, 33, 35,
	},
}

// scalingRaw holds one parameter set's decoded lists in raster order.
// Mask bits mark explicit lists (a default-escape counts as explicit:
// it already resolved to JVT at parse time).
type scalingRaw struct {
	present bool
	l4      [6][16]uint8
	m4      uint8
	l8      [2][64]uint8
	m8      uint8
}

// scalingJVT4 selects the default 4x4 list: intra for 0..2, inter for 3..5.
func scalingJVT4(i int) [16]uint8 {
	if i < 3 {
		return defaultScaling4[0]
	}
	return defaultScaling4[1]
}

// scalingJVT8 selects the default 8x8 list: intra-Y (6) or inter-Y (7;
// 4:4:4 chroma positions map the same way and are dropped).
func scalingJVT8(i int) [64]uint8 {
	if i == 6 || i == 8 || i == 10 {
		return defaultScaling8[0]
	}
	return defaultScaling8[1]
}

// parseScalingList decodes one scaling_list() into raster order.
// Bitstream order is zigzag in both sizes (4x4 and 8x8 tables). A
// leading zero (first delta -8) escapes to the default list.
func parseScalingList(r *Reader, size int, jvt []uint8) ([]uint8, error) {
	out := make([]uint8, size)
	scan := zigzag4x4[:]
	if size == 64 {
		scan = zigzag8x8[:]
	}
	last, next := int32(8), int32(8)
	for i := 0; i < size; i++ {
		if next != 0 {
			d, err := r.ReadSE()
			if err != nil {
				return nil, fmt.Errorf("%w: scaling delta: %v", ErrBadSPS, err)
			}
			if d < -128 || d > 127 {
				return nil, fmt.Errorf("%w: scaling delta %d", ErrBadSPS, d)
			}
			next = (last + d + 256) % 256
		}
		if i == 0 && next == 0 {
			return append([]uint8(nil), jvt...), nil
		}
		if next != 0 {
			last = next
		}
		out[scan[i]] = uint8(last)
	}
	return out, nil
}

// parseScalingMatrices consumes n scaling lists into raw (positions
// 0..5 4x4, 6..7 8x8 luma, 8..11 4:4:4-only 8x8 chroma dropped).
func parseScalingMatrices(r *Reader, raw *scalingRaw, n int) error {
	for i := 0; i < n; i++ {
		present, err := r.ReadBits(1)
		if err != nil {
			return fmt.Errorf("%w: scaling %d present: %v", ErrBadSPS, i, err)
		}
		if present == 0 {
			continue
		}
		if i < 6 {
			jvt := scalingJVT4(i)
			vals, err := parseScalingList(r, 16, jvt[:])
			if err != nil {
				return err
			}
			copy(raw.l4[i][:], vals)
			raw.m4 |= 1 << uint(i)
			continue
		}
		jvt := scalingJVT8(i)
		vals, err := parseScalingList(r, 64, jvt[:])
		if err != nil {
			return err
		}
		switch i {
		case 6:
			copy(raw.l8[0][:], vals)
			raw.m8 |= 1
		case 7:
			copy(raw.l8[1][:], vals)
			raw.m8 |= 2
		default:
			// 4:4:4-only chroma 8x8: consumed, dropped.
		}
	}
	return nil
}

// resolveScaling merges picture/sequence lists into effective weights:
// explicit picture lists win, then chained neighbours (chroma follows
// the just-resolved luma/inter list), then sequence lists, then the
// default or flat fallback depending on whether any matrix exists.
func resolveScaling(pps *PPS, sps *SPS, sc4 *[6][16]uint8, sc8 *[2][64]uint8) {
	flat4 := [16]uint8{16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16}
	var flat8 [64]uint8
	for i := range flat8 {
		flat8[i] = 16
	}
	var sps4 [6][16]uint8
	var sps8 [2][64]uint8
	// An absent sequence list means JVT whenever any matrix exists
	// (picture-present falls back to JVT, not flat).
	spsJVT := (sps != nil && sps.Scaling.present) || (pps != nil && pps.Scaling.present)
	for i := 0; i < 6; i++ {
		sps4[i] = flat4
		if sps != nil && sps.Scaling.m4>>uint(i)&1 != 0 {
			sps4[i] = sps.Scaling.l4[i]
		} else if spsJVT {
			sps4[i] = scalingJVT4(i)
		}
	}
	for j := 0; j < 2; j++ {
		sps8[j] = flat8
		if sps != nil && sps.Scaling.m8>>uint(j)&1 != 0 {
			sps8[j] = sps.Scaling.l8[j]
		} else if spsJVT {
			sps8[j] = defaultScaling8[j]
		}
	}
	if pps == nil || !pps.Scaling.present {
		*sc4 = sps4
		*sc8 = sps8
		return
	}
	exp4 := func(i int) ([16]uint8, bool) {
		if pps.Scaling.m4>>uint(i)&1 != 0 {
			return pps.Scaling.l4[i], true
		}
		return flat4, false
	}
	if v, ok := exp4(0); ok {
		sc4[0] = v
	} else {
		sc4[0] = sps4[0]
	}
	if v, ok := exp4(1); ok {
		sc4[1] = v
	} else {
		sc4[1] = sc4[0]
	}
	if v, ok := exp4(2); ok {
		sc4[2] = v
	} else {
		sc4[2] = sc4[1]
	}
	if v, ok := exp4(3); ok {
		sc4[3] = v
	} else {
		sc4[3] = sps4[3]
	}
	if v, ok := exp4(4); ok {
		sc4[4] = v
	} else {
		sc4[4] = sc4[3]
	}
	if v, ok := exp4(5); ok {
		sc4[5] = v
	} else {
		sc4[5] = sc4[4]
	}
	if pps.Scaling.m8&1 != 0 {
		sc8[0] = pps.Scaling.l8[0]
	} else {
		sc8[0] = sps8[0]
	}
	if pps.Scaling.m8&2 != 0 {
		sc8[1] = pps.Scaling.l8[1]
	} else {
		sc8[1] = sps8[1]
	}
}
