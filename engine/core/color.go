package core

import (
	"math"

	"github.com/energye/gpui/render"
)

// Color stores straight-alpha color with float64 components in [0, 1].
// It mirrors render.RGBA field for field but stays a separate type so game
// code (tints, lights, LUTs) never depends on render internals. Convert once
// at the render boundary with ToRender/ColorFromRender.
type Color struct {
	R, G, B, A float64
}

// RGB builds an opaque color.
func RGB(r, g, b float64) Color { return Color{R: r, G: g, B: b, A: 1} }

// RGBA builds a color from straight-alpha components.
func RGBA(r, g, b, a float64) Color { return Color{R: r, G: g, B: b, A: a} }

// Common colors.
var (
	Black       = RGB(0, 0, 0)
	White       = RGB(1, 1, 1)
	Red         = RGB(1, 0, 0)
	Green       = RGB(0, 1, 0)
	Blue        = RGB(0, 0, 1)
	Yellow      = RGB(1, 1, 0)
	Cyan        = RGB(0, 1, 1)
	Magenta     = RGB(1, 0, 1)
	Transparent = RGBA(0, 0, 0, 0)
)

func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// Clamped returns c with every component pinned to [0, 1].
func (c Color) Clamped() Color {
	return Color{clamp01(c.R), clamp01(c.G), clamp01(c.B), clamp01(c.A)}
}

// ToBytes returns 8-bit straight-alpha channels (clamped, rounded).
// render truncates instead, so edge values may differ by 1 by design;
// float path ToRender carries the exact value across the boundary.
func (c Color) ToBytes() (r, g, b, a uint8) {
	c = c.Clamped()
	return uint8(math.Round(c.R * 255)),
		uint8(math.Round(c.G * 255)),
		uint8(math.Round(c.B * 255)),
		uint8(math.Round(c.A * 255))
}

// ColorFromBytes builds a Color from 8-bit straight-alpha channels.
func ColorFromBytes(r, g, b, a uint8) Color {
	return Color{
		R: float64(r) / 255,
		G: float64(g) / 255,
		B: float64(b) / 255,
		A: float64(a) / 255,
	}
}

// ToRender converts to render.RGBA at the render boundary.
func (c Color) ToRender() render.RGBA {
	return render.RGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// ColorFromRender converts a render.RGBA back to game units.
func ColorFromRender(rc render.RGBA) Color {
	return Color{R: rc.R, G: rc.G, B: rc.B, A: rc.A}
}

// Lerp interpolates: t=0 returns c, t=1 returns o.
func (c Color) Lerp(o Color, t float64) Color {
	return Color{
		R: c.R + (o.R-c.R)*t,
		G: c.G + (o.G-c.G)*t,
		B: c.B + (o.B-c.B)*t,
		A: c.A + (o.A-c.A)*t,
	}
}

// Premultiplied returns c with RGB scaled by A.
func (c Color) Premultiplied() Color {
	return Color{R: c.R * c.A, G: c.G * c.A, B: c.B * c.A, A: c.A}
}

// Unpremultiplied reverses Premultiplied. Zero alpha stays zero (no NaN).
func (c Color) Unpremultiplied() Color {
	if c.A == 0 {
		return Color{}
	}
	return Color{R: c.R / c.A, G: c.G / c.A, B: c.B / c.A, A: c.A}
}

// ApproxEqual reports whether all four components differ by less than eps.
func (c Color) ApproxEqual(o Color, eps float64) bool {
	return math.Abs(c.R-o.R) < eps && math.Abs(c.G-o.G) < eps &&
		math.Abs(c.B-o.B) < eps && math.Abs(c.A-o.A) < eps
}

func hexVal(ch byte) (uint32, bool) {
	switch {
	case '0' <= ch && ch <= '9':
		return uint32(ch - '0'), true
	case 'a' <= ch && ch <= 'f':
		return uint32(ch-'a') + 10, true
	case 'A' <= ch && ch <= 'F':
		return uint32(ch-'A') + 10, true
	}
	return 0, false
}

// ParseHex parses "RGB", "RGBA", "RRGGBB" or "RRGGBBAA" (with or without
// '#'). Anything else returns a CodeBadData error, never a silent color.
func ParseHex(s string) (Color, error) {
	orig := s
	if s != "" && s[0] == '#' {
		s = s[1:]
	}
	var v [4]uint32
	v[3] = 255
	digits := -1
	switch len(s) {
	case 3, 4:
		digits = 1
	case 6, 8:
		digits = 2
	}
	ok := digits > 0
	var n [4]int
	switch len(s) {
	case 3:
		n = [4]int{0, 1, 2, -1}
	case 4:
		n = [4]int{0, 1, 2, 3}
	case 6:
		n = [4]int{0, 2, 4, -1}
	case 8:
		n = [4]int{0, 2, 4, 6}
	}
	if ok {
		for i, pos := range n {
			if pos < 0 {
				continue
			}
			var d uint32
			for k := 0; k < digits; k++ {
				var vdig uint32
				vdig, ok = hexVal(s[pos+k])
				if !ok {
					break
				}
				d = d*16 + vdig
			}
			if !ok {
				break
			}
			if digits == 1 {
				d = d * 17
			}
			v[i] = d
		}
	}
	if !ok {
		return Color{}, BadData("core.ParseHex", orig)
	}
	return Color{
		R: float64(v[0]) / 255,
		G: float64(v[1]) / 255,
		B: float64(v[2]) / 255,
		A: float64(v[3]) / 255,
	}, nil
}
