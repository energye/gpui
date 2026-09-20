package kit

// Fixed tour geometry aligned to antd 6.5.1 component tokens.
// Painting stays in prim when F0-2 lands; this file only computes rects.
const (
	TourMaskPanelWidthMax = 520.0
	TourMaskPanelH        = 180.0
	TourMaskPanelPad      = 16.0
	TourMaskPanelGap      = 12.0
	TourMaskZDefault      = 1001
)

// TourMaskHoleRect is the highlight hole in logical pixels.
type TourMaskHoleRect struct {
	X, Y, W, H float64
	Radius     float64
}

// TourMaskPanelRect is the guided panel rect in logical pixels.
type TourMaskPanelRect struct {
	X, Y, W, H float64
	ZIndex     int
	Placement  TourMaskPlacement
}

// ResolveTourMaskZ returns explicit zIndex else 1001 above modal 1000.
func ResolveTourMaskZ(props TourMaskProps) int {
	if props.ZIndexSet && props.ZIndex != 0 {
		return props.ZIndex
	}
	if props.ZIndex != 0 {
		return props.ZIndex
	}
	return TourMaskZDefault
}

// ComputeTourMaskHole expands the host-written target by gap offsets.
func ComputeTourMaskHole(target TourMaskRect, gap TourMaskGap) TourMaskHoleRect {
	if target.IsEmpty() {
		return TourMaskHoleRect{}
	}
	ox := gap.OffsetX
	oy := gap.OffsetY
	if ox < 0 {
		ox = 0
	}
	if oy < 0 {
		oy = 0
	}
	return TourMaskHoleRect{
		X:      target.X - ox,
		Y:      target.Y - oy,
		W:      target.W + ox*2,
		H:      target.H + oy*2,
		Radius: gap.Radius,
	}
}

// ComputeTourMaskPanel places the panel by placement around the hole.
// Empty target or center placement centers in the viewport.
func ComputeTourMaskPanel(viewportW, viewportH float64, hole TourMaskHoleRect, target TourMaskRect, placement TourMaskPlacement, zIndex int) TourMaskPanelRect {
	w := TourMaskPanelWidthMax
	if w > viewportW-48 {
		w = viewportW - 48
	}
	if w < 200 {
		w = 200
	}
	h := TourMaskPanelH
	if h > viewportH-48 {
		h = viewportH - 48
	}
	place := placement
	if place == "" {
		place = TourMaskPlacementBottom
	}
	if target.IsEmpty() {
		place = TourMaskPlacementCenter
	}
	var x, y float64
	switch place {
	case TourMaskPlacementCenter:
		x = (viewportW - w) / 2
		y = (viewportH - h) / 2
	case TourMaskPlacementTop, TourMaskPlacementTopLeft, TourMaskPlacementTopRight:
		x = hole.X + hole.W/2 - w/2
		y = hole.Y - h - TourMaskPanelGap
	case TourMaskPlacementBottom, TourMaskPlacementBottomLeft, TourMaskPlacementBottomRight:
		x = hole.X + hole.W/2 - w/2
		y = hole.Y + hole.H + TourMaskPanelGap
	case TourMaskPlacementLeft, TourMaskPlacementLeftTop, TourMaskPlacementLeftBottom:
		x = hole.X - w - TourMaskPanelGap
		y = hole.Y + hole.H/2 - h/2
	case TourMaskPlacementRight, TourMaskPlacementRightTop, TourMaskPlacementRightBottom:
		x = hole.X + hole.W + TourMaskPanelGap
		y = hole.Y + hole.H/2 - h/2
	default:
		x = hole.X + hole.W/2 - w/2
		y = hole.Y + hole.H + TourMaskPanelGap
	}
	if x < 16 {
		x = 16
	}
	if x+w > viewportW-16 {
		x = viewportW - w - 16
	}
	if y < 16 {
		y = 16
	}
	if y+h > viewportH-16 {
		y = viewportH - h - 16
	}
	return TourMaskPanelRect{X: x, Y: y, W: w, H: h, ZIndex: zIndex, Placement: place}
}
