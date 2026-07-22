package kit

import (
	"fmt"

	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Pagination is page navigation controls.
type Pagination struct {
	Root            *primitive.Flex
	Current         int
	Total           int // total pages
	PageSize        int
	TotalItems      int
	ShowQuickJumper bool
	Face            text.Face
	Theme           *core.Theme
	OnChange        func(page int)
}

// NewPagination creates pagination. totalPages at least 1.
func NewPagination(totalPages int) *Pagination {
	if totalPages < 1 {
		totalPages = 1
	}
	p := &Pagination{Current: 1, Total: totalPages, PageSize: 10}
	p.rebuild()
	return p
}

// Node returns the root.
func (p *Pagination) Node() core.Node {
	if p.Root == nil {
		p.rebuild()
	}
	return p.Root
}

// SetPage sets current page (1-based).
func (p *Pagination) SetPage(page int) {
	if page < 1 {
		page = 1
	}
	if page > p.Total {
		page = p.Total
	}
	if p.Current == page {
		return
	}
	p.Current = page
	p.rebuild()
	if p.OnChange != nil {
		p.OnChange(page)
	}
}

// SetTotalPages updates total page count and clamps Current.
func (p *Pagination) SetTotalPages(n int) {
	if n < 1 {
		n = 1
	}
	p.Total = n
	if p.Current > p.Total {
		p.Current = p.Total
	}
	p.rebuild()
}

func (p *Pagination) theme() *core.Theme {
	if p.Theme != nil {
		return p.Theme
	}
	return DefaultTheme()
}

func (p *Pagination) rebuild() {
	th := p.theme()
	prev := NewButton("<")
	prev.SetSize(ButtonSmall)
	prev.SetFace(p.Face)
	prev.SetOnClick(func() { p.SetPage(p.Current - 1) })
	if p.Current <= 1 {
		prev.SetDisabled(true)
	}
	next := NewButton(">")
	next.SetSize(ButtonSmall)
	next.SetFace(p.Face)
	next.SetOnClick(func() { p.SetPage(p.Current + 1) })
	if p.Current >= p.Total {
		next.SetDisabled(true)
	}
	info := primitive.NewText(fmt.Sprintf("%d / %d", p.Current, p.Total))
	info.FontSize = th.SizeOr(core.TokenFontSize, 14)
	info.Face = p.Face
	info.Color = th.Color(core.TokenColorText)

	// page number buttons (window of up to 5)
	nums := primitive.Row()
	nums.Gap = 4
	start := p.Current - 2
	if start < 1 {
		start = 1
	}
	end := start + 4
	if end > p.Total {
		end = p.Total
		start = end - 4
		if start < 1 {
			start = 1
		}
	}
	for i := start; i <= end; i++ {
		i := i
		b := NewButton(fmt.Sprintf("%d", i))
		b.SetSize(ButtonSmall)
		b.SetFace(p.Face)
		if i == p.Current {
			b.SetType(ButtonPrimary)
		}
		b.SetOnClick(func() { p.SetPage(i) })
		nums.AddChild(b.Node())
	}

	p.Root = primitive.Row(prev.Node(), nums, next.Node(), info)
	p.Root.Gap = 8
	p.Root.CrossAlign = core.CrossCenter
}
