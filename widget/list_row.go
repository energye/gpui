package widget

import "github.com/energye/gpui/render"

// ListRow is a single virtual-list row paint unit.
type ListRow struct {
	Bounds   Rect
	Title    string
	Subtitle string
	Selected bool
	Hovered  bool
	Pressed  bool
	// ShowAvatar draws a leading circle glyph placeholder.
	ShowAvatar bool
}

// HitTest reports row bounds hit.
func (r ListRow) HitTest(x, y float64) bool { return r.Bounds.Contains(x, y) }

// Draw paints the row.
func (r ListRow) Draw(dc *render.Context, th Theme) {
	if th.ControlHeight <= 0 {
		th = DefaultTheme()
	}
	bg := th.Surface
	if r.Selected {
		bg = th.SelectedBg
	} else if r.Pressed {
		bg = render.RGBA{R: 0.90, G: 0.91, B: 0.93, A: 1}
	} else if r.Hovered {
		bg = th.SurfaceAlt
	}
	fillRect(dc, r.Bounds, bg)

	// bottom hairline separator
	setRGBA(dc, th.Border)
	dc.SetLineWidth(1)
	dc.DrawLine(r.Bounds.X, r.Bounds.Y+r.Bounds.H-0.5, r.Bounds.X+r.Bounds.W, r.Bounds.Y+r.Bounds.H-0.5)
	_ = dc.Stroke()

	x := r.Bounds.X + th.PadX
	if r.ShowAvatar {
		cx := x + 12
		cy := r.Bounds.Y + r.Bounds.H*0.5
		setRGBA(dc, th.Primary)
		dc.DrawCircle(cx, cy, 10)
		_ = dc.Fill()
		x += 28
	}
	if r.Title != "" {
		setRGBA(dc, th.Text)
		dc.DrawString(r.Title, x, r.Bounds.Y+r.Bounds.H*0.5+th.FontSize*0.2)
	}
	if r.Subtitle != "" {
		setRGBA(dc, th.TextSecondary)
		dc.DrawString(r.Subtitle, x, r.Bounds.Y+r.Bounds.H*0.5+th.FontSize*1.1)
	}
}
