package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Modal is a centered dialog over a mask (B3).
//
//	OverlayPortal
//	  └─ Stack
//	       Mask · Decorated(panel)
//	            Flex(Column): title · content · footer
type Modal struct {
	Portal    *primitive.OverlayPortal
	Scope     *primitive.FocusScope
	panel     *primitive.Decorated
	title     *primitive.Text
	bodySlot  *primitive.Slot
	footer    *primitive.Flex
	okBtn     *Button
	cancelBtn *Button

	Open         bool
	Title        string
	Width        float64
	MaskClosable bool
	Face         text.Face
	Theme        *core.Theme
	Viewport     core.Size
	OnOk         func()
	OnCancel     func()
	// Content set via SetContent.
	content core.Node
}

// NewModal creates a closed modal.
func NewModal(title string) *Modal {
	m := &Modal{Title: title, Width: 480, MaskClosable: true}
	m.rebuild()
	return m
}

// Node returns a zero-size portal host node to place in the tree.
func (m *Modal) Node() core.Node {
	if m.Portal == nil {
		m.rebuild()
	}
	return m.Portal
}

// SetContent sets the body node.
func (m *Modal) SetContent(n core.Node) {
	m.content = n
	if m.bodySlot != nil {
		m.bodySlot.SetChild(n)
	} else {
		m.rebuild()
	}
}

// SetOpen shows/hides the modal.
func (m *Modal) SetOpen(open bool) {
	m.Open = open
	if m.Portal != nil {
		m.Portal.SetOpen(open)
	}
	if m.Scope != nil {
		m.Scope.Active = open
	}
	if open {
		m.layoutPanel()
	}
}

// SetFace applies the product font to title and footer buttons (same as gallery Buttons).
func (m *Modal) SetFace(face text.Face) {
	m.Face = face
	if m.title != nil {
		m.title.Face = face
	}
	if m.okBtn != nil {
		m.okBtn.SetFace(face)
	}
	if m.cancelBtn != nil {
		m.cancelBtn.SetFace(face)
	}
}

// Sync repositions the panel and refreshes footer button chrome (hover/press).
// Call once per frame while open — same role as Button.SyncState in the host loop.
func (m *Modal) Sync() {
	if m.okBtn != nil {
		m.okBtn.SyncState()
	}
	if m.cancelBtn != nil {
		m.cancelBtn.SyncState()
	}
	if m.Open {
		m.layoutPanel()
	}
}

func (m *Modal) theme() *core.Theme {
	if m.Theme != nil {
		return m.Theme
	}
	return DefaultTheme()
}

func (m *Modal) rebuild() {
	th := m.theme()
	if m.Width <= 0 {
		m.Width = 480
	}

	m.title = primitive.NewText(m.Title)
	m.title.FontSize = 16
	m.title.Face = m.Face
	m.title.Color = th.Color(core.TokenColorText)

	m.bodySlot = primitive.NewSlot("body", m.content)

	// Same Button kit as the Button tab — primary OK + default Cancel.
	m.okBtn = NewButton("OK")
	m.okBtn.SetType(ButtonPrimary)
	m.cancelBtn = NewButton("Cancel")
	m.cancelBtn.SetType(ButtonDefault)
	if m.Face != nil {
		m.okBtn.SetFace(m.Face)
		m.cancelBtn.SetFace(m.Face)
	}
	m.okBtn.SetOnClick(func() {
		if m.OnOk != nil {
			m.OnOk()
		}
		m.SetOpen(false)
	})
	m.cancelBtn.SetOnClick(func() {
		if m.OnCancel != nil {
			m.OnCancel()
		}
		m.SetOpen(false)
	})
	m.footer = primitive.Row(primitive.Spacer(), m.cancelBtn.Node(), m.okBtn.Node())
	m.footer.Gap = 8
	m.footer.CrossAlign = core.CrossCenter

	col := primitive.Column(m.title, m.bodySlot, m.footer)
	col.Gap = 16
	col.CrossAlign = core.CrossStart

	m.panel = primitive.NewDecorated(col)
	m.panel.Padding = primitive.All(24) // Ant Modal body padding
	m.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	m.panel.Background = th.Color(core.TokenColorBgContainer)
	m.panel.BorderWidth = 0
	m.panel.MinWidth = m.Width

	mask := primitive.NewMask()
	mask.OnDismiss = func() {
		if m.MaskClosable {
			if m.OnCancel != nil {
				m.OnCancel()
			}
			m.SetOpen(false)
		}
	}

	// Layer: full mask + centered panel via absolute offsets on a stack root
	// Use a custom root node that sizes to viewport in overlay layout.
	layer := &modalLayer{mask: mask, panel: m.panel, modal: m}
	layer.Init(layer)
	layer.Hit = core.HitDefer
	layer.AddChild(mask)
	layer.AddChild(m.panel)

	m.Scope = primitive.NewFocusScope(layer)
	m.Portal = primitive.NewOverlayPortal(m.Scope)
	m.Portal.ID = "modal"
	m.Portal.ZOrder = 500
	m.Portal.SetContentOffset(core.Point{})
}

func (m *Modal) layoutPanel() {
	if m.panel == nil {
		return
	}
	vw, vh := m.Viewport.Width, m.Viewport.Height
	if vw <= 0 {
		vw = 800
	}
	if vh <= 0 {
		vh = 600
	}
	// Layout children under loose viewport for sizing
	_ = m.panel.Layout(core.Loose(m.Width, vh))
	pw, ph := m.panel.Size().Width, m.panel.Size().Height
	if pw < m.Width {
		pw = m.Width
		m.panel.MinWidth = m.Width
	}
	x := (vw - pw) / 2
	y := (vh - ph) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	// mask fills viewport via modalLayer layout
	_ = x
	_ = y
	// offsets set in modalLayer.Layout
}

// modalLayer lays out mask fullscreen and centers panel.
type modalLayer struct {
	core.NodeBase
	mask  *primitive.Mask
	panel *primitive.Decorated
	modal *Modal
}

func (l *modalLayer) TypeID() string { return "kit.ModalLayer" }
func (l *modalLayer) Layout(c core.Constraints) core.Size {
	vw, vh := c.MaxWidth, c.MaxHeight
	if l.modal != nil && l.modal.Viewport.Width > 0 {
		vw, vh = l.modal.Viewport.Width, l.modal.Viewport.Height
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

	w := l.modal.Width
	if w <= 0 {
		w = 480
	}
	_ = l.panel.Layout(core.Constraints{MaxWidth: w, MaxHeight: vh * 0.9, MinWidth: w})
	pw, ph := l.panel.Size().Width, l.panel.Size().Height
	l.panel.SetOffset(core.Point{X: (vw - pw) / 2, Y: (vh - ph) / 2})
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}
func (l *modalLayer) Paint(pc *core.PaintContext)    { l.DefaultPaintChildren(pc) }
func (l *modalLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

// Drawer is a side panel with mask.
type Drawer struct {
	Portal *primitive.OverlayPortal
	panel  *primitive.Decorated
	title  *primitive.Text
	body   *primitive.Slot
	Open   bool
	Title  string
	Width  float64
	// Placement: left or right (default right).
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
func (l *drawerLayer) Paint(pc *core.PaintContext)    { l.DefaultPaintChildren(pc) }
func (l *drawerLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
