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

// SwitchSize is Ant Design Switch size (medium default, small).
// https://ant.design/components/switch — size: 'medium' | 'small'
type SwitchSize int

const (
	SwitchMedium SwitchSize = iota // default ≈ 44×22
	SwitchSmall                    // ≈ 28×16
)

func (s SwitchSize) String() string {
	if s == SwitchSmall {
		return "small"
	}
	return "medium"
}

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

// ---------------------------------------------------------------------------
// Input (docs/antd/input.md §6.10)
// ---------------------------------------------------------------------------

// InputSize is Ant Design Input size (medium → middle).
// https://ant.design/components/input — size: large | medium | small
type InputSize int

const (
	InputMiddle InputSize = iota // default; antd "medium"
	InputSmall
	InputLarge
)

// InputVariant is Ant Design 5.13+ visual variant.
// https://ant.design/components/input — variant
type InputVariant int

const (
	InputOutlined InputVariant = iota // default
	InputFilled
	InputBorderless
	InputUnderlined
)

// InputStatus is validation chrome (error | warning).
type InputStatus int

const (
	InputStatusNone InputStatus = iota
	InputStatusError
	InputStatusWarning
)

// InputType is the single-line input kind (native type subset).
// Use NewTextArea for multi-line — do not use type=textarea.
type InputType int

const (
	InputTypeText InputType = iota
	InputTypePassword
)

// SearchSource identifies what triggered Input.Search onSearch.
type SearchSource int

const (
	SearchFromInput  SearchSource = iota // Enter in field
	SearchFromClear                      // clear icon (antd may also fire onSearch)
	SearchFromButton                     // search icon / enterButton
)

func (s InputSize) String() string {
	switch s {
	case InputSmall:
		return "small"
	case InputLarge:
		return "large"
	default:
		return "middle"
	}
}

func (v InputVariant) String() string {
	switch v {
	case InputFilled:
		return "filled"
	case InputBorderless:
		return "borderless"
	case InputUnderlined:
		return "underlined"
	default:
		return "outlined"
	}
}

func (s InputStatus) String() string {
	switch s {
	case InputStatusError:
		return "error"
	case InputStatusWarning:
		return "warning"
	default:
		return ""
	}
}

func (t InputType) String() string {
	if t == InputTypePassword {
		return "password"
	}
	return "text"
}

func (s SearchSource) String() string {
	switch s {
	case SearchFromClear:
		return "clear"
	case SearchFromButton:
		return "button"
	default:
		return "input"
	}
}

// ToButtonSize maps InputSize → ButtonSize for Space.Compact co-sizing.
func (s InputSize) ToButtonSize() ButtonSize {
	switch s {
	case InputSmall:
		return ButtonSmall
	case InputLarge:
		return ButtonLarge
	default:
		return ButtonMiddle
	}
}

// InputSizeFromButton maps Compact/Button size onto Input.
func InputSizeFromButton(s ButtonSize) InputSize {
	switch s {
	case ButtonSmall:
		return InputSmall
	case ButtonLarge:
		return InputLarge
	default:
		return InputMiddle
	}
}
