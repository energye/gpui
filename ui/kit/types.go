package kit

// ButtonType is the classic Ant type (maps to Variant when Variant is Auto).
// https://ant.design/components/button
type ButtonType int

const (
	ButtonDefault ButtonType = iota
	ButtonPrimary
	ButtonDashed
	ButtonText
	ButtonLink
)

// ButtonSize is the Ant-like size token.
type ButtonSize int

const (
	ButtonMiddle ButtonSize = iota
	ButtonSmall
	ButtonLarge
)

// ButtonVariant is Ant Design 5.21+ visual variant.
// Auto (0) derives from Type for backward compatibility.
// https://ant.design/components/button#components-button-demo-color-variant
type ButtonVariant int

const (
	ButtonVariantAuto ButtonVariant = iota // use Type
	ButtonVariantSolid
	ButtonVariantOutlined
	ButtonVariantDashed
	ButtonVariantFilled
	ButtonVariantText
	ButtonVariantLink
)

// ButtonColor is Ant Design 5.21+ semantic / preset color for variants.
// Default (0) uses Type + Danger; Primary/Danger/Success/Warning map to theme tokens.
type ButtonColor int

const (
	ButtonColorDefault ButtonColor = iota
	ButtonColorPrimary
	ButtonColorDanger
	ButtonColorSuccess
	ButtonColorWarning
)

// ButtonShape is Ant Design button shape.
// https://ant.design/components/button#shape
type ButtonShape int

const (
	ButtonShapeDefault ButtonShape = iota // rectangular with size radius
	ButtonShapeCircle                     // circular; typically icon-only, w=h
	ButtonShapeRound                      // capsule, radius ≈ height/2
)

// ButtonIconPlacement places the icon relative to the label.
type ButtonIconPlacement int

const (
	ButtonIconStart ButtonIconPlacement = iota // leading (default)
	ButtonIconEnd                              // trailing
)

// String helpers for debugging.
func (t ButtonType) String() string {
	switch t {
	case ButtonPrimary:
		return "primary"
	case ButtonDashed:
		return "dashed"
	case ButtonText:
		return "text"
	case ButtonLink:
		return "link"
	default:
		return "default"
	}
}

func (s ButtonSize) String() string {
	switch s {
	case ButtonSmall:
		return "small"
	case ButtonLarge:
		return "large"
	default:
		return "middle"
	}
}

func (v ButtonVariant) String() string {
	switch v {
	case ButtonVariantSolid:
		return "solid"
	case ButtonVariantOutlined:
		return "outlined"
	case ButtonVariantDashed:
		return "dashed"
	case ButtonVariantFilled:
		return "filled"
	case ButtonVariantText:
		return "text"
	case ButtonVariantLink:
		return "link"
	default:
		return "auto"
	}
}

func (c ButtonColor) String() string {
	switch c {
	case ButtonColorPrimary:
		return "primary"
	case ButtonColorDanger:
		return "danger"
	case ButtonColorSuccess:
		return "success"
	case ButtonColorWarning:
		return "warning"
	default:
		return "default"
	}
}

func (p ButtonIconPlacement) String() string {
	if p == ButtonIconEnd {
		return "end"
	}
	return "start"
}

func (s ButtonShape) String() string {
	switch s {
	case ButtonShapeCircle:
		return "circle"
	case ButtonShapeRound:
		return "round"
	default:
		return "default"
	}
}
