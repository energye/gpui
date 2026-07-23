package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Modal is a centered dialog over a mask (B3).
//
//	OverlayPortal
//	  └─ FocusScope
//	       └─ modalLayer: Mask · Decorated(panel)
//
// Portal/Scope stay stable across content rebuilds so open state is not lost
// when SetFooterVisible / SetContent triggers rebuild.
type Modal struct {
	Portal    *primitive.OverlayPortal
	Scope     *primitive.FocusScope
	layer     *modalLayer
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
	trap    overlayFocusTrap
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
// Opening activates FocusScope trap, focuses the primary action, and wires
// Escape → OnCancel + close (Ant keyboard). Closing restores previous focus.
func (m *Modal) SetOpen(open bool) {
	if m == nil {
		return
	}
	if m.Portal == nil {
		m.rebuild()
	}
	was := m.Open
	m.Open = open
	if m.Portal != nil {
		m.Portal.SetOpen(open)
	}
	if open {
		m.trap.wire(m.Scope, true, m.onEscape)
		m.layoutPanel()
		var prefer core.Node
		if m.okBtn != nil {
			prefer = m.okBtn.Root
		}
		m.trap.enter(m.Scope, m.Portal, prefer)
	} else if was {
		m.trap.wire(m.Scope, false, nil)
		m.trap.leave(m.Scope, m.Portal)
	} else {
		m.trap.wire(m.Scope, false, nil)
	}
}

func (m *Modal) onEscape() {
	if m == nil || !m.Open {
		return
	}
	// Ant: Esc closes the dialog (MaskClosable only gates mask click).
	if m.OnCancel != nil {
		m.OnCancel()
	}
	m.SetOpen(false)
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

// SetFooterVisible shows or hides the OK/Cancel footer without replacing the portal.
func (m *Modal) SetFooterVisible(v bool) {
	if m.FooterVisible == v {
		return
	}
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
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry (automatic).
// Kept for one-shot forced reposition after external layout changes.
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
	var n core.Node
	if m.Portal != nil {
		n = m.Portal
	}
	return themeOf(m.Theme, n)
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

	// Keep layer/portal identity stable when already built (open rebuild safe).
	if m.layer == nil {
		m.layer = &modalLayer{modal: m}
		m.layer.Init(m.layer)
		m.layer.Hit = core.HitDefer
		m.layer.Role = "dialog"
		m.layer.Label = m.Title
	}
	m.layer.mask = mask
	m.layer.panel = m.panel
	m.layer.ClearChildren()
	m.layer.AddChild(mask)
	m.layer.AddChild(m.panel)

	if m.Scope == nil {
		m.Scope = primitive.NewFocusScope(m.layer)
	} else {
		// FocusScope content is the layer; layer children already refreshed.
	}
	m.trap.wire(m.Scope, m.Open, m.onEscape)

	if m.Portal == nil {
		m.Portal = primitive.NewOverlayPortal(m.Scope)
		// Intentional singleton id for one Modal per app shell; multi-instance apps
		// can assign Portal.ID after NewModal.
		m.Portal.ID = "modal"
		m.Portal.ZOrder = OverlayZModal
		m.Portal.SetContentOffset(core.Point{})
	} else {
		// Content pointer may already be Scope; ensure portal still points at it.
		m.Portal.Content = m.Scope
		m.Portal.ZOrder = OverlayZModal
	}
	if m.Open {
		m.Portal.SetOpen(true)
		m.layer.MarkNeedsLayout()
		m.layer.MarkNeedsPaint()
	}
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
	if m.layer != nil {
		m.layer.MarkNeedsLayout()
	}
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
	var portal *primitive.OverlayPortal
	var vp core.Size
	if l.modal != nil {
		portal = l.modal.Portal
		vp = l.modal.Viewport
	}
	vw, vh := resolveOverlayViewport(vp, portal, c.MaxWidth, c.MaxHeight)
	if l.mask != nil {
		l.mask.Width, l.mask.Height = vw, vh
		_ = l.mask.Layout(core.Tight(vw, vh))
		l.mask.SetOffset(core.Point{})
	}

	w := 480.0
	if l.modal != nil && l.modal.Width > 0 {
		w = l.modal.Width
	}
	if l.panel != nil {
		_ = l.panel.Layout(core.Constraints{MaxWidth: w, MaxHeight: vh * 0.9, MinWidth: w})
		pw, ph := l.panel.Size().Width, l.panel.Size().Height
		l.panel.SetOffset(core.Point{X: (vw - pw) / 2, Y: (vh - ph) / 2})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *modalLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *modalLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }
