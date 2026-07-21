package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Grid is a CSS-grid-like container (C-Grid simplified).
type Grid struct {
	core.NodeBase

	Columns   []core.GridTrack
	Rows      []core.GridTrack
	ColumnGap float64
	RowGap    float64
	// Cells optional explicit placement; empty → row-major flow of Children.
	Cells   []core.GridCell
	Padding EdgeInsets
}

// NewGrid creates a grid with the given column tracks.
func NewGrid(columns []core.GridTrack, children ...core.Node) *Grid {
	g := &Grid{Columns: columns, ColumnGap: 8, RowGap: 8}
	g.Init(g)
	g.Hit = core.HitDefer
	for _, c := range children {
		g.AddChild(c)
	}
	return g
}

// TypeID implements core.Node.
func (g *Grid) TypeID() string { return TypeGrid }

// Layout implements core.Node.
func (g *Grid) Layout(c core.Constraints) core.Size {
	inner := c.Deflate(g.Padding.Left, g.Padding.Top, g.Padding.Right, g.Padding.Bottom)
	cells := g.Cells
	if len(cells) == 0 {
		nCol := len(g.Columns)
		if nCol == 0 {
			nCol = 2
		}
		for i, ch := range g.Children() {
			cells = append(cells, core.GridCell{
				Node: ch, Col: i % nCol, Row: i / nCol, ColSpan: 1, RowSpan: 1,
			})
		}
	}
	// LayoutGrid positions nodes in Cells; ensure they are our children for hit/paint.
	sz := core.LayoutGrid(&g.NodeBase, inner, core.GridLayoutParams{
		Columns: g.Columns, Rows: g.Rows,
		ColumnGap: g.ColumnGap, RowGap: g.RowGap,
		Cells: cells,
	})
	if g.Padding.Left != 0 || g.Padding.Top != 0 {
		for _, ch := range g.Children() {
			o := ch.Base().Offset()
			ch.Base().SetOffset(core.Point{X: o.X + g.Padding.Left, Y: o.Y + g.Padding.Top})
		}
	}
	out := core.Size{
		Width:  sz.Width + g.Padding.Left + g.Padding.Right,
		Height: sz.Height + g.Padding.Top + g.Padding.Bottom,
	}
	out = c.Tighten(out)
	g.SetSize(out)
	return out
}

// Paint implements core.Node.
func (g *Grid) Paint(pc *core.PaintContext) { g.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (g *Grid) HitTest(p core.Point) core.Node { return g.DefaultHitTest(p) }

// Sticky pins content near a scroll edge (C-Sticky, simplified).
type Sticky struct {
	core.NodeBase
	Top    float64
	UseTop bool
}

// NewSticky wraps a child.
func NewSticky(child core.Node) *Sticky {
	s := &Sticky{UseTop: true}
	s.Init(s)
	s.Hit = core.HitDefer
	if child != nil {
		s.AddChild(child)
	}
	return s
}

// TypeID implements core.Node.
func (s *Sticky) TypeID() string { return TypeSticky }

// Layout implements core.Node.
func (s *Sticky) Layout(c core.Constraints) core.Size {
	kids := s.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		s.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	s.SetSize(out)
	return out
}

// Paint implements core.Node with sticky visual adjustment inside ScrollViewport.
func (s *Sticky) Paint(pc *core.PaintContext) {
	if s.UseTop && pc != nil {
		for p := s.Parent(); p != nil; p = p.Parent() {
			if _, ok := p.(*ScrollViewport); ok {
				oy := s.Offset().Y
				if oy < s.Top {
					adj := pc.WithOrigin(pc.Origin.Add(core.Point{Y: s.Top - oy}))
					s.DefaultPaintChildren(adj)
					return
				}
				break
			}
		}
	}
	s.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (s *Sticky) HitTest(p core.Point) core.Node { return s.DefaultHitTest(p) }

// Draggable is a drag handle/source (C-Drag).
type Draggable struct {
	core.NodeBase

	Dragging       bool
	StartX, StartY float64
	DX, DY         float64
	// Axis: 0 both, 1 horizontal, 2 vertical.
	Axis        int
	OnDragStart func()
	OnDrag      func(dx, dy float64)
	OnDragEnd   func(dx, dy float64)

	lastX, lastY float64
	active       bool
}

// NewDraggable wraps a child as a drag source.
func NewDraggable(child core.Node) *Draggable {
	d := &Draggable{}
	d.Init(d)
	d.Hit = core.HitTarget
	if child != nil {
		d.AddChild(child)
	}
	return d
}

// TypeID implements core.Node.
func (d *Draggable) TypeID() string { return TypeDraggable }

// Layout implements core.Node.
func (d *Draggable) Layout(c core.Constraints) core.Size {
	kids := d.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		d.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	d.SetSize(out)
	return out
}

// Paint implements core.Node.
func (d *Draggable) Paint(pc *core.PaintContext) { d.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (d *Draggable) HitTest(p core.Point) core.Node {
	if d.LocalBounds().Contains(p) {
		return d
	}
	return nil
}

// HandlePointer implements drag tracking.
func (d *Draggable) HandlePointer(ev *core.PointerEvent) {
	if ev == nil {
		return
	}
	switch ev.Type {
	case core.PointerDown:
		d.active = true
		d.Dragging = true
		d.StartX, d.StartY = ev.X, ev.Y
		d.lastX, d.lastY = ev.X, ev.Y
		d.DX, d.DY = 0, 0
		if d.OnDragStart != nil {
			d.OnDragStart()
		}
		ev.Handled = true
	case core.PointerMove:
		if !d.active {
			return
		}
		ddx, ddy := ev.X-d.lastX, ev.Y-d.lastY
		d.lastX, d.lastY = ev.X, ev.Y
		if d.Axis == 1 {
			ddy = 0
		} else if d.Axis == 2 {
			ddx = 0
		}
		d.DX += ddx
		d.DY += ddy
		if d.OnDrag != nil {
			d.OnDrag(d.DX, d.DY)
		}
		d.MarkNeedsPaint()
		ev.Handled = true
	case core.PointerUp, core.PointerCancel:
		if d.active {
			d.active = false
			d.Dragging = false
			if d.OnDragEnd != nil {
				d.OnDragEnd(d.DX, d.DY)
			}
			ev.Handled = true
		}
	}
}

// SplitPane is two panes with a draggable splitter.
type SplitPane struct {
	core.NodeBase

	Horizontal          bool
	Ratio               float64
	MinFirst, MinSecond float64
	Splitter            float64

	first, second core.Node
	handle        *Draggable
	startRatio    float64
}

// NewSplitPane creates a horizontal split (left|right).
func NewSplitPane(first, second core.Node) *SplitPane {
	s := &SplitPane{
		Horizontal: true, Ratio: 0.5,
		MinFirst: 40, MinSecond: 40, Splitter: 6,
		first: first, second: second,
	}
	s.Init(s)
	s.Hit = core.HitDefer

	bar := NewBox()
	bar.Color = render.Hex("#D9D9D9")
	s.handle = NewDraggable(bar)
	s.handle.Axis = 1
	s.handle.OnDragStart = func() { s.startRatio = s.Ratio }
	s.handle.OnDrag = func(dx, dy float64) {
		w := s.Size().Width
		if !s.Horizontal {
			w = s.Size().Height
		}
		avail := w - s.Splitter
		if avail <= 0 {
			return
		}
		delta := dx
		if !s.Horizontal {
			delta = dy
			s.handle.Axis = 2
		}
		s.Ratio = s.startRatio + delta/avail
		minR := s.MinFirst / avail
		maxR := 1 - s.MinSecond/avail
		if minR < 0.05 {
			minR = 0.05
		}
		if maxR > 0.95 {
			maxR = 0.95
		}
		if s.Ratio < minR {
			s.Ratio = minR
		}
		if s.Ratio > maxR {
			s.Ratio = maxR
		}
		s.MarkNeedsLayout()
	}

	s.ClearChildren()
	if first != nil {
		s.AddChild(first)
	}
	s.AddChild(s.handle)
	if second != nil {
		s.AddChild(second)
	}
	return s
}

// TypeID implements core.Node.
func (s *SplitPane) TypeID() string { return TypeSplitPane }

// Layout implements core.Node.
func (s *SplitPane) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if !c.HasBoundedWidth() {
		out.Width = 400
	}
	if !c.HasBoundedHeight() {
		out.Height = 300
	}
	s.SetSize(out)

	if s.Horizontal {
		avail := out.Width - s.Splitter
		if avail < 0 {
			avail = 0
		}
		w1 := avail * s.Ratio
		w2 := avail - w1
		if s.first != nil {
			_ = s.first.Layout(core.Tight(w1, out.Height))
			s.first.Base().SetOffset(core.Point{})
		}
		if s.handle != nil {
			s.handle.Axis = 1
			bar := s.handle.Children()
			if len(bar) > 0 {
				if b, ok := bar[0].(*Box); ok {
					b.Width, b.Height = s.Splitter, out.Height
				}
			}
			_ = s.handle.Layout(core.Tight(s.Splitter, out.Height))
			s.handle.SetOffset(core.Point{X: w1, Y: 0})
		}
		if s.second != nil {
			_ = s.second.Layout(core.Tight(w2, out.Height))
			s.second.Base().SetOffset(core.Point{X: w1 + s.Splitter, Y: 0})
		}
	} else {
		avail := out.Height - s.Splitter
		if avail < 0 {
			avail = 0
		}
		h1 := avail * s.Ratio
		h2 := avail - h1
		if s.first != nil {
			_ = s.first.Layout(core.Tight(out.Width, h1))
			s.first.Base().SetOffset(core.Point{})
		}
		if s.handle != nil {
			s.handle.Axis = 2
			bar := s.handle.Children()
			if len(bar) > 0 {
				if b, ok := bar[0].(*Box); ok {
					b.Width, b.Height = out.Width, s.Splitter
				}
			}
			_ = s.handle.Layout(core.Tight(out.Width, s.Splitter))
			s.handle.SetOffset(core.Point{X: 0, Y: h1})
		}
		if s.second != nil {
			_ = s.second.Layout(core.Tight(out.Width, h2))
			s.second.Base().SetOffset(core.Point{X: 0, Y: h1 + s.Splitter})
		}
	}
	return out
}

// Paint implements core.Node.
func (s *SplitPane) Paint(pc *core.PaintContext) { s.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (s *SplitPane) HitTest(p core.Point) core.Node { return s.DefaultHitTest(p) }
