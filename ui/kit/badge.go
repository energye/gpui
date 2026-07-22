package kit

import (
	"fmt"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Badge overlays a count/dot on a child (simplified Ant Badge).
// https://ant.design/components/badge
type Badge struct {
	Root  *primitive.Stack
	child core.Node
	Count int
	Dot   bool
	Face  text.Face
	Theme *core.Theme
}

// NewBadge wraps child with a count badge.
func NewBadge(child core.Node, count int) *Badge {
	b := &Badge{child: child, Count: count}
	b.rebuild()
	return b
}

// Node returns root.
func (b *Badge) Node() core.Node {
	if b.Root == nil {
		b.rebuild()
	}
	return b.Root
}

// SetCount updates badge number (0 hides count text when !Dot).
func (b *Badge) SetCount(n int) {
	b.Count = n
	b.rebuild()
}

func (b *Badge) rebuild() {
	th := DefaultTheme()
	if b.Theme != nil {
		th = b.Theme
	}
	b.Root = primitive.NewStack()
	if b.child != nil {
		b.Root.AddChild(b.child)
	}
	var mark core.Node
	if b.Dot {
		d := primitive.NewBox()
		d.Width, d.Height = 8, 8
		d.Color = th.Color(core.TokenColorError)
		if d.Color.A < 0.5 {
			d.Color = render.Hex("#FF4D4F")
		}
		mark = d
	} else if b.Count > 0 {
		txt := fmt.Sprintf("%d", b.Count)
		if b.Count > 99 {
			txt = "99+"
		}
		lab := primitive.NewText(txt)
		lab.FontSize = 10
		lab.Face = b.Face
		lab.Color = render.RGBA{R: 1, G: 1, B: 1, A: 1}
		dec := primitive.NewDecorated(lab)
		dec.Padding = primitive.Symmetric(5, 1)
		dec.Radius = 10
		dec.Background = th.Color(core.TokenColorError)
		if dec.Background.A < 0.5 {
			dec.Background = render.Hex("#FF4D4F")
		}
		mark = dec
	}
	if mark != nil {
		b.Root.AddChild(primitive.Positioned(core.AlignTopRight, mark))
	}
}
