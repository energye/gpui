package watermark

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// paintWatermark drives the real raster entry (paintMarks) into an image.
func paintWatermark(t *testing.T, w *Watermark, wpx, hpx int) image.Image {
	t.Helper()
	dc := render.NewContext(wpx, hpx)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	paintMarks := w.paintMarks
	paintMarks(rendering.NewPaintContext(dc, 1))
	return dc.Image()
}

// TestWatermark_SnapshotRace locks the snapshot handoff (R2-6, button
// paradigm): UI setters keep mutating while the paint path reads only the
// frozen snapshot. Run with -race.
func TestWatermark_SnapshotRace(t *testing.T) {
	w := NewWatermark(nil)
	w.SetContent("hello")
	w.SetWidth(120)
	w.SetHeight(40)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { // UI thread: setters refresh the snapshot via dirtyMark
		defer wg.Done()
		for j := 0; j < 200; j++ {
			w.SetContentLines(WatermarkContentLine{Text: "row", Font: WatermarkFont{FontSize: 12}})
			w.SetRotate(float64(-j % 45))
			w.SetGap(float64(90 + j%20), 100)
			if j%50 == 0 {
				w.SetImagePixels(2, 2, []byte{
					0, 0, 0, 255, 0, 0, 0, 255,
					0, 0, 0, 255, 0, 0, 0, 255,
				})
			}
		}
	}()
	go func() { // raster thread: paint from snapshot only
		defer wg.Done()
		for j := 0; j < 100; j++ {
			_ = paintWatermark(t, w, 300, 200)
		}
	}()
	wg.Wait()
}

// TestWatermark_SnapshotPurity locks the no-live-read contract: mutating
// the widget without dirty/refresh must not change the next paint.
func TestWatermark_SnapshotPurity(t *testing.T) {
	w := NewWatermark(nil)
	w.SetContent("purity")
	w.SetWidth(120)
	w.SetHeight(30)
	before := paintWatermark(t, w, 300, 200)
	// Live mutations without refresh: paint must keep showing the snapshot.
	w.SetRotate(-5)
	w.SetGap(7, 9)
	w.SetContentLines(WatermarkContentLine{Text: "changed"})
	after := paintWatermark(t, w, 300, 200)
	if !imagesEqualSnapshot(before, after) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
}

func imagesEqualSnapshot(a, b image.Image) bool {
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
