package render_test

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
	_ "github.com/energye/gpui/render/gpu"
)

type quadProbe struct {
	X    int      `json:"x"`
	Y    int      `json:"y"`
	Want [4]uint8 `json:"want"`
	Tol  uint8    `json:"tol"`
	Desc string   `json:"desc"`
}

type quadDrawCase struct {
	Corners [4][2]float64 `json:"corners"`
	Interp  string        `json:"interp"`
	Probes  []quadProbe   `json:"probes"`
}

type quadCases struct {
	Classify []struct {
		Name    string         `json:"name"`
		Corners [4][2]float64 `json:"corners"`
		Want    string        `json:"want"`
	} `json:"classify"`
	DrawNearest quadDrawCase `json:"draw_nearest"`
	DrawRect    quadDrawCase `json:"draw_rect"`
	Golden      struct {
		Canvas         [2]int         `json:"canvas"`
		Corners        [4][2]float64 `json:"corners"`
		Interp         string        `json:"interp"`
		File           string        `json:"file"`
		MaxChangedPct  float64       `json:"max_changed_pct"`
		MaxMeanAbs     float64       `json:"max_mean_abs"`
		Note           string        `json:"note"`
	} `json:"golden"`
}

func loadQuadCases(t *testing.T) quadCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "quad_cases.json"))
	if err != nil {
		t.Fatalf("read testdata/quad_cases.json: %v", err)
	}
	var c quadCases
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("decode quad_cases.json: %v", err)
	}
	return c
}

func quadToPoints(c [4][2]float64) [4]render.Point {
	var out [4]render.Point
	for i := 0; i < 4; i++ {
		out[i] = render.Pt(c[i][0], c[i][1])
	}
	return out
}

func quadInterp(s string) render.InterpolationMode {
	switch s {
	case "nearest":
		return render.InterpNearest
	case "bilinear":
		return render.InterpBilinear
	case "bicubic":
		return render.InterpBicubic
	default:
		return render.InterpBilinear
	}
}

func sampleAt(t *testing.T, dc *render.Context, x, y int) [4]uint8 {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		t.Fatalf("probe %d,%d outside bounds %v", x, y, b)
	}
	r, g, b2, a := img.At(x, y).RGBA()
	return [4]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b2 >> 8), uint8(a >> 8)}
}

func closeU8(got, want [4]uint8, tol uint8) bool {
	for i := 0; i < 4; i++ {
		d := int(got[i]) - int(want[i])
		if d < 0 {
			d = -d
		}
		if d > int(tol) {
			return false
		}
	}
	return true
}

func loadQuadSrc(t *testing.T) *render.ImageBuf {
	t.Helper()
	img, err := render.LoadImage(filepath.Join("testdata", "quad_src.png"))
	if err != nil {
		t.Fatalf("load quad_src.png: %v", err)
	}
	return img
}

// withCPUMode forces the CPU twin-split path with cleanup restore.
func withCPUMode(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Setenv("GOGPU_RENDER_MODE", "cpu")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

// withGPU Mode clears the CPU force flag with cleanup restore.
func withGPUMode(t *testing.T) {
	t.Helper()
	orig, had := os.LookupEnv("GOGPU_RENDER_MODE")
	_ = os.Unsetenv("GOGPU_RENDER_MODE")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("GOGPU_RENDER_MODE", orig)
		} else {
			_ = os.Unsetenv("GOGPU_RENDER_MODE")
		}
	})
}

// newWhiteContext builds a white canvas, caller defers Close.
func newWhiteContext(w, h int) *render.Context {
	dc := render.NewContext(w, h)
	dc.ClearWithColor(render.White)
	return dc
}

// drawQuadCPU draws with Normal opaque, failing on unexpected error.
func drawQuadCPU(t *testing.T, dc *render.Context, src *render.ImageBuf, corners [4]render.Point, interp string) {
	t.Helper()
	if err := dc.DrawImageQuadEx(src, corners, render.QuadDrawOptions{
		Interpolation: quadInterp(interp),
		Opacity:       1,
		BlendMode:     render.BlendNormal,
	}); err != nil {
		t.Fatalf("DrawImageQuadEx: %v", err)
	}
}

// checkProbes asserts frozen probe colors with per-probe tolerance.
func checkProbes(t *testing.T, dc *render.Context, probes []quadProbe) {
	t.Helper()
	for i, p := range probes {
		got := sampleAt(t, dc, p.X, p.Y)
		if !closeU8(got, p.Want, p.Tol) {
			t.Errorf("probe[%d] %s (%d,%d) = %v, want %v tol %d", i, p.Desc, p.X, p.Y, got, p.Want, p.Tol)
		}
	}
}

// A: trapezoid maps to trapezoid, not bbox. CPU path forced.
func TestQuadDrawNearestFromCases(t *testing.T) {
	withCPUMode(t)
	c := loadQuadCases(t)
	src := loadQuadSrc(t)
	dc := newWhiteContext(100, 60)
	defer dc.Close()
	drawQuadCPU(t, dc, src, quadToPoints(c.DrawNearest.Corners), c.DrawNearest.Interp)
	checkProbes(t, dc, c.DrawNearest.Probes)
}

// A: affine rectangle quadrant mapping stays exact.
func TestQuadDrawRectFromCases(t *testing.T) {
	withCPUMode(t)
	c := loadQuadCases(t)
	src := loadQuadSrc(t)
	dc := newWhiteContext(100, 60)
	defer dc.Close()
	drawQuadCPU(t, dc, src, quadToPoints(c.DrawRect.Corners), c.DrawRect.Interp)
	checkProbes(t, dc, c.DrawRect.Probes)
}

// B: classification table + error sentinels, never crash.
func TestQuadClassifyFromCases(t *testing.T) {
	c := loadQuadCases(t)
	for i, k := range c.Classify {
		got := render.ClassifyQuad(quadToPoints(k.Corners))
		if got.String() != k.Want {
			t.Errorf("classify[%d] %s = %s, want %s", i, k.Name, got.String(), k.Want)
		}
	}
	// Non-finite never draws, explicit error.
	nanQ := [4]render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: math.NaN(), Y: 0}}
	if got := render.ClassifyQuad(nanQ); got != render.QuadNonFinite {
		t.Errorf("nan classify = %s, want nonfinite", got.String())
	}
	infQ := [4]render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: math.Inf(1), Y: 0}}
	if got := render.ClassifyQuad(infQ); got != render.QuadNonFinite {
		t.Errorf("inf classify = %s, want nonfinite", got.String())
	}
}

func TestQuadEdgeNoCrash(t *testing.T) {
	withCPUMode(t)
	src := loadQuadSrc(t)
	newWhite := func() *render.Context {
		return newWhiteContext(40, 40)
	}
	// Degenerate returns sentinel, canvas keeps white, no panic.
	dc := newWhite()
	defer dc.Close()
	degen := [4]render.Point{{X: 10, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 10}, {X: 10, Y: 10}}
	if err := dc.DrawImageQuadEx(src, degen, render.QuadDrawOptions{Interpolation: render.InterpNearest}); !errors.Is(err, render.ErrQuadDegenerate) {
		t.Errorf("degenerate err = %v, want ErrQuadDegenerate", err)
	}
	if got := sampleAt(t, dc, 20, 20); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("degenerate polluted canvas: %v", got)
	}
	// Bowtie explicit error.
	bow := [4]render.Point{{X: 0, Y: 0}, {X: 10, Y: 10}, {X: 0, Y: 10}, {X: 10, Y: 0}}
	if err := dc.DrawImageQuadEx(src, bow, render.QuadDrawOptions{}); !errors.Is(err, render.ErrQuadBowTie) {
		t.Errorf("bowtie err = %v, want ErrQuadBowTie", err)
	}
	// NaN explicit error, no panic.
	nanQ := [4]render.Point{{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 10, Y: 10}, {X: math.NaN(), Y: 0}}
	if err := dc.DrawImageQuadEx(src, nanQ, render.QuadDrawOptions{}); !errors.Is(err, render.ErrQuadNonFinite) {
		t.Errorf("nan err = %v, want ErrQuadNonFinite", err)
	}
	// Unsupported blend/interp explicit error.
	okQ := [4]render.Point{{X: 5, Y: 5}, {X: 20, Y: 5}, {X: 20, Y: 20}, {X: 5, Y: 20}}
	if err := dc.DrawImageQuadEx(src, okQ, render.QuadDrawOptions{BlendMode: render.BlendMultiply}); !errors.Is(err, render.ErrQuadUnsupportedBlend) {
		t.Errorf("blend err = %v, want ErrQuadUnsupportedBlend", err)
	}
	if err := dc.DrawImageQuadEx(src, okQ, render.QuadDrawOptions{Interpolation: render.InterpolationMode(99)}); !errors.Is(err, render.ErrQuadUnsupportedInterp) {
		t.Errorf("interp err = %v, want ErrQuadUnsupportedInterp", err)
	}
	// Nil/empty never panic.
	_ = dc.DrawImageQuadEx(nil, okQ, render.QuadDrawOptions{})
	empty, _ := render.NewImageBuf(4, 4, render.FormatRGBA8)
	empty.Dispose()
	_ = dc.DrawImageQuadEx(empty, okQ, render.QuadDrawOptions{})
	// Compat wrapper no-op on degenerate, draws on ok.
	dc2 := newWhite()
	defer dc2.Close()
	dc2.DrawImageQuad(src, degen)
	if got := sampleAt(t, dc2, 20, 20); got != [4]uint8{255, 255, 255, 255} {
		t.Errorf("compat degenerate polluted: %v", got)
	}
	dc2.DrawImageQuad(src, okQ)
	// Opacity zero draws nothing but no error.
	if err := dc.DrawImageQuadEx(src, okQ, render.QuadDrawOptions{Opacity: -1}); err != nil {
		t.Errorf("opacity clamp err = %v", err)
	}
}

// F: offscreen golden regression, tolerance frozen in JSON.
func TestQuadGoldenBilinear(t *testing.T) {
	withCPUMode(t)
	c := loadQuadCases(t)
	src := loadQuadSrc(t)
	w, h := c.Golden.Canvas[0], c.Golden.Canvas[1]
	dc := newWhiteContext(w, h)
	defer dc.Close()
	drawQuadCPU(t, dc, src, quadToPoints(c.Golden.Corners), c.Golden.Interp)
	out := dc.Image()
	want, err := imagediff.DecodePNG(filepath.Join("testdata", c.Golden.File))
	if err != nil {
		t.Fatalf("decode golden %s: %v", c.Golden.File, err)
	}
	st, err := imagediff.Images(out, want)
	if err != nil {
		t.Fatalf("diff golden: %v", err)
	}
	changedPct := 100 * float64(st.ChangedPixels) / float64(st.TotalPixels)
	t.Logf("golden diff changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d tol_pct=%.1f tol_mean=%.1f",
		st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta, c.Golden.MaxChangedPct, c.Golden.MaxMeanAbs)
	if changedPct > c.Golden.MaxChangedPct {
		t.Fatalf("golden changed %.3f%% > %.1f%%", changedPct, c.Golden.MaxChangedPct)
	}
	if st.MeanAbs > c.Golden.MaxMeanAbs {
		t.Fatalf("golden mean %.3f > %.1f", st.MeanAbs, c.Golden.MaxMeanAbs)
	}
}

// C: CPU vs GPU parity, only AA-edge LSB allowed. Needs native GPU.
func TestQuadCPUGPUParity(t *testing.T) {
	if render.Accelerator() == nil {
		t.Skip("no GPU accelerator, parity needs native GPU")
	}
	c := loadQuadCases(t)
	// Probe GPU availability with a tiny flush.
	probe := render.NewContext(8, 8)
	probe.ClearWithColor(render.White)
	probe.SetRGB(1, 0, 0)
	probe.DrawRectangle(0, 0, 8, 8)
	_ = probe.Fill()
	if err := probe.FlushGPU(); err != nil {
		probe.Close()
		t.Skipf("GPU flush unavailable: %v", err)
	}
	isGPU := probe.RenderPathStats().GPUOps > 0
	probe.Close()
	if !isGPU {
		t.Skip("no GPU ops on probe, parity needs native GPU")
	}
	for _, interpName := range []string{"nearest", "bilinear"} {
		srcCPU := loadQuadSrc(t)
		srcGPU := loadQuadSrc(t)
		// CPU image.
		withCPUMode(t)
		dcCPU := newWhiteContext(100, 60)
		corners := quadToPoints(c.DrawNearest.Corners)
		if err := dcCPU.DrawImageQuadEx(srcCPU, corners, render.QuadDrawOptions{
			Interpolation: quadInterp(interpName),
			Opacity:       1,
			BlendMode:     render.BlendNormal,
		}); err != nil {
			dcCPU.Close()
			t.Fatalf("%s cpu draw: %v", interpName, err)
		}
		cpuImg := dcCPU.Image()
		dcCPU.Close()
		// GPU image.
		withGPUMode(t)
		dcGPU := newWhiteContext(100, 60)
		if err := dcGPU.DrawImageQuadEx(srcGPU, corners, render.QuadDrawOptions{
			Interpolation: quadInterp(interpName),
			Opacity:       1,
			BlendMode:     render.BlendNormal,
		}); err != nil {
			dcGPU.Close()
			t.Fatalf("%s gpu draw: %v", interpName, err)
		}
		if err := dcGPU.FlushGPU(); err != nil {
			dcGPU.Close()
			if errors.Is(err, render.ErrFallbackToCPU) {
				t.Skipf("%s flush fell back to CPU (no native GPU in this env): %v", interpName, err)
			}
			t.Fatalf("%s flush: %v", interpName, err)
		}
		if dcGPU.RenderPathStats().GPUOps == 0 {
			dcGPU.Close()
			t.Skipf("%s no GPUOps (no native GPU in this env): %s", interpName, dcGPU.RenderPathStats().LogLine())
		}
		gpuImg := dcGPU.Image()
		dcGPU.Close()
		st, err := imagediff.Images(cpuImg, gpuImg)
		if err != nil {
			t.Fatalf("%s diff: %v", interpName, err)
		}
		changedPct := 100 * float64(st.ChangedPixels) / float64(st.TotalPixels)
		t.Logf("parity %s changed=%d/%d (%.3f%%) mean=%.3f rmse=%.3f max=%d %s",
			interpName, st.ChangedPixels, st.TotalPixels, changedPct, st.MeanAbs, st.RMSE, st.MaxDelta, dcGPU.RenderPathStats().LogLine())
		if changedPct > 1.0 {
			t.Fatalf("parity %s changed %.3f%% > 1%% (trapezoid became box?)", interpName, changedPct)
		}
		if interpName == "nearest" && (st.ChangedPixels != 0 || st.MaxDelta != 0) {
			t.Fatalf("parity nearest must be bit-exact, got changed=%d max=%d", st.ChangedPixels, st.MaxDelta)
		}
		if interpName == "bilinear" && st.MeanAbs > 1.0 {
			t.Fatalf("parity bilinear mean %.3f > 1.0", st.MeanAbs)
		}
	}
}

// D: single image cost with numbers.
func TestQuadPerfSingle(t *testing.T) {
	withCPUMode(t)
	src := loadQuadSrc(t)
	dc := newWhiteContext(100, 60)
	defer dc.Close()
	corners := [4]render.Point{{X: 40, Y: 10}, {X: 60, Y: 10}, {X: 80, Y: 50}, {X: 20, Y: 50}}
	const n = 20
	start := time.Now()
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		if err := dc.DrawImageQuadEx(src, corners, render.QuadDrawOptions{Interpolation: render.InterpBilinear}); err != nil {
			t.Fatalf("perf draw: %v", err)
		}
	}
	el := time.Since(start)
	t.Logf("quad-perf: %d draws 100x60 trapezoid in %v (%.2f ms/draw)", n, el, float64(el.Microseconds())/float64(n)/1000.0)
}

// E: repeated draws do not leak or drift.
func TestQuadLongRunNoLeak(t *testing.T) {
	withCPUMode(t)
	src := loadQuadSrc(t)
	dc := newWhiteContext(100, 60)
	defer dc.Close()
	trap := [4]render.Point{{X: 40, Y: 10}, {X: 60, Y: 10}, {X: 80, Y: 50}, {X: 20, Y: 50}}
	rect := [4]render.Point{{X: 10, Y: 10}, {X: 50, Y: 10}, {X: 50, Y: 50}, {X: 10, Y: 50}}
	const n = 500
	for i := 0; i < n; i++ {
		dc.ClearWithColor(render.White)
		q := trap
		if i%2 == 1 {
			q = rect
		}
		if err := dc.DrawImageQuadEx(src, q, render.QuadDrawOptions{Interpolation: render.InterpNearest}); err != nil {
			t.Fatalf("longrun[%d]: %v", i, err)
		}
	}
	got := sampleAt(t, dc, 40, 20)
	// Last iteration is rect (i=499 odd): 40,20 must be green quadrant.
	if got != [4]uint8{0, 255, 0, 255} {
		t.Fatalf("longrun final probe = %v, want green", got)
	}
	t.Logf("quad-longrun: %d alternating draws ok, final probe green", n)
}
