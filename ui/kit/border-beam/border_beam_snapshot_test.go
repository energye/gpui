package border_beam

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintBeam drives the real paint entry into an image.
func paintBeam(t *testing.T, b *BorderBeam, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	b.paintBeam(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

// TestBorderBeam_SnapshotRace locks the snapshot handoff (R2-6, button
// paradigm): UI setters keep mutating while the paint path reads only the
// frozen snapshot (plus the atomic phase). Run with -race.
func TestBorderBeam_SnapshotRace(t *testing.T) {
	b := NewBorderBeam(nil)
	b.SetSize(60)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // UI thread: setters refresh the snapshot via dirty
		defer wg.Done()
		for j := 0; j < 200; j++ {
			b.SetColorStops(BorderBeamColorStop{Color: render.RGBA{R: 1, A: 1}, Percent: 0})
			b.SetSize(40 + float64(j%20))
			b.SetLineWidth(2)
			b.SetOutset(float64(j % 3))
			b.SetShowOnHover(j%2 == 0)
			b.SetHovered(j%4 == 0)
		}
	}()
	go func() { // raster thread: paint from snapshot only
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = paintBeam(t, b, 64, 64)
		}
	}()
	wg.Wait()
}

// TestBorderBeam_SnapshotPurity locks the no-live-read contract: mutating
// the widget without dirty/refresh must not change the next paint.
func TestBorderBeam_SnapshotPurity(t *testing.T) {
	b := NewBorderBeam(nil)
	before := paintBeam(t, b, 64, 64)
	// Live pokes bypass setters on purpose: setters funnel through
	// dirty→refreshSnapshot, so only a raw field write (what a concurrent
	// UI mutation looks like mid-frame) proves the paint reads the frozen
	// snapshot, not live state.
	b.size = 10
	b.lineWidth = 5
	b.stops = []BorderBeamColorStop{{Color: render.RGBA{R: 1, A: 1}, Percent: 0}}
	b.hasColor = true
	after := paintBeam(t, b, 64, 64)
	if !imagesEqualBeam(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func imagesEqualBeam(a, b image.Image) bool {
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
