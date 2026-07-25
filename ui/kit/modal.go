package kit

import (
	"sync/atomic"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Modal defaults — docs/antd/modal.md §6.2 / style/index.ts (non-wireframe).
const (
	DefaultModalWidth       = 520.0
	DefaultModalTop         = 100.0
	DefaultModalPaddingX    = 24.0 // paddingContentHorizontalLG
	DefaultModalPaddingY    = 16.0 // paddingMD
	DefaultModalPadding     = 24.0 // uniform alias (SetPadding); rebuild prefers XY defaults
	DefaultModalTitleFont   = 16.0 // fontSizeHeading5
	DefaultModalBodyGap     = 8.0  // headerMarginBottom / marginXS
	DefaultModalFooterGap   = 8.0  // gap between footer buttons (marginXS)
	DefaultModalFooterTop   = 12.0 // footerMarginTop / marginSM
	DefaultModalCloseSize   = 32.0 // controlHeight
	DefaultModalMaxHeightFr = 0.9
)

// Modal is a dialog over an optional mask (Ant Design Modal).
//
//	OverlayPortal
//	  └─ FocusScope
//	       └─ modalLayer: Mask? · Decorated(panel)
//
// Product contract: docs/antd/modal.md §6 P0. Portal / Scope / layer stay stable
// across rebuilds while open so open state and focus trap are not lost.
type Modal struct {
	Portal *primitive.OverlayPortal
	Scope  *primitive.FocusScope

	layer      *modalLayer
	panel      *primitive.Decorated
	titleNode  *primitive.Text
	bodySlot   *primitive.Slot
	footerSlot *primitive.Slot
	closeBtn   *Button
	okBtn      *Button
	cancelBtn  *Button
	loadingSk  *Skeleton

	Open            bool
	Title           string
	Width           float64 // 0 → DefaultModalWidth
	Top             float64 // 0 → DefaultModalTop when !Centered
	Centered        bool
	Closable        bool
	Mask            bool
	MaskClosable    bool
	Keyboard        bool
	Loading         bool
	ConfirmLoading  bool
	DestroyOnHidden bool
	OkText          string
	CancelText      string
	OkType          ButtonType
	FooterVisible   bool // false ≡ footer=null; custom footer also hides defaults

	// Padding uniform panel inset (0 → contentPadding via XY defaults). SetPaddingInsets for sides.
	Padding       float64
	TitleFontSize float64 // 0 → DefaultModalTitleFont
	BodyGap       float64 // 0 → DefaultModalBodyGap
	FooterGap     float64 // 0 → DefaultModalFooterGap
	Face          text.Face
	Theme         *core.Theme
	Viewport      core.Size
	AriaLabel     string

	OnOk         func()
	OnCancel     func()
	AfterClose   func()
	OnOpenChange func(open bool)

	content      core.Node
	customFooter core.Node
	footerRender func(ok, cancel core.Node) core.Node
	footerNull   bool
	trap         overlayFocusTrap
	pad          primitive.EdgeInsets
	padSet       bool
	life         tickerLifecycle
	okGuard      bool // true while ConfirmLoading swallows extra OK clicks
}

// NewModal creates a closed modal with Ant defaults.
func NewModal(title string) *Modal {
	m := &Modal{
		Title:         title,
		Width:         DefaultModalWidth,
		Closable:      true,
		Mask:          true,
		MaskClosable:  true,
		Keyboard:      true,
		FooterVisible: true,
		OkText:        "OK",
		CancelText:    "Cancel",
		OkType:        ButtonPrimary,
	}
	m.rebuild()
	return m
}

// Node returns the portal host node to place in the tree.
func (m *Modal) Node() core.Node {
	if m == nil {
		return nil
	}
	if m.Portal == nil {
		m.rebuild()
	}
	return m.Portal
}

// SetOpen shows or hides the modal.
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
	if m.bodySlot != nil {
		m.bodySlot.SetChild(m.bodyNode())
	}
	if open {
		m.trap.wire(m.Scope, true, m.onEscape)
		m.layoutPanel()
		var prefer core.Node
		if m.okBtn != nil && m.FooterVisible && !m.footerNull && m.customFooter == nil && m.footerRender == nil {
			prefer = m.okBtn.Root
		} else if m.closeBtn != nil && m.Closable {
			prefer = m.closeBtn.Root
		}
		m.trap.enter(m.Scope, m.Portal, prefer)
	} else if was {
		m.trap.wire(m.Scope, false, nil)
		m.trap.leave(m.Scope, m.Portal)
		if m.AfterClose != nil {
			m.AfterClose()
		}
	} else {
		m.trap.wire(m.Scope, false, nil)
	}
	if was != open {
		m.life.setActive(m.Loading && open)
		if m.OnOpenChange != nil {
			m.OnOpenChange(open)
		}
	}
	if m.layer != nil {
		m.layer.MarkNeedsLayout()
		m.layer.MarkNeedsPaint()
	}
}

// SetTitle updates the dialog title and accessible name.
func (m *Modal) SetTitle(title string) {
	if m == nil {
		return
	}
	m.Title = title
	if m.titleNode != nil {
		m.titleNode.SetValue(title)
	}
	if m.layer != nil {
		m.layer.Label = m.dialogLabel()
	}
}

// SetContent sets the body node.
func (m *Modal) SetContent(n core.Node) {
	if m == nil {
		return
	}
	m.content = n
	if m.bodySlot != nil {
		m.bodySlot.SetChild(m.bodyNode())
	} else {
		m.rebuild()
	}
}

// SetFooter replaces the default OK/Cancel footer with a custom node.
// Pass nil and use SetFooterNull / SetFooterVisible(false) for footer=null.
func (m *Modal) SetFooter(n core.Node) {
	if m == nil {
		return
	}
	m.customFooter = n
	m.footerRender = nil
	if n != nil {
		m.footerNull = false
		m.FooterVisible = true
	}
	m.rebuild()
}

// SetFooterNull hides the footer (antd footer={null}).
func (m *Modal) SetFooterNull() {
	if m == nil {
		return
	}
	m.footerNull = true
	m.FooterVisible = false
	m.customFooter = nil
	m.footerRender = nil
	m.rebuild()
}

// SetFooterRender builds the footer from default Ok/Cancel button nodes
// (antd footer render function with OkBtn / CancelBtn).
func (m *Modal) SetFooterRender(fn func(ok, cancel core.Node) core.Node) {
	if m == nil {
		return
	}
	m.footerRender = fn
	m.customFooter = nil
	if fn != nil {
		m.footerNull = false
		m.FooterVisible = true
	}
	m.rebuild()
}

// SetFooterVisible shows or hides the default footer.
// false is equivalent to SetFooterNull; true restores the default footer.
func (m *Modal) SetFooterVisible(v bool) {
	if m == nil {
		return
	}
	if v {
		m.footerNull = false
		m.FooterVisible = true
		m.customFooter = nil
		m.footerRender = nil
	} else {
		m.footerNull = true
		m.FooterVisible = false
		m.customFooter = nil
		m.footerRender = nil
	}
	m.rebuild()
}

// SetOkText sets the OK button label (i18n).
func (m *Modal) SetOkText(s string) {
	if m == nil {
		return
	}
	if s == "" {
		s = "OK"
	}
	m.OkText = s
	if m.okBtn != nil {
		m.okBtn.SetLabel(s)
	} else {
		m.rebuild()
	}
}

// SetCancelText sets the Cancel button label (i18n).
func (m *Modal) SetCancelText(s string) {
	if m == nil {
		return
	}
	if s == "" {
		s = "Cancel"
	}
	m.CancelText = s
	if m.cancelBtn != nil {
		m.cancelBtn.SetLabel(s)
	} else {
		m.rebuild()
	}
}

// SetOkType sets the OK button type (default primary).
func (m *Modal) SetOkType(t ButtonType) {
	if m == nil {
		return
	}
	m.OkType = t
	if m.okBtn != nil {
		m.okBtn.SetType(t)
	} else {
		m.rebuild()
	}
}

// SetConfirmLoading toggles OK button loading and blocks repeat OK clicks.
func (m *Modal) SetConfirmLoading(v bool) {
	if m == nil {
		return
	}
	m.ConfirmLoading = v
	m.okGuard = v
	if m.okBtn != nil {
		m.okBtn.SetLoading(v)
	}
}

// SetClosable shows or hides the top-right close control.
func (m *Modal) SetClosable(v bool) {
	if m == nil || m.Closable == v {
		return
	}
	m.Closable = v
	m.rebuild()
}

// SetMask shows or hides the dim mask.
func (m *Modal) SetMask(v bool) {
	if m == nil || m.Mask == v {
		return
	}
	m.Mask = v
	m.rebuild()
}

// SetMaskClosable controls whether mask click dismisses the dialog.
func (m *Modal) SetMaskClosable(v bool) {
	if m != nil {
		m.MaskClosable = v
	}
}

// SetKeyboard enables Esc → cancel/close.
func (m *Modal) SetKeyboard(v bool) {
	if m == nil {
		return
	}
	m.Keyboard = v
	m.trap.wire(m.Scope, m.Open, m.onEscape)
}

// SetCentered toggles vertical centering (default false → top offset layout).
func (m *Modal) SetCentered(v bool) {
	if m == nil || m.Centered == v {
		return
	}
	m.Centered = v
	if m.layer != nil {
		m.layer.MarkNeedsLayout()
	}
}

// SetWidth sets dialog width (0 → DefaultModalWidth on next rebuild).
func (m *Modal) SetWidth(w float64) {
	if m == nil {
		return
	}
	m.Width = w
	m.rebuild()
}

// SetTop sets the non-centered top offset (0 → DefaultModalTop).
func (m *Modal) SetTop(top float64) {
	if m == nil {
		return
	}
	m.Top = top
	if m.layer != nil {
		m.layer.MarkNeedsLayout()
	}
}

// SetLoading toggles body skeleton. Animates through Tree ticker while open.
func (m *Modal) SetLoading(v bool) {
	if m == nil || m.Loading == v {
		return
	}
	m.Loading = v
	if m.bodySlot != nil {
		m.bodySlot.SetChild(m.bodyNode())
	} else {
		m.rebuild()
	}
	m.life.setActive(v && m.Open)
}

// SetDestroyOnHidden unmounts body children while closed.
func (m *Modal) SetDestroyOnHidden(v bool) {
	if m == nil {
		return
	}
	m.DestroyOnHidden = v
	if m.bodySlot != nil {
		m.bodySlot.SetChild(m.bodyNode())
	}
}

// SetFace applies the product font.
func (m *Modal) SetFace(face text.Face) {
	if m == nil {
		return
	}
	m.Face = face
	m.rebuild()
}

// SetTheme applies an explicit theme override.
func (m *Modal) SetTheme(th *core.Theme) {
	if m == nil {
		return
	}
	m.Theme = th
	m.rebuild()
}

// SetAriaLabel overrides the dialog accessible name (default = Title).
func (m *Modal) SetAriaLabel(name string) {
	if m == nil {
		return
	}
	m.AriaLabel = name
	if m.layer != nil {
		m.layer.Label = m.dialogLabel()
	}
}

// SetPadding sets uniform panel inset (0 → content padding defaults).
func (m *Modal) SetPadding(px float64) {
	if m == nil {
		return
	}
	m.Padding = px
	m.padSet = false
	m.rebuild()
}

// SetPaddingInsets sets per-side panel inset (explicit, including all-zero).
func (m *Modal) SetPaddingInsets(p primitive.EdgeInsets) {
	if m == nil {
		return
	}
	m.pad = p
	m.padSet = true
	m.rebuild()
}

// AttachTicker registers loading skeleton animation.
func (m *Modal) AttachTicker(t *core.Tree) {
	if m != nil {
		m.life.attach(t, m, m.Loading && m.Open)
		if m.loadingSk != nil {
			m.loadingSk.AttachTicker(t)
		}
	}
}

// Tick advances loading skeleton animation.
func (m *Modal) Tick(dt float64) bool {
	if m == nil || !m.Loading || !m.Open {
		return false
	}
	if m.loadingSk != nil {
		_ = m.loadingSk.Tick(dt)
	}
	if m.bodySlot != nil {
		m.bodySlot.MarkNeedsPaint()
	}
	return m.Loading && m.Open
}

// Sync refreshes footer button chrome. Prefer Tree.Layout; kept for host loops
// that still call per-frame Sync on open overlays.
func (m *Modal) Sync() {
	if m == nil {
		return
	}
	if m.okBtn != nil {
		m.okBtn.SyncState()
	}
	if m.cancelBtn != nil {
		m.cancelBtn.SyncState()
	}
	if m.closeBtn != nil {
		m.closeBtn.SyncState()
	}
	if m.Open {
		m.layoutPanel()
	}
}

// OkButton exposes the default OK button for tests / footer-render demos.
func (m *Modal) OkButton() *Button {
	if m == nil {
		return nil
	}
	return m.okBtn
}

// CancelButton exposes the default Cancel button.
func (m *Modal) CancelButton() *Button {
	if m == nil {
		return nil
	}
	return m.cancelBtn
}

// CloseButton exposes the top-right close control when Closable.
func (m *Modal) CloseButton() *Button {
	if m == nil {
		return nil
	}
	return m.closeBtn
}

// Panel returns the dialog panel Decorated (tests / metrics).
func (m *Modal) Panel() *primitive.Decorated {
	if m == nil {
		return nil
	}
	return m.panel
}

func (m *Modal) dialogLabel() string {
	if m == nil {
		return ""
	}
	if m.AriaLabel != "" {
		return m.AriaLabel
	}
	return m.Title
}

func (m *Modal) onEscape() {
	if m == nil || !m.Open || !m.Keyboard {
		return
	}
	m.cancelFromUser()
}

func (m *Modal) cancelFromUser() {
	if m == nil {
		return
	}
	if m.OnCancel != nil {
		m.OnCancel()
	}
	m.SetOpen(false)
}

func (m *Modal) okFromUser() {
	if m == nil {
		return
	}
	if m.ConfirmLoading || m.okGuard {
		return
	}
	if m.OnOk != nil {
		m.OnOk()
	}
	// Controlled: do not auto-close. Callers / confirm host close explicitly.
}

func (m *Modal) theme() *core.Theme {
	var n core.Node
	if m.Portal != nil {
		n = m.Portal
	}
	return themeOf(m.Theme, n)
}

func (m *Modal) panelPadding() primitive.EdgeInsets {
	if m != nil && m.padSet {
		return m.pad
	}
	if m != nil && m.Padding > 0 {
		return primitive.All(m.Padding)
	}
	// antd contentPadding: paddingMD Y + paddingContentHorizontalLG X
	return primitive.EdgeInsets{
		Top:    DefaultModalPaddingY,
		Bottom: DefaultModalPaddingY,
		Left:   DefaultModalPaddingX,
		Right:  DefaultModalPaddingX,
	}
}

func (m *Modal) titleFont() float64 {
	if m != nil && m.TitleFontSize > 0 {
		return m.TitleFontSize
	}
	return DefaultModalTitleFont
}

func (m *Modal) bodyGap() float64 {
	if m != nil && m.BodyGap > 0 {
		return m.BodyGap
	}
	return DefaultModalBodyGap
}

func (m *Modal) footerGap() float64 {
	if m != nil && m.FooterGap > 0 {
		return m.FooterGap
	}
	return DefaultModalFooterGap
}

func (m *Modal) resolvedWidth() float64 {
	if m != nil && m.Width > 0 {
		return m.Width
	}
	return DefaultModalWidth
}

func (m *Modal) resolvedTop() float64 {
	if m != nil && m.Top > 0 {
		return m.Top
	}
	return DefaultModalTop
}

func (m *Modal) bodyNode() core.Node {
	if m == nil {
		return nil
	}
	if m.DestroyOnHidden && !m.Open {
		return nil
	}
	if m.Loading {
		w := m.resolvedWidth() - DefaultModalPaddingX*2
		if w < 120 {
			w = 120
		}
		m.loadingSk = NewSkeleton(w, 16)
		m.loadingSk.SetRows(3)
		m.loadingSk.Theme = m.Theme
		return m.loadingSk.Node()
	}
	m.loadingSk = nil
	return m.content
}

func (m *Modal) ensureDefaultButtons() {
	okLabel := m.OkText
	if okLabel == "" {
		okLabel = "OK"
	}
	cancelLabel := m.CancelText
	if cancelLabel == "" {
		cancelLabel = "Cancel"
	}
	if m.okBtn == nil {
		m.okBtn = NewButton(okLabel)
	} else {
		m.okBtn.SetLabel(okLabel)
	}
	m.okBtn.SetType(m.OkType)
	m.okBtn.SetFace(m.Face)
	m.okBtn.SetLoading(m.ConfirmLoading)
	m.okBtn.SetOnClick(func() { m.okFromUser() })

	if m.cancelBtn == nil {
		m.cancelBtn = NewButton(cancelLabel)
	} else {
		m.cancelBtn.SetLabel(cancelLabel)
	}
	m.cancelBtn.SetType(ButtonDefault)
	m.cancelBtn.SetFace(m.Face)
	m.cancelBtn.SetOnClick(func() { m.cancelFromUser() })
}

func (m *Modal) buildFooterNode() core.Node {
	if m.footerNull || !m.FooterVisible {
		return nil
	}
	m.ensureDefaultButtons()
	if m.footerRender != nil {
		return m.footerRender(m.okBtn.Node(), m.cancelBtn.Node())
	}
	if m.customFooter != nil {
		return m.customFooter
	}
	row := primitive.Row(primitive.Spacer(), m.cancelBtn.Node(), m.okBtn.Node())
	row.Gap = m.footerGap()
	row.CrossAlign = core.CrossCenter
	return row
}

func (m *Modal) rebuild() {
	if m == nil {
		return
	}
	th := m.theme()
	if m.Width <= 0 {
		m.Width = DefaultModalWidth
	}
	if m.OkText == "" {
		m.OkText = "OK"
	}
	if m.CancelText == "" {
		m.CancelText = "Cancel"
	}

	m.titleNode = primitive.NewText(m.Title)
	m.titleNode.FontSize = m.titleFont()
	m.titleNode.Face = m.Face
	m.titleNode.Color = th.Color(core.TokenColorText)

	titleRowKids := []core.Node{m.titleNode, primitive.Spacer()}
	if m.Closable {
		m.closeBtn = NewButton("×")
		m.closeBtn.SetType(ButtonText)
		m.closeBtn.SetAriaLabel("Close")
		m.closeBtn.SetFace(m.Face)
		m.closeBtn.SetFixedSize(DefaultModalCloseSize, DefaultModalCloseSize)
		m.closeBtn.SetOnClick(func() { m.cancelFromUser() })
		titleRowKids = append(titleRowKids, m.closeBtn.Node())
	} else {
		m.closeBtn = nil
	}
	titleRow := primitive.Row(titleRowKids...)
	titleRow.Gap = 8
	titleRow.CrossAlign = core.CrossCenter

	m.bodySlot = primitive.NewSlot("body", m.bodyNode())

	colKids := []core.Node{titleRow, m.bodySlot}
	footerNode := m.buildFooterNode()
	if footerNode != nil {
		// footer top margin via spacer-ish gap on a nested column section
		footWrap := primitive.Column(footerNode)
		footWrap.Padding = primitive.EdgeInsets{Top: DefaultModalFooterTop}
		m.footerSlot = primitive.NewSlot("footer", footWrap)
		colKids = append(colKids, m.footerSlot)
	} else {
		m.footerSlot = nil
		// Still ensure default buttons exist for OkButton()/footer-render callers
		// that toggle visibility after the fact — skip if footerNull permanently.
		if !m.footerNull {
			m.ensureDefaultButtons()
		} else {
			m.okBtn = nil
			m.cancelBtn = nil
		}
	}

	col := primitive.Column(colKids...)
	col.Gap = m.bodyGap()
	col.CrossAlign = core.CrossStretch

	m.panel = primitive.NewDecorated(col)
	m.panel.Padding = m.panelPadding()
	// antd contentBg = colorBgElevated; kit falls back to container white.
	bg := th.Color(core.TokenColorBgContainer)
	m.panel.Background = bg
	m.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	m.panel.BorderWidth = 0
	m.panel.MinWidth = m.resolvedWidth()

	var mask *primitive.Mask
	if m.Mask {
		mask = primitive.NewMask()
		mask.OnDismiss = func() {
			if m.MaskClosable {
				m.cancelFromUser()
			}
		}
	}

	if m.layer == nil {
		m.layer = &modalLayer{modal: m}
		m.layer.Init(m.layer)
		m.layer.Hit = core.HitDefer
		m.layer.Role = "dialog"
	}
	m.layer.modal = m
	m.layer.mask = mask
	m.layer.panel = m.panel
	m.layer.Label = m.dialogLabel()
	m.layer.ClearChildren()
	if mask != nil {
		m.layer.AddChild(mask)
	}
	m.layer.AddChild(m.panel)

	if m.Scope == nil {
		m.Scope = primitive.NewFocusScope(m.layer)
	}
	m.trap.wire(m.Scope, m.Open, m.onEscape)

	if m.Portal == nil {
		m.Portal = primitive.NewOverlayPortal(m.Scope)
		// Leave ID empty so multi-instance portals get unique stable ids.
		m.Portal.ZOrder = OverlayZModal
		m.Portal.SetContentOffset(core.Point{})
	} else {
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
	if m == nil || m.panel == nil {
		return
	}
	if m.layer != nil {
		m.layer.MarkNeedsLayout()
	}
}

// modalLayer lays out optional mask fullscreen and positions the panel
// (centered or top-offset).
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

	w := DefaultModalWidth
	if l.modal != nil {
		w = l.modal.resolvedWidth()
	}
	maxH := vh * DefaultModalMaxHeightFr
	if maxH < 1 {
		maxH = vh
	}
	if l.panel != nil {
		_ = l.panel.Layout(core.Constraints{MaxWidth: w, MaxHeight: maxH, MinWidth: w})
		pw, ph := l.panel.Size().Width, l.panel.Size().Height
		x := (vw - pw) / 2
		if x < 0 {
			x = 0
		}
		var y float64
		if l.modal != nil && l.modal.Centered {
			y = (vh - ph) / 2
		} else {
			top := DefaultModalTop
			if l.modal != nil {
				top = l.modal.resolvedTop()
			}
			y = top
			if y+ph > vh {
				// keep fully visible when possible
				y = vh - ph
			}
		}
		if y < 0 {
			y = 0
		}
		l.panel.SetOffset(core.Point{X: x, Y: y})
	}
	out := core.Size{Width: vw, Height: vh}
	l.SetSize(out)
	return out
}

func (l *modalLayer) Paint(pc *core.PaintContext) { l.DefaultPaintChildren(pc) }

func (l *modalLayer) HitTest(p core.Point) core.Node { return l.DefaultHitTest(p) }

// ---------------------------------------------------------------------------
// ModalHost — hooks / static-method equivalent (Modal.useModal / Modal.confirm)
// ---------------------------------------------------------------------------

var modalConfirmSeq atomic.Uint64

// ModalConfirmType selects the icon / chrome for imperative dialogs.
type ModalConfirmType string

const (
	ModalConfirmDefault ModalConfirmType = "confirm"
	ModalConfirmInfo    ModalConfirmType = "info"
	ModalConfirmSuccess ModalConfirmType = "success"
	ModalConfirmError   ModalConfirmType = "error"
	ModalConfirmWarning ModalConfirmType = "warning"
)

// ModalConfirmConfig is the Go-side config for Modal.confirm / info / …
type ModalConfirmConfig struct {
	Title      string
	Content    string
	Icon       string // icon name; empty → type default
	Type       ModalConfirmType
	OkText     string
	CancelText string
	OkCancel   bool // default true for confirm; false for info/success/error/warning
	Centered   bool
	Width      float64
	Keyboard   bool // default true
	Mask       bool // default true
	OnOk       func()
	OnCancel   func()
	AfterClose func()
}

// ModalConfirmHandle is returned from host.Confirm and friends.
type ModalConfirmHandle struct {
	host    *ModalHost
	id      uint64
	modal   *Modal
	destroy func()
}

// Destroy closes and removes the confirm dialog.
func (h *ModalConfirmHandle) Destroy() {
	if h == nil {
		return
	}
	if h.destroy != nil {
		h.destroy()
	}
}

// Update patches title/content/loading-like fields on the live dialog.
func (h *ModalConfirmHandle) Update(cfg ModalConfirmConfig) {
	if h == nil || h.modal == nil {
		return
	}
	if cfg.Title != "" {
		h.modal.SetTitle(cfg.Title)
	}
	if cfg.Content != "" {
		h.modal.SetContent(confirmBodyNode(cfg))
	}
	if cfg.OkText != "" {
		h.modal.SetOkText(cfg.OkText)
	}
	if cfg.CancelText != "" {
		h.modal.SetCancelText(cfg.CancelText)
	}
	if cfg.OnOk != nil {
		h.modal.OnOk = cfg.OnOk
	}
	if cfg.OnCancel != nil {
		h.modal.OnCancel = cfg.OnCancel
	}
}

// Modal returns the underlying modal (tests / advanced).
func (h *ModalConfirmHandle) Modal() *Modal {
	if h == nil {
		return nil
	}
	return h.modal
}

type modalHostItem struct {
	id     uint64
	modal  *Modal
	handle *ModalConfirmHandle
}

// ModalHost holds imperative confirm dialogs (antd Modal.useModal contextHolder).
// Mount Node() once under the app root (same pattern as MessageHost).
type ModalHost struct {
	Root     *primitive.Slot
	Face     text.Face
	Theme    *core.Theme
	Viewport core.Size
	items    []*modalHostItem
	stack    *primitive.Flex
}

// NewModalHost creates an empty host.
func NewModalHost() *ModalHost {
	h := &ModalHost{}
	h.stack = primitive.Column()
	h.Root = primitive.NewSlot("modal-host", h.stack)
	return h
}

// Node returns the host mount point (zero-size stack of portals).
func (h *ModalHost) Node() core.Node {
	if h == nil {
		return nil
	}
	if h.Root == nil {
		h.stack = primitive.Column()
		h.Root = primitive.NewSlot("modal-host", h.stack)
	}
	return h.Root
}

// Confirm opens a confirm dialog (OK + Cancel).
func (h *ModalHost) Confirm(cfg ModalConfirmConfig) *ModalConfirmHandle {
	if cfg.Type == "" {
		cfg.Type = ModalConfirmDefault
	}
	if !cfg.OkCancel && cfg.Type == ModalConfirmDefault {
		cfg.OkCancel = true
	}
	if cfg.Type == ModalConfirmDefault {
		cfg.OkCancel = true
	}
	return h.openConfirm(cfg)
}

// Info opens an info dialog (OK only by default).
func (h *ModalHost) Info(cfg ModalConfirmConfig) *ModalConfirmHandle {
	cfg.Type = ModalConfirmInfo
	cfg.OkCancel = false
	return h.openConfirm(cfg)
}

// Success opens a success dialog (OK only).
func (h *ModalHost) Success(cfg ModalConfirmConfig) *ModalConfirmHandle {
	cfg.Type = ModalConfirmSuccess
	cfg.OkCancel = false
	return h.openConfirm(cfg)
}

// Error opens an error dialog (OK only).
func (h *ModalHost) Error(cfg ModalConfirmConfig) *ModalConfirmHandle {
	cfg.Type = ModalConfirmError
	cfg.OkCancel = false
	return h.openConfirm(cfg)
}

// Warning opens a warning dialog (OK only).
func (h *ModalHost) Warning(cfg ModalConfirmConfig) *ModalConfirmHandle {
	cfg.Type = ModalConfirmWarning
	cfg.OkCancel = false
	return h.openConfirm(cfg)
}

func confirmIconName(cfg ModalConfirmConfig) string {
	if cfg.Icon != "" {
		return cfg.Icon
	}
	switch cfg.Type {
	case ModalConfirmInfo:
		return "info-circle"
	case ModalConfirmSuccess:
		return "check-circle"
	case ModalConfirmError:
		return "close-circle"
	case ModalConfirmWarning, ModalConfirmDefault:
		return "exclamation-circle"
	default:
		return "exclamation-circle"
	}
}

func confirmBodyNode(cfg ModalConfirmConfig) core.Node {
	iconName := confirmIconName(cfg)
	row := primitive.Row()
	row.Gap = 12
	row.CrossAlign = core.CrossStart
	// Prefer kit icon when available; fall back to text glyph.
	if iconName != "" {
		if ic := tryModalIcon(iconName); ic != nil {
			row.AddChild(ic)
		} else {
			glyph := primitive.NewText(modalIconGlyph(cfg.Type))
			glyph.FontSize = 22
			row.AddChild(glyph)
		}
	}
	body := primitive.Column()
	body.Gap = 4
	if cfg.Content != "" {
		body.AddChild(NewText(cfg.Content).Node())
	}
	row.AddChild(body)
	return row
}

func tryModalIcon(name string) core.Node {
	ic := NewIcon(name)
	if ic == nil {
		return nil
	}
	ic.SetSize(22)
	return ic.Node()
}

func modalIconGlyph(t ModalConfirmType) string {
	switch t {
	case ModalConfirmSuccess:
		return "✓"
	case ModalConfirmError:
		return "✕"
	case ModalConfirmInfo:
		return "ℹ"
	default:
		return "!"
	}
}

func (h *ModalHost) openConfirm(cfg ModalConfirmConfig) *ModalConfirmHandle {
	if h == nil {
		h = NewModalHost()
	}
	if h.stack == nil {
		h.stack = primitive.Column()
		h.Root = primitive.NewSlot("modal-host", h.stack)
	}
	id := modalConfirmSeq.Add(1)
	title := cfg.Title
	if title == "" {
		switch cfg.Type {
		case ModalConfirmInfo:
			title = "Info"
		case ModalConfirmSuccess:
			title = "Success"
		case ModalConfirmError:
			title = "Error"
		case ModalConfirmWarning:
			title = "Warning"
		default:
			title = "Confirm"
		}
	}
	m := NewModal(title)
	m.SetFace(h.Face)
	m.Theme = h.Theme
	m.Viewport = h.Viewport
	if cfg.Width > 0 {
		m.SetWidth(cfg.Width)
	} else {
		m.SetWidth(416) // antd confirm default width
	}
	m.SetCentered(cfg.Centered)
	m.SetKeyboard(true)
	m.SetMask(true)
	m.SetContent(confirmBodyNode(cfg))
	if cfg.OkText != "" {
		m.SetOkText(cfg.OkText)
	}
	if cfg.CancelText != "" {
		m.SetCancelText(cfg.CancelText)
	}
	showCancel := cfg.Type == ModalConfirmDefault || cfg.Type == ""
	if cfg.Type != "" && cfg.Type != ModalConfirmDefault {
		showCancel = cfg.OkCancel
	}
	if cfg.Type == ModalConfirmDefault {
		showCancel = true
	}
	if !showCancel {
		// info/success/error/warning: only OK (antd justOkText path)
		m.SetFooterRender(func(ok, _ core.Node) core.Node {
			row := primitive.Row(primitive.Spacer(), ok)
			row.Gap = DefaultModalFooterGap
			return row
		})
	}

	handle := &ModalConfirmHandle{host: h, id: id, modal: m}
	var closed bool
	var afterCloseOnce bool
	closeAndRemove := func() {
		if closed {
			return
		}
		closed = true
		if m.Open {
			m.SetOpen(false)
		}
		h.removeItem(id)
		if !afterCloseOnce && cfg.AfterClose != nil {
			afterCloseOnce = true
			cfg.AfterClose()
		}
	}
	handle.destroy = closeAndRemove

	m.OnOk = func() {
		if cfg.OnOk != nil {
			cfg.OnOk()
		}
		closeAndRemove()
	}
	m.OnCancel = func() {
		if cfg.OnCancel != nil {
			cfg.OnCancel()
		}
	}
	m.AfterClose = func() {
		h.removeItem(id)
		if !afterCloseOnce && cfg.AfterClose != nil {
			afterCloseOnce = true
			cfg.AfterClose()
		}
	}

	item := &modalHostItem{id: id, modal: m, handle: handle}
	h.items = append(h.items, item)
	h.stack.AddChild(m.Node())
	m.SetOpen(true)
	if h.Root != nil {
		h.Root.MarkNeedsLayout()
	}
	return handle
}

func (h *ModalHost) removeItem(id uint64) {
	if h == nil {
		return
	}
	out := h.items[:0]
	for _, it := range h.items {
		if it.id == id {
			continue
		}
		out = append(out, it)
	}
	h.items = out
	// rebuild stack children
	if h.stack != nil {
		h.stack.ClearChildren()
		for _, it := range h.items {
			if it.modal != nil {
				h.stack.AddChild(it.modal.Node())
			}
		}
		h.stack.MarkNeedsLayout()
	}
}

// Count returns active imperative dialogs (tests).
func (h *ModalHost) Count() int {
	if h == nil {
		return 0
	}
	return len(h.items)
}

// Sync refreshes open confirm chrome.
func (h *ModalHost) Sync() {
	if h == nil {
		return
	}
	for _, it := range h.items {
		if it.modal != nil {
			it.modal.Viewport = h.Viewport
			it.modal.Sync()
		}
	}
}
