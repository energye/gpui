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

	"github.com/energye/gpui/render"
)

// parseTransformList parses an SVG transform attribute value into a single
// affine matrix: "f1 f2 … fn" composes to M = M1·M2·…·Mn (points are
// transformed by fn first). Bitwise-identical to the previous per-function
// Context.Translate/Rotate/… chain — those build the same render.* matrices
// through the same Matrix.Multiply (context_transform.go).
//
// Supported functions: translate, scale, rotate, matrix, skewX, skewY.
func parseTransformList(s string) (render.Matrix, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return render.Identity(), nil
	}
	m := render.Identity()
	pos := 0
	for pos < len(s) {
		// Skip whitespace.
		for pos < len(s) && isSpace(s[pos]) {
			pos++
		}
		if pos >= len(s) {
			break
		}

		// Read function name.
		nameStart := pos
		for pos < len(s) && s[pos] != '(' && !isSpace(s[pos]) {
			pos++
		}
		name := s[nameStart:pos]

		// Skip whitespace before '('.
		for pos < len(s) && isSpace(s[pos]) {
			pos++
		}
		if pos >= len(s) || s[pos] != '(' {
			return m, fmt.Errorf("svg: expected '(' after transform function %q", name)
		}
		pos++ // skip '('

		// Find closing ')'.
		parenStart := pos
		for pos < len(s) && s[pos] != ')' {
			pos++
		}
		if pos >= len(s) {
			return m, fmt.Errorf("svg: missing ')' in transform %q", name)
		}
		argsStr := s[parenStart:pos]
		pos++ // skip ')'

		args, err := parseTransformArgs(argsStr)
		if err != nil {
			return m, fmt.Errorf("svg: transform %s: %w", name, err)
		}

		fm, err := parseTransformFunc(name, args)
		if err != nil {
			return m, err
		}
		m = m.Multiply(fm)
	}
	return m, nil
}

// applyTransformList parses and applies a transform list to the context.
func applyTransformList(dc *render.Context, s string) error {
	m, err := parseTransformList(s)
	if err != nil {
		return err
	}
	dc.Transform(m)
	return nil
}

// parseTransformFunc builds the matrix for one transform function.
func parseTransformFunc(name string, args []float64) (render.Matrix, error) {
	switch name {
	case "translate":
		if len(args) < 1 {
			return render.Matrix{}, fmt.Errorf("svg: translate requires at least 1 arg, got %d", len(args))
		}
		tx := args[0]
		ty := 0.0
		if len(args) >= 2 {
			ty = args[1]
		}
		return render.Translate(tx, ty), nil

	case "rotate":
		switch {
		case len(args) == 1:
			// rotate(angle) — angle in degrees, about origin
			return render.Rotate(args[0] * math.Pi / 180.0), nil
		case len(args) >= 3:
			// rotate(angle, cx, cy) — rotate about (cx, cy)
			// Identical composition to Context.RotateAbout: T·R·T⁻¹.
			rad := args[0] * math.Pi / 180.0
			return render.Translate(args[1], args[2]).
				Multiply(render.Rotate(rad)).
				Multiply(render.Translate(-args[1], -args[2])), nil
		default:
			return render.Matrix{}, fmt.Errorf("svg: rotate requires 1 or 3 args, got %d", len(args))
		}

	case "scale":
		if len(args) < 1 {
			return render.Matrix{}, fmt.Errorf("svg: scale requires at least 1 arg, got %d", len(args))
		}
		sx := args[0]
		sy := sx
		if len(args) >= 2 {
			sy = args[1]
		}
		return render.Scale(sx, sy), nil

	case "matrix":
		if len(args) != 6 {
			return render.Matrix{}, fmt.Errorf("svg: matrix requires 6 args, got %d", len(args))
		}
		// SVG matrix(a,b,c,d,e,f): x' = a*x + c*y + e, y' = b*x + d*y + f
		// gg Matrix{A,B,C,D,E,F}: x' = A*x + B*y + C, y' = D*x + E*y + F
		return render.Matrix{
			A: args[0], B: args[2], C: args[4],
			D: args[1], E: args[3], F: args[5],
		}, nil

	case "skewX":
		if len(args) != 1 {
			return render.Matrix{}, fmt.Errorf("svg: skewX requires 1 arg, got %d", len(args))
		}
		angle := args[0] * math.Pi / 180.0
		return render.Matrix{
			A: 1, B: math.Tan(angle), C: 0,
			D: 0, E: 1, F: 0,
		}, nil

	case "skewY":
		if len(args) != 1 {
			return render.Matrix{}, fmt.Errorf("svg: skewY requires 1 arg, got %d", len(args))
		}
		angle := args[0] * math.Pi / 180.0
		return render.Matrix{
			A: 1, B: 0, C: 0,
			D: math.Tan(angle), E: 1, F: 0,
		}, nil

	default:
		return render.Matrix{}, fmt.Errorf("svg: unsupported transform function %q", name)
	}
}

// applyTransform applies the element's transform attribute to the context
// (legacy entry, now a thin shell over parseTransformList).
func applyTransform(dc *render.Context, transform string) error {
	return applyTransformList(dc, transform)
}

// parseTransformArgs splits a comma/space-separated argument string into float64 values.
func parseTransformArgs(s string) ([]float64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	// Replace commas with spaces, then split on whitespace.
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)

	args := make([]float64, 0, len(parts))
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", p, err)
		}
		args = append(args, v)
	}
	return args, nil
}

// isSpace returns true for ASCII whitespace characters.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
