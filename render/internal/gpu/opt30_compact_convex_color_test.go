//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

func TestOpt30_ConvexVertexStride_Is16(t *testing.T) {
	if convexVertexStride != 16 {
		t.Fatalf("stride=%d want 16", convexVertexStride)
	}
	layout := convexVertexLayout()
	if layout[0].ArrayStride != 16 {
		t.Fatal(layout[0].ArrayStride)
	}
	if layout[0].Attributes[2].Format != types.VertexFormatUnorm8x4 {
		t.Fatalf("color format=%v want Unorm8x4", layout[0].Attributes[2].Format)
	}
}

func TestOpt30_PackColorUnorm8x4_ExactSolids(t *testing.T) {
	for _, c := range [][4]float32{{1, 0, 0, 1}, {0, 1, 0, 1}, {0, 0, 1, 1}, {1, 1, 1, 1}, {0, 0, 0, 0}} {
		u := packColorUnorm8x4(c)
		got := unpackColorUnorm8x4(u)
		for i := 0; i < 4; i++ {
			if got[i] != c[i] {
				t.Fatalf("%v → %v", c, got)
			}
		}
	}
}

func TestOpt30_PackMeshVerts_MatchesWriteConvexVertex(t *testing.T) {
	positions := []render.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}}
	colors := []render.RGBA{{R: 1, G: 0, B: 0, A: 1}, {R: 0, G: 1, B: 0, A: 1}, {R: 0, G: 0, B: 1, A: 1}}
	dst := make([]byte, len(positions)*convexMeshVertexStride)
	_ = packMeshVertsCoverage1(dst, positions, colors, true)
	for i := range positions {
		ref := make([]byte, convexMeshVertexStride)
		a := float32(colors[i].A)
		col := [4]float32{float32(colors[i].R) * a, float32(colors[i].G) * a, float32(colors[i].B) * a, a}
		writeConvexMeshVertex(ref, float32(positions[i].X), float32(positions[i].Y), col)
		off := i * convexMeshVertexStride
		for j := 0; j < convexMeshVertexStride; j++ {
			if dst[off+j] != ref[j] {
				t.Fatalf("vert %d byte %d: pack=%d write=%d", i, j, dst[off+j], ref[j])
			}
		}
	}
}

func TestOpt30_BuildConvex_WhiteSolid_ColorBits(t *testing.T) {
	cmds := []ConvexDrawCommand{{
		Points: []render.Point{{X: 8, Y: 8}, {X: 56, Y: 8}, {X: 32, Y: 56}},
		Color:  [4]float32{1, 1, 1, 1},
		SkipAA: true,
	}}
	data := BuildConvexVertices(cmds)
	if len(data) < convexVertexStride {
		t.Fatal("no verts")
	}
	col := unpackColorUnorm8x4(binary.LittleEndian.Uint32(data[12:16]))
	if col[0] != 1 || col[1] != 1 || col[2] != 1 || col[3] != 1 {
		t.Fatalf("white solid color quantize failed: %v", col)
	}
	cov := math.Float32frombits(binary.LittleEndian.Uint32(data[8:12]))
	if cov != 1 {
		t.Fatalf("cov=%v", cov)
	}
}
