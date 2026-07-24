//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerCheckbox() {
	// Checkbox — docs/antd/checkbox.md §6.8 P0
	// https://ant.design/components/checkbox
	// demos: basic / disabled / controller / group / check-all / layout / style-class / _semantic
	face, th := c.face, c.theme
	wire := func(cb *kit.Checkbox, tag string) *kit.Checkbox {
		cb.SetFace(face)
		if th != nil {
			cb.SetTheme(th)
		}
		cb.SetOnChange(func(v bool) {
			*c.status = fmt.Sprintf("checkbox %s → %v", tag, v)
		})
		return cb
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}

	// ---------- basic.tsx ----------
	basic := wire(kit.NewCheckbox("Checkbox"), "basic")
	secBasic := demoSection(face, th, "Basic",
		"The most basic usage.",
		basic.Node())

	// ---------- disabled.tsx ----------
	disOff := wire(kit.NewCheckbox(""), "dis-off")
	disOff.SetDefaultChecked(false)
	disOff.SetDisabled(true)
	disInd := wire(kit.NewCheckbox(""), "dis-ind")
	disInd.SetIndeterminate(true)
	disInd.SetDisabled(true)
	disOn := wire(kit.NewCheckbox(""), "dis-on")
	disOn.SetDefaultChecked(true)
	disOn.SetDisabled(true)
	secDisabled := demoSection(face, th, "Disabled",
		"Disabled state of Checkbox (off / indeterminate / on).",
		col(disOff.Node(), disInd.Node(), disOn.Node()))

	// ---------- controller.tsx ----------
	ctrl := wire(kit.NewCheckbox("Checked-Enabled"), "controlled")
	ctrl.SetControlled(true)
	ctrl.SetChecked(true)
	ctrl.SetOnChange(func(v bool) {
		ctrl.SetChecked(v)
		label := "Unchecked"
		if v {
			label = "Checked"
		}
		en := "Enabled"
		if ctrl.Disabled {
			en = "Disabled"
		}
		ctrl.SetLabel(label + "-" + en)
		*c.status = fmt.Sprintf("checkbox controlled → %v", v)
	})
	btnCheck := c.trackBtn(kit.NewButton("Uncheck"))
	btnCheck.SetType(kit.ButtonPrimary)
	btnCheck.SetSize(kit.ButtonSmall)
	btnCheck.SetOnClick(func() {
		next := !ctrl.Checked
		ctrl.SetChecked(next)
		if next {
			btnCheck.SetLabel("Uncheck")
		} else {
			btnCheck.SetLabel("Check")
		}
		label := "Unchecked"
		if next {
			label = "Checked"
		}
		en := "Enabled"
		if ctrl.Disabled {
			en = "Disabled"
		}
		ctrl.SetLabel(label + "-" + en)
		*c.status = fmt.Sprintf("checkbox controlled btn → %v", next)
	})
	btnDis := c.trackBtn(kit.NewButton("Disable"))
	btnDis.SetType(kit.ButtonPrimary)
	btnDis.SetSize(kit.ButtonSmall)
	btnDis.SetOnClick(func() {
		ctrl.SetDisabled(!ctrl.Disabled)
		if ctrl.Disabled {
			btnDis.SetLabel("Enable")
		} else {
			btnDis.SetLabel("Disable")
		}
		label := "Unchecked"
		if ctrl.Checked {
			label = "Checked"
		}
		en := "Enabled"
		if ctrl.Disabled {
			en = "Disabled"
		}
		ctrl.SetLabel(label + "-" + en)
		*c.status = fmt.Sprintf("checkbox controlled disabled=%v", ctrl.Disabled)
	})
	secControlled := demoSection(face, th, "Controlled",
		"checked + onChange; parent owns value. Toggle buttons mirror antd controller demo.",
		col(ctrl.Node(), spaceWrap(10, btnCheck.Node(), btnDis.Node())))

	// ---------- group.tsx ----------
	g1 := kit.NewCheckboxGroup()
	g1.SetFace(face)
	g1.SetStringOptions("Apple", "Pear", "Orange")
	g1.SetDefaultValue([]string{"Apple"})
	g1.SetOnChange(func(v []string) {
		*c.status = fmt.Sprintf("group plain → %v", v)
	})
	g2 := kit.NewCheckboxGroup()
	g2.SetFace(face)
	g2.SetOptions(
		kit.CheckboxOption{Label: "Apple", Value: "Apple"},
		kit.CheckboxOption{Label: "Pear", Value: "Pear"},
		kit.CheckboxOption{Label: "Orange", Value: "Orange"},
	)
	g2.SetDefaultValue([]string{"Pear"})
	g2.SetOnChange(func(v []string) {
		*c.status = fmt.Sprintf("group options → %v", v)
	})
	g3 := kit.NewCheckboxGroup()
	g3.SetFace(face)
	g3.SetOptions(
		kit.CheckboxOption{Label: "Apple", Value: "Apple"},
		kit.CheckboxOption{Label: "Pear", Value: "Pear"},
		kit.CheckboxOption{Label: "Orange", Value: "Orange"},
	)
	g3.SetDefaultValue([]string{"Apple"})
	g3.SetDisabled(true)
	secGroup := demoSection(face, th, "Checkbox Group",
		"Generate a group of checkboxes from options. Third row is disabled.",
		col(g1.Node(), g2.Node(), g3.Node()))

	// ---------- check-all.tsx ----------
	plain := []string{"Apple", "Pear", "Orange"}
	checkAll := wire(kit.NewCheckbox("Check all"), "check-all")
	checkAll.SetControlled(true)
	groupAll := kit.NewCheckboxGroup()
	groupAll.SetFace(face)
	groupAll.SetStringOptions(plain...)
	groupAll.SetValue([]string{"Apple", "Orange"})
	syncCheckAll := func() {
		vals := groupAll.Values()
		all := len(vals) == len(plain)
		half := len(vals) > 0 && !all
		checkAll.SetChecked(all)
		if half {
			checkAll.SetIndeterminate(true)
		}
	}
	syncCheckAll()
	checkAll.SetOnChange(func(v bool) {
		if v {
			groupAll.SetValue(plain)
		} else {
			groupAll.SetValue(nil)
		}
		syncCheckAll()
		*c.status = fmt.Sprintf("check-all → %v vals=%v", v, groupAll.Values())
	})
	groupAll.SetOnChange(func(v []string) {
		// controlled group: parent must apply
		groupAll.SetValue(v)
		syncCheckAll()
		*c.status = fmt.Sprintf("check-all group → %v", v)
	})
	div := kit.NewDivider()
	secCheckAll := demoSection(face, th, "Check all",
		"Indeterminate check-all + controlled group.",
		col(checkAll.Node(), div.Node(), groupAll.Node()))

	// ---------- layout.tsx ----------
	gLayout := kit.NewCheckboxGroup()
	gLayout.SetFace(face)
	var layoutBoxes []*kit.Checkbox
	for _, l := range []string{"A", "B", "C", "D", "E"} {
		cb := kit.NewCheckbox(l)
		cb.SetFace(face)
		cb.SetValue(l)
		layoutBoxes = append(layoutBoxes, cb)
	}
	gLayout.Add(layoutBoxes...)
	row := kit.NewRow()
	for _, cb := range layoutBoxes {
		colN := kit.NewCol(cb.Node())
		colN.SetSpan(8)
		row.Add(colN.Node())
	}
	gLayout.SetBody(row.Node())
	gLayout.SetOnChange(func(v []string) {
		*c.status = fmt.Sprintf("layout group → %v", v)
	})
	secLayout := demoSection(face, th, "Layout",
		"Use with Grid (Row/Col span=8) inside Checkbox.Group.",
		gLayout.Node())

	// ---------- style-class.tsx (P0 approximation via Style) ----------
	warn := th.Color(core.TokenColorWarning)
	styleObj := wire(kit.NewCheckbox("Object styles"), "style-obj")
	styleObj.SetStyle(kit.Style{
		Border: warn,
		Text:   th.Color(core.TokenColorPrimary),
		Radius: 6,
	})
	styleFn := wire(kit.NewCheckbox("Function styles"), "style-fn")
	styleFn.SetDefaultChecked(true)
	styleFn.SetStyle(kit.Style{Background: warn, Text: warn})
	secStyle := demoSection(face, th, "Custom styles (semantic)",
		"P0 maps antd styles/classNames to Style overrides on icon/label (full semantic fn depth is P1).",
		col(styleObj.Node(), styleFn.Node()))

	// ---------- _semantic.tsx ----------
	sem := wire(kit.NewCheckbox("Checkbox"), "semantic")
	semNote := kit.NewParagraph("Semantic parts: root (Pressable), icon (indicator 16×16), label (text).")
	semNote.SetFace(face)
	secSemantic := demoSection(face, th, "Semantic structure",
		"root / icon / label hooks for P0; classNames/styles function form is P1.",
		col(sem.Node(), semNote.Node()))

	page := demoPage(face,
		"Checkbox",
		"Collect user multiple selections. P0: checked/defaultChecked, indeterminate, onChange, disabled, title, Group value/defaultValue/options, a11y.",
		secBasic, secDisabled, secControlled, secGroup, secCheckAll, secLayout, secStyle, secSemantic,
	)
	c.addPage("checkbox", "Checkbox", page)
}
