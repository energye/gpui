package h264

// S1-D1 gate: deblock dispatch equals the scalar留守 bit for bit.
// Peer: ffmpeg libavcodec/x86/h264_deblock.asm (SIMD instance) vs our
// deblock_amd64.s 4-line kernels, and libavcodec/aarch64/h264dsp_neon.S
// (ff_h264_v/h_loop_filter_luma_neon + _intra_neon, same V/H split and
// 16-line macro) vs our deblock_arm64.s 4-line kernels; oracle is the
// scalar留守 itself (no new vectors here, VR2 exact pins pixels).
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

// Fast-path coverage: luma edges with bS > 0 report true where a
// kernel exists (amd64 SSE2/SSSE3, arm64 NEON), bS == 0 reports false
// everywhere (caller skips anyway).
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
		want := tc.want && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64") && !deblockScalarForced
		if got != want {
			t.Fatalf("bS=%d vertical=%v taken=%v want %v (arch %s)",
				tc.bS, tc.vertical, got, want, runtime.GOARCH)
		}
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		t.Logf("%s has no kernel yet, scalar留守 covers", runtime.GOARCH)
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
	var bS, tc [4]int
	for _, v := range []int{0, 1, 2, 3, 4} {
		for g := 0; g < 4; g++ {
			bS[g], tc[g] = v, filterTC(28, 0, v)
		}
		if n := testing.AllocsPerRun(20, func() {
			deblockLumaEdge16(p, 64, 32, 8, false, bS, tc, 40, 10)
			deblockLumaEdge16(p, 64, 8, 32, true, bS, tc, 40, 10)
		}); n != 0 {
			t.Fatalf("edge16 bS=%d allocs = %v want 0", v, n)
		}
	}
}

// S1-E16 gate: one whole-edge call equals four per-segment scalar calls
// bit for bit, both directions, uniform and mixed strength patterns
// (weak uniform, weak mixed with skips, all-4 strong, mixed weak/strong
// that must fall back to the per-segment path), the full QP range, and
// two stimulus fields.
func TestS1DeblockEdge16MatchesScalar(t *testing.T) {
	const W, H = 64, 64
	type placement struct {
		ex, ey   int
		vertical bool
	}
	placements := []placement{
		{32, 8, false},  // interior vertical edge
		{16, 16, false}, // interior vertical edge
		{8, 32, true},   // interior horizontal edge
		{32, 32, true},  // interior horizontal edge
	}
	patterns := [][4]int{
		{0, 0, 0, 0},
		{1, 1, 1, 1},
		{2, 2, 2, 2},
		{3, 3, 3, 3},
		{4, 4, 4, 4},
		{1, 0, 2, 0},
		{3, 2, 1, 0},
		{0, 3, 0, 3},
		{2, 1, 0, 3},
		{4, 4, 1, 4},
		{1, 4, 2, 3},
	}
	old := deblockScalarForced
	defer func() { deblockScalarForced = old }()
	for _, force := range []bool{false, true} {
		deblockScalarForced = force
		for _, qp := range []int32{0, 10, 28, 40, 51} {
			for _, off := range [][2]int32{{0, 0}, {6, -6}, {-6, 6}} {
				alpha, beta := filterAlphaBeta(qp, off[0], off[1])
				for _, pat := range patterns {
					var tcEdge [4]int
					for g := 0; g < 4; g++ {
						tcEdge[g] = filterTC(qp, off[0], pat[g])
					}
					for _, pl := range placements {
						for seed := 0; seed < 2; seed++ {
							base := deblockStimulus(W, H, seed)
							a := append([]uint8(nil), base...)
							b := append([]uint8(nil), base...)
							if deblockLumaEdge16(a, W, pl.ex, pl.ey, pl.vertical, pat, tcEdge, alpha, beta) {
								// Edge call ran (or all-zero skip).
							} else {
								// Per-segment production fallback.
								for seg := 0; seg < 4; seg++ {
									sx, sy := pl.ex, pl.ey
									if pl.vertical {
										sx = pl.ex + seg*4
									} else {
										sy = pl.ey + seg*4
									}
									if pat[seg] == 0 {
										continue
									}
									if !deblockLumaFast(a, W, sx, sy, pl.vertical, pat[seg], alpha, beta, tcEdge[seg]) {
										filterLumaEdge(a, W, sx, sy, pl.vertical, pat[seg], alpha, beta, tcEdge[seg])
									}
								}
							}
							for seg := 0; seg < 4; seg++ {
								sx, sy := pl.ex, pl.ey
								if pl.vertical {
									sx = pl.ex + seg*4
								} else {
									sy = pl.ey + seg*4
								}
								if pat[seg] == 0 {
									continue
								}
								filterLumaEdge(b, W, sx, sy, pl.vertical, pat[seg], alpha, beta, tcEdge[seg])
							}
							for i := range a {
								if a[i] != b[i] {
									t.Fatalf("force=%v qp=%d off=%v bS=%v alpha=%d beta=%d edge=(%d,%d,v=%v) seed=%d byte %d edge=%d scalar=%d",
										force, qp, off, pat, alpha, beta, pl.ex, pl.ey, pl.vertical, seed, i, a[i], b[i])
								}
							}
						}
					}
				}
			}
		}
	}
}

// TestS1DeblockEdge16Taken documents the taken/fallback matrix against
// the dispatch implementation itself (not a copy of the logic): uniform
// weak, uniform all-4 strong and all-zero report taken on amd64;
// mixed weak/strong blends report fallback; other arches report
// fallback except zero. Gated-off (alpha/beta zero) patterns like the
// QP-0 case below report taken without touching the picture.
func TestS1DeblockEdge16Taken(t *testing.T) {
	p := deblockStimulus(64, 64, 0)
	mkTC := func(pat [4]int) [4]int {
		var tc [4]int
		for g := 0; g < 4; g++ {
			tc[g] = filterTC(28, 0, pat[g])
		}
		return tc
	}
	for _, tc := range []struct {
		name     string
		bS       [4]int
		vertical bool
	}{
		{"weak", [4]int{1, 1, 1, 1}, false},
		{"weakmixed", [4]int{3, 0, 2, 1}, true},
		{"strong", [4]int{4, 4, 4, 4}, false},
		{"strongV", [4]int{4, 4, 4, 4}, true},
		{"halfstrong", [4]int{1, 4, 1, 1}, true},
		{"strongweak", [4]int{4, 4, 3, 4}, false},
		{"zero", [4]int{0, 0, 0, 0}, false},
	} {
		got := deblockLumaEdge16(p, 64, 32, 16, tc.vertical, tc.bS, mkTC(tc.bS), 40, 10)
		allStrong := true
		allWeakOrSkip := true
		anyFilter := false
		for _, v := range tc.bS {
			if v != 4 {
				allStrong = false
			}
			if v >= 4 {
				allWeakOrSkip = false
			}
			if v > 0 {
				anyFilter = true
			}
		}
		// Explicit (40,10) gates are open here, so taken mirrors the
		// kernel matrix: uniform filtering edges on amd64, zero
		// short-circuit everywhere (unless forced scalar), mixed
		// blends and forced scalar fall back.
		want := (allStrong || allWeakOrSkip) && anyFilter && (runtime.GOARCH == "amd64") && !deblockScalarForced
		// All-zero edges short-circuit before any arch check.
		if tc.name == "zero" && !deblockScalarForced {
			want = true
		}
		if got != want {
			t.Fatalf("%s bS=%v vertical=%v taken=%v want %v (arch %s)",
				tc.name, tc.bS, tc.vertical, got, want, runtime.GOARCH)
		}
	}
	old := deblockScalarForced
	deblockScalarForced = true
	defer func() { deblockScalarForced = old }()
	weak := [4]int{1, 2, 3, 1}
	if deblockLumaEdge16(p, 64, 32, 16, false, weak, mkTC(weak), 40, 10) {
		t.Fatalf("forced scalar edge16 taken, want fallback")
	}
	strong := [4]int{4, 4, 4, 4}
	if deblockLumaEdge16(p, 64, 32, 16, false, strong, mkTC(strong), 40, 10) {
		t.Fatalf("forced scalar strong edge16 taken, want fallback")
	}
}

// Hit-rate census (step C1: measure before cutting further). Decodes
// both reference clips end to end with the stats switch on and logs
// how the whole-edge dispatch splits: zero/gated skips, weak and
// strong single calls, mixed fallbacks. Decides whether去块 still has
// headroom (raise single-call share) or the next cut must move to
// interpolation. No pixel assertions here: TestRefClipErrorFree and
// the full-compare tools pin pixels; this pins the split.
// amd64-only: other archs run the per-segment path by design.
func TestS1Edge16HitRate(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skipf("hit-rate census is amd64-only (per-segment elsewhere)")
	}
	if deblockScalarForced {
		t.Skipf("forced scalar takes nothing: census would read all-fallback")
	}
	// Stats path allocates zero (classify + atomic bumps only).
	p := deblockStimulus(64, 64, 0)
	var bS, tc [4]int
	for g := 0; g < 4; g++ {
		bS[g], tc[g] = 2, filterTC(28, 0, 2)
	}
	old := deblockStatsOn
	deblockStatsOn = true
	defer func() { deblockStatsOn = old }()
	if n := testing.AllocsPerRun(20, func() {
		taken := deblockLumaEdge16(p, 64, 32, 8, false, bS, tc, 40, 10)
		edge16Note(classifyEdge16(bS, 40, 10), taken)
	}); n != 0 {
		t.Fatalf("stats path allocs = %v want 0", n)
	}
	for _, c := range refClips {
		path := "../testdata/" + c.file
		if _, err := os.Stat(path); err != nil {
			t.Skipf("clip absent: %v", err)
		}
		edge16Reset()
		decodeRefClipCount(t, path, c.w, c.h, c.samples)
		st := edge16Snapshot()
		edge16Reset()
		if st.total == 0 {
			t.Fatalf("%s: no edges counted", c.file)
		}
		parts := st.zero + st.gated + st.weakTaken + st.strongTaken + st.mixedFallback + st.otherFallback
		if parts != st.total {
			t.Fatalf("%s: buckets %d != total %d", c.file, parts, st.total)
		}
		if st.otherFallback != 0 {
			t.Fatalf("%s: weak/strong fell back %d edges (amd64 unforced must take all)", c.file, st.otherFallback)
		}
		pct := func(v uint64) float64 { return 100 * float64(v) / float64(st.total) }
		t.Logf("%s: edges=%d zero=%.1f%% gated=%.1f%% weak1call=%.1f%% strong1call=%.1f%% mixedFallback=%.1f%%",
			c.file, st.total, pct(st.zero), pct(st.gated), pct(st.weakTaken), pct(st.strongTaken), pct(st.mixedFallback))
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
			d0.bDeblockMVX1(), d0.bDeblockMVY1(), d0.bDeblockRef1(), d0.refList1, d0.bDeblockUseM(),
			nil, nil)
		deblockScalarForced = true
		pre1, _, _, _, _ := cfg(d1)
		DeblockPicture(pre1, qps, fIDC, fA, fB, d0.mbW, d0.mbH, d0.cOff0, d0.cOff1,
			d0.mbIntra, d0.nnzY, d0.mvX, d0.mvY, d0.refIdx, d0.refList, d0.mbT8,
			d0.bDeblockMVX1(), d0.bDeblockMVY1(), d0.bDeblockRef1(), d0.refList1, d0.bDeblockUseM(),
			nil, nil)
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

// Math baseline: one whole 16-line edge through the S1-E16 single call
// vs four per-segment dispatches. Reports the E16 multiple (call-count
// saving, not wider vectors: same 4-line bodies underneath).
func BenchmarkS1DeblockEdge16VsSegments(b *testing.B) {
	p := deblockStimulus(64, 64, 0)
	bS := [4]int{1, 2, 1, 2}
	var tc [4]int
	for g := 0; g < 4; g++ {
		tc[g] = filterTC(28, 0, bS[g])
	}
	b.Run("segments/dispatch", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for seg := 0; seg < 4; seg++ {
				deblockLumaFast(p, 64, 32, 8+seg*4, false, bS[seg], 40, 10, tc[seg])
			}
		}
	})
	b.Run("edge16/single", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			deblockLumaEdge16(p, 64, 32, 8, false, bS, tc, 40, 10)
		}
	})
	strong := [4]int{4, 4, 4, 4}
	var strongTC [4]int
	b.Run("strong-segments/dispatch", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for seg := 0; seg < 4; seg++ {
				deblockLumaFast(p, 64, 32, 8+seg*4, false, 4, 40, 10, 2)
			}
		}
	})
	b.Run("strong-edge16/single", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			deblockLumaEdge16(p, 64, 32, 8, false, strong, strongTC, 40, 10)
		}
	})
}

// S1-D gate: the edge-batch strength equals four per-segment Interior
// calls and four safe calls bit for bit, on every edge of a synthetic
// frame mixing all verdict classes (intra MBs -> 4/3, coded blocks ->
// 2, ref/mv differences -> 1, equal neighbours -> 0), both directions,
// MB-boundary and internal edges, P-style (single list) and B-style
// (dual list) states.
func TestS1InterBSEdgeMatchesSegments(t *testing.T) {
	const mbW, mbH = 4, 4
	stride := mbW * 4
	n4 := stride * mbH * 4
	nmb := mbW * mbH
	r0, err := NewPicture(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer r0.Release()
	r1, err := NewPicture(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	defer r1.Release()
	refs := []*Picture{r0, r1}
	mbIntra := make([]bool, nmb)
	nnzY := make([]int8, n4)
	mvX := make([]int16, n4)
	mvY := make([]int16, n4)
	refIdx := make([]int8, n4)
	mvX1 := make([]int16, n4)
	mvY1 := make([]int16, n4)
	refIdx1 := make([]int8, n4)
	useM := make([]uint8, n4)
	mbT8 := make([]bool, nmb)
	for mby := 0; mby < mbH; mby++ {
		for mbx := 0; mbx < mbW; mbx++ {
			addr := mby*mbW + mbx
			if (mbx+mby)%3 == 2 {
				mbIntra[addr] = true
			}
			if (mbx*mbH+mby)%2 == 0 {
				mbT8[addr] = true
			}
			for y := 0; y < 4; y++ {
				for x := 0; x < 4; x++ {
					bx, by := mbx*4+x, mby*4+y
					i := by*stride + bx
					if mbx < 2 {
						// Flat-motion zone: uniform (4,0) on ref 0 in
						// both lists forces verdict-0 edges (non-intra,
						// no coefficients, equal refs, zero motion
						// difference), which random formulas never hit.
						mvX[i], mvY[i] = 4, 0
						refIdx[i] = 0
						mvX1[i], mvY1[i] = 4, 0
						refIdx1[i] = 0
						useM[i] = useL0 | useL1
						continue
					}
					if (bx*7+by*13)%5 == 0 {
						nnzY[i] = int8((bx + by) % 3)
					}
					mvX[i] = int16(bx*3 - by*2)
					mvY[i] = int16(by*5 + bx)
					refIdx[i] = int8((bx+by)%3) - 1
					mvX1[i] = int16(by*2 - bx)
					mvY1[i] = int16(bx + by*3)
					refIdx1[i] = int8((bx*by)%3) - 1
					if (bx+by)%2 == 0 {
						useM[i] = useL0 | useL1
					} else {
						useM[i] = useL0
					}
				}
			}
		}
	}
	// P-style (single list, no use mask) and B-style (dual lists).
	type mode struct {
		useM        []uint8
		refs, refs1 []*Picture
		mvX1, mvY1  []int16
		refIdx1     []int8
	}
	modes := []mode{
		{nil, refs, nil, nil, nil, nil},
		{useM, refs, refs, mvX1, mvY1, refIdx1},
	}
	seen := map[int]int{}
	for _, md := range modes {
		for mby := 0; mby < mbH; mby++ {
			for mbx := 0; mbx < mbW; mbx++ {
				for vi := 0; vi < 2; vi++ {
					vertical := vi == 1
					for e := 0; e < 4; e++ {
						var ex, ey int
						var edgeMB bool
						if !vertical {
							ex = mbx*16 + e*4
							ey = mby * 16
							edgeMB = e == 0
						} else {
							ex = mbx * 16
							ey = mby*16 + e*4
							edgeMB = e == 0
						}
						if !edgeInterior(ex, ey, vertical, mbW, mbH) {
							continue
						}
						var batched [4]int
						interBSEdge(&batched, mbIntra, nnzY, mvX, mvY, refIdx, md.refs, mbW, mbH, ex, ey, vertical, edgeMB, mbT8, md.mvX1, md.mvY1, md.refIdx1, md.refs1, md.useM)
						for seg := 0; seg < 4; seg++ {
							sx, sy := ex, ey+seg*4
							if vertical {
								sx, sy = ex+seg*4, ey
							}
							want := interBSInterior(mbIntra, nnzY, mvX, mvY, refIdx, md.refs, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, md.mvX1, md.mvY1, md.refIdx1, md.refs1, md.useM)
							if batched[seg] != want {
								t.Fatalf("useM=%v edge=(%d,%d,v=%v,e=%d) seg=%d batch=%d interior=%d",
									md.useM != nil, ex, ey, vertical, e, seg, batched[seg], want)
							}
							safe := interBS(mbIntra, nnzY, mvX, mvY, refIdx, md.refs, mbW, mbH, sx, sy, vertical, edgeMB, mbT8, md.mvX1, md.mvY1, md.refIdx1, md.refs1, md.useM)
							if batched[seg] != safe {
								t.Fatalf("useM=%v edge=(%d,%d,v=%v,e=%d) seg=%d batch=%d safe=%d",
									md.useM != nil, ex, ey, vertical, e, seg, batched[seg], safe)
							}
							seen[batched[seg]]++
						}
					}
				}
			}
		}
	}
	for v := 0; v <= 4; v++ {
		if seen[v] == 0 {
			t.Fatalf("verdict %d never hit, test is vacuous", v)
		}
	}
	// Edge batch allocates zero (stack out-param only).
	var out [4]int
	if n := testing.AllocsPerRun(20, func() {
		interBSEdge(&out, mbIntra, nnzY, mvX, mvY, refIdx, refs, mbW, mbH, 16, 0, false, false, mbT8, nil, nil, nil, nil, nil)
	}); n != 0 {
		t.Fatalf("edge batch allocs = %v want 0", n)
	}
}

// S1b-E gate: the uniform-edge shortcut equals the per-segment slow
// path byte for byte, on mixed states that force both hit and fallback
// edges (uniform skip fields + random-motion zone + coded + intra).
// Runs P-style (single list) and B-style (dual list) verdict paths.
func TestS1DeblockUniformMatchesSlow(t *testing.T) {
	const mbW, mbH = 6, 6
	const W, H = mbW * 16, mbH * 16
	stride := mbW * 4
	nmb := mbW * mbH
	dmA := [2]bDirectMV{{ref: 0, mx: 4, my: 0, use: true}, {ref: 0, mx: -4, my: 0, use: true}}
	dmB := [2]bDirectMV{{ref: 1, mx: 8, my: 8, use: true}, {}}
	inA := func(mbx, mby int) bool { return mbx < 3 }
	inB := func(mbx, mby int) bool { return mbx >= 3 && mby >= 3 }
	isIntra := func(mbx, mby int) bool { return mbx == 4 && mby == 4 }

	n4 := stride * mbH * 4
	mvX := make([]int16, n4)
	mvY := make([]int16, n4)
	refIdx := make([]int8, n4)
	mvX1 := make([]int16, n4)
	mvY1 := make([]int16, n4)
	refIdx1 := make([]int8, n4)
	useM := make([]uint8, n4)
	mbIntra := make([]bool, nmb)
	uniSkip := make([]bool, nmb)
	uniDM := make([][2]bDirectMV, nmb)
	put := func(bx, by int, dm [2]bDirectMV) {
		i := by*stride + bx
		if dm[0].use {
			mvX[i], mvY[i] = dm[0].mx, dm[0].my
			refIdx[i] = dm[0].ref
			useM[i] |= useL0
		} else {
			refIdx[i] = -1
		}
		if dm[1].use {
			mvX1[i], mvY1[i] = dm[1].mx, dm[1].my
			refIdx1[i] = dm[1].ref
			useM[i] |= useL1
		} else {
			refIdx1[i] = -1
		}
	}
	for mby := 0; mby < mbH; mby++ {
		for mbx := 0; mbx < mbW; mbx++ {
			addr := mby*mbW + mbx
			switch {
			case isIntra(mbx, mby):
				mbIntra[addr] = true
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						i := (mby*4+y)*stride + mbx*4 + x
						refIdx[i], refIdx1[i] = -1, -1
					}
				}
			case inA(mbx, mby):
				uniSkip[addr] = true
				uniDM[addr] = dmA
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						put(mbx*4+x, mby*4+y, dmA)
					}
				}
			case inB(mbx, mby):
				uniSkip[addr] = true
				uniDM[addr] = dmB
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						put(mbx*4+x, mby*4+y, dmB)
					}
				}
			default:
				for y := 0; y < 4; y++ {
					for x := 0; x < 4; x++ {
						bx, by := mbx*4+x, mby*4+y
						dm := [2]bDirectMV{
							{ref: int8((bx + by) % 2), mx: int16(bx*3 - by), my: int16(by*5 + bx), use: true},
							{ref: int8((bx * by) % 2), mx: int16(by*2 - bx), my: int16(bx + by), use: (bx+by)%3 != 0},
						}
						put(bx, by, dm)
					}
				}
			}
		}
	}
	nnzY := make([]int8, n4)
	// Coded block in the random zone forces bS=2 verdicts (slow path
	// stays active, so the test is not vacuous). Chroma strength
	// reuses the luma verdict, so only luma nnz feeds the filter.
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			nnzY[(1*4+y)*stride+5*4+x] = 3
		}
	}
	qps := make([]int32, nmb)
	fIDC := make([]uint32, nmb)
	fA := make([]int32, nmb)
	fB := make([]int32, nmb)
	for i := range qps {
		qps[i] = 28
	}
	mbT8 := make([]bool, nmb)
	r0, err := NewPicture(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	r1, err := NewPicture(16, 16)
	if err != nil {
		t.Fatal(err)
	}
	mkPic := func() *Picture {
		base := deblockStimulus(W, H, 0)
		p := &Picture{Width: uint32(W), Height: uint32(H),
			Y:  append([]uint8(nil), base...),
			Cb: append([]uint8(nil), deblockStimulus(W/2, H/2, 0)...),
			Cr: append([]uint8(nil), deblockStimulus(W/2, H/2, 1)...)}
		return p
	}
	run := func(useSecondList bool, uni []bool, dm [][2]bDirectMV) *Picture {
		p := mkPic()
		var x1, y1 []int16
		var r1idx []int8
		var rlist1 []*Picture
		var um []uint8
		if useSecondList {
			x1, y1, r1idx = mvX1, mvY1, refIdx1
			rlist1 = []*Picture{r0, r1}
			um = useM
		}
		DeblockPicture(p, qps, fIDC, fA, fB, mbW, mbH, 0, 0,
			mbIntra, nnzY, mvX, mvY, refIdx, []*Picture{r0, r1}, mbT8,
			x1, y1, r1idx, rlist1, um, uni, dm)
		return p
	}
	for _, second := range []bool{false, true} {
		slow := run(second, nil, nil)
		fast := run(second, uniSkip, uniDM)
		for i := range slow.Y {
			if slow.Y[i] != fast.Y[i] {
				t.Fatalf("second=%v Y byte %d slow=%d fast=%d", second, i, slow.Y[i], fast.Y[i])
			}
		}
		for i := range slow.Cb {
			if slow.Cb[i] != fast.Cb[i] || slow.Cr[i] != fast.Cr[i] {
				t.Fatalf("second=%v chroma byte %d slow=(%d,%d) fast=(%d,%d)",
					second, i, slow.Cb[i], slow.Cr[i], fast.Cb[i], fast.Cr[i])
			}
		}
		// Non-vacuous: the slow path filtered something.
		base := mkPic()
		same := true
		for i := range slow.Y {
			if slow.Y[i] != base.Y[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatalf("second=%v slow path filtered nothing, test vacuous", second)
		}
	}
}
