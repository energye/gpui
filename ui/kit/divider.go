package kit

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Divider defaults — components/divider/style/index.ts.
// docs/antd/divider.md §6.2 / §6.10
const (
	DefaultDividerLineWidth            = 1.0
	DefaultDividerMarginUnset          = 24.0 // marginLG when size unset / large
	DefaultDividerMarginSmall          = 8.0  // antd marginXS
	DefaultDividerMarginMedium         = 16.0 // antd margin
	DefaultDividerMarginWithText       = 16.0 // dividerHorizontalWithTextGutterMargin
	DefaultDividerOrientationMargin    = 0.05 // near-side rail ratio
	DefaultDividerVerticalEm           = 0.9
	DefaultDividerVerticalMarginInline = 8.0  // antd marginXS
	DefaultDividerTitleFont            = 16.0 // fontSizeLG
	DefaultDividerPlainFont            = 14.0 // fontSize
	DefaultDividerTextPaddingEm        = 1.0
	DefaultDividerDashOn               = 4.0
	DefaultDividerDashOff              = 4.0
	DefaultDividerDotOn                = 1.0
	DefaultDividerDotOff               = 3.0
)

// DividerOrientation is antd orientation: horizontal | vertical.
type DividerOrientation int

const (
	DividerHorizontal DividerOrientation = iota
	DividerVertical
)

// DividerSize is antd size for horizontal marginBlock.
// Unset (0) → marginLG 24 (same visual as Large).
type DividerSize int

const (
	DividerSizeUnset DividerSize = iota
	DividerSizeSmall
	DividerSizeMedium // also covers antd "middle"
	DividerSizeLarge
)

// DividerVariant is antd variant: solid | dashed | dotted.
type DividerVariant int

const (
	DividerSolid DividerVariant = iota
	DividerDashed
	DividerDotted
)

// DividerTitlePlacement is antd titlePlacement: center | start | end.
type DividerTitlePlacement int

const (
	DividerTitleCenter DividerTitlePlacement = iota
	DividerTitleStart
	DividerTitleEnd
)

// Divider is a product-level separator composed from Flex + primitive.Divider + Text.
//
//	// horizontal plain
//	Flex (pad marginBlock)
//	  └─ Divider rail
//
//	// horizontal with title
//	Flex row CrossCenter
//	  Flexible(start) → rail · content · Flexible(end) → rail
//
//	// vertical
//	Flex → vertical rail (h≈0.9em)
//
// Product contract: docs/antd/divider.md §6 (P0 DoD).
// https://ant.design/components/divider
type Divider struct {
	// Root is the stable mount node; rebuild replaces children only.
	Root *primitive.Flex

	// rail is the primary line (plain horizontal / vertical). With-text keeps
	// railStart / railEnd instead.
	rail      *primitive.Divider
	railStart *primitive.Divider
	railEnd   *primitive.Divider
	label     *primitive.Text
	labelHost *primitive.Box

	// grow factors exposed for tests (titlePlacement).
	growStart, growEnd float64

	Orientation       DividerOrientation
	orientationSet    bool
	Vertical          bool // sugar; Orientation wins when orientationSet
	Size              DividerSize
	Variant           DividerVariant
	Dashed            bool
	Plain             bool
	Title             string
	TitleNode         core.Node // optional; preferred over Title when non-nil
	TitlePlacement    DividerTitlePlacement
	OrientationMargin float64 // 0 → DefaultDividerOrientationMargin (ratio 0..1)
	AriaLabel         string
	Face              text.Face
	Theme             *core.Theme
	// Style optional overrides: Border→line color, Text→title color, FontSize→title size.
	Style Style
}

// NewDivider creates a horizontal solid divider (antd defaults).
func NewDivider() *Divider {
	d := &Divider{
		Orientation:    DividerHorizontal,
		Size:           DividerSizeUnset,
		Variant:        DividerSolid,
		TitlePlacement: DividerTitleCenter,
	}
	d.rebuild()
	return d
}

// NewDividerWithTitle creates a horizontal divider with centered title text.
func NewDividerWithTitle(title string) *Divider {
	d := NewDivider()
	d.SetTitle(title)
	return d
}

// Node returns the root core.Node for tree attachment.
func (d *Divider) Node() core.Node {
	if d == nil {
		return nil
	}
	if d.Root == nil {
		d.rebuild()
	}
	return d.Root
}

// ChromeNode returns the root flex (for visual tests / composition).
func (d *Divider) ChromeNode() core.Node {
	return d.Node()
}

// RailNode returns the primary rail (plain line) or start rail (with-text).
func (d *Divider) RailNode() *primitive.Divider {
	if d == nil {
		return nil
	}
	if d.rail != nil {
		return d.rail
	}
	return d.railStart
}

// LabelNode returns the title text node when present.
func (d *Divider) LabelNode() *primitive.Text { return d.label }

// GrowFactors returns with-text rail flex grow (start, end). Zero when plain.
func (d *Divider) GrowFactors() (start, end float64) {
	if d == nil {
		return 0, 0
	}
	return d.growStart, d.growEnd
}

// SetOrientation sets horizontal/vertical. Takes priority over Vertical sugar.
func (d *Divider) SetOrientation(o DividerOrientation) {
	if d == nil {
		return
	}
	d.Orientation = o
	d.orientationSet = true
	d.rebuild()
}

// SetVertical is antd vertical sugar; ignored when Orientation was Set.
func (d *Divider) SetVertical(v bool) {
	if d == nil {
		return
	}
	d.Vertical = v
	if !d.orientationSet {
		if v {
			d.Orientation = DividerVertical
		} else {
			d.Orientation = DividerHorizontal
		}
	}
	d.rebuild()
}

// SetSize sets horizontal marginBlock ladder (antd size).
func (d *Divider) SetSize(s DividerSize) {
	if d == nil {
		return
	}
	d.Size = s
	d.rebuild()
}

// SetVariant sets solid/dashed/dotted.
func (d *Divider) SetVariant(v DividerVariant) {
	if d == nil {
		return
	}
	d.Variant = v
	d.rebuild()
}

// SetDashed toggles the dashed sugar flag.
func (d *Divider) SetDashed(v bool) {
	if d == nil {
		return
	}
	d.Dashed = v
	d.rebuild()
}

// SetPlain toggles plain body-style title typography.
func (d *Divider) SetPlain(v bool) {
	if d == nil {
		return
	}
	d.Plain = v
	d.rebuild()
}

// SetTitle sets the children title string (empty → pure line).
func (d *Divider) SetTitle(s string) {
	if d == nil {
		return
	}
	d.Title = s
	d.rebuild()
}

// SetText is an alias of SetTitle (legacy name).
func (d *Divider) SetText(s string) { d.SetTitle(s) }

// SetTitleNode sets an optional custom title node (preferred over Title).
func (d *Divider) SetTitleNode(n core.Node) {
	if d == nil {
		return
	}
	d.TitleNode = n
	d.rebuild()
}

// SetTitlePlacement sets start/end/center title position.
func (d *Divider) SetTitlePlacement(p DividerTitlePlacement) {
	if d == nil {
		return
	}
	d.TitlePlacement = p
	d.rebuild()
}

// SetOrientationMargin sets the near-side rail ratio for start/end (0 → 0.05).
func (d *Divider) SetOrientationMargin(ratio float64) {
	if d == nil {
		return
	}
	if ratio < 0 {
		ratio = 0
	}
	d.OrientationMargin = ratio
	d.rebuild()
}

// SetTheme overrides the theme used for Token resolution.
func (d *Divider) SetTheme(th *core.Theme) {
	if d == nil {
		return
	}
	d.Theme = th
	d.rebuild()
}

// SetStyle applies one-off visual overrides.
func (d *Divider) SetStyle(st Style) {
	if d == nil {
		return
	}
	d.Style = st
	d.rebuild()
}

// SetFace sets the optional title font face.
func (d *Divider) SetFace(face text.Face) {
	if d == nil {
		return
	}
	d.Face = face
	d.rebuild()
}

// SetAriaLabel sets an optional accessible name (root Role stays separator).
func (d *Divider) SetAriaLabel(name string) {
	if d == nil {
		return
	}
	d.AriaLabel = name
	d.applyA11y()
}

// EffectiveOrientation returns horizontal/vertical after sugar resolution.
func (d *Divider) EffectiveOrientation() DividerOrientation {
	if d == nil {
		return DividerHorizontal
	}
	if d.orientationSet {
		return d.Orientation
	}
	if d.Vertical || d.Orientation == DividerVertical {
		return DividerVertical
	}
	return DividerHorizontal
}

// IsVertical reports effective vertical orientation.
func (d *Divider) IsVertical() bool {
	return d.EffectiveOrientation() == DividerVertical
}

// EffectiveVariant resolves dotted > dashed|Dashed flag > solid.
func (d *Divider) EffectiveVariant() DividerVariant {
	if d == nil {
		return DividerSolid
	}
	if d.Variant == DividerDotted {
		return DividerDotted
	}
	if d.Variant == DividerDashed || d.Dashed {
		return DividerDashed
	}
	return DividerSolid
}

// MarginBlock returns the horizontal outer vertical padding (antd marginBlock).
// Vertical dividers return 0 (size ignored).
func (d *Divider) MarginBlock() float64 {
	if d == nil || d.IsVertical() {
		return 0
	}
	hasTitle := d.hasTitle()
	switch d.Size {
	case DividerSizeSmall:
		return DefaultDividerMarginSmall
	case DividerSizeMedium:
		return DefaultDividerMarginMedium
	case DividerSizeLarge:
		return DefaultDividerMarginUnset
	default:
		// unset
		if hasTitle {
			return DefaultDividerMarginWithText
		}
		return DefaultDividerMarginUnset
	}
}

// LineWidth returns the resolved line thickness.
func (d *Divider) LineWidth() float64 {
	th := d.theme()
	return th.SizeOr(core.TokenLineWidth, DefaultDividerLineWidth)
}

// LineColor returns the resolved rail color (Token colorSplit + Style.Border).
func (d *Divider) LineColor() render.RGBA {
	if d != nil && d.Style.hasBorder() {
		return d.Style.Border
	}
	th := d.theme()
	if c := th.Color(core.TokenColorSplit); c.A > 0 {
		return c
	}
	if c := th.Color(core.TokenColorBorderSecondary); c.A > 0 {
		return c
	}
	return th.Color(core.TokenColorBorder)
}

// TitleFontSize returns the resolved title font size (0 when no title).
func (d *Divider) TitleFontSize() float64 {
	if d == nil || !d.hasTitle() {
		return 0
	}
	if d.Style.FontSize > 0 {
		return d.Style.FontSize
	}
	th := d.theme()
	if d.Plain {
		return th.SizeOr(core.TokenFontSize, DefaultDividerPlainFont)
	}
	return th.SizeOr(core.TokenFontSizeLG, DefaultDividerTitleFont)
}

func (d *Divider) theme() *core.Theme {
	if d != nil && d.Theme != nil {
		return d.Theme
	}
	return DefaultTheme()
}

func (d *Divider) hasTitle() bool {
	if d == nil {
		return false
	}
	if d.IsVertical() {
		return false // antd: children ignored in vertical mode
	}
	return d.TitleNode != nil || d.Title != ""
}

func (d *Divider) orientationMarginRatio() float64 {
	if d != nil && d.OrientationMargin > 0 {
		if d.OrientationMargin > 1 {
			// absolute px path is P1; treat as ratio clamp
			return DefaultDividerOrientationMargin
		}
		return d.OrientationMargin
	}
	return DefaultDividerOrientationMargin
}

func (d *Divider) dashPattern() []float64 {
	switch d.EffectiveVariant() {
	case DividerDashed:
		return []float64{DefaultDividerDashOn, DefaultDividerDashOff}
	case DividerDotted:
		return []float64{DefaultDividerDotOn, DefaultDividerDotOff}
	default:
		return nil
	}
}

func (d *Divider) applyRail(r *primitive.Divider, vertical bool, length float64) {
	if r == nil {
		return
	}
	r.Vertical = vertical
	r.Thickness = d.LineWidth()
	r.Length = length
	r.Color = d.LineColor()
	r.ColorToken = core.TokenColorSplit
	if d.Style.hasBorder() {
		// Style override: solid color, drop token so Color wins.
		r.ColorToken = ""
		r.Color = d.Style.Border
	}
	r.Dash = d.dashPattern()
	r.Margin = primitive.EdgeInsets{}
}

func (d *Divider) applyA11y() {
	if d == nil || d.Root == nil {
		return
	}
	d.Root.Base().Role = "separator"
	d.Root.Base().Label = d.AriaLabel
	d.Root.Hit = core.HitTransparent
}

// rebuild reconstructs the visual tree from product fields + tokens.
func (d *Divider) rebuild() {
	if d == nil {
		return
	}
	if d.Root == nil {
		d.Root = primitive.NewFlex(core.AxisVertical)
		d.Root.Hit = core.HitTransparent
	}
	// Clear previous children / refs.
	d.Root.ClearChildren()
	d.rail = nil
	d.railStart = nil
	d.railEnd = nil
	d.label = nil
	d.labelHost = nil
	d.growStart, d.growEnd = 0, 0

	d.Root.Axis = core.AxisVertical
	d.Root.MainAlign = core.MainStart
	d.Root.CrossAlign = core.CrossStretch
	d.Root.Gap = 0
	d.Root.Padding = primitive.EdgeInsets{}
	d.Root.Wrap = false

	if d.IsVertical() {
		d.buildVertical()
	} else if d.hasTitle() {
		d.buildHorizontalWithTitle()
	} else {
		d.buildHorizontalPlain()
	}
	d.applyA11y()
	d.Root.MarkNeedsLayout()
	d.Root.MarkNeedsPaint()
}

func (d *Divider) buildHorizontalPlain() {
	mb := d.MarginBlock()
	d.Root.Axis = core.AxisVertical
	d.Root.CrossAlign = core.CrossStretch
	d.Root.Padding = primitive.EdgeInsets{Top: mb, Bottom: mb}

	rail := primitive.NewDivider()
	d.applyRail(rail, false, 0)
	d.rail = rail
	d.Root.AddChild(rail)
}

func (d *Divider) buildHorizontalWithTitle() {
	mb := d.MarginBlock()
	// With-text: CrossCenter row; size still drives marginBlock.
	d.Root.Axis = core.AxisHorizontal
	d.Root.CrossAlign = core.CrossCenter
	d.Root.MainAlign = core.MainStart
	d.Root.Padding = primitive.EdgeInsets{Top: mb, Bottom: mb}

	ratio := d.orientationMarginRatio()
	switch d.TitlePlacement {
	case DividerTitleStart:
		d.growStart, d.growEnd = ratio, 1-ratio
	case DividerTitleEnd:
		d.growStart, d.growEnd = 1-ratio, ratio
	default:
		d.growStart, d.growEnd = 1, 1
	}

	startRail := primitive.NewDivider()
	d.applyRail(startRail, false, 0)
	endRail := primitive.NewDivider()
	d.applyRail(endRail, false, 0)
	d.railStart, d.railEnd = startRail, endRail

	startHost := primitive.NewFlexible(d.growStart, startRail)
	startHost.FillChild = true
	endHost := primitive.NewFlexible(d.growEnd, endRail)
	endHost.FillChild = true

	content := d.buildTitleContent()
	d.Root.AddChild(startHost)
	d.Root.AddChild(content)
	d.Root.AddChild(endHost)
}

func (d *Divider) buildTitleContent() core.Node {
	if d.TitleNode != nil {
		// Custom node: still pad inline ≈ 1em of resolved title font.
		fs := d.TitleFontSize()
		if fs <= 0 {
			fs = DefaultDividerTitleFont
		}
		pad := fs * DefaultDividerTextPaddingEm
		box := primitive.NewBox(d.TitleNode)
		box.Padding = primitive.Symmetric(pad, 0)
		d.labelHost = box
		return box
	}

	fs := d.TitleFontSize()
	lab := primitive.NewText(d.Title)
	lab.FontSize = fs
	lab.Face = d.Face
	if d.Style.Face != nil {
		lab.Face = d.Style.Face
	}
	if d.Style.hasText() {
		lab.Color = d.Style.Text
	} else {
		lab.Color = d.theme().Color(core.TokenColorText)
	}
	d.label = lab

	pad := fs * DefaultDividerTextPaddingEm
	box := primitive.NewBox(lab)
	box.Padding = primitive.Symmetric(pad, 0)
	d.labelHost = box
	return box
}

func (d *Divider) buildVertical() {
	th := d.theme()
	font := th.SizeOr(core.TokenFontSize, DefaultDividerPlainFont)
	h := font * DefaultDividerVerticalEm
	mi := DefaultDividerVerticalMarginInline

	d.Root.Axis = core.AxisHorizontal
	d.Root.CrossAlign = core.CrossCenter
	d.Root.MainAlign = core.MainCenter
	d.Root.Padding = primitive.EdgeInsets{Left: mi, Right: mi}

	rail := primitive.NewVerticalDivider()
	d.applyRail(rail, true, h)
	d.rail = rail
	d.Root.AddChild(rail)
}
