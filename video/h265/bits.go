package h265

import "fmt"

// Reader reads bits MSB-first out of an RBSP buffer.
//
// Idea follows ffmpeg's GetBitContext (get_bits.h) and the Exp-Golomb
// helpers (golomb.h); the code itself is written from scratch for HEVC.
// Peer (read-only): libavcodec/get_bits.h + golomb.h + hevc/ps.c
// ff_hevc_decode_nal_vps/sps/pps (bit order, ue/se, trailing check).
type Reader struct {
	data []byte
	pos  int
}

// NewReader builds a bit reader over raw RBSP bytes.
func NewReader(data []byte) *Reader { return &Reader{data: data} }

// BitsLeft reports unread bits.
func (r *Reader) BitsLeft() int { return len(r.data)*8 - r.pos }

// ReadBit reads one bit.
func (r *Reader) ReadBit() (uint32, error) {
	if r.pos >= len(r.data)*8 {
		return 0, fmt.Errorf("%w: want 1 bit, have 0", ErrTruncated)
	}
	b := (r.data[r.pos/8] >> uint(7-(r.pos%8))) & 1
	r.pos++
	return uint32(b), nil
}

// ReadBits reads n bits (0 < n <= 32) as an unsigned value.
func (r *Reader) ReadBits(n int) (uint32, error) {
	if n <= 0 || n > 32 {
		return 0, fmt.Errorf("%w: bad bit count %d", ErrBadNALU, n)
	}
	var v uint32
	for i := 0; i < n; i++ {
		b, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		v = (v << 1) | b
	}
	return v, nil
}

// ReadUE reads an unsigned Exp-Golomb code ue(v).
func (r *Reader) ReadUE() (uint32, error) {
	zeros := 0
	for {
		b, err := r.ReadBit()
		if err != nil {
			return 0, err
		}
		if b == 1 {
			break
		}
		zeros++
		if zeros > 31 {
			return 0, fmt.Errorf("%w: ue overflow", ErrBadNALU)
		}
	}
	if zeros == 0 {
		return 0, nil
	}
	rest, err := r.ReadBits(zeros)
	if err != nil {
		return 0, err
	}
	return (1<<uint(zeros) - 1) + rest, nil
}

// ReadSE reads a signed Exp-Golomb code se(v).
func (r *Reader) ReadSE() (int32, error) {
	v, err := r.ReadUE()
	if err != nil {
		return 0, err
	}
	if v%2 == 0 {
		return -int32(v / 2), nil
	}
	return int32((v + 1) / 2), nil
}

// MoreRBSPData reports whether real data remains past the reader: false
// only when the rest is exactly rbsp_trailing_bits (stop bit plus
// alignment zeros) or nothing. Same rule as the H.264 reader.
func (r *Reader) MoreRBSPData() bool {
	left := r.BitsLeft()
	if left <= 0 {
		return false
	}
	if left > 8 {
		return true
	}
	if r.bitAt(r.pos) != 1 {
		return true
	}
	for i := r.pos + 1; i < len(r.data)*8; i++ {
		if r.bitAt(i) != 0 {
			return true
		}
	}
	return false
}

func (r *Reader) bitAt(p int) uint32 { return uint32((r.data[p/8] >> uint(7-(p%8))) & 1) }

// UnescapeRBSP removes emulation_prevention_three_byte sequences
// (00 00 03 -> 00 00) so bit reading sees the true RBSP.
func UnescapeRBSP(in []byte) []byte {
	out := make([]byte, 0, len(in))
	for i := 0; i < len(in); i++ {
		if i+2 < len(in) && in[i] == 0 && in[i+1] == 0 && in[i+2] == 3 {
			out = append(out, 0, 0)
			i += 2
			continue
		}
		out = append(out, in[i])
	}
	return out
}

// nalHeader2 splits the 2-byte HEVC header: forbidden bit, type,
// layer id, temporal id. Peer: hevcdec + parse.c header triage.
func nalHeader2(nalu []byte) (typ int, layer int, temporal int, err error) {
	if len(nalu) < 2 {
		return 0, 0, 0, fmt.Errorf("%w: empty unit", ErrBadNALU)
	}
	if nalu[0]&0x80 != 0 {
		return 0, 0, 0, fmt.Errorf("%w: forbidden bit set", ErrBadNALU)
	}
	typ = int((nalu[0] >> 1) & 0x3F)
	layer = int(((nalu[0] & 0x01) << 5) | ((nalu[1] >> 3) & 0x1F))
	temporal = int(nalu[1]&0x07) - 1
	if nalu[1]&0x07 == 0 {
		return 0, 0, 0, fmt.Errorf("%w: bad temporal id", ErrBadNALU)
	}
	return typ, layer, temporal, nil
}
