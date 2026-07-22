package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Layout is Ant Layout shell: header / sider / content / footer.
// https://ant.design/components/layout
type Layout struct {
	Root       *primitive.Flex
	Header     core.Node
	Sider      core.Node
	Content    core.Node
	Footer     core.Node
	SiderWidth float64 // 0 → hug sider content
}

// NewLayout builds a classic layout.
func NewLayout(header, sider, content, footer core.Node) *Layout {
	l := &Layout{Header: header, Sider: sider, Content: content, Footer: footer}
	l.rebuild()
	return l
}

// Node returns root.
func (l *Layout) Node() core.Node {
	if l.Root == nil {
		l.rebuild()
	}
	return l.Root
}

// SetSiderWidth fixes sider width (0 → hug content).
func (l *Layout) SetSiderWidth(w float64) {
	l.SiderWidth = w
	l.rebuild()
}

func (l *Layout) rebuild() {
	body := primitive.Row()
	body.CrossAlign = core.CrossStretch
	if l.Sider != nil {
		if l.SiderWidth > 0 {
			host := primitive.NewDecorated(l.Sider)
			host.Width = l.SiderWidth
			host.MinWidth = l.SiderWidth
			host.StretchChild = true
			body.AddChild(host)
		} else {
			body.AddChild(l.Sider)
		}
	}
	if l.Content != nil {
		host := primitive.NewFlexible(1, l.Content)
		host.FillChild = true
		body.AddChild(host)
	}
	col := primitive.Column()
	col.CrossAlign = core.CrossStretch
	if l.Header != nil {
		col.AddChild(l.Header)
	}
	mid := primitive.NewFlexible(1, body)
	mid.FillChild = true
	col.AddChild(mid)
	if l.Footer != nil {
		col.AddChild(l.Footer)
	}
	l.Root = col
}
