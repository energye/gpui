//go:build !nogpu

package gpu

import (
	"testing"

	"github.com/energye/gpui/gpu/hal"
)

// TestTextureSet_PruneStencilPoolExcept drops every pooled stencil except the
// requested size (drag-storm VRAM leak: each window size left one full-window
// depth texture pinned, 80M baseline growing to 170M). Evicted entries go
// through releaseOrRetire — immediate Destroy without retireFn, deferred
// queue with retireFn (no WaitIdle stall either way).
func TestTextureSet_PruneStencilPoolExcept(t *testing.T) {
	mk := func() (*testTexture, *testTextureView) {
		return &testTexture{}, &testTextureView{}
	}
	txA, vwA := mk()
	txB, vwB := mk()
	txCur, vwCur := mk()
	ts := &textureSet{
		stencilPool: map[stencilPoolKey]*pooledStencil{
			{w: 1868, h: 1016}: {tex: txA, view: vwA},
			{w: 1408, h: 509}:  {tex: txB, view: vwB},
			{w: 1200, h: 700}:  {tex: txCur, view: vwCur},
		},
	}
	ts.pruneStencilPoolExcept(1200, 700)
	if len(ts.stencilPool) != 1 {
		t.Fatalf("expected 1 pooled entry left, got %d", len(ts.stencilPool))
	}
	if _, ok := ts.stencilPool[stencilPoolKey{w: 1200, h: 700}]; !ok {
		t.Fatal("current-size entry must survive pruning")
	}
	for _, tc := range []struct {
		name string
		tx   *testTexture
		vw   *testTextureView
	}{
		{"stale-A", txA, vwA},
		{"stale-B", txB, vwB},
	} {
		if !tc.tx.destroyed || !tc.vw.destroyed {
			t.Fatalf("%s: evicted tex/view must be destroyed (tex=%v view=%v)",
				tc.name, tc.tx.destroyed, tc.vw.destroyed)
		}
	}
	if txCur.destroyed || vwCur.destroyed {
		t.Fatal("current-size entry must not be destroyed")
	}
}

func TestTextureSet_PruneStencilPoolExceptNilPool(t *testing.T) {
	var ts textureSet
	ts.pruneStencilPoolExcept(1200, 700) // must not panic
}

func TestTextureSet_PruneStencilPoolExceptDeferred(t *testing.T) {
	txOld, vwOld := &testTexture{}, &testTextureView{}
	var retiredTex []hal.Texture
	var retiredView []hal.TextureView
	var ts textureSet
	ts.retireFn = func(tex hal.Texture, view hal.TextureView) {
		retiredTex = append(retiredTex, tex)
		retiredView = append(retiredView, view)
	}
	ts.stencilPool = map[stencilPoolKey]*pooledStencil{
		{w: 1868, h: 1016}: {tex: txOld, view: vwOld},
	}
	ts.pruneStencilPoolExcept(1200, 700)
	if len(ts.stencilPool) != 0 {
		t.Fatalf("expected empty pool, got %d entries", len(ts.stencilPool))
	}
	if txOld.destroyed || vwOld.destroyed {
		t.Fatal("deferred entries must not be destroyed inline; they ride retireFn")
	}
	if len(retiredTex) != 1 || len(retiredView) != 1 {
		t.Fatalf("expected 1 tex + 1 view through retireFn, got %d + %d",
			len(retiredTex), len(retiredView))
	}
}
