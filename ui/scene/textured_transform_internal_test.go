package scene

import (
	"image"
	"math"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/recording"
)

// Rotated/scaled pictures enter the texture cache untransformed
// (pushRecordCTM strips the transform) and phase 2 blits them through the
// live CTM. These tests pin that decision without a GPU: the pixel proof
// lives in ui/scene/internal/gpupixel, the headless proof here is the
// recorded bounds/offset plus a blit (no vector replay) once the fake slot
// reports a live view.

func transformFillRecording(cw, ch int, x, y, w, h float64) *recording.Recording {
	rec := recording.NewRecorder(cw, ch)
	rec.SetRGBA(1, 0, 0, 1)
	rec.FillRectangle(x, y, w, h)
	return rec.FinishRecording()
}

func transformPacket(rec *recording.Recording, key uint64, dirty bool,
	tx, ty, rot, sx, sy float64) *FramePacket {
	b := NewLayerBuilder()
	b.PushTransform(tx, ty, rot, sx, sy)
	pl := b.AddPicture(dirty)
	pl.SetCacheKey(key)
	pl.SetRecording(rec)
	b.Pop()
	return b.BuildPacket(1, 1, 100, 100)
}

func cachedEntry(t *testing.T, tex *PictureTextureCache, key uint64) (bounds image.Rectangle, off image.Point, w, h int) {
	t.Helper()
	tex.mu.Lock()
	defer tex.mu.Unlock()
	e := tex.entries[key]
	if e == nil || e.contentSlot < 0 {
		t.Fatalf("key %d must have a recorded cache entry", key)
	}
	s := &e.slots[e.contentSlot]
	return e.bounds, e.off, s.w, s.h
}

func TestCompositeFramePacketTextured_RotateCachesUntransformed(t *testing.T) {
	ResetLayerIDGen()
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()

	tex := NewPictureTextureCache(dc, 8)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	rec := transformFillRecording(40, 40, -20, -20, 40, 40)
	st := CompositeFramePacketTextured(transformPacket(rec, 401, true, 50, 50, math.Pi/4, 1, 1), dc, tex)
	if st.RasterLayerCount != 1 {
		t.Fatalf("RasterLayerCount=%d want 1 (rotated picture must record)", st.RasterLayerCount)
	}
	bounds, off, w, h := cachedEntry(t, tex, 401)
	if want := image.Rect(-20, -20, 20, 20); bounds != want {
		t.Fatalf("recorded bounds=%v want %v (untransformed content)", bounds, want)
	}
	if want := image.Pt(-20, -20); off != want {
		t.Fatalf("blit offset=%v want %v", off, want)
	}
	if w != 42 || h != 42 {
		t.Fatalf("texture=%dx%d want 42x42 (bounds-sized, not full window)", w, h)
	}

	// Fake a live view, then composite a clean packet: phase 2 must blit the
	// cached texture (ReplayedOps==0) instead of vector-replaying the picture.
	injectLiveView(t, tex, 401)
	st2 := CompositeFramePacketTextured(transformPacket(rec, 401, false, 50, 50, math.Pi/4, 1, 1), dc, tex)
	if st2.ReplayedOps != 0 {
		t.Fatalf("clean rotated frame replayed %d ops want 0 (blit path)", st2.ReplayedOps)
	}
}

func TestCompositeFramePacketTextured_ScaleCachesUntransformed(t *testing.T) {
	ResetLayerIDGen()
	dc := render.NewContext(100, 100)
	defer dc.Close()
	dc.BeginFrame()

	tex := NewPictureTextureCache(dc, 8)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	rec := transformFillRecording(10, 10, 0, 0, 10, 10)
	st := CompositeFramePacketTextured(transformPacket(rec, 402, true, 10, 10, 0, 2, 2), dc, tex)
	if st.RasterLayerCount != 1 {
		t.Fatalf("RasterLayerCount=%d want 1 (scaled picture must record)", st.RasterLayerCount)
	}
	bounds, off, w, h := cachedEntry(t, tex, 402)
	if want := image.Rect(0, 0, 10, 10); bounds != want {
		t.Fatalf("recorded bounds=%v want %v (untransformed content)", bounds, want)
	}
	if off != (image.Point{}) {
		t.Fatalf("blit offset=%v want (0,0)", off)
	}
	if w != 12 || h != 12 {
		t.Fatalf("texture=%dx%d want 12x12 (bounds-sized, not full window)", w, h)
	}

	injectLiveView(t, tex, 402)
	st2 := CompositeFramePacketTextured(transformPacket(rec, 402, false, 10, 10, 0, 2, 2), dc, tex)
	if st2.ReplayedOps != 0 {
		t.Fatalf("clean scaled frame replayed %d ops want 0 (blit path)", st2.ReplayedOps)
	}
}

func injectLiveView(t *testing.T, tex *PictureTextureCache, key uint64) {
	t.Helper()
	tex.mu.Lock()
	defer tex.mu.Unlock()
	e := tex.entries[key]
	if e == nil || e.contentSlot < 0 {
		t.Fatalf("key %d has no content slot to fake", key)
	}
	e.slots[e.contentSlot].view = fakeLiveView()
}
