//go:build linux && !nogpu

package main

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerAlert() {
	// Alert — docs/antd/alert.md §6.8 P0
	// https://ant.design/components/alert
	// demos: basic / style / filled / closable / description / icon / banner / loop-banner
	//
	// P1 not shown: smooth-closed motion, ErrorBoundary, full action.tsx matrix,
	// custom-title-alignment, style-class semantic, ConfigProvider global icons,
	// debug component-token / custom-icon.

	face, th := c.face, c.theme
	status := c.status

	track := func(al *kit.Alert) *kit.Alert {
		al.SetFace(face)
		if th != nil {
			al.SetTheme(th)
		}
		return al
	}

	// ---------- basic.tsx ----------
	basic := track(kit.NewAlert("Success Text"))
	basic.SetType(kit.AlertSuccess)
	secBasic := demoSection(face, th, "基本",
		"最简单的用法；默认 type 以外的显式 success。",
		basic.Node())

	// ---------- style.tsx 四种样式 ----------
	styles := primitive.Column()
	styles.Gap = 12
	styles.CrossAlign = core.CrossStretch
	for _, typ := range []kit.AlertType{kit.AlertSuccess, kit.AlertInfo, kit.AlertWarning, kit.AlertError} {
		al := track(kit.NewAlert(string(typ) + " Text"))
		al.SetType(typ)
		styles.AddChild(al.Node())
	}
	secStyle := demoSection(face, th, "四种样式",
		"共有四种样式 success / info / warning / error。",
		styles)

	// ---------- filled.tsx 无边框 ----------
	filled := track(kit.NewAlert("Info Text"))
	filled.SetType(kit.AlertInfo)
	filled.SetVariant(kit.AlertFilled)
	secFilled := demoSection(face, th, "无边框",
		"variant=filled，无描边。",
		filled.Node())

	// ---------- closable.tsx ----------
	closables := primitive.Column()
	closables.Gap = 12
	closables.CrossAlign = core.CrossStretch
	for _, typ := range []kit.AlertType{kit.AlertWarning, kit.AlertSuccess, kit.AlertInfo, kit.AlertError} {
		al := track(kit.NewAlert(string(typ) + " Title"))
		al.SetType(typ)
		al.SetClosable(true)
		al.SetCloseAria("close")
		typ := typ
		al.SetOnClose(func() {
			if status != nil {
				*status = "Alert closed: " + string(typ)
			}
		})
		closables.AddChild(al.Node())
	}
	secClose := demoSection(face, th, "可关闭的警告提示",
		"closable + onClose；P0 瞬时隐藏。",
		closables)

	// ---------- description.tsx ----------
	descs := primitive.Column()
	descs.Gap = 12
	descs.CrossAlign = core.CrossStretch
	for _, typ := range []kit.AlertType{kit.AlertSuccess, kit.AlertInfo, kit.AlertWarning, kit.AlertError} {
		al := track(kit.NewAlert(string(typ) + " Text"))
		al.SetType(typ)
		al.SetDescription(string(typ) + " Description " + string(typ) + " Description " + string(typ) + " Description")
		descs.AddChild(al.Node())
	}
	secDesc := demoSection(face, th, "含有辅助性文字介绍",
		"含有 description 的 Alert，title 用 fontSizeLG。",
		descs)

	// ---------- icon.tsx ----------
	icons := primitive.Column()
	icons.Gap = 12
	icons.CrossAlign = core.CrossStretch
	iconCases := []struct {
		title string
		typ   kit.AlertType
		desc  string
		close bool
	}{
		{"Success Tips", kit.AlertSuccess, "", false},
		{"Informational Notes", kit.AlertInfo, "", false},
		{"Warning", kit.AlertWarning, "", true},
		{"Error", kit.AlertError, "", false},
		{"Success Tips", kit.AlertSuccess, "Detailed description and advice about successful copywriting.", false},
		{"Informational Notes", kit.AlertInfo, "Additional description and information about copywriting.", false},
		{"Warning", kit.AlertWarning, "This is a warning notice about copywriting.", true},
		{"Error", kit.AlertError, "This is an error message about copywriting.", false},
	}
	for _, c := range iconCases {
		al := track(kit.NewAlert(c.title))
		al.SetType(c.typ)
		al.SetShowIcon(true)
		if c.desc != "" {
			al.SetDescription(c.desc)
		}
		if c.close {
			al.SetClosable(true)
		}
		icons.AddChild(al.Node())
	}
	secIcon := demoSection(face, th, "图标",
		"showIcon；可与 description / closable 组合。",
		icons)

	// ---------- banner.tsx ----------
	banners := primitive.Column()
	banners.Gap = 12
	banners.CrossAlign = core.CrossStretch
	b1 := track(kit.NewAlert("Warning text"))
	b1.SetBanner(true)
	b2 := track(kit.NewAlert("Very long warning text warning text text text text text text text"))
	b2.SetBanner(true)
	b2.SetClosable(true)
	b3 := track(kit.NewAlert("Warning text without icon"))
	b3.SetBanner(true)
	b3.SetShowIcon(false)
	b4 := track(kit.NewAlert("Error text"))
	b4.SetBanner(true)
	b4.SetType(kit.AlertError)
	for _, al := range []*kit.Alert{b1, b2, b3, b4} {
		banners.AddChild(al.Node())
	}
	secBanner := demoSection(face, th, "顶部公告",
		"banner 模式：直角、无边；默认 type=warning、showIcon。",
		banners)

	// ---------- loop-banner.tsx ----------
	// TitleNode 模拟 marquee 文案（P0 不强制像素级跑马灯；长标题 + banner）
	loopTitle := kit.NewText("I can be a React component, multiple React components, or just some text.")
	loopTitle.SetFace(face)
	if th != nil {
		// keep text theme-aligned when available
		loopTitle.SetTheme(th)
	}
	loop := track(kit.NewAlert(""))
	loop.SetBanner(true)
	loop.SetTitleNode(loopTitle.Node())
	secLoop := demoSection(face, th, "轮播的公告",
		"banner + TitleNode（官方用 Marquee；P0 以自定义标题节点等价）。",
		loop.Node())

	// ---------- action slot (P0 槽位；完整 action.tsx 矩阵 P1) ----------
	undo := kit.NewButton("UNDO")
	undo.SetType(kit.ButtonText)
	undo.SetSize(kit.ButtonSmall)
	undo.SetFace(face)
	act := track(kit.NewAlert("Success Tips"))
	act.SetType(kit.AlertSuccess)
	act.SetShowIcon(true)
	act.SetAction(undo.Node())
	act.SetClosable(true)
	secAction := demoSection(face, th, "操作（槽位）",
		"action 可挂 Button；完整多按钮矩阵见 P1。",
		act.Node())

	page := demoPage(face, "Alert 警告提示",
		"Ant Design Alert · docs/antd/alert.md §6.8 P0",
		secBasic, secStyle, secFilled, secClose, secDesc, secIcon, secBanner, secLoop, secAction)
	c.addPage("alert", "Alert", page)
}
