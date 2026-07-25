//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTour() {
	// Tour — Ant Design demos (docs/antd/tour.md §6.8 P0)
	// https://ant.design/components/tour
	// P0: basic / non-modal / placement / mask / indicator / actions-render / gap / style-class
	// P1 (not shown): scrollIntoViewOptions, getPopupContainer, arrow pixel geometry,
	// semantic classNames/styles 函数形态, ConfigProvider 全局, debug-panel, 官网逐像素

	face, th := c.face, c.theme
	status := c.status

	wire := func(tr *kit.Tour) *kit.Tour {
		tr.SetFace(face)
		if th != nil {
			tr.SetTheme(th)
		}
		tr.Viewport = core.Size{Width: 960, Height: 640}
		c.trackTicker(tr)
		return tr
	}

	// Shared target rects (absolute within overlay viewport — host would update from real widgets).
	t1 := core.NewRect(48, 120, 88, 32)
	t2 := core.NewRect(160, 120, 88, 32)
	t3 := core.NewRect(280, 120, 48, 32)

	// Cover stand-in (antd uses <img>; kit uses a colored block).
	coverBox := func() core.Node {
		d := primitive.NewDecorated()
		d.Width, d.Height = 240, 100
		d.Radius = 8
		if th != nil {
			d.Background = th.Color(core.TokenColorPrimaryBg)
		} else {
			d.Background = render.Hex("#E6F4FF")
		}
		lab := kit.NewText("cover")
		lab.SetFace(face)
		// wrap text inside is complex; plain decorated is enough for demo
		_ = lab
		_ = d.Layout(core.Tight(240, 100))
		return d
	}

	// ── basic.tsx ────────────────────────────────────────────────
	basic := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Cover: coverBox(), Target: t1},
		kit.TourStep{Title: "Save", Description: "Save your changes.", Target: t2},
		kit.TourStep{Title: "Other Actions", Description: "Click to see other actions.", Target: t3},
	))
	basic.SetOnClose(func() { *status = "tour basic closed" })
	basic.SetOnChange(func(cur int) { *status = fmt.Sprintf("tour basic current=%d", cur) })
	basic.SetOnFinish(func() { *status = "tour basic finished" })
	beginBasic := c.trackBtn(kit.NewButton("Begin Tour"))
	beginBasic.SetType(kit.ButtonPrimary)
	beginBasic.SetOnClick(func() {
		// uncontrolled: write Current/Index only — SetCurrent would mark controlled
		basic.Current = 0
		basic.Index = 0
		basic.SetOpen(true)
		*status = "tour basic open"
	})
	// target stand-ins
	btnUp := c.trackBtn(kit.NewButton("Upload"))
	btnSave := c.trackBtn(kit.NewButton("Save"))
	btnSave.SetType(kit.ButtonPrimary)
	btnMore := c.trackBtn(kit.NewButton("···"))
	basicBody := primitive.Column(
		beginBasic.Node(),
		spaceWrap(8, btnUp.Node(), btnSave.Node(), btnMore.Node()),
		basic.Node(),
	)
	basicBody.Gap = 12
	secBasic := demoSection(face, th, "基本",
		"open + steps(title/description/cover/target) · onClose/onChange/onFinish（basic.tsx）。",
		basicBody)

	// ── non-modal.tsx ────────────────────────────────────────────
	nonModal := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Cover: coverBox(), Target: t1},
		kit.TourStep{Title: "Save", Description: "Save your changes.", Target: t2},
		kit.TourStep{Title: "Other Actions", Description: "Click to see other actions.", Target: t3},
	))
	nonModal.SetMask(false)
	nonModal.SetType(kit.TourTypePrimary)
	nonModal.SetOnClose(func() { *status = "tour non-modal closed" })
	beginNM := c.trackBtn(kit.NewButton("Begin non-modal Tour"))
	beginNM.SetType(kit.ButtonPrimary)
	beginNM.SetOnClick(func() {
		nonModal.Current = 0
		nonModal.Index = 0
		nonModal.SetOpen(true)
		*status = "tour non-modal open"
	})
	nmBody := primitive.Column(
		beginNM.Node(),
		spaceWrap(8,
			c.trackBtn(kit.NewButton("Upload")).Node(),
			func() core.Node {
				b := c.trackBtn(kit.NewButton("Save"))
				b.SetType(kit.ButtonPrimary)
				return b.Node()
			}(),
			c.trackBtn(kit.NewButton("···")).Node(),
		),
		nonModal.Node(),
	)
	nmBody.Gap = 12
	secNonModal := demoSection(face, th, "非模态",
		"mask=false · type=primary（non-modal.tsx）。",
		nmBody)

	// ── placement.tsx ────────────────────────────────────────────
	ref := core.NewRect(120, 200, 120, 36)
	sRight := kit.TourStep{Title: "Right", Description: "On the right of target.", Target: ref}
	sRight.SetPlacement(kit.TourRight)
	sTop := kit.TourStep{Title: "Top", Description: "On the top of target.", Target: ref}
	sTop.SetPlacement(kit.TourTop)
	place := wire(kit.NewTour(
		kit.TourStep{Title: "Center", Description: "Displayed in the center of screen."},
		sRight,
		sTop,
	))
	place.SetOnClose(func() { *status = "tour placement closed" })
	beginPlace := c.trackBtn(kit.NewButton("Begin Tour"))
	beginPlace.SetType(kit.ButtonPrimary)
	beginPlace.SetOnClick(func() {
		place.Current = 0
		place.Index = 0
		place.SetOpen(true)
		*status = "tour placement open"
	})
	placeBody := primitive.Column(beginPlace.Node(), place.Node())
	placeBody.Gap = 12
	secPlace := demoSection(face, th, "位置",
		"center / right / top（placement.tsx）。",
		placeBody)

	// ── mask.tsx ─────────────────────────────────────────────────
	maskC := render.RGBA{R: 80.0 / 255, G: 1, B: 1, A: 0.4}
	stepMask := kit.TourStep{Title: "Save", Description: "Save your changes.", Target: t2}
	stepMask.SetMask(true, render.RGBA{R: 40.0 / 255, G: 0, B: 1, A: 0.4})
	stepNoMask := kit.TourStep{Title: "Other Actions", Description: "Click to see other actions.", Target: t3}
	stepNoMask.SetMask(false, render.RGBA{})
	maskTour := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Cover: coverBox(), Target: t1},
		stepMask,
		stepNoMask,
	))
	maskTour.SetMaskColor(maskC)
	maskTour.SetOnClose(func() { *status = "tour mask closed" })
	beginMask := c.trackBtn(kit.NewButton("Begin Tour"))
	beginMask.SetType(kit.ButtonPrimary)
	beginMask.SetOnClick(func() {
		maskTour.Current = 0
		maskTour.Index = 0
		maskTour.SetOpen(true)
		*status = "tour mask open"
	})
	maskBody := primitive.Column(beginMask.Node(), maskTour.Node())
	maskBody.Gap = 12
	secMask := demoSection(face, th, "自定义遮罩样式",
		"mask.color 与 step.mask 覆盖/关闭（mask.tsx）。",
		maskBody)

	// ── indicator.tsx ────────────────────────────────────────────
	ind := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Target: t1},
		kit.TourStep{Title: "Save", Description: "Save your changes.", Target: t2},
		kit.TourStep{Title: "Other Actions", Description: "Click to see other actions.", Target: t3},
	))
	ind.SetIndicatorsRender(func(current, total int) core.Node {
		tx := kit.NewText(fmt.Sprintf("%d / %d", current+1, total))
		tx.SetFace(face)
		return tx.Node()
	})
	ind.SetOnClose(func() { *status = "tour indicator closed" })
	beginInd := c.trackBtn(kit.NewButton("Begin Tour"))
	beginInd.SetType(kit.ButtonPrimary)
	beginInd.SetOnClick(func() {
		ind.Current = 0
		ind.Index = 0
		ind.SetOpen(true)
		*status = "tour indicator open"
	})
	indBody := primitive.Column(beginInd.Node(), ind.Node())
	indBody.Gap = 12
	secInd := demoSection(face, th, "自定义指示器",
		"indicatorsRender(current, total)（indicator.tsx）。",
		indBody)

	// ── actions-render.tsx ───────────────────────────────────────
	act := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Target: t1},
		kit.TourStep{Title: "Save", Description: "Save your changes.", Target: t2},
		kit.TourStep{Title: "Other Actions", Description: "Click to see other actions.", Target: t3},
	))
	act.SetActionsRender(func(origin core.Node, current, total int) core.Node {
		if current == total-1 {
			return origin
		}
		skip := c.trackBtn(kit.NewButton("Skip"))
		skip.SetOnClick(func() {
			act.Close()
			*status = "tour actions Skip"
		})
		return primitive.Row(skip.Node(), origin)
	})
	act.SetOnClose(func() { *status = "tour actions closed" })
	beginAct := c.trackBtn(kit.NewButton("Begin Tour"))
	beginAct.SetType(kit.ButtonPrimary)
	beginAct.SetOnClick(func() {
		act.Current = 0
		act.Index = 0
		act.SetOpen(true)
		*status = "tour actions open"
	})
	actBody := primitive.Column(beginAct.Node(), act.Node())
	actBody.Gap = 12
	secAct := demoSection(face, th, "自定义操作按钮",
		"actionsRender 在 origin 前插入 Skip（actions-render.tsx）。",
		actBody)

	// ── gap.tsx ──────────────────────────────────────────────────
	gapTour := wire(kit.NewTour(
		kit.TourStep{Title: "Upload File", Description: "Put your files here.", Cover: coverBox(), Target: core.NewRect(80, 160, 200, 80)},
	))
	gapTour.SetGapXY(8, 8, 8)
	gapTour.SetOnClose(func() { *status = "tour gap closed" })
	beginGap := c.trackBtn(kit.NewButton("Begin Tour"))
	beginGap.SetType(kit.ButtonPrimary)
	beginGap.SetOnClick(func() {
		gapTour.SetOpen(true)
		*status = "tour gap open offset=8 radius=8"
	})
	// simple gap controls via buttons
	gOff := c.trackBtn(kit.NewButton("gap offset=2"))
	gOff.SetOnClick(func() {
		gapTour.SetGapXY(2, 2, 8)
		*status = "tour gap offset=2"
		if gapTour.IsOpen() {
			gapTour.Sync()
		}
	})
	gRad := c.trackBtn(kit.NewButton("gap radius=16"))
	gRad.SetOnClick(func() {
		gapTour.SetGapXY(8, 8, 16)
		*status = "tour gap radius=16"
		if gapTour.IsOpen() {
			gapTour.Sync()
		}
	})
	gapBody := primitive.Column(
		spaceWrap(8, beginGap.Node(), gOff.Node(), gRad.Node()),
		gapTour.Node(),
	)
	gapBody.Gap = 12
	secGap := demoSection(face, th, "自定义高亮区域的样式",
		"gap.offset / gap.radius（gap.tsx）。",
		gapBody)

	// ── style-class.tsx（浅 styles）──────────────────────────────
	styleTour := wire(kit.NewTour(
		kit.TourStep{
			Title:       "Upload File",
			Description: "Put your files here.",
			Cover:       coverBox(),
			Target:      t1,
			NextButtonProps: &kit.TourButtonProps{
				Style: kit.Style{Border: render.Hex("#CDC1FF"), Text: render.Hex("#CDC1FF")},
			},
		},
		kit.TourStep{
			Title:       "Save",
			Description: "Save your changes.",
			Target:      t2,
			PrevButtonProps: &kit.TourButtonProps{
				Style: kit.Style{Background: render.Hex("#CDC1FF"), Text: render.Hex("#FFFFFF")},
			},
			NextButtonProps: &kit.TourButtonProps{
				Style: kit.Style{Border: render.Hex("#CDC1FF"), Text: render.Hex("#CDC1FF")},
			},
		},
	))
	styleTour.SetStyles(kit.TourStyles{
		Mask:    kit.Style{Background: render.RGBA{R: 0, G: 0, B: 0, A: 0.3}},
		Section: kit.Style{Border: render.Hex("#4096ff"), Radius: 8},
	})
	styleTour.SetOnClose(func() { *status = "tour style closed" })
	beginStyle := c.trackBtn(kit.NewButton("Begin Tour (styles)"))
	beginStyle.SetType(kit.ButtonPrimary)
	beginStyle.SetOnClick(func() {
		styleTour.Current = 0
		styleTour.Index = 0
		styleTour.SetOpen(true)
		*status = "tour style-class open"
	})
	// primary type styles demo
	styleFn := wire(kit.NewTour(
		kit.TourStep{Title: "Primary panel", Description: "type=primary with section styles.", Target: t2},
	))
	styleFn.SetType(kit.TourTypePrimary)
	styleFn.SetStyles(kit.TourStyles{
		Mask:    kit.Style{Background: render.RGBA{R: 0, G: 0, B: 0, A: 0.3}},
		Section: kit.Style{Background: render.RGBA{R: 205.0 / 255, G: 193.0 / 255, B: 1, A: 0.85}},
	})
	beginStyleFn := c.trackBtn(kit.NewButton("Begin Tour (primary styles)"))
	beginStyleFn.SetOnClick(func() {
		styleFn.SetOpen(true)
		*status = "tour style primary open"
	})
	styleBody := primitive.Column(
		spaceWrap(8, beginStyle.Node(), beginStyleFn.Node()),
		styleTour.Node(),
		styleFn.Node(),
	)
	styleBody.Gap = 12
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"浅 styles.mask / styles.section + next/prevButtonProps（style-class.tsx；函数形态 P1）。",
		styleBody)

	page := demoPage(face, "Tour 漫游式引导",
		"用于分步引导用户了解产品功能的气泡组件。P0 对齐 docs/antd/tour.md §6；P1 见 coverage Notes。",
		secBasic, secNonModal, secPlace, secMask, secInd, secAct, secGap, secStyle,
	)
	c.addPage("tour", "Tour", page)
}
