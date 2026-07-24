//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSplitter() {
	// Splitter — docs/antd/splitter.md §6.8 P0
	// https://ant.design/components/splitter
	// demos: size / control / vertical / collapsible / collapsibleIcon /
	//         multiple / group / lazy
	// P1 deferred: customize / style-class / reset / semantic / debug

	face, th := c.face, c.theme

	desc := func(text string) core.Node {
		// antd demo Desc: Flex justify="center" align="center" height 100%
		// Decorated CenterContent centers both axes when StretchChild is false.
		tx := kit.NewText(text)
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: th.Color(core.TokenColorTextSecondary)})
		d := primitive.NewDecorated(tx.Node())
		d.ExpandWidth = true
		// fill panel height via parent tight constraints; do NOT StretchChild
		// (StretchChild disables CenterContent — labels would stick top-left).
		d.SetCenterContent(true)
		d.Background = render.RGBA{R: 0.97, G: 0.97, B: 0.97, A: 1}
		return d
	}
	playground := func(body core.Node, h float64) core.Node {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.StretchChild = true
		d.Radius = 6
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		d.BorderWidth = 1
		d.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.08}
		if h > 0 {
			d.Height = h
			d.MinHeight = h
		}
		return d
	}

	// ---------- size.tsx 基本用法 ----------
	p0 := kit.NewSplitterPanel(desc("First"))
	p0.SetDefaultSizePercent(40)
	p0.SetMinPercent(20)
	p0.SetMaxPercent(70)
	p1 := kit.NewSplitterPanel(desc("Second"))
	basic := kit.NewSplitter(p0, p1)
	basic.SetHeight(200)
	basic.SetTheme(th)
	secBasic := demoSection(face, th, "基本用法",
		"antd size.tsx：defaultSize 40% · min 20% · max 70%。",
		playground(basic.Node(), 200))

	// ---------- control.tsx 受控模式 ----------
	ctrlSizes := []float64{50, 50} // percent
	cp0 := kit.NewSplitterPanel(desc("First"))
	cp0.SetSizePercent(ctrlSizes[0])
	cp1 := kit.NewSplitterPanel(desc("Second"))
	cp1.SetSizePercent(ctrlSizes[1])
	ctrl := kit.NewSplitter(cp0, cp1)
	ctrl.SetHeight(200)
	ctrl.SetTheme(th)
	ctrl.OnResize = func(sizes []float64) {
		sum := sizes[0] + sizes[1]
		if sum <= 0 {
			return
		}
		ctrlSizes[0] = sizes[0] / sum * 100
		ctrlSizes[1] = sizes[1] / sum * 100
		cp0.SetSizePercent(ctrlSizes[0])
		cp1.SetSizePercent(ctrlSizes[1])
	}
	enSW := kit.NewSwitch()
	enSW.SetChecked(true)
	enSW.SetCheckedChildren("Enabled")
	enSW.SetUnCheckedChildren("Disabled")
	enSW.OnChange = func(on bool) {
		cp0.SetResizable(on)
		ctrl.SetPanels(cp0, cp1)
		cp0.SetSizePercent(ctrlSizes[0])
		cp1.SetSizePercent(ctrlSizes[1])
	}
	resetBtn := kit.NewButton("Reset")
	resetBtn.SetFace(face)
	resetBtn.OnClick = func() {
		ctrlSizes[0], ctrlSizes[1] = 50, 50
		cp0.SetSizePercent(50)
		cp1.SetSizePercent(50)
		if ctrl.Root != nil {
			ctrl.Root.MarkNeedsLayout()
		}
	}
	ctrlBar := kit.NewSpace(enSW.Node(), resetBtn.Node())
	ctrlBar.SetSize(kit.SpaceSizeMiddle)
	ctrlCol := kit.NewSpace(playground(ctrl.Node(), 200), ctrlBar.Node())
	ctrlCol.SetOrientation(kit.SpaceVertical)
	ctrlCol.SetSize(kit.SpaceSizeMiddle)
	ctrlCol.SetExpandMax(true)
	secCtrl := demoSection(face, th, "受控模式",
		"antd control.tsx：size 受控 + onResize；Switch 切换 resizable；Reset 50%/50%。",
		ctrlCol.Node())

	// ---------- vertical.tsx ----------
	vp0 := kit.NewSplitterPanel(desc("First"))
	vp1 := kit.NewSplitterPanel(desc("Second"))
	vert := kit.NewSplitter(vp0, vp1)
	vert.SetVertical(true)
	vert.SetHeight(300)
	vert.SetTheme(th)
	secVert := demoSection(face, th, "垂直方向",
		"antd vertical.tsx：vertical / orientation=vertical。",
		playground(vert.Node(), 300))

	// ---------- collapsible.tsx ----------
	// antd default showCollapsibleIcon=auto: both arrows on bar hover.
	mkCollapsible := func(vertical bool, h float64) core.Node {
		a := kit.NewSplitterPanel(desc("First"))
		a.SetCollapsible(true)
		a.SetMinPercent(20)
		a.SetShowCollapsibleIcon(kit.CollapsibleIconAuto)
		b := kit.NewSplitterPanel(desc("Second"))
		b.SetCollapsible(true)
		b.SetShowCollapsibleIcon(kit.CollapsibleIconAuto)
		sp := kit.NewSplitter(a, b)
		if vertical {
			sp.SetOrientation(kit.SplitterVertical)
		}
		sp.SetCollapsibleMotion(true)
		sp.SetHeight(h)
		sp.SetTheme(th)
		return playground(sp.Node(), h)
	}
	colStack := kit.NewSpace(
		mkCollapsible(false, 200),
		mkCollapsible(true, 300),
	)
	colStack.SetOrientation(kit.SpaceVertical)
	colStack.SetSize(kit.SpaceSizeMiddle)
	colStack.SetExpandMax(true)
	secCol := demoSection(face, th, "可折叠",
		"antd collapsible.tsx：两侧 collapsible；默认 auto=悬停显示 ←→ / ↑↓ 两个箭头。",
		colStack.Node())

	// ---------- collapsibleIcon.tsx ----------
	mkIconPanel := func(label string, mode kit.CollapsibleIconMode) *kit.SplitterPanel {
		p := kit.NewSplitterPanel(desc(label))
		p.SetCollapsibleSides(true, true)
		p.SetShowCollapsibleIcon(mode)
		return p
	}
	iconSp := kit.NewSplitter(
		mkIconPanel("First", kit.CollapsibleIconAlways),
		mkIconPanel("Second", kit.CollapsibleIconAlways),
		mkIconPanel("Third", kit.CollapsibleIconAlways),
	)
	iconSp.SetHeight(200)
	iconSp.SetTheme(th)
	ra := kit.NewRadio("auto", "Auto")
	rt := kit.NewRadio("true", "True")
	rf := kit.NewRadio("false", "False")
	for _, r := range []*kit.Radio{ra, rt, rf} {
		r.SetFace(face)
	}
	rg := kit.NewRadioGroup(ra, rt, rf)
	rg.Select("true")
	applyIconMode := func(mode string) {
		var m kit.CollapsibleIconMode
		switch mode {
		case "auto":
			m = kit.CollapsibleIconAuto
		case "false":
			m = kit.CollapsibleIconNever
		default:
			m = kit.CollapsibleIconAlways
		}
		for _, p := range iconSp.Panels() {
			p.SetShowCollapsibleIcon(m)
		}
		if iconSp.Root != nil {
			iconSp.Root.MarkNeedsLayout()
		}
	}
	rg.OnChange = func(key string) { applyIconMode(key) }
	if fl, ok := rg.Node().(*primitive.Flex); ok {
		fl.Axis = core.AxisHorizontal
		fl.CrossAlign = core.CrossCenter
		fl.Gap = 16
		fl.Wrap = true
	}
	lab := kit.NewText("ShowCollapsibleIcon:")
	lab.SetFace(face)
	modeRow := kit.NewSpace(lab.Node(), rg.Node())
	modeRow.SetSize(kit.SpaceSizeMiddle)
	iconCol := kit.NewSpace(modeRow.Node(), playground(iconSp.Node(), 200))
	iconCol.SetOrientation(kit.SpaceVertical)
	iconCol.SetSize(kit.SpaceSizeMiddle)
	iconCol.SetExpandMax(true)
	secIcon := demoSection(face, th, "可折叠图标显示",
		"antd collapsibleIcon.tsx：showCollapsibleIcon auto|true|false。",
		iconCol.Node())

	// ---------- multiple.tsx ----------
	mp0 := kit.NewSplitterPanel(desc("Panel 1"))
	mp0.SetCollapsible(true)
	mp1 := kit.NewSplitterPanel(desc("Panel 2"))
	mp1.SetCollapsibleSides(true, false)
	mp2 := kit.NewSplitterPanel(desc("Panel 3"))
	multi := kit.NewSplitter(mp0, mp1, mp2)
	multi.SetHeight(200)
	multi.SetTheme(th)
	secMulti := demoSection(face, th, "多面板",
		"antd multiple.tsx：三面板；部分 collapsible。",
		playground(multi.Node(), 200))

	// ---------- group.tsx 复杂组合 ----------
	innerTop := kit.NewSplitterPanel(desc("Top"))
	innerBot := kit.NewSplitterPanel(desc("Bottom"))
	inner := kit.NewSplitter(innerTop, innerBot)
	inner.SetOrientation(kit.SplitterVertical)
	inner.SetTheme(th)
	gLeft := kit.NewSplitterPanel(desc("Left"))
	gLeft.SetCollapsible(true)
	gRight := kit.NewSplitterPanel(inner.Node())
	group := kit.NewSplitter(gLeft, gRight)
	group.SetHeight(300)
	group.SetTheme(th)
	secGroup := demoSection(face, th, "复杂组合",
		"antd group.tsx：水平 + 嵌套垂直 Splitter。",
		playground(group.Node(), 300))

	// ---------- lazy.tsx ----------
	mkLazy := func(vertical bool) core.Node {
		a := kit.NewSplitterPanel(desc("First"))
		a.SetDefaultSizePercent(40)
		if vertical {
			a.SetMinPercent(30)
		} else {
			a.SetMinPercent(20)
		}
		a.SetMaxPercent(70)
		b := kit.NewSplitterPanel(desc("Second"))
		sp := kit.NewSplitter(a, b)
		sp.SetLazy(true)
		if vertical {
			sp.SetOrientation(kit.SplitterVertical)
		}
		sp.SetHeight(200)
		sp.SetTheme(th)
		return playground(sp.Node(), 200)
	}
	lazyStack := kit.NewSpace(mkLazy(false), mkLazy(true))
	lazyStack.SetOrientation(kit.SpaceVertical)
	lazyStack.SetSize(kit.SpaceSizeMiddle)
	lazyStack.SetExpandMax(true)
	secLazy := demoSection(face, th, "延迟渲染模式",
		"antd lazy.tsx：拖中预览线、松手提交；水平 + 垂直。",
		lazyStack.Node())

	page := kit.NewSpace(
		secBasic,
		secCtrl,
		secVert,
		secCol,
		secIcon,
		secMulti,
		secGroup,
		secLazy,
	)
	page.SetOrientation(kit.SpaceVertical)
	page.SetSize(kit.SpaceSizeLarge)
	page.SetExpandMax(true)

	c.add("splitter", "Splitter", "Layout · Splitter", page.Node())
}
