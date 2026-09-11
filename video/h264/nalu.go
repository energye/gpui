package h264

import "fmt"

// SplitAnnexB cuts an Annex B byte stream (start codes 00 00 01 or
// 00 00 00 01) into raw NALUs without the start codes.
// Empty units are skipped; trailing cabac_zero_word zeros are ignored.
// At least one non-empty unit is required.
func SplitAnnexB(stream []byte) ([][]byte, error) {
	var starts []int
	i := 0
	for i+2 < len(stream) {
		if stream[i] == 0 && stream[i+1] == 0 {
			if stream[i+2] == 1 {
				starts = append(starts, i)
				i += 3
				continue
			}
			if i+3 < len(stream) && stream[i+2] == 0 && stream[i+3] == 1 {
				starts = append(starts, i)
				i += 4
				continue
			}
		}
		i++
	}
	if len(starts) == 0 {
		return nil, fmt.Errorf("%w: no start code", ErrNoNALU)
	}
	var out [][]byte
	for k, s := range starts {
		codeLen := 3
		if s+3 < len(stream) && stream[s] == 0 && stream[s+1] == 0 && stream[s+2] == 0 {
			codeLen = 4
		}
		end := len(stream)
		if k+1 < len(starts) {
			end = starts[k+1]
		}
		unit := stream[s+codeLen : end]
		for len(unit) > 0 && unit[len(unit)-1] == 0 {
			unit = unit[:len(unit)-1]
		}
		if len(unit) == 0 {
			continue
		}
		cp := append([]byte(nil), unit...)
		out = append(out, cp)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: only empty units", ErrNoNALU)
	}
	return out, nil
}

// SplitAVCC cuts length-prefixed NALUs (MP4 sample payload) using the
// length field size from the avcC box (1, 2 or 4 bytes).
func SplitAVCC(sample []byte, lengthSize int) ([][]byte, error) {
	if lengthSize != 1 && lengthSize != 2 && lengthSize != 4 {
		return nil, fmt.Errorf("%w: length size %d", ErrBadNALU, lengthSize)
	}
	var out [][]byte
	off := 0
	for off < len(sample) {
		if len(sample)-off < lengthSize {
			return nil, fmt.Errorf("%w: short length at %d", ErrTruncated, off)
		}
		var n int
		switch lengthSize {
		case 1:
			n = int(sample[off])
		case 2:
			n = int(sample[off])<<8 | int(sample[off+1])
		default:
			n = int(sample[off])<<24 | int(sample[off+1])<<16 | int(sample[off+2])<<8 | int(sample[off+3])
		}
		off += lengthSize
		if n <= 0 {
			return nil, fmt.Errorf("%w: zero length at %d", ErrBadNALU, off)
		}
		if len(sample)-off < n {
			return nil, fmt.Errorf("%w: unit %d wants %d, have %d", ErrTruncated, len(out)+1, n, len(sample)-off)
		}
		cp := append([]byte(nil), sample[off:off+n]...)
		out = append(out, cp)
		off += n
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w: empty sample", ErrNoNALU)
	}
	return out, nil
}

// NALUHeader splits the one-byte header into its fields.
func NALUHeader(nalu []byte) (forbidden bool, refIDC int, typ int, err error) {
	if len(nalu) == 0 {
		return false, 0, 0, fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	h := nalu[0]
	return h&0x80 != 0, int((h >> 5) & 0x03), int(h & 0x1F), nil
}

// FirstMBInSlice reads first_mb_in_slice (the first ue(v) of a slice
// header for types 1 and 5) so frame boundaries can be found without a
// full slice decode.
func FirstMBInSlice(nalu []byte) (uint32, error) {
	t, ok := NALType(nalu)
	if !ok {
		return 0, fmt.Errorf("%w: empty unit", ErrBadSliceHeader)
	}
	if t != NALSliceNonIDR && t != NALSliceIDR {
		return 0, fmt.Errorf("%w: type %d is not a base slice", ErrBadSliceHeader, t)
	}
	r := NewReader(UnescapeRBSP(nalu[1:]))
	v, err := r.ReadUE()
	if err != nil {
		return 0, fmt.Errorf("%w: first_mb_in_slice: %v", ErrBadSliceHeader, err)
	}
	return v, nil
}
