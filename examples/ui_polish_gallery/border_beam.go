//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerBorderBeam() {
	// BorderBeam — docs/antd/border-beam.md §6.8 P0
	// https://ant.design/components/border-beam
	// demos: basic / hover / custom-container / customized-color / duration / size / line-width
	//
	// P1 not shown: semantic classNames/styles, CSS offset-path/mask 像素级,
	// ConfigProvider 全局 borderBeam, non-uniform radius, debug/官网逐像素.

	face, th := c.face, c.theme

	wire := func(b *kit.BorderBeam) *kit.BorderBeam {
		if th != nil {
			b.SetTheme(th)
		}
		c.trackTicker(b)
		return b
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}
	row := func(kids ...core.Node) core.Node {
		f := primitive.Row(kids...)
		f.Gap = 16
		f.CrossAlign = core.CrossStart
		f.Wrap = true
		return f
	}
	cardBody := func(title, body string, width float64) *kit.Card {
		cd := kit.NewCard(title)
		cd.SetFace(face)
		if th != nil {
			cd.SetTheme(th)
		}
		if width > 0 {
			cd.SetWidth(width)
		}
		tx := kit.NewText(body)
		tx.SetFace(face)
		tx.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.65}})
		cd.SetContent(tx.Node())
		return cd
	}

	// ---------- basic.tsx ----------
	basicCard := cardBody("Workspace overview",
		"Review task status, deployment health, and recent automation activity in one panel.", 360)
	basic := wire(kit.NewBorderBeam(basicCard.Node()))
	secBasic := demoSection(face, th, "基础用法",
		"basic.tsx：默认 duration=6 / size=100 / lineWidth=1，Theme primary 流光。",
		basic.Node())

	// ---------- hover.tsx ----------
	hoverCard := cardBody("Hover over the card",
		"The border beam appears when the pointer moves over this card.", 360)
	hoverCard.SetHoverable(true)
	hoverBB := wire(kit.NewBorderBeam(hoverCard.Node()))
	hoverBB.SetShowOnHover(true)
	// Pressable path: Card hoverable does not bubble SetHovered to BorderBeam.
	// Provide a demo toggle so the P0 path is exercisable without CSS :hover.
	hoverSw := kit.NewSwitch()
	if th != nil {
		hoverSw.Theme = th
	}
	hoverSw.SetOnChange(func(on bool) {
		hoverBB.SetHovered(on)
		if c.status != nil {
			if on {
				*c.status = "BorderBeam hover: beam visible"
			} else {
				*c.status = "BorderBeam hover: beam hidden"
			}
		}
	})
	hoverLab := kit.NewText("Simulate hover (showOnHover)")
	hoverLab.SetFace(face)
	hoverCtrl := primitive.Row(hoverLab.Node(), hoverSw.Node())
	hoverCtrl.Gap = 12
	hoverCtrl.CrossAlign = core.CrossCenter
	secHover := demoSection(face, th, "鼠标悬浮时显示",
		"hover.tsx：ShowOnHover=true，默认隐藏；悬停（或下方开关）后显示并运行。",
		col(hoverBB.Node(), hoverCtrl))

	// ---------- custom-container.tsx ----------
	panelInner := kit.NewText("Review task status, deployment health, and recent automation activity in one custom container.")
	panelInner.SetFace(face)
	panel := primitive.NewDecorated(panelInner.Node())
	panel.Width = 420
	panel.MinHeight = 160
	panel.Padding = primitive.All(24)
	panel.Radius = 8
	panel.BorderWidth = 1
	if th != nil {
		panel.Background = th.Color(core.TokenColorBgContainer)
		panel.BorderColor = th.Color(core.TokenColorBorderSecondary)
		panelInner.SetStyle(kit.Style{Text: th.Color(core.TokenColorText)})
	} else {
		panel.Background = render.Hex("#ffffff")
		panel.BorderColor = render.Hex("#f0f0f0")
	}
	panel.Hit = core.HitBlock
	custom := wire(kit.NewBorderBeam(panel))
	custom.SetBorderRadius(8)
	secCustom := demoSection(face, th, "自定义容器",
		"custom-container.tsx：非 Card 的 relative 容器 + borderRadius=8。",
		custom.Node())

	// ---------- customized-color.tsx ----------
	type preset struct {
		name, usage, desc string
		stops             []kit.BorderBeamColorStop
	}
	presets := []preset{
		{"Ocean", "Dashboard", "A calm blue-green accent that works well for data views and cloud tooling.",
			[]kit.BorderBeamColorStop{
				{Color: render.Hex("#1677ff"), Percent: 0},
				{Color: render.Hex("#36cfc9"), Percent: 52},
				{Color: render.Hex("#95de64"), Percent: 100},
			}},
		{"Sunset", "Upgrade", "A warm highlight for upgrade prompts, featured cards, and marketing blocks.",
			[]kit.BorderBeamColorStop{
				{Color: render.Hex("#ff7a45"), Percent: 0},
				{Color: render.Hex("#ff4d4f"), Percent: 49},
				{Color: render.Hex("#ff85c0"), Percent: 100},
			}},
		{"Aurora", "AI", "A vivid cool-toned beam suited for AI assistants, copilots, and automation panels.",
			[]kit.BorderBeamColorStop{
				{Color: render.Hex("#7c3aed"), Percent: 0},
				{Color: render.Hex("#06b6d4"), Percent: 57},
				{Color: render.Hex("#67e8f9"), Percent: 100},
			}},
		{"Forest", "Recommendation", "A bright natural palette that feels good on recommendation and growth-oriented cards.",
			[]kit.BorderBeamColorStop{
				{Color: render.Hex("#22c55e"), Percent: 0},
				{Color: render.Hex("#a3e635"), Percent: 54},
				{Color: render.Hex("#facc15"), Percent: 100},
			}},
	}
	gradCard := cardBody(presets[0].name, presets[0].desc, 420)
	gradBB := wire(kit.NewBorderBeam(gradCard.Node()))
	gradBB.SetColorStops(presets[0].stops...)
	seg := kit.NewSegmented("Ocean", "Sunset", "Aurora", "Forest")
	if th != nil {
		seg.SetTheme(th)
	}
	seg.SetOnChange(func(key string) {
		for _, p := range presets {
			if p.name == key {
				gradCard.SetTitle(p.name)
				// rebuild body text
				tx := kit.NewText(p.desc)
				tx.SetFace(face)
				tx.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.65}})
				gradCard.SetContent(tx.Node())
				gradBB.SetColorStops(p.stops...)
				if c.status != nil {
					*c.status = "BorderBeam gradient: " + p.name
				}
				return
			}
		}
	})
	secGrad := demoSection(face, th, "渐变色",
		"customized-color.tsx：percent 0–100 停靠点；Segmented 切换 Ocean/Sunset/Aurora/Forest。",
		col(seg.Node(), gradBB.Node()))

	// ---------- duration.tsx ----------
	durRow := row()
	for _, d := range []struct {
		name string
		sec  float64
		desc string
	}{
		{"Fast", 3, "A quick loop for temporary highlights and active modules."},
		{"Default", 6, "The original pacing for most emphasized containers."},
		{"Slow", 12, "A calmer loop for persistent panels and ambient surfaces."},
	} {
		cd := cardBody(d.name, d.desc, 220)
		bb := wire(kit.NewBorderBeam(cd.Node()))
		bb.SetDuration(d.sec)
		durRow.(*primitive.Flex).AddChild(bb.Node())
	}
	secDur := demoSection(face, th, "动画时长",
		"duration.tsx：3s / 6s / 12s 三档。",
		durRow)

	// ---------- size.tsx ----------
	sizeRow := row()
	for _, s := range []struct {
		name string
		sz   float64 // 0 = default 100
		desc string
	}{
		{"Default", 0, "Uses the default 100px visible beam segment."},
		{"Compact", 56, "Keeps the highlight shorter for dense card groups."},
		{"Extended", 160, "Creates a longer highlight for wider feature panels."},
	} {
		cd := cardBody(s.name, s.desc, 280)
		bb := wire(kit.NewBorderBeam(cd.Node()))
		if s.sz > 0 {
			bb.SetSize(s.sz)
		}
		sizeRow.(*primitive.Flex).AddChild(bb.Node())
	}
	secSize := demoSection(face, th, "尺寸",
		"size.tsx：流光段长默认 100 / 56 / 160（非控件 small|middle|large）。",
		sizeRow)

	// ---------- line-width.tsx ----------
	lwCard := cardBody("Custom line width",
		"Set lineWidth to match the border width of this container.", 360)
	lwBB := wire(kit.NewBorderBeam(lwCard.Node()))
	lwBB.SetLineWidth(2)
	secLW := demoSection(face, th, "线宽",
		"line-width.tsx：lineWidth=2。",
		lwBB.Node())

	// Lifecycle (#9)
	lifeCard := cardBody("Lifecycle", "structureChange: Child → Duration → LineWidth.", 300)
	life := wire(kit.NewBorderBeam(lifeCard.Node()))
	life.SetDuration(4)
	life.SetLineWidth(2)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Child → Duration → LineWidth；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6) — Root 是 borderBeamHost（TypeID=kit.BorderBeam）；Override 证明挂接点存在。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeBorderBeam, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeBorderBeam); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinCard := cardBody("Skin", "TypeID=kit.BorderBeam · Theme.Skin Override 可挂接.", 300)
	skinBB := wire(kit.NewBorderBeam(skinCard.Node()))
	skinBB.SetColor(render.Hex("#1677FF"))
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root TypeID=kit.BorderBeam 已注册；host 默认委托子节点绘制（含 beam layer）。",
		skinBB.Node())

	c.addPage("border-beam", "BorderBeam", demoPage(face,
		"BorderBeam 边框流光",
		"为容器边框提供持续流动的装饰性高亮。P0 对齐 docs/antd/border-beam.md §6；"+
			"官方 basic / hover / custom-container / customized-color / duration / size / line-width。"+
			"P1：semantic classNames、offset-path 像素级、ConfigProvider 全局、debug。Also #9 lifecycle + #6 Skin.",
		secBasic, secHover, secCustom, secGrad, secDur, secSize, secLW, secLife, secSkin,
	))
}
