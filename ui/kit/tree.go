package kit

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Tree defaults — components/tree/style prepareComponentToken.
// docs/antd/tree.md §6.2 / §6.10
// https://ant.design/components/tree
const (
	DefaultTreeTitleHeight     = 24.0 // controlHeightSM
	DefaultTreeSwitcherSize    = 24.0
	DefaultTreeIndentSize      = 24.0
	DefaultTreeNodePadBlock    = 4.0 // paddingXS / 2
	DefaultTreeNodePadInline   = 4.0
	DefaultTreeFocusRingOutset = 1.5
	DefaultTreeLeafIconSize    = 14.0
	DefaultTreeFontSize        = 14.0
	DefaultTreeLineWidth       = 1.0
	DefaultTreeRadius          = 6.0
	DefaultTreeRowGap          = 0.0
)

// TreeNode is one antd treeData entry (TreeDataNode subset).
// Key must be unique across the tree.
type TreeNode struct {
	Key             string
	Title           string
	Icon            string // icon name when ShowIcon
	SwitcherIcon    string // per-node switcher override
	Children        []TreeNode
	Disabled        bool
	DisableCheckbox bool
	IsLeaf          bool
	Loading         bool  // loadData in flight
	Selectable      *bool // nil → true
}

// DisplayTitle returns Title or falls back to Key.
func (n TreeNode) DisplayTitle() string {
	if n.Title != "" {
		return n.Title
	}
	return n.Key
}

// isSelectable reports whether the node accepts selection.
func (n TreeNode) isSelectable() bool {
	if n.Selectable != nil {
		return *n.Selectable
	}
	return true
}

// TreeDropInfo is the drop payload (antd onDrop info subset).
type TreeDropInfo struct {
	DragKey      string
	DropKey      string
	DropToGap    bool
	DropPosition int // -1 before, 0 inside, 1 after
}

// Tree is Ant Design Tree — hierarchical list with expand / select / check.
//
//	Decorated Root (role=tree)
//	  └─ Flex Column
//	       row × N (role=treeitem)
//	         indent · switcher · checkbox? · icon? · title
//
// Product contract: docs/antd/tree.md §6 (P0 DoD).
type Tree struct {
	Root *primitive.Decorated
	col  *primitive.Flex

	TreeData []TreeNode

	// Selection
	SelectedKeys        []string
	DefaultSelectedKeys []string
	Multiple            bool
	ControlledSelected  bool

	// Expand
	expanded            map[string]bool
	DefaultExpandedKeys []string
	DefaultExpandAll    bool
	DefaultExpandParent bool // default true
	AutoExpandParent    bool
	ControlledExpanded  bool

	// Check
	Checkable          bool
	CheckedKeys        []string
	halfChecked        map[string]bool
	DefaultCheckedKeys []string
	CheckStrictly      bool
	ControlledChecked  bool

	// Display
	Disabled     bool
	ShowLine     bool
	ShowLeafIcon bool // when ShowLine; default true
	ShowIcon     bool
	BlockNode    bool
	Directory    bool
	SwitcherIcon string // global switcher icon name
	SearchValue  string

	// Async
	LoadData   func(node TreeNode)
	LoadedKeys []string
	loadedSet  map[string]bool
	OnLoad     func(loadedKeys []string, node TreeNode)

	// Drag
	Draggable   bool
	dragKey     string
	dropHover   string
	OnDrop      func(info TreeDropInfo)
	OnDragStart func(key string)
	OnDragEnter func(key string)

	// Callbacks
	OnExpand func(expandedKeys []string, node TreeNode, expanded bool)
	OnSelect func(selectedKeys []string, node TreeNode, selected bool)
	OnCheck  func(checkedKeys []string, node TreeNode, checked bool)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// defaults applied once
	appliedDefaults bool
	showLeafIconSet bool

	// focus / keyboard
	focusKey string

	// loading spinner
	spinPhase float64
	boundTree *core.Tree
	spinners  []*primitive.Canvas

	// row index for tests
	visible []treeRow
}

type treeRow struct {
	Key      string
	Title    string
	Depth    int
	Node     TreeNode
	Expanded bool
	HasKids  bool
	IsLast   bool
	// ancestorIsLast[i] true if ancestor at depth i is last among siblings
	LineMask []bool
}

// NewTree creates a Tree with optional treeData roots.
// Defaults (§6.10): no check/select/expand seed, DefaultExpandParent=true.
func NewTree(treeData ...TreeNode) *Tree {
	tr := &Tree{
		TreeData:            append([]TreeNode(nil), treeData...),
		DefaultExpandParent: true,
		ShowLeafIcon:        true,
		expanded:            make(map[string]bool),
		halfChecked:         make(map[string]bool),
		loadedSet:           make(map[string]bool),
	}
	tr.rebuild()
	return tr
}

// NewDirectoryTree creates a DirectoryTree (directory + blockNode).
func NewDirectoryTree(treeData ...TreeNode) *Tree {
	tr := NewTree(treeData...)
	tr.Directory = true
	tr.BlockNode = true
	tr.rebuild()
	return tr
}

// Node returns the stable root.

// ensureBuilt materializes the control tree if missing (#9).
func (tr *Tree) ensureBuilt() {
	if tr == nil {
		return
	}
	if tr.Root == nil {
		tr.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (tr *Tree) structureChange() {
	if tr == nil {
		return
	}
	tr.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (tr *Tree) chromeChange() {
	if tr == nil {
		return
	}
	tr.ensureBuilt()
	tr.rebuild()
}

func (tr *Tree) Node() core.Node {
	if tr == nil {
		return nil
	}
	tr.ensureBuilt()
	return tr.Root
}

// ChromeNode returns the root Decorated chrome (tests / layout).
func (tr *Tree) ChromeNode() core.Node {
	if tr == nil {
		return nil
	}
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.Root
}

// ---------------------------------------------------------------------------
// Metrics (L2)
// ---------------------------------------------------------------------------

// TitleHeight returns resolved title row height (§6.2).
func (tr *Tree) TitleHeight() float64 {
	th := tr.theme()
	return th.SizeOr(core.TokenControlHeightSM, DefaultTreeTitleHeight)
}

// IndentSize returns resolved indent width (§6.2).
func (tr *Tree) IndentSize() float64 {
	return tr.TitleHeight() // antd indentSize = titleHeight
}

// SwitcherSize returns switcher box size (§6.2).
func (tr *Tree) SwitcherSize() float64 {
	return tr.TitleHeight()
}

// FontSize returns resolved title font size.
func (tr *Tree) FontSize() float64 {
	th := tr.theme()
	return th.SizeOr(core.TokenFontSize, DefaultTreeFontSize)
}

// ---------------------------------------------------------------------------
// Data
// ---------------------------------------------------------------------------

// SetTreeData replaces treeData and rebuilds.
func (tr *Tree) SetTreeData(nodes ...TreeNode) {
	if tr == nil {
		return
	}
	tr.TreeData = append([]TreeNode(nil), nodes...)
	if tr.DefaultExpandAll {
		tr.expandAllInternal()
	}
	tr.rebuild()
}

// TreeDataCopy returns a shallow copy of roots (children shared).
func (tr *Tree) TreeDataCopy() []TreeNode {
	if tr == nil {
		return nil
	}
	return append([]TreeNode(nil), tr.TreeData...)
}

// NotifyTreeDataChanged rebuilds after loadData mutates TreeData.
func (tr *Tree) NotifyTreeDataChanged() {
	if tr == nil {
		return
	}
	tr.rebuild()
	tr.syncTicker()
}

// ---------------------------------------------------------------------------
// Expand
// ---------------------------------------------------------------------------

// SetExpandedKeys sets expanded keys. When ControlledExpanded, this is the
// only path that mutates expansion (clicks only fire OnExpand).
func (tr *Tree) SetExpandedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.expanded = make(map[string]bool, len(keys))
	for _, k := range keys {
		tr.expanded[k] = true
	}
	tr.rebuild()
}

// SetDefaultExpandedKeys seeds non-controlled expansion (once via applyDefaults).
func (tr *Tree) SetDefaultExpandedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.DefaultExpandedKeys = append([]string(nil), keys...)
	if !tr.ControlledExpanded {
		if tr.expanded == nil {
			tr.expanded = make(map[string]bool)
		}
		for _, k := range keys {
			tr.expanded[k] = true
		}
		tr.rebuild()
	}
}

// SetDefaultExpandAll expands every non-leaf when defaults apply.
func (tr *Tree) SetDefaultExpandAll(v bool) {
	if tr == nil {
		return
	}
	tr.DefaultExpandAll = v
	if v && !tr.ControlledExpanded {
		tr.expandAllInternal()
		tr.rebuild()
	}
}

// SetDefaultExpandParent toggles defaultExpandParent (antd default true).
func (tr *Tree) SetDefaultExpandParent(v bool) {
	if tr == nil {
		return
	}
	tr.DefaultExpandParent = v
}

// SetAutoExpandParent toggles autoExpandParent (search composition).
func (tr *Tree) SetAutoExpandParent(v bool) {
	if tr == nil {
		return
	}
	tr.AutoExpandParent = v
	if v {
		tr.expandParentsOfSelectedAndChecked()
		tr.rebuild()
	}
}

// SetControlledExpanded marks expandedKeys as controlled.
func (tr *Tree) SetControlledExpanded(v bool) {
	if tr == nil {
		return
	}
	tr.ControlledExpanded = v
}

// ExpandedKeys returns currently expanded keys (stable order = DFS).
func (tr *Tree) ExpandedKeys() []string {
	if tr == nil {
		return nil
	}
	var out []string
	var walk func([]TreeNode)
	walk = func(nodes []TreeNode) {
		for _, n := range nodes {
			if tr.expanded[n.Key] {
				out = append(out, n.Key)
			}
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(tr.TreeData)
	return out
}

// ToggleExpand toggles one key (tests / API). Respects ControlledExpanded.
func (tr *Tree) ToggleExpand(key string) {
	if tr == nil || tr.Disabled {
		return
	}
	n, ok := tr.findNode(key)
	if !ok {
		return
	}
	cur := tr.isExpanded(key)
	tr.applyExpand(n, !cur)
}

// ExpandKey expands a node by key.
func (tr *Tree) ExpandKey(key string) {
	if tr == nil {
		return
	}
	n, ok := tr.findNode(key)
	if !ok {
		return
	}
	tr.applyExpand(n, true)
}

// CollapseKey collapses a node by key.
func (tr *Tree) CollapseKey(key string) {
	if tr == nil {
		return
	}
	n, ok := tr.findNode(key)
	if !ok {
		return
	}
	tr.applyExpand(n, false)
}

// ---------------------------------------------------------------------------
// Select
// ---------------------------------------------------------------------------

// SetSelectedKeys sets selection. Does not fire OnSelect.
func (tr *Tree) SetSelectedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.SelectedKeys = append([]string(nil), keys...)
	if len(keys) > 0 {
		tr.focusKey = keys[0]
	}
	tr.rebuild()
}

// SetDefaultSelectedKeys seeds non-controlled selection.
func (tr *Tree) SetDefaultSelectedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.DefaultSelectedKeys = append([]string(nil), keys...)
	// Apply when still unselected (NewTree already ran applyDefaultsOnce).
	if !tr.ControlledSelected && len(tr.SelectedKeys) == 0 {
		tr.SelectedKeys = append([]string(nil), keys...)
		if tr.DefaultExpandParent {
			tr.expandParentsOfSelectedAndChecked()
		}
		tr.rebuild()
	}
}

// SetMultiple enables multi-select.
func (tr *Tree) SetMultiple(v bool) {
	if tr == nil {
		return
	}
	tr.Multiple = v
	if !v && len(tr.SelectedKeys) > 1 {
		tr.SelectedKeys = tr.SelectedKeys[:1]
		tr.rebuild()
	}
}

// SetControlledSelected marks selectedKeys as controlled.
func (tr *Tree) SetControlledSelected(v bool) {
	if tr == nil {
		return
	}
	tr.ControlledSelected = v
}

// GetSelectedKeys returns a copy of selected keys.
func (tr *Tree) GetSelectedKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.SelectedKeys...)
}

// SelectKey programmatically selects a key (fires OnSelect when uncontrolled path).
func (tr *Tree) SelectKey(key string) {
	if tr == nil || tr.Disabled {
		return
	}
	n, ok := tr.findNode(key)
	if !ok || n.Disabled || !n.isSelectable() {
		return
	}
	tr.applySelect(n)
}

// ---------------------------------------------------------------------------
// Check
// ---------------------------------------------------------------------------

// SetCheckable toggles checkboxes before titles.
func (tr *Tree) SetCheckable(v bool) {
	if tr == nil {
		return
	}
	tr.Checkable = v
	tr.rebuild()
}

// SetCheckedKeys sets checked keys. Does not fire OnCheck.
func (tr *Tree) SetCheckedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.CheckedKeys = append([]string(nil), keys...)
	tr.recomputeHalf()
	tr.rebuild()
}

// SetDefaultCheckedKeys seeds non-controlled checks.
func (tr *Tree) SetDefaultCheckedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.DefaultCheckedKeys = append([]string(nil), keys...)
	if !tr.ControlledChecked && len(tr.CheckedKeys) == 0 {
		tr.CheckedKeys = append([]string(nil), keys...)
		tr.recomputeHalf()
		if tr.DefaultExpandParent {
			tr.expandParentsOfSelectedAndChecked()
		}
		tr.rebuild()
	}
}

// SetCheckStrictly disables parent-child check linkage.
func (tr *Tree) SetCheckStrictly(v bool) {
	if tr == nil {
		return
	}
	tr.CheckStrictly = v
	tr.recomputeHalf()
	tr.rebuild()
}

// SetControlledChecked marks checkedKeys as controlled.
func (tr *Tree) SetControlledChecked(v bool) {
	if tr == nil {
		return
	}
	tr.ControlledChecked = v
}

// GetCheckedKeys returns a copy of checked keys.
func (tr *Tree) GetCheckedKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.CheckedKeys...)
}

// HalfCheckedKeys returns keys in indeterminate state (non-strict).
func (tr *Tree) HalfCheckedKeys() []string {
	if tr == nil {
		return nil
	}
	var out []string
	for k, v := range tr.halfChecked {
		if v {
			out = append(out, k)
		}
	}
	return out
}

// ToggleCheck toggles a checkable node (tests).
func (tr *Tree) ToggleCheck(key string) {
	if tr == nil || tr.Disabled || !tr.Checkable {
		return
	}
	n, ok := tr.findNode(key)
	if !ok || n.Disabled || n.DisableCheckbox {
		return
	}
	tr.applyCheck(n)
}

// ---------------------------------------------------------------------------
// Display / config
// ---------------------------------------------------------------------------

// SetDisabled disables the whole tree.
func (tr *Tree) SetDisabled(v bool) {
	if tr == nil {
		return
	}
	tr.Disabled = v
	tr.rebuild()
}

// SetShowLine toggles connector lines.
func (tr *Tree) SetShowLine(v bool) {
	if tr == nil {
		return
	}
	tr.ShowLine = v
	tr.rebuild()
}

// SetShowLeafIcon toggles leaf icons under showLine.
func (tr *Tree) SetShowLeafIcon(v bool) {
	if tr == nil {
		return
	}
	tr.ShowLeafIcon = v
	tr.showLeafIconSet = true
	tr.rebuild()
}

// SetShowIcon toggles treeData icon before title.
func (tr *Tree) SetShowIcon(v bool) {
	if tr == nil {
		return
	}
	tr.ShowIcon = v
	tr.rebuild()
}

// SetBlockNode makes rows fill the row width.
func (tr *Tree) SetBlockNode(v bool) {
	if tr == nil {
		return
	}
	tr.BlockNode = v
	tr.rebuild()
}

// SetDirectory enables DirectoryTree chrome (selected primary fill).
func (tr *Tree) SetDirectory(v bool) {
	if tr == nil {
		return
	}
	tr.Directory = v
	if v {
		tr.BlockNode = true
	}
	tr.rebuild()
}

// SetSwitcherIcon sets the global switcher icon name (empty → chevron).
func (tr *Tree) SetSwitcherIcon(name string) {
	if tr == nil {
		return
	}
	tr.SwitcherIcon = name
	tr.rebuild()
}

// SetSearchValue sets the highlight query (external Search composition).
func (tr *Tree) SetSearchValue(q string) {
	if tr == nil {
		return
	}
	tr.SearchValue = q
	if tr.AutoExpandParent && q != "" {
		tr.expandParentsMatching(q)
	}
	tr.rebuild()
}

// SetLoadData sets the async loadData callback.
func (tr *Tree) SetLoadData(fn func(node TreeNode)) {
	if tr == nil {
		return
	}
	tr.LoadData = fn
}

// SetLoadedKeys sets already-loaded keys.
func (tr *Tree) SetLoadedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.LoadedKeys = append([]string(nil), keys...)
	tr.loadedSet = make(map[string]bool, len(keys))
	for _, k := range keys {
		tr.loadedSet[k] = true
	}
}

// GetLoadedKeys returns loaded keys.
func (tr *Tree) GetLoadedKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.LoadedKeys...)
}

// SetDraggable enables node drag-and-drop.
func (tr *Tree) SetDraggable(v bool) {
	if tr == nil {
		return
	}
	tr.Draggable = v
	tr.rebuild()
}

// SetTheme sets an explicit theme.
func (tr *Tree) SetTheme(th *core.Theme) {
	if tr == nil {
		return
	}
	tr.Theme = th
	tr.rebuild()
}

// SetFace sets the font face.
func (tr *Tree) SetFace(face text.Face) {
	if tr == nil {
		return
	}
	tr.Face = face
	tr.rebuild()
}

// SetStyle sets optional visual overrides.
func (tr *Tree) SetStyle(st Style) {
	if tr == nil {
		return
	}
	tr.Style = st
	tr.rebuild()
}

// SetAriaLabel sets the accessible name on the root tree.
func (tr *Tree) SetAriaLabel(name string) {
	if tr == nil {
		return
	}
	tr.AriaLabel = name
	if tr.Root != nil {
		tr.Root.Base().Label = name
	}
}

// SetOnExpand / SetOnSelect / SetOnCheck / SetOnDrop / SetOnLoad wire callbacks.
func (tr *Tree) SetOnExpand(fn func(expandedKeys []string, node TreeNode, expanded bool)) {
	if tr != nil {
		tr.OnExpand = fn
	}
}
func (tr *Tree) SetOnSelect(fn func(selectedKeys []string, node TreeNode, selected bool)) {
	if tr != nil {
		tr.OnSelect = fn
	}
}
func (tr *Tree) SetOnCheck(fn func(checkedKeys []string, node TreeNode, checked bool)) {
	if tr != nil {
		tr.OnCheck = fn
	}
}
func (tr *Tree) SetOnDrop(fn func(info TreeDropInfo)) {
	if tr != nil {
		tr.OnDrop = fn
	}
}
func (tr *Tree) SetOnLoad(fn func(loadedKeys []string, node TreeNode)) {
	if tr != nil {
		tr.OnLoad = fn
	}
}

// ---------------------------------------------------------------------------
// Drag API (tests / program)
// ---------------------------------------------------------------------------

// Drop performs a drop (antd onDrop semantics simplified).
// When OnDrop is nil and not controlled, mutates TreeData in place.
func (tr *Tree) Drop(dragKey, dropKey string, dropToGap bool, dropPosition int) {
	if tr == nil || tr.Disabled || !tr.Draggable {
		return
	}
	if dragKey == "" || dropKey == "" || dragKey == dropKey {
		return
	}
	info := TreeDropInfo{
		DragKey:      dragKey,
		DropKey:      dropKey,
		DropToGap:    dropToGap,
		DropPosition: dropPosition,
	}
	if tr.OnDrop != nil {
		tr.OnDrop(info)
		return
	}
	tr.applyDrop(info)
	tr.rebuild()
}

// ---------------------------------------------------------------------------
// Visible / a11y helpers (tests)
// ---------------------------------------------------------------------------

// VisibleKeys returns keys of currently painted rows.
func (tr *Tree) VisibleKeys() []string {
	if tr == nil {
		return nil
	}
	out := make([]string, len(tr.visible))
	for i, r := range tr.visible {
		out[i] = r.Key
	}
	return out
}

// VisibleTitles returns titles of currently painted rows.
func (tr *Tree) VisibleTitles() []string {
	if tr == nil {
		return nil
	}
	out := make([]string, len(tr.visible))
	for i, r := range tr.visible {
		out[i] = r.Title
	}
	return out
}

// IsExpanded reports expand state for a key.
func (tr *Tree) IsExpanded(key string) bool {
	if tr == nil {
		return false
	}
	return tr.isExpanded(key)
}

// IsSelected reports selection state.
func (tr *Tree) IsSelected(key string) bool {
	if tr == nil {
		return false
	}
	return tr.keyIn(tr.SelectedKeys, key)
}

// IsChecked reports full-checked state.
func (tr *Tree) IsChecked(key string) bool {
	if tr == nil {
		return false
	}
	checked, _ := tr.checkStateKey(key)
	return checked
}

// IsHalfChecked reports indeterminate state.
func (tr *Tree) IsHalfChecked(key string) bool {
	if tr == nil {
		return false
	}
	return tr.halfChecked[key]
}

// ShowLineEnabled reports showLine.
func (tr *Tree) ShowLineEnabled() bool { return tr != nil && tr.ShowLine }

// ---------------------------------------------------------------------------
// Ticker (loadData loading)
// ---------------------------------------------------------------------------

// AttachTicker registers loading spinner animation.
func (tr *Tree) AttachTicker(t *core.Tree) {
	if tr == nil || t == nil {
		return
	}
	tr.boundTree = t
	t.BindTicker(tr, tr.anyNodeLoading())
}

// Tick advances loading spinners. Implements core.Ticker.
func (tr *Tree) Tick(dt float64) bool {
	if tr == nil {
		return false
	}
	if !tr.anyNodeLoading() {
		return false
	}
	tr.spinPhase += dt * 1.4
	if tr.spinPhase > 1 {
		tr.spinPhase -= 1
	}
	for _, s := range tr.spinners {
		if s != nil {
			s.MarkNeedsPaint()
		}
	}
	if tr.Root != nil {
		tr.Root.MarkNeedsPaint()
	}
	return true
}

func (tr *Tree) syncTicker() {
	if tr.boundTree != nil {
		tr.boundTree.BindTicker(tr, tr.anyNodeLoading())
	}
}

func (tr *Tree) anyNodeLoading() bool {
	var walk func([]TreeNode) bool
	walk = func(nodes []TreeNode) bool {
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
	return walk(tr.TreeData)
}

// ---------------------------------------------------------------------------
// rebuild
// ---------------------------------------------------------------------------

func (tr *Tree) theme() *core.Theme {
	var n core.Node
	if tr.Root != nil {
		n = tr.Root
	}
	return themeOf(tr.Theme, n)
}

func (tr *Tree) applyDefaultsOnce() {
	if tr.appliedDefaults {
		return
	}
	tr.appliedDefaults = true
	if !tr.ControlledExpanded {
		if tr.DefaultExpandAll {
			tr.expandAllInternal()
		} else {
			for _, k := range tr.DefaultExpandedKeys {
				tr.expanded[k] = true
			}
		}
		if tr.DefaultExpandParent {
			tr.expandParentsOfSelectedAndChecked()
		}
	}
	if !tr.ControlledSelected && len(tr.SelectedKeys) == 0 && len(tr.DefaultSelectedKeys) > 0 {
		tr.SelectedKeys = append([]string(nil), tr.DefaultSelectedKeys...)
	}
	if !tr.ControlledChecked && len(tr.CheckedKeys) == 0 && len(tr.DefaultCheckedKeys) > 0 {
		tr.CheckedKeys = append([]string(nil), tr.DefaultCheckedKeys...)
	}
	tr.recomputeHalf()
}

func (tr *Tree) rebuild() {
	if tr == nil {
		return
	}
	tr.applyDefaultsOnce()
	th := tr.theme()
	if tr.expanded == nil {
		tr.expanded = make(map[string]bool)
	}
	if tr.halfChecked == nil {
		tr.halfChecked = make(map[string]bool)
	}
	if tr.loadedSet == nil {
		tr.loadedSet = make(map[string]bool)
	}

	tr.spinners = nil
	tr.visible = nil
	tr.col = primitive.Column()
	tr.col.Gap = DefaultTreeRowGap
	tr.col.CrossAlign = core.CrossStretch
	tr.col.MainAlign = core.MainStart

	tr.buildVisible()
	for i := range tr.visible {
		tr.paintRow(i, th)
	}

	if tr.Root == nil {
		tr.Root = primitive.NewDecorated(tr.col)
		tr.Root.SkinType = TypeTree
	} else {
		tr.Root.ClearChildren()
		tr.Root.AddChild(tr.col)
	}
	tr.Root.Padding = primitive.All(0)
	tr.Root.BorderWidth = 0
	tr.Root.Background = render.RGBA{}
	if tr.Style.Background.A > 0 {
		tr.Root.Background = tr.Style.Background
	}
	tr.Root.Radius = th.SizeOr(core.TokenBorderRadius, DefaultTreeRadius)
	tr.Root.Base().Role = "tree"
	if tr.AriaLabel != "" {
		tr.Root.Base().Label = tr.AriaLabel
	} else {
		tr.Root.Base().Label = "Tree"
	}
	tr.Root.MarkNeedsLayout()
	tr.Root.MarkNeedsPaint()
	tr.syncTicker()
}

func (tr *Tree) buildVisible() {
	tr.visible = tr.visible[:0]
	var walk func(nodes []TreeNode, depth int, ancestors []bool)
	walk = func(nodes []TreeNode, depth int, ancestors []bool) {
		for i, n := range nodes {
			isLast := i == len(nodes)-1
			hasKids := len(n.Children) > 0 && !n.IsLeaf
			// loadData candidate counts as expandable even without children
			if !n.IsLeaf && tr.LoadData != nil && len(n.Children) == 0 {
				hasKids = true
			}
			exp := tr.isExpanded(n.Key)
			mask := append([]bool(nil), ancestors...)
			tr.visible = append(tr.visible, treeRow{
				Key:      n.Key,
				Title:    n.DisplayTitle(),
				Depth:    depth,
				Node:     n,
				Expanded: exp,
				HasKids:  hasKids,
				IsLast:   isLast,
				LineMask: mask,
			})
			if hasKids && exp && len(n.Children) > 0 {
				nextAnc := append(append([]bool(nil), ancestors...), isLast)
				walk(n.Children, depth+1, nextAnc)
			}
		}
	}
	walk(tr.TreeData, 0, nil)
}

func (tr *Tree) paintRow(idx int, th *core.Theme) {
	row := tr.visible[idx]
	n := row.Node
	titleH := tr.TitleHeight()
	indent := tr.IndentSize()
	swSize := tr.SwitcherSize()
	fontSz := tr.FontSize()
	padBlock := DefaultTreeNodePadBlock
	padInline := DefaultTreeNodePadInline

	// indent + optional showLine rails
	var indentNode core.Node
	if row.Depth > 0 {
		w := float64(row.Depth) * indent
		if tr.ShowLine {
			indentNode = primitive.NewCanvas(w, titleH, tr.linePainter(row, indent, titleH, th))
		} else {
			indentBox := primitive.NewBox()
			indentBox.Width = w
			indentBox.Height = titleH
			indentNode = indentBox
		}
	}

	// switcher
	var switcher core.Node
	if row.HasKids {
		if n.Loading {
			sp := primitive.NewCanvas(swSize, swSize, tr.spinnerPainter())
			tr.spinners = append(tr.spinners, sp)
			switcher = sp
		} else {
			name := tr.switcherIconName(n, row.Expanded)
			ic := primitive.NewIcon(name)
			ic.Size = 12
			ic.Color = th.Color(core.TokenColorTextSecondary)
			sw := primitive.NewPressable(ic)
			sw.ShowFocusRing = false
			sw.Focusable = false
			sw.Base().Role = "button"
			sw.Base().Label = "expand " + n.Key
			if tr.Disabled || n.Disabled {
				sw.SetDisabled(true)
			} else {
				key := n.Key
				sw.Click = func() { tr.ToggleExpand(key) }
			}
			box := primitive.NewDecorated(sw)
			box.Width, box.Height = swSize, titleH
			box.BorderWidth = 0
			box.SetCenterContent(true)
			switcher = box
		}
	} else if tr.ShowLine && tr.ShowLeafIcon {
		leaf := primitive.NewCanvas(swSize, swSize, func(pc *core.PaintContext, size core.Size) {
			col := th.Color(core.TokenColorBorder)
			cx, cy := size.Width/2, size.Height/2
			r := DefaultTreeLeafIconSize / 2
			pc.StrokeLocalCircle(cx, cy, r, DefaultTreeLineWidth, col)
		})
		switcher = leaf
	} else {
		sp := primitive.NewBox()
		sp.Width, sp.Height = swSize, swSize
		switcher = sp
	}

	// checkbox
	var kids []core.Node
	if indentNode != nil {
		kids = append(kids, indentNode)
	}
	kids = append(kids, switcher)

	if tr.Checkable {
		cb := NewCheckbox("")
		cb.SetTheme(th)
		cb.SetFace(tr.Face)
		checked, half := tr.checkState(n)
		cb.SetControlled(true)
		cb.SetChecked(checked)
		cb.SetIndeterminate(half)
		cb.SetDisabled(tr.Disabled || n.Disabled || n.DisableCheckbox)
		key := n.Key
		cb.OnChange = func(bool) {
			if nn, ok := tr.findNode(key); ok {
				tr.applyCheck(nn)
			}
		}
		kids = append(kids, cb.Node())
	}

	// node icon
	if tr.ShowIcon && n.Icon != "" {
		ic := primitive.NewIcon(n.Icon)
		ic.Size = 14
		ic.Color = th.Color(core.TokenColorText)
		if tr.isSelected(n.Key) && tr.Directory {
			ic.Color = th.Color(core.TokenColorTextInverse)
			if ic.Color.A < 0.1 {
				ic.Color = render.RGBA{R: 255, G: 255, B: 255, A: 1}
			}
		}
		kids = append(kids, ic)
	}

	// title (with optional search highlight)
	titleNode := tr.buildTitle(n, fontSz, th)
	kids = append(kids, titleNode)

	rowFlex := primitive.Row(kids...)
	rowFlex.Gap = 4
	rowFlex.CrossAlign = core.CrossCenter
	rowFlex.MainAlign = core.MainStart

	press := primitive.NewPressable(rowFlex)
	press.Padding = primitive.Symmetric(padInline, padBlock)
	press.ShowFocusRing = true
	press.FocusRingOutset = DefaultTreeFocusRingOutset
	press.Focusable = !tr.Disabled && !n.Disabled
	press.Base().Role = "treeitem"
	press.Base().Label = n.DisplayTitle()

	// chrome
	selected := tr.isSelected(n.Key)
	disabled := tr.Disabled || n.Disabled
	if disabled {
		press.SetDisabled(true)
	} else {
		press.ColorHovered = antItemHoverFill(th)
	}
	if selected {
		if tr.Directory {
			press.Color = th.Color(core.TokenColorPrimary)
			if press.Color.A < 0.1 {
				press.Color = render.Hex("#1677FF")
			}
		} else {
			press.Color = antItemSelectedFill(th)
		}
	}
	if tr.dropHover == n.Key && tr.Draggable {
		press.Color = antItemHoverFill(th)
	}

	key := n.Key
	nodeCopy := n
	hasKids := row.HasKids
	if !disabled {
		press.Click = func() {
			// title click → select; directory also expands
			if nodeCopy.isSelectable() {
				tr.applySelect(nodeCopy)
			}
			if tr.Directory && hasKids {
				tr.applyExpand(nodeCopy, !tr.isExpanded(key))
			}
		}
	}

	// drag: if Draggable, wrap with Draggable for start + drop target
	var final core.Node = press
	if tr.Draggable && !disabled {
		d := primitive.NewDraggable(press)
		d.OnDragStart = func() {
			tr.dragKey = key
			if tr.OnDragStart != nil {
				tr.OnDragStart(key)
			}
		}
		d.OnDragEnd = func(dx, dy float64) {
			if tr.dropHover != "" && tr.dragKey != "" && tr.dropHover != tr.dragKey {
				tr.Drop(tr.dragKey, tr.dropHover, false, 0)
			}
			tr.dragKey = ""
			tr.dropHover = ""
			_ = dx
			_ = dy
		}
		press.OnStateChange = func() {
			if tr.dragKey != "" && tr.dragKey != key && press.State.Hovered {
				tr.dropHover = key
				if tr.OnDragEnter != nil {
					tr.OnDragEnter(key)
				}
			}
		}
		final = d
	}

	// ensure min row height via decorated shell when needed
	if tr.BlockNode {
		shell := primitive.NewDecorated(final)
		shell.MinHeight = titleH
		shell.ExpandWidth = true
		shell.BorderWidth = 0
		shell.Background = render.RGBA{}
		final = shell
	}

	tr.col.AddChild(final)
	_ = titleH
}

func (tr *Tree) buildTitle(n TreeNode, fontSz float64, th *core.Theme) core.Node {
	title := n.DisplayTitle()
	q := tr.SearchValue
	textCol := th.Color(core.TokenColorText)
	if tr.Disabled || n.Disabled {
		textCol = th.Color(core.TokenColorTextQuaternary)
		if textCol.A < 0.1 {
			textCol = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		}
	} else if tr.isSelected(n.Key) {
		if tr.Directory {
			textCol = th.Color(core.TokenColorTextInverse)
			if textCol.A < 0.1 {
				textCol = render.RGBA{R: 255, G: 255, B: 255, A: 1}
			}
		}
	}
	if tr.Style.Text.A > 0 && !tr.isSelected(n.Key) {
		textCol = tr.Style.Text
	}

	if q == "" || !strings.Contains(title, q) {
		lab := primitive.NewText(title)
		lab.FontSize = fontSz
		lab.Face = tr.Face
		lab.Color = textCol
		return lab
	}
	// highlight match with colorError-ish
	hi := th.Color(core.TokenColorError)
	if hi.A < 0.1 {
		hi = render.Hex("#f50")
	}
	idx := strings.Index(title, q)
	before := title[:idx]
	mid := title[idx : idx+len(q)]
	after := title[idx+len(q):]
	row := primitive.Row()
	row.Gap = 0
	row.CrossAlign = core.CrossCenter
	if before != "" {
		t := primitive.NewText(before)
		t.FontSize, t.Face, t.Color = fontSz, tr.Face, textCol
		row.AddChild(t)
	}
	mt := primitive.NewText(mid)
	mt.FontSize, mt.Face, mt.Color = fontSz, tr.Face, hi
	row.AddChild(mt)
	if after != "" {
		t := primitive.NewText(after)
		t.FontSize, t.Face, t.Color = fontSz, tr.Face, textCol
		row.AddChild(t)
	}
	return row
}

func (tr *Tree) switcherIconName(n TreeNode, expanded bool) string {
	if n.SwitcherIcon != "" {
		return n.SwitcherIcon
	}
	if tr.SwitcherIcon != "" {
		return tr.SwitcherIcon
	}
	if tr.ShowLine {
		if expanded {
			return "minus"
		}
		return "plus"
	}
	if expanded {
		return "chevron-down"
	}
	return "chevron-right"
}

func (tr *Tree) linePainter(row treeRow, indent, titleH float64, th *core.Theme) func(*core.PaintContext, core.Size) {
	col := th.Color(core.TokenColorBorder)
	return func(pc *core.PaintContext, size core.Size) {
		if pc == nil {
			return
		}
		// vertical rails for each ancestor depth
		for d := 0; d < row.Depth; d++ {
			// skip if ancestor was last sibling (no continuing line)
			if d < len(row.LineMask) && row.LineMask[d] {
				continue
			}
			x := float64(d)*indent + indent/2
			pc.FillLocalRect(x, 0, DefaultTreeLineWidth, size.Height, col)
		}
		// elbow for current level
		if row.Depth > 0 {
			x := float64(row.Depth-1)*indent + indent/2
			midY := size.Height / 2
			h := midY
			if !row.IsLast {
				h = size.Height
			}
			pc.FillLocalRect(x, 0, DefaultTreeLineWidth, h, col)
			pc.FillLocalRect(x, midY, indent/2, DefaultTreeLineWidth, col)
		}
		_ = titleH
	}
}

func (tr *Tree) spinnerPainter() func(*core.PaintContext, core.Size) {
	return func(pc *core.PaintContext, size core.Size) {
		tr.paintSpinner(pc, size)
	}
}

func (tr *Tree) paintSpinner(pc *core.PaintContext, size core.Size) {
	if pc == nil {
		return
	}
	th := tr.theme()
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
	start := -math.Pi/2 + tr.spinPhase*2*math.Pi
	end := start + math.Pi*1.4
	pts := make([]float64, 0, (steps+1)*2)
	for i := 0; i <= steps; i++ {
		ang := start + (end-start)*float64(i)/float64(steps)
		pts = append(pts, cx+r*math.Cos(ang), cy+r*math.Sin(ang))
	}
	pc.StrokeLocalPolyline(pts, stroke, col)
}

// ---------------------------------------------------------------------------
// interaction apply*
// ---------------------------------------------------------------------------

func (tr *Tree) applyExpand(n TreeNode, want bool) {
	if tr.Disabled || n.Disabled {
		return
	}
	cur := tr.isExpanded(n.Key)
	if cur == want {
		// still may need loadData on expand
		if want {
			tr.maybeLoad(n)
		}
		return
	}
	if tr.ControlledExpanded {
		// compute next keys for callback only
		next := tr.ExpandedKeys()
		if want {
			if !tr.keyIn(next, n.Key) {
				next = append(next, n.Key)
			}
		} else {
			next = tr.removeKey(next, n.Key)
		}
		if tr.OnExpand != nil {
			tr.OnExpand(next, n, want)
		}
		if want {
			tr.maybeLoad(n)
		}
		return
	}
	if tr.expanded == nil {
		tr.expanded = make(map[string]bool)
	}
	tr.expanded[n.Key] = want
	// autoExpandParent false after user expand (antd controlled demo pattern)
	if tr.AutoExpandParent && !want {
		// keep
	}
	if tr.OnExpand != nil {
		tr.OnExpand(tr.ExpandedKeys(), n, want)
	}
	if want {
		tr.maybeLoad(n)
	}
	tr.rebuild()
}

func (tr *Tree) maybeLoad(n TreeNode) {
	if tr.LoadData == nil || n.IsLeaf {
		return
	}
	if len(n.Children) > 0 {
		return
	}
	if tr.loadedSet[n.Key] {
		return
	}
	// mark loading on node in TreeData
	tr.setNodeLoading(n.Key, true)
	tr.rebuild()
	tr.syncTicker()
	tr.LoadData(n)
}

// MarkLoaded records a key as loaded and clears Loading (call after loadData).
func (tr *Tree) MarkLoaded(key string) {
	if tr == nil {
		return
	}
	if tr.loadedSet == nil {
		tr.loadedSet = make(map[string]bool)
	}
	tr.loadedSet[key] = true
	if !tr.keyIn(tr.LoadedKeys, key) {
		tr.LoadedKeys = append(tr.LoadedKeys, key)
	}
	tr.setNodeLoading(key, false)
	n, _ := tr.findNode(key)
	if tr.OnLoad != nil {
		tr.OnLoad(append([]string(nil), tr.LoadedKeys...), n)
	}
	tr.rebuild()
	tr.syncTicker()
}

func (tr *Tree) setNodeLoading(key string, loading bool) {
	var walk func(nodes []TreeNode) []TreeNode
	walk = func(nodes []TreeNode) []TreeNode {
		for i := range nodes {
			if nodes[i].Key == key {
				nodes[i].Loading = loading
				return nodes
			}
			if len(nodes[i].Children) > 0 {
				nodes[i].Children = walk(nodes[i].Children)
			}
		}
		return nodes
	}
	tr.TreeData = walk(tr.TreeData)
}

func (tr *Tree) applySelect(n TreeNode) {
	if tr.Disabled || n.Disabled || !n.isSelectable() {
		return
	}
	selected := !tr.isSelected(n.Key)
	var next []string
	if tr.Multiple {
		if selected {
			next = append(append([]string(nil), tr.SelectedKeys...), n.Key)
		} else {
			next = tr.removeKey(tr.SelectedKeys, n.Key)
		}
	} else {
		if selected {
			next = []string{n.Key}
		} else {
			// antd typically keeps last selected; allow deselect to empty
			next = nil
		}
		// single-select: clicking another selects it
		if !tr.isSelected(n.Key) {
			next = []string{n.Key}
			selected = true
		} else {
			next = []string{n.Key}
			selected = true
		}
	}
	tr.focusKey = n.Key
	if tr.ControlledSelected {
		if tr.OnSelect != nil {
			tr.OnSelect(next, n, selected)
		}
		return
	}
	tr.SelectedKeys = next
	if tr.OnSelect != nil {
		tr.OnSelect(append([]string(nil), tr.SelectedKeys...), n, selected)
	}
	tr.rebuild()
}

func (tr *Tree) applyCheck(n TreeNode) {
	if tr.Disabled || n.Disabled || n.DisableCheckbox || !tr.Checkable {
		return
	}
	checked, _ := tr.checkState(n)
	want := !checked
	var next []string
	if tr.CheckStrictly {
		if want {
			next = append(append([]string(nil), tr.CheckedKeys...), n.Key)
		} else {
			next = tr.removeKey(tr.CheckedKeys, n.Key)
		}
	} else {
		// cascade descendants
		var keys []string
		tr.collectDescendantKeys(n, &keys)
		next = append([]string(nil), tr.CheckedKeys...)
		if want {
			for _, k := range keys {
				if !tr.keyIn(next, k) {
					next = append(next, k)
				}
			}
		} else {
			for _, k := range keys {
				next = tr.removeKey(next, k)
			}
		}
	}
	if tr.ControlledChecked {
		if tr.OnCheck != nil {
			tr.OnCheck(next, n, want)
		}
		return
	}
	tr.CheckedKeys = next
	tr.recomputeHalf()
	if tr.OnCheck != nil {
		tr.OnCheck(append([]string(nil), tr.CheckedKeys...), n, want)
	}
	tr.rebuild()
}

func (tr *Tree) applyDrop(info TreeDropInfo) {
	// remove drag node
	var dragObj TreeNode
	var found bool
	var remove func(nodes []TreeNode) []TreeNode
	remove = func(nodes []TreeNode) []TreeNode {
		out := nodes[:0]
		// keep capacity carefully
		out = make([]TreeNode, 0, len(nodes))
		for _, n := range nodes {
			if n.Key == info.DragKey {
				dragObj = n
				found = true
				continue
			}
			if len(n.Children) > 0 {
				n.Children = remove(n.Children)
			}
			out = append(out, n)
		}
		return out
	}
	tr.TreeData = remove(tr.TreeData)
	if !found {
		return
	}
	// insert
	var insert func(nodes []TreeNode) []TreeNode
	insert = func(nodes []TreeNode) []TreeNode {
		for i := range nodes {
			if nodes[i].Key == info.DropKey {
				if !info.DropToGap {
					// inside
					nodes[i].Children = append([]TreeNode{dragObj}, nodes[i].Children...)
					tr.expanded[info.DropKey] = true
					return nodes
				}
				// gap: before (-1) or after (1)
				pos := i
				if info.DropPosition >= 0 {
					pos = i + 1
				}
				out := make([]TreeNode, 0, len(nodes)+1)
				out = append(out, nodes[:pos]...)
				out = append(out, dragObj)
				out = append(out, nodes[pos:]...)
				return out
			}
			if len(nodes[i].Children) > 0 {
				nodes[i].Children = insert(nodes[i].Children)
			}
		}
		return nodes
	}
	tr.TreeData = insert(tr.TreeData)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func (tr *Tree) isExpanded(key string) bool {
	if tr.expanded == nil {
		return false
	}
	return tr.expanded[key]
}

func (tr *Tree) isSelected(key string) bool {
	return tr.keyIn(tr.SelectedKeys, key)
}

func (tr *Tree) checkState(n TreeNode) (checked, half bool) {
	if tr.CheckStrictly || len(n.Children) == 0 {
		return tr.keyIn(tr.CheckedKeys, n.Key), false
	}
	var leaves []string
	tr.collectLeafKeys(n, &leaves)
	if len(leaves) == 0 {
		return tr.keyIn(tr.CheckedKeys, n.Key), false
	}
	on := 0
	for _, k := range leaves {
		if tr.keyIn(tr.CheckedKeys, k) {
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

func (tr *Tree) checkStateKey(key string) (checked, half bool) {
	n, ok := tr.findNode(key)
	if !ok {
		return false, false
	}
	return tr.checkState(n)
}

func (tr *Tree) recomputeHalf() {
	tr.halfChecked = make(map[string]bool)
	if tr.CheckStrictly {
		return
	}
	var walk func(nodes []TreeNode)
	walk = func(nodes []TreeNode) {
		for _, n := range nodes {
			if len(n.Children) > 0 {
				_, half := tr.checkState(n)
				if half {
					tr.halfChecked[n.Key] = true
				}
				walk(n.Children)
			}
		}
	}
	walk(tr.TreeData)
}

func (tr *Tree) collectDescendantKeys(n TreeNode, out *[]string) {
	*out = append(*out, n.Key)
	for _, c := range n.Children {
		tr.collectDescendantKeys(c, out)
	}
}

func (tr *Tree) collectLeafKeys(n TreeNode, out *[]string) {
	if len(n.Children) == 0 {
		*out = append(*out, n.Key)
		return
	}
	for _, c := range n.Children {
		tr.collectLeafKeys(c, out)
	}
}

func (tr *Tree) findNode(key string) (TreeNode, bool) {
	var walk func(nodes []TreeNode) (TreeNode, bool)
	walk = func(nodes []TreeNode) (TreeNode, bool) {
		for _, n := range nodes {
			if n.Key == key {
				return n, true
			}
			if c, ok := walk(n.Children); ok {
				return c, true
			}
		}
		return TreeNode{}, false
	}
	return walk(tr.TreeData)
}

func (tr *Tree) expandAllInternal() {
	if tr.expanded == nil {
		tr.expanded = make(map[string]bool)
	}
	var walk func([]TreeNode)
	walk = func(nodes []TreeNode) {
		for _, n := range nodes {
			if len(n.Children) > 0 || (!n.IsLeaf && tr.LoadData != nil) {
				tr.expanded[n.Key] = true
			}
			if len(n.Children) > 0 {
				walk(n.Children)
			}
		}
	}
	walk(tr.TreeData)
}

func (tr *Tree) expandParentsOfSelectedAndChecked() {
	keys := append(append([]string(nil), tr.SelectedKeys...), tr.CheckedKeys...)
	keys = append(keys, tr.DefaultSelectedKeys...)
	keys = append(keys, tr.DefaultCheckedKeys...)
	for _, k := range keys {
		tr.expandParentsOf(k)
	}
}

func (tr *Tree) expandParentsMatching(q string) {
	if q == "" {
		return
	}
	var walk func(nodes []TreeNode, parents []string)
	walk = func(nodes []TreeNode, parents []string) {
		for _, n := range nodes {
			if strings.Contains(n.DisplayTitle(), q) {
				for _, p := range parents {
					tr.expanded[p] = true
				}
			}
			if len(n.Children) > 0 {
				walk(n.Children, append(parents, n.Key))
			}
		}
	}
	walk(tr.TreeData, nil)
}

func (tr *Tree) expandParentsOf(key string) {
	var walk func(nodes []TreeNode, parents []string) bool
	walk = func(nodes []TreeNode, parents []string) bool {
		for _, n := range nodes {
			if n.Key == key {
				for _, p := range parents {
					tr.expanded[p] = true
				}
				return true
			}
			if walk(n.Children, append(parents, n.Key)) {
				return true
			}
		}
		return false
	}
	walk(tr.TreeData, nil)
}

func (tr *Tree) focusAdjacent(key string, dir int) {
	idx := -1
	for i, r := range tr.visible {
		if r.Key == key {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	next := idx + dir
	if next < 0 || next >= len(tr.visible) {
		return
	}
	tr.focusKey = tr.visible[next].Key
	// visual rebuild not strictly needed; select optional
	tr.rebuild()
}

func (tr *Tree) keyIn(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

func (tr *Tree) removeKey(keys []string, k string) []string {
	out := make([]string, 0, len(keys))
	for _, x := range keys {
		if x != k {
			out = append(out, x)
		}
	}
	return out
}
