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
	Expanded map[string]bool // optional expand override by key
	Face     text.Face
	Theme    *core.Theme
	OnSelect func(key string)
}

// NewTree creates a tree.
func NewTree(nodes ...*TreeNode) *Tree {
	tr := &Tree{Nodes: nodes, Expanded: make(map[string]bool)}
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

// SetSelected selects a node by key and rebuilds.
func (tr *Tree) SetSelected(key string) {
	tr.Selected = key
	if tr.OnSelect != nil {
		tr.OnSelect(key)
	}
	tr.rebuild()
}

// ToggleExpand toggles expanded state for a key.
func (tr *Tree) ToggleExpand(key string) {
	if tr.Expanded == nil {
		tr.Expanded = make(map[string]bool)
	}
	// flip map and node.Expanded when found
	if v, ok := tr.Expanded[key]; ok {
		tr.Expanded[key] = !v
	} else {
		// seed from node if present
		n := tr.findNode(tr.Nodes, key)
		cur := false
		if n != nil {
			cur = n.Expanded
		}
		tr.Expanded[key] = !cur
	}
	if n := tr.findNode(tr.Nodes, key); n != nil {
		n.Expanded = tr.Expanded[key]
	}
	tr.rebuild()
}

func (tr *Tree) findNode(nodes []*TreeNode, key string) *TreeNode {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if n.Key == key {
			return n
		}
		if c := tr.findNode(n.Children, key); c != nil {
			return c
		}
	}
	return nil
}

func (tr *Tree) isExpanded(n *TreeNode) bool {
	if n == nil {
		return false
	}
	if tr.Expanded != nil {
		if v, ok := tr.Expanded[n.Key]; ok {
			return v
		}
	}
	return n.Expanded
}

func (tr *Tree) theme() *core.Theme {
	if tr.Theme != nil {
		return tr.Theme
	}
	return DefaultTheme()
}

func (tr *Tree) rebuild() {
	th := tr.theme()
	if tr.Expanded == nil {
		tr.Expanded = make(map[string]bool)
	}
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
	expanded := tr.isExpanded(n)
	var chev core.Node
	if hasKids {
		name := "chevron-right"
		if expanded {
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
			if tr.Expanded == nil {
				tr.Expanded = make(map[string]bool)
			}
			cur := tr.isExpanded(node)
			tr.Expanded[node.Key] = !cur
			node.Expanded = !cur
		}
		tr.Selected = node.Key
		if tr.OnSelect != nil {
			tr.OnSelect(node.Key)
		}
		tr.rebuild()
	}
	parent.AddChild(press)
	if hasKids && expanded {
		for _, c := range n.Children {
			tr.addNode(parent, c, depth+1)
		}
	}
}
