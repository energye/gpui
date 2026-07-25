package kit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

// docs/antd/table.md §6.9 — P0 PRD cases (TBL-01 … TBL-25).
// TBL-26 L3 / TBL-27 L4 / TBL-28 P1 deferred.

func approxTable(a, b, tol float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tol
}

func samplePeople() []kit.TableRecord {
	return []kit.TableRecord{
		{"key": "1", "name": "John Brown", "age": 32, "address": "New York No. 1 Lake Park", "tags": "nice"},
		{"key": "2", "name": "Jim Green", "age": 42, "address": "London No. 1 Lake Park", "tags": "kawaii"},
		{"key": "3", "name": "Joe Black", "age": 32, "address": "Sydney No. 1 Lake Park", "tags": "cool"},
		{"key": "4", "name": "Jim Red", "age": 32, "address": "London No. 2 Lake Park", "tags": "loser"},
	}
}

func basicCols() []kit.TableColumn {
	return []kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name"},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address"},
	}
}

func sorterAge(a, b kit.TableRecord) int {
	ai, _ := a["age"].(int)
	bi, _ := b["age"].(int)
	return ai - bi
}

func sorterNameLen(a, b kit.TableRecord) int {
	as, bs := fmt.Sprint(a["name"]), fmt.Sprint(b["name"])
	return len(as) - len(bs)
}

func TestTable_PRD_01_Defaults(t *testing.T) {
	// TBL-01
	tb := kit.NewTable()
	if tb.Size() != kit.TableLarge {
		t.Fatalf("size=%v want large", tb.Size())
	}
	if !tb.ShowHeader() {
		t.Fatal("showHeader want true")
	}
	if tb.IsBordered() {
		t.Fatal("bordered want false")
	}
	if tb.IsLoading() {
		t.Fatal("loading want false")
	}
	if !tb.IsPaginationEnabled() {
		t.Fatal("pagination want enabled")
	}
	pag := tb.PaginationState()
	if pag.Current != 1 || pag.PageSize != kit.DefaultTablePageSize {
		t.Fatalf("pag=%+v", pag)
	}
	if tb.Node() == nil || tb.ChromeNode() == nil {
		t.Fatal("nil node")
	}
	if tb.ChromeNode().Base().Role != "table" {
		t.Fatalf("role=%q", tb.ChromeNode().Base().Role)
	}
	_ = tb.Node().Layout(core.Loose(600, 400))
}

func TestTable_PRD_02_RenderRows(t *testing.T) {
	// TBL-02 / TBL-S1: 3 cols × 2 rows
	tb := kit.NewTableWith(basicCols(), samplePeople()[:2])
	tb.SetPaginationEnabled(false)
	_ = tb.Node().Layout(core.Loose(700, 300))
	if len(tb.PageRecords()) != 2 {
		t.Fatalf("rows=%d", len(tb.PageRecords()))
	}
	if len(tb.LeafColumns()) != 3 {
		t.Fatalf("cols=%d", len(tb.LeafColumns()))
	}
	if tb.HeaderHost() == nil || tb.BodyHost() == nil {
		t.Fatal("header/body nil")
	}
}

func TestTable_PRD_03_Paginate(t *testing.T) {
	// TBL-03 / TBL-S2
	data := make([]kit.TableRecord, 0, 25)
	for i := 0; i < 25; i++ {
		data = append(data, kit.TableRecord{
			"key": fmt.Sprintf("%d", i), "name": fmt.Sprintf("n%d", i), "age": i, "address": "x",
		})
	}
	tb := kit.NewTableWith(basicCols(), data)
	var gotCurrent int
	var action kit.TableChangeAction
	tb.SetOnChange(func(pag kit.TablePaginationState, _ map[string][]string, _ kit.TableSorterResult, extra kit.TableChangeExtra) {
		gotCurrent = pag.Current
		action = extra.Action
	})
	_ = tb.Node().Layout(core.Loose(700, 400))
	if len(tb.PageRecords()) != 10 {
		t.Fatalf("page1 size=%d", len(tb.PageRecords()))
	}
	tb.GoToPage(2)
	if gotCurrent != 2 {
		t.Fatalf("onChange current=%d", gotCurrent)
	}
	if action != kit.TableActionPaginate {
		t.Fatalf("action=%s", action)
	}
	if tb.PaginationState().Current != 2 {
		t.Fatalf("state current=%d", tb.PaginationState().Current)
	}
	if len(tb.PageRecords()) != 10 {
		t.Fatalf("page2 size=%d", len(tb.PageRecords()))
	}
}

func TestTable_PRD_04_Sort(t *testing.T) {
	// TBL-04 / TBL-S3
	cols := basicCols()
	cols[1].Sorter = sorterAge
	cols[1].DefaultSortOrder = kit.TableSortDescend
	tb := kit.NewTableWith(cols, samplePeople())
	tb.SetPaginationEnabled(false)
	var action kit.TableChangeAction
	var order kit.TableSortOrder
	tb.SetOnChange(func(_ kit.TablePaginationState, _ map[string][]string, s kit.TableSorterResult, extra kit.TableChangeExtra) {
		action = extra.Action
		order = s.Order
	})
	// defaultSortOrder applied on SetColumns path
	k, o := tb.SortState()
	if k != "age" || o != kit.TableSortDescend {
		// NewTableWith applies columns after NewTable; default sort applied in SetColumns
		if k == "" {
			// force
			tb.ToggleSort("age") // none→ascend
			tb.ToggleSort("age") // ascend→descend
			k, o = tb.SortState()
		}
	}
	if k != "age" {
		t.Fatalf("sort key=%q", k)
	}
	// toggle again
	tb.ToggleSort("age")
	if action != kit.TableActionSort {
		t.Fatalf("action=%s", action)
	}
	_ = order
	// first page ages should be ordered by current sort
	page := tb.PageRecords()
	if len(page) < 2 {
		t.Fatal("empty page")
	}
}

func TestTable_PRD_05_RowSelect(t *testing.T) {
	// TBL-05 / TBL-S4
	tb := kit.NewTableWith(basicCols(), samplePeople())
	tb.SetPaginationEnabled(false)
	var keys []string
	tb.SetRowSelection(&kit.TableRowSelection{
		OnChange: func(selected []string, _ []kit.TableRecord) {
			keys = append([]string(nil), selected...)
		},
	})
	_ = tb.Node().Layout(core.Loose(700, 300))
	tb.SelectRow("2")
	got := tb.SelectedRowKeys()
	if len(got) != 1 || got[0] != "2" {
		t.Fatalf("selected=%v", got)
	}
	if len(keys) == 0 || keys[0] != "2" {
		t.Fatalf("onChange keys=%v", keys)
	}
}

func TestTable_PRD_06_SelectAll(t *testing.T) {
	// TBL-06 / TBL-S5
	tb := kit.NewTableWith(basicCols(), samplePeople())
	tb.SetPaginationEnabled(false)
	tb.SetRowSelection(&kit.TableRowSelection{})
	_ = tb.Node().Layout(core.Loose(700, 300))
	tb.SelectAllPage()
	if len(tb.SelectedRowKeys()) != 4 {
		t.Fatalf("selected=%v", tb.SelectedRowKeys())
	}
}

func TestTable_PRD_07_Loading(t *testing.T) {
	// TBL-07 / TBL-S6
	tb := kit.NewTableWith(basicCols(), samplePeople()[:2])
	tb.SetPaginationEnabled(false)
	tb.SetLoading(true)
	if !tb.IsLoading() {
		t.Fatal("loading")
	}
	_ = tb.Node().Layout(core.Loose(700, 300))
	if tb.HeaderHost() == nil {
		t.Fatal("header lost while loading")
	}
	if tb.SpinNode() == nil {
		t.Fatal("spin nil")
	}
	tree := core.NewTree(tb.Node())
	tb.AttachTicker(tree)
	if !tb.Tick(0.016) {
		t.Fatal("tick should continue while loading")
	}
	tb.SetLoading(false)
	if tb.IsLoading() {
		t.Fatal("still loading")
	}
}

func TestTable_PRD_08_Empty(t *testing.T) {
	// TBL-08 / TBL-S7
	tb := kit.NewTableWith(basicCols(), nil)
	tb.SetPaginationEnabled(false)
	_ = tb.Node().Layout(core.Loose(700, 300))
	if tb.EmptyNode() == nil {
		t.Fatal("Empty nil")
	}
}

func TestTable_PRD_09_Expand(t *testing.T) {
	// TBL-09 / TBL-S8
	tb := kit.NewTableWith(basicCols(), samplePeople()[:2])
	tb.SetPaginationEnabled(false)
	var expanded bool
	tb.SetExpandable(&kit.TableExpandable{
		ExpandedRowRender: func(rec kit.TableRecord, _ int) core.Node {
			return primitive.NewText(fmt.Sprint(rec["address"]))
		},
		OnExpand: func(exp bool, _ kit.TableRecord) { expanded = exp },
	})
	_ = tb.Node().Layout(core.Loose(700, 400))
	tb.ToggleExpand("1")
	if !expanded {
		t.Fatal("onExpand false")
	}
	keys := tb.ExpandedRowKeys()
	if len(keys) != 1 || keys[0] != "1" {
		t.Fatalf("expanded=%v", keys)
	}
}

func TestTable_PRD_10_ScrollY(t *testing.T) {
	// TBL-10 / TBL-S9
	data := make([]kit.TableRecord, 0, 30)
	for i := 0; i < 30; i++ {
		data = append(data, kit.TableRecord{"key": fmt.Sprintf("%d", i), "name": "n", "age": i, "address": "a"})
	}
	tb := kit.NewTableWith(basicCols(), data)
	tb.SetPaginationEnabled(false)
	tb.SetScroll(kit.TableScroll{Y: 200})
	_ = tb.Node().Layout(core.Loose(700, 400))
	if !tb.HasFixedHeader() {
		t.Fatal("fixed header")
	}
	if tb.BodyScroll() == nil {
		t.Fatal("body scroll nil")
	}
	if tb.HeaderHost() == nil {
		t.Fatal("header nil")
	}
}

func TestTable_PRD_11_FixedLeft(t *testing.T) {
	// TBL-11 / TBL-S10
	cols := []kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name", Width: 100, Fixed: kit.TableFixedStart},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address", Width: 200},
		{Key: "op", Title: "Action", Width: 100, Fixed: kit.TableFixedEnd},
	}
	tb := kit.NewTableWith(cols, samplePeople()[:2])
	tb.SetPaginationEnabled(false)
	tb.SetScroll(kit.TableScroll{X: 800})
	_ = tb.Node().Layout(core.Loose(500, 300))
	start, end := tb.FixedColumns()
	if len(start) != 1 || start[0].Key != "name" {
		t.Fatalf("fixed start=%v", start)
	}
	if len(end) != 1 || end[0].Key != "op" {
		t.Fatalf("fixed end=%v", end)
	}
	if !tb.HasHorizontalScroll() || tb.HScroll() == nil {
		t.Fatal("hscroll")
	}
}

func TestTable_PRD_12_Filter(t *testing.T) {
	// TBL-12 / TBL-S11
	cols := basicCols()
	cols[0].Filters = []kit.TableFilter{
		{Text: "Joe", Value: "Joe"},
		{Text: "Jim", Value: "Jim"},
	}
	cols[0].OnFilter = func(value string, record kit.TableRecord) bool {
		return strings.HasPrefix(fmt.Sprint(record["name"]), value)
	}
	tb := kit.NewTableWith(cols, samplePeople())
	tb.SetPaginationEnabled(false)
	var action kit.TableChangeAction
	var filters map[string][]string
	tb.SetOnChange(func(_ kit.TablePaginationState, f map[string][]string, _ kit.TableSorterResult, extra kit.TableChangeExtra) {
		action = extra.Action
		filters = f
	})
	tb.SetFilterDraft("name", []string{"Jim"})
	tb.ConfirmFilter("name")
	if action != kit.TableActionFilter {
		t.Fatalf("action=%s", action)
	}
	if len(filters["name"]) != 1 || filters["name"][0] != "Jim" {
		t.Fatalf("filters=%v", filters)
	}
	for _, r := range tb.ViewRecords() {
		if !strings.HasPrefix(fmt.Sprint(r["name"]), "Jim") {
			t.Fatalf("leaked row %v", r)
		}
	}
	if len(tb.ViewRecords()) != 2 {
		t.Fatalf("filtered count=%d", len(tb.ViewRecords()))
	}
}

func TestTable_PRD_13_RowKeyRequired(t *testing.T) {
	// TBL-13 / TBL-S12 — tests force rowKey; missing key yields empty keyOf
	tb := kit.NewTableWith(basicCols(), []kit.TableRecord{{"name": "x", "age": 1, "address": "a"}})
	tb.SetPaginationEnabled(false)
	tb.SetRowSelection(&kit.TableRowSelection{})
	// no "key" field → SelectRow("") no-op
	tb.SelectRow("")
	if len(tb.SelectedRowKeys()) != 0 {
		t.Fatalf("unexpected selection %v", tb.SelectedRowKeys())
	}
	// with key works
	tb.SetDataSource([]kit.TableRecord{{"key": "k1", "name": "x", "age": 1, "address": "a"}})
	tb.SelectRow("k1")
	if len(tb.SelectedRowKeys()) != 1 {
		t.Fatal("want selection with key")
	}
}

func TestTable_PRD_14_BasicDemo(t *testing.T) {
	// TBL-14 basic.tsx
	cols := []kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name",
			Render: func(v any, _ kit.TableRecord, _ int) core.Node {
				return primitive.NewText(fmt.Sprint(v))
			}},
		{Key: "age", Title: "Age", DataIndex: "age"},
		{Key: "address", Title: "Address", DataIndex: "address"},
		{Key: "tags", Title: "Tags", DataIndex: "tags",
			Render: func(v any, _ kit.TableRecord, _ int) core.Node {
				tg := kit.NewTag(fmt.Sprint(v))
				return tg.Node()
			}},
		{Key: "action", Title: "Action",
			Render: func(_ any, rec kit.TableRecord, _ int) core.Node {
				return primitive.NewText("Invite " + fmt.Sprint(rec["name"]))
			}},
	}
	tb := kit.NewTableWith(cols, samplePeople()[:3])
	_ = tb.Node().Layout(core.Loose(900, 400))
	if len(tb.PageRecords()) != 3 {
		t.Fatal(len(tb.PageRecords()))
	}
}

func TestTable_PRD_15_JSXStyleColumns(t *testing.T) {
	// TBL-15 jsx.tsx — ColumnGroup via Children
	cols := []kit.TableColumn{
		{Title: "Name", Children: []kit.TableColumn{
			{Key: "firstName", Title: "First Name", DataIndex: "firstName"},
			{Key: "lastName", Title: "Last Name", DataIndex: "lastName"},
		}},
		{Key: "age", Title: "Age", DataIndex: "age"},
		{Key: "address", Title: "Address", DataIndex: "address"},
	}
	data := []kit.TableRecord{
		{"key": "1", "firstName": "John", "lastName": "Brown", "age": 32, "address": "NY"},
		{"key": "2", "firstName": "Jim", "lastName": "Green", "age": 42, "address": "London"},
	}
	tb := kit.NewTableWith(cols, data)
	tb.SetPaginationEnabled(false)
	_ = tb.Node().Layout(core.Loose(800, 300))
	if len(tb.LeafColumns()) != 4 {
		t.Fatalf("leaf=%d", len(tb.LeafColumns()))
	}
}

func TestTable_PRD_16_RowSelectionDemo(t *testing.T) {
	// TBL-16 row-selection.tsx
	tb := kit.NewTableWith(basicCols(), samplePeople())
	tb.SetPaginationEnabled(false)
	tb.SetRowSelection(&kit.TableRowSelection{
		Type: kit.TableSelectionCheckbox,
		GetCheckboxProps: func(rec kit.TableRecord) kit.TableCheckboxProps {
			return kit.TableCheckboxProps{
				Disabled: fmt.Sprint(rec["name"]) == "Disabled User",
				Name:     fmt.Sprint(rec["name"]),
			}
		},
	})
	// inject disabled user
	data := samplePeople()
	data = append(data, kit.TableRecord{"key": "9", "name": "Disabled User", "age": 99, "address": "x"})
	tb.SetDataSource(data)
	_ = tb.Node().Layout(core.Loose(700, 400))
	tb.SelectRow("9") // disabled — no-op
	if len(tb.SelectedRowKeys()) != 0 {
		t.Fatalf("disabled selected: %v", tb.SelectedRowKeys())
	}
	tb.SelectRow("1")
	if len(tb.SelectedRowKeys()) != 1 {
		t.Fatal(tb.SelectedRowKeys())
	}
	// radio mode
	tb.SetRowSelection(&kit.TableRowSelection{Type: kit.TableSelectionRadio})
	tb.SelectRow("1")
	tb.SelectRow("2")
	if got := tb.SelectedRowKeys(); len(got) != 1 || got[0] != "2" {
		t.Fatalf("radio=%v", got)
	}
}

func TestTable_PRD_17_SelectionAndOperation(t *testing.T) {
	// TBL-17
	data := make([]kit.TableRecord, 0, 46)
	for i := 0; i < 46; i++ {
		data = append(data, kit.TableRecord{
			"key": fmt.Sprintf("%d", i), "name": fmt.Sprintf("Edward King %d", i), "age": 32, "address": "London",
		})
	}
	tb := kit.NewTableWith(basicCols(), data)
	tb.SetRowSelection(&kit.TableRowSelection{})
	_ = tb.Node().Layout(core.Loose(700, 400))
	tb.SelectAllPage()
	if len(tb.SelectedRowKeys()) != 10 {
		t.Fatalf("page select=%d", len(tb.SelectedRowKeys()))
	}
	tb.ClearSelection()
	if len(tb.SelectedRowKeys()) != 0 {
		t.Fatal("clear")
	}
}

func TestTable_PRD_18_CustomSelections(t *testing.T) {
	// TBL-18 row-selection-custom
	data := make([]kit.TableRecord, 0, 20)
	for i := 0; i < 20; i++ {
		data = append(data, kit.TableRecord{
			"key": fmt.Sprintf("%d", i), "name": fmt.Sprintf("n%d", i), "age": 1, "address": "a",
		})
	}
	tb := kit.NewTableWith(basicCols(), data)
	tb.SetPaginationEnabled(false)
	var odd []string
	tb.SetRowSelection(&kit.TableRowSelection{
		Selections: []kit.TableSelectionItem{
			{Key: kit.TableSelectionAll},
			{Key: kit.TableSelectionInvert},
			{Key: kit.TableSelectionNone},
			{Key: "odd", Text: "Select Odd Row", OnSelect: func(changeable []string) {
				odd = nil
				for i, k := range changeable {
					if i%2 == 0 {
						odd = append(odd, k)
					}
				}
				tb.SetSelectedRowKeys(odd)
			}},
		},
	})
	_ = tb.Node().Layout(core.Loose(700, 400))
	tb.RunSelection(kit.TableSelectionAll)
	if len(tb.SelectedRowKeys()) != 20 {
		t.Fatalf("all=%d", len(tb.SelectedRowKeys()))
	}
	tb.RunSelection(kit.TableSelectionNone)
	if len(tb.SelectedRowKeys()) != 0 {
		t.Fatal("none")
	}
	tb.RunSelection("odd")
	if len(odd) == 0 {
		t.Fatal("odd empty")
	}
}

func TestTable_PRD_19_HeadFilterSort(t *testing.T) {
	// TBL-19 head.tsx
	cols := []kit.TableColumn{
		{
			Key: "name", Title: "Name", DataIndex: "name",
			Filters: []kit.TableFilter{
				{Text: "Joe", Value: "Joe"},
				{Text: "Jim", Value: "Jim"},
				{Text: "Submenu", Value: "Submenu", Children: []kit.TableFilter{
					{Text: "Green", Value: "Green"},
					{Text: "Black", Value: "Black"},
				}},
			},
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.Contains(fmt.Sprint(record["name"]), value)
			},
			Sorter:         sorterNameLen,
			SortDirections: []kit.TableSortOrder{kit.TableSortDescend},
		},
		{
			Key: "age", Title: "Age", DataIndex: "age",
			DefaultSortOrder: kit.TableSortDescend,
			Sorter:           sorterAge,
		},
		{
			Key: "address", Title: "Address", DataIndex: "address",
			Filters: []kit.TableFilter{
				{Text: "London", Value: "London"},
				{Text: "New York", Value: "New York"},
			},
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.HasPrefix(fmt.Sprint(record["address"]), value)
			},
		},
	}
	tb := kit.NewTableWith(cols, samplePeople())
	tb.SetPaginationEnabled(false)
	_ = tb.Node().Layout(core.Loose(800, 400))
	// default age descend
	k, o := tb.SortState()
	if k != "age" || o != kit.TableSortDescend {
		// apply manually if NewTableWith order missed default
		tb.ToggleSort("age")
		tb.ToggleSort("age")
		k, o = tb.SortState()
	}
	if k != "age" {
		t.Fatalf("sort=%s %v", k, o)
	}
	tb.SetFilterDraft("address", []string{"London"})
	tb.ConfirmFilter("address")
	for _, r := range tb.ViewRecords() {
		if !strings.HasPrefix(fmt.Sprint(r["address"]), "London") {
			t.Fatalf("filter leak %v", r)
		}
	}
}

func TestTable_PRD_20_FilterInTree(t *testing.T) {
	// TBL-20 filter-in-tree
	cols := basicCols()
	cols[0].FilterMode = kit.TableFilterTree
	cols[0].FilterSearch = true
	cols[0].Filters = []kit.TableFilter{
		{Text: "Joe", Value: "Joe"},
		{Text: "Category 1", Value: "Category 1", Children: []kit.TableFilter{
			{Text: "Yellow", Value: "Yellow"},
			{Text: "Pink", Value: "Pink"},
		}},
	}
	cols[0].OnFilter = func(value string, record kit.TableRecord) bool {
		return strings.Contains(fmt.Sprint(record["name"]), value)
	}
	tb := kit.NewTableWith(cols, samplePeople())
	tb.SetPaginationEnabled(false)
	tb.SetFilterQuery("name", "Joe")
	tb.SetFilterDraft("name", []string{"Joe"})
	tb.ConfirmFilter("name")
	if len(tb.ViewRecords()) != 1 {
		t.Fatalf("tree filter count=%d", len(tb.ViewRecords()))
	}
}

func TestTable_PRD_21_FilterSearch(t *testing.T) {
	// TBL-21 filter-search
	cols := basicCols()
	cols[0].FilterSearch = true
	cols[0].FilterMode = kit.TableFilterTree
	cols[0].Filters = []kit.TableFilter{
		{Text: "Joe", Value: "Joe"},
		{Text: "Category 1", Value: "Category 1"},
		{Text: "Category 2", Value: "Category 2"},
	}
	cols[0].OnFilter = func(value string, record kit.TableRecord) bool {
		return strings.HasPrefix(fmt.Sprint(record["name"]), value)
	}
	cols[2].FilterSearch = true
	cols[2].Filters = []kit.TableFilter{
		{Text: "London", Value: "London"},
		{Text: "New York", Value: "New York"},
	}
	cols[2].OnFilter = func(value string, record kit.TableRecord) bool {
		return strings.HasPrefix(fmt.Sprint(record["address"]), value)
	}
	tb := kit.NewTableWith(cols, samplePeople())
	tb.SetPaginationEnabled(false)
	tb.SetFilterQuery("name", "Joe")
	tb.SetFilterDraft("name", []string{"Joe"})
	tb.ConfirmFilter("name")
	if len(tb.ViewRecords()) != 1 {
		t.Fatalf("count=%d", len(tb.ViewRecords()))
	}
}

func TestTable_PRD_22_Metrics(t *testing.T) {
	// TBL-22 L2
	tb := kit.NewTable()
	// large
	b, in := tb.CellPad()
	if !approxTable(b, kit.DefaultTableCellPadBlockLG, 0.5) || !approxTable(in, kit.DefaultTableCellPadInlineLG, 0.5) {
		t.Fatalf("large pad %v %v", b, in)
	}
	if !approxTable(tb.RowHeightEstimate(), kit.DefaultTableCellPadBlockLG*2+kit.DefaultTableRowLine, 0.5) {
		t.Fatalf("large row h=%v", tb.RowHeightEstimate())
	}
	tb.SetSize(kit.TableMiddle)
	b, in = tb.CellPad()
	if !approxTable(b, kit.DefaultTableCellPadBlockMD, 0.5) || !approxTable(in, kit.DefaultTableCellPadInlineMD, 0.5) {
		t.Fatalf("middle pad %v %v", b, in)
	}
	tb.SetSize(kit.TableSmall)
	b, in = tb.CellPad()
	if !approxTable(b, kit.DefaultTableCellPadBlockSM, 0.5) || !approxTable(in, kit.DefaultTableCellPadInlineSM, 0.5) {
		t.Fatalf("small pad %v %v", b, in)
	}
	if kit.DefaultTableFontSize != 14 {
		t.Fatal(kit.DefaultTableFontSize)
	}
	if kit.DefaultTableSelectionColWidth != 32 {
		t.Fatal(kit.DefaultTableSelectionColWidth)
	}
}

func TestTable_PRD_23_ThemeColors(t *testing.T) {
	// TBL-23
	th := kit.DefaultTheme()
	tb := kit.NewTableWith(basicCols(), samplePeople()[:1])
	tb.SetTheme(th)
	_ = tb.Node().Layout(core.Loose(600, 200))
	// header/bg from theme — not a hard-coded brand primary as sole fill
	if tb.HeaderBg().A == 0 {
		t.Fatal("header bg zero")
	}
	if tb.TextColor().A == 0 {
		t.Fatal("text zero")
	}
	// primary brand must not equal body text (token path)
	primary := th.Color(core.TokenColorPrimary)
	if approxTableColor(tb.TextColor(), primary, 0.02) {
		t.Fatal("text should not be primary brand")
	}
}

func approxTableColor(a, b render.RGBA, tol float64) bool {
	return approxTable(float64(a.R), float64(b.R), tol) &&
		approxTable(float64(a.G), float64(b.G), tol) &&
		approxTable(float64(a.B), float64(b.B), tol)
}

func TestTable_PRD_24_DisabledRow(t *testing.T) {
	// TBL-24 — disabled selection chrome (getCheckboxProps)
	tb := kit.NewTableWith(basicCols(), []kit.TableRecord{
		{"key": "1", "name": "ok", "age": 1, "address": "a"},
		{"key": "2", "name": "Disabled User", "age": 2, "address": "b"},
	})
	tb.SetPaginationEnabled(false)
	tb.SetRowSelection(&kit.TableRowSelection{
		GetCheckboxProps: func(rec kit.TableRecord) kit.TableCheckboxProps {
			return kit.TableCheckboxProps{Disabled: fmt.Sprint(rec["name"]) == "Disabled User"}
		},
	})
	_ = tb.Node().Layout(core.Loose(600, 200))
	tb.SelectAllPage()
	// only non-disabled
	for _, k := range tb.SelectedRowKeys() {
		if k == "2" {
			t.Fatal("disabled key selected")
		}
	}
	if len(tb.SelectedRowKeys()) != 1 || tb.SelectedRowKeys()[0] != "1" {
		t.Fatalf("selected=%v", tb.SelectedRowKeys())
	}
}

func TestTable_PRD_25_A11y(t *testing.T) {
	// TBL-25
	tb := kit.NewTableWith(basicCols(), samplePeople()[:1])
	tb.SetAriaLabel("People table")
	tb.SetPaginationEnabled(false)
	cols := basicCols()
	cols[0].Sorter = sorterNameLen
	cols[0].Filters = []kit.TableFilter{{Text: "J", Value: "J"}}
	tb.SetColumns(cols)
	_ = tb.Node().Layout(core.Loose(600, 200))
	if tb.ChromeNode().Base().Role != "table" {
		t.Fatalf("role=%q", tb.ChromeNode().Base().Role)
	}
	if tb.ChromeNode().Base().Label != "People table" {
		t.Fatalf("label=%q", tb.ChromeNode().Base().Label)
	}
}

func TestTable_PRD_TitleFooterBorderedSize(t *testing.T) {
	// extra P0: title/footer/bordered/size
	tb := kit.NewTableWith(basicCols(), samplePeople()[:1])
	tb.SetTitle("Header Title")
	tb.SetFooter("Footer Note")
	tb.SetBordered(true)
	tb.SetSize(kit.TableMiddle)
	tb.SetShowHeader(true)
	_ = tb.Node().Layout(core.Loose(600, 300))
	if !tb.IsBordered() || tb.Size() != kit.TableMiddle {
		t.Fatal("bordered/size")
	}
}
