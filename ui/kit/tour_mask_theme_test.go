package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestTourMask_StaticCallEatsTheme(t *testing.T) {
	base := kit.DefaultScopeCtx()
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	props := kit.DefaultTourMaskProps()
	props.Open = true
	host := kit.BuildTourMask(base, props, tourSteps())
	host.Update(skinCtx, props)
	got := kit.ResolveTourMask(skinCtx.Theme)
	if got.PanelBg != skin.ColorBgElevated {
		t.Fatal("panel must follow reskinned theme without code change")
	}
	if got.HoleBorder != skin.ColorPrimary {
		t.Fatal("hole border must follow primary token")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
	// Primary type flips panel to primary with inverse text.
	phost := kit.BuildTourMask(skinCtx, kit.DefaultTourMaskProps(), []kit.TourMaskStep{
		{Title: "t", Description: "d", Type: kit.TourMaskTypePrimary, TypeSet: true},
	})
	phost.SetOpen(true)
	if bg := kit.TourMaskPanelBg(skinCtx.Theme, phost); bg != skin.ColorPrimary {
		t.Fatal("primary panel must use primary token")
	}
	if tx := kit.TourMaskPanelText(skinCtx.Theme, phost); tx != theme.Hex("#ffffff") {
		t.Fatal("primary text must invert to white")
	}
	// Mask color override wins over seed.
	mprops := kit.DefaultTourMaskProps()
	mprops.MaskColorSet = true
	mprops.MaskColorR, mprops.MaskColorG, mprops.MaskColorB, mprops.MaskColorA = 255, 0, 0, 0.2
	if mc := kit.TourMaskMaskColor(skin, mprops); mc.R != 1 || mc.A != 0.2 {
		t.Fatalf("mask override = %+v want red 0.2", mc)
	}
}
