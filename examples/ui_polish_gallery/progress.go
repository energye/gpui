//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerProgress() {
	// Progress — antd demos §6.8 P0:
	// line / circle / line-mini / circle-micro / circle-mini / dynamic / format / dashboard
	// https://ant.design/components/progress · components/progress/demo/*.tsx
	//
	// P1 not shown: steps / linecap / gradient-line / segment / info-position / size full matrix.

	face, th := c.face, c.theme
	wire := func(p *kit.Progress) *kit.Progress {
		if th != nil {
			p.SetTheme(th)
		}
		return p
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}
	row := func(kids ...core.Node) core.Node {
		f := primitive.Row(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossCenter
		f.Wrap = true
		return f
	}

	// ---------- line.tsx ----------
	l30 := wire(kit.NewProgress(30))
	l30.SetWidth(320)
	l50a := wire(kit.NewProgress(50))
	l50a.SetWidth(320)
	l50a.SetStatus(kit.ProgressStatusActive)
	c.trackTicker(l50a)
	l70e := wire(kit.NewProgress(70))
	l70e.SetWidth(320)
	l70e.SetStatus(kit.ProgressStatusException)
	l100 := wire(kit.NewProgress(100))
	l100.SetWidth(320)
	l50h := wire(kit.NewProgress(50))
	l50h.SetWidth(320)
	l50h.SetShowInfo(false)
	secLine := demoSection(face, th, "Progress bar",
		"line.tsx · percent 30 / 50 active / 70 exception / 100 / 50 hideInfo.",
		col(l30.Node(), l50a.Node(), l70e.Node(), l100.Node(), l50h.Node()))

	// ---------- circle.tsx ----------
	c75 := wire(kit.NewProgress(75))
	c75.SetType(kit.ProgressCircle)
	c70e := wire(kit.NewProgress(70))
	c70e.SetType(kit.ProgressCircle)
	c70e.SetStatus(kit.ProgressStatusException)
	c100 := wire(kit.NewProgress(100))
	c100.SetType(kit.ProgressCircle)
	secCircle := demoSection(face, th, "Progress circle",
		"circle.tsx · 75 / 70 exception / 100.",
		row(c75.Node(), c70e.Node(), c100.Node()))

	// ---------- line-mini.tsx ----------
	lm30 := wire(kit.NewProgress(30))
	lm30.SetSize(kit.ProgressSizeSmall)
	lm30.SetWidth(180)
	lm50a := wire(kit.NewProgress(50))
	lm50a.SetSize(kit.ProgressSizeSmall)
	lm50a.SetWidth(180)
	lm50a.SetStatus(kit.ProgressStatusActive)
	c.trackTicker(lm50a)
	lm70e := wire(kit.NewProgress(70))
	lm70e.SetSize(kit.ProgressSizeSmall)
	lm70e.SetWidth(180)
	lm70e.SetStatus(kit.ProgressStatusException)
	lm100 := wire(kit.NewProgress(100))
	lm100.SetSize(kit.ProgressSizeSmall)
	lm100.SetWidth(180)
	secLineMini := demoSection(face, th, "Small progress bar",
		"line-mini.tsx · size=small width≈180.",
		col(lm30.Node(), lm50a.Node(), lm70e.Node(), lm100.Node()))

	// ---------- circle-mini.tsx ----------
	cm30 := wire(kit.NewProgress(30))
	cm30.SetType(kit.ProgressCircle)
	cm30.SetSizePx(80)
	cm70e := wire(kit.NewProgress(70))
	cm70e.SetType(kit.ProgressCircle)
	cm70e.SetSizePx(80)
	cm70e.SetStatus(kit.ProgressStatusException)
	cm100 := wire(kit.NewProgress(100))
	cm100.SetType(kit.ProgressCircle)
	cm100.SetSizePx(80)
	secCircleMini := demoSection(face, th, "Small progress circle",
		"circle-mini.tsx · size=80.",
		row(cm30.Node(), cm70e.Node(), cm100.Node()))

	// ---------- circle-micro.tsx ----------
	micro := wire(kit.NewProgress(60))
	micro.SetType(kit.ProgressCircle)
	micro.SetSizePx(14)
	micro.SetStrokeWidth(20)
	micro.SetRailColor(render.RGBA{R: 0.902, G: 0.957, B: 1, A: 1}) // #e6f4ff-ish
	micro.SetFormat(func(n, _ float64) string {
		return fmt.Sprintf("In progress, %.0f%% complete", n)
	})
	microLab := kit.NewText("Code release")
	microLab.SetFace(face)
	secMicro := demoSection(face, th, "Responsive progress circle",
		"circle-micro.tsx · size=14 strokeWidth=20 + external label.",
		row(micro.Node(), microLab.Node()))

	// ---------- dynamic.tsx ----------
	dynLine := wire(kit.NewProgress(0))
	dynLine.SetWidth(320)
	dynCircle := wire(kit.NewProgress(0))
	dynCircle.SetType(kit.ProgressCircle)
	decBtn := c.trackBtn(kit.NewButton("−"))
	incBtn := c.trackBtn(kit.NewButton("+"))
	decBtn.SetOnClick(func() {
		v := dynLine.Percent - 10
		if v < 0 {
			v = 0
		}
		dynLine.SetPercent(v)
		dynCircle.SetPercent(v)
		*c.status = fmt.Sprintf("progress dynamic %.0f%%", v)
	})
	incBtn.SetOnClick(func() {
		v := dynLine.Percent + 10
		if v > 100 {
			v = 100
		}
		dynLine.SetPercent(v)
		dynCircle.SetPercent(v)
		*c.status = fmt.Sprintf("progress dynamic %.0f%%", v)
	})
	secDynamic := demoSection(face, th, "Dynamic",
		"dynamic.tsx · ±10 line + circle share percent.",
		col(dynLine.Node(), dynCircle.Node(), row(decBtn.Node(), incBtn.Node())))

	// ---------- format.tsx ----------
	fmtDays := wire(kit.NewProgress(75))
	fmtDays.SetType(kit.ProgressCircle)
	fmtDays.SetFormat(func(percent, _ float64) string {
		return fmt.Sprintf("%.0f Days", percent)
	})
	fmtDone := wire(kit.NewProgress(100))
	fmtDone.SetType(kit.ProgressCircle)
	fmtDone.SetFormat(func(_, _ float64) string { return "Done" })
	secFormat := demoSection(face, th, "Custom text format",
		"format.tsx · format override on circle.",
		row(fmtDays.Node(), fmtDone.Node()))

	// ---------- dashboard.tsx ----------
	dash := wire(kit.NewProgress(75))
	dash.SetType(kit.ProgressDashboard)
	dash.SetGapDegree(75)
	dash.SetGapPlacement(kit.ProgressGapBottom)
	dash50 := wire(kit.NewProgress(75))
	dash50.SetType(kit.ProgressDashboard)
	dash50.SetGapDegree(50)
	dash50.SetGapPlacement(kit.ProgressGapBottom)
	secDash := demoSection(face, th, "Dashboard",
		"dashboard.tsx · gapDegree 75 (default) / 50 · gapPlacement=bottom.",
		row(dash.Node(), dash50.Node()))

	// Lifecycle (#9)
	life := wire(kit.NewProgress(40))
	life.SetType(kit.ProgressLine)
	life.SetWidth(220)
	life.SetShowInfo(true)
	life.SetPercent(70)
	life.SetStatus(kit.ProgressStatusActive)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Type/Width) then chrome (Percent/Status)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeProgress, func(pc *core.PaintContext, n core.Node) {
		// Default walk; Override proves hook runs (visual same).
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinP := wire(kit.NewProgress(60))
	skinP.SetWidth(220)
	skinP.SetTheme(th)
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"progressHost TypeID=kit.Progress；Theme.Skin Override 可命中。",
		skinP.Node())

	c.addPage("progress", "Progress",
		demoPage(face, "Progress 进度条",
			"Ant Design Progress · P0 + #9 lifecycle + #6 Skin.",
			secLine, secCircle, secLineMini, secCircleMini, secMicro, secDynamic, secFormat, secDash, secLife, secSkin))
}
