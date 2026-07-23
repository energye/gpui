package primitive

import (
	"math"
	"unsafe"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// ScrollViewport clips children and scrolls them by ScrollX/ScrollY (C-Scroll).
//
// Principle (Flutter Scrollable / ScrollbarPainter):
//   - Layout measures ContentW/H only; never stores scroll in child layout Offset.
//   - Scroll is ContentPaintOffset{-ScrollX,-ScrollY} (paint + hit + AbsoluteBounds).
//   - Thumb extent = f(viewport, content, track); independent of pixels.
//   - Thumb drag freezes extents; ScrollY = scroll0 + ΔpointerAbs/travel*maxScroll.
//
// Paint isolation: ScrollViewport is a RepaintBoundary. Scroll/drag MarkNeedsPaint
// dirties only this layer (not the full window base). Without that, each drag move
// forces NonBoundaryPaintDirty → full base rebuild → “跟手延迟后猛跳”.
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

	// drag freezes bar geometry for one thumb gesture (length cannot thrash).
	drag scrollThumbDrag
}

// scrollThumbDrag freezes bar metrics at pointer-down (Flutter Scrollbar drag).
//
// Principle (same as Flutter ScrollbarPainter + ScrollPosition):
//
//	travel    = trackMain - thumbMain          // frozen
//	maxScroll = content - viewport             // frozen
//	ScrollY   = scroll0 + (ptrAbs - ptr0Abs) / travel * maxScroll
//	thumbY    = ScrollY / maxScroll * travel
//	contentY  = -ScrollY   via ContentPaintOffset (NOT layout Offset)
//
// Pointer is in *event absolute* space so host layout cannot thrash mapping.
type scrollThumbDrag struct {
	axis int // 0 none, 1 Y, 2 X
	// Principle mapping anchors
	ptr0Abs float64 // event X or Y at pointer-down
	scroll0 float64 // Scroll at pointer-down
	// Frozen extents
	viewport  float64
	content   float64
	maxScroll float64
	trackMain float64
	thumbMain float64
	// Frozen cross-axis paint
	cross, pad, margin, trackCross float64
	viewW, viewH                   float64
}

func (d *scrollThumbDrag) active() bool { return d != nil && d.axis != 0 }

func (d *scrollThumbDrag) clear() {
	if d != nil {
		*d = scrollThumbDrag{}
	}
}

// pixelsForPointerAbs is the pure principle formula from frozen anchors.
func (d *scrollThumbDrag) pixelsForPointerAbs(ptrAbs float64) float64 {
	if d == nil || d.maxScroll <= 0 {
		if d == nil {
			return 0
		}
		return d.scroll0
	}
	travel := d.trackMain - d.thumbMain
	if travel <= 1e-6 {
		return d.scroll0
	}
	px := d.scroll0 + (ptrAbs-d.ptr0Abs)/travel*d.maxScroll
	if px < 0 {
		return 0
	}
	if px > d.maxScroll {
		return d.maxScroll
	}
	return px
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
	// Isolate scroll/drag paint so the compositor keeps the window base RT
	// (Flutter RepaintBoundary). Required for smooth thumb drag.
	s.SetRepaintBoundary(true)
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

// ContentPaintOffset implements core.PaintOffsetParent.
// Scroll is a paint/hit transform only — never mutates child layout Offset
// (Flutter Scrollable; avoids fighting Flex SetOffset → bottom thrash).
func (s *ScrollViewport) ContentPaintOffset() core.Point {
	if s == nil {
		return core.Point{}
	}
	return core.Point{X: -s.ScrollX, Y: -s.ScrollY}
}

// Dragging reports an active thumb drag (for hosts/tests).
func (s *ScrollViewport) Dragging() bool {
	return s != nil && s.drag.active()
}

// ThumbMainLength is the painted thumb extent along the scroll axis (for tests/diagnostics).
func (s *ScrollViewport) ThumbMainLength(vertical bool) float64 {
	if s == nil {
		return 0
	}
	sz := s.Size()
	if vertical {
		_, _, _, _, h := s.vThumbGeom(sz)
		return h
	}
	_, _, _, w, _ := s.hThumbGeom(sz)
	return w
}

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
// Avoids Clone() on the paint/hit hot path (drag frames allocate nothing for bar policy).
func (s *ScrollViewport) resolveBar() *Scrollbar {
	b := s.resolveBarMut()
	if s.ShowScrollbar {
		return b
	}
	// Shared disabled view — callers must not mutate. Hot path never allocates.
	return scrollbarDisabledView
}

// scrollbarDisabledView is a read-only "bars off" policy (no per-call Clone).
var scrollbarDisabledView = &Scrollbar{Enabled: false}

// Layout implements core.Node.
func (s *ScrollViewport) Layout(c core.Constraints) core.Size {
	vert, horiz := s.axes()
	bar := s.resolveBarMut()

	// Viewport size first (from fixed fields or parent).
	// During drag we still accept parent size so Flex/StretchChild placement is
	// stable (freezing outer size fought the rail and made the whole tab column 窜).
	// Only content extent is frozen (no child remeasure).
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

	if s.drag.active() {
		// Outer size from parent; content metrics frozen at pointer-down.
		if w <= 0 {
			w = s.drag.viewW
		}
		if h <= 0 {
			h = s.drag.viewH
		}
		out := c.Tighten(core.Size{Width: w, Height: h})
		s.SetSize(out)
		if s.drag.axis == 1 {
			s.ContentH = s.drag.content
		} else if s.drag.axis == 2 {
			s.ContentW = s.drag.content
		}
		// Do not layout children — preserves layout Offset chain (paint scroll only).
		s.RememberConstraints(c)
		return out
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
	s.RememberConstraints(c)
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

// barMetrics is the Flutter ScrollMetrics subset used by the scrollbar painter.
// Thumb extent is a pure function of (viewport, content, trackMain) — independent
// of pixels — matching ScrollbarPainter._thumbExtent.
type barMetrics struct {
	viewport  float64 // viewportDimension
	content   float64 // scrollExtent + viewportDimension ≈ ContentH/W
	maxScroll float64 // maxScrollExtent
	trackMain float64 // painted track length along main axis
	thumbMain float64 // thumb extent along main axis
}

// flutterThumbExtent — ScrollbarPainter: track * viewport / content, clamped.
func flutterThumbExtent(trackMain, viewport, content, minThumb, maxFrac float64) float64 {
	if trackMain < 1 {
		trackMain = 1
	}
	if content < viewport {
		content = viewport
	}
	if content <= viewport+1e-6 {
		return trackMain
	}
	if minThumb < 1 {
		minThumb = 20
	}
	// fractionVisible = viewport / content
	ext := trackMain * (viewport / content)
	if ext < minThumb {
		ext = minThumb
	}
	if maxFrac > 0 && maxFrac < 1 && ext > trackMain*maxFrac {
		ext = trackMain * maxFrac
	}
	if ext > trackMain {
		ext = trackMain
	}
	return ext
}

func flutterThumbOffset(pixels, maxScroll, trackMain, thumbMain float64) float64 {
	travel := trackMain - thumbMain
	if maxScroll <= 0 || travel <= 0 {
		return 0
	}
	// Linear map; hard clamp only. No epsilon snap (snap caused visible 窜 near end).
	y := pixels / maxScroll * travel
	if y < 0 {
		return 0
	}
	if y > travel {
		return travel
	}
	return y
}

// computeBarMetrics builds painter metrics for one axis.
// trackMain is the painted track length (outer size along axis, minus main-axis margin*2).
// viewport is the content-box size (Flutter viewportDimension).
func (s *ScrollViewport) computeBarMetrics(vertical bool, outerMain float64) barMetrics {
	bar := s.resolveBar()
	cs := s.ContentSize()
	var viewport, content float64
	if vertical {
		viewport, content = cs.Height, s.ContentH
	} else {
		viewport, content = cs.Width, s.ContentW
	}
	if viewport < 1 {
		viewport = 1
	}
	if content < 1 {
		content = 1
	}
	// Track main length = outer widget size along the axis (full visible height/width).
	// Content viewport may equal outer for single-axis bars; thumb + track share trackMain
	// so the rail visually seats flush top/bottom (no multi-pixel gap).
	_ = bar.margin()
	trackMain := outerMain
	if trackMain < 1 {
		trackMain = viewport
	}
	if trackMain < 1 {
		trackMain = 1
	}
	// maxScroll still uses content-box viewport (how much content can move).
	// thumb extent uses trackMain with visible fraction viewport/content.
	maxScroll := content - viewport
	if maxScroll < 0 {
		maxScroll = 0
	}
	thumb := flutterThumbExtent(trackMain, viewport, content, bar.minThumb(), bar.maxThumbFraction())
	return barMetrics{
		viewport:  viewport,
		content:   content,
		maxScroll: maxScroll,
		trackMain: trackMain,
		thumbMain: thumb,
	}
}

// metricsForPaint returns frozen metrics while dragging, else live compute.
func (s *ScrollViewport) metricsForPaint(vertical bool, outerMain float64) barMetrics {
	if vertical && s.drag.axis == 1 {
		return barMetrics{
			viewport:  s.drag.viewport,
			content:   s.drag.content,
			maxScroll: s.drag.maxScroll,
			trackMain: s.drag.trackMain,
			thumbMain: s.drag.thumbMain,
		}
	}
	if !vertical && s.drag.axis == 2 {
		return barMetrics{
			viewport:  s.drag.viewport,
			content:   s.drag.content,
			maxScroll: s.drag.maxScroll,
			trackMain: s.drag.trackMain,
			thumbMain: s.drag.thumbMain,
		}
	}
	return s.computeBarMetrics(vertical, outerMain)
}

// vThumbGeom returns vertical bar geometry.
// Flutter: thumb MAIN length is independent of ScrollY; during drag it is frozen.
func (s *ScrollViewport) vThumbGeom(sz core.Size) (trackX, thumbX, thumbY, thumbW, thumbH float64) {
	if s.drag.axis == 1 {
		// Exact freeze — no live thickness/ContentH/Size.
		trackX = s.drag.trackCross
		pad, cross := s.drag.pad, s.drag.cross
		thumbX = trackX + pad
		thumbW = cross - 2*pad
		if thumbW < 2 {
			thumbW = cross
			thumbX = trackX
		}
		thumbH = s.drag.thumbMain
		thumbY = flutterThumbOffset(s.ScrollY, s.drag.maxScroll, s.drag.trackMain, s.drag.thumbMain)
		return trackX, thumbX, thumbY, thumbW, thumbH
	}
	bar := s.resolveBar()
	on := s.onBarV
	th := bar.thickness(on)
	gutter := bar.GutterThickness()
	if gutter < th {
		gutter = th
	}
	margin := bar.margin()
	pad := bar.padding()
	trackX = sz.Width - gutter + (gutter-th)/2
	if margin > 0 && th+2*margin <= gutter {
		trackX = sz.Width - th - margin
	}
	thumbX = trackX + pad
	thumbW = th - 2*pad
	if thumbW < 2 {
		thumbW = th
		thumbX = trackX
	}
	m := s.computeBarMetrics(true, sz.Height)
	thumbY = flutterThumbOffset(s.ScrollY, m.maxScroll, m.trackMain, m.thumbMain)
	thumbH = m.thumbMain
	return trackX, thumbX, thumbY, thumbW, thumbH
}

func (s *ScrollViewport) hThumbGeom(sz core.Size) (trackY, thumbX, thumbY, thumbW, thumbH float64) {
	if s.drag.axis == 2 {
		trackY = s.drag.trackCross
		pad, cross := s.drag.pad, s.drag.cross
		thumbY = trackY + pad
		thumbH = cross - 2*pad
		if thumbH < 2 {
			thumbH = cross
			thumbY = trackY
		}
		thumbW = s.drag.thumbMain
		thumbX = flutterThumbOffset(s.ScrollX, s.drag.maxScroll, s.drag.trackMain, s.drag.thumbMain)
		return trackY, thumbX, thumbY, thumbW, thumbH
	}
	bar := s.resolveBar()
	on := s.onBarH
	th := bar.thickness(on)
	gutter := bar.GutterThickness()
	if gutter < th {
		gutter = th
	}
	margin := bar.margin()
	pad := bar.padding()
	trackY = sz.Height - gutter + (gutter-th)/2
	if margin > 0 && th+2*margin <= gutter {
		trackY = sz.Height - th - margin
	}
	thumbY = trackY + pad
	thumbH = th - 2*pad
	if thumbH < 2 {
		thumbH = th
		thumbY = trackY
	}
	m := s.computeBarMetrics(false, sz.Width)
	thumbX = flutterThumbOffset(s.ScrollX, m.maxScroll, m.trackMain, m.thumbMain)
	thumbW = m.thumbMain
	return trackY, thumbX, thumbY, thumbW, thumbH
}

func (s *ScrollViewport) cacheKey() uintptr {
	return uintptr(unsafe.Pointer(s))
}

// Paint implements core.Node. With LayerCache, content+chrome live in a retained
// layer so thumb drag re-rasterizes only this viewport (not the full window).
func (s *ScrollViewport) Paint(pc *core.PaintContext) {
	if pc == nil {
		return
	}
	sz := s.Size()
	w := int(math.Ceil(sz.Width))
	h := int(math.Ceil(sz.Height))
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	scale := pc.Scale
	if scale <= 0 {
		scale = 1
	}
	key := s.cacheKey()
	ox, oy := pc.Origin.X, pc.Origin.Y

	if pc.LayerCache != nil {
		// Re-rasterize only when this viewport (or nested content) is dirty.
		// ForceFullPaint alone must NOT rebuild a clean scroll layer (thumb lag with animations).
		// Nested Spin painted into this layer still triggers via scrollDescendantPaintDirty.
		needRaster := s.NeedsPaint() || scrollDescendantPaintDirty(s)
		if !needRaster {
			if !pc.LayerCache.BlitBoundary(key, nil, ox, oy, w, h) {
				needRaster = true
			}
		}
		if needRaster {
			ok := pc.LayerCache.RasterizeBoundary(key, pc.DC, ox, oy, w, h, scale, func(childPC *core.PaintContext) {
				if childPC == nil {
					return
				}
				if pc.Theme != nil {
					childPC.Theme = pc.Theme
				}
				s.paintBody(childPC, sz)
			})
			if !ok {
				s.paintDirect(pc, sz)
				return
			}
		}
		if !pc.DeferLayerBlit && pc.DC != nil {
			_ = pc.LayerCache.BlitBoundary(key, pc.DC, ox, oy, w, h)
		}
		s.ClearPaintDirty()
		return
	}

	s.paintDirect(pc, sz)
}

func (s *ScrollViewport) paintDirect(pc *core.PaintContext, sz core.Size) {
	if pc.CompositeOnly && !s.NeedsPaint() && !pc.ForceFullPaint && !scrollDescendantPaintDirty(s) {
		s.ClearPaintDirty()
		return
	}
	childPC := pc
	if s.NeedsPaint() || pc.ForceFullPaint {
		childPC = pc.WithForceFullPaint()
		childPC.CompositeOnly = false
	}
	s.paintBody(childPC, sz)
	s.ClearPaintDirty()
}

// paintBody draws clipped content + scrollbar chrome in the given paint space
// (parent origin or offscreen RT local origin).
func (s *ScrollViewport) paintBody(pc *core.PaintContext, sz core.Size) {
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

// scrollDescendantPaintDirty reports paint dirty under s (not including s).
func scrollDescendantPaintDirty(s *ScrollViewport) bool {
	if s == nil {
		return false
	}
	for _, c := range s.Children() {
		if nodeOrDescPaintDirty(c) {
			return true
		}
	}
	return false
}

func nodeOrDescPaintDirty(n core.Node) bool {
	if n == nil {
		return false
	}
	b := n.Base()
	if b.NeedsPaint() {
		return true
	}
	for _, c := range b.Children() {
		if nodeOrDescPaintDirty(c) {
			return true
		}
	}
	return false
}

func (s *ScrollViewport) paintScrollbars(pc *core.PaintContext, sz core.Size) {
	bar := s.resolveBar()
	vert, horiz := s.axes()
	cs := s.ContentSize()
	// During drag use frozen content for overflow so chrome does not flicker.
	contentH, contentW := s.ContentH, s.ContentW
	if s.drag.axis == 1 {
		contentH = s.drag.content
	} else if s.drag.axis == 2 {
		contentW = s.drag.content
	}
	overflowY := contentH > cs.Height+0.5
	overflowX := contentW > cs.Width+0.5
	reveal := s.revealLeft > 0
	draggingV := s.drag.axis == 1
	draggingH := s.drag.axis == 2

	showV := vert && bar.shouldShow(bar.Vertical, overflowY, s.hovered, draggingV, reveal)
	showH := horiz && bar.shouldShow(bar.Horizontal, overflowX, s.hovered, draggingH, reveal)
	if !showV && !showH {
		return
	}

	if showV {
		on := s.onBarV || draggingV
		trackX, thumbX, thumbY, thumbW, thumbH := s.vThumbGeom(sz)
		th := bar.thickness(on)
		if draggingV {
			th = s.drag.cross
			trackX = s.drag.trackCross
		}
		if bar.showTrack() {
			pc.FillLocalRoundRect(trackX, 0, th, sz.Height, bar.trackRadius(th), bar.trackCol(on))
		}
		pc.FillLocalRoundRect(thumbX, thumbY, thumbW, thumbH, bar.radius(th), bar.thumbCol(on, draggingV))
	}
	if showH {
		on := s.onBarH || draggingH
		trackY, thumbX, thumbY, thumbW, thumbH := s.hThumbGeom(sz)
		th := bar.thickness(on)
		if draggingH {
			th = s.drag.cross
			trackY = s.drag.trackCross
		}
		if bar.showTrack() {
			pc.FillLocalRoundRect(0, trackY, sz.Width, th, bar.trackRadius(th), bar.trackCol(on))
		}
		pc.FillLocalRoundRect(thumbX, thumbY, thumbW, thumbH, bar.radius(th), bar.thumbCol(on, draggingH))
	}
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
// Drag-move is a hot path: no AbsoluteBounds, no bar policy resolve, no hover work.
func (s *ScrollViewport) HandlePointer(ev *core.PointerEvent) {
	if s == nil || ev == nil {
		return
	}

	// ---- drag hot path (must stay cheap: thumb lag is mostly paint cost + here) ----
	if s.drag.axis == 1 {
		switch ev.Type {
		case core.PointerMove:
			next := s.drag.pixelsForPointerAbs(ev.Y)
			if next != s.ScrollY {
				s.ScrollY = next
				s.MarkNeedsPaint()
			}
			ev.Handled = true
			return
		case core.PointerUp, core.PointerCancel:
			s.drag.clear()
			s.bumpReveal()
			ev.Handled = true
			return
		}
	} else if s.drag.axis == 2 {
		switch ev.Type {
		case core.PointerMove:
			next := s.drag.pixelsForPointerAbs(ev.X)
			if next != s.ScrollX {
				s.ScrollX = next
				s.MarkNeedsPaint()
			}
			ev.Handled = true
			return
		case core.PointerUp, core.PointerCancel:
			s.drag.clear()
			s.bumpReveal()
			ev.Handled = true
			return
		}
	}

	sz := s.Size()
	bar := s.resolveBar()
	thV := bar.GutterThickness()
	thH := thV
	// Local coords for hit-testing against bar geometry (widget space).
	abs := core.AbsoluteBounds(s)
	lx := ev.X - abs.Min.X
	ly := ev.Y - abs.Min.Y

	// Track hover over bar regions for thickness expand (paint only on change).
	if ev.Type == core.PointerMove || ev.Type == core.PointerDown {
		prevV, prevH := s.onBarV, s.onBarH
		s.hovered = true
		cs := s.ContentSize()
		overflowY := s.ContentH > cs.Height+0.5
		overflowX := s.ContentW > cs.Width+0.5
		s.onBarV = overflowY && lx >= sz.Width-thV
		s.onBarH = overflowX && ly >= sz.Height-thH
		if s.onBarV != prevV || s.onBarH != prevH {
			s.MarkNeedsPaint()
		}
	}

	switch ev.Type {
	case core.PointerDown:
		if ev.Button != core.ButtonLeft && ev.Button != core.ButtonNone {
			return
		}
		vert, horiz := s.axes()
		if vert && s.ContentH > s.ContentSize().Height+0.5 {
			_, _, thumbY, _, thumbH := s.vThumbGeom(sz)
			// Full-gutter hit on Y span (thin painted thumb is easy to miss).
			inStrip := lx >= sz.Width-thV && lx <= sz.Width
			inThumb := ly >= thumbY && ly <= thumbY+thumbH
			if bar.dragEnabled() && inStrip && inThumb {
				s.beginThumbDrag(1, ev.Y, sz) // event absolute Y
				ev.Handled = true
				s.MarkNeedsPaint()
				return
			}
			if bar.trackClickEnabled() && inStrip {
				page := s.ContentSize().Height * bar.pageFraction()
				if ly < thumbY {
					s.SetScroll(s.ScrollX, s.ScrollY-page)
				} else if ly > thumbY+thumbH {
					s.SetScroll(s.ScrollX, s.ScrollY+page)
				}
				s.bumpReveal()
				ev.Handled = true
				return
			}
		}
		if horiz && s.ContentW > s.ContentSize().Width+0.5 {
			_, thumbX, _, thumbW, _ := s.hThumbGeom(sz)
			inStrip := ly >= sz.Height-thH && ly <= sz.Height
			inThumb := lx >= thumbX && lx <= thumbX+thumbW
			if bar.dragEnabled() && inStrip && inThumb {
				s.beginThumbDrag(2, ev.X, sz) // event absolute X
				ev.Handled = true
				s.MarkNeedsPaint()
				return
			}
			if bar.trackClickEnabled() && inStrip {
				page := s.ContentSize().Width * bar.pageFraction()
				if lx < thumbX {
					s.SetScroll(s.ScrollX-page, s.ScrollY)
				} else if lx > thumbX+thumbW {
					s.SetScroll(s.ScrollX+page, s.ScrollY)
				}
				s.bumpReveal()
				ev.Handled = true
				return
			}
		}
	}
}

// beginThumbDrag freezes metrics + paint sizes; ptrAbs is event.X or event.Y.
func (s *ScrollViewport) beginThumbDrag(axis int, ptrAbs float64, sz core.Size) {
	outer := sz.Height
	if axis == 2 {
		outer = sz.Width
	}
	m := s.computeBarMetrics(axis == 1, outer)
	bar := s.resolveBar()
	cross := bar.thickness(true) // freeze expanded thickness for gesture
	pad := bar.padding()
	margin := bar.margin()
	gutter := bar.GutterThickness()
	if gutter < cross {
		gutter = cross
	}
	var trackCross float64
	if axis == 1 {
		trackCross = sz.Width - gutter + (gutter-cross)/2
		if margin > 0 && cross+2*margin <= gutter {
			trackCross = sz.Width - cross - margin
		}
	} else {
		trackCross = sz.Height - gutter + (gutter-cross)/2
		if margin > 0 && cross+2*margin <= gutter {
			trackCross = sz.Height - cross - margin
		}
	}
	scroll0 := s.ScrollY
	if axis == 2 {
		scroll0 = s.ScrollX
	}
	s.drag = scrollThumbDrag{
		axis:       axis,
		ptr0Abs:    ptrAbs,
		scroll0:    scroll0,
		viewport:   m.viewport,
		content:    m.content,
		maxScroll:  m.maxScroll,
		trackMain:  m.trackMain,
		thumbMain:  m.thumbMain,
		cross:      cross,
		pad:        pad,
		margin:     margin,
		trackCross: trackCross,
		viewW:      sz.Width,
		viewH:      sz.Height,
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
	s.bumpReveal()
	// Paint-only: ContentPaintOffset updates scroll transform.
	s.MarkNeedsPaint()
	ev.Handled = true
}

// SetScroll sets offsets and clamps. No-op (no paint) when clamped values are unchanged.
func (s *ScrollViewport) SetScroll(x, y float64) {
	if s == nil {
		return
	}
	prevX, prevY := s.ScrollX, s.ScrollY
	s.ScrollX, s.ScrollY = x, y
	s.clampScroll()
	if s.ScrollX == prevX && s.ScrollY == prevY {
		return
	}
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
		return s.axesVert() && bar.shouldShow(bar.Vertical, s.ContentH > sz.Height+0.5, s.hovered, s.drag.axis == 1, s.revealLeft > 0)
	}
	return s.axesHoriz() && bar.shouldShow(bar.Horizontal, s.ContentW > sz.Width+0.5, s.hovered, s.drag.axis == 2, s.revealLeft > 0)
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
	// Principle: while dragging, maxScroll is frozen — never recompute from live layout.
	if s.drag.axis == 1 {
		if s.ScrollY < 0 {
			s.ScrollY = 0
		} else if s.ScrollY > s.drag.maxScroll {
			s.ScrollY = s.drag.maxScroll
		}
		s.ScrollX = 0
		return
	}
	if s.drag.axis == 2 {
		if s.ScrollX < 0 {
			s.ScrollX = 0
		} else if s.ScrollX > s.drag.maxScroll {
			s.ScrollX = s.drag.maxScroll
		}
		s.ScrollY = 0
		return
	}
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
	} else if s.ScrollX > maxX {
		s.ScrollX = maxX
	}
	if s.ScrollY < 0 {
		s.ScrollY = 0
	} else if s.ScrollY > maxY {
		s.ScrollY = maxY
	}
}
