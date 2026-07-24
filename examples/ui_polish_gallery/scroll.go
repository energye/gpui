//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// mkScrollLines builds overflow sample content shared by Scroll / Scrollbar demos.
func (c *catalogCtx) mkScrollLines(prefix string, n int) core.Node {
	inner := primitive.Column()
	inner.Gap = 2
	for i := 0; i < n; i++ {
		line := kit.NewText(fmt.Sprintf("%s · line %02d — wheel / drag thumb / track click", prefix, i+1))
		line.SetFace(c.face)
		inner.AddChild(line.Node())
	}
	return inner
}

func (c *catalogCtx) registerScroll() {
	// Scroll is the overflow container; Scrollbar chrome policy is configured on it.
	// Interactive live panel: switch visibility policy at runtime.
	live := kit.NewScroll(c.mkScrollLines("live", 20))
	live.SetSize(420, 140)
	live.SetScrollbar(primitive.DefaultScrollbar()) // Auto + non-overlap (default)
	*c.tickers = append(*c.tickers, live)

	policyLab := kit.NewText("policy: Auto — 溢出显示，内容区已减去条宽")
	policyLab.SetFace(c.face)
	policyLab.SetSecondary(true)

	setPolicy := func(name string, vis primitive.ScrollbarVisibility, overlay bool) {
		b := primitive.DefaultScrollbar()
		b.Vertical = vis
		b.Horizontal = vis
		b.Enabled = vis != primitive.ScrollbarNever
		b.Overlay = overlay
		b.Thickness = 6
		b.HoverThickness = 10
		b.AutoHideDelay = 1.2
		live.SetScrollbar(b)
		*c.status = "scrollbar " + name
		switch vis {
		case primitive.ScrollbarHover:
			policyLab.SetValue("policy: Hover — 溢出+悬停/滚轮显示；内容不重叠条")
		case primitive.ScrollbarAuto:
			policyLab.SetValue("policy: Auto — 溢出即显示；内容布局减去条宽/高")
		case primitive.ScrollbarAlways:
			policyLab.SetValue("policy: Always — 始终显示 track；内容不重叠")
		case primitive.ScrollbarNever:
			policyLab.SetValue("policy: Never — 无条；内容可用全宽")
		}
	}

	btnHover := kit.NewButton("Hover")
	btnHover.SetFace(c.face)
	btnHover.SetOnClick(func() { setPolicy("Hover", primitive.ScrollbarHover, false) })
	btnAuto := kit.NewButton("Auto")
	btnAuto.SetFace(c.face)
	btnAuto.SetType(kit.ButtonPrimary)
	btnAuto.SetOnClick(func() { setPolicy("Auto", primitive.ScrollbarAuto, false) })
	btnAlways := kit.NewButton("Always")
	btnAlways.SetFace(c.face)
	btnAlways.SetOnClick(func() { setPolicy("Always", primitive.ScrollbarAlways, false) })
	btnNever := kit.NewButton("Never")
	btnNever.SetFace(c.face)
	btnNever.SetOnClick(func() { setPolicy("Never", primitive.ScrollbarNever, false) })
	btnThick := kit.NewButton("Thick 12")
	btnThick.SetFace(c.face)
	btnThick.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetThickness(12).SetHoverThickness(18).SetMinThumb(28)
		})
		*c.status = "scrollbar thickness=12 hover=18"
	})
	btnTrack := kit.NewButton("Track on/off")
	btnTrack.SetFace(c.face)
	trackOn := true
	btnTrack.SetOnClick(func() {
		trackOn = !trackOn
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) { b.SetShowTrack(trackOn) })
		*c.status = fmt.Sprintf("showTrack=%v", trackOn)
	})
	btnHoverGrow := kit.NewButton("Hover grow")
	btnHoverGrow.SetFace(c.face)
	btnHoverGrow.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetExpandOnHover(true).SetThickness(6).SetHoverThickness(14)
		})
		*c.status = "expand on hover 6→14"
	})
	btnColor := kit.NewButton("Blue thumb")
	btnColor.SetFace(c.face)
	btnColor.SetOnClick(func() {
		live.ConfigureScrollbar(func(b *primitive.Scrollbar) {
			b.SetColors(
				render.RGBA{R: 0.9, G: 0.92, B: 0.96, A: 1},
				render.RGBA{R: 0.15, G: 0.45, B: 0.95, A: 0.85},
				render.RGBA{R: 0.1, G: 0.35, B: 0.9, A: 1},
			)
		})
		*c.status = "custom track/thumb colors"
	})
	*c.buttons = append(*c.buttons, btnHover, btnAuto, btnAlways, btnNever, btnThick, btnTrack, btnHoverGrow, btnColor)

	c.add("scroll", "Scroll", "Other · Scroll 容器（启用滚动）",
		sec(c.face, "垂直溢出 · 默认 Auto 条（内容区已减去条宽）"),
		live.Node(),
		policyLab.Node(),
		sec(c.face, "切换 Scrollbar 策略（独立控件配置）"),
		primitive.Row(btnHover.Node(), btnAuto.Node(), btnAlways.Node(), btnNever.Node()),
		primitive.Row(btnThick.Node(), btnTrack.Node(), btnHoverGrow.Node(), btnColor.Node()),
	)
}
