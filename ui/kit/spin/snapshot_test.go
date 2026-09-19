package spin

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// R2-6 snapshot quartet (button snapshot paradigm): paint follows the frozen
// snapshot, never live widget fields.

func spinPaintToImage(t *testing.T, s *Spin, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Node().Paint(rendering.NewPaintContext(dc, 1))
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

// spinPaintEqual checks the sp {80,80} cell is lighter than pure white after
// the builtin 4-dot looper paints (primary ink on white).
func spinPaintedSomething(m image.Image) bool {
	if m == nil {
		return false
	}
	for y := 0; y < m.Bounds().Dy(); y++ {
		for x := 0; x < m.Bounds().Dx(); x++ {
			r, g, b, a := m.At(x, y).RGBA()
			if a > 0 && (r < 65535 || g < 65535 || b < 65535) {
				return true
			}
		}
	}
	return true
}

func newSpinSnapped(t *testing.T) *Spin {
	t.Helper()
	s := NewSpin(nil)
	s.Layout(rendering.Loose(120, 120))
	return s
}

// TestSpin_SnapshotIgnoresLivePoke: live pokes without refresh must not
// change dispatched paint; refresh picks the change up.
func TestSpin_SnapshotIgnoresLivePoke(t *testing.T) {
	s := newSpinSnapped(t)
	before := spinPaintToImage(t, s, 120, 120)
	// Live pokes bypass setters on purpose (setters funnel through refresh).
	s.description = "poke"
	s.size = SpinLarge
	s.hasPercent = true
	s.percentVal = 66
	s.fullscreen = true
	s.spinning = false
	again := spinPaintToImage(t, s, 120, 120)
	if !imagesEqual(before, again) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the pokes up (freshness still works through refresh).
	s.refreshSnapshot()
	after := spinPaintToImage(t, s, 120, 120)
	if imagesEqual(before, after) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestSpin_FrozenSnapshotPaintsDeterministic: the same snapshot value paints
// identical pixels even while the live widget keeps mutating.
func TestSpin_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	s := newSpinSnapped(t)
	first := spinPaintToImage(t, s, 120, 120)
	// Keep mutating the live widget (no paint/refresh in between).
	s.description = "poke"
	s.tip = "tip-poke"
	s.size = SpinSmall
	s.hasPercent = true
	s.percentVal = 7
	s.percentAuto = true
	s.autoVal = 42
	s.fullscreen = true
	s.spinning = false
	s.delayMs = 400
	s.display = false
	second := spinPaintToImage(t, s, 120, 120)
	if !imagesEqual(first, second) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
}

// TestSpin_SnapshotConcurrentRace: single UI writer (setters, atomics +
// snapshot refresh) races against snapshot loads; the raster paint path is
// exercised serially afterward. Run with -race.
func TestSpin_SnapshotConcurrentRace(t *testing.T) {
	// UI setters refresh the snapshot; raster-side reads it concurrently (no
	// live Spin state). Node paint clears the rendering dirty flag, which is
	// a UI-thread-owned bool — the same engine protocol as PhaseRacesPaint.
	s := NewSpin(nil)
	s.Layout(rendering.Loose(120, 120))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		for j := 0; j < 200; j++ {
			s.SetSpinning(j%2 == 0)
			s.SetDescription("d")
			s.SetTip("t")
			s.SetPercent(float64(j % 100))
			s.SetSize(SpinSize([]string{"small", "medium", "large"}[j%3]))
			_ = s.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshot — exercises the raster paint path.
	spinPaintToImage(t, s, 120, 120)
}

// TestSpin_SnapshotPurity locks the no-live-read contract: raw field pokes
// without refresh must not change the next dispatched paint.
func TestSpin_SnapshotPurity(t *testing.T) {
	s := newSpinSnapped(t)
	before := spinPaintToImage(t, s, 120, 120)
	// Raw field pokes bypass setters on purpose (setters funnel through refresh).
	ind := rendering.NewRenderBox()
	s.description = "poke"
	s.tip = "tip-poke"
	s.size = SpinSmall
	s.hasPercent = true
	s.percentVal = 7
	s.percentAuto = true
	s.autoVal = 42
	s.fullscreen = true
	s.spinning = false
	s.delayMs = 400
	s.display = false
	s.reduceMotion = true
	s.indicator = ind
	after := spinPaintToImage(t, s, 120, 120)
	if !imagesEqual(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
	if !spinPaintedSomething(before) {
		t.Fatal("builtin looper painted nothing on a fresh spin")
	}
	snap := s.loadSnapshot()
	if !snap.Visible || snap.WantMask {
		t.Fatalf("snapshot lost the frozen visibility/mask decision: %+v", snap)
	}
	if snap.HasPercent {
		t.Fatalf("snapshot picked up live percent poke: %+v", snap)
	}
	if snap.Indicator != nil {
		t.Fatalf("snapshot picked up live indicator poke: %+v", snap)
	}
	if snap.Description != "" {
		t.Fatalf("snapshot picked up live description poke: %+v", snap)
	}
}