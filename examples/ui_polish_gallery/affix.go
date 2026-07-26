//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerAffix() {
	// Affix — docs/antd/affix.md §6.8 P0
	// https://ant.design/components/affix
	// demos: basic / on-change / target
	//
	// P1 not shown: semantic classNames/styles, ConfigProvider 全局,
	// 真 position:fixed 出流 / Portal 钉, debug 与官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(a *kit.Affix) *kit.Affix {
		if th != nil {
			a.SetTheme(th)
		}
		return a
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}
	tallPad := func(h float64) core.Node {
		b := primitive.NewBox()
		b.Width = 1
		b.Height = h
		return b
	}
	scrollBox := func(h float64, content core.Node, border render.RGBA) (*kit.Scroll, core.Node) {
		sc := kit.NewScroll(content)
		sc.SetSize(480, h)
		sc.SetShowScrollbar(true)
		host := primitive.NewDecorated(sc.Node())
		host.Width = 480
		host.Height = h
		host.Radius = 6
		host.BorderWidth = 1
		host.BorderColor = border
		if th != nil {
			host.Background = th.Color(core.TokenColorBgContainer)
		} else {
			host.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		}
		host.Hit = core.HitDefer
		return sc, host
	}

	// ---------- basic.tsx ----------
	// Affix offsetTop / offsetBottom with primary buttons; click bumps offset.
	topOff := 40.0
	botOff := 40.0
	topBtn := kit.NewButton(fmt.Sprintf("Affix top %g", topOff))
	topBtn.SetType(kit.ButtonPrimary)
	topBtn.SetFace(face)
	if th != nil {
		topBtn.Theme = th
	}
	topAffix := wire(kit.NewAffix(topBtn.Node()))
	topAffix.SetOffsetTop(topOff)
	topAffix.SetContentTop(0)
	topAffix.SetOnChange(func(affixed bool) {
		if status != nil {
			*status = fmt.Sprintf("basic top affixed=%v offsetTop=%g", affixed, topOff)
		}
	})
	topBtn.SetOnClick(func() {
		topOff += 10
		topAffix.SetOffsetTop(topOff)
		topBtn.SetLabel(fmt.Sprintf("Affix top %g", topOff))
		if status != nil {
			*status = fmt.Sprintf("offsetTop → %g", topOff)
		}
	})

	botBtn := kit.NewButton(fmt.Sprintf("Affix bottom %g", botOff))
	botBtn.SetType(kit.ButtonPrimary)
	botBtn.SetFace(face)
	if th != nil {
		botBtn.Theme = th
	}
	botAffix := wire(kit.NewAffix(botBtn.Node()))
	botAffix.SetOffsetBottom(botOff)
	botAffix.SetContentTop(320)
	botAffix.SetOnChange(func(affixed bool) {
		if status != nil {
			*status = fmt.Sprintf("basic bottom affixed=%v offsetBottom=%g", affixed, botOff)
		}
	})
	botBtn.SetOnClick(func() {
		botOff += 10
		botAffix.SetOffsetBottom(botOff)
		botBtn.SetLabel(fmt.Sprintf("Affix bottom %g", botOff))
		if status != nil {
			*status = fmt.Sprintf("offsetBottom → %g", botOff)
		}
	})

	basicInner := col(
		topAffix.Node(),
		tallPad(280),
		botAffix.Node(),
		tallPad(120),
	)
	basicSC, basicHost := scrollBox(180, basicInner, render.RGBA{R: 0.09, G: 0.47, B: 1, A: 0.45})
	if th != nil {
		basicHost.(*primitive.Decorated).BorderColor = th.Color(core.TokenColorPrimaryBorder)
	}
	topAffix.SetScrollTarget(basicSC.Viewport())
	botAffix.SetScrollTarget(basicSC.Viewport())
	// Initial evaluate after layout path
	_ = basicHost.Layout(core.Loose(500, 200))
	topAffix.UpdatePosition()
	botAffix.UpdatePosition()

	secBasic := demoSection(face, th, "基本",
		"basic.tsx：offsetTop / offsetBottom；点击按钮 +10（桌面 ScrollTarget 映射 window）。",
		basicHost)

	// ---------- on-change.tsx ----------
	chgBtn := kit.NewButton("120px to affix top")
	chgBtn.SetFace(face)
	if th != nil {
		chgBtn.Theme = th
	}
	chg := wire(kit.NewAffix(chgBtn.Node()))
	chg.SetOffsetTop(120)
	chg.SetContentTop(160)
	chg.SetOnChange(func(affixed bool) {
		if status != nil {
			*status = fmt.Sprintf("onChange affixed=%v", affixed)
		}
	})
	chgInner := col(tallPad(160), chg.Node(), tallPad(600))
	chgSC, chgHost := scrollBox(160, chgInner, render.RGBA{R: 0, G: 0, B: 0, A: 0.12})
	if th != nil {
		chgHost.(*primitive.Decorated).BorderColor = th.Color(core.TokenColorBorder)
	}
	chg.SetScrollTarget(chgSC.Viewport())
	_ = chgHost.Layout(core.Loose(500, 180))
	chg.UpdatePosition()

	secChange := demoSection(face, th, "固定状态改变的回调",
		"on-change.tsx：offsetTop=120；滚动时 onChange(true/false) 写入状态栏。",
		chgHost)

	// ---------- target.tsx ----------
	tgtBtn := kit.NewButton("Fixed at the top of container")
	tgtBtn.SetType(kit.ButtonPrimary)
	tgtBtn.SetFace(face)
	if th != nil {
		tgtBtn.Theme = th
	}
	tgt := wire(kit.NewAffix(tgtBtn.Node()))
	tgt.SetOffsetTop(0)
	tgt.SetContentTop(0)
	tgt.SetOnChange(func(affixed bool) {
		if status != nil {
			*status = fmt.Sprintf("target container affixed=%v", affixed)
		}
	})
	tgtInner := col(tgt.Node(), tallPad(1000))
	primaryBorder := render.RGBA{R: 0.09, G: 0.47, B: 1, A: 1}
	if th != nil {
		primaryBorder = th.Color(core.TokenColorPrimary)
	}
	tgtSC, tgtHost := scrollBox(100, tgtInner, primaryBorder)
	tgt.SetScrollTarget(tgtSC.Viewport())
	_ = tgtHost.Layout(core.Loose(500, 120))
	tgt.UpdatePosition()

	secTarget := demoSection(face, th, "滚动容器",
		"target.tsx：Affix 监听容器 ScrollViewport（antd target={() => container}）。",
		tgtHost)

	note := kit.NewParagraph("P0：offsetTop/offsetBottom/onChange/SetScrollTarget + 占位不跳变。P1：semantic classNames、ConfigProvider、真 fixed 出流、debug/官网逐像素。")
	note.SetFace(face)

	// Lifecycle (#9)
	lifeTxt := kit.NewText("Affix lifecycle content")
	lifeTxt.SetFace(face)
	life := wire(kit.NewAffix(lifeTxt.Node()))
	life.SetOffsetTop(10)
	life.SetContentTop(0)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Content → OffsetTop/ContentTop；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6) — Root 是 affixHost（TypeID=kit.Affix）；Override 证明挂接点存在。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeAffix, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeAffix); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinTxt := kit.NewText("TypeID=kit.Affix · Theme.Skin Override 可挂接")
	skinTxt.SetFace(face)
	skinAf := wire(kit.NewAffix(skinTxt.Node()))
	skinAf.SetContentTop(0)
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root TypeID=kit.Affix 已注册；host 自带 stickDY 平移 Paint，默认委托子节点绘制。",
		skinAf.Node())

	page := col(secBasic, secChange, secTarget, secLife, secSkin, note.Node())
	c.add("affix", "Affix", "Other · Affix", page)
}
