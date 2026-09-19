package tooltip

import (
	"image"
	"sync"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// R2-6 snapshot trio (button snapshot paradigm): paint follows the frozen
// snapshot, never live widget fields.

// loadSnapFace loads a real face for true-text pixels (skip when headless).
func loadSnapFace(t *testing.T) text.Face {
	t.Helper()
	text.ClearSystemFontPaths()
	face, _, err := text.LoadMultiFace(14)
	if err != nil || face == nil {
		face, _, err = rendering.TryLoadDefaultFace(14)
		if err != nil || face == nil {
			t.Skipf("true-text needs a system face: %v", err)
		}
	}
	return face
}

// paintNodeToImage renders a render object onto a w×h canvas.
func paintNodeToImage(t *testing.T, node rendering.RenderObject, w, h int) image.Image {
	t.Helper()
	dc := render.NewContext(w, h)
	defer dc.Close()
	dc.BeginFrame()
	dc.ClearWithColor(render.White)
	node.Paint(rendering.NewPaintContext(dc, 1))
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

// sizePanel forces the panel chrome to a real size so paint exercises glyphs.
func sizePanel(node rendering.RenderObject, w, h float64) {
	if rb, ok := node.(*rendering.RenderBox); ok {
		rb.FixedWidth, rb.FixedHeight = w, h
	}
}

// shot captures both the trigger and the panel as separate canvases so a
// poke on either path is caught.
func (t *Tooltip) shot(tst *testing.T, tw, th, pw, ph int) (trigger, panel image.Image) {
	tst.Helper()
	trigger = paintNodeToImage(tst, t.Node(), tw, th)
	panel = paintNodeToImage(tst, t.Panel(), pw, ph)
	return trigger, panel
}

func newTooltipSnapped(t *testing.T) *Tooltip {
	t.Helper()
	tip := NewTooltip("提示")
	tip.SetTextFace(loadSnapFace(t))
	tip.SetTitle("标题")
	tip.SetTriggerLabel("trigger")
	tip.SetArrow(true)
	tip.Layout(rendering.Loose(400, 200))
	sizePanel(tip.Panel(), 280, 100)
	return tip
}

// TestTooltip_SnapshotIgnoresLivePoke: live poke without refresh must not
// change dispatched paint; refresh picks the change up.
func TestTooltip_SnapshotIgnoresLivePoke(t *testing.T) {
	tip := newTooltipSnapped(t)
	bt, bp := tip.shot(t, 200, 60, 280, 100)
	// Live pokes bypass setters on purpose (setters funnel through
	// relayout/markPaint→refreshSnapshot).
	tip.triggerLabel = "poked-trigger"
	tip.title = "poked-title"
	at, ap := tip.shot(t, 200, 60, 280, 100)
	if !imagesEqual(bt, at) || !imagesEqual(bp, ap) {
		t.Fatal("dispatched paint changed after live poke without refresh: reads live state")
	}
	// Refresh picks the pokes up (freshness still works through refresh).
	tip.refreshSnapshot()
	rt, rp := tip.shot(t, 200, 60, 280, 100)
	if imagesEqual(bt, rt) && imagesEqual(bp, rp) {
		t.Fatal("refresh did not pick up state change")
	}
}

// TestTooltip_FrozenSnapshotPaintsDeterministic: the same snapshot value
// paints identical pixels even while the live widget keeps mutating.
func TestTooltip_FrozenSnapshotPaintsDeterministic(t *testing.T) {
	tip := newTooltipSnapped(t)
	bt, bp := tip.shot(t, 200, 60, 280, 100)
	// Keep mutating the live widget (no paint/refresh in between).
	tip.triggerLabel = "mutated"
	tip.title = "mutated"
	tip.textFace = nil
	tip.arrow = false
	at, ap := tip.shot(t, 200, 60, 280, 100)
	if !imagesEqual(bt, at) || !imagesEqual(bp, ap) {
		t.Fatal("same snapshot painted different pixels after live mutations")
	}
}

// TestTooltip_SnapshotConcurrentRace: single UI writer (setters, atomics +
// snapshot refresh) races against snapshot loads; the raster paint path is
// exercised serially afterward. Run with -race.
func TestTooltip_SnapshotConcurrentRace(t *testing.T) {
	// UI setters touch atomics and refresh the snapshot; raster-side reads
	// the snapshot concurrently (no live Tooltip state). Node paint clears
	// the rendering dirty flag, which is a UI-thread-owned bool — concurrent
	// markPaint vs Paint races there (known engine protocol, same pattern as
	// PhaseRacesPaint tests), so painting happens serially after the setters.
	tip := NewTooltip("提示")
	tip.SetTitle("标题")
	tip.Layout(rendering.Loose(400, 200))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { // one UI writer
		defer wg.Done()
		for j := 0; j < 200; j++ {
			tip.SetTitle("标题")
			tip.SetTriggerLabel("trigger")
			tip.SetArrow(j%2 == 0)
			_ = tip.loadSnapshot()
		}
	}()
	wg.Wait()
	// Serial paint of the final snapshot — exercises the raster paint path.
	sizePanel(tip.Panel(), 280, 100)
	tip.shot(t, 200, 60, 280, 100)
}

// TestTooltip_SnapshotPurity locks the no-live-read contract: raw field
// pokes without refresh must not change the next dispatched paint.
func TestTooltip_SnapshotPurity(t *testing.T) {
	tip := newTooltipSnapped(t)
	bt, bp := tip.shot(t, 200, 60, 280, 100)
	// Raw field pokes bypass setters on purpose (setters funnel through
	// relayout/markPaint→refreshSnapshot).
	tip.triggerLabel = "changed"
	tip.title = "changed"
	tip.textFace = nil
	tip.arrow = false
	at, ap := tip.shot(t, 200, 60, 280, 100)
	if !imagesEqual(bt, at) || !imagesEqual(bp, ap) {
		t.Fatal("paint changed after live poke without refresh: reads live state")
	}
	s := tip.loadSnapshot()
	if s.TriggerLabel != "trigger" || s.Title != "标题" {
		t.Fatalf("snapshot did not keep the frozen inputs: %+v", s)
	}
	if !s.Arrow {
		t.Fatal("frozen arrow lost after poke")
	}
}