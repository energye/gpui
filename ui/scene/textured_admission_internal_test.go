package scene

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

// Admission mirrors Flutter RasterCache access_threshold (default 3):
// changed content restarts the count and replays vector with damage;
// only unchanged sightings past the threshold may (re)establish the
// texture. These tests pin the counter, refusal, eviction, damage and
// sweep semantics without a GPU.

func TestAdmission_ThresholdCountsStableSightings(t *testing.T) {
	dc := render.NewContext(100, 80)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	for i := 1; i <= 3; i++ {
		if tex.markStableSight(11) {
			t.Fatalf("sighting %d must not admit (threshold 3)", i)
		}
	}
	if !tex.markStableSight(11) {
		t.Fatal("4th consecutive unchanged sighting must admit (>3)")
	}
	tex.markChanged(11)
	if tex.markStableSight(11) {
		t.Fatal("changed content restarts the count: next sighting must not admit")
	}
	if n := tex.markChanged(11); n != 1 {
		t.Fatalf("first changed frame churn=%d want 1 (record immediately)", n)
	}
	if n := tex.markChanged(11); n != 2 {
		t.Fatalf("second consecutive changed frame churn=%d want 2 (refuse)", n)
	}
	// Churn also restarts stability: four clean sightings to re-admit.
	for i := 1; i <= 3; i++ {
		if tex.markStableSight(11) {
			t.Fatalf("post-churn clean sighting %d must not admit yet", i)
		}
	}
	if !tex.markStableSight(11) {
		t.Fatal("4th post-churn clean sighting must admit")
	}
}

func TestAdmission_RefuseEvictsAndDamages(t *testing.T) {
	dc := render.NewContext(100, 80)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	b := image.Rect(0, 0, 40, 30)
	pic := RecordPicture(func(r *PictureRecorder) {
		r.FillRect(0, 0, 40, 30, 1, 0, 0, 1)
	})
	tex.BeginFrame()
	if _, ok := tex.recordLocalWith(21, &pic, b, nil); !ok {
		t.Fatal("establishing record must succeed")
	}
	// Headless has no device: fake a live view so Has reports the entry
	// (same as TestDropTransparent_KeepsShellForDamage).
	tex.mu.Lock()
	e := tex.entries[21]
	if e == nil || e.contentSlot < 0 {
		tex.mu.Unlock()
		t.Fatal("entry must exist after record")
	}
	e.slots[e.contentSlot].view = fakeLiveView()
	tex.mu.Unlock()
	if !tex.Has(21) {
		t.Fatal("entry must exist after record")
	}
	// A changed frame refuses: stale entry goes (a later clean blit would
	// paint it over correct content) and current bounds are noted so the
	// vector replay reaches the screen under LoadOpLoad present.
	tex.markChanged(21)
	tex.refuseChanged(21, b)
	if tex.Has(21) {
		t.Fatal("refused layer must have no texture to blit")
	}
	if got := tex.RefusedBounds(21); got != b {
		t.Fatalf("RefusedBounds=%v want %v", got, b)
	}
	// Refusal is frame-scoped like oversized: next frame starts clean.
	tex.BeginFrame()
	if got := tex.RefusedBounds(21); !got.Empty() {
		t.Fatalf("refused damage must expire after the frame, got %v", got)
	}
}

func TestAdmission_SweepDropsDeadKeys(t *testing.T) {
	dc := render.NewContext(100, 80)
	defer dc.Close()
	tex := NewPictureTextureCache(dc, 0)
	tex.markChanged(31)
	tex.markChanged(32)
	// No live info (unit tests compositing without SetLiveKeys) keeps
	// everything: under-counting only delays admission, never corrupts.
	tex.BeginFrame()
	tex.mu.Lock()
	_, ok31 := tex.stable[31]
	tex.mu.Unlock()
	if !ok31 {
		t.Fatal("empty liveKeys must not sweep (no live info yet)")
	}
	tex.SetLiveKeys([]uint64{31})
	tex.BeginFrame()
	tex.mu.Lock()
	_, ok31 = tex.stable[31]
	_, ok32 := tex.stable[32]
	tex.mu.Unlock()
	if !ok31 {
		t.Fatal("live key must survive the sweep")
	}
	if ok32 {
		t.Fatal("dead key must be swept")
	}
}

// A changed picture layer records on the first dirty frame (one-shot
// changes keep today's wave) but sustained churn replays vector with damage
// covering its bounds instead of re-recording: fullscreen animation then
// costs one direct pass per frame, not an offscreen record plus a blit.
func TestAdmission_CompositeRefusesSustainedChurn(t *testing.T) {
	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, _, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	build := func() *FramePacket {
		ResetLayerIDGen()
		b := NewLayerBuilder()
		pl := b.AddPicture(true) // needsRaster = changed every frame
		pl.SetCacheKey(101)
		pl.Record(func(r *PictureRecorder) {
			r.FillRect(0, 0, 40, 30, 1, 0, 0, 1)
		})
		return b.BuildPacket(1, 1, 100, 80)
	}

	// First dirty frame admits (records; headless records succeed like the
	// shrink tests — only Has needs a live view). Damage covers the new
	// content, nothing is refused.
	st1 := CompositeFramePacketTextured(build(), dc, tex)
	if got := tex.RefusedBounds(101); !got.Empty() {
		t.Fatalf("first dirty frame must not refuse, got %v", got)
	}
	want := image.Rect(0, 0, 40, 30)
	covered := false
	for _, d := range st1.DamageRects {
		if d.Min.X <= want.Min.X && d.Min.Y <= want.Min.Y &&
			d.Max.X >= want.Max.X && d.Max.Y >= want.Max.Y {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("first-frame DamageRects=%v must cover new content %v", st1.DamageRects, want)
	}
	// Second consecutive dirty frame refuses: no texture, noted bounds,
	// damage covers them, vector fallback paints the frame.
	st := CompositeFramePacketTextured(build(), dc, tex)
	// Refusal evicts the stale entry: a later clean frame must never blit
	// frame-1 content over the correct screen.
	tex.mu.Lock()
	_, stillThere := tex.entries[101]
	tex.mu.Unlock()
	if stillThere {
		t.Fatal("refusal must evict the stale entry (anti stale-blit)")
	}
	if got := tex.RefusedBounds(101); got.Empty() {
		t.Fatal("refused layer must note damage bounds")
	}
	want = image.Rect(0, 0, 40, 30)
	covered = false
	for _, d := range st.DamageRects {
		if d.Min.X <= want.Min.X && d.Min.Y <= want.Min.Y &&
			d.Max.X >= want.Max.X && d.Max.Y >= want.Max.Y {
			covered = true
		}
	}
	if !covered {
		t.Fatalf("DamageRects=%v must cover refused bounds %v", st.DamageRects, want)
	}
	if st.ReplayedOps < 1 {
		t.Fatalf("ReplayedOps=%d want ≥1 (vector fallback paints the frame)", st.ReplayedOps)
	}
}

// A stable layer earns its texture on the 4th unchanged sighting and
// blits after: static content keeps the cheap path, delayed by 3 frames
// exactly like Flutter's first three direct draws. (Headless records
// always fail, so admission is observed via record attempts, not entries.)
func TestAdmission_CompositeAdmitsStableLayer(t *testing.T) {
	dc := render.NewContext(100, 80)
	defer dc.Close()
	dc.BeginFrame()
	tex := NewPictureTextureCache(dc, 0)
	alloc, allocs, _ := fakeSlotFactory(t)
	tex.slotAlloc = alloc

	build := func(clean bool) *FramePacket {
		ResetLayerIDGen()
		b := NewLayerBuilder()
		pl := b.AddPicture(!clean) // needsRaster=false when clean
		pl.SetCacheKey(102)
		pl.Record(func(r *PictureRecorder) {
			r.FillRect(0, 0, 40, 30, 1, 0, 0, 1)
		})
		if clean {
			// Record always marks NeedsRaster (new display list =
			// changed content); a clean re-sighting keeps the same
			// picture with no raster demand.
			pl.NeedsRaster = false
		}
		pkt := b.BuildPacket(1, 1, 100, 80)
		if clean {
			pkt.DirtyLayerIDs = nil
		}
		return pkt
	}

	// One changed frame admits (record attempt); three clean sightings must
	// not attempt; the 4th clean sighting attempts the establishing record.
	attempts := func() int { return *allocs }
	CompositeFramePacketTextured(build(false), dc, tex)
	firstAttempts := attempts()
	if firstAttempts < 1 {
		t.Fatal("first dirty frame must attempt the record (one-shot wave)")
	}
	for i := 0; i < 3; i++ {
		CompositeFramePacketTextured(build(true), dc, tex)
		if attempts() != firstAttempts {
			t.Fatalf("clean sighting %d must not attempt a record yet (threshold 3)", i+1)
		}
	}
	CompositeFramePacketTextured(build(true), dc, tex)
	if attempts() != firstAttempts+1 {
		t.Fatal("4th unchanged sighting must attempt the establishing record")
	}
}
