//go:build arm64

package color

import "unsafe"

// S1 arm64 NEON dispatch (8 pixels per kernel iteration, scalar tail).
// Peer: ffmpeg libswscale/aarch64/yuv2rgb_neon.S; kernel in
// convert_arm64.s (WORD-encoded MUL/UQXTN, see notes there). Scalar留守
// stays the fallback (GPUI_SCALAR_CONVERT=1 or tail rows).

//go:noescape
func neonRowBulk(dst, y, cb, cr unsafe.Pointer, n int, yMul, yOff, rCr, gCb, gCr, bCb int)

func convertBandSIMD(dst, y, cb, cr []byte, w int, t coeffs, ys, ye int) bool {
	bulk := w &^ 7
	cw := w / 2
	yMul, yOff := t.yMul, t.yOff
	rCr, gCb, gCr, bCb := t.rCr, t.gCb, t.gCr, t.bCb
	for yy := ys; yy < ye; yy++ {
		yRow := y[yy*w : (yy+1)*w]
		dRow := dst[yy*w*4 : (yy+1)*w*4]
		cBase := (yy >> 1) * cw
		cbRow := cb[cBase : cBase+cw]
		crRow := cr[cBase : cBase+cw]
		if bulk > 0 {
			neonRowBulk(unsafe.Pointer(&dRow[0]), unsafe.Pointer(&yRow[0]), unsafe.Pointer(&cbRow[0]), unsafe.Pointer(&crRow[0]), bulk, yMul, yOff, rCr, gCb, gCr, bCb)
		}
		// Tail (at most three pairs: w is even): same math as the scalar
		//留守, kept inline (a helper would cost one call per 2 pixels,
		// see the NOTE on convertBandScalar); S1 pins both bit-identical.
		for xx := bulk; xx < w; xx += 2 {
			ci := xx >> 1
			d := int(cbRow[ci]) - 128
			e := int(crRow[ci]) - 128
			re := rCr * e
			gd := gCb * d
			ge := gCr * e
			bd := bCb * d
			o := xx * 4
			y0 := yMul * (int(yRow[xx]) - yOff)
			dRow[o] = clip8((y0 + re + 128) >> 8)
			dRow[o+1] = clip8((y0 - gd - ge + 128) >> 8)
			dRow[o+2] = clip8((y0 + bd + 128) >> 8)
			dRow[o+3] = 255
			y1 := yMul * (int(yRow[xx+1]) - yOff)
			dRow[o+4] = clip8((y1 + re + 128) >> 8)
			dRow[o+5] = clip8((y1 - gd - ge + 128) >> 8)
			dRow[o+6] = clip8((y1 + bd + 128) >> 8)
			dRow[o+7] = 255
		}
	}
	return true
}
