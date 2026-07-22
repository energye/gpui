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
	sticky  *primitive.Sticky
	// OffsetTop is the sticky top offset in logical px.
	OffsetTop float64
	// Affixed is true when currently stuck (host may update).
	Affixed bool
}

// NewAffix wraps content; uses Sticky if available, else identity.
func NewAffix(content core.Node) *Affix {
	st := primitive.NewSticky(content)
	return &Affix{Root: st, Content: content, sticky: st}
}

// Node returns root.
func (a *Affix) Node() core.Node {
	if a == nil {
		return nil
	}
	return a.Root
}

// SetOffsetTop sets sticky top offset.
func (a *Affix) SetOffsetTop(top float64) {
	if a == nil {
		return
	}
	a.OffsetTop = top
	if a.sticky != nil {
		a.sticky.Top = top
		a.sticky.UseTop = true
	}
}
