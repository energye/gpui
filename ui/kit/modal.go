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

	Open          bool
	Title         string
	Width         float64
	MaskClosable  bool
	FooterVisible bool // default true
	Face          text.Face
	Theme         *core.Theme
	Viewport      core.Size
	OnOk          func()
	OnCancel      func()
	// Content set via SetContent.
	content core.Node
}

// NewModal creates a closed modal.
func NewModal(title string) *Modal {
	m := &Modal{Title: title, Width: 480, MaskClosable: true, FooterVisible: true}
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

// SetTitle updates the dialog title.
func (m *Modal) SetTitle(title string) {
	m.Title = title
	if m.title != nil {
		m.title.Value = title
		m.title.MarkNeedsPaint()
	} else {
		m.rebuild()
	}
}

// SetFooterVisible shows or hides the OK/Cancel footer.
func (m *Modal) SetFooterVisible(v bool) {
	m.FooterVisible = v
	m.rebuild()
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

	col := primitive.Column(m.title, m.bodySlot)
	if m.FooterVisible {
		col.AddChild(m.footer)
	}
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
	if (vw <= 0 || vh <= 0) && m.Portal != nil {
		if t := m.Portal.Tree(); t != nil {
			tv := t.Viewport()
			if vw <= 0 && tv.Width > 0 {
				vw = tv.Width
			}
			if vh <= 0 && tv.Height > 0 {
				vh = tv.Height
			}
		}
	}
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
	if l.modal != nil && l.modal.Viewport.Width > 0 && l.modal.Viewport.Height > 0 {
		vw, vh = l.modal.Viewport.Width, l.modal.Viewport.Height
	} else if l.modal != nil {
		// Fall back to tree viewport so mask still covers full client when host
		// forgets to set Modal.Viewport (Ant: mask = full window).
		if t := l.Tree(); t != nil {
			tv := t.Viewport()
			if tv.Width > 0 && tv.Height > 0 {
				vw, vh = tv.Width, tv.Height
			}
		}
	}
	if vw >= core.Unbounded/2 || vw <= 0 {
		vw = 800
	}
	if vh >= core.Unbounded/2 || vh <= 0 {
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

func (l *modalLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *modalLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
