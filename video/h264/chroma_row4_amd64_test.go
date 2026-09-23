//go:build amd64

package h264

import (
	"testing"
)

// S1b-A2 gate: the 4-wide chroma row kernel equals the interior scalar
// bit for bit on every frac pair, and stores exactly 4 bytes per row
// (no overwrite of the right neighbour — the centerVec4 MOVQ lesson,
// 2026-09-22). Covers 4x4, 4x8, and 12x4 (8+4 in one row, both kernels).
func TestS1ChromaRow4MatchesScalar(t *testing.T) {
	const W, H = 64, 64
	plane := chromaStimulus(W, H)
	for _, sz := range [][2]int{{4, 4}, {4, 8}, {12, 4}} {
		cw, ch := sz[0], sz[1]
		for fx := 0; fx < 8; fx++ {
			for fy := 0; fy < 8; fy++ {
				if fx == 0 && fy == 0 {
					continue // integer path never reaches the kernel
				}
				got := make([]byte, cw*ch)
				want := make([]byte, cw*ch)
				ok := chromaBlock(got, plane, W, 20, 20, cw, ch, fx, fy)
				if !ok {
					t.Fatalf("%dx%d frac=%d,%d interior should take the fast path", cw, ch, fx, fy)
				}
				chromaInteriorScalar(want, plane, W, 20, 20, cw, ch, fx, fy)
				for i := range got {
					if got[i] != want[i] {
						t.Fatalf("%dx%d frac=%d,%d byte %d kernel=%d scalar=%d",
							cw, ch, fx, fy, i, got[i], want[i])
					}
				}
			}
		}
	}
	// Store width: 4x4 into a sentinel buffer must not touch byte 4..7
	// of any row (the 8-byte store bug would paint duplicates there).
	sentinel := make([]byte, 8*8)
	for i := range sentinel {
		sentinel[i] = 0xA5
	}
	out := sentinel
	// Use dstStride 8 with cw=4: rows at 0,8,16,24; bytes 4..7 per row
	// must stay sentinel.
	if !chromaBlockInto(out, 8, plane, W, 20, 20, 4, 4, 3, 5) {
		t.Fatalf("4x4 interior should take the fast path")
	}
	for row := 0; row < 4; row++ {
		for dx := 4; dx < 8; dx++ {
			if out[row*8+dx] != 0xA5 {
				t.Fatalf("row %d byte %d overwritten: %#x (store wider than 4)", row, dx, out[row*8+dx])
			}
		}
	}
	// Zero-alloc on the 4x4 path (one asm call per row, no heap).
	tmp := make([]byte, 16)
	if n := testing.AllocsPerRun(100, func() {
		chromaBlock(tmp, plane, W, 20, 20, 4, 4, 3, 5)
	}); n != 0 {
		t.Fatalf("4x4 chroma allocs = %v want 0", n)
	}
}
