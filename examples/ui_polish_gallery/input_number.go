//go:build linux && !nogpu

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerInputNumber() {
	// InputNumber — docs/antd/input-number.md §6.8 P0
	// https://ant.design/components/input-number
	// demos: basic / size / disabled / digit / formatter / keyboard /
	//         change-on-wheel / variant
	//
	// P1 not shown: spinner full page, out-of-range, presuffix, status page,
	// custom controls icons, semantic classNames/styles.

	face, th := c.face, c.theme
	track := func(n *kit.InputNumber) *kit.InputNumber {
		if n == nil {
			return nil
		}
		n.SetFace(face)
		if th != nil {
			n.SetTheme(th)
		}
		return n
	}
	statusf := func(tag string) func(float64) {
		return func(v float64) {
			*c.status = fmt.Sprintf("inputNumber %s → %v", tag, v)
		}
	}

	// ---------- basic.tsx ----------
	basic := track(kit.NewInputNumberValue(3))
	basic.SetMin(1)
	basic.SetMax(10)
	basic.SetOnChange(statusf("basic"))
	basic.SetFixedSize(120, 0)
	secBasic := demoSection(face, th, "Basic",
		"Numeric input with min/max and step handlers (antd basic.tsx).",
		basic.Node())

	// ---------- size.tsx ----------
	mkSize := func(s kit.InputSize, label string) core.Node {
		n := track(kit.NewInputNumberValue(3))
		n.SetMin(1)
		n.SetMax(100000)
		n.SetSize(s)
		n.SetOnChange(statusf("size-" + label))
		n.SetFixedSize(140, 0)
		return n.Node()
	}
	sizeRow := primitive.Row(
		mkSize(kit.InputLarge, "large"),
		mkSize(kit.InputMiddle, "middle"),
		mkSize(kit.InputSmall, "small"),
	)
	sizeRow.Gap = 12
	sizeRow.CrossAlign = core.CrossCenter
	secSize := demoSection(face, th, "Three sizes",
		"There are three sizes: large (40), middle (32) and small (24).",
		sizeRow)

	// ---------- disabled.tsx ----------
	dis := track(kit.NewInputNumberValue(3))
	dis.SetMin(1)
	dis.SetMax(10)
	dis.SetDisabled(true)
	dis.SetFixedSize(120, 0)
	btnToggle := c.trackBtn(kit.NewButton("Toggle disabled"))
	btnToggle.SetType(kit.ButtonPrimary)
	btnToggle.SetOnClick(func() {
		dis.SetDisabled(!dis.Disabled)
		*c.status = fmt.Sprintf("inputNumber disabled=%v", dis.Disabled)
	})
	disCol := primitive.Column(dis.Node(), btnToggle.Node())
	disCol.Gap = 12
	disCol.CrossAlign = core.CrossStart
	secDisabled := demoSection(face, th, "Disabled",
		"Click the button to toggle disabled state.",
		disCol)

	// ---------- digit.tsx ----------
	digit := track(kit.NewInputNumberValue(1))
	digit.SetMin(0)
	digit.SetMax(10)
	digit.SetStep(0.00000000000001)
	digit.SetStringMode(true)
	digit.SetOnChange(statusf("digit"))
	digit.SetFixedSize(200, 0)
	secDigit := demoSection(face, th, "High precision decimals",
		"stringMode + tiny step for high precision decimals (antd digit.tsx).",
		digit.Node())

	// ---------- formatter.tsx ----------
	money := track(kit.NewInputNumberValue(1000))
	money.SetFormatter(func(v float64, _ kit.InputNumberFormatterInfo) string {
		raw := strconv.FormatFloat(v, 'f', -1, 64)
		parts := strings.Split(raw, ".")
		intp := parts[0]
		var b strings.Builder
		for i, r := range intp {
			if i > 0 && (len(intp)-i)%3 == 0 {
				b.WriteByte(',')
			}
			b.WriteRune(r)
		}
		if len(parts) > 1 {
			return "$ " + b.String() + "." + parts[1]
		}
		return "$ " + b.String()
	})
	money.SetParser(func(s string) (float64, bool) {
		s = strings.ReplaceAll(s, "$", "")
		s = strings.ReplaceAll(s, ",", "")
		s = strings.TrimSpace(s)
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	})
	money.SetOnChange(statusf("money"))
	money.SetFixedSize(160, 0)

	pct := track(kit.NewInputNumberValue(100))
	pct.SetMin(0)
	pct.SetMax(100)
	pct.SetFormatter(func(v float64, _ kit.InputNumberFormatterInfo) string {
		return strconv.FormatFloat(v, 'f', -1, 64) + "%"
	})
	pct.SetParser(func(s string) (float64, bool) {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		v, err := strconv.ParseFloat(s, 64)
		return v, err == nil
	})
	pct.SetOnChange(statusf("pct"))
	pct.SetFixedSize(120, 0)

	fmtRow := primitive.Row(money.Node(), pct.Node())
	fmtRow.Gap = 12
	fmtRow.CrossAlign = core.CrossCenter
	secFmt := demoSection(face, th, "Formatter",
		"Display value with formatter / parser (currency & percent).",
		fmtRow)

	// ---------- keyboard.tsx ----------
	kb := track(kit.NewInputNumberValue(3))
	kb.SetMin(1)
	kb.SetMax(10)
	kb.SetKeyboard(true)
	kb.SetOnChange(statusf("keyboard"))
	kb.SetFixedSize(120, 0)
	kbCheck := kit.NewCheckbox("Toggle keyboard")
	kbCheck.SetFace(face)
	kbCheck.SetChecked(true)
	kbCheck.SetOnChange(func(v bool) {
		kb.SetKeyboard(v)
		*c.status = fmt.Sprintf("inputNumber keyboard=%v", v)
	})
	kbRow := primitive.Row(kb.Node(), kbCheck.Node())
	kbRow.Gap = 12
	kbRow.CrossAlign = core.CrossCenter
	secKB := demoSection(face, th, "Keyboard",
		"Control whether keyboard up/down can change the value.",
		kbRow)

	// ---------- change-on-wheel.tsx ----------
	wheel := track(kit.NewInputNumberValue(3))
	wheel.SetMin(1)
	wheel.SetMax(10)
	wheel.SetChangeOnWheel(true)
	wheel.SetOnChange(statusf("wheel"))
	wheel.SetOnStep(func(v float64, info kit.InputNumberStepInfo) {
		*c.status = fmt.Sprintf("inputNumber wheel onStep %v emitter=%s", v, info.Emitter)
	})
	wheel.SetFixedSize(120, 0)
	secWheel := demoSection(face, th, "Change on wheel",
		"Focus the field then use mouse wheel to step (antd change-on-wheel.tsx).",
		wheel.Node())

	// ---------- variant.tsx ----------
	mkVar := func(v kit.InputVariant, ph string) core.Node {
		n := track(kit.NewInputNumber())
		n.SetPlaceholder(ph)
		n.SetVariant(v)
		n.SetFixedSize(200, 0)
		return n.Node()
	}
	varCol := primitive.Column(
		mkVar(kit.InputOutlined, "Outlined"),
		mkVar(kit.InputFilled, "Filled"),
		mkVar(kit.InputBorderless, "Borderless"),
		mkVar(kit.InputUnderlined, "Underlined"),
	)
	varCol.Gap = 12
	varCol.CrossAlign = core.CrossStart
	secVariant := demoSection(face, th, "Variants",
		"Variants of InputNumber: outlined, filled, borderless, underlined.",
		varCol)

	c.addPage("input_number", "InputNumber",
		demoPage(face, "InputNumber",
			"Enter a number within certain range with keyboard, wheel, or handlers.",
			secBasic, secSize, secDisabled, secDigit, secFmt, secKB, secWheel, secVariant,
		))
}
