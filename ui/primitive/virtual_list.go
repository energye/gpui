package primitive

import "github.com/energye/gpui/ui/core"

// VirtualList renders only visible rows of a fixed-height item list (C-Virtual).
// ItemAt returns the node for index; caller should reuse or rebuild as needed.
type VirtualList struct {
	core.NodeBase

	// ItemCount total logical items.
	ItemCount int
	// ItemHeight fixed row height in logical px.
	ItemHeight float64
	// Width/Height of the viewport; 0 → expand.
	Width, Height float64
	// ScrollY content offset.
	ScrollY float64
	// ItemAt builds/returns a row node for index (required).
	ItemAt func(index int) core.Node

	// visible window
	first, last int
	// pooled children currently mounted
}

// NewVirtualList creates an empty virtual list.
func NewVirtualList(itemHeight float64, itemAt func(int) core.Node) *VirtualList {
	if itemHeight <= 0 {
		itemHeight = 32
	}
	v := &VirtualList{ItemHeight: itemHeight, ItemAt: itemAt}
	v.Init(v)
	v.Hit = core.HitBlock
	v.ClipHit = true
	return v
}

// TypeID implements core.Node.
func (v *VirtualList) TypeID() string { return TypeVirtualList }

// ContentHeight is ItemCount * ItemHeight.
func (v *VirtualList) ContentHeight() float64 {
	return float64(v.ItemCount) * v.ItemHeight
}

// Layout implements core.Node.
func (v *VirtualList) Layout(c core.Constraints) core.Size {
	w, h := v.Width, v.Height
	if w <= 0 {
		if c.HasBoundedWidth() {
			w = c.MaxWidth
		} else {
			w = 200
		}
	}
	if h <= 0 {
		if c.HasBoundedHeight() {
			h = c.MaxHeight
		} else {
			h = 200
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	v.SetSize(out)
	v.clampScroll()
	v.rebuildWindow()
	return out
}

func (v *VirtualList) rebuildWindow() {
	// Clear old children
	v.ClearChildren()
	if v.ItemCount <= 0 || v.ItemAt == nil || v.ItemHeight <= 0 {
		v.first, v.last = 0, -1
		return
	}
	vh := v.Size().Height
	first := int(v.ScrollY / v.ItemHeight)
	if first < 0 {
		first = 0
	}
	visible := int(vh/v.ItemHeight) + 2 // overscan
	last := first + visible
	if last > v.ItemCount {
		last = v.ItemCount
	}
	v.first, v.last = first, last-1
	for i := first; i < last; i++ {
		row := v.ItemAt(i)
		if row == nil {
			continue
		}
		// Layout row tight height
		_ = row.Layout(core.Tight(v.Size().Width, v.ItemHeight))
		y := float64(i)*v.ItemHeight - v.ScrollY
		row.Base().SetOffset(core.Point{X: 0, Y: y})
		v.AddChild(row)
	}
}

// Paint implements core.Node.
func (v *VirtualList) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := v.Size()
	pc.PushClipLocal(0, 0, sz.Width, sz.Height)
	v.DefaultPaintChildren(pc)
	pc.Pop()
}

// HitTest implements core.Node.
func (v *VirtualList) HitTest(p core.Point) core.Node {
	return v.DefaultHitTest(p)
}

// HandleScroll implements core.ScrollHandler.
func (v *VirtualList) HandleScroll(ev *core.ScrollEvent) {
	if ev == nil {
		return
	}
	v.ScrollY += ev.DY
	v.clampScroll()
	v.rebuildWindow()
	v.MarkNeedsPaint()
	ev.Handled = true
}

// SetScrollY sets offset.
func (v *VirtualList) SetScrollY(y float64) {
	v.ScrollY = y
	v.clampScroll()
	v.rebuildWindow()
	v.MarkNeedsPaint()
}

func (v *VirtualList) clampScroll() {
	max := v.ContentHeight() - v.Size().Height
	if max < 0 {
		max = 0
	}
	if v.ScrollY < 0 {
		v.ScrollY = 0
	}
	if v.ScrollY > max {
		v.ScrollY = max
	}
}

// FocusScope traps Tab focus within descendants when Active (C-Focus).
// Used by Modal/Drawer: keyboard must not escape to the main tree while open.
type FocusScope struct {
	core.NodeBase
	// Active enables tab trapping and Escape handling (e.g. modal open).
	Active bool
	// OnEscape is invoked on Escape key when Active (Ant Modal keyboard).
	OnEscape func()
}

// NewFocusScope wraps children.
func NewFocusScope(children ...core.Node) *FocusScope {
	f := &FocusScope{Active: true}
	f.Init(f)
	f.Hit = core.HitDefer
	for _, c := range children {
		f.AddChild(c)
	}
	return f
}

// TypeID implements core.Node.
func (f *FocusScope) TypeID() string { return TypeFocusScope }

// FocusTrapActive implements core.ActiveFocusScope.
func (f *FocusScope) FocusTrapActive() bool { return f != nil && f.Active }

// Layout implements core.Node.
func (f *FocusScope) Layout(c core.Constraints) core.Size {
	kids := f.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		f.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	// multi: stack max
	for i := 1; i < len(kids); i++ {
		s := kids[i].Layout(c.Expand())
		kids[i].Base().SetOffset(core.Point{})
		sz = core.MaxSize(sz, s)
	}
	out := c.Tighten(sz)
	f.SetSize(out)
	return out
}

// Paint implements core.Node.
func (f *FocusScope) Paint(pc *core.PaintContext) { f.DefaultPaintChildren(pc) }

// HitTest implements core.Node.
func (f *FocusScope) HitTest(p core.Point) core.Node { return f.DefaultHitTest(p) }

// HandleKey traps Tab within scope and handles Escape when Active.
func (f *FocusScope) HandleKey(ev *core.KeyEvent) {
	if !f.Active || ev == nil || ev.Type != core.KeyDown {
		return
	}
	// Escape → dismiss (Modal/Drawer OnCancel path).
	if ev.Key == "Escape" || ev.Key == "Esc" {
		if f.OnEscape != nil {
			f.OnEscape()
		}
		ev.Handled = true
		return
	}
	if ev.Key != "Tab" && ev.Key != "Shift+Tab" && ev.Key != "ISO_Left_Tab" {
		return
	}
	t := f.Tree()
	if t == nil {
		return
	}
	list := core.CollectFocusables(f)
	if len(list) == 0 {
		ev.Handled = true
		return
	}
	cur := t.Focus()
	idx := -1
	for i, n := range list {
		if n == cur {
			idx = i
			break
		}
	}
	shift := ev.Key == "Shift+Tab" || ev.Key == "ISO_Left_Tab"
	var next core.Node
	if shift {
		if idx <= 0 {
			next = list[len(list)-1]
		} else {
			next = list[idx-1]
		}
	} else {
		if idx < 0 || idx >= len(list)-1 {
			next = list[0]
		} else {
			next = list[idx+1]
		}
	}
	t.SetFocus(next)
	ev.Handled = true
}

// ContainsFocus reports whether n is this scope or a descendant.
func (f *FocusScope) ContainsFocus(n core.Node) bool {
	if f == nil || n == nil {
		return false
	}
	for x := n; x != nil; x = x.Parent() {
		if x == f {
			return true
		}
	}
	return false
}
