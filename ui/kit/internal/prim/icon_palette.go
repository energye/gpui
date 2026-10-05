//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package prim

import (
	"fmt"
	"math"
	"strings"

	"github.com/energye/gpui/ui/theme"
)

// Two-tone secondary derivation, 1:1 with the official chain:
//
//	@ant-design/icons getSecondaryColor(primary) =
//	  @ant-design/colors generate(primary)[0]  (lightest of 10, opaque)
//
// Sources (fetched 2026-10-05):
//   icons@6.0.0 lib/utils.js getSecondaryColor/normalizeTwoToneColors,
//   IconBase.js defaults (primary '#333', secondary '#E6E6E6'),
//   colors@7.2.1 lib/generate.js (hueStep 2, sat/bright steps below).
// Never derive secondary as translucent primary over background.

// AntdPalette generates the 10-step official palette for a base hex.
// Index 5 is the base itself; index 0 is the lightest (two-tone secondary).
func AntdPalette(baseHex string) []string {
	c := theme.Hex(baseHex)
	if c.A <= 0 && strings.TrimSpace(baseHex) != "" {
		// theme.Hex yields zero color on invalid input; refuse to invent.
		return nil
	}
	h, s, v := rgbToHsv(colorToRGB(c))
	var out []string
	for i := 5; i > 0; i-- {
		out = append(out, hsvToHex(paletteHSV(h, s, v, i, true)))
	}
	out = append(out, hsvToHex(h, s, v))
	for i := 1; i <= 4; i++ {
		out = append(out, hsvToHex(paletteHSV(h, s, v, i, false)))
	}
	return out
}

// AntdSecondaryForMain returns generate(main)[0] (opaque). Falls back to
// the legacy translucent main only when the base hex cannot be formed.
func AntdSecondaryForMain(main theme.Color) theme.Color {
	hex := fmt.Sprintf("#%02x%02x%02x",
		uint8(math.Round(main.R*255)),
		uint8(math.Round(main.G*255)),
		uint8(math.Round(main.B*255)))
	pal := AntdPalette(hex)
	if len(pal) != 10 {
		return theme.Color{R: main.R, G: main.G, B: main.B, A: 0.15}
	}
	c := theme.Hex(pal[0])
	c.A = 1
	return c
}

func colorToRGB(c theme.Color) (r, g, b float64) { return c.R, c.G, c.B }

func rgbToHsv(r, g, b float64) (h, s, v float64) {
	mx := math.Max(r, math.Max(g, b))
	mn := math.Min(r, math.Min(g, b))
	d := mx - mn
	v = mx
	if mx == 0 {
		return 0, 0, v
	}
	s = d / mx
	if d == 0 {
		return 0, s, v
	}
	switch mx {
	case r:
		h = 60 * (math.Mod((g-b)/d, 6))
	case g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

func hsvToHex(h, s, v float64) string {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c
	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return fmt.Sprintf("#%02x%02x%02x",
		uint8(math.Round((r+m)*255)),
		uint8(math.Round((g+m)*255)),
		uint8(math.Round((b+m)*255)))
}

func paletteHSV(h, s, v float64, i int, light bool) (float64, float64, float64) {
	return paletteHue(h, i, light), paletteSat(h, s, i, light), paletteVal(v, i, light)
}

func paletteHue(h float64, i int, light bool) float64 {
	const hueStep = 2.0
	var hue float64
	if math.Round(h) >= 60 && math.Round(h) <= 240 {
		if light {
			hue = math.Round(h) - hueStep*float64(i)
		} else {
			hue = math.Round(h) + hueStep*float64(i)
		}
	} else {
		if light {
			hue = math.Round(h) + hueStep*float64(i)
		} else {
			hue = math.Round(h) - hueStep*float64(i)
		}
	}
	if hue < 0 {
		hue += 360
	} else if hue >= 360 {
		hue -= 360
	}
	return hue
}

func paletteSat(h, s float64, i int, light bool) float64 {
	const saturationStep = 0.16
	const saturationStep2 = 0.05
	const lightColorCount = 5
	const darkColorCount = 4
	if h == 0 && s == 0 {
		return s
	}
	var sat float64
	if light {
		sat = s - saturationStep*float64(i)
	} else if i == darkColorCount {
		sat = s + saturationStep
	} else {
		sat = s + saturationStep2*float64(i)
	}
	if sat > 1 {
		sat = 1
	}
	if light && i == lightColorCount && sat > 0.1 {
		sat = 0.1
	}
	if sat < 0.06 {
		sat = 0.06
	}
	return math.Round(sat*100) / 100
}

func paletteVal(v float64, i int, light bool) float64 {
	const brightnessStep1 = 0.05
	const brightnessStep2 = 0.15
	var val float64
	if light {
		val = v + brightnessStep1*float64(i)
	} else {
		val = v - brightnessStep2*float64(i)
	}
	val = math.Max(0, math.Min(1, val))
	return math.Round(val*100) / 100
}
