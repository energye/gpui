package h264

import (
	"testing"
)

// S1b-E gate: the row-band edge path equals a pure per-pixel reference
// bit for bit. The reference spells the floor quarter-pel split plus
// lumaSample inline (no shared helper with the production path, so a
// bug in scalarPel cannot hide). Covers every frac, every partition
// size, and placements that force every shape: fully interior rows,
// top/bottom/left/right rims, corners, negative motion, and motion
// that pushes taps far outside (empty run on every row).
func TestS1EdgeRowBandMatchesPure(t *testing.T) {
	const W, H = 64, 64
	plane := qpelStimulus(W, H)
	sizes := [][2]int{{4, 4}, {8, 4}, {4, 8}, {8, 8}, {16, 8}, {8, 16}, {16, 16}}
	placements := []struct {
		px, py int
		mx, my int16
	}{
		{0, 0, 1, 1},      // top-left corner block
		{0, 0, -4, -4},    // taps far outside
		{1, 1, 2, 2},      // near-edge rims
		{W - 16, 0, 7, 1}, // top-right corner block
		{0, H - 8, 1, 7},  // bottom-left
		{W - 8, H - 8, -1, -1},
		{20, 0, 1, -5}, // top rim, negative vertical motion
		{0, 20, -5, 1}, // left rim, negative horizontal motion
		{20, 20, -6, -6},
		{5, 40, 13, -9}, // mixed rims both dims
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
					got := make([]byte, bw*bh)
					predictLumaBlockScalarInto(plane, W, H, px, py, bw, bh, mx, my, got, bw)
					for dy := 0; dy < bh; dy++ {
						for dx := 0; dx < bw; dx++ {
							qx := (px+dx)*4 + int(mx)
							qy := (py+dy)*4 + int(my)
							ix := qx >> 2
							iy := qy >> 2
							want := uint8(lumaSample(plane, W, H, ix, iy, qx-(ix<<2), qy-(iy<<2)))
							if got[dy*bw+dx] != want {
								t.Fatalf("%dx%d frac=%d,%d at (%d,%d) mv=(%d,%d) pel=(%d,%d) got=%d want=%d",
									bw, bh, fx, fy, px, py, mx, my, dx, dy, got[dy*bw+dx], want)
							}
						}
					}
				}
			}
		}
	}
}
