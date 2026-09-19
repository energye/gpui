package spin

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Spin is the loading indicator (docs/antd/spin.md §6).
//
// Simple (no content): centered section with a 4-dot looper or percent
// ring plus an optional description. Nested (content set): content stays
// in the tree with a mask blocking hits while display is on.
// Animation follows the host scheduler ticker; still subtrees stay cheap
// because the host is a repaint boundary and phase ticks only mark paint.
const (
	// SpinPeriodSec is one 4-dot revolution (spec §6.2.1, 1.2s).
	SpinPeriodSec = 1.2
	// autoStepSec is the percent=auto bucket width (spec §6.2.1, 200ms).
	autoStepSec = 0.2
	// maskAlpha approximates colorBgContainer at 0.4 (spec §6.2.2).
	maskAlpha = 0.4
	// defaultAriaLabel is the accessible name when no description is set.
	defaultAriaLabel = "Loading"
)

// SpinSize selects the indicator diameter (spec §6.3).
type SpinSize string

const (
	SpinSmall  SpinSize = "small"
	SpinMedium SpinSize = "medium"
	SpinLarge  SpinSize = "large"
)

// normalizeSize maps "" and "default" to medium.
func normalizeSize(s SpinSize) SpinSize {
	switch s {
	case SpinSmall:
		return SpinSmall
	case SpinLarge:
		return SpinLarge
	default:
		return SpinMedium
	}
}

// SpinClassNames holds shallow semantic hooks (spec §6.8 P0 shallow set).
type SpinClassNames struct {
	Root        string
	Section     string
	Indicator   string
	Description string
	Container   string
}

// SpinStyles holds shallow inline-style hooks (opaque strings, P0 shallow).
type SpinStyles struct {
	Root        string
	Section     string
	Indicator   string
	Description string
	Container   string
}

// Style holds optional root overrides. Non-positive values select tokens.
type Style struct {
	Gap      float64
	FontSize float64
}

var (
	globalMu        sync.RWMutex
	globalIndicator rendering.RenderObject
)

// SetDefaultIndicator sets the package-wide fallback indicator
// (antd Spin.setDefaultIndicator mapping). Nil clears it.
func SetDefaultIndicator(n rendering.RenderObject) {
	globalMu.Lock()
	globalIndicator = n
	globalMu.Unlock()
}

// DefaultIndicator returns the package-wide fallback (nil when unset).
func DefaultIndicator() rendering.RenderObject {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalIndicator
}

// Spin owns a repaint-boundary host node. Put Node() in the tree, drive
// Tick via a scheduler.TickerRegistry, and read the Effective* helpers
// for assertions.
type Spin struct {
	content     rendering.RenderObject
	spinning    bool
	delayMs     float64
	delayElaped float64
	display     bool
	size        SpinSize
	description string
	tip         string

	hasPercent  bool
	percentVal  float64
	percentAuto bool
	autoVal     float64
	autoAccum   float64

	indicator rendering.RenderObject

	provider *theme.Provider
	override *theme.Tokens
	// textFace is the paint-only font face for the description.
	textFace   text.Face
	style      Style
	classNames SpinClassNames
	styles     SpinStyles
	ariaLabel  string
	fullscreen bool

	reduceMotion bool
	// phase is atomic bits: Tick writes (UI), paint reads (raster).
	phase atomic.Uint64

	host    *rendering.RenderBox
	mask    *rendering.RenderColorBox
	overlay *rendering.RenderBox

	// indSize caches the custom-indicator measured size. Stored by Layout
	// (UI) and read on both threads (measure helpers on UI, paintCustom on
	// raster): atomic, never bare fields — paint must not write widget
	// state (R2-6 write-back). Zero value before first layout.
	indSize atomic.Value // rendering.Size

	// snap freezes every paint input on the UI thread (R2-6 button
	// paradigm). Raster paint reads only this snapshot plus the phase /
	// indSize / percent atomics above. See snapshot.go.
	snap atomic.Value // SpinSnap

	attached *scheduler.TickerRegistry
}

// NewSpin creates a spin with optional nested content (nil for simple).
// Defaults follow spec §6.10: spinning=true, size=medium, delay=0.
func NewSpin(content rendering.RenderObject) *Spin {
	s := &Spin{
		content:  content,
		spinning: true,
		size:     SpinMedium,
		display:  true,
	}
	s.host = rendering.NewRenderBox()
	s.host.SetRepaintBoundary(true)
	paint := s
	s.host.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		paint.paint(pc, size)
	}
	s.overlay = rendering.NewRenderBox()
	overlaySelf := s
	s.overlay.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		overlaySelf.paintSection(pc, size, overlaySelf.loadSnapshot())
	}
	tok := s.tokens()
	s.mask = rendering.NewRenderColorBox(0, 0, tok.ColorBgContainer.R, tok.ColorBgContainer.G, tok.ColorBgContainer.B, maskAlpha)
	s.syncStructure()
	s.refreshSnapshot()
	s.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return s
}

// Node returns the tree node (layout/paint/hit through it).
func (s *Spin) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.host
}

// Content returns the nested content (nil for simple mode).
func (s *Spin) Content() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.content
}

// SetContent replaces the nested content (nil selects simple mode).
func (s *Spin) SetContent(n rendering.RenderObject) {
	if s == nil {
		return
	}
	if s.content == n {
		return
	}
	if s.content != nil {
		s.host.RemoveChild(s.content)
	}
	s.content = n
	if n != nil {
		s.host.AddChild(n)
	}
	s.syncStructure()
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// SetSpinning sets the props flag; display may lag behind via delay.
func (s *Spin) SetSpinning(b bool) {
	if s == nil {
		return
	}
	s.spinning = b
	if !b {
		s.display = false
		s.delayElaped = 0
	} else if s.delayMs <= 0 {
		s.display = true
	} else {
		s.display = false
		s.delayElaped = 0
	}
	s.syncStructure()
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// Spinning reports the props flag (before delay).
func (s *Spin) Spinning() bool { return s != nil && s.spinning }

// SetDelay sets the anti-flicker delay in milliseconds (0 shows at once).
func (s *Spin) SetDelay(ms float64) {
	if s == nil {
		return
	}
	if ms < 0 {
		ms = 0
	}
	s.delayMs = ms
	if s.spinning {
		if ms <= 0 {
			s.display = true
		} else {
			s.display = false
			s.delayElaped = 0
		}
	}
	s.syncStructure()
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// Delay returns the configured delay in milliseconds.
func (s *Spin) Delay() float64 {
	if s == nil {
		return 0
	}
	return s.delayMs
}

// SetSize selects small/medium/large ("default" maps to medium).
func (s *Spin) SetSize(sz SpinSize) {
	if s == nil {
		return
	}
	n := normalizeSize(sz)
	if s.size == n {
		return
	}
	s.size = n
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// Size returns the normalized size.
func (s *Spin) Size() SpinSize {
	if s == nil {
		return SpinMedium
	}
	return normalizeSize(s.size)
}

// SetDescription sets the primary caption shown with the indicator.
func (s *Spin) SetDescription(d string) {
	if s == nil {
		return
	}
	if s.description == d {
		return
	}
	s.description = d
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// SetTip is the deprecated alias of SetDescription (description wins).
func (s *Spin) SetTip(t string) {
	if s == nil {
		return
	}
	if s.tip == t {
		return
	}
	s.tip = t
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// SetTextFace sets the paint-only font face for the description.
func (s *Spin) SetTextFace(f text.Face) {
	if s == nil {
		return
	}
	s.textFace = f
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

// TextFace returns the paint-only font face (nil when unset).
func (s *Spin) TextFace() text.Face {
	if s == nil {
		return nil
	}
	return s.textFace
}

// SetFullscreen selects viewport-mask mode (P1).
func (s *Spin) SetFullscreen(b bool) {
	if s == nil || s.fullscreen == b {
		return
	}
	s.fullscreen = b
	s.syncStructure()
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// Fullscreen reports viewport-mask mode.
func (s *Spin) Fullscreen() bool { return s != nil && s.fullscreen }

// EffectiveDescription returns description, falling back to tip.
func (s *Spin) EffectiveDescription() string {
	if s == nil {
		return ""
	}
	if s.description != "" {
		return s.description
	}
	return s.tip
}

// SetPercent sets a numeric progress ring and clears auto mode.
func (s *Spin) SetPercent(v float64) {
	if s == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	s.hasPercent = true
	s.percentAuto = false
	s.percentVal = v
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

// SetPercentAuto selects the never-ending simulated progress.
func (s *Spin) SetPercentAuto() {
	if s == nil {
		return
	}
	s.hasPercent = true
	s.percentAuto = true
	s.autoVal = 0
	s.autoAccum = 0
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

// ClearPercent returns to the 4-dot looper.
func (s *Spin) ClearPercent() {
	if s == nil {
		return
	}
	s.hasPercent = false
	s.percentAuto = false
	s.percentVal = 0
	s.autoVal = 0
	s.autoAccum = 0
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

// HasPercent reports whether a progress ring is active.
func (s *Spin) HasPercent() bool { return s != nil && s.hasPercent }

// EffectivePercent returns the numeric or simulated progress.
// Zero with HasPercent()==false means the 4-dot looper is active.
func (s *Spin) EffectivePercent() float64 {
	if s == nil || !s.hasPercent {
		return 0
	}
	if s.percentAuto {
		return s.autoVal
	}
	return s.percentVal
}

// SetIndicator sets the per-instance custom indicator (nil clears it).
func (s *Spin) SetIndicator(n rendering.RenderObject) {
	if s == nil {
		return
	}
	s.indicator = n
	s.refreshSnapshot()
	s.host.MarkNeedsPaint()
}

// EffectiveIndicator returns instance, then global, then nil for builtin.
func (s *Spin) EffectiveIndicator() rendering.RenderObject {
	if s == nil {
		return nil
	}
	if s.indicator != nil {
		return s.indicator
	}
	return DefaultIndicator()
}

// IsBuiltinIndicator reports whether the builtin 4-dot/ring paints.
func (s *Spin) IsBuiltinIndicator() bool {
	return s != nil && s.EffectiveIndicator() == nil
}

// SetProvider selects the theme source (nil selects process default).
func (s *Spin) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.refreshSnapshot()
	s.refreshMask()
	s.host.MarkNeedsPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (s *Spin) SetTheme(t *theme.Tokens) {
	if s == nil {
		return
	}
	s.override = t
	s.refreshSnapshot()
	s.refreshMask()
	s.host.MarkNeedsPaint()
}

// SetStyle stores the root style overrides.
func (s *Spin) SetStyle(st Style) {
	if s == nil {
		return
	}
	s.style = st
	s.refreshSnapshot()
	s.host.MarkNeedsLayout()
}

// Style returns the stored root overrides.
func (s *Spin) Style() Style {
	if s == nil {
		return Style{}
	}
	return s.style
}

// SetClassNames stores the shallow semantic hooks.
func (s *Spin) SetClassNames(c SpinClassNames) {
	if s == nil {
		return
	}
	s.classNames = c
}

// ClassNames returns the stored hooks.
func (s *Spin) ClassNames() SpinClassNames {
	if s == nil {
		return SpinClassNames{}
	}
	return s.classNames
}

// SetStyles stores the shallow inline-style hooks.
func (s *Spin) SetStyles(st SpinStyles) {
	if s == nil {
		return
	}
	s.styles = st
}

// Styles returns the stored hooks.
func (s *Spin) Styles() SpinStyles {
	if s == nil {
		return SpinStyles{}
	}
	return s.styles
}

// SetAriaLabel sets the accessible name (empty selects description/default).
func (s *Spin) SetAriaLabel(v string) {
	if s == nil {
		return
	}
	s.ariaLabel = v
}

// AriaLabel returns the accessible name.
func (s *Spin) AriaLabel() string {
	if s == nil {
		return defaultAriaLabel
	}
	if s.ariaLabel != "" {
		return s.ariaLabel
	}
	if d := s.EffectiveDescription(); d != "" {
		return d
	}
	return defaultAriaLabel
}

// Role returns status, or progressbar in percent mode (spec §6.6).
func (s *Spin) Role() string {
	if s != nil && s.hasPercent && s.IsDisplaySpinning() {
		return "progressbar"
	}
	return "status"
}

// AriaValueMin/Max/Now expose percent semantics (spec §6.6).
func (s *Spin) AriaValueMin() float64 { return 0 }
func (s *Spin) AriaValueMax() float64 { return 100 }
func (s *Spin) AriaValueNow() float64 { return s.EffectivePercent() }

// AriaLive returns the live-region politeness (spec §6.6).
func (s *Spin) AriaLive() string { return "polite" }

// AriaBusy reports whether the spinner currently shows.
func (s *Spin) AriaBusy() bool { return s.IsDisplaySpinning() }

// Focusable is always false: spin never takes keyboard focus.
func (s *Spin) Focusable() bool { return false }

// SetReduceMotion freezes rotation and auto progress (accessibility).
func (s *Spin) SetReduceMotion(b bool) {
	if s == nil {
		return
	}
	s.reduceMotion = b
}

// ReduceMotion reports the accessibility freeze flag.
func (s *Spin) ReduceMotion() bool { return s != nil && s.reduceMotion }

// Phase returns the 4-dot rotation phase in [0,1).
func (s *Spin) Phase() float64 {
	if s == nil {
		return 0
	}
	return math.Float64frombits(s.phase.Load())
}

// IsDisplaySpinning reports the post-delay visible state.
func (s *Spin) IsDisplaySpinning() bool {
	if s == nil || !s.spinning {
		return false
	}
	if s.delayMs <= 0 {
		return true
	}
	return s.display
}

// DotSize returns the indicator diameter from theme control heights
// (small 14 / medium 20 / large 32 with default tokens, ±0.5).
func (s *Spin) DotSize() float64 {
	tok := s.tokens()
	lg := tok.ControlHeightLG
	base := tok.ControlHeight
	if lg <= 0 {
		lg = 40
	}
	if base <= 0 {
		base = 32
	}
	switch s.Size() {
	case SpinSmall:
		return lg * 0.35
	case SpinLarge:
		return base
	default:
		return lg / 2
	}
}

// EffectiveGap returns the indicator-to-description gap (token PaddingSM).
func (s *Spin) EffectiveGap() float64 {
	if s != nil && s.style.Gap > 0 {
		return s.style.Gap
	}
	tok := s.tokens()
	if tok.PaddingSM > 0 {
		return tok.PaddingSM
	}
	return 12
}

// EffectiveFontSize returns the description size (token FontSize).
func (s *Spin) EffectiveFontSize() float64 {
	if s != nil && s.style.FontSize > 0 {
		return s.style.FontSize
	}
	tok := s.tokens()
	if tok.FontSize > 0 {
		return tok.FontSize
	}
	return 14
}

// EffectiveIndicatorColor returns the indicator color.
// Fullscreen uses white on the dark mask, otherwise ColorPrimary.
func (s *Spin) EffectiveIndicatorColor() render.RGBA {
	tok := s.tokens()
	if s != nil && s.fullscreen {
		return render.RGBA{R: tok.ColorWhite.R, G: tok.ColorWhite.G, B: tok.ColorWhite.B, A: 1}
	}
	return render.RGBA{R: tok.ColorPrimary.R, G: tok.ColorPrimary.G, B: tok.ColorPrimary.B, A: tok.ColorPrimary.A}
}

// EffectiveDescriptionColor returns the description ink.
// Fullscreen uses white, otherwise the primary indicator color.
func (s *Spin) EffectiveDescriptionColor() render.RGBA {
	if s != nil && s.fullscreen {
		tok := s.tokens()
		return render.RGBA{R: tok.ColorWhite.R, G: tok.ColorWhite.G, B: tok.ColorWhite.B, A: 1}
	}
	return s.EffectiveIndicatorColor()
}

// EffectiveMaskColor returns the overlay fill.
// Fullscreen uses ColorBgMask, nested uses ColorBgContainer at 0.4.
func (s *Spin) EffectiveMaskColor() render.RGBA {
	tok := s.tokens()
	if s != nil && s.fullscreen {
		return render.RGBA{R: tok.ColorBgMask.R, G: tok.ColorBgMask.G, B: tok.ColorBgMask.B, A: tok.ColorBgMask.A}
	}
	return render.RGBA{R: tok.ColorBgContainer.R, G: tok.ColorBgContainer.G, B: tok.ColorBgContainer.B, A: maskAlpha}
}

// EffectiveTrackColor returns the percent-ring track (ColorFillSecondary).
func (s *Spin) EffectiveTrackColor() render.RGBA {
	tok := s.tokens()
	return render.RGBA{R: tok.ColorFillSecondary.R, G: tok.ColorFillSecondary.G, B: tok.ColorFillSecondary.B, A: tok.ColorFillSecondary.A}
}

func (s *Spin) tokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

func (s *Spin) refreshMask() {
	if s == nil || s.mask == nil {
		return
	}
	mc := s.EffectiveMaskColor()
	s.mask.R, s.mask.G, s.mask.B = mc.R, mc.G, mc.B
	s.mask.A = mc.A
	s.mask.MarkNeedsPaint()
}

func (s *Spin) wantMask() bool {
	if s == nil || !s.IsDisplaySpinning() {
		return false
	}
	return s.content != nil || s.fullscreen
}

func (s *Spin) hasChild(n rendering.RenderObject) bool {
	if s == nil || s.host == nil || n == nil {
		return false
	}
	for _, c := range s.host.Children() {
		if c == n {
			return true
		}
	}
	return false
}

// syncStructure keeps host children at [content?, mask?, overlay?].
// Indicator nodes paint through callbacks so a shared global never reparents.
// Overlay sits above the mask so the section stays visible in nested mode.
func (s *Spin) syncStructure() {
	if s == nil || s.host == nil {
		return
	}
	if s.content != nil && !s.hasChild(s.content) {
		s.host.AddChild(s.content)
	}
	if s.wantMask() {
		s.refreshMask()
		if !s.hasChild(s.mask) {
			s.host.AddChild(s.mask)
		}
		if s.overlay != nil && !s.hasChild(s.overlay) {
			s.host.AddChild(s.overlay)
		}
	} else {
		if s.hasChild(s.mask) {
			s.host.RemoveChild(s.mask)
		}
		if s.overlay != nil && s.hasChild(s.overlay) {
			s.host.RemoveChild(s.overlay)
		}
	}
	// Keep content below mask and overlay above mask.
	if s.content != nil && s.wantMask() {
		s.host.RemoveChild(s.content)
		s.host.AddChild(s.content)
		s.host.RemoveChild(s.mask)
		s.host.AddChild(s.mask)
		if s.overlay != nil {
			s.host.RemoveChild(s.overlay)
			s.host.AddChild(s.overlay)
		}
	}
}

// Layout sizes the host: content size when nested, indicator plus
// description otherwise. hit == layout == paint stays on the host box.
func (s *Spin) Layout(c rendering.Constraints) rendering.Size {
	if s == nil {
		return rendering.Size{}
	}
	s.syncStructure()
	if s.content != nil {
		s.host.FixedWidth, s.host.FixedHeight = 0, 0
		inner := rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
		cs := s.content.Layout(inner)
		s.mask.Width, s.mask.Height = cs.Width, cs.Height
		if s.overlay != nil {
			s.overlay.FixedWidth, s.overlay.FixedHeight = cs.Width, cs.Height
		}
		sz := s.host.Layout(c)
		// Keep the mask covering the laid-out host.
		s.mask.Width, s.mask.Height = sz.Width, sz.Height
		if s.overlay != nil {
			s.overlay.FixedWidth, s.overlay.FixedHeight = sz.Width, sz.Height
			s.overlay.Layout(rendering.Tight(sz.Width, sz.Height))
		}
		return sz
	}
	if s.fullscreen && s.IsDisplaySpinning() && c.MaxWidth < rendering.Unbounded/2 && c.MaxHeight < rendering.Unbounded/2 {
		s.host.FixedWidth, s.host.FixedHeight = c.MaxWidth, c.MaxHeight
		s.mask.Width, s.mask.Height = c.MaxWidth, c.MaxHeight
		if s.overlay != nil {
			s.overlay.FixedWidth, s.overlay.FixedHeight = c.MaxWidth, c.MaxHeight
		}
		sz := s.host.Layout(c)
		s.mask.Width, s.mask.Height = sz.Width, sz.Height
		if s.overlay != nil {
			s.overlay.FixedWidth, s.overlay.FixedHeight = sz.Width, sz.Height
			s.overlay.Layout(rendering.Tight(sz.Width, sz.Height))
		}
		return sz
	}
	tok := s.tokens()
	_ = tok
	dot := s.DotSize()
	desc := s.EffectiveDescription()
	var descW, descH float64
	if desc != "" {
		descW, descH = rendering.EstimateTextSize(desc, s.EffectiveFontSize(), 0.55)
	}
	indW, indH := dot, dot
	if ind := s.EffectiveIndicator(); ind != nil && s.IsDisplaySpinning() {
		is := ind.Layout(rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight})
		indW, indH = is.Width, is.Height
		s.indSize.Store(rendering.Size{Width: indW, Height: indH})
	} else {
		s.indSize.Store(rendering.Size{})
	}
	w := indW
	if descW > w {
		w = descW
	}
	h := indH
	if desc != "" {
		h += s.EffectiveGap() + descH
	}
	if !s.IsDisplaySpinning() {
		// Hidden simple spin collapses to zero; children hit nothing.
		s.host.FixedWidth, s.host.FixedHeight = 0, 0
		return s.host.Layout(c)
	}
	s.host.FixedWidth, s.host.FixedHeight = w, h
	return s.host.Layout(c)
}

// Attach registers the animation ticker (stays until Detach).
func (s *Spin) Attach(reg *scheduler.TickerRegistry) {
	if s == nil || reg == nil {
		return
	}
	if s.attached != nil && s.attached != reg {
		s.attached.Remove(s)
	}
	s.attached = reg
	reg.Add(s)
}

// AttachTicker is an alias of Attach (spec §6.10 naming).
func (s *Spin) AttachTicker(reg *scheduler.TickerRegistry) { s.Attach(reg) }

// Detach unregisters the animation ticker.
func (s *Spin) Detach() {
	if s == nil || s.attached == nil {
		return
	}
	s.attached.Remove(s)
	s.attached = nil
}

// Tick advances delay, rotation phase, and percent=auto buckets.
// It only marks paint; structure changes happen on display flips.
// Stays registered (return true) so delay/auto keep pacing.
func (s *Spin) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	if s.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	if s.spinning && s.delayMs > 0 && !s.display {
		s.delayElaped += dt
		if s.delayElaped*1000 >= s.delayMs {
			s.display = true
			s.syncStructure()
			s.refreshSnapshot()
			s.host.MarkNeedsLayout()
		}
		return true
	}
	if !s.IsDisplaySpinning() {
		return true
	}
	if s.hasPercent && s.percentAuto {
		s.autoAccum += dt
		dirty := false
		for s.autoAccum >= autoStepSec {
			s.autoAccum -= autoStepSec
			var step float64
			switch {
			case s.autoVal >= 96:
				step = 0
			case s.autoVal >= 70:
				step = 1
			case s.autoVal >= 30:
				step = 3
			default:
				step = 5
			}
			if step > 0 {
				s.autoVal += step
				if s.autoVal > 98 {
					s.autoVal = 98
				}
				dirty = true
			}
		}
		if dirty {
			s.refreshSnapshot()
			s.host.MarkNeedsPaint()
		}
		return true
	}
	if s.hasPercent {
		return true
	}
	if s.EffectiveIndicator() != nil {
		return true
	}
	ph := math.Float64frombits(s.phase.Load()) + dt/SpinPeriodSec
	ph = math.Mod(ph, 1)
	if ph < 0 {
		ph++
	}
	s.phase.Store(math.Float64bits(ph))
	s.host.MarkNeedsPaint()
	return true
}

// WantsFrame reports frame demand: only the builtin looper or auto
// progress while visible and motion is allowed. Static rings, hidden
// spins, custom indicators, and reduced-motion never hold the loop open.
func (s *Spin) WantsFrame() bool {
	if s == nil || s.reduceMotion {
		return false
	}
	if !s.IsDisplaySpinning() {
		return false
	}
	if s.hasPercent && !s.percentAuto {
		return false
	}
	if s.EffectiveIndicator() != nil {
		return false
	}
	return true
}

// NextWake implements scheduler.DeadlineWanter for delay pacing: the loop
// may sleep until the pending display flip. Events still wake it early.
func (s *Spin) NextWake() (time.Duration, bool) {
	if s == nil {
		return 0, false
	}
	if s.spinning && s.delayMs > 0 && !s.display {
		rem := s.delayMs - s.delayElaped*1000
		if rem < 0 {
			rem = 0
		}
		return time.Duration(rem * float64(time.Millisecond)), true
	}
	return 0, false
}

func (s *Spin) paint(pc *rendering.PaintContext, size rendering.Size) {
	if s == nil || pc == nil {
		return
	}
	S := s.loadSnapshot()
	if !S.Visible || S.WantMask {
		return
	}
	s.paintSection(pc, size, S)
}

func (s *Spin) paintSection(pc *rendering.PaintContext, size rendering.Size, S SpinSnap) {
	if s == nil || pc == nil || !S.Visible {
		return
	}
	if pc.DC == nil {
		return
	}
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	primary := S.IndicatorColor
	desc := S.Description
	var descH float64
	if desc != "" {
		_, descH = rendering.EstimateTextSize(desc, S.FontSize, 0.55)
	}
	if S.HasPercent {
		s.paintRing(pc, size, S)
	} else if ind := S.Indicator; ind != nil {
		s.paintCustom(pc, size, S, ind, descH)
	} else {
		s.paintDots(pc, size, S, primary, descH)
	}
	if desc != "" {
		s.paintDescription(pc, size, S, desc, S.DescriptionColor)
	}
}

func (s *Spin) indicatorHeight() float64 {
	if s != nil && s.EffectiveIndicator() != nil {
		if v, ok := s.indSize.Load().(rendering.Size); ok && v.Height > 0 {
			return v.Height
		}
	}
	if s == nil {
		return 20
	}
	return s.DotSize()
}

func (s *Spin) indicatorHeightSnap(S SpinSnap) float64 {
	if S.Indicator != nil {
		if v, ok := s.indSize.Load().(rendering.Size); ok && v.Height > 0 {
			return v.Height
		}
	}
	return S.DotSize
}

func (s *Spin) sectionTopSnap(S SpinSnap, size rendering.Size, descH float64) float64 {
	indH := s.indicatorHeightSnap(S)
	if S.Description == "" {
		return (size.Height - indH) / 2
	}
	total := indH + S.Gap + descH
	top := (size.Height - total) / 2
	if top < 0 {
		top = 0
	}
	return top
}

func (s *Spin) paintDots(pc *rendering.PaintContext, size rendering.Size, S SpinSnap, primary render.RGBA, descH float64) {
	dot := S.DotSize
	top := s.sectionTopSnap(S, size, descH)
	cx := size.Width / 2
	cy := top + dot/2
	orbit := dot * 0.32
	radius := dot * 0.16
	if radius < 1.5 {
		radius = 1.5
	}
	base := math.Float64frombits(s.phase.Load()) * 2 * math.Pi
	for i := 0; i < 4; i++ {
		ang := base + float64(i)*math.Pi/2
		dx := math.Cos(ang) * orbit
		dy := math.Sin(ang) * orbit
		ph := math.Float64frombits(s.phase.Load()) + float64(i)*0.25
		ph -= math.Floor(ph)
		op := 0.3 + 0.7*math.Abs(0.5-ph)*2
		if op > 1 {
			op = 1
		}
		rendering.FillCircle(pc, cx+dx, cy+dy, radius, primary.R, primary.G, primary.B, primary.A*op)
	}
}

func (s *Spin) paintRing(pc *rendering.PaintContext, size rendering.Size, S SpinSnap) {
	dot := S.DotSize
	track := S.TrackColor
	cx := size.Width / 2
	top := s.sectionTopSnap(S, size, 0)
	if S.HasContent {
		top = (size.Height - dot) / 2
		if top < 0 {
			top = 0
		}
	}
	cy := top + dot/2
	stroke := dot * 0.2
	if stroke < 2 {
		stroke = 2
	}
	radius := dot/2 - stroke/2
	if radius < 1 {
		radius = 1
	}
	rendering.StrokeCircle(pc, cx, cy, radius, stroke, track.R, track.G, track.B, track.A)
	p := S.EffPercent
	if p <= 0 {
		return
	}
	if p > 100 {
		p = 100
	}
	start := -math.Pi / 2
	end := start + p/100*2*math.Pi
	arcInk := S.IndicatorColor
	rendering.StrokeArc(pc, cx, cy, radius, start, end, stroke, arcInk.R, arcInk.G, arcInk.B, arcInk.A)
}

func (s *Spin) paintCustom(pc *rendering.PaintContext, size rendering.Size, S SpinSnap, ind rendering.RenderObject, descH float64) {
	// Size comes from the UI-side Layout cache only (indSize); paint never
	// lays out nor writes back (R2-6). Unmeasured indicators fall back to
	// the dot size, matching indicatorHeight.
	w, h := S.DotSize, S.DotSize
	if v, ok := s.indSize.Load().(rendering.Size); ok && v.Width > 0 && v.Height > 0 {
		w, h = v.Width, v.Height
	}
	top := s.sectionTopSnap(S, size, descH)
	if S.HasContent {
		top = (size.Height - h) / 2
		if top < 0 {
			top = 0
		}
	}
	x := (size.Width - w) / 2
	if x < 0 {
		x = 0
	}
	ind.Paint(pc.WithOrigin(pc.OriginX+x, pc.OriginY+top))
}

func (s *Spin) paintDescription(pc *rendering.PaintContext, size rendering.Size, S SpinSnap, desc string, ink render.RGBA) {
	if s == nil || S.Face == nil {
		return
	}
	fs := S.FontSize
	_, descH := rendering.EstimateTextSize(desc, fs, 0.55)
	top := s.sectionTopSnap(S, size, descH)
	indH := s.indicatorHeightSnap(S)
	y := top + indH + S.Gap
	if S.HasContent {
		y = size.Height/2 + indH/2 + S.Gap/2
	}
	ax, ay := pc.Abs(size.Width/2, y)
	pc.DC.SetFont(S.Face)
	pc.DC.SetRGBA(ink.R, ink.G, ink.B, ink.A)
	pc.DC.DrawStringAnchored(desc, ax, ay, 0.5, 0)
}
