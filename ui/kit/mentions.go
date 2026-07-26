package kit

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Mentions defaults — docs/antd/mentions.md §6.2 / §6.10
// https://ant.design/components/mentions
const (
	DefaultMentionsGap             = 4.0
	DefaultMentionsPanelPad        = 4.0
	DefaultMentionsItemPadInline   = 12.0
	DefaultMentionsItemPadBlock    = 5.0
	DefaultMentionsPanelRadius     = 8.0
	DefaultMentionsMinWidth        = 200.0
	DefaultMentionsMaxListHeight   = 256.0
	DefaultMentionsFocusRingOutset = 1.5
	DefaultMentionsRows            = 1
	DefaultMentionsSplit           = " "
	DefaultMentionsPrefix          = "@"
	DefaultMentionsNotFound        = "Not Found"
)

// MentionsPlacement is the suggestion panel side relative to the field.
type MentionsPlacement int

const (
	// MentionsBottom opens below the field (antd default).
	MentionsBottom MentionsPlacement = iota
	// MentionsTop opens above the field.
	MentionsTop
)

// MentionsOption is one suggestion (antd options[] item).
type MentionsOption struct {
	Value     string
	Label     string // empty → Value
	Key       string
	Disabled  bool
	LabelNode core.Node // optional custom label content
}

// DisplayLabel returns Label or falls back to Value.
func (o MentionsOption) DisplayLabel() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

// MentionsFilterFunc matches antd filterOption function form.
// Return true to keep the option visible.
type MentionsFilterFunc func(input string, option MentionsOption) bool

// MentionsEntity is one extracted mention token (Mentions.GetMentions).
type MentionsEntity struct {
	Prefix string
	Value  string
}

// mentionMeasure is the active trigger range before the caret.
type mentionMeasure struct {
	prefix string
	search string
	start  int // rune index of prefix in value
	end    int // caret rune index
}

// Mentions is multiline Input + anchored mention suggestions (antd Mentions).
//
//	Column (Wrap)
//	  ├─ multiline Input (TextArea-like)
//	  └─ AnchoredPopup
//	       └─ panel (options | notFound | loading)
//
// Product contract: docs/antd/mentions.md §6 (P0 DoD).
type Mentions struct {
	Wrap  *primitive.Flex
	field *Input
	popup *primitive.AnchoredPopup
	panel *primitive.Decorated
	list  *primitive.Flex
	Nav   *core.KeyboardNav

	// Product fields (§6.10).
	Value           string
	DefaultValue    string
	Placeholder     string
	Options         []MentionsOption
	Size            InputSize
	Variant         InputVariant
	Status          InputStatus
	Disabled        bool
	ReadOnly        bool
	AllowClear      bool
	Open            bool
	Loading         bool
	NotFoundContent string
	Placement       MentionsPlacement
	// Prefix triggers; empty → ["@"].
	Prefix []string
	// Split appended after an inserted mention (default single space).
	Split string
	// Rows is TextArea row count (default 1).
	Rows int
	// FilterOptionEnabled defaults true. false → show all (async feeds Options).
	FilterOptionEnabled bool
	// FilterOption custom predicate; nil → containsFold on value/label.
	FilterOption MentionsFilterFunc
	AriaLabel    string
	Face         text.Face
	Theme        *core.Theme
	Viewport     core.Size
	// FixedWidth forces field width (0 = DefaultMentionsMinWidth).
	FixedWidth float64

	// Controlled: typing only raises OnChange; display stays until SetValue.
	Controlled bool

	// Callbacks.
	OnChange     func(value string)
	OnSearch     func(text, prefix string)
	OnSelect     func(option MentionsOption, prefix string)
	OnOpenChange func(open bool)
	OnClear      func()

	// Flattened visible options for keyboard / select.
	visible []MentionsOption
	// activeIndex into visible (-1 = none).
	activeIndex int

	// Active measure while typing a mention (nil when idle).
	measure *mentionMeasure

	// Loading spinner.
	spinner   *primitive.Canvas
	spinPhase float64
	boundTree *core.Tree

	// selecting suppresses onSearch when selection writes value.
	selecting bool

	// openControlled: SetOpen was used.
	openControlled bool
	appliedDefault bool
}

// NewMentions creates a Mentions field with optional plain option values
// (usernames without the trigger prefix — antd options[].value).
//
// Defaults (§6.10): middle/outlined, prefix=["@"], split=" ", rows=1,
// filter on, placement bottom, closed, uncontrolled.
func NewMentions(placeholder string, optionValues ...string) *Mentions {
	m := &Mentions{
		Placeholder:         placeholder,
		Size:                InputMiddle,
		Variant:             InputOutlined,
		Status:              InputStatusNone,
		Placement:           MentionsBottom,
		Prefix:              []string{DefaultMentionsPrefix},
		Split:               DefaultMentionsSplit,
		Rows:                DefaultMentionsRows,
		FilterOptionEnabled: true,
		activeIndex:         -1,
	}
	if len(optionValues) > 0 {
		m.Options = stringsToMentionsOptions(optionValues)
	}
	m.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	m.rebuild()
	return m
}

// Node returns the composition root (field + popup host).

// ensureBuilt materializes the control tree if missing (#9).
func (m *Mentions) ensureBuilt() {
	if m == nil {
		return
	}
	if m.Wrap == nil {
		m.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (m *Mentions) structureChange() {
	if m == nil {
		return
	}
	m.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (m *Mentions) chromeChange() {
	if m == nil {
		return
	}
	m.ensureBuilt()
	m.rebuild()
}

func (m *Mentions) Node() core.Node {
	if m == nil {
		return nil
	}
	m.ensureBuilt()
	return m.Wrap
}

// Field returns the multiline input.
func (m *Mentions) Field() *Input {
	if m == nil {
		return nil
	}
	return m.field
}

// Input is an alias for Field (symmetry with AutoComplete).
func (m *Mentions) Input() *Input { return m.Field() }

// Popup returns the anchored popup (tests / advanced hosts).
func (m *Mentions) Popup() *primitive.AnchoredPopup {
	if m == nil {
		return nil
	}
	return m.popup
}

// Panel returns the dropdown panel chrome (tests).
func (m *Mentions) Panel() *primitive.Decorated {
	if m == nil {
		return nil
	}
	return m.panel
}

// IsOpen reports whether the suggestion panel is visible.
func (m *Mentions) IsOpen() bool {
	return m != nil && m.Open && m.shouldShowPopup()
}

// VisibleOptions returns the filtered options currently shown.
func (m *Mentions) VisibleOptions() []MentionsOption {
	if m == nil {
		return nil
	}
	out := make([]MentionsOption, len(m.visible))
	copy(out, m.visible)
	return out
}

// ActiveIndex returns the highlighted option index (-1 if none).
func (m *Mentions) ActiveIndex() int {
	if m == nil {
		return -1
	}
	return m.activeIndex
}

// GetValue returns the current value.
func (m *Mentions) GetValue() string {
	if m == nil {
		return ""
	}
	return m.Value
}

// MeasurePrefix returns the active trigger prefix (empty when idle).
func (m *Mentions) MeasurePrefix() string {
	if m == nil || m.measure == nil {
		return ""
	}
	return m.measure.prefix
}

// MeasureSearch returns the search text after the active prefix.
func (m *Mentions) MeasureSearch() string {
	if m == nil || m.measure == nil {
		return ""
	}
	return m.measure.search
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue sets the displayed value. Does not fire OnChange (parent/API write).
func (m *Mentions) SetValue(v string) {
	if m == nil {
		return
	}
	m.Value = v
	if m.field != nil {
		was := m.field.OnChange
		m.field.OnChange = nil
		m.field.SetValue(v)
		m.field.OnChange = was
	}
	m.recomputeMeasure(false)
	m.refreshList(true)
}

// SetDefaultValue seeds uncontrolled value when Value is still empty.
func (m *Mentions) SetDefaultValue(v string) {
	if m == nil {
		return
	}
	m.DefaultValue = v
	if !m.Controlled && m.Value == "" && !m.appliedDefault {
		m.appliedDefault = true
		m.SetValue(v)
	}
}

// SetPlaceholder updates the placeholder.
func (m *Mentions) SetPlaceholder(s string) {
	if m == nil {
		return
	}
	m.Placeholder = s
	if m.field != nil {
		m.field.SetPlaceholder(s)
	}
}

// SetOptions replaces the options model.
func (m *Mentions) SetOptions(opts []MentionsOption) {
	if m == nil {
		return
	}
	m.Options = append([]MentionsOption(nil), opts...)
	m.refreshList(true)
}

// SetOptionValues is a convenience for plain string options.
func (m *Mentions) SetOptionValues(vals ...string) {
	if m == nil {
		return
	}
	m.SetOptions(stringsToMentionsOptions(vals))
}

// SetDisabled toggles disabled (no edit / no open).
func (m *Mentions) SetDisabled(d bool) {
	if m == nil {
		return
	}
	m.Disabled = d
	if m.field != nil {
		m.field.SetDisabled(d)
	}
	if d && m.Open {
		m.applyOpen(false, false)
	}
	m.applyA11y()
}

// SetReadOnly toggles read-only (focus ok; no edit / no insert).
func (m *Mentions) SetReadOnly(r bool) {
	if m == nil {
		return
	}
	m.ReadOnly = r
	if m.field != nil {
		m.field.SetReadOnly(r)
	}
	if r && m.Open {
		m.applyOpen(false, false)
	}
	m.applyA11y()
}

// SetSize updates control size.
func (m *Mentions) SetSize(s InputSize) {
	if m == nil {
		return
	}
	m.Size = s
	if m.field != nil {
		m.field.SetSize(s)
	}
	m.applyA11y()
	m.refreshList(false)
}

// SetVariant updates visual variant.
func (m *Mentions) SetVariant(v InputVariant) {
	if m == nil {
		return
	}
	m.Variant = v
	if m.field != nil {
		m.field.SetVariant(v)
	}
	m.applyA11y()
}

// SetStatus updates validation chrome.
func (m *Mentions) SetStatus(s InputStatus) {
	if m == nil {
		return
	}
	m.Status = s
	if m.field != nil {
		m.field.SetStatus(s)
	}
	m.applyA11y()
}

// SetAllowClear toggles clear affix.
func (m *Mentions) SetAllowClear(v bool) {
	if m == nil {
		return
	}
	m.AllowClear = v
	if m.field != nil {
		m.field.SetAllowClear(v)
	}
	m.applyA11y()
}

// SetPlacement sets top/bottom popup placement.
func (m *Mentions) SetPlacement(p MentionsPlacement) {
	if m == nil {
		return
	}
	m.Placement = p
	if m.popup != nil {
		m.popup.Placement = m.popupPlacement()
	}
}

// SetPrefix sets one or more trigger keywords (default "@").
func (m *Mentions) SetPrefix(prefixes ...string) {
	if m == nil {
		return
	}
	if len(prefixes) == 0 {
		m.Prefix = []string{DefaultMentionsPrefix}
	} else {
		m.Prefix = append([]string(nil), prefixes...)
	}
	m.recomputeMeasure(true)
}

// SetSplit sets the separator appended after an inserted mention.
func (m *Mentions) SetSplit(s string) {
	if m == nil {
		return
	}
	m.Split = s
}

// SetRows sets multiline row count (min 1).
func (m *Mentions) SetRows(n int) {
	if m == nil {
		return
	}
	if n < 1 {
		n = DefaultMentionsRows
	}
	m.Rows = n
	if m.field != nil {
		m.field.SetRows(n)
	}
	m.applyA11y()
}

// SetOpen sets visibility (test / controlled).
func (m *Mentions) SetOpen(open bool) {
	if m == nil {
		return
	}
	m.openControlled = true
	m.applyOpen(open, false)
}

// SetFilterOption enables/disables default filtering.
func (m *Mentions) SetFilterOption(enabled bool) {
	if m == nil {
		return
	}
	m.FilterOptionEnabled = enabled
	m.refreshList(true)
}

// SetFilterOptionFunc sets a custom filter (enables filtering).
func (m *Mentions) SetFilterOptionFunc(fn MentionsFilterFunc) {
	if m == nil {
		return
	}
	m.FilterOption = fn
	m.FilterOptionEnabled = true
	m.refreshList(true)
}

// SetNotFoundContent sets empty-list content.
func (m *Mentions) SetNotFoundContent(s string) {
	if m == nil {
		return
	}
	m.NotFoundContent = s
	m.refreshList(false)
}

// SetLoading toggles async-search spinner in the panel (Ticker).
func (m *Mentions) SetLoading(v bool) {
	if m == nil || m.Loading == v {
		return
	}
	m.Loading = v
	m.refreshList(false)
	if m.boundTree != nil {
		if v {
			m.boundTree.AddTicker(m)
		} else {
			m.boundTree.RemoveTicker(m)
		}
	}
	// Keep panel open while loading even with empty options.
	if v && m.measure != nil {
		m.requestOpen(true)
	}
}

// SetFixedWidth forces the field width.
func (m *Mentions) SetFixedWidth(w float64) {
	if m == nil {
		return
	}
	m.FixedWidth = w
	if m.field != nil {
		m.field.SetFixedSize(w, 0)
	}
}

// SetControlled toggles controlled value mode.
func (m *Mentions) SetControlled(v bool) {
	if m == nil {
		return
	}
	m.Controlled = v
	if m.field != nil {
		m.field.SetControlled(v)
	}
}

// SetOnChange sets the value-change callback.
func (m *Mentions) SetOnChange(fn func(string)) {
	if m == nil {
		return
	}
	m.OnChange = fn
}

// SetOnSearch sets the search callback (text after prefix + which prefix).
func (m *Mentions) SetOnSearch(fn func(text, prefix string)) {
	if m == nil {
		return
	}
	m.OnSearch = fn
}

// SetOnSelect sets the option-select callback.
func (m *Mentions) SetOnSelect(fn func(option MentionsOption, prefix string)) {
	if m == nil {
		return
	}
	m.OnSelect = fn
}

// SetOnOpenChange sets the open-change callback.
func (m *Mentions) SetOnOpenChange(fn func(open bool)) {
	if m == nil {
		return
	}
	m.OnOpenChange = fn
}

// SetOnClear sets the clear-button callback.
func (m *Mentions) SetOnClear(fn func()) {
	if m == nil {
		return
	}
	m.OnClear = fn
	if m.field != nil {
		m.field.SetOnClear(fn)
	}
}

// SetTheme sets an explicit theme override.
func (m *Mentions) SetTheme(th *core.Theme) {
	if m == nil {
		return
	}
	m.Theme = th
	if m.field != nil {
		m.field.SetTheme(th)
	}
	m.rebuild()
}

// SetFace sets the font face.
func (m *Mentions) SetFace(face text.Face) {
	if m == nil {
		return
	}
	m.Face = face
	if m.field != nil {
		m.field.SetFace(face)
	}
	m.refreshList(false)
}

// SetAriaLabel sets the accessible name.
func (m *Mentions) SetAriaLabel(s string) {
	if m == nil {
		return
	}
	m.AriaLabel = s
	m.applyA11y()
}

// Focus focuses the editor (Mentions ref.focus).
func (m *Mentions) Focus() {
	if m == nil || m.field == nil || m.field.Editor() == nil || m.boundTree == nil {
		return
	}
	m.boundTree.SetFocus(m.field.Editor())
}

// Blur clears focus from the editor (Mentions ref.blur).
func (m *Mentions) Blur() {
	if m == nil || m.boundTree == nil {
		return
	}
	m.boundTree.SetFocus(nil)
	if m.Open {
		m.applyOpen(false, true)
	}
}

// AttachTicker registers loading spinner animation.
func (m *Mentions) AttachTicker(t *core.Tree) {
	if m == nil || t == nil {
		return
	}
	m.boundTree = t
	if m.field != nil {
		m.field.AttachTicker(t)
	}
	t.BindTicker(m, m.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (m *Mentions) Tick(dt float64) bool {
	if m == nil || !m.Loading {
		return false
	}
	m.spinPhase += dt * 1.4
	if m.spinPhase > 1 {
		m.spinPhase -= 1
	}
	if m.spinner != nil {
		m.spinner.MarkNeedsPaint()
	} else if m.panel != nil {
		m.panel.MarkNeedsPaint()
	}
	return m.Loading
}

// HandleKey processes arrow/enter/escape when the field is focused.
func (m *Mentions) HandleKey(ev *core.KeyEvent) {
	if m == nil || m.Disabled || m.ReadOnly || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !m.IsOpen() {
		return
	}
	if m.Nav != nil && m.Nav.HandleKey(ev.Key) {
		m.activeIndex = m.Nav.Index
		m.refreshList(false)
		ev.Handled = true
		return
	}
	switch ev.Key {
	case "Enter", "Return":
		if m.activeIndex >= 0 && m.activeIndex < len(m.visible) {
			m.applySelect(m.visible[m.activeIndex])
		}
		ev.Handled = true
	case "Escape", "Esc":
		m.applyOpen(false, true)
		m.measure = nil
		ev.Handled = true
	}
}

// Clear empties the value (allowClear path / API).
func (m *Mentions) Clear() {
	if m == nil || m.Disabled || m.ReadOnly {
		return
	}
	if m.Controlled {
		if m.OnChange != nil {
			m.OnChange("")
		}
	} else {
		m.Value = ""
		if m.field != nil {
			was := m.field.OnChange
			m.field.OnChange = nil
			m.field.SetValue("")
			m.field.OnChange = was
		}
		if m.OnChange != nil {
			m.OnChange("")
		}
	}
	if m.OnClear != nil {
		m.OnClear()
	}
	m.measure = nil
	m.refreshList(true)
	m.requestOpen(false)
}

// GetMentions extracts mention tokens from value (antd Mentions.getMentions).
// Empty prefixes → ["@"]; empty split → " ".
func GetMentions(value string, prefixes []string, split string) []MentionsEntity {
	if split == "" {
		split = DefaultMentionsSplit
	}
	if len(prefixes) == 0 {
		prefixes = []string{DefaultMentionsPrefix}
	}
	parts := strings.Split(value, split)
	var out []MentionsEntity
	for _, str := range parts {
		if str == "" {
			continue
		}
		for _, p := range prefixes {
			if p == "" {
				continue
			}
			if strings.HasPrefix(str, p) {
				out = append(out, MentionsEntity{
					Prefix: p,
					Value:  str[len(p):],
				})
				break
			}
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (m *Mentions) theme() *core.Theme {
	var n core.Node
	if m.Wrap != nil {
		n = m.Wrap
	}
	return themeOf(m.Theme, n)
}

func (m *Mentions) prefixes() []string {
	if m == nil || len(m.Prefix) == 0 {
		return []string{DefaultMentionsPrefix}
	}
	return m.Prefix
}

func (m *Mentions) popupPlacement() primitive.Placement {
	if m != nil && m.Placement == MentionsTop {
		return primitive.PlaceTopStart
	}
	return primitive.PlaceBottomStart
}

func (m *Mentions) rebuild() {
	if m == nil {
		return
	}
	th := m.theme()
	wasOpen := m.Open

	// Multiline field (antd Mentions = textarea).
	if m.field == nil {
		m.field = NewInput(m.Placeholder)
		m.field.multiline = true
	}
	rows := m.Rows
	if rows < 1 {
		rows = DefaultMentionsRows
	}
	m.field.Theme = m.Theme
	m.field.SetFace(m.Face)
	m.field.SetSize(m.Size)
	m.field.SetVariant(m.Variant)
	m.field.SetStatus(m.Status)
	m.field.SetDisabled(m.Disabled)
	m.field.SetReadOnly(m.ReadOnly)
	m.field.SetAllowClear(m.AllowClear)
	m.field.SetControlled(m.Controlled)
	m.field.SetPlaceholder(m.Placeholder)
	m.field.SetRows(rows)
	if m.FixedWidth > 0 {
		m.field.SetFixedSize(m.FixedWidth, 0)
	} else {
		m.field.SetFixedSize(DefaultMentionsMinWidth, 0)
	}
	// Seed default value once.
	if !m.appliedDefault && m.DefaultValue != "" && m.Value == "" && !m.Controlled {
		m.Value = m.DefaultValue
		m.appliedDefault = true
	}
	{
		was := m.field.OnChange
		m.field.OnChange = nil
		m.field.SetValue(m.Value)
		m.field.OnChange = was
	}
	m.wireField()

	// List + panel.
	m.list = primitive.Column()
	m.list.Gap = 2
	m.list.CrossAlign = core.CrossStart
	m.panel = primitive.NewDecorated(m.list)
	m.panel.SkinType = TypeMentions
	m.panel.Padding = primitive.All(DefaultMentionsPanelPad)
	m.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultMentionsPanelRadius)
	m.panel.Background = th.Color(core.TokenColorBgContainer)
	m.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	m.panel.BorderColor = th.Color(core.TokenColorBorder)
	m.panel.MinWidth = DefaultMentionsMinWidth

	m.popup = primitive.NewAnchoredPopup(m.panel)
	m.popup.Placement = m.popupPlacement()
	m.popup.Gap = DefaultMentionsGap
	m.popup.Portal.ID = ""
	m.popup.DismissOnOutside = true
	m.popup.OnDismiss = func() {
		m.Open = false
		m.measure = nil
		if m.OnOpenChange != nil {
			m.OnOpenChange(false)
		}
		m.applyA11y()
	}
	m.popup.AnchorNode = m.field.Node()

	if m.Wrap == nil {
		m.Wrap = primitive.Column(m.field.Node(), m.popup)
	} else {
		m.Wrap.ClearChildren()
		m.Wrap.AddChild(m.field.Node())
		m.Wrap.AddChild(m.popup)
	}
	m.Wrap.CrossAlign = core.CrossStart
	m.Wrap.Gap = 0
	m.Wrap.SetThemeHook(func(*core.Theme) { m.rebuild() })
	m.Wrap.MarkNeedsLayout()
	m.Wrap.MarkNeedsPaint()

	m.applyA11y()
	m.refreshList(false)

	if wasOpen {
		m.applyOpen(true, false)
	}
}

func (m *Mentions) wireField() {
	if m == nil || m.field == nil {
		return
	}
	m.field.SetOnChange(func(v string) {
		if m.Disabled || m.selecting {
			return
		}
		if !m.Controlled {
			m.Value = v
		}
		if m.OnChange != nil {
			m.OnChange(v)
		}
		m.recomputeMeasure(true)
	})
	m.field.SetOnClear(func() {
		if m.Disabled || m.ReadOnly {
			return
		}
		if !m.Controlled {
			m.Value = ""
		}
		if m.OnClear != nil {
			m.OnClear()
		}
		m.measure = nil
		m.refreshList(true)
		m.requestOpen(false)
	})
	// Keyboard: intercept arrows when open; Enter selects.
	if ed := m.field.Editor(); ed != nil {
		ed.VerticalArrowHandler = func(dir int) bool {
			if m.Disabled || m.ReadOnly || !m.IsOpen() {
				return false
			}
			key := "ArrowDown"
			if dir < 0 {
				key = "ArrowUp"
			}
			if m.Nav != nil && m.Nav.HandleKey(key) {
				m.activeIndex = m.Nav.Index
				m.refreshList(false)
				return true
			}
			return false
		}
	}
	m.field.SetOnPressEnter(func(string) {
		if m.Disabled || m.ReadOnly {
			return
		}
		if m.IsOpen() && m.activeIndex >= 0 && m.activeIndex < len(m.visible) {
			m.applySelect(m.visible[m.activeIndex])
		}
	})
}

func (m *Mentions) liveValue() string {
	if m == nil {
		return ""
	}
	// Prefer live editor text for measure (controlled parents catch up via OnChange).
	if m.field != nil && m.field.Editor() != nil {
		return m.field.Editor().Value
	}
	return m.Value
}

func (m *Mentions) caretIndex() int {
	if m == nil || m.field == nil || m.field.Editor() == nil {
		return utf8.RuneCountInString(m.liveValue())
	}
	return m.field.Editor().Cursor
}

// recomputeMeasure scans for an active mention trigger before the caret.
// When fireSearch is true and measure is active, OnSearch is invoked.
func (m *Mentions) recomputeMeasure(fireSearch bool) {
	if m == nil || m.Disabled || m.ReadOnly {
		m.measure = nil
		m.requestOpen(false)
		return
	}
	val := m.liveValue()
	caret := m.caretIndex()
	meas := measureMention(val, caret, m.prefixes(), m.Split)
	prev := m.measure
	m.measure = meas
	if meas == nil {
		m.refreshList(true)
		m.requestOpen(false)
		return
	}
	if fireSearch && m.OnSearch != nil {
		// Fire when first opened or search/prefix changed.
		if prev == nil || prev.search != meas.search || prev.prefix != meas.prefix {
			m.OnSearch(meas.search, meas.prefix)
		}
	}
	m.refreshList(true)
	if len(m.visible) > 0 || m.Loading || m.NotFoundContent != "" {
		m.requestOpen(true)
	} else {
		// Empty static list with no notFound → close (antd FAQ-ish).
		m.requestOpen(false)
	}
}

// measureMention finds the last valid prefix before caret.
// Boundary: start of text / after whitespace. Search must not contain split.
func measureMention(value string, caret int, prefixes []string, split string) *mentionMeasure {
	if caret < 0 {
		caret = 0
	}
	runes := []rune(value)
	if caret > len(runes) {
		caret = len(runes)
	}
	before := string(runes[:caret])
	if before == "" || len(prefixes) == 0 {
		return nil
	}

	// Prefer longer prefixes first to disambiguate.
	ordered := append([]string(nil), prefixes...)
	for i := 0; i < len(ordered); i++ {
		for j := i + 1; j < len(ordered); j++ {
			if len(ordered[j]) > len(ordered[i]) {
				ordered[i], ordered[j] = ordered[j], ordered[i]
			}
		}
	}

	bestStart := -1
	bestPrefix := ""
	for _, p := range ordered {
		if p == "" {
			continue
		}
		// Find last occurrence of p in before.
		idx := strings.LastIndex(before, p)
		if idx < 0 {
			continue
		}
		// Boundary: idx==0 or previous char is whitespace.
		if idx > 0 {
			prev, _ := utf8.DecodeLastRuneInString(before[:idx])
			if prev != utf8.RuneError && !unicode.IsSpace(prev) {
				continue
			}
		}
		search := before[idx+len(p):]
		// Default validateSearch: search must not contain split.
		if split != "" && strings.Contains(search, split) {
			continue
		}
		// Also reject newlines in search (end of mention context).
		if strings.ContainsAny(search, "\n\r") {
			continue
		}
		if idx > bestStart {
			bestStart = idx
			bestPrefix = p
		}
	}
	if bestStart < 0 || bestPrefix == "" {
		return nil
	}
	// Convert byte idx → rune idx for start.
	startRunes := utf8.RuneCountInString(before[:bestStart])
	search := before[bestStart+len(bestPrefix):]
	return &mentionMeasure{
		prefix: bestPrefix,
		search: search,
		start:  startRunes,
		end:    caret,
	}
}

func (m *Mentions) matchOption(q string, opt MentionsOption) bool {
	if !m.FilterOptionEnabled {
		return true
	}
	if m.FilterOption != nil {
		return m.FilterOption(q, opt)
	}
	if q == "" {
		return true
	}
	return containsFold(opt.Value, q) || containsFold(opt.DisplayLabel(), q)
}

func (m *Mentions) collectVisible() []MentionsOption {
	q := ""
	if m.measure != nil {
		q = m.measure.search
	}
	var out []MentionsOption
	for _, o := range m.Options {
		if o.Disabled {
			continue
		}
		if m.matchOption(q, o) {
			out = append(out, o)
		}
	}
	return out
}

func (m *Mentions) shouldShowPopup() bool {
	if m == nil || m.Disabled || m.ReadOnly {
		return false
	}
	// Product default: only while measuring a trigger. Controlled SetOpen may
	// force the panel for tests/gallery without an active measure.
	if m.measure == nil && !m.openControlled {
		return false
	}
	if m.Loading {
		return true
	}
	if len(m.visible) > 0 {
		return true
	}
	return m.NotFoundContent != "" && m.Open
}

func (m *Mentions) requestOpen(open bool) {
	if m.openControlled {
		if m.OnOpenChange != nil && m.Open != open {
			m.OnOpenChange(open)
		}
		return
	}
	m.applyOpen(open, true)
}

func (m *Mentions) applyOpen(open bool, notify bool) {
	if m == nil {
		return
	}
	prev := m.Open
	m.Open = open
	if m.popup == nil {
		return
	}
	show := open && m.shouldShowPopup()
	if open && !show && m.NotFoundContent == "" && !m.Loading {
		m.popup.SetOpen(false)
	} else {
		if show {
			m.measurePanel()
			if m.popup.AnchorNode != nil {
				m.popup.UpdateAnchorFromNode(m.popup.AnchorNode)
			}
			if m.Viewport.Width > 0 {
				m.popup.Viewport = m.Viewport
			}
			if open && !prev {
				m.resetActive()
			}
		}
		m.popup.SetOpen(show)
	}
	// While the mention panel is open, treat Enter as select (OnSubmit path)
	// rather than inserting a newline in the multiline editor.
	m.syncEnterMode(show)
	if notify && prev != open && m.OnOpenChange != nil {
		m.OnOpenChange(open)
	}
	m.applyA11y()
	if m.list != nil {
		m.refreshList(false)
	}
}

func (m *Mentions) resetActive() {
	if len(m.visible) > 0 {
		m.activeIndex = 0
	} else {
		m.activeIndex = -1
	}
	if m.Nav != nil {
		m.Nav.SetCount(len(m.visible))
		if m.activeIndex >= 0 {
			m.Nav.Index = m.activeIndex
		} else {
			m.Nav.Index = 0
		}
	}
}

func (m *Mentions) refreshList(refilter bool) {
	if m == nil || m.list == nil {
		return
	}
	_ = refilter
	m.visible = m.collectVisible()
	if m.Nav != nil {
		m.Nav.SetCount(len(m.visible))
		if m.activeIndex >= len(m.visible) {
			m.activeIndex = len(m.visible) - 1
		}
		if m.activeIndex < 0 && len(m.visible) > 0 && m.Open {
			m.activeIndex = 0
		}
		if m.activeIndex >= 0 {
			m.Nav.Index = m.activeIndex
		}
	}

	th := m.theme()
	m.list.ClearChildren()
	m.spinner = nil

	if m.Loading {
		spin := primitive.NewCanvas(16, 16, m.paintSpinner)
		m.spinner = spin
		row := primitive.Row(spin)
		row.MainAlign = core.MainCenter
		row.CrossAlign = core.CrossCenter
		row.Padding = primitive.Symmetric(12, 8)
		m.list.AddChild(row)
	}

	if len(m.visible) == 0 {
		if m.NotFoundContent != "" && !m.Loading {
			lab := primitive.NewText(m.NotFoundContent)
			lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
			lab.Face = m.Face
			lab.Color = th.Color(core.TokenColorTextSecondary)
			pad := primitive.NewDecorated(lab)
			pad.Padding = primitive.Symmetric(DefaultMentionsItemPadInline, DefaultMentionsItemPadBlock)
			pad.Hit = core.HitTransparent
			m.list.AddChild(pad)
		}
	} else {
		for i, opt := range m.visible {
			m.list.AddChild(m.buildOptionRow(opt, i, th))
		}
	}

	// Keep popup visibility in sync.
	if m.popup != nil {
		show := m.Open && m.shouldShowPopup()
		if m.popup.Open != show {
			if show {
				m.measurePanel()
				if m.popup.AnchorNode != nil {
					m.popup.UpdateAnchorFromNode(m.popup.AnchorNode)
				}
			}
			m.popup.SetOpen(show)
		} else if show {
			m.measurePanel()
		}
		m.syncEnterMode(show)
	}
	m.list.MarkNeedsLayout()
	m.list.MarkNeedsPaint()
}

// syncEnterMode flips EditableText.Multiline so Enter selects while the panel is open.
func (m *Mentions) syncEnterMode(panelOpen bool) {
	if m == nil || m.field == nil {
		return
	}
	if ed := m.field.Editor(); ed != nil {
		// Multiline false → Enter fires OnSubmit (select); true → newline.
		ed.Multiline = !panelOpen
	}
}

func (m *Mentions) buildOptionRow(opt MentionsOption, idx int, th *core.Theme) core.Node {
	var content core.Node
	if opt.LabelNode != nil {
		content = opt.LabelNode
	} else {
		lab := primitive.NewText(opt.DisplayLabel())
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = m.Face
		lab.Color = th.Color(core.TokenColorText)
		content = lab
	}
	item := primitive.NewPressable(content)
	item.Padding = primitive.Symmetric(DefaultMentionsItemPadInline, DefaultMentionsItemPadBlock)
	item.ShowFocusRing = false
	if idx == m.activeIndex {
		item.Color = antItemSelectedFill(th)
	}
	item.ColorHovered = antItemHoverFill(th)
	optCopy := opt
	item.Click = func() {
		if m.Disabled || m.ReadOnly || optCopy.Disabled {
			return
		}
		m.applySelect(optCopy)
	}
	item.Base().Role = "option"
	item.Base().Label = opt.DisplayLabel()
	return item
}

func (m *Mentions) applySelect(opt MentionsOption) {
	if m == nil || m.Disabled || m.ReadOnly {
		return
	}
	m.selecting = true
	defer func() { m.selecting = false }()

	prefix := DefaultMentionsPrefix
	start, end := 0, utf8.RuneCountInString(m.liveValue())
	if m.measure != nil {
		prefix = m.measure.prefix
		start = m.measure.start
		end = m.measure.end
	}
	// Split may be "" when caller SetSplit(""); NewMentions defaults to " ".
	insert := prefix + opt.Value + m.Split
	valRunes := []rune(m.liveValue())
	if start < 0 {
		start = 0
	}
	if end > len(valRunes) {
		end = len(valRunes)
	}
	if start > end {
		start = end
	}
	newVal := string(append(append(append([]rune{}, valRunes[:start]...), []rune(insert)...), valRunes[end:]...))
	newCaret := start + utf8.RuneCountInString(insert)

	if !m.Controlled {
		m.Value = newVal
	}
	if m.field != nil {
		was := m.field.OnChange
		m.field.OnChange = nil
		m.field.SetValue(newVal)
		m.field.OnChange = was
		if ed := m.field.Editor(); ed != nil {
			ed.Cursor = newCaret
			ed.SelAnchor = newCaret
			ed.MarkNeedsPaint()
		}
	}
	if m.OnSelect != nil {
		m.OnSelect(opt, prefix)
	}
	if m.OnChange != nil {
		m.OnChange(newVal)
	}
	m.measure = nil
	m.applyOpen(false, true)
	m.refreshList(true)
}

func (m *Mentions) measurePanel() {
	if m == nil || m.panel == nil {
		return
	}
	w := DefaultMentionsMinWidth
	if m.popup != nil && m.popup.AnchorNode != nil {
		b := m.popup.AnchorNode.Base().Size()
		if b.Width > w {
			w = b.Width
		}
	}
	if m.field != nil && m.field.ChromeNode() != nil {
		if s := m.field.ChromeNode().Base().Size(); s.Width > w {
			w = s.Width
		}
	}
	if m.FixedWidth > w {
		w = m.FixedWidth
	}
	m.panel.MinWidth = w
	m.panel.Width = w
}

func (m *Mentions) applyA11y() {
	if m == nil || m.field == nil {
		return
	}
	label := m.AriaLabel
	if label == "" {
		label = m.Placeholder
	}
	// Avoid field.SetAriaLabel — Input.applyA11y would overwrite Role to textbox.
	if m.AriaLabel != "" {
		m.field.AriaLabel = m.AriaLabel
	}
	if ed := m.field.Editor(); ed != nil {
		b := ed.Base()
		b.Role = "combobox"
		b.Label = label
		if m.Status == InputStatusError {
			b.Live = "assertive"
			if label != "" && !strings.Contains(label, "invalid") {
				b.Label = label + " (invalid)"
			}
		} else {
			b.Live = ""
		}
	}
	if m.list != nil {
		m.list.Base().Role = "listbox"
	}
}

func (m *Mentions) paintSpinner(pc *core.PaintContext, size core.Size) {
	th := m.theme()
	col := th.Color(core.TokenColorPrimary)
	if col.A < 0.1 {
		col = render.Hex("#1677FF")
	}
	cx, cy := size.Width/2, size.Height/2
	r := math.Min(size.Width, size.Height)/2 - 1
	if r < 2 {
		r = 2
	}
	stroke := 1.5
	steps := 12
	start := -math.Pi/2 + m.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func stringsToMentionsOptions(vals []string) []MentionsOption {
	out := make([]MentionsOption, len(vals))
	for i, v := range vals {
		out[i] = MentionsOption{Value: v, Label: v, Key: v}
	}
	return out
}
