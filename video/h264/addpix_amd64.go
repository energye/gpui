//go:build amd64

package h264

import "unsafe"

//go:noescape
func addResidRow8(dst, pred, res unsafe.Pointer)

// S1b-A3 amd64 residual-add block path (SSE2 row kernel + scalar tail).
// The kernel (addpix_amd64.s) adds 8 int32 residuals onto 8 pred bytes
// per call with PACKUSDW saturation (== clipPixel 0..255); rows wider
// than 8 loop, the 4-wide path runs one row per call plus a scalar tail
// of 4. Values stay 16-bit (pred+res in -2^15..2^15 range of real
// streams; PACKUSDW saturates like clipPixel on overflow).

// addResidArch is the amd64 entry: rows of width 4 or 8.
func addResidArch(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) bool {
	if w != 4 && w != 8 {
		return false
	}
	var tmp [8]uint8
	for y := 0; y < h; y++ {
		base := (by+uint32(y))*stride + bx
		prow := pred[y*predStride : y*predStride+w]
		rrow := res[y*w : (y+1)*w]
		if w == 8 {
			addResidRow8(
				unsafe.Pointer(&pic[base]),
				unsafe.Pointer(&prow[0]),
				unsafe.Pointer(&rrow[0]),
			)
			continue
		}
		// 4-wide: kernel handles all 4 via 8-lane with zero high half;
		// simpler to run scalar on 4 (cheap, O(4)) — kernel reserved
		// for 8-wide rows where the win lives.
		for x := 0; x < 4; x++ {
			v := int32(prow[x]) + rrow[x]
			tmp[x] = clipPixel(v)
		}
		copy(pic[base:base+4], tmp[:4])
	}
	return true
}
