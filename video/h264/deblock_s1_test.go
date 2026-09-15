package h264

// S1-D1 gate: deblock dispatch equals the scalar留守 bit for bit.
// Peer: ffmpeg libavcodec/x86/h264_deblock.asm (SIMD instance) vs our
// deblock_amd64.s 4-line kernels; oracle is the scalar留守 itself
// (no new vectors here, VR2 exact pins pixels).
// Pass lines: bS 0-4 x both directions x QP range x edge/interior agree;
// taken only on amd64 with bS > 0; hot path allocates zero; VR2 exact
// stays green.

import (
	"os"
	"runtime"
	"testing"

	"github.com/energye/gpui/video/mp4"
)

// deblockStimulus builds two deterministic planes: a smooth gradient
// (gates pass, strong/weak paths write) and a sharp step field (gates
// reject, output must equal input).
func deblockStimulus(w, h, seed int) []byte {
	p := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if seed == 0 {
				p[y*w+x] = uint8((x*3 + y*5 + (x&15)*7) & 0xFF)
			} else if (x/8+y/8)%2 == 0 {
				p[y*w+x] = uint8(30 + (x+y)&31)
			} else {
				p[y*w+x] = uint8(200 + (x*3-y)&31)
			}
		}
	}
	return p
}

// Dispatch (SIMD or scalar fallback) agrees with the scalar留守 on every
// strength class, both directions, the full QP range, and MB-edge plus
// interior positions.
func TestS1DeblockDispatchMatchesScalar(t *testing.T) {
	const W, H = 64, 64
	type placement struct {
		ex, ey   int
		vertical bool
	}
	placements := []placement{
		{32, 8, false},  // interior vertical edge
		{16, 0, false},  // top-row vertical edge
		{60, 60, false}, // bottom-right vertical edge
		{4, 32, false},  // leftmost interior vertical edge
		{8, 32, true},   // interior horizontal edge
		{0, 16, true},   // left-column horizontal edge
		{60, 60, true},  // bottom-right horizontal edge
		{32, 4, true},   // topmost interior horizontal edge
	}
	for _, qp := range []int32{0, 10, 28, 40, 51} {
		for _, off := range [][2]int32{{0, 0}, {6, -6}, {-6, 6}} {
			alpha, beta := filterAlphaBeta(qp, off[0], off[1])
			for bS := 0; bS <= 4; bS++ {
				tc := filterTC(qp, off[0], bS)
				for _, pl := range placements {
					for seed := 0; seed < 2; seed++ {
						base := deblockStimulus(W, H, seed)
						a := append([]uint8(nil), base...)
						b := append([]uint8(nil), base...)
						if deblockLumaFast(a, W, pl.ex, pl.ey, pl.vertical, bS, alpha, beta, tc) {
							// Fast path ran; b must follow the留守.
						} else {
							// Fallback: run the留守 on a too.
							filterLumaEdge(a, W, pl.ex, pl.ey, pl.vertical, bS, alpha, beta, tc)
						}
						filterLumaEdge(b, W, pl.ex, pl.ey, pl.vertical, bS, alpha, beta, tc)
						for i := range a {
							if a[i] != b[i] {
								t.Fatalf("qp=%d off=%v bS=%d alpha=%d beta=%d tc=%d edge=(%d,%d,v=%v) seed=%d byte %d fast=%d scalar=%d",
									qp, off, bS, alpha, beta, tc, pl.ex, pl.ey, pl.vertical, seed, i, a[i], b[i])
							}
						}
					}
				}
			}
		}
	}
}

// Fast-path coverage: luma edges with bS > 0 report true on amd64 (the
// kernel runs), bS == 0 reports false everywhere (caller skips anyway).
func TestS1DeblockTaken(t *testing.T) {
	p := deblockStimulus(64, 64, 0)
	for _, tc := range []struct {
		bS       int
		vertical bool
		want     bool
	}{
		{1, false, true}, {3, false, true}, {4, false, true},
		{1, true, true}, {3, true, true}, {4, true, true},
		{0, false, false}, {0, true, false},
	} {
		got := deblockLumaFast(p, 64, 32, 32, tc.vertical, tc.bS, 40, 10, 1)
		want := tc.want && runtime.GOARCH == "amd64" && !deblockScalarForced
		if got != want {
			t.Fatalf("bS=%d vertical=%v taken=%v want %v (arch %s)",
				tc.bS, tc.vertical, got, want, runtime.GOARCH)
		}
	}
	if runtime.GOARCH != "amd64" {
		t.Logf("%s has no kernel yet (arm64随后), scalar留守 covers", runtime.GOARCH)
	}
}

// Hot path allocates zero (per-edge stack buffer only).
func TestS1DeblockZeroAlloc(t *testing.T) {
	p := deblockStimulus(64, 64, 0)
	for _, bS := range []int{1, 4} {
		if n := testing.AllocsPerRun(20, func() {
			deblockLumaFast(p, 64, 32, 8, false, bS, 40, 10, 2)
			deblockLumaFast(p, 64, 8, 32, true, bS, 40, 10, 2)
		}); n != 0 {
			t.Fatalf("bS=%d allocs = %v want 0", bS, n)
		}
	}
}

// Real-frame pictures agree dispatch-vs-scalar: decode the first frame
// of the 720p/480p clips twice from the same state and filter once per
// path (shared DPB start, so no reference drift). VR2 exact pins pixels
// downstream; this pins the dispatch switch itself.
func TestS1DeblockRealFrameMatchesScalar(t *testing.T) {
	old := deblockScalarForced
	defer func() { deblockScalarForced = old }()
	for _, path := range []string{"../testdata/vr2_720p.mp4", "../testdata/vr2_480p.mp4"} {
		if _, err := os.Stat(path); err != nil {
			t.Skipf("clip missing (run video/testdata/gen_vr2.sh): %v", err)
		}
		movie, err := mp4.ParseFile(path)
		if err != nil {
			t.Fatalf("demux: %v", err)
		}
		avcc, err := ParseAVCC(movie.Video.AVCConfig)
		if err != nil {
			t.Fatalf("avcc: %v", err)
		}
		mkDec := func(t *testing.T) *Decoder {
			d := NewDecoder(nil)
			for _, raw := range avcc.SPS {
				if err := d.DecodeNALU(raw); err != nil {
					t.Fatalf("sps: %v", err)
				}
			}
			for _, raw := range avcc.PPS {
				if err := d.DecodeNALU(raw); err != nil {
					t.Fatalf("pps: %v", err)
				}
			}
			return d
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer f.Close()
		s := movie.Video.Samples[0]
		buf := make([]byte, s.Size)
		if _, err := f.ReadAt(buf, int64(s.Offset)); err != nil {
			t.Fatalf("read sample 0: %v", err)
		}
		units, err := SplitAVCC(buf, avcc.LengthSize)
		if err != nil {
			t.Fatalf("split sample 0: %v", err)
		}
		d0, d1 := mkDec(t), mkDec(t)
		for _, u := range units {
			if err := d0.DecodeNALU(u); err != nil {
				t.Fatalf("slice dispatch: %v", err)
			}
			if err := d1.DecodeNALU(u); err != nil {
				t.Fatalf("slice scalar: %v", err)
			}
		}
		deblockScalarForced = false
		cfg := func(d *Decoder) (*Picture, []int32, []uint32, []int32, []int32) {
			return &Picture{Width: d.pic.Width, Height: d.pic.Height,
					Y:  append([]uint8(nil), d.pic.Y...),
					Cb: append([]uint8(nil), d.pic.Cb...),
					Cr: append([]uint8(nil), d.pic.Cr...)},
				append([]int32(nil), d.qps...), append([]uint32(nil), d.fIDC...),
				append([]int32(nil), d.fA...), append([]int32(nil), d.fB...)
		}
		pre0, qps, fIDC, fA, fB := cfg(d0)
		DeblockPicture(pre0, qps, fIDC, fA, fB, d0.mbW, d0.mbH, d0.cOff0, d0.cOff1,
			d0.mbIntra, d0.nnzY, d0.mvX, d0.mvY, d0.refIdx, d0.refList, d0.mbT8,
			d0.bDeblockMVX1(), d0.bDeblockMVY1(), d0.bDeblockRef1(), d0.refList1, d0.bDeblockUseM())
		deblockScalarForced = true
		pre1, _, _, _, _ := cfg(d1)
		DeblockPicture(pre1, qps, fIDC, fA, fB, d0.mbW, d0.mbH, d0.cOff0, d0.cOff1,
			d0.mbIntra, d0.nnzY, d0.mvX, d0.mvY, d0.refIdx, d0.refList, d0.mbT8,
			d0.bDeblockMVX1(), d0.bDeblockMVY1(), d0.bDeblockRef1(), d0.refList1, d0.bDeblockUseM())
		for j := range pre0.Y {
			if pre0.Y[j] != pre1.Y[j] {
				t.Fatalf("%s frame 0 Y byte %d dispatch=%d scalar=%d", path, j, pre0.Y[j], pre1.Y[j])
			}
		}
	}
}

// Math baseline: one 4-line edge through the scalar留守 vs the S1
// dispatch. Reports the S1-D1 multiple per strength class.
func BenchmarkS1DeblockScalarVsDispatch(b *testing.B) {
	p := deblockStimulus(64, 64, 0)
	for _, tc := range []struct {
		name string
		bS   int
	}{
		{"weak", 1},
		{"mid", 3},
		{"strong", 4},
	} {
		b.Run(tc.name+"/scalar", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				filterLumaEdge(p, 64, 32, 8, false, tc.bS, 40, 10, 2)
			}
		})
		b.Run(tc.name+"/dispatch", func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				deblockLumaFast(p, 64, 32, 8, false, tc.bS, 40, 10, 2)
			}
		})
	}
}
