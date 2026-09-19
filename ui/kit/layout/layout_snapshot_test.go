package layout

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintSection drives one section's paint entry into an image.
func paintSection(t *testing.T, node *rendering.RenderBox, paint func(*rendering.PaintContext, rendering.Size), w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	paint(rendering.NewPaintContext(dc, 1), rendering.Size{Width: float64(w), Height: float64(h)})
	_ = node
	return dc.Image()
}

// TestLayout_Sections_SnapshotRace locks the snapshot handoff (R2-6, button
// paradigm): UI setters keep mutating while the paint path reads only the
// frozen background. Run with -race.
func TestLayout_Sections_SnapshotRace(t *testing.T) {
	h := NewHeader()
	f := NewFooter()
	c := NewContent()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // UI thread: setters refresh the snapshot
		defer wg.Done()
		for j := 0; j < 200; j++ {
			h.SetBackground(render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1})
			h.SetHeight(60)
			f.SetBackground(render.RGBA{G: 0.5, A: 1})
			f.SetPaddingInsets(NewInsets(8, 8, 8, 8))
			c.SetBackground(render.RGBA{B: 0.3, A: float64(j % 2)})
			h.SetBackground(render.RGBA{})
		}
	}()
	go func() { // raster thread: paint from snapshot only
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = paintSection(t, h.node, h.paint, 200, 64)
			_ = paintSection(t, f.node, f.paint, 200, 70)
			_ = paintSection(t, c.node, c.paint, 200, 100)
		}
	}()
	wg.Wait()
}

// TestLayout_Sections_SnapshotPurity locks the no-live-read contract:
// mutating the widget without dirty/refresh must not change the next paint.
func TestLayout_Sections_SnapshotPurity(t *testing.T) {
	h := NewHeader()
	f := NewFooter()
	c := NewContent()
	hb := paintSection(t, h.node, h.paint, 200, 64)
	fb := paintSection(t, f.node, f.paint, 200, 70)
	cb := paintSection(t, c.node, c.paint, 200, 100)
	// Live pokes bypass setters on purpose: setters funnel through
	// markPaint→refreshSnapshot, so only a raw field write (what a
	// concurrent UI mutation looks like mid-frame) can prove the paint
	// reads the frozen snapshot, not live state.
	h.bg = render.RGBA{R: 1, A: 1}
	h.hasBg = true
	f.bg = render.RGBA{G: 1, A: 1}
	f.hasBg = true
	c.bg = render.RGBA{B: 1, A: 1}
	c.hasBg = true
	ha := paintSection(t, h.node, h.paint, 200, 64)
	fa := paintSection(t, f.node, f.paint, 200, 70)
	ca := paintSection(t, c.node, c.paint, 200, 100)
	if !imagesEqualLayout(hb, ha) || !imagesEqualLayout(fb, fa) || !imagesEqualLayout(cb, ca) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func imagesEqualLayout(a, b image.Image) bool {
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
