//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerPopover() {
	// Popover — Ant Design demos (docs/antd/popover.md §6.8 P0)
	// https://ant.design/components/popover
	// P0: basic / triggerType / placement / arrow / shift / control / hover-with-click
	// P1 (not shown): style-class semantic classNames/styles, color presets, delays, debug

	face, th := c.face, c.theme
	status := c.status

	track := func(p *kit.Popover) *kit.Popover {
		p.SetFace(face)
		p.Theme = th
		return p
	}

	// ── basic.tsx (default hover) ─────────────────────────────
	basicContent := primitive.Column(
		kit.NewText("Content").Node(),
		kit.NewText("Content").Node(),
	)
	basicContent.Gap = 4
	popBasic := track(kit.NewPopover("Hover me"))
	popBasic.SetTitle("Title")
	popBasic.SetContentNode(basicContent)

	// ── triggerType.tsx ───────────────────────────────────────
	mkTrig := func(label string, mode kit.PopoverTrigger) core.Node {
		p := track(kit.NewPopover(label))
		p.SetTitle("Title")
		p.SetContent("Content")
		p.SetTrigger(mode)
		return p.Node()
	}
	secTriggerBody := spaceWrap(8,
		mkTrig("Hover me", kit.PopoverTriggerHover),
		mkTrig("Focus me", kit.PopoverTriggerFocus),
		mkTrig("Click me", kit.PopoverTriggerClick),
	)

	// ── placement.tsx ─────────────────────────────────────────
	mkPlace := func(label string, pl kit.PopoverPlacement) core.Node {
		p := track(kit.NewPopover(label))
		p.SetTitle("Title")
		p.SetContent("Content")
		p.SetTrigger(kit.PopoverTriggerClick)
		p.SetPlacement(pl)
		return p.Node()
	}
	secPlaceBody := primitive.Column(
		spaceWrap(8,
			mkPlace("TL", kit.PopoverTopLeft),
			mkPlace("Top", kit.PopoverTop),
			mkPlace("TR", kit.PopoverTopRight),
		),
		spaceWrap(8,
			mkPlace("LT", kit.PopoverLeftTop),
			mkPlace("Left", kit.PopoverLeft),
			mkPlace("LB", kit.PopoverLeftBottom),
			mkPlace("RT", kit.PopoverRightTop),
			mkPlace("Right", kit.PopoverRight),
			mkPlace("RB", kit.PopoverRightBottom),
		),
		spaceWrap(8,
			mkPlace("BL", kit.PopoverBottomLeft),
			mkPlace("Bottom", kit.PopoverBottom),
			mkPlace("BR", kit.PopoverBottomRight),
		),
	)
	secPlaceBody.Gap = 12

	// ── arrow.tsx ─────────────────────────────────────────────
	mkArrow := func(label string, pl kit.PopoverPlacement, show bool, center bool) core.Node {
		p := track(kit.NewPopover(label))
		p.SetTitle("Title")
		p.SetContent("Content")
		p.SetTrigger(kit.PopoverTriggerClick)
		p.SetPlacement(pl)
		p.SetArrowConfig(show, center)
		return p.Node()
	}
	secArrowBody := primitive.Column(
		spaceWrap(8,
			mkArrow("Show TL", kit.PopoverTopLeft, true, false),
			mkArrow("Show Top", kit.PopoverTop, true, false),
			mkArrow("Show TR", kit.PopoverTopRight, true, false),
		),
		spaceWrap(8,
			mkArrow("Hide", kit.PopoverTop, false, false),
			mkArrow("Center TL", kit.PopoverTopLeft, true, true),
			mkArrow("Center Top", kit.PopoverTop, true, true),
		),
	)
	secArrowBody.Gap = 12

	// ── shift.tsx (autoAdjustOverflow) ────────────────────────
	popShift := track(kit.NewPopover("Scroll The Window"))
	popShift.SetContent("Thanks for using antd. Have a nice day !")
	popShift.SetAutoAdjustOverflow(true)
	popShift.SetOpen(true)

	// ── control.tsx (close from inside) ───────────────────────
	popCtrl := track(kit.NewPopover("Click me"))
	popCtrl.SetTitle("Title")
	popCtrl.SetTrigger(kit.PopoverTriggerClick)
	closeLink := c.trackBtn(kit.NewButton("Close"))
	closeLink.SetType(kit.ButtonLink)
	closeLink.SetOnClick(func() {
		popCtrl.SetOpen(false)
		*status = "popover controlled closed"
	})
	popCtrl.SetContentNode(closeLink.Node())
	popCtrl.SetOpen(false)
	popCtrl.SetOnOpenChange(func(open bool) {
		popCtrl.SetOpen(open)
		*status = "popover OnOpenChange open=" + map[bool]string{true: "true", false: "false"}[open]
	})

	// ── hover-with-click.tsx ──────────────────────────────────
	clickInner := track(kit.NewPopover("Hover and click"))
	clickInner.SetTitle("Click title")
	clickClose := c.trackBtn(kit.NewButton("Close"))
	clickClose.SetType(kit.ButtonLink)
	clickClose.SetOnClick(func() {
		clickInner.SetOpen(false)
		*status = "nested click closed"
	})
	clickBody := primitive.Column(
		kit.NewText("This is click content.").Node(),
		clickClose.Node(),
	)
	clickBody.Gap = 8
	clickInner.SetContentNode(clickBody)
	clickInner.SetTrigger(kit.PopoverTriggerClick)
	clickInner.SetOpen(false)
	clickInner.SetOnOpenChange(func(open bool) {
		clickInner.SetOpen(open)
		if open {
			// mutual exclusion with hover outer is host-side in antd demo
		}
	})

	hoverOuter := track(kit.NewPopover(""))
	hoverOuter.SetTitle("Hover title")
	hoverOuter.SetContent("This is hover content.")
	hoverOuter.SetTrigger(kit.PopoverTriggerHover)
	hoverOuter.SetTriggerNode(clickInner.Node())

	// ── disabled ──────────────────────────────────────────────
	popDis := track(kit.NewPopover("Disabled"))
	popDis.SetTitle("Title")
	popDis.SetContent("Content")
	popDis.SetDisabled(true)

	// ── panel background approx (P1 style-class hint) ─────────
	popStyle := track(kit.NewPopover("Panel bg"))
	popStyle.SetTitle("Styled")
	popStyle.SetContent("SetPanelBackground approx")
	popStyle.SetTrigger(kit.PopoverTriggerClick)
	popStyle.SetArrow(false)
	popStyle.SetPanelBackground(render.RGBA{R: 0.93, G: 0.93, B: 0.93, A: 1})

	c.items = append(c.items, ctlTab("popover", "Popover"))
	c.contents["popover"] = demoPage(face, "Popover",
		"气泡卡片。P0 对齐 docs/antd/popover.md §6（title/content、trigger hover|click|focus、placement 12 向、arrow、open/onOpenChange、autoAdjustOverflow、Token）。",
		demoSection(face, th, "基本", "最简单的气泡卡片。默认 trigger=hover（antd basic.tsx）。",
			popBasic.Node()),
		demoSection(face, th, "三种触发方式", "hover / focus / click（antd triggerType.tsx）。",
			secTriggerBody),
		demoSection(face, th, "位置", "支持 12 个弹出位置（antd placement.tsx）。",
			secPlaceBody),
		demoSection(face, th, "箭头展示", "arrow 显示/隐藏/pointAtCenter（antd arrow.tsx）。",
			secArrowBody),
		demoSection(face, th, "贴边偏移", "autoAdjustOverflow + Viewport（antd shift.tsx）。",
			popShift.Node()),
		demoSection(face, th, "从浮层内关闭", "受控 open + content 内关闭（antd control.tsx）。",
			popCtrl.Node()),
		demoSection(face, th, "悬停点击弹出窗口", "嵌套 hover + click（antd hover-with-click.tsx）。",
			hoverOuter.Node()),
		demoSection(face, th, "禁用", "disabled 时不打开。",
			popDis.Node()),
		demoSection(face, th, "面板底色（P1 近似）", "SetPanelBackground；完整 classNames/styles 属 P1。",
			popStyle.Node()),
	)
}
