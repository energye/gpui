package kit

import (
	"testing"

	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/theme"
)

func TestIcon_PRD_Theme(t *testing.T) {
	seed := theme.DefaultTokens()
	// ICO-16 default color follows theme text, never a literal.
	got := ResolveIcon(seed, DefaultIconProps("check"))
	if got.Main != seed.ColorText {
		t.Fatalf("default color must be theme colorText")
	}
	if got.Size != DefaultIconSize {
		t.Fatalf("default size=%v want 16", got.Size)
	}
	// ICO-08 explicit color wins.
	cp := DefaultIconProps("check")
	cp.Color, cp.ColorSet = "#ff0000", true
	rc := ResolveIcon(seed, cp)
	if rc.Main.A <= 0 || rc.Main.R < 0.9 {
		t.Fatalf("explicit color not honored: %+v", rc.Main)
	}
	// ICO-17 disabled maps to disabled token.
	dp := DefaultIconProps("check")
	dp.Disabled = true
	rd := ResolveIcon(seed, dp)
	if rd.Main != seed.ColorTextDisabled {
		t.Fatalf("disabled color must be theme disabled token")
	}
	// ICO-11 two-tone primary plus secondary participate.
	tp := DefaultIconProps("smile")
	tp.TwoToneSet, tp.TwoTonePrimary, tp.TwoToneSecondary, tp.HasSecondary = true, "#1677ff", "#e6f4ff", true
	rt := ResolveIcon(seed, tp)
	if !rt.TwoTone || !rt.HasSecond || rt.Main.A <= 0 || rt.Secondary.A <= 0 {
		t.Fatalf("two-tone must carry both colors: %+v", rt)
	}
	// Theme switch flows through Ctx.
	skin := seed
	skin.ColorText = theme.Hex("#722ed1")
	ctx := scope.DefaultCtx().WithTheme(skin)
	rs := ResolveIconCtx(ctx, DefaultIconProps("check"))
	if rs.Main != skin.ColorText {
		t.Fatalf("reskin must flow to icon color")
	}
}
