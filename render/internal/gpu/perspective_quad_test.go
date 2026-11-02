//go:build !nogpu

package gpu

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

func perspCmd() *ImageDrawCommand {
	return &ImageDrawCommand{
		GenerationID: 7,
		ImgWidth:     20,
		ImgHeight:    1,
		Opacity:      1,
		TLX:          0.5, TLY: 0.5,
		TRX: 240.5, TRY: 0.5,
		BRX: 240.5, BRY: 120.5,
		BLX: 0.5, BLY: 120.5,
		U0: 0, V0: 0, U1: 1, V1: 1,
		Perspective: true,
		Weights:     [4]float32{200, 100, 100, 200},
	}
}

func TestPerspVertexLayoutAndWriter(t *testing.T) {
	layout := perspVertexLayout()
	if len(layout) != 1 || len(layout[0].Attributes) != 4 {
		t.Fatalf("persp layout = %+v, want one buffer with 4 attributes", layout)
	}
	if layout[0].ArrayStride != imagePerspVertexStride {
		t.Fatalf("stride = %d, want %d", layout[0].ArrayStride, imagePerspVertexStride)
	}
	wantAttrs := []struct {
		off uint64
		loc uint32
		fmt types.VertexFormat
	}{
		{0, 0, types.VertexFormatFloat32x2},
		{8, 1, types.VertexFormatFloat32x2},
		{16, 2, types.VertexFormatFloat32x4},
		{32, 3, types.VertexFormatFloat32},
	}
	for i, want := range wantAttrs {
		got := layout[0].Attributes[i]
		if got.Offset != want.off || got.ShaderLocation != want.loc || got.Format != want.fmt {
			t.Fatalf("attr[%d] = (%v,off %d,loc %d), want (%v,off %d,loc %d)",
				i, got.Format, got.Offset, got.ShaderLocation, want.fmt, want.off, want.loc)
		}
	}
}

func TestBuildPerspVerticesIntoWireFormat(t *testing.T) {
	cmd := perspCmd()
	dst := make([]byte, 6*imagePerspVertexStride)
	buildPerspVerticesInto(dst, cmd)

	// TL, TR, BL, TR, BR, BL (same twin split as the affine writer).
	wantW := [6]float32{200, 100, 200, 100, 100, 200}
	wantX := [6]float32{0.5, 240.5, 0.5, 240.5, 240.5, 0.5}
	wantY := [6]float32{0.5, 0.5, 120.5, 0.5, 120.5, 120.5}
	wantU := [6]float32{0, 1, 0, 1, 1, 0}
	wantV := [6]float32{0, 0, 1, 0, 1, 1}
	for i := 0; i < 6; i++ {
		off := i * imagePerspVertexStride
		f32 := func(k int) float32 {
			return math.Float32frombits(binary.LittleEndian.Uint32(dst[off+4*k:]))
		}
		if f32(0) != wantX[i] || f32(1) != wantY[i] {
			t.Fatalf("vert[%d] pos = (%v,%v), want (%v,%v)", i, f32(0), f32(1), wantX[i], wantY[i])
		}
		if f32(2) != wantU[i] || f32(3) != wantV[i] {
			t.Fatalf("vert[%d] uv = (%v,%v), want (%v,%v)", i, f32(2), f32(3), wantU[i], wantV[i])
		}
		// tint identity white at 16..31
		for k := 4; k < 8; k++ {
			if f32(k) != 1 {
				t.Fatalf("vert[%d] tint channel = %v, want 1", i, f32(k))
			}
		}
		if f32(8) != wantW[i] {
			t.Fatalf("vert[%d] weight = %v, want %v", i, f32(8), wantW[i])
		}
	}
}

func TestBuildPerspVerticesIntoPanicsOnShortDst(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("short dst must panic, not overrun")
		}
	}()
	buildPerspVerticesInto(make([]byte, 6*imagePerspVertexStride-1), perspCmd())
}

func TestCanMergeImageDrawIsolatesPerspective(t *testing.T) {
	affine := ImageDrawCommand{GenerationID: 7, ImgWidth: 20, ImgHeight: 1, Opacity: 1}
	persp := *perspCmd()
	if canMergeImageDraw(&persp, &affine) || canMergeImageDraw(&affine, &persp) {
		t.Fatal("perspective quad merged with an affine quad")
	}
	other := persp
	if canMergeImageDraw(&persp, &other) {
		t.Fatal("perspective quads must never merge (one draw each)")
	}
	// Affine behavior unchanged: same-key pairs still merge.
	two := affine
	if !canMergeImageDraw(&affine, &two) {
		t.Fatal("identical affine quads stopped merging")
	}
}

func TestImageDrawCommandPerspectiveZeroValueIsAffine(t *testing.T) {
	var cmd ImageDrawCommand
	if cmd.Perspective || cmd.Weights != [4]float32{} {
		t.Fatal("zero-value command must keep the historic affine emission")
	}
	// The affine writer ignores Perspective entirely.
	buf := make([]byte, 6*imageVertexStride)
	cmd.BlendMode = render.BlendNormal
	buildImageVerticesInto(buf, &cmd)
}

// TestSliceImageResourcesGroupsByCommandIndex pins the clip-group contract:
// a draw call belongs to the group whose COMMAND range it covers. Perspective
// calls pack into their own 36-byte buffer, so their firstVertex counts
// perspective quads only — slicing by firstVertex handed every projective draw
// to the previous group and the last one lost its group entirely (the plate was
// then over-painted by geometry queued later in the frame).
func TestSliceImageResourcesGroupsByCommandIndex(t *testing.T) {
	combined := &imageFrameResources{
		drawCalls: []imageDrawCall{
			{firstVertex: 0, vertexCount: 6, cmdStart: 0, cmdCount: 1},
			{firstVertex: 0, vertexCount: 6, perspective: true, cmdStart: 1, cmdCount: 1},
			{firstVertex: 6, vertexCount: 6, perspective: true, cmdStart: 2, cmdCount: 1},
			{firstVertex: 12, vertexCount: 6, perspective: true, cmdStart: 3, cmdCount: 1},
		},
	}
	s := &GPURenderSession{}
	cases := []struct {
		start, count int
		wantFirstV   []uint32
	}{
		{0, 1, []uint32{0}},
		{1, 1, []uint32{0}}, // the projective call of command 1, not command 2's
		{2, 1, []uint32{6}},
		{3, 1, []uint32{12}},
		{0, 2, []uint32{0, 0}},
		{1, 3, []uint32{0, 6, 12}},
		{2, 2, []uint32{6, 12}},
	}
	for _, c := range cases {
		res := s.sliceImageResources(combined, c.start, c.count)
		got := []uint32{}
		if res != nil {
			for _, dc := range append([]imageDrawCall(nil), res.drawCalls...) {
				got = append(got, dc.firstVertex)
			}
		}
		if len(got) != len(c.wantFirstV) {
			t.Fatalf("group [%d,%d) = %v, want %v", c.start, c.start+c.count, got, c.wantFirstV)
		}
		for i := range got {
			if got[i] != c.wantFirstV[i] {
				t.Fatalf("group [%d,%d) = %v, want %v", c.start, c.start+c.count, got, c.wantFirstV)
			}
		}
	}

	// A merged affine slot covers several commands and must only join a group
	// that contains the whole span.
	merged := &imageFrameResources{
		drawCalls: []imageDrawCall{{firstVertex: 0, vertexCount: 12, cmdStart: 0, cmdCount: 2}},
	}
	if res := s.sliceImageResources(merged, 1, 1); res != nil {
		t.Fatalf("group [1,2) took a slot covering commands [0,2): %+v", res.drawCalls)
	}
	if res := s.sliceImageResources(merged, 0, 2); res == nil || len(res.drawCalls) != 1 {
		t.Fatalf("group [0,2) lost the merged slot: %+v", res)
	}

}
