package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: type/shape/icon order keeps FAB chrome.
func TestFloatButton_Lifecycle_SetterOrder(t *testing.T) {
	f := kit.NewFloatButton()
	f.SetIcon("plus")
	f.SetType(kit.ButtonPrimary)
	f.SetShape(kit.FloatButtonSquare)
	f.SetAriaLabel("add")
	dec := f.ChromeNode().(*primitive.Decorated)
	if dec.SkinType != kit.TypeFloatButton {
		t.Fatalf("SkinType=%q want %s", dec.SkinType, kit.TypeFloatButton)
	}
	if dec.Width <= 0 || dec.Height <= 0 {
		t.Fatalf("metrics empty %vx%v", dec.Width, dec.Height)
	}
}

// #6: default skin registers kit.FloatButton; Override painter runs.
func TestFloatButton_SkinType_UsesKitFloatButtonPainter(t *testing.T) {
	f := kit.NewFloatButton()
	f.SetType(kit.ButtonPrimary)
	th := skindefault.Theme()
	if th.Painter(kit.TypeFloatButton) == nil {
		t.Fatal("default skin missing kit.FloatButton painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeFloatButton, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#FA541C")
			primitive.PaintDecorated(pc, d)
		}
	})
	f.SetTheme(th)
	tree := core.NewTree(f.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 80, Height: 80})
	dc := render.NewContext(80, 80)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	f.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.FloatButton painter not invoked")
	}
}

func TestFloatButton_EnsureBuilt_Aria(t *testing.T) {
	f := kit.NewFloatButton()
	f.SetAriaLabel("compose")
	if f.Button() == nil || f.Button().Root == nil {
		t.Fatal("button not built")
	}
}
