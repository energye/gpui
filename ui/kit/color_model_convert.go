package kit

import (
	"math"
	"strconv"
	"strings"
)

// HSB→RGB and RGB→HSB follow docs/antd/color-picker.md §6.4, matched
// to @rc-component/color-picker within ±1 per channel.

// ColorModelHSBToRGB converts H(0..360) S/B/A(0..1) to 0..255 ints.
func ColorModelHSBToRGB(h, s, b float64) (r, g, bl int) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	if s < 0 {
		s = 0
	}
	if s > 1 {
		s = 1
	}
	if b < 0 {
		b = 0
	}
	if b > 1 {
		b = 1
	}
	c := b * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := b - c
	var rp, gp, bp float64
	switch int(h / 60) {
	case 0:
		rp, gp, bp = c, x, 0
	case 1:
		rp, gp, bp = x, c, 0
	case 2:
		rp, gp, bp = 0, c, x
	case 3:
		rp, gp, bp = 0, x, c
	case 4:
		rp, gp, bp = x, 0, c
	default:
		rp, gp, bp = c, 0, x
	}
	return int(math.Round((rp + m) * 255)),
		int(math.Round((gp + m) * 255)),
		int(math.Round((bp + m) * 255))
}

// ColorModelRGBToHSB converts 0..255 ints to H(0..360) S/B(0..1).
func ColorModelRGBToHSB(r, g, bl int) (h, s, b float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(bl)/255
	mx := math.Max(rf, math.Max(gf, bf))
	mn := math.Min(rf, math.Min(gf, bf))
	c := mx - mn
	b = mx
	if mx == 0 {
		s = 0
	} else {
		s = c / mx
	}
	if c == 0 {
		return 0, s, b
	}
	switch mx {
	case rf:
		h = 60 * math.Mod((gf-bf)/c, 6)
	case gf:
		h = 60 * ((bf-rf)/c + 2)
	default:
		h = 60 * ((rf-gf)/c + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, s, b
}

// NewColorModelColor builds a color from HSB; alpha defaults to 1
// unless given.
func NewColorModelColor(h, s, b float64, a ...float64) ColorModelColor {
	alpha := 1.0
	if len(a) > 0 {
		alpha = a[0]
	}
	r, g, bl := ColorModelHSBToRGB(h, s, b)
	return ColorModelColor{H: normHue(h), S: clamp01(s), B: clamp01(b), A: clamp01(alpha), R: r, G: g, BL: bl}
}

// NewColorModelColorRGB builds a color from RGB ints; HSB is derived.
func NewColorModelColorRGB(r, g, bl int, a ...float64) ColorModelColor {
	alpha := 1.0
	if len(a) > 0 {
		alpha = a[0]
	}
	h, s, b := ColorModelRGBToHSB(r, g, bl)
	return ColorModelColor{H: h, S: s, B: b, A: clamp01(alpha), R: r, G: g, BL: bl}
}

func normHue(h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ToHexString renders #rrggbb, or #rrggbbaa when transparent.
func (c ColorModelColor) ToHexString() string {
	if c.A >= 1 {
		return "#" + hex2(c.R) + hex2(c.G) + hex2(c.BL)
	}
	return "#" + hex2(c.R) + hex2(c.G) + hex2(c.BL) + hex2(int(math.Round(c.A*255)))
}

// ToHex renders hex without the leading #.
func (c ColorModelColor) ToHex() string {
	return strings.TrimPrefix(c.ToHexString(), "#")
}

// ToRGB renders r/g/b/a fields.
func (c ColorModelColor) ToRGB() (r, g, bl int, a float64) {
	return c.R, c.G, c.BL, c.A
}

// ToRgbString renders rgb(...) or rgba(...) with trimmed alpha.
func (c ColorModelColor) ToRgbString() string {
	if c.A >= 1 {
		return "rgb(" + strconv.Itoa(c.R) + ", " + strconv.Itoa(c.G) + ", " + strconv.Itoa(c.BL) + ")"
	}
	return "rgba(" + strconv.Itoa(c.R) + ", " + strconv.Itoa(c.G) + ", " + strconv.Itoa(c.BL) + ", " + trimFloat(c.A) + ")"
}

// ToHSB renders h/s/b/a fields.
func (c ColorModelColor) ToHSB() (h, s, b, a float64) {
	return c.H, c.S, c.B, c.A
}

// ToHsbString renders hsb(h, s%, b%).
func (c ColorModelColor) ToHsbString() string {
	return "hsb(" + strconv.Itoa(int(math.Round(c.H))) + ", " +
		strconv.Itoa(int(math.Round(c.S*100))) + "%, " +
		strconv.Itoa(int(math.Round(c.B*100))) + "%)"
}

// ToCssString renders the CSS form: rgb string for a single color.
func (c ColorModelColor) ToCssString() string {
	return c.ToRgbString()
}

func hex2(v int) string {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	const digits = "0123456789abcdef"
	return string([]byte{digits[v>>4], digits[v&15]})
}

func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 3, 64)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// ParseColorModelColor parses #rrggbb, #rrggbbaa, rgb()/rgba(),
// hsb(). Unknown input returns false.
func ParseColorModelColor(text string) (ColorModelColor, bool) {
	t := strings.TrimSpace(text)
	lower := strings.ToLower(t)
	switch {
	case strings.HasPrefix(lower, "#") || isHexBare(lower):
		return parseColorModelHex(t)
	case strings.HasPrefix(lower, "rgb"):
		return parseColorModelRGB(t)
	case strings.HasPrefix(lower, "hsb"):
		return parseColorModelHSB(t)
	default:
		return ColorModelColor{}, false
	}
}

func isHexBare(s string) bool {
	if len(s) != 6 && len(s) != 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

// parseColorModelHex strips non-hex chars then takes 6/8 digits,
// aligned to antd toHexFormat.
func parseColorModelHex(text string) (ColorModelColor, bool) {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if isHexDigit(text[i]) {
			b.WriteByte(text[i])
		}
	}
	d := b.String()
	if len(d) < 6 {
		return ColorModelColor{}, false
	}
	if len(d) > 8 {
		d = d[:8]
	}
	if len(d) != 6 && len(d) != 8 {
		d = d[:6]
	}
	rv, _ := strconv.ParseUint(d[0:2], 16, 8)
	gv, _ := strconv.ParseUint(d[2:4], 16, 8)
	bv, _ := strconv.ParseUint(d[4:6], 16, 8)
	a := 1.0
	if len(d) == 8 {
		av, _ := strconv.ParseUint(d[6:8], 16, 8)
		a = float64(av) / 255
	}
	return NewColorModelColorRGB(int(rv), int(gv), int(bv), a), true
}

func parseColorModelNums(inner string, want int) ([]float64, bool) {
	parts := strings.Split(inner, ",")
	if len(parts) != want {
		return nil, false
	}
	out := make([]float64, want)
	for i, p := range parts {
		p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), "%"))
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

func parseColorModelRGB(text string) (ColorModelColor, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	open := strings.IndexByte(lower, '(')
	close := strings.LastIndexByte(lower, ')')
	if open < 0 || close < 0 || close <= open {
		return ColorModelColor{}, false
	}
	nums, ok := parseColorModelNums(lower[open+1:close], 3)
	if !ok {
		if nums3, ok3 := parseColorModelNums(lower[open+1:close], 4); ok3 {
			nums = nums3
		} else {
			return ColorModelColor{}, false
		}
	}
	r, g, bl := int(math.Round(nums[0])), int(math.Round(nums[1])), int(math.Round(nums[2]))
	if r < 0 || r > 255 || g < 0 || g > 255 || bl < 0 || bl > 255 {
		return ColorModelColor{}, false
	}
	a := 1.0
	if len(nums) == 4 {
		a = nums[3]
		if a < 0 || a > 1 {
			return ColorModelColor{}, false
		}
	}
	return NewColorModelColorRGB(r, g, bl, a), true
}

func parseColorModelHSB(text string) (ColorModelColor, bool) {
	lower := strings.ToLower(strings.TrimSpace(text))
	open := strings.IndexByte(lower, '(')
	close := strings.LastIndexByte(lower, ')')
	if open < 0 || close < 0 || close <= open {
		return ColorModelColor{}, false
	}
	nums, ok := parseColorModelNums(lower[open+1:close], 3)
	if !ok {
		return ColorModelColor{}, false
	}
	return NewColorModelColor(nums[0], nums[1]/100, nums[2]/100), true
}

// NormalizeColorModelStops clamps percents to 0..100 and sorts ascending.
func NormalizeColorModelStops(stops []ColorModelStop) []ColorModelStop {
	out := append([]ColorModelStop(nil), stops...)
	for i := range out {
		if out[i].Percent < 0 {
			out[i].Percent = 0
		}
		if out[i].Percent > 100 {
			out[i].Percent = 100
		}
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Percent < out[j-1].Percent; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// GradientColorModelCSS renders linear-gradient(90deg, c1 p1%, ...).
func GradientColorModelCSS(stops []ColorModelStop) string {
	parts := make([]string, 0, len(stops))
	for _, s := range stops {
		parts = append(parts, s.Color.ToRgbString()+" "+trimFloat(s.Percent)+"%")
	}
	return "linear-gradient(90deg, " + strings.Join(parts, ", ") + ")"
}

// ToCssString renders the value: gradient css or single rgb string.
// Cleared renders the transparent single form.
func (v ColorModelValue) ToCssString() string {
	if v.Gradient && len(v.Stops) > 0 && !v.Cleared {
		return GradientColorModelCSS(v.Stops)
	}
	return v.Single.ToCssString()
}

// IsGradient reports a live gradient value.
func (v ColorModelValue) IsGradient() bool {
	return v.Gradient && !v.Cleared && len(v.Stops) > 0
}

// IsEmpty reports the cleared state.
func (v ColorModelValue) IsEmpty() bool {
	return v.Cleared
}

// Equals compares by hex strings (single) or stop-wise (gradient).
func (v ColorModelValue) Equals(o ColorModelValue) bool {
	if v.Cleared != o.Cleared || v.Gradient != o.Gradient {
		return false
	}
	if !v.Gradient {
		return v.Single.ToHexString() == o.Single.ToHexString()
	}
	if len(v.Stops) != len(o.Stops) {
		return false
	}
	for i := range v.Stops {
		if v.Stops[i].Percent != o.Stops[i].Percent ||
			v.Stops[i].Color.ToHexString() != o.Stops[i].Color.ToHexString() {
			return false
		}
	}
	return true
}
