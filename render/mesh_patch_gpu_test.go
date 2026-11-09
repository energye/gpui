//go:build !nogpu

package render_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

type gpuMeshPatchCase struct {
	Style  string       `json:"style"`
	Points [][2]float64 `json:"points"`
	Colors [][4]float64 `json:"colors"`
}

type gpuMeshPatchGrid struct {
	Patches []gpuMeshPatchCase `json:"patches"`
}

func loadGPUMeshPatch(t *testing.T, name string) render.MeshPatch {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_patch_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("decode cases: %v", err)
	}
	var c gpuMeshPatchCase
	if err := json.Unmarshal(all[name], &c); err != nil {
		t.Fatalf("decode %s: %v", name, err)
	}
	return gpuMeshPatchFromCase(t, c)
}

func gpuMeshPatchFromCase(t *testing.T, c gpuMeshPatchCase) render.MeshPatch {
	t.Helper()
	p := render.MeshPatch{Style: render.MeshPatchCoons}
	if c.Style == "bicubic" {
		p.Style = render.MeshPatchBicubic
	}
	for _, pt := range c.Points {
		p.Points = append(p.Points, render.Pt(pt[0], pt[1]))
	}
	for i := 0; i < 4 && i < len(c.Colors); i++ {
		p.Colors[i] = render.RGBA{R: c.Colors[i][0], G: c.Colors[i][1], B: c.Colors[i][2], A: c.Colors[i][3]}
	}
	return p
}

func TestMeshPatchGPUCornerColors(t *testing.T) {
	requireVertsGPU(t)
	patch := loadGPUMeshPatch(t, "coons_square")
	dc := newVertsWhite(128, 128)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.DrawMeshPatch(patch)
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
		{3, 3, [4]uint8{255, 0, 0, 255}, 16},
		{97, 3, [4]uint8{0, 255, 0, 255}, 16},
		{97, 97, [4]uint8{0, 0, 255, 255}, 16},
		{3, 97, [4]uint8{255, 255, 0, 255}, 16},
		{50, 50, [4]uint8{128, 128, 64, 255}, 20},
	}
	for _, pr := range probes {
		got := vertsSample(t, dc, pr.x, pr.y)
		if !vertsClose(got, pr.want, pr.tol) {
			t.Errorf("(%d,%d) = %v want %v tol %d", pr.x, pr.y, got, pr.want, pr.tol)
		}
	}
}

func TestMeshPatchGPUMatchesCPU(t *testing.T) {
	requireVertsGPU(t)
	patch := loadGPUMeshPatch(t, "coons_square")

	gpu := newVertsWhite(128, 128)
	defer gpu.Close()
	gpu.DrawMeshPatch(patch)
	if err := gpu.FlushGPU(); err != nil {
		t.Fatalf("gpu flush: %v", err)
	}
	gpuImg := gpu.Image()

	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	cpu := newVertsWhite(128, 128)
	defer cpu.Close()
	cpu.DrawMeshPatch(patch)
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

func TestMeshPatchGPUIndexedSeams(t *testing.T) {
	requireVertsGPU(t)
	raw, err := os.ReadFile(filepath.Join("testdata", "mesh_patch_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var all map[string]json.RawMessage
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatalf("decode cases: %v", err)
	}
	var grid gpuMeshPatchGrid
	if err := json.Unmarshal(all["grid_2x2"], &grid); err != nil {
		t.Fatalf("decode grid_2x2: %v", err)
	}
	dc := render.NewContext(220, 220)
	defer dc.Close()
	dc.Clear() // transparent: any seam gap shows as alpha 0
	for _, pc := range grid.Patches {
		dc.DrawMeshPatch(gpuMeshPatchFromCase(t, pc))
	}
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
