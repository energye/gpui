//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerEmpty() {
	// Empty — docs/antd/empty.md §6.8 P0
	// https://ant.design/components/empty
	// demos: basic / simple / customize / description / style-class
	//
	// P1 not shown: config-provider renderEmpty, _semantic.tsx debug,
	// true HTTP image decode, function-form styles depth.

	face, th := c.face, c.theme
	status := c.status

	wire := func(e *kit.Empty) *kit.Empty {
		e.SetFace(face)
		if th != nil {
			e.SetTheme(th)
		}
		return e
	}
	txt := func(s string) core.Node {
		t := kit.NewText(s)
		t.SetFace(face)
		return t.Node()
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewEmpty())
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：默认插画 + locale「No data」。",
		basic.Node())

	// ---------- simple.tsx ----------
	simple := wire(kit.NewEmpty())
	simple.SetImage(kit.EmptyImageSimple)
	secSimple := demoSection(face, th, "选择图片",
		"simple.tsx：Empty.PRESENTED_IMAGE_SIMPLE（empty-normal 度量）。",
		simple.Node())

	// ---------- customize.tsx ----------
	custom := wire(kit.NewEmpty())
	custom.SetImageSrc("https://gw.alipayobjects.com/zos/antfincdn/ZHrcdLPrvN/empty.svg")
	custom.SetImageHeight(60)
	custom.SetDescription("Customize Description")
	createBtn := c.trackBtn(kit.NewButton("Create Now"))
	createBtn.SetType(kit.ButtonPrimary)
	createBtn.SetOnClick(func() {
		if status != nil {
			*status = "Empty · Create Now"
		}
	})
	custom.SetChildren(createBtn.Node())
	secCustom := demoSection(face, th, "自定义",
		"customize.tsx：自定义图源/高度 + 描述 + footer 按钮。",
		custom.Node())

	// ---------- description.tsx ----------
	noDesc := wire(kit.NewEmpty())
	noDesc.HideDescription()
	secNoDesc := demoSection(face, th, "无描述",
		"description.tsx：description={false}，仅插画。",
		noDesc.Node())

	// ---------- style-class.tsx ----------
	styleObj := wire(kit.NewEmpty())
	styleObj.SetImage(kit.EmptyImageSimple)
	styleObj.SetDescription("Object styles")
	styleBtn := c.trackBtn(kit.NewButton("Create Now"))
	styleBtn.SetType(kit.ButtonPrimary)
	styleObj.SetChildren(styleBtn.Node())
	styleObj.SetClassNames(kit.EmptyClassNames{Root: "empty-style-demo"})
	styleObj.SetStyle(kit.Style{
		Background:  render.Hex("#F5F5F5"),
		Radius:      8,
		ForceRadius: true,
	})
	styleObj.SetDescriptionStyle(kit.Style{Text: render.Hex("#1890FF")})

	styleFn := wire(kit.NewEmpty())
	styleFn.SetImage(kit.EmptyImageSimple)
	styleFn.SetDescription("Function styles")
	styleFnBtn := c.trackBtn(kit.NewButton("Create Now"))
	styleFnBtn.SetType(kit.ButtonPrimary)
	styleFn.SetChildren(styleFnBtn.Node())
	styleFn.SetStyle(kit.Style{
		Background: render.Hex("#E6F7FF"),
		Border:     render.Hex("#91D5FF"),
	})
	styleFn.SetDescriptionStyle(kit.Style{Text: render.Hex("#1890FF")})

	styleCol := primitive.Column(styleObj.Node(), styleFn.Node())
	styleCol.Gap = 16
	styleCol.CrossAlign = core.CrossStretch
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"style-class.tsx：浅 styles.root/description + classNames.root（函数形态深度 P1）。",
		styleCol)

	// extra: description node slot
	nodeDesc := wire(kit.NewEmpty())
	nodeDesc.SetDescriptionNode(txt("Description as node"))
	secNode := demoSection(face, th, "描述节点",
		"SetDescriptionNode：描述区挂自定义 Node。",
		nodeDesc.Node())

	// Lifecycle (#9)
	life := wire(kit.NewEmpty())
	life.SetImage(kit.EmptyImageSimple)
	life.SetDescription("structure then chrome")
	life.SetStyle(kit.Style{Background: render.Hex("#FAFAFA"), Radius: 8, ForceRadius: true})
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Image/Description) then chromeChange (Style)；ensureBuilt 懒构建 Root。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeEmpty, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			if p := baseSkin.Painter(kit.TypeEmpty); p != nil {
				p(pc, n)
			}
			return
		}
		if d.Base().Key == "empty-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeEmpty); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinE := wire(kit.NewEmpty())
	skinE.SetImage(kit.EmptyImageSimple)
	skinE.SetDescription("Skin Override")
	if root, ok := skinE.ChromeNode().(*primitive.Decorated); ok {
		root.Base().Key = "empty-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Empty。Key=empty-skin-demo → 蓝边框 Override。",
		skinE.Node())

	page := demoPage(face, "Empty 空状态",
		"空状态占位图。P0：description / image DEFAULT|SIMPLE|Node|src / children footer / 浅 styles·classNames。\n"+
			"P1：ConfigProvider.renderEmpty、_semantic、真 URL 解码、styles 函数形态。Also #9 lifecycle + #6 Skin.",
		secBasic, secSimple, secCustom, secNoDesc, secStyle, secNode, secLife, secSkin)
	c.addPage("empty", "Empty", page)
}
