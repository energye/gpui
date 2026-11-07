//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image/color"

	"github.com/energye/gpui/render"
)

// renderState carries the per-traversal render context: the document (for
// style rules and reference lookup), the theming override color, and the
// current reference space for length resolution.
type renderState struct {
	doc           *Document
	overrideColor color.Color // non-nil → replace all non-"none" colors
	rs            RefSpace    // current viewport + inherited font-size
}

// renderElements renders a list of elements into the given render.Context.
func renderElements(dc *render.Context, elements []Element, p *props, st *renderState) {
	for _, elem := range elements {
		renderElement(dc, elem, p, st)
	}
}

// renderElement dispatches rendering to the element-specific function.
// Each element evaluates its own properties over the inherited set.
func renderElement(dc *render.Context, elem Element, parent *props, st *renderState) {
	if elem == nil {
		return
	}
	a := elem.attrs()
	p := evalProps(parent, *a, st.doc)
	child := *st
	child.rs.FontSize = p.fontSize

	switch e := elem.(type) {
	case *PathElement:
		renderPath(dc, e, a, p, &child)
	case *CircleElement:
		renderCircle(dc, e, a, p, &child)
	case *RectElement:
		renderRect(dc, e, a, p, &child)
	case *EllipseElement:
		renderEllipse(dc, e, a, p, &child)
	case *LineElement:
		renderLine(dc, e, a, p, &child)
	case *PolygonElement:
		renderPolygon(dc, e, a, p, &child)
	case *PolylineElement:
		renderPolyline(dc, e, a, p, &child)
	case *GroupElement:
		dc.Push()
		applyElementTransform(dc, a)
		renderElements(dc, e.Children, p, &child)
		dc.Pop()
	case *DefsElement:
		// Reference-only container: never rendered directly.
	case *StyleElement:
		// Rules are registered on the Document at parse.
	case *SymbolElement:
		// Only rendered through <use>.
	case *UseElement:
		renderUse(dc, e, p, &child)
	case *SVGElement:
		renderNestedSVG(dc, e, p, &child)
	}
}

// renderPath renders an SVG <path> element.
func renderPath(dc *render.Context, e *PathElement, a *Attrs, p *props, st *renderState) {
	if e.D == "" {
		return
	}
	path, err := render.ParseSVGPath(e.D)
	if err != nil {
		return // skip invalid paths silently
	}

	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		dc.DrawPath(path)
	})

	dc.Pop()
}

// renderCircle renders an SVG <circle> element.
func renderCircle(dc *render.Context, e *CircleElement, a *Attrs, p *props, st *renderState) {
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		dc.DrawCircle(e.CX.Resolve(st.rs, LengthAxisX), e.CY.Resolve(st.rs, LengthAxisY),
			e.R.Resolve(st.rs, LengthAxisDiagonal))
	})

	dc.Pop()
}

// renderRect renders an SVG <rect> element.
func renderRect(dc *render.Context, e *RectElement, a *Attrs, p *props, st *renderState) {
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		x := e.X.Resolve(st.rs, LengthAxisX)
		y := e.Y.Resolve(st.rs, LengthAxisY)
		w := e.W.Resolve(st.rs, LengthAxisX)
		h := e.H.Resolve(st.rs, LengthAxisY)
		if e.RX.Value > 0 || e.RY.Value > 0 {
			// Legacy single-radius rounding (max of rx/ry); distinct-axis
			// elliptical corners are a paint-batch item.
			r := e.RX.Resolve(st.rs, LengthAxisX)
			if ry := e.RY.Resolve(st.rs, LengthAxisY); ry > r {
				r = ry
			}
			dc.DrawRoundedRectangle(x, y, w, h, r)
		} else {
			dc.DrawRectangle(x, y, w, h)
		}
	})

	dc.Pop()
}

// renderEllipse renders an SVG <ellipse> element.
func renderEllipse(dc *render.Context, e *EllipseElement, a *Attrs, p *props, st *renderState) {
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		dc.DrawEllipse(e.CX.Resolve(st.rs, LengthAxisX), e.CY.Resolve(st.rs, LengthAxisY),
			e.RX.Resolve(st.rs, LengthAxisX), e.RY.Resolve(st.rs, LengthAxisY))
	})

	dc.Pop()
}

// renderLine renders an SVG <line> element. Stroke-only: with no effective
// stroke the element draws nothing (SVG semantics; fill has no area).
func renderLine(dc *render.Context, e *LineElement, a *Attrs, p *props, st *renderState) {
	if !effectiveStroke(a, p) {
		return
	}
	dc.Push()
	applyElementTransform(dc, a)

	applyStrokeAttrs(dc, p, st)
	dc.DrawLine(e.X1.Resolve(st.rs, LengthAxisX), e.Y1.Resolve(st.rs, LengthAxisY),
		e.X2.Resolve(st.rs, LengthAxisX), e.Y2.Resolve(st.rs, LengthAxisY))
	_ = dc.Stroke()

	dc.Pop()
}

// renderPolygon renders an SVG <polygon> element.
func renderPolygon(dc *render.Context, e *PolygonElement, a *Attrs, p *props, st *renderState) {
	if len(e.Points) < 4 {
		return // need at least 2 points
	}

	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		dc.ClearPath()
		drawPointsPath(dc, e.Points, true, st.rs)
	})

	dc.Pop()
}

// renderPolyline renders an SVG <polyline> element.
func renderPolyline(dc *render.Context, e *PolylineElement, a *Attrs, p *props, st *renderState) {
	if len(e.Points) < 4 {
		return
	}

	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, func() {
		dc.ClearPath()
		drawPointsPath(dc, e.Points, false, st.rs)
	})

	dc.Pop()
}

// renderUse instantiates a referenced element or symbol. Cycles and
// unresolved references were refused at parse (use.invalid).
func renderUse(dc *render.Context, e *UseElement, p *props, st *renderState) {
	if e.invalid || e.target == nil {
		return
	}
	dc.Push()
	applyElementTransform(dc, &e.Attrs)

	dx := e.X.Resolve(st.rs, LengthAxisX)
	dy := e.Y.Resolve(st.rs, LengthAxisY)
	if dx != 0 || dy != 0 {
		dc.Translate(dx, dy)
	}

	// The use element's own attributes are inherited into the referenced
	// content (SVG shadow-tree semantics); the target's own attrs win.
	useProps := evalProps(p, e.Attrs, st.doc)
	inner := *st
	inner.rs.FontSize = useProps.fontSize

	if e.kind == RefKindSymbol {
		sym, _ := e.target.(*SymbolElement)
		if sym == nil {
			dc.Pop()
			return
		}
		// Symbol viewport: use width/height (default 100% of the current
		// viewport), clipped, with the symbol's own viewBox/pAR mapping.
		vw := e.W.Resolve(st.rs, LengthAxisX)
		vh := e.H.Resolve(st.rs, LengthAxisY)
		if vw > 0 && vh > 0 {
			dc.ClipRect(0, 0, vw, vh)
		}
		if sym.HasViewBox {
			dc.Transform(viewportTransform(vw, vh, sym.ViewBox, sym.Par))
		}
		inner.rs.ViewportW, inner.rs.ViewportH = vw, vh
		renderElements(dc, sym.Children, useProps, &inner)
	} else {
		// Depth is bounded: the static reference graph was validated at
		// parse (cycles refused, depth ≤ 64).
		renderElement(dc, e.target, useProps, &inner)
	}

	dc.Pop()
}

// renderNestedSVG renders a nested <svg>: a new viewport with optional
// viewBox/pAR mapping, clipped by default.
func renderNestedSVG(dc *render.Context, e *SVGElement, p *props, st *renderState) {
	dc.Push()
	applyElementTransform(dc, &e.Attrs)

	vw := e.W.Resolve(st.rs, LengthAxisX)
	vh := e.H.Resolve(st.rs, LengthAxisY)
	dx := e.X.Resolve(st.rs, LengthAxisX)
	dy := e.Y.Resolve(st.rs, LengthAxisY)
	if dx != 0 || dy != 0 {
		dc.Translate(dx, dy)
	}

	if !e.OverflowVisible {
		if vw > 0 && vh > 0 {
			dc.ClipRect(0, 0, vw, vh)
		} else {
			dc.Pop()
			return // zero viewport: nothing to draw
		}
	}
	if e.HasViewBox {
		dc.Transform(viewportTransform(vw, vh, e.ViewBox, e.Par))
	}
	inner := *st
	inner.rs.ViewportW, inner.rs.ViewportH = vw, vh
	renderElements(dc, e.Children, p, &inner)

	dc.Pop()
}

// effectiveStroke reports whether a stroke would actually paint.
func effectiveStroke(a *Attrs, p *props) bool {
	return !a.strokeInvalid && p.stroke != "" && p.stroke != "none"
}

// fillAndStroke applies fill and/or stroke to the current path.
// The drawShape function should set up the path on the context.
func fillAndStroke(dc *render.Context, a *Attrs, p *props, st *renderState, drawShape func()) {
	hasFill := !a.fillInvalid && p.fill != "none"
	hasStroke := effectiveStroke(a, p)

	if hasFill {
		applyFillAttrs(dc, a, p, st)
		drawShape()
		if hasStroke {
			_ = dc.FillPreserve()
		} else {
			_ = dc.Fill()
		}
	}

	if hasStroke {
		applyStrokeAttrs(dc, p, st)
		if !hasFill {
			drawShape()
		}
		_ = dc.Stroke()
	}
}

// applyFillAttrs sets the fill color and fill rule on the context.
func applyFillAttrs(dc *render.Context, a *Attrs, p *props, st *renderState) {
	// Fill rule: fill-rule when explicitly set, else clip-rule (legacy
	// fallback), else nonzero.
	rule := p.fillRule
	if !p.fillRuleExplicit && p.clipRule != "" {
		rule = p.clipRule
	}
	switch rule {
	case "evenodd":
		dc.SetFillRule(render.FillRuleEvenOdd)
	default:
		dc.SetFillRule(render.FillRuleNonZero)
	}

	// Set fill color.
	if st.overrideColor != nil {
		setColorWithOpacity(dc, st.overrideColor, p.fillOpacity*p.opacity)
		return
	}

	c, err := parseColor(p.fill)
	if err != nil || c == nil {
		// Default fill is black per SVG spec.
		setColorWithOpacity(dc, color.Black, p.fillOpacity*p.opacity)
		return
	}
	setColorWithOpacity(dc, c, p.fillOpacity*p.opacity)
}

// applyStrokeAttrs sets stroke color, width, cap, join, miter and dash.
func applyStrokeAttrs(dc *render.Context, p *props, st *renderState) {
	dc.SetLineWidth(p.strokeWidth)

	switch p.strokeCap {
	case "round":
		dc.SetLineCap(render.LineCapRound)
	case "square":
		dc.SetLineCap(render.LineCapSquare)
	default:
		dc.SetLineCap(render.LineCapButt)
	}

	switch p.strokeJoin {
	case "round":
		dc.SetLineJoin(render.LineJoinRound)
	case "bevel":
		dc.SetLineJoin(render.LineJoinBevel)
	default:
		dc.SetLineJoin(render.LineJoinMiter)
	}
	dc.SetMiterLimit(p.strokeMiter)

	if len(p.dash) > 0 {
		dc.SetDash(p.dash...)
		dc.SetDashOffset(p.dashOffset)
	} else {
		dc.SetDash()
	}

	// Theming override replaces every effective stroke color ("none" never
	// reaches here — fillAndStroke gates on it).
	if st.overrideColor != nil {
		setColorWithOpacity(dc, st.overrideColor, p.strokeOpacity*p.opacity)
		return
	}

	c, err := parseColor(p.stroke)
	if err != nil || c == nil {
		return
	}
	setColorWithOpacity(dc, c, p.strokeOpacity*p.opacity)
}

// setColorWithOpacity sets the drawing color on the context, applying
// an additional opacity multiplier.
func setColorWithOpacity(dc *render.Context, c color.Color, opacity float64) {
	if opacity >= 1.0 {
		dc.SetColor(c)
		return
	}
	r, g, b, a := c.RGBA()
	// Un-premultiply, apply opacity, set as straight alpha RGBA.
	if a == 0 {
		dc.SetRGBA(0, 0, 0, 0)
		return
	}
	fa := float64(a) / 65535.0
	dc.SetRGBA(
		float64(r)/65535.0/fa,
		float64(g)/65535.0/fa,
		float64(b)/65535.0/fa,
		fa*opacity,
	)
}

// applyElementTransform applies the element's transform attribute to the context.
func applyElementTransform(dc *render.Context, a *Attrs) {
	if a.Transform != "" {
		// Errors in transforms are silently ignored (best effort).
		_ = applyTransform(dc, a.Transform)
	}
}

// drawPointsPath draws a path from alternating x,y point values.
// If closed is true, the path is closed (polygon). Otherwise it's open (polyline).
func drawPointsPath(dc *render.Context, points []Length, closed bool, rs RefSpace) {
	for i := 0; i+1 < len(points); i += 2 {
		x := points[i].Resolve(rs, LengthAxisX)
		y := points[i+1].Resolve(rs, LengthAxisY)
		if i == 0 {
			dc.MoveTo(x, y)
		} else {
			dc.LineTo(x, y)
		}
	}
	if closed {
		dc.ClosePath()
	}
}
