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
	// R2-6: one frozen geometry load (UI-stored at layout, read here on
	// raster) — never nine separate live reads.
	L := d.layoutSnap()
	if w <= 0 {
		w = L.w
	}
	if h <= 0 {
		h = L.h
	}
	if w <= 0 || h <= 0 {
		return
	}
	// R2-6: one frozen paint-input load (UI-stored at rebuild/markPaint,
	// read here on raster) — never live reads of title/face/style.
	S := d.paintSnapLocked()
	lc := S.LineColor
	lw := S.LineWidth
	if lw <= 0 {
		lw = 1
	}
	if S.Vertical {
		x := S.MarginInline + lw/2
		d.paintRail(pc, x, 0, x, h, lc, lw, S)
		return
	}
	if !S.HasTitle {
		y := h / 2
		d.paintRail(pc, 0, y, w, y, lc, lw, S)
		return
	}
	railY := L.railY + lw/2
	if railY <= 0 || railY > h {
		railY = h / 2
	}
	startW := L.railStart
	titleW := L.titleW
	// Re-derive split from paint size so retained paints stay aligned
	// even when layout was bypassed by a direct root.Layout call.
	if w != L.w {
		avail := w - titleW
		if avail < 0 {
			avail = 0
		}
		gs, ge := S.RailGrowStart, S.RailGrowEnd
		sum := gs + ge
		if sum > 0 {
			startW = avail * gs / sum
		}
	}
	if startW > 0 {
		d.paintRail(pc, 0, railY, startW, railY, lc, lw, S)
	}
	sx := startW + titleW
	if sx < w {
		d.paintRail(pc, sx, railY, w, railY, lc, lw, S)
	}
	d.paintTitle(pc, L, S)
}

func (d *Divider) paintTitle(pc *rendering.PaintContext, L dividerLayout, S paintSnap) {
	if d == nil || !S.HasTitle || S.HasTitleNod {
		return
	}
	if S.Title == "" || pc == nil || pc.DC == nil {
		return
	}
	tc := S.TitleColor
	fs := S.TitleFontSize
	if S.Face != nil {
		pc.DC.SetFont(S.Face)
	}
	pc.DC.SetRGBA(tc.R, tc.G, tc.B, tc.A)
	// Baseline sits below the title top; without font metrics use 0.8em.
	// No black-bar fallback: without a face DrawString no-ops, layout
	// keeps the estimate width so the rails stay split.
	y := L.titleY + fs*0.8
	x := L.titleX + fs
	ax, ay := pc.Abs(x, y)
	pc.DC.DrawString(S.Title, ax, ay)
}

func (d *Divider) paintRail(pc *rendering.PaintContext, x1, y1, x2, y2 float64, lc render.RGBA, lw float64, S paintSnap) {
	switch S.Variant {
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
