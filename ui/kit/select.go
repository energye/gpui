package kit

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Select defaults — docs/antd/select.md §6.2 / §6.10
// https://ant.design/components/select
const (
	DefaultSelectMinWidth        = 200.0
	DefaultSelectWidth           = 240.0
	DefaultSelectGap             = 4.0
	DefaultSelectPanelPad        = 4.0
	DefaultSelectItemPadInline   = 12.0
	DefaultSelectItemPadBlock    = 5.0
	DefaultSelectPanelRadius     = 8.0
	DefaultSelectListHeight      = 256.0
	DefaultSelectFocusRingOutset = 1.5
	DefaultSelectTagGap          = 4.0
)

// SelectMode is antd mode: default (single) | multiple | tags.
type SelectMode int

const (
	SelectSingle SelectMode = iota
	SelectMultiple
	SelectTags
)

// SelectPlacement is popup placement (antd placement).
type SelectPlacement int

const (
	SelectBottomLeft SelectPlacement = iota // default
	SelectBottomRight
	SelectTopLeft
	SelectTopRight
)

// SelectOption is one antd Select options[] entry.
type SelectOption struct {
	Value    string
	Label    string
	Disabled bool
	Title    string
	// Fields holds extra searchable string fields (search-multi-field.tsx).
	Fields map[string]string
	// Desc / Emoji are convenience fields for option-render demos.
	Desc  string
	Emoji string
	// LabelNode is custom option content (optionRender result / LabelNode).
	LabelNode core.Node
}

// DisplayLabel returns Label or falls back to Value.
func (o SelectOption) DisplayLabel() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

// SelectFilterFunc matches antd filterOption function form.
// Return true to keep the option visible.
type SelectFilterFunc func(input string, option SelectOption) bool

// SelectFilterSortFunc matches antd filterSort.
// Return <0 if a before b, >0 if a after b, 0 if equal.
type SelectFilterSortFunc func(a, b SelectOption) int

// SelectOptionRender customizes option row content (antd optionRender).
type SelectOptionRender func(option SelectOption) core.Node

// Select is antd Select — field trigger + anchored option list.
//
//	Column (Wrap)
//	  ├─ Pressable trigger (selector chrome)
//	  └─ AnchoredPopup
//	       └─ panel (options | notFound | loading)
//
// Product contract: docs/antd/select.md §6 (P0 DoD).
type Select struct {
	Wrap     *primitive.Flex
	Root     *primitive.Pressable // trigger shell; identity stable across rebuilds
	decor    *primitive.Decorated
	display  *primitive.Text
	searchEd *primitive.EditableText
	clearBtn *primitive.Pressable
	suffix   *primitive.Icon
	spinner  *primitive.Canvas
	popup    *primitive.AnchoredPopup
	panel    *primitive.Decorated
	list     *primitive.Flex
	Nav      *core.KeyboardNav

	// Product fields (§6.10).
	Options                  []SelectOption
	Value                    string   // single
	Values                   []string // multiple / tags
	DefaultValue             string
	DefaultValues            []string
	Placeholder              string
	Size                     InputSize
	Variant                  InputVariant
	Status                   InputStatus
	Mode                     SelectMode
	Disabled                 bool
	Loading                  bool
	AllowClear               bool
	ShowSearch               bool
	Open                     bool
	NotFoundContent          string
	DefaultActiveFirstOption bool
	PopupMatchSelectWidth    bool
	ListHeight               float64
	MaxTagCount              int // 0 = unlimited
	Placement                SelectPlacement
	Title                    string
	AriaLabel                string
	Face                     text.Face
	Theme                    *core.Theme
	Viewport                 core.Size
	FixedWidth               float64
	SearchValue              string

	// FilterOptionEnabled defaults true when ShowSearch. When false, all options shown.
	FilterOptionEnabled bool
	// FilterOption custom predicate; nil → optionFilterProp containsFold.
	FilterOption SelectFilterFunc
	// OptionFilterProp field names to match (default ["value"]).
	OptionFilterProp []string
	// FilterSort optional sort after filter.
	FilterSort SelectFilterSortFunc
	// OptionRender custom option body.
	OptionRender SelectOptionRender

	// Controlled: selection only raises OnChange; value waits for SetValue/SetValues.
	Controlled bool

	// Callbacks.
	OnChange      func(value string)
	OnChangeMulti func(values []string)
	OnSelect      func(value string, option SelectOption)
	OnDeselect    func(value string, option SelectOption)
	OnOpenChange  func(open bool)
	OnSearch      func(value string)
	OnClear       func()

	// openControlled: SetOpen used (antd open prop).
	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool

	// Flattened visible options for keyboard / select.
	visible     []SelectOption
	activeIndex int

	// Loading spinner.
	spinPhase float64
	boundTree *core.Tree

	// searchFocused: search editor has focus while open.
	searchFocused bool
}

// NewSelect creates a Select with optional options.
// Defaults (§6.10): middle/outlined, single mode, closed, uncontrolled,
// defaultActiveFirstOption=true, popupMatchSelectWidth=true, listHeight=256.
func NewSelect(placeholder string, options ...SelectOption) *Select {
	s := &Select{
		Placeholder:              placeholder,
		Options:                  append([]SelectOption(nil), options...),
		Size:                     InputMiddle,
		Variant:                  InputOutlined,
		Status:                   InputStatusNone,
		Mode:                     SelectSingle,
		DefaultActiveFirstOption: true,
		PopupMatchSelectWidth:    true,
		ListHeight:               DefaultSelectListHeight,
		Placement:                SelectBottomLeft,
		FilterOptionEnabled:      true,
		OptionFilterProp:         []string{"value"},
		activeIndex:              -1,
		NotFoundContent:          "Not Found",
	}
	s.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	s.rebuild()
	return s
}

// Node returns the composition root (trigger + popup host).
func (s *Select) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Wrap == nil {
		s.rebuild()
	}
	return s.Wrap
}

// ChromeNode returns the trigger Decorated chrome (tests / layout).
func (s *Select) ChromeNode() core.Node {
	if s == nil {
		return nil
	}
	if s.decor == nil {
		s.rebuild()
	}
	return s.decor
}

// Popup returns the anchored popup (tests / advanced hosts).
func (s *Select) Popup() *primitive.AnchoredPopup {
	if s == nil {
		return nil
	}
	return s.popup
}

// Panel returns the dropdown panel chrome (tests).
func (s *Select) Panel() *primitive.Decorated {
	if s == nil {
		return nil
	}
	return s.panel
}

// IsOpen reports whether the dropdown panel is visible.
func (s *Select) IsOpen() bool {
	return s != nil && s.Open
}

// VisibleOptions returns the filtered options currently shown.
func (s *Select) VisibleOptions() []SelectOption {
	if s == nil {
		return nil
	}
	out := make([]SelectOption, len(s.visible))
	copy(out, s.visible)
	return out
}

// ActiveIndex returns the highlighted option index (-1 if none).
func (s *Select) ActiveIndex() int {
	if s == nil {
		return -1
	}
	return s.activeIndex
}

// GetValue returns the single-select value.
func (s *Select) GetValue() string {
	if s == nil {
		return ""
	}
	return s.Value
}

// GetValues returns multi/tags values (copy).
func (s *Select) GetValues() []string {
	if s == nil {
		return nil
	}
	out := make([]string, len(s.Values))
	copy(out, s.Values)
	return out
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue sets the single-select value. Does not fire OnChange.
func (s *Select) SetValue(v string) {
	if s == nil {
		return
	}
	s.Value = v
	if s.Mode == SelectSingle {
		s.Values = nil
	}
	s.refreshDisplay()
	s.refreshList(false)
	s.applyChrome()
}

// SetValues sets multi/tags values. Does not fire OnChange.
func (s *Select) SetValues(vals []string) {
	if s == nil {
		return
	}
	s.Values = append([]string(nil), vals...)
	if s.Mode != SelectSingle && len(vals) > 0 {
		s.Value = vals[len(vals)-1]
	} else if s.Mode != SelectSingle {
		s.Value = ""
	}
	s.refreshDisplay()
	s.refreshList(false)
	s.applyChrome()
}

// SetDefaultValue seeds uncontrolled single value when still empty.
func (s *Select) SetDefaultValue(v string) {
	if s == nil || s.Controlled {
		return
	}
	s.DefaultValue = v
	if s.Value == "" && s.Mode == SelectSingle {
		s.Value = v
		s.appliedDefVal = true
		s.refreshDisplay()
	}
}

// SetDefaultValues seeds uncontrolled multi/tags values when still empty.
func (s *Select) SetDefaultValues(vals []string) {
	if s == nil || s.Controlled {
		return
	}
	s.DefaultValues = append([]string(nil), vals...)
	if len(s.Values) == 0 && s.Mode != SelectSingle {
		s.Values = append([]string(nil), vals...)
		s.appliedDefVal = true
		s.refreshDisplay()
	}
}

// SetPlaceholder updates placeholder text.
func (s *Select) SetPlaceholder(p string) {
	if s == nil {
		return
	}
	s.Placeholder = p
	s.refreshDisplay()
}

// SetOptions replaces the options model.
func (s *Select) SetOptions(opts ...SelectOption) {
	if s == nil {
		return
	}
	s.Options = append([]SelectOption(nil), opts...)
	s.refreshList(true)
}

// SetDisabled toggles disabled (no open / no select).
func (s *Select) SetDisabled(d bool) {
	if s == nil {
		return
	}
	s.Disabled = d
	if s.Root != nil {
		s.Root.SetDisabled(d)
	}
	if d && s.Open {
		s.applyOpen(false, false)
	}
	s.applyChrome()
	s.applyA11y()
}

// SetLoading toggles loading spinner on the suffix (Ticker).
func (s *Select) SetLoading(v bool) {
	if s == nil || s.Loading == v {
		return
	}
	s.Loading = v
	s.rebuild()
	if s.boundTree != nil {
		if v {
			s.boundTree.AddTicker(s)
		} else {
			s.boundTree.RemoveTicker(s)
		}
	}
}

// SetSize updates control height via Token.
func (s *Select) SetSize(sz InputSize) {
	if s == nil {
		return
	}
	s.Size = sz
	s.rebuild()
}

// SetVariant updates visual variant.
func (s *Select) SetVariant(v InputVariant) {
	if s == nil {
		return
	}
	s.Variant = v
	s.applyChrome()
}

// SetStatus updates validation chrome.
func (s *Select) SetStatus(st InputStatus) {
	if s == nil {
		return
	}
	s.Status = st
	s.applyChrome()
	s.applyA11y()
}

// SetMode updates selection mode (single/multiple/tags).
func (s *Select) SetMode(m SelectMode) {
	if s == nil {
		return
	}
	s.Mode = m
	s.rebuild()
}

// SetAllowClear toggles clear affordance.
func (s *Select) SetAllowClear(v bool) {
	if s == nil {
		return
	}
	s.AllowClear = v
	s.rebuild()
}

// SetShowSearch toggles search filtering in the selector.
func (s *Select) SetShowSearch(v bool) {
	if s == nil {
		return
	}
	s.ShowSearch = v
	s.rebuild()
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (s *Select) SetOpen(open bool) {
	if s == nil {
		return
	}
	s.openControlled = true
	s.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (s *Select) SetDefaultOpen(open bool) {
	if s == nil || s.openControlled {
		return
	}
	s.defaultOpen = open
	s.defaultOpenSet = true
	if !s.appliedDefault {
		s.appliedDefault = true
		s.applyOpen(open, false)
	}
}

// SetMaxTagCount sets max visible tags before +N fold (0 = unlimited).
func (s *Select) SetMaxTagCount(n int) {
	if s == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	s.MaxTagCount = n
	s.refreshDisplay()
}

// SetPlacement sets popup placement.
func (s *Select) SetPlacement(p SelectPlacement) {
	if s == nil {
		return
	}
	s.Placement = p
	if s.popup != nil {
		s.popup.Placement = mapSelectPlacement(p)
	}
}

// SetListHeight sets the panel max scroll height (default 256).
func (s *Select) SetListHeight(h float64) {
	if s == nil {
		return
	}
	if h <= 0 {
		h = DefaultSelectListHeight
	}
	s.ListHeight = h
}

// SetNotFoundContent sets empty-list content.
func (s *Select) SetNotFoundContent(c string) {
	if s == nil {
		return
	}
	s.NotFoundContent = c
	s.refreshList(false)
}

// SetTitle sets antd title (tooltip attribute / a11y hint).
func (s *Select) SetTitle(t string) {
	if s == nil {
		return
	}
	s.Title = t
	s.applyA11y()
}

// SetFixedWidth forces the trigger width (0 = DefaultSelectWidth).
func (s *Select) SetFixedWidth(w float64) {
	if s == nil {
		return
	}
	s.FixedWidth = w
	if s.decor != nil {
		if w > 0 {
			s.decor.Width = w
			s.decor.MinWidth = w
		}
		s.decor.MarkNeedsLayout()
	}
}

// SetOptionFilterProp sets fields used by default filter (antd optionFilterProp).
func (s *Select) SetOptionFilterProp(props ...string) {
	if s == nil {
		return
	}
	if len(props) == 0 {
		s.OptionFilterProp = []string{"value"}
	} else {
		s.OptionFilterProp = append([]string(nil), props...)
	}
	s.refreshList(true)
}

// SetFilterOption enables/disables default filtering (antd filterOption bool).
func (s *Select) SetFilterOption(enabled bool) {
	if s == nil {
		return
	}
	s.FilterOptionEnabled = enabled
	s.refreshList(true)
}

// SetFilterOptionFunc sets a custom filter (enables filtering).
func (s *Select) SetFilterOptionFunc(fn SelectFilterFunc) {
	if s == nil {
		return
	}
	s.FilterOption = fn
	s.FilterOptionEnabled = true
	s.refreshList(true)
}

// SetFilterSort sets option sort after filter (antd filterSort).
func (s *Select) SetFilterSort(fn SelectFilterSortFunc) {
	if s == nil {
		return
	}
	s.FilterSort = fn
	s.refreshList(true)
}

// SetSearchValue sets the search query (showSearch). Fires OnSearch.
func (s *Select) SetSearchValue(q string) {
	if s == nil {
		return
	}
	s.SearchValue = q
	if s.searchEd != nil {
		s.searchEd.Value = q
		s.searchEd.MarkNeedsPaint()
	}
	if s.OnSearch != nil {
		s.OnSearch(q)
	}
	s.refreshList(true)
	if s.ShowSearch && !s.Open && q != "" && !s.Disabled {
		s.requestOpen(true)
	}
}

// SetOptionRender sets custom option body renderer.
func (s *Select) SetOptionRender(fn SelectOptionRender) {
	if s == nil {
		return
	}
	s.OptionRender = fn
	s.refreshList(false)
}

// SetDefaultActiveFirstOption toggles first-option highlight on open.
func (s *Select) SetDefaultActiveFirstOption(v bool) {
	if s == nil {
		return
	}
	s.DefaultActiveFirstOption = v
}

// SetPopupMatchSelectWidth toggles panel min-width matching the trigger.
func (s *Select) SetPopupMatchSelectWidth(v bool) {
	if s == nil {
		return
	}
	s.PopupMatchSelectWidth = v
	s.measurePanel()
}

// SetControlled toggles controlled value mode.
func (s *Select) SetControlled(v bool) {
	if s == nil {
		return
	}
	s.Controlled = v
}

// SetOnChange sets the single-select change callback.
func (s *Select) SetOnChange(fn func(string)) {
	if s == nil {
		return
	}
	s.OnChange = fn
}

// SetOnChangeMulti sets the multi/tags change callback.
func (s *Select) SetOnChangeMulti(fn func([]string)) {
	if s == nil {
		return
	}
	s.OnChangeMulti = fn
}

// SetOnSelect sets the option-select callback.
func (s *Select) SetOnSelect(fn func(value string, option SelectOption)) {
	if s == nil {
		return
	}
	s.OnSelect = fn
}

// SetOnDeselect sets the multi/tags deselect callback.
func (s *Select) SetOnDeselect(fn func(value string, option SelectOption)) {
	if s == nil {
		return
	}
	s.OnDeselect = fn
}

// SetOnOpenChange sets the open-change callback.
func (s *Select) SetOnOpenChange(fn func(open bool)) {
	if s == nil {
		return
	}
	s.OnOpenChange = fn
}

// SetOnSearch sets the search callback.
func (s *Select) SetOnSearch(fn func(string)) {
	if s == nil {
		return
	}
	s.OnSearch = fn
}

// SetOnClear sets the clear-button callback.
func (s *Select) SetOnClear(fn func()) {
	if s == nil {
		return
	}
	s.OnClear = fn
}

// SetTheme sets an explicit theme override.
func (s *Select) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.rebuild()
}

// SetFace sets the font face.
func (s *Select) SetFace(face text.Face) {
	if s == nil {
		return
	}
	s.Face = face
	s.rebuild()
}

// SetAriaLabel sets the accessible name.
func (s *Select) SetAriaLabel(name string) {
	if s == nil {
		return
	}
	s.AriaLabel = name
	s.applyA11y()
}

// AttachTicker registers loading spinner animation.
func (s *Select) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (s *Select) Tick(dt float64) bool {
	if s == nil || !s.Loading {
		return false
	}
	s.spinPhase += dt * 1.4
	if s.spinPhase > 1 {
		s.spinPhase -= 1
	}
	if s.spinner != nil {
		s.spinner.MarkNeedsPaint()
	} else if s.decor != nil {
		s.decor.MarkNeedsPaint()
	}
	return s.Loading
}

// Clear empties the selection (allowClear path / API).
func (s *Select) Clear() {
	if s == nil || s.Disabled {
		return
	}
	// Prevent rebuild from re-applying default* after an explicit clear.
	s.appliedDefVal = true
	if s.Controlled {
		if s.Mode == SelectSingle {
			if s.OnChange != nil {
				s.OnChange("")
			}
		} else if s.OnChangeMulti != nil {
			s.OnChangeMulti(nil)
		}
	} else {
		s.Value = ""
		s.Values = nil
		if s.Mode == SelectSingle {
			if s.OnChange != nil {
				s.OnChange("")
			}
		} else if s.OnChangeMulti != nil {
			s.OnChangeMulti(nil)
		}
	}
	s.SearchValue = ""
	if s.searchEd != nil {
		s.searchEd.Value = ""
	}
	if s.OnClear != nil {
		s.OnClear()
	}
	s.refreshDisplay()
	s.refreshList(true)
	s.applyChrome()
}

// HandleKey for arrow/enter/escape when focused.
func (s *Select) HandleKey(ev *core.KeyEvent) {
	if s == nil || s.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !s.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" || ev.Key == "Down" {
			s.requestOpen(true)
			ev.Handled = true
		}
		return
	}
	if s.Nav != nil && s.Nav.HandleKey(ev.Key) {
		s.activeIndex = s.Nav.Index
		s.refreshList(false)
		ev.Handled = true
		return
	}
	switch ev.Key {
	case "Enter":
		if s.activeIndex >= 0 && s.activeIndex < len(s.visible) {
			s.applySelect(s.visible[s.activeIndex])
		} else if s.Mode == SelectTags && strings.TrimSpace(s.SearchValue) != "" {
			s.applyTagCreate(strings.TrimSpace(s.SearchValue))
		}
		ev.Handled = true
	case "Escape", "Esc":
		s.applyOpen(false, true)
		ev.Handled = true
	case "Backspace":
		// multi/tags: remove last tag when search empty
		if s.Mode != SelectSingle && s.SearchValue == "" && len(s.Values) > 0 {
			last := s.Values[len(s.Values)-1]
			s.toggleMulti(last, false)
			ev.Handled = true
		}
	}
}

// Sync repositions while open.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry (automatic).
func (s *Select) Sync() {
	if s == nil || !s.Open || s.popup == nil {
		return
	}
	s.syncPopupGeometry()
	s.popup.SetOpen(true)
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (s *Select) theme() *core.Theme {
	var n core.Node
	if s.Wrap != nil {
		n = s.Wrap
	}
	return themeOf(s.Theme, n)
}

func (s *Select) controlHeight() float64 {
	th := s.theme()
	switch s.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (s *Select) fontSize() float64 {
	th := s.theme()
	switch s.Size {
	case InputSmall:
		return th.SizeOr(core.TokenFontSizeSM, 12)
	case InputLarge:
		return th.SizeOr(core.TokenFontSizeLG, 16)
	default:
		return th.SizeOr(core.TokenFontSize, 14)
	}
}

func (s *Select) isMulti() bool {
	return s.Mode == SelectMultiple || s.Mode == SelectTags
}

func (s *Select) hasValue() bool {
	if s.isMulti() {
		return len(s.Values) > 0
	}
	return s.Value != ""
}

func (s *Select) rebuild() {
	if s == nil {
		return
	}
	// apply default value once
	if !s.appliedDefVal {
		if s.isMulti() && len(s.Values) == 0 && len(s.DefaultValues) > 0 {
			s.Values = append([]string(nil), s.DefaultValues...)
			s.appliedDefVal = true
		} else if !s.isMulti() && s.Value == "" && s.DefaultValue != "" {
			s.Value = s.DefaultValue
			s.appliedDefVal = true
		}
	}

	th := s.theme()
	h := s.controlHeight()
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	fontSz := s.fontSize()
	wasOpen := s.Open

	// --- field content ---
	content := s.buildSelectorContent(th, fontSz)

	// clear button
	var clearNode core.Node
	if s.AllowClear && s.hasValue() && !s.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		s.clearBtn = primitive.NewPressable(x)
		s.clearBtn.Focusable = false
		s.clearBtn.ShowFocusRing = false
		s.clearBtn.Base().Role = "button"
		s.clearBtn.Base().Label = "clear"
		s.clearBtn.Click = func() { s.Clear() }
		clearNode = s.clearBtn
	} else {
		s.clearBtn = nil
	}

	// suffix: loading spinner or chevron
	var suffixNode core.Node
	s.spinner = nil
	if s.Loading {
		spin := primitive.NewCanvas(14, 14, s.paintSpinner)
		s.spinner = spin
		suffixNode = spin
	} else {
		s.suffix = primitive.NewIcon("chevron-down")
		s.suffix.Size = 12
		sec := th.Color(core.TokenColorTextSecondary)
		if sec.A < 0.3 {
			sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
		}
		s.suffix.Color = sec
		suffixNode = s.suffix
	}

	rowKids := []core.Node{content, primitive.Spacer()}
	if clearNode != nil {
		rowKids = append(rowKids, clearNode)
	}
	if suffixNode != nil {
		rowKids = append(rowKids, suffixNode)
	}
	row := primitive.Row(rowKids...)
	row.CrossAlign = core.CrossCenter
	row.Gap = 8

	s.decor = primitive.NewDecorated(row)
	s.decor.Padding = primitive.Symmetric(padH, 0)
	s.decor.Radius = radius
	s.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	s.decor.MinHeight = h
	s.decor.Height = h
	w := s.FixedWidth
	if w <= 0 {
		w = DefaultSelectWidth
	}
	s.decor.MinWidth = DefaultSelectMinWidth
	s.decor.Width = w
	s.decor.SetCenterContent(true)
	s.decor.StretchChild = true
	s.applyChrome()

	// list + panel
	s.list = primitive.Column()
	s.list.Gap = 2
	s.list.CrossAlign = core.CrossStart
	s.list.Base().Role = "listbox"

	s.panel = primitive.NewDecorated(s.list)
	s.panel.Padding = primitive.All(DefaultSelectPanelPad)
	s.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultSelectPanelRadius)
	s.panel.Background = th.Color(core.TokenColorBgContainer)
	s.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	s.panel.BorderColor = th.Color(core.TokenColorBorder)
	s.panel.MinWidth = DefaultSelectMinWidth
	s.panel.Base().Role = "listbox"

	if s.popup == nil {
		s.popup = primitive.NewAnchoredPopup(s.panel)
	} else {
		s.popup.Content = s.panel
	}
	s.popup.Placement = mapSelectPlacement(s.Placement)
	s.popup.Gap = DefaultSelectGap
	s.popup.DismissOnOutside = true
	s.popup.OnDismiss = func() {
		s.Open = false
		s.SearchValue = ""
		if s.searchEd != nil {
			s.searchEd.Value = ""
		}
		if s.OnOpenChange != nil {
			s.OnOpenChange(false)
		}
		s.applyChrome()
		s.applyA11y()
	}

	// trigger shell
	if s.Root == nil {
		s.Root = primitive.NewPressable(s.decor)
	} else {
		s.Root.ClearChildren()
		s.Root.AddChild(s.decor)
	}
	s.Root.Focusable = true
	s.Root.ShowFocusRing = true
	s.Root.FocusRingRadius = s.decor.Radius
	s.Root.FocusRingOutset = DefaultSelectFocusRingOutset
	s.Root.SetDisabled(s.Disabled)
	s.Root.OnStateChange = func() {
		s.applyChrome()
	}
	s.Root.Click = func() {
		if s.Disabled {
			return
		}
		// clear button handles its own click; trigger toggles open
		s.requestOpen(!s.Open)
	}

	if s.Wrap == nil {
		s.Wrap = primitive.Column(s.Root, s.popup)
	} else {
		s.Wrap.ClearChildren()
		s.Wrap.AddChild(s.Root)
		s.Wrap.AddChild(s.popup)
	}
	s.Wrap.CrossAlign = core.CrossStart
	s.Wrap.Gap = 0
	s.Wrap.SetThemeHook(func(*core.Theme) { s.rebuild() })
	s.Wrap.MarkNeedsLayout()
	s.Wrap.MarkNeedsPaint()

	s.applyA11y()
	s.refreshList(true)

	if wasOpen {
		s.applyOpen(true, false)
	} else if s.defaultOpenSet && !s.openControlled && !s.appliedDefault {
		s.appliedDefault = true
		s.applyOpen(s.defaultOpen, false)
	}
}

func (s *Select) buildSelectorContent(th *core.Theme, fontSz float64) core.Node {
	// Multi: tags row (+ optional search)
	if s.isMulti() {
		return s.buildMultiContent(th, fontSz)
	}

	// Single + showSearch when open: editable search
	if s.ShowSearch && s.Open {
		return s.buildSearchEditor(th, fontSz)
	}

	// Single display label / placeholder
	label := s.displayLabel()
	s.display = primitive.NewText(label)
	s.display.FontSize = fontSz
	s.display.Face = s.Face
	s.display.Color = s.displayColor(th)
	return s.display
}

func (s *Select) buildSearchEditor(th *core.Theme, fontSz float64) core.Node {
	if s.searchEd == nil {
		s.searchEd = primitive.NewEditableText()
	}
	// Seed without firing OnChange.
	was := s.searchEd.OnChange
	s.searchEd.OnChange = nil
	s.searchEd.Value = s.SearchValue
	s.searchEd.Cursor = len([]rune(s.SearchValue))
	s.searchEd.SelAnchor = s.searchEd.Cursor
	s.searchEd.OnChange = was

	s.searchEd.FontSize = fontSz
	s.searchEd.Face = s.Face
	s.searchEd.Color = th.Color(core.TokenColorText)
	s.searchEd.ShowFocusRing = false
	s.searchEd.Placeholder = s.Placeholder
	if s.searchEd.Placeholder == "" {
		s.searchEd.Placeholder = "Search"
	}
	s.searchEd.PlaceholderColor = th.Color(core.TokenColorTextSecondary)
	s.searchEd.OnChange = func(v string) {
		if s.Disabled {
			return
		}
		s.SearchValue = v
		if s.OnSearch != nil {
			s.OnSearch(v)
		}
		s.refreshList(true)
		if !s.Open {
			s.requestOpen(true)
		}
	}
	// Keep a display pointer for tests that look for Text.
	s.display = primitive.NewText(s.SearchValue)
	s.display.FontSize = fontSz
	s.display.Face = s.Face
	s.display.Color = th.Color(core.TokenColorText)
	// Prefer live editor as the interactive content.
	return s.searchEd
}

func (s *Select) buildMultiContent(th *core.Theme, fontSz float64) core.Node {
	row := primitive.Row()
	row.Gap = DefaultSelectTagGap
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainStart

	vals := s.Values
	shown := vals
	rest := 0
	if s.MaxTagCount > 0 && len(vals) > s.MaxTagCount {
		shown = vals[:s.MaxTagCount]
		rest = len(vals) - s.MaxTagCount
	}
	for _, v := range shown {
		lab := s.labelForValue(v)
		tag := NewTag(lab)
		tag.SetFace(s.Face)
		tag.Theme = s.Theme
		if !s.Disabled {
			tag.SetClosable(true)
			vv := v
			tag.SetOnClose(func() {
				s.toggleMulti(vv, false)
			})
		}
		row.AddChild(tag.Node())
	}
	if rest > 0 {
		more := NewTag("+" + strconv.Itoa(rest) + " ...")
		more.SetFace(s.Face)
		more.Theme = s.Theme
		row.AddChild(more.Node())
	}

	// search or placeholder
	if s.ShowSearch && s.Open {
		row.AddChild(s.buildSearchEditor(th, fontSz))
	} else if len(vals) == 0 {
		ph := s.Placeholder
		if ph == "" {
			ph = "Please select"
		}
		s.display = primitive.NewText(ph)
		s.display.FontSize = fontSz
		s.display.Face = s.Face
		s.display.Color = s.displayColor(th)
		row.AddChild(s.display)
	} else {
		// keep a display node for layout tests
		s.display = primitive.NewText("")
		s.display.FontSize = fontSz
		s.display.Face = s.Face
	}
	return row
}

func (s *Select) displayLabel() string {
	if s.Value != "" {
		return s.labelForValue(s.Value)
	}
	ph := s.Placeholder
	if ph == "" {
		ph = "Please select"
	}
	return ph
}

func (s *Select) labelForValue(v string) string {
	for _, o := range s.Options {
		if o.Value == v {
			return o.DisplayLabel()
		}
	}
	return v
}

func (s *Select) displayColor(th *core.Theme) render.RGBA {
	if s.Disabled {
		col := th.Color(core.TokenColorDisabledText)
		if col.A < 0.2 {
			col = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		return col
	}
	if s.hasValue() {
		col := th.Color(core.TokenColorText)
		if col.A < 0.2 {
			col = render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
		}
		return col
	}
	col := th.Color(core.TokenColorTextSecondary)
	if col.A < 0.2 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	return col
}

// refreshLabel is kept for same-package callers (Pagination size changer).
func (s *Select) refreshLabel() { s.refreshDisplay() }

func (s *Select) refreshDisplay() {
	if s == nil {
		return
	}
	// Multi always rebuilds tag row for correctness.
	if s.isMulti() || (s.ShowSearch && s.Open) {
		s.rebuild()
		return
	}
	if s.display == nil {
		return
	}
	th := s.theme()
	s.display.SetValue(s.displayLabel())
	s.display.Color = s.displayColor(th)
	// clear button may need to appear/disappear
	if s.AllowClear {
		s.rebuild()
	}
}

func (s *Select) applyChrome() {
	if s == nil || s.decor == nil {
		return
	}
	th := s.theme()
	h := s.controlHeight()
	s.decor.MinHeight = h
	s.decor.Height = h
	s.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	s.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	switch s.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		s.decor.BorderWidth = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		s.decor.BorderWidth = 0
	case InputUnderlined:
		bg = render.RGBA{}
		s.decor.Radius = 0
	default:
		// outlined
	}

	if s.Disabled {
		dbg := th.Color(core.TokenColorDisabledBg)
		if dbg.A < 0.05 {
			dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bg = dbg
		bd = th.Color(core.TokenColorBorder)
		s.decor.Background = bg
		s.decor.BorderColor = bd
		if s.display != nil {
			s.display.Color = s.displayColor(th)
		}
		s.decor.MarkNeedsPaint()
		return
	}

	switch s.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
	default:
		if s.Root != nil && (s.Root.State.Focused || s.Open) {
			bd = th.Color(core.TokenColorPrimary)
		} else if s.Root != nil && s.Root.State.Hovered {
			hb := th.Color(core.TokenColorBorderHover)
			if hb.A < 0.5 {
				hb = th.Color(core.TokenColorPrimaryHover)
			}
			bd = hb
		}
	}
	if s.Variant == InputUnderlined {
		s.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	}
	s.decor.Background = bg
	s.decor.BorderColor = bd
	if s.display != nil {
		s.display.Color = s.displayColor(th)
	}
	s.decor.MarkNeedsPaint()
}

func (s *Select) applyA11y() {
	if s == nil || s.Root == nil {
		return
	}
	s.Root.Base().Role = "combobox"
	label := s.AriaLabel
	if label == "" {
		label = s.Title
	}
	if label == "" {
		label = s.Placeholder
	}
	if label == "" {
		label = "Select"
	}
	s.Root.Base().Label = label
	if s.Status == InputStatusError {
		s.Root.Base().Label = label + " invalid"
	}
	if s.list != nil {
		s.list.Base().Role = "listbox"
	}
}

func (s *Select) requestOpen(open bool) {
	if s == nil || s.Disabled {
		return
	}
	if s.openControlled {
		if s.OnOpenChange != nil && s.Open != open {
			s.OnOpenChange(open)
		}
		return
	}
	s.applyOpen(open, true)
}

func (s *Select) applyOpen(open bool, notify bool) {
	if s == nil {
		return
	}
	if s.Disabled && open {
		return
	}
	prev := s.Open
	s.Open = open
	if s.popup == nil {
		return
	}
	if open {
		s.refreshList(true)
		s.resetActive()
		s.measurePanel()
		s.syncPopupGeometry()
		s.popup.SetOpen(true)
		// rebuild selector to swap in search editor when showSearch
		if s.ShowSearch && prev != open {
			s.rebuildKeepOpen()
		}
	} else {
		s.popup.SetOpen(false)
		if s.SearchValue != "" {
			s.SearchValue = ""
			if s.searchEd != nil {
				s.searchEd.Value = ""
			}
			// rebuild to restore label when showSearch
			if s.ShowSearch {
				s.rebuild()
				s.Open = false
				if s.popup != nil {
					s.popup.SetOpen(false)
				}
			}
		} else if s.ShowSearch && prev {
			s.rebuild()
			s.Open = false
			if s.popup != nil {
				s.popup.SetOpen(false)
			}
		}
	}
	s.applyChrome()
	s.applyA11y()
	if notify && prev != open && s.OnOpenChange != nil {
		s.OnOpenChange(open)
	}
}

// rebuildKeepOpen rebuilds chrome while keeping popup open (search editor swap).
func (s *Select) rebuildKeepOpen() {
	open := s.Open
	s.rebuild()
	if open && s.popup != nil {
		s.Open = true
		s.syncPopupGeometry()
		s.popup.SetOpen(true)
		s.applyChrome()
	}
}

func (s *Select) syncPopupGeometry() {
	if s == nil || s.popup == nil || s.Root == nil {
		return
	}
	s.popup.UpdateAnchorFromNode(s.Root)
	if s.Viewport.Width > 0 {
		s.popup.Viewport = s.Viewport
	}
}

func (s *Select) resetActive() {
	if s.DefaultActiveFirstOption && len(s.visible) > 0 {
		s.activeIndex = 0
	} else {
		s.activeIndex = -1
	}
	if s.Nav != nil {
		s.Nav.SetCount(len(s.visible))
		if s.activeIndex >= 0 {
			s.Nav.Index = s.activeIndex
		} else {
			s.Nav.Index = 0
		}
	}
}

func (s *Select) matchOption(q string, opt SelectOption) bool {
	if !s.ShowSearch || !s.FilterOptionEnabled {
		return true
	}
	if s.FilterOption != nil {
		return s.FilterOption(q, opt)
	}
	if q == "" {
		return true
	}
	props := s.OptionFilterProp
	if len(props) == 0 {
		props = []string{"value"}
	}
	for _, p := range props {
		switch strings.ToLower(p) {
		case "value":
			if containsFold(opt.Value, q) {
				return true
			}
		case "label":
			if containsFold(opt.DisplayLabel(), q) {
				return true
			}
		case "title":
			if containsFold(opt.Title, q) {
				return true
			}
		case "desc":
			if containsFold(opt.Desc, q) {
				return true
			}
		default:
			if opt.Fields != nil {
				if containsFold(opt.Fields[p], q) {
					return true
				}
			}
		}
	}
	return false
}

func (s *Select) collectVisible() []SelectOption {
	q := s.SearchValue
	var out []SelectOption
	for _, o := range s.Options {
		if o.Disabled {
			// still show disabled options (antd shows them, non-selectable)
			if s.ShowSearch && s.FilterOptionEnabled && q != "" && !s.matchOption(q, o) {
				continue
			}
			out = append(out, o)
			continue
		}
		if s.matchOption(q, o) {
			out = append(out, o)
		}
	}
	if s.FilterSort != nil {
		sort.SliceStable(out, func(i, j int) bool {
			return s.FilterSort(out[i], out[j]) < 0
		})
	}
	return out
}

func (s *Select) refreshList(refilter bool) {
	if s == nil || s.list == nil {
		return
	}
	_ = refilter
	s.visible = s.collectVisible()
	if s.Nav != nil {
		s.Nav.SetCount(len(s.visible))
		if s.activeIndex >= len(s.visible) {
			s.activeIndex = len(s.visible) - 1
		}
		if s.DefaultActiveFirstOption && s.activeIndex < 0 && len(s.visible) > 0 && s.Open {
			s.activeIndex = 0
		}
		if s.activeIndex >= 0 {
			s.Nav.Index = s.activeIndex
		}
	}

	th := s.theme()
	s.list.ClearChildren()

	if s.Loading && len(s.visible) == 0 {
		spin := primitive.NewCanvas(16, 16, s.paintSpinner)
		row := primitive.Row(spin)
		row.MainAlign = core.MainCenter
		row.CrossAlign = core.CrossCenter
		row.Padding = primitive.Symmetric(12, 8)
		s.list.AddChild(row)
	} else if len(s.visible) == 0 {
		msg := s.NotFoundContent
		if msg == "" {
			msg = "Not Found"
		}
		lab := primitive.NewText(msg)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = s.Face
		lab.Color = th.Color(core.TokenColorTextSecondary)
		pad := primitive.NewDecorated(lab)
		pad.Padding = primitive.Symmetric(DefaultSelectItemPadInline, DefaultSelectItemPadBlock)
		pad.Hit = core.HitTransparent
		s.list.AddChild(pad)
	} else {
		for i, opt := range s.visible {
			s.list.AddChild(s.buildOptionRow(opt, i, th))
		}
	}

	if s.popup != nil && s.Open {
		s.measurePanel()
	}
	s.list.MarkNeedsLayout()
	s.list.MarkNeedsPaint()
}

func (s *Select) buildOptionRow(opt SelectOption, idx int, th *core.Theme) core.Node {
	var content core.Node
	if s.OptionRender != nil {
		content = s.OptionRender(opt)
	}
	if content == nil && opt.LabelNode != nil {
		content = opt.LabelNode
	}
	if content == nil {
		if opt.Emoji != "" || opt.Desc != "" {
			parts := []core.Node{}
			if opt.Emoji != "" {
				em := primitive.NewText(opt.Emoji)
				em.FontSize = th.SizeOr(core.TokenFontSize, 14)
				em.Face = s.Face
				parts = append(parts, em)
			}
			lab := primitive.NewText(opt.DisplayLabel())
			lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
			lab.Face = s.Face
			lab.Color = th.Color(core.TokenColorText)
			if opt.Desc != "" {
				lab.SetValue(opt.DisplayLabel() + " (" + opt.Desc + ")")
			}
			parts = append(parts, lab)
			row := primitive.Row(parts...)
			row.Gap = 8
			row.CrossAlign = core.CrossCenter
			content = row
		} else {
			lab := primitive.NewText(opt.DisplayLabel())
			lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
			lab.Face = s.Face
			lab.Color = th.Color(core.TokenColorText)
			if opt.Disabled {
				lab.Color = th.Color(core.TokenColorDisabledText)
			}
			content = lab
		}
	}

	item := primitive.NewPressable(content)
	item.Padding = primitive.Symmetric(DefaultSelectItemPadInline, DefaultSelectItemPadBlock)
	item.ShowFocusRing = false
	item.SetDisabled(opt.Disabled)

	selected := s.isSelected(opt.Value)
	if selected {
		item.Color = antItemSelectedFill(th)
	}
	if idx == s.activeIndex && !opt.Disabled {
		if !selected {
			item.Color = antItemHoverFill(th)
		}
	}
	item.ColorHovered = antItemHoverFill(th)

	optCopy := opt
	item.Click = func() {
		if s.Disabled || optCopy.Disabled {
			return
		}
		s.applySelect(optCopy)
	}
	item.Base().Role = "option"
	item.Base().Label = opt.DisplayLabel()
	if opt.Title != "" {
		item.Base().Label = opt.Title
	}
	return item
}

func (s *Select) isSelected(v string) bool {
	if s.isMulti() {
		for _, x := range s.Values {
			if x == v {
				return true
			}
		}
		return false
	}
	return s.Value == v
}

func (s *Select) applySelect(opt SelectOption) {
	if s == nil || s.Disabled || opt.Disabled {
		return
	}
	if s.isMulti() {
		if s.isSelected(opt.Value) {
			s.toggleMulti(opt.Value, false)
		} else {
			s.toggleMulti(opt.Value, true)
		}
		return
	}
	// single
	if !s.Controlled {
		s.Value = opt.Value
	}
	if s.OnSelect != nil {
		s.OnSelect(opt.Value, opt)
	}
	if s.OnChange != nil {
		s.OnChange(opt.Value)
	}
	s.SearchValue = ""
	s.applyOpen(false, true)
	s.refreshDisplay()
	s.refreshList(true)
}

func (s *Select) applyTagCreate(text string) {
	if s == nil || s.Mode != SelectTags || text == "" {
		return
	}
	// if already an option or selected, just select
	for _, o := range s.Options {
		if o.Value == text || o.DisplayLabel() == text {
			s.applySelect(o)
			return
		}
	}
	if s.isSelected(text) {
		return
	}
	// create ephemeral option and select
	opt := SelectOption{Value: text, Label: text}
	s.Options = append(s.Options, opt)
	s.toggleMulti(text, true)
}

func (s *Select) toggleMulti(value string, add bool) {
	if s == nil || s.Disabled {
		return
	}
	opt := s.findOption(value)
	var next []string
	if add {
		if s.isSelected(value) {
			return
		}
		next = append(append([]string(nil), s.Values...), value)
	} else {
		for _, v := range s.Values {
			if v != value {
				next = append(next, v)
			}
		}
	}
	if !s.Controlled {
		s.Values = next
		if len(next) > 0 {
			s.Value = next[len(next)-1]
		} else {
			s.Value = ""
		}
	}
	if add {
		if s.OnSelect != nil {
			s.OnSelect(value, opt)
		}
	} else {
		if s.OnDeselect != nil {
			s.OnDeselect(value, opt)
		}
	}
	if s.OnChangeMulti != nil {
		s.OnChangeMulti(append([]string(nil), next...))
	}
	// also fire OnChange with last for simple listeners (pagination-style)
	if s.OnChange != nil {
		if len(next) > 0 {
			s.OnChange(next[len(next)-1])
		} else {
			s.OnChange("")
		}
	}
	s.SearchValue = ""
	if s.searchEd != nil {
		s.searchEd.Value = ""
	}
	// multi stays open
	s.refreshDisplay()
	s.refreshList(true)
	s.applyChrome()
}

// Choose selects an option by value (user path: fires OnChange / OnSelect).
// Used by hosts and tests when pointer hit on portal options is unavailable.
func (s *Select) Choose(value string) {
	if s == nil || s.Disabled {
		return
	}
	s.applySelect(s.findOption(value))
}

func (s *Select) findOption(value string) SelectOption {
	for _, o := range s.Options {
		if o.Value == value {
			return o
		}
	}
	return SelectOption{Value: value, Label: value}
}

func (s *Select) measurePanel() {
	if s == nil || s.panel == nil {
		return
	}
	w := DefaultSelectMinWidth
	if s.PopupMatchSelectWidth {
		if s.Root != nil {
			if sz := s.Root.Base().Size(); sz.Width > 0 {
				w = sz.Width
			}
		}
		if s.decor != nil {
			if s.decor.Width > w {
				w = s.decor.Width
			}
		}
		if s.FixedWidth > w {
			w = s.FixedWidth
		}
	}
	s.panel.MinWidth = w
	if s.PopupMatchSelectWidth {
		s.panel.Width = w
	}
	// ListHeight is product default 256 (virtual scroll / hard clip is P1).
	_ = s.ListHeight
}

func (s *Select) paintSpinner(pc *core.PaintContext, size core.Size) {
	th := s.theme()
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
	start := -math.Pi/2 + s.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func mapSelectPlacement(p SelectPlacement) primitive.Placement {
	switch p {
	case SelectBottomRight:
		return primitive.PlaceBottomEnd
	case SelectTopLeft:
		return primitive.PlaceTopStart
	case SelectTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}
