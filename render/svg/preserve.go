//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"strings"

	"github.com/energye/gpui/render"
)

// ParAlign is the nine-point alignment of the viewBox content inside the
// viewport. The zero value is the SVG default (xMidYMid).
type ParAlign uint8

const (
	ParAlignXMidYMid ParAlign = iota // SVG default
	ParAlignXMinYMin
	ParAlignXMidYMin
	ParAlignXMaxYMin
	ParAlignXMinYMid
	ParAlignXMaxYMid
	ParAlignXMinYMax
	ParAlignXMidYMax
	ParAlignXMaxYMax
)

// PreserveAspectRatio is the SVG preserveAspectRatio attribute:
// "<align> <meetOrSlice>" with meetOrSlice defaulting to meet.
// The zero value is the SVG default: xMidYMid meet.
type PreserveAspectRatio struct {
	Align ParAlign
	Slice bool // true = slice (cover), false = meet (contain)
}

// parsePreserveAspectRatio parses the attribute value. Unrecognized values
// fall back to the default (xMidYMid meet) per the SVG error handling rule.
func parsePreserveAspectRatio(s string) PreserveAspectRatio {
	var out PreserveAspectRatio
	fields := strings.Fields(strings.TrimSpace(s))
	for _, f := range fields {
		switch f {
		case "meet":
			out.Slice = false
		case "slice":
			out.Slice = true
		default:
			out.Align = parseParAlign(f)
		}
	}
	return out
}

func parseParAlign(s string) ParAlign {
	switch s {
	case "xMinYMin":
		return ParAlignXMinYMin
	case "xMidYMin":
		return ParAlignXMidYMin
	case "xMaxYMin":
		return ParAlignXMaxYMin
	case "xMinYMid":
		return ParAlignXMinYMid
	case "xMidYMid", "none": // "none" (no pAR) is not part of the static
		// subset: fall back to the default rather than stretching.
		return ParAlignXMidYMid
	case "xMaxYMid":
		return ParAlignXMaxYMid
	case "xMinYMax":
		return ParAlignXMinYMax
	case "xMidYMax":
		return ParAlignXMidYMax
	case "xMaxYMax":
		return ParAlignXMaxYMax
	}
	return ParAlignXMidYMid
}

// viewportTransform maps viewBox user coordinates into a vw×vh viewport with
// uniform scale and per-axis alignment (SkSVGDOM meet/slice semantics).
//
// The result is T(tx,ty) ∘ S(s) ∘ T(−minX,−minY) as a single affine matrix:
//
//	x' = s·x + (tx − s·minX)
//	y' = s·y + (ty − s·minY)
//
// A degenerate viewBox yields the identity (callers skip the transform).
func viewportTransform(vw, vh float64, vb ViewBox, par PreserveAspectRatio) render.Matrix {
	if vb.Width <= 0 || vb.Height <= 0 || vw <= 0 || vh <= 0 {
		return render.Identity()
	}
	sx := vw / vb.Width
	sy := vh / vb.Height
	s := sx
	if par.Slice {
		if sy > s {
			s = sy
		}
	} else {
		if sy < s {
			s = sy
		}
	}
	contentW := vb.Width * s
	contentH := vb.Height * s
	var tx, ty float64
	switch par.Align {
	case ParAlignXMinYMin, ParAlignXMinYMid, ParAlignXMinYMax:
		tx = 0
	case ParAlignXMidYMid, ParAlignXMidYMin, ParAlignXMidYMax:
		tx = (vw - contentW) / 2
	default: // xMax*
		tx = vw - contentW
	}
	switch par.Align {
	case ParAlignXMinYMin, ParAlignXMidYMin, ParAlignXMaxYMin:
		ty = 0
	case ParAlignXMinYMid, ParAlignXMidYMid, ParAlignXMaxYMid:
		ty = (vh - contentH) / 2
	default: // *YMax
		ty = vh - contentH
	}
	return render.Matrix{
		A: s, B: 0, C: tx - s*vb.MinX,
		D: 0, E: s, F: ty - s*vb.MinY,
	}
}
