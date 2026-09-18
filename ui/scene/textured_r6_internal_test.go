package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// TestPhase1_BoundslessExtraSkipsTexture locks the R6-1 fix with a stubbed
// slot factory (no GPU needed): a RasterExtra with no bounds anywhere
// (zero-size node: empty picture, empty ExtraBounds) must NOT allocate a
// full-window texture (~4MB double-buffered each — dozens of 0×0 OnPaint
// boxes held >500MB in the button window). Phase 2 still runs the callback
// on the vector fallback, so there is no hole.
//
// Red without the fix: the old recordWith path allocates a slot (allocs=1)
// and keeps the entry (Has=true).
func TestPhase1_BoundslessExtraSkipsTexture(t *testing.T) {
	ResetLayerIDGen()
	b := NewLayerBuilder()
	b.PushOffset(10, 10)
	pl := b.AddPicture(true)
	pl.SetCacheKey(305)
	// Empty picture + empty ExtraBounds on purpose: a zero-size node.
	ran := 0
	pl.RasterExtra = func(dc *render.Context) {
		ran++
	}
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	CompositeFramePacketTextured(pkt, dc, tex)
	if ran == 0 {
		t.Fatal("bounds-less RasterExtra must still run on the vector fallback")
	}
	if *allocs != 0 {
		t.Fatalf("bounds-less RasterExtra allocated %d textures, want 0", *allocs)
	}
	if tex.Has(305) {
		t.Fatal("bounds-less RasterExtra must not hold a full-window texture")
	}
}

// TestPhase1_BoundedExtraStillTextures guards the other side: an OnPaint
// layer WITH paint bounds keeps its cheap bounds-sized texture.
func TestPhase1_BoundedExtraStillTextures(t *testing.T) {
	ResetLayerIDGen()
	b := NewLayerBuilder()
	pl := b.AddPicture(true)
	pl.SetCacheKey(306)
	ran := 0
	pl.RasterExtra = func(dc *render.Context) {
		ran++
	}
	pl.ExtraBounds = image.Rect(0, 0, 20, 10)
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	CompositeFramePacketTextured(pkt, dc, tex)
	if ran == 0 {
		t.Fatal("bounded RasterExtra must run (record or fallback)")
	}
	// Headless (stub slots, CPU dc) the flush cannot complete, so only
	// the path decision is observable: a bounded extra must ATTEMPT its
	// bounds-sized texture (allocs>0), unlike the skipped bounds-less
	// case above (allocs==0).
	if *allocs == 0 {
		t.Fatal("bounded RasterExtra must still take its bounds-sized texture path")
	}
}
