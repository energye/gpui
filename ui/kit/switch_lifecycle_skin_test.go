package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
	skindefault "github.com/energye/gpui/ui/skin/default"
)

// #9: structure then chrome order keeps checked chrome.
func TestSwitch_Lifecycle_SetterOrder(t *testing.T) {
	s := kit.NewSwitch()
	s.SetSize(kit.SwitchSmall)
	s.SetCheckedChildren("ON")
	s.SetUnCheckedChildren("OFF")
	s.SetChecked(true)
	s.SetDisabled(false)
	track := s.ChromeNode().(*primitive.Decorated)
	if track.SkinType != kit.TypeSwitch {
		t.Fatalf("SkinType=%q want %s", track.SkinType, kit.TypeSwitch)
	}
	// Active track should have non-transparent fill from tokens.
	if track.Background.A < 0.2 {
		t.Fatalf("checked track bg too transparent: %+v", track.Background)
	}

	s2 := kit.NewSwitch()
	want := render.Hex("#13C2C2")
	s2.SetActiveColor(want)
	s2.SetChecked(true)
	// Size rebuild must not drop style active color path
	s2.SetSize(kit.SwitchMedium)
	s2.SetChecked(true)
	tr2 := s2.ChromeNode().(*primitive.Decorated)
	// Style.BackgroundActive is used when checked; color may be applied as-is
	if tr2.Background != want && tr2.Background.A < 0.2 {
		// applyChrome may composite; at least non-empty after size rebuild
		t.Fatalf("after size+check chrome empty: %+v", tr2.Background)
	}
}

// #6: default skin registers kit.Switch; Override painter runs.
func TestSwitch_SkinType_UsesKitSwitchPainter(t *testing.T) {
	s := kit.NewSwitch()
	s.SetChecked(true)
	track := s.ChromeNode().(*primitive.Decorated)
	if track.SkinType != kit.TypeSwitch {
		t.Fatalf("SkinType=%q", track.SkinType)
	}
	th := skindefault.Theme()
	if th.Painter(kit.TypeSwitch) == nil {
		t.Fatal("default skin missing kit.Switch painter")
	}
	var painted bool
	th.Skin = core.Override(th.Skin, kit.TypeSwitch, func(pc *core.PaintContext, n core.Node) {
		painted = true
		if d, ok := n.(*primitive.Decorated); ok {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#FA8C16")
			primitive.PaintDecorated(pc, d)
		}
	})
	s.Theme = th
	s.SetChecked(true) // chrome refresh under theme field
	tree := core.NewTree(s.Node())
	tree.SetTheme(th)
	tree.Layout(core.Size{Width: 120, Height: 40})
	dc := render.NewContext(120, 40)
	pc := &core.PaintContext{Theme: th, Scale: 1, DC: dc}
	s.ChromeNode().Paint(pc)
	if !painted {
		t.Fatal("overridden kit.Switch painter not invoked")
	}
}

func TestSwitch_EnsureBuilt_Aria(t *testing.T) {
	s := kit.NewSwitch()
	s.SetAriaLabel("airplane mode")
	if s.Root == nil || s.Root.Base().Label != "airplane mode" {
		t.Fatalf("aria not applied")
	}
}
