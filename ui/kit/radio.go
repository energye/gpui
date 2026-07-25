package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Radio defaults — components/radio/style prepareComponentToken.
// docs/antd/radio.md §6.2 / §6.10
const (
	DefaultRadioIndicator = 16.0 // radioSize (= fontSizeLG)
	DefaultRadioDot       = 6.0  // inner disc (radioSize - (dotPadding+lineWidth)*2)
	DefaultRadioGap       = 8.0  // marginXS — indicator↔label & Group gap
	DefaultRadioLineWidth = 1.0
	DefaultRadioBtnPadH   = 15.0 // buttonPaddingInline ≈ 15
)

// RadioSize is antd size for button-style radios only (small|medium|large).
// https://ant.design/components/radio — size 只对按钮样式生效
type RadioSize int

const (
	RadioMiddle RadioSize = iota // medium / controlHeight 32
	RadioSmall                   // controlHeightSM 24
	RadioLarge                   // controlHeightLG 40
)

// RadioOptionType is antd optionType: default | button.
type RadioOptionType int

const (
	RadioOptionDefault RadioOptionType = iota
	RadioOptionButton
)

// RadioButtonStyle is antd buttonStyle: outline | solid.
type RadioButtonStyle int

const (
	RadioButtonOutline RadioButtonStyle = iota
	RadioButtonSolid
)

// RadioOrientation is antd orientation: horizontal | vertical.
type RadioOrientation int

const (
	RadioHorizontal RadioOrientation = iota
	RadioVertical
)

// Radio is a single radio option (antd Radio / Radio.Button).
//
//	Pressable (role=radio)
//	  └─ default: Row(indicator 16×16, label)
//	  └─ button:  Decorated chrome + label
//
// Product contract: docs/antd/radio.md §6 (P0 DoD).
type Radio struct {
	Root  *primitive.Pressable
	dot   *primitive.Decorated // default indicator (icon semantic)
	btn   *primitive.Decorated // button chrome (optionType=button / Radio.Button)
	label *primitive.Text

	// Label is the visible text (antd children).
	Label string
	// Title is option title (antd title); also a11y name fallback.
	Title string
	// Value is compared within RadioGroup (antd value). Not the checked bool.
	Value string

	Checked    bool
	Disabled   bool
	Controlled bool
	// ButtonMode: antd Radio.Button / optionType=button item chrome.
	ButtonMode bool

	OnChange  func(checked bool)
	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// group, if set, routes activation through RadioGroup (exclusive).
	group *RadioGroup

	// size/buttonStyle mirrored from group when bound (button chrome).
	size        RadioSize
	buttonStyle RadioButtonStyle

	lastHovered bool
	lastFocused bool
	indSize     float64
}

// RadioOption is one Group options[] entry (antd CheckboxOptionType subset).
type RadioOption struct {
	Label    string
	Value    string
	Disabled bool
	Title    string
}

// RadioGroup coordinates exclusive selection (antd Radio.Group).
//
// Default layout is inline-flex wrap with gap=marginXS (8). Vertical /
// block / button modes adjust axis, ExpandMax, and Flexible equal-width.
type RadioGroup struct {
	root *primitive.Flex
	body core.Node

	Value       string
	Options     []RadioOption
	Items       []*Radio
	Disabled    bool
	Name        string
	Size        RadioSize
	OptionType  RadioOptionType
	ButtonStyle RadioButtonStyle
	Orientation RadioOrientation
	// verticalHint is SetVertical(true) when Orientation is still default-horizontal.
	// orientation 优先：explicit Orientation Vertical wins; else verticalHint.
	verticalHint bool
	orientSet    bool
	Block        bool
	// Controlled: activation only fires OnChange; parent must SetValue.
	Controlled bool

	OnChange  func(value string)
	Face      text.Face
	Theme     *core.Theme
	Style     Style
	AriaLabel string

	Nav *core.KeyboardNav
}

// NewRadio creates a radio with the given label (antd children).
func NewRadio(label string) *Radio {
	r := &Radio{Label: label}
	r.rebuild()
	return r
}

// NewRadioButton creates a button-style radio (antd Radio.Button).
func NewRadioButton(label string) *Radio {
	r := &Radio{Label: label, ButtonMode: true}
	r.rebuild()
	return r
}

// Node returns the root Pressable.
func (r *Radio) Node() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// ChromeNode returns the root chrome (Pressable).
func (r *Radio) ChromeNode() core.Node {
	if r.Root == nil {
		r.rebuild()
	}
	return r.Root
}

// IndicatorNode returns the bare indicator (icon semantic part, no label).
// For button mode returns the button chrome Decorated.
func (r *Radio) IndicatorNode() core.Node {
	if r.isButton() {
		if r.btn == nil {
			r.rebuild()
		}
		return r.btn
	}
	if r.dot == nil {
		r.rebuild()
	}
	return r.dot
}

// LabelNode returns the label text node (label semantic part).
func (r *Radio) LabelNode() core.Node {
	if r.label == nil {
		r.rebuild()
	}
	return r.label
}

// SetChecked updates checked chrome. Does not fire OnChange.
func (r *Radio) SetChecked(v bool) {
	if r.Checked == v {
		r.applyChrome()
		return
	}
	r.Checked = v
	r.applyChrome()
	r.applyA11y()
}

// SetDefaultChecked sets the initial checked state when not controlled.
func (r *Radio) SetDefaultChecked(v bool) {
	if r.Controlled {
		return
	}
	r.SetChecked(v)
}

// SetControlled marks parent-owned checked (antd checked={…} controlled).
func (r *Radio) SetControlled(v bool) { r.Controlled = v }

// SetDisabled toggles disabled chrome and interaction.
func (r *Radio) SetDisabled(d bool) {
	r.Disabled = d
	if r.Root != nil {
		r.Root.SetDisabled(d || r.groupDisabled())
	}
	r.applyChrome()
	r.applyA11y()
}

// SetLabel updates the visible label text.
func (r *Radio) SetLabel(s string) {
	r.Label = s
	if r.label != nil {
		r.label.SetValue(s)
	}
	r.applyA11y()
}

// SetTitle sets antd title (option title / tooltip attribute).
func (r *Radio) SetTitle(s string) {
	r.Title = s
	r.applyA11y()
}

// SetValue sets the Group option value (not checked).
func (r *Radio) SetValue(v string) { r.Value = v }

// SetOnChange sets the change callback (desired next checked for standalone).
func (r *Radio) SetOnChange(fn func(bool)) { r.OnChange = fn }

// SetAriaLabel sets the accessible name override.
func (r *Radio) SetAriaLabel(name string) {
	r.AriaLabel = name
	r.applyA11y()
}

// SetFace sets the label font.
func (r *Radio) SetFace(face text.Face) {
	r.Face = face
	r.Style.Face = face
	if r.label != nil {
		r.label.Face = face
	}
}

// SetStyle applies visual overrides.
func (r *Radio) SetStyle(st Style) {
	r.Style = st
	if st.Face != nil {
		r.SetFace(st.Face)
	}
	if st.FontSize > 0 && r.label != nil {
		r.label.FontSize = st.FontSize
	}
	r.applyChrome()
}

// SetTheme sets an explicit theme (highest priority).
func (r *Radio) SetTheme(th *core.Theme) {
	r.Theme = th
	r.applyChrome()
}

// SetTextColor overrides label color.
func (r *Radio) SetTextColor(col render.RGBA) {
	r.Style.Text = col
	r.applyChrome()
}

// SetButtonMode toggles Radio.Button chrome (also applied by group optionType).
func (r *Radio) SetButtonMode(v bool) {
	if r.ButtonMode == v {
		return
	}
	r.ButtonMode = v
	r.rebuild()
}

// SyncState reapplies hover/focus chrome from Pressable.
func (r *Radio) SyncState() {
	if r.Root == nil {
		return
	}
	h := r.Root.State.Hovered
	f := r.Root.State.Focused && r.Root.State.FocusVisible
	if h == r.lastHovered && f == r.lastFocused {
		return
	}
	r.lastHovered = h
	r.lastFocused = f
	r.applyChrome()
}

func (r *Radio) theme() *core.Theme {
	var n core.Node
	if r.Root != nil {
		n = r.Root
	}
	return themeOf(r.Theme, n)
}

func (r *Radio) groupDisabled() bool {
	return r.group != nil && r.group.Disabled
}

func (r *Radio) isDisabled() bool {
	return r.Disabled || r.groupDisabled()
}

func (r *Radio) isButton() bool {
	if r.ButtonMode {
		return true
	}
	return r.group != nil && r.group.OptionType == RadioOptionButton
}

func (r *Radio) effectiveSize() RadioSize {
	if r.group != nil {
		return r.group.Size
	}
	return r.size
}

func (r *Radio) effectiveButtonStyle() RadioButtonStyle {
	if r.group != nil {
		return r.group.ButtonStyle
	}
	return r.buttonStyle
}

func (r *Radio) controlHeight() float64 {
	th := r.theme()
	switch r.effectiveSize() {
	case RadioSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case RadioLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (r *Radio) rebuild() {
	if r.isButton() {
		r.rebuildButton()
		return
	}
	r.rebuildDefault()
}

func (r *Radio) rebuildDefault() {
	th := r.theme()
	size := th.SizeOr(core.TokenSizeIndicator, DefaultRadioIndicator)
	r.indSize = size
	gap := th.SizeOr(core.TokenMarginSM, DefaultRadioGap)
	lineW := th.SizeOr(core.TokenLineWidth, DefaultRadioLineWidth)

	// Transparent Decorated shell for layout size; circle painted by PainterNode.
	r.dot = primitive.NewDecorated()
	r.dot.Width, r.dot.Height = size, size
	r.dot.MinWidth, r.dot.MinHeight = size, size
	r.dot.Radius = size / 2
	r.dot.BorderWidth = 0
	r.dot.Background = render.RGBA{}
	r.btn = nil

	ring := primitive.NewPainterNode(func(pc *core.PaintContext, sz core.Size) {
		if pc == nil {
			return
		}
		w, h := sz.Width, sz.Height
		if w <= 0 {
			w = size
		}
		if h <= 0 {
			h = size
		}
		cx, cy := w/2, h/2
		outerR := w / 2
		if h/2 < outerR {
			outerR = h / 2
		}
		// Background fill.
		bg := th.Color(core.TokenColorBgContainer)
		if r.isDisabled() {
			bg = th.Color(core.TokenColorDisabledBg)
		}
		pc.FillLocalCircle(cx, cy, outerR, bg)

		// Border color by state.
		bd := th.Color(core.TokenColorBorder)
		if r.isDisabled() {
			bd = th.Color(core.TokenColorBorder)
		} else if r.Checked {
			bd = th.Color(core.TokenColorPrimary)
		} else if r.Root != nil && r.Root.State.Hovered {
			bd = th.Color(core.TokenColorPrimary)
		}
		pc.StrokeLocalCircle(cx, cy, outerR, lineW, bd)

		// Selected: inner primary disc (antd radio dot).
		if r.Checked {
			dotR := DefaultRadioDot / 2
			// Scale with indicator size (16 → 6 diameter).
			if size > 0 {
				dotR = (size * (DefaultRadioDot / DefaultRadioIndicator)) / 2
			}
			col := th.Color(core.TokenColorPrimary)
			if r.isDisabled() {
				col = th.Color(core.TokenColorDisabledText)
				if col.A < 0.2 {
					p := th.Color(core.TokenColorPrimary)
					col = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.45}
				}
			}
			pc.FillLocalCircle(cx, cy, dotR, col)
		}
	})
	ring.Width, ring.Height = size, size
	r.dot.AddChild(ring)

	r.label = primitive.NewText(r.Label)
	r.label.FontSize = th.SizeOr(core.TokenFontSize, 14)
	if r.Style.FontSize > 0 {
		r.label.FontSize = r.Style.FontSize
	}
	r.label.Face = r.Face

	row := primitive.Row(r.dot, r.label)
	row.Gap = gap
	row.CrossAlign = core.CrossCenter

	if r.Root == nil {
		r.Root = primitive.NewPressable(row)
	} else {
		r.Root.ClearChildren()
		r.Root.AddChild(row)
	}
	r.Root.Padding = primitive.EdgeInsets{}
	r.Root.Focusable = true
	r.Root.ShowFocusRing = true // §6.6 focus ring visible
	r.Root.FocusRingRadius = size / 2
	r.Root.OnStateChange = r.SyncState
	r.Root.Click = r.onActivate
	r.Root.SetDisabled(r.isDisabled())
	r.applyA11y()
	r.applyChrome()
}

func (r *Radio) rebuildButton() {
	th := r.theme()
	h := r.controlHeight()
	radius := th.SizeOr(core.TokenBorderRadius, 6)
	lineW := th.SizeOr(core.TokenLineWidth, DefaultRadioLineWidth)
	font := th.SizeOr(core.TokenFontSize, 14)
	if r.effectiveSize() == RadioSmall {
		font = th.SizeOr(core.TokenFontSizeSM, 12)
	} else if r.effectiveSize() == RadioLarge {
		font = th.SizeOr(core.TokenFontSizeLG, 16)
	}
	if r.Style.FontSize > 0 {
		font = r.Style.FontSize
	}

	r.dot = nil
	r.label = primitive.NewText(r.Label)
	r.label.FontSize = font
	r.label.Face = r.Face

	r.btn = primitive.NewDecorated(r.label)
	r.btn.MinHeight = h
	r.btn.Height = h
	r.btn.Padding = primitive.Symmetric(DefaultRadioBtnPadH, 0)
	r.btn.Radius = radius
	r.btn.BorderWidth = lineW
	r.btn.SetCenterContent(true)

	if r.Root == nil {
		r.Root = primitive.NewPressable(r.btn)
	} else {
		r.Root.ClearChildren()
		r.Root.AddChild(r.btn)
	}
	r.Root.Padding = primitive.EdgeInsets{}
	r.Root.Focusable = true
	r.Root.ShowFocusRing = true
	r.Root.FocusRingRadius = radius
	r.Root.OnStateChange = r.SyncState
	r.Root.Click = r.onActivate
	r.Root.SetDisabled(r.isDisabled())
	// Block equal-width: parent Flexible fills; expand pressable to host.
	if r.group != nil && r.group.Block {
		// Pressable fills Flexible allocation via parent stretch.
	}
	r.applyA11y()
	r.applyChrome()
}

func (r *Radio) onActivate() {
	if r.isDisabled() {
		return
	}
	if r.group != nil {
		r.group.selectOption(r)
		return
	}
	// Standalone: select (cannot uncheck by re-click — antd radio).
	if r.Checked {
		// Keep checked; still allow OnChange(true) only if not already — antd no-op.
		return
	}
	if !r.Controlled {
		r.Checked = true
		r.applyChrome()
		r.applyA11y()
	}
	if r.OnChange != nil {
		r.OnChange(true)
	}
}

func (r *Radio) applyA11y() {
	if r.Root == nil {
		return
	}
	name := r.AriaLabel
	if name == "" {
		name = r.Label
	}
	if name == "" {
		name = r.Title
	}
	r.Root.Base().Role = "radio"
	r.Root.Base().Label = name
}

func (r *Radio) applyChrome() {
	if r.isButton() {
		r.applyButtonChrome()
		return
	}
	if r.dot == nil {
		return
	}
	th := r.theme()
	// Circle paint reads Checked/Disabled/Hovered each frame.
	if r.label != nil {
		if r.isDisabled() {
			r.label.Color = th.Color(core.TokenColorDisabledText)
		} else {
			r.label.Color = th.Color(core.TokenColorText)
			if r.Style.hasText() {
				r.label.Color = r.Style.Text
			}
		}
	}
	r.dot.MarkNeedsPaint()
}

func (r *Radio) applyButtonChrome() {
	if r.btn == nil {
		return
	}
	th := r.theme()
	disabled := r.isDisabled()
	hovered := r.Root != nil && r.Root.State.Hovered && !disabled
	solid := r.effectiveButtonStyle() == RadioButtonSolid

	if disabled {
		if r.Checked {
			// Checked disabled: muted primary (outline) or disabled filled (solid).
			if solid {
				p := th.Color(core.TokenColorPrimary)
				r.btn.Background = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.45}
				r.btn.BorderColor = r.btn.Background
				if r.label != nil {
					r.label.Color = th.Color(core.TokenColorTextInverse)
					if r.label.Color.A < 0.1 {
						r.label.Color = render.RGBA{R: 1, G: 1, B: 1, A: 0.55}
					}
				}
			} else {
				r.btn.Background = th.Color(core.TokenColorDisabledBg)
				r.btn.BorderColor = th.Color(core.TokenColorPrimary)
				if r.label != nil {
					r.label.Color = th.Color(core.TokenColorDisabledText)
				}
			}
		} else {
			r.btn.Background = th.Color(core.TokenColorDisabledBg)
			r.btn.BorderColor = th.Color(core.TokenColorBorder)
			if r.label != nil {
				r.label.Color = th.Color(core.TokenColorDisabledText)
			}
		}
		r.btn.MarkNeedsPaint()
		return
	}

	if r.Checked {
		if solid {
			// Solid checked: primary fill + inverse text.
			r.btn.Background = th.Color(core.TokenColorPrimary)
			r.btn.BorderColor = th.Color(core.TokenColorPrimary)
			if hovered {
				r.btn.Background = th.Color(core.TokenColorPrimaryHover)
				r.btn.BorderColor = r.btn.Background
			}
			if r.label != nil {
				r.label.Color = th.Color(core.TokenColorTextInverse)
				if r.label.Color.A < 0.1 {
					r.label.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
				}
			}
		} else {
			// Outline checked: light primary wash + primary border/text.
			p := th.Color(core.TokenColorPrimary)
			r.btn.Background = render.RGBA{R: p.R, G: p.G, B: p.B, A: 0.08}
			r.btn.BorderColor = p
			if r.label != nil {
				r.label.Color = p
			}
		}
	} else {
		r.btn.Background = th.Color(core.TokenColorBgContainer)
		r.btn.BorderColor = th.Color(core.TokenColorBorder)
		if r.label != nil {
			r.label.Color = th.Color(core.TokenColorText)
			if r.Style.hasText() {
				r.label.Color = r.Style.Text
			}
		}
		if hovered {
			r.btn.BorderColor = th.Color(core.TokenColorPrimary)
			if r.label != nil {
				r.label.Color = th.Color(core.TokenColorPrimary)
			}
		}
	}
	r.btn.MarkNeedsPaint()
}

// ── Radio.Group ──────────────────────────────────────────────────────────────

// NewRadioGroup creates an empty exclusive-select group (antd Radio.Group).
func NewRadioGroup() *RadioGroup {
	g := &RadioGroup{}
	g.ensureRoot()
	g.applyA11y()
	return g
}

func (g *RadioGroup) ensureRoot() {
	if g.root != nil {
		return
	}
	g.root = primitive.Row()
	g.root.Gap = DefaultRadioGap
	g.root.Wrap = true
	g.root.CrossAlign = core.CrossCenter
}

// Node returns the group root (flex or custom body host).
func (g *RadioGroup) Node() core.Node {
	g.ensureRoot()
	return g.root
}

// SetOptions replaces options and rebuilds item radios (antd options).
func (g *RadioGroup) SetOptions(opts ...RadioOption) {
	g.Options = append([]RadioOption(nil), opts...)
	g.rebuildFromOptions()
}

// SetStringOptions is plainOptions sugar: label=value=each string.
func (g *RadioGroup) SetStringOptions(labels ...string) {
	opts := make([]RadioOption, len(labels))
	for i, s := range labels {
		opts[i] = RadioOption{Label: s, Value: s}
	}
	g.SetOptions(opts...)
}

// SetValue sets the selected value (controlled-friendly).
func (g *RadioGroup) SetValue(v string) {
	g.Value = v
	g.Controlled = true
	g.syncItemsFromValue()
}

// SetDefaultValue sets initial selection when not controlled.
func (g *RadioGroup) SetDefaultValue(v string) {
	if g.Controlled {
		return
	}
	g.Value = v
	g.syncItemsFromValue()
}

// SetDisabled disables the whole group (and all items).
func (g *RadioGroup) SetDisabled(d bool) {
	g.Disabled = d
	for _, it := range g.Items {
		if it.Root != nil {
			it.Root.SetDisabled(it.Disabled || d)
		}
		it.applyChrome()
	}
}

// SetName sets the group name (antd name; desktop a11y metadata).
func (g *RadioGroup) SetName(name string) {
	g.Name = name
	g.applyA11y()
}

// SetOnChange sets the group change callback (selected value).
func (g *RadioGroup) SetOnChange(fn func(string)) { g.OnChange = fn }

// SetFace propagates face to option-built items.
func (g *RadioGroup) SetFace(face text.Face) {
	g.Face = face
	for _, it := range g.Items {
		it.SetFace(face)
	}
}

// SetTheme sets an explicit theme.
func (g *RadioGroup) SetTheme(th *core.Theme) {
	g.Theme = th
	for _, it := range g.Items {
		it.SetTheme(th)
	}
}

// SetAriaLabel sets the group accessible name.
func (g *RadioGroup) SetAriaLabel(s string) {
	g.AriaLabel = s
	g.applyA11y()
}

// SetSize sets button-style size (small|medium|large).
func (g *RadioGroup) SetSize(s RadioSize) {
	g.Size = s
	g.relayoutItems()
}

// SetOptionType sets default vs button chrome for options/items.
func (g *RadioGroup) SetOptionType(t RadioOptionType) {
	if g.OptionType == t {
		return
	}
	g.OptionType = t
	g.relayoutItems()
}

// SetButtonStyle sets outline | solid for button chrome.
func (g *RadioGroup) SetButtonStyle(s RadioButtonStyle) {
	g.ButtonStyle = s
	for _, it := range g.Items {
		it.applyChrome()
	}
}

// SetOrientation sets horizontal | vertical layout.
func (g *RadioGroup) SetOrientation(o RadioOrientation) {
	g.Orientation = o
	g.orientSet = true
	g.applyLayout()
}

// SetVertical is antd vertical convenience; orientation 优先.
func (g *RadioGroup) SetVertical(v bool) {
	g.verticalHint = v
	g.applyLayout()
}

// SetBlock toggles block (full-width) group layout.
func (g *RadioGroup) SetBlock(v bool) {
	g.Block = v
	g.applyLayout()
	// Block + button: rebuild to wrap Flexible equal-width.
	if g.body == nil && len(g.Options) > 0 {
		g.rebuildFromOptions()
	} else {
		g.relayoutItems()
	}
}

// Add registers child radios (children mode) and binds them.
// Does not change the visual tree; use SetBody or Node().AddChild for layout,
// or call Add then auto-append when body is nil.
func (g *RadioGroup) Add(items ...*Radio) {
	g.ensureRoot()
	for _, it := range items {
		if it == nil {
			continue
		}
		it.group = g
		if g.Face != nil && it.Face == nil {
			it.SetFace(g.Face)
		}
		if g.Theme != nil {
			it.SetTheme(g.Theme)
		}
		// Avoid double-register.
		found := false
		for _, ex := range g.Items {
			if ex == it {
				found = true
				break
			}
		}
		if !found {
			g.Items = append(g.Items, it)
		}
		// Sync checked from current value.
		want := it.optionValue() == g.Value
		it.Checked = want
		// Button mode from group optionType.
		if g.OptionType == RadioOptionButton && !it.ButtonMode {
			it.ButtonMode = true
			it.rebuild()
		} else if g.OptionType == RadioOptionDefault && it.ButtonMode {
			// keep explicit Radio.Button
			it.rebuild()
		} else {
			if it.Root != nil {
				it.Root.SetDisabled(it.Disabled || g.Disabled)
			}
			it.applyChrome()
			it.applyA11y()
		}
		if g.body == nil {
			// Append if not already a child.
			already := false
			for _, c := range g.root.Children() {
				if c == it.Node() {
					already = true
					break
				}
			}
			if !already {
				g.root.AddChild(g.wrapItem(it))
			}
		}
	}
	g.applyLayout()
	g.applyA11y()
}

// SetBody replaces the group visual children with a custom layout node.
// Bound items still coordinate via Add.
func (g *RadioGroup) SetBody(n core.Node) {
	g.body = n
	g.ensureRoot()
	g.root.ClearChildren()
	if n != nil {
		g.root.AddChild(n)
	}
	g.root.Wrap = false
	g.root.CrossAlign = core.CrossStretch
	g.root.ExpandMax = true
	g.applyA11y()
}

// HandleKey routes arrow keys among selectable radios (L1 RDO-09 / §6.6).
// Host may call after focus; also usable from tests (Tabs pattern).
func (g *RadioGroup) HandleKey(ev *core.KeyEvent) bool {
	if g == nil || g.Disabled || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	enabled := g.enabledItems()
	if len(enabled) == 0 {
		return false
	}
	if g.Nav == nil {
		mode := core.NavHorizontal
		if g.isVertical() {
			mode = core.NavVertical
		}
		g.Nav = core.NewKeyboardNav(mode, len(enabled))
	} else {
		if g.isVertical() {
			g.Nav.Mode = core.NavVertical
		} else {
			g.Nav.Mode = core.NavHorizontal
		}
		g.Nav.SetCount(len(enabled))
	}
	// Sync index to current value.
	for i, it := range enabled {
		if it.optionValue() == g.Value {
			g.Nav.Index = i
			break
		}
	}
	prev := g.Nav.Index
	if !g.Nav.HandleKey(ev.Key) {
		return false
	}
	if g.Nav.Index != prev && g.Nav.Index >= 0 && g.Nav.Index < len(enabled) {
		g.selectOption(enabled[g.Nav.Index])
		return true
	}
	return true
}

func (g *RadioGroup) enabledItems() []*Radio {
	out := make([]*Radio, 0, len(g.Items))
	for _, it := range g.Items {
		if it == nil || it.Disabled {
			continue
		}
		out = append(out, it)
	}
	return out
}

func (g *RadioGroup) isVertical() bool {
	if g.orientSet {
		return g.Orientation == RadioVertical
	}
	return g.verticalHint || g.Orientation == RadioVertical
}

func (g *RadioGroup) applyA11y() {
	g.ensureRoot()
	g.root.Base().Role = "radiogroup"
	g.root.Base().Label = g.AriaLabel
	if g.AriaLabel == "" && g.Name != "" {
		g.root.Base().Label = g.Name
	}
}

func (g *RadioGroup) applyLayout() {
	g.ensureRoot()
	if g.body != nil {
		return
	}
	if g.isVertical() {
		g.root.Axis = core.AxisVertical
		g.root.Wrap = false
		g.root.CrossAlign = core.CrossStart
	} else {
		g.root.Axis = core.AxisHorizontal
		g.root.Wrap = !g.Block || g.OptionType == RadioOptionDefault
		g.root.CrossAlign = core.CrossCenter
	}
	// Button group: tighter gap (joined look for outline).
	if g.OptionType == RadioOptionButton || g.hasButtonItem() {
		g.root.Gap = 0
		g.root.Wrap = false
	} else {
		g.root.Gap = DefaultRadioGap
	}
	g.root.ExpandMax = g.Block
	if g.root != nil {
		g.root.MarkNeedsLayout()
	}
}

func (g *RadioGroup) hasButtonItem() bool {
	for _, it := range g.Items {
		if it != nil && it.ButtonMode {
			return true
		}
	}
	return false
}

func (g *RadioGroup) wrapItem(it *Radio) core.Node {
	n := it.Node()
	if g.Block && (g.OptionType == RadioOptionButton || it.ButtonMode) {
		// Equal-width button cells.
		flex := primitive.NewFlexible(1, n)
		flex.FillChild = true
		return flex
	}
	if g.Block && g.OptionType == RadioOptionDefault {
		// Block default: each option stretches full width (column preferred).
		if g.isVertical() {
			host := primitive.NewDecorated(n)
			host.ExpandWidth = true
			return host
		}
	}
	return n
}

func (g *RadioGroup) rebuildFromOptions() {
	g.Items = g.Items[:0]
	g.ensureRoot()
	if g.body == nil {
		g.root.ClearChildren()
	}
	for _, opt := range g.Options {
		opt := opt
		var rd *Radio
		if g.OptionType == RadioOptionButton {
			rd = &Radio{Label: opt.Label, ButtonMode: true, group: g}
		} else {
			rd = &Radio{Label: opt.Label, group: g}
		}
		rd.Value = opt.Value
		if opt.Value == "" {
			rd.Value = opt.Label
		}
		rd.Title = opt.Title
		rd.Disabled = opt.Disabled
		if g.Face != nil {
			rd.Face = g.Face
		}
		if g.Theme != nil {
			rd.Theme = g.Theme
		}
		rd.rebuild()
		rd.Checked = rd.optionValue() == g.Value
		if rd.Root != nil {
			rd.Root.SetDisabled(rd.Disabled || g.Disabled)
		}
		rd.applyChrome()
		g.Items = append(g.Items, rd)
		if g.body == nil {
			g.root.AddChild(g.wrapItem(rd))
		}
	}
	g.applyLayout()
	g.applyA11y()
}

func (g *RadioGroup) relayoutItems() {
	// Rebuild visual tree when size/optionType/block changes.
	if len(g.Options) > 0 && g.body == nil {
		g.rebuildFromOptions()
		return
	}
	for _, it := range g.Items {
		if it == nil {
			continue
		}
		// Sync button mode from group when not explicit Radio.Button-only path.
		wantBtn := g.OptionType == RadioOptionButton || it.ButtonMode
		if wantBtn != it.isButton() || it.isButton() {
			if g.OptionType == RadioOptionButton {
				it.ButtonMode = true
			}
			it.rebuild()
		} else {
			it.applyChrome()
		}
	}
	if g.body == nil {
		g.ensureRoot()
		g.root.ClearChildren()
		for _, it := range g.Items {
			if it != nil {
				g.root.AddChild(g.wrapItem(it))
			}
		}
	}
	g.applyLayout()
}

func (g *RadioGroup) syncItemsFromValue() {
	for _, it := range g.Items {
		want := it.optionValue() == g.Value
		if it.Checked != want {
			it.Checked = want
			it.applyChrome()
			it.applyA11y()
		}
	}
}

func (r *Radio) optionValue() string {
	if r.Value != "" {
		return r.Value
	}
	return r.Label
}

func (g *RadioGroup) selectOption(rd *Radio) {
	if g.Disabled || rd == nil || rd.Disabled {
		return
	}
	val := rd.optionValue()
	// Re-clicking selected option keeps selection (no deselect).
	if val == g.Value && rd.Checked {
		return
	}
	if !g.Controlled {
		g.Value = val
		g.syncItemsFromValue()
	}
	// Fire item-level onChange (antd option.onChange).
	if rd.OnChange != nil {
		rd.OnChange(true)
	}
	if g.OnChange != nil {
		g.OnChange(val)
	}
}
