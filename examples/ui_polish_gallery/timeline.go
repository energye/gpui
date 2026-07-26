//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTimeline() {
	// Timeline — docs/antd/timeline.md §6.8 P0
	// https://ant.design/components/timeline
	// demos: basic / variant / pending / alternate / horizontal / custom / end / title
	//
	// P1 not shown: title-span 精细占比, semantic/style-class, debug, 官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(tl *kit.Timeline) *kit.Timeline {
		tl.SetFace(face)
		if th != nil {
			tl.SetTheme(th)
		}
		c.trackTicker(tl)
		return tl
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	))
	secBasic := demoSection(face, th, "基本用法",
		"basic.tsx：默认 mode=start、variant=outlined、4 项 content。",
		basic.Node())

	// ---------- variant.tsx ----------
	filled := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	))
	filled.SetVariant(kit.TimelineFilled)
	secVariant := demoSection(face, th, "变体样式",
		"variant.tsx：variant=filled 实心节点。",
		filled.Node())

	// ---------- pending.tsx ----------
	pending := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01"},
		kit.TimelineItem{Content: "Recording...", Loading: true},
	))
	revBtn := c.trackBtn(kit.NewButton("Toggle Reverse"))
	revBtn.SetType(kit.ButtonPrimary)
	revBtn.SetOnClick(func() {
		pending.SetReverse(!pending.Reverse())
		if status != nil {
			if pending.Reverse() {
				*status = "Timeline · reverse=true"
			} else {
				*status = "Timeline · reverse=false"
			}
		}
	})
	pendingCol := primitive.Column(pending.Node(), revBtn.Node())
	pendingCol.Gap = 16
	pendingCol.CrossAlign = core.CrossStart
	secPending := demoSection(face, th, "等待及排序",
		"pending.tsx：末项 loading=true + spinner（Ticker）；Button 切换 reverse。",
		pendingCol)

	// ---------- alternate.tsx ----------
	alt := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01", Color: "green"},
		kit.TimelineItem{
			Content: "Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo.",
			Icon:    "sync",
		},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01", Color: "red"},
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync"},
	))
	alt.SetMode(kit.TimelineModeAlternate)
	secAlt := demoSection(face, th, "交替展现",
		"alternate.tsx：mode=alternate；color/icon 混用。",
		alt.Node())

	// ---------- horizontal.tsx ----------
	hItems := []kit.TimelineItem{
		{Content: "Init"},
		{Content: "Start"},
		{Content: "Pending"},
		{Content: "Complete"},
	}
	hStart := wire(kit.NewTimeline(hItems...))
	hStart.SetOrientation(kit.TimelineHorizontal)
	hStart.SetMode(kit.TimelineModeStart)
	hEnd := wire(kit.NewTimeline(hItems...))
	hEnd.SetOrientation(kit.TimelineHorizontal)
	hEnd.SetMode(kit.TimelineModeEnd)
	hAlt := wire(kit.NewTimeline(hItems...))
	hAlt.SetOrientation(kit.TimelineHorizontal)
	hAlt.SetMode(kit.TimelineModeAlternate)
	hCol := primitive.Column(hStart.Node(), hEnd.Node(), hAlt.Node())
	hCol.Gap = 24
	hCol.CrossAlign = core.CrossStretch
	secHoriz := demoSection(face, th, "水平布局",
		"horizontal.tsx：orientation=horizontal × mode start|end|alternate。",
		hCol)

	// ---------- custom.tsx ----------
	custom := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync", Color: "red"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	))
	secCustom := demoSection(face, th, "自定义时间轴点",
		"custom.tsx：item.icon + color=red（ClockCircle 以 sync 图标代替）。",
		custom.Node())

	// ---------- end.tsx ----------
	end := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "Create a services site 2015-09-01"},
		kit.TimelineItem{Content: "Solve initial network problems 2015-09-01"},
		kit.TimelineItem{Content: "Technical testing 2015-09-01", Icon: "sync", Color: "red"},
		kit.TimelineItem{Content: "Network problems being solved 2015-09-01"},
	))
	end.SetMode(kit.TimelineModeEnd)
	secEnd := demoSection(face, th, "另一侧时间轴点",
		"end.tsx：mode=end，内容在起始侧、点在结束侧。",
		end.Node())

	// ---------- title.tsx ----------
	titleTL := wire(kit.NewTimeline(
		kit.TimelineItem{Title: "2015-09-01", Content: "Create a services"},
		kit.TimelineItem{Title: "2015-09-01 09:12:11", Content: "Solve initial network problems"},
		kit.TimelineItem{Content: "Technical testing"},
		kit.TimelineItem{Title: "2015-09-01 09:12:11", Content: "Network problems being solved"},
	))
	modeStart := c.trackBtn(kit.NewButton("Start"))
	modeEnd := c.trackBtn(kit.NewButton("End"))
	modeAlt := c.trackBtn(kit.NewButton("Alternate"))
	modeStart.SetType(kit.ButtonPrimary)
	modeStart.SetOnClick(func() {
		titleTL.SetMode(kit.TimelineModeStart)
		if status != nil {
			*status = "Timeline · mode=start"
		}
	})
	modeEnd.SetOnClick(func() {
		titleTL.SetMode(kit.TimelineModeEnd)
		if status != nil {
			*status = "Timeline · mode=end"
		}
	})
	modeAlt.SetOnClick(func() {
		titleTL.SetMode(kit.TimelineModeAlternate)
		if status != nil {
			*status = "Timeline · mode=alternate"
		}
	})
	modeRow := primitive.Row(modeStart.Node(), modeEnd.Node(), modeAlt.Node())
	modeRow.Gap = 8
	titleCol := primitive.Column(modeRow, titleTL.Node())
	titleCol.Gap = 16
	titleCol.CrossAlign = core.CrossStretch
	secTitle := demoSection(face, th, "标题",
		"title.tsx：item.title + mode 切换；vertical 含 title 启用双侧槽。",
		titleCol)

	// Lifecycle (#9)
	life := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "structureChange: items"},
		kit.TimelineItem{Content: "chromeChange: variant/mode"},
	))
	life.SetVariant(kit.TimelineFilled)
	life.SetMode(kit.TimelineModeStart)
	life.SetReverse(false)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Items → Variant → Mode/Reverse；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6) — Root 是 timelineHost（RepaintBoundary，TypeID=kit.Timeline）；
	// Override 证明挂接点存在（默认委托，不改变视觉）。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTimeline, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeTimeline); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinTL := wire(kit.NewTimeline(
		kit.TimelineItem{Content: "TypeID=kit.Timeline"},
		kit.TimelineItem{Content: "Theme.Skin Override 可挂接"},
	))
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root TypeID=kit.Timeline 已注册；host 内嵌 RepaintBoundary，Paint 走默认子节点绘制。",
		skinTL.Node())

	page := demoPage(face, "Timeline",
		"垂直/水平时间流。P0：items content/title/color/icon/loading/placement、mode、orientation、variant、reverse；loading 走 Ticker。Also #9 lifecycle + #6 Skin.",
		secBasic, secVariant, secPending, secAlt, secHoriz, secCustom, secEnd, secTitle, secLife, secSkin)
	c.addPage("timeline", "Timeline", page)
}
