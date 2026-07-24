package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Cascader defaults — docs/antd/cascader.md §6.2 / §6.10
// https://ant.design/components/cascader
const (
	DefaultCascaderMinWidth        = 200.0
	DefaultCascaderWidth           = 240.0
	DefaultCascaderGap             = 4.0
	DefaultCascaderPanelPad        = 4.0
	DefaultCascaderColumnWidth     = 140.0
	DefaultCascaderItemPadInline   = 12.0
	DefaultCascaderItemPadBlock    = 5.0
	DefaultCascaderPanelRadius     = 8.0
	DefaultCascaderFocusRingOutset = 1.5
	DefaultCascaderMaxListHeight   = 256.0
	DefaultCascaderSeparator       = " / "
)

// CascaderOption is one antd Cascader option node.
type CascaderOption struct {
	Value           string
	Label           string
	Disabled        bool
	DisableCheckbox bool
	IsLeaf          bool // true → no expand / no loadData
	Loading         bool // runtime: loadData in flight
	Children        []CascaderOption
}

// DisplayLabel returns Label or falls back to Value.
func (o CascaderOption) DisplayLabel() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

// CascaderExpandTrigger is expandTrigger: click | hover.
type CascaderExpandTrigger int

const (
	CascaderExpandClick CascaderExpandTrigger = iota // default
	CascaderExpandHover
)

// CascaderPlacement is popup placement (antd placement).
type CascaderPlacement int

const (
	CascaderBottomLeft CascaderPlacement = iota // default
	CascaderBottomRight
	CascaderTopLeft
	CascaderTopRight
)

// CascaderShowCheckedStrategy is multi-select display strategy.
type CascaderShowCheckedStrategy int

const (
	// CascaderShowParent (SHOW_PARENT): collapse full-child selections to parent.
	CascaderShowParent CascaderShowCheckedStrategy = iota
	// CascaderShowChild (SHOW_CHILD): show only leaf selections.
	CascaderShowChild
)

// SHOW_* aliases matching antd Cascader.SHOW_PARENT / SHOW_CHILD.
const (
	CascaderSHOW_PARENT = CascaderShowParent
	CascaderSHOW_CHILD  = CascaderShowChild
)

// CascaderDisplayRender customizes the single-select field text.
// labels are option labels along the path; selected are the options themselves.
type CascaderDisplayRender func(labels []string, selected []CascaderOption) string

// CascaderLoadData loads children for a non-leaf path (antd loadData).
// Caller should mutate option.Children / Loading then call Cascader.NotifyOptionsChanged.
type CascaderLoadData func(selectedOptions []CascaderOption)

// Cascader is antd Cascader — field trigger + multi-column cascade popup.
//
//	Column (Wrap)
//	  ├─ Pressable trigger (or TriggerNode)
//	  └─ AnchoredPopup
//	       └─ panel (columns | search hits | loading)
//
// Product contract: docs/antd/cascader.md §6 (P0 DoD).
type Cascader struct {
	Wrap       *primitive.Flex
	Root       *primitive.Pressable // trigger shell; identity stable across rebuilds
	decor      *primitive.Decorated
	display    *primitive.Text
	clearBtn   *primitive.Pressable
	suffix     *primitive.Icon
	popup      *primitive.AnchoredPopup
	panel      *primitive.Decorated
	columns    *primitive.Flex // row of column lists
	searchList *primitive.Flex

	// Product fields (§6.10).
	Options             []CascaderOption
	Value               []string   // single path
	MultiValue          [][]string // multiple paths
	DefaultValue        []string
	DefaultMultiValue   [][]string
	Placeholder         string
	Size                InputSize
	Variant             InputVariant
	Status              InputStatus
	Disabled            bool
	AllowClear          bool
	Open                bool
	ChangeOnSelect      bool
	Multiple            bool
	ShowSearch          bool
	ExpandTrigger       CascaderExpandTrigger
	ShowCheckedStrategy CascaderShowCheckedStrategy
	Placement           CascaderPlacement
	AriaLabel           string
	Face                text.Face
	Theme               *core.Theme
	Viewport            core.Size
	FixedWidth          float64
	SearchValue         string
	DisplayRender       CascaderDisplayRender
	LoadData            CascaderLoadData
	TriggerNode         core.Node // custom trigger (custom-trigger demo)

	// Callbacks.
	OnChange      func(value []string, selected []CascaderOption)
	OnChangeMulti func(values [][]string, selected [][]CascaderOption)
	OnOpenChange  func(open bool)
	OnClear       func()
	OnSearch      func(search string)

	// activePath is the expanded column path (values), not necessarily committed.
	activePath []string
	// activeNodes mirrors activePath options (for loadData / display).
	activeNodes []CascaderOption

	// openControlled: SetOpen used (antd open prop).
	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool

	// Loading spinner (any option.Loading or panel-level).
	spinPhase float64
	spinner   *primitive.Canvas
	boundTree *core.Tree

	// search mode flat paths.
	searchHits []cascaderHit

	// keyboard
	Nav *core.KeyboardNav
}

type cascaderHit struct {
	path    []string
	labels  []string
	options []CascaderOption
}

// NewCascader creates a Cascader with optional root options.
// Defaults (§6.10): middle/outlined, allowClear=true, expandTrigger=click,
// showCheckedStrategy=SHOW_PARENT, placement=bottomLeft, closed, uncontrolled.
func NewCascader(placeholder string, options ...CascaderOption) *Cascader {
	c := &Cascader{
		Placeholder:         placeholder,
		Options:             append([]CascaderOption(nil), options...),
		Size:                InputMiddle,
		Variant:             InputOutlined,
		Status:              InputStatusNone,
		AllowClear:          true, // antd default
		ExpandTrigger:       CascaderExpandClick,
		ShowCheckedStrategy: CascaderShowParent,
		Placement:           CascaderBottomLeft,
	}
	c.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	c.rebuild()
	return c
}

// Node returns the composition root (trigger + popup host).
func (c *Cascader) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.Wrap == nil {
		c.rebuild()
	}
	return c.Wrap
}

// Popup returns the anchored popup (tests / advanced hosts).
func (c *Cascader) Popup() *primitive.AnchoredPopup {
	if c == nil {
		return nil
	}
	return c.popup
}

// Panel returns the dropdown panel chrome (tests).
func (c *Cascader) Panel() *primitive.Decorated {
	if c == nil {
		return nil
	}
	return c.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (c *Cascader) TriggerShell() *primitive.Pressable {
	if c == nil {
		return nil
	}
	return c.Root
}

// IsOpen reports whether the cascade panel is visible.
func (c *Cascader) IsOpen() bool {
	return c != nil && c.Open
}

// ActivePath returns the currently expanded column path (values).
func (c *Cascader) ActivePath() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.activePath...)
}

// GetValue returns the single-select path.
func (c *Cascader) GetValue() []string {
	if c == nil {
		return nil
	}
	return append([]string(nil), c.Value...)
}

// GetMultiValue returns multi-select paths.
func (c *Cascader) GetMultiValue() [][]string {
	if c == nil {
		return nil
	}
	out := make([][]string, len(c.MultiValue))
	for i, p := range c.MultiValue {
		out[i] = append([]string(nil), p...)
	}
	return out
}

// VisibleSearchPaths returns filtered paths when showSearch + query.
func (c *Cascader) VisibleSearchPaths() [][]string {
	if c == nil {
		return nil
	}
	out := make([][]string, len(c.searchHits))
	for i, h := range c.searchHits {
		out[i] = append([]string(nil), h.path...)
	}
	return out
}

// DisplayText returns the current field display string.
func (c *Cascader) DisplayText() string {
	if c == nil {
		return ""
	}
	return c.computeDisplay()
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetOptions replaces the options tree.
func (c *Cascader) SetOptions(opts ...CascaderOption) {
	if c == nil {
		return
	}
	c.Options = append([]CascaderOption(nil), opts...)
	c.rebuildColumns()
	c.refreshDisplay()
}

// NotifyOptionsChanged rebuilds columns after loadData mutates Options.
func (c *Cascader) NotifyOptionsChanged() {
	if c == nil {
		return
	}
	c.rebuildColumns()
	c.syncLoadingTicker()
	c.refreshDisplay()
}

// SetValue sets the single-select path. Does not fire OnChange (API write).
func (c *Cascader) SetValue(path []string) {
	if c == nil {
		return
	}
	c.Value = append([]string(nil), path...)
	if len(path) > 0 {
		c.activePath = append([]string(nil), path...)
		c.syncActiveNodes()
	}
	c.refreshDisplay()
	if c.Open {
		c.rebuildColumns()
	}
}

// SetDefaultValue seeds uncontrolled value when Value is still empty.
func (c *Cascader) SetDefaultValue(path []string) {
	if c == nil {
		return
	}
	c.DefaultValue = append([]string(nil), path...)
	if !c.appliedDefVal && len(c.Value) == 0 && !c.Multiple {
		c.appliedDefVal = true
		c.Value = append([]string(nil), path...)
		if len(path) > 0 {
			c.activePath = append([]string(nil), path...)
			c.syncActiveNodes()
		}
		c.refreshDisplay()
	}
}

// SetMultiValue sets multi-select paths. Does not fire OnChangeMulti.
func (c *Cascader) SetMultiValue(paths [][]string) {
	if c == nil {
		return
	}
	c.MultiValue = clonePaths(paths)
	c.refreshDisplay()
	if c.Open {
		c.rebuildColumns()
	}
}

// SetDefaultMultiValue seeds uncontrolled multi value.
func (c *Cascader) SetDefaultMultiValue(paths [][]string) {
	if c == nil {
		return
	}
	c.DefaultMultiValue = clonePaths(paths)
	if !c.appliedDefVal && len(c.MultiValue) == 0 && c.Multiple {
		c.appliedDefVal = true
		c.MultiValue = clonePaths(paths)
		c.refreshDisplay()
	}
}

// Clear resets the selection (fires OnChange / OnClear).
func (c *Cascader) Clear() {
	if c == nil || c.Disabled {
		return
	}
	if c.Multiple {
		c.MultiValue = nil
		c.fireMultiChange()
	} else {
		c.Value = nil
		c.fireChange(nil, nil)
	}
	c.activePath = nil
	c.activeNodes = nil
	c.SearchValue = ""
	c.refreshDisplay()
	if c.Open {
		c.rebuildColumns()
	}
	if c.OnClear != nil {
		c.OnClear()
	}
}

// SetPlaceholder updates placeholder text.
func (c *Cascader) SetPlaceholder(s string) {
	if c == nil {
		return
	}
	c.Placeholder = s
	c.refreshDisplay()
}

// SetDisabled toggles disabled (no open / no select).
func (c *Cascader) SetDisabled(d bool) {
	if c == nil {
		return
	}
	c.Disabled = d
	if c.Root != nil {
		c.Root.SetDisabled(d)
	}
	if d && c.Open {
		c.applyOpen(false, false)
	}
	c.applyChrome()
	c.applyA11y()
}

// SetSize updates control height via Token.
func (c *Cascader) SetSize(s InputSize) {
	if c == nil {
		return
	}
	c.Size = s
	c.applyChrome()
}

// SetVariant updates visual variant.
func (c *Cascader) SetVariant(v InputVariant) {
	if c == nil {
		return
	}
	c.Variant = v
	c.applyChrome()
}

// SetStatus updates validation chrome.
func (c *Cascader) SetStatus(s InputStatus) {
	if c == nil {
		return
	}
	c.Status = s
	c.applyChrome()
	c.applyA11y()
}

// SetAllowClear toggles clear affordance (antd default true).
func (c *Cascader) SetAllowClear(v bool) {
	if c == nil {
		return
	}
	c.AllowClear = v
	c.rebuild()
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (c *Cascader) SetOpen(open bool) {
	if c == nil {
		return
	}
	c.openControlled = true
	c.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (c *Cascader) SetDefaultOpen(open bool) {
	if c == nil || c.openControlled {
		return
	}
	c.defaultOpen = open
	c.defaultOpenSet = true
	if !c.appliedDefault {
		c.appliedDefault = true
		c.applyOpen(open, false)
	}
}

// SetChangeOnSelect toggles per-level commit (single mode).
func (c *Cascader) SetChangeOnSelect(v bool) {
	if c == nil {
		return
	}
	c.ChangeOnSelect = v
}

// SetExpandTrigger sets click or hover expand.
func (c *Cascader) SetExpandTrigger(t CascaderExpandTrigger) {
	if c == nil {
		return
	}
	c.ExpandTrigger = t
	if c.Open {
		c.rebuildColumns()
	}
}

// SetMultiple enables multi-path selection.
func (c *Cascader) SetMultiple(v bool) {
	if c == nil {
		return
	}
	c.Multiple = v
	c.refreshDisplay()
	if c.Open {
		c.rebuildColumns()
	}
}

// SetShowCheckedStrategy sets multi display strategy.
func (c *Cascader) SetShowCheckedStrategy(s CascaderShowCheckedStrategy) {
	if c == nil {
		return
	}
	c.ShowCheckedStrategy = s
	c.refreshDisplay()
}

// SetShowSearch toggles path search filter.
func (c *Cascader) SetShowSearch(v bool) {
	if c == nil {
		return
	}
	c.ShowSearch = v
	if !v {
		c.SearchValue = ""
		c.searchHits = nil
	}
	if c.Open {
		c.rebuildColumns()
	}
}

// SetSearchValue sets the search query (showSearch). Fires OnSearch.
func (c *Cascader) SetSearchValue(q string) {
	if c == nil {
		return
	}
	c.SearchValue = q
	if c.OnSearch != nil {
		c.OnSearch(q)
	}
	c.rebuildSearchHits()
	if c.Open {
		c.rebuildColumns()
	}
	// Typing search opens the panel (uncontrolled).
	if c.ShowSearch && q != "" && !c.Open && !c.Disabled {
		c.applyOpen(true, false)
	}
}

// SetDisplayRender sets custom field text renderer.
func (c *Cascader) SetDisplayRender(fn CascaderDisplayRender) {
	if c == nil {
		return
	}
	c.DisplayRender = fn
	c.refreshDisplay()
}

// SetLoadData sets async children loader.
func (c *Cascader) SetLoadData(fn CascaderLoadData) {
	if c == nil {
		return
	}
	c.LoadData = fn
}

// SetTriggerNode sets a custom trigger node (nil restores default field).
func (c *Cascader) SetTriggerNode(n core.Node) {
	if c == nil {
		return
	}
	c.TriggerNode = n
	c.rebuild()
}

// SetPlacement sets popup placement.
func (c *Cascader) SetPlacement(p CascaderPlacement) {
	if c == nil {
		return
	}
	c.Placement = p
	if c.popup != nil {
		c.popup.Placement = mapCascaderPlacement(p)
		if c.Open {
			c.syncPopupGeometry()
		}
	}
}

// SetTheme sets an explicit theme override.
func (c *Cascader) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetFace sets the font face.
func (c *Cascader) SetFace(face text.Face) {
	if c == nil {
		return
	}
	c.Face = face
	c.rebuild()
}

// SetAriaLabel sets the accessible name.
func (c *Cascader) SetAriaLabel(s string) {
	if c == nil {
		return
	}
	c.AriaLabel = s
	c.applyA11y()
}

// SetOnChange sets single-select callback.
func (c *Cascader) SetOnChange(fn func(value []string, selected []CascaderOption)) {
	if c == nil {
		return
	}
	c.OnChange = fn
}

// SetOnChangeMulti sets multi-select callback.
func (c *Cascader) SetOnChangeMulti(fn func(values [][]string, selected [][]CascaderOption)) {
	if c == nil {
		return
	}
	c.OnChangeMulti = fn
}

// SetOnOpenChange sets open-change callback.
func (c *Cascader) SetOnOpenChange(fn func(open bool)) {
	if c == nil {
		return
	}
	c.OnOpenChange = fn
}

// SetOnClear sets clear callback.
func (c *Cascader) SetOnClear(fn func()) {
	if c == nil {
		return
	}
	c.OnClear = fn
}

// SetOnSearch sets search callback.
func (c *Cascader) SetOnSearch(fn func(search string)) {
	if c == nil {
		return
	}
	c.OnSearch = fn
}

// SelectPath simulates choosing each level until path (for tests / host).
// Respects changeOnSelect, leaf commit, disabled, multiple.
func (c *Cascader) SelectPath(path []string) {
	if c == nil || c.Disabled || len(path) == 0 {
		return
	}
	if !c.Open {
		c.applyOpen(true, false)
	}
	var walked []string
	var nodes []CascaderOption
	opts := c.Options
	for i, v := range path {
		idx := findOptionIndex(opts, v)
		if idx < 0 {
			return
		}
		opt := opts[idx]
		if opt.Disabled {
			return
		}
		walked = append(walked, opt.Value)
		nodes = append(nodes, opt)
		c.activePath = append([]string(nil), walked...)
		c.activeNodes = append([]CascaderOption(nil), nodes...)
		leaf := c.isLeaf(opt)
		isLast := i == len(path)-1

		if c.Multiple {
			if isLast || leaf {
				c.toggleMultiPath(walked, nodes)
			}
			if !leaf {
				c.maybeLoadData(nodes)
				opts = c.childrenOf(walked)
			}
			c.rebuildColumns()
			if leaf {
				return
			}
			continue
		}

		// single
		if leaf {
			c.commitSingle(walked, nodes, true)
			c.rebuildColumns()
			return
		}
		if c.ChangeOnSelect {
			c.commitSingle(walked, nodes, false)
		}
		c.maybeLoadData(nodes)
		opts = c.childrenOf(walked)
		c.rebuildColumns()
	}
}

// ExpandPath expands columns to path without committing selection.
func (c *Cascader) ExpandPath(path []string) {
	if c == nil || c.Disabled {
		return
	}
	if !c.Open {
		c.applyOpen(true, false)
	}
	c.activePath = append([]string(nil), path...)
	c.syncActiveNodes()
	// load along path if needed
	var walked []CascaderOption
	opts := c.Options
	for _, v := range path {
		idx := findOptionIndex(opts, v)
		if idx < 0 {
			break
		}
		opt := opts[idx]
		walked = append(walked, opt)
		if !c.isLeaf(opt) && len(opt.Children) == 0 && c.LoadData != nil {
			c.maybeLoadData(walked)
		}
		opts = opt.Children
	}
	c.activeNodes = append([]CascaderOption(nil), walked...)
	c.rebuildColumns()
}

// HoverExpand expands option at column level by index (expandTrigger=hover tests).
func (c *Cascader) HoverExpand(level, index int) {
	if c == nil || c.Disabled {
		return
	}
	if !c.Open {
		c.applyOpen(true, false)
	}
	opts := c.columnOptions(level)
	if index < 0 || index >= len(opts) {
		return
	}
	opt := opts[index]
	if opt.Disabled {
		return
	}
	// truncate active path to level, append
	if level <= 0 {
		c.activePath = []string{opt.Value}
		c.activeNodes = []CascaderOption{opt}
	} else {
		if len(c.activePath) > level {
			c.activePath = c.activePath[:level]
			c.activeNodes = c.activeNodes[:level]
		}
		for len(c.activePath) < level {
			// incomplete path — abort
			return
		}
		c.activePath = append(c.activePath[:level], opt.Value)
		if len(c.activeNodes) > level {
			c.activeNodes = c.activeNodes[:level]
		}
		c.activeNodes = append(c.activeNodes, opt)
	}
	if !c.isLeaf(opt) {
		c.maybeLoadData(c.activeNodes)
	}
	c.rebuildColumns()
}

// AttachTicker registers loading spinner animation for loadData.
func (c *Cascader) AttachTicker(t *core.Tree) {
	if c == nil || t == nil {
		return
	}
	c.boundTree = t
	t.BindTicker(c, c.anyLoading())
}

// Tick advances the loading spinner. Implements core.Ticker when loading.
func (c *Cascader) Tick(dt float64) bool {
	if c == nil || !c.anyLoading() {
		return false
	}
	c.spinPhase += dt * 1.4
	if c.spinPhase > 1 {
		c.spinPhase -= 1
	}
	if c.spinner != nil {
		c.spinner.MarkNeedsPaint()
	} else if c.panel != nil {
		c.panel.MarkNeedsPaint()
	}
	return c.anyLoading()
}

// HandleKey processes open / escape / enter when focused.
func (c *Cascader) HandleKey(ev *core.KeyEvent) {
	if c == nil || c.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !c.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" || ev.Key == "Down" {
			c.applyOpen(true, false)
			ev.Handled = true
		}
		return
	}
	if ev.Key == "Escape" || ev.Key == "Esc" {
		c.applyOpen(false, false)
		ev.Handled = true
		return
	}
	// Search typing: host may call SetSearchValue; arrows navigate search hits.
	if c.ShowSearch && c.SearchValue != "" {
		if c.Nav.HandleKey(ev.Key) {
			ev.Handled = true
			return
		}
		if (ev.Key == "Enter" || ev.Key == " ") && c.Nav.Index >= 0 && c.Nav.Index < len(c.searchHits) {
			h := c.searchHits[c.Nav.Index]
			c.commitSearchHit(h)
			ev.Handled = true
		}
	}
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (c *Cascader) theme() *core.Theme {
	var n core.Node
	if c.Wrap != nil {
		n = c.Wrap
	}
	return themeOf(c.Theme, n)
}

func (c *Cascader) controlHeight() float64 {
	th := c.theme()
	switch c.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (c *Cascader) rebuild() {
	if c == nil {
		return
	}
	// apply default value once
	if !c.appliedDefVal {
		if c.Multiple && len(c.MultiValue) == 0 && len(c.DefaultMultiValue) > 0 {
			c.MultiValue = clonePaths(c.DefaultMultiValue)
			c.appliedDefVal = true
		} else if !c.Multiple && len(c.Value) == 0 && len(c.DefaultValue) > 0 {
			c.Value = append([]string(nil), c.DefaultValue...)
			c.activePath = append([]string(nil), c.DefaultValue...)
			c.syncActiveNodes()
			c.appliedDefVal = true
		}
	}

	th := c.theme()
	h := c.controlHeight()
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	fontSz := th.SizeOr(core.TokenFontSize, 14)
	if c.Size == InputSmall {
		fontSz = th.SizeOr(core.TokenFontSizeSM, 12)
	} else if c.Size == InputLarge {
		fontSz = th.SizeOr(core.TokenFontSizeLG, 16)
	}

	// display text
	c.display = primitive.NewText(c.computeDisplay())
	c.display.FontSize = fontSz
	c.display.Face = c.Face
	c.display.Color = c.displayColor(th)

	// clear button
	var clearNode core.Node
	if c.AllowClear && c.hasValue() && !c.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		c.clearBtn = primitive.NewPressable(x)
		c.clearBtn.Focusable = false
		c.clearBtn.ShowFocusRing = false
		c.clearBtn.Base().Role = "button"
		c.clearBtn.Base().Label = "clear"
		c.clearBtn.Click = func() {
			c.Clear()
		}
		clearNode = c.clearBtn
	} else {
		c.clearBtn = nil
	}

	// suffix chevron
	c.suffix = primitive.NewIcon("chevron-down")
	c.suffix.Size = 12
	sec := th.Color(core.TokenColorTextSecondary)
	if sec.A < 0.3 {
		sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
	}
	c.suffix.Color = sec

	rowKids := []core.Node{c.display, primitive.Spacer()}
	if clearNode != nil {
		rowKids = append(rowKids, clearNode)
	}
	rowKids = append(rowKids, c.suffix)
	row := primitive.Row(rowKids...)
	row.CrossAlign = core.CrossCenter
	row.Gap = 8

	c.decor = primitive.NewDecorated(row)
	c.decor.Padding = primitive.Symmetric(padH, 0)
	c.decor.Radius = radius
	c.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	c.decor.MinHeight = h
	c.decor.Height = h
	w := c.FixedWidth
	if w <= 0 {
		w = DefaultCascaderWidth
	}
	c.decor.MinWidth = DefaultCascaderMinWidth
	c.decor.Width = w
	c.decor.SetCenterContent(true)
	c.decor.StretchChild = true
	c.applyChrome()

	// columns host
	c.columns = primitive.Row()
	c.columns.Gap = 0
	c.columns.CrossAlign = core.CrossStart
	c.searchList = primitive.Column()
	c.searchList.Gap = 2
	c.searchList.CrossAlign = core.CrossStart

	panelBody := primitive.Column(c.columns)
	panelBody.CrossAlign = core.CrossStart

	c.panel = primitive.NewDecorated(panelBody)
	c.panel.Padding = primitive.All(DefaultCascaderPanelPad)
	c.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultCascaderPanelRadius)
	c.panel.Background = th.Color(core.TokenColorBgContainer)
	c.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	c.panel.BorderColor = th.Color(core.TokenColorBorder)
	c.panel.MinWidth = DefaultCascaderColumnWidth
	c.panel.Base().Role = "listbox"

	if c.popup == nil {
		c.popup = primitive.NewAnchoredPopup(c.panel)
	} else {
		c.popup.Content = c.panel
	}
	c.popup.Placement = mapCascaderPlacement(c.Placement)
	c.popup.Gap = DefaultCascaderGap
	c.popup.DismissOnOutside = true
	c.popup.OnDismiss = func() {
		c.Open = false
		if c.OnOpenChange != nil {
			c.OnOpenChange(false)
		}
		c.applyChrome()
	}

	// trigger shell
	var triggerChild core.Node = c.decor
	if c.TriggerNode != nil {
		triggerChild = c.TriggerNode
	}
	if c.Root == nil {
		c.Root = primitive.NewPressable(triggerChild)
	} else {
		c.Root.ClearChildren()
		c.Root.AddChild(triggerChild)
	}
	c.Root.Focusable = true
	c.Root.ShowFocusRing = true
	c.Root.FocusRingRadius = radius
	c.Root.FocusRingOutset = DefaultCascaderFocusRingOutset
	c.Root.SetDisabled(c.Disabled)
	c.Root.OnStateChange = func() {
		c.applyChrome()
	}
	c.Root.Click = func() {
		if c.Disabled {
			return
		}
		c.applyOpen(!c.Open, false)
	}
	c.applyA11y()

	if c.Wrap == nil {
		c.Wrap = primitive.Column(c.Root, c.popup)
	} else {
		c.Wrap.ClearChildren()
		c.Wrap.AddChild(c.Root)
		c.Wrap.AddChild(c.popup)
	}
	c.Wrap.CrossAlign = core.CrossStart
	c.Wrap.SetThemeHook(func(*core.Theme) { c.rebuild() })

	// defaultOpen once
	if c.defaultOpenSet && !c.appliedDefault && !c.openControlled {
		c.appliedDefault = true
		if c.defaultOpen {
			c.Open = true
		}
	}

	c.rebuildColumns()
	if c.Open {
		c.syncPopupGeometry()
		if c.popup != nil {
			c.popup.SetOpen(true)
		}
	}
	c.Wrap.MarkNeedsLayout()
	c.Wrap.MarkNeedsPaint()
}

func (c *Cascader) applyChrome() {
	if c == nil || c.decor == nil {
		return
	}
	th := c.theme()
	h := c.controlHeight()
	c.decor.MinHeight = h
	c.decor.Height = h
	c.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	c.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	switch c.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		c.decor.BorderWidth = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		c.decor.BorderWidth = 0
	case InputUnderlined:
		bg = render.RGBA{}
		c.decor.Radius = 0
	default:
		// outlined
	}

	if c.Disabled {
		dbg := th.Color(core.TokenColorDisabledBg)
		if dbg.A < 0.05 {
			dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bg = dbg
		bd = th.Color(core.TokenColorBorder)
		c.decor.Background = bg
		c.decor.BorderColor = bd
		if c.display != nil {
			c.display.Color = th.Color(core.TokenColorDisabledText)
			if c.display.Color.A < 0.2 {
				c.display.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
		}
		c.decor.MarkNeedsPaint()
		return
	}

	// status border
	switch c.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
	default:
		if c.Root != nil && (c.Root.State.Focused || c.Open) {
			bd = th.Color(core.TokenColorPrimary)
		} else if c.Root != nil && c.Root.State.Hovered {
			hb := th.Color(core.TokenColorBorderHover)
			if hb.A < 0.5 {
				hb = th.Color(core.TokenColorPrimaryHover)
			}
			bd = hb
		}
	}
	if c.Variant == InputUnderlined {
		// only bottom emphasis — approximate with border color
		c.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	}
	c.decor.Background = bg
	c.decor.BorderColor = bd
	if c.display != nil {
		c.display.Color = c.displayColor(th)
	}
	c.decor.MarkNeedsPaint()
}

func (c *Cascader) displayColor(th *core.Theme) render.RGBA {
	if c.hasValue() {
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

func (c *Cascader) applyA11y() {
	if c == nil || c.Root == nil {
		return
	}
	c.Root.Base().Role = "combobox"
	label := c.AriaLabel
	if label == "" {
		label = c.Placeholder
	}
	if label == "" {
		label = "Cascader"
	}
	c.Root.Base().Label = label
	// invalid when status=error
	if c.Status == InputStatusError {
		c.Root.Base().Label = label + " invalid"
	}
}

func (c *Cascader) applyOpen(open bool, skipCallback bool) {
	if c == nil {
		return
	}
	if c.Disabled && open {
		return
	}
	prev := c.Open
	c.Open = open
	if c.popup != nil {
		if open {
			// seed active path from value when opening
			if len(c.activePath) == 0 {
				if c.Multiple && len(c.MultiValue) > 0 {
					c.activePath = append([]string(nil), c.MultiValue[0]...)
				} else if len(c.Value) > 0 {
					c.activePath = append([]string(nil), c.Value...)
				}
				c.syncActiveNodes()
			}
			c.rebuildColumns()
			c.syncPopupGeometry()
			c.popup.SetOpen(true)
		} else {
			c.popup.SetOpen(false)
			// clear search on close (antd-ish)
			if c.SearchValue != "" {
				c.SearchValue = ""
				c.searchHits = nil
			}
		}
	}
	c.applyChrome()
	if !skipCallback && prev != open && c.OnOpenChange != nil {
		c.OnOpenChange(open)
	}
}

func (c *Cascader) syncPopupGeometry() {
	if c == nil || c.popup == nil || c.Root == nil {
		return
	}
	c.popup.UpdateAnchorFromNode(c.Root)
	if c.Viewport.Width > 0 {
		c.popup.Viewport = c.Viewport
	}
}

func (c *Cascader) rebuildColumns() {
	if c == nil || c.columns == nil || c.panel == nil {
		return
	}
	// search mode
	if c.ShowSearch && strings.TrimSpace(c.SearchValue) != "" {
		c.rebuildSearchHits()
		c.renderSearchPanel()
		return
	}

	th := c.theme()
	c.columns.ClearChildren()
	// ensure panel body shows columns
	if c.panel != nil {
		c.panel.ClearChildren()
		body := primitive.Column(c.columns)
		body.CrossAlign = core.CrossStart
		c.panel.AddChild(body)
	}

	// column 0 = roots
	c.addColumn(0, c.Options, th)
	// subsequent columns from activePath
	opts := c.Options
	for level, v := range c.activePath {
		idx := findOptionIndex(opts, v)
		if idx < 0 {
			break
		}
		opt := opts[idx]
		if c.isLeaf(opt) {
			break
		}
		// loading placeholder column
		if opt.Loading && len(opt.Children) == 0 {
			c.addLoadingColumn(th)
			break
		}
		children := opt.Children
		// re-resolve from live tree (loadData may have filled)
		if live := c.liveOption(c.activePath[:level+1]); live != nil {
			children = live.Children
		}
		if len(children) == 0 {
			break
		}
		c.addColumn(level+1, children, th)
		opts = children
	}
	c.columns.MarkNeedsLayout()
	c.columns.MarkNeedsPaint()
	if c.panel != nil {
		c.panel.MarkNeedsLayout()
		c.panel.MarkNeedsPaint()
	}
}

func (c *Cascader) addColumn(level int, opts []CascaderOption, th *core.Theme) {
	col := primitive.Column()
	col.Gap = 2
	col.CrossAlign = core.CrossStart
	host := primitive.NewDecorated(col)
	host.MinWidth = DefaultCascaderColumnWidth
	host.Width = DefaultCascaderColumnWidth
	host.BorderWidth = 0

	// divider between columns
	if level > 0 {
		line := primitive.NewBox()
		line.Width = 1
		line.Height = 32
		line.Color = th.Color(core.TokenColorSplit)
		if line.Color.A < 0.05 {
			line.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
		}
		row := primitive.Row(line, host)
		row.CrossAlign = core.CrossStretch
		c.columns.AddChild(row)
	} else {
		c.columns.AddChild(host)
	}

	activeVal := ""
	if level < len(c.activePath) {
		activeVal = c.activePath[level]
	}
	selectedLeaf := ""
	if !c.Multiple && level < len(c.Value) {
		selectedLeaf = c.Value[level]
	}

	for i := range opts {
		opt := opts[i]
		i, opt := i, opt
		lab := primitive.NewText(opt.DisplayLabel())
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = c.Face
		lab.Color = th.Color(core.TokenColorText)
		if opt.Disabled {
			lab.Color = th.Color(core.TokenColorDisabledText)
			if lab.Color.A < 0.2 {
				lab.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
		}

		// trailing: loading | expand chevron | check
		var trail core.Node
		if opt.Loading {
			sp := c.makeSpinner(th, 12)
			trail = sp
		} else if !c.isLeaf(opt) {
			ch := primitive.NewIcon("chevron-right")
			ch.Size = 10
			ch.Color = th.Color(core.TokenColorTextSecondary)
			trail = ch
		} else if c.Multiple {
			// checkbox indicator
			mark := "☐"
			if c.pathSelected(append(c.pathPrefix(level), opt.Value)) {
				mark = "☑"
			}
			cb := primitive.NewText(mark)
			cb.FontSize = 12
			cb.Face = c.Face
			cb.Color = th.Color(core.TokenColorPrimary)
			trail = cb
		}

		var inner core.Node
		if trail != nil {
			r := primitive.Row(lab, primitive.Spacer(), trail)
			r.CrossAlign = core.CrossCenter
			r.Gap = 4
			inner = r
		} else {
			inner = lab
		}

		item := primitive.NewPressable(inner)
		item.Padding = primitive.Symmetric(DefaultCascaderItemPadInline, DefaultCascaderItemPadBlock)
		item.Base().Role = "option"
		item.Base().Label = opt.DisplayLabel()
		item.SetDisabled(opt.Disabled)
		item.ColorHovered = antItemHoverFill(th)

		// selected / active highlight
		if opt.Value == activeVal || (!c.Multiple && opt.Value == selectedLeaf) ||
			(c.Multiple && c.pathSelected(append(c.pathPrefix(level), opt.Value))) {
			item.Color = antItemSelectedFill(th)
			lab.Color = antItemSelectedText(th)
		}

		level, opt, i := level, opt, i
		item.Click = func() {
			c.onOptionActivate(level, i, opt, false)
		}
		if c.ExpandTrigger == CascaderExpandHover && !c.isLeaf(opt) && !opt.Disabled {
			item.OnStateChange = func() {
				if item.State.Hovered {
					c.onOptionActivate(level, i, opt, true)
				}
			}
		}
		col.AddChild(item)
	}
}

func (c *Cascader) addLoadingColumn(th *core.Theme) {
	col := primitive.Column()
	col.Gap = 8
	col.CrossAlign = core.CrossCenter
	col.Padding = primitive.All(16)
	sp := c.makeSpinner(th, 16)
	lab := primitive.NewText("Loading")
	lab.FontSize = 12
	lab.Face = c.Face
	lab.Color = th.Color(core.TokenColorTextSecondary)
	col.AddChild(sp)
	col.AddChild(lab)
	host := primitive.NewDecorated(col)
	host.MinWidth = DefaultCascaderColumnWidth
	host.Width = DefaultCascaderColumnWidth
	host.BorderWidth = 0

	line := primitive.NewBox()
	line.Width = 1
	line.Height = 32
	line.Color = th.Color(core.TokenColorSplit)
	row := primitive.Row(line, host)
	row.CrossAlign = core.CrossStretch
	c.columns.AddChild(row)
}

func (c *Cascader) makeSpinner(th *core.Theme, size float64) *primitive.Canvas {
	sp := primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		col := th.Color(core.TokenColorPrimary)
		if col.A < 0.1 {
			col = render.Hex("#1677FF")
		}
		cx, cy := sz.Width/2, sz.Height/2
		r := math.Min(sz.Width, sz.Height)/2 - 1
		if r < 2 {
			r = 2
		}
		stroke := 1.5
		steps := 12
		start := -math.Pi/2 + c.spinPhase*2*math.Pi
		end := start + math.Pi*1.4
		pts := make([]float64, 0, (steps+1)*2)
		for i := 0; i <= steps; i++ {
			ang := start + (end-start)*float64(i)/float64(steps)
			pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
		}
		pc.StrokeLocalPolyline(pts, stroke, col)
	})
	c.spinner = sp
	return sp
}

func (c *Cascader) onOptionActivate(level int, index int, opt CascaderOption, hoverOnly bool) {
	if c == nil || c.Disabled || opt.Disabled {
		return
	}
	// build path
	path := c.pathPrefix(level)
	path = append(path, opt.Value)
	nodes := c.nodesPrefix(level)
	nodes = append(nodes, opt)

	c.activePath = append([]string(nil), path...)
	c.activeNodes = append([]CascaderOption(nil), nodes...)

	leaf := c.isLeaf(opt)

	if c.Multiple {
		if !hoverOnly {
			// toggle at this node (antd multiple: select any level)
			if !opt.DisableCheckbox {
				c.toggleMultiPath(path, nodes)
			}
		}
		if !leaf {
			c.maybeLoadData(nodes)
		}
		c.rebuildColumns()
		return
	}

	// single
	if hoverOnly {
		// only expand
		if !leaf {
			c.maybeLoadData(nodes)
		}
		c.rebuildColumns()
		return
	}

	if leaf {
		c.commitSingle(path, nodes, true)
		c.rebuildColumns()
		return
	}

	// non-leaf click
	if c.ChangeOnSelect {
		c.commitSingle(path, nodes, false)
	}
	c.maybeLoadData(nodes)
	c.rebuildColumns()
}

func (c *Cascader) commitSingle(path []string, nodes []CascaderOption, close bool) {
	c.Value = append([]string(nil), path...)
	c.refreshDisplay()
	c.fireChange(path, nodes)
	if close && !c.openControlled {
		c.applyOpen(false, false)
	} else if close && c.openControlled {
		// still notify intent; parent controls open
		if c.OnOpenChange != nil {
			c.OnOpenChange(false)
		}
	}
}

func (c *Cascader) toggleMultiPath(path []string, nodes []CascaderOption) {
	// If path already selected → remove it and descendants; else add.
	if idx := findPathIndex(c.MultiValue, path); idx >= 0 {
		c.MultiValue = append(c.MultiValue[:idx], c.MultiValue[idx+1:]...)
	} else {
		// remove any descendant paths of path, and ancestors
		filtered := c.MultiValue[:0]
		for _, p := range c.MultiValue {
			if pathPrefixOf(path, p) || pathPrefixOf(p, path) {
				continue
			}
			filtered = append(filtered, p)
		}
		c.MultiValue = append(filtered, append([]string(nil), path...))
	}
	c.refreshDisplay()
	c.fireMultiChange()
}

func (c *Cascader) fireChange(path []string, nodes []CascaderOption) {
	if c.OnChange != nil {
		sel := append([]CascaderOption(nil), nodes...)
		c.OnChange(append([]string(nil), path...), sel)
	}
}

func (c *Cascader) fireMultiChange() {
	if c.OnChangeMulti != nil {
		vals := clonePaths(c.MultiValue)
		sels := make([][]CascaderOption, len(vals))
		for i, p := range vals {
			sels[i] = c.resolvePath(p)
		}
		c.OnChangeMulti(vals, sels)
	}
	// also emit OnChange with first path for simple listeners
	if c.OnChange != nil {
		if len(c.MultiValue) > 0 {
			p := c.MultiValue[0]
			c.OnChange(append([]string(nil), p...), c.resolvePath(p))
		} else {
			c.OnChange(nil, nil)
		}
	}
}

func (c *Cascader) maybeLoadData(nodes []CascaderOption) {
	if c == nil || c.LoadData == nil || len(nodes) == 0 {
		return
	}
	last := nodes[len(nodes)-1]
	if last.IsLeaf || len(last.Children) > 0 || last.Loading {
		// re-check live
		if live := c.liveOption(pathValues(nodes)); live != nil {
			if live.IsLeaf || len(live.Children) > 0 || live.Loading {
				return
			}
		} else {
			return
		}
	}
	// mark loading on live option
	if live := c.liveOption(pathValues(nodes)); live != nil {
		live.Loading = true
		c.syncLoadingTicker()
		c.rebuildColumns()
		// call loadData with snapshot
		snap := append([]CascaderOption(nil), nodes...)
		c.LoadData(snap)
	}
}

func (c *Cascader) syncLoadingTicker() {
	if c.boundTree == nil {
		return
	}
	if c.anyLoading() {
		c.boundTree.AddTicker(c)
	} else {
		c.boundTree.RemoveTicker(c)
	}
}

func (c *Cascader) anyLoading() bool {
	var walk func([]CascaderOption) bool
	walk = func(opts []CascaderOption) bool {
		for i := range opts {
			if opts[i].Loading {
				return true
			}
			if walk(opts[i].Children) {
				return true
			}
		}
		return false
	}
	return walk(c.Options)
}

func (c *Cascader) rebuildSearchHits() {
	c.searchHits = nil
	q := strings.TrimSpace(c.SearchValue)
	if q == "" || !c.ShowSearch {
		return
	}
	var walk func(opts []CascaderOption, path []string, labels []string, nodes []CascaderOption)
	walk = func(opts []CascaderOption, path []string, labels []string, nodes []CascaderOption) {
		for _, o := range opts {
			if o.Disabled {
				continue
			}
			np := append(append([]string(nil), path...), o.Value)
			nl := append(append([]string(nil), labels...), o.DisplayLabel())
			nn := append(append([]CascaderOption(nil), nodes...), o)
			// match any label in path
			matched := false
			for _, lb := range nl {
				if containsFold(lb, q) || containsFold(o.Value, q) {
					matched = true
					break
				}
			}
			if c.isLeaf(o) || len(o.Children) == 0 {
				if matched {
					c.searchHits = append(c.searchHits, cascaderHit{path: np, labels: nl, options: nn})
				}
			} else {
				walk(o.Children, np, nl, nn)
			}
		}
	}
	walk(c.Options, nil, nil, nil)
	if c.Nav != nil {
		c.Nav.SetCount(len(c.searchHits))
		if len(c.searchHits) > 0 {
			c.Nav.Index = 0
		} else {
			c.Nav.Index = -1
		}
	}
}

func (c *Cascader) renderSearchPanel() {
	if c.panel == nil || c.searchList == nil {
		return
	}
	th := c.theme()
	c.searchList.ClearChildren()
	c.panel.ClearChildren()
	c.panel.AddChild(c.searchList)

	if len(c.searchHits) == 0 {
		empty := primitive.NewText("Not Found")
		empty.FontSize = 14
		empty.Face = c.Face
		empty.Color = th.Color(core.TokenColorTextSecondary)
		pad := primitive.NewDecorated(empty)
		pad.Padding = primitive.Symmetric(12, 8)
		c.searchList.AddChild(pad)
		return
	}
	for i, h := range c.searchHits {
		i, h := i, h
		text := strings.Join(h.labels, DefaultCascaderSeparator)
		lab := primitive.NewText(text)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = c.Face
		lab.Color = th.Color(core.TokenColorText)
		item := primitive.NewPressable(lab)
		item.Padding = primitive.Symmetric(DefaultCascaderItemPadInline, DefaultCascaderItemPadBlock)
		item.Base().Role = "option"
		item.Base().Label = text
		item.ColorHovered = antItemHoverFill(th)
		if c.Nav != nil && c.Nav.Index == i {
			item.Color = antItemSelectedFill(th)
			lab.Color = antItemSelectedText(th)
		}
		item.Click = func() {
			c.commitSearchHit(h)
		}
		c.searchList.AddChild(item)
	}
	c.searchList.MarkNeedsLayout()
	c.searchList.MarkNeedsPaint()
}

func (c *Cascader) commitSearchHit(h cascaderHit) {
	if c.Multiple {
		c.toggleMultiPath(h.path, h.options)
		c.SearchValue = ""
		c.searchHits = nil
		c.rebuildColumns()
		return
	}
	c.activePath = append([]string(nil), h.path...)
	c.activeNodes = append([]CascaderOption(nil), h.options...)
	c.commitSingle(h.path, h.options, true)
	c.SearchValue = ""
	c.searchHits = nil
}

func (c *Cascader) hasValue() bool {
	if c.Multiple {
		return len(c.MultiValue) > 0
	}
	return len(c.Value) > 0
}

func (c *Cascader) computeDisplay() string {
	if c.Multiple {
		return c.computeMultiDisplay()
	}
	if len(c.Value) == 0 {
		if c.Placeholder != "" {
			return c.Placeholder
		}
		return "Please select"
	}
	nodes := c.resolvePath(c.Value)
	labels := make([]string, 0, len(nodes))
	for _, n := range nodes {
		labels = append(labels, n.DisplayLabel())
	}
	// if resolve incomplete, use raw values
	if len(labels) == 0 {
		labels = append([]string(nil), c.Value...)
	}
	if c.DisplayRender != nil {
		return c.DisplayRender(labels, nodes)
	}
	return strings.Join(labels, DefaultCascaderSeparator)
}

func (c *Cascader) computeMultiDisplay() string {
	if len(c.MultiValue) == 0 {
		if c.Placeholder != "" {
			return c.Placeholder
		}
		return "Please select"
	}
	paths := c.displayMultiPaths()
	parts := make([]string, 0, len(paths))
	for _, p := range paths {
		nodes := c.resolvePath(p)
		labels := make([]string, 0, len(nodes))
		for _, n := range nodes {
			labels = append(labels, n.DisplayLabel())
		}
		if len(labels) == 0 {
			labels = p
		}
		parts = append(parts, strings.Join(labels, DefaultCascaderSeparator))
	}
	return strings.Join(parts, ", ")
}

// displayMultiPaths applies showCheckedStrategy.
func (c *Cascader) displayMultiPaths() [][]string {
	if c.ShowCheckedStrategy == CascaderShowChild {
		// only leaves
		var out [][]string
		for _, p := range c.MultiValue {
			nodes := c.resolvePath(p)
			if len(nodes) == 0 {
				out = append(out, p)
				continue
			}
			last := nodes[len(nodes)-1]
			if c.isLeaf(last) {
				out = append(out, p)
			} else {
				// expand to leaves under p
				out = append(out, c.collectLeaves(p, last.Children)...)
			}
		}
		return out
	}
	// SHOW_PARENT: if all children of a node are selected, show parent only.
	// Simplified: return MultiValue as stored (toggle already collapses).
	return clonePaths(c.MultiValue)
}

func (c *Cascader) collectLeaves(prefix []string, children []CascaderOption) [][]string {
	var out [][]string
	for _, ch := range children {
		p := append(append([]string(nil), prefix...), ch.Value)
		if c.isLeaf(ch) {
			out = append(out, p)
		} else {
			out = append(out, c.collectLeaves(p, ch.Children)...)
		}
	}
	return out
}

func (c *Cascader) refreshDisplay() {
	if c == nil || c.display == nil {
		return
	}
	c.display.SetValue(c.computeDisplay())
	c.display.Color = c.displayColor(c.theme())
	// clear button visibility may change
	if c.AllowClear {
		// cheapest: rebuild trigger content when clear presence flips
		wantClear := c.hasValue() && !c.Disabled
		haveClear := c.clearBtn != nil
		if wantClear != haveClear && c.TriggerNode == nil {
			c.rebuild()
			return
		}
	}
	if c.display != nil {
		c.display.MarkNeedsPaint()
	}
}

func (c *Cascader) isLeaf(opt CascaderOption) bool {
	if opt.IsLeaf {
		return true
	}
	if len(opt.Children) > 0 {
		return false
	}
	// no children: leaf unless loadData can fill (IsLeaf=false and LoadData set)
	if c.LoadData != nil && !opt.IsLeaf {
		return false
	}
	return true
}

func (c *Cascader) pathPrefix(level int) []string {
	if level <= 0 || len(c.activePath) == 0 {
		return nil
	}
	if level > len(c.activePath) {
		level = len(c.activePath)
	}
	return append([]string(nil), c.activePath[:level]...)
}

func (c *Cascader) nodesPrefix(level int) []CascaderOption {
	if level <= 0 || len(c.activeNodes) == 0 {
		return nil
	}
	if level > len(c.activeNodes) {
		level = len(c.activeNodes)
	}
	return append([]CascaderOption(nil), c.activeNodes[:level]...)
}

func (c *Cascader) columnOptions(level int) []CascaderOption {
	if level <= 0 {
		return c.Options
	}
	opts := c.Options
	for i := 0; i < level && i < len(c.activePath); i++ {
		idx := findOptionIndex(opts, c.activePath[i])
		if idx < 0 {
			return nil
		}
		opts = opts[idx].Children
	}
	return opts
}

func (c *Cascader) childrenOf(path []string) []CascaderOption {
	if live := c.liveOption(path); live != nil {
		return live.Children
	}
	return nil
}

func (c *Cascader) liveOption(path []string) *CascaderOption {
	opts := c.Options
	var cur *CascaderOption
	for _, v := range path {
		idx := findOptionIndex(opts, v)
		if idx < 0 {
			return nil
		}
		cur = &opts[idx]
		opts = cur.Children
	}
	return cur
}

func (c *Cascader) syncActiveNodes() {
	c.activeNodes = c.resolvePath(c.activePath)
}

func (c *Cascader) resolvePath(path []string) []CascaderOption {
	var out []CascaderOption
	opts := c.Options
	for _, v := range path {
		idx := findOptionIndex(opts, v)
		if idx < 0 {
			break
		}
		out = append(out, opts[idx])
		opts = opts[idx].Children
	}
	return out
}

func (c *Cascader) pathSelected(path []string) bool {
	for _, p := range c.MultiValue {
		if pathsEqual(p, path) || pathPrefixOf(path, p) || pathPrefixOf(p, path) {
			return true
		}
	}
	return false
}

func findOptionIndex(opts []CascaderOption, value string) int {
	for i := range opts {
		if opts[i].Value == value {
			return i
		}
	}
	return -1
}

func pathValues(nodes []CascaderOption) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Value
	}
	return out
}

func pathsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func pathPrefixOf(prefix, full []string) bool {
	if len(prefix) > len(full) {
		return false
	}
	for i := range prefix {
		if prefix[i] != full[i] {
			return false
		}
	}
	return true
}

func findPathIndex(paths [][]string, path []string) int {
	for i, p := range paths {
		if pathsEqual(p, path) {
			return i
		}
	}
	return -1
}

func clonePaths(paths [][]string) [][]string {
	if paths == nil {
		return nil
	}
	out := make([][]string, len(paths))
	for i, p := range paths {
		out[i] = append([]string(nil), p...)
	}
	return out
}

func mapCascaderPlacement(p CascaderPlacement) primitive.Placement {
	switch p {
	case CascaderBottomRight:
		return primitive.PlaceBottomEnd
	case CascaderTopLeft:
		return primitive.PlaceTopStart
	case CascaderTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}
