package divider

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

func (d *Divider) paint(pc *rendering.PaintContext, size rendering.Size) {
	if d == nil || pc == nil || pc.DC == nil {
		return
	}
	w, h := size.Width, size.Height
	if w <= 0 {
		w = d.lastW
	}
	if h <= 0 {
		h = d.lastH
	}
	if w <= 0 || h <= 0 {
		return
	}
	lc := d.LineColor()
	lw := d.LineWidth()
	if lw <= 0 {
		lw = 1
	}
	if d.IsVertical() {
		mi := d.MarginInline()
		x := mi + lw/2
		d.paintRail(pc, x, 0, x, h, lc, lw)
		return
	}
	if !d.HasTitle() {
		y := h / 2
		d.paintRail(pc, 0, y, w, y, lc, lw)
		return
	}
	railY := d.lastRailY + lw/2
	if railY <= 0 || railY > h {
		railY = h / 2
	}
	startW := d.lastRailStart
	titleW := d.lastTitleW
	// Re-derive split from paint size so retained paints stay aligned
	// even when layout was bypassed by a direct root.Layout call.
	if w != d.lastW {
		avail := w - titleW
		if avail < 0 {
			avail = 0
		}
		gs, ge := d.RailGrows()
		sum := gs + ge
		if sum > 0 {
			startW = avail * gs / sum
		}
	}
	if startW > 0 {
		d.paintRail(pc, 0, railY, startW, railY, lc, lw)
	}
	sx := startW + titleW
	if sx < w {
		d.paintRail(pc, sx, railY, w, railY, lc, lw)
	}
	d.paintTitle(pc)
}

func (d *Divider) paintTitle(pc *rendering.PaintContext) {
	if d == nil || !d.HasTitle() || d.titleNode != nil {
		return
	}
	if d.title == "" || pc == nil || pc.DC == nil {
		return
	}
	tc := d.TitleColor()
	fs := d.TitleFontSize()
	if d.face != nil {
		pc.DC.SetFont(d.face)
	}
	pc.DC.SetRGBA(tc.R, tc.G, tc.B, tc.A)
	// Baseline sits below the title top; without font metrics use 0.8em.
	// No black-bar fallback: without a face DrawString no-ops, layout
	// keeps the estimate width so the rails stay split.
	y := d.lastTitleY + fs*0.8
	x := d.lastTitleX + fs
	ax, ay := pc.Abs(x, y)
	pc.DC.DrawString(d.title, ax, ay)
}

func (d *Divider) paintRail(pc *rendering.PaintContext, x1, y1, x2, y2 float64, lc render.RGBA, lw float64) {
	switch d.EffectiveVariant() {
	case Dotted:
		paintDotted(pc, x1, y1, x2, y2, lc, lw)
	case Dashed:
		paintDashed(pc, x1, y1, x2, y2, lc, lw)
	default:
		paintSolid(pc, x1, y1, x2, y2, lc, lw)
	}
}

func paintSolid(pc *rendering.PaintContext, x1, y1, x2, y2 float64, lc render.RGBA, lw float64) {
	if x1 == x2 {
		x := x1 - lw/2
		if x < 0 {
			x = 0
		}
		h := y2 - y1
		if h <= 0 {
			return
		}
		rendering.FillRect(pc, x, y1, lw, h, lc.R, lc.G, lc.B, lc.A)
		return
	}
	y := y1 - lw/2
	if y < 0 {
		y = 0
	}
	w := x2 - x1
	if w <= 0 {
		return
	}
	rendering.FillRect(pc, x1, y, w, lw, lc.R, lc.G, lc.B, lc.A)
}

func paintDashed(pc *rendering.PaintContext, x1, y1, x2, y2 float64, lc render.RGBA, lw float64) {
	if pc == nil || pc.DC == nil {
		return
	}
	pc.DC.SetRGBA(lc.R, lc.G, lc.B, lc.A)
	pc.DC.SetLineWidth(lw)
	pc.DC.SetDash(6, 4)
	pc.DC.DrawLine(pc.OriginX+x1, pc.OriginY+y1, pc.OriginX+x2, pc.OriginY+y2)
	_ = pc.DC.Stroke()
	pc.DC.ClearDash()
}

func paintDotted(pc *rendering.PaintContext, x1, y1, x2, y2 float64, lc render.RGBA, lw float64) {
	horizontal := y1 == y2
	r := lw / 2
	if r < 0.8 {
		r = 0.8
	}
	step := lw * 3
	if step < 4 {
		step = 4
	}
	if horizontal {
		for x := x1 + r; x <= x2-r+0.5; x += step {
			rendering.FillCircle(pc, x, y1, r, lc.R, lc.G, lc.B, lc.A)
		}
		return
	}
	for y := y1 + r; y <= y2-r+0.5; y += step {
		rendering.FillCircle(pc, x1, y, r, lc.R, lc.G, lc.B, lc.A)
	}
}
