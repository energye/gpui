//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerScrollbar() {
	// Static side-by-side samples of Scrollbar visibility policy.
	sample := func(vis primitive.ScrollbarVisibility, title string) core.Node {
		sc := kit.NewScroll(c.mkScrollLines(title, 12))
		sc.SetSize(200, 96)
		sc.SetScrollbarVisibility(vis)
		sc.SetOverlay(false) // content never under bar
		if vis == primitive.ScrollbarHover {
			*c.tickers = append(*c.tickers, sc)
		}
		box := primitive.Column(
			sec(c.face, title),
			sc.Node(),
		)
		box.Gap = 6
		return box
	}
	samples := primitive.Row(
		sample(primitive.ScrollbarHover, "Hover"),
		sample(primitive.ScrollbarAuto, "Auto"),
		sample(primitive.ScrollbarAlways, "Always"),
		sample(primitive.ScrollbarNever, "Never"),
	)
	samples.Gap = 12
	samples.CrossAlign = core.CrossStart

	// Horizontal scroll demo
	hInner := primitive.Row()
	hInner.Gap = 8
	for i := 0; i < 12; i++ {
		cell := primitive.NewBox()
		cell.Width, cell.Height = 72, 48
		cell.Color = render.RGBA{R: 0.85, G: 0.9, B: 0.98, A: 1}
		lab := kit.NewText(fmt.Sprintf("H%d", i+1))
		lab.SetFace(c.face)
		stack := primitive.NewStack(cell, primitive.Positioned(core.AlignCenter, lab.Node()))
		hInner.AddChild(stack)
	}
	hScroll := kit.NewScroll(hInner)
	hScroll.SetSize(420, 64)
	hScroll.SetAxis(false, true)
	hBar := primitive.DefaultScrollbar()
	hBar.Vertical = primitive.ScrollbarNever
	hBar.Horizontal = primitive.ScrollbarAuto
	hBar.Overlay = false
	hScroll.SetScrollbar(hBar)

	// Lifecycle (#9)
	life := kit.NewScroll(c.mkScrollLines("Lifecycle", 12))
	life.SetSize(200, 96)
	life.SetScrollbarVisibility(primitive.ScrollbarAlways)
	life.SetOverlay(false)
	secLife := demoSection(c.face, c.theme, "Lifecycle (#9)",
		"structureChange/chromeChange 均走 ensureBuilt（Root ScrollViewport 稳定）。",
		life.Node())

	// Skin (#6) — Root 是 ScrollViewport；TypeID=kit.Scroll 已注册，Override 证明挂接点存在。
	baseSkin := c.theme.Skin
	c.theme.Skin = core.Override(baseSkin, kit.TypeScroll, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeScroll); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinSc := kit.NewScroll(c.mkScrollLines("Skin", 12))
	skinSc.SetSize(200, 96)
	skinSc.SetScrollbarVisibility(primitive.ScrollbarAlways)
	secSkin := demoSection(c.face, c.theme, "Skin painter (#6)",
		"TypeID=kit.Scroll 已注册；Theme.Skin Override 可挂接（ScrollViewport 自绘 bar chrome）。",
		skinSc.Node())

	c.add("scrollbar", "Scrollbar", "Other · Scrollbar 策略对照",
		sec(c.face, "同一溢出内容 · 四种显示策略并排"),
		samples,
		sec(c.face, "水平滚动 · Horizontal=Auto"),
		hScroll.Node(),
		sec(c.face, "配置项: Enabled / Visibility(Never·Auto·Always·Hover) / Overlay / Thickness / HoverThickness / MinThumb / AutoHideDelay / DragThumb / TrackClick / WheelStep / Colors"),
		secLife, secSkin,
	)
}
