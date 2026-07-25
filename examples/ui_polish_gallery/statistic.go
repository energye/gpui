//go:build linux && !nogpu

package main

import (
	"fmt"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerStatistic() {
	// Statistic — docs/antd/statistic.md §6.8 P0
	// https://ant.design/components/statistic
	// demos: basic / unit / animated / card / timer / style-class
	//
	// P1 not shown: CountUp pixel animation, _semantic.tsx, styles function depth,
	// ConfigProvider 全局, 官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(s *kit.Statistic) *kit.Statistic {
		s.SetFace(face)
		if th != nil {
			s.SetTheme(th)
		}
		c.trackTicker(s)
		return s
	}

	// ---------- basic.tsx ----------
	basicA := wire(kit.NewStatistic())
	basicA.SetTitle("Active Users")
	basicA.SetValue(112893)

	basicB := wire(kit.NewStatistic())
	basicB.SetTitle("Account Balance (CNY)")
	basicB.SetValue(112893)
	basicB.SetPrecision(2)
	recharge := c.trackBtn(kit.NewButton("Recharge"))
	recharge.SetType(kit.ButtonPrimary)
	recharge.SetOnClick(func() {
		if status != nil {
			*status = "Statistic · Recharge"
		}
	})
	basicBCol := primitive.Column(basicB.Node(), recharge.Node())
	basicBCol.Gap = 16
	basicBCol.CrossAlign = core.CrossStart

	basicC := wire(kit.NewStatistic())
	basicC.SetTitle("Active Users")
	basicC.SetValue(112893)
	basicC.SetLoading(true)

	basicRow := primitive.Row(basicA.Node(), basicBCol, basicC.Node())
	basicRow.Gap = 24
	basicRow.CrossAlign = core.CrossStart
	secBasic := demoSection(face, th, "基本",
		"basic.tsx：title + value + precision + loading Skeleton。",
		basicRow)

	// ---------- unit.tsx ----------
	unitA := wire(kit.NewStatistic())
	unitA.SetTitle("Feedback")
	unitA.SetValue(1128)
	heart := kit.NewIcon("heart")
	heart.SetSize(20)
	if th != nil {
		heart.SetTheme(th)
	}
	unitA.SetPrefixNode(heart.Node())

	unitB := wire(kit.NewStatistic())
	unitB.SetTitle("Unmerged")
	unitB.SetValue(93)
	unitB.SetSuffix("/ 100")

	unitRow := primitive.Row(unitA.Node(), unitB.Node())
	unitRow.Gap = 48
	unitRow.CrossAlign = core.CrossStart
	secUnit := demoSection(face, th, "单位",
		"unit.tsx：prefix 图标 + suffix 文案。",
		unitRow)

	// ---------- animated.tsx ----------
	// P0: formatter 终值（CountUp 像素动画 P1）
	animFmt := func(v any) string {
		n := kit.NewStatistic()
		n.SetValue(v)
		return n.DisplayText()
	}
	animA := wire(kit.NewStatistic())
	animA.SetTitle("Active Users")
	animA.SetValue(112893)
	animA.SetFormatter(animFmt)

	animB := wire(kit.NewStatistic())
	animB.SetTitle("Account Balance (CNY)")
	animB.SetValue(112893)
	animB.SetPrecision(2)
	animB.SetFormatter(func(v any) string {
		n := kit.NewStatistic()
		n.SetValue(v)
		n.SetPrecision(2)
		return n.DisplayText()
	})

	animRow := primitive.Row(animA.Node(), animB.Node())
	animRow.Gap = 48
	animRow.CrossAlign = core.CrossStart
	secAnim := demoSection(face, th, "动画效果",
		"animated.tsx：formatter 路径（CountUp 像素级 P1，P0 瞬时终值）。",
		animRow)

	// ---------- card.tsx ----------
	cardUp := wire(kit.NewStatistic())
	cardUp.SetTitle("Active")
	cardUp.SetValue(11.28)
	cardUp.SetPrecision(2)
	cardUp.SetContentStyle(kit.Style{Text: render.Hex("#3f8600")})
	cardUp.SetPrefix("↑")
	cardUp.SetSuffix("%")
	cardA := kit.NewCard("")
	cardA.SetVariant(kit.CardBorderless)
	cardA.SetFace(face)
	if th != nil {
		cardA.SetTheme(th)
	}
	cardA.SetContent(cardUp.Node())

	cardDown := wire(kit.NewStatistic())
	cardDown.SetTitle("Idle")
	cardDown.SetValue(9.3)
	cardDown.SetPrecision(2)
	cardDown.SetContentStyle(kit.Style{Text: render.Hex("#cf1322")})
	cardDown.SetPrefix("↓")
	cardDown.SetSuffix("%")
	cardB := kit.NewCard("")
	cardB.SetVariant(kit.CardBorderless)
	cardB.SetFace(face)
	if th != nil {
		cardB.SetTheme(th)
	}
	cardB.SetContent(cardDown.Node())

	cardRow := primitive.Row(cardA.Node(), cardB.Node())
	cardRow.Gap = 16
	cardRow.CrossAlign = core.CrossStart
	secCard := demoSection(face, th, "在卡片中使用",
		"card.tsx：Card borderless + content 色 + prefix/suffix。",
		cardRow)

	// ---------- timer.tsx ----------
	now := time.Now().UnixMilli()
	deadline := now + 1000*60*60*24*2 + 1000*30
	before := now - 1000*60*60*24*2 + 1000*30
	tenLater := now + 10*1000

	t1 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountdown))
	t1.SetValue(deadline)
	t1.SetOnFinish(func() {
		if status != nil {
			*status = "Statistic · countdown finished"
		}
	})

	t2 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountdown))
	t2.SetTitle("Million Seconds")
	t2.SetValue(deadline)
	t2.SetFormat("HH:mm:ss:SSS")

	t3 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountdown))
	t3.SetTitle("Countdown")
	t3.SetValue(tenLater)
	t3.SetOnChange(func(diff float64) {
		if status != nil && diff > 0 && diff < 5000 {
			*status = fmt.Sprintf("Statistic · onChange %.0fms", diff)
		}
	})
	t3.SetOnFinish(func() {
		if status != nil {
			*status = "Statistic · 10s countdown finished"
		}
	})

	t4 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountup))
	t4.SetTitle("Countup")
	t4.SetValue(before)

	timerRow1 := primitive.Row(t1.Node(), t2.Node(), t3.Node(), t4.Node())
	timerRow1.Gap = 24
	timerRow1.CrossAlign = core.CrossStart

	t5 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountdown))
	t5.SetTitle("Day Level (Countdown)")
	t5.SetValue(deadline)
	t5.SetFormat("D 天 H 时 m 分 s 秒")

	t6 := wire(kit.NewStatisticTimer(kit.StatisticTimerCountup))
	t6.SetTitle("Day Level (Countup)")
	t6.SetValue(before)
	t6.SetFormat("D 天 H 时 m 分 s 秒")

	timerCol := primitive.Column(timerRow1, t5.Node(), t6.Node())
	timerCol.Gap = 24
	timerCol.CrossAlign = core.CrossStart
	secTimer := demoSection(face, th, "计时器",
		"timer.tsx：Statistic.Timer countdown/countup + format + onChange/onFinish（Ticker）。",
		timerCol)

	// ---------- style-class.tsx ----------
	styleObj := wire(kit.NewStatistic())
	styleObj.SetTitle("Monthly Active Users")
	styleObj.SetValue(93241)
	styleObj.SetSuffix("users")
	styleObj.SetPrefix("↑")
	styleObj.SetClassNames(kit.StatisticClassNames{Root: "stat-style-demo"})
	styleObj.SetStyle(kit.Style{
		Border:      render.Hex("#CCCCCC"),
		Radius:      8,
		ForceRadius: true,
	})
	styleObj.SetTitleStyle(kit.Style{Text: render.Hex("#1890ff")})
	styleObj.SetContentStyle(kit.Style{FontSize: 24})
	styleObj.SetValueStyle(kit.Style{
		Background: render.Hex("#e6f4ff"),
		Text:       render.Hex("#0958d9"),
		Radius:     4,
	})

	styleNeg := wire(kit.NewStatistic())
	styleNeg.SetTitle("Yearly Loss")
	styleNeg.SetValue(-18.7)
	styleNeg.SetPrecision(1)
	styleNeg.SetSuffix("%")
	styleNeg.SetPrefix("↑")
	styleNeg.SetStyle(kit.Style{
		Border:      render.Hex("#CCCCCC"),
		Radius:      8,
		ForceRadius: true,
	})
	styleNeg.SetTitleStyle(kit.Style{Text: render.Hex("#ff4d4f")})
	styleNeg.SetContentStyle(kit.Style{Text: render.Hex("#ff7875")})
	styleNeg.SetValueStyle(kit.Style{
		Background: render.Hex("#fff1f0"),
		Radius:     4,
	})

	styleCol := primitive.Column(styleObj.Node(), styleNeg.Node())
	styleCol.Gap = 16
	styleCol.CrossAlign = core.CrossStretch
	secStyle := demoSection(face, th, "自定义语义结构的样式和类",
		"style-class.tsx：浅 styles.root/title/content/value + classNames.root（函数形态深度 P1）。",
		styleCol)

	page := demoPage(face, "Statistic 统计数值",
		"展示统计数值。P0：value/title/prefix/suffix/precision/分隔符/formatter/loading、Timer countdown|countup、浅 styles·classNames。\n"+
			"P1：CountUp 像素动画、_semantic、styles 函数深度、ConfigProvider 全局、官网逐像素。",
		secBasic, secUnit, secAnim, secCard, secTimer, secStyle)
	c.addPage("statistic", "Statistic", page)
}
