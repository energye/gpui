package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestSegmented_SetValueKeepsChildren(t *testing.T) {
	s := kit.NewSegmented("a", "b", "c")
	root := s.Node()
	tree := core.NewTree(root)
	tree.Layout(core.Size{Width: 400, Height: 40})
	before := root.Base().Children()
	if len(before) != 1 {
		t.Fatalf("children=%d", len(before))
	}
	row0 := before[0]
	s.SetValue("b")
	after := root.Base().Children()
	if len(after) != 1 || after[0] != row0 {
		t.Fatalf("SetValue rebuilt option tree (must only recolor)")
	}
	if s.Value != "b" {
		t.Fatal(s.Value)
	}
}

func TestFlex_JustifyLiveRelayout(t *testing.T) {
	a := primitive.NewDecorated(nil)
	a.Width, a.Height = 40, 20
	b := primitive.NewDecorated(nil)
	b.Width, b.Height = 40, 20
	f := kit.NewFlex(a, b)
	f.SetJustify(kit.FlexJustifyStart)
	play := primitive.NewDecorated(f.Node())
	play.Width, play.Height = 200, 40
	play.StretchChild = true
	tree := core.NewTree(play)
	tree.Layout(core.Size{Width: 200, Height: 40})
	x0 := a.Base().Offset().X
	f.SetJustify(kit.FlexJustifySpaceBetween)
	// host must re-layout after MarkNeedsLayout from apply
	tree.Layout(core.Size{Width: 200, Height: 40})
	x1 := a.Base().Offset().X
	x2 := b.Base().Offset().X
	if x2 < 150 {
		t.Fatalf("space-between second x=%v want ~160 (first was %v→%v)", x2, x0, x1)
	}
}
