package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func TestButton_SetBlock_ExpandWidth(t *testing.T) {
	b := kit.NewButton("Block")
	b.SetType(kit.ButtonPrimary)
	b.SetBlock(true)
	// Parent column with bounded width (like CrossStretch content area).
	col := primitive.Column(b.Node())
	col.CrossAlign = core.CrossStretch
	sz := col.Layout(core.Constraints{MaxWidth: 320, MaxHeight: 200, MinWidth: 320})
	if sz.Width < 319.5 {
		t.Fatalf("col width %v", sz.Width)
	}
	btnSz := b.Node().Base().Size()
	if btnSz.Width < 319.5 {
		t.Fatalf("block button width %v want ~320", btnSz.Width)
	}
}
