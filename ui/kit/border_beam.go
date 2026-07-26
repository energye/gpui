package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Ant Design BorderBeam defaults — docs/antd/border-beam.md §6.2
// components/border-beam/util.ts + style/index.ts
const (
	// DefaultBorderBeamDuration is antd duration default (seconds per loop).
	DefaultBorderBeamDuration = 6.0
	// DefaultBorderBeamSize is antd size default (visible beam segment, px).
	DefaultBorderBeamSize = 100.0
	// DefaultBorderBeamLineWidth is antd lineWidth default (px).
	DefaultBorderBeamLineWidth = 1.0
	// DefaultBorderBeamOutset is kit fallback when outset is unset (px).
	// antd uses child border width as negative inset; kit defaults to 0 (flush).
	DefaultBorderBeamOutset = 0.0
	// DefaultBorderBeamRadius is Token borderRadius fallback.
	DefaultBorderBeamRadius = 6.0
	// DefaultBorderBeamFontSize is §6.2 content fontSize (children only).
	DefaultBorderBeamFontSize = 14.0
	// MaxBorderBeamColorStopPercent maps user 0..100 into the visible beam head.
	// Trailing (100-70)=30% is reserved for transparent fade (antd util.ts).
	MaxBorderBeamColorStopPercent = 70.0
	// beamPaintSamples is the number of short strokes along the beam segment.
	beamPaintSamples = 28
)

// BorderBeamColorStop is one gradient stop (antd { color, percent }).
// Percent is the public 0..100 input range.
type BorderBeamColorStop struct {
	Color   render.RGBA
	Percent float64
}

// BorderBeam is Ant Design BorderBeam — decorative moving highlight along a border.
//
//	borderBeamHost (HitDefer · layout = children box)
//	  ├─ child (interactive content)
//	  └─ beamLayer (HitTransparent · presentation)
//
// Product contract: docs/antd/border-beam.md §6 (P0).
// Animation uses Host Tick (not a second frame loop). ReduceMotion hides the beam.
type BorderBeam struct {
	Root *borderBeamHost

	child core.Node
	layer *borderBeamLayer

	// Duration is seconds per full loop. ≤0 → DefaultBorderBeamDuration (6).
	Duration float64
	// Size is the visible beam segment length in px. ≤0 → DefaultBorderBeamSize (100).
	// NOTE: this is NOT control size small|middle|large — antd names it "size".
	Size float64
	// LineWidth is stroke width in px. ≤0 → DefaultBorderBeamLineWidth (1).
	LineWidth float64
	// Outset expands the beam path outward from the host edge (px).
	Outset    float64
	outsetSet bool
	// BorderRadius is the path corner radius. ≤0 and unset → Token / default 6.
	BorderRadius float64
	radiusSet    bool

	// Color stops (empty → Theme primary gradient).
	stops    []BorderBeamColorStop
	colorSet bool

	// ShowOnHover maps hover.tsx: hide beam until hovered.
	ShowOnHover bool
	// hovered is set via SetHovered / host hoverable / tree hover ancestry.
	hovered bool

	Theme     *core.Theme
	Style     Style
	AriaLabel string

	phase float64 // 0..1 along perimeter
	life  tickerLifecycle
}

// borderBeamHost sizes to children; beam layer paints above with HitTransparent.
type borderBeamHost struct {
	core.NodeBase
	bb    *BorderBeam
	child core.Node
	layer *borderBeamLayer
}

func (h *borderBeamHost) TypeID() string { return "kit.BorderBeam" }

func (h *borderBeamHost) Layout(c core.Constraints) core.Size {
	if h == nil {
		return core.Size{}
	}
	if sz, ok := h.LayoutSkipIfClean(c); ok {
		return sz
	}
	var csz core.Size
	if h.child != nil {
		csz = h.child.Layout(c)
		h.child.Base().SetOffset(core.Point{})
	}
	out := c.Tighten(csz)
	if c.IsTight() {
		out = core.Size{Width: c.MaxWidth, Height: c.MaxHeight}
		if h.child != nil {
			_ = h.child.Layout(core.Tight(out.Width, out.Height))
			h.child.Base().SetOffset(core.Point{})
		}
	}
	if h.layer != nil {
		_ = h.layer.Layout(core.Tight(out.Width, out.Height))
		h.layer.Base().SetOffset(core.Point{})
	}
	h.SetSize(out)
	h.RememberConstraints(c)
	h.ClearLayoutDirty()
	return out
}

func (h *borderBeamHost) Paint(pc *core.PaintContext) {
	if h == nil {
		return
	}
	h.DefaultPaintChildren(pc)
	if pc != nil {
		h.ClearPaintDirty()
	}
}

func (h *borderBeamHost) HitTest(p core.Point) core.Node {
	if h == nil {
		return nil
	}
	// Prefer children so interactive content keeps hits (BB-S8).
	if h.child != nil {
		if hit := h.child.HitTest(p.Sub(h.child.Base().Offset())); hit != nil {
			return hit
		}
	}
	// Host is HitDefer-like: no self hit when empty.
	return nil
}

// SetHovered implements tree hoverable when the host itself is the hover target.
func (h *borderBeamHost) SetHovered(v bool) {
	if h == nil || h.bb == nil {
		return
	}
	h.bb.SetHovered(v)
}

func (h *borderBeamHost) OnMount() {
	if h == nil || h.bb == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.bb.life.attach(t, h.bb, h.bb.needsTicker())
	}
}

func (h *borderBeamHost) OnUnmount() {
	if h == nil || h.bb == nil {
		return
	}
	h.bb.life.unmount()
}

// borderBeamLayer paints the moving beam; never takes hits.
type borderBeamLayer struct {
	core.NodeBase
	bb *BorderBeam
}

func (l *borderBeamLayer) TypeID() string { return "kit.BorderBeamLayer" }

func (l *borderBeamLayer) Layout(c core.Constraints) core.Size {
	out := c.Tighten(core.Size{Width: c.MaxWidth, Height: c.MaxHeight})
	if c.MaxWidth > 0 && c.MaxHeight > 0 {
		out = core.Size{Width: c.MaxWidth, Height: c.MaxHeight}
	}
	l.SetSize(out)
	return out
}

func (l *borderBeamLayer) Paint(pc *core.PaintContext) {
	if l == nil || l.bb == nil || pc == nil {
		return
	}
	// Hover ancestry may change without SetHovered on host — re-evaluate visibility.
	if l.bb.ShowOnHover {
		l.bb.refreshHoverFromTree()
	}
	if l.bb.IsBeamVisible() {
		l.bb.paintBeam(pc, l.Size())
	}
	l.ClearPaintDirty()
}

func (l *borderBeamLayer) HitTest(core.Point) core.Node { return nil }

// NewBorderBeam wraps child with a border beam overlay.
// Defaults: duration=6, size=100, lineWidth=1, outset=0, Theme primary gradient.
func NewBorderBeam(child core.Node) *BorderBeam {
	b := &BorderBeam{
		child:     child,
		Duration:  DefaultBorderBeamDuration,
		Size:      DefaultBorderBeamSize,
		LineWidth: DefaultBorderBeamLineWidth,
	}
	b.rebuild()
	return b
}

// Node returns the mount root.

// ensureBuilt materializes the control tree if missing (#9).
func (b *BorderBeam) ensureBuilt() {
	if b == nil {
		return
	}
	if b.Root == nil {
		b.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (b *BorderBeam) structureChange() {
	if b == nil {
		return
	}
	b.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (b *BorderBeam) chromeChange() {
	if b == nil {
		return
	}
	b.ensureBuilt()
	b.rebuild()
}

func (b *BorderBeam) Node() core.Node {
	if b == nil {
		return nil
	}
	b.ensureBuilt()
	return b.Root
}

// Child returns the wrapped content.
func (b *BorderBeam) Child() core.Node {
	if b == nil {
		return nil
	}
	return b.child
}

// Phase returns the animation phase in [0,1).
func (b *BorderBeam) Phase() float64 {
	if b == nil {
		return 0
	}
	return b.phase
}

// AttachTicker registers the beam animation ticker.
func (b *BorderBeam) AttachTicker(t *core.Tree) {
	if b != nil {
		b.life.attach(t, b, b.needsTicker())
	}
}

// Tick advances phase when the beam is visible and motion is allowed.
func (b *BorderBeam) Tick(dt float64) bool {
	if b == nil {
		return false
	}
	var nt *core.Tree
	if b.Root != nil {
		nt = b.Root.Tree()
	}
	if !b.life.stillMounted(nt) {
		return false
	}
	if b.ShowOnHover {
		b.refreshHoverFromTree()
	}
	if !b.IsBeamVisible() {
		return b.needsTicker()
	}
	if b.reduceMotion(nt) {
		return false
	}
	d := b.ResolvedDuration()
	if d <= 0 {
		d = DefaultBorderBeamDuration
	}
	b.phase += dt / d
	if b.phase >= 1 {
		b.phase -= math.Floor(b.phase)
	}
	if b.layer != nil {
		b.layer.MarkNeedsPaint()
	} else if b.Root != nil {
		b.Root.MarkNeedsPaint()
	}
	return b.needsTicker()
}

func (b *BorderBeam) reduceMotion(nt *core.Tree) bool {
	if nt != nil && nt.Clock() != nil && nt.Clock().ReduceMotion {
		return true
	}
	if b.life.tree != nil && b.life.tree.Clock() != nil && b.life.tree.Clock().ReduceMotion {
		return true
	}
	return false
}

func (b *BorderBeam) needsTicker() bool {
	if b == nil {
		return false
	}
	// Poll hover when show-on-hover so enter/leave can re-arm without idle spin
	// when not hovered: only tick while visible (or while showOnHover waits for
	// first paint-driven refresh). Prefer paint-time refresh + active only when visible.
	if !b.IsBeamVisible() {
		return false
	}
	var nt *core.Tree
	if b.Root != nil {
		nt = b.Root.Tree()
	}
	if b.reduceMotion(nt) {
		return false
	}
	return true
}

func (b *BorderBeam) syncTicker() {
	b.life.setActive(b.needsTicker())
}

// IsBeamVisible reports whether the beam should paint (BB-S1/S2/S6).
func (b *BorderBeam) IsBeamVisible() bool {
	if b == nil {
		return false
	}
	var nt *core.Tree
	if b.Root != nil {
		nt = b.Root.Tree()
	}
	if b.reduceMotion(nt) {
		return false
	}
	if b.ShowOnHover && !b.effectiveHover() {
		return false
	}
	return true
}

func (b *BorderBeam) effectiveHover() bool {
	if b == nil {
		return false
	}
	if b.hovered {
		return true
	}
	return b.hoverFromTree()
}

func (b *BorderBeam) hoverFromTree() bool {
	if b == nil || b.Root == nil {
		return false
	}
	t := b.Root.Tree()
	if t == nil {
		return false
	}
	for n := t.Hover(); n != nil; n = n.Parent() {
		if n == b.Root {
			return true
		}
	}
	return false
}

func (b *BorderBeam) refreshHoverFromTree() {
	if b == nil {
		return
	}
	// tree.Hover() ancestry is live; re-arm ticker when paint sees hover enter/leave.
	b.syncTicker()
}

// SetChild replaces the wrapped content.
func (b *BorderBeam) SetChild(n core.Node) {
	if b == nil {
		return
	}
	b.child = n
	b.rebuild()
}

// SetDuration sets seconds per loop (≤0 restores default 6).
func (b *BorderBeam) SetDuration(seconds float64) {
	if b == nil {
		return
	}
	b.Duration = seconds
	b.syncTicker()
	b.markPaint()
}

// SetSize sets the visible beam segment length in px (antd size).
func (b *BorderBeam) SetSize(px float64) {
	if b == nil {
		return
	}
	b.Size = px
	b.markPaint()
}

// SetLineWidth sets stroke width in px.
func (b *BorderBeam) SetLineWidth(px float64) {
	if b == nil {
		return
	}
	b.LineWidth = px
	b.markPaint()
}

// SetOutset sets outward expansion (explicit, including 0).
func (b *BorderBeam) SetOutset(px float64) {
	if b == nil {
		return
	}
	b.Outset = px
	b.outsetSet = true
	b.markPaint()
}

// ClearOutset restores unset outset (default 0).
func (b *BorderBeam) ClearOutset() {
	if b == nil {
		return
	}
	b.outsetSet = false
	b.Outset = 0
	b.markPaint()
}

// SetBorderRadius sets path corner radius (explicit, including 0).
func (b *BorderBeam) SetBorderRadius(px float64) {
	if b == nil {
		return
	}
	b.BorderRadius = px
	b.radiusSet = true
	b.markPaint()
}

// SetColor sets a solid beam color (mapped as a single stop at 0%).
func (b *BorderBeam) SetColor(c render.RGBA) {
	if b == nil {
		return
	}
	b.stops = []BorderBeamColorStop{{Color: c, Percent: 0}}
	b.colorSet = true
	b.markPaint()
}

// SetColorStops sets gradient stops (percent 0..100 public range).
func (b *BorderBeam) SetColorStops(stops ...BorderBeamColorStop) {
	if b == nil {
		return
	}
	b.stops = append([]BorderBeamColorStop(nil), stops...)
	b.colorSet = true
	b.markPaint()
}

// ClearColor restores Theme primary gradient default.
func (b *BorderBeam) ClearColor() {
	if b == nil {
		return
	}
	b.stops = nil
	b.colorSet = false
	b.markPaint()
}

// SetShowOnHover enables hover.tsx behavior.
func (b *BorderBeam) SetShowOnHover(v bool) {
	if b == nil {
		return
	}
	b.ShowOnHover = v
	b.syncTicker()
	b.markPaint()
}

// SetHovered injects hover state (tests / host).
func (b *BorderBeam) SetHovered(v bool) {
	if b == nil {
		return
	}
	if b.hovered == v {
		return
	}
	b.hovered = v
	b.syncTicker()
	b.markPaint()
}

// SetTheme sets the theme override.
func (b *BorderBeam) SetTheme(th *core.Theme) {
	if b == nil {
		return
	}
	b.Theme = th
	b.markPaint()
}

// SetStyle sets optional style overrides (Radius may feed border radius when unset).
func (b *BorderBeam) SetStyle(st Style) {
	if b == nil {
		return
	}
	b.Style = st
	b.markPaint()
}

// SetAriaLabel sets an optional accessible name on the host.
func (b *BorderBeam) SetAriaLabel(s string) {
	if b == nil {
		return
	}
	b.AriaLabel = s
	if b.Root != nil {
		b.Root.Label = s
	}
}

// ResolvedDuration returns effective duration seconds.
func (b *BorderBeam) ResolvedDuration() float64 {
	if b == nil || b.Duration <= 0 {
		return DefaultBorderBeamDuration
	}
	return b.Duration
}

// ResolvedSize returns effective beam segment length.
func (b *BorderBeam) ResolvedSize() float64 {
	if b == nil || b.Size <= 0 {
		return DefaultBorderBeamSize
	}
	return b.Size
}

// ResolvedLineWidth returns effective stroke width.
func (b *BorderBeam) ResolvedLineWidth() float64 {
	if b == nil {
		return DefaultBorderBeamLineWidth
	}
	if b.LineWidth > 0 {
		return b.LineWidth
	}
	th := b.theme()
	if th != nil {
		return th.SizeOr(core.TokenLineWidth, DefaultBorderBeamLineWidth)
	}
	return DefaultBorderBeamLineWidth
}

// ResolvedOutset returns effective outset.
func (b *BorderBeam) ResolvedOutset() float64 {
	if b == nil {
		return DefaultBorderBeamOutset
	}
	if b.outsetSet {
		return b.Outset
	}
	return DefaultBorderBeamOutset
}

// ResolvedBorderRadius returns effective path radius.
func (b *BorderBeam) ResolvedBorderRadius() float64 {
	if b == nil {
		return DefaultBorderBeamRadius
	}
	if b.radiusSet {
		return b.BorderRadius
	}
	if b.Style.hasRadius() {
		return b.Style.Radius
	}
	th := b.theme()
	if th != nil {
		return th.SizeOr(core.TokenBorderRadius, DefaultBorderBeamRadius)
	}
	return DefaultBorderBeamRadius
}

// ResolvedFontSize returns §6.2 content fontSize baseline (for L2 tests).
func (b *BorderBeam) ResolvedFontSize() float64 {
	th := b.theme()
	if th != nil {
		return th.SizeOr(core.TokenFontSize, DefaultBorderBeamFontSize)
	}
	return DefaultBorderBeamFontSize
}

// ResolvedColorStops returns the effective gradient stops (public percent 0..100).
// When unset, returns Theme primary → primaryHover (not a hard-coded brand-only skin).
func (b *BorderBeam) ResolvedColorStops() []BorderBeamColorStop {
	if b == nil {
		return nil
	}
	if b.colorSet && len(b.stops) > 0 {
		out := make([]BorderBeamColorStop, len(b.stops))
		copy(out, b.stops)
		return out
	}
	th := b.theme()
	if th == nil {
		th = core.DefaultTheme()
	}
	primary := th.Color(core.TokenColorPrimary)
	hover := th.Color(core.TokenColorPrimaryHover)
	return []BorderBeamColorStop{
		{Color: primary, Percent: 0},
		{Color: hover, Percent: 100},
	}
}

func (b *BorderBeam) theme() *core.Theme {
	if b != nil && b.Theme != nil {
		return b.Theme
	}
	return core.DefaultTheme()
}

func (b *BorderBeam) markPaint() {
	if b == nil {
		return
	}
	if b.layer != nil {
		b.layer.MarkNeedsPaint()
	}
	if b.Root != nil {
		b.Root.MarkNeedsPaint()
	}
}

func (b *BorderBeam) rebuild() {
	if b == nil {
		return
	}
	if b.Duration <= 0 {
		b.Duration = DefaultBorderBeamDuration
	}
	if b.Size <= 0 {
		b.Size = DefaultBorderBeamSize
	}
	if b.LineWidth <= 0 {
		b.LineWidth = DefaultBorderBeamLineWidth
	}

	var h *borderBeamHost
	if b.Root != nil {
		h = b.Root
		h.bb = b
		h.ClearChildren()
	} else {
		h = &borderBeamHost{bb: b}
		h.Init(h)
		h.Hit = core.HitDefer
		h.Role = "presentation"
		b.Root = h
	}
	if b.AriaLabel != "" {
		h.Label = b.AriaLabel
	}

	layer := &borderBeamLayer{bb: b}
	layer.Init(layer)
	layer.Hit = core.HitTransparent
	layer.Role = "presentation"
	layer.PaintOrder = 1
	b.layer = layer

	h.child = b.child
	h.layer = layer
	if b.child != nil {
		h.AddChild(b.child)
	}
	h.AddChild(layer)

	b.syncTicker()
	h.MarkNeedsLayout()
	h.MarkNeedsPaint()
}

// paintBeam draws a short gradient segment along the rounded-rect perimeter.
func (b *BorderBeam) paintBeam(pc *core.PaintContext, sz core.Size) {
	if pc == nil || sz.Width <= 0 || sz.Height <= 0 {
		return
	}
	outset := b.ResolvedOutset()
	lw := b.ResolvedLineWidth()
	if lw <= 0 {
		lw = DefaultBorderBeamLineWidth
	}
	r := b.ResolvedBorderRadius()
	// Path is expanded by outset; inset slightly by half line so stroke sits on edge.
	x := -outset
	y := -outset
	w := sz.Width + 2*outset
	h := sz.Height + 2*outset
	if w <= 0 || h <= 0 {
		return
	}
	// Clamp radius to box.
	maxR := math.Min(w, h) / 2
	if r > maxR {
		r = maxR
	}
	if r < 0 {
		r = 0
	}

	peri := roundedRectPerimeter(w, h, r)
	if peri <= 1e-6 {
		return
	}
	segLen := b.ResolvedSize()
	if segLen <= 0 {
		segLen = DefaultBorderBeamSize
	}
	if segLen > peri*0.95 {
		segLen = peri * 0.95
	}
	// Head position advances with phase (antd offset-distance 0→100%).
	head := b.phase * peri

	stops := b.mapStopsForPaint()
	n := beamPaintSamples
	if n < 8 {
		n = 8
	}
	step := segLen / float64(n)
	var prevX, prevY float64
	var havePrev bool
	for i := 0; i <= n; i++ {
		// Sample from head backwards so the head is bright and the tail fades
		// (matches linear-gradient(to left, colors..., transparent) on a moving blob).
		along := head - float64(i)*step
		for along < 0 {
			along += peri
		}
		for along >= peri {
			along -= peri
		}
		px, py := pointOnRoundedRect(x, y, w, h, r, along, peri)
		// Progress along the beam segment: 0 at head, 1 at tail.
		t := float64(i) / float64(n)
		col := sampleBeamColor(stops, t)
		if col.A < 0.01 {
			havePrev = false
			continue
		}
		if havePrev {
			// Slight width falloff toward tail for a softer trail.
			width := lw * (1.0 - 0.35*t)
			if width < 0.5 {
				width = 0.5
			}
			pc.StrokeLocalLine(prevX, prevY, px, py, width, col)
		}
		prevX, prevY = px, py
		havePrev = true
	}
}

// mapStopsForPaint converts public 0..100 stops into beam-local 0..1 with 70% head mapping.
func (b *BorderBeam) mapStopsForPaint() []BorderBeamColorStop {
	raw := b.ResolvedColorStops()
	if len(raw) == 0 {
		return nil
	}
	// Ensure a terminal stop at 100% (antd fillGradientEnd).
	last := raw[len(raw)-1]
	if last.Percent < 100 {
		raw = append(raw, BorderBeamColorStop{Color: last.Color, Percent: 100})
	}
	out := make([]BorderBeamColorStop, 0, len(raw)+1)
	for _, s := range raw {
		p := s.Percent
		if p < 0 {
			p = 0
		}
		if p > 100 {
			p = 100
		}
		// Map into 0..MaxBorderBeamColorStopPercent of the beam segment.
		mapped := (p / 100) * MaxBorderBeamColorStopPercent
		out = append(out, BorderBeamColorStop{Color: s.Color, Percent: mapped})
	}
	// Explicit transparent at 100% of beam segment (tail fade).
	tail := out[len(out)-1].Color
	tail.A = 0
	out = append(out, BorderBeamColorStop{Color: tail, Percent: 100})
	return out
}

// sampleBeamColor samples mapped stops where t is 0..1 along the beam segment (head→tail).
func sampleBeamColor(stops []BorderBeamColorStop, t float64) render.RGBA {
	if len(stops) == 0 {
		return render.RGBA{}
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	pct := t * 100
	if pct <= stops[0].Percent {
		return stops[0].Color
	}
	for i := 1; i < len(stops); i++ {
		if pct <= stops[i].Percent {
			a := stops[i-1]
			b := stops[i]
			span := b.Percent - a.Percent
			if span <= 1e-9 {
				return b.Color
			}
			u := (pct - a.Percent) / span
			return lerpRGBA(a.Color, b.Color, u)
		}
	}
	return stops[len(stops)-1].Color
}

func lerpRGBA(a, b render.RGBA, t float64) render.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return render.RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func roundedRectPerimeter(w, h, r float64) float64 {
	if r <= 0 {
		return 2 * (w + h)
	}
	// 4 straight segments + full circle for 4 quarter arcs.
	return 2*(w-2*r) + 2*(h-2*r) + 2*math.Pi*r
}

// pointOnRoundedRect returns a point at arc-length `along` on the outer path.
// Path order (antd offset-path rect): top edge L→R, right T→B, bottom R→L, left B→T.
func pointOnRoundedRect(x, y, w, h, r, along, peri float64) (float64, float64) {
	if peri <= 0 {
		return x, y
	}
	for along < 0 {
		along += peri
	}
	for along >= peri {
		along -= peri
	}
	if r <= 0 {
		// Sharp rectangle.
		top, right, bottom := w, h, w
		if along < top {
			return x + along, y
		}
		along -= top
		if along < right {
			return x + w, y + along
		}
		along -= right
		if along < bottom {
			return x + w - along, y + h
		}
		along -= bottom
		return x, y + h - along
	}

	// Straight lengths and arc length (quarter circle).
	straightW := w - 2*r
	straightH := h - 2*r
	if straightW < 0 {
		straightW = 0
	}
	if straightH < 0 {
		straightH = 0
	}
	arc := 0.5 * math.Pi * r // quarter

	// top straight
	if along < straightW {
		return x + r + along, y
	}
	along -= straightW
	// top-right arc (from north to east)
	if along < arc {
		ang := -0.5*math.Pi + along/r // -90° → 0°
		cx, cy := x+w-r, y+r
		return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
	}
	along -= arc
	// right straight
	if along < straightH {
		return x + w, y + r + along
	}
	along -= straightH
	// bottom-right arc (east → south)
	if along < arc {
		ang := 0 + along/r
		cx, cy := x+w-r, y+h-r
		return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
	}
	along -= arc
	// bottom straight (right → left)
	if along < straightW {
		return x + w - r - along, y + h
	}
	along -= straightW
	// bottom-left arc (south → west)
	if along < arc {
		ang := 0.5*math.Pi + along/r
		cx, cy := x+r, y+h-r
		return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
	}
	along -= arc
	// left straight (bottom → top)
	if along < straightH {
		return x, y + h - r - along
	}
	along -= straightH
	// top-left arc (west → north)
	ang := math.Pi + along/r
	cx, cy := x+r, y+r
	return cx + r*math.Cos(ang), cy + r*math.Sin(ang)
}
