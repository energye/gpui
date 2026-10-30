//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// rebuild must not immediately destroy old texture views — commands
// queued earlier in the frame may still reference them. These tests pin the
// textureSet retire-fn behavior with fake resources (Destroy-recording stubs;
// no concrete type named). Deferred releases themselves are owned by the
// res registry (see render_session retireNative).

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
