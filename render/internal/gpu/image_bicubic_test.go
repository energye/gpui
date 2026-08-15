package gpu

import (
	"math"
	"testing"

	intImage "github.com/energye/gpui/render/internal/image"
)

// TestCanMergeImageDraw_BicubicIsolation verifies that bicubic commands never
// merge with linear or nearest batches (I.03): they require the bicubic
// pipeline variant, so sharing a bind group/draw would select the wrong
// sampling path.
func TestCanMergeImageDraw_BicubicIsolation(t *testing.T) {
	base := &ImageDrawCommand{
		GenerationID:   7,
		Opacity:        1.0,
		ViewportWidth:  800,
		ViewportHeight: 600,
		ImgWidth:       100,
		ImgHeight:      100,
		U0:             0, V0: 0, U1: 1, V1: 1,
	}
	sameFilter := *base
	if !canMergeImageDraw(base, &sameFilter) {
		t.Error("identical commands must merge")
	}
	bicubic := *base
	bicubic.Bicubic = true
	if canMergeImageDraw(base, &bicubic) {
		t.Error("linear must not merge with bicubic")
	}
	lin := bicubic
	lin.Bicubic = false
	if canMergeImageDraw(&bicubic, &lin) {
		t.Error("bicubic must not merge back with linear")
	}
	near := bicubic
	near.Nearest = true
	if canMergeImageDraw(&bicubic, &near) {
		t.Error("bicubic must not merge with nearest")
	}
	twoBicubic := bicubic
	if !canMergeImageDraw(&bicubic, &twoBicubic) {
		t.Error("two bicubic commands with identical state must merge")
	}
}

// bicubicWeight implements the WGSL cubic_weight kernel (Catmull-Rom,
// B=0, C=0.5) in Go — the exact algorithm of textured_quad_bicubic.wgsl.
// Used to verify the shader math against the CPU bicubic sampler without
// requiring a GPU.
func bicubicWeight(t, B, C float64) float64 {
	at := t
	if at < 0 {
		at = -at
	}
	if at < 1.0 {
		return (12-9*B-6*C)/6*at*at*at + (-18+12*B+6*C)/6*at*at + (6-2*B)/6
	}
	if at < 2.0 {
		return (-B-6*C)/6*at*at*at + (6*B+30*C)/6*at*at + (-12*B-48*C)/6*at + (8*B+24*C)/6
	}
	return 0
}

// bicubicSample16 is the Go mirror of the GPU 16-tap bicubic sampler
// (textured_quad_bicubic.wgsl fs_main): texel-centered coordinates,
// separable 4x4 Catmull-Rom taps, weight normalization, clamp-to-edge.
func bicubicSample16(img *intImage.ImageBuf, u, v float64) (r, g, b, a byte) {
	w, h := img.Bounds()
	dims := [2]float64{float64(w), float64(h)}
	texelX := u*dims[0] - 0.5
	texelY := v*dims[1] - 0.5
	baseX := math.Floor(texelX)
	baseY := math.Floor(texelY)
	fracX := texelX - baseX
	fracY := texelY - baseY

	const B, C = 0.0, 0.5 // Catmull-Rom (matches CPU SampleBicubic)
	var accR, accG, accB, accA float64
	for j := 0; j < 4; j++ {
		wy := bicubicWeight(fracY-float64(j-1), B, C)
		iy := int(baseY + float64(j-1))
		for i := 0; i < 4; i++ {
			wx := bicubicWeight(fracX-float64(i-1), B, C)
			ix := int(baseX + float64(i-1))
			// clamp-to-edge
			cx := ix
			if cx < 0 {
				cx = 0
			} else if cx > w-1 {
				cx = w - 1
			}
			cy := iy
			if cy < 0 {
				cy = 0
			} else if cy > h-1 {
				cy = h - 1
			}
			pr, pg, pb, pa := img.GetRGBA(cx, cy)
			weight := wx * wy
			accR += float64(pr) * weight
			accG += float64(pg) * weight
			accB += float64(pb) * weight
			accA += float64(pa) * weight
		}
	}
	// No weight normalization (matches CPU bicubicInterp + Skia): kernel sums
	// to 1.0; clamp-to-edge handles borders.
	clamp := func(x float64) byte {
		if x < 0 {
			return 0
		}
		if x > 255 {
			return 255
		}
		return byte(math.Round(x))
	}
	return clamp(accR), clamp(accG), clamp(accB), clamp(accA)
}

// TestBicubicKernelProperties verifies the Catmull-Rom kernel invariants
// used by textured_quad_bicubic.wgsl: w(0)=1, w(1)=w(2)=0, and unit sum
// over the 4-tap window for any fraction in [0,1).
func TestBicubicKernelProperties(t *testing.T) {
	const B, C = 0.0, 0.5
	if got := bicubicWeight(0, B, C); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("w(0) = %v, want 1.0", got)
	}
	if got := bicubicWeight(1, B, C); math.Abs(got) > 1e-9 {
		t.Errorf("w(1) = %v, want 0", got)
	}
	if got := bicubicWeight(2, B, C); math.Abs(got) > 1e-9 {
		t.Errorf("w(2) = %v, want 0", got)
	}
	for _, f := range []float64{0.0, 0.1, 0.25, 0.5, 0.75, 0.9, 0.999} {
		sum := 0.0
		for i := -1; i <= 2; i++ {
			sum += bicubicWeight(f-float64(i), B, C)
		}
		if math.Abs(sum-1.0) > 1e-6 {
			t.Errorf("kernel sum at f=%v = %v, want 1.0", f, sum)
		}
	}
}

// TestBicubicGPUAlgorithmMatchesCPU verifies that the GPU bicubic shader
// algorithm (mirrored in Go as bicubicSample16) is pixel-consistent with
// the CPU sampler (internal/image SampleBicubic) — the acceptance basis for
// the GPU path (I.03).
func TestBicubicGPUAlgorithmMatchesCPU(t *testing.T) {
	w, h := 32, 24
	img, err := intImage.NewImageBuf(w, h, intImage.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	// Deterministic gradient + noise image with strong edges.
	seed := uint32(0x12345678)
	next := func() uint32 { seed = seed*1664525 + 1013904223; return seed }
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			base := byte((x*7 + y*3) % 256)
			n := byte(next() % 24)
			if err := img.SetRGBA(x, y, base^n, byte(x*9%256), byte(y*11%256), 255); err != nil {
				t.Fatalf("SetRGBA(%d,%d): %v", x, y, err)
			}
		}
	}
	maxDiff := 0.0
	for i := 0; i < 200; i++ {
		u := float64(next()%1000) / 1000
		v := float64(next()%1000) / 1000
		cr, cg, cb, ca := bicubicSample16(img, u, v)
		gr, gg, gb, ga := intImage.Sample(img, u, v, intImage.InterpBicubic)
		d := func(a, b byte) float64 {
			d := float64(a) - float64(b)
			if d < 0 {
				d = -d
			}
			return d
		}
		if d := d(cr, gr); d > maxDiff {
			maxDiff = d
		}
		if d := d(cg, gg); d > maxDiff {
			maxDiff = d
		}
		if d := d(cb, gb); d > maxDiff {
			maxDiff = d
		}
		if d := d(ca, ga); d > maxDiff {
			maxDiff = d
		}
	}
	if maxDiff > 2.0 {
		t.Errorf("GPU-algorithm bicubic deviates from CPU SampleBicubic: max channel diff = %v (want <= 2/255)", maxDiff)
	}
}
