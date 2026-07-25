package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design TreeSelect defaults — docs/antd/tree-select.md §6.2 / §6.10
// https://ant.design/components/tree-select
const (
	DefaultTreeSelectMinWidth        = 200.0
	DefaultTreeSelectWidth           = 240.0
	DefaultTreeSelectGap             = 4.0
	DefaultTreeSelectPanelPad        = 4.0
	DefaultTreeSelectItemPadInline   = 8.0
	DefaultTreeSelectItemPadBlock    = 4.0
	DefaultTreeSelectPanelRadius     = 8.0
	DefaultTreeSelectListHeight      = 256.0
	DefaultTreeSelectFocusRingOutset = 1.5
	DefaultTreeSelectIndent          = 18.0
	DefaultTreeSelectSwitcherSize    = 16.0
	DefaultTreeSelectRowHeight       = 28.0
)

// TreeSelectNode is one antd treeData entry.
// value is unique across the tree; title is the display label.
type TreeSelectNode struct {
	Value           string
	Title           string
	Disabled        bool
	DisableCheckbox bool
	Selectable      bool // when false, node not selectable (default true via SelectableSet)
	SelectableSet   bool // if false, Selectable defaults to true
	Checkable       bool
	CheckableSet    bool // if false, inherits TreeCheckable
	IsLeaf          bool
	Loading         bool
	Children        []TreeSelectNode
	// Simple-mode fields (treeDataSimpleMode).
	ID  string
	PID string
}

// DisplayTitle returns Title or falls back to Value.
func (n TreeSelectNode) DisplayTitle() string {
	if n.Title != "" {
		return n.Title
	}
	return n.Value
}

// isSelectable reports whether the node accepts selection.
func (n TreeSelectNode) isSelectable() bool {
	if n.SelectableSet {
		return n.Selectable
	}
	return true
}

// TreeSelectPlacement is popup placement (antd placement).
type TreeSelectPlacement int

const (
	TreeSelectBottomLeft TreeSelectPlacement = iota // default
	TreeSelectBottomRight
	TreeSelectTopLeft
	TreeSelectTopRight
)

// TreeSelectShowCheckedStrategy is showCheckedStrategy for treeCheckable.
type TreeSelectShowCheckedStrategy int

const (
	// TreeSelectSHOW_CHILD shows only leaf checked nodes (antd default).
	TreeSelectSHOW_CHILD TreeSelectShowCheckedStrategy = iota
	// TreeSelectSHOW_PARENT collapses full-child selections to the parent.
	TreeSelectSHOW_PARENT
	// TreeSelectSHOW_ALL shows every checked node including parents.
	TreeSelectSHOW_ALL
)

// TreeSelectLoadData loads children for a non-leaf node (antd loadData).
// Caller should mutate node children / Loading then call TreeSelect.NotifyTreeDataChanged.
type TreeSelectLoadData func(node TreeSelectNode)

// TreeSelect is antd TreeSelect — field trigger + expandable tree popup.
//
//	Column (Wrap)
//	  ├─ Pressable trigger (selector chrome)
//	  └─ AnchoredPopup
//	       └─ panel (search? + tree rows | notFound)
//
// Product contract: docs/antd/tree-select.md §6 (P0 DoD).
type TreeSelect struct {
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

	// Product fields (§6.10 / §6.8 P0).
	TreeData              []TreeSelectNode
	Value                 string   // single
	Values                []string // multiple / checkable
	DefaultValue          string
	DefaultValues         []string
	Placeholder           string
	Size                  InputSize
	Variant               InputVariant
	Status                InputStatus
	Disabled              bool
	Loading               bool
	AllowClear            bool
	ShowSearch            bool
	Open                  bool
	Multiple              bool
	TreeCheckable         bool
	TreeCheckStrictly     bool
	TreeDefaultExpandAll  bool
	TreeLine              bool
	TreeIcon              bool
	ListHeight            float64
	Placement             TreeSelectPlacement
	ShowCheckedStrategy   TreeSelectShowCheckedStrategy
	Title                 string // native title / tooltip
	AriaLabel             string
	Face                  text.Face
	Theme                 *core.Theme
	Viewport              core.Size
	FixedWidth            float64
	SearchValue           string
	TreeDataSimpleMode    bool
	LoadData              TreeSelectLoadData
	PopupMatchSelectWidth bool

	// Expanded keys (value → expanded). nil until first expand interaction.
	Expanded map[string]bool

	// Callbacks.
	OnChange      func(value string)
	OnChangeMulti func(values []string)
	OnSelect      func(value string, node TreeSelectNode)
	OnDeselect    func(value string, node TreeSelectNode)
	OnOpenChange  func(open bool)
	OnSearch      func(value string)
	OnClear       func()
	OnTreeExpand  func(expandedKeys []string)

	// openControlled: SetOpen used (antd open prop).
	openControlled bool
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool
	appliedDefVal  bool
	// controlled: selection only raises OnChange; value waits for SetValue/SetValues.
	Controlled bool

	// Flattened visible rows for keyboard / paint.
	visible     []treeSelectRow
	activeIndex int

	// Loading spinner.
	spinPhase float64
	boundTree *core.Tree

	// resolved tree (after simple-mode convert).
	resolved []TreeSelectNode
}

type treeSelectRow struct {
	Node     TreeSelectNode
	Depth    int
	Expanded bool
	HasKids  bool
	Checked  bool
	Half     bool
	Selected bool
	// Match for search: true if title/value matches query (or no query).
	Match bool
}

// NewTreeSelect creates a TreeSelect with optional treeData roots.
// Defaults (§6.10): middle/outlined, closed, uncontrolled, listHeight=256,
// placement=bottomLeft, showCheckedStrategy=SHOW_CHILD, popupMatchSelectWidth=true.
func NewTreeSelect(placeholder string, treeData ...TreeSelectNode) *TreeSelect {
	ts := &TreeSelect{
		Placeholder:           placeholder,
		TreeData:              append([]TreeSelectNode(nil), treeData...),
		Size:                  InputMiddle,
		Variant:               InputOutlined,
		Status:                InputStatusNone,
		ListHeight:            DefaultTreeSelectListHeight,
		Placement:             TreeSelectBottomLeft,
		ShowCheckedStrategy:   TreeSelectSHOW_CHILD,
		PopupMatchSelectWidth: true,
		activeIndex:           -1,
		Expanded:              make(map[string]bool),
	}
	ts.Nav = core.NewKeyboardNav(core.NavVertical, 0)
	ts.rebuild()
	return ts
}

// Node returns the composition root (trigger + popup host).
func (ts *TreeSelect) Node() core.Node {
	if ts == nil {
		return nil
	}
	if ts.Wrap == nil {
		ts.rebuild()
	}
	return ts.Wrap
}

// ChromeNode returns the trigger Decorated chrome (tests / layout).
func (ts *TreeSelect) ChromeNode() core.Node {
	if ts == nil {
		return nil
	}
	if ts.decor == nil {
		ts.rebuild()
	}
	return ts.decor
}

// Popup returns the anchored popup (tests / advanced hosts).
func (ts *TreeSelect) Popup() *primitive.AnchoredPopup {
	if ts == nil {
		return nil
	}
	return ts.popup
}

// Panel returns the dropdown panel chrome (tests).
func (ts *TreeSelect) Panel() *primitive.Decorated {
	if ts == nil {
		return nil
	}
	return ts.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (ts *TreeSelect) TriggerShell() *primitive.Pressable {
	if ts == nil {
		return nil
	}
	return ts.Root
}

// IsOpen reports whether the dropdown panel is visible.
func (ts *TreeSelect) IsOpen() bool {
	return ts != nil && ts.Open
}

// VisibleRows returns the currently painted tree rows (tests).
func (ts *TreeSelect) VisibleRows() []treeSelectRow {
	if ts == nil {
		return nil
	}
	out := make([]treeSelectRow, len(ts.visible))
	copy(out, ts.visible)
	return out
}

// VisibleTitles returns display titles of visible rows (tests / search).
func (ts *TreeSelect) VisibleTitles() []string {
	if ts == nil {
		return nil
	}
	out := make([]string, 0, len(ts.visible))
	for _, r := range ts.visible {
		out = append(out, r.Node.DisplayTitle())
	}
	return out
}

// GetValue returns the single-select value.
func (ts *TreeSelect) GetValue() string {
	if ts == nil {
		return ""
	}
	return ts.Value
}

// GetValues returns multi/checkable values.
func (ts *TreeSelect) GetValues() []string {
	if ts == nil {
		return nil
	}
	return append([]string(nil), ts.Values...)
}

// DisplayText returns the current field display string.
func (ts *TreeSelect) DisplayText() string {
	if ts == nil {
		return ""
	}
	return ts.computeDisplay()
}

// ExpandedKeys returns currently expanded node values.
func (ts *TreeSelect) ExpandedKeys() []string {
	if ts == nil || ts.Expanded == nil {
		return nil
	}
	var keys []string
	for k, v := range ts.Expanded {
		if v {
			keys = append(keys, k)
		}
	}
	return keys
}

// ---------------------------------------------------------------------------
// Setters — product API (§6.10)
// ---------------------------------------------------------------------------

// SetTreeData replaces treeData and rebuilds.
func (ts *TreeSelect) SetTreeData(nodes ...TreeSelectNode) {
	if ts == nil {
		return
	}
	ts.TreeData = append([]TreeSelectNode(nil), nodes...)
	ts.resolved = nil
	if ts.TreeDefaultExpandAll {
		ts.seedExpandAll()
	}
	ts.refreshTree(true)
	ts.refreshDisplay()
}

// NotifyTreeDataChanged rebuilds after loadData mutates TreeData.
func (ts *TreeSelect) NotifyTreeDataChanged() {
	if ts == nil {
		return
	}
	ts.resolved = nil
	ts.refreshTree(true)
	ts.syncLoadingTicker()
	ts.refreshDisplay()
}

// SetValue sets the single-select value. Does not fire OnChange (API write).
func (ts *TreeSelect) SetValue(v string) {
	if ts == nil {
		return
	}
	ts.Value = v
	ts.appliedDefVal = true
	ts.refreshDisplay()
	if ts.Open {
		ts.refreshTree(false)
	}
}

// SetDefaultValue seeds uncontrolled value when Value is still empty.
func (ts *TreeSelect) SetDefaultValue(v string) {
	if ts == nil {
		return
	}
	ts.DefaultValue = v
	if !ts.appliedDefVal && ts.Value == "" && !ts.isMulti() {
		ts.appliedDefVal = true
		ts.Value = v
		ts.refreshDisplay()
	}
}

// SetValues sets multi/checkable values. Does not fire OnChangeMulti.
func (ts *TreeSelect) SetValues(vals []string) {
	if ts == nil {
		return
	}
	ts.Values = append([]string(nil), vals...)
	ts.appliedDefVal = true
	ts.refreshDisplay()
	if ts.Open {
		ts.refreshTree(false)
	}
}

// SetDefaultValues seeds uncontrolled multi values.
func (ts *TreeSelect) SetDefaultValues(vals []string) {
	if ts == nil {
		return
	}
	ts.DefaultValues = append([]string(nil), vals...)
	if !ts.appliedDefVal && len(ts.Values) == 0 && ts.isMulti() {
		ts.appliedDefVal = true
		ts.Values = append([]string(nil), vals...)
		ts.refreshDisplay()
	}
}

// SetPlaceholder sets the empty-field placeholder.
func (ts *TreeSelect) SetPlaceholder(p string) {
	if ts == nil {
		return
	}
	ts.Placeholder = p
	ts.refreshDisplay()
}

// SetDisabled toggles disabled (no open / no select).
func (ts *TreeSelect) SetDisabled(d bool) {
	if ts == nil {
		return
	}
	ts.Disabled = d
	if d && ts.Open {
		ts.applyOpen(false, true)
	}
	if ts.Root != nil {
		ts.Root.SetDisabled(d)
	}
	ts.applyChrome()
	ts.applyA11y()
}

// SetLoading toggles the suffix loading spinner (Ticker).
func (ts *TreeSelect) SetLoading(v bool) {
	if ts == nil {
		return
	}
	ts.Loading = v
	ts.syncLoadingTicker()
	ts.rebuild()
}

// SetSize sets control height ladder (small/middle/large).
func (ts *TreeSelect) SetSize(sz InputSize) {
	if ts == nil {
		return
	}
	ts.Size = sz
	ts.rebuild()
}

// SetVariant sets outlined | filled | borderless | underlined.
func (ts *TreeSelect) SetVariant(v InputVariant) {
	if ts == nil {
		return
	}
	ts.Variant = v
	ts.applyChrome()
}

// SetStatus sets validation chrome (error | warning).
func (ts *TreeSelect) SetStatus(st InputStatus) {
	if ts == nil {
		return
	}
	ts.Status = st
	ts.applyChrome()
	ts.applyA11y()
}

// SetAllowClear toggles the clear affordance.
func (ts *TreeSelect) SetAllowClear(v bool) {
	if ts == nil {
		return
	}
	ts.AllowClear = v
	ts.rebuild()
}

// SetShowSearch toggles in-field / panel search filtering.
func (ts *TreeSelect) SetShowSearch(v bool) {
	if ts == nil {
		return
	}
	ts.ShowSearch = v
	ts.rebuild()
}

// SetOpen sets controlled open state.
func (ts *TreeSelect) SetOpen(open bool) {
	if ts == nil {
		return
	}
	ts.openControlled = true
	ts.applyOpen(open, true)
}

// SetDefaultOpen seeds initial open once (uncontrolled).
func (ts *TreeSelect) SetDefaultOpen(open bool) {
	if ts == nil {
		return
	}
	ts.defaultOpen = open
	ts.defaultOpenSet = true
	if !ts.appliedDefault && !ts.openControlled {
		ts.appliedDefault = true
		ts.applyOpen(open, true)
	}
}

// SetMultiple enables multi-select (antd multiple). treeCheckable forces multi.
func (ts *TreeSelect) SetMultiple(v bool) {
	if ts == nil {
		return
	}
	ts.Multiple = v
	ts.refreshDisplay()
	if ts.Open {
		ts.refreshTree(false)
	}
}

// SetTreeCheckable shows checkboxes and forces multi-select semantics.
func (ts *TreeSelect) SetTreeCheckable(v bool) {
	if ts == nil {
		return
	}
	ts.TreeCheckable = v
	if v {
		ts.Multiple = true
	}
	ts.refreshDisplay()
	if ts.Open {
		ts.refreshTree(true)
	}
}

// SetTreeCheckStrictly disables parent-child check linkage.
func (ts *TreeSelect) SetTreeCheckStrictly(v bool) {
	if ts == nil {
		return
	}
	ts.TreeCheckStrictly = v
	if ts.Open {
		ts.refreshTree(false)
	}
}

// SetTreeDefaultExpandAll expands all nodes when tree first materializes.
func (ts *TreeSelect) SetTreeDefaultExpandAll(v bool) {
	if ts == nil {
		return
	}
	ts.TreeDefaultExpandAll = v
	if v {
		ts.seedExpandAll()
		if ts.Open {
			ts.refreshTree(true)
		}
	}
}

// SetTreeLine toggles tree guide lines in the panel.
func (ts *TreeSelect) SetTreeLine(v bool) {
	if ts == nil {
		return
	}
	ts.TreeLine = v
	if ts.Open {
		ts.refreshTree(true)
	}
}

// SetTreeIcon toggles a leading icon slot before titles.
func (ts *TreeSelect) SetTreeIcon(v bool) {
	if ts == nil {
		return
	}
	ts.TreeIcon = v
	if ts.Open {
		ts.refreshTree(true)
	}
}

// SetShowCheckedStrategy sets checkable display strategy.
func (ts *TreeSelect) SetShowCheckedStrategy(s TreeSelectShowCheckedStrategy) {
	if ts == nil {
		return
	}
	ts.ShowCheckedStrategy = s
	ts.refreshDisplay()
}

// SetPlacement sets popup placement.
func (ts *TreeSelect) SetPlacement(p TreeSelectPlacement) {
	if ts == nil {
		return
	}
	ts.Placement = p
	if ts.popup != nil {
		ts.popup.Placement = mapTreeSelectPlacement(p)
	}
}

// SetListHeight sets the panel max scroll height (product field; hard clip is P1).
func (ts *TreeSelect) SetListHeight(h float64) {
	if ts == nil {
		return
	}
	if h <= 0 {
		h = DefaultTreeSelectListHeight
	}
	ts.ListHeight = h
	_ = ts.ListHeight
}

// SetTitle sets the native title / tooltip string.
func (ts *TreeSelect) SetTitle(t string) {
	if ts == nil {
		return
	}
	ts.Title = t
	ts.applyA11y()
}

// SetFixedWidth sets the trigger width (0 → default 240).
func (ts *TreeSelect) SetFixedWidth(w float64) {
	if ts == nil {
		return
	}
	ts.FixedWidth = w
	if ts.decor != nil {
		if w <= 0 {
			w = DefaultTreeSelectWidth
		}
		ts.decor.Width = w
		ts.decor.MarkNeedsLayout()
	}
}

// SetSearchValue sets the search query (filters tree when showSearch).
func (ts *TreeSelect) SetSearchValue(q string) {
	if ts == nil {
		return
	}
	ts.SearchValue = q
	if ts.searchEd != nil {
		was := ts.searchEd.OnChange
		ts.searchEd.OnChange = nil
		ts.searchEd.Value = q
		ts.searchEd.OnChange = was
	}
	if ts.OnSearch != nil {
		ts.OnSearch(q)
	}
	// Expand ancestors of matches while searching.
	if strings.TrimSpace(q) != "" {
		ts.expandForSearch(q)
	}
	ts.refreshTree(true)
}

// SetTreeDataSimpleMode enables flat {id,pId} treeData conversion.
func (ts *TreeSelect) SetTreeDataSimpleMode(v bool) {
	if ts == nil {
		return
	}
	ts.TreeDataSimpleMode = v
	ts.resolved = nil
	ts.refreshTree(true)
}

// SetLoadData sets the async loadData callback.
func (ts *TreeSelect) SetLoadData(fn TreeSelectLoadData) {
	if ts == nil {
		return
	}
	ts.LoadData = fn
}

// SetPopupMatchSelectWidth matches panel min-width to trigger.
func (ts *TreeSelect) SetPopupMatchSelectWidth(v bool) {
	if ts == nil {
		return
	}
	ts.PopupMatchSelectWidth = v
}

// SetControlled toggles controlled selection mode.
func (ts *TreeSelect) SetControlled(v bool) {
	if ts == nil {
		return
	}
	ts.Controlled = v
}

// SetOnChange sets single-value change handler.
func (ts *TreeSelect) SetOnChange(fn func(string)) {
	if ts == nil {
		return
	}
	ts.OnChange = fn
}

// SetOnChangeMulti sets multi-value change handler.
func (ts *TreeSelect) SetOnChangeMulti(fn func([]string)) {
	if ts == nil {
		return
	}
	ts.OnChangeMulti = fn
}

// SetOnSelect sets select callback.
func (ts *TreeSelect) SetOnSelect(fn func(value string, node TreeSelectNode)) {
	if ts == nil {
		return
	}
	ts.OnSelect = fn
}

// SetOnDeselect sets deselect callback.
func (ts *TreeSelect) SetOnDeselect(fn func(value string, node TreeSelectNode)) {
	if ts == nil {
		return
	}
	ts.OnDeselect = fn
}

// SetOnOpenChange sets open change handler.
func (ts *TreeSelect) SetOnOpenChange(fn func(open bool)) {
	if ts == nil {
		return
	}
	ts.OnOpenChange = fn
}

// SetOnSearch sets search query handler.
func (ts *TreeSelect) SetOnSearch(fn func(string)) {
	if ts == nil {
		return
	}
	ts.OnSearch = fn
}

// SetOnClear sets clear handler.
func (ts *TreeSelect) SetOnClear(fn func()) {
	if ts == nil {
		return
	}
	ts.OnClear = fn
}

// SetTheme sets an explicit theme override.
func (ts *TreeSelect) SetTheme(th *core.Theme) {
	if ts == nil {
		return
	}
	ts.Theme = th
	ts.rebuild()
}

// SetFace sets the text face.
func (ts *TreeSelect) SetFace(face text.Face) {
	if ts == nil {
		return
	}
	ts.Face = face
	ts.rebuild()
}

// SetAriaLabel sets the accessible name.
func (ts *TreeSelect) SetAriaLabel(name string) {
	if ts == nil {
		return
	}
	ts.AriaLabel = name
	ts.applyA11y()
}

// AttachTicker registers loading spinner animation.
func (ts *TreeSelect) AttachTicker(t *core.Tree) {
	if ts == nil || t == nil {
		return
	}
	ts.boundTree = t
	t.BindTicker(ts, ts.Loading || ts.anyNodeLoading())
}

// Tick advances the loading spinner. Implements core.Ticker when Loading.
func (ts *TreeSelect) Tick(dt float64) bool {
	if ts == nil {
		return false
	}
	active := ts.Loading || ts.anyNodeLoading()
	if !active {
		return false
	}
	ts.spinPhase += dt * 1.4
	if ts.spinPhase > 1 {
		ts.spinPhase -= 1
	}
	if ts.spinner != nil {
		ts.spinner.MarkNeedsPaint()
	} else if ts.decor != nil {
		ts.decor.MarkNeedsPaint()
	}
	return active
}

// Clear empties the selection (allowClear path / API).
func (ts *TreeSelect) Clear() {
	if ts == nil || ts.Disabled {
		return
	}
	ts.appliedDefVal = true
	if ts.Controlled {
		if ts.isMulti() {
			if ts.OnChangeMulti != nil {
				ts.OnChangeMulti(nil)
			}
		} else if ts.OnChange != nil {
			ts.OnChange("")
		}
	} else {
		ts.Value = ""
		ts.Values = nil
		if ts.isMulti() {
			if ts.OnChangeMulti != nil {
				ts.OnChangeMulti(nil)
			}
		} else if ts.OnChange != nil {
			ts.OnChange("")
		}
	}
	ts.SearchValue = ""
	if ts.searchEd != nil {
		ts.searchEd.Value = ""
	}
	if ts.OnClear != nil {
		ts.OnClear()
	}
	ts.refreshDisplay()
	ts.refreshTree(false)
	ts.applyChrome()
}

// SelectValue selects a node by value (tests / programmatic).
func (ts *TreeSelect) SelectValue(v string) {
	if ts == nil || ts.Disabled {
		return
	}
	n, ok := ts.findNode(v)
	if !ok || n.Disabled || !n.isSelectable() {
		return
	}
	ts.applySelect(n)
}

// ToggleCheck toggles a checkable node by value (tests).
func (ts *TreeSelect) ToggleCheck(v string) {
	if ts == nil || ts.Disabled || !ts.TreeCheckable {
		return
	}
	n, ok := ts.findNode(v)
	if !ok || n.Disabled || n.DisableCheckbox {
		return
	}
	ts.toggleCheck(n)
}

// ExpandValue expands a node by value (tests / loadData path).
func (ts *TreeSelect) ExpandValue(v string) {
	if ts == nil {
		return
	}
	if ts.Expanded == nil {
		ts.Expanded = make(map[string]bool)
	}
	ts.Expanded[v] = true
	n, ok := ts.findNode(v)
	if ok {
		ts.maybeLoadData(n)
	}
	ts.refreshTree(true)
	if ts.OnTreeExpand != nil {
		ts.OnTreeExpand(ts.ExpandedKeys())
	}
}

// CollapseValue collapses a node by value.
func (ts *TreeSelect) CollapseValue(v string) {
	if ts == nil {
		return
	}
	if ts.Expanded == nil {
		ts.Expanded = make(map[string]bool)
	}
	ts.Expanded[v] = false
	ts.refreshTree(true)
	if ts.OnTreeExpand != nil {
		ts.OnTreeExpand(ts.ExpandedKeys())
	}
}

// HandleKey for arrow/enter/escape when focused.
func (ts *TreeSelect) HandleKey(ev *core.KeyEvent) {
	if ts == nil || ts.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if !ts.Open {
		if ev.Key == "Enter" || ev.Key == " " || ev.Key == "ArrowDown" || ev.Key == "Down" {
			ts.requestOpen(true)
			ev.Handled = true
		}
		return
	}
	if ts.Nav != nil && ts.Nav.HandleKey(ev.Key) {
		ts.activeIndex = ts.Nav.Index
		ts.refreshTree(false)
		ev.Handled = true
		return
	}
	switch ev.Key {
	case "Enter", " ":
		if ts.activeIndex >= 0 && ts.activeIndex < len(ts.visible) {
			row := ts.visible[ts.activeIndex]
			if ts.TreeCheckable {
				ts.toggleCheck(row.Node)
			} else {
				ts.applySelect(row.Node)
			}
		}
		ev.Handled = true
	case "Escape", "Esc":
		ts.applyOpen(false, false)
		ev.Handled = true
	case "ArrowRight", "Right":
		if ts.activeIndex >= 0 && ts.activeIndex < len(ts.visible) {
			row := ts.visible[ts.activeIndex]
			if row.HasKids || !row.Node.IsLeaf {
				ts.ExpandValue(row.Node.Value)
			}
		}
		ev.Handled = true
	case "ArrowLeft", "Left":
		if ts.activeIndex >= 0 && ts.activeIndex < len(ts.visible) {
			row := ts.visible[ts.activeIndex]
			if row.Expanded {
				ts.CollapseValue(row.Node.Value)
			}
		}
		ev.Handled = true
	}
}

// ---------------------------------------------------------------------------
// Internals
// ---------------------------------------------------------------------------

func (ts *TreeSelect) theme() *core.Theme {
	var n core.Node
	if ts.Wrap != nil {
		n = ts.Wrap
	}
	return themeOf(ts.Theme, n)
}

func (ts *TreeSelect) controlHeight() float64 {
	th := ts.theme()
	switch ts.Size {
	case InputSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case InputLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (ts *TreeSelect) fontSize() float64 {
	th := ts.theme()
	switch ts.Size {
	case InputSmall:
		return th.SizeOr(core.TokenFontSizeSM, 12)
	case InputLarge:
		return th.SizeOr(core.TokenFontSizeLG, 16)
	default:
		return th.SizeOr(core.TokenFontSize, 14)
	}
}

func (ts *TreeSelect) isMulti() bool {
	return ts.Multiple || ts.TreeCheckable
}

func (ts *TreeSelect) hasValue() bool {
	if ts.isMulti() {
		return len(ts.displayValues()) > 0
	}
	return ts.Value != ""
}

func (ts *TreeSelect) rebuild() {
	if ts == nil {
		return
	}
	// apply default value once
	if !ts.appliedDefVal {
		if ts.isMulti() && len(ts.Values) == 0 && len(ts.DefaultValues) > 0 {
			ts.Values = append([]string(nil), ts.DefaultValues...)
			ts.appliedDefVal = true
		} else if !ts.isMulti() && ts.Value == "" && ts.DefaultValue != "" {
			ts.Value = ts.DefaultValue
			ts.appliedDefVal = true
		}
	}
	if ts.TreeDefaultExpandAll && len(ts.Expanded) == 0 {
		ts.seedExpandAll()
	}

	th := ts.theme()
	h := ts.controlHeight()
	padH := th.SizeOr(core.TokenControlPaddingInline, 11)
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	fontSz := ts.fontSize()
	wasOpen := ts.Open

	// --- field content ---
	content := ts.buildSelectorContent(th, fontSz)

	// clear button
	var clearNode core.Node
	if ts.AllowClear && ts.hasValue() && !ts.Disabled {
		x := primitive.NewIcon("close")
		x.Size = 10
		x.Color = th.Color(core.TokenColorTextSecondary)
		ts.clearBtn = primitive.NewPressable(x)
		ts.clearBtn.Focusable = false
		ts.clearBtn.ShowFocusRing = false
		ts.clearBtn.Base().Role = "button"
		ts.clearBtn.Base().Label = "clear"
		ts.clearBtn.Click = func() { ts.Clear() }
		clearNode = ts.clearBtn
	} else {
		ts.clearBtn = nil
	}

	// suffix: loading spinner or chevron
	var suffixNode core.Node
	ts.spinner = nil
	if ts.Loading {
		spin := primitive.NewCanvas(14, 14, ts.paintSpinner)
		ts.spinner = spin
		suffixNode = spin
	} else {
		ts.suffix = primitive.NewIcon("chevron-down")
		ts.suffix.Size = 12
		sec := th.Color(core.TokenColorTextSecondary)
		if sec.A < 0.3 {
			sec = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
		}
		ts.suffix.Color = sec
		suffixNode = ts.suffix
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

	ts.decor = primitive.NewDecorated(row)
	ts.decor.Padding = primitive.Symmetric(padH, 0)
	ts.decor.Radius = radius
	ts.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	ts.decor.MinHeight = h
	ts.decor.Height = h
	w := ts.FixedWidth
	if w <= 0 {
		w = DefaultTreeSelectWidth
	}
	ts.decor.MinWidth = DefaultTreeSelectMinWidth
	ts.decor.Width = w
	ts.decor.SetCenterContent(true)
	ts.decor.StretchChild = true
	ts.applyChrome()

	// list + panel
	ts.list = primitive.Column()
	ts.list.Gap = 0
	ts.list.CrossAlign = core.CrossStart
	ts.list.Base().Role = "tree"

	ts.panel = primitive.NewDecorated(ts.list)
	ts.panel.Padding = primitive.All(DefaultTreeSelectPanelPad)
	ts.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultTreeSelectPanelRadius)
	ts.panel.Background = th.Color(core.TokenColorBgContainer)
	ts.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	ts.panel.BorderColor = th.Color(core.TokenColorBorder)
	ts.panel.MinWidth = DefaultTreeSelectMinWidth
	// ListHeight is product default 256 (virtual scroll / hard clip is P1).
	_ = ts.ListHeight
	ts.panel.Base().Role = "listbox"

	if ts.popup == nil {
		ts.popup = primitive.NewAnchoredPopup(ts.panel)
	} else {
		ts.popup.Content = ts.panel
	}
	ts.popup.Placement = mapTreeSelectPlacement(ts.Placement)
	ts.popup.Gap = DefaultTreeSelectGap
	ts.popup.DismissOnOutside = true
	ts.popup.OnDismiss = func() {
		ts.Open = false
		ts.SearchValue = ""
		if ts.searchEd != nil {
			ts.searchEd.Value = ""
		}
		if ts.OnOpenChange != nil {
			ts.OnOpenChange(false)
		}
		ts.applyChrome()
		ts.applyA11y()
	}

	// trigger shell
	if ts.Root == nil {
		ts.Root = primitive.NewPressable(ts.decor)
	} else {
		ts.Root.ClearChildren()
		ts.Root.AddChild(ts.decor)
	}
	ts.Root.Focusable = true
	ts.Root.ShowFocusRing = true
	ts.Root.FocusRingRadius = ts.decor.Radius
	ts.Root.FocusRingOutset = DefaultTreeSelectFocusRingOutset
	ts.Root.SetDisabled(ts.Disabled)
	ts.Root.OnStateChange = func() {
		ts.applyChrome()
	}
	ts.Root.Click = func() {
		if ts.Disabled {
			return
		}
		ts.requestOpen(!ts.Open)
	}

	if ts.Wrap == nil {
		ts.Wrap = primitive.Column(ts.Root, ts.popup)
	} else {
		ts.Wrap.ClearChildren()
		ts.Wrap.AddChild(ts.Root)
		ts.Wrap.AddChild(ts.popup)
	}
	ts.Wrap.CrossAlign = core.CrossStart
	ts.Wrap.Gap = 0
	ts.Wrap.SetThemeHook(func(*core.Theme) { ts.rebuild() })
	ts.Wrap.MarkNeedsLayout()
	ts.Wrap.MarkNeedsPaint()

	ts.applyA11y()
	ts.refreshTree(true)

	// Restore open popup without re-entering applyOpen→rebuild (Select.rebuildKeepOpen).
	if wasOpen {
		ts.Open = true
		ts.syncPopupGeometry()
		if ts.popup != nil {
			ts.popup.SetOpen(true)
		}
		ts.applyChrome()
	} else if ts.defaultOpenSet && !ts.openControlled && !ts.appliedDefault {
		ts.appliedDefault = true
		ts.applyOpen(ts.defaultOpen, true)
	}
}

func (ts *TreeSelect) buildSelectorContent(th *core.Theme, fontSz float64) core.Node {
	if ts.isMulti() {
		return ts.buildMultiContent(th, fontSz)
	}
	if ts.ShowSearch && ts.Open {
		return ts.buildSearchEditor(th, fontSz)
	}
	label := ts.computeDisplay()
	ts.display = primitive.NewText(label)
	ts.display.FontSize = fontSz
	ts.display.Face = ts.Face
	ts.display.Color = ts.displayColor(th)
	return ts.display
}

func (ts *TreeSelect) buildSearchEditor(th *core.Theme, fontSz float64) core.Node {
	if ts.searchEd == nil {
		ts.searchEd = primitive.NewEditableText()
	}
	was := ts.searchEd.OnChange
	ts.searchEd.OnChange = nil
	ts.searchEd.Value = ts.SearchValue
	ts.searchEd.Cursor = len([]rune(ts.SearchValue))
	ts.searchEd.SelAnchor = ts.searchEd.Cursor
	ts.searchEd.OnChange = was

	ts.searchEd.FontSize = fontSz
	ts.searchEd.Face = ts.Face
	ts.searchEd.Color = th.Color(core.TokenColorText)
	ts.searchEd.ShowFocusRing = false
	ts.searchEd.Placeholder = ts.Placeholder
	if ts.searchEd.Placeholder == "" {
		ts.searchEd.Placeholder = "Search"
	}
	ts.searchEd.PlaceholderColor = th.Color(core.TokenColorTextSecondary)
	ts.searchEd.OnChange = func(v string) {
		if ts.Disabled {
			return
		}
		ts.SearchValue = v
		if ts.OnSearch != nil {
			ts.OnSearch(v)
		}
		if strings.TrimSpace(v) != "" {
			ts.expandForSearch(v)
		}
		ts.refreshTree(true)
		if !ts.Open {
			ts.requestOpen(true)
		}
	}
	ts.display = primitive.NewText(ts.SearchValue)
	ts.display.FontSize = fontSz
	ts.display.Face = ts.Face
	ts.display.Color = th.Color(core.TokenColorText)
	return ts.searchEd
}

func (ts *TreeSelect) buildMultiContent(th *core.Theme, fontSz float64) core.Node {
	row := primitive.Row()
	row.Gap = 4
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainStart

	vals := ts.displayValues()
	if len(vals) == 0 {
		ph := ts.Placeholder
		if ph == "" {
			ph = "Please select"
		}
		ts.display = primitive.NewText(ph)
		ts.display.FontSize = fontSz
		ts.display.Face = ts.Face
		ts.display.Color = ts.displayColor(th)
		row.AddChild(ts.display)
	} else {
		// Join titles with comma for compact multi display (P0; tags optional later).
		parts := make([]string, 0, len(vals))
		for _, v := range vals {
			parts = append(parts, ts.labelForValue(v))
		}
		txt := strings.Join(parts, ", ")
		ts.display = primitive.NewText(txt)
		ts.display.FontSize = fontSz
		ts.display.Face = ts.Face
		ts.display.Color = ts.displayColor(th)
		row.AddChild(ts.display)
	}
	if ts.ShowSearch && ts.Open {
		row.AddChild(ts.buildSearchEditor(th, fontSz))
	}
	return row
}

func (ts *TreeSelect) computeDisplay() string {
	if ts.isMulti() {
		vals := ts.displayValues()
		if len(vals) == 0 {
			return ts.placeholderText()
		}
		parts := make([]string, 0, len(vals))
		for _, v := range vals {
			parts = append(parts, ts.labelForValue(v))
		}
		return strings.Join(parts, ", ")
	}
	if ts.Value != "" {
		return ts.labelForValue(ts.Value)
	}
	return ts.placeholderText()
}

func (ts *TreeSelect) placeholderText() string {
	if ts.Placeholder != "" {
		return ts.Placeholder
	}
	return "Please select"
}

func (ts *TreeSelect) labelForValue(v string) string {
	if n, ok := ts.findNode(v); ok {
		return n.DisplayTitle()
	}
	return v
}

func (ts *TreeSelect) displayColor(th *core.Theme) render.RGBA {
	if ts.Disabled {
		col := th.Color(core.TokenColorDisabledText)
		if col.A < 0.2 {
			col = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
		return col
	}
	if ts.hasValue() {
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

func (ts *TreeSelect) applyChrome() {
	if ts == nil || ts.decor == nil {
		return
	}
	th := ts.theme()
	h := ts.controlHeight()
	ts.decor.MinHeight = h
	ts.decor.Height = h
	ts.decor.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	ts.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)

	bg := th.Color(core.TokenColorBgContainer)
	bd := th.Color(core.TokenColorBorder)
	switch ts.Variant {
	case InputFilled:
		bg = th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bd = render.RGBA{}
		ts.decor.BorderWidth = 0
	case InputBorderless:
		bg = render.RGBA{}
		bd = render.RGBA{}
		ts.decor.BorderWidth = 0
	case InputUnderlined:
		bg = render.RGBA{}
		ts.decor.Radius = 0
	default:
		// outlined
	}

	if ts.Disabled {
		dbg := th.Color(core.TokenColorDisabledBg)
		if dbg.A < 0.05 {
			dbg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		bg = dbg
		bd = th.Color(core.TokenColorBorder)
		ts.decor.Background = bg
		ts.decor.BorderColor = bd
		if ts.display != nil {
			ts.display.Color = th.Color(core.TokenColorDisabledText)
			if ts.display.Color.A < 0.2 {
				ts.display.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
		}
		ts.decor.MarkNeedsPaint()
		return
	}

	switch ts.Status {
	case InputStatusError:
		bd = th.Color(core.TokenColorError)
	case InputStatusWarning:
		bd = th.Color(core.TokenColorWarning)
	default:
		if ts.Root != nil && (ts.Root.State.Focused || ts.Open) {
			bd = th.Color(core.TokenColorPrimary)
		} else if ts.Root != nil && ts.Root.State.Hovered {
			hb := th.Color(core.TokenColorBorderHover)
			if hb.A < 0.5 {
				hb = th.Color(core.TokenColorPrimaryHover)
			}
			bd = hb
		}
	}
	if ts.Variant == InputUnderlined {
		ts.decor.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	}
	ts.decor.Background = bg
	ts.decor.BorderColor = bd
	if ts.display != nil {
		ts.display.Color = ts.displayColor(th)
	}
	ts.decor.MarkNeedsPaint()
}

func (ts *TreeSelect) applyA11y() {
	if ts == nil || ts.Root == nil {
		return
	}
	ts.Root.Base().Role = "combobox"
	label := ts.AriaLabel
	if label == "" {
		label = ts.Title
	}
	if label == "" {
		label = ts.Placeholder
	}
	if label == "" {
		label = "TreeSelect"
	}
	ts.Root.Base().Label = label
	if ts.Status == InputStatusError {
		ts.Root.Base().Label = label + " invalid"
	}
}

func (ts *TreeSelect) requestOpen(open bool) {
	if ts.openControlled {
		// raise only; parent drives SetOpen
		if ts.OnOpenChange != nil {
			ts.OnOpenChange(open)
		}
		return
	}
	ts.applyOpen(open, false)
}

func (ts *TreeSelect) applyOpen(open bool, skipCallback bool) {
	if ts == nil {
		return
	}
	if ts.Disabled && open {
		return
	}
	prev := ts.Open
	ts.Open = open
	if ts.popup == nil {
		ts.applyChrome()
		if !skipCallback && prev != open && ts.OnOpenChange != nil {
			ts.OnOpenChange(open)
		}
		return
	}
	if open {
		if ts.TreeDefaultExpandAll {
			ts.seedExpandAll()
		}
		ts.refreshTree(true)
		ts.syncPopupGeometry()
		ts.popup.SetOpen(true)
		// Swap in search editor only on open transition (avoid rebuild re-entry).
		if ts.ShowSearch && prev != open {
			ts.rebuildKeepOpen()
		}
	} else {
		ts.popup.SetOpen(false)
		if ts.SearchValue != "" {
			ts.SearchValue = ""
			if ts.searchEd != nil {
				ts.searchEd.Value = ""
			}
		}
		// Restore label when leaving search field.
		if ts.ShowSearch && prev {
			ts.rebuild()
			ts.Open = false
			if ts.popup != nil {
				ts.popup.SetOpen(false)
			}
		}
	}
	ts.applyChrome()
	ts.applyA11y()
	if !skipCallback && prev != open && ts.OnOpenChange != nil {
		ts.OnOpenChange(open)
	}
}

// rebuildKeepOpen rebuilds chrome while keeping popup open (search editor swap).
func (ts *TreeSelect) rebuildKeepOpen() {
	open := ts.Open
	ts.rebuild()
	if open && ts.popup != nil {
		ts.Open = true
		ts.syncPopupGeometry()
		ts.popup.SetOpen(true)
		ts.applyChrome()
	}
}

func (ts *TreeSelect) syncPopupGeometry() {
	if ts == nil || ts.popup == nil || ts.Root == nil {
		return
	}
	ts.popup.UpdateAnchorFromNode(ts.Root)
	if ts.Viewport.Width > 0 {
		ts.popup.Viewport = ts.Viewport
	}
	if ts.PopupMatchSelectWidth && ts.decor != nil {
		w := ts.decor.Width
		if w <= 0 {
			w = DefaultTreeSelectWidth
		}
		if ts.panel != nil {
			ts.panel.MinWidth = w
		}
	}
}

func (ts *TreeSelect) refreshDisplay() {
	if ts == nil {
		return
	}
	// Multi / allowClear / search-open need full rebuild for chrome affordances.
	if ts.isMulti() || ts.AllowClear || (ts.ShowSearch && ts.Open) {
		open := ts.Open
		ts.rebuild()
		if open && ts.popup != nil {
			ts.Open = true
			ts.syncPopupGeometry()
			ts.popup.SetOpen(true)
			ts.applyChrome()
		}
		return
	}
	if ts.display == nil {
		return
	}
	th := ts.theme()
	ts.display.SetValue(ts.computeDisplay())
	ts.display.Color = ts.displayColor(th)
	ts.applyChrome()
}

func (ts *TreeSelect) resolvedTree() []TreeSelectNode {
	if ts == nil {
		return nil
	}
	if ts.resolved != nil {
		return ts.resolved
	}
	if ts.TreeDataSimpleMode {
		ts.resolved = convertSimpleTreeData(ts.TreeData)
	} else {
		ts.resolved = ts.TreeData
	}
	return ts.resolved
}

func convertSimpleTreeData(flat []TreeSelectNode) []TreeSelectNode {
	if len(flat) == 0 {
		return nil
	}
	// index by ID (or Value)
	type item struct {
		node TreeSelectNode
		kids []string
	}
	byID := make(map[string]*item, len(flat))
	order := make([]string, 0, len(flat))
	for _, n := range flat {
		id := n.ID
		if id == "" {
			id = n.Value
		}
		cp := n
		cp.Children = nil
		byID[id] = &item{node: cp}
		order = append(order, id)
	}
	var roots []string
	for _, n := range flat {
		id := n.ID
		if id == "" {
			id = n.Value
		}
		pid := n.PID
		if pid == "" || pid == "0" {
			roots = append(roots, id)
			continue
		}
		if p, ok := byID[pid]; ok {
			p.kids = append(p.kids, id)
		} else {
			roots = append(roots, id)
		}
	}
	var build func(id string) TreeSelectNode
	build = func(id string) TreeSelectNode {
		it := byID[id]
		if it == nil {
			return TreeSelectNode{Value: id}
		}
		n := it.node
		for _, cid := range it.kids {
			n.Children = append(n.Children, build(cid))
		}
		return n
	}
	out := make([]TreeSelectNode, 0, len(roots))
	for _, r := range roots {
		out = append(out, build(r))
	}
	return out
}

func (ts *TreeSelect) seedExpandAll() {
	if ts.Expanded == nil {
		ts.Expanded = make(map[string]bool)
	}
	var walk func([]TreeSelectNode)
	walk = func(nodes []TreeSelectNode) {
		for _, n := range nodes {
			if len(n.Children) > 0 || (!n.IsLeaf && ts.LoadData != nil) {
				ts.Expanded[n.Value] = true
			}
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(ts.resolvedTree())
}

func (ts *TreeSelect) expandForSearch(q string) {
	if ts.Expanded == nil {
		ts.Expanded = make(map[string]bool)
	}
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return
	}
	var walk func([]TreeSelectNode, []string) bool
	walk = func(nodes []TreeSelectNode, ancestors []string) bool {
		any := false
		for _, n := range nodes {
			title := strings.ToLower(n.DisplayTitle())
			val := strings.ToLower(n.Value)
			self := strings.Contains(title, q) || strings.Contains(val, q)
			childHit := false
			if len(n.Children) > 0 {
				next := append(append([]string(nil), ancestors...), n.Value)
				childHit = walk(n.Children, next)
			}
			if self || childHit {
				any = true
				for _, a := range ancestors {
					ts.Expanded[a] = true
				}
				if childHit {
					ts.Expanded[n.Value] = true
				}
			}
		}
		return any
	}
	walk(ts.resolvedTree(), nil)
}

func (ts *TreeSelect) isExpanded(v string) bool {
	if ts.Expanded == nil {
		return false
	}
	return ts.Expanded[v]
}

func (ts *TreeSelect) hasChildren(n TreeSelectNode) bool {
	return len(n.Children) > 0
}

func (ts *TreeSelect) canExpand(n TreeSelectNode) bool {
	if n.IsLeaf {
		return false
	}
	if len(n.Children) > 0 {
		return true
	}
	// non-leaf without children → loadData candidate
	return ts.LoadData != nil
}

func (ts *TreeSelect) refreshTree(resetActive bool) {
	if ts == nil || ts.list == nil {
		return
	}
	ts.buildVisible()
	if resetActive {
		ts.activeIndex = -1
		if len(ts.visible) > 0 {
			// prefer selected
			for i, r := range ts.visible {
				if r.Selected || r.Checked {
					ts.activeIndex = i
					break
				}
			}
			if ts.activeIndex < 0 {
				ts.activeIndex = 0
			}
		}
	}
	if ts.Nav != nil {
		ts.Nav.Count = len(ts.visible)
		if ts.activeIndex >= 0 {
			ts.Nav.Index = ts.activeIndex
		}
	}
	ts.paintTreeRows()
}

func (ts *TreeSelect) buildVisible() {
	ts.visible = ts.visible[:0]
	q := ""
	if ts.ShowSearch {
		q = strings.ToLower(strings.TrimSpace(ts.SearchValue))
	}
	matchSet := map[string]bool{}
	if q != "" {
		ts.collectMatches(ts.resolvedTree(), q, matchSet)
	}
	var walk func(nodes []TreeSelectNode, depth int, ancestorHidden bool)
	walk = func(nodes []TreeSelectNode, depth int, ancestorHidden bool) {
		for _, n := range nodes {
			show := true
			if q != "" {
				// show if self or any descendant matches, and ancestors of matches
				show = matchSet[n.Value]
			}
			if !show {
				continue
			}
			exp := ts.isExpanded(n.Value)
			// force expand under search so matched descendants appear
			if q != "" && matchSet[n.Value] && len(n.Children) > 0 {
				// keep map state; already expanded by expandForSearch
				exp = ts.isExpanded(n.Value)
			}
			checked, half := ts.checkState(n)
			sel := false
			if ts.isMulti() {
				sel = ts.valueIn(n.Value)
			} else {
				sel = ts.Value == n.Value
			}
			ts.visible = append(ts.visible, treeSelectRow{
				Node:     n,
				Depth:    depth,
				Expanded: exp,
				HasKids:  ts.canExpand(n),
				Checked:  checked,
				Half:     half,
				Selected: sel,
				Match:    q == "" || strings.Contains(strings.ToLower(n.DisplayTitle()), q) || strings.Contains(strings.ToLower(n.Value), q),
			})
			if exp && len(n.Children) > 0 {
				walk(n.Children, depth+1, false)
			}
		}
	}
	walk(ts.resolvedTree(), 0, false)
}

func (ts *TreeSelect) collectMatches(nodes []TreeSelectNode, q string, out map[string]bool) bool {
	any := false
	for _, n := range nodes {
		self := strings.Contains(strings.ToLower(n.DisplayTitle()), q) || strings.Contains(strings.ToLower(n.Value), q)
		child := false
		if len(n.Children) > 0 {
			child = ts.collectMatches(n.Children, q, out)
		}
		if self || child {
			out[n.Value] = true
			any = true
		}
	}
	return any
}

func (ts *TreeSelect) paintTreeRows() {
	if ts.list == nil {
		return
	}
	ts.list.ClearChildren()
	th := ts.theme()
	fontSz := ts.fontSize()
	if len(ts.visible) == 0 {
		msg := "Not Found"
		if strings.TrimSpace(ts.SearchValue) == "" && len(ts.resolvedTree()) == 0 {
			msg = "No Data"
		}
		empty := primitive.NewText(msg)
		empty.FontSize = fontSz
		empty.Face = ts.Face
		empty.Color = th.Color(core.TokenColorTextSecondary)
		pad := primitive.NewDecorated(empty)
		pad.Padding = primitive.Symmetric(DefaultTreeSelectItemPadInline, DefaultTreeSelectItemPadBlock)
		ts.list.AddChild(pad)
		ts.list.MarkNeedsLayout()
		return
	}
	for i, row := range ts.visible {
		ts.list.AddChild(ts.makeRowNode(i, row, th, fontSz))
	}
	ts.list.MarkNeedsLayout()
	ts.list.MarkNeedsPaint()
}

func (ts *TreeSelect) makeRowNode(index int, row treeSelectRow, th *core.Theme, fontSz float64) core.Node {
	// switcher
	var switcher core.Node
	if row.HasKids {
		iconName := "chevron-right"
		if row.Expanded {
			iconName = "chevron-down"
		}
		ic := primitive.NewIcon(iconName)
		ic.Size = 12
		ic.Color = th.Color(core.TokenColorTextSecondary)
		sw := primitive.NewPressable(ic)
		sw.Focusable = false
		sw.ShowFocusRing = false
		sw.Base().Role = "button"
		sw.Base().Label = "expand " + row.Node.DisplayTitle()
		val := row.Node.Value
		expanded := row.Expanded
		sw.Click = func() {
			if expanded {
				ts.CollapseValue(val)
			} else {
				ts.ExpandValue(val)
			}
		}
		// treeLine: paint a small vertical hint via decorated left border
		if ts.TreeLine {
			box := primitive.NewDecorated(sw)
			box.Width = DefaultTreeSelectSwitcherSize
			box.Height = DefaultTreeSelectRowHeight
			box.BorderWidth = 0
			box.SetCenterContent(true)
			switcher = box
		} else {
			switcher = sw
		}
	} else {
		// leaf spacer (treeLine may draw a stub)
		sp := primitive.NewBox(nil)
		sp.Width = DefaultTreeSelectSwitcherSize
		sp.Height = DefaultTreeSelectSwitcherSize
		if ts.TreeLine {
			// leaf stub under treeLine (icon registry has no "dot")
			stub := primitive.NewDecorated(nil)
			stub.Width = 6
			stub.Height = 6
			stub.Radius = 3
			stub.Background = th.Color(core.TokenColorTextQuaternary)
			if stub.Background.A < 0.1 {
				stub.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
			}
			sw := primitive.NewDecorated(stub)
			sw.Width = DefaultTreeSelectSwitcherSize
			sw.Height = DefaultTreeSelectRowHeight
			sw.SetCenterContent(true)
			switcher = sw
		} else {
			switcher = sp
		}
	}

	// checkbox
	var checkNode core.Node
	if ts.TreeCheckable {
		nodeCheckable := true
		if row.Node.CheckableSet {
			nodeCheckable = row.Node.Checkable
		}
		if nodeCheckable && !row.Node.DisableCheckbox {
			cb := NewCheckbox("")
			cb.SetChecked(row.Checked)
			if row.Half {
				cb.SetIndeterminate(true)
			}
			cb.SetDisabled(row.Node.Disabled || ts.Disabled)
			n := row.Node
			cb.SetOnChange(func(checked bool) {
				_ = checked
				ts.toggleCheck(n)
			})
			checkNode = cb.Node()
		}
	}

	// optional tree icon (use chevron as neutral glyph; custom icons are P1)
	var iconNode core.Node
	if ts.TreeIcon {
		ic := primitive.NewIcon("chevron-right")
		ic.Size = 12
		ic.Color = th.Color(core.TokenColorTextSecondary)
		iconNode = ic
	}

	// title
	title := primitive.NewText(row.Node.DisplayTitle())
	title.FontSize = fontSz
	title.Face = ts.Face
	if row.Node.Disabled {
		title.Color = th.Color(core.TokenColorDisabledText)
	} else {
		title.Color = th.Color(core.TokenColorText)
	}

	kids := []core.Node{}
	// indent
	if row.Depth > 0 {
		ind := primitive.NewBox(nil)
		ind.Width = DefaultTreeSelectIndent * float64(row.Depth)
		ind.Height = 1
		kids = append(kids, ind)
	}
	kids = append(kids, switcher)
	if checkNode != nil {
		kids = append(kids, checkNode)
	}
	if iconNode != nil {
		kids = append(kids, iconNode)
	}
	kids = append(kids, title)

	inner := primitive.Row(kids...)
	inner.Gap = 6
	inner.CrossAlign = core.CrossCenter

	body := primitive.NewDecorated(inner)
	body.Padding = primitive.Symmetric(DefaultTreeSelectItemPadInline, DefaultTreeSelectItemPadBlock)
	body.Radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
	body.MinHeight = DefaultTreeSelectRowHeight
	body.Height = DefaultTreeSelectRowHeight
	body.StretchChild = true

	// selection / active chrome
	if row.Selected || (ts.TreeCheckable && row.Checked && !ts.isMulti()) {
		bg := th.Color(core.TokenColorPrimaryBg)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0.9, G: 0.93, B: 1, A: 1}
		}
		body.Background = bg
	} else if index == ts.activeIndex {
		bg := th.Color(core.TokenColorFillSecondary)
		if bg.A < 0.05 {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
		}
		body.Background = bg
	}

	press := primitive.NewPressable(body)
	press.Focusable = false
	press.ShowFocusRing = false
	press.Base().Role = "option"
	press.Base().Label = row.Node.DisplayTitle()
	if row.Node.Disabled {
		press.SetDisabled(true)
	}
	n := row.Node
	idx := index
	press.Click = func() {
		if n.Disabled {
			return
		}
		ts.activeIndex = idx
		if ts.TreeCheckable {
			// click row toggles check
			ts.toggleCheck(n)
			return
		}
		if !n.isSelectable() {
			// still expand if has kids
			if ts.canExpand(n) {
				if ts.isExpanded(n.Value) {
					ts.CollapseValue(n.Value)
				} else {
					ts.ExpandValue(n.Value)
				}
			}
			return
		}
		ts.applySelect(n)
	}
	return press
}

func (ts *TreeSelect) applySelect(n TreeSelectNode) {
	if ts == nil || n.Disabled || !n.isSelectable() {
		return
	}
	if ts.isMulti() && !ts.TreeCheckable {
		// toggle multi without checkboxes
		if ts.valueIn(n.Value) {
			ts.removeValue(n.Value)
			if ts.OnDeselect != nil {
				ts.OnDeselect(n.Value, n)
			}
		} else {
			if !ts.Controlled {
				ts.Values = append(ts.Values, n.Value)
			}
			if ts.OnSelect != nil {
				ts.OnSelect(n.Value, n)
			}
		}
		ts.fireMultiChange()
		ts.refreshDisplay()
		ts.refreshTree(false)
		return
	}
	// single
	if ts.Controlled {
		if ts.OnChange != nil {
			ts.OnChange(n.Value)
		}
	} else {
		ts.Value = n.Value
		if ts.OnChange != nil {
			ts.OnChange(n.Value)
		}
	}
	if ts.OnSelect != nil {
		ts.OnSelect(n.Value, n)
	}
	ts.refreshDisplay()
	// single closes
	if !ts.openControlled {
		ts.applyOpen(false, false)
	} else if ts.OnOpenChange != nil {
		ts.OnOpenChange(false)
	} else {
		ts.refreshTree(false)
	}
}

func (ts *TreeSelect) toggleCheck(n TreeSelectNode) {
	if ts == nil || n.Disabled || n.DisableCheckbox {
		return
	}
	checked, _ := ts.checkState(n)
	want := !checked
	if ts.TreeCheckStrictly {
		if want {
			if !ts.valueIn(n.Value) {
				ts.Values = append(ts.Values, n.Value)
			}
			if ts.OnSelect != nil {
				ts.OnSelect(n.Value, n)
			}
		} else {
			ts.removeValue(n.Value)
			if ts.OnDeselect != nil {
				ts.OnDeselect(n.Value, n)
			}
		}
	} else {
		// cascade to descendants
		var vals []string
		ts.collectDescendantValues(n, &vals)
		if want {
			for _, v := range vals {
				if !ts.valueIn(v) {
					ts.Values = append(ts.Values, v)
				}
			}
			if ts.OnSelect != nil {
				ts.OnSelect(n.Value, n)
			}
		} else {
			for _, v := range vals {
				ts.removeValue(v)
			}
			if ts.OnDeselect != nil {
				ts.OnDeselect(n.Value, n)
			}
		}
		// also update ancestors' implied state by not storing parent unless SHOW_ALL
	}
	ts.fireMultiChange()
	ts.refreshDisplay()
	ts.refreshTree(false)
}

func (ts *TreeSelect) collectDescendantValues(n TreeSelectNode, out *[]string) {
	*out = append(*out, n.Value)
	for _, c := range n.Children {
		ts.collectDescendantValues(c, out)
	}
}

func (ts *TreeSelect) checkState(n TreeSelectNode) (checked, half bool) {
	if ts.TreeCheckStrictly || len(n.Children) == 0 {
		return ts.valueIn(n.Value), false
	}
	// parent state derived from children
	var leaves []string
	ts.collectLeafValues(n, &leaves)
	if len(leaves) == 0 {
		return ts.valueIn(n.Value), false
	}
	on := 0
	for _, v := range leaves {
		if ts.valueIn(v) {
			on++
		}
	}
	if on == 0 {
		return false, false
	}
	if on == len(leaves) {
		return true, false
	}
	return false, true
}

func (ts *TreeSelect) collectLeafValues(n TreeSelectNode, out *[]string) {
	if len(n.Children) == 0 {
		*out = append(*out, n.Value)
		return
	}
	for _, c := range n.Children {
		ts.collectLeafValues(c, out)
	}
}

func (ts *TreeSelect) valueIn(v string) bool {
	for _, x := range ts.Values {
		if x == v {
			return true
		}
	}
	return false
}

func (ts *TreeSelect) removeValue(v string) {
	out := ts.Values[:0]
	for _, x := range ts.Values {
		if x != v {
			out = append(out, x)
		}
	}
	// keep capacity
	if len(out) == 0 {
		ts.Values = nil
	} else {
		ts.Values = append([]string(nil), out...)
	}
}

func (ts *TreeSelect) fireMultiChange() {
	vals := ts.displayValues()
	if ts.OnChangeMulti != nil {
		ts.OnChangeMulti(append([]string(nil), vals...))
	}
	// also fire OnChange with joined for convenience? antd multi uses array only.
}

// displayValues applies showCheckedStrategy for checkable multi display/onChange.
func (ts *TreeSelect) displayValues() []string {
	if !ts.TreeCheckable || ts.TreeCheckStrictly {
		return append([]string(nil), ts.Values...)
	}
	switch ts.ShowCheckedStrategy {
	case TreeSelectSHOW_ALL:
		// include parents that are fully checked
		var out []string
		var walk func([]TreeSelectNode)
		walk = func(nodes []TreeSelectNode) {
			for _, n := range nodes {
				checked, half := ts.checkState(n)
				if checked && !half {
					out = append(out, n.Value)
				} else if half || checked {
					// partial: still walk
				}
				if len(n.Children) > 0 {
					walk(n.Children)
				} else if ts.valueIn(n.Value) {
					// leaf already added if checked
					if !checked {
						// nothing
					}
				}
			}
		}
		// simpler: all values in Values + fully checked parents
		seen := map[string]bool{}
		for _, v := range ts.Values {
			if !seen[v] {
				out = append(out, v)
				seen[v] = true
			}
		}
		var addParents func([]TreeSelectNode)
		addParents = func(nodes []TreeSelectNode) {
			for _, n := range nodes {
				if len(n.Children) > 0 {
					checked, half := ts.checkState(n)
					if checked && !half && !seen[n.Value] {
						out = append(out, n.Value)
						seen[n.Value] = true
					}
					addParents(n.Children)
				}
			}
		}
		addParents(ts.resolvedTree())
		return out
	case TreeSelectSHOW_PARENT:
		var out []string
		var walk func([]TreeSelectNode)
		walk = func(nodes []TreeSelectNode) {
			for _, n := range nodes {
				checked, half := ts.checkState(n)
				if checked && !half {
					out = append(out, n.Value)
					continue // skip children
				}
				if half || len(n.Children) > 0 {
					walk(n.Children)
				} else if ts.valueIn(n.Value) {
					out = append(out, n.Value)
				}
			}
		}
		walk(ts.resolvedTree())
		return out
	default: // SHOW_CHILD
		var out []string
		var walk func([]TreeSelectNode)
		walk = func(nodes []TreeSelectNode) {
			for _, n := range nodes {
				if len(n.Children) == 0 {
					if ts.valueIn(n.Value) {
						out = append(out, n.Value)
					}
				} else {
					walk(n.Children)
				}
			}
		}
		walk(ts.resolvedTree())
		// if a checked node is leaf-less non-parent stored value, include
		if len(out) == 0 {
			return append([]string(nil), ts.Values...)
		}
		return out
	}
}

func (ts *TreeSelect) maybeLoadData(n TreeSelectNode) {
	if ts.LoadData == nil {
		return
	}
	if n.IsLeaf || len(n.Children) > 0 || n.Loading {
		return
	}
	// mark loading on matching node in TreeData
	ts.setNodeLoading(n.Value, true)
	ts.syncLoadingTicker()
	ts.refreshTree(false)
	ts.LoadData(n)
}

func (ts *TreeSelect) setNodeLoading(value string, loading bool) {
	var walk func(nodes []TreeSelectNode) bool
	walk = func(nodes []TreeSelectNode) bool {
		for i := range nodes {
			if nodes[i].Value == value {
				nodes[i].Loading = loading
				return true
			}
			if walk(nodes[i].Children) {
				return true
			}
		}
		return false
	}
	if walk(ts.TreeData) {
		ts.resolved = nil
	}
}

func (ts *TreeSelect) anyNodeLoading() bool {
	var walk func([]TreeSelectNode) bool
	walk = func(nodes []TreeSelectNode) bool {
		for _, n := range nodes {
			if n.Loading {
				return true
			}
			if walk(n.Children) {
				return true
			}
		}
		return false
	}
	return walk(ts.resolvedTree())
}

func (ts *TreeSelect) syncLoadingTicker() {
	if ts.boundTree == nil {
		return
	}
	ts.boundTree.BindTicker(ts, ts.Loading || ts.anyNodeLoading())
}

func (ts *TreeSelect) findNode(value string) (TreeSelectNode, bool) {
	var walk func([]TreeSelectNode) (TreeSelectNode, bool)
	walk = func(nodes []TreeSelectNode) (TreeSelectNode, bool) {
		for _, n := range nodes {
			if n.Value == value {
				return n, true
			}
			if c, ok := walk(n.Children); ok {
				return c, true
			}
		}
		return TreeSelectNode{}, false
	}
	return walk(ts.resolvedTree())
}

func (ts *TreeSelect) paintSpinner(pc *core.PaintContext, size core.Size) {
	th := ts.theme()
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
	start := -math.Pi/2 + ts.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

func mapTreeSelectPlacement(p TreeSelectPlacement) primitive.Placement {
	switch p {
	case TreeSelectBottomRight:
		return primitive.PlaceBottomEnd
	case TreeSelectTopLeft:
		return primitive.PlaceTopStart
	case TreeSelectTopRight:
		return primitive.PlaceTopEnd
	default:
		return primitive.PlaceBottomStart
	}
}
