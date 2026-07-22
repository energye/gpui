package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// ScrollViewport clips children and offsets them by ScrollX/ScrollY (C-Scroll).
//
// Flutter SingleChildScrollView + browser overflow: content may exceed the
// viewport; wheel / drag thumb scrolls.
//
// Scrollbar chrome is a separate policy object (see Scrollbar). Enable/configure
// via SetScrollbar / Scrollbar(). Default: Auto visibility + non-overlay gutter
// (content layout subtracts bar thickness so content/styles never overlap the bar).
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

	// Bar is the scrollbar policy (separate control). Nil → DefaultScrollbar().
	Bar *Scrollbar
	// ShowScrollbar master enable (false forces Never). Kept for API compat;
	// prefer SetScrollbar / Bar.Enabled.
	ShowScrollbar bool

	// Legacy color / size fields — applied when Bar is nil at paint, or
	// copied into Bar on first resolve if still zero on Bar.
	BarSize                float64
	TrackColor, ThumbColor render.RGBA

	// hover / auto-hide state
	hovered    bool
	onBarV     bool
	onBarH     bool
	revealLeft float64 // seconds of Hover reveal after wheel/scroll
	tree       *core.Tree

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
		Bar:           DefaultScrollbar(),
		BarSize:       6,
	}
	// Vertical-only default: no bottom gutter from Horizontal policy.
	s.Bar.Horizontal = ScrollbarNever
	s.Init(s)
	s.Hit = core.HitBlock
	s.ClipHit = true
	for _, c := range children {
		s.AddChild(c)
	}
	return s
}

// SetScrollbar installs a scrollbar policy (nil → DefaultScrollbar).
// This is the preferred way for containers to enable/configure bars.
func (s *ScrollViewport) SetScrollbar(bar *Scrollbar) {
	if s == nil {
		return
	}
	if bar == nil {
		s.Bar = DefaultScrollbar()
	} else {
		s.Bar = bar
	}
	s.ShowScrollbar = s.Bar.Enabled
	s.MarkNeedsPaint()
}

// Scrollbar returns the mutable policy object for custom configuration
// (visibility, Overlay, thickness, colors, …). Never nil.
// Changes take effect on next layout/paint; call MarkNeedsLayout/Paint as needed.
func (s *ScrollViewport) Scrollbar() *Scrollbar {
	if s == nil {
		return DefaultScrollbar()
	}
	return s.resolveBarMut()
}

// SetShowScrollbar toggles chrome (false = Never; true keeps current policy Enabled).
func (s *ScrollViewport) SetShowScrollbar(v bool) {
	if s == nil {
		return
	}
	s.ShowScrollbar = v
	b := s.resolveBarMut()
	b.Enabled = v
	s.MarkNeedsPaint()
}

// SetScrollbarVisibility sets both axes to the same visibility mode.
func (s *ScrollViewport) SetScrollbarVisibility(v ScrollbarVisibility) {
	if s == nil {
		return
	}
	b := s.resolveBarMut()
	b.Vertical = v
	b.Horizontal = v
	b.Enabled = v != ScrollbarNever
	s.ShowScrollbar = b.Enabled
	s.MarkNeedsPaint()
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

// AttachTicker registers for auto-hide Tick (Hover mode reveal countdown).
func (s *ScrollViewport) AttachTicker(t *core.Tree) {
	if s == nil {
		return
	}
	s.tree = t
	if t != nil {
		t.BindTicker(s, s.revealLeft > 0)
	}
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

// resolveBarMut returns the mutable stored policy (never a clone).
func (s *ScrollViewport) resolveBarMut() *Scrollbar {
	if s.Bar == nil {
		s.Bar = DefaultScrollbar()
	}
	// Sync legacy fields into bar when still defaults.
	if s.BarSize > 0 && s.Bar.Thickness <= 0 {
		s.Bar.Thickness = s.BarSize
	}
	if s.TrackColor.A > 0 && s.Bar.TrackColor.A <= 0 {
		s.Bar.TrackColor = s.TrackColor
	}
	if s.ThumbColor.A > 0 && s.Bar.ThumbColor.A <= 0 {
		s.Bar.ThumbColor = s.ThumbColor
	}
	return s.Bar
}

// resolveBar returns effective policy for paint/hit (ShowScrollbar=false ⇒ Enabled off).
func (s *ScrollViewport) resolveBar() *Scrollbar {
	b := s.resolveBarMut()
	if !s.ShowScrollbar {
		c := b.Clone()
		c.Enabled = false
		return c
	}
	return b
}

// Layout implements core.Node.
func (s *ScrollViewport) Layout(c core.Constraints) core.Size {
	vert, horiz := s.axes()
	bar := s.resolveBarMut()

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

	// Content insets: when scrollbar enabled and not Overlay, subtract bar size
	// so content/styles never occupy the scrollbar strip.
	insL, insT, insR, insB := s.barInsets(bar, vert, horiz, w, h, false)
	contentW := w - insL - insR
	contentH := h - insT - insB
	if contentW < 1 {
		contentW = 1
	}
	if contentH < 1 {
		contentH = 1
	}

	// Content constraints: unbounded on scroll axes; tight to content box on the other.
	contentC := core.Constraints{
		MaxWidth:  core.Unbounded,
		MaxHeight: core.Unbounded,
	}
	if vert && !horiz && w > 0 {
		contentC.MaxWidth = contentW
	}
	if horiz && !vert && h > 0 {
		contentC.MaxHeight = contentH
	}
	if vert && horiz {
		// 2D: still cap to content box so children don't paint under bars
		if w > 0 {
			contentC.MaxWidth = contentW
		}
		if h > 0 {
			contentC.MaxHeight = contentH
		}
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

	// If still open, size viewport to content + insets.
	if w <= 0 {
		w = cw + insL + insR
	}
	if h <= 0 {
		h = ch + insT + insB
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	s.SetSize(out)
	// Second pass: if Auto/Hover with Overlay=false and overflow just appeared,
	// insets are already stable (reserved whenever policy != Never).
	s.clampScroll()
	s.applyChildOffsets()
	return out
}

// barInsets returns content padding so layout never overlaps the bar strip.
// When Overlay=true, insets are 0 (legacy paint-over; not recommended).
// When Overlay=false (default), reserve Thickness on trailing edges for each
// enabled axis whose visibility is not Never.
func (s *ScrollViewport) barInsets(bar *Scrollbar, vert, horiz bool, viewW, viewH float64, force bool) (left, top, right, bottom float64) {
	if bar == nil || !bar.Enabled || bar.Overlay {
		return
	}
	th := bar.GutterThickness()
	if th <= 0 {
		th = 6
	}
	if vert && bar.Vertical != ScrollbarNever {
		right = th
	}
	if horiz && bar.Horizontal != ScrollbarNever {
		bottom = th
	}
	return
}

// ContentInsets returns the current content-area insets (left, top, right, bottom)
// used so content does not overlap scrollbars.
func (s *ScrollViewport) ContentInsets() (left, top, right, bottom float64) {
	if s == nil {
		return
	}
	vert, horiz := s.axes()
	sz := s.Size()
	return s.barInsets(s.resolveBarMut(), vert, horiz, sz.Width, sz.Height, false)
}

// ContentSize returns the layout box available to children (viewport minus bar gutters).
func (s *ScrollViewport) ContentSize() core.Size {
	if s == nil {
		return core.Size{}
	}
	sz := s.Size()
	l, t, r, b := s.ContentInsets()
	w := sz.Width - l - r
	h := sz.Height - t - b
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return core.Size{Width: w, Height: h}
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
	l, t, r, b := s.ContentInsets()
	// Clip content to the area excluding scrollbar gutters (no style under bars).
	cw := sz.Width - l - r
	ch := sz.Height - t - b
	if cw < 0 {
		cw = 0
	}
	if ch < 0 {
		ch = 0
	}
	pc.PushClipLocal(l, t, cw, ch)
	s.DefaultPaintChildren(pc)
	pc.Pop()
	s.paintScrollbars(pc, sz)
}

func (s *ScrollViewport) paintScrollbars(pc *core.PaintContext, sz core.Size) {
	bar := s.resolveBar()
	vert, horiz := s.axes()
	cs := s.ContentSize()
	overflowY := s.ContentH > cs.Height+0.5
	overflowX := s.ContentW > cs.Width+0.5
	reveal := s.revealLeft > 0
	draggingV := s.dragAxis == 1
	draggingH := s.dragAxis == 2

	showV := vert && bar.shouldShow(bar.Vertical, overflowY, s.hovered, draggingV, reveal)
	showH := horiz && bar.shouldShow(bar.Horizontal, overflowX, s.hovered, draggingH, reveal)
	if !showV && !showH {
		return
	}

	if showV {
		on := s.onBarV || draggingV
		th := bar.thickness(on)
		gutter := bar.GutterThickness()
		if gutter < th {
			gutter = th
		}
		// Align bar to trailing edge inside gutter (margin inward).
		margin := bar.margin()
		pad := bar.padding()
		xTrack := sz.Width - gutter + (gutter-th)/2
		if margin > 0 && th+2*margin <= gutter {
			xTrack = sz.Width - th - margin
		}
		// Track
		if bar.showTrack() {
			pc.FillLocalRoundRect(xTrack, 0, th, sz.Height, bar.trackRadius(th), bar.trackCol(on))
		}
		// Thumb
		x, y, w, h := s.vThumbRect(sz, th, bar.minThumb())
		// Reposition x to expanded bar; inset padding
		x = xTrack + pad
		w = th - 2*pad
		if w < 2 {
			w = th
			x = xTrack
		}
		y += pad
		h -= 2 * pad
		if h < bar.minThumb()/2 {
			h = bar.minThumb() / 2
			if h < 8 {
				h = 8
			}
		}
		pc.FillLocalRoundRect(x, y, w, h, bar.radius(th), bar.thumbCol(on, draggingV))
	}
	if showH {
		on := s.onBarH || draggingH
		th := bar.thickness(on)
		gutter := bar.GutterThickness()
		if gutter < th {
			gutter = th
		}
		margin := bar.margin()
		pad := bar.padding()
		yTrack := sz.Height - gutter + (gutter-th)/2
		if margin > 0 && th+2*margin <= gutter {
			yTrack = sz.Height - th - margin
		}
		if bar.showTrack() {
			pc.FillLocalRoundRect(0, yTrack, sz.Width, th, bar.trackRadius(th), bar.trackCol(on))
		}
		x, y, w, h := s.hThumbRect(sz, th, bar.minThumb())
		y = yTrack + pad
		h = th - 2*pad
		if h < 2 {
			h = th
			y = yTrack
		}
		x += pad
		w -= 2 * pad
		if w < bar.minThumb()/2 {
			w = bar.minThumb() / 2
			if w < 8 {
				w = 8
			}
		}
		pc.FillLocalRoundRect(x, y, w, h, bar.radius(th), bar.thumbCol(on, draggingH))
	}
}

func (s *ScrollViewport) vThumbRect(sz core.Size, bar, minThumb float64) (x, y, w, h float64) {
	viewH := s.ContentSize().Height
	if viewH < 1 {
		viewH = sz.Height
	}
	contentH := s.ContentH
	if contentH < 1 {
		contentH = 1
	}
	h = viewH * viewH / contentH
	if h < minThumb {
		h = minThumb
	}
	maxF := s.resolveBar().maxThumbFraction()
	if maxF < 1 && h > viewH*maxF {
		h = viewH * maxF
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

func (s *ScrollViewport) hThumbRect(sz core.Size, bar, minThumb float64) (x, y, w, h float64) {
	viewW := s.ContentSize().Width
	if viewW < 1 {
		viewW = sz.Width
	}
	contentW := s.ContentW
	if contentW < 1 {
		contentW = 1
	}
	w = viewW * viewW / contentW
	if w < minThumb {
		w = minThumb
	}
	maxF := s.resolveBar().maxThumbFraction()
	if maxF < 1 && w > viewW*maxF {
		w = viewW * maxF
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

// HitTest implements core.Node.
// Scrollbar track/thumb wins over content so drag/track-click reach this node
// (children otherwise cover the full rail width and steal capture).
func (s *ScrollViewport) HitTest(p core.Point) core.Node {
	if !s.LocalBounds().Contains(p) {
		return nil
	}
	if s.hitOnScrollbar(p) {
		return s
	}
	if hit := s.DefaultHitTest(p); hit != nil && hit != s {
		return hit
	}
	return s
}

// hitOnScrollbar reports whether local point p is on the scrollbar strip.
// The strip is the gutter reserved by ContentInsets (and a small slop when overlay).
func (s *ScrollViewport) hitOnScrollbar(p core.Point) bool {
	sz := s.Size()
	bar := s.resolveBar()
	if !bar.Enabled {
		return false
	}
	vert, horiz := s.axes()
	// Content box height used for overflow checks (full viewport height when only V bar).
	l, t, r, b := s.ContentInsets()
	contentH := sz.Height - t - b
	contentW := sz.Width - l - r
	overflowY := s.ContentH > contentH+0.5
	overflowX := s.ContentW > contentW+0.5
	th := bar.GutterThickness()
	if th < 1 {
		th = 6
	}
	slop := th
	if slop < 8 {
		slop = 8
	}
	// Prefer reserved gutter; if Overlay, still use trailing slop when overflowing.
	if vert && bar.Vertical != ScrollbarNever {
		strip := r
		if strip < slop {
			strip = slop
		}
		if (overflowY || bar.Vertical == ScrollbarAlways) && p.X >= sz.Width-strip && p.X <= sz.Width && p.Y >= 0 && p.Y <= sz.Height {
			return true
		}
	}
	if horiz && bar.Horizontal != ScrollbarNever {
		strip := b
		if strip < slop {
			strip = slop
		}
		if (overflowX || bar.Horizontal == ScrollbarAlways) && p.Y >= sz.Height-strip && p.Y <= sz.Height && p.X >= 0 && p.X <= sz.Width {
			return true
		}
	}
	return false
}

// SetHovered implements core hover tracking for Hover visibility policy.
func (s *ScrollViewport) SetHovered(h bool) {
	if s == nil || s.hovered == h {
		return
	}
	s.hovered = h
	if !h {
		s.onBarV, s.onBarH = false, false
	}
	s.MarkNeedsPaint()
}

// Tick decays post-scroll reveal for Hover auto-hide.
func (s *ScrollViewport) Tick(dt float64) bool {
	if s == nil || s.revealLeft <= 0 {
		return false
	}
	s.revealLeft -= dt
	if s.revealLeft <= 0 {
		s.revealLeft = 0
		s.MarkNeedsPaint()
		return false
	}
	return true
}

func (s *ScrollViewport) bumpReveal() {
	bar := s.resolveBar()
	s.revealLeft = bar.autoHideDelay()
	if s.tree != nil {
		s.tree.BindTicker(s, true)
	}
	s.MarkNeedsPaint()
}

// HandlePointer implements thumb drag (pointer capture via tree on Down).
func (s *ScrollViewport) HandlePointer(ev *core.PointerEvent) {
	if s == nil || ev == nil {
		return
	}
	sz := s.Size()
	bar := s.resolveBar()
	thV := bar.GutterThickness()
	thH := bar.GutterThickness()
	thVPaint := bar.thickness(s.onBarV)
	thHPaint := bar.thickness(s.onBarH)
	// Convert absolute event to local.
	abs := core.AbsoluteBounds(s)
	lx := ev.X - abs.Min.X
	ly := ev.Y - abs.Min.Y

	// Track hover over bar regions for thickness expand.
	if ev.Type == core.PointerMove || ev.Type == core.PointerDown {
		s.hovered = true
		cs := s.ContentSize()
		overflowY := s.ContentH > cs.Height+0.5
		overflowX := s.ContentW > cs.Width+0.5
		s.onBarV = overflowY && lx >= sz.Width-thV
		s.onBarH = overflowX && ly >= sz.Height-thH
	}

	switch ev.Type {
	case core.PointerDown:
		if ev.Button != core.ButtonLeft && ev.Button != core.ButtonNone {
			return
		}
		vert, horiz := s.axes()
		if vert && s.ContentH > s.ContentSize().Height+0.5 {
			x, y, w, h := s.vThumbRect(sz, thVPaint, bar.minThumb())
			if bar.dragEnabled() && lx >= x && lx <= x+w && ly >= y && ly <= y+h {
				s.dragAxis = 1
				s.dragStart = ly
				s.dragScroll = s.ScrollY
				ev.Handled = true
				s.MarkNeedsPaint()
				return
			}
			// Track click: jump page
			if bar.trackClickEnabled() && lx >= sz.Width-thV {
				page := s.ContentSize().Height * bar.pageFraction()
				if ly < y {
					s.SetScroll(s.ScrollX, s.ScrollY-page)
				} else if ly > y+h {
					s.SetScroll(s.ScrollX, s.ScrollY+page)
				}
				s.bumpReveal()
				ev.Handled = true
				return
			}
		}
		if horiz && s.ContentW > s.ContentSize().Width+0.5 {
			x, y, w, h := s.hThumbRect(sz, thHPaint, bar.minThumb())
			if bar.dragEnabled() && lx >= x && lx <= x+w && ly >= y && ly <= y+h {
				s.dragAxis = 2
				s.dragStart = lx
				s.dragScroll = s.ScrollX
				ev.Handled = true
				s.MarkNeedsPaint()
				return
			}
			if bar.trackClickEnabled() && ly >= sz.Height-thH {
				page := s.ContentSize().Width * bar.pageFraction()
				if lx < x {
					s.SetScroll(s.ScrollX-page, s.ScrollY)
				} else if lx > x+w {
					s.SetScroll(s.ScrollX+page, s.ScrollY)
				}
				s.bumpReveal()
				ev.Handled = true
				return
			}
		}
	case core.PointerMove:
		if s.dragAxis == 1 {
			_, _, _, th := s.vThumbRect(sz, thV, bar.minThumb())
			track := sz.Height - th
			maxScroll := s.ContentH - s.ContentSize().Height
			if track > 0 && maxScroll > 0 {
				dy := ly - s.dragStart
				s.SetScroll(s.ScrollX, s.dragScroll+dy/track*maxScroll)
			}
			ev.Handled = true
		} else if s.dragAxis == 2 {
			_, _, tw, _ := s.hThumbRect(sz, thH, bar.minThumb())
			track := sz.Width - tw
			maxScroll := s.ContentW - s.ContentSize().Width
			if track > 0 && maxScroll > 0 {
				dx := lx - s.dragStart
				s.SetScroll(s.dragScroll+dx/track*maxScroll, s.ScrollY)
			}
			ev.Handled = true
		} else {
			// Hover paint updates for Hover visibility / thickness.
			s.MarkNeedsPaint()
		}
	case core.PointerUp, core.PointerCancel:
		if s.dragAxis != 0 {
			s.dragAxis = 0
			s.bumpReveal()
			ev.Handled = true
		}
	}
}

// HandleScroll implements core.ScrollHandler (wheel).
func (s *ScrollViewport) HandleScroll(ev *core.ScrollEvent) {
	if ev == nil {
		return
	}
	step := s.resolveBar().wheelStep()
	// Prefer vertical; shift+wheel or DX for horizontal.
	s.ScrollX += ev.DX * step
	s.ScrollY += ev.DY * step
	s.clampScroll()
	s.applyChildOffsets()
	s.bumpReveal()
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

// OverflowY reports whether content is taller than the content box (viewport minus bar gutters).
func (s *ScrollViewport) OverflowY() bool {
	if s == nil {
		return false
	}
	return s.ContentH > s.ContentSize().Height+0.5
}

// OverflowX reports whether content is wider than the content box.
func (s *ScrollViewport) OverflowX() bool {
	if s == nil {
		return false
	}
	return s.ContentW > s.ContentSize().Width+0.5
}

// BarVisible reports whether a scrollbar would paint for the given axis now.
func (s *ScrollViewport) BarVisible(vertical bool) bool {
	if s == nil {
		return false
	}
	bar := s.resolveBar()
	sz := s.Size()
	if vertical {
		return s.axesVert() && bar.shouldShow(bar.Vertical, s.ContentH > sz.Height+0.5, s.hovered, s.dragAxis == 1, s.revealLeft > 0)
	}
	return s.axesHoriz() && bar.shouldShow(bar.Horizontal, s.ContentW > sz.Width+0.5, s.hovered, s.dragAxis == 2, s.revealLeft > 0)
}

func (s *ScrollViewport) axesVert() bool {
	v, _ := s.axes()
	return v
}

func (s *ScrollViewport) axesHoriz() bool {
	_, h := s.axes()
	return h
}

func (s *ScrollViewport) clampScroll() {
	cs := s.ContentSize()
	maxX := s.ContentW - cs.Width
	maxY := s.ContentH - cs.Height
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
