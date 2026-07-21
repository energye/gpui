package primitive

import (
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
)

// Type IDs (stable strings for Skin/Plugin).
const (
	TypeBox            = "primitive.Box"
	TypeFlex           = "primitive.Flex"
	TypeStack          = "primitive.Stack"
	TypeFlexible       = "primitive.Flexible"
	TypeText           = "primitive.Text"
	TypePressable      = "primitive.Pressable"
	TypeClip           = "primitive.Clip"
	TypeDecorated      = "primitive.Decorated"
	TypeSlot           = "primitive.Slot"
	TypeIcon           = "primitive.Icon"
	TypeFocusable      = "primitive.Focusable"
	TypeHitTarget      = "primitive.HitTarget"
	TypeDivider        = "primitive.Divider"
	TypePainterNode    = "primitive.PainterNode"
	TypeEditableText   = "primitive.EditableText"
	TypeScrollViewport = "primitive.ScrollViewport"
	TypeOverlayPortal  = "primitive.OverlayPortal"
	TypeMask           = "primitive.Mask"
	TypeAnchoredPopup  = "primitive.AnchoredPopup"
	TypeTrigger        = "primitive.Trigger"
	TypeVirtualList    = "primitive.VirtualList"
	TypeFocusScope     = "primitive.FocusScope"
)

// EdgeInsets is padding/margin on four sides.
type EdgeInsets struct {
	Left, Top, Right, Bottom float64
}

// All returns equal insets on all sides.
func All(v float64) EdgeInsets { return EdgeInsets{v, v, v, v} }

// Symmetric returns horizontal/vertical insets.
func Symmetric(h, v float64) EdgeInsets {
	return EdgeInsets{Left: h, Top: v, Right: h, Bottom: v}
}

// Box is a layout box: optional fixed size, padding, background, single/multi children.
type Box struct {
	core.NodeBase

	// Width/Height when > 0 request a preferred size (still constrained).
	Width, Height float64
	Padding       EdgeInsets
	// Color is optional background (A=0 means no fill).
	Color render.RGBA
}

// NewBox constructs a Box and initializes the node base.
func NewBox(children ...core.Node) *Box {
	b := &Box{}
	b.Init(b)
	b.Hit = core.HitBlock
	for _, c := range children {
		b.AddChild(c)
	}
	return b
}

// TypeID implements core.Node.
func (b *Box) TypeID() string { return TypeBox }

// Layout implements core.Node.
func (b *Box) Layout(c core.Constraints) core.Size {
	inner := c.Deflate(b.Padding.Left, b.Padding.Top, b.Padding.Right, b.Padding.Bottom)
	// Preferred size if set.
	if b.Width > 0 {
		inner = inner.WithMaxWidth(b.Width - b.Padding.Left - b.Padding.Right)
		if inner.MinWidth > inner.MaxWidth {
			inner.MinWidth = inner.MaxWidth
		}
	}
	if b.Height > 0 {
		inner = inner.WithMaxHeight(b.Height - b.Padding.Top - b.Padding.Bottom)
		if inner.MinHeight > inner.MaxHeight {
			inner.MinHeight = inner.MaxHeight
		}
	}

	content := core.Size{}
	kids := b.Children()
	if len(kids) == 1 {
		content = kids[0].Layout(inner.Expand())
		kids[0].Base().SetOffset(core.Point{X: b.Padding.Left, Y: b.Padding.Top})
	} else if len(kids) > 1 {
		// Stack-like: max of children at padding origin.
		for _, child := range kids {
			sz := child.Layout(inner.Expand())
			child.Base().SetOffset(core.Point{X: b.Padding.Left, Y: b.Padding.Top})
			content = core.MaxSize(content, sz)
		}
	}

	w := content.Width + b.Padding.Left + b.Padding.Right
	h := content.Height + b.Padding.Top + b.Padding.Bottom
	if b.Width > 0 {
		w = b.Width
	}
	if b.Height > 0 {
		h = b.Height
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	b.SetSize(out)
	return out
}

// Paint implements core.Node.
func (b *Box) Paint(pc *core.PaintContext) {
	if b.Color.A > 0 && pc != nil {
		pc.FillLocalRect(0, 0, b.Size().Width, b.Size().Height, b.Color)
	}
	b.DefaultPaintChildren(pc)
}

// HitTest implements core.Node.
func (b *Box) HitTest(p core.Point) core.Node { return b.DefaultHitTest(p) }
