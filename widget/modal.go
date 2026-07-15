package widget

import "github.com/energye/gpui/render"

// Modal is a centered dialog panel with optional dimming overlay.
type Modal struct {
	// Host is the full window/surface size used for the dim overlay.
	HostW, HostH float64
	// Panel is the dialog bounds (caller positions; helper CenterPanel available).
	Panel       Rect
	Title       string
	Body        string
	OKLabel     string
	CancelLabel string
	// ShowOverlay draws a semi-transparent host scrim behind the panel.
	ShowOverlay bool
}

// CenterPanel returns a panel of size (pw,ph) centered in host.
func CenterPanel(hostW, hostH, pw, ph float64) Rect {
	return Rect{X: (hostW - pw) * 0.5, Y: (hostH - ph) * 0.5, W: pw, H: ph}
}

// OKButton returns the primary action button geometry.
func (m Modal) OKButton(th Theme) Button {
	label := m.OKLabel
	if label == "" {
		label = "OK"
	}
	bw := 96.0
	bh := th.ControlHeight
	return Button{
		Bounds: Rect{X: m.Panel.X + m.Panel.W - th.PadX - bw, Y: m.Panel.Y + m.Panel.H - th.PadX - bh, W: bw, H: bh},
		Label:  label,
		Style:  ButtonPrimary,
	}
}

// CancelButton returns the secondary action button geometry (left of OK).
func (m Modal) CancelButton(th Theme) Button {
	label := m.CancelLabel
	if label == "" {
		label = "Cancel"
	}
	ok := m.OKButton(th)
	bw := 96.0
	return Button{
		Bounds: Rect{X: ok.Bounds.X - 8 - bw, Y: ok.Bounds.Y, W: bw, H: ok.Bounds.H},
		Label:  label,
		Style:  ButtonDefault,
	}
}

// Draw paints overlay + panel + title/body + action buttons.
func (m Modal) Draw(dc *render.Context, th Theme) {
	if th.ControlHeight <= 0 {
		th = DefaultTheme()
	}
	if m.ShowOverlay && m.HostW > 0 && m.HostH > 0 {
		fillRect(dc, Rect{X: 0, Y: 0, W: m.HostW, H: m.HostH}, th.Overlay)
	}
	// Panel surface with subtle elevation via rounded rect.
	fillRound(dc, m.Panel, th.RadiusMD, th.Surface)
	strokeRound(dc, m.Panel, th.RadiusMD, 1, th.Border)

	if m.Title != "" {
		setRGBA(dc, th.Text)
		dc.DrawString(m.Title, m.Panel.X+th.PadX*1.5, m.Panel.Y+36)
	}
	if m.Body != "" {
		setRGBA(dc, th.TextSecondary)
		dc.DrawString(m.Body, m.Panel.X+th.PadX*1.5, m.Panel.Y+72)
	}
	m.CancelButton(th).Draw(dc, th)
	m.OKButton(th).Draw(dc, th)
}
