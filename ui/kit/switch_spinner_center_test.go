package kit_test

import (
	"math"
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestSwitch_PRD_LoadingSpinnerCentered(t *testing.T) {
	sw := kit.NewSwitch()
	sw.SetLoading(true)
	sw.SetDefaultChecked(true)
	tree := core.NewTree(sw.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})

	thumb := sw.ThumbNode().(*primitive.Decorated)
	if len(thumb.Children()) < 1 {
		t.Fatal("no spinner host in thumb")
	}
	// Stack fills thumb; Positioned(AlignCenter) wraps canvas.
	stack := thumb.Children()[0]
	if len(stack.Children()) < 1 {
		t.Fatal("empty spinner stack")
	}
	host := stack.Children()[0]
	// walk to canvas
	var canvas *primitive.Canvas
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		if c, ok := n.(*primitive.Canvas); ok {
			canvas = c
			return
		}
		for _, ch := range n.Children() {
			walk(ch)
		}
	}
	walk(thumb)
	if canvas == nil {
		t.Fatal("no canvas spinner")
	}
	// Absolute center of canvas vs thumb
	tAbs := core.AbsoluteBounds(thumb)
	cAbs := core.AbsoluteBounds(canvas)
	tcx := (tAbs.Min.X + tAbs.Max.X) / 2
	tcy := (tAbs.Min.Y + tAbs.Max.Y) / 2
	ccx := (cAbs.Min.X + cAbs.Max.X) / 2
	ccy := (cAbs.Min.Y + cAbs.Max.Y) / 2
	if math.Abs(ccx-tcx) > 1.0 || math.Abs(ccy-tcy) > 1.0 {
		t.Fatalf("spinner center (%.1f,%.1f) vs thumb center (%.1f,%.1f) hostOff=%v",
			ccx, ccy, tcx, tcy, host.Base().Offset())
	}
}
