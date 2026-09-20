package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestModalHost_StaticCallEatsTheme(t *testing.T) {
	base := kit.DefaultScopeCtx()
	host := kit.BuildModalHost(base, kit.DefaultModalHostProps())
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	host.Update(skinCtx, kit.DefaultModalHostProps())
	got := kit.ResolveModalHost(skinCtx.Theme)
	if got.PanelBg != skin.ColorBgElevated {
		t.Fatal("panel must follow reskinned theme without code change")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
	okBg := kit.ModalHostOkBg(skin, kit.ScopeStatePressed)
	if okBg != skin.ColorPrimaryActive {
		t.Fatal("pressed ok must use active token")
	}
	dis := kit.ModalHostOkBg(skin, kit.ScopeStateDisabled|kit.ScopeStatePressed)
	if dis != skin.ColorBgContainerDisabled {
		t.Fatal("disabled must win over pressed")
	}
}
