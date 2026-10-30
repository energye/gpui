//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/render/internal/gpu/res"
)

// Rebuild must not immediately destroy old texture views — commands queued
// earlier in the frame may still reference them. Since P2.4d the trio lives in
// res.Cache and the session hands the ref to its submission (deferRelease);
// these tests pin the textureSet release routing with fake resources.

func TestTextureSet_DestroyDefersTrioToSubmission(t *testing.T) {
	native := &surfaceTrio{msaaTex: &testTexture{}, msaaView: &testTextureView{}}
	ref := res.NewRegistry().DeviceOwner().Register(native)
	var deferredRef res.Ref
	var deferredRetire bool
	ts := &textureSet{
		msaaView:   native.msaaView,
		msaaTex:    native.msaaTex,
		width:      10,
		height:     10,
		surfaceRef: ref,
		deferRelease: func(r res.Ref, retire bool) {
			deferredRef, deferredRetire = r, retire
		},
	}

	ts.destroyTextures()
	if deferredRef.IsNil() {
		t.Fatal("destroyTextures must hand the cache ref to deferRelease")
	}
	if deferredRetire {
		t.Fatal("a full-size trio must release (pool), not retire")
	}
	if ts.msaaView != nil || ts.msaaTex != nil {
		t.Fatal("fields must be cleared after destroy")
	}
	if ts.width != 0 || ts.height != 0 {
		t.Fatal("dimensions must reset after destroy")
	}
	if !ts.surfaceRef.IsNil() {
		t.Fatal("surfaceRef must be cleared after hand-off")
	}
}

func TestTextureSet_DestroyRetiresDegradedTrio(t *testing.T) {
	native := &surfaceTrio{stencilTex: &testTexture{}, stencilView: &testTextureView{}, degraded: true}
	reg := res.NewRegistry()
	ref := reg.DeviceOwner().Register(native)
	var deferredRetire bool
	ts := &textureSet{
		stencilView:  native.stencilView,
		stencilTex:   native.stencilTex,
		surfaceRef:   ref,
		degradedTrio: true,
		deferRelease: func(r res.Ref, retire bool) {
			deferredRetire = retire
		},
	}
	ts.destroyTextures()
	if !deferredRetire {
		t.Fatal("a degraded 1x1 trio must retire, never enter the pool")
	}
}

func TestTextureSet_DestroyDirectTrioReleasesInline(t *testing.T) {
	tex := &testTexture{}
	view := &testTextureView{}
	ts := &textureSet{directTrio: &surfaceTrio{msaaTex: tex, msaaView: view}}
	ts.destroyTextures()
	if !tex.destroyed || !view.destroyed {
		t.Fatal("a directly owned trio must be destroyed inline")
	}
}

func TestTextureSet_DestroyLooseTexturesReleasesImmediately(t *testing.T) {
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
