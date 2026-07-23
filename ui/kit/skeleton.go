package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Skeleton is a placeholder shimmer block (M5).
// Root is a RepaintBoundary; ticker auto-binds on mount when Active.
type Skeleton struct {
	Root      *skeletonHost
	decorated *primitive.Decorated
	host      *primitive.Flex
	Width     float64
	Height    float64
	Rows      int
	Avatar    bool
	Active    bool
	phase     float64
	Theme     *core.Theme
	life      tickerLifecycle
}

type skeletonHost struct {
	primitive.RepaintBoundary
	sk *Skeleton
}

func (h *skeletonHost) TypeID() string { return "kit.Skeleton" }

func (h *skeletonHost) OnMount() {
	if h == nil || h.sk == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.sk.life.attach(t, h.sk, h.sk.Active)
	}
}

func (h *skeletonHost) OnUnmount() {
	if h != nil && h.sk != nil {
		h.sk.life.unmount()
	}
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

// Tick advances shimmer. Drops when unmounted (bound tree + no mount).
func (s *Skeleton) Tick(dt float64) (still bool) {
	if !s.Active {
		return false
	}
	var nt *core.Tree
	if s.Root != nil {
		nt = s.Root.Tree()
	}
	if !s.life.stillMounted(nt) {
		return false
	}
	s.phase += dt * 1.5
	if s.phase > 1 {
		s.phase -= 1
	}
	s.applyChrome()
	return true
}

// AttachTicker registers this skeleton (also automatic OnMount).
func (s *Skeleton) AttachTicker(t *core.Tree) {
	if s != nil {
		s.life.attach(t, s, s.Active)
	}
}

// SetRows sets placeholder bar count (rebuilds multi-row layout).
func (s *Skeleton) SetRows(n int) {
	s.Rows = n
	s.rebuild()
}

// SetActive enables/disables shimmer and ticker membership.
func (s *Skeleton) SetActive(v bool) {
	s.Active = v
	s.life.setActive(v)
}

func (s *Skeleton) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Skeleton) rebuild() {
	w, h := s.Width, s.Height
	if w <= 0 {
		w = 120
	}
	if h <= 0 {
		h = 16
	}
	rows := s.Rows
	if rows < 1 {
		rows = 1
	}
	if rows == 1 && !s.Avatar {
		s.decorated = primitive.NewDecorated()
		s.decorated.Width, s.decorated.Height, s.decorated.Radius = w, h, 4
		s.decorated.Base().Role = "presentation"
		s.host = nil
		s.wrapRoot()
		s.applyChrome()
		return
	}
	col := primitive.Column()
	col.Gap = 8
	for i := 0; i < rows; i++ {
		bar := primitive.NewDecorated()
		bar.Width, bar.Height, bar.Radius = w, h, 4
		col.AddChild(bar)
	}
	if s.Avatar {
		av := primitive.NewDecorated()
		av.Width, av.Height, av.Radius = 40, 40, 20
		row := primitive.Row(av, col)
		row.Gap = 12
		row.CrossAlign = core.CrossStart
		s.host = row
	} else {
		s.host = col
	}
	s.decorated = primitive.NewDecorated(s.host)
	s.decorated.Base().Role = "presentation"
	s.wrapRoot()
	s.applyChrome()
}

func (s *Skeleton) wrapRoot() {
	h := &skeletonHost{sk: s}
	h.Init(h)
	h.Hit = core.HitDefer
	h.SetRepaintBoundary(true)
	if s.decorated != nil {
		h.AddChild(s.decorated)
	}
	s.Root = h
}

func (s *Skeleton) applyChrome() {
	if s.decorated == nil {
		return
	}
	base := s.theme().Color(core.TokenColorFillSecondary)
	a := 0.06 + 0.08*float64(0.5+0.5*(1-2*abs01(s.phase-0.5)*2))
	if a > 0.2 {
		a = 0.2
	}
	col := render.RGBA{R: base.R, G: base.G, B: base.B, A: a}
	if col.A < 0.04 {
		col = render.RGBA{R: 0, G: 0, B: 0, A: 0.06 + 0.06*s.phase}
	}
	s.decorated.Background = col
	if s.host != nil {
		applySkeletonChrome(s.host, col)
	}
	s.decorated.MarkNeedsPaint()
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
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
