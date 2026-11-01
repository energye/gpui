//go:build !nogpu

package render_test

import (
	"image"
	"os"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
)

// Canonical P3.5 batch 3 quad, in device pixels (matches the CPU-side
// matrix4_quad_perspective_test.go case): axis-aligned screen rectangle
// TL(0.5,0.5) TR(240.5,0.5) BR(240.5,120.5) BL(0.5,120.5) with homogeneous
// weights 200/100/100/200 (left corners depth 100, right corners depth 0
// through focal 100). The projective edge-mid rule gives u = wTL/(wTL+wTR)
// = 2/3 at the top-edge screen midpoint; at pixel (180,5) the exact closed
// form is u = 6/7 (blue side of the band boundary at 0.8) while the affine
// twin would give u = 3/4 (red side).
var perspGPUCorners = [4]render.Point{
	{X: 0.5, Y: 0.5},
	{X: 240.5, Y: 0.5},
	{X: 240.5, Y: 120.5},
	{X: 0.5, Y: 120.5},
}

var perspGPUWeights = [4]float64{200, 100, 100, 200}

func newPerspBandImage(t *testing.T) *render.ImageBuf {
	t.Helper()
	img, err := render.NewImageBuf(20, 1, render.FormatRGBA8)
	if err != nil {
		t.Fatalf("NewImageBuf: %v", err)
	}
	for x := 0; x < 20; x++ {
		r, g, b := uint8(255), uint8(0), uint8(0)
		if x >= 16 {
			r, g, b = 0, 0, 255
		}
		if err := img.SetRGBA(x, 0, r, g, b, 255); err != nil {
			t.Fatalf("SetRGBA: %v", err)
		}
	}
	return img
}

type pixClass int

const (
	classRed pixClass = iota
	classBlue
	classOther // quad-edge AA / mixed band-line pixels
)

func classify(x, y int, img image.Image) pixClass {
	r, _, b, _ := img.At(x, y).RGBA()
	rr, bb := int(r>>8), int(b>>8)
	switch {
	case rr > 250 && bb < 5:
		return classRed
	case bb > 250 && rr < 5:
		return classBlue
	default:
		return classOther
	}
}

func renderPerspQuadGPU(t *testing.T, img *render.ImageBuf, persp bool) (image.Image, render.RenderPathStats) {
	t.Helper()
	dc := render.NewContext(241, 121)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	err := dc.DrawImageQuad(img, perspGPUCorners, render.QuadDrawOptions{
		Perspective:   persp,
		Weights:       perspGPUWeights,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	})
	if err != nil {
		t.Fatalf("DrawImageQuad persp=%v: %v", persp, err)
	}
	if ferr := dc.FlushGPU(); ferr != nil {
		t.Fatalf("FlushGPU: %v", ferr)
	}
	st := dc.RenderPathStats()
	return dc.Image(), st
}

func renderPerspQuadCPU(t *testing.T, img *render.ImageBuf, persp bool) image.Image {
	t.Helper()
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	dc := render.NewContext(241, 121)
	defer func() { _ = dc.Close() }()
	dc.ClearWithColor(render.White)
	if err := dc.DrawImageQuad(img, perspGPUCorners, render.QuadDrawOptions{
		Perspective:   persp,
		Weights:       perspGPUWeights,
		Interpolation: render.InterpNearest,
		Opacity:       1,
	}); err != nil {
		t.Fatalf("cpu DrawImageQuad persp=%v: %v", persp, err)
	}
	_ = dc.FlushGPU()
	return dc.Image()
}

var perspProbePoints = []struct{ x, y int }{{180, 5}, {60, 5}, {120, 60}, {180, 90}, {10, 60}}

// expectedProbeClasses are the closed-form homography colors (computed from
// uv = Σ bᵢ·uvᵢ/wᵢ / Σ bᵢ/wᵢ at the pixel centers against the band boundary
// u = 0.8): blue only at the (180,*) probes.
var expectedProbeClasses = []pixClass{classBlue, classRed, classRed, classBlue, classRed}

// TestQuadPerspectiveGPURoutes verifies the perspective quad really rides the
// GPU image path (GPUOps > 0, no CPU fallback) — otherwise every pixel gate
// below could pass on a silent CPU fallback.
func TestQuadPerspectiveGPURoutes(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
	img := newPerspBandImage(t)
	defer img.Dispose()

	_, st := renderPerspQuadGPU(t, img, true)
	if st.GPUOps == 0 {
		t.Fatalf("perspective quad took no GPU ops: %s", st.LogLine())
	}
	if st.CPUFallbackOps != 0 {
		t.Fatalf("perspective quad fell back to CPU: %s", st.LogLine())
	}
}

// TestQuadPerspectiveGPUProbesClosedForm pins the GPU raster against the
// closed-form homography: the discriminating probes must land on the
// perspective side, not the affine side.
func TestQuadPerspectiveGPUProbesClosedForm(t *testing.T) {
	requireNativeGPU(t)
	os.Unsetenv("GOGPU_RENDER_MODE")
	img := newPerspBandImage(t)
	defer img.Dispose()

	gpuImg, st := renderPerspQuadGPU(t, img, true)
	if st.GPUOps == 0 {
		t.Fatalf("no GPU ops: %s", st.LogLine())
	}
	for i, p := range perspProbePoints {
		got := classify(p.x, p.y, gpuImg)
		if got != expectedProbeClasses[i] {
			t.Errorf("GPU probe (%d,%d) class = %d, want %d", p.x, p.y, got, expectedProbeClasses[i])
		}
	}
	// And the affine twin on the same GPU rasterizes differently — the
	// difference is the whole point of clip-space w.
	os.Unsetenv("GOGPU_RENDER_MODE")
	affineImg, ast := renderPerspQuadGPU(t, img, false)
	if ast.GPUOps == 0 {
		t.Fatalf("affine twin took no GPU ops: %s", ast.LogLine())
	}
	aff := classify(180, 5, affineImg)
	if aff != classRed {
		t.Errorf("affine GPU probe (180,5) class = %d, want red (pin lost discrimination)", aff)
	}
}

// TestQuadPerspectiveCPUGPUParity compares the GPU projective raster with the
// CPU rational twin pixel-by-pixel. Declared tolerances: settled pure-color
// pixels agree within 1/255 per channel; classification flips only inside the
// homography band-line / quad-edge AA budgets (f32 clip-w vs f64 CPU).
func TestQuadPerspectiveCPUGPUParity(t *testing.T) {
	requireNativeGPU(t)
	img := newPerspBandImage(t)
	defer img.Dispose()

	os.Unsetenv("GOGPU_RENDER_MODE")
	gpuImg, st := renderPerspQuadGPU(t, img, true)
	if st.GPUOps == 0 {
		t.Fatalf("no GPU ops: %s", st.LogLine())
	}
	cpuImg := renderPerspQuadCPU(t, img, true)

	b := gpuImg.Bounds()
	const total = 241 * 121
	settledWorst, mismatch, other, flips := 0, 0, 0, 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := classify(x, y, gpuImg)
			c := classify(x, y, cpuImg)
			if g == classOther || c == classOther {
				other++
				continue
			}
			if g != c {
				flips++
				continue
			}
			gr, _, gb, ga := gpuImg.At(x, y).RGBA()
			cr, _, cb, ca := cpuImg.At(x, y).RGBA()
			for _, d := range []int{
				channelDiff(gr, cr), channelDiff(gb, cb), channelDiff(ga, ca),
			} {
				if d > settledWorst {
					settledWorst = d
				}
			}
		}
	}
	// Measured tolerances: band-line flips ride the u=0.8 homography line
	// (~121 px long), edge AA rides the quad border. Budgets leave 3x headroom.
	if settledWorst > 1 {
		t.Errorf("settled channel diff = %d, want <= 1", settledWorst)
	}
	if mismatch != 0 {
		t.Errorf("settled classification mismatch = %d, want 0", mismatch)
	}
	if flips > total/50 {
		t.Errorf("band-line classification flips = %d, want <= %d (2%%)", flips, total/50)
	}
	if other > total/20 {
		t.Errorf("edge/mixed pixels = %d, want <= %d (5%%)", other, total/20)
	}
}

func channelDiff(a, b uint32) int {
	if a > b {
		return int((a - b) >> 8)
	}
	return int((b - a) >> 8)
}
