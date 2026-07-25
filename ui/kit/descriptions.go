package kit

import (
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Descriptions component tokens — prepareComponentToken / genDescriptionStyles.
// docs/antd/descriptions.md §6.2 · components/descriptions/style/index.ts
//
// antd seed paddingSM=12 / paddingXS=8; kit TokenPaddingSM=8 maps to antd paddingXS.
// Medium/small paddings below match antd Descriptions, not raw kit seed SM.
const (
	// DefaultDescriptionsFontSize is fontSize (14).
	DefaultDescriptionsFontSize = 14.0
	// DefaultDescriptionsTitleFontSize is fontSizeLG (16).
	DefaultDescriptionsTitleFontSize = 16.0
	// DefaultDescriptionsTitleMarginBottom ≈ fontSizeSM * lineHeightSM (12*1.666≈20).
	DefaultDescriptionsTitleMarginBottom = 20.0
	// DefaultDescriptionsItemPadBottom is non-bordered item padding-bottom (large = padding).
	DefaultDescriptionsItemPadBottom = 16.0
	// DefaultDescriptionsItemPadBottomMD is medium (antd paddingSM=12).
	DefaultDescriptionsItemPadBottomMD = 12.0
	// DefaultDescriptionsItemPadBottomSM is small (antd paddingXS=8).
	DefaultDescriptionsItemPadBottomSM = 8.0
	// DefaultDescriptionsItemPadEnd is non-bordered item padding-inline-end (padding).
	DefaultDescriptionsItemPadEnd = 16.0
	// DefaultDescriptionsColonMarginLeft is colonMarginLeft (marginXXS/2 ≈ 2).
	DefaultDescriptionsColonMarginLeft = 2.0
	// DefaultDescriptionsColonMarginRight is colonMarginRight (marginXS=4).
	DefaultDescriptionsColonMarginRight = 4.0
	// DefaultDescriptionsBorderedPadV is bordered cell pad-block large (padding=16).
	DefaultDescriptionsBorderedPadV = 16.0
	// DefaultDescriptionsBorderedPadH is bordered cell pad-inline large (paddingLG=24).
	DefaultDescriptionsBorderedPadH = 24.0
	// DefaultDescriptionsBorderedPadVMD is bordered pad-block medium (12).
	DefaultDescriptionsBorderedPadVMD = 12.0
	// DefaultDescriptionsBorderedPadHMD is bordered pad-inline medium (24).
	DefaultDescriptionsBorderedPadHMD = 24.0
	// DefaultDescriptionsBorderedPadVSM is bordered pad-block small (8).
	DefaultDescriptionsBorderedPadVSM = 8.0
	// DefaultDescriptionsBorderedPadHSM is bordered pad-inline small (16).
	DefaultDescriptionsBorderedPadHSM = 16.0
	// DefaultDescriptionsRadius is borderRadiusLG (view).
	DefaultDescriptionsRadius = 8.0
	// DefaultDescriptionsLineWidth is lineWidth.
	DefaultDescriptionsLineWidth = 1.0
	// DefaultDescriptionsColumn is antd default column count (md+).
	DefaultDescriptionsColumn = 3
	// DefaultDescriptionsFocusRingOutset approximates Ant focus-visible outset.
	DefaultDescriptionsFocusRingOutset = 1.5
)

// DescriptionsSize is antd size: large | medium | small.
// Default is large (antd Descriptions default; not middle).
type DescriptionsSize int

const (
	// DescriptionsLarge is the antd default size.
	DescriptionsLarge DescriptionsSize = iota
	// DescriptionsMiddle is antd medium.
	DescriptionsMiddle
	// DescriptionsSmall is compact.
	DescriptionsSmall
)

// DescriptionsLayout is antd layout: horizontal | vertical.
type DescriptionsLayout int

const (
	// DescriptionsHorizontal places label and content on one line (default).
	DescriptionsHorizontal DescriptionsLayout = iota
	// DescriptionsVertical stacks label above content.
	DescriptionsVertical
)

// DescriptionsItem is one Descriptions.Item (antd items[] entry).
//
// Span rules (antd useItems + useRow):
//   - Span ≤ 0 → 1
//   - SpanFilled → fill remaining columns of the current row and close the row
//   - SpanMap → responsive span via ViewportWidth (keys: xs|sm|md|lg|xl|xxl|xxxl)
//
// Content: ChildrenNode overrides Children string when non-nil.
// Label: LabelNode overrides Label string when non-nil.
type DescriptionsItem struct {
	Key          string
	Label        string
	LabelNode    core.Node
	Children     string
	ChildrenNode core.Node
	// Span is the column span (0 → 1). Ignored when SpanFilled.
	Span int
	// SpanFilled is antd span="filled" (consume rest of row).
	SpanFilled bool
	// SpanMap is responsive span {xs:…, sm:…}. Empty keys ignored.
	SpanMap map[string]int
}

// Descriptions is Ant Design Descriptions (只读字段组合).
//
//	Decorated root
//	  └─ Column
//	       header? (title · extra)
//	       view → rows of Flexible(span) cells
//
// Product contract: docs/antd/descriptions.md §6 (P0 DoD).
// Root pointer stays stable across rebuild when possible (ClearChildren).
type Descriptions struct {
	Root *primitive.Decorated
	col  *primitive.Flex
	view *primitive.Decorated

	titleLab  *primitive.Text
	extraSlot *primitive.Slot

	Items []DescriptionsItem

	// Title is the header title string (used when TitleNode is nil).
	Title     string
	TitleNode core.Node
	Extra     core.Node

	Size     DescriptionsSize
	Bordered bool
	Layout   DescriptionsLayout
	// Colon shows ":" after labels when non-bordered (antd default true).
	Colon bool

	// Column is items per row when > 0. 0 → resolve via ColumnMap / DEFAULT_COLUMN_MAP.
	Column int
	// ColumnMap is responsive column counts (keys: xs|sm|md|lg|xl|xxl|xxxl).
	ColumnMap map[string]int
	// ViewportWidth drives responsive column/span. 0 → treat as md (768).
	ViewportWidth float64
	// columnSet: true after SetColumn (including 0 to force map resolution).
	columnSet bool

	// LabelStyle / ContentStyle: shallow semantic overrides (style-class P0).
	LabelStyle   Style
	ContentStyle Style

	AriaLabel string
	Face      text.Face
	Theme     *core.Theme
	Style     Style

	// packed is the last computed row layout (resolved spans).
	packed [][]descCell
}

// descCell is one laid-out item with resolved span.
type descCell struct {
	item DescriptionsItem
	span int
}

// NewDescriptions creates a Descriptions with antd defaults
// (size=large, column=3, colon=true, layout=horizontal, bordered=false).
func NewDescriptions(items ...DescriptionsItem) *Descriptions {
	d := &Descriptions{
		Items: append([]DescriptionsItem(nil), items...),
		Size:  DescriptionsLarge,
		Colon: true,
	}
	d.rebuild()
	return d
}

// Node returns the mount root (stable Decorated).
func (d *Descriptions) Node() core.Node {
	if d == nil {
		return nil
	}
	if d.Root == nil {
		d.rebuild()
	}
	return d.Root
}

// ChromeNode returns the visual shell (Decorated).
func (d *Descriptions) ChromeNode() core.Node {
	return d.Node()
}

// --- metrics (for tests / §6.2) ---

// ItemPadBottom returns non-bordered item padding-bottom for current size.
func (d *Descriptions) ItemPadBottom() float64 {
	if d == nil {
		return DefaultDescriptionsItemPadBottom
	}
	switch d.Size {
	case DescriptionsMiddle:
		return DefaultDescriptionsItemPadBottomMD
	case DescriptionsSmall:
		return DefaultDescriptionsItemPadBottomSM
	default:
		return DefaultDescriptionsItemPadBottom
	}
}

// BorderedPadV returns bordered cell vertical padding for current size.
func (d *Descriptions) BorderedPadV() float64 {
	if d == nil {
		return DefaultDescriptionsBorderedPadV
	}
	switch d.Size {
	case DescriptionsMiddle:
		return DefaultDescriptionsBorderedPadVMD
	case DescriptionsSmall:
		return DefaultDescriptionsBorderedPadVSM
	default:
		return DefaultDescriptionsBorderedPadV
	}
}

// BorderedPadH returns bordered cell horizontal padding for current size.
func (d *Descriptions) BorderedPadH() float64 {
	if d == nil {
		return DefaultDescriptionsBorderedPadH
	}
	switch d.Size {
	case DescriptionsMiddle:
		return DefaultDescriptionsBorderedPadHMD
	case DescriptionsSmall:
		return DefaultDescriptionsBorderedPadHSM
	default:
		return DefaultDescriptionsBorderedPadH
	}
}

// TitleFontSize returns title font size.
func (d *Descriptions) TitleFontSize() float64 {
	if d == nil {
		return DefaultDescriptionsTitleFontSize
	}
	return d.titleFontResolved()
}

// FontSize returns body/label font size.
func (d *Descriptions) FontSize() float64 {
	if d == nil {
		return DefaultDescriptionsFontSize
	}
	return d.fontResolved()
}

// Radius returns view corner radius.
func (d *Descriptions) Radius() float64 {
	if d == nil {
		return DefaultDescriptionsRadius
	}
	return d.radiusResolved()
}

// LineWidth returns border line width.
func (d *Descriptions) LineWidth() float64 {
	if d == nil {
		return DefaultDescriptionsLineWidth
	}
	return d.lineWResolved()
}

// ResolvedColumn returns the effective column count (after map / default).
func (d *Descriptions) ResolvedColumn() int {
	if d == nil {
		return DefaultDescriptionsColumn
	}
	return d.resolveColumn()
}

// RowCount returns the number of packed rows after last rebuild.
func (d *Descriptions) RowCount() int {
	if d == nil {
		return 0
	}
	return len(d.packed)
}

// RowSpans returns resolved spans per row (copy).
func (d *Descriptions) RowSpans() [][]int {
	if d == nil || len(d.packed) == 0 {
		return nil
	}
	out := make([][]int, len(d.packed))
	for i, row := range d.packed {
		out[i] = make([]int, len(row))
		for j, c := range row {
			out[i][j] = c.span
		}
	}
	return out
}

// IsBordered reports bordered chrome.
func (d *Descriptions) IsBordered() bool { return d != nil && d.Bordered }

// IsVertical reports layout=vertical.
func (d *Descriptions) IsVertical() bool {
	return d != nil && d.Layout == DescriptionsVertical
}

// --- setters ---

// SetItems replaces items and rebuilds.
func (d *Descriptions) SetItems(items ...DescriptionsItem) {
	if d == nil {
		return
	}
	d.Items = append([]DescriptionsItem(nil), items...)
	d.rebuild()
}

// SetTitle sets the string title (clears TitleNode).
func (d *Descriptions) SetTitle(s string) {
	if d == nil {
		return
	}
	d.Title = s
	d.TitleNode = nil
	d.rebuild()
}

// SetTitleNode sets a custom title node (overrides Title string).
func (d *Descriptions) SetTitleNode(n core.Node) {
	if d == nil {
		return
	}
	d.TitleNode = n
	d.rebuild()
}

// SetExtra sets the header extra area.
func (d *Descriptions) SetExtra(n core.Node) {
	if d == nil {
		return
	}
	d.Extra = n
	if d.extraSlot != nil {
		d.extraSlot.SetChild(n)
		return
	}
	d.rebuild()
}

// SetSize sets large | middle | small.
func (d *Descriptions) SetSize(sz DescriptionsSize) {
	if d == nil {
		return
	}
	d.Size = sz
	d.rebuild()
}

// SetBordered toggles table border chrome.
func (d *Descriptions) SetBordered(v bool) {
	if d == nil {
		return
	}
	d.Bordered = v
	d.rebuild()
}

// SetLayout sets horizontal | vertical.
func (d *Descriptions) SetLayout(l DescriptionsLayout) {
	if d == nil {
		return
	}
	d.Layout = l
	d.rebuild()
}

// SetColon toggles label colon (non-bordered only).
func (d *Descriptions) SetColon(v bool) {
	if d == nil {
		return
	}
	d.Colon = v
	d.rebuild()
}

// SetColumn sets a fixed column count. Pass 0 to fall back to ColumnMap / default map.
func (d *Descriptions) SetColumn(n int) {
	if d == nil {
		return
	}
	d.Column = n
	d.columnSet = true
	d.rebuild()
}

// SetColumnMap sets responsive column counts (antd column={{ xs:1, sm:2, … }}).
// When Column is 0 (or unset as fixed), map is matched against ViewportWidth.
func (d *Descriptions) SetColumnMap(m map[string]int) {
	if d == nil {
		return
	}
	if m == nil {
		d.ColumnMap = nil
	} else {
		d.ColumnMap = make(map[string]int, len(m))
		for k, v := range m {
			d.ColumnMap[strings.ToLower(strings.TrimSpace(k))] = v
		}
	}
	d.rebuild()
}

// SetViewportWidth sets the width used for responsive column/span resolution.
func (d *Descriptions) SetViewportWidth(w float64) {
	if d == nil {
		return
	}
	d.ViewportWidth = w
	d.rebuild()
}

// SetLabelStyle sets shallow label style overrides (Text color, …).
func (d *Descriptions) SetLabelStyle(st Style) {
	if d == nil {
		return
	}
	d.LabelStyle = st
	d.rebuild()
}

// SetContentStyle sets shallow content style overrides.
func (d *Descriptions) SetContentStyle(st Style) {
	if d == nil {
		return
	}
	d.ContentStyle = st
	d.rebuild()
}

// SetStyle sets optional root visual overrides.
func (d *Descriptions) SetStyle(st Style) {
	if d == nil {
		return
	}
	d.Style = st
	d.applyChrome()
}

// SetTheme sets theme override.
func (d *Descriptions) SetTheme(th *core.Theme) {
	if d == nil {
		return
	}
	d.Theme = th
	d.rebuild()
}

// SetFace sets font face for text chrome.
func (d *Descriptions) SetFace(face text.Face) {
	if d == nil {
		return
	}
	d.Face = face
	d.rebuild()
}

// SetAriaLabel sets the accessible name on the root group.
func (d *Descriptions) SetAriaLabel(s string) {
	if d == nil {
		return
	}
	d.AriaLabel = s
	d.applyA11y()
}

// --- resolve ---

func (d *Descriptions) theme() *core.Theme {
	if d != nil && d.Theme != nil {
		return d.Theme
	}
	return DefaultTheme()
}

func (d *Descriptions) fontResolved() float64 {
	th := d.theme()
	return th.SizeOr(core.TokenFontSize, DefaultDescriptionsFontSize)
}

func (d *Descriptions) titleFontResolved() float64 {
	th := d.theme()
	return th.SizeOr(core.TokenFontSizeLG, DefaultDescriptionsTitleFontSize)
}

func (d *Descriptions) radiusResolved() float64 {
	if d.Style.hasRadius() {
		return d.Style.Radius
	}
	th := d.theme()
	return th.SizeOr(core.TokenBorderRadiusLG, DefaultDescriptionsRadius)
}

func (d *Descriptions) lineWResolved() float64 {
	th := d.theme()
	return th.SizeOr(core.TokenLineWidth, DefaultDescriptionsLineWidth)
}

func (d *Descriptions) titleMarginBottomResolved() float64 {
	// antd: fontSizeSM * lineHeightSM ≈ 20; fall back constant.
	th := d.theme()
	fsm := th.SizeOr(core.TokenFontSizeSM, 12)
	if fsm > 0 {
		// lineHeightSM ≈ 1.6667
		return fsm * 1.6667
	}
	return DefaultDescriptionsTitleMarginBottom
}

// resolveColumn mirrors antd:
//
//	if number column → use it
//	else matchScreen(columnMap) ?? matchScreen(DEFAULT_COLUMN_MAP) ?? 3
func (d *Descriptions) resolveColumn() int {
	if d.Column > 0 {
		return d.Column
	}
	// Explicit SetColumn(0) or never set with ColumnMap → map path.
	if n := matchDescScreen(d.viewport(), d.ColumnMap); n > 0 {
		return n
	}
	if n := matchDescScreen(d.viewport(), defaultDescriptionsColumnMap()); n > 0 {
		return n
	}
	return DefaultDescriptionsColumn
}

func (d *Descriptions) viewport() float64 {
	if d != nil && d.ViewportWidth > 0 {
		return d.ViewportWidth
	}
	// Unspecified → md floor (768) so default map yields 3 (antd desktop default).
	return ScreenMD
}

func defaultDescriptionsColumnMap() map[string]int {
	// components/descriptions/constant.ts
	return map[string]int{
		"xxxl": 4,
		"xxl":  3,
		"xl":   3,
		"lg":   3,
		"md":   3,
		"sm":   2,
		"xs":   1,
	}
}

// matchDescScreen picks the largest breakpoint ≤ viewport that is set in m.
// Breakpoint mins mirror kit Screen* (antd screen*Min).
func matchDescScreen(viewport float64, m map[string]int) int {
	if len(m) == 0 {
		return 0
	}
	// antd matchScreen: highest active breakpoint present in m wins.
	// Active means viewport ≥ min (xs always active at 0).
	var best int
	found := false
	asc := []struct {
		key string
		min float64
	}{
		{"xs", ScreenXS},
		{"sm", ScreenSM},
		{"md", ScreenMD},
		{"lg", ScreenLG},
		{"xl", ScreenXL},
		{"xxl", ScreenXXL},
		{"xxxl", ScreenXXXL},
	}
	for _, b := range asc {
		if viewport < b.min && b.min > 0 {
			continue
		}
		if v, ok := m[b.key]; ok && v > 0 {
			best = v
			found = true
		}
	}
	if !found {
		return 0
	}
	return best
}

func (d *Descriptions) resolveItemSpan(it DescriptionsItem, col int) (span int, filled bool) {
	if it.SpanFilled {
		return 0, true
	}
	if len(it.SpanMap) > 0 {
		if n := matchDescScreen(d.viewport(), it.SpanMap); n > 0 {
			return n, false
		}
	}
	if it.Span > 0 {
		return it.Span, false
	}
	return 1, false
}

// packRows ports antd getCalcRows (hooks/useRow.ts).
func (d *Descriptions) packRows() [][]descCell {
	col := d.resolveColumn()
	if col < 1 {
		col = 1
	}
	var rows [][]descCell
	var tmp []descCell
	count := 0

	for _, it := range d.Items {
		span, filled := d.resolveItemSpan(it, col)
		if filled {
			// fill remaining of current row
			rest := col - count
			if rest < 1 {
				rest = col
			}
			tmp = append(tmp, descCell{item: it, span: rest})
			rows = append(rows, tmp)
			tmp = nil
			count = 0
			continue
		}
		if span < 1 {
			span = 1
		}
		restSpan := col - count
		count += span
		if count >= col {
			if count > col {
				// clamp to remaining
				tmp = append(tmp, descCell{item: it, span: restSpan})
			} else {
				tmp = append(tmp, descCell{item: it, span: span})
			}
			rows = append(rows, tmp)
			tmp = nil
			count = 0
		} else {
			tmp = append(tmp, descCell{item: it, span: span})
		}
	}
	if len(tmp) > 0 {
		rows = append(rows, tmp)
	}

	// Expand last item span so each row fills `col` columns (antd behavior).
	for i := range rows {
		sum := 0
		for _, c := range rows[i] {
			sum += c.span
		}
		if sum < col && len(rows[i]) > 0 {
			last := len(rows[i]) - 1
			rows[i][last].span = col - (sum - rows[i][last].span)
		}
	}
	return rows
}

// --- rebuild ---

func (d *Descriptions) rebuild() {
	if d == nil {
		return
	}
	th := d.theme()
	d.packed = d.packRows()

	// Root shell
	if d.Root == nil {
		d.Root = primitive.NewDecorated(nil)
		d.Root.ExpandWidth = true
		d.Root.Hit = core.HitDefer
	}
	d.Root.ClearChildren()
	d.col = primitive.Column()
	d.col.CrossAlign = core.CrossStretch
	d.col.MainAlign = core.MainStart
	d.col.ExpandMax = true
	d.Root.AddChild(d.col)

	// Header
	hasTitle := d.TitleNode != nil || d.Title != ""
	hasExtra := d.Extra != nil
	if hasTitle || hasExtra {
		header := primitive.Row()
		header.CrossAlign = core.CrossCenter
		header.ExpandMax = true
		header.MainAlign = core.MainStart
		// marginBottom = titleMarginBottom
		header.Padding = primitive.EdgeInsets{Bottom: d.titleMarginBottomResolved()}

		if hasTitle {
			var titleNode core.Node
			if d.TitleNode != nil {
				titleNode = d.TitleNode
			} else {
				lab := primitive.NewText(d.Title)
				lab.FontSize = d.titleFontResolved()
				lab.Face = d.Face
				lab.Color = th.Color(core.TokenColorText)
				// antd title uses fontWeightStrong; kit Text has no weight field — size LG is enough for L2.
				d.titleLab = lab
				titleNode = lab
			}
			header.AddChild(primitive.NewFlexible(1, titleNode))
		} else {
			header.AddChild(primitive.Spacer())
		}
		if hasExtra {
			d.extraSlot = primitive.NewSlot("desc-extra", d.Extra)
			header.AddChild(d.extraSlot)
		} else {
			d.extraSlot = nil
		}
		d.col.AddChild(header)
	} else {
		d.titleLab = nil
		d.extraSlot = nil
	}

	// View
	d.view = primitive.NewDecorated(nil)
	d.view.ExpandWidth = true
	d.view.Hit = core.HitDefer
	d.view.Radius = d.radiusResolved()
	if d.Bordered {
		lw := d.lineWResolved()
		d.view.BorderWidth = lw
		d.view.BorderColor = th.Color(core.TokenColorSplit)
		d.view.Background = th.Color(core.TokenColorBgContainer)
	} else {
		d.view.BorderWidth = 0
	}

	body := primitive.Column()
	body.CrossAlign = core.CrossStretch
	body.ExpandMax = true
	d.view.AddChild(body)
	d.view.StretchChild = true

	for ri, row := range d.packed {
		lastRow := ri == len(d.packed)-1
		body.AddChild(d.buildRow(row, lastRow, th))
	}

	d.col.AddChild(d.view)
	d.applyChrome()
	d.applyA11y()
}

func (d *Descriptions) buildRow(cells []descCell, lastRow bool, th *core.Theme) core.Node {
	row := primitive.Row()
	row.CrossAlign = core.CrossStretch
	row.ExpandMax = true
	row.MainAlign = core.MainStart

	for i, c := range cells {
		lastCell := i == len(cells)-1
		cell := d.buildCell(c, lastRow, lastCell, th)
		grow := float64(c.span)
		if grow < 1 {
			grow = 1
		}
		wrap := primitive.NewFlexible(grow, cell)
		wrap.FillChild = true
		row.AddChild(wrap)
	}
	return row
}

func (d *Descriptions) buildCell(c descCell, lastRow, lastCell bool, th *core.Theme) core.Node {
	if d.Bordered {
		return d.buildBorderedCell(c, lastRow, lastCell, th)
	}
	return d.buildPlainCell(c, lastRow, lastCell, th)
}

func (d *Descriptions) buildPlainCell(c descCell, lastRow, lastCell bool, th *core.Theme) core.Node {
	padBottom := d.ItemPadBottom()
	if lastRow {
		padBottom = 0
	}
	padEnd := DefaultDescriptionsItemPadEnd
	if lastCell {
		padEnd = 0
	}

	labelN := d.makeLabelNode(c.item, th, false)
	contentN := d.makeContentNode(c.item, th)

	var inner core.Node
	if d.Layout == DescriptionsVertical {
		col := primitive.Column(labelN, contentN)
		col.Gap = 4
		col.CrossAlign = core.CrossStart
		inner = col
	} else {
		r := primitive.Row(labelN, contentN)
		r.Gap = 0
		r.CrossAlign = core.CrossStart
		inner = r
	}

	box := primitive.NewDecorated(inner)
	box.Padding = primitive.EdgeInsets{Bottom: padBottom, Right: padEnd}
	box.Hit = core.HitDefer
	box.ExpandWidth = true
	return box
}

func (d *Descriptions) buildBorderedCell(c descCell, lastRow, lastCell bool, th *core.Theme) core.Node {
	padV := d.BorderedPadV()
	padH := d.BorderedPadH()
	lw := d.lineWResolved()
	split := th.Color(core.TokenColorSplit)
	labelBg := th.Color(core.TokenColorFillSecondary)

	labelN := d.makeLabelNode(c.item, th, true)
	contentN := d.makeContentNode(c.item, th)

	var body core.Node
	if d.Layout == DescriptionsVertical {
		labBox := d.padBox(labelN, padV, padH, labelBg)
		conBox := d.padBox(contentN, padV, padH, render.RGBA{})
		hsep := d.hLine(lw, split)
		col := primitive.Column(labBox, hsep, conBox)
		col.CrossAlign = core.CrossStretch
		col.ExpandMax = true
		body = col
	} else {
		labBox := d.padBox(labelN, padV, padH, labelBg)
		conBox := d.padBox(contentN, padV, padH, render.RGBA{})
		vsep := d.vLine(lw, split)
		row := primitive.Row()
		row.CrossAlign = core.CrossStretch
		row.ExpandMax = true
		fl := primitive.NewFlexible(1, labBox)
		fl.FillChild = true
		fc := primitive.NewFlexible(2, conBox)
		fc.FillChild = true
		row.AddChild(fl)
		row.AddChild(vsep)
		row.AddChild(fc)
		body = row
	}

	// Attach right / bottom grid lines.
	if !lastCell {
		vsep := d.vLine(lw, split)
		row := primitive.Row()
		row.CrossAlign = core.CrossStretch
		row.ExpandMax = true
		fb := primitive.NewFlexible(1, body)
		fb.FillChild = true
		row.AddChild(fb)
		row.AddChild(vsep)
		body = row
	}
	if !lastRow {
		hsep := d.hLine(lw, split)
		col := primitive.Column(body, hsep)
		col.CrossAlign = core.CrossStretch
		col.ExpandMax = true
		body = col
	}
	return body
}

func (d *Descriptions) padBox(child core.Node, padV, padH float64, bg render.RGBA) *primitive.Decorated {
	box := primitive.NewDecorated(child)
	box.Padding = primitive.EdgeInsets{Top: padV, Bottom: padV, Left: padH, Right: padH}
	box.Background = bg
	box.Hit = core.HitDefer
	box.ExpandWidth = true
	box.StretchChild = false
	return box
}

func (d *Descriptions) hLine(lw float64, c render.RGBA) *primitive.Decorated {
	sep := primitive.NewDecorated(nil)
	sep.Height = lw
	sep.MinHeight = lw
	sep.ExpandWidth = true
	sep.Background = c
	sep.Hit = core.HitDefer
	return sep
}

func (d *Descriptions) vLine(lw float64, c render.RGBA) *primitive.Decorated {
	sep := primitive.NewDecorated(nil)
	sep.Width = lw
	sep.MinWidth = lw
	sep.Background = c
	sep.Hit = core.HitDefer
	return sep
}

func (d *Descriptions) makeLabelNode(it DescriptionsItem, th *core.Theme, bordered bool) core.Node {
	if it.LabelNode != nil {
		return it.LabelNode
	}
	color := th.Color(core.TokenColorTextTertiary)
	if d.LabelStyle.hasText() {
		color = d.LabelStyle.Text
	}
	fs := d.fontResolved()
	if d.LabelStyle.FontSize > 0 {
		fs = d.LabelStyle.FontSize
	}
	face := d.Face
	if d.LabelStyle.Face != nil {
		face = d.LabelStyle.Face
	}

	labelText := it.Label
	// Colon: non-bordered only when Colon=true and label non-empty (antd).
	showColon := d.Colon && !bordered && labelText != ""
	if showColon {
		// "Label" + spacing + ":" + spacing — approximate with single string "Label:"
		// and rely on text; visual gap is tight but acceptable for L1/L2.
		// Prefer explicit ":" child for spacing fidelity.
		lab := primitive.NewText(labelText)
		lab.FontSize = fs
		lab.Face = face
		lab.Color = color
		colon := primitive.NewText(":")
		colon.FontSize = fs
		colon.Face = face
		colon.Color = color
		// Margins via padded host around colon
		colonHost := primitive.NewDecorated(colon)
		colonHost.Padding = primitive.EdgeInsets{
			Left:  DefaultDescriptionsColonMarginLeft,
			Right: DefaultDescriptionsColonMarginRight,
		}
		colonHost.Hit = core.HitDefer
		row := primitive.Row(lab, colonHost)
		row.CrossAlign = core.CrossStart
		return row
	}
	lab := primitive.NewText(labelText)
	lab.FontSize = fs
	lab.Face = face
	lab.Color = color
	return lab
}

func (d *Descriptions) makeContentNode(it DescriptionsItem, th *core.Theme) core.Node {
	if it.ChildrenNode != nil {
		return it.ChildrenNode
	}
	color := th.Color(core.TokenColorText)
	if d.ContentStyle.hasText() {
		color = d.ContentStyle.Text
	}
	fs := d.fontResolved()
	if d.ContentStyle.FontSize > 0 {
		fs = d.ContentStyle.FontSize
	}
	face := d.Face
	if d.ContentStyle.Face != nil {
		face = d.ContentStyle.Face
	}
	// Multi-line: split on \n into a column of texts (border demo Config Info).
	if strings.Contains(it.Children, "\n") {
		parts := strings.Split(it.Children, "\n")
		col := primitive.Column()
		col.CrossAlign = core.CrossStart
		col.Gap = 2
		for _, p := range parts {
			t := primitive.NewText(p)
			t.FontSize = fs
			t.Face = face
			t.Color = color
			col.AddChild(t)
		}
		return col
	}
	t := primitive.NewText(it.Children)
	t.FontSize = fs
	t.Face = face
	t.Color = color
	return t
}

func (d *Descriptions) applyChrome() {
	if d == nil || d.Root == nil {
		return
	}
	th := d.theme()
	bg := th.Color(core.TokenColorBgContainer)
	if d.Style.hasBG() {
		bg = d.Style.Background
	}
	// Root is usually transparent; view carries bordered chrome.
	// Style.Background paints root when set (style-class large demo).
	d.Root.Background = bg
	if d.Style.hasBG() {
		// keep
	} else {
		d.Root.Background = render.RGBA{} // transparent root
	}
	if d.Style.hasBorder() {
		d.Root.BorderWidth = d.lineWResolved()
		d.Root.BorderColor = d.Style.Border
	} else {
		d.Root.BorderWidth = 0
	}
	if d.Style.hasRadius() {
		d.Root.Radius = d.Style.Radius
	} else {
		d.Root.Radius = 0
	}
	if d.view != nil {
		d.view.Radius = d.radiusResolved()
		if d.Bordered {
			d.view.BorderWidth = d.lineWResolved()
			d.view.BorderColor = th.Color(core.TokenColorSplit)
			d.view.Background = th.Color(core.TokenColorBgContainer)
		}
	}
}

func (d *Descriptions) applyA11y() {
	if d == nil || d.Root == nil {
		return
	}
	d.Root.Base().Role = "group"
	name := d.AriaLabel
	if name == "" {
		name = d.Title
	}
	d.Root.Base().Label = name
}
