package h264

import (
	"testing"
)

// S1-W gate: integer direct-write equals the dense predict+copy path
// bit for bit. Covers luma and chroma, all partition sizes, inside and
// outside gates (outside falls back, tested via the gate helpers),
// plus the strided weight rects vs dense weights.
func TestS1IntDirectMatchesDense(t *testing.T) {
	const rw, rh = 64, 64
	plane := make([]uint8, rw*rh)
	for i := range plane {
		plane[i] = uint8((i*37 + 11) & 0xFF)
	}
	ref := &Picture{Width: rw, Height: rh, Y: plane}
	// Luma sizes incl. all live partitions.
	for _, wh := range [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}} {
		w, h := wh[0], wh[1]
		for _, mv := range [][2]int16{{0, 0}, {4, 0}, {0, 4}, {4, 4}, {-4, 8}, {8, -4}} {
			mx, my := mv[0], mv[1]
			px, py := 16, 16
			sx, sy, ok := lumaIntSrc(rw, rh, px, py, w, h, mx, my)
			if !ok {
				continue
			}
			// Dense reference.
			var blk [256]uint8
			predictLumaBlock(ref, px, py, w, h, mx, my, blk[:w*h])
			// Direct into stride-16 plane at origin (16x16 fills the
			// whole predY; smaller blocks sit top-left — offsets only
			// move inside the plane, origin keeps every size in bounds).
			var pred [256]uint8
			ox0, oy0 := 0, 0
			copyBlockStrided(pred[:], 16, ox0, oy0, plane, rw, sx, sy, w, h)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if pred[(oy0+y)*16+ox0+x] != blk[y*w+x] {
						t.Fatalf("luma w=%d h=%d mv=%v x=%d y=%d direct=%d dense=%d",
							w, h, mv, x, y, pred[(oy0+y)*16+ox0+x], blk[y*w+x])
					}
				}
			}
		}
	}
	// Chroma integer direct vs dense.
	cw_, ch_ := rw/2, rh/2
	cplane := make([]uint8, cw_*ch_)
	for i := range cplane {
		cplane[i] = uint8((i*53 + 7) & 0xFF)
	}
	for _, wh := range [][2]int{{8, 8}, {8, 4}, {4, 8}, {4, 4}, {4, 2}, {2, 4}, {2, 2}} {
		cw, ch := wh[0], wh[1]
		for _, mv := range [][2]int16{{0, 0}, {8, 0}, {0, 8}, {8, 8}} {
			mx, my := mv[0], mv[1]
			px, py := 8, 8
			sx, sy, ok := chromaIntSrc(cw_, ch_, px, py, cw, ch, mx, my)
			if !ok {
				continue
			}
			var blk [64]uint8
			predictChromaBlock(cplane, cw_, ch_, px, py, cw, ch, mx, my, blk[:cw*ch])
			var pred [64]uint8
			cx, cy := 0, 0
			copyBlockStrided(pred[:], 8, cx, cy, cplane, cw_, sx, sy, cw, ch)
			for y := 0; y < ch; y++ {
				for x := 0; x < cw; x++ {
					if pred[(cy+y)*8+cx+x] != blk[y*cw+x] {
						t.Fatalf("chroma cw=%d ch=%d mv=%v x=%d y=%d direct=%d dense=%d",
							cw, ch, mv, x, y, pred[(cy+y)*8+cx+x], blk[y*cw+x])
					}
				}
			}
		}
	}
	// Gate negatives: outside frame must report false on both helpers.
	if _, _, ok := lumaIntSrc(rw, rh, 0, 0, 16, 16, -4, 0); ok {
		t.Fatalf("lumaIntSrc outside reported ok")
	}
	if _, _, ok := chromaIntSrc(cw_, ch_, 0, 0, 8, 8, 0, -8); ok {
		t.Fatalf("chromaIntSrc outside reported ok")
	}
	// Weight rects equal dense weights (explicit factors on).
	d := &Decoder{wDenomL: 5, wDenomC: 5}
	for i := range d.wW0 {
		d.wW0[i] = 30
		d.wO0[i] = 5
		d.wW1[i] = 34
		d.wO1[i] = -3
	}
	for i := range d.wWC0 {
		for c := 0; c < 2; c++ {
			d.wWC0[i][c] = 28
			d.wOC0[i][c] = 4
			d.wWC1[i][c] = 36
			d.wOC1[i][c] = -2
		}
	}
	var pred [256]uint8
	for i := range pred {
		pred[i] = uint8(i & 0xFF)
	}
	var dense [64]uint8
	copy(dense[:], pred[4*16+4:4*16+4+8])
	// Build dense block matching rect (8x8 at (4,4)): gather rows.
	var blkD [64]uint8
	for y := 0; y < 8; y++ {
		copy(blkD[y*8:(y+1)*8], pred[(4+y)*16+4:(4+y)*16+4+8])
	}
	d.weightLuma(blkD[:], 3)
	d.weightLumaRect(&pred, 4, 4, 8, 8, 3, false)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			if pred[(4+y)*16+4+x] != blkD[y*8+x] {
				t.Fatalf("weightLumaRect x=%d y=%d rect=%d dense=%d", x, y, pred[(4+y)*16+4+x], blkD[y*8+x])
			}
		}
	}
	_ = dense
	// Zero-alloc: direct path (classify + copies + rect weight) — copies
	// are runtime memmove, no heap.
	var pred2 [256]uint8
	if n := testing.AllocsPerRun(20, func() {
		copyBlockStrided(pred2[:], 16, 4, 5, plane, rw, 16, 16, 8, 8)
		d.weightLumaRect(&pred2, 4, 5, 8, 8, 3, false)
	}); n != 0 {
		t.Fatalf("direct path allocs = %v want 0", n)
	}
}

// S1-W gate: strided sub-pel direct-write equals the dense path bit
// for bit. Covers all 16 luma fracs and all 64 chroma frac families,
// every live partition size, interior and edge-touching vectors (edge
// falls back to the strided scalar留守 on both paths), and non-zero
// destination offsets (the mcParts subslice shape: stride 16/8 with
// ox/oy inside the MB).
func TestS1SubpelDirectMatchesDense(t *testing.T) {
	const rw, rh = 64, 64
	plane := make([]uint8, rw*rh)
	for i := range plane {
		plane[i] = uint8((i*37 + 11) & 0xFF)
	}
	ref := &Picture{Width: rw, Height: rh, Y: plane}
	sizes := [][2]int{{16, 16}, {16, 8}, {8, 16}, {8, 8}, {8, 4}, {4, 8}, {4, 4}}
	// Motion vectors: integer, half, quarter, negative, edge-touching.
	mvs := [][2]int16{{0, 0}, {2, 0}, {0, 2}, {2, 2}, {1, 1}, {3, 3}, {1, 2}, {4, 0}, {-3, 5}, {7, -2}}
	for _, wh := range sizes {
		w, h := wh[0], wh[1]
		for _, mv := range mvs {
			mx, my := mv[0], mv[1]
			for _, org := range [][2]int{{8, 8}, {0, 0}, {40, 40}} {
				px, py := org[0], org[1]
				// Dense reference (all paths: int copy, arch, scalar).
				dense := make([]uint8, w*h)
				predictLumaBlock(ref, px, py, w, h, mx, my, dense)
				// Strided direct at MB offsets (stride 16).
				var pred [256]uint8
				ox0, oy0 := 3, 5
				if ox0+w > 16 || oy0+h > 16 {
					ox0, oy0 = 0, 0
				}
				predictLumaBlockInto(ref, px, py, w, h, mx, my, pred[oy0*16+ox0:], 16)
				for y := 0; y < h; y++ {
					for x := 0; x < w; x++ {
						if pred[(oy0+y)*16+ox0+x] != dense[y*w+x] {
							t.Fatalf("luma w=%d h=%d mv=%v org=(%d,%d) off=(%d,%d) x=%d y=%d direct=%d dense=%d",
								w, h, mv, px, py, ox0, oy0, x, y, pred[(oy0+y)*16+ox0+x], dense[y*w+x])
						}
					}
				}
			}
		}
	}
	// Chroma: all frac families incl. integer-exact (direct copy) and
	// sub-pel (kernel/scalar), sizes down to 2x2.
	cw_, ch_ := rw/2, rh/2
	cplane := make([]uint8, cw_*ch_)
	for i := range cplane {
		cplane[i] = uint8((i*53 + 7) & 0xFF)
	}
	csizes := [][2]int{{8, 8}, {8, 4}, {4, 8}, {4, 4}, {4, 2}, {2, 2}}
	cmvs := [][2]int16{{0, 0}, {8, 0}, {0, 8}, {8, 8}, {4, 0}, {0, 4}, {4, 4}, {3, 5}, {7, 7}}
	for _, wh := range csizes {
		cw, ch := wh[0], wh[1]
		for _, mv := range cmvs {
			mx, my := mv[0], mv[1]
			for _, org := range [][2]int{{4, 4}, {0, 0}, {20, 20}} {
				px, py := org[0], org[1]
				dense := make([]uint8, cw*ch)
				predictChromaBlock(cplane, cw_, ch_, px, py, cw, ch, mx, my, dense)
				var pred [64]uint8
				cx, cy := 1, 2
				if cx+cw > 8 || cy+ch > 8 {
					cx, cy = 0, 0
				}
				predictChromaBlockInto(cplane, cw_, ch_, px, py, cw, ch, mx, my, pred[cy*8+cx:], 8)
				for y := 0; y < ch; y++ {
					for x := 0; x < cw; x++ {
						if pred[(cy+y)*8+cx+x] != dense[y*cw+x] {
							t.Fatalf("chroma cw=%d ch=%d mv=%v org=(%d,%d) off=(%d,%d) x=%d y=%d direct=%d dense=%d",
								cw, ch, mv, px, py, cx, cy, x, y, pred[(cy+y)*8+cx+x], dense[y*cw+x])
						}
					}
				}
			}
		}
	}
	// Forced-scalar: both paths fall back together (dispatch pinned
	// elsewhere; here the strided scalar留守 must still match).
	old := qpelScalarForced
	qpelScalarForced = true
	defer func() { qpelScalarForced = old }()
	dense := make([]uint8, 16*16)
	predictLumaBlock(ref, 8, 8, 16, 16, 2, 2, dense)
	var pred [256]uint8
	predictLumaBlockInto(ref, 8, 8, 16, 16, 2, 2, pred[:], 16)
	for i := range dense {
		if pred[i] != dense[i] {
			t.Fatalf("forced scalar lane %d direct=%d dense=%d", i, pred[i], dense[i])
		}
	}
	// Zero-alloc on the strided sub-pel path (stack temps only).
	var p3 [256]uint8
	if n := testing.AllocsPerRun(20, func() {
		predictLumaBlockInto(ref, 8, 8, 16, 16, 1, 3, p3[:], 16)
	}); n != 0 {
		t.Fatalf("strided sub-pel allocs = %v want 0", n)
	}
}
