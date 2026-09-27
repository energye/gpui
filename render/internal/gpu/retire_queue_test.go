package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// P4: rebuild must not immediately destroy old texture views — commands
// queued earlier in the frame may still reference them. These tests pin the
// deferred-release queue and the textureSet retire-fn behavior with fake
// resources (Destroy-recording stubs; no concrete type named).

func TestPendingTexRetire_AddDrain(t *testing.T) {
	var q pendingTexRetire
	v1 := &testTextureView{}
	v2 := &testTextureView{}
	tx1 := &testTexture{}
	q.Add(v1, nil)
	q.Add(v2, tx1)
	if got := q.PendingCount(); got != 3 {
		t.Fatalf("expected 3 pending after adds, got %d", got)
	}
	q.Drain()
	if got := q.PendingCount(); got != 0 {
		t.Fatalf("expected 0 pending after drain, got %d", got)
	}
	if !v1.destroyed || !v2.destroyed || !tx1.destroyed {
		t.Fatalf("drain must release every queued resource")
	}
	// Idempotent + nil-safe.
	q.Drain()
	q.Add(nil, nil)
	if got := q.PendingCount(); got != 0 {
		t.Fatalf("nil adds must be no-ops, got %d", got)
	}
}

func TestTextureSet_DestroyWithRetireFnDefersRelease(t *testing.T) {
	ts := &textureSet{}
	view := &testTextureView{}
	tex := &testTexture{}
	retired := 0
	ts.retireFn = func(tx hal.Texture, tv hal.TextureView) {
		retired++
		if view.destroyed {
			t.Fatalf("view must NOT be released while deferred")
		}
		if tex.destroyed {
			t.Fatalf("texture must NOT be released while deferred")
		}
	}
	ts.msaaView = view
	ts.msaaTex = tex
	ts.width, ts.height = 10, 10

	ts.destroyTextures()
	if retired != 2 {
		t.Fatalf("expected retireFn called for both resources, got %d", retired)
	}
	if view.destroyed || tex.destroyed {
		t.Fatalf("resources must remain alive when retireFn installed (deferred)")
	}
	if ts.msaaView != nil || ts.msaaTex != nil {
		t.Fatalf("fields must be cleared after destroy")
	}
	if ts.width != 0 || ts.height != 0 {
		t.Fatalf("dimensions must reset after destroy")
	}
}

func TestTextureSet_DestroyWithoutRetireFnReleasesImmediately(t *testing.T) {
	// Default (stencil/SDF textureSets have no retireFn): destroy releases now.
	ts := &textureSet{}
	v := &testTextureView{}
	tx := &testTexture{}
	ts.resolveView = v
	ts.resolveTex = tx
	ts.destroyTextures()
	if !v.destroyed || !tx.destroyed {
		t.Fatalf("default destroy must release immediately")
	}
}

func TestSessionRetireTextureNilSafe(t *testing.T) {
	// Nil session → immediate release fallback, no panic.
	v := &testTextureView{}
	var s *GPURenderSession
	s.RetireTexture(nil, v)
	if !v.destroyed {
		t.Fatalf("nil-session fallback must release immediately")
	}
}
