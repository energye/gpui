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

// Tour is a multi-step spotlight tour (M5 lite).
// Portal/Scope are stable across step changes (rebuild only refreshes layer content).
type Tour struct {
	Portal   *primitive.OverlayPortal
	Scope    *primitive.FocusScope
	Steps    []TourStep
	Index    int
	Open     bool
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	OnClose  func()
	OnChange func(index int)

	layer *tourLayer
	trap  overlayFocusTrap
}

// NewTour creates a closed tour.
func NewTour(steps ...TourStep) *Tour {
	t := &Tour{Steps: steps}
	t.ensureShell()
	return t
}

type tourLayer struct {
	core.NodeBase
	tour  *Tour
	theme *core.Theme
}

// Node returns the portal node.
func (t *Tour) Node() core.Node {
	if t == nil {
		return nil
	}
	t.ensureShell()
	return t.Portal
}

// SetOpen shows/hides the tour. Esc / mask click ends the tour (focus trap while open).
func (t *Tour) SetOpen(open bool) {
	if t == nil {
		return
	}
	t.ensureShell()
	was := t.Open
	t.Open = open
	if t.Portal != nil {
		t.Portal.SetOpen(open)
	}
	if open {
		t.layer.MarkNeedsLayout()
		t.trap.wire(t.Scope, true, t.onEscape)
		t.trap.enter(t.Scope, t.Portal, nil)
	} else {
		t.trap.wire(t.Scope, false, nil)
		if was {
			t.trap.leave(t.Scope, t.Portal)
			if t.OnClose != nil {
				t.OnClose()
			}
		}
	}
}

func (t *Tour) onEscape() {
	if t == nil || !t.Open {
		return
	}
	t.SetOpen(false)
}

// SetCurrent jumps to step index without replacing the portal (keeps overlay mounted).
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
	t.invalidateStep()
}

// Next advances step.
func (t *Tour) Next() {
	if t.Index+1 < len(t.Steps) {
		t.Index++
		if t.OnChange != nil {
			t.OnChange(t.Index)
		}
		t.invalidateStep()
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
		t.invalidateStep()
	}
}

// Sync repositions for viewport.
func (t *Tour) Sync() {
	if t.Open && t.Portal != nil {
		t.layer.MarkNeedsLayout()
		t.Portal.SetOpen(true)
	}
}

func (t *Tour) theme() *core.Theme {
	var n core.Node
	if t.Portal != nil {
		n = t.Portal
	}
	return themeOf(t.Theme, n)
}

// ensureShell builds a stable Portal/Scope/layer once.
func (t *Tour) ensureShell() {
	if t.Portal != nil && t.layer != nil && t.Scope != nil {
		return
	}
	th := t.theme()
	t.layer = &tourLayer{tour: t, theme: th}
	t.layer.Init(t.layer)
	t.layer.Hit = core.HitDefer
	t.layer.Role = "dialog"
	t.layer.Label = "Tour"
	t.Scope = primitive.NewFocusScope(t.layer)
	t.trap.wire(t.Scope, t.Open, t.onEscape)
	t.Portal = primitive.NewOverlayPortal(t.Scope)
	t.Portal.ID = "" // unique auto-id
	t.Portal.ZOrder = OverlayZTour
	if t.Open {
		t.Portal.SetOpen(true)
	}
}

func (t *Tour) invalidateStep() {
	t.ensureShell()
	if t.layer != nil {
		t.layer.MarkNeedsLayout()
		t.layer.MarkNeedsPaint()
	}
	if t.Open && t.Portal != nil {
		t.Portal.SetOpen(true)
		if tr := t.Portal.Tree(); tr != nil {
			tr.MarkDirty()
		}
	}
}

func (l *tourLayer) TypeID() string { return "kit.TourLayer" }

func (l *tourLayer) Layout(c core.Constraints) core.Size {
	var portal *primitive.OverlayPortal
	var vp core.Size
	if l.tour != nil {
		portal = l.tour.Portal
		vp = l.tour.Viewport
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
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
