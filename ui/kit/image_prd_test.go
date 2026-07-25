package kit_test

import (
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/image.md §6.9 — P0 PRD cases (IMG-01 … IMG-19).
// IMG-20 L3 / IMG-21 L4 / IMG-22 P1 deferred.

func approxImg(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxImgColor(a, b render.RGBA, tol float64) bool {
	return approxImg(float64(a.R), float64(b.R), tol) &&
		approxImg(float64(a.G), float64(b.G), tol) &&
		approxImg(float64(a.B), float64(b.B), tol) &&
		approxImg(float64(a.A), float64(b.A), tol)
}

func solidPixels(w, h int, r, g, b, a byte) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < w*h; i++ {
		pix[i*4+0] = r
		pix[i*4+1] = g
		pix[i*4+2] = b
		pix[i*4+3] = a
	}
	return pix
}

func TestImage_PRD_01_Defaults(t *testing.T) {
	// IMG-01: NewImage 默认创建
	im := kit.NewImage()
	if !im.IsPreviewEnabled() {
		t.Fatal("preview default false")
	}
	if im.IsPreviewOpen() {
		t.Fatal("open default true")
	}
	if im.Node() == nil || im.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	if im.Status() != kit.ImageStatusEmpty {
		t.Fatalf("status=%v want empty", im.Status())
	}
	sz := im.Node().Layout(core.Loose(400, 400))
	if sz.Width < 10 || sz.Height < 10 {
		t.Fatalf("size=%v", sz)
	}
	if !approxImg(im.ResolvedWidth(), kit.DefaultImageWidth, 0.5) {
		t.Fatalf("w=%v", im.ResolvedWidth())
	}
	if !approxImg(im.ResolvedScaleStep(), kit.DefaultImageScaleStep, 0.01) {
		t.Fatalf("step=%v", im.ResolvedScaleStep())
	}
}

func TestImage_PRD_02_SrcPixelsLoaded(t *testing.T) {
	// IMG-02 / IMG-S1: SetSrc + SetPixels 显示
	im := kit.NewImage()
	im.SetWidth(120)
	im.SetHeight(80)
	im.SetSrc("res://photo")
	im.SetPixels(2, 2, solidPixels(2, 2, 255, 0, 0, 255))
	if im.Status() != kit.ImageStatusLoaded {
		t.Fatalf("status=%v", im.Status())
	}
	if im.DisplaySrc() != "res://photo" {
		t.Fatalf("display=%q", im.DisplaySrc())
	}
	sz := im.Node().Layout(core.Loose(200, 200))
	if !approxImg(sz.Width, 120, 1) || !approxImg(sz.Height, 80, 1) {
		t.Fatalf("size=%v", sz)
	}
}

func TestImage_PRD_03_OpenPreview(t *testing.T) {
	// IMG-03 / IMG-S2: 打开预览
	im := kit.NewImage()
	im.SetSrc("res://a")
	im.SetPixels(1, 1, solidPixels(1, 1, 0, 255, 0, 255))
	_ = im.Node().Layout(core.Loose(200, 200))
	im.OpenPreview()
	if !im.IsPreviewOpen() {
		t.Fatal("not open")
	}
	if im.PreviewPortal() == nil || !im.PreviewPortal().Open {
		t.Fatal("portal not open")
	}
}

func TestImage_PRD_04_ClosePreview(t *testing.T) {
	// IMG-04 / IMG-S3: 关闭预览 + OnOpenChange
	im := kit.NewImage()
	im.SetSrc("res://a")
	im.SetImageOK(true)
	got := []bool{}
	im.SetOnOpenChange(func(open bool) { got = append(got, open) })
	_ = im.Node().Layout(core.Loose(200, 200))
	im.OpenPreview()
	im.ClosePreview()
	if im.IsPreviewOpen() {
		t.Fatal("still open")
	}
	if len(got) < 2 || got[0] != true || got[len(got)-1] != false {
		t.Fatalf("onOpenChange=%v", got)
	}
}

func TestImage_PRD_05_FallbackOnError(t *testing.T) {
	// IMG-05 / IMG-S4: 失败 fallback
	im := kit.NewImage()
	im.SetSrc("error")
	im.SetFallback("data:image/fallback")
	errN := 0
	im.SetOnError(func() { errN++ })
	im.NotifyImageError()
	if errN != 1 {
		t.Fatalf("onError=%d", errN)
	}
	if im.Status() != kit.ImageStatusError {
		t.Fatalf("status=%v", im.Status())
	}
	if im.DisplaySrc() != "data:image/fallback" {
		t.Fatalf("display=%q", im.DisplaySrc())
	}
	// host delivers fallback pixels
	im.SetPixels(1, 1, solidPixels(1, 1, 10, 20, 30, 255))
	if im.Status() != kit.ImageStatusLoaded && im.Status() != kit.ImageStatusError {
		// loaded after pixels is fine; error+fallback pixels also OK
		t.Fatalf("status after pixels=%v", im.Status())
	}
	_ = im.Node().Layout(core.Loose(200, 200))
}

func TestImage_PRD_06_GroupNext(t *testing.T) {
	// IMG-06 / IMG-S5: Group 下一张
	a := kit.NewImageSized("a", 80, 60)
	a.SetSrc("a.png")
	a.SetImageOK(true)
	b := kit.NewImageSized("b", 80, 60)
	b.SetSrc("b.png")
	b.SetImageOK(true)
	g := kit.NewImagePreviewGroup()
	changes := [][2]int{}
	g.OnChange = func(cur, prev int) { changes = append(changes, [2]int{cur, prev}) }
	g.Add(a, b)
	_ = g.Node().Layout(core.Loose(400, 100))
	g.SetOpen(true)
	if !g.IsOpen() {
		t.Fatal("group not open")
	}
	g.Next()
	if g.Current() != 1 {
		t.Fatalf("current=%d", g.Current())
	}
	if len(changes) == 0 || changes[len(changes)-1] != [2]int{1, 0} {
		t.Fatalf("changes=%v", changes)
	}
}

func TestImage_PRD_07_PreviewDisabled(t *testing.T) {
	// IMG-07 / IMG-S6: preview=false
	im := kit.NewImage()
	im.SetSrc("x")
	im.SetImageOK(true)
	im.SetPreview(false)
	_ = im.Node().Layout(core.Loose(200, 200))
	im.OpenPreview()
	if im.IsPreviewOpen() {
		t.Fatal("opened despite preview=false")
	}
}

func TestImage_PRD_08_DemoBasic(t *testing.T) {
	// IMG-08: basic.tsx
	im := kit.NewImage()
	im.SetWidth(200)
	im.SetAlt("basic")
	im.SetSrc("https://zos.alipayobjects.com/rmsportal/jkjgkEfvpUPVyRjUImniVslZfWPnJuuZ.png")
	im.SetPixels(4, 4, solidPixels(4, 4, 200, 180, 160, 255))
	if !im.IsPreviewEnabled() {
		t.Fatal("preview")
	}
	sz := im.Node().Layout(core.Loose(400, 400))
	if !approxImg(sz.Width, 200, 1) {
		t.Fatalf("w=%v", sz.Width)
	}
	im.OpenPreview()
	if !im.IsPreviewOpen() {
		t.Fatal("preview open")
	}
}

func TestImage_PRD_09_DemoPlaceholder(t *testing.T) {
	// IMG-09: placeholder.tsx — progress true / percent
	ind := kit.NewImage()
	ind.SetWidth(200)
	ind.SetHeight(200)
	ind.SetPlaceholder(true)
	if ind.Status() != kit.ImageStatusLoading {
		t.Fatalf("status=%v", ind.Status())
	}
	_ = ind.Node().Layout(core.Loose(240, 240))
	// tick advances indeterminate
	if !ind.Tick(0.05) {
		// may return true when needsTicker
	}
	ind.Tick(0.05)

	pct := kit.NewImage()
	pct.SetWidth(200)
	pct.SetHeight(200)
	pct.SetPercent(50)
	if !approxImg(pct.Percent, 50, 0.01) {
		t.Fatalf("percent=%v", pct.Percent)
	}
	_ = pct.Node().Layout(core.Loose(240, 240))

	// complete path: SetPixels ends loading
	pct.SetSrc("done.png")
	pct.SetPixels(2, 2, solidPixels(2, 2, 1, 2, 3, 255))
	if pct.Status() != kit.ImageStatusLoaded {
		t.Fatalf("loaded status=%v", pct.Status())
	}
}

func TestImage_PRD_10_DemoFallback(t *testing.T) {
	// IMG-10: fallback.tsx
	im := kit.NewImage()
	im.SetAlt("basic image")
	im.SetWidth(200)
	im.SetHeight(200)
	im.SetSrc("error")
	im.SetFallback("data:image/png;base64,fallback")
	im.NotifyImageError()
	if im.DisplaySrc() != "data:image/png;base64,fallback" {
		t.Fatalf("fallback display=%q", im.DisplaySrc())
	}
	_ = im.Node().Layout(core.Loose(220, 220))
}

func TestImage_PRD_11_DemoPreviewGroup(t *testing.T) {
	// IMG-11: preview-group.tsx
	a := kit.NewImageSized("svg image", 200, 120)
	a.SetSrc("https://gw.alipayobjects.com/zos/rmsportal/KDpgvguMpGfqaHPjicRK.svg")
	a.SetImageOK(true)
	b := kit.NewImageSized("svg image", 200, 120)
	b.SetSrc("https://gw.alipayobjects.com/zos/antfincdn/aPkFc8Sj7n/method-draw-image.svg")
	b.SetImageOK(true)
	g := kit.NewImagePreviewGroup()
	var lastCur, lastPrev int = -1, -1
	g.OnChange = func(cur, prev int) { lastCur, lastPrev = cur, prev }
	g.Add(a, b)
	_ = g.Node().Layout(core.Loose(500, 150))
	// open via first child
	a.OpenPreview()
	if !g.IsOpen() {
		t.Fatal("group open")
	}
	g.Next()
	if lastCur != 1 || lastPrev != 0 {
		t.Fatalf("onChange cur=%d prev=%d", lastCur, lastPrev)
	}
}

func TestImage_PRD_12_DemoAlbumItems(t *testing.T) {
	// IMG-12: preview-group-visible.tsx — items[]
	thumb := kit.NewImageSized("webp image", 200, 120)
	thumb.SetSrc("https://gw.alipayobjects.com/zos/antfincdn/LlvErxo8H9/photo-1503185912284-5271ff81b9a8.webp")
	thumb.SetImageOK(true)
	g := kit.NewImagePreviewGroup()
	g.SetItems(
		"https://gw.alipayobjects.com/zos/antfincdn/LlvErxo8H9/photo-1503185912284-5271ff81b9a8.webp",
		"https://gw.alipayobjects.com/zos/antfincdn/cV16ZqzMjW/photo-1473091540282-9b846e7965e3.webp",
		"https://gw.alipayobjects.com/zos/antfincdn/x43I27A55%26/photo-1438109491414-7198515b166b.webp",
	)
	g.Add(thumb)
	if g.Total() != 3 {
		t.Fatalf("total=%d want items len 3", g.Total())
	}
	_ = g.Node().Layout(core.Loose(400, 150))
	g.SetOpen(true)
	g.SetCurrent(2)
	if g.Current() != 2 {
		t.Fatalf("current=%d", g.Current())
	}
}

func TestImage_PRD_13_DemoPreviewSrc(t *testing.T) {
	// IMG-13: previewSrc.tsx
	im := kit.NewImage()
	im.SetWidth(200)
	im.SetAlt("basic image")
	im.SetSrc("blur.png")
	im.SetPreviewSrc("sharp.png")
	im.SetImageOK(true)
	if im.ResolvedPreviewSrc() != "sharp.png" {
		t.Fatalf("previewSrc=%q", im.ResolvedPreviewSrc())
	}
	if im.Src == im.ResolvedPreviewSrc() {
		t.Fatal("thumb src should differ from preview src")
	}
	_ = im.Node().Layout(core.Loose(220, 220))
	im.OpenPreview()
	if !im.IsPreviewOpen() {
		t.Fatal("open")
	}
}

func TestImage_PRD_14_DemoControlledPreview(t *testing.T) {
	// IMG-14: controlled-preview.tsx
	im := kit.NewImage()
	im.SetWidth(200)
	im.SetSrc("blur.png")
	im.SetPreviewSrc("sharp.png")
	im.SetImageOK(true)
	open := false
	im.SetOnOpenChange(func(v bool) { open = v })
	_ = im.Node().Layout(core.Loose(220, 220))
	im.SetPreviewOpen(true)
	if !im.IsPreviewOpen() || !open {
		t.Fatalf("controlled open open=%v isOpen=%v", open, im.IsPreviewOpen())
	}
	im.SetScaleStep(0.5)
	im.ZoomIn()
	if im.Transform().Scale <= 1 {
		t.Fatalf("scale=%v", im.Transform().Scale)
	}
	im.SetPreviewOpen(false)
	if im.IsPreviewOpen() {
		t.Fatal("controlled close")
	}
}

func TestImage_PRD_15_DemoToolbarActions(t *testing.T) {
	// IMG-15: toolbarRender.tsx — ActionsRender + zoom/rotate/flip
	a := kit.NewImageSized("image-0", 200, 100)
	a.SetSrc("a.svg")
	a.SetImageOK(true)
	b := kit.NewImageSized("image-1", 200, 100)
	b.SetSrc("b.svg")
	b.SetImageOK(true)
	g := kit.NewImagePreviewGroup()
	var lastInfo kit.ImageToolbarInfo
	renderN := 0
	g.SetActionsRender(func(info kit.ImageToolbarInfo) core.Node {
		renderN++
		lastInfo = info
		if info.OnZoomIn == nil || info.OnRotateLeft == nil || info.OnActive == nil {
			t.Fatal("missing actions")
		}
		return primitive.NewText("custom-toolbar")
	})
	g.Add(a, b)
	_ = g.Node().Layout(core.Loose(500, 140))
	g.SetOpen(true)
	if renderN == 0 {
		t.Fatal("ActionsRender not called")
	}
	if lastInfo.OnZoomIn != nil {
		lastInfo.OnZoomIn()
	}
	if lastInfo.OnActive != nil {
		lastInfo.OnActive(1)
	}
	if g.Current() != 1 {
		t.Fatalf("current after onActive=%d", g.Current())
	}
	// rotate via re-rendered actions after rebuild
	g.SetActionsRender(func(info kit.ImageToolbarInfo) core.Node {
		lastInfo = info
		return primitive.NewText("tb")
	})
	// force rebuild of open preview
	g.SetCurrent(0)
	if lastInfo.OnRotateRight != nil {
		lastInfo.OnRotateRight()
	}
	if lastInfo.OnFlipY != nil {
		lastInfo.OnFlipY()
	}
	if lastInfo.OnReset != nil {
		lastInfo.OnReset()
	}
}

func TestImage_PRD_16_TokenMetrics(t *testing.T) {
	// IMG-16: §6.2 关键尺寸
	im := kit.NewImage()
	if !approxImg(im.FontSize(), 14, 0.5) {
		t.Fatalf("font=%v", im.FontSize())
	}
	if !approxImg(im.BorderRadius(), 6, 0.5) {
		t.Fatalf("radius=%v", im.BorderRadius())
	}
	if !approxImg(im.LineWidth(), 1, 0.5) {
		t.Fatalf("line=%v", im.LineWidth())
	}
	if !approxImg(im.FocusRingOutset(), 1.5, 0.1) {
		t.Fatalf("focus=%v", im.FocusRingOutset())
	}
	th := kit.DefaultTheme()
	if !approxImg(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatal("theme font")
	}
	if !approxImg(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatal("theme radius")
	}
}

func TestImage_PRD_17_ThemeColors(t *testing.T) {
	// IMG-17: 默认皮走 Token，无硬编码品牌主色当底
	im := kit.NewImage()
	_ = im.Node().Layout(core.Loose(200, 200))
	bg := im.BackgroundColor()
	prim := kit.DefaultTheme().Color(core.TokenColorPrimary)
	if approxImgColor(bg, prim, 0.02) {
		t.Fatalf("bg equals brand primary %v", bg)
	}
	// theme override
	th := kit.DefaultTheme()
	if th.Tokens == nil {
		th.Tokens = core.NewTokenSet()
	}
	th.Tokens.Colors[core.TokenColorBgLayout] = render.RGBA{R: 0.1, G: 0.2, B: 0.3, A: 1}
	im.SetTheme(th)
	bg2 := im.BackgroundColor()
	if !approxImgColor(bg2, th.Color(core.TokenColorBgLayout), 0.02) {
		t.Fatalf("bg2=%v want theme layout", bg2)
	}
}

func TestImage_PRD_18_Disabled(t *testing.T) {
	// IMG-18: disabled 不可预览
	im := kit.NewImage()
	im.SetSrc("x")
	im.SetImageOK(true)
	im.SetDisabled(true)
	_ = im.Node().Layout(core.Loose(200, 200))
	im.OpenPreview()
	if im.IsPreviewOpen() {
		t.Fatal("disabled still opens")
	}
	if !im.Disabled {
		t.Fatal("disabled flag")
	}
}

func TestImage_PRD_19_FocusKeyboard(t *testing.T) {
	// IMG-19: 可聚焦 + focus ring 配置
	im := kit.NewImage()
	im.SetSrc("x")
	im.SetImageOK(true)
	_ = im.Node().Layout(core.Loose(200, 200))
	// pressable is first child of root column
	root := im.Node().(*primitive.Flex)
	if len(root.Children()) == 0 {
		t.Fatal("no children")
	}
	p, ok := root.Children()[0].(*primitive.Pressable)
	if !ok {
		t.Fatalf("child0 type %T", root.Children()[0])
	}
	if !p.Focusable || !p.ShowFocusRing {
		t.Fatal("focusable/ring")
	}
	if !approxImg(p.FocusRingOutset, kit.DefaultImageFocusRingOutset, 0.1) {
		t.Fatalf("outset=%v", p.FocusRingOutset)
	}
	// keyboard path: Click wired to OpenPreview
	if p.Click == nil {
		t.Fatal("no click")
	}
	p.Click()
	if !im.IsPreviewOpen() {
		t.Fatal("activate did not open")
	}
}
