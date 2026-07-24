//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerAutoComplete() {
	// AutoComplete — antd demos §6.8 P0:
	// basic / options / custom / non-case-sensitive /
	// certain-category / uncertain-category / status / variant
	// https://ant.design/components/auto-complete
	//
	// P1 not shown: allowClear custom icon, style-class semantic, virtual, backfill.

	face, th := c.face, c.theme
	status := c.status

	track := func(ac *kit.AutoComplete) *kit.AutoComplete {
		if ac == nil {
			return nil
		}
		ac.SetFace(face)
		if th != nil {
			ac.SetTheme(th)
		}
		*c.tickers = append(*c.tickers, ac)
		return ac
	}

	// ---------- basic.tsx ----------
	basicUncontrolled := track(kit.NewAutoComplete("input here"))
	basicUncontrolled.SetFixedWidth(200)
	basicUncontrolled.SetFilterOption(false)
	basicUncontrolled.SetOnSearch(func(text string) {
		if text == "" {
			basicUncontrolled.SetOptions(nil)
			return
		}
		basicUncontrolled.SetOptions([]kit.AutoCompleteOption{
			{Value: text}, {Value: text + text}, {Value: text + text + text},
		})
	})
	basicUncontrolled.SetOnChange(func(text string) {
		if text == "" {
			basicUncontrolled.SetOptions(nil)
			return
		}
		basicUncontrolled.SetOptions([]kit.AutoCompleteOption{
			{Value: text}, {Value: text + text}, {Value: text + text + text},
		})
	})
	basicUncontrolled.SetOnSelect(func(v string, _ kit.AutoCompleteOption) {
		*status = "basic select=" + v
	})

	basicControlled := track(kit.NewAutoComplete("control mode"))
	basicControlled.SetFixedWidth(200)
	basicControlled.SetControlled(true)
	basicControlled.SetFilterOption(false)
	basicControlled.SetOnChange(func(text string) {
		basicControlled.SetValue(text)
		if text == "" {
			basicControlled.SetOptions(nil)
			return
		}
		basicControlled.SetOptions([]kit.AutoCompleteOption{
			{Value: text}, {Value: text + text}, {Value: text + text + text},
		})
	})
	basicControlled.SetOnSearch(func(text string) {
		*status = "basic search=" + text
	})
	basicCol := primitive.Column(basicUncontrolled.Node(), basicControlled.Node())
	basicCol.Gap = 16
	basicCol.CrossAlign = core.CrossStart
	secBasic := demoSection(face, th, "Basic Usage",
		"Basic Usage, set data source of autocomplete with options / onSearch.",
		basicCol)

	// ---------- options.tsx ----------
	emailAC := track(kit.NewAutoComplete("input here"))
	emailAC.SetFixedWidth(200)
	emailAC.SetFilterOption(false)
	fillEmail := func(value string) {
		if value == "" || strings.Contains(value, "@") {
			emailAC.SetOptions(nil)
			return
		}
		domains := []string{"gmail.com", "163.com", "qq.com"}
		opts := make([]kit.AutoCompleteOption, len(domains))
		for i, d := range domains {
			v := value + "@" + d
			opts[i] = kit.AutoCompleteOption{Value: v, Label: v}
		}
		emailAC.SetOptions(opts)
	}
	emailAC.SetOnSearch(fillEmail)
	emailAC.SetOnChange(fillEmail)
	secOptions := demoSection(face, th, "Customized",
		"Could customize option label and value by building options from onSearch.",
		emailAC.Node())

	// ---------- custom.tsx (TextArea children) ----------
	customAC := track(kit.NewAutoComplete(""))
	customAC.SetFixedWidth(200)
	ta := kit.NewTextArea("input here", 2)
	ta.SetFace(face)
	customAC.SetChildren(ta)
	customAC.SetFilterOption(false)
	fillCustom := func(v string) {
		if v == "" {
			customAC.SetOptions(nil)
			return
		}
		customAC.SetOptions([]kit.AutoCompleteOption{
			{Value: v}, {Value: v + v}, {Value: v + v + v},
		})
	}
	customAC.SetOnSearch(fillCustom)
	customAC.SetOnChange(fillCustom)
	secCustom := demoSection(face, th, "Customize Input Component",
		"Customize Input Component with TextArea children.",
		customAC.Node())

	// ---------- non-case-sensitive.tsx ----------
	caseAC := track(kit.NewAutoComplete("try to type `b`",
		"Burns Bay Road", "Downing Street", "Wall Street"))
	caseAC.SetFixedWidth(200)
	caseAC.SetFilterOptionFunc(func(input string, option kit.AutoCompleteOption) bool {
		return strings.Contains(strings.ToUpper(option.Value), strings.ToUpper(input))
	})
	secCase := demoSection(face, th, "Non-case-sensitive AutoComplete",
		"A non-case-sensitive AutoComplete.",
		caseAC.Node())

	// ---------- certain-category.tsx ----------
	certainAC := track(kit.NewAutoComplete(""))
	certainAC.SetFixedWidth(250)
	certainAC.SetFilterOption(false)
	certainSearch := kit.NewSearch("input here")
	certainSearch.SetFace(face)
	certainSearch.SetSize(kit.InputLarge)
	certainAC.SetChildren(certainSearch)
	certainAC.SetOptions([]kit.AutoCompleteOption{
		{
			Label: "Libraries",
			Options: []kit.AutoCompleteOption{
				{Value: "AntDesign", Label: "AntDesign", Extra: "10000"},
				{Value: "AntDesign UI", Label: "AntDesign UI", Extra: "10600"},
			},
		},
		{
			Label: "Solutions",
			Options: []kit.AutoCompleteOption{
				{Value: "AntDesign UI FAQ", Label: "AntDesign UI FAQ", Extra: "60100"},
				{Value: "AntDesign FAQ", Label: "AntDesign FAQ", Extra: "30010"},
			},
		},
		{
			Label: "Articles",
			Options: []kit.AutoCompleteOption{
				{Value: "AntDesign design language", Label: "AntDesign design language", Extra: "100000"},
			},
		},
	})
	certainAC.SetOnSelect(func(v string, _ kit.AutoCompleteOption) {
		*status = "category=" + v
	})
	secCertain := demoSection(face, th, "Lookup-Patterns - Certain Category",
		"Demonstration of Lookup Patterns with category groups + Input.Search.",
		certainAC.Node())

	// ---------- uncertain-category.tsx ----------
	uncertainAC := track(kit.NewAutoComplete(""))
	uncertainAC.SetFixedWidth(300)
	uncertainAC.SetFilterOption(false)
	uSearch := kit.NewSearch("input here")
	uSearch.SetFace(face)
	uSearch.SetSize(kit.InputLarge)
	uSearch.SetEnterButton(true)
	uncertainAC.SetChildren(uSearch)
	fillUncertain := func(q string) {
		if q == "" {
			uncertainAC.SetOptions(nil)
			return
		}
		opts := make([]kit.AutoCompleteOption, 0, 3)
		for i := 0; i < 3; i++ {
			cat := fmt.Sprintf("%s%d", q, i)
			opts = append(opts, kit.AutoCompleteOption{
				Value: cat,
				Label: fmt.Sprintf("Found %s on %s", q, cat),
				Extra: fmt.Sprintf("%d results", 100+i*17),
			})
		}
		uncertainAC.SetOptions(opts)
	}
	uncertainAC.SetOnSearch(fillUncertain)
	uncertainAC.SetOnChange(fillUncertain)
	uncertainAC.SetOnSelect(func(v string, _ kit.AutoCompleteOption) {
		*status = "uncertain=" + v
	})
	secUncertain := demoSection(face, th, "Lookup-Patterns - Uncertain Category",
		"Demonstration of Lookup Patterns with dynamic result rows + enterButton Search.",
		uncertainAC.Node())

	// ---------- status.tsx ----------
	stErr := track(kit.NewAutoComplete(""))
	stErr.SetFixedWidth(200)
	stErr.SetStatus(kit.InputStatusError)
	stErr.SetFilterOption(false)
	stErr.SetOnChange(func(text string) {
		if text == "" {
			stErr.SetOptions(nil)
			return
		}
		stErr.SetOptions([]kit.AutoCompleteOption{{Value: text}, {Value: text + text}})
	})
	stWarn := track(kit.NewAutoComplete(""))
	stWarn.SetFixedWidth(200)
	stWarn.SetStatus(kit.InputStatusWarning)
	stWarn.SetFilterOption(false)
	stWarn.SetOnChange(func(text string) {
		if text == "" {
			stWarn.SetOptions(nil)
			return
		}
		stWarn.SetOptions([]kit.AutoCompleteOption{{Value: text}, {Value: text + text}})
	})
	stCol := primitive.Column(stErr.Node(), stWarn.Node())
	stCol.Gap = 12
	stCol.CrossAlign = core.CrossStart
	secStatus := demoSection(face, th, "Status",
		"Add status to AutoComplete with options: error and warning.",
		stCol)

	// ---------- variant.tsx ----------
	mkVar := func(v kit.InputVariant, ph string) core.Node {
		ac := track(kit.NewAutoComplete(ph))
		ac.SetFixedWidth(200)
		ac.SetVariant(v)
		ac.SetFilterOption(false)
		ac.SetOnChange(func(text string) {
			if text == "" {
				ac.SetOptions(nil)
				return
			}
			ac.SetOptions([]kit.AutoCompleteOption{
				{Value: text}, {Value: text + text}, {Value: text + text + text},
			})
		})
		return ac.Node()
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
		"Variants of AutoComplete, there are four variants: outlined, filled, borderless and underlined.",
		varCol)

	// Page
	page := primitive.Column(
		secBasic, secOptions, secCustom, secCase,
		secCertain, secUncertain, secStatus, secVariant,
	)
	page.Gap = 16
	page.CrossAlign = core.CrossStretch
	page.Padding = primitive.All(12)
	c.addPage("auto_complete", "AutoComplete", page)
}
