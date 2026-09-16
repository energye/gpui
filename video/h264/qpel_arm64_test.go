//go:build arm64

package h264

import "testing"

// S1 arm64 NEON qpel gate: the NEON block path equals the scalar留守
// bit for bit on every frac, every partition size, interior and edge.
// Same stimulus and placements as TestS1QpelDispatchMatchesScalar; the
// only difference is the entry (predictLumaBlockArm). Edges must fall
// back and still match. Runs only where the NEON kernel exists (here);
// amd64 keeps its own gate untouched.
func TestS1QpelArmMatchesScalar(t *testing.T) {
	const W, H = 64, 64
	plane := qpelStimulus(W, H)
	sizes := [][2]int{{4, 4}, {8, 4}, {4, 8}, {8, 8}, {16, 8}, {8, 16}, {16, 16}}
	placements := []struct {
		px, py int
		mx, my int16
	}{
		{20, 20, 1, 1},
		{0, 0, 1, 1},
		{0, 0, -4, -4},
		{1, 1, 2, 2},
		{20, 20, -6, -6},
	}
	for _, sz := range sizes {
		bw, bh := sz[0], sz[1]
		for fx := 0; fx < 4; fx++ {
			for fy := 0; fy < 4; fy++ {
				for _, pl := range placements {
					mx := int16(pl.mx&^3) | int16(fx)
					my := int16(pl.my&^3) | int16(fy)
					px, py := pl.px, pl.py
					if px+bw > W {
						px = W - bw
					}
					if py+bh > H {
						py = H - bh
					}
					a := make([]byte, bw*bh)
					b := make([]byte, bw*bh)
					predictLumaBlockArm(&Picture{Width: W, Height: H, Y: plane}, px, py, bw, bh, mx, my, a)
					predictLumaBlockScalar(plane, W, H, px, py, bw, bh, mx, my, b)
					for i := range a {
						if a[i] != b[i] {
							t.Fatalf("%dx%d frac=%d,%d at (%d,%d) mv=(%d,%d) byte %d neon=%d scalar=%d",
								bw, bh, fx, fy, px, py, mx, my, i, a[i], b[i])
						}
					}
				}
			}
		}
	}
}

// NEON block-path coverage: interior blocks report true (the kernel
// runs), edge blocks report false everywhere (留守 clips).
func TestS1QpelArmBlockTaken(t *testing.T) {
	const W, H = 64, 64
	plane := qpelStimulus(W, H)
	interior := qpelBlock(make([]byte, 16*16), plane, W, H, 20, 20, 16, 16, 1, 1)
	edge := qpelBlock(make([]byte, 16*16), plane, W, H, 0, 0, 16, 16, 1, 1)
	if edge {
		t.Fatal("edge block must fall back to the scalar留守")
	}
	if !interior {
		t.Fatal("interior block must take the NEON path on arm64")
	}
}

// Interior NEON dispatch allocates zero (hot path: no per-block heap).
func TestS1QpelArmZeroAlloc(t *testing.T) {
	const W, H = 1280, 720
	plane := qpelStimulus(W, H)
	ref := &Picture{Width: W, Height: H, Y: plane}
	for _, frac := range [][2]int16{{2, 0}, {0, 2}, {2, 2}, {1, 1}, {3, 3}, {1, 2}} {
		out := make([]byte, 16*16)
		mx, my := frac[0], frac[1]
		if n := testing.AllocsPerRun(20, func() {
			predictLumaBlockArm(ref, 100, 100, 16, 16, mx, my, out)
		}); n != 0 {
			t.Fatalf("frac=%d,%d allocs = %v want 0", mx, my, n)
		}
	}
}
