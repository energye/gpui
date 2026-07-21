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

// Icon paints a named icon glyph.
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
	drawIcon(pc, def.Kind, pc.Origin.X, pc.Origin.Y, s, col)
}

// HitTest implements core.Node.
func (ic *Icon) HitTest(p core.Point) core.Node { return ic.DefaultHitTest(p) }

func drawIcon(pc *core.PaintContext, kind IconKind, x, y, s float64, col render.RGBA) {
	dc := pc.DC
	dc.SetRGBA(col.R, col.G, col.B, col.A)
	dc.SetLineWidth(1.5)
	pad := s * 0.2
	x0, y0 := x+pad, y+pad
	x1, y1 := x+s-pad, y+s-pad
	cx, cy := x+s/2, y+s/2

	switch kind {
	case IconCheck:
		dc.MoveTo(x0, cy)
		dc.LineTo(cx-s*0.05, y1)
		dc.LineTo(x1, y0)
		_ = dc.Stroke()
	case IconClose:
		dc.MoveTo(x0, y0)
		dc.LineTo(x1, y1)
		dc.MoveTo(x1, y0)
		dc.LineTo(x0, y1)
		_ = dc.Stroke()
	case IconPlus:
		dc.MoveTo(cx, y0)
		dc.LineTo(cx, y1)
		dc.MoveTo(x0, cy)
		dc.LineTo(x1, cy)
		_ = dc.Stroke()
	case IconMinus:
		dc.MoveTo(x0, cy)
		dc.LineTo(x1, cy)
		_ = dc.Stroke()
	case IconChevronRight:
		dc.MoveTo(x0+s*0.1, y0)
		dc.LineTo(x1-s*0.05, cy)
		dc.LineTo(x0+s*0.1, y1)
		_ = dc.Stroke()
	case IconChevronDown:
		dc.MoveTo(x0, y0+s*0.1)
		dc.LineTo(cx, y1-s*0.05)
		dc.LineTo(x1, y0+s*0.1)
		_ = dc.Stroke()
	case IconSearch:
		r := s * 0.28
		dc.DrawCircle(cx-s*0.08, cy-s*0.08, r)
		_ = dc.Stroke()
		dc.MoveTo(cx+r*0.4, cy+r*0.4)
		dc.LineTo(x1, y1)
		_ = dc.Stroke()
	case IconInfo:
		dc.DrawCircle(cx, cy, s*0.35)
		_ = dc.Stroke()
		dc.SetLineWidth(1.8)
		dc.MoveTo(cx, cy-s*0.08)
		dc.LineTo(cx, cy+s*0.18)
		_ = dc.Stroke()
		dc.DrawCircle(cx, cy-s*0.22, s*0.04)
		_ = dc.Fill()
	default: // placeholder diamond
		dc.MoveTo(cx, y0)
		dc.LineTo(x1, cy)
		dc.LineTo(cx, y1)
		dc.LineTo(x0, cy)
		dc.ClosePath()
		_ = dc.Stroke()
	}
}
