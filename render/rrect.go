//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import "math"

// RRectType describes the specialization of an RRect. The values increase as
// the geometry becomes less restrictive, matching SkRRect::Type.
type RRectType uint8

const (
	// RRectEmpty is a rectangle with zero width or height.
	RRectEmpty RRectType = iota
	// RRectRect is a non-empty rectangle with all corner radii zero.
	RRectRect
	// RRectOval is an ellipse: all corners share the half-width/half-height
	// radii.
	RRectOval
	// RRectSimple has equal radii on all four corners but is not an oval.
	RRectSimple
	// RRectNinePatch has matching radii along each side, so its corner
	// ellipses divide the bounds into nine axis-aligned patches.
	RRectNinePatch
	// RRectComplex is a valid rounded rectangle that is neither simple nor a
	// nine-patch.
	RRectComplex
)

// String returns the Skia-style type name.
func (t RRectType) String() string {
	switch t {
	case RRectEmpty:
		return "empty"
	case RRectRect:
		return "rect"
	case RRectOval:
		return "oval"
	case RRectSimple:
		return "simple"
	case RRectNinePatch:
		return "nine-patch"
	case RRectComplex:
		return "complex"
	default:
		return "unknown"
	}
}

// RRectCorner identifies one corner of an RRect. Radii are stored in
// upper-left, upper-right, lower-right, lower-left order, matching Skia.
type RRectCorner uint8

const (
	// RRectUpperLeft is the top-left corner.
	RRectUpperLeft RRectCorner = iota
	// RRectUpperRight is the top-right corner.
	RRectUpperRight
	// RRectLowerRight is the bottom-right corner.
	RRectLowerRight
	// RRectLowerLeft is the bottom-left corner.
	RRectLowerLeft
)

// RRect is an axis-aligned rectangle with an independent x/y radius at each
// corner. It is the SkRRect analogue used by the per-corner draw and clip
// entry points.
//
// Radii are normalized on construction: non-finite or negative radii are
// clamped to zero, a corner with either axis at zero becomes square, and
// overlapping corner curves are scaled down proportionally. These are the
// same rules SkRRect::setRectRadii uses, so a uniform RRect is byte-for-byte
// equivalent to the existing single-radius construction.
//
// The zero value is an empty RRect.
type RRect struct {
	rect  Rect
	radii [4]Point
	typ   RRectType
}

// NewRRect returns a rectangle with no rounded corners.
func NewRRect(rect Rect) RRect {
	var rr RRect
	rr.SetRect(rect)
	return rr
}

// NewRRectXY returns a rectangle with the same x/y radii on all four corners.
func NewRRectXY(rect Rect, rx, ry float64) RRect {
	var rr RRect
	rr.SetRectXY(rect, rx, ry)
	return rr
}

// NewRRectRadii returns a rectangle with one radius pair per corner. The
// order is upper-left, upper-right, lower-right, lower-left.
func NewRRectRadii(rect Rect, radii [4]Point) RRect {
	var rr RRect
	rr.SetRectRadii(rect, radii)
	return rr
}

// SetEmpty resets rr to the empty rectangle with zero radii.
func (rr *RRect) SetEmpty() {
	rr.rect = Rect{}
	rr.radii = [4]Point{}
	rr.typ = RRectEmpty
}

// SetRect makes rr a rectangle with no rounded corners.
func (rr *RRect) SetRect(rect Rect) {
	if !rrRectFinite(rect) {
		rr.SetEmpty()
		return
	}
	rect = NewRect(rect.Min, rect.Max)
	if rect.Width() <= 0 || rect.Height() <= 0 {
		rr.SetEmpty()
		return
	}
	rr.rect = rect
	rr.radii = [4]Point{}
	rr.typ = RRectRect
}

// SetOval makes rr an ellipse filling rect.
func (rr *RRect) SetOval(rect Rect) {
	if !rrRectFinite(rect) {
		rr.SetEmpty()
		return
	}
	rect = NewRect(rect.Min, rect.Max)
	if rect.Width() <= 0 || rect.Height() <= 0 {
		rr.SetEmpty()
		return
	}
	rx := rect.Width() / 2
	ry := rect.Height() / 2
	rr.rect = rect
	rr.radii = [4]Point{{rx, ry}, {rx, ry}, {rx, ry}, {rx, ry}}
	rr.typ = RRectOval
}

// SetRectXY makes rr a rectangle with the same (rx, ry) radii at every
// corner. The radii are reduced proportionally if they would overlap.
func (rr *RRect) SetRectXY(rect Rect, rx, ry float64) {
	if !rrRectFinite(rect) {
		rr.SetEmpty()
		return
	}
	rect = NewRect(rect.Min, rect.Max)
	if rect.Width() <= 0 || rect.Height() <= 0 {
		rr.SetEmpty()
		return
	}
	if math.IsNaN(rx) || math.IsNaN(ry) || math.IsInf(rx, 0) || math.IsInf(ry, 0) ||
		rx <= 0 || ry <= 0 {
		rr.SetRect(rect)
		return
	}

	w, h := rect.Width(), rect.Height()
	if w < 2*rx || h < 2*ry {
		scale := math.Min(w/(2*rx), h/(2*ry))
		rx *= scale
		ry *= scale
	}
	if rx <= 0 || ry <= 0 {
		rr.SetRect(rect)
		return
	}

	rr.rect = rect
	rr.radii = [4]Point{{rx, ry}, {rx, ry}, {rx, ry}, {rx, ry}}
	if rx >= w/2 && ry >= h/2 {
		rr.typ = RRectOval
	} else {
		rr.typ = RRectSimple
	}
}

// SetRectRadii makes rr a rectangle with per-corner radii. Radii are stored in
// upper-left, upper-right, lower-right, lower-left order. The input is
// normalized exactly like SkRRect::setRectRadii: invalid radii fall back to a
// sharp rectangle, zero axes zero the whole corner, and overlapping curves
// are scaled down proportionally.
func (rr *RRect) SetRectRadii(rect Rect, radii [4]Point) {
	if !rrRectFinite(rect) {
		rr.SetEmpty()
		return
	}
	rect = NewRect(rect.Min, rect.Max)
	if rect.Width() <= 0 || rect.Height() <= 0 {
		rr.SetEmpty()
		return
	}
	for _, r := range radii {
		if math.IsNaN(r.X) || math.IsNaN(r.Y) || math.IsInf(r.X, 0) || math.IsInf(r.Y, 0) {
			rr.SetRect(rect)
			return
		}
	}

	if rrClampRadiiToZero(&radii) {
		rr.SetRect(rect)
		return
	}
	rrScaleRadii(rect, &radii)
	if rrClampRadiiToZero(&radii) {
		rr.SetRect(rect)
		return
	}

	rr.rect = rect
	rr.radii = radii
	rr.computeType()
}

// Type returns the normalized specialization of rr.
func (rr RRect) Type() RRectType {
	return rr.typ
}

// Rect returns rr's bounds.
func (rr RRect) Rect() Rect {
	return rr.rect
}

// Bounds is an alias for Rect.
func (rr RRect) Bounds() Rect {
	return rr.rect
}

// Width returns the bounds width.
func (rr RRect) Width() float64 {
	return rr.rect.Width()
}

// Height returns the bounds height.
func (rr RRect) Height() float64 {
	return rr.rect.Height()
}

// Radii returns the x/y radius pair for one corner.
func (rr RRect) Radii(corner RRectCorner) Point {
	if corner > RRectLowerLeft {
		return Point{}
	}
	return rr.radii[corner]
}

// RadiiArray returns a copy of all four corner radii in upper-left,
// upper-right, lower-right, lower-left order.
func (rr RRect) RadiiArray() [4]Point {
	return rr.radii
}

// IsEmpty reports whether rr has zero width or height.
func (rr RRect) IsEmpty() bool { return rr.typ == RRectEmpty }

// IsRect reports whether rr has no rounded corners.
func (rr RRect) IsRect() bool { return rr.typ == RRectRect }

// IsOval reports whether rr fills its bounds as an ellipse.
func (rr RRect) IsOval() bool { return rr.typ == RRectOval }

// IsSimple reports whether all four corners have equal radii but rr is not an
// oval.
func (rr RRect) IsSimple() bool { return rr.typ == RRectSimple }

// IsSimpleCircular reports whether rr is simple and has circular corners.
func (rr RRect) IsSimpleCircular() bool {
	return rr.typ == RRectSimple && rr.radii[0].X == rr.radii[0].Y
}

// IsNinePatch reports whether matching radii along each side divide rr into
// nine axis-aligned patches.
func (rr RRect) IsNinePatch() bool { return rr.typ == RRectNinePatch }

// IsComplex reports whether rr is neither simple nor a nine-patch.
func (rr RRect) IsComplex() bool { return rr.typ == RRectComplex }

// computeType classifies rr after its radii have been normalized. It mirrors
// SkRRect::computeType, including the side-matching test that distinguishes a
// nine-patch from a general complex RRect.
func (rr *RRect) computeType() {
	if rr.rect.Width() <= 0 || rr.rect.Height() <= 0 {
		rr.typ = RRectEmpty
		return
	}

	allCornersSquare := true
	for i := 0; i < 4; i++ {
		if rr.radii[i].X != 0 && rr.radii[i].Y != 0 {
			allCornersSquare = false
			break
		}
	}
	if allCornersSquare {
		rr.radii = [4]Point{}
		rr.typ = RRectRect
		return
	}

	allRadiiEqual := true
	for i := 1; i < 4; i++ {
		if rr.radii[i] != rr.radii[i-1] {
			allRadiiEqual = false
			break
		}
	}
	if allRadiiEqual {
		if rr.radii[0].X >= rr.rect.Width()/2 && rr.radii[0].Y >= rr.rect.Height()/2 {
			rr.typ = RRectOval
		} else {
			rr.typ = RRectSimple
		}
		return
	}

	if rrRadiiAreNinePatch(rr.radii) {
		rr.typ = RRectNinePatch
	} else {
		rr.typ = RRectComplex
	}
}

func rrRadiiAreNinePatch(radii [4]Point) bool {
	ul, ur, lr, ll := radii[0], radii[1], radii[2], radii[3]
	return ul.X == ll.X && ul.Y == ur.Y && ur.X == lr.X && ll.Y == lr.Y
}

func rrRectFinite(rect Rect) bool {
	return !math.IsNaN(rect.Min.X) && !math.IsNaN(rect.Min.Y) &&
		!math.IsNaN(rect.Max.X) && !math.IsNaN(rect.Max.Y) &&
		!math.IsInf(rect.Min.X, 0) && !math.IsInf(rect.Min.Y, 0) &&
		!math.IsInf(rect.Max.X, 0) && !math.IsInf(rect.Max.Y, 0)
}

// rrClampRadiiToZero applies Skia's corner rule: if either axis is zero or
// negative, the corner is square on both axes. It reports whether every corner
// ended up square.
func rrClampRadiiToZero(radii *[4]Point) bool {
	allZero := true
	for i := range radii {
		if radii[i].X <= 0 || radii[i].Y <= 0 {
			radii[i] = Point{}
		}
		if radii[i].X != 0 || radii[i].Y != 0 {
			allZero = false
		}
	}
	return allZero
}

// rrScaleRadii implements the CSS Backgrounds 5.5 overlapping-curve rule used
// by SkRRect::scaleRadii: find the smallest side/radii-sum ratio and scale all
// radii by it.
func rrScaleRadii(rect Rect, radii *[4]Point) {
	w, h := rect.Width(), rect.Height()
	scale := 1.0
	scale = rrMinScale(radii[0].X, radii[1].X, w, scale)
	scale = rrMinScale(radii[1].Y, radii[2].Y, h, scale)
	scale = rrMinScale(radii[2].X, radii[3].X, w, scale)
	scale = rrMinScale(radii[3].Y, radii[0].Y, h, scale)
	if scale >= 1 {
		return
	}
	for i := range radii {
		radii[i].X *= scale
		radii[i].Y *= scale
	}

	// Guard against floating-point round-off putting an edge slightly over its
	// limit. Skia's SkScaleToSides::AdjustRadii does the same pair-wise repair.
	rrAdjustRadiiPair(w, &radii[0].X, &radii[1].X)
	rrAdjustRadiiPair(h, &radii[1].Y, &radii[2].Y)
	rrAdjustRadiiPair(w, &radii[2].X, &radii[3].X)
	rrAdjustRadiiPair(h, &radii[3].Y, &radii[0].Y)
}

func rrMinScale(a, b, limit, current float64) float64 {
	if a+b > limit {
		if s := limit / (a + b); s < current {
			return s
		}
	}
	return current
}

func rrAdjustRadiiPair(limit float64, a, b *float64) {
	if *a+*b <= limit {
		return
	}
	s := limit / (*a + *b)
	*a *= s
	*b *= s
}
