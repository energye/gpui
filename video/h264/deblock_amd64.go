//go:build amd64

package h264

import "unsafe"

// S1 amd64 deblock direct path (SSE2/SSSE3 4-line kernels, no Go copy).
// Each kernel (deblock_amd64.s) loads taps straight from the picture,
// filters 4 lines at once (one lane per line, branchless condition
// masks), and stores straight back. No lane buffer, no per-byte Go
// bounds checks, no gather/scatter loops in Go. Semantics match
// filterLumaEdge exactly. Only edges DeblockPicture already cleared
// (picture-border MB edges skip there) arrive here, so every tap is
// in-bounds like the scalar path.

//go:noescape
func deblockVWeak(p unsafe.Pointer, stride, alpha, beta, tc int)

//go:noescape
func deblockVStrong(p unsafe.Pointer, stride, alpha, beta int)

//go:noescape
func deblockHWeak(p unsafe.Pointer, stride, alpha, beta, tc int)

//go:noescape
func deblockHStrong(p unsafe.Pointer, stride, alpha, beta int)

func deblockLumaArch(p []uint8, stride, ex, ey int, vertical bool, bS, alpha, beta, tc int) bool {
	base := unsafe.Pointer(&p[ey*stride+ex])
	if !vertical {
		if bS < 4 {
			deblockVWeak(base, stride, alpha, beta, tc)
		} else {
			deblockVStrong(base, stride, alpha, beta)
		}
		return true
	}
	if bS < 4 {
		deblockHWeak(base, stride, alpha, beta, tc)
	} else {
		deblockHStrong(base, stride, alpha, beta)
	}
	return true
}
