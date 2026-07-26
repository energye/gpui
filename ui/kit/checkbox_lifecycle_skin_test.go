package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: chrome setters after structure stay consistent.
func TestCheckbox_Lifecycle_SetterOrder(t *testing.T) {
	c := kit.NewCheckbox("Agree")
	c.SetIndeterminate(true)
	c.SetChecked(true) // clears indeterminate
	c.SetDisabled(false)
	c.SetTextColor(render.Hex("#1677FF"))
	box := c.IndicatorNode().(*primitive.Decorated)
	if box.SkinType != kit.TypeCheckbox {
		t.Fatalf("SkinType=%q want %s", box.SkinType, kit.TypeCheckbox)
	}
	if !c.Checked || c.Indeterminate {
		t.Fatalf("checked=%v ind=%v", c.Checked, c.Indeterminate)
	}
}

// #6: default skin registers kit.Checkbox; Override painter runs on indicator.
func TestCheckbox_SkinType_UsesKitCheckboxPainter(t *testing.T) {
	c := kit.NewCheckbox("Skin")
	c.SetChecked(true)
	box := c.IndicatorNode().(*primitive.Decorated)
	if box.SkinType != kit.TypeCheckbox {
		t.Fatalf("SkinType=%q", box.SkinType)
	}
	th := skindefault.Theme()
	if th.Painter(kit.TypeCheckbox) == nil {
		t.Fatal("default skin missing kit.Checkbox painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeCheckbox, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#52C41A")
			primitive.PaintDecorated(pc, d)
		}
	})
	c.SetTheme(th)
	c.SetChecked(true)
	tree := core.NewTree(c.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 200, Height: 40})
	dc := render.NewContext(200, 40)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	c.IndicatorNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Checkbox painter not invoked")
	}
}

func TestCheckbox_EnsureBuilt_Aria(t *testing.T) {
	c := kit.NewCheckbox("x")
	c.SetAriaLabel("accept terms")
	if c.Root == nil || c.Root.Base().Label != "accept terms" {
		t.Fatalf("aria not applied: %+v", c.Root)
	}
}
