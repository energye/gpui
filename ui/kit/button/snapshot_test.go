package button

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// T2 D1: paint follows the UI snapshot, never live widget fields.
// Poke live state without refresh — dispatched paint must not change.
func TestButton_SnapshotIgnoresLivePoke(t *testing.T) {
	b := NewButton("确定")
	b.Layout(rendering.Loose(1000, 1000))
	sz := b.LaidOut()
	b.PointerMove(sz.Width/2, sz.Height/2) // hover on, snapshot refreshed
	if !b.Hovered() {
		t.Fatal("pointer inside must hover")
	}
	hoveredPixels := paintToImage(t, b)
	// Live poke without refresh: paint must keep showing the snapshot.
	b.hovered = false
	if b.Hovered() {
		t.Fatal("live poke did not clear Hovered()")
	}
	again := paintToImage(t, b)
	if !imagesEqual(hoveredPixels, again) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the poke up (freshness still works through refresh).
	b.refreshSnapshot()
	unhovered := paintToImage(t, b)
	if imagesEqual(hoveredPixels, unhovered) {
		t.Fatal("refresh did not pick up state change")
	}
}

// T2 D1: a frozen snapshot value paints deterministically — same value,
// same pixels, even while the live widget keeps mutating.
func TestButton_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	b := NewButton("确定")
	b.Layout(rendering.Loose(1000, 1000))
	sz := b.LaidOut()
	b.PointerDown(sz.Width/2, sz.Height/2)
	snap := b.loadSnapshot()
	a := paintSnapToImage(t, sz, snap)
	b.PointerUp(sz.Width/2, sz.Height/2)
	b.PointerMove(-100, -100)
	b.Tick(0.05)
	c := paintSnapToImage(t, sz, snap)
	if !imagesEqual(a, c) {
		t.Fatal("same snapshot value painted different pixels after live mutations")
	}
}

// T2 D11 race smoke: single UI writer (Pointer/Tick/refresh, as the real UI
// thread) vs concurrent raster readers (snapshot load + paint) must be
// race-clean. Empty label keeps paint on the chrome path (no text shaping
// globals), so -race isolates widget-state synchronization.
func TestButton_SnapshotConcurrentRace(t *testing.T) {
	b := NewButton("")
	b.Layout(rendering.Loose(1000, 1000))
	sz := b.LaidOut()
	var wg sync.WaitGroup
	// One UI writer.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			b.PointerMove(float64(j), 5)
			b.Tick(0.016)
			_ = b.loadSnapshot()
		}
	}()
	// Concurrent raster readers.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				snap := b.loadSnapshot()
				dc := render.NewContext(int(sz.Width), int(sz.Height))
				dc.BeginFrame()
				PaintButton(rendering.NewPaintContext(dc, 1), sz, snap)
				dc.Close()
			}
		}()
	}
	wg.Wait()
}

func paintToImage(t *testing.T, b *Button) image.Image {
	t.Helper()
	sz := b.LaidOut()
	return paintSnapToImage(t, sz, b.loadSnapshot())
}

func paintSnapToImage(t *testing.T, sz rendering.Size, snap ButtonSnap) image.Image {
	t.Helper()
	dc := render.NewContext(int(sz.Width), int(sz.Height))
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	PaintButton(rendering.NewPaintContext(dc, 1), sz, snap)
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
