package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Space defaults — components/space/style/index.ts
// spaceGapSmall/Middle/Large = paddingXS/padding/paddingLG → 8/16/24.
// docs/antd/space.md §6.2 / §6.10
// Default size is **small** (not middle).
const (
	DefaultSpaceGapSmall     = 8.0
	DefaultSpaceGapMiddle    = 16.0
	DefaultSpaceGapLarge     = 24.0
	DefaultSpaceFontSize     = 14.0
	DefaultSpaceBorderRadius = 6.0
	DefaultSpaceLineWidth    = 1.0
)

// SpaceOrientation is antd orientation: horizontal | vertical.
type SpaceOrientation int

const (
	SpaceHorizontal SpaceOrientation = iota
	SpaceVertical
)

// SpaceSize is antd size preset: small | middle(medium) | large.
// Zero value is SpaceSizeSmall — antd default size="small".
type SpaceSize int

const (
	SpaceSizeSmall  SpaceSize = iota
	SpaceSizeMiddle           // also covers antd "medium"
	SpaceSizeLarge
)

// SpaceAlign is antd align: start | end | center | baseline.
// SpaceAlignAuto follows antd: horizontal → center; vertical → start (no force).
type SpaceAlign int

const (
	SpaceAlignAuto SpaceAlign = iota
	SpaceAlignStart
	SpaceAlignEnd
	SpaceAlignCenter
	SpaceAlignBaseline // P0: mapped to CrossStart (core has no baseline)
)

// Space is Ant Design Space: inline-flex gap between children.
//
//	primitive.Flex (axis / gap / wrap / cross)
//	  └─ [child · separator? · child · …]
//
// Product contract: docs/antd/space.md §6 (P0 DoD).
// https://ant.design/components/space
//
// Layout-only: no press / disabled / loading chrome. hit == layout == paint.
type Space struct {
	// Root is the stable mount node; apply() mutates layout fields in place.
	Root *primitive.Flex

	Orientation    SpaceOrientation
	orientationSet bool
	// Vertical is antd vertical sugar; ignored when Orientation was Set.
	Vertical bool
	Wrap     bool
	Align    SpaceAlign
	// Size preset; overridden by SetSizePx / SetSizeXY when sizeMode is custom.
	Size SpaceSize
	// SizePx is the last numeric gap (meaningful when sizeMode==spaceSizePx).
	SizePx float64
	// SizeRow is the secondary (row) gap when sizeMode==spaceSizeXY (wrap cross gap approx).
	SizeRow  float64
	sizeMode spaceSizeMode

	// Separator factory; called once per inter-item gap so nodes are not double-parented.
	Separator func() core.Node

	// children holds the product children (without injected separators).
	children  []core.Node
	AriaLabel string
	Theme     *core.Theme
	// ExpandMax makes the root block-level (fill parent MaxWidth), equivalent to
	// antd style={{ display: 'flex' }} vs default inline-flex. Needed so nested
	// Compact block children resolve width against a finite parent (SPC-16).
	ExpandMax bool
}

type spaceSizeMode int

const (
	spaceSizePreset spaceSizeMode = iota
	spaceSizePx
	spaceSizeXY
)

// NewSpace creates a horizontal Space (antd defaults: size=small, align=center) with optional children.
func NewSpace(children ...core.Node) *Space {
	s := &Space{
		Orientation: SpaceHorizontal,
		Align:       SpaceAlignAuto,
		Size:        SpaceSizeSmall,
		sizeMode:    spaceSizePreset,
	}
	s.Root = primitive.Row()
	s.Root.Hit = core.HitDefer
	// Space is inline-flex: hug content (unlike kit.Flex block ExpandMax).
	s.Root.ExpandMax = false
	s.children = filterNilNodes(children)
	s.rebuildChildren()
	s.apply()
	return s
}

// Node returns the root core.Node for tree attachment.
func (s *Space) Node() core.Node {
	if s == nil {
		return nil
	}
	if s.Root == nil {
		s.Root = primitive.Row()
		s.Root.Hit = core.HitDefer
	}
	s.apply()
	return s.Root
}

// ChromeNode returns the layout root (for visual tests / composition).
func (s *Space) ChromeNode() core.Node { return s.Node() }

// SetOrientation sets horizontal/vertical. Takes priority over Vertical sugar.
func (s *Space) SetOrientation(o SpaceOrientation) {
	if s == nil {
		return
	}
	s.Orientation = o
	s.orientationSet = true
	s.apply()
}

// SetVertical is antd vertical sugar; ignored when Orientation was Set.
func (s *Space) SetVertical(v bool) {
	if s == nil {
		return
	}
	s.Vertical = v
	if !s.orientationSet {
		if v {
			s.Orientation = SpaceVertical
		} else {
			s.Orientation = SpaceHorizontal
		}
	}
	s.apply()
}

// SetAlign sets cross-axis alignment (Auto → antd default by orientation).
func (s *Space) SetAlign(a SpaceAlign) {
	if s == nil {
		return
	}
	s.Align = a
	s.apply()
}

// SetSize sets preset gap (small/middle/large). Clears custom px/xy.
func (s *Space) SetSize(sz SpaceSize) {
	if s == nil {
		return
	}
	s.Size = sz
	s.sizeMode = spaceSizePreset
	s.apply()
}

// SetSizePx sets a numeric gap in logical px (antd size={n}). Explicit, including 0.
// Breaking: previously SetSize(float64); callers must migrate.
func (s *Space) SetSizePx(px float64) {
	if s == nil {
		return
	}
	s.SizePx = px
	s.sizeMode = spaceSizePx
	s.apply()
}

// SetSizeXY sets antd size={[columnGap, rowGap]}.
// Main-axis gap uses the axis-appropriate value; wrap cross gap uses row when horizontal.
func (s *Space) SetSizeXY(col, row float64) {
	if s == nil {
		return
	}
	s.SizePx = col
	s.SizeRow = row
	s.sizeMode = spaceSizeXY
	s.apply()
}

// SetWrap enables multi-line packing when horizontal and main axis is bounded.
// Vertical orientation ignores wrap (antd: wrap only for horizontal).
func (s *Space) SetWrap(v bool) {
	if s == nil {
		return
	}
	s.Wrap = v
	s.apply()
}

// SetSeparator sets a factory that builds a separator node between each pair of children.
// Pass nil to clear. Factory is preferred over a single node to avoid dual-parenting.
func (s *Space) SetSeparator(fn func() core.Node) {
	if s == nil {
		return
	}
	s.Separator = fn
	s.rebuildChildren()
	s.apply()
}

// SetTheme sets the theme used for preset gap token resolution.
func (s *Space) SetTheme(th *core.Theme) {
	if s == nil {
		return
	}
	s.Theme = th
	s.apply()
}

// SetAriaLabel sets an optional accessible name on the layout root.
func (s *Space) SetAriaLabel(name string) {
	if s == nil {
		return
	}
	s.AriaLabel = name
	s.applyA11y()
}

// SetExpandMax toggles block-level fill (antd display:flex vs inline-flex).
func (s *Space) SetExpandMax(v bool) {
	if s == nil {
		return
	}
	s.ExpandMax = v
	s.apply()
}

// Add appends a product child (rebuilds separator slots).
func (s *Space) Add(n core.Node) {
	if s == nil || n == nil {
		return
	}
	s.children = append(s.children, n)
	s.rebuildChildren()
	s.apply()
}

// SetChildren replaces all product children.
func (s *Space) SetChildren(children ...core.Node) {
	if s == nil {
		return
	}
	s.children = filterNilNodes(children)
	s.rebuildChildren()
	s.apply()
}

// ClearChildren removes all product children.
func (s *Space) ClearChildren() {
	if s == nil {
		return
	}
	s.children = nil
	s.rebuildChildren()
	s.apply()
}

// Children returns a copy of product children (without separators).
func (s *Space) Children() []core.Node {
	if s == nil {
		return nil
	}
	out := make([]core.Node, len(s.children))
	copy(out, s.children)
	return out
}

// IsVertical reports effective vertical main axis.
func (s *Space) IsVertical() bool {
	return s != nil && s.EffectiveOrientation() == SpaceVertical
}

// EffectiveOrientation returns orientation after sugar resolution.
func (s *Space) EffectiveOrientation() SpaceOrientation {
	if s == nil {
		return SpaceHorizontal
	}
	if s.orientationSet {
		return s.Orientation
	}
	if s.Vertical {
		return SpaceVertical
	}
	return s.Orientation
}

// ResolvedGap returns the effective main-axis gap in px (§6.2: 8/16/24 or custom).
func (s *Space) ResolvedGap() float64 {
	if s == nil {
		return DefaultSpaceGapSmall
	}
	switch s.sizeMode {
	case spaceSizePx:
		return s.SizePx
	case spaceSizeXY:
		if s.IsVertical() {
			return s.SizeRow
		}
		return s.SizePx
	default:
		th := s.theme()
		switch s.Size {
		case SpaceSizeMiddle:
			return th.SizeOr(core.TokenPadding, DefaultSpaceGapMiddle)
		case SpaceSizeLarge:
			return th.SizeOr(core.TokenPaddingLG, DefaultSpaceGapLarge)
		default: // Small
			// antd paddingXS = sizeXS = 8; kit TokenPaddingXS is 4 — do not use it.
			return DefaultSpaceGapSmall
		}
	}
}

// ResolvedCrossGap returns the wrap cross-axis gap when size is [col,row]; else same as ResolvedGap.
func (s *Space) ResolvedCrossGap() float64 {
	if s == nil {
		return DefaultSpaceGapSmall
	}
	if s.sizeMode == spaceSizeXY {
		if s.IsVertical() {
			return s.SizePx
		}
		return s.SizeRow
	}
	return s.ResolvedGap()
}

// ResolvedAlign maps to core.CrossAxisAlignment (Auto → antd default).
func (s *Space) ResolvedAlign() core.CrossAxisAlignment {
	if s == nil {
		return core.CrossCenter
	}
	switch s.Align {
	case SpaceAlignStart:
		return core.CrossStart
	case SpaceAlignEnd:
		return core.CrossEnd
	case SpaceAlignCenter:
		return core.CrossCenter
	case SpaceAlignBaseline:
		// core has no baseline; start is the safe P0 approximation for text rows.
		return core.CrossStart
	default: // Auto
		// antd: align === undefined && !vertical → center; vertical leaves undefined → start.
		if s.IsVertical() {
			return core.CrossStart
		}
		return core.CrossCenter
	}
}

func (s *Space) theme() *core.Theme {
	if s != nil && s.Theme != nil {
		return s.Theme
	}
	return DefaultTheme()
}

func (s *Space) apply() {
	if s == nil {
		return
	}
	if s.Root == nil {
		s.Root = primitive.Row()
		s.Root.Hit = core.HitDefer
	}
	s.Root.ExpandMax = s.ExpandMax
	if s.IsVertical() {
		s.Root.Axis = core.AxisVertical
	} else {
		s.Root.Axis = core.AxisHorizontal
	}
	// When separator is present it is an extra flex item; gap still applies around it
	// (antd flex gap + separator node).
	s.Root.Gap = s.ResolvedGap()
	// wrap only for horizontal (antd)
	s.Root.Wrap = s.Wrap && !s.IsVertical()
	s.Root.MainAlign = core.MainStart
	s.Root.CrossAlign = s.ResolvedAlign()
	s.Root.MarkNeedsLayout()
	s.applyA11y()
}

func (s *Space) applyA11y() {
	if s == nil || s.Root == nil {
		return
	}
	// Layout container: optional name only; no role/keyboard (not interactive).
	s.Root.Base().Label = s.AriaLabel
}

// rebuildChildren mounts product children and optional separators into Root.
func (s *Space) rebuildChildren() {
	if s == nil {
		return
	}
	if s.Root == nil {
		s.Root = primitive.Row()
		s.Root.Hit = core.HitDefer
	}
	s.Root.ClearChildren()
	n := len(s.children)
	for i, c := range s.children {
		if c == nil {
			continue
		}
		s.Root.AddChild(c)
		if s.Separator != nil && i < n-1 {
			sep := s.Separator()
			if sep != nil {
				// Decorative separator: no focus / a11y name.
				if base := sep.Base(); base != nil {
					if base.Label == "" {
						// leave empty; pure decoration
					}
				}
				s.Root.AddChild(sep)
			}
		}
	}
}

func filterNilNodes(in []core.Node) []core.Node {
	if len(in) == 0 {
		return nil
	}
	out := make([]core.Node, 0, len(in))
	for _, c := range in {
		if c != nil {
			out = append(out, c)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Space.Compact — tight connected controls (merged borders).
// ---------------------------------------------------------------------------

// SpaceCompact is Ant Design Space.Compact: inline-flex of connected items.
//
// P0 approximation without per-corner radius:
//   - gap = -lineWidth so adjacent borders overlap (no double seam)
//   - middle Button items get ForceRadius + Radius 0
//   - first/last Buttons keep default radius (uniform); visual is "no double border"
//
// Product contract: docs/antd/space.md §6.8 compact / compact-buttons.
type SpaceCompact struct {
	Root *primitive.Flex

	Orientation    SpaceOrientation
	orientationSet bool
	Vertical       bool
	Block          bool
	// Size is component size passed to compact-aware children (Button).
	Size  ButtonSize
	Theme *core.Theme

	// children product items (Button / Input / Addon …).
	children []core.Node
	// buttons tracks *Button among children for compact chrome.
	buttons []*Button
	// addons tracks *SpaceAddon for size / theme push from Compact.
	addons []*SpaceAddon
}

// NewSpaceCompact creates a horizontal compact group.
func NewSpaceCompact(children ...core.Node) *SpaceCompact {
	c := &SpaceCompact{
		Orientation: SpaceHorizontal,
		Size:        ButtonMiddle,
	}
	c.Root = primitive.Row()
	c.Root.Hit = core.HitDefer
	c.Root.ExpandMax = false
	c.SetChildren(children...)
	return c
}

// Node returns the compact root.
func (c *SpaceCompact) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	c.apply()
	return c.Root
}

// ChromeNode returns the layout root.
func (c *SpaceCompact) ChromeNode() core.Node { return c.Node() }

// SetOrientation sets compact direction; wins over Vertical.
func (c *SpaceCompact) SetOrientation(o SpaceOrientation) {
	if c == nil {
		return
	}
	c.Orientation = o
	c.orientationSet = true
	c.apply()
}

// SetVertical is sugar; ignored when Orientation was Set.
func (c *SpaceCompact) SetVertical(v bool) {
	if c == nil {
		return
	}
	c.Vertical = v
	if !c.orientationSet {
		if v {
			c.Orientation = SpaceVertical
		} else {
			c.Orientation = SpaceHorizontal
		}
	}
	c.apply()
}

// SetBlock makes the compact group fill parent width.
func (c *SpaceCompact) SetBlock(v bool) {
	if c == nil {
		return
	}
	c.Block = v
	c.apply()
}

// SetSize sets the compact item size (antd Compact size → child control size).
func (c *SpaceCompact) SetSize(sz ButtonSize) {
	if c == nil {
		return
	}
	c.Size = sz
	c.restyleCompactItems()
	c.apply()
}

// SetTheme sets theme for lineWidth gap and compact-aware children.
func (c *SpaceCompact) SetTheme(th *core.Theme) {
	if c == nil {
		return
	}
	c.Theme = th
	c.restyleCompactItems()
	c.apply()
}

// Add appends a child.
func (c *SpaceCompact) Add(n core.Node) {
	if c == nil || n == nil {
		return
	}
	c.children = append(c.children, n)
	c.remount()
	c.apply()
}

// IsVertical reports effective orientation.
func (c *SpaceCompact) IsVertical() bool {
	return c != nil && c.EffectiveOrientation() == SpaceVertical
}

// EffectiveOrientation after sugar.
func (c *SpaceCompact) EffectiveOrientation() SpaceOrientation {
	if c == nil {
		return SpaceHorizontal
	}
	if c.orientationSet {
		return c.Orientation
	}
	if c.Vertical {
		return SpaceVertical
	}
	return c.Orientation
}

// OverlapGap is the negative gap that merges adjacent borders (≈ -lineWidth).
func (c *SpaceCompact) OverlapGap() float64 {
	th := DefaultTheme()
	if c != nil && c.Theme != nil {
		th = c.Theme
	}
	lw := th.SizeOr(core.TokenLineWidth, DefaultSpaceLineWidth)
	if lw <= 0 {
		lw = DefaultSpaceLineWidth
	}
	return -lw
}

func (c *SpaceCompact) remount() {
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	c.Root.ClearChildren()
	for _, n := range c.children {
		if n == nil {
			continue
		}
		c.Root.AddChild(n)
	}
}

// SetChildren replaces children. Button/Addon compact chrome is cleared;
// use SetButtons / AddAddon / AddButton for compact-aware items.
func (c *SpaceCompact) SetChildren(children ...core.Node) {
	if c == nil {
		return
	}
	c.children = filterNilNodes(children)
	c.buttons = nil
	c.addons = nil
	c.remount()
	c.apply()
}

// ClearChildren removes all children.
func (c *SpaceCompact) ClearChildren() {
	if c == nil {
		return
	}
	c.children = nil
	c.buttons = nil
	c.addons = nil
	if c.Root != nil {
		c.Root.ClearChildren()
	}
	c.apply()
}

// AddButton appends a Button and registers it for compact radius treatment.
func (c *SpaceCompact) AddButton(b *Button) {
	if c == nil || b == nil {
		return
	}
	c.children = append(c.children, b.Node())
	c.buttons = append(c.buttons, b)
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	c.Root.AddChild(b.Node())
	c.restyleCompactItems()
	c.apply()
}

// AddAddon appends a SpaceAddon and registers it for compact size push.
func (c *SpaceCompact) AddAddon(a *SpaceAddon) {
	if c == nil || a == nil {
		return
	}
	c.children = append(c.children, a.Node())
	c.addons = append(c.addons, a)
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	c.Root.AddChild(a.Node())
	c.restyleCompactItems()
	c.apply()
}

// SetButtons replaces children with the given Buttons (compact chrome applied).
func (c *SpaceCompact) SetButtons(buttons ...*Button) {
	if c == nil {
		return
	}
	c.children = nil
	c.buttons = nil
	c.addons = nil
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	c.Root.ClearChildren()
	for _, b := range buttons {
		if b == nil {
			continue
		}
		c.children = append(c.children, b.Node())
		c.buttons = append(c.buttons, b)
		c.Root.AddChild(b.Node())
	}
	c.restyleCompactItems()
	c.apply()
}

// restyleCompactItems pushes Size/Theme to compact-aware children (Button, Addon).
// This is the general Compact → item contract (antd useCompactItemContext).
func (c *SpaceCompact) restyleCompactItems() {
	if c == nil {
		return
	}
	nBtn := len(c.buttons)
	for i, b := range c.buttons {
		if b == nil {
			continue
		}
		b.SetSize(c.Size)
		// Middle items: square corners so overlapping borders read as one bar.
		// First/last keep theme radius (P0 without per-corner radii).
		if nBtn > 1 && i > 0 && i < nBtn-1 {
			st := b.Style
			st.Radius = 0
			st.ForceRadius = true
			b.SetStyle(st)
		} else if nBtn > 1 {
			st := b.Style
			if st.ForceRadius && st.Radius == 0 {
				st.ForceRadius = false
				b.SetStyle(st)
			}
		}
	}
	for _, a := range c.addons {
		if a == nil {
			continue
		}
		if c.Theme != nil {
			a.SetTheme(c.Theme)
		}
		a.SetSize(c.Size)
	}
}

func (c *SpaceCompact) apply() {
	if c == nil {
		return
	}
	if c.Root == nil {
		c.Root = primitive.Row()
		c.Root.Hit = core.HitDefer
	}
	if c.IsVertical() {
		c.Root.Axis = core.AxisVertical
	} else {
		c.Root.Axis = core.AxisHorizontal
	}
	c.Root.Gap = c.OverlapGap()
	c.Root.Wrap = false
	c.Root.MainAlign = core.MainStart
	// Compact row centers items of unequal intrinsic height on the cross axis
	// (antd align-items default for connected controls).
	c.Root.CrossAlign = core.CrossCenter
	// block → CSS display:flex; width 100%
	c.Root.ExpandMax = c.Block
	c.Root.MarkNeedsLayout()
}

// ---------------------------------------------------------------------------
// Space.Addon — custom cell inside Compact (antd Space.Addon).
// ---------------------------------------------------------------------------

// SpaceAddon is Ant Design Space.Addon: a non-interactive cell in Compact.
//
// Structure mirrors antd CSS (style/addon.ts):
//
//	Decorated  — border / bg / radius / controlHeight (size)
//	  └─ Flex(Row) CrossCenter Gap=0  — display:inline-flex; align-items:center
//	       └─ children… (text / icon / multi-node; not $ -specific)
//
// StretchChild fills the fixed control height so Flex has a real cross size to
// center into — works for single glyph, multi-child, and size small|middle|large.
//
// Product contract: docs/antd/space.md §6.8 compact; SPC-16.
type SpaceAddon struct {
	Root *primitive.Decorated
	row  *primitive.Flex

	// Size follows Compact size (antd compactSize → -small / default / -large).
	Size ButtonSize
	// Disabled dims text/bg (antd -disabled).
	Disabled bool
	Theme    *core.Theme

	children []core.Node
}

// NewSpaceAddon wraps optional children as a compact addon cell.
func NewSpaceAddon(children ...core.Node) *SpaceAddon {
	a := &SpaceAddon{
		Size:     ButtonMiddle,
		children: filterNilNodes(children),
	}
	a.rebuild()
	return a
}

// Node returns the addon chrome.
func (a *SpaceAddon) Node() core.Node {
	if a == nil {
		return nil
	}
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// ChromeNode returns the layout root.
func (a *SpaceAddon) ChromeNode() core.Node { return a.Node() }

// SetChild replaces content with a single child (convenience).
func (a *SpaceAddon) SetChild(n core.Node) {
	if a == nil {
		return
	}
	if n == nil {
		a.children = nil
	} else {
		a.children = []core.Node{n}
	}
	a.rebuild()
}

// SetChildren replaces all content nodes (multi-child / icon+text).
func (a *SpaceAddon) SetChildren(children ...core.Node) {
	if a == nil {
		return
	}
	a.children = filterNilNodes(children)
	a.rebuild()
}

// Add appends a content child.
func (a *SpaceAddon) Add(n core.Node) {
	if a == nil || n == nil {
		return
	}
	a.children = append(a.children, n)
	a.rebuild()
}

// SetSize sets control size (small/middle/large → height, pad, radius).
func (a *SpaceAddon) SetSize(sz ButtonSize) {
	if a == nil {
		return
	}
	a.Size = sz
	a.rebuild()
}

// SetDisabled toggles disabled chrome.
func (a *SpaceAddon) SetDisabled(v bool) {
	if a == nil {
		return
	}
	a.Disabled = v
	a.rebuild()
}

// SetTheme updates token-backed chrome.
func (a *SpaceAddon) SetTheme(th *core.Theme) {
	if a == nil {
		return
	}
	a.Theme = th
	a.rebuild()
}

func (a *SpaceAddon) theme() *core.Theme {
	if a != nil && a.Theme != nil {
		return a.Theme
	}
	return DefaultTheme()
}

// metrics resolves height / paddingInline / radius for Size (§6.2 + antd addon.ts).
func (a *SpaceAddon) metrics() (height, padInline, radius, fontSize float64) {
	th := a.theme()
	switch a.Size {
	case ButtonSmall:
		// antd -small: paddingInline paddingXS, radiusSM, fontSizeSM; height controlHeightSM
		height = th.SizeOr(core.TokenControlHeightSM, 24)
		padInline = th.SizeOr(core.TokenPaddingXS, 8) // antd paddingXS; kit TokenPaddingXS may be 4
		if padInline < 7 {
			padInline = 7 // match controlPaddingInlineSM rhythm
		}
		radius = th.SizeOr(core.TokenBorderRadiusSM, 4)
		fontSize = th.SizeOr(core.TokenFontSizeSM, 12)
	case ButtonLarge:
		height = th.SizeOr(core.TokenControlHeightLG, 40)
		padInline = th.SizeOr(core.TokenPaddingSM, 12)
		radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
		fontSize = th.SizeOr(core.TokenFontSizeLG, 16)
	default: // middle
		height = th.SizeOr(core.TokenControlHeight, 32)
		padInline = th.SizeOr(core.TokenPaddingSM, 12)
		radius = th.SizeOr(core.TokenBorderRadius, 6)
		fontSize = th.SizeOr(core.TokenFontSize, 14)
	}
	return height, padInline, radius, fontSize
}

func (a *SpaceAddon) rebuild() {
	if a == nil {
		return
	}
	th := a.theme()
	height, padInline, radius, _ := a.metrics()
	lineW := th.SizeOr(core.TokenLineWidth, DefaultSpaceLineWidth)

	if a.row == nil {
		a.row = primitive.Row()
		a.row.Hit = core.HitDefer
	}
	a.row.ClearChildren()
	for _, c := range a.children {
		if c != nil {
			a.row.AddChild(c)
		}
	}
	// antd: display:inline-flex; align-items:center; gap:0
	a.row.Axis = core.AxisHorizontal
	a.row.MainAlign = core.MainStart
	a.row.CrossAlign = core.CrossCenter
	a.row.Gap = 0
	a.row.Wrap = false
	a.row.ExpandMax = false

	if a.Root == nil {
		a.Root = primitive.NewDecorated()
	}
	a.Root.ClearChildren()
	a.Root.AddChild(a.row)
	a.Root.Hit = core.HitDefer
	a.Root.Padding = primitive.EdgeInsets{Left: padInline, Right: padInline, Top: 0, Bottom: 0}
	a.Root.MinHeight = height
	a.Root.Height = height
	a.Root.Radius = radius
	a.Root.BorderWidth = lineW
	// Stretch inner Flex to full content box so CrossCenter has a real cross size
	// (generic multi-child path; not single-glyph special case).
	a.Root.StretchChild = true
	a.Root.CenterContent = false // flex owns alignment
	a.Root.ExpandWidth = false

	if a.Disabled {
		a.Root.BorderColor = th.Color(core.TokenColorBorder)
		a.Root.Background = th.Color(core.TokenColorDisabledBg)
	} else {
		// antd default addon: border colorBorder, bg colorBgContainerDisabled (fill secondary)
		a.Root.BorderColor = th.Color(core.TokenColorBorder)
		a.Root.Background = th.Color(core.TokenColorFillSecondary)
	}
	a.Root.MarkNeedsLayout()
	a.row.MarkNeedsLayout()
}
