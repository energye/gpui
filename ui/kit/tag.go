package kit

import (
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Tag tokens — components/tag/style prepareToken / prepareComponentToken.
// docs/antd/tag.md §6.2
const (
	// DefaultTagFontSize is tagFontSize (= fontSizeSM).
	DefaultTagFontSize = 12.0
	// DefaultTagLineHeight is content line box (~lineHeightSM * fontSizeSM).
	DefaultTagLineHeight = 20.0
	// DefaultTagHeight is the visual chip height (antd demos use 22).
	DefaultTagHeight = 22.0
	// DefaultTagPaddingInline is tagPaddingHorizontal(8) − lineWidth(1).
	DefaultTagPaddingInline = 7.0
	// DefaultTagPaddingBlock is vertical pad to reach ~22 with line box.
	DefaultTagPaddingBlock = 1.0
	// DefaultTagPaddingHorizontal is the fixed antd tagPaddingHorizontal.
	DefaultTagPaddingHorizontal = 8.0
	// DefaultTagIconSize is tagIconSize (~fontSizeIcon − 2*lineWidth).
	DefaultTagIconSize = 10.0
	// DefaultTagIconGap is icon↔label margin (≈ paddingInline).
	DefaultTagIconGap = 7.0
	// DefaultTagCloseGap is close-icon marginInlineStart (~paddingXXS − lineWidth).
	DefaultTagCloseGap = 3.0
	// DefaultTagRadius is borderRadiusSM.
	DefaultTagRadius = 4.0
	// DefaultTagLineWidth is lineWidth.
	DefaultTagLineWidth = 1.0
	// DefaultTagGroupGap is CheckableTagGroup gap (= paddingXS).
	DefaultTagGroupGap = 8.0
	// DefaultTagFocusOutset approximates focus-visible outset.
	DefaultTagFocusOutset = 1.5
	// DefaultTagCloseAria is the default accessible name for the close control.
	DefaultTagCloseAria = "Close"
	// DefaultTagSolidBG is colorBgSolid fallback when Theme has no solid token.
	DefaultTagSolidBG = "#000000E0"
	// DefaultTagDefaultBG is defaultBg fallback (fillTertiary on container).
	DefaultTagDefaultBG = "#FAFAFA"
)

// TagVariant is antd Tag variant: filled | solid | outlined.
type TagVariant int

const (
	// TagFilled is the default (shallow fill, no border).
	TagFilled TagVariant = iota
	// TagSolid is solid fill with inverse text.
	TagSolid
	// TagOutlined is bordered chip.
	TagOutlined
)

// TagCloseEvent is passed to OnClose; call PreventDefault to keep the tag visible.
type TagCloseEvent struct {
	prevented bool
}

// PreventDefault blocks the default hide behavior (antd e.preventDefault()).
func (e *TagCloseEvent) PreventDefault() {
	if e != nil {
		e.prevented = true
	}
}

// DefaultPrevented reports whether PreventDefault was called.
func (e *TagCloseEvent) DefaultPrevented() bool {
	return e != nil && e.prevented
}

// Tag is Ant Design Tag (data display chip).
//
//	[Pressable?]                 // OnClick
//	  └─ Decorated chrome        // Root
//	       └─ Row(Icon? · Label · ClosePressable?)
//
// Product contract: docs/antd/tag.md §6 (P0 DoD).
type Tag struct {
	Root     *primitive.Decorated
	press    *primitive.Pressable
	row      *primitive.Flex
	label    *primitive.Text
	closeBtn *primitive.Pressable
	iconNode core.Node

	// Value is the label text (antd children). Prefer SetLabel; Value kept for compat.
	Value string
	// Color is preset name, status, or #hex (empty = default skin).
	Color string
	// ColorRGBA is used when set via SetColorRGBA (custom solid).
	ColorRGBA render.RGBA
	colorRGBA bool

	Variant  TagVariant
	Closable bool
	// CloseIcon replaces the default "×" node when Closable.
	CloseIcon core.Node
	// Icon is a registry icon name (leading).
	Icon string
	// IconNode is a custom leading icon (preferred over Icon when non-nil).
	IconNode core.Node
	// IconSpin enables Icon.SetSpin (status=processing).
	IconSpin bool

	Disabled bool
	// Hidden is set after a successful close (antd display:none).
	Hidden bool
	// Bordered is the deprecated antd bordered flag. Only applied when
	// borderedSet and variant was not explicitly set via SetVariant.
	Bordered    bool
	borderedSet bool
	variantSet  bool

	OnClose   func(*TagCloseEvent)
	OnClick   func()
	AriaLabel string
	CloseAria string

	Face  text.Face
	Theme *core.Theme
	Style Style

	// resolved chrome (tests / L2)
	fontSize  float64
	padH      float64
	padV      float64
	radius    float64
	lineW     float64
	iconSz    float64
	bg        render.RGBA
	fg        render.RGBA
	bd        render.RGBA
	hasBorder bool
}

// # Lifecycle (#9, Button pattern)
//
// NewTag always builds once. After that:
//
//   - structureChange() — color/variant/icon/closable/layout rebuild
//   - chromeChange()    — currently aliases structureChange (chrome resolved in rebuild)
//   - ensureBuilt()     — Node/ChromeNode paths
//
// Root Decorated uses SkinType = kit.Tag (#6).
//
// NewTag creates a Tag with antd defaults (variant=filled, not closable).
func NewTag(label string) *Tag {
	t := &Tag{
		Value:    label,
		Variant:  TagFilled,
		Bordered: true, // historical default; variant filled still wins for border
	}
	t.rebuild()
	return t
}

// ensureBuilt materializes Decorated chrome if missing.
func (t *Tag) ensureBuilt() {
	if t == nil {
		return
	}
	if t.Root == nil {
		t.rebuild()
	}
}

// structureChange rebuilds chip content (icons, close, colors, layout).
func (t *Tag) structureChange() {
	if t == nil {
		return
	}
	t.rebuild()
}

// chromeChange refreshes token/style chrome. Tag resolves colors inside rebuild,
// so this aliases structureChange (symmetric with Button's chrome path intent).
func (t *Tag) chromeChange() {
	if t == nil {
		return
	}
	t.ensureBuilt()
	t.structureChange()
}

// Node returns the mount root (Pressable when clickable, else Decorated).
func (t *Tag) Node() core.Node {
	if t == nil {
		return nil
	}
	t.ensureBuilt()
	if t.Hidden {
		return t.Root
	}
	if t.press != nil && t.OnClick != nil && !t.Disabled {
		return t.press
	}
	return t.Root
}

// ChromeNode returns the Decorated chrome (tests / composition).
func (t *Tag) ChromeNode() core.Node {
	if t == nil {
		return nil
	}
	t.ensureBuilt()
	return t.Root
}

// CloseNode returns the close Pressable when closable, else nil.
func (t *Tag) CloseNode() core.Node {
	if t == nil {
		return nil
	}
	return t.closeBtn
}

// Visible reports whether the tag is shown (not Hidden).
func (t *Tag) Visible() bool {
	return t != nil && !t.Hidden
}

// FontSize returns resolved label font size (L2).
func (t *Tag) FontSize() float64 {
	t.resolveMetrics()
	return t.fontSize
}

// PadH returns resolved horizontal padding (L2).
func (t *Tag) PadH() float64 {
	t.resolveMetrics()
	return t.padH
}

// Radius returns resolved corner radius (L2).
func (t *Tag) Radius() float64 {
	t.resolveMetrics()
	return t.radius
}

// Background returns resolved fill (L2).
func (t *Tag) Background() render.RGBA {
	t.resolveChrome()
	return t.bg
}

// Foreground returns resolved text color (L2).
func (t *Tag) Foreground() render.RGBA {
	t.resolveChrome()
	return t.fg
}

// HasBorder reports whether a border is drawn (L2).
func (t *Tag) HasBorder() bool {
	t.resolveChrome()
	return t.hasBorder
}

// SetLabel updates the tag text.
func (t *Tag) SetLabel(s string) {
	if t == nil {
		return
	}
	t.Value = s
	if t.label != nil {
		t.label.Value = s
		t.label.MarkNeedsLayout()
		t.label.MarkNeedsPaint()
	}
	t.applyA11y()
}

// SetValue is an alias for SetLabel (compat with older kit callers).
func (t *Tag) SetValue(s string) { t.SetLabel(s) }

// SetColor sets antd color string: preset | status | #hex | "" (default skin).
func (t *Tag) SetColor(nameOrHex string) {
	if t == nil {
		return
	}
	t.Color = strings.TrimSpace(nameOrHex)
	t.colorRGBA = false
	t.ColorRGBA = render.RGBA{}
	t.structureChange()
}

// SetColorRGBA sets a custom solid color (clears named Color).
func (t *Tag) SetColorRGBA(c render.RGBA) {
	if t == nil {
		return
	}
	t.ColorRGBA = c
	t.colorRGBA = c.A > 0
	if t.colorRGBA {
		t.Color = ""
	}
	t.structureChange()
}

// SetVariant sets filled | solid | outlined.
func (t *Tag) SetVariant(v TagVariant) {
	if t == nil {
		return
	}
	t.Variant = v
	t.variantSet = true
	t.structureChange()
}

// SetBordered is the deprecated antd bordered flag.
// false → filled (no border); true → outlined when variant not explicitly set.
func (t *Tag) SetBordered(on bool) {
	if t == nil {
		return
	}
	t.Bordered = on
	t.borderedSet = true
	if !t.variantSet {
		if on {
			t.Variant = TagOutlined
		} else {
			t.Variant = TagFilled
		}
	}
	t.structureChange()
}

// SetClosable toggles the close control.
func (t *Tag) SetClosable(on bool) {
	if t == nil {
		return
	}
	t.Closable = on
	t.structureChange()
}

// SetCloseIcon sets a custom close node (nil → default "×" when Closable).
func (t *Tag) SetCloseIcon(n core.Node) {
	if t == nil {
		return
	}
	t.CloseIcon = n
	t.structureChange()
}

// SetIcon sets a leading registry icon name.
func (t *Tag) SetIcon(name string) {
	if t == nil {
		return
	}
	t.Icon = name
	t.structureChange()
}

// SetIconNode sets a custom leading icon node.
func (t *Tag) SetIconNode(n core.Node) {
	if t == nil {
		return
	}
	t.IconNode = n
	t.structureChange()
}

// SetIconSpin enables continuous spin on the leading Icon (processing status).
func (t *Tag) SetIconSpin(on bool) {
	if t == nil {
		return
	}
	t.IconSpin = on
	t.structureChange()
}

// SetDisabled dims chrome and blocks close/click.
func (t *Tag) SetDisabled(v bool) {
	if t == nil {
		return
	}
	t.Disabled = v
	t.structureChange()
}

// SetOnClick enables whole-tag click (New Tag / link demos).
func (t *Tag) SetOnClick(fn func()) {
	if t == nil {
		return
	}
	t.OnClick = fn
	t.structureChange()
}

// SetOnClose sets the close callback (simple form; always allows hide unless
// the event PreventDefault is used via OnClose field directly).
func (t *Tag) SetOnClose(fn func()) {
	if t == nil {
		return
	}
	if fn == nil {
		t.OnClose = nil
		return
	}
	t.OnClose = func(*TagCloseEvent) { fn() }
}

// SetHidden forces visibility (tests / parent recovery).
func (t *Tag) SetHidden(v bool) {
	if t == nil {
		return
	}
	t.Hidden = v
	t.structureChange()
}

// SetTheme sets the theme override.
func (t *Tag) SetTheme(th *core.Theme) {
	if t == nil {
		return
	}
	t.Theme = th
	t.structureChange()
}

// SetFace sets the label face.
func (t *Tag) SetFace(face text.Face) {
	if t == nil {
		return
	}
	t.Face = face
	if t.label != nil {
		t.label.Face = face
		t.label.MarkNeedsPaint()
	}
}

// SetStyle applies optional Style overrides.
func (t *Tag) SetStyle(st Style) {
	if t == nil {
		return
	}
	t.Style = st
	if st.Face != nil {
		t.Face = st.Face
	}
	t.structureChange()
}

// SetAriaLabel sets the accessible name override.
func (t *Tag) SetAriaLabel(name string) {
	if t == nil {
		return
	}
	t.AriaLabel = name
	t.applyA11y()
}

func (t *Tag) theme() *core.Theme {
	var n core.Node
	if t.Root != nil {
		n = t.Root
	}
	return themeOf(t.Theme, n)
}

func (t *Tag) resolveMetrics() {
	if t == nil {
		return
	}
	th := t.theme()
	t.fontSize = th.SizeOr(core.TokenFontSizeSM, DefaultTagFontSize)
	if t.Style.FontSize > 0 {
		t.fontSize = t.Style.FontSize
	}
	t.lineW = th.SizeOr(core.TokenLineWidth, DefaultTagLineWidth)
	t.radius = th.SizeOr(core.TokenBorderRadiusSM, DefaultTagRadius)
	if t.Style.hasRadius() {
		t.radius = t.Style.Radius
	}
	t.padH = DefaultTagPaddingHorizontal - t.lineW
	if t.padH < 0 {
		t.padH = DefaultTagPaddingInline
	}
	t.padV = DefaultTagPaddingBlock
	t.iconSz = DefaultTagIconSize
	if t.iconSz < t.fontSize-2 {
		t.iconSz = t.fontSize - 2
	}
}

func (t *Tag) resolveVariant() TagVariant {
	if t.variantSet {
		return t.Variant
	}
	if t.borderedSet {
		if t.Bordered {
			return TagOutlined
		}
		return TagFilled
	}
	return t.Variant
}

func (t *Tag) resolveChrome() {
	if t == nil {
		return
	}
	t.resolveMetrics()
	th := t.theme()
	v := t.resolveVariant()

	defBG := th.Color(core.TokenColorDisabledBg)
	if defBG.A == 0 {
		defBG = render.Hex(DefaultTagDefaultBG)
	}
	defFG := th.Color(core.TokenColorText)
	defBD := th.Color(core.TokenColorBorder)
	inv := th.Color(core.TokenColorTextInverse)
	if inv.A == 0 {
		inv = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	solidBG := render.Hex(DefaultTagSolidBG)

	// color source
	var (
		preset    *tagPalette
		custom    render.RGBA
		hasCustom bool
	)
	if t.colorRGBA && t.ColorRGBA.A > 0 {
		custom = t.ColorRGBA
		hasCustom = true
	} else if c := strings.TrimSpace(t.Color); c != "" {
		if p := tagPreset(c); p != nil {
			preset = p
		} else if isHexColor(c) {
			custom = render.Hex(c)
			hasCustom = custom.A > 0
		} else if st := tagStatusPalette(c, th); st != nil {
			preset = st
		}
	}

	t.hasBorder = false
	switch {
	case t.Disabled:
		t.bg = th.Color(core.TokenColorDisabledBg)
		t.fg = th.Color(core.TokenColorDisabledText)
		t.bd = defBD
		if v == TagOutlined {
			t.hasBorder = true
		}
	case preset != nil:
		switch v {
		case TagSolid:
			t.bg = preset.dark
			t.fg = inv
			t.bd = preset.dark
			t.hasBorder = false
		case TagOutlined:
			t.bg = preset.light
			t.fg = preset.text
			t.bd = preset.border
			t.hasBorder = true
		default: // filled
			t.bg = preset.light
			t.fg = preset.text
			t.bd = render.RGBA{}
			t.hasBorder = false
		}
	case hasCustom:
		switch v {
		case TagSolid:
			t.bg = custom
			t.fg = inv
			t.hasBorder = false
		case TagOutlined:
			t.bg = lighten(custom, 0.85)
			t.fg = custom
			t.bd = custom
			t.hasBorder = true
		default:
			t.bg = lighten(custom, 0.85)
			t.fg = custom
			t.hasBorder = false
		}
	default:
		switch v {
		case TagSolid:
			t.bg = solidBG
			t.fg = inv
			t.hasBorder = false
		case TagOutlined:
			t.bg = th.Color(core.TokenColorBgContainer)
			if t.bg.A == 0 {
				t.bg = defBG
			}
			t.fg = defFG
			t.bd = defBD
			t.hasBorder = true
		default: // filled
			t.bg = defBG
			t.fg = defFG
			t.hasBorder = false
		}
	}

	if t.Style.hasBG() {
		t.bg = t.Style.Background
	}
	if t.Style.hasText() {
		t.fg = t.Style.Text
	}
	if t.Style.hasBorder() {
		t.bd = t.Style.Border
		t.hasBorder = true
	}
}

func (t *Tag) rebuild() {
	if t == nil {
		return
	}
	t.resolveChrome()
	th := t.theme()

	// label
	t.label = primitive.NewText(t.Value)
	t.label.FontSize = t.fontSize
	t.label.Face = t.Face
	t.label.Color = t.fg
	t.label.MaxWidth = 200
	t.label.Ellipsis = true

	// row content
	t.row = primitive.Row()
	t.row.CrossAlign = core.CrossCenter
	t.row.Gap = 0

	// leading icon
	t.iconNode = nil
	if t.IconNode != nil {
		t.iconNode = t.IconNode
		t.row.AddChild(t.IconNode)
		t.row.Gap = DefaultTagIconGap
	} else if t.Icon != "" {
		ic := NewIcon(t.Icon)
		ic.SetSize(t.iconSz)
		ic.SetColor(t.fg)
		ic.SetTheme(th)
		if t.IconSpin {
			ic.SetSpin(true)
		}
		t.iconNode = ic.Node()
		t.row.AddChild(t.iconNode)
		t.row.Gap = DefaultTagIconGap
	}
	t.row.AddChild(t.label)

	// close
	t.closeBtn = nil
	if t.Closable && !t.Hidden {
		var closeChild core.Node
		if t.CloseIcon != nil {
			closeChild = t.CloseIcon
		} else {
			x := primitive.NewText("×")
			x.FontSize = t.fontSize
			x.Face = t.Face
			x.Color = t.fg
			if t.Disabled {
				x.Color = th.Color(core.TokenColorDisabledText)
			}
			closeChild = x
		}
		cp := primitive.NewPressable(closeChild)
		cp.Focusable = !t.Disabled
		cp.ShowFocusRing = true
		cp.FocusRingRadius = t.radius
		cp.FocusRingOutset = DefaultTagFocusOutset
		cp.EnableRipple = false
		cp.Padding = primitive.EdgeInsets{Left: DefaultTagCloseGap}
		cp.SetDisabled(t.Disabled)
		cp.Click = func() {
			if t.Disabled {
				return
			}
			t.handleClose()
		}
		aria := t.CloseAria
		if aria == "" {
			aria = DefaultTagCloseAria
		}
		cp.Base().Role = "button"
		cp.Base().Label = aria
		t.closeBtn = cp
		t.row.AddChild(cp)
	}

	// chrome
	if t.Root == nil {
		t.Root = primitive.NewDecorated(t.row)
	} else {
		t.Root.ClearChildren()
		t.Root.AddChild(t.row)
	}
	// Product skin key so Theme.Skin can override Tag chrome only (#6).
	t.Root.SkinType = TypeTag
	t.Root.Padding = primitive.EdgeInsets{
		Left: t.padH, Right: t.padH,
		Top: t.padV, Bottom: t.padV,
	}
	t.Root.Radius = t.radius
	t.Root.Background = t.bg
	t.Root.MinHeight = DefaultTagHeight
	t.Root.SetCenterContent(true)
	if t.hasBorder {
		t.Root.BorderWidth = t.lineW
		t.Root.BorderColor = t.bd
	} else {
		t.Root.BorderWidth = 0
		t.Root.BorderColor = render.RGBA{}
	}
	if t.Hidden {
		t.Root.Hit = core.HitTransparent
		// collapse: clear children so layout size shrinks
		t.Root.ClearChildren()
	} else {
		t.Root.Hit = core.HitDefer
	}

	// optional whole-tag press
	t.press = nil
	if t.OnClick != nil && !t.Hidden {
		p := primitive.NewPressable(t.Root)
		p.Focusable = !t.Disabled
		p.ShowFocusRing = true
		p.FocusRingRadius = t.radius
		p.FocusRingOutset = DefaultTagFocusOutset
		p.EnableRipple = false
		p.SetDisabled(t.Disabled)
		p.Click = func() {
			if t.Disabled || t.OnClick == nil {
				return
			}
			t.OnClick()
		}
		t.press = p
	}

	t.applyA11y()
	if t.Root != nil {
		t.Root.SetThemeHook(func(*core.Theme) { t.structureChange() })
		t.Root.MarkNeedsLayout()
		t.Root.MarkNeedsPaint()
	}
}

func (t *Tag) handleClose() {
	e := &TagCloseEvent{}
	if t.OnClose != nil {
		t.OnClose(e)
	}
	if e.DefaultPrevented() {
		return
	}
	t.Hidden = true
	t.structureChange()
}

func (t *Tag) applyA11y() {
	if t == nil || t.Root == nil {
		return
	}
	name := t.AriaLabel
	if name == "" {
		name = t.Value
	}
	if name != "" {
		t.Root.Base().Label = name
	}
}

// ---------- CheckableTag ----------

// CheckableTag is Ant Design Tag.CheckableTag.
type CheckableTag struct {
	Root  *primitive.Pressable
	chip  *primitive.Decorated
	row   *primitive.Flex
	label *primitive.Text

	Value string // label text
	// Checked is the current selection (controlled when controlled=true).
	Checked        bool
	controlled     bool
	defaultSet     bool
	DefaultChecked bool

	Icon     string
	IconNode core.Node
	Disabled bool

	OnChange  func(checked bool)
	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style
}

// NewCheckableTag creates an unchecked CheckableTag.
func NewCheckableTag(label string) *CheckableTag {
	c := &CheckableTag{Value: label}
	c.rebuild()
	return c
}

// Node returns the Pressable root.
func (c *CheckableTag) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// ChromeNode returns the Decorated chip.
func (c *CheckableTag) ChromeNode() core.Node {
	if c == nil {
		return nil
	}
	if c.chip == nil {
		c.rebuild()
	}
	return c.chip
}

// IsChecked returns the current checked state.
func (c *CheckableTag) IsChecked() bool {
	if c == nil {
		return false
	}
	return c.Checked
}

// SetLabel updates text.
func (c *CheckableTag) SetLabel(s string) {
	if c == nil {
		return
	}
	c.Value = s
	if c.label != nil {
		c.label.Value = s
		c.label.MarkNeedsLayout()
		c.label.MarkNeedsPaint()
	}
}

// SetChecked sets checked (controlled).
func (c *CheckableTag) SetChecked(v bool) {
	if c == nil {
		return
	}
	c.Checked = v
	c.controlled = true
	c.applyChrome()
}

// SetDefaultChecked sets the uncontrolled initial checked state.
func (c *CheckableTag) SetDefaultChecked(v bool) {
	if c == nil {
		return
	}
	if c.controlled {
		return
	}
	c.DefaultChecked = v
	c.defaultSet = true
	c.Checked = v
	c.applyChrome()
}

// SetOnChange sets the change callback.
func (c *CheckableTag) SetOnChange(fn func(bool)) {
	if c == nil {
		return
	}
	c.OnChange = fn
}

// SetIcon sets leading icon name.
func (c *CheckableTag) SetIcon(name string) {
	if c == nil {
		return
	}
	c.Icon = name
	c.rebuild()
}

// SetIconNode sets custom leading icon.
func (c *CheckableTag) SetIconNode(n core.Node) {
	if c == nil {
		return
	}
	c.IconNode = n
	c.rebuild()
}

// SetDisabled toggles disabled.
func (c *CheckableTag) SetDisabled(v bool) {
	if c == nil {
		return
	}
	c.Disabled = v
	c.rebuild()
}

// SetFace sets font face.
func (c *CheckableTag) SetFace(face text.Face) {
	if c == nil {
		return
	}
	c.Face = face
	if c.label != nil {
		c.label.Face = face
	}
}

// SetTheme sets theme override.
func (c *CheckableTag) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.rebuild()
}

// SetAriaLabel sets accessible name.
func (c *CheckableTag) SetAriaLabel(name string) {
	if c == nil {
		return
	}
	c.AriaLabel = name
	if c.Root != nil {
		c.Root.Base().Label = name
	}
}

func (c *CheckableTag) theme() *core.Theme {
	var n core.Node
	if c.Root != nil {
		n = c.Root
	}
	return themeOf(c.Theme, n)
}

func (c *CheckableTag) rebuild() {
	if c == nil {
		return
	}
	th := c.theme()
	font := th.SizeOr(core.TokenFontSizeSM, DefaultTagFontSize)
	radius := th.SizeOr(core.TokenBorderRadiusSM, DefaultTagRadius)
	padH := DefaultTagPaddingInline
	padV := DefaultTagPaddingBlock
	iconSz := DefaultTagIconSize

	c.label = primitive.NewText(c.Value)
	c.label.FontSize = font
	c.label.Face = c.Face

	c.row = primitive.Row()
	c.row.CrossAlign = core.CrossCenter
	c.row.Gap = 0
	if c.IconNode != nil {
		c.row.AddChild(c.IconNode)
		c.row.Gap = DefaultTagIconGap
	} else if c.Icon != "" {
		ic := NewIcon(c.Icon)
		ic.SetSize(iconSz)
		ic.SetTheme(th)
		c.row.AddChild(ic.Node())
		c.row.Gap = DefaultTagIconGap
	}
	c.row.AddChild(c.label)

	c.chip = primitive.NewDecorated(c.row)
	c.chip.Padding = primitive.EdgeInsets{Left: padH, Right: padH, Top: padV, Bottom: padV}
	c.chip.Radius = radius
	c.chip.BorderWidth = 0
	c.chip.MinHeight = DefaultTagHeight
	c.chip.SetCenterContent(true)
	c.chip.Hit = core.HitDefer

	c.Root = primitive.NewPressable(c.chip)
	c.Root.Focusable = !c.Disabled
	c.Root.ShowFocusRing = true
	c.Root.FocusRingRadius = radius
	c.Root.FocusRingOutset = DefaultTagFocusOutset
	c.Root.EnableRipple = false
	c.Root.SetDisabled(c.Disabled)
	c.Root.Click = func() { c.toggle() }
	name := c.AriaLabel
	if name == "" {
		name = c.Value
	}
	c.Root.Base().Role = "checkbox"
	c.Root.Base().Label = name

	c.applyChrome()
}

func (c *CheckableTag) toggle() {
	if c == nil || c.Disabled {
		return
	}
	next := !c.Checked
	if !c.controlled {
		c.Checked = next
		c.applyChrome()
	}
	if c.OnChange != nil {
		c.OnChange(next)
	}
	// controlled: parent should SetChecked
	if c.controlled {
		// still reflect if parent already synced
		c.applyChrome()
	}
}

func (c *CheckableTag) applyChrome() {
	if c == nil || c.chip == nil {
		return
	}
	th := c.theme()
	primary := th.Color(core.TokenColorPrimary)
	primaryHover := th.Color(core.TokenColorPrimaryHover)
	inv := th.Color(core.TokenColorTextInverse)
	if inv.A == 0 {
		inv = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	fillSec := th.Color(core.TokenColorFillSecondary)
	textCol := th.Color(core.TokenColorText)
	disBG := th.Color(core.TokenColorDisabledBg)
	disFG := th.Color(core.TokenColorDisabledText)

	var bg, fg render.RGBA
	switch {
	case c.Disabled && c.Checked:
		bg, fg = disBG, disFG
	case c.Disabled:
		bg, fg = render.RGBA{}, disFG
	case c.Checked:
		bg, fg = primary, inv
	default:
		bg, fg = render.RGBA{}, textCol
	}
	c.chip.Background = bg
	c.chip.BorderWidth = 0
	if c.label != nil {
		c.label.Color = fg
		c.label.MarkNeedsPaint()
	}
	// hover colors on pressable (unchecked only)
	if c.Root != nil {
		if !c.Disabled && !c.Checked {
			c.Root.ColorHovered = fillSec
			c.Root.Color = render.RGBA{}
		} else if !c.Disabled && c.Checked {
			c.Root.ColorHovered = primaryHover
			c.Root.Color = primary
		} else {
			c.Root.Color = render.RGBA{}
			c.Root.ColorHovered = render.RGBA{}
		}
		c.Root.SetDisabled(c.Disabled)
		c.Root.MarkNeedsPaint()
	}
	c.chip.MarkNeedsPaint()
}

// ---------- CheckableTagGroup ----------

// TagOption is one CheckableTagGroup options[] entry.
type TagOption struct {
	Label    string
	Value    string // empty → Label
	Icon     string
	IconNode core.Node
	Disabled bool
}

// CheckableTagGroup is Ant Design Tag.CheckableTagGroup.
type CheckableTagGroup struct {
	Root *primitive.Flex
	tags []*CheckableTag

	Options  []TagOption
	Multiple bool
	Disabled bool

	// value (single) / values (multiple)
	value      string
	values     []string
	controlled bool

	OnChange      func(value string)
	OnChangeMulti func(values []string)

	Face  text.Face
	Theme *core.Theme
}

// NewCheckableTagGroup creates a group from options (string labels ok via TagOption).
func NewCheckableTagGroup(opts ...TagOption) *CheckableTagGroup {
	g := &CheckableTagGroup{
		Options: append([]TagOption(nil), opts...),
	}
	g.rebuild()
	return g
}

// NewCheckableTagGroupStrings is a sugar for string options.
func NewCheckableTagGroupStrings(labels ...string) *CheckableTagGroup {
	opts := make([]TagOption, len(labels))
	for i, s := range labels {
		opts[i] = TagOption{Label: s, Value: s}
	}
	return NewCheckableTagGroup(opts...)
}

// Node returns the flex group root.
func (g *CheckableTagGroup) Node() core.Node {
	if g == nil {
		return nil
	}
	if g.Root == nil {
		g.rebuild()
	}
	return g.Root
}

// Value returns the single-select value.
func (g *CheckableTagGroup) Value() string {
	if g == nil {
		return ""
	}
	return g.value
}

// Values returns a copy of multi-select values.
func (g *CheckableTagGroup) Values() []string {
	if g == nil {
		return nil
	}
	return append([]string(nil), g.values...)
}

// SetOptions replaces options and rebuilds.
func (g *CheckableTagGroup) SetOptions(opts ...TagOption) {
	if g == nil {
		return
	}
	g.Options = append([]TagOption(nil), opts...)
	g.rebuild()
}

// SetMultiple toggles multi-select mode.
func (g *CheckableTagGroup) SetMultiple(on bool) {
	if g == nil {
		return
	}
	g.Multiple = on
	g.rebuild()
}

// SetValue sets single-select controlled value.
func (g *CheckableTagGroup) SetValue(v string) {
	if g == nil {
		return
	}
	g.value = v
	g.controlled = true
	g.syncChecks()
}

// SetValues sets multi-select controlled values.
func (g *CheckableTagGroup) SetValues(vs []string) {
	if g == nil {
		return
	}
	g.values = append([]string(nil), vs...)
	g.controlled = true
	g.Multiple = true
	g.syncChecks()
}

// SetDefaultValue sets uncontrolled single initial value.
func (g *CheckableTagGroup) SetDefaultValue(v string) {
	if g == nil || g.controlled {
		return
	}
	g.value = v
	g.syncChecks()
}

// SetDefaultValues sets uncontrolled multi initial values.
func (g *CheckableTagGroup) SetDefaultValues(vs []string) {
	if g == nil || g.controlled {
		return
	}
	g.values = append([]string(nil), vs...)
	g.Multiple = true
	g.syncChecks()
}

// SetOnChange sets single-select change handler.
func (g *CheckableTagGroup) SetOnChange(fn func(string)) {
	if g == nil {
		return
	}
	g.OnChange = fn
}

// SetOnChangeMulti sets multi-select change handler.
func (g *CheckableTagGroup) SetOnChangeMulti(fn func([]string)) {
	if g == nil {
		return
	}
	g.OnChangeMulti = fn
}

// SetDisabled disables the whole group.
func (g *CheckableTagGroup) SetDisabled(v bool) {
	if g == nil {
		return
	}
	g.Disabled = v
	g.rebuild()
}

// SetFace propagates face to children.
func (g *CheckableTagGroup) SetFace(face text.Face) {
	if g == nil {
		return
	}
	g.Face = face
	for _, tg := range g.tags {
		if tg != nil {
			tg.SetFace(face)
		}
	}
}

// SetTheme sets theme override.
func (g *CheckableTagGroup) SetTheme(th *core.Theme) {
	if g == nil {
		return
	}
	g.Theme = th
	g.rebuild()
}

// OptionTags returns child CheckableTag pointers (tests).
func (g *CheckableTagGroup) OptionTags() []*CheckableTag {
	if g == nil {
		return nil
	}
	return g.tags
}

func (g *CheckableTagGroup) rebuild() {
	if g == nil {
		return
	}
	th := themeOf(g.Theme, nil)
	gap := th.SizeOr(core.TokenPaddingXS, DefaultTagGroupGap)

	g.Root = primitive.Row()
	g.Root.Gap = gap
	g.Root.CrossAlign = core.CrossCenter
	g.Root.MainAlign = core.MainStart
	// wrap-like: use flex wrap if available
	g.Root.Wrap = true

	g.tags = make([]*CheckableTag, 0, len(g.Options))
	for i, opt := range g.Options {
		lab := opt.Label
		if lab == "" {
			lab = opt.Value
		}
		val := opt.Value
		if val == "" {
			val = lab
		}
		ct := NewCheckableTag(lab)
		ct.Theme = g.Theme
		ct.SetFace(g.Face)
		if opt.IconNode != nil {
			ct.SetIconNode(opt.IconNode)
		} else if opt.Icon != "" {
			ct.SetIcon(opt.Icon)
		}
		ct.SetDisabled(g.Disabled || opt.Disabled)
		idx := i
		v := val
		ct.SetOnChange(func(on bool) {
			g.handleToggle(idx, v, on)
		})
		// mark controlled children so they don't flip without parent
		ct.controlled = true
		g.tags = append(g.tags, ct)
		g.Root.AddChild(ct.Node())
	}
	g.syncChecks()
	if g.Root != nil {
		g.Root.MarkNeedsLayout()
		g.Root.MarkNeedsPaint()
	}
}

func (g *CheckableTagGroup) handleToggle(index int, val string, on bool) {
	if g == nil || g.Disabled {
		return
	}
	if g.Multiple {
		next := append([]string(nil), g.values...)
		if on {
			if !containsStr(next, val) {
				next = append(next, val)
			}
		} else {
			next = removeStr(next, val)
		}
		if !g.controlled {
			g.values = next
			g.syncChecks()
		}
		if g.OnChangeMulti != nil {
			g.OnChangeMulti(append([]string(nil), next...))
		}
		// also fire OnChange with joined? keep multi-only
		return
	}
	// single: clicking selected keeps it selected (radio-like); antd sets the value
	var next string
	if on {
		next = val
	} else {
		// re-click selected → stay selected (CheckableTagGroup single typically selects)
		next = val
	}
	if !g.controlled {
		g.value = next
		g.syncChecks()
	}
	if g.OnChange != nil {
		g.OnChange(next)
	}
	_ = index
}

func (g *CheckableTagGroup) syncChecks() {
	if g == nil {
		return
	}
	for i, ct := range g.tags {
		if ct == nil || i >= len(g.Options) {
			continue
		}
		val := g.Options[i].Value
		if val == "" {
			val = g.Options[i].Label
		}
		var on bool
		if g.Multiple {
			on = containsStr(g.values, val)
		} else {
			on = g.value == val
		}
		ct.Checked = on
		ct.applyChrome()
	}
}

// ---------- palette helpers ----------

type tagPalette struct {
	text, light, border, dark render.RGBA
}

// antd PresetColors default seed (light/border/dark/text).
func tagPreset(name string) *tagPalette {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "magenta":
		return &tagPalette{text: render.Hex("#C41D7F"), light: render.Hex("#FFF0F6"), border: render.Hex("#FFADD2"), dark: render.Hex("#EB2F96")}
	case "red":
		return &tagPalette{text: render.Hex("#CF1322"), light: render.Hex("#FFF1F0"), border: render.Hex("#FFA39E"), dark: render.Hex("#F5222D")}
	case "volcano":
		return &tagPalette{text: render.Hex("#D4380D"), light: render.Hex("#FFF2E8"), border: render.Hex("#FFBB96"), dark: render.Hex("#FA541C")}
	case "orange":
		return &tagPalette{text: render.Hex("#D46B08"), light: render.Hex("#FFF7E6"), border: render.Hex("#FFD591"), dark: render.Hex("#FA8C16")}
	case "gold":
		return &tagPalette{text: render.Hex("#D48806"), light: render.Hex("#FFFBE6"), border: render.Hex("#FFE58F"), dark: render.Hex("#FAAD14")}
	case "lime":
		return &tagPalette{text: render.Hex("#7CB305"), light: render.Hex("#FCFFE6"), border: render.Hex("#EAFF8F"), dark: render.Hex("#A0D911")}
	case "green":
		return &tagPalette{text: render.Hex("#389E0D"), light: render.Hex("#F6FFED"), border: render.Hex("#B7EB8F"), dark: render.Hex("#52C41A")}
	case "cyan":
		return &tagPalette{text: render.Hex("#08979C"), light: render.Hex("#E6FFFB"), border: render.Hex("#87E8DE"), dark: render.Hex("#13C2C2")}
	case "blue":
		return &tagPalette{text: render.Hex("#0958D9"), light: render.Hex("#E6F4FF"), border: render.Hex("#91CAFF"), dark: render.Hex("#1677FF")}
	case "geekblue":
		return &tagPalette{text: render.Hex("#1D39C4"), light: render.Hex("#F0F5FF"), border: render.Hex("#ADC6FF"), dark: render.Hex("#2F54EB")}
	case "purple":
		return &tagPalette{text: render.Hex("#531DAB"), light: render.Hex("#F9F0FF"), border: render.Hex("#D3ADF7"), dark: render.Hex("#722ED1")}
	default:
		return nil
	}
}

func tagStatusPalette(name string, th *core.Theme) *tagPalette {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "success":
		c := th.Color(core.TokenColorSuccess)
		if c.A == 0 {
			c = render.Hex("#52C41A")
		}
		return &tagPalette{text: c, light: lighten(c, 0.9), border: lighten(c, 0.55), dark: c}
	case "error":
		c := th.Color(core.TokenColorError)
		if c.A == 0 {
			c = render.Hex("#FF4D4F")
		}
		return &tagPalette{text: c, light: lighten(c, 0.9), border: lighten(c, 0.55), dark: c}
	case "warning":
		c := th.Color(core.TokenColorWarning)
		if c.A == 0 {
			c = render.Hex("#FAAD14")
		}
		return &tagPalette{text: c, light: lighten(c, 0.9), border: lighten(c, 0.55), dark: c}
	case "processing":
		c := th.Color(core.TokenColorPrimary)
		if c.A == 0 {
			c = render.Hex("#1677FF")
		}
		bg := th.Color(core.TokenColorPrimaryBg)
		if bg.A == 0 {
			bg = lighten(c, 0.9)
		}
		bd := th.Color(core.TokenColorPrimaryBorder)
		if bd.A == 0 {
			bd = lighten(c, 0.55)
		}
		return &tagPalette{text: c, light: bg, border: bd, dark: c}
	case "default":
		// neutral status: use default text + fill
		return &tagPalette{
			text:   th.Color(core.TokenColorText),
			light:  th.Color(core.TokenColorDisabledBg),
			border: th.Color(core.TokenColorBorder),
			dark:   render.Hex(DefaultTagSolidBG),
		}
	default:
		return nil
	}
}

func isHexColor(s string) bool {
	if s == "" {
		return false
	}
	if s[0] != '#' {
		return false
	}
	n := len(s) - 1
	return n == 3 || n == 4 || n == 6 || n == 8
}

// lighten blends c toward white by t∈[0,1].
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
		A: 1,
	}
}

func containsStr(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}

func removeStr(ss []string, v string) []string {
	var out []string
	for _, s := range ss {
		if s != v {
			out = append(out, s)
		}
	}
	return out
}
