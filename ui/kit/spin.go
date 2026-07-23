package kit

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Spin overlays a progress ring on content (or alone).
// Root is a RepaintBoundary; paint dirty stops here. Ticker auto-binds on mount.
type Spin struct {
	Root     *spinHost
	stack    *primitive.Stack
	ring     *primitive.Canvas
	content  core.Node
	Spinning bool
	Tip      string
	angle    float64
	Size     float64
	Theme    *core.Theme
	life     tickerLifecycle
}

type spinHost struct {
	primitive.RepaintBoundary
	spin *Spin
}

func (h *spinHost) TypeID() string { return "kit.Spin" }

func (h *spinHost) OnMount() {
	if h == nil || h.spin == nil {
		return
	}
	if t := h.Tree(); t != nil {
		h.spin.life.attach(t, h.spin, h.spin.Spinning)
	}
}

func (h *spinHost) OnUnmount() {
	if h != nil && h.spin != nil {
		h.spin.life.unmount()
	}
}

// NewSpin creates a spinner; content may be nil.
func NewSpin(content core.Node) *Spin {
	s := &Spin{content: content, Spinning: true}
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

// Tick advances spin angle. Drops when unmounted (bound tree + no mount).
func (s *Spin) Tick(dt float64) (still bool) {
	if !s.Spinning {
		return false
	}
	var nt *core.Tree
	if s.Root != nil {
		nt = s.Root.Tree()
	}
	if !s.life.stillMounted(nt) {
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
	return true
}

// AttachTicker registers this spin (also automatic OnMount).
func (s *Spin) AttachTicker(t *core.Tree) {
	if s != nil {
		s.life.attach(t, s, s.Spinning)
	}
}

// SetTip sets the loading tip text under the ring.
func (s *Spin) SetTip(tip string) {
	s.Tip = tip
	s.rebuild()
}

// SetSpinning enables/disables the spinner ticker.
func (s *Spin) SetSpinning(v bool) {
	s.Spinning = v
	s.life.setActive(v)
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

func (s *Spin) theme() *core.Theme {
	var n core.Node
	if s.Root != nil {
		n = s.Root
	}
	return themeOf(s.Theme, n)
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
	s.ring = primitive.NewCanvas(size, size, func(pc *core.PaintContext, sz core.Size) {
		if pc == nil || pc.DC == nil || !s.Spinning {
			return
		}
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
		start := -math.Pi/2 + s.angle*2*math.Pi
		end := start + 2*math.Pi*0.75
		dc.SetRGBA(fill.R, fill.G, fill.B, fill.A)
		for i := 0; i <= 64; i++ {
			a := start + (end-start)*float64(i)/64
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
	var spinUI core.Node = s.ring
	if s.Tip != "" && s.ring != nil {
		lab := primitive.NewText(s.Tip)
		lab.FontSize = 12
		lab.Color = th.Color(core.TokenColorTextSecondary)
		col := primitive.Column(s.ring, lab)
		col.Gap = 8
		col.CrossAlign = core.CrossCenter
		spinUI = col
	}
	if spinUI != nil {
		kids = append(kids, primitive.Positioned(core.AlignCenter, spinUI))
	}
	s.stack = primitive.NewStack(kids...)
	s.stack.Base().Role = "status"
	if s.Tip != "" {
		s.stack.Base().Label = s.Tip
	} else {
		s.stack.Base().Label = "Loading"
	}
	s.stack.Base().Live = "polite"

	h := &spinHost{spin: s}
	h.Init(h)
	h.Hit = core.HitDefer
	h.SetRepaintBoundary(true)
	h.AddChild(s.stack)
	s.Root = h
}
