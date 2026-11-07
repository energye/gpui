//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package svg

import (
	"image"
	"math"
	"strings"

	"github.com/energye/gpui/render"
)

// imageRGBA aliases the concrete raster type for tile sampling.
type imageRGBA = image.RGBA

// Paint-server resolution: fill="url(#id)" / stroke="url(#id)" targets.
// The mapping onto the render brush base is exact:
//
//   - userSpaceOnUse + identity gradientTransform → native brush
//     (LinearGradientBrush / RadialGradientBrush, keeps the GPU gradient
//     ramp path: gradientExtend + linear_ramp_mask extend uniform).
//   - objectBoundingBox or non-identity gradientTransform → CustomBrush
//     wrapping the native brush with the inverse mapping (an affine-mapped
//     gradient is NOT expressible by endpoint/center transforms alone for
//     non-conformal maps — the inverse-map ColorFunc is the exact form).
//
// All failures are fail-closed: no stops / degenerate bbox / unknown target
// → no paint + warning (never black).

// paintRefID extracts the fragment id from a "url(#id)" paint value.
// Empty when the value is not a paint reference.
func paintRefID(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "url(") {
		return "", false
	}
	id := extractURLRefID(v)
	return id, id != ""
}

// resolvePaintBrush resolves a paint reference for the current element.
// bbox is the element's object bounding box in user space (objectBoundingBox
// units), rs the current reference space (viewport/units), color the
// inherited color for currentColor stops. ok=false means "paint nothing"
// (fail-closed).
func (d *Document) resolvePaintBrush(id string, bbox render.Rect, rs RefSpace, color string) (render.Brush, bool) {
	tgt := d.ElementByID(id)
	if tgt == nil {
		return nil, false
	}
	switch g := tgt.(type) {
	case *LinearGradientElement:
		return d.buildLinearBrush(g, bbox, rs, color)
	case *RadialGradientElement:
		return d.buildRadialBrush(g, bbox, rs, color)
	case *PatternElement:
		return d.buildPatternBrush(g, bbox, rs)
	}
	return nil, false
}

// gradientStopsToColorStops converts SVG stops (href chain resolved,
// currentColor resolved, offsets clamped) into render color stops.
// ok=false when the gradient has no stops (invalid paint).
func gradientStopsToColorStops(d *Document, g stopsProvider, color string) ([]render.ColorStop, bool) {
	stops := EffectiveStops(d, g)
	if len(stops) == 0 {
		return nil, false
	}
	base := &props{color: color}
	out := make([]render.ColorStop, 0, len(stops))
	for _, s := range stops {
		c, err := parseColor(resolveColorToken(s.Color, base))
		if err != nil || c == nil {
			// "none"/invalid stop color: skip the stop (the gradient stays
			// renderable when other stops exist).
			continue
		}
		r, gg, b, a := c.RGBA()
		fa := float64(a>>8) / 255 * clamp01(s.Opacity)
		out = append(out, render.ColorStop{
			Offset: clamp01(s.Offset),
			Color: render.RGBA{
				R: float64(r>>8) / 255,
				G: float64(gg>>8) / 255,
				B: float64(b>>8) / 255,
				A: fa,
			},
		})
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// gradientMappingFrom builds the user-space map for a resolved gradient's
// own coordinate space: objectBoundingBox adds T(bx,by)·S(bw,bh); both add
// gradientTransform.
func gradientMappingFrom(g interface{ unitsAndTransform() (Units, string) }, bbox render.Rect, rs RefSpace) (render.Matrix, bool) {
	m := render.Identity()
	units, transform := g.unitsAndTransform()
	if units == UnitsObjectBoundingBox {
		w := bbox.Max.X - bbox.Min.X
		h := bbox.Max.Y - bbox.Min.Y
		if w <= 0 || h <= 0 {
			return render.Matrix{}, false // degenerate bbox: paint invalid per SVG
		}
		m = render.Translate(bbox.Min.X, bbox.Min.Y).Multiply(render.Scale(w, h))
	}
	gt, err := parseTransformList(transform)
	if err != nil {
		return render.Matrix{}, false
	}
	return m.Multiply(gt), true
}

// gradientCoordsIn resolves a gradient geometry length in its own space:
// userSpaceOnUse resolves against the viewport/units; objectBoundingBox uses
// the authored fraction as-is.
func gradientCoordsIn(g interface{ unitsAndTransform() (Units, string) }, l Length, axis LengthAxis, rs RefSpace) float64 {
	units, _ := g.unitsAndTransform()
	if units == UnitsUserSpaceOnUse {
		return l.Resolve(rs, axis)
	}
	return l.Value
}

func (d *Document) buildLinearBrush(g *LinearGradientElement, bbox render.Rect, rs RefSpace, color string) (render.Brush, bool) {
	// Per-attribute href inheritance (SVG 1.1): unset attributes come from
	// the referenced gradient, first-set-wins along the chain.
	e := *g // shallow copy as the accumulation base
	x1s, y1s, x2s, y2s := g.setX1, g.setY1, g.setX2, g.setY2
	us, ts, sp := g.Units != UnitsObjectBoundingBox || g.unitsSet, g.Transform != "", g.spreadSet
	cur := g
	seen := map[Element]bool{g: true}
	for {
		if !x1s && cur.setX1 {
			e.X1 = cur.X1
			x1s = true
		}
		if !y1s && cur.setY1 {
			e.Y1 = cur.Y1
			y1s = true
		}
		if !x2s && cur.setX2 {
			e.X2 = cur.X2
			x2s = true
		}
		if !y2s && cur.setY2 {
			e.Y2 = cur.Y2
			y2s = true
		}
		if !us && cur.unitsSet {
			e.Units = cur.Units
			us = true
		}
		if !ts && cur.Transform != "" {
			e.Transform = cur.Transform
			ts = true
		}
		if !sp && cur.spreadSet {
			e.Spread = cur.Spread
			sp = true
		}
		if cur.HRef == "" {
			break
		}
		tgt := d.ElementByID(strings.TrimPrefix(cur.HRef, "#"))
		nl, ok := tgt.(*LinearGradientElement)
		if !ok || seen[nl] {
			break
		}
		seen[nl] = true
		cur = nl
	}

	stops, ok := gradientStopsToColorStops(d, &e, color)
	if !ok {
		return nil, false
	}
	mapping, ok := gradientMappingFrom(&e, bbox, rs)
	if !ok {
		return nil, false
	}
	brush := render.NewLinearGradientBrush(
		gradientCoordsIn(&e, e.X1, LengthAxisX, rs),
		gradientCoordsIn(&e, e.Y1, LengthAxisY, rs),
		gradientCoordsIn(&e, e.X2, LengthAxisX, rs),
		gradientCoordsIn(&e, e.Y2, LengthAxisY, rs),
	)
	brush.Stops = stops
	brush.Extend = e.Spread.extendOf()
	return wrapBrushIfMapped(brush, mapping), true
}

func (d *Document) buildRadialBrush(g *RadialGradientElement, bbox render.Rect, rs RefSpace, color string) (render.Brush, bool) {
	e := *g
	cxs, cys, rr, fxs, fys := g.setCX, g.setCY, g.setR, g.setFX, g.setFY
	us, ts, sp := g.unitsSet, g.Transform != "", g.spreadSet
	cur := g
	seen := map[Element]bool{g: true}
	for {
		if !cxs && cur.setCX {
			e.CX = cur.CX
			cxs = true
		}
		if !cys && cur.setCY {
			e.CY = cur.CY
			cys = true
		}
		if !rr && cur.setR {
			e.R = cur.R
			rr = true
		}
		if !fxs && cur.setFX {
			e.FX = cur.FX
			fxs = true
		}
		if !fys && cur.setFY {
			e.FY = cur.FY
			fys = true
		}
		if !us && cur.unitsSet {
			e.Units = cur.Units
			us = true
		}
		if !ts && cur.Transform != "" {
			e.Transform = cur.Transform
			ts = true
		}
		if !sp && cur.spreadSet {
			e.Spread = cur.Spread
			sp = true
		}
		if cur.HRef == "" {
			break
		}
		tgt := d.ElementByID(strings.TrimPrefix(cur.HRef, "#"))
		nr, ok := tgt.(*RadialGradientElement)
		if !ok || seen[nr] {
			break
		}
		seen[nr] = true
		cur = nr
	}

	stops, ok := gradientStopsToColorStops(d, &e, color)
	if !ok {
		return nil, false
	}
	mapping, ok := gradientMappingFrom(&e, bbox, rs)
	if !ok {
		return nil, false
	}
	cx := gradientCoordsIn(&e, e.CX, LengthAxisX, rs)
	cy := gradientCoordsIn(&e, e.CY, LengthAxisY, rs)
	r := gradientCoordsIn(&e, e.R, LengthAxisDiagonal, rs)
	brush := render.NewRadialGradientBrush(cx, cy, 0, r)
	brush.Stops = stops
	brush.Extend = e.Spread.extendOf()
	if e.HasFocus {
		fx := gradientCoordsIn(&e, e.FX, LengthAxisX, rs)
		fy := gradientCoordsIn(&e, e.FY, LengthAxisY, rs)
		if fx != cx || fy != cy {
			brush.SetFocus(fx, fy)
		}
	}
	return wrapBrushIfMapped(brush, mapping), true
}

// wrapBrushIfMapped returns the native brush when the mapping is identity,
// otherwise an exact inverse-mapping CustomBrush.
func wrapBrushIfMapped(brush render.Brush, mapping render.Matrix) render.Brush {
	if mapping == render.Identity() {
		return brush
	}
	if singularMatrix(mapping) {
		return nil // singular mapping: paint invalid
	}
	minv := mapping.Invert()
	return render.NewCustomBrush(func(x, y float64) render.RGBA {
		p := minv.TransformPoint(render.Point{X: x, Y: y})
		return colorAtOfBrush(brush, p.X, p.Y)
	}).WithName("svg_gradient")
}

// singularMatrix reports a near-singular affine (render.Matrix.Invert
// silently returns identity for |det|<1e-10 — detect before relying on it).
func singularMatrix(m render.Matrix) bool {
	det := m.A*m.E - m.B*m.D
	return det > -1e-10 && det < 1e-10
}

// colorAtOfBrush evaluates a native gradient brush at a point (the gradient
// brushes implement ColorAt directly).
func colorAtOfBrush(b render.Brush, x, y float64) render.RGBA {
	switch g := b.(type) {
	case *render.LinearGradientBrush:
		return g.ColorAt(x, y)
	case *render.RadialGradientBrush:
		return g.ColorAt(x, y)
	case render.CustomBrush:
		return g.Func(x, y)
	}
	return render.RGBA{}
}

// isPaintServer reports whether an element can back a url() paint.
func isPaintServer(e Element) bool {
	switch e.(type) {
	case *LinearGradientElement, *RadialGradientElement, *PatternElement:
		return true
	}
	return false
}

// buildPatternBrush renders the pattern tile offscreen once and wraps it in
// a tiled CustomBrush. patternTransform maps tile space to user space; the
// sampling function evaluates through its inverse (mod the tile size).
func (d *Document) buildPatternBrush(p *PatternElement, bbox render.Rect, rs RefSpace) (render.Brush, bool) {
	// href chain: per-attribute first-set-wins (children from the first
	// pattern that has any), same discipline as gradients.
	e := *p
	xs, ys, ws, hs := p.setX, p.setY, p.setW, p.setH
	us, cus, ts := p.unitsSet, p.contentUnitsSet, p.Transform != ""
	cur := p
	seen := map[Element]bool{p: true}
	for len(cur.Children) == 0 {
		if cur.HRef == "" {
			return nil, false
		}
		tgt := d.ElementByID(strings.TrimPrefix(cur.HRef, "#"))
		np, ok := tgt.(*PatternElement)
		if !ok || seen[np] {
			return nil, false
		}
		seen[np] = true
		if !xs && np.setX {
			e.X = np.X
			xs = true
		}
		if !ys && np.setY {
			e.Y = np.Y
			ys = true
		}
		if !ws && np.setW {
			e.W = np.W
			ws = true
		}
		if !hs && np.setH {
			e.H = np.H
			hs = true
		}
		if !us && np.unitsSet {
			e.Units = np.Units
			us = true
		}
		if !cus && np.contentUnitsSet {
			e.ContentUnits = np.ContentUnits
			cus = true
		}
		if !ts && np.Transform != "" {
			e.Transform = np.Transform
			ts = true
		}
		if !e.HasViewBox && np.HasViewBox {
			e.ViewBox = np.ViewBox
			e.HasViewBox = true
		}
		cur = np
	}
	tileW := gradientCoordsIn(&e, e.W, LengthAxisX, rs)
	tileH := gradientCoordsIn(&e, e.H, LengthAxisY, rs)
	originX := gradientCoordsIn(&e, e.X, LengthAxisX, rs)
	originY := gradientCoordsIn(&e, e.Y, LengthAxisY, rs)
	if e.Units == UnitsObjectBoundingBox {
		w := bbox.Max.X - bbox.Min.X
		h := bbox.Max.Y - bbox.Min.Y
		if w <= 0 || h <= 0 {
			return nil, false
		}
		tileW *= w
		tileH *= h
		originX = bbox.Min.X + originX*w
		originY = bbox.Min.Y + originY*h
	}
	if tileW <= 0 || tileH <= 0 {
		return nil, false
	}

	// Tile offscreen (user-space content, shifted so the tile origin lands
	// at (0,0); a viewBox remaps the content into the tile box).
	tw, th := int(tileW), int(tileH)
	if tw < 1 || th < 1 {
		return nil, false
	}
	tc := render.NewContext(tw, th)
	if e.Units == UnitsUserSpaceOnUse {
		tc.Translate(-originX, -originY)
	}
	if e.HasViewBox {
		tc.Transform(viewportTransform(float64(tw), float64(th), e.ViewBox, e.Par))
	}
	tileProps := evalProps(rootProps(), e.Attrs, d)
	tileSt := renderState{doc: d, rs: rs}
	renderElements(tc, cur.Children, tileProps, &tileSt)
	tileImg, ok := tc.Image().(*imageRGBA)
	if !ok || tileImg == nil {
		return nil, false
	}

	// Sampling map: user point → tile space (patternTransform inverse),
	// then mod the tile size relative to the tile origin.
	m := render.Identity()
	if e.Transform != "" {
		pt, err := parseTransformList(e.Transform)
		if err != nil {
			return nil, false
		}
		m = pt
	}
	var minv render.Matrix
	if m != render.Identity() {
		if singularMatrix(m) {
			return nil, false
		}
		minv = m.Invert()
	}
	iw, ih := float64(tw), float64(th)
	return render.NewCustomBrush(func(x, y float64) render.RGBA {
		if minv != (render.Matrix{}) {
			q := minv.TransformPoint(render.Point{X: x, Y: y})
			x, y = q.X, q.Y
		}
		u := math.Mod(x-originX, iw)
		if u < 0 {
			u += iw
		}
		v := math.Mod(y-originY, ih)
		if v < 0 {
			v += ih
		}
		px, py := int(u), int(v)
		if px < 0 {
			px = 0
		} else if px >= tw {
			px = tw - 1
		}
		if py < 0 {
			py = 0
		} else if py >= th {
			py = th - 1
		}
		c := tileImg.RGBAAt(px, py)
		return render.RGBA{
			R: float64(c.R) / 255,
			G: float64(c.G) / 255,
			B: float64(c.B) / 255,
			A: float64(c.A) / 255,
		}
	}).WithName("svg_pattern"), true
}
