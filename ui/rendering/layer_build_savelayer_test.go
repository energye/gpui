package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// twoSavelayerBoxes builds an OnPaint-only RenderBox that requests one
// SaveLayer per paint. Since C3 the paint runs UI-side while the packet is
// built, so the budget gates the request at build time directly.
func twoSavelayerBoxes() *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = 40, 20
	b.SetRepaintBoundary(true)
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		pc.SaveLayer(size.Width, size.Height, 0.5)
		rendering.FillRect(pc, 0, 0, size.Width, size.Height, 1, 0, 0, 1)
		pc.RestoreLayer()
	}
	return b
}

// TestBuildFramePacketWithSaveLayer_WiresBuildBudget: the per-frame
// SaveLayer budget must gate build-time OnPaint requests (retained path) —
// first box allowed, second refused by MaxOps=1, outcomes counted.
func TestBuildFramePacketWithSaveLayer_WiresBuildBudget(t *testing.T) {
	root := rendering.NewAbsoluteBox(120, 60)
	root.Place(twoSavelayerBoxes(), 0, 0)
	root.Place(twoSavelayerBoxes(), 50, 0)

	stats := &rendering.SaveLayerStats{}
	budget := &rendering.SaveLayerBudget{MaxOps: 1, MaxArea: 1e9}
	pkt := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 120, 60, stats, budget)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}

	if got := stats.Allow.Load(); got != 1 {
		t.Fatalf("Allow=%d want 1 (first request consumes MaxOps)", got)
	}
	if got := stats.Reject.Load(); got != 1 {
		t.Fatalf("Reject=%d want 1 (second request must be budget-refused)", got)
	}

	// Per-frame reset: a fresh frame allows exactly one again.
	budget.Reset()
	rendering.BuildFramePacketWithSaveLayer(root, 2, 1, 120, 60, stats, budget)
	if got := stats.Allow.Load(); got != 2 {
		t.Fatalf("Allow=%d want 2 after reset", got)
	}
}

// TestBuildFramePacket_LegacyUnwired: the plain BuildFramePacket keeps the
// legacy behavior — SaveLayer requests during the build are unlimited and no
// outcome counters are touched.
func TestBuildFramePacket_LegacyUnwired(t *testing.T) {
	root := rendering.NewAbsoluteBox(120, 60)
	root.Place(twoSavelayerBoxes(), 0, 0)
	root.Place(twoSavelayerBoxes(), 50, 0)
	// Layout first: an unmeasured box reports size 0, and a 0-sized recording
	// is invisible by definition, which would prove nothing about the build.
	root.Layout(rendering.Tight(120, 60))

	stats := &rendering.SaveLayerStats{}
	pkt := rendering.BuildFramePacket(root, 1, 1, 120, 60)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	if n := countRecordedPictures(pkt); n != 2 {
		t.Fatalf("recorded picture layers=%d want 2 (unwired build paints both)", n)
	}
	if got := stats.Allow.Load() + stats.Reject.Load(); got != 0 {
		t.Fatalf("legacy path must not touch stats, got %d", got)
	}
}

// countRecordedPictures counts keyed picture layers that carry visible
// recording content — proof both OnPaint boxes painted into their recordings.
func countRecordedPictures(pkt *scene.FramePacket) int {
	n := 0
	scene.Walk(pkt.Root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok && pl.Recording.HasVisiblePaint() {
			n++
		}
	})
	return n
}
