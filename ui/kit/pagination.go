package kit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Ant Design Pagination defaults — docs/antd/pagination.md §6.2 / §6.10
// https://ant.design/components/pagination
const (
	DefaultPaginationCurrent                      = 1
	DefaultPaginationPageSize                     = 10
	DefaultPaginationTotalBoundaryShowSizeChanger = 50
	DefaultPaginationItemGap                      = 8.0
)

// PaginationSize is the control size ladder (antd size).
type PaginationSize int

const (
	// PaginationMiddle is the default size (item 32).
	PaginationMiddle PaginationSize = iota
	// PaginationSmall is compact (item 24).
	PaginationSmall
	// PaginationLarge is large (item 40).
	PaginationLarge
)

// PaginationAlign is root main-axis alignment (antd align).
type PaginationAlign int

const (
	// PaginationAlignStart packs items to the start (default).
	PaginationAlignStart PaginationAlign = iota
	// PaginationAlignCenter centers the pager.
	PaginationAlignCenter
	// PaginationAlignEnd packs items to the end.
	PaginationAlignEnd
)

// PaginationItemKind identifies a hit target for tests / itemRender hooks.
type PaginationItemKind string

const (
	PaginationItemPage     PaginationItemKind = "page"
	PaginationItemPrev     PaginationItemKind = "prev"
	PaginationItemNext     PaginationItemKind = "next"
	PaginationItemJumpPrev PaginationItemKind = "jump-prev"
	PaginationItemJumpNext PaginationItemKind = "jump-next"
)

// Pagination is Ant Design Pagination (page navigator).
//
//	Flex root (role=navigation)
//	  [showTotal] prev | pages/ellipsis | next [sizeChanger] [quickJumper]
//
// Product contract: docs/antd/pagination.md §6 (P0 DoD).
// Root identity is stable across page / size changes (ClearChildren + rebuild).
type Pagination struct {
	Root *primitive.Flex

	// Total is data item count (antd total), not page count.
	Total int
	// Current is the active page (1-based).
	Current int
	// PageSize is items per page.
	PageSize int

	Disabled                     bool
	Size                         PaginationSize
	Align                        PaginationAlign
	Simple                       bool
	SimpleReadOnly               bool
	ShowQuickJumper              bool
	HideOnSinglePage             bool
	ShowLessItems                bool
	PageSizeOptions              []int
	TotalBoundaryShowSizeChanger int

	// ShowTotal formats the leading total label. Nil → hidden.
	// range is inclusive 1-based [start, end] of current page items (0,0 when empty).
	ShowTotal func(total, start, end int) string

	Face      text.Face
	Theme     *core.Theme
	AriaLabel string

	// OnChange fires after page or pageSize changes (antd onChange(page, pageSize)).
	OnChange func(page, pageSize int)
	// OnShowSizeChange fires when pageSize changes (before/with OnChange).
	OnShowSizeChange func(current, size int)

	// Internal controlled flags (SetCurrent / SetPageSize mark controlled).
	currentControlled  bool
	pageSizeControlled bool
	// showSizeChanger: tri-state — unset → auto by totalBoundary; set → fixed.
	showSizeChanger    bool
	showSizeChangerSet bool

	// default seeds for uncontrolled mode.
	defaultCurrent  int
	defaultPageSize int
	appliedDefault  bool

	// Test / chrome hooks.
	items map[string]*primitive.Pressable // key = kind or kind:page
	// option widgets (rebuilt each time; not identity-stable).
	sizeSelect *Select
	jumper     *Input
}

// NewPagination creates Pagination with Ant defaults
// (current=1, pageSize=10, total=0 → one empty page).
func NewPagination() *Pagination {
	p := &Pagination{
		Current:                      DefaultPaginationCurrent,
		PageSize:                     DefaultPaginationPageSize,
		defaultCurrent:               DefaultPaginationCurrent,
		defaultPageSize:              DefaultPaginationPageSize,
		Size:                         PaginationMiddle,
		Align:                        PaginationAlignStart,
		TotalBoundaryShowSizeChanger: DefaultPaginationTotalBoundaryShowSizeChanger,
		PageSizeOptions:              []int{10, 20, 50, 100},
	}
	p.rebuild()
	return p
}

// Node returns the stable root.

// ensureBuilt materializes the control tree if missing (#9).
func (p *Pagination) ensureBuilt() {
	if p == nil {
		return
	}
	if p.Root == nil {
		p.rebuild()
	}
}

// structureChange rebuilds the control tree (#9).
func (p *Pagination) structureChange() {
	if p == nil {
		return
	}
	p.rebuild()
}

// chromeChange refreshes chrome via rebuild (#9).
func (p *Pagination) chromeChange() {
	if p == nil {
		return
	}
	p.ensureBuilt()
	p.rebuild()
}

func (p *Pagination) Node() core.Node {
	if p == nil {
		return nil
	}
	p.ensureBuilt()
	return p.Root
}

// ChromeNode returns the navigation root (same as Node for Pagination).
func (p *Pagination) ChromeNode() core.Node { return p.Node() }

// PageCount returns total pages: ceil(total/pageSize), at least 1.
func (p *Pagination) PageCount() int {
	if p == nil {
		return 1
	}
	ps := p.effectivePageSize()
	if ps < 1 {
		ps = DefaultPaginationPageSize
	}
	if p.Total <= 0 {
		return 1
	}
	n := (p.Total + ps - 1) / ps
	if n < 1 {
		return 1
	}
	return n
}

// EffectiveCurrent returns the clamped current page.
func (p *Pagination) EffectiveCurrent() int {
	if p == nil {
		return 1
	}
	return p.clampPage(p.Current)
}

// SetTotal sets data item count and clamps current.
func (p *Pagination) SetTotal(n int) {
	if p == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	p.Total = n
	p.Current = p.clampPage(p.Current)
	p.rebuild()
}

// SetCurrent sets the controlled current page (antd current).
func (p *Pagination) SetCurrent(page int) {
	if p == nil {
		return
	}
	p.currentControlled = true
	p.Current = p.clampPage(page)
	p.rebuild()
}

// SetDefaultCurrent sets the uncontrolled initial page (ignored once controlled).
func (p *Pagination) SetDefaultCurrent(page int) {
	if p == nil || p.currentControlled {
		return
	}
	p.defaultCurrent = page
	if page < 1 {
		page = 1
	}
	p.Current = p.clampPage(page)
	p.rebuild()
}

// SetPageSize sets controlled pageSize and clamps current.
func (p *Pagination) SetPageSize(n int) {
	if p == nil {
		return
	}
	if n < 1 {
		n = DefaultPaginationPageSize
	}
	p.pageSizeControlled = true
	p.PageSize = n
	p.Current = p.clampPage(p.Current)
	p.rebuild()
}

// SetDefaultPageSize sets uncontrolled default pageSize (ignored once controlled).
func (p *Pagination) SetDefaultPageSize(n int) {
	if p == nil || p.pageSizeControlled {
		return
	}
	if n < 1 {
		n = DefaultPaginationPageSize
	}
	p.defaultPageSize = n
	p.PageSize = n
	p.Current = p.clampPage(p.Current)
	p.rebuild()
}

// SetDisabled toggles interaction.
func (p *Pagination) SetDisabled(d bool) {
	if p == nil || p.Disabled == d {
		return
	}
	p.Disabled = d
	p.rebuild()
}

// SetSize sets small/middle/large item geometry.
func (p *Pagination) SetSize(s PaginationSize) {
	if p == nil || p.Size == s {
		return
	}
	p.Size = s
	p.rebuild()
}

// SetAlign sets root main-axis alignment.
func (p *Pagination) SetAlign(a PaginationAlign) {
	if p == nil || p.Align == a {
		return
	}
	p.Align = a
	p.rebuild()
}

// SetSimple enables simple pager (prev | n/N | next).
func (p *Pagination) SetSimple(v bool) {
	if p == nil || p.Simple == v {
		return
	}
	p.Simple = v
	p.rebuild()
}

// SetSimpleReadOnly makes the simple current page non-editable.
func (p *Pagination) SetSimpleReadOnly(v bool) {
	if p == nil || p.SimpleReadOnly == v {
		return
	}
	p.SimpleReadOnly = v
	if p.Simple {
		p.rebuild()
	}
}

// SetShowQuickJumper toggles the jump-to-page input.
func (p *Pagination) SetShowQuickJumper(v bool) {
	if p == nil || p.ShowQuickJumper == v {
		return
	}
	p.ShowQuickJumper = v
	p.rebuild()
}

// SetShowSizeChanger forces size-changer visibility (overrides auto boundary).
func (p *Pagination) SetShowSizeChanger(v bool) {
	if p == nil {
		return
	}
	p.showSizeChangerSet = true
	p.showSizeChanger = v
	p.rebuild()
}

// ShowSizeChanger reports whether the size changer is currently shown.
func (p *Pagination) ShowSizeChanger() bool {
	if p == nil {
		return false
	}
	return p.effectiveShowSizeChanger()
}

// SetHideOnSinglePage hides the pager when PageCount <= 1.
func (p *Pagination) SetHideOnSinglePage(v bool) {
	if p == nil || p.HideOnSinglePage == v {
		return
	}
	p.HideOnSinglePage = v
	p.rebuild()
}

// SetShowLessItems reduces the page buffer around current (ellipsis sooner).
func (p *Pagination) SetShowLessItems(v bool) {
	if p == nil || p.ShowLessItems == v {
		return
	}
	p.ShowLessItems = v
	p.rebuild()
}

// SetPageSizeOptions sets size-changer choices (empty → default 10/20/50/100).
func (p *Pagination) SetPageSizeOptions(opts []int) {
	if p == nil {
		return
	}
	p.PageSizeOptions = append([]int(nil), opts...)
	p.rebuild()
}

// SetTotalBoundaryShowSizeChanger sets the auto showSizeChanger threshold.
func (p *Pagination) SetTotalBoundaryShowSizeChanger(n int) {
	if p == nil {
		return
	}
	if n < 0 {
		n = 0
	}
	p.TotalBoundaryShowSizeChanger = n
	p.rebuild()
}

// SetShowTotal sets the total-label formatter (nil hides the label).
func (p *Pagination) SetShowTotal(fn func(total, start, end int) string) {
	if p == nil {
		return
	}
	p.ShowTotal = fn
	p.rebuild()
}

// SetOnChange sets the page/pageSize change callback.
func (p *Pagination) SetOnChange(fn func(page, pageSize int)) {
	if p == nil {
		return
	}
	p.OnChange = fn
}

// SetOnShowSizeChange sets the pageSize-only callback.
func (p *Pagination) SetOnShowSizeChange(fn func(current, size int)) {
	if p == nil {
		return
	}
	p.OnShowSizeChange = fn
}

// SetTheme sets an explicit theme override.
func (p *Pagination) SetTheme(th *core.Theme) {
	if p == nil {
		return
	}
	p.Theme = th
	p.rebuild()
}

// SetFace sets the label font face.
func (p *Pagination) SetFace(face text.Face) {
	if p == nil {
		return
	}
	p.Face = face
	p.rebuild()
}

// SetAriaLabel sets the accessible name on the navigation root.
func (p *Pagination) SetAriaLabel(name string) {
	if p == nil {
		return
	}
	p.AriaLabel = name
	if p.Root != nil {
		p.Root.Base().Label = name
	}
}

// ItemPressable returns the pressable for a page control (tests / gallery).
// kind is page|prev|next|jump-prev|jump-next; page is used for kind=page.
func (p *Pagination) ItemPressable(kind PaginationItemKind, page int) *primitive.Pressable {
	if p == nil {
		return nil
	}
	if p.Root == nil {
		p.rebuild()
	}
	if p.items == nil {
		return nil
	}
	key := itemKey(kind, page)
	return p.items[key]
}

// SizeSelect returns the pageSize Select when showSizeChanger is on (tests).
func (p *Pagination) SizeSelect() *Select {
	if p == nil {
		return nil
	}
	if p.Root == nil {
		p.rebuild()
	}
	return p.sizeSelect
}

// JumperInput returns the quick-jumper Input when showQuickJumper is on (tests).
func (p *Pagination) JumperInput() *Input {
	if p == nil {
		return nil
	}
	if p.Root == nil {
		p.rebuild()
	}
	return p.jumper
}

// Range returns inclusive 1-based [start, end] of items on the current page.
// When total=0 both are 0.
func (p *Pagination) Range() (start, end int) {
	if p == nil || p.Total <= 0 {
		return 0, 0
	}
	ps := p.effectivePageSize()
	cur := p.EffectiveCurrent()
	start = (cur-1)*ps + 1
	end = cur * ps
	if end > p.Total {
		end = p.Total
	}
	if start > p.Total {
		start = p.Total
	}
	return start, end
}

// ─── internals ───────────────────────────────────────────────────────────────

func (p *Pagination) theme() *core.Theme {
	var n core.Node
	if p.Root != nil {
		n = p.Root
	}
	return themeOf(p.Theme, n)
}

func (p *Pagination) effectivePageSize() int {
	ps := p.PageSize
	if ps < 1 {
		ps = p.defaultPageSize
	}
	if ps < 1 {
		ps = DefaultPaginationPageSize
	}
	return ps
}

func (p *Pagination) clampPage(page int) int {
	if page < 1 {
		page = 1
	}
	max := p.PageCount()
	if page > max {
		page = max
	}
	return page
}

func (p *Pagination) effectiveShowSizeChanger() bool {
	if p.showSizeChangerSet {
		return p.showSizeChanger
	}
	boundary := p.TotalBoundaryShowSizeChanger
	if boundary <= 0 {
		boundary = DefaultPaginationTotalBoundaryShowSizeChanger
	}
	return p.Total > boundary
}

func (p *Pagination) itemSize() float64 {
	th := p.theme()
	switch p.Size {
	case PaginationSmall:
		return th.SizeOr(core.TokenControlHeightSM, 24)
	case PaginationLarge:
		return th.SizeOr(core.TokenControlHeightLG, 40)
	default:
		return th.SizeOr(core.TokenControlHeight, 32)
	}
}

func (p *Pagination) fontSize() float64 {
	th := p.theme()
	switch p.Size {
	case PaginationSmall:
		return th.SizeOr(core.TokenFontSizeSM, 12)
	case PaginationLarge:
		return th.SizeOr(core.TokenFontSizeLG, 16)
	default:
		return th.SizeOr(core.TokenFontSize, 14)
	}
}

func (p *Pagination) radius() float64 {
	th := p.theme()
	switch p.Size {
	case PaginationSmall:
		return th.SizeOr(core.TokenBorderRadiusSM, 4)
	case PaginationLarge:
		return th.SizeOr(core.TokenBorderRadiusLG, 8)
	default:
		return th.SizeOr(core.TokenBorderRadius, 6)
	}
}

func itemKey(kind PaginationItemKind, page int) string {
	if kind == PaginationItemPage {
		return fmt.Sprintf("%s:%d", kind, page)
	}
	return string(kind)
}

// goTo changes page (uncontrolled writes Current; always fires OnChange when changed).
func (p *Pagination) goTo(page int) {
	if p == nil || p.Disabled {
		return
	}
	page = p.clampPage(page)
	ps := p.effectivePageSize()
	prev := p.EffectiveCurrent()
	if page == prev {
		return
	}
	if !p.currentControlled {
		p.Current = page
		p.rebuild()
	}
	// Controlled: parent must SetCurrent; still notify.
	if p.OnChange != nil {
		p.OnChange(page, ps)
	}
	// Controlled rebuild only when parent already updated Current to page
	// (common test pattern: onChange → SetCurrent). If still stale, leave UI;
	// gallery wires SetCurrent in OnChange.
	if p.currentControlled && p.Current == page {
		p.rebuild()
	}
}

// changePageSize updates pageSize and keeps a reasonable current page.
func (p *Pagination) changePageSize(size int) {
	if p == nil || p.Disabled {
		return
	}
	if size < 1 {
		size = DefaultPaginationPageSize
	}
	if size == p.effectivePageSize() {
		return
	}
	// Keep first item of old page visible when possible.
	start, _ := p.Range()
	newCurrent := 1
	if start > 0 {
		newCurrent = (start-1)/size + 1
	}
	prevCurrent := p.EffectiveCurrent()
	if !p.pageSizeControlled {
		p.PageSize = size
	}
	if !p.currentControlled {
		p.Current = newCurrent
	}
	// When controlled, parent owns both; still notify with intended values.
	page := newCurrent
	if p.currentControlled {
		page = prevCurrent
		// antd: onShowSizeChange(current, size) then onChange(page, size) with
		// current often recalculated by parent. We report the suggested page.
		page = newCurrent
	}
	if !p.pageSizeControlled || !p.currentControlled {
		p.rebuild()
	}
	if p.OnShowSizeChange != nil {
		p.OnShowSizeChange(page, size)
	}
	if p.OnChange != nil {
		p.OnChange(page, size)
	}
	if (p.pageSizeControlled && p.PageSize == size) ||
		(p.currentControlled && p.Current == page) {
		p.rebuild()
	}
}

func (p *Pagination) rebuild() {
	if p == nil {
		return
	}
	// Apply one-shot defaults.
	if !p.appliedDefault {
		if !p.currentControlled && p.defaultCurrent > 0 {
			p.Current = p.defaultCurrent
		}
		if !p.pageSizeControlled && p.defaultPageSize > 0 {
			p.PageSize = p.defaultPageSize
		}
		p.appliedDefault = true
	}
	p.Current = p.clampPage(p.Current)
	if p.PageSize < 1 {
		p.PageSize = DefaultPaginationPageSize
	}

	th := p.theme()
	if p.Root == nil {
		p.Root = primitive.Row()
	} else {
		p.Root.ClearChildren()
	}
	p.items = make(map[string]*primitive.Pressable)
	p.sizeSelect = nil
	p.jumper = nil

	p.Root.Gap = DefaultPaginationItemGap
	p.Root.SkinType = TypePagination
	p.Root.CrossAlign = core.CrossCenter
	switch p.Align {
	case PaginationAlignCenter:
		p.Root.MainAlign = core.MainCenter
	case PaginationAlignEnd:
		p.Root.MainAlign = core.MainEnd
	default:
		p.Root.MainAlign = core.MainStart
	}
	p.Root.Base().Role = "navigation"
	if p.AriaLabel != "" {
		p.Root.Base().Label = p.AriaLabel
	} else {
		p.Root.Base().Label = "Pagination"
	}

	// hideOnSinglePage
	if p.HideOnSinglePage && p.PageCount() <= 1 {
		p.Root.MarkNeedsLayout()
		p.Root.MarkNeedsPaint()
		return
	}

	// showTotal
	if p.ShowTotal != nil {
		start, end := p.Range()
		txt := p.ShowTotal(p.Total, start, end)
		if txt != "" {
			lab := primitive.NewText(txt)
			lab.FontSize = p.fontSize()
			lab.Face = p.Face
			if p.Disabled {
				lab.Color = th.Color(core.TokenColorDisabledText)
			} else {
				lab.Color = th.Color(core.TokenColorText)
			}
			p.Root.AddChild(lab)
		}
	}

	if p.Simple {
		p.buildSimple(th)
	} else {
		p.buildFull(th)
	}

	// size changer
	if p.effectiveShowSizeChanger() && !p.Simple {
		p.Root.AddChild(p.buildSizeChanger())
	}

	// quick jumper
	if p.ShowQuickJumper && !p.Simple {
		p.Root.AddChild(p.buildQuickJumper())
	}

	p.Root.MarkNeedsLayout()
	p.Root.MarkNeedsPaint()
}

func (p *Pagination) buildSimple(th *core.Theme) {
	cur := p.EffectiveCurrent()
	pages := p.PageCount()

	prev := p.makeItem("<", PaginationItemPrev, 0, cur <= 1, func() {
		p.goTo(cur - 1)
	})
	p.Root.AddChild(prev)

	// current / total
	if p.SimpleReadOnly || p.Disabled {
		info := primitive.NewText(fmt.Sprintf("%d / %d", cur, pages))
		info.FontSize = p.fontSize()
		info.Face = p.Face
		if p.Disabled {
			info.Color = th.Color(core.TokenColorDisabledText)
		} else {
			info.Color = th.Color(core.TokenColorText)
		}
		p.Root.AddChild(info)
	} else {
		in := NewInput("")
		in.SetFace(p.Face)
		in.SetTheme(p.Theme)
		switch p.Size {
		case PaginationSmall:
			in.SetSize(InputSmall)
		case PaginationLarge:
			in.SetSize(InputLarge)
		default:
			in.SetSize(InputMiddle)
		}
		in.SetValue(strconv.Itoa(cur))
		in.SetDisabled(p.Disabled)
		in.SetOnPressEnter(func(s string) {
			n, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return
			}
			p.goTo(n)
		})
		// modest width for page number
		in.SetFixedSize(p.itemSize()*1.6, p.itemSize())
		p.jumper = in
		slash := primitive.NewText(fmt.Sprintf(" / %d", pages))
		slash.FontSize = p.fontSize()
		slash.Face = p.Face
		slash.Color = th.Color(core.TokenColorText)
		row := primitive.Row(in.Node(), slash)
		row.Gap = 4
		row.CrossAlign = core.CrossCenter
		p.Root.AddChild(row)
	}

	next := p.makeItem(">", PaginationItemNext, 0, cur >= pages, func() {
		p.goTo(cur + 1)
	})
	p.Root.AddChild(next)
}

func (p *Pagination) buildFull(th *core.Theme) {
	_ = th
	cur := p.EffectiveCurrent()
	pages := p.PageCount()

	prev := p.makeItem("<", PaginationItemPrev, 0, cur <= 1, func() {
		p.goTo(cur - 1)
	})
	p.Root.AddChild(prev)

	for _, it := range p.pageList() {
		switch it.kind {
		case PaginationItemJumpPrev:
			jp := p.makeItem("•••", PaginationItemJumpPrev, 0, false, func() {
				// antd jumps ~5 pages
				p.goTo(cur - 5)
			})
			p.Root.AddChild(jp)
		case PaginationItemJumpNext:
			jn := p.makeItem("•••", PaginationItemJumpNext, 0, false, func() {
				p.goTo(cur + 5)
			})
			p.Root.AddChild(jn)
		default:
			page := it.page
			active := page == cur
			label := strconv.Itoa(page)
			item := p.makeItem(label, PaginationItemPage, page, false, func() {
				p.goTo(page)
			})
			if active {
				item.Base().Role = "button"
				item.Base().Label = fmt.Sprintf("Page %d, current", page)
			} else {
				item.Base().Label = fmt.Sprintf("Page %d", page)
			}
			// apply active chrome after makeItem
			if active {
				p.styleActive(item)
			} else if p.Disabled {
				// already styled disabled in makeItem
			}
			p.Root.AddChild(item)
		}
	}

	next := p.makeItem(">", PaginationItemNext, 0, cur >= pages, func() {
		p.goTo(cur + 1)
	})
	p.Root.AddChild(next)
}

type pageListItem struct {
	kind PaginationItemKind
	page int
}

// pageList builds page numbers + ellipsis (rc-pagination style).
func (p *Pagination) pageList() []pageListItem {
	totalPage := p.PageCount()
	current := p.EffectiveCurrent()
	buffer := 2
	if p.ShowLessItems {
		buffer = 1
	}

	// Show all when few pages: 1..total within buffer window + ends.
	// antd: all pages when totalPage <= 5 (buffer=2) or <= 3 (buffer=1) roughly
	// more precisely: totalPage <= buffer*2 + 3
	if totalPage <= buffer*2+3 {
		out := make([]pageListItem, 0, totalPage)
		for i := 1; i <= totalPage; i++ {
			out = append(out, pageListItem{kind: PaginationItemPage, page: i})
		}
		return out
	}

	// Window around current, always keep first & last.
	left := current - buffer
	right := current + buffer
	if left < 2 {
		right = maxInt(right, buffer*2+2)
		left = 2
	}
	if right > totalPage-1 {
		left = minInt(left, totalPage-buffer*2-1)
		right = totalPage - 1
	}
	if left < 2 {
		left = 2
	}
	if right > totalPage-1 {
		right = totalPage - 1
	}

	out := make([]pageListItem, 0, buffer*2+5)
	// first
	out = append(out, pageListItem{kind: PaginationItemPage, page: 1})
	// left ellipsis
	if left > 2 {
		out = append(out, pageListItem{kind: PaginationItemJumpPrev})
	} else if left == 2 {
		// show page 2 as normal
	}
	for i := left; i <= right; i++ {
		if i >= 2 && i <= totalPage-1 {
			out = append(out, pageListItem{kind: PaginationItemPage, page: i})
		}
	}
	// right ellipsis
	if right < totalPage-1 {
		out = append(out, pageListItem{kind: PaginationItemJumpNext})
	}
	// last
	out = append(out, pageListItem{kind: PaginationItemPage, page: totalPage})
	return out
}

func (p *Pagination) makeItem(label string, kind PaginationItemKind, page int, disabled bool, onClick func()) *primitive.Pressable {
	th := p.theme()
	h := p.itemSize()
	r := p.radius()
	fs := p.fontSize()

	lab := primitive.NewText(label)
	lab.FontSize = fs
	lab.Face = p.Face
	dis := disabled || p.Disabled
	if dis {
		lab.Color = th.Color(core.TokenColorDisabledText)
	} else {
		lab.Color = th.Color(core.TokenColorText)
	}

	dec := primitive.NewDecorated(lab)
	dec.MinWidth = h
	dec.Width = h
	dec.Height = h
	dec.MinHeight = h
	dec.Radius = r
	dec.SetCenterContent(true)
	dec.Background = render.RGBA{} // transparent default
	if dis {
		// no hover fill
	}

	pr := primitive.NewPressable(dec)
	pr.ShowFocusRing = true
	pr.FocusRingRadius = r
	pr.FocusRingOutset = 1.5
	pr.SetDisabled(dis)
	pr.Base().Role = "button"
	switch kind {
	case PaginationItemPrev:
		pr.Base().Label = "Previous page"
	case PaginationItemNext:
		pr.Base().Label = "Next page"
	case PaginationItemJumpPrev:
		pr.Base().Label = "Previous 5 pages"
		lab.Value = "•••"
	case PaginationItemJumpNext:
		pr.Base().Label = "Next 5 pages"
		lab.Value = "•••"
	default:
		pr.Base().Label = fmt.Sprintf("Page %s", label)
	}

	// Hover / press fill via OnStateChange (Token, no hard brand).
	pr.OnStateChange = func() {
		if dis {
			return
		}
		switch {
		case pr.State.Pressed:
			dec.Background = th.Color(core.TokenColorBgTextHover)
			if !isActiveItem(pr) {
				lab.Color = th.Color(core.TokenColorPrimary)
			}
		case pr.State.Hovered:
			dec.Background = th.Color(core.TokenColorBgTextHover)
			if !isActiveItem(pr) {
				lab.Color = th.Color(core.TokenColorPrimary)
			}
		default:
			if isActiveItem(pr) {
				// keep active style
				return
			}
			dec.Background = render.RGBA{}
			lab.Color = th.Color(core.TokenColorText)
		}
		dec.MarkNeedsPaint()
	}

	if !dis && onClick != nil {
		pr.Click = onClick
	}

	key := itemKey(kind, page)
	p.items[key] = pr
	return pr
}

func isActiveItem(pr *primitive.Pressable) bool {
	if pr == nil {
		return false
	}
	// Active pages carry "current" in Label.
	return strings.Contains(pr.Base().Label, "current")
}

func (p *Pagination) styleActive(pr *primitive.Pressable) {
	if pr == nil {
		return
	}
	kids := pr.Children()
	if len(kids) == 0 {
		return
	}
	th := p.theme()
	dec, ok := kids[0].(*primitive.Decorated)
	if !ok {
		return
	}
	if p.Disabled {
		// disabled active: muted fill, disabled text (no primary solid)
		dec.Background = th.Color(core.TokenColorDisabledBg)
		dec.BorderWidth = 0
		for _, c := range dec.Children() {
			if lab, ok := c.(*primitive.Text); ok {
				lab.Color = th.Color(core.TokenColorDisabledText)
			}
		}
		return
	}
	primary := th.Color(core.TokenColorPrimary)
	dec.Background = primary
	// fill primary like antd item-active
	dec.BorderWidth = 0
	for _, c := range dec.Children() {
		if lab, ok := c.(*primitive.Text); ok {
			inv := th.Color(core.TokenColorTextInverse)
			if inv.A < 0.1 {
				inv = render.RGBA{R: 1, G: 1, B: 1, A: 1}
			}
			lab.Color = inv
		}
	}
}

func (p *Pagination) buildSizeChanger() core.Node {
	opts := p.PageSizeOptions
	if len(opts) == 0 {
		opts = []int{10, 20, 50, 100}
	}
	selOpts := make([]SelectOption, 0, len(opts))
	for _, n := range opts {
		selOpts = append(selOpts, SelectOption{
			Value: strconv.Itoa(n),
			Label: fmt.Sprintf("%d / page", n),
		})
	}
	sel := NewSelect("page size", selOpts...)
	sel.Face = p.Face
	sel.Theme = p.Theme
	sel.SetDisabled(p.Disabled)
	// Seed value without firing OnChange (SetValue is silent).
	cur := strconv.Itoa(p.effectivePageSize())
	sel.SetValue(cur)
	sel.SetOnChange(func(v string) {
		n, err := strconv.Atoi(v)
		if err != nil {
			return
		}
		p.changePageSize(n)
	})
	p.sizeSelect = sel
	return sel.Node()
}

func (p *Pagination) buildQuickJumper() core.Node {
	th := p.theme()
	prefix := primitive.NewText("Go to")
	prefix.FontSize = p.fontSize()
	prefix.Face = p.Face
	if p.Disabled {
		prefix.Color = th.Color(core.TokenColorDisabledText)
	} else {
		prefix.Color = th.Color(core.TokenColorText)
	}

	in := NewInput("")
	in.SetFace(p.Face)
	in.SetTheme(p.Theme)
	switch p.Size {
	case PaginationSmall:
		in.SetSize(InputSmall)
	case PaginationLarge:
		in.SetSize(InputLarge)
	default:
		in.SetSize(InputMiddle)
	}
	in.SetDisabled(p.Disabled)
	in.SetFixedSize(p.itemSize()*1.8, p.itemSize())
	in.SetOnPressEnter(func(s string) {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return
		}
		p.goTo(n)
	})
	p.jumper = in

	row := primitive.Row(prefix, in.Node())
	row.Gap = 8
	row.CrossAlign = core.CrossCenter
	return row
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
