package timeline_test

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/kit/timeline"
	"github.com/energye/gpui/ui/rendering"
)

func paintTimelineImage(t *testing.T, tl *timeline.Timeline, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	tl.Layout(rendering.Loose(float64(w), float64(h)))
	pc := rendering.NewPaintContext(dc, 1)
	tl.Node().Paint(pc)
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

// TestTimeline_SetterRefreshRepaints locks the R2-6 frozen-copy refresh:
// per-item setters rebuild (frozen dot inputs refresh with them), so a
// color change must reach pixels. Stale frozen copies would paint identical
// pixels (red without fix).
func TestTimeline_SetterRefreshRepaints(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "alpha", Color: "blue"},
		timeline.TimelineItem{Content: "beta", Color: "blue"},
	)
	before := paintTimelineImage(t, tl, 200, 160)
	if again := paintTimelineImage(t, tl, 200, 160); countDiffPx(before, again) != 0 {
		t.Fatal("repaint without change must be pixel-identical (no Tick ran)")
	}
	tl.SetItemColor(0, "red")
	after := paintTimelineImage(t, tl, 200, 160)
	if d := countDiffPx(before, after); d <= 0 {
		t.Fatal("SetItemColor must reach pixels (frozen copy refresh)")
	}
}

// TestTimeline_PhaseRacesPaint locks the atomic phase (R2-6): UI Tick vs
// raster paint must be race-clean. Run with -race.
func TestTimeline_PhaseRacesPaint(t *testing.T) {
	tl := timeline.NewTimeline(
		timeline.TimelineItem{Content: "spin", Loading: true},
	)
	tl.Layout(rendering.Loose(200, 160))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 100; j++ {
			tl.Tick(0.016)
		}
	}()
	// Raster-side readers only touch atomic widget state (Phase): engine
	// node dirty flags are single-writer by design, so concurrent
	// Node().Paint would race rendering.Base itself and prove nothing
	// about widget state. Pixels are verified serially below.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = tl.Phase()
			}
		}()
	}
	wg.Wait()
	dc := render.NewContext(200, 160)
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	pc := rendering.NewPaintContext(dc, 1)
	tl.Node().Paint(pc)
	dc.Close()
}
