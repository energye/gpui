package icon

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// T2 D1: paint follows the UI snapshot, never live fields.
func TestIcon_SnapshotIgnoresLivePoke(t *testing.T) {
	ic := NewIcon("check")
	ic.Layout(rendering.Loose(1000, 1000))
	before := paintSnapToImage(t, ic, ic.loadSnap())
	// Live pokes without refresh: dispatched paint must not change.
	ic.name = "close"
	ic.phase = 0.75
	ic.rotate = 90
	after := paintSnapToImage(t, ic, ic.loadSnap())
	if !imagesEqual(before, after) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the pokes up.
	ic.refreshSnapshot()
	changed := paintSnapToImage(t, ic, ic.loadSnap())
	if imagesEqual(before, changed) {
		t.Fatal("refresh did not pick up state change")
	}
}

// T2 D1: same snapshot value paints deterministically under live mutation.
func TestIcon_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	ic := NewIcon("star")
	ic.Layout(rendering.Loose(1000, 1000))
	snap := ic.loadSnap()
	a := paintSnapToImage(t, ic, snap)
	ic.SetName("heart")
	ic.SetRotate(45)
	ic.SetSpin(true)
	ic.Tick(0.1)
	c := paintSnapToImage(t, ic, snap)
	if !imagesEqual(a, c) {
		t.Fatal("same snapshot value painted different pixels after live mutations")
	}
}

// T2 D11 race smoke: single UI writer vs concurrent raster readers.
func TestIcon_SnapshotConcurrentRace(t *testing.T) {
	ic := NewIcon("check")
	ic.Layout(rendering.Loose(1000, 1000))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			ic.SetRotate(float64(j))
			ic.Tick(0.016)
			_ = ic.loadSnap()
		}
	}()
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				snap := ic.loadSnap()
				dc := render.NewContext(32, 32)
				dc.BeginFrame()
				PaintIcon(rendering.NewPaintContext(dc, 1), 32, snap)
				dc.Close()
			}
		}()
	}
	wg.Wait()
}

func paintSnapToImage(t *testing.T, ic *Icon, snap PaintSnap) image.Image {
	t.Helper()
	sz := ic.EffectiveSize()
	dc := render.NewContext(int(sz), int(sz))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	PaintIcon(rendering.NewPaintContext(dc, 1), sz, snap)
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
