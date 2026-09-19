package alert

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// AlertSnap is the frozen paint input for one alert frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it on every paint-affecting
// setter (refreshSnapshot inside markPaint); the raster-thread paint path
// reads only this value, never the live Alert. Setters may keep mutating
// the widget while raster paints — the pixels always match the last UI
// refresh, with no cross-thread reads or writes of widget state.
type AlertSnap struct {
	// Background/Border resolved against the theme at refresh time.
	Bg       render.RGBA
	Radius   float64
	HasBd    bool
	Bd       render.RGBA
	LineW    float64
	// Icon: resolved name presence and color; custom icon node pointer is
	// copied into the snapshot (value copy — paint never reads the live
	// pointer field, it draws whatever this frame's snapshot holds).
	IconVisible bool
	IconColor   render.RGBA
	IconNode    rendering.RenderObject
	// Text block frozen at refresh time; custom title/desc nodes same
	// pointer-copy rule as IconNode.
	Title       string
	TitleNode   rendering.RenderObject
	Description string
	DescNode    rendering.RenderObject
	HasDesc     bool
	TextFace    text.Face
	TextColor   render.RGBA
	TitleFontSz float64
	DescFontSz  float64
	// Action block: presence + node pointer (custom node paints itself).
	ActionVisible bool
	ActionNode    rendering.RenderObject
	// Geometry inputs frozen at refresh time (pads, margins, icon size,
	// RTL, action box size) so paint never resolves them live.
	PadH, PadV         float64
	IconSize           float64
	MarginXXS, XS, SM  float64
	RTL                bool
	ActionW, ActionH   float64
	// Close block: closable/hidden stay atomic on the widget; the close
	// node pointer, glyph color and focus-ring color are frozen here.
	CloseNode       rendering.RenderObject
	CloseGlyphColor render.RGBA
	FocusRingColor  render.RGBA
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from markPaint). Pure value build: no raster-side field is touched, so a
// concurrent paint of the previous snapshot is unaffected.
func (a *Alert) refreshSnapshot() {
	if a == nil {
		return
	}
	tok := a.themeTokens()
	var actionW, actionH float64
	if a.ActionVisible() && a.action != nil {
		if sz := a.action.Size(); sz.Width > 0 {
			actionW, actionH = sz.Width, sz.Height
		} else {
			actionW, actionH = measureNode(a.action)
		}
	}
	s := AlertSnap{
		Bg:              a.Background(),
		Radius:          a.Radius(),
		HasBd:           a.HasBorder(),
		Bd:              a.BorderColor(),
		LineW:           a.LineWidth(),
		IconVisible:     a.IconVisible(),
		IconColor:       a.IconColor(),
		IconNode:        a.iconNode,
		Title:           a.title,
		TitleNode:       a.titleNode,
		Description:     a.description,
		DescNode:        a.descNode,
		HasDesc:         a.HasDescription(),
		TextFace:        a.textFace,
		TextColor:       themeToRGBA(tok.ColorText),
		TitleFontSz:     a.TitleFontSize(),
		DescFontSz:      tok.FontSize,
		ActionVisible:   a.ActionVisible(),
		ActionNode:      a.action,
		PadH:            a.PadH(),
		PadV:            a.PadV(),
		IconSize:        a.IconSize(),
		MarginXXS:       tok.MarginXXS,
		XS:              tok.MarginXS,
		SM:              tok.MarginSM,
		RTL:             a.rtl,
		ActionW:         actionW,
		ActionH:         actionH,
		CloseNode:       a.closeIcon,
		CloseGlyphColor: themeToRGBA(tok.ColorTextTertiary),
		FocusRingColor:  themeToRGBA(tok.ColorPrimary),
	}
	a.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (empty text, zero-alpha colors).
func (a *Alert) loadSnapshot() AlertSnap {
	if a == nil {
		return AlertSnap{}
	}
	if v, ok := a.snap.Load().(AlertSnap); ok {
		return v
	}
	return AlertSnap{}
}
