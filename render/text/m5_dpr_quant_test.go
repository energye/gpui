package text

import (
	"testing"
)

// TestDPRQuantNoExplosion_M5 locks M5-4: the implemented 1/4px quantization
// (SubpixelXQ2/YQ2) keeps atlas key cardinality bounded across DPR scales.
// 1 glyph x 4 DPR sizes x N fractional offsets must collapse to
// 4 sizes x 4 X-quarters = 16 keys no matter how dense the offset sweep is.
func TestDPRQuantNoExplosion_M5(t *testing.T) {
	count := func(offsets []float64) int {
		seen := map[GlyphMaskKey]bool{}
		for _, dpr := range []float64{1, 1.25, 2, 3} {
			for _, frac := range offsets {
				seen[MakeGlyphMaskKey(7, 65, 16*dpr, frac, 0)] = true
			}
		}
		return len(seen)
	}
	sparse := []float64{0, 0.1, 0.24, 0.25, 0.49, 0.5, 0.74, 0.75, 0.99}
	dense := make([]float64, 0, 101)
	for i := 0; i <= 100; i++ {
		dense = append(dense, float64(i)/100)
	}
	nSparse, nDense := count(sparse), count(dense)
	if nSparse != 16 {
		t.Fatalf("sparse sweep keys = %d, want 16 (4 sizes x 4 quarters)", nSparse)
	}
	if nDense != nSparse {
		t.Fatalf("dense sweep keys = %d, want same as sparse %d (no explosion)", nDense, nSparse)
	}
}
