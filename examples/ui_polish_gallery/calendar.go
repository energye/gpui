//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerCalendar() {
	// Calendar — docs/antd/calendar.md §6.8 P0
	// https://ant.design/components/calendar
	// demos: basic / notice-calendar / event-range / card / select / lunar / week / customize-header
	//
	// P1 not shown: style-class / _semantic, full locale table, built-in lunar lib,
	// ConfigProvider global, debug / pixel-hash.

	face, th := c.face, c.theme
	status := c.status

	track := func(cal *kit.Calendar) *kit.Calendar {
		if cal == nil {
			return nil
		}
		cal.SetFace(face)
		if th != nil {
			cal.SetTheme(th)
		}
		c.trackTicker(cal)
		return cal
	}

	// ---------- basic.tsx ----------
	basic := track(kit.NewCalendar())
	basic.SetOnPanelChange(func(v kit.DateValue, m kit.CalendarMode) {
		if status != nil {
			*status = fmt.Sprintf("Calendar basic panel → %04d-%02d-%02d %s", v.Year, v.Month, v.Day, m)
		}
	})
	secBasic := demoSection(face, th, "基本",
		"默认全屏月面板；切换年月或 mode 触发 onPanelChange。",
		basic.Node())

	// ---------- notice-calendar.tsx ----------
	notice := track(kit.NewCalendar())
	notice.SetDefaultValue(kit.DateOf(2026, 7, 1))
	notice.SetCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if info.Type != kit.CalendarCellDate || !cur.Valid {
			return nil
		}
		var items []string
		switch cur.Day {
		case 8:
			items = []string{"warning", "success"}
		case 10:
			items = []string{"warning", "success", "error"}
		case 15:
			items = []string{"warning", "success", "error"}
		default:
			return nil
		}
		col := primitive.Column()
		col.Gap = 2
		col.CrossAlign = core.CrossStart
		for _, st := range items {
			b := kit.NewBadge()
			b.SetFace(face)
			if th != nil {
				b.SetTheme(th)
			}
			switch st {
			case "success":
				b.SetStatus(kit.BadgeStatusSuccess)
			case "error":
				b.SetStatus(kit.BadgeStatusError)
			case "warning":
				b.SetStatus(kit.BadgeStatusWarning)
			default:
				b.SetStatus(kit.BadgeStatusDefault)
			}
			b.SetText(st + " event")
			col.AddChild(b.Node())
		}
		return col
	})
	secNotice := demoSection(face, th, "通知事项日历",
		"cellRender 在日期格内挂 Badge 事项列表（8/10/15 日）。",
		notice.Node())

	// ---------- event-range.tsx ----------
	type calEvent struct {
		key, title string
		start, end kit.DateValue
		color      render.RGBA
	}
	evColor := func(hex string) render.RGBA {
		if th != nil {
			switch hex {
			case "primary":
				return th.Color(core.TokenColorPrimary)
			case "success":
				return th.Color(core.TokenColorSuccess)
			case "warning":
				return th.Color(core.TokenColorWarning)
			case "error":
				return th.Color(core.TokenColorError)
			}
		}
		return render.Hex(hex)
	}
	events := []calEvent{
		{"release", "Release window", kit.DateOf(2026, 1, 8), kit.DateOf(2026, 1, 10), evColor("primary")},
		{"design", "Design review", kit.DateOf(2026, 1, 14), kit.DateOf(2026, 1, 14), evColor("success")},
		{"maint", "Maintenance", kit.DateOf(2026, 1, 21), kit.DateOf(2026, 1, 24), evColor("warning")},
		{"bug", "Bug fix", kit.DateOf(2026, 1, 30), kit.DateOf(2026, 1, 31), evColor("error")},
	}
	inRange := func(cur, a, b kit.DateValue) bool {
		if !cur.Valid || !a.Valid || !b.Valid {
			return false
		}
		ck := cur.Year*400 + cur.Month*32 + cur.Day
		return ck >= a.Year*400+a.Month*32+a.Day && ck <= b.Year*400+b.Month*32+b.Day
	}
	rangePos := func(cur, a, b kit.DateValue) string {
		if cur.EqualDate(a) && cur.EqualDate(b) {
			return "single"
		}
		if cur.EqualDate(a) {
			return "start"
		}
		if cur.EqualDate(b) {
			return "end"
		}
		return "middle"
	}
	evCal := track(kit.NewCalendar())
	evCal.SetDefaultValue(kit.DateOf(2026, 1, 1))
	evCal.SetCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if info.Type != kit.CalendarCellDate {
			return nil
		}
		col := primitive.Column()
		col.Gap = 2
		col.CrossAlign = core.CrossStretch
		for _, e := range events {
			if !inRange(cur, e.start, e.end) {
				continue
			}
			pos := rangePos(cur, e.start, e.end)
			lab := ""
			if pos == "start" || pos == "single" {
				lab = e.title
			}
			txt := kit.NewText(lab)
			txt.SetFace(face)
			txt.SetFontSize(12)
			txt.SetStyle(kit.Style{Text: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
			bar := primitive.NewDecorated(txt.Node())
			bar.Background = e.color
			bar.Height = 18
			bar.Radius = 999
			bar.BorderWidth = 0
			bar.Padding = primitive.EdgeInsets{Left: 6, Right: 6}
			bar.StretchChild = true
			col.AddChild(bar)
		}
		if len(col.Children()) == 0 {
			return nil
		}
		return col
	})
	secEvent := demoSection(face, th, "跨日期事件",
		"cellRender 画跨日色条（2026-01 示例数据）。",
		evCal.Node())

	// ---------- card.tsx ----------
	cardCal := track(kit.NewCalendar())
	cardCal.SetFullscreen(false)
	cardCal.SetOnPanelChange(func(v kit.DateValue, m kit.CalendarMode) {
		if status != nil {
			*status = fmt.Sprintf("Calendar card panel → %04d-%02d %s", v.Year, v.Month, m)
		}
	})
	cardWrap := primitive.NewDecorated(cardCal.Node())
	cardWrap.Width = 300
	cardWrap.BorderWidth = 1
	if th != nil {
		cardWrap.BorderColor = th.Color(core.TokenColorBorderSecondary)
		if cardWrap.BorderColor.A == 0 {
			cardWrap.BorderColor = th.Color(core.TokenColorBorder)
		}
		cardWrap.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		cardWrap.Background = th.Color(core.TokenColorBgContainer)
	} else {
		cardWrap.BorderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
		cardWrap.Radius = 8
	}
	secCard := demoSection(face, th, "卡片模式",
		"fullscreen=false；外层 300px 卡片边框。",
		cardWrap)

	// ---------- select.tsx ----------
	selCal := track(kit.NewCalendar())
	selCal.SetControlled(true)
	selCal.SetValue(kit.DateOf(2017, 1, 25))
	selAlert := kit.NewAlert(fmt.Sprintf("You selected date: %04d-%02d-%02d", 2017, 1, 25))
	selAlert.SetFace(face)
	selCal.SetOnSelect(func(v kit.DateValue, _ kit.CalendarSelectSource) {
		selCal.SetValue(v)
		selAlert.SetTitle(fmt.Sprintf("You selected date: %04d-%02d-%02d", v.Year, v.Month, v.Day))
		if status != nil {
			*status = fmt.Sprintf("Calendar select → %04d-%02d-%02d", v.Year, v.Month, v.Day)
		}
	})
	selCal.SetOnPanelChange(func(v kit.DateValue, _ kit.CalendarMode) {
		selCal.SetValue(v)
	})
	selCol := primitive.Column(selAlert.Node(), selCal.Node())
	selCol.Gap = 8
	selCol.CrossAlign = core.CrossStretch
	secSelect := demoSection(face, th, "选择功能",
		"受控 value + onSelect / onPanelChange；顶部 Alert 显示选中日。",
		selCol)

	// ---------- lunar.tsx (fullCellRender 钩子，无内置农历库) ----------
	lunar := track(kit.NewCalendar())
	lunar.SetFullscreen(false)
	lunar.SetFullCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if !cur.Valid {
			return nil
		}
		if info.Type == kit.CalendarCellMonth {
			lab := kit.NewText(fmt.Sprintf("%d月", cur.Month))
			lab.SetFace(face)
			return lab.Node()
		}
		if info.Type != kit.CalendarCellDate {
			return nil
		}
		// synthetic lunar secondary line (gallery stand-in for lunar-typescript)
		main := kit.NewText(fmt.Sprintf("%d", cur.Day))
		main.SetFace(face)
		sub := kit.NewText(fmt.Sprintf("农%02d", cur.Day))
		sub.SetFace(face)
		sub.SetFontSize(11)
		sub.SetStyle(kit.Style{Text: render.RGBA{R: 0, G: 0, B: 0, A: 0.45}})
		col := primitive.Column(main.Node(), sub.Node())
		col.Gap = 2
		col.CrossAlign = core.CrossCenter
		return col
	})
	lunarWrap := primitive.NewDecorated(lunar.Node())
	lunarWrap.Width = 450
	lunarWrap.BorderWidth = 1
	lunarWrap.Padding = primitive.All(5)
	if th != nil {
		lunarWrap.BorderColor = th.Color(core.TokenColorBorder)
		lunarWrap.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	}
	secLunar := demoSection(face, th, "农历日历",
		"fullCellRender 注入副文案（示意农历；算法由业务库提供）。",
		lunarWrap)

	// ---------- week.tsx ----------
	weekFull := track(kit.NewCalendar())
	weekFull.SetShowWeek(true)
	weekMini := track(kit.NewCalendar())
	weekMini.SetFullscreen(false)
	weekMini.SetShowWeek(true)
	weekMiniWrap := primitive.NewDecorated(weekMini.Node())
	weekMiniWrap.Width = 300
	weekMiniWrap.BorderWidth = 1
	if th != nil {
		weekMiniWrap.BorderColor = th.Color(core.TokenColorBorder)
		weekMiniWrap.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	}
	weekCol := primitive.Column(weekFull.Node(), weekMiniWrap)
	weekCol.Gap = 16
	weekCol.CrossAlign = core.CrossStart
	secWeek := demoSection(face, th, "周数",
		"showWeek 全屏与卡片两种。",
		weekCol)

	// ---------- customize-header.tsx ----------
	custom := track(kit.NewCalendar())
	custom.SetFullscreen(false)
	custom.SetHeaderRender(func(cfg kit.CalendarHeaderConfig) core.Node {
		title := kit.NewText("Custom header")
		title.SetFace(face)
		title.SetFontSize(16)

		mode := kit.NewRadioGroup()
		mode.SetOptionType(kit.RadioOptionButton)
		mode.SetSize(kit.RadioSmall)
		mode.SetFace(face)
		if th != nil {
			mode.SetTheme(th)
		}
		mode.SetOptions(
			kit.RadioOption{Label: "Month", Value: "month"},
			kit.RadioOption{Label: "Year", Value: "year"},
		)
		if cfg.Type == kit.CalendarYear {
			mode.SetValue("year")
		} else {
			mode.SetValue("month")
		}
		mode.SetOnChange(func(v string) {
			if v == "year" {
				cfg.OnTypeChange(kit.CalendarYear)
			} else {
				cfg.OnTypeChange(kit.CalendarMonth)
			}
		})

		prev := kit.NewButton("<")
		prev.SetType(kit.ButtonText)
		prev.SetSize(kit.ButtonSmall)
		prev.SetFace(face)
		prev.SetOnClick(func() {
			m := cfg.Value.Month - 1
			y := cfg.Value.Year
			if m < 1 {
				m = 12
				y--
			}
			cfg.OnChange(kit.DateOf(y, m, 1))
		})
		next := kit.NewButton(">")
		next.SetType(kit.ButtonText)
		next.SetSize(kit.ButtonSmall)
		next.SetFace(face)
		next.SetOnClick(func() {
			m := cfg.Value.Month + 1
			y := cfg.Value.Year
			if m > 12 {
				m = 1
				y++
			}
			cfg.OnChange(kit.DateOf(y, m, 1))
		})
		lab := kit.NewText(fmt.Sprintf("%04d-%02d", cfg.Value.Year, cfg.Value.Month))
		lab.SetFace(face)

		row := primitive.Row(mode.Node(), prev.Node(), lab.Node(), next.Node())
		row.Gap = 8
		row.CrossAlign = core.CrossCenter
		col := primitive.Column(title.Node(), row)
		col.Gap = 8
		col.Padding = primitive.All(8)
		col.CrossAlign = core.CrossStart
		return col
	})
	custom.SetOnPanelChange(func(v kit.DateValue, m kit.CalendarMode) {
		if status != nil {
			*status = fmt.Sprintf("Calendar custom header → %04d-%02d %s", v.Year, v.Month, m)
		}
	})
	customWrap := primitive.NewDecorated(custom.Node())
	customWrap.Width = 300
	customWrap.BorderWidth = 1
	if th != nil {
		customWrap.BorderColor = th.Color(core.TokenColorBorder)
		customWrap.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	}
	secCustom := demoSection(face, th, "自定义头部",
		"headerRender 替换默认年/月 Select；保留 mode 切换与切月。",
		customWrap)

	// Lifecycle (#9)
	life := track(kit.NewCalendar())
	life.SetFullscreen(false)
	life.SetShowWeek(true)
	life.SetMode(kit.CalendarMonth)
	lifeWrap := primitive.NewDecorated(life.Node())
	lifeWrap.Width = 300
	lifeWrap.BorderWidth = 1
	if th != nil {
		lifeWrap.BorderColor = th.Color(core.TokenColorBorder)
	}
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Fullscreen → ShowWeek → Mode；ensureBuilt 懒构建。",
		lifeWrap)

	// Skin (#6) — Root 是 Flex（无 SkinType 标记）；TypeID=kit.Calendar 已注册，
	// Override 证明挂接点存在（默认委托，不改变视觉）。
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeCalendar, func(pc *core.PaintContext, n core.Node) {
		if p := baseSkin.Painter(kit.TypeCalendar); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinCal := track(kit.NewCalendar())
	skinCal.SetFullscreen(false)
	skinWrap := primitive.NewDecorated(skinCal.Node())
	skinWrap.Width = 300
	skinWrap.BorderWidth = 2
	skinWrap.BorderColor = render.Hex("#1677FF")
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"TypeID=kit.Calendar 已注册；Theme.Skin Override 可挂接（Root Flex 不带 SkinType，边框由 demo 包裹示意）。",
		skinWrap)

	page := demoPage(face, "Calendar 日历",
		"按照日历形式展示数据的容器。P0 对齐 docs/antd/calendar.md §6（value/defaultValue/onChange/onSelect/onPanelChange、mode、fullscreen、showWeek、disabledDate、cellRender/fullCellRender、headerRender、loading Ticker）。Also #9 lifecycle + #6 Skin.",
		secBasic, secNotice, secEvent, secCard, secSelect, secLunar, secWeek, secCustom, secLife, secSkin)
	c.addPage("calendar", "Calendar", page)
}
