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
	"math"

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
// Wrapping order per SVG rendering model: mask(clip(body)) — the mask
// applies to the already-clipped result (fail-closed: unresolved references
// render nothing).
func renderElement(dc *render.Context, elem Element, parent *props, st *renderState) {
	if elem == nil {
		return
	}
	a := elem.attrs()
	if a.Mask != "" {
		if a.maskInvalid {
			return // unresolved/wrong-type mask reference: not rendered
		}
		if id, isRef := paintRefID(a.Mask); isRef {
			renderMasked(dc, elem, a, id, parent, st)
			return
		}
	}
	renderClipped(dc, elem, a, parent, st)
}

// renderClipped wraps the element body with its clip-path (if any).
func renderClipped(dc *render.Context, elem Element, a *Attrs, parent *props, st *renderState) {
	if a.ClipPath != "" {
		if a.clipInvalid {
			return // unresolved/wrong-type clip reference: not rendered
		}
		if id, isRef := paintRefID(a.ClipPath); isRef {
			if bbox, hasBBox := elementBBox(elem, st); hasBBox {
				if clip, rule, ok := st.doc.clipRegionFor(id, bbox, st.rs); ok {
					dc.Push()
					dc.SetFillRule(rule)
					dc.DrawPath(clip)
					dc.Clip()
					renderElementBody(dc, elem, a, parent, st)
					dc.Pop()
					return
				}
			}
			return // degenerate bbox or empty clip region: nothing visible
		}
	}
	renderElementBody(dc, elem, a, parent, st)
}

// renderMasked composes the element through a luminance mask using three
// user-space offscreens (mask content → LuminanceMask → element content →
// ApplyMask → DrawImage back). All composition happens in user space, so
// any enclosing CTM applies once at the final DrawImage (same shape as
// Skia saveLayer / Flutter user-space masks).
func renderMasked(dc *render.Context, elem Element, a *Attrs, id string, parent *props, st *renderState) {
	mt := st.doc.ElementByID(id)
	m, ok := mt.(*MaskElement)
	if !ok {
		return
	}
	bbox, hasBBox := elementBBox(elem, st)
	if !hasBBox {
		return
	}
	if m.ContentUnits == UnitsObjectBoundingBox {
		// Rare content-unit form: named refusal (registered).
		return
	}
	bw := bbox.Max.X - bbox.Min.X
	bh := bbox.Max.Y - bbox.Min.Y
	regionCoord := func(l Length, axis LengthAxis, min float64, dim float64) float64 {
		if m.Units == UnitsUserSpaceOnUse {
			return l.Resolve(st.rs, axis)
		}
		frac := l.Value
		if l.Unit == LengthUnitPercent {
			frac = l.Value / 100
		}
		return min + frac*dim
	}
	rx := regionCoord(m.X, LengthAxisX, bbox.Min.X, bw)
	ry := regionCoord(m.Y, LengthAxisY, bbox.Min.Y, bh)
	rw := regionCoord(m.W, LengthAxisX, bbox.Min.X, bw) - rx
	rh := regionCoord(m.H, LengthAxisY, bbox.Min.Y, bh) - ry
	if rw <= 0 || rh <= 0 {
		return // empty mask region: everything hidden
	}

	// Offscreen A: mask content (luminance source).
	ca := render.NewContext(int(rw), int(rh))
	ca.Translate(-rx, -ry)
	maskProps := evalProps(parent, m.Attrs, st.doc)
	maskSt := *st
	maskSt.rs.FontSize = maskProps.fontSize
	renderElements(ca, m.Children, maskProps, &maskSt)
	lm := render.NewLuminanceMask(ca.Image())
	if lm == nil {
		return
	}

	// Offscreen B: the element content (clip and its own rendering inside).
	cb := render.NewContext(int(rw), int(rh))
	cb.Translate(-rx, -ry)
	renderClipped(cb, elem, a, parent, st)
	cb.ApplyMask(lm)

	// Composite back at the region origin (user space).
	img := cb.Image()
	if img == nil {
		return
	}
	dc.DrawImage(render.ImageBufFromImage(img), rx, ry)
}

// renderElementBody is renderElement's dispatch (after clip handling).
func renderElementBody(dc *render.Context, elem Element, a *Attrs, parent *props, st *renderState) {
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
	case *ImageElement:
		renderImageElement(dc, e, a, p, &child)
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

	fillAndStroke(dc, a, p, st, path.BoundingBox(), func() {
		dc.DrawPath(path)
	})

	dc.Pop()
}

// renderCircle renders an SVG <circle> element.
func renderCircle(dc *render.Context, e *CircleElement, a *Attrs, p *props, st *renderState) {
	cx := e.CX.Resolve(st.rs, LengthAxisX)
	cy := e.CY.Resolve(st.rs, LengthAxisY)
	r := e.R.Resolve(st.rs, LengthAxisDiagonal)
	bbox := render.Rect{Min: render.Point{X: cx - r, Y: cy - r}, Max: render.Point{X: cx + r, Y: cy + r}}
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, bbox, func() {
		dc.DrawCircle(cx, cy, r)
	})

	dc.Pop()
}

// renderRect renders an SVG <rect> element.
func renderRect(dc *render.Context, e *RectElement, a *Attrs, p *props, st *renderState) {
	x := e.X.Resolve(st.rs, LengthAxisX)
	y := e.Y.Resolve(st.rs, LengthAxisY)
	w := e.W.Resolve(st.rs, LengthAxisX)
	h := e.H.Resolve(st.rs, LengthAxisY)
	bbox := render.Rect{Min: render.Point{X: x, Y: y}, Max: render.Point{X: x + w, Y: y + h}}
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, bbox, func() {
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
	cx := e.CX.Resolve(st.rs, LengthAxisX)
	cy := e.CY.Resolve(st.rs, LengthAxisY)
	rx := e.RX.Resolve(st.rs, LengthAxisX)
	ry := e.RY.Resolve(st.rs, LengthAxisY)
	bbox := render.Rect{Min: render.Point{X: cx - rx, Y: cy - ry}, Max: render.Point{X: cx + rx, Y: cy + ry}}
	dc.Push()
	applyElementTransform(dc, a)

	fillAndStroke(dc, a, p, st, bbox, func() {
		dc.DrawEllipse(cx, cy, rx, ry)
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

	if !applyStrokeAttrs(dc, p, st, render.Rect{}) {
		dc.Pop()
		return
	}
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

	fillAndStroke(dc, a, p, st, pointsBBox(e.Points, st.rs), func() {
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

	fillAndStroke(dc, a, p, st, pointsBBox(e.Points, st.rs), func() {
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

// renderImageElement renders <image>: data-URI raster into its viewport
// with preserveAspectRatio (reuse of viewportTransform: the viewport is
// x/y/w/h and the "viewBox" is the intrinsic image size). Slices clip to
// the viewport; the default width/height is the intrinsic size (SVG 2).
func renderImageElement(dc *render.Context, e *ImageElement, a *Attrs, p *props, st *renderState) {
	if !e.HasImage {
		return // refused/undecodable at parse (warning already recorded)
	}
	vw := e.W.Resolve(st.rs, LengthAxisX)
	vh := e.H.Resolve(st.rs, LengthAxisY)
	if vw <= 0 || vh <= 0 {
		vw, vh = float64(e.ImgW), float64(e.ImgH) // intrinsic default
	}
	if vw <= 0 || vh <= 0 {
		return
	}
	x := e.X.Resolve(st.rs, LengthAxisX)
	y := e.Y.Resolve(st.rs, LengthAxisY)

	dc.Push()
	applyElementTransform(dc, a)
	dc.Translate(x, y)
	// Viewport clip (slice overflow must not leak).
	dc.ClipRect(0, 0, vw, vh)
	// Map the intrinsic image rect into the viewport with pAR semantics.
	dc.Transform(viewportTransform(vw, vh, ViewBox{0, 0, float64(e.ImgW), float64(e.ImgH)}, e.Par))
	// Decode lazily at render from the stored raw bytes (cache-friendly: the
	// parse keeps the payload; here we re-render from the decoded raster).
	if buf := e.renderBuf(); buf != nil {
		defer func() {
			buf.Dispose()
		}()
		dc.DrawImage(buf, 0, 0)
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
// The drawShape function should set up the path on the context; bbox is the
// element's object bounding box in user space (objectBoundingBox paints).
func fillAndStroke(dc *render.Context, a *Attrs, p *props, st *renderState, bbox render.Rect, drawShape func()) {
	hasFill := !a.fillInvalid && p.fill != "none"
	hasStroke := effectiveStroke(a, p)

	if hasFill {
		hasFill = applyFillAttrs(dc, a, p, st, bbox)
	}
	if hasFill {
		drawShape()
		if hasStroke {
			_ = dc.FillPreserve()
		} else {
			_ = dc.Fill()
		}
	}

	if hasStroke {
		hasStroke = applyStrokeAttrs(dc, p, st, bbox)
	}
	if hasStroke {
		if !hasFill {
			drawShape()
		}
		_ = dc.Stroke()
	}
}

// elementBBox computes an element's object bounding box in user space
// (objectBoundingBox paints / clip units). Groups union their children.
func elementBBox(elem Element, st *renderState) (render.Rect, bool) {
	switch e := elem.(type) {
	case *PathElement:
		if e.D == "" {
			return render.Rect{}, false
		}
		p, err := render.ParseSVGPath(e.D)
		if err != nil {
			return render.Rect{}, false
		}
		return p.BoundingBox(), true
	case *RectElement:
		x, y := e.X.Resolve(st.rs, LengthAxisX), e.Y.Resolve(st.rs, LengthAxisY)
		w, h := e.W.Resolve(st.rs, LengthAxisX), e.H.Resolve(st.rs, LengthAxisY)
		return render.Rect{Min: render.Point{X: x, Y: y}, Max: render.Point{X: x + w, Y: y + h}}, true
	case *CircleElement:
		cx, cy := e.CX.Resolve(st.rs, LengthAxisX), e.CY.Resolve(st.rs, LengthAxisY)
		r := e.R.Resolve(st.rs, LengthAxisDiagonal)
		return render.Rect{Min: render.Point{X: cx - r, Y: cy - r}, Max: render.Point{X: cx + r, Y: cy + r}}, true
	case *EllipseElement:
		cx, cy := e.CX.Resolve(st.rs, LengthAxisX), e.CY.Resolve(st.rs, LengthAxisY)
		rx, ry := e.RX.Resolve(st.rs, LengthAxisX), e.RY.Resolve(st.rs, LengthAxisY)
		return render.Rect{Min: render.Point{X: cx - rx, Y: cy - ry}, Max: render.Point{X: cx + rx, Y: cy + ry}}, true
	case *LineElement:
		x1, y1 := e.X1.Resolve(st.rs, LengthAxisX), e.Y1.Resolve(st.rs, LengthAxisY)
		x2, y2 := e.X2.Resolve(st.rs, LengthAxisX), e.Y2.Resolve(st.rs, LengthAxisY)
		return render.Rect{Min: render.Point{X: math.Min(x1, x2), Y: math.Min(y1, y2)}, Max: render.Point{X: math.Max(x1, x2), Y: math.Max(y1, y2)}}, true
	case *PolygonElement:
		return pointsBBox(e.Points, st.rs), true
	case *PolylineElement:
		return pointsBBox(e.Points, st.rs), true
	case *GroupElement:
		out, ok := render.Rect{}, false
		for _, c := range e.Children {
			if cb, cok := elementBBox(c, st); cok {
				if !ok {
					out, ok = cb, true
					continue
				}
				if cb.Min.X < out.Min.X {
					out.Min.X = cb.Min.X
				}
				if cb.Min.Y < out.Min.Y {
					out.Min.Y = cb.Min.Y
				}
				if cb.Max.X > out.Max.X {
					out.Max.X = cb.Max.X
				}
				if cb.Max.Y > out.Max.Y {
					out.Max.Y = cb.Max.Y
				}
			}
		}
		return out, ok
	}
	return render.Rect{}, false
}

// pointsBBox computes the object bounding box of a points list.
func pointsBBox(points []Length, rs RefSpace) render.Rect {
	first := true
	var minX, minY, maxX, maxY float64
	for i := 0; i+1 < len(points); i += 2 {
		x := points[i].Resolve(rs, LengthAxisX)
		y := points[i+1].Resolve(rs, LengthAxisY)
		if first {
			minX, maxX, minY, maxY = x, x, y, y
			first = false
			continue
		}
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}
	return render.Rect{Min: render.Point{X: minX, Y: minY}, Max: render.Point{X: maxX, Y: maxY}}
}

// applyFillAttrs sets the fill color (or paint brush) and fill rule on the
// context. Returns false when the fill cannot resolve (fail-closed skip).
func applyFillAttrs(dc *render.Context, a *Attrs, p *props, st *renderState, bbox render.Rect) bool {
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

	// Set fill color (or paint server brush).
	if st.overrideColor != nil {
		setColorWithOpacity(dc, st.overrideColor, p.fillOpacity*p.opacity)
		return true
	}
	if id, isRef := paintRefID(p.fill); isRef {
		brush, ok := st.doc.resolvePaintBrush(id, bbox, st.rs, p.color)
		if !ok {
			return false
		}
		dc.SetFillBrush(brush)
		return true
	}

	c, err := parseColor(p.fill)
	if err != nil || c == nil {
		// Default fill is black per SVG spec.
		setColorWithOpacity(dc, color.Black, p.fillOpacity*p.opacity)
		return true
	}
	setColorWithOpacity(dc, c, p.fillOpacity*p.opacity)
	return true
}

// applyStrokeAttrs sets stroke color (or paint brush), width, cap, join,
// miter and dash. Returns false when the stroke cannot resolve.
func applyStrokeAttrs(dc *render.Context, p *props, st *renderState, bbox render.Rect) bool {
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
		return true
	}
	if id, isRef := paintRefID(p.stroke); isRef {
		brush, ok := st.doc.resolvePaintBrush(id, bbox, st.rs, p.color)
		if !ok {
			return false
		}
		dc.SetStrokeBrush(brush)
		return true
	}

	c, err := parseColor(p.stroke)
	if err != nil || c == nil {
		return false
	}
	setColorWithOpacity(dc, c, p.strokeOpacity*p.opacity)
	return true
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
