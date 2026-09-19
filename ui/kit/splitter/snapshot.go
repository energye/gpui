package splitter

import (
	"github.com/energye/gpui/render"
)

// SplitterSnap is the frozen paint input for one splitter frame (R2-6,
// button snapshot paradigm). The UI thread refreshes it in the rebuild /
// markLayout / markPaint funnels and during Layout (this last point also
// picks up direct SplitterPanel config pokes that never touch the splitter);
// the raster paint path (paintBar / paintCollapseMarks) reads only this
// value plus the atomic event state (hovered / dragging / barActive /
// focusedBar).
type SplitterSnap struct {
	Vertical      bool
	BarSize       float64
	DraggableSize float64

	BaseColor   render.RGBA
	HoverColor  render.RGBA
	ActiveColor render.RGBA
	HandleColor render.RGBA
	FocusColor  render.RGBA

	HasDragger        bool
	HasCollapseStart  bool
	HasCollapseEnd    bool

	// Per-bar frozen config (index i separates panels i and i+1).
	Resizable       []bool
	CollapsibleLeft  []bool
	CollapsibleRight []bool
}

// refreshSnapshot freezes the current paint inputs (UI thread only).
func (s *Splitter) refreshSnapshot() {
	if s == nil {
		return
	}
	v := SplitterSnap{
		Vertical:          s.IsVertical(),
		BarSize:           s.SplitBarSize(),
		DraggableSize:     s.SplitBarDraggableSize(),
		BaseColor:         s.EffectiveBarColor(),
		HoverColor:        s.EffectiveBarHoverColor(),
		ActiveColor:       s.EffectiveBarActiveColor(),
		HandleColor:       s.EffectiveHandleColor(),
		FocusColor:        s.EffectiveFocusColor(),
		HasDragger:        s.draggerIcon != nil,
		HasCollapseStart:  s.collapseStartIcon != nil,
		HasCollapseEnd:    s.collapseEndIcon != nil,
	}
	if len(s.panels) >= 2 {
		n := len(s.panels) - 1
		v.Resizable = make([]bool, n)
		v.CollapsibleLeft = make([]bool, n)
		v.CollapsibleRight = make([]bool, n)
		for i := 0; i < n; i++ {
			v.Resizable[i] = s.barResizable(i)
			v.CollapsibleLeft[i] = snapShow(s.panels[i])
			v.CollapsibleRight[i] = snapShow(s.panels[i+1])
		}
	}
	s.snap.Store(v)
}

// snapShow mirrors paintCollapseMarks' live show() predicate.
func snapShow(p *SplitterPanel) bool {
	if p == nil || !p.collapsible {
		return false
	}
	return p.ShowCollapsibleIcon() != CollapsibleIconNever
}

// loadSnapshot returns the last frozen snapshot; empty Resizable paints
// nothing (no bars).
func (s *Splitter) loadSnapshot() SplitterSnap {
	if s == nil {
		return SplitterSnap{}
	}
	if v, ok := s.snap.Load().(SplitterSnap); ok {
		return v
	}
	return SplitterSnap{}
}