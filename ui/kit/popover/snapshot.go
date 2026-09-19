package popover

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// PopoverSnap is the frozen paint input for one popover frame (R2-6, button
// snapshot paradigm). The UI thread refreshes it in markPaint (funnel for
// every paint-affecting setter) and in NewPopover; the raster paint path
// reads only this value, never the live Popover. disabled/hovered/pressed/
// focused stay atomic live reads on the widget (event state); everything
// theme/config derived is frozen here.
type PopoverSnap struct {
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
	TriggerFontSz  float64
	TriggerNode    rendering.RenderObject
	TriggerLabel   string
	// Panel chrome/content frozen at refresh time.
	PanelBg          render.RGBA
	PanelBd          render.RGBA
	PanelRadius      float64
	PanelLineW       float64
	Pad              float64
	TitleMarginBottom float64
	FontSize         float64
	TitleColor       render.RGBA
	ContentColor     render.RGBA
	Title            string
	TitleNode        rendering.RenderObject
	Content          string
	ContentNode      rendering.RenderObject
	Arrow            bool
	ArrowSize        float64
	Face             text.Face
}

// refreshSnapshot freezes the current paint inputs (UI thread only). Pure
// value build: no raster-side field is touched, so a concurrent paint of
// the previous snapshot is unaffected.
func (p *Popover) refreshSnapshot() {
	if p == nil {
		return
	}
	tok := p.themeTokens()
	s := PopoverSnap{
		Bg:             themeToRGBA(tok.ColorBgContainer),
		Bd:             themeToRGBA(tok.ColorBorder),
		Tx:             themeToRGBA(tok.ColorText),
		BgDisabled:     themeToRGBA(tok.ColorFillTertiary),
		TxDisabled:     themeToRGBA(tok.ColorTextDisabled),
		Accent:         themeToRGBA(tok.ColorPrimary),
		Ring:           themeToRGBA(tok.ColorPrimary),
		TriggerRadius:  tok.Radius,
		TriggerFontSz:  tok.FontSize,
		TriggerNode:    p.triggerNode,
		TriggerLabel:   p.triggerLabel,
		PanelBg:        p.PanelBackground(),
		PanelBd:        p.BorderColor(),
		PanelRadius:    p.Radius(),
		PanelLineW:     p.LineWidth(),
		Pad:            p.InnerPadding(),
		TitleMarginBottom: p.TitleMarginBottom(),
		FontSize:       p.FontSize(),
		TitleColor:     p.TitleColor(),
		ContentColor:   p.ContentColor(),
		Title:          p.title,
		TitleNode:      p.titleNode,
		Content:        p.content,
		ContentNode:    p.contentNode,
		Arrow:          p.arrow,
		ArrowSize:      p.ArrowSize(),
		Face:           p.textFace,
	}
	if s.TriggerRadius <= 0 {
		s.TriggerRadius = 6
	}
	if s.PanelRadius <= 0 {
		s.PanelRadius = 8
	}
	p.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; zero value paints nothing
// harmful (empty strings, zero-alpha colors).
func (p *Popover) loadSnapshot() PopoverSnap {
	if p == nil {
		return PopoverSnap{}
	}
	if v, ok := p.snap.Load().(PopoverSnap); ok {
		return v
	}
	return PopoverSnap{}
}
