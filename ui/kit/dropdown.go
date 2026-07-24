package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Dropdown defaults — components/dropdown/style + docs/antd/dropdown.md §6.2.
const (
	DefaultDropdownGap           = 4.0
	DefaultDropdownMenuMinWidth  = 160.0
	DefaultDropdownPanelPad      = 4.0  // paddingXXS / dropdownEdgeChildPadding
	DefaultDropdownItemPadInline = 12.0 // controlPaddingHorizontal
	DefaultDropdownItemPadBlock  = 5.0  // paddingBlock
	DefaultDropdownArrowSize     = 8.0
	DefaultDropdownFontSize      = 14.0
)

// DropdownTrigger is an open trigger mode (antd trigger[]).
type DropdownTrigger int

const (
	// DropdownTriggerHover is the antd default.
	DropdownTriggerHover DropdownTrigger = iota
	DropdownTriggerClick
	DropdownTriggerContextMenu
)

// DropdownPlacement is antd placement (12-way).
type DropdownPlacement int

const (
	DropdownBottomLeft DropdownPlacement = iota // default
	DropdownBottom
	DropdownBottomRight
	DropdownTopLeft
	DropdownTop
	DropdownTopRight
	DropdownLeftTop
	DropdownLeft
	DropdownLeftBottom
	DropdownRightTop
	DropdownRight
	DropdownRightBottom
)

// DropdownOpenSource is onOpenChange info.source (antd 5.11+).
type DropdownOpenSource string

const (
	DropdownSourceTrigger DropdownOpenSource = "trigger"
	DropdownSourceMenu    DropdownOpenSource = "menu"
)

// Dropdown is Ant Design Dropdown: trigger + anchored menu popup.
//
//	Column
//	  ├─ pointer host (contextMenu) / trigger shell (Pressable)
//	  └─ AnchoredPopup
//	       └─ FocusScope (Esc)
//	            └─ panel (+ optional arrow) / menu items
//
// Product contract: docs/antd/dropdown.md §6 (P0 DoD).
type Dropdown struct {
	Wrap  *primitive.Flex
	shell *primitive.Pressable
	popup *primitive.AnchoredPopup
	scope *primitive.FocusScope
	panel *primitive.Decorated
	list  *primitive.Flex
	arrow *primitive.Canvas
	btn   *Button // default text trigger when no custom node

	// Items is the menu model (antd menu.items).
	Items []MenuItem
	// TriggerLabel is the default trigger text when TriggerNode is nil.
	TriggerLabel string
	// TriggerNode optional custom trigger (overrides label button).
	TriggerNode core.Node

	// Placement default bottomLeft.
	Placement DropdownPlacement
	// Triggers empty → [hover] (antd default).
	Triggers []DropdownTrigger
	// Arrow show / pointAtCenter.
	Arrow bool
	// ArrowPointAtCenter when Arrow and true (antd arrow={{ pointAtCenter }}).
	ArrowPointAtCenter bool
	// AutoAdjustOverflow default true (AnchoredPopup flip/shift).
	AutoAdjustOverflow bool
	// Disabled blocks open.
	Disabled bool
	// Open is the current visibility.
	Open bool

	// Selected last clicked menu key (convenience).
	Selected string
	Viewport core.Size
	Face     text.Face
	Theme    *core.Theme
	// AriaLabel accessible name for the trigger.
	AriaLabel string

	// OnOpenChange fires when open intent/state changes (source trigger|menu).
	OnOpenChange func(open bool, source DropdownOpenSource)
	// OnMenuClick fires when a selectable item is chosen (antd menu.onClick).
	OnMenuClick func(key string)

	// openControlled: SetOpen was used (antd open prop).
	openControlled bool
	// defaultOpen applied once when not controlled.
	defaultOpen    bool
	defaultOpenSet bool
	appliedDefault bool

	// expanded submenu keys (one-level Children).
	expanded map[string]bool
	// item rows for hover tracking.
	itemRows []*primitive.Pressable

	// hover close grace (leave trigger → menu without flicker).
	hoverClosePending bool
	// right-click host sits above shell for contextMenu trigger.
	ctxHost *dropdownPointerHost
}

// NewDropdown creates a Dropdown with optional default trigger label and menu items.
// Defaults (§6.10): trigger=[hover], placement=bottomLeft, arrow=false, closed, uncontrolled.
func NewDropdown(label string, items ...MenuItem) *Dropdown {
	d := &Dropdown{
		TriggerLabel:       label,
		Items:              append([]MenuItem(nil), items...),
		Placement:          DropdownBottomLeft,
		AutoAdjustOverflow: true,
		expanded:           map[string]bool{},
	}
	d.rebuild()
	return d
}

// Node returns the composition root (trigger + popup host).
func (d *Dropdown) Node() core.Node {
	if d == nil {
		return nil
	}
	if d.Wrap == nil {
		d.rebuild()
	}
	return d.Wrap
}

// Popup returns the anchored popup (tests / advanced hosts).
func (d *Dropdown) Popup() *primitive.AnchoredPopup {
	if d == nil {
		return nil
	}
	return d.popup
}

// IsOpen reports whether the menu is visible.
func (d *Dropdown) IsOpen() bool {
	return d != nil && d.Open
}

// Panel returns the menu panel chrome (tests).
func (d *Dropdown) Panel() *primitive.Decorated {
	if d == nil {
		return nil
	}
	return d.panel
}

// TriggerShell returns the trigger pressable (tests / a11y).
func (d *Dropdown) TriggerShell() *primitive.Pressable {
	if d == nil {
		return nil
	}
	return d.shell
}

// SetItems replaces menu items.
func (d *Dropdown) SetItems(items ...MenuItem) {
	if d == nil {
		return
	}
	d.Items = append([]MenuItem(nil), items...)
	d.rebuildMenu()
	if d.Open {
		d.measurePanel()
		d.syncPopupGeometry()
	}
}

// SetTriggerLabel sets the default button/link trigger text.
func (d *Dropdown) SetTriggerLabel(label string) {
	if d == nil {
		return
	}
	d.TriggerLabel = label
	if d.TriggerNode == nil {
		d.rebuild()
	} else if d.btn != nil {
		d.btn.SetLabel(label)
	}
	d.applyA11y()
}

// SetTriggerNode sets a custom trigger node (nil restores label button).
func (d *Dropdown) SetTriggerNode(n core.Node) {
	if d == nil {
		return
	}
	d.TriggerNode = n
	d.rebuild()
}

// SetTriggerModes sets open triggers (empty → hover default).
func (d *Dropdown) SetTriggerModes(modes ...DropdownTrigger) {
	if d == nil {
		return
	}
	d.Triggers = append([]DropdownTrigger(nil), modes...)
	d.wireTrigger()
}

// SetTrigger is a single-mode convenience (overwrites Triggers).
func (d *Dropdown) SetTrigger(mode DropdownTrigger) {
	d.SetTriggerModes(mode)
}

// SetPlacement sets popup placement.
func (d *Dropdown) SetPlacement(p DropdownPlacement) {
	if d == nil {
		return
	}
	d.Placement = p
	if d.popup != nil {
		d.popup.Placement = mapDropdownPlacement(p, d.ArrowPointAtCenter)
		if d.Open {
			d.syncPopupGeometry()
		}
	}
}

// SetArrow toggles the arrow indicator.
func (d *Dropdown) SetArrow(show bool) {
	if d == nil {
		return
	}
	d.Arrow = show
	d.rebuildMenu()
}

// SetArrowConfig sets arrow show + pointAtCenter (antd arrow object).
func (d *Dropdown) SetArrowConfig(show, pointAtCenter bool) {
	if d == nil {
		return
	}
	d.Arrow = show
	d.ArrowPointAtCenter = pointAtCenter
	if d.popup != nil {
		d.popup.Placement = mapDropdownPlacement(d.Placement, d.ArrowPointAtCenter)
	}
	d.rebuildMenu()
}

// SetAutoAdjustOverflow enables flip/shift (default true). When false, viewport is zeroed for placement.
func (d *Dropdown) SetAutoAdjustOverflow(v bool) {
	if d == nil {
		return
	}
	d.AutoAdjustOverflow = v
	d.applyViewportToPopup()
}

// SetDisabled disables the dropdown (cannot open).
func (d *Dropdown) SetDisabled(v bool) {
	if d == nil {
		return
	}
	d.Disabled = v
	if d.shell != nil {
		d.shell.SetDisabled(v)
	}
	if d.btn != nil {
		d.btn.SetDisabled(v)
	}
	if v && d.Open {
		d.applyOpen(false, DropdownSourceTrigger, false)
	}
}

// SetSelected records the last chosen key (tests / host state). Does not open the menu.
func (d *Dropdown) SetSelected(key string) {
	if d == nil {
		return
	}
	d.Selected = key
}

// SetOpen sets visibility and marks controlled (antd open prop).
func (d *Dropdown) SetOpen(open bool) {
	if d == nil {
		return
	}
	d.openControlled = true
	d.applyOpen(open, DropdownSourceTrigger, false)
}

// SetDefaultOpen sets initial open for uncontrolled mode.
func (d *Dropdown) SetDefaultOpen(open bool) {
	if d == nil || d.openControlled {
		return
	}
	d.defaultOpen = open
	d.defaultOpenSet = true
	if !d.appliedDefault {
		d.appliedDefault = true
		d.applyOpen(open, DropdownSourceTrigger, false)
	}
}

// SetOnOpenChange sets the open-change callback.
func (d *Dropdown) SetOnOpenChange(fn func(open bool, source DropdownOpenSource)) {
	if d == nil {
		return
	}
	d.OnOpenChange = fn
}

// SetOnMenuClick sets the menu item click callback.
func (d *Dropdown) SetOnMenuClick(fn func(key string)) {
	if d == nil {
		return
	}
	d.OnMenuClick = fn
}

// SetTheme sets an explicit theme override.
func (d *Dropdown) SetTheme(th *core.Theme) {
	if d == nil {
		return
	}
	d.Theme = th
	d.rebuild()
}

// SetFace sets the font face for labels.
func (d *Dropdown) SetFace(face text.Face) {
	if d == nil {
		return
	}
	d.Face = face
	d.rebuild()
}

// SetAriaLabel sets the accessible name on the trigger.
func (d *Dropdown) SetAriaLabel(name string) {
	if d == nil {
		return
	}
	d.AriaLabel = name
	d.applyA11y()
}

// AttachTicker is a no-op placeholder for gallery tickers slices
// (hover close uses AddTicker on demand).
func (d *Dropdown) AttachTicker(t *core.Tree) {
	_ = t
}

// Tick implements core.Ticker — resolves deferred hover leave close.
func (d *Dropdown) Tick(dt float64) bool {
	_ = dt
	if d == nil || !d.hoverClosePending {
		return false
	}
	d.hoverClosePending = false
	if !d.hasTrigger(DropdownTriggerHover) {
		return false
	}
	if d.pointerOverDropdown() {
		return false
	}
	d.requestOpen(false, DropdownSourceTrigger)
	return false
}

// Sync repositions while open.
// Deprecated: prefer Tree.Layout + AnchoredPopup.RefreshOpenGeometry.
func (d *Dropdown) Sync() {
	if d != nil && d.Open {
		d.syncPopupGeometry()
		if d.popup != nil {
			d.popup.SetOpen(true)
		}
	}
}

func (d *Dropdown) theme() *core.Theme {
	var n core.Node
	if d.Wrap != nil {
		n = d.Wrap
	}
	return themeOf(d.Theme, n)
}

func (d *Dropdown) hasTrigger(mode DropdownTrigger) bool {
	if d == nil {
		return false
	}
	modes := d.Triggers
	if len(modes) == 0 {
		modes = []DropdownTrigger{DropdownTriggerHover}
	}
	for _, m := range modes {
		if m == mode {
			return true
		}
	}
	return false
}

func (d *Dropdown) rebuild() {
	if d.expanded == nil {
		d.expanded = map[string]bool{}
	}
	th := d.theme()
	wasOpen := d.Open

	// Default trigger: link-style button with chevron (antd Space + DownOutlined).
	var trigger core.Node
	if d.TriggerNode != nil {
		trigger = d.TriggerNode
		d.btn = nil
	} else {
		label := d.TriggerLabel
		if label == "" {
			label = "Hover me"
		}
		d.btn = NewButton(label)
		d.btn.SetType(ButtonLink)
		d.btn.SetFace(d.Face)
		d.btn.Theme = d.Theme
		d.btn.SetIcon("chevron-down")
		d.btn.SetIconPlacement(ButtonIconEnd)
		if d.Disabled {
			d.btn.SetDisabled(true)
		}
		trigger = d.btn.Node()
	}

	d.shell = primitive.NewPressable(trigger)
	d.shell.Focusable = true
	d.shell.FocusRingRadius = th.SizeOr(core.TokenBorderRadius, 6)
	d.shell.SetDisabled(d.Disabled)
	d.applyA11y()

	// Context-menu host wraps shell so right-click bubbles.
	d.ctxHost = newDropdownPointerHost(d.shell, d)

	d.rebuildMenu()

	d.popup = primitive.NewAnchoredPopup(d.scope)
	d.popup.Placement = mapDropdownPlacement(d.Placement, d.ArrowPointAtCenter)
	d.popup.Gap = DefaultDropdownGap
	d.popup.Portal.ID = ""
	d.popup.DismissOnOutside = true
	d.popup.OnDismiss = func() {
		// Outside pointer: user intent (respect controlled open).
		d.requestOpen(false, DropdownSourceTrigger)
		// Uncontrolled already closed the popup via AnchoredPopup.SetOpen(false);
		// keep product Open in sync when uncontrolled (requestOpen→applyOpen).
		// Controlled: popup was closed by AnchoredPopup — re-open if parent still wants open.
		if d.openControlled && d.Open {
			d.popup.SetOpen(true)
		}
	}
	d.applyViewportToPopup()

	if d.Wrap == nil {
		d.Wrap = primitive.Column(d.ctxHost, d.popup)
	} else {
		d.Wrap.ClearChildren()
		d.Wrap.AddChild(d.ctxHost)
		d.Wrap.AddChild(d.popup)
	}
	d.Wrap.CrossAlign = core.CrossStart
	d.Wrap.SetThemeHook(func(*core.Theme) { d.rebuild() })

	d.wireTrigger()

	// Restore open after rebuild without re-notifying.
	if wasOpen || (d.defaultOpenSet && !d.openControlled && d.defaultOpen && !d.appliedDefault) {
		d.appliedDefault = true
		d.applyOpen(true, DropdownSourceTrigger, false)
	}

	d.Wrap.MarkNeedsLayout()
	d.Wrap.MarkNeedsPaint()
}

func (d *Dropdown) applyA11y() {
	if d.shell == nil {
		return
	}
	name := d.AriaLabel
	if name == "" {
		name = d.TriggerLabel
	}
	if name == "" {
		name = "dropdown"
	}
	d.shell.Base().Role = "button"
	d.shell.Base().Label = name
	if d.scope != nil {
		d.scope.Base().Role = "menu"
	}
	if d.panel != nil {
		d.panel.Base().Role = "menu"
	}
}

func (d *Dropdown) wireTrigger() {
	if d.shell == nil {
		return
	}
	d.shell.Click = func() {
		if d.Disabled {
			return
		}
		if d.hasTrigger(DropdownTriggerClick) {
			d.requestOpen(!d.Open, DropdownSourceTrigger)
		}
	}
	d.shell.OnStateChange = func() {
		if d.Disabled || !d.hasTrigger(DropdownTriggerHover) {
			return
		}
		if d.shell.State.Hovered {
			d.hoverClosePending = false
			d.requestOpen(true, DropdownSourceTrigger)
			return
		}
		// Leave trigger: defer close so move-to-menu can cancel.
		d.hoverClosePending = true
		if t := d.shell.Tree(); t != nil {
			t.AddTicker(d)
		} else if !d.pointerOverDropdown() {
			d.requestOpen(false, DropdownSourceTrigger)
		}
	}
}

func (d *Dropdown) rebuildMenu() {
	th := d.theme()
	d.itemRows = d.itemRows[:0]
	d.list = primitive.Column()
	d.list.Gap = 0
	d.list.CrossAlign = core.CrossStretch
	d.buildItems(d.list, d.Items, 0)

	inner := core.Node(d.list)
	if d.Arrow {
		sz := DefaultDropdownArrowSize
		border := th.Color(core.TokenColorBorder)
		bg := th.Color(core.TokenColorBgContainer)
		d.arrow = primitive.NewCanvas(sz, sz/2+1, func(pc *core.PaintContext, size core.Size) {
			if pc == nil {
				return
			}
			// Simple caret: two strokes forming a peak (works for bottom placements).
			mid := size.Width / 2
			pc.StrokeLocalLine(0, size.Height-1, mid, 1, 1, border)
			pc.StrokeLocalLine(mid, 1, size.Width, size.Height-1, 1, border)
			pc.FillLocalRect(mid-1, 2, 2, size.Height-2, bg)
		})
		col := primitive.Column()
		col.CrossAlign = core.CrossCenter
		col.Gap = 0
		switch d.Placement {
		case DropdownTop, DropdownTopLeft, DropdownTopRight:
			col.AddChild(d.list)
			col.AddChild(d.arrow)
		default:
			col.AddChild(d.arrow)
			col.AddChild(d.list)
		}
		inner = col
	} else {
		d.arrow = nil
	}

	d.panel = primitive.NewDecorated(inner)
	d.panel.Padding = primitive.All(DefaultDropdownPanelPad)
	d.panel.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	d.panel.Background = th.Color(core.TokenColorBgContainer)
	d.panel.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	d.panel.BorderColor = th.Color(core.TokenColorBorder)
	d.panel.MinWidth = DefaultDropdownMenuMinWidth
	d.panel.Base().Role = "menu"

	if d.scope == nil {
		d.scope = primitive.NewFocusScope(d.panel)
	} else {
		d.scope.ClearChildren()
		d.scope.AddChild(d.panel)
	}
	d.scope.Active = d.Open
	d.scope.OnEscape = func() {
		d.requestOpen(false, DropdownSourceTrigger)
	}
	d.scope.Base().Role = "menu"
	d.applyA11y()

	if d.popup != nil {
		d.popup.Content = d.scope
		if d.popup.Portal != nil {
			d.popup.Portal.Content = d.scope
		}
	}
}

func (d *Dropdown) buildItems(list *primitive.Flex, items []MenuItem, depth int) {
	th := d.theme()
	fs := th.SizeOr(core.TokenFontSize, DefaultDropdownFontSize)
	padH := DefaultDropdownItemPadInline + float64(depth)*12
	padV := DefaultDropdownItemPadBlock
	for _, it := range items {
		it := it
		if it.Divider {
			line := primitive.NewDecorated(nil)
			line.Height = 1
			line.ExpandWidth = true
			line.Background = th.Color(core.TokenColorSplit)
			if line.Background.A < 0.05 {
				line.Background = th.Color(core.TokenColorBorder)
			}
			line.Base().Role = "separator"
			wrap := primitive.NewDecorated(line)
			wrap.Padding = primitive.Symmetric(0, 4)
			wrap.ExpandWidth = true
			list.AddChild(wrap)
			continue
		}

		var kids []core.Node
		if it.Icon != "" {
			ic := primitive.NewIcon(it.Icon)
			ic.Size = fs
			ic.Color = th.Color(core.TokenColorText)
			if it.Disabled {
				ic.Color = th.Color(core.TokenColorDisabledText)
			}
			if it.Danger {
				ic.Color = th.Color(core.TokenColorError)
			}
			kids = append(kids, ic)
		}
		lab := primitive.NewText(it.Label)
		lab.FontSize = fs
		lab.Face = d.Face
		lab.Color = th.Color(core.TokenColorText)
		if it.Danger {
			lab.Color = th.Color(core.TokenColorError)
		}
		if it.Disabled {
			lab.Color = th.Color(core.TokenColorDisabledText)
		}
		kids = append(kids, lab)
		if it.Extra != "" {
			kids = append(kids, primitive.Spacer())
			ex := primitive.NewText(it.Extra)
			ex.FontSize = th.SizeOr(core.TokenFontSizeSM, 12)
			ex.Face = d.Face
			ex.Color = th.Color(core.TokenColorTextSecondary)
			if it.Disabled {
				ex.Color = th.Color(core.TokenColorDisabledText)
			}
			kids = append(kids, ex)
		}
		if len(it.Children) > 0 {
			kids = append(kids, primitive.Spacer())
			chev := primitive.NewIcon("chevron-right")
			chev.Size = 12
			chev.Color = th.Color(core.TokenColorTextSecondary)
			kids = append(kids, chev)
		}

		rowInner := primitive.Row(kids...)
		rowInner.CrossAlign = core.CrossCenter
		rowInner.Gap = 8
		row := primitive.NewPressable(rowInner)
		row.Padding = primitive.Symmetric(padH, padV)
		row.Base().Role = "menuitem"
		row.Base().Label = it.Label
		if it.Disabled {
			row.SetDisabled(true)
		} else {
			row.ColorHovered = antItemHoverFill(th)
		}
		key := it.Key
		hasChildren := len(it.Children) > 0
		row.Click = func() {
			if d.Disabled || it.Disabled {
				return
			}
			if hasChildren {
				d.expanded[key] = !d.expanded[key]
				d.rebuildMenu()
				d.measurePanel()
				if d.popup != nil && d.Open {
					d.popup.SetOpen(true)
				}
				return
			}
			d.Selected = key
			if d.OnMenuClick != nil {
				d.OnMenuClick(key)
			}
			d.requestOpen(false, DropdownSourceMenu)
		}
		row.OnStateChange = func() {
			if d.hasTrigger(DropdownTriggerHover) && row.State.Hovered {
				d.hoverClosePending = false
				if !d.Open {
					d.requestOpen(true, DropdownSourceTrigger)
				}
			}
		}
		d.itemRows = append(d.itemRows, row)
		list.AddChild(row)

		if hasChildren && d.expanded[key] {
			d.buildItems(list, it.Children, depth+1)
		}
	}
}

func (d *Dropdown) pointerOverDropdown() bool {
	if d.shell != nil && d.shell.State.Hovered {
		return true
	}
	for _, r := range d.itemRows {
		if r != nil && r.State.Hovered {
			return true
		}
	}
	return false
}

// requestOpen is the user-intent path (trigger / menu / esc / outside).
// Controlled mode only fires OnOpenChange; visibility waits for SetOpen.
func (d *Dropdown) requestOpen(open bool, src DropdownOpenSource) {
	if d == nil {
		return
	}
	if d.Disabled && open {
		return
	}
	if d.openControlled {
		if d.Open != open && d.OnOpenChange != nil {
			d.OnOpenChange(open, src)
		}
		return
	}
	if d.Open == open {
		return
	}
	d.applyOpen(open, src, true)
}

func (d *Dropdown) applyOpen(open bool, src DropdownOpenSource, notify bool) {
	if d == nil {
		return
	}
	if d.Disabled && open {
		return
	}
	prev := d.Open
	d.Open = open
	if d.scope != nil {
		d.scope.Active = open
		if open {
			d.scope.OnEscape = func() {
				d.requestOpen(false, DropdownSourceTrigger)
			}
		}
	}
	if d.popup != nil {
		if open {
			d.measurePanel()
			d.syncPopupGeometry()
		}
		d.popup.SetOpen(open)
	}
	if notify && prev != open && d.OnOpenChange != nil {
		d.OnOpenChange(open, src)
	}
	if d.Wrap != nil {
		d.Wrap.MarkNeedsLayout()
		d.Wrap.MarkNeedsPaint()
	}
}

func (d *Dropdown) measurePanel() {
	if d.panel != nil {
		_ = d.panel.Layout(core.Loose(400, 800))
	} else if d.scope != nil {
		_ = d.scope.Layout(core.Loose(400, 800))
	}
}

func (d *Dropdown) syncPopupGeometry() {
	if d.popup == nil {
		return
	}
	var anchor core.Node = d.shell
	if d.ctxHost != nil {
		anchor = d.ctxHost
	}
	d.popup.UpdateAnchorFromNode(anchor)
	d.applyViewportToPopup()
}

func (d *Dropdown) applyViewportToPopup() {
	if d.popup == nil {
		return
	}
	if !d.AutoAdjustOverflow {
		d.popup.Viewport = core.Size{}
		return
	}
	if d.Viewport.Width > 0 {
		d.popup.Viewport = d.Viewport
	}
}

func mapDropdownPlacement(p DropdownPlacement, pointAtCenter bool) primitive.Placement {
	// pointAtCenter: prefer centered variants on the primary axis for *Left/*Right.
	if pointAtCenter {
		switch p {
		case DropdownBottomLeft, DropdownBottomRight:
			return primitive.PlaceBottom
		case DropdownTopLeft, DropdownTopRight:
			return primitive.PlaceTop
		case DropdownLeftTop, DropdownLeftBottom:
			return primitive.PlaceLeft
		case DropdownRightTop, DropdownRightBottom:
			return primitive.PlaceRight
		}
	}
	switch p {
	case DropdownBottom:
		return primitive.PlaceBottom
	case DropdownBottomRight:
		return primitive.PlaceBottomEnd
	case DropdownTopLeft:
		return primitive.PlaceTopStart
	case DropdownTop:
		return primitive.PlaceTop
	case DropdownTopRight:
		return primitive.PlaceTopEnd
	case DropdownLeftTop:
		return primitive.PlaceLeftStart
	case DropdownLeft:
		return primitive.PlaceLeft
	case DropdownLeftBottom:
		return primitive.PlaceLeftEnd
	case DropdownRightTop:
		return primitive.PlaceRightStart
	case DropdownRight:
		return primitive.PlaceRight
	case DropdownRightBottom:
		return primitive.PlaceRightEnd
	default:
		return primitive.PlaceBottomStart
	}
}

// dropdownPointerHost wraps the trigger to capture context-menu (right button).
type dropdownPointerHost struct {
	core.NodeBase
	child *primitive.Pressable
	dd    *Dropdown
}

func newDropdownPointerHost(child *primitive.Pressable, dd *Dropdown) *dropdownPointerHost {
	h := &dropdownPointerHost{child: child, dd: dd}
	h.Init(h)
	h.Hit = core.HitDefer
	if child != nil {
		h.AddChild(child)
	}
	return h
}

func (h *dropdownPointerHost) TypeID() string { return "kit.dropdownPointerHost" }

func (h *dropdownPointerHost) Layout(c core.Constraints) core.Size {
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

func (h *dropdownPointerHost) Paint(pc *core.PaintContext) { h.DefaultPaintChildren(pc) }

func (h *dropdownPointerHost) HitTest(p core.Point) core.Node { return h.DefaultHitTest(p) }

func (h *dropdownPointerHost) HandlePointer(ev *core.PointerEvent) {
	if h == nil || h.dd == nil || ev == nil || h.dd.Disabled {
		return
	}
	if !h.dd.hasTrigger(DropdownTriggerContextMenu) {
		return
	}
	if ev.Type == core.PointerDown && ev.Button == core.ButtonRight {
		h.dd.requestOpen(true, DropdownSourceTrigger)
		ev.Handled = true
	}
}
