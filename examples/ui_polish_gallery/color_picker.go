//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerColorPicker() {
	// ColorPicker — docs/antd/color-picker.md §6.8 P0
	// https://ant.design/components/color-picker
	// demos: base / size / controlled / line-gradient / text-render / disabled / disabled-alpha / allowClear
	face, th := c.face, c.theme
	wire := func(cp *kit.ColorPicker, tag string) *kit.ColorPicker {
		cp.SetFace(face)
		if th != nil {
			cp.SetTheme(th)
		}
		cp.SetOnChange(func(col kit.Color, css string) {
			*c.status = fmt.Sprintf("color_picker %s → %s", tag, css)
		})
		cp.SetOnOpenChange(func(open bool) {
			*c.status = fmt.Sprintf("color_picker %s open=%v", tag, open)
		})
		return cp
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}
	row := func(kids ...core.Node) core.Node {
		return spaceWrap(12, kids...)
	}

	// ---------- base.tsx ----------
	base := wire(kit.NewColorPicker(), "base")
	base.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	secBase := demoSection(face, th, "Basic",
		"The most basic usage.",
		base.Node())

	// ---------- size.tsx ----------
	sm := wire(kit.NewColorPicker(), "size-sm")
	sm.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	sm.SetSize(kit.InputSmall)
	md := wire(kit.NewColorPicker(), "size-md")
	md.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	lg := wire(kit.NewColorPicker(), "size-lg")
	lg.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	lg.SetSize(kit.InputLarge)
	smT := wire(kit.NewColorPicker(), "size-sm-text")
	smT.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	smT.SetSize(kit.InputSmall)
	smT.SetShowText(true)
	mdT := wire(kit.NewColorPicker(), "size-md-text")
	mdT.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	mdT.SetShowText(true)
	lgT := wire(kit.NewColorPicker(), "size-lg-text")
	lgT.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	lgT.SetSize(kit.InputLarge)
	lgT.SetShowText(true)
	secSize := demoSection(face, th, "Trigger size",
		"small / middle / large; with and without showText.",
		row(
			col(sm.Node(), md.Node(), lg.Node()),
			col(smT.Node(), mdT.Node(), lgT.Node()),
		))

	// ---------- controlled.tsx ----------
	ctrl := wire(kit.NewColorPicker(), "controlled-change")
	color := kit.ColorFromHex("#1677ff")
	ctrl.SetValue(color)
	ctrl.SetOnChange(func(col kit.Color, css string) {
		color = col
		ctrl.SetValue(col)
		*c.status = fmt.Sprintf("color_picker controlled onChange → %s", css)
	})
	ctrlComplete := wire(kit.NewColorPicker(), "controlled-complete")
	ctrlComplete.SetValue(color)
	ctrlComplete.SetOnChangeComplete(func(col kit.Color) {
		color = col
		ctrl.SetValue(col)
		ctrlComplete.SetValue(col)
		*c.status = fmt.Sprintf("color_picker controlled onChangeComplete → %s", col.ToHexString())
	})
	// keep both in sync on change too
	ctrlComplete.SetOnChange(func(col kit.Color, css string) {
		// preview only; complete commits
		*c.status = fmt.Sprintf("color_picker controlled complete-path change → %s", css)
	})
	secCtrl := demoSection(face, th, "Controlled",
		"value + onChange (left) and onChangeComplete (right).",
		row(ctrl.Node(), ctrlComplete.Node()))

	// ---------- line-gradient.tsx ----------
	stops := []kit.ColorStop{
		{Color: render.Hex("#108ee9"), Percent: 0},
		{Color: render.Hex("#87d068"), Percent: 100},
	}
	gradBoth := wire(kit.NewColorPicker(), "grad-both")
	gradBoth.SetDefaultValue(kit.ColorGradient(stops...))
	gradBoth.SetAllowClear(true)
	gradBoth.SetShowText(true)
	gradBoth.SetModes(kit.ColorModeSingle, kit.ColorModeGradient)
	gradBoth.SetMode(kit.ColorModeGradient)
	gradOnly := wire(kit.NewColorPicker(), "grad-only")
	gradOnly.SetDefaultValue(kit.ColorGradient(stops...))
	gradOnly.SetAllowClear(true)
	gradOnly.SetShowText(true)
	gradOnly.SetMode(kit.ColorModeGradient)
	secGrad := demoSection(face, th, "Gradient",
		"mode single|gradient and pure gradient.",
		col(gradBoth.Node(), gradOnly.Node()))

	// ---------- text-render.tsx ----------
	txt1 := wire(kit.NewColorPicker(), "text-basic")
	txt1.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	txt1.SetShowText(true)
	txt1.SetAllowClear(true)
	txt2 := wire(kit.NewColorPicker(), "text-custom")
	txt2.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	txt2.SetShowTextRender(func(col kit.Color) string {
		return "Custom Text (" + col.ToHexString() + ")"
	})
	txt3Open := false
	txt3 := wire(kit.NewColorPicker(), "text-open")
	txt3.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	txt3.SetShowTextRender(func(kit.Color) string {
		if txt3Open {
			return "▲"
		}
		return "▼"
	})
	txt3.SetOnOpenChange(func(o bool) {
		txt3Open = o
		// refresh text via re-set render
		txt3.SetShowTextRender(func(kit.Color) string {
			if txt3Open {
				return "▲"
			}
			return "▼"
		})
		*c.status = fmt.Sprintf("color_picker text-open open=%v", o)
	})
	secText := demoSection(face, th, "Trigger text",
		"showText, custom text render, open-linked text.",
		col(txt1.Node(), txt2.Node(), txt3.Node()))

	// ---------- disabled.tsx ----------
	dis := wire(kit.NewColorPicker(), "disabled")
	dis.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	dis.SetShowText(true)
	dis.SetDisabled(true)
	secDis := demoSection(face, th, "Disabled",
		"Disabled ColorPicker with showText.",
		dis.Node())

	// ---------- disabled-alpha.tsx ----------
	noA := wire(kit.NewColorPicker(), "no-alpha")
	noA.SetDefaultValue(kit.ColorFromHex("#1677ff"))
	noA.SetDisabledAlpha(true)
	secNoA := demoSection(face, th, "Disabled alpha",
		"Hide alpha slider (disabledAlpha).",
		noA.Node())

	// ---------- allowClear.tsx ----------
	clr := wire(kit.NewColorPicker(), "allow-clear")
	clrVal := kit.ColorFromHex("#1677ff")
	clr.SetValue(clrVal)
	clr.SetAllowClear(true)
	clr.SetOnChange(func(col kit.Color, css string) {
		clrVal = col
		clr.SetValue(col)
		*c.status = fmt.Sprintf("color_picker allowClear → %q", css)
	})
	clr.SetOnClear(func() {
		*c.status = "color_picker allowClear → cleared"
	})
	secClr := demoSection(face, th, "Allow clear",
		"allowClear with controlled value.",
		clr.Node())

	c.addPage("color_picker", "ColorPicker",
		demoPage(face, "ColorPicker",
			"Color selection trigger + panel. P0 aligns docs/antd/color-picker.md §6 "+
				"(value/defaultValue/onChange/onChangeComplete, open, size, mode, showText, "+
				"disabled, disabledAlpha, allowClear, format hex).",
			secBase, secSize, secCtrl, secGrad, secText, secDis, secNoA, secClr,
		))
}
