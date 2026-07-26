//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerRadio() {
	// Radio — docs/antd/radio.md §6.8 P0
	// https://ant.design/components/radio
	// demos: basic / disabled / radiogroup / radiogroup-more / radiogroup-block /
	//        radiogroup-options / radiobutton / radiogroup-with-name
	face, th := c.face, c.theme
	wireG := func(g *kit.RadioGroup, tag string) *kit.RadioGroup {
		g.SetFace(face)
		if th != nil {
			g.SetTheme(th)
		}
		g.SetOnChange(func(v string) {
			*c.status = fmt.Sprintf("radio %s → %s", tag, v)
		})
		return g
	}
	wireR := func(r *kit.Radio, tag string) *kit.Radio {
		r.SetFace(face)
		if th != nil {
			r.SetTheme(th)
		}
		r.SetOnChange(func(v bool) {
			*c.status = fmt.Sprintf("radio %s → %v", tag, v)
		})
		return r
	}
	col := func(kids ...core.Node) core.Node {
		f := primitive.Column(kids...)
		f.Gap = 12
		f.CrossAlign = core.CrossStart
		f.MainAlign = core.MainStart
		return f
	}

	// ---------- basic.tsx ----------
	basic := wireR(kit.NewRadio("Radio"), "basic")
	secBasic := demoSection(face, th, "Basic",
		"The simplest use.",
		basic.Node())

	// ---------- disabled.tsx ----------
	disOff := wireR(kit.NewRadio("Disabled"), "dis-off")
	disOff.SetDefaultChecked(false)
	disOff.SetDisabled(true)
	disOn := wireR(kit.NewRadio("Disabled"), "dis-on")
	disOn.SetDefaultChecked(true)
	disOn.SetDisabled(true)
	btnToggle := c.trackBtn(kit.NewButton("Toggle disabled"))
	btnToggle.SetType(kit.ButtonPrimary)
	btnToggle.SetOnClick(func() {
		next := !disOff.Disabled
		disOff.SetDisabled(next)
		disOn.SetDisabled(next)
		*c.status = fmt.Sprintf("radio disabled → %v", next)
	})
	secDisabled := demoSection(face, th, "Disabled",
		"Disabled radio (off / on) with toggle.",
		col(spaceWrap(16, disOff.Node(), disOn.Node()), btnToggle.Node()))

	// ---------- radiogroup.tsx ----------
	rg := wireG(kit.NewRadioGroup(), "group")
	rg.SetOptions(
		kit.RadioOption{Label: "LineChart", Value: "1"},
		kit.RadioOption{Label: "DotChart", Value: "2"},
		kit.RadioOption{Label: "BarChart", Value: "3"},
		kit.RadioOption{Label: "PieChart", Value: "4"},
	)
	rg.SetDefaultValue("1")
	secGroup := demoSection(face, th, "Radio Group",
		"A group of radio components.",
		rg.Node())

	// ---------- radiogroup-more.tsx ----------
	vert := wireG(kit.NewRadioGroup(), "vertical")
	vert.SetVertical(true)
	vert.SetOptions(
		kit.RadioOption{Label: "Option A", Value: "1"},
		kit.RadioOption{Label: "Option B", Value: "2"},
		kit.RadioOption{Label: "Option C", Value: "3"},
		kit.RadioOption{Label: "More...", Value: "4", Title: "More"},
	)
	vert.SetDefaultValue("1")
	vertBtn := wireG(kit.NewRadioGroup(), "vertical-btn")
	vertBtn.SetVertical(true)
	vertBtn.SetOptionType(kit.RadioOptionButton)
	vertBtn.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Title: "Orange"},
	)
	secVertical := demoSection(face, th, "Vertical Radio.Group",
		"Vertical Radio.Group with more options + button type.",
		spaceWrap(32, vert.Node(), vertBtn.Node()))

	// ---------- radiogroup-block.tsx ----------
	blockOpts := []kit.RadioOption{
		{Label: "Apple", Value: "Apple"},
		{Label: "Pear", Value: "Pear"},
		{Label: "Orange", Value: "Orange"},
	}
	b1 := wireG(kit.NewRadioGroup(), "block-default")
	b1.SetBlock(true)
	b1.SetOptions(blockOpts...)
	b1.SetDefaultValue("Apple")
	b2 := wireG(kit.NewRadioGroup(), "block-solid")
	b2.SetBlock(true)
	b2.SetOptionType(kit.RadioOptionButton)
	b2.SetButtonStyle(kit.RadioButtonSolid)
	b2.SetOptions(blockOpts...)
	b2.SetDefaultValue("Apple")
	b3 := wireG(kit.NewRadioGroup(), "block-outline")
	b3.SetBlock(true)
	b3.SetOptionType(kit.RadioOptionButton)
	b3.SetOptions(blockOpts...)
	b3.SetDefaultValue("Pear")
	// Expand hosts so block fill is visible.
	blockHost := func(n core.Node) core.Node {
		h := primitive.NewDecorated(n)
		h.ExpandWidth = true
		return h
	}
	secBlock := demoSection(face, th, "Block Radio.Group",
		"block + default / button solid / button outline.",
		col(blockHost(b1.Node()), blockHost(b2.Node()), blockHost(b3.Node())))

	// ---------- radiogroup-options.tsx ----------
	o1 := wireG(kit.NewRadioGroup(), "opts-plain")
	o1.SetStringOptions("Apple", "Pear", "Orange")
	o1.SetDefaultValue("Apple")
	o2 := wireG(kit.NewRadioGroup(), "opts-disabled")
	o2.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Disabled: true},
	)
	o2.SetDefaultValue("Apple")
	o3 := wireG(kit.NewRadioGroup(), "opts-btn")
	o3.SetOptionType(kit.RadioOptionButton)
	o3.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Title: "Orange"},
	)
	o3.SetDefaultValue("Apple")
	o4 := wireG(kit.NewRadioGroup(), "opts-solid")
	o4.SetOptionType(kit.RadioOptionButton)
	o4.SetButtonStyle(kit.RadioButtonSolid)
	o4.SetOptions(
		kit.RadioOption{Label: "Apple", Value: "Apple"},
		kit.RadioOption{Label: "Pear", Value: "Pear"},
		kit.RadioOption{Label: "Orange", Value: "Orange", Disabled: true},
	)
	o4.SetDefaultValue("Apple")
	secOptions := demoSection(face, th, "Radio.Group options",
		"plainOptions / disabled option / button / solid button.",
		col(o1.Node(), o2.Node(), o3.Node(), o4.Node()))

	// ---------- radiobutton.tsx ----------
	mkBtnGroup := func(tag string, disableItem, disableGroup bool) *kit.RadioGroup {
		g := wireG(kit.NewRadioGroup(), tag)
		for _, p := range []struct{ v, l string }{
			{"a", "Hangzhou"}, {"b", "Shanghai"}, {"c", "Beijing"}, {"d", "Chengdu"},
		} {
			rb := kit.NewRadioButton(p.l)
			rb.SetFace(face)
			rb.SetValue(p.v)
			if disableItem && p.v == "b" {
				rb.SetDisabled(true)
			}
			g.Add(rb)
		}
		g.SetDefaultValue("a")
		if disableGroup {
			g.SetDisabled(true)
		}
		return g
	}
	secBtn := demoSection(face, th, "Radio button style",
		"Radio.Button group; middle row disables Shanghai; bottom disables whole group.",
		col(mkBtnGroup("btn-basic", false, false).Node(),
			mkBtnGroup("btn-item-dis", true, false).Node(),
			mkBtnGroup("btn-group-dis", false, true).Node()))

	// ---------- radiogroup-with-name.tsx ----------
	named := wireG(kit.NewRadioGroup(), "named")
	named.SetName("radiogroup")
	named.SetOptions(
		kit.RadioOption{Label: "A", Value: "1"},
		kit.RadioOption{Label: "B", Value: "2"},
		kit.RadioOption{Label: "C", Value: "3"},
		kit.RadioOption{Label: "D", Value: "4"},
	)
	named.SetDefaultValue("1")
	secName := demoSection(face, th, "Radio.Group with name",
		"Passing the name property to all input[type=\"radio\"] that are in the same Radio.Group.",
		named.Node())

	// Lifecycle (#9)
	life := kit.NewRadio("Lifecycle")
	life.SetFace(face)
	life.SetTheme(th)
	life.SetChecked(true)
	life.SetButtonMode(false)
	life.SetTextColor(th.Color(core.TokenColorPrimary))
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"chromeChange (Checked/TextColor) + structureChange (ButtonMode) — ensureBuilt 保树稳定。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeRadio, func(pc *core.PaintContext, n core.Node) {
		d, ok := n.(*primitive.Decorated)
		if !ok || d == nil {
			return
		}
		if d.Base().Key == "radio-skin-demo" {
			d.BorderWidth = 2
			d.BorderColor = render.Hex("#1677FF")
		}
		if p := baseSkin.Painter(kit.TypeRadio); p != nil {
			p(pc, d)
			return
		}
		primitive.PaintDecorated(pc, d)
	})
	skinR := kit.NewRadio("Skin radio")
	skinR.SetFace(face)
	skinR.SetTheme(th)
	skinR.SetChecked(true)
	if ind, ok := skinR.IndicatorNode().(*primitive.Decorated); ok {
		ind.Base().Key = "radio-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"indicator/button.SkinType=kit.Radio。Key=radio-skin-demo → 蓝边框 Override。",
		skinR.Node())

	page := demoPage(face,
		"Radio",
		"Select a single state from multiple options. P0 + #9 lifecycle + #6 Skin.",
		secBasic, secDisabled, secGroup, secVertical, secBlock, secOptions, secBtn, secName, secLife, secSkin,
	)
	c.addPage("radio", "Radio", page)
}
