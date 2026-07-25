//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerBadge() {
	// Badge — docs/antd/badge.md §6.8 P0
	// https://ant.design/components/badge
	// demos: basic / no-wrapper / overflow / dot / change / link / offset / size
	//
	// P1 not shown: full status/colorful/ribbon gallery pages, semantic classNames,
	// ScrollNumber animation, ConfigProvider global, debug demos.

	face, th := c.face, c.theme
	status := c.status

	track := func(b *kit.Badge) *kit.Badge {
		b.SetFace(face)
		if th != nil {
			b.SetTheme(th)
		}
		c.trackTicker(b)
		return b
	}

	sqAvatar := func() *kit.Avatar {
		a := kit.NewAvatar("")
		a.SetShape(kit.AvatarSquare)
		a.SetSize(kit.AvatarLarge)
		a.SetFace(face)
		if th != nil {
			a.SetTheme(th)
		}
		return a
	}

	// ---------- basic.tsx ----------
	b5 := track(kit.NewBadge())
	b5.SetChild(sqAvatar().Node())
	b5.SetCount(5)

	b0 := track(kit.NewBadge())
	b0.SetChild(sqAvatar().Node())
	b0.SetCount(0)
	b0.SetShowZero(true)

	bIcon := track(kit.NewBadge())
	bIcon.SetChild(sqAvatar().Node())
	bIcon.SetCountNode(kit.NewIcon("clock").Node())

	secBasic := demoSection(face, th, "基本",
		"数字徽标、showZero 与自定义 count 节点。",
		spaceWrap(24, b5.Node(), b0.Node(), bIcon.Node()))

	// ---------- no-wrapper.tsx ----------
	showNW := true
	nw1 := track(kit.NewBadge())
	nw1.SetCount(11)
	nw1.SetShowZero(true)
	nw1.SetColor("#faad14")
	nw2 := track(kit.NewBadge())
	nw2.SetCount(25)
	nw3 := track(kit.NewBadge())
	nw3.SetCount(109)
	nw3.SetStyle(kit.Style{Background: render.Hex("#52c41a")})
	swNW := kit.NewSwitch()
	swNW.SetChecked(true)
	swNW.SetOnChange(func(on bool) {
		showNW = on
		if showNW {
			nw1.SetCount(11)
			nw2.SetCount(25)
			nw3.SetCount(109)
		} else {
			nw1.SetCount(0)
			nw1.SetShowZero(false)
			nw2.SetCount(0)
			nw3.SetCount(0)
		}
		if status != nil {
			*status = fmt.Sprintf("badge no-wrapper show=%v", showNW)
		}
	})
	secNW := demoSection(face, th, "独立使用",
		"无 children 时徽标独立展示；可配 color / Style。",
		spaceWrap(16, swNW.Node(), nw1.Node(), nw2.Node(), nw3.Node()))

	// ---------- overflow.tsx ----------
	mkOv := func(count, ov int) core.Node {
		b := track(kit.NewBadge())
		b.SetChild(sqAvatar().Node())
		b.SetCount(count)
		if ov > 0 {
			b.SetOverflowCount(ov)
		}
		return b.Node()
	}
	secOv := demoSection(face, th, "封顶数字",
		"超过 overflowCount 显示 N+。",
		spaceWrap(24,
			mkOv(99, 0),
			mkOv(100, 0),
			mkOv(99, 10),
			mkOv(1000, 999),
		))

	// ---------- dot.tsx ----------
	d1 := track(kit.NewBadge())
	d1.SetChild(kit.NewIcon("bell").Node())
	d1.SetDot(true)
	d2 := track(kit.NewBadge())
	d2.SetChild(kit.NewText("Link something").Node())
	d2.SetDot(true)
	secDot := demoSection(face, th, "讨嫌的小红点",
		"不展示数字，只显示红点。",
		spaceWrap(24, d1.Node(), d2.Node()))

	// ---------- change.tsx ----------
	count := 5
	showDot := true
	dyn := track(kit.NewBadge())
	dyn.SetChild(sqAvatar().Node())
	dyn.SetCount(count)
	dynDot := track(kit.NewBadge())
	dynDot.SetChild(sqAvatar().Node())
	dynDot.SetDot(true)

	btnMinus := kit.NewButton("-")
	btnMinus.SetSize(kit.ButtonSmall)
	btnMinus.SetOnClick(func() {
		if count > 0 {
			count--
		}
		dyn.SetCount(count)
		if status != nil {
			*status = fmt.Sprintf("badge count=%d", count)
		}
	})
	*c.buttons = append(*c.buttons, btnMinus)
	btnPlus := kit.NewButton("+")
	btnPlus.SetSize(kit.ButtonSmall)
	btnPlus.SetOnClick(func() {
		count++
		dyn.SetCount(count)
		if status != nil {
			*status = fmt.Sprintf("badge count=%d", count)
		}
	})
	*c.buttons = append(*c.buttons, btnPlus)
	btnRand := kit.NewButton("?")
	btnRand.SetSize(kit.ButtonSmall)
	btnRand.SetOnClick(func() {
		count = (count*17 + 13) % 100
		dyn.SetCount(count)
		if status != nil {
			*status = fmt.Sprintf("badge count=%d", count)
		}
	})
	*c.buttons = append(*c.buttons, btnRand)
	swDot := kit.NewSwitch()
	swDot.SetChecked(true)
	swDot.SetOnChange(func(on bool) {
		showDot = on
		dynDot.SetDot(showDot)
		if status != nil {
			*status = fmt.Sprintf("badge dot=%v", showDot)
		}
	})
	rowDyn1 := spaceWrap(16, dyn.Node(), btnMinus.Node(), btnPlus.Node(), btnRand.Node())
	rowDyn2 := spaceWrap(16, dynDot.Node(), swDot.Node())
	colDyn := primitive.Column(rowDyn1, rowDyn2)
	colDyn.Gap = 16
	colDyn.CrossAlign = core.CrossStart
	secDyn := demoSection(face, th, "动态",
		"动态改变 count 与 dot 显隐。",
		colDyn)

	// ---------- link.tsx ----------
	linkB := track(kit.NewBadge())
	linkB.SetChild(sqAvatar().Node())
	linkB.SetCount(5)
	linkB.SetOnClick(func() {
		if status != nil {
			*status = "badge link clicked"
		}
	})
	secLink := demoSection(face, th, "可点击",
		"整枚徽标可点击（OnClick）。",
		spaceWrap(16, linkB.Node()))

	// ---------- offset.tsx ----------
	offB := track(kit.NewBadge())
	offB.SetChild(sqAvatar().Node())
	offB.SetCount(5)
	offB.SetOffset(10, 10)
	secOff := demoSection(face, th, "自定义位置偏移",
		"offset=[10,10] 相对默认半出右上角锚点。",
		spaceWrap(16, offB.Node()))

	// ---------- size.tsx ----------
	szMed := track(kit.NewBadge())
	szMed.SetChild(sqAvatar().Node())
	szMed.SetCount(5)
	szMed.SetSize(kit.BadgeMedium)
	szSm := track(kit.NewBadge())
	szSm.SetChild(sqAvatar().Node())
	szSm.SetCount(5)
	szSm.SetSize(kit.BadgeSmall)
	secSize := demoSection(face, th, "大小",
		"count 的 medium / small 高度档。",
		spaceWrap(24, szMed.Node(), szSm.Node()))

	page := demoPage(face, "Badge 徽标数",
		"图标右上角的圆形徽标数字。P0 对齐 docs/antd/badge.md §6；P1 状态点/多彩/缎带 gallery 未铺。",
		secBasic, secNW, secOv, secDot, secDyn, secLink, secOff, secSize)
	c.addPage("badge", "Badge", page)
}
