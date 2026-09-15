package h264

import (
	"math/bits"
	"os"
)

// S3 CABAC fast path, step 2 (residual full-block batch: map + levels,
// scalar留守).
//
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/cabac_functions.h:114-138 get_cabac_inline (C branchless
//   core: LPSRange + MLPSState + NormShift tables, mask-selected MPS/LPS
//   path; our batch below reuses the same tables and the same per-bin
//   order, only hoisting low/range/nbits into registers across the
//   block's bins) against video/h264/cabac.go:bin;
//   libavcodec/x86/cabac.h:110-213 BRANCHLESS_GET_CABAC (asm keeps
//   low/range in registers across bins with raw table pointers; our
//   batch is the pure-Go shape of the same idea: one call per block
//   instead of one call per bin, fields in locals, refill still exact)
//   + :260-343 bypass/sign asm (our batch inlines the same bypass/sign
//   shape with locals; levels prefix included since step 2);
//   libavcodec/aarch64/cabac.h:30-100 get_cabac_inline_aarch64 (same
//   shape on arm64); libavcodec/cabac.c:162-188 ff_init_cabac_decoder
//   (init, untouched); libavcodec/h264_cabac.c:1591-1705
//   decode_cabac_residual_internal (sig/last map shape, our batch covers
//   map) + :1724-1765 STORE_BLOCK (level prefix + bypass suffix shape,
//   our batch covers levels since step 2).
// Profile (2026-09-15, same machine): High detail clip h_8x8 CABAC
// ~38% of decode (SigLast+Levels+BinFast), Main m_main ~15%, Main 720p
// ~1% (mcParts/Deblock dominate, batch insensitive there by design).
// Difference: ffmpeg batches in asm with no calls at all; we batch the
// full residual block in Go (one call per block, locals across map and
// levels, refills exact, so the diff stays reviewable).
// Ours: cabac_syntax.go cabacCoeffData/cabacCoeffData8x8 (dispatch,
// per-block branch, same switch as S1) + this file (batch); scalar loops
// stay in cabac_syntax.go as留守. Output is bit-identical to the scalar
//留守 (cabac_s3_test.go pins map + full-clip equality). Set
// GPUI_SCALAR_CONVERT=1 to force scalar (same switch as S1 convert/qpel/
// deblock, one flag for a full-scalar double run).

// cabacScalarForced skips the batch kernels, read once from the
// environment so the hot path pays one predictable branch per block.
var cabacScalarForced = func() bool {
	switch os.Getenv("GPUI_SCALAR_CONVERT") {
	case "1", "true", "TRUE":
		return true
	}
	return false
}()

// cabacSigLastFast4x4 runs one 4x4 significance map with low/range/nbits
// in locals. It mirrors the scalar loop in cabacCoeffData exactly
// (same bin order, same tables, same refill): sig then last per
// position, tail when the map runs to maxCoeff-1. Levels follow in
// cabacLevelsFast4x4 with the same locals discipline.
func cabacSigLastFast4x4(d *Decoder, sigBase, lastBase uint16, maxCoeff int, index *[16]int) (cc, last int) {
	c := d.cab
	low, rng, nbits := c.low, c.rng, c.nbits
	cc = 0
	last = 0
	for ; last < maxCoeff-1; last++ {
		// sig bin (scalar body, fields in locals).
		st := &d.cabCtx[sigBase+uint16(last)]
		s := int(*st)
		lps := int(uint8(cabacLPSRange[2*(int(rng)&0xC0)+s]))
		rng -= int32(lps)
		mask := (rng<<17 - low) >> 31
		low -= (rng << 17) & mask
		rng += int32(lps-int(rng)) & mask
		s ^= int(mask)
		*st = cabacMLPSState[128+s]
		bit := s & 1
		shift := uint(cabacNormShift[rng])
		rng <<= shift
		low <<= shift
		nbits += int(shift)
		if low&0xFFFF == 0 {
			i := bits.TrailingZeros32(uint32(low)) - 16
			var b0, b1 uint32
			if c.pos < len(c.buf) {
				b0 = uint32(c.buf[c.pos])
			}
			if c.pos+1 < len(c.buf) {
				b1 = uint32(c.buf[c.pos+1])
			}
			x := uint32(0xFFFF0001) + b0<<9 + b1<<1
			low += int32(x << uint(i))
			if c.pos < c.end {
				c.pos += 2
			}
		}
		if bit == 0 {
			continue
		}
		index[cc] = last
		cc++
		// last bin (same body, last base).
		st = &d.cabCtx[lastBase+uint16(last)]
		s = int(*st)
		lps = int(uint8(cabacLPSRange[2*(int(rng)&0xC0)+s]))
		rng -= int32(lps)
		mask = (rng<<17 - low) >> 31
		low -= (rng << 17) & mask
		rng += int32(lps-int(rng)) & mask
		s ^= int(mask)
		*st = cabacMLPSState[128+s]
		bit = s & 1
		shift = uint(cabacNormShift[rng])
		rng <<= shift
		low <<= shift
		nbits += int(shift)
		if low&0xFFFF == 0 {
			i := bits.TrailingZeros32(uint32(low)) - 16
			var b0, b1 uint32
			if c.pos < len(c.buf) {
				b0 = uint32(c.buf[c.pos])
			}
			if c.pos+1 < len(c.buf) {
				b1 = uint32(c.buf[c.pos+1])
			}
			x := uint32(0xFFFF0001) + b0<<9 + b1<<1
			low += int32(x << uint(i))
			if c.pos < c.end {
				c.pos += 2
			}
		}
		if bit != 0 {
			last = maxCoeff
			break
		}
	}
	if last == maxCoeff-1 {
		index[cc] = last
		cc++
	}
	c.low, c.rng, c.nbits = low, rng, nbits
	return cc, last
}

// cabacSigLastFast8x8 runs one 8x8 luma significance map with locals.
// Same mirror rule against cabacCoeffData8x8 (sig offsets via
// cabacSigOffset8x8, last offsets via cabacLastCoeffOffset8x8).
func cabacSigLastFast8x8(d *Decoder, index *[64]int) (cc, last int) {
	const sigBase, lastBase = uint16(402), uint16(417)
	c := d.cab
	low, rng, nbits := c.low, c.rng, c.nbits
	cc, last = 0, 0
	for ; last < 63; last++ {
		st := &d.cabCtx[sigBase+cabacSigOffset8x8[last]]
		s := int(*st)
		lps := int(uint8(cabacLPSRange[2*(int(rng)&0xC0)+s]))
		rng -= int32(lps)
		mask := (rng<<17 - low) >> 31
		low -= (rng << 17) & mask
		rng += int32(lps-int(rng)) & mask
		s ^= int(mask)
		*st = cabacMLPSState[128+s]
		bit := s & 1
		shift := uint(cabacNormShift[rng])
		rng <<= shift
		low <<= shift
		nbits += int(shift)
		if low&0xFFFF == 0 {
			i := bits.TrailingZeros32(uint32(low)) - 16
			var b0, b1 uint32
			if c.pos < len(c.buf) {
				b0 = uint32(c.buf[c.pos])
			}
			if c.pos+1 < len(c.buf) {
				b1 = uint32(c.buf[c.pos+1])
			}
			x := uint32(0xFFFF0001) + b0<<9 + b1<<1
			low += int32(x << uint(i))
			if c.pos < c.end {
				c.pos += 2
			}
		}
		if bit == 0 {
			continue
		}
		index[cc] = last
		cc++
		st = &d.cabCtx[lastBase+uint16(cabacLastCoeffOffset8x8[last])]
		s = int(*st)
		lps = int(uint8(cabacLPSRange[2*(int(rng)&0xC0)+s]))
		rng -= int32(lps)
		mask = (rng<<17 - low) >> 31
		low -= (rng << 17) & mask
		rng += int32(lps-int(rng)) & mask
		s ^= int(mask)
		*st = cabacMLPSState[128+s]
		bit = s & 1
		shift = uint(cabacNormShift[rng])
		rng <<= shift
		low <<= shift
		nbits += int(shift)
		if low&0xFFFF == 0 {
			i := bits.TrailingZeros32(uint32(low)) - 16
			var b0, b1 uint32
			if c.pos < len(c.buf) {
				b0 = uint32(c.buf[c.pos])
			}
			if c.pos+1 < len(c.buf) {
				b1 = uint32(c.buf[c.pos+1])
			}
			x := uint32(0xFFFF0001) + b0<<9 + b1<<1
			low += int32(x << uint(i))
			if c.pos < c.end {
				c.pos += 2
			}
		}
		if bit != 0 {
			last = 64
			break
		}
	}
	if last == 63 {
		index[cc] = last
		cc++
	}
	c.low, c.rng, c.nbits = low, rng, nbits
	return cc, last
}

// cabacBinFast decodes one context bin with low/range/nbits in locals.
// Same body as cabacDec.bin (same tables, same order, same refill2);
// the caller keeps the locals across the block's bins.
func cabacBinFast(c *cabacDec, d *Decoder, ctx uint16, low, rng int32, nbits int) (bit int, nlow, nrng int32, nnbits int) {
	st := &d.cabCtx[ctx]
	s := int(*st)
	lps := int(uint8(cabacLPSRange[2*(int(rng)&0xC0)+s]))
	rng -= int32(lps)
	mask := (rng<<17 - low) >> 31
	low -= (rng << 17) & mask
	rng += int32(lps-int(rng)) & mask
	s ^= int(mask)
	*st = cabacMLPSState[128+s]
	bit = s & 1
	shift := uint(cabacNormShift[rng])
	rng <<= shift
	low <<= shift
	nbits += int(shift)
	if low&0xFFFF == 0 {
		i := bits.TrailingZeros32(uint32(low)) - 16
		var b0, b1 uint32
		if c.pos < len(c.buf) {
			b0 = uint32(c.buf[c.pos])
		}
		if c.pos+1 < len(c.buf) {
			b1 = uint32(c.buf[c.pos+1])
		}
		x := uint32(0xFFFF0001) + b0<<9 + b1<<1
		low += int32(x << uint(i))
		if c.pos < c.end {
			c.pos += 2
		}
	}
	return bit, low, rng, nbits
}

// cabacBypassFast decodes one equiprobable bin with locals (refill, not
// refill2, same as cabacDec.bypass).
func cabacBypassFast(c *cabacDec, low, rng int32, nbits int) (bit int, nlow int32, nnbits int) {
	low += low
	nbits++
	if low&0xFFFF == 0 {
		var b0, b1 int32
		if c.pos < len(c.buf) {
			b0 = int32(c.buf[c.pos])
		}
		if c.pos+1 < len(c.buf) {
			b1 = int32(c.buf[c.pos+1])
		}
		low += b0<<9 | b1<<1
		low -= 0xFFFF
		if c.pos < c.end {
			c.pos += 2
		}
	}
	if low < rng<<17 {
		return 0, low, nbits
	}
	low -= rng << 17
	return 1, low, nbits
}

// cabacBypassSignFast maps one bypass bin onto +/-v with locals (same as
// cabacDec.bypassSign).
func cabacBypassSignFast(c *cabacDec, v, low, rng int32, nbits int) (out, nlow int32, nnbits int) {
	low += low
	nbits++
	if low&0xFFFF == 0 {
		var b0, b1 int32
		if c.pos < len(c.buf) {
			b0 = int32(c.buf[c.pos])
		}
		if c.pos+1 < len(c.buf) {
			b1 = int32(c.buf[c.pos+1])
		}
		low += b0<<9 | b1<<1
		low -= 0xFFFF
		if c.pos < c.end {
			c.pos += 2
		}
	}
	r := rng << 17
	low -= r
	mask := low >> 31
	r &= mask
	low += r
	v = (v ^ mask) - mask
	return v, low, nbits
}

// cabacLevelsFast4x4 runs one 4x4 level loop with locals. It mirrors the
// scalar level loop in cabacCoeffData exactly (same prefix order, same
// node machine, same bypass suffixes and sign): levels close in reverse
// with the shared node state.
func cabacLevelsFast4x4(d *Decoder, lvlBase uint16, index *[16]int, cc, scanOff int, out *[16]int32) {
	c := d.cab
	low, rng, nbits := c.low, c.rng, c.nbits
	node := 0
	for cc > 0 {
		cc--
		j := index[cc] + scanOff
		lvl := 1
		var bit int
		bit, low, rng, nbits = cabacBinFast(c, d, lvlBase+uint16(cabacLevel1Ctx[node]), low, rng, nbits)
		if bit == 0 {
			node = int(cabacLevelTrans[node])
		} else {
			lvl = 2
			gctx := lvlBase + uint16(cabacLevelGt1Ctx[node])
			node = int(cabacLevelTrans[8+node])
			for lvl < 15 {
				bit, low, rng, nbits = cabacBinFast(c, d, gctx, low, rng, nbits)
				if bit == 0 {
					break
				}
				lvl++
			}
			if lvl >= 15 {
				k := 0
				for k < 23 {
					bit, low, nbits = cabacBypassFast(c, low, rng, nbits)
					if bit == 0 {
						break
					}
					k++
				}
				lvl = 1
				for k > 0 {
					k--
					bit, low, nbits = cabacBypassFast(c, low, rng, nbits)
					lvl += lvl + bit
				}
				lvl += 14
			}
		}
		var sv int32
		sv, low, nbits = cabacBypassSignFast(c, int32(-lvl), low, rng, nbits)
		out[j] = sv
	}
	c.low, c.rng, c.nbits = low, rng, nbits
}

// cabacLevelsFast8x8 runs one 8x8 luma level loop with locals (same mirror
// rule against cabacCoeffData8x8).
func cabacLevelsFast8x8(d *Decoder, index *[64]int, cc int, out *[64]int32) {
	const lvlBase = uint16(426)
	c := d.cab
	low, rng, nbits := c.low, c.rng, c.nbits
	node := 0
	for cc > 0 {
		cc--
		j := index[cc]
		lvl := 1
		var bit int
		bit, low, rng, nbits = cabacBinFast(c, d, lvlBase+uint16(cabacLevel1Ctx[node]), low, rng, nbits)
		if bit == 0 {
			node = int(cabacLevelTrans[node])
		} else {
			lvl = 2
			gctx := lvlBase + uint16(cabacLevelGt1Ctx[node])
			node = int(cabacLevelTrans[8+node])
			for lvl < 15 {
				bit, low, rng, nbits = cabacBinFast(c, d, gctx, low, rng, nbits)
				if bit == 0 {
					break
				}
				lvl++
			}
			if lvl >= 15 {
				k := 0
				for k < 23 {
					bit, low, nbits = cabacBypassFast(c, low, rng, nbits)
					if bit == 0 {
						break
					}
					k++
				}
				lvl = 1
				for k > 0 {
					k--
					bit, low, nbits = cabacBypassFast(c, low, rng, nbits)
					lvl += lvl + bit
				}
				lvl += 14
			}
		}
		var sv int32
		sv, low, nbits = cabacBypassSignFast(c, int32(-lvl), low, rng, nbits)
		out[j] = sv
	}
	c.low, c.rng, c.nbits = low, rng, nbits
}
