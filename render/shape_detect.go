package render

import (
	"math"
)

// ShapeKind identifies detected shapes for GPU SDF acceleration.
type ShapeKind int

const (
	// ShapeUnknown indicates the path is too complex for shape detection.
	ShapeUnknown ShapeKind = iota

	// ShapeCircle indicates a circular path.
	ShapeCircle

	// ShapeEllipse indicates an elliptical path.
	ShapeEllipse

	// ShapeRect indicates an axis-aligned rectangular path.
	ShapeRect

	// ShapeRRect indicates a rounded rectangle path.
	ShapeRRect

	// ShapeArc indicates an open circular arc path (DrawArc: MoveTo + n×CubicTo,
	// no Close). Parameters: CenterX/Y, RadiusX (=RadiusY), Angle0/Angle1.
	ShapeArc
)

// DetectedShape holds parameters of a recognized geometric shape.
// The Kind field indicates which parameters are meaningful.
type DetectedShape struct {
	Kind         ShapeKind
	CenterX      float64 // Center X coordinate.
	CenterY      float64 // Center Y coordinate.
	RadiusX      float64 // X radius. For circle: RadiusX == RadiusY.
	RadiusY      float64 // Y radius. For circle: RadiusX == RadiusY.
	Width        float64 // Total width for rect/rrect.
	Height       float64 // Total height for rect/rrect.
	CornerRadius float64 // Corner radius for rrect only.
	Angle0       float64 // Arc start angle (radians), ShapeArc only.
	Angle1       float64 // Arc end angle (radians), ShapeArc only.
}

// kappa is the cubic Bezier control point distance for circle approximation.
// Equal to 4/3 * (sqrt(2) - 1).
const kappa = 0.5522847498307936

// shapeDetectTolerance is the maximum allowed error for shape detection.
const shapeDetectTolerance = 1e-3

// tanDotTol bounds |cos(angle between cubic control offset and radius)| for
// circular-arc segments (0 for exact arcs; ellipse arcs deviate well beyond).
const tanDotTol = 1e-3

// DetectShape analyzes a Path and returns the identified shape if recognized.
// Returns a DetectedShape with Kind == ShapeUnknown if the path cannot be
// identified as a simple geometric primitive.
func DetectShape(path *Path) DetectedShape {
	if path == nil {
		return DetectedShape{Kind: ShapeUnknown}
	}

	nv := path.NumVerbs()
	if nv == 0 {
		return DetectedShape{Kind: ShapeUnknown}
	}

	verbs := path.Verbs()
	coords := path.Coords()

	// Try circle/ellipse: MoveTo + 4xCubicTo + Close = 6 elements
	if nv == 6 {
		if shape, ok := detectCircleOrEllipse(verbs, coords); ok {
			return shape
		}
	}

	// Try arc: open path MoveTo + n×CubicTo (no Close), all endpoints on a
	// common circle with sweep < 2π. GPU renders arc strokes through the SDF
	// annular pipeline (smoothstep AA at 1x sample count). Circular arcs are
	// detected first; axis-aligned elliptical arcs (DrawEllipticalArc) fall
	// back to detectEllipticalArc and render through the same kind-2 SDF with
	// per-axis radii.
	if verbs[0] == MoveTo && verbs[nv-1] != Close {
		if shape, ok := detectArc(verbs, coords); ok {
			return shape
		}
		if shape, ok := detectEllipticalArc(verbs, coords); ok {
			return shape
		}
	}

	// Try rect: MoveTo + 3xLineTo + Close = 5 elements
	if nv == 5 {
		if shape, ok := detectRect(verbs, coords); ok {
			return shape
		}
	}

	// Try rrect: MoveTo + (CubicTo + LineTo)*4 + Close = 10 elements
	// Or variation with arcs: more elements
	if nv >= 9 {
		if shape, ok := detectRRect(verbs, coords); ok {
			return shape
		}
	}

	return DetectedShape{Kind: ShapeUnknown}
}

// detectCircleOrEllipse checks if 6 verbs form a circle or ellipse.
// Expected pattern: MoveTo, CubicTo, CubicTo, CubicTo, CubicTo, Close.
func detectCircleOrEllipse(verbs []PathVerb, coords []float64) (DetectedShape, bool) {
	if verbs[0] != MoveTo || verbs[5] != Close {
		return DetectedShape{}, false
	}
	for i := 1; i <= 4; i++ {
		if verbs[i] != CubicTo {
			return DetectedShape{}, false
		}
	}

	// MoveTo: coords[0..1]
	// CubicTo 1: coords[2..7] (c1x,c1y,c2x,c2y,x,y)
	// CubicTo 2: coords[8..13]
	// CubicTo 3: coords[14..19]
	// CubicTo 4: coords[20..25]

	// Extract the 5 endpoints: start + 4 cubic endpoints.
	pts := [5]Point{
		Pt(coords[0], coords[1]),   // MoveTo
		Pt(coords[6], coords[7]),   // CubicTo 1 endpoint
		Pt(coords[12], coords[13]), // CubicTo 2 endpoint
		Pt(coords[18], coords[19]), // CubicTo 3 endpoint
		Pt(coords[24], coords[25]), // CubicTo 4 endpoint
	}

	// The path must close: endpoint of last cubic == start.
	if !pointsClose(pts[4], pts[0]) {
		return DetectedShape{}, false
	}

	// Calculate center from opposing points.
	cx := (pts[0].X + pts[2].X) / 2
	cy := (pts[0].Y + pts[2].Y) / 2

	cx2 := (pts[1].X + pts[3].X) / 2
	cy2 := (pts[1].Y + pts[3].Y) / 2

	if math.Abs(cx-cx2) > shapeDetectTolerance || math.Abs(cy-cy2) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	rx := math.Abs(pts[0].X - cx)
	ry := math.Abs(pts[1].Y - cy)

	if rx < shapeDetectTolerance || ry < shapeDetectTolerance {
		return DetectedShape{}, false
	}

	// Verify control points match the kappa-based circle/ellipse approximation.
	// Each cubic has 6 coords: c1x, c1y, c2x, c2y, x, y. Control points are at offsets 0..3.
	cubicCPs := [4]cubicControlPair{
		{coords[2], coords[3], coords[4], coords[5]},
		{coords[8], coords[9], coords[10], coords[11]},
		{coords[14], coords[15], coords[16], coords[17]},
		{coords[20], coords[21], coords[22], coords[23]},
	}

	if !verifyEllipseCPs(cubicCPs, cx, cy, rx, ry) {
		return DetectedShape{}, false
	}

	if math.Abs(rx-ry) < shapeDetectTolerance {
		r := (rx + ry) / 2
		return DetectedShape{
			Kind: ShapeCircle, CenterX: cx, CenterY: cy, RadiusX: r, RadiusY: r,
		}, true
	}

	return DetectedShape{
		Kind: ShapeEllipse, CenterX: cx, CenterY: cy, RadiusX: rx, RadiusY: ry,
	}, true
}

// cubicControlPair holds the two control points (c1x,c1y,c2x,c2y) of a cubic.
type cubicControlPair struct{ c1x, c1y, c2x, c2y float64 }

// verifyEllipseCPs validates that cubic Bezier control points
// match the standard kappa-based ellipse approximation.
func verifyEllipseCPs(cps [4]cubicControlPair, cx, cy, rx, ry float64) bool {
	kx := rx * kappa
	ky := ry * kappa

	// Quadrant 1: CP1 = (cx+rx, cy+ky), CP2 = (cx+kx, cy+ry)
	if !checkCPXY(cps[0].c1x, cps[0].c1y, cx+rx, cy+ky) || !checkCPXY(cps[0].c2x, cps[0].c2y, cx+kx, cy+ry) {
		return false
	}
	// Quadrant 2: CP1 = (cx-kx, cy+ry), CP2 = (cx-rx, cy+ky)
	if !checkCPXY(cps[1].c1x, cps[1].c1y, cx-kx, cy+ry) || !checkCPXY(cps[1].c2x, cps[1].c2y, cx-rx, cy+ky) {
		return false
	}
	// Quadrant 3: CP1 = (cx-rx, cy-ky), CP2 = (cx-kx, cy-ry)
	if !checkCPXY(cps[2].c1x, cps[2].c1y, cx-rx, cy-ky) || !checkCPXY(cps[2].c2x, cps[2].c2y, cx-kx, cy-ry) {
		return false
	}
	// Quadrant 4: CP1 = (cx+kx, cy-ry), CP2 = (cx+rx, cy-ky)
	if !checkCPXY(cps[3].c1x, cps[3].c1y, cx+kx, cy-ry) || !checkCPXY(cps[3].c2x, cps[3].c2y, cx+rx, cy-ky) {
		return false
	}

	return true
}

// checkCPXY verifies a control point (x, y) is close to expected coordinates.
func checkCPXY(ax, ay, ex, ey float64) bool {
	return math.Abs(ax-ex) < shapeDetectTolerance && math.Abs(ay-ey) < shapeDetectTolerance
}

// detectRect checks if 5 verbs form an axis-aligned rectangle.
// Expected pattern: MoveTo, LineTo, LineTo, LineTo, Close.
func detectRect(verbs []PathVerb, coords []float64) (DetectedShape, bool) {
	if verbs[0] != MoveTo || verbs[4] != Close {
		return DetectedShape{}, false
	}
	for i := 1; i <= 3; i++ {
		if verbs[i] != LineTo {
			return DetectedShape{}, false
		}
	}

	// MoveTo: coords[0..1], LineTo 1: coords[2..3], LineTo 2: coords[4..5], LineTo 3: coords[6..7]
	corners := [4]Point{
		Pt(coords[0], coords[1]),
		Pt(coords[2], coords[3]),
		Pt(coords[4], coords[5]),
		Pt(coords[6], coords[7]),
	}

	// Verify axis-aligned: each consecutive pair must share X or Y.
	for i := 0; i < 4; i++ {
		j := (i + 1) % 4
		dx := math.Abs(corners[i].X - corners[j].X)
		dy := math.Abs(corners[i].Y - corners[j].Y)
		if dx > shapeDetectTolerance && dy > shapeDetectTolerance {
			return DetectedShape{}, false
		}
	}

	// Find bounding box.
	minX, maxX := corners[0].X, corners[0].X
	minY, maxY := corners[0].Y, corners[0].Y
	for _, c := range corners[1:] {
		minX = math.Min(minX, c.X)
		maxX = math.Max(maxX, c.X)
		minY = math.Min(minY, c.Y)
		maxY = math.Max(maxY, c.Y)
	}

	w := maxX - minX
	h := maxY - minY

	if w < shapeDetectTolerance || h < shapeDetectTolerance {
		return DetectedShape{}, false
	}

	return DetectedShape{
		Kind:    ShapeRect,
		CenterX: (minX + maxX) / 2,
		CenterY: (minY + maxY) / 2,
		Width:   w,
		Height:  h,
	}, true
}

// detectRRect checks if verbs form a rounded rectangle.
// Expected: MoveTo + (LineTo + CubicTo)*4 + Close = 10 verbs.
func detectRRect(verbs []PathVerb, coords []float64) (DetectedShape, bool) {
	if len(verbs) != 10 {
		return DetectedShape{}, false
	}

	if verbs[0] != MoveTo || verbs[9] != Close {
		return DetectedShape{}, false
	}

	// Verify alternating LineTo, CubicTo pattern.
	for i := 0; i < 4; i++ {
		if verbs[1+i*2] != LineTo || verbs[2+i*2] != CubicTo {
			return DetectedShape{}, false
		}
	}

	// Coord layout: MoveTo(2) + [LineTo(2) + CubicTo(6)]*4 = 2 + 4*8 = 34
	// MoveTo:     coords[0..1]
	// LineTo 0:   coords[2..3]
	// CubicTo 0:  coords[4..9]  (c1x,c1y,c2x,c2y,x,y)
	// LineTo 1:   coords[10..11]
	// CubicTo 1:  coords[12..17]
	// LineTo 2:   coords[18..19]
	// CubicTo 2:  coords[20..25]
	// LineTo 3:   coords[26..27]
	// CubicTo 3:  coords[28..33]

	moveX, moveY := coords[0], coords[1]

	var linePoints [4]Point
	var cubicEndpoints [4]Point

	for i := 0; i < 4; i++ {
		lBase := 2 + i*8 // LineTo coord offset
		linePoints[i] = Pt(coords[lBase], coords[lBase+1])

		cBase := lBase + 2 // CubicTo coord offset (6 coords: c1x,c1y,c2x,c2y,x,y)
		cubicEndpoints[i] = Pt(coords[cBase+4], coords[cBase+5])
	}

	// Verify geometry.
	topY := moveY
	if math.Abs(linePoints[0].Y-topY) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	rightX := cubicEndpoints[0].X
	if math.Abs(linePoints[1].X-rightX) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	bottomY := cubicEndpoints[1].Y
	if math.Abs(linePoints[2].Y-bottomY) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	leftX := cubicEndpoints[2].X
	if math.Abs(linePoints[3].X-leftX) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	w := rightX - leftX
	h := bottomY - topY
	if w < shapeDetectTolerance || h < shapeDetectTolerance {
		return DetectedShape{}, false
	}

	r1 := moveX - leftX
	r2 := rightX - linePoints[0].X
	if r1 < 0 || r2 < 0 {
		return DetectedShape{}, false
	}
	if math.Abs(r1-r2) > shapeDetectTolerance {
		return DetectedShape{}, false
	}
	cornerR := (r1 + r2) / 2

	r3 := cubicEndpoints[0].Y - topY
	r4 := bottomY - linePoints[1].Y
	if math.Abs(r3-cornerR) > shapeDetectTolerance || math.Abs(r4-cornerR) > shapeDetectTolerance {
		return DetectedShape{}, false
	}

	return DetectedShape{
		Kind:         ShapeRRect,
		CenterX:      (leftX + rightX) / 2,
		CenterY:      (topY + bottomY) / 2,
		Width:        w,
		Height:       h,
		CornerRadius: cornerR,
	}, true
}

// detectArc checks for an open circular arc: MoveTo + n×CubicTo, no Close,
// where all endpoints lie on a common circle with sweep angle < 2π. This is
// the exact shape DrawArc emits (cubic arc approximation segments).
func detectArc(verbs []PathVerb, coords []float64) (DetectedShape, bool) {
	n := len(verbs)
	if n < 2 {
		return DetectedShape{}, false
	}
	for i := 1; i < n; i++ {
		if verbs[i] != CubicTo {
			return DetectedShape{}, false
		}
	}

	// Collect endpoints: MoveTo point + each CubicTo endpoint.
	pts := make([]Point, 0, n)
	pts = append(pts, Pt(coords[0], coords[1]))
	ci := 2
	for i := 1; i < n; i++ {
		pts = append(pts, Pt(coords[ci+4], coords[ci+5]))
		ci += 6
	}
	if len(pts) < 3 {
		return DetectedShape{}, false
	}

	// Fit a circle through the first, middle, and last endpoints, then verify
	// consistency with two more endpoint triples. An ellipse arc drifts the
	// fitted center/radius across triples (curvature varies); a true circular
	// arc keeps them stable within tolerance.
	cx, cy, r, ok := circleFromPoints(pts[0], pts[len(pts)/2], pts[len(pts)-1])
	if !ok || r < shapeDetectTolerance {
		return DetectedShape{}, false
	}
	ctol := arcCenterTolerance(r)
	for _, tri := range [][3]Point{
		{pts[0], pts[1], pts[len(pts)-1]},
		{pts[0], pts[len(pts)-2], pts[len(pts)-1]},
	} {
		cx2, cy2, r2, ok2 := circleFromPoints(tri[0], tri[1], tri[2])
		if !ok2 {
			return DetectedShape{}, false
		}
		if math.Hypot(cx2-cx, cy2-cy) > ctol || math.Abs(r2-r) > ctol {
			return DetectedShape{}, false
		}
	}
	// All endpoints must lie on the fitted circle.
	tol := arcRadiusTolerance(r)
	for _, p := range pts {
		if math.Abs(math.Hypot(p.X-cx, p.Y-cy)-r) > tol {
			return DetectedShape{}, false
		}
	}
	// Each cubic's first control point must lie on the tangent at its start
	// endpoint (perpendicular to the radius). True for the circular-arc cubic
	// approximation DrawArc emits; ellipse arcs deviate (tangent direction is
	// not perpendicular to the radius in general) and are rejected here.
	ci = 2
	for i := 1; i < n; i++ {
		p := pts[i-1]
		tx, ty := coords[ci]-p.X, coords[ci+1]-p.Y // c1 offset
		rx, ry := p.X-cx, p.Y-cy
		tl := math.Hypot(tx, ty)
		rl := math.Hypot(rx, ry)
		if tl < 1e-9 || rl < 1e-9 {
			return DetectedShape{}, false
		}
		if math.Abs(tx*rx+ty*ry) > tanDotTol*tl*rl {
			return DetectedShape{}, false
		}
		ci += 6
	}

	a0 := math.Atan2(pts[0].Y-cy, pts[0].X-cx)
	a1 := math.Atan2(pts[len(pts)-1].Y-cy, pts[len(pts)-1].X-cx)
	// Normalize like DrawArc: sweep must be positive and less than a full turn.
	for a1 < a0 {
		a1 += 2 * math.Pi
	}
	if a1-a0 >= 2*math.Pi-shapeDetectTolerance {
		return DetectedShape{}, false // full circle, not an open arc
	}

	return DetectedShape{
		Kind: ShapeArc, CenterX: cx, CenterY: cy, RadiusX: r, RadiusY: r,
		Angle0: a0, Angle1: a1,
	}, true
}

// ellipseArcFitTolerance bounds the implicit-quadratic residual (in equation
// units) accepted for an axis-aligned elliptical arc.
var ellipseArcFitTolerance = 0.01

// ellipseArcTanTol bounds |cos(angle between cubic control offset and the
// ellipse tangent)| for elliptical-arc segments (the kappa construction is
// exact; the least-squares radius fit adds ~0.1px noise → ~2e-3 rad — but the
// per-axis kappa approximation of a flat ellipse (rx/ry ~1.6) adds up to
// ~5e-3 rad, so the combined tolerance must stay above that).
const ellipseArcTanTol = 0.01

// detectEllipticalArc checks for an open axis-aligned elliptical arc:
// MoveTo + n×CubicTo, no Close, matching DrawEllipticalArc's kappa cubic
// construction (per-axis radii rx/ry, parameter angles a0..a1). The ellipse
// is recovered with a linear least-squares fit of the implicit equation
// A·x² + B·y² + C·x + D·y + E = 0 (axis-aligned, no xy term) over sampled
// curve points, then verified by residual, tangent, and parameter-sweep
// consistency. Rotated ellipses / arbitrary curves fail the fit and stay
// ShapeUnknown (GPU falls back to the expand/stencil path).
func detectEllipticalArc(verbs []PathVerb, coords []float64) (DetectedShape, bool) {
	n := len(verbs)
	if n < 2 {
		return DetectedShape{}, false
	}
	for i := 1; i < n; i++ {
		if verbs[i] != CubicTo {
			return DetectedShape{}, false
		}
	}

	// Sample each cubic at t ∈ {0, 0.25, 0.5, 0.75, 1} (4n+1 unique points).
	pts := make([]Point, 0, 4*n+1)
	ci := 2
	prevX, prevY := coords[0], coords[1]
	for i := 1; i < n; i++ {
		c1x, c1y, c2x, c2y, x, y := coords[ci], coords[ci+1], coords[ci+2], coords[ci+3], coords[ci+4], coords[ci+5]
		ci += 6
		for _, t := range []float64{0, 0.25, 0.5, 0.75, 1}[1:] {
			mt := 1 - t
			a, b, c := mt*mt*mt, 3*mt*mt*t, 3*mt*t*t
			d := t * t * t
			pts = append(pts, Pt(a*prevX+b*c1x+c*c2x+d*x, a*prevY+b*c1y+c*c2y+d*y))
		}
		prevX, prevY = x, y
	}
	if len(pts) < 5 {
		return DetectedShape{}, false
	}

	// Least squares on the implicit axis-aligned ellipse A·x² + B·y² + C·x +
	// D·y + E' = 0 with the scale fixed by A + B = 1 (substituting A = 1−B
	// turns the homogeneous problem into a well-conditioned linear fit with
	// rhs = −x²; the E'=1 normalization degenerates for partial-arc data).
	// Data is shifted by the point-cloud centroid and scaled to [-1,1] so the
	// normal matrix stays well-conditioned; the fitted (u,v) = ((x-mx)/s,
	// (y-my)/s) ellipse is converted back to pixel space afterwards.
	mx, my := 0.0, 0.0
	minX, maxX, minY, maxY := pts[0].X, pts[0].X, pts[0].Y, pts[0].Y
	for _, p := range pts {
		mx += p.X
		my += p.Y
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	mx /= float64(len(pts))
	my /= float64(len(pts))
	s := math.Max(maxX-minX, maxY-minY) / 2
	if s < 1e-9 {
		return DetectedShape{}, false
	}

	// Solve (B,C,D,E') from (−u²+v²)·B + u·C + v·D + E' = −u².
	var m [4][4]float64
	var b [4]float64
	for _, p := range pts {
		u := (p.X - mx) / s
		v := (p.Y - my) / s
		u2, v2 := u*u, v*v
		col := [4]float64{-u2 + v2, u, v, 1}
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				m[i][j] += col[i] * col[j]
			}
			b[i] += col[i] * (-u2)
		}
	}
	return solveEllipseFit(m, b, pts, verbs, coords, n, mx, my, s)
}

// solveEllipseFit solves the 4x4 normal system (in centroid-normalized
// (u,v) coords with the A+B=1 scale) and validates the recovered
// axis-aligned ellipse against the sampled curve points.
func solveEllipseFit(m [4][4]float64, b [4]float64, pts []Point, verbs []PathVerb, coords []float64, n int, mx, my, s float64) (DetectedShape, bool) {
	// Gaussian elimination (partial pivot) on the augmented 4x4.
	a := m
	rhs := b
	for col := 0; col < 4; col++ {
		piv := col
		for r := col + 1; r < 4; r++ {
			if math.Abs(a[r][col]) > math.Abs(a[piv][col]) {
				piv = r
			}
		}
		if math.Abs(a[piv][col]) < 1e-14 {
			return DetectedShape{}, false
		}
		if piv != col {
			for j := 0; j < 4; j++ {
				a[piv][j], a[col][j] = a[col][j], a[piv][j]
			}
			rhs[piv], rhs[col] = rhs[col], rhs[piv]
		}
		for r := col + 1; r < 4; r++ {
			f := a[r][col] / a[col][col]
			for j := col; j < 4; j++ {
				a[r][j] -= f * a[col][j]
			}
			rhs[r] -= f * rhs[col]
		}
	}
	sol := [4]float64{}
	for i := 3; i >= 0; i-- {
		sv := rhs[i]
		for j := i + 1; j < 4; j++ {
			sv -= a[i][j] * sol[j]
		}
		sol[i] = sv / a[i][i]
	}
	B, C, D, E := sol[0], sol[1], sol[2], sol[3]
	A := 1 - B
	// Convert to center/radii (u,v) → pixel space; require a real
	// axis-aligned ellipse.
	if A <= 0 || B <= 0 {
		return DetectedShape{}, false
	}
	uc := -C / (2 * A)
	vc := -D / (2 * B)
	F := A*uc*uc + B*vc*vc - E
	if F <= 0 {
		return DetectedShape{}, false
	}
	cx := mx + uc*s
	cy := my + vc*s
	rx := s * math.Sqrt(F/A)
	ry := s * math.Sqrt(F/B)
	if rx < shapeDetectTolerance || ry < shapeDetectTolerance || ry < rx*1e-2 {
		return DetectedShape{}, false
	}

	// Residual check on every sampled point (normalized equation units;
	// 0.01 ≈ 0.45px deviation for a 90px-radius ellipse).
	for _, p := range pts {
		u := (p.X - mx) / s
		v := (p.Y - my) / s
		res := A*u*u + B*v*v + C*u + D*v + E
		if math.Abs(res) > ellipseArcFitTolerance {
			return DetectedShape{}, false
		}
	}

	// Tangent consistency: each cubic's entry control point must be parallel
	// to the ellipse tangent at the segment start (the kappa construction).
	ci := 2
	prevX, prevY := coords[0], coords[1]
	for i := 1; i < n; i++ {
		c1x, c1y := coords[ci], coords[ci+1]
		ci += 6
		tx, ty := c1x-prevX, c1y-prevY
		nx, ny := (prevX-cx)/(rx*rx), (prevY-cy)/(ry*ry) // ellipse normal direction
		tl := math.Hypot(tx, ty)
		nl := math.Hypot(nx, ny)
		if tl < 1e-9 || nl < 1e-9 {
			return DetectedShape{}, false
		}
		if math.Abs(tx*nx+ty*ny) > ellipseArcTanTol*tl*nl {
			return DetectedShape{}, false
		}
		prevX, prevY = coords[ci-2], coords[ci-1]
	}

	// Parameter-sweep consistency: all sampled points' parameter angles
	// θ = atan2((y−cy)/ry, (x−cx)/rx) lie in [a0, a1] (wrap-normalized).
	// a0/a1 come from the TRUE path endpoints (MoveTo point and last cubic's
	// endpoint), not interior samples — otherwise the SDF angle window would
	// cut ~(t_start·segment) radians off the arc's start cap.
	startIdx := len(coords) - 2
	a0 := math.Atan2((coords[1]-cy)/ry, (coords[0]-cx)/rx)
	a1 := math.Atan2((coords[startIdx+1]-cy)/ry, (coords[startIdx]-cx)/rx)
	for a1 < a0 {
		a1 += 2 * math.Pi
	}
	if a1-a0 >= 2*math.Pi-shapeDetectTolerance {
		return DetectedShape{}, false
	}
	const sweepTol = 0.02
	for _, p := range pts {
		th := math.Atan2((p.Y-cy)/ry, (p.X-cx)/rx)
		for th < a0-sweepTol {
			th += 2 * math.Pi
		}
		if th > a1+sweepTol {
			return DetectedShape{}, false
		}
	}

	return DetectedShape{
		Kind: ShapeArc, CenterX: cx, CenterY: cy, RadiusX: rx, RadiusY: ry,
		Angle0: a0, Angle1: a1,
	}, true
}

// arcRadiusTolerance returns the endpoint radius check tolerance for an arc
// of radius r: absolute floor plus a small relative allowance.
func arcRadiusTolerance(r float64) float64 {
	t := shapeDetectTolerance
	if rt := r * 1e-4; rt > t {
		t = rt
	}
	return t
}

// arcCenterTolerance returns the cross-triple center-consistency tolerance for
// an arc of radius r. Tighter than the radius check: circular arcs fit stably,
// ellipse arcs drift the fitted center well beyond this.
func arcCenterTolerance(r float64) float64 {
	t := shapeDetectTolerance * 10
	if rt := r * 1e-3; rt > t {
		t = rt
	}
	return t
}

// circleFromPoints returns the center (cx, cy) and radius r of the circle
// through three non-collinear points, or ok=false for degenerate input.
func circleFromPoints(p0, p1, p2 Point) (cx, cy, r float64, ok bool) {
	// Perpendicular bisectors of p0-p1 and p1-p2.
	ax, ay := p1.X-p0.X, p1.Y-p0.Y // direction of chord p0→p1
	bx, by := p2.X-p1.X, p2.Y-p1.Y // direction of chord p1→p2
	mx01, my01 := (p0.X+p1.X)/2, (p0.Y+p1.Y)/2
	mx12, my12 := (p1.X+p2.X)/2, (p1.Y+p2.Y)/2

	// Solve: mid01 + t·perp(ab) == mid12 + s·perp(bc), where perp(v)=( -v.y, v.x ).
	// perp(ab) = (-ay, ax); perp(bc) = (-by, bx).
	// Matrix: [ -ay,  by ] [t]   [ mx12-mx01 ]
	//         [  ax, -bx ] [s] = [ my12-my01 ]
	det := (-ay)*(-bx) - (by)*(ax) // (-ay)(-bx) - (by)(ax) = ay·bx - by·ax
	if math.Abs(det) < 1e-12 {
		return 0, 0, 0, false
	}
	dx := mx12 - mx01
	dy := my12 - my01
	// Cramer's rule
	t := (dx*(-bx) - (by)*dy) / det
	cx = mx01 + t*(-ay)
	cy = my01 + t*ax
	r = math.Hypot(p0.X-cx, p0.Y-cy)
	return cx, cy, r, true
}

// pointsClose checks if two points are within tolerance.
func pointsClose(a, b Point) bool {
	return math.Abs(a.X-b.X) < shapeDetectTolerance && math.Abs(a.Y-b.Y) < shapeDetectTolerance
}
