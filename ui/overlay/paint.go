package overlay

import (
	"github.com/energye/gpui/ui/rendering"
)

// Paint draws overlay entries (bottom → top) in window coordinates.
// Does not paint the main band. Clears NeedsPaint on entries after paint when
// the child does not report NeedsPaint.
func (s *State) Paint(pc *rendering.PaintContext) {
	if s == nil || pc == nil {
		return
	}
	for _, e := range s.entries {
		if e == nil || e.Child == nil {
			continue
		}
		childPC := pc.WithOrigin(pc.OriginX+e.X, pc.OriginY+e.Y)
		// Overlay content always paints when entry is dirty or child needs paint.
		if e.NeedsPaint {
			childPC.CompositeOnly = false
		}
		e.Child.Paint(childPC)
		if !e.Child.NeedsPaint() {
			e.NeedsPaint = false
		}
	}
}

// LayoutAndPaint is a convenience for tests: layout then paint.
func (s *State) LayoutAndPaint(pc *rendering.PaintContext, viewportW, viewportH float64) {
	s.Layout(viewportW, viewportH)
	s.Paint(pc)
}

// Silence unused import if paint is only used with Context.
var _ = rendering.Point{}
