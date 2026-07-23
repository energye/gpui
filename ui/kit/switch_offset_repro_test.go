package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Verifies thumb *layout Offset* moves when toggled (not only Padding field).
func TestSwitchThumbOffsetMoves(t *testing.T) {
	sw := kit.NewSwitch()
	root := primitive.NewBox(sw.Node())
	root.Width, root.Height = 100, 40
	tree := core.NewTree(root)
	sw.AttachTicker(tree)
	tree.Layout(core.Size{Width: 100, Height: 40})
	track := sw.IndicatorNode().(*primitive.Decorated)
	if len(track.Children()) < 1 {
		t.Fatal("no thumb")
	}
	thumb := track.Children()[0]
	off0 := thumb.Base().Offset().X
	t.Logf("off pad=%v offsetX=%v", track.Padding.Left, off0)
	sw.SetChecked(true)
	for i := 0; i < 20; i++ {
		tree.TickActive(0.02)
	}
	off1 := thumb.Base().Offset().X
	t.Logf("on pad=%v offsetX=%v needsLayout=%v", track.Padding.Left, off1, track.Base().NeedsLayout())
	if track.Padding.Left < 15 {
		t.Fatalf("pad not updated: %v", track.Padding.Left)
	}
	if off1 <= off0+5 {
		t.Fatalf("thumb Offset did not move: %v → %v (LayoutSkipIfClean bug?)", off0, off1)
	}
}
