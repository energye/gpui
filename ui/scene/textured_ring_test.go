package scene

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render"
)

// fakeSlotFactory returns a slot factory that counts allocations and records
// deferred releases, so ring alternation / reallocation is observable without
// a GPU (the default factory would return nil views on no-GPU systems).
func fakeSlotFactory(t *testing.T) (func(c *PictureTextureCache, w, h int) (*pictureTextureSlot, bool), *int, *[]string) {
	t.Helper()
	allocs := 0
	released := []string{}
	f := func(c *PictureTextureCache, w, h int) (*pictureTextureSlot, bool) {
		allocs++
		return &pictureTextureSlot{
			view:    render.TextureView{},
			release: func() { released = append(released, fmt.Sprintf("%dx%d", w, h)) },
			w:       w,
			h:       h,
		}, true
	}
	return f, &allocs, &released
}

// TestTextureRing_PersistentSlots verifies the skia/flutter-aligned
// double-buffer: same-size re-records never allocate (slots alternate
// per frame), and the write slot alternates so a texture that was sampled
// by the previous frame is not re-recorded in the current one.
func TestTextureRing_PersistentSlots(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	// Frame 1: first record allocates slot 0.
	tex.BeginFrame()
	e := tex.allocEntry(1, 10, 10)
	if e == nil {
		t.Fatalf("frame1 allocEntry failed")
	}
	if *allocs != 1 {
		t.Fatalf("allocs=%d want 1", *allocs)
	}
	if e.contentSlot != 0 || e.slots[0].lastWriteFrame != 1 || e.slots[0].w != 10 {
		t.Fatalf("frame1: contentSlot=%d slot0.frame=%d want 0/1", e.contentSlot, e.slots[0].lastWriteFrame)
	}

	// Frame 2: re-record alternates to slot 1 — its first allocation, then
	// the ring reaches steady state (both slots exist → pure reuse).
	tex.BeginFrame()
	e = tex.allocEntry(1, 10, 10)
	if e == nil {
		t.Fatalf("frame2 allocEntry failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2 (slot1 first allocation)", *allocs)
	}
	if e.contentSlot != 1 || e.slots[1].lastWriteFrame != 2 {
		t.Fatalf("frame2: contentSlot=%d slot1.frame=%d want 1/2", e.contentSlot, e.slots[1].lastWriteFrame)
	}

	// Frame 3: alternates back to slot 0, zero new allocations from here on.
	tex.BeginFrame()
	e = tex.allocEntry(1, 10, 10)
	if e == nil {
		t.Fatalf("frame3 allocEntry failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2 (persistent slots, steady state)", *allocs)
	}
	if e.contentSlot != 0 || e.slots[0].lastWriteFrame != 3 {
		t.Fatalf("frame3: contentSlot=%d slot0.frame=%d want 0/3", e.contentSlot, e.slots[0].lastWriteFrame)
	}
	// Frame 4: alternates to slot 1 again — still zero allocations.
	tex.BeginFrame()
	e = tex.allocEntry(1, 10, 10)
	if e == nil {
		t.Fatalf("frame4 allocEntry failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2 (never allocates after steady state)", *allocs)
	}
	if e.contentSlot != 1 || e.slots[1].lastWriteFrame != 4 {
		t.Fatalf("frame4: contentSlot=%d slot1.frame=%d want 1/4", e.contentSlot, e.slots[1].lastWriteFrame)
	}
}

// TestTextureRing_DoubleRecordSameFrame covers two writes in one frame: the
// second write goes to the other (untouched) slot — never the one the first
// write's blit may sample — and a third write falls back to slot 0 reuse
// (all writes are COLOR_TARGET, so no usage conflict).
func TestTextureRing_DoubleRecordSameFrame(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	tex.BeginFrame()
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("record1 failed")
	}
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("record2 failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2 (slot1 fresh for the second write)", *allocs)
	}
	e := tex.entries[1]
	if e.contentSlot != 1 || e.slots[0].lastWriteFrame != 1 || e.slots[1].lastWriteFrame != 1 {
		t.Fatalf("double record: contentSlot=%d slot0.frame=%d slot1.frame=%d want 1/1/1",
			e.contentSlot, e.slots[0].lastWriteFrame, e.slots[1].lastWriteFrame)
	}
	// Third write in the same frame: both slots written → slot 0 reuse
	// (same usage, no allocation, no conflict).
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("record3 failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2 (third write reuses slot 0)", *allocs)
	}
	if e.contentSlot != 0 {
		t.Fatalf("triple record: contentSlot=%d want 0", e.contentSlot)
	}
}

// TestTextureRing_ReallocOnSizeChange verifies a size change reallocates only
// the target slot (deferred release of the old view) while keeping the other
// slot alive, and that the deferred release fires after a few frames.
func TestTextureRing_ReallocOnSizeChange(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, released := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	tex.BeginFrame()
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("frame1 allocEntry failed")
	}
	// Size change (e.g. after a resize): the other (empty) slot is built.
	tex.BeginFrame()
	if e := tex.allocEntry(1, 30, 30); e == nil {
		t.Fatalf("frame2 allocEntry failed")
	}
	if *allocs != 2 {
		t.Fatalf("allocs=%d want 2", *allocs)
	}
	e := tex.entries[1]
	if e.contentSlot != 1 || e.slots[1].w != 30 || e.slots[1].h != 30 {
		t.Fatalf("after frame2: contentSlot=%d slot1=%dx%d want 1/30x30", e.contentSlot, e.slots[1].w, e.slots[1].h)
	}
	// Frame 3 alternates back to slot 0, whose old 10x10 view must be
	// reallocated and deferred-released.
	tex.BeginFrame()
	if e := tex.allocEntry(1, 30, 30); e == nil {
		t.Fatalf("frame3 allocEntry failed")
	}
	if *allocs != 3 {
		t.Fatalf("allocs=%d want 3 (slot0 reallocated)", *allocs)
	}
	// The old slot 0 view must be held in the deferred queue, not released yet.
	if len(tex.deferred) == 0 {
		t.Fatalf("deferred empty: old slot should be deferred-released")
	}
	if len(*released) != 0 {
		t.Fatalf("released=%v want none yet (deferred)", *released)
	}
	// Two frames later the deferred release executes.
	tex.BeginFrame()
	tex.BeginFrame()
	if len(*released) == 0 {
		t.Fatalf("deferred release never fired: %v", *released)
	}
	// Entry still alive with the new-size content slot.
	if e.contentSlot != 0 || e.slots[0].w != 30 || e.slots[0].h != 30 {
		t.Fatalf("after resize: contentSlot=%d slot0=%dx%d want 0/30x30", e.contentSlot, e.slots[0].w, e.slots[0].h)
	}
}

// TestTextureRing_EvictionReleasesBothSlots verifies eviction deferred-releases
// both persistent slot views (EndFrame unused-entry path).
func TestTextureRing_EvictionReleasesBothSlots(t *testing.T) {
	dc := render.NewContext(20, 20)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, released := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	tex.BeginFrame()
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("allocEntry failed")
	}
	tex.BeginFrame()
	if e := tex.allocEntry(1, 10, 10); e == nil {
		t.Fatalf("allocEntry2 failed")
	}
	tex.usedNow = map[uint64]struct{}{} // entry unused this frame
	tex.EndFrame()
	if tex.Len() != 0 {
		t.Fatalf("Len=%d want 0 after eviction", tex.Len())
	}
	tex.BeginFrame() // drain (deferredFrames=2)
	tex.BeginFrame()
	if len(*released) != *allocs {
		t.Fatalf("released=%d want %d (both slots)", len(*released), *allocs)
	}
}
