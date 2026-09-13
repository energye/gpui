package overlay

import "github.com/energye/gpui/ui/rendering"

// Placement is one of the twelve Ant anchor positions.
//
// Aligned to antd _util/placements.ts PlacementAlignMap:
// top/bottom/left/right + corner variants. Corner names read from the
// anchor side first (topLeft = above the anchor, hugging its left edge).
type Placement string

const (
	Top         Placement = "top"
	TopLeft     Placement = "topLeft"
	TopRight    Placement = "topRight"
	Bottom      Placement = "bottom"
	BottomLeft  Placement = "bottomLeft"
	BottomRight Placement = "bottomRight"
	Left        Placement = "left"
	LeftTop     Placement = "leftTop"
	LeftBottom  Placement = "leftBottom"
	Right       Placement = "right"
	RightTop    Placement = "rightTop"
	RightBottom Placement = "rightBottom"
)

// AllPlacements lists the twelve positions in stable order.
var AllPlacements = []Placement{
	Top, TopLeft, TopRight, Bottom, BottomLeft, BottomRight,
	Left, LeftTop, LeftBottom, Right, RightTop, RightBottom,
}

// ResolveOptions tunes Resolve. Zero values select Ant defaults.
type ResolveOptions struct {
	// Gap is the main-axis distance between anchor and overlay.
	// <=0 selects 12 (half arrow 8 + marginXXS 4, tooltip default).
	Gap float64
	// ArrowWidth is the arrow box edge; <=0 selects 16 (sizePopupArrow).
	ArrowWidth float64
	// ArrowOffsetH/V clamp the arrow away from overlay corners.
	// <=0 selects 12 horizontal / 8 vertical (placementArrow defaults).
	ArrowOffsetH float64
	ArrowOffsetV float64
	// Flip enables edge flipping to the opposite side on main-axis overflow.
	Flip bool
	// Shift enables cross-axis clamping into the viewport.
	Shift bool
	// ViewportW/H is the window client size in logical px.
	ViewportW, ViewportH float64
	// ArrowPointAtCenter aims corner arrows at the anchor center
	// (antd arrow.pointAtCenter); default pins them to the fixed offset.
	ArrowPointAtCenter bool
}

func (o *ResolveOptions) withDefaults() ResolveOptions {
	out := ResolveOptions{}
	if o != nil {
		out = *o
	}
	if out.Gap <= 0 {
		out.Gap = 12
	}
	if out.ArrowWidth <= 0 {
		out.ArrowWidth = 16
	}
	if out.ArrowOffsetH <= 0 {
		out.ArrowOffsetH = 12
	}
	if out.ArrowOffsetV <= 0 {
		out.ArrowOffsetV = 8
	}
	// Flip/Shift default on when options are fully zero (common call).
	if o == nil || (*o == ResolveOptions{}) {
		out.Flip = true
		out.Shift = true
	}
	return out
}

// Resolved is the overlay origin plus arrow geometry in overlay coordinates.
type Resolved struct {
	X, Y   float64
	Actual  Placement
	Flipped bool
	// Arrow center within the overlay box (for drawing the caret).
	ArrowX, ArrowY float64
}

// FlipPlacement returns the opposite side used on main-axis overflow.
func FlipPlacement(p Placement) Placement {
	switch p {
	case Top:
		return Bottom
	case Bottom:
		return Top
	case TopLeft:
		return BottomLeft
	case BottomLeft:
		return TopLeft
	case TopRight:
		return BottomRight
	case BottomRight:
		return TopRight
	case Left:
		return Right
	case Right:
		return Left
	case LeftTop:
		return RightTop
	case RightTop:
		return LeftTop
	case LeftBottom:
		return RightBottom
	case RightBottom:
		return LeftBottom
	default:
		return p
	}
}

// ideal returns the unclamped origin for want.
func ideal(anchor rendering.Rect, ow, oh float64, want Placement, gap float64) (x, y float64) {
	ax, ay := anchor.Min.X, anchor.Min.Y
	aw := anchor.Max.X - anchor.Min.X
	ah := anchor.Max.Y - anchor.Min.Y
	switch want {
	case Top:
		return ax + (aw-ow)/2, ay - oh - gap
	case Bottom:
		return ax + (aw-ow)/2, ay + ah + gap
	case TopLeft:
		return ax, ay - oh - gap
	case TopRight:
		return ax + aw - ow, ay - oh - gap
	case BottomLeft:
		return ax, ay + ah + gap
	case BottomRight:
		return ax + aw - ow, ay + ah + gap
	case Left:
		return ax - ow - gap, ay + (ah-oh)/2
	case Right:
		return ax + aw + gap, ay + (ah-oh)/2
	case LeftTop:
		return ax - ow - gap, ay
	case LeftBottom:
		return ax - ow - gap, ay + ah - oh
	case RightTop:
		return ax + aw + gap, ay
	case RightBottom:
		return ax + aw + gap, ay + ah - oh
	default:
		return ax + (aw-ow)/2, ay + ah + gap
	}
}

// mainOverflow measures how far r sticks out of [0,vw]×[0,vh] on the
// placement's main axis only (the axis flip can fix).
func mainOverflow(x, y, ow, oh float64, p Placement, vw, vh float64) float64 {
	var o float64
	switch p {
	case Top:
		if y < 0 {
			o += -y
		}
	case Bottom:
		if y+oh > vh {
			o += y + oh - vh
		}
	case TopLeft, TopRight:
		if y < 0 {
			o += -y
		}
	case BottomLeft, BottomRight:
		if y+oh > vh {
			o += y + oh - vh
		}
	case Left, LeftTop, LeftBottom:
		if x < 0 {
			o += -x
		}
	case Right, RightTop, RightBottom:
		if x+ow > vw {
			o += x + ow - vw
		}
	}
	return o
}

func clamp(v, lo, hi float64) float64 {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// arrow computes the caret center in overlay coordinates.
func arrow(anchor rendering.Rect, x, y, ow, oh float64, p Placement, offH, offV float64, pointAtCenter bool) (ax, ay float64) {
	aw := anchor.Max.X - anchor.Min.X
	ah := anchor.Max.Y - anchor.Min.Y
	cx := anchor.Min.X + aw/2
	cy := anchor.Min.Y + ah/2
	switch p {
	case Top:
		return clamp(cx-x, offH, ow-offH), oh
	case Bottom:
		return clamp(cx-x, offH, ow-offH), 0
	case TopLeft:
		if pointAtCenter {
			return clamp(cx-x, offH, ow-offH), oh
		}
		return clamp(offH, offH, ow-offH), oh
	case TopRight:
		if pointAtCenter {
			return clamp(cx-x, offH, ow-offH), oh
		}
		return clamp(ow-offH, offH, ow-offH), oh
	case BottomLeft:
		if pointAtCenter {
			return clamp(cx-x, offH, ow-offH), 0
		}
		return clamp(offH, offH, ow-offH), 0
	case BottomRight:
		if pointAtCenter {
			return clamp(cx-x, offH, ow-offH), 0
		}
		return clamp(ow-offH, offH, ow-offH), 0
	case Left:
		return ow, clamp(cy-y, offV, oh-offV)
	case Right:
		return 0, clamp(cy-y, offV, oh-offV)
	case LeftTop:
		if pointAtCenter {
			return ow, clamp(cy-y, offV, oh-offV)
		}
		return ow, clamp(offV, offV, oh-offV)
	case LeftBottom:
		if pointAtCenter {
			return ow, clamp(cy-y, offV, oh-offV)
		}
		return ow, clamp(oh-offV, offV, oh-offV)
	case RightTop:
		if pointAtCenter {
			return 0, clamp(cy-y, offV, oh-offV)
		}
		return 0, clamp(offV, offV, oh-offV)
	case RightBottom:
		if pointAtCenter {
			return 0, clamp(cy-y, offV, oh-offV)
		}
		return 0, clamp(oh-offV, offV, oh-offV)
	default:
		return clamp(cx-x, offH, ow-offH), 0
	}
}

// Resolve places an ow×oh overlay relative to anchor in window coordinates.
//
// Steps mirror antd: ideal per-placement origin, main-axis flip on overflow,
// cross-axis shift into the viewport, then arrow-follow clamping.
func Resolve(anchor rendering.Rect, ow, oh float64, want Placement, opt *ResolveOptions) Resolved {
	o := opt.withDefaults()
	if ow < 0 {
		ow = 0
	}
	if oh < 0 {
		oh = 0
	}
	x, y := ideal(anchor, ow, oh, want, o.Gap)
	actual := want
	flipped := false
	if o.Flip && o.ViewportW > 0 && o.ViewportH > 0 {
		cur := mainOverflow(x, y, ow, oh, want, o.ViewportW, o.ViewportH)
		if cur > 0 {
			fx, fy := ideal(anchor, ow, oh, FlipPlacement(want), o.Gap)
			if alt := mainOverflow(fx, fy, ow, oh, FlipPlacement(want), o.ViewportW, o.ViewportH); alt < cur {
				x, y = fx, fy
				actual = FlipPlacement(want)
				flipped = true
			}
		}
	}
	if o.Shift && o.ViewportW > 0 && o.ViewportH > 0 {
		// Cross-axis clamp keeps the whole box on screen when it fits;
		// oversized overlays pin to the origin.
		if ow < o.ViewportW {
			x = clamp(x, 0, o.ViewportW-ow)
		} else {
			x = 0
		}
		if oh < o.ViewportH {
			y = clamp(y, 0, o.ViewportH-oh)
		} else {
			y = 0
		}
	}
	ax, ay := arrow(anchor, x, y, ow, oh, actual, o.ArrowOffsetH, o.ArrowOffsetV, o.ArrowPointAtCenter)
	return Resolved{X: x, Y: y, Actual: actual, Flipped: flipped, ArrowX: ax, ArrowY: ay}
}
