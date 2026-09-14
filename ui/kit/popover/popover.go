// Package popover implements the Popover control (docs/antd/popover.md §6).
//
// P0 scope: title/content, trigger hover/click/focus/contextMenu multi,
// placement 12-way, arrow+pointAtCenter, open/defaultOpen/onOpenChange,
// autoAdjustOverflow, disabled, zIndex. Reuses ui/rendering for draw,
// ui/theme for tokens, ui/overlay for placement, ui/focus for keyboard.
package popover

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks (§6.2.1). Token-backed accessors prefer theme values.
const (
	DefaultPopoverFontSize         = 14.0
	DefaultPopoverInnerPadding     = 12.0
	DefaultPopoverTitleMinWidth    = 177.0
	DefaultPopoverTitleMarginBottom = 8.0
	DefaultPopoverGap              = 8.0
	DefaultPopoverArrowSize        = 8.0

	focusRingOutset = 1.5
	minTouchTarget  = 44.0
	triggerPadX     = 15.0
	triggerHeight   = 32.0
)

// PopoverTrigger is one trigger behavior (multi-select).
type PopoverTrigger string

const (
	TriggerHover       PopoverTrigger = "hover"
	TriggerClick       PopoverTrigger = "click"
	TriggerFocus       PopoverTrigger = "focus"
	TriggerContextMenu PopoverTrigger = "contextMenu"
)

// PopoverPlacement is one of the twelve anchor positions.
type PopoverPlacement string

const (
	Top         PopoverPlacement = "top"
	TopLeft     PopoverPlacement = "topLeft"
	TopRight    PopoverPlacement = "topRight"
	Bottom      PopoverPlacement = "bottom"
	BottomLeft  PopoverPlacement = "bottomLeft"
	BottomRight PopoverPlacement = "bottomRight"
	Left        PopoverPlacement = "left"
	LeftTop     PopoverPlacement = "leftTop"
	LeftBottom  PopoverPlacement = "leftBottom"
	Right       PopoverPlacement = "right"
	RightTop    PopoverPlacement = "rightTop"
	RightBottom PopoverPlacement = "rightBottom"
)

// AllPlacements lists the twelve positions in stable order.
var AllPlacements = []PopoverPlacement{
	Top, TopLeft, TopRight, Bottom, BottomLeft, BottomRight,
	Left, LeftTop, LeftBottom, Right, RightTop, RightBottom,
}

func (p PopoverPlacement) toOverlay() overlay.Placement {
	switch p {
	case Top:
		return overlay.Top
	case TopLeft:
		return overlay.TopLeft
	case TopRight:
		return overlay.TopRight
	case Bottom:
		return overlay.Bottom
	case BottomLeft:
		return overlay.BottomLeft
	case BottomRight:
		return overlay.BottomRight
	case Left:
		return overlay.Left
	case LeftTop:
		return overlay.LeftTop
	case LeftBottom:
		return overlay.LeftBottom
	case Right:
		return overlay.Right
	case RightTop:
		return overlay.RightTop
	case RightBottom:
		return overlay.RightBottom
	default:
		return overlay.Top
	}
}

func placementFromOverlay(p overlay.Placement) PopoverPlacement {
	switch p {
	case overlay.Top:
		return Top
	case overlay.TopLeft:
		return TopLeft
	case overlay.TopRight:
		return TopRight
	case overlay.Bottom:
		return Bottom
	case overlay.BottomLeft:
		return BottomLeft
	case overlay.BottomRight:
		return BottomRight
	case overlay.Left:
		return Left
	case overlay.LeftTop:
		return LeftTop
	case overlay.LeftBottom:
		return LeftBottom
	case overlay.Right:
		return Right
	case overlay.RightTop:
		return RightTop
	case overlay.RightBottom:
		return RightBottom
	default:
		return Top
	}
}

// Popover is the popover widget (§6.10).
type Popover struct {
	triggerLabel string
	title        string
	content      string
	titleNode    rendering.RenderObject
	contentNode  rendering.RenderObject
	triggerNode  rendering.RenderObject
	triggers     []PopoverTrigger
	placement    PopoverPlacement
	arrow        bool
	pointAtCenter bool
	autoAdjust   bool
	disabled     bool
	zIndex       int

	controlled     bool
	controlledOpen bool
	uncontrolledOpen bool
	onOpenChange   func(bool)

	// Theme 字段: exact tokens pin, provider selects source.
	Theme    *theme.Tokens
	provider *theme.Provider

	ariaLabel string
	faceName  string
	textFace  text.Face
	rtl       bool

	panelBackground    render.RGBA
	hasPanelBackground bool

	contentAction func()

	hovered         bool
	pressed         bool
	inBound         bool
	focused         bool
	hoverInTrigger  bool
	hoverInPanel    bool
	pendingHoverClose bool

	viewportW float64
	viewportH float64

	focusNode *focus.FocusNode

	root       *rendering.RenderBox
	triggerBox *rendering.RenderBox
	panelBox   *rendering.RenderBox

	lastTrigger rendering.Size
	lastPanel   rendering.Size

	overlayEntry *overlay.Entry
	overlayState *overlay.State
}

// NewPopover creates a closed popover with hover trigger.
func NewPopover(triggerLabel string) *Popover {
	p := &Popover{
		triggerLabel: triggerLabel,
		placement:    Top,
		arrow:        true,
		autoAdjust:   true,
		viewportW:    800,
		viewportH:    600,
	}
	p.root = rendering.NewRenderBox()
	p.root.SetRepaintBoundary(true)
	p.root.SetRelayoutBoundary(true)
	p.triggerBox = rendering.NewRenderBox()
	p.triggerBox.SetRepaintBoundary(true)
	p.panelBox = rendering.NewRenderBox()
	p.panelBox.SetRepaintBoundary(true)
	self := p
	p.triggerBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paintTrigger(pc, size)
	}
	p.panelBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paintPanel(pc, size)
	}
	p.root.AddChild(p.triggerBox)
	p.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return p
}

func (p *Popover) syncTrigger() {
	if p == nil || p.triggerBox == nil || p.root == nil {
		return
	}
	p.triggerBox.FixedWidth, p.triggerBox.FixedHeight = p.lastTrigger.Width, p.lastTrigger.Height
}

func (p *Popover) markPaint() {
	if p == nil {
		return
	}
	if p.triggerBox != nil {
		p.triggerBox.MarkNeedsPaint()
	}
	if p.panelBox != nil {
		p.panelBox.MarkNeedsPaint()
	}
	if p.root != nil {
		p.root.MarkNeedsPaint()
	}
}

func (p *Popover) markLayout() {
	if p == nil {
		return
	}
	if p.root != nil {
		p.root.MarkNeedsLayout()
	}
	if p.triggerBox != nil {
		p.triggerBox.MarkNeedsLayout()
	}
	if p.panelBox != nil {
		p.panelBox.MarkNeedsLayout()
	}
	p.markPaint()
}

// SetTitle sets the card title.
func (p *Popover) SetTitle(s string) *Popover {
	if p == nil || p.title == s {
		return p
	}
	p.title = s
	p.markLayout()
	return p
}

// Title returns the card title.
func (p *Popover) Title() string {
	if p == nil {
		return ""
	}
	return p.title
}

// SetTitleNode installs a custom title node.
func (p *Popover) SetTitleNode(n rendering.RenderObject) *Popover {
	if p == nil {
		return p
	}
	p.titleNode = n
	p.markLayout()
	return p
}

// TitleNode returns the custom title, if any.
func (p *Popover) TitleNode() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.titleNode
}

// SetContent sets the card content.
func (p *Popover) SetContent(s string) *Popover {
	if p == nil || p.content == s {
		return p
	}
	p.content = s
	p.markLayout()
	return p
}

// Content returns the card content.
func (p *Popover) Content() string {
	if p == nil {
		return ""
	}
	return p.content
}

// SetContentNode installs a custom content node.
func (p *Popover) SetContentNode(n rendering.RenderObject) *Popover {
	if p == nil {
		return p
	}
	p.contentNode = n
	p.markLayout()
	return p
}

// ContentNode returns the custom content, if any.
func (p *Popover) ContentNode() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.contentNode
}

// SetTriggerLabel sets the trigger text.
func (p *Popover) SetTriggerLabel(s string) *Popover {
	if p == nil || p.triggerLabel == s {
		return p
	}
	p.triggerLabel = s
	p.markLayout()
	return p
}

// TriggerLabel returns the trigger text.
func (p *Popover) TriggerLabel() string {
	if p == nil {
		return ""
	}
	return p.triggerLabel
}

// SetTriggerNode installs a custom trigger node.
func (p *Popover) SetTriggerNode(n rendering.RenderObject) *Popover {
	if p == nil {
		return p
	}
	p.triggerNode = n
	p.markLayout()
	return p
}

// TriggerNode returns the custom trigger, if any.
func (p *Popover) TriggerNode() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.triggerNode
}

// SetTriggerModes sets multi triggers.
func (p *Popover) SetTriggerModes(modes ...PopoverTrigger) *Popover {
	if p == nil {
		return p
	}
	cp := append([]PopoverTrigger(nil), modes...)
	for _, m := range cp {
		switch m {
		case TriggerHover, TriggerClick, TriggerFocus, TriggerContextMenu:
		default:
			p.triggers = nil
			p.markPaint()
			return p
		}
	}
	p.triggers = cp
	p.markPaint()
	return p
}

// SetTrigger is the single-mode convenience.
func (p *Popover) SetTrigger(m PopoverTrigger) *Popover {
	return p.SetTriggerModes(m)
}

// Triggers returns effective modes (empty means default hover).
func (p *Popover) Triggers() []PopoverTrigger {
	if p == nil || len(p.triggers) == 0 {
		return []PopoverTrigger{TriggerHover}
	}
	return append([]PopoverTrigger(nil), p.triggers...)
}

func (p *Popover) hasTrigger(m PopoverTrigger) bool {
	for _, t := range p.Triggers() {
		if t == m {
			return true
		}
	}
	return false
}

// SetPlacement sets the 12-way placement.
func (p *Popover) SetPlacement(v PopoverPlacement) *Popover {
	if p == nil {
		return p
	}
	switch v {
	case Top, TopLeft, TopRight, Bottom, BottomLeft, BottomRight,
		Left, LeftTop, LeftBottom, Right, RightTop, RightBottom:
	default:
		v = Top
	}
	if p.placement == v {
		return p
	}
	p.placement = v
	p.markPaint()
	return p
}

// Placement returns the configured placement.
func (p *Popover) Placement() PopoverPlacement {
	if p == nil || p.placement == "" {
		return Top
	}
	return p.placement
}

// EffectivePlacement mirrors left/right under RTL.
func (p *Popover) EffectivePlacement() PopoverPlacement {
	if p == nil || !p.rtl {
		return p.Placement()
	}
	switch p.Placement() {
	case Left:
		return Right
	case Right:
		return Left
	case LeftTop:
		return RightTop
	case RightTop:
		return LeftTop
	case LeftBottom:
		return RightBottom
	case RightBottom:
		return LeftBottom
	case TopLeft:
		return TopRight
	case TopRight:
		return TopLeft
	case BottomLeft:
		return BottomRight
	case BottomRight:
		return BottomLeft
	default:
		return p.Placement()
	}
}

// SetArrow toggles the caret.
func (p *Popover) SetArrow(v bool) *Popover {
	if p == nil || p.arrow == v {
		return p
	}
	p.arrow = v
	p.markPaint()
	return p
}

// Arrow reports the caret flag.
func (p *Popover) Arrow() bool {
	if p == nil {
		return true
	}
	return p.arrow
}

// SetArrowConfig sets show + pointAtCenter.
func (p *Popover) SetArrowConfig(show, pointAtCenter bool) *Popover {
	if p == nil {
		return p
	}
	p.arrow = show
	p.pointAtCenter = pointAtCenter
	p.markPaint()
	return p
}

// PointAtCenter reports the arrow centering flag.
func (p *Popover) PointAtCenter() bool { return p != nil && p.pointAtCenter }

// SetAutoAdjustOverflow toggles flip/shift.
func (p *Popover) SetAutoAdjustOverflow(v bool) *Popover {
	if p == nil || p.autoAdjust == v {
		return p
	}
	p.autoAdjust = v
	p.markPaint()
	return p
}

// AutoAdjustOverflow reports the flip/shift flag.
func (p *Popover) AutoAdjustOverflow() bool {
	if p == nil {
		return true
	}
	return p.autoAdjust
}

// SetOpen sets controlled open (no callback, programmatic).
func (p *Popover) SetOpen(v bool) *Popover {
	if p == nil {
		return p
	}
	p.controlled = true
	p.controlledOpen = v
	p.pendingHoverClose = false
	p.markPaint()
	p.refreshOverlay()
	return p
}

// Controlled reports whether SetOpen was used.
func (p *Popover) Controlled() bool { return p != nil && p.controlled }

// SetDefaultOpen sets the uncontrolled initial value.
func (p *Popover) SetDefaultOpen(v bool) *Popover {
	if p == nil {
		return p
	}
	if !p.controlled {
		p.uncontrolledOpen = v
		p.markPaint()
	}
	return p
}

// SetOnOpenChange sets the visibility callback.
func (p *Popover) SetOnOpenChange(fn func(bool)) *Popover {
	if p == nil {
		return p
	}
	p.onOpenChange = fn
	return p
}

// SetZIndex overrides the portal order (0 = default).
func (p *Popover) SetZIndex(v int) *Popover {
	if p == nil || p.zIndex == v {
		return p
	}
	p.zIndex = v
	p.markPaint()
	return p
}

// ZIndex returns the override (0 = default).
func (p *Popover) ZIndex() int {
	if p == nil {
		return 0
	}
	return p.zIndex
}

// EffectiveZIndex maps to the portal ladder.
func (p *Popover) EffectiveZIndex() int {
	if p == nil {
		return 1030
	}
	if p.zIndex != 0 {
		return p.zIndex
	}
	tok := p.themeTokens()
	base := tok.ZIndexPopupBase
	if base <= 0 {
		base = 1000
	}
	return int(base) + 30
}

// SetDisabled swallows open intents.
func (p *Popover) SetDisabled(v bool) *Popover {
	if p == nil || p.disabled == v {
		return p
	}
	p.disabled = v
	if p.focusNode != nil {
		p.focusNode.Enabled = !v
	}
	if v {
		p.hoverInTrigger = false
		p.hoverInPanel = false
		p.pendingHoverClose = false
		p.hovered = false
		p.pressed = false
		if p.controlled {
			if p.controlledOpen && p.onOpenChange != nil {
				p.onOpenChange(false)
			}
			p.controlledOpen = false
		} else if p.uncontrolledOpen {
			p.uncontrolledOpen = false
			if p.onOpenChange != nil {
				p.onOpenChange(false)
			}
		}
	}
	p.markPaint()
	p.refreshOverlay()
	return p
}

// Disabled reports the flag.
func (p *Popover) Disabled() bool { return p != nil && p.disabled }

// SetProvider selects the theme source.
func (p *Popover) SetProvider(v *theme.Provider) *Popover {
	if p == nil {
		return p
	}
	p.provider = v
	p.markPaint()
	return p
}

// SetTheme pins exact tokens (nil clears).
func (p *Popover) SetTheme(t *theme.Tokens) *Popover {
	if p == nil {
		return p
	}
	p.Theme = t
	p.markPaint()
	return p
}

// SetFace stores the font family hook.
func (p *Popover) SetFace(family string) *Popover {
	if p == nil {
		return p
	}
	p.faceName = family
	return p
}

// SetTextFace sets the paint-only font face.
func (p *Popover) SetTextFace(f text.Face) *Popover {
	if p == nil {
		return p
	}
	p.textFace = f
	p.markPaint()
	return p
}

// SetAriaLabel pins the accessible name.
func (p *Popover) SetAriaLabel(s string) *Popover {
	if p == nil {
		return p
	}
	p.ariaLabel = s
	return p
}

// AriaName is ariaLabel or the trigger label.
func (p *Popover) AriaName() string {
	if p == nil {
		return ""
	}
	if p.ariaLabel != "" {
		return p.ariaLabel
	}
	return p.triggerLabel
}

// Role is the trigger reader role.
func (p *Popover) Role() string { return "button" }

// PanelRole is the floating card reader role.
func (p *Popover) PanelRole() string { return "dialog" }

// PanelLabel prefers title for the accessible name.
func (p *Popover) PanelLabel() string {
	if p == nil {
		return ""
	}
	if p.title != "" {
		return p.title
	}
	return p.content
}

// SetRTL mirrors placement and icon order.
func (p *Popover) SetRTL(v bool) *Popover {
	if p == nil || p.rtl == v {
		return p
	}
	p.rtl = v
	p.markPaint()
	return p
}

// RTL reports mirror mode.
func (p *Popover) RTL() bool { return p != nil && p.rtl }

// IsRTL is an alias of RTL.
func (p *Popover) IsRTL() bool { return p.RTL() }

// SetPanelBackground approximates the color preset (P1 color stays staged).
func (p *Popover) SetPanelBackground(c render.RGBA) *Popover {
	if p == nil {
		return p
	}
	p.panelBackground = c
	p.hasPanelBackground = true
	p.markPaint()
	return p
}

// ClearPanelBackground removes the override.
func (p *Popover) ClearPanelBackground() *Popover {
	if p == nil {
		return p
	}
	p.hasPanelBackground = false
	p.markPaint()
	return p
}

// SetViewport sets the overflow viewport for Resolve.
func (p *Popover) SetViewport(w, h float64) *Popover {
	if p == nil {
		return p
	}
	p.viewportW, p.viewportH = w, h
	return p
}

// SetContentAction installs the inner-button callback (stays open).
func (p *Popover) SetContentAction(fn func()) *Popover {
	if p == nil {
		return p
	}
	p.contentAction = fn
	return p
}

// PressContentAction fires the inner action without closing.
func (p *Popover) PressContentAction() bool {
	if p == nil || !p.IsOpen() {
		return false
	}
	if p.contentAction != nil {
		p.contentAction()
		return true
	}
	return false
}

// IsOpen reports visibility.
func (p *Popover) IsOpen() bool {
	if p == nil {
		return false
	}
	if p.controlled {
		return p.controlledOpen
	}
	return p.uncontrolledOpen
}

func (p *Popover) requestOpen(want bool) {
	if p == nil {
		return
	}
	if want && p.disabled {
		return
	}
	if p.controlled {
		if want != p.controlledOpen && p.onOpenChange != nil {
			p.onOpenChange(want)
		}
		return
	}
	if want == p.uncontrolledOpen {
		return
	}
	p.uncontrolledOpen = want
	if p.onOpenChange != nil {
		p.onOpenChange(want)
	}
	p.markPaint()
	p.refreshOverlay()
}

// HoverEnter opens for hover trigger (instant P0).
func (p *Popover) HoverEnter() {
	if p == nil || p.disabled {
		return
	}
	p.hoverInTrigger = true
	p.pendingHoverClose = false
	p.hovered = true
	if p.hasTrigger(TriggerHover) {
		p.requestOpen(true)
	} else {
		p.markPaint()
	}
}

// HoverLeave arms the one-Tick grace.
func (p *Popover) HoverLeave() {
	if p == nil {
		return
	}
	p.hoverInTrigger = false
	p.hovered = false
	if p.hasTrigger(TriggerHover) && p.IsOpen() {
		p.pendingHoverClose = true
	}
	p.markPaint()
}

// PanelEnter cancels the grace (trigger to panel gap).
func (p *Popover) PanelEnter() {
	if p == nil {
		return
	}
	p.hoverInPanel = true
	p.pendingHoverClose = false
}

// PanelLeave re-arms the grace.
func (p *Popover) PanelLeave() {
	if p == nil {
		return
	}
	p.hoverInPanel = false
	if p.hasTrigger(TriggerHover) && p.IsOpen() {
		p.pendingHoverClose = true
	}
}

// Tick closes after the grace when still outside.
func (p *Popover) Tick() {
	if p == nil || !p.pendingHoverClose {
		return
	}
	if p.hoverInTrigger || p.hoverInPanel {
		p.pendingHoverClose = false
		return
	}
	p.pendingHoverClose = false
	if p.hasTrigger(TriggerHover) {
		p.requestOpen(false)
	}
}

// ClickTrigger toggles for click trigger.
func (p *Popover) ClickTrigger() bool {
	if p == nil || p.disabled || !p.hasTrigger(TriggerClick) {
		return false
	}
	p.requestOpen(!p.IsOpen())
	return true
}

// FocusTrigger opens for focus trigger.
func (p *Popover) FocusTrigger() {
	if p == nil || p.disabled {
		return
	}
	if p.hasTrigger(TriggerFocus) {
		p.requestOpen(true)
	}
}

// BlurTrigger closes for focus trigger.
func (p *Popover) BlurTrigger() {
	if p == nil {
		return
	}
	if p.hasTrigger(TriggerFocus) {
		p.requestOpen(false)
	}
}

// ContextMenu opens for contextMenu trigger.
func (p *Popover) ContextMenu() bool {
	if p == nil || p.disabled || !p.hasTrigger(TriggerContextMenu) {
		return false
	}
	p.requestOpen(true)
	return true
}

// OutsidePress dismisses a non-controlled open.
func (p *Popover) OutsidePress() bool {
	if p == nil || !p.IsOpen() {
		return false
	}
	p.requestOpen(false)
	return true
}

// Escape closes the open card (FocusScope parity).
func (p *Popover) Escape() bool { return p.OutsidePress() }

// PressKey handles keyboard names (Enter/Space/Escape).
func (p *Popover) PressKey(key string) bool {
	if p == nil || p.disabled {
		return false
	}
	switch key {
	case "Escape", "Esc", "escape":
		return p.Escape()
	case "Enter", "Space", " ", "\r":
		if p.hasTrigger(TriggerClick) {
			return p.ClickTrigger()
		}
		if p.hasTrigger(TriggerFocus) && p.focused {
			p.requestOpen(!p.IsOpen())
			return true
		}
	}
	return false
}

// Focusable is false while disabled.
func (p *Popover) Focusable() bool { return p != nil && !p.disabled }

// Focused reports keyboard focus.
func (p *Popover) Focused() bool { return p != nil && p.focused }

// Hovered reports pointer hover on the trigger.
func (p *Popover) Hovered() bool { return p != nil && p.hovered }

// FocusNode lazily builds the trigger node.
func (p *Popover) FocusNode() *focus.FocusNode {
	if p == nil {
		return nil
	}
	if p.focusNode == nil {
		n := focus.NewFocusNode("popover:" + p.AriaName())
		n.Enabled = !p.disabled
		self := p
		n.OnActivate = func() { self.PressKey("Enter") }
		n.OnFocusChange = func(f bool) {
			self.focused = f
			if f {
				self.FocusTrigger()
			} else {
				self.BlurTrigger()
			}
			self.markPaint()
		}
		p.focusNode = n
	}
	return p.focusNode
}

func (p *Popover) inside(x, y float64) bool {
	w, h := p.lastTrigger.Width, p.lastTrigger.Height
	var dx, dy float64
	if w < minTouchTarget {
		dx = (minTouchTarget - w) / 2
	}
	if h < minTouchTarget {
		dy = (minTouchTarget - h) / 2
	}
	return x >= -dx && y >= -dy && x < w+dx && y < h+dy
}

// HitSize is the tappable extent (44px floor).
func (p *Popover) HitSize() rendering.Size {
	if p == nil {
		return rendering.Size{Width: minTouchTarget, Height: minTouchTarget}
	}
	w, h := p.lastTrigger.Width, p.lastTrigger.Height
	if w < minTouchTarget {
		w = minTouchTarget
	}
	if h < minTouchTarget {
		h = minTouchTarget
	}
	return rendering.Size{Width: w, Height: h}
}

// PointerMove tracks hover with Tick grace.
func (p *Popover) PointerMove(x, y float64) {
	if p == nil || p.disabled {
		return
	}
	in := p.inside(x, y)
	if in && !p.hoverInTrigger {
		p.HoverEnter()
	} else if !in && p.hoverInTrigger {
		p.HoverLeave()
	}
}

// PointerDown starts a press; click trigger consumes.
func (p *Popover) PointerDown(x, y float64) bool {
	if p == nil || p.disabled || !p.inside(x, y) {
		return false
	}
	p.pressed, p.inBound = true, true
	if p.focusNode != nil {
		p.focusNode.RequestFocus()
	}
	p.markPaint()
	return p.hasTrigger(TriggerClick) || p.hasTrigger(TriggerContextMenu)
}

// PointerUp releases; contained release toggles click trigger.
func (p *Popover) PointerUp(x, y float64) bool {
	if p == nil || !p.pressed {
		return false
	}
	p.pressed = false
	in := !p.disabled && p.inside(x, y) && p.inBound
	p.inBound = false
	p.markPaint()
	if in && p.hasTrigger(TriggerClick) {
		p.ClickTrigger()
		return true
	}
	return false
}

// Semantics builds the read tree (trigger button + open dialog).
func (p *Popover) Semantics() *semantics.Node {
	if p == nil {
		return nil
	}
	root := &semantics.Node{Role: semantics.RoleGeneric, Label: p.AriaName()}
	root.Add(&semantics.Node{Role: semantics.RoleButton, Label: p.AriaName(), Focusable: p.Focusable()})
	if p.IsOpen() {
		root.Add(&semantics.Node{Role: semantics.RoleDialog, Label: p.PanelLabel(), Focusable: false})
	}
	return root
}

// Node returns the trigger tree node.
func (p *Popover) Node() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.root
}

// TriggerShell returns the trigger paint node.
func (p *Popover) TriggerShell() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.triggerBox
}

// Panel returns the floating card node.
func (p *Popover) Panel() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.panelBox
}

// Popup returns the floating card node (overlay child).
func (p *Popover) Popup() rendering.RenderObject {
	if p == nil {
		return nil
	}
	return p.panelBox
}

// LaidOut returns the last trigger size.
func (p *Popover) LaidOut() rendering.Size {
	if p == nil {
		return rendering.Size{}
	}
	return p.lastTrigger
}

// PanelLaidOut returns the last panel size.
func (p *Popover) PanelLaidOut() rendering.Size {
	if p == nil {
		return rendering.Size{}
	}
	return p.lastPanel
}

func (p *Popover) themeTokens() theme.Tokens {
	if p != nil && p.Theme != nil {
		return *p.Theme
	}
	if p != nil && p.provider != nil {
		return p.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// ContrastRatio flattens fg over bg and returns the WCAG ratio.
func ContrastRatio(fg, bg render.RGBA) float64 {
	lf := relLum(flat(fg.R, bg.R, fg.A), flat(fg.G, bg.G, fg.A), flat(fg.B, bg.B, fg.A))
	lb := relLum(bg.R, bg.G, bg.B)
	hi, lo := lf, lb
	if lo > hi {
		hi, lo = lo, hi
	}
	return (hi + 0.05) / (lo + 0.05)
}

func flat(fg, bg, a float64) float64 { return fg*a + bg*(1-a) }

func relLum(r, g, b float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// PanelBackground is colorBgContainer unless overridden.
func (p *Popover) PanelBackground() render.RGBA {
	if p != nil && p.hasPanelBackground {
		return p.panelBackground
	}
	return themeToRGBA(p.themeTokens().ColorBgContainer)
}

// TitleColor is heading ink.
func (p *Popover) TitleColor() render.RGBA { return themeToRGBA(p.themeTokens().ColorTextHeading) }

// ContentColor is body ink.
func (p *Popover) ContentColor() render.RGBA { return themeToRGBA(p.themeTokens().ColorText) }

// BorderColor is the panel edge.
func (p *Popover) BorderColor() render.RGBA { return themeToRGBA(p.themeTokens().ColorBorder) }

// Radius is borderRadiusLG.
func (p *Popover) Radius() float64 { return p.themeTokens().RadiusLG }

// LineWidth is the panel edge width.
func (p *Popover) LineWidth() float64 { return p.themeTokens().LineWidth }

// FontSize is the body size.
func (p *Popover) FontSize() float64 { return p.themeTokens().FontSize }

// InnerPadding is 12 (PaddingSM).
func (p *Popover) InnerPadding() float64 {
	if v := p.themeTokens().PaddingSM; v > 0 {
		return v
	}
	return DefaultPopoverInnerPadding
}

// TitleMinWidth is 177.
func (p *Popover) TitleMinWidth() float64 { return DefaultPopoverTitleMinWidth }

// TitleMarginBottom is marginXS 8.
func (p *Popover) TitleMarginBottom() float64 {
	if v := p.themeTokens().MarginXS; v > 0 {
		return v
	}
	return DefaultPopoverTitleMarginBottom
}

// Gap is the trigger to panel distance 8.
func (p *Popover) Gap() float64 { return DefaultPopoverGap }

// ArrowSize is the caret edge 8.
func (p *Popover) ArrowSize() float64 { return DefaultPopoverArrowSize }

func measureNode(n rendering.RenderObject) (float64, float64) {
	if n == nil {
		return 0, 0
	}
	sz := n.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return sz.Width, sz.Height
}

func estimateText(s string, fontSize float64) (float64, float64) {
	return rendering.EstimateTextSize(s, fontSize, 0.55)
}

func (p *Popover) triggerIdeal() (float64, float64) {
	if p.triggerNode != nil {
		if w, h := measureNode(p.triggerNode); w > 0 && h > 0 {
			return w, h
		}
	}
	tok := p.themeTokens()
	h := tok.ControlHeight
	if h <= 0 {
		h = triggerHeight
	}
	fs := tok.FontSize
	if fs <= 0 {
		fs = DefaultPopoverFontSize
	}
	tw, _ := estimateText(p.triggerLabel, fs)
	return tw + 2*triggerPadX, h
}

func (p *Popover) panelIdeal() (float64, float64) {
	tok := p.themeTokens()
	fs := tok.FontSize
	if fs <= 0 {
		fs = DefaultPopoverFontSize
	}
	pad := p.InnerPadding()
	var titleW, titleH float64
	hasTitle := p.title != "" || p.titleNode != nil
	if p.titleNode != nil {
		titleW, titleH = measureNode(p.titleNode)
	} else if p.title != "" {
		titleW, titleH = estimateText(p.title, fs)
	}
	var contentW, contentH float64
	hasContent := p.content != "" || p.contentNode != nil
	if p.contentNode != nil {
		contentW, contentH = measureNode(p.contentNode)
	} else if p.content != "" {
		contentW, contentH = estimateText(p.content, fs)
	} else {
		contentW, contentH = 60, fs*1.25
	}
	bodyW := titleW
	if contentW > bodyW {
		bodyW = contentW
	}
	if hasTitle && bodyW < p.TitleMinWidth() {
		bodyW = p.TitleMinWidth()
	}
	w := bodyW + 2*pad
	h := 2*pad + titleH
	if hasTitle && hasContent {
		h += p.TitleMarginBottom()
	}
	if hasContent {
		h += contentH
	}
	if h < 40 {
		h = 40
	}
	return w, h
}

// TriggerIdeal exposes the trigger size for tests.
func (p *Popover) TriggerIdeal() rendering.Size {
	w, h := p.triggerIdeal()
	return rendering.Size{Width: w, Height: h}
}

// PanelIdeal exposes the panel size for tests.
func (p *Popover) PanelIdeal() rendering.Size {
	w, h := p.panelIdeal()
	return rendering.Size{Width: w, Height: h}
}

// Layout sizes trigger (Exact/Min/Max matrix) and records panel size.
func (p *Popover) Layout(c rendering.Constraints) rendering.Size {
	if p == nil {
		return rendering.Size{}
	}
	tw, th := p.triggerIdeal()
	out := c.Tighten(rendering.Size{Width: tw, Height: th})
	p.lastTrigger = out
	pw, ph := p.panelIdeal()
	p.lastPanel = rendering.Size{Width: pw, Height: ph}
	p.syncTrigger()
	if p.triggerBox != nil {
		p.triggerBox.FixedWidth, p.triggerBox.FixedHeight = out.Width, out.Height
	}
	if p.panelBox != nil {
		p.panelBox.FixedWidth, p.panelBox.FixedHeight = pw, ph
		p.panelBox.Layout(rendering.Tight(pw, ph))
	}
	if p.root == nil {
		return out
	}
	sz := p.root.Layout(c)
	return sz
}

// ResolveOptions builds overlay options from flags.
func (p *Popover) ResolveOptions() *overlay.ResolveOptions {
	auto := false
	if p != nil {
		auto = p.autoAdjust
	}
	vw, vh := 800.0, 600.0
	point := false
	if p != nil {
		vw, vh = p.viewportW, p.viewportH
		point = p.pointAtCenter
	}
	if !auto {
		return &overlay.ResolveOptions{Gap: DefaultPopoverGap, Flip: false, Shift: false, ArrowPointAtCenter: point}
	}
	return &overlay.ResolveOptions{Gap: DefaultPopoverGap, Flip: true, Shift: true, ViewportW: vw, ViewportH: vh, ArrowPointAtCenter: point}
}

// Resolve places a panel box relative to anchor (overlay call, no rewrite).
func (p *Popover) Resolve(anchor rendering.Rect, ow, oh, vw, vh float64) overlay.Resolved {
	point := false
	auto := true
	if p != nil {
		point = p.pointAtCenter
		auto = p.autoAdjust
	}
	if !auto {
		return overlay.Resolve(anchor, ow, oh, p.EffectivePlacement().toOverlay(), &overlay.ResolveOptions{Gap: DefaultPopoverGap, Flip: false, Shift: false, ArrowPointAtCenter: point})
	}
	return overlay.Resolve(anchor, ow, oh, p.EffectivePlacement().toOverlay(), &overlay.ResolveOptions{Gap: DefaultPopoverGap, Flip: true, Shift: true, ViewportW: vw, ViewportH: vh, ArrowPointAtCenter: point})
}

// PopupOrigin resolves the open panel origin for a trigger at (tx,ty).
func (p *Popover) PopupOrigin(tx, ty float64) overlay.Resolved {
	anchor := rendering.NewRect(tx, ty, p.lastTrigger.Width, p.lastTrigger.Height)
	return p.Resolve(anchor, p.lastPanel.Width, p.lastPanel.Height, p.viewportW, p.viewportH)
}

// AttachToOverlay inserts the panel entry at the resolved origin.
func (p *Popover) AttachToOverlay(st *overlay.State, anchor rendering.Rect) *overlay.Entry {
	if p == nil || st == nil {
		return nil
	}
	if !p.IsOpen() {
		p.DetachOverlay()
		return nil
	}
	res := p.Resolve(anchor, p.lastPanel.Width, p.lastPanel.Height, p.viewportW, p.viewportH)
	if p.overlayEntry != nil {
		st.Remove(p.overlayEntry)
		p.overlayEntry = nil
	}
	e := overlay.NewEntry(p.panelBox, res.X, res.Y, p.lastPanel.Width, p.lastPanel.Height)
	st.Insert(e)
	p.overlayEntry = e
	p.overlayState = st
	return e
}

// DetachOverlay removes the panel entry.
func (p *Popover) DetachOverlay() {
	if p == nil {
		return
	}
	if p.overlayState != nil && p.overlayEntry != nil {
		p.overlayState.Remove(p.overlayEntry)
	}
	p.overlayEntry = nil
	p.overlayState = nil
}

func (p *Popover) refreshOverlay() {
	if p == nil || p.overlayState == nil {
		return
	}
	if !p.IsOpen() {
		p.DetachOverlay()
	}
}

func (p *Popover) paintText(pc *rendering.PaintContext, s string, x, y, w, h, fontSize float64, c render.RGBA) {
	if pc == nil || pc.DC == nil || s == "" || w <= 0 || h <= 0 {
		return
	}
	if p == nil || p.textFace == nil {
		return
	}
	pc.DC.SetFont(p.textFace)
	pc.DC.SetRGBA(c.R, c.G, c.B, c.A)
	baseline := y + h/2 + fontSize*0.35
	ax, ay := pc.Abs(x, baseline)
	pc.DC.DrawString(s, ax, ay)
}

func (p *Popover) paintTrigger(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || p == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	p.lastTrigger = size
	tok := p.themeTokens()
	w, h := size.Width, size.Height
	bg := themeToRGBA(tok.ColorBgContainer)
	bd := themeToRGBA(tok.ColorBorder)
	tx := themeToRGBA(tok.ColorText)
	if p.disabled {
		bg = themeToRGBA(tok.ColorFillTertiary)
		tx = themeToRGBA(tok.ColorTextDisabled)
	} else if p.hovered && !p.pressed {
		acc := themeToRGBA(tok.ColorPrimary)
		bd = acc
		tx = acc
	}
	radius := tok.Radius
	if radius <= 0 {
		radius = 6
	}
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, bg.R, bg.G, bg.B, bg.A)
	rendering.StrokeRoundRect(pc, 0.5, 0.5, w-1, h-1, radius, 1, bd.R, bd.G, bd.B, bd.A)
	if p.triggerNode != nil {
		p.triggerNode.Paint(pc.WithOrigin(pc.OriginX, pc.OriginY))
		return
	}
	p.paintText(pc, p.triggerLabel, 4, 0, w-8, h, p.FontSize(), tx)
	if p.focused && !p.disabled {
		ring := themeToRGBA(tok.ColorPrimary)
		rx, ry, rw, rh := focus.FocusRingRect(0, 0, w, h, focusRingOutset)
		rendering.StrokeRoundRect(pc, rx, ry, rw, rh, radius+focusRingOutset, 2, ring.R, ring.G, ring.B, ring.A)
	}
}

func (p *Popover) paintPanel(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || p == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	tok := p.themeTokens()
	w, h := size.Width, size.Height
	bg := p.PanelBackground()
	bd := p.BorderColor()
	radius := p.Radius()
	if radius <= 0 {
		radius = 8
	}
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, bg.R, bg.G, bg.B, bg.A)
	lw := p.LineWidth()
	if lw <= 0 {
		lw = 1
	}
	rendering.StrokeRoundRect(pc, lw/2, lw/2, w-lw, h-lw, radius, lw, bd.R, bd.G, bd.B, bd.A)
	pad := p.InnerPadding()
	hasTitle := p.title != "" || p.titleNode != nil
	hasContent := p.content != "" || p.contentNode != nil
	y := pad
	if hasTitle {
		th := 20.0
		if p.titleNode != nil {
			p.titleNode.Paint(pc.WithOrigin(pc.OriginX+pad, pc.OriginY+y))
			_, nh := measureNode(p.titleNode)
			if nh > 0 {
				th = nh
			}
		} else {
			p.paintText(pc, p.title, pad, y, w-2*pad, th, p.FontSize(), p.TitleColor())
		}
		y += th
		if hasContent {
			y += p.TitleMarginBottom()
		}
	}
	if hasContent {
		ch := h - y - pad
		if ch < 0 {
			ch = 0
		}
		if p.contentNode != nil {
			p.contentNode.Paint(pc.WithOrigin(pc.OriginX+pad, pc.OriginY+y))
		} else {
			p.paintText(pc, p.content, pad, y, w-2*pad, ch, p.FontSize(), p.ContentColor())
		}
	}
	if p.arrow {
		p.paintArrow(pc, w, h, tok)
	}
}

func (p *Popover) paintArrow(pc *rendering.PaintContext, w, h float64, tok theme.Tokens) {
	bg := p.PanelBackground()
	bd := p.BorderColor()
	s := p.ArrowSize()
	if s <= 0 {
		s = 8
	}
	anchor := rendering.NewRect(0, 0, p.lastTrigger.Width, p.lastTrigger.Height)
	res := p.Resolve(anchor, w, h, 0, 0)
	ax, ay := res.ArrowX, res.ArrowY
	if ax < s {
		ax = s
	}
	if ax > w-s {
		ax = w - s
	}
	if ay < s {
		ay = s
	}
	if ay > h-s {
		ay = h - s
	}
	half := s / 2
	var cx, cy float64
	switch p.EffectivePlacement() {
	case Top, TopLeft, TopRight:
		cx, cy = ax, h-0.5
	case Bottom, BottomLeft, BottomRight:
		cx, cy = ax, 0.5
	case Left, LeftTop, LeftBottom:
		cx, cy = w-0.5, ay
	default:
		cx, cy = 0.5, ay
	}
	path := rendering.NewPath()
	path.MoveTo(cx-half, cy)
	path.LineTo(cx, cy+half)
	path.LineTo(cx+half, cy)
	path.LineTo(cx, cy-half)
	path.Close()
	_ = tok
	rendering.FillPath(pc, path, bg.R, bg.G, bg.B, bg.A)
	rendering.StrokePath(pc, path, 1, bd.R, bd.G, bd.B, bd.A)
}
