package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/webgpu"
)

// P6: submission-tracked deferred release — buffers retired during grow-only
// rebuilds stay alive until the next safe point; device-loss invalidation
// drops bookkeeping without touching native.

func TestP6_PendingBufRetire_AddDrain(t *testing.T) {
	var q pendingBufRetire
	b1 := &webgpu.Buffer{}
	b2 := &webgpu.Buffer{}
	q.Add(b1)
	q.Add(b2)
	q.Add(nil) // no-op
	if got := q.PendingCount(); got != 2 {
		t.Fatalf("expected 2 pending buffers, got %d", got)
	}
	q.Drain()
	if got := q.PendingCount(); got != 0 {
		t.Fatalf("expected 0 after drain, got %d", got)
	}
	if !b1.Released() || !b2.Released() {
		t.Fatalf("drain must release every queued buffer")
	}
	q.Drain() // idempotent
}

func TestP6_SessionRetireBufferNilSafe(t *testing.T) {
	var s *GPURenderSession
	s.RetireBuffer(nil) // nil receiver + nil buffer: no panic
}

func TestP6_InvalidateForDeviceLoss_DropsBookkeeping(t *testing.T) {
	s := NewGPURenderSession(nil, nil, 4) // no GPU resources allocated
	t.Cleanup(func() { s.Destroy() })

	// Populate bookkeeping with fake resources (never reaches native).
	b := &webgpu.Buffer{}
	s.RetireBuffer(b)
	s.pendingTexRetire.Add(&webgpu.TextureView{}, &webgpu.Texture{})
	if s.pendingTexRetire.PendingCount() == 0 || s.pendingBufRetire.PendingCount() == 0 {
		t.Fatalf("expected non-empty retire queues before invalidation")
	}

	// Device loss: bookkeeping cleared, natives untouched (buffers/textures
	// above must NOT be released — the abandon flow owns native teardown).
	s.InvalidateForDeviceLoss()
	if s.pendingTexRetire.PendingCount() != 0 || s.pendingBufRetire.PendingCount() != 0 {
		t.Fatalf("retire queues must be drained by InvalidateForDeviceLoss")
	}
	if b.Released() {
		t.Fatalf("device-loss invalidation must not release native buffers")
	}
	if s.resReg != nil && s.resReg.Count() != 0 {
		t.Fatalf("registry must be empty after device-loss invalidation")
	}
}
