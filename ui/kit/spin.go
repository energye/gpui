package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Spin overlays a progress ring on content (or alone).
// Angle is painted in-place — no per-tick ring node rebuild.
type Spin struct {
	Root     *primitive.Stack
	ring     *primitive.Canvas
	content  core.Node
	Spinning bool
	// angle for rotation simulation (0..1 wraps)
	angle     float64
	Size      float64
	Theme     *core.Theme
	boundTree *core.Tree
}

// NewSpin creates a spinner; content may be nil.
func NewSpin(content core.Node) *Spin {
	s := &Spin{content: content, Spinning: true, Size: 0} // Size 0 → theme TokenSpinSize
	s.rebuild()
	return s
}

// Node returns the root.
func (s *Spin) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// Tick advances spin angle. Implements core.Ticker when Spinning.
func (s *Spin) Tick(dt float64) (still bool) {
	if !s.Spinning {
		return false
	}
	s.angle += dt * 1.2
	if s.angle > 1 {
		s.angle -= 1
	}
	if s.ring != nil {
		s.ring.MarkNeedsPaint()
	} else if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
	return s.Spinning
}

// AttachTicker registers this spin on the tree.
func (s *Spin) AttachTicker(t *core.Tree) {
	if s == nil || t == nil {
		return
	}
	s.boundTree = t
	t.BindTicker(s, s.Spinning)
}

// SetSpinning enables/disables the spinner ticker.
func (s *Spin) SetSpinning(v bool) {
	s.Spinning = v
	if s.boundTree == nil {
		return
	}
	if v {
		s.boundTree.AddTicker(s)
	} else {
		s.boundTree.RemoveTicker(s)
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

func (s *Spin) theme() *core.Theme {
	if s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Spin) rebuild() {
	th := s.theme()
	size := s.Size
	if size <= 0 {
		size = th.SizeOr(core.TokenSpinSize, 20)
	}
	track := th.Color(core.TokenColorFillSecondary)
	if track.A < 0.08 {
		track = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	fill := th.Color(core.TokenColorPrimary)
	// Fixed canvas; PaintFn reads s.angle each paint (no alloc per tick).
	s.ring = primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil || pc.DC == nil || !s.Spinning {
			return
		}
		// Ant Spin: round-capped arc on a light track (true circle stroke).
		prog := 0.75
		stroke := size * 0.12
		if stroke < 2 {
			stroke = 2
		}
		if stroke > 3 {
			stroke = 3
		}
		dc := pc.DC
		cx := pc.Origin.X + sz.Width/2
		cy := pc.Origin.Y + sz.Height/2
		r := size/2 - stroke
		if r < 1 {
			r = 1
		}
		dc.SetLineCap(render.LineCapRound)
		dc.SetLineJoin(render.LineJoinRound)
		dc.SetLineWidth(stroke)
		dc.SetRGBA(track.R, track.G, track.B, track.A)
		dc.DrawCircle(cx, cy, r)
		_ = dc.Stroke()
		// Active arc from rotating start — more steps for smoother AA.
		start := -math.Pi/2 + s.angle*2*math.Pi
		end := start + 2*math.Pi*prog
		steps := 64
		dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
		for i := 0; i <= steps; i++ {
			a := start + (end-start)*float64(i)/float64(steps)
			x := cx + r*math.Cos(a)
			y := cy + r*math.Sin(a)
			if i == 0 {
				dc.MoveTo(x, y)
			} else {
				dc.LineTo(x, y)
			}
		}
		_ = dc.Stroke()
	})
	kids := []core.Node{}
	if s.content != nil {
		kids = append(kids, s.content)
	}
	if s.ring != nil {
		kids = append(kids, primitive.Positioned(core.AlignCenter, s.ring))
	}
	s.Root = primitive.NewStack(kids...)
	s.Root.Base().Role = "status"
	s.Root.Base().Label = "Loading"
	s.Root.Base().Live = "polite"
	// Phase B: keep spin dirty local under CompositeOnly present.
	if s.Root != nil {
		s.Root.SetRepaintBoundary(true)
	}
}
