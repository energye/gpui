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
	v := (int(s) + 16) >> 5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func qpelArmCenterAt(jt *[21][16]int16, dy, dx int) int {
	s := int(jt[dy][dx]) + int(jt[dy+5][dx]) -
		5*(int(jt[dy+1][dx])+int(jt[dy+4][dx])) +
		20*(int(jt[dy+2][dx])+int(jt[dy+3][dx]))
	return clipInt((s+512)>>10, 0, 255)
}

//go:noescape
func neonQpelHorSums(sums, src unsafe.Pointer, n int)

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

func qpelArmHorOnly(dst, plane []byte, stride, ix, iy, bw, bh, fx int) {
	var sums [16]int16
	var hh [16]uint8
	for dy := 0; dy < bh; dy++ {
		ay := iy + dy
		qpelArmHorSumsRow(sums[:], plane, stride, ix, ay, bw)
		for i := 0; i < bw; i++ {
			hh[i] = qpelArmRoundHalf(sums[i])
		}
		base := ay*stride + ix
		out := dst[dy*bw : (dy+1)*bw]
		switch fx {
		case 2:
			copy(out, hh[:bw])
		case 1:
			for i := 0; i < bw; i++ {
				out[i] = uint8(avg2(int(plane[base+i]), int(hh[i])))
			}
		default:
			for i := 0; i < bw; i++ {
				out[i] = uint8(avg2(int(hh[i]), int(plane[base+i+1])))
			}
		}
	}
}

func qpelArmVerOnly(dst, plane []byte, stride, ix, iy, bw, bh, fy int) {
	for dy := 0; dy < bh; dy++ {
		ay := iy + dy
		out := dst[dy*bw : (dy+1)*bw]
		for dx := 0; dx < bw; dx++ {
			ax := ix + dx
			p0 := int(plane[(ay-2)*stride+ax])
			p1 := int(plane[(ay-1)*stride+ax])
			p2 := int(plane[ay*stride+ax])
			p3 := int(plane[(ay+1)*stride+ax])
			p4 := int(plane[(ay+2)*stride+ax])
			p5 := int(plane[(ay+3)*stride+ax])
			hv := clipInt((p0+p5-5*(p1+p4)+20*(p2+p3)+16)>>5, 0, 255)
			switch fy {
			case 2:
				out[dx] = uint8(hv)
			case 1:
				out[dx] = uint8(avg2(p2, hv))
			default:
				out[dx] = uint8(avg2(hv, int(plane[(ay+1)*stride+ax])))
			}
		}
	}
}

func qpelArmGeneral(dst, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	rows := bh
	if fy == 3 {
		rows++
	}
	var hh [17][16]uint8
	var sums [16]int16
	for dy := 0; dy < rows; dy++ {
		qpelArmHorSumsRow(sums[:], plane, stride, ix, iy+dy, bw)
		for i := 0; i < bw; i++ {
			hh[dy][i] = qpelArmRoundHalf(sums[i])
		}
	}
	cols := bw
	if fx == 3 {
		cols++
	}
	var hv [16][17]uint8
	for dy := 0; dy < bh; dy++ {
		ay := iy + dy
		for dx := 0; dx < cols; dx++ {
			ax := ix + dx
			p0 := int(plane[(ay-2)*stride+ax])
			p1 := int(plane[(ay-1)*stride+ax])
			p2 := int(plane[ay*stride+ax])
			p3 := int(plane[(ay+1)*stride+ax])
			p4 := int(plane[(ay+2)*stride+ax])
			p5 := int(plane[(ay+3)*stride+ax])
			hv[dy][dx] = uint8(clipInt((p0+p5-5*(p1+p4)+20*(p2+p3)+16)>>5, 0, 255))
		}
	}
	var jt [21][16]int16
	if fx == 2 || fy == 2 {
		for r := 0; r < bh+5; r++ {
			qpelArmHorSumsRow(jt[r][:], plane, stride, ix, iy-2+r, bw)
		}
	}
	for dy := 0; dy < bh; dy++ {
		out := dst[dy*bw : (dy+1)*bw]
		for dx := 0; dx < bw; dx++ {
			var v int
			switch {
			case fx == 2 && fy == 2:
				v = qpelArmCenterAt(&jt, dy, dx)
			case fx == 1 && fy == 1:
				v = avg2(int(hh[dy][dx]), int(hv[dy][dx]))
			case fx == 2 && fy == 1:
				v = avg2(int(hh[dy][dx]), qpelArmCenterAt(&jt, dy, dx))
			case fx == 3 && fy == 1:
				v = avg2(int(hh[dy][dx]), int(hv[dy][dx+1]))
			case fx == 1 && fy == 2:
				v = avg2(int(hv[dy][dx]), qpelArmCenterAt(&jt, dy, dx))
			case fx == 3 && fy == 2:
				v = avg2(qpelArmCenterAt(&jt, dy, dx), int(hv[dy][dx+1]))
			case fx == 1 && fy == 3:
				v = avg2(int(hv[dy][dx]), int(hh[dy+1][dx]))
			case fx == 2 && fy == 3:
				v = avg2(int(hh[dy+1][dx]), qpelArmCenterAt(&jt, dy, dx))
			default:
				v = avg2(int(hh[dy+1][dx]), int(hv[dy][dx+1]))
			}
			out[dx] = uint8(v)
		}
	}
}
