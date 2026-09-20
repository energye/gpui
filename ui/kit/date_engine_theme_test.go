package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestDateEngine_LocaleEatsCtx(t *testing.T) {
	cases := loadDateCases(t)
	loc := cases["locale"].(map[string]any)
	en := kit.ResolveDateEngineLocale("en-US")
	if en.Placeholder != loc["placeholder_en"].(string) {
		t.Fatalf("en placeholder = %q", en.Placeholder)
	}
	zh := kit.ResolveDateEngineLocale("zh-CN")
	if zh.Placeholder != loc["placeholder_zh"].(string) {
		t.Fatalf("zh placeholder = %q", zh.Placeholder)
	}
	if got := en.PanelTitle(kit.DateEngineModeDate, 2024, 2); got != loc["title_en_feb2024"].(string) {
		t.Fatalf("en title = %q", got)
	}
	// Week header rotates with weekStart.
	if h := en.WeekHeader(1); h[0] != "Mon" || h[6] != "Sun" {
		t.Fatalf("mon-first header = %v", h)
	}
	if h := zh.WeekHeader(1); h[0] != "一" {
		t.Fatalf("zh header = %v", h)
	}
	// Unknown locale never renders empty: falls back to en.
	if unk := kit.ResolveDateEngineLocale("xx-YY"); unk.Placeholder == "" || unk.Today == "" {
		t.Fatal("unknown locale must fall back to en copy")
	}
	// Static call eats Ctx locale and reskinned seed.
	base := kit.DefaultScopeCtx()
	host := kit.BuildDateEngine(base.WithLocale("zh-CN"), kit.DefaultDateEngineProps())
	if host.Locale() != "zh-CN" {
		t.Fatalf("locale = %q must follow Ctx", host.Locale())
	}
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	got := kit.ResolveDateEngine(skin)
	if got.Selected != skin.ColorPrimary {
		t.Fatal("selected day must follow reskinned primary without code change")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
}
