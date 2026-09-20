// Package prim layout facades: pure layout math plus builders over
// existing ui/rendering nodes (P1).
//
// rendering.RenderObject seals its implementation to ui/rendering
// (unexported setSize/clear methods), so this package does not declare
// new RenderObjects. Instead it provides constraint/size math verified
// by unit tests plus constructors that compose existing nodes
// (RenderBox, RenderAlignBox, AbsoluteBox, Viewport, VirtualList).
// Spacing and sizes arrive via props or scope.Ctx, never hardcoded.
package prim

import (
	"github.com/energye/gpui/ui/kit/internal/scope"
	"github.com/energye/gpui/ui/rendering"
)

// Direction follows scope.Direction so Row order tracks Ctx.Dir.
type Direction = scope.Direction

const (
	// DirLTR lays Row children left to right.
	DirLTR = scope.DirLTR
	// DirRTL lays Row children right to left.
	DirRTL = scope.DirRTL
)

// CrossAlign selects the cross-axis placement inside flex and wrap runs.
type CrossAlign int

const (
	// CrossStart packs children at the cross start.
	CrossStart CrossAlign = iota
	// CrossCenter centers children on the cross axis.
	CrossCenter
	// CrossStretch documents full-cross fill (applied by callers).
	CrossStretch
)

// ---- Pad ----

// PadOuter returns the outer size of a padded child.
func PadOuter(child rendering.Size, l, t, r, b float64) rendering.Size {
	return rendering.Size{Width: child.Width + l + r, Height: child.Height + t + b}
}

// PadChildConstraints shrinks parent constraints by insets for the child.
func PadChildConstraints(parent rendering.Constraints, l, t, r, b float64) rendering.Constraints {
	return rendering.Constraints{
		MinWidth:  0,
		MaxWidth:  subClamp(parent.MaxWidth, l+r),
		MinHeight: 0,
		MaxHeight: subClamp(parent.MaxHeight, t+b),
	}
}

func subClamp(v, d float64) float64 {
	if v >= rendering.Unbounded/2 {
		return v
	}
	if v -= d; v < 0 {
		return 0
	} else {
		return v
	}
}

// NewPadBox wraps child with uniform pad using the existing RenderBox.
// Per-side insets use PadOuter/PadChildConstraints with an AbsoluteBox
// host positioned by the caller.
func NewPadBox(child rendering.RenderObject, pad float64) *rendering.RenderBox {
	if pad < 0 {
		pad = 0
	}
	b := rendering.NewRenderBox()
	b.Pad = pad
	if child != nil {
		b.AddChild(child)
	}
	return b
}

// Center wraps child in a fractional align box at 0.5,0.5.
func Center(child rendering.RenderObject) *rendering.RenderAlignBox {
	return rendering.NewRenderAlignBox(child, 0.5, 0.5)
}

// AlignBox wraps child with fractional alignment ax,ay in 0..1.
func AlignBox(child rendering.RenderObject, ax, ay float64) *rendering.RenderAlignBox {
	return rendering.NewRenderAlignBox(child, ax, ay)
}

// ---- Sized / Constrained / Aspect ----

// NewSizedBox builds a fixed-size host for an optional child.
func NewSizedBox(w, h float64, child rendering.RenderObject) *rendering.RenderBox {
	b := rendering.NewRenderBox()
	b.FixedWidth, b.FixedHeight = w, h
	if child != nil {
		b.AddChild(child)
	}
	return b
}

// Limits carries optional min/max edges. Negative means "no limit".
type Limits struct {
	MinW, MaxW, MinH, MaxH float64
}

// Apply merges parent constraints with the limits (ConstrainedBox rule).
func (l Limits) Apply(parent rendering.Constraints) rendering.Constraints {
	out := parent
	if l.MinW >= 0 && out.MinWidth < l.MinW {
		out.MinWidth = l.MinW
	}
	if l.MaxW >= 0 && out.MaxWidth > l.MaxW {
		out.MaxWidth = l.MaxW
	}
	if l.MinH >= 0 && out.MinHeight < l.MinH {
		out.MinHeight = l.MinH
	}
	if l.MaxH >= 0 && out.MaxHeight > l.MaxH {
		out.MaxHeight = l.MaxH
	}
	if out.MinWidth > out.MaxWidth {
		out.MinWidth = out.MaxWidth
	}
	if out.MinHeight > out.MaxHeight {
		out.MinHeight = out.MaxHeight
	}
	return out
}

// ConstrainSize tightens the child size into the effective constraints.
func ConstrainSize(l Limits, parent rendering.Constraints, child rendering.Size) rendering.Size {
	return l.Apply(parent).Tighten(child)
}

// AspectFit returns the largest ratio-preserving size inside parent.
// Ratio is width/height and falls back to 1 when <= 0.
func AspectFit(parent rendering.Constraints, ratio float64) rendering.Size {
	if ratio <= 0 {
		ratio = 1
	}
	maxW, maxH := parent.MaxWidth, parent.MaxHeight
	if maxW >= rendering.Unbounded/2 && maxH >= rendering.Unbounded/2 {
		w := parent.MinWidth
		if w <= 0 {
			w = 100
		}
		return parent.Tighten(rendering.Size{Width: w, Height: w / ratio})
	}
	if maxW >= rendering.Unbounded/2 {
		return parent.Tighten(rendering.Size{Width: maxH * ratio, Height: maxH})
	}
	if maxH >= rendering.Unbounded/2 {
		return parent.Tighten(rendering.Size{Width: maxW, Height: maxW / ratio})
	}
	w, h := maxW, maxW/ratio
	if h > maxH {
		h = maxH
		w = h * ratio
	}
	if w < parent.MinWidth {
		w = parent.MinWidth
		h = w / ratio
	}
	if h < parent.MinHeight {
		h = parent.MinHeight
		w = h * ratio
	}
	return parent.Tighten(rendering.Size{Width: w, Height: h})
}

// ---- Offstage ----

// Offstage keeps the child measured size while hidden: layout占位但不画.
// Visible=false still reports ChildSize; paint and hit are skipped.
type Offstage struct {
	ChildSize rendering.Size
	Hidden    bool
}

// Size reports the layout footprint (always the child size).
func (o Offstage) Size() rendering.Size { return o.ChildSize }

// PaintEnabled reports whether the child may paint.
func (o Offstage) PaintEnabled() bool { return !o.Hidden }

// HitEnabled reports whether the child may receive hits.
func (o Offstage) HitEnabled() bool { return !o.Hidden }

// ---- Flex: Row / Column / Expanded / Flexible / Spacer ----

// FlexSpec is one flex child. FixedW/FixedH carry the measured natural
// size of a non-flex child; Flex > 0 shares leftover main space; Tight
// mirrors Expanded (must fill share) versus Flexible (may be smaller);
// IsSpacer marks empty space with no content.
type FlexSpec struct {
	FixedW, FixedH float64
	Flex           int
	Tight          bool
	IsSpacer       bool
}

// FixedSpec wraps a measured child with no flex factor.
func FixedSpec(w, h float64) FlexSpec { return FlexSpec{FixedW: w, FixedH: h} }

// ExpandedSpec must fill its share exactly.
func ExpandedSpec(flex int) FlexSpec {
	if flex <= 0 {
		flex = 1
	}
	return FlexSpec{Flex: flex, Tight: true}
}

// FlexibleSpec may be smaller than its share.
func FlexibleSpec(flex int) FlexSpec {
	if flex <= 0 {
		flex = 1
	}
	return FlexSpec{Flex: flex}
}

// SpacerSpec is empty flex space.
func SpacerSpec(flex int) FlexSpec {
	if flex <= 0 {
		flex = 1
	}
	return FlexSpec{Flex: flex, Tight: true, IsSpacer: true}
}

// FlexPlacement is the computed geometry of one flex child.
type FlexPlacement struct {
	W, H float64
	X, Y float64
}

// FlexResult is the full flex geometry: outer size plus per-child boxes.
type FlexResult struct {
	Outer rendering.Size
	Boxes []FlexPlacement
}

// FlexLayout computes Row (horizontal=true) or Column geometry.
// mainMax/crossMax come from the parent constraints; spacing arrives
// from theme; dir mirrors Ctx.Dir for Row RTL.
func FlexLayout(horizontal bool, mainMax, crossMax, spacing float64, dir Direction, cross CrossAlign, items []FlexSpec) FlexResult {
	if spacing < 0 {
		spacing = 0
	}
	if mainMax >= rendering.Unbounded/2 {
		mainMax = 1e9
	}
	bounded := mainMax < 1e9/2
	n := len(items)
	sizes := make([]FlexPlacement, n)
	// Fixed pass.
	var usedMain float64
	var totalFlex float64
	isFlex := make([]bool, n)
	for i, it := range items {
		if it.Flex > 0 {
			isFlex[i] = true
			totalFlex += float64(it.Flex)
			continue
		}
		if horizontal {
			sizes[i] = FlexPlacement{W: it.FixedW, H: it.FixedH}
			usedMain += it.FixedW
		} else {
			sizes[i] = FlexPlacement{W: it.FixedW, H: it.FixedH}
			usedMain += it.FixedH
		}
	}
	gaps := 0.0
	if n > 1 {
		gaps = spacing * float64(n-1)
	}
	leftover := 0.0
	if bounded {
		leftover = mainMax - usedMain - gaps
		if leftover < 0 {
			leftover = 0
		}
	}
	for i, it := range items {
		if !isFlex[i] {
			continue
		}
		share := 0.0
		if totalFlex > 0 {
			share = leftover * float64(it.Flex) / totalFlex
		}
		if horizontal {
			sizes[i].W = share
			if !it.IsSpacer {
				sizes[i].H = 0
			}
		} else {
			sizes[i].H = share
			if !it.IsSpacer {
				sizes[i].W = 0
			}
		}
	}
	// Cross extent is the max child cross (callers stretch when asked).
	crossUsed := 0.0
	for i, it := range items {
		if isFlex[i] && !it.IsSpacer {
			continue
		}
		c := it.FixedH
		if horizontal {
			c = it.FixedH
		} else {
			c = it.FixedW
		}
		_ = i
		if c > crossUsed {
			crossUsed = c
		}
	}
	if cross == CrossStretch && crossMax < rendering.Unbounded/2 {
		crossUsed = crossMax
	}
	// Outer: bounded main fills max, else wrap content.
	mainUsed := gaps
	for i, it := range items {
		if isFlex[i] {
			if horizontal {
				mainUsed += sizes[i].W
			} else {
				mainUsed += sizes[i].H
			}
		} else if horizontal {
			mainUsed += it.FixedW
		} else {
			mainUsed += it.FixedH
		}
	}
	var outer rendering.Size
	if horizontal {
		w := mainUsed
		if bounded {
			w = mainMax
		}
		outer = rendering.Size{Width: w, Height: crossUsed}
	} else {
		h := mainUsed
		if bounded {
			h = mainMax
		}
		outer = rendering.Size{Width: crossUsed, Height: h}
	}
	// Positions; Row mirrors under RTL.
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	if horizontal && dir == DirRTL {
		for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
			order[i], order[j] = order[j], order[i]
		}
	}
	cursor := 0.0
	crossExtent := outer.Height
	if !horizontal {
		crossExtent = outer.Width
	}
	for _, oi := range order {
		if horizontal {
			sizes[oi].X = cursor
			sizes[oi].Y = crossOffset(cross, crossExtent, sizes[oi].H)
			cursor += sizes[oi].W + spacing
		} else {
			sizes[oi].Y = cursor
			sizes[oi].X = crossOffset(cross, crossExtent, sizes[oi].W)
			cursor += sizes[oi].H + spacing
		}
	}
	return FlexResult{Outer: outer, Boxes: sizes}
}

func crossOffset(cross CrossAlign, extent, child float64) float64 {
	if cross == CrossCenter && extent > child {
		return (extent - child) / 2
	}
	return 0
}

// ---- Stack / Positioned / IndexedStack ----

// StackSpec is one stack layer. Positioned layers carry explicit offsets
// and optional sizes (<= 0 wraps content); others stack at origin.
type StackSpec struct {
	W, H       float64 // measured content size
	Positioned bool
	X, Y       float64
	Width      float64
	Height     float64
}

// StackedSpec wraps a normal layer at origin.
func StackedSpec(w, h float64) StackSpec { return StackSpec{W: w, H: h} }

// PositionedSpec wraps a layer with explicit offsets and optional size.
func PositionedSpec(w, h, x, y, pw, ph float64) StackSpec {
	return StackSpec{W: w, H: h, Positioned: true, X: x, Y: y, Width: pw, Height: ph}
}

// StackResult is the stack outer size plus per-layer offsets.
type StackResult struct {
	Outer rendering.Size
	Boxes []FlexPlacement
}

// StackLayout sizes to the largest non-positioned layer; positioned
// layers keep their offsets.
func StackLayout(parent rendering.Constraints, layers []StackSpec) StackResult {
	var maxW, maxH float64
	for _, l := range layers {
		if l.Positioned {
			continue
		}
		if l.W > maxW {
			maxW = l.W
		}
		if l.H > maxH {
			maxH = l.H
		}
	}
	outer := parent.Tighten(rendering.Size{Width: maxW, Height: maxH})
	boxes := make([]FlexPlacement, len(layers))
	for i, l := range layers {
		if !l.Positioned {
			boxes[i] = FlexPlacement{W: l.W, H: l.H}
			continue
		}
		w, h := l.Width, l.Height
		if w <= 0 {
			w = l.W
		}
		if h <= 0 {
			h = l.H
		}
		boxes[i] = FlexPlacement{W: w, H: h, X: l.X, Y: l.Y}
	}
	return StackResult{Outer: outer, Boxes: boxes}
}

// IndexedOuter sizes an IndexedStack to the largest page.
func IndexedOuter(parent rendering.Constraints, pages []rendering.Size) rendering.Size {
	var maxW, maxH float64
	for _, s := range pages {
		if s.Width > maxW {
			maxW = s.Width
		}
		if s.Height > maxH {
			maxH = s.Height
		}
	}
	return parent.Tighten(rendering.Size{Width: maxW, Height: maxH})
}

// ---- Wrap ----

// WrapResult is the flow geometry: outer size plus per-child boxes.
type WrapResult struct {
	Outer rendering.Size
	Boxes []FlexPlacement
}

// WrapLayout flows child sizes into runs inside parentMaxW.
func WrapLayout(parent rendering.Constraints, spacing, runSpacing float64, cross CrossAlign, children []rendering.Size) WrapResult {
	if spacing < 0 {
		spacing = 0
	}
	if runSpacing < 0 {
		runSpacing = 0
	}
	maxW := parent.MaxWidth
	bounded := maxW < rendering.Unbounded/2
	if !bounded {
		maxW = 1e9
	}
	boxes := make([]FlexPlacement, len(children))
	type run struct{ start, end int }
	var runs []run
	curW := 0.0
	start := 0
	for i, s := range children {
		need := s.Width
		if i > start {
			need += spacing
		}
		if bounded && i > start && curW+need > maxW {
			runs = append(runs, run{start, i})
			start = i
			curW = 0
		}
		curW += s.Width
		if i > start {
			curW += spacing
		}
		boxes[i] = FlexPlacement{W: s.Width, H: s.Height}
		_ = curW
	}
	runs = append(runs, run{start, len(children)})
	y := 0.0
	maxRowW := 0.0
	for ri, r := range runs {
		runH := 0.0
		rowW := 0.0
		for i := r.start; i < r.end; i++ {
			if children[i].Height > runH {
				runH = children[i].Height
			}
			rowW += children[i].Width
			if i > r.start {
				rowW += spacing
			}
		}
		if rowW > maxRowW {
			maxRowW = rowW
		}
		x := 0.0
		for i := r.start; i < r.end; i++ {
			oy := y
			if cross == CrossCenter && runH > children[i].Height {
				oy += (runH - children[i].Height) / 2
			}
			boxes[i].X, boxes[i].Y = x, oy
			x += children[i].Width + spacing
		}
		y += runH
		if ri < len(runs)-1 {
			y += runSpacing
		}
	}
	width := maxRowW
	if bounded && parent.MinWidth == parent.MaxWidth {
		width = parent.MaxWidth
	}
	return WrapResult{Outer: parent.Tighten(rendering.Size{Width: width, Height: y}), Boxes: boxes}
}

// ---- CustomLayout / LayoutBuilder ----

// CustomMeasure delegates measurement to fn (CustomLayout protocol).
func CustomMeasure(parent rendering.Constraints, children []rendering.RenderObject, fn func(rendering.Constraints, []rendering.RenderObject) rendering.Size) rendering.Size {
	if fn != nil {
		return parent.Tighten(fn(parent, children))
	}
	var maxW, maxH float64
	for _, ch := range children {
		if ch == nil {
			continue
		}
		sz := ch.Layout(rendering.Constraints{MinWidth: 0, MaxWidth: parent.MaxWidth, MinHeight: 0, MaxHeight: parent.MaxHeight})
		if sz.Width > maxW {
			maxW = sz.Width
		}
		if sz.Height > maxH {
			maxH = sz.Height
		}
	}
	return parent.Tighten(rendering.Size{Width: maxW, Height: maxH})
}

// BuilderFunc rebuilds content from constraints (LayoutBuilder protocol).
type BuilderFunc func(c rendering.Constraints) rendering.RenderObject

// ---- ViewportBox ----

// ViewportBox wires a virtual list under a viewport with scroll handling.
// It holds existing rendering nodes; List/Scroll stay reachable for the
// virtualization gate (bind_count << item_count).
type ViewportBox struct {
	Viewport *rendering.RenderViewport
	List     *rendering.VirtualList
	Scroll   *rendering.Scrollable
}

// NewViewportBox builds a scrollable fixed-extent row host.
func NewViewportBox(count int, extent float64, builder rendering.ItemBuilder) *ViewportBox {
	list := rendering.NewVirtualList(count, extent, builder)
	vp := rendering.NewRenderViewport(list)
	return &ViewportBox{Viewport: vp, List: list, Scroll: rendering.NewScrollable(vp)}
}

// NewVariableViewportBox builds a variable-height variant.
func NewVariableViewportBox(count int, fallback float64, extentAt rendering.ItemExtentFunc, builder rendering.ItemBuilder) *ViewportBox {
	list := rendering.NewVariableVirtualList(count, fallback, extentAt, builder)
	vp := rendering.NewRenderViewport(list)
	return &ViewportBox{Viewport: vp, List: list, Scroll: rendering.NewScrollable(vp)}
}

// BindCount reports mounted rows for the virtualization gate.
func (v *ViewportBox) BindCount() int {
	if v == nil || v.List == nil {
		return 0
	}
	return v.List.BindCount
}
