//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerAnchor() {
	// Anchor — antd demos §6.8 P0:
	// basic / horizontal / static / onClick / customizeHighlight /
	// targetOffset / onChange / replace
	// https://ant.design/components/anchor · components/anchor/demo/*.tsx
	//
	// P1 not shown: style-class / semantic, targetOffset-per-link debug, component-token.

	face, th := c.face, c.theme
	playground := func(body core.Node) *primitive.Decorated {
		d := primitive.NewDecorated(body)
		d.ExpandWidth = true
		d.Padding = primitive.All(12)
		d.Radius = 6
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		return d
	}
	sectionBlock := func(title string, h float64, bg render.RGBA) core.Node {
		lab := kit.NewText(title)
		lab.SetFace(face)
		box := primitive.NewDecorated(lab.Node())
		box.Height = h
		box.ExpandWidth = true
		box.Padding = primitive.All(12)
		box.Background = bg
		box.Radius = 4
		return box
	}
	offsets := map[string]float64{
		"#part-1": 0,
		"#part-2": 160,
		"#part-3": 320,
	}
	mkPartsSV := func() (*primitive.ScrollViewport, map[string]float64) {
		col := primitive.Column(
			sectionBlock("Part 1", 160, render.RGBA{R: 1, G: 0, B: 0, A: 0.04}),
			sectionBlock("Part 2", 160, render.RGBA{R: 0, G: 1, B: 0, A: 0.04}),
			sectionBlock("Part 3", 160, render.RGBA{R: 0, G: 0, B: 1, A: 0.04}),
		)
		col.Gap = 0
		col.CrossAlign = core.CrossStretch
		sv := primitive.NewScrollViewport(col)
		sv.Height = 220
		return sv, offsets
	}
	bindSpy := func(a *kit.Anchor, sv *primitive.ScrollViewport, off map[string]float64) {
		// SetScrollTarget installs OnScroll → SyncFromScroll (wheel / thumb / SetScroll).
		a.SetScrollTarget(sv)
		a.SetSectionOffsets(off)
		a.SyncFromScroll()
	}
	wireChange := func(a *kit.Anchor, tag string) {
		a.SetOnChange(func(link string) {
			*c.status = fmt.Sprintf("anchor %s onChange → %s", tag, link)
		})
	}
	flexPart := func(grow float64, n core.Node) core.Node {
		fl := primitive.NewFlexible(grow, n)
		fl.FillChild = true
		return fl
	}

	items3 := []kit.AnchorItem{
		{Key: "part-1", Href: "#part-1", Title: "Part 1"},
		{Key: "part-2", Href: "#part-2", Title: "Part 2"},
		{Key: "part-3", Href: "#part-3", Title: "Part 3"},
	}
	nested := []kit.AnchorItem{
		{Key: "1", Href: "#basic", Title: "Basic demo"},
		{Key: "2", Href: "#static", Title: "Static demo"},
		{
			Key: "3", Href: "#api", Title: "API",
			Children: []kit.AnchorItem{
				{Key: "4", Href: "#anchor-props", Title: "Anchor Props"},
				{Key: "5", Href: "#link-props", Title: "Link Props"},
			},
		},
	}

	// ---------- basic.tsx ----------
	basicSV, basicOff := mkPartsSV()
	basicA := kit.NewAnchor(items3...)
	basicA.SetFace(face)
	basicA.SetTheme(th)
	// Default Affix=true (antd); sticky needs scroll host — still default API.
	bindSpy(basicA, basicSV, basicOff)
	wireChange(basicA, "basic")
	basicRow := kit.NewFlex(flexPart(2, basicSV), flexPart(1, basicA.Node()))
	basicRow.SetGapSize(kit.FlexGapMedium)
	basicRow.SetAlign(kit.FlexAlignStart)
	secBasic := demoSection(face, th, "基本",
		"antd basic.tsx：三 section + items；默认 affix=true、direction=vertical、ink 随 active。",
		playground(basicRow.Node()))

	// ---------- horizontal.tsx ----------
	hSV, hOff := mkPartsSV()
	hSV.Height = 180
	hA := kit.NewAnchor(items3...)
	hA.SetFace(face)
	hA.SetTheme(th)
	hA.SetDirection(kit.AnchorHorizontal)
	hA.SetAffix(false)
	hA.SetShowInkInFixed(true)
	bindSpy(hA, hSV, hOff)
	wireChange(hA, "horizontal")
	hCol := kit.NewFlex(hA.Node(), hSV)
	hCol.SetVertical(true)
	hCol.SetGapSize(kit.FlexGapSmall)
	secHoriz := demoSection(face, th, "横向 Anchor",
		"antd horizontal.tsx：direction=horizontal；children 嵌套忽略；底轨 ink（showInkInFixed）。",
		playground(hCol.Node()))

	// ---------- static.tsx ----------
	staticA := kit.NewAnchor(nested...)
	staticA.SetFace(face)
	staticA.SetTheme(th)
	staticA.SetAffix(false)
	secStatic := demoSection(face, th, "静态位置",
		"antd static.tsx：affix=false；嵌套 children 二级链接可见；默认无 ink。",
		playground(staticA.Node()))

	// ---------- onClick.tsx ----------
	clickA := kit.NewAnchor(nested...)
	clickA.SetFace(face)
	clickA.SetTheme(th)
	clickA.SetAffix(false)
	clickA.SetOnClick(func(link kit.AnchorLinkInfo) {
		*c.status = fmt.Sprintf("anchor onClick → title=%q href=%q key=%q", link.Title, link.Href, link.Key)
	})
	secClick := demoSection(face, th, "自定义 onClick 事件",
		"antd onClick.tsx：点击回调载荷 {title, href, key}；affix=false。",
		playground(clickA.Node()))

	// ---------- customizeHighlight.tsx ----------
	hlA := kit.NewAnchor(nested...)
	hlA.SetFace(face)
	hlA.SetTheme(th)
	hlA.SetAffix(false)
	hlA.SetShowInkInFixed(true)
	hlA.SetGetCurrentAnchor(func(string) string { return "#static" })
	hlA.SetActiveLink("#basic")
	secHL := demoSection(face, th, "自定义锚点高亮",
		"antd customizeHighlight.tsx：getCurrentAnchor 固定高亮 #static。",
		playground(hlA.Node()))

	// ---------- targetOffset.tsx ----------
	toSV, toOff := mkPartsSV()
	toSV.Height = 200
	toA := kit.NewAnchor(items3...)
	toA.SetFace(face)
	toA.SetTheme(th)
	toA.SetAffix(false)
	toA.SetShowInkInFixed(true)
	toA.SetTargetOffset(40)
	bindSpy(toA, toSV, toOff)
	wireChange(toA, "targetOffset")
	toRow := kit.NewFlex(flexPart(2, toSV), flexPart(1, toA.Node()))
	toRow.SetGapSize(kit.FlexGapMedium)
	toRow.SetAlign(kit.FlexAlignStart)
	secTO := demoSection(face, th, "设置锚点滚动偏移量",
		"antd targetOffset.tsx：targetOffset=40；点击后 ScrollY = sectionY − 40。",
		playground(toRow.Node()))

	// ---------- onChange.tsx ----------
	chgA := kit.NewAnchor(nested...)
	chgA.SetFace(face)
	chgA.SetTheme(th)
	chgA.SetAffix(false)
	chgA.SetShowInkInFixed(true)
	wireChange(chgA, "onChange")
	secChg := demoSection(face, th, "监听锚点链接改变",
		"antd onChange.tsx：OnChange(currentActiveLink)；状态栏显示最近一次链接。",
		playground(chgA.Node()))

	// ---------- replace.tsx ----------
	repSV, repOff := mkPartsSV()
	repSV.Height = 180
	repA := kit.NewAnchor(items3...)
	repA.SetFace(face)
	repA.SetTheme(th)
	repA.SetAffix(false)
	repA.SetShowInkInFixed(true)
	repA.SetReplace(true)
	bindSpy(repA, repSV, repOff)
	repA.SetOnClick(func(link kit.AnchorLinkInfo) {
		*c.status = fmt.Sprintf("anchor replace History=%v Current=%s", repA.History, repA.CurrentHref)
	})
	repRow := kit.NewFlex(flexPart(2, repSV), flexPart(1, repA.Node()))
	repRow.SetGapSize(kit.FlexGapMedium)
	repRow.SetAlign(kit.FlexAlignStart)
	secRep := demoSection(face, th, "替换历史中的 href",
		"antd replace.tsx：replace=true 时 History 末项被替换（桌面映射，非 window.history）。",
		playground(repRow.Node()))

	note := kit.NewParagraph("默认 Affix=true（basic）；静态/横向等示例 affix=false。P1：semantic classNames、per-link targetOffset、ink 滑动动画。")
	note.SetFace(face)
	secNote := demoSection(face, th, "说明", "", note.Node())

	c.addPage("anchor", "Anchor", demoPage(face, "Anchor 锚点",
		"用于跳转到页面指定位置。P0 对齐 docs/antd/anchor.md §6 / antd 6.5。",
		secBasic, secHoriz, secStatic, secClick, secHL, secTO, secChg, secRep, secNote,
	))
}
