package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// Verifies thumb *layout Offset* moves when toggled via Stack PositionedAt.
func TestSwitchThumbOffsetMoves(t *testing.T) {
	sw := kit.NewSwitch()
	root := primitive.NewBox(sw.Node())
	root.Width, root.Height = 100, 40
	tree := core.NewTree(root)
	sw.AttachTicker(tree)
	tree.Layout(core.Size{Width: 100, Height: 40})
	thumb := sw.ThumbNode()
	if thumb == nil {
		t.Fatal("nil thumb")
	}
	host := thumb.Base().Parent()
	if host == nil {
		t.Fatal("no thumb host")
	}
	off0 := host.Base().Offset().X
	t.Logf("off offsetX=%v", off0)
	sw.SetChecked(true)
	for i := 0; i < 20; i++ {
		tree.TickActive(0.02)
	}
	off1 := host.Base().Offset().X
	t.Logf("on offsetX=%v", off1)
	if off1 <= off0+5 {
		t.Fatalf("thumb Offset did not move: %v → %v", off0, off1)
	}
}
