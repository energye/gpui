//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render"
)

// TestOpt23_PackMeshVertsCoverage1_MatchesWriteConvexVertex ensures the opt23
// tight pack path produces identical 28-byte verts to writeConvexVertex.
func TestOpt23_PackMeshVertsCoverage1_MatchesWriteConvexVertex(t *testing.T) {
	positions := []render.Point{{X: 1.5, Y: 2.25}, {X: 10, Y: 0}, {X: 0, Y: 10}, {X: 4, Y: 8}}
	colors := []render.RGBA{
		{R: 1, G: 0, B: 0, A: 0.5},
		{R: 0, G: 1, B: 0, A: 1},
		{R: 0, G: 0, B: 1, A: 0.25},
		{R: 1, G: 1, B: 0, A: 0.8},
	}
	dst := make([]byte, len(positions)*convexVertexStride)
	gotSolid := packMeshVertsCoverage1(dst, positions, colors, true)

	ref := make([]byte, len(positions)*convexVertexStride)
	var refSolid [4]float32
	for i := range positions {
		c := colors[i]
		a := float32(c.A)
		col := [4]float32{float32(c.R) * a, float32(c.G) * a, float32(c.B) * a, a}
		if i < 3 {
			refSolid[0] += col[0]
			refSolid[1] += col[1]
			refSolid[2] += col[2]
			refSolid[3] += col[3]
		}
		writeConvexVertex(ref[i*convexVertexStride:], float32(positions[i].X), float32(positions[i].Y), 1.0, col)
	}
	refSolid[0] /= 3
	refSolid[1] /= 3
	refSolid[2] /= 3
	refSolid[3] /= 3
	for i := range dst {
		if dst[i] != ref[i] {
			t.Fatalf("byte mismatch at %d: got %02x want %02x", i, dst[i], ref[i])
		}
	}
	for i := 0; i < 4; i++ {
		if gotSolid[i] != refSolid[i] {
			t.Fatalf("solid[%d]=%v want %v", i, gotSolid[i], refSolid[i])
		}
	}

	// Solid-color path
	dst2 := make([]byte, len(positions)*convexVertexStride)
	solid := packMeshVertsCoverage1(dst2, positions, []render.RGBA{{R: 0.2, G: 0.4, B: 0.6, A: 0.5}}, false)
	ref2 := make([]byte, len(positions)*convexVertexStride)
	a := float32(0.5)
	col := [4]float32{0.2 * a, 0.4 * a, 0.6 * a, a}
	for i := range positions {
		writeConvexVertex(ref2[i*convexVertexStride:], float32(positions[i].X), float32(positions[i].Y), 1.0, col)
	}
	for i := range dst2 {
		if dst2[i] != ref2[i] {
			t.Fatalf("solid pack byte mismatch at %d", i)
		}
	}
	if solid != col {
		t.Fatalf("solid color %v want %v", solid, col)
	}
}
