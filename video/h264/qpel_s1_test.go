package h264

// S1-I1 gate: qpel dispatch equals the scalar留守 bit for bit.
// Peer: ffmpeg libavcodec/h264qpel_template.c:317-460 H264_QPEL macro
// family (row-run horizontal pass + vertical/avg combine, our
// qpel_amd64.go skeleton) vs our qpel_amd64.s row kernel; oracle is the
// scalar留守 itself (no new vectors here, VR2 exact pins pixels).
// Pass lines: all 16 fracs x partition sizes x interior/edge agree;
// interior 16x16 dispatch allocates zero; VR2 exact stays green.

import (
	"runtime"
	"testing"
)

// qpelStimulus is one deterministic reference plane (stimulus only;
// oracle is predictLumaBlockScalar, so no golden file needed).
func qpelStimulus(w, h int) []byte {
	p := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p[y*w+x] = uint8((x*7 + y*13 + (x * y & 0xFF)) & 0xFF)
		}
	}
	return p
}

// Dispatch (SIMD or scalar fallback) agrees with the scalar留守 on every
// frac, every partition size, interior and edge-touching positions
// (edges must fall back and still match).
func TestS1QpelDispatchMatchesScalar(t *testing.T) {
	const W, H = 64, 64
	plane := qpelStimulus(W, H)
	sizes := [][2]int{{4, 4}, {8, 4}, {4, 8}, {8, 8}, {16, 8}, {8, 16}, {16, 16}}
	// (px,py,mx,my): interior, top-left edge, bottom-right edge,
	// near-edge fallback, negative-motion fallback.
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
					// Motion whose frac is (fx,fy): base 0 keeps the
					// placement's edge/interior character.
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
					predictLumaBlock(&Picture{Width: W, Height: H, Y: plane}, px, py, bw, bh, mx, my, a)
					predictLumaBlockScalar(plane, W, H, px, py, bw, bh, mx, my, b)
					for i := range a {
						if a[i] != b[i] {
							t.Fatalf("%dx%d frac=%d,%d at (%d,%d) mv=(%d,%d) byte %d dispatch=%d scalar=%d",
								bw, bh, fx, fy, px, py, mx, my, i, a[i], b[i])
						}
					}
				}
			}
		}
	}
}

// qpelBlock fast-path coverage: interior blocks report true on amd64
// (the kernel runs), edge blocks report false everywhere (留守 clips).
func TestS1QpelBlockTaken(t *testing.T) {
	const W, H = 64, 64
	plane := qpelStimulus(W, H)
	interior := qpelBlock(make([]byte, 16*16), plane, W, H, 20, 20, 16, 16, 1, 1)
	edge := qpelBlock(make([]byte, 16*16), plane, W, H, 0, 0, 16, 16, 1, 1)
	if edge {
		t.Fatal("edge block must fall back to the scalar留守")
	}
	if runtime.GOARCH == "amd64" && !interior {
		t.Fatal("interior block must take the fast path on amd64")
	}
	if runtime.GOARCH != "amd64" && interior {
		t.Fatalf("%s has no kernel yet (arm64随后), must stay scalar", runtime.GOARCH)
	}
}

// Interior dispatch allocates zero (hot path: no per-block heap).
func TestS1QpelZeroAlloc(t *testing.T) {
	const W, H = 1280, 720
	plane := qpelStimulus(W, H)
	ref := &Picture{Width: W, Height: H, Y: plane}
	for _, frac := range [][2]int16{{2, 0}, {0, 2}, {2, 2}, {1, 1}, {3, 3}, {1, 2}} {
		out := make([]byte, 16*16)
		mx, my := frac[0], frac[1]
		if n := testing.AllocsPerRun(20, func() {
			predictLumaBlock(ref, 100, 100, 16, 16, mx, my, out)
		}); n != 0 {
			t.Fatalf("frac=%d,%d allocs = %v want 0", mx, my, n)
		}
	}
}

// Math baseline (no threads): one 16x16 block through the scalar留守
// vs the S1 dispatch. Reports the S1-I1 multiple per frac family.
func BenchmarkS1QpelScalarVsDispatch(b *testing.B) {
	const W, H = 1280, 720
	plane := qpelStimulus(W, H)
	ref := &Picture{Width: W, Height: H, Y: plane}
	for _, tc := range []struct {
		name   string
		mx, my int16
	}{
		{"halfH", 2, 0},
		{"halfV", 0, 2},
		{"center", 2, 2},
		{"qpel11", 1, 1},
		{"qpel33", 3, 3},
	} {
		out := make([]byte, 16*16)
		b.Run(tc.name+"/scalar", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				predictLumaBlockScalar(plane, W, H, 100, 100, 16, 16, tc.mx, tc.my, out)
			}
		})
		b.Run(tc.name+"/dispatch", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				predictLumaBlock(ref, 100, 100, 16, 16, tc.mx, tc.my, out)
			}
		})
	}
}
