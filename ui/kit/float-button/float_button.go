package float_button

import (
	"math"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
	"github.com/energye/gpui/ui/theme"
)

// Geometry baselines (docs/antd/float-button.md §6.2.1, scale=1).
// Widgets read live values from theme tokens; these are fallbacks.
const (
	FloatButtonSize            = 40.0
	FloatButtonIconSize        = 18.0
	FloatButtonContentFontSize = 12.0
	FloatButtonVPadding        = 4.0
	FloatButtonSquareRadius    = 8.0
	FloatButtonGroupGap        = 16.0
	FloatButtonFocusOutset     = 1.5
	DefaultFloatButtonIcon     = "info-circle"
)

// spinPeriodSec is one loading-spinner revolution.
const spinPeriodSec = 1.0

// ButtonType selects the FloatButton skin (antd type).
type ButtonType string

const (
	ButtonTypeDefault ButtonType = "default"
	ButtonTypePrimary ButtonType = "primary"
)

// FloatButtonShape selects the FloatButton outline (antd shape).
type FloatButtonShape string

const (
	FloatButtonShapeCircle FloatButtonShape = "circle"
	FloatButtonShapeSquare FloatButtonShape = "square"
)

// FloatButton is the single floating button (docs/antd/float-button.md §6.10).
//
// It owns a rendering.RenderBox node: put Node() in the tree and drive Tick
// via a scheduler.TickerRegistry for the loading spinner.
type FloatButton struct {
	typ       ButtonType
	shape     FloatButtonShape
	icon      string
	content   string
	tooltip   string
	disabled  bool
	loading   bool
	ariaLabel string
	onClick   func()

	// P1 fields (docs/antd/float-button.md §6.8; staged in this package).
	badgeCount    int
	hasBadge      bool
	badgeDot      bool
	badgeOverflow int
	badgeShowZero bool
	href          string
	target        string
	htmlType      FloatHtmlType
	onNavigate    func(href, target string)
	classNames    map[FloatSemanticKey]string
	semStyles     map[FloatSemanticKey]FloatSemanticStyle
	draggable     bool
	dragX         float64
	dragY         float64
	tipPlacement  FloatTooltipPlacement
	tipDelayMs    int

	hovered      bool
	pressed      bool
	focused      bool
	phase        float64
	reduceMotion bool

	provider *theme.Provider
	override *theme.Tokens

	// textFace is the paint-only font face (nil keeps headless estimate).
	textFace text.Face

	node     *rendering.RenderBox
	attached *scheduler.TickerRegistry
}

// NewFloatButton creates a default button (type=default, shape=circle).
func NewFloatButton() *FloatButton {
	b := &FloatButton{typ: ButtonTypeDefault, shape: FloatButtonShapeCircle}
	b.applyGlobalDefaults()
	b.node = rendering.NewRenderBox()
	b.node.SetRepaintBoundary(true)
	self := b
	b.node.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	b.syncNode()
	return b
}

// SetType selects default/primary (other values fall back to default).
func (b *FloatButton) SetType(t ButtonType) {
	if b == nil {
		return
	}
	if t != ButtonTypePrimary {
		t = ButtonTypeDefault
	}
	if b.typ == t {
		return
	}
	b.typ = t
	b.dirty()
}

// Type returns the effective type.
func (b *FloatButton) Type() ButtonType {
	if b == nil || b.typ != ButtonTypePrimary {
		return ButtonTypeDefault
	}
	return ButtonTypePrimary
}

// SetShape selects circle/square (other values fall back to circle).
func (b *FloatButton) SetShape(s FloatButtonShape) {
	if b == nil {
		return
	}
	if s != FloatButtonShapeSquare {
		s = FloatButtonShapeCircle
	}
	if b.shape == s {
		return
	}
	b.shape = s
	b.dirty()
}

// Shape returns the effective shape.
func (b *FloatButton) Shape() FloatButtonShape {
	if b == nil || b.shape != FloatButtonShapeSquare {
		return FloatButtonShapeCircle
	}
	return FloatButtonShapeSquare
}

// SetIcon sets the glyph name (empty clears; see EffectiveIcon).
func (b *FloatButton) SetIcon(name string) {
	if b == nil || b.icon == name {
		return
	}
	b.icon = name
	b.dirty()
}

// Icon returns the raw icon setting.
func (b *FloatButton) Icon() string {
	if b == nil {
		return ""
	}
	return b.icon
}

// EffectiveIcon resolves the painted glyph: explicit icon wins; empty icon
// with empty content falls back to the default glyph; content-only buttons
// paint no icon.
func (b *FloatButton) EffectiveIcon() string {
	if b == nil {
		return ""
	}
	if b.icon != "" {
		return b.icon
	}
	if b.content == "" {
		return DefaultFloatButtonIcon
	}
	return ""
}

// SetContent sets the antd content text (replaces deprecated description).
func (b *FloatButton) SetContent(s string) {
	if b == nil || b.content == s {
		return
	}
	b.content = s
	b.dirty()
}

// Content returns the text content.
func (b *FloatButton) Content() string {
	if b == nil {
		return ""
	}
	return b.content
}

// SetTooltip sets the hover bubble text (empty clears).
func (b *FloatButton) SetTooltip(s string) {
	if b == nil || b.tooltip == s {
		return
	}
	b.tooltip = s
	b.dirty()
}

// SetTextFace sets the paint-only font face (nil clears; layout keeps the
// rune estimate so headless tests stay stable).
func (b *FloatButton) SetTextFace(f text.Face) {
	if b == nil {
		return
	}
	b.textFace = f
	b.dirty()
}

// Tooltip returns the bubble text.
func (b *FloatButton) Tooltip() string {
	if b == nil {
		return ""
	}
	return b.tooltip
}

// TooltipVisible reports whether the hover bubble shows (P0 string mapping;
// full TooltipProps positioning is P1). Never shows while disabled.
func (b *FloatButton) TooltipVisible() bool {
	return b != nil && b.hovered && b.tooltip != "" && !b.disabled
}

// SetDisabled toggles the disabled state (swallows activation).
func (b *FloatButton) SetDisabled(v bool) {
	if b == nil || b.disabled == v {
		return
	}
	b.disabled = v
	if v {
		b.focused = false
	}
	b.dirty()
}

// Disabled reports the disabled flag.
func (b *FloatButton) Disabled() bool { return b != nil && b.disabled }

// SetLoading toggles the spinner state (swallows repeated clicks, FB-S13).
func (b *FloatButton) SetLoading(v bool) {
	if b == nil || b.loading == v {
		return
	}
	b.loading = v
	b.dirty()
}

// Loading reports the spinner flag.
func (b *FloatButton) Loading() bool { return b != nil && b.loading }

// SetReduceMotion freezes the spinner phase (accessibility).
func (b *FloatButton) SetReduceMotion(v bool) {
	if b == nil {
		return
	}
	b.reduceMotion = v
}

// Phase returns the spinner phase in [0,1).
func (b *FloatButton) Phase() float64 {
	if b == nil {
		return 0
	}
	return b.phase
}

// SetOnClick sets the click callback.
func (b *FloatButton) SetOnClick(fn func()) {
	if b == nil {
		return
	}
	b.onClick = fn
}

// Click activates the button once; disabled/loading swallow the event.
func (b *FloatButton) Click() bool {
	if b == nil || b.disabled || b.loading {
		return false
	}
	if b.onClick != nil {
		b.onClick()
	}
	// P1 href mapping: desktop offers OnNavigate instead of <a> navigation.
	if b.href != "" && b.onNavigate != nil {
		b.onNavigate(b.href, b.target)
	}
	return true
}

// SetHover updates the hover state (paint-only; no layout).
func (b *FloatButton) SetHover(v bool) {
	if b == nil || b.hovered == v {
		return
	}
	b.hovered = v
	b.dirty()
}

// Hovered reports the hover flag.
func (b *FloatButton) Hovered() bool { return b != nil && b.hovered }

// SetPressed updates the pressed state (paint-only; no layout).
func (b *FloatButton) SetPressed(v bool) {
	if b == nil || b.pressed == v {
		return
	}
	b.pressed = v
	b.dirty()
}

// Pressed reports the pressed flag.
func (b *FloatButton) Pressed() bool { return b != nil && b.pressed }

// HasHoverHighlight reports whether hover feedback applies (never on disabled).
func (b *FloatButton) HasHoverHighlight() bool { return b != nil && !b.disabled }

// Role returns the accessible role.
func (b *FloatButton) Role() string { return "button" }

// Focusable reports Tab reachability (disabled buttons are skipped).
func (b *FloatButton) Focusable() bool { return b != nil && !b.disabled }

// Focus takes keyboard focus; false when disabled.
func (b *FloatButton) Focus() bool {
	if b == nil || b.disabled {
		return false
	}
	if b.focused {
		return true
	}
	b.focused = true
	b.dirty()
	return true
}

// Blur releases keyboard focus.
func (b *FloatButton) Blur() {
	if b == nil || !b.focused {
		return
	}
	b.focused = false
	b.dirty()
}

// Focused reports keyboard focus.
func (b *FloatButton) Focused() bool { return b != nil && b.focused }

// FocusRingVisible reports whether the focus ring paints.
func (b *FloatButton) FocusRingVisible() bool {
	return b != nil && b.focused && !b.disabled
}

// KeyActivate handles Enter/Space while focused (false when not activatable).
func (b *FloatButton) KeyActivate(key string) bool {
	if b == nil || !b.focused || b.disabled || b.loading {
		return false
	}
	switch key {
	case "Enter", "Space", " ":
		return b.Click()
	}
	return false
}

// SetAriaLabel sets the accessible name (required for icon-only buttons).
func (b *FloatButton) SetAriaLabel(s string) {
	if b == nil {
		return
	}
	b.ariaLabel = s
}

// AriaLabel returns the accessible name ("" means unnamed).
func (b *FloatButton) AriaLabel() string {
	if b == nil {
		return ""
	}
	return b.ariaLabel
}

// AccessibleName resolves the spoken name: explicit label wins, else content.
func (b *FloatButton) AccessibleName() string {
	if b == nil {
		return ""
	}
	if b.ariaLabel != "" {
		return b.ariaLabel
	}
	return b.content
}

// NeedAriaLabel reports whether an explicit label is required (icon-only).
func (b *FloatButton) NeedAriaLabel() bool {
	return b != nil && b.content == "" && b.ariaLabel == ""
}

// SetProvider selects the theme source (nil selects process default).
func (b *FloatButton) SetProvider(p *theme.Provider) {
	if b == nil {
		return
	}
	b.provider = p
	b.dirty()
}

// SetTheme pins exact tokens (nil clears to provider).
func (b *FloatButton) SetTheme(t *theme.Tokens) {
	if b == nil {
		return
	}
	b.override = t
	b.dirty()
}

func (b *FloatButton) themeTokens() theme.Tokens {
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

func shade(c render.RGBA, f float64) render.RGBA {
	return render.RGBA{R: c.R * f, G: c.G * f, B: c.B * f, A: c.A}
}

func mixWhite(c render.RGBA, t float64) render.RGBA {
	return render.RGBA{
		R: c.R + (1-c.R)*t,
		G: c.G + (1-c.G)*t,
		B: c.B + (1-c.B)*t,
		A: c.A,
	}
}

// EffectiveSize is the button edge (controlHeightLG token, §6.2.1).
func (b *FloatButton) EffectiveSize() float64 {
	tok := b.themeTokens()
	if tok.ControlHeightLG > 0 {
		return tok.ControlHeightLG
	}
	return FloatButtonSize
}

// EffectiveIconSize is the glyph edge (fontSizeIcon×1.5, §6.2.1).
func (b *FloatButton) EffectiveIconSize() float64 {
	tok := b.themeTokens()
	base := tok.FontSizeIcon
	if base <= 0 {
		base = 12
	}
	return base * 1.5
}

// ContentFontSize is the content text size (fontSizeSM token, §6.2.1).
func (b *FloatButton) ContentFontSize() float64 {
	tok := b.themeTokens()
	if tok.FontSizeSM > 0 {
		return tok.FontSizeSM
	}
	return FloatButtonContentFontSize
}

// EffectiveRadius is the corner radius (circle: edge/2; square: borderRadiusLG).
func (b *FloatButton) EffectiveRadius() float64 {
	if b.Shape() == FloatButtonShapeSquare {
		tok := b.themeTokens()
		if tok.RadiusLG > 0 {
			return tok.RadiusLG
		}
		return FloatButtonSquareRadius
	}
	return b.EffectiveSize() / 2
}

// EffectiveBorderWidth is the outline width (lineWidth token).
func (b *FloatButton) EffectiveBorderWidth() float64 {
	tok := b.themeTokens()
	if tok.LineWidth > 0 {
		return tok.LineWidth
	}
	return 1
}

// EffectiveBackground is the state-aware fill color (§6.5 chrome).
func (b *FloatButton) EffectiveBackground() render.RGBA {
	tok := b.themeTokens()
	if b != nil && b.disabled {
		return themeToRGBA(tok.ColorFillTertiary)
	}
	// P1 semantic root override (explicit style wins on enabled buttons).
	if s, ok := b.semStyle(FloatSemanticRoot); ok && s.UseBg {
		return s.Bg
	}
	if b.Type() == ButtonTypePrimary {
		base := themeToRGBA(tok.ColorPrimary)
		if b != nil && b.pressed {
			return shade(base, 0.9)
		}
		if b != nil && b.hovered {
			return mixWhite(base, 0.12)
		}
		return base
	}
	return themeToRGBA(tok.ColorBgContainer)
}

// EffectiveForeground is the state-aware icon/text color (§6.5 chrome).
func (b *FloatButton) EffectiveForeground() render.RGBA {
	tok := b.themeTokens()
	if b != nil && b.disabled {
		return themeToRGBA(tok.ColorTextDisabled)
	}
	// P1 semantic ink overrides (content wins over icon).
	if s, ok := b.semStyle(FloatSemanticContent); ok && s.UseText {
		return s.Text
	}
	if s, ok := b.semStyle(FloatSemanticIcon); ok && s.UseText {
		return s.Text
	}
	if b.Type() == ButtonTypePrimary {
		return themeToRGBA(tok.ColorWhite)
	}
	if b != nil && (b.hovered || b.pressed) {
		return themeToRGBA(tok.ColorPrimary)
	}
	return themeToRGBA(tok.ColorText)
}

// EffectiveBorder is the state-aware outline color (§6.5 chrome).
func (b *FloatButton) EffectiveBorder() render.RGBA {
	tok := b.themeTokens()
	if b != nil && b.disabled {
		return themeToRGBA(tok.ColorBorderSecondary)
	}
	if b.Type() == ButtonTypePrimary {
		return b.EffectiveBackground()
	}
	if b != nil && (b.hovered || b.pressed) {
		return themeToRGBA(tok.ColorPrimary)
	}
	return themeToRGBA(tok.ColorBorder)
}

// Node returns the tree node (layout/paint/hit through it).
func (b *FloatButton) Node() rendering.RenderObject {
	if b == nil {
		return nil
	}
	b.syncNode()
	return b.node
}

// ChromeNode returns the decorated chrome node (§6.11 Decorated layer).
func (b *FloatButton) ChromeNode() rendering.RenderObject { return b.Node() }

// Layout sizes the node to EffectiveSize under constraints.
func (b *FloatButton) Layout(c rendering.Constraints) rendering.Size {
	if b == nil {
		return rendering.Size{}
	}
	b.syncNode()
	return b.node.Layout(c)
}

// HitContains reports whether a node-local point hits the button.
func (b *FloatButton) HitContains(x, y float64) bool {
	if b == nil {
		return false
	}
	s := b.EffectiveSize()
	return x >= 0 && y >= 0 && x < s && y < s
}

// Attach registers the loading ticker.
func (b *FloatButton) Attach(reg *scheduler.TickerRegistry) {
	if b == nil || reg == nil {
		return
	}
	if b.attached != nil && b.attached != reg {
		b.attached.Remove(b)
	}
	b.attached = reg
	reg.Add(b)
}

// Detach unregisters the loading ticker.
func (b *FloatButton) Detach() {
	if b == nil || b.attached == nil {
		return
	}
	b.attached.Remove(b)
	b.attached = nil
}

// Tick advances the spinner phase (scheduler.Ticker). Stays registered.
func (b *FloatButton) Tick(dt float64) bool {
	if b == nil {
		return false
	}
	if !b.loading || b.reduceMotion {
		return true
	}
	if dt < 0 {
		dt = 0
	}
	b.phase = math.Mod(b.phase+dt/spinPeriodSec, 1)
	if b.phase < 0 {
		b.phase++
	}
	b.dirty()
	return true
}

// WantsFrame reports spinner frame demand (scheduler.FrameWanter).
func (b *FloatButton) WantsFrame() bool {
	return b != nil && b.loading && !b.reduceMotion
}

func (b *FloatButton) syncNode() {
	if b == nil || b.node == nil {
		return
	}
	s := b.EffectiveSize()
	b.node.FixedWidth = s
	b.node.FixedHeight = s
}

// syncSize refreshes the fixed edge from tokens (group reuse, same package).
func (b *FloatButton) syncSize() { b.syncNode() }

func (b *FloatButton) dirty() {
	if b == nil || b.node == nil {
		return
	}
	b.syncNode()
	b.node.MarkNeedsPaint()
}

func (b *FloatButton) paint(pc *rendering.PaintContext, size rendering.Size) {
	if pc == nil || size.Width <= 0 || size.Height <= 0 {
		return
	}
	bg := b.EffectiveBackground()
	bd := b.EffectiveBorder()
	fg := b.EffectiveForeground()
	lw := b.EffectiveBorderWidth()
	cx, cy := size.Width/2, size.Height/2
	if b.Shape() == FloatButtonShapeSquare {
		r := b.EffectiveRadius()
		rendering.FillRoundRect(pc, 0, 0, size.Width, size.Height, r, bg.R, bg.G, bg.B, bg.A)
		rendering.StrokeRoundRect(pc, 0, 0, size.Width, size.Height, r, lw, bd.R, bd.G, bd.B, bd.A)
	} else {
		r := size.Width / 2
		if size.Height/2 < r {
			r = size.Height / 2
		}
		rendering.FillCircle(pc, cx, cy, r, bg.R, bg.G, bg.B, bg.A)
		rendering.StrokeCircle(pc, cx, cy, r-lw/2, lw, bd.R, bd.G, bd.B, bd.A)
	}
	if b.FocusRingVisible() {
		tok := b.themeTokens()
		rc := themeToRGBA(tok.ColorPrimary)
		rw := tok.ControlOutlineWidth
		if rw <= 0 {
			rw = 2
		}
		if b.Shape() == FloatButtonShapeSquare {
			o := FloatButtonFocusOutset
			r := b.EffectiveRadius() + o
			rendering.StrokeRoundRect(pc, -o, -o, size.Width+2*o, size.Height+2*o, r, rw, rc.R, rc.G, rc.B, rc.A)
		} else {
			rendering.StrokeCircle(pc, cx, cy, size.Width/2+FloatButtonFocusOutset, rw, rc.R, rc.G, rc.B, rc.A)
		}
	}
	if name := b.EffectiveIcon(); name != "" && !b.loading {
		s := b.EffectiveIconSize()
		ox := (size.Width - s) / 2
		oy := (size.Height - s) / 2
		if b.content != "" {
			oy = size.Height/2 - 7 - s/2
		}
		drawFloatIcon(pc, name, ox, oy, s, fg)
	}
	if b.content != "" {
		fs := b.ContentFontSize()
		w, _ := rendering.EstimateTextSize(b.content, fs, 0.55)
		x := (size.Width - w) / 2
		if x < FloatButtonVPadding {
			x = FloatButtonVPadding
		}
		y := size.Height/2 + 4
		if b.EffectiveIcon() != "" {
			y = size.Height/2 + 13
		}
		if pc.DC != nil {
			ax, ay := pc.Abs(x, y)
			if b.textFace != nil {
				pc.DC.SetFont(b.textFace)
			}
			pc.DC.SetRGBA(fg.R, fg.G, fg.B, fg.A)
			pc.DC.DrawString(b.content, ax, ay)
		}
	}
	if b.loading {
		ang := b.phase * 2 * math.Pi
		rendering.StrokeArc(pc, cx, cy, 9, ang, ang+4.2, 2, fg.R, fg.G, fg.B, fg.A)
	}
	// P1 badge overlay (never steals the main click).
	b.paintBadge(pc, size)
}

func iconLineWidth(s float64) float64 {
	w := s * 0.11
	if w < 1.6 {
		return 1.6
	}
	if w > 2.5 {
		return 2.5
	}
	return w
}

// drawFloatIcon paints a deterministic glyph in the s×s box at (x,y).
func drawFloatIcon(pc *rendering.PaintContext, name string, x, y, s float64, fg render.RGBA) {
	r, g, bl, a := fg.R, fg.G, fg.B, fg.A
	w := iconLineWidth(s)
	ox := func(f float64) float64 { return x + s*f }
	oy := func(f float64) float64 { return y + s*f }
	switch name {
	case "plus":
		rendering.StrokeLine(pc, ox(0.5), oy(0.24), ox(0.5), oy(0.76), w, r, g, bl, a)
		rendering.StrokeLine(pc, ox(0.24), oy(0.5), ox(0.76), oy(0.5), w, r, g, bl, a)
	case "close":
		rendering.StrokeLine(pc, ox(0.28), oy(0.28), ox(0.72), oy(0.72), w, r, g, bl, a)
		rendering.StrokeLine(pc, ox(0.72), oy(0.28), ox(0.28), oy(0.72), w, r, g, bl, a)
	case "up":
		p := rendering.NewPath()
		p.MoveTo(ox(0.24), oy(0.62))
		p.LineTo(ox(0.5), oy(0.36))
		p.LineTo(ox(0.76), oy(0.62))
		rendering.StrokePath(pc, p, w, r, g, bl, a)
	case "search":
		rendering.StrokeCircle(pc, ox(0.44), oy(0.44), s*0.24, w, r, g, bl, a)
		rendering.StrokeLine(pc, ox(0.62), oy(0.62), ox(0.80), oy(0.80), w, r, g, bl, a)
	case "info", "info-circle", "question-circle":
		rendering.StrokeCircle(pc, ox(0.5), oy(0.5), s*0.32, w, r, g, bl, a)
		rendering.StrokeLine(pc, ox(0.5), oy(0.44), ox(0.5), oy(0.66), w, r, g, bl, a)
		rendering.FillCircle(pc, ox(0.5), oy(0.32), w*0.5, r, g, bl, a)
	default:
		p := rendering.NewPath()
		p.MoveTo(ox(0.5), oy(0.12))
		p.LineTo(ox(0.88), oy(0.5))
		p.LineTo(ox(0.5), oy(0.88))
		p.LineTo(ox(0.12), oy(0.5))
		p.Close()
		rendering.StrokePath(pc, p, w, r, g, bl, a)
	}
}
