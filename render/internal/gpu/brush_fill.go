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
// blend modes (Multiply/Screen/Overlay) against the current destination in
// target.Data, then uploads the result as a GPU textured quad (real GPUOps).
//
// Flow (Skia-style "resolve dest, blend, upload"):
//  1. Flush pending GPU so target.Data holds current destination pixels.
//  2. Software-fill onto a working pixmap copy with the advanced blend mode.
//  3. Upload the path bounds region and QueueImageDraw (SourceOver blit).
//
// AA edge pixels may not be bit-exact under the subsequent readback SO pass;
// gates sample opaque interiors.
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

	// Working copy of full destination for correct advanced blend sampling.
	work := render.NewPixmap(tw, th)
	copy(work.Data(), target.Data[:tw*th*4])
	sr := render.NewSoftwareRenderer(tw, th)
	sr.SetAntiAlias(rc.antiAlias)
	if err := sr.Fill(work, path, paint); err != nil {
		return render.ErrFallbackToCPU
	}
	work.NotifyPixelsChanged()

	src := work.Data()
	stride := tw * 4
	pixelData := make([]byte, bw*bh*4)
	for row := 0; row < bh; row++ {
		srcOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
		dstOff := row * bw * 4
		copy(pixelData[dstOff:dstOff+bw*4], src[srcOff:srcOff+bw*4])
	}

	// Skip fully transparent upload.
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

	rc.QueueImageDraw(target, pixelData, work.GenerationID(), bw, bh, bw*4,
		x0, y0, x1, y0, x1, y1, x0, y1,
		1.0, vpW, vpH,
		0, 0, 1, 1,
		false,
	)
	rc.sceneStats.PathCount++
	rc.sceneStats.ShapeCount++
	return nil
}
