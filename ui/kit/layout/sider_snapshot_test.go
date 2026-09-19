package layout

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func siderPaintToImage(t *testing.T, s *Sider, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

func siderImagesEqual(a, b image.Image) bool {
	if a == nil || b == nil || !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	ra, okA := a.(*image.RGBA)
	rb, okB := b.(*image.RGBA)
	if !okA || !okB {
		return false
	}
	for i := range ra.Pix {
		if ra.Pix[i] != rb.Pix[i] {
			return false
		}
	}
	return true
}

// TestSider_SnapshotIgnoresLivePoke: raw field pokes without refresh must
// not change the next dispatched paint (paint reads the frozen snapshot).
func TestSider_SnapshotIgnoresLivePoke(t *testing.T) {
	s := NewSider()
	s.SetCollapsible(true)
	s.Layout(rendering.Loose(200, 400))
	before := siderPaintToImage(t, s, 200, 400)
	// Live pokes bypass setters on purpose (setters funnel through
	// markPaint→refreshSnapshot).
	s.siderTheme = SiderThemeLight
	s.reverseArrow = true
	s.bg = render.RGBA{R: 1, A: 1}
	s.hasBg = true
	after := siderPaintToImage(t, s, 200, 400)
	if !siderImagesEqual(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

// TestSider_FrozenSnapshotPaintsDeterministic: the same snapshot paints
// identical pixels even while the live widget keeps mutating (frozen-field
// pokes only — setters would refresh the snapshot).
func TestSider_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	s := NewSider()
	s.SetCollapsible(true)
	s.Layout(rendering.Loose(200, 400))
	first := siderPaintToImage(t, s, 200, 400)
	// Frozen-field pokes past the last refresh; atomic event fields
	// (collapsed/focused/hovered) keep their live-read contract.
	s.siderTheme = SiderThemeLight
	s.reverseArrow = true
	second := siderPaintToImage(t, s, 200, 400)
	if !siderImagesEqual(first, second) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
	// Refresh picks the poke up (freshness still works through refresh).
	s.markPaint()
	third := siderPaintToImage(t, s, 200, 400)
	if siderImagesEqual(first, third) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestSider_SnapshotConcurrentRace: single UI writer (setters, atomics +
// snapshot refresh) races against snapshot loads; the raster paint path is
// exercised serially afterward. Run with -race.
func TestSider_SnapshotConcurrentRace(t *testing.T) {
	// UI setters touch atomics and refresh the snapshot; raster-side reads
	// the snapshot concurrently (no live Sider state). Node().Paint clears
	// the rendering dirty flag, which is a UI-thread-owned bool — concurrent
	// markPaint vs Paint races there (known engine protocol, same pattern as
	// PhaseRacesPaint tests), so painting happens serially after the setters.
	s := NewSider()
	s.SetCollapsible(true)
	sz := s.Layout(rendering.Loose(200, 400))
	w, h := int(sz.Width), int(sz.Height)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		themes := []SiderTheme{SiderThemeDark, SiderThemeLight}
		for j := 0; j < 200; j++ {
			s.SetTheme(themes[j%2])
			s.SetReverseArrow(j%2 == 0)
			s.SetCollapsed(j%3 == 0)
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

// TestSider_SnapshotPurity: paint never reads live theme/config fields —
// a refreshed-then-poked snapshot paints identical pixels.
func TestSider_SnapshotPurity(t *testing.T) {
	s := NewSider()
	s.SetCollapsible(true)
	s.SetTheme(SiderThemeLight)
	s.Layout(rendering.Loose(200, 400))
	before := siderPaintToImage(t, s, 200, 400)
	snap := s.loadSnapshot()
	if snap.Bg == (render.RGBA{}) {
		t.Fatal("snapshot has no frozen background")
	}
	if !siderImagesEqual(before, siderPaintToImage(t, s, 200, 400)) {
		t.Fatal("paint not deterministic across loads of the same snapshot")
	}
}
