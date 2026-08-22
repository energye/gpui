package rendering_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// R10 paint-only contract: SetImage marks the node for repaint only — it must
// never dirty layout (the image size is fixed; a buffer swap is a pixel
// change, not a geometry change).
func TestRenderImage_SetImage_PaintOnly(t *testing.T) {
	im := rendering.NewRenderImage(96, 56)
	im.SetLoading()
	root := rendering.NewAbsoluteBox(200, 120)
	root.Place(im, 10, 10)

	owner := rendering.NewPipelineOwner(root)
	owner.FlushLayout(rendering.Size{Width: 200, Height: 120}, true)
	visits := int64(0)
	owner.FlushPaint(&rendering.PaintContext{PaintVisits: &visits}, false)

	if im.NeedsPaint() || root.NeedsLayout() || im.NeedsLayout() {
		t.Fatal("tree must be settled before SetImage")
	}

	m := image.NewRGBA(image.Rect(0, 0, 96, 56))
	m.Set(0, 0, color.RGBA{R: 30, G: 160, B: 200, A: 255})
	buf := render.ImageBufFromImage(m)
	defer buf.Dispose()

	im.SetImage(buf)

	if !im.NeedsPaint() {
		t.Fatal("SetImage must mark the node for repaint")
	}
	if im.NeedsLayout() || root.NeedsLayout() {
		t.Fatal("SetImage must not dirty layout (paint-only)")
	}
}
