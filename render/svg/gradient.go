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
	"strconv"
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
func (d *Document) resolvePaintBrush(id string, bbox render.Rect, rs RefSpace, color string, depth int) (render.Brush, bool) {
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
		return d.buildPatternBrushDepth(g, bbox, rs, depth)
	}
	return nil, false
}

// stopNorm is one normalized stop (usvg Stop): offset clamped to [0,1],
// color premultiplied-ready components, opacity folded in later.
type stopNorm struct {
	offset  float64
	r, g, b float64 // 0..1 straight colors
	a       float64 // stop-opacity (0..1)
}

// gradientStopsToColorStops converts the owning gradient's stops into
// render color stops with usvg convert_stops semantics applied verbatim:
//
//   - offset missing/invalid/other-unit → previous stop's resolved number
//     (first stop → 0); percent → /100; clamp [0,1] AFTER prev tracking;
//   - stop-color: currentColor → inherited color; inherit → the owning
//     gradient's stop-color (one level, usvg resolve_inherit); invalid →
//     black + warning; missing → black;
//   - stop-opacity: inherit → the owning gradient's stop-opacity; missing
//     or invalid → 1;
//   - post-passes: three consecutive equal offsets drop the middle one,
//     zero-offset pairs get an epsilon bump, equal offsets shift apart.
//
// ok=false when there are no stops at all (invalid paint, usvg
// find_gradient_with_stops → None).
func gradientStopsToColorStops(d *Document, stops []*GradientStopElement, owner Attrs, color string) ([]render.ColorStop, bool) {
	if len(stops) == 0 {
		return nil, false
	}
	base := &props{color: color}
	norm := make([]stopNorm, 0, len(stops))
	prev := 0.0
	for _, st := range stops {
		offset := prev
		if st.OffsetSet {
			if v, ok := parseStopOffset(st.OffsetRaw); ok {
				offset = v
			}
		}
		prev = offset
		offset = clamp01(offset)

		colorToken := st.Color
		switch {
		case colorToken == "inherit":
			// One level into the owning gradient (usvg resolve_inherit at
			// tree build); a missing parent value falls back to black.
			colorToken = owner.Raw["stop-color"]
			if colorToken == "" || colorToken == "inherit" {
				colorToken = "black"
			}
		case colorToken == "":
			colorToken = "black"
		}
		c, err := parseColor(resolveColorToken(resolveVarTokens(d, colorToken), base))
		if err != nil || c == nil {
			pcWarnStopColor(d, colorToken)
			c, _ = parseColor("black")
		}
		r, gg, b, a := c.RGBA()

		opacity := 1.0
		if st.OpacitySet {
			switch st.OpacityRaw {
			case "inherit":
				if v := owner.Raw["stop-opacity"]; v != "" && v != "inherit" {
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						opacity = clamp01(f)
					}
				}
			default:
				if f, err := strconv.ParseFloat(st.OpacityRaw, 64); err == nil {
					opacity = clamp01(f)
				}
			}
		}

		norm = append(norm, stopNorm{
			offset: offset,
			r:      float64(r>>8) / 255,
			g:      float64(gg>>8) / 255,
			b:      float64(b>>8) / 255,
			a:      float64(a>>8) / 255 * opacity,
		})
	}
	normalizeStopOffsets(norm)

	out := make([]render.ColorStop, 0, len(norm))
	for _, st := range norm {
		out = append(out, render.ColorStop{
			Offset: st.offset,
			Color:  render.RGBA{R: st.r, G: st.g, B: st.b, A: st.a},
		})
	}
	return out, true
}

// pcWarnStopColor surfaces an invalid stop-color once per document parse —
// the warning path needs a parse context, which paint-time conversion does
// not carry; the fail-closed value (black) is what usvg logs too.
func pcWarnStopColor(d *Document, value string) {
	d.Warnings = append(d.Warnings, ParseWarning{
		Element: "stop", Attribute: "stop-color", Reason: warnInvalidValue,
		Detail: "failed to parse stop-color value: " + value,
	})
}

// parseStopOffset parses one offset token: number or percent; anything
// else (other units, invalid) falls back to the previous offset (ok=false).
func parseStopOffset(raw string) (float64, bool) {
	if raw == "" {
		return 0, false
	}
	if strings.HasSuffix(raw, "%") {
		f, err := strconv.ParseFloat(strings.TrimSuffix(raw, "%"), 64)
		if err != nil {
			return 0, false
		}
		return f / 100, true
	}
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// normalizeStopOffsets applies the usvg offset post-passes verbatim:
// three consecutive equal offsets drop the middle (ULPS 4), adjacent zero
// offsets get an epsilon bump, equal (or decreasing) neighbors shift the
// earlier one down by an epsilon.
func normalizeStopOffsets(stops []stopNorm) {
	eq := func(a, b float64) bool { return approxEqUlpsF32(a, b, 4) }
	if len(stops) >= 3 {
		i := 0
		for i < len(stops)-2 {
			if eq(stops[i].offset, stops[i+1].offset) && eq(stops[i+1].offset, stops[i+2].offset) {
				stops = append(stops[:i+1], stops[i+2:]...)
			} else {
				i++
			}
		}
	}
	if len(stops) >= 2 {
		for i := 0; i < len(stops)-1; i++ {
			if eq(stops[i].offset, 0) && eq(stops[i+1].offset, 0) {
				stops[i+1].offset = clamp01(stops[i].offset + f32Epsilon)
			}
		}
	}
	for i := 1; i < len(stops); i++ {
		prev, cur := stops[i-1].offset, stops[i].offset
		if prev > cur || eq(prev, cur) {
			stops[i-1].offset = clamp01(prev - f32Epsilon)
			stops[i].offset = clamp01(prev)
		}
	}
}

const f32Epsilon = 1.1920929e-7 // math float32 epsilon (usvg f32::EPSILON)

// approxEqUlpsF32 compares two values as float32s within ulps ULPs
// (usvg approx_eq_ulps(N) on f32 offsets).
func approxEqUlpsF32(af, bf float64, ulps int) bool {
	ai := int32(math.Float32bits(float32(af)))
	bi := int32(math.Float32bits(float32(bf)))
	if ai < 0 != (bi < 0) {
		// Different signs: only both-zero (±0) are near.
		return af == 0 && bf == 0
	}
	d := ai - bi
	if d < 0 {
		d = -d
	}
	return d <= int32(ulps)
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
	if transform != "" {
		gt, err := parseTransformList(transform)
		if err != nil || singularMatrix(gt) {
			// Invalid or singular gradientTransform falls back to identity
			// (usvg: unparseable attribute → default transform; resvg
			// renders the untransformed gradient — reference-probed).
			gt = render.Identity()
		}
		m = m.Multiply(gt)
	}
	return m, true
}

// gradientCoordsIn resolves a gradient geometry length in its own space:
// userSpaceOnUse resolves against the viewport/units; objectBoundingBox uses
// the authored fraction as-is.
func gradientCoordsIn(g interface{ unitsAndTransform() (Units, string) }, l Length, axis LengthAxis, rs RefSpace) float64 {
	units, _ := g.unitsAndTransform()
	if units == UnitsUserSpaceOnUse {
		return l.Resolve(rs, axis)
	}
	// objectBoundingBox coordinates are fractions of the bbox: percentages
	// divide by 100 (a bare "50%" used to pass through as 50 — 50 bbox
	// widths away, pushing every sample past the last stop).
	v := l.Value
	if l.Unit == LengthUnitPercent {
		v /= 100
	}
	return v
}

// gradientChain walks self-first along xlink:href (usvg href_iter):
// self/origin references stop the walk with a warning; every kind of
// element may appear (callers type-check per usvg rules).
func (d *Document) gradientChain(start Element) []Element {
	out := []Element{start}
	seen := map[Element]bool{start: true}
	cur := start
	for {
		var href string
		if a := cur.attrs(); a != nil {
			href = a.Raw["href"]
			if href == "" {
				href = a.Raw["xlink:href"]
			}
		}
		if href == "" {
			return out
		}
		tgt := d.ElementByID(strings.TrimPrefix(strings.TrimSpace(href), "#"))
		if tgt == nil || seen[tgt] {
			return out // dangling or cycle: chain ends here
		}
		out = append(out, tgt)
		seen[tgt] = true
		cur = tgt
	}
}

// gradientAttrSource walks the chain for one attribute per usvg
// resolve_lg_attr/resolve_rg_attr: coordinates only from same-type
// gradients, units/spread/transform from any gradient kind. Returns the
// raw value and whether any chain element carries it.
func gradientAttrSource(chain []Element, self Element, attr string, sameTypeOnly bool) (string, bool) {
	for _, e := range chain {
		accepted := true
		if sameTypeOnly {
			// Coordinates resolve only from a gradient of the same kind.
			switch self.(type) {
			case *LinearGradientElement:
				_, accepted = e.(*LinearGradientElement)
			case *RadialGradientElement:
				_, accepted = e.(*RadialGradientElement)
			}
		} else {
			switch e.(type) {
			case *LinearGradientElement, *RadialGradientElement:
			default:
				accepted = false // any-gradient attributes stop at non-gradients
			}
		}
		if !accepted {
			return "", false // usvg `_ => break`: the chain stops here
		}
		a := e.attrs()
		if a == nil {
			continue
		}
		if v, ok := a.Raw[attr]; ok {
			return v, true
		}
	}
	return "", false
}

// gradientStopsFromChain walks the chain (self first) for the first
// gradient with stop children (usvg find_gradient_with_stops); a
// non-gradient link fails the whole paint.
func gradientStopsFromChain(chain []Element) ([]*GradientStopElement, Attrs, bool) {
	for _, e := range chain {
		var lp *LinearGradientElement
		var rp *RadialGradientElement
		switch g := e.(type) {
		case *LinearGradientElement:
			lp = g
		case *RadialGradientElement:
			rp = g
		default:
			// usvg: non-gradient in the href chain fails stop lookup.
			return nil, Attrs{}, false
		}
		if lp != nil && len(lp.Stops) > 0 {
			return lp.Stops, lp.Attrs, true
		}
		if rp != nil && len(rp.Stops) > 0 {
			return rp.Stops, rp.Attrs, true
		}
	}
	return nil, Attrs{}, false
}

// unitsFromChain resolves gradientUnits (any gradient kind, default oBB).
func unitsFromChain(chain []Element) Units {
	if v, ok := gradientAttrSource(chain, chain[0], "gradientUnits", false); ok {
		if strings.TrimSpace(v) == "userSpaceOnUse" {
			return UnitsUserSpaceOnUse
		}
		return UnitsObjectBoundingBox
	}
	return UnitsObjectBoundingBox
}

func (d *Document) buildLinearBrush(g *LinearGradientElement, bbox render.Rect, rs RefSpace, color string) (render.Brush, bool) {
	chain := d.gradientChain(g)

	stopsRaw, owner, ok := gradientStopsFromChain(chain)
	if !ok {
		return nil, false
	}
	stops, ok := gradientStopsToColorStops(d, stopsRaw, owner, color)
	if !ok {
		return nil, false
	}
	// usvg stops_to_color: a single stop paints a solid color.
	if len(stops) == 1 {
		return solidStopBrush(stops[0]), true
	}

	e := LinearGradientElement{Attrs: g.Attrs, Units: UnitsObjectBoundingBox}
	e.Units = unitsFromChain(chain)
	if v, ok := gradientAttrSource(chain, g, "x1", true); ok {
		e.X1 = parseGradientLength(v)
	}
	if v, ok := gradientAttrSource(chain, g, "y1", true); ok {
		e.Y1 = parseGradientLength(v)
	}
	if v, ok := gradientAttrSource(chain, g, "x2", true); ok {
		e.X2 = parseGradientLength(v)
	} else {
		e.X2 = Length{Value: 100, Unit: LengthUnitPercent}
	}
	if v, ok := gradientAttrSource(chain, g, "y2", true); ok {
		e.Y2 = parseGradientLength(v)
	}
	if v, ok := gradientAttrSource(chain, g, "spreadMethod", false); ok {
		e.Spread = spreadToken(v)
		e.spreadSet = true
	}
	if v, ok := gradientAttrSource(chain, g, "gradientTransform", false); ok {
		e.Transform = v
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
	if wrapped := wrapBrushIfMapped(brush, mapping); wrapped != nil {
		return wrapped, true
	}
	return nil, false
}

// solidStopBrush paints everything with one stop's color (usvg
// stops_to_color / the zero-radius rule).
func solidStopBrush(st render.ColorStop) render.Brush {
	b := render.NewCustomBrush(func(x, y float64) render.RGBA {
		return st.Color
	}).WithName("svg_solid_stop")
	return b
}

// parseGradientLength parses one gradient geometry value (number or
// percent); invalid values fall back to 0 (usvg: attribute parse failure
// → default).
func parseGradientLength(v string) Length {
	l, err := parseLength(strings.TrimSpace(v))
	if err != nil {
		return Length{}
	}
	return l
}

func (d *Document) buildRadialBrush(g *RadialGradientElement, bbox render.Rect, rs RefSpace, color string) (render.Brush, bool) {
	chain := d.gradientChain(g)

	stopsRaw, owner, ok := gradientStopsFromChain(chain)
	if !ok {
		return nil, false
	}
	stops, ok := gradientStopsToColorStops(d, stopsRaw, owner, color)
	if !ok {
		return nil, false
	}
	if len(stops) == 1 {
		return solidStopBrush(stops[0]), true
	}

	e := RadialGradientElement{Attrs: g.Attrs, Units: UnitsObjectBoundingBox}
	e.Units = unitsFromChain(chain)
	// Defaults first (usvg resolve_number defaults): cx/cy/r = 50%, then
	// per-attribute chain resolution (coordinates same-type only).
	e.CX = Length{Value: 50, Unit: LengthUnitPercent}
	e.CY = Length{Value: 50, Unit: LengthUnitPercent}
	e.R = Length{Value: 50, Unit: LengthUnitPercent}
	if v, ok := gradientAttrSource(chain, g, "cx", true); ok {
		e.CX = parseGradientLength(v)
	}
	if v, ok := gradientAttrSource(chain, g, "cy", true); ok {
		e.CY = parseGradientLength(v)
	}
	if v, ok := gradientAttrSource(chain, g, "r", true); ok {
		e.R = parseGradientLength(v)
	}
	// fx/fy default INDEPENDENTLY to the resolved cx/cy numbers (usvg
	// resolve_number defaults: Length::new_number(cx as f64) / cy) — an
	// explicit fx with no fy still focuses at (fx, cy_resolved).
	fxFound := false
	fyFound := false
	if v, ok := gradientAttrSource(chain, g, "fx", true); ok {
		e.FX = parseGradientLength(v)
		fxFound = true
		e.HasFocus = true
	}
	if v, ok := gradientAttrSource(chain, g, "fy", true); ok {
		e.FY = parseGradientLength(v)
		fyFound = true
		e.HasFocus = true
	}
	if e.HasFocus {
		if !fxFound {
			e.FX = Length{Value: gradientCoordsIn(&e, e.CX, LengthAxisX, rs)}
		}
		if !fyFound {
			e.FY = Length{Value: gradientCoordsIn(&e, e.CY, LengthAxisY, rs)}
		}
	}
	if v, ok := gradientAttrSource(chain, g, "spreadMethod", false); ok {
		e.Spread = spreadToken(v)
		e.spreadSet = true
	}
	if v, ok := gradientAttrSource(chain, g, "gradientTransform", false); ok {
		e.Transform = v
	}

	// r invalid (zero or negative) → solid paint of the last stop
	// (usvg: !r.is_valid_length() → last stop color; spec 5.8.5 quote).
	rv := gradientCoordsIn(&e, e.R, LengthAxisDiagonal, rs)
	if rv <= 0 {
		return solidStopBrush(stops[len(stops)-1]), true
	}

	mapping, ok := gradientMappingFrom(&e, bbox, rs)
	if !ok {
		return nil, false
	}
	cx := gradientCoordsIn(&e, e.CX, LengthAxisX, rs)
	cy := gradientCoordsIn(&e, e.CY, LengthAxisY, rs)
	brush := render.NewRadialGradientBrush(cx, cy, 0, rv)
	brush.Stops = stops
	brush.Extend = e.Spread.extendOf()
	if e.HasFocus {
		// No pre-correction: resvg feeds the raw fx/fy to tiny-skia's
		// two-point conical shader, whose degenerate mask (no real t)
		// produces the corpus-verified output for outside-focal cases.
		fx := gradientCoordsIn(&e, e.FX, LengthAxisX, rs)
		fy := gradientCoordsIn(&e, e.FY, LengthAxisY, rs)
		if fx != cx || fy != cy {
			brush.SetFocus(fx, fy)
		}
	}
	if wrapped := wrapBrushIfMapped(brush, mapping); wrapped != nil {
		return wrapped, true
	}
	return nil, false
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
	return d.buildPatternBrushDepth(p, bbox, rs, 0)
}

// buildPatternBrushDepth carries the pattern-tile recursion depth: a
// pattern whose tile fills with another pattern (directly or transitively)
// fails closed past the ceiling — resvg's recursive-pattern cases recurse
// forever without this.
func (d *Document) buildPatternBrushDepth(p *PatternElement, bbox render.Rect, rs RefSpace, depth int) (render.Brush, bool) {
	if depth >= maxPropChainDepth {
		return nil, false
	}
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
	// 离屏 tile 必须与 Document.Render 一样钉 CPU 光栅：窗口进程里
	// render/gpu 注册了加速器，不带 WithCPUOnly 时 tile 会被 GPU 光栅化，
	// 画布左边缘 AA 覆盖不足（实测列 0 alpha=236/255），平铺采样到该列
	// 即每格首列一条暗竖缝（P3.8 批② tilePatt 专项）。
	tc := render.NewContext(tw, th, render.WithCPUOnly())
	defer tc.Close()
	// Tile content renders 1:1 in tile space (resvg render_pattern_pixmap:
	// content with scale only — no translate by (x,y); the tile is the
	// snapshot of content-space [0,w]×[0,h] and the rect origin anchors
	// the TILE placement). viewBox (when present) maps the viewBox onto
	// the tile box; oBB content units scale content by the FILL BBOX
	// dimensions — usvg to_user_coordinates pushes Transform::from_scale(
	// bbox.width, bbox.height) with no shift ("no need to shift patterns").
	if e.HasViewBox {
		tc.Transform(viewportTransform(float64(tw), float64(th), e.ViewBox, e.Par))
	} else if e.ContentUnits == UnitsObjectBoundingBox {
		tc.Scale(bbox.Max.X-bbox.Min.X, bbox.Max.Y-bbox.Min.Y)
	}
	tileProps := evalProps(rootProps(), e.Attrs, d)
	tileSt := renderState{doc: d, rs: rs, paintDepth: depth + 1}
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
		if err != nil || singularMatrix(pt) {
			// Invalid or singular patternTransform → identity (usvg
			// default-transform semantics; resvg reference-probed).
			pt = render.Identity()
		}
		// SVG2 transform-origin on the pattern shifts the transform
		// origin (usvg resolve_transform on PatternTransform).
		m = withTransformOrigin(pt, &e.Attrs, rs)
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
