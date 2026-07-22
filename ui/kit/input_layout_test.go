package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func TestInputPlaceholderInsideField(t *testing.T) {
	in := kit.NewInput("Type here")
	in.SetFixedSize(360, 32)
	root := in.Node()
	sz := root.Layout(core.Loose(800, 600))
	if sz.Width < 300 || sz.Height < 28 {
		t.Fatalf("input size=%v want ~360×32", sz)
	}
	ed := in.Editor()
	if ed == nil {
		t.Fatal("nil editor")
	}
	// After layout, editor should occupy most of the field (not a sliver on the right).
	if ed.Size().Width < 200 {
		t.Fatalf("editor width=%v too small — empty prefix Slot likely ate the row", ed.Size().Width)
	}
	if ed.Size().Width > sz.Width {
		t.Fatalf("editor width %v > input %v", ed.Size().Width, sz.Width)
	}
}

func TestSelectShowsPlaceholderText(t *testing.T) {
	s := kit.NewSelect("Please select",
		kit.SelectOption{Value: "1", Label: "One"},
	)
	root := s.Node()
	sz := root.Layout(core.Loose(400, 200))
	if sz.Width < 100 || sz.Height < 24 {
		t.Fatalf("select size=%v", sz)
	}
	// Walk for Text with placeholder
	var found bool
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		// primitive.Text via TypeID
		if n.TypeID() == "primitive.Text" {
			found = true
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	if !found {
		t.Fatal("select has no Text label node")
	}
}
