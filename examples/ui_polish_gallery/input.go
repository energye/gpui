//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerInput() {
	// Input — antd demos §6.8 P0:
	// basic / size / variant / compact-style / search-input /
	// search-input-loading / textarea / autosize-textarea
	// https://ant.design/components/input
	//
	// P1 not shown: OTP, formatter tooltip, presuffix full page, password demo page,
	// showCount/count, semantic classNames/styles.

	face, th := c.face, c.theme
	track := func(in *kit.Input) *kit.Input {
		if in == nil {
			return nil
		}
		in.SetFace(face)
		if th != nil {
			in.SetTheme(th)
		}
		*c.tickers = append(*c.tickers, in)
		return in
	}
	fixIn := func(ph string, w float64) *kit.Input {
		in := track(kit.NewInput(ph))
		if w > 0 {
			in.SetFixedSize(w, 0)
		}
		return in
	}

	// ---------- basic.tsx ----------
	basic := fixIn("Basic usage", 240)
	secBasic := demoSection(face, th, "Basic usage",
		"Basic usage example.",
		basic.Node())

	// ---------- size.tsx ----------
	mkSize := func(s kit.InputSize, ph string) core.Node {
		in := track(kit.NewInput(ph))
		in.SetSize(s)
		in.SetPrefix(primitive.NewIcon("info"))
		in.SetFixedSize(280, 0)
		return in.Node()
	}
	sizeCol := primitive.Column(
		mkSize(kit.InputLarge, "large size"),
		mkSize(kit.InputMiddle, "default size"),
		mkSize(kit.InputSmall, "small size"),
	)
	sizeCol.Gap = 12
	sizeCol.CrossAlign = core.CrossStart
	secSize := demoSection(face, th, "Three sizes",
		"There are three sizes of an Input box: large (40px), middle (32px) and small (24px).",
		sizeCol)

	// ---------- variant.tsx ----------
	mkVar := func(v kit.InputVariant, ph string) core.Node {
		in := track(kit.NewInput(ph))
		in.SetVariant(v)
		in.SetFixedSize(280, 0)
		return in.Node()
	}
	varCol := primitive.Column(
		mkVar(kit.InputOutlined, "Outlined"),
		mkVar(kit.InputFilled, "Filled"),
		mkVar(kit.InputBorderless, "Borderless"),
		mkVar(kit.InputUnderlined, "Underlined"),
	)
	varCol.Gap = 12
	sFilled := track(kit.NewSearch("Filled").Input)
	sFilled.SetVariant(kit.InputFilled)
	sFilled.SetFixedSize(280, 0)
	varCol.AddChild(sFilled.Node())
	secVariant := demoSection(face, th, "Variants",
		"Variants of Input, including outlined, filled, borderless and underlined.",
		varCol)

	// ---------- compact-style.tsx ----------
	c1 := kit.NewInput("")
	c1.SetValue("26888888")
	track(c1)
	c2a := kit.NewInput("")
	c2a.SetValue("0571")
	track(c2a)
	c2b := kit.NewInput("")
	c2b.SetValue("26888888")
	track(c2b)
	compactCol := primitive.Column(
		kit.NewSpaceCompact(c1.Node()).Node(),
		kit.NewSpaceCompact(c2a.Node(), c2b.Node()).Node(),
	)
	// Search + addon
	searchCompact := kit.NewSearch("input search text")
	track(searchCompact.Input)
	searchCompact.SetAllowClear(true)
	addon := kit.NewSpaceAddon(kit.NewText("https://").Node())
	compactCol.AddChild(kit.NewSpaceCompact(addon.Node(), searchCompact.Node()).Node())
	// Input + Button
	combineIn := track(kit.NewInput(""))
	combineIn.SetValue("Combine input and button")
	submit := c.trackBtn(kit.NewButton("Submit"))
	submit.SetType(kit.ButtonPrimary)
	compactCol.AddChild(kit.NewSpaceCompact(combineIn.Node(), submit.Node()).Node())
	compactCol.Gap = 12
	compactCol.CrossAlign = core.CrossStretch
	secCompact := demoSection(face, th, "Compact mode",
		"Use Space.Compact for connected inputs (antd compact-style).",
		compactCol)

	// ---------- search-input.tsx ----------
	mkSearch := func(cfg func(*kit.Search)) core.Node {
		s := kit.NewSearch("input search text")
		track(s.Input)
		s.SetOnSearch(func(v string, src kit.SearchSource) {
			*c.status = fmt.Sprintf("search %s → %q", src.String(), v)
		})
		if cfg != nil {
			cfg(s)
		}
		s.SetFixedSize(280, 0)
		return s.Node()
	}
	searchCol := primitive.Column(
		mkSearch(nil),
		mkSearch(func(s *kit.Search) { s.SetAllowClear(true) }),
		mkSearch(func(s *kit.Search) { s.SetEnterButton(true) }),
		mkSearch(func(s *kit.Search) {
			s.SetAllowClear(true)
			s.SetEnterButtonText("Search")
			s.SetSize(kit.InputLarge)
			s.SetFixedSize(360, 0)
		}),
	)
	searchCol.Gap = 12
	secSearch := demoSection(face, th, "Search box",
		"Example of creating a search box by grouping a standard input with a search button.",
		searchCol)

	// ---------- search-input-loading.tsx ----------
	loadCol := primitive.Column(
		func() core.Node {
			s := kit.NewSearch("input search loading default")
			track(s.Input)
			s.SetLoading(true)
			s.SetFixedSize(280, 0)
			return s.Node()
		}(),
		func() core.Node {
			s := kit.NewSearch("input search loading with enterButton")
			track(s.Input)
			s.SetLoading(true)
			s.SetEnterButton(true)
			s.SetFixedSize(320, 0)
			return s.Node()
		}(),
		func() core.Node {
			s := kit.NewSearch("input search text")
			track(s.Input)
			s.SetEnterButtonText("Search")
			s.SetSize(kit.InputLarge)
			s.SetLoading(true)
			s.SetFixedSize(360, 0)
			return s.Node()
		}(),
	)
	loadCol.Gap = 12
	secLoad := demoSection(face, th, "Search box loading",
		"Search in loading state with Ticker-driven spinner.",
		loadCol)

	// ---------- textarea.tsx ----------
	ta1 := kit.NewTextArea("", 4)
	track(ta1.Input)
	ta1.SetFixedSize(360, 0)
	ta2 := kit.NewTextArea("maxLength is 6", 4)
	track(ta2.Input)
	ta2.SetMaxLength(6)
	ta2.SetFixedSize(360, 0)
	taCol := primitive.Column(ta1.Node(), ta2.Node())
	taCol.Gap = 12
	secTA := demoSection(face, th, "TextArea",
		"For multi-line input.",
		taCol)

	// ---------- autosize-textarea.tsx ----------
	auto1 := kit.NewTextArea("Autosize height based on content lines", 2)
	track(auto1.Input)
	auto1.SetAutoSize(true)
	auto1.SetFixedSize(360, 0)
	auto2 := kit.NewTextArea("Autosize height with min/max rows", 2)
	track(auto2.Input)
	auto2.SetAutoSizeRange(2, 6)
	auto2.SetFixedSize(360, 0)
	autoCol := primitive.Column(auto1.Node(), auto2.Node())
	autoCol.Gap = 24
	secAuto := demoSection(face, th, "Autosize TextArea",
		"Height grows with content between minRows and maxRows.",
		autoCol)

	// Extra P0 surfaces: status / allowClear / disabled (not separate official pages but §6.8)
	stErr := track(kit.NewInput("error"))
	stErr.SetStatus(kit.InputStatusError)
	stErr.SetFixedSize(200, 0)
	stWarn := track(kit.NewInput("warning"))
	stWarn.SetStatus(kit.InputStatusWarning)
	stWarn.SetFixedSize(200, 0)
	clr := track(kit.NewInput("allowClear"))
	clr.SetAllowClear(true)
	clr.SetValue("clear me")
	clr.SetFixedSize(200, 0)
	dis := track(kit.NewInput("disabled"))
	dis.SetDisabled(true)
	dis.SetValue("disabled")
	dis.SetFixedSize(200, 0)
	secStatus := demoSection(face, th, "Status / clear / disabled",
		"Validation status, allowClear and disabled chrome.",
		spaceWrap(8, stErr.Node(), stWarn.Node(), clr.Node(), dis.Node()))

	page := demoPage(face, "Input",
		"A basic widget for getting the user input is a text field. Keyboard and mouse can be used for providing or changing data.",
		secBasic, secSize, secVariant, secCompact, secSearch, secLoad, secTA, secAuto, secStatus,
	)
	c.addPage("input", "Input", page)
}
