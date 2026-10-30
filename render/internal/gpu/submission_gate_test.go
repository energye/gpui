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

func TestP6_SessionRetireBufferNilSafe(t *testing.T) {
	var s *GPURenderSession
	s.RetireBuffer(nil) // nil receiver + nil buffer: no panic
}

func TestP6_InvalidateForDeviceLoss_DropsBookkeeping(t *testing.T) {
	s := NewGPURenderSession(nil, nil, 4) // no GPU resources allocated
	t.Cleanup(func() { s.Destroy() })

	// Populate the registry with fake resources (never reaches native).
	b := &testBuffer{}
	view := &testTextureView{}
	tex := &testTexture{}
	s.RetireBuffer(b)
	s.RetireTexture(tex, view)
	if got := s.PendingTexRetireCount(); got != 2 {
		t.Fatalf("expected 2 retired entries before invalidation, got %d", got)
	}

	// Device loss: bookkeeping cleared, natives untouched (buffers/textures
	// above must NOT be released — the abandon flow owns native teardown).
	s.InvalidateForDeviceLoss()
	if got := s.PendingTexRetireCount(); got != 0 {
		t.Fatalf("retire bookkeeping must be dropped by InvalidateForDeviceLoss, got %d", got)
	}
	if b.destroyed || view.destroyed || tex.destroyed {
		t.Fatalf("device-loss invalidation must not release natives")
	}
	if s.resReg != nil && s.resReg.Count() != 0 {
		t.Fatalf("registry must be empty after device-loss invalidation")
	}
}
