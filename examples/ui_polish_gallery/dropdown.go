//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerDropdown() {
	// Dropdown — Ant Design demos (docs/antd/dropdown.md §6.8 P0)
	// https://ant.design/components/dropdown
	// P0: basic / extra / placement / arrow / item / arrow-center / trigger / event
	// P1 (not shown): dropdown-button, custom-dropdown, sub-menu multi, overlay-open, …

	face, th := c.face, c.theme
	status := c.status

	baseItems := []kit.MenuItem{
		{Key: "1", Label: "1st menu item"},
		{Key: "2", Label: "2nd menu item", Disabled: true, Icon: "info"},
		{Key: "3", Label: "3rd menu item", Disabled: true},
		{Key: "4", Label: "a danger item", Danger: true},
	}
	placeItems := []kit.MenuItem{
		{Key: "1", Label: "1st menu item"},
		{Key: "2", Label: "2nd menu item"},
		{Key: "3", Label: "3rd menu item"},
	}

	track := func(d *kit.Dropdown) *kit.Dropdown {
		d.SetFace(face)
		d.Theme = th
		return d
	}

	// ── basic.tsx (default hover) ─────────────────────────────
	ddBasic := track(kit.NewDropdown("Hover me", baseItems...))
	ddBasic.SetOnMenuClick(func(k string) { *status = "basic=" + k })

	// ── extra.tsx ─────────────────────────────────────────────
	ddExtra := track(kit.NewDropdown("Hover me",
		kit.MenuItem{Key: "1", Label: "My Account", Disabled: true},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "2", Label: "Profile", Extra: "⌘P"},
		kit.MenuItem{Key: "3", Label: "Billing", Extra: "⌘B"},
		kit.MenuItem{Key: "4", Label: "Settings", Icon: "info", Extra: "⌘S"},
	))

	// ── placement.tsx ─────────────────────────────────────────
	mkPlace := func(label string, pl kit.DropdownPlacement) core.Node {
		d := track(kit.NewDropdown(label, placeItems...))
		d.SetTrigger(kit.DropdownTriggerClick)
		d.SetPlacement(pl)
		return d.Node()
	}
	secPlaceBody := primitive.Column(
		spaceWrap(8,
			mkPlace("bottomLeft", kit.DropdownBottomLeft),
			mkPlace("bottom", kit.DropdownBottom),
			mkPlace("bottomRight", kit.DropdownBottomRight),
		),
		spaceWrap(8,
			mkPlace("topLeft", kit.DropdownTopLeft),
			mkPlace("top", kit.DropdownTop),
			mkPlace("topRight", kit.DropdownTopRight),
		),
		spaceWrap(8,
			mkPlace("leftTop", kit.DropdownLeftTop),
			mkPlace("left", kit.DropdownLeft),
			mkPlace("leftBottom", kit.DropdownLeftBottom),
		),
		spaceWrap(8,
			mkPlace("rightTop", kit.DropdownRightTop),
			mkPlace("right", kit.DropdownRight),
			mkPlace("rightBottom", kit.DropdownRightBottom),
		),
	)
	secPlaceBody.Gap = 12

	// ── arrow.tsx ─────────────────────────────────────────────
	mkArrow := func(label string, pl kit.DropdownPlacement) core.Node {
		d := track(kit.NewDropdown(label, placeItems...))
		d.SetTrigger(kit.DropdownTriggerClick)
		d.SetPlacement(pl)
		d.SetArrow(true)
		return d.Node()
	}
	secArrowBody := primitive.Column(
		spaceWrap(8,
			mkArrow("bottomLeft", kit.DropdownBottomLeft),
			mkArrow("bottom", kit.DropdownBottom),
			mkArrow("bottomRight", kit.DropdownBottomRight),
		),
		spaceWrap(8,
			mkArrow("topLeft", kit.DropdownTopLeft),
			mkArrow("top", kit.DropdownTop),
			mkArrow("topRight", kit.DropdownTopRight),
		),
	)
	secArrowBody.Gap = 12

	// ── item.tsx ──────────────────────────────────────────────
	ddItem := track(kit.NewDropdown("Hover me",
		kit.MenuItem{Key: "0", Label: "1st menu item"},
		kit.MenuItem{Key: "1", Label: "2nd menu item"},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "3", Label: "3rd menu item（disabled）", Disabled: true},
	))

	// ── arrow-center.tsx ──────────────────────────────────────
	mkArrowCenter := func(label string, pl kit.DropdownPlacement) core.Node {
		d := track(kit.NewDropdown(label, placeItems...))
		d.SetTrigger(kit.DropdownTriggerClick)
		d.SetPlacement(pl)
		d.SetArrowConfig(true, true)
		return d.Node()
	}
	secArrowCenterBody := primitive.Column(
		spaceWrap(8,
			mkArrowCenter("bottomLeft", kit.DropdownBottomLeft),
			mkArrowCenter("bottom", kit.DropdownBottom),
			mkArrowCenter("bottomRight", kit.DropdownBottomRight),
		),
		spaceWrap(8,
			mkArrowCenter("topLeft", kit.DropdownTopLeft),
			mkArrowCenter("top", kit.DropdownTop),
			mkArrowCenter("topRight", kit.DropdownTopRight),
		),
	)
	secArrowCenterBody.Gap = 12

	// ── trigger.tsx (click) ───────────────────────────────────
	ddTrigger := track(kit.NewDropdown("Click me",
		kit.MenuItem{Key: "0", Label: "1st menu item"},
		kit.MenuItem{Key: "1", Label: "2nd menu item"},
		kit.MenuItem{Divider: true},
		kit.MenuItem{Key: "3", Label: "3rd menu item"},
	))
	ddTrigger.SetTrigger(kit.DropdownTriggerClick)

	// ── event.tsx ─────────────────────────────────────────────
	ddEvent := track(kit.NewDropdown("Hover me, Click menu item",
		kit.MenuItem{Key: "1", Label: "1st menu item"},
		kit.MenuItem{Key: "2", Label: "2nd menu item"},
		kit.MenuItem{Key: "3", Label: "3rd menu item"},
	))
	ddEvent.SetTrigger(kit.DropdownTriggerClick)
	ddEvent.SetOnMenuClick(func(k string) { *status = "dropdown event key=" + k })

	// ── disabled ──────────────────────────────────────────────
	ddDis := track(kit.NewDropdown("Disabled", baseItems...))
	ddDis.SetDisabled(true)

	// ── controlled open ───────────────────────────────────────
	ddCtrl := track(kit.NewDropdown("Controlled open", placeItems...))
	ddCtrl.SetTrigger(kit.DropdownTriggerClick)
	ddCtrl.SetOpen(true)
	ddCtrl.SetOnOpenChange(func(open bool, src kit.DropdownOpenSource) {
		*status = "controlled OnOpenChange open=" + map[bool]string{true: "true", false: "false"}[open] + " src=" + string(src)
		// parent re-applies open to keep controlled panel visible when user tries to close
		if !open {
			// demo: allow close then reopen via SetOpen below buttons
		}
		ddCtrl.SetOpen(open)
	})
	btnClose := c.trackBtn(kit.NewButton("Close"))
	btnClose.SetOnClick(func() { ddCtrl.SetOpen(false); *status = "controlled closed" })
	btnOpen := c.trackBtn(kit.NewButton("Open"))
	btnOpen.SetType(kit.ButtonPrimary)
	btnOpen.SetOnClick(func() { ddCtrl.SetOpen(true); *status = "controlled opened" })
	ctrlRow := spaceWrap(8, ddCtrl.Node(), btnOpen.Node(), btnClose.Node())

	// Lifecycle (#9)
	ddLife := track(kit.NewDropdown("Old label",
		kit.MenuItem{Key: "old", Label: "old item"},
	))
	ddLife.SetItems(
		kit.MenuItem{Key: "1", Label: "Lifecycle 1"},
		kit.MenuItem{Key: "2", Label: "Lifecycle 2"},
	)
	ddLife.SetTriggerLabel("Lifecycle")
	ddLife.SetTrigger(kit.DropdownTriggerClick)

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeDropdown, func(pc *core.PaintContext, n core.Node) {
		if f, ok := n.(*primitive.Flex); ok && f != nil {
			if f.Base().Key == "dropdown-skin-demo" {
				sz := f.Size()
				if pc != nil && sz.Width > 0 && sz.Height > 0 {
					pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, 4, render.Hex("#E6F4FF"))
				}
			}
			if p := baseSkin.Painter(kit.TypeDropdown); p != nil {
				p(pc, f)
				return
			}
			f.DefaultPaintChildren(pc)
			return
		}
		if p := baseSkin.Painter(kit.TypeDropdown); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	ddSkin := track(kit.NewDropdown("Skin Dropdown", placeItems...))
	ddSkin.SetTrigger(kit.DropdownTriggerClick)
	if wrap, ok := ddSkin.Node().(*primitive.Flex); ok {
		wrap.SkinType = kit.TypeDropdown
		wrap.Base().Key = "dropdown-skin-demo"
	}

	c.items = append(c.items, ctlTab("dropdown", "Dropdown"))
	c.contents["dropdown"] = demoPage(face, "Dropdown",
		"向下弹出的列表。P0 对齐 docs/antd/dropdown.md §6（trigger hover|click|contextMenu、placement 12 向、arrow、open/onOpenChange、menu items danger/extra/divider、Token）+ #9 lifecycle + #6 Skin。",
		demoSection(face, th, "基本", "最简单的下拉菜单。默认 trigger=hover（antd basic.tsx）。",
			ddBasic.Node()),
		demoSection(face, th, "额外节点", "菜单项可带 extra 快捷键与 icon（antd extra.tsx）。",
			ddExtra.Node()),
		demoSection(face, th, "弹出位置", "支持 12 个弹出位置（antd placement.tsx）。",
			secPlaceBody),
		demoSection(face, th, "箭头", "下拉框箭头（antd arrow.tsx）。",
			secArrowBody),
		demoSection(face, th, "其他元素", "分割线与禁用项（antd item.tsx）。",
			ddItem.Node()),
		demoSection(face, th, "箭头指向", "arrow.pointAtCenter 指向触发器中心（antd arrow-center.tsx）。",
			secArrowCenterBody),
		demoSection(face, th, "触发方式", "click 触发（antd trigger.tsx；默认仍为 hover）。",
			ddTrigger.Node()),
		demoSection(face, th, "触发事件", "点击菜单项回调 OnMenuClick（antd event.tsx）。",
			ddEvent.Node()),
		demoSection(face, th, "禁用", "disabled 时不打开。",
			ddDis.Node()),
		demoSection(face, th, "受控 open", "SetOpen + OnOpenChange（antd open / onOpenChange）。",
			ctrlRow),
		demoSection(face, th, "Lifecycle (#9)",
			"structureChange：SetItems → chromeChange：SetTriggerLabel/SetTrigger；ensureBuilt 懒构建。",
			ddLife.Node()),
		demoSection(face, th, "Skin painter (#6)",
			"Wrap.SkinType=kit.Dropdown。Key=dropdown-skin-demo → 浅蓝底 Override；其它 Dropdown 不受影响。",
			ddSkin.Node()),
	)
}
