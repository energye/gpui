//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package image

// Row-wise 2x2 box downsample.
//
// This is the hot path of the mip chain (P3.6 batch 2 turns it into a real
// per-image cost instead of a test-only one), so it reads and writes whole rows
// instead of calling GetRGBA/SetRGBA per pixel. The per-pixel version measured
// 17.6ms for one 512x512 chain (≈350k lock acquisitions, four per output pixel
// of every level); this version produces byte-identical levels — pinned by
// TestDownsampleByteIdenticalToPerPixelReference, which rebuilds each level with
// the original per-pixel semantics and compares.
//
// Averaging happens on the stored bytes of the decoded RGBA tuple, exactly as
// before: for premultiplied formats that means premultiplied averaging (what
// Skia's SkMipmap::Build does too), and Gray16 keeps reading the high byte.
func downsample(src *ImageBuf) *ImageBuf {
	srcW, srcH := src.Bounds()
	if srcW <= 0 || srcH <= 0 {
		return nil
	}
	dstW := max(1, srcW/2)
	dstH := max(1, srcH/2)

	dst := GetFromDefault(dstW, dstH, src.Format())
	if dst == nil {
		return nil
	}

	format := src.Format()
	bpe := format.BytesPerPixel()
	dstStride := dst.Stride()
	dstData := dst.Data()
	if dstData == nil || len(dstData) < dstStride*dstH {
		dst.InvalidatePremulCache()
		return dst
	}

	gray := format == FormatGray8 || format == FormatGray16
	// Channel offsets within one texel, matching GetRGBA's decode and
	// SetRGBA's encode for this format.
	var rOff, gOff, bOff, aOff int
	switch format {
	case FormatBGRA8, FormatBGRAPremul:
		rOff, gOff, bOff, aOff = 2, 1, 0, 3
	default:
		rOff, gOff, bOff, aOff = 0, 1, 2, 3
	}

	for dy := 0; dy < dstH; dy++ {
		sy0 := min(dy*2, srcH-1)
		sy1 := min(sy0+1, srcH-1)
		row0 := src.RowBytes(sy0)
		row1 := src.RowBytes(sy1)
		if row0 == nil || row1 == nil {
			continue
		}
		drow := dstData[dy*dstStride:]
		for dx := 0; dx < dstW; dx++ {
			sx0 := min(dx*2, srcW-1)
			sx1 := min(sx0+1, srcW-1)
			p00 := row0[sx0*bpe:]
			p01 := row0[sx1*bpe:]
			p10 := row1[sx0*bpe:]
			p11 := row1[sx1*bpe:]

			var sumR, sumG, sumB, sumA uint16
			if gray {
				// GetRGBA decodes gray as (v,v,v,255); Gray16 reads the high
				// byte, so the average is over those four reads.
				off := 0
				if format == FormatGray16 {
					off = 1
				}
				v := uint16(p00[off]) + uint16(p01[off]) + uint16(p10[off]) + uint16(p11[off])
				c := byte(v / 4)
				// SetRGBA re-encodes (c,c,c,255) through the luminance formula in
				// int arithmetic (299+587+114 = 1000, so it round-trips) —
				// reproduce that width, uint16 would overflow at ×587.
				gv := byte((int(c)*299 + int(c)*587 + int(c)*114) / 1000)
				drow[dx*bpe] = gv
				if format == FormatGray16 {
					drow[dx*bpe+1] = gv
				}
				continue
			}
			if format == FormatRGB8 {
				// No alpha channel: GetRGBA reports 255, SetRGBA writes 3 bytes.
				sumA = 1020
			} else {
				sumA = uint16(p00[aOff]) + uint16(p01[aOff]) + uint16(p10[aOff]) + uint16(p11[aOff])
			}
			sumR = uint16(p00[rOff]) + uint16(p01[rOff]) + uint16(p10[rOff]) + uint16(p11[rOff])
			sumG = uint16(p00[gOff]) + uint16(p01[gOff]) + uint16(p10[gOff]) + uint16(p11[gOff])
			sumB = uint16(p00[bOff]) + uint16(p01[bOff]) + uint16(p10[bOff]) + uint16(p11[bOff])

			drow[dx*bpe+rOff] = byte(sumR / 4)
			drow[dx*bpe+gOff] = byte(sumG / 4)
			drow[dx*bpe+bOff] = byte(sumB / 4)
			if format != FormatRGB8 {
				drow[dx*bpe+aOff] = byte(sumA / 4)
			}
		}
	}
	// Rows were written behind GetRGBA's back: drop any premul cache the pooled
	// buffer may have carried from its previous life.
	dst.InvalidatePremulCache()
	return dst
}
