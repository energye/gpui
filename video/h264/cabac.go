package h264

import (
	"fmt"
	"math/bits"
)

// Binary arithmetic decoder (the high-compression entropy path, used by
// Main/High档). 16-bit low/range core with byte refills; context states
// are packed bytes (bit0 = MPS, bits 6:1 = probability index). Numbers
// come from cabac_tables.go, all control flow is written from scratch.

// cabacDec decodes bins from a byte-aligned RBSP tail.
type cabacDec struct {
	low   int32
	rng   int32
	buf   []byte
	pos   int
	end   int
	nbits int
}

// consumedBytes reports whole bytes used so far, for byte-align sync
// (CABAC PCM path): every renorm shift and bypass step spends stream
// bits, so the count is exact.
func (c *cabacDec) consumedBytes() int { return (c.nbits + 7) / 8 }

// newCabacDec starts the decoder at buf[0]. The buffer must hold the
// slice payload tail plus a few zero pad bytes for the standard overread.
func newCabacDec(buf []byte) (*cabacDec, error) {
	if len(buf) < 2 {
		return nil, fmt.Errorf("%w: cabac init needs 2 bytes", ErrBadSliceHeader)
	}
	c := &cabacDec{buf: buf, end: len(buf)}
	c.low = int32(buf[0])<<18 | int32(buf[1])<<10
	c.pos = 2
	// Slice payloads start byte-aligned on fresh buffers, which is the
	// aligned-init case the reference always takes in practice.
	if len(buf) > 2 {
		c.low += int32(buf[2])<<2 + 2
		c.pos = 3
	} else {
		c.low += 1 << 9
	}
	c.rng = 0x1FE
	if c.rng<<17 < c.low {
		return nil, fmt.Errorf("%w: cabac init offset out of range", ErrBadSliceHeader)
	}
	return c, nil
}

// refill loads two fresh bytes after the low window empties.
// Missing tail bytes read as zero so truncated streams error out
// downstream instead of panicking here.
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

// refill2 folds two fresh bytes into a partially-used window, carrying
// the outstanding bits (ctz finds how far the window has shifted).
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
	// Table bytes at and above 128 wrap negative in storage; the LPS
	// range is always an unsigned magnitude.
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
	var v int
	if c.low < c.rng<<17 {
		v = 0
	} else {
		c.low -= c.rng << 17
		v = 1
	}
	return v
}

// bypassSign maps one bypass bin onto +/-v.
func (c *cabacDec) bypassSign(v int32) int32 {
	c.low += c.low
	c.nbits++
	if c.low&0xFFFF == 0 {
		c.refill()
	}
	r := c.rng << 17
	c.low -= r
	mask := c.low >> 31
	r &= mask
	c.low += r
	v = (v ^ mask) - mask
	return v
}

// terminate decodes the end-of-slice flag: 1 ends the slice.
func (c *cabacDec) terminate() int {
	c.rng -= 2
	end := c.low >= c.rng<<17
	if !end {
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
	return 1
}

// initCabacCtx fills 1024 packed states from one (m,n) table at the slice QP.
func initCabacCtx(state *[1024]uint8, tab []int8, sliceQP int32) {
	for i := 0; i < 1024; i++ {
		m := int32(tab[2*i])
		n := int32(tab[2*i+1])
		pre := 2*(((m*sliceQP)>>4)+n) - 127
		pre ^= pre >> 31
		if pre > 124 {
			pre = 124 + (pre & 1)
		}
		state[i] = uint8(pre)
	}
}
