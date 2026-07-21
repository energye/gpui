package primitive

import "github.com/energye/gpui/ui/core"

// Focusable is a focus shell without press semantics (C-Focus).
// Use when a region should be keyboard-focusable but not a button.
type Focusable struct {
	core.NodeBase

	focused  bool
	Disabled bool
	// AcceptFocus defaults true.
	AcceptFocus bool
	// OnFocusChange is optional.
	OnFocusChange func(focused bool)
	// Child padding.
	Padding EdgeInsets
}

// NewFocusable wraps a child as a focus target.
func NewFocusable(child core.Node) *Focusable {
	f := &Focusable{AcceptFocus: true}
	f.Init(f)
	f.Hit = core.HitTarget
	if child != nil {
		f.AddChild(child)
	}
	return f
}

// TypeID implements core.Node.
func (f *Focusable) TypeID() string { return TypeFocusable }

// Layout implements core.Node.
func (f *Focusable) Layout(c core.Constraints) core.Size {
	inner := c.Deflate(f.Padding.Left, f.Padding.Top, f.Padding.Right, f.Padding.Bottom)
	content := core.Size{}
	kids := f.Children()
	if len(kids) > 0 {
		content = kids[0].Layout(inner.Expand())
		kids[0].Base().SetOffset(core.Point{X: f.Padding.Left, Y: f.Padding.Top})
	}
	out := c.Tighten(core.Size{
		Width:  content.Width + f.Padding.Left + f.Padding.Right,
		Height: content.Height + f.Padding.Top + f.Padding.Bottom,
	})
	f.SetSize(out)
	return out
}

// Paint implements core.Node.
func (f *Focusable) Paint(pc *core.PaintContext) {
	// Focus ring
	if f.focused && pc != nil {
		col := core.DefaultTheme().Color(core.TokenColorPrimary)
		if pc.Theme != nil {
			col = pc.Theme.Color(core.TokenColorPrimary)
		}
		sz := f.Size()
		pc.StrokeLocalRoundRect(-2, -2, sz.Width+4, sz.Height+4, 4, 2, col)
	}
	f.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (f *Focusable) HitTest(p core.Point) core.Node {
	if f.Disabled {
		return nil
	}
	return f.DefaultHitTest(p)
}

// CanFocus implements focusable.
func (f *Focusable) CanFocus() bool { return f.AcceptFocus && !f.Disabled }

// IsFocused reports focus state.
func (f *Focusable) IsFocused() bool { return f.focused }

// SetFocused implements core.FocusTarget.
func (f *Focusable) SetFocused(v bool) {
	if f.focused == v {
		return
	}
	f.focused = v
	f.MarkNeedsPaint()
	if f.OnFocusChange != nil {
		f.OnFocusChange(v)
	}
}
