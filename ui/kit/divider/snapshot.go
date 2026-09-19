package divider

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// paintSnap is the frozen paint input for one divider frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it inside rebuild/markPaint
// (every setter funnels through one of the two); the raster-thread paint
// path reads only this value, never the live Divider. Setters may keep
// mutating the widget while raster paints — the pixels always match the
// last UI refresh, with no cross-thread reads or writes of widget state.
type paintSnap struct {
	// Vertical is the resolved orientation at refresh time.
	Vertical bool
	// HasTitle: a title is present (custom node or text; vertical never).
	HasTitle    bool
	HasTitleNod bool // custom title node present: paint skips the string
	Title       string
	Face        text.Face
	// Line style resolved against the theme at refresh time.
	LineColor render.RGBA
	LineWidth float64
	// MarginInline is the resolved vertical margin for vertical rails.
	MarginInline float64
	// Variant is the resolved dotted/dashed/solid at refresh time.
	Variant DividerVariant
	// RailGrows start/end factors for with-text rail re-derive.
	RailGrowStart, RailGrowEnd float64
	// Title style resolved against the theme at refresh time.
	TitleColor    render.RGBA
	TitleFontSize float64
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from rebuild and markPaint). Pure value build: no raster-side field is
// touched, so a concurrent paint of the previous snapshot is unaffected.
func (d *Divider) refreshSnapshot() {
	if d == nil {
		return
	}
	gs, ge := d.RailGrows()
	s := paintSnap{
		Vertical:      d.IsVertical(),
		HasTitle:      d.HasTitle(),
		HasTitleNod:   d.titleNode != nil,
		Title:         d.title,
		Face:          d.face,
		LineColor:     d.LineColor(),
		LineWidth:     d.LineWidth(),
		MarginInline:  d.MarginInline(),
		Variant:       d.EffectiveVariant(),
		RailGrowStart: gs,
		RailGrowEnd:   ge,
		TitleColor:    d.TitleColor(),
		TitleFontSize: d.TitleFontSize(),
	}
	d.paintCache.Store(s)
}

// paintSnapLocked loads the last frozen snapshot (zero before first refresh
// paints nothing: zero Variant is Solid, empty title skips the text).
func (d *Divider) paintSnapLocked() paintSnap {
	if d == nil {
		return paintSnap{}
	}
	if v, ok := d.paintCache.Load().(paintSnap); ok {
		return v
	}
	return paintSnap{}
}
