package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Splitter is kit wrapper for SplitPane.
// https://ant.design/components/splitter
type Splitter struct {
	Root *primitive.SplitPane
}

// NewSplitter creates a split of two children.
func NewSplitter(first, second core.Node) *Splitter {
	return &Splitter{Root: primitive.NewSplitPane(first, second)}
}

// Node returns root.
func (s *Splitter) Node() core.Node {
	if s == nil {
		return nil
	}
	return s.Root
}
