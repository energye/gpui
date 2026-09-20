package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestColorModel_StaticCallEatsTheme(t *testing.T) {
	cases := loadColorCases(t)
	sw := cases["swatch"].(map[string]any)
	if kit.ColorModelSwatchSize(kit.ColorModelSizeSmall) != sw["small"].(float64) {
		t.Fatal("small swatch must be 16")
	}
	if kit.ColorModelSwatchSize(kit.ColorModelSizeMedium) != sw["medium"].(float64) {
		t.Fatal("medium swatch must be 24")
	}
	if kit.ColorModelSwatchSize(kit.ColorModelSizeLarge) != sw["large"].(float64) {
		t.Fatal("large swatch must be 32")
	}
	base := kit.DefaultScopeCtx()
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	props := kit.DefaultColorModelProps()
	props.ShowText = true
	host := kit.BuildColorModel(base, props)
	host.Update(skinCtx, props)
	got := kit.ResolveColorModel(skinCtx.Theme)
	if got.PanelBg != skin.ColorBgElevated {
		t.Fatal("panel must follow reskinned theme without code change")
	}
	if got.Selected != skin.ColorPrimary {
		t.Fatal("selected must follow reskinned primary")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
	// Subtree disable ORs with widget disable.
	disCtx := base
	disCtx.Disabled = true
	dhost := kit.BuildColorModel(disCtx, kit.DefaultColorModelProps())
	if !dhost.Disabled() {
		t.Fatal("subtree disabled must disable the model")
	}
	// Cleared swatch renders transparent.
	r, g, b, a := kit.ColorModelSwatch(host)
	if a != 0 || r != 0 || g != 0 || b != 0 {
		t.Fatalf("cleared swatch = %v,%v,%v,%v want transparent", r, g, b, a)
	}
	host.SetHSB(215, 0.91, 1, 1)
	r, g, b, a = kit.ColorModelSwatch(host)
	if a != 1 {
		t.Fatal("set swatch must be opaque")
	}
	if host.DisplayText() == "" {
		t.Fatal("showText must render hex by default")
	}
}
