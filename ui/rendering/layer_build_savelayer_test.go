package rendering_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// twoSavelayerBoxes builds an OnPaint-only RenderBox that requests one
// SaveLayer per paint (raster-time in the retained path).
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

// runRasterExtras executes every RasterExtra callback in layer order — what
// the retained composite pass does on the raster thread during texture record.
func runRasterExtras(t *testing.T, dc *render.Context, root scene.Layer) int {
	t.Helper()
	n := 0
	scene.Walk(root, func(l scene.Layer) {
		if pl, ok := l.(*scene.PictureLayer); ok && pl.RasterExtra != nil {
			pl.RasterExtra(dc)
			n++
		}
	})
	return n
}

// TestBuildFramePacketWithSaveLayer_WiresRasterBudget: the per-frame
// SaveLayer budget must gate raster-time OnPaint requests (retained path) —
// first box allowed, second refused by MaxOps=1, outcomes counted.
func TestBuildFramePacketWithSaveLayer_WiresRasterBudget(t *testing.T) {
	root := rendering.NewAbsoluteBox(120, 60)
	root.Place(twoSavelayerBoxes(), 0, 0)
	root.Place(twoSavelayerBoxes(), 50, 0)

	stats := &rendering.SaveLayerStats{}
	budget := &rendering.SaveLayerBudget{MaxOps: 1, MaxArea: 1e9}
	pkt := rendering.BuildFramePacketWithSaveLayer(root, 1, 1, 120, 60, stats, budget)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}

	dc := render.NewContext(120, 60)
	defer dc.Close()

	if n := runRasterExtras(t, dc, pkt.Root); n != 2 {
		t.Fatalf("raster extras=%d want 2", n)
	}
	if got := stats.Allow.Load(); got != 1 {
		t.Fatalf("Allow=%d want 1 (first request consumes MaxOps)", got)
	}
	if got := stats.Reject.Load(); got != 1 {
		t.Fatalf("Reject=%d want 1 (second request must be budget-refused)", got)
	}

	// Per-frame reset: a fresh frame allows exactly one again.
	budget.Reset()
	if n := runRasterExtras(t, dc, pkt.Root); n != 2 {
		t.Fatalf("raster extras=%d want 2", n)
	}
	if got := stats.Allow.Load(); got != 2 {
		t.Fatalf("Allow=%d want 2 after reset", got)
	}
}

// TestBuildFramePacket_LegacyUnwired: the plain BuildFramePacket keeps the
// legacy behavior — raster-time SaveLayer requests are unlimited and no
// outcome counters are touched.
func TestBuildFramePacket_LegacyUnwired(t *testing.T) {
	root := rendering.NewAbsoluteBox(120, 60)
	root.Place(twoSavelayerBoxes(), 0, 0)
	root.Place(twoSavelayerBoxes(), 50, 0)

	stats := &rendering.SaveLayerStats{}
	pkt := rendering.BuildFramePacket(root, 1, 1, 120, 60)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	dc := render.NewContext(120, 60)
	defer dc.Close()
	if n := runRasterExtras(t, dc, pkt.Root); n != 2 {
		t.Fatalf("raster extras=%d want 2", n)
	}
	if got := stats.Allow.Load() + stats.Reject.Load(); got != 0 {
		t.Fatalf("legacy path must not touch stats, got %d", got)
	}
}
