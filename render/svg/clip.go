//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"math"

	"github.com/energye/gpui/render"
)

// clipPath construction: child shapes → one clip region in user space.
//
// A single child keeps its own path untouched (winding rules and holes
// survive; the fill rule comes from the shape's clip-rule). Multiple
// children union via the P3.5 boolean base (curve-preserving); the union
// is a nonzero region, so evenodd holes *between* sibling shapes are out
// of this scope (registered limitation — rare in practice).

// clipRegionFor builds the clip region for a clip-path reference at the
// referencing element. refBBox is the referencing element's object bounding
// box (objectBoundingBox units), rs the current reference space. rule is the
// fill rule the region must be rasterized with (the single shape's
// clip-rule; unions are nonzero). ok=false = render nothing (fail-closed).
func (d *Document) clipRegionFor(id string, refBBox render.Rect, rs RefSpace) (*render.Path, render.FillRule, bool) {
	tgt := d.ElementByID(id)
	cp, ok := tgt.(*ClipPathElement)
	if !ok {
		return nil, render.FillRuleNonZero, false
	}
	// Unit mapping for clipPathUnits=objectBoundingBox: shape coordinates
	// are fractions of the referencing element's bbox.
	unitM := render.Identity()
	if cp.Units == UnitsObjectBoundingBox {
		w := refBBox.Max.X - refBBox.Min.X
		h := refBBox.Max.Y - refBBox.Min.Y
		if w <= 0 || h <= 0 {
			return nil, render.FillRuleNonZero, false
		}
		unitM = render.Translate(refBBox.Min.X, refBBox.Min.Y).Multiply(render.Scale(w, h))
	}

	// The clipPath element's own transform shifts the whole clip region
	// (usvg clip conversion: resolve_transform — transform-origin shifts
	// the origin, percentages against the viewBox).
	var cpM render.Matrix
	if cp.Attrs.Transform != "" {
		if m, err := parseTransformList(cp.Attrs.Transform); err == nil {
			cpM = withTransformOrigin(m, &cp.Attrs, rs)
		}
	}

	var paths []*render.Path
	for _, child := range cp.Children {
		// <use> inside clipPath resolves to its target shape.
		child = resolveUseTarget(d, child)
		p := shapePath(child, rs)
		if p == nil {
			continue // non-shape children contribute nothing
		}
		if unitM != render.Identity() {
			p = transformPath(p, unitM)
		}
		if cpM != render.Identity() && cpM != (render.Matrix{}) {
			p = transformPath(p, cpM)
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return nil, render.FillRuleNonZero, false
	}
	if len(paths) == 1 {
		// Single shape: keep its own winding; the shape's clip-rule decides
		// the fill rule (clip-rule 归位).
		rule := render.FillRuleNonZero
		raw := resolveUseTarget(d, cp.Children[0])
		if a := raw.attrs(); a != nil && a.ClipRule == "evenodd" {
			rule = render.FillRuleEvenOdd
		}
		return paths[0], rule, true
	}
	union := paths[0]
	for _, p := range paths[1:] {
		union = union.Op(p, render.PathOpUnion)
	}
	return union, render.FillRuleNonZero, true
}

// resolveUseTarget resolves a <use> inside a clipPath to its target shape.
func resolveUseTarget(d *Document, e Element) Element {
	u, ok := e.(*UseElement)
	if !ok || u.invalid || u.target == nil {
		return e
	}
	return u.target
}

// shapePath converts one shape element into a path in user space
// (coordinates resolved against rs; element transform attributes are
// applied — clip shapes honor their own transform).
func shapePath(e Element, rs RefSpace) *render.Path {
	var b *render.PathBuilder
	switch t := e.(type) {
	case *PathElement:
		if t.D == "" {
			return nil
		}
		p, err := render.ParseSVGPath(t.D)
		if err != nil {
			return nil
		}
		if t.Attrs.Transform != "" {
			if m, err := parseTransformList(t.Attrs.Transform); err == nil {
				p = transformPath(p, m)
			}
		}
		return p
	case *RectElement:
		x, y := t.X.Resolve(rs, LengthAxisX), t.Y.Resolve(rs, LengthAxisY)
		w, h := t.W.Resolve(rs, LengthAxisX), t.H.Resolve(rs, LengthAxisY)
		b = render.BuildPath().Rect(x, y, w, h)
		if t.RX.Value > 0 || t.RY.Value > 0 {
			r := t.RX.Resolve(rs, LengthAxisX)
			if ry := t.RY.Resolve(rs, LengthAxisY); ry > r {
				r = ry
			}
			b = render.BuildPath().RoundRect(x, y, w, h, r)
		}
	case *CircleElement:
		cx, cy := t.CX.Resolve(rs, LengthAxisX), t.CY.Resolve(rs, LengthAxisY)
		r := t.R.Resolve(rs, LengthAxisDiagonal)
		b = render.BuildPath().Ellipse(cx, cy, r, r)
	case *EllipseElement:
		b = render.BuildPath().Ellipse(
			t.CX.Resolve(rs, LengthAxisX), t.CY.Resolve(rs, LengthAxisY),
			t.RX.Resolve(rs, LengthAxisX), t.RY.Resolve(rs, LengthAxisY))
	case *PolygonElement:
		b = pointsBuilder(t.Points, rs, true)
	case *PolylineElement:
		b = pointsBuilder(t.Points, rs, false)
	default:
		return nil // line/text/... contribute nothing (zero area)
	}
	if b == nil {
		return nil
	}
	p := b.Path()
	if a := e.attrs(); a != nil && a.Transform != "" {
		if m, err := parseTransformList(a.Transform); err == nil {
			p = transformPath(p, m)
		}
	}
	return p
}

func pointsBuilder(points []Length, rs RefSpace, closed bool) *render.PathBuilder {
	var b *render.PathBuilder
	for i := 0; i+1 < len(points); i += 2 {
		x := points[i].Resolve(rs, LengthAxisX)
		y := points[i+1].Resolve(rs, LengthAxisY)
		if b == nil {
			b = render.BuildPath().MoveTo(x, y)
			continue
		}
		b = b.LineTo(x, y)
	}
	if closed && b != nil {
		b = b.Close()
	}
	return b
}

// transformPath applies an affine to every coordinate of a path. Conics
// carry a rational weight that only survives conformal maps — non-conformal
// affines expand conics to quads first (render.Path.ExpandConicsToQuads,
// the P3.5 batch-1 boundary expansion) and transform the quads.
func transformPath(p *render.Path, m render.Matrix) *render.Path {
	if !affineConformal(m) && p.HasConics() {
		p = p.ExpandConicsToQuads(0.1)
	}
	out := newPathBuilder()
	p.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			pt := m.TransformPoint(render.Point{X: coords[0], Y: coords[1]})
			out.MoveTo(pt.X, pt.Y)
		case render.LineTo:
			pt := m.TransformPoint(render.Point{X: coords[0], Y: coords[1]})
			out.LineTo(pt.X, pt.Y)
		case render.QuadTo:
			c := m.TransformPoint(render.Point{X: coords[0], Y: coords[1]})
			e := m.TransformPoint(render.Point{X: coords[2], Y: coords[3]})
			out.QuadTo(c.X, c.Y, e.X, e.Y)
		case render.CubicTo:
			c1 := m.TransformPoint(render.Point{X: coords[0], Y: coords[1]})
			c2 := m.TransformPoint(render.Point{X: coords[2], Y: coords[3]})
			e := m.TransformPoint(render.Point{X: coords[4], Y: coords[5]})
			out.CubicTo(c1.X, c1.Y, c2.X, c2.Y, e.X, e.Y)
		case render.ConicTo:
			// Conformal map: weight is invariant, transform points as-is.
			c := m.TransformPoint(render.Point{X: coords[0], Y: coords[1]})
			e := m.TransformPoint(render.Point{X: coords[2], Y: coords[3]})
			out.ConicTo(c.X, c.Y, e.X, e.Y, coords[4])
		case render.Close:
			out.Close()
		}
	})
	return out.Path()
}

// affineConformal reports whether the affine preserves circles (uniform
// scale + rotation/translation), making conic weights transformable as-is.
func affineConformal(m render.Matrix) bool {
	// Columns must be orthogonal with equal length.
	a, bd := m.A, m.D // first column (A, D)
	b1, e := m.B, m.E // second column (B, E)
	dot := a*b1 + bd*e
	la := a*a + bd*bd
	lb := b1*b1 + e*e
	return math.Abs(dot) < 1e-9 && math.Abs(la-lb) < 1e-9*la
}

// newPathBuilder builds a path builder (thin indirection so the clip code
// does not depend on the builder constructor name).
func newPathBuilder() *render.PathBuilder {
	return render.BuildPath()
}
