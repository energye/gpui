package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestQRCode_StaticCallEatsTheme(t *testing.T) {
	base := kit.DefaultScopeCtx()
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	props := kit.DefaultQRCodeProps()
	props.Value = "https://ant.design"
	host := kit.BuildQRCode(base, props)
	host.Update(skinCtx, props)
	got := kit.ResolveQRCode(skinCtx.Theme, "", "")
	if got.Module != skin.ColorText {
		t.Fatal("module must default to colorText, never a hardcoded brand")
	}
	if got.Border != skin.ColorBorderSecondary {
		t.Fatal("border must follow colorSplit token")
	}
	if got.CoverBg.A < 0.9 {
		t.Fatal("cover must stay near-opaque")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
	// Explicit color/bgColor override the tokens.
	over := kit.ResolveQRCode(skin, "#ff0000", "#ffff00")
	if over.Module != theme.Hex("#ff0000") || over.RootBg != theme.Hex("#ffff00") {
		t.Fatalf("override = %+v on %+v", over.Module, over.RootBg)
	}
}
