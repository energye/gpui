package h264

// S1b-A1 gate: chroma dispatch equals the scalar留守 bit for bit.
// Peer: ffmpeg libavcodec/h264chroma_template.c:29-70 FUNCC chroma_mc
// family (A=(8-x)*(8-y) etc, (sum+32)>>6, our chroma_amd64.s row kernel)
// vs our chroma_amd64.go block skeleton; oracle is the scalar留守
// itself (predictChromaBlock in inter.go, no new vectors here, VR2
// exact pins pixels).
// Pass lines: all 64 fracs x partition sizes x interior/edge agree;
// interior 8x8 dispatch allocates zero; VR2 exact stays green.

import (
	"runtime"
	"testing"
)

// chromaStimulus is one deterministic reference plane (stimulus only;
// oracle is the scalar path with clipping, so no golden file needed).
func chromaStimulus(w, h int) []byte {
	p := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p[y*w+x] = uint8((x*5 + y*11 + (x * y & 0xFF)) & 0xFF)
		}
	}
	return p
}

// chromaScalarOracle runs the pre-S1b scalar verbatim (per-tap clip).
func chromaScalarOracle(plane []byte, w, h, px, py, cw, ch int, mx, my int16, out []byte) {
	for dy := 0; dy < ch; dy++ {
		for dx := 0; dx < cw; dx++ {
			ex := (px+dx)*8 + int(mx)
			ey := (py+dy)*8 + int(my)
			ix := ex >> 3
			iy := ey >> 3
			fx := ex - (ix << 3)
			fy := ey - (iy << 3)
			a := fullPel(plane, w, h, ix, iy)
			b := fullPel(plane, w, h, ix+1, iy)
			c := fullPel(plane, w, h, ix, iy+1)
			d := fullPel(plane, w, h, ix+1, iy+1)
			v := ((8-fx)*(8-fy)*a + fx*(8-fy)*b + (8-fx)*fy*c + fx*fy*d + 32) >> 6
			out[dy*cw+dx] = uint8(clipInt(v, 0, 255))
		}
	}
}

// Dispatch (SIMD or interior scalar) agrees with the scalar oracle on
// every frac, every partition size, interior and edge-touching positions
// (edges must fall back and still match).
func TestS1ChromaDispatchMatchesScalar(t *testing.T) {
	const W, H = 64, 64
	plane := chromaStimulus(W, H)
	sizes := [][2]int{{2, 2}, {4, 4}, {8, 4}, {4, 8}, {8, 8}}
	placements := []struct {
		px, py int
		mx, my int16
	}{
		{20, 20, 1, 1},
		{0, 0, 1, 1},
		{0, 0, -8, -8},
		{1, 1, 3, 5},
		{20, 20, -9, -7},
	}
	for _, sz := range sizes {
		cw, ch := sz[0], sz[1]
		for fx := 0; fx < 8; fx++ {
			for fy := 0; fy < 8; fy++ {
				for _, pl := range placements {
					mx := int16(pl.mx&^7) | int16(fx)
					my := int16(pl.my&^7) | int16(fy)
					px, py := pl.px, pl.py
					if px+cw > W {
						px = W - cw
					}
					if py+ch > H {
						py = H - ch
					}
					a := make([]byte, cw*ch)
					b := make([]byte, cw*ch)
					predictChromaBlock(plane, W, H, px, py, cw, ch, mx, my, a)
					chromaScalarOracle(plane, W, H, px, py, cw, ch, mx, my, b)
					for i := range a {
						if a[i] != b[i] {
							t.Fatalf("%dx%d frac=%d,%d at (%d,%d) mv=(%d,%d) byte %d dispatch=%d scalar=%d",
								cw, ch, fx, fy, px, py, mx, my, i, a[i], b[i])
						}
					}
				}
			}
		}
	}
}

// chromaBlock fast-path coverage: interior blocks report true, edge
// blocks report false (fall back and still match above). Under
// GPUI_SCALAR_CONVERT=1 the dispatch stays scalar by design (same
// switch as qpel/deblock); the block kernel itself is still available.
func TestS1ChromaBlockCoverage(t *testing.T) {
	const W, H = 32, 32
	plane := chromaStimulus(W, H)
	out := make([]byte, 64)
	if !chromaBlock(out, plane, W, 8, 8, 8, 8, 3, 5) {
		t.Fatalf("interior 8x8 should take the fast path")
	}
	// Edge bounds stay the dispatch's job (chromaBlock assumes interior,
	// like qpelBlock); the edge case below pins the fallback.
	if qpelScalarForced {
		if chromaFast(plane, W, H, 8, 8, 8, 8, 3, 5, out) {
			t.Fatalf("forced-scalar dispatch must stay scalar")
		}
		return
	}
	if !chromaFast(plane, W, H, 8, 8, 8, 8, 3, 5, out) {
		t.Fatalf("interior 8x8 should take the fast path")
	}
	if chromaFast(plane, W, H, 0, 0, 8, 8, -9, -9, out) {
		t.Fatalf("edge block should fall back to scalar")
	}
}

// TestS1ChromaZeroAlloc pins the interior 8x8 dispatch zero-alloc.
func TestS1ChromaZeroAlloc(t *testing.T) {
	const W, H = 64, 64
	plane := chromaStimulus(W, H)
	out := make([]byte, 64)
	// Warm the dispatch once (env read is init-time, no alloc after).
	predictChromaBlock(plane, W, H, 8, 8, 8, 8, 3, 5, out)
	n := testing.AllocsPerRun(100, func() {
		predictChromaBlock(plane, W, H, 8, 8, 8, 8, 3, 5, out)
	})
	if n != 0 {
		t.Fatalf("interior 8x8 allocs = %v, want 0", n)
	}
}

func BenchmarkS1ChromaScalarVsDispatch(b *testing.B) {
	const W, H = 768, 432
	plane := chromaStimulus(W, H)
	out := make([]byte, 64)
	b.Run("dispatch", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			predictChromaBlock(plane, W, H, 100, 100, 8, 8, 3, 5, out)
		}
	})
	b.Run("scalar", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			chromaScalarOracle(plane, W, H, 100, 100, 8, 8, 3, 5, out)
		}
	})
}

// S1-D gate: one whole-edge chroma call equals four per-segment scalar
// calls bit for bit, both directions, weak-only strength patterns
// (uniform and mixed with skips), the full QP range, and two stimulus
// fields. Intra/mixed-bS==4 edges stay scalar (covered by the taken
// test below and the production fallback).
func TestS1ChromaDebEdgeMatchesScalar(t *testing.T) {
	const W, H = 64, 64
	type placement struct {
		cx, cy   int
		vertical bool
	}
	placements := []placement{
		{16, 8, false}, // interior vertical edge
		{32, 24, false},
		{8, 16, true}, // interior horizontal edge
		{24, 32, true},
	}
	patterns := [][4]int{
		{0, 0, 0, 0},
		{1, 1, 1, 1},
		{2, 2, 2, 2},
		{3, 3, 3, 3},
		{1, 0, 2, 0},
		{3, 2, 1, 0},
		{0, 3, 0, 3},
		{2, 1, 0, 3},
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
						if pat[g] < 1 {
							tcEdge[g] = -1
						} else {
							tcEdge[g] = filterTC(qp, off[0], pat[g]) + 1
						}
					}
					for _, pl := range placements {
						for seed := 0; seed < 2; seed++ {
							base := chromaStimulus(W, H)
							if seed == 1 {
								for i, v := range base {
									base[i] = 255 - v
								}
							}
							a := append([]uint8(nil), base...)
							b := append([]uint8(nil), base...)
							if chromaDebEdge16(a, W, pl.cx, pl.cy, pl.vertical, pat, tcEdge, alpha, beta) {
								// Edge call ran (or skip).
							} else {
								// Per-segment production fallback.
								for seg := 0; seg < 4; seg++ {
									sx, sy := pl.cx, pl.cy
									if pl.vertical {
										sx = pl.cx + seg*2
									} else {
										sy = pl.cy + seg*2
									}
									if pat[seg] < 1 {
										continue
									}
									filterChromaEdge(a, W, sx, sy, pl.vertical, pat[seg], alpha, beta, tcEdge[seg]-1)
								}
							}
							for seg := 0; seg < 4; seg++ {
								sx, sy := pl.cx, pl.cy
								if pl.vertical {
									sx = pl.cx + seg*2
								} else {
									sy = pl.cy + seg*2
								}
								if pat[seg] < 1 {
									continue
								}
								filterChromaEdge(b, W, sx, sy, pl.vertical, pat[seg], alpha, beta, tcEdge[seg]-1)
							}
							for i := range a {
								if a[i] != b[i] {
									t.Fatalf("force=%v qp=%d off=%v bS=%v alpha=%d beta=%d edge=(%d,%d,v=%v) seed=%d byte %d edge=%d scalar=%d",
										force, qp, off, pat, alpha, beta, pl.cx, pl.cy, pl.vertical, seed, i, a[i], b[i])
								}
							}
						}
					}
				}
			}
		}
	}
}

// S1-D taken: weak-only edges with any filtering report true on amd64
// (one asm call) and false under forced-scalar; any bS==4 reports
// false everywhere (intra scalar path); other arches report false
// except the all-zero skip, which reports true without touching the
// plane.
func TestS1ChromaDebEdgeTaken(t *testing.T) {
	p := chromaStimulus(64, 64)
	mkTC := func(pat [4]int) [4]int {
		var tc [4]int
		for g := 0; g < 4; g++ {
			if pat[g] < 1 {
				tc[g] = -1
			} else {
				tc[g] = filterTC(28, 0, pat[g]) + 1
			}
		}
		return tc
	}
	for _, tc := range []struct {
		name     string
		bS       [4]int
		vertical bool
	}{
		{"weak", [4]int{1, 1, 1, 1}, false},
		{"mixed", [4]int{3, 0, 2, 1}, true},
		{"strong", [4]int{4, 4, 4, 4}, false},
		{"halfstrong", [4]int{1, 4, 1, 1}, true},
		{"zero", [4]int{0, 0, 0, 0}, false},
	} {
		got := chromaDebEdge16(p, 64, 16, 16, tc.vertical, tc.bS, mkTC(tc.bS), 40, 10)
		weakOnly := true
		anyFilter := false
		for _, v := range tc.bS {
			if v == 4 {
				weakOnly = false
			}
			if v > 0 {
				anyFilter = true
			}
		}
		want := weakOnly && anyFilter && (runtime.GOARCH == "amd64") && !deblockScalarForced
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
	if chromaDebEdge16(p, 64, 16, 16, false, weak, mkTC(weak), 40, 10) {
		t.Fatalf("forced scalar chroma edge taken, want fallback")
	}
}
