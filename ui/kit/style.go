package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
)

// Style holds optional visual overrides for kit controls.
// Zero values mean "use Theme / size defaults".
//
//	btn.SetStyle(kit.Style{
//	    Background: render.Hex("#1677FF"),
//	    Text:       render.Hex("#FFFFFF"),
//	    FontSize:   14,
//	    Height:     32,
//	    Radius:     6,
//	})
//
// Prefer Theme tokens for app-wide theming; use Style for one-off overrides.
type Style struct {
	// Background is the idle fill (A>0 to apply).
	Background render.RGBA
	// BackgroundHover / BackgroundActive optional state fills.
	BackgroundHover  render.RGBA
	BackgroundActive render.RGBA
	// Border is the idle border color (A>0 to apply).
	Border render.RGBA
	// Text is the label/value color (A>0 to apply).
	Text render.RGBA
	// FontSize in points; 0 = theme default for control size.
	FontSize float64
	// Face optional font face.
	Face text.Face
	// Height forces control height (Button/Input/Select); 0 = size token.
	Height float64
	// Width optional fixed width; 0 = content/min.
	Width float64
	// Radius corner radius; 0 = theme default (unless ForceRadius).
	Radius float64
	// ForceRadius applies Radius even when 0 (square).
	ForceRadius bool
}

func (st Style) hasBG() bool       { return st.Background.A > 0 }
func (st Style) hasBGHover() bool  { return st.BackgroundHover.A > 0 }
func (st Style) hasBGActive() bool { return st.BackgroundActive.A > 0 }
func (st Style) hasBorder() bool   { return st.Border.A > 0 }
func (st Style) hasText() bool     { return st.Text.A > 0 }
func (st Style) hasRadius() bool   { return st.ForceRadius || st.Radius > 0 }
