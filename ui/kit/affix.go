package kit

import (
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Affix wraps content (sticky behavior via primitive.Sticky when available).
// https://ant.design/components/affix
type Affix struct {
	Root    core.Node
	Content core.Node
}

// NewAffix wraps content; uses Sticky if available, else identity.
func NewAffix(content core.Node) *Affix {
	// Prefer Sticky wrapper when type exists
	st := primitive.NewSticky(content)
	return &Affix{Root: st, Content: content}
}

// Node returns root.
func (a *Affix) Node() core.Node {
	if a == nil {
		return nil
	}
	return a.Root
}
