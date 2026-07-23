package core

// Axis is the main axis for Flex layout.
type Axis int

const (
	AxisHorizontal Axis = iota // Row
	AxisVertical               // Column
)

// MainAxisAlignment distributes free space on the main axis.
type MainAxisAlignment int

const (
	MainStart MainAxisAlignment = iota
	MainCenter
	MainEnd
	MainSpaceBetween
	MainSpaceAround
	MainSpaceEvenly
)

// CrossAxisAlignment aligns children on the cross axis.
type CrossAxisAlignment int

const (
	CrossStart CrossAxisAlignment = iota
	CrossCenter
	CrossEnd
	CrossStretch
)

// FlexLayoutParams configures a single Flex layout pass.
type FlexLayoutParams struct {
	Axis       Axis
	MainAlign  MainAxisAlignment
	CrossAlign CrossAxisAlignment
	Gap        float64
	// Wrap packs children onto multiple lines when the main axis is bounded
	// and the next child would exceed maxMain. Gap is used both within a line
	// and between lines. Flex-grow is ignored while Wrap is true (Ant Space
	// wrap / multi-line row of fixed controls). Unbounded main never wraps.
	Wrap bool
}

// LayoutFlex performs flex layout for children of parent.
// parent must already have NodeBase children attached; sizes/offsets are written
// onto each child. Returns the parent's resulting size (constrained).
//
// When Wrap is true and the main axis max is finite, children are packed into
// multiple lines (row→next row below, column→next column to the right).
// Otherwise a single-line pass runs (with optional flex-grow).
func LayoutFlex(parent *NodeBase, c Constraints, p FlexLayoutParams) Size {
	kids := parent.children
	n := len(kids)
	if n == 0 {
		s := c.Tighten(Size{})
		parent.SetSize(s)
		return s
	}

	horizontal := p.Axis == AxisHorizontal
	maxMain := c.MaxWidth
	maxCross := c.MaxHeight
	minMain := c.MinWidth
	minCross := c.MinHeight
	if !horizontal {
		maxMain, maxCross = c.MaxHeight, c.MaxWidth
		minMain, minCross = c.MinHeight, c.MinWidth
	}

	if p.Wrap && isFinite(maxMain) {
		return layoutFlexWrap(parent, c, p, horizontal, maxMain, maxCross, minMain, minCross)
	}

	// Gap total between children.
	gapTotal := 0.0
	if n > 1 && p.Gap > 0 {
		gapTotal = p.Gap * float64(n-1)
	}

	// First pass: layout non-flex children with loose constraints; measure flex intrinsics at 0 grow.
	type item struct {
		node     Node
		base     *NodeBase
		flex     float64
		mainSize float64
		cross    float64
	}
	items := make([]item, n)
	var totalFlex float64
	var fixedMain float64

	for i, child := range kids {
		it := item{node: child, base: child.Base()}
		if fn, ok := child.(FlexFactorNode); ok {
			it.flex = fn.FlexGrow()
		}
		if it.flex > 0 {
			totalFlex += it.flex
			// Intrinsic: layout with zero main max to get minimum content size.
			childC := flexChildConstraints(horizontal, 0, maxCross, p.CrossAlign)
			sz := child.Layout(childC)
			if horizontal {
				it.mainSize = sz.Width
				it.cross = sz.Height
			} else {
				it.mainSize = sz.Height
				it.cross = sz.Width
			}
		} else {
			childC := flexChildConstraints(horizontal, maxMain, maxCross, p.CrossAlign)
			// Non-flex: unbounded main for intrinsic, capped by remaining later — use maxMain for simplicity.
			if horizontal {
				childC = Constraints{MaxWidth: maxMain, MaxHeight: maxCross}
				if p.CrossAlign == CrossStretch && c.HasBoundedHeight() {
					childC.MinHeight = maxCross
					childC.MaxHeight = maxCross
				}
			} else {
				childC = Constraints{MaxWidth: maxCross, MaxHeight: maxMain}
				if p.CrossAlign == CrossStretch && c.HasBoundedWidth() {
					childC.MinWidth = maxCross
					childC.MaxWidth = maxCross
				}
			}
			sz := child.Layout(childC)
			if horizontal {
				it.mainSize = sz.Width
				it.cross = sz.Height
			} else {
				it.mainSize = sz.Height
				it.cross = sz.Width
			}
			fixedMain += it.mainSize
		}
		items[i] = it
	}

	// Free main space for flex children.
	freeMain := maxMain - fixedMain - gapTotal
	if freeMain < 0 || !isFinite(maxMain) {
		// Unbounded main: flex children keep intrinsic; grow does not expand infinitely.
		freeMain = 0
	}

	// Intrinsic main of flex kids counted separately for parent size when unbounded.
	var flexIntrinsicMain float64
	for i := range items {
		it := &items[i]
		if it.flex <= 0 {
			continue
		}
		flexIntrinsicMain += it.mainSize
		share := 0.0
		if totalFlex > 0 && freeMain > 0 {
			share = freeMain * (it.flex / totalFlex)
		}
		// Re-layout flex child with allocated main size.
		mainAlloc := share
		if mainAlloc < it.mainSize {
			// Allow growing from intrinsic zero; if share is larger use share.
			mainAlloc = share
		}
		// Prefer allocated share when free space exists; else intrinsic.
		if freeMain > 0 {
			mainAlloc = freeMain * (it.flex / totalFlex)
		} else {
			mainAlloc = it.mainSize
		}
		var childC Constraints
		if horizontal {
			childC = Constraints{MinWidth: mainAlloc, MaxWidth: mainAlloc, MaxHeight: maxCross}
			if p.CrossAlign == CrossStretch && c.HasBoundedHeight() {
				childC.MinHeight = maxCross
				childC.MaxHeight = maxCross
			}
		} else {
			childC = Constraints{MaxWidth: maxCross, MinHeight: mainAlloc, MaxHeight: mainAlloc}
			if p.CrossAlign == CrossStretch && c.HasBoundedWidth() {
				childC.MinWidth = maxCross
				childC.MaxWidth = maxCross
			}
		}
		sz := it.node.Layout(childC)
		if horizontal {
			it.mainSize = sz.Width
			it.cross = sz.Height
		} else {
			it.mainSize = sz.Height
			it.cross = sz.Width
		}
	}

	// Content main/cross.
	contentMain := fixedMain + flexIntrinsicMain
	if freeMain > 0 && totalFlex > 0 {
		contentMain = fixedMain + freeMain
	}
	// Recompute contentMain from final item sizes.
	contentMain = 0
	contentCross := 0.0
	for i := range items {
		contentMain += items[i].mainSize
		if items[i].cross > contentCross {
			contentCross = items[i].cross
		}
	}
	contentMain += gapTotal

	// Parent size.
	var out Size
	if horizontal {
		out = c.Tighten(Size{Width: contentMain, Height: contentCross})
		if p.CrossAlign == CrossStretch && c.HasBoundedHeight() && c.MaxHeight < Unbounded {
			out.Height = c.Tighten(Size{Width: out.Width, Height: c.MaxHeight}).Height
		}
	} else {
		out = c.Tighten(Size{Width: contentCross, Height: contentMain})
		if p.CrossAlign == CrossStretch && c.HasBoundedWidth() && c.MaxWidth < Unbounded {
			out.Width = c.Tighten(Size{Width: c.MaxWidth, Height: out.Height}).Width
		}
	}
	// Ensure min.
	if horizontal {
		if out.Width < minMain {
			out.Width = minMain
		}
		if out.Height < minCross {
			out.Height = minCross
		}
	} else {
		if out.Height < minMain {
			out.Height = minMain
		}
		if out.Width < minCross {
			out.Width = minCross
		}
	}
	parent.SetSize(out)

	parentMain := out.Width
	parentCross := out.Height
	if !horizontal {
		parentMain, parentCross = out.Height, out.Width
	}

	// Main-axis free space for alignment.
	remaining := parentMain - contentMain
	if remaining < 0 {
		remaining = 0
	}
	leading, between := mainAxisPlacement(p.MainAlign, remaining, n)

	// Position children.
	mainPos := leading
	for i := range items {
		it := &items[i]
		crossPos := crossAxisOffset(p.CrossAlign, parentCross, it.cross)
		// Stretch: re-layout if needed with exact cross size.
		if p.CrossAlign == CrossStretch {
			crossPos = 0
			if horizontal {
				if it.cross != parentCross {
					childC := Constraints{
						MinWidth: it.mainSize, MaxWidth: it.mainSize,
						MinHeight: parentCross, MaxHeight: parentCross,
					}
					sz := it.node.Layout(childC)
					it.mainSize, it.cross = sz.Width, sz.Height
				}
			} else {
				if it.cross != parentCross {
					childC := Constraints{
						MinWidth: parentCross, MaxWidth: parentCross,
						MinHeight: it.mainSize, MaxHeight: it.mainSize,
					}
					sz := it.node.Layout(childC)
					it.mainSize, it.cross = sz.Height, sz.Width
				}
			}
		}
		if horizontal {
			it.base.SetOffset(Point{X: mainPos, Y: crossPos})
		} else {
			it.base.SetOffset(Point{X: crossPos, Y: mainPos})
		}
		mainPos += it.mainSize + p.Gap + between
	}

	return out
}

func flexChildConstraints(horizontal bool, maxMain, maxCross float64, cross CrossAxisAlignment) Constraints {
	if horizontal {
		c := Constraints{MaxWidth: maxMain, MaxHeight: maxCross}
		if cross == CrossStretch && isFinite(maxCross) {
			c.MinHeight = maxCross
		}
		return c
	}
	c := Constraints{MaxWidth: maxCross, MaxHeight: maxMain}
	if cross == CrossStretch && isFinite(maxCross) {
		c.MinWidth = maxCross
	}
	return c
}

func mainAxisPlacement(align MainAxisAlignment, remaining float64, n int) (leading, between float64) {
	if n <= 0 || remaining <= 0 {
		return 0, 0
	}
	switch align {
	case MainCenter:
		return remaining / 2, 0
	case MainEnd:
		return remaining, 0
	case MainSpaceBetween:
		if n == 1 {
			return 0, 0
		}
		return 0, remaining / float64(n-1)
	case MainSpaceAround:
		if n == 0 {
			return 0, 0
		}
		between = remaining / float64(n)
		return between / 2, between
	case MainSpaceEvenly:
		between = remaining / float64(n+1)
		return between, between
	default: // MainStart
		return 0, 0
	}
}

func crossAxisOffset(align CrossAxisAlignment, parentCross, childCross float64) float64 {
	switch align {
	case CrossCenter:
		return (parentCross - childCross) / 2
	case CrossEnd:
		return parentCross - childCross
	default: // Start, Stretch
		return 0
	}
}

func isFinite(v float64) bool {
	return v < Unbounded/2
}

// layoutFlexWrap packs children into multiple lines along the main axis.
// Flex-grow is not applied (wrap is for fixed/control rows like Ant Space).
func layoutFlexWrap(parent *NodeBase, c Constraints, p FlexLayoutParams, horizontal bool, maxMain, maxCross, minMain, minCross float64) Size {
	kids := parent.children
	n := len(kids)
	type item struct {
		node     Node
		base     *NodeBase
		mainSize float64
		cross    float64
	}
	items := make([]item, n)

	// Measure each child: main max capped by container so a single item can still
	// sit alone on a line; cross is loose (line height comes from max child).
	for i, child := range kids {
		var childC Constraints
		if horizontal {
			childC = Constraints{MaxWidth: maxMain, MaxHeight: maxCross}
		} else {
			childC = Constraints{MaxWidth: maxCross, MaxHeight: maxMain}
		}
		sz := child.Layout(childC)
		it := item{node: child, base: child.Base()}
		if horizontal {
			it.mainSize = sz.Width
			it.cross = sz.Height
		} else {
			it.mainSize = sz.Height
			it.cross = sz.Width
		}
		items[i] = it
	}

	// Pack into lines (indices into items).
	type line struct {
		start, end int // half-open [start, end)
		main       float64
		cross      float64
	}
	var lines []line
	lineStart := 0
	lineMain := 0.0
	lineCross := 0.0
	for i := 0; i < n; i++ {
		need := items[i].mainSize
		if i > lineStart {
			need += p.Gap
		}
		if i > lineStart && lineMain+need > maxMain {
			lines = append(lines, line{start: lineStart, end: i, main: lineMain, cross: lineCross})
			lineStart = i
			lineMain = items[i].mainSize
			lineCross = items[i].cross
			continue
		}
		if i > lineStart {
			lineMain += p.Gap
		}
		lineMain += items[i].mainSize
		if items[i].cross > lineCross {
			lineCross = items[i].cross
		}
	}
	if lineStart < n {
		lines = append(lines, line{start: lineStart, end: n, main: lineMain, cross: lineCross})
	}

	// Content size: main = max line main; cross = sum line crosses + gaps between lines.
	contentMain := 0.0
	contentCross := 0.0
	for i, ln := range lines {
		if ln.main > contentMain {
			contentMain = ln.main
		}
		contentCross += ln.cross
		if i > 0 {
			contentCross += p.Gap
		}
	}

	var out Size
	if horizontal {
		out = c.Tighten(Size{Width: contentMain, Height: contentCross})
	} else {
		out = c.Tighten(Size{Width: contentCross, Height: contentMain})
	}
	if horizontal {
		if out.Width < minMain {
			out.Width = minMain
		}
		if out.Height < minCross {
			out.Height = minCross
		}
	} else {
		if out.Height < minMain {
			out.Height = minMain
		}
		if out.Width < minCross {
			out.Width = minCross
		}
	}
	// When parent is stretched to a larger main (e.g. Max==Min), lines align within that.
	parentMain := out.Width
	if !horizontal {
		parentMain = out.Height
	}
	parent.SetSize(out)

	// Position each line then each child within the line.
	crossPos := 0.0
	for li, ln := range lines {
		if li > 0 {
			crossPos += p.Gap
		}
		count := ln.end - ln.start
		remaining := parentMain - ln.main
		if remaining < 0 {
			remaining = 0
		}
		leading, between := mainAxisPlacement(p.MainAlign, remaining, count)
		mainPos := leading
		// Line cross for CrossAlign / Stretch within this line.
		lineCross := ln.cross
		for i := ln.start; i < ln.end; i++ {
			it := &items[i]
			// Stretch re-layout to line cross size.
			if p.CrossAlign == CrossStretch {
				var childC Constraints
				if horizontal {
					childC = Constraints{
						MinWidth: it.mainSize, MaxWidth: it.mainSize,
						MinHeight: lineCross, MaxHeight: lineCross,
					}
				} else {
					childC = Constraints{
						MinWidth: lineCross, MaxWidth: lineCross,
						MinHeight: it.mainSize, MaxHeight: it.mainSize,
					}
				}
				sz := it.node.Layout(childC)
				if horizontal {
					it.mainSize, it.cross = sz.Width, sz.Height
				} else {
					it.mainSize, it.cross = sz.Height, sz.Width
				}
			}
			childCrossOff := crossAxisOffset(p.CrossAlign, lineCross, it.cross)
			if horizontal {
				it.base.SetOffset(Point{X: mainPos, Y: crossPos + childCrossOff})
			} else {
				it.base.SetOffset(Point{X: crossPos + childCrossOff, Y: mainPos})
			}
			mainPos += it.mainSize + p.Gap + between
		}
		crossPos += lineCross
	}
	return out
}
