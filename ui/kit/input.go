package kit

import (
	"math"
	"strings"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Input defaults — docs/antd/input.md §6.2 / §6.10
// https://ant.design/components/input
const (
	// DefaultInputPaddingInline is paddingSM−lineWidth ≈ 11 (Token fallback).
	DefaultInputPaddingInline = 11.0
	// DefaultInputLineHeight is Ant line-height ≈ 1.5714285714 (22/14).
	DefaultInputLineHeight = 1.5714285714
	// DefaultInputAffixGap is gap between prefix/editor/suffix (~6).
	DefaultInputAffixGap = 6.0
	// DefaultInputFocusRingOutset is visible focus ring outset (§6.2).
	DefaultInputFocusRingOutset = 1.5
	// DefaultTextAreaRows is used when rows < 2.
	DefaultTextAreaRows = 3
)

// Input is a single-line text field composed from Decorated + Flex + EditableText.
//
//	Decorated
//	  └─ Flex(Row) CrossStretch
//	       prefix? · Flexible(EditableText) · clear? · suffix? · search/eye?
//
// Product contract: docs/antd/input.md §6 (P0 DoD).
// Subtypes: Search (NewSearch), Password (NewPassword), TextArea (NewTextArea).
type Input struct {
	Root   *primitive.Decorated
	editor *primitive.EditableText
	row    *primitive.Flex
	prefix *primitive.Slot
	suffix *primitive.Slot

	// Product fields (§6.10).
	Value        string
	DefaultValue string
	Placeholder  string
	Size         InputSize
	Variant      InputVariant
	Status       InputStatus
	Type         InputType
	Disabled     bool
	ReadOnly     bool
	AllowClear   bool
	MaxLength    int
	AriaLabel    string
	Face         text.Face
	Theme        *core.Theme
	Style        Style
	OnChange     func(string)
	OnPressEnter func(string)
	OnClear      func()
	// OnSubmit is an alias for OnPressEnter (legacy smoke tests).
	OnSubmit func(string)

	// Controlled: typing only raises OnChange; display stays until SetValue.
	Controlled bool

	// Affix nodes (P0 for size demo / Search composition).
	prefixNode core.Node
	suffixNode core.Node

	// Internal interaction.
	focused bool
	hovered bool

	// Clear button (allowClear).
	clearBtn  *primitive.Pressable
	clearSlot *primitive.Slot

	// Password mode (also via Type=password or NewPassword).
	passwordMode     bool
	passwordVisible  bool
	visibilityToggle bool
	eyeBtn           *primitive.Pressable
	// Search mode (NewSearch).
	searchMode      bool
	enterButton     bool
	enterButtonText string
	loading         bool
	OnSearch        func(value string, source SearchSource)
	searchBtn       *primitive.Pressable
	searchEnterBtn  *Button
	spinner         *primitive.Canvas
	spinPhase       float64
	boundTree       *core.Tree

	// TextArea mode.
	multiline bool
	rows      int
	autoSize  bool
	minRows   int
	maxRows   int

	// Outer host when Search has enterButton (field + button row).
	host *primitive.Flex

	// Fixed outer size overrides (0 = unset).
	fixedW, fixedH float64

	// underLine is a 1px bottom bar for underlined variant.
	underLine *primitive.Decorated
}

// NewInput creates a single-line Input with optional placeholder.
// Defaults (§6.10): Size=middle, Variant=outlined, Status=none, Type=text, flags false.
func NewInput(placeholder string) *Input {
	in := &Input{
		Placeholder:      placeholder,
		Size:             InputMiddle,
		Variant:          InputOutlined,
		Status:           InputStatusNone,
		Type:             InputTypeText,
		visibilityToggle: true,
	}
	in.rebuild()
	return in
}

// NewInputWithDefault creates an Input seeded with defaultValue (uncontrolled).
func NewInputWithDefault(placeholder, defaultValue string) *Input {
	in := NewInput(placeholder)
	in.DefaultValue = defaultValue
	in.Value = defaultValue
	if in.editor != nil {
		in.applyEditorValue(defaultValue)
	}
	return in
}

// Node returns the root core.Node for tree attachment.
func (in *Input) Node() core.Node {
	if in == nil {
		return nil
	}
	if in.Root == nil {
		in.rebuild()
	}
	if in.host != nil {
		return in.host
	}
	return in.Root
}

// ChromeNode returns the Decorated field chrome (for tests / composition).
func (in *Input) ChromeNode() core.Node {
	if in == nil {
		return nil
	}
	if in.Root == nil {
		in.rebuild()
	}
	return in.Root
}

// Editor returns the inner EditableText (caret / IME / headless inject).
func (in *Input) Editor() *primitive.EditableText {
	if in == nil {
		return nil
	}
	if in.editor == nil {
		in.rebuild()
	}
	return in.editor
}

// IsFocused reports whether the inner editor is focused.
func (in *Input) IsFocused() bool {
	return in != nil && in.focused
}

// IsLoading reports Search loading state.
func (in *Input) IsLoading() bool { return in != nil && in.loading }

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue sets the displayed value. Fires OnChange when the value changes
// (legacy kit + form BindInput path). Typing still goes through editor → OnChange.
// Controlled mode: display follows SetValue; editor mutations do not write back.
func (in *Input) SetValue(v string) {
	if in == nil {
		return
	}
	prev := in.Value
	in.Value = v
	in.applyEditorValue(v)
	if prev != v && in.OnChange != nil {
		in.OnChange(v)
	}
	in.syncAffixVisibility()
	if in.autoSize {
		in.applyAutoSizeHeight()
	}
}

// GetValue returns the current value (product field).
func (in *Input) GetValue() string {
	if in == nil {
		return ""
	}
	return in.Value
}

// ValueString is an alias for GetValue (avoids clashing with the Value field).
func (in *Input) ValueString() string { return in.GetValue() }

// SetDefaultValue sets the uncontrolled seed. Applies only when Value is still empty
// and the field has not been controlled.
func (in *Input) SetDefaultValue(v string) {
	if in == nil {
		return
	}
	in.DefaultValue = v
	if !in.Controlled && in.Value == "" {
		in.Value = v
		in.applyEditorValue(v)
	}
}

// SetPlaceholder updates the placeholder text.
func (in *Input) SetPlaceholder(s string) {
	if in == nil {
		return
	}
	in.Placeholder = s
	if in.editor != nil {
		in.editor.Placeholder = s
		in.editor.MarkNeedsPaint()
	}
}

// SetSize updates control size (small/middle/large → h 24/32/40).
func (in *Input) SetSize(s InputSize) {
	if in == nil || in.Size == s {
		return
	}
	in.Size = s
	in.rebuild()
}

// SetVariant updates visual variant.
func (in *Input) SetVariant(v InputVariant) {
	if in == nil || in.Variant == v {
		return
	}
	in.Variant = v
	in.applyChrome()
}

// SetStatus updates validation chrome (error/warning/none).
func (in *Input) SetStatus(s InputStatus) {
	if in == nil || in.Status == s {
		return
	}
	in.Status = s
	in.applyChrome()
	in.applyA11y()
}

// SetType sets text/password. Prefer NewPassword for full visibility toggle.
func (in *Input) SetType(t InputType) {
	if in == nil || in.Type == t {
		return
	}
	in.Type = t
	in.passwordMode = t == InputTypePassword
	in.rebuild()
}

// SetDisabled toggles disabled (no edit, no onChange from input).
func (in *Input) SetDisabled(d bool) {
	if in == nil {
		return
	}
	in.Disabled = d
	if in.editor != nil {
		in.editor.Disabled = d
	}
	if in.clearBtn != nil {
		in.clearBtn.SetDisabled(d)
	}
	if in.eyeBtn != nil {
		in.eyeBtn.SetDisabled(d)
	}
	if in.searchBtn != nil {
		in.searchBtn.SetDisabled(d || in.loading)
	}
	if in.searchEnterBtn != nil {
		in.searchEnterBtn.SetDisabled(d || in.loading)
	}
	in.applyChrome()
	in.applyA11y()
}

// SetReadOnly toggles read-only (focusable, not editable).
func (in *Input) SetReadOnly(r bool) {
	if in == nil {
		return
	}
	in.ReadOnly = r
	if in.editor != nil {
		in.editor.ReadOnly = r
	}
}

// SetMaxLength sets max rune length (0 = unlimited).
func (in *Input) SetMaxLength(n int) {
	if in == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	in.MaxLength = n
	if in.editor != nil {
		in.editor.MaxLength = n
	}
}

// SetAllowClear toggles the clear affix.
func (in *Input) SetAllowClear(v bool) {
	if in == nil || in.AllowClear == v {
		return
	}
	in.AllowClear = v
	in.rebuild()
}

// SetPrefix sets a leading node (icon/text). nil clears.
func (in *Input) SetPrefix(n core.Node) {
	if in == nil {
		return
	}
	in.prefixNode = n
	if in.prefix != nil {
		in.prefix.SetChild(n)
	}
}

// SetSuffix sets a trailing node. nil clears. Overridden visually by clear/search/eye when active.
func (in *Input) SetSuffix(n core.Node) {
	if in == nil {
		return
	}
	in.suffixNode = n
	if in.suffix != nil {
		in.suffix.SetChild(n)
	}
}

// SetOnChange sets the change callback.
func (in *Input) SetOnChange(fn func(string)) {
	if in == nil {
		return
	}
	in.OnChange = fn
}

// SetOnPressEnter sets the Enter callback (single-line).
func (in *Input) SetOnPressEnter(fn func(string)) {
	if in == nil {
		return
	}
	in.OnPressEnter = fn
	in.OnSubmit = fn
}

// SetOnSubmit is an alias for SetOnPressEnter (legacy).
func (in *Input) SetOnSubmit(fn func(string)) { in.SetOnPressEnter(fn) }

// SetOnClear sets the clear-button callback.
func (in *Input) SetOnClear(fn func()) {
	if in == nil {
		return
	}
	in.OnClear = fn
}

// SetControlled toggles controlled mode (INP-02 / INP-S1).
func (in *Input) SetControlled(v bool) {
	if in == nil {
		return
	}
	in.Controlled = v
}

// SetTheme sets an explicit theme override.
func (in *Input) SetTheme(th *core.Theme) {
	if in == nil {
		return
	}
	in.Theme = th
	in.rebuild()
}

// SetFace sets the font face.
func (in *Input) SetFace(face text.Face) {
	if in == nil {
		return
	}
	in.Face = face
	in.Style.Face = face
	if in.editor != nil {
		in.editor.Face = face
	}
}

// SetStyle applies visual overrides.
func (in *Input) SetStyle(st Style) {
	if in == nil {
		return
	}
	in.Style = st
	if st.Face != nil {
		in.SetFace(st.Face)
	}
	if st.FontSize > 0 || st.Height > 0 || st.Width > 0 {
		in.rebuild()
		return
	}
	in.applyChrome()
}

// SetBackground overrides fill (Style helper).
func (in *Input) SetBackground(c render.RGBA) {
	if in == nil {
		return
	}
	in.Style.Background = c
	in.applyChrome()
}

// SetTextColor overrides editor text color.
func (in *Input) SetTextColor(c render.RGBA) {
	if in == nil {
		return
	}
	in.Style.Text = c
	in.applyChrome()
}

// SetFontSize overrides editor font size.
func (in *Input) SetFontSize(px float64) {
	if in == nil {
		return
	}
	in.Style.FontSize = px
	in.rebuild()
}

// SetFixedSize forces outer chrome size (forms / gallery).
// Pass 0 for an axis to leave that axis on size/token metrics.
func (in *Input) SetFixedSize(w, h float64) {
	if in == nil {
		return
	}
	in.fixedW, in.fixedH = w, h
	if in.Root == nil {
		in.rebuild()
		return
	}
	if w > 0 {
		in.Root.Width = w
		in.Root.MinWidth = w
	}
	if h > 0 && !in.multiline {
		in.Root.Height = h
		in.Root.MinHeight = h
	}
	in.Root.MarkNeedsLayout()
}

// SetAriaLabel sets the accessible name.
func (in *Input) SetAriaLabel(s string) {
	if in == nil {
		return
	}
	in.AriaLabel = s
	in.applyA11y()
}

// AttachTicker registers caret blink and Search loading spinner.
func (in *Input) AttachTicker(t *core.Tree) {
	if in == nil || t == nil {
		return
	}
	in.boundTree = t
	if in.editor != nil {
		in.editor.AttachTicker(t)
	}
	t.BindTicker(in, in.loading)
}

// Tick advances the Search loading spinner. Implements core.Ticker when Loading.
func (in *Input) Tick(dt float64) bool {
	if in == nil || !in.loading {
		return false
	}
	in.spinPhase += dt * 1.4
	if in.spinPhase > 1 {
		in.spinPhase -= 1
	}
	if in.spinner != nil {
		in.spinner.MarkNeedsPaint()
	} else if in.Root != nil {
		in.Root.MarkNeedsPaint()
	}
	return in.loading
}

// ---------------------------------------------------------------------------
// Search / Password / TextArea configuration (shared on Input)
// ---------------------------------------------------------------------------

// SetLoading toggles Search loading (spinner via Ticker).
func (in *Input) SetLoading(v bool) {
	if in == nil || in.loading == v {
		return
	}
	in.loading = v
	in.rebuild()
	if in.boundTree != nil {
		if v {
			in.boundTree.AddTicker(in)
		} else {
			in.boundTree.RemoveTicker(in)
		}
	}
}

// SetEnterButton enables the confirm search button (antd enterButton).
func (in *Input) SetEnterButton(v bool) {
	if in == nil || in.enterButton == v {
		return
	}
	in.enterButton = v
	if v {
		in.searchMode = true
	}
	in.rebuild()
}

// SetEnterButtonText sets enterButton label (non-empty implies enterButton).
func (in *Input) SetEnterButtonText(s string) {
	if in == nil {
		return
	}
	in.enterButtonText = s
	if s != "" {
		in.enterButton = true
		in.searchMode = true
	}
	in.rebuild()
}

// SetOnSearch sets the Search callback.
func (in *Input) SetOnSearch(fn func(value string, source SearchSource)) {
	if in == nil {
		return
	}
	in.OnSearch = fn
	in.searchMode = true
}

// SetVisibilityToggle enables the Password eye control (default true).
func (in *Input) SetVisibilityToggle(v bool) {
	if in == nil || in.visibilityToggle == v {
		return
	}
	in.visibilityToggle = v
	if in.passwordMode {
		in.rebuild()
	}
}

// SetPasswordVisible toggles mask visibility (Password).
func (in *Input) SetPasswordVisible(v bool) {
	if in == nil || in.passwordVisible == v {
		return
	}
	in.passwordVisible = v
	in.applyPasswordMask()
	in.syncAffixVisibility()
}

// IsPasswordVisible reports whether the password is shown in clear text.
func (in *Input) IsPasswordVisible() bool {
	return in != nil && in.passwordVisible
}

// SetRows sets TextArea fixed row count (ignored when autoSize).
func (in *Input) SetRows(n int) {
	if in == nil {
		return
	}
	if n < 1 {
		n = DefaultTextAreaRows
	}
	in.rows = n
	in.multiline = true
	in.rebuild()
}

// SetAutoSize enables free auto-grow (min 1 row).
func (in *Input) SetAutoSize(v bool) {
	if in == nil {
		return
	}
	in.autoSize = v
	in.multiline = true
	if v && in.minRows < 1 {
		in.minRows = 1
	}
	in.rebuild()
}

// SetAutoSizeRange enables autoSize with min/max rows (antd autoSize object).
func (in *Input) SetAutoSizeRange(minRows, maxRows int) {
	if in == nil {
		return
	}
	if minRows < 1 {
		minRows = 1
	}
	if maxRows > 0 && maxRows < minRows {
		maxRows = minRows
	}
	in.autoSize = true
	in.minRows = minRows
	in.maxRows = maxRows
	in.multiline = true
	in.rebuild()
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (in *Input) theme() *core.Theme {
	var n core.Node
	if in.Root != nil {
		n = in.Root
	}
	return themeOf(in.Theme, n)
}

func (in *Input) metrics() (padH, padV, height, fontSize, radius, lineW, gap float64) {
	th := in.theme()
	fontSize = th.SizeOr(core.TokenFontSize, 14)
	radius = th.SizeOr(core.TokenBorderRadius, 6)
	lineW = th.SizeOr(core.TokenLineWidth, 1)
	gap = DefaultInputAffixGap
	padH = th.SizeOr(core.TokenControlPaddingInline, DefaultInputPaddingInline)
	padV = 0
	switch in.Size {
	case InputSmall:
		height = th.SizeOr(core.TokenControlHeightSM, 24)
		fontSize = th.SizeOr(core.TokenFontSizeSM, 12)
		padH = th.SizeOr(core.TokenControlPaddingInlineSM, 7)
		radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
	case InputLarge:
		height = th.SizeOr(core.TokenControlHeightLG, 40)
		fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
		padH = th.SizeOr(core.TokenControlPaddingInlineLG, 11)
		radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		height = th.SizeOr(core.TokenControlHeight, 32)
	}
	if in.Style.FontSize > 0 {
		fontSize = in.Style.FontSize
	}
	if in.Style.Height > 0 {
		height = in.Style.Height
	}
	if in.Style.hasRadius() {
		radius = in.Style.Radius
	}
	if in.multiline {
		// TextArea: vertical padding ≈ 4 (paddingXS).
		padV = th.SizeOr(core.TokenPaddingXS, 4)
		if !in.autoSize {
			rows := in.rows
			if rows < 1 {
				rows = DefaultTextAreaRows
			}
			height = fontSize*DefaultInputLineHeight*float64(rows) + padV*2
		}
	}
	return padH, padV, height, fontSize, radius, lineW, gap
}

func (in *Input) rebuild() {
	if in == nil {
		return
	}
	th := in.theme()
	padH, padV, height, fontSize, radius, lineW, gap := in.metrics()

	// Editor
	if in.editor == nil {
		in.editor = primitive.NewEditableText()
	}
	in.editor.Placeholder = in.Placeholder
	in.editor.Disabled = in.Disabled
	in.editor.ReadOnly = in.ReadOnly
	in.editor.MaxLength = in.MaxLength
	in.editor.Face = in.Face
	in.editor.FontSize = fontSize
	in.editor.ShowFocusRing = false // outer Decorated is focus chrome
	in.editor.Multiline = in.multiline
	in.editor.Color = th.Color(core.TokenColorText)
	in.editor.PlaceholderColor = th.Color(core.TokenColorTextSecondary)
	if in.editor.PlaceholderColor.A < 0.15 {
		in.editor.PlaceholderColor = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	// Seed value without OnChange.
	in.applyEditorValue(in.Value)
	in.applyPasswordMask()

	in.editor.OnChange = in.handleEditorChange
	in.editor.OnSubmit = func(v string) {
		if in.Disabled || in.ReadOnly {
			return
		}
		if in.searchMode && in.OnSearch != nil {
			in.OnSearch(v, SearchFromInput)
		}
		fn := in.OnPressEnter
		if fn == nil {
			fn = in.OnSubmit
		}
		if fn != nil {
			fn(v)
		}
	}
	in.editor.OnFocusChange = func(f bool) {
		in.focused = f
		in.applyChrome()
	}
	in.editor.OnHoverChange = func(h bool) {
		in.hovered = h
		in.applyChrome()
	}

	// Row: prefix · editor · clear · suffix · eye/search/spinner
	flexEd := primitive.NewFlexible(1, in.editor)
	flexEd.FillChild = true

	in.prefix = primitive.NewSlot("prefix", in.prefixNode)
	in.suffix = primitive.NewSlot("suffix", in.suffixNode)

	kids := []core.Node{in.prefix, flexEd}

	// allowClear — Slot collapses to 0 when empty (hidden).
	in.clearBtn = nil
	in.clearSlot = nil
	if in.AllowClear {
		in.clearBtn = in.makeIconPressable("close", func() {
			if in.Disabled || in.ReadOnly {
				return
			}
			prev := in.Value
			if in.Controlled {
				if in.OnChange != nil {
					in.OnChange("")
				}
			} else {
				in.Value = ""
				in.applyEditorValue("")
				if in.OnChange != nil {
					in.OnChange("")
				}
			}
			if in.OnClear != nil {
				in.OnClear()
			}
			if in.searchMode && in.OnSearch != nil && prev != "" {
				in.OnSearch("", SearchFromClear)
			}
			in.syncAffixVisibility()
			if in.autoSize {
				in.applyAutoSizeHeight()
			}
		})
		in.clearSlot = primitive.NewSlot("clear", nil)
		kids = append(kids, in.clearSlot)
	}

	kids = append(kids, in.suffix)

	// Password eye
	in.eyeBtn = nil
	if in.passwordMode && in.visibilityToggle {
		in.eyeBtn = in.makeIconPressable("eye", func() {
			if in.Disabled {
				return
			}
			in.SetPasswordVisible(!in.passwordVisible)
		})
		kids = append(kids, in.eyeBtn)
	}

	// Search icon (when !enterButton)
	in.searchBtn = nil
	in.spinner = nil
	if in.searchMode && !in.enterButton {
		if in.loading {
			spinSize := fontSize
			if spinSize < 12 {
				spinSize = 12
			}
			in.spinner = primitive.NewCanvas(spinSize, spinSize, in.paintSpinner)
			kids = append(kids, in.spinner)
		} else {
			in.searchBtn = in.makeIconPressable("search", func() {
				if in.Disabled || in.loading {
					return
				}
				if in.OnSearch != nil {
					in.OnSearch(in.Value, SearchFromButton)
				}
			})
			kids = append(kids, in.searchBtn)
		}
	}

	in.row = primitive.Row(kids...)
	in.row.Gap = gap
	in.row.CrossAlign = core.CrossStretch

	// Underline bar (underlined variant)
	in.underLine = primitive.NewDecorated()
	in.underLine.Height = lineW
	in.underLine.MinHeight = lineW
	in.underLine.ExpandWidth = true
	in.underLine.Background = th.Color(core.TokenColorBorder)
	in.underLine.Hit = core.HitTransparent

	body := core.Node(in.row)
	if in.Variant == InputUnderlined {
		col := primitive.Column(in.row, in.underLine)
		col.Gap = 0
		col.CrossAlign = core.CrossStretch
		body = col
	}

	if in.Root == nil {
		in.Root = primitive.NewDecorated(body)
	} else {
		in.Root.ClearChildren()
		in.Root.AddChild(body)
	}
	in.Root.Padding = primitive.Symmetric(padH, padV)
	in.Root.Radius = radius
	in.Root.BorderWidth = lineW
	in.Root.Hit = core.HitDefer
	if in.multiline {
		in.Root.SetCenterContent(false)
		if in.autoSize {
			in.applyAutoSizeHeight()
		} else {
			in.Root.MinHeight = height
			in.Root.Height = height
		}
	} else {
		in.Root.SetCenterContent(true)
		in.Root.MinHeight = height
		in.Root.Height = height
	}
	if in.fixedW > 0 {
		in.Root.Width = in.fixedW
		in.Root.MinWidth = in.fixedW
	}
	if in.fixedH > 0 && !in.multiline {
		in.Root.Height = in.fixedH
		in.Root.MinHeight = in.fixedH
	}
	in.Root.SetThemeHook(func(*core.Theme) { in.rebuild() })
	in.applyA11y()

	// Search enterButton → outer Flex(field, button)
	in.searchEnterBtn = nil
	in.host = nil
	if in.searchMode && in.enterButton {
		label := in.enterButtonText
		if label == "" {
			label = "Search"
		}
		btn := NewButton(label)
		if in.enterButtonText == "" {
			// Icon-only search when no custom text: use "Search" label with icon.
			btn.SetIcon("search")
			if in.enterButtonText == "" && label == "Search" {
				// antd enterButton={true} → primary search button with icon
				btn.SetLabel("")
				btn.SetIcon("search")
				btn.SetAriaLabel("Search")
			}
		}
		btn.SetType(ButtonPrimary)
		btn.SetSize(in.Size.ToButtonSize())
		btn.SetDisabled(in.Disabled || in.loading)
		btn.SetLoading(in.loading)
		btn.SetOnClick(func() {
			if in.Disabled || in.loading {
				return
			}
			if in.OnSearch != nil {
				in.OnSearch(in.Value, SearchFromButton)
			}
		})
		if in.Face != nil {
			btn.SetFace(in.Face)
		}
		if in.Theme != nil {
			btn.Theme = in.Theme
		}
		in.searchEnterBtn = btn
		host := primitive.Row(in.Root, btn.Node())
		host.Gap = -lineW // compact join
		host.CrossAlign = core.CrossCenter
		in.host = host
	}

	in.syncAffixVisibility()
	in.applyChrome()
	if in.boundTree != nil && in.editor != nil {
		in.editor.AttachTicker(in.boundTree)
	}
	in.Root.MarkNeedsLayout()
	in.Root.MarkNeedsPaint()
}

func (in *Input) makeIconPressable(iconName string, click func()) *primitive.Pressable {
	th := in.theme()
	_, _, _, fontSize, _, _, _ := in.metrics()
	if iconName == "eye" {
		// No built-in eye glyph — use a clear-text marker for the visibility toggle.
		lab := primitive.NewText("👁")
		lab.FontSize = fontSize
		lab.Color = th.Color(core.TokenColorTextSecondary)
		p := primitive.NewPressable(lab)
		p.Focusable = false
		p.ShowFocusRing = false
		p.Click = click
		return p
	}
	ic := primitive.NewIcon(iconName)
	ic.Size = fontSize
	ic.Color = th.Color(core.TokenColorTextSecondary)
	p := primitive.NewPressable(ic)
	p.Focusable = false
	p.ShowFocusRing = false
	p.Click = click
	return p
}

func (in *Input) handleEditorChange(v string) {
	if in.Disabled {
		return
	}
	if in.Controlled {
		if in.OnChange != nil {
			in.OnChange(v)
		}
		// Do not write back — restore display from controlled Value.
		in.applyEditorValue(in.Value)
		return
	}
	in.Value = v
	if in.OnChange != nil {
		in.OnChange(v)
	}
	in.syncAffixVisibility()
	if in.autoSize {
		in.applyAutoSizeHeight()
	}
}

func (in *Input) applyEditorValue(v string) {
	if in.editor == nil {
		return
	}
	if in.editor.Value == v {
		return
	}
	in.editor.Value = v
	n := utf8.RuneCountInString(v)
	in.editor.Cursor = n
	in.editor.SelAnchor = n
	in.editor.MarkNeedsPaint()
}

func (in *Input) applyPasswordMask() {
	if in.editor == nil {
		return
	}
	mask := in.passwordMode && !in.passwordVisible
	in.editor.Password = mask
	in.editor.MarkNeedsPaint()
}

func (in *Input) syncAffixVisibility() {
	// Hide clear when empty or disabled (Slot nil child → size 0).
	if in.clearSlot != nil {
		show := in.AllowClear && in.Value != "" && !in.Disabled
		if show {
			in.clearSlot.SetChild(in.clearBtn)
		} else {
			in.clearSlot.SetChild(nil)
		}
	}
	if in.eyeBtn != nil {
		in.eyeBtn.MarkNeedsPaint()
	}
}

func (in *Input) applyAutoSizeHeight() {
	if in == nil || in.Root == nil || !in.autoSize {
		return
	}
	_, padV, _, fontSize, _, _, _ := in.metrics()
	lineH := fontSize * DefaultInputLineHeight
	lines := 1
	if in.Value != "" {
		lines = 1 + strings.Count(in.Value, "\n")
	}
	minR := in.minRows
	if minR < 1 {
		minR = 1
	}
	if lines < minR {
		lines = minR
	}
	if in.maxRows > 0 && lines > in.maxRows {
		lines = in.maxRows
	}
	h := lineH*float64(lines) + padV*2
	in.Root.Height = h
	in.Root.MinHeight = h
	if in.editor != nil {
		in.editor.Height = lineH * float64(lines)
	}
	in.Root.MarkNeedsLayout()
}

func (in *Input) applyA11y() {
	if in == nil || in.Root == nil {
		return
	}
	// Role/Label on editor (focus target). NodeBase has no aria-invalid field —
	// status=error is exposed via Label suffix for screen-reader-ish hosts.
	if in.editor != nil {
		in.editor.Base().Role = "textbox"
		name := in.AriaLabel
		if name == "" {
			name = in.Placeholder
		}
		if in.Status == InputStatusError {
			if name != "" {
				name = name + " (invalid)"
			} else {
				name = "invalid"
			}
		}
		in.editor.Base().Label = name
	}
}

func (in *Input) applyChrome() {
	if in == nil || in.Root == nil {
		return
	}
	th := in.theme()
	_, _, _, _, radius, lineW, _ := in.metrics()
	if in.Style.hasRadius() {
		radius = in.Style.Radius
	}
	in.Root.Radius = radius

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	bw := lineW

	switch in.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.01 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{} // transparent
		bw = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		bw = 0
	case InputUnderlined:
		bg = render.RGBA{}
		bd = render.RGBA{}
		bw = 0
		if in.underLine != nil {
			in.underLine.Background = th.Color(core.TokenColorBorder)
			in.underLine.Height = lineW
		}
	default: // outlined
		bg = th.Color(core.TokenColorBgContainer)
		bd = th.Color(core.TokenColorBorder)
		bw = lineW
	}

	if in.Style.hasBG() {
		bg = in.Style.Background
	}
	if in.Style.hasBorder() {
		bd = in.Style.Border
	}

	textCol := th.Color(core.TokenColorText)
	if in.Style.hasText() {
		textCol = in.Style.Text
	}

	switch {
	case in.Disabled:
		bg = th.Color(core.TokenColorDisabledBg)
		bd = th.Color(core.TokenColorBorder)
		if in.Variant == InputBorderless || in.Variant == InputUnderlined {
			// keep soft disabled fill
		}
		if in.Variant == InputOutlined || in.Variant == InputFilled {
			bw = lineW
			if in.Variant == InputFilled {
				bw = 0
			}
		}
		textCol = th.Color(core.TokenColorDisabledText)
		if in.underLine != nil && in.Variant == InputUnderlined {
			in.underLine.Background = th.Color(core.TokenColorBorder)
		}
	case in.Status == InputStatusError:
		bd = th.Color(core.TokenColorError)
		bw = lineW
		if in.Variant == InputUnderlined && in.underLine != nil {
			in.underLine.Background = bd
			bw = 0
		}
	case in.Status == InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
		bw = lineW
		if in.Variant == InputUnderlined && in.underLine != nil {
			in.underLine.Background = bd
			bw = 0
		}
	case in.focused:
		bd = th.Color(core.TokenColorPrimary)
		bw = lineW
		if in.Variant == InputBorderless {
			// still show focus affordance via ring; keep borderless fill
			bw = 0
		}
		if in.Variant == InputUnderlined && in.underLine != nil {
			in.underLine.Background = bd
			bw = 0
		}
		if in.Variant == InputFilled {
			// filled + focus: primary border
			bw = lineW
		}
	case in.hovered && !in.Disabled:
		hbd := th.Color(core.TokenColorBorderHover)
		if hbd.A < 0.5 {
			hbd = th.Color(core.TokenColorPrimaryHover)
		}
		if in.Variant == InputOutlined {
			bd = hbd
		}
		if in.Variant == InputFilled {
			// slightly stronger fill on hover
			bg = th.Color(core.TokenColorBgTextHover)
			if bg.A < 0.01 {
				bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
			}
		}
		if in.Variant == InputUnderlined && in.underLine != nil {
			in.underLine.Background = hbd
		}
	}

	in.Root.Background = bg
	in.Root.BorderColor = bd
	in.Root.BorderWidth = bw

	if in.editor != nil {
		in.editor.Color = textCol
		if in.Disabled {
			in.editor.Color = th.Color(core.TokenColorDisabledText)
		}
	}
	in.Root.MarkNeedsPaint()
}

func (in *Input) paintSpinner(pc *core.PaintContext, sz core.Size) {
	if pc == nil || !in.loading {
		return
	}
	th := in.theme()
	col := th.Color(core.TokenColorPrimary)
	if in.Disabled {
		col = th.Color(core.TokenColorDisabledText)
	}
	track := render.RGBA{R: col.R, G: col.G, B: col.B, A: col.A * 0.35}
	if track.A < 0.1 {
		track.A = 0.2
	}
	stroke := 2.0
	if sz.Width < 14 {
		stroke = 1.5
	}
	cx, cy := sz.Width/2, sz.Height/2
	r := sz.Width/2 - stroke
	if r < 1 {
		r = 1
	}
	pc.StrokeLocalCircle(cx, cy, r, stroke, track)
	start := -math.Pi/2 + in.spinPhase*2*math.Pi
	end := start + 2*math.Pi*0.7
	steps := 40
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		a := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(a), cy+r*math.Sin(a))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

// Search is Ant Design Input.Search.
type Search struct {
	*Input
}

// NewSearch creates an Input.Search with search icon / onSearch.
func NewSearch(placeholder string) *Search {
	in := NewInput(placeholder)
	in.searchMode = true
	in.rebuild()
	return &Search{Input: in}
}

// ---------------------------------------------------------------------------
// Password
// ---------------------------------------------------------------------------

// Password is Ant Design Input.Password.
type Password struct {
	*Input
}

// NewPassword creates a masked password field with visibility toggle.
func NewPassword(placeholder string) *Password {
	in := NewInput(placeholder)
	in.passwordMode = true
	in.Type = InputTypePassword
	in.visibilityToggle = true
	in.passwordVisible = false
	in.rebuild()
	return &Password{Input: in}
}

// ---------------------------------------------------------------------------
// TextArea
// ---------------------------------------------------------------------------

// TextArea is Ant Design Input.TextArea (multi-line).
type TextArea struct {
	*Input
	Rows int
}

// NewTextArea creates a multi-line field with the given row count.
// rows < 2 → DefaultTextAreaRows (3).
func NewTextArea(placeholder string, rows int) *TextArea {
	if rows < 2 {
		rows = DefaultTextAreaRows
	}
	in := NewInput(placeholder)
	in.multiline = true
	in.rows = rows
	in.rebuild()
	return &TextArea{Input: in, Rows: rows}
}

// SetRows updates fixed rows (disables autoSize bounds unless re-set).
func (ta *TextArea) SetRows(n int) {
	if ta == nil {
		return
	}
	ta.Rows = n
	ta.Input.SetRows(n)
}

// SetAutoSize enables auto-grow.
func (ta *TextArea) SetAutoSize(v bool) {
	if ta == nil {
		return
	}
	ta.Input.SetAutoSize(v)
}

// SetAutoSizeRange sets min/max rows for autoSize.
func (ta *TextArea) SetAutoSizeRange(minRows, maxRows int) {
	if ta == nil {
		return
	}
	ta.Input.SetAutoSizeRange(minRows, maxRows)
}
