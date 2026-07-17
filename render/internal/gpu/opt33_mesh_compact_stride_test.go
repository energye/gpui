//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

func TestOpt33_MeshVertexStride_Is12(t *testing.T) {
	if convexMeshVertexStride != 12 {
		t.Fatalf("mesh stride=%d want 12", convexMeshVertexStride)
	}
	if convexVertexStride != 16 {
		t.Fatalf("aa stride=%d want 16", convexVertexStride)
	}
	layout := convexMeshVertexLayout()
	if layout[0].ArrayStride != 12 {
		t.Fatal(layout[0].ArrayStride)
	}
	if layout[0].Attributes[1].Format != types.VertexFormatUnorm8x4 {
		t.Fatalf("color fmt=%v", layout[0].Attributes[1].Format)
	}
	if layout[0].Attributes[1].ShaderLocation != 1 {
		t.Fatal("color location")
	}
}

func TestOpt33_AllConvexCommandsMeshCompact(t *testing.T) {
	pk := make([]byte, 3*convexMeshVertexStride)
	cmds := []ConvexDrawCommand{{
		TriangleList: true, SkipAA: true, PackedVerts: pk,
	}}
	if !allConvexCommandsMeshCompact(cmds) {
		t.Fatal("expected compact")
	}
	cmds[0].BlendMode = render.BlendMultiply
	if allConvexCommandsMeshCompact(cmds) {
		t.Fatal("non-normal blend must not compact")
	}
	cmds[0].BlendMode = render.BlendNormal
	cmds = append(cmds, ConvexDrawCommand{
		Points: []render.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}},
		Color:  [4]float32{1, 1, 1, 1},
	})
	if allConvexCommandsMeshCompact(cmds) {
		t.Fatal("mixed AA must not compact")
	}
}

func TestOpt33_PackMesh_12B_NoCoverageChannel(t *testing.T) {
	pos := []render.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}}
	col := []render.RGBA{{R: 1, G: 0, B: 0, A: 1}}
	dst := make([]byte, len(pos)*convexMeshVertexStride)
	_ = packMeshVertsCoverage1(dst, pos, col, false)
	// color at offset 8; no f32 1.0 at 8
	bits8 := binary.LittleEndian.Uint32(dst[8:12])
	if bits8 == math.Float32bits(1.0) {
		// solid red unorm is 0xFF0000FF or similar LE - not 1.0f
	}
	want := packColorUnorm8x4RGBA(1, 0, 0, 1)
	if bits8 != want {
		t.Fatalf("color=%#08x want %#08x", bits8, want)
	}
}

func TestOpt33_ExpandMesh12To16_InBuildConvex(t *testing.T) {
	// Mixed batch: packed mesh + AA path expands mesh to 16B layout.
	pk := make([]byte, 3*convexMeshVertexStride)
	writeConvexMeshVertex(pk[0:], 1, 2, [4]float32{1, 0, 0, 1})
	writeConvexMeshVertex(pk[12:], 3, 4, [4]float32{0, 1, 0, 1})
	writeConvexMeshVertex(pk[24:], 5, 6, [4]float32{0, 0, 1, 1})
	cmds := []ConvexDrawCommand{
		{TriangleList: true, SkipAA: true, PackedVerts: pk},
		{
			Points: []render.Point{{X: 10, Y: 10}, {X: 20, Y: 10}, {X: 15, Y: 20}},
			Color:  [4]float32{1, 1, 1, 1},
			// AA on
		},
	}
	if allConvexCommandsMeshCompact(cmds) {
		t.Fatal("mixed should not be compact")
	}
	data := BuildConvexVertices(cmds)
	// first 3 verts are expanded mesh at 16B
	cov := math.Float32frombits(binary.LittleEndian.Uint32(data[8:12]))
	if cov != 1 {
		t.Fatalf("expanded cov=%v", cov)
	}
	col := unpackColorUnorm8x4(binary.LittleEndian.Uint32(data[12:16]))
	if col[0] != 1 || col[3] != 1 {
		t.Fatalf("expanded color %v", col)
	}
}
