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
	"math"
	"strconv"
	"strings"

	"github.com/energye/gpui/render"
)

// markerVertex is one path vertex with its incoming/outgoing tangent angles
// (radians). NaN means "not set" (start has no incoming, open end has no
// outgoing; zero-length segments are skipped).
type markerVertex struct {
	pt      render.Point
	in, out float64
}

// markerSubpath is one M..Z run. For a closed subpath the start and end are the
// same vertex (index 0); mid markers cover 1..len-1. For an open subpath start
// is 0, end is len-1, mid is 1..len-2.
type markerSubpath struct {
	pts    []markerVertex
	closed bool
}

const markerNaN = math.MaxFloat64

func markerAngle(from, to render.Point) float64 {
	dx, dy := to.X-from.X, to.Y-from.Y
	if math.Abs(dx) < 1e-12 && math.Abs(dy) < 1e-12 {
		return markerNaN
	}
	return math.Atan2(dy, dx)
}

func markerSetOut(v *markerVertex, a float64) {
	if a != markerNaN && v.out == markerNaN {
		v.out = a
	}
}

// markerSubpaths splits a path into subpaths and records vertex tangents.
// Marker positions are the original command endpoints (never a flattening
// polyline), so mid markers appear once per vertex, including curve endpoints.
func markerSubpaths(path *render.Path) []markerSubpath {
	if path == nil {
		return nil
	}
	verbs, coords := path.Verbs(), path.Coords()
	ci := 0
	var out []markerSubpath
	var cur markerSubpath
	var curPt, startPt render.Point
	started := false

	flush := func() {
		if started && len(cur.pts) > 0 {
			out = append(out, cur)
		}
		cur = markerSubpath{}
		started = false
	}
	begin := func(p render.Point) {
		cur.pts = append(cur.pts, markerVertex{pt: p, in: markerNaN, out: markerNaN})
		curPt, startPt = p, p
		started = true
	}
	appendPt := func(p render.Point, inAng, outAng float64) {
		if !started {
			begin(curPt)
		}
		last := &cur.pts[len(cur.pts)-1]
		markerSetOut(last, outAng)
		cur.pts = append(cur.pts, markerVertex{pt: p, in: inAng, out: markerNaN})
		curPt = p
	}

	for _, v := range verbs {
		switch v {
		case render.MoveTo:
			if ci+1 >= len(coords) {
				continue
			}
			p := render.Point{X: coords[ci], Y: coords[ci+1]}
			ci += 2
			flush()
			begin(p)
		case render.LineTo:
			if ci+1 >= len(coords) {
				continue
			}
			p := render.Point{X: coords[ci], Y: coords[ci+1]}
			ci += 2
			a := markerAngle(curPt, p)
			appendPt(p, a, a)
		case render.QuadTo:
			if ci+3 >= len(coords) {
				continue
			}
			c := render.Point{X: coords[ci], Y: coords[ci+1]}
			p := render.Point{X: coords[ci+2], Y: coords[ci+3]}
			ci += 4
			appendPt(p, markerAngle(c, p), markerAngle(curPt, c))
		case render.CubicTo:
			if ci+5 >= len(coords) {
				continue
			}
			c1 := render.Point{X: coords[ci], Y: coords[ci+1]}
			c2 := render.Point{X: coords[ci+2], Y: coords[ci+3]}
			p := render.Point{X: coords[ci+4], Y: coords[ci+5]}
			ci += 6
			appendPt(p, markerAngle(c2, p), markerAngle(curPt, c1))
		case render.ConicTo:
			if ci+4 >= len(coords) {
				continue
			}
			c := render.Point{X: coords[ci], Y: coords[ci+1]}
			p := render.Point{X: coords[ci+2], Y: coords[ci+3]}
			ci += 5 // conic weight (coords[ci+4]) does not affect the endpoint tangent
			appendPt(p, markerAngle(c, p), markerAngle(curPt, c))
		case render.Close:
			if !started {
				continue
			}
			a := markerAngle(curPt, startPt)
			if len(cur.pts) > 0 {
				markerSetOut(&cur.pts[len(cur.pts)-1], a)
				cur.pts[0].in = a
			}
			cur.closed = true
			flush()
			// A command after Z starts a new subpath at the closed start.
			curPt = startPt
		}
	}
	flush()
	return out
}

// markerAngleAt returns the tangent for a vertex, using the outgoing angle for
// start, incoming for end, and the bisector for mid. Fallbacks keep a
// zero-length first/last segment from dropping the marker.
func markerAngleAt(v markerVertex, kind int) float64 {
	if v.in == markerNaN && v.out == markerNaN {
		return 0 // degenerate vertex (e.g. zero-length subpath)
	}
	switch kind {
	case 0: // start
		if v.out != markerNaN {
			return v.out
		}
		return v.in
	case 2: // end
		if v.in != markerNaN {
			return v.in
		}
		return v.out
	default: // mid
		if v.in == markerNaN {
			return v.out
		}
		if v.out == markerNaN {
			return v.in
		}
		ux, uy := math.Cos(v.in), math.Sin(v.in)
		vx, vy := math.Cos(v.out), math.Sin(v.out)
		sx, sy := ux+vx, uy+vy
		if math.Hypot(sx, sy) < 1e-9 {
			return v.out // 180° reversal: keep the outgoing direction
		}
		return math.Atan2(sy, sx)
	}
}

// renderMarkers paints marker-start/mid/end for one shape path after its
// fill/stroke (SVG paint order: markers last). The caller has already applied
// the element transform, so vertices/tangents are in the element's local user
// space.
func renderMarkers(dc *render.Context, path *render.Path, a *Attrs, p *props, st *renderState) {
	if dc == nil || path == nil || p == nil || st == nil || st.markerDepth >= maxPropChainDepth {
		return
	}
	startInvalid, midInvalid, endInvalid := false, false, false
	if a != nil {
		startInvalid, midInvalid, endInvalid = a.markerStartInvalid, a.markerMidInvalid, a.markerEndInvalid
	}
	refs := [3]string{p.markerStart, p.markerMid, p.markerEnd}
	invalids := [3]bool{startInvalid, midInvalid, endInvalid}
	if refs[0] == "" && refs[1] == "" && refs[2] == "" {
		return
	}
	subs := markerSubpaths(path)
	if len(subs) == 0 {
		return
	}
	for kind := 0; kind < 3; kind++ {
		ref := strings.TrimSpace(refs[kind])
		if ref == "" || strings.EqualFold(ref, "none") || invalids[kind] {
			continue
		}
		id, ok := paintRefID(ref)
		if !ok {
			continue
		}
		m, ok := st.doc.ElementByID(id).(*MarkerElement)
		if !ok || m == nil {
			continue
		}
		for _, sub := range subs {
			if len(sub.pts) == 0 {
				continue
			}
			switch kind {
			case 0:
				renderOneMarker(dc, m, sub.pts[0], markerAngleAt(sub.pts[0], 0), kind, p, st)
			case 2:
				idx := len(sub.pts) - 1
				if sub.closed {
					idx = 0
				}
				renderOneMarker(dc, m, sub.pts[idx], markerAngleAt(sub.pts[idx], 2), kind, p, st)
			case 1:
				last := len(sub.pts) - 1
				if sub.closed {
					last = len(sub.pts) - 1
				} else {
					last = len(sub.pts) - 2
				}
				for i := 1; i <= last; i++ {
					renderOneMarker(dc, m, sub.pts[i], markerAngleAt(sub.pts[i], 1), kind, p, st)
				}
			}
		}
	}
}

// markerOrientAngle resolves orient: "" / angle (degrees) / auto /
// auto-start-reverse (start only; mid/end behave as auto).
func markerOrientAngle(orient string, tangent float64, kind int) float64 {
	v := strings.ToLower(strings.TrimSpace(orient))
	switch v {
	case "", "0":
		return 0
	case "auto":
		return tangent
	case "auto-start-reverse":
		if kind == 0 {
			return tangent + math.Pi
		}
		return tangent
	}
	v = strings.TrimSpace(strings.TrimSuffix(v, "deg"))
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f * math.Pi / 180
	}
	return 0
}

// renderOneMarker instantiates one marker at a vertex. Transform order:
// translate(vertex) · rotate · scale(markerUnits) · viewBox · translate(-refX,-refY).
func renderOneMarker(dc *render.Context, m *MarkerElement, v markerVertex, tangent float64, kind int, p *props, st *renderState) {
	if m == nil || st == nil || st.markerDepth >= maxPropChainDepth {
		return
	}
	mw := m.MarkerW.Resolve(st.rs, LengthAxisX)
	mh := m.MarkerH.Resolve(st.rs, LengthAxisY)
	if mw <= 0 || mh <= 0 {
		mw, mh = 3, 3 // SVG 1.1 defaults; degenerate explicit values fail closed
	}
	refX := m.RefX.Resolve(st.rs, LengthAxisX)
	refY := m.RefY.Resolve(st.rs, LengthAxisY)
	viewBoxM := render.Identity()
	if m.HasViewBox {
		viewBoxM = viewportTransform(mw, mh, m.ViewBox, m.Par)
	}
	scale := 1.0
	if !strings.EqualFold(m.MarkerUnits, "userSpaceOnUse") {
		scale = p.strokeWidth
		if scale <= 0 {
			scale = 1
		}
	}
	if math.IsNaN(tangent) {
		tangent = 0
	}
	angle := markerOrientAngle(m.Orient, tangent, kind)
	mat := render.Translate(v.pt.X, v.pt.Y).
		Multiply(render.Rotate(angle)).
		Multiply(render.Scale(scale, scale)).
		Multiply(viewBoxM).
		Multiply(render.Translate(-refX, -refY))

	mp := evalProps(st.root, m.Attrs, st.doc)
	child := *st
	child.markerDepth++
	child.contextFill = markerContextToken(p.fill, st.contextFill)
	child.contextStroke = markerContextToken(p.stroke, st.contextStroke)

	dc.Push()
	dc.Transform(mat)
	if !m.OverflowVisible {
		dc.ClipRect(0, 0, mw, mh)
	}
	if mp.opacity < 1 {
		dc.PushLayer(render.BlendNormal, mp.opacity)
		renderElements(dc, m.Children, mp, &child)
		dc.PopLayer()
	} else {
		renderElements(dc, m.Children, mp, &child)
	}
	dc.Pop()
}

// markerContextToken resolves a marker content paint token against the
// referencing shape's fill/stroke for SVG 2 context-fill/context-stroke.
func markerContextToken(tok, outer string) string {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "context-fill", "context-stroke":
		if outer != "" {
			return outer
		}
	}
	return tok
}
