package layout

import (
	"github.com/energye/gpui/render"
)

// SiderSnap is the frozen paint input for one sider frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it in every paint-affecting
// setter and in NewSider; the raster paint path reads only this value.
// collapsed/focused/hovered stay atomic live reads on the widget (event
// state, same rule as the other sections); everything theme/config derived
// is frozen here so raster never touches mutable Sider fields.
type SiderSnap struct {
	Bg        render.RGBA
	TriggerBg render.RGBA
	ArrowCol  render.RGBA // arrow stroke color resolved at refresh time
	RingColor render.RGBA // focus ring color resolved at refresh time
	// Geometry/direction frozen at refresh time (collapsed participates via
	// the atomic state read inside refreshSnapshot on the UI thread).
	TriggerVisible bool
	ZeroTrigger    bool
	TriggerW       float64
	TriggerH       float64
	ArrowLeft      bool
}

// refreshSnapshot freezes the current paint inputs (UI thread only). Pure
// value build: no raster-side field is touched, so a concurrent paint of
// the previous snapshot is unaffected.
func (s *Sider) refreshSnapshot() {
	if s == nil {
		return
	}
	s.snap.Store(SiderSnap{
		Bg:             s.EffectiveBackground(),
		TriggerBg:      s.EffectiveTriggerBackground(),
		ArrowCol:       s.effectiveArrowColor(),
		RingColor:      s.effectiveRingColor(),
		TriggerVisible: s.TriggerVisible(),
		ZeroTrigger:    s.IsZeroTrigger(),
		TriggerW:       s.EffectiveTriggerWidth(),
		TriggerH:       s.EffectiveTriggerHeight(),
		ArrowLeft:      s.ArrowPointsLeft(),
	})
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (zero-alpha fills, no trigger).
func (s *Sider) loadSnapshot() SiderSnap {
	if s == nil {
		return SiderSnap{}
	}
	if v, ok := s.snap.Load().(SiderSnap); ok {
		return v
	}
	return SiderSnap{}
}

// effectiveArrowColor resolves the arrow stroke color (theme-derived, light
// theme text vs white on dark).
func (s *Sider) effectiveArrowColor() render.RGBA {
	if s.siderTheme == SiderThemeLight {
		tok := s.themeTokens()
		c := themeToRGBA(tok.ColorText)
		c.A = 1
		return c
	}
	return render.RGBA{R: 1, G: 1, B: 1, A: 1}
}

// effectiveRingColor resolves the focus ring color (theme primary with the
// same fallback paintFocusRing used before snapshotting).
func (s *Sider) effectiveRingColor() render.RGBA {
	tok := s.themeTokens()
	c := themeToRGBA(tok.ColorPrimary)
	if c.A <= 0 {
		c = render.RGBA{R: 0.09, G: 0.47, B: 1, A: 1}
	}
	return c
}
