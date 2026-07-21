package kit

// ButtonType is the Ant-like visual type.
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
