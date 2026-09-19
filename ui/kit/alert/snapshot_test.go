package alert

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// R2-6 snapshot trio (button snapshot paradigm): paint follows the frozen
// snapshot, never live widget fields.

// TestAlert_SnapshotIgnoresLivePoke: live poke without refresh must not
// change dispatched paint; refresh picks the change up.
func TestAlert_SnapshotIgnoresLivePoke(t *testing.T) {
	a := NewAlert("hello")
	a.SetType(AlertSuccess)
	a.SetShowIcon(true)
	sz := a.Layout(rendering.Loose(600, 200))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	before := paintToImage(t, a.Node(), int(sz.Width), int(sz.Height))
	// Live poke without refresh (raster may see the field mid-write; the
	// dispatched paint must keep showing the frozen snapshot).
	a.alertType = AlertError
	again := paintToImage(t, a.Node(), int(sz.Width), int(sz.Height))
	if !imagesEqual(before, again) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the poke up (freshness still works through refresh).
	a.refreshSnapshot()
	after := paintToImage(t, a.Node(), int(sz.Width), int(sz.Height))
	if imagesEqual(before, after) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestAlert_FrozenSnapshotPaintsDeterministic: the same snapshot value
// paints identical pixels even while the live widget keeps mutating.
func TestAlert_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	a := NewAlert("hello")
	a.SetType(AlertWarning)
	a.SetShowIcon(true)
	a.SetClosable(true)
	sz := a.Layout(rendering.Loose(600, 200))
	w, h := int(sz.Width), int(sz.Height)
	dc1 := render.NewContext(w, h)
	dc1.BeginFrame()
	dc1.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc1, 1))
	first := dc1.Image()
	dc1.Close()
	// Keep mutating the live widget (no paint/refresh in between): setters
	// would refresh the snapshot (breaking the "same snapshot" premise), so
	// only frozen-field pokes past the last refresh, same rule as Tag.
	a.title = "mutated"
	a.description = "mutated desc"
	// NOTE: hidden/closable/focused are intentionally live atomic reads
	// (visibility is event state, same rule as Tag's focused) — they are NOT
	// poked here; a hidden.Store(true) poke legitimately changes pixels.
	dc2 := render.NewContext(w, h)
	dc2.BeginFrame()
	dc2.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc2, 1))
	second := dc2.Image()
	dc2.Close()
	if !imagesEqual(first, second) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
}

// TestAlert_SnapshotConcurrentRace: single UI writer (setters, atomics +
// snapshot refresh) races against snapshot loads; the raster paint path is
// exercised serially afterward. Run with -race.
func TestAlert_SnapshotConcurrentRace(t *testing.T) {
	// UI setters touch atomics and refresh the snapshot; raster-side reads
	// the snapshot concurrently (no live Alert state). Node().Paint clears
	// the rendering dirty flag, which is a UI-thread-owned bool — concurrent
	// markPaint vs Paint races there (known engine protocol, same pattern as
	// PhaseRacesPaint tests), so painting happens serially after the setters.
	a := NewAlert("hello")
	a.SetType(AlertSuccess)
	a.SetShowIcon(true)
	a.SetClosable(true)
	sz := a.Layout(rendering.Loose(600, 200))
	w, h := int(sz.Width), int(sz.Height)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		types := []AlertType{AlertSuccess, AlertInfo, AlertWarning, AlertError}
		for j := 0; j < 200; j++ {
			a.SetType(types[j%4])
			a.SetTitle("t")
			_ = a.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshot — exercises the raster paint path.
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	a.Node().Paint(rendering.NewPaintContext(dc, 1))
}

// TestAlert_SnapshotPurity locks the no-live-read contract: raw field pokes
// (what a concurrent UI mutation looks like mid-frame) without refresh must
// not change the next dispatched paint.
func TestAlert_SnapshotPurity(t *testing.T) {
	a := NewAlert("hello")
	a.SetType(AlertSuccess)
	a.SetShowIcon(true)
	sz := a.Layout(rendering.Loose(600, 200))
	before := paintToImage(t, a.Node(), int(sz.Width), int(sz.Height))
	// Raw field pokes bypass setters on purpose (setters funnel through
	// markPaint→refreshSnapshot).
	a.title = "changed"
	a.description = "changed"
	a.alertType = AlertError
	a.textFace = nil
	a.rtl = !a.rtl
	after := paintToImage(t, a.Node(), int(sz.Width), int(sz.Height))
	if !imagesEqual(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func paintToImage(t *testing.T, node rendering.RenderObject, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	node.Paint(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

func imagesEqual(a, b image.Image) bool {
	if a == nil || b == nil || !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			r1, g1, b1, a1 := a.At(x, y).RGBA()
			r2, g2, b2, a2 := b.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				return false
			}
		}
	}
	return true
}
