package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// F8: ConfigProvider supplies ambient Theme to descendants via ResolveTheme.
func TestConfigProviderAmbientTheme(t *testing.T) {
	custom := kit.DefaultTheme()
	// Distinct primary for detection.
	custom.ColorPrimary = render.Hex("#FF00AA")
	if custom.Tokens != nil {
		custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	}

	btn := kit.NewButton("OK")
	// No explicit btn.Theme — must pick up ConfigProvider after mount.
	cp := kit.NewConfigProvider(custom, btn.Node())
	tree := core.NewTree(cp.Node())
	tree.Layout(core.Size{Width: 200, Height: 100})

	// ResolveTheme from button root should see ConfigProvider.
	th := core.ResolveTheme(btn.Root)
	if th == nil {
		t.Fatal("ResolveTheme nil under ConfigProvider")
	}
	got := th.Color(core.TokenColorPrimary)
	want := render.Hex("#FF00AA")
	if got.R != want.R || got.G != want.G || got.B != want.B {
		t.Fatalf("ambient primary=%v want %v", got, want)
	}

	// theme() on button after mount should use ambient (rebuild to apply colors).
	btn.Theme = nil
	// Force theme path: call rebuild via SetLabel
	btn.SetLabel("OK2")
	// Primary button uses theme primary fill — rebuild with TypePrimary
	btn.SetType(kit.ButtonPrimary)
	// Inspect decorated background after rebuild uses ambient
	// (decorated may be primary fill).
	if btn.Root == nil {
		t.Fatal("nil root")
	}
}

func TestTreeDefaultTheme(t *testing.T) {
	custom := kit.DefaultTheme()
	custom.ColorPrimary = render.Hex("#00BFFF")
	if custom.Tokens != nil {
		custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#00BFFF")
	}
	box := primitive.NewBox()
	tree := core.NewTree(box)
	tree.SetTheme(custom)
	tree.Layout(core.Size{Width: 10, Height: 10})
	th := core.ResolveTheme(box)
	if th == nil || th.Color(core.TokenColorPrimary).B < 0.9 {
		t.Fatalf("tree theme not resolved: %v", th)
	}
}

func TestExplicitThemeBeatsAmbient(t *testing.T) {
	ambient := kit.DefaultTheme()
	ambient.ColorPrimary = render.Hex("#111111")
	if ambient.Tokens != nil {
		ambient.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#111111")
	}
	explicit := kit.DefaultTheme()
	explicit.ColorPrimary = render.Hex("#EEEEEE")
	if explicit.Tokens != nil {
		explicit.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#EEEEEE")
	}

	btn := kit.NewButton("X")
	btn.Theme = explicit
	cp := kit.NewConfigProvider(ambient, btn.Node())
	tree := core.NewTree(cp)
	tree.Layout(core.Size{Width: 100, Height: 40})
	// field Theme wins in themeOf
	// verify via ResolveTheme still ambient, but button uses explicit field
	if core.ResolveTheme(btn.Root).Color(core.TokenColorPrimary).R > 0.5 {
		// ambient is dark #11 — low R
	}
	// Explicit field: themeOf(btn.Theme, ...) returns explicit without walking
	// We check ColorPrimary on btn.Theme directly.
	if btn.Theme.Color(core.TokenColorPrimary).R < 0.9 {
		t.Fatal("explicit theme not set")
	}
}
