package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: chrome after structure (button mode toggle) stays consistent.
func TestRadio_Lifecycle_SetterOrder(t *testing.T) {
	r := kit.NewRadio("Option A")
	r.SetChecked(true)
	r.SetDisabled(false)
	r.SetTextColor(render.Hex("#1677FF"))
	dot := r.IndicatorNode().(*primitive.Decorated)
	if dot.SkinType != kit.TypeRadio {
		t.Fatalf("SkinType=%q want %s", dot.SkinType, kit.TypeRadio)
	}
	// structure: switch to button mode then check
	r.SetButtonMode(true)
	btn := r.IndicatorNode().(*primitive.Decorated)
	if btn.SkinType != kit.TypeRadio {
		t.Fatalf("button SkinType=%q", btn.SkinType)
	}
	r.SetChecked(true)
	if !r.Checked {
		t.Fatal("checked lost after button mode")
	}
}

// #6: default skin registers kit.Radio; Override painter runs.
func TestRadio_SkinType_UsesKitRadioPainter(t *testing.T) {
	r := kit.NewRadio("Skin")
	r.SetChecked(true)
	th := skindefault.Theme()
	if th.Painter(kit.TypeRadio) == nil {
		t.Fatal("default skin missing kit.Radio painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeRadio, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
			primitive.PaintDecorated(pc, d)
		}
	})
	r.SetTheme(th)
	r.SetChecked(true)
	tree := core.NewTree(r.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 200, Height: 40})
	dc := render.NewContext(200, 40)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	r.IndicatorNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Radio painter not invoked")
	}
}

func TestRadio_EnsureBuilt_Aria(t *testing.T) {
	r := kit.NewRadio("x")
	r.SetAriaLabel("plan free")
	if r.Root == nil || r.Root.Base().Label != "plan free" {
		t.Fatalf("aria not applied")
	}
}
