//go:build arm64

package h264

import "unsafe"

// S1 arm64 deblock direct path (NEON, 4-line kernels, no Go copy).
// Mirrors deblock_amd64.go: each kernel loads taps straight from the
// picture, filters 4 lines at once (one H4 lane per line, branchless
// masks), and stores straight back. Semantics match filterLumaEdge
// exactly. Only edges DeblockPicture already cleared arrive here.
//
// NEON note: Go's arm64 vector mnemonics cover VADD/VSUB, VUMIN/VUMAX,
// VCMEQ, VAND/VORR/VEOR, VDUP, VMOV (lane), VUSHR/VSHL, VUXTL — no
// UABD/CMHI/SMIN/SMAX/SSHR/SQXTUN. The kernels below use only that
// named set (same discipline as qpel_arm64.s):
//   - abs(a-b) via wrapping SUBs both ways + VUMIN (one side is the
//     true abs 0..255, the other 65536-abs, unsigned min is the abs).
//   - a<b (unsigned 0..255) via VUMIN+VCMEQ: le=(min==a), eq=(a==b),
//     lt=le&~eq.
//   - arithmetic >>3/>>1 of possibly-negative numerators via bias
//     (+2048/+512, multiples of the divisor) + logical VUSHR, then
//     subtract bias>>shift (exact floor, no SSHR needed).
//   - signed clip3 via bias +256 into positive + VUMAX/VUMIN, then -256.
//   - saturating 0..255 via bias +256 the same way.
//   - blend new/old via VAND/VORR/VEOR with the FFFF/0 masks (no VBSL:
//     Go only allows B8/B16 there and the operand order is easy to
//     misread; AND/OR is explicit).
// Scalar byte gather/scatter (MOVBU/MOVB + VMOV lane) mirrors the
// amd64 kernels' MOVBLZX/PINSRW gather.

//go:noescape
func deblockVWeakArm(p unsafe.Pointer, stride, alpha, beta, tc int)

//go:noescape
func deblockHWeakArm(p unsafe.Pointer, stride, alpha, beta, tc int)

//go:noescape
func deblockVStrongArm(p unsafe.Pointer, stride, alpha, beta int)

//go:noescape
func deblockHStrongArm(p unsafe.Pointer, stride, alpha, beta int)

func deblockLumaArch(p []uint8, stride, ex, ey int, vertical bool, bS, alpha, beta, tc int) bool {
	base := unsafe.Pointer(&p[ey*stride+ex])
	if !vertical {
		if bS < 4 {
			deblockVWeakArm(base, stride, alpha, beta, tc)
		} else {
			deblockVStrongArm(base, stride, alpha, beta)
		}
		return true
	}
	if bS < 4 {
		deblockHWeakArm(base, stride, alpha, beta, tc)
	} else {
		deblockHStrongArm(base, stride, alpha, beta)
	}
	return true
}

// deblockLumaEdge16 stays on the per-segment path on arm64 (the NEON
// 16-line twin of deblockVWeak16/deblockHWeak16 is open work; output
// is identical via the 4-line kernels, only the call count differs).
func deblockLumaEdge16(p []uint8, stride, ex, ey int, vertical bool, bS, tc [4]int, alpha, beta int) bool {
	return false
}
