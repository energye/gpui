//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerCard() {
	// Card — docs/antd/card.md §6.8 P0
	// https://ant.design/components/card
	// demos: basic / border-less / simple / flexible-content / in-column /
	//        loading / grid-card / inner
	//
	// P1 not shown: tabs, meta standalone page, style-class / semantic,
	// ConfigProvider global, debug demos.

	face, th := c.face, c.theme
	status := c.status

	wire := func(card *kit.Card) *kit.Card {
		card.SetFace(face)
		if th != nil {
			card.SetTheme(th)
		}
		c.trackTicker(card)
		return card
	}
	txt := func(s string) core.Node {
		t := kit.NewText(s)
		t.SetFace(face)
		return t.Node()
	}
	bodyLines := func() core.Node {
		col := primitive.Column(txt("Card content"), txt("Card content"), txt("Card content"))
		col.Gap = 4
		col.CrossAlign = core.CrossStart
		return col
	}

	// ---------- basic.tsx ----------
	basicMid := wire(kit.NewCard("Default size card"))
	basicMid.SetWidth(300)
	basicMid.SetExtra(txt("More"))
	basicMid.SetContent(bodyLines())
	basicSM := wire(kit.NewCard("Small size card"))
	basicSM.SetSize(kit.CardSmall)
	basicSM.SetWidth(300)
	basicSM.SetExtra(txt("More"))
	basicSM.SetContent(bodyLines())
	basicCol := primitive.Column(basicMid.Node(), basicSM.Node())
	basicCol.Gap = 16
	basicCol.CrossAlign = core.CrossStart
	secBasic := demoSection(face, th, "典型卡片",
		"basic.tsx：title + extra；size middle / small。",
		basicCol)

	// ---------- border-less.tsx ----------
	bl := wire(kit.NewCard("Card title"))
	bl.SetVariant(kit.CardBorderless)
	bl.SetWidth(300)
	bl.SetContent(bodyLines())
	secBL := demoSection(face, th, "无边框",
		"border-less.tsx：variant=borderless。",
		bl.Node())

	// ---------- simple.tsx ----------
	simple := wire(kit.NewCard(""))
	simple.SetWidth(300)
	simple.SetContent(bodyLines())
	secSimple := demoSection(face, th, "简洁卡片",
		"simple.tsx：无 header，仅 body。",
		simple.Node())

	// ---------- flexible-content.tsx ----------
	meta := kit.NewCardMeta()
	meta.SetFace(face)
	if th != nil {
		meta.SetTheme(th)
	}
	meta.SetTitle("Europe Street beat")
	meta.SetDescription("www.instagram.com")
	flexC := wire(kit.NewCard(""))
	flexC.SetHoverable(true)
	flexC.SetVariant(kit.CardBorderless)
	flexC.SetWidth(240)
	cover := kit.NewImageSized("cover", 240, 150)
	flexC.SetCover(cover.Node())
	flexC.SetContent(meta.Node())
	secFlex := demoSection(face, th, "更灵活的内容展示",
		"flexible-content.tsx：cover + Meta + hoverable + borderless。",
		flexC.Node())

	// ---------- in-column.tsx ----------
	mkColCard := func() core.Node {
		card := wire(kit.NewCard("Card title"))
		card.SetVariant(kit.CardBorderless)
		card.SetContent(txt("Card content"))
		return card.Node()
	}
	col1, col2, col3 := kit.NewCol(mkColCard()), kit.NewCol(mkColCard()), kit.NewCol(mkColCard())
	col1.SetSpan(8)
	col2.SetSpan(8)
	col3.SetSpan(8)
	gridRow := kit.NewRow(col1.Node(), col2.Node(), col3.Node())
	gridRow.SetGutter(16)
	secInCol := demoSection(face, th, "栅格卡片",
		"in-column.tsx：Row gutter=16 + Col span=8 ×3，borderless。",
		gridRow.Node())

	// ---------- loading.tsx ----------
	loading := true
	mkLoadingCard := func(seed string) *kit.Card {
		m := kit.NewCardMeta()
		m.SetFace(face)
		if th != nil {
			m.SetTheme(th)
		}
		av := kit.NewAvatar(seed)
		av.SetFace(face)
		m.SetAvatar(av.Node())
		m.SetTitle("Card title")
		m.SetDescription("This is the description")
		card := wire(kit.NewCard(""))
		card.SetWidth(300)
		card.SetLoading(loading)
		card.SetActions(
			kit.NewIcon("edit").Node(),
			kit.NewIcon("search").Node(),
			kit.NewIcon("plus").Node(),
		)
		card.SetContent(m.Node())
		return card
	}
	loadA := mkLoadingCard("A")
	loadB := mkLoadingCard("B")
	sw := kit.NewSwitch()
	sw.SetChecked(!loading)
	sw.SetOnChange(func(on bool) {
		loading = !on
		loadA.SetLoading(loading)
		loadB.SetLoading(loading)
		if status != nil {
			*status = fmt.Sprintf("card loading=%v", loading)
		}
	})
	loadCol := primitive.Column(sw.Node(), loadA.Node(), loadB.Node())
	loadCol.Gap = 12
	loadCol.CrossAlign = core.CrossStart
	secLoad := demoSection(face, th, "预加载的卡片",
		"loading.tsx：Switch 切换 loading；Skeleton body + actions。",
		loadCol)

	// ---------- grid-card.tsx ----------
	gridCard := wire(kit.NewCard("Card Title"))
	var grids []*kit.CardGrid
	for i := 0; i < 7; i++ {
		g := kit.NewCardGrid()
		g.SetFace(face)
		if th != nil {
			g.SetTheme(th)
		}
		g.SetWidthFrac(0.25)
		g.SetWidthPx(120)
		g.SetContent(txt("Content"))
		if i == 1 {
			g.SetHoverable(false)
		}
		grids = append(grids, g)
	}
	gridCard.SetGrids(grids...)
	secGrid := demoSection(face, th, "网格型内嵌卡片",
		"grid-card.tsx：Card.Grid ×7，width 25%；第二格 hoverable=false。",
		gridCard.Node())

	// ---------- inner.tsx ----------
	inner1 := wire(kit.NewCard("Inner Card title"))
	inner1.SetType(kit.CardTypeInner)
	inner1.SetExtra(txt("More"))
	inner1.SetContent(txt("Inner Card content"))
	inner2 := wire(kit.NewCard("Inner Card title"))
	inner2.SetType(kit.CardTypeInner)
	inner2.SetExtra(txt("More"))
	inner2.SetContent(txt("Inner Card content"))
	innerBody := primitive.Column(inner1.Node(), inner2.Node())
	innerBody.Gap = 16
	innerBody.CrossAlign = core.CrossStretch
	outer := wire(kit.NewCard("Card title"))
	outer.SetContent(innerBody)
	secInner := demoSection(face, th, "内部卡片",
		"inner.tsx：外层 Card 嵌套 type=inner 子卡片。",
		outer.Node())

	// Lifecycle (#9)
	life := wire(kit.NewCard("Lifecycle"))
	life.SetContent(bodyLines())
	life.SetSize(kit.CardSmall)
	life.SetHoverable(true)
	life.SetWidth(300)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Title/Content → Size → Hoverable；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeCard, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "card-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeCard); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeCard); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinC := wire(kit.NewCard("Skin"))
	skinC.SetContent(txt("TypeID=kit.Card"))
	skinC.SetWidth(300)
	skinNode := skinC.Node()
	if d, ok := skinC.ChromeNode().(*primitive.Decorated); ok && d != nil {
		d.Base().Key = "card-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Card 已注册。Key=card-skin-demo → 蓝边框 Override。",
		skinNode)

	c.addPage("card", "Card", demoPage(face, "Card 卡片",
		"数据展示 · docs/antd/card.md §6 P0（basic/border-less/simple/flexible-content/in-column/loading/grid-card/inner）。P1：tabs、semantic classNames/styles、boxShadow 像素级、ConfigProvider。Also #9 lifecycle + #6 Skin.",
		secBasic, secBL, secSimple, secFlex, secInCol, secLoad, secGrid, secInner, secLife, secSkin,
	))
}
