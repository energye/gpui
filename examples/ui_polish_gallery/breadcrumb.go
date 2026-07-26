//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerBreadcrumb() {
	// Breadcrumb — antd demos §6.8 P0:
	// basic / withIcon / withParams / separator / overlay /
	// separator-component / debug-routes
	// https://ant.design/components/breadcrumb · components/breadcrumb/demo/*.tsx
	//
	// P1 not shown: style-class / semantic classNames/styles / component-token.

	face, th := c.face, c.theme
	wire := func(b *kit.Breadcrumb, tag string) *kit.Breadcrumb {
		b.SetFace(face)
		b.SetTheme(th)
		b.SetOnClick(func(i int, it kit.BreadcrumbItem) {
			*c.status = fmt.Sprintf("breadcrumb %s click → [%d] %s", tag, i, it.Title)
		})
		b.SetOnMenuClick(func(i int, key string) {
			*c.status = fmt.Sprintf("breadcrumb %s menu → item=%d key=%s", tag, i, key)
		})
		return b
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 16
		f.CrossAlign = core.CrossStretch
		f.MainAlign = core.MainStart
		return f
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Title: "An Application"},
	), "basic")
	secBasic := demoSection(face, th, "基本",
		"antd basic.tsx：四项；中间两项 Link；末项非链接强调。",
		basic.Node())

	// ---------- withIcon.tsx ----------
	withIcon := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Link: true, Icon: "user"},
		kit.BreadcrumbItem{Link: true, Icon: "user", Title: "Application List"},
		kit.BreadcrumbItem{Title: "Application"},
	), "withIcon")
	secIcon := demoSection(face, th, "带有图标的",
		"antd withIcon.tsx：Icon + Title 混排（registry: user）。",
		withIcon.Node())

	// ---------- withParams.tsx ----------
	withParams := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Users"},
		kit.BreadcrumbItem{Title: ":id", Link: true},
	), "withParams")
	withParams.SetParams(map[string]string{"id": "1"})
	secParams := demoSection(face, th, "带有参数的",
		"antd withParams.tsx：params 将 :id 替换为 1。",
		withParams.Node())

	// ---------- separator.tsx ----------
	sepRoot := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Title: "An Application"},
	), "separator")
	sepRoot.SetSeparator(">")
	secSep := demoSection(face, th, "分隔符",
		"antd separator.tsx：根 separator=\">\"。",
		sepRoot.Node())

	// ---------- overlay.tsx ----------
	overlay := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Ant Design"},
		kit.BreadcrumbItem{Title: "Component", Link: true},
		kit.BreadcrumbItem{
			Title: "General",
			Link:  true,
			Menu: []kit.MenuItem{
				{Key: "1", Label: "General"},
				{Key: "2", Label: "Layout"},
				{Key: "3", Label: "Navigation"},
			},
		},
		kit.BreadcrumbItem{Title: "Button"},
	), "overlay")
	secOverlay := demoSection(face, th, "带下拉菜单的面包屑",
		"antd overlay.tsx：General 项带 menu；hover 打开 Dropdown。",
		overlay.Node())

	// ---------- separator-component.tsx ----------
	sepComp := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Location"},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType, Separator: ":"},
		kit.BreadcrumbItem{Title: "Application Center", Link: true},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType},
		kit.BreadcrumbItem{Title: "Application List", Link: true},
		kit.BreadcrumbItem{Type: kit.BreadcrumbItemSeparatorType},
		kit.BreadcrumbItem{Title: "An Application"},
	), "separator-component")
	sepComp.SetSeparator("")
	secSepComp := demoSection(face, th, "独立的分隔符",
		"antd separator-component.tsx：type=separator 项；根 separator 空串关闭自动 sep。",
		sepComp.Node())

	// ---------- debug-routes.tsx ----------
	routes := kit.BreadcrumbFromRoutes(
		kit.BreadcrumbItem{Path: "/home", Title: "Home"},
		kit.BreadcrumbItem{
			Path:  "/user",
			Title: "User",
			Children: []kit.BreadcrumbItem{
				{Path: "/user1", Title: "User1"},
				{Path: "/user2", Title: "User2"},
			},
		},
	)
	debugRoutes := wire(kit.NewBreadcrumb(routes...), "debug-routes")
	secRoutes := demoSection(face, th, "Debug Routes",
		"antd debug-routes.tsx：legacy routes 映射 — path 拼接 + children→menu。",
		debugRoutes.Node())

	// Lifecycle (#9) — structureChange order (items → separator → theme).
	life := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Home", Link: true},
		kit.BreadcrumbItem{Title: "Temp"},
	), "life")
	life.SetItems([]kit.BreadcrumbItem{
		{Title: "Home", Link: true},
		{Title: "Docs", Link: true},
		{Title: "Breadcrumb"},
	})
	life.SetSeparator("›")
	life.SetTheme(th)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：SetItems → SetSeparator → SetTheme；ensureBuilt 保 Root 稳定。",
		life.Node())

	// Skin (#6) — Theme.Skin Override for kit.Breadcrumb (Flex.SkinType).
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeBreadcrumb, func(pc *core.PaintContext, n core.Node) {
		f, ok := n.(*primitive.Flex)
		if !ok || f == nil {
			return
		}
		if f.Base().Key == "bc-skin-demo" {
			sz := f.Size()
			if pc != nil && sz.Width > 0 && sz.Height > 0 {
				pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, 4, render.Hex("#FFF0F6"))
			}
		}
		if p := baseSkin.Painter(kit.TypeBreadcrumb); p != nil {
			p(pc, f)
			return
		}
		f.DefaultPaintChildren(pc)
	})
	skinBC := wire(kit.NewBreadcrumb(
		kit.BreadcrumbItem{Title: "Skin", Link: true},
		kit.BreadcrumbItem{Title: "Trail", Link: true},
		kit.BreadcrumbItem{Title: "Demo"},
	), "skin")
	if flex, ok := skinBC.ChromeNode().(*primitive.Flex); ok {
		flex.Base().Key = "bc-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"Root Flex.SkinType=kit.Breadcrumb。Key=bc-skin-demo 时 Override 画粉底；其它 Breadcrumb 不受影响。",
		skinBC.Node())

	c.addPage("breadcrumb", "Breadcrumb",
		demoPage(face, "Breadcrumb",
			"Ant Design Breadcrumb · docs/antd/breadcrumb.md §6 P0\n"+
				"items / title / type=separator / separator / params / href|path|Link / menu / onClick / dropdownIcon\n"+
				"Also verifies #9 lifecycle + #6 Skin.\n"+
				"P1 未展：semantic classNames/styles、style-class、component-token。",
			col(secBasic, secIcon, secParams, secSep, secOverlay, secSepComp, secRoutes, secLife, secSkin),
		))
}
