//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/kit"
)

func (c *catalogCtx) registerSwitch() {
	// Switch — docs/antd/switch.md §6.8 P0
	// https://ant.design/components/switch
	trackSw := func(s *kit.Switch) *kit.Switch {
		*c.tickers = append(*c.tickers, s)
		return s
	}
	swBasic := trackSw(kit.NewSwitch())
	swBasic.SetDefaultChecked(true)
	swBasic.SetOnChange(func(v bool) { *c.status = fmt.Sprintf("switch → %v", v) })
	swBasic.SetAriaLabel("basic")

	swDis := trackSw(kit.NewSwitch())
	swDis.SetDefaultChecked(true)
	swDis.SetDisabled(true)
	swDisToggle := kit.NewButton("Toggle disabled")
	swDisToggle.SetType(kit.ButtonPrimary)
	swDisToggle.SetOnClick(func() {
		swDis.SetDisabled(!swDis.Disabled)
		if swDis.Disabled {
			*c.status = "switch disabled"
		} else {
			*c.status = "switch enabled"
		}
	})
	*c.buttons = append(*c.buttons, swDisToggle)

	swText1 := trackSw(kit.NewSwitch())
	swText1.SetFace(c.face)
	swText1.SetCheckedChildren("On")
	swText1.SetUnCheckedChildren("Off")
	swText1.SetDefaultChecked(true)
	swText2 := trackSw(kit.NewSwitch())
	swText2.SetFace(c.face)
	swText2.SetCheckedChildren("1")
	swText2.SetUnCheckedChildren("0")
	swText2.SetDefaultChecked(true)

	swMed := trackSw(kit.NewSwitch())
	swMed.SetDefaultChecked(true)
	swSm := trackSw(kit.NewSwitch())
	swSm.SetSize(kit.SwitchSmall)
	swSm.SetDefaultChecked(true)

	swLoad1 := trackSw(kit.NewSwitch())
	swLoad1.SetLoading(true)
	swLoad1.SetDefaultChecked(true)
	swLoad2 := trackSw(kit.NewSwitch())
	swLoad2.SetSize(kit.SwitchSmall)
	swLoad2.SetLoading(true)

	swCtrl := trackSw(kit.NewSwitch())
	swCtrl.SetControlled(true)
	swCtrl.SetChecked(false)
	swCtrl.SetOnChange(func(v bool) {
		// Parent applies value (controlled demo).
		swCtrl.SetChecked(v)
		*c.status = fmt.Sprintf("controlled → %v", v)
	})
	swCtrl.SetAriaLabel("controlled")

	secSwBasic := demoSection(c.face, c.theme, "Basic",
		"The most basic usage.",
		spaceWrap(12, swBasic.Node()))
	secSwDis := demoSection(c.face, c.theme, "Disabled",
		"Disabled state of Switch.",
		spaceWrap(12, swDis.Node(), swDisToggle.Node()))
	secSwText := demoSection(c.face, c.theme, "Text & icon",
		"With text checkedChildren / unCheckedChildren (string).",
		spaceWrap(12, swText1.Node(), swText2.Node()))
	secSwSize := demoSection(c.face, c.theme, "Two sizes",
		"size=medium (default) and size=small.",
		spaceWrap(12, swMed.Node(), swSm.Node()))
	secSwLoad := demoSection(c.face, c.theme, "Loading",
		"Mark a pending state of switch.",
		spaceWrap(12, swLoad1.Node(), swLoad2.Node()))
	secSwCtrl := demoSection(c.face, c.theme, "Controlled",
		"SetControlled + SetChecked: parent owns the value.",
		spaceWrap(12, swCtrl.Node()))
	c.add("switch", "Switch", "Data Entry · Switch",
		demoPage(c.face, "Switch",
			"Switching Selector. P0: checked/value, defaultChecked, controlled, onChange/onClick, disabled, loading, size, children text.",
			secSwBasic, secSwDis, secSwText, secSwSize, secSwLoad, secSwCtrl))
}
