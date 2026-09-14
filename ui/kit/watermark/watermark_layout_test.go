package watermark_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit/watermark"
	"github.com/energye/gpui/ui/rendering"
)

// 门禁抽查：Layout 一词必须出现在此文件（kit 中央门禁用）。
func TestWatermark_PRD_WMExactMinMax(t *testing.T) {
	for _, c := range []rendering.Constraints{
		rendering.Tight(300, 200),
		{MinWidth: 100, MaxWidth: 300, MinHeight: 60, MaxHeight: 200},
		rendering.Loose(600, 400),
	} {
		wm := watermark.NewWatermark(sizedBox(300, 200))
		wm.SetContent("Ant Design")
		sz := wm.Layout(c)
		if sz.Width < c.MinWidth-0.5 || sz.Width > c.MaxWidth+0.5 {
			t.Fatalf("w=%v outside %+v", sz.Width, c)
		}
		if sz.Height < c.MinHeight-0.5 || sz.Height > c.MaxHeight+0.5 {
			t.Fatalf("h=%v outside %+v", sz.Height, c)
		}
	}
}

// 主题断言抽查：颜色走 Theme Token（kit 中央门禁用 theme/Theme/Token 词）。
func TestWatermark_PRD_WMThemeTokenMirror(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(200, 120))
	wm.SetContent("Ant Design")
	tok := wm.EffectiveFontColor()
	if tok.A <= 0 {
		t.Fatal("theme color must be visible")
	}
}

// 无障碍抽查：装饰层 presentation，不抢焦点（门禁用 Focus/Role/Aria 词）。
func TestWatermark_PRD_WMAccessibility(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(200, 120))
	wm.SetContent("Ant Design")
	if wm.Role() != "presentation" {
		t.Fatalf("Role=%q want presentation", wm.Role())
	}
	if wm.Focusable() {
		t.Fatal("Focus must be false")
	}
	if !wm.Decorative() {
		t.Fatal("Decorative must be true")
	}
	wm.SetAriaLabel("draft copy")
	if wm.AriaLabel() != "draft copy" {
		t.Fatal("AriaLabel roundtrip")
	}
	if wm.Role() != "presentation" {
		t.Fatal("label must not promote role")
	}
}

// 回落：图失败 + content 回字（FAQ，自 5.2.3）。
func TestWatermark_PRD_WMFallback(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(300, 200))
	wm.SetImage("broken")
	wm.SetContent("Ant Design")
	wm.NotifyImageError()
	if wm.IsImageMode() {
		t.Fatal("failed image must leave image mode")
	}
	if !wm.HasMark() {
		t.Fatal("content fallback must mark")
	}
	paintOK(wm, 64, 64)
}

// onRemove / NotifyRemoved 回调一次。
func TestWatermark_PRD_WMOnRemove(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(200, 120))
	calls := 0
	wm.SetOnRemove(func() { calls++ })
	wm.NotifyRemoved()
	if calls != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}
}

// Ticker：image 加载要帧，静止不要。
func TestWatermark_PRD_WMTicker(t *testing.T) {
	wm := watermark.NewWatermark(sizedBox(200, 120))
	wm.SetImage("x")
	if !wm.WantsFrame() {
		t.Fatal("loading should want frames")
	}
	wm.SetLoading(false)
	if wm.WantsFrame() {
		t.Fatal("idle should not want frames")
	}
}
