package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Flex is kit wrapper for Ant Flex.
// https://ant.design/components/flex
type Flex struct {
	Root *primitive.Flex
	// Wrap is stored for Ant API parity (layout wrap deferred).
	Wrap bool
}

// NewFlexRow creates a horizontal flex.
func NewFlexRow(children ...core.Node) *Flex {
	return &Flex{Root: primitive.Row(children...)}
}

// NewFlexColumn creates a vertical flex.
func NewFlexColumn(children ...core.Node) *Flex {
	return &Flex{Root: primitive.Column(children...)}
}

// Node returns root.
func (f *Flex) Node() core.Node {
	if f == nil {
		return nil
	}
	return f.Root
}

// SetGap sets gap.
func (f *Flex) SetGap(g float64) {
	if f != nil && f.Root != nil {
		f.Root.Gap = g
	}
}

// SetWrap sets wrap flag (layout wrap deferred; API parity).
func (f *Flex) SetWrap(v bool) {
	if f != nil {
		f.Wrap = v
	}
}
