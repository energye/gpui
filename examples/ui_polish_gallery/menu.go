//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerMenu() {
	// Menu — docs/antd/menu.md §6.8 P0
	// https://ant.design/components/menu
	// demos: horizontal / inline / inline-collapsed / tooltip /
	//        sider-current / vertical / theme / submenu-theme
	// P1 deferred: switch-mode, style-class, custom-popup-render, semantic

	face, th := c.face, c.theme
	status := c.status

	playground := func(body core.Node, w, h float64) core.Node {
		d := primitive.NewDecorated(body)
		d.Padding = primitive.All(8)
		d.Radius = 6
		d.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		if w > 0 {
			d.Width = w
			d.MinWidth = w
		}
		if h > 0 {
			d.MinHeight = h
		}
		d.ExpandWidth = w <= 0
		return d
	}

	track := func(m *kit.Menu) *kit.Menu {
		m.SetFace(face)
		m.Theme = th
		return m
	}

	// Shared nested items (inline / collapsed / vertical family).
	navItems := []kit.MenuItem{
		{Key: "1", Label: "Option 1", Icon: "info"},
		{Key: "2", Label: "Option 2", Icon: "star"},
		{Key: "3", Label: "Option 3", Icon: "heart"},
		{
			Key:   "sub1",
			Label: "Navigation One",
			Icon:  "info",
			Children: []kit.MenuItem{
				{Key: "5", Label: "Option 5"},
				{Key: "6", Label: "Option 6"},
				{Key: "7", Label: "Option 7"},
				{Key: "8", Label: "Option 8"},
			},
		},
		{
			Key:   "sub2",
			Label: "Navigation Two",
			Icon:  "star",
			Children: []kit.MenuItem{
				{Key: "9", Label: "Option 9"},
				{Key: "10", Label: "Option 10"},
				{
					Key:   "sub3",
					Label: "Submenu",
					Children: []kit.MenuItem{
						{Key: "11", Label: "Option 11"},
						{Key: "12", Label: "Option 12"},
					},
				},
			},
		},
	}

	// ── horizontal.tsx ─────────────────────────────────────────
	hItems := []kit.MenuItem{
		{Key: "mail", Label: "Navigation One", Icon: "info"},
		{Key: "app", Label: "Navigation Two", Icon: "star", Disabled: true},
		{
			Key:   "SubMenu",
			Label: "Navigation Three",
			Icon:  "heart",
			Children: []kit.MenuItem{
				{Group: true, Label: "Item 1", Children: []kit.MenuItem{
					{Key: "setting:1", Label: "Option 1"},
					{Key: "setting:2", Label: "Option 2"},
				}},
				{Group: true, Label: "Item 2", Children: []kit.MenuItem{
					{Key: "setting:3", Label: "Option 3"},
					{Key: "setting:4", Label: "Option 4"},
				}},
			},
		},
		{Key: "alipay", Label: "Navigation Four"},
	}
	mHoriz := track(kit.NewMenu(hItems...))
	mHoriz.SetMode(kit.MenuModeHorizontal)
	mHoriz.SetDefaultSelectedKeys("mail")
	mHoriz.SetOnClick(func(info kit.MenuInfo) {
		*status = "horizontal click=" + info.Key
		mHoriz.SetSelectedKeys(info.Key)
	})

	// ── inline.tsx ─────────────────────────────────────────────
	mInline := track(kit.NewMenu(navItems...))
	mInline.SetMode(kit.MenuModeInline)
	mInline.SetDefaultSelectedKeys("1")
	mInline.SetDefaultOpenKeys("sub1")
	mInline.SetOnClick(func(info kit.MenuInfo) {
		*status = "inline click=" + info.Key
	})

	// ── inline-collapsed.tsx ───────────────────────────────────
	mCollapsed := track(kit.NewMenu(navItems...))
	mCollapsed.SetMode(kit.MenuModeInline)
	mCollapsed.SetColorTheme(kit.MenuColorDark)
	mCollapsed.SetDefaultSelectedKeys("1")
	mCollapsed.SetDefaultOpenKeys("sub1")
	collapsed := false
	btnCollapse := c.trackBtn(kit.NewButton("Toggle collapsed"))
	btnCollapse.SetType(kit.ButtonPrimary)
	btnCollapse.SetOnClick(func() {
		collapsed = !collapsed
		mCollapsed.SetInlineCollapsed(collapsed)
		*status = fmt.Sprintf("inlineCollapsed=%v", collapsed)
	})

	// ── tooltip.tsx (collapsed + tooltip switch) ───────────────
	mTip := track(kit.NewMenu(navItems...))
	mTip.SetMode(kit.MenuModeInline)
	mTip.SetColorTheme(kit.MenuColorDark)
	mTip.SetInlineCollapsed(true)
	mTip.SetDefaultSelectedKeys("1")
	tipOn := true
	btnTip := c.trackBtn(kit.NewButton("Tooltip On/Off"))
	btnTip.SetOnClick(func() {
		tipOn = !tipOn
		mTip.SetTooltipEnabled(tipOn)
		*status = fmt.Sprintf("menu tooltip=%v", tipOn)
	})

	// ── sider-current.tsx (only one parent open) ───────────────
	mSider := track(kit.NewMenu(navItems...))
	mSider.SetMode(kit.MenuModeInline)
	mSider.SetDefaultOpenKeys("sub1")
	mSider.SetDefaultSelectedKeys("1")
	mSider.SetOnOpenChange(func(keys []string) {
		// Keep only the latest root SubMenu open.
		if len(keys) == 0 {
			mSider.SetOpenKeys()
			return
		}
		mSider.SetOpenKeys(keys[len(keys)-1])
		*status = "sider open=" + keys[len(keys)-1]
	})
	mSider.SetOnClick(func(info kit.MenuInfo) {
		*status = "sider click=" + info.Key
	})

	// ── vertical.tsx ───────────────────────────────────────────
	mVert := track(kit.NewMenu(navItems...))
	mVert.SetMode(kit.MenuModeVertical)
	mVert.SetDefaultSelectedKeys("1")
	mVert.SetOnClick(func(info kit.MenuInfo) {
		*status = "vertical click=" + info.Key
	})

	// ── theme.tsx ──────────────────────────────────────────────
	mTheme := track(kit.NewMenu(navItems...))
	mTheme.SetMode(kit.MenuModeInline)
	mTheme.SetDefaultSelectedKeys("1")
	mTheme.SetDefaultOpenKeys("sub1")
	dark := false
	btnTheme := c.trackBtn(kit.NewButton("Light / Dark"))
	btnTheme.SetOnClick(func() {
		dark = !dark
		if dark {
			mTheme.SetColorTheme(kit.MenuColorDark)
		} else {
			mTheme.SetColorTheme(kit.MenuColorLight)
		}
		*status = fmt.Sprintf("menu theme dark=%v", dark)
	})

	// ── submenu-theme.tsx ──────────────────────────────────────
	subThemeDark := true
	mkSubItems := func(darkSub bool) []kit.MenuItem {
		subCT := kit.MenuColorLight
		if darkSub {
			subCT = kit.MenuColorDark
		}
		return []kit.MenuItem{
			{
				Key:           "sub1",
				Label:         "Navigation One",
				Icon:          "info",
				ColorTheme:    subCT,
				ColorThemeSet: true,
				Children: []kit.MenuItem{
					{Key: "1", Label: "Option 1"},
					{Key: "2", Label: "Option 2"},
					{Key: "3", Label: "Option 3"},
				},
			},
			{Key: "5", Label: "Option 5"},
			{Key: "6", Label: "Option 6"},
		}
	}
	mSubTheme := track(kit.NewMenu(mkSubItems(true)...))
	mSubTheme.SetMode(kit.MenuModeVertical)
	mSubTheme.SetColorTheme(kit.MenuColorDark)
	mSubTheme.SetOpenKeys("sub1")
	mSubTheme.SetSelectedKeys("1")
	mSubTheme.SetOnClick(func(info kit.MenuInfo) {
		*status = "submenu-theme click=" + info.Key
		mSubTheme.SetSelectedKeys(info.Key)
	})
	btnSubTheme := c.trackBtn(kit.NewButton("SubMenu Light/Dark"))
	btnSubTheme.SetOnClick(func() {
		subThemeDark = !subThemeDark
		mSubTheme.SetItems(mkSubItems(subThemeDark)...)
		mSubTheme.SetOpenKeys("sub1")
		*status = fmt.Sprintf("submenu theme dark=%v", subThemeDark)
	})

	c.addPage("menu", "Menu", demoPage(face,
		"Menu 导航菜单",
		"为页面和功能提供导航的菜单列表。P0 对齐 docs/antd/menu.md §6（mode / items / selectedKeys / openKeys / theme / inlineCollapsed / onClick / onOpenChange）。",
		demoSection(face, th, "顶部导航 horizontal",
			"mode=horizontal；disabled 项；SubMenu 分组弹出。",
			playground(mHoriz.Node(), 0, 56)),
		demoSection(face, th, "内嵌菜单 inline",
			"mode=inline；defaultOpenKeys / defaultSelectedKeys。",
			playground(mInline.Node(), 256, 0)),
		demoSection(face, th, "缩起内嵌菜单 inline-collapsed",
			"theme=dark + Toggle collapsed（宽 80）。",
			primitive.Column(btnCollapse.Node(), playground(mCollapsed.Node(), 256, 0))),
		demoSection(face, th, "菜单项提示 tooltip",
			"inlineCollapsed + tooltip 开关（折叠时 Title/Label 悬浮）。",
			primitive.Column(btnTip.Node(), playground(mTip.Node(), 80, 0))),
		demoSection(face, th, "只展开当前父级 sider-current",
			"受控 openKeys：OnOpenChange 只保留最新父级。",
			playground(mSider.Node(), 256, 0)),
		demoSection(face, th, "垂直菜单 vertical",
			"mode=vertical；SubMenu 右侧弹出。",
			playground(mVert.Node(), 256, 0)),
		demoSection(face, th, "主题 theme",
			"light / dark 色板切换。",
			primitive.Column(btnTheme.Node(), playground(mTheme.Node(), 256, 0))),
		demoSection(face, th, "子菜单主题 submenu-theme",
			"根 dark；SubMenu ColorTheme 覆盖，按钮切换子菜单 light/dark。",
			primitive.Column(btnSubTheme.Node(), playground(mSubTheme.Node(), 256, 0))),
	))
}
