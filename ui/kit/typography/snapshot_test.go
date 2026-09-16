package typography

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// T2 D1: paint follows the UI snapshot, never live fields.
func TestTypo_SnapshotIgnoresLivePoke(t *testing.T) {
	tp := NewTypography("Ant Design Text basic")
	tp.Layout(rendering.Loose(1000, 1000))
	before := paintSnapToImage(t, tp, tp.loadSnap())
	// Live pokes without refresh: dispatched paint must not change.
	tp.value = "mutated without refresh"
	tp.underline = true
	after := paintSnapToImage(t, tp, tp.loadSnap())
	if !imagesEqual(before, after) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the pokes up.
	tp.refreshSnapshot()
	changed := paintSnapToImage(t, tp, tp.loadSnap())
	if imagesEqual(before, changed) {
		t.Fatal("refresh did not pick up state change")
	}
}

// T2 D1: same snapshot value paints deterministically under live mutation.
func TestTypo_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	tp := NewTypography("Paragraph line for layout and paint")
	tp.Layout(rendering.Loose(1000, 1000))
	snap := tp.loadSnap()
	a := paintSnapToImage(t, tp, snap)
	tp.SetValue("changed after freeze")
	tp.SetUnderline(true)
	tp.SetMark(true)
	c := paintSnapToImage(t, tp, snap)
	if !imagesEqual(a, c) {
		t.Fatal("same snapshot value painted different pixels after live mutations")
	}
}

// T2 D11 race smoke: single UI writer vs concurrent raster readers.
func TestTypo_SnapshotConcurrentRace(t *testing.T) {
	tp := NewTypography("race smoke text")
	tp.Layout(rendering.Loose(1000, 1000))
	sz := tp.Node().Size()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			tp.SetUnderline(j%2 == 0)
			tp.SetMark(j%3 == 0)
			_ = tp.loadSnap()
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				snap := tp.loadSnap()
				dc := render.NewContext(int(sz.Width), int(sz.Height))
				dc.BeginFrame()
				PaintTypo(rendering.NewPaintContext(dc, 1), sz, snap)
				dc.Close()
			}
		}()
	}
	wg.Wait()
}

func paintSnapToImage(t *testing.T, tp *Typography, snap PaintSnap) image.Image {
	t.Helper()
	sz := tp.Node().Size()
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	PaintTypo(rendering.NewPaintContext(dc, 1), sz, snap)
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
