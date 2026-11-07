//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package svg

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
)

const (
	// colorNone is the SVG "none" color value, meaning no fill or stroke.
	colorNone = "none"
)

// namedColors implements the CSS Color Module Level 4 extended named
// color list (148 entries, incl. grey/gray and slategrey/gray aliases).
// This is the full set real-world SVG documents use; the previous table
// held ~22 common entries and silently refused fills like "seagreen".
var namedColors = map[string]color.Color{
	"aliceblue":            color.NRGBA{R: 0xf0, G: 0xf8, B: 0xff, A: 255},
	"antiquewhite":         color.NRGBA{R: 0xfa, G: 0xeb, B: 0xd7, A: 255},
	"aqua":                 color.NRGBA{R: 0x00, G: 0xff, B: 0xff, A: 255},
	"aquamarine":           color.NRGBA{R: 0x7f, G: 0xff, B: 0xd4, A: 255},
	"azure":                color.NRGBA{R: 0xf0, G: 0xff, B: 0xff, A: 255},
	"beige":                color.NRGBA{R: 0xf5, G: 0xf5, B: 0xdc, A: 255},
	"bisque":               color.NRGBA{R: 0xff, G: 0xe4, B: 0xc4, A: 255},
	"black":                color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 255},
	"blanchedalmond":       color.NRGBA{R: 0xff, G: 0xeb, B: 0xcd, A: 255},
	"blue":                 color.NRGBA{R: 0x00, G: 0x00, B: 0xff, A: 255},
	"blueviolet":           color.NRGBA{R: 0x8a, G: 0x2b, B: 0xe2, A: 255},
	"brown":                color.NRGBA{R: 0xa5, G: 0x2a, B: 0x2a, A: 255},
	"burlywood":            color.NRGBA{R: 0xde, G: 0xb8, B: 0x87, A: 255},
	"cadetblue":            color.NRGBA{R: 0x5f, G: 0x9e, B: 0xa0, A: 255},
	"chartreuse":           color.NRGBA{R: 0x7f, G: 0xff, B: 0x00, A: 255},
	"chocolate":            color.NRGBA{R: 0xd2, G: 0x69, B: 0x1e, A: 255},
	"coral":                color.NRGBA{R: 0xff, G: 0x7f, B: 0x50, A: 255},
	"cornflowerblue":       color.NRGBA{R: 0x64, G: 0x95, B: 0xed, A: 255},
	"cornsilk":             color.NRGBA{R: 0xff, G: 0xf8, B: 0xdc, A: 255},
	"crimson":              color.NRGBA{R: 0xdc, G: 0x14, B: 0x3c, A: 255},
	"cyan":                 color.NRGBA{R: 0x00, G: 0xff, B: 0xff, A: 255},
	"darkblue":             color.NRGBA{R: 0x00, G: 0x00, B: 0x8b, A: 255},
	"darkcyan":             color.NRGBA{R: 0x00, G: 0x8b, B: 0x8b, A: 255},
	"darkgoldenrod":        color.NRGBA{R: 0xb8, G: 0x86, B: 0x0b, A: 255},
	"darkgray":             color.NRGBA{R: 0xa9, G: 0xa9, B: 0xa9, A: 255},
	"darkgreen":            color.NRGBA{R: 0x00, G: 0x64, B: 0x00, A: 255},
	"darkgrey":             color.NRGBA{R: 0xa9, G: 0xa9, B: 0xa9, A: 255},
	"darkkhaki":            color.NRGBA{R: 0xbd, G: 0xb7, B: 0x6b, A: 255},
	"darkmagenta":          color.NRGBA{R: 0x8b, G: 0x00, B: 0x8b, A: 255},
	"darkolivegreen":       color.NRGBA{R: 0x55, G: 0x6b, B: 0x2f, A: 255},
	"darkorange":           color.NRGBA{R: 0xff, G: 0x8c, B: 0x00, A: 255},
	"darkorchid":           color.NRGBA{R: 0x99, G: 0x32, B: 0xcc, A: 255},
	"darkred":              color.NRGBA{R: 0x8b, G: 0x00, B: 0x00, A: 255},
	"darksalmon":           color.NRGBA{R: 0xe9, G: 0x96, B: 0x7a, A: 255},
	"darkseagreen":         color.NRGBA{R: 0x8f, G: 0xbc, B: 0x8f, A: 255},
	"darkslateblue":        color.NRGBA{R: 0x48, G: 0x3d, B: 0x8b, A: 255},
	"darkslategray":        color.NRGBA{R: 0x2f, G: 0x4f, B: 0x4f, A: 255},
	"darkslategrey":        color.NRGBA{R: 0x2f, G: 0x4f, B: 0x4f, A: 255},
	"darkturquoise":        color.NRGBA{R: 0x00, G: 0xce, B: 0xd1, A: 255},
	"darkviolet":           color.NRGBA{R: 0x94, G: 0x00, B: 0xd3, A: 255},
	"deeppink":             color.NRGBA{R: 0xff, G: 0x14, B: 0x93, A: 255},
	"deepskyblue":          color.NRGBA{R: 0x00, G: 0xbf, B: 0xff, A: 255},
	"dimgray":              color.NRGBA{R: 0x69, G: 0x69, B: 0x69, A: 255},
	"dimgrey":              color.NRGBA{R: 0x69, G: 0x69, B: 0x69, A: 255},
	"dodgerblue":           color.NRGBA{R: 0x1e, G: 0x90, B: 0xff, A: 255},
	"firebrick":            color.NRGBA{R: 0xb2, G: 0x22, B: 0x22, A: 255},
	"floralwhite":          color.NRGBA{R: 0xff, G: 0xfa, B: 0xf0, A: 255},
	"forestgreen":          color.NRGBA{R: 0x22, G: 0x8b, B: 0x22, A: 255},
	"fuchsia":              color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 255},
	"gainsboro":            color.NRGBA{R: 0xdc, G: 0xdc, B: 0xdc, A: 255},
	"ghostwhite":           color.NRGBA{R: 0xf8, G: 0xf8, B: 0xff, A: 255},
	"gold":                 color.NRGBA{R: 0xff, G: 0xd7, B: 0x00, A: 255},
	"goldenrod":            color.NRGBA{R: 0xda, G: 0xa5, B: 0x20, A: 255},
	"gray":                 color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 255},
	"green":                color.NRGBA{R: 0x00, G: 0x80, B: 0x00, A: 255},
	"greenyellow":          color.NRGBA{R: 0xad, G: 0xff, B: 0x2f, A: 255},
	"grey":                 color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 255},
	"honeydew":             color.NRGBA{R: 0xf0, G: 0xff, B: 0xf0, A: 255},
	"hotpink":              color.NRGBA{R: 0xff, G: 0x69, B: 0xb4, A: 255},
	"indianred":            color.NRGBA{R: 0xcd, G: 0x5c, B: 0x5c, A: 255},
	"indigo":               color.NRGBA{R: 0x4b, G: 0x00, B: 0x82, A: 255},
	"ivory":                color.NRGBA{R: 0xff, G: 0xff, B: 0xf0, A: 255},
	"khaki":                color.NRGBA{R: 0xf0, G: 0xe6, B: 0x8c, A: 255},
	"lavender":             color.NRGBA{R: 0xe6, G: 0xe6, B: 0xfa, A: 255},
	"lavenderblush":        color.NRGBA{R: 0xff, G: 0xf0, B: 0xf5, A: 255},
	"lawngreen":            color.NRGBA{R: 0x7c, G: 0xfc, B: 0x00, A: 255},
	"lemonchiffon":         color.NRGBA{R: 0xff, G: 0xfa, B: 0xcd, A: 255},
	"lightblue":            color.NRGBA{R: 0xad, G: 0xd8, B: 0xe6, A: 255},
	"lightcoral":           color.NRGBA{R: 0xf0, G: 0x80, B: 0x80, A: 255},
	"lightcyan":            color.NRGBA{R: 0xe0, G: 0xff, B: 0xff, A: 255},
	"lightgoldenrodyellow": color.NRGBA{R: 0xfa, G: 0xfa, B: 0xd2, A: 255},
	"lightgray":            color.NRGBA{R: 0xd3, G: 0xd3, B: 0xd3, A: 255},
	"lightgreen":           color.NRGBA{R: 0x90, G: 0xee, B: 0x90, A: 255},
	"lightgrey":            color.NRGBA{R: 0xd3, G: 0xd3, B: 0xd3, A: 255},
	"lightpink":            color.NRGBA{R: 0xff, G: 0xb6, B: 0xc1, A: 255},
	"lightsalmon":          color.NRGBA{R: 0xff, G: 0xa0, B: 0x7a, A: 255},
	"lightseagreen":        color.NRGBA{R: 0x20, G: 0xb2, B: 0xaa, A: 255},
	"lightskyblue":         color.NRGBA{R: 0x87, G: 0xce, B: 0xfa, A: 255},
	"lightslategray":       color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 255},
	"lightslategrey":       color.NRGBA{R: 0x77, G: 0x88, B: 0x99, A: 255},
	"lightsteelblue":       color.NRGBA{R: 0xb0, G: 0xc4, B: 0xde, A: 255},
	"lightyellow":          color.NRGBA{R: 0xff, G: 0xff, B: 0xe0, A: 255},
	"lime":                 color.NRGBA{R: 0x00, G: 0xff, B: 0x00, A: 255},
	"limegreen":            color.NRGBA{R: 0x32, G: 0xcd, B: 0x32, A: 255},
	"linen":                color.NRGBA{R: 0xfa, G: 0xf0, B: 0xe6, A: 255},
	"magenta":              color.NRGBA{R: 0xff, G: 0x00, B: 0xff, A: 255},
	"maroon":               color.NRGBA{R: 0x80, G: 0x00, B: 0x00, A: 255},
	"mediumaquamarine":     color.NRGBA{R: 0x66, G: 0xcd, B: 0xaa, A: 255},
	"mediumblue":           color.NRGBA{R: 0x00, G: 0x00, B: 0xcd, A: 255},
	"mediumorchid":         color.NRGBA{R: 0xba, G: 0x55, B: 0xd3, A: 255},
	"mediumpurple":         color.NRGBA{R: 0x93, G: 0x70, B: 0xdb, A: 255},
	"mediumseagreen":       color.NRGBA{R: 0x3c, G: 0xb3, B: 0x71, A: 255},
	"mediumslateblue":      color.NRGBA{R: 0x7b, G: 0x68, B: 0xee, A: 255},
	"mediumspringgreen":    color.NRGBA{R: 0x00, G: 0xfa, B: 0x9a, A: 255},
	"mediumturquoise":      color.NRGBA{R: 0x48, G: 0xd1, B: 0xcc, A: 255},
	"mediumvioletred":      color.NRGBA{R: 0xc7, G: 0x15, B: 0x85, A: 255},
	"midnightblue":         color.NRGBA{R: 0x19, G: 0x19, B: 0x70, A: 255},
	"mintcream":            color.NRGBA{R: 0xf5, G: 0xff, B: 0xfa, A: 255},
	"mistyrose":            color.NRGBA{R: 0xff, G: 0xe4, B: 0xe1, A: 255},
	"moccasin":             color.NRGBA{R: 0xff, G: 0xe4, B: 0xb5, A: 255},
	"navajowhite":          color.NRGBA{R: 0xff, G: 0xde, B: 0xad, A: 255},
	"navy":                 color.NRGBA{R: 0x00, G: 0x00, B: 0x80, A: 255},
	"oldlace":              color.NRGBA{R: 0xfd, G: 0xf5, B: 0xe6, A: 255},
	"olive":                color.NRGBA{R: 0x80, G: 0x80, B: 0x00, A: 255},
	"olivedrab":            color.NRGBA{R: 0x6b, G: 0x8e, B: 0x23, A: 255},
	"orange":               color.NRGBA{R: 0xff, G: 0xa5, B: 0x00, A: 255},
	"orangered":            color.NRGBA{R: 0xff, G: 0x45, B: 0x00, A: 255},
	"orchid":               color.NRGBA{R: 0xda, G: 0x70, B: 0xd6, A: 255},
	"palegoldenrod":        color.NRGBA{R: 0xee, G: 0xe8, B: 0xaa, A: 255},
	"palegreen":            color.NRGBA{R: 0x98, G: 0xfb, B: 0x98, A: 255},
	"paleturquoise":        color.NRGBA{R: 0xaf, G: 0xee, B: 0xee, A: 255},
	"palevioletred":        color.NRGBA{R: 0xdb, G: 0x70, B: 0x93, A: 255},
	"papayawhip":           color.NRGBA{R: 0xff, G: 0xef, B: 0xd5, A: 255},
	"peachpuff":            color.NRGBA{R: 0xff, G: 0xda, B: 0xb9, A: 255},
	"peru":                 color.NRGBA{R: 0xcd, G: 0x85, B: 0x3f, A: 255},
	"pink":                 color.NRGBA{R: 0xff, G: 0xc0, B: 0xcb, A: 255},
	"plum":                 color.NRGBA{R: 0xdd, G: 0xa0, B: 0xdd, A: 255},
	"powderblue":           color.NRGBA{R: 0xb0, G: 0xe0, B: 0xe6, A: 255},
	"purple":               color.NRGBA{R: 0x80, G: 0x00, B: 0x80, A: 255},
	"rebeccapurple":        color.NRGBA{R: 0x66, G: 0x33, B: 0x99, A: 255},
	"red":                  color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 255},
	"rosybrown":            color.NRGBA{R: 0xbc, G: 0x8f, B: 0x8f, A: 255},
	"royalblue":            color.NRGBA{R: 0x41, G: 0x69, B: 0xe1, A: 255},
	"saddlebrown":          color.NRGBA{R: 0x8b, G: 0x45, B: 0x13, A: 255},
	"salmon":               color.NRGBA{R: 0xfa, G: 0x80, B: 0x72, A: 255},
	"sandybrown":           color.NRGBA{R: 0xf4, G: 0xa4, B: 0x60, A: 255},
	"seagreen":             color.NRGBA{R: 0x2e, G: 0x8b, B: 0x57, A: 255},
	"seashell":             color.NRGBA{R: 0xff, G: 0xf5, B: 0xee, A: 255},
	"sienna":               color.NRGBA{R: 0xa0, G: 0x52, B: 0x2d, A: 255},
	"silver":               color.NRGBA{R: 0xc0, G: 0xc0, B: 0xc0, A: 255},
	"skyblue":              color.NRGBA{R: 0x87, G: 0xce, B: 0xeb, A: 255},
	"slateblue":            color.NRGBA{R: 0x6a, G: 0x5a, B: 0xcd, A: 255},
	"slategray":            color.NRGBA{R: 0x70, G: 0x80, B: 0x90, A: 255},
	"slategrey":            color.NRGBA{R: 0x70, G: 0x80, B: 0x90, A: 255},
	"snow":                 color.NRGBA{R: 0xff, G: 0xfa, B: 0xfa, A: 255},
	"springgreen":          color.NRGBA{R: 0x00, G: 0xff, B: 0x7f, A: 255},
	"steelblue":            color.NRGBA{R: 0x46, G: 0x82, B: 0xb4, A: 255},
	"tan":                  color.NRGBA{R: 0xd2, G: 0xb4, B: 0x8c, A: 255},
	"teal":                 color.NRGBA{R: 0x00, G: 0x80, B: 0x80, A: 255},
	"thistle":              color.NRGBA{R: 0xd8, G: 0xbf, B: 0xd8, A: 255},
	"tomato":               color.NRGBA{R: 0xff, G: 0x63, B: 0x47, A: 255},
	"turquoise":            color.NRGBA{R: 0x40, G: 0xe0, B: 0xd0, A: 255},
	"violet":               color.NRGBA{R: 0xee, G: 0x82, B: 0xee, A: 255},
	"wheat":                color.NRGBA{R: 0xf5, G: 0xde, B: 0xb3, A: 255},
	"white":                color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 255},
	"whitesmoke":           color.NRGBA{R: 0xf5, G: 0xf5, B: 0xf5, A: 255},
	"yellow":               color.NRGBA{R: 0xff, G: 0xff, B: 0x00, A: 255},
	"yellowgreen":          color.NRGBA{R: 0x9a, G: 0xcd, B: 0x32, A: 255},
}

// parseRGBFunc parses "rgb(r,g,b)" or "rgba(r,g,b,a)" format.
func parseRGBFunc(s string) (color.Color, error) {
	// Strip function name and parens.
	idx := strings.Index(s, "(")
	if idx < 0 || s[len(s)-1] != ')' {
		return nil, fmt.Errorf("svg: malformed color function %q", s)
	}
	inner := s[idx+1 : len(s)-1]
	parts := strings.Split(inner, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch len(parts) {
	case 3: // rgb(r, g, b)
		r, err1 := strconv.Atoi(parts[0])
		g, err2 := strconv.Atoi(parts[1])
		b, err3 := strconv.Atoi(parts[2])
		if err := firstErr(err1, err2, err3); err != nil {
			return nil, fmt.Errorf("svg: invalid rgb() values: %w", err)
		}
		return color.NRGBA{R: clampByte(r), G: clampByte(g), B: clampByte(b), A: 255}, nil

	case 4: // rgba(r, g, b, a)
		r, err1 := strconv.Atoi(parts[0])
		g, err2 := strconv.Atoi(parts[1])
		b, err3 := strconv.Atoi(parts[2])
		a, err4 := strconv.ParseFloat(parts[3], 64)
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return nil, fmt.Errorf("svg: invalid rgba() values: %w", err)
		}
		return color.NRGBA{R: clampByte(r), G: clampByte(g), B: clampByte(b), A: clampByte(int(a * 255))}, nil

	default:
		return nil, fmt.Errorf("svg: expected 3 or 4 values in %q, got %d", s, len(parts))
	}
}

// clampByte clamps an integer to the 0-255 range.
func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// firstErr returns the first non-nil error from a list.
func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// parseColor parses an SVG color string into a Go color.Color.
// Supported formats: #RGB/#RRGGBB/#RRGGBBAA hex, rgb()/rgba() functions,
// the CSS Level 4 named color list, "none"/"currentColor"/"" (nil color).
func parseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == colorNone || s == "currentColor" {
		return nil, nil //nolint:nilnil // nil color is valid for "none"/""/currentColor
	}

	// Hex colors
	if s[0] == '#' {
		return parseHexColor(s[1:])
	}

	// rgb()/rgba()
	if strings.HasPrefix(s, "rgb") {
		return parseRGBFunc(s)
	}

	// Named colors
	if c, ok := namedColors[strings.ToLower(s)]; ok {
		return c, nil
	}

	return nil, fmt.Errorf("svg: unsupported color value %q", s)
}

// parseHexColor parses hex digits (without leading #) into color.NRGBA.
func parseHexColor(hex string) (color.Color, error) {
	switch len(hex) {
	case 3: // RGB
		r, err1 := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
		g, err2 := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
		b, err3 := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
		if err := firstErr(err1, err2, err3); err != nil {
			return nil, fmt.Errorf("svg: invalid hex color #%s: %w", hex, err)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil

	case 4: // RGBA
		r, err1 := strconv.ParseUint(string(hex[0])+string(hex[0]), 16, 8)
		g, err2 := strconv.ParseUint(string(hex[1])+string(hex[1]), 16, 8)
		b, err3 := strconv.ParseUint(string(hex[2])+string(hex[2]), 16, 8)
		a, err4 := strconv.ParseUint(string(hex[3])+string(hex[3]), 16, 8)
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return nil, fmt.Errorf("svg: invalid hex color #%s: %w", hex, err)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil

	case 6: // RRGGBB
		r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
		g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
		b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
		if err := firstErr(err1, err2, err3); err != nil {
			return nil, fmt.Errorf("svg: invalid hex color #%s: %w", hex, err)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil

	case 8: // RRGGBBAA
		r, err1 := strconv.ParseUint(hex[0:2], 16, 8)
		g, err2 := strconv.ParseUint(hex[2:4], 16, 8)
		b, err3 := strconv.ParseUint(hex[4:6], 16, 8)
		a, err4 := strconv.ParseUint(hex[6:8], 16, 8)
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return nil, fmt.Errorf("svg: invalid hex color #%s: %w", hex, err)
		}
		return color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil

	default:
		return nil, fmt.Errorf("svg: invalid hex color length %d in #%s", len(hex), hex)
	}
}
