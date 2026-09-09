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

func findExtra(l scene.Layer) (found bool, w, h int) {
	if l == nil {
		return false, 0, 0
	}
	if pl, ok := l.(*scene.PictureLayer); ok && pl != nil {
		if pl.RasterExtra != nil {
			return true, pl.ExtraBounds.Dx(), pl.ExtraBounds.Dy()
		}
	}
	for _, c := range l.Children() {
		if ok, w, h := findExtra(c); ok {
			return ok, w, h
		}
	}
	return false, 0, 0
}

// TestLayerTree_WrapperOnPaintCaptured locks that a wrapper embedding
// *RenderBox (InputBox family pattern) gets its OnPaint captured as a
// RasterExtra picture layer so the retained path renders box borders.
func TestLayerTree_WrapperOnPaintCaptured(t *testing.T) {
	inner := rendering.NewRenderBox()
	w := &paintWrapper{RenderBox: inner}
	inner.Init(w)
	w.FixedWidth, w.FixedHeight = 100, 40
	painted := false
	w.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		painted = true
		_ = painted
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
	ok, ew, eh := findExtra(pkt.Root)
	if !ok {
		t.Fatal("wrapper OnPaint missing from layer tree (no RasterExtra layer)")
	}
	if ew != 100 || eh != 40 {
		t.Fatalf("ExtraBounds=(%d,%d) want (100,40)", ew, eh)
	}
}
