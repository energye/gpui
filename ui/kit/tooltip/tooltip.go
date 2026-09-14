// Package tooltip implements the Tooltip control (docs/antd/tooltip.md §6).
//
// Simple hover/focus/click text bubble: spotlight panel + arrow + 12-way
// placement. Owns rendering nodes and reuses ui/rendering for draw,
// ui/theme for tokens, ui/overlay for placement (Resolve) and the portal
// stack (State Insert/Remove), ui/focus for keyboard and ui/semantics for
// the read tree. No second event/frame system.
package tooltip

import (
	"math"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/overlay"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks (docs/antd/tooltip.md §6.2.1).
const (
	DefaultTooltipPaddingX  = 8.0
	DefaultTooltipPaddingY  = 6.0
	DefaultTooltipRadius    = 6.0
	DefaultTooltipFontSize  = 14.0
	DefaultTooltipMaxWidth  = 250.0
	DefaultTooltipArrowSize = 8.0
	DefaultTooltipGap       = 8.0
	DefaultTooltipPortalZ   = 1070
	DefaultEnterDelay       = 0.1
	DefaultLeaveDelay       = 0.1
	// triggerHitFloor is the a11y hit floor; visuals keep spec size.
	triggerHitFloor = 44.0
)

// TooltipTrigger is hover|focus|click|contextMenu (antd §6.3).
type TooltipTrigger string

const (
	TriggerHover       TooltipTrigger = "hover"
	TriggerFocus       TooltipTrigger = "focus"
	TriggerClick       TooltipTrigger = "click"
	TriggerContextMenu TooltipTrigger = "contextMenu"
)

// TooltipPlacement is the 12-way bubble position (antd §6.3).
type TooltipPlacement string

const (
	Top         TooltipPlacement = "top"
	TopLeft     TooltipPlacement = "topLeft"
	TopRight    TooltipPlacement = "topRight"
	Bottom      TooltipPlacement = "bottom"
	BottomLeft  TooltipPlacement = "bottomLeft"
	BottomRight TooltipPlacement = "bottomRight"
	Left        TooltipPlacement = "left"
	LeftTop     TooltipPlacement = "leftTop"
	LeftBottom  TooltipPlacement = "leftBottom"
	Right       TooltipPlacement = "right"
	RightTop    TooltipPlacement = "rightTop"
	RightBottom TooltipPlacement = "rightBottom"
)

// AllPlacements lists the twelve positions in stable order.
var AllPlacements = []TooltipPlacement{
	Top, TopLeft, TopRight, Bottom, BottomLeft, BottomRight,
	Left, LeftTop, LeftBottom, Right, RightTop, RightBottom,
}

// presetHex covers the antd preset hues (same source as tag).
var presetHex = map[string]string{
	"blue":     "#1677ff",
	"purple":   "#722ed1",
	"cyan":     "#13c2c2",
	"green":    "#52c41a",
	"magenta":  "#eb2f96",
	"pink":     "#ff85c0",
	"red":      "#f5222d",
	"orange":   "#fa8c16",
	"yellow":   "#fadb14",
	"volcano":  "#fa541c",
	"geekblue": "#2f54eb",
	"lime":     "#a0d911",
	"gold":     "#faad14",
}

// Popup is the positioned bubble view model (placement + rect + arrow).
type Popup struct {
	Want    TooltipPlacement
	Actual  TooltipPlacement
	Flipped bool
	X, Y    float64
	W, H    float64
	ArrowX  float64
	ArrowY  float64
}

// Tooltip is the Tooltip widget (docs/antd/tooltip.md §6.10).
type Tooltip struct {
	title          string
	titleNode      rendering.RenderObject
	triggerLabel   string
	triggerNode    rendering.RenderObject
	placement      TooltipPlacement
	arrow          bool
	arrowCenter    bool
	triggers       []TooltipTrigger
	autoAdjust     bool
	zIndex         int
	controlled     bool
	controlledOpen bool
	open           bool
	onOpenChange   func(bool)
	enterDelay     float64
	leaveDelay     float64
	disabled       bool
	rtl            bool
	ariaLabel      string
	color          string
	viewportW      float64
	viewportH      float64
	anchorX        float64
	anchorY        float64
	provider       *theme.Provider
	override       *theme.Tokens
	textFace       text.Face
	hovered        bool
	focused        bool
	focusNode      *focus.FocusNode
	tickerOwner    *rendering.PipelineOwner
	pending        bool
	pendingWant    bool
	pendingElapsed float64
	pendingDelay   float64
	node           *rendering.RenderBox
	panelBox       *rendering.RenderBox
	overlayState   *overlay.State
	overlayEntry   *overlay.Entry
	lastSize       rendering.Size
	panelSize      rendering.Size
	resolved       overlay.Resolved
}

// NewTooltip creates a tooltip with title (empty title never opens).
func NewTooltip(title string) *Tooltip {
	t := &Tooltip{
		title: title, placement: Top, arrow: true,
		autoAdjust: true, zIndex: DefaultTooltipPortalZ,
		enterDelay: DefaultEnterDelay, leaveDelay: DefaultLeaveDelay,
		viewportW: 1200, viewportH: 800, anchorX: 200, anchorY: 160,
	}
	t.node = rendering.NewRenderBox()
	t.node.SetRepaintBoundary(true)
	t.node.SetRelayoutBoundary(true)
	self := t
	t.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paintTrigger(pc, size)
	}
	t.panelBox = rendering.NewRenderBox()
	t.panelBox.SetRepaintBoundary(true)
	t.panelBox.SetRelayoutBoundary(true)
	t.panelBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paintPanel(pc, size)
	}
	t.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return t
}

func (t *Tooltip) syncNode() {
	if t == nil {
		return
	}
	if t.node == nil {
		t.node = rendering.NewRenderBox()
		t.node.SetRepaintBoundary(true)
		t.node.SetRelayoutBoundary(true)
		self := t
		t.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			self.paintTrigger(pc, size)
		}
	}
	if t.panelBox == nil {
		t.panelBox = rendering.NewRenderBox()
		t.panelBox.SetRepaintBoundary(true)
		t.panelBox.SetRelayoutBoundary(true)
		self := t
		t.panelBox.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			self.paintPanel(pc, size)
		}
	}
}

// SetTitle sets the tip text (empty + no TitleNode never opens).
func (t *Tooltip) SetTitle(s string) *Tooltip {
	if t == nil {
		return t
	}
	if t.title == s {
		return t
	}
	was := t.IsOpen()
	t.title = s
	if !t.hasContent() && !t.controlled && t.open {
		t.open = false
		t.cancelPending()
		if was && t.onOpenChange != nil {
			t.onOpenChange(false)
		}
	}
	t.relayout()
	t.syncOverlay()
	return t
}

// Title returns the tip text.
func (t *Tooltip) Title() string {
	if t == nil {
		return ""
	}
	return t.title
}

// SetTitleNode installs a custom title slot.
func (t *Tooltip) SetTitleNode(n rendering.RenderObject) *Tooltip {
	if t == nil {
		return t
	}
	t.titleNode = n
	t.relayout()
	t.syncOverlay()
	return t
}

// TitleNode returns the custom title, if any.
func (t *Tooltip) TitleNode() rendering.RenderObject {
	if t == nil {
		return nil
	}
	return t.titleNode
}

// SetTriggerLabel sets the trigger text.
func (t *Tooltip) SetTriggerLabel(s string) *Tooltip {
	if t == nil {
		return t
	}
	if t.triggerLabel == s {
		return t
	}
	t.triggerLabel = s
	t.relayout()
	return t
}

// TriggerLabel returns the trigger text.
func (t *Tooltip) TriggerLabel() string {
	if t == nil {
		return ""
	}
	return t.triggerLabel
}

// SetTriggerNode installs a custom trigger.
func (t *Tooltip) SetTriggerNode(n rendering.RenderObject) *Tooltip {
	if t == nil {
		return t
	}
	t.triggerNode = n
	t.relayout()
	return t
}

// TriggerNode returns the custom trigger, if any.
func (t *Tooltip) TriggerNode() rendering.RenderObject {
	if t == nil {
		return nil
	}
	return t.triggerNode
}

// SetTrigger replaces the trigger set (empty resets to hover).
func (t *Tooltip) SetTrigger(m TooltipTrigger) *Tooltip {
	if t == nil {
		return t
	}
	t.triggers = []TooltipTrigger{normTrigger(m)}
	return t
}

// SetTriggerModes sets multiple triggers (empty resets to hover).
func (t *Tooltip) SetTriggerModes(modes ...TooltipTrigger) *Tooltip {
	if t == nil {
		return t
	}
	if len(modes) == 0 {
		t.triggers = nil
		return t
	}
	out := make([]TooltipTrigger, 0, len(modes))
	for _, m := range modes {
		out = append(out, normTrigger(m))
	}
	t.triggers = out
	return t
}

func normTrigger(m TooltipTrigger) TooltipTrigger {
	switch m {
	case TriggerHover, TriggerFocus, TriggerClick, TriggerContextMenu:
		return m
	default:
		return TriggerHover
	}
}

// Triggers returns the effective trigger set (empty means hover).
func (t *Tooltip) Triggers() []TooltipTrigger {
	if t == nil || len(t.triggers) == 0 {
		return []TooltipTrigger{TriggerHover}
	}
	return append([]TooltipTrigger(nil), t.triggers...)
}

// HasTrigger reports membership in the effective set.
func (t *Tooltip) HasTrigger(m TooltipTrigger) bool {
	for _, x := range t.Triggers() {
		if x == m {
			return true
		}
	}
	return false
}

// SetPlacement sets the bubble position (default top).
func (t *Tooltip) SetPlacement(p TooltipPlacement) *Tooltip {
	if t == nil {
		return t
	}
	if p == "" {
		p = Top
	}
	if t.placement == p {
		return t
	}
	t.placement = p
	t.refreshGeometry()
	t.markPaint()
	t.syncOverlay()
	return t
}

// Placement returns the wanted placement.
func (t *Tooltip) Placement() TooltipPlacement {
	if t == nil || t.placement == "" {
		return Top
	}
	return t.placement
}

// EffectivePlacement applies RTL mirroring and pointAtCenter collapsing.
func (t *Tooltip) EffectivePlacement() TooltipPlacement {
	p := t.Placement()
	if t != nil && t.arrowCenter {
		switch p {
		case TopLeft, TopRight:
			p = Top
		case BottomLeft, BottomRight:
			p = Bottom
		case LeftTop, LeftBottom:
			p = Left
		case RightTop, RightBottom:
			p = Right
		}
	}
	if t != nil && t.rtl {
		p = mirrorPlacement(p)
	}
	return p
}

func mirrorPlacement(p TooltipPlacement) TooltipPlacement {
	switch p {
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
		return p
	}
}

// ActualPlacement is the post-flip placement from Resolve.
func (t *Tooltip) ActualPlacement() TooltipPlacement {
	if t == nil {
		return Top
	}
	if t.resolved.Actual == "" {
		return t.EffectivePlacement()
	}
	return TooltipPlacement(t.resolved.Actual)
}

// SetArrow toggles the caret.
func (t *Tooltip) SetArrow(v bool) *Tooltip {
	if t == nil {
		return t
	}
	if t.arrow == v {
		return t
	}
	t.arrow = v
	t.markPaint()
	return t
}

// Arrow reports the caret flag.
func (t *Tooltip) Arrow() bool { return t == nil || t.arrow }

// SetArrowConfig sets show + pointAtCenter together.
func (t *Tooltip) SetArrowConfig(show, pointAtCenter bool) *Tooltip {
	if t == nil {
		return t
	}
	t.arrow = show
	t.arrowCenter = pointAtCenter
	t.refreshGeometry()
	t.markPaint()
	return t
}

// ArrowPointAtCenter reports the center-aim flag.
func (t *Tooltip) ArrowPointAtCenter() bool { return t != nil && t.arrowCenter }

// HasArrow reports whether the caret paints now.
func (t *Tooltip) HasArrow() bool { return t != nil && t.arrow }

// SetColor sets preset name or #hex ("" restores the default skin).
func (t *Tooltip) SetColor(s string) *Tooltip {
	if t == nil {
		return t
	}
	if t.color == s {
		return t
	}
	t.color = s
	t.markPaint()
	return t
}

// Color returns the raw color key.
func (t *Tooltip) Color() string {
	if t == nil {
		return ""
	}
	return t.color
}

// SetAutoAdjustOverflow toggles flip/shift (default true).
func (t *Tooltip) SetAutoAdjustOverflow(v bool) *Tooltip {
	if t == nil {
		return t
	}
	if t.autoAdjust == v {
		return t
	}
	t.autoAdjust = v
	t.refreshGeometry()
	t.syncOverlay()
	return t
}

// AutoAdjustOverflow reports the flip/shift flag.
func (t *Tooltip) AutoAdjustOverflow() bool { return t == nil || t.autoAdjust }

// SetZIndex sets the portal z-order.
func (t *Tooltip) SetZIndex(z int) *Tooltip {
	if t == nil {
		return t
	}
	t.zIndex = z
	return t
}

// ZIndex returns the portal z-order.
func (t *Tooltip) ZIndex() int {
	if t == nil {
		return DefaultTooltipPortalZ
	}
	if t.zIndex == 0 {
		return DefaultTooltipPortalZ
	}
	return t.zIndex
}

// SetOpen takes controlled ownership and sets the state.
func (t *Tooltip) SetOpen(v bool) *Tooltip {
	if t == nil {
		return t
	}
	t.controlled = true
	if t.controlledOpen == v {
		return t
	}
	t.controlledOpen = v
	t.cancelPending()
	t.markPaint()
	t.syncOverlay()
	return t
}

// IsControlled reports external ownership.
func (t *Tooltip) IsControlled() bool { return t != nil && t.controlled }

// SetDefaultOpen sets the uncontrolled initial state.
func (t *Tooltip) SetDefaultOpen(v bool) *Tooltip {
	if t == nil {
		return t
	}
	if t.controlled {
		return t
	}
	if t.open == v {
		return t
	}
	if v && (!t.hasContent() || t.disabled) {
		return t
	}
	t.open = v
	t.markPaint()
	t.syncOverlay()
	return t
}

// DefaultOpen reports the uncontrolled state (controlled ignores it).
func (t *Tooltip) DefaultOpen() bool { return t != nil && !t.controlled && t.open }

// SetOnOpenChange sets the visibility callback.
func (t *Tooltip) SetOnOpenChange(fn func(bool)) *Tooltip {
	if t == nil {
		return t
	}
	t.onOpenChange = fn
	return t
}

// SetMouseEnterDelay sets the hover-open delay in seconds.
func (t *Tooltip) SetMouseEnterDelay(sec float64) *Tooltip {
	if t == nil {
		return t
	}
	if sec < 0 {
		sec = 0
	}
	t.enterDelay = sec
	return t
}

// MouseEnterDelay returns the hover-open delay.
func (t *Tooltip) MouseEnterDelay() float64 {
	if t == nil {
		return DefaultEnterDelay
	}
	return t.enterDelay
}

// SetMouseLeaveDelay sets the hover-close delay in seconds.
func (t *Tooltip) SetMouseLeaveDelay(sec float64) *Tooltip {
	if t == nil {
		return t
	}
	if sec < 0 {
		sec = 0
	}
	t.leaveDelay = sec
	return t
}

// MouseLeaveDelay returns the hover-close delay.
func (t *Tooltip) MouseLeaveDelay() float64 {
	if t == nil {
		return DefaultLeaveDelay
	}
	return t.leaveDelay
}

// SetDisabled hard-closes and swallows triggers.
func (t *Tooltip) SetDisabled(v bool) *Tooltip {
	if t == nil {
		return t
	}
	if t.disabled == v {
		return t
	}
	t.disabled = v
	if t.focusNode != nil {
		t.focusNode.Enabled = !v
	}
	t.cancelPending()
	if v && !t.controlled && t.open {
		t.open = false
		if t.onOpenChange != nil {
			t.onOpenChange(false)
		}
	}
	t.markPaint()
	t.syncOverlay()
	return t
}

// Disabled reports the flag.
func (t *Tooltip) Disabled() bool { return t != nil && t.disabled }

// SetRTL mirrors left/right placements for the RTL snapshot.
func (t *Tooltip) SetRTL(v bool) *Tooltip {
	if t == nil {
		return t
	}
	if t.rtl == v {
		return t
	}
	t.rtl = v
	t.refreshGeometry()
	t.markPaint()
	t.syncOverlay()
	return t
}

// IsRTL reports mirror mode.
func (t *Tooltip) IsRTL() bool { return t != nil && t.rtl }

// SetAnchor sets the overlay anchor origin (window coords, headless probe).
func (t *Tooltip) SetAnchor(x, y float64) *Tooltip {
	if t == nil {
		return t
	}
	t.anchorX, t.anchorY = x, y
	t.refreshGeometry()
	t.syncOverlay()
	return t
}

// Anchor returns the overlay anchor origin.
func (t *Tooltip) Anchor() (float64, float64) {
	if t == nil {
		return 0, 0
	}
	return t.anchorX, t.anchorY
}

// SetViewport sets the flip/shift bounds (window client size).
func (t *Tooltip) SetViewport(w, h float64) *Tooltip {
	if t == nil {
		return t
	}
	t.viewportW, t.viewportH = w, h
	t.refreshGeometry()
	t.syncOverlay()
	return t
}

// Viewport returns the flip/shift bounds.
func (t *Tooltip) Viewport() (float64, float64) {
	if t == nil {
		return 0, 0
	}
	return t.viewportW, t.viewportH
}

// SetProvider selects the theme source (nil selects process default).
func (t *Tooltip) SetProvider(p *theme.Provider) *Tooltip {
	if t == nil {
		return t
	}
	t.provider = p
	t.markPaint()
	return t
}

// SetTheme pins exact tokens (nil clears to provider).
func (t *Tooltip) SetTheme(tok *theme.Tokens) *Tooltip {
	if t == nil {
		return t
	}
	t.override = tok
	t.markPaint()
	return t
}

// SetFace sets the paint-only font face (nil clears; layout keeps the
// rune estimate so headless tests stay stable).
func (t *Tooltip) SetFace(f text.Face) *Tooltip {
	if t == nil {
		return t
	}
	t.textFace = f
	t.markPaint()
	return t
}

// SetTextFace is the alias of SetFace.
func (t *Tooltip) SetTextFace(f text.Face) *Tooltip { return t.SetFace(f) }

// TextFace returns the paint-only font face, if any.
func (t *Tooltip) TextFace() text.Face {
	if t == nil {
		return nil
	}
	return t.textFace
}

// SetAriaLabel pins the accessible name.
func (t *Tooltip) SetAriaLabel(s string) *Tooltip {
	if t == nil {
		return t
	}
	t.ariaLabel = s
	return t
}

// AriaLabel returns the accessible name override.
func (t *Tooltip) AriaLabel() string {
	if t == nil {
		return ""
	}
	return t.ariaLabel
}

// AriaName is ariaLabel, else trigger label, else the title summary.
func (t *Tooltip) AriaName() string {
	if t == nil {
		return ""
	}
	if t.ariaLabel != "" {
		return t.ariaLabel
	}
	if t.triggerLabel != "" {
		return t.triggerLabel
	}
	return t.title
}

// Role is the bubble reader role (§6.6).
func (t *Tooltip) Role() string { return "tooltip" }

// TriggerRole is the trigger reader role when focusable.
func (t *Tooltip) TriggerRole() string { return "button" }

// Semantics builds the read tree (trigger button + tooltip bubble).
func (t *Tooltip) Semantics() *semantics.Node {
	if t == nil {
		return nil
	}
	root := &semantics.Node{Role: semantics.Role(t.TriggerRole()), Label: t.AriaName(), Focusable: t.Focusable()}
	if t.IsOpen() {
		root.Add(&semantics.Node{Role: semantics.Role("tooltip"), Label: t.title})
	}
	return root
}

// Focusable is false while disabled (manager skips the node).
func (t *Tooltip) Focusable() bool { return t != nil && !t.disabled }

// Focused reports keyboard focus on the trigger.
func (t *Tooltip) Focused() bool { return t != nil && t.focused }

// Hovered reports pointer hover on the trigger.
func (t *Tooltip) Hovered() bool { return t != nil && t.hovered }

// FocusNode lazily builds the manager node.
func (t *Tooltip) FocusNode() *focus.FocusNode {
	if t == nil {
		return nil
	}
	if t.focusNode == nil {
		n := focus.NewFocusNode("tooltip:" + t.AriaName())
		n.Enabled = !t.disabled
		n.TabIndex = 0
		self := t
		n.OnFocusChange = func(f bool) {
			self.focused = f
			if f {
				self.Focus()
			} else {
				self.Blur()
			}
			self.markPaint()
		}
		n.OnActivate = func() {
			if self.HasTrigger(TriggerClick) {
				self.Click()
			}
		}
		t.focusNode = n
	}
	return t.focusNode
}

// Node returns the trigger tree node (layout/paint/hit through it).
func (t *Tooltip) Node() rendering.RenderObject {
	if t == nil {
		return nil
	}
	t.syncNode()
	return t.node
}

// Panel returns the bubble chrome node (overlay child + showcase paint).
func (t *Tooltip) Panel() rendering.RenderObject {
	if t == nil {
		return nil
	}
	t.syncNode()
	return t.panelBox
}

// TriggerShell returns the trigger chrome node (same node, single boundary).
func (t *Tooltip) TriggerShell() rendering.RenderObject { return t.Node() }

// Popup returns the positioned bubble view model.
func (t *Tooltip) Popup() *Popup {
	if t == nil {
		return nil
	}
	pw, ph := t.panelSize.Width, t.panelSize.Height
	if pw <= 0 || ph <= 0 {
		pw, ph = t.idealPanel()
	}
	return &Popup{
		Want: t.Placement(), Actual: t.ActualPlacement(), Flipped: t.resolved.Flipped,
		X: t.resolved.X, Y: t.resolved.Y, W: pw, H: ph,
		ArrowX: t.resolved.ArrowX, ArrowY: t.resolved.ArrowY,
	}
}

// OverlayEntry returns the portal entry (nil unless attached and open).
func (t *Tooltip) OverlayEntry() *overlay.Entry {
	if t == nil {
		return nil
	}
	return t.overlayEntry
}

// Resolved returns the last overlay.Resolve outcome.
func (t *Tooltip) Resolved() overlay.Resolved {
	if t == nil {
		return overlay.Resolved{}
	}
	return t.resolved
}

// PopupRect returns the bubble bounds in window coords.
func (t *Tooltip) PopupRect() rendering.Rect {
	p := t.Popup()
	if p == nil {
		return rendering.Rect{}
	}
	return rendering.NewRect(p.X, p.Y, p.W, p.H)
}

// PanelSize returns the last bubble size.
func (t *Tooltip) PanelSize() rendering.Size {
	if t == nil {
		return rendering.Size{}
	}
	return t.panelSize
}

// TriggerSize returns the last trigger size.
func (t *Tooltip) TriggerSize() rendering.Size {
	if t == nil {
		return rendering.Size{}
	}
	return t.lastSize
}

// HitSize is the effective tappable trigger extent (44px floor).
func (t *Tooltip) HitSize() rendering.Size {
	if t == nil {
		return rendering.Size{Width: triggerHitFloor, Height: triggerHitFloor}
	}
	w, h := t.lastSize.Width, t.lastSize.Height
	if w < triggerHitFloor {
		w = triggerHitFloor
	}
	if h < triggerHitFloor {
		h = triggerHitFloor
	}
	return rendering.Size{Width: w, Height: h}
}

// AttachOverlay binds the portal stack (nil detaches).
func (t *Tooltip) AttachOverlay(st *overlay.State) *Tooltip {
	if t == nil {
		return t
	}
	if t.overlayState != nil && t.overlayEntry != nil {
		t.overlayState.Remove(t.overlayEntry)
		t.overlayEntry = nil
	}
	t.overlayState = st
	t.syncOverlay()
	return t
}

// AttachTicker stores the frame owner for delay ticks (nil clears).
func (t *Tooltip) AttachTicker(o *rendering.PipelineOwner) *Tooltip {
	if t == nil {
		return t
	}
	t.tickerOwner = o
	return t
}

// Tick accumulates hover delay; true when a pending open/close fired.
func (t *Tooltip) Tick(dt float64) bool {
	if t == nil || !t.pending {
		return false
	}
	if dt < 0 {
		dt = 0
	}
	t.pendingElapsed += dt
	if t.pendingElapsed < t.pendingDelay {
		return false
	}
	want := t.pendingWant
	t.cancelPending()
	t.applyWant(want)
	return true
}

// Sync refreshes geometry (deprecated: Layout covers it).
func (t *Tooltip) Sync() {
	if t == nil {
		return
	}
	t.refreshGeometry()
	t.markPaint()
	t.syncOverlay()
}

// IsOpen reports visible state (empty/disabled never opens).
func (t *Tooltip) IsOpen() bool {
	if t == nil || !t.hasContent() || t.disabled {
		return false
	}
	if t.controlled {
		return t.controlledOpen
	}
	return t.open
}

// Open reports the same as IsOpen.
func (t *Tooltip) Open() bool { return t.IsOpen() }

// HoverEnter starts the hover path (enter delay applies).
func (t *Tooltip) HoverEnter() {
	if t == nil || !t.HasTrigger(TriggerHover) || t.disabled || !t.hasContent() {
		return
	}
	t.hovered = true
	t.markPaint()
	if t.enterDelay <= 0 {
		t.cancelPending()
		t.applyWant(true)
		return
	}
	t.pending = true
	t.pendingWant = true
	t.pendingElapsed = 0
	t.pendingDelay = t.enterDelay
}

// HoverLeave starts the hover-close path (leave delay applies).
func (t *Tooltip) HoverLeave() {
	if t == nil {
		return
	}
	t.hovered = false
	t.markPaint()
	if !t.HasTrigger(TriggerHover) {
		return
	}
	if t.leaveDelay <= 0 {
		t.cancelPending()
		t.applyWant(false)
		return
	}
	t.pending = true
	t.pendingWant = false
	t.pendingElapsed = 0
	t.pendingDelay = t.leaveDelay
}

// Focus opens immediately on the focus path.
func (t *Tooltip) Focus() {
	if t == nil || !t.HasTrigger(TriggerFocus) || t.disabled || !t.hasContent() {
		return
	}
	t.cancelPending()
	t.applyWant(true)
}

// Blur closes on the focus path.
func (t *Tooltip) Blur() {
	if t == nil || !t.HasTrigger(TriggerFocus) {
		return
	}
	t.cancelPending()
	t.applyWant(false)
}

// Click toggles on the click path.
func (t *Tooltip) Click() bool {
	if t == nil || !t.HasTrigger(TriggerClick) || t.disabled || !t.hasContent() {
		return false
	}
	t.cancelPending()
	want := !t.IsOpen()
	t.applyWant(want)
	return want
}

// ContextMenu opens on the right-click path.
func (t *Tooltip) ContextMenu() bool {
	if t == nil || !t.HasTrigger(TriggerContextMenu) || t.disabled || !t.hasContent() {
		return false
	}
	t.cancelPending()
	t.applyWant(true)
	return true
}

// Escape closes click/focus bubbles (hover-only keeps focus-agnostic).
func (t *Tooltip) Escape() bool {
	if t == nil || !t.IsOpen() {
		return false
	}
	if !t.HasTrigger(TriggerClick) && !t.HasTrigger(TriggerFocus) {
		return false
	}
	t.cancelPending()
	t.applyWant(false)
	return true
}

// PressKey routes Escape/Enter/Space by trigger set.
func (t *Tooltip) PressKey(key string) bool {
	if t == nil {
		return false
	}
	switch key {
	case "Escape", "Esc":
		return t.Escape()
	case "Enter", "Space", " ", "\r":
		if t.HasTrigger(TriggerClick) && (t.focused || t.hovered || t.IsOpen()) {
			t.Click()
			return true
		}
	}
	return false
}

func (t *Tooltip) hasContent() bool {
	return t != nil && (t.title != "" || t.titleNode != nil)
}

func (t *Tooltip) cancelPending() {
	if t == nil {
		return
	}
	t.pending = false
	t.pendingWant = false
	t.pendingElapsed = 0
	t.pendingDelay = 0
}

func (t *Tooltip) applyWant(want bool) {
	if t == nil {
		return
	}
	if want && (!t.hasContent() || t.disabled) {
		return
	}
	if t.controlled {
		if want != t.controlledOpen && t.onOpenChange != nil {
			t.onOpenChange(want)
		}
		return
	}
	if t.open == want {
		return
	}
	t.open = want
	if t.onOpenChange != nil {
		t.onOpenChange(want)
	}
	t.markPaint()
	t.syncOverlay()
}

func (t *Tooltip) themeTokens() theme.Tokens {
	if t != nil && t.override != nil {
		return *t.override
	}
	if t != nil && t.provider != nil {
		return t.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// Background is the bubble fill (spotlight token, preset/#hex on SetColor).
func (t *Tooltip) Background() render.RGBA {
	if t != nil && t.color != "" {
		if bg, ok := colorBackground(strings.TrimSpace(t.color)); ok {
			return bg
		}
	}
	tok := t.themeTokens()
	bg := themeToRGBA(tok.ColorBgSpotlight)
	if bg.A <= 0 {
		bg = render.RGBA{R: 0, G: 0, B: 0, A: 0.85}
	}
	return bg
}

func colorBackground(key string) (render.RGBA, bool) {
	if key == "" {
		return render.RGBA{}, false
	}
	lower := strings.ToLower(strings.TrimSpace(key))
	if strings.HasPrefix(lower, "#") {
		hex := lower
		if len(hex) == 4 || len(hex) == 7 {
			c := theme.Hex(hex)
			if c.A > 0 || c.R > 0 || c.G > 0 || c.B > 0 || hex == "#000" || hex == "#000000" {
				return render.RGBA{R: c.R, G: c.G, B: c.B, A: 1}, true
			}
		}
		return render.RGBA{}, false
	}
	if hx, ok := presetHex[lower]; ok {
		c := theme.Hex(hx)
		return render.RGBA{R: c.R, G: c.G, B: c.B, A: 1}, true
	}
	return render.RGBA{}, false
}

func relLum(r, g, b float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// TextColor is inverse white, flipped to dark text on light customs.
func (t *Tooltip) TextColor() render.RGBA {
	bg := t.Background()
	tok := t.themeTokens()
	white := themeToRGBA(tok.ColorWhite)
	if white.A == 0 {
		white = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	if relLum(bg.R, bg.G, bg.B) > 0.45 {
		return themeToRGBA(tok.ColorText)
	}
	return white
}

// Radius is the bubble corner radius.
func (t *Tooltip) Radius() float64 {
	tok := t.themeTokens()
	if tok.Radius > 0 {
		return tok.Radius
	}
	return DefaultTooltipRadius
}

// FontSize is the bubble font size.
func (t *Tooltip) FontSize() float64 {
	tok := t.themeTokens()
	if tok.FontSize > 0 {
		return tok.FontSize
	}
	return DefaultTooltipFontSize
}

// PadX/PadY are the bubble paddings (8 horizontal, 6 vertical).
func (t *Tooltip) PadX() float64 { return DefaultTooltipPaddingX }

// PadY is the vertical bubble padding.
func (t *Tooltip) PadY() float64 { return DefaultTooltipPaddingY }

// MaxWidth is the bubble clamp.
func (t *Tooltip) MaxWidth() float64 { return DefaultTooltipMaxWidth }

// ArrowSize is the caret edge.
func (t *Tooltip) ArrowSize() float64 { return DefaultTooltipArrowSize }

// Gap is the anchor spacing.
func (t *Tooltip) Gap() float64 { return DefaultTooltipGap }

// TriggerHeight is the default trigger height (controlHeight).
func (t *Tooltip) TriggerHeight() float64 {
	tok := t.themeTokens()
	if tok.ControlHeight > 0 {
		return tok.ControlHeight
	}
	return 32
}

func measureNode(n rendering.RenderObject) (float64, float64) {
	if n == nil {
		return 0, 0
	}
	sz := n.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return sz.Width, sz.Height
}

func (t *Tooltip) idealTrigger() (float64, float64) {
	if t.triggerNode != nil {
		if w, h := measureNode(t.triggerNode); w > 0 && h > 0 {
			return w, h
		}
	}
	if t.triggerLabel != "" {
		w, _ := rendering.EstimateTextSize(t.triggerLabel, t.FontSize(), 0.55)
		return w + 2*15, t.TriggerHeight()
	}
	return 80, t.TriggerHeight()
}

func (t *Tooltip) idealPanel() (float64, float64) {
	px, py := t.PadX(), t.PadY()
	var cw, ch float64
	if t.titleNode != nil {
		cw, ch = measureNode(t.titleNode)
	} else {
		cw, ch = rendering.EstimateTextSize(t.title, t.FontSize(), 0.55)
	}
	w := cw + 2*px
	h := ch + 2*py
	if w > t.MaxWidth() {
		w = t.MaxWidth()
	}
	if w < 2*px+8 {
		w = 2*px + 8
	}
	if h < 2*py+8 {
		h = 2*py + 8
	}
	return w, h
}

// Layout sizes the trigger under constraints (Exact/Min/Max matrix).
func (t *Tooltip) Layout(c rendering.Constraints) rendering.Size {
	if t == nil {
		return rendering.Size{}
	}
	t.syncNode()
	iw, ih := t.idealTrigger()
	out := c.Tighten(rendering.Size{Width: iw, Height: ih})
	t.lastSize = out
	t.node.FixedWidth, t.node.FixedHeight = out.Width, out.Height
	sz := t.node.Layout(c)
	t.lastSize = sz
	pw, ph := t.idealPanel()
	t.panelSize = rendering.Size{Width: pw, Height: ph}
	t.panelBox.FixedWidth, t.panelBox.FixedHeight = pw, ph
	t.panelBox.Layout(rendering.Tight(pw, ph))
	t.refreshGeometry()
	t.syncOverlay()
	return sz
}

func (t *Tooltip) refreshGeometry() {
	if t == nil {
		return
	}
	tw, th := t.lastSize.Width, t.lastSize.Height
	if tw <= 0 || th <= 0 {
		tw, th = t.idealTrigger()
	}
	pw, ph := t.panelSize.Width, t.panelSize.Height
	if pw <= 0 || ph <= 0 {
		pw, ph = t.idealPanel()
		t.panelSize = rendering.Size{Width: pw, Height: ph}
	}
	anchor := rendering.NewRect(t.anchorX, t.anchorY, tw, th)
	vw, vh := t.viewportW, t.viewportH
	if vw <= 0 {
		vw = 1200
	}
	if vh <= 0 {
		vh = 800
	}
	t.resolved = overlay.Resolve(anchor, pw, ph, overlay.Placement(t.EffectivePlacement()), &overlay.ResolveOptions{
		Gap: t.Gap(), Flip: t.autoAdjust, Shift: t.autoAdjust,
		ViewportW: vw, ViewportH: vh, ArrowPointAtCenter: t.arrowCenter,
	})
}

func (t *Tooltip) syncOverlay() {
	if t == nil || t.overlayState == nil {
		return
	}
	if t.IsOpen() {
		pw, ph := t.panelSize.Width, t.panelSize.Height
		if pw <= 0 || ph <= 0 {
			pw, ph = t.idealPanel()
		}
		if t.overlayEntry != nil {
			t.overlayState.Remove(t.overlayEntry)
			t.overlayEntry = nil
		}
		t.panelBox.FixedWidth, t.panelBox.FixedHeight = pw, ph
		t.panelBox.Layout(rendering.Tight(pw, ph))
		e := overlay.NewEntry(t.panelBox, t.resolved.X, t.resolved.Y, pw, ph)
		t.overlayState.Insert(e)
		t.overlayEntry = e
		return
	}
	if t.overlayEntry != nil {
		t.overlayState.Remove(t.overlayEntry)
		t.overlayEntry = nil
	}
}

func (t *Tooltip) markPaint() {
	if t == nil {
		return
	}
	if t.node != nil {
		t.node.MarkNeedsPaint()
	}
	if t.panelBox != nil {
		t.panelBox.MarkNeedsPaint()
	}
}

func (t *Tooltip) relayout() {
	if t == nil {
		return
	}
	if t.node != nil {
		t.node.MarkNeedsLayout()
		t.node.MarkNeedsPaint()
	}
	if t.panelBox != nil {
		t.panelBox.MarkNeedsLayout()
		t.panelBox.MarkNeedsPaint()
	}
}

func (t *Tooltip) paintText(pc *rendering.PaintContext, s string, x, y, w, h, fontSize float64, c render.RGBA) {
	if pc == nil || pc.DC == nil || s == "" || w <= 0 || h <= 0 {
		return
	}
	if t == nil || t.textFace == nil {
		return
	}
	pc.DC.SetFont(t.textFace)
	pc.DC.SetRGBA(c.R, c.G, c.B, c.A)
	baseline := y + h/2 + fontSize*0.35
	ax, ay := pc.Abs(x, baseline)
	pc.DC.DrawString(s, ax, ay)
}

func (t *Tooltip) paintTrigger(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || t == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	tok := t.themeTokens()
	w, h := size.Width, size.Height
	bg := themeToRGBA(tok.ColorBgContainer)
	if bg.A == 0 {
		bg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	bd := themeToRGBA(tok.ColorBorder)
	tx := themeToRGBA(tok.ColorText)
	if t.disabled {
		bg = themeToRGBA(tok.ColorFillTertiary)
		tx = themeToRGBA(tok.ColorTextDisabled)
	}
	radius := tok.Radius
	if radius <= 0 {
		radius = DefaultTooltipRadius
	}
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, bg.R, bg.G, bg.B, bg.A)
	lw := tok.LineWidth
	if lw <= 0 {
		lw = 1
	}
	bc := bd
	if t.hovered && !t.disabled {
		prim := themeToRGBA(tok.ColorPrimary)
		if prim.A > 0 {
			bc = prim
		}
	}
	rendering.StrokeRoundRect(pc, lw/2, lw/2, w-lw, h-lw, radius, lw, bc.R, bc.G, bc.B, bc.A)
	if t.triggerNode != nil {
		t.triggerNode.Paint(pc.WithOrigin(pc.OriginX, pc.OriginY))
	} else {
		label := t.triggerLabel
		if label == "" {
			label = "trigger"
		}
		t.paintText(pc, label, 8, 0, w-16, h, t.FontSize(), tx)
	}
	if t.focused && !t.disabled {
		ring := themeToRGBA(tok.ColorPrimary)
		if ring.A == 0 {
			ring = render.RGBA{R: 0x16 / 255.0, G: 0x77 / 255.0, B: 0xff / 255.0, A: 1}
		}
		rx, ry, rw, rh := focus.FocusRingRect(0, 0, w, h, 1.5)
		ow := tok.ControlOutlineWidth
		if ow <= 0 {
			ow = 2
		}
		rendering.StrokeRoundRect(pc, rx, ry, rw, rh, radius+1.5, ow, ring.R, ring.G, ring.B, ring.A)
	}
}

func (t *Tooltip) paintPanel(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || t == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	if !t.hasContent() {
		return
	}
	w, h := size.Width, size.Height
	bg := t.Background()
	fg := t.TextColor()
	rendering.FillRoundRect(pc, 0, 0, w, h, t.Radius(), bg.R, bg.G, bg.B, bg.A)
	px, py := t.PadX(), t.PadY()
	if t.titleNode != nil {
		t.titleNode.Paint(pc.WithOrigin(pc.OriginX+px, pc.OriginY+py))
	} else {
		t.paintText(pc, t.title, px, py, w-2*px, h-2*py, t.FontSize(), fg)
	}
	if t.arrow {
		t.paintArrow(pc, w, h, bg)
	}
}

func (t *Tooltip) paintArrow(pc *rendering.PaintContext, w, h float64, c render.RGBA) {
	ax, ay := t.resolved.ArrowX, t.resolved.ArrowY
	if ax <= 0 || ay < 0 {
		ax = w / 2
	}
	s := t.ArrowSize() / 2
	if s <= 0 {
		s = 4
	}
	var pts []rendering.Point
	switch mainSide(t.EffectivePlacement()) {
	case "top":
		cx := clampF(ax, s, w-s)
		pts = []rendering.Point{{X: cx - s, Y: h - 0.5}, {X: cx + s, Y: h - 0.5}, {X: cx, Y: h + s}}
	case "bottom":
		cx := clampF(ax, s, w-s)
		pts = []rendering.Point{{X: cx - s, Y: 0.5}, {X: cx + s, Y: 0.5}, {X: cx, Y: -s}}
	case "left":
		cy := clampF(ay, s, h-s)
		pts = []rendering.Point{{X: w - 0.5, Y: cy - s}, {X: w - 0.5, Y: cy + s}, {X: w + s, Y: cy}}
	default:
		cy := clampF(ay, s, h-s)
		pts = []rendering.Point{{X: 0.5, Y: cy - s}, {X: 0.5, Y: cy + s}, {X: -s, Y: cy}}
	}
	rendering.DrawVertices(pc, pts, []rendering.ColorRGBA{
		{R: c.R, G: c.G, B: c.B, A: c.A},
		{R: c.R, G: c.G, B: c.B, A: c.A},
		{R: c.R, G: c.G, B: c.B, A: c.A},
	}, rendering.VertexModeTriangles)
}

func mainSide(p TooltipPlacement) string {
	switch p {
	case Top, TopLeft, TopRight:
		return "top"
	case Bottom, BottomLeft, BottomRight:
		return "bottom"
	case Left, LeftTop, LeftBottom:
		return "left"
	default:
		return "right"
	}
}

func clampF(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
