package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Anchor is in-page link list (scroll-spy deferred).
// https://ant.design/components/anchor
type Anchor struct {
	Root   *primitive.Flex
	Items  []string
	Active string
	Face   text.Face
	// ScrollTarget when set: clicking an item scrolls it into view (scroll-spy basic).
	ScrollTarget *primitive.ScrollViewport
	// SectionOffsets maps item key → content Y in the scroll content (caller fills).
	SectionOffsets map[string]float64
	OnClick        func(item string)
}

// NewAnchor creates anchor links.
func NewAnchor(items ...string) *Anchor {
	a := &Anchor{Items: append([]string(nil), items...)}
	a.rebuild()
	return a
}

// Node returns root.
func (a *Anchor) Node() core.Node {
	if a.Root == nil {
		a.rebuild()
	}
	return a.Root
}

// SetFace sets font.
func (a *Anchor) SetFace(face text.Face) {
	a.Face = face
	a.rebuild()
}

// SetActive sets the active item key.
func (a *Anchor) SetActive(key string) {
	if a.Active == key {
		return
	}
	a.Active = key
	a.rebuild()
}

// SetItems replaces link items.
func (a *Anchor) SetItems(items []string) {
	a.Items = append([]string(nil), items...)
	a.rebuild()
}

// SyncFromScroll updates Active from ScrollTarget.ScrollY + SectionOffsets (scroll-spy R2).
func (a *Anchor) SyncFromScroll() {
	if a == nil || a.ScrollTarget == nil || len(a.SectionOffsets) == 0 {
		return
	}
	y := a.ScrollTarget.ScrollY
	// pick last section whose offset <= scrollY
	best, bestY := "", -1.0
	for k, off := range a.SectionOffsets {
		if off <= y+1 && off >= bestY {
			best, bestY = k, off
		}
	}
	if best != "" && best != a.Active {
		a.Active = best
		a.rebuild()
	}
}

func (a *Anchor) rebuild() {
	th := DefaultTheme()
	if a.Root == nil {
		a.Root = primitive.Column()
	} else {
		a.Root.ClearChildren()
	}
	a.Root.Gap = 4
	for _, it := range a.Items {
		it := it
		lab := primitive.NewText(it)
		lab.FontSize = 14
		lab.Face = a.Face
		if it == a.Active {
			lab.Color = th.Color(core.TokenColorText)
		} else {
			lab.Color = th.Color(core.TokenColorPrimary)
		}
		p := primitive.NewPressable(lab)
		p.ShowFocusRing = false
		p.Click = func() {
			a.Active = it
			if a.ScrollTarget != nil && a.SectionOffsets != nil {
				if y, ok := a.SectionOffsets[it]; ok {
					a.ScrollTarget.SetScroll(a.ScrollTarget.ScrollX, y)
				}
			}
			a.rebuild()
			if a.OnClick != nil {
				a.OnClick(it)
			}
			if a.Root != nil {
				a.Root.MarkNeedsLayout()
			}
		}
		a.Root.AddChild(p)
	}
}
