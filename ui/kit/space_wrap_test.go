package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestSpace_WrapGeometry(t *testing.T) {
	mk := func() core.Node {
		b := primitive.NewBox()
		b.Width, b.Height = 40, 20
		return b
	}
	sp := kit.NewSpace(mk(), mk(), mk())
	sp.SetSizePx(8)
	sp.SetWrap(true)
	n := sp.Node()
	sz := n.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
	kids := n.Base().Children()
	if len(kids) != 3 {
		t.Fatalf("kids %d", len(kids))
	}
	if kids[2].Base().Offset().Y < 27.5 {
		t.Fatalf("third child should be on line 2, y=%v", kids[2].Base().Offset().Y)
	}
}

func TestFlex_WrapGeometry(t *testing.T) {
	mk := func() core.Node {
		b := primitive.NewBox()
		b.Width, b.Height = 40, 20
		return b
	}
	f := kit.NewFlex(mk(), mk(), mk())
	f.SetGap(8)
	f.SetWrap(true)
	n := f.Node()
	sz := n.Layout(core.Constraints{MaxWidth: 100, MaxHeight: 400})
	if sz.Height < 47.5 || sz.Height > 48.5 {
		t.Fatalf("height got %v want 48", sz.Height)
	}
}
