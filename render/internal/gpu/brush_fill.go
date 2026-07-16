//go:build !nogpu

package gpu

import (
	"image"
	"math"

	"github.com/energye/gpui/render"
)

// maxBrushFillPixels caps CPU staging textures for non-solid GPU fills.
// Above this, fall back to pure CPU on the context pixmap.
const maxBrushFillPixels = 8 * 1024 * 1024

// dualTexTileMax is the max pixels per dual-tex tile (G.07).
const dualTexTileMax = 512 * 512

// fillBrushCoverageColorAt is a GPU* path for non-solid brushes when native
// stages cannot run (non-convex / EvenOdd / custom):
//
//  1. Software-rasterize SOLID WHITE coverage only (fill rule + AA) — no ColorAt.
//  2. ColorAt only where coverage alpha > 0; premul by coverage.
//  3. GPU textured-quad blit (real GPUOps).
//
// GPU-FIRST: call only after fillBrushNative native stages fail. Faster/sparser
// than full software gradient Fill into stage when many transparent pixels.
// Still GPU* (not pure CPU). Returns ErrFallbackToCPU if staging impossible.
func (rc *GPURenderContext) fillBrushCoverageColorAt(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	if path == nil || path.NumVerbs() == 0 || paint == nil {
		return nil
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}

	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}
	if bw*bh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Coverage-only stage (solid white).
	pm := render.NewPixmap(tw, th)
	pm.Clear(render.Transparent)
	sr := render.NewSoftwareRenderer(tw, th)
	sr.SetAntiAlias(rc.antiAlias)
	cov := paint.Clone()
	cov.BlendMode = render.BlendNormal
	cov.SetBrush(render.SolidBrush{Color: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
	if err := sr.Fill(pm, path, cov); err != nil {
		return render.ErrFallbackToCPU
	}
	pm.NotifyPixelsChanged()

	src := pm.Data()
	stride := tw * 4
	pixelData := make([]byte, bw*bh*4)
	any := false
	for row := 0; row < bh; row++ {
		py := bounds.Min.Y + row
		for col := 0; col < bw; col++ {
			px := bounds.Min.X + col
			srcOff := py*stride + px*4
			aCov := src[srcOff+3]
			dstOff := (row*bw + col) * 4
			if aCov == 0 {
				// leave zero
				continue
			}
			any = true
			// ColorAt at pixel center; modulate by coverage alpha.
			c := paint.ColorAt(float64(px)+0.5, float64(py)+0.5)
			ca := float64(aCov) / 255.0
			a := c.A * ca
			pixelData[dstOff+0] = uint8(clamp255(c.R * a * 255))
			pixelData[dstOff+1] = uint8(clamp255(c.G * a * 255))
			pixelData[dstOff+2] = uint8(clamp255(c.B * a * 255))
			pixelData[dstOff+3] = uint8(clamp255(a * 255))
		}
	}
	if !any {
		return nil
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec

	// Distinct gen so image cache does not collide with full-Fill bootstrap frames.
	genID := pm.GenerationID() ^ 0xC0A7_C01A_0000_0001

	rc.QueueImageDraw(target, pixelData, genID, bw, bh, bw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// fillColorAtFieldMaskedGPU is the shared GPU* field×coverage path (N1/N2/N3):
//
//  1. Coverage mask at field resolution: prefer GPU stencil-then-cover
//     (MSAA solid white → alpha); software solid Analytic as fallback.
//  2. Sample ColorAt into an unmasked premul RGBA field (device-space centers).
//  3. Modulate field × mask via true GPU R8 shader (`maskR8Modulate`);
//     CPU premul only if GPU modulate fails.
//  4. QueueImageDraw blits the result (real GPUOps).
//
// GPU-FIRST: call only after native span/field/convex/rect-pattern fail.
// N1 (v2.6): coverage prefers GPU stencil-then-cover; field ColorAt + GPU R8
// modulate. Session Destroy detaches shared stencil BGLs. Native span/field/
// convex stay first. Textured cover (no CPU field) remains next.
// Cap field at 512² like fillColorAtFieldNative.
func (rc *GPURenderContext) fillColorAtFieldMaskedGPU(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	colorAt func(x, y float64) render.RGBA,
	genSeed uint64,
) error {
	if path == nil || path.NumVerbs() == 0 || paint == nil || colorAt == nil {
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
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}

	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}

	const maxSide = 512
	nw, nh := bw, bh
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Map path into field pixel space.
	sx := float64(nw) / float64(bw)
	sy := float64(nh) / float64(bh)
	toField := render.Scale(sx, sy).Multiply(
		render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)),
	)
	localPath := path.Transform(toField)

	// Coverage: GPU stencil-then-cover first (AA on); software solid as fallback.
	mask, covGPU, err := rc.rasterCoverageMask(localPath, nw, nh, paint.FillRule)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if mask == nil {
		return nil
	}

	field := make([]byte, nw*nh*4)
	any := false
	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)
	fw := float64(bw)
	fh := float64(bh)
	for y := 0; y < nh; y++ {
		py := by + (float64(y)+0.5)/float64(nh)*fh
		row := y * nw * 4
		mrow := y * nw
		for x := 0; x < nw; x++ {
			aCov := mask[mrow+x]
			if aCov == 0 {
				continue
			}
			any = true
			px := bx + (float64(x)+0.5)/float64(nw)*fw
			c := colorAt(px, py)
			a := c.A
			off := row + x*4
			field[off+0] = uint8(clamp255(c.R * a * 255))
			field[off+1] = uint8(clamp255(c.G * a * 255))
			field[off+2] = uint8(clamp255(c.B * a * 255))
			field[off+3] = uint8(clamp255(a * 255))
		}
	}
	if !any {
		return nil
	}

	pixelData := field
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.maskR8
	rc.shared.mu.Unlock()
	usedGPUMod := false
	if device != nil && queue != nil {
		if out, err := maskR8Modulate(device, queue, cache, field, mask, nw, nh); err == nil && len(out) == len(field) {
			pixelData = out
			usedGPUMod = true
		}
	}
	if !usedGPUMod {
		pixelData = make([]byte, nw*nh*4)
		for y := 0; y < nh; y++ {
			row := y * nw * 4
			mrow := y * nw
			for x := 0; x < nw; x++ {
				aCov := mask[mrow+x]
				if aCov == 0 {
					continue
				}
				ca := float64(aCov) / 255.0
				off := row + x*4
				pixelData[off+0] = uint8(clamp255(float64(field[off+0]) * ca))
				pixelData[off+1] = uint8(clamp255(float64(field[off+1]) * ca))
				pixelData[off+2] = uint8(clamp255(float64(field[off+2]) * ca))
				pixelData[off+3] = uint8(clamp255(float64(field[off+3]) * ca))
			}
		}
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := genSeed ^ fieldGeomGenID(bx, by, fw, fh, nw, nh) ^ 0xF1E1DA5A50001
	if usedGPUMod {
		genID ^= 0x00000000A11C0001
	}
	if covGPU {
		genID ^= 0x0000000057EAC001 // stencil coverage namespace
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, pixelData, genID, nw, nh, nw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// rasterCoverageMask builds an R8 coverage plane for path at nw×nh.
// Prefer GPU stencil-then-cover when AA is on and StencilRenderer is ready;
// otherwise software solid-white fill. covGPU is true when stencil was used.
func (rc *GPURenderContext) rasterCoverageMask(
	localPath *render.Path,
	nw, nh int,
	fillRule render.FillRule,
) (mask []byte, covGPU bool, err error) {
	if localPath == nil || nw <= 0 || nh <= 0 {
		return nil, false, nil
	}

	// GPU stencil-then-cover coverage (AA on): same algorithm as solid non-convex
	// fills. Shared StencilRenderer must DetachExternalLayouts on session Destroy
	// so mask BGL is never dangling across Context.Close.
	if rc.antiAlias {
		rc.shared.mu.Lock()
		sr := rc.shared.stencilRenderer
		rc.shared.mu.Unlock()
		if sr != nil {
			data := make([]byte, nw*nh*4)
			tmp := render.GPURenderTarget{
				Data: data, Width: nw, Height: nh, Stride: nw * 4,
			}
			white := render.RGBA{R: 1, G: 1, B: 1, A: 1}
			if rerr := sr.RenderPath(tmp, localPath, white, fillRule); rerr == nil {
				mask = make([]byte, nw*nh)
				any := false
				for i := 0; i < nw*nh; i++ {
					a := data[i*4+3]
					mask[i] = a
					if a != 0 {
						any = true
					}
				}
				if any {
					return mask, true, nil
				}
				return nil, true, nil
			}
			// fall through to software on stencil error
		}
	}

	// Software solid coverage (FillRule + AA).
	pm := render.NewPixmap(nw, nh)
	pm.Clear(render.Transparent)
	srSoft := render.NewSoftwareRenderer(nw, nh)
	srSoft.SetAntiAlias(rc.antiAlias)
	cov := render.NewPaint()
	cov.BlendMode = render.BlendNormal
	cov.FillRule = fillRule
	cov.SetBrush(render.SolidBrush{Color: render.RGBA{R: 1, G: 1, B: 1, A: 1}})
	if ferr := srSoft.Fill(pm, localPath, cov); ferr != nil {
		return nil, false, ferr
	}
	pm.NotifyPixelsChanged()
	src := pm.Data()
	mask = make([]byte, nw*nh)
	any := false
	for y := 0; y < nh; y++ {
		row := y * nw * 4
		mrow := y * nw
		for x := 0; x < nw; x++ {
			a := src[row+x*4+3]
			mask[mrow+x] = a
			if a != 0 {
				any = true
			}
		}
	}
	if !any {
		return nil, false, nil
	}
	return mask, false, nil
}

// fillLinearGradientFieldMasked is N1 textured linear cover for non-convex/EvenOdd:
// coverage (GPU stencil preferred) × 1D ColorAt ramp sampled on GPU by projected t
// (O(n) ColorAt only; no O(pixels) CPU field expand), then GPU blit.
//
// GPU path: linearRampMaskExpand (ramp texture + R8 mask + projection uniforms).
// CPU expand + maskR8Modulate remains only if GPU expand fails.
// Native span/field/convex still run first and must not be demoted.
func (rc *GPURenderContext) fillLinearGradientFieldMasked(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	g *render.LinearGradientBrush,
) error {
	if g == nil || path == nil || paint == nil {
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
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}

	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}

	const maxSide = 512
	nw, nh := bw, bh
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	dx := g.End.X - g.Start.X
	dy := g.End.Y - g.Start.Y
	len2 := dx*dx + dy*dy
	if len2 < 1e-12 {
		// Degenerate → solid ColorAt at start.
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedLinear(g))
	}

	// Project AABB corners to gradient parameter t in [unbounded] (ColorAt extends).
	corners := [4][2]float64{
		{float64(bounds.Min.X), float64(bounds.Min.Y)},
		{float64(bounds.Max.X), float64(bounds.Min.Y)},
		{float64(bounds.Min.X), float64(bounds.Max.Y)},
		{float64(bounds.Max.X), float64(bounds.Max.Y)},
	}
	tMin, tMax := 0.0, 0.0
	for i, c := range corners {
		tx := ((c[0]-g.Start.X)*dx + (c[1]-g.Start.Y)*dy) / len2
		if i == 0 || tx < tMin {
			tMin = tx
		}
		if i == 0 || tx > tMax {
			tMax = tx
		}
	}
	// Pad slightly so edge samples stay inside.
	span := tMax - tMin
	if span < 1e-9 {
		span = 1e-9
		tMax = tMin + span
	}

	// Ramp density: ~1 sample per field diagonal pixel, clamped.
	diag := math.Hypot(float64(nw), float64(nh))
	n := int(diag + 0.5)
	if n < 64 {
		n = 64
	}
	if n > 2048 {
		n = 2048
	}

	// Premul RGBA ramp in parameter space [tMin, tMax] — O(n) ColorAt only.
	ramp := make([]byte, n*4)
	for i := 0; i < n; i++ {
		tt := tMin + (float64(i)+0.5)/float64(n)*span
		px := g.Start.X + tt*dx
		py := g.Start.Y + tt*dy
		c := g.ColorAt(px, py)
		a := c.A
		off := i * 4
		ramp[off+0] = uint8(clamp255(c.R * a * 255))
		ramp[off+1] = uint8(clamp255(c.G * a * 255))
		ramp[off+2] = uint8(clamp255(c.B * a * 255))
		ramp[off+3] = uint8(clamp255(a * 255))
	}

	// Coverage at field resolution (GPU stencil preferred).
	sx := float64(nw) / float64(bw)
	sy := float64(nh) / float64(bh)
	toField := render.Scale(sx, sy).Multiply(
		render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)),
	)
	localPath := path.Transform(toField)
	mask, covGPU, err := rc.rasterCoverageMask(localPath, nw, nh, paint.FillRule)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if mask == nil {
		return nil
	}

	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)
	fw := float64(bw)
	fh := float64(bh)
	invSpan := 1.0 / span
	invLen2 := 1.0 / len2

	// Prefer true GPU textured expand: sample 1D ramp by projected t × R8 mask.
	// No O(pixels) CPU field write.
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rampCache := &rc.shared.linearRampMask
	maskCache := &rc.shared.maskR8
	rc.shared.mu.Unlock()

	usedGPURamp := false
	var pixelData []byte
	if device != nil && queue != nil {
		params := linearRampMaskParams{
			boundsMinX: float32(bx),
			boundsMinY: float32(by),
			boundsW:    float32(fw),
			boundsH:    float32(fh),
			startX:     float32(g.Start.X),
			startY:     float32(g.Start.Y),
			dX:         float32(dx),
			dY:         float32(dy),
			invLen2:    float32(invLen2),
			tMin:       float32(tMin),
			invSpan:    float32(invSpan),
			mode:       0, // linear
		}
		if out, gerr := linearRampMaskExpand(device, queue, rampCache, ramp, n, mask, nw, nh, params); gerr == nil && len(out) == nw*nh*4 {
			pixelData = out
			usedGPURamp = true
		}
	}

	// Fallback: CPU expand 1D→2D then GPU R8 modulate (previous v2.7 path).
	if !usedGPURamp {
		field := make([]byte, nw*nh*4)
		any := false
		for y := 0; y < nh; y++ {
			py := by + (float64(y)+0.5)/float64(nh)*fh
			row := y * nw * 4
			mrow := y * nw
			for x := 0; x < nw; x++ {
				if mask[mrow+x] == 0 {
					continue
				}
				any = true
				px := bx + (float64(x)+0.5)/float64(nw)*fw
				tt := ((px-g.Start.X)*dx + (py-g.Start.Y)*dy) * invLen2
				u := (tt - tMin) * invSpan
				if u < 0 {
					u = 0
				}
				if u > 1 {
					u = 1
				}
				ri := int(u*float64(n-1) + 0.5)
				if ri < 0 {
					ri = 0
				}
				if ri >= n {
					ri = n - 1
				}
				roff := ri * 4
				off := row + x*4
				field[off+0] = ramp[roff+0]
				field[off+1] = ramp[roff+1]
				field[off+2] = ramp[roff+2]
				field[off+3] = ramp[roff+3]
			}
		}
		if !any {
			return nil
		}
		pixelData = field
		usedGPUMod := false
		if device != nil && queue != nil {
			if out, merr := maskR8Modulate(device, queue, maskCache, field, mask, nw, nh); merr == nil && len(out) == len(field) {
				pixelData = out
				usedGPUMod = true
			}
		}
		if !usedGPUMod {
			pixelData = make([]byte, nw*nh*4)
			for y := 0; y < nh; y++ {
				row := y * nw * 4
				mrow := y * nw
				for x := 0; x < nw; x++ {
					aCov := mask[mrow+x]
					if aCov == 0 {
						continue
					}
					ca := float64(aCov) / 255.0
					off := row + x*4
					pixelData[off+0] = uint8(clamp255(float64(field[off+0]) * ca))
					pixelData[off+1] = uint8(clamp255(float64(field[off+1]) * ca))
					pixelData[off+2] = uint8(clamp255(float64(field[off+2]) * ca))
					pixelData[off+3] = uint8(clamp255(float64(field[off+3]) * ca))
				}
			}
		}
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := brushFieldSeedLinear(g) ^ fieldGeomGenID(bx, by, fw, fh, nw, nh) ^ 0x11AEA8A5000001
	if usedGPURamp {
		genID ^= 0x00000000A1A1A001 // GPU 1D ramp × mask textured expand
	} else {
		// CPU expand fallback namespace (may also XOR A11C if modulate used — not tracked here).
		genID ^= 0x00000000C01D0001
	}
	if covGPU {
		genID ^= 0x0000000057EAC001
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, pixelData, genID, nw, nh, nw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// fillRadialGradientFieldMasked is N1 textured radial cover:
//   - simple (focus==center): t=(dist-startR)/radiusDiff
//   - focal (focus!=center): GPU computeTFocal (mode 3)
//
// O(n) ColorAt 1D ramp + GPU t×R8 mask. Native rect/convex still first.
func (rc *GPURenderContext) fillRadialGradientFieldMasked(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	g *render.RadialGradientBrush,
) error {
	if g == nil || path == nil || paint == nil {
		return render.ErrFallbackToCPU
	}
	focal := g.Focus.X != g.Center.X || g.Focus.Y != g.Center.Y
	radiusDiff := g.EndRadius - g.StartRadius
	if !focal && math.Abs(radiusDiff) < 1e-12 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedRadial(g))
	}
	if focal && g.EndRadius <= 1e-12 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedRadial(g))
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}
	const maxSide = 512
	nw, nh := bw, bh
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Parameter range over AABB.
	var tMin, tMax float64
	if !focal {
		minD, maxD := distRangeToAABB(g.Center.X, g.Center.Y, bounds)
		invRD := 1.0 / radiusDiff
		tMin = (minD - g.StartRadius) * invRD
		tMax = (maxD - g.StartRadius) * invRD
	} else {
		// Sample computeT via ColorAt-equivalent at corners + focus-in-bounds.
		corners := [4][2]float64{
			{float64(bounds.Min.X), float64(bounds.Min.Y)},
			{float64(bounds.Max.X), float64(bounds.Min.Y)},
			{float64(bounds.Min.X), float64(bounds.Max.Y)},
			{float64(bounds.Max.X), float64(bounds.Max.Y)},
		}
		for i, c := range corners {
			tt := radialFocalT(g, c[0], c[1])
			if i == 0 || tt < tMin {
				tMin = tt
			}
			if i == 0 || tt > tMax {
				tMax = tt
			}
		}
		if g.Focus.X >= float64(bounds.Min.X) && g.Focus.X <= float64(bounds.Max.X) &&
			g.Focus.Y >= float64(bounds.Min.Y) && g.Focus.Y <= float64(bounds.Max.Y) {
			if 0 < tMin {
				tMin = 0
			}
		}
		// Mid-edge samples reduce under-range for large AABBs.
		mids := [4][2]float64{
			{float64(bounds.Min.X+bounds.Max.X) * 0.5, float64(bounds.Min.Y)},
			{float64(bounds.Min.X+bounds.Max.X) * 0.5, float64(bounds.Max.Y)},
			{float64(bounds.Min.X), float64(bounds.Min.Y+bounds.Max.Y) * 0.5},
			{float64(bounds.Max.X), float64(bounds.Min.Y+bounds.Max.Y) * 0.5},
		}
		for _, c := range mids {
			tt := radialFocalT(g, c[0], c[1])
			if tt < tMin {
				tMin = tt
			}
			if tt > tMax {
				tMax = tt
			}
		}
	}
	if tMax < tMin {
		tMin, tMax = tMax, tMin
	}
	span := tMax - tMin
	if span < 1e-9 {
		span = 1e-9
		tMax = tMin + span
	}
	diag := math.Hypot(float64(nw), float64(nh))
	n := int(diag + 0.5)
	if n < 64 {
		n = 64
	}
	if n > 2048 {
		n = 2048
	}

	// Premul ramp via ColorAt at synthetic positions with known-ish t (Extend baked in).
	ramp := make([]byte, n*4)
	if !focal {
		for i := 0; i < n; i++ {
			tt := tMin + (float64(i)+0.5)/float64(n)*span
			dist := g.StartRadius + tt*radiusDiff
			c := g.ColorAt(g.Center.X+dist, g.Center.Y)
			writePremulRGBA(ramp, i*4, c)
		}
	} else {
		// Ray from focus toward center (or +X); place at d = tt * interDist.
		ux := g.Center.X - g.Focus.X
		uy := g.Center.Y - g.Focus.Y
		ulen := math.Hypot(ux, uy)
		if ulen < 1e-12 {
			ux, uy, ulen = 1, 0, 1
		}
		ux /= ulen
		uy /= ulen
		inter := focalRayIntersectDist(g, ux, uy)
		if inter < 1e-12 {
			inter = g.EndRadius
			if inter < 1e-12 {
				inter = 1
			}
		}
		for i := 0; i < n; i++ {
			tt := tMin + (float64(i)+0.5)/float64(n)*span
			d := tt * inter
			c := g.ColorAt(g.Focus.X+ux*d, g.Focus.Y+uy*d)
			writePremulRGBA(ramp, i*4, c)
		}
	}

	sx := float64(nw) / float64(bw)
	sy := float64(nh) / float64(bh)
	toField := render.Scale(sx, sy).Multiply(
		render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)),
	)
	localPath := path.Transform(toField)
	mask, covGPU, err := rc.rasterCoverageMask(localPath, nw, nh, paint.FillRule)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if mask == nil {
		return nil
	}

	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)
	fw := float64(bw)
	fh := float64(bh)
	invSpan := 1.0 / span

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rampCache := &rc.shared.linearRampMask
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedRadial(g))
	}

	var params linearRampMaskParams
	if !focal {
		params = linearRampMaskParams{
			boundsMinX: float32(bx),
			boundsMinY: float32(by),
			boundsW:    float32(fw),
			boundsH:    float32(fh),
			startX:     float32(g.Center.X),
			startY:     float32(g.Center.Y),
			dX:         float32(g.StartRadius),
			dY:         float32(1.0 / radiusDiff),
			invLen2:    0,
			tMin:       float32(tMin),
			invSpan:    float32(invSpan),
			mode:       1,
		}
	} else {
		params = linearRampMaskParams{
			boundsMinX: float32(bx),
			boundsMinY: float32(by),
			boundsW:    float32(fw),
			boundsH:    float32(fh),
			startX:     float32(g.Focus.X),
			startY:     float32(g.Focus.Y),
			dX:         float32(g.Center.X),
			dY:         float32(g.Center.Y),
			invLen2:    float32(g.EndRadius),
			tMin:       float32(tMin),
			invSpan:    float32(invSpan),
			mode:       3, // focal
		}
	}
	out, gerr := linearRampMaskExpand(device, queue, rampCache, ramp, n, mask, nw, nh, params)
	if gerr != nil || len(out) != nw*nh*4 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedRadial(g))
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := brushFieldSeedRadial(g) ^ fieldGeomGenID(bx, by, fw, fh, nw, nh) ^ 0x11AEA8A5AAD0001
	if focal {
		genID ^= 0x00000000F0CA1001
	}
	if covGPU {
		genID ^= 0x0000000057EAC001
	}
	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, out, genID, nw, nh, nw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// writePremulRGBA stores premul RGBA8 at off.
func writePremulRGBA(dst []byte, off int, c render.RGBA) {
	a := c.A
	dst[off+0] = uint8(clamp255(c.R * a * 255))
	dst[off+1] = uint8(clamp255(c.G * a * 255))
	dst[off+2] = uint8(clamp255(c.B * a * 255))
	dst[off+3] = uint8(clamp255(a * 255))
}

// radialFocalT mirrors RadialGradientBrush.computeTFocal for range estimation.
func radialFocalT(g *render.RadialGradientBrush, x, y float64) float64 {
	dx := x - g.Focus.X
	dy := y - g.Focus.Y
	fx := g.Center.X - g.Focus.X
	fy := g.Center.Y - g.Focus.Y
	a := dx*dx + dy*dy
	if a == 0 {
		return 0
	}
	b := -2 * (dx*fx + dy*fy)
	c := fx*fx + fy*fy - g.EndRadius*g.EndRadius
	disc := b*b - 4*a*c
	if disc < 0 {
		return 1
	}
	sqrtD := math.Sqrt(disc)
	t1 := (-b - sqrtD) / (2 * a)
	t2 := (-b + sqrtD) / (2 * a)
	var t float64
	switch {
	case t1 > 0 && t2 > 0:
		t = math.Min(t1, t2)
	case t1 > 0:
		t = t1
	case t2 > 0:
		t = t2
	default:
		return 0
	}
	pointDist := math.Sqrt(a)
	intersectDist := t * pointDist
	if intersectDist == 0 {
		return 0
	}
	return pointDist / intersectDist
}

// focalRayIntersectDist is the distance from focus along unit (ux,uy) to the
// end-radius circle (positive root), matching computeTFocal geometry.
func focalRayIntersectDist(g *render.RadialGradientBrush, ux, uy float64) float64 {
	fx := g.Center.X - g.Focus.X
	fy := g.Center.Y - g.Focus.Y
	// Point = focus + 1*(ux,uy) ⇒ d=(ux,uy), a=1
	a := ux*ux + uy*uy
	if a < 1e-18 {
		return g.EndRadius
	}
	b := -2 * (ux*fx + uy*fy)
	c := fx*fx + fy*fy - g.EndRadius*g.EndRadius
	disc := b*b - 4*a*c
	if disc < 0 {
		return g.EndRadius
	}
	sqrtD := math.Sqrt(disc)
	t1 := (-b - sqrtD) / (2 * a)
	t2 := (-b + sqrtD) / (2 * a)
	switch {
	case t1 > 0 && t2 > 0:
		return math.Min(t1, t2) // a≈1 so t≈distance
	case t1 > 0:
		return t1
	case t2 > 0:
		return t2
	default:
		return g.EndRadius
	}
}

// fillSweepGradientFieldMasked is N1 textured sweep cover for positive sweep ranges:
// O(n) ColorAt angular ramp + GPU atan2 projection × R8 mask.
// Negative / zero sweep falls back to ColorAt field×R8. Native rect/convex first.
func (rc *GPURenderContext) fillSweepGradientFieldMasked(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	g *render.SweepGradientBrush,
) error {
	if g == nil || path == nil || paint == nil {
		return render.ErrFallbackToCPU
	}
	sweepRange := g.EndAngle - g.StartAngle
	if sweepRange <= 1e-12 {
		// Zero/negative: keep ColorAt field (shader assumes positive wrap).
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedSweep(g))
	}
	sweepStart := g.StartAngle
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}
	const maxSide = 512
	nw, nh := bw, bh
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Parameter range: if center inside AABB, full [0,1]; else corner angles.
	tMin, tMax := 0.0, 1.0
	cx, cy := g.Center.X, g.Center.Y
	inside := cx >= float64(bounds.Min.X) && cx <= float64(bounds.Max.X) &&
		cy >= float64(bounds.Min.Y) && cy <= float64(bounds.Max.Y)
	if !inside {
		corners := [4][2]float64{
			{float64(bounds.Min.X), float64(bounds.Min.Y)},
			{float64(bounds.Max.X), float64(bounds.Min.Y)},
			{float64(bounds.Min.X), float64(bounds.Max.Y)},
			{float64(bounds.Max.X), float64(bounds.Max.Y)},
		}
		invSR := 1.0 / sweepRange
		for i, c := range corners {
			ang := math.Atan2(c[1]-cy, c[0]-cx)
			rel := ang - sweepStart
			twoPi := 2 * math.Pi
			rel = rel - math.Floor(rel/twoPi)*twoPi
			tt := rel * invSR
			if i == 0 || tt < tMin {
				tMin = tt
			}
			if i == 0 || tt > tMax {
				tMax = tt
			}
		}
		// Angle arc may wrap; if span looks inverted/huge, cover full domain.
		if tMax-tMin < 1e-9 || tMax-tMin > 1.0 {
			tMin, tMax = 0, 1
		}
	}
	span := tMax - tMin
	if span < 1e-9 {
		span = 1e-9
		tMax = tMin + span
	}
	diag := math.Hypot(float64(nw), float64(nh))
	n := int(diag + 0.5)
	if n < 64 {
		n = 64
	}
	if n > 2048 {
		n = 2048
	}
	ramp := make([]byte, n*4)
	for i := 0; i < n; i++ {
		tt := tMin + (float64(i)+0.5)/float64(n)*span
		ang := sweepStart + tt*sweepRange
		c := g.ColorAt(cx+math.Cos(ang), cy+math.Sin(ang))
		a := c.A
		off := i * 4
		ramp[off+0] = uint8(clamp255(c.R * a * 255))
		ramp[off+1] = uint8(clamp255(c.G * a * 255))
		ramp[off+2] = uint8(clamp255(c.B * a * 255))
		ramp[off+3] = uint8(clamp255(a * 255))
	}

	sx := float64(nw) / float64(bw)
	sy := float64(nh) / float64(bh)
	toField := render.Scale(sx, sy).Multiply(
		render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)),
	)
	localPath := path.Transform(toField)
	mask, covGPU, err := rc.rasterCoverageMask(localPath, nw, nh, paint.FillRule)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if mask == nil {
		return nil
	}

	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)
	fw := float64(bw)
	fh := float64(bh)
	invSpan := 1.0 / span
	invSR := 1.0 / sweepRange

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rampCache := &rc.shared.linearRampMask
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedSweep(g))
	}
	params := linearRampMaskParams{
		boundsMinX: float32(bx),
		boundsMinY: float32(by),
		boundsW:    float32(fw),
		boundsH:    float32(fh),
		startX:     float32(cx),
		startY:     float32(cy),
		dX:         float32(sweepStart),
		dY:         float32(invSR),
		invLen2:    0,
		tMin:       float32(tMin),
		invSpan:    float32(invSpan),
		mode:       2, // sweep
	}
	out, gerr := linearRampMaskExpand(device, queue, rampCache, ramp, n, mask, nw, nh, params)
	if gerr != nil || len(out) != nw*nh*4 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, g.ColorAt, brushFieldSeedSweep(g))
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := brushFieldSeedSweep(g) ^ fieldGeomGenID(bx, by, fw, fh, nw, nh) ^ 0x11AEA8A55EE0001
	if covGPU {
		genID ^= 0x0000000057EAC001
	}
	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, out, genID, nw, nh, nw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// distRangeToAABB returns min/max Euclidean distance from (cx,cy) to points in rect.
func distRangeToAABB(cx, cy float64, r image.Rectangle) (minD, maxD float64) {
	// Closest point on AABB to center.
	closestX := cx
	if closestX < float64(r.Min.X) {
		closestX = float64(r.Min.X)
	} else if closestX > float64(r.Max.X) {
		closestX = float64(r.Max.X)
	}
	closestY := cy
	if closestY < float64(r.Min.Y) {
		closestY = float64(r.Min.Y)
	} else if closestY > float64(r.Max.Y) {
		closestY = float64(r.Max.Y)
	}
	if cx >= float64(r.Min.X) && cx <= float64(r.Max.X) &&
		cy >= float64(r.Min.Y) && cy <= float64(r.Max.Y) {
		minD = 0
	} else {
		minD = math.Hypot(cx-closestX, cy-closestY)
	}
	corners := [4][2]float64{
		{float64(r.Min.X), float64(r.Min.Y)},
		{float64(r.Max.X), float64(r.Min.Y)},
		{float64(r.Min.X), float64(r.Max.Y)},
		{float64(r.Max.X), float64(r.Max.Y)},
	}
	for i, c := range corners {
		d := math.Hypot(cx-c[0], cy-c[1])
		if i == 0 || d > maxD {
			maxD = d
		}
	}
	return minD, maxD
}

// fillGradientFieldMasked is N1: gradient ColorAt field × path coverage (GPU R8).
func (rc *GPURenderContext) fillGradientFieldMasked(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	colorAt func(x, y float64) render.RGBA,
	genSeed uint64,
) error {
	return rc.fillColorAtFieldMaskedGPU(target, path, paint, colorAt, genSeed)
}

// fillImagePatternFieldMasked is N2: non-rect ImagePattern fill via GPU texture
// sample (inverse affine UV) × R8 coverage. Used when AA-rect GPU tile path
// cannot run. Keeps fillImagePatternNative first for rects (no demotion).
// Falls back to ColorAt field×R8 only if GPU sample path fails.
func (rc *GPURenderContext) fillImagePatternFieldMasked(
	target render.GPURenderTarget,
	path *render.Path,
	paint *render.Paint,
	pat *render.ImagePattern,
) error {
	if pat == nil {
		return render.ErrFallbackToCPU
	}
	img, srcX, srcY, srcW, srcH, inv, opacity, clamp := pat.GPUPatternSource()
	if img == nil || srcW <= 0 || srcH <= 0 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, pat.ColorAt, patternFieldSeed(pat))
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}
	const maxSide = 512
	nw, nh := bw, bh
	if nw > maxSide {
		nw = maxSide
	}
	if nh > maxSide {
		nh = maxSide
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	if nw*nh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}
	// Cap source tile upload size (safety).
	if srcW*srcH > maxBrushFillPixels {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, pat.ColorAt, patternFieldSeed(pat))
	}

	fullW, fullH := img.Bounds()
	if srcX < 0 || srcY < 0 || srcX+srcW > fullW || srcY+srcH > fullH {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, pat.ColorAt, patternFieldSeed(pat))
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

	// Coverage at field resolution (GPU stencil preferred).
	sx := float64(nw) / float64(bw)
	sy := float64(nh) / float64(bh)
	toField := render.Scale(sx, sy).Multiply(
		render.Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)),
	)
	localPath := path.Transform(toField)
	mask, covGPU, err := rc.rasterCoverageMask(localPath, nw, nh, paint.FillRule)
	if err != nil {
		return render.ErrFallbackToCPU
	}
	if mask == nil {
		return nil
	}

	op := opacity
	if op <= 0 {
		op = 1
	}
	if op > 1 {
		op = 1
	}
	clampMode := float32(0)
	if clamp {
		clampMode = 1
	}
	bx := float64(bounds.Min.X)
	by := float64(bounds.Min.Y)
	fw := float64(bw)
	fh := float64(bh)

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.patternMaskSample
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, pat.ColorAt, patternFieldSeed(pat))
	}
	params := patternMaskSampleParams{
		boundsMinX: float32(bx),
		boundsMinY: float32(by),
		boundsW:    float32(fw),
		boundsH:    float32(fh),
		invA:       float32(inv.A),
		invB:       float32(inv.B),
		invC:       float32(inv.C),
		invD:       float32(inv.D),
		invE:       float32(inv.E),
		invF:       float32(inv.F),
		patW:       float32(srcW),
		patH:       float32(srcH),
		opacity:    float32(op),
		clampMode:  clampMode,
	}
	out, gerr := patternMaskSampleExpand(device, queue, cache, tile, srcW, srcH, mask, nw, nh, params)
	if gerr != nil || len(out) != nw*nh*4 {
		return rc.fillColorAtFieldMaskedGPU(target, path, paint, pat.ColorAt, patternFieldSeed(pat))
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if !target.View.IsNil() && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}
	genID := patternFieldSeed(pat) ^ fieldGeomGenID(bx, by, fw, fh, nw, nh) ^ 0x0A77E41100000001
	if covGPU {
		genID ^= 0x0000000057EAC001
	}
	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	rc.QueueImageDraw(target, out, genID, nw, nh, nw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// fillBrushAsImage rasterizes a non-solid paint (gradient/pattern) with the
// software path into a staging pixmap, then queues a GPU textured-quad
// composite. This matches Skia's "rasterize shader then GPU blit" bootstrap
// path and keeps correct AA/fill-rule while producing real GPUOps.
func (rc *GPURenderContext) fillBrushAsImage(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	if path == nil || path.NumVerbs() == 0 || paint == nil {
		return nil
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	// Non-SourceOver needs destination pixels → full CPU on context pixmap.
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}

	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		// Path may have subpixel bounds outside floor/ceil; expand slightly.
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 {
		return nil
	}
	if bw*bh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Stage full target-sized transparent pixmap so ColorAt coords match device
	// space (same as software path on the context pixmap).
	pm := render.NewPixmap(tw, th)
	pm.Clear(render.Transparent)
	sr := render.NewSoftwareRenderer(tw, th)
	sr.SetAntiAlias(rc.antiAlias)
	// Local paint copy: force SourceOver; keep brush/pattern.
	local := paint.Clone()
	local.BlendMode = render.BlendNormal
	if err := sr.Fill(pm, path, local); err != nil {
		return render.ErrFallbackToCPU
	}
	pm.NotifyPixelsChanged()

	// Extract sub-rect premul RGBA for upload.
	src := pm.Data()
	stride := tw * 4
	pixelData := make([]byte, bw*bh*4)
	for row := 0; row < bh; row++ {
		srcOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
		dstOff := row * bw * 4
		copy(pixelData[dstOff:dstOff+bw*4], src[srcOff:srcOff+bw*4])
	}

	// Skip fully transparent staging (nothing to draw).
	any := false
	for i := 3; i < len(pixelData); i += 4 {
		if pixelData[i] != 0 {
			any = true
			break
		}
	}
	if !any {
		return nil
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec

	rc.QueueImageDraw(target, pixelData, pm.GenerationID(), bw, bh, bw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}

// detectedShapeToPath builds a path for a detected shape (device space).
func detectedShapeToPath(shape render.DetectedShape) *render.Path {
	path := render.NewPath()
	switch shape.Kind {
	case render.ShapeCircle:
		path.Circle(shape.CenterX, shape.CenterY, shape.RadiusX)
	case render.ShapeEllipse:
		path.Ellipse(shape.CenterX, shape.CenterY, shape.RadiusX, shape.RadiusY)
	case render.ShapeRect:
		x := shape.CenterX - shape.Width/2
		y := shape.CenterY - shape.Height/2
		path.Rectangle(x, y, shape.Width, shape.Height)
	case render.ShapeRRect:
		x := shape.CenterX - shape.Width/2
		y := shape.CenterY - shape.Height/2
		path.RoundedRectangle(x, y, shape.Width, shape.Height, shape.CornerRadius)
	default:
		return nil
	}
	return path
}

// fillAdvancedBlendAsImage composites a solid (or brush) path using advanced
// blend modes (Multiply/Screen/Overlay) via true GPU dual-texture sampling:
//
//  1. Flush pending GPU so target.Data holds current destination pixels.
//  2. Software-rasterize the *source only* (transparent dest, SourceOver) for coverage/AA.
//  3. GPU dual-tex pass samples dest region + src coverage and evaluates blend on GPU.
//  4. QueueImageDraw uploads the GPU-composited premul result (SourceOver blit of final pixels).
//
// This is dual-texture fragment blend (not CPU composite of dest*src). AA edge
// pixels may not be bit-exact under the subsequent SO blit; gates sample interiors.
func (rc *GPURenderContext) fillAdvancedBlendAsImage(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	return rc.fillAdvancedBlendTiled(target, path, paint)
}

// fillMaskedAsImage applies an alpha mask via true R8 GPU sampling (L.06):
//
//  1. Software-rasterize the shape without mask (geometry + AA coverage only).
//  2. Build an R8 mask plane from MaskAware GPU plane (preferred) or MaskCoverage.
//  3. GPU fragment shader samples src RGBA + R8 mask and multiplies (premul).
//  4. QueueImageDraw blits the GPU-modulated result (real GPUOps on chain).
//
// Mask application is no longer baked only on the CPU; the R8 texture is
// sampled in a WGSL shader on the render→webgpu→rwgpu→native path. When the
// accelerator implements MaskAware, setupGPUMask uploads a full-surface R8
// texture and fillMaskedAsImage reuses that plane (native mask path).
func (rc *GPURenderContext) fillMaskedAsImage(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
	if path == nil || path.NumVerbs() == 0 || paint == nil || paint.MaskCoverage == nil {
		return nil
	}
	if !rc.shared.gpuReady {
		rc.shared.mu.Lock()
		err := rc.shared.ensureGPU()
		rc.shared.mu.Unlock()
		if err != nil || !rc.shared.gpuReady {
			return render.ErrFallbackToCPU
		}
	}
	// Non-SourceOver + mask: keep full CPU for now.
	if paint.BlendMode != render.BlendNormal {
		return render.ErrFallbackToCPU
	}

	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}

	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		x0 := int(math.Floor(bb.Min.X)) - 1
		y0 := int(math.Floor(bb.Min.Y)) - 1
		x1 := int(math.Ceil(bb.Max.X)) + 1
		y1 := int(math.Ceil(bb.Max.Y)) + 1
		bounds = image.Rect(x0, y0, x1, y1).Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	if bw <= 0 || bh <= 0 || bw*bh > maxBrushFillPixels {
		return render.ErrFallbackToCPU
	}

	// Source-only raster: geometry coverage without mask bake.
	pm := render.NewPixmap(tw, th)
	pm.Clear(render.Transparent)
	sr := render.NewSoftwareRenderer(tw, th)
	sr.SetAntiAlias(rc.antiAlias)
	local := paint.Clone()
	local.BlendMode = render.BlendNormal
	local.MaskCoverage = nil
	if err := sr.Fill(pm, path, local); err != nil {
		return render.ErrFallbackToCPU
	}
	pm.NotifyPixelsChanged()

	srcFull := pm.Data()
	stride := tw * 4
	srcRegion := make([]byte, bw*bh*4)
	maskRegion := make([]byte, bw*bh)

	// Prefer GPU-uploaded full-surface R8 plane (MaskAware / L.06) when size matches.
	// This avoids re-evaluating MaskCoverage and proves native mask texture path.
	var (
		maskPlane []byte
		mpW, mpH  int
		usePlane  bool
	)
	if plane, pw, ph, ok := rc.shared.MaskPlane(); ok && pw == tw && ph == th {
		maskPlane, mpW, mpH, usePlane = plane, pw, ph, true
		_ = mpW
		_ = mpH
	}
	maskFn := paint.MaskCoverage
	any := false
	for row := 0; row < bh; row++ {
		py := bounds.Min.Y + row
		srcOff := py*stride + bounds.Min.X*4
		dstOff := row * bw * 4
		copy(srcRegion[dstOff:dstOff+bw*4], srcFull[srcOff:srcOff+bw*4])
		for col := 0; col < bw; col++ {
			px := bounds.Min.X + col
			var m uint8
			if usePlane {
				m = maskPlane[py*tw+px]
			} else {
				m = maskFn(px, py)
			}
			maskRegion[row*bw+col] = m
			if srcRegion[dstOff+col*4+3] != 0 && m != 0 {
				any = true
			}
		}
	}
	if !any {
		return nil
	}

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.maskR8
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return render.ErrFallbackToCPU
	}

	pixelData, err := maskR8Modulate(device, queue, cache, srcRegion, maskRegion, bw, bh)
	if err != nil {
		// Fallback: CPU bake mask then blit (legacy bootstrap).
		pm2 := render.NewPixmap(tw, th)
		pm2.Clear(render.Transparent)
		sr2 := render.NewSoftwareRenderer(tw, th)
		sr2.SetAntiAlias(rc.antiAlias)
		local2 := paint.Clone()
		local2.BlendMode = render.BlendNormal
		if err2 := sr2.Fill(pm2, path, local2); err2 != nil {
			return render.ErrFallbackToCPU
		}
		pm2.NotifyPixelsChanged()
		pixelData = make([]byte, bw*bh*4)
		for row := 0; row < bh; row++ {
			srcOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
			dstOff := row * bw * 4
			copy(pixelData[dstOff:dstOff+bw*4], pm2.Data()[srcOff:srcOff+bw*4])
		}
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec

	rc.QueueImageDraw(target, pixelData, pm.GenerationID(), bw, bh, bw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}
