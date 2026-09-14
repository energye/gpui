// Package grid implements the grid control (docs/antd/grid.md §6).
//
// Composition over new frameworks: Row/Col own rendering.AbsoluteBox nodes
// and reuse ui/rendering for layout/paint, ui/theme for tokens.
// No new event or frame system; pure layout container, no chrome.
package grid

import (
	"sort"
	"strconv"
	"strings"

	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/theme"
)

// GridColumns is the antd 24-column base (docs/antd/grid.md §6.2.1).
const GridColumns = 24

// Breakpoint widths (docs/antd/grid.md §6.2.1).
const (
	ScreenSM   = 576.0
	ScreenMD   = 768.0
	ScreenLG   = 992.0
	ScreenXL   = 1200.0
	ScreenXXL  = 1600.0
	ScreenXXXL = 1920.0
)

// RowAlign selects cross-axis alignment.
type RowAlign string

const (
	RowAlignTop     RowAlign = "top"
	RowAlignMiddle  RowAlign = "middle"
	RowAlignBottom  RowAlign = "bottom"
	RowAlignStretch RowAlign = "stretch"
)

// RowJustify selects main-axis distribution.
type RowJustify string

const (
	RowJustifyStart        RowJustify = "start"
	RowJustifyEnd          RowJustify = "end"
	RowJustifyCenter       RowJustify = "center"
	RowJustifySpaceAround  RowJustify = "space-around"
	RowJustifySpaceBetween RowJustify = "space-between"
	RowJustifySpaceEvenly  RowJustify = "space-evenly"
)

// Row is the grid row container (docs/antd/grid.md §6.10).
type Row struct {
	align         RowAlign
	justify       RowJustify
	wrap          bool
	gutterH       float64
	gutterV       float64
	viewportWidth float64
	provider      *theme.Provider
	override      *theme.Tokens
	ariaLabel     string
	cols          []*Col
	node          *rendering.AbsoluteBox
}

// NewRow creates a row with default align top, justify start, wrap true.
func NewRow(cols ...*Col) *Row {
	r := &Row{align: RowAlignTop, justify: RowJustifyStart, wrap: true}
	r.node = rendering.NewAbsoluteBox(0, 0)
	r.node.SetRepaintBoundary(true)
	r.node.SetRelayoutBoundary(true)
	for _, c := range cols {
		r.Add(c)
	}
	return r
}

// NewGrid is the §6 product-name alias of NewRow.
func NewGrid(cols ...*Col) *Row { return NewRow(cols...) }

// SetAlign sets vertical alignment (unknown values ignored).
func (r *Row) SetAlign(a RowAlign) {
	if r == nil {
		return
	}
	switch a {
	case RowAlignTop, RowAlignMiddle, RowAlignBottom, RowAlignStretch:
	default:
		return
	}
	if r.align == a {
		return
	}
	r.align = a
	r.node.MarkNeedsLayout()
}

// Align returns the effective align.
func (r *Row) Align() RowAlign {
	if r == nil || r.align == "" {
		return RowAlignTop
	}
	return r.align
}

// SetJustify sets horizontal distribution (unknown values ignored).
func (r *Row) SetJustify(j RowJustify) {
	if r == nil {
		return
	}
	switch j {
	case RowJustifyStart, RowJustifyEnd, RowJustifyCenter,
		RowJustifySpaceAround, RowJustifySpaceBetween, RowJustifySpaceEvenly:
	default:
		return
	}
	if r.justify == j {
		return
	}
	r.justify = j
	r.node.MarkNeedsLayout()
}

// Justify returns the effective justify.
func (r *Row) Justify() RowJustify {
	if r == nil || r.justify == "" {
		return RowJustifyStart
	}
	return r.justify
}

// SetWrap toggles multi-line layout.
func (r *Row) SetWrap(b bool) {
	if r == nil || r.wrap == b {
		return
	}
	r.wrap = b
	r.node.MarkNeedsLayout()
}

// Wrap reports the wrap flag.
func (r *Row) Wrap() bool { return r == nil || r.wrap }

// SetGutter sets horizontal gutter, vertical 0 (antd number form).
func (r *Row) SetGutter(h float64) {
	if r == nil {
		return
	}
	if h < 0 {
		h = 0
	}
	if r.gutterH == h && r.gutterV == 0 {
		return
	}
	r.gutterH, r.gutterV = h, 0
	r.node.MarkNeedsLayout()
}

// SetGutterHV sets [horizontal, vertical] gutter (antd array form).
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
	if r.gutterH == h && r.gutterV == v {
		return
	}
	r.gutterH, r.gutterV = h, v
	r.node.MarkNeedsLayout()
}

// GutterH returns horizontal gutter.
func (r *Row) GutterH() float64 {
	if r == nil {
		return 0
	}
	return r.gutterH
}

// GutterV returns vertical gutter.
func (r *Row) GutterV() float64 {
	if r == nil {
		return 0
	}
	return r.gutterV
}

// SetViewportWidth sets breakpoint viewport (0 = use layout MaxWidth).
func (r *Row) SetViewportWidth(w float64) {
	if r == nil {
		return
	}
	if w < 0 {
		w = 0
	}
	if r.viewportWidth == w {
		return
	}
	r.viewportWidth = w
	r.node.MarkNeedsLayout()
}

// ViewportWidth returns the override viewport.
func (r *Row) ViewportWidth() float64 {
	if r == nil {
		return 0
	}
	return r.viewportWidth
}

// SetProvider selects the theme source (nil selects process default).
func (r *Row) SetProvider(p *theme.Provider) {
	if r == nil {
		return
	}
	r.provider = p
	r.node.MarkNeedsPaint()
}

// SetTheme pins exact tokens (nil clears to provider).
func (r *Row) SetTheme(t *theme.Tokens) {
	if r == nil {
		return
	}
	r.override = t
	r.node.MarkNeedsPaint()
}

// EffectiveTokens returns override, provider, or default tokens.
func (r *Row) EffectiveTokens() theme.Tokens {
	if r != nil && r.override != nil {
		return *r.override
	}
	if r != nil && r.provider != nil {
		return r.provider.Current()
	}
	return theme.Default.Current()
}

// SetAriaLabel names the container (empty keeps it unnamed).
func (r *Row) SetAriaLabel(s string) {
	if r == nil || r.ariaLabel == s {
		return
	}
	r.ariaLabel = s
	r.node.MarkNeedsPaint()
}

// AriaLabel returns the accessible name.
func (r *Row) AriaLabel() string {
	if r == nil {
		return ""
	}
	return r.ariaLabel
}

// Role has no forced role for layout containers.
func (r *Row) Role() string { return "" }

// Focusable is always false: containers never take Tab.
func (r *Row) Focusable() bool { return false }

// Add appends a column.
func (r *Row) Add(c *Col) {
	if r == nil || c == nil || c.node == nil {
		return
	}
	r.cols = append(r.cols, c)
	r.node.AddChild(c.node)
}

// SetChildren replaces all columns.
func (r *Row) SetChildren(cols ...*Col) {
	if r == nil {
		return
	}
	for _, c := range r.cols {
		if c != nil && c.node != nil {
			r.node.RemoveChild(c.node)
		}
	}
	r.cols = nil
	for _, c := range cols {
		if c != nil {
			r.cols = append(r.cols, c)
			if c.node != nil {
				r.node.AddChild(c.node)
			}
		}
	}
	r.node.MarkNeedsLayout()
}

// ClearChildren removes all columns.
func (r *Row) ClearChildren() { r.SetChildren() }

// Cols returns the live column list.
func (r *Row) Cols() []*Col {
	if r == nil {
		return nil
	}
	return append([]*Col(nil), r.cols...)
}

// Node returns the tree node.
func (r *Row) Node() rendering.RenderObject {
	if r == nil {
		return nil
	}
	return r.node
}

// Size returns the laid-out size.
func (r *Row) Size() rendering.Size {
	if r == nil || r.node == nil {
		return rendering.Size{}
	}
	return r.node.Size()
}

// Layout runs grid layout under constraints.
func (r *Row) Layout(c rendering.Constraints) rendering.Size {
	if r == nil || r.node == nil {
		return rendering.Size{}
	}
	r.layoutGrid(c)
	return r.node.Size()
}

// Col is the grid column (docs/antd/grid.md §6.10).
type Col struct {
	span     int // -1 unset, 0 hidden, 1..24 span
	offset   int
	order    int
	push     int
	pull     int
	flexSet  bool
	flexNone bool
	flexGrow float64
	flexFix  float64
	xs       int
	sm       int
	md       int
	lg       int
	xl       int
	xxl      int
	xxxl     int
	node     *rendering.AbsoluteBox
}

func newColNode() *rendering.AbsoluteBox {
	n := rendering.NewAbsoluteBox(0, 0)
	n.SetRepaintBoundary(true)
	return n
}

// NewCol creates a column with optional content children.
func NewCol(children ...rendering.RenderObject) *Col {
	c := &Col{span: -1, xs: -1, sm: -1, md: -1, lg: -1, xl: -1, xxl: -1, xxxl: -1}
	c.node = newColNode()
	for _, ch := range children {
		if ch != nil {
			c.node.AddChild(ch)
		}
	}
	return c
}

// SetSpan sets占位格数 (0 hidden, -1 clears to unset).
func (c *Col) SetSpan(n int) {
	if c == nil {
		return
	}
	if n < -1 {
		n = -1
	}
	if n > GridColumns {
		n = GridColumns
	}
	if c.span == n {
		return
	}
	c.span = n
	c.node.MarkNeedsLayout()
}

// Span returns the raw span (-1 unset).
func (c *Col) Span() int {
	if c == nil {
		return -1
	}
	return c.span
}

// SetOffset sets left gap columns.
func (c *Col) SetOffset(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if n > GridColumns {
		n = GridColumns
	}
	if c.offset == n {
		return
	}
	c.offset = n
	c.node.MarkNeedsLayout()
}

// Offset returns the offset.
func (c *Col) Offset() int {
	if c == nil {
		return 0
	}
	return c.offset
}

// SetOrder sets sort order.
func (c *Col) SetOrder(n int) {
	if c == nil || c.order == n {
		return
	}
	c.order = n
	c.node.MarkNeedsLayout()
}

// Order returns the order.
func (c *Col) Order() int {
	if c == nil {
		return 0
	}
	return c.order
}

// SetPush sets right shift columns.
func (c *Col) SetPush(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if n > GridColumns {
		n = GridColumns
	}
	if c.push == n {
		return
	}
	c.push = n
	c.node.MarkNeedsLayout()
}

// Push returns the push.
func (c *Col) Push() int {
	if c == nil {
		return 0
	}
	return c.push
}

// SetPull sets left shift columns.
func (c *Col) SetPull(n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	if n > GridColumns {
		n = GridColumns
	}
	if c.pull == n {
		return
	}
	c.pull = n
	c.node.MarkNeedsLayout()
}

// Pull returns the pull.
func (c *Col) Pull() int {
	if c == nil {
		return 0
	}
	return c.pull
}

// SetFlexNumber sets numeric flex (n n auto).
func (c *Col) SetFlexNumber(n float64) {
	if c == nil {
		return
	}
	if n < 0 {
		c.flexSet, c.flexNone, c.flexGrow, c.flexFix = false, false, 0, 0
		c.node.MarkNeedsLayout()
		return
	}
	c.flexSet, c.flexNone, c.flexGrow, c.flexFix = true, false, n, 0
	c.node.MarkNeedsLayout()
}

// SetFlexString sets string flex (auto/none/100px/1 1 200px/n).
func (c *Col) SetFlexString(s string) {
	if c == nil {
		return
	}
	s = strings.TrimSpace(s)
	if s == "" {
		c.flexSet, c.flexNone, c.flexGrow, c.flexFix = false, false, 0, 0
		c.node.MarkNeedsLayout()
		return
	}
	low := strings.ToLower(s)
	if low == "none" {
		c.flexSet, c.flexNone, c.flexGrow, c.flexFix = true, true, 0, 0
		c.node.MarkNeedsLayout()
		return
	}
	if low == "auto" {
		c.flexSet, c.flexNone, c.flexGrow, c.flexFix = true, false, 1, 0
		c.node.MarkNeedsLayout()
		return
	}
	grow, fix := parseFlexString(s)
	c.flexSet, c.flexNone, c.flexGrow, c.flexFix = true, false, grow, fix
	c.node.MarkNeedsLayout()
}

// SetFlexAuto sets flex auto (fill).
func (c *Col) SetFlexAuto() { c.SetFlexString("auto") }

// FlexRaw reports whether flex is set.
func (c *Col) HasFlex() bool { return c != nil && c.flexSet }

// SetXs sets xs breakpoint span.
func (c *Col) SetXs(n int) { c.setBP(0, n) }

// SetSm sets sm breakpoint span.
func (c *Col) SetSm(n int) { c.setBP(1, n) }

// SetMd sets md breakpoint span.
func (c *Col) SetMd(n int) { c.setBP(2, n) }

// SetLg sets lg breakpoint span.
func (c *Col) SetLg(n int) { c.setBP(3, n) }

// SetXl sets xl breakpoint span.
func (c *Col) SetXl(n int) { c.setBP(4, n) }

// SetXxl sets xxl breakpoint span.
func (c *Col) SetXxl(n int) { c.setBP(5, n) }

// SetXxxl sets xxxl breakpoint span.
func (c *Col) SetXxxl(n int) { c.setBP(6, n) }

func (c *Col) setBP(which, n int) {
	if c == nil {
		return
	}
	if n < 0 {
		n = -1
	}
	if n > GridColumns {
		n = GridColumns
	}
	switch which {
	case 0:
		c.xs = n
	case 1:
		c.sm = n
	case 2:
		c.md = n
	case 3:
		c.lg = n
	case 4:
		c.xl = n
	case 5:
		c.xxl = n
	case 6:
		c.xxxl = n
	}
	c.node.MarkNeedsLayout()
}

// SetChildren replaces content children.
func (c *Col) SetChildren(children ...rendering.RenderObject) {
	if c == nil || c.node == nil {
		return
	}
	for _, ch := range c.node.Children() {
		c.node.RemoveChild(ch)
	}
	for _, ch := range children {
		if ch != nil {
			c.node.AddChild(ch)
		}
	}
	c.node.MarkNeedsLayout()
}

// Add appends a content child.
func (c *Col) Add(ch rendering.RenderObject) {
	if c == nil || c.node == nil || ch == nil {
		return
	}
	c.node.AddChild(ch)
}

// Node returns the tree node.
func (c *Col) Node() rendering.RenderObject {
	if c == nil {
		return nil
	}
	return c.node
}

// Size returns the laid-out outer size.
func (c *Col) Size() rendering.Size {
	if c == nil || c.node == nil {
		return rendering.Size{}
	}
	return c.node.Size()
}

func parseFlexString(s string) (grow, fix float64) {
	fields := strings.Fields(s)
	if len(fields) == 1 {
		f := fields[0]
		if strings.HasSuffix(strings.ToLower(f), "px") {
			num := strings.TrimSuffix(strings.ToLower(f), "px")
			if v, err := strconv.ParseFloat(strings.TrimSpace(num), 64); err == nil && v >= 0 {
				return 0, v
			}
			return 0, 0
		}
		if v, err := strconv.ParseFloat(f, 64); err == nil && v >= 0 {
			return v, 0
		}
		return 0, 0
	}
	// Multi-token: first numeric is grow, last px token is basis.
	for _, f := range fields {
		if v, err := strconv.ParseFloat(f, 64); err == nil && v >= 0 {
			grow = v
			break
		}
	}
	for i := len(fields) - 1; i >= 0; i-- {
		f := strings.ToLower(fields[i])
		if strings.HasSuffix(f, "px") {
			num := strings.TrimSuffix(f, "px")
			if v, err := strconv.ParseFloat(strings.TrimSpace(num), 64); err == nil && v >= 0 {
				fix = v
				break
			}
		}
	}
	return grow, fix
}

func (c *Col) effectiveSpan(viewport float64) int {
	if c == nil {
		return -1
	}
	vals := [7]int{c.xs, c.sm, c.md, c.lg, c.xl, c.xxl, c.xxxl}
	thresh := [7]float64{0, ScreenSM, ScreenMD, ScreenLG, ScreenXL, ScreenXXL, ScreenXXXL}
	best := -2
	for i := 6; i >= 0; i-- {
		if viewport+1e-9 >= thresh[i] && vals[i] != -1 {
			best = vals[i]
			break
		}
	}
	if best != -2 {
		return best
	}
	return c.span
}

type colWork struct {
	col     *Col
	idx     int
	hidden  bool
	offsetW float64
	outerW  float64
	outerH  float64
	grow    float64
	fixPart float64
	isGrow  bool
	isAuto  bool
	shift   float64
}

func (r *Row) layoutGrid(c rendering.Constraints) {
	W := c.MaxWidth
	if W >= rendering.Unbounded/2 {
		W = r.viewportWidth
		if W <= 0 {
			W = 0
		}
	}
	if W < 0 {
		W = 0
	}
	V := r.viewportWidth
	if V <= 0 {
		V = W
	}
	gH := r.gutterH
	if gH < 0 {
		gH = 0
	}
	gV := r.gutterV
	if gV < 0 {
		gV = 0
	}
	maxH := c.MaxHeight
	works := make([]*colWork, 0, len(r.cols))
	for i, col := range r.cols {
		if col == nil || col.node == nil {
			continue
		}
		w := &colWork{col: col, idx: i}
		span := col.effectiveSpan(V)
		if span == 0 {
			w.hidden = true
			works = append(works, w)
			continue
		}
		w.offsetW = float64(col.offset) / float64(GridColumns) * W
		w.shift = float64(col.push-col.pull) / float64(GridColumns) * W
		if col.flexSet {
			if col.flexNone {
				w.isAuto = true
			} else if col.flexGrow > 0 {
				w.isGrow = true
				w.grow = col.flexGrow
				w.fixPart = col.flexFix
			} else {
				w.outerW = col.flexFix
			}
		} else if span > 0 {
			w.outerW = float64(span) / float64(GridColumns) * W
		} else {
			w.isAuto = true
		}
		works = append(works, w)
	}
	// Measure auto columns with loose content width.
	for _, w := range works {
		if w.hidden || !w.isAuto {
			continue
		}
		kids := w.col.node.Children()
		if len(kids) == 0 {
			w.outerW, w.outerH = 0, 0
			continue
		}
		var maxW, sumH float64
		for _, ch := range kids {
			if rendering.ManualLayoutOf(ch) {
				sz := ch.Size()
				if sz.Width > maxW {
					maxW = sz.Width
				}
				sumH += sz.Height
				continue
			}
			sz := ch.Layout(rendering.Constraints{MaxWidth: W, MaxHeight: maxH})
			if sz.Width > maxW {
				maxW = sz.Width
			}
			sumH += sz.Height
		}
		w.outerW = maxW + gH
		if w.outerW < 0 {
			w.outerW = 0
		}
		w.outerH = sumH
	}
	// Distribute remaining to grow columns.
	var fixedSum float64
	var totalGrow float64
	for _, w := range works {
		if w.hidden {
			continue
		}
		fixedSum += w.offsetW
		if w.isGrow {
			fixedSum += w.fixPart
			totalGrow += w.grow
		} else {
			fixedSum += w.outerW
		}
	}
	remain := W - fixedSum
	if remain < 0 {
		remain = 0
	}
	if totalGrow > 0 {
		for _, w := range works {
			if w.hidden || !w.isGrow {
				continue
			}
			w.outerW = w.fixPart + remain*w.grow/totalGrow
		}
	}
	// Layout fixed/grow column contents.
	for _, w := range works {
		if w.hidden || w.isAuto {
			continue
		}
		contentW := w.outerW - gH
		if contentW < 0 {
			contentW = 0
		}
		var y float64
		for _, ch := range w.col.node.Children() {
			if rendering.ManualLayoutOf(ch) {
				off := ch.Offset()
				sz := ch.Size()
				ch.SetOffset(rendering.Point{X: gH / 2, Y: y})
				_ = off
				y += sz.Height
				continue
			}
			sz := ch.Layout(rendering.Constraints{MaxWidth: contentW, MaxHeight: maxH})
			ch.SetOffset(rendering.Point{X: gH / 2, Y: y})
			y += sz.Height
		}
		w.outerH = y
	}
	// Fix auto content offsets to gutter padding.
	for _, w := range works {
		if w.hidden || !w.isAuto {
			continue
		}
		var y float64
		for _, ch := range w.col.node.Children() {
			if rendering.ManualLayoutOf(ch) {
				continue
			}
			off := ch.Offset()
			_ = off
			ch.SetOffset(rendering.Point{X: gH / 2, Y: y})
			y += ch.Size().Height
		}
		w.outerH = y
	}
	// Stable order sort for visible columns.
	visible := make([]*colWork, 0, len(works))
	for _, w := range works {
		if !w.hidden {
			visible = append(visible, w)
		}
	}
	sort.SliceStable(visible, func(i, j int) bool {
		return visible[i].col.order < visible[j].col.order
	})
	// Wrap into lines.
	type line struct {
		cols []*colWork
		adv  float64
		maxH float64
	}
	var lines []line
	if len(visible) > 0 {
		cur := line{}
		for _, w := range visible {
			adv := w.offsetW + w.outerW
			if r.wrap && len(cur.cols) > 0 && cur.adv+adv > W+1e-9 {
				for _, cw := range cur.cols {
					if cw.outerH > cur.maxH {
						cur.maxH = cw.outerH
					}
				}
				lines = append(lines, cur)
				cur = line{}
			}
			cur.cols = append(cur.cols, w)
			cur.adv += adv
		}
		for _, cw := range cur.cols {
			if cw.outerH > cur.maxH {
				cur.maxH = cw.outerH
			}
		}
		lines = append(lines, cur)
	}
	// Position lines with justify/align.
	var totalH float64
	for li := range lines {
		if li > 0 {
			totalH += gV
		}
		totalH += lines[li].maxH
	}
	yBase := 0.0
	for _, ln := range lines {
		n := len(ln.cols)
		free := W - ln.adv
		var start, gap float64
		switch r.justify {
		case RowJustifyEnd:
			start = free
		case RowJustifyCenter:
			start = free / 2
		case RowJustifySpaceBetween:
			if n > 1 {
				gap = free / float64(n-1)
			}
		case RowJustifySpaceAround:
			if n > 0 {
				gap = free / float64(n)
				start = gap / 2
			}
		case RowJustifySpaceEvenly:
			if n > 0 {
				gap = free / float64(n+1)
				start = gap
			}
		default:
			start = 0
		}
		flow := start
		for j, w := range ln.cols {
			flow += w.offsetW
			vx := flow + w.shift
			var vy float64
			finalH := w.outerH
			switch r.align {
			case RowAlignMiddle:
				vy = yBase + (ln.maxH-w.outerH)/2
			case RowAlignBottom:
				vy = yBase + (ln.maxH - w.outerH)
			case RowAlignStretch:
				vy = yBase
				finalH = ln.maxH
			default:
				vy = yBase
			}
			w.col.node.FixedWidth = w.outerW
			w.col.node.FixedHeight = finalH
			w.col.node.SetOffset(rendering.Point{X: vx, Y: vy})
			w.outerH = finalH
			flow += w.outerW
			if j < n-1 {
				flow += gap
			}
		}
		yBase += ln.maxH + gV
	}
	// Hidden columns collapse.
	for _, w := range works {
		if !w.hidden {
			continue
		}
		w.col.node.FixedWidth = 0
		w.col.node.FixedHeight = 0
		w.col.node.SetOffset(rendering.Point{})
		w.col.node.Layout(rendering.Tight(0, 0))
	}
	// Finalize visible column nodes then the row node.
	for _, w := range works {
		if w.hidden {
			continue
		}
		w.col.node.Layout(rendering.Tight(w.outerW, w.outerH))
	}
	r.node.FixedWidth = W
	r.node.FixedHeight = totalH
	r.node.Layout(rendering.Tight(W, totalH))
}
