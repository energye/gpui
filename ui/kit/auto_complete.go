package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design AutoComplete defaults — docs/antd/auto-complete.md §6.2 / §6.10
// https://ant.design/components/auto-complete
const (
	DefaultAutoCompleteGap             = 4.0
	DefaultAutoCompletePanelPad        = 4.0
	DefaultAutoCompleteItemPadInline   = 12.0
	DefaultAutoCompleteItemPadBlock    = 5.0
	DefaultAutoCompletePanelRadius     = 8.0
	DefaultAutoCompleteGroupTitlePad   = 8.0
	DefaultAutoCompleteMinWidth        = 200.0
	DefaultAutoCompleteMaxListHeight   = 256.0
	DefaultAutoCompleteFocusRingOutset = 1.5
)

// AutoCompleteOption is one suggestion (antd options[] item).
// When Options is non-empty the entry is a group header with nested children.
type AutoCompleteOption struct {
	Value     string
	Label     string // empty → Value
	Disabled  bool
	Extra     string               // trailing meta (category demos)
	Options   []AutoCompleteOption // non-empty = group
	LabelNode core.Node            // optional custom label content
}

// DisplayLabel returns Label or falls back to Value.
func (o AutoCompleteOption) DisplayLabel() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

// AutoCompleteFilterFunc matches antd filterOption function form.
// Return true to keep the option visible.
type AutoCompleteFilterFunc func(input string, option AutoCompleteOption) bool

// AutoComplete is Input + anchored suggestion list (antd AutoComplete).
//
//	Column (Wrap)
//	  ├─ Input | children
//	  └─ AnchoredPopup
//	       └─ panel (options | notFound | loading)
//
// Product contract: docs/antd/auto-complete.md §6 (P0 DoD).
type AutoComplete struct {
	Wrap  *primitive.Flex
	input *Input
	popup *primitive.AnchoredPopup
	panel *primitive.Decorated
	list  *primitive.Flex
	Nav   *core.KeyboardNav

	// Product fields (§6.10).
	Value                    string
	DefaultValue             string
	Placeholder              string
	Options                  []AutoCompleteOption
	Size                     InputSize
	Variant                  InputVariant
	Status                   InputStatus
	Disabled                 bool
	AllowClear               bool
	Open                     bool
	NotFoundContent          string
	DefaultActiveFirstOption bool
	PopupMatchSelectWidth    bool
	Loading                  bool
	AriaLabel                string
	Face                     text.Face
	Theme                    *core.Theme
	Viewport                 core.Size
	// FixedWidth forces trigger width (0 = DefaultAutoCompleteMinWidth).
	FixedWidth float64

	// FilterOptionEnabled defaults true. When false, all leaf options are shown
	// (caller usually feeds options via OnSearch).
	FilterOptionEnabled bool
	// FilterOption custom predicate; nil → containsFold on value/label.
	FilterOption AutoCompleteFilterFunc

	// Controlled: typing only raises OnChange; display stays until SetValue.
	Controlled bool

	// Callbacks.
	OnChange     func(value string)
	OnSearch     func(value string)
	OnSelect     func(value string, option AutoCompleteOption)
	OnOpenChange func(open bool)
	OnClear      func()

	// openControlled: SetOpen was used (antd open prop).
	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool

	// children: custom input node (Search / TextArea / any). When set, default
	// Input is still used for value/chrome if childrenInput is non-nil.
	childrenNode  core.Node
	childrenInput *Input // optional *Input extracted from children

	// Flattened visible leaves for keyboard / select.
	visible []AutoCompleteOption
	// activeIndex into visible (-1 = none).
	activeIndex int

	// Loading spinner.
	spinner   *primitive.Canvas
	spinPhase float64
	boundTree *core.Tree

	// selecting suppresses onSearch when selection writes value.
	selecting bool

	// searchText is the last typed query used for filtering (may differ from
	// Value briefly in controlled mode before parent SetValue).
	searchText string
}

// NewAutoComplete creates an AutoComplete with optional string option values.
// Defaults (§6.10): middle/outlined, filter on, defaultActiveFirstOption=true,
// popupMatchSelectWidth=true, closed, uncontrolled.
func NewAutoComplete(placeholder string, optionValues ...string) *AutoComplete {
	a := &AutoComplete{
		Placeholder:              placeholder,
		Size:                     InputMiddle,
		Variant:                  InputOutlined,
		Status:                   InputStatusNone,
		FilterOptionEnabled:      true,
		DefaultActiveFirstOption: true,
		PopupMatchSelectWidth:    true,
		activeIndex:              -1,
	}
	if len(optionValues) > 0 {
		a.Options = stringsToOptions(optionValues)
	}
	a.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	a.rebuild()
	return a
}

// Node returns the composition root (field + popup host).
func (a *AutoComplete) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Wrap == nil {
		a.rebuild()
	}
	return a.Wrap
}

// Input returns the active field (children Input if set, else default).
func (a *AutoComplete) Input() *Input {
	if a == nil {
		return nil
	}
	if a.childrenInput != nil {
		return a.childrenInput
	}
	return a.input
}

// Popup returns the anchored popup (tests / advanced hosts).
func (a *AutoComplete) Popup() *primitive.AnchoredPopup {
	if a == nil {
		return nil
	}
	return a.popup
}

// Panel returns the dropdown panel chrome (tests).
func (a *AutoComplete) Panel() *primitive.Decorated {
	if a == nil {
		return nil
	}
	return a.panel
}

// IsOpen reports whether the suggestion panel is visible.
func (a *AutoComplete) IsOpen() bool {
	return a != nil && a.Open && a.shouldShowPopup()
}

// VisibleOptions returns the filtered leaf options currently shown.
func (a *AutoComplete) VisibleOptions() []AutoCompleteOption {
	if a == nil {
		return nil
	}
	out := make([]AutoCompleteOption, len(a.visible))
	copy(out, a.visible)
	return out
}

// ActiveIndex returns the highlighted option index (-1 if none).
func (a *AutoComplete) ActiveIndex() int {
	if a == nil {
		return -1
	}
	return a.activeIndex
}

// GetValue returns the current value.
func (a *AutoComplete) GetValue() string {
	if a == nil {
		return ""
	}
	return a.Value
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetValue sets the displayed value. Does not fire OnChange (parent/API write).
// Typing and select paths fire OnChange themselves.
func (a *AutoComplete) SetValue(v string) {
	if a == nil {
		return
	}
	a.Value = v
	a.searchText = v
	if in := a.Input(); in != nil {
		cb := in.OnChange
		in.OnChange = nil
		in.SetValue(v)
		in.OnChange = cb
	}
	a.refreshList(true)
}

// SetDefaultValue seeds uncontrolled value when Value is still empty.
func (a *AutoComplete) SetDefaultValue(v string) {
	if a == nil {
		return
	}
	a.DefaultValue = v
	if !a.Controlled && a.Value == "" {
		a.Value = v
		if in := a.Input(); in != nil {
			was := in.OnChange
			in.OnChange = nil
			in.SetValue(v)
			in.OnChange = was
		}
	}
}

// SetPlaceholder updates the placeholder on the active input.
func (a *AutoComplete) SetPlaceholder(s string) {
	if a == nil {
		return
	}
	a.Placeholder = s
	if in := a.Input(); in != nil {
		in.SetPlaceholder(s)
	}
}

// SetOptions replaces the options model (groups allowed).
func (a *AutoComplete) SetOptions(opts []AutoCompleteOption) {
	if a == nil {
		return
	}
	a.Options = append([]AutoCompleteOption(nil), opts...)
	a.refreshList(true)
}

// SetOptionValues is a convenience for plain string options.
func (a *AutoComplete) SetOptionValues(vals ...string) {
	if a == nil {
		return
	}
	a.SetOptions(stringsToOptions(vals))
}

// SetDisabled toggles disabled (no edit / no open).
func (a *AutoComplete) SetDisabled(d bool) {
	if a == nil {
		return
	}
	a.Disabled = d
	if in := a.Input(); in != nil {
		in.SetDisabled(d)
	}
	if d && a.Open {
		a.applyOpen(false, false)
	}
	a.applyA11y()
}

// SetSize updates control size on the active input.
func (a *AutoComplete) SetSize(s InputSize) {
	if a == nil {
		return
	}
	a.Size = s
	if in := a.Input(); in != nil {
		in.SetSize(s)
	}
}

// SetVariant updates visual variant on the active input.
func (a *AutoComplete) SetVariant(v InputVariant) {
	if a == nil {
		return
	}
	a.Variant = v
	if in := a.Input(); in != nil {
		in.SetVariant(v)
	}
}

// SetStatus updates validation chrome.
func (a *AutoComplete) SetStatus(s InputStatus) {
	if a == nil {
		return
	}
	a.Status = s
	if in := a.Input(); in != nil {
		in.SetStatus(s)
	}
	a.applyA11y()
}

// SetAllowClear toggles clear affix on the active input.
func (a *AutoComplete) SetAllowClear(v bool) {
	if a == nil {
		return
	}
	a.AllowClear = v
	if in := a.Input(); in != nil {
		in.SetAllowClear(v)
	}
}

// SetOpen sets visibility and marks controlled (antd open prop).
// Empty filtered options never show the panel (antd FAQ).
func (a *AutoComplete) SetOpen(open bool) {
	if a == nil {
		return
	}
	a.openControlled = true
	a.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (a *AutoComplete) SetDefaultOpen(open bool) {
	if a == nil || a.openControlled {
		return
	}
	a.defaultOpen = open
	a.defaultOpenSet = true
	if !a.appliedDefault {
		a.appliedDefault = true
		a.applyOpen(open, false)
	}
}

// SetFilterOption enables/disables default filtering (antd filterOption bool).
func (a *AutoComplete) SetFilterOption(enabled bool) {
	if a == nil {
		return
	}
	a.FilterOptionEnabled = enabled
	a.refreshList(true)
}

// SetFilterOptionFunc sets a custom filter (enables filtering).
func (a *AutoComplete) SetFilterOptionFunc(fn AutoCompleteFilterFunc) {
	if a == nil {
		return
	}
	a.FilterOption = fn
	a.FilterOptionEnabled = true
	a.refreshList(true)
}

// SetDefaultActiveFirstOption toggles first-option highlight on open.
func (a *AutoComplete) SetDefaultActiveFirstOption(v bool) {
	if a == nil {
		return
	}
	a.DefaultActiveFirstOption = v
}

// SetNotFoundContent sets empty-list content (shown only when open + empty leaves + set).
func (a *AutoComplete) SetNotFoundContent(s string) {
	if a == nil {
		return
	}
	a.NotFoundContent = s
	a.refreshList(false)
}

// SetChildren replaces the default input with a custom field.
// Accepts *Input, *Search, *TextArea, *Password, or any core.Node.
// When a kit Input subtype is passed, value/disabled/size sync through it.
func (a *AutoComplete) SetChildren(n any) {
	if a == nil {
		return
	}
	a.childrenInput = nil
	a.childrenNode = nil
	switch v := n.(type) {
	case nil:
		// restore default input
	case *Input:
		a.childrenInput = v
		a.childrenNode = v.Node()
	case *Search:
		a.childrenInput = v.Input
		a.childrenNode = v.Node()
	case *TextArea:
		a.childrenInput = v.Input
		a.childrenNode = v.Node()
	case *Password:
		a.childrenInput = v.Input
		a.childrenNode = v.Node()
	case core.Node:
		a.childrenNode = v
		a.childrenInput = extractInput(v)
	}
	a.rebuild()
}

// SetPopupMatchSelectWidth toggles panel min-width matching the trigger.
func (a *AutoComplete) SetPopupMatchSelectWidth(v bool) {
	if a == nil {
		return
	}
	a.PopupMatchSelectWidth = v
	a.measurePanel()
}

// SetLoading toggles async-search spinner in the panel (Ticker).
func (a *AutoComplete) SetLoading(v bool) {
	if a == nil || a.Loading == v {
		return
	}
	a.Loading = v
	a.refreshList(false)
	if a.boundTree != nil {
		if v {
			a.boundTree.AddTicker(a)
		} else {
			a.boundTree.RemoveTicker(a)
		}
	}
}

// SetFixedWidth forces the default input width.
func (a *AutoComplete) SetFixedWidth(w float64) {
	if a == nil {
		return
	}
	a.FixedWidth = w
	if in := a.Input(); in != nil {
		in.SetFixedSize(w, 0)
	}
}

// SetControlled toggles controlled value mode.
func (a *AutoComplete) SetControlled(v bool) {
	if a == nil {
		return
	}
	a.Controlled = v
	if in := a.Input(); in != nil {
		in.SetControlled(v)
	}
}

// SetOnChange sets the value-change callback.
func (a *AutoComplete) SetOnChange(fn func(string)) {
	if a == nil {
		return
	}
	a.OnChange = fn
}

// SetOnSearch sets the search callback (not fired on option click).
func (a *AutoComplete) SetOnSearch(fn func(string)) {
	if a == nil {
		return
	}
	a.OnSearch = fn
}

// SetOnSelect sets the option-select callback.
func (a *AutoComplete) SetOnSelect(fn func(value string, option AutoCompleteOption)) {
	if a == nil {
		return
	}
	a.OnSelect = fn
}

// SetOnOpenChange sets the open-change callback.
func (a *AutoComplete) SetOnOpenChange(fn func(open bool)) {
	if a == nil {
		return
	}
	a.OnOpenChange = fn
}

// SetOnClear sets the clear-button callback.
func (a *AutoComplete) SetOnClear(fn func()) {
	if a == nil {
		return
	}
	a.OnClear = fn
	if in := a.Input(); in != nil {
		in.SetOnClear(fn)
	}
}

// SetTheme sets an explicit theme override.
func (a *AutoComplete) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	if in := a.Input(); in != nil {
		in.SetTheme(th)
	}
	a.rebuild()
}

// SetFace sets the font face.
func (a *AutoComplete) SetFace(face text.Face) {
	if a == nil {
		return
	}
	a.Face = face
	if in := a.Input(); in != nil {
		in.SetFace(face)
	}
	a.refreshList(false)
}

// SetAriaLabel sets the accessible name.
func (a *AutoComplete) SetAriaLabel(s string) {
	if a == nil {
		return
	}
	a.AriaLabel = s
	a.applyA11y()
}

// AttachTicker registers loading spinner animation.
func (a *AutoComplete) AttachTicker(t *core.Tree) {
	if a == nil || t == nil {
		return
	}
	a.boundTree = t
	if in := a.Input(); in != nil {
		in.AttachTicker(t)
	}
	t.BindTicker(a, a.Loading)
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (a *AutoComplete) Tick(dt float64) bool {
	if a == nil || !a.Loading {
		return false
	}
	a.spinPhase += dt * 1.4
	if a.spinPhase > 1 {
		a.spinPhase -= 1
	}
	if a.spinner != nil {
		a.spinner.MarkNeedsPaint()
	} else if a.panel != nil {
		a.panel.MarkNeedsPaint()
	}
	return a.Loading
}

// HandleKey processes arrow/enter/escape when the field is focused.
func (a *AutoComplete) HandleKey(ev *core.KeyEvent) {
	if a == nil || a.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !a.IsOpen() {
		if ev.Key == "ArrowDown" || ev.Key == "Down" {
			a.requestOpen(true)
			ev.Handled = true
		}
		return
	}
	if a.Nav != nil && a.Nav.HandleKey(ev.Key) {
		a.activeIndex = a.Nav.Index
		a.refreshList(false)
		ev.Handled = true
		return
	}
	switch ev.Key {
	case "Enter":
		if a.activeIndex >= 0 && a.activeIndex < len(a.visible) {
			a.applySelect(a.visible[a.activeIndex])
		}
		ev.Handled = true
	case "Escape", "Esc":
		// Always close visually (same as outside dismiss); notify parent.
		a.applyOpen(false, true)
		ev.Handled = true
	}
}

// Clear empties the value (allowClear path / API).
func (a *AutoComplete) Clear() {
	if a == nil || a.Disabled {
		return
	}
	if a.Controlled {
		if a.OnChange != nil {
			a.OnChange("")
		}
	} else {
		a.Value = ""
		if in := a.Input(); in != nil {
			was := in.OnChange
			in.OnChange = nil
			in.SetValue("")
			in.OnChange = was
		}
		if a.OnChange != nil {
			a.OnChange("")
		}
	}
	if a.OnClear != nil {
		a.OnClear()
	}
	a.refreshList(true)
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (a *AutoComplete) theme() *core.Theme {
	var n core.Node
	if a.Wrap != nil {
		n = a.Wrap
	}
	return themeOf(a.Theme, n)
}

func (a *AutoComplete) rebuild() {
	if a == nil {
		return
	}
	th := a.theme()
	wasOpen := a.Open

	// Default input (always kept; children may replace visual host).
	if a.input == nil {
		a.input = NewInput(a.Placeholder)
	}
	a.input.Theme = a.Theme
	a.input.SetFace(a.Face)
	a.input.SetSize(a.Size)
	a.input.SetVariant(a.Variant)
	a.input.SetStatus(a.Status)
	a.input.SetDisabled(a.Disabled)
	a.input.SetAllowClear(a.AllowClear)
	a.input.SetControlled(a.Controlled)
	a.input.SetPlaceholder(a.Placeholder)
	if a.FixedWidth > 0 {
		a.input.SetFixedSize(a.FixedWidth, 0)
	} else {
		a.input.SetFixedSize(DefaultAutoCompleteMinWidth, 0)
	}
	// Seed value silently.
	{
		was := a.input.OnChange
		a.input.OnChange = nil
		a.input.SetValue(a.Value)
		a.input.OnChange = was
	}
	a.wireInput(a.input)

	// Children input wiring.
	if a.childrenInput != nil {
		ci := a.childrenInput
		ci.Theme = a.Theme
		ci.SetFace(a.Face)
		if a.Size != InputMiddle {
			ci.SetSize(a.Size)
		}
		ci.SetDisabled(a.Disabled)
		ci.SetControlled(a.Controlled)
		if a.Placeholder != "" && ci.Placeholder == "" {
			ci.SetPlaceholder(a.Placeholder)
		}
		{
			was := ci.OnChange
			ci.OnChange = nil
			ci.SetValue(a.Value)
			ci.OnChange = was
		}
		a.wireInput(ci)
	}

	// List + panel.
	a.list = primitive.Column()
	a.list.Gap = 2
	a.list.CrossAlign = core.CrossStart
	a.panel = primitive.NewDecorated(a.list)
	a.panel.Padding = primitive.All(DefaultAutoCompletePanelPad)
	a.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultAutoCompletePanelRadius)
	a.panel.Background = th.Color(core.TokenColorBgContainer)
	a.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	a.panel.BorderColor = th.Color(core.TokenColorBorder)
	a.panel.MinWidth = DefaultAutoCompleteMinWidth

	a.popup = primitive.NewAnchoredPopup(a.panel)
	a.popup.Placement = primitive.PlaceBottomStart
	a.popup.Gap = DefaultAutoCompleteGap
	a.popup.Portal.ID = ""
	a.popup.DismissOnOutside = true
	a.popup.OnDismiss = func() {
		a.Open = false
		if a.OnOpenChange != nil {
			a.OnOpenChange(false)
		}
		a.applyA11y()
	}

	// Host node: children or default input.
	var field core.Node
	if a.childrenNode != nil {
		field = a.childrenNode
	} else {
		field = a.input.Node()
	}
	a.popup.AnchorNode = field

	if a.Wrap == nil {
		a.Wrap = primitive.Column(field, a.popup)
	} else {
		a.Wrap.ClearChildren()
		a.Wrap.AddChild(field)
		a.Wrap.AddChild(a.popup)
	}
	a.Wrap.CrossAlign = core.CrossStart
	a.Wrap.Gap = 0
	a.Wrap.SetThemeHook(func(*core.Theme) { a.rebuild() })
	a.Wrap.MarkNeedsLayout()
	a.Wrap.MarkNeedsPaint()

	a.applyA11y()
	a.refreshList(false)

	if wasOpen {
		a.applyOpen(true, false)
	} else if a.defaultOpenSet && !a.openControlled && !a.appliedDefault {
		a.appliedDefault = true
		a.applyOpen(a.defaultOpen, false)
	}
}

func (a *AutoComplete) wireInput(in *Input) {
	if in == nil {
		return
	}
	in.SetOnChange(func(v string) {
		if a.Disabled || a.selecting {
			return
		}
		a.searchText = v
		if !a.Controlled {
			a.Value = v
		}
		if a.OnChange != nil {
			a.OnChange(v)
		}
		if a.OnSearch != nil {
			a.OnSearch(v)
		}
		a.refreshList(true)
		if len(a.visible) > 0 || a.Loading || a.NotFoundContent != "" {
			a.requestOpen(true)
		} else {
			a.requestOpen(false)
		}
	})
	in.SetOnClear(func() {
		if a.Disabled {
			return
		}
		a.searchText = ""
		if !a.Controlled {
			a.Value = ""
		}
		if a.OnClear != nil {
			a.OnClear()
		}
		// Input clear already fired OnChange("") on the input; surface at AC level too
		// only when Input did not (controlled input still fires).
		a.refreshList(true)
		a.requestOpen(false)
	})
	// Wire Enter on input to HandleKey-compatible select when open.
	in.SetOnPressEnter(func(string) {
		if a.Disabled {
			return
		}
		if a.IsOpen() && a.activeIndex >= 0 && a.activeIndex < len(a.visible) {
			a.applySelect(a.visible[a.activeIndex])
		}
	})
}

func (a *AutoComplete) filterQuery() string {
	if a == nil {
		return ""
	}
	// Prefer live editor text when uncontrolled.
	if !a.Controlled {
		if in := a.Input(); in != nil && in.Editor() != nil {
			return in.Editor().Value
		}
	}
	if a.searchText != "" {
		return a.searchText
	}
	return a.Value
}

func (a *AutoComplete) matchOption(q string, opt AutoCompleteOption) bool {
	if !a.FilterOptionEnabled {
		return true
	}
	if a.FilterOption != nil {
		return a.FilterOption(q, opt)
	}
	// Default: case-insensitive contains on value or label.
	if q == "" {
		// antd with static options often shows all on empty; basic demo uses onSearch
		// which returns []. For static lists (non-case-sensitive), empty shows all.
		return true
	}
	return containsFold(opt.Value, q) || containsFold(opt.DisplayLabel(), q)
}

func (a *AutoComplete) collectVisible() []AutoCompleteOption {
	q := a.filterQuery()
	var out []AutoCompleteOption
	var walk func(opts []AutoCompleteOption)
	walk = func(opts []AutoCompleteOption) {
		for _, o := range opts {
			if len(o.Options) > 0 {
				// Group: include children that match; group itself is not selectable.
				for _, c := range o.Options {
					if c.Disabled {
						continue
					}
					if a.matchOption(q, c) {
						out = append(out, c)
					}
				}
				continue
			}
			if o.Disabled {
				continue
			}
			if a.matchOption(q, o) {
				out = append(out, o)
			}
		}
	}
	walk(a.Options)
	return out
}

func (a *AutoComplete) shouldShowPopup() bool {
	if a == nil || a.Disabled {
		return false
	}
	if a.Loading {
		return true
	}
	if len(a.visible) > 0 {
		return true
	}
	// Empty options: only show when notFoundContent is set (antd FAQ: empty options hide).
	return a.NotFoundContent != "" && a.Open
}

func (a *AutoComplete) requestOpen(open bool) {
	if a.openControlled {
		// Notify parent; display follows SetOpen from outside.
		if a.OnOpenChange != nil && a.Open != open {
			a.OnOpenChange(open)
		}
		return
	}
	a.applyOpen(open, true)
}

func (a *AutoComplete) applyOpen(open bool, notify bool) {
	if a == nil {
		return
	}
	// Product Open stores intent; actual panel visibility is shouldShowPopup.
	prev := a.Open
	a.Open = open
	if a.popup == nil {
		return
	}
	show := open && a.shouldShowPopup()
	// When open requested but nothing to show, keep Open intent for controlled
	// FAQ tests, but hide panel.
	if open && !show && a.NotFoundContent == "" && !a.Loading {
		// FAQ: empty options → no panel even if open=true.
		a.popup.SetOpen(false)
	} else {
		if show {
			a.measurePanel()
			if a.popup.AnchorNode != nil {
				a.popup.UpdateAnchorFromNode(a.popup.AnchorNode)
			}
			if a.Viewport.Width > 0 {
				a.popup.Viewport = a.Viewport
			}
			// Reset active index on open.
			if open && !prev {
				a.resetActive()
			}
		}
		a.popup.SetOpen(show)
	}
	if notify && prev != open && a.OnOpenChange != nil {
		a.OnOpenChange(open)
	}
	a.applyA11y()
	if a.list != nil {
		a.refreshList(false)
	}
}

func (a *AutoComplete) resetActive() {
	if a.DefaultActiveFirstOption && len(a.visible) > 0 {
		a.activeIndex = 0
	} else {
		a.activeIndex = -1
	}
	if a.Nav != nil {
		a.Nav.SetCount(len(a.visible))
		if a.activeIndex >= 0 {
			a.Nav.Index = a.activeIndex
		} else {
			a.Nav.Index = 0
		}
	}
}

func (a *AutoComplete) refreshList(refilter bool) {
	if a == nil || a.list == nil {
		return
	}
	if refilter || a.visible == nil {
		a.visible = a.collectVisible()
	} else {
		// Always refilter from current options/query for correctness.
		a.visible = a.collectVisible()
	}
	if a.Nav != nil {
		a.Nav.SetCount(len(a.visible))
		if a.activeIndex >= len(a.visible) {
			a.activeIndex = len(a.visible) - 1
		}
		if a.DefaultActiveFirstOption && a.activeIndex < 0 && len(a.visible) > 0 && a.Open {
			a.activeIndex = 0
		}
		if a.activeIndex >= 0 {
			a.Nav.Index = a.activeIndex
		}
	}

	th := a.theme()
	a.list.ClearChildren()
	a.spinner = nil

	if a.Loading {
		spin := primitive.NewCanvas(16, 16, a.paintSpinner)
		a.spinner = spin
		row := primitive.Row(spin)
		row.MainAlign = core.MainCenter
		row.CrossAlign = core.CrossCenter
		row.Padding = primitive.Symmetric(12, 8)
		a.list.AddChild(row)
	}

	// Render groups with titles when present; otherwise flat leaves.
	hasGroup := false
	for _, o := range a.Options {
		if len(o.Options) > 0 {
			hasGroup = true
			break
		}
	}
	if hasGroup {
		q := a.filterQuery()
		for _, g := range a.Options {
			if len(g.Options) == 0 {
				if !g.Disabled && a.matchOption(q, g) {
					a.list.AddChild(a.buildOptionRow(g, indexOfVisible(a.visible, g), th))
				}
				continue
			}
			// Collect matching children.
			var kids []AutoCompleteOption
			for _, c := range g.Options {
				if !c.Disabled && a.matchOption(q, c) {
					kids = append(kids, c)
				}
			}
			if len(kids) == 0 {
				continue
			}
			title := g.DisplayLabel()
			if title != "" {
				lab := primitive.NewText(title)
				lab.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
				lab.Face = a.Face
				lab.Color = th.Color(core.TokenColorTextSecondary)
				pad := primitive.NewDecorated(lab)
				pad.Padding = primitive.Symmetric(DefaultAutoCompleteGroupTitlePad, 4)
				pad.Hit = core.HitTransparent
				a.list.AddChild(pad)
			}
			for _, c := range kids {
				a.list.AddChild(a.buildOptionRow(c, indexOfVisible(a.visible, c), th))
			}
		}
	} else if len(a.visible) == 0 {
		if a.NotFoundContent != "" && !a.Loading {
			lab := primitive.NewText(a.NotFoundContent)
			lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
			lab.Face = a.Face
			lab.Color = th.Color(core.TokenColorTextSecondary)
			pad := primitive.NewDecorated(lab)
			pad.Padding = primitive.Symmetric(DefaultAutoCompleteItemPadInline, DefaultAutoCompleteItemPadBlock)
			pad.Hit = core.HitTransparent
			a.list.AddChild(pad)
		}
	} else {
		for i, opt := range a.visible {
			a.list.AddChild(a.buildOptionRow(opt, i, th))
		}
	}

	// Keep popup visibility in sync when options become empty/non-empty.
	if a.popup != nil {
		show := a.Open && a.shouldShowPopup()
		if a.popup.Open != show {
			if show {
				a.measurePanel()
				if a.popup.AnchorNode != nil {
					a.popup.UpdateAnchorFromNode(a.popup.AnchorNode)
				}
			}
			a.popup.SetOpen(show)
		} else if show {
			a.measurePanel()
		}
	}
	a.list.MarkNeedsLayout()
	a.list.MarkNeedsPaint()
}

func (a *AutoComplete) buildOptionRow(opt AutoCompleteOption, idx int, th *core.Theme) core.Node {
	var content core.Node
	if opt.LabelNode != nil {
		content = opt.LabelNode
	} else {
		label := opt.DisplayLabel()
		lab := primitive.NewText(label)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = a.Face
		lab.Color = th.Color(core.TokenColorText)
		if opt.Extra != "" {
			extra := primitive.NewText(opt.Extra)
			extra.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
			extra.Face = a.Face
			extra.Color = th.Color(core.TokenColorTextSecondary)
			row := primitive.Row(lab, primitive.Spacer(), extra)
			row.CrossAlign = core.CrossCenter
			row.Gap = 8
			content = row
		} else {
			content = lab
		}
	}
	item := primitive.NewPressable(content)
	item.Padding = primitive.Symmetric(DefaultAutoCompleteItemPadInline, DefaultAutoCompleteItemPadBlock)
	item.ShowFocusRing = false
	if idx == a.activeIndex {
		item.Color = antItemSelectedFill(th)
	}
	item.ColorHovered = antItemHoverFill(th)
	optCopy := opt
	item.Click = func() {
		if a.Disabled || optCopy.Disabled {
			return
		}
		a.applySelect(optCopy)
	}
	item.Base().Role = "option"
	item.Base().Label = opt.DisplayLabel()
	return item
}

func (a *AutoComplete) applySelect(opt AutoCompleteOption) {
	if a == nil || a.Disabled {
		return
	}
	a.selecting = true
	defer func() { a.selecting = false }()

	v := opt.Value
	if !a.Controlled {
		a.Value = v
		if in := a.Input(); in != nil {
			was := in.OnChange
			in.OnChange = nil
			in.SetValue(v)
			in.OnChange = was
		}
	} else if in := a.Input(); in != nil {
		// Controlled: parent writes; still update editor for snappy UX until SetValue.
		was := in.OnChange
		in.OnChange = nil
		in.SetValue(v)
		in.OnChange = was
	}
	if a.OnSelect != nil {
		a.OnSelect(v, opt)
	}
	if a.OnChange != nil {
		a.OnChange(v)
	}
	// Close after select (antd) — always apply; notify parent for controlled open.
	a.applyOpen(false, true)
	a.refreshList(true)
}

func (a *AutoComplete) measurePanel() {
	if a == nil || a.panel == nil {
		return
	}
	w := DefaultAutoCompleteMinWidth
	if a.PopupMatchSelectWidth {
		if a.popup != nil && a.popup.AnchorNode != nil {
			b := a.popup.AnchorNode.Base().Size()
			if b.Width > 0 {
				w = b.Width
			}
		}
		if in := a.Input(); in != nil && in.ChromeNode() != nil {
			if s := in.ChromeNode().Base().Size(); s.Width > w {
				w = s.Width
			}
		}
		if a.FixedWidth > w {
			w = a.FixedWidth
		}
	}
	a.panel.MinWidth = w
	if a.PopupMatchSelectWidth {
		a.panel.Width = w
	}
}

func (a *AutoComplete) applyA11y() {
	if a == nil {
		return
	}
	in := a.Input()
	if in == nil {
		return
	}
	label := a.AriaLabel
	if label == "" {
		label = a.Placeholder
	}
	if in.Editor() != nil {
		b := in.Editor().Base()
		b.Role = "combobox"
		b.Label = label
		if a.Status == InputStatusError {
			b.Live = "assertive"
		} else {
			b.Live = ""
		}
	}
	if a.list != nil {
		a.list.Base().Role = "listbox"
	}
	if in.AriaLabel == "" && a.AriaLabel != "" {
		in.SetAriaLabel(a.AriaLabel)
	}
}

func (a *AutoComplete) paintSpinner(pc *core.PaintContext, size core.Size) {
	th := a.theme()
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
	start := -math.Pi/2 + a.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func stringsToOptions(vals []string) []AutoCompleteOption {
	out := make([]AutoCompleteOption, len(vals))
	for i, v := range vals {
		out[i] = AutoCompleteOption{Value: v, Label: v}
	}
	return out
}

func extractInput(n core.Node) *Input {
	// *Input / *Search etc. are not core.Node; only walk is a no-op.
	// Callers that pass kit types use SetChildren(any) type switch instead.
	_ = n
	return nil
}

func indexOfVisible(vis []AutoCompleteOption, opt AutoCompleteOption) int {
	for i, v := range vis {
		if v.Value == opt.Value && v.DisplayLabel() == opt.DisplayLabel() {
			return i
		}
	}
	return -1
}
