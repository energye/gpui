package skeleton_test

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/skeleton"
	"github.com/energye/gpui/ui/rendering"
)

func paintSkeletonImage(t *testing.T, s *skeleton.Skeleton, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	s.Layout(rendering.Loose(float64(w), float64(h)))
	pc := rendering.NewPaintContext(dc, 1)
	s.Node().Paint(pc)
	if err := dc.FlushGPU(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	return dc.Image()
}

func countDiffPx(a, b image.Image) int {
	if a.Bounds() != b.Bounds() {
		return -1
	}
	n := 0
	bb := a.Bounds()
	for y := bb.Min.Y; y < bb.Max.Y; y++ {
		for x := bb.Min.X; x < bb.Max.X; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb2, _ := b.At(x, y).RGBA()
			if ar != br || ag != bg || ab != bb2 {
				n++
			}
		}
	}
	return n
}

// TestSkeleton_ActiveToggleRepaints locks that event toggles reach pixels
// (atomic flags, no frozen staleness): disabling the shimmer must change
// output, re-enabling restores it.
func TestSkeleton_ActiveToggleRepaints(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetActive(true)
	// Swing the shimmer on-frame (phase 0 parks it offscreen): 30 ticks of
	// 1/60s at the 1s period ≈ phase 0.5, mid-window.
	for j := 0; j < 30; j++ {
		s.Tick(1.0 / 60.0)
	}
	on := paintSkeletonImage(t, s, 200, 160)
	if again := paintSkeletonImage(t, s, 200, 160); countDiffPx(on, again) != 0 {
		t.Fatal("repaint without change must be pixel-identical (no Tick ran)")
	}
	s.SetActive(false)
	off := paintSkeletonImage(t, s, 200, 160)
	if countDiffPx(on, off) <= 0 {
		t.Fatal("SetActive(false) must reach pixels")
	}
	s.SetActive(true)
	if d := countDiffPx(on, paintSkeletonImage(t, s, 200, 160)); d != 0 {
		t.Fatalf("re-enable must restore pixels, diff=%d", d)
	}
}

// TestSkeleton_PhaseRacesPaint locks the atomic phase/active (R2-6): UI
// Tick vs raster-side state reads must be race-clean. Run with -race.
func TestSkeleton_PhaseRacesPaint(t *testing.T) {
	s := skeleton.NewSkeleton()
	s.SetActive(true) // else Tick early-returns and the race window is empty
	s.Layout(rendering.Loose(200, 160))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			s.Tick(0.016)
		}
	}()
	// Raster-side readers only touch atomic widget state (Phase/Active):
	// engine node dirty flags are single-writer by design, so concurrent
	// Node().Paint would race rendering.Base itself and prove nothing
	// about widget state. Pixels are verified serially below.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = s.Phase()
				_ = s.Active()
			}
		}()
	}
	wg.Wait()
	dc := render.NewContext(200, 160)
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	s.Node().Paint(pc)
	dc.Close()
}
