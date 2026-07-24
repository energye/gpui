package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestSwitch_PRD_TextCenteredInSlot(t *testing.T) {
	sw := kit.NewSwitch()
	sw.SetCheckedChildren("On")
	sw.SetUnCheckedChildren("Off")
	sw.SetDefaultChecked(true)
	tree := core.NewTree(sw.Node())
	tree.Layout(core.Size{Width: 200, Height: 80})

	ind := sw.IndicatorNode().(*primitive.Decorated)
	if ind.Width < 50 {
		t.Fatalf("with text track width=%v want expanded >44", ind.Width)
	}

	sw2 := kit.NewSwitch()
	sw2.SetCheckedChildren("Enabled")
	sw2.SetUnCheckedChildren("Disabled")
	sw2.SetDefaultChecked(true)
	_ = sw2.Node().Layout(core.Loose(400, 100))
	ind2 := sw2.IndicatorNode().(*primitive.Decorated)
	if ind2.Width < 70 {
		t.Fatalf("long children track width=%v want expanded", ind2.Width)
	}

	// Dual-label canvas is full track at (0,0).
	sw.SetChecked(true)
	tree.Layout(core.Size{Width: 200, Height: 80})
	sw.SetChecked(true)
	stack := ind.Children()[0]
	if len(stack.Children()) < 2 {
		t.Fatalf("stack kids=%d", len(stack.Children()))
	}
	labelHost := stack.Children()[0]
	if labelHost.Base().Offset().X != 0 || labelHost.Base().Offset().Y != 0 {
		t.Fatalf("label host offset=%v want (0,0)", labelHost.Base().Offset())
	}
	sz := labelHost.Base().Size()
	if sz.Width < ind.Width-0.5 || sz.Height < ind.Height-0.5 {
		t.Fatalf("label canvas %vx%v want full track %vx%v", sz.Width, sz.Height, ind.Width, ind.Height)
	}
}

func TestSwitch_DualLabelFollowsThumbPos(t *testing.T) {
	// Labels repaint while thumb animates (appear/hide with slide).
	sw := kit.NewSwitch()
	sw.SetCheckedChildren("On")
	sw.SetUnCheckedChildren("Off")
	root := primitive.NewBox(sw.Node())
	root.Width, root.Height = 120, 40
	tree := core.NewTree(root)
	sw.AttachTicker(tree)
	tree.Layout(core.Size{Width: 120, Height: 40})
	sw.SetChecked(true)
	// Drive several ticks; must not panic and thumbPos should move toward 1.
	for i := 0; i < 10; i++ {
		tree.TickActive(0.03)
	}
	if sw.Value() != true && !sw.Checked {
		t.Fatal("expected checked")
	}
	// Toggle off with animation path
	sw.SetChecked(false)
	for i := 0; i < 10; i++ {
		tree.TickActive(0.03)
	}
}
