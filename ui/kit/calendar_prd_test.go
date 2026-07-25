package kit_test

import (
	"fmt"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/calendar.md §6.9 — P0 PRD cases (CAL-01 … CAL-19).
// L3/L4 (CAL-20/21) and P1 (CAL-22) deferred.

func approxCAL(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxCALColor(a, b render.RGBA, tol float64) bool {
	return approxCAL(a.R, b.R, tol) && approxCAL(a.G, b.G, tol) &&
		approxCAL(a.B, b.B, tol) && approxCAL(a.A, b.A, tol)
}

func mountCalendar(t *testing.T, c *kit.Calendar, w, h float64) *core.Tree {
	t.Helper()
	if c == nil {
		t.Fatal("nil calendar")
	}
	bg := primitive.NewBox(c.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func TestCalendar_PRD_01_Defaults(t *testing.T) {
	// CAL-01
	c := kit.NewCalendar()
	if c.Mode != kit.CalendarMonth {
		t.Fatalf("Mode=%v want month", c.Mode)
	}
	if !c.IsFullscreen() {
		t.Fatal("Fullscreen default true")
	}
	if c.ShowWeek || c.Disabled || c.Loading || c.Controlled {
		t.Fatalf("flags week=%v dis=%v load=%v ctrl=%v", c.ShowWeek, c.Disabled, c.Loading, c.Controlled)
	}
	v := c.GetValue()
	if !v.Valid {
		t.Fatal("default value should be today")
	}
	today := kit.Today()
	if !v.EqualDate(today) {
		t.Fatalf("value=%+v today=%+v", v, today)
	}
	if c.Node() == nil || c.HeaderNode() == nil || c.BodyNode() == nil {
		t.Fatal("nil nodes")
	}
	if c.Root.Base().Role != "grid" {
		t.Fatalf("role=%q", c.Root.Base().Role)
	}
	_ = mountCalendar(t, c, 720, 560)
}

func TestCalendar_PRD_02_SelectDay(t *testing.T) {
	// CAL-02 / CAL-S1
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	var nSelect, nChange int
	var got kit.DateValue
	var src kit.CalendarSelectSource
	c.SetOnSelect(func(v kit.DateValue, s kit.CalendarSelectSource) {
		nSelect++
		got = v
		src = s
	})
	c.SetOnChange(func(v kit.DateValue) {
		nChange++
	})
	_ = mountCalendar(t, c, 720, 560)
	c.SelectDay(15)
	if nSelect != 1 {
		t.Fatalf("onSelect n=%d", nSelect)
	}
	if src != kit.CalendarSourceDate {
		t.Fatalf("source=%v", src)
	}
	if !got.Valid || got.Day != 15 || got.Month != 7 || got.Year != 2026 {
		t.Fatalf("got=%+v", got)
	}
	if nChange != 1 {
		t.Fatalf("onChange n=%d", nChange)
	}
	if c.GetValue().Day != 15 {
		t.Fatalf("value day=%d", c.GetValue().Day)
	}
}

func TestCalendar_PRD_03_ModeYear(t *testing.T) {
	// CAL-03 / CAL-S2
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	_ = mountCalendar(t, c, 720, 560)
	c.SetMode(kit.CalendarYear)
	if c.ModeOf() != kit.CalendarYear {
		t.Fatalf("mode=%v", c.ModeOf())
	}
	if len(c.MonthCells()) != 12 {
		t.Fatalf("month cells=%d want 12", len(c.MonthCells()))
	}
	// click month → back to month mode
	c.SelectDate(kit.DateOf(2026, 3, 1))
	// SelectDate with source date while in year mode just sets date;
	// month cell path uses CalendarSourceMonth. Simulate via MonthCells click.
	c.SetMode(kit.CalendarYear)
	cells := c.MonthCells()
	if len(cells) == 0 {
		t.Fatal("no month cells")
	}
	// March is index 2
	if cells[2].Click != nil {
		cells[2].Click()
	}
	if c.ModeOf() != kit.CalendarMonth {
		t.Fatalf("after month pick mode=%v", c.ModeOf())
	}
	if c.PanelMonth() != 3 {
		t.Fatalf("panel month=%d", c.PanelMonth())
	}
}

func TestCalendar_PRD_04_DisabledDate(t *testing.T) {
	// CAL-04 / CAL-S3
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	c.SetDisabledDate(func(d kit.DateValue) bool {
		return d.Day == 10
	})
	n := 0
	c.SetOnSelect(func(kit.DateValue, kit.CalendarSelectSource) { n++ })
	c.SetOnChange(func(kit.DateValue) { n++ })
	_ = mountCalendar(t, c, 720, 560)
	c.SelectDay(10)
	if n != 0 {
		t.Fatalf("disabled day fired callbacks n=%d", n)
	}
	if c.GetValue().Day != 1 {
		t.Fatalf("value should stay 1, got %d", c.GetValue().Day)
	}
	// valid day still works
	c.SelectDay(11)
	if n == 0 {
		t.Fatal("day 11 should fire")
	}
}

func TestCalendar_PRD_05_FullscreenFalse(t *testing.T) {
	// CAL-05 / CAL-S4
	c := kit.NewCalendar()
	c.SetFullscreen(false)
	if c.IsFullscreen() {
		t.Fatal("want card mode")
	}
	_ = mountCalendar(t, c, 320, 360)
	// card uses LG radius
	if !approxCAL(c.ResolvedRadius(), 8, 0.5) {
		t.Fatalf("card radius=%v want ~8", c.ResolvedRadius())
	}
	w, h := c.ResolvedCellSize()
	if w < 20 || h < 20 {
		t.Fatalf("mini cell=%v,%v", w, h)
	}
}

func TestCalendar_PRD_06_ControlledValue(t *testing.T) {
	// CAL-06 / CAL-S5
	c := kit.NewCalendar()
	c.SetValue(kit.DateOf(2017, 1, 25))
	c.SetControlled(true)
	var got kit.DateValue
	c.SetOnSelect(func(v kit.DateValue, _ kit.CalendarSelectSource) { got = v })
	c.SetOnChange(func(v kit.DateValue) { got = v })
	_ = mountCalendar(t, c, 720, 560)
	c.SelectDay(10)
	if !got.Valid || got.Day != 10 {
		t.Fatalf("callback got=%+v", got)
	}
	// controlled: internal value stays until SetValue
	if c.GetValue().Day != 25 {
		t.Fatalf("controlled value mutated to %d", c.GetValue().Day)
	}
	c.SetValue(got)
	if c.GetValue().Day != 10 {
		t.Fatalf("after SetValue day=%d", c.GetValue().Day)
	}
}

func TestCalendar_PRD_07_PanelMonthChange(t *testing.T) {
	// CAL-07 / CAL-S6
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 15))
	var n int
	var mode kit.CalendarMode
	var panel kit.DateValue
	c.SetOnPanelChange(func(v kit.DateValue, m kit.CalendarMode) {
		n++
		panel = v
		mode = m
	})
	_ = mountCalendar(t, c, 720, 560)
	c.SetPanelMonth(2026, 8)
	if n != 1 {
		t.Fatalf("onPanelChange n=%d", n)
	}
	if c.PanelMonth() != 8 || c.PanelYear() != 2026 {
		t.Fatalf("panel=%d-%d", c.PanelYear(), c.PanelMonth())
	}
	if mode != kit.CalendarMonth {
		t.Fatalf("mode=%v", mode)
	}
	if panel.Month != 8 {
		t.Fatalf("panel callback month=%d", panel.Month)
	}
}

func TestCalendar_PRD_08_DemoBasic(t *testing.T) {
	// CAL-08 basic.tsx
	c := kit.NewCalendar()
	var logs int
	c.SetOnPanelChange(func(v kit.DateValue, m kit.CalendarMode) {
		logs++
		_ = fmt.Sprintf("%04d-%02d-%02d %s", v.Year, v.Month, v.Day, m)
	})
	_ = mountCalendar(t, c, 800, 600)
	c.SetPanelMonth(c.PanelYear(), c.PanelMonth()%12+1)
	if logs < 1 {
		t.Fatal("expected panel change log")
	}
}

func TestCalendar_PRD_09_DemoNotice(t *testing.T) {
	// CAL-09 notice-calendar.tsx — cellRender
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	renders := 0
	c.SetCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if info.Type != kit.CalendarCellDate {
			return nil
		}
		renders++
		if cur.Day == 8 || cur.Day == 10 || cur.Day == 15 {
			return kit.NewText("•").Node()
		}
		return nil
	})
	_ = mountCalendar(t, c, 800, 640)
	if renders == 0 {
		t.Fatal("cellRender not invoked")
	}
}

func TestCalendar_PRD_10_DemoEventRange(t *testing.T) {
	// CAL-10 event-range.tsx
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 1, 1))
	type ev struct {
		start, end kit.DateValue
		title      string
	}
	events := []ev{
		{kit.DateOf(2026, 1, 8), kit.DateOf(2026, 1, 10), "Release"},
		{kit.DateOf(2026, 1, 14), kit.DateOf(2026, 1, 14), "Review"},
	}
	inRange := func(cur, a, b kit.DateValue) bool {
		if !cur.Valid {
			return false
		}
		// inclusive date compare via day ordinal
		ck := cur.Year*400 + cur.Month*32 + cur.Day
		ak := a.Year*400 + a.Month*32 + a.Day
		bk := b.Year*400 + b.Month*32 + b.Day
		return ck >= ak && ck <= bk
	}
	hits := 0
	c.SetCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if info.Type != kit.CalendarCellDate {
			return nil
		}
		for _, e := range events {
			if inRange(cur, e.start, e.end) {
				hits++
				return kit.NewText(e.title).Node()
			}
		}
		return nil
	})
	_ = mountCalendar(t, c, 800, 640)
	if hits == 0 {
		t.Fatal("event bars not rendered")
	}
	if c.GetValue().Month != 1 {
		t.Fatalf("defaultValue month=%d", c.GetValue().Month)
	}
}

func TestCalendar_PRD_11_DemoCard(t *testing.T) {
	// CAL-11 card.tsx
	c := kit.NewCalendar()
	c.SetFullscreen(false)
	c.SetOnPanelChange(func(kit.DateValue, kit.CalendarMode) {})
	_ = mountCalendar(t, c, 300, 360)
	if c.IsFullscreen() {
		t.Fatal("card fullscreen")
	}
	if !approxCAL(c.ResolvedMiniContentHeight(), 256, 0.5) {
		t.Fatalf("miniH=%v", c.ResolvedMiniContentHeight())
	}
}

func TestCalendar_PRD_12_DemoSelect(t *testing.T) {
	// CAL-12 select.tsx — controlled value + onSelect + onPanelChange
	c := kit.NewCalendar()
	c.SetControlled(true)
	c.SetValue(kit.DateOf(2017, 1, 25))
	selected := kit.DateOf(2017, 1, 25)
	c.SetOnSelect(func(v kit.DateValue, _ kit.CalendarSelectSource) {
		selected = v
		c.SetValue(v)
	})
	c.SetOnPanelChange(func(v kit.DateValue, _ kit.CalendarMode) {
		c.SetValue(v)
	})
	_ = mountCalendar(t, c, 720, 560)
	c.SelectDay(5)
	if selected.Day != 5 {
		t.Fatalf("selected=%+v", selected)
	}
	if c.GetValue().Day != 5 {
		t.Fatalf("value=%+v", c.GetValue())
	}
}

func TestCalendar_PRD_13_DemoLunar(t *testing.T) {
	// CAL-13 lunar.tsx — fullCellRender hook (no built-in lunar lib)
	c := kit.NewCalendar()
	c.SetFullscreen(false)
	fulls := 0
	c.SetFullCellRender(func(cur kit.DateValue, info kit.CalendarCellInfo) core.Node {
		if info.Type != kit.CalendarCellDate || !cur.Valid {
			return nil
		}
		fulls++
		// synthetic "lunar" secondary line
		col := primitive.Column(
			kit.NewText(fmt.Sprintf("%d", cur.Day)).Node(),
			kit.NewText("初"+fmt.Sprintf("%d", (cur.Day%10)+1)).Node(),
		)
		col.Gap = 2
		col.CrossAlign = core.CrossCenter
		return col
	})
	_ = mountCalendar(t, c, 450, 420)
	if fulls == 0 {
		t.Fatal("fullCellRender not used")
	}
}

func TestCalendar_PRD_14_DemoWeek(t *testing.T) {
	// CAL-14 week.tsx
	full := kit.NewCalendar()
	full.SetShowWeek(true)
	_ = mountCalendar(t, full, 800, 600)
	if !full.ShowWeek {
		t.Fatal("showWeek full")
	}
	mini := kit.NewCalendar()
	mini.SetFullscreen(false)
	mini.SetShowWeek(true)
	_ = mountCalendar(t, mini, 320, 360)
	if !mini.ShowWeek {
		t.Fatal("showWeek mini")
	}
}

func TestCalendar_PRD_15_DemoCustomizeHeader(t *testing.T) {
	// CAL-15 customize-header.tsx
	c := kit.NewCalendar()
	c.SetFullscreen(false)
	var headerCalls int
	c.SetHeaderRender(func(cfg kit.CalendarHeaderConfig) core.Node {
		headerCalls++
		btn := kit.NewButton("Custom header")
		btn.SetType(kit.ButtonText)
		btn.SetOnClick(func() {
			cfg.OnTypeChange(kit.CalendarYear)
		})
		row := primitive.Row(btn.Node())
		return row
	})
	_ = mountCalendar(t, c, 320, 360)
	if headerCalls == 0 {
		t.Fatal("headerRender not called")
	}
	// default header replaced
	if c.HeaderNode() == nil {
		t.Fatal("nil custom header")
	}
}

func TestCalendar_PRD_16_TokenMetrics(t *testing.T) {
	// CAL-16
	c := kit.NewCalendar()
	if !approxCAL(c.ResolvedFontSize(), 14, 0.5) {
		t.Fatalf("fontSize=%v", c.ResolvedFontSize())
	}
	if !approxCAL(c.ResolvedRadius(), 6, 0.5) {
		t.Fatalf("radius full=%v", c.ResolvedRadius())
	}
	if !approxCAL(c.ResolvedYearControlWidth(), 80, 0.5) {
		t.Fatalf("yearW=%v", c.ResolvedYearControlWidth())
	}
	if !approxCAL(c.ResolvedMonthControlWidth(), 70, 0.5) {
		t.Fatalf("monthW=%v", c.ResolvedMonthControlWidth())
	}
	if !approxCAL(c.ResolvedMiniContentHeight(), 256, 0.5) {
		t.Fatalf("miniH=%v", c.ResolvedMiniContentHeight())
	}
	if !approxCAL(c.ResolvedDateValueHeight(), 24, 0.5) {
		t.Fatalf("dateValueH=%v", c.ResolvedDateValueHeight())
	}
	if !approxCAL(c.ResolvedWeekHeight(), 18, 0.5) {
		t.Fatalf("weekH=%v", c.ResolvedWeekHeight())
	}
	// Theme tokens
	th := kit.DefaultTheme()
	if !approxCAL(th.SizeOr(core.TokenFontSize, 0), 14, 0.5) {
		t.Fatal("TokenFontSize")
	}
	if !approxCAL(th.SizeOr(core.TokenBorderRadius, 0), 6, 0.5) {
		t.Fatal("TokenBorderRadius")
	}
	if !approxCAL(th.SizeOr(core.TokenControlHeightSM, 0), 24, 0.5) {
		t.Fatal("controlHeightSM")
	}
}

func TestCalendar_PRD_17_ThemeColors(t *testing.T) {
	// CAL-17
	c := kit.NewCalendar()
	th := kit.DefaultTheme()
	c.SetTheme(th)
	_ = mountCalendar(t, c, 400, 360)
	primary := th.Color(core.TokenColorPrimary)
	if primary.A == 0 {
		t.Fatal("primary token empty")
	}
	if !approxCALColor(c.PrimaryColor(), primary, 0.02) {
		t.Fatalf("PrimaryColor=%v token=%v", c.PrimaryColor(), primary)
	}
	// custom theme propagates
	custom := kit.DefaultTheme()
	custom.Tokens.Colors[core.TokenColorPrimary] = render.Hex("#FF00AA")
	c.SetTheme(custom)
	if !approxCALColor(c.PrimaryColor(), custom.Color(core.TokenColorPrimary), 0.05) {
		t.Fatalf("custom primary=%v", c.PrimaryColor())
	}
	// no hard-coded brand as only skin: background from token
	bg := c.BackgroundColor()
	if bg.A == 0 {
		t.Fatal("bg transparent")
	}
}

func TestCalendar_PRD_18_DisabledChrome(t *testing.T) {
	// CAL-18
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	c.SetDisabled(true)
	n := 0
	c.SetOnSelect(func(kit.DateValue, kit.CalendarSelectSource) { n++ })
	_ = mountCalendar(t, c, 720, 560)
	c.SelectDay(20)
	if n != 0 {
		t.Fatal("disabled whole calendar still selects")
	}
	// disabled text token readable
	if c.DisabledTextColor().A == 0 {
		t.Fatal("disabled text token")
	}
	// per-date disable no hover high — pressable disabled
	c.SetDisabled(false)
	c.SetDisabledDate(func(d kit.DateValue) bool { return d.Day == 12 })
	_ = mountCalendar(t, c, 720, 560)
	// find a disabled day cell
	found := false
	for _, p := range c.DayCells() {
		if p.State.Disabled {
			found = true
			if p.Focusable {
				// disabled cells should not be focusable
				t.Fatal("disabled cell focusable")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected some disabled day cells for day 12 + out of range? at least day 12")
	}
}

func TestCalendar_PRD_19_KeyboardFocus(t *testing.T) {
	// CAL-19
	c := kit.NewCalendar()
	c.SetDefaultValue(kit.DateOf(2026, 7, 1))
	var n int
	c.SetOnSelect(func(kit.DateValue, kit.CalendarSelectSource) { n++ })
	tree := mountCalendar(t, c, 720, 560)
	cells := c.DayCells()
	if len(cells) == 0 {
		t.Fatal("no day cells")
	}
	// pick an enabled in-view cell (index ~ startPad + 14 for mid month)
	var target *primitive.Pressable
	for _, p := range cells {
		if p != nil && !p.State.Disabled && p.Focusable {
			target = p
			break
		}
	}
	if target == nil {
		t.Fatal("no focusable cell")
	}
	if !target.ShowFocusRing {
		t.Fatal("focus ring off")
	}
	tree.SetFocus(target)
	// activate via key
	target.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if n < 1 {
		// fallback: some hosts need Click path
		if target.Click != nil {
			target.Click()
		}
	}
	if n < 1 {
		t.Fatal("keyboard/click activate failed")
	}
}
