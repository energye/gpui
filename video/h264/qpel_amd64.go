//go:build amd64

package h264

import "unsafe"

// S1 amd64 qpel block path (SSE4.1 row kernel + scalar combine).
// The kernel (qpel_amd64.s) emits unrounded 6-tap horizontal sums,
// 8 pixels per iteration; rounding/clip/avg and the vertical pass stay
// in Go (cheap, O(block)). Only interior blocks run here (all taps
// inside the frame, no per-tap clipping); edges fall back to the
// scalar留守 which clips per tap.

//go:noescape
func qpelHorSums(sums, src unsafe.Pointer, n int)

const (
	qpelMaxW = 16
	qpelMaxH = 16
)

// qpelBlock predicts one interior sub-pel partition. w/h are the
// reference dims, (ix,iy) the integer base, (bw,bh) the block size,
// (fx,fy) the quarter fracs. False means "run the scalar留守".
func qpelBlock(dst, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	if bw <= 0 || bh <= 0 || bw > qpelMaxW || bh > qpelMaxH {
		return false
	}
	// Interior: taps read x-2..x+3 / y-2..y+3 around every output,
	// plus the +1 neighbour for quarter avgs.
	if ix < 2 || iy < 2 || ix+bw+4 > w || iy+bh+4 > h {
		return false
	}
	switch {
	case fy == 0:
		qpelHorOnly(dst, plane, w, ix, iy, bw, bh, fx)
	case fx == 0:
		qpelVerOnly(dst, plane, w, ix, iy, bw, bh, fy)
	default:
		qpelGeneral(dst, plane, w, ix, iy, bw, bh, fx, fy)
	}
	return true
}

// horSumsRow writes bw unrounded 6-tap sums for absolute row ay,
// base column ax (taps [1 -5 20 20 -5 1], same sum as halfH pre-round).
func horSumsRow(sums []int16, plane []byte, stride, ax, ay, bw int) {
	bulk := bw &^ 7
	if bulk > 0 {
		qpelHorSums(unsafe.Pointer(&sums[0]), unsafe.Pointer(&plane[ay*stride+ax-2]), bulk)
	}
	off := ay*stride + ax
	for i := bulk; i < bw; i++ {
		o := off + i
		sums[i] = int16(int(plane[o-2]) + int(plane[o+3]) -
			5*(int(plane[o-1])+int(plane[o+2])) +
			20*(int(plane[o])+int(plane[o+1])))
	}
}

// roundHalf rounds one unrounded sum like halfH: (s+16)>>5 clipped.
func roundHalf(s int16) uint8 {
	v := (int(s) + 16) >> 5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// qpelHorOnly handles fy==0: one horizontal pass per row, then avg.
func qpelHorOnly(dst, plane []byte, stride, ix, iy, bw, bh, fx int) {
	var sums [qpelMaxW]int16
	var hh [qpelMaxW]uint8
	for dy := 0; dy < bh; dy++ {
		ay := iy + dy
		horSumsRow(sums[:], plane, stride, ix, ay, bw)
		for i := 0; i < bw; i++ {
			hh[i] = roundHalf(sums[i])
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
		default: // fx == 3
			for i := 0; i < bw; i++ {
				out[i] = uint8(avg2(int(hh[i]), int(plane[base+i+1])))
			}
		}
	}
}

// qpelVerOnly handles fx==0: one vertical pass per column, then avg.
func qpelVerOnly(dst, plane []byte, stride, ix, iy, bw, bh, fy int) {
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
			default: // fy == 3
				out[dx] = uint8(avg2(hv, int(plane[(ay+1)*stride+ax])))
			}
		}
	}
}

// qpelGeneral handles fx!=0 && fy!=0: rounded halves plus the
// single-rounding center cascade (unrounded horizontal temp, then
// vertical with +512>>10, like centerJ).
func qpelGeneral(dst, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	rows := bh
	if fy == 3 {
		rows++ // hh1 at y+1
	}
	var hh [qpelMaxH + 1][qpelMaxW]uint8
	var sums [qpelMaxW]int16
	for dy := 0; dy < rows; dy++ {
		horSumsRow(sums[:], plane, stride, ix, iy+dy, bw)
		for i := 0; i < bw; i++ {
			hh[dy][i] = roundHalf(sums[i])
		}
	}
	cols := bw
	if fx == 3 {
		cols++ // hv1 at x+1
	}
	var hv [qpelMaxH][qpelMaxW + 1]uint8
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
	var jt [qpelMaxH + 5][qpelMaxW]int16
	if fx == 2 || fy == 2 {
		for r := 0; r < bh+5; r++ {
			horSumsRow(jt[r][:], plane, stride, ix, iy-2+r, bw)
		}
	}
	for dy := 0; dy < bh; dy++ {
		out := dst[dy*bw : (dy+1)*bw]
		for dx := 0; dx < bw; dx++ {
			var v int
			switch {
			case fx == 2 && fy == 2:
				v = centerAt(&jt, dy, dx)
			case fx == 1 && fy == 1:
				v = avg2(int(hh[dy][dx]), int(hv[dy][dx]))
			case fx == 2 && fy == 1:
				v = avg2(int(hh[dy][dx]), centerAt(&jt, dy, dx))
			case fx == 3 && fy == 1:
				v = avg2(int(hh[dy][dx]), int(hv[dy][dx+1]))
			case fx == 1 && fy == 2:
				v = avg2(int(hv[dy][dx]), centerAt(&jt, dy, dx))
			case fx == 3 && fy == 2:
				v = avg2(centerAt(&jt, dy, dx), int(hv[dy][dx+1]))
			case fx == 1 && fy == 3:
				v = avg2(int(hv[dy][dx]), int(hh[dy+1][dx]))
			case fx == 2 && fy == 3:
				v = avg2(int(hh[dy+1][dx]), centerAt(&jt, dy, dx))
			default: // fx == 3 && fy == 3
				v = avg2(int(hh[dy+1][dx]), int(hv[dy][dx+1]))
			}
			out[dx] = uint8(v)
		}
	}
}

// centerAt rounds one center-cascade column like centerJ:
// vertical 6-tap over unrounded horizontal sums, single +512>>10 step.
func centerAt(jt *[qpelMaxH + 5][qpelMaxW]int16, dy, dx int) int {
	s := int(jt[dy][dx]) + int(jt[dy+5][dx]) -
		5*(int(jt[dy+1][dx])+int(jt[dy+4][dx])) +
		20*(int(jt[dy+2][dx])+int(jt[dy+3][dx]))
	return clipInt((s+512)>>10, 0, 255)
}
