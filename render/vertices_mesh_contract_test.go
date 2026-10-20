package render_test

import (
	"errors"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/testutil/imagediff"
)

// A11 contract regressions: DrawVertices/DrawMesh are a V.03 subset
// (triangle list / fan, optional per-vertex colors, optional uint16 indices).
// A non-empty Indices list always selects the indexed path — it must never be
// silently reinterpreted as a triangle list, and a bad index must never reach
// the GPU. These cases pin the non-Ex and Ex behavior on the CPU path; the
// GPU half lives in vertices_mesh_gpu_test.go.

func vertsMeshQuad() ([]render.Point, []render.RGBA, []uint16) {
	pos := []render.Point{{X: 10, Y: 10}, {X: 54, Y: 10}, {X: 54, Y: 54}, {X: 10, Y: 54}}
	col := []render.RGBA{
		{R: 1, A: 1},
		{G: 1, A: 1},
		{B: 1, A: 1},
		{R: 1, G: 1, A: 1},
	}
	return pos, col, []uint16{0, 1, 2, 0, 2, 3}
}

func vertsMeshNonWhite(t *testing.T, dc *render.Context) int {
	t.Helper()
	img := dc.Image()
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if r>>8 < 250 || g>>8 < 250 || bb>>8 < 250 {
				n++
			}
		}
	}
	return n
}

// One or two indices form no complete triangle: the mesh is skipped, not
// drawn as a triangle list from all positions.
func TestVertsMeshShortIndicesDrawNothingCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, _ := vertsMeshQuad()
	for _, tc := range []struct {
		name string
		idx  []uint16
	}{
		{"one", []uint16{0}},
		{"two", []uint16{0, 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dc := newVertsWhite(64, 64)
			defer dc.Close()
			dc.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: tc.idx})
			if n := vertsMeshNonWhite(t, dc); n != 0 {
				t.Fatalf("DrawMesh short indices drew %d non-white pixels, want 0", n)
			}
			res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: tc.idx}, render.VertDrawOptions{})
			if err != nil {
				t.Fatalf("DrawMeshEx short indices: %v", err)
			}
			if !res.Skipped || res.Triangles != 0 {
				t.Fatalf("DrawMeshEx short indices = %+v, want Skipped with 0 triangles", res)
			}
			if n := vertsMeshNonWhite(t, dc); n != 0 {
				t.Fatalf("DrawMeshEx short indices drew %d non-white pixels, want 0", n)
			}
		})
	}
}

// DrawMeshEx rejects any out-of-range index, including one that lands in an
// incomplete trailing group and would not have been drawn.
func TestVertsMeshExBadIndexRejectedCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, _ := vertsMeshQuad()
	for _, tc := range []struct {
		name string
		idx  []uint16
	}{
		{"full group", []uint16{0, 1, 9}},
		{"partial tail", []uint16{0, 1, 2, 9}},
		{"short bad", []uint16{9}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dc := newVertsWhite(64, 64)
			defer dc.Close()
			if _, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: tc.idx}, render.VertDrawOptions{}); !errors.Is(err, render.ErrVertsBadIndex) {
				t.Fatalf("DrawMeshEx(%v) err = %v, want ErrVertsBadIndex", tc.idx, err)
			}
			if n := vertsMeshNonWhite(t, dc); n != 0 {
				t.Fatalf("rejected mesh drew %d non-white pixels, want 0", n)
			}
		})
	}
}

// The old DrawMesh has no error return; it must match the CPU fallback rule
// and skip only the index groups that contain an out-of-range index.
func TestVertsMeshOldBadIndexSkipsBadGroupsCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, _ := vertsMeshQuad()

	draw := func(idx []uint16) *render.Context {
		dc := newVertsWhite(64, 64)
		dc.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: idx})
		return dc
	}
	bad := draw([]uint16{0, 1, 2, 1, 2, 9})
	defer bad.Close()
	good := draw([]uint16{0, 1, 2})
	defer good.Close()

	st, err := imagediff.Images(good.Image(), bad.Image())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if st.ChangedPixels != 0 {
		t.Fatalf("bad group changed frame: changed=%d max=%d", st.ChangedPixels, st.MaxDelta)
	}
}

// A complete trailing group is ignored (no error), the draw still reports the
// complete triangles.
func TestVertsMeshExPartialTailIgnoredCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, _ := vertsMeshQuad()
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: []uint16{0, 1, 2, 0}}, render.VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshEx partial tail: %v", err)
	}
	if res.Skipped || res.Triangles != 1 || !res.Degraded {
		t.Fatalf("DrawMeshEx partial tail = %+v, want Degraded 1 triangle", res)
	}
	if n := vertsMeshNonWhite(t, dc); n == 0 {
		t.Fatal("partial tail mesh drew nothing")
	}
}

// Mismatched colors fall back to the solid paint color, same as DrawMesh.
func TestVertsMeshMismatchedColorsSolidCPU(t *testing.T) {
	withVertsCPU(t)
	pos, _, idx := vertsMeshQuad()
	dc := newVertsWhite(64, 64)
	defer dc.Close()
	dc.SetRGBA(0.2, 0.9, 0.3, 1)
	res, err := dc.DrawMeshEx(render.Mesh{
		Positions: pos,
		Colors:    []render.RGBA{{R: 1, A: 1}}, // length mismatch: solid fallback
		Indices:   idx,
	}, render.VertDrawOptions{})
	if err != nil {
		t.Fatalf("DrawMeshEx mismatched colors: %v", err)
	}
	if res.Skipped || !res.Degraded || res.Triangles != 2 {
		t.Fatalf("DrawMeshEx mismatched colors = %+v, want Degraded 2 triangles", res)
	}
	got := vertsSample(t, dc, 40, 20)
	if !vertsClose(got, [4]uint8{51, 229, 76, 255}, 2) {
		t.Fatalf("mismatched-color mesh interior = %v, want solid paint ~[51 229 76 255]", got)
	}
}

// Indexed and CPU-expanded rendering must agree bit for bit (same geometry,
// same colors, same triangle order).
func TestVertsMeshIndexedMatchesExpandedCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, idx := vertsMeshQuad()
	expandedPos := []render.Point{pos[0], pos[1], pos[2], pos[0], pos[2], pos[3]}
	expandedCol := []render.RGBA{col[0], col[1], col[2], col[0], col[2], col[3]}

	dcIdx := newVertsWhite(64, 64)
	defer dcIdx.Close()
	dcIdx.DrawMesh(render.Mesh{Positions: pos, Colors: col, Indices: idx})

	dcExp := newVertsWhite(64, 64)
	defer dcExp.Close()
	dcExp.DrawVertices(expandedPos, expandedCol, render.VertexModeTriangles)

	st, err := imagediff.Images(dcIdx.Image(), dcExp.Image())
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	if st.ChangedPixels != 0 {
		t.Fatalf("indexed vs expanded: changed=%d max=%d mean=%.4f", st.ChangedPixels, st.MaxDelta, st.MeanAbs)
	}
}

// Nil/empty Indices keeps the historical triangle-list interpretation.
func TestVertsMeshNilIndicesTriangleListCPU(t *testing.T) {
	withVertsCPU(t)
	pos, col, _ := vertsMeshQuad()
	pos = append(pos, pos[0], pos[2]) // two triangles as a plain list
	col = append(col, col[0], col[2])
	for _, idx := range [][]uint16{nil, {}} {
		dc := newVertsWhite(64, 64)
		res, err := dc.DrawMeshEx(render.Mesh{Positions: pos, Colors: col, Indices: idx}, render.VertDrawOptions{})
		dc.Close()
		if err != nil {
			t.Fatalf("DrawMeshEx nil indices: %v", err)
		}
		if res.Skipped || res.Triangles != 2 || !res.Degraded {
			t.Fatalf("DrawMeshEx nil indices = %+v, want Degraded 2 triangles", res)
		}
	}
}
