package tooltip

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// TooltipSnap is the frozen paint input for one tooltip frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it in markPaint (funnel for
// every paint-affecting setter) and in NewTooltip; the raster paint path
// reads only this value, never the live Tooltip. disabled/hovered/focused
// stay atomic live reads on the widget (event state); everything
// theme/config derived is frozen here.
type TooltipSnap struct {
	// Trigger chrome resolved at refresh time; the disabled/hovered branch
	// stays a live atomic read and picks among these frozen colors.
	Bg             render.RGBA
	Bd             render.RGBA
	Tx             render.RGBA
	BgDisabled     render.RGBA
	TxDisabled     render.RGBA
	Accent         render.RGBA
	Ring           render.RGBA
	TriggerRadius  float64
	TriggerLineW   float64
	TriggerFontSz  float64
	RingOutlineW   float64
	TriggerNode    rendering.RenderObject
	TriggerLabel   string
	// Panel chrome/content frozen at refresh time.
	PanelBg    render.RGBA
	PanelFg    render.RGBA
	PanelRad   float64
	PadX       float64
	PadY       float64
	FontSize   float64
	Title      string
	TitleNode  rendering.RenderObject
	Arrow      bool
	ArrowSize  float64
	Face       text.Face
}

// refreshSnapshot freezes the current paint inputs (UI thread only). Pure
// value build: no raster-side field is touched, so a concurrent paint of
// the previous snapshot is unaffected.
func (t *Tooltip) refreshSnapshot() {
	if t == nil {
		return
	}
	tok := t.themeTokens()
	s := TooltipSnap{
		Bg:            themeToRGBA(tok.ColorBgContainer),
		Bd:            themeToRGBA(tok.ColorBorder),
		Tx:            themeToRGBA(tok.ColorText),
		BgDisabled:    themeToRGBA(tok.ColorFillTertiary),
		TxDisabled:    themeToRGBA(tok.ColorTextDisabled),
		Accent:        themeToRGBA(tok.ColorPrimary),
		Ring:          themeToRGBA(tok.ColorPrimary),
		TriggerRadius: tok.Radius,
		TriggerLineW:  tok.LineWidth,
		TriggerFontSz: tok.FontSize,
		RingOutlineW:  tok.ControlOutlineWidth,
		TriggerNode:   t.triggerNode,
		TriggerLabel:  t.triggerLabel,
		PanelBg:       t.Background(),
		PanelFg:       t.TextColor(),
		PanelRad:      t.Radius(),
		PadX:          t.PadX(),
		PadY:          t.PadY(),
		FontSize:      t.FontSize(),
		Title:         t.title,
		TitleNode:     t.titleNode,
		Arrow:         t.arrow,
		ArrowSize:     t.ArrowSize(),
		Face:          t.textFace,
	}
	if s.TriggerRadius <= 0 {
		s.TriggerRadius = DefaultTooltipRadius
	}
	if s.TriggerLineW <= 0 {
		s.TriggerLineW = 1
	}
	if s.Ring == (render.RGBA{}) || s.Ring.A == 0 {
		s.Ring = render.RGBA{R: 0x16 / 255.0, G: 0x77 / 255.0, B: 0xff / 255.0, A: 1}
	}
	if s.Bg.A == 0 {
		s.Bg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	if s.RingOutlineW <= 0 {
		s.RingOutlineW = 2
	}
	t.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (empty strings, zero-alpha colors).
func (t *Tooltip) loadSnapshot() TooltipSnap {
	if t == nil {
		return TooltipSnap{}
	}
	if v, ok := t.snap.Load().(TooltipSnap); ok {
		return v
	}
	return TooltipSnap{}
}
