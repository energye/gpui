package primitive

import "github.com/energye/gpui/ui/core"

// ScrollViewport clips children and offsets them by ScrollX/ScrollY (C-Scroll).
type ScrollViewport struct {
	core.NodeBase

	ScrollX, ScrollY float64
	// Content intrinsic size after layout of children.
	ContentW, ContentH float64
	// Width/Height when > 0 fix viewport size.
	Width, Height float64
}

// NewScrollViewport wraps children in a scrollable clip.
func NewScrollViewport(children ...core.Node) *ScrollViewport {
	s := &ScrollViewport{}
	s.Init(s)
	s.Hit = core.HitBlock
	s.ClipHit = true
	for _, c := range children {
		s.AddChild(c)
	}
	return s
}

// TypeID implements core.Node.
func (s *ScrollViewport) TypeID() string { return TypeScrollViewport }

// Layout implements core.Node.
func (s *ScrollViewport) Layout(c core.Constraints) core.Size {
	// Measure content unbounded on scroll axes.
	contentC := core.Constraints{
		MaxWidth:  core.Unbounded,
		MaxHeight: core.Unbounded,
	}
	if c.HasBoundedWidth() {
		// allow content wider than viewport
		contentC.MaxWidth = core.Unbounded
	}
	cw, ch := 0.0, 0.0
	for _, child := range s.Children() {
		sz := child.Layout(contentC)
		child.Base().SetOffset(core.Point{X: -s.ScrollX, Y: -s.ScrollY})
		if sz.Width > cw {
			cw = sz.Width
		}
		if sz.Height > ch {
			ch = sz.Height
		}
	}
	s.ContentW, s.ContentH = cw, ch
	w, h := s.Width, s.Height
	if w <= 0 {
		if c.HasBoundedWidth() {
			w = c.MaxWidth
		} else {
			w = cw
		}
	}
	if h <= 0 {
		if c.HasBoundedHeight() {
			h = c.MaxHeight
		} else {
			h = ch
		}
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	s.SetSize(out)
	s.clampScroll()
	// re-apply offsets after clamp
	for _, child := range s.Children() {
		child.Base().SetOffset(core.Point{X: -s.ScrollX, Y: -s.ScrollY})
	}
	return out
}

// Paint implements core.Node.
func (s *ScrollViewport) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := s.Size()
	pc.PushClipLocal(0, 0, sz.Width, sz.Height)
	s.DefaultPaintChildren(pc)
	pc.Pop()
}

// HitTest implements core.Node.
func (s *ScrollViewport) HitTest(p core.Point) core.Node {
	return s.DefaultHitTest(p)
}

// HandleScroll implements core.ScrollHandler.
func (s *ScrollViewport) HandleScroll(ev *core.ScrollEvent) {
	if ev == nil {
		return
	}
	s.ScrollX += ev.DX
	s.ScrollY += ev.DY
	s.clampScroll()
	for _, child := range s.Children() {
		child.Base().SetOffset(core.Point{X: -s.ScrollX, Y: -s.ScrollY})
	}
	s.MarkNeedsPaint()
	ev.Handled = true
}

// SetScroll sets offsets and clamps.
func (s *ScrollViewport) SetScroll(x, y float64) {
	s.ScrollX, s.ScrollY = x, y
	s.clampScroll()
	for _, child := range s.Children() {
		child.Base().SetOffset(core.Point{X: -s.ScrollX, Y: -s.ScrollY})
	}
	s.MarkNeedsPaint()
}

func (s *ScrollViewport) clampScroll() {
	maxX := s.ContentW - s.Size().Width
	maxY := s.ContentH - s.Size().Height
	if maxX < 0 {
		maxX = 0
	}
	if maxY < 0 {
		maxY = 0
	}
	if s.ScrollX < 0 {
		s.ScrollX = 0
	}
	if s.ScrollY < 0 {
		s.ScrollY = 0
	}
	if s.ScrollX > maxX {
		s.ScrollX = maxX
	}
	if s.ScrollY > maxY {
		s.ScrollY = maxY
	}
}
