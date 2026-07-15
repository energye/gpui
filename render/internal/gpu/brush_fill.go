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
	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 || len(target.Data) < tw*th*4 {
		return render.ErrFallbackToCPU
	}

	// Ensure destination is current in target.Data.
	if rc.PendingCount() > 0 {
		if err := rc.Flush(target); err != nil {
			return err
		}
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

	// Source-only raster: transparent dest + SourceOver keeps advanced blend for the GPU pass.
	srcPM := render.NewPixmap(tw, th)
	srcPM.Clear(render.Transparent)
	sr := render.NewSoftwareRenderer(tw, th)
	sr.SetAntiAlias(rc.antiAlias)
	srcPaint := paint.Clone()
	srcPaint.BlendMode = render.BlendNormal
	if err := sr.Fill(srcPM, path, srcPaint); err != nil {
		return render.ErrFallbackToCPU
	}
	srcPM.NotifyPixelsChanged()

	// Extract dest + src tight regions.
	dstFull := target.Data
	srcFull := srcPM.Data()
	stride := tw * 4
	dstRegion := make([]byte, bw*bh*4)
	srcRegion := make([]byte, bw*bh*4)
	for row := 0; row < bh; row++ {
		srcOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
		dstOff := row * bw * 4
		copy(dstRegion[dstOff:dstOff+bw*4], dstFull[srcOff:srcOff+bw*4])
		copy(srcRegion[dstOff:dstOff+bw*4], srcFull[srcOff:srcOff+bw*4])
	}

	// Skip fully transparent source.
	any := false
	for i := 3; i < len(srcRegion); i += 4 {
		if srcRegion[i] != 0 {
			any = true
			break
		}
	}
	if !any {
		return nil
	}

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.dualTexBlend
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return render.ErrFallbackToCPU
	}

	pixelData, err := dualTexAdvancedBlend(device, queue, cache, dstRegion, srcRegion, bw, bh, paint.BlendMode)
	if err != nil {
		// Fall back to legacy CPU composite path on dual-tex failure.
		work := render.NewPixmap(tw, th)
		copy(work.Data(), target.Data[:tw*th*4])
		sr2 := render.NewSoftwareRenderer(tw, th)
		sr2.SetAntiAlias(rc.antiAlias)
		if err2 := sr2.Fill(work, path, paint); err2 != nil {
			return render.ErrFallbackToCPU
		}
		work.NotifyPixelsChanged()
		pixelData = make([]byte, bw*bh*4)
		for row := 0; row < bh; row++ {
			srcOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
			dstOff := row * bw * 4
			copy(pixelData[dstOff:dstOff+bw*4], work.Data()[srcOff:srcOff+bw*4])
		}
	}

	x0 := float32(bounds.Min.X)
	y0 := float32(bounds.Min.Y)
	x1 := float32(bounds.Max.X)
	y1 := float32(bounds.Max.Y)
	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec

	rc.QueueImageDraw(target, pixelData, srcPM.GenerationID(), bw, bh, bw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
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
