package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// TreeNode is one tree entry.
type TreeNode struct {
	Key      string
	Title    string
	Children []*TreeNode
	Expanded bool
}

// Tree is an expandable tree (B4 base, non-virtual).
type Tree struct {
	Root     *primitive.Decorated
	col      *primitive.Flex
	Nodes    []*TreeNode
	Selected string
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(key string)
}

// NewTree creates a tree.
func NewTree(nodes ...*TreeNode) *Tree {
	tr := &Tree{Nodes: nodes}
	tr.rebuild()
	return tr
}

// Node returns the root.
func (tr *Tree) Node() core.Node {
	if tr.Root == nil {
		tr.rebuild()
	}
	return tr.Root
}

func (tr *Tree) theme() *core.Theme {
	if tr.Theme != nil {
		return tr.Theme
	}
	return DefaultTheme()
}

func (tr *Tree) rebuild() {
	th := tr.theme()
	tr.col = primitive.Column()
	tr.col.Gap = 2
	tr.col.CrossAlign = core.CrossStart
	for _, n := range tr.Nodes {
		tr.addNode(tr.col, n, 0)
	}
	tr.Root = primitive.NewDecorated(tr.col)
	tr.Root.Padding = primitive.All(4)
	tr.Root.BorderWidth = th.SizeOr(core.TokenLineWidth, 1)
	tr.Root.BorderColor = th.Color(core.TokenColorBorder)
	tr.Root.Radius = th.SizeOr(core.TokenBorderRadius, 6)
	tr.Root.Background = th.Color(core.TokenColorBgContainer)
}

func (tr *Tree) addNode(parent *primitive.Flex, n *TreeNode, depth int) {
	if n == nil {
		return
	}
	th := tr.theme()
	hasKids := len(n.Children) > 0
	var chev core.Node
	if hasKids {
		name := "chevron-right"
		if n.Expanded {
			name = "chevron-down"
		}
		ic := primitive.NewIcon(name)
		ic.Size = 12
		ic.Color = th.Color(core.TokenColorTextSecondary)
		chev = ic
	} else {
		sp := primitive.NewBox()
		sp.Width, sp.Height = 12, 12
		chev = sp
	}
	lab := primitive.NewText(n.Title)
	lab.FontSize = th.SizeOr(core.TokenFontSize, 14)
	lab.Face = tr.Face
	lab.Color = th.Color(core.TokenColorText)
	indent := primitive.NewBox()
	indent.Width = float64(depth) * 16

	row := primitive.Row(indent, chev, lab)
	row.Gap = 6
	row.CrossAlign = core.CrossCenter
	press := primitive.NewPressable(row)
	press.Padding = primitive.Symmetric(4, 5) // Ant Tree node
	if n.Key == tr.Selected {
		press.Color = antItemSelectedFill(th)
		lab.Color = antItemSelectedText(th)
	}
	press.ColorHovered = antItemHoverFill(th)
	node := n
	press.Click = func() {
		if hasKids {
			node.Expanded = !node.Expanded
		}
		tr.Selected = node.Key
		if tr.OnSelect != nil {
			tr.OnSelect(node.Key)
		}
		tr.rebuild()
	}
	parent.AddChild(press)
	if hasKids && n.Expanded {
		for _, c := range n.Children {
			tr.addNode(parent, c, depth+1)
		}
	}
}
