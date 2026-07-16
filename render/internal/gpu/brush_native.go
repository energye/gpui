//go:build !nogpu

package gpu

import (
	"math"

	"github.com/energye/gpui/render"
)

// gradientEdgeSamples densifies convex outlines so multi-stop gradients
// approximate stop colors under Gouraud shading (P0-2).
const gradientEdgeSamples = 16

// fillBrushNative tries GPU-native gradient/pattern fills without large-area
// CPU ColorAt rasterization. Returns ErrFallbackToCPU when the brush/path
// combination is not supported (caller uses fillBrushAsImage bootstrap).
func (rc *GPURenderContext) fillBrushNative(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	if path == nil || path.NumVerbs() == 0 || paint == nil {
		return nil
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}

	// G.03: ImagePattern tiled/clamped texture fill on axis-aligned rect paths.
	if pat, ok := paint.Pattern.(*render.ImagePattern); ok {
		if err := rc.fillImagePatternNative(target, path, paint, pat); err == nil {
			return nil
		}
		// Non-rect / rotated / complex path: ColorAt stage + GPU blit.
		if err := rc.fillBrushAsImage(target, path, paint); err == nil {
			rc.noteBrushBootstrap("brush:pattern-path")
			return nil
		}
		return render.ErrFallbackToCPU
	}

	// EvenOdd / non-convex: no Gouraud fan; bootstrap with reason (not silent).
	if paint.FillRule == render.FillRuleEvenOdd {
		if err := rc.fillBrushAsImage(target, path, paint); err == nil {
			rc.noteBrushBootstrap("brush:evenodd")
			return nil
		}
		return render.ErrFallbackToCPU
	}

	// G.02: Linear / Radial / Sweep gradients.
	brush := paint.GetBrush()
	switch b := brush.(type) {
	case *render.LinearGradientBrush:
		// H/V linear + AA rect: 1D ColorAt ramp (all Extend modes).
		if err := rc.fillLinearGradientSpanNative(target, path, paint, b); err == nil {
			return nil
		}
		// Diagonal (or any orientation) linear + AA rect: 2D ColorAt field.
		if err := rc.fillLinearGradientFieldNative(target, path, paint, b); err == nil {
			return nil
		}
		// Pad + convex: Gouraud densify.
		if err := rc.fillGradientConvexNative(target, path, paint, brush); err == nil {
			return nil
		}
		// Non-convex / complex outline: software coverage + GPU blit.
		if err := rc.fillBrushAsImage(target, path, paint); err == nil {
			rc.noteBrushBootstrap("brush:nonconvex-path")
			return nil
		}
		return render.ErrFallbackToCPU
	case *render.RadialGradientBrush, *render.SweepGradientBrush:
		// Radial/sweep on AA rect: 2D field (all Extend); else Pad convex Gouraud.
		if err := rc.fillRadialSweepFieldNative(target, path, paint, brush); err == nil {
			return nil
		}
		if err := rc.fillGradientConvexNative(target, path, paint, brush); err == nil {
			return nil
		}
		if err := rc.fillBrushAsImage(target, path, paint); err == nil {
			rc.noteBrushBootstrap("brush:nonconvex-path")
			return nil
		}
		return render.ErrFallbackToCPU
	case render.CustomBrush:
		// G.04: no fragment ColorAt on GPU — bootstrap ColorAt stage + GPU blit,
		// with explicit reason (not silent).
		if err := rc.fillBrushAsImage(target, path, paint); err == nil {
			rc.noteBrushBootstrap("brush:custom")
			return nil
		}
		return render.ErrFallbackToCPU
	default:
		// Other ColorAt brushes (unknown) — same bootstrap with reason.
		if brush != nil {
			if err := rc.fillBrushAsImage(target, path, paint); err == nil {
				rc.noteBrushBootstrap("brush:colorat")
				return nil
			}
		}
		return render.ErrFallbackToCPU
	}
}

func (rc *GPURenderContext) fillGradientConvexNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	brush render.Brush,
) error {
	// Gouraud VertexColors cannot express ExtendRepeat/Reflect discontinuities
	// (color wraps in parameter space). Those stay on fillBrushAsImage bootstrap.
	if gradientExtend(brush) != render.ExtendPad {
		return render.ErrFallbackToCPU
	}

	var points []render.Point
	var ok bool
	if cache := rc.shared.ConvexPathCache(); cache != nil {
		points, ok = cache.GetOrClassify(path)
	} else {
		points, ok = extractConvexPolygon(path)
	}
	if !ok || len(points) < 3 {
		return render.ErrFallbackToCPU
	}

	// Multi-stop: densify edges so Gouraud segments track stop colors.
	samples := 1
	if gradientStopCount(brush) > 2 {
		samples = gradientEdgeSamples
	}
	// Radial/sweep always densify — curvature of color field is high.
	switch brush.(type) {
	case *render.RadialGradientBrush, *render.SweepGradientBrush:
		if samples < gradientEdgeSamples {
			samples = gradientEdgeSamples
		}
	}
	if samples > 1 {
		points = densifyConvexOutline(points, samples)
	}

	colors := make([][4]float32, len(points))
	for i, pt := range points {
		c := brush.ColorAt(pt.X, pt.Y)
		colors[i] = premulRGBA(c)
	}

	// Fan hub color: ColorAt(centroid) for radial/sweep; mean is fine for linear.
	var cx, cy float64
	for _, pt := range points {
		cx += pt.X
		cy += pt.Y
	}
	invN := 1.0 / float64(len(points))
	cx *= invN
	cy *= invN
	cHub := premulRGBA(brush.ColorAt(cx, cy))

	cmd := ConvexDrawCommand{
		Points:           points,
		Color:            cHub,
		VertexColors:     colors,
		HasCentroidColor: true,
		CentroidColor:    cHub,
		BlendMode:        paintBlendMode(paint),
	}
	rc.QueueConvex(target, cmd)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

func gradientStopCount(brush render.Brush) int {
	switch g := brush.(type) {
	case *render.LinearGradientBrush:
		return len(g.Stops)
	case *render.RadialGradientBrush:
		return len(g.Stops)
	case *render.SweepGradientBrush:
		return len(g.Stops)
	default:
		return 0
	}
}

func gradientExtend(brush render.Brush) render.ExtendMode {
	switch g := brush.(type) {
	case *render.LinearGradientBrush:
		return g.Extend
	case *render.RadialGradientBrush:
		return g.Extend
	case *render.SweepGradientBrush:
		return g.Extend
	default:
		return render.ExtendPad
	}
}

func densifyConvexOutline(points []render.Point, samplesPerEdge int) []render.Point {
	if samplesPerEdge < 1 {
		samplesPerEdge = 1
	}
	n := len(points)
	if n < 3 {
		return points
	}
	out := make([]render.Point, 0, n*samplesPerEdge)
	for i := 0; i < n; i++ {
		a := points[i]
		b := points[(i+1)%n]
		for s := 0; s < samplesPerEdge; s++ {
			t := float64(s) / float64(samplesPerEdge)
			out = append(out, render.Point{
				X: a.X*(1-t) + b.X*t,
				Y: a.Y*(1-t) + b.Y*t,
			})
		}
	}
	return out
}

func premulRGBA(c render.RGBA) [4]float32 {
	return [4]float32{
		float32(c.R * c.A),
		float32(c.G * c.A),
		float32(c.B * c.A),
		float32(c.A),
	}
}

// fillLinearGradientSpanNative fills an axis-aligned rect with a pure
// horizontal or vertical LinearGradient using a 1D ColorAt ramp uploaded as a
// GPU texture (G.02). Supports ExtendPad/Repeat/Reflect because ColorAt already
// applies extend; Gouraud cannot express wrap discontinuities.
func (rc *GPURenderContext) fillLinearGradientSpanNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	g *render.LinearGradientBrush,
) error {
	if g == nil || path == nil {
		return render.ErrFallbackToCPU
	}
	rx, ry, rw, rh, ok := pathAxisAlignedRect(path)
	if !ok || rw < 1 || rh < 1 {
		return render.ErrFallbackToCPU
	}
	dx := g.End.X - g.Start.X
	dy := g.End.Y - g.Start.Y
	absdx, absdy := math.Abs(dx), math.Abs(dy)
	const eps = 1e-6
	horizontal := absdy <= eps && absdx > eps
	vertical := absdx <= eps && absdy > eps
	if !horizontal && !vertical {
		return render.ErrFallbackToCPU
	}

	// Sample density: ~1 sample per dest pixel, clamped for cost.
	n := 256
	if horizontal {
		if int(rw) > n {
			n = int(rw)
		}
	} else if int(rh) > n {
		n = int(rh)
	}
	if n < 32 {
		n = 32
	}
	if n > 2048 {
		n = 2048
	}

	ramp := make([]byte, n*4)
	if horizontal {
		midY := ry + rh*0.5
		for i := 0; i < n; i++ {
			x := rx + (float64(i)+0.5)/float64(n)*rw
			c := g.ColorAt(x, midY)
			off := i * 4
			// premul RGBA8
			a := c.A
			ramp[off+0] = uint8(clamp255(c.R * a * 255))
			ramp[off+1] = uint8(clamp255(c.G * a * 255))
			ramp[off+2] = uint8(clamp255(c.B * a * 255))
			ramp[off+3] = uint8(clamp255(a * 255))
		}
	} else {
		midX := rx + rw*0.5
		for i := 0; i < n; i++ {
			y := ry + (float64(i)+0.5)/float64(n)*rh
			c := g.ColorAt(midX, y)
			off := i * 4
			a := c.A
			ramp[off+0] = uint8(clamp255(c.R * a * 255))
			ramp[off+1] = uint8(clamp255(c.G * a * 255))
			ramp[off+2] = uint8(clamp255(c.B * a * 255))
			ramp[off+3] = uint8(clamp255(a * 255))
		}
	}

	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}

	// Unique gen per span content so image atlas does not stale-hit wrong ramp.
	genID := linearRampGenID(g, rx, ry, rw, rh, n, horizontal)

	fx0 := float32(rx)
	fy0 := float32(ry)
	fx1 := float32(rx + rw)
	fy1 := float32(ry + rh)
	if horizontal {
		// n x 1 ramp stretched over rect.
		rc.QueueImageDraw(target, ramp, genID, n, 1, n*4,
			fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
			1, vpW, vpH,
			0, 0, 1, 1,
			false,
		)
	} else {
		// 1 x n ramp.
		rc.QueueImageDraw(target, ramp, genID, 1, n, 4,
			fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
			1, vpW, vpH,
			0, 0, 1, 1,
			false,
		)
	}
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

func clamp255(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func linearRampGenID(g *render.LinearGradientBrush, rx, ry, rw, rh float64, n int, horizontal bool) uint64 {
	// FNV-ish mix of geometry + stops + extend (stable, collision-resistant enough).
	h := uint64(0xcbf29ce484222325)
	mix := func(v uint64) {
		h ^= v
		h *= 0x100000001b3
	}
	mix(math.Float64bits(g.Start.X))
	mix(math.Float64bits(g.Start.Y))
	mix(math.Float64bits(g.End.X))
	mix(math.Float64bits(g.End.Y))
	mix(uint64(g.Extend))
	mix(math.Float64bits(rx))
	mix(math.Float64bits(ry))
	mix(math.Float64bits(rw))
	mix(math.Float64bits(rh))
	mix(uint64(n))
	if horizontal {
		mix(1)
	} else {
		mix(2)
	}
	for _, s := range g.Stops {
		mix(math.Float64bits(s.Offset))
		mix(math.Float64bits(s.Color.R))
		mix(math.Float64bits(s.Color.G))
		mix(math.Float64bits(s.Color.B))
		mix(math.Float64bits(s.Color.A))
	}
	// Namespace away from pixmap GenerationIDs.
	return h ^ 0x6022a39000000000
}

// fillLinearGradientFieldNative fills an AA rect with a linear gradient of any
// orientation (including diagonal) via a 2D ColorAt sample grid uploaded as a
// GPU texture (G.02 residual). Supports all Extend modes.
func (rc *GPURenderContext) fillLinearGradientFieldNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	g *render.LinearGradientBrush,
) error {
	if g == nil {
		return render.ErrFallbackToCPU
	}
	return rc.fillColorAtFieldNative(target, path, paint, g.ColorAt, brushFieldSeedLinear(g))
}

// fillRadialSweepFieldNative samples radial/sweep ColorAt over an AA rect.
func (rc *GPURenderContext) fillRadialSweepFieldNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	brush render.Brush,
) error {
	if brush == nil {
		return render.ErrFallbackToCPU
	}
	switch b := brush.(type) {
	case *render.RadialGradientBrush:
		return rc.fillColorAtFieldNative(target, path, paint, b.ColorAt, brushFieldSeedRadial(b))
	case *render.SweepGradientBrush:
		return rc.fillColorAtFieldNative(target, path, paint, b.ColorAt, brushFieldSeedSweep(b))
	default:
		return render.ErrFallbackToCPU
	}
}

// fillColorAtFieldNative rasterizes ColorAt on an AA-rect grid (cap 512²) and
// GPU-blits the result. Prefer over full software path fill for gradient fields.
func (rc *GPURenderContext) fillColorAtFieldNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	colorAt func(x, y float64) render.RGBA,
	genSeed uint64,
) error {
	if colorAt == nil || path == nil {
		return render.ErrFallbackToCPU
	}
	rx, ry, rw, rh, ok := pathAxisAlignedRect(path)
	if !ok || rw < 1 || rh < 1 {
		return render.ErrFallbackToCPU
	}
	const maxSide = 512
	nw := int(rw + 0.5)
	nh := int(rh + 0.5)
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	data := make([]byte, nw*nh*4)
	for y := 0; y < nh; y++ {
		py := ry + (float64(y)+0.5)/float64(nh)*rh
		row := y * nw * 4
		for x := 0; x < nw; x++ {
			px := rx + (float64(x)+0.5)/float64(nw)*rw
			c := colorAt(px, py)
			off := row + x*4
			a := c.A
			data[off+0] = uint8(clamp255(c.R * a * 255))
			data[off+1] = uint8(clamp255(c.G * a * 255))
			data[off+2] = uint8(clamp255(c.B * a * 255))
			data[off+3] = uint8(clamp255(a * 255))
		}
	}

	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := genSeed ^ fieldGeomGenID(rx, ry, rw, rh, nw, nh)

	fx0 := float32(rx)
	fy0 := float32(ry)
	fx1 := float32(rx + rw)
	fy1 := float32(ry + rh)
	rc.QueueImageDraw(target, data, genID, nw, nh, nw*4,
		fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
		1, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

func fieldGeomGenID(rx, ry, rw, rh float64, nw, nh int) uint64 {
	h := uint64(0xcbf29ce484222325)
	mix := func(v uint64) {
		h ^= v
		h *= 0x100000001b3
	}
	mix(math.Float64bits(rx))
	mix(math.Float64bits(ry))
	mix(math.Float64bits(rw))
	mix(math.Float64bits(rh))
	mix(uint64(nw))
	mix(uint64(nh))
	return h
}

func brushFieldSeedLinear(g *render.LinearGradientBrush) uint64 {
	h := uint64(0x61202a1100000001)
	mix := func(v uint64) {
		h ^= v
		h *= 0x100000001b3
	}
	mix(math.Float64bits(g.Start.X))
	mix(math.Float64bits(g.Start.Y))
	mix(math.Float64bits(g.End.X))
	mix(math.Float64bits(g.End.Y))
	mix(uint64(g.Extend))
	for _, s := range g.Stops {
		mix(math.Float64bits(s.Offset))
		mix(math.Float64bits(s.Color.R))
		mix(math.Float64bits(s.Color.G))
		mix(math.Float64bits(s.Color.B))
		mix(math.Float64bits(s.Color.A))
	}
	return h
}

func brushFieldSeedRadial(g *render.RadialGradientBrush) uint64 {
	h := uint64(0x61202a1200000001)
	mix := func(v uint64) {
		h ^= v
		h *= 0x100000001b3
	}
	mix(math.Float64bits(g.Center.X))
	mix(math.Float64bits(g.Center.Y))
	mix(math.Float64bits(g.Focus.X))
	mix(math.Float64bits(g.Focus.Y))
	mix(math.Float64bits(g.StartRadius))
	mix(math.Float64bits(g.EndRadius))
	mix(uint64(g.Extend))
	for _, s := range g.Stops {
		mix(math.Float64bits(s.Offset))
		mix(math.Float64bits(s.Color.R))
		mix(math.Float64bits(s.Color.G))
		mix(math.Float64bits(s.Color.B))
		mix(math.Float64bits(s.Color.A))
	}
	return h
}

func brushFieldSeedSweep(g *render.SweepGradientBrush) uint64 {
	h := uint64(0x61202a1300000001)
	mix := func(v uint64) {
		h ^= v
		h *= 0x100000001b3
	}
	mix(math.Float64bits(g.Center.X))
	mix(math.Float64bits(g.Center.Y))
	mix(math.Float64bits(g.StartAngle))
	mix(math.Float64bits(g.EndAngle))
	mix(uint64(g.Extend))
	for _, s := range g.Stops {
		mix(math.Float64bits(s.Offset))
		mix(math.Float64bits(s.Color.R))
		mix(math.Float64bits(s.Color.G))
		mix(math.Float64bits(s.Color.B))
		mix(math.Float64bits(s.Color.A))
	}
	return h
}

// fillImagePatternNative draws ImagePattern as GPU textured quads over the
// path's axis-aligned bounds. Only axis-aligned rectangular paths are accepted
// so coverage matches the path without a soft mask (G.03).
func (rc *GPURenderContext) fillImagePatternNative(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	pat *render.ImagePattern,
) error {
	img, srcX, srcY, srcW, srcH, inv, opacity, clamp := pat.GPUPatternSource()
	if img == nil || srcW <= 0 || srcH <= 0 {
		return render.ErrFallbackToCPU
	}
	rx, ry, rw, rh, ok := pathAxisAlignedRect(path)
	if !ok {
		return render.ErrFallbackToCPU
	}

	// Extract source region premul RGBA.
	fullW, fullH := img.Bounds()
	if srcX < 0 || srcY < 0 || srcX+srcW > fullW || srcY+srcH > fullH {
		return render.ErrFallbackToCPU
	}
	data := img.PremultipliedData()
	if len(data) < fullW*fullH*4 {
		data = img.Data()
	}
	stride := fullW * 4
	tile := make([]byte, srcW*srcH*4)
	for row := 0; row < srcH; row++ {
		srcOff := (srcY+row)*stride + srcX*4
		dstOff := row * srcW * 4
		copy(tile[dstOff:dstOff+srcW*4], data[srcOff:srcOff+srcW*4])
	}
	genID := img.GenerationID()

	// Forward map: image-space → device-space via inverse.Invert().
	// Affine: x' = A*x + B*y + C; y' = D*x + E*y + F.
	fwd := inv.Invert()
	sx := math.Hypot(fwd.A, fwd.D) // |basis X|
	sy := math.Hypot(fwd.B, fwd.E) // |basis Y|
	if sx < 1e-6 {
		sx = 1
	}
	if sy < 1e-6 {
		sy = 1
	}
	tileDW := float64(srcW) * sx
	tileDH := float64(srcH) * sy
	if tileDW < 1e-3 || tileDH < 1e-3 {
		return render.ErrFallbackToCPU
	}

	// Origin of pattern in device space: image (0,0) → device.
	origin := fwd.TransformPoint(render.Pt(0, 0))

	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	op := float32(opacity)
	if op < 0 {
		op = 0
	}
	if op > 1 {
		op = 1
	}

	// Support pure translate+scale (no rotation/skew) for AABB tile grid.
	if math.Abs(fwd.B) > 1e-5 || math.Abs(fwd.D) > 1e-5 {
		// Rotated/skewed pattern: single clamp quad only.
		if !clamp {
			return render.ErrFallbackToCPU
		}
		return rc.queuePatternQuad(target, tile, genID, srcW, srcH,
			rx, ry, rx+rw, ry, rx+rw, ry+rh, rx, ry+rh,
			op, vpW, vpH, inv, float64(srcW), float64(srcH))
	}

	minIX := int(math.Floor((rx - origin.X) / tileDW))
	maxIX := int(math.Floor((rx + rw - origin.X - 1e-9) / tileDW))
	minIY := int(math.Floor((ry - origin.Y) / tileDH))
	maxIY := int(math.Floor((ry + rh - origin.Y - 1e-9) / tileDH))
	if clamp {
		// Only the (0,0) tile in image space; clip to dest.
		minIX, maxIX, minIY, maxIY = 0, 0, 0, 0
	}
	// Cap tile count for safety.
	const maxTiles = 4096
	nTiles := (maxIX - minIX + 1) * (maxIY - minIY + 1)
	if nTiles <= 0 || nTiles > maxTiles {
		return render.ErrFallbackToCPU
	}

	for iy := minIY; iy <= maxIY; iy++ {
		for ix := minIX; ix <= maxIX; ix++ {
			x0 := origin.X + float64(ix)*tileDW
			y0 := origin.Y + float64(iy)*tileDH
			x1 := x0 + tileDW
			y1 := y0 + tileDH
			// Clip tile to path rect.
			cx0 := math.Max(x0, rx)
			cy0 := math.Max(y0, ry)
			cx1 := math.Min(x1, rx+rw)
			cy1 := math.Min(y1, ry+rh)
			if cx1-cx0 < 1e-4 || cy1-cy0 < 1e-4 {
				continue
			}
			// UV within tile for clipped sub-rect.
			u0 := float32((cx0 - x0) / tileDW)
			v0 := float32((cy0 - y0) / tileDH)
			u1 := float32((cx1 - x0) / tileDW)
			v1 := float32((cy1 - y0) / tileDH)
			rc.QueueImageDraw(target, tile, genID, srcW, srcH, srcW*4,
				float32(cx0), float32(cy0), float32(cx1), float32(cy0),
				float32(cx1), float32(cy1), float32(cx0), float32(cy1),
				op, vpW, vpH,
				u0, v0, u1, v1,
				false,
			)
		}
	}
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

func (rc *GPURenderContext) queuePatternQuad(
	target render.GPURenderTarget,
	tile []byte, genID uint64, srcW, srcH int,
	tlX, tlY, trX, trY, brX, brY, blX, blY float64,
	op float32, vpW, vpH uint32,
	inv render.Matrix, imgW, imgH float64,
) error {
	// Map device TL/BR to image UV (AABB UV; clamp sampler handles edges).
	tl := inv.TransformPoint(render.Pt(tlX, tlY))
	br := inv.TransformPoint(render.Pt(brX, brY))
	u0 := float32(tl.X / imgW)
	v0 := float32(tl.Y / imgH)
	u1 := float32(br.X / imgW)
	v1 := float32(br.Y / imgH)
	rc.QueueImageDraw(target, tile, genID, srcW, srcH, srcW*4,
		float32(tlX), float32(tlY), float32(trX), float32(trY),
		float32(brX), float32(brY), float32(blX), float32(blY),
		op, vpW, vpH,
		u0, v0, u1, v1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// pathAxisAlignedRect reports whether path is an axis-aligned rectangle.
func pathAxisAlignedRect(path *render.Path) (x, y, w, h float64, ok bool) {
	if path == nil {
		return 0, 0, 0, 0, false
	}
	pts, isConvex := extractConvexPolygon(path)
	if !isConvex || len(pts) != 4 {
		// Also accept denser rect outlines? Require exact 4 for clean UV.
		// Try bounds for rectangle verb paths that extract as 4 points.
		if !isConvex || len(pts) < 4 {
			return 0, 0, 0, 0, false
		}
		// More than 4 points: use bounds only if all points lie on AABB.
		bb := path.BoundingBox()
		const eps = 0.51
		for _, p := range pts {
			onV := math.Abs(p.X-bb.Min.X) < eps || math.Abs(p.X-bb.Max.X) < eps
			onH := math.Abs(p.Y-bb.Min.Y) < eps || math.Abs(p.Y-bb.Max.Y) < eps
			if !onV && !onH {
				return 0, 0, 0, 0, false
			}
			if !((onV && p.Y >= bb.Min.Y-eps && p.Y <= bb.Max.Y+eps) ||
				(onH && p.X >= bb.Min.X-eps && p.X <= bb.Max.X+eps)) {
				return 0, 0, 0, 0, false
			}
		}
		return bb.Min.X, bb.Min.Y, bb.Max.X - bb.Min.X, bb.Max.Y - bb.Min.Y, true
	}
	// 4-point convex: check axis-aligned.
	minX, maxX := pts[0].X, pts[0].X
	minY, maxY := pts[0].Y, pts[0].Y
	for _, p := range pts[1:] {
		if p.X < minX {
			minX = p.X
		}
		if p.X > maxX {
			maxX = p.X
		}
		if p.Y < minY {
			minY = p.Y
		}
		if p.Y > maxY {
			maxY = p.Y
		}
	}
	const eps = 0.51
	for _, p := range pts {
		onCornerX := math.Abs(p.X-minX) < eps || math.Abs(p.X-maxX) < eps
		onCornerY := math.Abs(p.Y-minY) < eps || math.Abs(p.Y-maxY) < eps
		if !onCornerX || !onCornerY {
			return 0, 0, 0, 0, false
		}
	}
	return minX, minY, maxX - minX, maxY - minY, true
}
