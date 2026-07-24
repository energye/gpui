package kit

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/energye/gpui/ui/core"
)

// Ant Design Grid (Row + Col) — 24 columns.
// Source: components/grid/{row,col}.tsx + style/index.ts
// Product contract: docs/antd/grid.md §6 (P0 DoD).
// https://ant.design/components/grid

const (
	// GridColumns is antd gridColumns (24).
	GridColumns = 24

	// Default metric fallbacks (§6.2).
	DefaultGridFontSize     = 14.0
	DefaultGridBorderRadius = 6.0
	DefaultGridLineWidth    = 1.0

	// Breakpoints — antd screen*Min (Bootstrap-derived).
	ScreenXS   = 0.0
	ScreenSM   = 576.0
	ScreenMD   = 768.0
	ScreenLG   = 992.0
	ScreenXL   = 1200.0
	ScreenXXL  = 1600.0
	ScreenXXXL = 1920.0
)

// RowAlign is antd Row.align: top | middle | bottom | stretch.
type RowAlign int

const (
	RowAlignTop RowAlign = iota
	RowAlignMiddle
	RowAlignBottom
	RowAlignStretch
)

// RowJustify is antd Row.justify.
type RowJustify int

const (
	RowJustifyStart RowJustify = iota
	RowJustifyEnd
	RowJustifyCenter
	RowJustifySpaceAround
	RowJustifySpaceBetween
	RowJustifySpaceEvenly
)

// ColFlex describes parsed Col.flex (antd parseFlex).
type ColFlex struct {
	// Active is true when flex was set (overrides span sizing when true).
	Active bool
	// Grow/Shrink participate in free-space distribution on the line.
	Grow, Shrink float64
	// BasisPx >= 0 is a fixed main-size basis; BasisAuto uses content min size.
	BasisPx   float64
	BasisAuto bool
}

// Row is antd Row: flex-flow row wrap container for Col children.
//
//	Row (custom packer, ExpandMax width)
//	  └─ Col × N
//	       └─ children…
//
// Layout-only: no press / disabled / loading chrome. hit == layout == paint.
type Row struct {
	root *gridRow

	Align   RowAlign
	Justify RowJustify
	Wrap    bool
	GutterH float64
	GutterV float64
	// ViewportWidth drives Col responsive span (xs…xxxl). 0 → use layout MaxWidth.
	ViewportWidth float64
	viewportSet   bool
	AriaLabel     string
	Theme         *core.Theme
}

// Col is antd Col: span/offset/order/push/pull/flex cell inside a Row.
type Col struct {
	root *gridCol

	// spanSet distinguishes "unset" from explicit 0 (hidden).
	span    int
	spanSet bool

	Offset int
	Order  int
	Push   int
	Pull   int

	Flex ColFlex

	// Responsive span overrides (P0: number span only). -1 = unset.
	xs, sm, md, lg, xl, xxl, xxxl                      int
	xsSet, smSet, mdSet, lgSet, xlSet, xxlSet, xxxlSet bool

	// padH is applied by parent Row from gutterH/2 (antd paddingInline).
	padH float64
}

// --- constructors -----------------------------------------------------------

// NewRow creates an antd Row (wrap=true, align=top, justify=start, gutter=0).
func NewRow(children ...core.Node) *Row {
	r := &Row{
		Align:   RowAlignTop,
		Justify: RowJustifyStart,
		Wrap:    true,
	}
	r.root = &gridRow{owner: r}
	r.root.Init(r.root)
	r.root.Hit = core.HitDefer
	for _, c := range children {
		if c != nil {
			r.root.AddChild(c)
		}
	}
	r.applyA11y()
	return r
}

// NewGrid is the product entry alias for NewRow (docs/antd/grid.md §6.10 / GRD-01).
func NewGrid(children ...core.Node) *Row { return NewRow(children...) }

// NewCol creates an antd Col with optional children.
func NewCol(children ...core.Node) *Col {
	c := &Col{
		xs: -1, sm: -1, md: -1, lg: -1, xl: -1, xxl: -1, xxxl: -1,
	}
	c.root = &gridCol{owner: c}
	c.root.Init(c.root)
	c.root.Hit = core.HitDefer
	for _, ch := range children {
		if ch != nil {
			c.root.AddChild(ch)
		}
	}
	return c
}

// --- Row API ----------------------------------------------------------------

// Node returns the stable layout root.
func (r *Row) Node() core.Node {
	if r == nil {
		return nil
	}
	if r.root == nil {
		r.root = &gridRow{owner: r}
		r.root.Init(r.root)
		r.root.Hit = core.HitDefer
	}
	r.applyA11y()
	return r.root
}

// ChromeNode returns the layout root (visual tests / composition).
func (r *Row) ChromeNode() core.Node { return r.Node() }

// SetAlign sets vertical alignment of cols within a line.
func (r *Row) SetAlign(a RowAlign) {
	if r == nil {
		return
	}
	r.Align = a
	r.mark()
}

// SetJustify sets horizontal free-space distribution on each line.
func (r *Row) SetJustify(j RowJustify) {
	if r == nil {
		return
	}
	r.Justify = j
	r.mark()
}

// SetWrap enables multi-line packing (antd default true).
func (r *Row) SetWrap(v bool) {
	if r == nil {
		return
	}
	r.Wrap = v
	r.mark()
}

// SetGutter sets horizontal gutter (antd gutter={n}).
func (r *Row) SetGutter(h float64) {
	if r == nil {
		return
	}
	if h < 0 {
		h = 0
	}
	r.GutterH = h
	r.mark()
}

// SetGutterHV sets [horizontal, vertical] gutter (antd gutter={[h,v]}).
func (r *Row) SetGutterHV(h, v float64) {
	if r == nil {
		return
	}
	if h < 0 {
		h = 0
	}
	if v < 0 {
		v = 0
	}
	r.GutterH, r.GutterV = h, v
	r.mark()
}

// SetViewportWidth sets the width used for Col breakpoint resolution.
// When unset (viewportSet=false), layout MaxWidth is used.
func (r *Row) SetViewportWidth(w float64) {
	if r == nil {
		return
	}
	r.ViewportWidth = w
	r.viewportSet = true
	r.mark()
}

// SetTheme stores theme (metric fallbacks; Grid chrome is transparent).
func (r *Row) SetTheme(th *core.Theme) {
	if r == nil {
		return
	}
	r.Theme = th
	r.mark()
}

// SetAriaLabel sets an optional accessible name on the layout root.
func (r *Row) SetAriaLabel(s string) {
	if r == nil {
		return
	}
	r.AriaLabel = s
	r.applyA11y()
}

// Add appends a child (typically *Col.Node()).
func (r *Row) Add(n core.Node) {
	if r == nil {
		return
	}
	_ = r.Node()
	if n != nil {
		r.root.AddChild(n)
	}
	r.mark()
}

// SetChildren replaces all children.
func (r *Row) SetChildren(children ...core.Node) {
	if r == nil {
		return
	}
	_ = r.Node()
	r.root.ClearChildren()
	for _, c := range children {
		if c != nil {
			r.root.AddChild(c)
		}
	}
	r.mark()
}

// ClearChildren removes all children.
func (r *Row) ClearChildren() {
	if r == nil || r.root == nil {
		return
	}
	r.root.ClearChildren()
	r.mark()
}

// ResolvedViewport returns the width used for breakpoint resolution.
func (r *Row) ResolvedViewport(layoutMaxW float64) float64 {
	if r == nil {
		return layoutMaxW
	}
	if r.viewportSet && r.ViewportWidth > 0 {
		return r.ViewportWidth
	}
	if layoutMaxW > 0 && layoutMaxW < core.Unbounded {
		return layoutMaxW
	}
	return ScreenXL // desktop default when unbounded
}

func (r *Row) theme() *core.Theme {
	if r != nil && r.Theme != nil {
		return r.Theme
	}
	return DefaultTheme()
}

func (r *Row) mark() {
	if r != nil && r.root != nil {
		r.root.MarkNeedsLayout()
	}
}

func (r *Row) applyA11y() {
	if r == nil || r.root == nil {
		return
	}
	r.root.Base().Label = r.AriaLabel
}

// --- Col API ----------------------------------------------------------------

// Node returns the stable layout root.
func (c *Col) Node() core.Node {
	if c == nil {
		return nil
	}
	if c.root == nil {
		c.root = &gridCol{owner: c}
		c.root.Init(c.root)
		c.root.Hit = core.HitDefer
	}
	return c.root
}

// SetSpan sets grid span (0 = hidden). Negative clears span mode.
func (c *Col) SetSpan(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		c.spanSet = false
		c.span = 0
	} else {
		c.spanSet = true
		c.span = n
	}
	c.mark()
}

// SetOffset sets left gutter columns.
func (c *Col) SetOffset(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	c.Offset = n
	c.mark()
}

// SetOrder sets flex order.
func (c *Col) SetOrder(n int) {
	if c == nil {
		return
	}
	c.Order = n
	c.mark()
}

// SetPush shifts the col right by n columns (antd push).
func (c *Col) SetPush(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	c.Push = n
	c.mark()
}

// SetPull shifts the col left by n columns (antd pull).
func (c *Col) SetPull(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	c.Pull = n
	c.mark()
}

// SetFlexNumber sets flex from a number (antd: `n n auto`).
func (c *Col) SetFlexNumber(n float64) {
	if c == nil {
		return
	}
	c.Flex = ColFlex{Active: true, Grow: n, Shrink: n, BasisAuto: true}
	c.mark()
}

// SetFlexAuto sets flex="auto" → 1 1 auto.
func (c *Col) SetFlexAuto() {
	if c == nil {
		return
	}
	c.Flex = ColFlex{Active: true, Grow: 1, Shrink: 1, BasisAuto: true}
	c.mark()
}

// SetFlexNone sets flex="none" → 0 0 auto.
func (c *Col) SetFlexNone() {
	if c == nil {
		return
	}
	c.Flex = ColFlex{Active: true, Grow: 0, Shrink: 0, BasisAuto: true}
	c.mark()
}

// SetFlexString parses antd flex string ("auto", "none", "100px", "1 1 200px", "2").
func (c *Col) SetFlexString(s string) {
	if c == nil {
		return
	}
	c.Flex = parseColFlex(s)
	c.mark()
}

// SetXs/Sm/Md/… set responsive span numbers (P0).
func (c *Col) SetXs(span int)   { c.setBP(&c.xs, &c.xsSet, span) }
func (c *Col) SetSm(span int)   { c.setBP(&c.sm, &c.smSet, span) }
func (c *Col) SetMd(span int)   { c.setBP(&c.md, &c.mdSet, span) }
func (c *Col) SetLg(span int)   { c.setBP(&c.lg, &c.lgSet, span) }
func (c *Col) SetXl(span int)   { c.setBP(&c.xl, &c.xlSet, span) }
func (c *Col) SetXxl(span int)  { c.setBP(&c.xxl, &c.xxlSet, span) }
func (c *Col) SetXxxl(span int) { c.setBP(&c.xxxl, &c.xxxlSet, span) }

func (c *Col) setBP(dst *int, set *bool, span int) {
	if c == nil {
		return
	}
	if span < 0 {
		*set = false
		*dst = -1
	} else {
		*set = true
		*dst = span
	}
	c.mark()
}

// SetChildren replaces col children.
func (c *Col) SetChildren(children ...core.Node) {
	if c == nil {
		return
	}
	_ = c.Node()
	c.root.ClearChildren()
	for _, ch := range children {
		if ch != nil {
			c.root.AddChild(ch)
		}
	}
	c.mark()
}

// Add appends a child.
func (c *Col) Add(n core.Node) {
	if c == nil {
		return
	}
	_ = c.Node()
	if n != nil {
		c.root.AddChild(n)
	}
	c.mark()
}

func (c *Col) mark() {
	if c != nil && c.root != nil {
		c.root.MarkNeedsLayout()
	}
}

// ResolvedSpan returns effective span for viewport width (and whether span mode is active).
// hidden=true when span resolves to 0.
func (c *Col) ResolvedSpan(viewport float64) (span int, spanMode bool, hidden bool) {
	if c == nil {
		return 0, false, true
	}
	// Responsive: largest matching breakpoint wins (antd responsiveArray large→small).
	type bp struct {
		min float64
		set bool
		v   int
	}
	bps := []bp{
		{ScreenXXXL, c.xxxlSet, c.xxxl},
		{ScreenXXL, c.xxlSet, c.xxl},
		{ScreenXL, c.xlSet, c.xl},
		{ScreenLG, c.lgSet, c.lg},
		{ScreenMD, c.mdSet, c.md},
		{ScreenSM, c.smSet, c.sm},
		{ScreenXS, c.xsSet, c.xs},
	}
	for _, b := range bps {
		if !b.set {
			continue
		}
		// xs matches always when set; others need viewport >= min.
		if b.min == ScreenXS || viewport >= b.min {
			if b.v == 0 {
				return 0, true, true
			}
			return b.v, true, false
		}
	}
	if c.spanSet {
		if c.span == 0 {
			return 0, true, true
		}
		return c.span, true, false
	}
	return 0, false, false
}

// --- flex parse -------------------------------------------------------------

func parseColFlex(s string) ColFlex {
	s = strings.TrimSpace(s)
	if s == "" {
		return ColFlex{}
	}
	switch s {
	case "auto":
		return ColFlex{Active: true, Grow: 1, Shrink: 1, BasisAuto: true}
	case "none":
		return ColFlex{Active: true, Grow: 0, Shrink: 0, BasisAuto: true}
	}
	// Pure number string "n" → flex: n 1 0 (antd doc).
	if n, err := strconv.ParseFloat(s, 64); err == nil {
		return ColFlex{Active: true, Grow: n, Shrink: 1, BasisPx: 0, BasisAuto: false}
	}
	// Dimension: 100px / 10% / 2rem → 0 0 dim (only px supported numerically).
	if strings.HasSuffix(s, "px") {
		if n, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 64); err == nil {
			return ColFlex{Active: true, Grow: 0, Shrink: 0, BasisPx: n}
		}
	}
	// Three-part: "1 1 200px" or "1 1 auto"
	parts := strings.Fields(s)
	if len(parts) >= 3 {
		g, _ := strconv.ParseFloat(parts[0], 64)
		sh, _ := strconv.ParseFloat(parts[1], 64)
		basis := parts[2]
		if basis == "auto" {
			return ColFlex{Active: true, Grow: g, Shrink: sh, BasisAuto: true}
		}
		if strings.HasSuffix(basis, "px") {
			if n, err := strconv.ParseFloat(strings.TrimSuffix(basis, "px"), 64); err == nil {
				return ColFlex{Active: true, Grow: g, Shrink: sh, BasisPx: n}
			}
		}
		return ColFlex{Active: true, Grow: g, Shrink: sh, BasisAuto: true}
	}
	// Fallback: treat as grow number if possible.
	return ColFlex{Active: true, Grow: 1, Shrink: 1, BasisAuto: true}
}

// --- layout nodes -----------------------------------------------------------

type gridRow struct {
	core.NodeBase
	owner *Row
}

func (g *gridRow) TypeID() string { return "kit.Row" }

func (g *gridRow) Layout(c core.Constraints) core.Size {
	if sz, ok := g.LayoutSkipIfClean(c); ok {
		return sz
	}
	r := g.owner
	if r == nil {
		out := c.Tighten(core.Size{})
		g.SetSize(out)
		g.RememberConstraints(c)
		return out
	}

	// Block-level width fill (antd .ant-row is flex container full width).
	rowW := 0.0
	if c.HasBoundedWidth() {
		rowW = c.MaxWidth
		if c.MinWidth > rowW {
			rowW = c.MinWidth
		}
	}

	viewport := r.ResolvedViewport(rowW)
	gutterH, gutterV := r.GutterH, r.GutterV
	unit := 0.0
	if rowW > 0 {
		unit = rowW / float64(GridColumns)
	}

	type item struct {
		col    *Col
		node   core.Node
		order  int
		idx    int
		span   int
		offset int
		push   int
		pull   int
		hidden bool
		flex   ColFlex
		// measured
		baseMain float64 // before grow
		cross    float64
		main     float64 // final main (incl. offset margin consumed)
		contentW float64 // laid-out content box width
		// placement
		flowX   float64
		visualX float64
		y       float64
	}

	kids := g.Children()
	items := make([]item, 0, len(kids))
	for i, ch := range kids {
		it := item{node: ch, idx: i, order: 0}
		if gc, ok := ch.(*gridCol); ok && gc.owner != nil {
			col := gc.owner
			it.col = col
			it.order = col.Order
			it.offset = col.Offset
			it.push = col.Push
			it.pull = col.Pull
			it.flex = col.Flex
			span, spanMode, hidden := col.ResolvedSpan(viewport)
			it.hidden = hidden
			if hidden {
				// span 0 → display:none
				it.span = 0
			} else if spanMode {
				it.span = span
			} else if !col.Flex.Active {
				// no span & no flex → content-sized (antd auto)
				it.span = -1
			}
			// pad for gutter
			col.padH = gutterH / 2
		}
		items = append(items, it)
	}

	// order (stable)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order != items[j].order {
			return items[i].order < items[j].order
		}
		return items[i].idx < items[j].idx
	})

	// If width unbounded, estimate from span/flex content once.
	if rowW <= 0 {
		// Prefer content-driven width: measure each at loose constraints.
		var total float64
		var maxCross float64
		for i := range items {
			it := &items[i]
			if it.hidden {
				it.node.Layout(core.Tight(0, 0))
				it.node.Base().SetOffset(core.Point{})
				it.node.Base().SetSize(core.Size{})
				continue
			}
			sz := it.node.Layout(core.Loose(core.Unbounded, c.MaxHeight))
			it.baseMain = sz.Width
			it.cross = sz.Height
			it.main = sz.Width
			it.contentW = sz.Width
			total += sz.Width
			if sz.Height > maxCross {
				maxCross = sz.Height
			}
		}
		// place in one line start
		x := 0.0
		for i := range items {
			it := &items[i]
			if it.hidden {
				continue
			}
			it.node.Base().SetOffset(core.Point{X: x, Y: 0})
			x += it.main
		}
		out := c.Tighten(core.Size{Width: total, Height: maxCross})
		g.SetSize(out)
		g.RememberConstraints(c)
		return out
	}

	// Pack into lines.
	type line struct {
		items []int // indices into items
		// sum of flow main before justify
		flowMain float64
		// grow total for flex items
		grow float64
		// fixed main of non-grow
		fixedMain float64
		cross     float64
	}
	var lines []line
	cur := line{}
	flowX := 0.0

	flush := func() {
		if len(cur.items) == 0 {
			return
		}
		lines = append(lines, cur)
		cur = line{}
		flowX = 0
	}

	for i := range items {
		it := &items[i]
		if it.hidden {
			// zero-size layout
			it.node.Layout(core.Tight(0, 0))
			it.node.Base().SetSize(core.Size{})
			it.node.Base().SetOffset(core.Point{})
			continue
		}

		// Determine flow contribution (offset margin + main box).
		var flowNeed, mainBox float64
		if it.flex.Active {
			// Flex basis
			if it.flex.BasisAuto {
				// measure intrinsic with loose main
				childMaxH := c.MaxHeight
				sz := it.node.Layout(core.Loose(core.Unbounded, childMaxH))
				it.baseMain = sz.Width
				it.cross = sz.Height
			} else {
				it.baseMain = it.flex.BasisPx
			}
			mainBox = it.baseMain
			flowNeed = mainBox // offset still applies for flex cols
			if it.offset > 0 {
				flowNeed += float64(it.offset) * unit
			}
		} else if it.span >= 0 {
			// span mode (including content-auto span==-1 handled below)
			if it.span > 0 {
				mainBox = float64(it.span) * unit
			} else {
				// span==-1 content size
				sz := it.node.Layout(core.Loose(rowW, c.MaxHeight))
				it.baseMain = sz.Width
				it.cross = sz.Height
				mainBox = sz.Width
			}
			flowNeed = mainBox + float64(it.offset)*unit
		} else {
			sz := it.node.Layout(core.Loose(rowW, c.MaxHeight))
			it.baseMain = sz.Width
			it.cross = sz.Height
			mainBox = sz.Width
			flowNeed = mainBox + float64(it.offset)*unit
		}
		it.main = mainBox

		// wrap?
		if r.Wrap && len(cur.items) > 0 && flowX+flowNeed > rowW+0.5 {
			flush()
		}
		it.flowX = flowX + float64(it.offset)*unit
		flowX += flowNeed
		cur.flowMain = flowX
		cur.items = append(cur.items, i)
		if it.flex.Active && it.flex.Grow > 0 {
			cur.grow += it.flex.Grow
		} else {
			cur.fixedMain += flowNeed
		}
	}
	flush()

	// Second pass: distribute flex grow per line, layout content, place.
	var y float64
	var maxLineW float64
	for li := range lines {
		ln := &lines[li]
		// Free space on line for flex-grow and justify.
		// flowMain currently = sum of bases + offsets for all items as packed.
		// Recompute fixed vs grow bases more carefully.
		var basisSum float64
		var growSum float64
		for _, ii := range ln.items {
			it := &items[ii]
			off := float64(it.offset) * unit
			if it.flex.Active {
				basisSum += it.baseMain + off
				if it.flex.Grow > 0 {
					growSum += it.flex.Grow
				}
			} else {
				basisSum += it.main + off
			}
		}
		free := rowW - basisSum
		if free < 0 {
			free = 0
		}

		// Assign flex grow widths.
		for _, ii := range ln.items {
			it := &items[ii]
			if it.flex.Active && it.flex.Grow > 0 && growSum > 0 {
				it.main = it.baseMain + free*(it.flex.Grow/growSum)
			}
		}
		// After grow, remaining free for justify (only non-flex-grow lines or leftover).
		var used float64
		for _, ii := range ln.items {
			it := &items[ii]
			used += it.main + float64(it.offset)*unit
		}
		remain := rowW - used
		if remain < 0 {
			remain = 0
		}
		leading, between := rowMainPlacement(r.Justify, remain, len(ln.items))

		// Layout each col to final content size and measure cross.
		var lineCross float64
		for _, ii := range ln.items {
			it := &items[ii]
			contentW := it.main
			if contentW < 0 {
				contentW = 0
			}
			it.contentW = contentW
			// Pass pad via col.padH; layout col with tight width.
			childC := core.Constraints{
				MinWidth: contentW, MaxWidth: contentW,
				MaxHeight: c.MaxHeight,
			}
			if r.Align == RowAlignStretch && c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded && c.MaxHeight > 0 {
				// definite stretch later after lineCross known — first measure loose cross
			}
			sz := it.node.Layout(childC)
			it.cross = sz.Height
			if it.cross > lineCross {
				lineCross = it.cross
			}
		}
		// Stretch re-layout when align stretch and line has height.
		if r.Align == RowAlignStretch && lineCross > 0 {
			for _, ii := range ln.items {
				it := &items[ii]
				childC := core.Constraints{
					MinWidth: it.contentW, MaxWidth: it.contentW,
					MinHeight: lineCross, MaxHeight: lineCross,
				}
				sz := it.node.Layout(childC)
				it.cross = sz.Height
			}
		}
		ln.cross = lineCross

		// Position
		x := leading
		for _, ii := range ln.items {
			it := &items[ii]
			flowStart := x + float64(it.offset)*unit
			visualX := flowStart + float64(it.push-it.pull)*unit
			crossPos := rowCrossOffset(r.Align, lineCross, it.cross)
			it.node.Base().SetOffset(core.Point{X: visualX, Y: y + crossPos})
			// Ensure size is content box (col layout already set size).
			x += it.main + float64(it.offset)*unit + between
		}
		if rowW > maxLineW {
			maxLineW = rowW
		}
		y += lineCross
		if li < len(lines)-1 {
			y += gutterV
		}
	}

	// Hidden already zeroed.
	outH := y
	if outH < c.MinHeight {
		outH = c.MinHeight
	}
	outW := rowW
	if outW < c.MinWidth {
		outW = c.MinWidth
	}
	out := c.Tighten(core.Size{Width: outW, Height: outH})
	// Block fill: prefer full max width when bounded.
	if c.HasBoundedWidth() && out.Width < c.MaxWidth && c.MaxWidth < core.Unbounded {
		out.Width = c.MaxWidth
		if out.Width < c.MinWidth {
			out.Width = c.MinWidth
		}
	}
	g.SetSize(out)
	g.RememberConstraints(c)
	return out
}

func (g *gridRow) Paint(pc *core.PaintContext) { g.DefaultPaintChildren(pc) }

func (g *gridRow) HitTest(p core.Point) core.Node { return g.DefaultHitTest(p) }

type gridCol struct {
	core.NodeBase
	owner *Col
}

func (g *gridCol) TypeID() string { return "kit.Col" }

func (g *gridCol) Layout(c core.Constraints) core.Size {
	if sz, ok := g.LayoutSkipIfClean(c); ok {
		return sz
	}
	col := g.owner
	pad := 0.0
	if col != nil {
		pad = col.padH
	}
	if pad < 0 {
		pad = 0
	}

	// Outer size is constraints (Row assigns tight width).
	// Unbounded / zero max → intrinsic content measure (flex-basis:auto).
	bounded := c.HasBoundedWidth() && c.MaxWidth > 0
	outerW := 0.0
	if bounded {
		outerW = c.MaxWidth
		if c.MinWidth > outerW {
			outerW = c.MinWidth
		}
	}

	innerMaxW := core.Unbounded
	if bounded {
		innerMaxW = outerW - 2*pad
		if innerMaxW < 0 {
			innerMaxW = 0
		}
	}
	innerMaxH := c.MaxHeight
	innerMinH := 0.0
	if c.MinHeight == c.MaxHeight && c.MaxHeight < core.Unbounded {
		innerMinH = c.MinHeight
	}

	kids := g.Children()
	var contentH float64
	var contentW float64
	if len(kids) == 0 {
		// antd minHeight: 1 to prevent collapse
		contentH = 1
		if innerMinH > contentH {
			contentH = innerMinH
		}
	} else if len(kids) == 1 {
		childC := core.Constraints{
			MinWidth: 0, MaxWidth: innerMaxW,
			MinHeight: innerMinH, MaxHeight: innerMaxH,
		}
		// stretch: fill assigned width when Row gave a definite outer
		if bounded {
			childC.MinWidth = innerMaxW
			childC.MaxWidth = innerMaxW
		}
		sz := kids[0].Layout(childC)
		kids[0].Base().SetOffset(core.Point{X: pad, Y: 0})
		contentW = sz.Width
		contentH = sz.Height
	} else {
		// stack children vertically inside col (rare; demos are single child)
		var y float64
		var maxW float64
		for _, ch := range kids {
			childC := core.Constraints{MaxWidth: innerMaxW, MaxHeight: innerMaxH}
			sz := ch.Layout(childC)
			ch.Base().SetOffset(core.Point{X: pad, Y: y})
			y += sz.Height
			if sz.Width > maxW {
				maxW = sz.Width
			}
		}
		contentW = maxW
		contentH = y
	}
	if contentH < 1 {
		contentH = 1
	}
	if innerMinH > contentH {
		contentH = innerMinH
	}

	outW := outerW
	if !bounded {
		outW = contentW + 2*pad
	}
	outH := contentH
	out := c.Tighten(core.Size{Width: outW, Height: outH})
	g.SetSize(out)
	g.RememberConstraints(c)
	return out
}

func (g *gridCol) Paint(pc *core.PaintContext) { g.DefaultPaintChildren(pc) }

func (g *gridCol) HitTest(p core.Point) core.Node { return g.DefaultHitTest(p) }

// --- placement helpers ------------------------------------------------------

func rowMainPlacement(j RowJustify, remaining float64, n int) (leading, between float64) {
	if n <= 0 || remaining <= 0 {
		return 0, 0
	}
	switch j {
	case RowJustifyCenter:
		return remaining / 2, 0
	case RowJustifyEnd:
		return remaining, 0
	case RowJustifySpaceBetween:
		if n == 1 {
			return 0, 0
		}
		return 0, remaining / float64(n-1)
	case RowJustifySpaceAround:
		gap := remaining / float64(n)
		return gap / 2, gap
	case RowJustifySpaceEvenly:
		gap := remaining / float64(n+1)
		return gap, gap
	default:
		return 0, 0
	}
}

func rowCrossOffset(a RowAlign, lineCross, childCross float64) float64 {
	switch a {
	case RowAlignMiddle:
		return math.Max(0, (lineCross-childCross)/2)
	case RowAlignBottom:
		return math.Max(0, lineCross-childCross)
	case RowAlignStretch:
		return 0
	default: // top
		return 0
	}
}
