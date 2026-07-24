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
	// Prefer ThumbNode host offset (Stack PositionedAt).
	thumb := sw.ThumbNode()
	if thumb == nil {
		t.Fatal("nil thumb")
	}
	// Host is parent of thumb.
	host := thumb.Base().Parent()
	if host == nil {
		t.Fatal("no thumb host")
	}
	off0 := host.Base().Offset().X
	if off0 > 5 {
		t.Fatalf("off offsetX=%v want ~2", off0)
	}
	sw.SetChecked(true)
	for i := 0; i < 20; i++ {
		tree.TickActive(0.02)
	}
	off1 := host.Base().Offset().X
	if off1 < 15 {
		t.Fatalf("after ticks offsetX=%v want ~24 (on)", off1)
	}
}
