//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerCollapse() {
	// Collapse — docs/antd/collapse.md §6.8 P0
	// https://ant.design/components/collapse
	// demos: basic / size / accordion / mix / borderless / custom / noarrow / extra
	//
	// P1 not shown: ghost page, collapsible three-mode page, style-class / semantic,
	// ConfigProvider global, debug demos.

	face, th := c.face, c.theme
	status := c.status

	wire := func(col *kit.Collapse) *kit.Collapse {
		col.SetFace(face)
		if th != nil {
			col.SetTheme(th)
		}
		return col
	}
	txt := func(s string) core.Node {
		t := kit.NewText(s)
		t.SetFace(face)
		return t.Node()
	}
	dog := `A dog is a type of domesticated animal.
Known for its loyalty and faithfulness,
it can be found as a welcome guest in many households across the world.`

	mkItems := func() []kit.CollapseItem {
		return []kit.CollapseItem{
			{Key: "1", Label: "This is panel header 1", Children: txt(dog)},
			{Key: "2", Label: "This is panel header 2", Children: txt(dog)},
			{Key: "3", Label: "This is panel header 3", Children: txt(dog)},
		}
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewCollapse(mkItems()...))
	basic.SetDefaultActiveKey("1")
	basic.SetOnChange(func(keys []string) {
		if status != nil {
			*status = fmt.Sprintf("collapse basic active=%v", keys)
		}
	})
	secBasic := demoSection(face, th, "折叠面板",
		"basic.tsx：items + defaultActiveKey=['1'] + onChange。",
		basic.Node())

	// ---------- size.tsx ----------
	sizeMid := wire(kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "This is medium size panel header", Children: txt(dog),
	}))
	sizeSM := wire(kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "This is small size panel header", Children: txt(dog),
	}))
	sizeSM.SetSize(kit.CollapseSmall)
	sizeLG := wire(kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "This is large size panel header", Children: txt(dog),
	}))
	sizeLG.SetSize(kit.CollapseLarge)
	sizeCol := primitive.Column(sizeMid.Node(), sizeSM.Node(), sizeLG.Node())
	sizeCol.Gap = 16
	sizeCol.CrossAlign = core.CrossStretch
	secSize := demoSection(face, th, "面板尺寸",
		"size.tsx：medium / small / large（头/体内边距与 large 字号）。",
		sizeCol)

	// ---------- accordion.tsx ----------
	acc := wire(kit.NewCollapse(mkItems()...))
	acc.SetAccordion(true)
	acc.SetOnChange(func(keys []string) {
		if status != nil {
			*status = fmt.Sprintf("accordion active=%v", keys)
		}
	})
	secAcc := demoSection(face, th, "手风琴",
		"accordion.tsx：accordion=true，至多展开一项。",
		acc.Node())

	// ---------- mix.tsx ----------
	nest := wire(kit.NewCollapse(kit.CollapseItem{
		Key: "1", Label: "This is panel nest panel", Children: txt(dog),
	}))
	nest.SetDefaultActiveKey("1")
	mix := wire(kit.NewCollapse(
		kit.CollapseItem{Key: "1", Label: "This is panel header 1", Children: nest.Node()},
		kit.CollapseItem{Key: "2", Label: "This is panel header 2", Children: txt(dog)},
		kit.CollapseItem{Key: "3", Label: "This is panel header 3", Children: txt(dog)},
	))
	secMix := demoSection(face, th, "面板嵌套",
		"mix.tsx：item children 再挂 Collapse。",
		mix.Node())

	// ---------- borderless.tsx ----------
	bl := wire(kit.NewCollapse(mkItems()...))
	bl.SetBordered(false)
	bl.SetDefaultActiveKey("1")
	secBL := demoSection(face, th, "简洁风格",
		"borderless.tsx：bordered=false。",
		bl.Node())

	// ---------- custom.tsx ----------
	panelTh := th
	if panelTh == nil {
		panelTh = kit.DefaultTheme()
	}
	panelBG := panelTh.Color(core.TokenColorFillSecondary)
	customItems := make([]kit.CollapseItem, 3)
	for i, key := range []string{"1", "2", "3"} {
		customItems[i] = kit.CollapseItem{
			Key:      key,
			Label:    fmt.Sprintf("This is panel header %s", key),
			Children: txt(dog),
			Style: kit.Style{
				Background:  panelBG,
				Radius:      kit.DefaultCollapseRadius,
				ForceRadius: true,
			},
		}
	}
	custom := wire(kit.NewCollapse(customItems...))
	custom.SetBordered(false)
	custom.SetDefaultActiveKey("1")
	custom.SetExpandIcon(func(active bool, _ kit.CollapseItem) core.Node {
		s := "▷"
		if active {
			s = "▼"
		}
		t := kit.NewText(s)
		t.SetFace(face)
		return t.Node()
	})
	secCustom := demoSection(face, th, "自定义面板",
		"custom.tsx：borderless + 自定义 expandIcon + item 背景圆角。",
		custom.Node())

	// ---------- noarrow.tsx ----------
	no := false
	noarrow := wire(kit.NewCollapse(
		kit.CollapseItem{Key: "1", Label: "This is panel header with arrow icon", Children: txt(dog)},
		kit.CollapseItem{Key: "2", Label: "This is panel header with no arrow icon", Children: txt(dog), ShowArrow: &no},
	))
	noarrow.SetDefaultActiveKey("1")
	secNoArrow := demoSection(face, th, "隐藏箭头",
		"noarrow.tsx：item showArrow=false。",
		noarrow.Node())

	// ---------- extra.tsx ----------
	mkExtra := func() core.Node {
		b := c.trackBtn(kit.NewButton("⚙"))
		b.SetType(kit.ButtonText)
		b.SetOnClick(func() {
			if status != nil {
				*status = "collapse extra clicked (no toggle)"
			}
		})
		return b.Node()
	}
	extra := wire(kit.NewCollapse(
		kit.CollapseItem{Key: "1", Label: "This is panel header 1", Children: txt(dog), Extra: mkExtra()},
		kit.CollapseItem{Key: "2", Label: "This is panel header 2", Children: txt(dog), Extra: mkExtra()},
		kit.CollapseItem{Key: "3", Label: "This is panel header 3", Children: txt(dog), Extra: mkExtra()},
	))
	extra.SetDefaultActiveKey("1")
	placeStart := c.trackBtn(kit.NewButton("start"))
	placeEnd := c.trackBtn(kit.NewButton("end"))
	placeStart.SetType(kit.ButtonPrimary)
	placeStart.SetOnClick(func() {
		extra.SetExpandIconPlacement(kit.CollapseIconStart)
		if status != nil {
			*status = "expandIconPlacement=start"
		}
	})
	placeEnd.SetOnClick(func() {
		extra.SetExpandIconPlacement(kit.CollapseIconEnd)
		if status != nil {
			*status = "expandIconPlacement=end"
		}
	})
	placeRow := primitive.Row(txt("Expand Icon Placement: "), placeStart.Node(), placeEnd.Node())
	placeRow.Gap = 8
	placeRow.CrossAlign = core.CrossCenter
	extraCol := primitive.Column(extra.Node(), placeRow)
	extraCol.Gap = 12
	extraCol.CrossAlign = core.CrossStretch
	secExtra := demoSection(face, th, "额外节点",
		"extra.tsx：item.extra（点击不折叠）+ expandIconPlacement start|end。",
		extraCol)

	page := demoPage(face, "Collapse 折叠面板",
		"可以折叠/展开的内容区域。P0 对齐 docs/antd/collapse.md §6（items / activeKey / accordion / size / bordered / showArrow / extra / expandIcon）。",
		secBasic, secSize, secAcc, secMix, secBL, secCustom, secNoArrow, secExtra)
	c.addPage("collapse", "Collapse", page)
}
