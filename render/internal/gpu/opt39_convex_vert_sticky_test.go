//go:build !nogpu

package gpu

import (
	"os"
	"testing"

	"github.com/energye/gpui/render"
)

// TestOpt39_ConvexVertSticky_SkipsRepeatWrite: classic (non-meshCompact) convex
// rebuilds with no deferred uses stay on slot 0 and skip Queue.WriteBuffer.
func TestOpt39_ConvexVertSticky_SkipsRepeatWrite(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, testSampleCount(t, device))
	t.Cleanup(func() { s.Destroy() })
	if err := s.ensurePipelines(); err != nil {
		t.Fatal(err)
	}

	// Classic AA convex (not TriangleList+SkipAA mesh) so sticky path is active.
	cmd := ConvexDrawCommand{
		Points: []render.Point{{X: 0, Y: 0}, {X: 20, Y: 0}, {X: 20, Y: 20}, {X: 0, Y: 20}},
		Color:  [4]float32{1, 0, 0, 1},
	}

	res1, err := s.buildConvexResources([]ConvexDrawCommand{cmd}, 128, 128)
	if err != nil || res1 == nil {
		t.Fatalf("first: res=%v err=%v", res1, err)
	}
	if s.convexVertSlot != 0 {
		t.Fatalf("expected slot 0 with no deferred uses, got %d", s.convexVertSlot)
	}
	if s.convexVertSlotLen[0] == 0 {
		t.Fatal("vert sticky not armed")
	}
	w1 := s.lastSubmitStats.WriteBuffers

	res2, err := s.buildConvexResources([]ConvexDrawCommand{cmd}, 128, 128)
	if err != nil || res2 == nil {
		t.Fatalf("second: %v", err)
	}
	w2 := s.lastSubmitStats.WriteBuffers
	if w2 != w1 {
		t.Fatalf("identical classic verts should skip WriteBuffer, %d→%d", w1, w2)
	}
	if res2.vertBuf != res1.vertBuf {
		t.Fatal("expected same vert buffer on sticky rebuild")
	}

	cmd2 := ConvexDrawCommand{
		Points: []render.Point{{X: 2, Y: 2}, {X: 22, Y: 2}, {X: 22, Y: 22}, {X: 2, Y: 22}},
		Color:  [4]float32{1, 0, 0, 1},
	}
	if _, err := s.buildConvexResources([]ConvexDrawCommand{cmd2}, 128, 128); err != nil {
		t.Fatal(err)
	}
	w3 := s.lastSubmitStats.WriteBuffers
	if w3 <= w2 {
		t.Fatalf("geometry change must WriteBuffer verts, %d→%d", w2, w3)
	}
}

func TestOpt39_ConvexVertSticky_Slot0WhenNoDeferred(t *testing.T) {
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	device, queue, cleanup := createNativeTestDevice(t)
	t.Cleanup(cleanup)
	s := NewGPURenderSession(device, queue, 1)
	t.Cleanup(func() { s.Destroy() })

	for i := 0; i < 6; i++ {
		if slot := s.allocConvexVertSlot(); slot != 0 {
			t.Fatalf("iter %d: slot=%d want 0", i, slot)
		}
	}
	s.deferredConvexUses = 2
	s.convexVertSlot = 0
	if slot := s.allocConvexVertSlot(); slot == 0 {
		t.Fatalf("with deferredConvexUses, expected ring advance, got 0")
	}
}
