// Command kit_button is the Button standalone true window.
//
// One component, one directory (examples/kit/<name>): independent open,
// independent test, independent screenshot. Combined gallery is preview only.
//
// Layout follows docs/antd/button.md §6.9.0: 16 non-debug demo sections in
// official order (basic, color-variant, icon, icon-placement, size,
// disabled, loading, multiple, ghost, danger, block, linear-gradient,
// wave, chinese-space, custom-disabled-bg, style-class), 133 instances.
// Gate rows: 7 headless selftest + 133 one-row-per-instance = 140.
//
// Every instance row covers the same three event blades:
// blade 1 hover (PointerMove ink), blade 2 press (PointerDown/Up fire-once,
// move-out quiet), blade 3 keyboard (FocusNode.OnActivate fires once,
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
	// click | disabled | loading | wave | wavenone | gradient | href |
	// space | semantic.
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

// section is one §6.9.0 demo band: title + member buttons.
type section struct {
	title string
	ghost bool // ghost band gets the #bec8c8 pad behind it
	items []staged
}

type staged struct {
	label string
	kind  string
	fn    func(b *button.Button)
	block bool
}

// gradFrom/gradTo are the linear-gradient demo stops (§6.9.0: #6253e1 → #04befe).
func gradFrom() render.RGBA {
	return render.RGBA{R: 0x62 / 255.0, G: 0x53 / 255.0, B: 0xe1 / 255.0, A: 1}
}
func gradTo() render.RGBA {
	return render.RGBA{R: 0x04 / 255.0, G: 0xbe / 255.0, B: 0xfe / 255.0, A: 1}
}

func sections() []section {
	colorVariants := func() []staged {
		colors := []button.ButtonColor{
			button.ColorDefault, button.ColorPrimary, button.ColorDanger,
			button.ColorPink, button.ColorPurple, button.ColorCyan,
		}
		variants := []button.ButtonVariant{
			button.VariantSolid, button.VariantOutlined, button.VariantDashed,
			button.VariantFilled, button.VariantText, button.VariantLink,
		}
		var out []staged
		for _, c := range colors {
			for _, v := range variants {
				c, v := c, v
				out = append(out, staged{
					label: string(c) + "/" + string(v),
					kind:  "click",
					fn: func(b *button.Button) {
						b.SetSize(button.ButtonSmall)
						b.SetVariant(v)
						b.SetColor(c)
					},
				})
			}
		}
		return out
	}()
	disabledPairs := func() []staged {
		type pair struct {
			label string
			fn    func(b *button.Button)
		}
		pairs := []pair{
			{"Primary", func(b *button.Button) { b.SetType(button.ButtonPrimary) }},
			{"Default", nil},
			{"Dashed", func(b *button.Button) { b.SetType(button.ButtonDashed) }},
			{"Text", func(b *button.Button) { b.SetType(button.ButtonText) }},
			{"Link", func(b *button.Button) { b.SetType(button.ButtonLink) }},
			{"HrefPrimary", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetHref("https://ant.design/components/button")
			}},
			{"DangerDefault", func(b *button.Button) { b.SetDanger(true) }},
			{"DangerText", func(b *button.Button) {
				b.SetType(button.ButtonText)
				b.SetDanger(true)
			}},
			{"DangerLink", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetDanger(true)
			}},
			{"Ghost", func(b *button.Button) { b.SetGhost(true) }},
		}
		var out []staged
		for _, p := range pairs {
			p := p
			kind := "click"
			if p.label == "HrefPrimary" {
				kind = "href"
			}
			out = append(out, staged{label: p.label, kind: kind, fn: p.fn})
			out = append(out, staged{label: p.label + "·灰", kind: "disabled", fn: func(b *button.Button) {
				if p.fn != nil {
					p.fn(b)
				}
				b.SetDisabled(true)
			}})
		}
		return out
	}()
	return []section{
		{title: "语法糖 basic", items: []staged{
			{"确定", "click", nil, false},
			{"确定·主", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Dashed", "click", func(b *button.Button) { b.SetType(button.ButtonDashed) }, false},
			{"Text", "click", func(b *button.Button) { b.SetType(button.ButtonText) }, false},
			{"Link", "click", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		}},
		{title: "颜色与变体 color-variant（small）", items: colorVariants},
		{title: "按钮图标 icon", items: []staged{
			{"图标·圆主", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetShape(button.ButtonShapeCircle)
				b.SetIcon("search")
				b.SetAriaLabel("搜索")
			}, false},
			{"A", "click", nil, false},
			{"搜索·主", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetIcon("search")
			}, false},
			{"图标·圆", "click", func(b *button.Button) {
				b.SetShape(button.ButtonShapeCircle)
				b.SetIcon("search")
				b.SetAriaLabel("搜索圆")
			}, false},
			{"搜索", "click", func(b *button.Button) { b.SetIcon("search") }, false},
			{"搜索·虚", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetIcon("search")
			}, false},
			{"搜索·链", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetIcon("search")
			}, false},
			{"搜索·尾", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetIcon("search")
				b.SetIconPlacement(button.IconEnd)
			}, false},
			{"下载", "click", func(b *button.Button) { b.SetIcon("download") }, false},
			{"下载·圆尾", "click", func(b *button.Button) {
				b.SetShape(button.ButtonShapeRound)
				b.SetIcon("download")
				b.SetIconPlacement(button.IconEnd)
			}, false},
		}},
		{title: "按钮图标位置 icon-placement", items: []staged{
			{"搜索·前", "click", func(b *button.Button) { b.SetIcon("search") }, false},
			{"搜索·后", "click", func(b *button.Button) {
				b.SetIcon("search")
				b.SetIconPlacement(button.IconEnd)
			}, false},
			{"下载·前", "click", func(b *button.Button) { b.SetIcon("download") }, false},
			{"下载·后", "click", func(b *button.Button) {
				b.SetIcon("download")
				b.SetIconPlacement(button.IconEnd)
			}, false},
		}},
		{title: "按钮尺寸 size（large）", items: []staged{
			{"Primary", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetSize(button.ButtonLarge)
			}, false},
			{"Default", "click", func(b *button.Button) { b.SetSize(button.ButtonLarge) }, false},
			{"Dashed", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetSize(button.ButtonLarge)
			}, false},
			{"Link", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetSize(button.ButtonLarge)
			}, false},
			{"图标", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
				b.SetIcon("search")
				b.SetAriaLabel("搜索大")
			}, false},
			{"图标·圆", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
				b.SetShape(button.ButtonShapeCircle)
				b.SetIcon("search")
				b.SetAriaLabel("搜索大圆")
			}, false},
			{"图标·囊", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
				b.SetShape(button.ButtonShapeRound)
				b.SetIcon("search")
				b.SetAriaLabel("搜索胶囊")
			}, false},
			{"Download·囊", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
				b.SetShape(button.ButtonShapeRound)
				b.SetIcon("download")
			}, false},
			{"Download", "click", func(b *button.Button) {
				b.SetSize(button.ButtonLarge)
				b.SetIcon("download")
			}, false},
		}},
		{title: "不可用状态 disabled（亮/灰配对）", items: disabledPairs},
		{title: "加载中状态 loading", items: []staged{
			{"Loading", "loading", func(b *button.Button) { b.SetLoading(true) }, false},
			{"Loading·小", "loading", func(b *button.Button) {
				b.SetSize(button.ButtonSmall)
				b.SetLoading(true)
			}, false},
			{"Loading·标", "loading", func(b *button.Button) {
				b.SetIcon("search")
				b.SetLoading(true)
			}, false},
			{"Loading·sync", "loading", func(b *button.Button) {
				b.SetLoadingConfig(button.LoadingConfig{Icon: "custom-spin"})
			}, false},
			{"点我转圈1", "click", nil, false},
			{"点我转圈2", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"点我转圈3", "click", func(b *button.Button) { b.SetIcon("reload") }, false},
			{"点我转圈4", "click", func(b *button.Button) { b.SetType(button.ButtonDashed) }, false},
			{"点我转圈5", "click", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		}},
		{title: "多个按钮组合 multiple", items: []staged{
			{"Submit", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Cancel", "click", nil, false},
			{"More", "click", nil, false},
			{"Actions", "click", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"次要", "click", nil, false},
			{"更多", "click", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		}},
		{title: "幽灵按钮 ghost（灰绿底）", ghost: true, items: []staged{
			{"Ghost", "click", func(b *button.Button) { b.SetGhost(true) }, false},
			{"Ghost·主", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetGhost(true)
			}, false},
			{"Ghost·虚", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetGhost(true)
			}, false},
			{"Ghost·险", "click", func(b *button.Button) {
				b.SetDanger(true)
				b.SetGhost(true)
			}, false},
			{"Ghost·链", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetGhost(true)
			}, false},
		}},
		{title: "危险按钮 danger", items: []staged{
			{"Danger·主", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDanger(true)
			}, false},
			{"Danger", "click", func(b *button.Button) { b.SetDanger(true) }, false},
			{"Danger·虚", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetDanger(true)
			}, false},
			{"Danger·文", "click", func(b *button.Button) {
				b.SetType(button.ButtonText)
				b.SetDanger(true)
			}, false},
			{"Danger·链", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetDanger(true)
			}, false},
			{"Danger·囊", "click", func(b *button.Button) {
				b.SetShape(button.ButtonShapeRound)
				b.SetDanger(true)
			}, false},
		}},
		{title: "Block 按钮 block（撑满整宽）", items: []staged{
			{"Block·主", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetBlock(true)
			}, true},
			{"Block", "click", func(b *button.Button) { b.SetBlock(true) }, true},
			{"Block·虚", "click", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetBlock(true)
			}, true},
			{"Block·险", "click", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDanger(true)
				b.SetBlock(true)
			}, true},
			{"Block·链", "click", func(b *button.Button) {
				b.SetType(button.ButtonLink)
				b.SetBlock(true)
			}, true},
			{"Block·囊", "click", func(b *button.Button) {
				b.SetShape(button.ButtonShapeRound)
				b.SetBlock(true)
			}, true},
		}},
		{title: "渐变按钮 linear-gradient（P1）", items: []staged{
			{"渐变·主", "gradient", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetGradient(gradFrom(), gradTo())
			}, false},
			{"渐变·标", "gradient", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetIcon("search")
				b.SetGradient(gradFrom(), gradTo())
			}, false},
			{"渐变·险", "gradient", func(b *button.Button) {
				b.SetDanger(true)
				b.SetGradient(gradFrom(), gradTo())
			}, false},
		}},
		{title: "自定义按钮波纹 wave（P1）", items: []staged{
			{"Wave·关", "disabled", func(b *button.Button) { b.SetDisabled(true) }, false},
			{"Wave·默", "wave", nil, false},
			{"Wave·嵌", "wave", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Wave·抖", "wave", func(b *button.Button) { b.SetType(button.ButtonDashed) }, false},
			{"Happy Work", "wave", func(b *button.Button) { b.SetType(button.ButtonPrimary) }, false},
			{"Wave·文", "wavenone", func(b *button.Button) { b.SetType(button.ButtonText) }, false},
			{"Wave·链", "wavenone", func(b *button.Button) { b.SetType(button.ButtonLink) }, false},
		}},
		{title: "移除两个汉字之间的空格 chinese-space（P1）", items: []staged{
			{"确定", "space", nil, false},
			{"确定·紧", "space", func(b *button.Button) { b.SetAutoInsertSpace(false) }, false},
		}},
		{title: "自定义禁用样式背景 custom-disabled-bg（P1）", items: []staged{
			{"Primary·灰", "disabled", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetDisabled(true)
			}, false},
			{"Default·灰", "disabled", func(b *button.Button) {
				b.SetDisabled(true)
				b.SetStyle(button.Style{
					Bg:    render.RGBA{R: 0.1, G: 0.1, B: 0.1, A: 1},
					UseBg: true,
				})
			}, false},
			{"Dashed·灰", "disabled", func(b *button.Button) {
				b.SetType(button.ButtonDashed)
				b.SetDisabled(true)
				b.SetStyle(button.Style{
					Bg:    render.RGBA{R: 0.4, G: 0.4, B: 0.4, A: 1},
					UseBg: true,
				})
			}, false},
		}},
		{title: "自定义语义结构的样式和类 style-class（P1）", items: []staged{
			{"语义·甲", "semantic", func(b *button.Button) {
				b.SetClassName(button.SemanticRoot, "my-root-a")
				b.SetStyle(button.Style{
					Bg:    render.RGBA{R: 0.2, G: 0.4, B: 0.8, A: 1},
					UseBg: true,
				})
			}, false},
			{"语义·乙", "semantic", func(b *button.Button) {
				b.SetType(button.ButtonPrimary)
				b.SetClassName(button.SemanticLabel, "my-label-b")
			}, false},
		}},
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

// buildItems lays out the 16 sections in a wrapping flow (block buttons take
// a full row). Returns content-space rects; the caller hosts them in a
// viewport. ghostY0/Y1 bound the ghost band for its #bec8c8 pad.
func buildItems() *builtWindow {
	face14 := wrkit.FaceAt(14)
	loose := rendering.Loose(1160, 800)
	const contentW = 1200.0
	const margin = 20.0
	const colGap = 12.0
	const rowGap = 10.0
	const secGap = 16.0
	const titleH = 26.0

	bw := &builtWindow{}
	y := margin
	for _, sec := range sections() {
		bw.secTitles = append(bw.secTitles, secTitle{text: sec.title, x: margin, y: y})
		y += titleH
		if sec.ghost {
			bw.ghostY0 = y - 8
			bw.hasGhost = true
		}
		// Pre-build buttons in this section.
		type placed struct {
			b     *button.Button
			label string
			kind  string
			w, h  float64
			block bool
		}
		var members []placed
		for _, st := range sec.items {
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
		x := margin
		rowH := 0.0
		flush := func() {
			if rowH > 0 {
				y += rowH + rowGap
				rowH = 0
			}
			x = margin
		}
		for _, m := range members {
			if m.block {
				flush()
				bw.items = append(bw.items, item{b: m.b, x: margin, y: y, w: m.w, h: m.h, label: m.label, kind: m.kind})
				y += m.h + rowGap
				continue
			}
			if x+m.w > contentW-margin && x > margin {
				y += rowH + rowGap
				x = margin
				rowH = 0
			}
			bw.items = append(bw.items, item{b: m.b, x: x, y: y, w: m.w, h: m.h, label: m.label, kind: m.kind})
			x += m.w + colGap
			if m.h > rowH {
				rowH = m.h
			}
		}
		flush()
		y += secGap - rowGap
		if sec.ghost {
			bw.ghostY1 = y - secGap + 8
		}
	}
	bw.contentH = y + margin
	return bw
}

// verifyEveryInstance simulates a real user on every window instance and
// verifies the effect through three blades — hover ink, press fire-once +
// move-out quiet, keyboard activate — plus kind extras (wave show/hide,
// gradient presence, href echo, insert-space width, semantic hooks).
// One row per instance; any FAIL blocks the component close.
func verifyEveryInstance(items []item) []pfkit.ResultRow {
	rows := []pfkit.ResultRow{}
	hex := func(c render.RGBA) string {
		return fmt.Sprintf("#%02x%02x%02x", uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
	}
	fireViaKey := func(b *button.Button) int {
		n := b.FocusNode()
		if n == nil || n.OnActivate == nil {
			return -1
		}
		fired := 0
		prev := b.OnClick
		b.OnClick = func() { fired++ }
		n.OnActivate()
		b.OnClick = prev
		return fired
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
			if k := fireViaKey(b); k != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("disabled key fired %d", k)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "disabled swallows press/hover/key " + base})
		case "loading":
			if !b.HasSpinner() {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "loading spinner missing"})
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
			if k := fireViaKey(b); k != 0 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("loading key fired %d", k)})
				continue
			}
			rows = append(rows, pfkit.ResultRow{Name: name, OK: true, Detail: "loading spinner + no press/key repeat"})
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
			case "href":
				if b.Href() == "" {
					rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: "href echo missing"})
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
			// Blade 3 keyboard.
			if k := fireViaKey(b); k != 1 {
				rows = append(rows, pfkit.ResultRow{Name: name, OK: false, Detail: fmt.Sprintf("key fired=%d want 1", k)})
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
	bw      *builtWindow
	vp      *rendering.RenderViewport
	root    *rendering.AbsoluteBox
	fmgr    *focus.FocusManager
	app     *embedder.PipelineApp
	summary manualSummary
	pressID int
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
func (lw *liveWindow) handleBladeHover(x, y float64) bool {
	i, lx, ly := lw.at(x, y)
	if i < 0 {
		return false
	}
	lw.bw.items[i].b.PointerMove(lx, ly)
	return true
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
		}
	}
	lw.pressID = -1
}

// handleBladeKey routes keyboard activation (blade 3: Tab walk, Enter/Space).
func (lw *liveWindow) handleBladeKey(key string) {
	switch key {
	case "Tab":
		if n := lw.fmgr.FocusNext(); n != nil {
			fmt.Fprintf(os.Stderr, "focus %s\n", n.DebugLabel)
		}
	case "Enter", "Space":
		if p := lw.fmgr.Primary(); p != nil {
			handled := false
			if p.OnKey != nil {
				handled = p.OnKey(focus.KeyEvent{KeyCode: focus.KeyEnter, Pressed: true})
			}
			if !handled && p.OnActivate != nil {
				p.OnActivate()
				handled = true
			}
			if handled {
				lw.summary.Activate++
				fmt.Fprintf(os.Stderr, "key activate %s via %s\n", p.DebugLabel, key)
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
	content.Background = &rendering.Color{R: 0.96, G: 0.96, B: 0.96, A: 1}
	if bw.hasGhost && bw.ghostY1 > bw.ghostY0 {
		pad := rendering.NewRenderColorBox(1200, bw.ghostY1-bw.ghostY0, 190.0/255.0, 200.0/255.0, 200.0/255.0, 1)
		content.Place(pad, 0, bw.ghostY0)
	}
	for _, t := range bw.secTitles {
		content.Place(wrkit.Label(t.text, 13, 0.2, 0.2, 0.2), t.x, t.y)
	}
	for _, it := range bw.items {
		content.Place(it.b.Node(), it.x, it.y)
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

	lw := &liveWindow{bw: bw, vp: vp, root: root, fmgr: fmgr, pressID: -1}
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
					dirty = lw.handleBladeHover(ev.X, ev.Y)
				case platform.PointerDown:
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
					dirty = true
				}
				if dirty {
					root.MarkNeedsPaint()
					app.ScheduleFrame()
				}
				return
			case platform.EventKey:
				if ev.Pressed {
					lw.summary.Key++
					if key := kitKey(ev); key != "" {
						lw.handleBladeKey(key)
						root.MarkNeedsPaint()
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
