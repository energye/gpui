package kit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Table tokens — components/table/style prepareComponentToken.
// docs/antd/table.md §6.2 / §6.10
const (
	// DefaultTableCellPadBlockLG is cellPaddingBlock (padding=16) for size=large.
	DefaultTableCellPadBlockLG = 16.0
	// DefaultTableCellPadInlineLG is cellPaddingInline (padding=16).
	DefaultTableCellPadInlineLG = 16.0
	// DefaultTableCellPadBlockMD is cellPaddingBlockMD (paddingSM=12).
	DefaultTableCellPadBlockMD = 12.0
	// DefaultTableCellPadInlineMD is cellPaddingInlineMD (paddingXS=8).
	DefaultTableCellPadInlineMD = 8.0
	// DefaultTableCellPadBlockSM is cellPaddingBlockSM (paddingXS=8).
	DefaultTableCellPadBlockSM = 8.0
	// DefaultTableCellPadInlineSM is cellPaddingInlineSM (paddingXS=8).
	DefaultTableCellPadInlineSM = 8.0
	// DefaultTableFontSize is cellFontSize (fontSize=14).
	DefaultTableFontSize = 14.0
	// DefaultTableRadius is table borderRadius (6).
	DefaultTableRadius = 6.0
	// DefaultTableHeaderRadius is headerBorderRadius (borderRadiusLG=8).
	DefaultTableHeaderRadius = 8.0
	// DefaultTableLineWidth is lineWidth.
	DefaultTableLineWidth = 1.0
	// DefaultTableSelectionColWidth is selectionColumnWidth (=controlHeight 32).
	DefaultTableSelectionColWidth = 32.0
	// DefaultTableExpandColWidth is expand column width fallback.
	DefaultTableExpandColWidth = 48.0
	// DefaultTableRowLine is approximate content line height for row-height estimate.
	DefaultTableRowLine = 22.0
	// DefaultTableFocusRingOutset approximates Ant focus-visible outset.
	DefaultTableFocusRingOutset = 1.5
	// DefaultTableFilterDropdownWidth is tableFilterDropdownWidth.
	DefaultTableFilterDropdownWidth = 120.0
	// DefaultTableFilterDropdownHeight is tableFilterDropdownHeight.
	DefaultTableFilterDropdownHeight = 264.0
	// DefaultTableFilterSearchWidth is tableFilterDropdownSearchWidth.
	DefaultTableFilterSearchWidth = 140.0
	// DefaultTablePageSize is pagination.pageSize default.
	DefaultTablePageSize = 10
	// DefaultTableEmptyText is locale.emptyText (en).
	DefaultTableEmptyText = "No data"
	// DefaultTableRowKey is antd rowKey default field name.
	DefaultTableRowKey = "key"
)

// TableSize is antd size: large | middle | small (default large).
type TableSize int

const (
	// TableLarge is the antd default size.
	TableLarge TableSize = iota
	// TableMiddle is medium density.
	TableMiddle
	// TableSmall is compact density.
	TableSmall
)

// TableSortOrder is ascend | descend | none.
type TableSortOrder int

const (
	// TableSortNone clears sort.
	TableSortNone TableSortOrder = iota
	// TableSortAscend is ascend.
	TableSortAscend
	// TableSortDescend is descend.
	TableSortDescend
)

// TableFilterMode is menu | tree (antd filterMode).
type TableFilterMode int

const (
	// TableFilterMenu is the default flat filter list.
	TableFilterMenu TableFilterMode = iota
	// TableFilterTree is hierarchical filter options.
	TableFilterTree
)

// TableFixed pins a column (antd fixed).
type TableFixed int

const (
	// TableFixedNone scrolls with body.
	TableFixedNone TableFixed = iota
	// TableFixedStart pins to the start (left in LTR; antd 'start' | 'left' | true).
	TableFixedStart
	// TableFixedEnd pins to the end (right in LTR; antd 'end' | 'right').
	TableFixedEnd
)

// TableSelectionType is checkbox | radio.
type TableSelectionType int

const (
	// TableSelectionCheckbox is the default multi-select.
	TableSelectionCheckbox TableSelectionType = iota
	// TableSelectionRadio is single-select.
	TableSelectionRadio
)

// TableChangeAction is onChange extra.action.
type TableChangeAction string

const (
	TableActionPaginate TableChangeAction = "paginate"
	TableActionSort     TableChangeAction = "sort"
	TableActionFilter   TableChangeAction = "filter"
)

// TableRecord is one dataSource row (antd record object).
type TableRecord map[string]any

// TableFilter is one filter option (antd ColumnFilterItem).
type TableFilter struct {
	Text     string
	Value    string
	Children []TableFilter
}

// TableColumn describes one column or column-group (antd ColumnType).
type TableColumn struct {
	Key              string
	Title            string
	DataIndex        string // empty → Key
	Width            float64
	Flex             float64 // used when Width==0; default 1
	Fixed            TableFixed
	Sorter           func(a, b TableRecord) int // <0 a before b
	SortDirections   []TableSortOrder           // empty → ascend,descend
	DefaultSortOrder TableSortOrder
	Filters          []TableFilter
	OnFilter         func(value string, record TableRecord) bool
	FilterMode       TableFilterMode
	FilterSearch     bool
	// Render custom cell; nil → string of record[dataIndex].
	Render func(value any, record TableRecord, index int) core.Node
	// Children builds a header group (antd ColumnGroup / jsx ColumnGroup).
	Children []TableColumn
}

// TableCheckboxProps mirrors getCheckboxProps return.
type TableCheckboxProps struct {
	Disabled bool
	Name     string
}

// TableSelectionItem is a custom selection menu entry (antd selections[]).
type TableSelectionItem struct {
	Key      string
	Text     string
	OnSelect func(changeableRowKeys []string)
}

// Built-in selection keys (antd Table.SELECTION_*).
const (
	TableSelectionAll    = "SELECTION_ALL"
	TableSelectionInvert = "SELECTION_INVERT"
	TableSelectionNone   = "SELECTION_NONE"
)

// TableRowSelection is antd rowSelection config (P0 subset).
type TableRowSelection struct {
	Type            TableSelectionType
	SelectedRowKeys []string
	// Controlled: when true, selection only updates via SetSelectedRowKeys.
	Controlled bool
	OnChange   func(selectedRowKeys []string, selectedRows []TableRecord)
	// GetCheckboxProps disables/names a row checkbox.
	GetCheckboxProps func(record TableRecord) TableCheckboxProps
	// Selections: empty → no extra menu; use TableSelectionAll/Invert/None keys
	// or custom TableSelectionItem entries.
	Selections []TableSelectionItem
	// HideSelectAll hides the header checkbox (antd hideSelectAll).
	HideSelectAll bool
	// ColumnWidth overrides selection column width (0 → default).
	ColumnWidth float64
}

// TableExpandable is antd expandable config (P0 subset).
type TableExpandable struct {
	ExpandedRowRender func(record TableRecord, index int) core.Node
	RowExpandable     func(record TableRecord) bool
	ExpandedRowKeys   []string
	// Controlled expanded keys.
	Controlled bool
	OnExpand   func(expanded bool, record TableRecord)
	// ColumnWidth overrides expand column width (0 → default).
	ColumnWidth float64
}

// TableScroll is antd scroll config.
type TableScroll struct {
	X float64 // min content width / horizontal scroll threshold
	Y float64 // body max height; >0 enables body scroll + fixed header
}

// TablePaginationState is the pagination slice of onChange.
type TablePaginationState struct {
	Current  int
	PageSize int
	Total    int
}

// TableSorterResult is the sorter slice of onChange.
type TableSorterResult struct {
	ColumnKey string
	Field     string
	Order     TableSortOrder
}

// TableChangeExtra is onChange extra.
type TableChangeExtra struct {
	Action            TableChangeAction
	CurrentDataSource []TableRecord
}

// TableLocale holds empty / filter strings.
type TableLocale struct {
	EmptyText               string
	FilterConfirm           string
	FilterReset             string
	FilterSearchPlaceholder string
}

// Table is Ant Design Table — data grid with sort/filter/select/expand/page.
//
//	Column root (role=table)
//	  title?
//	  frame (bordered optional)
//	    header (fixed when scroll.y)
//	    body (ScrollViewport when scroll.y / rows)
//	    loading Spin overlay
//	  footer?
//	  Pagination?
//
// Product contract: docs/antd/table.md §6 (P0 DoD).
// Root pointer stays stable across rebuild when possible (ClearChildren).
type Table struct {
	Root *primitive.Flex

	frame      *primitive.Decorated
	headerHost *primitive.Decorated
	bodyHost   *primitive.Decorated
	bodyScroll *primitive.ScrollViewport
	hScroll    *primitive.ScrollViewport
	emptyNode  *Empty
	spin       *Spin
	pager      *Pagination
	titleHost  *primitive.Decorated
	footerHost *primitive.Decorated

	columns    []TableColumn
	dataSource []TableRecord

	rowKey     string
	rowKeyFunc func(TableRecord) string

	size         TableSize
	bordered     bool
	showHeader   bool
	loading      bool
	rowHoverable bool

	titleText  string
	titleNode  core.Node
	footerText string
	footerNode core.Node

	paginationEnabled bool
	pagCurrent        int
	pagPageSize       int
	pagControlled     bool

	rowSelection *TableRowSelection
	expandable   *TableExpandable
	scroll       TableScroll

	// Internal non-controlled selection / expand / sort / filter state.
	selectedKeys []string
	expandedKeys []string
	sortKey      string
	sortOrder    TableSortOrder
	// filters: columnKey → selected filter values
	filters map[string][]string
	// filter drafts while dropdown open
	filterDraft map[string][]string
	filterOpen  map[string]bool
	filterQuery map[string]string

	// onChange
	onChange func(TablePaginationState, map[string][]string, TableSorterResult, TableChangeExtra)

	// Derived view (filter → sort → page)
	viewAll  []TableRecord // after filter+sort
	viewPage []TableRecord // current page slice
	// flat leaf columns (no groups)
	leafCols []TableColumn

	// test / chrome hooks
	headerCells map[string]*primitive.Pressable // column key
	rowNodes    []*primitive.Pressable
	selectAllCB *Checkbox
	filterDDs   map[string]*Dropdown

	Locale    TableLocale
	Face      text.Face
	Theme     *core.Theme
	AriaLabel string
	Style     Style

	life tickerLifecycle

	// cached colors for L2 tests
	headerBg  render.RGBA
	textColor render.RGBA
	borderCol render.RGBA
}

// NewTable creates an empty Table with Ant defaults (§6.10).
func NewTable() *Table {
	t := &Table{
		size:              TableLarge,
		showHeader:        true,
		rowHoverable:      true,
		paginationEnabled: true,
		pagCurrent:        1,
		pagPageSize:       DefaultTablePageSize,
		rowKey:            DefaultTableRowKey,
		filters:           map[string][]string{},
		filterDraft:       map[string][]string{},
		filterOpen:        map[string]bool{},
		filterQuery:       map[string]string{},
		headerCells:       map[string]*primitive.Pressable{},
		filterDDs:         map[string]*Dropdown{},
		Locale: TableLocale{
			EmptyText:               DefaultTableEmptyText,
			FilterConfirm:           "OK",
			FilterReset:             "Reset",
			FilterSearchPlaceholder: "Search in filters",
		},
	}
	t.rebuild()
	return t
}

// NewTableWith creates a Table with columns and data (convenience).
func NewTableWith(columns []TableColumn, data []TableRecord) *Table {
	t := NewTable()
	t.SetColumns(columns)
	t.SetDataSource(data)
	return t
}

// TableRecordsFromStringMaps converts legacy []map[string]string rows.
func TableRecordsFromStringMaps(rows []map[string]string) []TableRecord {
	out := make([]TableRecord, len(rows))
	for i, r := range rows {
		rec := TableRecord{}
		for k, v := range r {
			rec[k] = v
		}
		out[i] = rec
	}
	return out
}

// Node returns the composition root.
func (t *Table) Node() core.Node {
	if t == nil {
		return nil
	}
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// ChromeNode returns the root (a11y host).
func (t *Table) ChromeNode() core.Node { return t.Node() }

// Frame returns the bordered table frame (tests).
func (t *Table) Frame() *primitive.Decorated { return t.frame }

// HeaderHost returns the header chrome (tests).
func (t *Table) HeaderHost() *primitive.Decorated { return t.headerHost }

// BodyHost returns the body chrome (tests).
func (t *Table) BodyHost() *primitive.Decorated { return t.bodyHost }

// EmptyNode returns the Empty instance when data is empty (tests).
func (t *Table) EmptyNode() *Empty { return t.emptyNode }

// SpinNode returns the loading Spin (tests).
func (t *Table) SpinNode() *Spin { return t.spin }

// PaginationNode returns the composed Pagination (tests; nil when disabled).
func (t *Table) PaginationNode() *Pagination { return t.pager }

// ViewRecords returns the filtered+sorted full list (tests).
func (t *Table) ViewRecords() []TableRecord { return append([]TableRecord(nil), t.viewAll...) }

// PageRecords returns the current page slice (tests).
func (t *Table) PageRecords() []TableRecord { return append([]TableRecord(nil), t.viewPage...) }

// SelectedRowKeys returns current selection keys.
func (t *Table) SelectedRowKeys() []string {
	return append([]string(nil), t.selectedKeys...)
}

// ExpandedRowKeys returns current expanded keys.
func (t *Table) ExpandedRowKeys() []string {
	return append([]string(nil), t.expandedKeys...)
}

// SortState returns active sort key/order.
func (t *Table) SortState() (key string, order TableSortOrder) {
	return t.sortKey, t.sortOrder
}

// Filters returns a copy of active filters.
func (t *Table) Filters() map[string][]string {
	out := make(map[string][]string, len(t.filters))
	for k, v := range t.filters {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// PaginationState returns current pagination snapshot.
func (t *Table) PaginationState() TablePaginationState {
	return TablePaginationState{
		Current:  t.pagCurrent,
		PageSize: t.pagPageSize,
		Total:    len(t.viewAll),
	}
}

// CellPad returns resolved cell padding for current size (tests / L2).
func (t *Table) CellPad() (block, inline float64) {
	return t.cellPad()
}

// RowHeightEstimate returns pad*2 + line for current size (tests / L2).
func (t *Table) RowHeightEstimate() float64 {
	b, _ := t.cellPad()
	return b*2 + DefaultTableRowLine
}

// HeaderBg returns resolved header background (tests / L2).
func (t *Table) HeaderBg() render.RGBA { return t.headerBg }

// TextColor returns resolved body text color (tests / L2).
func (t *Table) TextColor() render.RGBA { return t.textColor }

// BorderColor returns resolved border color (tests / L2).
func (t *Table) BorderColor() render.RGBA { return t.borderCol }

// IsLoading reports loading state.
func (t *Table) IsLoading() bool { return t.loading }

// Size returns current size.
func (t *Table) Size() TableSize { return t.size }

// IsBordered reports bordered.
func (t *Table) IsBordered() bool { return t.bordered }

// ShowHeader reports showHeader.
func (t *Table) ShowHeader() bool { return t.showHeader }

// IsPaginationEnabled reports whether pagination is on.
func (t *Table) IsPaginationEnabled() bool { return t.paginationEnabled }

// LeafColumns returns flattened leaf columns (tests).
func (t *Table) LeafColumns() []TableColumn {
	return append([]TableColumn(nil), t.leafCols...)
}

// --- setters ---

// SetColumns replaces column definitions.
func (t *Table) SetColumns(cols []TableColumn) {
	if t == nil {
		return
	}
	t.columns = append([]TableColumn(nil), cols...)
	t.applyDefaultSortFromColumns()
	t.rebuild()
}

// SetDataSource replaces data and recomputes the view.
func (t *Table) SetDataSource(data []TableRecord) {
	if t == nil {
		return
	}
	t.dataSource = append([]TableRecord(nil), data...)
	t.rebuild()
}

// SetRowKey sets the record field used as row key (default "key").
func (t *Table) SetRowKey(field string) {
	if t == nil {
		return
	}
	if field == "" {
		field = DefaultTableRowKey
	}
	t.rowKey = field
	t.rowKeyFunc = nil
	t.rebuild()
}

// SetRowKeyFunc sets a custom row key extractor (overrides field).
func (t *Table) SetRowKeyFunc(fn func(TableRecord) string) {
	if t == nil {
		return
	}
	t.rowKeyFunc = fn
	t.rebuild()
}

// SetSize sets large|middle|small.
func (t *Table) SetSize(sz TableSize) {
	if t == nil {
		return
	}
	t.size = sz
	t.rebuild()
}

// SetBordered toggles outer + cell borders.
func (t *Table) SetBordered(v bool) {
	if t == nil {
		return
	}
	t.bordered = v
	t.rebuild()
}

// SetShowHeader toggles the header row.
func (t *Table) SetShowHeader(v bool) {
	if t == nil {
		return
	}
	t.showHeader = v
	t.rebuild()
}

// SetLoading toggles body Spin overlay (header retained).
func (t *Table) SetLoading(v bool) {
	if t == nil {
		return
	}
	t.loading = v
	t.life.setActive(v)
	t.rebuild()
}

// SetRowHoverable toggles row hover chrome (default true).
func (t *Table) SetRowHoverable(v bool) {
	if t == nil {
		return
	}
	t.rowHoverable = v
	t.rebuild()
}

// SetTitle sets the string title area above the table.
func (t *Table) SetTitle(s string) {
	if t == nil {
		return
	}
	t.titleText = s
	t.titleNode = nil
	t.rebuild()
}

// SetTitleNode sets a custom title node.
func (t *Table) SetTitleNode(n core.Node) {
	if t == nil {
		return
	}
	t.titleNode = n
	t.titleText = ""
	t.rebuild()
}

// SetFooter sets the string footer area below the table body.
func (t *Table) SetFooter(s string) {
	if t == nil {
		return
	}
	t.footerText = s
	t.footerNode = nil
	t.rebuild()
}

// SetFooterNode sets a custom footer node.
func (t *Table) SetFooterNode(n core.Node) {
	if t == nil {
		return
	}
	t.footerNode = n
	t.footerText = ""
	t.rebuild()
}

// SetPaginationEnabled toggles pagination (false → show all rows).
func (t *Table) SetPaginationEnabled(v bool) {
	if t == nil {
		return
	}
	t.paginationEnabled = v
	if !v {
		t.pagCurrent = 1
	}
	t.rebuild()
}

// SetPagination sets current page + pageSize (controlled-style).
func (t *Table) SetPagination(current, pageSize int) {
	if t == nil {
		return
	}
	if current < 1 {
		current = 1
	}
	if pageSize < 1 {
		pageSize = DefaultTablePageSize
	}
	t.pagCurrent = current
	t.pagPageSize = pageSize
	t.pagControlled = true
	t.paginationEnabled = true
	t.rebuild()
}

// SetPageSize sets pageSize (keeps current when possible).
func (t *Table) SetPageSize(n int) {
	if t == nil {
		return
	}
	if n < 1 {
		n = DefaultTablePageSize
	}
	t.pagPageSize = n
	t.pagControlled = true
	t.rebuild()
}

// SetCurrentPage sets pagination current (1-based).
func (t *Table) SetCurrentPage(page int) {
	if t == nil {
		return
	}
	if page < 1 {
		page = 1
	}
	t.pagCurrent = page
	t.pagControlled = true
	t.rebuild()
}

// SetRowSelection installs rowSelection config (nil clears).
func (t *Table) SetRowSelection(rs *TableRowSelection) {
	if t == nil {
		return
	}
	t.rowSelection = rs
	if rs != nil {
		t.selectedKeys = append([]string(nil), rs.SelectedRowKeys...)
	} else {
		t.selectedKeys = nil
	}
	t.rebuild()
}

// SetSelectedRowKeys updates selection (controlled path also updates config).
func (t *Table) SetSelectedRowKeys(keys []string) {
	if t == nil {
		return
	}
	t.selectedKeys = append([]string(nil), keys...)
	if t.rowSelection != nil {
		t.rowSelection.SelectedRowKeys = append([]string(nil), keys...)
	}
	t.rebuild()
}

// SetExpandable installs expandable config (nil clears).
func (t *Table) SetExpandable(ex *TableExpandable) {
	if t == nil {
		return
	}
	t.expandable = ex
	if ex != nil {
		t.expandedKeys = append([]string(nil), ex.ExpandedRowKeys...)
	} else {
		t.expandedKeys = nil
	}
	t.rebuild()
}

// SetExpandedRowKeys updates expanded keys.
func (t *Table) SetExpandedRowKeys(keys []string) {
	if t == nil {
		return
	}
	t.expandedKeys = append([]string(nil), keys...)
	if t.expandable != nil {
		t.expandable.ExpandedRowKeys = append([]string(nil), keys...)
	}
	t.rebuild()
}

// SetScroll sets scroll.x / scroll.y.
func (t *Table) SetScroll(sc TableScroll) {
	if t == nil {
		return
	}
	t.scroll = sc
	t.rebuild()
}

// SetOnChange sets the unified change callback.
func (t *Table) SetOnChange(fn func(TablePaginationState, map[string][]string, TableSorterResult, TableChangeExtra)) {
	if t == nil {
		return
	}
	t.onChange = fn
}

// SetLocale overrides empty/filter strings.
func (t *Table) SetLocale(loc TableLocale) {
	if t == nil {
		return
	}
	if loc.EmptyText != "" {
		t.Locale.EmptyText = loc.EmptyText
	}
	if loc.FilterConfirm != "" {
		t.Locale.FilterConfirm = loc.FilterConfirm
	}
	if loc.FilterReset != "" {
		t.Locale.FilterReset = loc.FilterReset
	}
	if loc.FilterSearchPlaceholder != "" {
		t.Locale.FilterSearchPlaceholder = loc.FilterSearchPlaceholder
	}
	t.rebuild()
}

// SetTheme sets the theme override.
func (t *Table) SetTheme(th *core.Theme) {
	if t == nil {
		return
	}
	t.Theme = th
	t.rebuild()
}

// SetFace sets the font face.
func (t *Table) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	t.rebuild()
}

// SetAriaLabel sets the accessible name.
func (t *Table) SetAriaLabel(name string) {
	if t == nil {
		return
	}
	t.AriaLabel = name
	if t.Root != nil {
		t.Root.Base().Label = name
	}
}

// SetStyle applies shallow root style overrides.
func (t *Table) SetStyle(st Style) {
	if t == nil {
		return
	}
	t.Style = st
	t.rebuild()
}

// AttachTicker binds loading Spin animation.
func (t *Table) AttachTicker(tree *core.Tree) {
	if t == nil {
		return
	}
	t.life.attach(tree, t, t.loading)
	if t.spin != nil {
		t.spin.AttachTicker(tree)
	}
}

// Tick advances loading spin when active.
func (t *Table) Tick(dt float64) bool {
	if t == nil || !t.loading {
		return false
	}
	var nt *core.Tree
	if t.Root != nil {
		nt = t.Root.Tree()
	}
	if !t.life.stillMounted(nt) {
		return false
	}
	if t.spin != nil {
		return t.spin.Tick(dt)
	}
	return true
}

// --- interactive helpers (tests + internal) ---

// GoToPage changes page and fires onChange(paginate) when not controlled externally.
func (t *Table) GoToPage(page int) {
	if t == nil || !t.paginationEnabled {
		return
	}
	pages := t.pageCount()
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	if page == t.pagCurrent {
		return
	}
	if !t.pagControlled {
		t.pagCurrent = page
	} else {
		// still update internal display for uncontrolled parent wiring via onChange
		t.pagCurrent = page
	}
	t.recomputeView(false)
	t.rebuildBodyOnly()
	t.fireChange(TableActionPaginate)
}

// ToggleSort cycles sort on column key (none → directions…).
func (t *Table) ToggleSort(columnKey string) {
	if t == nil {
		return
	}
	col := t.findLeaf(columnKey)
	if col == nil || col.Sorter == nil {
		return
	}
	dirs := col.SortDirections
	if len(dirs) == 0 {
		dirs = []TableSortOrder{TableSortAscend, TableSortDescend}
	}
	// cycle: none → dirs[0] → … → none
	next := TableSortNone
	if t.sortKey != columnKey || t.sortOrder == TableSortNone {
		next = dirs[0]
	} else {
		found := -1
		for i, d := range dirs {
			if d == t.sortOrder {
				found = i
				break
			}
		}
		if found >= 0 && found+1 < len(dirs) {
			next = dirs[found+1]
		} else {
			next = TableSortNone
		}
	}
	if next == TableSortNone {
		t.sortKey = ""
		t.sortOrder = TableSortNone
	} else {
		t.sortKey = columnKey
		t.sortOrder = next
	}
	// filter/sort change resets to page 1 (antd)
	if !t.pagControlled {
		t.pagCurrent = 1
	} else {
		t.pagCurrent = 1
	}
	t.recomputeView(false)
	t.rebuild()
	t.fireChange(TableActionSort)
}

// ConfirmFilter applies draft filters for a column and fires onChange(filter).
func (t *Table) ConfirmFilter(columnKey string) {
	if t == nil {
		return
	}
	t.filters[columnKey] = append([]string(nil), t.filterDraft[columnKey]...)
	t.filterOpen[columnKey] = false
	if !t.pagControlled {
		t.pagCurrent = 1
	} else {
		t.pagCurrent = 1
	}
	t.recomputeView(false)
	t.rebuild()
	t.fireChange(TableActionFilter)
}

// ResetFilter clears filters for a column.
func (t *Table) ResetFilter(columnKey string) {
	if t == nil {
		return
	}
	delete(t.filters, columnKey)
	delete(t.filterDraft, columnKey)
	t.filterOpen[columnKey] = false
	if !t.pagControlled {
		t.pagCurrent = 1
	} else {
		t.pagCurrent = 1
	}
	t.recomputeView(false)
	t.rebuild()
	t.fireChange(TableActionFilter)
}

// SetFilterDraft sets the in-progress filter values for a column (before confirm).
func (t *Table) SetFilterDraft(columnKey string, values []string) {
	if t == nil {
		return
	}
	t.filterDraft[columnKey] = append([]string(nil), values...)
}

// SetFilterQuery sets the filter-search text for a column.
func (t *Table) SetFilterQuery(columnKey, q string) {
	if t == nil {
		return
	}
	t.filterQuery[columnKey] = q
}

// SelectRow toggles/selects a row by key (respects radio/checkbox + disabled).
func (t *Table) SelectRow(key string) {
	if t == nil || t.rowSelection == nil || key == "" {
		return
	}
	rec := t.recordByKey(key)
	if rec != nil && t.rowSelection.GetCheckboxProps != nil {
		if p := t.rowSelection.GetCheckboxProps(rec); p.Disabled {
			return
		}
	}
	next := append([]string(nil), t.selectedKeys...)
	if t.rowSelection.Type == TableSelectionRadio {
		next = []string{key}
	} else {
		if idx := indexOfStr(next, key); idx >= 0 {
			next = append(next[:idx], next[idx+1:]...)
		} else {
			next = append(next, key)
		}
	}
	t.applySelection(next, true)
}

// SelectAllPage selects all changeable keys on the current page (checkbox).
func (t *Table) SelectAllPage() {
	if t == nil || t.rowSelection == nil || t.rowSelection.Type == TableSelectionRadio {
		return
	}
	keys := t.changeableKeys(t.viewPage)
	set := map[string]bool{}
	for _, k := range t.selectedKeys {
		set[k] = true
	}
	for _, k := range keys {
		set[k] = true
	}
	next := make([]string, 0, len(set))
	for k := range set {
		next = append(next, k)
	}
	sort.Strings(next)
	t.applySelection(next, true)
}

// ClearSelection clears selected keys.
func (t *Table) ClearSelection() {
	if t == nil {
		return
	}
	t.applySelection(nil, true)
}

// InvertSelection inverts selection on current page changeable keys.
func (t *Table) InvertSelection() {
	if t == nil || t.rowSelection == nil {
		return
	}
	page := t.changeableKeys(t.viewPage)
	set := map[string]bool{}
	for _, k := range t.selectedKeys {
		set[k] = true
	}
	for _, k := range page {
		if set[k] {
			delete(set, k)
		} else {
			set[k] = true
		}
	}
	next := make([]string, 0, len(set))
	for k := range set {
		next = append(next, k)
	}
	sort.Strings(next)
	t.applySelection(next, true)
}

// SelectAllData selects all changeable keys in the filtered dataset.
func (t *Table) SelectAllData() {
	if t == nil || t.rowSelection == nil {
		return
	}
	keys := t.changeableKeys(t.viewAll)
	t.applySelection(keys, true)
}

// RunSelection runs a built-in or custom selection item by key.
func (t *Table) RunSelection(itemKey string) {
	if t == nil || t.rowSelection == nil {
		return
	}
	switch itemKey {
	case TableSelectionAll:
		t.SelectAllData()
		return
	case TableSelectionInvert:
		t.InvertSelection()
		return
	case TableSelectionNone:
		t.ClearSelection()
		return
	}
	for _, it := range t.rowSelection.Selections {
		if it.Key == itemKey {
			changeable := t.changeableKeys(t.viewAll)
			if it.OnSelect != nil {
				it.OnSelect(changeable)
			}
			return
		}
	}
}

// ToggleExpand expands/collapses a row by key.
func (t *Table) ToggleExpand(key string) {
	if t == nil || t.expandable == nil || key == "" {
		return
	}
	rec := t.recordByKey(key)
	if rec == nil {
		return
	}
	if t.expandable.RowExpandable != nil && !t.expandable.RowExpandable(rec) {
		return
	}
	expanded := false
	next := append([]string(nil), t.expandedKeys...)
	if idx := indexOfStr(next, key); idx >= 0 {
		next = append(next[:idx], next[idx+1:]...)
		expanded = false
	} else {
		next = append(next, key)
		expanded = true
	}
	controlled := t.expandable.Controlled
	if !controlled {
		t.expandedKeys = next
		t.expandable.ExpandedRowKeys = append([]string(nil), next...)
		t.rebuild()
	}
	if t.expandable.OnExpand != nil {
		t.expandable.OnExpand(expanded, rec)
	}
	if controlled {
		// parent must SetExpandedRowKeys
		return
	}
}

// --- internals ---

func (t *Table) theme() *core.Theme {
	var n core.Node
	if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
}

func (t *Table) cellPad() (block, inline float64) {
	// Ant table prepareComponentToken uses padding/paddingSM/paddingXS seed values
	// (16/12/8). kit TokenPaddingSM is 8 (antd paddingXS) — do not map MD through it.
	switch t.size {
	case TableMiddle:
		return DefaultTableCellPadBlockMD, DefaultTableCellPadInlineMD
	case TableSmall:
		return DefaultTableCellPadBlockSM, DefaultTableCellPadInlineSM
	default: // large
		return DefaultTableCellPadBlockLG, DefaultTableCellPadInlineLG
	}
}

func (t *Table) fontSize() float64 {
	return t.theme().SizeOr(core.TokenFontSize, DefaultTableFontSize)
}

func (t *Table) selectionColWidth() float64 {
	if t.rowSelection != nil && t.rowSelection.ColumnWidth > 0 {
		return t.rowSelection.ColumnWidth
	}
	return t.theme().SizeOr(core.TokenControlHeight, DefaultTableSelectionColWidth)
}

func (t *Table) expandColWidth() float64 {
	if t.expandable != nil && t.expandable.ColumnWidth > 0 {
		return t.expandable.ColumnWidth
	}
	return DefaultTableExpandColWidth
}

func (t *Table) applyDefaultSortFromColumns() {
	for _, c := range flattenColumns(t.columns) {
		if c.DefaultSortOrder != TableSortNone && c.Sorter != nil {
			t.sortKey = colKey(c)
			t.sortOrder = c.DefaultSortOrder
			return
		}
	}
}

func (t *Table) rebuild() {
	if t == nil {
		return
	}
	th := t.theme()
	t.leafCols = flattenColumns(t.columns)
	t.recomputeView(true)
	t.headerCells = map[string]*primitive.Pressable{}
	t.filterDDs = map[string]*Dropdown{}
	t.rowNodes = nil
	t.selectAllCB = nil

	// colors
	t.headerBg = antHeaderFill(th)
	t.textColor = th.Color(core.TokenColorText)
	t.borderCol = th.Color(core.TokenColorBorderSecondary)
	if t.borderCol.A == 0 {
		t.borderCol = th.Color(core.TokenColorBorder)
	}
	if t.Style.hasBorder() {
		t.borderCol = t.Style.Border
	}
	bg := th.Color(core.TokenColorBgContainer)
	if t.Style.hasBG() {
		bg = t.Style.Background
	}
	radius := th.SizeOr(core.TokenBorderRadius, DefaultTableRadius)
	if t.Style.hasRadius() {
		radius = t.Style.Radius
	}
	lw := th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)

	// root column
	if t.Root == nil {
		t.Root = primitive.Column()
		t.Root.CrossAlign = core.CrossStretch
	} else {
		t.Root.ClearChildren()
	}
	t.Root.Base().Role = "table"
	if t.AriaLabel != "" {
		t.Root.Base().Label = t.AriaLabel
	} else {
		t.Root.Base().Label = "table"
	}

	// title
	if t.titleNode != nil || t.titleText != "" {
		var content core.Node
		if t.titleNode != nil {
			content = t.titleNode
		} else {
			lab := primitive.NewText(t.titleText)
			lab.FontSize = t.fontSize()
			lab.Face = t.Face
			lab.Color = th.Color(core.TokenColorText)
			content = lab
		}
		t.titleHost = primitive.NewDecorated(content)
		b, in := t.cellPad()
		t.titleHost.Padding = primitive.Symmetric(in, b)
		t.Root.AddChild(t.titleHost)
	} else {
		t.titleHost = nil
	}

	// table frame
	inner := primitive.Column()
	inner.CrossAlign = core.CrossStretch

	// header
	if t.showHeader {
		hdr := t.buildHeaderRow(th)
		t.headerHost = primitive.NewDecorated(hdr)
		t.headerHost.Background = t.headerBg
		t.headerHost.Base().Role = "rowgroup"
		// sticky when scroll.y
		if t.scroll.Y > 0 {
			sticky := primitive.NewSticky(t.headerHost)
			sticky.UseTop = true
			inner.AddChild(sticky)
		} else {
			inner.AddChild(t.headerHost)
		}
	} else {
		t.headerHost = nil
	}

	// body
	bodyContent := t.buildBody(th)
	t.bodyHost = primitive.NewDecorated(bodyContent)
	t.bodyHost.Background = bg
	t.bodyHost.Base().Role = "rowgroup"

	// loading overlay via Spin
	var bodyNode core.Node = t.bodyHost
	if t.loading {
		t.spin = NewSpin(t.bodyHost)
		t.spin.SetSpinning(true)
		t.spin.Theme = th
		bodyNode = t.spin.Node()
	} else {
		t.spin = nil
	}

	if t.scroll.Y > 0 {
		// fixed body height
		sv := primitive.NewScrollViewport(bodyNode)
		sv.Height = t.scroll.Y
		t.bodyScroll = sv
		inner.AddChild(sv)
	} else {
		t.bodyScroll = nil
		inner.AddChild(bodyNode)
	}

	// horizontal scroll wrapper when scroll.x
	var frameChild core.Node = inner
	if t.scroll.X > 0 {
		// ensure min content width via Decorated shell
		wide := primitive.NewDecorated(inner)
		wide.MinWidth = t.scroll.X
		hsv := primitive.NewScrollViewport(wide)
		hsv.SetAxis(false, true)
		t.hScroll = hsv
		frameChild = hsv
	} else {
		t.hScroll = nil
	}

	t.frame = primitive.NewDecorated(frameChild)
	t.frame.Background = bg
	t.frame.Radius = radius
	if t.bordered {
		t.frame.BorderWidth = lw
		t.frame.BorderColor = t.borderCol
	}
	t.Root.AddChild(t.frame)

	// footer
	if t.footerNode != nil || t.footerText != "" {
		var content core.Node
		if t.footerNode != nil {
			content = t.footerNode
		} else {
			lab := primitive.NewText(t.footerText)
			lab.FontSize = t.fontSize()
			lab.Face = t.Face
			lab.Color = th.Color(core.TokenColorText)
			content = lab
		}
		t.footerHost = primitive.NewDecorated(content)
		t.footerHost.Background = t.headerBg
		b, in := t.cellPad()
		t.footerHost.Padding = primitive.Symmetric(in, b)
		t.Root.AddChild(t.footerHost)
	} else {
		t.footerHost = nil
	}

	// pagination
	if t.paginationEnabled {
		if t.pager == nil {
			t.pager = NewPagination()
		}
		t.pager.SetFace(t.Face)
		t.pager.SetTheme(th)
		t.pager.SetTotal(len(t.viewAll))
		// drive display without marking controlled from outside
		t.pager.Current = t.pagCurrent
		t.pager.PageSize = t.pagPageSize
		t.pager.SetOnChange(func(page, pageSize int) {
			changed := page != t.pagCurrent || pageSize != t.pagPageSize
			t.pagCurrent = page
			t.pagPageSize = pageSize
			if changed {
				t.recomputeView(false)
				t.rebuild()
				t.fireChange(TableActionPaginate)
			}
		})
		// force pager rebuild for new total/current
		t.pager.SetTotal(len(t.viewAll))
		t.Root.AddChild(t.pager.Node())
	} else {
		t.pager = nil
	}

	t.Root.MarkNeedsLayout()
	t.Root.MarkNeedsPaint()
}

// rebuildBodyOnly is a lighter path after page change — full rebuild is fine.
func (t *Table) rebuildBodyOnly() {
	t.rebuild()
}

func (t *Table) recomputeView(clampPage bool) {
	// filter
	filtered := make([]TableRecord, 0, len(t.dataSource))
	for _, rec := range t.dataSource {
		if t.recordPassesFilters(rec) {
			filtered = append(filtered, rec)
		}
	}
	// sort
	if t.sortKey != "" && t.sortOrder != TableSortNone {
		col := t.findLeaf(t.sortKey)
		if col != nil && col.Sorter != nil {
			asc := t.sortOrder == TableSortAscend
			sort.SliceStable(filtered, func(i, j int) bool {
				cmp := col.Sorter(filtered[i], filtered[j])
				if asc {
					return cmp < 0
				}
				return cmp > 0
			})
		}
	}
	t.viewAll = filtered
	// page
	if !t.paginationEnabled {
		t.viewPage = filtered
		return
	}
	ps := t.pagPageSize
	if ps < 1 {
		ps = DefaultTablePageSize
		t.pagPageSize = ps
	}
	pages := t.pageCount()
	if clampPage || t.pagCurrent > pages {
		if t.pagCurrent > pages {
			t.pagCurrent = pages
		}
	}
	if t.pagCurrent < 1 {
		t.pagCurrent = 1
	}
	start := (t.pagCurrent - 1) * ps
	if start >= len(filtered) {
		t.viewPage = nil
		return
	}
	end := start + ps
	if end > len(filtered) {
		end = len(filtered)
	}
	t.viewPage = filtered[start:end]
}

func (t *Table) pageCount() int {
	ps := t.pagPageSize
	if ps < 1 {
		ps = DefaultTablePageSize
	}
	n := len(t.viewAll)
	if n == 0 {
		return 1
	}
	return (n + ps - 1) / ps
}

func (t *Table) recordPassesFilters(rec TableRecord) bool {
	for _, col := range t.leafCols {
		ck := colKey(col)
		vals := t.filters[ck]
		if len(vals) == 0 {
			continue
		}
		ok := false
		for _, v := range vals {
			if col.OnFilter != nil {
				if col.OnFilter(v, rec) {
					ok = true
					break
				}
			} else {
				// default: dataIndex string contains filter value
				cell := recordValue(rec, col)
				if strings.Contains(fmt.Sprint(cell), v) {
					ok = true
					break
				}
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

func (t *Table) buildHeaderRow(th *core.Theme) core.Node {
	// support one-level column groups: two header rows when any Children present
	hasGroup := false
	for _, c := range t.columns {
		if len(c.Children) > 0 {
			hasGroup = true
			break
		}
	}
	if hasGroup {
		return t.buildGroupedHeader(th)
	}
	return t.buildFlatHeader(th, t.leafCols)
}

func (t *Table) buildFlatHeader(th *core.Theme, cols []TableColumn) core.Node {
	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.Base().Role = "row"

	if t.expandable != nil && t.expandable.ExpandedRowRender != nil {
		row.AddChild(t.headerSpacer(t.expandColWidth(), th))
	}
	if t.rowSelection != nil {
		row.AddChild(t.buildSelectionHeaderCell(th))
	}
	for _, col := range cols {
		row.AddChild(t.buildHeaderCell(col, th))
	}
	return row
}

func (t *Table) buildGroupedHeader(th *core.Theme) core.Node {
	// two rows: group titles + leaf titles
	top := primitive.Row()
	top.CrossAlign = core.CrossCenter
	bot := primitive.Row()
	bot.CrossAlign = core.CrossCenter

	// leading expand/selection span both rows via empty top + content bottom
	leadW := 0.0
	if t.expandable != nil && t.expandable.ExpandedRowRender != nil {
		leadW += t.expandColWidth()
	}
	if t.rowSelection != nil {
		leadW += t.selectionColWidth()
	}
	if leadW > 0 {
		top.AddChild(t.headerSpacer(leadW, th))
		if t.expandable != nil && t.expandable.ExpandedRowRender != nil {
			bot.AddChild(t.headerSpacer(t.expandColWidth(), th))
		}
		if t.rowSelection != nil {
			bot.AddChild(t.buildSelectionHeaderCell(th))
		}
	}

	for _, c := range t.columns {
		if len(c.Children) == 0 {
			// leaf spanning both — put title on bottom, empty top with same width
			w := colWidth(c)
			top.AddChild(t.headerSpacer(w, th))
			bot.AddChild(t.buildHeaderCell(c, th))
			continue
		}
		// group
		groupW := 0.0
		for _, ch := range c.Children {
			groupW += colWidth(ch)
		}
		glab := primitive.NewText(c.Title)
		glab.FontSize = t.fontSize()
		glab.Face = t.Face
		glab.Color = th.Color(core.TokenColorTextSecondary)
		if glab.Color.A == 0 {
			glab.Color = t.textColor
		}
		gcell := primitive.NewDecorated(glab)
		b, in := t.cellPad()
		gcell.Padding = primitive.Symmetric(in, b)
		gcell.Background = t.headerBg
		gcell.Width = groupW
		if t.bordered {
			gcell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
			gcell.BorderColor = t.borderCol
		}
		top.AddChild(gcell)
		for _, ch := range c.Children {
			bot.AddChild(t.buildHeaderCell(ch, th))
		}
	}
	col := primitive.Column(top, bot)
	col.CrossAlign = core.CrossStretch
	return col
}

func (t *Table) headerSpacer(w float64, th *core.Theme) core.Node {
	cell := primitive.NewDecorated(primitive.NewBox())
	b, in := t.cellPad()
	cell.Padding = primitive.Symmetric(in, b)
	cell.Background = t.headerBg
	cell.Width = w
	if t.bordered {
		cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		cell.BorderColor = t.borderCol
	}
	return cell
}

func (t *Table) buildSelectionHeaderCell(th *core.Theme) core.Node {
	w := t.selectionColWidth()
	var content core.Node
	if t.rowSelection.Type == TableSelectionCheckbox && !t.rowSelection.HideSelectAll {
		cb := NewCheckbox("")
		cb.SetTheme(th)
		cb.SetFace(t.Face)
		cb.SetAriaLabel("Select all")
		// indeterminate / checked from page selection
		pageKeys := t.changeableKeys(t.viewPage)
		selCount := 0
		set := map[string]bool{}
		for _, k := range t.selectedKeys {
			set[k] = true
		}
		for _, k := range pageKeys {
			if set[k] {
				selCount++
			}
		}
		if len(pageKeys) > 0 && selCount == len(pageKeys) {
			cb.SetChecked(true)
		} else if selCount > 0 {
			cb.SetIndeterminate(true)
		}
		cb.SetOnChange(func(checked bool) {
			if checked {
				t.SelectAllPage()
			} else {
				// deselect page keys only
				pageSet := map[string]bool{}
				for _, k := range pageKeys {
					pageSet[k] = true
				}
				next := make([]string, 0, len(t.selectedKeys))
				for _, k := range t.selectedKeys {
					if !pageSet[k] {
						next = append(next, k)
					}
				}
				t.applySelection(next, true)
			}
		})
		t.selectAllCB = cb
		content = cb.Node()
		// selections menu
		if len(t.rowSelection.Selections) > 0 {
			items := make([]MenuItem, 0, len(t.rowSelection.Selections))
			for _, it := range t.rowSelection.Selections {
				text := it.Text
				if text == "" {
					switch it.Key {
					case TableSelectionAll:
						text = "Select all data"
					case TableSelectionInvert:
						text = "Invert current page"
					case TableSelectionNone:
						text = "Clear all"
					default:
						text = it.Key
					}
				}
				key := it.Key
				items = append(items, MenuItem{Key: key, Label: text})
			}
			dd := NewDropdown("▾", items...)
			dd.SetTriggerModes(DropdownTriggerClick)
			dd.SetTheme(th)
			dd.SetFace(t.Face)
			dd.SetOnMenuClick(func(k string) { t.RunSelection(k) })
			row := primitive.Row(cb.Node(), dd.Node())
			row.Gap = 4
			row.CrossAlign = core.CrossCenter
			content = row
		}
	} else {
		content = primitive.NewBox()
	}
	cell := primitive.NewDecorated(content)
	b, in := t.cellPad()
	cell.Padding = primitive.Symmetric(in, b)
	cell.Background = t.headerBg
	cell.Width = w
	if t.bordered {
		cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		cell.BorderColor = t.borderCol
	}
	return cell
}

func (t *Table) buildHeaderCell(col TableColumn, th *core.Theme) core.Node {
	ck := colKey(col)
	b, in := t.cellPad()
	titleColor := th.Color(core.TokenColorTextSecondary)
	if titleColor.A == 0 {
		titleColor = th.Color(core.TokenColorText)
	}

	lab := primitive.NewText(col.Title)
	lab.FontSize = t.fontSize()
	lab.Face = t.Face
	lab.Color = titleColor

	parts := []core.Node{lab}
	// sorter icon
	if col.Sorter != nil {
		icon := "↕"
		if t.sortKey == ck {
			switch t.sortOrder {
			case TableSortAscend:
				icon = "↑"
			case TableSortDescend:
				icon = "↓"
			}
		}
		ic := primitive.NewText(icon)
		ic.FontSize = t.fontSize()
		ic.Face = t.Face
		ic.Color = th.Color(core.TokenColorTextSecondary)
		parts = append(parts, ic)
	}
	// filter trigger
	if len(col.Filters) > 0 {
		active := len(t.filters[ck]) > 0
		ft := "▾"
		if active {
			ft = "▼"
		}
		fic := primitive.NewText(ft)
		fic.FontSize = t.fontSize()
		fic.Face = t.Face
		if active {
			fic.Color = th.Color(core.TokenColorPrimary)
		} else {
			fic.Color = th.Color(core.TokenColorTextSecondary)
		}
		// filter as dropdown: option keys + OK/Reset
		dd := NewDropdown("")
		dd.SetTriggerNode(fic)
		dd.SetTriggerModes(DropdownTriggerClick)
		dd.SetTheme(th)
		dd.SetFace(t.Face)
		dd.SetAriaLabel(col.Title + " filter")
		items := t.filterMenuItems(col)
		extra := []MenuItem{
			{Key: "__ok__", Label: t.Locale.FilterConfirm},
			{Key: "__reset__", Label: t.Locale.FilterReset},
		}
		dd.SetItems(append(items, extra...)...)
		dd.SetOnMenuClick(func(v string) {
			switch v {
			case "__ok__":
				if _, ok := t.filterDraft[ck]; !ok {
					t.filterDraft[ck] = append([]string(nil), t.filters[ck]...)
				}
				t.ConfirmFilter(ck)
			case "__reset__":
				t.ResetFilter(ck)
			default:
				draft := append([]string(nil), t.filterDraft[ck]...)
				if len(draft) == 0 {
					draft = append([]string(nil), t.filters[ck]...)
				}
				if idx := indexOfStr(draft, v); idx >= 0 {
					draft = append(draft[:idx], draft[idx+1:]...)
				} else {
					draft = append(draft, v)
				}
				t.filterDraft[ck] = draft
			}
		})
		t.filterDDs[ck] = dd
		parts = append(parts, dd.Node())
	}

	row := primitive.Row(parts...)
	row.Gap = 4
	row.CrossAlign = core.CrossCenter

	press := primitive.NewPressable(row)
	press.Base().Role = "columnheader"
	press.Base().Label = col.Title
	if col.Sorter != nil {
		key := ck
		press.Click = func() { t.ToggleSort(key) }
	}
	t.headerCells[ck] = press

	cell := primitive.NewDecorated(press)
	cell.Padding = primitive.Symmetric(in, b)
	cell.Background = t.headerBg
	if t.sortKey == ck && t.sortOrder != TableSortNone {
		// sorted header tint
		cell.Background = antHeaderFill(th)
	}
	if t.bordered {
		cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		cell.BorderColor = t.borderCol
	}
	if col.Width > 0 {
		cell.Width = col.Width
		return cell
	}
	fl := col.Flex
	if fl <= 0 {
		fl = 1
	}
	return primitive.NewFlexible(fl, cell)
}

func (t *Table) filterMenuItems(col TableColumn) []MenuItem {
	q := strings.ToLower(t.filterQuery[colKey(col)])
	var out []MenuItem
	var walk func(filters []TableFilter, depth int)
	walk = func(filters []TableFilter, depth int) {
		for _, f := range filters {
			if q != "" && !strings.Contains(strings.ToLower(f.Text), q) &&
				!strings.Contains(strings.ToLower(f.Value), q) {
				// still walk children
				if len(f.Children) > 0 {
					walk(f.Children, depth+1)
				}
				continue
			}
			label := f.Text
			if depth > 0 {
				label = strings.Repeat("· ", depth) + label
			}
			out = append(out, MenuItem{Key: f.Value, Label: label})
			if col.FilterMode == TableFilterTree && len(f.Children) > 0 {
				walk(f.Children, depth+1)
			} else if len(f.Children) > 0 {
				// menu mode still lists nested as flat with depth prefix
				walk(f.Children, depth+1)
			}
		}
	}
	walk(col.Filters, 0)
	return out
}

// FixedColumns reports columns with Fixed != None (tests / P0 fixed pin).
func (t *Table) FixedColumns() (start, end []TableColumn) {
	for _, c := range t.leafCols {
		switch c.Fixed {
		case TableFixedStart:
			start = append(start, c)
		case TableFixedEnd:
			end = append(end, c)
		}
	}
	return start, end
}

// HasFixedHeader reports scroll.y fixed-header mode.
func (t *Table) HasFixedHeader() bool { return t != nil && t.scroll.Y > 0 }

// HasHorizontalScroll reports scroll.x mode.
func (t *Table) HasHorizontalScroll() bool { return t != nil && t.scroll.X > 0 }

// BodyScroll returns the vertical body ScrollViewport when scroll.y > 0.
func (t *Table) BodyScroll() *primitive.ScrollViewport { return t.bodyScroll }

// HScroll returns the horizontal ScrollViewport when scroll.x > 0.
func (t *Table) HScroll() *primitive.ScrollViewport { return t.hScroll }

func (t *Table) buildBody(th *core.Theme) core.Node {
	if len(t.viewPage) == 0 {
		empty := NewEmpty()
		empty.SetFace(t.Face)
		empty.SetTheme(th)
		empty.SetDescription(t.Locale.EmptyText)
		empty.SetImage(EmptyImageSimple)
		t.emptyNode = empty
		wrap := primitive.NewDecorated(empty.Node())
		b, in := t.cellPad()
		wrap.Padding = primitive.Symmetric(in, b*2)
		return wrap
	}
	t.emptyNode = nil

	col := primitive.Column()
	col.CrossAlign = core.CrossStretch
	for i, rec := range t.viewPage {
		col.AddChild(t.buildDataRow(rec, i, th))
		// expanded row
		if t.expandable != nil && t.expandable.ExpandedRowRender != nil {
			key := t.keyOf(rec)
			if indexOfStr(t.expandedKeys, key) >= 0 {
				col.AddChild(t.buildExpandedRow(rec, i, th))
			}
		}
	}
	return col
}

func (t *Table) buildDataRow(rec TableRecord, index int, th *core.Theme) core.Node {
	row := primitive.Row()
	row.CrossAlign = core.CrossCenter
	row.Base().Role = "row"
	key := t.keyOf(rec)

	// expand cell
	if t.expandable != nil && t.expandable.ExpandedRowRender != nil {
		can := true
		if t.expandable.RowExpandable != nil {
			can = t.expandable.RowExpandable(rec)
		}
		var expNode core.Node
		if can {
			expanded := indexOfStr(t.expandedKeys, key) >= 0
			symbol := "▶"
			if expanded {
				symbol = "▼"
			}
			lab := primitive.NewText(symbol)
			lab.FontSize = t.fontSize()
			lab.Face = t.Face
			lab.Color = t.textColor
			pr := primitive.NewPressable(lab)
			pr.Base().Label = "expand row"
			k := key
			pr.Click = func() { t.ToggleExpand(k) }
			expNode = pr
		} else {
			expNode = primitive.NewBox()
		}
		cell := primitive.NewDecorated(expNode)
		b, in := t.cellPad()
		cell.Padding = primitive.Symmetric(in, b)
		cell.Width = t.expandColWidth()
		if t.bordered {
			cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
			cell.BorderColor = t.borderCol
		}
		row.AddChild(cell)
	}

	// selection cell
	if t.rowSelection != nil {
		row.AddChild(t.buildSelectionCell(rec, key, th))
	}

	// data cells
	for _, col := range t.leafCols {
		row.AddChild(t.buildDataCell(col, rec, index, th))
	}

	bg := render.RGBA{}
	selected := indexOfStr(t.selectedKeys, key) >= 0
	if selected {
		bg = antItemSelectedFill(th)
	}
	press := primitive.NewPressable(row)
	press.Color = bg
	if t.rowHoverable {
		press.ColorHovered = antItemHoverFill(th)
	}
	// click row selects when selection enabled (checkbox toggle / radio select)
	if t.rowSelection != nil {
		k := key
		press.Click = func() { t.SelectRow(k) }
	}
	press.Base().Role = "row"
	if key != "" {
		press.Base().Label = key
	}
	t.rowNodes = append(t.rowNodes, press)

	// bottom border
	wrap := primitive.NewDecorated(press)
	if !t.bordered {
		wrap.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		// only bottom via full border is coarse; accept full cell borders when bordered
		// For non-bordered, draw bottom split using BorderWidth on wrap bottom only —
		// Decorated has uniform border; use background strip approximation:
		wrap.BorderWidth = 0
	}
	// bottom line via a thin decorated border bottom is not available; skip.
	return wrap
}

func (t *Table) buildSelectionCell(rec TableRecord, key string, th *core.Theme) core.Node {
	props := TableCheckboxProps{}
	if t.rowSelection.GetCheckboxProps != nil {
		props = t.rowSelection.GetCheckboxProps(rec)
	}
	var content core.Node
	if t.rowSelection.Type == TableSelectionRadio {
		r := NewRadio("")
		r.SetTheme(th)
		r.SetFace(t.Face)
		if props.Name != "" {
			r.SetAriaLabel(props.Name)
		} else {
			r.SetAriaLabel("select row")
		}
		r.SetDisabled(props.Disabled)
		r.SetChecked(indexOfStr(t.selectedKeys, key) >= 0)
		k := key
		r.SetOnChange(func(checked bool) {
			if checked {
				t.SelectRow(k)
			}
		})
		content = r.Node()
	} else {
		cb := NewCheckbox("")
		cb.SetTheme(th)
		cb.SetFace(t.Face)
		if props.Name != "" {
			cb.SetAriaLabel(props.Name)
		} else {
			cb.SetAriaLabel("select row")
		}
		cb.SetDisabled(props.Disabled)
		cb.SetChecked(indexOfStr(t.selectedKeys, key) >= 0)
		k := key
		cb.SetOnChange(func(checked bool) {
			// SelectRow toggles; align to desired checked
			has := indexOfStr(t.selectedKeys, k) >= 0
			if checked != has {
				t.SelectRow(k)
			}
		})
		content = cb.Node()
	}
	cell := primitive.NewDecorated(content)
	b, in := t.cellPad()
	cell.Padding = primitive.Symmetric(in, b)
	cell.Width = t.selectionColWidth()
	if t.bordered {
		cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		cell.BorderColor = t.borderCol
	}
	return cell
}

func (t *Table) buildDataCell(col TableColumn, rec TableRecord, index int, th *core.Theme) core.Node {
	val := recordValue(rec, col)
	var content core.Node
	if col.Render != nil {
		content = col.Render(val, rec, index)
		if content == nil {
			content = primitive.NewBox()
		}
	} else {
		lab := primitive.NewText(formatCell(val))
		lab.FontSize = t.fontSize()
		lab.Face = t.Face
		if t.Style.hasText() {
			lab.Color = t.Style.Text
		} else {
			lab.Color = t.textColor
		}
		content = lab
	}
	cell := primitive.NewDecorated(content)
	b, in := t.cellPad()
	cell.Padding = primitive.Symmetric(in, b)
	cell.Base().Role = "cell"
	if t.bordered {
		cell.BorderWidth = th.SizeOr(core.TokenLineWidth, DefaultTableLineWidth)
		cell.BorderColor = t.borderCol
	}
	// fixed column marker (visual weight via background on fixed sections handled at row level)
	if col.Width > 0 {
		cell.Width = col.Width
		return cell
	}
	fl := col.Flex
	if fl <= 0 {
		fl = 1
	}
	return primitive.NewFlexible(fl, cell)
}

func (t *Table) buildExpandedRow(rec TableRecord, index int, th *core.Theme) core.Node {
	var content core.Node
	if t.expandable.ExpandedRowRender != nil {
		content = t.expandable.ExpandedRowRender(rec, index)
	}
	if content == nil {
		content = primitive.NewBox()
	}
	// indent under expand column
	pad := primitive.NewDecorated(content)
	b, in := t.cellPad()
	pad.Padding = primitive.Symmetric(in, b)
	pad.Background = antHeaderFill(th) // rowExpandedBg ≈ fillAlter
	// leading spacer for expand+selection columns
	lead := t.expandColWidth()
	if t.rowSelection != nil {
		lead += t.selectionColWidth()
	}
	spacer := primitive.NewBox()
	spacer.Width = lead
	row := primitive.Row(spacer, primitive.NewFlexible(1, pad))
	row.CrossAlign = core.CrossStart
	row.Base().Role = "row"
	return row
}

func (t *Table) applySelection(keys []string, notify bool) {
	controlled := t.rowSelection != nil && t.rowSelection.Controlled
	if !controlled {
		t.selectedKeys = append([]string(nil), keys...)
		if t.rowSelection != nil {
			t.rowSelection.SelectedRowKeys = append([]string(nil), keys...)
		}
		t.rebuild()
	} else {
		// keep internal for read; parent SetSelectedRowKeys applies
		t.selectedKeys = append([]string(nil), keys...)
	}
	if notify && t.rowSelection != nil && t.rowSelection.OnChange != nil {
		rows := make([]TableRecord, 0, len(keys))
		for _, k := range keys {
			if r := t.recordByKey(k); r != nil {
				rows = append(rows, r)
			}
		}
		t.rowSelection.OnChange(append([]string(nil), keys...), rows)
	}
}

func (t *Table) fireChange(action TableChangeAction) {
	if t.onChange == nil {
		return
	}
	pag := t.PaginationState()
	filters := t.Filters()
	sorter := TableSorterResult{
		ColumnKey: t.sortKey,
		Field:     t.sortKey,
		Order:     t.sortOrder,
	}
	if col := t.findLeaf(t.sortKey); col != nil {
		if col.DataIndex != "" {
			sorter.Field = col.DataIndex
		}
	}
	extra := TableChangeExtra{
		Action:            action,
		CurrentDataSource: append([]TableRecord(nil), t.viewAll...),
	}
	t.onChange(pag, filters, sorter, extra)
}

func (t *Table) keyOf(rec TableRecord) string {
	if rec == nil {
		return ""
	}
	if t.rowKeyFunc != nil {
		return t.rowKeyFunc(rec)
	}
	field := t.rowKey
	if field == "" {
		field = DefaultTableRowKey
	}
	if v, ok := rec[field]; ok && v != nil {
		return fmt.Sprint(v)
	}
	// unstable fallback — index not known here
	return ""
}

func (t *Table) recordByKey(key string) TableRecord {
	for _, r := range t.dataSource {
		if t.keyOf(r) == key {
			return r
		}
	}
	// also search view (same refs usually)
	for _, r := range t.viewAll {
		if t.keyOf(r) == key {
			return r
		}
	}
	return nil
}

func (t *Table) changeableKeys(rows []TableRecord) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		k := t.keyOf(r)
		if k == "" {
			continue
		}
		if t.rowSelection != nil && t.rowSelection.GetCheckboxProps != nil {
			if p := t.rowSelection.GetCheckboxProps(r); p.Disabled {
				continue
			}
		}
		out = append(out, k)
	}
	return out
}

func (t *Table) findLeaf(key string) *TableColumn {
	for i := range t.leafCols {
		if colKey(t.leafCols[i]) == key {
			return &t.leafCols[i]
		}
	}
	return nil
}

func flattenColumns(cols []TableColumn) []TableColumn {
	var out []TableColumn
	var walk func([]TableColumn)
	walk = func(list []TableColumn) {
		for _, c := range list {
			if len(c.Children) > 0 {
				walk(c.Children)
				continue
			}
			out = append(out, c)
		}
	}
	walk(cols)
	return out
}

func colKey(c TableColumn) string {
	if c.Key != "" {
		return c.Key
	}
	if c.DataIndex != "" {
		return c.DataIndex
	}
	return c.Title
}

func colWidth(c TableColumn) float64 {
	if c.Width > 0 {
		return c.Width
	}
	// rough default for group width calc
	return 120
}

func recordValue(rec TableRecord, col TableColumn) any {
	if rec == nil {
		return nil
	}
	idx := col.DataIndex
	if idx == "" {
		idx = col.Key
	}
	if idx == "" {
		return nil
	}
	return rec[idx]
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case bool:
		if x {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func indexOfStr(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}
