package kit

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

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
