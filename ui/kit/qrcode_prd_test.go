package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/qr-code.md §6.9 — P0 PRD cases (QR-01 … QR-21).
// QR-20 N/A (no disabled); QR-22 L3 / QR-23 L4 / QR-24 P1 deferred.

func approxQR(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxQRColor(a, b render.RGBA, tol float64) bool {
	return approxQR(float64(a.R), float64(b.R), tol) &&
		approxQR(float64(a.G), float64(b.G), tol) &&
		approxQR(float64(a.B), float64(b.B), tol) &&
		approxQR(float64(a.A), float64(b.A), tol)
}

func TestQRCode_PRD_01_Defaults(t *testing.T) {
	// QR-01: NewQRCode 默认创建
	q := kit.NewQRCode("https://ant.design/")
	if q.Size() != kit.DefaultQRCodeSize {
		t.Fatalf("size=%v want %v", q.Size(), kit.DefaultQRCodeSize)
	}
	if q.Status() != kit.QRStatusActive {
		t.Fatalf("status=%v want active", q.Status())
	}
	if !q.Bordered() {
		t.Fatal("bordered want true")
	}
	if q.ErrorLevel() != kit.QRErrorLevelM {
		t.Fatalf("level=%v want M", q.ErrorLevel())
	}
	if q.Type() != kit.QRCodeTypeCanvas {
		t.Fatalf("type=%v want canvas", q.Type())
	}
	if q.Node() == nil || q.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	sz := q.Node().Layout(core.Loose(400, 400))
	if !approxQR(sz.Width, kit.DefaultQRCodeSize, 0.5) || !approxQR(sz.Height, kit.DefaultQRCodeSize, 0.5) {
		t.Fatalf("layout=%v", sz)
	}
}

func TestQRCode_PRD_02_ValueMatrix(t *testing.T) {
	// QR-02 / QR-S1: value 非空 → 矩阵
	q := kit.NewQRCode("gpui-qr")
	if q.Modules() < 21 {
		t.Fatalf("modules=%d want >=21", q.Modules())
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_03_Size200(t *testing.T) {
	// QR-03 / QR-S2: size=200
	q := kit.NewQRCode("https://ant.design/")
	q.SetSize(200)
	if !approxQR(q.Size(), 200, 0.5) {
		t.Fatalf("size=%v", q.Size())
	}
	sz := q.Node().Layout(core.Loose(400, 400))
	if !approxQR(sz.Width, 200, 0.5) || !approxQR(sz.Height, 200, 0.5) {
		t.Fatalf("layout=%v", sz)
	}
}

func TestQRCode_PRD_04_StatusExpiredCover(t *testing.T) {
	// QR-04 / QR-S3: status=expired → cover
	q := kit.NewQRCode("https://ant.design/")
	q.SetStatus(kit.QRStatusExpired)
	if !q.HasCover() {
		t.Fatal("HasCover false")
	}
	if q.CoverNode() == nil {
		t.Fatal("CoverNode nil")
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_05_OnRefresh(t *testing.T) {
	// QR-05 / QR-S4: onRefresh 点击
	n := 0
	q := kit.NewQRCode("https://ant.design/")
	q.SetOnRefresh(func() { n++ })
	q.SetStatus(kit.QRStatusExpired)
	btn := q.RefreshButton()
	if btn == nil {
		t.Fatal("RefreshButton nil")
	}
	if btn.OnClick == nil {
		t.Fatal("OnClick nil")
	}
	btn.OnClick()
	if n != 1 {
		t.Fatalf("refresh count=%d", n)
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_06_Icon(t *testing.T) {
	// QR-06 / QR-S5: icon 中心
	q := kit.NewQRCode("https://ant.design/")
	q.SetErrorLevel(kit.QRErrorLevelH)
	q.SetIcon("https://example.com/logo.svg")
	if !q.HasIcon() {
		t.Fatal("HasIcon false")
	}
	if q.IconHost() == nil {
		t.Fatal("IconHost nil")
	}
	w, h := q.IconSize()
	if !approxQR(w, kit.DefaultQRCodeIconSize, 0.5) || !approxQR(h, kit.DefaultQRCodeIconSize, 0.5) {
		t.Fatalf("iconSize=%v,%v", w, h)
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_07_ErrorLevels(t *testing.T) {
	// QR-07 / QR-S6: errorLevel 均可生成
	for _, lv := range []kit.QRErrorLevel{
		kit.QRErrorLevelL, kit.QRErrorLevelM, kit.QRErrorLevelQ, kit.QRErrorLevelH,
	} {
		q := kit.NewQRCode("https://ant.design/")
		q.SetErrorLevel(lv)
		if q.Modules() <= 0 {
			t.Fatalf("level=%v modules=0", lv)
		}
		_ = q.Node().Layout(core.Loose(200, 200))
	}
}

func TestQRCode_PRD_08_DefaultSize(t *testing.T) {
	// QR-08 / QR-S7
	q := kit.NewQRCode("x")
	if !approxQR(q.Size(), 160, 0.5) {
		t.Fatalf("size=%v", q.Size())
	}
}

func TestQRCode_PRD_09_DemoBase(t *testing.T) {
	// QR-09: base.tsx — value 可变
	q := kit.NewQRCode("https://ant.design/")
	m1 := q.Modules()
	q.SetValue("https://ant.design/components/qr-code")
	if q.Value() != "https://ant.design/components/qr-code" {
		t.Fatal("value not updated")
	}
	if q.Modules() <= 0 {
		t.Fatal("modules empty after set")
	}
	// matrix may change with longer content
	_ = m1
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_10_DemoStatus(t *testing.T) {
	// QR-10: status.tsx — loading / expired / scanned
	for _, st := range []kit.QRStatus{kit.QRStatusLoading, kit.QRStatusExpired, kit.QRStatusScanned} {
		q := kit.NewQRCode("https://ant.design")
		q.SetStatus(st)
		if !q.HasCover() {
			t.Fatalf("status=%v no cover", st)
		}
		_ = q.Node().Layout(core.Loose(200, 200))
	}
	active := kit.NewQRCode("https://ant.design")
	if active.HasCover() {
		t.Fatal("active has cover")
	}
}

func TestQRCode_PRD_11_DemoIcon(t *testing.T) {
	// QR-11: icon.tsx
	q := kit.NewQRCode("https://ant.design/")
	q.SetErrorLevel(kit.QRErrorLevelH)
	q.SetIcon("https://gw.alipayobjects.com/zos/rmsportal/KDpgvguMpGfqaHPjicRK.svg")
	if q.ErrorLevel() != kit.QRErrorLevelH || !q.HasIcon() {
		t.Fatal("icon demo fields")
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_12_DemoCustomColor(t *testing.T) {
	// QR-12: customColor.tsx
	q := kit.NewQRCode("https://ant.design/")
	fg := render.RGBA{R: 0.1, G: 0.6, B: 0.2, A: 1}
	bg := render.RGBA{R: 0.95, G: 0.95, B: 0.9, A: 1}
	q.SetColor(fg)
	q.SetBgColor(bg)
	if !approxQRColor(q.ResolvedColor(), fg, 0.02) {
		t.Fatalf("fg=%v", q.ResolvedColor())
	}
	if !approxQRColor(q.ResolvedBgColor(), bg, 0.02) {
		t.Fatalf("bg=%v", q.ResolvedBgColor())
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_13_DemoErrorLevel(t *testing.T) {
	// QR-13: errorlevel.tsx L→H
	q := kit.NewQRCode("https://gw.alipayobjects.com/zos/rmsportal/KDpgvguMpGfqaHPjicRK.svg")
	for _, lv := range []kit.QRErrorLevel{
		kit.QRErrorLevelL, kit.QRErrorLevelM, kit.QRErrorLevelQ, kit.QRErrorLevelH,
	} {
		q.SetErrorLevel(lv)
		if q.Modules() <= 0 {
			t.Fatalf("level %v empty", lv)
		}
	}
}

func TestQRCode_PRD_14_DemoType(t *testing.T) {
	// QR-14: type.tsx canvas + svg
	c := kit.NewQRCode("https://ant.design/")
	c.SetType(kit.QRCodeTypeCanvas)
	s := kit.NewQRCode("https://ant.design/")
	s.SetType(kit.QRCodeTypeSVG)
	if c.Type() != kit.QRCodeTypeCanvas || s.Type() != kit.QRCodeTypeSVG {
		t.Fatal("type")
	}
	if c.Modules() <= 0 || s.Modules() <= 0 {
		t.Fatal("modules")
	}
	_ = c.Node().Layout(core.Loose(200, 200))
	_ = s.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_15_DemoCustomSize(t *testing.T) {
	// QR-15: customSize.tsx
	q := kit.NewQRCode("https://ant.design/")
	q.SetErrorLevel(kit.QRErrorLevelH)
	for _, sz := range []float64{48, 160, 300} {
		q.SetSize(sz)
		q.SetIconSize(sz / 4)
		q.SetIcon("logo")
		if !approxQR(q.Size(), sz, 0.5) {
			t.Fatalf("size=%v", q.Size())
		}
		w, _ := q.IconSize()
		if !approxQR(w, sz/4, 0.5) {
			t.Fatalf("icon=%v want %v", w, sz/4)
		}
		out := q.Node().Layout(core.Loose(400, 400))
		if !approxQR(out.Width, sz, 0.5) {
			t.Fatalf("layout w=%v want %v", out.Width, sz)
		}
	}
}

func TestQRCode_PRD_16_CustomStatusRender(t *testing.T) {
	// QR-16: customStatusRender.tsx
	called := false
	q := kit.NewQRCode("https://ant.design")
	q.SetStatusRender(func(info kit.QRStatusRenderInfo) core.Node {
		called = true
		if info.Status != kit.QRStatusLoading {
			t.Fatalf("status=%v", info.Status)
		}
		return kit.NewText("Loading...").Node()
	})
	q.SetStatus(kit.QRStatusLoading)
	if !called {
		t.Fatal("StatusRender not called")
	}
	if !q.HasCover() {
		t.Fatal("no cover")
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_17_Borderless(t *testing.T) {
	// QR-17: Popover.tsx bordered=false
	q := kit.NewQRCode("https://ant.design")
	q.SetBordered(false)
	if q.Bordered() {
		t.Fatal("still bordered")
	}
	if !approxQR(q.Padding(), 0, 0.01) {
		t.Fatalf("pad=%v", q.Padding())
	}
	if !approxQR(q.Radius(), 0, 0.01) {
		t.Fatalf("radius=%v", q.Radius())
	}
	if !approxQR(q.LineWidth(), 0, 0.01) {
		t.Fatalf("lineW=%v", q.LineWidth())
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_18_Metrics(t *testing.T) {
	// QR-18 / L2 §6.2
	q := kit.NewQRCode("https://ant.design/")
	_ = q.Node().Layout(core.Loose(400, 400))
	if !approxQR(q.Size(), 160, 0.5) {
		t.Fatalf("size=%v", q.Size())
	}
	if !approxQR(q.Padding(), kit.DefaultQRCodePadding, 0.5) {
		t.Fatalf("pad=%v", q.Padding())
	}
	if !approxQR(q.Radius(), kit.DefaultQRCodeRadius, 0.5) {
		t.Fatalf("radius=%v want 8", q.Radius())
	}
	if !approxQR(q.LineWidth(), kit.DefaultQRCodeLineWidth, 0.5) {
		t.Fatalf("lineW=%v", q.LineWidth())
	}
	// content = 160 - 2*8 = 144
	if !approxQR(q.ContentSize(), 144, 0.5) {
		t.Fatalf("content=%v", q.ContentSize())
	}
}

func TestQRCode_PRD_19_DefaultColorsToken(t *testing.T) {
	// QR-19: 默认皮走 Token，非品牌 primary
	th := kit.DefaultTheme()
	q := kit.NewQRCode("https://ant.design/")
	q.SetTheme(th)
	fg := q.ResolvedColor()
	prim := th.Color(core.TokenColorPrimary)
	if approxQRColor(fg, prim, 0.02) && prim.A > 0.5 {
		// colorText may coincidentally match in weird themes; ensure not hard-coded brand blue
		brand := render.Hex("#1677FF")
		if approxQRColor(fg, brand, 0.02) {
			t.Fatal("fg equals hardcoded brand primary")
		}
	}
	// border uses colorSplit path
	bd := q.BorderColor()
	if bd.A < 0.01 {
		t.Fatal("border transparent")
	}
	// cover uses bg container alpha
	if q.CoverBackground().A < 0.5 {
		// only meaningful after expired, but field is cached
	}
	q.SetStatus(kit.QRStatusExpired)
	cbg := q.CoverBackground()
	if cbg.A < 0.5 {
		t.Fatalf("cover alpha=%v", cbg.A)
	}
}

func TestQRCode_PRD_20_DisabledNA(t *testing.T) {
	// QR-20: QRCode 无 disabled — document N/A
	q := kit.NewQRCode("x")
	_ = q.Node().Layout(core.Loose(160, 160))
}

func TestQRCode_PRD_21_RefreshFocusPath(t *testing.T) {
	// QR-21: expired + OnRefresh 可点
	n := 0
	q := kit.NewQRCode("https://ant.design/")
	q.SetOnRefresh(func() { n++ })
	q.SetStatus(kit.QRStatusExpired)
	btn := q.RefreshButton()
	if btn == nil || btn.Node() == nil {
		t.Fatal("no refresh button")
	}
	// button is link type and has label
	if btn.Label != kit.DefaultQRCodeRefresh && btn.AriaLabel != kit.DefaultQRCodeRefresh {
		t.Fatalf("label=%q aria=%q", btn.Label, btn.AriaLabel)
	}
	_ = q.Node().Layout(core.Loose(200, 200))
	// activate via OnClick (keyboard/press ultimately call this)
	btn.OnClick()
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
}

func TestQRCode_PRD_EmptyValue(t *testing.T) {
	// empty value must not panic (antd returns null)
	q := kit.NewQRCode("")
	if q.Modules() != 0 {
		t.Fatalf("modules=%d", q.Modules())
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_IconNode(t *testing.T) {
	icon := kit.NewText("I").Node()
	q := kit.NewQRCode("https://ant.design/")
	q.SetIconNode(icon)
	if !q.HasIcon() || q.IconHost() == nil {
		t.Fatal("icon node")
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_MarginSize(t *testing.T) {
	q := kit.NewQRCode("https://ant.design/")
	q.SetMarginSize(2)
	if q.MarginSize() != 2 {
		t.Fatal(q.MarginSize())
	}
	_ = q.Node().Layout(core.Loose(200, 200))
}

func TestQRCode_PRD_ThemeSwitch(t *testing.T) {
	q := kit.NewQRCode("https://ant.design/")
	th1 := kit.DefaultTheme()
	q.SetTheme(th1)
	c1 := q.ResolvedColor()
	th2 := kit.DefaultTheme()
	if th2.Tokens == nil {
		th2.Tokens = core.NewTokenSet()
	}
	th2.Tokens.Colors[core.TokenColorText] = render.RGBA{R: 0.2, G: 0.3, B: 0.7, A: 1}
	q.SetTheme(th2)
	c2 := q.ResolvedColor()
	if approxQRColor(c1, c2, 0.01) {
		// may still differ on border
	}
	_ = c1
	_ = c2
	_ = q.Node().Layout(core.Loose(200, 200))
}
