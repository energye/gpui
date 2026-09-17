package h265

import (
	"fmt"
	"math/bits"
)

// Binary arithmetic decoder (HEVC entropy path; I slices in this clip
// use it for every CU/split/mode/coefficient decision).
// Same standard engine shape as our H.264 decoder (16-bit low/range
// core with byte refills; packed states: bit0 = MPS, bits 6:1 =
// probability index; numbers from cabac_tables.go); control flow is
// written fresh for HEVC.
// Peer (read-only): libavcodec/cabac_functions.h get_cabac family
// (renorm/bypass/terminate order) + hevc/cabac.c:430 cabac_init_state
// (initType + m/n formula). Only ideas travel.
type cabacDec struct {
	low   int32
	rng   int32
	buf   []byte
	pos   int
	end   int
	nbits int
}

// newCabacDec starts the decoder at buf[0]. The buffer must hold the
// slice payload tail plus a few zero pad bytes for the standard
// overread. Payloads start byte-aligned, the aligned-init case.
func newCabacDec(buf []byte) (*cabacDec, error) {
	if len(buf) < 2 {
		return nil, fmt.Errorf("%w: cabac needs 2 bytes", ErrBadSlice)
	}
	c := &cabacDec{buf: buf, end: len(buf)}
	c.low = int32(buf[0])<<18 | int32(buf[1])<<10
	c.pos = 2
	if len(buf) > 2 {
		c.low += int32(buf[2])<<2 + 2
		c.pos = 3
	} else {
		c.low += 1 << 9
	}
	c.rng = 0x1FE
	if c.rng<<17 < c.low {
		return nil, fmt.Errorf("%w: cabac init offset out of range", ErrBadSlice)
	}
	return c, nil
}

// consumedBits reports stream bits spent so far (for the trailing check).
func (c *cabacDec) consumedBits() int { return c.nbits }

// refill loads two fresh bytes after the low window empties.
func (c *cabacDec) refill() {
	var b0, b1 int32
	if c.pos < len(c.buf) {
		b0 = int32(c.buf[c.pos])
	}
	if c.pos+1 < len(c.buf) {
		b1 = int32(c.buf[c.pos+1])
	}
	c.low += b0<<9 | b1<<1
	c.low -= 0xFFFF
	if c.pos < c.end {
		c.pos += 2
	}
}

// refill2 folds two fresh bytes into a partially-used window.
func (c *cabacDec) refill2() {
	i := bits.TrailingZeros32(uint32(c.low)) - 16
	var b0, b1 uint32
	if c.pos < len(c.buf) {
		b0 = uint32(c.buf[c.pos])
	}
	if c.pos+1 < len(c.buf) {
		b1 = uint32(c.buf[c.pos+1])
	}
	x := uint32(0xFFFF0001) + b0<<9 + b1<<1
	c.low += int32(x << uint(i))
	if c.pos < c.end {
		c.pos += 2
	}
}

// bin decodes one bin with the packed context state.
func (c *cabacDec) bin(st *uint8) int {
	s := int(*st)
	lps := int(uint8(cabacLPSRange[2*(int(c.rng)&0xC0)+s]))
	c.rng -= int32(lps)
	mask := (c.rng<<17 - c.low) >> 31
	c.low -= (c.rng << 17) & mask
	c.rng += int32(lps-int(c.rng)) & mask
	s ^= int(mask)
	*st = cabacMLPSState[128+s]
	bit := s & 1
	shift := uint(cabacNormShift[c.rng])
	c.rng <<= shift
	c.low <<= shift
	c.nbits += int(shift)
	if c.low&0xFFFF == 0 {
		c.refill2()
	}
	return bit
}

// bypass decodes one equiprobable bin.
func (c *cabacDec) bypass() int {
	c.low += c.low
	c.nbits++
	if c.low&0xFFFF == 0 {
		c.refill()
	}
	if c.low < c.rng<<17 {
		return 0
	}
	c.low -= c.rng << 17
	return 1
}

// bypassBins reads n equiprobable bins as an unsigned value.
func (c *cabacDec) bypassBins(n int) (uint32, error) {
	if n < 0 || n > 32 {
		return 0, fmt.Errorf("%w: bad bypass count %d", ErrBadSlice, n)
	}
	var v uint32
	for i := 0; i < n; i++ {
		v = (v << 1) | uint32(c.bypass())
	}
	return v, nil
}

// terminate decodes the end-of-slice flag: 0 continues, 1 ends.
func (c *cabacDec) terminate() int {
	c.rng -= 2
	if c.low >= c.rng<<17 {
		return 1
	}
	if c.rng < 0x100 {
		c.rng <<= 1
		c.low <<= 1
		c.nbits++
	}
	if c.low&0xFFFF == 0 {
		c.refill()
	}
	return 0
}

// initCtx fills the 199 packed states for the slice: initType follows
// the slice type (I=0, P=1, B=2, flipped by cabac_init), each state
// from the initValue m/n formula at the clipped slice QP.
func initCtx(state *[199]uint8, sliceType uint32, cabacInit bool, sliceQP int32) {
	initType := int(2 - sliceType)
	if cabacInit && sliceType != SliceI {
		initType ^= 3
	}
	qp := sliceQP
	if qp < 0 {
		qp = 0
	}
	if qp > 51 {
		qp = 51
	}
	for i := 0; i < 199; i++ {
		v := int(cabacInitValue[initType][i])
		m := (v>>4)*5 - 45
		n := ((v & 15) << 3) - 16
		pre := 2*(((m*int(qp))>>4)+n) - 127
		pre ^= pre >> 31
		if pre > 124 {
			pre = 124 + (pre & 1)
		}
		state[i] = uint8(pre)
	}
}
