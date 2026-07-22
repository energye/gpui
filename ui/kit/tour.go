package kit

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

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

type tourLayer struct {
	core.NodeBase
	tour  *Tour
	theme *core.Theme
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

// SetCurrent jumps to step index.
func (t *Tour) SetCurrent(i int) {
	if i < 0 {
		i = 0
	}
	if len(t.Steps) > 0 && i >= len(t.Steps) {
		i = len(t.Steps) - 1
	}
	if t.Index == i {
		return
	}
	t.Index = i
	if t.OnChange != nil {
		t.OnChange(t.Index)
	}
	t.rebuild()
	if t.Open && t.Portal != nil {
		t.Portal.SetOpen(true)
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

func (l *tourLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *tourLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
