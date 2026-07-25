//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerAvatar() {
	// Avatar — docs/antd/avatar.md §6.8 P0
	// https://ant.design/components/avatar
	// demos: basic / type / dynamic / badge / group / responsive
	//
	// P1 not shown: max.popover, real HTTP src decode, srcSet/crossOrigin,
	// semantic classNames, ConfigProvider global, debug demos.

	face, th := c.face, c.theme
	status := c.status

	track := func(a *kit.Avatar) *kit.Avatar {
		a.SetFace(face)
		if th != nil {
			a.SetTheme(th)
		}
		c.trackTicker(a)
		return a
	}

	// ---------- basic.tsx ----------
	mkIcon := func(shape kit.AvatarShape, set func(*kit.Avatar)) *kit.Avatar {
		a := track(kit.NewAvatarIcon("user"))
		a.SetShape(shape)
		if set != nil {
			set(a)
		}
		return a
	}
	rowCircle := spaceWrap(16,
		mkIcon(kit.AvatarCircle, func(a *kit.Avatar) { a.SetSizePx(64) }).Node(),
		mkIcon(kit.AvatarCircle, func(a *kit.Avatar) { a.SetSize(kit.AvatarLarge) }).Node(),
		mkIcon(kit.AvatarCircle, nil).Node(),
		mkIcon(kit.AvatarCircle, func(a *kit.Avatar) { a.SetSize(kit.AvatarSmall) }).Node(),
		mkIcon(kit.AvatarCircle, func(a *kit.Avatar) { a.SetSizePx(14) }).Node(),
	)
	rowSquare := spaceWrap(16,
		mkIcon(kit.AvatarSquare, func(a *kit.Avatar) { a.SetSizePx(64) }).Node(),
		mkIcon(kit.AvatarSquare, func(a *kit.Avatar) { a.SetSize(kit.AvatarLarge) }).Node(),
		mkIcon(kit.AvatarSquare, nil).Node(),
		mkIcon(kit.AvatarSquare, func(a *kit.Avatar) { a.SetSize(kit.AvatarSmall) }).Node(),
		mkIcon(kit.AvatarSquare, func(a *kit.Avatar) { a.SetSizePx(14) }).Node(),
	)
	basicCol := primitive.Column(rowCircle, rowSquare)
	basicCol.Gap = 16
	basicCol.CrossAlign = core.CrossStart
	secBasic := demoSection(face, th, "基本",
		"三种尺寸与 circle / square；自定义 number 尺寸。",
		basicCol)

	// ---------- type.tsx ----------
	typeIcon := track(kit.NewAvatarIcon("user"))
	typeLetter := track(kit.NewAvatar("U"))
	typeLong := track(kit.NewAvatar("USER"))
	typeLong.SetSizePx(40)
	typeSrc := track(kit.NewAvatar(""))
	typeSrc.SetSrc("demo://avatar.svg")
	typeSrc.SetImageOK(true)
	typeSrc.SetAlt("avatar")
	typeStyled := track(kit.NewAvatar("U"))
	typeStyled.SetStyle(kit.Style{
		Background: render.Hex("#fde3cf"),
		Text:       render.Hex("#f56a00"),
	})
	typeIconGreen := track(kit.NewAvatarIcon("user"))
	typeIconGreen.SetStyle(kit.Style{Background: render.Hex("#87d068")})
	secType := demoSection(face, th, "类型",
		"支持图标、字符、图片与 Style 自定义色。",
		spaceWrap(16,
			typeIcon.Node(), typeLetter.Node(), typeLong.Node(),
			typeSrc.Node(), typeStyled.Node(), typeIconGreen.Node(),
		))

	// ---------- dynamic.tsx ----------
	users := []string{"U", "Lucy", "Tom", "Edward"}
	colors := []string{"#f56a00", "#7265e6", "#ffbf00", "#00a2ae"}
	gaps := []float64{4, 3, 2, 1}
	ui, ci, gi := 0, 0, 0
	dyn := track(kit.NewAvatar(users[0]))
	dyn.SetSize(kit.AvatarLarge)
	dyn.SetGap(gaps[0])
	dyn.SetStyle(kit.Style{Background: render.Hex(colors[0])})
	btnUser := kit.NewButton("ChangeUser")
	btnUser.SetSize(kit.ButtonSmall)
	btnUser.SetOnClick(func() {
		ui = (ui + 1) % len(users)
		ci = (ci + 1) % len(colors)
		dyn.SetText(users[ui])
		dyn.SetStyle(kit.Style{Background: render.Hex(colors[ci])})
		if status != nil {
			*status = fmt.Sprintf("avatar user=%s", users[ui])
		}
	})
	*c.buttons = append(*c.buttons, btnUser)
	btnGap := kit.NewButton("changeGap")
	btnGap.SetSize(kit.ButtonSmall)
	btnGap.SetOnClick(func() {
		gi = (gi + 1) % len(gaps)
		dyn.SetGap(gaps[gi])
		if status != nil {
			*status = fmt.Sprintf("avatar gap=%.0f scale=%.2f", gaps[gi], dyn.TextScale)
		}
	})
	*c.buttons = append(*c.buttons, btnGap)
	secDyn := demoSection(face, th, "自动调整字符大小",
		"gap 控制字符左右留白；长串自动 scale。",
		spaceWrap(16, dyn.Node(), btnUser.Node(), btnGap.Node()))

	// ---------- badge.tsx ----------
	bAv1 := track(kit.NewAvatarIcon("user"))
	bAv1.SetShape(kit.AvatarSquare)
	badge1 := kit.NewBadge(bAv1.Node(), 1)
	bAv2 := track(kit.NewAvatarIcon("user"))
	bAv2.SetShape(kit.AvatarSquare)
	badge2 := kit.NewBadge(bAv2.Node(), 0)
	badge2.SetDot(true)
	secBadge := demoSection(face, th, "带徽标的头像",
		"通常与 Badge 组合使用。",
		spaceWrap(24, badge1.Node(), badge2.Node()))

	// ---------- group.tsx ----------
	mkG := func(txt, bg string) *kit.Avatar {
		a := track(kit.NewAvatar(txt))
		if bg != "" {
			a.SetStyle(kit.Style{Background: render.Hex(bg)})
		}
		return a
	}
	g1 := kit.NewAvatarGroup(
		func() *kit.Avatar {
			a := track(kit.NewAvatar(""))
			a.SetSrc("demo://g1")
			a.SetImageOK(true)
			return a
		}(),
		mkG("K", "#f56a00"),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("user"))
			a.SetStyle(kit.Style{Background: render.Hex("#87d068")})
			return a
		}(),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("info"))
			a.SetStyle(kit.Style{Background: render.Hex("#1677ff")})
			return a
		}(),
	)
	g1.SetFace(face)

	g2 := kit.NewAvatarGroup(
		func() *kit.Avatar {
			a := track(kit.NewAvatar(""))
			a.SetSrc("demo://g2")
			a.SetImageOK(true)
			return a
		}(),
		mkG("K", "#f56a00"),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("user"))
			a.SetStyle(kit.Style{Background: render.Hex("#87d068")})
			return a
		}(),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("info"))
			a.SetStyle(kit.Style{Background: render.Hex("#1677ff")})
			return a
		}(),
	)
	g2.SetFace(face)
	g2.SetMaxCount(2)
	g2.SetMaxStyle(kit.Style{Background: render.Hex("#fde3cf"), Text: render.Hex("#f56a00")})

	g3 := kit.NewAvatarGroup(
		func() *kit.Avatar {
			a := track(kit.NewAvatar(""))
			a.SetSrc("demo://g3")
			a.SetImageOK(true)
			return a
		}(),
		mkG("K", "#f56a00"),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("user"))
			a.SetStyle(kit.Style{Background: render.Hex("#87d068")})
			return a
		}(),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("info"))
			a.SetStyle(kit.Style{Background: render.Hex("#1677ff")})
			return a
		}(),
	)
	g3.SetFace(face)
	g3.SetSize(kit.AvatarLarge)
	g3.SetMaxCount(2)
	g3.SetMaxStyle(kit.Style{Background: render.Hex("#fde3cf"), Text: render.Hex("#f56a00")})

	g4 := kit.NewAvatarGroup(
		mkG("A", "#fde3cf"),
		mkG("K", "#f56a00"),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("user"))
			a.SetStyle(kit.Style{Background: render.Hex("#87d068")})
			return a
		}(),
		func() *kit.Avatar {
			a := track(kit.NewAvatarIcon("info"))
			a.SetStyle(kit.Style{Background: render.Hex("#1677ff")})
			return a
		}(),
	)
	g4.SetFace(face)
	g4.SetShape(kit.AvatarSquare)

	groupCol := primitive.Column(g1.Node(), g2.Node(), g3.Node(), g4.Node())
	groupCol.Gap = 16
	groupCol.CrossAlign = core.CrossStart
	secGroup := demoSection(face, th, "Avatar.Group",
		"头像组合排列；max.count 溢出显示 +N（P0 不含 popover）。",
		groupCol)

	// ---------- responsive.tsx ----------
	resp := track(kit.NewAvatarIcon("info"))
	resp.SetResponsiveSize(kit.AvatarResponsiveSize{
		XS: 24, SM: 32, MD: 40, LG: 64, XL: 80, XXL: 100,
	})
	resp.SetBreakpoint("md")
	bpBtn := kit.NewButton("Cycle breakpoint")
	bps := []string{"xs", "sm", "md", "lg", "xl", "xxl"}
	bpi := 2
	bpBtn.SetOnClick(func() {
		bpi = (bpi + 1) % len(bps)
		resp.SetBreakpoint(bps[bpi])
		if status != nil {
			*status = fmt.Sprintf("avatar bp=%s size=%.0f", bps[bpi], resp.ResolvedSize())
		}
	})
	*c.buttons = append(*c.buttons, bpBtn)
	secResp := demoSection(face, th, "响应式尺寸",
		"SetResponsiveSize + SetBreakpoint 注入当前断点（桌面宿主）。",
		spaceWrap(16, resp.Node(), bpBtn.Node()))

	// ---------- loading / disabled 抽测 ----------
	load := track(kit.NewAvatar("L"))
	load.SetLoading(true)
	dis := track(kit.NewAvatar("D"))
	dis.SetDisabled(true)
	secExtra := demoSection(face, th, "Loading / Disabled",
		"loading 走 Ticker 旋转环；disabled 用禁用色。",
		spaceWrap(16, load.Node(), dis.Node()))

	c.add("avatar", "Avatar", "Data Display · Avatar",
		demoPage(face, "Avatar",
			"用来代表用户或事物，支持图片、图标或字符展示。P0：size/shape/icon/text+gap/src+onError、Group max、响应式、Token、loading Ticker。",
			secBasic, secType, secDyn, secBadge, secGroup, secResp, secExtra))
}
