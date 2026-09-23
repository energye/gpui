//go:build amd64

package h264

import (
	"math/rand"
	"testing"
	"unsafe"
)

// S1-F pin: the withdrawn fused block kernels stay bit-identical to
// the split row paths (a future attempt can reuse the shape without
// re-deriving the taps). Covers the two live widths (8, 16) for the
// horizontal and vertical sums+round fusion plus the jt-temp round
// fusion, over stimulus and edge-heavy (0/255 runs) planes.
func TestS1FusedBlockMatchesSplit(t *testing.T) {
	mkPlane := func(edge bool) []byte {
		W, H := 64, 64
		plane := make([]byte, W*H)
		for y := 0; y < H; y++ {
			for x := 0; x < W; x++ {
				if edge {
					if (x+y)&1 == 0 {
						plane[y*W+x] = 0
					} else {
						plane[y*W+x] = 255
					}
				} else {
					plane[y*W+x] = byte((x*37 + y*11 + x*y) & 0xFF)
				}
			}
		}
		return plane
	}
	for _, edge := range []bool{false, true} {
		plane := mkPlane(edge)
		const W = 64
		for _, bw := range []int{8, 16} {
			const bh = 8
			// hor fused vs split.
			var hhSplit [17][16]uint8
			var hhFused [17][16]uint8
			var sums [16]int16
			for dy := 0; dy < bh; dy++ {
				horSumsRow(sums[:], plane, W, 20, 20+dy, bw)
				roundHalfRow(hhSplit[dy][:], sums[:], bw)
			}
			horRoundBlock(unsafe.Pointer(&hhFused[0][0]), uintptr(16),
				unsafe.Pointer(&plane[20*W+20-2]), uintptr(W), bw, bh)
			for dy := 0; dy < bh; dy++ {
				for dx := 0; dx < bw; dx++ {
					if hhSplit[dy][dx] != hhFused[dy][dx] {
						t.Fatalf("edge=%v bw=%d hor dy=%d dx=%d split=%d fused=%d",
							edge, bw, dy, dx, hhSplit[dy][dx], hhFused[dy][dx])
					}
				}
			}
			// ver fused vs split.
			var hvSplit [16][17]uint8
			var hvFused [16][17]uint8
			var vs [17]int16
			for dy := 0; dy < bh; dy++ {
				verSumsRow(vs[:], plane, W, 20, 20+dy, bw)
				roundHalfRow(hvSplit[dy][:], vs[:], bw)
			}
			verRoundBlock(unsafe.Pointer(&hvFused[0][0]), uintptr(17),
				unsafe.Pointer(&plane[18*W+20]), uintptr(W), bw, bh)
			for dy := 0; dy < bh; dy++ {
				for dx := 0; dx < bw; dx++ {
					if hvSplit[dy][dx] != hvFused[dy][dx] {
						t.Fatalf("edge=%v bw=%d ver dy=%d dx=%d split=%d fused=%d",
							edge, bw, dy, dx, hvSplit[dy][dx], hvFused[dy][dx])
					}
				}
			}
			// jt-temp horSums fusion vs split, then round fusion vs split.
			var jt [21][16]int16
			var jtFused [21][16]int16
			for r := 0; r < bh+5; r++ {
				horSumsRow(jt[r][:], plane, W, 20, 18+r, bw)
			}
			horSumsBlock(unsafe.Pointer(&jtFused[0][0]), uintptr(32),
				unsafe.Pointer(&plane[18*W+20-2]), uintptr(W), bw, bh+5)
			for r := 0; r < bh+5; r++ {
				for dx := 0; dx < bw; dx++ {
					if jt[r][dx] != jtFused[r][dx] {
						t.Fatalf("edge=%v bw=%d jt-hor r=%d dx=%d split=%d fused=%d",
							edge, bw, r, dx, jt[r][dx], jtFused[r][dx])
					}
				}
			}
			var hh2Split [17][16]uint8
			var hh2Fused [17][16]uint8
			for dy := 0; dy < bh; dy++ {
				roundHalfRow(hh2Split[dy][:], jt[dy+2][:], bw)
			}
			roundHalfBlock(unsafe.Pointer(&hh2Fused[0][0]), uintptr(16),
				unsafe.Pointer(&jt[2][0]), uintptr(32), bw, bh)
			for dy := 0; dy < bh; dy++ {
				for dx := 0; dx < bw; dx++ {
					if hh2Split[dy][dx] != hh2Fused[dy][dx] {
						t.Fatalf("edge=%v bw=%d jt dy=%d dx=%d split=%d fused=%d",
							edge, bw, dy, dx, hh2Split[dy][dx], hh2Fused[dy][dx])
					}
				}
			}
		}
	}
}

// S1b-V pin: centerAvgBlock equals centerRowAvg4+avgRowInto bit for
// bit on every live width (4/8/16) x height (4/8/16). The jt window
// spans the live hor-sum band plus full-int16 extremes (32-bit math
// never wraps); the avg plane spans all byte pairs (PAVGB rounding
// in every direction). Strided dst (dstStride > w) pins the row-end
// pointer math; the +1 column offset pins the (3,2)/(2,3) hv1 call
// shape. Zero-alloc: one noescape call, no heap.
func TestS1CenterAvgBlockMatchesRow(t *testing.T) {
	r := rand.New(rand.NewSource(20260923))
	for _, edge := range []bool{false, true} {
		var jt [qpelMaxH + 5][qpelMaxW]int16
		var avg [qpelMaxH + 1][qpelMaxW + 1]uint8
		for y := range jt {
			for x := range jt[y] {
				switch r.Intn(4) {
				case 0:
					jt[y][x] = int16(r.Intn(21421) - 10710)
				case 1:
					jt[y][x] = int16(int32(r.Uint32()))
				case 2:
					if r.Intn(2) == 0 {
						jt[y][x] = -32768
					} else {
						jt[y][x] = 32767
					}
				default:
					jt[y][x] = 0
				}
			}
		}
		for y := range avg {
			for x := range avg[y] {
				if edge {
					if (x+y)&1 == 0 {
						avg[y][x] = 0
					} else {
						avg[y][x] = 255
					}
				} else {
					avg[y][x] = uint8(r.Intn(256))
				}
			}
		}
		for _, bw := range []int{4, 8, 16} {
			for _, bh := range []int{4, 8, 16} {
				for _, off := range []int{0, 1} {
					dstStride := bw + 3
					dst := make([]byte, dstStride*bh)
					want := make([]byte, dstStride*bh)
					centerAvgBlock(
						unsafe.Pointer(&dst[0]), uintptr(dstStride),
						unsafe.Pointer(&jt[0][0]),
						unsafe.Pointer(&avg[0][off]), uintptr(qpelMaxW+1),
						bw, bh,
					)
					for dy := 0; dy < bh; dy++ {
						var cc [4]uint8
						dx := 0
						for ; dx+4 <= bw; dx += 4 {
							centerRow4(cc[:], &jt, dy, dx)
							avgRowInto(want[dy*dstStride+dx:dy*dstStride+dx+4], avg[dy][off+dx:off+dx+4], cc[:])
						}
						for ; dx < bw; dx++ {
							want[dy*dstStride+dx] = uint8(avg2(int(avg[dy][off+dx]), centerAt(&jt, dy, dx)))
						}
					}
					for i := range dst {
						if dst[i] != want[i] {
							t.Fatalf("edge=%v bw=%d bh=%d off=%d byte %d block=%d row=%d",
								edge, bw, bh, off, i, dst[i], want[i])
						}
					}
				}
			}
		}
	}
	// Store width: rows after bh and columns after bw stay sentinel.
	var jt2 [qpelMaxH + 5][qpelMaxW]int16
	var avg2p [qpelMaxH + 1][qpelMaxW + 1]uint8
	sentinel := make([]byte, 20*16)
	for i := range sentinel {
		sentinel[i] = 0xA5
	}
	centerAvgBlock(
		unsafe.Pointer(&sentinel[0]), uintptr(20),
		unsafe.Pointer(&jt2[0][0]),
		unsafe.Pointer(&avg2p[0][0]), uintptr(qpelMaxW+1),
		4, 4,
	)
	for row := 0; row < 4; row++ {
		for dx := 4; dx < 20; dx++ {
			if sentinel[row*20+dx] != 0xA5 {
				t.Fatalf("row %d byte %d overwritten: %#x", row, dx, sentinel[row*20+dx])
			}
		}
	}
	for i := 4 * 20; i < len(sentinel); i++ {
		if sentinel[i] != 0xA5 {
			t.Fatalf("tail byte %d overwritten: %#x", i, sentinel[i])
		}
	}
	// Zero-alloc on every live width.
	for _, bw := range []int{4, 8, 16} {
		dst := make([]byte, bw*8)
		if n := testing.AllocsPerRun(20, func() {
			centerAvgBlock(
				unsafe.Pointer(&dst[0]), uintptr(bw),
				unsafe.Pointer(&jt2[0][0]),
				unsafe.Pointer(&avg2p[0][0]), uintptr(qpelMaxW+1),
				bw, 8,
			)
		}); n != 0 {
			t.Fatalf("bw=%d centerAvgBlock allocs = %v want 0", bw, n)
		}
	}
}
