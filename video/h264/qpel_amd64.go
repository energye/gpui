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

//go:noescape
func qpelVerSums(sums, r0, r1, r2, r3, r4, r5 unsafe.Pointer)

//go:noescape
func qpelRoundHalf8(dst, sums unsafe.Pointer)

//go:noescape
func centerVec4(dst, r0, r1, r2, r3, r4, r5 unsafe.Pointer)

//go:noescape
func centerAvg4(dst, r0, r1, r2, r3, r4, r5, avg unsafe.Pointer)

//go:noescape
func centerAvgBlock(dst unsafe.Pointer, dstStride uintptr, jt unsafe.Pointer, avg unsafe.Pointer, avgStride uintptr, w, h int)

//go:noescape
func horRoundBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)

//go:noescape
func verRoundBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)

//go:noescape
func roundHalfBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)

//go:noescape
func horSumsBlock(dst unsafe.Pointer, dstStride uintptr, src unsafe.Pointer, srcStride uintptr, w, h int)

//go:noescape
func avgRow16(dst, a, b unsafe.Pointer)

//go:noescape
func avgRow8(dst, a, b unsafe.Pointer)

//go:noescape
func avgRow4(dst, a, b unsafe.Pointer)

//go:noescape
func avgBlock(dst unsafe.Pointer, dstStride uintptr, a unsafe.Pointer, aStride uintptr, b unsafe.Pointer, bStride uintptr, w, h int)

const (
	qpelMaxW = 16
	qpelMaxH = 16
)

// qpelBlock predicts one interior sub-pel partition. w/h are the
// reference dims, (ix,iy) the integer base, (bw,bh) the block size,
// (fx,fy) the quarter fracs. False means "run the scalar留守".
func qpelBlock(dst, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
	return qpelBlockInto(dst, bw, plane, w, h, ix, iy, bw, bh, fx, fy)
}

// qpelBlockInto is the strided entry (S1-W direct-write): interior
// gate identical, output rows stride out. False means "run the scalar
// 留守" (same callers, same fallback).
func qpelBlockInto(dst []byte, dstStride int, plane []byte, w, h, ix, iy, bw, bh, fx, fy int) bool {
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
		qpelHorOnlyInto(dst, dstStride, plane, w, ix, iy, bw, bh, fx)
	case fx == 0:
		qpelVerOnlyInto(dst, dstStride, plane, w, ix, iy, bw, bh, fy)
	default:
		qpelGeneralInto(dst, dstStride, plane, w, ix, iy, bw, bh, fx, fy)
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

// verSumsRow writes bw unrounded 6-tap vertical sums for output row
// ay, base column ax (taps rows ay-2..ay+3 at column ax+dx, same sum
// as halfV pre-round). Bulk 8 via the SSE kernel, tail scalar bit
// for bit (all live widths are multiples of 8; the tail is cover).
func verSumsRow(sums []int16, plane []byte, stride, ax, ay, bw int) {
	dx0 := 0
	for ; dx0+8 <= bw; dx0 += 8 {
		qpelVerSums(unsafe.Pointer(&sums[dx0]),
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

// roundHalf rounds one unrounded sum like halfH: (s+16)>>5 clipped.
// Branchless clip (clipU8): same bits, no mispredicts in the per-row
// rounding loops.
func roundHalf(s int16) uint8 {
	return uint8(clipU8((int(s) + 16) >> 5))
}

// roundHalfRow rounds n sums into dst (bulk 8 through the SSE kernel,
// tail scalar bit for bit). All live widths are multiples of 8; the
// tail is cover for odd chroma-adjacent partitions and tests.
func roundHalfRow(dst []uint8, sums []int16, n int) {
	bulk := n &^ 7
	for i := 0; i < bulk; i += 8 {
		qpelRoundHalf8(unsafe.Pointer(&dst[i]), unsafe.Pointer(&sums[i]))
	}
	for i := bulk; i < n; i++ {
		dst[i] = roundHalf(sums[i])
	}
}

// avgRowInto writes n averaged bytes: dst[i] = (a[i]+b[i]+1)>>1,
// bit-identical to the avg2 scalar loop (PAVGB computes exactly that,
// same as ffmpeg's pavgb avg instances). Widths 16/8/4 go through one
// kernel call each; odd tails stay scalar (cover: all live luma widths
// are multiples of 4). Zero-alloc: noescape kernels, no heap.
func avgRowInto(dst, a, b []byte) {
	n := len(dst)
	i := 0
	for ; i+16 <= n; i += 16 {
		avgRow16(unsafe.Pointer(&dst[i]), unsafe.Pointer(&a[i]), unsafe.Pointer(&b[i]))
	}
	if i+8 <= n {
		avgRow8(unsafe.Pointer(&dst[i]), unsafe.Pointer(&a[i]), unsafe.Pointer(&b[i]))
		i += 8
	}
	if i+4 <= n {
		avgRow4(unsafe.Pointer(&dst[i]), unsafe.Pointer(&a[i]), unsafe.Pointer(&b[i]))
		i += 4
	}
	for ; i < n; i++ {
		dst[i] = uint8(avg2(int(a[i]), int(b[i])))
	}
}

// qpelHorOnly handles fy==0: one horizontal pass per row, then avg.
// The fx branch hoists to the whole block (one switch, not one per row).
func qpelHorOnly(dst, plane []byte, stride, ix, iy, bw, bh, fx int) {
	qpelHorOnlyInto(dst, bw, plane, stride, ix, iy, bw, bh, fx)
}

// qpelHorOnlyInto is the strided entry (S1-W direct-write): output row
// dy lands at dst[dy*dstStride:dy*dstStride+bw] instead of a dense
// block. Same taps, same order, same bytes; the dense entry above is
// one call with dstStride == bw.
func qpelHorOnlyInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fx int) {
	var sums [qpelMaxW]int16
	var hh [qpelMaxW]uint8
	// S1b-HV hor-only block fusion (one call per block kills the
	// per-row Go wrapper: horSumsRow+roundHalfRow ran as two kernel
	// calls plus two Go row loops per block row; the wrappers cost
	// ~90ms per 90f profile vs the SIMD kernels themselves).
	// Same taps, same (s+16)>>5, same clip, same avg — bit-identical
	// to the split row paths (gated by qpel_s1_test.go all-frac pins
	// + TestS1FusedBlockMatchesSplit). Fuse widths 8/16 (the 8-wide
	// hor body reads a full 16-byte window: safe only when bw>=8;
	// bw==4 keeps the split path). arm64 stays on its row path (no
	// block kernel there yet, scalar-exact留守).
	fusedHor := bw == 8 || bw == 16
	switch fx {
	case 2:
		if fusedHor {
			horRoundBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&plane[iy*stride+ix-2]), uintptr(stride),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			horSumsRow(sums[:], plane, stride, ix, ay, bw)
			out := dst[dy*dstStride : dy*dstStride+bw]
			roundHalfRow(out, sums[:], bw)
		}
	case 1:
		if fusedHor {
			var hhB [qpelMaxH][qpelMaxW]uint8
			horRoundBlock(
				unsafe.Pointer(&hhB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[iy*stride+ix-2]), uintptr(stride),
				bw, bh,
			)
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&plane[iy*stride+ix]), uintptr(stride),
				unsafe.Pointer(&hhB[0][0]), uintptr(qpelMaxW),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			horSumsRow(sums[:], plane, stride, ix, ay, bw)
			roundHalfRow(hh[:], sums[:], bw)
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, plane[base:base+bw], hh[:bw])
		}
	default: // fx == 3
		if fusedHor {
			var hhB [qpelMaxH][qpelMaxW]uint8
			horRoundBlock(
				unsafe.Pointer(&hhB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[iy*stride+ix-2]), uintptr(stride),
				bw, bh,
			)
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hhB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[iy*stride+ix+1]), uintptr(stride),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			horSumsRow(sums[:], plane, stride, ix, ay, bw)
			roundHalfRow(hh[:], sums[:], bw)
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hh[:bw], plane[base+1:base+1+bw])
		}
	}
}

// qpelVerOnly handles fx==0: one vertical pass per row, then avg.
// Bulk vertical sums run the SSE kernel 8 columns at a time; rounding
// and the quarter avg stay scalar. The fy branch hoists to the whole
// block (one switch, not one per pixel).
func qpelVerOnly(dst, plane []byte, stride, ix, iy, bw, bh, fy int) {
	qpelVerOnlyInto(dst, bw, plane, stride, ix, iy, bw, bh, fy)
}

// qpelVerOnlyInto is the strided entry (S1-W direct-write): same
// contract as qpelHorOnlyInto, vertical direction.
func qpelVerOnlyInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fy int) {
	var sums [qpelMaxW]int16
	var hv [qpelMaxW]uint8
	// S1b-HV ver-only block fusion (same shape as hor-only above):
	// verRoundBlock covers 4/8/16 with exact-width loads/stores
	// (v4rows uses MOVL/MOVD, no neighbour paint), so bw==4 fuses
	// here too; horRoundBlock overreads on 4-wide, hence the
	// narrower hor gate. avg pairs the hv temp with the plane rows
	// (fy==1: plane row ay; fy==3: plane row ay+1) via avgBlock
	// (4/8/16 exact-width, PAVGB == avg2 bit for bit).
	fusedVer := bw == 4 || bw == 8 || bw == 16
	switch fy {
	case 2:
		if fusedVer {
			verRoundBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&plane[(iy-2)*stride+ix]), uintptr(stride),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			verSumsRow(sums[:], plane, stride, ix, ay, bw)
			out := dst[dy*dstStride : dy*dstStride+bw]
			roundHalfRow(out, sums[:], bw)
		}
	case 1:
		if fusedVer {
			var hvB [qpelMaxH][qpelMaxW]uint8
			verRoundBlock(
				unsafe.Pointer(&hvB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[(iy-2)*stride+ix]), uintptr(stride),
				bw, bh,
			)
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&plane[iy*stride+ix]), uintptr(stride),
				unsafe.Pointer(&hvB[0][0]), uintptr(qpelMaxW),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			verSumsRow(sums[:], plane, stride, ix, ay, bw)
			roundHalfRow(hv[:], sums[:], bw)
			base := ay*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, plane[base:base+bw], hv[:bw])
		}
	default: // fy == 3
		if fusedVer {
			var hvB [qpelMaxH][qpelMaxW]uint8
			verRoundBlock(
				unsafe.Pointer(&hvB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[(iy-2)*stride+ix]), uintptr(stride),
				bw, bh,
			)
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hvB[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[(iy+1)*stride+ix]), uintptr(stride),
				bw, bh,
			)
			return
		}
		for dy := 0; dy < bh; dy++ {
			ay := iy + dy
			verSumsRow(sums[:], plane, stride, ix, ay, bw)
			roundHalfRow(hv[:], sums[:], bw)
			base := (ay+1)*stride + ix
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hv[:bw], plane[base:base+bw])
		}
	}
}

// qpelGeneral handles fx!=0 && fy!=0: rounded halves plus the
// single-rounding center cascade (unrounded horizontal temp, then
// vertical with +512>>10, like centerJ). Vertical halves run the SSE
// kernel; the final quarter combine hoists to one switch per block.
//
// Case-gated passes: only the inputs the frac pair actually combines
// are computed. The pure-center pair (2,2) skips both rounded-half
// passes; half-center pairs skip the unused half; no-center pairs
// skip the temp. Horizontal sums shared between the temp and the
// rounded halves come from one computation (jt rows [2:2+rows] feed
// hh: identical horSumsRow inputs, so bit-identical).
func qpelGeneral(dst, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	qpelGeneralInto(dst, bw, plane, stride, ix, iy, bw, bh, fx, fy)
}

// qpelGeneralInto is the strided entry (S1-W direct-write): the temp
// passes (hh/hv/jt) stay dense stack arrays (consumed in place), only
// the final combine stores stride out. Same bytes as the dense entry.
func qpelGeneralInto(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh, fx, fy int) {
	rows := bh
	if fy == 3 {
		rows++ // hh1 at y+1
	}
	cols := bw
	if fx == 3 {
		cols++ // hv1 at x+1
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
	var hh [qpelMaxH + 1][qpelMaxW]uint8
	var hv [qpelMaxH][qpelMaxW + 1]uint8
	var jt [qpelMaxH + 5][qpelMaxW]int16
	var sums [qpelMaxW]int16
	var vsums [qpelMaxW + 1]int16
	// S1-F fused block kernels (one call per block kills the per-row
	// wrapper overhead: horSumsRow+roundHalfRow ran as two kernel calls
	// plus two Go row loops per block row; the wrappers cost ~360ms
	// per 90f profile vs ~300ms for the SIMD kernels themselves).
	// Same taps, same (s+16)>>5, same clip — bit-identical to the split
	// row paths (TestS1FusedBlockMatchesSplit pins all three fusions).
	// Passes fuse widths 8/16 (the 8-wide row bodies read a full
	// 16-byte window: safe only when w>=8); the corner average fuses
	// 4/8/16 (avgBlock reads exactly w bytes per plane). Odd widths
	// keep the split path.
	fusedW := bw == 8 || bw == 16
	fusedAvgW := bw == 4 || bw == 8 || bw == 16
	if needJT {
		if fusedW {
			// S1-F jt-temp fusion: one call sums the whole bh+5
			// window (same taps as horSumsRow, raw int16 store).
			horSumsBlock(
				unsafe.Pointer(&jt[0][0]), uintptr(qpelMaxW*2),
				unsafe.Pointer(&plane[(iy-2)*stride+ix-2]), uintptr(stride),
				bw, bh+5,
			)
		} else {
			for r := 0; r < bh+5; r++ {
				horSumsRow(jt[r][:], plane, stride, ix, iy-2+r, bw)
			}
		}
	}
	if needHH {
		if needJT {
			// jt[dy+2] sums row iy+dy: same inputs horSumsRow
			// would read, so rounding them equals hh bit for bit.
			if fusedW {
				roundHalfBlock(
					unsafe.Pointer(&hh[0][0]), uintptr(qpelMaxW),
					unsafe.Pointer(&jt[2][0]), uintptr(qpelMaxW*2),
					bw, rows,
				)
			} else {
				for dy := 0; dy < rows; dy++ {
					roundHalfRow(hh[dy][:], jt[dy+2][:], bw)
				}
			}
		} else if fusedW {
			horRoundBlock(
				unsafe.Pointer(&hh[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&plane[iy*stride+ix-2]), uintptr(stride),
				bw, rows,
			)
		} else {
			for dy := 0; dy < rows; dy++ {
				horSumsRow(sums[:], plane, stride, ix, iy+dy, bw)
				roundHalfRow(hh[dy][:], sums[:], bw)
			}
		}
	}
	if needHV {
		if fusedW && (cols == bw || cols == bw+1) {
			verRoundBlock(
				unsafe.Pointer(&hv[0][0]), uintptr(qpelMaxW+1),
				unsafe.Pointer(&plane[(iy-2)*stride+ix]), uintptr(stride),
				bw, bh,
			)
			// S1-F corner tail (fx==3 pairs need hv1 at x+1, so one
			// extra column): the fused kernel covers the first bw
			// columns; the last column runs the split scalar path
			// (1-wide: verSumsRow/roundHalfRow skip their kernels and
			// run bit-for-bit scalar). Corner pairs are 55% of general
			// on 1080p, so leaving them split would strand most of the
			// vertical wrapper win. Taps stay inside the frame by the
			// same interior gate (ix+bw+4<=w covers taps ix+bw-2..+3).
			if cols == bw+1 {
				for dy := 0; dy < bh; dy++ {
					ay := iy + dy
					verSumsRow(vsums[:], plane, stride, ix+bw, ay, 1)
					roundHalfRow(hv[dy][bw:bw+1], vsums[:], 1)
				}
			}
		} else {
			for dy := 0; dy < bh; dy++ {
				ay := iy + dy
				verSumsRow(vsums[:], plane, stride, ix, ay, cols)
				roundHalfRow(hv[dy][:], vsums[:], cols)
			}
		}
	}
	switch {
	case fx == 2 && fy == 2:
		// S1-Q center entry: shared two-pass helper (one call site;
		// the 32-bit fused kernel plugs in here when written).
		centerRows8Into(dst, dstStride, plane, stride, ix, iy, bw, bh)
		return
	case fx == 1 && fy == 1:
		// S1b-T corner fusion (one call per block): the fused
		// passes already materialized hh/hv; averaging them per
		// row cost a call + Go loop + slices per row. FusedW
		// covers the live widths (4/8/16); odd widths keep rows.
		if fusedAvgW {
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hh[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&hv[0][0]), uintptr(qpelMaxW+1),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hh[dy][:bw], hv[dy][:bw])
		}
	case fx == 2 && fy == 1:
		// S1b-V block fusion (one call per block): the dx-group
		// loop cost a call + 6 index mults per 4 pixels; the
		// block kernel loops in-asm. FusedAvgW covers live widths;
		// odd widths keep the row path + scalar tail.
		if fusedAvgW {
			centerAvgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&jt[0][0]),
				unsafe.Pointer(&hh[0][0]), uintptr(qpelMaxW),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			dx := 0
			for ; dx+4 <= bw; dx += 4 {
				centerRowAvg4(out[dx:dx+4], &jt, dy, dx, hh[dy][dx:dx+4])
			}
			for ; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy][dx]), centerAt(&jt, dy, dx)))
			}
		}
	case fx == 3 && fy == 1:
		if fusedAvgW {
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hh[0][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&hv[0][1]), uintptr(qpelMaxW+1),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hh[dy][:bw], hv[dy][1:bw+1])
		}
	case fx == 1 && fy == 2:
		if fusedAvgW {
			centerAvgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&jt[0][0]),
				unsafe.Pointer(&hv[0][0]), uintptr(qpelMaxW+1),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			dx := 0
			for ; dx+4 <= bw; dx += 4 {
				centerRowAvg4(out[dx:dx+4], &jt, dy, dx, hv[dy][dx:dx+4])
			}
			for ; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hv[dy][dx]), centerAt(&jt, dy, dx)))
			}
		}
	case fx == 3 && fy == 2:
		if fusedAvgW {
			centerAvgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&jt[0][0]),
				unsafe.Pointer(&hv[0][1]), uintptr(qpelMaxW+1),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			dx := 0
			for ; dx+4 <= bw; dx += 4 {
				centerRowAvg4(out[dx:dx+4], &jt, dy, dx, hv[dy][dx+1:dx+5])
			}
			for ; dx < bw; dx++ {
				out[dx] = uint8(avg2(centerAt(&jt, dy, dx), int(hv[dy][dx+1])))
			}
		}
	case fx == 1 && fy == 3:
		if fusedAvgW {
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hv[0][0]), uintptr(qpelMaxW+1),
				unsafe.Pointer(&hh[1][0]), uintptr(qpelMaxW),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hv[dy][:bw], hh[dy+1][:bw])
		}
	case fx == 2 && fy == 3:
		if fusedAvgW {
			centerAvgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&jt[0][0]),
				unsafe.Pointer(&hh[1][0]), uintptr(qpelMaxW),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			dx := 0
			for ; dx+4 <= bw; dx += 4 {
				centerRowAvg4(out[dx:dx+4], &jt, dy, dx, hh[dy+1][dx:dx+4])
			}
			for ; dx < bw; dx++ {
				out[dx] = uint8(avg2(int(hh[dy+1][dx]), centerAt(&jt, dy, dx)))
			}
		}
	default: // fx == 3 && fy == 3
		if fusedAvgW {
			avgBlock(
				unsafe.Pointer(&dst[0]), uintptr(dstStride),
				unsafe.Pointer(&hh[1][0]), uintptr(qpelMaxW),
				unsafe.Pointer(&hv[0][1]), uintptr(qpelMaxW+1),
				bw, bh,
			)
			break
		}
		for dy := 0; dy < bh; dy++ {
			out := dst[dy*dstStride : dy*dstStride+bw]
			avgRowInto(out, hh[dy+1][:bw], hv[dy][1:bw+1])
		}
	}
}

// centerRowAvg4 writes 4 center+avg outputs (dy, dx..dx+3) through
// the fused kernel: same row pointers as centerRow4, avg plane bytes
// passed straight through. Bit-identical to centerRow4+avgRowInto
// (same taps, same rounds). Zero-alloc: noescape kernel, no heap.
func centerRowAvg4(out []uint8, jt *[qpelMaxH + 5][qpelMaxW]int16, dy, dx int, avg []uint8) {
	// Unrolled row pointers (no closure, no heap: plain arithmetic).
	base := unsafe.Pointer(&jt[0][0])
	r0 := unsafe.Add(base, ((dy)*qpelMaxW+dx)*2)
	r1 := unsafe.Add(base, ((dy+1)*qpelMaxW+dx)*2)
	r2 := unsafe.Add(base, ((dy+2)*qpelMaxW+dx)*2)
	r3 := unsafe.Add(base, ((dy+3)*qpelMaxW+dx)*2)
	r4 := unsafe.Add(base, ((dy+4)*qpelMaxW+dx)*2)
	r5 := unsafe.Add(base, ((dy+5)*qpelMaxW+dx)*2)
	centerAvg4(unsafe.Pointer(&out[0]), r0, r1, r2, r3, r4, r5, unsafe.Pointer(&avg[0]))
}

// the vector kernel. Row pointers come from the jt base without
// bounds checks (unsafe.Add on the contiguous window: dy+5 rows and
// dx+4 columns are inside by construction — the caller guarantees
// dy+bh+5 <= len(jt) and dx+bw <= qpelMaxW, same as the scalar loops).
// Bit-identical to 4x centerAt (the gate test pins the full int16
// input range: 32-bit math never wraps).
func centerRow4(out []uint8, jt *[qpelMaxH + 5][qpelMaxW]int16, dy, dx int) {
	// Unrolled row pointers (no closure, no heap: plain arithmetic).
	base := unsafe.Pointer(&jt[0][0])
	r0 := unsafe.Add(base, ((dy)*qpelMaxW+dx)*2)
	r1 := unsafe.Add(base, ((dy+1)*qpelMaxW+dx)*2)
	r2 := unsafe.Add(base, ((dy+2)*qpelMaxW+dx)*2)
	r3 := unsafe.Add(base, ((dy+3)*qpelMaxW+dx)*2)
	r4 := unsafe.Add(base, ((dy+4)*qpelMaxW+dx)*2)
	r5 := unsafe.Add(base, ((dy+5)*qpelMaxW+dx)*2)
	centerVec4(unsafe.Pointer(&out[0]), r0, r1, r2, r3, r4, r5)
}

// centerAt rounds one center-cascade column like centerJ:
// vertical 6-tap over unrounded horizontal sums, single +512>>10 step.
func centerAt(jt *[qpelMaxH + 5][qpelMaxW]int16, dy, dx int) int {
	s := int(jt[dy][dx]) + int(jt[dy+5][dx]) -
		5*(int(jt[dy+1][dx])+int(jt[dy+4][dx])) +
		20*(int(jt[dy+2][dx])+int(jt[dy+3][dx]))
	return clipU8((s + 512) >> 10)
}

// centerRows8 filters the (2,2) pure-center path: 8 columns per row
// through the horizontal kernel, then the vertical cascade in Go over
// the shared jt window. Two-pass shape (kept, 2026-09-22): fusing the
// vertical 6-tap into the row kernel is NOT bit-safe in 16 lanes —
// horizontal sums reach ±10710 per row and six of them sum to ~±200k,
// far outside int16, so PMULLW wraps (measured: true 57600 -> 8).
// A widened 32-bit vertical combine (PMOVSXWD+PMADDWD/SSE4.1) is the
// open route; until then the temp rows stay in Go (zero-alloc stack).
func centerRows8(dst, plane []byte, stride, ix, iy, bw, bh int) {
	centerRows8Into(dst, bw, plane, stride, ix, iy, bw, bh)
}

// centerRows8Into is the strided entry (S1-W direct-write): same jt
// window, final stores stride out.
func centerRows8Into(dst []byte, dstStride int, plane []byte, stride, ix, iy, bw, bh int) {
	var jt [qpelMaxH + 5][qpelMaxW]int16
	for r := 0; r < bh+5; r++ {
		horSumsRow(jt[r][:], plane, stride, ix, iy-2+r, bw)
	}
	for dy := 0; dy < bh; dy++ {
		out := dst[dy*dstStride : dy*dstStride+bw]
		dx := 0
		for ; dx+4 <= bw; dx += 4 {
			centerRow4(out[dx:dx+4], &jt, dy, dx)
		}
		for ; dx < bw; dx++ {
			out[dx] = uint8(centerAt(&jt, dy, dx))
		}
	}
}
