package widget

import "github.com/energye/gpui/render"

// ButtonStyle selects visual treatment.
type ButtonStyle int

const (
	ButtonPrimary ButtonStyle = iota
	ButtonDefault
	ButtonDanger
	ButtonText
)

// Button is a paint-only control.
type Button struct {
	Bounds   Rect
	Label    string
	Style    ButtonStyle
	Disabled bool
	Hovered  bool
	Pressed  bool
	Focused  bool
}

// HitTest reports whether (x,y) is inside the button bounds.
func (b Button) HitTest(x, y float64) bool { return b.Bounds.Contains(x, y) }

// Draw paints the button into dc using th. Caller must have a font loaded if Label is non-empty.
func (b Button) Draw(dc *render.Context, th Theme) {
	if th.ControlHeight <= 0 {
		th = DefaultTheme()
	}
	r := b.Bounds
	if r.H <= 0 {
		r.H = th.ControlHeight
	}
	radius := th.RadiusSM

	var bg, fg, border render.RGBA
	switch b.Style {
	case ButtonDanger:
		bg, fg = th.Danger, th.DangerText
		if b.Pressed && !b.Disabled {
			bg = render.RGBA{R: 0.75, G: 0.18, B: 0.16, A: 1}
		} else if b.Hovered && !b.Disabled {
			bg = render.RGBA{R: 0.95, G: 0.32, B: 0.28, A: 1}
		}
		border = bg
	case ButtonText:
		bg = render.RGBA{R: 0, G: 0, B: 0, A: 0}
		fg = th.Primary
		border = bg
		if b.Pressed && !b.Disabled {
			bg = render.RGBA{R: 0.13, G: 0.40, B: 0.90, A: 0.16}
		} else if b.Hovered && !b.Disabled {
			bg = render.RGBA{R: 0.13, G: 0.40, B: 0.90, A: 0.08}
		}
	case ButtonDefault:
		bg, fg, border = th.DefaultBg, th.DefaultText, th.DefaultBorder
		if b.Pressed && !b.Disabled {
			border = th.Primary
			fg = th.Primary
			bg = render.RGBA{R: 0.93, G: 0.95, B: 1.0, A: 1}
		} else if b.Hovered && !b.Disabled {
			border = th.Primary
			fg = th.Primary
		}
	default: // Primary
		bg, fg = th.Primary, th.PrimaryText
		if b.Pressed && !b.Disabled {
			bg = render.RGBA{R: 0.10, G: 0.32, B: 0.78, A: 1}
		} else if b.Hovered && !b.Disabled {
			bg = th.PrimaryHover
		}
		border = bg
	}
	if b.Disabled {
		if b.Style == ButtonPrimary || b.Style == ButtonDanger {
			bg = render.RGBA{R: bg.R*0.5 + 0.5, G: bg.G*0.5 + 0.5, B: bg.B*0.5 + 0.5, A: 1}
			fg = th.TextDisabled
		} else {
			fg = th.TextDisabled
			border = th.Border
		}
	}

	if bg.A > 0 {
		fillRound(dc, r, radius, bg)
	}
	if b.Style == ButtonDefault || b.Style == ButtonText {
		if b.Style == ButtonDefault {
			strokeRound(dc, r, radius, 1, border)
		}
	}
	if b.Focused && !b.Disabled {
		strokeRound(dc, r.Inset(-2), radius+2, 2, th.FocusRing)
	}

	if b.Label != "" {
		setRGBA(dc, fg)
		tw := ApproxTextWidth(b.Label, th.FontSize)
		x := textX(r, th.PadX, AlignCenter, tw)
		y := textBaselineY(r, th.FontSize)
		dc.DrawString(b.Label, x, y)
	}
}
