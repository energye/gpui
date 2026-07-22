package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Divider is kit wrapper around primitive.Divider (Ant Divider).
// https://ant.design/components/divider
type Divider struct {
	Root *primitive.Divider
}

// NewDivider creates a horizontal divider.
func NewDivider() *Divider {
	d := primitive.NewDivider()
	d.ColorToken = core.TokenColorBorder
	return &Divider{Root: d}
}

// Node returns root.
func (d *Divider) Node() core.Node {
	if d == nil {
		return nil
	}
	return d.Root
}

// SetVertical makes a vertical divider.
func (d *Divider) SetVertical(v bool) {
	if d != nil && d.Root != nil {
		d.Root.Vertical = v
	}
}
