package rendering_test

import (
	"testing"

	"github.com/energye/gpui/ui/rendering"
)

func TestAbsoluteBox_PreservesOffsets(t *testing.T) {
	root := rendering.NewAbsoluteBox(200, 100)
	a := rendering.NewRenderColorBox(40, 20, 1, 0, 0, 1)
	b := rendering.NewRenderColorBox(40, 20, 0, 1, 0, 1)
	root.Place(a, 10, 15)
	root.Place(b, 80, 40)
	root.Layout(rendering.Tight(200, 100))
	if a.Offset().X != 10 || a.Offset().Y != 15 {
		t.Fatalf("a offset=%v", a.Offset())
	}
	if b.Offset().X != 80 || b.Offset().Y != 40 {
		t.Fatalf("b offset=%v", b.Offset())
	}
	// Second layout must not clobber
	root.Layout(rendering.Tight(200, 100))
	if a.Offset().X != 10 || b.Offset().Y != 40 {
		t.Fatal("offsets clobbered on second layout")
	}
}
