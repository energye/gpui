package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/webgpu"
)

// TestShrinkDue_Matrix locks the pure hysteresis core (R6-2, no GPU):
// usage below a quarter of capacity sustains calm, anything else resets;
// at texShrinkCalmFrames the buffer is due and calm restarts.
func TestShrinkDue_Matrix(t *testing.T) {
	if calm, due := shrinkDue(100, 1000, 10, 0); calm != 0 || due {
		t.Fatalf("at/below floor: calm=%d due=%v want 0/false", calm, due)
	}
	if calm, due := shrinkDue(1000, 300, 10, 0); calm != 0 || due {
		t.Fatalf("heavy use: calm=%d due=%v want 0/false", calm, due)
	}
	if calm, due := shrinkDue(1000, 100, 10, 0); calm != 1 || due {
		t.Fatalf("tiny use: calm=%d due=%v want 1/false", calm, due)
	}
	if calm, due := shrinkDue(1000, 100, 10, texShrinkCalmFrames-1); calm != 0 || !due {
		t.Fatalf("threshold: calm=%d due=%v want 0/true", calm, due)
	}
	// Boundary: need == cap/4 is NOT below quarter → reset.
	if calm, due := shrinkDue(1000, 250, 10, 50); calm != 0 || due {
		t.Fatalf("quarter boundary: calm=%d due=%v want 0/false", calm, due)
	}
}

// TestSessionFrameBufferShrink drives the session hysteresis headlessly
// (R6-2, no GPU): sustained tiny use retires grow-only buffers; steady use
// never shrinks. Fake buffers only flow through RetireBuffer (deferred
// queue, no native calls).
func TestSessionFrameBufferShrink(t *testing.T) {
	s := &GPURenderSession{}
	const big = 1 << 20
	s.gpuTexVertexStaging = make([]byte, big)
	s.imageVertexStaging = make([]byte, big)
	s.gpuTexVertBuf = &webgpu.Buffer{}
	s.gpuTexVertBufCap = big
	s.gpuTexBaseVertBuf = &webgpu.Buffer{}
	s.gpuTexBaseVertBufCap = big
	s.gpuTexUniformSlab = &webgpu.Buffer{}
	s.gpuTexUniformSlabCap = big
	s.imageVertBuf = &webgpu.Buffer{}
	s.imageVertBufCap = big
	s.imageUniformSlab = &webgpu.Buffer{}
	s.imageUniformSlabCap = big
	// A live BG ring entry must be dropped with its slab (offsets die).
	s.gpuTexBGCaches = make([]gpuTexBGSlotCache, 1)
	s.gpuTexBGCaches[0].entries[0].bg = &webgpu.BindGroup{}

	for i := 0; i < texShrinkCalmFrames; i++ {
		s.maybeShrinkFrameBuffers()
	}
	if s.gpuTexVertexStaging != nil || s.imageVertexStaging != nil {
		t.Fatal("staging must be freed after sustained disuse")
	}
	for name, ptr := range map[string]**webgpu.Buffer{
		"texVert": &s.gpuTexVertBuf, "texBase": &s.gpuTexBaseVertBuf,
		"texSlab": &s.gpuTexUniformSlab, "imgVert": &s.imageVertBuf,
		"imgSlab": &s.imageUniformSlab,
	} {
		if *ptr != nil {
			t.Fatalf("%s must be retired after sustained disuse", name)
		}
	}
	if n := s.pendingBufRetire.PendingCount(); n != 5 {
		t.Fatalf("retire queue=%d want 5 (vert+base+slab+imgVert+imgSlab)", n)
	}
	if len(s.gpuTexBGCaches) != 1 || s.gpuTexBGCaches[0].entries[0].bg != nil {
		t.Fatal("slab shrink must drop BG rings")
	}

	// Steady use never shrinks: fresh peak at capacity stays put.
	s2 := &GPURenderSession{}
	s2.gpuTexVertBuf = &webgpu.Buffer{}
	s2.gpuTexVertBufCap = big
	s2.texLastVertNeed = big
	for i := 0; i < texShrinkCalmFrames; i++ {
		s2.maybeShrinkFrameBuffers()
	}
	if s2.gpuTexVertBuf == nil {
		t.Fatal("steady use must never shrink")
	}
	if n := s2.pendingBufRetire.PendingCount(); n != 0 {
		t.Fatalf("steady use retire queue=%d want 0", n)
	}
}

// TestSessionShrinkViaBeginFrame proves the BeginFrame wiring headlessly:
// the shrink check runs at the frame boundary and peak trackers reset for
// the new frame (no device needed — empty state touches no native calls).
func TestSessionShrinkViaBeginFrame(t *testing.T) {
	s := &GPURenderSession{}
	s.gpuTexVertexStaging = make([]byte, 1<<20)
	for i := 0; i < texShrinkCalmFrames; i++ {
		s.BeginFrame()
	}
	if s.gpuTexVertexStaging != nil {
		t.Fatal("BeginFrame must drive staging shrink after sustained disuse")
	}
}
