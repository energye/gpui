//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"math"

	intImage "github.com/energye/gpui/render/internal/image"
)

// DrawImageQuad draws the full source image into corners with explicit options.
// Returns a sentinel error for degenerate/self-intersecting/non-finite quads
// or unsupported blend/interp instead of silently drawing wrong.
func (c *Context) DrawImageQuad(img *ImageBuf, corners [4]Point, opts QuadDrawOptions) error {
	if c == nil || img == nil {
		return nil
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return nil
	}
	interp, err := normalizeQuadInterp(opts.Interpolation)
	if err != nil {
		return err
	}
	if opts.BlendMode != BlendNormal {
		return ErrQuadUnsupportedBlend
	}
	if opts.Perspective {
		for _, w := range opts.Weights {
			if math.IsNaN(w) || math.IsInf(w, 0) || w <= 0 {
				return ErrQuadBadWeights
			}
		}
		if interp == InterpBicubic {
			return ErrQuadPerspectiveBicubic
		}
	}
	opacity := normalizeQuadOpacity(opts.Opacity)
	if !quadTintFinite(opts.Tint) {
		return ErrQuadBadTint
	}
	if kind := ClassifyQuad(corners); kind != QuadOK {
		return quadKindErr(kind)
	}
	ctm := c.totalMatrix()
	dev := [4]Point{}
	for i := 0; i < 4; i++ {
		dev[i] = ctm.TransformPoint(corners[i])
	}
	if quadHasNonFinite(dev) || quadAreaSum(dev) < 1e-9 {
		return ErrQuadDegenerate
	}
	// Damage bounds (device space).
	minX, minY, maxX, maxY := quadBounds(dev)
	c.trackDamage(image.Rect(int(minX), int(minY), int(maxX)+1, int(maxY)+1))

	// GOGPU_RENDER_MODE=cpu 强制走 CPU 双三角，供 C 两边齐离屏对比取 CPU 真值。
	if !CPUOnlyMode() {
		if rc := c.gpuCtxOps(); rc != nil {
			pixelData := img.PremultipliedData()
			if len(pixelData) == 0 {
				if c.gpuPathAvailable() {
					c.recordCPUFallbackReason("image:DrawImageQuad")
				}
				c.drawImageQuadCPUSplit(img, dev, interp, opacity, opts.Perspective, opts.Weights, opts.Tint)
				return nil
			}
			defer c.setGPUClipRect()()
			target := c.gpuRenderTarget()
			vpW := uint32(target.Width)  //nolint:gosec
			vpH := uint32(target.Height) //nolint:gosec
			nearest := interp == InterpNearest
			bicubic := interp == InterpBicubic
			tr, tg, tb, ta := atlasTintFactors(opts.Tint)
			// 与图集腿同一式：顶点染色按 premul（tr*ta）下发，合成链才与 CPU 一致。
			ptR, ptG, ptB := tr*ta, tg*ta, tb*ta
			if opts.Perspective {
				rc.QueueImageDrawQuadW(target, pixelData, img.GenerationID(), img, opts.Mipmap, imgW, imgH, img.Stride(),
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(opts.Weights[0]), float32(opts.Weights[1]),
					float32(opts.Weights[2]), float32(opts.Weights[3]),
					float32(opacity), vpW, vpH,
					0, 0, 1, 1,
					float32(ptR), float32(ptG), float32(ptB), float32(ta),
					nearest)
				c.recordGPUOp()
				return nil
			}
			if atlasTintIsIdentity(opts.Tint) {
				rc.QueueImageDraw(target, pixelData, img.GenerationID(), img, opts.Mipmap, imgW, imgH, img.Stride(),
					float32(dev[0].X), float32(dev[0].Y),
					float32(dev[1].X), float32(dev[1].Y),
					float32(dev[2].X), float32(dev[2].Y),
					float32(dev[3].X), float32(dev[3].Y),
					float32(opacity), vpW, vpH,
					0, 0, 1, 1,
					nearest, bicubic)
				c.recordGPUOp()
				return nil
			}
			rc.QueueImageDrawTint(target, pixelData, img.GenerationID(), img, opts.Mipmap, imgW, imgH, img.Stride(),
				float32(dev[0].X), float32(dev[0].Y),
				float32(dev[1].X), float32(dev[1].Y),
				float32(dev[2].X), float32(dev[2].Y),
				float32(dev[3].X), float32(dev[3].Y),
				float32(opacity), vpW, vpH,
				0, 0, 1, 1,
				float32(ptR), float32(ptG), float32(ptB), float32(ta),
				nearest, bicubic)
			c.recordGPUOp()
			return nil
		}
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("image:DrawImageQuad")
	}
	c.drawImageQuadCPUSplit(img, dev, interp, opacity, opts.Perspective, opts.Weights, opts.Tint)
	return nil
}

// normalizeQuadInterp maps zero to Bilinear and rejects unknown modes.
// normalizeQuadInterp maps zero to Bilinear and rejects unknown modes.
func normalizeQuadInterp(m InterpolationMode) (InterpolationMode, error) {
	if m == 0 {
		m = InterpBilinear
	}
	if m != InterpNearest && m != InterpBilinear && m != InterpBicubic {
		return InterpBilinear, ErrQuadUnsupportedInterp
	}
	return m, nil
}

// normalizeQuadOpacity maps zero to opaque (DrawImageEx convention) and clamps 0..1.
// normalizeQuadOpacity maps zero to opaque (DrawImageEx convention) and clamps 0..1.
func normalizeQuadOpacity(o float64) float64 {
	if o == 0 {
		return 1
	}
	if o < 0 {
		return 0
	}
	if o > 1 {
		return 1
	}
	return o
}

// isFinitePt reports finite coordinates (NaN/Inf safe for ClassifyQuad).
// isFinitePt reports finite coordinates (NaN/Inf safe for ClassifyQuad).
func isFinitePt(p Point) bool {
	return !math.IsNaN(p.X) && !math.IsNaN(p.Y) &&
		!math.IsInf(p.X, 0) && !math.IsInf(p.Y, 0)
}

// drawImageQuadCPUSplit rasterizes device-space quad with GPU twin split.
// persp=true swaps both twin samplers for 1/w rational interpolation (P3.5
// batch 3 projective quads; weights must be finite and > 0).
func (c *Context) drawImageQuadCPUSplit(img *ImageBuf, dev [4]Point, interp InterpolationMode, opacity float64, persp bool, weights [4]float64, tint RGBA) {
	if c == nil || c.pixmap == nil || img == nil {
		return
	}
	opacity = normalizeQuadOpacity(opacity)
	if opacity <= 0 {
		return
	}
	// Keep z-order with pending GPU when drawing on base canvas.
	if c.layerStack == nil || len(c.layerStack.layers) == 0 {
		c.flushGPUAccelerator()
	}
	c.noteLayerCPUDraw()
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return
	}
	minXF, minYF, maxXF, maxYF := quadBounds(dev)
	x0 := int(math.Floor(minXF))
	y0 := int(math.Floor(minYF))
	x1 := int(math.Ceil(maxXF))
	y1 := int(math.Ceil(maxYF))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > pw {
		x1 = pw
	}
	if y1 > ph {
		y1 = ph
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	premulSrc := img.Format().IsPremultiplied()
	hasClip := c.clipStack != nil && c.clipStack.Depth() > 0
	mask := c.mask
	// 染色与图集 CPU 腿逐因子一致（straight tr/tg/tb × ta），GPU 侧下发的
	// 是同一因子的 premul 形式（tr*ta 存进顶点）。
	tr, tg, tb, ta := atlasTintFactors(tint)
	// GPU twin UVs: TL(0,0) TR(1,0) BR(1,1) BL(0,1), denominators hoisted per quad.
	var t1, t2 triSampler
	if persp {
		t1 = newTriSamplerPerspective(dev[0], dev[1], dev[3], [3][2]float64{{0, 0}, {1, 0}, {0, 1}},
			weights[0], weights[1], weights[3])
		t2 = newTriSamplerPerspective(dev[1], dev[2], dev[3], [3][2]float64{{1, 0}, {1, 1}, {0, 1}},
			weights[1], weights[2], weights[3])
	} else {
		t1 = newTriSampler(dev[0], dev[1], dev[3], [3][2]float64{{0, 0}, {1, 0}, {0, 1}})
		t2 = newTriSampler(dev[1], dev[2], dev[3], [3][2]float64{{1, 0}, {1, 1}, {0, 1}})
	}
	for y := y0; y < y1; y++ {
		py := float64(y) + 0.5
		for x := x0; x < x1; x++ {
			px := float64(x) + 0.5
			u, v, ok := t1.uv(px, py)
			if !ok {
				u, v, ok = t2.uv(px, py)
				if !ok {
					continue
				}
			}
			if u < 0 || u > 1 || v < 0 || v > 1 {
				continue
			}
			sr, sg, sb, sa := intImage.Sample(img, u, v, intImage.InterpolationMode(interp))
			if sa == 0 {
				continue
			}
			cover := 255
			if hasClip {
				cover = int(c.clipStack.Coverage(px, py))
				if cover == 0 {
					continue
				}
			}
			mcov := 255
			if mask != nil {
				mcov = int(mask.At(x, y))
				if mcov == 0 {
					continue
				}
			}
			k := opacity * float64(cover) / 255.0 * float64(mcov) / 255.0
			if k <= 0 {
				continue
			}
			var srcR, srcG, srcB, srcA float64
			if premulSrc {
				srcA = float64(sa) / 255.0 * ta * k
				srcR = float64(sr) / 255.0 * tr * ta * k
				srcG = float64(sg) / 255.0 * tg * ta * k
				srcB = float64(sb) / 255.0 * tb * ta * k
			} else {
				baseA := float64(sa) / 255.0
				srcA = baseA * ta * k
				srcR = float64(sr) / 255.0 * tr * srcA
				srcG = float64(sg) / 255.0 * tg * srcA
				srcB = float64(sb) / 255.0 * tb * srcA
			}
			if srcA <= 0 {
				continue
			}
			if srcA >= 1 {
				c.pixmap.SetPixelPremul(x, y, uint8(srcR*255+0.5), uint8(srcG*255+0.5), uint8(srcB*255+0.5), 255)
				continue
			}
			dstR, dstG, dstB, dstA := c.pixmap.getPremul(x, y)
			inv := 1 - srcA
			c.pixmap.setPremul(x, y,
				srcR+dstR*inv,
				srcG+dstG*inv,
				srcB+dstB*inv,
				srcA+dstA*inv,
			)
		}
	}
}

// triSampler holds one split triangle with hoisted barycentric factors.
// Same math as the per-pixel formula, denominators computed once per quad.
type triSampler struct {
	x2, y2 float64
	a0, b0 float64
	a1, b1 float64
	den    float64
	uvs    [3][2]float64
	degen  bool
	persp  bool
	invW   [3]float64 // per-corner 1/w when persp (projective quad, P3.5 batch 3)
}

// PushBackdropLayer creates a layer pre-filled with a snapshot of the current
// parent canvas (L.05 backdrop subset). Subsequent drawing/filters operate over
// that backdrop; PopLayer composites with the given blend/opacity.
func (c *Context) PushBackdropLayer(blendMode BlendMode, opacity float64) {
	if c == nil {
		return
	}
	// Parent device before the push. Inside a layer the parent content lives on
	// its GPU RT; its pixmap is an unseeded pooled buffer (stale), so the
	// snapshot must copy RT→RT — Skia saveLayer-with-backdrop reads the current
	// device pixels, and a stale mirror let the full-surface Pop blit cover the
	// enclosing layer's real content (backdrop window: card D bands vanished).
	parentView := c.gpuRenderTarget().View
	// Ensure GPU content is resolved into the current pixmap before snapshot.
	_ = c.FlushGPU()
	c.applyDitherIfEnabled()

	// Skia saveLayer bounds: the clip active at push limits where the filtered
	// backdrop may land (Flutter BackdropFilterLayer paints "everywhere in cull
	// rect"); outside it the parent content stays untouched.
	backdropBounds, backdropBounded := c.clipDeviceBounds()

	parent := c.pixmap
	c.pushLayerSurface(blendMode, c.BlendSpace(), opacity, false, false)
	if !parentView.IsNil() && !c.gpuRenderTarget().View.IsNil() {
		// RT→RT snapshot. Skipping the pixmap seed leaves the new RT on its
		// first-pass clear, so this full-surface copy fully defines the layer.
		savedMat := c.matrix
		c.matrix = Identity()
		c.copyGPUTextureToTarget(parentView, 0, 0, c.width, c.height)
		c.matrix = savedMat
	} else {
		if parent != nil && c.pixmap != nil {
			dst := c.pixmap.Data()
			src := parent.Data()
			n := len(dst)
			if len(src) < n {
				n = len(src)
			}
			copy(dst[:n], src[:n])
			c.pixmap.NotifyPixelsChanged()
		}
		// R1 L.05: seed layer GPU RT from snapshot so subsequent GPU draws composite
		// over real backdrop content (not an empty RT).
		seedOK := c.seedTopLayerGPUFromPixmap()
		if !seedOK {
			// If seed fails, mark CPU drew so Pop uses pixmap composite path.
			c.noteLayerCPUDraw()
		}
	}
	// Backdrop is a full-surface snapshot; Pop must blend the whole layer and
	// ignore the active clip (the parent queue was consumed mid-frame).
	c.markLayerFullComposite()
	c.markLayerBackdrop(backdropBounds, backdropBounded)
}
