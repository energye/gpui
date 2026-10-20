//go:build !nogpu

package render_test

import (
	"errors"
	"testing"

	"github.com/energye/gpui/render"
	_ "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

// requireVertsGPU skips when the native GPU mesh path is unavailable so the
// A11 regressions run on the real path in CI and skip cleanly elsewhere.
// A non-indexed vertex flush is the cheapest probe: it fails with
// ErrFallbackToCPU without a native device and never exercises the index
// upload path that these tests are meant to verify.
func requireVertsGPU(t *testing.T) {
	t.Helper()
	if render.Accelerator() == nil {
		t.Skip("GPU accelerator not registered")
	}
	dc := render.NewContext(16, 16)
	defer dc.Close()
	dc.ResetRenderPathStats()
	dc.DrawVertices([]render.Point{{X: 1, Y: 1}, {X: 14, Y: 1}, {X: 1, Y: 14}},
		[]render.RGBA{{R: 1, A: 1}, {G: 1, A: 1}, {B: 1, A: 1}}, render.VertexModeTriangles)
	if err := dc.FlushGPU(); err != nil {
		if errors.Is(err, render.ErrFallbackToCPU) {
			t.Skipf("native GPU mesh path unavailable: %v", err)
		}
		t.Fatalf("GPU probe flush: %v", err)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 {
		t.Skipf("native GPU mesh path unavailable: %s", st.LogLine())
	}
}

// A mesh with one triangle (3 indices, 6 bytes) used to fail WebGPU
// Queue.WriteBuffer's 4-byte alignment rule. It must upload, flush and draw.
func TestVertsMeshGPUOddIndexCountUploads(t *testing.T) {
	requireVertsGPU(t)
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	tri := render.Mesh{
		Positions: []render.Point{{X: 8, Y: 56}, {X: 56, Y: 56}, {X: 32, Y: 8}},
		Colors:    []render.RGBA{{R: 1, A: 1}, {G: 1, A: 1}, {B: 1, A: 1}},
		Indices:   []uint16{0, 1, 2},
	}
	res, err := dc.DrawMeshEx(tri, render.VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshEx 3 indices: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("3-index mesh flush: %v", err)
	}
	if res.Degraded || res.Triangles != 1 {
		t.Fatalf("3-index mesh Ex = %+v, want GPU 1 triangle", res)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("3-index mesh path stats: %s", st.LogLine())
	}
	if n := vertsMeshNonWhite(t, dc); n == 0 {
		t.Fatal("3-index mesh drew nothing")
	}
}

// Short index lists must not be reinterpreted as a triangle list on GPU either.
func TestVertsMeshGPUShortIndicesDrawNothing(t *testing.T) {
	requireVertsGPU(t)
	pos, col, _ := vertsMeshQuad()
	for _, idx := range [][]uint16{{0}, {0, 1}} {
		dc := newVertsWhite(64, 64)
		dc.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: idx})
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("short-indices flush: %v", err)
		}
		if n := vertsMeshNonWhite(t, dc); n != 0 {
			t.Fatalf("GPU DrawMesh short indices drew %d non-white pixels, want 0", n)
		}
		dc.Close()
	}
}

// An out-of-range index in one mesh must never read the vertices of the next
// mesh packed into the same GPU vertex buffer (multi-cmd contiguous packing).
func TestVertsMeshGPUBadIndexDoesNotBleed(t *testing.T) {
	requireVertsGPU(t)
	mkMesh1 := func(idx []uint16) render.Mesh {
		return render.Mesh{
			Positions: []render.Point{{X: 8, Y: 8}, {X: 52, Y: 8}, {X: 52, Y: 52}, {X: 8, Y: 52}},
			Colors:    []render.RGBA{{R: 1, A: 1}, {R: 1, G: 1, A: 1}, {R: 1, G: 0.5, A: 1}, {R: 1, G: 0, A: 1}},
			Indices:   idx,
		}
	}
	mesh2 := render.Mesh{
		Positions: []render.Point{{X: 220, Y: 8}, {X: 250, Y: 8}, {X: 250, Y: 40}},
		Colors:    []render.RGBA{{B: 1, A: 1}, {B: 1, A: 1}, {B: 1, A: 1}},
		Indices:   []uint16{0, 1, 2},
	}
	drawFrame := func(m1 render.Mesh) *render.Context {
		dc := newVertsWhite(256, 64)
		dc.DrawMesh(m1)
		dc.DrawMesh(mesh2)
		if err := dc.FlushGPU(); err != nil {
			t.Fatalf("bleed flush: %v", err)
		}
		return dc
	}
	bad := drawFrame(mkMesh1([]uint16{0, 1, 2, 1, 2, 4}))
	defer bad.Close()
	good := drawFrame(mkMesh1([]uint16{0, 1, 2}))
	defer good.Close()

	st, err := imagediff.Images(good.Image(), bad.Image())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if st.ChangedPixels != 0 {
		t.Fatalf("out-of-range index changed the frame: changed=%d max=%d mean=%.3f", st.ChangedPixels, st.MaxDelta, st.MeanAbs)
	}
}

// GPU indexed and GPU expanded must agree (same geometry/order, both SkipAA).
func TestVertsMeshGPUIndexedMatchesExpanded(t *testing.T) {
	requireVertsGPU(t)
	pos, col, idx := vertsMeshQuad()
	expandedPos := []render.Point{pos[0], pos[1], pos[2], pos[0], pos[2], pos[3]}
	expandedCol := []render.RGBA{col[0], col[1], col[2], col[0], col[2], col[3]}

	dcIdx := newVertsWhite(64, 64)
	defer dcIdx.Close()
	dcIdx.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: idx})
	if err := dcIdx.FlushGPU(); err != nil {
		t.Fatalf("indexed flush: %v", err)
	}
	dcExp := newVertsWhite(64, 64)
	defer dcExp.Close()
	dcExp.DrawVertices(expandedPos, expandedCol, render.VertexModeTriangles)
	if err := dcExp.FlushGPU(); err != nil {
		t.Fatalf("expanded flush: %v", err)
	}
	st, err := imagediff.Images(dcIdx.Image(), dcExp.Image())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if st.ChangedPixels != 0 {
		t.Fatalf("GPU indexed vs expanded: changed=%d max=%d mean=%.4f", st.ChangedPixels, st.MaxDelta, st.MeanAbs)
	}
}

// Ex must report the real GPU route (no degrade) when the mesh is GPU-drawn.
func TestVertsMeshExGPUReportsGPU(t *testing.T) {
	requireVertsGPU(t)
	pos, col, idx := vertsMeshQuad()
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.ResetRenderPathStats()
	res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: idx}, render.VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshEx: %v", err)
	}
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if res.Degraded || res.Reason != "" || res.Triangles != 2 {
		t.Fatalf("DrawMeshEx GPU = %+v, want 2 triangles no degrade", res)
	}
	if st := dc.RenderPathStats(); st.GPUOps == 0 || st.CPUFallbackOps != 0 {
		t.Fatalf("DrawMeshEx path stats: %s", st.LogLine())
	}
	if reason := dc.LastCPUFallbackReason(); reason != "" {
		t.Fatalf("LastCPUFallbackReason = %q, want empty", reason)
	}
}
