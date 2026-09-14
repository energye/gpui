package border_beam

import (
	"math"
	"sort"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Defaults mirror docs/antd/border-beam.md §6.2 (scale=1).
const (
	DefaultDuration  = 6.0
	DefaultSize      = 100.0
	DefaultLineWidth = 1.0
	// MaxStopPercent maps user 0..100 stops onto the leading 70% of the beam;
	// the trailing 30% fades to transparent.
	MaxStopPercent = 70.0
	// beamSteps is the polyline resolution of one beam segment.
	beamSteps = 24
)

// BorderBeamColorStop is one gradient stop; Percent uses the 0..100 input range.
type BorderBeamColorStop struct {
	Color   render.RGBA
	Percent float64
}

// Style is the semantic hook (no CSS engine).
type Style struct {
	ClassName string
}

// beamLayer is the decoration overlay: paints the beam, never hits.
type beamLayer struct {
	*rendering.RenderBox
}

func (l *beamLayer) HitTest(rendering.Point) rendering.RenderObject { return nil }

// BorderBeam decorates a child box with a beam looping its border
// (docs/antd/border-beam.md §6). Host box equals the child box;
// the beam layer is HitTransparent and aria-hidden.
type BorderBeam struct {
	child rendering.RenderObject
	host  *rendering.RenderBox
	beam  *beamLayer

	stops     []BorderBeamColorStop
	hasColor  bool
	duration  float64
	lineWidth float64
	outset    float64
	hasOutset bool
	size      float64
	radius    float64
	hasRadius bool

	showOnHover  bool
	hovered      bool
	reduceMotion bool
	rtl          bool
	phase        float64

	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	style     Style

	attached *scheduler.TickerRegistry
}

// NewBorderBeam wraps child (nil allowed; still lays out).
func NewBorderBeam(child rendering.RenderObject) *BorderBeam {
	b := &BorderBeam{}
	b.host = rendering.NewRenderBox()
	b.host.SetRepaintBoundary(true)
	inner := rendering.NewRenderBox()
	inner.SetRepaintBoundary(true)
	b.beam = &beamLayer{RenderBox: inner}
	beam := b
	inner.OnPaint = func(pc *rendering.PaintContext, _ rendering.Size) {
		beam.paintBeam(pc)
	}
	if child != nil {
		b.host.AddChild(child)
	}
	b.host.AddChild(b.beam)
	b.child = child
	return b
}

// SetChild swaps the decorated content; the beam stays on top.
func (b *BorderBeam) SetChild(child rendering.RenderObject) {
	if b == nil || b.host == nil {
		return
	}
	if b.child == child {
		return
	}
	if b.beam != nil {
		b.host.RemoveChild(b.beam)
	}
	if b.child != nil {
		b.host.RemoveChild(b.child)
	}
	b.child = child
	if child != nil {
		b.host.AddChild(child)
	}
	if b.beam != nil {
		b.host.AddChild(b.beam)
	}
}

// Child returns the decorated content (may be nil).
func (b *BorderBeam) Child() rendering.RenderObject {
	if b == nil {
		return nil
	}
	return b.child
}

// Node returns the tree node (layout/paint/hit through it).
func (b *BorderBeam) Node() rendering.RenderObject {
	if b == nil {
		return nil
	}
	return b.host
}

// Layout sizes the host to the child box under constraints.
func (b *BorderBeam) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.host == nil {
		return rendering.Size{}
	}
	return b.host.Layout(c)
}

// SetColor sets a single beam color.
func (b *BorderBeam) SetColor(c render.RGBA) {
	if b == nil {
		return
	}
	b.stops = []BorderBeamColorStop{{Color: c, Percent: 0}}
	b.hasColor = true
	b.dirty()
}

// SetColorStops sets the gradient stops (empty clears to the theme default).
func (b *BorderBeam) SetColorStops(stops ...BorderBeamColorStop) {
	if b == nil {
		return
	}
	if len(stops) == 0 {
		b.ClearColor()
		return
	}
	cp := append([]BorderBeamColorStop(nil), stops...)
	for i := range cp {
		cp[i].Percent = clamp(cp[i].Percent, 0, 100)
	}
	sort.Slice(cp, func(i, j int) bool { return cp[i].Percent < cp[j].Percent })
	b.stops = cp
	b.hasColor = true
	b.dirty()
}

// ClearColor falls back to the theme default gradient.
func (b *BorderBeam) ClearColor() {
	if b == nil {
		return
	}
	b.stops = nil
	b.hasColor = false
	b.dirty()
}

// ResolvedColorStops returns the explicit stops or the theme default.
func (b *BorderBeam) ResolvedColorStops() []BorderBeamColorStop {
	if b != nil && b.hasColor && len(b.stops) > 0 {
		return append([]BorderBeamColorStop(nil), b.stops...)
	}
	return b.defaultStops()
}

// SetDuration sets seconds per loop (<=0 selects DefaultDuration).
func (b *BorderBeam) SetDuration(s float64) {
	if b == nil {
		return
	}
	b.duration = s
	b.dirty()
}

// ResolvedDuration returns the effective seconds per loop.
func (b *BorderBeam) ResolvedDuration() float64 {
	if b == nil || b.duration <= 0 {
		return DefaultDuration
	}
	return b.duration
}

// SetSize sets the beam segment length in px (<=0 selects DefaultSize).
func (b *BorderBeam) SetSize(px float64) {
	if b == nil {
		return
	}
	b.size = px
	b.dirty()
}

// ResolvedSize returns the effective segment length.
func (b *BorderBeam) ResolvedSize() float64 {
	if b == nil || b.size <= 0 {
		return DefaultSize
	}
	return b.size
}

// SetLineWidth sets the beam width in px (<=0 selects DefaultLineWidth).
func (b *BorderBeam) SetLineWidth(px float64) {
	if b == nil {
		return
	}
	b.lineWidth = px
	b.dirty()
}

// ResolvedLineWidth returns the effective beam width.
func (b *BorderBeam) ResolvedLineWidth() float64 {
	if b == nil || b.lineWidth <= 0 {
		return DefaultLineWidth
	}
	return b.lineWidth
}

// SetOutset sets the beam outset in px (explicit, 0 allowed).
func (b *BorderBeam) SetOutset(px float64) {
	if b == nil {
		return
	}
	b.outset = px
	b.hasOutset = true
	b.dirty()
}

// ClearOutset falls back to edge-hugging (0).
func (b *BorderBeam) ClearOutset() {
	if b == nil {
		return
	}
	b.outset = 0
	b.hasOutset = false
	b.dirty()
}

// ResolvedOutset returns the effective outset.
func (b *BorderBeam) ResolvedOutset() float64 {
	if b == nil || !b.hasOutset {
		return 0
	}
	return b.outset
}

// HasOutset reports whether outset was explicitly set.
func (b *BorderBeam) HasOutset() bool { return b != nil && b.hasOutset }

// SetBorderRadius sets the beam path radius (0 = sharp).
func (b *BorderBeam) SetBorderRadius(px float64) {
	if b == nil {
		return
	}
	b.radius = px
	b.hasRadius = true
	b.dirty()
}

// ClearBorderRadius falls back to the theme radius token.
func (b *BorderBeam) ClearBorderRadius() {
	if b == nil {
		return
	}
	b.radius = 0
	b.hasRadius = false
	b.dirty()
}

// ResolvedBorderRadius returns the explicit radius or the theme token.
func (b *BorderBeam) ResolvedBorderRadius() float64 {
	if b != nil && b.hasRadius {
		return math.Max(0, b.radius)
	}
	return b.themeTokens().Radius
}

// ContentFontSize returns the theme font size for child text (§6.2).
func (b *BorderBeam) ContentFontSize() float64 {
	return b.themeTokens().FontSize
}

// SetShowOnHover hides the beam until hovered.
func (b *BorderBeam) SetShowOnHover(v bool) {
	if b == nil {
		return
	}
	b.showOnHover = v
	b.dirty()
}

// ShowOnHover reports the hover-only flag.
func (b *BorderBeam) ShowOnHover() bool { return b != nil && b.showOnHover }

// SetHovered injects the hover state (hosts forward pointer events).
func (b *BorderBeam) SetHovered(v bool) {
	if b == nil {
		return
	}
	b.hovered = v
	b.dirty()
}

// Hovered reports the injected hover state.
func (b *BorderBeam) Hovered() bool { return b != nil && b.hovered }

// SetReduceMotion hides the beam and freezes the phase.
func (b *BorderBeam) SetReduceMotion(v bool) {
	if b == nil {
		return
	}
	b.reduceMotion = v
	b.dirty()
}

// ReduceMotion reports the reduced-motion flag.
func (b *BorderBeam) ReduceMotion() bool { return b != nil && b.reduceMotion }

// SetRTL mirrors the travel direction.
func (b *BorderBeam) SetRTL(v bool) {
	if b == nil {
		return
	}
	b.rtl = v
	b.dirty()
}

// RTL reports the mirror flag.
func (b *BorderBeam) RTL() bool { return b != nil && b.rtl }

// Phase returns the loop progress in [0,1).
func (b *BorderBeam) Phase() float64 {
	if b == nil {
		return 0
	}
	return b.phase
}

// EffectivePhase applies the RTL mirror to Phase.
func (b *BorderBeam) EffectivePhase() float64 {
	if b == nil {
		return 0
	}
	if !b.rtl {
		return b.phase
	}
	e := 1 - b.phase
	return e - math.Floor(e)
}

// IsBeamVisible reports whether the beam paints this frame.
func (b *BorderBeam) IsBeamVisible() bool {
	if b == nil || b.reduceMotion {
		return false
	}
	if b.showOnHover && !b.hovered {
		return false
	}
	return true
}

// SetProvider selects the theme source (nil selects process default).
func (b *BorderBeam) SetProvider(p *theme.Provider) {
	if b == nil {
		return
	}
	b.provider = p
	b.dirty()
}

// SetTheme pins exact tokens (nil clears to provider).
func (b *BorderBeam) SetTheme(t *theme.Tokens) {
	if b == nil {
		return
	}
	b.override = t
	b.dirty()
}

// SetStyle stores the semantic hook.
func (b *BorderBeam) SetStyle(s Style) {
	if b == nil {
		return
	}
	b.style = s
}

// Style returns the stored hook.
func (b *BorderBeam) Style() Style {
	if b == nil {
		return Style{}
	}
	return b.style
}

// SetAriaLabel names the root (rare; the beam stays decorative).
func (b *BorderBeam) SetAriaLabel(s string) {
	if b == nil {
		return
	}
	b.ariaLabel = s
}

// AriaLabel returns the accessible name ("" means decorative).
func (b *BorderBeam) AriaLabel() string {
	if b == nil {
		return ""
	}
	return b.ariaLabel
}

// AriaHidden is true while the beam is purely decorative.
func (b *BorderBeam) AriaHidden() bool { return b == nil || b.ariaLabel == "" }

// Role returns "presentation" for the decoration, "group" when named.
func (b *BorderBeam) Role() string {
	if b != nil && b.ariaLabel != "" {
		return "group"
	}
	return "presentation"
}

// Focusable is always false: the beam never takes Tab.
func (b *BorderBeam) Focusable() bool { return false }

// AttachTicker registers the animation ticker.
func (b *BorderBeam) AttachTicker(reg *scheduler.TickerRegistry) {
	if b == nil || reg == nil {
		return
	}
	if b.attached != nil && b.attached != reg {
		b.attached.Remove(b)
	}
	b.attached = reg
	reg.Add(b)
}

// Attach is an alias of AttachTicker.
func (b *BorderBeam) Attach(reg *scheduler.TickerRegistry) { b.AttachTicker(reg) }

// Detach unregisters the animation ticker.
func (b *BorderBeam) Detach() {
	if b == nil || b.attached == nil {
		return
	}
	b.attached.Remove(b)
	b.attached = nil
}

// Tick advances the phase; hidden beams freeze. Stays registered.
func (b *BorderBeam) Tick(dt float64) bool {
	if b == nil {
		return false
	}
	if !b.IsBeamVisible() {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	d := b.ResolvedDuration()
	if d <= 0 {
		d = DefaultDuration
	}
	b.phase = b.phase + dt/d
	b.phase -= math.Floor(b.phase)
	if dt > 0 {
		b.dirty()
	}
	return true
}

// WantsFrame reports frame demand (scheduler.FrameWanter).
func (b *BorderBeam) WantsFrame() bool { return b.IsBeamVisible() }

func (b *BorderBeam) dirty() {
	if b == nil || b.beam == nil {
		return
	}
	b.beam.MarkNeedsPaint()
}

func (b *BorderBeam) themeTokens() theme.Tokens {
	if b != nil && b.override != nil {
		return *b.override
	}
	if b != nil && b.provider != nil {
		return b.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func lighten(c render.RGBA, t float64) render.RGBA {
	return render.RGBA{
		R: c.R + (1-c.R)*t,
		G: c.G + (1-c.G)*t,
		B: c.B + (1-c.B)*t,
		A: c.A,
	}
}

// defaultStops builds the theme gradient: primary head, derived hover mid.
func (b *BorderBeam) defaultStops() []BorderBeamColorStop {
	tok := b.themeTokens()
	head := themeToRGBA(tok.ColorPrimary)
	return []BorderBeamColorStop{
		{Color: head, Percent: 0},
		{Color: lighten(head, 0.15), Percent: 100},
	}
}

// paintBeam draws the phase-positioned segment along the host border.
func (b *BorderBeam) paintBeam(pc *rendering.PaintContext) {
	if b == nil || pc == nil || !b.IsBeamVisible() {
		return
	}
	hs := b.host.Size()
	o := b.ResolvedOutset()
	w, h := hs.Width+2*o, hs.Height+2*o
	if w <= 0 || h <= 0 {
		return
	}
	lw := b.ResolvedLineWidth()
	rect := beamRect{x0: -o, y0: -o, w: w, h: h, r: b.ResolvedBorderRadius() + math.Max(0, o)}
	total := rect.perimeter()
	if total <= 0 {
		return
	}
	seg := b.ResolvedSize()
	if seg <= 0 {
		return
	}
	if seg > total {
		seg = total
	}
	stops := b.ResolvedColorStops()
	if len(stops) == 0 {
		return
	}
	if pc.DC != nil {
		pc.DC.SetLineCap(render.LineCapRound)
	}
	start := b.EffectivePhase() * total
	for i := 0; i < beamSteps; i++ {
		t0 := float64(i) / beamSteps
		t1 := float64(i+1) / beamSteps
		tm := (t0 + t1) / 2
		x0, y0 := rect.pointAt(start + t0*seg)
		x1, y1 := rect.pointAt(start + t1*seg)
		c := beamColorAt(stops, tm)
		rendering.StrokeLine(pc, x0, y0, x1, y1, lw, c.R, c.G, c.B, c.A)
	}
}

// beamColorAt maps tail(0)..head(1) through the stops then a transparent fade.
func beamColorAt(stops []BorderBeamColorStop, t float64) render.RGBA {
	last := stops[len(stops)-1].Color
	if t > MaxStopPercent/100 {
		f := 1 - (t-MaxStopPercent/100)/(1-MaxStopPercent/100)
		if f < 0 {
			f = 0
		}
		return render.RGBA{R: last.R, G: last.G, B: last.B, A: last.A * f}
	}
	s := t / (MaxStopPercent / 100)
	pos := func(p float64) float64 { return clamp(p, 0, 100) / 100 * (MaxStopPercent / 100) }
	if s <= pos(stops[0].Percent) {
		return stops[0].Color
	}
	for i := 0; i+1 < len(stops); i++ {
		p0, p1 := pos(stops[i].Percent), pos(stops[i+1].Percent)
		if s <= p1 {
			span := p1 - p0
			if span <= 0 {
				return stops[i+1].Color
			}
			return lerpRGBA(stops[i].Color, stops[i+1].Color, (s-p0)/span)
		}
	}
	return last
}

func lerpRGBA(a, b render.RGBA, t float64) render.RGBA {
	return render.RGBA{
		R: a.R + (b.R-a.R)*t,
		G: a.G + (b.G-a.G)*t,
		B: a.B + (b.B-a.B)*t,
		A: a.A + (b.A-a.A)*t,
	}
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// beamRect is a uniform rounded rect in beam-local coordinates.
type beamRect struct {
	x0, y0, w, h, r float64
}

func (r beamRect) radius() float64 {
	rr := r.r
	if rr < 0 {
		rr = 0
	}
	if m := math.Min(r.w, r.h) / 2; rr > m {
		rr = m
	}
	return rr
}

func (r beamRect) perimeter() float64 {
	if r.w <= 0 || r.h <= 0 {
		return 0
	}
	if rr := r.radius(); rr > 0 {
		return 2*(r.w-2*rr) + 2*(r.h-2*rr) + 2*math.Pi*rr
	}
	return 2 * (r.w + r.h)
}

// pointAt walks clockwise from the top edge start.
func (r beamRect) pointAt(d float64) (float64, float64) {
	p := r.perimeter()
	if p <= 0 {
		return r.x0, r.y0
	}
	d -= math.Floor(d/p) * p
	if d < 0 {
		d += p
	}
	rr := r.radius()
	x0, y0 := r.x0, r.y0
	if rr <= 0 {
		switch {
		case d < r.w:
			return x0 + d, y0
		case d < r.w+r.h:
			return x0 + r.w, y0 + (d - r.w)
		case d < 2*r.w+r.h:
			return x0 + r.w - (d - r.w - r.h), y0 + r.h
		default:
			return x0, y0 + r.h - (d - 2*r.w - r.h)
		}
	}
	wt, ht := r.w-2*rr, r.h-2*rr
	arc := math.Pi * rr / 2
	if d < wt {
		return x0 + rr + d, y0
	}
	d -= wt
	if d < arc {
		a := -math.Pi/2 + d/rr
		return x0 + r.w - rr + rr*math.Cos(a), y0 + rr + rr*math.Sin(a)
	}
	d -= arc
	if d < ht {
		return x0 + r.w, y0 + rr + d
	}
	d -= ht
	if d < arc {
		a := d / rr
		return x0 + r.w - rr + rr*math.Cos(a), y0 + r.h - rr + rr*math.Sin(a)
	}
	d -= arc
	if d < wt {
		return x0 + r.w - rr - d, y0 + r.h
	}
	d -= wt
	if d < arc {
		a := math.Pi/2 + d/rr
		return x0 + rr + rr*math.Cos(a), y0 + r.h - rr + rr*math.Sin(a)
	}
	d -= arc
	if d < ht {
		return x0, y0 + r.h - rr - d
	}
	d -= ht
	a := math.Pi + d/rr
	return x0 + rr + rr*math.Cos(a), y0 + rr + rr*math.Sin(a)
}
