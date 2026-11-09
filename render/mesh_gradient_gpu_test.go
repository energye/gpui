//go:build !nogpu

package render_test

import (
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

func mgGPUGrid(t *testing.T) *render.MeshGradient {
	t.Helper()
	colors := [9]render.RGBA{
		{R: 1, A: 1}, {G: 1, A: 1}, {B: 1, A: 1},
		{R: 1, G: 1, A: 1}, {R: 1, B: 1, A: 1}, {G: 1, B: 1, A: 1},
		{R: 1, G: 0.5, A: 1}, {G: 1, B: 0.5, A: 1}, {R: 0.5, B: 1, A: 1},
	}
	vs := make([]render.MeshGradientVertex, 9)
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			vs[r*3+c] = render.MeshGradientVertex{
				Pos:   render.Pt(float64(c)*100, float64(r)*100),
				Color: colors[r*3+c],
			}
		}
	}
	g, err := render.NewMeshGradient(2, 2, vs)
	if err != nil {
		t.Fatalf("NewMeshGradient: %v", err)
	}
	return g
}

func TestMeshGradientGPUProbes(t *testing.T) {
	requireVertsGPU(t)
	g := mgGPUGrid(t)
	dc := newVertsWhite(220, 220)
	defer dc.Close()
	dc.ResetRenderPathStats()
	if _, err := dc.DrawMeshGradientEx(g, render.VertDrawOptions{}); err != nil {
		t.Fatalf("DrawMeshGradientEx: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("GPU path stats: %s", st.LogLine())
	}
	probes := []struct {
		x, y int
		want [4]uint8
		tol  uint8
	}{
		{3, 3, [4]uint8{255, 0, 0, 255}, 18},
		{197, 3, [4]uint8{0, 0, 255, 255}, 18},
		{197, 197, [4]uint8{128, 0, 255, 255}, 24},
		{3, 197, [4]uint8{255, 128, 0, 255}, 24},
		{100, 100, [4]uint8{255, 0, 255, 255}, 24},
	}
	for _, pr := range probes {
		got := vertsSample(t, dc, pr.x, pr.y)
		if !vertsClose(got, pr.want, pr.tol) {
			t.Errorf("(%d,%d) = %v want %v tol %d", pr.x, pr.y, got, pr.want, pr.tol)
		}
	}
}

func TestMeshGradientGPUMatchesCPU(t *testing.T) {
	requireVertsGPU(t)
	g := mgGPUGrid(t)

	gpu := newVertsWhite(220, 220)
	defer gpu.Close()
	gpu.DrawMeshGradient(g)
	if err := gpu.FlushGPU(); err != nil {
		t.Fatalf("gpu flush: %v", err)
	}
	gpuImg := gpu.Image()

	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	cpu := newVertsWhite(220, 220)
	defer cpu.Close()
	cpu.DrawMeshGradient(g)
	cpuImg := cpu.Image()

	st, err := imagediff.Images(cpuImg, gpuImg)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	total := int64(gpuImg.Bounds().Dx() * gpuImg.Bounds().Dy())
	if int64(st.ChangedPixels) > total/1000 || st.MaxDelta > 2 {
		t.Fatalf("GPU vs CPU changed=%d/%d max=%d mean=%.3f, want <=0.1%% and max<=2",
			st.ChangedPixels, total, st.MaxDelta, st.MeanAbs)
	}
}

func TestMeshGradientGPUSeams(t *testing.T) {
	requireVertsGPU(t)
	g := mgGPUGrid(t)
	dc := render.NewContext(220, 220)
	defer dc.Close()
	dc.Clear()
	dc.DrawMeshGradient(g)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	seams := [][2]int{{100, 50}, {100, 150}, {50, 100}, {150, 100}, {100, 100}}
	for _, s := range seams {
		got := vertsSample(t, dc, s[0], s[1])
		if got[3] == 0 {
			t.Errorf("seam %v alpha=0 (gap)", s)
		}
	}
}
