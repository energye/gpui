package overlay_test

import (
	"testing"

	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

func TestOverlay_InsertRemove_StackDepth(t *testing.T) {
	st := overlay.New()
	e1 := overlay.NewEntry(rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1), 0, 0, 40, 40)
	e2 := overlay.NewEntry(rendering.NewRenderColorBox(40, 40, 0, 1, 0, 1), 10, 10, 40, 40)
	st.Insert(e1)
	st.Insert(e2)
	if st.Len() != 2 {
		t.Fatalf("len=%d", st.Len())
	}
	if e1.ID() == 0 || e2.ID() == 0 || e1.ID() == e2.ID() {
		t.Fatalf("ids %d %d", e1.ID(), e2.ID())
	}
	removed := 0
	e2.OnRemove = func() { removed++ }
	if !st.Remove(e2) || st.Len() != 1 || removed != 1 {
		t.Fatalf("remove e2 len=%d removed=%d", st.Len(), removed)
	}
	st.Remove(e1)
	if st.Len() != 0 {
		t.Fatal("expected empty")
	}
}

func TestOverlay_BuildBand_NonEmpty(t *testing.T) {
	st := overlay.New()
	child := rendering.NewRenderColorBox(30, 30, 0.1, 0.2, 0.3, 1)
	child.SetRepaintBoundary(true)
	st.Insert(overlay.NewEntry(child, 5, 5, 30, 30))
	st.Layout(200, 200)
	layer, dirty := st.BuildOverlayBand()
	if layer == nil {
		t.Fatal("nil overlay layer")
	}
	n := scene.Walk(layer, nil)
	if n < 2 {
		t.Fatalf("walk count=%d want ≥2 (container+content)", n)
	}
	if len(dirty) < 1 {
		t.Fatalf("expected dirty ids, got %v", dirty)
	}
}

func TestOverlay_AttachToPacket_DoesNotClearMainDirty(t *testing.T) {
	main := rendering.NewRenderColorBox(100, 100, 0.5, 0.5, 0.5, 1)
	main.SetRepaintBoundary(true)
	// Build main packet with dirty boundary.
	pkt := rendering.BuildFramePacket(main, 1, 1, 100, 100)
	mainDirty := append([]uint64(nil), pkt.DirtyLayerIDs...)
	if len(mainDirty) < 1 {
		t.Fatal("expected main dirty")
	}

	st := overlay.New()
	ov := rendering.NewRenderColorBox(20, 20, 1, 0, 0, 1)
	ov.SetRepaintBoundary(true)
	st.Insert(overlay.NewEntry(ov, 0, 0, 20, 20))
	st.Layout(100, 100)
	st.AttachToPacket(pkt)

	// Main dirties still present; overlay dirties appended.
	if len(pkt.DirtyLayerIDs) < len(mainDirty) {
		t.Fatalf("main dirty lost: %v", pkt.DirtyLayerIDs)
	}
	for _, id := range mainDirty {
		found := false
		for _, d := range pkt.DirtyLayerIDs {
			if d == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("main dirty id %d missing after AttachToPacket", id)
		}
	}
	if pkt.Overlay == nil {
		t.Fatal("overlay nil")
	}
	// Overlay band should have content nodes.
	if scene.Walk(pkt.Overlay, nil) < 2 {
		t.Fatal("overlay band empty")
	}
}

func TestOverlay_HitTest_TopFirst(t *testing.T) {
	st := overlay.New()
	bottom := rendering.NewRenderColorBox(100, 100, 0, 0, 1, 1)
	top := rendering.NewRenderColorBox(50, 50, 1, 0, 0, 1)
	st.Insert(overlay.NewEntry(bottom, 0, 0, 100, 100))
	st.Insert(overlay.NewEntry(top, 0, 0, 50, 50))
	st.Layout(200, 200)

	hr := st.HitTest(rendering.Point{X: 10, Y: 10})
	if !hr.Consumed || hr.Child != top {
		t.Fatalf("expected top child, got entry=%v child=%T", hr.Entry, hr.Child)
	}
	// Point only on bottom
	hr = st.HitTest(rendering.Point{X: 80, Y: 80})
	if !hr.Consumed || hr.Child != bottom {
		t.Fatalf("expected bottom, got %T", hr.Child)
	}
}

func layoutMain(main *rendering.RenderColorBox, w, h float64) {
	main.Layout(rendering.Constraints{MinWidth: w, MaxWidth: w, MinHeight: h, MaxHeight: h})
}

func TestOverlay_HitBlocksMain(t *testing.T) {
	main := rendering.NewRenderColorBox(200, 200, 0.2, 0.2, 0.2, 1)
	layoutMain(main, 200, 200)
	st := overlay.New()
	ov := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	st.Insert(overlay.NewEntry(ov, 10, 10, 40, 40))
	st.Layout(200, 200)

	band, hit, ent := overlay.HitTestStack(main, st, rendering.Point{X: 20, Y: 20})
	if band != overlay.BandOverlay || hit != ov || ent == nil {
		t.Fatalf("band=%v hit=%T ent=%v", band, hit, ent)
	}
	// Outside overlay → main
	band, hit, ent = overlay.HitTestStack(main, st, rendering.Point{X: 100, Y: 100})
	if band != overlay.BandMain || hit != main || ent != nil {
		t.Fatalf("main miss: band=%v hit=%T", band, hit)
	}
}

func TestOverlay_RemoveRestoresMainHit(t *testing.T) {
	main := rendering.NewRenderColorBox(200, 200, 0.2, 0.2, 0.2, 1)
	layoutMain(main, 200, 200)
	st := overlay.New()
	ov := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	e := st.Insert(overlay.NewEntry(ov, 10, 10, 40, 40))
	st.Layout(200, 200)
	st.Remove(e)
	band, hit, _ := overlay.HitTestStack(main, st, rendering.Point{X: 20, Y: 20})
	if band != overlay.BandMain || hit != main {
		t.Fatalf("after remove band=%v hit=%T", band, hit)
	}
}

func TestOverlay_BarrierEatsPointer(t *testing.T) {
	main := rendering.NewRenderColorBox(200, 200, 0.2, 0.2, 0.2, 1)
	layoutMain(main, 200, 200)
	st := overlay.New()
	// Barrier with no child content hit — still consumes (after Layout).
	st.Insert(overlay.NewBarrierEntry(0, 0, 200, 200, nil))
	st.Layout(200, 200)
	band, hit, ent := overlay.HitTestStack(main, st, rendering.Point{X: 50, Y: 50})
	if band != overlay.BandOverlay || hit != nil || ent == nil || !ent.Barrier {
		t.Fatalf("barrier: band=%v hit=%T ent=%v", band, hit, ent)
	}
}

func TestOverlay_InsertRemove100_NoLeak(t *testing.T) {
	st := overlay.New()
	var removes int
	for i := 0; i < 100; i++ {
		e := overlay.NewEntry(rendering.NewRenderColorBox(10, 10, 1, 1, 1, 1), 0, 0, 10, 10)
		e.OnRemove = func() { removes++ }
		st.Insert(e)
		st.Remove(e)
	}
	if st.Len() != 0 || removes != 100 {
		t.Fatalf("len=%d removes=%d", st.Len(), removes)
	}
}

func TestOverlay_EnsureEmptyBand(t *testing.T) {
	st := overlay.New()
	layer, dirty := st.BuildOverlayBand()
	if layer == nil {
		t.Fatal("nil")
	}
	if len(dirty) != 0 {
		t.Fatalf("dirty=%v", dirty)
	}
}

// TestOverlay_AttachToPacket_BandSeparatedDirtyIDs (R8): the overlay portion
// of the frame's dirty set must be mirrored into OverlayDirtyLayerIDs while
// DirtyLayerIDs keeps the main ids plus the appended overlay ids — so
// "overlay opened dirtied only the overlay band" is assertable per band.
func TestOverlay_AttachToPacket_BandSeparatedDirtyIDs(t *testing.T) {
	main := rendering.NewRenderColorBox(100, 100, 0.5, 0.5, 0.5, 1)
	main.SetRepaintBoundary(true)
	pkt := rendering.BuildFramePacket(main, 1, 1, 100, 100)
	mainDirty := append([]uint64(nil), pkt.DirtyLayerIDs...)
	if len(mainDirty) < 1 {
		t.Fatal("expected main dirty")
	}

	st := overlay.New()
	ov := rendering.NewRenderColorBox(20, 20, 1, 0, 0, 1)
	ov.SetRepaintBoundary(true)
	st.Insert(overlay.NewEntry(ov, 0, 0, 20, 20))
	st.Layout(100, 100)
	st.AttachToPacket(pkt)

	if len(pkt.OverlayDirtyLayerIDs) == 0 {
		t.Fatal("OverlayDirtyLayerIDs empty — band mirror missing")
	}
	for _, id := range pkt.OverlayDirtyLayerIDs {
		found := false
		for _, d := range pkt.DirtyLayerIDs {
			if d == id {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("overlay id %d not merged into DirtyLayerIDs", id)
		}
	}
	if len(pkt.DirtyLayerIDs) != len(mainDirty)+len(pkt.OverlayDirtyLayerIDs) {
		t.Fatalf("dirty split mismatch: main=%d overlay=%d total=%d",
			len(mainDirty), len(pkt.OverlayDirtyLayerIDs), len(pkt.DirtyLayerIDs))
	}
}

// TestOverlay_AttachToPacket_EmptyClearsMirror: a closed overlay (empty state)
// must clear OverlayDirtyLayerIDs so steady frames report no overlay dirties.
func TestOverlay_AttachToPacket_EmptyClearsMirror(t *testing.T) {
	st := overlay.New()
	pkt := &scene.FramePacket{FrameID: 1}
	pkt.OverlayDirtyLayerIDs = []uint64{7}
	st.AttachToPacket(pkt)
	if len(pkt.OverlayDirtyLayerIDs) != 0 {
		t.Fatalf("mirror not cleared: %v", pkt.OverlayDirtyLayerIDs)
	}
}

// TestOverlay_HitGatedUntilLayout: a just-inserted entry must NOT answer
// hit tests until the first Layout pass (Flutter _RenderDeferredLayoutBox:
// overlay children participate only after deferred layout ran). C8 bug:
// same-tick probes against a fresh entry silently missed.
func TestOverlay_HitGatedUntilLayout(t *testing.T) {
	st := overlay.New()
	ov := rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1)
	st.Insert(overlay.NewEntry(ov, 10, 10, 40, 40))

	// Same tick, before Layout: gated → miss falls through to main.
	hr := st.HitTest(rendering.Point{X: 20, Y: 20})
	if hr.Consumed || hr.Entry != nil || hr.Child != nil {
		t.Fatalf("pre-layout hit consumed=%v entry=%v child=%T want none", hr.Consumed, hr.Entry, hr.Child)
	}

	// Next tick's Layout pass opens the gate.
	st.Layout(200, 200)
	hr = st.HitTest(rendering.Point{X: 20, Y: 20})
	if !hr.Consumed || hr.Child != ov {
		t.Fatalf("post-layout hit consumed=%v child=%T want ov", hr.Consumed, hr.Child)
	}
}

// TestOverlay_BarrierAlsoGatedUntilLayout: barrier entries have no child but
// their bounds are equally invalid before Layout — they must not eat pointer
// events in the insert tick.
func TestOverlay_BarrierAlsoGatedUntilLayout(t *testing.T) {
	st := overlay.New()
	st.Insert(overlay.NewBarrierEntry(0, 0, 200, 200, nil))

	if hr := st.HitTest(rendering.Point{X: 100, Y: 100}); hr.Consumed {
		t.Fatal("pre-layout barrier consumed the pointer")
	}
	st.Layout(200, 200)
	if hr := st.HitTest(rendering.Point{X: 100, Y: 100}); !hr.Consumed || hr.Entry == nil || !hr.Entry.Barrier {
		t.Fatalf("post-layout barrier: consumed=%v entry=%v", hr.Consumed, hr.Entry)
	}
}

// TestOverlay_ReinsertRemovedEntryStaysDead: Remove is final — the id is
// cleared and Insert refuses a removed entry. Reuse requires a fresh Entry
// (a resurrected entry would ghost-hit at stale coordinates).
func TestOverlay_ReinsertRemovedEntryStaysDead(t *testing.T) {
	st := overlay.New()
	e := overlay.NewEntry(rendering.NewRenderColorBox(40, 40, 1, 0, 0, 1), 10, 10, 40, 40)
	st.Insert(e)
	st.Layout(200, 200)
	if !st.Remove(e) {
		t.Fatal("remove failed")
	}
	if e.ID() != 0 {
		t.Fatalf("removed entry keeps id %d want 0", e.ID())
	}
	st.Insert(e) // refused
	if st.Len() != 0 {
		t.Fatalf("removed entry resurrected, len=%d", st.Len())
	}
	if hr := st.HitTest(rendering.Point{X: 20, Y: 20}); hr.Consumed {
		t.Fatal("dead entry answered a hit test")
	}
}

// TestOverlay_FixedSizeAlsoGatedAndHit: fixed-size entries go through the
// same gate even though W/H are known at Insert — the Child offset is only
// valid after Layout, which is what the gate protects.
func TestOverlay_FixedSizeAlsoGatedAndHit(t *testing.T) {
	st := overlay.New()
	ov := rendering.NewRenderColorBox(30, 30, 0, 1, 0, 1)
	e := overlay.NewEntry(ov, 50, 50, 30, 30)
	st.Insert(e)

	if hr := st.HitTest(rendering.Point{X: 60, Y: 60}); hr.Consumed {
		t.Fatal("fixed-size entry hit before Layout")
	}
	st.Layout(200, 200)
	if e.W != 30 || e.H != 30 {
		t.Fatalf("layout clobbered fixed size: %vx%v", e.W, e.H)
	}
	if hr := st.HitTest(rendering.Point{X: 60, Y: 60}); !hr.Consumed || hr.Child != ov {
		t.Fatalf("fixed-size post-layout hit: consumed=%v child=%T", hr.Consumed, hr.Child)
	}
}
