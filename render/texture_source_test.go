//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "testing"

type recordingTextureSource struct {
	calls int
}

func (s *recordingTextureSource) PaintTexture(dc *Context, opts TexturePaintOptions) (bool, bool) {
	s.calls++
	return true, true
}

// TestTextureSourceRegistryLifecycle locks id → source resolution and the
// register/unregister balance (资源谁建谁放): a retained layer ships an id, so
// a leaked registration would keep resolving to a producer whose native
// texture is already gone.
func TestTextureSourceRegistryLifecycle(t *testing.T) {
	base := LiveTextureSources()
	a := &recordingTextureSource{}
	b := &recordingTextureSource{}

	if id := RegisterTextureSource(nil); id != 0 {
		t.Fatalf("nil source id=%d want 0 (fail closed, no registry entry)", id)
	}
	if LiveTextureSources() != base {
		t.Fatalf("nil registration grew the registry to %d", LiveTextureSources())
	}

	idA := RegisterTextureSource(a)
	defer UnregisterTextureSource(idA)
	idB := RegisterTextureSource(b)
	defer UnregisterTextureSource(idB)
	if idA == 0 || idB == 0 {
		t.Fatalf("ids=%d,%d want nonzero", idA, idB)
	}
	if idB <= idA {
		t.Fatalf("ids not monotonic: %d then %d", idA, idB)
	}
	if LiveTextureSources() != base+2 {
		t.Fatalf("live=%d want %d", LiveTextureSources(), base+2)
	}
	got, ok := GetTextureSource(idA)
	if !ok || got != TextureSource(a) {
		t.Fatalf("Get(%d)=%v,%v want the registered source", idA, got, ok)
	}

	UnregisterTextureSource(idA)
	if _, ok := GetTextureSource(idA); ok {
		t.Fatal("unregistered id still resolves")
	}
	UnregisterTextureSource(idA) // idempotent
	UnregisterTextureSource(0)   // no-op
	if LiveTextureSources() != base+1 {
		t.Fatalf("live=%d want %d after one unregister", LiveTextureSources(), base+1)
	}
	if _, ok := GetTextureSource(0); ok {
		t.Fatal("id 0 resolved: 0 means no source")
	}
}

// TestVideoBridgeIsTextureSource locks the route's role as a texture source:
// registration comes from construction and goes away at Close, a source with no
// frame fails closed, and paint accounting is filed per view so a
// picture-in-picture window cannot report one view's outcome as the other's.
func TestVideoBridgeIsTextureSource(t *testing.T) {
	// CPU context: the noop device has no GPU session to draw a slot into, so
	// the bridge converges to its counted fallback path and the outcome of a
	// paint is deterministic.
	t.Setenv("GOGPU_RENDER_MODE", "cpu")
	c := loadVideoUploadCases(t).Draw
	dev := openNoopVideoDevice(t)
	base := LiveTextureSources()
	br := NewVideoBridge(dev)
	if LiveTextureSources() != base+1 {
		t.Fatalf("live sources=%d want %d: NewVideoBridge must register itself", LiveTextureSources(), base+1)
	}
	id := br.ID()
	if id == 0 {
		t.Fatal("bridge ID=0 after construction")
	}
	if got, ok := GetTextureSource(id); !ok || got != TextureSource(br) {
		t.Fatalf("Get(%d)=%v,%v want the bridge", id, got, ok)
	}

	dc := NewContext(64, 64)
	defer dc.Close()

	// No frame yet: nothing painted, nothing covered.
	if direct, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 32, Height: 18}); ok || direct {
		t.Fatalf("PaintTexture before any upload direct=%v ok=%v want false/false", direct, ok)
	}
	if st := br.PaintStats(); st.FailedPaints != 1 || st.Paints != 1 || st.ViewPaints[0] != 1 {
		t.Fatalf("stats after miss=%+v want paints=1 failed=1 view0=1", st)
	}

	pix := make([]byte, c.W*c.H*4)
	for i := range pix {
		pix[i] = 0x7f
	}
	if !br.UploadFrame(c.W, c.H, pix) {
		t.Fatalf("UploadFrame(%dx%d) = false, want true", c.W, c.H)
	}
	// Two views of one route: main + PiP. The noop device has no GPU session,
	// so both draws take the counted fallback and still report ok.
	if direct, ok := br.PaintTexture(dc, TexturePaintOptions{X: c.X, Y: c.Y, Width: c.DstW, Height: c.DstH}); !ok || direct {
		t.Fatalf("main view direct=%v ok=%v want false/true", direct, ok)
	}
	if _, ok := br.PaintTexture(dc, TexturePaintOptions{X: 20, Y: 20, Width: 12, Height: 8, DrawIndex: 1}); !ok {
		t.Fatal("second view ok=false, want true")
	}
	st := br.PaintStats()
	if st.Paints != 3 || st.FailedPaints != 1 || st.DirectPaints != 0 || st.FallbackPaints != 2 {
		t.Fatalf("stats=%+v want paints=3 failed=1 direct=0 fallback=2", st)
	}
	if st.ViewPaints[0] != 2 || st.ViewPaints[1] != 1 {
		t.Fatalf("ViewPaints=%v want [2,1,…]", st.ViewPaints)
	}
	if !st.ViewOK[0] || !st.ViewOK[1] {
		t.Fatalf("ViewOK=%v want both views drawn", st.ViewOK)
	}
	if !st.ViewLastOK[0] || !st.ViewLastOK[1] {
		t.Fatalf("ViewLastOK=%v want both views' most recent paint successful", st.ViewLastOK)
	}
	if st.ViewDirect[0] || st.ViewDirect[1] {
		t.Fatalf("ViewDirect=%v want false on the fallback path", st.ViewDirect)
	}
	// A view index past the bound folds into the last slot instead of
	// corrupting memory.
	if _, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 8, Height: 8, DrawIndex: videoMaxViews + 5}); !ok {
		t.Fatal("out-of-range view paint failed")
	}
	if got := br.PaintStats().ViewPaints[videoMaxViews-1]; got != 1 {
		t.Fatalf("clamped slot=%v want 1", got)
	}

	br.Close()
	if _, ok := GetTextureSource(id); ok {
		t.Fatal("Close left the route registered")
	}
	if LiveTextureSources() != base {
		t.Fatalf("live sources=%d want %d after Close", LiveTextureSources(), base)
	}
	// A closed route keeps the ever-drew proof but its most recent paint must
	// report the miss: gates that assert the current picture use the Last pair.
	if _, ok := br.PaintTexture(dc, TexturePaintOptions{Width: 8, Height: 8}); ok {
		t.Fatal("closed bridge PaintTexture ok=true, want false")
	}
	st = br.PaintStats()
	if !st.ViewOK[0] || st.ViewLastOK[0] {
		t.Fatalf("after close ViewOK=%v ViewLastOK=%v want ever=true last=false", st.ViewOK, st.ViewLastOK)
	}
	// The id a layer may still carry resolves to nothing rather than to a
	// released route: fail closed.
	var nilBridge *VideoBridge
	if _, ok := nilBridge.PaintTexture(nil, TexturePaintOptions{}); ok {
		t.Fatal("nil bridge PaintTexture ok=true, want false")
	}
}
