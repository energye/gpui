package primitive

import "github.com/energye/gpui/ui/core"

// layoutPaddedChild is the Flutter-style single-child + padding pass used by
// Box / Focusable / PainterNode shells:
//
//	child gets Expand() of deflated constraints
//	child offset = (padL, padT)  // top-left, hit == paint
//	outer size = child + padding, then parent Tighten
//
// Preferred Width/Height (when > 0) lock the outer box before Tighten.
func layoutPaddedChild(
	parent *core.NodeBase,
	c core.Constraints,
	pad EdgeInsets,
	prefW, prefH float64,
) core.Size {
	inner := c.Deflate(pad.Left, pad.Top, pad.Right, pad.Bottom)
	if prefW > 0 {
		iw := prefW - pad.Left - pad.Right
		if iw < 0 {
			iw = 0
		}
		inner = inner.WithMaxWidth(iw)
		if inner.MinWidth > inner.MaxWidth {
			inner.MinWidth = inner.MaxWidth
		}
	}
	if prefH > 0 {
		ih := prefH - pad.Top - pad.Bottom
		if ih < 0 {
			ih = 0
		}
		inner = inner.WithMaxHeight(ih)
		if inner.MinHeight > inner.MaxHeight {
			inner.MinHeight = inner.MaxHeight
		}
	}

	content := core.Size{}
	kids := parent.Children()
	if len(kids) == 1 {
		content = kids[0].Layout(inner.Expand())
		kids[0].Base().SetOffset(core.Point{X: pad.Left, Y: pad.Top})
	} else if len(kids) > 1 {
		// Stack: all children at padding origin; size = max.
		for _, child := range kids {
			sz := child.Layout(inner.Expand())
			child.Base().SetOffset(core.Point{X: pad.Left, Y: pad.Top})
			content = core.MaxSize(content, sz)
		}
	}

	w := content.Width + pad.Left + pad.Right
	h := content.Height + pad.Top + pad.Bottom
	if prefW > 0 {
		w = prefW
	}
	if prefH > 0 {
		h = prefH
	}
	out := c.Tighten(core.Size{Width: w, Height: h})
	parent.SetSize(out)
	return out
}
