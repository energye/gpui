package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Skeleton is a placeholder shimmer block (M5).
// Tick only mutates paint chrome (no tree rebuild). Prefer wrapping Node() in
// primitive.RepaintBoundary so paint dirty stays local.
type Skeleton struct {
	Root   *primitive.Decorated
	host   *primitive.Flex
	Width  float64
	Height float64
	// Rows number of placeholder bars (0/1 → single block).
	Rows int
	// Avatar shows a circle placeholder beside rows.
	Avatar bool
	// Active enables shimmer phase (advance via Tick / Ticker).
	Active bool
	phase  float64
	Theme  *core.Theme
	// boundTree is set by AttachTicker for demand-frame registration.
	boundTree *core.Tree
}

// NewSkeleton creates a skeleton bar.
func NewSkeleton(w, h float64) *Skeleton {
	s := &Skeleton{Width: w, Height: h, Active: true}
	s.rebuild()
	return s
}

// Node returns the root.
func (s *Skeleton) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// Tick advances shimmer. Implements core.Ticker when Active.
func (s *Skeleton) Tick(dt float64) (still bool) {
	if !s.Active {
		return false
	}
	s.phase += dt * 1.5
	if s.phase > 1 {
		s.phase -= 1
	}
	s.applyChrome()
	return s.Active
}

// AttachTicker registers this skeleton on the tree for ANIMATING demand frames.
func (s *Skeleton) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.Active)
}

// SetRows sets placeholder bar count (rebuilds multi-row layout).
func (s *Skeleton) SetRows(n int) {
	s.Rows = n
	s.rebuild()
}

// SetActive enables/disables shimmer and ticker membership.
func (s *Skeleton) SetActive(v bool) {
	s.Active = v
	if s.boundTree == nil {
		return
	}
	if v {
		s.boundTree.AddTicker(s)
	} else {
		s.boundTree.RemoveTicker(s)
	}
}

func (s *Skeleton) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Skeleton) rebuild() {
	w := s.Width
	if w <= 0 {
		w = 120
	}
	h := s.Height
	if h <= 0 {
		h = 16
	}
	rows := s.Rows
	if rows < 1 {
		rows = 1
	}
	if rows == 1 && !s.Avatar {
		s.Root = primitive.NewDecorated()
		s.Root.Width = w
		s.Root.Height = h
		s.Root.Radius = 4
		s.Root.Base().Role = "presentation"
		s.Root.SetRepaintBoundary(true)
		s.host = nil
		s.applyChrome()
		return
	}
	col := primitive.Column()
	col.Gap = 8
	for i := 0; i < rows; i++ {
		bar := primitive.NewDecorated()
		bar.Width = w
		bar.Height = h
		bar.Radius = 4
		col.AddChild(bar)
	}
	if s.Avatar {
		av := primitive.NewDecorated()
		av.Width, av.Height = 40, 40
		av.Radius = 20
		row := primitive.Row(av, col)
		row.Gap = 12
		row.CrossAlign = core.CrossStart
		s.host = row
	} else {
		s.host = col
	}
	s.Root = primitive.NewDecorated(s.host)
	s.Root.Base().Role = "presentation"
	s.Root.SetRepaintBoundary(true)
	s.applyChrome()
}

func (s *Skeleton) applyChrome() {
	if s.Root == nil {
		return
	}
	// pulse between fill secondary intensities
	base := s.theme().Color(core.TokenColorFillSecondary)
	a := 0.06 + 0.08*float64(0.5+0.5*( /*sin-ish*/ 1-2*abs01(s.phase-0.5)*2))
	if a > 0.2 {
		a = 0.2
	}
	col := render.RGBA{R: base.R, G: base.G, B: base.B, A: a}
	if col.A < 0.04 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.06 + 0.06*s.phase}
	}
	s.Root.Background = col
	if s.host != nil {
		applySkeletonChrome(s.host, col)
	}
	s.Root.MarkNeedsPaint()
}

func applySkeletonChrome(n core.Node, col render.RGBA) {
	if n == nil {
		return
	}
	if d, ok := n.(*primitive.Decorated); ok {
		d.Background = col
		d.MarkNeedsPaint()
	}
	for _, c := range n.Base().Children() {
		applySkeletonChrome(c, col)
	}
}
