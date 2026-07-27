package theme

// Color is an sRGB color in 0..1 (matches painting fill convention).
type Color struct {
	R, G, B, A float64
}

// Tokens is the minimal design-token set for L2 shells and early kit work.
type Tokens struct {
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
	Spacing float64
	Radius  float64
}

// Space returns n × Spacing.
func (t Tokens) Space(n float64) float64 {
	if t.Spacing <= 0 {
		return n * 8
	}
	return n * t.Spacing
}

// DefaultTokens returns a dark-ish neutral baseline (not Ant-accurate).
func DefaultTokens() Tokens {
	return Tokens{
		Primary:    Color{R: 0.13, G: 0.47, B: 0.90, A: 1},
		OnPrimary:  Color{R: 1, G: 1, B: 1, A: 1},
		Surface:    Color{R: 0.10, G: 0.12, B: 0.16, A: 1},
		OnSurface:  Color{R: 0.90, G: 0.91, B: 0.93, A: 1},
		Border:     Color{R: 0.28, G: 0.30, B: 0.35, A: 1},
		Danger:     Color{R: 0.86, G: 0.22, B: 0.22, A: 1},
		FontSize:   14,
		FontSizeSM: 12,
		FontSizeLG: 16,
		Spacing:    8,
		Radius:     6,
	}
}
