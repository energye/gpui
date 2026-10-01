package ffmpeg

import (
	"math"
	"testing"
)

func lumaMeanRGBA(pix []byte) float64 {
	if len(pix) == 0 {
		return 0
	}
	var s float64
	n := 0
	for i := 0; i+2 < len(pix); i += 16384 {
		s += 0.299*float64(pix[i]) + 0.587*float64(pix[i+1]) + 0.114*float64(pix[i+2])
		n++
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}

func planeMean(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	var s uint64
	n := uint64(0)
	for i := 0; i < len(b); i += 4096 {
		s += uint64(b[i])
		n++
	}
	return float64(s) / float64(n)
}

// TestNV12MatchesRGBA decodes one clip twice (RGBA vs NV12) and pins
// the cross-check: same size, Y mean within 8 of the RGBA luma mean,
// UV non-trivial, and NV12 bytes deterministic across runs.
func TestNV12MatchesRGBA(t *testing.T) {
	path := "../testdata/vr2_720p.mp4"
	dr, err := Open(path)
	if err != nil {
		t.Skipf("open %s: %v", path, err)
	}
	var rgbMean float64
	w, h := 0, 0
	for i := 0; i < 3; i++ {
		fr, err := dr.Next()
		if err != nil {
			t.Fatalf("rgba Next: %v", err)
		}
		if fr.NV12 {
			t.Fatalf("default path returned NV12")
		}
		if i == 0 {
			w, h = fr.Width, fr.Height
			rgbMean = lumaMeanRGBA(fr.Pix)
		}
		fr.Release()
	}
	dr.Close()

	dn, err := Open(path)
	if err != nil {
		t.Skipf("open %s: %v", path, err)
	}
	defer dn.Close()
	dn.SetNV12(true)
	var y0, uv0 []byte
	for i := 0; i < 3; i++ {
		fr, err := dn.Next()
		if err != nil {
			t.Fatalf("nv12 Next: %v", err)
		}
		if !fr.NV12 {
			t.Fatalf("NV12 path returned RGBA")
		}
		if fr.Width != w || fr.Height != h {
			t.Fatalf("size %dx%d, want %dx%d", fr.Width, fr.Height, w, h)
		}
		if len(fr.Y) != w*h || len(fr.UV) != w*h/2 {
			t.Fatalf("plane lens %d/%d, want %d/%d", len(fr.Y), len(fr.UV), w*h, w*h/2)
		}
		if i == 0 {
			y0 = append([]byte(nil), fr.Y...)
			uv0 = append([]byte(nil), fr.UV...)
			if d := math.Abs(planeMean(fr.Y) - rgbMean); d > 8 {
				t.Fatalf("Y mean %.1f vs RGBA luma %.1f, diff %.1f > 8", planeMean(fr.Y), rgbMean, d)
			}
			if planeMean(fr.UV) < 5 {
				t.Fatalf("UV mean %.1f, want non-trivial chroma", planeMean(fr.UV))
			}
		}
		fr.Release()
	}
	// Determinism: decode again, bytes must match.
	dn2, err := Open(path)
	if err != nil {
		t.Skipf("reopen %s: %v", path, err)
	}
	defer dn2.Close()
	dn2.SetNV12(true)
	fr, err := dn2.Next()
	if err != nil {
		t.Fatalf("redecode Next: %v", err)
	}
	defer fr.Release()
	if len(fr.Y) != len(y0) || len(fr.UV) != len(uv0) {
		t.Fatalf("redecode lens %d/%d, want %d/%d", len(fr.Y), len(fr.UV), len(y0), len(uv0))
	}
	for i := range y0 {
		if fr.Y[i] != y0[i] {
			t.Fatalf("Y byte %d differs across runs", i)
		}
	}
	for i := range uv0 {
		if fr.UV[i] != uv0[i] {
			t.Fatalf("UV byte %d differs across runs", i)
		}
	}
}
