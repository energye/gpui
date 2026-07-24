//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTabs() {
	// Tabs — antd demos §6.8 P0:
	// basic / disabled / centered / icon / custom-indicator / slide / extra / size
	// https://ant.design/components/tabs · components/tabs/demo/*.tsx
	//
	// P1 not shown: placement full matrix page, card page, editable-card full page,
	// custom-add-trigger, custom-tab-bar, drag.

	face, th := c.face, c.theme
	wire := func(t *kit.Tabs) *kit.Tabs {
		t.SetFace(face)
		t.SetTheme(th)
		return t
	}
	box := func(n core.Node, w, h float64) core.Node {
		b := primitive.NewBox(n)
		b.Width, b.Height = w, h
		return b
	}
	pane := func(s string) core.Node {
		tx := kit.NewText(s)
		tx.SetFace(face)
		return tx.Node()
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: pane("Content of Tab Pane 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: pane("Content of Tab Pane 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: pane("Content of Tab Pane 3")},
	))
	basic.SetOnChange(func(k string) { *c.status = fmt.Sprintf("tabs basic → %s", k) })
	secBasic := demoSection(face, th, "Basic",
		"items + onChange (basic.tsx).",
		box(basic.Node(), 520, 140))

	// ---------- disabled.tsx ----------
	dis := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: pane("Tab 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Disabled: true, Children: pane("Tab 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: pane("Tab 3")},
	))
	secDisabled := demoSection(face, th, "Disabled",
		"item.disabled blocks selection (disabled.tsx).",
		box(dis.Node(), 520, 140))

	// ---------- centered.tsx ----------
	cen := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: pane("Tab 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: pane("Tab 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: pane("Tab 3")},
	))
	cen.SetCentered(true)
	secCentered := demoSection(face, th, "Centered",
		"centered=true (centered.tsx).",
		box(cen.Node(), 520, 140))

	// ---------- icon.tsx ----------
	ic := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Icon: "info", Children: pane("Tab 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Icon: "star", Children: pane("Tab 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Icon: "heart", Children: pane("Tab 3")},
	))
	secIcon := demoSection(face, th, "Icon",
		"item.icon + label (icon.tsx).",
		box(ic.Node(), 520, 140))

	// ---------- custom-indicator.tsx ----------
	ind := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: pane("Tab 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: pane("Tab 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: pane("Tab 3")},
	))
	ind.SetIndicator(kit.TabsIndicator{Size: 32, Align: kit.TabsIndicatorCenter})
	secInd := demoSection(face, th, "Indicator",
		"indicator size=32 align=center (custom-indicator.tsx).",
		box(ind.Node(), 520, 140))

	// ---------- slide.tsx ----------
	var slideItems []kit.TabItem
	for i := 1; i <= 12; i++ {
		k := fmt.Sprintf("%d", i)
		slideItems = append(slideItems, kit.TabItem{
			Key: k, Label: "Tab " + k, Children: pane("Content of tab " + k),
		})
	}
	slide := wire(kit.NewTabs(slideItems...))
	slide.SetOnChange(func(k string) { *c.status = fmt.Sprintf("tabs slide → %s", k) })
	secSlide := demoSection(face, th, "Slide",
		"many tabs; bar ScrollViewport when overflowing (slide.tsx).",
		box(slide.Node(), 420, 140))

	// ---------- extra.tsx ----------
	extraBtn := kit.NewButton("Extra Action")
	extraBtn.SetFace(face)
	extraBtn.SetOnClick(func() { *c.status = "tabs extra action" })
	extraLeft := kit.NewText("Left Extra")
	extraLeft.SetFace(face)
	ex := wire(kit.NewTabs(
		kit.TabItem{Key: "1", Label: "Tab 1", Children: pane("Tab 1")},
		kit.TabItem{Key: "2", Label: "Tab 2", Children: pane("Tab 2")},
		kit.TabItem{Key: "3", Label: "Tab 3", Children: pane("Tab 3")},
	))
	ex.SetTabBarExtraContent(extraLeft.Node(), extraBtn.Node())
	secExtra := demoSection(face, th, "Extra content",
		"tabBarExtraContent left + right (extra.tsx).",
		box(ex.Node(), 560, 140))

	// ---------- size.tsx ----------
	mkSize := func(sz kit.TabsSize, title string) core.Node {
		t := wire(kit.NewTabs(
			kit.TabItem{Key: "1", Label: "Tab 1", Children: pane(title)},
			kit.TabItem{Key: "2", Label: "Tab 2", Children: pane(title)},
			kit.TabItem{Key: "3", Label: "Tab 3", Children: pane(title)},
		))
		t.SetSize(sz)
		return box(t.Node(), 480, 120)
	}
	sizeCol := primitive.Column(
		mkSize(kit.TabsSmall, "small"),
		mkSize(kit.TabsMiddle, "medium"),
		mkSize(kit.TabsLarge, "large"),
	)
	sizeCol.Gap = 12
	sizeCol.CrossAlign = core.CrossStretch
	secSize := demoSection(face, th, "Size",
		"size small / medium / large (size.tsx).",
		sizeCol)

	c.addPage("tabs", "Tabs", demoPage(face,
		"Tabs",
		"选项卡切换 · docs/antd/tabs.md §6 P0（basic/disabled/centered/icon/indicator/slide/extra/size）。P1：placement 全页、card/editable 全页、自定义触发器。",
		secBasic, secDisabled, secCentered, secIcon, secInd, secSlide, secExtra, secSize,
	))
}
