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
	// FocusRingRadius matches child chrome (0 → 6).
	FocusRingRadius float64
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
	return layoutPaddedChild(&f.NodeBase, c, f.Padding, 0, 0)
}

// Paint implements core.Node.
func (f *Focusable) Paint(pc *core.PaintContext) {
	f.DefaultPaintChildren(pc)
	if f.focused && pc != nil {
		sz := f.Size()
		r := f.FocusRingRadius
		if r <= 0 {
			r = 6
		}
		PaintFocusRing(pc, sz.Width, sz.Height, r, 2, 2)
	}
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
