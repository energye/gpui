package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// TestPhase1_EmptyRecordingSkipsTexture locks the R6-1 fix with a stubbed
// slot factory (no GPU needed): a zero-size OnPaint box records nothing
// visible, so it must NOT allocate a full-window texture (~4MB
// double-buffered each — dozens of 0×0 OnPaint boxes held >500MB in the
// button window). The no-hole contract is structural now: the recording is
// the content, and an empty recording has nothing to draw on either path.
//
// Red without the fix: the old recordWith path allocates a slot (allocs=1)
// and keeps the entry (Has=true).
func TestPhase1_EmptyRecordingSkipsTexture(t *testing.T) {
	ResetLayerIDGen()
	b := NewLayerBuilder()
	b.PushOffset(10, 10)
	pl := b.AddPicture(true)
	pl.SetCacheKey(305)
	// Zero-size node: the OnPaint baked an empty recording.
	pl.SetRecording(recording.NewRecorder(0, 0).FinishRecording())
	b.Pop() // offset
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	st := CompositeFramePacketTextured(pkt, dc, tex)
	if *allocs != 0 {
		t.Fatalf("empty recording allocated %d textures, want 0", *allocs)
	}
	if tex.Has(305) {
		t.Fatal("empty recording must not hold a full-window texture")
	}
	if st.ReplayedOps != 0 {
		t.Fatalf("empty recording replayed %d ops, want 0", st.ReplayedOps)
	}
}

// TestPhase1_InvisibleRecordingReplaysVector keeps the other half of the old
// R6-1 assertion ("the content still lands — no hole"): a recording WITH
// geometry but no visible paint (zero-alpha fill) is refused a texture
// (record/recordLocal's HasVisiblePaint guard, keeping dropTransparentLocked's
// damage), yet phase 2 still vector-replays it every frame it misses.
func TestPhase1_InvisibleRecordingReplaysVector(t *testing.T) {
	ResetLayerIDGen()
	b := NewLayerBuilder()
	pl := b.AddPicture(true)
	pl.SetCacheKey(307)
	rec := recording.NewRecorder(20, 10)
	rec.SetRGBA(0, 0, 0, 0)
	rec.DrawRectangle(0, 0, 20, 10)
	_ = rec.Fill()
	pl.SetRecording(rec.FinishRecording())
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	st := CompositeFramePacketTextured(pkt, dc, tex)
	if *allocs != 0 {
		t.Fatalf("invisible recording allocated %d textures, want 0", *allocs)
	}
	if tex.Has(307) {
		t.Fatal("invisible recording must not hold a texture")
	}
	if st.ReplayedOps == 0 {
		t.Fatal("invisible recording must still vector-replay (no hole)")
	}
}

// TestPhase1_BoundedRecordingTextures guards the other side: an OnPaint
// layer WITH visible paint bounds keeps its cheap bounds-sized texture.
func TestPhase1_BoundedRecordingTextures(t *testing.T) {
	ResetLayerIDGen()
	b := NewLayerBuilder()
	pl := b.AddPicture(true)
	pl.SetCacheKey(306)
	pl.SetRecording(fillRecording(image.Rect(0, 0, 20, 10)))
	pkt := b.BuildPacket(1, 1, 100, 80)

	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	CompositeFramePacketTextured(pkt, dc, tex)
	// Headless (stub slots, CPU dc) the flush cannot complete, so only
	// the path decision is observable: a bounded visible recording must
	// ATTEMPT its bounds-sized texture (allocs>0), unlike the refused
	// invisible case above (allocs==0).
	if *allocs == 0 {
		t.Fatal("bounded visible recording must still take its bounds-sized texture path")
	}
}
