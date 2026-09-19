package tag

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// TagSnap is the frozen paint input for one tag frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it on every paint-affecting
// setter (refreshSnapshot inside markPaint/markLayout); the raster-thread
// paint path reads only this value, never the live Tag. Setters may keep
// mutating the widget while raster paints — the pixels always match the
// last UI refresh, with no cross-thread reads or writes of widget state.
type TagSnap struct {
	// Chrome resolved against the theme at refresh time (EffectiveChrome).
	Bg        render.RGBA
	Border    render.RGBA
	BorderW   float64
	Text      render.RGBA
	// Geometry resolved at refresh time.
	Radius   float64
	PadH     float64
	IconSize float64
	FontSize float64
	// Content frozen at refresh time.
	HasIcon  bool
	Closable bool
	Label    string
	Face     text.Face
	// Focus ring color resolved at refresh time (visibility stays the
	// atomic focused read on the widget).
	RingColor render.RGBA
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from markPaint and markLayout). Pure value build: no raster-side field is
// touched, so a concurrent paint of the previous snapshot is unaffected.
func (t *Tag) refreshSnapshot() {
	if t == nil {
		return
	}
	tok := t.themeTokens()
	ch := t.EffectiveChrome()
	s := TagSnap{
		Bg:        ch.Bg,
		Border:    ch.Border,
		BorderW:   ch.BorderWidth,
		Text:      ch.Text,
		Radius:    t.EffectiveRadius(),
		PadH:      t.EffectivePadH(),
		IconSize:  t.EffectiveIconSize(),
		FontSize:  t.EffectiveFontSize(),
		HasIcon:   t.HasIcon(),
		Closable:  t.closable,
		Label:     t.label,
		Face:      t.face,
		RingColor: themeToRGBA(tok.ColorPrimary),
	}
	t.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (empty label, zero-alpha colors).
func (t *Tag) loadSnapshot() TagSnap {
	if t == nil {
		return TagSnap{}
	}
	if v, ok := t.snap.Load().(TagSnap); ok {
		return v
	}
	return TagSnap{}
}
