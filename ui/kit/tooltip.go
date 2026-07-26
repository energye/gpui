package kit

import (
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Tooltip defaults — components/tooltip/style + docs/antd/tooltip.md §6.2.
const (
	DefaultTooltipFontSize        = 14.0
	DefaultTooltipPaddingX        = 8.0 // antd paddingXS
	DefaultTooltipPaddingY        = 6.0 // antd paddingSM/2
	DefaultTooltipMaxWidth        = 250.0
	DefaultTooltipMinHeight       = 32.0 // controlHeight
	DefaultTooltipArrowSize       = 8.0
	DefaultTooltipGap             = 8.0
	DefaultTooltipPortalZ         = 1070 // zIndexPopupBase + 70
	DefaultTooltipFocusOutset     = 1.5
	DefaultTooltipMouseEnterDelay = 0.1 // seconds
	DefaultTooltipMouseLeaveDelay = 0.1
)

// tooltipSpotlightBG is antd light colorBgSpotlight ≈ rgba(0,0,0,0.85).
var tooltipSpotlightBG = render.RGBA{R: 0, G: 0, B: 0, A: 0.85}

// TooltipTrigger is an open trigger mode (antd trigger[]).
type TooltipTrigger int

const (
	// TooltipTriggerHover is the antd default.
	TooltipTriggerHover TooltipTrigger = iota
	TooltipTriggerClick
	TooltipTriggerFocus
	TooltipTriggerContextMenu
)

// TooltipPlacement is antd placement (12-way). Default: Top.
type TooltipPlacement int

const (
	TooltipTop TooltipPlacement = iota // default (antd)
	TooltipTopLeft
	TooltipTopRight
	TooltipBottom
	TooltipBottomLeft
	TooltipBottomRight
	TooltipLeft
	TooltipLeftTop
	TooltipLeftBottom
	TooltipRight
	TooltipRightTop
	TooltipRightBottom
)

// Tooltip is Ant Design Tooltip: trigger + anchored title bubble.
//
//	Column
//	  ├─ shell Pressable (trigger)
//	  └─ AnchoredPopup
//	       └─ panel (+ optional arrow) / title
//
// Product contract: docs/antd/tooltip.md §6 (P0 DoD).
type Tooltip struct {
	Wrap  *primitive.Flex
	shell *primitive.Pressable
	popup *primitive.AnchoredPopup
	panel *primitive.Decorated
	arrow *primitive.Canvas
	btn   *Button // default trigger when no custom node

	titleLab *primitive.Text
	titleN   core.Node

	// Title is plain-text tip when TitleNode is nil.
	Title string
	// TitleNode optional custom title (overrides Title string).
	TitleNode core.Node
	// TriggerLabel is the default trigger text when TriggerNode is nil.
	TriggerLabel string
	// TriggerNode optional custom trigger (overrides label button / text).
	TriggerNode core.Node

	// Placement default Top (antd).
	Placement TooltipPlacement
	// Triggers empty → [hover] (antd default).
	Triggers []TooltipTrigger
	// Arrow show / pointAtCenter (antd default true).
	Arrow bool
	// ArrowPointAtCenter when Arrow and true (antd arrow={{ pointAtCenter }}).
	ArrowPointAtCenter bool
	// AutoAdjustOverflow default true (AnchoredPopup flip/shift).
	AutoAdjustOverflow bool
	// Color is preset name or #hex (empty = spotlight default skin).
	Color string
	// Disabled blocks open (antd demos often use empty title instead).
	Disabled bool
	// Open is the current visibility.
	Open bool
	// ZIndex overrides Portal.ZOrder when > 0.
	ZIndex int
	// MouseEnterDelay seconds before open on hover (antd default 0.1).
	MouseEnterDelay float64
	// MouseLeaveDelay seconds before close on leave (antd default 0.1).
	MouseLeaveDelay float64
	// enterDelaySet / leaveDelaySet track explicit zero vs unset.
	enterDelaySet bool
	leaveDelaySet bool

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

	// delay state (Ticker)
	pendingOpen  bool // intent while waiting enter delay
	pendingClose bool // intent while waiting leave delay
	enterAcc     float64
	leaveAcc     float64
	boundTree    *core.Tree
	tickerBound  bool
}

// NewTooltip creates a Tooltip with the given tip title.
// Defaults (§6.10): trigger=[hover], placement=top, arrow=true, delays=0.1, closed, uncontrolled.
func NewTooltip(title string) *Tooltip {
	tt := &Tooltip{
		Title:              title,
		Placement:          TooltipTop,
		Arrow:              true,
		AutoAdjustOverflow: true,
		MouseEnterDelay:    DefaultTooltipMouseEnterDelay,
		MouseLeaveDelay:    DefaultTooltipMouseLeaveDelay,
	}
	tt.rebuild()
	return tt
}

// Node returns the composition root (trigger + popup host).

// ensureBuilt materializes the control tree if missing (#9).
func (tt *Tooltip) ensureBuilt() {
	if tt == nil {
		return
	}
	if tt.Wrap == nil {
		tt.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (tt *Tooltip) structureChange() {
	if tt == nil {
		return
	}
	tt.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (tt *Tooltip) chromeChange() {
	if tt == nil {
		return
	}
	tt.ensureBuilt()
	tt.rebuild()
}

func (tt *Tooltip) Node() core.Node {
	if tt == nil {
		return nil
	}
	tt.ensureBuilt()
	return tt.Wrap
}

// Popup returns the anchored popup (tests / advanced hosts).
func (tt *Tooltip) Popup() *primitive.AnchoredPopup {
	if tt == nil {
		return nil
	}
	return tt.popup
}

// IsOpen reports whether the tip is visible.
func (tt *Tooltip) IsOpen() bool {
	return tt != nil && tt.Open
}

// Panel returns the tip chrome (tests).
func (tt *Tooltip) Panel() *primitive.Decorated {
	if tt == nil {
		return nil
	}
	return tt.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (tt *Tooltip) TriggerShell() *primitive.Pressable {
	if tt == nil {
		return nil
	}
	return tt.shell
}

// TitleRoot returns the resolved title node (tests).
func (tt *Tooltip) TitleRoot() core.Node {
	if tt == nil {
		return nil
	}
	return tt.titleN
}

// SetTitle sets plain-text tip (clears TitleNode). Empty → never open (antd).
func (tt *Tooltip) SetTitle(title string) {
	if tt == nil {
		return
	}
	tt.Title = title
	tt.TitleNode = nil
	tt.rebuildPanel()
	if tt.Open {
		if !tt.hasTitle() {
			tt.applyOpen(false, false)
		} else {
			tt.measurePanel()
			tt.syncPopupGeometry()
		}
	}
}

// SetTitleNode sets a custom title node (nil falls back to Title string).
func (tt *Tooltip) SetTitleNode(n core.Node) {
	if tt == nil {
		return
	}
	tt.TitleNode = n
	tt.rebuildPanel()
	if tt.Open {
		if !tt.hasTitle() {
			tt.applyOpen(false, false)
		} else {
			tt.measurePanel()
			tt.syncPopupGeometry()
		}
	}
}

// SetTriggerLabel sets the default button trigger text.
func (tt *Tooltip) SetTriggerLabel(label string) {
	if tt == nil {
		return
	}
	tt.TriggerLabel = label
	if tt.TriggerNode == nil {
		tt.rebuild()
	} else if tt.btn != nil {
		tt.btn.SetLabel(label)
	}
	tt.applyA11y()
}

// SetTriggerNode sets a custom trigger node (nil restores default label trigger).
func (tt *Tooltip) SetTriggerNode(n core.Node) {
	if tt == nil {
		return
	}
	tt.TriggerNode = n
	tt.rebuild()
}

// SetTriggerModes sets open triggers (empty → hover default).
func (tt *Tooltip) SetTriggerModes(modes ...TooltipTrigger) {
	if tt == nil {
		return
	}
	tt.Triggers = append([]TooltipTrigger(nil), modes...)
	tt.wireTrigger()
}

// SetTrigger is a single-mode convenience (overwrites Triggers).
func (tt *Tooltip) SetTrigger(mode TooltipTrigger) {
	tt.SetTriggerModes(mode)
}

// SetPlacement sets popup placement.
func (tt *Tooltip) SetPlacement(pl TooltipPlacement) {
	if tt == nil {
		return
	}
	tt.Placement = pl
	if tt.popup != nil {
		tt.popup.Placement = mapTooltipPlacement(pl, tt.ArrowPointAtCenter)
		if tt.Open {
			tt.syncPopupGeometry()
		}
	}
	if tt.Arrow {
		tt.rebuildPanel()
	}
}

// SetArrow toggles the arrow indicator.
func (tt *Tooltip) SetArrow(show bool) {
	if tt == nil {
		return
	}
	tt.Arrow = show
	tt.rebuildPanel()
}

// SetArrowConfig sets arrow show + pointAtCenter (antd arrow object).
func (tt *Tooltip) SetArrowConfig(show, pointAtCenter bool) {
	if tt == nil {
		return
	}
	tt.Arrow = show
	tt.ArrowPointAtCenter = pointAtCenter
	if tt.popup != nil {
		tt.popup.Placement = mapTooltipPlacement(tt.Placement, tt.ArrowPointAtCenter)
	}
	tt.rebuildPanel()
}

// SetAutoAdjustOverflow enables flip/shift (default true).
func (tt *Tooltip) SetAutoAdjustOverflow(v bool) {
	if tt == nil {
		return
	}
	tt.AutoAdjustOverflow = v
	tt.applyViewportToPopup()
}

// SetColor sets preset name or #hex background ("" restores spotlight default).
func (tt *Tooltip) SetColor(c string) {
	if tt == nil {
		return
	}
	tt.Color = c
	tt.rebuildPanel()
}

// SetDisabled disables the tooltip (cannot open).
func (tt *Tooltip) SetDisabled(v bool) {
	if tt == nil {
		return
	}
	tt.Disabled = v
	if tt.shell != nil {
		tt.shell.SetDisabled(v)
	}
	if tt.btn != nil {
		tt.btn.SetDisabled(v)
	}
	if v && tt.Open {
		tt.applyOpen(false, false)
	}
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (tt *Tooltip) SetOpen(open bool) {
	if tt == nil {
		return
	}
	tt.openControlled = true
	tt.clearDelay()
	tt.applyOpen(open, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (tt *Tooltip) SetDefaultOpen(open bool) {
	if tt == nil || tt.openControlled {
		return
	}
	tt.defaultOpen = open
	tt.defaultOpenSet = true
	if !tt.appliedDefault {
		tt.appliedDefault = true
		tt.applyOpen(open, false)
	}
}

// SetOnOpenChange sets the open-change callback.
func (tt *Tooltip) SetOnOpenChange(fn func(open bool)) {
	if tt == nil {
		return
	}
	tt.OnOpenChange = fn
}

// SetZIndex sets Portal.ZOrder (0 keeps default).
func (tt *Tooltip) SetZIndex(z int) {
	if tt == nil {
		return
	}
	tt.ZIndex = z
	if tt.popup != nil && tt.popup.Portal != nil {
		if z > 0 {
			tt.popup.Portal.ZOrder = z
		} else {
			tt.popup.Portal.ZOrder = DefaultTooltipPortalZ
		}
	}
}

// SetMouseEnterDelay sets hover open delay in seconds (antd mouseEnterDelay).
func (tt *Tooltip) SetMouseEnterDelay(sec float64) {
	if tt == nil {
		return
	}
	if sec < 0 {
		sec = 0
	}
	tt.MouseEnterDelay = sec
	tt.enterDelaySet = true
}

// SetMouseLeaveDelay sets hover close delay in seconds (antd mouseLeaveDelay).
func (tt *Tooltip) SetMouseLeaveDelay(sec float64) {
	if tt == nil {
		return
	}
	if sec < 0 {
		sec = 0
	}
	tt.MouseLeaveDelay = sec
	tt.leaveDelaySet = true
}

// SetTheme sets an explicit theme override.
func (tt *Tooltip) SetTheme(th *core.Theme) {
	if tt == nil {
		return
	}
	tt.Theme = th
	tt.rebuild()
}

// SetFace sets the font face for labels.
func (tt *Tooltip) SetFace(face text.Face) {
	if tt == nil {
		return
	}
	tt.Face = face
	tt.rebuild()
}

// SetAriaLabel sets the accessible name on the trigger.
func (tt *Tooltip) SetAriaLabel(name string) {
	if tt == nil {
		return
	}
	tt.AriaLabel = name
	tt.applyA11y()
}

// AttachTicker registers this tooltip as a Tree ticker for delay resolution.
func (tt *Tooltip) AttachTicker(t *core.Tree) {
	if tt == nil || t == nil {
		return
	}
	tt.boundTree = t
	t.AddTicker(tt)
	tt.tickerBound = true
}

// Tick implements core.Ticker — resolves delayed open/close.
func (tt *Tooltip) Tick(dt float64) bool {
	if tt == nil {
		return false
	}
	if tt.pendingOpen {
		tt.enterAcc += dt
		if tt.enterAcc >= tt.enterDelay() {
			tt.pendingOpen = false
			tt.enterAcc = 0
			if tt.shell != nil && tt.shell.State.Hovered && tt.hasTrigger(TooltipTriggerHover) {
				tt.requestOpen(true)
			}
		}
		return tt.pendingOpen || tt.pendingClose
	}
	if tt.pendingClose {
		tt.leaveAcc += dt
		if tt.leaveAcc >= tt.leaveDelay() {
			tt.pendingClose = false
			tt.leaveAcc = 0
			if tt.shell == nil || !tt.shell.State.Hovered {
				if !tt.hasTrigger(TooltipTriggerFocus) || tt.shell == nil || !tt.shell.State.Focused {
					tt.requestOpen(false)
				}
			}
		}
		return tt.pendingOpen || tt.pendingClose
	}
	return false
}

// Sync repositions while open.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry.
func (tt *Tooltip) Sync() {
	if tt != nil && tt.Open {
		tt.syncPopupGeometry()
		if tt.popup != nil {
			tt.popup.SetOpen(true)
		}
	}
}

// Metrics returns resolved L2 geometry (tests / §6.2).
func (tt *Tooltip) Metrics() (padX, padY, radius, fontSize, maxW float64) {
	th := tt.theme()
	padX = DefaultTooltipPaddingX
	padY = DefaultTooltipPaddingY
	radius = th.SizeOr(core.TokenBorderRadius, 6)
	fontSize = th.SizeOr(core.TokenFontSize, DefaultTooltipFontSize)
	maxW = DefaultTooltipMaxWidth
	return
}

// PanelBackground returns the resolved tip fill (tests).
func (tt *Tooltip) PanelBackground() render.RGBA {
	if tt == nil || tt.panel == nil {
		return render.RGBA{}
	}
	return tt.panel.Background
}

func (tt *Tooltip) theme() *core.Theme {
	var n core.Node
	if tt.Wrap != nil {
		n = tt.Wrap
	}
	return themeOf(tt.Theme, n)
}

func (tt *Tooltip) hasTitle() bool {
	if tt == nil {
		return false
	}
	if tt.TitleNode != nil {
		return true
	}
	return strings.TrimSpace(tt.Title) != ""
}

func (tt *Tooltip) hasTrigger(mode TooltipTrigger) bool {
	if tt == nil {
		return false
	}
	modes := tt.Triggers
	if len(modes) == 0 {
		modes = []TooltipTrigger{TooltipTriggerHover}
	}
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

func (tt *Tooltip) enterDelay() float64 {
	if tt == nil {
		return DefaultTooltipMouseEnterDelay
	}
	return tt.MouseEnterDelay
}

func (tt *Tooltip) leaveDelay() float64 {
	if tt == nil {
		return DefaultTooltipMouseLeaveDelay
	}
	return tt.MouseLeaveDelay
}

func (tt *Tooltip) clearDelay() {
	tt.pendingOpen = false
	tt.pendingClose = false
	tt.enterAcc = 0
	tt.leaveAcc = 0
}

func (tt *Tooltip) ensureTicker() {
	if tt == nil {
		return
	}
	if tt.boundTree != nil {
		if !tt.tickerBound {
			tt.boundTree.AddTicker(tt)
			tt.tickerBound = true
		}
		return
	}
	if tt.shell != nil {
		if t := tt.shell.Tree(); t != nil {
			tt.boundTree = t
			t.AddTicker(tt)
			tt.tickerBound = true
		}
	}
}

func (tt *Tooltip) rebuild() {
	th := tt.theme()
	wasOpen := tt.Open

	var trigger core.Node
	if tt.TriggerNode != nil {
		trigger = tt.TriggerNode
		tt.btn = nil
	} else {
		label := tt.TriggerLabel
		if label == "" {
			label = "Hover me"
		}
		tt.btn = NewButton(label)
		tt.btn.SetFace(tt.Face)
		tt.btn.Theme = tt.Theme
		if tt.Disabled {
			tt.btn.SetDisabled(true)
		}
		trigger = tt.btn.Node()
	}

	tt.shell = primitive.NewPressable(trigger)
	tt.shell.Focusable = true
	tt.shell.FocusRingRadius = th.SizeOr(core.TokenBorderRadius, 6)
	tt.shell.SetDisabled(tt.Disabled)
	tt.applyA11y()

	tt.rebuildPanel()

	tt.popup = primitive.NewAnchoredPopup(tt.panel)
	tt.popup.Placement = mapTooltipPlacement(tt.Placement, tt.ArrowPointAtCenter)
	tt.popup.Gap = DefaultTooltipGap
	tt.popup.Portal.ID = ""
	// Hover-only: no outside dismiss; click/focus may dismiss outside.
	tt.popup.DismissOnOutside = tt.hasTrigger(TooltipTriggerClick) || tt.hasTrigger(TooltipTriggerContextMenu)
	if tt.ZIndex > 0 {
		tt.popup.Portal.ZOrder = tt.ZIndex
	} else {
		tt.popup.Portal.ZOrder = DefaultTooltipPortalZ
	}
	tt.popup.OnDismiss = func() {
		tt.requestOpen(false)
		if tt.openControlled && tt.Open {
			tt.popup.SetOpen(true)
		}
	}
	tt.applyViewportToPopup()

	if tt.Wrap == nil {
		tt.Wrap = primitive.Column(tt.shell, tt.popup)
	} else {
		tt.Wrap.ClearChildren()
		tt.Wrap.AddChild(tt.shell)
		tt.Wrap.AddChild(tt.popup)
	}
	tt.Wrap.CrossAlign = core.CrossStart
	tt.Wrap.SetThemeHook(func(*core.Theme) { tt.rebuild() })

	tt.wireTrigger()

	if wasOpen || (tt.defaultOpenSet && !tt.openControlled && tt.defaultOpen && !tt.appliedDefault) {
		tt.appliedDefault = true
		tt.applyOpen(true, false)
	}

	tt.Wrap.MarkNeedsLayout()
	tt.Wrap.MarkNeedsPaint()
}

func (tt *Tooltip) applyA11y() {
	if tt.shell == nil {
		return
	}
	name := tt.AriaLabel
	if name == "" {
		name = tt.TriggerLabel
	}
	if name == "" {
		name = tt.Title
	}
	if name == "" {
		name = "tooltip"
	}
	tt.shell.Base().Role = "button"
	tt.shell.Base().Label = name
	if tt.panel != nil {
		tt.panel.Base().Role = "tooltip"
		tipName := tt.Title
		if tipName == "" {
			tipName = name
		}
		tt.panel.Base().Label = tipName
	}
}

func (tt *Tooltip) wireTrigger() {
	if tt.shell == nil {
		return
	}
	tt.shell.Click = func() {
		if tt.Disabled || !tt.hasTitle() {
			return
		}
		if tt.hasTrigger(TooltipTriggerClick) {
			tt.clearDelay()
			tt.requestOpen(!tt.Open)
		}
	}
	tt.shell.OnStateChange = func() {
		if tt.Disabled {
			return
		}
		// Hover
		if tt.hasTrigger(TooltipTriggerHover) {
			if tt.shell.State.Hovered {
				tt.pendingClose = false
				tt.leaveAcc = 0
				d := tt.enterDelay()
				if d <= 0 {
					tt.pendingOpen = false
					tt.enterAcc = 0
					tt.requestOpen(true)
				} else {
					tt.pendingOpen = true
					tt.enterAcc = 0
					tt.ensureTicker()
				}
			} else if !tt.hasTrigger(TooltipTriggerFocus) || !tt.shell.State.Focused {
				tt.pendingOpen = false
				tt.enterAcc = 0
				d := tt.leaveDelay()
				if d <= 0 {
					tt.pendingClose = false
					tt.leaveAcc = 0
					tt.requestOpen(false)
				} else {
					tt.pendingClose = true
					tt.leaveAcc = 0
					tt.ensureTicker()
				}
			}
		}
		// Focus
		if tt.hasTrigger(TooltipTriggerFocus) {
			if tt.shell.State.Focused {
				tt.clearDelay()
				tt.requestOpen(true)
			} else if !tt.hasTrigger(TooltipTriggerHover) || !tt.shell.State.Hovered {
				tt.clearDelay()
				tt.requestOpen(false)
			}
		}
	}
	// Re-apply dismiss mode when triggers change.
	if tt.popup != nil {
		tt.popup.DismissOnOutside = tt.hasTrigger(TooltipTriggerClick) || tt.hasTrigger(TooltipTriggerContextMenu)
	}
}

func (tt *Tooltip) rebuildPanel() {
	th := tt.theme()
	fs := th.SizeOr(core.TokenFontSize, DefaultTooltipFontSize)
	bg, fg := tt.resolveColors()

	var body core.Node
	if tt.TitleNode != nil {
		body = tt.TitleNode
		tt.titleLab = nil
	} else if strings.TrimSpace(tt.Title) != "" {
		tt.titleLab = primitive.NewText(tt.Title)
		tt.titleLab.FontSize = fs
		tt.titleLab.Face = tt.Face
		tt.titleLab.Color = fg
		body = tt.titleLab
	} else {
		tt.titleLab = nil
		// Empty placeholder keeps measure stable when forced open in tests.
		tx := primitive.NewText("")
		tx.FontSize = fs
		body = tx
	}
	tt.titleN = body

	inner := body
	if tt.Arrow {
		sz := DefaultTooltipArrowSize
		// Solid caret block matching panel fill (P0 geometry indicator).
		tt.arrow = primitive.NewCanvas(sz, sz/2+1, func(pc *core.PaintContext, size core.Size) {
			if pc == nil {
				return
			}
			mid := size.Width / 2
			pc.FillLocalRect(mid-1, 0, 2, size.Height, bg)
			pc.FillLocalRect(mid-3, size.Height*0.35, 6, size.Height*0.65, bg)
			pc.FillLocalRect(0, size.Height-2, size.Width, 2, bg)
		})
		col := primitive.Column()
		col.CrossAlign = core.CrossCenter
		col.Gap = 0
		switch tt.Placement {
		case TooltipBottom, TooltipBottomLeft, TooltipBottomRight:
			col.AddChild(tt.arrow)
			col.AddChild(body)
		case TooltipTop, TooltipTopLeft, TooltipTopRight:
			col.AddChild(body)
			col.AddChild(tt.arrow)
		default:
			col.AddChild(tt.arrow)
			col.AddChild(body)
		}
		inner = col
	} else {
		tt.arrow = nil
	}

	tt.panel = primitive.NewDecorated(inner)
	tt.panel.SkinType = TypeTooltip
	tt.panel.Padding = primitive.Symmetric(DefaultTooltipPaddingX, DefaultTooltipPaddingY)
	tt.panel.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	tt.panel.Background = bg
	tt.panel.MinHeight = th.SizeOr(core.TokenControlHeight, DefaultTooltipMinHeight)
	tt.panel.Base().Role = "tooltip"
	tt.applyA11y()

	if tt.popup != nil {
		tt.popup.Content = tt.panel
		if tt.popup.Portal != nil {
			tt.popup.Portal.Content = tt.panel
		}
	}
}

func (tt *Tooltip) resolveColors() (bg, fg render.RGBA) {
	th := tt.theme()
	fg = th.Color(core.TokenColorTextInverse)
	if fg.A == 0 {
		fg = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	// Try optional spotlight token key (may be absent).
	bg = th.Color("colorBgSpotlight")
	if bg.A == 0 {
		bg = tooltipSpotlightBG
	}

	c := strings.TrimSpace(tt.Color)
	if c == "" {
		return bg, fg
	}
	if p := tagPreset(c); p != nil {
		return p.dark, fg
	}
	// Extra presets used by antd colorful demo (not all in Tag).
	if extra := tooltipExtraPreset(c); extra.A > 0 {
		return extra, fg
	}
	if isHexColor(c) {
		custom := render.Hex(c)
		if custom.A == 0 {
			return bg, fg
		}
		// Adaptive text: light bg → dark text.
		if tooltipLuma(custom) > 0.6 {
			return custom, render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
		}
		return custom, fg
	}
	return bg, fg
}

func tooltipExtraPreset(name string) render.RGBA {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "pink":
		return render.Hex("#EB2F96")
	case "yellow":
		return render.Hex("#FADB14")
	default:
		return render.RGBA{}
	}
}

func tooltipLuma(c render.RGBA) float64 {
	// relative luminance approx on sRGB channels already 0..1
	return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B
}

func (tt *Tooltip) requestOpen(open bool) {
	if tt == nil {
		return
	}
	if open && (tt.Disabled || !tt.hasTitle()) {
		return
	}
	if tt.openControlled {
		if tt.Open != open && tt.OnOpenChange != nil {
			tt.OnOpenChange(open)
		}
		return
	}
	if tt.Open == open {
		return
	}
	tt.applyOpen(open, true)
}

func (tt *Tooltip) applyOpen(open bool, notify bool) {
	if tt == nil {
		return
	}
	if open && (tt.Disabled || !tt.hasTitle()) {
		return
	}
	prev := tt.Open
	tt.Open = open
	if tt.popup != nil {
		if open {
			tt.measurePanel()
			tt.syncPopupGeometry()
		}
		tt.popup.SetOpen(open)
	}
	if notify && prev != open && tt.OnOpenChange != nil {
		tt.OnOpenChange(open)
	}
	if tt.Wrap != nil {
		tt.Wrap.MarkNeedsLayout()
		tt.Wrap.MarkNeedsPaint()
	}
}

func (tt *Tooltip) measurePanel() {
	if tt.panel != nil {
		_ = tt.panel.Layout(core.Loose(DefaultTooltipMaxWidth, 400))
	}
}

func (tt *Tooltip) syncPopupGeometry() {
	if tt.popup == nil {
		return
	}
	tt.popup.UpdateAnchorFromNode(tt.shell)
	tt.applyViewportToPopup()
}

func (tt *Tooltip) applyViewportToPopup() {
	if tt.popup == nil {
		return
	}
	if !tt.AutoAdjustOverflow {
		tt.popup.Viewport = core.Size{}
		return
	}
	if tt.Viewport.Width > 0 {
		tt.popup.Viewport = tt.Viewport
	}
}

func mapTooltipPlacement(pl TooltipPlacement, pointAtCenter bool) primitive.Placement {
	// Reuse Popover mapping via cast-compatible enum values.
	return mapPopoverPlacement(PopoverPlacement(pl), pointAtCenter)
}
