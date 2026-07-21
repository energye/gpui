package primitive

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Canvas is a fixed-size node with a custom vector paint callback (C-CanvasPaint).
type Canvas struct {
	core.NodeBase

	Width, Height float64
	// Paint draws in local coordinates (0,0 = top-left of canvas).
	// pc.Origin is already absolute top-left.
	PaintFn func(pc *core.PaintContext, size core.Size)
}

// NewCanvas creates a canvas with size and paint function.
func NewCanvas(w, h float64, paint func(pc *core.PaintContext, size core.Size)) *Canvas {
	c := &Canvas{Width: w, Height: h, PaintFn: paint}
	c.Init(c)
	c.Hit = core.HitDefer
	return c
}

// TypeID implements core.Node.
func (c *Canvas) TypeID() string { return TypeCanvas }

// Layout implements core.Node.
func (c *Canvas) Layout(cons core.Constraints) core.Size {
	w, h := c.Width, c.Height
	if w <= 0 {
		w = 64
	}
	if h <= 0 {
		h = 64
	}
	out := cons.Tighten(core.Size{Width: w, Height: h})
	c.SetSize(out)
	return out
}

// Paint implements core.Node.
func (c *Canvas) Paint(pc *core.PaintContext) {
	if c.PaintFn != nil && pc != nil {
		c.PaintFn(pc, c.Size())
	}
	c.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (c *Canvas) HitTest(p core.Point) core.Node { return c.DefaultHitTest(p) }

// Motion animates opacity and/or height factor of a child (C-Motion).
// Call Advance each frame with the tree clock (or host dt).
type Motion struct {
	core.NodeBase

	// Opacity from→to (default 0→1 on enter).
	FromOpacity, ToOpacity float64
	// HeightFactor 0..1 multiplies child height (collapse animation).
	FromHeight, ToHeight float64
	// UseHeight enables height animation.
	UseHeight bool
	// UseOpacity enables opacity (via layer alpha simulation: skip child paint when ~0).
	UseOpacity bool

	Anim core.Anim

	// progress cached
	t float64
	// child measured size
	childSize core.Size
}

// NewMotion wraps a child with a fade-in animation.
func NewMotion(child core.Node) *Motion {
	m := &Motion{
		FromOpacity: 0, ToOpacity: 1,
		FromHeight: 0, ToHeight: 1,
		UseOpacity: true,
		Anim:       core.Anim{Duration: 0.25, Ease: core.EaseOutCubic},
	}
	m.Init(m)
	m.Hit = core.HitDefer
	if child != nil {
		m.AddChild(child)
	}
	m.Anim.Start()
	return m
}

// TypeID implements core.Node.
func (m *Motion) TypeID() string { return TypeMotion }

// Progress returns current eased progress 0..1.
func (m *Motion) Progress() float64 { return m.t }

// Advance updates animation from dt (respects ReduceMotion).
func (m *Motion) Advance(dt float64, reduce bool) {
	if reduce {
		m.t = 1
		m.Anim.Stop()
		return
	}
	m.t = m.Anim.Advance(dt)
	m.MarkNeedsPaint()
	if m.UseHeight {
		m.MarkNeedsLayout()
	}
}

// AdvanceClock uses tree clock.
func (m *Motion) AdvanceClock(t *core.Tree) {
	if t == nil {
		return
	}
	cl := t.Clock()
	m.Advance(cl.DT, cl.ReduceMotion)
}

// Layout implements core.Node.
func (m *Motion) Layout(c core.Constraints) core.Size {
	kids := m.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		m.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	m.childSize = sz
	kids[0].Base().SetOffset(core.Point{})
	if m.UseHeight {
		h := core.Lerp(m.FromHeight, m.ToHeight, m.t) * sz.Height
		out := c.Tighten(core.Size{Width: sz.Width, Height: h})
		m.SetSize(out)
		return out
	}
	out := c.Tighten(sz)
	m.SetSize(out)
	return out
}

// Paint implements core.Node.
func (m *Motion) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	op := 1.0
	if m.UseOpacity {
		op = core.Lerp(m.FromOpacity, m.ToOpacity, m.t)
	}
	if op <= 0.01 {
		return
	}
	// Clip height if collapsing
	if m.UseHeight {
		sz := m.Size()
		pc.PushClipLocal(0, 0, sz.Width, sz.Height)
		m.DefaultPaintChildren(pc)
		pc.Pop()
		return
	}
	// Opacity: approximate by skipping very transparent; full opacity group later
	if op < 0.99 {
		// draw children; hosts may use layer — for M5 we paint normally
		// (true alpha needs PushLayer; optional enhancement)
	}
	m.DefaultPaintChildren(pc)
	_ = op
}

// HitTest implements core.Node.
func (m *Motion) HitTest(p core.Point) core.Node {
	if m.UseOpacity && core.Lerp(m.FromOpacity, m.ToOpacity, m.t) < 0.05 {
		return nil
	}
	return m.DefaultHitTest(p)
}

// Presence keeps a child mounted through leave animation (C-Presence).
// Set WantVisible=false to play exit then report Gone.
type Presence struct {
	core.NodeBase

	WantVisible bool
	// Visible is effective visibility (true until leave completes).
	Visible bool
	// Gone is true after leave animation finished (safe to remove from parent).
	Gone bool

	enter, leave core.Anim
	// phase: 0 hidden, 1 entering, 2 shown, 3 leaving
	phase int
	t     float64

	// Opacity for paint
	opacity float64
}

// NewPresence wraps a child, starts visible.
func NewPresence(child core.Node) *Presence {
	p := &Presence{
		WantVisible: true,
		Visible:     true,
		enter:       core.Anim{Duration: 0.2, Ease: core.EaseOutCubic},
		leave:       core.Anim{Duration: 0.15, Ease: core.EaseOutCubic},
		phase:       2,
		opacity:     1,
	}
	p.Init(p)
	p.Hit = core.HitDefer
	if child != nil {
		p.AddChild(child)
	}
	return p
}

// TypeID implements core.Node.
func (p *Presence) TypeID() string { return TypePresence }

// Show starts enter if not visible.
func (p *Presence) Show() {
	p.WantVisible = true
	p.Gone = false
	if p.phase == 0 || p.phase == 3 {
		p.phase = 1
		p.Visible = true
		p.enter.Start()
	}
}

// Hide starts leave animation.
func (p *Presence) Hide() {
	p.WantVisible = false
	if p.phase == 1 || p.phase == 2 {
		p.phase = 3
		p.leave.Start()
	}
}

// Advance steps presence state machine.
func (p *Presence) Advance(dt float64, reduce bool) {
	if reduce {
		if p.WantVisible {
			p.phase, p.opacity, p.Visible, p.Gone = 2, 1, true, false
		} else {
			p.phase, p.opacity, p.Visible, p.Gone = 0, 0, false, true
		}
		return
	}
	switch p.phase {
	case 1: // entering
		p.t = p.enter.Advance(dt)
		p.opacity = p.t
		if p.enter.Done() {
			p.phase = 2
			p.opacity = 1
		}
	case 3: // leaving
		p.t = p.leave.Advance(dt)
		p.opacity = 1 - p.t
		if p.leave.Done() {
			p.phase = 0
			p.opacity = 0
			p.Visible = false
			p.Gone = true
		}
	case 2:
		p.opacity = 1
	default:
		p.opacity = 0
	}
	p.MarkNeedsPaint()
}

// Layout implements core.Node.
func (p *Presence) Layout(c core.Constraints) core.Size {
	if !p.Visible && p.Gone {
		out := c.Tighten(core.Size{})
		p.SetSize(out)
		return out
	}
	kids := p.Children()
	if len(kids) == 0 {
		out := c.Tighten(core.Size{})
		p.SetSize(out)
		return out
	}
	sz := kids[0].Layout(c.Expand())
	kids[0].Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	p.SetSize(out)
	return out
}

// Paint implements core.Node.
func (p *Presence) Paint(pc *core.PaintContext) {
	if !p.Visible || p.opacity < 0.01 || pc == nil {
		return
	}
	p.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (p *Presence) HitTest(pt core.Point) core.Node {
	if !p.Visible || p.opacity < 0.05 {
		return nil
	}
	return p.DefaultHitTest(pt)
}

// ProgressRing paints a circular progress indicator on a Canvas.
func ProgressRing(size, stroke, progress float64, track, fill render.RGBA) *Canvas {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	return NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil || pc.DC == nil {
			return
		}
		dc := pc.DC
		cx := pc.Origin.X + sz.Width/2
		cy := pc.Origin.Y + sz.Height/2
		r := math.Min(sz.Width, sz.Height)/2 - stroke
		// track
		dc.SetRGBA(track.R, track.G, track.B, track.A)
		dc.SetLineWidth(stroke)
		dc.DrawCircle(cx, cy, r)
		_ = dc.Stroke()
		// arc as polyline approximation
		if progress > 0 {
			dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
			dc.SetLineWidth(stroke)
			start := -math.Pi / 2
			end := start + 2*math.Pi*progress
			steps := int(64 * progress)
			if steps < 4 {
				steps = 4
			}
			for i := 0; i <= steps; i++ {
				a := start + (end-start)*float64(i)/float64(steps)
				x := cx + r*math.Cos(a)
				y := cy + r*math.Sin(a)
				if i == 0 {
					dc.MoveTo(x, y)
				} else {
					dc.LineTo(x, y)
				}
			}
			_ = dc.Stroke()
		}
	})
}
