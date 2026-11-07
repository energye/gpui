//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// LengthUnit is an SVG <length> unit (CSS units at the 96dpi reference pixel,
// matching SkSVGAttributeParser). Font-relative units (em/ex) resolve against
// the inherited font-size at the element.
type LengthUnit uint8

const (
	LengthUnitPx LengthUnit = iota // user units / CSS px (default)
	LengthUnitPt                   // 96/72 px
	LengthUnitPc                   // 16 px
	LengthUnitIn                   // 96 px
	LengthUnitCm                   // 96/2.54 px
	LengthUnitMm                   // 96/25.4 px
	LengthUnitQ                    // 96/254 px
	LengthUnitEm                   // × inherited font-size
	LengthUnitEx                   // × inherited font-size × 0.5 (x-height approximation, Skia alike)
	LengthUnitPercent              // × viewport axis (see LengthAxis)
)

// String returns the unit suffix as written in SVG.
func (u LengthUnit) String() string {
	switch u {
	case LengthUnitPx:
		return "px"
	case LengthUnitPt:
		return "pt"
	case LengthUnitPc:
		return "pc"
	case LengthUnitIn:
		return "in"
	case LengthUnitCm:
		return "cm"
	case LengthUnitMm:
		return "mm"
	case LengthUnitQ:
		return "q"
	case LengthUnitEm:
		return "em"
	case LengthUnitEx:
		return "ex"
	case LengthUnitPercent:
		return "%"
	}
	return "?"
}

// LengthAxis is the viewport axis a percentage length resolves against
// (SVG 1.1 §7.10: x/width → viewport width, y/height → viewport height,
// radii → normalized diagonal sqrt((w²+h²)/2)).
type LengthAxis uint8

const (
	LengthAxisX LengthAxis = iota
	LengthAxisY
	LengthAxisDiagonal
)

// RefSpace is the coordinate context a Length resolves in: the current
// viewport size and the inherited font-size at the element.
type RefSpace struct {
	ViewportW, ViewportH float64
	FontSize             float64 // px
}

// Length is a parsed SVG <length>: a value plus its unit. Resolution is
// deferred to the referencing viewport (viewBox can nest), so geometry is
// stored unresolved and resolved at render time.
type Length struct {
	Value float64
	Unit  LengthUnit
}

// Resolve converts the length to user-unit pixels in the given reference
// space along the given axis.
func (l Length) Resolve(r RefSpace, axis LengthAxis) float64 {
	switch l.Unit {
	case LengthUnitPx:
		return l.Value
	case LengthUnitPt:
		return l.Value * 96 / 72
	case LengthUnitPc:
		return l.Value * 16
	case LengthUnitIn:
		return l.Value * 96
	case LengthUnitCm:
		return l.Value * 96 / 2.54
	case LengthUnitMm:
		return l.Value * 96 / 25.4
	case LengthUnitQ:
		return l.Value * 96 / 254
	case LengthUnitEm:
		return l.Value * r.FontSize
	case LengthUnitEx:
		return l.Value * r.FontSize * 0.5
	case LengthUnitPercent:
		var base float64
		switch axis {
		case LengthAxisX:
			base = r.ViewportW
		case LengthAxisY:
			base = r.ViewportH
		case LengthAxisDiagonal:
			base = math.Sqrt((r.ViewportW*r.ViewportW + r.ViewportH*r.ViewportH) / 2)
		}
		return l.Value * base / 100
	}
	return l.Value
}

// String renders the length back to its SVG form (diagnostics).
func (l Length) String() string {
	return fmt.Sprintf("%g%s", l.Value, l.Unit)
}

// parseLength parses an SVG <length> value with full unit support.
// A bare number is user units (px). An empty string resolves to zero px,
// which is the SVG initial value for geometry offsets (x, y, cx, cy...).
func parseLength(s string) (Length, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Length{}, nil
	}
	// Unit suffix: the longest suffixes first so "px" does not shadow a bare
	// number ending in 'x' (none exist, but keep the parse explicit).
	for _, u := range []struct {
		suffix string
		unit   LengthUnit
	}{
		{"px", LengthUnitPx}, {"pt", LengthUnitPt}, {"pc", LengthUnitPc},
		{"in", LengthUnitIn}, {"cm", LengthUnitCm}, {"mm", LengthUnitMm},
		{"q", LengthUnitQ}, {"em", LengthUnitEm}, {"ex", LengthUnitEx},
		{"%", LengthUnitPercent},
	} {
		if strings.HasSuffix(s, u.suffix) {
			raw := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return Length{}, fmt.Errorf("invalid length %q: %w", s, err)
			}
			return Length{Value: v, Unit: u.unit}, nil
		}
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Length{}, fmt.Errorf("invalid length %q: %w", s, err)
	}
	return Length{Value: v, Unit: LengthUnitPx}, nil
}

// mustLength parses a length, reporting the owning element/attribute on
// failure through the parse context (rendering continues with zero).
func mustLength(pc *parseCtx, elem, attr, s string) Length {
	l, err := parseLength(s)
	if err != nil {
		pc.warn(elem, attr, warnInvalidValue, err.Error())
		return Length{}
	}
	return l
}
