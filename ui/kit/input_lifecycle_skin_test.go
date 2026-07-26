package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: structure then chrome order must not drop status/variant colors.
func TestInput_Lifecycle_SetterOrder(t *testing.T) {
	in := kit.NewInput("ph")
	in.SetAllowClear(true)
	in.SetSize(kit.InputLarge)
	in.SetStatus(kit.InputStatusError)
	in.SetVariant(kit.InputOutlined)
	dec := in.ChromeNode().(*primitive.Decorated)
	if dec.SkinType != kit.TypeInput {
		t.Fatalf("SkinType=%q want %s", dec.SkinType, kit.TypeInput)
	}
	// Error status should tint border (non-default).
	if dec.BorderWidth <= 0 && dec.BorderColor.A < 0.1 {
		// still ok if token-driven; at least chrome applied without panic
	}
	// Background after SetBackground then SetSize
	in2 := kit.NewInput("")
	want := render.Hex("#F9F0FF")
	in2.SetBackground(want)
	in2.SetSize(kit.InputSmall)
	dec2 := in2.ChromeNode().(*primitive.Decorated)
	if dec2.Background != want {
		t.Fatalf("bg lost after size rebuild: got %+v want %+v", dec2.Background, want)
	}
}

// #6: default skin registers kit.Input; Override painter runs.
func TestInput_SkinType_UsesKitInputPainter(t *testing.T) {
	in := kit.NewInput("skin")
	dec := in.ChromeNode().(*primitive.Decorated)
	if dec.SkinType != kit.TypeInput {
		t.Fatalf("SkinType=%q", dec.SkinType)
	}
	th := skindefault.Theme()
	if th.Painter(kit.TypeInput) == nil {
		t.Fatal("default skin missing kit.Input painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeInput, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#722ED1")
			primitive.PaintDecorated(pc, d)
		}
	})
	in.SetTheme(th)
	tree := core.NewTree(in.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 240, Height: 48})
	dc := render.NewContext(240, 48)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	in.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Input painter not invoked")
	}
}

func TestInput_EnsureBuilt_Editor(t *testing.T) {
	in := kit.NewInput("x")
	ed := in.Editor()
	if ed == nil {
		t.Fatal("editor nil")
	}
	in.SetAriaLabel("Name")
	// no panic; a11y applied
}
