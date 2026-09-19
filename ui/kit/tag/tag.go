// Package tag implements the tag control (docs/antd/tag.md §6).
//
// Three layers share one file: Tag, CheckableTag and CheckableTagGroup.
// The widget owns rendering nodes (Node) and reuses ui/rendering for
// layout/paint, ui/theme for tokens. No second event or frame system;
// hit == layout == paint on every node.
package tag

import (
	"strings"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Geometry fallbacks when theme tokens are zero (antd §6.2.1).
const (
	DefaultTagFontSize = 12.0
	DefaultTagRadius   = 4.0
	DefaultTagPadH     = 7.0
	DefaultTagPadV     = 1.0
	DefaultTagHeight   = 22.0
	DefaultTagIconSize = 10.0
	DefaultTagGroupGap = 8.0
	DefaultTagCloseGap = 3.0
)

// TagVariant selects the chrome style (antd variant).
type TagVariant string

const (
	TagVariantFilled   TagVariant = "filled"
	TagVariantOutlined TagVariant = "outlined"
	TagVariantSolid    TagVariant = "solid"
)

// Chrome is the resolved background/border/text triple.
type Chrome struct {
	Bg          render.RGBA
	Border      render.RGBA
	BorderWidth float64
	Text        render.RGBA
}

// TagCloseEvent carries OnClose; PreventDefault keeps the tag visible.
type TagCloseEvent struct {
	Tag       *Tag
	prevented bool
}

// PreventDefault stops the default hide.
func (e *TagCloseEvent) PreventDefault() {
	if e != nil {
		e.prevented = true
	}
}

// Prevented reports whether PreventDefault was called.
func (e *TagCloseEvent) Prevented() bool { return e != nil && e.prevented }

// presetHex covers the 13 antd preset names (§6.5).
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

func themeToRGBA(c theme.Color) render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

func lighten(c render.RGBA, t float64) render.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return render.RGBA{
		R: c.R + (1-c.R)*t,
		G: c.G + (1-c.G)*t,
		B: c.B + (1-c.B)*t,
		A: c.A,
	}
}

func statusBase(name string, tok theme.Tokens) (render.RGBA, bool) {
	switch name {
	case "success":
		return themeToRGBA(tok.ColorSuccess), true
	case "processing", "info":
		return themeToRGBA(tok.ColorInfo), true
	case "error":
		return themeToRGBA(tok.ColorError), true
	case "warning":
		return themeToRGBA(tok.ColorWarning), true
	default:
		return render.RGBA{}, false
	}
}

func chromeFromBase(base render.RGBA, variant TagVariant) Chrome {
	white := render.White
	switch variant {
	case TagVariantSolid:
		return Chrome{Bg: base, Border: base, BorderWidth: 0, Text: white}
	case TagVariantOutlined:
		return Chrome{Bg: lighten(base, 0.9), Border: lighten(base, 0.6), BorderWidth: 1, Text: base}
	default:
		return Chrome{Bg: lighten(base, 0.9), Border: lighten(base, 0.9), BorderWidth: 0, Text: base}
	}
}

func defaultChrome(variant TagVariant, tok theme.Tokens) Chrome {
	switch variant {
	case TagVariantSolid:
		bg := render.RGBA{R: 0, G: 0, B: 0, A: 0.88}
		return Chrome{Bg: bg, Border: bg, BorderWidth: 0, Text: render.White}
	case TagVariantOutlined:
		return Chrome{
			Bg:          themeToRGBA(tok.ColorFillTertiary),
			Border:      themeToRGBA(tok.ColorBorder),
			BorderWidth: 1,
			Text:        themeToRGBA(tok.ColorText),
		}
	default:
		return Chrome{
			Bg:          themeToRGBA(tok.ColorFillTertiary),
			Border:      themeToRGBA(tok.ColorFillTertiary),
			BorderWidth: 0,
			Text:        themeToRGBA(tok.ColorText),
		}
	}
}

// Tag is the main label (docs/antd/tag.md §6.10).
//
// Owns a tagBox node: put Node() in the tree, drive Layout through it,
// read EffectiveChrome for assertions.
type Tag struct {
	label      string
	value      string
	hasValue   bool
	color      string
	colorRGBA  render.RGBA
	hasRGBA    bool
	variant    TagVariant
	variantSet bool
	bordered   bool
	closable   bool
	closeIcon  rendering.RenderObject
	iconName   string
	iconNode   rendering.RenderObject
	disabled   bool
	// hidden/focused are atomic: event writes (UI), paint reads (raster).
	hidden  atomic.Bool
	onClick func()
	// OnClose fires on close press; PreventDefault keeps visible.
	OnClose   func(*TagCloseEvent)
	provider  *theme.Provider
	override  *theme.Tokens
	face      text.Face
	style     map[string]string
	className string
	ariaLabel string
	focused   atomic.Bool
	node      *tagBox
}

// NewTag creates a filled non-closable enabled tag.
func NewTag(label string) *Tag {
	t := &Tag{label: label, variant: TagVariantFilled, bordered: true}
	t.node = newTagBox(t)
	return t
}

// Label returns the display text.
func (t *Tag) Label() string {
	if t == nil {
		return ""
	}
	return t.label
}

// SetLabel sets the display text (layout).
func (t *Tag) SetLabel(s string) {
	if t == nil || t.label == s {
		return
	}
	t.label = s
	t.markLayout()
}

// Value returns the value (defaults to label).
func (t *Tag) Value() string {
	if t == nil {
		return ""
	}
	if t.hasValue {
		return t.value
	}
	return t.label
}

// SetValue sets the value (layout, display follows label).
func (t *Tag) SetValue(s string) {
	if t == nil {
		return
	}
	t.value = s
	t.hasValue = true
	t.markLayout()
}

// SetColor sets preset/status/#hex/"" (paint only).
func (t *Tag) SetColor(nameOrHex string) {
	if t == nil || t.color == nameOrHex {
		return
	}
	t.color = nameOrHex
	t.hasRGBA = false
	t.markPaint()
}

// Color returns the raw color string.
func (t *Tag) Color() string {
	if t == nil {
		return ""
	}
	return t.color
}

// SetColorRGBA pins an explicit color (paint only, A<=0 clears).
func (t *Tag) SetColorRGBA(c render.RGBA) {
	if t == nil {
		return
	}
	t.colorRGBA = c
	t.hasRGBA = c.A > 0
	if c.A <= 0 {
		t.hasRGBA = false
	}
	t.markPaint()
}

// SetVariant sets filled/solid/outlined (layout for border, paint for skin).
func (t *Tag) SetVariant(v TagVariant) {
	if t == nil {
		return
	}
	if v != TagVariantSolid && v != TagVariantOutlined {
		v = TagVariantFilled
	}
	if t.variant == v && t.variantSet {
		return
	}
	t.variant = v
	t.variantSet = true
	t.markLayout()
}

// Variant returns the raw variant (default filled).
func (t *Tag) Variant() TagVariant {
	if t == nil || t.variant == "" {
		return TagVariantFilled
	}
	return t.variant
}

// EffectiveVariant applies bordered=false compat (false forces filled).
func (t *Tag) EffectiveVariant() TagVariant {
	if t != nil && !t.bordered {
		return TagVariantFilled
	}
	return t.Variant()
}

// SetBordered sets the deprecated bordered compat (layout).
func (t *Tag) SetBordered(b bool) {
	if t == nil || t.bordered == b {
		return
	}
	t.bordered = b
	t.markLayout()
}

// Bordered returns the compat flag (default true).
func (t *Tag) Bordered() bool { return t == nil || t.bordered }

// EffectiveBorderWidth is 1 for outlined, else 0 (TAG-S6/S7).
func (t *Tag) EffectiveBorderWidth() float64 {
	if t != nil && t.disabled {
		return 0
	}
	if t.EffectiveVariant() == TagVariantOutlined {
		return 1
	}
	return 0
}

// SetClosable toggles the close affordance (layout).
func (t *Tag) SetClosable(b bool) {
	if t == nil || t.closable == b {
		return
	}
	t.closable = b
	t.markLayout()
}

// Closable reports the close flag.
func (t *Tag) Closable() bool { return t != nil && t.closable }

// SetCloseIcon stores a custom close node (paint only).
func (t *Tag) SetCloseIcon(n rendering.RenderObject) {
	if t == nil {
		return
	}
	t.closeIcon = n
	t.markPaint()
}

// CloseNode returns the close affordance (nil when not closable).
func (t *Tag) CloseNode() rendering.RenderObject {
	if t == nil || !t.closable {
		return nil
	}
	if t.closeIcon != nil {
		return t.closeIcon
	}
	return t.node
}

// HasClose reports whether a close affordance exists.
func (t *Tag) HasClose() bool { return t != nil && t.closable }

// SetIcon sets the leading icon name (layout).
func (t *Tag) SetIcon(name string) {
	if t == nil || t.iconName == name {
		return
	}
	t.iconName = name
	t.markLayout()
}

// SetIconNode stores a custom icon node (layout).
func (t *Tag) SetIconNode(n rendering.RenderObject) {
	if t == nil {
		return
	}
	t.iconNode = n
	t.markLayout()
}

// IconName returns the icon name.
func (t *Tag) IconName() string {
	if t == nil {
		return ""
	}
	return t.iconName
}

// HasIcon reports whether a leading icon exists.
func (t *Tag) HasIcon() bool { return t != nil && (t.iconName != "" || t.iconNode != nil) }

// SetDisabled toggles disabled (paint only, no hover/close/change).
func (t *Tag) SetDisabled(b bool) {
	if t == nil || t.disabled == b {
		return
	}
	t.disabled = b
	t.markPaint()
}

// Disabled reports the flag.
func (t *Tag) Disabled() bool { return t != nil && t.disabled }

// SetOnClick sets the whole-tag click handler (paint for focusability).
func (t *Tag) SetOnClick(fn func()) {
	if t == nil {
		return
	}
	t.onClick = fn
	t.markPaint()
}

// Click simulates a whole-tag press (disabled/hidden swallow).
func (t *Tag) Click() {
	if t == nil || t.disabled || t.hidden.Load() {
		return
	}
	if t.onClick != nil {
		t.onClick()
	}
}

// Press is an alias of Click (pointer/keyboard entry).
func (t *Tag) Press() { t.Click() }

// PressKey activates on Space/Enter when focusable (TAG-S12 for Tag path).
func (t *Tag) PressKey(key string) {
	if t == nil || t.disabled || t.hidden.Load() || !t.Focusable() {
		return
	}
	if key == "Space" || key == "Enter" || key == " " || key == "\n" {
		t.Click()
	}
}

// Close simulates the close-icon press (TAG-S2/S3/S11).
func (t *Tag) Close() {
	if t == nil || t.disabled || t.hidden.Load() || !t.closable {
		return
	}
	ev := &TagCloseEvent{Tag: t}
	if t.OnClose != nil {
		t.OnClose(ev)
	}
	if ev.prevented {
		return
	}
	t.hidden.Store(true)
	t.markLayout()
}

// Show clears Hidden.
func (t *Tag) Show() {
	if t == nil || !t.hidden.Load() {
		return
	}
	t.hidden.Store(false)
	t.markLayout()
}

// Hidden reports whether the tag is hidden after close.
func (t *Tag) Hidden() bool { return t != nil && t.hidden.Load() }

// Visible is !Hidden.
func (t *Tag) Visible() bool { return !t.Hidden() }

// SetProvider selects the theme source (layout, metrics follow theme).
func (t *Tag) SetProvider(p *theme.Provider) {
	if t == nil {
		return
	}
	t.provider = p
	t.markLayout()
}

// SetTheme pins exact tokens (layout).
func (t *Tag) SetTheme(tok *theme.Tokens) {
	if t == nil {
		return
	}
	t.override = tok
	t.markLayout()
}

// SetFace stores the text face (paint only; heuristic layout keeps size).
func (t *Tag) SetFace(f text.Face) {
	if t == nil {
		return
	}
	t.face = f
	t.markPaint()
}

// SetStyle stores the semantic style hook (no CSS engine, paint only).
func (t *Tag) SetStyle(m map[string]string) {
	if t == nil {
		return
	}
	t.style = m
	t.markPaint()
}

// Style returns the stored hook.
func (t *Tag) Style() map[string]string {
	if t == nil {
		return nil
	}
	return t.style
}

// SetClassName stores the semantic hook (paint only).
func (t *Tag) SetClassName(s string) {
	if t == nil {
		return
	}
	t.className = s
	t.markPaint()
}

// ClassName returns the stored hook.
func (t *Tag) ClassName() string {
	if t == nil {
		return ""
	}
	return t.className
}

// SetAriaLabel names the tag (paint only).
func (t *Tag) SetAriaLabel(s string) {
	if t == nil || t.ariaLabel == s {
		return
	}
	t.ariaLabel = s
	t.markPaint()
}

// AriaLabel returns explicit name else label (text is name).
func (t *Tag) AriaLabel() string {
	if t == nil {
		return ""
	}
	if t.ariaLabel != "" {
		return t.ariaLabel
	}
	return t.label
}

// Role is button when focusable, else "" (static tags carry no role).
func (t *Tag) Role() string {
	if t != nil && t.Focusable() {
		return "button"
	}
	return ""
}

// Focusable is closable/clickable when enabled and visible.
func (t *Tag) Focusable() bool {
	return t != nil && !t.disabled && !t.hidden.Load() && (t.closable || t.onClick != nil)
}

// Focus sets keyboard focus (paint only).
func (t *Tag) Focus() {
	if t == nil || !t.Focusable() {
		return
	}
	t.focused.Store(true)
	t.markPaint()
}

// Blur clears focus (paint only).
func (t *Tag) Blur() {
	if t == nil || !t.focused.Load() {
		return
	}
	t.focused.Store(false)
	t.markPaint()
}

// Focused reports focus.
func (t *Tag) Focused() bool { return t != nil && t.focused.Load() && t.Focusable() }

// FocusRingVisible requires focusable + focused (must be painted).
func (t *Tag) FocusRingVisible() bool { return t.Focused() }

func (t *Tag) themeTokens() theme.Tokens {
	if t != nil && t.override != nil {
		return *t.override
	}
	if t != nil && t.provider != nil {
		return t.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveFontSize reads FontSizeSM (12).
func (t *Tag) EffectiveFontSize() float64 {
	tok := t.themeTokens()
	if tok.FontSizeSM > 0 {
		return tok.FontSizeSM
	}
	return DefaultTagFontSize
}

// EffectiveRadius reads RadiusSM (4).
func (t *Tag) EffectiveRadius() float64 {
	tok := t.themeTokens()
	if tok.RadiusSM > 0 {
		return tok.RadiusSM
	}
	return DefaultTagRadius
}

// EffectivePadH is PaddingXS-LineWidth (8-1=7).
func (t *Tag) EffectivePadH() float64 {
	tok := t.themeTokens()
	pad, lw := tok.PaddingXS, tok.LineWidth
	if pad <= 0 {
		pad = 8
	}
	if lw <= 0 {
		lw = 1
	}
	return pad - lw
}

// EffectivePadV凑 ~22 高度 (1).
func (t *Tag) EffectivePadV() float64 { return DefaultTagPadV }

// EffectiveContentH is FontHeightSM (20).
func (t *Tag) EffectiveContentH() float64 {
	tok := t.themeTokens()
	if tok.FontHeightSM > 0 {
		return tok.FontHeightSM
	}
	return 20
}

// EffectiveHeight is content + pads (≈22).
func (t *Tag) EffectiveHeight() float64 {
	return t.EffectiveContentH() + 2*t.EffectivePadV()
}

// EffectiveIconSize is FontSizeIcon-2*LineWidth (≈10).
func (t *Tag) EffectiveIconSize() float64 {
	tok := t.themeTokens()
	sz, lw := tok.FontSizeIcon, tok.LineWidth
	if sz <= 0 {
		sz = 12
	}
	if lw <= 0 {
		lw = 1
	}
	v := sz - 2*lw
	if v < 8 {
		v = 8
	}
	if v > 12 {
		v = 12
	}
	return v
}

// EffectiveCloseGap is PaddingXXS-LineWidth (≈3).
func (t *Tag) EffectiveCloseGap() float64 {
	tok := t.themeTokens()
	p, lw := tok.PaddingXXS, tok.LineWidth
	if p <= 0 {
		p = 4
	}
	if lw <= 0 {
		lw = 1
	}
	return p - lw
}

// TextWidth estimates the label width (heuristic, no font file).
func (t *Tag) TextWidth() float64 {
	if t == nil || t.label == "" {
		return 0
	}
	w, _ := rendering.EstimateTextSize(t.label, t.EffectiveFontSize(), 0.6)
	return w
}

// PreferredSize is the content size before constraints.
func (t *Tag) PreferredSize() rendering.Size {
	if t == nil || t.hidden.Load() {
		return rendering.Size{}
	}
	w := 2*t.EffectivePadH() + t.TextWidth()
	if t.HasIcon() {
		w += t.EffectiveIconSize() + t.EffectivePadH()
	}
	if t.closable {
		w += t.EffectiveIconSize() + t.EffectiveCloseGap()
	}
	return rendering.Size{Width: w, Height: t.EffectiveHeight()}
}

// EffectiveChrome resolves bg/border/text per §6.5.
func (t *Tag) EffectiveChrome() Chrome {
	tok := t.themeTokens()
	variant := t.EffectiveVariant()
	if t != nil && t.disabled {
		return Chrome{
			Bg:          themeToRGBA(tok.ColorFillSecondary),
			Border:      themeToRGBA(tok.ColorFillSecondary),
			BorderWidth: 0,
			Text:        themeToRGBA(tok.ColorTextDisabled),
		}
	}
	if t != nil && t.hasRGBA {
		return chromeFromBase(t.colorRGBA, variant)
	}
	name := ""
	if t != nil {
		name = strings.TrimSpace(strings.ToLower(t.color))
	}
	baseName := strings.TrimSuffix(name, "-inverse")
	inverse := name != baseName
	if inverse && (t == nil || !t.variantSet) {
		variant = TagVariantSolid
	}
	if name == "" || baseName == "default" && (name == "default" || name == "default-inverse") {
		return defaultChrome(variant, tok)
	}
	hexStr := name
	if inverse {
		hexStr = baseName
	}
	if strings.HasPrefix(hexStr, "#") {
		if c, err := render.ParseHex(hexStr); err == nil {
			return chromeFromBase(c, variant)
		}
		return defaultChrome(variant, tok)
	}
	key := baseName
	if key == "" {
		key = name
	}
	if hex, ok := presetHex[key]; ok {
		return chromeFromBase(render.Hex(hex), variant)
	}
	if c, ok := statusBase(key, tok); ok {
		return chromeFromBase(c, variant)
	}
	if key == "default" {
		return defaultChrome(variant, tok)
	}
	return defaultChrome(variant, tok)
}

// Node returns the tree node.
func (t *Tag) Node() rendering.RenderObject {
	if t == nil {
		return nil
	}
	return t.node
}

// ChromeNode is the painted chrome (same node, single box).
func (t *Tag) ChromeNode() rendering.RenderObject { return t.Node() }

// Layout sizes the node under constraints.
func (t *Tag) Layout(c rendering.Constraints) rendering.Size {
	if t == nil || t.node == nil {
		return rendering.Size{}
	}
	return t.node.Layout(c)
}

func (t *Tag) markLayout() {
	if t == nil || t.node == nil {
		return
	}
	t.node.MarkNeedsLayout()
}

func (t *Tag) markPaint() {
	if t == nil || t.node == nil {
		return
	}
	t.node.MarkNeedsPaint()
}

// tagBox is the layout/paint node (single owner, no second engine).
type tagBox struct {
	*rendering.RenderBox
	tag *Tag
}

func newTagBox(t *Tag) *tagBox {
	b := &tagBox{RenderBox: rendering.NewRenderBox(), tag: t}
	b.SetRepaintBoundary(true)
	b.SetRelayoutBoundary(true)
	self := b
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	return b
}

// Layout implements RenderObject (content size via Fixed, box does Tighten).
func (b *tagBox) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.tag == nil || b.RenderBox == nil {
		return rendering.Size{}
	}
	if !b.ShouldRelayout(c) {
		return b.Size()
	}
	pref := b.tag.PreferredSize()
	b.FixedWidth, b.FixedHeight = pref.Width, pref.Height
	return b.RenderBox.Layout(c)
}

// paint draws the tag chrome (called from RenderBox OnPaint).
func (b *tagBox) paint(pc *rendering.PaintContext, size rendering.Size) {
	if b == nil || pc == nil {
		return
	}
	if b.tag == nil || b.tag.hidden.Load() {
		return
	}
	t := b.tag
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	ch := t.EffectiveChrome()
	radius := t.EffectiveRadius()
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, ch.Bg.R, ch.Bg.G, ch.Bg.B, ch.Bg.A)
	if ch.BorderWidth > 0 {
		rendering.StrokeRoundRect(pc, 0, 0, w, h, radius, ch.BorderWidth, ch.Border.R, ch.Border.G, ch.Border.B, ch.Border.A)
	}
	padH := t.EffectivePadH()
	iconSize := t.EffectiveIconSize()
	cy := h / 2
	x := padH
	if t.HasIcon() {
		rendering.FillCircle(pc, x+iconSize/2, cy, iconSize*0.35, ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
		x += iconSize + padH
	}
	_ = x
	if t.closable {
		cs := t.EffectiveIconSize()
		cx := w - padH - cs/2
		r := cs * 0.28
		lw := 1.2
		if r < 1 {
			r = 1
		}
		rendering.StrokeLine(pc, cx-r, cy-r, cx+r, cy+r, lw, ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
		rendering.StrokeLine(pc, cx+r, cy-r, cx-r, cy+r, lw, ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
	}
	if t.FocusRingVisible() {
		tok := t.themeTokens()
		c := themeToRGBA(tok.ColorPrimary)
		rendering.StrokeRoundRect(pc, -1.5, -1.5, w+3, h+3, radius+1.5, 2, c.R, c.G, c.B, c.A)
	}
	if t.label != "" && pc.DC != nil {
		if t.face != nil {
			pc.DC.SetFont(t.face)
		}
		pc.DC.SetRGBA(ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
		baseline := h/2 + t.EffectiveFontSize()*0.35
		ax, ay := pc.Abs(x, baseline)
		pc.DC.DrawString(t.label, ax, ay)
	}
}

// CheckableTag is the selectable tag (checkbox semantics).
type CheckableTag struct {
	label string
	// checked/focused are atomic: same threading as Tag above.
	checked        atomic.Bool
	controlled     bool
	defaultChecked bool
	onChange       func(bool)
	iconName       string
	iconNode       rendering.RenderObject
	disabled       bool
	provider       *theme.Provider
	override       *theme.Tokens
	face           text.Face
	ariaLabel      string
	focused        atomic.Bool
	node           *checkableBox
}

// NewCheckableTag creates an unchecked tag.
func NewCheckableTag(label string) *CheckableTag {
	c := &CheckableTag{label: label}
	c.node = newCheckableBox(c)
	if c.defaultChecked {
		c.checked.Store(true)
	}
	return c
}

// Label returns the text.
func (c *CheckableTag) Label() string {
	if c == nil {
		return ""
	}
	return c.label
}

// SetLabel sets the text (layout).
func (c *CheckableTag) SetLabel(s string) {
	if c == nil || c.label == s {
		return
	}
	c.label = s
	c.markLayout()
}

// Checked reports selection.
func (c *CheckableTag) Checked() bool { return c != nil && c.checked.Load() }

// SetChecked sets controlled selection (paint only).
func (c *CheckableTag) SetChecked(b bool) {
	if c == nil {
		return
	}
	c.checked.Store(b)
	c.controlled = true
	c.markPaint()
}

// SetDefaultChecked sets the uncontrolled initial value.
func (c *CheckableTag) SetDefaultChecked(b bool) {
	if c == nil {
		return
	}
	c.defaultChecked = b
	if !c.controlled {
		c.checked.Store(b)
		c.markPaint()
	}
}

// SetOnChange sets the toggle callback.
func (c *CheckableTag) SetOnChange(fn func(bool)) {
	if c == nil {
		return
	}
	c.onChange = fn
}

// Toggle flips selection unless disabled (TAG-S4/S11/S12).
func (c *CheckableTag) Toggle() {
	if c == nil || c.disabled {
		return
	}
	c.checked.Store(!c.checked.Load())
	c.markPaint()
	if c.onChange != nil {
		c.onChange(c.checked.Load())
	}
}

// Press is an alias of Toggle.
func (c *CheckableTag) Press() { c.Toggle() }

// PressKey toggles on Space/Enter when focusable.
func (c *CheckableTag) PressKey(key string) {
	if c == nil || c.disabled || !c.Focusable() {
		return
	}
	if key == "Space" || key == "Enter" || key == " " || key == "\n" {
		c.Toggle()
	}
}

// SetIcon sets the leading icon name (layout).
func (c *CheckableTag) SetIcon(name string) {
	if c == nil || c.iconName == name {
		return
	}
	c.iconName = name
	c.markLayout()
}

// SetIconNode stores a custom icon node (layout).
func (c *CheckableTag) SetIconNode(n rendering.RenderObject) {
	if c == nil {
		return
	}
	c.iconNode = n
	c.markLayout()
}

// HasIcon reports icon presence.
func (c *CheckableTag) HasIcon() bool { return c != nil && (c.iconName != "" || c.iconNode != nil) }

// SetDisabled toggles disabled (paint only).
func (c *CheckableTag) SetDisabled(b bool) {
	if c == nil || c.disabled == b {
		return
	}
	c.disabled = b
	c.markPaint()
}

// Disabled reports the flag.
func (c *CheckableTag) Disabled() bool { return c != nil && c.disabled }

// SetProvider selects the theme source (layout).
func (c *CheckableTag) SetProvider(p *theme.Provider) {
	if c == nil {
		return
	}
	c.provider = p
	c.markLayout()
}

// SetTheme pins exact tokens (layout).
func (c *CheckableTag) SetTheme(t *theme.Tokens) {
	if c == nil {
		return
	}
	c.override = t
	c.markLayout()
}

// SetFace stores the text face (paint only).
func (c *CheckableTag) SetFace(f text.Face) {
	if c == nil {
		return
	}
	c.face = f
	c.markPaint()
}

// SetAriaLabel names the checkbox (paint only).
func (c *CheckableTag) SetAriaLabel(s string) {
	if c == nil || c.ariaLabel == s {
		return
	}
	c.ariaLabel = s
	c.markPaint()
}

// AriaLabel returns explicit name else label.
func (c *CheckableTag) AriaLabel() string {
	if c == nil {
		return ""
	}
	if c.ariaLabel != "" {
		return c.ariaLabel
	}
	return c.label
}

// Role is checkbox (a11y §6.6).
func (c *CheckableTag) Role() string { return "checkbox" }

// Focusable is true unless disabled.
func (c *CheckableTag) Focusable() bool { return c != nil && !c.disabled }

// Focus sets keyboard focus (paint only).
func (c *CheckableTag) Focus() {
	if c == nil || !c.Focusable() {
		return
	}
	c.focused.Store(true)
	c.markPaint()
}

// Blur clears focus (paint only).
func (c *CheckableTag) Blur() {
	if c == nil || !c.focused.Load() {
		return
	}
	c.focused.Store(false)
	c.markPaint()
}

// Focused reports focus.
func (c *CheckableTag) Focused() bool { return c != nil && c.focused.Load() && c.Focusable() }

// FocusRingVisible requires focused.
func (c *CheckableTag) FocusRingVisible() bool { return c.Focused() }

func (c *CheckableTag) themeTokens() theme.Tokens {
	if c != nil && c.override != nil {
		return *c.override
	}
	if c != nil && c.provider != nil {
		return c.provider.Current()
	}
	return theme.Default.Current()
}

// EffectiveFontSize reads FontSizeSM.
func (c *CheckableTag) EffectiveFontSize() float64 {
	tok := c.themeTokens()
	if tok.FontSizeSM > 0 {
		return tok.FontSizeSM
	}
	return DefaultTagFontSize
}

// EffectiveRadius reads RadiusSM.
func (c *CheckableTag) EffectiveRadius() float64 {
	tok := c.themeTokens()
	if tok.RadiusSM > 0 {
		return tok.RadiusSM
	}
	return DefaultTagRadius
}

// EffectivePadH is PaddingXS-LineWidth.
func (c *CheckableTag) EffectivePadH() float64 {
	tok := c.themeTokens()
	pad, lw := tok.PaddingXS, tok.LineWidth
	if pad <= 0 {
		pad = 8
	}
	if lw <= 0 {
		lw = 1
	}
	return pad - lw
}

// EffectiveHeight is content + pads.
func (c *CheckableTag) EffectiveHeight() float64 {
	tok := c.themeTokens()
	ch := 20.0
	if tok.FontHeightSM > 0 {
		ch = tok.FontHeightSM
	}
	return ch + 2*DefaultTagPadV
}

// EffectiveIconSize clamps FontSizeIcon-2*LineWidth.
func (c *CheckableTag) EffectiveIconSize() float64 {
	tok := c.themeTokens()
	sz, lw := tok.FontSizeIcon, tok.LineWidth
	if sz <= 0 {
		sz = 12
	}
	if lw <= 0 {
		lw = 1
	}
	v := sz - 2*lw
	if v < 8 {
		v = 8
	}
	if v > 12 {
		v = 12
	}
	return v
}

// TextWidth estimates label width.
func (c *CheckableTag) TextWidth() float64 {
	if c == nil || c.label == "" {
		return 0
	}
	w, _ := rendering.EstimateTextSize(c.label, c.EffectiveFontSize(), 0.6)
	return w
}

// PreferredSize is the content size.
func (c *CheckableTag) PreferredSize() rendering.Size {
	if c == nil {
		return rendering.Size{}
	}
	w := 2*c.EffectivePadH() + c.TextWidth()
	if c.HasIcon() {
		w += c.EffectiveIconSize() + c.EffectivePadH()
	}
	tok := c.themeTokens()
	ch := 20.0
	if tok.FontHeightSM > 0 {
		ch = tok.FontHeightSM
	}
	return rendering.Size{Width: w, Height: ch + 2*DefaultTagPadV}
}

// EffectiveChrome is primary solid when checked, default filled otherwise.
func (c *CheckableTag) EffectiveChrome() Chrome {
	tok := c.themeTokens()
	if c != nil && c.disabled {
		return Chrome{
			Bg:          themeToRGBA(tok.ColorFillSecondary),
			Border:      themeToRGBA(tok.ColorFillSecondary),
			BorderWidth: 0,
			Text:        themeToRGBA(tok.ColorTextDisabled),
		}
	}
	if c != nil && c.checked.Load() {
		p := themeToRGBA(tok.ColorPrimary)
		return Chrome{Bg: p, Border: p, BorderWidth: 0, Text: render.White}
	}
	return Chrome{
		Bg:          themeToRGBA(tok.ColorFillTertiary),
		Border:      themeToRGBA(tok.ColorFillTertiary),
		BorderWidth: 0,
		Text:        themeToRGBA(tok.ColorText),
	}
}

// Node returns the tree node.
func (c *CheckableTag) Node() rendering.RenderObject {
	if c == nil {
		return nil
	}
	return c.node
}

// ChromeNode is the painted chrome.
func (c *CheckableTag) ChromeNode() rendering.RenderObject { return c.Node() }

// Layout sizes the node.
func (c *CheckableTag) Layout(cs rendering.Constraints) rendering.Size {
	if c == nil || c.node == nil {
		return rendering.Size{}
	}
	return c.node.Layout(cs)
}

func (c *CheckableTag) markLayout() {
	if c == nil || c.node == nil {
		return
	}
	c.node.MarkNeedsLayout()
}

func (c *CheckableTag) markPaint() {
	if c == nil || c.node == nil {
		return
	}
	c.node.MarkNeedsPaint()
}

type checkableBox struct {
	*rendering.RenderBox
	tag *CheckableTag
}

func newCheckableBox(t *CheckableTag) *checkableBox {
	b := &checkableBox{RenderBox: rendering.NewRenderBox(), tag: t}
	b.SetRepaintBoundary(true)
	b.SetRelayoutBoundary(true)
	self := b
	b.OnPaint = func(pc *rendering.PaintContext, size rendering.Size) {
		self.paint(pc, size)
	}
	return b
}

// Layout implements RenderObject (content size via Fixed, box does Tighten).
func (b *checkableBox) Layout(cs rendering.Constraints) rendering.Size {
	if b == nil || b.tag == nil || b.RenderBox == nil {
		return rendering.Size{}
	}
	if !b.ShouldRelayout(cs) {
		return b.Size()
	}
	pref := b.tag.PreferredSize()
	b.FixedWidth, b.FixedHeight = pref.Width, pref.Height
	return b.RenderBox.Layout(cs)
}

// paint draws the checkable chrome (called from RenderBox OnPaint).
func (b *checkableBox) paint(pc *rendering.PaintContext, size rendering.Size) {
	if b == nil || pc == nil || b.tag == nil {
		return
	}
	t := b.tag
	w, h := size.Width, size.Height
	if w <= 0 || h <= 0 {
		return
	}
	ch := t.EffectiveChrome()
	radius := t.EffectiveRadius()
	rendering.FillRoundRect(pc, 0, 0, w, h, radius, ch.Bg.R, ch.Bg.G, ch.Bg.B, ch.Bg.A)
	if ch.BorderWidth > 0 {
		rendering.StrokeRoundRect(pc, 0, 0, w, h, radius, ch.BorderWidth, ch.Border.R, ch.Border.G, ch.Border.B, ch.Border.A)
	}
	padH := t.EffectivePadH()
	iconSize := t.EffectiveIconSize()
	x := padH
	if t.HasIcon() {
		rendering.FillCircle(pc, x+iconSize/2, h/2, iconSize*0.35, ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
		x += iconSize + padH
	}
	_ = x
	if t.FocusRingVisible() {
		tok := t.themeTokens()
		c := themeToRGBA(tok.ColorPrimary)
		rendering.StrokeRoundRect(pc, -1.5, -1.5, w+3, h+3, radius+1.5, 2, c.R, c.G, c.B, c.A)
	}
	if t.label != "" && pc.DC != nil {
		if t.face != nil {
			pc.DC.SetFont(t.face)
		}
		pc.DC.SetRGBA(ch.Text.R, ch.Text.G, ch.Text.B, ch.Text.A)
		baseline := h/2 + t.EffectiveFontSize()*0.35
		ax, ay := pc.Abs(x, baseline)
		pc.DC.DrawString(t.label, ax, ay)
	}
}

// TagOption describes one group choice.
type TagOption struct {
	Label    string
	Value    string
	Icon     string
	IconNode rendering.RenderObject
	Disabled bool
}

// CheckableTagGroup is single/multi selection over options.
type CheckableTagGroup struct {
	options       []TagOption
	tags          []*CheckableTag
	multiple      bool
	single        string
	hasSingle     bool
	multi         []string
	hasMulti      bool
	defSingle     string
	hasDefSingle  bool
	defMulti      []string
	onChange      func(string)
	onChangeMulti func([]string)
	disabled      bool
	ariaLabel     string
	node          *groupBox
}

// NewCheckableTagGroup creates a group over opts.
func NewCheckableTagGroup(opts ...TagOption) *CheckableTagGroup {
	g := &CheckableTagGroup{}
	g.node = newGroupBox(g)
	g.SetOptions(opts)
	return g
}

// SetOptions replaces choices (layout).
func (g *CheckableTagGroup) SetOptions(opts []TagOption) {
	if g == nil {
		return
	}
	cp := append([]TagOption(nil), opts...)
	g.options = cp
	g.rebuildTags()
	if g.node != nil {
		g.node.MarkNeedsLayout()
	}
}

// Options returns a copy.
func (g *CheckableTagGroup) Options() []TagOption {
	if g == nil {
		return nil
	}
	return append([]TagOption(nil), g.options...)
}

// Order returns values in display order (draggable combo).
func (g *CheckableTagGroup) Order() []string {
	if g == nil {
		return nil
	}
	out := make([]string, 0, len(g.options))
	for _, o := range g.options {
		v := o.Value
		if v == "" {
			v = o.Label
		}
		out = append(out, v)
	}
	return out
}

// Reorder moves entry from oldIndex to newIndex (layout, combo示意).
func (g *CheckableTagGroup) Reorder(oldIndex, newIndex int) {
	if g == nil || oldIndex < 0 || newIndex < 0 || oldIndex >= len(g.options) || newIndex >= len(g.options) {
		return
	}
	o := g.options[oldIndex]
	cp := append([]TagOption(nil), g.options...)
	cp = append(cp[:oldIndex], cp[oldIndex+1:]...)
	cp = append(cp[:newIndex], append([]TagOption{o}, cp[newIndex:]...)...)
	g.options = cp
	g.rebuildTags()
	if g.node != nil {
		g.node.MarkNeedsLayout()
	}
}

func (g *CheckableTagGroup) rebuildTags() {
	if g == nil || g.node == nil {
		return
	}
	for _, ch := range g.node.Children() {
		g.node.RemoveChild(ch)
	}
	g.tags = nil
	vals := g.ValuesSet()
	for _, o := range g.options {
		v := o.Value
		if v == "" {
			v = o.Label
		}
		ct := NewCheckableTag(o.Label)
		if o.Icon != "" {
			ct.SetIcon(o.Icon)
		}
		if o.IconNode != nil {
			ct.SetIconNode(o.IconNode)
		}
		if o.Disabled || g.disabled {
			ct.SetDisabled(true)
		}
		if _, ok := vals[v]; ok {
			ct.SetChecked(true)
		}
		vv := v
		ct.SetOnChange(func(bool) { g.onOptionToggle(vv) })
		g.tags = append(g.tags, ct)
		g.node.AddChild(ct.Node())
	}
}

// ValuesSet is the selected set.
func (g *CheckableTagGroup) ValuesSet() map[string]bool {
	out := map[string]bool{}
	if g == nil {
		return out
	}
	if g.multiple {
		if g.hasMulti {
			for _, v := range g.multi {
				out[v] = true
			}
			return out
		}
		for _, v := range g.defMulti {
			out[v] = true
		}
		return out
	}
	if g.hasSingle {
		if g.single != "" {
			out[g.single] = true
		}
		return out
	}
	if g.hasDefSingle && g.defSingle != "" {
		out[g.defSingle] = true
	}
	return out
}

func (g *CheckableTagGroup) onOptionToggle(v string) {
	if g == nil || g.disabled {
		return
	}
	for _, o := range g.options {
		vv := o.Value
		if vv == "" {
			vv = o.Label
		}
		if vv == v && o.Disabled {
			return
		}
	}
	if g.multiple {
		cur := map[string]bool{}
		for k := range g.ValuesSet() {
			cur[k] = true
		}
		if cur[v] {
			delete(cur, v)
		} else {
			cur[v] = true
		}
		next := make([]string, 0, len(cur))
		for _, o := range g.options {
			vv := o.Value
			if vv == "" {
				vv = o.Label
			}
			if cur[vv] {
				next = append(next, vv)
			}
		}
		g.multi = next
		g.hasMulti = true
		g.syncTags()
		if g.onChangeMulti != nil {
			g.onChangeMulti(append([]string(nil), next...))
		}
		if g.onChange != nil && len(next) > 0 {
			g.onChange(next[0])
		}
		return
	}
	g.single = v
	g.hasSingle = true
	g.syncTags()
	if g.onChange != nil {
		g.onChange(v)
	}
	if g.onChangeMulti != nil {
		g.onChangeMulti([]string{v})
	}
}

func (g *CheckableTagGroup) syncTags() {
	if g == nil {
		return
	}
	vals := g.ValuesSet()
	for i, ct := range g.tags {
		if i >= len(g.options) {
			continue
		}
		v := g.options[i].Value
		if v == "" {
			v = g.options[i].Label
		}
		want := vals[v]
		if ct.Checked() != want {
			ct.SetChecked(want)
		}
		dis := g.disabled || g.options[i].Disabled
		if ct.Disabled() != dis {
			ct.SetDisabled(dis)
		}
	}
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}

// SetMultiple toggles single/multi (layout).
func (g *CheckableTagGroup) SetMultiple(b bool) {
	if g == nil || g.multiple == b {
		return
	}
	g.multiple = b
	if g.node != nil {
		g.node.MarkNeedsLayout()
	}
}

// Multiple reports the mode.
func (g *CheckableTagGroup) Multiple() bool { return g != nil && g.multiple }

// SetValue sets controlled single selection.
func (g *CheckableTagGroup) SetValue(v string) {
	if g == nil {
		return
	}
	g.single = v
	g.hasSingle = true
	g.syncTags()
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}

// SetValues sets controlled multi selection.
func (g *CheckableTagGroup) SetValues(vs []string) {
	if g == nil {
		return
	}
	g.multi = append([]string(nil), vs...)
	g.hasMulti = true
	g.syncTags()
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}

// SetDefaultValue sets uncontrolled single initial.
func (g *CheckableTagGroup) SetDefaultValue(v string) {
	if g == nil {
		return
	}
	g.defSingle = v
	g.hasDefSingle = true
	if !g.hasSingle {
		g.syncTags()
	}
}

// SetDefaultValues sets uncontrolled multi initial.
func (g *CheckableTagGroup) SetDefaultValues(vs []string) {
	if g == nil {
		return
	}
	g.defMulti = append([]string(nil), vs...)
	if !g.hasMulti {
		g.syncTags()
	}
}

// Value returns single selection (first of multi).
func (g *CheckableTagGroup) Value() string {
	if g == nil {
		return ""
	}
	if g.multiple {
		vs := g.Values()
		if len(vs) > 0 {
			return vs[0]
		}
		return ""
	}
	if g.hasSingle {
		return g.single
	}
	if g.hasDefSingle {
		return g.defSingle
	}
	return ""
}

// Values returns selection in display order.
func (g *CheckableTagGroup) Values() []string {
	if g == nil {
		return nil
	}
	set := g.ValuesSet()
	out := []string{}
	for _, o := range g.options {
		v := o.Value
		if v == "" {
			v = o.Label
		}
		if set[v] {
			out = append(out, v)
		}
	}
	return out
}

// SetOnChange sets single-selection callback.
func (g *CheckableTagGroup) SetOnChange(fn func(string)) {
	if g == nil {
		return
	}
	g.onChange = fn
}

// SetOnChangeMulti sets multi-selection callback.
func (g *CheckableTagGroup) SetOnChangeMulti(fn func([]string)) {
	if g == nil {
		return
	}
	g.onChangeMulti = fn
}

// SetDisabled disables the whole group (paint).
func (g *CheckableTagGroup) SetDisabled(b bool) {
	if g == nil || g.disabled == b {
		return
	}
	g.disabled = b
	g.syncTags()
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}

// Disabled reports the flag.
func (g *CheckableTagGroup) Disabled() bool { return g != nil && g.disabled }

// SetAriaLabel names the group landmark.
func (g *CheckableTagGroup) SetAriaLabel(s string) {
	if g == nil {
		return
	}
	g.ariaLabel = s
	if g.node != nil {
		g.node.MarkNeedsPaint()
	}
}

// AriaLabel returns the name.
func (g *CheckableTagGroup) AriaLabel() string {
	if g == nil {
		return ""
	}
	return g.ariaLabel
}

// Role is group when named.
func (g *CheckableTagGroup) Role() string {
	if g != nil && g.ariaLabel != "" {
		return "group"
	}
	return ""
}

// Focusable is always false (options own focus).
func (g *CheckableTagGroup) Focusable() bool { return false }

// PressOption simulates clicking one option (disabled swallows).
func (g *CheckableTagGroup) PressOption(value string) {
	if g == nil || g.disabled {
		return
	}
	for i, o := range g.options {
		v := o.Value
		if v == "" {
			v = o.Label
		}
		if v == value {
			if o.Disabled {
				return
			}
			if i < len(g.tags) {
				g.tags[i].Toggle()
			} else {
				g.onOptionToggle(value)
			}
			return
		}
	}
}

// Tags returns the live option tags.
func (g *CheckableTagGroup) Tags() []*CheckableTag {
	if g == nil {
		return nil
	}
	return g.tags
}

// EffectiveGroupGap reads PaddingXS (8).
func (g *CheckableTagGroup) EffectiveGroupGap() float64 {
	var tok theme.Tokens
	if g != nil && g.node != nil {
		tok = theme.Default.Current()
	} else {
		tok = theme.Default.Current()
	}
	if tok.PaddingXS > 0 {
		return tok.PaddingXS
	}
	return DefaultTagGroupGap
}

// Node returns the tree node.
func (g *CheckableTagGroup) Node() rendering.RenderObject {
	if g == nil {
		return nil
	}
	return g.node
}

// Layout sizes the node.
func (g *CheckableTagGroup) Layout(c rendering.Constraints) rendering.Size {
	if g == nil || g.node == nil {
		return rendering.Size{}
	}
	return g.node.Layout(c)
}

type groupBox struct {
	*rendering.RenderBox
	group *CheckableTagGroup
}

func newGroupBox(g *CheckableTagGroup) *groupBox {
	b := &groupBox{RenderBox: rendering.NewRenderBox(), group: g}
	b.SetRepaintBoundary(true)
	b.SetRelayoutBoundary(true)
	return b
}

// Layout implements RenderObject (row wrap, gap=PaddingXS).
// Children are measured first; offsets are assigned after the inner box
// layout so the box Pad reset never survives.
func (b *groupBox) Layout(c rendering.Constraints) rendering.Size {
	if b == nil || b.group == nil || b.RenderBox == nil {
		return rendering.Size{}
	}
	if !b.ShouldRelayout(c) {
		return b.Size()
	}
	gap := b.group.EffectiveGroupGap()
	kids := b.Children()
	n := len(kids)
	sizes := make([]rendering.Size, n)
	inner := rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
	for i, ch := range kids {
		if rendering.ManualLayoutOf(ch) {
			sizes[i] = ch.Size()
			continue
		}
		sizes[i] = ch.Layout(inner)
	}
	limit := c.MaxWidth
	wrap := limit < rendering.Unbounded/2
	var x, y, lineH, contentW float64
	offs := make([]rendering.Point, n)
	for i, sz := range sizes {
		if wrap && x > 0 && x+sz.Width > limit+1e-9 {
			if x-gap > contentW {
				contentW = x - gap
			}
			x = 0
			y += lineH + gap
			lineH = 0
		}
		offs[i] = rendering.Point{X: x, Y: y}
		x += sz.Width + gap
		if sz.Height > lineH {
			lineH = sz.Height
		}
		if x-gap > contentW {
			contentW = x - gap
		}
	}
	contentH := y + lineH
	if n == 0 {
		contentW, contentH = 0, 0
	}
	b.FixedWidth, b.FixedHeight = contentW, contentH
	out := b.RenderBox.Layout(c)
	for i, ch := range kids {
		ch.SetOffset(offs[i])
	}
	return out
}
