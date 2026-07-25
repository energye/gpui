package kit

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Transfer defaults — components/transfer/style prepareComponentToken.
// docs/antd/transfer.md §6.2 / §6.10
const (
	DefaultTransferListWidth    = 180.0
	DefaultTransferListHeight   = 200.0
	DefaultTransferListWidthLG  = 250.0
	DefaultTransferHeaderHeight = 40.0 // controlHeightLG
	DefaultTransferItemHeight   = 32.0 // controlHeight
	DefaultTransferPageSize     = 10
	DefaultTransferActionRight  = ">"
	DefaultTransferActionLeft   = "<"
	DefaultTransferItemUnit     = "项"
	DefaultTransferItemsUnit    = "项"
	DefaultTransferSearchPH     = "请输入搜索内容"
	DefaultTransferNotFound     = "暂无数据"
	DefaultTransferFocusOutset  = 1.5
)

// TransferDirection is left (source) or right (target).
type TransferDirection string

const (
	TransferLeft  TransferDirection = "left"
	TransferRight TransferDirection = "right"
)

// TransferStatus is Form validation chrome for both sections.
type TransferStatus int

const (
	TransferStatusNone TransferStatus = iota
	TransferStatusError
	TransferStatusWarning
)

// TransferItem is one row in dataSource (antd TransferItem).
type TransferItem struct {
	Key         string
	Title       string
	Description string
	Disabled    bool
}

// TransferLocale holds UI strings (antd locale.Transfer).
type TransferLocale struct {
	ItemUnit          string
	ItemsUnit         string
	SearchPlaceholder string
	NotFoundContent   string
}

// TransferListBodyProps is the children-render contract for custom list bodies
// (table-transfer, etc.). Matches antd Transfer render-props subset.
type TransferListBodyProps struct {
	Direction       TransferDirection
	Disabled        bool
	FilteredItems   []TransferItem
	SelectedKeys    []string
	OnItemSelect    func(key string, selected bool)
	OnItemSelectAll func(keys []string, selected bool)
}

// Transfer is Ant Design Transfer (dual-list shuttle).
//
//	Flex Row (Root, role=group)
//	  ├─ Section left
//	  ├─ Actions (primary small)
//	  └─ Section right
//
// Product contract: docs/antd/transfer.md §6 (P0 DoD).
type Transfer struct {
	Root *primitive.Flex

	leftSection  *primitive.Decorated
	rightSection *primitive.Decorated
	toRightBtn   *Button
	toLeftBtn    *Button

	dataSource []TransferItem
	targetKeys []string

	sourceSelected []string
	targetSelected []string
	// selectedControlled: SetSelectedKeys marks selection controlled.
	selectedControlled bool

	// Controlled: move only fires OnChange; parent must SetTargetKeys.
	Controlled bool
	Disabled   bool
	OneWay     bool
	ShowSearch bool
	// ShowSelectAll defaults true (antd).
	ShowSelectAll bool
	Status        TransferStatus

	Titles  [2]string
	Actions [2]string // [toRight, toLeft]; empty → defaults

	// ActionLoading drives Button.Loading on operation buttons (Ticker).
	ActionLoadingRight bool
	ActionLoadingLeft  bool

	FilterOption func(input string, item TransferItem, direction TransferDirection) bool
	RenderItem   func(item TransferItem) string
	Footer       func(direction TransferDirection) core.Node
	ListBody     func(props TransferListBodyProps) core.Node

	// Pagination
	paginationEnabled bool
	pageSize          int
	paginationSimple  bool
	leftPage          int
	rightPage         int

	// Geometry overrides (0 → default token).
	listWidth  float64
	listHeight float64

	Locale TransferLocale

	searchLeft  string
	searchRight string

	OnChange       func(targetKeys []string, direction TransferDirection, moveKeys []string)
	OnSelectChange func(sourceSelected, targetSelected []string)
	OnSearch       func(direction TransferDirection, value string)

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// metric cache
	metricListW   float64
	metricListH   float64
	metricHeaderH float64
	metricItemH   float64

	boundTree *core.Tree
}

// TransferItemsFromTitles builds TransferItems with Key=Title=s for each title.
func TransferItemsFromTitles(titles ...string) []TransferItem {
	out := make([]TransferItem, len(titles))
	for i, t := range titles {
		out[i] = TransferItem{Key: t, Title: t}
	}
	return out
}

// NewTransfer creates an empty Transfer with Ant defaults.
func NewTransfer() *Transfer {
	tr := &Transfer{
		ShowSelectAll:    true,
		Actions:          [2]string{DefaultTransferActionRight, DefaultTransferActionLeft},
		pageSize:         DefaultTransferPageSize,
		paginationSimple: true,
		leftPage:         1,
		rightPage:        1,
		Locale: TransferLocale{
			ItemUnit:          DefaultTransferItemUnit,
			ItemsUnit:         DefaultTransferItemsUnit,
			SearchPlaceholder: DefaultTransferSearchPH,
			NotFoundContent:   DefaultTransferNotFound,
		},
	}
	tr.rebuild()
	return tr
}

// Node returns the stable root.
func (tr *Transfer) Node() core.Node {
	if tr == nil {
		return nil
	}
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.Root
}

// --- data ---

// SetDataSource replaces the full data source and rebuilds.
func (tr *Transfer) SetDataSource(items []TransferItem) {
	if tr == nil {
		return
	}
	tr.dataSource = append([]TransferItem(nil), items...)
	tr.clampPages()
	tr.rebuild()
}

// DataSource returns a copy of the data source.
func (tr *Transfer) DataSource() []TransferItem {
	if tr == nil {
		return nil
	}
	return append([]TransferItem(nil), tr.dataSource...)
}

// SetTargetKeys sets the right-panel keys (antd targetKeys).
func (tr *Transfer) SetTargetKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.targetKeys = uniqueKeys(keys)
	// Drop selections that no longer belong to their side.
	tr.sourceSelected = filterKeysInSet(tr.sourceSelected, tr.leftKeySet())
	tr.targetSelected = filterKeysInSet(tr.targetSelected, tr.rightKeySet())
	tr.clampPages()
	tr.rebuild()
}

// TargetKeys returns a copy of current target keys.
func (tr *Transfer) TargetKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.targetKeys...)
}

// SetControlled toggles controlled targetKeys mode.
func (tr *Transfer) SetControlled(v bool) {
	if tr == nil {
		return
	}
	tr.Controlled = v
}

// SetSelectedKeys sets combined selected keys (split by side membership).
func (tr *Transfer) SetSelectedKeys(keys []string) {
	if tr == nil {
		return
	}
	tr.selectedControlled = true
	leftSet := tr.leftKeySet()
	rightSet := tr.rightKeySet()
	var src, tgt []string
	for _, k := range uniqueKeys(keys) {
		if leftSet[k] {
			src = append(src, k)
		} else if rightSet[k] {
			tgt = append(tgt, k)
		}
	}
	tr.sourceSelected = src
	tr.targetSelected = tgt
	tr.rebuild()
}

// SourceSelectedKeys returns left-side selected keys.
func (tr *Transfer) SourceSelectedKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.sourceSelected...)
}

// TargetSelectedKeys returns right-side selected keys.
func (tr *Transfer) TargetSelectedKeys() []string {
	if tr == nil {
		return nil
	}
	return append([]string(nil), tr.targetSelected...)
}

// --- config ---

// SetDisabled toggles whole-control disabled.
func (tr *Transfer) SetDisabled(v bool) {
	if tr == nil || tr.Disabled == v {
		return
	}
	tr.Disabled = v
	tr.rebuild()
}

// SetOneWay enables one-way mode (no left action; right items removable).
func (tr *Transfer) SetOneWay(v bool) {
	if tr == nil || tr.OneWay == v {
		return
	}
	tr.OneWay = v
	tr.rebuild()
}

// SetShowSearch toggles search boxes on both sides.
func (tr *Transfer) SetShowSearch(v bool) {
	if tr == nil || tr.ShowSearch == v {
		return
	}
	tr.ShowSearch = v
	tr.rebuild()
}

// SetShowSelectAll toggles header select-all checkboxes (default true).
func (tr *Transfer) SetShowSelectAll(v bool) {
	if tr == nil || tr.ShowSelectAll == v {
		return
	}
	tr.ShowSelectAll = v
	tr.rebuild()
}

// SetStatus sets validation chrome.
func (tr *Transfer) SetStatus(s TransferStatus) {
	if tr == nil || tr.Status == s {
		return
	}
	tr.Status = s
	tr.rebuild()
}

// SetTitles sets left and right header titles.
func (tr *Transfer) SetTitles(left, right string) {
	if tr == nil {
		return
	}
	tr.Titles = [2]string{left, right}
	tr.rebuild()
}

// SetActions sets operation button labels (empty → defaults ">" / "<").
func (tr *Transfer) SetActions(toRight, toLeft string) {
	if tr == nil {
		return
	}
	if toRight == "" {
		toRight = DefaultTransferActionRight
	}
	if toLeft == "" {
		toLeft = DefaultTransferActionLeft
	}
	tr.Actions = [2]string{toRight, toLeft}
	tr.rebuild()
}

// SetActionLoading sets loading on operation buttons (uses Button Ticker).
func (tr *Transfer) SetActionLoading(toRight, toLeft bool) {
	if tr == nil {
		return
	}
	tr.ActionLoadingRight = toRight
	tr.ActionLoadingLeft = toLeft
	if tr.toRightBtn != nil {
		tr.toRightBtn.SetLoading(toRight)
	}
	if tr.toLeftBtn != nil {
		tr.toLeftBtn.SetLoading(toLeft)
	}
	tr.bindTickers()
}

// SetFilterOption sets custom search filter.
func (tr *Transfer) SetFilterOption(fn func(input string, item TransferItem, direction TransferDirection) bool) {
	if tr == nil {
		return
	}
	tr.FilterOption = fn
	tr.clampPages()
	tr.rebuild()
}

// SetRender sets the row label renderer (string; complex nodes P1).
func (tr *Transfer) SetRender(fn func(item TransferItem) string) {
	if tr == nil {
		return
	}
	tr.RenderItem = fn
	tr.rebuild()
}

// SetFooter sets optional footer factory per direction.
func (tr *Transfer) SetFooter(fn func(direction TransferDirection) core.Node) {
	if tr == nil {
		return
	}
	tr.Footer = fn
	tr.rebuild()
}

// SetPagination enables/disables list pagination (pageSize default 10, simple).
func (tr *Transfer) SetPagination(enabled bool) {
	if tr == nil {
		return
	}
	tr.paginationEnabled = enabled
	if enabled && tr.pageSize <= 0 {
		tr.pageSize = DefaultTransferPageSize
	}
	tr.leftPage, tr.rightPage = 1, 1
	tr.rebuild()
}

// SetPaginationConfig sets pageSize and simple flag (also enables pagination).
func (tr *Transfer) SetPaginationConfig(pageSize int, simple bool) {
	if tr == nil {
		return
	}
	tr.paginationEnabled = true
	if pageSize <= 0 {
		pageSize = DefaultTransferPageSize
	}
	tr.pageSize = pageSize
	tr.paginationSimple = simple
	tr.leftPage, tr.rightPage = 1, 1
	tr.rebuild()
}

// SetListWidth overrides list width (0 → token default).
func (tr *Transfer) SetListWidth(w float64) {
	if tr == nil {
		return
	}
	tr.listWidth = w
	tr.rebuild()
}

// SetListHeight overrides list height (0 → token default).
func (tr *Transfer) SetListHeight(h float64) {
	if tr == nil {
		return
	}
	tr.listHeight = h
	tr.rebuild()
}

// SetListBody sets a custom list body factory (table-transfer). When set,
// built-in item list and pagination are not rendered (antd rule).
func (tr *Transfer) SetListBody(fn func(props TransferListBodyProps) core.Node) {
	if tr == nil {
		return
	}
	tr.ListBody = fn
	tr.rebuild()
}

// SetLocale overrides locale strings (zero fields keep previous/default).
func (tr *Transfer) SetLocale(loc TransferLocale) {
	if tr == nil {
		return
	}
	if loc.ItemUnit != "" {
		tr.Locale.ItemUnit = loc.ItemUnit
	}
	if loc.ItemsUnit != "" {
		tr.Locale.ItemsUnit = loc.ItemsUnit
	}
	if loc.SearchPlaceholder != "" {
		tr.Locale.SearchPlaceholder = loc.SearchPlaceholder
	}
	if loc.NotFoundContent != "" {
		tr.Locale.NotFoundContent = loc.NotFoundContent
	}
	tr.rebuild()
}

// SetOnChange sets the transfer callback.
func (tr *Transfer) SetOnChange(fn func(targetKeys []string, direction TransferDirection, moveKeys []string)) {
	if tr != nil {
		tr.OnChange = fn
	}
}

// SetOnSelectChange sets the selection callback.
func (tr *Transfer) SetOnSelectChange(fn func(sourceSelected, targetSelected []string)) {
	if tr != nil {
		tr.OnSelectChange = fn
	}
}

// SetOnSearch sets the search callback.
func (tr *Transfer) SetOnSearch(fn func(direction TransferDirection, value string)) {
	if tr != nil {
		tr.OnSearch = fn
	}
}

// SetTheme sets an explicit theme.
func (tr *Transfer) SetTheme(th *core.Theme) {
	if tr == nil {
		return
	}
	tr.Theme = th
	tr.rebuild()
}

// SetFace sets the font face for labels.
func (tr *Transfer) SetFace(face text.Face) {
	if tr == nil {
		return
	}
	tr.Face = face
	tr.rebuild()
}

// SetStyle sets optional visual overrides.
func (tr *Transfer) SetStyle(st Style) {
	if tr == nil {
		return
	}
	tr.Style = st
	if st.Face != nil {
		tr.Face = st.Face
	}
	tr.rebuild()
}

// SetAriaLabel sets the accessible name on the root group.
func (tr *Transfer) SetAriaLabel(name string) {
	if tr == nil {
		return
	}
	tr.AriaLabel = name
	if tr.Root != nil {
		tr.Root.Base().Label = name
	}
}

// --- commands ---

// SelectItem toggles/sets selection for a key on the given side.
func (tr *Transfer) SelectItem(dir TransferDirection, key string, selected bool) {
	if tr == nil || tr.Disabled || key == "" {
		return
	}
	item := tr.itemByKey(key)
	if item == nil || item.Disabled {
		return
	}
	if dir == TransferLeft {
		if !tr.leftKeySet()[key] {
			return
		}
		tr.sourceSelected = setKeySelected(tr.sourceSelected, key, selected)
	} else {
		if !tr.rightKeySet()[key] {
			return
		}
		tr.targetSelected = setKeySelected(tr.targetSelected, key, selected)
	}
	tr.fireSelectChange()
	tr.rebuild()
}

// SelectAllVisible selects or clears all enabled filtered items on a side
// (respects current search filter; full filtered set, not only current page —
// matches header checkbox over filtered list; page only limits display).
func (tr *Transfer) SelectAllVisible(dir TransferDirection) {
	if tr == nil || tr.Disabled {
		return
	}
	items := tr.filteredItems(dir)
	keys := make([]string, 0, len(items))
	for _, it := range items {
		if !it.Disabled {
			keys = append(keys, it.Key)
		}
	}
	// If all already selected → clear; else select all (antd header checkbox).
	cur := tr.selectedOf(dir)
	allOn := len(keys) > 0 && keysSubset(keys, cur)
	if dir == TransferLeft {
		if allOn {
			tr.sourceSelected = removeKeys(tr.sourceSelected, keys)
		} else {
			tr.sourceSelected = uniqueKeys(append(tr.sourceSelected, keys...))
		}
	} else {
		if allOn {
			tr.targetSelected = removeKeys(tr.targetSelected, keys)
		} else {
			tr.targetSelected = uniqueKeys(append(tr.targetSelected, keys...))
		}
	}
	tr.fireSelectChange()
	tr.rebuild()
}

// Move transfers selected items toward direction:
//   - TransferRight: source selected → target
//   - TransferLeft: target selected → source
func (tr *Transfer) Move(dir TransferDirection) {
	if tr == nil || tr.Disabled {
		return
	}
	var moveKeys []string
	if dir == TransferRight {
		if tr.ActionLoadingRight {
			return
		}
		moveKeys = filterEnabledKeys(tr.sourceSelected, tr.dataSource)
		// only keys currently on left
		left := tr.leftKeySet()
		var ok []string
		for _, k := range moveKeys {
			if left[k] {
				ok = append(ok, k)
			}
		}
		moveKeys = ok
		if len(moveKeys) == 0 {
			return
		}
		next := uniqueKeys(append(append([]string(nil), tr.targetKeys...), moveKeys...))
		tr.applyMove(next, TransferRight, moveKeys)
		tr.sourceSelected = removeKeys(tr.sourceSelected, moveKeys)
	} else {
		if tr.OneWay || tr.ActionLoadingLeft {
			return
		}
		moveKeys = filterEnabledKeys(tr.targetSelected, tr.dataSource)
		right := tr.rightKeySet()
		var ok []string
		for _, k := range moveKeys {
			if right[k] {
				ok = append(ok, k)
			}
		}
		moveKeys = ok
		if len(moveKeys) == 0 {
			return
		}
		next := removeKeys(tr.targetKeys, moveKeys)
		tr.applyMove(next, TransferLeft, moveKeys)
		tr.targetSelected = removeKeys(tr.targetSelected, moveKeys)
	}
	tr.fireSelectChange()
	tr.rebuild()
}

// SetSearch sets the search query for a side (also fires OnSearch).
func (tr *Transfer) SetSearch(dir TransferDirection, query string) {
	if tr == nil {
		return
	}
	if dir == TransferLeft {
		tr.searchLeft = query
		tr.leftPage = 1
	} else {
		tr.searchRight = query
		tr.rightPage = 1
	}
	if tr.OnSearch != nil {
		tr.OnSearch(dir, query)
	}
	tr.rebuild()
}

// SearchQuery returns the current search string for a side.
func (tr *Transfer) SearchQuery(dir TransferDirection) string {
	if tr == nil {
		return ""
	}
	if dir == TransferLeft {
		return tr.searchLeft
	}
	return tr.searchRight
}

// RemoveTargetItem removes one key from target (oneWay right-item remove).
func (tr *Transfer) RemoveTargetItem(key string) {
	if tr == nil || tr.Disabled || key == "" {
		return
	}
	if !tr.rightKeySet()[key] {
		return
	}
	item := tr.itemByKey(key)
	if item != nil && item.Disabled {
		return
	}
	next := removeKeys(tr.targetKeys, []string{key})
	tr.applyMove(next, TransferLeft, []string{key})
	tr.targetSelected = removeKeys(tr.targetSelected, []string{key})
	tr.fireSelectChange()
	tr.rebuild()
}

// FilteredItems returns filtered items for a side (tests / ListBody).
func (tr *Transfer) FilteredItems(dir TransferDirection) []TransferItem {
	if tr == nil {
		return nil
	}
	return tr.filteredItems(dir)
}

// PageItems returns the current page slice when pagination is on.
func (tr *Transfer) PageItems(dir TransferDirection) []TransferItem {
	if tr == nil {
		return nil
	}
	return tr.pageItems(dir)
}

// ListWidth returns effective list width.
func (tr *Transfer) ListWidth() float64 {
	if tr == nil {
		return DefaultTransferListWidth
	}
	if tr.metricListW > 0 {
		return tr.metricListW
	}
	return tr.resolveListWidth()
}

// ListHeight returns effective list height.
func (tr *Transfer) ListHeight() float64 {
	if tr == nil {
		return DefaultTransferListHeight
	}
	if tr.metricListH > 0 {
		return tr.metricListH
	}
	return tr.resolveListHeight()
}

// HeaderHeight returns header height metric.
func (tr *Transfer) HeaderHeight() float64 {
	if tr == nil {
		return DefaultTransferHeaderHeight
	}
	if tr.metricHeaderH > 0 {
		return tr.metricHeaderH
	}
	return tr.theme().SizeOr(core.TokenControlHeightLG, DefaultTransferHeaderHeight)
}

// ItemHeight returns item height metric.
func (tr *Transfer) ItemHeight() float64 {
	if tr == nil {
		return DefaultTransferItemHeight
	}
	if tr.metricItemH > 0 {
		return tr.metricItemH
	}
	return tr.theme().SizeOr(core.TokenControlHeight, DefaultTransferItemHeight)
}

// SectionNodes returns left and right section roots.
func (tr *Transfer) SectionNodes() (left, right core.Node) {
	if tr == nil {
		return nil, nil
	}
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.leftSection, tr.rightSection
}

// ActionButtons returns the operation buttons (toLeft may be nil in oneWay).
func (tr *Transfer) ActionButtons() (toRight, toLeft *Button) {
	if tr == nil {
		return nil, nil
	}
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.toRightBtn, tr.toLeftBtn
}

// AttachTicker binds operation-button loading tickers to the tree.
func (tr *Transfer) AttachTicker(t *core.Tree) {
	if tr == nil {
		return
	}
	tr.boundTree = t
	tr.bindTickers()
}

func (tr *Transfer) bindTickers() {
	if tr.boundTree == nil {
		return
	}
	if tr.toRightBtn != nil {
		tr.toRightBtn.AttachTicker(tr.boundTree)
	}
	if tr.toLeftBtn != nil {
		tr.toLeftBtn.AttachTicker(tr.boundTree)
	}
}

// --- internals ---

func (tr *Transfer) theme() *core.Theme {
	var n core.Node
	if tr.Root != nil {
		n = tr.Root
	}
	return themeOf(tr.Theme, n)
}

func (tr *Transfer) resolveListWidth() float64 {
	if tr.listWidth > 0 {
		return tr.listWidth
	}
	if tr.paginationEnabled && tr.ListBody == nil {
		return DefaultTransferListWidthLG
	}
	return DefaultTransferListWidth
}

func (tr *Transfer) resolveListHeight() float64 {
	if tr.listHeight > 0 {
		return tr.listHeight
	}
	return DefaultTransferListHeight
}

func (tr *Transfer) itemByKey(key string) *TransferItem {
	for i := range tr.dataSource {
		if tr.dataSource[i].Key == key {
			return &tr.dataSource[i]
		}
	}
	return nil
}

func (tr *Transfer) rightKeySet() map[string]bool {
	m := make(map[string]bool, len(tr.targetKeys))
	for _, k := range tr.targetKeys {
		m[k] = true
	}
	return m
}

func (tr *Transfer) leftKeySet() map[string]bool {
	right := tr.rightKeySet()
	m := make(map[string]bool)
	for _, it := range tr.dataSource {
		if !right[it.Key] {
			m[it.Key] = true
		}
	}
	return m
}

func (tr *Transfer) itemsOf(dir TransferDirection) []TransferItem {
	right := tr.rightKeySet()
	out := make([]TransferItem, 0, len(tr.dataSource))
	if dir == TransferRight {
		// preserve targetKeys order
		byKey := make(map[string]TransferItem, len(tr.dataSource))
		for _, it := range tr.dataSource {
			byKey[it.Key] = it
		}
		for _, k := range tr.targetKeys {
			if it, ok := byKey[k]; ok {
				out = append(out, it)
			}
		}
		return out
	}
	for _, it := range tr.dataSource {
		if !right[it.Key] {
			out = append(out, it)
		}
	}
	return out
}

func (tr *Transfer) itemLabel(it TransferItem) string {
	if tr.RenderItem != nil {
		return tr.RenderItem(it)
	}
	if it.Title != "" {
		return it.Title
	}
	return it.Key
}

func (tr *Transfer) matchFilter(input string, it TransferItem, dir TransferDirection) bool {
	if input == "" {
		return true
	}
	if tr.FilterOption != nil {
		return tr.FilterOption(input, it, dir)
	}
	// default: case-insensitive includes on rendered label + description
	q := strings.ToLower(input)
	lab := strings.ToLower(tr.itemLabel(it))
	if strings.Contains(lab, q) {
		return true
	}
	return strings.Contains(strings.ToLower(it.Description), q)
}

func (tr *Transfer) searchOf(dir TransferDirection) string {
	if dir == TransferLeft {
		return tr.searchLeft
	}
	return tr.searchRight
}

func (tr *Transfer) filteredItems(dir TransferDirection) []TransferItem {
	src := tr.itemsOf(dir)
	q := tr.searchOf(dir)
	if q == "" && tr.FilterOption == nil {
		return src
	}
	out := make([]TransferItem, 0, len(src))
	for _, it := range src {
		if tr.matchFilter(q, it, dir) {
			out = append(out, it)
		}
	}
	return out
}

func (tr *Transfer) pageOf(dir TransferDirection) int {
	if dir == TransferLeft {
		return tr.leftPage
	}
	return tr.rightPage
}

func (tr *Transfer) setPage(dir TransferDirection, page int) {
	if page < 1 {
		page = 1
	}
	if dir == TransferLeft {
		tr.leftPage = page
	} else {
		tr.rightPage = page
	}
}

func (tr *Transfer) pageItems(dir TransferDirection) []TransferItem {
	all := tr.filteredItems(dir)
	if !tr.paginationEnabled || tr.ListBody != nil {
		return all
	}
	ps := tr.pageSize
	if ps <= 0 {
		ps = DefaultTransferPageSize
	}
	page := tr.pageOf(dir)
	total := len(all)
	pages := total / ps
	if total%ps != 0 {
		pages++
	}
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		page = pages
		tr.setPage(dir, page)
	}
	start := (page - 1) * ps
	if start >= total {
		return nil
	}
	end := start + ps
	if end > total {
		end = total
	}
	return all[start:end]
}

func (tr *Transfer) clampPages() {
	for _, dir := range []TransferDirection{TransferLeft, TransferRight} {
		_ = tr.pageItems(dir) // clamps via pageItems
	}
}

func (tr *Transfer) selectedOf(dir TransferDirection) []string {
	if dir == TransferLeft {
		return tr.sourceSelected
	}
	return tr.targetSelected
}

func (tr *Transfer) applyMove(next []string, dir TransferDirection, moveKeys []string) {
	if !tr.Controlled {
		tr.targetKeys = next
	}
	if tr.OnChange != nil {
		tr.OnChange(append([]string(nil), next...), dir, append([]string(nil), moveKeys...))
	}
}

func (tr *Transfer) fireSelectChange() {
	if tr.OnSelectChange != nil {
		tr.OnSelectChange(append([]string(nil), tr.sourceSelected...), append([]string(nil), tr.targetSelected...))
	}
}

func (tr *Transfer) rebuild() {
	th := tr.theme()
	tr.metricListW = tr.resolveListWidth()
	tr.metricListH = tr.resolveListHeight()
	tr.metricHeaderH = th.SizeOr(core.TokenControlHeightLG, DefaultTransferHeaderHeight)
	tr.metricItemH = th.SizeOr(core.TokenControlHeight, DefaultTransferItemHeight)

	// Actions
	toRightLab := tr.Actions[0]
	if toRightLab == "" {
		toRightLab = DefaultTransferActionRight
	}
	toLeftLab := tr.Actions[1]
	if toLeftLab == "" {
		toLeftLab = DefaultTransferActionLeft
	}

	tr.toRightBtn = NewButton(toRightLab)
	tr.toRightBtn.SetType(ButtonPrimary)
	tr.toRightBtn.SetSize(ButtonSmall)
	tr.toRightBtn.SetFace(tr.Face)
	if tr.Theme != nil {
		tr.toRightBtn.Theme = tr.Theme
	}
	tr.toRightBtn.SetLoading(tr.ActionLoadingRight)
	tr.toRightBtn.SetDisabled(tr.Disabled || !tr.canMove(TransferRight))
	tr.toRightBtn.SetAriaLabel("to right")
	tr.toRightBtn.SetOnClick(func() { tr.Move(TransferRight) })

	var leftBtnNode core.Node
	if !tr.OneWay {
		tr.toLeftBtn = NewButton(toLeftLab)
		tr.toLeftBtn.SetType(ButtonPrimary)
		tr.toLeftBtn.SetSize(ButtonSmall)
		tr.toLeftBtn.SetFace(tr.Face)
		if tr.Theme != nil {
			tr.toLeftBtn.Theme = tr.Theme
		}
		tr.toLeftBtn.SetLoading(tr.ActionLoadingLeft)
		tr.toLeftBtn.SetDisabled(tr.Disabled || !tr.canMove(TransferLeft))
		tr.toLeftBtn.SetAriaLabel("to left")
		tr.toLeftBtn.SetOnClick(func() { tr.Move(TransferLeft) })
		leftBtnNode = tr.toLeftBtn.Node()
	} else {
		tr.toLeftBtn = nil
	}

	actionsCol := primitive.Column(tr.toRightBtn.Node())
	if leftBtnNode != nil {
		actionsCol.AddChild(leftBtnNode)
	}
	// antd marginXXS ≈ 4 between action buttons
	actionsCol.Gap = 4
	actionsCol.MainAlign = core.MainCenter
	actionsCol.CrossAlign = core.CrossCenter
	// margin around actions (antd marginXS ≈ 8)
	actionsWrap := primitive.NewBox(actionsCol)
	mx := th.SizeOr(core.TokenMarginSM, 8)
	if mx == 0 {
		mx = 8
	}
	actionsWrap.Padding = primitive.Symmetric(mx, 0)

	tr.leftSection = tr.buildSection(TransferLeft, th)
	tr.rightSection = tr.buildSection(TransferRight, th)

	if tr.Root == nil {
		tr.Root = primitive.Row()
	} else {
		tr.Root.ClearChildren()
	}
	tr.Root.AddChild(tr.leftSection)
	tr.Root.AddChild(actionsWrap)
	tr.Root.AddChild(tr.rightSection)
	tr.Root.MainAlign = core.MainStart
	tr.Root.CrossAlign = core.CrossCenter
	tr.Root.Gap = 0
	tr.Root.Base().Role = "group"
	if tr.AriaLabel != "" {
		tr.Root.Base().Label = tr.AriaLabel
	} else {
		tr.Root.Base().Label = "transfer"
	}
	tr.Root.SetThemeHook(func(*core.Theme) { tr.rebuild() })
	tr.Root.MarkNeedsLayout()
	tr.Root.MarkNeedsPaint()
	tr.bindTickers()
}

func (tr *Transfer) canMove(dir TransferDirection) bool {
	var sel []string
	if dir == TransferRight {
		sel = tr.sourceSelected
		set := tr.leftKeySet()
		for _, k := range sel {
			if !set[k] {
				continue
			}
			if it := tr.itemByKey(k); it != nil && !it.Disabled {
				return true
			}
		}
		return false
	}
	sel = tr.targetSelected
	set := tr.rightKeySet()
	for _, k := range sel {
		if !set[k] {
			continue
		}
		if it := tr.itemByKey(k); it != nil && !it.Disabled {
			return true
		}
	}
	return false
}

func (tr *Transfer) buildSection(dir TransferDirection, th *core.Theme) *primitive.Decorated {
	w := tr.metricListW
	h := tr.metricListH
	filtered := tr.filteredItems(dir)
	selected := tr.selectedOf(dir)
	selectedSet := make(map[string]bool, len(selected))
	for _, k := range selected {
		selectedSet[k] = true
	}

	// counts: selected among filtered enabled / total filtered
	var selectedCount, totalCount, enabledCount int
	totalCount = len(filtered)
	for _, it := range filtered {
		if it.Disabled {
			continue
		}
		enabledCount++
		if selectedSet[it.Key] {
			selectedCount++
		}
	}

	col := primitive.Column()
	col.MainAlign = core.MainStart
	col.CrossAlign = core.CrossStretch

	// Header
	header := tr.buildHeader(dir, selectedCount, totalCount, enabledCount, selectedSet, filtered, th)
	col.AddChild(header)

	// Search
	if tr.ShowSearch {
		col.AddChild(tr.buildSearch(dir, th))
	}

	// Body
	var body core.Node
	if tr.ListBody != nil {
		props := TransferListBodyProps{
			Direction:     dir,
			Disabled:      tr.Disabled,
			FilteredItems: append([]TransferItem(nil), filtered...),
			SelectedKeys:  append([]string(nil), selected...),
			OnItemSelect: func(key string, sel bool) {
				tr.SelectItem(dir, key, sel)
			},
			OnItemSelectAll: func(keys []string, sel bool) {
				if tr.Disabled {
					return
				}
				if sel {
					for _, k := range keys {
						tr.SelectItem(dir, k, true)
					}
				} else {
					for _, k := range keys {
						tr.SelectItem(dir, k, false)
					}
				}
			},
		}
		body = tr.ListBody(props)
		if body == nil {
			body = primitive.NewBox()
		}
	} else {
		body = tr.buildDefaultList(dir, th)
	}
	// flex body to fill remaining height roughly via fixed height box
	bodyBox := primitive.NewBox(body)
	// reserve header (+search +footer +pagination) — simple fixed body area
	bodyH := h - tr.metricHeaderH
	if tr.ShowSearch {
		bodyH -= th.SizeOr(core.TokenControlHeight, 32) + 8
	}
	if tr.Footer != nil {
		bodyH -= 40
	}
	if tr.paginationEnabled && tr.ListBody == nil {
		bodyH -= 36
	}
	if bodyH < 40 {
		bodyH = 40
	}
	bodyBox.Height = bodyH
	bodyBox.Width = w - 2 // border
	col.AddChild(bodyBox)

	// Footer
	if tr.Footer != nil {
		if fn := tr.Footer(dir); fn != nil {
			col.AddChild(fn)
		}
	}

	// Pagination (built-in list only)
	if tr.paginationEnabled && tr.ListBody == nil {
		col.AddChild(tr.buildPagination(dir, len(filtered), th))
	}

	sec := primitive.NewDecorated(col)
	sec.Width = w
	if tr.paginationEnabled && tr.ListBody == nil {
		// height auto-ish: keep min list height
		sec.Height = 0
	} else {
		sec.Height = h
	}
	sec.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	sec.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	sec.Background = th.Color(core.TokenColorBgContainer)
	sec.BorderColor = th.Color(core.TokenColorBorder)
	switch tr.Status {
	case TransferStatusError:
		sec.BorderColor = th.Color(core.TokenColorError)
	case TransferStatusWarning:
		sec.BorderColor = th.Color(core.TokenColorWarning)
	}
	if tr.Disabled {
		sec.Background = th.Color(core.TokenColorDisabledBg)
	}
	sec.Base().Role = "listbox"
	title := tr.Titles[0]
	if dir == TransferRight {
		title = tr.Titles[1]
	}
	if title != "" {
		sec.Base().Label = title
	} else if dir == TransferLeft {
		sec.Base().Label = "source list"
	} else {
		sec.Base().Label = "target list"
	}
	return sec
}

func (tr *Transfer) buildHeader(
	dir TransferDirection,
	selectedCount, totalCount, enabledCount int,
	selectedSet map[string]bool,
	filtered []TransferItem,
	th *core.Theme,
) core.Node {
	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainStart
	padX := th.SizeOr(core.TokenPaddingSM, 8)
	if padX == 0 {
		padX = 8
	}

	if tr.ShowSelectAll {
		cb := NewCheckbox("")
		cb.SetFace(tr.Face)
		if tr.Theme != nil {
			cb.SetTheme(tr.Theme)
		}
		allOn := enabledCount > 0 && selectedCount == enabledCount
		half := selectedCount > 0 && selectedCount < enabledCount
		cb.SetChecked(allOn)
		cb.SetIndeterminate(half)
		cb.SetDisabled(tr.Disabled || enabledCount == 0)
		cb.SetAriaLabel("select all")
		cb.SetOnChange(func(bool) {
			tr.SelectAllVisible(dir)
		})
		row.AddChild(cb.Node())
	}

	title := tr.Titles[0]
	if dir == TransferRight {
		title = tr.Titles[1]
	}
	if title != "" {
		lab := primitive.NewText(title)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = tr.Face
		lab.Color = th.Color(core.TokenColorText)
		row.AddChild(lab)
	}

	// flexible spacer pushes count to the end
	row.AddChild(primitive.NewFlexible(1, primitive.NewBox()))

	unit := tr.Locale.ItemUnit
	if totalCount > 1 {
		if tr.Locale.ItemsUnit != "" {
			unit = tr.Locale.ItemsUnit
		}
	}
	countText := fmt.Sprintf("%d/%d %s", selectedCount, totalCount, unit)
	cnt := primitive.NewText(countText)
	cnt.FontSize = th.SizeOr(core.TokenFontSize, 14)
	cnt.Face = tr.Face
	cnt.Color = th.Color(core.TokenColorTextSecondary)
	row.AddChild(cnt)

	headerBox := primitive.NewBox(row)
	headerBox.Height = tr.metricHeaderH
	headerBox.Padding = primitive.Symmetric(padX, 0)

	line := primitive.NewBox()
	line.Height = th.SizeOr(core.TokenLineWidth, 1)
	line.Color = th.Color(core.TokenColorSplit)
	out := primitive.Column(headerBox, line)
	out.CrossAlign = core.CrossStretch
	return out
}

func (tr *Transfer) buildSearch(dir TransferDirection, th *core.Theme) core.Node {
	ph := tr.Locale.SearchPlaceholder
	if ph == "" {
		ph = DefaultTransferSearchPH
	}
	in := NewInput(ph)
	in.SetFace(tr.Face)
	if tr.Theme != nil {
		in.SetTheme(tr.Theme)
	}
	in.SetSize(InputSmall)
	in.SetDisabled(tr.Disabled)
	in.SetValue(tr.searchOf(dir))
	in.SetOnChange(func(v string) {
		// update without full rebuild first? need rebuild for filter
		if dir == TransferLeft {
			tr.searchLeft = v
			tr.leftPage = 1
		} else {
			tr.searchRight = v
			tr.rightPage = 1
		}
		if tr.OnSearch != nil {
			tr.OnSearch(dir, v)
		}
		tr.rebuild()
	})
	pad := primitive.NewBox(in.Node())
	pad.Padding = primitive.All(8)
	return pad
}

func (tr *Transfer) buildDefaultList(dir TransferDirection, th *core.Theme) core.Node {
	items := tr.pageItems(dir)
	if len(items) == 0 {
		emptyLab := tr.Locale.NotFoundContent
		if emptyLab == "" {
			emptyLab = DefaultTransferNotFound
		}
		empty := primitive.NewText(emptyLab)
		empty.FontSize = th.SizeOr(core.TokenFontSize, 14)
		empty.Face = tr.Face
		empty.Color = th.Color(core.TokenColorDisabledText)
		box := primitive.NewBox(empty)
		box.Padding = primitive.All(16)
		return box
	}
	selected := tr.selectedOf(dir)
	selSet := make(map[string]bool, len(selected))
	for _, k := range selected {
		selSet[k] = true
	}
	col := primitive.Column()
	col.Gap = 0
	col.CrossAlign = core.CrossStretch
	showRemove := tr.OneWay && dir == TransferRight
	for _, it := range items {
		col.AddChild(tr.buildItem(dir, it, selSet[it.Key], showRemove, th))
	}
	scroll := primitive.NewScrollViewport(col)
	return scroll
}

func (tr *Transfer) buildItem(dir TransferDirection, it TransferItem, checked, showRemove bool, th *core.Theme) core.Node {
	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.MainAlign = core.MainStart
	padX := th.SizeOr(core.TokenPaddingSM, 8)
	if padX == 0 {
		padX = 8
	}

	itemDisabled := tr.Disabled || it.Disabled

	if !showRemove {
		cb := NewCheckbox("")
		cb.SetFace(tr.Face)
		if tr.Theme != nil {
			cb.SetTheme(tr.Theme)
		}
		cb.SetChecked(checked)
		cb.SetDisabled(itemDisabled)
		key := it.Key
		cb.SetOnChange(func(next bool) {
			tr.SelectItem(dir, key, next)
		})
		row.AddChild(cb.Node())
	}

	lab := primitive.NewText(tr.itemLabel(it))
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = tr.Face
	if itemDisabled {
		lab.Color = th.Color(core.TokenColorDisabledText)
	} else {
		lab.Color = th.Color(core.TokenColorText)
	}
	row.AddChild(primitive.NewFlexible(1, lab))

	if showRemove {
		rm := NewButton("×")
		rm.SetType(ButtonText)
		rm.SetSize(ButtonSmall)
		rm.SetFace(tr.Face)
		if tr.Theme != nil {
			rm.Theme = tr.Theme
		}
		rm.SetDisabled(itemDisabled)
		rm.SetAriaLabel("remove")
		key := it.Key
		rm.SetOnClick(func() { tr.RemoveTargetItem(key) })
		row.AddChild(rm.Node())
	}

	rowBox := primitive.NewBox(row)
	rowBox.Height = tr.metricItemH
	rowBox.Padding = primitive.Symmetric(padX, 0)

	bg := render.RGBA{}
	if checked && !itemDisabled {
		bg = antItemSelectedFill(th)
	}
	dec := primitive.NewDecorated(rowBox)
	dec.Background = bg
	dec.Base().Role = "option"
	dec.Base().Label = tr.itemLabel(it)
	if itemDisabled {
		return dec
	}
	// click row toggles select (except remove button handles itself)
	if !showRemove {
		key := it.Key
		was := checked
		press := primitive.NewPressable(dec)
		press.ColorHovered = antItemHoverFill(th)
		press.Click = func() {
			tr.SelectItem(dir, key, !was)
		}
		return press
	}
	return dec
}

func (tr *Transfer) buildPagination(dir TransferDirection, totalFiltered int, th *core.Theme) core.Node {
	ps := tr.pageSize
	if ps <= 0 {
		ps = DefaultTransferPageSize
	}
	p := NewPagination()
	p.SetFace(tr.Face)
	if tr.Theme != nil {
		p.SetTheme(tr.Theme)
	}
	p.SetTotal(totalFiltered)
	p.SetPageSize(ps)
	p.SetSimple(tr.paginationSimple)
	p.SetSimpleReadOnly(true)
	p.SetCurrent(tr.pageOf(dir))
	p.SetDisabled(tr.Disabled)
	p.SetOnChange(func(page, _ int) {
		tr.setPage(dir, page)
		tr.rebuild()
	})
	box := primitive.NewBox(p.Node())
	box.Padding = primitive.All(8)
	return box
}

// --- helpers ---

func uniqueKeys(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if k == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func setKeySelected(keys []string, key string, selected bool) []string {
	if selected {
		return uniqueKeys(append(keys, key))
	}
	return removeKeys(keys, []string{key})
}

func removeKeys(keys, drop []string) []string {
	if len(keys) == 0 {
		return nil
	}
	m := make(map[string]bool, len(drop))
	for _, k := range drop {
		m[k] = true
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !m[k] {
			out = append(out, k)
		}
	}
	return out
}

func filterKeysInSet(keys []string, set map[string]bool) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if set[k] {
			out = append(out, k)
		}
	}
	return out
}

func filterEnabledKeys(keys []string, data []TransferItem) []string {
	dis := make(map[string]bool, len(data))
	for _, it := range data {
		if it.Disabled {
			dis[it.Key] = true
		}
	}
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		if !dis[k] {
			out = append(out, k)
		}
	}
	return out
}

func keysSubset(need, have []string) bool {
	if len(need) == 0 {
		return true
	}
	m := make(map[string]bool, len(have))
	for _, k := range have {
		m[k] = true
	}
	for _, k := range need {
		if !m[k] {
			return false
		}
	}
	return true
}
