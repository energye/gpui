//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"testing"

	"github.com/energye/gpui/render"
)

// refPackColorUnorm8x4 is the pre-opt31 reference (array + clampUnorm8).
// Kept only in tests to prove opt31 scalar pack is bit-identical.
func refPackColorUnorm8x4(color [4]float32) uint32 {
	return uint32(clampUnorm8(color[0])) |
		uint32(clampUnorm8(color[1]))<<8 |
		uint32(clampUnorm8(color[2]))<<16 |
		uint32(clampUnorm8(color[3]))<<24
}

func TestOpt31_PackColorUnorm8x4RGBA_MatchesArray(t *testing.T) {
	cases := [][4]float32{
		{1, 0, 0, 1},
		{0, 1, 0, 1},
		{0, 0, 1, 1},
		{1, 1, 1, 1},
		{0, 0, 0, 0},
		{0.5, 0.25, 0.125, 0.75},
		{0.2, 0.4, 0.6, 0.5},
		{-0.1, 1.5, 0.999, 0.001},
		{1.0 / 255, 2.0 / 255, 254.0 / 255, 255.0 / 255},
	}
	for _, c := range cases {
		want := refPackColorUnorm8x4(c)
		gotA := packColorUnorm8x4(c)
		gotB := packColorUnorm8x4RGBA(c[0], c[1], c[2], c[3])
		if gotA != want || gotB != want {
			t.Fatalf("%v: ref=%#08x pack=%#08x rgba=%#08x", c, want, gotA, gotB)
		}
	}
}

func TestOpt31_PackMeshVerts_MatchesWriteConvexVertex(t *testing.T) {
	positions := []render.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}, {X: 7.5, Y: -1}}
	colors := []render.RGBA{
		{R: 1, G: 0, B: 0, A: 1},
		{R: 0, G: 1, B: 0, A: 0.5},
		{R: 0.2, G: 0.4, B: 0.6, A: 0.75},
		{R: 0.9, G: 0.1, B: 0.3, A: 0.2},
	}
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
		// Golden: color bits match scalar quantizer.
		wantCol := packColorUnorm8x4RGBA(col[0], col[1], col[2], col[3])
		gotCol := binary.LittleEndian.Uint32(dst[off+8 : off+12])
		if gotCol != wantCol {
			t.Fatalf("vert %d color bits %#08x want %#08x", i, gotCol, wantCol)
		}
	}
}

func TestOpt31_PackMeshVerts_SolidColor_NoPerVertQuant(t *testing.T) {
	positions := []render.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 0, Y: 1}}
	colors := []render.RGBA{{R: 0.25, G: 0.5, B: 0.75, A: 0.5}}
	dst := make([]byte, len(positions)*convexMeshVertexStride)
	solid := packMeshVertsCoverage1(dst, positions, colors, false)
	want := packColorUnorm8x4RGBA(solid[0], solid[1], solid[2], solid[3])
	for i := range positions {
		got := binary.LittleEndian.Uint32(dst[i*convexMeshVertexStride+8 : i*convexMeshVertexStride+12])
		if got != want {
			t.Fatalf("vert %d color %#08x want %#08x", i, got, want)
		}
	}
}
