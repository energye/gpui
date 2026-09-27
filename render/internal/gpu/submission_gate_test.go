//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// testBuffer is a hal.Buffer stub that records Destroy calls.
type testBuffer struct {
	hal.Buffer
	destroyed bool
}

func (b *testBuffer) Destroy() { b.destroyed = true }

// testTexture is a hal.Texture stub that records Destroy calls (片7d:
// replaces concrete texture fakes so render names no concrete type).
type testTexture struct {
	hal.Texture
	destroyed bool
}

func (t *testTexture) Destroy() { t.destroyed = true }

// testTextureView is a hal.TextureView stub that records Destroy calls.
type testTextureView struct {
	hal.TextureView
	destroyed bool
}

func (v *testTextureView) Destroy() { v.destroyed = true }

// P6: submission-tracked deferred release — buffers retired during grow-only
// rebuilds stay alive until the next safe point; device-loss invalidation
// drops bookkeeping without touching native.

func TestP6_PendingBufRetire_AddDrain(t *testing.T) {
	var q pendingBufRetire
	b1 := &testBuffer{}
	b2 := &testBuffer{}
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
	if !b1.destroyed || !b2.destroyed {
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
	b := &testBuffer{}
	s.RetireBuffer(b)
	s.pendingTexRetire.Add(&testTextureView{}, &testTexture{})
	if s.pendingTexRetire.PendingCount() == 0 || s.pendingBufRetire.PendingCount() == 0 {
		t.Fatalf("expected non-empty retire queues before invalidation")
	}

	// Device loss: bookkeeping cleared, natives untouched (buffers/textures
	// above must NOT be released — the abandon flow owns native teardown).
	s.InvalidateForDeviceLoss()
	if s.pendingTexRetire.PendingCount() != 0 || s.pendingBufRetire.PendingCount() != 0 {
		t.Fatalf("retire queues must be drained by InvalidateForDeviceLoss")
	}
	if b.destroyed {
		t.Fatalf("device-loss invalidation must not release native buffers")
	}
	if s.resReg != nil && s.resReg.Count() != 0 {
		t.Fatalf("registry must be empty after device-loss invalidation")
	}
}
