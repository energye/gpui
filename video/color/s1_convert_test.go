package color

// S1-T2 gate: vector dispatch equals the scalar留守 bit for bit.
// Peer: ffmpeg libswscale x86 yuv_2_rgb.asm (SIMD instance) vs our
// convert_amd64.s row kernel; output pinned by testdata/vr3_vectors.json
// (no new vectors here). Pass line: every vector converts byte-exact
// through the dispatch, scalar and dispatch agree on all four
// matrix/range variants plus the 854 tail width, zero alloc kept.

import (
	"testing"
)

func TestS1DispatchMatchesVectors(t *testing.T) {
	for _, vc := range loadVectors(t) {
		opt := Options{FullRange: vc.FullRange, Matrix: vc.Matrix, MatrixPresent: vc.MatrixPresent}
		f, err := Convert(SamplingYUV420P, vc.Y, vc.Cb, vc.Cr, vc.W, vc.H, opt)
		if err != nil {
			t.Fatalf("%s: %v", vc.Desc, err)
		}
		for i := range f.Pix {
			if f.Pix[i] != vc.WantRGBA[i] {
				t.Fatalf("%s: dispatch byte %d = %d want vector %d", vc.Desc, i, f.Pix[i], vc.WantRGBA[i])
			}
		}
	}
}

// Dispatch (SIMD or scalar fallback) agrees with the scalar留守 on all
// four matrix/range variants, including the 854 tail width (bulk 4 + tail).
func TestS1DispatchMatchesScalar(t *testing.T) {
	for _, wh := range [][2]int{{2, 2}, {8, 4}, {854, 8}, {320, 240}} {
		w, h := wh[0], wh[1]
		y := make([]byte, w*h)
		cb := make([]byte, w*h/4)
		cr := make([]byte, w*h/4)
		for i := range y {
			y[i] = uint8((i * 7) & 0xFF)
		}
		for i := range cb {
			cb[i] = uint8((i*13 + 40) & 0xFF)
			cr[i] = uint8((i*29 + 90) & 0xFF)
		}
		for _, tc := range []Options{
			{},
			{Matrix: MatrixBT709, MatrixPresent: true},
			{FullRange: true},
			{FullRange: true, Matrix: MatrixBT709, MatrixPresent: true},
		} {
			m, err := resolveMatrix(tc)
			if err != nil {
				t.Fatal(err)
			}
			coeff := tableFor(m, tc.FullRange)
			a := make([]byte, w*h*4)
			b := make([]byte, w*h*4)
			convertBand(a, y, cb, cr, w, coeff, 0, h)
			convertBandScalar(b, y, cb, cr, w, coeff, 0, h)
			for i := range a {
				if a[i] != b[i] {
					t.Fatalf("%dx%d opt %+v byte %d dispatch=%d scalar=%d", w, h, tc, i, a[i], b[i])
				}
			}
		}
	}
}

// Math-only baseline (no band threads): one 480p frame through the scalar
// 留守 vs the S1 dispatch. Reports the S1-T2 multiple; end-to-end
// ConvertInto keeps its own benchmarks (threading overhead stays).
func BenchmarkS1BandScalarVsDispatch(b *testing.B) {
	w, h := 854, 480
	y := make([]byte, w*h)
	cb := make([]byte, w*h/4)
	cr := make([]byte, w*h/4)
	for i := range y {
		y[i] = uint8((i * 7) & 0xFF)
	}
	for i := range cb {
		cb[i] = uint8((i*13 + 40) & 0xFF)
		cr[i] = uint8((i*29 + 90) & 0xFF)
	}
	coeff := tableFor(MatrixSMPTE170M, false)
	dst := make([]byte, w*h*4)
	b.Run("scalar", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertBandScalar(dst, y, cb, cr, w, coeff, 0, h)
		}
	})
	b.Run("dispatch", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			convertBand(dst, y, cb, cr, w, coeff, 0, h)
		}
	})
}
