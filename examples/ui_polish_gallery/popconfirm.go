//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerPopconfirm() {
	// Popconfirm — Ant Design demos (docs/antd/popconfirm.md §6.8 P0)
	// https://ant.design/components/popconfirm
	// P0: basic / locale / placement / shift / dynamic-trigger / icon / async / promise
	// P1 (not shown): style-class semantic, debug panels

	face, th := c.face, c.theme
	status := c.status

	track := func(pc *kit.Popconfirm) *kit.Popconfirm {
		pc.SetFace(face)
		pc.SetTheme(th)
		return pc
	}

	// ── basic.tsx ────────────────────────────────────────────────
	basic := track(kit.NewPopconfirm("Delete the task"))
	basic.SetDescription("Are you sure to delete this task?")
	basic.SetOkText("Yes")
	basic.SetCancelText("No")
	basicTrig := c.trackBtn(kit.NewButton("Delete"))
	basicTrig.SetDanger(true)
	basic.SetTriggerNode(basicTrig.Node())
	basic.SetOnConfirm(func() { *status = "basic: Yes" })
	basic.SetOnCancel(func() { *status = "basic: No" })

	// ── locale.tsx (i18n button labels) ──────────────────────────
	locale := track(kit.NewPopconfirm("Delete the task"))
	locale.SetDescription("Are you sure to delete this task?")
	locale.SetOkText("Yes")
	locale.SetCancelText("No")
	localeTrig := c.trackBtn(kit.NewButton("Delete"))
	localeTrig.SetDanger(true)
	locale.SetTriggerNode(localeTrig.Node())

	// ── placement.tsx ────────────────────────────────────────────
	mkPlace := func(label string, pl kit.PopoverPlacement) core.Node {
		pc := track(kit.NewPopconfirm("Are you sure to delete this task?"))
		pc.SetDescription("Delete the task")
		pc.SetOkText("Yes")
		pc.SetCancelText("No")
		pc.SetPlacement(pl)
		btn := c.trackBtn(kit.NewButton(label))
		pc.SetTriggerNode(btn.Node())
		return pc.Node()
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

	// ── shift.tsx (autoAdjustOverflow) ───────────────────────────
	shift := track(kit.NewPopconfirm("Thanks for using antd. Have a nice day !"))
	shift.SetAutoAdjustOverflow(true)
	shiftBtn := c.trackBtn(kit.NewButton("Scroll The Window"))
	shiftBtn.SetType(kit.ButtonPrimary)
	shift.SetTriggerNode(shiftBtn.Node())
	shift.SetOpen(true)

	// ── dynamic-trigger.tsx ──────────────────────────────────────
	dyn := track(kit.NewPopconfirm("Delete the task"))
	dyn.SetDescription("Are you sure to delete this task?")
	dyn.SetOkText("Yes")
	dyn.SetCancelText("No")
	condition := true
	sw := kit.NewSwitch()
	sw.SetDefaultChecked(true)
	sw.SetOnChange(func(v bool) { condition = v })
	dyn.SetOpen(false)
	dyn.SetOnOpenChange(func(open bool) {
		if !open {
			dyn.SetOpen(false)
			return
		}
		if condition {
			*status = "dynamic: direct next step (no popconfirm)"
			return
		}
		dyn.SetOpen(true)
	})
	dyn.SetOnConfirm(func() {
		dyn.SetOpen(false)
		*status = "dynamic: confirmed"
	})
	dyn.SetOnCancel(func() {
		dyn.SetOpen(false)
		*status = "dynamic: cancelled"
	})
	dynTrig := c.trackBtn(kit.NewButton("Delete a task"))
	dynTrig.SetDanger(true)
	dyn.SetTriggerNode(dynTrig.Node())
	dynRow := primitive.Column(
		dyn.Node(),
		spaceWrap(8, kit.NewText("Whether directly execute：").Node(), sw.Node()),
	)
	dynRow.Gap = 12

	// ── icon.tsx ─────────────────────────────────────────────────
	iconPC := track(kit.NewPopconfirm("Delete the task"))
	iconPC.SetDescription("Are you sure to delete this task?")
	q := primitive.NewText("?")
	q.FontSize = 14
	q.Color = render.RGBA{R: 1, A: 1}
	iconPC.SetIconNode(q)
	iconTrig := c.trackBtn(kit.NewButton("Delete"))
	iconTrig.SetDanger(true)
	iconPC.SetTriggerNode(iconTrig.Node())

	// ── async.tsx (ConfirmLoading keep-open) ─────────────────────
	asyncPC := track(kit.NewPopconfirm("Title"))
	asyncPC.SetDescription("Open Popconfirm with async logic")
	asyncPC.SetOpen(false)
	asyncPC.SetOnOpenChange(func(open bool) {
		if asyncPC.ConfirmLoading && !open {
			return
		}
		asyncPC.SetOpen(open)
	})
	asyncPC.SetOnConfirm(func() {
		asyncPC.SetConfirmLoading(true)
		*status = "async: loading… click Cancel or resolve"
	})
	asyncPC.SetOnCancel(func() {
		asyncPC.SetConfirmLoading(false)
		asyncPC.SetOpen(false)
		*status = "async: cancelled"
	})
	// Resolve: second OK while loading is blocked; provide finish via cancel.
	// Also allow resolve by clearing loading + close from status interaction:
	asyncTrig := c.trackBtn(kit.NewButton("Open Popconfirm with async logic"))
	asyncTrig.SetType(kit.ButtonPrimary)
	asyncPC.SetTriggerNode(asyncTrig.Node())
	c.trackTicker(asyncPC)

	// ── promise.tsx ──────────────────────────────────────────────
	promise := track(kit.NewPopconfirm("Title"))
	promise.SetDescription("Open Popconfirm with Promise")
	promise.SetOnConfirmAsync(func(finish func()) {
		*status = "promise: pending… resolving"
		finish()
		*status = "promise: resolved"
	})
	promise.SetOnOpenChange(func(open bool) {
		*status = "promise: open=" + map[bool]string{true: "true", false: "false"}[open]
	})
	promiseTrig := c.trackBtn(kit.NewButton("Open Popconfirm with Promise"))
	promiseTrig.SetType(kit.ButtonPrimary)
	promise.SetTriggerNode(promiseTrig.Node())
	c.trackTicker(promise)

	// Lifecycle (#9)
	life := track(kit.NewPopconfirm("Lifecycle (#9)"))
	life.SetDescription("structureChange: Title → Description → OkText")
	life.SetOkText("Yes")
	life.SetCancelText("No")
	lifeTrig := c.trackBtn(kit.NewButton("Lifecycle"))
	life.SetTriggerNode(lifeTrig.Node())
	secLife := demoSection(c.face, c.theme, "Lifecycle (#9)",
		"structureChange：Title → Description → OkText/CancelText；ensureBuilt 懒构建 Wrap。",
		life.Node())

	// Skin (#6) — panel chrome comes from embedded Popover (SkinType=kit.Popover);
	// TypeID=kit.Popconfirm is registered in the default skin.
	baseSkin := c.theme.Skin
	c.theme.Skin = core.Override(baseSkin, kit.TypePopover, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "popconfirm-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypePopover); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypePopover); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinPC := track(kit.NewPopconfirm("Skin painter (#6)"))
	skinPC.SetDescription("panel.SkinType=kit.Popover (shared chrome)")
	skinTrig := c.trackBtn(kit.NewButton("Skin popconfirm"))
	skinPC.SetTriggerNode(skinTrig.Node())
	skinNode := skinPC.Node()
	if panel := skinPC.Panel(); panel != nil {
		panel.Base().Key = "popconfirm-skin-demo"
	}
	secSkin := demoSection(c.face, c.theme, "Skin painter (#6)",
		"TypeID=kit.Popconfirm 已注册；panel 共用 kit.Popover chrome，Key=popconfirm-skin-demo → 蓝边框 Override。",
		skinNode)

	page := demoPage(c.face, "Popconfirm", "Feedback / Popconfirm",
		demoSection(c.face, c.theme, "Basic", "title + description + Yes/No", basic.Node()),
		demoSection(c.face, c.theme, "Locale", "custom okText / cancelText", locale.Node()),
		demoSection(c.face, c.theme, "Placement", "12-way placement", secPlaceBody),
		demoSection(c.face, c.theme, "Shift", "autoAdjustOverflow + open", shift.Node()),
		demoSection(c.face, c.theme, "Conditional trigger", "OnOpenChange intercept", dynRow),
		demoSection(c.face, c.theme, "Custom icon", "SetIconNode", iconPC.Node()),
		demoSection(c.face, c.theme, "Async close", "controlled open + ConfirmLoading", asyncPC.Node()),
		demoSection(c.face, c.theme, "Promise close", "OnConfirmAsync finish", promise.Node()),
		secLife, secSkin,
	)
	c.addPage("popconfirm", "Popconfirm", page)
}
