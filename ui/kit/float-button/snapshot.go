package float_button

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// FloatSnap is the frozen paint input for one float-button frame (R2-6,
// button snapshot paradigm). The UI thread refreshes it inside dirty()
// (every paint-affecting setter funnels through dirty); the raster-thread
// paint path reads only this value, never the live FloatButton. Setters may
// keep mutating the widget while raster paints — the pixels always match
// the last UI refresh, with no cross-thread reads or writes of widget state.
type FloatSnap struct {
	// Shape is the resolved circle/square at refresh time.
	Shape FloatButtonShape
	// Resolved chrome colors and metrics at refresh time (Effective*
	// applied against the theme plus hover/press/disabled state).
	Bg, Bd, Fg render.RGBA
	BorderW    float64
	Radius     float64
	// Icon is the resolved glyph (empty paints none) and its edge.
	Icon     string
	IconSize float64
	// Content text and its font size/face at refresh time.
	Content         string
	ContentFontSize float64
	TextFace        text.Face
	// State flags frozen at refresh time.
	FocusRing bool
	Loading   bool
	// Focus ring color/width resolved at refresh time.
	RingColor render.RGBA
	RingWidth float64
	// Badge overlay frozen at refresh time.
	BadgeVisible bool
	BadgeDot     bool
	BadgeLabel   string
	BadgeBg      render.RGBA
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from dirty). Pure value build: no raster-side field is touched, so a
// concurrent paint of the previous snapshot is unaffected.
func (b *FloatButton) refreshSnapshot() {
	if b == nil {
		return
	}
	s := FloatSnap{
		Shape:           b.Shape(),
		Bg:              b.EffectiveBackground(),
		Bd:              b.EffectiveBorder(),
		Fg:              b.EffectiveForeground(),
		BorderW:         b.EffectiveBorderWidth(),
		Radius:          b.EffectiveRadius(),
		Icon:            b.EffectiveIcon(),
		IconSize:        b.EffectiveIconSize(),
		Content:         b.content,
		ContentFontSize: b.ContentFontSize(),
		TextFace:        b.textFace,
		FocusRing:       b.FocusRingVisible(),
		Loading:         b.loading,
		BadgeVisible:    b.BadgeVisible(),
		BadgeDot:        b.badgeDot,
		BadgeLabel:      b.BadgeText(),
		BadgeBg:         b.BadgeBackground(),
	}
	if s.FocusRing {
		tok := b.themeTokens()
		s.RingColor = themeToRGBA(tok.ColorPrimary)
		s.RingWidth = tok.ControlOutlineWidth
		if s.RingWidth <= 0 {
			s.RingWidth = 2
		}
	}
	b.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; never nil for a live widget
// (zero value paints nothing: empty icon/content, zero variant colors).
func (b *FloatButton) loadSnapshot() FloatSnap {
	if b == nil {
		return FloatSnap{}
	}
	if v, ok := b.snap.Load().(FloatSnap); ok {
		return v
	}
	return FloatSnap{}
}
