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

func (c *catalogCtx) registerDatePicker() {
	// DatePicker — docs/antd/date-picker.md §6.8 P0
	// demos: basic / range-picker / multiple / needConfirm / switchable / format / time / mask
	// https://ant.design/components/date-picker
	//
	// P1 not shown: controlled mode panel, min/maxDate demos, presets, cellRender,
	// disabled full demo page, allow-empty deep input.

	face, th := c.face, c.theme
	status := c.status
	vp := core.Size{Width: 1280, Height: 800}

	wire := func(dp *kit.DatePicker, tag string) *kit.DatePicker {
		if dp == nil {
			return nil
		}
		dp.SetFace(face)
		if th != nil {
			dp.SetTheme(th)
		}
		dp.Viewport = vp
		dp.SetOnChange(func(v kit.DateValue, s string) {
			if status != nil {
				*status = fmt.Sprintf("DatePicker %s → %s", tag, s)
			}
		})
		dp.SetOnChangeRange(func(a, b kit.DateValue, ss [2]string) {
			if status != nil {
				*status = fmt.Sprintf("DatePicker %s range → %s ~ %s", tag, ss[0], ss[1])
			}
		})
		dp.SetOnChangeMulti(func(vs []kit.DateValue, ss []string) {
			if status != nil {
				*status = fmt.Sprintf("DatePicker %s multi → %v", tag, ss)
			}
		})
		dp.SetOnOpenChange(func(open bool) {
			if status != nil {
				*status = fmt.Sprintf("DatePicker %s open=%v", tag, open)
			}
		})
		return dp
	}
	row := func(kids ...core.Node) core.Node {
		return spaceWrap(12, kids...)
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}

	// ── 基本 basic.tsx ─────────────────────────────────────
	basicDate := wire(kit.NewDatePicker(), "basic-date")
	basicWeek := wire(kit.NewDatePicker(), "basic-week")
	basicWeek.SetPicker(kit.DatePickerWeek)
	basicMonth := wire(kit.NewDatePicker(), "basic-month")
	basicMonth.SetPicker(kit.DatePickerMonth)
	basicQuarter := wire(kit.NewDatePicker(), "basic-quarter")
	basicQuarter.SetPicker(kit.DatePickerQuarter)
	basicYear := wire(kit.NewDatePicker(), "basic-year")
	basicYear.SetPicker(kit.DatePickerYear)
	secBasic := demoSection(face, th, "基本",
		"最简单的用法，在浮层中可以选择或者输入日期。picker 支持 date / week / month / quarter / year。",
		col(
			basicDate.Node(),
			basicWeek.Node(),
			basicMonth.Node(),
			basicQuarter.Node(),
			basicYear.Node(),
		))

	// ── 范围选择器 range-picker.tsx ────────────────────────
	rg := wire(kit.NewRangePicker(), "range")
	rgTime := wire(kit.NewRangePicker(), "range-time")
	rgTime.SetShowTime(true)
	rgMonth := wire(kit.NewRangePicker(), "range-month")
	rgMonth.SetPicker(kit.DatePickerMonth)
	secRange := demoSection(face, th, "范围选择器",
		"通过 NewRangePicker 选择一段时间。可与 showTime / picker 组合。",
		col(rg.Node(), rgTime.Node(), rgMonth.Node()))

	// ── 多选 multiple.tsx ──────────────────────────────────
	multiSM := wire(kit.NewDatePicker(), "multi-sm")
	multiSM.SetMultiple(true)
	multiSM.SetSize(kit.InputSmall)
	multiSM.SetDefaultMultiValue([]kit.DateValue{
		kit.DateOf(2000, 1, 1), kit.DateOf(2000, 1, 3), kit.DateOf(2000, 1, 5),
	})
	multiMD := wire(kit.NewDatePicker(), "multi-md")
	multiMD.SetMultiple(true)
	multiMD.SetDefaultMultiValue([]kit.DateValue{
		kit.DateOf(2000, 1, 1), kit.DateOf(2000, 1, 3), kit.DateOf(2000, 1, 5),
	})
	multiLG := wire(kit.NewDatePicker(), "multi-lg")
	multiLG.SetMultiple(true)
	multiLG.SetSize(kit.InputLarge)
	multiLG.SetDefaultMultiValue([]kit.DateValue{
		kit.DateOf(2000, 1, 1), kit.DateOf(2000, 1, 3), kit.DateOf(2000, 1, 5),
	})
	secMulti := demoSection(face, th, "多选",
		"multiple 多选日期；size 支持 small / middle / large。",
		col(multiSM.Node(), multiMD.Node(), multiLG.Node()))

	// ── 选择确认 needConfirm.tsx ───────────────────────────
	need := wire(kit.NewDatePicker(), "needConfirm")
	need.SetNeedConfirm(true)
	need.SetOnOk(func(v kit.DateValue, s string) {
		if status != nil {
			*status = fmt.Sprintf("DatePicker needConfirm OK → %s", s)
		}
	})
	secNeed := demoSection(face, th, "选择确认",
		"needConfirm：选日后需点 OK 才提交 onChange。",
		need.Node())

	// ── 切换不同的选择器 switchable.tsx ────────────────────
	sw := wire(kit.NewDatePicker(), "switchable")
	opts := []kit.SelectOption{
		{Value: "date", Label: "Date"},
		{Value: "week", Label: "Week"},
		{Value: "month", Label: "Month"},
		{Value: "quarter", Label: "Quarter"},
		{Value: "year", Label: "Year"},
	}
	sel := kit.NewSelect("Picker Type", opts...)
	sel.SetFace(face)
	sel.SetValue("date")
	sel.OnChange = func(v string) {
		switch v {
		case "week":
			sw.SetPicker(kit.DatePickerWeek)
		case "month":
			sw.SetPicker(kit.DatePickerMonth)
		case "quarter":
			sw.SetPicker(kit.DatePickerQuarter)
		case "year":
			sw.SetPicker(kit.DatePickerYear)
		default:
			sw.SetPicker(kit.DatePickerDate)
		}
		if status != nil {
			*status = "DatePicker switchable picker=" + v
		}
	}
	secSwitch := demoSection(face, th, "切换不同的选择器",
		"提供选择器，自由切换不同类型的日期选择器。",
		row(sel.Node(), sw.Node()))

	// ── 日期格式 format.tsx ────────────────────────────────
	fmt1 := wire(kit.NewDatePicker(), "fmt1")
	fmt1.SetFormat("YYYY/MM/DD")
	fmt1.SetDefaultValue(kit.DateOf(2015, 1, 1))
	fmt2 := wire(kit.NewDatePicker(), "fmt2")
	fmt2.SetFormat("DD/MM/YYYY")
	fmt2.SetDefaultValue(kit.DateOf(2015, 1, 1))
	fmtM := wire(kit.NewDatePicker(), "fmt-month")
	fmtM.SetPicker(kit.DatePickerMonth)
	fmtM.SetFormat("YYYY/MM")
	fmtM.SetDefaultValue(kit.DateOf(2015, 1, 1))
	fmtR := wire(kit.NewRangePicker(), "fmt-range")
	fmtR.SetFormat("YYYY/MM/DD")
	fmtR.SetDefaultRangeValue(kit.DateOf(2015, 1, 1), kit.DateOf(2015, 1, 1))
	secFormat := demoSection(face, th, "日期格式",
		"使用 format 属性，可以自定义日期显示格式。",
		col(fmt1.Node(), fmt2.Node(), fmtM.Node(), fmtR.Node()))

	// ── 日期时间选择 time.tsx ──────────────────────────────
	dt := wire(kit.NewDatePicker(), "datetime")
	dt.SetShowTime(true)
	dtR := wire(kit.NewRangePicker(), "datetime-range")
	dtR.SetShowTime(true)
	dtR.SetFormat("YYYY-MM-DD HH:mm")
	secTime := demoSection(face, th, "日期时间选择",
		"增加选择时间功能，当 showTime 为一个对象时，其属性会传递给内建的 TimePicker。",
		col(dt.Node(), dtR.Node()))

	// ── 格式对齐 mask.tsx ──────────────────────────────────
	mask1 := wire(kit.NewDatePicker(), "mask")
	mask1.SetFormat("YYYY-MM-DD")
	mask1.SetFormatMask(true)
	mask2 := wire(kit.NewDatePicker(), "mask-time")
	mask2.SetFormat("YYYY-MM-DD HH:mm:ss")
	mask2.SetFormatMask(true)
	mask2.SetShowTime(true)
	secMask := demoSection(face, th, "格式对齐",
		"format type=mask：占位与格式对齐（P0 展示层）。",
		col(mask1.Node(), mask2.Node()))

	// ── size / status / disabled 抽测 ──────────────────────
	szS := wire(kit.NewDatePicker(), "sz-sm")
	szS.SetSize(kit.InputSmall)
	szM := wire(kit.NewDatePicker(), "sz-md")
	szL := wire(kit.NewDatePicker(), "sz-lg")
	szL.SetSize(kit.InputLarge)
	stErr := wire(kit.NewDatePicker(), "status-err")
	stErr.SetStatus(kit.InputStatusError)
	stErr.SetDefaultValue(kit.DateOf(time.Now().Year(), int(time.Now().Month()), 1))
	dis := wire(kit.NewDatePicker(), "disabled")
	dis.SetDisabled(true)
	dis.SetDefaultValue(kit.DateOf(2026, 1, 1))
	secExtra := demoSection(face, th, "尺寸 / 状态 / 禁用",
		"size 24/32/40；status=error；disabled 不可打开。",
		col(
			row(szS.Node(), szM.Node(), szL.Node()),
			stErr.Node(),
			dis.Node(),
		))

	// Lifecycle (#9)
	life := wire(kit.NewDatePicker(), "lifecycle")
	life.SetDefaultValue(kit.DateOf(2026, 7, 26))
	life.SetSize(kit.InputLarge)
	life.SetStatus(kit.InputStatusWarning)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange (Size) then chromeChange (Status)；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeDatePicker, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "date-picker-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeDatePicker); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinDP := wire(kit.NewDatePicker(), "skin")
	skinNode := skinDP.Node()
	if shell := skinDP.TriggerShell(); shell != nil {
		if kids := shell.Children(); len(kids) > 0 {
			if dec, ok := kids[0].(*primitive.Decorated); ok {
				dec.Base().Key = "date-picker-skin-demo"
			}
		}
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"decor.SkinType=kit.DatePicker。Key=date-picker-skin-demo → 蓝色 2px 边框 Override。",
		skinNode)

	c.addPage("date_picker", "DatePicker",
		demoPage(face, "DatePicker",
			"输入或选择日期的控件。P0 对齐 docs/antd/date-picker.md §6。",
			secBasic, secRange, secMulti, secNeed, secSwitch, secFormat, secTime, secMask, secExtra, secLife, secSkin,
		))
}
