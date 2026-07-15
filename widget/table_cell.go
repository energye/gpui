package widget

import "github.com/energye/gpui/render"

// TableCell is a dense table cell paint unit.
type TableCell struct {
	Bounds Rect
	Text   string
	Header bool
	Align  Align
	// Grid draws right/bottom borders.
	Grid bool
}

// HitTest reports cell bounds hit.
func (c TableCell) HitTest(x, y float64) bool { return c.Bounds.Contains(x, y) }

// Draw paints the cell.
func (c TableCell) Draw(dc *render.Context, th Theme) {
	if th.ControlHeight <= 0 {
		th = DefaultTheme()
	}
	bg := th.Surface
	if c.Header {
		bg = th.HeaderBg
	}
	fillRect(dc, c.Bounds, bg)
	if c.Grid {
		setRGBA(dc, th.Border)
		dc.SetLineWidth(1)
		// right + bottom edges
		dc.DrawLine(c.Bounds.X+c.Bounds.W-0.5, c.Bounds.Y, c.Bounds.X+c.Bounds.W-0.5, c.Bounds.Y+c.Bounds.H)
		_ = dc.Stroke()
		dc.DrawLine(c.Bounds.X, c.Bounds.Y+c.Bounds.H-0.5, c.Bounds.X+c.Bounds.W, c.Bounds.Y+c.Bounds.H-0.5)
		_ = dc.Stroke()
	}
	if c.Text != "" {
		col := th.Text
		if c.Header {
			col = th.TextSecondary
		}
		setRGBA(dc, col)
		tw := ApproxTextWidth(c.Text, th.FontSize)
		x := textX(c.Bounds, th.PadX*0.75, c.Align, tw)
		y := textBaselineY(c.Bounds, th.FontSize)
		dc.DrawString(c.Text, x, y)
	}
}
