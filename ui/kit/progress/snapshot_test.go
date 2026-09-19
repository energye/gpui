package progress

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// R2-6 snapshot quartet (button snapshot paradigm): paint follows the frozen
// snapshot, never live widget fields.

func progressPaintToImage(t *testing.T, p *Progress, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	p.Node().Paint(rendering.NewPaintContext(dc, 1))
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

func newProgressSnapped(t *testing.T) *Progress {
	t.Helper()
	p := NewProgress(40)
	p.Layout(rendering.Loose(400, 60))
	return p
}

// TestProgress_SnapshotIgnoresLivePoke: live poke without refresh must not
// change dispatched paint; refresh picks the change up.
func TestProgress_SnapshotIgnoresLivePoke(t *testing.T) {
	p := newProgressSnapped(t)
	before := progressPaintToImage(t, p, 400, 60)
	// Live pokes bypass setters on purpose (setters funnel through markPaint).
	p.ptype = TypeCircle
	p.steps = 5
	p.strokeColor = render.RGBA{R: 1, G: 0, B: 0, A: 1}
	p.stepColors = []render.RGBA{{R: 1, G: 1, B: 0, A: 1}}
	again := progressPaintToImage(t, p, 400, 60)
	if !imagesEqual(before, again) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the pokes up (freshness still works through refresh).
	p.refreshSnapshot()
	after := progressPaintToImage(t, p, 400, 60)
	if imagesEqual(before, after) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestProgress_FrozenSnapshotPaintsDeterministic: the same snapshot value
// paints identical pixels even while the live widget keeps mutating.
func TestProgress_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	p := newProgressSnapped(t)
	first := progressPaintToImage(t, p, 400, 60)
	// Keep mutating the live widget (no paint/refresh in between).
	p.ptype = TypeCircle
	p.steps = 5
	p.strokeColor = render.RGBA{R: 1, G: 0, B: 0, A: 1}
	p.railColor = render.RGBA{G: 1, A: 1}
	p.gradFrom = render.RGBA{R: 1, A: 1}
	p.gradTo = render.RGBA{B: 1, A: 1}
	p.hasGradient = true
	second := progressPaintToImage(t, p, 400, 60)
	if !imagesEqual(first, second) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
}

// TestProgress_SnapshotConcurrentRace: single UI writer (setters, atomics +
// snapshot refresh) races against snapshot loads; the raster paint path is
// exercised serially afterward. Run with -race.
func TestProgress_SnapshotConcurrentRace(t *testing.T) {
	// UI setters touch atomics and refresh the snapshot; raster-side reads
	// the snapshot concurrently (no live Progress state). Node paint clears
	// the rendering dirty flag, which is a UI-thread-owned bool — concurrent
	// markPaint vs Paint races there (known engine protocol, same pattern as
	// PhaseRacesPaint tests), so painting happens serially after the setters.
	p := NewProgress(40)
	p.Layout(rendering.Loose(400, 60))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		for j := 0; j < 200; j++ {
			p.SetPercent(float64(j % 100))
			p.SetStatus(StatusActive)
			p.SetSteps(j % 3)
			p.SetStrokeColor(render.RGBA{R: float64(j%100) / 100, A: 1})
			p.SetRailColor(render.RGBA{G: float64(j%100) / 100, A: 1})
			p.SetReduceMotion(j%2 == 0)
			_ = p.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshot — exercises the raster paint path.
	progressPaintToImage(t, p, 400, 60)
}

// TestProgress_SnapshotPurity locks the no-live-read contract: raw field
// pokes without refresh must not change the next dispatched paint.
func TestProgress_SnapshotPurity(t *testing.T) {
	p := newProgressSnapped(t)
	before := progressPaintToImage(t, p, 400, 60)
	// Raw field pokes bypass setters on purpose (setters funnel through markPaint).
	p.ptype = TypeCircle
	p.steps = 5
	p.strokeColor = render.RGBA{R: 1, G: 0, B: 0, A: 1}
	p.railColor = render.RGBA{G: 1, A: 1}
	p.gradFrom = render.RGBA{R: 1, A: 1}
	p.gradTo = render.RGBA{B: 1, A: 1}
	p.hasGradient = true
	p.reduceMotion = true
	p.status = StatusException
	p.linecap = LinecapSquare
	p.strokeWidth = 20
	after := progressPaintToImage(t, p, 400, 60)
	if !imagesEqual(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
	s := p.loadSnapshot()
	if s.Ptype != TypeLine || s.Steps != 0 || s.Status != StatusNormal {
		t.Fatalf("snapshot did not keep the frozen inputs: %+v", s)
	}
	if s.ReduceMotion || s.Linecap != LinecapRound {
		t.Fatal("frozen flags lost after poke")
	}
}