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

// DrawImageQuad draws an image into a free-form destination quad (T.04 non-affine subset).
// corners are user-space points in order: top-left, top-right, bottom-right, bottom-left.
// GPU path uses QueueImageDraw with arbitrary corner mapping (perspective-like trapezoids).
// CPU path uses the same TL-TR-BL + TR-BR-BL split for pixel parity.
// Compat wrapper: defaults Bilinear + opaque + Normal, degenerate quads no-op.
func (c *Context) DrawImageQuad(img *ImageBuf, corners [4]Point) {
	_ = c.DrawImageQuadEx(img, corners, QuadDrawOptions{
		Interpolation: InterpBilinear, Opacity: 1, BlendMode: BlendNormal,
	})
}

// DrawImageQuadEx draws the full source image into corners with explicit options.
// Returns a sentinel error for degenerate/self-intersecting/non-finite quads
// or unsupported blend/interp instead of silently drawing wrong.
// DrawImageQuadEx draws the full source image into corners with explicit options.
// Returns a sentinel error for degenerate/self-intersecting/non-finite quads
// or unsupported blend/interp instead of silently drawing wrong.
func (c *Context) DrawImageQuadEx(img *ImageBuf, corners [4]Point, opts QuadDrawOptions) error {
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
	opacity := normalizeQuadOpacity(opts.Opacity)
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
				c.drawImageQuadCPUSplit(img, dev, interp, opacity)
				return nil
			}
			defer c.setGPUClipRect()()
			target := c.gpuRenderTarget()
			vpW := uint32(target.Width)  //nolint:gosec
			vpH := uint32(target.Height) //nolint:gosec
			nearest := interp == InterpNearest
			bicubic := interp == InterpBicubic
			rc.QueueImageDraw(target, pixelData, img.GenerationID(), imgW, imgH, img.Stride(),
				float32(dev[0].X), float32(dev[0].Y),
				float32(dev[1].X), float32(dev[1].Y),
				float32(dev[2].X), float32(dev[2].Y),
				float32(dev[3].X), float32(dev[3].Y),
				float32(opacity), vpW, vpH,
				0, 0, 1, 1,
				nearest, false, bicubic)
			c.recordGPUOp()
			return nil
		}
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("image:DrawImageQuad")
	}
	c.drawImageQuadCPUSplit(img, dev, interp, opacity)
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

// quadHasNonFinite reports any non-finite corner.
func (c *Context) drawImageQuadCPU(img *ImageBuf, corners [4]Point) {
	// Compat: user-space corners in, device split inside Ex.
	_ = c.DrawImageQuadEx(img, corners, QuadDrawOptions{
		Interpolation: InterpBilinear, Opacity: 1, BlendMode: BlendNormal,
	})
}

// drawImageQuadCPUSplit rasterizes device-space quad with GPU twin split.
// drawImageQuadCPUSplit rasterizes device-space quad with GPU twin split.
func (c *Context) drawImageQuadCPUSplit(img *ImageBuf, dev [4]Point, interp InterpolationMode, opacity float64) {
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
	// GPU twin UVs: TL(0,0) TR(1,0) BR(1,1) BL(0,1), denominators hoisted per quad.
	t1 := newTriSampler(dev[0], dev[1], dev[3], [3][2]float64{{0, 0}, {1, 0}, {0, 1}})
	t2 := newTriSampler(dev[1], dev[2], dev[3], [3][2]float64{{1, 0}, {1, 1}, {0, 1}})
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
				srcA = float64(sa) / 255.0 * k
				srcR = float64(sr) / 255.0 * k
				srcG = float64(sg) / 255.0 * k
				srcB = float64(sb) / 255.0 * k
			} else {
				baseA := float64(sa) / 255.0
				srcA = baseA * k
				srcR = float64(sr) / 255.0 * srcA
				srcG = float64(sg) / 255.0 * srcA
				srcB = float64(sb) / 255.0 * srcA
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
}

// PushBackdropLayer creates a layer pre-filled with a snapshot of the current
// parent canvas (L.05 backdrop subset). Subsequent drawing/filters operate over
// that backdrop; PopLayer composites with the given blend/opacity.
func (c *Context) PushBackdropLayer(blendMode BlendMode, opacity float64) {
	if c == nil {
		return
	}
	// Ensure GPU content is resolved into the current pixmap before snapshot.
	_ = c.FlushGPU()
	c.applyDitherIfEnabled()

	parent := c.pixmap
	c.pushLayerSurface(blendMode, opacity, false, false)
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
	if !c.seedTopLayerGPUFromPixmap() {
		// If seed fails, mark CPU drew so Pop uses pixmap composite path.
		c.noteLayerCPUDraw()
	}
	// Backdrop is a full-surface snapshot; Pop must blend the whole layer.
	c.markLayerFullComposite()
}
