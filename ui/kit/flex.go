package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Flex defaults — components/flex/style/index.ts
// flexGapSM/MD/LG = paddingXS/padding/paddingLG → 8/16/24 (sizeXS/size/sizeLG).
// docs/antd/flex.md §6.2 / §6.10
const (
	DefaultFlexGapSmall     = 8.0
	DefaultFlexGapMedium    = 16.0
	DefaultFlexGapLarge     = 24.0
	DefaultFlexFontSize     = 14.0
	DefaultFlexBorderRadius = 6.0
	DefaultFlexLineWidth    = 1.0
)

// FlexOrientation is antd orientation: horizontal | vertical.
type FlexOrientation int

const (
	FlexHorizontal FlexOrientation = iota
	FlexVertical
)

// FlexGapSize is antd gap preset: unset | small | medium(middle) | large.
type FlexGapSize int

const (
	FlexGapUnset FlexGapSize = iota
	FlexGapSmall
	FlexGapMedium // also covers antd "middle"
	FlexGapLarge
)

// FlexJustify is antd justify-content (P0 subset).
type FlexJustify int

const (
	FlexJustifyStart FlexJustify = iota // normal / flex-start / start
	FlexJustifyCenter
	FlexJustifyEnd
	FlexJustifySpaceBetween
	FlexJustifySpaceAround
	FlexJustifySpaceEvenly
)

// FlexAlign is antd align-items (P0 subset).
// FlexAlignAuto follows antd utils: horizontal → start, vertical → stretch.
type FlexAlign int

const (
	FlexAlignAuto FlexAlign = iota
	FlexAlignStart
	FlexAlignCenter
	FlexAlignEnd
	FlexAlignStretch
)

// Flex is a product-level layout container composed from primitive.Flex.
//
//	primitive.Flex (axis / gap / wrap / main / cross)
//	  └─ children…
//
// Product contract: docs/antd/flex.md §6 (P0 DoD).
// https://ant.design/components/flex
//
// Layout-only: no press / disabled / loading chrome. hit == layout == paint via primitive.Flex.
type Flex struct {
	// Root is the stable mount node; apply() mutates layout fields in place.
	Root *primitive.Flex

	Orientation    FlexOrientation
	orientationSet bool
	// Vertical is antd vertical sugar; ignored when Orientation was Set.
	Vertical bool
	Wrap     bool
	Justify  FlexJustify
	Align    FlexAlign
	// GapSize preset; overridden by SetGap when gapCustom.
	GapSize FlexGapSize
	// GapPx is the last SetGap numeric value (meaningful when gapCustom).
	GapPx     float64
	gapCustom bool
	AriaLabel string
	Theme     *core.Theme
}

// NewFlex creates a horizontal flex (antd defaults) with optional children.
func NewFlex(children ...core.Node) *Flex {
	f := &Flex{
		Orientation: FlexHorizontal,
		Justify:     FlexJustifyStart,
		Align:       FlexAlignAuto,
		GapSize:     FlexGapUnset,
	}
	f.Root = primitive.Row()
	f.Root.Hit = core.HitDefer
	// Ant Design Flex is a block-level flex container (width fills parent).
	// Required for justify free-space under ScrollViewport/Slot loose constraints.
	f.Root.ExpandMax = true
	for _, c := range children {
		if c != nil {
			f.Root.AddChild(c)
		}
	}
	f.apply()
	return f
}

// Node returns the root core.Node for tree attachment.
func (f *Flex) Node() core.Node {
	if f == nil {
		return nil
	}
	if f.Root == nil {
		f.Root = primitive.Row()
		f.Root.Hit = core.HitDefer
		f.Root.ExpandMax = true
	}
	f.apply()
	return f.Root
}

// ChromeNode returns the layout root (for visual tests / composition).
func (f *Flex) ChromeNode() core.Node { return f.Node() }

// SetOrientation sets horizontal/vertical. Takes priority over Vertical sugar.
func (f *Flex) SetOrientation(o FlexOrientation) {
	if f == nil {
		return
	}
	f.Orientation = o
	f.orientationSet = true
	f.apply()
}

// SetVertical is antd vertical sugar; ignored when Orientation was Set.
func (f *Flex) SetVertical(v bool) {
	if f == nil {
		return
	}
	f.Vertical = v
	if !f.orientationSet {
		if v {
			f.Orientation = FlexVertical
		} else {
			f.Orientation = FlexHorizontal
		}
	}
	f.apply()
}

// SetWrap enables multi-line packing when the main axis is bounded.
func (f *Flex) SetWrap(v bool) {
	if f == nil {
		return
	}
	f.Wrap = v
	f.apply()
}

// SetJustify sets main-axis alignment.
func (f *Flex) SetJustify(j FlexJustify) {
	if f == nil {
		return
	}
	f.Justify = j
	f.apply()
}

// SetAlign sets cross-axis alignment (Auto → antd default by orientation).
func (f *Flex) SetAlign(a FlexAlign) {
	if f == nil {
		return
	}
	f.Align = a
	f.apply()
}

// SetGapSize sets preset gap (small/medium/large/unset). Clears custom gap.
func (f *Flex) SetGapSize(s FlexGapSize) {
	if f == nil {
		return
	}
	f.GapSize = s
	f.gapCustom = false
	f.apply()
}

// SetGap sets a numeric gap in logical px (antd gap={n}). Explicit, including 0.
func (f *Flex) SetGap(px float64) {
	if f == nil {
		return
	}
	f.GapPx = px
	f.gapCustom = true
	f.apply()
}

// SetTheme sets the theme used for preset gap token resolution.
func (f *Flex) SetTheme(th *core.Theme) {
	if f == nil {
		return
	}
	f.Theme = th
	f.apply()
}

// SetAriaLabel sets an optional accessible name on the layout root.
func (f *Flex) SetAriaLabel(s string) {
	if f == nil {
		return
	}
	f.AriaLabel = s
	f.applyA11y()
}

// Add appends a child.
func (f *Flex) Add(n core.Node) {
	if f == nil {
		return
	}
	if f.Root == nil {
		f.Root = primitive.Row()
		f.Root.Hit = core.HitDefer
		f.Root.ExpandMax = true
	}
	if n != nil {
		f.Root.AddChild(n)
	}
	f.apply()
}

// SetChildren replaces all children.
func (f *Flex) SetChildren(children ...core.Node) {
	if f == nil {
		return
	}
	if f.Root == nil {
		f.Root = primitive.Row()
		f.Root.Hit = core.HitDefer
		f.Root.ExpandMax = true
	}
	f.Root.ClearChildren()
	for _, c := range children {
		if c != nil {
			f.Root.AddChild(c)
		}
	}
	f.apply()
}

// ClearChildren removes all children.
func (f *Flex) ClearChildren() {
	if f == nil || f.Root == nil {
		return
	}
	f.Root.ClearChildren()
	f.apply()
}

// IsVertical reports effective vertical main axis.
func (f *Flex) IsVertical() bool {
	return f != nil && f.EffectiveOrientation() == FlexVertical
}

// EffectiveOrientation returns orientation after sugar resolution.
func (f *Flex) EffectiveOrientation() FlexOrientation {
	if f == nil {
		return FlexHorizontal
	}
	if f.orientationSet {
		return f.Orientation
	}
	if f.Vertical {
		return FlexVertical
	}
	return f.Orientation
}

// ResolvedGap returns the effective gap in px (§6.2: 0 / 8 / 16 / 24 or custom).
func (f *Flex) ResolvedGap() float64 {
	if f == nil {
		return 0
	}
	if f.gapCustom {
		return f.GapPx
	}
	th := f.theme()
	switch f.GapSize {
	case FlexGapSmall:
		// antd paddingXS = sizeXS = 8; kit TokenPaddingXS is 4 — do not use it.
		return DefaultFlexGapSmall
	case FlexGapMedium:
		return th.SizeOr(core.TokenPadding, DefaultFlexGapMedium)
	case FlexGapLarge:
		return th.SizeOr(core.TokenPaddingLG, DefaultFlexGapLarge)
	default:
		return 0
	}
}

// ResolvedJustify maps to core.MainAxisAlignment.
func (f *Flex) ResolvedJustify() core.MainAxisAlignment {
	if f == nil {
		return core.MainStart
	}
	switch f.Justify {
	case FlexJustifyCenter:
		return core.MainCenter
	case FlexJustifyEnd:
		return core.MainEnd
	case FlexJustifySpaceBetween:
		return core.MainSpaceBetween
	case FlexJustifySpaceAround:
		return core.MainSpaceAround
	case FlexJustifySpaceEvenly:
		return core.MainSpaceEvenly
	default:
		return core.MainStart
	}
}

// ResolvedAlign maps to core.CrossAxisAlignment (Auto → antd default).
func (f *Flex) ResolvedAlign() core.CrossAxisAlignment {
	if f == nil {
		return core.CrossStart
	}
	switch f.Align {
	case FlexAlignStart:
		return core.CrossStart
	case FlexAlignCenter:
		return core.CrossCenter
	case FlexAlignEnd:
		return core.CrossEnd
	case FlexAlignStretch:
		return core.CrossStretch
	default: // Auto / normal
		// antd utils: vertical without align → stretch; horizontal → start (doc 向上对齐).
		if f.IsVertical() {
			return core.CrossStretch
		}
		return core.CrossStart
	}
}

func (f *Flex) theme() *core.Theme {
	if f != nil && f.Theme != nil {
		return f.Theme
	}
	return DefaultTheme()
}

func (f *Flex) apply() {
	if f == nil {
		return
	}
	if f.Root == nil {
		f.Root = primitive.Row()
		f.Root.Hit = core.HitDefer
		f.Root.ExpandMax = true
	}
	// Keep block-level fill even if Root was recreated.
	f.Root.ExpandMax = true
	if f.IsVertical() {
		f.Root.Axis = core.AxisVertical
	} else {
		f.Root.Axis = core.AxisHorizontal
	}
	f.Root.Gap = f.ResolvedGap()
	f.Root.Wrap = f.Wrap
	f.Root.MainAlign = f.ResolvedJustify()
	f.Root.CrossAlign = f.ResolvedAlign()
	f.Root.MarkNeedsLayout()
	f.applyA11y()
}

func (f *Flex) applyA11y() {
	if f == nil || f.Root == nil {
		return
	}
	// Layout container: optional name only; no role/keyboard (not interactive).
	f.Root.Base().Label = f.AriaLabel
}
