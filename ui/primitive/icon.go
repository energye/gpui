package primitive

import (
	"sync"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// IconRegistry maps icon name → path data or paint callback (C-IconReg).
type IconRegistry struct {
	mu    sync.RWMutex
	paths map[string]IconDef
}

// IconDef describes a simple geometric icon for M1 (path-less shapes).
type IconDef struct {
	// Kind selects a built-in glyph shape.
	Kind IconKind
	// Path is reserved for SVG path data (later).
	Path string
}

// IconKind enumerates built-in M1 icons.
type IconKind int

const (
	IconNone IconKind = iota
	IconCheck
	IconClose
	IconPlus
	IconMinus
	IconChevronRight
	IconChevronDown
	IconSearch
	IconInfo
	IconCustom // uses Path later; draws a diamond placeholder
)

// GlobalIcons is the process-wide icon registry.
var GlobalIcons = NewIconRegistry()

// NewIconRegistry creates an empty registry with built-in defaults.
func NewIconRegistry() *IconRegistry {
	r := &IconRegistry{paths: make(map[string]IconDef)}
	r.Register("check", IconDef{Kind: IconCheck})
	r.Register("close", IconDef{Kind: IconClose})
	r.Register("plus", IconDef{Kind: IconPlus})
	r.Register("minus", IconDef{Kind: IconMinus})
	r.Register("chevron-right", IconDef{Kind: IconChevronRight})
	r.Register("chevron-down", IconDef{Kind: IconChevronDown})
	r.Register("search", IconDef{Kind: IconSearch})
	r.Register("info", IconDef{Kind: IconInfo})
	return r
}

// Register adds or replaces an icon definition.
func (r *IconRegistry) Register(name string, def IconDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths[name] = def
}

// Lookup returns an icon definition.
func (r *IconRegistry) Lookup(name string) (IconDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.paths[name]
	return d, ok
}

// Icon paints a named icon glyph via shared PaintContext stroke helpers
// (round caps, AA, consistent line widths).
type Icon struct {
	core.NodeBase

	Name  string
	Size  float64 // logical px; 0 → 16
	Color render.RGBA
	// Registry defaults to GlobalIcons.
	Registry *IconRegistry
}

// NewIcon constructs an Icon by name.
func NewIcon(name string) *Icon {
	ic := &Icon{
		Name:  name,
		Size:  16,
		Color: render.RGBA{R: 0, G: 0, B: 0, A: 0.88},
	}
	ic.Init(ic)
	ic.Hit = core.HitDefer
	return ic
}

// TypeID implements core.Node.
func (ic *Icon) TypeID() string { return TypeIcon }

// Layout implements core.Node.
func (ic *Icon) Layout(c core.Constraints) core.Size {
	s := ic.Size
	if s <= 0 {
		s = 16
	}
	out := c.Tighten(core.Size{Width: s, Height: s})
	ic.SetSize(out)
	return out
}

// Paint implements core.Node.
func (ic *Icon) Paint(pc *core.PaintContext) {
	if pc == nil || pc.DC == nil {
		return
	}
	reg := ic.Registry
	if reg == nil {
		reg = GlobalIcons
	}
	def, ok := reg.Lookup(ic.Name)
	if !ok {
		def = IconDef{Kind: IconCustom}
	}
	s := ic.Size
	if s <= 0 {
		s = 16
	}
	col := ic.Color
	if pc.Theme != nil && col.A == 0 {
		col = pc.Theme.Color(core.TokenColorText)
	}
	// Draw in local space (Origin is already icon top-left).
	drawIconLocal(pc, def.Kind, s, col)
}

// HitTest implements core.Node.
func (ic *Icon) HitTest(p core.Point) core.Node { return ic.DefaultHitTest(p) }

// drawIconLocal paints built-in icons using shared stroke/circle APIs only.
// Coordinates are local to the icon box [0,s]×[0,s].
func drawIconLocal(pc *core.PaintContext, kind IconKind, s float64, col render.RGBA) {
	if pc == nil {
		return
	}
	lw := s * 0.125
	if lw < 1.6 {
		lw = 1.6
	}
	if lw > 2.5 {
		lw = 2.5
	}
	pad := s * 0.18
	x0, y0 := pad, pad
	x1, y1 := s-pad, s-pad
	cx, cy := s/2, s/2

	switch kind {
	case IconCheck:
		pc.PaintLocalCheck(s, s, lw, col)
	case IconClose:
		pc.PaintLocalClose(s, s, pad, lw, col)
	case IconPlus:
		pc.StrokeLocalLine(cx, y0, cx, y1, lw, col)
		pc.StrokeLocalLine(x0, cy, x1, cy, lw, col)
	case IconMinus:
		pc.StrokeLocalLine(x0, cy, x1, cy, lw, col)
	case IconChevronRight:
		pc.StrokeLocalPolyline([]float64{
			x0 + s*0.1, y0,
			x1 - s*0.05, cy,
			x0 + s*0.1, y1,
		}, lw, col)
	case IconChevronDown:
		pc.StrokeLocalPolyline([]float64{
			x0, y0 + s*0.1,
			cx, y1 - s*0.05,
			x1, y0 + s*0.1,
		}, lw, col)
	case IconSearch:
		r := s * 0.28
		pc.StrokeLocalCircle(cx-s*0.08, cy-s*0.08, r, lw, col)
		pc.StrokeLocalLine(cx+r*0.4, cy+r*0.4, x1, y1, lw, col)
	case IconInfo:
		pc.StrokeLocalCircle(cx, cy, s*0.35, lw, col)
		pc.StrokeLocalLine(cx, cy-s*0.08, cx, cy+s*0.18, lw*1.15, col)
		pc.FillLocalCircle(cx, cy-s*0.22, s*0.04, col)
	default: // placeholder diamond
		pc.StrokeLocalPolyline([]float64{
			cx, y0,
			x1, cy,
			cx, y1,
			x0, cy,
			cx, y0, // close
		}, lw, col)
	}
}
