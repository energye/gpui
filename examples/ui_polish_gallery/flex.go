//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerFlex() {
	// Flex — antd demos: basic / align / gap / wrap / combination
	// https://ant.design/components/flex · components/flex/demo/*.tsx
	//
	// Responsive rules (match antd docs, avoid overflow outside parent):
	//   - playgrounds use ExpandWidth so they shrink with the tab body
	//   - wrap Flex must see a bounded MaxWidth (ExpandWidth host)
	//   - fixed 480/520/620 widths were causing children to paint past the card
	// Playground host: fills available width, clips overflow, optional fixed height.
	playground := func(body core.Node, h float64, border render.RGBA) *primitive.Decorated {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.StretchChild = true
		d.Radius = 6
		d.Padding = primitive.All(0)
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		if h > 0 {
			d.Height = h
		}
		if border.A > 0 {
			d.BorderWidth = 1
			d.BorderColor = border
		}
		return d
	}
	// antd basic.tsx bars: height 54.
	// horizontal → Flexible equal width (≈25%); vertical → fixed h=54, stretch width.
	mkBar := func(solid bool) *primitive.Decorated {
		d := primitive.NewDecorated(nil)
		d.Height = 54
		d.MinHeight = 54
		if solid {
			d.Background = render.Hex("#1677ff")
		} else {
			d.Background = render.Hex("#1677ffbf")
		}
		return d
	}
	basicBars := kit.NewFlex()
	fillBasicBars := func(vertical bool) {
		solids := []bool{false, true, false, true}
		kids := make([]core.Node, 0, 4)
		for _, solid := range solids {
			d := mkBar(solid)
			if vertical {
				kids = append(kids, d)
			} else {
				fl := primitive.NewFlexible(1, d)
				fl.FillChild = true
				kids = append(kids, fl)
			}
		}
		basicBars.SetChildren(kids...)
		basicBars.SetVertical(vertical)
		if vertical {
			basicBars.SetAlign(kit.FlexAlignStretch)
		} else {
			basicBars.SetAlign(kit.FlexAlignAuto)
		}
	}
	fillBasicBars(false)
	mkPrimaryBtn := func(label string) *kit.Button {
		b := kit.NewButton(label)
		b.SetFace(c.face)
		b.SetType(kit.ButtonPrimary)
		return c.trackBtn(b)
	}
	mkBtnTyped := func(label string, typ kit.ButtonType) *kit.Button {
		b := kit.NewButton(label)
		b.SetFace(c.face)
		b.SetType(typ)
		return c.trackBtn(b)
	}
	radioRow := func(g *kit.RadioGroup) {
		if fl, ok := g.Node().(*primitive.Flex); ok {
			fl.Axis = core.AxisHorizontal
			fl.CrossAlign = core.CrossCenter
			fl.Gap = 16
			fl.Wrap = true
			fl.MarkNeedsLayout()
		}
	}

	// ---------- basic.tsx ----------
	basicHost := playground(basicBars.Node(), 0, render.RGBA{})
	basicHost.Radius = 0

	rh := kit.NewRadio("horizontal", "horizontal")
	rv := kit.NewRadio("vertical", "vertical")
	rh.SetFace(c.face)
	rv.SetFace(c.face)
	basicRG := kit.NewRadioGroup(rh, rv)
	basicRG.Select("horizontal")
	radioRow(basicRG)
	basicRG.OnChange = func(v string) {
		fillBasicBars(v == "vertical")
		basicHost.MarkNeedsLayout()
		*c.status = "flex basic → " + v
	}
	basicOuter := kit.NewFlex(basicRG.Node(), basicHost)
	basicOuter.SetVertical(true)
	basicOuter.SetGapSize(kit.FlexGapMedium)
	secFlexBasic := demoSection(c.face, c.theme, "基本布局",
		"antd basic.tsx：Radio 切换 horizontal / vertical。色块高 54；水平均分宽度，垂直通栏堆叠。宿主 ExpandWidth，随窗口收缩。",
		basicOuter.Node())

	// ---------- align.tsx ----------
	alignBtns := kit.NewFlex(
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
		mkPrimaryBtn("Primary").Node(),
	)
	alignBtns.SetJustify(kit.FlexJustifyStart)
	alignBtns.SetAlign(kit.FlexAlignStart)
	// antd align.tsx: no wrap — align-items needs a single line in the tall playground
	alignBtns.SetWrap(false)
	alignBtns.SetGap(8)
	alignPlay := playground(alignBtns.Node(), 120, render.Hex("#40a9ff"))

	applyJustify := func(v string) {
		switch v {
		case "center":
			alignBtns.SetJustify(kit.FlexJustifyCenter)
		case "flex-end":
			alignBtns.SetJustify(kit.FlexJustifyEnd)
		case "space-between":
			alignBtns.SetJustify(kit.FlexJustifySpaceBetween)
		case "space-around":
			alignBtns.SetJustify(kit.FlexJustifySpaceAround)
		case "space-evenly":
			alignBtns.SetJustify(kit.FlexJustifySpaceEvenly)
		default:
			alignBtns.SetJustify(kit.FlexJustifyStart)
		}
		alignPlay.MarkNeedsLayout()
		*c.status = "flex justify → " + v
	}
	applyAlign := func(v string) {
		switch v {
		case "center":
			alignBtns.SetAlign(kit.FlexAlignCenter)
		case "flex-end":
			alignBtns.SetAlign(kit.FlexAlignEnd)
		default:
			alignBtns.SetAlign(kit.FlexAlignStart)
		}
		alignPlay.MarkNeedsLayout()
		*c.status = "flex align → " + v
	}
	justifySeg := kit.NewSegmented(
		"flex-start", "center", "flex-end", "space-between", "space-around", "space-evenly",
	)
	justifySeg.SetFace(c.face)
	justifySeg.OnChange = applyJustify
	alignSeg := kit.NewSegmented("flex-start", "center", "flex-end")
	alignSeg.SetFace(c.face)
	alignSeg.OnChange = applyAlign
	lblJ := kit.NewText("Select justify :")
	lblJ.SetFace(c.face)
	lblA := kit.NewText("Select align :")
	lblA.SetFace(c.face)
	alignOuter := kit.NewFlex(
		lblJ.Node(), justifySeg.Node(),
		lblA.Node(), alignSeg.Node(),
		alignPlay,
	)
	alignOuter.SetVertical(true)
	alignOuter.SetGapSize(kit.FlexGapMedium)
	alignOuter.SetAlign(kit.FlexAlignStart)
	secFlexAlign := demoSection(c.face, c.theme, "对齐方式",
		"antd align.tsx：Segmented 选 justify / align。场地高度 120、边框 #40a9ff、宽度 100%（ExpandWidth），缩窗口时按钮可 wrap，不跑出父容器。",
		alignOuter.Node())

	// ---------- gap.tsx ----------
	gapRow := kit.NewFlex(
		mkBtnTyped("Primary", kit.ButtonPrimary).Node(),
		mkBtnTyped("Default", kit.ButtonDefault).Node(),
		mkBtnTyped("Dashed", kit.ButtonDashed).Node(),
		mkBtnTyped("Link", kit.ButtonLink).Node(),
	)
	gapRow.SetGapSize(kit.FlexGapSmall)
	gapRow.SetWrap(true)

	rSmall := kit.NewRadio("small", "small")
	rMed := kit.NewRadio("medium", "medium")
	rLarge := kit.NewRadio("large", "large")
	rCust := kit.NewRadio("customize", "customize")
	for _, r := range []*kit.Radio{rSmall, rMed, rLarge, rCust} {
		r.SetFace(c.face)
	}
	gapRG := kit.NewRadioGroup(rSmall, rMed, rLarge, rCust)
	gapRG.Select("small")
	radioRow(gapRG)
	gapSlider := kit.NewSlider(16)
	gapSlider.Min, gapSlider.Max = 0, 64
	gapSlider.Width = 0 // follow host
	// Slider host expands with content width when shown
	sliderSlot := primitive.NewSlot("flex-gap-slider", nil)

	applyGapMode := func(mode string) {
		switch mode {
		case "medium":
			gapRow.SetGapSize(kit.FlexGapMedium)
			sliderSlot.SetChild(nil)
		case "large":
			gapRow.SetGapSize(kit.FlexGapLarge)
			sliderSlot.SetChild(nil)
		case "customize":
			gapRow.SetGap(gapSlider.Value)
			// wrap slider in ExpandWidth host so it tracks tab width
			sh := primitive.NewDecorated(gapSlider.Node())
			sh.ExpandWidth = true
			sh.Height = 32
			sh.StretchChild = true
			sliderSlot.SetChild(sh)
		default:
			gapRow.SetGapSize(kit.FlexGapSmall)
			sliderSlot.SetChild(nil)
		}
		*c.status = "flex gap → " + mode
	}
	gapRG.OnChange = applyGapMode
	gapSlider.OnChange = func(v float64) {
		if gapRG.Value == "customize" {
			gapRow.SetGap(v)
			*c.status = fmt.Sprintf("flex gap custom → %.0f", v)
		}
	}
	gapOuter := kit.NewFlex(gapRG.Node(), sliderSlot, gapRow.Node())
	gapOuter.SetVertical(true)
	gapOuter.SetGapSize(kit.FlexGapMedium)
	secFlexGap := demoSection(c.face, c.theme, "设置间隙",
		"antd gap.tsx：Radio 选 small(8)/medium(16)/large(24)/customize。customize 时出现 Slider；按钮行 wrap，窄窗不溢出。",
		gapOuter.Node())

	// ---------- wrap.tsx ----------
	wrapKids := make([]core.Node, 0, 24)
	for i := 0; i < 24; i++ {
		wrapKids = append(wrapKids, mkPrimaryBtn("Button").Node())
	}
	flexWrap := kit.NewFlex(wrapKids...)
	flexWrap.SetWrap(true)
	flexWrap.SetGapSize(kit.FlexGapSmall)
	// ExpandWidth host passes bounded MaxWidth so wrap actually engages (antd 100% row).
	wrapHost := playground(flexWrap.Node(), 0, render.RGBA{R: 0, G: 0, B: 0, A: 0.06})
	wrapHost.Padding = primitive.All(8)
	wrapHost.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	secFlexWrap := demoSection(c.face, c.theme, "自动换行",
		"antd wrap.tsx：wrap + gap=small，24 个 Primary Button。宿主宽度随内容区收缩；窗口变窄时自动换行，不横向撑破父级。",
		wrapHost)

	// ---------- combination.tsx ----------
	// antd combination.tsx (Card 620, body pad 0):
	//   Flex justify=space-between
	//     img width 273 (photo ~square → height ≈ 180–273)
	//     Flex vertical align=end justify=space-between style={{padding:32}}
	//       Title level 3
	//       Button "Get Started"
	//
	// Critical for "Get Started" at bottom-right with a large gap:
	// the right column must be TALLER than title+button. antd gets that from
	// the image height; if title wraps taller than the image, space-between
	// has zero free space and the button sticks under the title.
	const comboImgW, comboImgH = 273.0, 273.0
	flexImg := primitive.NewDecorated(nil)
	flexImg.Width = comboImgW
	flexImg.Height = comboImgH
	flexImg.MinWidth = comboImgW
	flexImg.Background = render.Hex("#1677ff")

	quote := kit.NewTitle("“antd is an enterprise-class UI design language and React UI library.”", 3)
	quote.SetFace(c.face)
	quote.SetEllipsisRows(3)
	// Cap title width so level-3 wraps like antd (~ right column − pad).
	quoteBox := primitive.NewDecorated(quote.Node())
	quoteBox.Width = 250

	getStarted := mkPrimaryBtn("Get Started")
	comboInner := kit.NewFlex(quoteBox, getStarted.Node())
	comboInner.SetVertical(true)
	comboInner.SetAlign(kit.FlexAlignEnd)              // items to the right
	comboInner.SetJustify(kit.FlexJustifySpaceBetween) // title top, button bottom

	// Definite height = image height so StretchChild gives the inner flex a
	// tight height and space-between creates the large vertical gap.
	innerPad := primitive.NewDecorated(comboInner.Node())
	innerPad.Padding = primitive.All(32)
	innerPad.Width = 620 - comboImgW // 347
	innerPad.Height = comboImgH
	innerPad.StretchChild = true

	comboRow := kit.NewFlex(flexImg, innerPad)
	comboRow.SetJustify(kit.FlexJustifyStart)
	comboRow.SetAlign(kit.FlexAlignStart) // both panes fixed size; no stretch fight
	comboRow.SetWrap(false)
	comboRow.SetGap(0)

	cardShell := primitive.NewDecorated(comboRow.Node())
	cardShell.Width = 620
	cardShell.Radius = c.theme.SizeOr(core.TokenBorderRadiusLG, 8)
	cardShell.BorderWidth = 1
	cardShell.BorderColor = c.theme.Color(core.TokenColorBorder)
	cardShell.Background = c.theme.Color(core.TokenColorBgContainer)
	cardShell.Padding = primitive.EdgeInsets{}
	secFlexCombo := demoSection(c.face, c.theme, "组合使用",
		"antd combination.tsx：左图 273×273；右栏宽 347、高与图相同、pad 32；vertical + align=end + space-between → 标题右上、Get Started 右下且中间大间距。",
		cardShell)

	c.items = append(c.items, ctlTab("flex", "Flex"))
	c.contents["flex"] = demoPage(c.face, "Flex",
		"用于对齐的弹性布局容器。示例对齐 antd 6.5 官方非 debug demo。交互改 props 后立即重排；宿主随窗口宽度收缩，子项 wrap/均分，不溢出父容器。",
		secFlexBasic, secFlexAlign, secFlexGap, secFlexWrap, secFlexCombo,
	)
}
