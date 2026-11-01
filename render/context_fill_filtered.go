//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
//----------------------------------------

package render

import (
	"image"
	"math"
)

// filteredFillDamageBounds expands path bounds by the mask-filter blur pad
// for damage tracking (the feather halo extends beyond the geometry; filter
// output bounds refinement for the ImageFilter families is a registered
// follow-up).
func filteredFillDamageBounds(pathBounds image.Rectangle, maskBlur float64) image.Rectangle {
	if maskBlur <= 0 {
		return pathBounds
	}
	pad := int(math.Ceil(3 * maskBlur))
	return image.Rect(pathBounds.Min.X-pad, pathBounds.Min.Y-pad, pathBounds.Max.X+pad, pathBounds.Max.Y+pad)
}

// fillPaintFilteredCPU is the CPU twin of internal/gpu's fillPaintFilteredAsImage
// (P3.4 paint-level filters): content raster with the filters stripped →
// MaskFilter blur (premul content, blur keeps the storage invariant) →
// ImageFilter nodes (CPU ops) → mask×clip modulation → premul SourceOver
// paste-back into the pixmap. It runs on the doFill CPU fallback (CPU-only
// mode, no accelerator, or GPU route refusal) so filtered paints feather
// identically on both tiers.
//
// Paste-back is SourceOver premul — the GPU filtered fill rides the same
// semantics (textured-quad draw); combining advanced blend modes with
// paint-level filters is a registered follow-up on both tiers.
func (c *Context) fillPaintFilteredCPU(path *Path) error {
	paint := c.paint
	hasMaskBlur := paint.MaskFilterRadius > 0
	hasImageFilter := len(paint.ImageFilter) > 0
	if !hasMaskBlur && !hasImageFilter {
		return c.renderer.Fill(c.pixmap, path, paint)
	}
	tw, th := c.pixmap.Width(), c.pixmap.Height()
	bounds := path.Bounds().Intersect(image.Rect(0, 0, tw, th))
	if bounds.Empty() {
		bb := path.BoundingBox()
		bounds = image.Rect(int(bb.Min.X)-1, int(bb.Min.Y)-1, int(bb.Max.X)+1, int(bb.Max.Y)+1).
			Intersect(image.Rect(0, 0, tw, th))
	}
	// Skia mask-filter semantics: the blurred coverage extends beyond the
	// geometry — expand the staging bounds by the blur pad so the outward
	// halo is not clipped at the shape bounds (mirrors the GPU filtered fill).
	if paint.MaskFilterRadius > 0 {
		pad := int(math.Ceil(3 * paint.MaskFilterRadius))
		bounds = image.Rect(bounds.Min.X-pad, bounds.Min.Y-pad, bounds.Max.X+pad, bounds.Max.Y+pad).
			Intersect(image.Rect(0, 0, tw, th))
	}
	if bounds.Empty() {
		return nil
	}
	bw, bh := bounds.Dx(), bounds.Dy()

	localPath := path
	if bounds.Min.X != 0 || bounds.Min.Y != 0 {
		localPath = path.Transform(Translate(-float64(bounds.Min.X), -float64(bounds.Min.Y)))
	}
	pm := NewPixmap(bw, bh)
	sr := NewSoftwareRenderer(bw, bh)
	sr.SetAntiAlias(c.antiAlias)
	local := paint.Clone()
	local.ImageFilter = nil
	local.MaskFilterRadius = 0
	local.MaskCoverage = nil
	local.ClipCoverage = nil
	if err := sr.Fill(pm, localPath, local); err != nil {
		return err
	}
	pm.NotifyPixelsChanged()

	// MaskFilter: soften the content edges through the registered CPU blur
	// (the same closure the ambient mask-chain A8 preprocessing uses).
	if hasMaskBlur {
		if blurXYApply == nil {
			return c.renderer.Fill(c.pixmap, path, paint)
		}
		blurred := filterPixmapPool.GetForOverwrite(bw, bh)
		blurXYApply(pm, blurred, paint.MaskFilterRadius, paint.MaskFilterRadius)
		filterPixmapPool.Put(pm)
		pm = blurred
	}

	// ImageFilter nodes: sequential CPU ops (the same runner ApplyMerge's
	// CPU fallback uses); each node reads the previous output and the
	// previous buffer returns to the pool.
	if hasImageFilter {
		work := pm
		for i := range paint.ImageFilter {
			n := paint.ImageFilter[i]
			if !imageFilterNodeRunnable(n) {
				continue
			}
			next := filterPixmapPool.GetForOverwrite(bw, bh)
			copy(next.Data(), work.Data())
			applyImageFilterNode(n, work, next)
			filterPixmapPool.Put(work)
			work = next
		}
		pm = work
	}

	// mask × clip modulation at target coordinates (mirrors the GPU filtered
	// fill's MaskPlane/MaskCoverage sampling and the software renderer's
	// clip×mask multiplicative coverage).
	content := pm.Data()
	maskRegion := make([]byte, bw*bh)
	any := false
	for row := 0; row < bh; row++ {
		py := bounds.Min.Y + row
		for col := 0; col < bw; col++ {
			px := bounds.Min.X + col
			var m uint8 = 255
			if maskFn := paint.MaskCoverage; maskFn != nil {
				m = maskFn(px, py)
			}
			if clipFn := paint.ClipCoverage; clipFn != nil && m != 0 {
				m = uint8(uint32(m) * uint32(clipFn(float64(px), float64(py))) / 255)
			}
			maskRegion[row*bw+col] = m
			if content[(row*bw+col)*4+3] != 0 && m != 0 {
				any = true
			}
		}
	}
	if any {
		for i, m := range maskRegion {
			if m == 255 {
				continue
			}
			px := content[i*4 : i*4+4]
			px[0] = uint8(uint32(px[0]) * uint32(m) / 255)
			px[1] = uint8(uint32(px[1]) * uint32(m) / 255)
			px[2] = uint8(uint32(px[2]) * uint32(m) / 255)
			px[3] = uint8(uint32(px[3]) * uint32(m) / 255)
		}

		// Paste back: premul SourceOver into the pixmap.
		dst := c.pixmap.Data()
		for row := 0; row < bh; row++ {
			drow := (bounds.Min.Y+row)*tw + bounds.Min.X
			for col := 0; col < bw; col++ {
				sp := content[(row*bw+col)*4 : (row*bw+col)*4+4]
				sa := uint32(sp[3])
				if sa == 0 {
					continue
				}
				dp := dst[(drow+col)*4 : (drow+col)*4+4]
				inv := 255 - sa
				dp[0] = uint8(uint32(sp[0]) + uint32(dp[0])*inv/255)
				dp[1] = uint8(uint32(sp[1]) + uint32(dp[1])*inv/255)
				dp[2] = uint8(uint32(sp[2]) + uint32(dp[2])*inv/255)
				dp[3] = uint8(sa + uint32(dp[3])*inv/255)
			}
		}
		c.pixmap.NotifyPixelsChanged()
	}
	filterPixmapPool.Put(pm)
	return nil
}
