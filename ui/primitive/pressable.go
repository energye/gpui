package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// PressableState is the interactive state surface (skin maps state → tokens later).
type PressableState struct {
	Hovered bool
	Pressed bool
	Focused bool
	// FocusVisible: ring only for keyboard focus (Ant :focus-visible; not mouse click).
	FocusVisible bool
	Disabled     bool
}

// Pressable is a hit target with hover/press and OnClick (C-Hit + C-Event).
//
// Generic interaction (Skia/Ant-style, configured here — not per-kit hacks):
//
//   - EnableRipple: gray ink expanding ~RippleExtra beyond the control (default on)
//   - ShowFocusRing: keyboard focus outline (default on; kit disables for Switch etc.)
//   - Click only fires when pointer is released over this target (tree enforces)
type Pressable struct {
	core.NodeBase

	State PressableState
	// Click is invoked on a completed press (down+up on same target).
	// Named Click (not OnClick) so it does not collide with ClickHandler.OnClick.
	Click func()
	// OnStateChange is invoked when Hovered/Pressed/Focused/Disabled change.
	OnStateChange func()
	// Padding around the child content.
	Padding EdgeInsets

	// Optional state colors (A=0 → skip fill for that state).
	Color         render.RGBA
	ColorHovered  render.RGBA
	ColorPressed  render.RGBA
	ColorDisabled render.RGBA
	// Focusable allows Tab focus; default true.
	Focusable bool
	// ShowFocusRing draws keyboard focus chrome when Focused (default true).
	ShowFocusRing bool
	// FocusRingRadius matches child chrome corner radius (0 → default 6).
	FocusRingRadius float64
	// FocusRingOutset distance outside the box (0 → 1.5 Ant-tight).
	FocusRingOutset float64

	// EnableRipple: same-shape gray wave on confirmed click (release inside), not on press-down.
	// Set false to disable for a control (or globally via DefaultPressableRipple).
	EnableRipple bool
	// RippleExtra is how far the wave extends past the control edge (default 8).
	RippleExtra float64
	// RippleColor defaults to rgba(0,0,0,0.12).
	RippleColor render.RGBA
	// RippleDuration seconds (default 0.4).
	RippleDuration float64

	// ripple animation state (0..1); driven by Tick when active.
	ripplePhase  float64
	rippleActive bool
	rippleCX     float64 // local origin of wave
	rippleCY     float64
}

// DefaultPressableRipple is the package default for new Pressables (true).
// Set false at app start to disable ink globally.
var DefaultPressableRipple = true

// NewPressable wraps a child as a pressable with default interaction chrome.
func NewPressable(child core.Node) *Pressable {
	p := &Pressable{
		Focusable:      true,
		ShowFocusRing:  true,
		EnableRipple:   DefaultPressableRipple,
		RippleExtra:    8, // ~8px past control outer edge
		RippleDuration: 0.4,
		RippleColor:    render.RGBA{R: 0, G: 0, B: 0, A: 0.12},
	}
	p.Init(p)
	// Not a RepaintBoundary by default. Tabs/gallery mount dozens of Pressables;
	// each boundary must be visited + markLive on every ANIMATING frame
	// (CompositeLive live-set protocol). That path alone pushed idle Spin to ~4% CPU.
	// Spin/Skeleton/ScrollViewport own dedicated boundaries; ripple paint bubbles
	// to the nearest ancestor boundary instead.
	p.Hit = core.HitTarget
	p.Cursor = core.CursorPointer // clickable hand by default
	if child != nil {
		p.AddChild(child)
	}
	return p
}

// TypeID implements core.Node.
func (p *Pressable) TypeID() string { return TypePressable }

// Layout implements core.Node.
//
// hit == paint (Flutter Align.topLeft):
//   - Own size is content+padding unless the parent forces a TIGHT axis (Min==Max),
//     e.g. tab item host 160×40 — then expand to fill so the whole chrome hits.
//   - Child offset is (padL, padT) top-left. Vertical center only when height is
//     already tight (same rule as expand). Never use loose MaxHeight to center:
//     Tabs body passes a huge MaxHeight and that used to paint chrome mid/bottom
//     while hit stayed on the content-sized box at the top (gallery bug).
func (p *Pressable) Layout(c core.Constraints) core.Size {
	inner := c.Deflate(p.Padding.Left, p.Padding.Top, p.Padding.Right, p.Padding.Bottom)
	content := core.Size{}
	kids := p.Children()
	tightH := c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded
	tightW := c.MinWidth == c.MaxWidth && c.MaxWidth < core.Unbounded

	if len(kids) > 0 {
		// Under tight height, give the child the full inner height so Decorated
		// chrome can center its own label (Button). Under loose max, keep loose
		// so intrinsic controls stay content-sized.
		childC := inner.Expand()
		if tightH {
			ih := c.MaxHeight - p.Padding.Top - p.Padding.Bottom
			if ih < 0 {
				ih = 0
			}
			childC.MinHeight, childC.MaxHeight = ih, ih
		}
		if tightW {
			iw := c.MaxWidth - p.Padding.Left - p.Padding.Right
			if iw < 0 {
				iw = 0
			}
			childC.MinWidth, childC.MaxWidth = iw, iw
		}
		content = kids[0].Layout(childC)
		// Top-left padding only. Do NOT center using loose MaxHeight.
		// (If tight, child already fills; Decorated.CenterContent handles chrome.)
		kids[0].Base().SetOffset(core.Point{X: p.Padding.Left, Y: p.Padding.Top})
	}
	out := core.Size{
		Width:  content.Width + p.Padding.Left + p.Padding.Right,
		Height: content.Height + p.Padding.Top + p.Padding.Bottom,
	}
	// Expand only when parent forces a tight size on that axis (tab host, etc.).
	if tightW {
		out.Width = c.MaxWidth
	}
	if tightH {
		out.Height = c.MaxHeight
	}
	if out.Width < c.MinWidth {
		out.Width = c.MinWidth
	}
	if out.Height < c.MinHeight {
		out.Height = c.MinHeight
	}
	if out.Width > c.MaxWidth {
		out.Width = c.MaxWidth
	}
	if out.Height > c.MaxHeight {
		out.Height = c.MaxHeight
	}
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
	sz := p.Size()
	// Round fill when FocusRingRadius is set so kit Button chrome never shows a
	// square press/hover plate over rounded Decorated children.
	if col.A > 0 && pc != nil {
		r := p.FocusRingRadius
		if r > 0 {
			pc.FillLocalRoundRect(0, 0, sz.Width, sz.Height, r, col)
		} else {
			pc.FillLocalRect(0, 0, sz.Width, sz.Height, col)
		}
	}
	p.DefaultPaintChildren(pc)

	// Ink ripple above chrome, clipped to rounded control when radius known.
	if p.EnableRipple && p.rippleActive && pc != nil {
		p.paintRipple(pc, sz)
	}

	// Focus ring only for keyboard focus (:focus-visible). Mouse click focuses
	// without ring — matches Ant Design Button.
	if p.State.Focused && p.State.FocusVisible && p.ShowFocusRing && pc != nil {
		r := p.FocusRingRadius
		if r <= 0 {
			r = 6
		}
		outset := p.FocusRingOutset
		if outset <= 0 {
			outset = 1.5
		}
		PaintFocusRing(pc, sz.Width, sz.Height, r, outset, 2)
	}
}

func (p *Pressable) paintRipple(pc *core.PaintContext, sz core.Size) {
	// phase 0→1: same-shape outline expands past control outer edge, alpha fades.
	// Starts at control bounds (w×h + radius), grows by RippleExtra (~3–4px).
	t := p.ripplePhase
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	ease := 1 - (1-t)*(1-t) // ease-out
	extra := p.RippleExtra
	if extra <= 0 {
		extra = 8
	}
	outset := extra * ease
	if outset < 0.25 && t < 0.05 {
		outset = 0.25 // visible first frame
	}
	col := p.RippleColor
	if col.A <= 0 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.14}
	}
	col.A = col.A * (1 - t)
	if col.A < 0.01 {
		return
	}
	// Match control chrome shape (FocusRingRadius = corner radius of outer chrome).
	radius := p.FocusRingRadius
	if radius < 0 {
		radius = 0
	}
	// Outer rounded rect grows from control size; stroke keeps a thin wave band.
	band := 1.5
	if band > outset+0.5 {
		band = outset + 0.5
	}
	if band < 1 {
		band = 1
	}
	ringR := radius + outset
	pc.StrokeLocalRoundRect(-outset, -outset, sz.Width+2*outset, sz.Height+2*outset, ringR, band, col)
}

// HitTest implements core.Node.
func (p *Pressable) HitTest(pt core.Point) core.Node {
	if p.State.Disabled {
		return nil
	}
	if !p.LocalBounds().Contains(pt) {
		return nil
	}
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
			// Ripple only on confirmed click (OnClick).
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		p.setPressed(false)
		ev.Handled = true
	case core.PointerMove:
		// hover maintained by tree
	}
}

func (p *Pressable) startRipple(ev *core.PointerEvent) {
	if !p.EnableRipple {
		return
	}
	sz := p.Size()
	// Local press point if event carries position relative to node;
	// tree events are absolute — convert via AbsoluteBounds.
	cx, cy := sz.Width/2, sz.Height/2
	if ev != nil {
		abs := core.AbsoluteBounds(p)
		if !abs.Empty() {
			cx = ev.X - abs.Min.X
			cy = ev.Y - abs.Min.Y
		}
	}
	p.rippleCX, p.rippleCY = cx, cy
	p.ripplePhase = 0
	p.rippleActive = true
	p.MarkNeedsPaint()
	// Register ticker on owning tree for ANIMATING demand frames.
	if tr := p.Tree(); tr != nil {
		tr.AddTicker(p)
	}
}

// Tick advances the ripple wave. Implements core.Ticker.
func (p *Pressable) Tick(dt float64) bool {
	if p == nil || !p.rippleActive {
		return false
	}
	dur := p.RippleDuration
	if dur <= 0 {
		dur = 0.4
	}
	p.ripplePhase += dt / dur
	p.MarkNeedsPaint()
	if p.ripplePhase >= 1 {
		p.ripplePhase = 1
		p.rippleActive = false
		return false
	}
	return true
}

// OnClick implements core.ClickHandler.
// Tree only invokes this when pointer-up hits the same target (release-inside).
func (p *Pressable) OnClick(ev *core.PointerEvent) {
	if p.State.Disabled {
		return
	}
	// Ripple only when a real click fires (release still over control).
	p.startRipple(ev)
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
		// Keyboard activate: ripple from center + click.
		p.startRipple(nil)
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
	p.fireStateChange()
}

// SetFocused implements core.FocusTarget.
func (p *Pressable) SetFocused(f bool) {
	if p.State.Focused == f {
		return
	}
	p.State.Focused = f
	if !f {
		p.State.FocusVisible = false
	}
	p.MarkNeedsPaint()
	p.fireStateChange()
}

// SetFocusVisible implements core.FocusVisibleTarget (keyboard vs pointer focus).
func (p *Pressable) SetFocusVisible(v bool) {
	if p.State.FocusVisible == v {
		return
	}
	p.State.FocusVisible = v
	p.MarkNeedsPaint()
	p.fireStateChange()
}

func (p *Pressable) setPressed(v bool) {
	if p.State.Pressed == v {
		return
	}
	p.State.Pressed = v
	p.MarkNeedsPaint()
	p.fireStateChange()
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
		p.rippleActive = false
		p.Cursor = core.CursorNotAllowed
	} else if p.Cursor == core.CursorNotAllowed || p.Cursor == core.CursorInherit {
		p.Cursor = core.CursorPointer
	}
	p.MarkNeedsPaint()
	p.fireStateChange()
}

func (p *Pressable) fireStateChange() {
	if p != nil && p.OnStateChange != nil {
		p.OnStateChange()
	}
}

// SetColors is a convenience for M0 smoke (normal / hover / pressed).
func (p *Pressable) SetColors(normal, hovered, pressed render.RGBA) *Pressable {
	p.Color = normal
	p.ColorHovered = hovered
	p.ColorPressed = pressed
	return p
}
