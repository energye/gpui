package icon

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/rendering"
)

// PaintSnap is the frozen paint input for one icon frame (T2 D1).
// The UI thread refreshes it on every dirty/sync (refreshSnapshot); the
// raster-thread paint path reads only this value, never the live Icon.
type PaintSnap struct {
	Name                    string
	Base                    render.RGBA
	Primary, Secondary       render.RGBA
	Angle                   float64
	Custom                  Painter
	FamilyPainter           Painter
	FamDef                  Def
	FamDefOK                bool
}

// refreshSnapshot freezes the current paint inputs (UI thread only).
func (ic *Icon) refreshSnapshot() {
	if ic == nil {
		return
	}
	var s PaintSnap
	s.Name = ic.name
	s.Base = ic.EffectiveColor()
	s.Primary, s.Secondary = ic.TwoToneColors()
	s.Angle = ic.EffectiveAngle()
	s.Custom = ic.painter
	if p, ok := ic.family.lookupPainter(ic.name); ok && p != nil {
		s.FamilyPainter = p
	}
	if d, ok := ic.resolveDef(); ok {
		s.FamDef, s.FamDefOK = d, true
	}
	ic.paintSnap.Store(s)
}

// loadSnap returns the last UI refresh (zero value before the first one).
func (ic *Icon) loadSnap() PaintSnap {
	if ic == nil {
		return PaintSnap{}
	}
	if s, ok := ic.paintSnap.Load().(PaintSnap); ok {
		return s
	}
	return PaintSnap{}
}

// PaintIcon paints one glyph from a frozen snapshot (raster thread only).
// Same selection order as paint: custom painter, family painter, resolved
// def glyph, placeholder — pixel-identical for the same inputs.
func PaintIcon(pc *rendering.PaintContext, size float64, s PaintSnap) {
	if pc == nil || size <= 0 {
		return
	}
	cx, cy := size/2, size/2
	pc.Save()
	pc.RotateAbout(s.Angle*math.Pi/180, cx, cy)
	if s.Custom != nil {
		s.Custom(pc, size, s.Primary, s.Secondary)
		pc.RestoreCanvas()
		return
	}
	if s.FamilyPainter != nil {
		s.FamilyPainter(pc, size, s.Primary, s.Secondary)
		pc.RestoreCanvas()
		return
	}
	if s.FamDefOK {
		drawGlyph(pc, s.Name, size, s.Base, s.Primary, s.Secondary, s.FamDef.TwoTone)
		pc.RestoreCanvas()
		return
	}
	if d, ok := Global.Lookup(s.Name); ok {
		drawGlyph(pc, s.Name, size, s.Base, s.Primary, s.Secondary, d.TwoTone)
		pc.RestoreCanvas()
		return
	}
	drawPlaceholder(pc, size, s.Base)
	pc.RestoreCanvas()
}
