//go:build !nogpu

package gpu

import (
	"testing"
	"unsafe"

	"github.com/energye/gpui/render"
)

func TestOpt25_PackedMeshVertsContiguous_MultiCmd(t *testing.T) {
	// Simulate Queue grow-only packing into one backing array.
	backing := make([]byte, 0, 12*convexVertexStride)
	mk := func(n int, x0 float32) (pk []byte, positions []render.Point, colors []render.RGBA) {
		positions = make([]render.Point, n)
		colors = make([]render.RGBA, n)
		for i := 0; i < n; i++ {
			positions[i] = render.Point{X: float64(x0) + float64(i), Y: float64(i)}
			colors[i] = render.RGBA{R: 1, A: 1}
		}
		base := len(backing)
		need := n * convexVertexStride
		backing = backing[:base+need]
		pk = backing[base : base+need : base+need]
		_ = packMeshVertsCoverage1(pk, positions, colors, true)
		return pk, positions, colors
	}
	pk0, _, _ := mk(3, 0)
	pk1, _, _ := mk(6, 10)
	pk2, _, _ := mk(3, 20)
	cmds := []ConvexDrawCommand{
		{TriangleList: true, SkipAA: true, PackedVerts: pk0},
		{TriangleList: true, SkipAA: true, PackedVerts: pk1},
		{TriangleList: true, SkipAA: true, PackedVerts: pk2},
	}
	data, ok := packedMeshVertsContiguous(cmds)
	if !ok {
		t.Fatal("expected contiguous multi-cmd packed verts")
	}
	want := 12 * convexVertexStride
	if len(data) != want {
		t.Fatalf("len=%d want %d", len(data), want)
	}
	// Must be zero-copy into backing.
	if unsafe.Pointer(&data[0]) != unsafe.Pointer(&backing[0]) {
		t.Fatal("expected zero-copy view into backing")
	}
	// Non-contiguous: separate allocs.
	sep := make([]byte, 3*convexVertexStride)
	copy(sep, pk0)
	cmds2 := []ConvexDrawCommand{
		{TriangleList: true, SkipAA: true, PackedVerts: sep},
		{TriangleList: true, SkipAA: true, PackedVerts: pk1},
	}
	if _, ok := packedMeshVertsContiguous(cmds2); ok {
		t.Fatal("separate allocs must not report contiguous")
	}
}

func TestOpt25_PackedMeshIndicesContiguous_MultiCmd(t *testing.T) {
	backing := make([]uint16, 0, 24)
	mkIdx := func(n int, baseV uint16) []uint16 {
		b := len(backing)
		backing = backing[:b+n]
		s := backing[b : b+n : b+n]
		for i := 0; i < n; i++ {
			s[i] = baseV + uint16(i%3)
		}
		return s
	}
	// Dummy packed verts so convexCmdIndexCount accepts cmds.
	pv := make([]byte, 3*convexVertexStride)
	i0 := mkIdx(6, 0)
	i1 := mkIdx(12, 0)
	cmds := []ConvexDrawCommand{
		{TriangleList: true, SkipAA: true, PackedVerts: pv, Indices: i0},
		{TriangleList: true, SkipAA: true, PackedVerts: pv, Indices: i1},
	}
	data, count, ok := packedMeshIndicesContiguous(cmds)
	if !ok || count != 18 {
		t.Fatalf("ok=%v count=%d", ok, count)
	}
	if len(data) != 36 {
		t.Fatalf("bytes=%d", len(data))
	}
	if unsafe.Pointer(&data[0]) != unsafe.Pointer(&backing[0]) {
		t.Fatal("expected zero-copy index view")
	}

	// Gap: non-adjacent slice
	other := []uint16{0, 1, 2, 0, 2, 3}
	cmds2 := []ConvexDrawCommand{
		{TriangleList: true, SkipAA: true, PackedVerts: pv, Indices: i0},
		{TriangleList: true, SkipAA: true, PackedVerts: pv, Indices: other},
	}
	if _, _, ok := packedMeshIndicesContiguous(cmds2); ok {
		t.Fatal("non-adjacent indices must fail")
	}
}

func TestOpt25_BuildConvexResources_MultiCmdZeroCopy(t *testing.T) {
	if testing.Short() {
		// still need GPU
	}
	// Use WGPU if available.
	// Environment checked inside createNativeTestDevice via Skip.
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatalf("pipelines: %v", err)
	}

	// Two indexed meshes packed contiguously like QueueColoredMeshIndexed.
	// Pre-size capacity so grow does not reallocate (real Queue path is
	// contiguous only when convexMeshPacked already has capacity — warm frames).
	packed := make([]byte, 0, 64*convexVertexStride)
	indices := make([]uint16, 0, 64)
	mk := func(nV int, nI int) ConvexDrawCommand {
		positions := make([]render.Point, nV)
		colors := make([]render.RGBA, nV)
		for i := 0; i < nV; i++ {
			positions[i] = render.Point{X: float64(i), Y: float64(i % 3)}
			colors[i] = render.RGBA{R: 0.5, G: 0.5, B: 1, A: 1}
		}
		pb := len(packed)
		need := nV * convexVertexStride
		if cap(packed) < pb+need {
			np := make([]byte, pb, (pb+need)*2)
			copy(np, packed)
			packed = np
		}
		packed = packed[:pb+need]
		pk := packed[pb : pb+need : pb+need]
		_ = packMeshVertsCoverage1(pk, positions, colors, true)
		ib := len(indices)
		if cap(indices) < ib+nI {
			ni := make([]uint16, ib, ib+nI+8)
			copy(ni, indices)
			indices = ni
		}
		indices = indices[:ib+nI]
		idx := indices[ib : ib+nI : ib+nI]
		for i := 0; i < nI; i++ {
			idx[i] = uint16(i % nV)
		}
		return ConvexDrawCommand{
			TriangleList: true, SkipAA: true,
			PackedVerts: pk, Indices: idx,
		}
	}
	cmds := []ConvexDrawCommand{mk(4, 6), mk(5, 12)}
	if _, ok := packedMeshVertsContiguous(cmds); !ok {
		t.Fatal("test setup verts not contiguous")
	}
	if _, n, ok := packedMeshIndicesContiguous(cmds); !ok || n != 18 {
		t.Fatalf("test setup indices contiguous n=%d ok=%v", n, ok)
	}
	res, err := s.buildConvexResources(cmds, 128, 128)
	if err != nil || res == nil {
		t.Fatalf("build: res=%v err=%v", res, err)
	}
	if res.vertCount != 9 {
		t.Fatalf("vertCount=%d want 9", res.vertCount)
	}
	if res.indexCount != 18 {
		t.Fatalf("indexCount=%d want 18", res.indexCount)
	}
	if res.indexBuf == nil || res.vertBuf == nil {
		t.Fatal("missing GPU buffers")
	}
	// Ranges: 2 indexed draws with baseVertex offsets.
	if len(res.ranges) != 2 {
		t.Fatalf("ranges=%d want 2", len(res.ranges))
	}
	if !res.ranges[0].indexed || res.ranges[0].indexCount != 6 {
		t.Fatalf("range0=%+v", res.ranges[0])
	}
	if !res.ranges[1].indexed || res.ranges[1].firstVertex != 4 || res.ranges[1].indexCount != 12 {
		t.Fatalf("range1=%+v", res.ranges[1])
	}
}
