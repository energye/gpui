//go:build linux && !nogpu

package main

import (
	"fmt"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerTable() {
	// Table — docs/antd/table.md §6.8 P0
	// https://ant.design/components/table
	// demos: basic / jsx / row-selection / row-selection-and-operation /
	//        row-selection-custom / head / filter-in-tree / filter-search
	//
	// P1 not shown: multiple-sorter, controlled filters, custom filterDropdown,
	// ajax remote, sticky depth, nested/edit/drag, semantic classNames,
	// ConfigProvider, 官网逐像素.

	face, th := c.face, c.theme
	status := c.status

	wire := func(tb *kit.Table) *kit.Table {
		tb.SetFace(face)
		if th != nil {
			tb.SetTheme(th)
		}
		c.trackTicker(tb)
		return tb
	}

	people := []kit.TableRecord{
		{"key": "1", "name": "John Brown", "age": 32, "address": "New York No. 1 Lake Park", "tags": "nice developer"},
		{"key": "2", "name": "Jim Green", "age": 42, "address": "London No. 1 Lake Park", "tags": "kawaii"},
		{"key": "3", "name": "Joe Black", "age": 32, "address": "Sydney No. 1 Lake Park", "tags": "cool teacher"},
		{"key": "4", "name": "Jim Red", "age": 32, "address": "London No. 2 Lake Park", "tags": "loser"},
	}

	// ---------- basic.tsx ----------
	basicCols := []kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name",
			Render: func(v any, _ kit.TableRecord, _ int) core.Node {
				return primitive.NewText(fmt.Sprint(v))
			}},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address"},
		{Key: "tags", Title: "Tags", DataIndex: "tags",
			Render: func(v any, _ kit.TableRecord, _ int) core.Node {
				parts := strings.Fields(fmt.Sprint(v))
				row := primitive.Row()
				row.Gap = 4
				for _, p := range parts {
					tg := kit.NewTag(strings.ToUpper(p))
					tg.SetFace(face)
					row.AddChild(tg.Node())
				}
				return row
			}},
		{Key: "action", Title: "Action", Width: 160,
			Render: func(_ any, rec kit.TableRecord, _ int) core.Node {
				return primitive.NewText("Invite " + fmt.Sprint(rec["name"]))
			}},
	}
	basic := wire(kit.NewTableWith(basicCols, people[:3]))
	basic.SetOnChange(func(pag kit.TablePaginationState, _ map[string][]string, _ kit.TableSorterResult, extra kit.TableChangeExtra) {
		if status != nil {
			*status = fmt.Sprintf("Table basic · %s page=%d", extra.Action, pag.Current)
		}
	})
	secBasic := demoSection(face, th, "基本用法",
		"basic.tsx：columns + dataSource + render（Tag / Action）。",
		basic.Node())

	// ---------- jsx.tsx (ColumnGroup) ----------
	jsxCols := []kit.TableColumn{
		{Title: "Name", Children: []kit.TableColumn{
			{Key: "firstName", Title: "First Name", DataIndex: "firstName"},
			{Key: "lastName", Title: "Last Name", DataIndex: "lastName"},
		}},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address"},
	}
	jsxData := []kit.TableRecord{
		{"key": "1", "firstName": "John", "lastName": "Brown", "age": 32, "address": "New York No. 1 Lake Park"},
		{"key": "2", "firstName": "Jim", "lastName": "Green", "age": 42, "address": "London No. 1 Lake Park"},
		{"key": "3", "firstName": "Joe", "lastName": "Black", "age": 32, "address": "Sydney No. 1 Lake Park"},
	}
	jsx := wire(kit.NewTableWith(jsxCols, jsxData))
	secJSX := demoSection(face, th, "JSX 风格的 API",
		"jsx.tsx：ColumnGroup（Children）分组表头。",
		jsx.Node())

	// ---------- row-selection.tsx ----------
	rsData := append([]kit.TableRecord{}, people...)
	rsData = append(rsData, kit.TableRecord{"key": "9", "name": "Disabled User", "age": 99, "address": "Sydney No. 1 Lake Park"})
	rsCols := []kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name"},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address"},
	}
	rs := wire(kit.NewTableWith(rsCols, rsData))
	rs.SetPaginationEnabled(false)
	rs.SetRowSelection(&kit.TableRowSelection{
		Type: kit.TableSelectionCheckbox,
		GetCheckboxProps: func(rec kit.TableRecord) kit.TableCheckboxProps {
			return kit.TableCheckboxProps{
				Disabled: fmt.Sprint(rec["name"]) == "Disabled User",
				Name:     fmt.Sprint(rec["name"]),
			}
		},
		OnChange: func(keys []string, _ []kit.TableRecord) {
			if status != nil {
				*status = fmt.Sprintf("rowSelection keys=%v", keys)
			}
		},
	})
	// radio twin
	rsRadio := wire(kit.NewTableWith(rsCols, rsData[:3]))
	rsRadio.SetPaginationEnabled(false)
	rsRadio.SetRowSelection(&kit.TableRowSelection{
		Type: kit.TableSelectionRadio,
		OnChange: func(keys []string, _ []kit.TableRecord) {
			if status != nil {
				*status = fmt.Sprintf("radio keys=%v", keys)
			}
		},
	})
	rsRow := primitive.Column(rs.Node(), rsRadio.Node())
	rsRow.Gap = 16
	secRS := demoSection(face, th, "可选择",
		"row-selection.tsx：checkbox + radio；Disabled User 不可选。",
		rsRow)

	// ---------- row-selection-and-operation.tsx ----------
	opData := make([]kit.TableRecord, 0, 46)
	for i := 0; i < 46; i++ {
		opData = append(opData, kit.TableRecord{
			"key": fmt.Sprintf("%d", i), "name": fmt.Sprintf("Edward King %d", i),
			"age": 32, "address": fmt.Sprintf("London, Park Lane no. %d", i),
		})
	}
	op := wire(kit.NewTableWith(rsCols, opData))
	op.SetRowSelection(&kit.TableRowSelection{
		OnChange: func(keys []string, _ []kit.TableRecord) {
			if status != nil {
				*status = fmt.Sprintf("Selected %d items", len(keys))
			}
		},
	})
	reload := c.trackBtn(kit.NewButton("Reload"))
	reload.SetType(kit.ButtonPrimary)
	reload.SetOnClick(func() {
		op.ClearSelection()
		op.SetLoading(true)
		// gallery: clear loading on next tick path — instant demo
		op.SetLoading(false)
		if status != nil {
			*status = "Reload · selection cleared"
		}
	})
	selLab := kit.NewText("select rows then Reload")
	selLab.SetFace(face)
	opTop := primitive.Row(reload.Node(), selLab.Node())
	opTop.Gap = 12
	opTop.CrossAlign = core.CrossCenter
	opCol := primitive.Column(opTop, op.Node())
	opCol.Gap = 12
	secOp := demoSection(face, th, "选择和操作",
		"row-selection-and-operation.tsx：分页多选 + 操作栏。",
		opCol)

	// ---------- row-selection-custom.tsx ----------
	custom := wire(kit.NewTableWith(rsCols, opData[:20]))
	custom.SetPaginationEnabled(false)
	custom.SetRowSelection(&kit.TableRowSelection{
		Selections: []kit.TableSelectionItem{
			{Key: kit.TableSelectionAll},
			{Key: kit.TableSelectionInvert},
			{Key: kit.TableSelectionNone},
			{Key: "odd", Text: "Select Odd Row", OnSelect: func(changeable []string) {
				var odd []string
				for i, k := range changeable {
					if i%2 == 0 {
						odd = append(odd, k)
					}
				}
				custom.SetSelectedRowKeys(odd)
			}},
			{Key: "even", Text: "Select Even Row", OnSelect: func(changeable []string) {
				var even []string
				for i, k := range changeable {
					if i%2 != 0 {
						even = append(even, k)
					}
				}
				custom.SetSelectedRowKeys(even)
			}},
		},
		OnChange: func(keys []string, _ []kit.TableRecord) {
			if status != nil {
				*status = fmt.Sprintf("custom selection n=%d", len(keys))
			}
		},
	})
	secCustom := demoSection(face, th, "自定义选择项",
		"row-selection-custom.tsx：SELECTION_ALL / INVERT / NONE + odd/even。",
		custom.Node())

	// ---------- head.tsx ----------
	headCols := []kit.TableColumn{
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
			Sorter: func(a, b kit.TableRecord) int {
				return len(fmt.Sprint(a["name"])) - len(fmt.Sprint(b["name"]))
			},
			SortDirections: []kit.TableSortOrder{kit.TableSortDescend},
		},
		{
			Key: "age", Title: "Age", DataIndex: "age", Width: 80,
			DefaultSortOrder: kit.TableSortDescend,
			Sorter: func(a, b kit.TableRecord) int {
				ai, _ := a["age"].(int)
				bi, _ := b["age"].(int)
				return ai - bi
			},
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
	head := wire(kit.NewTableWith(headCols, people))
	head.SetPaginationEnabled(false)
	head.SetOnChange(func(_ kit.TablePaginationState, filters map[string][]string, sorter kit.TableSorterResult, extra kit.TableChangeExtra) {
		if status != nil {
			*status = fmt.Sprintf("head · %s sorter=%s filters=%v", extra.Action, sorter.ColumnKey, filters)
		}
	})
	secHead := demoSection(face, th, "筛选和排序",
		"head.tsx：表头 sorter + filters（含子菜单）+ onChange。",
		head.Node())

	// ---------- filter-in-tree.tsx ----------
	treeCols := []kit.TableColumn{
		{
			Key: "name", Title: "Name", DataIndex: "name", Width: 200,
			Filters: []kit.TableFilter{
				{Text: "Joe", Value: "Joe"},
				{Text: "Category 1", Value: "Category 1", Children: []kit.TableFilter{
					{Text: "Yellow", Value: "Yellow"},
					{Text: "Pink", Value: "Pink"},
				}},
				{Text: "Category 2", Value: "Category 2", Children: []kit.TableFilter{
					{Text: "Green", Value: "Green"},
					{Text: "Black", Value: "Black"},
				}},
			},
			FilterMode:   kit.TableFilterTree,
			FilterSearch: true,
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.Contains(fmt.Sprint(record["name"]), value)
			},
		},
		{
			Key: "age", Title: "Age", DataIndex: "age", Width: 80,
			Sorter: func(a, b kit.TableRecord) int {
				ai, _ := a["age"].(int)
				bi, _ := b["age"].(int)
				return ai - bi
			},
		},
		{
			Key: "address", Title: "Address", DataIndex: "address",
			Filters: []kit.TableFilter{
				{Text: "London", Value: "London"},
				{Text: "New York", Value: "New York"},
			},
			FilterSearch: true,
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.HasPrefix(fmt.Sprint(record["address"]), value)
			},
		},
	}
	treeF := wire(kit.NewTableWith(treeCols, people))
	treeF.SetPaginationEnabled(false)
	secTree := demoSection(face, th, "树型筛选菜单",
		"filter-in-tree.tsx：filterMode=tree + filterSearch。",
		treeF.Node())

	// ---------- filter-search.tsx ----------
	searchCols := []kit.TableColumn{
		{
			Key: "name", Title: "Name", DataIndex: "name",
			Filters: []kit.TableFilter{
				{Text: "Joe", Value: "Joe"},
				{Text: "Category 1", Value: "Category 1"},
				{Text: "Category 2", Value: "Category 2"},
			},
			FilterMode:   kit.TableFilterTree,
			FilterSearch: true,
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.HasPrefix(fmt.Sprint(record["name"]), value)
			},
		},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80,
			Sorter: func(a, b kit.TableRecord) int {
				ai, _ := a["age"].(int)
				bi, _ := b["age"].(int)
				return ai - bi
			}},
		{Key: "address", Title: "Address", DataIndex: "address",
			Filters: []kit.TableFilter{
				{Text: "London", Value: "London"},
				{Text: "New York", Value: "New York"},
			},
			FilterSearch: true,
			OnFilter: func(value string, record kit.TableRecord) bool {
				return strings.HasPrefix(fmt.Sprint(record["address"]), value)
			}},
	}
	search := wire(kit.NewTableWith(searchCols, people))
	search.SetPaginationEnabled(false)
	secSearch := demoSection(face, th, "自定义筛选的搜索",
		"filter-search.tsx：filterSearch 过滤筛选项。",
		search.Node())

	// ---------- size / bordered / loading / empty / expand / scroll (P0 extras) ----------
	sizeRow := primitive.Row()
	sizeRow.Gap = 16
	for _, sz := range []struct {
		label string
		size  kit.TableSize
	}{
		{"large", kit.TableLarge},
		{"middle", kit.TableMiddle},
		{"small", kit.TableSmall},
	} {
		t := wire(kit.NewTableWith(rsCols, people[:2]))
		t.SetPaginationEnabled(false)
		t.SetSize(sz.size)
		t.SetBordered(true)
		box := primitive.Column(primitive.NewText(sz.label), t.Node())
		box.Gap = 8
		sizeRow.AddChild(box)
	}
	loading := wire(kit.NewTableWith(rsCols, people[:2]))
	loading.SetPaginationEnabled(false)
	loading.SetLoading(true)
	loading.SetTitle("Loading table")
	empty := wire(kit.NewTableWith(rsCols, nil))
	empty.SetPaginationEnabled(false)
	empty.SetLocale(kit.TableLocale{EmptyText: "No data"})
	expand := wire(kit.NewTableWith(rsCols, people))
	expand.SetPaginationEnabled(false)
	expand.SetExpandable(&kit.TableExpandable{
		ExpandedRowRender: func(rec kit.TableRecord, _ int) core.Node {
			return primitive.NewText("My name is " + fmt.Sprint(rec["name"]) + ", living in " + fmt.Sprint(rec["address"]))
		},
		RowExpandable: func(rec kit.TableRecord) bool {
			return fmt.Sprint(rec["name"]) != "Jim Red"
		},
	})
	scroll := wire(kit.NewTableWith([]kit.TableColumn{
		{Key: "name", Title: "Name", DataIndex: "name", Width: 120, Fixed: kit.TableFixedStart},
		{Key: "age", Title: "Age", DataIndex: "age", Width: 80},
		{Key: "address", Title: "Address", DataIndex: "address", Width: 240},
		{Key: "tags", Title: "Tags", DataIndex: "tags", Width: 160},
		{Key: "op", Title: "Action", Width: 100, Fixed: kit.TableFixedEnd,
			Render: func(any, kit.TableRecord, int) core.Node { return primitive.NewText("action") }},
	}, people))
	scroll.SetPaginationEnabled(false)
	scroll.SetScroll(kit.TableScroll{X: 900, Y: 160})

	extraCol := primitive.Column(
		demoSection(face, th, "尺寸 / 边框", "size large|middle|small + bordered。", sizeRow),
		demoSection(face, th, "Loading", "loading Spin 遮罩，表头保留。", loading.Node()),
		demoSection(face, th, "Empty", "dataSource=[] → Empty。", empty.Node()),
		demoSection(face, th, "展开行", "expandable.expandedRowRender + rowExpandable。", expand.Node()),
		demoSection(face, th, "固定表头 / 固定列", "scroll.y + scroll.x + column.fixed。", scroll.Node()),
	)
	extraCol.Gap = 24
	extraCol.CrossAlign = core.CrossStretch

	// Lifecycle (#9)
	life := wire(kit.NewTableWith(rsCols, people[:2]))
	life.SetPaginationEnabled(false)
	life.SetSize(kit.TableSmall)
	life.SetBordered(true)
	secLife := demoSection(face, th, "Lifecycle (#9)",
		"structureChange：Columns/DataSource → Size → Bordered；ensureBuilt 懒构建。",
		life.Node())

	// Skin (#6)
	baseSkin := th.Skin
	th.Skin = core.Override(baseSkin, kit.TypeTable, func(pc *core.PaintContext, n core.Node) {
		if d, ok := n.(*primitive.Decorated); ok && d != nil {
			if d.Base().Key == "table-skin-demo" {
				d.BorderWidth = 2
				d.BorderColor = render.Hex("#1677FF")
			}
			if p := baseSkin.Painter(kit.TypeTable); p != nil {
				p(pc, d)
				return
			}
			primitive.PaintDecorated(pc, d)
			return
		}
		if p := baseSkin.Painter(kit.TypeTable); p != nil {
			p(pc, n)
			return
		}
		if base, ok := n.(interface{ DefaultPaintChildren(*core.PaintContext) }); ok {
			base.DefaultPaintChildren(pc)
		}
	})
	skinT := wire(kit.NewTableWith(rsCols, people[:2]))
	skinT.SetPaginationEnabled(false)
	skinNode := skinT.Node()
	if fr := skinT.Frame(); fr != nil {
		fr.Base().Key = "table-skin-demo"
	}
	secSkin := demoSection(face, th, "Skin painter (#6)",
		"frame.SkinType=kit.Table。Key=table-skin-demo → 蓝边框 Override。",
		skinNode)

	page := demoPage(face, "Table 表格",
		"Ant Design Table P0 — docs/antd/table.md §6。数据展示：分页 / 选择 / 排序 / 筛选 / 展开 / 滚动。Also #9 lifecycle + #6 Skin.",
		secBasic, secJSX, secRS, secOp, secCustom, secHead, secTree, secSearch, extraCol, secLife, secSkin)
	c.addPage("table", "Table", page)
}
