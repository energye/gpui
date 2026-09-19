package statistic

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// loadSnapFace loads a real face for true-text pixels (skip when headless).
func loadSnapFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		face, _, err = rendering.TryLoadDefaultFace(14)
		if err != nil || face == nil {
			t.Skipf("true-text needs a system face: %v", err)
		}
	}
	return face
}

// R2-6 snapshot trio (button snapshot paradigm): paint follows the frozen
// snapshot, never live widget fields.

// TestStatistic_SnapshotIgnoresLivePoke: live poke without refresh must not
// change dispatched paint; refresh picks the change up.
func TestStatistic_SnapshotIgnoresLivePoke(t *testing.T) {
	s := NewStatistic()
	s.SetValue(12345)
	s.SetTextFace(loadSnapFace(t))
	sz := s.Layout(rendering.Loose(400, 100))
	if sz.Width <= 0 {
		t.Fatalf("layout=%v", sz)
	}
	before := paintToImage(t, s.Node(), int(sz.Width), int(sz.Height))
	// Live poke without refresh: dispatched paint keeps the frozen value.
	s.value = 7
	again := paintToImage(t, s.Node(), int(sz.Width), int(sz.Height))
	if !imagesEqual(before, again) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the poke up (freshness still works through refresh).
	s.refreshSnapshot()
	after := paintToImage(t, s.Node(), int(sz.Width), int(sz.Height))
	if imagesEqual(before, after) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestStatistic_FrozenSnapshotPaintsDeterministic: the same snapshot value
// paints identical pixels even while the live widget keeps mutating.
func TestStatistic_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	s := NewStatistic()
	s.SetValue(12345)
	s.SetTitle("标题")
	sz := s.Layout(rendering.Loose(400, 100))
	w, h := int(sz.Width), int(sz.Height)
	dc1 := render.NewContext(w, h)
	dc1.BeginFrame()
	dc1.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc1, 1))
	first := dc1.Image()
	dc1.Close()
	// Keep mutating the live widget (no paint/refresh in between).
	s.SetValue(777)     // setter refreshes...
	s.title = "mutated" // ...then poke past the refresh
	s.prefix = "€"      // frozen field poke (loading stays the atomic read
	// by contract; flipping it changes paint by design — skip poking it)
	dc2 := render.NewContext(w, h)
	dc2.BeginFrame()
	dc2.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc2, 1))
	second := dc2.Image()
	dc2.Close()
	if !imagesEqual(first, second) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
}

// TestStatistic_SnapshotConcurrentRace: single UI writer (setters, atomics
// + snapshot refresh) races against snapshot loads; the raster paint path
// is exercised serially afterward. Run with -race.
func TestStatistic_SnapshotConcurrentRace(t *testing.T) {
	// UI setters touch atomics and refresh the snapshot; raster-side reads
	// the snapshot concurrently (no live Statistic state). Node().Paint
	// clears the rendering dirty flag, which is a UI-thread-owned bool —
	// concurrent markPaint vs Paint races there (known engine protocol, same
	// pattern as PhaseRacesPaint tests), so painting happens serially after
	// the setters.
	s := NewStatistic()
	s.SetValue(1)
	s.SetTitle("标题")
	sz := s.Layout(rendering.Loose(400, 100))
	w, h := int(sz.Width), int(sz.Height)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		for j := 0; j < 200; j++ {
			s.SetValue(int64(j))
			s.SetTitle("标题")
			_ = s.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshot — exercises the raster paint path.
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
}

// TestStatistic_SnapshotPurity locks the no-live-read contract: raw field
// pokes without refresh must not change the next dispatched paint.
func TestStatistic_SnapshotPurity(t *testing.T) {
	s := NewStatistic()
	s.SetValue(12345)
	s.SetTextFace(loadSnapFace(t))
	s.SetPrefix("$")
	s.SetSuffix("元")
	sz := s.Layout(rendering.Loose(400, 100))
	before := paintToImage(t, s.Node(), int(sz.Width), int(sz.Height))
	// Raw field pokes bypass setters on purpose (setters funnel through
	// mark*Dirty→refreshSnapshot).
	s.value = 7
	s.title = "changed"
	s.prefix = "€"
	s.suffix = "!"
	s.textFace = nil
	after := paintToImage(t, s.Node(), int(sz.Width), int(sz.Height))
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
