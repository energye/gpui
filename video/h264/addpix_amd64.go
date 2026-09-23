//go:build amd64

package h264

import "unsafe"

//go:noescape
func addResidRow8(dst, pred, res unsafe.Pointer)

//go:noescape
func addResidRow4(dst, pred, res unsafe.Pointer)

//go:noescape
func addResidBlock8(dst unsafe.Pointer, dstStride uintptr, pred unsafe.Pointer, predStride uintptr, res unsafe.Pointer, h int)

//go:noescape
func addResidBlock4(dst unsafe.Pointer, dstStride uintptr, pred unsafe.Pointer, predStride uintptr, res unsafe.Pointer, h int)

// S1b-A3 amd64 residual-add block path (SSE2 row kernels + scalar tail).
// The kernels (addpix_amd64.s) add int32 residuals onto pred bytes
// per call with PACKUSWB saturation (== clipPixel 0..255): row8 covers
// 8-wide rows, row4 covers 4-wide rows (same formula, same clip).
// Peer (ffmpeg, read-only, ideas only, no code copied):
//   libavcodec/h264addpx_template.c:30-60 FUNCC(ff_h264_add_pixels4/8)
//   (dst[i] += src[i] lane add, our row4/row8 kernels) hooked via
//   libavcodec/h264dsp.c:78-98 add_pixels/idct table (our addResid
//   dispatch below). Values stay 16-bit (pred+res in -2^15..2^15 range
// of real streams; PACKSSLW pre-saturation + PACKUSWB saturate like
// clipPixel on overflow).

// addResidArch is the amd64 entry: rows of width 4 or 8.
//
// S1b-R fused block path (one call per block): the row kernels above
// cost a call + a Go wrapper row (slicing, bounds checks, base math)
// per row; the wrapper matched the kernels' own flat. Whole blocks
// fuse into one call each (strides ride as uintptr, res rows are
// contiguous); odd sizes keep the row path. Same formula, same clip.
func addResidArch(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) bool {
	if w != 4 && w != 8 {
		return false
	}
	if w == 8 {
		addResidBlock8(
			unsafe.Pointer(&pic[by*stride+bx]), uintptr(stride),
			unsafe.Pointer(&pred[0]), uintptr(predStride),
			unsafe.Pointer(&res[0]), h,
		)
		return true
	}
	addResidBlock4(
		unsafe.Pointer(&pic[by*stride+bx]), uintptr(stride),
		unsafe.Pointer(&pred[0]), uintptr(predStride),
		unsafe.Pointer(&res[0]), h,
	)
	return true
}
