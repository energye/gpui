// Package alert implements the Alert control (docs/antd/alert.md §6).
//
// L1 static feedback, no overlay. Owns a rendering.RenderBox node and
// reuses ui/rendering for draw, ui/theme for tokens, ui/focus for the
// close key path and ui/semantics for the read tree. No new event loop.
package alert

import (
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/semantics"
	"github.com/energye/gpui/ui/theme"
)

// AlertType is success|info|warning|error (antd §6.3).
type AlertType string

const (
	AlertInfo    AlertType = "info"
	AlertSuccess AlertType = "success"
	AlertWarning AlertType = "warning"
	AlertError   AlertType = "error"
)

// AlertVariant is outlined|filled (antd §6.3, 6.4.0).
type AlertVariant string

const (
	AlertOutlined AlertVariant = "outlined"
	AlertFilled   AlertVariant = "filled"
)

// Close hit keeps the 44px rule visible (hit == layout == paint).
const closeHitSize = 44.0

// Close glyph half size inside the hit box.
const closeGlyphHalf = 6.0

// AlertCloseEvent carries a closable tap. PreventDefault keeps Visible.
type AlertCloseEvent struct {
	alert     *Alert
	prevented bool
}

// Alert returns the source control.
func (e *AlertCloseEvent) Alert() *Alert {
	if e == nil {
		return nil
	}
	return e.alert
}

// PreventDefault stops the hide (ALT-S8, Tag parity).
func (e *AlertCloseEvent) PreventDefault() {
	if e != nil {
		e.prevented = true
	}
}

// Prevented reports whether PreventDefault was called.
func (e *AlertCloseEvent) Prevented() bool {
	return e != nil && e.prevented
}

// P1 ConfigProvider staging: package-level global defaults for closeIcon and
// per-type icons (antd closeIcon/errorIcon/infoIcon/successIcon/warningIcon).
// Explicit props win over these globals; ResetAlertGlobals clears them.
// The real ConfigProvider kit is NotStarted, so this hook is the P1 staging
// point and is covered by ALT-22.
var (
	alertGlobalCloseIcon rendering.RenderObject
	alertGlobalTypeIcons = map[AlertType]string{}
)

// SetGlobalCloseIcon pins the package-level close node fallback.
func SetGlobalCloseIcon(n rendering.RenderObject) { alertGlobalCloseIcon = n }

// GlobalCloseIcon returns the package-level close fallback, if any.
func GlobalCloseIcon() rendering.RenderObject { return alertGlobalCloseIcon }

// SetGlobalTypeIcon pins the package-level icon name for one AlertType.
func SetGlobalTypeIcon(t AlertType, name string) {
	if alertGlobalTypeIcons == nil {
		alertGlobalTypeIcons = map[AlertType]string{}
	}
	alertGlobalTypeIcons[t] = name
}

// GlobalTypeIcon returns the package-level icon name for one AlertType.
func GlobalTypeIcon(t AlertType) string { return alertGlobalTypeIcons[t] }

// ResetAlertGlobals clears all package-level icon fallbacks (test helper).
func ResetAlertGlobals() {
	alertGlobalCloseIcon = nil
	alertGlobalTypeIcons = map[AlertType]string{}
}

// Alert is the Alert widget (docs/antd/alert.md §6.10).
type Alert struct {
	title       string
	description string
	titleNode   rendering.RenderObject
	descNode    rendering.RenderObject
	iconNode    rendering.RenderObject
	iconName    string

	alertType AlertType
	typeSet   bool
	variant   AlertVariant
	banner    bool

	showIcon    bool
	showIconSet bool
	// closable/hidden/focused are atomic: event writes (UI), paint reads (raster).
	closable  atomic.Bool
	closeIcon rendering.RenderObject
	closeAria string
	closeSet  bool

	action rendering.RenderObject
	hidden atomic.Bool
	rtl    bool

	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	face      string
	// textFace is the paint-only font face (nil keeps headless rune
	// estimate; showcase/gallery sets it via SetTextFace so paint draws
	// real glyphs, same chain as button/tag: SetFont+DrawString via Abs).
	textFace   text.Face
	styleTag   map[string]string
	className  string
	classNames map[string]string

	// P1 leave-motion staging: P0 stays instant hide; these hooks record
	// the smooth-closed intent without changing P0 behavior.
	motionEnabled   bool
	leaveDurationMs float64
	onLeaveStart    func()

	onClose    func(*AlertCloseEvent)
	afterClose func()

	node       *rendering.RenderBox
	closeFocus *focus.FocusNode
	focused    atomic.Bool
	cachedSize rendering.Size
}

// NewAlert creates an alert with title (message alias kept for compat).
func NewAlert(title string) *Alert {
	a := &Alert{title: title, alertType: AlertInfo, variant: AlertOutlined, closeAria: "Close"}
	a.node = rendering.NewRenderBox()
	a.node.SetRepaintBoundary(true)
	a.node.SetRelayoutBoundary(true)
	self := a
	a.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	// Intrinsic size up front so Node() already has content size without an
	// extra Layout call, same contract as button (New tail pre-layout).
	a.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return a
}

func (a *Alert) syncNode() {
	if a == nil {
		return
	}
	if a.node == nil {
		a.node = rendering.NewRenderBox()
		a.node.SetRepaintBoundary(true)
		a.node.SetRelayoutBoundary(true)
		self := a
		a.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
			self.paint(pc, size)
		}
	}
}

// SetTitle sets the main text.
func (a *Alert) SetTitle(s string) *Alert {
	if a == nil {
		return a
	}
	if a.title == s {
		return a
	}
	a.title = s
	a.markLayout()
	return a
}

// Title returns the main text.
func (a *Alert) Title() string {
	if a == nil {
		return ""
	}
	return a.title
}

// SetTitleNode installs a custom title (loop-banner Ticker slot).
func (a *Alert) SetTitleNode(n rendering.RenderObject) *Alert {
	if a == nil {
		return a
	}
	a.titleNode = n
	a.markLayout()
	return a
}

// TitleNode returns the custom title, if any.
func (a *Alert) TitleNode() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.titleNode
}

// SetMessage is the deprecated alias of SetTitle.
func (a *Alert) SetMessage(s string) *Alert { return a.SetTitle(s) }

// SetDescription sets the helper text (double row).
func (a *Alert) SetDescription(s string) *Alert {
	if a == nil {
		return a
	}
	if a.description == s {
		return a
	}
	a.description = s
	a.markLayout()
	return a
}

// Description returns the helper text.
func (a *Alert) Description() string {
	if a == nil {
		return ""
	}
	return a.description
}

// HasDescription reports the double-row structure (ALT-S3).
func (a *Alert) HasDescription() bool {
	return a != nil && (a.description != "" || a.descNode != nil)
}

// SetDescriptionNode installs a custom description.
func (a *Alert) SetDescriptionNode(n rendering.RenderObject) *Alert {
	if a == nil {
		return a
	}
	a.descNode = n
	a.markLayout()
	return a
}

// DescriptionNode returns the custom description, if any.
func (a *Alert) DescriptionNode() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.descNode
}

// SetType sets the semantic type.
func (a *Alert) SetType(t AlertType) *Alert {
	if a == nil {
		return a
	}
	if a.alertType == t && a.typeSet {
		return a
	}
	a.alertType = t
	a.typeSet = true
	a.markPaint()
	return a
}

// SetTypeString accepts a plain string (info|success|warning|error).
func (a *Alert) SetTypeString(s string) *Alert {
	return a.SetType(AlertType(s))
}

// EffectiveType applies banner default warning (ALT-S5).
func (a *Alert) EffectiveType() AlertType {
	if a == nil {
		return AlertInfo
	}
	if a.typeSet {
		switch a.alertType {
		case AlertSuccess, AlertInfo, AlertWarning, AlertError:
			return a.alertType
		default:
			return AlertInfo
		}
	}
	if a.banner {
		return AlertWarning
	}
	return AlertInfo
}

// Type returns the effective type.
func (a *Alert) Type() AlertType { return a.EffectiveType() }

// SetVariant sets outlined|filled.
func (a *Alert) SetVariant(v AlertVariant) *Alert {
	if a == nil {
		return a
	}
	if a.variant == v {
		return a
	}
	a.variant = v
	a.markPaint()
	return a
}

// Variant returns the variant.
func (a *Alert) Variant() AlertVariant {
	if a == nil {
		return AlertOutlined
	}
	if a.variant == AlertFilled {
		return AlertFilled
	}
	return AlertOutlined
}

// SetBanner toggles top-banner chrome (radius 0, no border).
func (a *Alert) SetBanner(b bool) *Alert {
	if a == nil {
		return a
	}
	if a.banner == b {
		return a
	}
	a.banner = b
	a.markLayout()
	return a
}

// IsBanner reports banner mode.
func (a *Alert) IsBanner() bool { return a != nil && a.banner }

// Banner is an alias of IsBanner.
func (a *Alert) Banner() bool { return a.IsBanner() }

// SetShowIcon toggles the helper icon.
func (a *Alert) SetShowIcon(b bool) *Alert {
	if a == nil {
		return a
	}
	if a.showIcon == b && a.showIconSet {
		return a
	}
	a.showIcon = b
	a.showIconSet = true
	a.markLayout()
	return a
}

// EffectiveShowIcon applies banner default true.
func (a *Alert) EffectiveShowIcon() bool {
	if a == nil {
		return false
	}
	if a.showIconSet {
		return a.showIcon
	}
	return a.banner
}

// ShowIcon returns the effective flag.
func (a *Alert) ShowIcon() bool { return a.EffectiveShowIcon() }

// IconVisible reports whether the icon slot paints.
func (a *Alert) IconVisible() bool {
	return a != nil && !a.hidden.Load() && a.EffectiveShowIcon()
}

// SetIcon sets a custom icon name (shown when showIcon).
func (a *Alert) SetIcon(name string) *Alert {
	if a == nil {
		return a
	}
	if a.iconName == name {
		return a
	}
	a.iconName = name
	a.markPaint()
	return a
}

// IconName returns the custom icon name (explicit prop, else P1 global).
func (a *Alert) IconName() string {
	if a == nil {
		return ""
	}
	if a.iconName != "" {
		return a.iconName
	}
	return alertGlobalTypeIcons[a.EffectiveType()]
}

// EffectiveIconName is the resolved icon name (explicit > global > "").
func (a *Alert) EffectiveIconName() string { return a.IconName() }

// SetIconNode installs a custom icon node.
func (a *Alert) SetIconNode(n rendering.RenderObject) *Alert {
	if a == nil {
		return a
	}
	a.iconNode = n
	a.markLayout()
	return a
}

// IconNode returns the custom icon, if any.
func (a *Alert) IconNode() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.iconNode
}

// SetClosable toggles the close affordance.
func (a *Alert) SetClosable(b bool) *Alert {
	if a == nil {
		return a
	}
	if a.closable.Load() == b {
		return a
	}
	a.closable.Store(b)
	if a.closeFocus != nil {
		a.closeFocus.Enabled = b && !a.hidden.Load()
	}
	a.markLayout()
	return a
}

// Closable reports the flag.
func (a *Alert) Closable() bool { return a != nil && a.closable.Load() }

// IsClosable is an alias of Closable.
func (a *Alert) IsClosable() bool { return a.Closable() }

// SetCloseIcon installs a custom close node (P0 slot, paint via tree when set).
func (a *Alert) SetCloseIcon(n rendering.RenderObject) *Alert {
	if a == nil {
		return a
	}
	a.closeIcon = n
	a.markPaint()
	return a
}

// CloseIcon returns the custom close node, if any.
func (a *Alert) CloseIcon() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.closeIcon
}

// EffectiveCloseIcon resolves explicit closeIcon else P1 global fallback.
func (a *Alert) EffectiveCloseIcon() rendering.RenderObject {
	if a == nil {
		return nil
	}
	if a.closeIcon != nil {
		return a.closeIcon
	}
	return alertGlobalCloseIcon
}

// SetOnClose sets the close callback (PreventDefault keeps visible).
func (a *Alert) SetOnClose(fn func(*AlertCloseEvent)) *Alert {
	if a == nil {
		return a
	}
	a.onClose = fn
	return a
}

// OnClose is an alias of SetOnClose.
func (a *Alert) OnClose(fn func(*AlertCloseEvent)) *Alert { return a.SetOnClose(fn) }

// SetAfterClose sets the post-hide callback (P0 sync, no leave anim).
func (a *Alert) SetAfterClose(fn func()) *Alert {
	if a == nil {
		return a
	}
	a.afterClose = fn
	return a
}

// SetCloseAria sets the close accessible name (default Close).
func (a *Alert) SetCloseAria(s string) *Alert {
	if a == nil {
		return a
	}
	a.closeAria = s
	a.closeSet = true
	return a
}

// CloseAria returns the close name.
func (a *Alert) CloseAria() string {
	if a == nil {
		return "Close"
	}
	if a.closeAria == "" {
		return "Close"
	}
	return a.closeAria
}

// CloseAriaLabel is an alias of CloseAria.
func (a *Alert) CloseAriaLabel() string { return a.CloseAria() }

// SetAction mounts the right operation slot (Button host).
func (a *Alert) SetAction(n rendering.RenderObject) *Alert {
	if a == nil {
		return a
	}
	a.action = n
	a.markLayout()
	return a
}

// Action returns the slot node, if any.
func (a *Alert) Action() rendering.RenderObject {
	if a == nil {
		return nil
	}
	return a.action
}

// ActionNode is an alias of Action.
func (a *Alert) ActionNode() rendering.RenderObject { return a.Action() }

// HasAction reports whether the slot is mounted.
func (a *Alert) HasAction() bool { return a != nil && a.action != nil }

// ActionVisible reports slot visibility.
func (a *Alert) ActionVisible() bool { return a != nil && !a.hidden.Load() && a.action != nil }

// SetProvider selects the theme source (nil selects process default).
func (a *Alert) SetProvider(p *theme.Provider) *Alert {
	if a == nil {
		return a
	}
	a.provider = p
	a.markPaint()
	return a
}

// SetTheme pins exact tokens (nil clears to provider).
func (a *Alert) SetTheme(t *theme.Tokens) *Alert {
	if a == nil {
		return a
	}
	a.override = t
	a.markPaint()
	return a
}

// SetFace stores the font family hook (paint proxy keeps headless stable).
func (a *Alert) SetFace(family string) *Alert {
	if a == nil {
		return a
	}
	a.face = family
	return a
}

// SetStyle stores semantic style hooks (P1 classNames/styles parity, no CSS engine).
func (a *Alert) SetStyle(m map[string]string) *Alert {
	if a == nil {
		return a
	}
	cp := map[string]string{}
	for k, v := range m {
		cp[k] = v
	}
	a.styleTag = cp
	return a
}

// Styles returns the stored semantic style hooks.
func (a *Alert) Styles() map[string]string {
	if a == nil {
		return nil
	}
	return a.styleTag
}

// SetClassName stores the semantic class hook (P1, paint only, no CSS engine).
func (a *Alert) SetClassName(s string) *Alert {
	if a == nil {
		return a
	}
	a.className = s
	return a
}

// ClassName returns the stored semantic class hook.
func (a *Alert) ClassName() string {
	if a == nil {
		return ""
	}
	return a.className
}

// SetClassNames stores per-node semantic class hooks (P1, paint only).
func (a *Alert) SetClassNames(m map[string]string) *Alert {
	if a == nil {
		return a
	}
	cp := map[string]string{}
	for k, v := range m {
		cp[k] = v
	}
	a.classNames = cp
	return a
}

// ClassNames returns the stored per-node semantic class hooks.
func (a *Alert) ClassNames() map[string]string {
	if a == nil {
		return nil
	}
	return a.classNames
}

// SetTextFace sets the paint-only font face (nil clears; layout keeps the
// rune estimate so headless tests stay stable). Same chain as button:
// paint does SetFont+DrawString via pc.Abs when face is set.
func (a *Alert) SetTextFace(f text.Face) *Alert {
	if a == nil {
		return a
	}
	a.textFace = f
	a.markPaint()
	return a
}

// TextFace returns the paint-only font face, if any.
func (a *Alert) TextFace() text.Face {
	if a == nil {
		return nil
	}
	return a.textFace
}

// SetMotionEnabled stages the P1 smooth-closed intent (default false = P0
// instant hide). Enabling never delays P0 hide; pixel animation stays staged.
func (a *Alert) SetMotionEnabled(b bool) *Alert {
	if a == nil {
		return a
	}
	a.motionEnabled = b
	return a
}

// MotionEnabled reports the staged leave-motion flag.
func (a *Alert) MotionEnabled() bool { return a != nil && a.motionEnabled }

// SetLeaveDurationMs stores the staged leave duration (P1 hook, paint only).
func (a *Alert) SetLeaveDurationMs(ms float64) *Alert {
	if a == nil {
		return a
	}
	if ms < 0 {
		ms = 0
	}
	a.leaveDurationMs = ms
	return a
}

// LeaveDurationMs returns the staged leave duration.
func (a *Alert) LeaveDurationMs() float64 {
	if a == nil {
		return 0
	}
	return a.leaveDurationMs
}

// SetOnLeaveStart stores the staged leave-animation start hook (P1).
func (a *Alert) SetOnLeaveStart(fn func()) *Alert {
	if a == nil {
		return a
	}
	a.onLeaveStart = fn
	return a
}

// SetAriaLabel sets the root accessible name (empty keeps role-only).
func (a *Alert) SetAriaLabel(s string) *Alert {
	if a == nil {
		return a
	}
	a.ariaLabel = s
	return a
}

// AriaLabel returns the root name.
func (a *Alert) AriaLabel() string {
	if a == nil {
		return ""
	}
	return a.ariaLabel
}

// SetRTL mirrors icon/close placement for the RTL snapshot.
func (a *Alert) SetRTL(b bool) *Alert {
	if a == nil {
		return a
	}
	if a.rtl == b {
		return a
	}
	a.rtl = b
	a.markPaint()
	return a
}

// IsRTL reports mirror mode.
func (a *Alert) IsRTL() bool { return a != nil && a.rtl }

// Visible reports hide state (false after Close without PreventDefault).
func (a *Alert) Visible() bool { return a != nil && !a.hidden.Load() }

// Hidden reports the closed state.
func (a *Alert) Hidden() bool { return a != nil && a.hidden.Load() }

// Reset reopens a closed alert (Queue→Layout→Frame→Reset helper).
func (a *Alert) Reset() *Alert {
	if a == nil {
		return a
	}
	if !a.hidden.Load() {
		return a
	}
	a.hidden.Store(false)
	if a.closeFocus != nil {
		a.closeFocus.Enabled = a.closable.Load()
	}
	a.markLayout()
	return a
}

// ClickClose runs onClose → hide → afterClose (P0 instant, ALT-S2).
// P1 staging: onLeaveStart fires before hide when motion is enabled, but
// hide stays instant (pixel leave animation is staged, hook only).
func (a *Alert) ClickClose() bool {
	if a == nil || !a.closable.Load() || a.hidden.Load() {
		return false
	}
	ev := &AlertCloseEvent{alert: a}
	if a.onClose != nil {
		a.onClose(ev)
	}
	if ev.prevented {
		return false
	}
	if a.motionEnabled && a.onLeaveStart != nil {
		a.onLeaveStart()
	}
	a.hidden.Store(true)
	if a.closeFocus != nil {
		a.closeFocus.Enabled = false
		if a.closeFocus.HasFocus() {
			a.closeFocus.Unfocus()
		}
	}
	a.markLayout()
	if a.afterClose != nil {
		a.afterClose()
	}
	return true
}

// Close is an alias of ClickClose (programmatic close).
func (a *Alert) Close() bool { return a.ClickClose() }

// PressCloseKey activates the close via keyboard (Enter/Space, ALT-19).
func (a *Alert) PressCloseKey(key string) bool {
	if a == nil || !a.closable.Load() || a.hidden.Load() {
		return false
	}
	if key == "Enter" || key == "Space" || key == " " || key == "\r" {
		return a.ClickClose()
	}
	return false
}

// CloseFocusNode is the keyboard target for the close (Tab reachable).
func (a *Alert) CloseFocusNode() *focus.FocusNode {
	if a == nil {
		return nil
	}
	if a.closeFocus == nil {
		n := focus.NewFocusNode("alert-close")
		n.Enabled = a.closable.Load() && !a.hidden.Load()
		n.TabIndex = 0
		self := a
		n.OnActivate = func() { self.ClickClose() }
		n.OnFocusChange = func(f bool) {
			self.focused.Store(f)
			self.markPaint()
		}
		a.closeFocus = n
	}
	return a.closeFocus
}

// Focusable is false for the static root: display never steals Tab.
func (a *Alert) Focusable() bool { return false }

// CloseFocusable reports whether the close takes Tab.
func (a *Alert) CloseFocusable() bool { return a != nil && a.closable.Load() && !a.hidden.Load() }

// Role is the root reader role (a11y §6.6).
func (a *Alert) Role() string { return "alert" }

// CloseRole is the close reader role.
func (a *Alert) CloseRole() string { return "button" }

// Semantics builds the read tree (root alert + optional close button).
func (a *Alert) Semantics() *semantics.Node {
	if a == nil {
		return nil
	}
	label := a.title
	if a.ariaLabel != "" {
		label = a.ariaLabel
	}
	root := &semantics.Node{Role: semantics.Role("alert"), Label: label}
	if a.description != "" {
		root.Add(&semantics.Node{Role: semantics.RoleText, Label: a.description})
	}
	if a.closable.Load() && !a.hidden.Load() {
		root.Add(&semantics.Node{Role: semantics.RoleButton, Label: a.CloseAria(), Focusable: true})
	}
	return root
}

// Node returns the tree node (layout/paint/hit through it).
func (a *Alert) Node() rendering.RenderObject {
	if a == nil {
		return nil
	}
	a.syncNode()
	return a.node
}

// ChromeNode returns the decorated root (same node, single boundary).
func (a *Alert) ChromeNode() rendering.RenderObject { return a.Node() }

// CloseNode returns the close hit box (44px, hit == layout == paint).
func (a *Alert) CloseNode() rendering.RenderObject {
	if a == nil {
		return nil
	}
	b := rendering.NewRenderColorBox(closeHitSize, closeHitSize, 0, 0, 0, 0)
	b.SetRepaintBoundary(false)
	return b
}

// CloseHitSize exposes the a11y target for tests.
func (a *Alert) CloseHitSize() float64 { return closeHitSize }

func (a *Alert) themeTokens() theme.Tokens {
	if a != nil && a.override != nil {
		return *a.override
	}
	if a != nil && a.provider != nil {
		return a.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func lighten(c theme.Color, t float64) render.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	a := c.A
	if a <= 0 {
		a = 1
	}
	return render.RGBA{R: c.R + (1-c.R)*t, G: c.G + (1-c.G)*t, B: c.B + (1-c.B)*t, A: a}
}

func (a *Alert) semantic() theme.Color {
	tok := a.themeTokens()
	switch a.EffectiveType() {
	case AlertSuccess:
		return tok.ColorSuccess
	case AlertWarning:
		return tok.ColorWarning
	case AlertError:
		return tok.ColorError
	default:
		return tok.ColorInfo
	}
}

// Background is the tinted shell (theme semantic lightened, never a bare hex).
func (a *Alert) Background() render.RGBA { return lighten(a.semantic(), 0.88) }

// IconColor is the full semantic icon color.
func (a *Alert) IconColor() render.RGBA { return themeToRGBA(a.semantic()) }

// BorderColor is the outlined edge (transparent when borderless).
func (a *Alert) BorderColor() render.RGBA {
	if !a.HasBorder() {
		return render.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	return lighten(a.semantic(), 0.55)
}

// HasBorder reports outlined && !banner (ALT-S7).
func (a *Alert) HasBorder() bool {
	if a == nil || a.hidden.Load() {
		return false
	}
	if a.banner {
		return false
	}
	return a.Variant() == AlertOutlined
}

// LineWidth is theme lineWidth when bordered, else 0.
func (a *Alert) LineWidth() float64 {
	if !a.HasBorder() {
		return 0
	}
	return a.themeTokens().LineWidth
}

// Radius is LG normally, 0 for banner (ALT-S5).
func (a *Alert) Radius() float64 {
	if a != nil && a.banner {
		return 0
	}
	return a.themeTokens().RadiusLG
}

// PadH is 12 normally, 24 with description (theme paddings, no magic).
func (a *Alert) PadH() float64 {
	tok := a.themeTokens()
	if a.HasDescription() {
		return tok.PaddingLG
	}
	return tok.PaddingSM
}

// PadV is 8 normally, 20 with description.
func (a *Alert) PadV() float64 {
	tok := a.themeTokens()
	if a.HasDescription() {
		return tok.PaddingMD
	}
	return tok.PaddingXS
}

// FontSize is the body size (description + plain title).
func (a *Alert) FontSize() float64 { return a.themeTokens().FontSize }

// TitleFontSize is 16 with description, else 14 (ALT-S3).
func (a *Alert) TitleFontSize() float64 {
	if a.HasDescription() {
		return a.themeTokens().FontSizeLG
	}
	return a.themeTokens().FontSize
}

// IconSize is 14 normally, 24 with description (§6.2.1).
func (a *Alert) IconSize() float64 {
	if a.HasDescription() {
		return a.themeTokens().SizeLG
	}
	return a.themeTokens().FontSize
}

func (a *Alert) markPaint() {
	if a == nil || a.node == nil {
		return
	}
	a.node.MarkNeedsPaint()
}

func (a *Alert) markLayout() {
	if a == nil || a.node == nil {
		return
	}
	a.node.MarkNeedsLayout()
}

func measureNode(n rendering.RenderObject) (float64, float64) {
	if n == nil {
		return 0, 0
	}
	sz := n.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return sz.Width, sz.Height
}

// ideal computes the unconstrained shell size.
func (a *Alert) ideal() (float64, float64) {
	tok := a.themeTokens()
	padH, padV := a.PadH(), a.PadV()
	titleFont := a.TitleFontSize()
	var titleW, titleH float64
	if a.titleNode != nil {
		titleW, titleH = measureNode(a.titleNode)
	} else {
		titleW, titleH = rendering.EstimateTextSize(a.title, titleFont, 0.55)
	}
	var descW, descH float64
	hasDesc := a.HasDescription()
	if hasDesc {
		if a.descNode != nil {
			descW, descH = measureNode(a.descNode)
		} else {
			descW, descH = rendering.EstimateTextSize(a.description, tok.FontSize, 0.55)
		}
	}
	contentW := titleW
	if descW > contentW {
		contentW = descW
	}
	contentH := titleH
	if hasDesc {
		contentH += tok.MarginXXS + descH
	}
	iconW := 0.0
	iconSize := a.IconSize()
	if a.IconVisible() {
		gap := tok.MarginXS
		if hasDesc {
			gap = tok.MarginSM
		}
		customW := 0.0
		if a.iconNode != nil {
			customW, _ = measureNode(a.iconNode)
			if customW > iconSize {
				iconSize = customW
			}
		}
		iconW = iconSize + gap
	}
	actionW, actionH := 0.0, 0.0
	if a.ActionVisible() {
		aw, ah := measureNode(a.action)
		actionW = aw + tok.MarginXS
		actionH = ah
	}
	closeW, closeH := 0.0, 0.0
	if a.closable.Load() && !a.hidden.Load() {
		closeW = closeHitSize + tok.MarginXS
		closeH = closeHitSize
	}
	rowH := contentH
	if a.IconVisible() && iconSize > rowH {
		rowH = iconSize
	}
	if actionH > rowH {
		rowH = actionH
	}
	if closeH > rowH {
		rowH = closeH
	}
	return padH*2 + iconW + contentW + actionW + closeW, padV*2 + rowH
}

// Layout sizes the shell under constraints (Exact/Min/Max matrix).
func (a *Alert) Layout(c rendering.Constraints) rendering.Size {
	if a == nil {
		return rendering.Size{}
	}
	a.syncNode()
	if a.hidden.Load() {
		a.node.FixedWidth, a.node.FixedHeight = 0, 0
		sz := a.node.Layout(rendering.Tight(0, 0))
		a.cachedSize = sz
		return sz
	}
	iw, ih := a.ideal()
	if a.banner && c.MaxWidth > 0 && c.MaxWidth < rendering.Unbounded/2 {
		iw = c.MaxWidth
	}
	out := c.Tighten(rendering.Size{Width: iw, Height: ih})
	a.node.FixedWidth, a.node.FixedHeight = out.Width, out.Height
	sz := a.node.Layout(c)
	a.cachedSize = sz
	return sz
}

// rect is a local box for paint/hit sharing.
type rect struct{ x, y, w, h float64 }

// geometry splits the shell into icon/title/desc/action/close boxes.
func (a *Alert) geometry(tok theme.Tokens, w, h float64) (icon, title, desc, action, close rect) {
	padH, padV := a.PadH(), a.PadV()
	titleFont := a.TitleFontSize()
	var titleW, titleH float64
	if a.titleNode != nil {
		if sz := a.titleNode.Size(); sz.Width > 0 {
			titleW, titleH = sz.Width, sz.Height
		} else {
			titleW, titleH = measureNode(a.titleNode)
		}
	} else {
		titleW, titleH = rendering.EstimateTextSize(a.title, titleFont, 0.55)
	}
	var descW, descH float64
	hasDesc := a.HasDescription()
	if hasDesc {
		if a.descNode != nil {
			if sz := a.descNode.Size(); sz.Width > 0 {
				descW, descH = sz.Width, sz.Height
			} else {
				descW, descH = measureNode(a.descNode)
			}
		} else {
			descW, descH = rendering.EstimateTextSize(a.description, tok.FontSize, 0.55)
		}
	}
	contentH := titleH
	if hasDesc {
		contentH += tok.MarginXXS + descH
	}
	rowH := h - padV*2
	if rowH < 0 {
		rowH = 0
	}
	iconSize := a.IconSize()
	iconW := 0.0
	if a.IconVisible() {
		gap := tok.MarginXS
		if hasDesc {
			gap = tok.MarginSM
		}
		iconW = iconSize + gap
	}
	var actionW, actionH float64
	if a.ActionVisible() {
		if sz := a.action.Size(); sz.Width > 0 {
			actionW, actionH = sz.Width, sz.Height
		} else {
			actionW, actionH = measureNode(a.action)
		}
	}
	closeW := 0.0
	if a.closable.Load() && !a.hidden.Load() {
		closeW = closeHitSize + tok.MarginXS
	}
	// Right-anchored close/action keep hit == paint when clamped.
	closeX := w - padH - closeHitSize
	actionX := closeX
	if a.ActionVisible() {
		actBoxW := actionW
		if a.closable.Load() {
			actionX = closeX - tok.MarginXS - actBoxW
		} else {
			actionX = w - padH - actBoxW
		}
	}
	left := padH
	if a.rtl {
		// Mirror: icon right, close left.
		if a.IconVisible() {
			icon.x = w - padH - iconSize
			if hasDesc {
				icon.y = padV
			} else {
				icon.y = padV + (rowH-iconSize)/2
			}
			icon.w, icon.h = iconSize, iconSize
			left = padH
		}
		contentW := w - padH*2 - iconW - actionW - closeW
		if contentW < 0 {
			contentW = 0
		}
		cx := padH
		if a.closable.Load() {
			cx += closeW
		}
		if a.ActionVisible() {
			cx += actionW
		}
		cy := padV
		if !hasDesc {
			cy = padV + (rowH-contentH)/2
		}
		title.x, title.y = cx, cy
		title.w, title.h = minF(titleW, contentW), titleH
		if hasDesc {
			desc.x = cx
			desc.y = cy + titleH + tok.MarginXXS
			desc.w, desc.h = minF(descW, contentW), descH
		}
		if a.closable.Load() {
			close.x = padH
			close.y = padV + (rowH-closeHitSize)/2
			close.w, close.h = closeHitSize, closeHitSize
		}
		if a.ActionVisible() {
			ax := padH
			if a.closable.Load() {
				ax += closeW
			}
			action.x = ax
			action.y = padV + (rowH-actionH)/2
			action.w, action.h = actionW, actionH
		}
		_ = left
		return icon, title, desc, action, close
	}
	if a.IconVisible() {
		icon.x = left
		if hasDesc {
			icon.y = padV
		} else {
			icon.y = padV + (rowH-iconSize)/2
		}
		icon.w, icon.h = iconSize, iconSize
		left += iconW
	}
	cy := padV
	if !hasDesc {
		cy = padV + (rowH-contentH)/2
	}
	title.x, title.y = left, cy
	avail := w - left - padH - actionW - closeW
	if avail < 0 {
		avail = 0
	}
	title.w, title.h = minF(titleW, avail), titleH
	if hasDesc {
		desc.x = left
		desc.y = cy + titleH + tok.MarginXXS
		desc.w, desc.h = minF(descW, avail), descH
	}
	if a.ActionVisible() {
		action.x, action.y = actionX, padV+(rowH-actionH)/2
		action.w, action.h = actionW, actionH
	}
	if a.closable.Load() && !a.hidden.Load() {
		close.x, close.y = closeX, padV+(rowH-closeHitSize)/2
		close.w, close.h = closeHitSize, closeHitSize
	}
	return icon, title, desc, action, close
}

func minF(x, y float64) float64 {
	if x < y {
		return x
	}
	return y
}

// paintText draws real glyphs when a text face is set (same chain as
// button/tag: SetFont+DrawString via pc.Abs). Without a face it draws
// nothing: headless layout still estimates width, but no black bar is
// ever painted (black bars are banned; DrawString without a face is a
// safe no-op so the guard below simply skips).
func (a *Alert) paintText(pc *rendering.PaintContext, s string, x, y, w, h, fontSize float64, c render.RGBA) {
	if pc == nil || pc.DC == nil || s == "" || w <= 0 || h <= 0 {
		return
	}
	if a == nil || a.textFace == nil {
		return
	}
	pc.DC.SetFont(a.textFace)
	pc.DC.SetRGBA(c.R, c.G, c.B, c.A)
	baseline := y + h/2 + fontSize*0.35
	ax, ay := pc.Abs(x, baseline)
	pc.DC.DrawString(s, ax, ay)
}

func (a *Alert) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || a == nil || a.hidden.Load() {
		return
	}
	tok := a.themeTokens()
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	bg := a.Background()
	radius := a.Radius()
	if radius <= 0 {
		rendering.FillRect(pc, 0, 0, w, h, bg.R, bg.G, bg.B, bg.A)
	} else {
		rendering.FillRoundRect(pc, 0, 0, w, h, radius, bg.R, bg.G, bg.B, bg.A)
	}
	if a.HasBorder() {
		bd := a.BorderColor()
		lw := a.LineWidth()
		if lw <= 0 {
			lw = 1
		}
		if radius <= 0 {
			rendering.StrokeRect(pc, lw/2, lw/2, w-lw, h-lw, lw, bd.R, bd.G, bd.B, bd.A)
		} else {
			rendering.StrokeRoundRect(pc, lw/2, lw/2, w-lw, h-lw, radius, lw, bd.R, bd.G, bd.B, bd.A)
		}
	}
	icon, title, desc, action, close := a.geometry(tok, w, h)
	if a.IconVisible() && icon.w > 0 {
		ic := a.IconColor()
		if a.iconNode != nil {
			a.iconNode.Paint(pc.WithOrigin(pc.OriginX+icon.x, pc.OriginY+icon.y))
		} else {
			rendering.FillCircle(pc, icon.x+icon.w/2, icon.y+icon.h/2, icon.w*0.32, ic.R, ic.G, ic.B, ic.A)
			rendering.FillRect(pc, icon.x+icon.w*0.42, icon.y+icon.h*0.30, icon.w*0.16, icon.h*0.40,
				1, 1, 1, 1)
		}
	}
	textC := themeToRGBA(tok.ColorText)
	if a.titleNode != nil {
		a.titleNode.Paint(pc.WithOrigin(pc.OriginX+title.x, pc.OriginY+title.y))
	} else {
		a.paintText(pc, a.title, title.x, title.y, title.w, title.h, a.TitleFontSize(), textC)
	}
	if a.HasDescription() {
		if a.descNode != nil {
			a.descNode.Paint(pc.WithOrigin(pc.OriginX+desc.x, pc.OriginY+desc.y))
		} else {
			a.paintText(pc, a.description, desc.x, desc.y, desc.w, desc.h, tok.FontSize, textC)
		}
	}
	if a.ActionVisible() {
		if a.action != nil {
			a.action.Paint(pc.WithOrigin(pc.OriginX+action.x, pc.OriginY+action.y))
		} else if action.w > 0 {
			rendering.FillRoundRect(pc, action.x, action.y, action.w, action.h, 4,
				textC.R, textC.G, textC.B, 0.12)
		}
		_ = action
	}
	if a.closable.Load() && !a.hidden.Load() && close.w > 0 {
		if effClose := a.EffectiveCloseIcon(); effClose != nil {
			effClose.Paint(pc.WithOrigin(pc.OriginX+close.x, pc.OriginY+close.y))
		} else {
			cc := themeToRGBA(tok.ColorTextTertiary)
			cx, cy := close.x+close.w/2, close.y+close.h/2
			rendering.StrokeLine(pc, cx-closeGlyphHalf, cy-closeGlyphHalf, cx+closeGlyphHalf, cy+closeGlyphHalf,
				1.6, cc.R, cc.G, cc.B, cc.A)
			rendering.StrokeLine(pc, cx+closeGlyphHalf, cy-closeGlyphHalf, cx-closeGlyphHalf, cy+closeGlyphHalf,
				1.6, cc.R, cc.G, cc.B, cc.A)
		}
		if a.focused.Load() {
			fc := themeToRGBA(tok.ColorPrimary)
			rendering.StrokeRoundRect(pc, close.x-1.5, close.y-1.5, close.w+3, close.h+3, 6, 2, fc.R, fc.G, fc.B, fc.A)
		}
	}
}
