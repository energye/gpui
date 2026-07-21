package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// PressableState is the interactive state surface (skin maps state → tokens later).
type PressableState struct {
	Hovered  bool
	Pressed  bool
	Focused  bool
	Disabled bool
}

// Pressable is a hit target with hover/press and OnClick (C-Hit + C-Event).
// Optional background colors per state for M0 smoke without Decorated/Skin.
type Pressable struct {
	core.NodeBase

	State PressableState
	// Click is invoked on a completed press (down+up on same target).
	// Named Click (not OnClick) so it does not collide with ClickHandler.OnClick.
	Click func()
	// Padding around the child content.
	Padding EdgeInsets

	// Optional state colors (A=0 → skip fill for that state).
	Color         render.RGBA
	ColorHovered  render.RGBA
	ColorPressed  render.RGBA
	ColorDisabled render.RGBA
	// Focusable allows Tab focus in M0+; default true.
	Focusable bool
}

// NewPressable wraps a child as a pressable.
func NewPressable(child core.Node) *Pressable {
	p := &Pressable{Focusable: true}
	p.Init(p)
	p.Hit = core.HitTarget
	if child != nil {
		p.AddChild(child)
	}
	return p
}

// TypeID implements core.Node.
func (p *Pressable) TypeID() string { return TypePressable }

// Layout implements core.Node.
func (p *Pressable) Layout(c core.Constraints) core.Size {
	inner := c.Deflate(p.Padding.Left, p.Padding.Top, p.Padding.Right, p.Padding.Bottom)
	content := core.Size{}
	kids := p.Children()
	if len(kids) > 0 {
		content = kids[0].Layout(inner.Expand())
		kids[0].Base().SetOffset(core.Point{X: p.Padding.Left, Y: p.Padding.Top})
	}
	out := c.Tighten(core.Size{
		Width:  content.Width + p.Padding.Left + p.Padding.Right,
		Height: content.Height + p.Padding.Top + p.Padding.Bottom,
	})
	p.SetSize(out)
	return out
}

// Paint implements core.Node.
func (p *Pressable) Paint(pc *core.PaintContext) {
	col := p.Color
	switch {
	case p.State.Disabled && p.ColorDisabled.A > 0:
		col = p.ColorDisabled
	case p.State.Pressed && p.ColorPressed.A > 0:
		col = p.ColorPressed
	case p.State.Hovered && p.ColorHovered.A > 0:
		col = p.ColorHovered
	}
	if col.A > 0 && pc != nil {
		pc.FillLocalRect(0, 0, p.Size().Width, p.Size().Height, col)
	}
	p.DefaultPaintChildren(pc)
	// Focus ring on top.
	if p.State.Focused && pc != nil {
		ring := render.RGBA{R: 0.09, G: 0.42, B: 0.93, A: 0.55}
		if pc.Theme != nil {
			if c := pc.Theme.Color(core.TokenColorPrimary); c.A > 0 {
				ring = c
				ring.A = 0.55
			}
		}
		sz := p.Size()
		pc.StrokeLocalRoundRect(-2, -2, sz.Width+4, sz.Height+4, 4, 2, ring)
	}
}

// HitTest implements core.Node.
func (p *Pressable) HitTest(pt core.Point) core.Node {
	if p.State.Disabled {
		return nil
	}
	if !p.LocalBounds().Contains(pt) {
		return nil
	}
	// Prefer self as target (children are visual).
	return p
}

// HandlePointer implements core.PointerHandler.
func (p *Pressable) HandlePointer(ev *core.PointerEvent) {
	if p.State.Disabled || ev == nil {
		return
	}
	switch ev.Type {
	case core.PointerDown:
		if ev.Button == core.ButtonLeft || ev.Button == core.ButtonNone {
			p.setPressed(true)
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		p.setPressed(false)
		ev.Handled = true
	case core.PointerMove:
		// hover maintained by tree; paint dirty on state change
	}
}

// OnClick implements core.ClickHandler.
func (p *Pressable) OnClick(ev *core.PointerEvent) {
	if p.State.Disabled {
		return
	}
	if p.Click != nil {
		p.Click()
	}
	if ev != nil {
		ev.Handled = true
	}
}

// CanFocus implements focusable.
func (p *Pressable) CanFocus() bool { return p.Focusable && !p.State.Disabled }

// IsFocused reports focus state.
func (p *Pressable) IsFocused() bool { return p.State.Focused }

// HandleKey implements core.KeyHandler — Space/Enter activate when focused.
func (p *Pressable) HandleKey(ev *core.KeyEvent) {
	if p.State.Disabled || ev == nil || ev.Type != core.KeyDown {
		return
	}
	if ev.Key == " " || ev.Key == "Space" || ev.Key == "Enter" || ev.Key == "Return" {
		if p.Click != nil {
			p.Click()
		}
		ev.Handled = true
	}
}

// SetHovered updates hover state (used by core.Tree).
func (p *Pressable) SetHovered(h bool) {
	if p.State.Hovered == h {
		return
	}
	p.State.Hovered = h
	p.MarkNeedsPaint()
}

// SetFocused implements core.FocusTarget.
func (p *Pressable) SetFocused(f bool) {
	if p.State.Focused == f {
		return
	}
	p.State.Focused = f
	p.MarkNeedsPaint()
}

func (p *Pressable) setPressed(v bool) {
	if p.State.Pressed == v {
		return
	}
	p.State.Pressed = v
	p.MarkNeedsPaint()
}

// SetDisabled updates disabled state.
func (p *Pressable) SetDisabled(d bool) {
	if p.State.Disabled == d {
		return
	}
	p.State.Disabled = d
	if d {
		p.State.Pressed = false
		p.State.Hovered = false
	}
	p.MarkNeedsPaint()
}

// SetColors is a convenience for M0 smoke (normal / hover / pressed).
func (p *Pressable) SetColors(normal, hovered, pressed render.RGBA) *Pressable {
	p.Color = normal
	p.ColorHovered = hovered
	p.ColorPressed = pressed
	return p
}
