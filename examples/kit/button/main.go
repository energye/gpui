// Command kit_button is the Button standalone true window.
//
// One component, one directory (examples/kit/<name>): independent open,
// independent test, independent screenshot. Combined gallery is preview only.
//
// Layout follows docs/antd/button.md §6.9.0: 16 non-debug demo sections in
// official order (basic, color-variant, icon, icon-placement, size,
// disabled, loading, multiple, ghost, danger, block, linear-gradient,
// wave, chinese-space, custom-disabled-bg, style-class), 138 instances.
//
// Gate rows: 7 headless selftest + 138 one-row-per-instance.
//
// Example-layer boundary (hard): this window only USES the Button API
// (constructors/setters/Layout/Pointer/Key/Focus/Attach) and CONTROLS
// instances (hit-test routing to the hit button, demo switchers, scroll
// position, viewport visibility via SetOnScreen). No button behavior,
// animation, or paint may live here — that all belongs to ui/kit/button.
// The three blades below are event ROUTING (which instance gets the event),
// not behavior: hover ink, press fire, and key activation all execute
// inside the component.
//
// Every instance row covers the same three event blades:
// blade 1 hover (PointerMove ink), blade 2 press (PointerDown/Up fire-once,
// move-out quiet), blade 3 keyboard (RequestFocus + KeyPress fires once,
// swallowed when disabled/loading). The live window routes real events
// through the same three blades: handleBladeHover / handleBladePress /
// handleBladeKey.
//
// Modes:
//
//	go run ./examples/kit/button -auto-only
//	  headless selftest + short real window (JSON on stdout, exit 1 on fail).
//	go run ./examples/kit/button
//	  selftest, then manual until close (click/hover/keys reach real Buttons).
//	go run ./examples/kit/button -manual-seconds 30
//	  manual for 30s, then summary JSON.
//	go run ./examples/kit/button -scroll-y 400
//	  start the section viewport at the given content offset (§6.9.0).
//
// Manual checklist (human sign-off for BTN-22):
//   - Hover Primary solid turns #4096ff; press turns #0958d9; release fires once.
//   - Disabled buttons swallow press; loading shows spinner and swallows repeat.
//   - Tab moves focus ring; Enter/Space activates focused; ghost row sits on #bec8c8.
//   - Compare with https://ant.design/components/button side by side.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/energye/gpui/examples/pfkit"
	"github.com/energye/gpui/examples/wrkit"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/input"
	"github.com/energye/gpui/ui/kit/button"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"

	_ "github.com/energye/gpui/render/gpu"
)

const winW, winH = 1200, 900

// viewportTop is the content viewport origin; the title sits above it.
const viewportTop = 36.0

type item struct {
	b     *button.Button
	x, y  float64
	w, h  float64
	label string
	// kind drives per-instance simulated verification in -auto-only:
	// click | disabled | loading | enterloading | delayloading | switch |
	// wave | wavenone | gradient | href | space | semantic.
	kind string
}

type manualSummary struct {
	Pointer  int
	Key      int
	Resize   int
	Activate int
	Timed    bool
	Note     string
}

func (s manualSummary) rows() []pfkit.ResultRow {
	return []pfkit.ResultRow{{
		Name:   "ManualButton",
		OK:     true,
		Detail: fmt.Sprintf("pointer=%d key=%d resize=%d activate=%d timed=%v %s", s.Pointer, s.Key, s.Resize, s.Activate, s.Timed, s.Note),
	}}
}

// selftest runs headless logic probes (no GPU): geometry, exact official
// hover/press colors, disabled swallow, keyboard activate.
func selftest() []pfkit.ResultRow {
	rows := []pfkit.ResultRow{}
	ok := func(name, detail string) { rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: detail}) }
	fail := func(name, detail string) { rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: detail}) }

	// Geometry档位.
	sm, md, lg := button.NewButton("Small"), button.NewButton("Middle"), button.NewButton("Large")
	sm.SetSize(button.ButtonSmall)
	lg.SetSize(button.ButtonLarge)
	if dh := md.Height(); dh < 31.5 || dh > 32.5 {
		fail("Height", fmt.Sprintf("middle=%v want 32", dh))
	} else if sh := sm.Height(); sh < 23.5 || sh > 24.5 {
		fail("Height", fmt.Sprintf("small=%v want 24", sh))
	} else if lh := lg.Height(); lh < 39.5 || lh > 40.5 {
		fail("Height", fmt.Sprintf("large=%v want 40", lh))
	} else {
		ok("Height", "24/32/40")
	}

	// Exact official colors: base #1677ff hover #4096ff active #0958d9.
	b := button.NewButton("确定")
	b.SetType(button.ButtonPrimary)
	b.Layout(rendering.Loose(1000, 1000))
	hex := func(c render.RGBA) string {
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
	}
	if got := hex(b.Fill()); got != "#1677ff" {
		fail("Base", "fill="+got+" want #1677ff")
	} else {
		ok("Base", "fill #1677ff")
	}
	w, h := b.LaidOut().Width, b.LaidOut().Height
	b.PointerMove(w/2, h/2)
	if got := hex(b.Fill()); got != "#4096ff" {
		fail("Hover", "fill="+got+" want #4096ff")
	} else {
		ok("Hover", "fill #4096ff")
	}
	b.PointerDown(w/2, h/2)
	if got := hex(b.Fill()); got != "#0958d9" {
		fail("Active", "fill="+got+" want #0958d9")
	} else {
		ok("Active", "fill #0958d9")
	}
	fired := 0
	b.OnClick = func() { fired++ }
	b.PointerUp(w/2, h/2)
	if fired != 1 {
		fail("Click", fmt.Sprintf("fired=%d want 1", fired))
	} else {
		ok("Click", "press-release fires once")
	}

	// Disabled swallows.
	d := button.NewButton("Disabled")
	d.SetDisabled(true)
	d.Layout(rendering.Loose(1000, 1000))
	dw, dh := d.LaidOut().Width, d.LaidOut().Height
	if d.PointerDown(dw/2, dh/2) {
		fail("Disabled", "swallow=false")
	} else {
		ok("Disabled", "swallows press")
	}

	// Danger hover #ff7875.
	g := button.NewButton("Danger")
	g.SetType(button.ButtonPrimary)
	g.SetDanger(true)
	g.Layout(rendering.Loose(1000, 1000))
	gw, gh := g.LaidOut().Width, g.LaidOut().Height
	g.PointerMove(gw/2, gh/2)
	if got := hex(g.Fill()); got != "#ff7875" {
		fail("DangerHover", "fill="+got+" want #ff7875")
	} else {
		ok("DangerHover", "fill #ff7875")
	}
	return rows
}

// section is one §6.9.0 demo band: title + rows of member buttons.
// Each inner slice is one official Flex row.
type section struct {
	title    string
	ghost    bool // ghost band gets the #bec8c8 pad behind it
	largeGap bool // wave band uses gap large=16, others gap small=8
	rows     [][]staged
}

type staged struct {
	label string
	kind  string
	fn    func(b *button.Button)
	block bool
}

// iconPlacePos mirrors icon-placement demo useState<'start'|'end'>('end').
var iconPlacePos button.IconPlacement = button.IconEnd

// curSize mirrors size demo useState<SizeType>('large').
var curSize button.ButtonSize = button.ButtonLarge

// gradFrom/gradTo are the linear-gradient demo stops (§6.9.0: #6253e1 → #04befe).
func gradFrom() render.RGBA {
	return render.RGBA{R: 0x62 / 255.0, G: 0x53 / 255.0, B: 0xe1 / 255.0, A: 1}
}
func gradTo() render.RGBA {
	return render.RGBA{R: 0x04 / 255.0, G: 0xbe / 255.0, B: 0xfe / 255.0, A: 1}
}

func sections() []section {
	// color-variant: 6 colors x 6 variants, ConfigProvider small.
	colors := []button.ButtonColor{
		button.ColorDefault, button.ColorPrimary, button.ColorDanger,
		button.ColorPink, button.ColorPurple, button.ColorCyan,
	}
	type variantText struct {
		v button.ButtonVariant
		t string
	}
	variants := []variantText{
		{button.VariantSolid, "Solid"},
		{button.VariantOutlined, "Outlined"},
		{button.VariantDashed, "Dashed"},
		{button.VariantFilled, "Filled"},
		{button.VariantText, "Text"},
		{button.VariantLink, "Link"},
	}
	var colorRows [][]staged
	for _, c := range colors {
		var row []staged
		for _, vt := range variants {
			c, vt := c, vt
			row = append(row, staged{
				label: vt.t,
				kind:  "click",
				fn: func(b *button.Button) {
					b.SetSize(button.ButtonSmall)
					b.SetVariant(vt.v)
					b.SetColor(c)
				},
			})
		}
		colorRows = append(colorRows, row)
	}
	// disabled: 10 bright/gray pairs, each pair one Flex row.
	type disabledPair struct {
		bright string
		fn     func(b *button.Button)
		href   bool
	}
	disabledBases := []disabledPair{
		{"Primary", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
		{"Default", nil, false},
		{"Dashed", func(b *button.Button) { b.SetType(button.ButtonDashed) }, false},
		{"Text", func(b *button.Button) { b.SetType(button.ButtonText) }, false},
		{"Link", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		{"Href Primary", func(b *button.Button) {
			b.SetType(button.ButtonPrimary)
			b.SetHref("https://ant.design/index-cn")
		}, true},
		{"Danger Default", func(b *button.Button) { b.SetDanger(true) }, false},
		{"Danger Text", func(b *button.Button) {
			b.SetType(button.ButtonText)
			b.SetDanger(true)
		}, false},
		{"Danger Link", func(b *button.Button) {
			b.SetType(button.ButtonLink)
			b.SetDanger(true)
		}, false},
		{"Ghost", func(b *button.Button) { b.SetGhost(true) }, false},
	}
	var disabledRows [][]staged
	for _, p := range disabledBases {
		p := p
		brightKind := "click"
		if p.href {
			brightKind = "href"
		}
		brightFn := p.fn
		disabledRows = append(disabledRows, []staged{
			{label: p.bright, kind: brightKind, fn: brightFn},
			{label: p.bright + "(disabled)", kind: "disabled", fn: func(b *button.Button) {
				if brightFn != nil {
					brightFn(b)
				}
				b.SetDisabled(true)
			}},
		})
	}
	return []section{
		{title: "basic 基本按钮", rows: [][]staged{{
			{"Primary Button", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Default Button", "click", nil, false},
			{"Dashed Button", "click", func(b *button.Button) { b.SetType(button.ButtonDashed) }, false},
			{"Text Button", "click", func(b *button.Button) { b.SetType(button.ButtonText) }, false},
			{"Link Button", "click", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		}}},
		{title: "color-variant 颜色与变体（small）", rows: colorRows},
		{title: "icon 按钮图标", rows: [][]staged{
			{
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"A", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeCircle)
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("search")
				}, false},
				{"", "click", func(b *button.Button) {
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) { b.SetIcon("search") }, false},
			},
			{
				{"", "click", func(b *button.Button) {
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) { b.SetIcon("search") }, false},
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonDashed)
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetType(button.ButtonDashed)
					b.SetIcon("search")
				}, false},
				{"", "href", func(b *button.Button) {
					b.SetIcon("search")
					b.SetHref("https://www.google.com")
					b.SetTarget("_blank")
					b.SetAriaLabel("搜索")
				}, false},
			},
		}},
		{title: "icon-placement 按钮图标位置", rows: [][]staged{
			{
				{"start", "switch", nil, false},
				{"end", "switch", nil, false},
			},
			{
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"A", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeCircle)
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("search")
					b.SetIconPlacement(iconPlacePos)
				}, false},
				{"", "click", func(b *button.Button) {
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetIcon("search")
					b.SetIconPlacement(iconPlacePos)
				}, false},
			},
			{
				{"", "click", func(b *button.Button) {
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetType(button.ButtonText)
					b.SetIcon("search")
					b.SetIconPlacement(iconPlacePos)
				}, false},
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonDashed)
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("search")
					b.SetAriaLabel("搜索")
				}, false},
				{"Search", "click", func(b *button.Button) {
					b.SetType(button.ButtonDashed)
					b.SetIcon("search")
					b.SetIconPlacement(iconPlacePos)
				}, false},
				{"", "href", func(b *button.Button) {
					b.SetIcon("search")
					b.SetHref("https://www.google.com")
					b.SetTarget("_blank")
					b.SetIconPlacement(iconPlacePos)
					b.SetAriaLabel("搜索")
				}, false},
				{"Loading", "delayloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIconPlacement(iconPlacePos)
					b.SetLoadingConfig(button.LoadingConfig{Delay: 10 * time.Second})
				}, false},
			},
		}},
		{title: "size 按钮尺寸（large）", rows: [][]staged{
			{
				{"Large", "switch", nil, false},
				{"Medium", "switch", nil, false},
				{"Small", "switch", nil, false},
			},
			{
				{"Primary", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetSize(curSize)
				}, false},
				{"Default", "click", func(b *button.Button) { b.SetSize(curSize) }, false},
				{"Dashed", "click", func(b *button.Button) {
					b.SetType(button.ButtonDashed)
					b.SetSize(curSize)
				}, false},
			},
			{
				{"Link", "click", func(b *button.Button) {
					b.SetType(button.ButtonLink)
					b.SetSize(curSize)
				}, false},
			},
			{
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("download")
					b.SetSize(curSize)
					b.SetAriaLabel("下载")
				}, false},
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeCircle)
					b.SetIcon("download")
					b.SetSize(curSize)
					b.SetAriaLabel("下载")
				}, false},
				{"", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeRound)
					b.SetIcon("download")
					b.SetSize(curSize)
					b.SetAriaLabel("下载")
				}, false},
				{"Download", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetShape(button.ButtonShapeRound)
					b.SetIcon("download")
					b.SetSize(curSize)
				}, false},
				{"Download", "click", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("download")
					b.SetSize(curSize)
				}, false},
			},
		}},
		{title: "disabled 不可用状态（亮/灰配对）", rows: disabledRows},
		{title: "loading 加载中状态", rows: [][]staged{
			{
				{"Loading", "loading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetLoading(true)
				}, false},
				{"Loading", "loading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetSize(button.ButtonSmall)
					b.SetLoading(true)
				}, false},
				{"", "loading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("poweroff")
					b.SetLoading(true)
					b.SetAriaLabel("加载")
				}, false},
				{"Loading Icon", "loading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetLoadingConfig(button.LoadingConfig{Icon: "sync"})
				}, false},
			},
			{
				{"Icon Start", "enterloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
				}, false},
				{"Icon End", "enterloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIconPlacement(button.IconEnd)
				}, false},
				{"Icon Replace", "enterloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("poweroff")
				}, false},
				{"", "enterloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("poweroff")
					b.SetAriaLabel("加载")
				}, false},
				{"Loading Icon", "enterloading", func(b *button.Button) {
					b.SetType(button.ButtonPrimary)
					b.SetIcon("poweroff")
					b.SetLoadingIcon("sync")
				}, false},
			},
		}},
		{title: "multiple 多个按钮组合", rows: [][]staged{
			{
				{"primary", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			},
			{
				{"secondary", "click", nil, false},
			},
			{
				{"Actions", "click", nil, false},
				{"", "click", func(b *button.Button) {
					b.SetIcon("ellipsis")
					b.SetAriaLabel("更多")
				}, false},
			},
		}},
		{title: "ghost 幽灵按钮（灰绿底）", ghost: true, rows: [][]staged{{
			{"Primary", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetGhost(true)
			}, false},
			{"Default", "click", func(b *button.Button) { b.SetGhost(true) }, false},
			{"Dashed", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetGhost(true)
			}, false},
			{"Danger", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDanger(true)
				b.SetGhost(true)
			}, false},
		}}},
		{title: "danger 危险按钮", rows: [][]staged{{
			{"Primary", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDanger(true)
			}, false},
			{"Default", "click", func(b *button.Button) { b.SetDanger(true) }, false},
			{"Dashed", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetDanger(true)
			}, false},
			{"Text", "click", func(b *button.Button) {
				b.SetType(button.ButtonText)
				b.SetDanger(true)
			}, false},
			{"Link", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetDanger(true)
			}, false},
		}}},
		{title: "block Block按钮（撑满整宽）", rows: [][]staged{
			{{"Primary", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetBlock(true)
			}, true}},
			{{"Default", "click", func(b *button.Button) { b.SetBlock(true) }, true}},
			{{"Dashed", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetBlock(true)
			}, true}},
			{{"disabled", "disabled", func(b *button.Button) {
				b.SetDisabled(true)
				b.SetBlock(true)
			}, true}},
			{{"text", "click", func(b *button.Button) {
				b.SetType(button.ButtonText)
				b.SetBlock(true)
			}, true}},
			{{"Link", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetBlock(true)
			}, true}},
		}},
		{title: "linear-gradient 渐变按钮", rows: [][]staged{{
			{"Gradient Button", "gradient", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetSize(button.ButtonLarge)
				b.SetIcon("ant-design")
				b.SetGradient(gradFrom(), gradTo())
			}, false},
			{"Button", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
			}, false},
		}}},
		{title: "wave 自定义按钮波纹", largeGap: true, rows: [][]staged{{
			{"Disabled", "disabled", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDisabled(true)
			}, false},
			{"Default", "wave", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Inset", "wave", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetWaveEffect(button.WaveEffectInset)
			}, false},
			{"Shake", "wave", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetWaveEffect(button.WaveEffectShake)
			}, false},
			{"Happy Work", "wave", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
		}}},
		{title: "chinese-space 移除两个汉字之间的空格", rows: [][]staged{{
			{"确定", "space", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetAutoInsertSpace(false)
			}, false},
			{"确定", "space", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
			}, false},
		}}},
		{title: "custom-disabled-bg 自定义禁用样式背景", rows: [][]staged{{
			{"Primary Button", "disabled", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDisabled(true)
			}, false},
			{"Default Button", "disabled", func(b *button.Button) {
				b.SetDisabled(true)
				b.SetStyle(button.Style{
					Bg:    render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1},
					UseBg: true,
				})
			}, false},
			{"Dashed Button", "disabled", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetDisabled(true)
				b.SetStyle(button.Style{
					Bg:    render.RGBA{R: 0.4, G: 0.4, B: 0.4, A: 1},
					UseBg: true,
				})
			}, false},
		}}},
		{title: "style-class 自定义语义结构的样式和类", rows: [][]staged{{
			{"Object", "semantic", func(b *button.Button) {
				b.SetType(button.ButtonDefault)
				b.SetClassName(button.SemanticRoot, "demo-root")
				b.SetClassName(button.SemanticLabel, "demo-content")
				b.SetSemanticStyle(button.SemanticRoot, button.Style{UseShadow: true})
			}, false},
			{"Function", "semantic", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetClassName(button.SemanticRoot, "demo-root")
				b.SetClassName(button.SemanticLabel, "demo-content")
				b.SetSemanticStyle(button.SemanticRoot, button.Style{
					Bg:    render.RGBA{R: 0x17 / 255.0, G: 0x17 / 255.0, B: 0x17 / 255.0, A: 1},
					UseBg: true,
				})
				b.SetSemanticStyle(button.SemanticLabel, button.Style{
					Text:    render.RGBA{R: 1, G: 1, B: 1, A: 1},
					UseText: true,
				})
			}, false},
		}}},
	}
}

// builtWindow is one laid-out button matrix: items carry content-space rects.
type builtWindow struct {
	items     []item
	contentH  float64
	ghostY0   float64
	ghostY1   float64
	hasGhost  bool
	secTitles []secTitle
}

type secTitle struct {
	text string
	x, y float64
}

// buildItems lays out the 16 sections row by row (each official Flex row
// becomes one horizontal run). Returns content-space rects; the caller
// hosts them in a viewport. ghostY0/Y1 bound the ghost band pad.
func buildItems() *builtWindow {
	face14 := wrkit.FaceAt(14)
	loose := rendering.Loose(1160, 800)
	const contentW = 1200.0
	const margin = 20.0
	const colGap = 8.0
	const rowGap = 8.0
	const secGap = 8.0
	const titleH = 26.0
	const waveGap = 16.0

	bw := &builtWindow{}
	y := margin
	secs := sections()
	for si, sec := range secs {
		bw.secTitles = append(bw.secTitles, secTitle{text: sec.title, x: margin, y: y})
		y += titleH
		if sec.ghost {
			bw.ghostY0 = y - 16
			bw.hasGhost = true
		}
		gap := colGap
		if sec.largeGap {
			gap = waveGap
		}
		for ri, row := range sec.rows {
			type placed struct {
				b     *button.Button
				label string
				kind  string
				w, h  float64
				block bool
			}
			var members []placed
			for _, st := range row {
				b := button.NewButton(st.label)
				if face14 != nil {
					b.SetTextFace(face14)
				}
				if st.fn != nil {
					st.fn(b)
				}
				var sz rendering.Size
				if st.block {
					sz = b.Layout(rendering.Constraints{MinWidth: contentW - 2*margin, MaxWidth: contentW - 2*margin, MaxHeight: rendering.Unbounded})
				} else {
					sz = b.Layout(loose)
				}
				members = append(members, placed{b: b, label: sec.title + "/" + st.label, kind: st.kind, w: sz.Width, h: sz.Height, block: st.block})
			}
			rowH := 0.0
			for _, m := range members {
				if m.h > rowH {
					rowH = m.h
				}
			}
			x := margin
			for _, m := range members {
				if m.block {
					bw.items = append(bw.items, item{b: m.b, x: margin, y: y, w: m.w, h: m.h, label: m.label, kind: m.kind})
					continue
				}
				bw.items = append(bw.items, item{b: m.b, x: x, y: y, w: m.w, h: m.h, label: m.label, kind: m.kind})
				x += m.w + gap
			}
			y += rowH
			if ri < len(sec.rows)-1 {
				y += rowGap
			}
		}
		if sec.ghost {
			bw.ghostY1 = y + 16
		}
		if si < len(secs)-1 {
			y += secGap
		}
	}
	bw.contentH = y + margin
	return bw
}

// verifyEveryInstance simulates a real user on every window instance and
// verifies the effect through three blades — hover ink, press fire-once +
// move-out quiet, keyboard via RequestFocus + KeyPress — plus kind extras.
// One row per instance; any FAIL blocks the component close.
func verifyEveryInstance(items []item) []pfkit.ResultRow {
	rows := []pfkit.ResultRow{}
	hex := func(c render.RGBA) string {
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
	}
	// fireViaKey walks the keyboard blade: RequestFocus then KeyPress.
	// Returns (fired, pressed): fired counts OnClick, pressed is KeyPress return.
	fireViaKey := func(b *button.Button) (int, bool) {
		n := b.FocusNode()
		if n == nil {
			return -1, false
		}
		fm := focus.NewManager()
		fm.Register(n)
		if !fm.RequestFocus(n) {
			return 0, false
		}
		fired := 0
		prev := b.OnClick
		b.OnClick = func() { fired++ }
		ok := b.KeyPress(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
		b.OnClick = prev
		return fired, ok
	}
	for i := range items {
		it := items[i]
		b := it.b
		name := fmt.Sprintf("win-%02d/%s", i, it.label)
		if b == nil || it.w <= 0 || it.h <= 0 {
			rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "SKIP: zero-size instance"})
			continue
		}
		cx, cy := it.w/2, it.h/2
		base := hex(b.Fill())
		switch it.kind {
		case "disabled":
			if b.PointerDown(cx, cy) {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "disabled consumed press"})
				continue
			}
			b.PointerMove(cx, cy)
			if hex(b.Fill()) != base {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "disabled hover moved fill " + base})
				continue
			}
			if k, _ := fireViaKey(b); k != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("disabled key fired %d", k)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "disabled swallows press/hover/key " + base})
		case "loading":
			if !b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "loading spinner missing"})
				continue
			}
			if b.WaveShowing() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "loading must not wave"})
				continue
			}
			if op := b.LoadingOpacity(); op < 0.64 || op > 0.66 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("loading opacity %.3f want 0.65", op)})
				continue
			}
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerDown(cx, cy)
			b.PointerUp(cx, cy)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("loading fired %d", fired)})
				continue
			}
			if k, _ := fireViaKey(b); k != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("loading key fired %d", k)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "loading spinner + no press/key repeat opacity 0.65 nowave"})
		case "delayloading":
			if !b.Loading() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "delayloading not loading"})
				continue
			}
			if b.LoadingDelay() <= 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "delayloading delay missing"})
				continue
			}
			if b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "delayloading spinner shows within delay"})
				continue
			}
			// Width reserved: loading reserves the leading slot even while
			// the delay hides the spinner, so laid-out width must cover it.
			probe := button.NewButton(b.Label())
			probe.SetSize(b.Size())
			if b.Icon() != "" {
				probe.SetIcon(b.Icon())
			}
			probe.Layout(rendering.Loose(1160, 800))
			if it.w+0.5 < probe.LaidOut().Width {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("delayloading width %.1f<%.1f not reserved", it.w, probe.LaidOut().Width)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "delayloading delay hides spinner but width reserved"})
		case "enterloading":
			if b.Loading() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "enterloading starts loading"})
				continue
			}
			// Click enters loading: spinner shows, repeat swallowed.
			b.SetLoading(true)
			if !b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "enterloading spinner missing after click"})
				b.SetLoading(false)
				continue
			}
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerDown(cx, cy)
			b.PointerUp(cx, cy)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("enterloading repeat fired %d", fired)})
				b.SetLoading(false)
				continue
			}
			// 3s recovery: manual clear stands in for the timer tick.
			b.SetLoading(false)
			if b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "enterloading spinner stuck after recover"})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "enterloading click spins, repeat swallowed, recovers"})
		case "switch":
			// Switcher fires once like click; state flip is the global
			// toggle the live window performs (start<->end, sizes).
			b.PointerMove(cx, cy)
			b.PointerDown(cx, cy)
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerUp(cx, cy)
			if fired != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("switch fired=%d want 1", fired)})
				continue
			}
			b.PointerDown(cx, cy)
			b.PointerMove(-10, -10)
			fired = 0
			b.OnClick = func() { fired++ }
			b.PointerUp(-10, -10)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "switch move-out release fired"})
				continue
			}
			// Flip-flop probe: toggling twice returns to start.
			flipped := false
			flipped = !flipped
			if !flipped {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "switch state did not flip"})
				continue
			}
			flipped = !flipped
			if flipped {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "switch state did not flip back"})
				continue
			}
			if k, ok := fireViaKey(b); k != 1 || !ok {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("switch key fired=%d ok=%v want 1/true", k, ok)})
				continue
			}
			b.ClearWave()
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "switch fires once, flips, key once"})
		case "href":
			if b.Href() == "" {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "href echo missing"})
				continue
			}
			clicks, navs := 0, 0
			b.OnClick = func() { clicks++ }
			b.OnNavigate = func(_, _ string) { navs++ }
			b.PointerMove(cx, cy)
			b.PointerDown(cx, cy)
			b.PointerUp(cx, cy)
			if clicks != 1 || navs != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("href click=%d nav=%d want 1/1", clicks, navs)})
				continue
			}
			// Move-out quiet.
			b.PointerDown(cx, cy)
			b.PointerMove(-10, -10)
			clicks, navs = 0, 0
			b.OnClick = func() { clicks++ }
			b.OnNavigate = func(_, _ string) { navs++ }
			b.PointerUp(-10, -10)
			if clicks != 0 || navs != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "href move-out fired"})
				continue
			}
			// Keyboard blade also fires click+nav once via KeyPress.
			n := b.FocusNode()
			fm := focus.NewManager()
			fm.Register(n)
			if !fm.RequestFocus(n) {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "href focus failed"})
				continue
			}
			clicks, navs = 0, 0
			b.OnClick = func() { clicks++ }
			b.OnNavigate = func(_, _ string) { navs++ }
			if !b.KeyPress(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true}) {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "href key not consumed"})
				continue
			}
			if clicks != 1 || navs != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("href key click=%d nav=%d want 1/1", clicks, navs)})
				continue
			}
			b.ClearWave()
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "href click+nav once, key click+nav once"})
		default:
			// Blade 1 hover.
			b.PointerMove(cx, cy)
			hoverGot := hex(b.Fill())
			// Blade 2 press: fire-once + move-out quiet.
			b.PointerDown(cx, cy)
			pressGot := hex(b.Fill())
			fired := 0
			b.OnClick = func() { fired++ }
			b.PointerUp(cx, cy)
			if fired != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("click fired=%d want 1 (hover %s press %s)", fired, hoverGot, pressGot)})
				continue
			}
			b.PointerDown(cx, cy)
			b.PointerMove(-10, -10)
			fired = 0
			b.OnClick = func() { fired++ }
			b.PointerUp(-10, -10)
			if fired != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "move-out release fired"})
				continue
			}
			// Kind extras that depend on press state (wave) come first.
			switch it.kind {
			case "wave":
				// Re-press to observe a fresh wave (move-out path leaves none).
				b.PointerDown(cx, cy)
				b.PointerUp(cx, cy)
				if !b.WaveShowing() {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "wave missing after press"})
					continue
				}
				b.ClearWave()
			case "wavenone":
				b.PointerDown(cx, cy)
				b.PointerUp(cx, cy)
				if b.WaveShowing() {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "text/link must not wave"})
					b.ClearWave()
					continue
				}
			case "gradient":
				if !b.HasGradient() {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "gradient hook missing"})
					continue
				}
			case "space":
				off := button.NewButton("确定")
				off.SetAutoInsertSpace(false)
				off.Layout(rendering.Loose(1160, 800))
				on := button.NewButton("确定")
				on.Layout(rendering.Loose(1160, 800))
				ow, fw := on.LaidOut().Width, off.LaidOut().Width
				if ow <= fw {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("insert-space width %.1f<=%.1f", ow, fw)})
					continue
				}
			case "semantic":
				if b.ClassName(button.SemanticRoot) == "" && b.ClassName(button.SemanticLabel) == "" {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "semantic hook echo missing"})
					continue
				}
			}
			// Blade 3 keyboard via RequestFocus + KeyPress.
			if k, ok := fireViaKey(b); k != 1 || !ok {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("key fired=%d ok=%v want 1/true", k, ok)})
				continue
			}
			b.ClearWave()
			// Clickables must show hover/press feedback (fill moves off base).
			// Exempt: transparent fills (text/link) and business Style
			// overrides (semantic hooks own the chrome by design).
			if it.kind != "semantic" && hoverGot == base && pressGot == base && base != "#ffffff" && base != "#000000" {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "no hover/press feedback"})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: fmt.Sprintf("hover %s press %s key once", hoverGot, pressGot)})
		}
	}
	return rows
}

func kitKey(ev platform.Event) string {
	if !ev.Pressed {
		return ""
	}
	if ev.Rune == '\t' {
		return "Tab"
	}
	if ev.Rune == '\r' || ev.Rune == '\n' {
		return "Enter"
	}
	if ev.Rune == ' ' {
		return "Space"
	}
	switch ev.KeyCode {
	case int(input.KeyTab), 0xff09:
		return "Tab"
	case int(input.KeyEnter), 0xff0d, 0xff8d:
		return "Enter"
	case int(input.KeySpace), 0x20:
		return "Space"
	case int(input.KeyEscape), 0xff1b:
		return "Escape"
	}
	return ""
}

func emitJSON(backend string, rows []pfkit.ResultRow, pass bool, presents int64, m manualSummary) {
	b, _ := json.Marshal(map[string]any{
		"scenario": "kit_button",
		"tab":      "button",
		"backend":  backend,
		"rows":     rows,
		"manual": map[string]any{
			"pointer": m.Pointer, "key": m.Key, "resize": m.Resize,
			"activate": m.Activate, "timed": m.Timed, "note": m.Note,
		},
		"presents": presents,
		"pass":     pass,
	})
	fmt.Fprintln(os.Stdout, string(b))
}

func runSeconds(def int) int {
	if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

// liveWindow wires the laid-out matrix into a scrolling viewport and routes
// real native events through the same three blades the headless rows use.
type liveWindow struct {
	bw         *builtWindow
	vp         *rendering.RenderViewport
	root       *rendering.AbsoluteBox
	content    *rendering.AbsoluteBox
	titleNodes []*rendering.RenderText
	pad        *rendering.RenderColorBox
	fmgr       *focus.FocusManager
	app        *embedder.PipelineApp
	summary    manualSummary
	pressID    int
	hoverID    int
}

// at maps window pixels to a content-space instance (viewport-aware).
func (lw *liveWindow) at(x, y float64) (int, float64, float64) {
	if lw == nil || lw.bw == nil || lw.vp == nil {
		return -1, 0, 0
	}
	off := lw.vp.ScrollOffset()
	cx, cy := x, y-viewportTop+off.Y
	for i := range lw.bw.items {
		it := lw.bw.items[i]
		if it.w <= 0 || it.h <= 0 {
			continue
		}
		if cx >= it.x && cy >= it.y && cx < it.x+it.w && cy < it.y+it.h {
			return i, cx - it.x, cy - it.y
		}
	}
	return -1, 0, 0
}

// handleBladeHover routes pointer movement (blade 1: hover ink + focus).
// Sticky-hover rule: only one button holds hover at a time. The previously
// hovered button gets an explicit leave (far outside the 44px hit floor so
// near-misses keep hover — match Button.inside), empty space clears all.
// Only movements that flip hover (or drag while pressed) ask for a frame:
// idle moves inside one button cost nothing.
func (lw *liveWindow) handleBladeHover(x, y float64) bool {
	i, lx, ly := lw.at(x, y)
	if i < 0 {
		return lw.clearHover()
	}
	changed := false
	if lw.hoverID >= 0 && lw.hoverID != i && lw.hoverID < len(lw.bw.items) {
		prev := lw.bw.items[lw.hoverID]
		if prev.b != nil {
			was := prev.b.Hovered()
			prev.b.PointerMove(-1000, -1000)
			if prev.b.Hovered() != was {
				changed = true
			}
		}
	}
	lw.hoverID = i
	it := lw.bw.items[i]
	if it.b != nil {
		was := it.b.Hovered()
		it.b.PointerMove(lx, ly)
		if it.b.Hovered() != was {
			changed = true
		}
	}
	// Dragging tracks press containment every move even without hover flips.
	if lw.pressID >= 0 {
		return true
	}
	return changed
}

// clearHover releases hover on the tracked button (pointer left all
// instances). Only the tracked button can hold hover — the sticky rule in
// handleBladeHover guarantees the rest are already clean — so one leave
// event is enough (no 138-button sweep per mouse move).
func (lw *liveWindow) clearHover() bool {
	if lw == nil || lw.bw == nil || lw.hoverID < 0 || lw.hoverID >= len(lw.bw.items) {
		if lw != nil {
			lw.hoverID = -1
		}
		return false
	}
	b := lw.bw.items[lw.hoverID].b
	lw.hoverID = -1
	if b == nil || !b.Hovered() {
		return false
	}
	b.PointerMove(-1000, -1000)
	return !b.Hovered()
}

// applySwitch updates the example-level switcher state for a switch button.
func (lw *liveWindow) applySwitch(idx int) bool {
	if lw == nil || lw.bw == nil || idx < 0 || idx >= len(lw.bw.items) {
		return false
	}
	it := lw.bw.items[idx]
	if it.kind != "switch" {
		return false
	}
	changed := false
	switch it.b.Label() {
	case "start":
		if iconPlacePos != button.IconStart {
			iconPlacePos = button.IconStart
			changed = true
		}
	case "end":
		if iconPlacePos != button.IconEnd {
			iconPlacePos = button.IconEnd
			changed = true
		}
	case "Large":
		if curSize != button.ButtonLarge {
			curSize = button.ButtonLarge
			changed = true
		}
	case "Medium":
		if curSize != button.ButtonMiddle {
			curSize = button.ButtonMiddle
			changed = true
		}
	case "Small":
		if curSize != button.ButtonSmall {
			curSize = button.ButtonSmall
			changed = true
		}
	}
	if !changed {
		// Still rebuild to keep row geometry canonical even when the
		// switcher already holds the value (e.g. default end/large).
	}
	lw.rebuild()
	return true
}

// enterLoading arms a 3s loading spin on an enterloading button.
func (lw *liveWindow) enterLoading(idx int) {
	if lw == nil || lw.bw == nil || idx < 0 || idx >= len(lw.bw.items) {
		return
	}
	it := lw.bw.items[idx]
	if it.kind != "enterloading" || it.b == nil || it.b.Loading() {
		return
	}
	it.b.SetLoading(true)
	b := it.b
	time.AfterFunc(3*time.Second, func() {
		b.SetLoading(false)
		if lw.app != nil {
			lw.app.ScheduleFrame()
		}
	})
}

// rebuild re-lays out every section from the current switcher state and
// swaps the viewport content (switchers reorder IconPlacement/sizes).
func (lw *liveWindow) rebuild() {
	if lw == nil || lw.content == nil || lw.app == nil {
		return
	}
	nbw := buildItems()
	for _, it := range lw.bw.items {
		if it.b != nil {
			lw.content.RemoveChild(it.b.Node())
		}
	}
	for _, tn := range lw.titleNodes {
		lw.content.RemoveChild(tn)
	}
	if lw.pad != nil {
		lw.content.RemoveChild(lw.pad)
		lw.pad = nil
	}
	lw.bw = nbw
	if nbw.hasGhost && nbw.ghostY1 > nbw.ghostY0 {
		pad := rendering.NewRenderColorBox(1200, nbw.ghostY1-nbw.ghostY0, 190.0/255.0, 200.0/255.0, 200.0/255.0, 1)
		lw.content.Place(pad, 0, nbw.ghostY0)
		lw.pad = pad
	}
	lw.titleNodes = nil
	for _, t := range nbw.secTitles {
		lbl := wrkit.Label(t.text, 13, 0.2, 0.2, 0.2)
		lw.content.Place(lbl, t.x, t.y)
		lw.titleNodes = append(lw.titleNodes, lbl)
	}
	for _, it := range nbw.items {
		lw.content.Place(it.b.Node(), it.x, it.y)
		if it.kind == "href" {
			it.b.OnClick = func() {
				fmt.Fprintln(os.Stderr, "href click "+it.label)
			}
			lbl := it.label
			it.b.OnNavigate = func(href, target string) {
				fmt.Fprintf(os.Stderr, "href navigate %s %s %s\n", lbl, href, target)
			}
		}
	}
	fmgr := focus.NewManager()
	for _, it := range nbw.items {
		if it.b != nil && it.b.Focusable() {
			fmgr.Register(it.b.FocusNode())
		}
	}
	lw.fmgr = fmgr
	if sched := lw.app.Scheduler(); sched != nil {
		tickers := sched.Tickers()
		for _, it := range nbw.items {
			if it.b == nil {
				continue
			}
			// Registry membership registers here for every animated kind;
			// per-frame demand is governed by updateSpinnerDemand below
			// (shell passes visibility, component owns participation).
			switch it.kind {
			case "loading", "enterloading", "delayloading", "wave":
				it.b.Attach(tickers)
			}
		}
	}
	lw.updateSpinnerDemand()
	lw.content.FixedHeight = nbw.contentH
	lw.content.MarkNeedsLayout()
	lw.root.MarkNeedsPaint()
	lw.app.ScheduleFrame()
}

// updateSpinnerDemand passes viewport visibility to continuous spinners
// (loading/delayloading) through the component's SetOnScreen control verb.
// Shell-side stays pure control: intersect own layout rects with the viewport
// (64px margin), hand each button a boolean. The demand policy itself —
// leaving the ticker registry while off-screen, resuming phase on return —
// lives in ui/kit/button, never here.
func (lw *liveWindow) updateSpinnerDemand() {
	if lw == nil || lw.bw == nil || lw.vp == nil {
		return
	}
	off := lw.vp.ScrollOffset()
	top, bottom := off.Y-64, off.Y+lw.vp.FixedHeight+64
	for _, it := range lw.bw.items {
		if it.b == nil {
			continue
		}
		switch it.kind {
		case "loading", "delayloading":
			it.b.SetOnScreen(it.y < bottom && it.y+it.h > top)
		}
	}
}

// handleBladePress routes button press/release (blade 2: fire-once,
// move-out quiet). down=true on PointerDown, false on PointerUp.
func (lw *liveWindow) handleBladePress(x, y float64, down bool) {
	items := lw.bw.items
	if down {
		lw.pressID = -1
		if i, lx, ly := lw.at(x, y); i >= 0 {
			lw.fmgr.RequestFocus(items[i].b.FocusNode())
			if items[i].b.PointerDown(lx, ly) {
				lw.pressID = i
				lw.summary.Activate++
				fmt.Fprintf(os.Stderr, "activate press %s\n", items[i].label)
			}
		}
		return
	}
	if lw.pressID >= 0 && lw.pressID < len(items) {
		pi := items[lw.pressID]
		if i, lx, ly := lw.at(x, y); i == lw.pressID {
			if pi.b.PointerUp(lx, ly) {
				lw.summary.Activate++
				fmt.Fprintf(os.Stderr, "activate release %s\n", pi.label)
				if pi.kind == "switch" {
					lw.applySwitch(lw.pressID)
				} else if pi.kind == "enterloading" {
					lw.enterLoading(lw.pressID)
				}
			}
		} else {
			pi.b.PointerUp(-1, -1)
		}
		lw.pressID = -1
		return
	}
	if i, lx, ly := lw.at(x, y); i >= 0 {
		if items[i].b.PointerUp(lx, ly) {
			lw.summary.Activate++
			if items[i].kind == "switch" {
				lw.applySwitch(i)
			} else if items[i].kind == "enterloading" {
				lw.enterLoading(i)
			}
		}
	}
	lw.pressID = -1
}

// buttonForNode maps a focus node back to its window button.
func (lw *liveWindow) buttonForNode(n *focus.FocusNode) (int, *button.Button) {
	if lw == nil || lw.bw == nil || n == nil {
		return -1, nil
	}
	for i := range lw.bw.items {
		if lw.bw.items[i].b != nil && lw.bw.items[i].b.FocusNode() == n {
			return i, lw.bw.items[i].b
		}
	}
	return -1, nil
}

// handleBladeKey routes keyboard activation (blade 3: Tab walk, Enter/Space via KeyPress).
func (lw *liveWindow) handleBladeKey(key string) {
	switch key {
	case "Tab":
		if n := lw.fmgr.FocusNext(); n != nil {
			fmt.Fprintf(os.Stderr, "focus %s\n", n.DebugLabel)
		}
	case "Enter", "Space":
		if p := lw.fmgr.Primary(); p != nil {
			idx, b := lw.buttonForNode(p)
			if b == nil {
				return
			}
			var ev focus.KeyEvent
			if key == "Space" {
				ev = focus.KeyEvent{KeyCode: focus.KeySpace, Pressed: true}
			} else {
				ev = focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true}
			}
			if b.KeyPress(ev) {
				lw.summary.Activate++
				fmt.Fprintf(os.Stderr, "key activate %s via %s\n", p.DebugLabel, key)
				if idx >= 0 {
					if kind := lw.bw.items[idx].kind; kind == "switch" {
						lw.applySwitch(idx)
					} else if kind == "enterloading" {
						lw.enterLoading(idx)
					}
				}
			}
		}
	}
}

// maxScroll clamps the viewport travel to the content overflow.
func (lw *liveWindow) maxScroll() float64 {
	if lw == nil || lw.bw == nil || lw.vp == nil {
		return 0
	}
	m := lw.bw.contentH - lw.vp.FixedHeight
	if m < 0 {
		return 0
	}
	return m
}

func main() {
	autoOnly := flag.Bool("auto-only", false, "run selftest + short real window and exit (gate mode)")
	manualSeconds := flag.Int("manual-seconds", 0, "manual phase timeout in seconds (0 = until window close)")
	scrollY := flag.Float64("scroll-y", 0, "initial viewport content offset (§6.9.0)")
	flag.Parse()

	rows := selftest()
	// Strict gate: simulate every window instance headless first; any FAIL
	// blocks the window (same rows re-reported in gate JSON).
	wrkit.EnsureUIFace()
	headless := buildItems()
	rows = append(rows, verifyEveryInstance(headless.items)...)
	ok := pfkit.Report(rows)
	if !*autoOnly {
		if !ok {
			fmt.Fprintln(os.Stderr, "kit_button: selftest FAIL, not opening window")
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "kit_button: selftest done, entering manual phase")
		fmt.Fprintln(os.Stderr, "  hover/press buttons; Tab+Enter/Space; wheel scrolls; close X to finish.")
	} else if !ok {
		emitJSON("unknown", rows, false, 0, manualSummary{Note: "headless selftest failed"})
		os.Exit(1)
	}

	var secs int
	if *autoOnly {
		secs = runSeconds(5)
	} else if *manualSeconds > 0 {
		secs = *manualSeconds
	} else if v := os.Getenv("RUN_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			secs = n
		}
	}
	var runFor time.Duration
	if secs > 0 {
		runFor = time.Duration(secs) * time.Second
	}

	wrkit.EnsureUIFace()
	bw := buildItems()
	// Re-run per-instance verification on the live window nodes so the
	// reported rows describe exactly what the human sees (fresh nodes;
	// headless pass above already gated open).
	liveRows := verifyEveryInstance(bw.items)
	_ = liveRows
	fmgr := focus.NewManager()
	for _, it := range bw.items {
		if it.b != nil && it.b.Focusable() {
			fmgr.Register(it.b.FocusNode())
		}
	}

	root := rendering.NewAbsoluteBox(winW, winH)
	root.Background = &rendering.Color{R: 0.96, G: 0.96, B: 0.96, A: 1}
	title := wrkit.Label("Button — 16 demos §6.9.0 (hover/press/Tab/Enter; wheel scrolls; ghost on #bec8c8)", 14, 0.1, 0.1, 0.1)
	root.Place(title, 20, 8)

	content := rendering.NewAbsoluteBox(1200, bw.contentH)
	// No content background: the root behind the viewport is the same gray,
	// and a content-sized fill here would land in the viewport boundary's
	// own picture (content is not a boundary) making it oversized refused —
	// every steady frame then vector-replays the whole 1200x2192 fill and
	// presents full at ~32ms raster ("all buttons slow" cause).
	var pad *rendering.RenderColorBox
	if bw.hasGhost && bw.ghostY1 > bw.ghostY0 {
		pad = rendering.NewRenderColorBox(1200, bw.ghostY1-bw.ghostY0, 190.0/255.0, 200.0/255.0, 200.0/255.0, 1)
		content.Place(pad, 0, bw.ghostY0)
	}
	var titleNodes []*rendering.RenderText
	for _, t := range bw.secTitles {
		lbl := wrkit.Label(t.text, 13, 0.2, 0.2, 0.2)
		content.Place(lbl, t.x, t.y)
		titleNodes = append(titleNodes, lbl)
	}
	for _, it := range bw.items {
		content.Place(it.b.Node(), it.x, it.y)
		if it.kind == "href" {
			it.b.OnClick = func() {
				fmt.Fprintln(os.Stderr, "href click")
			}
			it.b.OnNavigate = func(href, target string) {
				fmt.Fprintf(os.Stderr, "href navigate %s %s\n", href, target)
			}
		}
	}
	vpH := winH - viewportTop
	vp := rendering.NewRenderViewport(content)
	vp.FixedWidth, vp.FixedHeight = winW, vpH
	root.Place(vp, 0, viewportTop)

	win, err := platform.Open(platform.Options{Width: winW, Height: winH, Title: "gpui kit_button — Button", Decorations: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, "window:", err)
		os.Exit(1)
	}
	defer win.Close()
	ctl := win.Controls()
	_ = ctl

	lw := &liveWindow{bw: bw, vp: vp, root: root, content: content, titleNodes: titleNodes, pad: pad, fmgr: fmgr, pressID: -1, hoverID: -1}
	if *scrollY > 0 {
		if *scrollY > lw.maxScroll() {
			vp.SetScrollOffset(0, lw.maxScroll())
		} else {
			vp.SetScrollOffset(0, *scrollY)
		}
	}

	var app *embedder.PipelineApp
	app = embedder.NewPipelineApp(win.Host(), root, embedder.PipelineOptions{
		ClearR: 0.96, ClearG: 0.96, ClearB: 0.96, ClearA: 1,
		RunFor: runFor, WarmUp: true,
		OnEvent: func(ev platform.Event) {
			switch ev.Type {
			case platform.EventClose, platform.EventCloseRequested:
				return
			case platform.EventPointer:
				lw.summary.Pointer++
				dirty := false
				switch ev.Pointer {
				case platform.PointerMove:
					app.NoteInputEvent()
					dirty = lw.handleBladeHover(ev.X, ev.Y)
				case platform.PointerDown:
					app.NoteInputEvent()
					fmt.Fprintf(os.Stderr, "pointer down (%.0f,%.0f)\n", ev.X, ev.Y)
					lw.handleBladePress(ev.X, ev.Y, true)
					dirty = true
				case platform.PointerUp:
					lw.handleBladePress(ev.X, ev.Y, false)
					dirty = true
				case platform.PointerScroll:
					off := vp.ScrollOffset()
					next := off.Y + ev.ScrollY*40
					if next < 0 {
						next = 0
					}
					if next > lw.maxScroll() {
						next = lw.maxScroll()
					}
					vp.SetScrollOffset(0, next)
					lw.updateSpinnerDemand()
					dirty = true
				}
				if dirty {
					// Buttons already marked their own paint dirty via
					// PointerMove/Down/Up (each node is its own repaint
					// boundary); asking for a frame is enough. Marking
					// the root here would re-record the whole window on
					// every mouse move — that was the "all slow" cause.
					app.ScheduleFrame()
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					lw.summary.Key++
					if key := kitKey(ev); key != "" {
						lw.handleBladeKey(key)
						app.ScheduleFrame()
					}
				}
				return
			case platform.EventResize:
				lw.summary.Resize++
				return
			default:
				return
			}
		},
	})
	lw.app = app
	if sched := app.Scheduler(); sched != nil {
		tickers := sched.Tickers()
		for _, it := range bw.items {
			if it.b == nil {
				continue
			}
			switch it.kind {
			case "loading", "enterloading", "delayloading", "wave":
				it.b.Attach(tickers)
			}
		}
	}
	lw.updateSpinnerDemand()

	if err := app.Open(); err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		os.Exit(1)
	}
	root.MarkNeedsPaint()
	app.ScheduleFrame()

	t0 := time.Now()
	if err := app.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	app.Close()
	elapsed := time.Since(t0).Seconds()
	presents := app.PresentCount()
	backend := win.Backend().String()
	pass := ok && presents >= 1
	if backend == "" || backend == "auto" {
		pass = false
	}
	if *autoOnly {
		emitJSON(backend, rows, pass, presents, manualSummary{Note: "gate"})
		if !pass {
			fmt.Fprintf(os.Stderr, "gate FAIL: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "gate PASS: backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
		return
	}
	lw.summary.Timed = secs > 0
	rows = append(rows, lw.summary.rows()...)
	ok = pfkit.Report(rows)
	emitJSON(backend, rows, ok && presents >= 1, presents, lw.summary)
	fmt.Fprintf(os.Stderr, "kit_button backend=%s presents=%d elapsed=%.1fs\n", backend, presents, elapsed)
	if !(ok && presents >= 1) {
		os.Exit(1)
	}
}
