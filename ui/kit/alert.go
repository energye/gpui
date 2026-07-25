package kit

import (
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Alert tokens — components/alert/style/index.ts
// docs/antd/alert.md §6.2
const (
	// DefaultAlertPadV is paddingContentVerticalSM (no description).
	DefaultAlertPadV = 8.0
	// DefaultAlertPadH is the fixed horizontal pad in prepareComponentToken.
	DefaultAlertPadH = 12.0
	// DefaultAlertDescPadV is paddingMD (with description).
	DefaultAlertDescPadV = 16.0
	// DefaultAlertDescPadH is paddingContentHorizontalLG (with description).
	DefaultAlertDescPadH = 24.0
	// DefaultAlertFontSize is title/description fontSize.
	DefaultAlertFontSize = 14.0
	// DefaultAlertTitleFontLG is title fontSizeLG when description is present.
	DefaultAlertTitleFontLG = 16.0
	// DefaultAlertRadius is borderRadiusLG.
	DefaultAlertRadius = 8.0
	// DefaultAlertLineWidth is lineWidth.
	DefaultAlertLineWidth = 1.0
	// DefaultAlertIconSize is the simple (no-description) icon size.
	DefaultAlertIconSize = 14.0
	// DefaultAlertDescIconSize is withDescriptionIconSize (= fontSizeHeading3).
	DefaultAlertDescIconSize = 24.0
	// DefaultAlertIconGap is marginXS (icon marginInlineEnd, simple).
	DefaultAlertIconGap = 8.0
	// DefaultAlertDescIconGap is marginSM (icon marginInlineEnd with description).
	DefaultAlertDescIconGap = 12.0
	// DefaultAlertTitleDescGap is marginXS between title and description.
	DefaultAlertTitleDescGap = 8.0
	// DefaultAlertActionGap is marginXS for action / close marginInlineStart.
	DefaultAlertActionGap = 8.0
	// DefaultAlertFocusOutset approximates focus-visible outset on close.
	DefaultAlertFocusOutset = 1.5
	// DefaultAlertCloseAria is the default accessible name for close.
	DefaultAlertCloseAria = "Close"

	// Fallback light skins when Theme has no *Bg / *Border tokens (antd palette[1]/[3]).
	defaultAlertSuccessBG     = "#F6FFED"
	defaultAlertSuccessBorder = "#B7EB8F"
	defaultAlertInfoBG        = "#E6F4FF"
	defaultAlertInfoBorder    = "#91CAFF"
	defaultAlertWarningBG     = "#FFFBE6"
	defaultAlertWarningBorder = "#FFE58F"
	defaultAlertErrorBG       = "#FFF2F0"
	defaultAlertErrorBorder   = "#FFCCC7"
)

// AlertType is antd Alert type.
type AlertType string

const (
	// AlertInfo is the default type (banner defaults to warning).
	AlertInfo AlertType = "info"
	// AlertSuccess is success green semantics.
	AlertSuccess AlertType = "success"
	// AlertWarning is warning orange semantics.
	AlertWarning AlertType = "warning"
	// AlertError is error red semantics.
	AlertError AlertType = "error"
)

// AlertVariant is antd Alert variant (6.4.0+).
type AlertVariant int

const (
	// AlertOutlined is the default bordered skin.
	AlertOutlined AlertVariant = iota
	// AlertFilled is filled skin with transparent border.
	AlertFilled
)

// AlertCloseEvent is passed to OnClose; call PreventDefault to keep visible.
type AlertCloseEvent struct {
	prevented bool
}

// PreventDefault blocks the default hide behavior (antd e.preventDefault()).
func (e *AlertCloseEvent) PreventDefault() {
	if e != nil {
		e.prevented = true
	}
}

// DefaultPrevented reports whether PreventDefault was called.
func (e *AlertCloseEvent) DefaultPrevented() bool {
	return e != nil && e.prevented
}

// Alert is Ant Design Alert (feedback banner).
//
//	Decorated Root (role=alert)
//	  └─ Row(Icon? · Flexible(Section) · Action? · Close?)
//
// Product contract: docs/antd/alert.md §6 (P0 DoD).
type Alert struct {
	Root     *primitive.Decorated
	row      *primitive.Flex
	titleEl  *primitive.Text
	descEl   *primitive.Text
	closeBtn *primitive.Pressable
	iconNode core.Node
	section  *primitive.Flex

	// Title is the primary content (antd title; replaces deprecated message).
	Title string
	// TitleNode overrides Title when non-nil.
	TitleNode core.Node
	// Description is secondary helper text.
	Description string
	// DescriptionNode overrides Description when non-nil.
	DescriptionNode core.Node

	// Type is success|info|warning|error. Empty + Banner → warning; else info.
	Type    AlertType
	typeSet bool

	Variant AlertVariant

	// ShowIcon shows the leading icon. Unset + Banner → true.
	ShowIcon    bool
	showIconSet bool

	// Icon is a registry icon name used when ShowIcon and IconNode is nil.
	Icon string
	// IconNode is a custom leading icon (preferred over Icon / default).
	IconNode core.Node

	Banner   bool
	Closable bool
	// CloseIcon replaces the default "×" when Closable.
	CloseIcon core.Node
	// CloseAria is the accessible name for close (default "Close").
	CloseAria string
	// Action is an optional trailing action node (Button, etc.).
	Action core.Node

	// OnClose fires on close click; may PreventDefault to keep visible.
	OnClose func(*AlertCloseEvent)
	// AfterClose fires after hide (P0: immediately after hide; no leave motion).
	AfterClose func()

	// Hidden is set after a successful close (antd display:none / unmount).
	Hidden bool

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// resolved chrome (tests / L2)
	fontSize      float64
	titleFontSize float64
	padH, padV    float64
	radius        float64
	lineW         float64
	iconSz        float64
	iconGap       float64
	bg, fg, iconC render.RGBA
	bd            render.RGBA
	hasBorder     bool
	hasDesc       bool
}

// NewAlert creates an Alert with antd defaults (type=info, outlined, no icon).
func NewAlert(title string) *Alert {
	a := &Alert{
		Title:   title,
		Type:    AlertInfo,
		Variant: AlertOutlined,
	}
	a.rebuild()
	return a
}

// Node returns the mount root.
func (a *Alert) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// ChromeNode returns the Decorated chrome (tests / composition).
func (a *Alert) ChromeNode() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// CloseNode returns the close Pressable when closable, else nil.
func (a *Alert) CloseNode() core.Node {
	if a == nil {
		return nil
	}
	return a.closeBtn
}

// IconVisible reports whether the leading icon is shown.
func (a *Alert) IconVisible() bool {
	return a != nil && a.resolveShowIcon()
}

// Visible reports whether the alert is shown (not Hidden).
func (a *Alert) Visible() bool {
	return a != nil && !a.Hidden
}

// ResolvedType returns the effective type after banner defaults.
func (a *Alert) ResolvedType() AlertType {
	if a == nil {
		return AlertInfo
	}
	return a.resolveType()
}

// FontSize returns resolved body/description font size (L2).
func (a *Alert) FontSize() float64 {
	a.resolveMetrics()
	return a.fontSize
}

// TitleFontSize returns resolved title font size (L2).
func (a *Alert) TitleFontSize() float64 {
	a.resolveMetrics()
	return a.titleFontSize
}

// PadH returns resolved horizontal padding (L2).
func (a *Alert) PadH() float64 {
	a.resolveMetrics()
	return a.padH
}

// PadV returns resolved vertical padding (L2).
func (a *Alert) PadV() float64 {
	a.resolveMetrics()
	return a.padV
}

// Radius returns resolved corner radius (L2).
func (a *Alert) Radius() float64 {
	a.resolveMetrics()
	return a.radius
}

// LineWidth returns resolved border width (L2; 0 when no border).
func (a *Alert) LineWidth() float64 {
	a.resolveChrome()
	if !a.hasBorder {
		return 0
	}
	return a.lineW
}

// Background returns resolved fill (L2).
func (a *Alert) Background() render.RGBA {
	a.resolveChrome()
	return a.bg
}

// IconColor returns resolved icon color (L2).
func (a *Alert) IconColor() render.RGBA {
	a.resolveChrome()
	return a.iconC
}

// BorderColor returns resolved border color (L2).
func (a *Alert) BorderColor() render.RGBA {
	a.resolveChrome()
	return a.bd
}

// HasBorder reports whether a border is drawn (L2).
func (a *Alert) HasBorder() bool {
	a.resolveChrome()
	return a.hasBorder
}

// HasDescription reports whether description content is present.
func (a *Alert) HasDescription() bool {
	return a != nil && a.resolveHasDesc()
}

// SetTitle updates the title text.
func (a *Alert) SetTitle(s string) {
	if a == nil {
		return
	}
	a.Title = s
	if a.titleEl != nil && a.TitleNode == nil {
		a.titleEl.Value = s
		a.titleEl.MarkNeedsLayout()
		a.titleEl.MarkNeedsPaint()
		a.applyA11y()
		return
	}
	a.rebuild()
}

// SetMessage is a deprecated alias for SetTitle (antd message → title).
func (a *Alert) SetMessage(s string) { a.SetTitle(s) }

// Message returns Title (compat with older callers reading .Message).
func (a *Alert) Message() string {
	if a == nil {
		return ""
	}
	return a.Title
}

// SetTitleNode sets a custom title node (loop-banner marquee, etc.).
func (a *Alert) SetTitleNode(n core.Node) {
	if a == nil {
		return
	}
	a.TitleNode = n
	a.rebuild()
}

// SetDescription sets secondary description text.
func (a *Alert) SetDescription(s string) {
	if a == nil {
		return
	}
	a.Description = s
	a.rebuild()
}

// SetDescriptionNode sets a custom description node.
func (a *Alert) SetDescriptionNode(n core.Node) {
	if a == nil {
		return
	}
	a.DescriptionNode = n
	a.rebuild()
}

// SetType sets semantic type (success|info|warning|error or free string).
func (a *Alert) SetType(typ any) {
	if a == nil {
		return
	}
	switch v := typ.(type) {
	case AlertType:
		a.Type = normalizeAlertType(string(v))
	case string:
		a.Type = normalizeAlertType(v)
	default:
		a.Type = AlertInfo
	}
	a.typeSet = true
	a.rebuild()
}

// SetVariant sets outlined | filled.
func (a *Alert) SetVariant(v AlertVariant) {
	if a == nil {
		return
	}
	a.Variant = v
	a.rebuild()
}

// SetBanner toggles banner mode (top announcement skin + defaults).
func (a *Alert) SetBanner(on bool) {
	if a == nil {
		return
	}
	a.Banner = on
	a.rebuild()
}

// SetShowIcon toggles the leading icon explicitly.
func (a *Alert) SetShowIcon(on bool) {
	if a == nil {
		return
	}
	a.ShowIcon = on
	a.showIconSet = true
	a.rebuild()
}

// SetIcon sets a registry icon name (used when showIcon).
func (a *Alert) SetIcon(name string) {
	if a == nil {
		return
	}
	a.Icon = strings.TrimSpace(name)
	a.rebuild()
}

// SetIconNode sets a custom leading icon node.
func (a *Alert) SetIconNode(n core.Node) {
	if a == nil {
		return
	}
	a.IconNode = n
	a.rebuild()
}

// SetClosable toggles the close control.
func (a *Alert) SetClosable(on bool) {
	if a == nil {
		return
	}
	a.Closable = on
	a.rebuild()
}

// SetCloseIcon sets a custom close node (nil → default "×").
func (a *Alert) SetCloseIcon(n core.Node) {
	if a == nil {
		return
	}
	a.CloseIcon = n
	a.rebuild()
}

// SetCloseAria sets the accessible name for the close control.
func (a *Alert) SetCloseAria(name string) {
	if a == nil {
		return
	}
	a.CloseAria = name
	if a.closeBtn != nil {
		aria := name
		if aria == "" {
			aria = DefaultAlertCloseAria
		}
		a.closeBtn.Base().Label = aria
	}
}

// SetOnClose sets a simple close callback (always allows hide unless
// OnClose field is used with PreventDefault).
func (a *Alert) SetOnClose(fn func()) {
	if a == nil {
		return
	}
	if fn == nil {
		a.OnClose = nil
		return
	}
	a.OnClose = func(*AlertCloseEvent) { fn() }
}

// SetAfterClose sets the post-hide callback (P0: fires immediately after hide).
func (a *Alert) SetAfterClose(fn func()) {
	if a == nil {
		return
	}
	a.AfterClose = fn
}

// SetAction sets the trailing action node.
func (a *Alert) SetAction(n core.Node) {
	if a == nil {
		return
	}
	a.Action = n
	a.rebuild()
}

// SetHidden forces visibility (tests / parent recovery).
func (a *Alert) SetHidden(v bool) {
	if a == nil {
		return
	}
	a.Hidden = v
	a.rebuild()
}

// SetTheme sets the theme override.
func (a *Alert) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	a.rebuild()
}

// SetFace sets the text face.
func (a *Alert) SetFace(face text.Face) {
	if a == nil {
		return
	}
	a.Face = face
	if a.titleEl != nil {
		a.titleEl.Face = face
		a.titleEl.MarkNeedsPaint()
	}
	if a.descEl != nil {
		a.descEl.Face = face
		a.descEl.MarkNeedsPaint()
	}
}

// SetStyle applies optional Style overrides.
func (a *Alert) SetStyle(st Style) {
	if a == nil {
		return
	}
	a.Style = st
	if st.Face != nil {
		a.Face = st.Face
	}
	a.rebuild()
}

// SetAriaLabel sets the accessible name override.
func (a *Alert) SetAriaLabel(name string) {
	if a == nil {
		return
	}
	a.AriaLabel = name
	a.applyA11y()
}

func (a *Alert) theme() *core.Theme {
	var n core.Node
	if a.Root != nil {
		n = a.Root
	}
	return themeOf(a.Theme, n)
}

func normalizeAlertType(s string) AlertType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "success":
		return AlertSuccess
	case "warning":
		return AlertWarning
	case "error":
		return AlertError
	case "info", "":
		return AlertInfo
	default:
		return AlertType(strings.ToLower(strings.TrimSpace(s)))
	}
}

func (a *Alert) resolveType() AlertType {
	if a.typeSet && a.Type != "" {
		return normalizeAlertType(string(a.Type))
	}
	if a.Banner && !a.typeSet {
		return AlertWarning
	}
	if a.Type != "" {
		return normalizeAlertType(string(a.Type))
	}
	return AlertInfo
}

func (a *Alert) resolveShowIcon() bool {
	if a.showIconSet {
		return a.ShowIcon
	}
	if a.Banner {
		return true
	}
	return a.ShowIcon
}

func (a *Alert) resolveHasDesc() bool {
	return a.DescriptionNode != nil || strings.TrimSpace(a.Description) != ""
}

func (a *Alert) resolveMetrics() {
	if a == nil {
		return
	}
	th := a.theme()
	a.hasDesc = a.resolveHasDesc()
	a.fontSize = th.SizeOr(core.TokenFontSize, DefaultAlertFontSize)
	if a.Style.FontSize > 0 {
		a.fontSize = a.Style.FontSize
	}
	a.titleFontSize = a.fontSize
	if a.hasDesc {
		a.titleFontSize = th.SizeOr(core.TokenFontSizeLG, DefaultAlertTitleFontLG)
	}
	a.lineW = th.SizeOr(core.TokenLineWidth, DefaultAlertLineWidth)
	if a.Banner {
		a.radius = 0
	} else {
		a.radius = th.SizeOr(core.TokenBorderRadiusLG, DefaultAlertRadius)
	}
	if a.Style.hasRadius() && !a.Banner {
		a.radius = a.Style.Radius
	}
	if a.hasDesc {
		a.padV = th.SizeOr(core.TokenPadding, DefaultAlertDescPadV)
		a.padH = th.SizeOr(core.TokenPaddingLG, DefaultAlertDescPadH)
		// antd: paddingMD × paddingContentHorizontalLG — LG is 24.
		if a.padH < DefaultAlertDescPadH {
			a.padH = DefaultAlertDescPadH
		}
		a.iconSz = DefaultAlertDescIconSize
		a.iconGap = th.SizeOr(core.TokenMarginSM, DefaultAlertDescIconGap)
	} else {
		a.padV = th.SizeOr(core.TokenPaddingSM, DefaultAlertPadV)
		a.padH = DefaultAlertPadH
		a.iconSz = DefaultAlertIconSize
		a.iconGap = th.SizeOr(core.TokenMarginXS, DefaultAlertIconGap)
		// marginXS seed is 4 in some themes; antd Alert icon gap uses marginXS
		// but visual rhythm is 8 with default icons — prefer Default when token < 8.
		if a.iconGap < DefaultAlertIconGap {
			a.iconGap = DefaultAlertIconGap
		}
	}
}

func (a *Alert) resolveChrome() {
	if a == nil {
		return
	}
	a.resolveMetrics()
	th := a.theme()
	typ := a.resolveType()

	// semantic icon color + light bg/border
	var icon, bg, bd render.RGBA
	switch typ {
	case AlertSuccess:
		icon = th.Color(core.TokenColorSuccess)
		if icon.A == 0 {
			icon = render.Hex("#52C41A")
		}
		bg = render.Hex(defaultAlertSuccessBG)
		bd = render.Hex(defaultAlertSuccessBorder)
	case AlertWarning:
		icon = th.Color(core.TokenColorWarning)
		if icon.A == 0 {
			icon = render.Hex("#FAAD14")
		}
		bg = render.Hex(defaultAlertWarningBG)
		bd = render.Hex(defaultAlertWarningBorder)
	case AlertError:
		icon = th.Color(core.TokenColorError)
		if icon.A == 0 {
			icon = render.Hex("#FF4D4F")
		}
		bg = render.Hex(defaultAlertErrorBG)
		bd = render.Hex(defaultAlertErrorBorder)
	default: // info
		icon = th.Color(core.TokenColorPrimary)
		if icon.A == 0 {
			icon = render.Hex("#1677FF")
		}
		bg = th.Color(core.TokenColorPrimaryBg)
		if bg.A == 0 {
			bg = render.Hex(defaultAlertInfoBG)
		}
		bd = th.Color(core.TokenColorPrimaryBorder)
		if bd.A == 0 {
			bd = render.Hex(defaultAlertInfoBorder)
		}
	}

	// Prefer lighten of semantic when available (theme-driven skin).
	if c := icon; c.A > 0 {
		// keep explicit fallbacks above as antd-accurate defaults
		_ = c
	}

	a.iconC = icon
	a.bg = bg
	a.bd = bd
	a.fg = th.Color(core.TokenColorText)
	if a.fg.A == 0 {
		a.fg = render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
	}

	// border rules
	a.hasBorder = false
	switch {
	case a.Banner:
		a.hasBorder = false
		a.bd = render.RGBA{}
	case a.Variant == AlertFilled:
		a.hasBorder = false
		a.bd = render.RGBA{}
	default: // outlined
		a.hasBorder = true
	}

	if a.Style.hasBG() {
		a.bg = a.Style.Background
	}
	if a.Style.hasText() {
		a.fg = a.Style.Text
	}
	if a.Style.hasBorder() {
		a.bd = a.Style.Border
		a.hasBorder = true
	}
}

func (a *Alert) rebuild() {
	if a == nil {
		return
	}
	a.resolveChrome()
	th := a.theme()
	typ := a.resolveType()
	// keep Type field in sync with resolved value for callers reading a.Type
	if !a.typeSet {
		// expose effective default without marking set
		if a.Banner {
			a.Type = AlertWarning
		} else if a.Type == "" {
			a.Type = AlertInfo
		}
	} else {
		a.Type = typ
	}
	// expose ShowIcon effective for callers reading field after rebuild when banner
	if !a.showIconSet && a.Banner {
		a.ShowIcon = true
	}

	a.closeBtn = nil
	a.iconNode = nil
	a.titleEl = nil
	a.descEl = nil
	a.section = nil
	a.row = nil

	if a.Root == nil {
		a.Root = primitive.NewDecorated(nil)
	} else {
		a.Root.ClearChildren()
	}

	if a.Hidden {
		a.Root.Padding = primitive.EdgeInsets{}
		a.Root.BorderWidth = 0
		a.Root.BorderColor = render.RGBA{}
		a.Root.Background = render.RGBA{}
		a.Root.Radius = 0
		a.Root.Hit = core.HitTransparent
		a.Root.Base().Role = "alert"
		a.applyA11y()
		a.Root.MarkNeedsLayout()
		a.Root.MarkNeedsPaint()
		return
	}

	// --- title ---
	var titleNode core.Node
	if a.TitleNode != nil {
		titleNode = a.TitleNode
	} else {
		t := primitive.NewText(a.Title)
		t.FontSize = a.titleFontSize
		t.Face = a.Face
		t.Color = a.fg
		a.titleEl = t
		titleNode = t
	}

	// --- description ---
	var descNode core.Node
	if a.DescriptionNode != nil {
		descNode = a.DescriptionNode
	} else if strings.TrimSpace(a.Description) != "" {
		d := primitive.NewText(a.Description)
		d.FontSize = a.fontSize
		d.Face = a.Face
		d.Color = a.fg
		a.descEl = d
		descNode = d
	}

	// --- section ---
	sec := primitive.Column()
	sec.CrossAlign = core.CrossStart
	sec.MainAlign = core.MainStart
	if descNode != nil {
		sec.Gap = DefaultAlertTitleDescGap
	}
	if titleNode != nil {
		sec.AddChild(titleNode)
	}
	if descNode != nil {
		sec.AddChild(descNode)
	}
	a.section = sec

	// --- row ---
	row := primitive.Row()
	if a.hasDesc {
		row.CrossAlign = core.CrossStart
	} else {
		row.CrossAlign = core.CrossCenter
	}
	row.MainAlign = core.MainStart
	row.Gap = 0
	a.row = row

	// icon
	if a.resolveShowIcon() {
		var ic core.Node
		if a.IconNode != nil {
			ic = a.IconNode
		} else if a.Icon != "" {
			icon := NewIcon(a.Icon)
			icon.SetSize(a.iconSz)
			icon.SetColor(a.iconC)
			icon.SetTheme(th)
			ic = icon.Node()
		} else {
			// default type glyph via registry when possible
			name := defaultAlertIconName(typ)
			if name != "" {
				icon := NewIcon(name)
				icon.SetSize(a.iconSz)
				icon.SetColor(a.iconC)
				icon.SetTheme(th)
				ic = icon.Node()
			} else {
				// warning fallback glyph
				g := primitive.NewText(alertIcon(string(typ)))
				g.FontSize = a.iconSz
				g.Face = a.Face
				g.Color = a.iconC
				ic = g
			}
		}
		// wrap with end margin
		wrap := primitive.NewBox(ic)
		wrap.Padding = primitive.EdgeInsets{Right: a.iconGap}
		a.iconNode = wrap
		row.AddChild(wrap)
	}

	// flexible section
	flex := primitive.NewFlexible(1, sec)
	flex.FillChild = true
	row.AddChild(flex)

	// action
	if a.Action != nil {
		act := primitive.NewBox(a.Action)
		act.Padding = primitive.EdgeInsets{Left: DefaultAlertActionGap}
		row.AddChild(act)
	}

	// close
	if a.Closable {
		var closeChild core.Node
		if a.CloseIcon != nil {
			closeChild = a.CloseIcon
		} else {
			x := primitive.NewText("×")
			x.FontSize = a.fontSize
			x.Face = a.Face
			closeCol := th.Color(core.TokenColorTextTertiary)
			if closeCol.A == 0 {
				closeCol = th.Color(core.TokenColorTextSecondary)
			}
			if closeCol.A == 0 {
				closeCol = render.RGBA{R: 0, G: 0, B: 0, A: 0.45}
			}
			x.Color = closeCol
			closeChild = x
		}
		cp := primitive.NewPressable(closeChild)
		cp.Focusable = true
		cp.ShowFocusRing = true
		cp.FocusRingRadius = a.radius
		if a.radius <= 0 {
			cp.FocusRingRadius = 4
		}
		cp.FocusRingOutset = DefaultAlertFocusOutset
		cp.EnableRipple = false
		cp.Padding = primitive.EdgeInsets{Left: DefaultAlertActionGap}
		cp.Click = func() { a.handleClose() }
		aria := a.CloseAria
		if aria == "" {
			aria = DefaultAlertCloseAria
		}
		cp.Base().Role = "button"
		cp.Base().Label = aria
		a.closeBtn = cp
		row.AddChild(cp)
	}

	a.Root.AddChild(row)
	a.Root.Padding = primitive.EdgeInsets{
		Left: a.padH, Right: a.padH,
		Top: a.padV, Bottom: a.padV,
	}
	a.Root.Radius = a.radius
	a.Root.Background = a.bg
	if a.hasBorder {
		a.Root.BorderWidth = a.lineW
		a.Root.BorderColor = a.bd
	} else {
		a.Root.BorderWidth = 0
		a.Root.BorderColor = render.RGBA{}
	}
	a.Root.Hit = core.HitBlock
	a.Root.Base().Role = "alert"
	a.applyA11y()
	a.Root.MarkNeedsLayout()
	a.Root.MarkNeedsPaint()
}

func (a *Alert) handleClose() {
	if a == nil || a.Hidden {
		return
	}
	e := &AlertCloseEvent{}
	if a.OnClose != nil {
		a.OnClose(e)
	}
	if e.DefaultPrevented() {
		return
	}
	a.Hidden = true
	a.rebuild()
	if a.AfterClose != nil {
		a.AfterClose()
	}
}

func (a *Alert) applyA11y() {
	if a == nil || a.Root == nil {
		return
	}
	name := a.AriaLabel
	if name == "" {
		name = a.Title
	}
	if name != "" {
		a.Root.Base().Label = name
	}
	a.Root.Base().Role = "alert"
}

func defaultAlertIconName(typ AlertType) string {
	switch typ {
	case AlertSuccess:
		return "check"
	case AlertInfo:
		return "info"
	case AlertError:
		return "close"
	default:
		// no built-in warning glyph; caller uses text fallback
		return ""
	}
}
