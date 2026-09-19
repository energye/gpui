package float_button

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintFloatButton drives the real raster entry (paint) into an image.
func paintFloatButton(t *testing.T, b *FloatButton, wpx, hpx int) image.Image {
	t.Helper()
	dc := render.NewContext(wpx, hpx)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.paint(rendering.NewPaintContext(dc, 1), rendering.Size{Width: float64(wpx), Height: float64(hpx)})
	return dc.Image()
}

// TestFloatButton_SnapshotRace locks the snapshot handoff (R2-6, button
// paradigm): UI setters keep mutating while the paint path reads only the
// frozen snapshot (plus the atomic phase). Run with -race.
func TestFloatButton_SnapshotRace(t *testing.T) {
	b := NewFloatButton()
	b.SetContent("race")
	b.SetType(ButtonTypePrimary)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // UI thread: setters refresh the snapshot via dirty
		defer wg.Done()
		for j := 0; j < 200; j++ {
			b.SetContent("row")
			b.SetShape(FloatButtonShapeSquare)
			b.SetDisabled(j%2 == 0)
			b.SetHover(j%3 == 0)
			b.SetShape(FloatButtonShapeCircle)
			b.SetLoading(j%50 == 0)
			b.SetLoading(false)
		}
	}()
	go func() { // raster thread: paint from snapshot only
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = paintFloatButton(t, b, 64, 64)
		}
	}()
	wg.Wait()
}

// TestFloatButton_SnapshotPurity locks the no-live-read contract: mutating
// the widget without dirty/refresh must not change the next paint.
func TestFloatButton_SnapshotPurity(t *testing.T) {
	b := NewFloatButton()
	b.SetContent("purity")
	before := paintFloatButton(t, b, 64, 64)
	// Live pokes bypass setters on purpose: setters funnel through
	// dirty→refreshSnapshot, so only a raw field write (what a concurrent
	// UI mutation looks like mid-frame) proves the paint reads the frozen
	// snapshot, not live state.
	b.content = "changed"
	b.shape = FloatButtonShapeSquare
	b.disabled = true
	after := paintFloatButton(t, b, 64, 64)
	if !imagesEqualFloat(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func imagesEqualFloat(a, b image.Image) bool {
	if a == nil || b == nil || !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	for y := 0; y < a.Bounds().Dy(); y++ {
		for x := 0; x < a.Bounds().Dx(); x++ {
			ar, ag, ab, aa := a.At(x, y).RGBA()
			br, bg, bb, ba := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb || aa != ba {
				return false
			}
		}
	}
	return true
}
