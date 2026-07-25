package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Segmented defaults — components/segmented/style prepareComponentToken.
// docs/antd/segmented.md §6.2 / §6.10
const (
	DefaultSegmentedTrackPadding = 2.0  // lineWidthBold / trackPadding
	DefaultSegmentedPadH         = 11.0 // controlPaddingHorizontal − lineWidth
	DefaultSegmentedPadHSM       = 7.0  // controlPaddingHorizontalSM − lineWidth
	DefaultSegmentedIconGap      = 4.0  // marginSM / 2
	DefaultSegmentedIconSize     = 14.0
	DefaultSegmentedFocusOutset  = 1.5
	DefaultSegmentedRadiusXS     = 2.0 // borderRadiusXS fallback
)

// SegmentedSize is antd size: small | middle | large.
type SegmentedSize int

const (
	SegmentedMiddle SegmentedSize = iota // controlHeight 32
	SegmentedSmall                       // controlHeightSM 24
	SegmentedLarge                       // controlHeightLG 40
)

// SegmentedOrientation is antd orientation: horizontal | vertical.
type SegmentedOrientation int

const (
	SegmentedHorizontal SegmentedOrientation = iota
	SegmentedVertical
)

// SegmentedShape is antd shape: default | round.
type SegmentedShape int

const (
	SegmentedShapeDefault SegmentedShape = iota
	SegmentedShapeRound
)

// SegmentedOption is one options[] entry (antd SegmentedItemType subset).
type SegmentedOption struct {
	// Label is the display text (antd label).
	Label string
	// Value is the option value; empty falls back to Label.
	Value string
	// Icon is a registry icon name (antd icon).
	Icon string
	// IconNode is a custom icon node (preferred over Icon when non-nil).
	IconNode core.Node
	// LabelNode replaces the default icon+label content (custom.tsx).
	LabelNode core.Node
	// Disabled disables this option only.
	Disabled bool
	// Title is a short tooltip fallback (P1 full TooltipProps deferred).
	Title string
	// AriaLabel is the accessible name; recommended for icon-only options.
	AriaLabel string
}

// Segmented is Ant Design Segmented (data display single-select).
//
//	Decorated track (Root, role=radiogroup)
//	  └─ Flex group (row | column; block → ExpandMax + Flexible equal-width)
//	       └─ Pressable item (role=radio) × N
//	            └─ Decorated chip
//	                 └─ LabelNode | Row(Icon?, Label?)
//
// Product contract: docs/antd/segmented.md §6 (P0 DoD).
// SetValue only recolors (Root identity stable); SetOptions rebuilds items.
type Segmented struct {
	Root  *primitive.Decorated
	group *primitive.Flex
	items []*segItem

	// Options is the current option list (antd options).
	Options []SegmentedOption
	// Value is the selected option value (antd value / internal state).
	Value string
	// Controlled: activation only fires OnChange; parent must SetValue.
	Controlled bool
	// Disabled disables the whole control.
	Disabled bool
	// Block expands to parent width with equal-width items (antd block).
	Block bool
	// Size ladder (antd SizeType).
	Size SegmentedSize
	// Orientation is horizontal (default) or vertical.
	Orientation SegmentedOrientation
	// verticalHint is antd vertical sugar; orientation takes precedence when set.
	verticalHint bool
	orientSet    bool
	// Shape is default rectangle or round capsule.
	Shape SegmentedShape
	// Name is the radio group name (P1 gallery; kept for API parity).
	Name string

	OnChange  func(value string)
	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// cached metrics / colors for applySelection
	trackPad   float64
	itemPadH   float64
	itemMinH   float64
	fontSize   float64
	trackR     float64
	itemR      float64
	iconSize   float64
	iconGap    float64
	controlH   float64
	trackBG    render.RGBA
	selectedBG render.RGBA
	itemColor  render.RGBA
	selColor   render.RGBA
	hoverColor render.RGBA
	hoverBG    render.RGBA
	activeBG   render.RGBA
	disColor   render.RGBA
}

// segItem is one focusable option chip.
type segItem struct {
	press *primitive.Pressable
	chip  *primitive.Decorated
	row   *primitive.Flex
	label *primitive.Text
	icon  *primitive.Icon

	seg   *Segmented
	index int
	value string
	dis   bool // option-level disabled
}

// NewSegmented creates a Segmented from string options (value == label).
// Defaults: size=middle, orientation=horizontal, shape=default, value=first option.
func NewSegmented(options ...string) *Segmented {
	opts := make([]SegmentedOption, len(options))
	for i, s := range options {
		opts[i] = SegmentedOption{Label: s, Value: s}
	}
	return NewSegmentedOptions(opts...)
}

// NewSegmentedOptions creates a Segmented from full option descriptors.
func NewSegmentedOptions(opts ...SegmentedOption) *Segmented {
	s := &Segmented{
		Options:     append([]SegmentedOption(nil), opts...),
		Size:        SegmentedMiddle,
		Orientation: SegmentedHorizontal,
		Shape:       SegmentedShapeDefault,
	}
	if len(s.Options) > 0 {
		s.Value = s.optionValue(s.Options[0])
	}
	s.rebuild()
	return s
}

// Node returns the track root (stable across SetValue).
func (s *Segmented) Node() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// ChromeNode returns the track Decorated (tests / composition).
func (s *Segmented) ChromeNode() core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	return s.Root
}

// OptionNodes returns each option Pressable (layout / click tests).
func (s *Segmented) OptionNodes() []core.Node {
	if s.Root == nil {
		s.rebuild()
	}
	out := make([]core.Node, len(s.items))
	for i, it := range s.items {
		if it != nil {
			out[i] = it.press
		}
	}
	return out
}

// SelectedIndex returns the index of Value in Options, or -1.
func (s *Segmented) SelectedIndex() int {
	if s == nil {
		return -1
	}
	for i, opt := range s.Options {
		if s.optionValue(opt) == s.Value {
			return i
		}
	}
	return -1
}

// TrackPadding returns resolved track padding (tests / L2).
func (s *Segmented) TrackPadding() float64 {
	s.resolveMetrics()
	return s.trackPad
}

// ControlHeight returns resolved outer control height (tests / L2).
func (s *Segmented) ControlHeight() float64 {
	s.resolveMetrics()
	return s.controlH
}

// ItemMinHeight returns resolved item label min height (tests / L2).
func (s *Segmented) ItemMinHeight() float64 {
	s.resolveMetrics()
	return s.itemMinH
}

// ItemPadH returns resolved horizontal item padding (tests / L2).
func (s *Segmented) ItemPadH() float64 {
	s.resolveMetrics()
	return s.itemPadH
}

// TrackRadius returns resolved track corner radius (tests / L2).
func (s *Segmented) TrackRadius() float64 {
	s.resolveMetrics()
	return s.trackR
}

// ItemRadius returns resolved item corner radius (tests / L2).
func (s *Segmented) ItemRadius() float64 {
	s.resolveMetrics()
	return s.itemR
}

// TrackBG returns track background color (tests / L2).
func (s *Segmented) TrackBG() render.RGBA {
	s.resolveMetrics()
	return s.trackBG
}

// SelectedBG returns selected item background (tests / L2).
func (s *Segmented) SelectedBG() render.RGBA {
	s.resolveMetrics()
	return s.selectedBG
}

// ItemColor returns default item text color (tests / L2).
func (s *Segmented) ItemColor() render.RGBA {
	s.resolveMetrics()
	return s.itemColor
}

// SelectedColor returns selected item text color (tests / L2).
func (s *Segmented) SelectedColor() render.RGBA {
	s.resolveMetrics()
	return s.selColor
}

// SetOptions replaces the option list and rebuilds items (antd options=).
// Preserves Value when still present; otherwise selects the first option.
func (s *Segmented) SetOptions(opts ...SegmentedOption) {
	s.Options = append([]SegmentedOption(nil), opts...)
	if s.SelectedIndex() < 0 {
		if len(s.Options) > 0 {
			s.Value = s.optionValue(s.Options[0])
		} else {
			s.Value = ""
		}
	}
	s.rebuild()
}

// SetOptionsStrings is a convenience for string options (value == label).
func (s *Segmented) SetOptionsStrings(options ...string) {
	opts := make([]SegmentedOption, len(options))
	for i, v := range options {
		opts[i] = SegmentedOption{Label: v, Value: v}
	}
	s.SetOptions(opts...)
}

// SetValue sets the selected value programmatically (antd value=).
// Does not fire OnChange. Root / item tree identity is preserved when possible.
func (s *Segmented) SetValue(v string) {
	if s == nil {
		return
	}
	s.Value = v
	if s.group == nil || len(s.items) != len(s.Options) {
		s.rebuild()
		return
	}
	s.applySelection()
	s.applyA11y()
}

// SetDefaultValue sets the initial value when not controlled.
func (s *Segmented) SetDefaultValue(v string) {
	if s.Controlled {
		return
	}
	s.Value = v
	if s.group == nil || len(s.items) != len(s.Options) {
		s.rebuild()
		return
	}
	s.applySelection()
	s.applyA11y()
}

// SetControlled marks parent-owned value (antd value={…}).
func (s *Segmented) SetControlled(c bool) { s.Controlled = c }

// SetDisabled toggles whole-control disabled.
func (s *Segmented) SetDisabled(d bool) {
	s.Disabled = d
	for _, it := range s.items {
		if it == nil || it.press == nil {
			continue
		}
		it.press.SetDisabled(d || it.dis)
	}
	s.applySelection()
	s.applyA11y()
}

// SetBlock toggles full-width equal item layout.
func (s *Segmented) SetBlock(v bool) {
	if s.Block == v {
		return
	}
	s.Block = v
	s.rebuild()
}

// SetSize updates the size ladder and rebuilds metrics.
func (s *Segmented) SetSize(sz SegmentedSize) {
	if s.Size == sz {
		return
	}
	s.Size = sz
	s.rebuild()
}

// SetOrientation sets horizontal | vertical layout.
func (s *Segmented) SetOrientation(o SegmentedOrientation) {
	s.Orientation = o
	s.orientSet = true
	s.rebuild()
}

// SetVertical is antd vertical convenience; orientation wins when SetOrientation was used.
func (s *Segmented) SetVertical(v bool) {
	s.verticalHint = v
	if !s.orientSet {
		if v {
			s.Orientation = SegmentedVertical
		} else {
			s.Orientation = SegmentedHorizontal
		}
	}
	s.rebuild()
}

// SetShape sets default | round (capsule).
func (s *Segmented) SetShape(sh SegmentedShape) {
	if s.Shape == sh {
		return
	}
	s.Shape = sh
	s.rebuild()
}

// SetName sets the radio group name (API parity; P1 gallery).
func (s *Segmented) SetName(name string) { s.Name = name }

// SetOnChange registers the value-change callback.
func (s *Segmented) SetOnChange(fn func(value string)) { s.OnChange = fn }

// SetFace sets the font face (rebuilds labels).
func (s *Segmented) SetFace(face text.Face) {
	s.Face = face
	s.rebuild()
}

// SetTheme sets the theme and refreshes chrome.
func (s *Segmented) SetTheme(th *core.Theme) {
	s.Theme = th
	s.resolveMetrics()
	s.applyTrackChrome()
	s.applySelection()
}

// SetStyle applies optional one-off visual overrides.
func (s *Segmented) SetStyle(st Style) {
	s.Style = st
	s.rebuild()
}

// SetAriaLabel sets the radiogroup accessible name.
func (s *Segmented) SetAriaLabel(name string) {
	s.AriaLabel = name
	s.applyA11y()
}

func (s *Segmented) theme() *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Segmented) optionValue(opt SegmentedOption) string {
	if opt.Value != "" {
		return opt.Value
	}
	return opt.Label
}

func (s *Segmented) resolvedOrientation() SegmentedOrientation {
	if s.orientSet {
		return s.Orientation
	}
	if s.verticalHint {
		return SegmentedVertical
	}
	return s.Orientation
}

func (s *Segmented) resolveMetrics() {
	th := s.theme()
	s.trackPad = DefaultSegmentedTrackPadding
	if v := th.SizeOr("lineWidthBold", 0); v > 0 {
		s.trackPad = v
	}
	s.iconGap = DefaultSegmentedIconGap
	s.iconSize = DefaultSegmentedIconSize

	switch s.Size {
	case SegmentedSmall:
		s.controlH = th.SizeOr(core.TokenControlHeightSM, 24)
		s.itemPadH = DefaultSegmentedPadHSM
		s.fontSize = th.SizeOr(core.TokenFontSize, 14)
		s.trackR = th.SizeOr(core.TokenBorderRadiusSM, 4)
		s.itemR = DefaultSegmentedRadiusXS
	case SegmentedLarge:
		s.controlH = th.SizeOr(core.TokenControlHeightLG, 40)
		s.itemPadH = DefaultSegmentedPadH
		s.fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
		s.trackR = th.SizeOr(core.TokenBorderRadiusLG, 8)
		s.itemR = th.SizeOr(core.TokenBorderRadius, 6)
	default: // middle
		s.controlH = th.SizeOr(core.TokenControlHeight, 32)
		s.itemPadH = DefaultSegmentedPadH
		s.fontSize = th.SizeOr(core.TokenFontSize, 14)
		s.trackR = th.SizeOr(core.TokenBorderRadius, 6)
		s.itemR = th.SizeOr(core.TokenBorderRadiusSM, 4)
	}
	if s.Style.FontSize > 0 {
		s.fontSize = s.Style.FontSize
	}
	if s.Style.Height > 0 {
		s.controlH = s.Style.Height
	}
	s.itemMinH = s.controlH - 2*s.trackPad
	if s.itemMinH < 0 {
		s.itemMinH = 0
	}
	if s.Shape == SegmentedShapeRound {
		s.trackR = 9999
		s.itemR = 9999
	} else if s.Style.hasRadius() {
		s.trackR = s.Style.Radius
	}

	s.trackBG = th.Color(core.TokenColorBgLayout)
	if s.trackBG.A < 0.01 {
		s.trackBG = render.RGBA{R: 0, G: 0, B: 0, A: 0.04}
	}
	// itemSelectedBg → colorBgElevated ≈ colorBgContainer
	s.selectedBG = th.Color(core.TokenColorBgContainer)
	if s.selectedBG.A < 0.5 {
		s.selectedBG = render.RGBA{R: 1, G: 1, B: 1, A: 1}
	}
	// itemColor → colorTextLabel ≈ secondary
	s.itemColor = th.Color(core.TokenColorTextSecondary)
	if s.itemColor.A < 0.01 {
		s.itemColor = th.Color(core.TokenColorText)
	}
	s.selColor = th.Color(core.TokenColorText)
	s.hoverColor = th.Color(core.TokenColorText)
	s.hoverBG = th.Color(core.TokenColorFillSecondary)
	if s.hoverBG.A < 0.01 {
		s.hoverBG = render.RGBA{R: 0, G: 0, B: 0, A: 0.06}
	}
	// itemActiveBg ≈ deeper fill
	s.activeBG = th.Color(core.TokenColorBgTextActive)
	if s.activeBG.A < 0.01 {
		s.activeBG = render.RGBA{R: 0, G: 0, B: 0, A: 0.15}
	}
	s.disColor = th.Color(core.TokenColorDisabledText)

	if s.Style.hasBG() {
		s.trackBG = s.Style.Background
	}
	if s.Style.hasText() {
		s.selColor = s.Style.Text
		s.itemColor = s.Style.Text
	}
}

func (s *Segmented) rebuild() {
	s.resolveMetrics()

	vert := s.resolvedOrientation() == SegmentedVertical
	if s.group == nil {
		if vert {
			s.group = primitive.Column()
		} else {
			s.group = primitive.Row()
		}
	} else {
		s.group.ClearChildren()
		if vert {
			s.group.Axis = core.AxisVertical
		} else {
			s.group.Axis = core.AxisHorizontal
		}
	}
	s.group.Gap = 0
	s.group.CrossAlign = core.CrossStretch
	s.group.MainAlign = core.MainStart
	s.group.ExpandMax = s.Block
	s.group.Hit = core.HitDefer

	s.items = s.items[:0]
	for i, opt := range s.Options {
		it := s.buildItem(i, opt)
		s.items = append(s.items, it)
		var host core.Node = it.press
		if s.Block {
			flex := primitive.NewFlexible(1, it.press)
			flex.FillChild = true
			host = flex
		}
		s.group.AddChild(host)
	}

	if s.Root == nil {
		s.Root = primitive.NewDecorated(s.group)
	} else {
		// Keep Root identity; swap single child if needed.
		kids := s.Root.Children()
		if len(kids) != 1 || kids[0] != s.group {
			s.Root.ClearChildren()
			s.Root.AddChild(s.group)
		}
	}
	s.Root.Hit = core.HitBlock
	s.Root.SetThemeHook(func(*core.Theme) {
		s.resolveMetrics()
		s.applyTrackChrome()
		s.applySelection()
	})
	s.applyTrackChrome()
	s.applySelection()
	s.applyA11y()
	s.Root.MarkNeedsLayout()
	s.Root.MarkNeedsPaint()
}

func (s *Segmented) buildItem(i int, opt SegmentedOption) *segItem {
	it := &segItem{
		seg:   s,
		index: i,
		value: s.optionValue(opt),
		dis:   opt.Disabled,
	}

	var content core.Node
	if opt.LabelNode != nil {
		content = opt.LabelNode
	} else {
		row := primitive.Row()
		row.Gap = s.iconGap
		row.CrossAlign = core.CrossCenter
		row.MainAlign = core.MainCenter
		row.Hit = core.HitDefer
		it.row = row

		if opt.IconNode != nil {
			row.AddChild(opt.IconNode)
		} else if opt.Icon != "" {
			ic := primitive.NewIcon(opt.Icon)
			ic.Size = s.iconSize
			it.icon = ic
			row.AddChild(ic)
		}
		if opt.Label != "" {
			lab := primitive.NewText(opt.Label)
			lab.FontSize = s.fontSize
			lab.Face = s.Face
			if s.Style.Face != nil {
				lab.Face = s.Style.Face
			}
			it.label = lab
			row.AddChild(lab)
		}
		content = row
	}

	chip := primitive.NewDecorated(content)
	chip.Padding = primitive.Symmetric(s.itemPadH, 0)
	chip.Radius = s.itemR
	chip.Hit = core.HitDefer
	chip.CenterContent = true
	if s.itemMinH > 0 {
		chip.MinHeight = s.itemMinH
	}
	if s.Block {
		chip.ExpandWidth = true
		chip.StretchChild = true
	}
	it.chip = chip

	p := primitive.NewPressable(chip)
	p.ShowFocusRing = true
	p.FocusRingRadius = s.itemR
	p.FocusRingOutset = DefaultSegmentedFocusOutset
	p.EnableRipple = false // Segmented uses fill hover, not ink
	p.SetDisabled(s.Disabled || opt.Disabled)
	val := it.value
	idx := i
	p.Click = func() { s.activate(idx, val) }
	p.OnStateChange = func() { s.applyItemChrome(it) }
	// Keyboard arrows handled at group via focused item's key — also wire item Key.
	// Pressable already handles Enter/Space → Click.
	it.press = p

	// a11y name
	name := opt.AriaLabel
	if name == "" {
		name = opt.Title
	}
	if name == "" {
		name = opt.Label
	}
	if name == "" {
		name = it.value
	}
	p.Base().Role = "radio"
	p.Base().Label = name

	return it
}

func (s *Segmented) applyTrackChrome() {
	if s.Root == nil {
		return
	}
	s.Root.Padding = primitive.All(s.trackPad)
	s.Root.Radius = s.trackR
	s.Root.Background = s.trackBG
	if s.Style.Width > 0 {
		s.Root.Width = s.Style.Width
	}
	s.Root.ExpandWidth = s.Block
}

func (s *Segmented) applySelection() {
	if s == nil {
		return
	}
	s.resolveMetrics()
	for _, it := range s.items {
		s.applyItemChrome(it)
	}
	if s.Root != nil {
		s.Root.MarkNeedsPaint()
	}
}

func (s *Segmented) applyItemChrome(it *segItem) {
	if it == nil || it.chip == nil {
		return
	}
	selected := it.value == s.Value
	disabled := s.Disabled || it.dis
	hovered := it.press != nil && it.press.State.Hovered
	pressed := it.press != nil && it.press.State.Pressed

	// background
	bg := render.RGBA{} // transparent
	fg := s.itemColor
	if selected {
		bg = s.selectedBG
		fg = s.selColor
	} else if !disabled {
		if pressed {
			bg = s.activeBG
			fg = s.hoverColor
		} else if hovered {
			bg = s.hoverBG
			fg = s.hoverColor
		}
	}
	if disabled {
		fg = s.disColor
		if selected {
			// keep selected bg but mute text
			bg = s.selectedBG
		}
	}

	it.chip.Background = bg
	it.chip.Radius = s.itemR
	it.chip.Padding = primitive.Symmetric(s.itemPadH, 0)
	if s.itemMinH > 0 {
		it.chip.MinHeight = s.itemMinH
	}

	if it.label != nil {
		it.label.Color = fg
		it.label.FontSize = s.fontSize
		it.label.Face = s.Face
		if s.Style.Face != nil {
			it.label.Face = s.Style.Face
		}
	}
	if it.icon != nil {
		it.icon.Color = fg
		it.icon.Size = s.iconSize
	}
	if it.press != nil {
		it.press.FocusRingRadius = s.itemR
		it.press.SetDisabled(disabled)
		if disabled {
			it.press.SetCursor(core.CursorDefault)
		} else {
			it.press.SetCursor(core.CursorPointer)
		}
	}
	it.chip.MarkNeedsPaint()
	if it.press != nil {
		it.press.MarkNeedsPaint()
	}
}

func (s *Segmented) applyA11y() {
	if s.Root == nil {
		return
	}
	s.Root.Base().Role = "radiogroup"
	s.Root.Base().Label = s.AriaLabel
	if s.AriaLabel == "" && s.Name != "" {
		s.Root.Base().Label = s.Name
	}
	for _, it := range s.items {
		if it == nil || it.press == nil {
			continue
		}
		it.press.Base().Role = "radio"
		// Checked state: append to label for headless a11y (no Selected field on NodeBase).
		// Visual selection is via applyItemChrome.
	}
}

// activate is the user interaction path (click / keyboard).
func (s *Segmented) activate(index int, value string) {
	if s == nil || s.Disabled {
		return
	}
	if index >= 0 && index < len(s.Options) && s.Options[index].Disabled {
		return
	}
	// Re-select same → no-op (no deselect, no repeated onChange).
	if value == s.Value {
		return
	}
	if !s.Controlled {
		s.Value = value
		s.applySelection()
		s.applyA11y()
	}
	if s.OnChange != nil {
		s.OnChange(value)
	}
}

// HandleKey implements optional group-level keyboard (call from host or tests).
// Arrow keys move selection among enabled options.
func (s *Segmented) HandleKey(ev *core.KeyEvent) bool {
	if s == nil || s.Disabled || ev == nil || ev.Type != core.KeyDown {
		return false
	}
	vert := s.resolvedOrientation() == SegmentedVertical
	dir := 0
	switch ev.Key {
	case "ArrowRight":
		if !vert {
			dir = 1
		}
	case "ArrowLeft":
		if !vert {
			dir = -1
		}
	case "ArrowDown":
		if vert {
			dir = 1
		}
	case "ArrowUp":
		if vert {
			dir = -1
		}
	default:
		return false
	}
	if dir == 0 {
		return false
	}
	n := len(s.Options)
	if n == 0 {
		return false
	}
	cur := s.SelectedIndex()
	if cur < 0 {
		cur = 0
	}
	for step := 1; step <= n; step++ {
		next := (cur + dir*step + n*10) % n
		if s.Options[next].Disabled {
			continue
		}
		s.activate(next, s.optionValue(s.Options[next]))
		return true
	}
	return false
}
