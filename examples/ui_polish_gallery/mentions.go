//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerMentions() {
	// Mentions — docs/antd/mentions.md §6.8 P0
	// https://ant.design/components/mentions
	// demos: basic / size / variant / async / form / prefix / readonly / placement
	//
	// P1 not shown: allowClear custom icon, autoSize, status page, style-class.

	face, th := c.face, c.theme
	track := func(m *kit.Mentions) *kit.Mentions {
		if m == nil {
			return nil
		}
		m.SetFace(face)
		if th != nil {
			m.SetTheme(th)
		}
		c.trackTicker(m)
		return m
	}
	statusf := func(tag string) func(string) {
		return func(v string) {
			*c.status = fmt.Sprintf("mentions %s → %q", tag, v)
		}
	}
	defaultOpts := []kit.MentionsOption{
		{Value: "afc163", Label: "afc163", Key: "afc163"},
		{Value: "zombieJ", Label: "zombieJ", Key: "zombieJ"},
		{Value: "yesmeck", Label: "yesmeck", Key: "yesmeck"},
	}

	// ---------- basic.tsx ----------
	basic := track(kit.NewMentions(""))
	basic.SetDefaultValue("@afc163")
	basic.SetOptions(defaultOpts)
	basic.SetOnChange(statusf("basic"))
	basic.SetOnSelect(func(opt kit.MentionsOption, prefix string) {
		*c.status = fmt.Sprintf("mentions select %s%v", prefix, opt.Value)
	})
	basic.SetFixedWidth(360)
	secBasic := demoSection(face, th, "Basic",
		"Type @ to mention someone. defaultValue + options + onChange/onSelect (antd basic.tsx).",
		basic.Node())

	// ---------- size.tsx ----------
	mkSize := func(s kit.InputSize, ph string) core.Node {
		m := track(kit.NewMentions(ph))
		m.SetSize(s)
		m.SetOptions(defaultOpts)
		m.SetFixedWidth(280)
		return m.Node()
	}
	sizeCol := primitive.Column(
		mkSize(kit.InputLarge, "large size"),
		mkSize(kit.InputMiddle, "default size"),
		mkSize(kit.InputSmall, "small size"),
	)
	sizeCol.Gap = 12
	sizeCol.CrossAlign = core.CrossStart
	secSize := demoSection(face, th, "Three sizes",
		"large / middle / small control the font size ladder (antd size.tsx).",
		sizeCol)

	// ---------- variant.tsx ----------
	mkVar := func(v kit.InputVariant, ph string) core.Node {
		m := track(kit.NewMentions(ph))
		m.SetVariant(v)
		m.SetOptions(defaultOpts)
		m.SetFixedWidth(280)
		return m.Node()
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
		"outlined / filled / borderless / underlined (antd variant.tsx).",
		varCol)

	// ---------- async.tsx ----------
	asyncM := track(kit.NewMentions("search users after @"))
	asyncM.SetFilterOption(false)
	asyncM.SetFixedWidth(360)
	asyncM.SetOnSearch(func(text, prefix string) {
		*c.status = fmt.Sprintf("mentions async search %q prefix=%s", text, prefix)
		if text == "" {
			asyncM.SetLoading(false)
			asyncM.SetOptions(nil)
			return
		}
		asyncM.SetLoading(true)
		asyncM.SetOptions(nil)
		// Mock async resolve (gallery has no network).
		asyncM.SetLoading(false)
		asyncM.SetOptions([]kit.MentionsOption{
			{Value: text, Label: text + " (mock)"},
			{Value: text + "2", Label: text + "2 (mock)"},
			{Value: text + "3", Label: text + "3 (mock)"},
		})
	})
	secAsync := demoSection(face, th, "Async loading",
		"onSearch + loading spinner (Ticker). Mock options — no network (antd async.tsx).",
		asyncM.Node())

	// ---------- form.tsx ----------
	coders := track(kit.NewMentions(""))
	coders.SetRows(1)
	coders.SetOptions(defaultOpts)
	coders.SetFixedWidth(320)
	coders.SetOnChange(statusf("form-coders"))
	bio := track(kit.NewMentions("You can use @ to ref user here"))
	bio.SetRows(3)
	bio.SetOptions(defaultOpts)
	bio.SetFixedWidth(320)
	bio.SetOnChange(func(v string) {
		ents := kit.GetMentions(v, nil, "")
		*c.status = fmt.Sprintf("mentions form bio mentions=%d value=%q", len(ents), v)
	})
	submit := c.trackBtn(kit.NewButton("Submit"))
	submit.SetType(kit.ButtonPrimary)
	submit.SetOnClick(func() {
		ents := kit.GetMentions(bio.GetValue(), nil, "")
		if len(ents) < 2 {
			*c.status = "mentions form: need ≥2 mentions in Bio"
			return
		}
		*c.status = fmt.Sprintf("mentions form submit ok coders=%q bio=%q", coders.GetValue(), bio.GetValue())
	})
	reset := c.trackBtn(kit.NewButton("Reset"))
	reset.SetOnClick(func() {
		coders.SetValue("")
		bio.SetValue("")
		*c.status = "mentions form reset"
	})
	btnRow := primitive.Row(submit.Node(), reset.Node())
	btnRow.Gap = 8
	formCol := primitive.Column(
		kit.NewFormItem("coders", "Top coders", coders.Node()).Node(),
		kit.NewFormItem("bio", "Bio", bio.Node()).Node(),
		btnRow,
	)
	formCol.Gap = 12
	formCol.CrossAlign = core.CrossStart
	secForm := demoSection(face, th, "With Form",
		"rows=1 / rows=3 + GetMentions validation (≥2) (antd form.tsx).",
		formCol)

	// ---------- prefix.tsx ----------
	prefixM := track(kit.NewMentions("input @ to mention people, # to mention tag"))
	prefixM.SetPrefix("@", "#")
	prefixM.SetFixedWidth(360)
	mock := map[string][]string{
		"@": {"afc163", "zombiej", "yesmeck"},
		"#": {"1.0", "2.0", "3.0"},
	}
	prefixM.SetOnSearch(func(_, prefix string) {
		vals := mock[prefix]
		opts := make([]kit.MentionsOption, len(vals))
		for i, v := range vals {
			opts[i] = kit.MentionsOption{Value: v, Key: v, Label: v}
		}
		prefixM.SetOptions(opts)
		*c.status = fmt.Sprintf("mentions prefix=%s options=%d", prefix, len(opts))
	})
	secPrefix := demoSection(face, th, "Custom trigger",
		"prefix=['@','#'] switches option sets via onSearch (antd prefix.tsx).",
		prefixM.Node())

	// ---------- readonly.tsx ----------
	dis := track(kit.NewMentions("this is disabled Mentions"))
	dis.SetOptions(defaultOpts)
	dis.SetDisabled(true)
	dis.SetFixedWidth(360)
	ro := track(kit.NewMentions("this is readOnly Mentions"))
	ro.SetOptions(defaultOpts)
	ro.SetReadOnly(true)
	ro.SetFixedWidth(360)
	roCol := primitive.Column(dis.Node(), ro.Node())
	roCol.Gap = 10
	roCol.CrossAlign = core.CrossStart
	secRO := demoSection(face, th, "Disabled or readOnly",
		"disabled blocks all interaction; readOnly is focusable but not editable (antd readonly.tsx).",
		roCol)

	// ---------- placement.tsx ----------
	top := track(kit.NewMentions(""))
	top.SetPlacement(kit.MentionsTop)
	top.SetOptions(defaultOpts)
	top.SetFixedWidth(360)
	secPlace := demoSection(face, th, "Placement top",
		"Suggestion panel opens above the field (antd placement.tsx).",
		top.Node())

	page := demoPage(face, "Mentions",
		"Mention people or tags in text. P0 aligns docs/antd/mentions.md §6 — basic, size, variant, async, form, prefix, readonly, placement.",
		secBasic, secSize, secVariant, secAsync, secForm, secPrefix, secRO, secPlace,
	)
	c.addPage("mentions", "Mentions", page)
}
