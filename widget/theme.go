package widget

import "github.com/energye/gpui/render"

// Theme holds shared visual tokens for the first-batch controls.
// Values are inspired by common design-system defaults but are not an Ant Design port.
type Theme struct {
	Primary       render.RGBA
	PrimaryHover  render.RGBA
	PrimaryText   render.RGBA
	DefaultBg     render.RGBA
	DefaultBorder render.RGBA
	DefaultText   render.RGBA
	Danger        render.RGBA
	DangerText    render.RGBA
	Surface       render.RGBA
	SurfaceAlt    render.RGBA
	Border        render.RGBA
	Text          render.RGBA
	TextSecondary render.RGBA
	TextDisabled  render.RGBA
	Placeholder   render.RGBA
	Error         render.RGBA
	Overlay       render.RGBA
	FocusRing     render.RGBA
	SelectedBg    render.RGBA
	HeaderBg      render.RGBA
	RadiusSM      float64
	RadiusMD      float64
	FontSize      float64
	ControlHeight float64
	PadX          float64
}

// DefaultTheme returns a light theme suitable for dense UI shells.
func DefaultTheme() Theme {
	return Theme{
		Primary:       render.RGBA{R: 0.13, G: 0.40, B: 0.90, A: 1},
		PrimaryHover:  render.RGBA{R: 0.20, G: 0.48, B: 0.95, A: 1},
		PrimaryText:   render.RGBA{R: 1, G: 1, B: 1, A: 1},
		DefaultBg:     render.RGBA{R: 1, G: 1, B: 1, A: 1},
		DefaultBorder: render.RGBA{R: 0.85, G: 0.86, B: 0.88, A: 1},
		DefaultText:   render.RGBA{R: 0.15, G: 0.16, B: 0.20, A: 1},
		Danger:        render.RGBA{R: 0.90, G: 0.25, B: 0.22, A: 1},
		DangerText:    render.RGBA{R: 1, G: 1, B: 1, A: 1},
		Surface:       render.RGBA{R: 1, G: 1, B: 1, A: 1},
		SurfaceAlt:    render.RGBA{R: 0.97, G: 0.97, B: 0.98, A: 1},
		Border:        render.RGBA{R: 0.88, G: 0.89, B: 0.91, A: 1},
		Text:          render.RGBA{R: 0.15, G: 0.16, B: 0.20, A: 1},
		TextSecondary: render.RGBA{R: 0.45, G: 0.47, B: 0.52, A: 1},
		TextDisabled:  render.RGBA{R: 0.70, G: 0.72, B: 0.75, A: 1},
		Placeholder:   render.RGBA{R: 0.65, G: 0.67, B: 0.70, A: 1},
		Error:         render.RGBA{R: 0.90, G: 0.25, B: 0.22, A: 1},
		Overlay:       render.RGBA{R: 0, G: 0, B: 0, A: 0.45},
		FocusRing:     render.RGBA{R: 0.25, G: 0.55, B: 0.98, A: 0.55},
		SelectedBg:    render.RGBA{R: 0.90, G: 0.94, B: 1.0, A: 1},
		HeaderBg:      render.RGBA{R: 0.96, G: 0.97, B: 0.98, A: 1},
		RadiusSM:      4,
		RadiusMD:      8,
		FontSize:      13,
		ControlHeight: 32,
		PadX:          12,
	}
}

func setRGBA(dc *render.Context, c render.RGBA) {
	dc.SetRGBA(c.R, c.G, c.B, c.A)
}

func fillRound(dc *render.Context, r Rect, radius float64, c render.RGBA) {
	setRGBA(dc, c)
	dc.DrawRoundedRectangle(r.X, r.Y, r.W, r.H, radius)
	_ = dc.Fill()
}

func strokeRound(dc *render.Context, r Rect, radius, width float64, c render.RGBA) {
	setRGBA(dc, c)
	dc.SetLineWidth(width)
	dc.DrawRoundedRectangle(r.X+width/2, r.Y+width/2, r.W-width, r.H-width, radius)
	_ = dc.Stroke()
}

func fillRect(dc *render.Context, r Rect, c render.RGBA) {
	setRGBA(dc, c)
	dc.DrawRectangle(r.X, r.Y, r.W, r.H)
	_ = dc.Fill()
}

// textBaselineY approximates a vertical center baseline for a control height.
func textBaselineY(r Rect, fontPts float64) float64 {
	// DrawString uses baseline; center-ish for Latin UI.
	return r.Y + r.H*0.5 + fontPts*0.35
}

func textX(r Rect, pad float64, align Align, approxTextW float64) float64 {
	switch align {
	case AlignCenter:
		return r.X + (r.W-approxTextW)*0.5
	case AlignRight:
		return r.X + r.W - pad - approxTextW
	default:
		return r.X + pad
	}
}

// ApproxTextWidth is a crude advance estimate when Face metrics are unavailable.
func ApproxTextWidth(s string, fontPts float64) float64 {
	if fontPts <= 0 {
		fontPts = 13
	}
	// ~0.55em average Latin/CJK mix heuristic for layout only.
	return float64(len([]rune(s))) * fontPts * 0.55
}
