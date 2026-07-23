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
	info    *primitive.Text
	Percent float64 // 0..100
	Width   float64
	// Status: "" | "success" | "exception" | "active" | "normal"
	Status   string
	ShowInfo bool // show percent label
	Theme    *core.Theme
}

// NewProgress creates a progress bar.
func NewProgress(percent float64) *Progress {
	p := &Progress{Percent: percent, Width: 200, ShowInfo: true}
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

// SetStatus sets status chrome (success|exception|active|normal).
func (p *Progress) SetStatus(status string) {
	p.Status = status
	p.applyFill()
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
	th := p.theme()
	if p.bar != nil {
		p.bar.Width = w * (p.Percent / 100)
		switch p.Status {
		case "success":
			p.bar.Color = th.Color(core.TokenColorSuccess)
		case "exception":
			p.bar.Color = th.Color(core.TokenColorError)
		default:
			p.bar.Color = th.Color(core.TokenColorPrimary)
		}
		p.bar.MarkNeedsLayout()
		p.bar.MarkNeedsPaint()
	}
	if p.info != nil {
		if p.ShowInfo {
			p.info.Value = fmt.Sprintf("%.0f%%", p.Percent)
		} else {
			p.info.Value = ""
		}
		p.info.MarkNeedsPaint()
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
	switch p.Status {
	case "success":
		p.bar.Color = th.Color(core.TokenColorSuccess)
	case "exception":
		p.bar.Color = th.Color(core.TokenColorError)
	default:
		p.bar.Color = th.Color(core.TokenColorPrimary)
	}

	p.track = primitive.NewBox(p.bar)
	p.track.Width = w
	p.track.Height = barH
	p.track.Color = th.Color(core.TokenColorFillSecondary)
	if p.track.Color.A < 0.05 {
		p.track.Color = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}

	trackHost := primitive.NewDecorated(p.track)
	trackHost.Radius = barH / 2
	p.info = primitive.NewText("")
	p.info.FontSize = 12
	p.info.Color = th.Color(core.TokenColorTextSecondary)
	if p.ShowInfo {
		p.info.Value = fmt.Sprintf("%.0f%%", p.Percent)
	}
	row := primitive.Row(trackHost, p.info)
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	p.Root = primitive.NewDecorated(row)
	p.Root.Base().Role = "progressbar"
	p.Root.Base().Label = fmt.Sprintf("%.0f percent", p.Percent)
	// Not a RepaintBoundary: nested under ScrollViewport, SkipRepaintBoundaries
	// would leave a hole and Progress can disappear until a full scroll re-raster.
	// Progress updates are infrequent (SetPercent) — baking into parent is fine.
}
