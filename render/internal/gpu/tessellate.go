//go:build !nogpu

package gpu

import (
	"math"

	"github.com/energye/gpui/render"
)

// fanFlattenTolerance is the maximum allowed deviation between a curve and its
// linear approximation, in pixels. Smaller values produce more triangles but
// smoother curves. 0.1 keeps sub-pixel fidelity closer to Skia/kurbo defaults
// (was 0.25 — visible faceting on large circles and diagonal strokes).
const fanFlattenTolerance = 0.1

// fanCoverPadding is the number of pixels added around the AABB when
// generating the cover quad. This padding ensures that anti-aliased edges
// at the path boundary are fully covered during the cover pass.
const fanCoverPadding = 1.75

// fanInitialVertexCapacity is the initial capacity of the vertex slice,
// measured in float32 values (not triangles). 6 floats = 1 triangle.
// 256 floats = ~42 triangles, a reasonable starting size for typical paths.
const fanInitialVertexCapacity = 256

// FanTessellator converts path data into triangle fan vertices for stencil fill.
//
// For each contour in the path, it picks the first vertex (v0) as the fan center
// and emits triangles (v0, vi, vi+1) for every subsequent edge. Cubic and
// quadratic Bezier curves are adaptively flattened to line segments before
// triangulation.
//
// This is an O(n) algorithm that works correctly for any path topology
// (concave, self-intersecting, paths with holes) because the stencil buffer
// handles winding number correctness during the stencil pass.
//
// The tessellator is designed to be reused across frames via Reset().
type FanTessellator struct {
	// vertices holds x, y pairs for fan triangles.
	// Every 6 consecutive floats represent one triangle (3 vertices x 2 coords).
	vertices []float32

	// bounds is the axis-aligned bounding box: [minX, minY, maxX, maxY].
	bounds [4]float32

	// hasBounds tracks whether any vertex has been added to the bounds.
	hasBounds bool

	// Analytic-AA fringe state (TessellateAA):
	// aaVerts — cover fan as (x, y, signedEdgeDist) triples.
	// bandVerts — exterior band quads as (x, y, signedEdgeDist) triples.
	// segments — flattened boundary edges; segContours maps each segment to
	// its contour index; contourAreas accumulates the contour's signed 2×area.
	aaVerts      []float32
	bandVerts    []float32
	segments     []aaSegment
	segContours  []int
	contourAreas []float64
}

// NewFanTessellator creates a new tessellator with pre-allocated capacity.
func NewFanTessellator() *FanTessellator {
	return &FanTessellator{
		vertices: make([]float32, 0, fanInitialVertexCapacity),
	}
}

// Reset clears the tessellator state for reuse without releasing memory.
func (ft *FanTessellator) Reset() {
	ft.vertices = ft.vertices[:0]
	ft.bounds = [4]float32{}
	ft.hasBounds = false
	ft.aaVerts = ft.aaVerts[:0]
	ft.bandVerts = ft.bandVerts[:0]
	ft.segments = ft.segments[:0]
	ft.segContours = ft.segContours[:0]
	ft.contourAreas = ft.contourAreas[:0]
}

// TessellatePath converts a render.Path into triangle fan vertices.
//
// For each contour (started by MoveTo), the first vertex becomes the fan center.
// Subsequent line/curve segments are flattened and triangulated as fan triangles
// from the center vertex. Close elements generate a closing triangle back to the
// contour start.
//
// Returns the total number of vertices emitted. Each triangle uses 3 vertices
// (6 floats), so the triangle count is vertexCount/3.
func (ft *FanTessellator) TessellatePath(path *render.Path) int {
	if path == nil || path.NumVerbs() == 0 {
		return 0
	}

	var (
		fanOriginX, fanOriginY float64
		prevX, prevY           float64
		contourStarted         bool
	)

	path.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			fanOriginX, fanOriginY = coords[0], coords[1]
			prevX, prevY = coords[0], coords[1]
			contourStarted = true
			ft.updateBounds(float32(coords[0]), float32(coords[1]))

		case render.LineTo:
			if !contourStarted {
				return
			}
			ft.updateBounds(float32(coords[0]), float32(coords[1]))
			ft.emitFanTriangle(fanOriginX, fanOriginY, prevX, prevY, coords[0], coords[1])
			prevX, prevY = coords[0], coords[1]

		case render.QuadTo:
			if !contourStarted {
				return
			}
			ft.flattenQuadFan(
				fanOriginX, fanOriginY,
				prevX, prevY,
				coords[0], coords[1],
				coords[2], coords[3],
				fanFlattenTolerance,
			)
			prevX, prevY = coords[2], coords[3]

		case render.CubicTo:
			if !contourStarted {
				return
			}
			ft.flattenCubicFan(
				fanOriginX, fanOriginY,
				prevX, prevY,
				coords[0], coords[1],
				coords[2], coords[3],
				coords[4], coords[5],
				fanFlattenTolerance,
			)
			prevX, prevY = coords[4], coords[5]

		case render.Close:
			if !contourStarted {
				return
			}
			if prevX != fanOriginX || prevY != fanOriginY {
				ft.emitFanTriangle(fanOriginX, fanOriginY, prevX, prevY, fanOriginX, fanOriginY)
			}
			prevX, prevY = fanOriginX, fanOriginY
			contourStarted = false
		}
	})

	return len(ft.vertices) / 2
}

// Vertices returns the raw vertex buffer data as x,y float32 pairs.
// Every 6 consecutive values represent one triangle (3 vertices).
func (ft *FanTessellator) Vertices() []float32 {
	return ft.vertices
}

// Bounds returns the axis-aligned bounding box of all tessellated vertices.
// Format: [minX, minY, maxX, maxY]. Returns zeroes if no vertices were emitted.
func (ft *FanTessellator) Bounds() [4]float32 {
	return ft.bounds
}

// CoverQuad returns 6 vertices (2 triangles) forming a rectangle that covers
// the entire path bounding box plus fanCoverPadding pixels of padding.
// The padding ensures anti-aliased edges at the boundary are fully rendered.
//
// Triangle layout (counter-clockwise):
//
//	Triangle 1: (minX, minY), (maxX, minY), (maxX, maxY)
//	Triangle 2: (minX, minY), (maxX, maxY), (minX, maxY)
func (ft *FanTessellator) CoverQuad() [12]float32 {
	minX := ft.bounds[0] - fanCoverPadding
	minY := ft.bounds[1] - fanCoverPadding
	maxX := ft.bounds[2] + fanCoverPadding
	maxY := ft.bounds[3] + fanCoverPadding

	return [12]float32{
		// Triangle 1
		minX, minY, maxX, minY, maxX, maxY,
		// Triangle 2
		minX, minY, maxX, maxY, minX, maxY,
	}
}

// TriangleCount returns the number of triangles in the tessellated output.
func (ft *FanTessellator) TriangleCount() int {
	return len(ft.vertices) / 6
}

// emitFanTriangle appends a single triangle (v0, v1, v2) to the vertex buffer.
// Degenerate triangles (zero area) are skipped.
func (ft *FanTessellator) emitFanTriangle(v0x, v0y, v1x, v1y, v2x, v2y float64) {
	// Skip degenerate triangles using cross product (2x area)
	ax, ay := v1x-v0x, v1y-v0y
	bx, by := v2x-v0x, v2y-v0y
	cross := ax*by - ay*bx
	if cross == 0 {
		return
	}

	ft.vertices = append(ft.vertices,
		float32(v0x), float32(v0y),
		float32(v1x), float32(v1y),
		float32(v2x), float32(v2y),
	)
}

// updateBounds expands the AABB to include the given point.
func (ft *FanTessellator) updateBounds(x, y float32) {
	if !ft.hasBounds {
		ft.bounds = [4]float32{x, y, x, y}
		ft.hasBounds = true
		return
	}
	if x < ft.bounds[0] {
		ft.bounds[0] = x
	}
	if y < ft.bounds[1] {
		ft.bounds[1] = y
	}
	if x > ft.bounds[2] {
		ft.bounds[2] = x
	}
	if y > ft.bounds[3] {
		ft.bounds[3] = y
	}
}

// flattenQuadFan flattens a quadratic Bezier curve and emits fan triangles.
// Uses recursive de Casteljau subdivision with the specified tolerance.
func (ft *FanTessellator) flattenQuadFan(
	fanX, fanY, x0, y0, cx, cy, x1, y1, tol float64,
) {
	ft.flattenQuadFanDepth(fanX, fanY, x0, y0, cx, cy, x1, y1, tol, 0)
}

func (ft *FanTessellator) flattenQuadFanDepth(
	fanX, fanY, x0, y0, cx, cy, x1, y1, tol float64, depth int,
) {
	// NaN/Inf or runaway subdivision: terminate (matches CPU edge_builder guards).
	if depth > 32 || !isFinite6(x0, y0, cx, cy, x1, y1) {
		if isFinite2(x1, y1) {
			ft.updateBounds(float32(x1), float32(y1))
			if isFinite2(x0, y0) {
				ft.emitFanTriangle(fanX, fanY, x0, y0, x1, y1)
			}
		}
		return
	}
	// Check flatness: deviation of control point from chord midpoint
	midX := 0.25*x0 + 0.5*cx + 0.25*x1
	midY := 0.25*y0 + 0.5*cy + 0.25*y1
	chordMidX := 0.5 * (x0 + x1)
	chordMidY := 0.5 * (y0 + y1)

	dx := midX - chordMidX
	dy := midY - chordMidY
	distSq := dx*dx + dy*dy

	if distSq <= tol*tol {
		// Flat enough: emit single fan triangle
		ft.updateBounds(float32(x1), float32(y1))
		ft.emitFanTriangle(fanX, fanY, x0, y0, x1, y1)
		return
	}

	// De Casteljau subdivision at t=0.5
	ax := 0.5 * (x0 + cx)
	ay := 0.5 * (y0 + cy)
	bx := 0.5 * (cx + x1)
	by := 0.5 * (cy + y1)
	mx := 0.5 * (ax + bx)
	my := 0.5 * (ay + by)

	ft.flattenQuadFanDepth(fanX, fanY, x0, y0, ax, ay, mx, my, tol, depth+1)
	ft.flattenQuadFanDepth(fanX, fanY, mx, my, bx, by, x1, y1, tol, depth+1)
}

// flattenCubicFan flattens a cubic Bezier curve and emits fan triangles.
// Uses the standard cubic flatness test: both control points must be within
// tolerance of the chord line. The factor of 16 accounts for the cubic
// approximation error bound.
func (ft *FanTessellator) flattenCubicFan(
	fanX, fanY, x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol float64,
) {
	ft.flattenCubicFanDepth(fanX, fanY, x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol, 0)
}

func (ft *FanTessellator) flattenCubicFanDepth(
	fanX, fanY, x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol float64, depth int,
) {
	// NaN/Inf or runaway subdivision: terminate (matches CPU edge_builder guards).
	if depth > 32 || !isFinite8(x0, y0, c1x, c1y, c2x, c2y, x1, y1) {
		if isFinite2(x1, y1) {
			ft.updateBounds(float32(x1), float32(y1))
			if isFinite2(x0, y0) {
				ft.emitFanTriangle(fanX, fanY, x0, y0, x1, y1)
			}
		}
		return
	}
	// Cubic flatness test: check control point deviation from chord
	ux := 3*c1x - 2*x0 - x1
	uy := 3*c1y - 2*y0 - y1
	vx := 3*c2x - x0 - 2*x1
	vy := 3*c2y - y0 - 2*y1

	distSq := math.Max(ux*ux+uy*uy, vx*vx+vy*vy)

	// Factor of 16 for cubic approximation error bound
	if distSq <= 16*tol*tol {
		// Flat enough: emit single fan triangle
		ft.updateBounds(float32(x1), float32(y1))
		ft.emitFanTriangle(fanX, fanY, x0, y0, x1, y1)
		return
	}

	// De Casteljau subdivision at t=0.5
	ab1x := 0.5 * (x0 + c1x)
	ab1y := 0.5 * (y0 + c1y)
	ab2x := 0.5 * (c1x + c2x)
	ab2y := 0.5 * (c1y + c2y)
	ab3x := 0.5 * (c2x + x1)
	ab3y := 0.5 * (c2y + y1)

	bc1x := 0.5 * (ab1x + ab2x)
	bc1y := 0.5 * (ab1y + ab2y)
	bc2x := 0.5 * (ab2x + ab3x)
	bc2y := 0.5 * (ab2y + ab3y)

	mx := 0.5 * (bc1x + bc2x)
	my := 0.5 * (bc1y + bc2y)

	ft.flattenCubicFanDepth(fanX, fanY, x0, y0, ab1x, ab1y, bc1x, bc1y, mx, my, tol, depth+1)
	ft.flattenCubicFanDepth(fanX, fanY, mx, my, bc2x, bc2y, ab3x, ab3y, x1, y1, tol, depth+1)
}

// ---- Analytic-AA fringe tessellation (sampleCount==1 stencil covers) ----
//
// The binary stencil cover paints a bbox quad through a NotEqual(0) stencil
// test — at 1x sample count that yields hard binary edges for non-convex
// paths (e.g. stroked arc bands). The AA variant replaces the cover geometry
// with the path's own fan triangles, each vertex carrying the signed distance
// to the triangle's boundary edge. Distance to a line is affine, so fragment
// interpolation is exact, and the cover shader converts it to smoothstep
// coverage over the ±aaCoverHalfWidth band (Skia GrAATriangulator-style
// analytic AA; convex paths already get this via BuildConvexVertices).

// aaCoverHalfWidth is the half width (px) of the analytic fringe band,
// matching sdf_render.wgsl aa_hw and convexAAExpand (0.75px).
const aaCoverHalfWidth = 0.75

// aaSegment is one flattened path-boundary edge in pixel coords.
type aaSegment struct{ ax, ay, bx, by float64 }

// aaVerts holds the AA cover fan as (x, y, signedEdgeDist) triples and
// bandVerts the exterior band quads as (x, y, signedEdgeDist) triples
// (dist ∈ [-aaCoverHalfWidth, 0]). Populated by TessellateAA.

// aaAddSegment appends a flattened boundary edge and its contour's signed
// 2×area contribution (cross(A,B)).
func (ft *FanTessellator) aaAddSegment(ax, ay, bx, by float64) {
	ft.segments = append(ft.segments, aaSegment{ax, ay, bx, by})
	ft.segContours = append(ft.segContours, len(ft.contourAreas)-1)
	ft.contourAreas[len(ft.contourAreas)-1] += ax*by - ay*bx
}

// TessellateAA flattens the path into boundary segments, then emits:
//
//  1. aaVerts: fan triangles (fanOrigin, A, B) per segment with signed pixel
//     distance to edge A→B at the fan-origin vertex (0 at A/B). The sign is
//     normalized by the contour's orientation so distances are positive on
//     the fill-interior side.
//  2. bandVerts: exterior band quads per segment (edge extruded ±aaCoverHalfWidth
//     along the edge direction and -aaCoverHalfWidth along the outward normal),
//     painted with stencil Equal(0) for the outside half of the fringe.
//
// Returns the number of AA cover vertices emitted (triple floats; each
// triangle is 3 vertices).
func (ft *FanTessellator) TessellateAA(path *render.Path) int {
	ft.aaVerts = ft.aaVerts[:0]
	ft.bandVerts = ft.bandVerts[:0]
	ft.segments = ft.segments[:0]
	ft.segContours = ft.segContours[:0]
	ft.contourAreas = ft.contourAreas[:0]
	if path == nil || path.NumVerbs() == 0 {
		return 0
	}

	// Pass 1: flatten into boundary segments, accumulating per-contour
	// signed area for the interior-side sign convention.
	ft.aaCollectSegments(path)

	// Pass 2: emit AA fan + band from the flattened segments.
	seg := 0
	for c := 0; c < len(ft.contourAreas); c++ {
		orient := 1.0
		if ft.contourAreas[c] < 0 {
			orient = -1.0
		}
		// Skip degenerate contours (zero area).
		start := seg
		for seg < len(ft.segments) && ft.segContours[seg] == c {
			seg++
		}
		if seg == start {
			continue
		}
		// Fan origin = first segment's A (path contour start).
		ox, oy := ft.segments[start].ax, ft.segments[start].ay
		for i := start; i < seg; i++ {
			s := ft.segments[i]
			// Skip zero-length edges (they contribute no boundary).
			ex, ey := s.bx-s.ax, s.by-s.ay
			elen := math.Hypot(ex, ey)
			if elen < 1e-9 {
				continue
			}
			// Signed distance from the fan origin to the edge line, positive
			// on the fill-interior side (orientation-normalized).
			dO := orient * (ex*(oy-s.ay) - ey*(ox-s.ax)) / elen
			ft.emitAATriangle(ox, oy, dO, s.ax, s.ay, 0, s.bx, s.by, 0)
			ft.emitAABand(s.ax, s.ay, s.bx, s.by, orient)
		}
	}
	return len(ft.aaVerts) / 3
}

// aaCollectSegments walks the path and flattens every edge into aaSegments
// (same flatness tolerance and guards as TessellatePath).
func (ft *FanTessellator) aaCollectSegments(path *render.Path) {
	var (
		prevX, prevY float64
		contourOn    bool
		closeX, closeY float64
	)
	path.Iterate(func(verb render.PathVerb, coords []float64) {
		switch verb {
		case render.MoveTo:
			prevX, prevY = coords[0], coords[1]
			closeX, closeY = coords[0], coords[1]
			contourOn = true
			ft.contourAreas = append(ft.contourAreas, 0)
		case render.LineTo:
			if !contourOn {
				return
			}
			ft.aaAddSegment(prevX, prevY, coords[0], coords[1])
			prevX, prevY = coords[0], coords[1]
		case render.QuadTo:
			if !contourOn {
				return
			}
			ft.aaFlattenQuad(prevX, prevY, coords[0], coords[1], coords[2], coords[3], fanFlattenTolerance, 0)
			prevX, prevY = coords[2], coords[3]
		case render.CubicTo:
			if !contourOn {
				return
			}
			ft.aaFlattenCubic(prevX, prevY, coords[0], coords[1], coords[2], coords[3], coords[4], coords[5], fanFlattenTolerance, 0)
			prevX, prevY = coords[4], coords[5]
		case render.Close:
			if !contourOn {
				return
			}
			if prevX != closeX || prevY != closeY {
				// Closing edge participates in the exterior band; the fan
				// triangle for it is degenerate (origin==endpoint) so the
				// closing edge's interior fringe is covered by the adjacent
				// fan triangles' attr band only near the corners.
				ft.aaAddSegment(prevX, prevY, closeX, closeY)
			}
			prevX, prevY = closeX, closeY
			contourOn = false
		}
	})
}

func (ft *FanTessellator) aaFlattenQuad(x0, y0, cx, cy, x1, y1, tol float64, depth int) {
	if depth > 32 || !isFinite6(x0, y0, cx, cy, x1, y1) {
		if isFinite2(x0, y0) && isFinite2(x1, y1) {
			ft.aaAddSegment(x0, y0, x1, y1)
		}
		return
	}
	midX := 0.25*x0 + 0.5*cx + 0.25*x1
	midY := 0.25*y0 + 0.5*cy + 0.25*y1
	chordMidX := 0.5 * (x0 + x1)
	chordMidY := 0.5 * (y0 + y1)
	dx := midX - chordMidX
	dy := midY - chordMidY
	if dx*dx+dy*dy <= tol*tol {
		ft.aaAddSegment(x0, y0, x1, y1)
		return
	}
	ax := 0.5 * (x0 + cx)
	ay := 0.5 * (y0 + cy)
	bx := 0.5 * (cx + x1)
	by := 0.5 * (cy + y1)
	mx := 0.5 * (ax + bx)
	my := 0.5 * (ay + by)
	ft.aaFlattenQuad(x0, y0, ax, ay, mx, my, tol, depth+1)
	ft.aaFlattenQuad(mx, my, bx, by, x1, y1, tol, depth+1)
}

func (ft *FanTessellator) aaFlattenCubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol float64, depth int) {
	if depth > 32 || !isFinite8(x0, y0, c1x, c1y, c2x, c2y, x1, y1) {
		if isFinite2(x0, y0) && isFinite2(x1, y1) {
			ft.aaAddSegment(x0, y0, x1, y1)
		}
		return
	}
	ux := 3*c1x - 2*x0 - x1
	uy := 3*c1y - 2*y0 - y1
	vx := 3*c2x - x0 - 2*x1
	vy := 3*c2y - y0 - 2*y1
	if ux*ux+uy*uy <= 16*tol*tol && vx*vx+vy*vy <= 16*tol*tol {
		ft.aaAddSegment(x0, y0, x1, y1)
		return
	}
	ab1x := 0.5 * (x0 + c1x)
	ab1y := 0.5 * (y0 + c1y)
	ab2x := 0.5 * (c1x + c2x)
	ab2y := 0.5 * (c1y + c2y)
	ab3x := 0.5 * (c2x + x1)
	ab3y := 0.5 * (c2y + y1)
	bc1x := 0.5 * (ab1x + ab2x)
	bc1y := 0.5 * (ab1y + ab2y)
	bc2x := 0.5 * (ab2x + ab3x)
	bc2y := 0.5 * (ab2y + ab3y)
	mx := 0.5 * (bc1x + bc2x)
	my := 0.5 * (bc1y + bc2y)
	ft.aaFlattenCubic(x0, y0, ab1x, ab1y, bc1x, bc1y, mx, my, tol, depth+1)
	ft.aaFlattenCubic(mx, my, bc2x, bc2y, ab3x, ab3y, x1, y1, tol, depth+1)
}

// emitAATriangle appends one fan triangle with per-vertex signed edge
// distances (d at the boundary-edge endpoints is 0). Degenerate triangles
// (zero area) are skipped, mirroring emitFanTriangle.
func (ft *FanTessellator) emitAATriangle(v0x, v0y, d0, v1x, v1y, d1, v2x, v2y, d2 float64) {
	ax, ay := v1x-v0x, v1y-v0y
	bx, by := v2x-v0x, v2y-v0y
	if ax*by-ay*bx == 0 {
		return
	}
	ft.aaVerts = append(ft.aaVerts,
		float32(v0x), float32(v0y), float32(d0),
		float32(v1x), float32(v1y), float32(d1),
		float32(v2x), float32(v2y), float32(d2),
	)
}

// emitAABand appends the exterior band quad for one boundary edge A→B:
// the edge extruded aaCoverHalfWidth along the edge direction on both ends
// (covers joints) and -aaCoverHalfWidth along the outward normal. Vertices
// carry d = 0 on the boundary line and d = -aaCoverHalfWidth on the outside
// edge, so the cover_aa.wgsl smoothstep fades 0.5 → 0 across the band.
func (ft *FanTessellator) emitAABand(ax, ay, bx, by, orient float64) {
	ex, ey := bx-ax, by-ay
	elen := math.Hypot(ex, ey)
	if elen < 1e-9 {
		return
	}
	aa := aaCoverHalfWidth
	tx, ty := ex/elen, ey/elen
	// Outward normal: right of the direction for CCW (fill left), left for CW.
	nx, ny := orient*ty, -orient*tx
	// Band corners (d = 0 on the line, -aa outside).
	b0x, b0y := ax-tx*aa, ay-ty*aa
	b1x, b1y := bx+tx*aa, by+ty*aa
	b2x, b2y := b0x-nx*aa, b0y-ny*aa
	b3x, b3y := b1x-nx*aa, b1y-ny*aa
	ft.bandVerts = append(ft.bandVerts,
		float32(b0x), float32(b0y), 0,
		float32(b1x), float32(b1y), 0,
		float32(b2x), float32(b2y), float32(-aa),
		float32(b1x), float32(b1y), 0,
		float32(b3x), float32(b3y), float32(-aa),
		float32(b2x), float32(b2y), float32(-aa),
	)
}

func isFinite2(a, b float64) bool {
	return !math.IsNaN(a) && !math.IsInf(a, 0) && !math.IsNaN(b) && !math.IsInf(b, 0)
}

func isFinite6(a, b, c, d, e, f float64) bool {
	return isFinite2(a, b) && isFinite2(c, d) && isFinite2(e, f)
}

func isFinite8(a, b, c, d, e, f, g, h float64) bool {
	return isFinite6(a, b, c, d, e, f) && isFinite2(g, h)
}
