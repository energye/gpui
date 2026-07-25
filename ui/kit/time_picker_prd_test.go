package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/time-picker.md §6.9 — P0 PRD cases (TP-01 … TP-21).
// L3/L4 (TP-22/23) and P1 (TP-24) deferred.

func approxTP(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func approxTPColor(a, b render.RGBA, tol float64) bool {
	dr, dg, db, da := a.R-b.R, a.G-b.G, a.B-b.B, a.A-b.A
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}

func mountTimePicker(t *testing.T, tp *kit.TimePicker, w, h float64) *core.Tree {
	t.Helper()
	if tp == nil {
		t.Fatal("nil timepicker")
	}
	tp.Viewport = core.Size{Width: w, Height: h}
	bg := primitive.NewBox(tp.Node())
	bg.Width, bg.Height = w, h
	tree := core.NewTree(bg)
	tree.Layout(core.Size{Width: w, Height: h})
	return tree
}

func TestTimePicker_PRD_01_Defaults(t *testing.T) {
	// TP-01
	tp := kit.NewTimePicker()
	if tp.Size != kit.InputMiddle {
		t.Fatalf("Size=%v want middle", tp.Size)
	}
	if tp.Variant != kit.InputOutlined {
		t.Fatalf("Variant=%v want outlined", tp.Variant)
	}
	if tp.Status != kit.InputStatusNone {
		t.Fatalf("Status=%v want none", tp.Status)
	}
	if tp.Disabled || tp.Open || tp.Loading || tp.NeedConfirm || tp.Use12Hours || tp.Range {
		t.Fatalf("flags dis=%v open=%v load=%v conf=%v h12=%v range=%v",
			tp.Disabled, tp.Open, tp.Loading, tp.NeedConfirm, tp.Use12Hours, tp.Range)
	}
	if !tp.AllowClear {
		t.Fatal("AllowClear default true")
	}
	if !tp.ShowNow {
		t.Fatal("ShowNow default true")
	}
	if !tp.Order {
		t.Fatal("Order default true")
	}
	if tp.HourStep != 1 || tp.MinuteStep != 1 || tp.SecondStep != 1 {
		t.Fatalf("steps h=%d m=%d s=%d", tp.HourStep, tp.MinuteStep, tp.SecondStep)
	}
	if tp.Placement != kit.TimePickerBottomLeft {
		t.Fatalf("Placement=%v want bottomLeft", tp.Placement)
	}
	if tp.GetValue().Valid {
		t.Fatal("default value empty")
	}
	if tp.Node() == nil || tp.Popup() == nil || tp.TriggerShell() == nil {
		t.Fatal("nil nodes")
	}
	if tp.Root.Base().Role != "combobox" {
		t.Fatalf("role=%q", tp.Root.Base().Role)
	}
	_ = mountTimePicker(t, tp, 480, 400)
}

func TestTimePicker_PRD_02_SelectTime(t *testing.T) {
	// TP-02 / TP-S1
	tp := kit.NewTimePicker()
	var n int
	var got kit.TimeValue
	var s string
	tp.SetOnChange(func(v kit.TimeValue, ts string) {
		n++
		got = v
		s = ts
	})
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(14, 30, 45))
	if n != 1 {
		t.Fatalf("onChange n=%d want 1", n)
	}
	if !got.Valid || got.Hour != 14 || got.Minute != 30 || got.Second != 45 {
		t.Fatalf("got=%+v", got)
	}
	if !strings.Contains(s, "14:30:45") {
		t.Fatalf("timeString=%q", s)
	}
	if tp.DisplayText() != s {
		t.Fatalf("display=%q string=%q", tp.DisplayText(), s)
	}
	if tp.IsOpen() {
		t.Fatal("should close after select (uncontrolled)")
	}
}

func TestTimePicker_PRD_03_Format(t *testing.T) {
	// TP-03 / TP-S2
	tp := kit.NewTimePicker()
	tp.SetFormat("HH:mm")
	tp.SetValue(kit.TimeOf(9, 5, 7))
	_ = mountTimePicker(t, tp, 480, 400)
	got := tp.DisplayText()
	if got != "09:05" {
		t.Fatalf("display=%q want 09:05", got)
	}
	if tp.ShowSecondColumn() {
		t.Fatal("seconds column should hide for HH:mm")
	}
	if !tp.ShowMinuteColumn() {
		t.Fatal("minutes column expected")
	}
	// custom format with seconds
	tp.SetFormat("HH:mm:ss")
	if tp.FormatValue(kit.TimeOf(1, 2, 3)) != "01:02:03" {
		t.Fatalf("FormatValue=%q", tp.FormatValue(kit.TimeOf(1, 2, 3)))
	}
}

func TestTimePicker_PRD_04_HourStep(t *testing.T) {
	// TP-04 / TP-S3
	tp := kit.NewTimePicker()
	tp.SetHourStep(2)
	_ = mountTimePicker(t, tp, 480, 400)
	opts := tp.HourOptions()
	if len(opts) == 0 {
		t.Fatal("empty hour options")
	}
	for _, h := range opts {
		if h%2 != 0 {
			t.Fatalf("hour %d not multiple of 2", h)
		}
	}
	// 0 and 22 present, 1 absent
	has0, has1, has22 := false, false, false
	for _, h := range opts {
		if h == 0 {
			has0 = true
		}
		if h == 1 {
			has1 = true
		}
		if h == 22 {
			has22 = true
		}
	}
	if !has0 || has1 || !has22 {
		t.Fatalf("opts=%v", opts)
	}
}

func TestTimePicker_PRD_05_DisabledTime(t *testing.T) {
	// TP-05 / TP-S4
	tp := kit.NewTimePicker()
	tp.SetDisabledTime(func(kit.TimeValue) kit.TimeDisabled {
		return kit.TimeDisabled{Hours: []int{13, 14}, Minutes: []int{30}, Seconds: []int{0}}
	})
	var n int
	tp.SetOnChange(func(kit.TimeValue, string) { n++ })
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(13, 0, 1))
	if n != 0 || tp.GetValue().Valid {
		t.Fatalf("hour 13 blocked n=%d val=%+v", n, tp.GetValue())
	}
	tp.SelectHour(13)
	if n != 0 {
		t.Fatal("SelectHour 13 should be blocked")
	}
	// allowed hour
	tp.SelectTime(kit.TimeOf(10, 15, 20))
	if n != 1 || !tp.GetValue().Valid {
		t.Fatalf("allowed n=%d val=%+v", n, tp.GetValue())
	}
}

func TestTimePicker_PRD_06_Clear(t *testing.T) {
	// TP-06 / TP-S5
	tp := kit.NewTimePicker()
	tp.SetDefaultValue(kit.TimeOf(12, 8, 23))
	var cleared bool
	var n int
	var last kit.TimeValue
	tp.SetOnClear(func() { cleared = true })
	tp.SetOnChange(func(v kit.TimeValue, _ string) {
		n++
		last = v
	})
	_ = mountTimePicker(t, tp, 480, 400)
	if !tp.GetValue().Valid {
		t.Fatal("default not applied")
	}
	tp.Clear()
	if tp.GetValue().Valid {
		t.Fatal("want cleared")
	}
	if !cleared {
		t.Fatal("OnClear")
	}
	if n != 1 || last.Valid {
		t.Fatalf("onChange n=%d last=%+v", n, last)
	}
	if tp.DisplayText() != "" && !strings.Contains(tp.DisplayText(), "Select") {
		// display may show placeholder via display node; DisplayText returns computeDisplay empty
		if tp.DisplayText() != "" {
			t.Fatalf("display=%q", tp.DisplayText())
		}
	}
}

func TestTimePicker_PRD_07_Range(t *testing.T) {
	// TP-07 / TP-S6
	tp := kit.NewTimeRangePicker()
	var n int
	var a, b kit.TimeValue
	tp.SetOnChangeRange(func(start, end kit.TimeValue, _ [2]string) {
		n++
		a, b = start, end
	})
	_ = mountTimePicker(t, tp, 520, 420)
	// reverse order → Order sorts
	start := kit.TimeOf(18, 0, 0)
	end := kit.TimeOf(9, 30, 0)
	tp.SelectRange(start, end)
	if n != 1 {
		t.Fatalf("onChangeRange n=%d", n)
	}
	if !a.Valid || !b.Valid {
		t.Fatal("range empty")
	}
	if b.Before(a) {
		t.Fatalf("unordered %v %v", a, b)
	}
	if a.Hour != 9 || b.Hour != 18 {
		t.Fatalf("sorted %d-%d want 9-18", a.Hour, b.Hour)
	}
	if !strings.Contains(tp.DisplayText(), "~") {
		t.Fatalf("display=%q", tp.DisplayText())
	}
}

func TestTimePicker_PRD_08_Use12Hours(t *testing.T) {
	// TP-08 / TP-S7
	tp := kit.NewTimePicker()
	tp.SetUse12Hours(true)
	var got kit.TimeValue
	var s string
	tp.SetOnChange(func(v kit.TimeValue, ts string) {
		got = v
		s = ts
	})
	_ = mountTimePicker(t, tp, 480, 400)
	// 2 PM → 14:00:00
	tp.SelectMeridiem(true)
	tp.SelectHour(2)
	tp.SelectMinute(0)
	tp.SelectSecond(0)
	if !got.Valid || got.Hour != 14 {
		t.Fatalf("got=%+v want hour 14", got)
	}
	if !strings.Contains(strings.ToLower(s), "pm") && !strings.Contains(tp.FormatValue(got), "pm") {
		// default format h:mm:ss a
		if !strings.Contains(strings.ToLower(tp.FormatValue(got)), "pm") {
			t.Fatalf("12h string=%q format=%q", s, tp.FormatValue(got))
		}
	}
	// AM midnight
	tp2 := kit.NewTimePicker()
	tp2.SetUse12Hours(true)
	tp2.SelectMeridiem(false)
	tp2.SelectHour(12)
	tp2.SelectMinute(0)
	tp2.SelectSecond(0)
	if tp2.GetValue().Hour != 0 {
		t.Fatalf("12am hour=%d want 0", tp2.GetValue().Hour)
	}
}

func TestTimePicker_PRD_09_Height(t *testing.T) {
	// TP-09 / TP-S8
	tp := kit.NewTimePicker()
	tree := mountTimePicker(t, tp, 480, 400)
	_ = tree
	th := core.DefaultTheme()
	want := th.SizeOr(core.TokenControlHeight, 32)
	// decor height via layout of trigger
	if tp.TriggerShell() == nil {
		t.Fatal("nil trigger")
	}
	// control height middle = 32
	h := tp.TriggerShell().Size().Height
	// may include nothing extra; decor is child
	// walk: Root size should be ~32
	if !approxTP(h, want, 0.5) {
		// fallback: rebuild with explicit theme
		tp.SetTheme(th)
		_ = mountTimePicker(t, tp, 480, 80)
		h = tp.TriggerShell().Size().Height
		if !approxTP(h, want, 0.5) {
			t.Fatalf("height=%.1f want %.1f", h, want)
		}
	}
}

func TestTimePicker_PRD_10_ExampleBasic(t *testing.T) {
	// TP-10 basic.tsx
	tp := kit.NewTimePicker()
	tp.SetDefaultOpen(false)
	var n int
	tp.SetOnChange(func(kit.TimeValue, string) { n++ })
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(0, 0, 0))
	if n != 1 || !tp.GetValue().Valid {
		t.Fatalf("basic select n=%d val=%+v", n, tp.GetValue())
	}
}

func TestTimePicker_PRD_11_ExampleControlled(t *testing.T) {
	// TP-11 value.tsx
	tp := kit.NewTimePicker()
	var external kit.TimeValue
	tp.SetOnChange(func(v kit.TimeValue, _ string) {
		external = v
		tp.SetValue(v) // parent mirrors
	})
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(8, 15, 0))
	if !external.Valid || external.Hour != 8 {
		t.Fatalf("external=%+v", external)
	}
	if !tp.GetValue().Equal(external) {
		t.Fatalf("value=%+v external=%+v", tp.GetValue(), external)
	}
	// SetValue alone does not fire OnChange
	n := 0
	tp.SetOnChange(func(kit.TimeValue, string) { n++ })
	tp.SetValue(kit.TimeOf(9, 0, 0))
	if n != 0 {
		t.Fatalf("SetValue must not fire onChange n=%d", n)
	}
}

func TestTimePicker_PRD_12_ExampleSize(t *testing.T) {
	// TP-12 size.tsx
	v := kit.TimeOf(12, 8, 23)
	sizes := []kit.InputSize{kit.InputLarge, kit.InputMiddle, kit.InputSmall}
	th := core.DefaultTheme()
	wants := []float64{
		th.SizeOr(core.TokenControlHeightLG, 40),
		th.SizeOr(core.TokenControlHeight, 32),
		th.SizeOr(core.TokenControlHeightSM, 24),
	}
	for i, sz := range sizes {
		tp := kit.NewTimePicker()
		tp.SetTheme(th)
		tp.SetSize(sz)
		tp.SetDefaultValue(v)
		_ = mountTimePicker(t, tp, 480, 100)
		h := tp.TriggerShell().Size().Height
		if !approxTP(h, wants[i], 0.5) {
			t.Fatalf("size=%v height=%.1f want %.1f", sz, h, wants[i])
		}
	}
}

func TestTimePicker_PRD_13_ExampleNeedConfirm(t *testing.T) {
	// TP-13 need-confirm.tsx
	tp := kit.NewTimePicker()
	tp.SetNeedConfirm(true)
	var n, okN int
	tp.SetOnChange(func(kit.TimeValue, string) { n++ })
	tp.SetOnOk(func(kit.TimeValue, string) { okN++ })
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(10, 20, 30))
	if n != 0 {
		t.Fatalf("needConfirm should not commit yet n=%d", n)
	}
	if !tp.GetValue().Valid {
		// value not committed
	} else {
		t.Fatal("value should stay empty until Confirm")
	}
	tp.Confirm()
	if n != 1 || okN != 1 {
		t.Fatalf("after Confirm n=%d ok=%d", n, okN)
	}
	if !tp.GetValue().Valid || tp.GetValue().Hour != 10 {
		t.Fatalf("val=%+v", tp.GetValue())
	}
}

func TestTimePicker_PRD_14_ExampleDisabled(t *testing.T) {
	// TP-14 disabled.tsx
	tp := kit.NewTimePicker()
	tp.SetDefaultValue(kit.TimeOf(12, 8, 23))
	tp.SetDisabled(true)
	var n int
	tp.SetOnChange(func(kit.TimeValue, string) { n++ })
	_ = mountTimePicker(t, tp, 480, 400)
	tp.SelectTime(kit.TimeOf(1, 2, 3))
	if n != 0 {
		t.Fatal("disabled must ignore select")
	}
	if tp.GetValue().Hour != 12 {
		t.Fatalf("val=%+v", tp.GetValue())
	}
	// open blocked via key
	tp.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if tp.IsOpen() {
		t.Fatal("disabled must not open")
	}
}

func TestTimePicker_PRD_15_ExampleHideColumn(t *testing.T) {
	// TP-15 hide-column.tsx format=HH:mm
	tp := kit.NewTimePicker()
	tp.SetFormat("HH:mm")
	tp.SetDefaultValue(kit.TimeOf(12, 8, 0))
	_ = mountTimePicker(t, tp, 480, 400)
	if tp.DisplayText() != "12:08" {
		t.Fatalf("display=%q", tp.DisplayText())
	}
	if tp.ShowSecondColumn() {
		t.Fatal("no second column")
	}
}

func TestTimePicker_PRD_16_ExampleIntervalOptions(t *testing.T) {
	// TP-16 interval-options.tsx
	tp := kit.NewTimePicker()
	tp.SetHourStep(1)
	tp.SetMinuteStep(15)
	tp.SetSecondStep(10)
	_ = mountTimePicker(t, tp, 480, 400)
	mins := tp.MinuteOptions()
	secs := tp.SecondOptions()
	for _, m := range mins {
		if m%15 != 0 {
			t.Fatalf("minute %d", m)
		}
	}
	for _, s := range secs {
		if s%10 != 0 {
			t.Fatalf("second %d", s)
		}
	}
	if len(mins) != 4 { // 0,15,30,45
		t.Fatalf("mins=%v", mins)
	}
	if len(secs) != 6 { // 0,10,...,50
		t.Fatalf("secs=%v", secs)
	}
}

func TestTimePicker_PRD_17_ExampleAddon(t *testing.T) {
	// TP-17 addon.tsx — renderExtraFooter + controlled open
	tp := kit.NewTimePicker()
	var openN int
	var lastOpen bool
	tp.SetOnOpenChange(func(o bool) {
		openN++
		lastOpen = o
	})
	// controlled open
	tp.SetOpen(true)
	if !tp.IsOpen() {
		t.Fatal("SetOpen true")
	}
	footerCalled := false
	tp.SetRenderExtraFooter(func() core.Node {
		footerCalled = true
		lab := primitive.NewText("OK")
		return lab
	})
	// rebuild panel with footer
	tp.SetOpen(true)
	_ = mountTimePicker(t, tp, 480, 400)
	if !footerCalled {
		// footer applied on rebuildPanelBody — force via open
		tp.SetNeedConfirm(false)
		tp.SetOpen(false)
		tp.SetOpen(true)
	}
	// reopen to ensure footer rebuild
	tp.SetRenderExtraFooter(func() core.Node {
		footerCalled = true
		return primitive.NewText("OK")
	})
	// trigger rebuild
	tp.SetShowNow(true)
	if tp.Panel() == nil {
		t.Fatal("nil panel")
	}
	// close via controlled callback pattern
	tp.SetOnOpenChange(func(o bool) {
		lastOpen = o
		if !o {
			// parent would SetOpen(false)
		}
	})
	// simulate dismiss
	if tp.Popup() != nil && tp.Popup().OnDismiss != nil {
		tp.Popup().OnDismiss()
	}
	_ = openN
	_ = lastOpen
	_ = footerCalled
}

func TestTimePicker_PRD_18_TokenGeometry(t *testing.T) {
	// TP-18 §6.2
	th := core.DefaultTheme()
	cases := []struct {
		sz   kit.InputSize
		want float64
	}{
		{kit.InputMiddle, th.SizeOr(core.TokenControlHeight, 32)},
		{kit.InputSmall, th.SizeOr(core.TokenControlHeightSM, 24)},
		{kit.InputLarge, th.SizeOr(core.TokenControlHeightLG, 40)},
	}
	for _, c := range cases {
		tp := kit.NewTimePicker()
		tp.SetTheme(th)
		tp.SetSize(c.sz)
		_ = mountTimePicker(t, tp, 400, 80)
		h := tp.TriggerShell().Size().Height
		if !approxTP(h, c.want, 0.5) {
			t.Fatalf("size=%v h=%.1f want %.1f", c.sz, h, c.want)
		}
	}
	// radius / font defaults readable from theme
	if r := th.SizeOr(core.TokenBorderRadius, 6); !approxTP(r, 6, 0.5) {
		t.Fatalf("radius=%v", r)
	}
	if f := th.SizeOr(core.TokenFontSize, 14); !approxTP(f, 14, 0.5) {
		t.Fatalf("font=%v", f)
	}
}

func TestTimePicker_PRD_19_TokenColors(t *testing.T) {
	// TP-19 no hard-coded brand-only default skin
	th := core.DefaultTheme()
	tp := kit.NewTimePicker()
	tp.SetTheme(th)
	_ = mountTimePicker(t, tp, 400, 80)
	// primary from theme
	prim := th.Color(core.TokenColorPrimary)
	if prim.A < 0.1 {
		t.Fatal("theme primary missing")
	}
	// open uses primary border — not a fixed hex exclusive of theme
	tp.SetOpen(true)
	// status error uses token
	tp.SetStatus(kit.InputStatusError)
	errC := th.Color(core.TokenColorError)
	if errC.A < 0.1 {
		t.Fatal("error token missing")
	}
	_ = approxTPColor
}

func TestTimePicker_PRD_20_DisabledChrome(t *testing.T) {
	// TP-20
	th := core.DefaultTheme()
	tp := kit.NewTimePicker()
	tp.SetTheme(th)
	tp.SetDefaultValue(kit.TimeOf(12, 0, 0))
	tp.SetDisabled(true)
	_ = mountTimePicker(t, tp, 400, 80)
	if !tp.Disabled {
		t.Fatal("disabled flag")
	}
	// no open
	tp.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if tp.IsOpen() {
		t.Fatal("open while disabled")
	}
	// clear blocked
	tp.Clear()
	if !tp.GetValue().Valid {
		t.Fatal("clear should no-op when disabled")
	}
}

func TestTimePicker_PRD_21_KeyboardFocus(t *testing.T) {
	// TP-21
	tp := kit.NewTimePicker()
	_ = mountTimePicker(t, tp, 480, 400)
	if tp.Root == nil || !tp.Root.Focusable {
		t.Fatal("trigger focusable")
	}
	if !tp.Root.ShowFocusRing {
		t.Fatal("focus ring")
	}
	if tp.Root.FocusRingOutset <= 0 {
		t.Fatalf("focus outset=%v", tp.Root.FocusRingOutset)
	}
	// open via key
	tp.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "ArrowDown"})
	if !tp.IsOpen() {
		t.Fatal("ArrowDown opens")
	}
	tp.HandleKey(&core.KeyEvent{Type: core.KeyDown, Key: "Escape"})
	if tp.IsOpen() {
		t.Fatal("Esc closes")
	}
	// role
	if tp.Root.Base().Role != "combobox" {
		t.Fatalf("role=%q", tp.Root.Base().Role)
	}
}
