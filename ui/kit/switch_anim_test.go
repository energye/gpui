package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestSwitchThumbMoves(t *testing.T) {
	sw := kit.NewSwitch()
	root := primitive.NewBox(sw.Node())
	root.Width, root.Height = 100, 40
	tree := core.NewTree(root)
	sw.AttachTicker(tree)
	tree.Layout(core.Size{Width: 100, Height: 40})
	// off: thumb near left
	// get track padding via IndicatorNode
	track := sw.IndicatorNode().(*primitive.Decorated)
	if track.Padding.Left > 5 {
		t.Fatalf("off pad left=%v want ~2", track.Padding.Left)
	}
	sw.SetChecked(true)
	// animate: several ticks
	for i := 0; i < 20; i++ {
		tree.TickActive(0.02)
	}
	if track.Padding.Left < 15 {
		t.Fatalf("after ticks pad left=%v want ~24 (on)", track.Padding.Left)
	}
}
