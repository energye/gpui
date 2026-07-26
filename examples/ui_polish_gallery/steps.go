//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerSteps() {
	// Steps — antd demos §6.8 P0:
	// simple / error / vertical / clickable / panel / icon / title-placement / max-count
	// https://ant.design/components/steps · components/steps/demo/*.tsx
	//
	// P1 not shown: progress-dot / nav / inline / inline-variant / style-class.

	face, th := c.face, c.theme
	wire := func(s *kit.Steps, tag string) *kit.Steps {
		s.SetFace(face)
		s.SetTheme(th)
		return s
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 16
		f.CrossAlign = core.CrossStretch
		f.MainAlign = core.MainStart
		return f
	}
	content := "This is a content."
	simpleItems := []kit.StepItem{
		{Title: "Finished", Content: content},
		{Title: "In Progress", Content: content, SubTitle: "Left 00:00:08"},
		{Title: "Waiting", Content: content},
	}

	// ---------- simple.tsx ----------
	s1 := wire(kit.NewSteps(simpleItems...), "simple")
	s1.SetCurrent(1)
	s1o := wire(kit.NewSteps(simpleItems...), "simple-outlined")
	s1o.SetCurrent(1)
	s1o.SetVariant(kit.StepsVariantOutlined)
	s1s := wire(kit.NewSteps(simpleItems...), "simple-small")
	s1s.SetCurrent(1)
	s1s.SetSize(kit.StepsSmall)
	s1so := wire(kit.NewSteps(simpleItems...), "simple-small-outlined")
	s1so.SetCurrent(1)
	s1so.SetSize(kit.StepsSmall)
	s1so.SetVariant(kit.StepsVariantOutlined)
	secSimple := demoSection(face, th, "Basic",
		"current=1 · filled/outlined × medium/small (simple.tsx).",
		col(s1.Node(), s1o.Node(), s1s.Node(), s1so.Node()))

	// ---------- error.tsx ----------
	sErr := wire(kit.NewSteps(simpleItems...), "error")
	sErr.SetCurrent(1)
	sErr.SetStatus(kit.StepsError)
	secError := demoSection(face, th, "Error",
		"current=1 status=error (error.tsx).",
		sErr.Node())

	// ---------- vertical.tsx ----------
	sV := wire(kit.NewSteps(simpleItems...), "vertical")
	sV.SetOrientation(kit.StepsVertical)
	sV.SetCurrent(1)
	sVs := wire(kit.NewSteps(simpleItems...), "vertical-small")
	sVs.SetOrientation(kit.StepsVertical)
	sVs.SetSize(kit.StepsSmall)
	sVs.SetCurrent(1)
	rowV := primitive.Row(sV.Node(), sVs.Node())
	rowV.Gap = 32
	rowV.CrossAlign = core.CrossStart
	secVertical := demoSection(face, th, "Vertical",
		"orientation=vertical · medium + small (vertical.tsx).",
		rowV)

	// ---------- clickable.tsx ----------
	sClick := wire(kit.NewSteps(
		kit.StepItem{Title: "Step 1", Content: content},
		kit.StepItem{Title: "Step 2", Content: content},
		kit.StepItem{Title: "Step 3", Content: content},
	), "clickable")
	sClick.SetOnChange(func(cur int) {
		sClick.SetCurrent(cur)
		*c.status = fmt.Sprintf("steps clickable → current=%d", cur)
	})
	sClickV := wire(kit.NewSteps(
		kit.StepItem{Title: "Step 1", Content: content},
		kit.StepItem{Title: "Step 2", Content: content},
		kit.StepItem{Title: "Step 3", Content: content},
	), "clickable-v")
	sClickV.SetOrientation(kit.StepsVertical)
	sClickV.SetOnChange(func(cur int) {
		sClickV.SetCurrent(cur)
		*c.status = fmt.Sprintf("steps clickable vertical → current=%d", cur)
	})
	secClick := demoSection(face, th, "Clickable",
		"onChange switches current (clickable.tsx).",
		col(sClick.Node(), sClickV.Node()))

	// ---------- panel.tsx ----------
	panelItems := []kit.StepItem{
		{Title: "Step 1", SubTitle: "00:00", Content: content},
		{Title: "Step 2", Content: content, Status: kit.StepsError},
		{Title: "Step 3", Content: content},
	}
	sPanel := wire(kit.NewSteps(panelItems...), "panel")
	sPanel.SetType(kit.StepsTypePanel)
	sPanel.SetOnChange(func(cur int) {
		sPanel.SetCurrent(cur)
		*c.status = fmt.Sprintf("steps panel → current=%d", cur)
	})
	sPanelS := wire(kit.NewSteps(panelItems...), "panel-small")
	sPanelS.SetType(kit.StepsTypePanel)
	sPanelS.SetSize(kit.StepsSmall)
	sPanelS.SetVariant(kit.StepsVariantOutlined)
	sPanelS.SetOnChange(func(cur int) {
		sPanelS.SetCurrent(cur)
		*c.status = fmt.Sprintf("steps panel small → current=%d", cur)
	})
	secPanel := demoSection(face, th, "Panel",
		"type=panel · medium + small outlined (panel.tsx).",
		col(sPanel.Node(), sPanelS.Node()))

	// ---------- icon.tsx ----------
	sIcon := wire(kit.NewSteps(
		kit.StepItem{Title: "Login", Status: kit.StepsFinish, Icon: "check"},
		kit.StepItem{Title: "Verification", Status: kit.StepsFinish, Icon: "info"},
		kit.StepItem{Title: "Pay", Status: kit.StepsProcess, Icon: "loading"},
		kit.StepItem{Title: "Done", Status: kit.StepsWait, Icon: "star"},
	), "icon")
	c.trackTicker(sIcon)
	secIcon := demoSection(face, th, "With icon",
		"custom icons + per-item status; loading uses Ticker (icon.tsx).",
		sIcon.Node())

	// ---------- title-placement.tsx ----------
	sTP := wire(kit.NewSteps(simpleItems...), "title-placement")
	sTP.SetCurrent(1)
	sTP.SetTitlePlacement(kit.StepsTitleVertical)
	sTPPct := wire(kit.NewSteps(simpleItems...), "title-placement-percent")
	sTPPct.SetCurrent(1)
	sTPPct.SetTitlePlacement(kit.StepsTitleVertical)
	sTPPct.SetPercent(60)
	sTPPctS := wire(kit.NewSteps(simpleItems...), "title-placement-percent-sm")
	sTPPctS.SetCurrent(1)
	sTPPctS.SetTitlePlacement(kit.StepsTitleVertical)
	sTPPctS.SetPercent(80)
	sTPPctS.SetSize(kit.StepsSmall)
	secTP := demoSection(face, th, "Title placement & progress",
		"titlePlacement=vertical · percent on process step (title-placement.tsx).",
		col(sTP.Node(), sTPPct.Node(), sTPPctS.Node()))

	// ---------- max-count.tsx ----------
	maxItems := make([]kit.StepItem, 7)
	for i := 0; i < 7; i++ {
		maxItems[i] = kit.StepItem{Title: fmt.Sprintf("Step %d", i+1)}
	}
	sMax := wire(kit.NewSteps(maxItems...), "max-count")
	sMax.SetCurrent(3)
	sMax.SetMaxCount(5)
	sMax.SetOnChange(func(cur int) {
		sMax.SetCurrent(cur)
		*c.status = fmt.Sprintf("steps maxCount → current=%d", cur)
	})
	secMax := demoSection(face, th, "Max count",
		"maxCount=5 collapses 7 steps with disabled ellipsis (max-count.tsx).",
		sMax.Node())

	// Lifecycle (#9)
	life := wire(kit.NewSteps(kit.StepItem{Title: "Old"}), "life")
	life.SetItems(simpleItems)
	life.SetCurrent(1)
	life.SetSize(kit.StepsSmall)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：SetItems → chromeChange：SetCurrent/SetSize；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeSteps, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok && f != nil {
			if f.Base().Key == "steps-skin-demo" {
				sz := f.Size()
				if pc != nil && sz.Width > 0 && sz.Height > 0 {
					pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, 4, render.Hex("#E6F4FF"))
				}
			}
			if p := baseSkin.Painter(kit.TypeSteps); p != nil {
				p(pc, f)
				return
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if p := baseSkin.Painter(kit.TypeSteps); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinS := wire(kit.NewSteps(simpleItems...), "skin")
	skinS.SetCurrent(1)
	if root, ok := skinS.ChromeNode().(*primitive.Flex); ok {
		root.Base().Key = "steps-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root.SkinType=kit.Steps。Key=steps-skin-demo → 浅蓝底 Override；其它 Steps 不受影响。",
		skinS.Node())

	c.addPage("steps", "Steps",
		demoPage(face, "Steps",
			"Navigation · Steps — antd v6.5 P0 demos (docs/antd/steps.md §6.8). Also verifies #9+#6.",
			secSimple, secError, secVertical, secClick, secPanel, secIcon, secTP, secMax, secLife, secSkin,
		))
}
