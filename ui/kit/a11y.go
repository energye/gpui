package kit

import "github.com/energye/gpui/ui/core"

// A11yNode is a flattened accessibility snapshot for tests/tools.
type A11yNode struct {
	Role   string
	Label  string
	Live   string
	Type   string
	Bounds core.Rect
}

// CollectA11y returns a flat list of accessible nodes (for tests/tools).
func CollectA11y(root core.Node) []A11yNode {
	var out []A11yNode
	var walk func(core.Node)
	walk = func(n core.Node) {
		if n == nil {
			return
		}
		b := n.Base()
		if b.Role != "" || b.Label != "" {
			out = append(out, A11yNode{
				Role: b.Role, Label: b.Label, Live: b.Live,
				Type: n.TypeID(), Bounds: core.AbsoluteBounds(n),
			})
		}
		for _, c := range n.Children() {
			walk(c)
		}
	}
	walk(root)
	return out
}
