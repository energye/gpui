package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Cascader is a simplified multi-column cascade picker (B4 simplified).
// Root stays stable when columns are added/removed on selection.
type Cascader struct {
	Root     *primitive.Flex
	Columns  []*List
	Options  []*TreeNode // reuse tree structure
	Path     []string    // selected keys (alias of Value)
	Value    []string    // selected path keys
	Face     text.Face
	Theme    *core.Theme
	OnChange func(path []string)
}

// NewCascader creates a cascader from tree options.
func NewCascader(options ...*TreeNode) *Cascader {
	c := &Cascader{Options: options}
	c.rebuild()
	return c
}

// Node returns the root.
func (c *Cascader) Node() core.Node {
	if c.Root == nil {
		c.rebuild()
	}
	return c.Root
}

// SetValue sets the selected path keys.
func (c *Cascader) SetValue(path []string) {
	c.Value = append([]string(nil), path...)
	c.Path = append([]string(nil), path...)
	if c.OnChange != nil {
		c.OnChange(c.Value)
	}
}

// GetValue returns the selected path keys.
func (c *Cascader) GetValue() []string {
	if len(c.Value) > 0 {
		return append([]string(nil), c.Value...)
	}
	return append([]string(nil), c.Path...)
}

func (c *Cascader) rebuild() {
	if c.Root == nil {
		c.Root = primitive.Row()
	} else {
		c.Root.ClearChildren()
	}
	c.Root.Gap = 8
	// first column = roots
	labels := make([]string, 0, len(c.Options))
	for _, n := range c.Options {
		labels = append(labels, n.Title)
	}
	col0 := NewList(labels...)
	col0.Face = c.Face
	col0.OnSelect = func(i int, _ string) {
		if i < 0 || i >= len(c.Options) {
			return
		}
		c.Path = []string{c.Options[i].Key}
		c.Value = append([]string(nil), c.Path...)
		c.showLevel(1, c.Options[i].Children)
		if c.OnChange != nil {
			c.OnChange(c.Path)
		}
	}
	c.Columns = []*List{col0}
	c.Root.AddChild(col0.Node())
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}

func (c *Cascader) showLevel(level int, nodes []*TreeNode) {
	// trim extra columns
	for len(c.Columns) > level {
		c.Columns = c.Columns[:level]
	}
	if len(nodes) == 0 {
		c.rebuildRootFromColumns()
		return
	}
	labels := make([]string, 0, len(nodes))
	for _, n := range nodes {
		labels = append(labels, n.Title)
	}
	col := NewList(labels...)
	col.Face = c.Face
	col.OnSelect = func(i int, _ string) {
		if i < 0 || i >= len(nodes) {
			return
		}
		// truncate path to level
		if len(c.Path) > level {
			c.Path = c.Path[:level]
		}
		if len(c.Path) == level {
			c.Path = append(c.Path, nodes[i].Key)
		} else {
			c.Path = append(c.Path[:level], nodes[i].Key)
		}
		c.Value = append([]string(nil), c.Path...)
		c.showLevel(level+1, nodes[i].Children)
		if c.OnChange != nil {
			c.OnChange(c.Path)
		}
	}
	c.Columns = append(c.Columns, col)
	c.rebuildRootFromColumns()
}

func (c *Cascader) rebuildRootFromColumns() {
	if c.Root == nil {
		c.Root = primitive.Row()
	} else {
		c.Root.ClearChildren()
	}
	c.Root.Gap = 8
	for _, col := range c.Columns {
		c.Root.AddChild(col.Node())
	}
	c.Root.MarkNeedsLayout()
	c.Root.MarkNeedsPaint()
}
