package h264

import "fmt"

// Reader reads bits MSB-first out of an RBSP buffer.
// Idea follows ffmpeg's GetBitContext (get_bits.h) and the Exp-Golomb
// helpers (golomb.h); the code itself is written from scratch.
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
		return 0, fmt.Errorf("%w: bad bit count %d", ErrBadSPS, n)
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
			return 0, fmt.Errorf("%w: ue overflow", ErrBadSPS)
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

// MoreRBSPData is a small heuristic for trailing optional fields: only a
// stop bit plus alignment (<= 8 bits) means nothing more follows. Callers
// use it before PPS extension flags. Exact stop-bit walking is overkill
// for VR1; the rule is documented and covered by tests both ways.
func (r *Reader) MoreRBSPData() bool { return r.BitsLeft() > 8 }

// AlignToByte skips to the next byte boundary (rbsp alignment).
func (r *Reader) AlignToByte() {
	for r.pos%8 != 0 {
		r.pos++
	}
}

// ReadBytes reads n whole bytes; the reader must be byte-aligned.
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if r.pos%8 != 0 {
		return nil, fmt.Errorf("unaligned byte read")
	}
	if n < 0 || r.pos/8+n > len(r.data) {
		return nil, fmt.Errorf("byte read past end")
	}
	out := append([]byte(nil), r.data[r.pos/8:r.pos/8+n]...)
	r.pos += n * 8
	return out, nil
}

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
