package theme

// Color is an sRGB color in 0..1 (matches painting fill convention).
type Color struct {
	R, G, B, A float64
}

// Hex parses #rgb / #rrggbb into Color (alpha 1). Invalid input returns zero.
func Hex(s string) Color {
	hex := s
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) == 3 {
		hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
	}
	if len(hex) != 6 {
		return Color{}
	}
	atoi := func(c byte) float64 {
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return -1
		}
		return float64(v)
	}
	rh, gh, bh := -1.0, -1.0, -1.0
	if a, b := atoi(hex[0]), atoi(hex[1]); a >= 0 && b >= 0 {
		rh = a*16 + b
	}
	if a, b := atoi(hex[2]), atoi(hex[3]); a >= 0 && b >= 0 {
		gh = a*16 + b
	}
	if a, b := atoi(hex[4]), atoi(hex[5]); a >= 0 && b >= 0 {
		bh = a*16 + b
	}
	if rh < 0 || gh < 0 || bh < 0 {
		return Color{}
	}
	return Color{R: rh / 255, G: gh / 255, B: bh / 255, A: 1}
}

// RGBA builds a Color from 0..255 channels and 0..1 alpha.
func RGBA(r, g, b float64, a float64) Color {
	return Color{R: r / 255, G: g / 255, B: b / 255, A: a}
}

// Tokens is the global seed token set, aligned to antd v6.5.1 default theme.
//
// Old short names (Primary/Surface/...) stay for compat and now map to the
// Ant values below. New Seed_* / Alias_* fields carry the full global seed
// so L2 assertions have numbers to check. Component-specific tokens land
// with each ui/kit/<name>/ package, not here.
type Tokens struct {
	// Compat shorthand (Ant default light).
	Primary   Color
	OnPrimary Color
	Surface   Color
	OnSurface Color
	Border    Color
	Danger    Color

	FontSize   float64 // logical px
	FontSizeSM float64
	FontSizeLG float64

	// Spacing unit (logical px); Space(n) = n * Spacing.
	// Equals Ant sizeXS (8).
	Spacing float64
	Radius  float64

	// Seed colors.
	ColorPrimary Color
	ColorSuccess Color
	ColorWarning Color
	ColorError   Color
	ColorInfo    Color
	ColorLink    Color
	ColorWhite   Color

	// Text hierarchy (black base with alpha).
	ColorText            Color
	ColorTextSecondary   Color
	ColorTextTertiary    Color
	ColorTextQuaternary  Color
	ColorTextPlaceholder Color
	ColorTextDisabled    Color
	ColorTextHeading     Color

	// Backgrounds.
	ColorBgBase      Color
	ColorBgLayout    Color
	ColorBgContainer Color
	ColorBgElevated  Color
	ColorBgSpotlight Color
	ColorBgMask      Color

	// Borders and fills.
	ColorBorder          Color
	ColorBorderSecondary Color
	ColorFill            Color
	ColorFillSecondary   Color
	ColorFillTertiary    Color
	ColorFillQuaternary  Color

	// Type scale (base 14).
	FontSizeXL   float64
	LineHeight   float64
	LineHeightSM float64
	LineHeightLG float64
	FontHeight   float64
	FontHeightSM float64
	FontHeightLG float64

	// Spacing scale (sizeUnit 4, sizeStep 4).
	SizeXXL float64
	SizeXL  float64
	SizeLG  float64
	SizeMD  float64
	SizeMS  float64
	Size    float64
	SizeSM  float64
	SizeXS  float64
	SizeXXS float64

	// Control heights (base 32).
	ControlHeight   float64
	ControlHeightSM float64
	ControlHeightXS float64
	ControlHeightLG float64

	// Radius scale (base 6).
	RadiusXS    float64
	RadiusSM    float64
	RadiusLG    float64
	RadiusOuter float64

	// Padding / margin (from size scale).
	PaddingXXS float64
	PaddingXS  float64
	PaddingSM  float64
	Padding    float64
	PaddingMD  float64
	PaddingLG  float64
	PaddingXL  float64
	MarginXXS  float64
	MarginXS   float64
	MarginSM   float64
	Margin     float64
	MarginMD   float64
	MarginLG   float64
	MarginXL   float64
	MarginXXL  float64

	// Lines and interaction.
	LineWidth          float64
	LineWidthBold      float64
	LineWidthFocus     float64
	ControlOutlineWidth float64
	ControlInteractiveSize float64
	FontWeightStrong   float64
	FontSizeIcon       float64
	OpacityLoading     float64

	// Motion, zIndex, arrows (CSS-ish values kept as strings/numbers).
	MotionDurationFast string
	MotionDurationMid  string
	MotionDurationSlow string
	ZIndexBase         float64
	ZIndexPopupBase    float64
	SizePopupArrow     float64
	BoxShadow          string
	BoxShadowSecondary string
	BoxShadowTertiary  string
}

// Space returns n × Spacing.
func (t Tokens) Space(n float64) float64 {
	if t.Spacing <= 0 {
		return n * 8
	}
	return n * t.Spacing
}

// DefaultTokens returns the Ant Design v6.5.1 default (light) seed.
//
// Source: components/theme/themes/seed.ts + shared derivatives
// (font/size/radius/control) + util/alias.ts (padding/margin/shadow).
func DefaultTokens() Tokens {
	primary := Hex("#1677ff")
	return Tokens{
		Primary:    primary,
		OnPrimary:  Hex("#ffffff"),
		Surface:    Hex("#ffffff"),
		OnSurface:  RGBA(0, 0, 0, 0.88),
		Border:     Hex("#d9d9d9"),
		Danger:     Hex("#ff4d4f"),
		FontSize:   14,
		FontSizeSM: 12,
		FontSizeLG: 16,
		Spacing:    8,
		Radius:     6,

		ColorPrimary: primary,
		ColorSuccess: Hex("#52c41a"),
		ColorWarning: Hex("#faad14"),
		ColorError:   Hex("#ff4d4f"),
		ColorInfo:    Hex("#1677ff"),
		ColorLink:    Hex("#1677ff"),
		ColorWhite:   Hex("#ffffff"),

		ColorText:            RGBA(0, 0, 0, 0.88),
		ColorTextSecondary:   RGBA(0, 0, 0, 0.65),
		ColorTextTertiary:    RGBA(0, 0, 0, 0.45),
		ColorTextQuaternary:  RGBA(0, 0, 0, 0.25),
		ColorTextPlaceholder: RGBA(0, 0, 0, 0.25),
		ColorTextDisabled:    RGBA(0, 0, 0, 0.25),
		ColorTextHeading:     RGBA(0, 0, 0, 0.88),

		ColorBgBase:      Hex("#ffffff"),
		ColorBgLayout:    Hex("#f5f5f5"),
		ColorBgContainer: Hex("#ffffff"),
		ColorBgElevated:  Hex("#ffffff"),
		ColorBgSpotlight: RGBA(0, 0, 0, 0.85),
		ColorBgMask:      RGBA(0, 0, 0, 0.45),

		ColorBorder:          Hex("#d9d9d9"),
		ColorBorderSecondary: Hex("#f0f0f0"),
		ColorFill:            RGBA(0, 0, 0, 0.15),
		ColorFillSecondary:   RGBA(0, 0, 0, 0.06),
		ColorFillTertiary:    RGBA(0, 0, 0, 0.04),
		ColorFillQuaternary:  RGBA(0, 0, 0, 0.02),

		FontSizeXL:   20,
		LineHeight:   1.5714285714285714,
		LineHeightSM: 1.6666666666666667,
		LineHeightLG: 1.5,
		FontHeight:   22,
		FontHeightSM: 20,
		FontHeightLG: 24,

		SizeXXL: 48,
		SizeXL:  32,
		SizeLG:  24,
		SizeMD:  20,
		SizeMS:  16,
		Size:    16,
		SizeSM:  12,
		SizeXS:  8,
		SizeXXS: 4,

		ControlHeight:   32,
		ControlHeightSM: 24,
		ControlHeightXS: 16,
		ControlHeightLG: 40,

		RadiusXS:    2,
		RadiusSM:    4,
		RadiusLG:    8,
		RadiusOuter: 4,

		PaddingXXS: 4,
		PaddingXS:  8,
		PaddingSM:  12,
		Padding:    16,
		PaddingMD:  20,
		PaddingLG:  24,
		PaddingXL:  32,
		MarginXXS:  4,
		MarginXS:   8,
		MarginSM:   12,
		Margin:     16,
		MarginMD:   20,
		MarginLG:   24,
		MarginXL:   32,
		MarginXXL:  48,

		LineWidth:            1,
		LineWidthBold:        2,
		LineWidthFocus:       3,
		ControlOutlineWidth:  2,
		ControlInteractiveSize: 16,
		FontWeightStrong:     600,
		FontSizeIcon:         12,
		OpacityLoading:       0.65,

		MotionDurationFast: "0.1s",
		MotionDurationMid:  "0.2s",
		MotionDurationSlow: "0.3s",
		ZIndexBase:         0,
		ZIndexPopupBase:    1000,
		SizePopupArrow:     16,
		BoxShadow:          "0 6px 16px 0 rgba(0, 0, 0, 0.08), 0 3px 6px -4px rgba(0, 0, 0, 0.12), 0 9px 28px 8px rgba(0, 0, 0, 0.05)",
		BoxShadowSecondary: "0 6px 16px 0 rgba(0, 0, 0, 0.08), 0 3px 6px -4px rgba(0, 0, 0, 0.12), 0 9px 28px 8px rgba(0, 0, 0, 0.05)",
		BoxShadowTertiary:  "0 1px 2px 0 rgba(0, 0, 0, 0.05), 0 1px 6px -1px rgba(0, 0, 0, 0.03), 0 2px 4px 0 rgba(0, 0, 0, 0.03)",
	}
}
