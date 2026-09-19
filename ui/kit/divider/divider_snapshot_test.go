package divider

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintDivider drives the real paint entry into an image.
func paintDivider(t *testing.T, d *Divider, w, h float64) image.Image {
	t.Helper()
	dc := render.NewContext(int(w), int(h))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	d.paint(rendering.NewPaintContext(dc, 1), rendering.Size{Width: w, Height: h})
	return dc.Image()
}

// TestDivider_SnapshotRace locks the snapshot handoff (R2-6, button
// paradigm): UI setters keep mutating while the paint path reads only the
// frozen snapshot. Run with -race.
func TestDivider_SnapshotRace(t *testing.T) {
	d := NewDividerWithTitle("race")
	d.SetVariant(Dashed)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // UI thread: setters refresh the snapshot
		defer wg.Done()
		for j := 0; j < 200; j++ {
			d.SetTitle("row")
			d.SetVariant(Dotted)
			d.SetStyle(Style{Border: render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1}})
			d.SetOrientationMargin(0.1)
			d.SetPlain(j%2 == 0)
			d.SetVariant(Solid)
		}
	}()
	go func() { // raster thread: paint from snapshot only
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = paintDivider(t, d, 300, 60)
		}
	}()
	wg.Wait()
}

// TestDivider_SnapshotPurity locks the no-live-read contract: mutating the
// widget without dirty/refresh must not change the next paint.
func TestDivider_SnapshotPurity(t *testing.T) {
	d := NewDividerWithTitle("purity")
	before := paintDivider(t, d, 300, 60)
	// Live pokes bypass setters on purpose: setters funnel through
	// rebuild/markPaint→refreshSnapshot, so only a raw field write (what a
	// concurrent UI mutation looks like mid-frame) proves the paint reads
	// the frozen snapshot, not live state.
	d.title = "changed"
	d.variant = Dotted
	d.style = Style{Border: render.RGBA{R: 1, A: 1}}
	after := paintDivider(t, d, 300, 60)
	if !imagesEqualDivider(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func imagesEqualDivider(a, b image.Image) bool {
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
