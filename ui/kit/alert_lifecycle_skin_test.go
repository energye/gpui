package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: type/variant/icon order keeps alert chrome.
func TestAlert_Lifecycle_SetterOrder(t *testing.T) {
	a := kit.NewAlert("Title")
	a.SetType(kit.AlertSuccess)
	a.SetVariant(kit.AlertFilled)
	a.SetShowIcon(true)
	a.SetDescription("details")
	root := a.ChromeNode().(*primitive.Decorated)
	if root.SkinType != kit.TypeAlert {
		t.Fatalf("SkinType=%q want %s", root.SkinType, kit.TypeAlert)
	}
	if !a.HasDescription() {
		t.Fatal("expected description")
	}
	if !a.IconVisible() {
		t.Fatal("expected icon")
	}
}

// #6: default skin registers kit.Alert; Override painter runs.
func TestAlert_SkinType_UsesKitAlertPainter(t *testing.T) {
	a := kit.NewAlert("Skin")
	a.SetType(kit.AlertInfo)
	th := skindefault.Theme()
	if th.Painter(kit.TypeAlert) == nil {
		t.Fatal("default skin missing kit.Alert painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeAlert, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
			primitive.PaintDecorated(pc, d)
		}
	})
	a.SetTheme(th)
	tree := core.NewTree(a.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 400, Height: 80})
	dc := render.NewContext(400, 80)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	a.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Alert painter not invoked")
	}
}

func TestAlert_EnsureBuilt_Node(t *testing.T) {
	a := kit.NewAlert("x")
	if a.Node() == nil || a.ChromeNode() == nil {
		t.Fatal("not built")
	}
}
