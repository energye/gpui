//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

// TestOpt24_StickyIndex_ReusesRingSlotOnSameTopology verifies multi-slot
// index fingerprint reuse: same topology skips WriteBuffer, different
// topology can occupy another ring slot and both remain reusable.
func TestOpt24_StickyIndex_ReusesRingSlotOnSameTopology(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatalf("pipelines: %v", err)
	}

	positions := []render.Point{
		{X: 0, Y: 0}, {X: 10, Y: 0}, {X: 0, Y: 10}, {X: 10, Y: 10},
	}
	colors := []render.RGBA{
		{R: 1, G: 0, B: 0, A: 1},
		{R: 0, G: 1, B: 0, A: 1},
		{R: 0, G: 0, B: 1, A: 1},
		{R: 1, G: 1, B: 0, A: 1},
	}
	indicesA := []uint16{0, 1, 2, 1, 3, 2}
	pk := make([]byte, len(positions)*convexVertexStride)
	_ = packMeshVertsCoverage1(pk, positions, colors, true)

	cmdA := ConvexDrawCommand{TriangleList: true, SkipAA: true, PackedVerts: pk, Indices: indicesA}
	res1, err := s.buildConvexResources([]ConvexDrawCommand{cmdA}, 100, 100)
	if err != nil || res1 == nil || res1.indexBuf == nil {
		t.Fatalf("first build: res=%v err=%v", res1, err)
	}
	bufA := res1.indexBuf

	// Animate verts, same topology → reuse bufA.
	positions2 := []render.Point{{X: 1, Y: 1}, {X: 11, Y: 1}, {X: 1, Y: 11}, {X: 11, Y: 11}}
	pk2 := make([]byte, len(positions2)*convexVertexStride)
	_ = packMeshVertsCoverage1(pk2, positions2, colors, true)
	cmdA2 := ConvexDrawCommand{TriangleList: true, SkipAA: true, PackedVerts: pk2, Indices: indicesA}
	res2, err := s.buildConvexResources([]ConvexDrawCommand{cmdA2}, 100, 100)
	if err != nil || res2 == nil {
		t.Fatalf("second build: %v", err)
	}
	if res2.indexBuf != bufA {
		t.Fatal("expected ring-slot reuse for identical topology")
	}

	// Different topology → may allocate another slot.
	indicesB := []uint16{0, 2, 1, 1, 2, 3}
	cmdB := ConvexDrawCommand{TriangleList: true, SkipAA: true, PackedVerts: pk2, Indices: indicesB}
	res3, err := s.buildConvexResources([]ConvexDrawCommand{cmdB}, 100, 100)
	if err != nil || res3 == nil || res3.indexBuf == nil {
		t.Fatalf("third build: %v", err)
	}
	bufB := res3.indexBuf

	// Return to topology A — must reuse bufA without depending on "last write".
	res4, err := s.buildConvexResources([]ConvexDrawCommand{cmdA2}, 100, 100)
	if err != nil || res4 == nil {
		t.Fatalf("fourth build: %v", err)
	}
	if res4.indexBuf != bufA {
		t.Fatal("expected multi-slot reuse of topology A after B")
	}
	// And B still reusable.
	res5, err := s.buildConvexResources([]ConvexDrawCommand{cmdB}, 100, 100)
	if err != nil || res5 == nil {
		t.Fatalf("fifth build: %v", err)
	}
	if res5.indexBuf != bufB {
		t.Fatal("expected multi-slot reuse of topology B")
	}
}

func TestOpt24_LayoutTemplate_HitBeforeShape(t *testing.T) {
	face := r75Face(t, 14)
	eng := NewGlyphMaskEngine()
	eng.ResetLayoutTemplateCacheStats()
	mat := render.Identity()
	col := render.RGBA{R: 1, G: 1, B: 1, A: 1}
	const s = "HUD static line opt24"
	b1, err := eng.LayoutText(face, s, 10, 20, col, mat, 1)
	if err != nil || len(b1.Quads) == 0 {
		t.Fatalf("first layout: quads=%d err=%v", len(b1.Quads), err)
	}
	b2, err := eng.LayoutText(face, s, 10, 20, col, mat, 1)
	if err != nil || len(b2.Quads) != len(b1.Quads) {
		t.Fatalf("second layout: quads=%d err=%v", len(b2.Quads), err)
	}
	hits, misses, _ := eng.LayoutTemplateCacheStats()
	if hits < 1 {
		t.Fatalf("expected template hit on second layout, hits=%d misses=%d", hits, misses)
	}
	col2 := render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}
	b3, err := eng.LayoutText(face, s, 10, 20, col2, mat, 1)
	if err != nil || len(b3.Quads) != len(b1.Quads) {
		t.Fatalf("color layout: %v", err)
	}
	hits2, _, _ := eng.LayoutTemplateCacheStats()
	if hits2 < 2 {
		t.Fatalf("expected second hit for color-only, hits=%d", hits2)
	}
	if b3.Color == b1.Color {
		t.Fatalf("color should differ: %v vs %v", b3.Color, b1.Color)
	}
}

func TestOpt24_ImageUniformSkip_SameViewportOpacity(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatalf("pipelines: %v", err)
	}

	px := []byte{255, 0, 0, 255}
	cmd := ImageDrawCommand{
		PixelData: px, GenerationID: 42, ImgWidth: 1, ImgHeight: 1, ImgStride: 4,
		DstX: 0, DstY: 0, DstW: 10, DstH: 10,
		TLX: 0, TLY: 0, TRX: 10, TRY: 0, BRX: 10, BRY: 10, BLX: 0, BLY: 10,
		U0: 0, V0: 0, U1: 1, V1: 1, Opacity: 1,
		ViewportWidth: 100, ViewportHeight: 80,
	}
	res1, err := s.buildImageResources([]ImageDrawCommand{cmd}, 100, 80, nil)
	if err != nil || res1 == nil {
		t.Fatalf("first image build: %v", err)
	}
	if len(s.imageUniformLast) < 1 || !s.imageUniformLast[0].valid {
		t.Fatal("expected image uniform last armed")
	}
	last := s.imageUniformLast[0]

	cmd.DstX, cmd.DstY = 5, 5
	res2, err := s.buildImageResources([]ImageDrawCommand{cmd}, 100, 80, nil)
	if err != nil || res2 == nil {
		t.Fatalf("second image build: %v", err)
	}
	if !s.imageUniformLast[0].valid || s.imageUniformLast[0] != last {
		t.Fatalf("uniform last changed on dest-only move: %+v vs %+v", s.imageUniformLast[0], last)
	}

	cmd.Opacity = 0.5
	_, err = s.buildImageResources([]ImageDrawCommand{cmd}, 100, 80, nil)
	if err != nil {
		t.Fatalf("opacity build: %v", err)
	}
	if s.imageUniformLast[0].opacity != 0.5 || !s.imageUniformLast[0].valid {
		t.Fatalf("expected opacity update, got %+v", s.imageUniformLast[0])
	}
}
