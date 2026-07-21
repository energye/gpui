package kit

import (
	"fmt"

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

// Table is a simple data table with sticky header + virtual body (B4 base).
type Table struct {
	Root       *primitive.Flex
	header     *primitive.Flex
	body       *primitive.VirtualList
	Columns    []TableColumn
	Data       []map[string]string
	RowHeight  float64
	Face       text.Face
	Theme      *core.Theme
	Selection  *core.SelectionModel
	OnRowClick func(index int, row map[string]string)
}

// NewTable creates a table.
func NewTable(columns []TableColumn, data []map[string]string) *Table {
	t := &Table{
		Columns: columns, Data: data, RowHeight: 36,
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
	if t.body != nil {
		t.body.ItemCount = len(data)
		t.body.MarkNeedsLayout()
	}
}

func (t *Table) theme() *core.Theme {
	if t.Theme != nil {
		return t.Theme
	}
	return DefaultTheme()
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
		cell.Padding = primitive.Symmetric(10, 8)
		cell.Background = th.Color(core.TokenColorFillSecondary)
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
	frame.BorderWidth = 1
	frame.BorderColor = th.Color(core.TokenColorBorder)
	frame.Radius = th.SizeOr(core.TokenBorderRadius, 6)

	t.Root = primitive.Column(frame)
	t.Root.CrossAlign = core.CrossStretch
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
		cell.Padding = primitive.Symmetric(10, 6)
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
		bg = th.Color(core.TokenColorPrimary)
		bg.A = 0.08
	}
	press := primitive.NewPressable(r)
	press.Color = bg
	press.ColorHovered = th.Color(core.TokenColorFillSecondary)
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

// List is a simple vertical list (non-virtual or virtual for large N).
type List struct {
	Root       *primitive.Decorated
	scroll     *primitive.ScrollViewport
	vlist      *primitive.VirtualList
	Items      []string
	Selected   int
	ItemHeight float64
	Virtual    bool
	Face       text.Face
	Theme      *core.Theme
	OnSelect   func(index int, item string)
}

// NewList creates a list.
func NewList(items ...string) *List {
	l := &List{Items: items, Selected: -1, ItemHeight: 36, Virtual: len(items) > 40}
	l.rebuild()
	return l
}

// Node returns the root.
func (l *List) Node() core.Node {
	if l.Root == nil {
		l.rebuild()
	}
	return l.Root
}

// SetItems replaces items.
func (l *List) SetItems(items []string) {
	l.Items = items
	l.Virtual = len(items) > 40
	l.rebuild()
}

func (l *List) theme() *core.Theme {
	if l.Theme != nil {
		return l.Theme
	}
	return DefaultTheme()
}

func (l *List) rebuild() {
	th := l.theme()
	if l.Virtual {
		l.vlist = primitive.NewVirtualList(l.ItemHeight, func(i int) core.Node {
			return l.itemNode(i)
		})
		l.vlist.ItemCount = len(l.Items)
		l.vlist.Height = 200
		l.Root = primitive.NewDecorated(l.vlist)
	} else {
		col := primitive.Column()
		col.Gap = 0
		for i := range l.Items {
			col.AddChild(l.itemNode(i))
		}
		l.scroll = primitive.NewScrollViewport(col)
		l.scroll.Height = 200
		l.Root = primitive.NewDecorated(l.scroll)
	}
	l.Root.BorderWidth = 1
	l.Root.BorderColor = th.Color(core.TokenColorBorder)
	l.Root.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	l.Root.Padding = primitive.All(4)
}

func (l *List) itemNode(i int) core.Node {
	th := l.theme()
	if i < 0 || i >= len(l.Items) {
		return primitive.NewBox()
	}
	lab := primitive.NewText(l.Items[i])
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = l.Face
	lab.Color = th.Color(core.TokenColorText)
	press := primitive.NewPressable(lab)
	press.Padding = primitive.Symmetric(12, 8)
	if i == l.Selected {
		press.Color = th.Color(core.TokenColorFillSecondary)
	}
	press.ColorHovered = th.Color(core.TokenColorFillSecondary)
	idx := i
	press.Click = func() {
		l.Selected = idx
		if l.OnSelect != nil {
			l.OnSelect(idx, l.Items[idx])
		}
		l.rebuild()
	}
	return press
}

// TreeNode is one tree entry.
type TreeNode struct {
	Key      string
	Title    string
	Children []*TreeNode
	Expanded bool
}

// Tree is an expandable tree (B4 base, non-virtual).
type Tree struct {
	Root     *primitive.Decorated
	col      *primitive.Flex
	Nodes    []*TreeNode
	Selected string
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(key string)
}

// NewTree creates a tree.
func NewTree(nodes ...*TreeNode) *Tree {
	tr := &Tree{Nodes: nodes}
	tr.rebuild()
	return tr
}

// Node returns the root.
func (tr *Tree) Node() core.Node {
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.Root
}

func (tr *Tree) theme() *core.Theme {
	if tr.Theme != nil {
		return tr.Theme
	}
	return DefaultTheme()
}

func (tr *Tree) rebuild() {
	th := tr.theme()
	tr.col = primitive.Column()
	tr.col.Gap = 2
	tr.col.CrossAlign = core.CrossStart
	for _, n := range tr.Nodes {
		tr.addNode(tr.col, n, 0)
	}
	tr.Root = primitive.NewDecorated(tr.col)
	tr.Root.Padding = primitive.All(6)
	tr.Root.BorderWidth = 1
	tr.Root.BorderColor = th.Color(core.TokenColorBorder)
	tr.Root.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	tr.Root.Background = th.Color(core.TokenColorBgContainer)
}

func (tr *Tree) addNode(parent *primitive.Flex, n *TreeNode, depth int) {
	if n == nil {
		return
	}
	th := tr.theme()
	hasKids := len(n.Children) > 0
	var chev core.Node
	if hasKids {
		name := "chevron-right"
		if n.Expanded {
			name = "chevron-down"
		}
		ic := primitive.NewIcon(name)
		ic.Size = 12
		ic.Color = th.Color(core.TokenColorTextSecondary)
		chev = ic
	} else {
		sp := primitive.NewBox()
		sp.Width, sp.Height = 12, 12
		chev = sp
	}
	lab := primitive.NewText(n.Title)
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = tr.Face
	lab.Color = th.Color(core.TokenColorText)
	indent := primitive.NewBox()
	indent.Width = float64(depth) * 16

	row := primitive.Row(indent, chev, lab)
	row.Gap = 6
	row.CrossAlign = core.CrossCenter
	press := primitive.NewPressable(row)
	press.Padding = primitive.Symmetric(6, 4)
	if n.Key == tr.Selected {
		press.Color = th.Color(core.TokenColorFillSecondary)
	}
	press.ColorHovered = th.Color(core.TokenColorFillSecondary)
	node := n
	press.Click = func() {
		if hasKids {
			node.Expanded = !node.Expanded
		}
		tr.Selected = node.Key
		if tr.OnSelect != nil {
			tr.OnSelect(node.Key)
		}
		tr.rebuild()
	}
	parent.AddChild(press)
	if hasKids && n.Expanded {
		for _, c := range n.Children {
			tr.addNode(parent, c, depth+1)
		}
	}
}

// Pagination is page navigation controls.
type Pagination struct {
	Root       *primitive.Flex
	Current    int
	Total      int // total pages
	PageSize   int
	TotalItems int
	Face       text.Face
	Theme      *core.Theme
	OnChange   func(page int)
}

// NewPagination creates pagination. totalPages at least 1.
func NewPagination(totalPages int) *Pagination {
	if totalPages < 1 {
		totalPages = 1
	}
	p := &Pagination{Current: 1, Total: totalPages, PageSize: 10}
	p.rebuild()
	return p
}

// Node returns the root.
func (p *Pagination) Node() core.Node {
	if p.Root == nil {
		p.rebuild()
	}
	return p.Root
}

// SetPage sets current page (1-based).
func (p *Pagination) SetPage(page int) {
	if page < 1 {
		page = 1
	}
	if page > p.Total {
		page = p.Total
	}
	if p.Current == page {
		return
	}
	p.Current = page
	p.rebuild()
	if p.OnChange != nil {
		p.OnChange(page)
	}
}

func (p *Pagination) theme() *core.Theme {
	if p.Theme != nil {
		return p.Theme
	}
	return DefaultTheme()
}

func (p *Pagination) rebuild() {
	th := p.theme()
	prev := NewButton("<")
	prev.SetSize(ButtonSmall)
	prev.SetFace(p.Face)
	prev.SetOnClick(func() { p.SetPage(p.Current - 1) })
	if p.Current <= 1 {
		prev.SetDisabled(true)
	}
	next := NewButton(">")
	next.SetSize(ButtonSmall)
	next.SetFace(p.Face)
	next.SetOnClick(func() { p.SetPage(p.Current + 1) })
	if p.Current >= p.Total {
		next.SetDisabled(true)
	}
	info := primitive.NewText(fmt.Sprintf("%d / %d", p.Current, p.Total))
	info.FontSize = th.SizeOr(core.TokenFontSize, 14)
	info.Face = p.Face
	info.Color = th.Color(core.TokenColorText)

	// page number buttons (window of up to 5)
	nums := primitive.Row()
	nums.Gap = 4
	start := p.Current - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > p.Total {
		end = p.Total
		start = end - 4
		if start < 1 {
			start = 1
		}
	}
	for i := start; i <= end; i++ {
		i := i
		b := NewButton(fmt.Sprintf("%d", i))
		b.SetSize(ButtonSmall)
		b.SetFace(p.Face)
		if i == p.Current {
			b.SetType(ButtonPrimary)
		}
		b.SetOnClick(func() { p.SetPage(i) })
		nums.AddChild(b.Node())
	}

	p.Root = primitive.Row(prev.Node(), nums, next.Node(), info)
	p.Root.Gap = 8
	p.Root.CrossAlign = core.CrossCenter
}

// Dropdown is a trigger + menu popup (refined from Select for arbitrary menu).
type Dropdown struct {
	Wrap     *primitive.Flex
	Trigger  *Button
	popup    *primitive.AnchoredPopup
	menu     *Menu
	Open     bool
	Viewport core.Size
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(key string)
}

// NewDropdown creates a dropdown with a labeled trigger.
func NewDropdown(label string, items ...MenuItem) *Dropdown {
	d := &Dropdown{}
	d.Trigger = NewButton(label)
	d.Trigger.SetFace(d.Face)
	d.menu = NewMenu(items...)
	d.menu.Face = d.Face
	d.menu.OnSelect = func(key string) {
		d.SetOpen(false)
		if d.OnSelect != nil {
			d.OnSelect(key)
		}
	}
	d.rebuild()
	return d
}

// Node returns the root.
func (d *Dropdown) Node() core.Node {
	if d.Wrap == nil {
		d.rebuild()
	}
	return d.Wrap
}

// SetOpen toggles the menu.
func (d *Dropdown) SetOpen(open bool) {
	d.Open = open
	if d.popup != nil {
		d.popup.UpdateAnchorFromNode(d.Trigger.Root)
		if d.Viewport.Width > 0 {
			d.popup.Viewport = d.Viewport
		}
		d.popup.SetOpen(open)
	}
}

// Sync repositions while open.
func (d *Dropdown) Sync() {
	if d.Open {
		d.SetOpen(true)
	}
}

func (d *Dropdown) rebuild() {
	d.popup = primitive.NewAnchoredPopup(d.menu.Node())
	d.popup.Placement = primitive.PlaceBottomStart
	d.popup.Gap = 4
	d.popup.Portal.ID = "dropdown"
	d.Trigger.SetOnClick(func() { d.SetOpen(!d.Open) })
	d.Wrap = primitive.Column(d.Trigger.Node(), d.popup)
	d.Wrap.CrossAlign = core.CrossStart
}

// Transfer is a simplified dual-list picker (B4 simplified).
type Transfer struct {
	Root     *primitive.Flex
	Source   *List
	Target   *List
	Face     text.Face
	Theme    *core.Theme
	OnChange func(target []string)
}

// NewTransfer creates a transfer with source items.
func NewTransfer(source []string) *Transfer {
	tr := &Transfer{}
	tr.Source = NewList(source...)
	tr.Target = NewList()
	tr.Source.Face = tr.Face
	tr.Target.Face = tr.Face
	tr.Source.OnSelect = func(i int, item string) {
		// move to target
		items := append([]string{}, tr.Source.Items...)
		if i < 0 || i >= len(items) {
			return
		}
		moved := items[i]
		tr.Source.SetItems(append(items[:i], items[i+1:]...))
		tr.Target.SetItems(append(tr.Target.Items, moved))
		if tr.OnChange != nil {
			tr.OnChange(tr.Target.Items)
		}
	}
	tr.Target.OnSelect = func(i int, item string) {
		items := append([]string{}, tr.Target.Items...)
		if i < 0 || i >= len(items) {
			return
		}
		moved := items[i]
		tr.Target.SetItems(append(items[:i], items[i+1:]...))
		tr.Source.SetItems(append(tr.Source.Items, moved))
		if tr.OnChange != nil {
			tr.OnChange(tr.Target.Items)
		}
	}
	tr.Root = primitive.Row(tr.Source.Node(), tr.Target.Node())
	tr.Root.Gap = 12
	return tr
}

// Node returns the root.
func (tr *Transfer) Node() core.Node { return tr.Root }

// Cascader is a simplified multi-column cascade picker (B4 simplified).
type Cascader struct {
	Root     *primitive.Flex
	Columns  []*List
	Options  []*TreeNode // reuse tree structure
	Path     []string
	Face     text.Face
	Theme    *core.Theme
	OnChange func(path []string)
}

// NewCascader creates a cascader from tree options.
func NewCascader(options ...*TreeNode) *Cascader {
	c := &Cascader{Options: options}
	c.rebuild()
	return c
}

// Node returns the root.
func (c *Cascader) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

func (c *Cascader) rebuild() {
	c.Root = primitive.Row()
	c.Root.Gap = 8
	// first column = roots
	labels := make([]string, 0, len(c.Options))
	for _, n := range c.Options {
		labels = append(labels, n.Title)
	}
	col0 := NewList(labels...)
	col0.Face = c.Face
	col0.OnSelect = func(i int, _ string) {
		if i < 0 || i >= len(c.Options) {
			return
		}
		c.Path = []string{c.Options[i].Key}
		c.showLevel(1, c.Options[i].Children)
		if c.OnChange != nil {
			c.OnChange(c.Path)
		}
	}
	c.Columns = []*List{col0}
	c.Root.AddChild(col0.Node())
}

func (c *Cascader) showLevel(level int, nodes []*TreeNode) {
	// trim extra columns
	for len(c.Columns) > level {
		// rebuild root from remaining columns
		c.Columns = c.Columns[:level]
	}
	if len(nodes) == 0 {
		c.rebuildRootFromColumns()
		return
	}
	labels := make([]string, 0, len(nodes))
	for _, n := range nodes {
		labels = append(labels, n.Title)
	}
	col := NewList(labels...)
	col.Face = c.Face
	col.OnSelect = func(i int, _ string) {
		if i < 0 || i >= len(nodes) {
			return
		}
		// truncate path to level
		if len(c.Path) > level {
			c.Path = c.Path[:level]
		}
		if len(c.Path) == level {
			c.Path = append(c.Path, nodes[i].Key)
		} else {
			c.Path = append(c.Path[:level], nodes[i].Key)
		}
		c.showLevel(level+1, nodes[i].Children)
		if c.OnChange != nil {
			c.OnChange(c.Path)
		}
	}
	c.Columns = append(c.Columns, col)
	c.rebuildRootFromColumns()
}

func (c *Cascader) rebuildRootFromColumns() {
	c.Root = primitive.Row()
	c.Root.Gap = 8
	for _, col := range c.Columns {
		c.Root.AddChild(col.Node())
	}
}
