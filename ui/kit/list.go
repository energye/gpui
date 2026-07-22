package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// List is a simple vertical list (non-virtual or virtual for large N).
type List struct {
	Root       *primitive.Decorated
	scroll     *primitive.ScrollViewport
	vlist      *primitive.VirtualList
	Items      []string
	Selected   int
	ItemHeight float64
	Virtual    bool
	Face       text.Face
	Theme      *core.Theme
	OnSelect   func(index int, item string)
}

// NewList creates a list.
func NewList(items ...string) *List {
	l := &List{Items: items, Selected: -1, ItemHeight: 40, Virtual: len(items) > 40}
	l.rebuild()
	return l
}

// Node returns the root.
func (l *List) Node() core.Node {
	if l.Root == nil {
		l.rebuild()
	}
	return l.Root
}

// SetItems replaces items.
func (l *List) SetItems(items []string) {
	l.Items = items
	l.Virtual = len(items) > 40
	l.rebuild()
}

// SetSelected selects an index (-1 clears).
func (l *List) SetSelected(index int) {
	if index < -1 {
		index = -1
	}
	if index >= len(l.Items) {
		index = len(l.Items) - 1
	}
	l.Selected = index
	l.rebuild()
}

func (l *List) theme() *core.Theme {
	if l.Theme != nil {
		return l.Theme
	}
	return DefaultTheme()
}

func (l *List) rebuild() {
	th := l.theme()
	if l.Virtual {
		l.vlist = primitive.NewVirtualList(l.ItemHeight, func(i int) core.Node {
			return l.itemNode(i)
		})
		l.vlist.ItemCount = len(l.Items)
		l.vlist.Height = 200
		l.Root = primitive.NewDecorated(l.vlist)
	} else {
		col := primitive.Column()
		col.Gap = 0
		for i := range l.Items {
			col.AddChild(l.itemNode(i))
		}
		l.scroll = primitive.NewScrollViewport(col)
		l.scroll.Height = 200
		l.Root = primitive.NewDecorated(l.scroll)
	}
	l.Root.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	l.Root.BorderColor = th.Color(core.TokenColorBorder)
	l.Root.Radius = th.SizeOr(core.TokenBorderRadiusLG, 8)
	l.Root.Padding = primitive.All(4)
	l.Root.Background = th.Color(core.TokenColorBgContainer)
}

func (l *List) itemNode(i int) core.Node {
	th := l.theme()
	if i < 0 || i >= len(l.Items) {
		return primitive.NewBox()
	}
	lab := primitive.NewText(l.Items[i])
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = l.Face
	lab.Color = th.Color(core.TokenColorText)
	press := primitive.NewPressable(lab)
	press.Padding = primitive.Symmetric(12, 5) // Ant list item
	if i == l.Selected {
		press.Color = antItemSelectedFill(th)
		lab.Color = antItemSelectedText(th)
	}
	press.ColorHovered = antItemHoverFill(th)
	idx := i
	press.Click = func() {
		l.Selected = idx
		if l.OnSelect != nil {
			l.OnSelect(idx, l.Items[idx])
		}
		l.rebuild()
	}
	return press
}
