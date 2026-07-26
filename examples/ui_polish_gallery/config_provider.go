//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerConfigProvider() {
	face, th := c.face, c.theme

	basic := kit.NewConfigProvider(th, kit.NewText("ConfigProvider child").Node())
	secBasic := demoSection(face, th, "基本",
		"ambient Theme 提供者；子树 ResolveTheme 命中。",
		basic.Node())

	// Lifecycle (#9)
	lifeTxt := kit.NewText("Lifecycle child")
	lifeTxt.SetFace(face)
	life := kit.NewConfigProvider(th, nil)
	life.SetChild(lifeTxt.Node())
	life.SetTheme(th)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：SetChild；chromeChange：SetTheme 广播；ensureBuilt 空实现（无懒结构）。",
		life.Node())

	// Skin (#6) — ConfigProvider 自绘子节点；TypeID=kit.ConfigProvider 已注册，
	// Override 证明挂接点存在（默认委托，不改变视觉）。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeConfigProvider, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeConfigProvider); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinTxt := kit.NewText("TypeID=kit.ConfigProvider · Theme.Skin Override 可挂接")
	skinTxt.SetFace(face)
	skinCP := kit.NewConfigProvider(th, skinTxt.Node())
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"TypeID=kit.ConfigProvider 已注册；Paint 默认委托子节点绘制。",
		skinCP.Node())

	page := demoPage(face, "ConfigProvider",
		"Other · ConfigProvider — ambient Theme 提供者。Also #9 lifecycle + #6 Skin.",
		secBasic, secLife, secSkin)
	c.addPage("config_provider", "ConfigProvider", page)
}
