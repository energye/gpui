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

// Ratio returns the split ratio (0..1).
func (s *Splitter) Ratio() float64 {
	if s == nil || s.Root == nil {
		return 0.5
	}
	return s.Root.Ratio
}

// SetRatio sets the split ratio (0..1).
func (s *Splitter) SetRatio(r float64) {
	if s == nil || s.Root == nil {
		return
	}
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	s.Root.Ratio = r
	s.Root.MarkNeedsLayout()
}
