package h264

// S1b-A3 gate: residual add equals the SetY scalar loop bit for bit.
// Oracle is the pre-S1b formula (pred+res+clipPixel per pixel, no new
// vectors here, VR2 exact pins pixels).

import (
	"testing"
)

// addResidScalarOracle runs the pre-S1b scalar verbatim on a plane.
func addResidScalarOracle(pic []uint8, stride uint32, bx, by uint32, pred []uint8, predStride int, res []int32, w, h int) {
	for y := 0; y < h; y++ {
		base := (by+uint32(y))*stride + bx
		for x := 0; x < w; x++ {
			v := int32(pred[y*predStride+x]) + res[y*w+x]
			pic[base+uint32(x)] = clipPixel(v)
		}
	}
}

func TestS1AddResidMatchesScalar(t *testing.T) {
	for _, w := range []int{4, 8} {
		for _, h := range []int{4, 8} {
			const stride = 64
			picA := make([]uint8, stride*stride)
			picB := make([]uint8, stride*stride)
			for i := range picA {
				picA[i] = uint8((i * 7) & 0xFF)
				picB[i] = picA[i]
			}
			pred := make([]uint8, (h-1)*16+w)
			res := make([]int32, w*h)
			for i := range pred {
				pred[i] = uint8((i*13 + 5) & 0xFF)
			}
			for i := range res {
				// Span negative, zero, positive, and clip both sides.
				res[i] = int32((i*37)%600 - 300)
			}
			positions := [][2]uint32{{0, 0}, {3, 5}, {16, 16}, {uint32(64 - w), uint32(64 - h)}}
			for _, pos := range positions {
				bx, by := pos[0], pos[1]
				addResidScalarOracle(picA, stride, bx, by, pred, 16, res, w, h)
				if !addResidBlock(picB, stride, bx, by, pred, 16, res, w, h) {
					// Edge fallback: run oracle on B too.
					addResidScalarOracle(picB, stride, bx, by, pred, 16, res, w, h)
				}
				for i := range picA {
					if picA[i] != picB[i] {
						t.Fatalf("w=%d h=%d at (%d,%d) byte %d got=%d want=%d",
							w, h, bx, by, i, picB[i], picA[i])
					}
				}
			}
		}
	}
}

// TestS1AddResidEdgeFallsBack pins the crop-border guard: blocks past
// the buffer report false so callers keep the clipping SetY loop.
func TestS1AddResidEdgeFallsBack(t *testing.T) {
	pic := make([]uint8, 64*64)
	pred := make([]uint8, 8*8)
	res := make([]int32, 8*8)
	if addResidBlock(pic, 64, 61, 61, pred, 8, res, 8, 8) {
		t.Fatalf("overhang block should fall back to SetY")
	}
	if !addResidBlock(pic, 64, 0, 0, pred, 8, res, 8, 8) {
		t.Fatalf("interior block should take the fast path")
	}
}

func TestS1AddResidZeroAlloc(t *testing.T) {
	pic := make([]uint8, 64*64)
	pred := make([]uint8, 8*8)
	res := make([]int32, 8*8)
	for i := range res {
		res[i] = int32(i - 32)
	}
	n := testing.AllocsPerRun(100, func() {
		addResidBlock(pic, 64, 8, 8, pred, 8, res, 8, 8)
	})
	if n != 0 {
		t.Fatalf("addresid 8x8 allocs = %v, want 0", n)
	}
}

func BenchmarkS1AddResidScalarVsDispatch(b *testing.B) {
	pic := make([]uint8, 64*64)
	pred := make([]uint8, 8*8)
	res := make([]int32, 8*8)
	for i := range res {
		res[i] = int32(i - 32)
	}
	b.Run("dispatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			addResidBlock(pic, 64, 8, 8, pred, 8, res, 8, 8)
		}
	})
	b.Run("scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			addResidScalarOracle(pic, 64, 8, 8, pred, 8, res, 8, 8)
		}
	})
}
