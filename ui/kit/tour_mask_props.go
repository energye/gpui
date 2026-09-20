package kit

// TourMaskType selects the panel skin.
type TourMaskType string

const (
	TourMaskTypeDefault TourMaskType = "default"
	TourMaskTypePrimary TourMaskType = "primary"
)

// TourMaskPlacement selects the panel side relative to the target.
// Center (or empty target) means viewport centered.
type TourMaskPlacement string

const (
	TourMaskPlacementCenter      TourMaskPlacement = "center"
	TourMaskPlacementLeft        TourMaskPlacement = "left"
	TourMaskPlacementLeftTop     TourMaskPlacement = "leftTop"
	TourMaskPlacementLeftBottom  TourMaskPlacement = "leftBottom"
	TourMaskPlacementRight       TourMaskPlacement = "right"
	TourMaskPlacementRightTop    TourMaskPlacement = "rightTop"
	TourMaskPlacementRightBottom TourMaskPlacement = "rightBottom"
	TourMaskPlacementTop         TourMaskPlacement = "top"
	TourMaskPlacementTopLeft     TourMaskPlacement = "topLeft"
	TourMaskPlacementTopRight    TourMaskPlacement = "topRight"
	TourMaskPlacementBottom      TourMaskPlacement = "bottom"
	TourMaskPlacementBottomLeft  TourMaskPlacement = "bottomLeft"
	TourMaskPlacementBottomRight TourMaskPlacement = "bottomRight"
)

// TourMaskGap controls the highlight hole outset and corner radius.
type TourMaskGap struct {
	OffsetX float64
	OffsetY float64
	Radius  float64
}

// DefaultTourMaskGap returns offset 6 radius 2 per antd 6.5.1.
func DefaultTourMaskGap() TourMaskGap {
	return TourMaskGap{OffsetX: 6, OffsetY: 6, Radius: 2}
}

// TourMaskRect is a host-written absolute target rectangle.
// Empty (W<=0 or H<=0) means no target: panel centers in viewport.
type TourMaskRect struct {
	X, Y, W, H float64
}

// IsEmpty reports whether the rect carries no target.
func (r TourMaskRect) IsEmpty() bool {
	return r.W <= 0 || r.H <= 0
}

// TourMaskStep is one guided card.
type TourMaskStep struct {
	Title           string
	Description     string
	CoverSet        bool
	Target          TourMaskRect
	TargetSet       bool
	Placement       TourMaskPlacement
	PlacementSet    bool
	Type            TourMaskType
	TypeSet         bool
	Mask            bool
	MaskSet         bool
	Arrow           bool
	ArrowSet        bool
	NextText        string
	PrevText        string
	StyleSectionSet bool
	StyleSectionR   float64
	StyleSectionG   float64
	StyleSectionB   float64
}

// TourMaskProps configures the guided tour host.
type TourMaskProps struct {
	Open                bool
	Current             int
	CurrentSet          bool
	DefaultCurrent      int
	Type                TourMaskType
	Placement           TourMaskPlacement
	PlacementSet        bool
	Mask                bool
	MaskSet             bool
	MaskColorSet        bool
	MaskColorR          float64
	MaskColorG          float64
	MaskColorB          float64
	MaskColorA          float64
	Gap                 TourMaskGap
	GapSet              bool
	Arrow               bool
	ArrowSet            bool
	Keyboard            bool
	KeyboardSet         bool
	CloseIcon           bool
	CloseIconSet        bool
	DisabledInteraction bool
	ZIndex              int
	ZIndexSet           bool
	ScrollIntoView      bool
	ScrollIntoViewSet   bool
}

// DefaultTourMaskProps returns antd 6.5.1 aligned defaults.
func DefaultTourMaskProps() TourMaskProps {
	return TourMaskProps{
		Open:                false,
		Current:             0,
		CurrentSet:          false,
		DefaultCurrent:      0,
		Type:                TourMaskTypeDefault,
		Placement:           TourMaskPlacementBottom,
		Mask:                true,
		MaskSet:             false,
		Gap:                 DefaultTourMaskGap(),
		GapSet:              false,
		Arrow:               true,
		ArrowSet:            false,
		Keyboard:            true,
		KeyboardSet:         false,
		CloseIcon:           true,
		CloseIconSet:        false,
		DisabledInteraction: false,
		ZIndex:              1001,
		ZIndexSet:           false,
		ScrollIntoView:      true,
		ScrollIntoViewSet:   false,
	}
}

// ResolveTourMaskStepButtons fills Prev/Next/Finish copy from locale.
func ResolveTourMaskStepButtons(prev, next string, last bool, locale string) (prevOut, nextOut string) {
	prevOut, nextOut = prev, next
	zh := len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z')
	if prevOut == "" {
		if zh {
			prevOut = "上一步"
		} else {
			prevOut = "Previous"
		}
	}
	if nextOut == "" {
		if last {
			if zh {
				nextOut = "完成"
			} else {
				nextOut = "Finish"
			}
		} else if zh {
			nextOut = "下一步"
		} else {
			nextOut = "Next"
		}
	}
	return prevOut, nextOut
}

// ResolveTourMaskIndicatorText fills the custom indicator copy.
func ResolveTourMaskIndicatorText(current, total int, locale string) string {
	zh := len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z')
	if zh {
		return itoaTour(uint64(current+1)) + " / " + itoaTour(uint64(total))
	}
	return itoaTour(uint64(current+1)) + " / " + itoaTour(uint64(total))
}

func itoaTour(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
