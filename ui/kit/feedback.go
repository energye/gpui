package kit

import (
	"fmt"
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Skeleton is a placeholder shimmer block (M5).
// Tick only mutates paint chrome (no tree rebuild). Prefer wrapping Node() in
// primitive.RepaintBoundary so paint dirty stays local.
type Skeleton struct {
	Root   *primitive.Decorated
	Width  float64
	Height float64
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
	s.Root = primitive.NewDecorated()
	s.Root.Width = s.Width
	s.Root.Height = s.Height
	if s.Root.Width <= 0 {
		s.Root.Width = 120
	}
	if s.Root.Height <= 0 {
		s.Root.Height = 16
	}
	s.Root.Radius = 4
	s.Root.Base().Role = "presentation"
	// Phase B: isolate paint dirty under CompositeOnly present.
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
	s.Root.Background = render.RGBA{R: base.R, G: base.G, B: base.B, A: a}
	if s.Root.Background.A < 0.04 {
		s.Root.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.06 + 0.06*s.phase}
	}
	s.Root.MarkNeedsPaint()
}

func abs01(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

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

// TourStep is one guided step.
type TourStep struct {
	Title string
	Body  string
	// Target absolute rect to highlight (set by host each frame).
	Target core.Rect
}

// Tour is a minimal multi-step spotlight tour (M5 lite).
type Tour struct {
	Portal   *primitive.OverlayPortal
	Steps    []TourStep
	Index    int
	Open     bool
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	OnClose  func()
	OnChange func(index int)

	layer *tourLayer
}

// NewTour creates a closed tour.
func NewTour(steps ...TourStep) *Tour {
	t := &Tour{Steps: steps}
	t.rebuild()
	return t
}

// Node returns the portal node.
func (t *Tour) Node() core.Node {
	if t.Portal == nil {
		t.rebuild()
	}
	return t.Portal
}

// SetOpen shows/hides the tour.
func (t *Tour) SetOpen(open bool) {
	t.Open = open
	if t.Portal != nil {
		t.Portal.SetOpen(open)
	}
	if !open && t.OnClose != nil {
		t.OnClose()
	}
}

// Next advances step.
func (t *Tour) Next() {
	if t.Index+1 < len(t.Steps) {
		t.Index++
		if t.OnChange != nil {
			t.OnChange(t.Index)
		}
		t.rebuild()
		t.Portal.SetOpen(true)
	} else {
		t.SetOpen(false)
	}
}

// Prev goes back.
func (t *Tour) Prev() {
	if t.Index > 0 {
		t.Index--
		if t.OnChange != nil {
			t.OnChange(t.Index)
		}
		t.rebuild()
		t.Portal.SetOpen(true)
	}
}

// Sync repositions for viewport.
func (t *Tour) Sync() {
	if t.Open && t.Portal != nil {
		t.Portal.SetOpen(true)
	}
}

func (t *Tour) theme() *core.Theme {
	if t.Theme != nil {
		return t.Theme
	}
	return DefaultTheme()
}

func (t *Tour) rebuild() {
	th := t.theme()
	t.layer = &tourLayer{tour: t, theme: th}
	t.layer.Init(t.layer)
	t.layer.Hit = core.HitDefer
	t.layer.Role = "dialog"
	t.layer.Label = "Tour"
	t.Portal = primitive.NewOverlayPortal(t.layer)
	t.Portal.ID = "tour"
	t.Portal.ZOrder = 700
}

type tourLayer struct {
	core.NodeBase
	tour  *Tour
	theme *core.Theme
}

func (l *tourLayer) TypeID() string { return "kit.TourLayer" }

func (l *tourLayer) Layout(c core.Constraints) core.Size {
	vw, vh := c.MaxWidth, c.MaxHeight
	if l.tour.Viewport.Width > 0 {
		vw, vh = l.tour.Viewport.Width, l.tour.Viewport.Height
	}
	if vw >= core.Unbounded/2 {
		vw = 800
	}
	if vh >= core.Unbounded/2 {
		vh = 600
	}
	l.ClearChildren()

	// dim mask
	mask := primitive.NewMask()
	mask.Width, mask.Height = vw, vh
	mask.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.55}
	mask.OnDismiss = func() { l.tour.SetOpen(false) }
	_ = mask.Layout(core.Tight(vw, vh))
	mask.SetOffset(core.Point{})
	l.AddChild(mask)

	// highlight hole (drawn as clear rect stroke)
	var target core.Rect
	if l.tour.Index >= 0 && l.tour.Index < len(l.tour.Steps) {
		target = l.tour.Steps[l.tour.Index].Target
	}
	if !target.Empty() {
		// border around target
		hole := primitive.NewDecorated()
		hole.Width = target.Width()
		hole.Height = target.Height()
		hole.BorderWidth = 2
		hole.BorderColor = l.theme.Color(core.TokenColorPrimary)
		hole.Background = render.RGBA{}
		_ = hole.Layout(core.Tight(hole.Width, hole.Height))
		hole.SetOffset(core.Point{X: target.Min.X, Y: target.Min.Y})
		l.AddChild(hole)
	}

	// panel
	step := TourStep{Title: "Done", Body: ""}
	if l.tour.Index >= 0 && l.tour.Index < len(l.tour.Steps) {
		step = l.tour.Steps[l.tour.Index]
	}
	title := primitive.NewText(step.Title)
	title.FontSize = 16
	title.Face = l.tour.Face
	title.Color = l.theme.Color(core.TokenColorText)
	body := primitive.NewText(step.Body)
	body.FontSize = 13
	body.Face = l.tour.Face
	body.Color = l.theme.Color(core.TokenColorTextSecondary)
	info := primitive.NewText(fmt.Sprintf("%d / %d", l.tour.Index+1, len(l.tour.Steps)))
	info.FontSize = 12
	info.Face = l.tour.Face
	info.Color = l.theme.Color(core.TokenColorTextSecondary)

	next := NewButton("Next")
	next.SetType(ButtonPrimary)
	next.SetFace(l.tour.Face)
	next.SetOnClick(func() { l.tour.Next() })
	prev := NewButton("Back")
	prev.SetFace(l.tour.Face)
	prev.SetOnClick(func() { l.tour.Prev() })
	if l.tour.Index == 0 {
		prev.SetDisabled(true)
	}
	skip := NewButton("Skip")
	skip.SetType(ButtonText)
	skip.SetFace(l.tour.Face)
	skip.SetOnClick(func() { l.tour.SetOpen(false) })

	footer := primitive.Row(skip.Node(), primitive.Spacer(), prev.Node(), next.Node())
	footer.Gap = 8
	col := primitive.Column(title, body, info, footer)
	col.Gap = 10
	col.CrossAlign = core.CrossStart
	panel := primitive.NewDecorated(col)
	panel.Padding = primitive.All(16)
	panel.Radius = 8
	panel.Background = l.theme.Color(core.TokenColorBgContainer)
	panel.MinWidth = 280
	_ = panel.Layout(core.Loose(320, vh))
	// place below target or center
	px := (vw - panel.Size().Width) / 2
	py := vh * 0.6
	if !target.Empty() {
		px = target.Min.X
		py = target.Max.Y + 12
		if py+panel.Size().Height > vh {
			py = target.Min.Y - panel.Size().Height - 12
		}
		if px+panel.Size().Width > vw {
			px = vw - panel.Size().Width - 8
		}
		if px < 8 {
			px = 8
		}
	}
	panel.SetOffset(core.Point{X: px, Y: py})
	l.AddChild(panel)

	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *tourLayer) Paint(pc *core.PaintContext)    { l.DefaultPaintChildren(pc) }
func (l *tourLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

// Progress is a linear progress bar.
type Progress struct {
	Root    *primitive.Decorated
	track   *primitive.Box
	bar     *primitive.Box
	Percent float64 // 0..100
	Width   float64
	Theme   *core.Theme
}

// NewProgress creates a progress bar.
func NewProgress(percent float64) *Progress {
	p := &Progress{Percent: percent, Width: 200}
	p.rebuild()
	return p
}

// Node returns the root.
func (p *Progress) Node() core.Node {
	if p.Root == nil {
		p.rebuild()
	}
	return p.Root
}

// SetPercent updates fill 0..100 without rebuilding the node tree.
func (p *Progress) SetPercent(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	if p.Percent == v && p.bar != nil {
		return
	}
	p.Percent = v
	p.applyFill()
}

func (p *Progress) theme() *core.Theme {
	if p.Theme != nil {
		return p.Theme
	}
	return DefaultTheme()
}

func (p *Progress) applyFill() {
	if p.Root == nil {
		p.rebuild()
		return
	}
	w := p.Width
	if w <= 0 {
		w = 200
	}
	if p.bar != nil {
		p.bar.Width = w * (p.Percent / 100)
		p.bar.MarkNeedsLayout()
		p.bar.MarkNeedsPaint()
	}
	if p.track != nil {
		p.track.MarkNeedsLayout()
	}
	p.Root.Base().Label = fmt.Sprintf("%.0f percent", p.Percent)
	p.Root.MarkNeedsPaint()
}

func (p *Progress) rebuild() {
	th := p.theme()
	w := p.Width
	if w <= 0 {
		w = 200
	}
	barH := th.SizeOr(core.TokenProgressHeight, 8)
	p.bar = primitive.NewBox()
	p.bar.Height = barH
	p.bar.Width = w * (p.Percent / 100)
	p.bar.Color = th.Color(core.TokenColorPrimary)

	p.track = primitive.NewBox(p.bar)
	p.track.Width = w
	p.track.Height = barH
	p.track.Color = th.Color(core.TokenColorFillSecondary)
	if p.track.Color.A < 0.05 {
		p.track.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}

	p.Root = primitive.NewDecorated(p.track)
	// Ant Progress line: fully rounded ends (pill).
	p.Root.Radius = barH / 2
	p.Root.Base().Role = "progressbar"
	p.Root.Base().Label = fmt.Sprintf("%.0f percent", p.Percent)
	if p.Root != nil {
		p.Root.SetRepaintBoundary(true)
	}
}

// Density constants for Theme.Density.
const (
	DensityDefault = "default"
	DensityCompact = "compact"
	DensityLarge   = "large"
)

// ApplyDensity mutates control heights on a theme token set.
func ApplyDensity(th *core.Theme, density string) {
	if th == nil || th.Tokens == nil {
		return
	}
	th.Density = density
	switch density {
	case DensityCompact:
		th.Tokens.Sizes[core.TokenControlHeight] = 24
		th.Tokens.Sizes[core.TokenControlHeightSM] = 20
		th.Tokens.Sizes[core.TokenControlHeightLG] = 32
		th.Tokens.Sizes[core.TokenFontSize] = 12
	case DensityLarge:
		th.Tokens.Sizes[core.TokenControlHeight] = 40
		th.Tokens.Sizes[core.TokenControlHeightSM] = 32
		th.Tokens.Sizes[core.TokenControlHeightLG] = 48
		th.Tokens.Sizes[core.TokenFontSize] = 16
	default:
		th.Tokens.Sizes[core.TokenControlHeight] = 32
		th.Tokens.Sizes[core.TokenControlHeightSM] = 24
		th.Tokens.Sizes[core.TokenControlHeightLG] = 40
		th.Tokens.Sizes[core.TokenFontSize] = 14
	}
}

// CollectA11y walks the tree and returns nodes with Role or Label set.
type A11yNode struct {
	Role   string
	Label  string
	Live   string
	Type   string
	Bounds core.Rect
}

// CollectA11y returns a flat list of accessible nodes (for tests/tools).
func CollectA11y(root core.Node) []A11yNode {
	var out []A11yNode
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		b := n.Base()
		if b.Role != "" || b.Label != "" {
			out = append(out, A11yNode{
				Role: b.Role, Label: b.Label, Live: b.Live,
				Type: n.TypeID(), Bounds: core.AbsoluteBounds(n),
			})
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	return out
}
