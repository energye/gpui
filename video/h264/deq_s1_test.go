package h264

// S1b-A4 gate: flat-list fast dequant equals the explicit loop bit for
// bit across every QP, flat and non-flat lists, sparse and dense input.
// Oracle is the explicit levelScale loop itself (no new vectors here,
// VR2 exact pins pixels).

import (
	"testing"
)

func deqStimulus() [][16]int32 {
	var out [][16]int32
	var zero [16]int32
	out = append(out, zero)
	var single [16]int32
	single[0] = 5
	out = append(out, single)
	var dense [16]int32
	for i := range dense {
		dense[i] = int32((i*37)%201 - 100)
	}
	out = append(out, dense)
	var sparse [16]int32
	sparse[1] = -300
	sparse[7] = 1024
	sparse[15] = -1
	out = append(out, sparse)
	var wild [16]int32
	for i := range wild {
		wild[i] = int32(i*1000 - 8000)
	}
	out = append(out, wild)
	return out
}

func TestS1DeqFlatMatchesLoop(t *testing.T) {
	stim := deqStimulus()
	flat := flatW16
	custom := flatW16
	custom[0], custom[5], custom[15] = 8, 32, 24
	for qp := uint32(0); qp < 52; qp++ {
		for _, c := range stim {
			a := ITransform4x4Scaled(c, qp, flat)
			b := itrans4x4Core(deqLoopRef(c, qp, flat))
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("qp=%d flat byte %d got=%d want=%d", qp, i, a[i], b[i])
				}
			}
			// Non-flat must stay on the loop (still exact vs itself).
			d := ITransform4x4Scaled(c, qp, custom)
			e := itrans4x4Core(deqLoopRef(c, qp, custom))
			for i := range d {
				if d[i] != e[i] {
					t.Fatalf("qp=%d custom byte %d got=%d want=%d", qp, i, d[i], e[i])
				}
			}
			// WithDC twin.
			f := ITransform4x4WithDCScaled(c, 12345, qp, flat)
			g := itrans4x4Core(deqLoopDCRef(c, 12345, qp, flat))
			for i := range f {
				if f[i] != g[i] {
					t.Fatalf("qp=%d dc byte %d got=%d want=%d", qp, i, f[i], g[i])
				}
			}
		}
	}
}

// TestS18x8ZeroSkipsCore pins the 8x8 zero fast lane: all-zero coeff
// must return all-zero residual (same bytes as the full core path,
// which rounds the +32 DC offset back to zero through the two IDCT
// passes). Flat and custom lists, every QP.
func TestS18x8ZeroSkipsCore(t *testing.T) {
	var zero [64]int32
	var flat [64]uint8
	for i := range flat {
		flat[i] = 16
	}
	custom := flat
	custom[0], custom[27], custom[63] = 6, 33, 42
	var coreIn [64]int32
	wantCore := itrans8x8Core(coreIn)
	for i := range wantCore {
		if wantCore[i] != 0 {
			t.Fatalf("core(zero) byte %d = %d, want 0", i, wantCore[i])
		}
	}
	for qp := uint32(0); qp < 52; qp++ {
		for _, w := range [2][64]uint8{flat, custom} {
			got := ITransform8x8Scaled(zero, qp, w)
			for i := range got {
				if got[i] != 0 {
					t.Fatalf("qp=%d byte %d got=%d want 0", qp, i, got[i])
				}
				if got[i] != wantCore[i] {
					t.Fatalf("qp=%d byte %d got=%d core=%d", qp, i, got[i], wantCore[i])
				}
			}
		}
	}
}

// deqLoopRef runs the pre-A4 explicit loop verbatim.
func deqLoopRef(coeff [16]int32, qp uint32, w [16]uint8) [16]int32 {
	m := int(qp % 6)
	shift := int(qp/6) + 2
	var c [16]int32
	for scan, v := range coeff {
		if v == 0 {
			continue
		}
		pos := zigzag4x4[scan]
		ls := levelScale(m, pos%4, pos/4)
		q := int64(v) * int64(ls) * int64(w[pos]) << uint(shift)
		c[pos] = int32((q + 32) >> 6)
	}
	return c
}

func deqLoopDCRef(ac [16]int32, dc int32, qp uint32, w [16]uint8) [16]int32 {
	m := int(qp % 6)
	shift := int(qp/6) + 2
	var c [16]int32
	c[0] = dc
	for scan := 1; scan < 16; scan++ {
		v := ac[scan]
		if v == 0 {
			continue
		}
		pos := zigzag4x4[scan]
		ls := levelScale(m, pos%4, pos/4)
		q := int64(v) * int64(ls) * int64(w[pos]) << uint(shift)
		c[pos] = int32((q + 32) >> 6)
	}
	return c
}

func BenchmarkS1DeqFlatVsLoop(b *testing.B) {
	var dense [16]int32
	for i := range dense {
		dense[i] = int32((i*37)%201 - 100)
	}
	b.Run("flat", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			ITransform4x4Scaled(dense, uint32(28), flatW16)
		}
	})
	b.Run("loop", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			itrans4x4Core(deqLoopRef(dense, uint32(28), flatW16))
		}
	})
}
