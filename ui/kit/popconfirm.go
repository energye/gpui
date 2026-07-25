package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Popconfirm defaults — components/popconfirm/style + docs/antd/popconfirm.md §6.2.
// Shared panel chrome comes from Popover (pad 12, radiusLG, gap 8).
const (
	DefaultPopconfirmOkText              = "OK"
	DefaultPopconfirmCancelText          = "Cancel"
	DefaultPopconfirmIconName            = "exclamation-circle"
	DefaultPopconfirmMessageMarginBottom = 8.0 // marginXS
	DefaultPopconfirmIconGap             = 8.0 // marginXS
	DefaultPopconfirmDescMarginTop       = 4.0 // marginXXS
	DefaultPopconfirmButtonGap           = 8.0 // marginXS
	DefaultPopconfirmTriggerLabel        = "Delete"
)

// Popconfirm is Ant Design Popconfirm: click trigger + confirm card (icon/title/description + OK/Cancel).
//
//	Popover (trigger=click, placement=top)
//	  └─ panel content:
//	       ├─ message: icon + title/description
//	       └─ buttons: [Cancel?] OK (size=small)
//
// Product contract: docs/antd/popconfirm.md §6 (P0 DoD).
type Popconfirm struct {
	*Popover

	// Title is plain-text title when TitleNode is nil.
	Title string
	// TitleNode optional custom title (overrides Title).
	TitleNode core.Node
	// Description plain-text body when DescriptionNode is nil.
	Description string
	// DescriptionNode optional custom description.
	DescriptionNode core.Node

	// Icon is a registry icon name used when ShowIcon and IconNode is nil.
	Icon string
	// IconNode custom leading icon (preferred over Icon).
	IconNode core.Node
	// ShowIcon defaults true. False hides the leading icon.
	ShowIcon bool

	// OkText / CancelText button labels (empty → defaults).
	OkText     string
	CancelText string
	// OkType defaults to ButtonPrimary.
	OkType ButtonType
	// ShowCancel defaults true.
	ShowCancel bool
	// ConfirmLoading maps okButtonProps.loading (async demo).
	ConfirmLoading bool

	// OnConfirm sync path: fires then closes (uncontrolled) or requests close (controlled),
	// unless ConfirmLoading is left true by the callback (controlled async demo).
	OnConfirm func()
	// OnConfirmAsync Promise path: OK enters loading; call finish() to clear loading and close.
	// When set, takes precedence over OnConfirm for the OK click path.
	OnConfirmAsync func(finish func())
	// OnCancel fires on cancel then closes / requests close.
	OnCancel func()
	// OnPopupClick fires when the confirm card message area is pressed.
	OnPopupClick func()

	titleLab  *primitive.Text
	descLab   *primitive.Text
	okBtn     *Button
	cancelBtn *Button
	iconHost  core.Node

	// asyncPending is true while OnConfirmAsync has not finished.
	asyncPending bool
}

// NewPopconfirm creates a Popconfirm with the given title.
// Defaults (§6.10): trigger=click, placement=top, arrow=true, showCancel=true,
// okType=primary, ok/cancel text OK/Cancel, closed, uncontrolled.
func NewPopconfirm(title string) *Popconfirm {
	pc := &Popconfirm{
		Title:      title,
		OkText:     DefaultPopconfirmOkText,
		CancelText: DefaultPopconfirmCancelText,
		OkType:     ButtonPrimary,
		ShowCancel: true,
		ShowIcon:   true,
		Icon:       DefaultPopconfirmIconName,
	}
	pc.Popover = NewPopover(DefaultPopconfirmTriggerLabel)
	// Popconfirm overrides Popover hover default → click (antd mergedTrigger).
	pc.Popover.SetTrigger(PopoverTriggerClick)
	pc.Popover.SetPlacement(PopoverTop)
	pc.Popover.SetArrow(true)
	pc.rebuildContent()
	return pc
}

// Node returns the composition root (trigger + popup).
func (p *Popconfirm) Node() core.Node {
	if p == nil || p.Popover == nil {
		return nil
	}
	return p.Popover.Node()
}

// OkButton returns the confirm button (tests / advanced).
func (p *Popconfirm) OkButton() *Button {
	if p == nil {
		return nil
	}
	return p.okBtn
}

// CancelButton returns the cancel button (tests / advanced).
func (p *Popconfirm) CancelButton() *Button {
	if p == nil {
		return nil
	}
	return p.cancelBtn
}

// SetTitle sets plain-text title (clears TitleNode).
func (p *Popconfirm) SetTitle(title string) {
	if p == nil {
		return
	}
	p.Title = title
	p.TitleNode = nil
	p.rebuildContent()
}

// SetTitleNode sets a custom title node.
func (p *Popconfirm) SetTitleNode(n core.Node) {
	if p == nil {
		return
	}
	p.TitleNode = n
	p.rebuildContent()
}

// SetDescription sets plain-text description (clears DescriptionNode).
func (p *Popconfirm) SetDescription(desc string) {
	if p == nil {
		return
	}
	p.Description = desc
	p.DescriptionNode = nil
	p.rebuildContent()
}

// SetDescriptionNode sets a custom description node.
func (p *Popconfirm) SetDescriptionNode(n core.Node) {
	if p == nil {
		return
	}
	p.DescriptionNode = n
	p.rebuildContent()
}

// SetIcon sets the registry icon name (clears IconNode).
func (p *Popconfirm) SetIcon(name string) {
	if p == nil {
		return
	}
	p.Icon = name
	p.IconNode = nil
	p.ShowIcon = true
	p.rebuildContent()
}

// SetIconNode sets a custom leading icon node.
func (p *Popconfirm) SetIconNode(n core.Node) {
	if p == nil {
		return
	}
	p.IconNode = n
	p.ShowIcon = true
	p.rebuildContent()
}

// SetShowIcon toggles the leading icon.
func (p *Popconfirm) SetShowIcon(show bool) {
	if p == nil {
		return
	}
	p.ShowIcon = show
	p.rebuildContent()
}

// SetOkText sets the OK button label.
func (p *Popconfirm) SetOkText(s string) {
	if p == nil {
		return
	}
	if s == "" {
		s = DefaultPopconfirmOkText
	}
	p.OkText = s
	if p.okBtn != nil {
		p.okBtn.SetLabel(s)
	} else {
		p.rebuildContent()
	}
}

// SetCancelText sets the Cancel button label.
func (p *Popconfirm) SetCancelText(s string) {
	if p == nil {
		return
	}
	if s == "" {
		s = DefaultPopconfirmCancelText
	}
	p.CancelText = s
	if p.cancelBtn != nil {
		p.cancelBtn.SetLabel(s)
	} else {
		p.rebuildContent()
	}
}

// SetOkType sets the OK button type (default primary).
func (p *Popconfirm) SetOkType(t ButtonType) {
	if p == nil {
		return
	}
	p.OkType = t
	if p.okBtn != nil {
		p.okBtn.SetType(t)
	} else {
		p.rebuildContent()
	}
}

// SetShowCancel toggles the cancel button (default true).
func (p *Popconfirm) SetShowCancel(show bool) {
	if p == nil {
		return
	}
	p.ShowCancel = show
	p.rebuildContent()
}

// SetConfirmLoading toggles OK loading (antd okButtonProps.loading).
// While true, OK shows spinner; does not by itself close the popup.
func (p *Popconfirm) SetConfirmLoading(v bool) {
	if p == nil {
		return
	}
	p.ConfirmLoading = v
	if p.okBtn != nil {
		p.okBtn.SetLoading(v || p.asyncPending)
	}
}

// SetOnConfirm sets the sync confirm callback.
func (p *Popconfirm) SetOnConfirm(fn func()) {
	if p == nil {
		return
	}
	p.OnConfirm = fn
}

// SetOnConfirmAsync sets the Promise-style confirm path.
func (p *Popconfirm) SetOnConfirmAsync(fn func(finish func())) {
	if p == nil {
		return
	}
	p.OnConfirmAsync = fn
}

// SetOnCancel sets the cancel callback.
func (p *Popconfirm) SetOnCancel(fn func()) {
	if p == nil {
		return
	}
	p.OnCancel = fn
}

// SetOnPopupClick sets the popup body click hook.
func (p *Popconfirm) SetOnPopupClick(fn func()) {
	if p == nil {
		return
	}
	p.OnPopupClick = fn
}

// SetDisabled disables the popconfirm (cannot open).
func (p *Popconfirm) SetDisabled(v bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetDisabled(v)
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (p *Popconfirm) SetOpen(open bool) {
	if p == nil || p.Popover == nil {
		return
	}
	if !open {
		p.asyncPending = false
		p.ConfirmLoading = false
		if p.okBtn != nil {
			p.okBtn.SetLoading(false)
		}
	}
	p.Popover.SetOpen(open)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (p *Popconfirm) SetDefaultOpen(open bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetDefaultOpen(open)
}

// SetOnOpenChange sets the open-change callback (delegates to Popover).
func (p *Popconfirm) SetOnOpenChange(fn func(open bool)) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetOnOpenChange(fn)
}

// SetTriggerLabel sets the default trigger button text.
func (p *Popconfirm) SetTriggerLabel(label string) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetTriggerLabel(label)
}

// SetTriggerNode sets a custom trigger node.
func (p *Popconfirm) SetTriggerNode(n core.Node) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetTriggerNode(n)
}

// SetPlacement sets popup placement.
func (p *Popconfirm) SetPlacement(pl PopoverPlacement) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetPlacement(pl)
}

// SetTrigger sets a single open trigger mode.
func (p *Popconfirm) SetTrigger(mode PopoverTrigger) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetTrigger(mode)
}

// SetTriggerModes sets open triggers.
func (p *Popconfirm) SetTriggerModes(modes ...PopoverTrigger) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetTriggerModes(modes...)
}

// SetArrow toggles the arrow.
func (p *Popconfirm) SetArrow(show bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetArrow(show)
}

// SetArrowConfig sets arrow show + pointAtCenter.
func (p *Popconfirm) SetArrowConfig(show, pointAtCenter bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetArrowConfig(show, pointAtCenter)
}

// SetAutoAdjustOverflow enables flip/shift (default true).
func (p *Popconfirm) SetAutoAdjustOverflow(v bool) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetAutoAdjustOverflow(v)
}

// SetZIndex sets Portal.ZOrder.
func (p *Popconfirm) SetZIndex(z int) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetZIndex(z)
}

// SetTheme sets an explicit theme override.
func (p *Popconfirm) SetTheme(th *core.Theme) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetTheme(th)
	p.rebuildContent()
}

// SetFace sets the font face for labels and buttons.
func (p *Popconfirm) SetFace(face text.Face) {
	if p == nil {
		return
	}
	if p.Popover != nil {
		p.Popover.Face = face
	}
	if p.titleLab != nil {
		p.titleLab.Face = face
		p.titleLab.MarkNeedsLayout()
		p.titleLab.MarkNeedsPaint()
	}
	if p.descLab != nil {
		p.descLab.Face = face
		p.descLab.MarkNeedsLayout()
		p.descLab.MarkNeedsPaint()
	}
	if p.okBtn != nil {
		p.okBtn.SetFace(face)
	}
	if p.cancelBtn != nil {
		p.cancelBtn.SetFace(face)
	}
}

// SetAriaLabel sets the accessible name on the trigger.
func (p *Popconfirm) SetAriaLabel(name string) {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.SetAriaLabel(name)
}

// AttachTicker registers OK loading spinner when active.
func (p *Popconfirm) AttachTicker(t *core.Tree) {
	if p == nil {
		return
	}
	if p.okBtn != nil {
		p.okBtn.AttachTicker(t)
	}
	if p.Popover != nil {
		p.Popover.AttachTicker(t)
	}
}

// IsOpen reports whether the card is visible.
func (p *Popconfirm) IsOpen() bool {
	return p != nil && p.Popover != nil && p.Popover.IsOpen()
}

func (p *Popconfirm) theme() *core.Theme {
	if p == nil || p.Popover == nil {
		return DefaultTheme()
	}
	return p.Popover.theme()
}

func (p *Popconfirm) rebuildContent() {
	if p == nil || p.Popover == nil {
		return
	}
	th := p.theme()
	fs := th.SizeOr(core.TokenFontSize, DefaultPopoverFontSize)
	face := p.Popover.Face
	// Popconfirm spacing follows antd marginXS=8 / marginXXS=4 (component style).
	// Note: kit TokenMarginXS is 4 (≈ antd XXS); do not use it for message/button gaps.
	gapIcon := DefaultPopconfirmIconGap
	msgBottom := DefaultPopconfirmMessageMarginBottom
	btnGap := DefaultPopconfirmButtonGap
	descTop := DefaultPopconfirmDescMarginTop

	// ── message: icon + text column ──────────────────────────────
	msgRow := primitive.Row()
	msgRow.Gap = gapIcon
	msgRow.CrossAlign = core.CrossStart

	if p.ShowIcon {
		p.iconHost = p.resolveIcon(th, fs)
		if p.iconHost != nil {
			msgRow.AddChild(p.iconHost)
		}
	} else {
		p.iconHost = nil
	}

	textCol := primitive.Column()
	textCol.CrossAlign = core.CrossStart
	textCol.Gap = 0

	var titleN core.Node
	if p.TitleNode != nil {
		titleN = p.TitleNode
		p.titleLab = nil
	} else if p.Title != "" {
		p.titleLab = primitive.NewText(p.Title)
		p.titleLab.FontSize = fs
		p.titleLab.Face = face
		p.titleLab.Color = th.Color(core.TokenColorText)
		titleN = p.titleLab
	} else {
		p.titleLab = nil
	}
	if titleN != nil {
		textCol.AddChild(titleN)
	}

	var descN core.Node
	if p.DescriptionNode != nil {
		descN = p.DescriptionNode
		p.descLab = nil
	} else if p.Description != "" {
		p.descLab = primitive.NewText(p.Description)
		p.descLab.FontSize = fs
		p.descLab.Face = face
		p.descLab.Color = th.Color(core.TokenColorText)
		descN = p.descLab
	} else {
		p.descLab = nil
	}
	if descN != nil {
		if titleN != nil {
			wrap := primitive.NewDecorated(descN)
			wrap.Padding = primitive.EdgeInsets{Top: descTop}
			textCol.AddChild(wrap)
		} else {
			textCol.AddChild(descN)
		}
	}
	msgRow.AddChild(textCol)

	// ── buttons ──────────────────────────────────────────────────
	okLabel := p.OkText
	if okLabel == "" {
		okLabel = DefaultPopconfirmOkText
	}
	cancelLabel := p.CancelText
	if cancelLabel == "" {
		cancelLabel = DefaultPopconfirmCancelText
	}

	p.okBtn = NewButton(okLabel)
	p.okBtn.SetType(p.OkType)
	p.okBtn.SetSize(ButtonSmall)
	p.okBtn.SetFace(face)
	p.okBtn.Theme = p.Popover.Theme
	p.okBtn.SetLoading(p.ConfirmLoading || p.asyncPending)
	p.okBtn.SetOnClick(func() { p.handleConfirm() })

	btnRow := primitive.Row()
	btnRow.Gap = btnGap
	btnRow.MainAlign = core.MainEnd
	btnRow.CrossAlign = core.CrossCenter

	if p.ShowCancel {
		p.cancelBtn = NewButton(cancelLabel)
		p.cancelBtn.SetType(ButtonDefault)
		p.cancelBtn.SetSize(ButtonSmall)
		p.cancelBtn.SetFace(face)
		p.cancelBtn.Theme = p.Popover.Theme
		p.cancelBtn.SetOnClick(func() { p.handleCancel() })
		btnRow.AddChild(p.cancelBtn.Node())
	} else {
		p.cancelBtn = nil
	}
	btnRow.AddChild(p.okBtn.Node())

	// ── body column ──────────────────────────────────────────────
	body := primitive.Column()
	body.CrossAlign = core.CrossStretch
	body.Gap = msgBottom
	msgHost := newPopconfirmClickHost(msgRow, p)
	body.AddChild(msgHost)
	body.AddChild(btnRow)

	// All chrome lives in content (no Popover title row).
	p.Popover.Title = ""
	p.Popover.TitleNode = nil
	p.Popover.SetContentNode(body)

	// a11y: dialog accessible name from product title.
	name := p.Title
	if name == "" && p.TitleNode == nil {
		name = "popconfirm"
	} else if name == "" {
		name = "popconfirm"
	}
	if p.Popover.scope != nil {
		p.Popover.scope.Base().Role = "dialog"
		p.Popover.scope.Base().Label = name
	}
	if p.Popover.panel != nil {
		p.Popover.panel.Base().Role = "dialog"
		p.Popover.panel.Base().Label = name
	}
}

func (p *Popconfirm) resolveIcon(th *core.Theme, fs float64) core.Node {
	if p.IconNode != nil {
		return p.IconNode
	}
	name := p.Icon
	if name == "" {
		name = DefaultPopconfirmIconName
	}
	ic := NewIcon(name)
	if ic != nil {
		ic.SetSize(fs)
		warn := th.Color(core.TokenColorWarning)
		if warn.A > 0 {
			ic.SetColor(warn)
		}
		if n := ic.Node(); n != nil {
			return n
		}
	}
	glyph := primitive.NewText("!")
	glyph.FontSize = fs
	if p.Popover != nil {
		glyph.Face = p.Popover.Face
	}
	glyph.Color = th.Color(core.TokenColorWarning)
	return glyph
}

func (p *Popconfirm) handleConfirm() {
	if p == nil {
		return
	}
	if p.asyncPending {
		return
	}
	if p.OnConfirmAsync != nil {
		p.asyncPending = true
		if p.okBtn != nil {
			p.okBtn.SetLoading(true)
		}
		p.OnConfirmAsync(func() {
			p.asyncPending = false
			p.ConfirmLoading = false
			if p.okBtn != nil {
				p.okBtn.SetLoading(false)
			}
			p.requestClose()
		})
		return
	}
	if p.OnConfirm != nil {
		p.OnConfirm()
	}
	// Controlled async demo: host may SetConfirmLoading(true) inside OnConfirm and keep open.
	if p.ConfirmLoading || p.asyncPending {
		return
	}
	p.requestClose()
}

func (p *Popconfirm) handleCancel() {
	if p == nil {
		return
	}
	p.asyncPending = false
	p.ConfirmLoading = false
	if p.okBtn != nil {
		p.okBtn.SetLoading(false)
	}
	if p.OnCancel != nil {
		p.OnCancel()
	}
	p.requestClose()
}

// requestClose closes uncontrolled or notifies controlled parent (Popover.requestOpen).
func (p *Popconfirm) requestClose() {
	if p == nil || p.Popover == nil {
		return
	}
	p.Popover.requestOpen(false)
}

// popconfirmClickHost wraps message area for OnPopupClick.
type popconfirmClickHost struct {
	core.NodeBase
	child core.Node
	pc    *Popconfirm
}

func newPopconfirmClickHost(child core.Node, pc *Popconfirm) *popconfirmClickHost {
	h := &popconfirmClickHost{child: child, pc: pc}
	h.Init(h)
	h.Hit = core.HitDefer
	if child != nil {
		h.AddChild(child)
	}
	return h
}

func (h *popconfirmClickHost) TypeID() string { return "kit.popconfirmClickHost" }

func (h *popconfirmClickHost) Layout(c core.Constraints) core.Size {
	if h.child == nil {
		out := c.Tighten(core.Size{})
		h.SetSize(out)
		return out
	}
	sz := h.child.Layout(c)
	h.child.Base().SetOffset(core.Point{})
	out := c.Tighten(sz)
	h.SetSize(out)
	return out
}

func (h *popconfirmClickHost) Paint(pc *core.PaintContext) { h.DefaultPaintChildren(pc) }

func (h *popconfirmClickHost) HitTest(p core.Point) core.Node { return h.DefaultHitTest(p) }

func (h *popconfirmClickHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || h.pc == nil || ev == nil {
		return
	}
	if ev.Type == core.PointerUp && ev.Button == core.ButtonLeft && h.pc.OnPopupClick != nil {
		h.pc.OnPopupClick()
	}
}
