package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design FloatButton defaults — components/float-button/style.
// docs/antd/float-button.md §6.2 / §6.10
const (
	DefaultFloatButtonSize           = 40.0 // controlHeightLG
	DefaultFloatButtonSquareRadius   = 8.0  // borderRadiusLG
	DefaultFloatButtonIconSize       = 18.0 // fontSizeIcon×1.5
	DefaultFloatButtonContentFont    = 12.0 // fontSizeSM
	DefaultFloatButtonPadV           = 4.0  // paddingXXS
	DefaultFloatButtonInsetInlineEnd = 24.0 // marginLG
	DefaultFloatButtonInsetBlockEnd  = 48.0 // marginXXL
	DefaultFloatButtonGroupGap       = 16.0 // padding
	defaultFloatButtonIconName       = "info"
	defaultFloatButtonCloseIcon      = "close"
)

// FloatButtonShape is Ant shape prop (circle | square).
type FloatButtonShape int

const (
	FloatButtonCircle FloatButtonShape = iota
	FloatButtonSquare
)

// FloatButtonTrigger is Group menu trigger (none | click | hover).
type FloatButtonTrigger int

const (
	// FloatButtonTriggerNone — children always visible (non-menu group).
	FloatButtonTriggerNone FloatButtonTrigger = iota
	FloatButtonTriggerClick
	FloatButtonTriggerHover
)

// FloatButtonPlacement is Group menu expand direction.
type FloatButtonPlacement int

const (
	FloatButtonTop FloatButtonPlacement = iota
	FloatButtonBottom
	FloatButtonLeft
	FloatButtonRight
)

// FloatButton is Ant Design FloatButton (FAB).
//
//	Pressable
//	  └─ Decorated chrome
//	       └─ content (icon / column(icon+caption) / spinner)
//
// Positioning is layout-only (Stack / offsets) — not OS always-on-top.
// Product contract: docs/antd/float-button.md §6 (P0 DoD).
//
// Composes kit.Button for chrome / hover / loading / keyboard; overrides
// metrics to floatButtonSize and FAB padding.
type FloatButton struct {
	btn *Button

	// tooltipHost wraps btn + AnchoredPopup when Tooltip is set.
	tooltipHost *primitive.Flex
	tooltipPop  *primitive.AnchoredPopup
	tooltipLab  *primitive.Text
	tooltipBub  *primitive.Decorated

	// Type: default (antd) or primary. Other ButtonType values fall back to default.
	Type ButtonType
	// Shape circle (default) or square.
	Shape FloatButtonShape
	// IconName optional icon; empty + empty Content → default icon.
	IconName string
	// Content caption text (antd content; replaces deprecated description).
	Content string
	// Tooltip hover bubble text (empty = none).
	Tooltip string
	// Disabled / Loading.
	Disabled bool
	Loading  bool
	// AriaLabel accessible name; required for icon-only (§6.6 / FB-13).
	AriaLabel string
	OnClick   func()
	Face      text.Face
	Theme     *core.Theme
	Style     Style
}

// NewFloatButton creates a FloatButton with Ant defaults.
// Defaults (§6.10): Type=default, Shape=circle, no content, default icon.
func NewFloatButton() *FloatButton {
	f := &FloatButton{
		Type:  ButtonDefault,
		Shape: FloatButtonCircle,
	}
	f.rebuild()
	return f
}

// Node returns the root core.Node (button, or button+tooltip host).
func (f *FloatButton) Node() core.Node {
	if f == nil {
		return nil
	}
	if f.btn == nil {
		f.rebuild()
	}
	if f.Tooltip != "" && f.tooltipHost != nil {
		return f.tooltipHost
	}
	return f.btn.Node()
}

// ChromeNode returns the Decorated chrome (tests / composition).
func (f *FloatButton) ChromeNode() core.Node {
	if f == nil {
		return nil
	}
	if f.btn == nil {
		f.rebuild()
	}
	return f.btn.ChromeNode()
}

// Button exposes the embedded kit.Button (tests / hover sync / composition).
func (f *FloatButton) Button() *Button {
	if f == nil {
		return nil
	}
	if f.btn == nil {
		f.rebuild()
	}
	return f.btn
}

// SetType sets primary or default chrome (other types → default).
func (f *FloatButton) SetType(t ButtonType) {
	if f == nil {
		return
	}
	if t != ButtonPrimary {
		t = ButtonDefault
	}
	f.Type = t
	if f.btn != nil {
		f.btn.SetType(t)
		f.applyMetrics()
	}
}

// SetShape sets circle or square.
func (f *FloatButton) SetShape(shape FloatButtonShape) {
	if f == nil {
		return
	}
	f.Shape = shape
	f.applyMetrics()
}

// SetIcon sets icon name (empty clears; may fall back to default icon).
// Icon-only FABs update the glyph without a full Button.rebuild when possible,
// so open/close (plus↔close) keeps centering without a top-left flash frame.
func (f *FloatButton) SetIcon(name string) {
	if f == nil {
		return
	}
	if f.IconName == name && f.Content == "" && !f.Loading {
		// Still re-assert chrome (name may equal but glyph was swapped externally).
		f.applyMetrics()
		return
	}
	f.IconName = name
	if f.Content == "" && !f.Loading && f.btn != nil && f.btn.decorated != nil {
		// Fast path: swap icon in place, re-center via applyMetrics.
		f.btn.IconName = f.resolvedIcon()
		f.applyMetrics()
		return
	}
	f.applyContent()
	f.applyMetrics()
}

// SetContent sets caption text (antd content).
func (f *FloatButton) SetContent(s string) {
	if f == nil {
		return
	}
	if f.Content == s {
		return
	}
	f.Content = s
	f.applyContent()
	f.applyMetrics()
}

// SetTooltip sets hover bubble title (empty clears).
func (f *FloatButton) SetTooltip(s string) {
	if f == nil {
		return
	}
	f.Tooltip = s
	f.rebuildTooltip()
	f.wireHover()
}

// SetDisabled toggles disabled.
func (f *FloatButton) SetDisabled(d bool) {
	if f == nil {
		return
	}
	f.Disabled = d
	if f.btn != nil {
		f.btn.SetDisabled(d)
	}
}

// SetLoading toggles loading spinner (Ticker via AttachTicker / Button).
func (f *FloatButton) SetLoading(v bool) {
	if f == nil {
		return
	}
	f.Loading = v
	if f.btn != nil {
		f.btn.SetLoading(v)
		f.applyMetrics()
	}
}

// SetOnClick sets the click handler.
func (f *FloatButton) SetOnClick(fn func()) {
	if f == nil {
		return
	}
	f.OnClick = fn
	if f.btn != nil {
		f.btn.SetOnClick(fn)
	}
}

// SetAriaLabel sets the accessible name.
func (f *FloatButton) SetAriaLabel(name string) {
	if f == nil {
		return
	}
	f.AriaLabel = name
	if f.btn != nil {
		f.btn.SetAriaLabel(name)
	}
}

// SetFace sets the font face.
func (f *FloatButton) SetFace(face text.Face) {
	if f == nil {
		return
	}
	f.Face = face
	if f.btn != nil {
		f.btn.SetFace(face)
	}
	if f.tooltipLab != nil {
		f.tooltipLab.Face = face
	}
}

// SetTheme sets theme override.
func (f *FloatButton) SetTheme(th *core.Theme) {
	if f == nil {
		return
	}
	f.Theme = th
	if f.btn != nil {
		f.btn.Theme = th
		f.btn.applyChrome()
	}
	f.applyMetrics()
	f.rebuildTooltip()
}

// SetStyle applies visual overrides.
func (f *FloatButton) SetStyle(st Style) {
	if f == nil {
		return
	}
	f.Style = st
	if f.btn != nil {
		f.btn.SetStyle(st)
	}
	f.applyMetrics()
}

// SyncState refreshes hover/press chrome.
func (f *FloatButton) SyncState() {
	if f != nil && f.btn != nil {
		f.btn.SyncState()
		f.syncTooltip()
	}
}

// AttachTicker forwards loading tickers to the embedded Button.
func (f *FloatButton) AttachTicker(t *core.Tree) {
	if f != nil && f.btn != nil {
		f.btn.AttachTicker(t)
	}
}

// Tick advances loading spinner (core.Ticker via Button).
func (f *FloatButton) Tick(dt float64) bool {
	if f == nil || f.btn == nil {
		return false
	}
	return f.btn.Tick(dt)
}

func (f *FloatButton) theme() *core.Theme {
	var n core.Node
	if f.btn != nil && f.btn.Root != nil {
		n = f.btn.Root
	}
	return themeOf(f.Theme, n)
}

func (f *FloatButton) sizePx() float64 {
	th := f.theme()
	if f.Style.Width > 0 {
		return f.Style.Width
	}
	if f.Style.Height > 0 {
		return f.Style.Height
	}
	return th.SizeOr(core.TokenControlHeightLG, DefaultFloatButtonSize)
}

func (f *FloatButton) squareRadius() float64 {
	th := f.theme()
	if f.Style.hasRadius() {
		return f.Style.Radius
	}
	return th.SizeOr(core.TokenBorderRadiusLG, DefaultFloatButtonSquareRadius)
}

func (f *FloatButton) radius() float64 {
	sz := f.sizePx()
	if f != nil && f.Shape == FloatButtonSquare {
		return f.squareRadius()
	}
	return sz / 2
}

func (f *FloatButton) resolvedIcon() string {
	if f.IconName != "" {
		return f.IconName
	}
	if f.Content != "" {
		return ""
	}
	return defaultFloatButtonIconName
}

func (f *FloatButton) applyMetrics() {
	if f == nil || f.btn == nil {
		return
	}
	sz := f.sizePx()
	h := sz
	// content mode: slightly taller for caption (antd minHeight=size, height auto).
	if f.Content != "" {
		h = sz + 12
	}
	r := f.radius()
	// Drive Button metrics via Style so applyChrome keeps FAB radius.
	f.btn.Style.Radius = r
	f.btn.Style.ForceRadius = true
	f.btn.Style.Width = sz
	f.btn.Style.Height = h
	if f.Content != "" {
		f.btn.Style.FontSize = f.theme().SizeOr(core.TokenFontSizeSM, DefaultFloatButtonContentFont)
	}
	// Shape mapping: circle → Button circle; square → rectangle + forced radius.
	if f.Shape == FloatButtonCircle {
		f.btn.Shape = ButtonShapeCircle
	} else {
		f.btn.Shape = ButtonShapeDefault
	}
	f.btn.Size = ButtonLarge
	f.btn.SetFixedSize(sz, h)
	f.btn.applyChrome()

	// FAB chrome: fixed box, no inline pad (Button large padH=15 would left-align).
	if f.btn.decorated != nil {
		f.btn.decorated.Radius = r
		f.btn.decorated.Padding = primitive.EdgeInsets{}
		f.btn.decorated.Width = sz
		f.btn.decorated.MinWidth = sz
		f.btn.decorated.Height = h
		f.btn.decorated.MinHeight = h
		f.btn.decorated.ExpandWidth = false
		f.btn.decorated.SetCenterContent(true)

		iconOnly := f.Content == "" && !f.Loading
		if iconOnly {
			// Icon-only: mount Icon as sole child and center via CenterContent.
			// Avoid StretchChild+Flex path which can leave glyphs left-biased after
			// Button.rebuild (padH / StretchChild=false race) in menu Group triggers.
			iconSz := DefaultFloatButtonIconSize
			if fs := f.theme().SizeOr(core.TokenFontSizeSM, 12); fs > 0 {
				iconSz = fs * 1.5
			}
			name := f.resolvedIcon()
			if f.btn.icon == nil || f.btn.icon.Name != name {
				f.btn.icon = primitive.NewIcon(name)
			}
			f.btn.icon.Name = name
			f.btn.icon.Size = iconSz
			if f.btn.label != nil {
				f.btn.icon.Color = f.btn.label.Color
			}
			f.btn.row = nil
			f.btn.decorated.ClearChildren()
			f.btn.decorated.AddChild(f.btn.icon)
			f.btn.decorated.StretchChild = false // CenterContent both axes
		} else if f.Content != "" {
			padV := f.theme().SizeOr(core.TokenPaddingXS, DefaultFloatButtonPadV)
			f.btn.decorated.Padding = primitive.Symmetric(0, padV)
			f.btn.decorated.StretchChild = true // column fills + centers
			// Recolor content column after applyChrome.
			if f.btn.label != nil {
				fg := f.btn.label.Color
				for _, c := range f.btn.decorated.Children() {
					if col, ok := c.(*primitive.Flex); ok {
						for _, cc := range col.Children() {
							if ic, ok := cc.(*primitive.Icon); ok {
								ic.Color = fg
							}
							if tx, ok := cc.(*primitive.Text); ok {
								tx.Color = fg
							}
						}
					}
				}
			}
		} else {
			// Loading: keep Button row (spinner) stretched + centered.
			f.btn.decorated.StretchChild = true
			if f.btn.row != nil {
				f.btn.row.MainAlign = core.MainCenter
				f.btn.row.CrossAlign = core.CrossCenter
				f.btn.row.Gap = 0
			}
		}
		// Immediate self-layout: SetIcon on open/close re-parents the icon and
		// leaves offset={0,0} until Tree.Layout. Gallery can paint that frame
		// (top-left flash). Tight layout here keeps icon centered before paint.
		_ = f.btn.decorated.Layout(core.Tight(sz, h))
		f.btn.decorated.MarkNeedsLayout()
		f.btn.decorated.MarkNeedsPaint()
	}
	if f.btn.Root != nil {
		f.btn.Root.FocusRingRadius = r
		f.btn.Root.FocusRingOutset = 1.5
		// FAB is fixed 40×40 — never expand under CrossStretch.
		f.btn.Root.FixedSize = true
		// Sync Pressable box to same fixed size (hit == paint).
		_ = f.btn.Root.Layout(core.Tight(sz, h))
		if len(f.btn.Root.Children()) > 0 {
			f.btn.Root.Children()[0].Base().SetOffset(core.Point{})
		}
	}
	f.installThemeHook()
}

func (f *FloatButton) applyContent() {
	if f == nil || f.btn == nil {
		return
	}
	if f.Content != "" {
		// Vertical icon + caption (antd content / description).
		f.btn.SetLabel("")
		f.btn.SetIcon("")
		f.ensureContentBody()
	} else {
		icon := f.resolvedIcon()
		f.btn.SetLabel("")
		f.btn.SetIcon(icon)
	}
	if f.btn.Root != nil {
		a11y := f.AriaLabel
		if a11y == "" {
			a11y = f.Content
		}
		if a11y == "" {
			a11y = f.IconName
		}
		if a11y == "" {
			a11y = defaultFloatButtonIconName
		}
		f.btn.Root.Base().Label = a11y
		f.btn.Root.Base().Role = "button"
		if f.AriaLabel != "" {
			f.btn.AriaLabel = f.AriaLabel
		}
	}
}

// ensureContentBody builds a centered column (icon + content) inside Button chrome.
func (f *FloatButton) ensureContentBody() {
	if f == nil || f.btn == nil || f.btn.decorated == nil {
		return
	}
	col := primitive.Column()
	col.Gap = 2
	col.MainAlign = core.MainCenter
	col.CrossAlign = core.CrossCenter
	iconName := f.IconName
	if iconName != "" {
		ic := primitive.NewIcon(iconName)
		ic.Size = 16
		col.AddChild(ic)
	}
	tx := primitive.NewText(f.Content)
	tx.FontSize = f.theme().SizeOr(core.TokenFontSizeSM, DefaultFloatButtonContentFont)
	tx.Face = f.Face
	col.AddChild(tx)
	if f.btn.label == nil {
		f.btn.label = primitive.NewText("")
	}
	f.btn.row = nil
	f.btn.decorated.ClearChildren()
	f.btn.decorated.AddChild(col)
	// Keep label pointer for applyChrome color writes.
	f.btn.label = tx
}

func (f *FloatButton) rebuild() {
	if f == nil {
		return
	}
	if f.btn == nil {
		f.btn = NewButton("")
	}
	f.btn.Theme = f.Theme
	if f.Type != ButtonPrimary {
		f.Type = ButtonDefault
	}
	f.btn.Type = f.Type
	f.btn.Size = ButtonLarge
	f.btn.Disabled = f.Disabled
	f.btn.Loading = f.Loading
	f.btn.AriaLabel = f.AriaLabel
	f.btn.OnClick = f.OnClick
	if f.Face != nil {
		f.btn.Face = f.Face
	}
	// First rebuild of Button chrome, then FAB content/metrics.
	f.btn.rebuild()
	f.applyContent()
	f.applyMetrics()
	f.rebuildTooltip()
	f.wireHover()
	f.installThemeHook()
}

func (f *FloatButton) installThemeHook() {
	if f == nil || f.btn == nil || f.btn.Root == nil {
		return
	}
	f.btn.Root.SetThemeHook(func(*core.Theme) {
		f.applyMetrics()
	})
}

func (f *FloatButton) wireHover() {
	if f == nil || f.btn == nil || f.btn.Root == nil {
		return
	}
	// Chain after Button.SyncState so tooltip follows hover.
	f.btn.Root.OnStateChange = func() {
		f.btn.SyncState()
		f.syncTooltip()
	}
}

func (f *FloatButton) rebuildTooltip() {
	if f == nil {
		return
	}
	if f.Tooltip == "" {
		f.tooltipHost = nil
		f.tooltipPop = nil
		f.tooltipLab = nil
		f.tooltipBub = nil
		return
	}
	th := f.theme()
	f.tooltipLab = primitive.NewText(f.Tooltip)
	f.tooltipLab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	f.tooltipLab.Face = f.Face
	f.tooltipLab.Color = th.Color(core.TokenColorTextInverse)

	f.tooltipBub = primitive.NewDecorated(f.tooltipLab)
	f.tooltipBub.Padding = primitive.Symmetric(8, 6)
	f.tooltipBub.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	f.tooltipBub.Background = render.RGBA{R: 0, G: 0, B: 0, A: 0.85}

	f.tooltipPop = primitive.NewAnchoredPopup(f.tooltipBub)
	f.tooltipPop.Placement = primitive.PlaceTop
	f.tooltipPop.Gap = 6

	// Host: button + popup; no extra Pressable (hit stays on FAB).
	btnNode := f.btn.Node()
	f.tooltipHost = primitive.Column(btnNode, f.tooltipPop)
	f.tooltipHost.CrossAlign = core.CrossStart
	f.syncTooltip()
}

func (f *FloatButton) syncTooltip() {
	if f == nil || f.tooltipPop == nil || f.btn == nil || f.btn.Root == nil {
		return
	}
	open := f.btn.Root.State.Hovered && f.Tooltip != "" && !f.Disabled
	f.tooltipPop.UpdateAnchorFromNode(f.btn.Root)
	f.tooltipPop.SetOpen(open)
}

// ---------------------------------------------------------------------------
// FloatButtonGroup — Ant FloatButton.Group
// ---------------------------------------------------------------------------

// FloatButtonGroup is Ant Design FloatButton.Group.
//
// Without trigger: children always visible (column/row by shape).
// With trigger (menu mode): trigger FAB + expandable child list (placement).
type FloatButtonGroup struct {
	root    *primitive.Flex
	list    *primitive.Flex
	trigger *FloatButton
	shell   *primitive.Pressable // hover trigger surface

	Children  []*FloatButton
	Trigger   FloatButtonTrigger
	Placement FloatButtonPlacement
	Shape     FloatButtonShape
	Type      ButtonType
	IconName  string
	CloseIcon string
	Disabled  bool
	// Open is the expanded state (menu mode).
	Open bool
	// Controlled when SetOpen has been used.
	Controlled   bool
	OnOpenChange func(open bool)
	OnClick      func() // trigger click
	Face         text.Face
	Theme        *core.Theme
}

// NewFloatButtonGroup creates a group with optional children.
func NewFloatButtonGroup(children ...*FloatButton) *FloatButtonGroup {
	g := &FloatButtonGroup{
		Children:  children,
		Shape:     FloatButtonCircle,
		Type:      ButtonDefault,
		Placement: FloatButtonTop,
		CloseIcon: defaultFloatButtonCloseIcon,
		IconName:  defaultFloatButtonIconName,
	}
	g.rebuild()
	return g
}

// Node returns the group root.
func (g *FloatButtonGroup) Node() core.Node {
	if g == nil {
		return nil
	}
	if g.root == nil {
		g.rebuild()
	}
	if g.shell != nil {
		return g.shell
	}
	return g.root
}

// IsOpen reports menu expanded state.
func (g *FloatButtonGroup) IsOpen() bool {
	if g == nil {
		return false
	}
	if g.Trigger == FloatButtonTriggerNone {
		return true
	}
	return g.Open
}

// SetChildren replaces group children.
func (g *FloatButtonGroup) SetChildren(children ...*FloatButton) {
	if g == nil {
		return
	}
	g.Children = children
	g.rebuild()
}

// Add appends a child FloatButton.
func (g *FloatButtonGroup) Add(fb *FloatButton) {
	if g == nil || fb == nil {
		return
	}
	g.Children = append(g.Children, fb)
	g.rebuild()
}

// SetTrigger sets menu trigger mode (none | click | hover).
func (g *FloatButtonGroup) SetTrigger(t FloatButtonTrigger) {
	if g == nil {
		return
	}
	g.Trigger = t
	g.rebuild()
}

// SetPlacement sets expand direction (menu mode).
func (g *FloatButtonGroup) SetPlacement(p FloatButtonPlacement) {
	if g == nil {
		return
	}
	g.Placement = p
	g.rebuild()
}

// SetOpen sets expanded state and marks controlled.
func (g *FloatButtonGroup) SetOpen(open bool) {
	if g == nil {
		return
	}
	g.Controlled = true
	if g.Open == open {
		g.applyOpenVisibility()
		return
	}
	g.Open = open
	g.applyOpenVisibility()
	g.syncTriggerIcon()
}

// SetDefaultOpen sets initial open when not controlled.
func (g *FloatButtonGroup) SetDefaultOpen(open bool) {
	if g == nil || g.Controlled {
		return
	}
	g.Open = open
	g.applyOpenVisibility()
	g.syncTriggerIcon()
}

// SetOnOpenChange sets the open-change callback.
func (g *FloatButtonGroup) SetOnOpenChange(fn func(bool)) {
	if g != nil {
		g.OnOpenChange = fn
	}
}

// SetType sets trigger / group type.
func (g *FloatButtonGroup) SetType(t ButtonType) {
	if g == nil {
		return
	}
	if t != ButtonPrimary {
		t = ButtonDefault
	}
	g.Type = t
	g.rebuild()
}

// SetShape sets circle or square for group + children context.
func (g *FloatButtonGroup) SetShape(s FloatButtonShape) {
	if g == nil {
		return
	}
	g.Shape = s
	g.rebuild()
}

// SetIcon sets the trigger icon (closed menu).
func (g *FloatButtonGroup) SetIcon(name string) {
	if g == nil {
		return
	}
	g.IconName = name
	g.syncTriggerIcon()
}

// SetCloseIcon sets the trigger icon when open.
func (g *FloatButtonGroup) SetCloseIcon(name string) {
	if g == nil {
		return
	}
	if name == "" {
		name = defaultFloatButtonCloseIcon
	}
	g.CloseIcon = name
	g.syncTriggerIcon()
}

// SetDisabled disables the group / trigger.
func (g *FloatButtonGroup) SetDisabled(d bool) {
	if g == nil {
		return
	}
	g.Disabled = d
	if g.trigger != nil {
		g.trigger.SetDisabled(d)
	}
	if g.shell != nil {
		g.shell.SetDisabled(d)
	}
}

// SetOnClick sets trigger click handler (also toggles menu on click trigger).
func (g *FloatButtonGroup) SetOnClick(fn func()) {
	if g != nil {
		g.OnClick = fn
	}
}

// SetFace sets face on trigger and children that have none.
func (g *FloatButtonGroup) SetFace(face text.Face) {
	if g == nil {
		return
	}
	g.Face = face
	if g.trigger != nil {
		g.trigger.SetFace(face)
	}
	for _, c := range g.Children {
		if c != nil && c.Face == nil {
			c.SetFace(face)
		}
	}
}

// SetTheme sets theme override.
func (g *FloatButtonGroup) SetTheme(th *core.Theme) {
	if g == nil {
		return
	}
	g.Theme = th
	if g.trigger != nil {
		g.trigger.SetTheme(th)
	}
	for _, c := range g.Children {
		if c != nil {
			c.SetTheme(th)
		}
	}
}

// ListNode returns the children list flex (tests / placement).
func (g *FloatButtonGroup) ListNode() core.Node {
	if g == nil {
		return nil
	}
	return g.list
}

// TriggerButton returns the menu trigger FAB (nil when non-menu).
func (g *FloatButtonGroup) TriggerButton() *FloatButton {
	if g == nil {
		return nil
	}
	return g.trigger
}

func (g *FloatButtonGroup) menuMode() bool {
	return g != nil && (g.Trigger == FloatButtonTriggerClick || g.Trigger == FloatButtonTriggerHover)
}

func (g *FloatButtonGroup) setOpenInternal(open bool) {
	if g == nil || g.Disabled {
		return
	}
	if g.Controlled {
		// Controlled: do not flip Open; notify only.
		if g.OnOpenChange != nil {
			g.OnOpenChange(open)
		}
		return
	}
	if g.Open == open {
		return
	}
	g.Open = open
	g.applyOpenVisibility()
	g.syncTriggerIcon()
	if g.OnOpenChange != nil {
		g.OnOpenChange(open)
	}
}

func (g *FloatButtonGroup) toggleOpen() {
	g.setOpenInternal(!g.Open)
}

func (g *FloatButtonGroup) syncTriggerIcon() {
	if g == nil || g.trigger == nil {
		return
	}
	icon := g.IconName
	if icon == "" {
		icon = defaultFloatButtonIconName
	}
	if g.menuMode() && g.Open {
		closeI := g.CloseIcon
		if closeI == "" {
			closeI = defaultFloatButtonCloseIcon
		}
		icon = closeI
	}
	// Only swap glyph — avoid SetContent("") which used to force Button.rebuild
	// and left the icon at {0,0} until the next Tree.Layout (top-left flash).
	g.trigger.SetIcon(icon)
	if g.trigger.Content != "" {
		g.trigger.SetContent("")
	}
}

func (g *FloatButtonGroup) applyOpenVisibility() {
	if g == nil || g.root == nil || !g.menuMode() || g.trigger == nil {
		return
	}
	showList := g.Open
	trig := g.trigger.Node()
	// Rebuild root children order by placement; omit list when closed.
	// top:    list above trigger
	// bottom: trigger above list
	// left:   list · trigger (row)
	// right:  trigger · list (row)
	g.root.ClearChildren()
	switch g.Placement {
	case FloatButtonBottom:
		g.root.AddChild(trig)
		if showList && g.list != nil {
			g.root.AddChild(g.list)
		}
	case FloatButtonLeft:
		if showList && g.list != nil {
			g.root.AddChild(g.list)
		}
		g.root.AddChild(trig)
	case FloatButtonRight:
		g.root.AddChild(trig)
		if showList && g.list != nil {
			g.root.AddChild(g.list)
		}
	default: // top
		if showList && g.list != nil {
			g.root.AddChild(g.list)
		}
		g.root.AddChild(trig)
	}
	g.root.MarkNeedsLayout()
	g.root.MarkNeedsPaint()
}

func (g *FloatButtonGroup) rebuild() {
	if g == nil {
		return
	}
	gap := DefaultFloatButtonGroupGap
	th := themeOf(g.Theme, nil)
	if th != nil {
		if p := th.SizeOr(core.TokenPadding, DefaultFloatButtonGroupGap); p > 0 {
			gap = p
		}
	}

	// Propagate shape to children.
	for _, c := range g.Children {
		if c == nil {
			continue
		}
		c.SetShape(g.Shape)
		if g.Theme != nil {
			c.Theme = g.Theme
		}
		if g.Face != nil && c.Face == nil {
			c.Face = g.Face
		}
	}

	// List of children.
	vertical := g.Placement == FloatButtonTop || g.Placement == FloatButtonBottom || !g.menuMode()
	if g.menuMode() {
		vertical = g.Placement == FloatButtonTop || g.Placement == FloatButtonBottom
	} else {
		// Non-menu: antd Flex vertical when circle "individual" stack — use column.
		vertical = true
	}

	var childNodes []core.Node
	for _, c := range g.Children {
		if c != nil {
			childNodes = append(childNodes, c.Node())
		}
	}
	if vertical {
		g.list = primitive.Column(childNodes...)
	} else {
		g.list = primitive.Row(childNodes...)
	}
	g.list.Gap = gap
	g.list.MainAlign = core.MainStart
	g.list.CrossAlign = core.CrossCenter

	if !g.menuMode() {
		// Always-visible group: just the list.
		g.root = g.list
		g.trigger = nil
		g.shell = nil
		g.applyOpenVisibility()
		return
	}

	// Menu mode: trigger + list ordered by placement.
	if g.trigger == nil {
		g.trigger = NewFloatButton()
	}
	g.trigger.SetType(g.Type)
	g.trigger.SetShape(g.Shape)
	g.trigger.SetDisabled(g.Disabled)
	g.trigger.SetAriaLabel("float-button-group-trigger")
	if g.Face != nil {
		g.trigger.SetFace(g.Face)
	}
	if g.Theme != nil {
		g.trigger.SetTheme(g.Theme)
	}
	g.syncTriggerIcon()
	g.trigger.SetOnClick(func() {
		if g.Trigger == FloatButtonTriggerClick {
			g.toggleOpen()
		}
		if g.OnClick != nil {
			g.OnClick()
		}
	})

	// Build root flex by placement: list relative to trigger.
	var parts []core.Node
	switch g.Placement {
	case FloatButtonBottom:
		parts = []core.Node{g.trigger.Node(), g.list}
		g.root = primitive.Column(parts...)
	case FloatButtonLeft:
		parts = []core.Node{g.list, g.trigger.Node()}
		g.root = primitive.Row(parts...)
	case FloatButtonRight:
		parts = []core.Node{g.trigger.Node(), g.list}
		g.root = primitive.Row(parts...)
	default: // top
		parts = []core.Node{g.list, g.trigger.Node()}
		g.root = primitive.Column(parts...)
	}
	g.root.Gap = gap
	g.root.MainAlign = core.MainStart
	g.root.CrossAlign = core.CrossCenter

	// Hover surface for hover trigger.
	if g.Trigger == FloatButtonTriggerHover {
		g.shell = primitive.NewPressable(g.root)
		g.shell.Focusable = false
		g.shell.SetDisabled(g.Disabled)
		g.shell.OnStateChange = func() {
			if g.Disabled {
				return
			}
			g.setOpenInternal(g.shell.State.Hovered)
		}
	} else {
		g.shell = nil
	}

	g.applyOpenVisibility()
}
