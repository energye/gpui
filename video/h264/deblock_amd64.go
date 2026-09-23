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

// S1-E16 whole-edge weak kernels (one call filters 16 lines with
// per-segment tc; tc[g]<0 skips group g). Bodies equal four 4-line
// calls bit for bit; only weak edges arrive here (all bS<4, checked by
// deblockLumaEdge16). Strong/mixed edges stay on the per-segment path.

//go:noescape
func deblockVWeak16(p unsafe.Pointer, stride, alpha, beta int, tc unsafe.Pointer)

//go:noescape
func deblockHWeak16(p unsafe.Pointer, stride, alpha, beta int, tc unsafe.Pointer)

// S1-E16-strong whole-edge kernels (one call filters a 16-line
// all-strong edge; no tc, no skip groups: every segment filters).
// Bodies equal four 4-line strong calls bit for bit.

//go:noescape
func deblockVStrong16(p unsafe.Pointer, stride, alpha, beta int)

//go:noescape
func deblockHStrong16(p unsafe.Pointer, stride, alpha, beta int)

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

// deblockLumaEdge16 filters one whole 16-line luma edge in a single
// asm call (ffmpeg filter_mb_edgev shape: bS[4], one gate, one kernel).
// The taken matrix lives in classifyEdge16 (deblock_fast.go, single
// source): zero/gated edges report true without touching the picture
// (every per-segment filter would skip: scalar returns early on
// bS<=0, the weak 16-line kernels skip tc<0 groups, and closed gates
// write nothing on every path); uniform weak edges run the
// per-segment-tc kernel; uniform strong (all-4, e.g. intra MB edges)
// runs the strong kernel with no skips; mixed weak/strong edges
// (e.g. intra/inter borders) report false and run the per-segment
// dispatch with identical output. Forced-scalar mode reports false.
// tc[g] must equal filterTC(qp, fa, bS[g]) (negative for bS<=0).
// arm64/NEON 16-line kernels are open work (same output via the
// per-segment path; perf differs, see plan §12 S1); i386/32-bit arm
// stay scalar by design.
func deblockLumaEdge16(p []uint8, stride, ex, ey int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	if deblockScalarForced {
		return false
	}
	switch classifyEdge16(bS, alpha, beta) {
	case edge16Zero, edge16Gated:
		return true
	case edge16Mixed:
		// Rare but must stay exact via the per-segment dispatch.
		return false
	case edge16Strong:
		base := unsafe.Pointer(&p[ey*stride+ex])
		if !vertical {
			deblockVStrong16(base, stride, alpha, beta)
		} else {
			deblockHStrong16(base, stride, alpha, beta)
		}
		return true
	default:
		base := unsafe.Pointer(&p[ey*stride+ex])
		tcp := unsafe.Pointer(&tc[0])
		if !vertical {
			deblockVWeak16(base, stride, alpha, beta, tcp)
		} else {
			deblockHWeak16(base, stride, alpha, beta, tcp)
		}
		return true
	}
}
