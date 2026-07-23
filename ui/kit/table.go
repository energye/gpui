package kit

import (
	"fmt"
	"sort"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TableColumn describes one table column.
type TableColumn struct {
	Key   string
	Title string
	Width float64 // 0 → flex
	Flex  float64 // default 1 when Width==0
}

// Table is a simple data table with fixed header + virtual body (B4 base).
//
// Sticky policy (UI_FOUNDATION_P0 C3 decision B): header is a fixed Column
// sibling above VirtualList — not scroll-container sticky. primitive.Sticky
// wraps the header for future in-scroll sticky paint; hit/layout remain fixed.
type Table struct {
	Root       *primitive.Flex
	header     *primitive.Flex
	body       *primitive.VirtualList
	Columns    []TableColumn
	Data       []map[string]string
	RowHeight  float64
	SortKey    string
	SortAsc    bool
	Face       text.Face
	Theme      *core.Theme
	Selection  *core.SelectionModel
	OnRowClick func(index int, row map[string]string)
}

// NewTable creates a table.
func NewTable(columns []TableColumn, data []map[string]string) *Table {
	t := &Table{
		Columns: columns, Data: data, RowHeight: 47, // Ant table middle row
		Selection: core.NewSelectionModel(core.SelectSingle),
	}
	t.rebuild()
	return t
}

// Node returns the root.
func (t *Table) Node() core.Node {
	if t.Root == nil {
		t.rebuild()
	}
	return t.Root
}

// SetData replaces rows and rebuilds the body.
func (t *Table) SetData(data []map[string]string) {
	t.Data = data
	if t.SortKey != "" {
		t.applySort()
	}
	if t.body != nil {
		t.body.ItemCount = len(t.Data)
		t.body.MarkNeedsLayout()
	}
}

// SetSort sorts rows by column key ascending or descending and rebuilds.
func (t *Table) SetSort(key string, asc bool) {
	t.SortKey = key
	t.SortAsc = asc
	t.applySort()
	if t.body != nil {
		t.body.ItemCount = len(t.Data)
		t.body.MarkNeedsLayout()
	} else {
		t.rebuild()
	}
}

func (t *Table) applySort() {
	if t.SortKey == "" || len(t.Data) == 0 {
		return
	}
	key := t.SortKey
	asc := t.SortAsc
	sort.SliceStable(t.Data, func(i, j int) bool {
		a, b := t.Data[i][key], t.Data[j][key]
		if asc {
			return a < b
		}
		return a > b
	})
}

func (t *Table) theme() *core.Theme {
	var n core.Node
	if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
}

func (t *Table) rebuild() {
	th := t.theme()
	// header
	t.header = primitive.Row()
	t.header.CrossAlign = core.CrossCenter
	for _, col := range t.Columns {
		lab := primitive.NewText(col.Title)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = t.Face
		lab.Color = th.Color(core.TokenColorTextSecondary)
		cell := primitive.NewDecorated(lab)
		cell.Padding = primitive.Symmetric(8, 12) // Ant table header cell
		cell.Background = antHeaderFill(th)
		if col.Width > 0 {
			cell.Width = col.Width
		} else {
			// flex via Flexible
			fl := col.Flex
			if fl <= 0 {
				fl = 1
			}
			t.header.AddChild(primitive.NewFlexible(fl, cell))
			continue
		}
		t.header.AddChild(cell)
	}
	// sticky header wrapper
	sticky := primitive.NewSticky(t.header)
	sticky.UseTop = true

	t.body = primitive.NewVirtualList(t.RowHeight, t.rowAt)
	t.body.ItemCount = len(t.Data)
	t.body.Height = 240

	// scroll viewport containing sticky + virtual is complex; simplify:
	// column: header (fixed) + virtual body
	frame := primitive.NewDecorated(primitive.Column(sticky, t.body))
	frame.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	frame.BorderColor = th.Color(core.TokenColorBorder)
	frame.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	frame.Background = th.Color(core.TokenColorBgContainer)

	if t.Root == nil {
		t.Root = primitive.Column(frame)
	} else {
		t.Root.ClearChildren()
		t.Root.AddChild(frame)
	}
	t.Root.CrossAlign = core.CrossStretch
	t.Root.MarkNeedsLayout()
	t.Root.MarkNeedsPaint()
}

func (t *Table) rowAt(i int) core.Node {
	th := t.theme()
	if i < 0 || i >= len(t.Data) {
		return primitive.NewBox()
	}
	row := t.Data[i]
	r := primitive.Row()
	r.CrossAlign = core.CrossCenter
	for _, col := range t.Columns {
		val := row[col.Key]
		lab := primitive.NewText(val)
		lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
		lab.Face = t.Face
		lab.Color = th.Color(core.TokenColorText)
		cell := primitive.NewBox(lab)
		cell.Padding = primitive.Symmetric(8, 12)
		if col.Width > 0 {
			cell.Width = col.Width
			r.AddChild(cell)
		} else {
			fl := col.Flex
			if fl <= 0 {
				fl = 1
			}
			r.AddChild(primitive.NewFlexible(fl, cell))
		}
	}
	key := fmt.Sprintf("%d", i)
	bg := render.RGBA{}
	if t.Selection != nil && t.Selection.Has(key) {
		bg = antItemSelectedFill(th)
	}
	press := primitive.NewPressable(r)
	press.Color = bg
	press.ColorHovered = antItemHoverFill(th)
	idx := i
	press.Click = func() {
		if t.Selection != nil {
			t.Selection.Set(key)
		}
		if t.OnRowClick != nil {
			t.OnRowClick(idx, t.Data[idx])
		}
		// force body refresh for selection chrome
		if t.body != nil {
			t.body.MarkNeedsLayout()
		}
	}
	return press
}
