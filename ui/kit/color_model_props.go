package kit

// ColorModelFormat selects the display/parse format.
type ColorModelFormat string

const (
	ColorModelFormatHex ColorModelFormat = "hex"
	ColorModelFormatRGB ColorModelFormat = "rgb"
	ColorModelFormatHSB ColorModelFormat = "hsb"
)

// ColorModelMode selects the value shape.
type ColorModelMode string

const (
	ColorModelModeSingle   ColorModelMode = "single"
	ColorModelModeGradient ColorModelMode = "gradient"
)

// ColorModelSize selects the trigger tier.
type ColorModelSize string

const (
	ColorModelSizeSmall  ColorModelSize = "small"
	ColorModelSizeMedium ColorModelSize = "medium"
	ColorModelSizeLarge  ColorModelSize = "large"
)

// ColorModelTrigger selects the popup trigger gesture.
type ColorModelTrigger string

const (
	ColorModelTriggerClick ColorModelTrigger = "click"
	ColorModelTriggerHover ColorModelTrigger = "hover"
)

// ColorModelColor is one opaque-or-transparent color. H is 0..360,
// S/B/A are 0..1, R/G/B are 0..255 ints. Both sides are kept so a
// string parse formats back to the identical string (no precision
// loss); the HSB side is canonical for panel math.
type ColorModelColor struct {
	H, S, B, A float64
	R, G, BL   int
}

// ColorModelStop is one gradient stop.
type ColorModelStop struct {
	Color   ColorModelColor
	Percent float64
}

// ColorModelValue is the AggregationColor equivalent: either a
// single color or an ordered stop list. Cleared means empty.
type ColorModelValue struct {
	Single   ColorModelColor
	Stops    []ColorModelStop
	Gradient bool
	Cleared  bool
}

// ColorModelPreset is one preset group; each entry is a single
// color or a gradient stop list.
type ColorModelPreset struct {
	Label       string
	DefaultOpen bool
	Colors      []ColorModelValue
}

// ColorModelShowTextRender customizes the trigger text (P0 string hook).
type ColorModelShowTextRender func(v ColorModelValue) string

// ColorModelPanelRender customizes the panel wrapper (P0 string hook;
// the full ReactNode visual lands with the F3 component).
type ColorModelPanelRender func(panel string, hasPicker bool, hasPresets bool) string

// ColorModelProps configures one color model host.
type ColorModelProps struct {
	Value           *ColorModelValue
	DefaultValue    ColorModelValue
	DefaultValueSet bool
	Format          ColorModelFormat
	FormatSet       bool
	DefaultFormat   ColorModelFormat
	Mode            ColorModelMode
	ModeSet         bool
	Modes           []ColorModelMode
	Disabled        bool
	DisabledAlpha   bool
	DisabledFormat  bool
	AllowClear      bool
	ShowText        bool
	ShowTextRender  ColorModelShowTextRender
	PanelRender     ColorModelPanelRender
	Presets         []ColorModelPreset
	Size            ColorModelSize
	SizeSet         bool
	Placement       string
	Trigger         ColorModelTrigger
	Open            bool
	OpenSet         bool
	DefaultOpen     bool
	AriaLabel       string
}

// DefaultColorModelProps returns antd 6.5.1 aligned defaults.
func DefaultColorModelProps() ColorModelProps {
	return ColorModelProps{
		Format:        ColorModelFormatHex,
		FormatSet:     false,
		DefaultFormat: ColorModelFormatHex,
		Mode:          ColorModelModeSingle,
		ModeSet:       false,
		Size:          ColorModelSizeMedium,
		SizeSet:       false,
		Placement:     "bottomLeft",
		Trigger:       ColorModelTriggerClick,
	}
}

// DefaultColorModelColor returns opaque blue #1677ff.
func DefaultColorModelColor() ColorModelColor {
	c, _ := ParseColorModelColor("#1677ff")
	return c
}

// ResolveColorModelFormat returns the explicit format else hex.
func ResolveColorModelFormat(props ColorModelProps, current ColorModelFormat) ColorModelFormat {
	if current != "" {
		return current
	}
	if props.FormatSet && props.Format != "" {
		return props.Format
	}
	if props.Format != "" {
		return props.Format
	}
	if props.DefaultFormat != "" {
		return props.DefaultFormat
	}
	return ColorModelFormatHex
}

// ResolveColorModelMode returns the explicit mode else single.
func ResolveColorModelMode(props ColorModelProps, current ColorModelMode) ColorModelMode {
	if current != "" {
		return current
	}
	if props.ModeSet && props.Mode != "" {
		return props.Mode
	}
	if props.Mode != "" {
		return props.Mode
	}
	return ColorModelModeSingle
}

// ResolveColorModelModes returns the switchable mode list.
func ResolveColorModelModes(props ColorModelProps) []ColorModelMode {
	if len(props.Modes) > 0 {
		return props.Modes
	}
	return []ColorModelMode{ResolveColorModelMode(props, "")}
}

// ColorModelModeAllowed reports whether m is switchable.
func ColorModelModeAllowed(props ColorModelProps, m ColorModelMode) bool {
	for _, a := range ResolveColorModelModes(props) {
		if a == m {
			return true
		}
	}
	return false
}
