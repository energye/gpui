package button

import (
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/focus"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Button types (docs/antd/button.md §1.3 type, syntax sugar over color+variant).
type ButtonType string

const (
	ButtonDefault ButtonType = "default"
	ButtonPrimary ButtonType = "primary"
	ButtonDashed  ButtonType = "dashed"
	ButtonText    ButtonType = "text"
	ButtonLink    ButtonType = "link"
)

// Button sizes (docs/antd/button.md §6.2.1).
type ButtonSize string

const (
	ButtonSmall  ButtonSize = "small"
	ButtonMiddle ButtonSize = "middle"
	ButtonLarge  ButtonSize = "large"
)

// Button shapes (docs/antd/button.md §1.3 shape).
type ButtonShape string

const (
	ButtonShapeDefault ButtonShape = "default"
	ButtonShapeCircle  ButtonShape = "circle"
	ButtonShapeRound   ButtonShape = "round"
)

// Button variants (docs/antd/button.md §1.3 variant; Auto derives from Type).
type ButtonVariant string

const (
	VariantAuto     ButtonVariant = "auto"
	VariantSolid    ButtonVariant = "solid"
	VariantOutlined ButtonVariant = "outlined"
	VariantDashed   ButtonVariant = "dashed"
	VariantFilled   ButtonVariant = "filled"
	VariantText     ButtonVariant = "text"
	VariantLink     ButtonVariant = "link"
)

// Button colors: P0 default|primary|danger plus P1 PresetColors and
// success/warning (docs/antd/button.md §6.8 P1, §6.7).
type ButtonColor string

const (
	ColorDefault ButtonColor = "default"
	ColorPrimary ButtonColor = "primary"
	ColorDanger  ButtonColor = "danger"
	// P1 extended semantic colors (theme ColorSuccess/ColorWarning).
	ColorSuccess ButtonColor = "success"
	ColorWarning ButtonColor = "warning"
	// P1 PresetColors full palette (antd 6.5.1 type PresetColors).
	ColorBlue     ButtonColor = "blue"
	ColorPurple   ButtonColor = "purple"
	ColorCyan     ButtonColor = "cyan"
	ColorGreen    ButtonColor = "green"
	ColorMagenta  ButtonColor = "magenta"
	ColorPink     ButtonColor = "pink"
	ColorRed      ButtonColor = "red"
	ColorOrange   ButtonColor = "orange"
	ColorYellow   ButtonColor = "yellow"
	ColorVolcano  ButtonColor = "volcano"
	ColorGeekBlue ButtonColor = "geekblue"
	ColorLime     ButtonColor = "lime"
	ColorGold     ButtonColor = "gold"
)

// presetHex is the P1 component palette base (button-owned tokens; global
// seed has no button presets). Bases follow antd seed defaultPresetColors.
func presetHex(c ButtonColor) (render.RGBA, bool) {
	switch c {
	case ColorBlue:
		return render.RGBA{R: 0x16 / 255.0, G: 0x77 / 255.0, B: 0xff / 255.0, A: 1}, true
	case ColorPurple:
		return render.RGBA{R: 0x72 / 255.0, G: 0x2e / 255.0, B: 0xd1 / 255.0, A: 1}, true
	case ColorCyan:
		return render.RGBA{R: 0x13 / 255.0, G: 0xc2 / 255.0, B: 0xc2 / 255.0, A: 1}, true
	case ColorGreen:
		return render.RGBA{R: 0x52 / 255.0, G: 0xc4 / 255.0, B: 0x1a / 255.0, A: 1}, true
	case ColorMagenta:
		return render.RGBA{R: 0xeb / 255.0, G: 0x2f / 255.0, B: 0x96 / 255.0, A: 1}, true
	case ColorPink:
		return render.RGBA{R: 0xeb / 255.0, G: 0x2f / 255.0, B: 0x96 / 255.0, A: 1}, true
	case ColorRed:
		return render.RGBA{R: 0xf5 / 255.0, G: 0x22 / 255.0, B: 0x2d / 255.0, A: 1}, true
	case ColorOrange:
		return render.RGBA{R: 0xfa / 255.0, G: 0x8c / 255.0, B: 0x16 / 255.0, A: 1}, true
	case ColorYellow:
		return render.RGBA{R: 0xfa / 255.0, G: 0xdb / 255.0, B: 0x14 / 255.0, A: 1}, true
	case ColorVolcano:
		return render.RGBA{R: 0xfa / 255.0, G: 0x54 / 255.0, B: 0x1c / 255.0, A: 1}, true
	case ColorGeekBlue:
		return render.RGBA{R: 0x2f / 255.0, G: 0x54 / 255.0, B: 0xeb / 255.0, A: 1}, true
	case ColorLime:
		return render.RGBA{R: 0xa0 / 255.0, G: 0xd9 / 255.0, B: 0x11 / 255.0, A: 1}, true
	case ColorGold:
		return render.RGBA{R: 0xfa / 255.0, G: 0xad / 255.0, B: 0x14 / 255.0, A: 1}, true
	}
	return render.RGBA{}, false
}

// presetStyle holds exact @ant-design/colors generate() derivatives per
// preset (hover=palette[5], active=palette[7], light=[1], lightHover=[2],
// lightActive=[3]; verified 2026-09-14 via published package).
type presetStyle struct {
	base, hover, active, light, lightHover, lightActive render.RGBA
}

func presetStyleFor(c ButtonColor) (presetStyle, bool) {
	hex := func(s string) render.RGBA {
		return render.RGBA{R: float64(hexByte(s, 1)) / 255.0, G: float64(hexByte(s, 3)) / 255.0, B: float64(hexByte(s, 5)) / 255.0, A: 1}
	}
	switch c {
	case ColorBlue:
		return presetStyle{hex("#1677ff"), hex("#4096ff"), hex("#0958d9"), hex("#e6f4ff"), hex("#bae0ff"), hex("#91caff")}, true
	case ColorPurple:
		return presetStyle{hex("#722ed1"), hex("#9254de"), hex("#531dab"), hex("#f9f0ff"), hex("#efdbff"), hex("#d3adf7")}, true
	case ColorCyan:
		return presetStyle{hex("#13c2c2"), hex("#36cfc9"), hex("#08979c"), hex("#e6fffb"), hex("#b5f5ec"), hex("#87e8de")}, true
	case ColorGreen:
		return presetStyle{hex("#52c41a"), hex("#73d13d"), hex("#389e0d"), hex("#f6ffed"), hex("#d9f7be"), hex("#b7eb8f")}, true
	case ColorMagenta, ColorPink:
		return presetStyle{hex("#eb2f96"), hex("#f759ab"), hex("#c41d7f"), hex("#fff0f6"), hex("#ffd6e7"), hex("#ffadd2")}, true
	case ColorRed:
		return presetStyle{hex("#f5222d"), hex("#ff4d4f"), hex("#cf1322"), hex("#fff1f0"), hex("#ffccc7"), hex("#ffa39e")}, true
	case ColorOrange:
		return presetStyle{hex("#fa8c16"), hex("#ffa940"), hex("#d46b08"), hex("#fff7e6"), hex("#ffe7ba"), hex("#ffd591")}, true
	case ColorYellow:
		return presetStyle{hex("#fadb14"), hex("#ffec3d"), hex("#d4b106"), hex("#feffe6"), hex("#ffffb8"), hex("#fffb8f")}, true
	case ColorVolcano:
		return presetStyle{hex("#fa541c"), hex("#ff7a45"), hex("#d4380d"), hex("#fff2e8"), hex("#ffd8bf"), hex("#ffbb96")}, true
	case ColorGeekBlue:
		return presetStyle{hex("#2f54eb"), hex("#597ef7"), hex("#1d39c4"), hex("#f0f5ff"), hex("#d6e4ff"), hex("#adc6ff")}, true
	case ColorGold:
		return presetStyle{hex("#faad14"), hex("#ffc53d"), hex("#d48806"), hex("#fffbe6"), hex("#fff1b8"), hex("#ffe58f")}, true
	case ColorLime:
		return presetStyle{hex("#a0d911"), hex("#bae637"), hex("#7cb305"), hex("#fcffe6"), hex("#f4ffb8"), hex("#eaff8f")}, true
	}
	return presetStyle{}, false
}

func hexByte(s string, i int) uint8 {
	hexVal := func(c byte) uint8 {
		switch {
		case c >= '0' && c <= '9':
			return c - '0'
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10
		}
		return 0
	}
	if i+1 >= len(s) {
		return 0
	}
	return hexVal(s[i])*16 + hexVal(s[i+1])
}

// IsPresetColor reports the 13 PresetColors palette membership.
func IsPresetColor(c ButtonColor) bool {
	_, ok := presetHex(c)
	return ok
}

// Icon placements, logical sides mirrored by RTL (§6.4 B-S9, §6.6).
type IconPlacement string

const (
	IconStart IconPlacement = "start"
	IconEnd   IconPlacement = "end"
)

// IconPosition is the deprecated alias of IconPlacement (5.17.0, use IconPlacement).
type IconPosition = IconPlacement

const (
	IconPositionStart IconPosition = IconStart
	IconPositionEnd   IconPosition = IconEnd
)

// HtmlType maps the native button type (desktop: thrown as event for Form).
type HtmlType string

const (
	HtmlButton HtmlType = "button"
	HtmlSubmit HtmlType = "submit"
	HtmlReset  HtmlType = "reset"
)

// SemanticKey names the classNames/styles hooks (P1 semantic DOM).
type SemanticKey string

const (
	SemanticRoot  SemanticKey = "root"
	SemanticLabel SemanticKey = "label"
	SemanticIcon  SemanticKey = "icon"
)

// Gradient is a two-stop linear background (P1 linear-gradient demo hook).
type Gradient struct {
	From, To render.RGBA
	Enabled  bool
}

// LoadingConfig is the object form of loading (P1 delay + custom icon).
type LoadingConfig struct {
	Delay time.Duration
	Icon  string
}

// GlobalConfig mirrors ConfigProvider button defaults (P1, button-owned so
// shared packages stay untouched). Unset fields leave NewButton defaults.
type GlobalConfig struct {
	Size            ButtonSize
	Variant         ButtonVariant
	Color           ButtonColor
	AutoInsertSpace *bool
	WaveDisabled    *bool
	ReducedMotion   *bool
	LoadingIcon     string
	HasLoadingIcon  bool
	HtmlType        HtmlType
	HasHtmlType     bool
}

var globalMu sync.RWMutex
var globalCfg GlobalConfig

// SetGlobalConfig installs ConfigProvider-style button defaults (P1).
func SetGlobalConfig(c GlobalConfig) {
	globalMu.Lock()
	globalCfg = c
	globalMu.Unlock()
}

// GetGlobalConfig returns the current global defaults snapshot.
func GetGlobalConfig() GlobalConfig {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalCfg
}

// ResetGlobalConfig clears global defaults to NewButton baselines.
func ResetGlobalConfig() {
	globalMu.Lock()
	globalCfg = GlobalConfig{}
	globalMu.Unlock()
}

func boolPtr(v bool) *bool {
	b := v
	return &b
}

// Component geometry fallback (docs/antd/button.md §6.2.1).
// Heights/font sizes prefer theme tokens; paddings/gaps live here because
// the global seed has no button paddings (tokens.go: component tokens land
// with each ui/kit/<name>/ package).
const (
	paddingInlineSM = 7.0
	paddingInline   = 15.0
	iconGapSM       = 4.0
	iconGap         = 8.0
	// minTouchTarget is the WCAG-flavored hit floor; visual size keeps the
	// spec height, only hit-testing expands (see HitSize).
	minTouchTarget = 44.0
	// dashOn/dashOff draw the dashed variant border (§6.5 dash period ~3-2).
	dashOn, dashOff = 3.0, 2.0
	// focusRingOutset matches the §6.2 focus ring baseline.
	focusRingOutset = 1.5
)

// Style is an optional business override (P1 gradient hook, §6.5/§6.7).
// Zero value disables every field; set a Use flag to enable one.
type Style struct {
	Bg, Border, Text          render.RGBA
	UseBg, UseBorder, UseText bool
}

// Button is the Button widget (docs/antd/button.md §6.10).
//
// It owns a rendering.RenderBox node: put Node() in the tree and drive
// layout through Layout (same contract as ui/kit/icon Icon). State follows
// the §6.4 machine; visuals follow the §6.5 chrome table.
type Button struct {
	label         string
	iconName      string
	iconPlacement IconPlacement
	typ           ButtonType
	size          ButtonSize
	shape         ButtonShape
	variant       ButtonVariant
	color         ButtonColor
	danger        bool
	ghost         bool
	block         bool
	disabled      bool
	loading       bool
	rtl           bool
	ariaLabel     string
	style         Style
	// P1 fields (docs/antd/button.md §6.7-§6.8).
	autoInsertSpace bool
	href            string
	target          string
	htmlType        HtmlType
	loadingDelay    time.Duration
	loadingStart    time.Time
	loadingIcon     string
	waveDisabled    bool
	reducedMotion   bool
	gradient        Gradient
	classNames      map[SemanticKey]string
	semanticStyles  map[SemanticKey]Style
	// OnNavigate maps href/target to a desktop open-URL callback (P1 href).
	OnNavigate func(href, target string)

	hovered bool
	pressed bool
	inBound bool // press started inside and pointer still inside
	focused bool

	// OnClick fires once per in-bounds press-release or keyboard activate.
	OnClick func()

	provider *theme.Provider
	override *theme.Tokens

	focusNode *focus.FocusNode
	// textFace is the paint-only font face (nil keeps headless rune
	// estimate; gallery sets it via SetTextFace so true windows draw text).
	textFace  text.Face
	node      *rendering.RenderBox
	lastSize  rendering.Size
}

// NewButton creates a default middle button (Type default, Variant auto).
// P1 global defaults (SetGlobalConfig) apply here; unset fields keep baselines.
func NewButton(label string) *Button {
	b := &Button{label: label, iconPlacement: IconStart, typ: ButtonDefault, size: ButtonMiddle, shape: ButtonShapeDefault, variant: VariantAuto, color: ColorDefault, autoInsertSpace: true, htmlType: HtmlButton}
	globalMu.RLock()
	g := globalCfg
	globalMu.RUnlock()
	if g.Size == ButtonSmall || g.Size == ButtonMiddle || g.Size == ButtonLarge {
		b.size = g.Size
	}
	switch g.Variant {
	case VariantSolid, VariantOutlined, VariantDashed, VariantFilled, VariantText, VariantLink:
		b.variant = g.Variant
	}
	if g.Color != "" {
		b.color = g.Color
	}
	if g.AutoInsertSpace != nil {
		b.autoInsertSpace = *g.AutoInsertSpace
	}
	if g.WaveDisabled != nil {
		b.waveDisabled = *g.WaveDisabled
	}
	if g.ReducedMotion != nil {
		b.reducedMotion = *g.ReducedMotion
	}
	if g.HasLoadingIcon {
		b.loadingIcon = g.LoadingIcon
	}
	if g.HasHtmlType && g.HtmlType != "" {
		b.htmlType = g.HtmlType
	}
	b.node = rendering.NewRenderBox()
	b.node.SetRepaintBoundary(true)
	self := b
	b.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	// Intrinsic size up front so tree embedding (owner-driven Layout that
	// reaches the node directly) already sees content size, like Icon.
	b.Layout(rendering.Loose(rendering.Unbounded, rendering.Unbounded))
	return b
}

// Label returns the text label.
func (b *Button) Label() string {
	if b == nil {
		return ""
	}
	return b.label
}

// SetLabel changes the text (layout may grow).
func (b *Button) SetLabel(s string) {
	if b == nil || b.label == s {
		return
	}
	b.label = s
	b.relayout()
}

// Icon returns the icon name ("" = none).
func (b *Button) Icon() string {
	if b == nil {
		return ""
	}
	return b.iconName
}

// SetIcon sets the icon by name ("" clears).
func (b *Button) SetIcon(name string) {
	if b == nil || b.iconName == name {
		return
	}
	b.iconName = name
	b.relayout()
}

// IconPlacement returns start/end.
func (b *Button) IconPlacement() IconPlacement {
	if b == nil || b.iconPlacement == "" {
		return IconStart
	}
	return b.iconPlacement
}

// SetIconPlacement swaps icon/text order (mirrored by RTL, B-S9).
func (b *Button) SetIconPlacement(p IconPlacement) {
	if b == nil {
		return
	}
	if p != IconStart && p != IconEnd {
		p = IconStart
	}
	if b.iconPlacement == p {
		return
	}
	b.iconPlacement = p
	b.dirty()
}

// Type returns the sugar type.
func (b *Button) Type() ButtonType {
	if b == nil || b.typ == "" {
		return ButtonDefault
	}
	return b.typ
}

// SetType sets the sugar type (variant/color win when set, §6.3).
func (b *Button) SetType(t ButtonType) {
	if b == nil {
		return
	}
	switch t {
	case ButtonPrimary, ButtonDashed, ButtonText, ButtonLink, ButtonDefault:
	default:
		t = ButtonDefault
	}
	if b.typ == t {
		return
	}
	b.typ = t
	b.dirty()
}

// Size returns the size档位.
func (b *Button) Size() ButtonSize {
	if b == nil || b.size == "" {
		return ButtonMiddle
	}
	return b.size
}

// SetSize switches small/middle/large (height档位, BTN-07).
func (b *Button) SetSize(s ButtonSize) {
	if b == nil {
		return
	}
	switch s {
	case ButtonSmall, ButtonMiddle, ButtonLarge:
	default:
		s = ButtonMiddle
	}
	if b.size == s {
		return
	}
	b.size = s
	b.relayout()
}

// Shape returns default/circle/round.
func (b *Button) Shape() ButtonShape {
	if b == nil || b.shape == "" {
		return ButtonShapeDefault
	}
	return b.shape
}

// SetShape switches shape (circle forces square, B-S10).
func (b *Button) SetShape(s ButtonShape) {
	if b == nil {
		return
	}
	switch s {
	case ButtonShapeCircle, ButtonShapeRound, ButtonShapeDefault:
	default:
		s = ButtonShapeDefault
	}
	if b.shape == s {
		return
	}
	b.shape = s
	b.relayout()
}

// Variant returns the explicit variant (auto = derive from Type).
func (b *Button) Variant() ButtonVariant {
	if b == nil || b.variant == "" {
		return VariantAuto
	}
	return b.variant
}

// SetVariant pins a variant; non-auto wins over Type (BTN-19).
func (b *Button) SetVariant(v ButtonVariant) {
	if b == nil {
		return
	}
	switch v {
	case VariantSolid, VariantOutlined, VariantDashed, VariantFilled, VariantText, VariantLink, VariantAuto:
	default:
		v = VariantAuto
	}
	if b.variant == v {
		return
	}
	b.variant = v
	b.dirty()
}

// ColorName returns the color key.
func (b *Button) ColorName() ButtonColor {
	if b == nil || b.color == "" {
		return ColorDefault
	}
	return b.color
}

// SetColor sets default/primary/danger plus P1 success/warning/presets.
func (b *Button) SetColor(c ButtonColor) {
	if b == nil {
		return
	}
	switch c {
	case ColorPrimary, ColorDanger, ColorDefault, ColorSuccess, ColorWarning,
		ColorBlue, ColorPurple, ColorCyan, ColorGreen, ColorMagenta, ColorPink,
		ColorRed, ColorOrange, ColorYellow, ColorVolcano, ColorGeekBlue, ColorLime, ColorGold:
	default:
		c = ColorDefault
	}
	if b.color == c {
		return
	}
	b.color = c
	b.dirty()
}

// SetDanger is sugar for color=danger combined with current variant (§6.3).
func (b *Button) SetDanger(v bool) {
	if b == nil || b.danger == v {
		return
	}
	b.danger = v
	b.dirty()
}

// Danger reports the danger flag.
func (b *Button) Danger() bool { return b != nil && b.danger }

// SetGhost makes fill transparent for dark/complex backdrops.
func (b *Button) SetGhost(v bool) {
	if b == nil || b.ghost == v {
		return
	}
	b.ghost = v
	b.dirty()
}

// Ghost reports the ghost flag.
func (b *Button) Ghost() bool { return b != nil && b.ghost }

// SetBlock stretches width to the parent (height keeps size档位, B-S8).
func (b *Button) SetBlock(v bool) {
	if b == nil || b.block == v {
		return
	}
	b.block = v
	b.relayout()
}

// Block reports the block flag.
func (b *Button) Block() bool { return b != nil && b.block }

// SetDisabled swallows clicks and flattens chrome (B-S1).
func (b *Button) SetDisabled(v bool) {
	if b == nil || b.disabled == v {
		return
	}
	b.disabled = v
	if b.focusNode != nil {
		b.focusNode.Enabled = !v
	}
	b.dirty()
}

// Disabled reports the disabled flag.
func (b *Button) Disabled() bool { return b != nil && b.disabled }

// SetLoading shows the spinner and swallows repeat clicks (B-S2).
func (b *Button) SetLoading(v bool) {
	if b == nil {
		return
	}
	if b.loading == v && v {
		return
	}
	if b.loading == v {
		return
	}
	b.loading = v
	if v {
		b.loadingStart = time.Now()
	}
	b.relayout()
}

// SetLoadingConfig enables loading with P1 delay + custom icon in one call.
func (b *Button) SetLoadingConfig(cfg LoadingConfig) {
	if b == nil {
		return
	}
	b.loadingDelay = cfg.Delay
	if cfg.Delay < 0 {
		b.loadingDelay = 0
	}
	b.loadingIcon = cfg.Icon
	if !b.loading {
		b.loading = true
		b.loadingStart = time.Now()
	}
	b.relayout()
}

// SetLoadingDelay sets the P1 spinner delay (BTN-25). Negative clears.
func (b *Button) SetLoadingDelay(d time.Duration) {
	if b == nil {
		return
	}
	if d < 0 {
		d = 0
	}
	if b.loadingDelay == d {
		return
	}
	b.loadingDelay = d
	b.dirty()
}

// LoadingDelay reports the spinner delay.
func (b *Button) LoadingDelay() time.Duration {
	if b == nil {
		return 0
	}
	return b.loadingDelay
}

// SetLoadingIcon sets the P1 custom loading icon ("" clears to spinner).
func (b *Button) SetLoadingIcon(name string) {
	if b == nil || b.loadingIcon == name {
		return
	}
	b.loadingIcon = name
	b.dirty()
}

// LoadingIcon reports the custom loading icon name ("" = default spinner).
func (b *Button) LoadingIcon() string {
	if b == nil {
		return ""
	}
	return b.loadingIcon
}

// Loading reports the loading flag.
func (b *Button) Loading() bool { return b != nil && b.loading }

// HasSpinner reports whether the loading indicator paints now.
// With a P1 delay, false until the delay elapses (delay到期前不转).
func (b *Button) HasSpinner() bool {
	if b == nil || !b.loading {
		return false
	}
	if b.loadingDelay > 0 && time.Since(b.loadingStart) < b.loadingDelay {
		return false
	}
	return true
}

// SetRTL flips logical start/end for icon placement (§6.6 RTL).
func (b *Button) SetRTL(v bool) {
	if b == nil || b.rtl == v {
		return
	}
	b.rtl = v
	b.dirty()
}

// RTL reports right-to-left mirroring.
func (b *Button) RTL() bool { return b != nil && b.rtl }

// SetAriaLabel pins the accessible name (required when icon-only, §6.6).
func (b *Button) SetAriaLabel(s string) {
	if b == nil || b.ariaLabel == s {
		return
	}
	b.ariaLabel = s
}

// AriaName is ariaLabel when set, else the visible label.
func (b *Button) AriaName() string {
	if b == nil {
		return ""
	}
	if b.ariaLabel != "" {
		return b.ariaLabel
	}
	return b.label
}

// HasAccessibleName is false for icon-only buttons without AriaLabel (BTN-23).
func (b *Button) HasAccessibleName() bool { return b != nil && b.AriaName() != "" }

// Role is the screen-reader role (§6.6).
func (b *Button) Role() string { return "button" }

// SetProvider selects the theme source (nil = process default).
func (b *Button) SetProvider(p *theme.Provider) {
	if b == nil {
		return
	}
	b.provider = p
	b.dirty()
}

// SetTheme pins exact tokens (nil clears to provider).
func (b *Button) SetTheme(t *theme.Tokens) {
	if b == nil {
		return
	}
	b.override = t
	b.dirty()
}

// SetStyle installs the business paint override.
func (b *Button) SetStyle(s Style) {
	if b == nil {
		return
	}
	b.style = s
	b.dirty()
}

// SetTextFace sets the paint-only font face (nil clears; layout keeps the
// rune estimate so headless tests stay stable).
func (b *Button) SetTextFace(f text.Face) {
	if b == nil {
		return
	}
	b.textFace = f
	b.dirty()
}

// SetIconPosition is the deprecated alias of SetIconPlacement (P1 compat).
func (b *Button) SetIconPosition(p IconPosition) { b.SetIconPlacement(p) }

// IconPosition reports the placement under the deprecated name.
func (b *Button) IconPosition() IconPlacement { return b.IconPlacement() }

// SetAutoInsertSpace toggles the two-Han spacing (P1, default true, BTN-26).
func (b *Button) SetAutoInsertSpace(v bool) {
	if b == nil || b.autoInsertSpace == v {
		return
	}
	b.autoInsertSpace = v
	b.relayout()
}

// AutoInsertSpace reports the two-Han spacing switch.
func (b *Button) AutoInsertSpace() bool { return b == nil || b.autoInsertSpace }

// DisplayLabel is the painted/measured text (autoInsertSpace applied).
func (b *Button) DisplayLabel() string {
	if b == nil {
		return ""
	}
	if !b.autoInsertSpace {
		return b.label
	}
	r := []rune(b.label)
	if len(r) != 2 {
		return b.label
	}
	if !isHan(r[0]) || !isHan(r[1]) {
		return b.label
	}
	return string(r[0]) + " " + string(r[1])
}

func isHan(r rune) bool {
	if r >= ' ' && r < 128 {
		return false
	}
	return unicode.Is(unicode.Han, r)
}

// SetHref sets the P1 link address (desktop: OnNavigate, not <a> nav).
func (b *Button) SetHref(s string) {
	if b == nil || b.href == s {
		return
	}
	b.href = s
	b.dirty()
}

// Href reports the link address ("" = plain button).
func (b *Button) Href() string {
	if b == nil {
		return ""
	}
	return b.href
}

// IsLink reports href presence (target only effective then, P1).
func (b *Button) IsLink() bool { return b != nil && b.href != "" }

// SetTarget sets the P1 href target (effective only when Href != "").
func (b *Button) SetTarget(s string) {
	if b == nil || b.target == s {
		return
	}
	b.target = strings.TrimSpace(s)
	b.dirty()
}

// Target reports the href target.
func (b *Button) Target() string {
	if b == nil {
		return ""
	}
	return b.target
}

// SetHtmlType sets submit/reset/button (default button, P1 Form hook).
func (b *Button) SetHtmlType(t HtmlType) {
	if b == nil {
		return
	}
	switch t {
	case HtmlSubmit, HtmlReset, HtmlButton:
	default:
		t = HtmlButton
	}
	if b.htmlType == t {
		return
	}
	b.htmlType = t
	b.dirty()
}

// HtmlTypeName reports the native type (default button).
func (b *Button) HtmlTypeName() HtmlType {
	if b == nil || b.htmlType == "" {
		return HtmlButton
	}
	return b.htmlType
}

// SetWaveDisabled closes the click ripple (P1 default open, BTN-27).
func (b *Button) SetWaveDisabled(v bool) {
	if b == nil || b.waveDisabled == v {
		return
	}
	b.waveDisabled = v
	b.dirty()
}

// WaveDisabled reports the ripple close switch.
func (b *Button) WaveDisabled() bool { return b != nil && b.waveDisabled }

// SetReducedMotion disables wave/motion feedback (P1 reduced-motion).
func (b *Button) SetReducedMotion(v bool) {
	if b == nil || b.reducedMotion == v {
		return
	}
	b.reducedMotion = v
	b.dirty()
}

// ReducedMotion reports the motion-off switch.
func (b *Button) ReducedMotion() bool { return b != nil && b.reducedMotion }

// WaveActive reports whether a press paints a ripple.
func (b *Button) WaveActive() bool {
	return b != nil && !b.waveDisabled && !b.reducedMotion
}

// SetGradient installs a P1 two-stop linear background (linear-gradient demo).
func (b *Button) SetGradient(from, to render.RGBA) {
	if b == nil {
		return
	}
	b.gradient = Gradient{From: from, To: to, Enabled: true}
	b.dirty()
}

// ClearGradient removes the gradient background.
func (b *Button) ClearGradient() {
	if b == nil || !b.gradient.Enabled {
		return
	}
	b.gradient.Enabled = false
	b.dirty()
}

// HasGradient reports the gradient background.
func (b *Button) HasGradient() bool { return b != nil && b.gradient.Enabled }

// GradientColors returns the gradient stops.
func (b *Button) GradientColors() (from, to render.RGBA, ok bool) {
	if b == nil || !b.gradient.Enabled {
		return render.RGBA{}, render.RGBA{}, false
	}
	return b.gradient.From, b.gradient.To, true
}

// SetClassName pins one semantic class hook (P1 classNames).
func (b *Button) SetClassName(k SemanticKey, v string) {
	if b == nil {
		return
	}
	if b.classNames == nil {
		b.classNames = map[SemanticKey]string{}
	}
	if v == "" {
		delete(b.classNames, k)
		return
	}
	b.classNames[k] = v
}

// ClassName reads one semantic class hook.
func (b *Button) ClassName(k SemanticKey) string {
	if b == nil || b.classNames == nil {
		return ""
	}
	return b.classNames[k]
}

// SetClassNames replaces all semantic class hooks (nil clears).
func (b *Button) SetClassNames(m map[SemanticKey]string) {
	if b == nil {
		return
	}
	if len(m) == 0 {
		b.classNames = nil
		return
	}
	cp := make(map[SemanticKey]string, len(m))
	for k, v := range m {
		if v != "" {
			cp[k] = v
		}
	}
	b.classNames = cp
}

// SetSemanticStyle pins one semantic inline-style hook (P1 styles).
func (b *Button) SetSemanticStyle(k SemanticKey, s Style) {
	if b == nil {
		return
	}
	if b.semanticStyles == nil {
		b.semanticStyles = map[SemanticKey]Style{}
	}
	b.semanticStyles[k] = s
	b.dirty()
}

// SemanticStyle reads one semantic style hook.
func (b *Button) SemanticStyle(k SemanticKey) (Style, bool) {
	if b == nil || b.semanticStyles == nil {
		return Style{}, false
	}
	s, ok := b.semanticStyles[k]
	return s, ok
}

// ClearSemanticStyles removes all semantic style hooks.
func (b *Button) ClearSemanticStyles() {
	if b == nil {
		return
	}
	b.semanticStyles = nil
	b.dirty()
}

func (b *Button) themeTokens() theme.Tokens {
	if b != nil && b.override != nil {
		return *b.override
	}
	if b != nil && b.provider != nil {
		return b.provider.Current()
	}
	return theme.Default.Current()
}

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// EffectiveVariant resolves auto through Type (§6.3 mapping table).
func (b *Button) EffectiveVariant() ButtonVariant {
	v := VariantAuto
	if b != nil {
		v = b.Variant()
	}
	if v != VariantAuto {
		return v
	}
	switch b.Type() {
	case ButtonPrimary:
		return VariantSolid
	case ButtonDashed:
		return VariantDashed
	case ButtonText:
		return VariantText
	case ButtonLink:
		return VariantLink
	default:
		return VariantOutlined
	}
}

// dangerActive folds the danger sugar into the color key.
// Explicit color wins over the danger flag (§6.3): only default color
// lets the flag through; ColorDanger always counts.
func (b *Button) dangerActive() bool {
	if b == nil {
		return false
	}
	if b.ColorName() == ColorDanger {
		return true
	}
	if b.color != "" && b.color != ColorDefault {
		return false
	}
	return b.danger
}

// accent returns the semantic accent (primary/error/success/warning/
// presets/link) or transparent when neutral default.
func (b *Button) accent(tok theme.Tokens) render.RGBA {
	if b.dangerActive() {
		return themeToRGBA(tok.ColorError)
	}
	if b == nil {
		return render.RGBA{}
	}
	switch b.ColorName() {
	case ColorPrimary:
		return themeToRGBA(tok.ColorPrimary)
	case ColorSuccess:
		return themeToRGBA(tok.ColorSuccess)
	case ColorWarning:
		return themeToRGBA(tok.ColorWarning)
	case ColorDanger:
		return themeToRGBA(tok.ColorError)
	}
	if c, ok := presetHex(b.ColorName()); ok {
		return c
	}
	if b.EffectiveVariant() == VariantLink {
		return themeToRGBA(tok.ColorLink)
	}
	return render.RGBA{}
}

// Height is the size档位 height (§6.2.1, token first).
func (b *Button) Height() float64 {
	tok := b.themeTokens()
	switch b.Size() {
	case ButtonSmall:
		if tok.ControlHeightSM > 0 {
			return tok.ControlHeightSM
		}
		return 24
	case ButtonLarge:
		if tok.ControlHeightLG > 0 {
			return tok.ControlHeightLG
		}
		return 40
	default:
		if tok.ControlHeight > 0 {
			return tok.ControlHeight
		}
		return 32
	}
}

// FontSize is the size档位 font size (§6.2.1).
func (b *Button) FontSize() float64 {
	tok := b.themeTokens()
	switch b.Size() {
	case ButtonSmall:
		if tok.FontSizeSM > 0 {
			return tok.FontSizeSM
		}
		return 12
	case ButtonLarge:
		if tok.FontSizeLG > 0 {
			return tok.FontSizeLG
		}
		return 16
	default:
		if tok.FontSize > 0 {
			return tok.FontSize
		}
		return 14
	}
}

// PaddingInline is the horizontal chrome padding (§6.2.1).
func (b *Button) PaddingInline() float64 {
	if b.Size() == ButtonSmall {
		return paddingInlineSM
	}
	return paddingInline
}

// IconEdge follows §6.2.1 (middle ≈ 字号+2).
func (b *Button) IconEdge() float64 {
	if b.Size() == ButtonSmall {
		return b.FontSize()
	}
	return b.FontSize() + 2
}

// IconGap is the icon/text spacing (§6.2.1: small 4, else marginXS 8).
func (b *Button) IconGap() float64 {
	if b.Size() == ButtonSmall {
		return iconGapSM
	}
	return iconGap
}

// Radius is the chrome corner radius (round = capsule h/2).
func (b *Button) Radius() float64 {
	h := b.Height()
	if b.Shape() == ButtonShapeRound {
		return h / 2
	}
	tok := b.themeTokens()
	switch b.Size() {
	case ButtonSmall:
		if tok.RadiusSM > 0 {
			return tok.RadiusSM
		}
		return 4
	case ButtonLarge:
		if tok.RadiusLG > 0 {
			return tok.RadiusLG
		}
		return 8
	default:
		if tok.Radius > 0 {
			return tok.Radius
		}
		return 6
	}
}

// textWidth estimates the DisplayLabel advance (no font headless keeps the
// rune estimator; face only affects paint ink, layout stays stable).
func (b *Button) textWidth() float64 {
	if b == nil {
		return 0
	}
	s := b.DisplayLabel()
	if s == "" {
		return 0
	}
	ascii, wide := 0, 0
	for _, r := range s {
		if r < 128 {
			ascii++
		} else {
			wide++
		}
	}
	return (float64(ascii)*0.6 + float64(wide)*1.0) * b.FontSize()
}

// leadingWidth is the icon-or-spinner slot (loading replaces icon, B-S2).
// With a P1 delay the spinner slot stays empty until the delay elapses;
// a plain icon still shows while the delayed spinner waits.
func (b *Button) leadingWidth() float64 {
	if b == nil {
		return 0
	}
	if b.HasSpinner() {
		return b.IconEdge()
	}
	if b.iconName != "" {
		return b.IconEdge()
	}
	return 0
}

// ContentWidth is text + leading slot + gap (excludes padding).
func (b *Button) ContentWidth() float64 {
	tw, lw := b.textWidth(), b.leadingWidth()
	if tw > 0 && lw > 0 {
		return tw + b.IconGap() + lw
	}
	return tw + lw
}

// IconFirst reports whether the icon/leading slot paints before text
// (placement start/end mirrored by RTL, B-S9).
func (b *Button) IconFirst() bool {
	return (b.IconPlacement() == IconStart) != b.RTL()
}

func mix(a, b render.RGBA, t float64) render.RGBA {
	return render.RGBA{R: a.R + (b.R-a.R)*t, G: a.G + (b.G-a.G)*t, B: a.B + (b.B-a.B)*t, A: a.A + (b.A-a.A)*t}
}

// fam resolves the exact official color family for the current color +
// variant (antd 6.5.1 @ant-design/colors generate() + token.ts derivations).
// Link variant always uses link blue (or danger red) regardless of color.
// Default color returns neutralDefault=true (caller branches per variant).
func (b *Button) fam() (base, hover, active, light, lightHover, lightActive render.RGBA, neutralDefault bool) {
	tok := b.themeTokens()
	pick := func(c theme.Color, fallback string) render.RGBA {
		var zero theme.Color
		if c != zero {
			return themeToRGBA(c)
		}
		return parseHexRender(fallback)
	}
	v := b.EffectiveVariant()
	danger := b.dangerActive()
	if v == VariantLink {
		if danger {
			return pick(tok.ColorError, "#ff4d4f"), pick(tok.ColorErrorHover, "#ff7875"), pick(tok.ColorErrorActive, "#d9363e"), render.RGBA{}, render.RGBA{}, render.RGBA{}, false
		}
		return pick(tok.ColorLink, "#1677ff"), pick(tok.ColorLinkHover, "#69b1ff"), pick(tok.ColorLinkActive, "#0958d9"), render.RGBA{}, render.RGBA{}, render.RGBA{}, false
	}
	if danger {
		return pick(tok.ColorError, "#ff4d4f"), pick(tok.ColorErrorHover, "#ff7875"), pick(tok.ColorErrorActive, "#d9363e"),
			pick(tok.ColorErrorBg, "#fff2f0"), pick(tok.ColorErrorBgFilledHover, "#ffdfdc"), pick(tok.ColorErrorBgActive, "#ffccc7"), false
	}
	// Type sugar carries color when Color stays default (primary type =>
	// primary blue even under explicit variant, BTN-19).
	if b.ColorName() == ColorDefault && b.Type() == ButtonPrimary {
		return pick(tok.ColorPrimary, "#1677ff"), pick(tok.ColorPrimaryHover, "#4096ff"), pick(tok.ColorPrimaryActive, "#0958d9"),
			pick(tok.ColorPrimaryBg, "#e6f4ff"), pick(tok.ColorPrimaryBgHover, "#bae0ff"), pick(tok.ColorPrimaryBorder, "#91caff"), false
	}
	switch b.ColorName() {
	case ColorPrimary:
		return pick(tok.ColorPrimary, "#1677ff"), pick(tok.ColorPrimaryHover, "#4096ff"), pick(tok.ColorPrimaryActive, "#0958d9"),
			pick(tok.ColorPrimaryBg, "#e6f4ff"), pick(tok.ColorPrimaryBgHover, "#bae0ff"), pick(tok.ColorPrimaryBorder, "#91caff"), false
	case ColorDanger:
		return pick(tok.ColorError, "#ff4d4f"), pick(tok.ColorErrorHover, "#ff7875"), pick(tok.ColorErrorActive, "#d9363e"),
			pick(tok.ColorErrorBg, "#fff2f0"), pick(tok.ColorErrorBgFilledHover, "#ffdfdc"), pick(tok.ColorErrorBgActive, "#ffccc7"), false
	case ColorSuccess:
		return pick(tok.ColorSuccess, "#52c41a"), pick(tok.ColorSuccessHover, "#95de64"), pick(tok.ColorSuccessActive, "#389e0d"),
			pick(tok.ColorSuccessBg, "#f6ffed"), pick(tok.ColorSuccessBgHover, "#d9f7be"), pick(tok.ColorSuccessBorder, "#b7eb8f"), false
	case ColorWarning:
		return pick(tok.ColorWarning, "#faad14"), pick(tok.ColorWarningHover, "#ffd666"), pick(tok.ColorWarningActive, "#d48806"),
			pick(tok.ColorWarningBg, "#fffbe6"), pick(tok.ColorWarningBgHover, "#fff1b8"), pick(tok.ColorWarningBorder, "#ffe58f"), false
	case ColorDefault:
		return render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, true
	default:
		if ps, ok := presetStyleFor(b.ColorName()); ok {
			return ps.base, ps.hover, ps.active, ps.light, ps.lightHover, ps.lightActive, false
		}
		return render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, render.RGBA{}, true
	}
}

func parseHexRender(s string) render.RGBA {
	if len(s) == 0 || s[0] != '#' {
		return render.RGBA{}
	}
	h := s[1:]
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return render.RGBA{}
	}
	return render.RGBA{R: float64(hexByte(s, 1)) / 255.0, G: float64(hexByte(s, 3)) / 255.0, B: float64(hexByte(s, 5)) / 255.0, A: 1}
}

// hoverActive keeps the old mix fallback for provider overrides without
// palette derivatives; chrome() now resolves exact families directly.
func (b *Button) hoverActive(base render.RGBA, hover bool) render.RGBA {
	tok := b.themeTokens()
	var zero theme.Color
	pick := func(c theme.Color) (render.RGBA, bool) {
		if c == zero {
			return render.RGBA{}, false
		}
		return themeToRGBA(c), true
	}
	acc := b.accent(tok)
	baseIsPrimary := base == themeToRGBA(tok.ColorPrimary)
	baseIsError := base == themeToRGBA(tok.ColorError)
	baseIsSuccess := base == themeToRGBA(tok.ColorSuccess)
	baseIsWarning := base == themeToRGBA(tok.ColorWarning)
	isPrimary := baseIsPrimary
	isError := baseIsError
	isSuccess := baseIsSuccess
	isWarning := baseIsWarning
	if !isPrimary && !isError && !isSuccess && !isWarning {
		isPrimary = acc == themeToRGBA(tok.ColorPrimary)
		isError = acc == themeToRGBA(tok.ColorError)
		isSuccess = acc == themeToRGBA(tok.ColorSuccess)
		isWarning = acc == themeToRGBA(tok.ColorWarning)
	}
	if hover {
		switch {
		case isPrimary:
			if c, ok := pick(tok.ColorPrimaryHover); ok {
				return c
			}
		case isError:
			if c, ok := pick(tok.ColorErrorHover); ok {
				return c
			}
		case isSuccess:
			if c, ok := pick(tok.ColorSuccessHover); ok {
				return c
			}
		case isWarning:
			if c, ok := pick(tok.ColorWarningHover); ok {
				return c
			}
		}
		if c, ok := presetHex(b.ColorName()); ok && b.ColorName() != ColorDefault {
			_ = c
			// Preset hover/active stay on the mix path (palette[5] per
			// preset is not in the global seed; button keeps its mix).
		}
		return shade(base, true)
	}
	switch {
	case isPrimary:
		if c, ok := pick(tok.ColorPrimaryActive); ok {
			return c
		}
	case isError:
		if c, ok := pick(tok.ColorErrorActive); ok {
			return c
		}
	case isSuccess:
		if c, ok := pick(tok.ColorSuccessActive); ok {
			return c
		}
	case isWarning:
		if c, ok := pick(tok.ColorWarningActive); ok {
			return c
		}
	}
	return shade(base, false)
}

func shade(c render.RGBA, hover bool) render.RGBA {
	white := render.RGBA{R: 1, G: 1, B: 1, A: c.A}
	black := render.RGBA{R: 0, G: 0, B: 0, A: c.A}
	if hover {
		return mix(c, white, 0.15)
	}
	return mix(c, black, 0.12)
}

// chrome resolves fill/border/text for the current state (§6.5 table).
// Exact official mapping (variant.ts + token.ts, 6.5.1): solid uses
// base/hover/active fills; outlined keeps white fill; filled uses
// light fills; text uses light fills on hover/active; link stays
// transparent. Disabled uses container-disabled gray; loading dims 0.65.
func (b *Button) chrome() (fill, border, text render.RGBA, dashed, hasBorder bool) {
	tok := b.themeTokens()
	container := themeToRGBA(tok.ColorBgContainer)
	if container.A == 0 {
		container = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	ink := themeToRGBA(tok.ColorText)
	white := themeToRGBA(tok.ColorWhite)
	if white.A == 0 {
		white = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	inverse := themeToRGBA(tok.OnPrimary)
	if inverse.A == 0 {
		inverse = white
	}
	v := b.EffectiveVariant()
	base, hoverC, activeC, light, lightHover, lightActive, neutral := b.fam()
	disabledFill := themeToRGBA(tok.ColorBgContainerDisabled)
	if disabledFill.A == 0 {
		disabledFill = themeToRGBA(tok.ColorFillTertiary)
	}
	disabledBorder := themeToRGBA(tok.ColorBorder)
	disabledText := themeToRGBA(tok.ColorTextDisabled)

	if b.Disabled() {
		if v == VariantText || v == VariantLink {
			return render.RGBA{}, render.RGBA{}, disabledText, false, false
		}
		dashed = v == VariantDashed
		hasBorder = v == VariantOutlined || v == VariantDashed
		return disabledFill, disabledBorder, disabledText, dashed, hasBorder
	}

	isHover := !b.Disabled() && b.hovered && !b.pressed
	isPress := !b.Disabled() && !b.loading && b.pressed && b.inBound

	if b.ghost {
		// Ghost: transparent fill always; default color uses white ink,
		// other colors keep family ink on transparent.
		fill = render.RGBA{}
		if neutral {
			switch v {
			case VariantSolid, VariantOutlined, VariantDashed:
				border, text = white, white
				hasBorder = true
				dashed = v == VariantDashed
			default:
				border, text = render.RGBA{}, white
			}
		} else {
			switch v {
			case VariantSolid, VariantOutlined, VariantDashed:
				cur := base
				if isHover {
					cur = hoverC
				} else if isPress {
					cur = activeC
				}
				border, text = cur, cur
				hasBorder = true
				dashed = v == VariantDashed
			case VariantFilled, VariantText, VariantLink:
				cur := base
				if v == VariantLink {
					// Link ghost keeps link blue hover/active.
					if isHover {
						cur = hoverC
					} else if isPress {
						cur = activeC
					}
				} else if isHover {
					cur = hoverC
				} else if isPress {
					cur = activeC
				}
				border, text = render.RGBA{}, cur
			}
		}
		goto applyStyle
	}

	if neutral {
		// Default color per-variant bases.
		switch v {
		case VariantSolid:
			solidBase := themeToRGBA(tok.ColorBgSolid)
			if solidBase.A == 0 {
				solidBase = render.RGBA{R: 0, G: 0, B: 0, A: 1}
			}
			fill, border, text = solidBase, solidBase, inverse
			hasBorder = false
			if isHover {
				h := themeToRGBA(tok.ColorBgSolidHover)
				if h.A == 0 {
					h = render.RGBA{R: 0, G: 0, B: 0, A: 0.75}
				}
				fill, border = h, h
			} else if isPress {
				a := themeToRGBA(tok.ColorBgSolidActive)
				if a.A == 0 {
					a = render.RGBA{R: 0, G: 0, B: 0, A: 0.95}
				}
				fill, border = a, a
			}
		case VariantOutlined, VariantDashed:
			fill = container
			dashed = v == VariantDashed
			hasBorder = true
			border, text = themeToRGBA(tok.ColorBorder), ink
			if isHover {
				h := themeToRGBA(tok.ColorPrimaryHover)
				if h.A == 0 {
					h = themeToRGBA(tok.ColorPrimary)
				}
				border, text = h, h
			} else if isPress {
				a := themeToRGBA(tok.ColorPrimaryActive)
				if a.A == 0 {
					a = themeToRGBA(tok.ColorPrimary)
				}
				border, text = a, a
			}
		case VariantFilled:
			hasBorder = false
			fill = themeToRGBA(tok.ColorFillTertiary)
			if fill.A == 0 {
				fill = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
			}
			text = ink
			if isHover {
				fill = themeToRGBA(tok.ColorFillSecondary)
			} else if isPress {
				fill = themeToRGBA(tok.ColorFill)
			}
		case VariantText:
			hasBorder = false
			fill = render.RGBA{}
			text = ink
			if isHover {
				fill = themeToRGBA(tok.ColorFillTertiary)
			} else if isPress {
				fill = themeToRGBA(tok.ColorFill)
			}
		default:
			fill = render.RGBA{}
			hasBorder = false
			text = ink
		}
		goto applyLoading
	}

	// Family colors (primary/danger/success/warning/presets/link).
	switch v {
	case VariantSolid:
		cur := base
		if isHover {
			cur = hoverC
		} else if isPress {
			cur = activeC
		}
		fill, border, text = cur, cur, inverse
		hasBorder = false
	case VariantOutlined, VariantDashed:
		fill = container
		dashed = v == VariantDashed
		hasBorder = true
		cur := base
		if isHover {
			cur = hoverC
		} else if isPress {
			cur = activeC
		}
		border, text = cur, cur
	case VariantFilled:
		hasBorder = false
		cur := base
		f := light
		if isHover {
			cur, f = hoverC, lightHover
		} else if isPress {
			cur, f = activeC, lightActive
		}
		fill, text = f, cur
	case VariantText:
		hasBorder = false
		cur := base
		f := render.RGBA{}
		if isHover {
			cur, f = hoverC, light
		} else if isPress {
			cur, f = activeC, lightActive
		}
		fill, text = f, cur
	case VariantLink:
		hasBorder = false
		cur := base
		if isHover {
			cur = hoverC
		} else if isPress {
			cur = activeC
		}
		fill, text = render.RGBA{}, cur
	}

applyLoading:
	if b.loading {
		const dim = 0.65
		fill.A *= dim
		border.A *= dim
		text.A *= dim
	}

applyStyle:

	if b.style.UseBg {
		fill = b.style.Bg
	}
	if b.style.UseBorder {
		border, hasBorder = b.style.Border, true
	}
	if b.style.UseText {
		text = b.style.Text
	}
	// P1 semantic hooks: root overrides chrome, label wins for text.
	if b.semanticStyles != nil {
		if s, ok := b.semanticStyles[SemanticRoot]; ok {
			if s.UseBg {
				fill = s.Bg
			}
			if s.UseBorder {
				border, hasBorder = s.Border, true
			}
			if s.UseText {
				text = s.Text
			}
		}
		if s, ok := b.semanticStyles[SemanticLabel]; ok && s.UseText {
			text = s.Text
		}
	}
	return fill, border, text, dashed, hasBorder
}

// IconColor exposes the leading icon/spinner ink (semantic icon hook wins).
func (b *Button) IconColor() render.RGBA {
	_, _, t, _, _ := b.chrome()
	if b != nil && b.semanticStyles != nil {
		if s, ok := b.semanticStyles[SemanticIcon]; ok && s.UseText {
			return s.Text
		}
	}
	return t
}

// Fill/BorderColor/TextColor expose chrome for tests and gallery probes.
func (b *Button) Fill() render.RGBA {
	f, _, _, _, _ := b.chrome()
	return f
}

// BorderColor exposes the border ink.
func (b *Button) BorderColor() render.RGBA {
	_, br, _, _, _ := b.chrome()
	return br
}

// TextColor exposes the label ink.
func (b *Button) TextColor() render.RGBA {
	_, _, t, _, _ := b.chrome()
	return t
}

// DashedBorder reports the dashed variant border (BTN-11).
func (b *Button) DashedBorder() bool {
	_, _, _, d, has := b.chrome()
	return d && has
}

// HasBorder reports a visible border box.
func (b *Button) HasBorder() bool {
	_, _, _, _, has := b.chrome()
	return has
}

// Focused reports keyboard focus (ring paints when true).
func (b *Button) Focused() bool { return b != nil && b.focused }

// Hovered reports pointer hover.
func (b *Button) Hovered() bool { return b != nil && b.hovered }

// Focusable is false while disabled (manager skips the node).
func (b *Button) Focusable() bool { return b != nil && !b.disabled }

// FocusNode lazily builds the manager node: activate clicks, focus change
// repaints the ring without touching layout (C5 dirty path).
func (b *Button) FocusNode() *focus.FocusNode {
	if b == nil {
		return nil
	}
	if b.focusNode == nil {
		n := focus.NewFocusNode("button:" + b.AriaName())
		n.Enabled = !b.disabled
		self := b
		n.OnActivate = func() { self.click() }
		n.OnFocusChange = func(f bool) {
			self.focused = f
			self.dirty()
		}
		b.focusNode = n
	}
	return b.focusNode
}

// KeyPress handles Space/Enter directly (same path the manager routes).
func (b *Button) KeyPress(e focus.KeyEvent) bool {
	if b == nil || !b.focused || !e.Pressed || !e.IsActivate() {
		return false
	}
	b.click()
	return true
}

func (b *Button) click() {
	if b == nil || b.disabled || b.loading {
		return
	}
	if b.OnClick != nil {
		b.OnClick()
	}
	// P1 href desktop mapping: still fire click, then offer navigation.
	if b.IsLink() && b.OnNavigate != nil {
		b.OnNavigate(b.href, b.target)
	}
}

// inside reports hit including the 44px floor expansion (a11y, §6.6).
func (b *Button) inside(x, y float64) bool {
	w, h := b.lastSize.Width, b.lastSize.Height
	hw, hh := HitExpand(w, h)
	return x >= -hw && y >= -hh && x < w+hw && y < h+hh
}

// HitExpand returns the per-side hit outset so max(size,44) is tappable.
func HitExpand(w, h float64) (dx, dy float64) {
	if w < minTouchTarget {
		dx = (minTouchTarget - w) / 2
	}
	if h < minTouchTarget {
		dy = (minTouchTarget - h) / 2
	}
	return dx, dy
}

// HitSize is the effective tappable extent (≥44 each axis, a11y probe).
func (b *Button) HitSize() rendering.Size {
	if b == nil {
		return rendering.Size{Width: minTouchTarget, Height: minTouchTarget}
	}
	w, h := b.lastSize.Width, b.lastSize.Height
	dx, dy := HitExpand(w, h)
	return rendering.Size{Width: w + 2*dx, Height: h + 2*dy}
}

// PointerDown starts a press; disabled/loading/outside swallow (B-S1/B-S2).
func (b *Button) PointerDown(x, y float64) bool {
	if b == nil || b.disabled || b.loading || !b.inside(x, y) {
		return false
	}
	b.pressed, b.inBound = true, true
	if b.focusNode != nil {
		b.focusNode.RequestFocus()
	}
	b.dirty()
	return true
}

// PointerMove tracks hover and press containment (B-S4 needs the move-out).
func (b *Button) PointerMove(x, y float64) {
	if b == nil || b.disabled {
		return
	}
	in := b.inside(x, y)
	if b.hovered != in {
		b.hovered = in
		b.dirty()
	}
	if b.pressed {
		inside := in
		if b.inBound != inside {
			b.inBound = inside
			b.dirty()
		}
	}
}

// PointerUp releases; a contained release clicks exactly once (B-S3).
func (b *Button) PointerUp(x, y float64) bool {
	if b == nil || !b.pressed {
		return false
	}
	b.pressed = false
	in := !b.disabled && !b.loading && b.inside(x, y) && b.inBound
	b.inBound = false
	b.dirty()
	if in {
		b.click()
		return true
	}
	return false
}

// Node returns the tree node (layout/paint/hit through it).
func (b *Button) Node() rendering.RenderObject {
	if b == nil {
		return nil
	}
	b.sync()
	return b.node
}

// LaidOut returns the last laid-out size.
func (b *Button) LaidOut() rendering.Size {
	if b == nil {
		return rendering.Size{}
	}
	return b.lastSize
}

// Layout sizes the button: height from the size档位, width from content +
// padding (circle forces square, block fills MaxWidth). Exact constraints
// win via Tighten, matching RenderBox semantics.
func (b *Button) Layout(c rendering.Constraints) rendering.Size {
	if b == nil {
		return rendering.Size{}
	}
	h := b.Height()
	w := b.ContentWidth() + 2*b.PaddingInline()
	if b.shape == ButtonShapeCircle {
		w = h
	}
	if b.block && c.MaxWidth < rendering.Unbounded/2 {
		w = c.MaxWidth
		if w < h && b.shape == ButtonShapeCircle {
			w = h
		}
	}
	out := c.Tighten(rendering.Size{Width: w, Height: h})
	b.lastSize = out
	b.sync()
	return b.node.Layout(c)
}

func (b *Button) sync() {
	if b == nil || b.node == nil {
		return
	}
	b.node.FixedWidth, b.node.FixedHeight = b.lastSize.Width, b.lastSize.Height
}

func (b *Button) dirty() {
	if b == nil || b.node == nil {
		return
	}
	b.sync()
	b.node.MarkNeedsPaint()
}

func (b *Button) relayout() {
	if b == nil || b.node == nil {
		return
	}
	b.node.MarkNeedsLayout()
	b.node.MarkNeedsPaint()
}

// slots splits the content row into leading/text centers for paint and tests.
func (b *Button) slots(w, h float64) (leadCX, textCX, textW float64, hasLead, hasText bool) {
	pad := b.PaddingInline()
	inner := w - 2*pad
	if b.shape == ButtonShapeCircle {
		pad, inner = 0, w
	}
	tw, lw := b.textWidth(), b.leadingWidth()
	hasLead, hasText = lw > 0, tw > 0
	gap := 0.0
	if hasLead && hasText {
		gap = b.IconGap()
	}
	total := tw + lw + gap
	x := pad + (inner-total)/2
	if b.IconFirst() {
		if hasLead {
			leadCX = x + lw/2
			x += lw + gap
		}
		if hasText {
			textCX, textW = x+tw/2, tw
		}
		return leadCX, textCX, textW, hasLead, hasText
	}
	if hasText {
		textCX, textW = x+tw/2, tw
		x += tw + gap
	}
	if hasLead {
		leadCX = x + lw/2
	}
	_ = h
	return leadCX, textCX, textW, hasLead, hasText
}

// LeadCenterX/TextCenterX expose slot geometry for order assertions (BTN-18).
func (b *Button) LeadCenterX() float64 {
	if b == nil {
		return 0
	}
	x, _, _, has, _ := b.slots(b.lastSize.Width, b.lastSize.Height)
	if !has {
		return math.NaN()
	}
	return x
}

// TextCenterX exposes the label center for order assertions.
func (b *Button) TextCenterX() float64 {
	if b == nil {
		return 0
	}
	_, x, _, _, has := b.slots(b.lastSize.Width, b.lastSize.Height)
	if !has {
		return math.NaN()
	}
	return x
}

func (b *Button) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	b.lastSize = size
	w, h := size.Width, size.Height
	fill, border, text, dashed, hasBorder := b.chrome()
	radius := b.Radius()
	if b.shape == ButtonShapeCircle {
		radius = h / 2
	}

	// P1 gradient replaces the solid fill (Style.UseBg still wins above).
	useGradient := b.HasGradient() && !b.ghost && !b.style.UseBg
	if useGradient {
		from, to, _ := b.GradientColors()
		pc.PushClipRRect(0, 0, w, h, radius)
		rendering.FillLinearGradient(pc, 0, 0, w, h, 0, 0, w, h,
			from.R, from.G, from.B, from.A, to.R, to.G, to.B, to.A)
		pc.PopClip()
	} else if fill.A > 0 {
		rendering.FillRoundRect(pc, 0, 0, w, h, radius, fill.R, fill.G, fill.B, fill.A)
	}
	if hasBorder && w > 2 && h > 2 {
		if pc.DC != nil && dashed {
			pc.DC.SetDash(dashOn, dashOff)
		}
		rendering.StrokeRoundRect(pc, 0.5, 0.5, w-1, h-1, math.Max(radius-0.5, 0), 1, border.R, border.G, border.B, border.A)
		if pc.DC != nil && dashed {
			pc.DC.SetDash()
		}
	}

	leadCX, textCX, _, hasLead, hasText := b.slots(w, h)
	cy := h / 2
	if hasLead {
		lw := b.leadingWidth()
		r := lw * 0.32
		ink := b.IconColor()
		if b.HasSpinner() {
			// Spinner ring (Tick-driven rotation is host-owned; static arc here).
			if b.loadingIcon != "" {
				// P1 custom loading icon: solid dot instead of the ring.
				rendering.FillCircle(pc, leadCX, cy, r, ink.R, ink.G, ink.B, ink.A)
			} else {
				rendering.StrokeArc(pc, leadCX, cy, r, 0.6, 5.4, math.Max(2, r*0.4), ink.R, ink.G, ink.B, ink.A)
				rendering.FillCircle(pc, leadCX+r*0.85, cy-r*0.35, math.Max(1.2, r*0.28), ink.R, ink.G, ink.B, ink.A)
			}
		} else {
			rendering.StrokeCircle(pc, leadCX, cy, r, math.Max(1.6, r*0.35), ink.R, ink.G, ink.B, ink.A)
		}
	}
	disp := b.DisplayLabel()
	if hasText && pc.DC != nil && disp != "" {
		if b.textFace != nil {
			pc.DC.SetFont(b.textFace)
		}
		pc.DC.SetRGBA(text.R, text.G, text.B, text.A)
		// ay=0 anchors the text top; center the font-size block in h.
		top := (h - b.FontSize()) / 2
		if top < 0 {
			top = 0
		}
		// True-text chain: SetFont above + DrawStringAnchored via Abs coords.
		ax, ay := pc.Abs(textCX, top)
		pc.DC.DrawStringAnchored(disp, ax, ay, 0.5, 0)
	}
	// P1 wave ripple: pressed + WaveActive paints an outer ring.
	if b.pressed && b.inBound && b.WaveActive() && !b.Disabled() {
		tok := b.themeTokens()
		wc := b.accent(tok)
		if wc.A == 0 {
			wc = themeToRGBA(tok.ColorPrimary)
		}
		wc.A = 0.55
		rendering.StrokeRoundRect(pc, -3, -3, w+6, h+6, radius+3, 2, wc.R, wc.G, wc.B, wc.A)
	}

	if b.focused && !b.disabled {
		tok := b.themeTokens()
		ring := b.accent(tok)
		if ring.A == 0 {
			ring = themeToRGBA(tok.ColorPrimary)
		}
		rx, ry, rw, rh := focus.FocusRingRect(0, 0, w, h, focusRingOutset)
		rendering.StrokeRoundRect(pc, rx, ry, rw, rh, radius+focusRingOutset, tok.ControlOutlineWidth, ring.R, ring.G, ring.B, ring.A)
		if tok.ControlOutlineWidth <= 0 {
			rendering.StrokeRoundRect(pc, rx, ry, rw, rh, radius+focusRingOutset, 2, ring.R, ring.G, ring.B, ring.A)
		}
	}
}

// ContrastRatio flattens fg over bg (sRGB) and returns the WCAG ratio.
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

func relLum(r, g, bl float64) float64 {
	lin := func(v float64) float64 {
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(bl)
}
