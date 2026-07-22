package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// ScrollViewport clips children and offsets them by ScrollX/ScrollY (C-Scroll).
//
// Flutter SingleChildScrollView + browser overflow: content may exceed the
// viewport; wheel / drag thumb scrolls. When ShowScrollbar is true (default)
// and content overflows, a thin Ant-style thumb is painted on the trailing edge.
//
// Axis:
//   - AxisVertical (default): content max width = viewport; height unbounded
//   - AxisHorizontal: content max height = viewport; width unbounded
//   - both flags true: both axes unbounded (2D scroll)
type ScrollViewport struct {
	core.NodeBase

	ScrollX, ScrollY float64
	// Content intrinsic size after layout of children.
	ContentW, ContentH float64
	// Width/Height when > 0 fix viewport size; else take parent max / content.
	Width, Height float64

	// AxisVertical / AxisHorizontal enable that scroll axis (default: vertical only).
	// Set both for free 2D pan.
	AxisVertical   bool
	AxisHorizontal bool
	axisSet        bool

	// ShowScrollbar paints track/thumb when overflowing (default true).
	ShowScrollbar bool
	// BarSize logical thickness of the scrollbar (default 6).
	BarSize float64
	// TrackColor / ThumbColor when A>0 override defaults.
	TrackColor, ThumbColor render.RGBA

	// drag state (thumb)
	dragAxis   int // 0 none, 1 Y, 2 X
	dragStart  float64
	dragScroll float64
}

// NewScrollViewport wraps children in a scrollable clip (vertical by default).
func NewScrollViewport(children ...core.Node) *ScrollViewport {
	s := &ScrollViewport{
		AxisVertical:  true,
		ShowScrollbar: true,
		BarSize:       6,
	}
	s.Init(s)
	s.Hit = core.HitBlock
	s.ClipHit = true
	for _, c := range children {
		s.AddChild(c)
	}
	return s
}

// SetAxis configures which axes scroll. Call once after construction.
func (s *ScrollViewport) SetAxis(vertical, horizontal bool) {
	if s == nil {
		return
	}
	s.AxisVertical = vertical
	s.AxisHorizontal = horizontal
	s.axisSet = true
}

// TypeID implements core.Node.
func (s *ScrollViewport) TypeID() string { return TypeScrollViewport }

func (s *ScrollViewport) axes() (vert, horiz bool) {
	if s == nil {
		return true, false
	}
	if !s.axisSet && !s.AxisVertical && !s.AxisHorizontal {
		return true, false // default vertical
	}
	if s.AxisVertical || s.AxisHorizontal {
		return s.AxisVertical, s.AxisHorizontal
	}
	return true, false
}

// Layout implements core.Node.
func (s *ScrollViewport) Layout(c core.Constraints) core.Size {
	vert, horiz := s.axes()

	// Viewport size first (from fixed fields or parent).
	w, h := s.Width, s.Height
	if w <= 0 {
		if c.HasBoundedWidth() && c.MaxWidth < core.Unbounded {
			w = c.MaxWidth
		}
	}
	if h <= 0 {
		if c.HasBoundedHeight() && c.MaxHeight < core.Unbounded {
			h = c.MaxHeight
		}
	}

	// Content constraints: unbounded on scroll axes; tight to viewport on the other.
	contentC := core.Constraints{
		MaxWidth:  core.Unbounded,
		MaxHeight: core.Unbounded,
	}
	if vert && !horiz && w > 0 {
		// Vertical scroll: wrap to viewport width.
		contentC.MaxWidth = w
	}
	if horiz && !vert && h > 0 {
		contentC.MaxHeight = h
	}

	cw, ch := 0.0, 0.0
	for _, child := range s.Children() {
		sz := child.Layout(contentC)
		if sz.Width > cw {
			cw = sz.Width
		}
		if sz.Height > ch {
			ch = sz.Height
		}
	}
	s.ContentW, s.ContentH = cw, ch

	// If viewport still open, size to content on non-scroll axes.
	if w <= 0 {
		w = cw
	}
	if h <= 0 {
		h = ch
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	s.SetSize(out)
	s.clampScroll()
	s.applyChildOffsets()
	return out
}

func (s *ScrollViewport) applyChildOffsets() {
	for _, child := range s.Children() {
		child.Base().SetOffset(core.Point{X: -s.ScrollX, Y: -s.ScrollY})
	}
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
	if s.ShowScrollbar {
		s.paintScrollbars(pc, sz)
	}
}

func (s *ScrollViewport) paintScrollbars(pc *core.PaintContext, sz core.Size) {
	bar := s.BarSize
	if bar <= 0 {
		bar = 6
	}
	track := s.TrackColor
	if track.A <= 0 {
		track = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	thumb := s.ThumbColor
	if thumb.A <= 0 {
		thumb = render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
	}
	vert, horiz := s.axes()
	if vert && s.ContentH > sz.Height+0.5 {
		x, y, w, h := s.vThumbRect(sz, bar)
		// track
		pc.FillLocalRect(sz.Width-bar, 0, bar, sz.Height, track)
		// thumb
		pc.FillLocalRoundRect(x, y, w, h, bar/2, thumb)
	}
	if horiz && s.ContentW > sz.Width+0.5 {
		x, y, w, h := s.hThumbRect(sz, bar)
		pc.FillLocalRect(0, sz.Height-bar, sz.Width, bar, track)
		pc.FillLocalRoundRect(x, y, w, h, bar/2, thumb)
	}
}

func (s *ScrollViewport) vThumbRect(sz core.Size, bar float64) (x, y, w, h float64) {
	viewH := sz.Height
	contentH := s.ContentH
	if contentH < 1 {
		contentH = 1
	}
	h = viewH * viewH / contentH
	if h < 20 {
		h = 20
	}
	if h > viewH {
		h = viewH
	}
	maxScroll := contentH - viewH
	if maxScroll < 0 {
		maxScroll = 0
	}
	track := viewH - h
	y = 0
	if maxScroll > 0 && track > 0 {
		y = s.ScrollY / maxScroll * track
	}
	return sz.Width - bar, y, bar, h
}

func (s *ScrollViewport) hThumbRect(sz core.Size, bar float64) (x, y, w, h float64) {
	viewW := sz.Width
	contentW := s.ContentW
	if contentW < 1 {
		contentW = 1
	}
	w = viewW * viewW / contentW
	if w < 20 {
		w = 20
	}
	if w > viewW {
		w = viewW
	}
	maxScroll := contentW - viewW
	if maxScroll < 0 {
		maxScroll = 0
	}
	track := viewW - w
	x = 0
	if maxScroll > 0 && track > 0 {
		x = s.ScrollX / maxScroll * track
	}
	return x, sz.Height - bar, w, bar
}

// HitTest implements core.Node — children first, then self (for empty / bar hits).
func (s *ScrollViewport) HitTest(p core.Point) core.Node {
	if !s.LocalBounds().Contains(p) {
		return nil
	}
	// Prefer children under content (scrollbar still receives via self if miss).
	if hit := s.DefaultHitTest(p); hit != nil && hit != s {
		return hit
	}
	return s
}

// HandlePointer implements thumb drag (pointer capture via tree on Down).
func (s *ScrollViewport) HandlePointer(ev *core.PointerEvent) {
	if s == nil || ev == nil {
		return
	}
	sz := s.Size()
	bar := s.BarSize
	if bar <= 0 {
		bar = 6
	}
	// Convert absolute event to local.
	abs := core.AbsoluteBounds(s)
	lx := ev.X - abs.Min.X
	ly := ev.Y - abs.Min.Y

	switch ev.Type {
	case core.PointerDown:
		if ev.Button != core.ButtonLeft && ev.Button != core.ButtonNone {
			return
		}
		vert, horiz := s.axes()
		if vert && s.ContentH > sz.Height+0.5 {
			x, y, w, h := s.vThumbRect(sz, bar)
			if lx >= x && lx <= x+w && ly >= y && ly <= y+h {
				s.dragAxis = 1
				s.dragStart = ly
				s.dragScroll = s.ScrollY
				ev.Handled = true
				return
			}
			// Track click: jump page
			if lx >= sz.Width-bar {
				if ly < y {
					s.SetScroll(s.ScrollX, s.ScrollY-sz.Height*0.9)
				} else if ly > y+h {
					s.SetScroll(s.ScrollX, s.ScrollY+sz.Height*0.9)
				}
				ev.Handled = true
				return
			}
		}
		if horiz && s.ContentW > sz.Width+0.5 {
			x, y, w, h := s.hThumbRect(sz, bar)
			if lx >= x && lx <= x+w && ly >= y && ly <= y+h {
				s.dragAxis = 2
				s.dragStart = lx
				s.dragScroll = s.ScrollX
				ev.Handled = true
				return
			}
		}
	case core.PointerMove:
		if s.dragAxis == 1 {
			sz := s.Size()
			bar := s.BarSize
			if bar <= 0 {
				bar = 6
			}
			_, _, _, th := s.vThumbRect(sz, bar)
			track := sz.Height - th
			maxScroll := s.ContentH - sz.Height
			if track > 0 && maxScroll > 0 {
				dy := ly - s.dragStart
				s.SetScroll(s.ScrollX, s.dragScroll+dy/track*maxScroll)
			}
			ev.Handled = true
		} else if s.dragAxis == 2 {
			sz := s.Size()
			bar := s.BarSize
			if bar <= 0 {
				bar = 6
			}
			_, _, tw, _ := s.hThumbRect(sz, bar)
			track := sz.Width - tw
			maxScroll := s.ContentW - sz.Width
			if track > 0 && maxScroll > 0 {
				dx := lx - s.dragStart
				s.SetScroll(s.dragScroll+dx/track*maxScroll, s.ScrollY)
			}
			ev.Handled = true
		}
	case core.PointerUp, core.PointerCancel:
		if s.dragAxis != 0 {
			s.dragAxis = 0
			ev.Handled = true
		}
	}
}

// HandleScroll implements core.ScrollHandler (wheel).
func (s *ScrollViewport) HandleScroll(ev *core.ScrollEvent) {
	if ev == nil {
		return
	}
	// Prefer vertical; shift+wheel or DX for horizontal.
	s.ScrollX += ev.DX
	s.ScrollY += ev.DY
	s.clampScroll()
	s.applyChildOffsets()
	s.MarkNeedsLayout() // offsets changed — keep layout clean via paint+offset
	s.MarkNeedsPaint()
	ev.Handled = true
}

// SetScroll sets offsets and clamps.
func (s *ScrollViewport) SetScroll(x, y float64) {
	s.ScrollX, s.ScrollY = x, y
	s.clampScroll()
	s.applyChildOffsets()
	s.MarkNeedsPaint()
}

// OverflowY reports whether content is taller than the viewport.
func (s *ScrollViewport) OverflowY() bool {
	return s != nil && s.ContentH > s.Size().Height+0.5
}

// OverflowX reports whether content is wider than the viewport.
func (s *ScrollViewport) OverflowX() bool {
	return s != nil && s.ContentW > s.Size().Width+0.5
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
	vert, horiz := s.axes()
	if !horiz {
		s.ScrollX = 0
		maxX = 0
	}
	if !vert {
		s.ScrollY = 0
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
