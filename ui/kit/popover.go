package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Popover defaults — components/popover/style + docs/antd/popover.md §6.2.
const (
	DefaultPopoverGap               = 8.0
	DefaultPopoverInnerPadding      = 12.0
	DefaultPopoverTitleMinWidth     = 177.0
	DefaultPopoverTitleMarginBottom = 4.0
	DefaultPopoverArrowSize         = 8.0
	DefaultPopoverFontSize          = 14.0
	DefaultPopoverPortalZ           = 200
)

// PopoverTrigger is an open trigger mode (antd trigger[]).
type PopoverTrigger int

const (
	// PopoverTriggerHover is the antd default.
	PopoverTriggerHover PopoverTrigger = iota
	PopoverTriggerClick
	PopoverTriggerFocus
	PopoverTriggerContextMenu
)

// PopoverPlacement is antd placement (12-way). Default: Top.
type PopoverPlacement int

const (
	PopoverTop PopoverPlacement = iota // default (antd)
	PopoverTopLeft
	PopoverTopRight
	PopoverBottom
	PopoverBottomLeft
	PopoverBottomRight
	PopoverLeft
	PopoverLeftTop
	PopoverLeftBottom
	PopoverRight
	PopoverRightTop
	PopoverRightBottom
)

// Popover is Ant Design Popover: trigger + anchored title/content card.
//
//	Column
//	  ├─ pointer host (contextMenu) / trigger shell (Pressable)
//	  └─ AnchoredPopup
//	       └─ FocusScope (Esc)
//	            └─ panel (+ optional arrow) / title + content
//
// Product contract: docs/antd/popover.md §6 (P0 DoD).
type Popover struct {
	Wrap  *primitive.Flex
	shell *primitive.Pressable
	popup *primitive.AnchoredPopup
	scope *primitive.FocusScope
	panel *primitive.Decorated
	arrow *primitive.Canvas
	btn   *Button // default trigger when no custom node

	titleLab *primitive.Text
	bodyCol  *primitive.Flex
	contentN core.Node

	// Title is plain-text title when TitleNode is nil.
	Title string
	// TitleNode optional custom title node (overrides Title).
	TitleNode core.Node
	// Content is plain-text content when ContentNode is nil.
	Content string
	// ContentNode optional custom content (overrides Content string).
	// Kept for Popconfirm / advanced hosts that walk the content tree.
	ContentNode core.Node
	// TriggerLabel is the default trigger text when TriggerNode is nil.
	TriggerLabel string
	// TriggerNode optional custom trigger (overrides label button).
	TriggerNode core.Node

	// Placement default Top (antd).
	Placement PopoverPlacement
	// Triggers empty → [hover] (antd default).
	Triggers []PopoverTrigger
	// Arrow show / pointAtCenter (antd default true).
	Arrow bool
	// ArrowPointAtCenter when Arrow and true (antd arrow={{ pointAtCenter }}).
	ArrowPointAtCenter bool
	// AutoAdjustOverflow default true (AnchoredPopup flip/shift).
	AutoAdjustOverflow bool
	// Disabled blocks open.
	Disabled bool
	// Open is the current visibility.
	Open bool
	// ZIndex overrides Portal.ZOrder when > 0.
	ZIndex int
	// PanelBackground optional panel fill override (P1 color approx).
	PanelBackground render.RGBA
	// hasPanelBg tracks whether PanelBackground was set (zero RGBA is valid transparent).
	hasPanelBg bool

	Viewport core.Size
	Face     text.Face
	Theme    *core.Theme
	// AriaLabel accessible name for the trigger.
	AriaLabel string

	// OnOpenChange fires when open intent/state changes.
	OnOpenChange func(open bool)

	// openControlled: SetOpen was used (antd open prop).
	openControlled bool
	// defaultOpen applied once when not controlled.
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool

	// hover close grace (leave trigger → panel without flicker).
	hoverClosePending bool
	// right-click host sits above shell for contextMenu trigger.
	ctxHost *popoverPointerHost
}

// NewPopover creates a Popover with optional default trigger label.
// Defaults (§6.10): trigger=[hover], placement=top, arrow=true, closed, uncontrolled.
func NewPopover(triggerLabel string) *Popover {
	p := &Popover{
		TriggerLabel:       triggerLabel,
		Placement:          PopoverTop,
		Arrow:              true,
		AutoAdjustOverflow: true,
	}
	p.rebuild()
	return p
}

// Node returns the composition root (trigger + popup host).
func (p *Popover) Node() core.Node {
	if p == nil {
		return nil
	}
	if p.Wrap == nil {
		p.rebuild()
	}
	return p.Wrap
}

// Popup returns the anchored popup (tests / advanced hosts).
func (p *Popover) Popup() *primitive.AnchoredPopup {
	if p == nil {
		return nil
	}
	return p.popup
}

// IsOpen reports whether the card is visible.
func (p *Popover) IsOpen() bool {
	return p != nil && p.Open
}

// Panel returns the card chrome (tests).
func (p *Popover) Panel() *primitive.Decorated {
	if p == nil {
		return nil
	}
	return p.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (p *Popover) TriggerShell() *primitive.Pressable {
	if p == nil {
		return nil
	}
	return p.shell
}

// ContentRoot returns the resolved content node (tests / Popconfirm).
func (p *Popover) ContentRoot() core.Node {
	if p == nil {
		return nil
	}
	return p.contentN
}

// SetTitle sets plain-text title (clears TitleNode).
func (p *Popover) SetTitle(title string) {
	if p == nil {
		return
	}
	p.Title = title
	p.TitleNode = nil
	p.rebuildPanel()
	if p.Open {
		p.measurePanel()
		p.syncPopupGeometry()
	}
}

// SetTitleNode sets a custom title node (nil falls back to Title string).
func (p *Popover) SetTitleNode(n core.Node) {
	if p == nil {
		return
	}
	p.TitleNode = n
	p.rebuildPanel()
	if p.Open {
		p.measurePanel()
		p.syncPopupGeometry()
	}
}

// SetContent sets plain-text content (clears ContentNode).
func (p *Popover) SetContent(text string) {
	if p == nil {
		return
	}
	p.Content = text
	p.ContentNode = nil
	p.rebuildPanel()
	if p.Open {
		p.measurePanel()
		p.syncPopupGeometry()
	}
}

// SetContentNode sets a custom content node (nil falls back to Content string).
func (p *Popover) SetContentNode(n core.Node) {
	if p == nil {
		return
	}
	p.ContentNode = n
	p.rebuildPanel()
	if p.Open {
		p.measurePanel()
		p.syncPopupGeometry()
	}
}

// SetTriggerLabel sets the default button trigger text.
func (p *Popover) SetTriggerLabel(label string) {
	if p == nil {
		return
	}
	p.TriggerLabel = label
	if p.TriggerNode == nil {
		p.rebuild()
	} else if p.btn != nil {
		p.btn.SetLabel(label)
	}
	p.applyA11y()
}

// SetTriggerNode sets a custom trigger node (nil restores label button).
func (p *Popover) SetTriggerNode(n core.Node) {
	if p == nil {
		return
	}
	p.TriggerNode = n
	p.rebuild()
}

// SetTriggerModes sets open triggers (empty → hover default).
func (p *Popover) SetTriggerModes(modes ...PopoverTrigger) {
	if p == nil {
		return
	}
	p.Triggers = append([]PopoverTrigger(nil), modes...)
	p.wireTrigger()
}

// SetTrigger is a single-mode convenience (overwrites Triggers).
func (p *Popover) SetTrigger(mode PopoverTrigger) {
	p.SetTriggerModes(mode)
}

// SetPlacement sets popup placement.
func (p *Popover) SetPlacement(pl PopoverPlacement) {
	if p == nil {
		return
	}
	p.Placement = pl
	if p.popup != nil {
		p.popup.Placement = mapPopoverPlacement(pl, p.ArrowPointAtCenter)
		if p.Open {
			p.syncPopupGeometry()
		}
	}
	// Arrow order may depend on placement.
	if p.Arrow {
		p.rebuildPanel()
	}
}

// SetArrow toggles the arrow indicator.
func (p *Popover) SetArrow(show bool) {
	if p == nil {
		return
	}
	p.Arrow = show
	p.rebuildPanel()
}

// SetArrowConfig sets arrow show + pointAtCenter (antd arrow object).
func (p *Popover) SetArrowConfig(show, pointAtCenter bool) {
	if p == nil {
		return
	}
	p.Arrow = show
	p.ArrowPointAtCenter = pointAtCenter
	if p.popup != nil {
		p.popup.Placement = mapPopoverPlacement(p.Placement, p.ArrowPointAtCenter)
	}
	p.rebuildPanel()
}

// SetAutoAdjustOverflow enables flip/shift (default true). When false, viewport is zeroed for placement.
func (p *Popover) SetAutoAdjustOverflow(v bool) {
	if p == nil {
		return
	}
	p.AutoAdjustOverflow = v
	p.applyViewportToPopup()
}

// SetDisabled disables the popover (cannot open).
func (p *Popover) SetDisabled(v bool) {
	if p == nil {
		return
	}
	p.Disabled = v
	if p.shell != nil {
		p.shell.SetDisabled(v)
	}
	if p.btn != nil {
		p.btn.SetDisabled(v)
	}
	if v && p.Open {
		p.applyOpen(false, false)
	}
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (p *Popover) SetOpen(open bool) {
	if p == nil {
		return
	}
	p.openControlled = true
	p.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (p *Popover) SetDefaultOpen(open bool) {
	if p == nil || p.openControlled {
		return
	}
	p.defaultOpen = open
	p.defaultOpenSet = true
	if !p.appliedDefault {
		p.appliedDefault = true
		p.applyOpen(open, false)
	}
}

// SetOnOpenChange sets the open-change callback.
func (p *Popover) SetOnOpenChange(fn func(open bool)) {
	if p == nil {
		return
	}
	p.OnOpenChange = fn
}

// SetZIndex sets Portal.ZOrder (0 keeps default).
func (p *Popover) SetZIndex(z int) {
	if p == nil {
		return
	}
	p.ZIndex = z
	if p.popup != nil && p.popup.Portal != nil {
		if z > 0 {
			p.popup.Portal.ZOrder = z
		} else {
			p.popup.Portal.ZOrder = DefaultPopoverPortalZ
		}
	}
}

// SetPanelBackground overrides panel fill (P1 color approx / style-class demo).
func (p *Popover) SetPanelBackground(c render.RGBA) {
	if p == nil {
		return
	}
	p.PanelBackground = c
	p.hasPanelBg = true
	if p.panel != nil {
		p.panel.Background = c
		p.panel.MarkNeedsPaint()
	}
}

// SetTheme sets an explicit theme override.
func (p *Popover) SetTheme(th *core.Theme) {
	if p == nil {
		return
	}
	p.Theme = th
	p.rebuild()
}

// SetFace sets the font face for labels.
func (p *Popover) SetFace(face text.Face) {
	if p == nil {
		return
	}
	p.Face = face
	p.rebuild()
}

// SetAriaLabel sets the accessible name on the trigger.
func (p *Popover) SetAriaLabel(name string) {
	if p == nil {
		return
	}
	p.AriaLabel = name
	p.applyA11y()
}

// AttachTicker is a no-op placeholder for gallery tickers slices.
func (p *Popover) AttachTicker(t *core.Tree) {
	_ = t
}

// Tick implements core.Ticker — resolves deferred hover leave close.
func (p *Popover) Tick(dt float64) bool {
	_ = dt
	if p == nil || !p.hoverClosePending {
		return false
	}
	p.hoverClosePending = false
	if !p.hasTrigger(PopoverTriggerHover) {
		return false
	}
	if p.pointerOverPopover() {
		return false
	}
	p.requestOpen(false)
	return false
}

// Sync repositions while open.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry.
func (p *Popover) Sync() {
	if p != nil && p.Open {
		p.syncPopupGeometry()
		if p.popup != nil {
			p.popup.SetOpen(true)
		}
	}
}

func (p *Popover) theme() *core.Theme {
	var n core.Node
	if p.Wrap != nil {
		n = p.Wrap
	}
	return themeOf(p.Theme, n)
}

func (p *Popover) hasTrigger(mode PopoverTrigger) bool {
	if p == nil {
		return false
	}
	modes := p.Triggers
	if len(modes) == 0 {
		modes = []PopoverTrigger{PopoverTriggerHover}
	}
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

func (p *Popover) rebuild() {
	th := p.theme()
	wasOpen := p.Open

	// Default trigger: primary-ish button (antd basic uses primary Button).
	var trigger core.Node
	if p.TriggerNode != nil {
		trigger = p.TriggerNode
		p.btn = nil
	} else {
		label := p.TriggerLabel
		if label == "" {
			label = "Hover me"
		}
		p.btn = NewButton(label)
		p.btn.SetType(ButtonPrimary)
		p.btn.SetFace(p.Face)
		p.btn.Theme = p.Theme
		if p.Disabled {
			p.btn.SetDisabled(true)
		}
		trigger = p.btn.Node()
	}

	p.shell = primitive.NewPressable(trigger)
	p.shell.Focusable = true
	p.shell.FocusRingRadius = th.SizeOr(core.TokenBorderRadius, 6)
	p.shell.SetDisabled(p.Disabled)
	p.applyA11y()

	p.ctxHost = newPopoverPointerHost(p.shell, p)

	p.rebuildPanel()

	p.popup = primitive.NewAnchoredPopup(p.scope)
	p.popup.Placement = mapPopoverPlacement(p.Placement, p.ArrowPointAtCenter)
	p.popup.Gap = DefaultPopoverGap
	p.popup.Portal.ID = ""
	p.popup.DismissOnOutside = true
	if p.ZIndex > 0 {
		p.popup.Portal.ZOrder = p.ZIndex
	} else {
		p.popup.Portal.ZOrder = DefaultPopoverPortalZ
	}
	p.popup.OnDismiss = func() {
		// Outside pointer: user intent (respect controlled open).
		p.requestOpen(false)
		// Controlled: popup was closed by AnchoredPopup — re-open if parent still wants open.
		if p.openControlled && p.Open {
			p.popup.SetOpen(true)
		}
	}
	p.applyViewportToPopup()

	if p.Wrap == nil {
		p.Wrap = primitive.Column(p.ctxHost, p.popup)
	} else {
		p.Wrap.ClearChildren()
		p.Wrap.AddChild(p.ctxHost)
		p.Wrap.AddChild(p.popup)
	}
	p.Wrap.CrossAlign = core.CrossStart
	p.Wrap.SetThemeHook(func(*core.Theme) { p.rebuild() })

	p.wireTrigger()

	// Restore open after rebuild without re-notifying.
	if wasOpen || (p.defaultOpenSet && !p.openControlled && p.defaultOpen && !p.appliedDefault) {
		p.appliedDefault = true
		p.applyOpen(true, false)
	}

	p.Wrap.MarkNeedsLayout()
	p.Wrap.MarkNeedsPaint()
}

func (p *Popover) applyA11y() {
	if p.shell == nil {
		return
	}
	name := p.AriaLabel
	if name == "" {
		name = p.TriggerLabel
	}
	if name == "" {
		name = "popover"
	}
	p.shell.Base().Role = "button"
	p.shell.Base().Label = name
	panelName := p.Title
	if panelName == "" {
		panelName = name
	}
	if p.scope != nil {
		p.scope.Base().Role = "dialog"
		p.scope.Base().Label = panelName
	}
	if p.panel != nil {
		p.panel.Base().Role = "dialog"
		p.panel.Base().Label = panelName
	}
}

func (p *Popover) wireTrigger() {
	if p.shell == nil {
		return
	}
	p.shell.Click = func() {
		if p.Disabled {
			return
		}
		if p.hasTrigger(PopoverTriggerClick) {
			p.requestOpen(!p.Open)
		}
	}
	p.shell.OnStateChange = func() {
		if p.Disabled {
			return
		}
		// Hover
		if p.hasTrigger(PopoverTriggerHover) {
			if p.shell.State.Hovered {
				p.hoverClosePending = false
				p.requestOpen(true)
			} else if !p.hasTrigger(PopoverTriggerFocus) || !p.shell.State.Focused {
				// Leave trigger: defer close so move-to-panel can cancel.
				p.hoverClosePending = true
				if t := p.shell.Tree(); t != nil {
					t.AddTicker(p)
				} else if !p.pointerOverPopover() {
					p.requestOpen(false)
				}
			}
		}
		// Focus
		if p.hasTrigger(PopoverTriggerFocus) {
			if p.shell.State.Focused {
				p.hoverClosePending = false
				p.requestOpen(true)
			} else if !p.hasTrigger(PopoverTriggerHover) || !p.shell.State.Hovered {
				p.requestOpen(false)
			}
		}
	}
}

func (p *Popover) rebuildPanel() {
	th := p.theme()
	fs := th.SizeOr(core.TokenFontSize, DefaultPopoverFontSize)

	// Resolve title
	var titleN core.Node
	if p.TitleNode != nil {
		titleN = p.TitleNode
		p.titleLab = nil
	} else if p.Title != "" {
		p.titleLab = primitive.NewText(p.Title)
		p.titleLab.FontSize = fs
		p.titleLab.Face = p.Face
		p.titleLab.Color = th.Color(core.TokenColorText)
		titleN = p.titleLab
	} else {
		p.titleLab = nil
	}

	// Resolve content
	var content core.Node
	if p.ContentNode != nil {
		content = p.ContentNode
	} else if p.Content != "" {
		tx := primitive.NewText(p.Content)
		tx.FontSize = fs
		tx.Face = p.Face
		tx.Color = th.Color(core.TokenColorText)
		content = tx
	} else {
		// Empty placeholder keeps layout stable for measure.
		tx := primitive.NewText("")
		tx.FontSize = fs
		content = tx
	}
	p.contentN = content

	p.bodyCol = primitive.Column()
	p.bodyCol.CrossAlign = core.CrossStart
	p.bodyCol.Gap = 0
	if titleN != nil {
		// Title min width + bottom margin (antd titleMinWidth / titleMarginBottom).
		titleWrap := primitive.NewDecorated(titleN)
		titleWrap.MinWidth = DefaultPopoverTitleMinWidth
		titleWrap.Padding = primitive.Symmetric(0, 0)
		// margin bottom via outer gap
		p.bodyCol.AddChild(titleWrap)
		p.bodyCol.Gap = DefaultPopoverTitleMarginBottom
	}
	p.bodyCol.AddChild(content)

	inner := core.Node(p.bodyCol)
	if p.Arrow {
		sz := DefaultPopoverArrowSize
		border := th.Color(core.TokenColorBorder)
		bg := th.Color(core.TokenColorBgContainer)
		if p.hasPanelBg {
			bg = p.PanelBackground
		}
		p.arrow = primitive.NewCanvas(sz, sz/2+1, func(pc *core.PaintContext, size core.Size) {
			if pc == nil {
				return
			}
			mid := size.Width / 2
			pc.StrokeLocalLine(0, size.Height-1, mid, 1, 1, border)
			pc.StrokeLocalLine(mid, 1, size.Width, size.Height-1, 1, border)
			pc.FillLocalRect(mid-1, 2, 2, size.Height-2, bg)
		})
		col := primitive.Column()
		col.CrossAlign = core.CrossCenter
		col.Gap = 0
		switch p.Placement {
		case PopoverBottom, PopoverBottomLeft, PopoverBottomRight:
			col.AddChild(p.arrow)
			col.AddChild(p.bodyCol)
		case PopoverTop, PopoverTopLeft, PopoverTopRight:
			col.AddChild(p.bodyCol)
			col.AddChild(p.arrow)
		default:
			// left/right: keep body only for P0 arrow geometry (caret still drawn above for bottom-ish).
			col.AddChild(p.arrow)
			col.AddChild(p.bodyCol)
		}
		inner = col
	} else {
		p.arrow = nil
	}

	p.panel = primitive.NewDecorated(inner)
	p.panel.Padding = primitive.All(DefaultPopoverInnerPadding)
	p.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	if p.hasPanelBg {
		p.panel.Background = p.PanelBackground
	} else {
		p.panel.Background = th.Color(core.TokenColorBgContainer)
	}
	p.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	p.panel.BorderColor = th.Color(core.TokenColorBorder)
	if titleN != nil {
		p.panel.MinWidth = DefaultPopoverTitleMinWidth
	}
	p.panel.Base().Role = "dialog"

	if p.scope == nil {
		p.scope = primitive.NewFocusScope(p.panel)
	} else {
		p.scope.ClearChildren()
		p.scope.AddChild(p.panel)
	}
	p.scope.Active = p.Open
	p.scope.OnEscape = func() {
		p.requestOpen(false)
	}
	p.scope.Base().Role = "dialog"
	p.applyA11y()

	if p.popup != nil {
		p.popup.Content = p.scope
		if p.popup.Portal != nil {
			p.popup.Portal.Content = p.scope
		}
	}
}

func (p *Popover) pointerOverPopover() bool {
	if p.shell != nil && p.shell.State.Hovered {
		return true
	}
	// Panel itself may not track hover via Pressable; keep shell-only grace.
	// If content has pressables that set hover while open, leave stays open until leave.
	return false
}

// requestOpen is the user-intent path (trigger / esc / outside).
// Controlled mode only fires OnOpenChange; visibility waits for SetOpen.
func (p *Popover) requestOpen(open bool) {
	if p == nil {
		return
	}
	if p.Disabled && open {
		return
	}
	if p.openControlled {
		if p.Open != open && p.OnOpenChange != nil {
			p.OnOpenChange(open)
		}
		return
	}
	if p.Open == open {
		return
	}
	p.applyOpen(open, true)
}

func (p *Popover) applyOpen(open bool, notify bool) {
	if p == nil {
		return
	}
	if p.Disabled && open {
		return
	}
	prev := p.Open
	p.Open = open
	if p.scope != nil {
		p.scope.Active = open
		if open {
			p.scope.OnEscape = func() {
				p.requestOpen(false)
			}
		}
	}
	if p.popup != nil {
		if open {
			p.measurePanel()
			p.syncPopupGeometry()
		}
		p.popup.SetOpen(open)
	}
	if notify && prev != open && p.OnOpenChange != nil {
		p.OnOpenChange(open)
	}
	if p.Wrap != nil {
		p.Wrap.MarkNeedsLayout()
		p.Wrap.MarkNeedsPaint()
	}
}

func (p *Popover) measurePanel() {
	if p.panel != nil {
		_ = p.panel.Layout(core.Loose(480, 800))
	} else if p.scope != nil {
		_ = p.scope.Layout(core.Loose(480, 800))
	}
}

func (p *Popover) syncPopupGeometry() {
	if p.popup == nil {
		return
	}
	var anchor core.Node = p.shell
	if p.ctxHost != nil {
		anchor = p.ctxHost
	}
	p.popup.UpdateAnchorFromNode(anchor)
	p.applyViewportToPopup()
}

func (p *Popover) applyViewportToPopup() {
	if p.popup == nil {
		return
	}
	if !p.AutoAdjustOverflow {
		p.popup.Viewport = core.Size{}
		return
	}
	if p.Viewport.Width > 0 {
		p.popup.Viewport = p.Viewport
	}
}

func mapPopoverPlacement(pl PopoverPlacement, pointAtCenter bool) primitive.Placement {
	if pointAtCenter {
		switch pl {
		case PopoverTopLeft, PopoverTopRight:
			return primitive.PlaceTop
		case PopoverBottomLeft, PopoverBottomRight:
			return primitive.PlaceBottom
		case PopoverLeftTop, PopoverLeftBottom:
			return primitive.PlaceLeft
		case PopoverRightTop, PopoverRightBottom:
			return primitive.PlaceRight
		}
	}
	switch pl {
	case PopoverTop:
		return primitive.PlaceTop
	case PopoverTopLeft:
		return primitive.PlaceTopStart
	case PopoverTopRight:
		return primitive.PlaceTopEnd
	case PopoverBottom:
		return primitive.PlaceBottom
	case PopoverBottomLeft:
		return primitive.PlaceBottomStart
	case PopoverBottomRight:
		return primitive.PlaceBottomEnd
	case PopoverLeft:
		return primitive.PlaceLeft
	case PopoverLeftTop:
		return primitive.PlaceLeftStart
	case PopoverLeftBottom:
		return primitive.PlaceLeftEnd
	case PopoverRight:
		return primitive.PlaceRight
	case PopoverRightTop:
		return primitive.PlaceRightStart
	case PopoverRightBottom:
		return primitive.PlaceRightEnd
	default:
		return primitive.PlaceTop
	}
}

// popoverPointerHost wraps the trigger to capture context-menu (right button).
type popoverPointerHost struct {
	core.NodeBase
	child *primitive.Pressable
	po    *Popover
}

func newPopoverPointerHost(child *primitive.Pressable, po *Popover) *popoverPointerHost {
	h := &popoverPointerHost{child: child, po: po}
	h.Init(h)
	h.Hit = core.HitDefer
	if child != nil {
		h.AddChild(child)
	}
	return h
}

func (h *popoverPointerHost) TypeID() string { return "kit.popoverPointerHost" }

func (h *popoverPointerHost) Layout(c core.Constraints) core.Size {
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

func (h *popoverPointerHost) Paint(pc *core.PaintContext) { h.DefaultPaintChildren(pc) }

func (h *popoverPointerHost) HitTest(p core.Point) core.Node { return h.DefaultHitTest(p) }

func (h *popoverPointerHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || h.po == nil || ev == nil || h.po.Disabled {
		return
	}
	if !h.po.hasTrigger(PopoverTriggerContextMenu) {
		return
	}
	if ev.Type == core.PointerDown && ev.Button == core.ButtonRight {
		h.po.requestOpen(true)
		ev.Handled = true
	}
}
