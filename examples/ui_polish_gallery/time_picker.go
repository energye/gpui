//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTimePicker() {
	// TimePicker — docs/antd/time-picker.md §6.8 P0
	// demos: basic / value / size / need-confirm / disabled / hide-column / interval-options / addon
	// https://ant.design/components/time-picker
	//
	// P1 not shown: 12hours full page, change-on-scroll, range-picker full page,
	// variant full page, status/suffix/style-class demos.

	face, th := c.face, c.theme
	status := c.status
	vp := core.Size{Width: 1280, Height: 800}

	wire := func(tp *kit.TimePicker, tag string) *kit.TimePicker {
		if tp == nil {
			return nil
		}
		tp.SetFace(face)
		if th != nil {
			tp.SetTheme(th)
		}
		tp.Viewport = vp
		tp.SetOnChange(func(v kit.TimeValue, s string) {
			if status != nil {
				*status = fmt.Sprintf("TimePicker %s → %s", tag, s)
			}
		})
		tp.SetOnChangeRange(func(a, b kit.TimeValue, ss [2]string) {
			if status != nil {
				*status = fmt.Sprintf("TimePicker %s range → %s ~ %s", tag, ss[0], ss[1])
			}
		})
		tp.SetOnOpenChange(func(open bool) {
			if status != nil {
				*status = fmt.Sprintf("TimePicker %s open=%v", tag, open)
			}
		})
		tp.SetOnClear(func() {
			if status != nil {
				*status = fmt.Sprintf("TimePicker %s cleared", tag)
			}
		})
		return tp
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
	basic := wire(kit.NewTimePicker(), "basic")
	secBasic := demoSection(face, th, "基本",
		"最简单的用法，在浮层中选择时间。",
		basic.Node())

	// ── 受控组件 value.tsx ─────────────────────────────────
	ctrl := wire(kit.NewTimePicker(), "controlled")
	// controlled: parent keeps value via onChange → SetValue
	ctrl.SetOnChange(func(v kit.TimeValue, s string) {
		ctrl.SetValue(v)
		if status != nil {
			*status = fmt.Sprintf("TimePicker controlled → %s", s)
		}
	})
	secCtrl := demoSection(face, th, "受控组件",
		"value + onChange 受控；选择后由父级写回 SetValue。",
		ctrl.Node())

	// ── 三种大小 size.tsx ──────────────────────────────────
	szL := wire(kit.NewTimePicker(), "size-lg")
	szL.SetSize(kit.InputLarge)
	szL.SetDefaultValue(kit.TimeOf(12, 8, 23))
	szM := wire(kit.NewTimePicker(), "size-md")
	szM.SetDefaultValue(kit.TimeOf(12, 8, 23))
	szS := wire(kit.NewTimePicker(), "size-sm")
	szS.SetSize(kit.InputSmall)
	szS.SetDefaultValue(kit.TimeOf(12, 8, 23))
	secSize := demoSection(face, th, "三种大小",
		"size large / middle / small → 高度 40 / 32 / 24。",
		row(szL.Node(), szM.Node(), szS.Node()))

	// ── 选择确认 need-confirm.tsx ──────────────────────────
	need := wire(kit.NewTimePicker(), "needConfirm")
	need.SetNeedConfirm(true)
	need.SetOnOk(func(v kit.TimeValue, s string) {
		if status != nil {
			*status = fmt.Sprintf("TimePicker needConfirm OK → %s", s)
		}
	})
	secNeed := demoSection(face, th, "选择确认",
		"needConfirm：选时后需点 OK 才提交 onChange。",
		need.Node())

	// ── 禁用 disabled.tsx ──────────────────────────────────
	dis := wire(kit.NewTimePicker(), "disabled")
	dis.SetDefaultValue(kit.TimeOf(12, 8, 23))
	dis.SetDisabled(true)
	secDis := demoSection(face, th, "禁用",
		"disabled 灰态，不可打开与清除。",
		dis.Node())

	// ── 选择时分 hide-column.tsx ───────────────────────────
	hm := wire(kit.NewTimePicker(), "hhmm")
	hm.SetFormat("HH:mm")
	hm.SetDefaultValue(kit.TimeOf(12, 8, 0))
	secHM := demoSection(face, th, "选择时分",
		"format=HH:mm 时不展示秒列。",
		hm.Node())

	// ── 步长选项 interval-options.tsx ──────────────────────
	step := wire(kit.NewTimePicker(), "step")
	step.SetHourStep(1)
	step.SetMinuteStep(15)
	step.SetSecondStep(10)
	secStep := demoSection(face, th, "步长选项",
		"hourStep / minuteStep / secondStep 控制列间隔。",
		step.Node())

	// ── 附加内容 addon.tsx ─────────────────────────────────
	addon := wire(kit.NewTimePicker(), "addon")
	// controlled open like official demo
	addon.SetOnOpenChange(func(open bool) {
		addon.SetOpen(open)
		if status != nil {
			*status = fmt.Sprintf("TimePicker addon open=%v", open)
		}
	})
	addon.SetRenderExtraFooter(func() core.Node {
		btn := kit.NewButton("OK")
		btn.SetSize(kit.ButtonSmall)
		btn.SetType(kit.ButtonPrimary)
		btn.SetFace(face)
		btn.OnClick = func() {
			addon.SetOpen(false)
			if status != nil {
				*status = "TimePicker addon footer OK"
			}
		}
		return btn.Node()
	})
	secAddon := demoSection(face, th, "附加内容",
		"renderExtraFooter 在面板底部渲染附加内容；配合受控 open。",
		addon.Node())

	// ── status / variant 抽测（P0 配置，非完整官方页）──────
	stErr := wire(kit.NewTimePicker(), "status-err")
	stErr.SetStatus(kit.InputStatusError)
	stErr.SetDefaultValue(kit.TimeOf(10, 0, 0))
	stWarn := wire(kit.NewTimePicker(), "status-warn")
	stWarn.SetStatus(kit.InputStatusWarning)
	filled := wire(kit.NewTimePicker(), "variant-filled")
	filled.SetVariant(kit.InputFilled)
	under := wire(kit.NewTimePicker(), "variant-underlined")
	under.SetVariant(kit.InputUnderlined)
	secExtra := demoSection(face, th, "状态 / 形态（抽测）",
		"status=error|warning；variant filled|underlined。完整页见 P1。",
		col(
			row(stErr.Node(), stWarn.Node()),
			row(filled.Node(), under.Node()),
		))

	c.addPage("time_picker", "TimePicker",
		demoPage(face, "TimePicker",
			"输入或选择时间的控件。P0 对齐 docs/antd/time-picker.md §6。",
			secBasic, secCtrl, secSize, secNeed, secDis, secHM, secStep, secAddon, secExtra,
		))
}
