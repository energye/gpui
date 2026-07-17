//go:build !nogpu

package gpu

import (
	"fmt"
	"image"
	"math"
	"unsafe"

	gpucontext "github.com/energye/gpui/gpu/context"
	"github.com/energye/gpui/render"
)

// fillAdvancedBlendTiled composites advanced blend modes via dual-tex GPU,
// processing large regions in tiles (P0-3 / G.06 / G.07).
func (rc *GPURenderContext) fillAdvancedBlendTiled(target render.GPURenderTarget, path *render.Path, paint *render.Paint) error {
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
	if tw <= 0 || th <= 0 {
		return render.ErrFallbackToCPU
	}
	hasData := len(target.Data) >= tw*th*4
	hasView := !target.View.IsNil()
	if !hasData && !hasView {
		return render.ErrFallbackToCPU
	}

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

	// Source-only raster into TIGHT bounds (not full surface).
	srcPM := render.NewPixmap(bw, bh)
	srcPM.Clear(render.Transparent)
	sr := render.NewSoftwareRenderer(bw, bh)
	sr.SetAntiAlias(rc.antiAlias)
	srcPaint := paint.Clone()
	srcPaint.BlendMode = render.BlendNormal
	srcPath := path.Clone().Transform(render.Translate(float64(-bounds.Min.X), float64(-bounds.Min.Y)))
	if err := sr.Fill(srcPM, srcPath, srcPaint); err != nil {
		return render.ErrFallbackToCPU
	}
	srcPM.NotifyPixelsChanged()

	// GPU layer/present targets: sample dest from View into Data for dual-tex.
	if hasView {
		if err := rc.syncViewRegionToData(target, bounds); err != nil {
			if !hasData {
				return render.ErrFallbackToCPU
			}
		} else {
			hasData = len(target.Data) >= tw*th*4
		}
	}
	if !hasData {
		return render.ErrFallbackToCPU
	}

	dstFull := target.Data
	srcFull := srcPM.Data()
	dstStride := tw * 4
	srcStride := bw * 4

	tileSide := int(math.Sqrt(float64(dualTexTileMax)))
	if tileSide < 64 {
		tileSide = 64
	}

	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	cache := &rc.shared.dualTexBlend
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return render.ErrFallbackToCPU
	}

	vpW := uint32(tw) //nolint:gosec
	vpH := uint32(th) //nolint:gosec
	if hasView && target.ViewWidth > 0 && target.ViewHeight > 0 {
		vpW, vpH = target.ViewWidth, target.ViewHeight
	}

	anyTile := false
	genBase := srcPM.GenerationID()
	tileIdx := uint64(0)
	for y0 := bounds.Min.Y; y0 < bounds.Max.Y; y0 += tileSide {
		for x0 := bounds.Min.X; x0 < bounds.Max.X; x0 += tileSide {
			x1 := x0 + tileSide
			if x1 > bounds.Max.X {
				x1 = bounds.Max.X
			}
			y1 := y0 + tileSide
			if y1 > bounds.Max.Y {
				y1 = bounds.Max.Y
			}
			tb := image.Rect(x0, y0, x1, y1)
			tbw, tbh := tb.Dx(), tb.Dy()
			if tbw <= 0 || tbh <= 0 {
				continue
			}
			dstRegion := make([]byte, tbw*tbh*4)
			srcRegion := make([]byte, tbw*tbh*4)
			relX := tb.Min.X - bounds.Min.X
			relY := tb.Min.Y - bounds.Min.Y
			for row := 0; row < tbh; row++ {
				dstOff := row * tbw * 4
				dstRow := (tb.Min.Y+row)*dstStride + tb.Min.X*4
				srcRow := (relY+row)*srcStride + relX*4
				copy(dstRegion[dstOff:dstOff+tbw*4], dstFull[dstRow:dstRow+tbw*4])
				copy(srcRegion[dstOff:dstOff+tbw*4], srcFull[srcRow:srcRow+tbw*4])
			}
			any := false
			for i := 3; i < len(srcRegion); i += 4 {
				if srcRegion[i] != 0 {
					any = true
					break
				}
			}
			if !any {
				continue
			}

			outTex, outView, err := dualTexAdvancedBlendNoReadback(device, queue, cache, dstRegion, srcRegion, tbw, tbh, paint.BlendMode)
			if err != nil {
				pixelData, err2 := dualTexAdvancedBlend(device, queue, cache, dstRegion, srcRegion, tbw, tbh, paint.BlendMode)
				if err2 != nil {
					return render.ErrFallbackToCPU
				}
				for row := 0; row < tbh; row++ {
					srcOff := row * tbw * 4
					dstOff := (tb.Min.Y+row)*dstStride + tb.Min.X*4
					copy(dstFull[dstOff:dstOff+tbw*4], pixelData[srcOff:srcOff+tbw*4])
				}
				fx0 := float32(tb.Min.X)
				fy0 := float32(tb.Min.Y)
				fx1 := float32(tb.Max.X)
				fy1 := float32(tb.Max.Y)
				rc.QueueImageDraw(target, pixelData, genBase+tileIdx, tbw, tbh, tbw*4,
					fx0, fy0, fx1, fy0, fx1, fy1, fx0, fy1,
					1.0, vpW, vpH, 0, 0, 1, 1, false, false)
			} else {
				rc.retainBrushCoverResult(outTex, outView)
				rc.QueueGPUTextureDraw(target, gpucontext.NewTextureView(unsafe.Pointer(outView)), //nolint:gosec
					float32(tb.Min.X), float32(tb.Min.Y), float32(tbw), float32(tbh), 1.0, vpW, vpH)
			}
			tileIdx++
			anyTile = true
		}
	}
	if anyTile {
		rc.sceneStats.PathCount++
		rc.sceneStats.ShapeCount++
	}
	return nil
}

// syncViewRegionToData copies a GPU texture view region into target.Data (RGBA premul).
func (rc *GPURenderContext) syncViewRegionToData(target render.GPURenderTarget, bounds image.Rectangle) error {
	if target.View.IsNil() || bounds.Empty() {
		return nil
	}
	tw, th := target.Width, target.Height
	if tw <= 0 || th <= 0 || len(target.Data) < tw*th*4 {
		return fmt.Errorf("syncViewRegionToData: bad target")
	}
	rc.shared.mu.Lock()
	device := rc.shared.device
	queue := rc.shared.queue
	rc.shared.mu.Unlock()
	if device == nil || queue == nil {
		return fmt.Errorf("syncViewRegionToData: no device")
	}
	rgba, err := readTextureViewRegionRGBA(device, queue, target.View, bounds, tw, th)
	if err != nil {
		return err
	}
	bw, bh := bounds.Dx(), bounds.Dy()
	stride := tw * 4
	for row := 0; row < bh; row++ {
		srcOff := row * bw * 4
		dstOff := (bounds.Min.Y+row)*stride + bounds.Min.X*4
		copy(target.Data[dstOff:dstOff+bw*4], rgba[srcOff:srcOff+bw*4])
	}
	return nil
}

// CompositeAdvancedLayer dual-tex composites a GPU layer RT onto parent pixels.
// Present path defers work to Flush via QueueAdvancedLayerComposite so the
// surface View exists as dest (draw-time has no present View yet).
func (rc *GPURenderContext) CompositeAdvancedLayer(
	parentData []byte, parentW, parentH int,
	srcView gpucontext.TextureView, srcW, srcH int,
	damage image.Rectangle,
	mode render.BlendMode, opacity float64,
) error {
	if rc == nil || srcView.IsNil() || parentW <= 0 || parentH <= 0 || srcW <= 0 || srcH <= 0 {
		return render.ErrFallbackToCPU
	}
	full := image.Rect(0, 0, parentW, parentH).Intersect(image.Rect(0, 0, srcW, srcH))
	bounds := damage
	if bounds.Empty() {
		bounds = full
	} else {
		bounds = bounds.Intersect(full)
	}
	if bounds.Empty() {
		return nil
	}
	// release is attached by queueLayerAdvancedGPU in context_layer.go
	rc.QueueAdvancedLayerComposite(srcView, srcW, srcH, bounds, mode, opacity, nil)
	return nil
}
