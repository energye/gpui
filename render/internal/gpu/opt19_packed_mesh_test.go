//go:build !nogpu

package gpu

import (
	"bytes"
	"testing"

	"github.com/energye/gpui/render"
)

// TestOpt19_PackedVerts_MatchesTriangleListPack verifies pre-packed mesh
// vertices produce identical GPU vertex bytes to classic Points+VC packing.
func TestOpt19_PackedVerts_MatchesTriangleListPack(t *testing.T) {
	pts := []render.Point{
		{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 15, Y: 20},
		{X: 30, Y: 30}, {X: 40, Y: 30}, {X: 35, Y: 45},
	}
	cols := [][4]float32{
		{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1},
		{0.5, 0.5, 0, 1}, {0, 0.5, 0.5, 1}, {0.5, 0, 0.5, 1},
	}
	classic := []ConvexDrawCommand{{
		Points:       pts,
		VertexColors: cols,
		SkipAA:       true,
		TriangleList: true,
	}}
	// Build packed blob the same way writeConvexVertex would.
	packed := make([]byte, len(pts)*convexMeshVertexStride)
	for i := range pts {
		writeConvexMeshVertex(packed[i*convexMeshVertexStride:], float32(pts[i].X), float32(pts[i].Y), cols[i])
	}
	pre := []ConvexDrawCommand{{
		SkipAA:       true,
		TriangleList: true,
		PackedVerts:  packed,
	}}
	_, a := buildConvexVerticesReuse(classic, nil)
	_, b := buildConvexVerticesReuse(pre, nil)
	if !bytes.Equal(a, b) {
		t.Fatalf("packed verts diverge: classic=%d pre=%d", len(a), len(b))
	}
	if convexCmdVertexCount(&pre[0]) != 6 {
		t.Fatalf("vertex count=%d want 6", convexCmdVertexCount(&pre[0]))
	}
}
