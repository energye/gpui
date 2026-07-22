package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Drawer is a side panel with mask.
type Drawer struct {
	Portal *primitive.OverlayPortal
	panel  *primitive.Decorated
	title  *primitive.Text
	body   *primitive.Slot
	Open   bool
	Title  string
	Width  float64
	// Placement: "left" or "right" (default right).
	Placement string
	// Left is true when Placement is "left" (kept for layout).
	Left     bool
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	OnClose  func()
	content  core.Node
}

// NewDrawer creates a closed drawer.
func NewDrawer(title string) *Drawer {
	d := &Drawer{Title: title, Width: 378} // Ant Drawer default
	d.rebuild()
	return d
}

// Node returns the portal node.
func (d *Drawer) Node() core.Node {
	if d.Portal == nil {
		d.rebuild()
	}
	return d.Portal
}

// SetContent sets body.
func (d *Drawer) SetContent(n core.Node) {
	d.content = n
	if d.body != nil {
		d.body.SetChild(n)
	}
}

// SetOpen shows/hides.
func (d *Drawer) SetOpen(open bool) {
	d.Open = open
	if d.Portal != nil {
		d.Portal.SetOpen(open)
	}
}

// SetPlacement sets side ("left" or "right").
func (d *Drawer) SetPlacement(p string) {
	d.Placement = p
	d.Left = p == "left"
	if d.Portal != nil {
		d.Portal.MarkNeedsLayout()
	}
}

// SetWidth sets panel width.
func (d *Drawer) SetWidth(w float64) {
	d.Width = w
	if d.Portal != nil {
		d.Portal.MarkNeedsLayout()
	}
}

func (d *Drawer) theme() *core.Theme {
	if d.Theme != nil {
		return d.Theme
	}
	return DefaultTheme()
}

func (d *Drawer) rebuild() {
	th := d.theme()
	d.title = primitive.NewText(d.Title)
	d.title.FontSize = 16
	d.title.Face = d.Face
	d.title.Color = th.Color(core.TokenColorText)
	d.body = primitive.NewSlot("body", d.content)
	closeBtn := NewButton("Close")
	closeBtn.SetType(ButtonText)
	closeBtn.SetFace(d.Face)
	closeBtn.SetOnClick(func() {
		if d.OnClose != nil {
			d.OnClose()
		}
		d.SetOpen(false)
	})
	head := primitive.Row(d.title, primitive.Spacer(), closeBtn.Node())
	head.CrossAlign = core.CrossCenter
	col := primitive.Column(head, primitive.NewDivider(), d.body)
	col.Gap = 12
	col.CrossAlign = core.CrossStart

	d.panel = primitive.NewDecorated(col)
	d.panel.Padding = primitive.All(16)
	d.panel.Background = th.Color(core.TokenColorBgContainer)
	d.panel.Radius = 0

	mask := primitive.NewMask()
	mask.OnDismiss = func() {
		if d.OnClose != nil {
			d.OnClose()
		}
		d.SetOpen(false)
	}

	layer := &drawerLayer{mask: mask, panel: d.panel, drawer: d}
	layer.Init(layer)
	layer.Hit = core.HitDefer
	layer.AddChild(mask)
	layer.AddChild(d.panel)

	d.Portal = primitive.NewOverlayPortal(layer)
	d.Portal.ID = "drawer"
	d.Portal.ZOrder = 400
}

type drawerLayer struct {
	core.NodeBase
	mask   *primitive.Mask
	panel  *primitive.Decorated
	drawer *Drawer
}

func (l *drawerLayer) TypeID() string { return "kit.DrawerLayer" }

func (l *drawerLayer) Layout(c core.Constraints) core.Size {
	vw, vh := c.MaxWidth, c.MaxHeight
	if l.drawer.Viewport.Width > 0 {
		vw, vh = l.drawer.Viewport.Width, l.drawer.Viewport.Height
	}
	if vw >= core.Unbounded/2 {
		vw = 800
	}
	if vh >= core.Unbounded/2 {
		vh = 600
	}
	l.mask.Width, l.mask.Height = vw, vh
	_ = l.mask.Layout(core.Tight(vw, vh))
	l.mask.SetOffset(core.Point{})

	w := l.drawer.Width
	if w <= 0 {
		w = 360
	}
	_ = l.panel.Layout(core.Tight(w, vh))
	x := vw - w
	if l.drawer.Left {
		x = 0
	}
	l.panel.SetOffset(core.Point{X: x, Y: 0})
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *drawerLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *drawerLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
