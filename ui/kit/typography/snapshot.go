package typography

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
)

// PaintSnap is the frozen paint input for one typography frame (T2 D1).
// The UI thread refreshes it on every layout/paint-dirty (refreshSnapshot);
// the raster-thread paint path reads only this value, never the live widget.
type PaintSnap struct {
	Disp               string
	FontSize           float64
	Face               text.Face
	Color              render.RGBA
	UseBg              bool
	Bg                 render.RGBA
	Mark               bool
	Code, Keyboard     bool
	CodeBg             render.RGBA
	BorderColor        render.RGBA
	X0                 float64
	Underline, Deleted bool
	FocusRing          bool
	RingColor          render.RGBA
	ActionCount        int
	ActionsStart       bool
	ActionColor        render.RGBA
}

// refreshSnapshot freezes the current paint inputs (UI thread only).
func (t *Typography) refreshSnapshot() {
	if t == nil {
		return
	}
	var s PaintSnap
	disp := t.DisplayText()
	if disp == "" {
		disp = t.value
	}
	s.Disp = disp
	s.FontSize = t.EffectiveFontSize()
	s.Face = t.face
	s.Color = t.EffectiveColor()
	s.UseBg = t.style.UseBg
	s.Bg = t.style.Bg
	s.Mark = t.mark
	s.Code, s.Keyboard = t.code, t.keyboard
	if t.code || t.keyboard {
		s.CodeBg = t.CodeBg()
	}
	tok := t.themeTokens()
	s.BorderColor = themeToRGBA(tok.ColorBorder)
	x0 := 0.0
	if t.code || t.keyboard {
		x0 = 4
	}
	if t.actionsPlacement == PlacementStart {
		x0 += t.ActionWidth()
	}
	s.X0 = x0
	s.Underline, s.Deleted = t.underline, t.deleted
	if t.FocusRingVisible() {
		s.FocusRing = true
		s.RingColor = themeToRGBA(tok.ColorPrimary)
	}
	s.ActionCount = t.ActionCount()
	s.ActionsStart = t.actionsPlacement == PlacementStart
	s.ActionColor = themeToRGBA(tok.ColorTextSecondary)
	t.paintSnap.Store(s)
}

// loadSnap returns the last UI refresh (zero value before the first one).
func (t *Typography) loadSnap() PaintSnap {
	if t == nil {
		return PaintSnap{}
	}
	if s, ok := t.paintSnap.Load().(PaintSnap); ok {
		return s
	}
	return PaintSnap{}
}

// textWidthOf estimates the advance of s (same formula as TextWidth:
// ascii 0.6em, wide 1em) without touching the live widget.
func textWidthOf(s string, fontSize float64) float64 {
	var w float64
	for _, r := range s {
		if r < 128 {
			w += 0.6 * fontSize
		} else {
			w += 1.0 * fontSize
		}
	}
	return w
}

// PaintTypo paints text chrome from a frozen snapshot (raster thread only).
// Same order and geometry as paint, pixel-identical for the same inputs.
func PaintTypo(pc *rendering.PaintContext, size rendering.Size, s PaintSnap) {
	if pc == nil || pc.DC == nil {
		return
	}
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	if s.UseBg {
		rendering.FillRect(pc, 0, 0, w, h, s.Bg.R, s.Bg.G, s.Bg.B, s.Bg.A)
	} else if s.Mark {
		mb := MarkBg()
		rendering.FillRect(pc, 0, 0, w, h, mb.R, mb.G, mb.B, mb.A)
	}
	if s.Code {
		rendering.FillRoundRect(pc, 0, 0, w, h, CodeRadius, s.CodeBg.R, s.CodeBg.G, s.CodeBg.B, s.CodeBg.A)
	}
	if s.Keyboard {
		rendering.FillRoundRect(pc, 0, 0, w, h, CodeRadius, s.CodeBg.R, s.CodeBg.G, s.CodeBg.B, s.CodeBg.A)
		lc := s.BorderColor
		rendering.StrokeRoundRect(pc, 0, 0, w, h, CodeRadius, LineWidth, lc.R, lc.G, lc.B, lc.A)
	}
	disp := s.Disp
	fs := s.FontSize
	if s.Face != nil {
		pc.DC.SetFont(s.Face)
	}
	pc.DC.SetRGBA(s.Color.R, s.Color.G, s.Color.B, s.Color.A)
	x := s.X0
	baseline := h/2 + fs*0.35
	if baseline < fs*0.8 {
		baseline = fs * 0.8
	}
	ax, ay := pc.Abs(x, baseline)
	// Same condition as paint (TrimSpace check or non-empty collapses to
	// non-empty); without a loaded face DrawString is a safe no-op.
	if disp != "" {
		pc.DC.DrawString(disp, ax, ay)
	}
	tw := textWidthOf(disp, fs)
	if s.Underline {
		y := baseline + 2
		rendering.StrokeLine(pc, x, y, x+tw, y, 1, s.Color.R, s.Color.G, s.Color.B, s.Color.A)
	}
	if s.Deleted {
		y := baseline - fs*0.25
		rendering.StrokeLine(pc, x, y, x+tw, y, 1, s.Color.R, s.Color.G, s.Color.B, s.Color.A)
	}
	if s.FocusRing {
		rc := s.RingColor
		rendering.StrokeRoundRect(pc, -FocusRingOutset, -FocusRingOutset, w+2*FocusRingOutset, h+2*FocusRingOutset, ContainerRadius, 2, rc.R, rc.G, rc.B, rc.A)
	}
	n := s.ActionCount
	if n > 0 {
		ax0 := w - float64(n)*(ActionIconSize+ActionGap) + ActionGap
		if s.ActionsStart {
			ax0 = 0
		}
		ay0 := h/2 - ActionIconSize/2
		ac := s.ActionColor
		for i := 0; i < n; i++ {
			rendering.FillRect(pc, ax0+float64(i)*(ActionIconSize+ActionGap), ay0, 6, 6, ac.R, ac.G, ac.B, ac.A)
		}
	}
}
