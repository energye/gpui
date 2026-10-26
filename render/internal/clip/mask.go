//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package clip

import (
	"github.com/energye/gpui/render/internal/image"
)

// PathVerb represents a path construction command.
// Values match render.PathVerb for zero-cost conversion.
type PathVerb byte

const (
	// VerbMoveTo moves the current point. Consumes 2 coords (x, y).
	VerbMoveTo PathVerb = iota
	// VerbLineTo draws a line. Consumes 2 coords (x, y).
	VerbLineTo
	// VerbQuadTo draws a quadratic Bezier. Consumes 4 coords (cx, cy, x, y).
	VerbQuadTo
	// VerbCubicTo draws a cubic Bezier. Consumes 6 coords (c1x, c1y, c2x, c2y, x, y).
	VerbCubicTo
	// VerbClose closes the current subpath. Consumes 0 coords.
	VerbClose
)

// FillRule selects how path contour overlaps resolve when a clip mask is
// rasterized. Values match render.FillRule for zero-cost conversion
// (Skia: clip paths are filled using the path fill type).
type FillRule int

const (
	// FillRuleNonZero keeps pixels where the signed winding number is non-zero.
	FillRuleNonZero FillRule = iota
	// FillRuleEvenOdd keeps pixels crossed an odd number of times.
	FillRuleEvenOdd
)

// MaskClipper performs alpha mask-based clipping for anti-aliased complex clips.
// It rasterizes a path into a grayscale mask where each pixel's value represents
// coverage (0 = outside, 255 = fully inside).
type MaskClipper struct {
	mask   *image.ImageBuf
	bounds Rect

	// Cached raw Gray8 view of mask. Coverage is called once per destination
	// pixel by ensureClipMaskGPU; going through ImageBuf.Width/Height/GetRGBA
	// per pixel took an RWMutex three times per pixel and dominated the
	// full-surface clip plane build (R7). The mask is immutable after
	// NewMaskClipper returns, so the raw slice is a safe read-only view.
	data   []byte
	stride int
	width  int
	height int
}

// NewMaskClipper creates a mask clipper by rasterizing the given path
// (as SOA verb+coords slices) into an alpha mask.
//
// Parameters:
//   - verbs: Path verb stream
//   - coords: Path coordinate stream
//   - bounds: Bounding rectangle for the mask
//   - antiAlias: Enable anti-aliased rendering (currently always on)
//   - fillRule: How contour overlaps resolve (winding or even-odd)
//
// The mask is stored as FormatGray8 (1 byte per pixel) for memory efficiency.
func NewMaskClipper(verbs []PathVerb, coords []float64, bounds Rect, antiAlias bool, fillRule FillRule) (*MaskClipper, error) {
	// Validate bounds - empty bounds means no clipping needed
	if bounds.IsEmpty() {
		return nil, image.ErrInvalidDimensions
	}

	// Calculate mask dimensions (ceiling to ensure we cover all pixels)
	width := int(bounds.W + 0.5)
	height := int(bounds.H + 0.5)
	if width <= 0 || height <= 0 {
		return nil, image.ErrInvalidDimensions
	}

	// Create grayscale mask buffer
	mask, err := image.NewImageBuf(width, height, image.FormatGray8)
	if err != nil {
		return nil, err
	}

	mc := &MaskClipper{
		mask:   mask,
		bounds: bounds,
	}

	// Rasterize path into mask
	mc.rasterizePath(verbs, coords, antiAlias, fillRule)
	mc.cacheView()

	return mc, nil
}

// cacheView captures the raw Gray8 mask once so Coverage can index it without
// per-pixel ImageBuf lock traffic. Call after the mask is fully rasterized.
func (mc *MaskClipper) cacheView() {
	if mc == nil || mc.mask == nil {
		return
	}
	mc.data = mc.mask.Data()
	mc.stride = mc.mask.Stride()
	mc.width, mc.height = mc.mask.Width(), mc.mask.Height()
}

// Coverage returns the coverage value (0-255) at the given point.
// Points outside the mask bounds return 0 (no coverage).
func (mc *MaskClipper) Coverage(x, y float64) byte {
	if mc == nil {
		return 0
	}
	// Convert to mask coordinates.
	mx := x - mc.bounds.X
	my := y - mc.bounds.Y

	// Check bounds using the cached dimensions (avoids three RWMutex reads
	// per pixel that ImageBuf.Width/Height/GetRGBA would take).
	if mx < 0 || my < 0 || mx >= float64(mc.width) || my >= float64(mc.height) {
		return 0
	}
	ix := int(mx)
	iy := int(my)
	if mc.data == nil || mc.stride < mc.width || len(mc.data) < (mc.height-1)*mc.stride+mc.width {
		// Defensive fallback for a clipper constructed without the cached
		// view (should not happen; NewMaskClipper always fills it).
		gray, _, _, _ := mc.mask.GetRGBA(ix, iy) //nolint:dogsled // Gray8 format has r=g=b
		return gray
	}
	return mc.data[iy*mc.stride+ix]
}

// ApplyCoverage modulates the source alpha by the mask coverage at the given point.
// Returns the modulated alpha value (0-255).
func (mc *MaskClipper) ApplyCoverage(x, y float64, srcAlpha byte) byte {
	coverage := mc.Coverage(x, y)
	if coverage == 0 {
		return 0
	}
	if coverage == 255 {
		return srcAlpha
	}

	// Modulate: result = srcAlpha * coverage / 255
	// Use 16-bit math to avoid overflow
	result := (uint16(srcAlpha) * uint16(coverage)) / 255
	return byte(result)
}

// Bounds returns the bounding rectangle of the mask.
func (mc *MaskClipper) Bounds() Rect {
	return mc.bounds
}

// Mask returns the underlying grayscale image buffer.
// This is useful for debugging or advanced use cases.
func (mc *MaskClipper) Mask() *image.ImageBuf {
	return mc.mask
}

// rasterizePath converts path (verb+coords) into a coverage mask.
func (mc *MaskClipper) rasterizePath(verbs []PathVerb, coords []float64, antiAlias bool, fillRule FillRule) {
	if len(verbs) == 0 {
		return
	}

	// Flatten path into per-subpath point lists. Edges never connect across
	// MoveTo boundaries, and every subpath carries its closing edge
	// (explicit Close or the implicit close used by fill semantics).
	subpaths := mc.flattenSubpaths(verbs, coords)
	edgeCap := 0
	for _, sp := range subpaths {
		edgeCap += len(sp)
	}
	edges := make([]edge, 0, edgeCap)
	for _, sp := range subpaths {
		for i := 0; i+1 < len(sp); i++ {
			p0 := sp[i]
			p1 := sp[i+1]

			// Skip horizontal edges
			if p1.Y == p0.Y {
				continue
			}

			edges = append(edges, mc.makeEdge(p0, p1))
		}
	}

	if len(edges) == 0 {
		return
	}

	// Scanline rasterization
	if antiAlias {
		for y := 0; y < mc.mask.Height(); y++ {
			mc.rasterizeScanlineAA(edges, y, fillRule)
		}
	} else {
		for y := 0; y < mc.mask.Height(); y++ {
			mc.rasterizeScanline(edges, y, fillRule)
		}
	}
}

// edge represents a scanline edge for rasterization.
type edge struct {
	x0, y0 float64 // Start point
	x1, y1 float64 // End point
	dir    int     // Direction: +1 for down, -1 for up
}

// makeEdge creates an edge from two points, ensuring y0 < y1.
func (mc *MaskClipper) makeEdge(p0, p1 Point) edge {
	// Convert to mask coordinates
	x0 := p0.X - mc.bounds.X
	y0 := p0.Y - mc.bounds.Y
	x1 := p1.X - mc.bounds.X
	y1 := p1.Y - mc.bounds.Y

	if y0 > y1 {
		// Swap to ensure y0 < y1
		x0, x1 = x1, x0
		y0, y1 = y1, y0
		return edge{x0: x0, y0: y0, x1: x1, y1: y1, dir: -1}
	}
	return edge{x0: x0, y0: y0, x1: x1, y1: y1, dir: 1}
}

// rasterizeScanlineAA fills a single scanline with anti-aliasing using 4x
// Y-supersampling and fractional X-edge coverage. Each pixel is sampled at 4
// sub-scanlines (y+0.125, y+0.375, y+0.625, y+0.875), edge crossings are
// walked left-to-right accumulating the winding number, and coverage is
// averaged per pixel. Edge pixels get fractional coverage based on the exact
// intersection.
func (mc *MaskClipper) rasterizeScanlineAA(edges []edge, y int, fillRule FillRule) {
	width := mc.mask.Width()
	// Accumulate coverage from 4 sub-scanlines (each contributes 0-255/4).
	coverage := make([]uint16, width)

	subOffsets := [4]float64{0.125, 0.375, 0.625, 0.875}
	for _, off := range subOffsets {
		scanY := float64(y) + off

		// Find edge crossings at this sub-scanline.
		var crossings []crossing
		for _, e := range edges {
			if e.y0 <= scanY && scanY < e.y1 {
				t := (scanY - e.y0) / (e.y1 - e.y0)
				crossings = append(crossings, crossing{x: e.x0 + t*(e.x1-e.x0), dir: e.dir})
			}
		}
		if len(crossings) == 0 {
			continue
		}
		sortCrossings(crossings)

		// Fill spans where the winding number is inside the fill area.
		winding := 0
		inSpan := false
		spanStart := 0.0
		for _, c := range crossings {
			winding += c.dir
			inside := spanInside(winding, fillRule)
			if inside && !inSpan {
				spanStart = c.x
				inSpan = true
			} else if !inside && inSpan {
				emitSpanAA(coverage, spanStart, c.x, width)
				inSpan = false
			}
		}
	}

	// Write accumulated coverage to mask. Fetch the Gray8 row once: SetRGBA
	// takes PixelOffset's RLock plus premul-cache locks for every pixel, which
	// made a full-screen clip mask spend most of its time in lock traffic.
	row := mc.mask.RowBytes(y)
	if len(row) < width {
		return
	}
	for x := 0; x < width; x++ {
		if coverage[x] > 0 {
			c := coverage[x]
			if c > 255 {
				c = 255
			}
			row[x] = byte(c)
		}
	}
}

// emitSpanAA adds fractional coverage for one subsample of the span [x1, x2]
// to the coverage row. Each subsample contributes up to 64 per pixel.
func emitSpanAA(coverage []uint16, x1, x2 float64, width int) {
	if x2 <= x1 {
		return
	}
	// Clip the span to the mask columns.
	if x1 < 0 {
		x1 = 0
	}
	if x2 > float64(width) {
		x2 = float64(width)
	}
	if x2 <= x1 {
		return
	}

	px1 := int(x1)
	px2 := int(x2)
	if px2 >= width {
		px2 = width - 1
	}

	if px1 == px2 {
		// Single pixel span — coverage = span width.
		frac := x2 - x1
		if frac > 1.0 {
			frac = 1.0
		}
		coverage[px1] += uint16(frac * 64) // 64 = 255/4 ≈ per-subsample max
		return
	}

	// Left edge pixel: fractional coverage.
	leftFrac := 1.0 - (x1 - float64(px1))
	if leftFrac > 1.0 {
		leftFrac = 1.0
	}
	if leftFrac < 0 {
		leftFrac = 0
	}
	coverage[px1] += uint16(leftFrac * 64)

	// Interior pixels: full coverage for this subsample.
	for x := px1 + 1; x < px2; x++ {
		coverage[x] += 64 // 255/4
	}

	// Right edge pixel: fractional coverage.
	if px2 > px1 {
		rightFrac := x2 - float64(px2)
		if rightFrac > 1.0 {
			rightFrac = 1.0
		}
		if rightFrac < 0 {
			rightFrac = 0
		}
		coverage[px2] += uint16(rightFrac * 64)
	}
}

// crossing is an edge intersection with a scanline, carrying the edge
// direction so the winding number can be accumulated while walking spans.
type crossing struct {
	x   float64
	dir int
}

// spanInside reports whether a span with the accumulated winding number is
// inside the fill area for the given fill rule. Even-odd keeps odd crossing
// parity; non-zero keeps any non-zero winding sum.
func spanInside(winding int, rule FillRule) bool {
	if rule == FillRuleEvenOdd {
		return winding&1 != 0
	}
	return winding != 0
}

// sortCrossings orders crossings by x. Scanline lists are short, so a
// simple bubble sort keeps the hot path allocation-free.
func sortCrossings(cs []crossing) {
	for i := 0; i < len(cs)-1; i++ {
		for j := 0; j < len(cs)-i-1; j++ {
			if cs[j].x > cs[j+1].x {
				cs[j], cs[j+1] = cs[j+1], cs[j]
			}
		}
	}
}

// rasterizeScanline fills a single scanline honoring the fill rule: sorted
// edge crossings are walked left-to-right accumulating the winding number,
// and every span inside the fill area is filled solid.
func (mc *MaskClipper) rasterizeScanline(edges []edge, y int, fillRule FillRule) {
	scanY := float64(y) + 0.5

	// Find edges that cross this scanline
	var crossings []crossing
	for _, e := range edges {
		if e.y0 <= scanY && scanY < e.y1 {
			t := (scanY - e.y0) / (e.y1 - e.y0)
			crossings = append(crossings, crossing{x: e.x0 + t*(e.x1-e.x0), dir: e.dir})
		}
	}

	if len(crossings) == 0 {
		return
	}

	sortCrossings(crossings)

	// Fill spans where the winding number is inside the fill area.
	winding := 0
	inSpan := false
	spanStart := 0.0
	for _, c := range crossings {
		winding += c.dir
		inside := spanInside(winding, fillRule)
		if inside && !inSpan {
			spanStart = c.x
			inSpan = true
		} else if !inside && inSpan {
			mc.emitSpanPixel(y, spanStart, c.x)
			inSpan = false
		}
	}
}

// emitSpanPixel fills the pixels covered by span [x1, x2] on a scanline
// (non-AA path: every touched pixel gets full coverage).
func (mc *MaskClipper) emitSpanPixel(y int, x1, x2 float64) {
	width := mc.mask.Width()
	if x2 <= x1 {
		return
	}
	// Clip the span to the mask columns.
	if x1 < 0 {
		x1 = 0
	}
	if x2 > float64(width) {
		x2 = float64(width)
	}
	if x2 <= x1 {
		return
	}

	px1 := int(x1)
	px2 := int(x2)
	if px2 >= width {
		px2 = width - 1
	}
	if px2 < px1 {
		return
	}
	row := mc.mask.RowBytes(y)
	if len(row) < width {
		return
	}
	for x := px1; x <= px2; x++ {
		row[x] = 255
	}
}

// flattenSubpaths converts path (verb+coords) into per-subpath point lists.
// Subpaths stay independent: edges never connect across MoveTo boundaries,
// Close returns to the current subpath start (not the global first point),
// and an open trailing subpath receives the implicit closing edge used by
// fill semantics (Skia fills an unclosed contour as if it were closed).
func (mc *MaskClipper) flattenSubpaths(verbs []PathVerb, coords []float64) [][]Point {
	var subs [][]Point
	var cur []Point
	var pen Point
	hasPen := false
	ci := 0

	// closeSubpath finalizes the subpath in progress by closing it back to
	// its start point, stores it when it forms an area, and parks the pen on
	// the start point (SVG/Skia: after Close the current point is the
	// subpath start).
	closeSubpath := func() {
		if len(cur) >= 2 {
			start := cur[0]
			cur = append(cur, start)
			subs = append(subs, cur)
			pen = start
			hasPen = true
		}
		cur = nil
	}

	// beginOrContinue returns the subpath to append a drawing verb to,
	// starting one at the pen (or the implicit origin) when needed.
	beginOrContinue := func() []Point {
		if len(cur) == 0 {
			if !hasPen {
				pen = Point{} // Skia path iterator: implicit moveTo(0, 0)
				hasPen = true
			}
			cur = append(cur, pen)
		}
		return cur
	}

	for _, v := range verbs {
		switch v {
		case VerbMoveTo:
			closeSubpath()
			pen = Point{X: coords[ci], Y: coords[ci+1]}
			hasPen = true
			cur = append(cur, pen)
			ci += 2

		case VerbLineTo:
			cur = beginOrContinue()
			pen = Point{X: coords[ci], Y: coords[ci+1]}
			cur = append(cur, pen)
			ci += 2

		case VerbQuadTo:
			cur = beginOrContinue()
			ctrl := Point{X: coords[ci], Y: coords[ci+1]}
			end := Point{X: coords[ci+2], Y: coords[ci+3]}
			prev := pen
			steps := 10
			for i := 1; i <= steps; i++ {
				t := float64(i) / float64(steps)
				cur = append(cur, evalQuadraticBezier(prev, ctrl, end, t))
			}
			pen = end
			ci += 4

		case VerbCubicTo:
			cur = beginOrContinue()
			c1 := Point{X: coords[ci], Y: coords[ci+1]}
			c2 := Point{X: coords[ci+2], Y: coords[ci+3]}
			end := Point{X: coords[ci+4], Y: coords[ci+5]}
			prev := pen
			steps := 16
			for i := 1; i <= steps; i++ {
				t := float64(i) / float64(steps)
				cur = append(cur, evalCubicBezier(prev, c1, c2, end, t))
			}
			pen = end
			ci += 6

		case VerbClose:
			closeSubpath()
		}
	}

	// Trailing open subpath: filled as if closed.
	closeSubpath()

	return subs
}

// evalQuadraticBezier evaluates a quadratic Bezier curve at parameter t.
func evalQuadraticBezier(p0, p1, p2 Point, t float64) Point {
	s := 1 - t
	return Point{
		X: s*s*p0.X + 2*s*t*p1.X + t*t*p2.X,
		Y: s*s*p0.Y + 2*s*t*p1.Y + t*t*p2.Y,
	}
}

// evalCubicBezier evaluates a cubic Bezier curve at parameter t.
func evalCubicBezier(p0, p1, p2, p3 Point, t float64) Point {
	s := 1 - t
	s2 := s * s
	s3 := s2 * s
	t2 := t * t
	t3 := t2 * t
	return Point{
		X: s3*p0.X + 3*s2*t*p1.X + 3*s*t2*p2.X + t3*p3.X,
		Y: s3*p0.Y + 3*s2*t*p1.Y + 3*s*t2*p2.Y + t3*p3.Y,
	}
}

// sortFloats sorts a slice of float64 values (simple bubble sort for small slices).
func sortFloats(values []float64) {
	n := len(values)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if values[j] > values[j+1] {
				values[j], values[j+1] = values[j+1], values[j]
			}
		}
	}
}
