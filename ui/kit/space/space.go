// Space widget (docs/antd/space.md §6.10).
//
// Composition over new frameworks: Space owns a spaceNode built on
// ui/rendering RenderBox for layout/paint, ui/theme for gap tokens.
// No new event or frame system; pure layout container, no chrome.
package space

import (
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// Gap fallbacks when theme tokens are zero (antd §6.2.1).
const (
	DefaultGapSmall  = 8.0
	DefaultGapMiddle = 16.0
	DefaultGapLarge  = 24.0
)

// SpaceOrientation selects the main axis (orientation wins over Vertical).
type SpaceOrientation string

const (
	SpaceHorizontal SpaceOrientation = "horizontal"
	SpaceVertical   SpaceOrientation = "vertical"
)

// SpaceAlign selects cross-axis alignment (auto follows the antd rule:
// horizontal centers, vertical starts).
type SpaceAlign string

const (
	SpaceAlignAuto     SpaceAlign = "auto"
	SpaceAlignStart    SpaceAlign = "start"
	SpaceAlignEnd      SpaceAlign = "end"
	SpaceAlignCenter   SpaceAlign = "center"
	SpaceAlignBaseline SpaceAlign = "baseline"
)

// SpaceSize selects the preset gap (also the Compact/Addon control档).
type SpaceSize int

const (
	SpaceSizeSmall SpaceSize = iota
	SpaceSizeMiddle
	SpaceSizeLarge
)

// SpaceSizeMedium is the antd middle alias of medium.
const SpaceSizeMedium = SpaceSizeMiddle

// sizeMode tracks which setter won last (last-set wins).
type sizeMode int

const (
	sizePreset sizeMode = iota
	sizePx
	sizeXY
)

// Space is the Space widget (docs/antd/space.md §6.10).
//
// Owns a spaceNode: put Node() in the tree, drive Layout through it,
// read EffectiveOrientation/ResolvedGap for assertions.
type Space struct {
	orient    SpaceOrientation
	orientSet bool
	vertical  bool
	align     SpaceAlign
	size      SpaceSize
	mode      sizeMode
	px        float64
	col       float64
	row       float64
	wrap      bool
	separator func() rendering.RenderObject
	expandMax bool
	rtl       bool
	provider  *theme.Provider
	override  *theme.Tokens
	ariaLabel string
	logical   []rendering.RenderObject
	seps      []rendering.RenderObject
	node      *spaceNode
}

// NewSpace creates a horizontal small-gap container (antd §6.10 defaults).
func NewSpace(children ...rendering.RenderObject) *Space {
	s := &Space{align: SpaceAlignAuto}
	s.node = newSpaceNode(s)
	s.SetChildren(children...)
	return s
}

// SetOrientation sets horizontal/vertical (wins over Vertical).
func (s *Space) SetOrientation(o SpaceOrientation) {
	if s == nil {
		return
	}
	if o != SpaceVertical {
		o = SpaceHorizontal
	}
	if s.orientSet && s.orient == o {
		return
	}
	s.orient = o
	s.orientSet = true
	s.node.MarkNeedsLayout()
}

// SetVertical sets the antd vertical sugar (ignored after SetOrientation).
func (s *Space) SetVertical(b bool) {
	if s == nil || s.vertical == b {
		return
	}
	s.vertical = b
	s.node.MarkNeedsLayout()
}

// EffectiveOrientation resolves orientation > vertical > horizontal.
func (s *Space) EffectiveOrientation() SpaceOrientation {
	if s != nil && s.orientSet {
		return s.orient
	}
	if s != nil && s.vertical {
		return SpaceVertical
	}
	return SpaceHorizontal
}

// IsVertical reports whether the main axis is vertical.
func (s *Space) IsVertical() bool { return s.EffectiveOrientation() == SpaceVertical }

// SetAlign sets cross-axis alignment (auto follows the antd rule).
func (s *Space) SetAlign(a SpaceAlign) {
	if s == nil {
		return
	}
	switch a {
	case SpaceAlignStart, SpaceAlignEnd, SpaceAlignCenter, SpaceAlignBaseline:
		s.align = a
	default:
		s.align = SpaceAlignAuto
	}
	s.node.MarkNeedsLayout()
}

// Align returns the stored alignment (auto when unset).
func (s *Space) Align() SpaceAlign {
	if s == nil {
		return SpaceAlignAuto
	}
	return s.align
}

// EffectiveAlign resolves auto to center (horizontal) or start (vertical).
func (s *Space) EffectiveAlign() SpaceAlign {
	if s != nil && s.align != SpaceAlignAuto {
		return s.align
	}
	if s.IsVertical() {
		return SpaceAlignStart
	}
	return SpaceAlignCenter
}

// SetSize sets the preset gap (small/middle/large).
func (s *Space) SetSize(v SpaceSize) {
	if s == nil {
		return
	}
	if v != SpaceSizeMiddle && v != SpaceSizeLarge {
		v = SpaceSizeSmall
	}
	s.size = v
	s.mode = sizePreset
	s.node.MarkNeedsLayout()
}

// Size returns the preset gap selector.
func (s *Space) Size() SpaceSize {
	if s == nil {
		return SpaceSizeSmall
	}
	return s.size
}

// SetSizePx sets a numeric gap for both axes (explicit 0 allowed).
func (s *Space) SetSizePx(v float64) {
	if s == nil {
		return
	}
	if v < 0 {
		v = 0
	}
	s.px = v
	s.mode = sizePx
	s.node.MarkNeedsLayout()
}

// SetSizeXY sets antd size={[col,row]} numeric gaps.
func (s *Space) SetSizeXY(col, row float64) {
	if s == nil {
		return
	}
	if col < 0 {
		col = 0
	}
	if row < 0 {
		row = 0
	}
	s.col, s.row = col, row
	s.mode = sizeXY
	s.node.MarkNeedsLayout()
}

// presetGap maps a preset through theme tokens with numeric fallback.
func presetGap(tok theme.Tokens, v SpaceSize) float64 {
	switch v {
	case SpaceSizeLarge:
		if tok.PaddingLG > 0 {
			return tok.PaddingLG
		}
		return DefaultGapLarge
	case SpaceSizeMiddle:
		if tok.Padding > 0 {
			return tok.Padding
		}
		return DefaultGapMiddle
	default:
		if tok.PaddingXS > 0 {
			return tok.PaddingXS
		}
		return DefaultGapSmall
	}
}

// colRowGap resolves the (col, row) pair for the current size mode.
func (s *Space) colRowGap() (col, row float64) {
	tok := s.themeTokens()
	switch s.mode {
	case sizePx:
		return s.px, s.px
	case sizeXY:
		return s.col, s.row
	default:
		g := presetGap(tok, s.size)
		return g, g
	}
}

// ResolvedColGap returns the column (x-axis) gap.
func (s *Space) ResolvedColGap() float64 {
	if s == nil {
		return DefaultGapSmall
	}
	col, _ := s.colRowGap()
	return col
}

// ResolvedRowGap returns the row (y-axis) gap.
func (s *Space) ResolvedRowGap() float64 {
	if s == nil {
		return DefaultGapSmall
	}
	_, row := s.colRowGap()
	return row
}

// ResolvedGap returns the main-axis gap (col when horizontal, row vertical).
func (s *Space) ResolvedGap() float64 {
	if s.IsVertical() {
		return s.ResolvedRowGap()
	}
	return s.ResolvedColGap()
}

// SetWrap enables wrapping (horizontal only, §6.3).
func (s *Space) SetWrap(b bool) {
	if s == nil || s.wrap == b {
		return
	}
	s.wrap = b
	s.node.MarkNeedsLayout()
}

// Wrap reports the wrap flag.
func (s *Space) Wrap() bool { return s != nil && s.wrap }

// SetSeparator sets the per-gap factory (nil clears). The factory runs
// once per gap on structural change so one node never mounts twice.
func (s *Space) SetSeparator(fn func() rendering.RenderObject) {
	if s == nil {
		return
	}
	s.separator = fn
	s.rebuild()
}

// SetChildren replaces the logical children.
func (s *Space) SetChildren(children ...rendering.RenderObject) {
	if s == nil {
		return
	}
	s.logical = s.logical[:0]
	for _, c := range children {
		if c != nil {
			s.logical = append(s.logical, c)
		}
	}
	s.rebuild()
}

// Add appends logical children.
func (s *Space) Add(children ...rendering.RenderObject) {
	if s == nil {
		return
	}
	for _, c := range children {
		if c != nil {
			s.logical = append(s.logical, c)
		}
	}
	s.rebuild()
}

// ClearChildren removes all children and separators.
func (s *Space) ClearChildren() {
	if s == nil {
		return
	}
	s.logical = nil
	s.rebuild()
}

// Children returns the logical children (separators excluded).
func (s *Space) Children() []rendering.RenderObject {
	if s == nil {
		return nil
	}
	return append([]rendering.RenderObject(nil), s.logical...)
}

// ChildCount returns the logical child count.
func (s *Space) ChildCount() int {
	if s == nil {
		return 0
	}
	return len(s.logical)
}

// SeparatorCount returns the live separator node count.
func (s *Space) SeparatorCount() int {
	if s == nil {
		return 0
	}
	return len(s.seps)
}

// SeparatorAt returns separator i (nil out of range).
func (s *Space) SeparatorAt(i int) rendering.RenderObject {
	if s == nil || i < 0 || i >= len(s.seps) {
		return nil
	}
	return s.seps[i]
}

// SeparatorsAriaHidden is always true: separators are decorative (§6.6).
func (s *Space) SeparatorsAriaHidden() bool { return true }

// SeparatorFocusable is always false: separators never take Tab (§6.6).
func (s *Space) SeparatorFocusable() bool { return false }

// SeparatorColor follows the split track token (no brand hardcode, §6.2.2).
func (s *Space) SeparatorColor() theme.Color {
	tok := s.themeTokens()
	if tok.ColorBorderSecondary != (theme.Color{}) {
		return tok.ColorBorderSecondary
	}
	return theme.Hex("#f0f0f0")
}

// SetExpandMax fills the parent width (block-level flex, §6.10).
func (s *Space) SetExpandMax(b bool) {
	if s == nil || s.expandMax == b {
		return
	}
	s.expandMax = b
	s.node.MarkNeedsLayout()
}

// ExpandMax reports the block fill flag.
func (s *Space) ExpandMax() bool { return s != nil && s.expandMax }

// SetRTL mirrors row direction / cross start-end (antd rtl, §4.5).
func (s *Space) SetRTL(b bool) {
	if s == nil || s.rtl == b {
		return
	}
	s.rtl = b
	s.node.MarkNeedsLayout()
}

// RTL reports the mirror flag.
func (s *Space) RTL() bool { return s != nil && s.rtl }

// SetTheme pins exact tokens (nil clears to provider).
func (s *Space) SetTheme(tok *theme.Tokens) {
	if s == nil {
		return
	}
	s.override = tok
	s.node.MarkNeedsLayout()
}

// SetProvider selects the theme source (nil selects process default).
func (s *Space) SetProvider(p *theme.Provider) {
	if s == nil {
		return
	}
	s.provider = p
	s.node.MarkNeedsLayout()
}

func (s *Space) themeTokens() theme.Tokens {
	if s != nil && s.override != nil {
		return *s.override
	}
	if s != nil && s.provider != nil {
		return s.provider.Current()
	}
	return theme.Default.Current()
}

// SetAriaLabel names the group (paint-only; never takes focus).
func (s *Space) SetAriaLabel(v string) {
	if s == nil || s.ariaLabel == v {
		return
	}
	s.ariaLabel = v
	s.node.MarkNeedsPaint()
}

// AriaLabel returns the accessible name ("" means unnamed).
func (s *Space) AriaLabel() string {
	if s == nil {
		return ""
	}
	return s.ariaLabel
}

// Role is "group" when named, "" otherwise (container never forces role).
func (s *Space) Role() string {
	if s != nil && s.ariaLabel != "" {
		return "group"
	}
	return ""
}

// Focusable is always false: layout containers never take Tab (§6.6).
func (s *Space) Focusable() bool { return false }

// Node returns the tree node (layout/paint/hit through it).
func (s *Space) Node() rendering.RenderObject {
	if s == nil {
		return nil
	}
	return s.node
}

// ChromeNode mirrors Node (Space has no separate chrome).
func (s *Space) ChromeNode() rendering.RenderObject { return s.Node() }

// Layout sizes the node under constraints.
func (s *Space) Layout(c rendering.Constraints) rendering.Size {
	if s == nil || s.node == nil {
		return rendering.Size{}
	}
	return s.node.Layout(c)
}

// rebuild re-syncs node children: interleaved logical + fresh separators.
func (s *Space) rebuild() {
	if s == nil || s.node == nil {
		return
	}
	for _, c := range append([]rendering.RenderObject(nil), s.node.Children()...) {
		s.node.RemoveChild(c)
	}
	s.seps = nil
	seen := map[rendering.RenderObject]bool{}
	for _, c := range s.logical {
		seen[c] = true
	}
	if s.separator != nil {
		for i := 0; i+1 < len(s.logical); i++ {
			sep := s.separator()
			if sep == nil || seen[sep] {
				continue
			}
			seen[sep] = true
			s.seps = append(s.seps, sep)
		}
	}
	// Strict interleave: child, sep, child, sep, ...
	ordered := make([]rendering.RenderObject, 0, len(s.logical)+len(s.seps))
	for i, c := range s.logical {
		ordered = append(ordered, c)
		if i < len(s.seps) {
			ordered = append(ordered, s.seps[i])
		}
	}
	for _, c := range append([]rendering.RenderObject(nil), s.node.Children()...) {
		s.node.RemoveChild(c)
	}
	for _, c := range ordered {
		s.node.AddChild(c)
	}
	s.node.MarkNeedsLayout()
}

// spaceNode is the layout node. It embeds *RenderBox so all RenderObject
// storage (size/dirty/parent, including unexported Base bits) lives in the
// rendering package; this file only overrides Layout for gap rules.
type spaceNode struct {
	*rendering.RenderBox
	space *Space
}

func newSpaceNode(s *Space) *spaceNode {
	n := &spaceNode{
		RenderBox: rendering.NewRenderBox(),
		space:     s,
	}
	n.SetRepaintBoundary(true)
	n.SetRelayoutBoundary(true)
	return n
}

// slots interleaves logical children with separators in node order.
func (s *Space) slots() []rendering.RenderObject {
	if s == nil {
		return nil
	}
	if len(s.seps) == 0 {
		return append([]rendering.RenderObject(nil), s.logical...)
	}
	out := make([]rendering.RenderObject, 0, len(s.logical)+len(s.seps))
	for i, c := range s.logical {
		out = append(out, c)
		if i < len(s.seps) {
			out = append(out, s.seps[i])
		}
	}
	return out
}

// Layout arranges slots with gap rules, then delegates sizing to RenderBox
// (offsets are assigned after the inner layout, so the box Pad reset
// never survives).
func (n *spaceNode) Layout(c rendering.Constraints) rendering.Size {
	if n == nil || n.space == nil || n.RenderBox == nil {
		return rendering.Size{}
	}
	s := n.space
	// Fast path: clean subtree keeps prior size/offsets.
	if !n.ShouldRelayout(c) {
		return n.Size()
	}
	vertical := s.IsVertical()
	colGap, rowGap := s.colRowGap()
	gap := colGap
	if vertical {
		gap = rowGap
	}
	kids := s.slots()
	count := len(kids)
	if count == 0 {
		n.FixedWidth, n.FixedHeight = 0, 0
		return n.RenderBox.Layout(c)
	}
	inner := rendering.Constraints{MaxWidth: c.MaxWidth, MaxHeight: c.MaxHeight}
	sizes := make([]rendering.Size, count)
	for i, ch := range kids {
		if rendering.ManualLayoutOf(ch) {
			sizes[i] = ch.Size()
			continue
		}
		sizes[i] = ch.Layout(inner)
	}
	mainOf := func(sz rendering.Size) float64 {
		if vertical {
			return sz.Height
		}
		return sz.Width
	}
	crossOf := func(sz rendering.Size) float64 {
		if vertical {
			return sz.Width
		}
		return sz.Height
	}
	// Lines: wrap breaks horizontal rows only (§6.3).
	limit := c.MaxWidth
	lines := [][]int{}
	if vertical || !s.wrap || limit >= rendering.Unbounded/2 {
		line := make([]int, count)
		for i := range line {
			line[i] = i
		}
		lines = append(lines, line)
	} else {
		cur := []int{}
		var curMain float64
		for i := 0; i < count; i++ {
			need := mainOf(sizes[i])
			if len(cur) > 0 {
				need += gap
			}
			if len(cur) > 0 && curMain+need > limit+1e-9 {
				lines = append(lines, cur)
				cur = []int{i}
				curMain = mainOf(sizes[i])
				continue
			}
			cur = append(cur, i)
			curMain += need
		}
		if len(cur) > 0 {
			lines = append(lines, cur)
		}
	}
	lineMain := make([]float64, len(lines))
	lineCross := make([]float64, len(lines))
	for li, line := range lines {
		var m, x float64
		for j, idx := range line {
			m += mainOf(sizes[idx])
			if j > 0 {
				m += gap
			}
			if v := crossOf(sizes[idx]); v > x {
				x = v
			}
		}
		lineMain[li] = m
		lineCross[li] = x
	}
	var contentW, contentH float64
	if !vertical {
		for _, m := range lineMain {
			if m > contentW {
				contentW = m
			}
		}
		for j, x := range lineCross {
			contentH += x
			if j > 0 {
				contentH += rowGap
			}
		}
		if s.expandMax && c.MaxWidth < rendering.Unbounded/2 {
			contentW = c.MaxWidth
		}
	} else {
		for j, m := range lineMain {
			contentH += m
			if j > 0 {
				contentH += gap
			}
		}
		for _, x := range lineCross {
			if x > contentW {
				contentW = x
			}
		}
		if s.expandMax && c.MaxWidth < rendering.Unbounded/2 {
			contentW = c.MaxWidth
		}
	}
	n.FixedWidth, n.FixedHeight = contentW, contentH
	out := n.RenderBox.Layout(c)
	align := s.EffectiveAlign()
	// Place each line; offsets assigned after the inner layout.
	var crossCursor float64
	for li, line := range lines {
		lx := lineCross[li]
		base := lx
		if len(lines) == 1 {
			var containerCross float64
			if !vertical {
				containerCross = out.Height
			} else {
				containerCross = out.Width
			}
			if containerCross > base {
				base = containerCross
			}
		}
		cursor := 0.0
		for _, idx := range line {
			ch := kids[idx]
			sz := sizes[idx]
			cross := crossOf(sz)
			var coff float64
			switch align {
			case SpaceAlignCenter:
				coff = (base - cross) / 2
			case SpaceAlignEnd:
				coff = base - cross
			default:
				coff = 0
			}
			if !vertical {
				ch.SetOffset(rendering.Point{X: cursor, Y: crossCursor + coff})
				cursor += mainOf(sz) + gap
			} else {
				ch.SetOffset(rendering.Point{X: crossCursor + coff, Y: cursor})
				cursor += mainOf(sz) + gap
			}
		}
		if !vertical {
			crossCursor += lx + rowGap
		} else {
			crossCursor += lx + gap
		}
	}
	// RTL mirror: horizontal mirrors row direction, vertical the cross.
	if s.rtl {
		if !vertical {
			for _, idx := range allLineIdx(lines) {
				ch := kids[idx]
				off := ch.Offset()
				sz := sizes[idx]
				off.X = contentW - off.X - sz.Width
				ch.SetOffset(off)
			}
		} else {
			for _, ch := range kids {
				off := ch.Offset()
				sz := ch.Size()
				off.X = contentW - off.X - sz.Width
				ch.SetOffset(off)
			}
		}
	}
	return out
}

func allLineIdx(lines [][]int) []int {
	var out []int
	for _, l := range lines {
		out = append(out, l...)
	}
	return out
}
