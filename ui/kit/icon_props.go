package kit

// IconProps configures one icon host (WIDGET_MODEL Props segment).
// Value semantics: create, pass to BuildIcon, mutate via Update with a copy.
type IconProps struct {
	Name string
	// Size edge in logical px; 0 means DefaultIconSize (16).
	Size    float64
	SizeSet bool
	// Color single-tone; zero alpha falls back to theme colorText.
	Color    string
	ColorSet bool
	// Rotate static degrees.
	Rotate float64
	// Spin enables ticker rotation.
	Spin bool
	// TwoTone primary or [primary, secondary].
	TwoTonePrimary   string
	TwoToneSecondary string
	TwoToneSet       bool
	HasSecondary     bool
	// Custom painter wins over name (antd component). Stored as a
	// painter key so props stay comparable without func values.
	CustomPainter   string
	CustomPainterID string
	// Variant selects the official theme: outlined|filled|twotone.
	// Empty means outlined (antd default).
	Variant string
	// Decorative defaults true (aria-hidden equivalent).
	Decorative   bool
	DecorSet     bool
	Disabled     bool
	AriaLabel    string
	ClassName    string
	RegistryName string
}

// DefaultIconProps returns antd-aligned defaults: size 0->16,
// spin false, rotate 0, decorative true.
func DefaultIconProps(name string) IconProps {
	return IconProps{Name: name, Decorative: true}
}

// DefaultIconSize is the 16px default edge (icon.md 6.2.1).
const DefaultIconSize = 16.0

// ResolveIconSize returns explicit size else DefaultIconSize.
func ResolveIconSize(p IconProps) float64 {
	if p.SizeSet && p.Size > 0 {
		return p.Size
	}
	if p.Size > 0 {
		return p.Size
	}
	return DefaultIconSize
}
