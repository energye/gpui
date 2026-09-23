//go:build arm64

package h264

// S1 qpel shared skeleton (arch row kernel + Go combine), arm64 side.
// amd64 keeps its file-local qpelBlock/horSumsRow in qpel_amd64.go;
// this file provides the identical skeleton bound to the NEON kernel
// (qpel_arm64.s). Same gates, same combine, same scalar tail bit for
// bit; qpel_s1_test.go pins all 16 fracs on every arch via the dispatch.

import "unsafe"

// Shared combine helpers (roundHalf/centerAt) and the size caps live in
// qpel_amd64.go, built for amd64 only today. arm64 gets its own copies
// here (same math, arm-suffixed names) so the arm64 build never depends
// on amd64-tagged files; qpel_s1_test.go pins both bit-identical via
// the scalar留守.
const (
	qpelArmMaxW = 16
	qpelArmMaxH = 16
)

func qpelArmRoundHalf(s int16) uint8 {
	return uint8(clipU8((int(s) + 16) >> 5))
}

func qpelArmCenterAt(jt *[21][16]int16, dy, dx int) int {
	s := int(jt[dy][dx]) + int(jt[dy+5][dx]) -
		5*(int(jt[dy+1][dx])+int(jt[dy+4][dx])) +
		20*(int(jt[dy+2][dx])+int(jt[dy+3][dx]))
	return clipU8((s + 512) >> 10)
}

//go:noescape
func neonQpelHorSums(sums, src unsafe.Pointer, n int)

//go:noescape
func neonQpelVerSums(sums, r0, r1, r2, r3, r4, r5 unsafe.Pointer)

// qpelBlock is the arm64 entry of the shared S1 dispatch (qpelFast in
// qpel_fast.go): one interior sub-pel partition through the NEON block
// path. Same gate and same combine as amd64 qpelBlock; false means
// "run the scalar留守".
func qpelBlock(dst, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	if bw <= 0 || bh <= 0 || bw > qpelArmMaxW || bh > qpelArmMaxH {
		return false
	}
	if ix < 2 || iy < 2 || ix+bw+4 > w || iy+bh+4 > h {
		return false
	}
	switch {
	case fy == 0:
		qpelArmHorOnly(dst, plane, w, ix, iy, bw, bh, fx)
	case fx == 0:
		qpelArmVerOnly(dst, plane, w, ix, iy, bw, bh, fy)
	default:
		qpelArmGeneral(dst, plane, w, ix, iy, bw, bh, fx, fy)
	}
	return true
}

// qpelBlockInto is the strided entry (S1-W direct-write): same gate,
// output rows stride out.
func qpelBlockInto(dst []byte, dstStride int, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	if bw <= 0 || bh <= 0 || bw > qpelArmMaxW || bh > qpelArmMaxH {
		return false
	}
	if ix < 2 || iy < 2 || ix+bw+4 > w || iy+bh+4 > h {
		return false
	}
	switch {
	case fy == 0:
		qpelArmHorOnlyInto(dst, dstStride, plane, w, ix, iy, bw, bh, fx)
	case fx == 0:
		qpelArmVerOnlyInto(dst, dstStride, plane, w, ix, iy, bw, bh, fy)
	default:
		qpelArmGeneralInto(dst, dstStride, plane, w, ix, iy, bw, bh, fx, fy)
	}
	return true
}

// copyBlockInto copies a w*h block (scalar留守 for arm64; amd64
// runs the copyBlock kernel in qpel_amd64.go/s): same bytes as the
// per-row copy builtin, bit for bit.
func copyBlockInto(dst []byte, dstStride int, src []byte, srcStride int, w, h int) {
	for dy := 0; dy < h; dy++ {
		copy(dst[dy*dstStride:dy*dstStride+w], src[dy*srcStride:dy*srcStride+w])
	}
}

func qpelArmHorSumsRow(sums []int16, plane []byte, stride, ax, ay, bw int) {
	// The NEON kernel writes exactly 8 int16 sums per call (one H8
	// store). Feed it full-8 windows only; the scalar tail covers
	// the rest. bw <= 16 by the caller gate.
	for bw >= 8 {
		neonQpelHorSums(unsafe.Pointer(&sums[0]), unsafe.Pointer(&plane[ay*stride+ax-2]), 8)
		sums = sums[8:]
		ax += 8
		bw -= 8
	}
	off := ay*stride + ax
	for i := 0; i < bw; i++ {
		o := off + i
		sums[i] = int16(int(plane[o-2]) + int(plane[o+3]) -
			5*(int(plane[o-1])+int(plane[o+2])) +
			20*(int(plane[o])+int(plane[o+1])))
	}
}

// qpelArmVerSumsRow writes bw unrounded 6-tap vertical sums for output
// row ay, base column ax (taps rows ay-2..ay+3, same sum as halfV
// pre-round). Bulk 8 via the NEON kernel, tail scalar bit for bit.
func qpelArmVerSumsRow(sums []int16, plane []byte, stride, ax, ay, bw int) {
	dx0 := 0
	for ; dx0+8 <= bw; dx0 += 8 {
		neonQpelVerSums(unsafe.Pointer(&sums[dx0]),
			unsafe.Pointer(&plane[(ay-2)*stride+ax+dx0]),
			unsafe.Pointer(&plane[(ay-1)*stride+ax+dx0]),
			unsafe.Pointer(&plane[ay*stride+ax+dx0]),
			unsafe.Pointer(&plane[(ay+1)*stride+ax+dx0]),
			unsafe.Pointer(&plane[(ay+2)*stride+ax+dx0]),
			unsafe.Pointer(&plane[(ay+3)*stride+ax+dx0]))
	}
	for ; dx0 < bw; dx0++ {
		axx := ax + dx0
		p0 := int(plane[(ay-2)*stride+axx])
		p1 := int(plane[(ay-1)*stride+axx])
		p2 := int(plane[ay*stride+axx])
		p3 := int(plane[(ay+1)*stride+axx])
		p4 := int(plane[(ay+2)*stride+axx])
		p5 := int(plane[(ay+3)*stride+axx])
		sums[dx0] = int16(p0 + p5 - 5*(p1+p4) + 20*(p2+p3))
	}
}

func qpelArmHorOnly(dst, plane []byte, stride, ix, iy, bw, bh, fx int) {
	qpelArmHorOnlyInto(dst, bw, plane, stride, ix, iy, bw, bh, fx)
}

func qpelArmHorOnlyInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fx int) {
	var sums [16]int16
	var hh [16]uint8
	switch fx {
	case 2:
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmHorSumsRow(sums[:], plane, stride, ix, ay, bw)
			out := dst[dy*dstStride : dy*dstStride+bw]
			for i := 0; i < bw; i++ {
				out[i] = qpelArmRoundHalf(sums[i])
			}
		}
	case 1:
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmHorSumsRow(sums[:], plane, stride, ix, ay, bw)
			for i := 0; i < bw; i++ {
				hh[i] = qpelArmRoundHalf(sums[i])
			}
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			for i := 0; i < bw; i++ {
				out[i] = uint8(avg2(int(plane[base+i]), int(hh[i])))
			}
		}
	default: // fx == 3
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmHorSumsRow(sums[:], plane, stride, ix, ay, bw)
			for i := 0; i < bw; i++ {
				hh[i] = qpelArmRoundHalf(sums[i])
			}
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			for i := 0; i < bw; i++ {
				out[i] = uint8(avg2(int(hh[i]), int(plane[base+i+1])))
			}
		}
	}
}

func qpelArmVerOnly(dst, plane []byte, stride, ix, iy, bw, bh, fy int) {
	qpelArmVerOnlyInto(dst, bw, plane, stride, ix, iy, bw, bh, fy)
}

func qpelArmVerOnlyInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fy int) {
	var sums [16]int16
	var hv [16]uint8
	switch fy {
	case 2:
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmVerSumsRow(sums[:], plane, stride, ix, ay, bw)
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = qpelArmRoundHalf(sums[dx])
			}
		}
	case 1:
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmVerSumsRow(sums[:], plane, stride, ix, ay, bw)
			for dx := 0; dx < bw; dx++ {
				hv[dx] = qpelArmRoundHalf(sums[dx])
			}
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(plane[base+dx]), int(hv[dx])))
			}
		}
	default: // fy == 3
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmVerSumsRow(sums[:], plane, stride, ix, ay, bw)
			for dx := 0; dx < bw; dx++ {
				hv[dx] = qpelArmRoundHalf(sums[dx])
			}
			base := (ay+1)*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hv[dx]), int(plane[base+dx])))
			}
		}
	}
}

func qpelArmGeneral(dst, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	qpelArmGeneralInto(dst, bw, plane, stride, ix, iy, bw, bh, fx, fy)
}

func qpelArmGeneralInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	rows := bh
	if fy == 3 {
		rows++
	}
	cols := bw
	if fx == 3 {
		cols++
	}
	var needHH, needHV, needJT bool
	switch {
	case fx == 2 && fy == 2:
		needJT = true
	case fx == 2 && fy == 1, fx == 2 && fy == 3:
		needHH, needJT = true, true
	case fx == 1 && fy == 2, fx == 3 && fy == 2:
		needHV, needJT = true, true
	default:
		needHH, needHV = true, true
	}
	var hh [17][16]uint8
	var hv [16][17]uint8
	var jt [21][16]int16
	var sums [16]int16
	var vsums [17]int16
	if needJT {
		for r := 0; r < bh+5; r++ {
			qpelArmHorSumsRow(jt[r][:], plane, stride, ix, iy-2+r, bw)
		}
	}
	if needHH {
		if needJT {
			// jt[dy+2] sums row iy+dy: same inputs, bit-identical.
			for dy := 0; dy < rows; dy++ {
				for i := 0; i < bw; i++ {
					hh[dy][i] = qpelArmRoundHalf(jt[dy+2][i])
				}
			}
		} else {
			for dy := 0; dy < rows; dy++ {
				qpelArmHorSumsRow(sums[:], plane, stride, ix, iy+dy, bw)
				for i := 0; i < bw; i++ {
					hh[dy][i] = qpelArmRoundHalf(sums[i])
				}
			}
		}
	}
	if needHV {
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			qpelArmVerSumsRow(vsums[:], plane, stride, ix, ay, cols)
			for dx := 0; dx < cols; dx++ {
				hv[dy][dx] = qpelArmRoundHalf(vsums[dx])
			}
		}
	}
	switch {
	case fx == 2 && fy == 2:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(qpelArmCenterAt(&jt, dy, dx))
			}
		}
	case fx == 1 && fy == 1:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy][dx]), int(hv[dy][dx])))
			}
		}
	case fx == 2 && fy == 1:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy][dx]), qpelArmCenterAt(&jt, dy, dx)))
			}
		}
	case fx == 3 && fy == 1:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy][dx]), int(hv[dy][dx+1])))
			}
		}
	case fx == 1 && fy == 2:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hv[dy][dx]), qpelArmCenterAt(&jt, dy, dx)))
			}
		}
	case fx == 3 && fy == 2:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(qpelArmCenterAt(&jt, dy, dx), int(hv[dy][dx+1])))
			}
		}
	case fx == 1 && fy == 3:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hv[dy][dx]), int(hh[dy+1][dx])))
			}
		}
	case fx == 2 && fy == 3:
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy+1][dx]), qpelArmCenterAt(&jt, dy, dx)))
			}
		}
	default: // fx == 3 && fy == 3
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			for dx := 0; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy+1][dx]), int(hv[dy][dx+1])))
			}
		}
	}
}
