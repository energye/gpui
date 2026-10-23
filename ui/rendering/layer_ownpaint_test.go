package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scene"
)

// paintWrapper mimics the InputBox family: a custom type embedding
// *rendering.RenderBox with OnPaint set on the embedded box.
type paintWrapper struct {
	*rendering.RenderBox
}

// findPaintedPicture returns whether any keyed picture layer carries visible
// recording content of exactly w×h (the wrapper's own 100×40 border paint).
func findPaintedPicture(l scene.Layer, w, h int) bool {
	if l == nil {
		return false
	}
	if pl, ok := l.(*scene.PictureLayer); ok && pl != nil {
		if pl.CacheKey != 0 && pl.Recording.HasVisiblePaint() {
			b := pl.Recording.Bounds()
			if b.Dx() == w && b.Dy() == h {
				return true
			}
		}
	}
	for _, c := range l.Children() {
		if findPaintedPicture(c, w, h) {
			return true
		}
	}
	return false
}

// TestLayerTree_WrapperOnPaintCaptured locks that a wrapper embedding
// *RenderBox (InputBox family pattern) gets its OnPaint captured into the
// layer's recording during the UI-side build (the retained path draws box
// borders from it; there is no raster-thread closure any more).
func TestLayerTree_WrapperOnPaintCaptured(t *testing.T) {
	inner := rendering.NewRenderBox()
	w := &paintWrapper{RenderBox: inner}
	inner.Init(w)
	w.FixedWidth, w.FixedHeight = 100, 40
	w.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		rendering.FillRect(pc, 0, 0, size.Width, size.Height, 1, 0, 0, 1)
	}
	// Wrapper boxes own children (InputBox owns clip/text/caret).
	kid := rendering.NewRenderColorBox(10, 10, 1, 0, 0, 1)
	w.AddChild(kid)

	root := rendering.NewRenderBox(w)
	root.Layout(rendering.Tight(400, 300))
	lb := rendering.BuildLayerTree(root)
	pkt := lb.BuildPacket(1, 1, 400, 300)
	if pkt == nil || pkt.Root == nil {
		t.Fatal("nil packet/root")
	}
	if !findPaintedPicture(pkt.Root, 100, 40) {
		t.Fatal("wrapper OnPaint missing from layer tree (no visible 100x40 recording)")
	}
}
