package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Switch is an on/off toggle (Ant default 44×22).
// Thumb slide uses primitive.FloatAnim (shared demand-frame animation primitive).
type Switch struct {
	Root     *primitive.Pressable
	track    *primitive.Decorated
	thumb    *primitive.Decorated
	Checked  bool
	Disabled bool
	Theme    *core.Theme
	OnChange func(checked bool)
	// Style optional overrides (Background = off track, BackgroundActive = on track, etc.).
	Style Style

	trackW, trackH float64
	thumbSize      float64
	pad            float64
	lastHovered    bool
	lastPressed    bool

	// thumbPos is animated 0 (off) → 1 (on); left padding derives from it.
	thumbPos  primitive.FloatAnim
	boundTree *core.Tree
}

// NewSwitch creates a switch.
func NewSwitch() *Switch {
	s := &Switch{}
	s.rebuild()
	return s
}

// Node returns the root.
func (s *Switch) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// IndicatorNode returns the track (includes thumb) for visual tests.
func (s *Switch) IndicatorNode() core.Node {
	if s.track == nil {
		s.rebuild()
	}
	return s.track
}

// SetChecked updates state and animates the thumb.
func (s *Switch) SetChecked(v bool) {
	if s.Checked == v {
		return
	}
	s.Checked = v
	s.animateThumb()
	s.applyChrome()
}

// SetStyle applies visual overrides.
func (s *Switch) SetStyle(st Style) {
	s.Style = st
	s.applyChrome()
}

// SetBackground sets off-track color (A>0).
func (s *Switch) SetBackground(c render.RGBA) {
	s.Style.Background = c
	s.applyChrome()
}

// SetActiveColor sets on-track color (A>0).
func (s *Switch) SetActiveColor(c render.RGBA) {
	s.Style.BackgroundActive = c
	s.applyChrome()
}

// SetDisabled toggles disabled.
func (s *Switch) SetDisabled(d bool) {
	s.Disabled = d
	if s.Root != nil {
		s.Root.SetDisabled(d)
	}
	s.applyChrome()
}

// SyncState applies hover/press chrome (including Ant press thumb stretch).
func (s *Switch) SyncState() {
	if s.Root == nil {
		return
	}
	h, p := s.Root.State.Hovered, s.Root.State.Pressed
	if h == s.lastHovered && p == s.lastPressed {
		return
	}
	s.lastHovered, s.lastPressed = h, p
	s.applyThumbShape()
	s.applyChrome()
}

func (s *Switch) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Switch) rebuild() {
	th := s.theme()
	s.trackW = th.SizeOr(core.TokenSwitchWidth, 44)
	s.trackH = th.SizeOr(core.TokenSwitchHeight, 22)
	s.pad = 2
	s.thumbSize = s.trackH - 2*s.pad // 18

	// Shared FloatAnim for thumb travel.
	s.thumbPos.Duration = 0.3 // Ant motionDurationMid-ish; ease-in-out feels steadier
	s.thumbPos.Easing = primitive.EaseInOutCubic
	if s.Checked {
		s.thumbPos.Snap(1)
	} else {
		s.thumbPos.Snap(0)
	}
	s.thumbPos.OnUpdate = func(v float64) {
		s.applyThumbShape()
	}

	s.thumb = primitive.NewDecorated()
	s.thumb.Width, s.thumb.Height = s.thumbSize, s.thumbSize
	s.thumb.MinWidth, s.thumb.MinHeight = s.thumbSize, s.thumbSize
	s.thumb.Radius = s.thumbSize / 2 // true circle
	s.thumb.BorderWidth = 0
	s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}

	s.track = primitive.NewDecorated(s.thumb)
	s.track.Width, s.track.Height = s.trackW, s.trackH
	s.track.MinWidth, s.track.MinHeight = s.trackW, s.trackH
	s.track.Radius = s.trackH / 2 // pill
	s.track.BorderWidth = 0
	s.track.Padding = primitive.EdgeInsets{Left: s.pad, Top: s.pad, Right: s.pad, Bottom: s.pad}

	s.Root = primitive.NewPressable(s.track)
	s.Root.Focusable = true
	s.Root.ShowFocusRing = false          // Ant: no focus ring on switch
	s.Root.FocusRingRadius = s.trackH / 2 // pill: same-shape ripple (full round ends)
	s.Root.OnStateChange = s.SyncState
	s.Root.Click = func() {
		if s.Disabled {
			return
		}
		s.Checked = !s.Checked
		s.animateThumb()
		s.applyChrome()
		if s.OnChange != nil {
			s.OnChange(s.Checked)
		}
	}
	s.Root.SetDisabled(s.Disabled)
	s.applyChrome()
}

// AttachTicker registers Switch for demand-frame ANIMATING (thumb slide).
func (s *Switch) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.thumbPos.Active())
}

// Tick advances thumb animation. Implements core.Ticker.
func (s *Switch) Tick(dt float64) bool {
	if s == nil {
		return false
	}
	still := s.thumbPos.Tick(dt)
	if !still && s.boundTree != nil {
		s.boundTree.RemoveTicker(s)
	}
	return still
}

func (s *Switch) animateThumb() {
	to := 0.0
	if s.Checked {
		to = 1
	}
	// Prefer live tree from mounted Root so gallery works without AttachTicker.
	if s.boundTree == nil && s.Root != nil {
		s.boundTree = s.Root.Tree()
	}
	// Without a tree there is no demand-frame Tick — snap for headless/tests.
	if s.boundTree == nil {
		s.thumbPos.Snap(to)
		s.applyThumbPad(to)
		return
	}
	s.thumbPos.Duration = 0.3
	s.thumbPos.SetTarget(to)
	if s.thumbPos.Active() {
		s.boundTree.AddTicker(s)
	}
}

// thumbWidth returns handle width.
// Ant: press elongates ~+1/5; recovery is blended into the slide (not a snap on release).
func (s *Switch) thumbWidth() float64 {
	base := s.thumbSize
	if base <= 0 {
		base = 18
	}
	const stretch = 1.2 // +1/5
	// While finger down: fully stretched.
	if s.Root != nil && s.Root.State.Pressed && !s.Disabled {
		return base * stretch
	}
	// During slide: ease stretch → 1.0 with position animation progress (imperceptible recovery).
	if s.thumbPos.Active() {
		den := s.thumbPos.Target - s.thumbPos.From
		prog := 1.0
		if den != 0 {
			prog = (s.thumbPos.Current - s.thumbPos.From) / den
			if prog < 0 {
				prog = -prog
			}
			if prog > 1 {
				prog = 1
			}
		}
		// progress 0 → width 1.2×; progress 1 → width 1.0×
		return base * (stretch + (1-stretch)*prog)
	}
	return base
}

func (s *Switch) applyThumbShape() {
	if s.thumb == nil {
		return
	}
	w := s.thumbWidth()
	s.thumb.Width = w
	s.thumb.MinWidth = w
	s.thumb.Height = s.thumbSize
	s.thumb.MinHeight = s.thumbSize
	// Stadium: radius = half height so ends stay round when stretched.
	s.thumb.Radius = s.thumbSize / 2
	// Paint-only: do not MarkNeedsLayout (bubbles to tree root and thrashs scroll drag).
	s.applyThumbPad(s.thumbPos.Current)
}

func (s *Switch) applyThumbPad(t float64) {
	if s.track == nil {
		return
	}
	tw := s.thumbWidth()
	leftOff := s.pad
	leftOn := s.trackW - s.pad - tw
	if leftOn < s.pad {
		leftOn = s.pad
	}
	left := leftOff + (leftOn-leftOff)*t
	s.track.Padding = primitive.EdgeInsets{Left: left, Top: s.pad, Right: s.pad, Bottom: s.pad}
	// Local remeasure of fixed-size track only — never MarkNeedsLayout (that bubbles
	// past RepaintBoundary and forces full-tree layout every tick → scroll thumb jump).
	if s.track.Width > 0 && s.track.Height > 0 {
		_ = s.track.Layout(core.Tight(s.track.Width, s.track.Height))
	}
	s.track.MarkNeedsPaint()
	if s.thumb != nil {
		s.thumb.MarkNeedsPaint()
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

func (s *Switch) applyChrome() {
	if s.track == nil {
		return
	}
	th := s.theme()
	hovered := s.Root != nil && s.Root.State.Hovered && !s.Disabled
	if s.Checked {
		bg := th.Color(core.TokenColorPrimary)
		if hovered {
			bg = th.Color(core.TokenColorPrimaryHover)
		}
		if s.Style.hasBGActive() {
			bg = s.Style.BackgroundActive
		}
		s.track.Background = bg
	} else {
		bg := render.RGBA{R: 0, G: 0, B: 0, A: 0.25}
		if hovered {
			bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.35}
		}
		if s.Style.hasBG() {
			bg = s.Style.Background
			if hovered && s.Style.hasBGHover() {
				bg = s.Style.BackgroundHover
			}
		}
		s.track.Background = bg
	}
	if s.Disabled {
		s.track.Background = th.Color(core.TokenColorDisabledBg)
		s.track.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
		s.track.BorderColor = th.Color(core.TokenColorBorder)
	} else {
		s.track.BorderWidth = 0
	}
	s.applyThumbShape()
	if s.thumb != nil {
		s.thumb.Background = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		s.thumb.MarkNeedsPaint()
	}
}
