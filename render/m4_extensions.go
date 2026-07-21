package render

import (
	"image"
	"math"
)

// SetDither enables ordered dithering of soft fills after GPU resolve (P.09).
// When enabled, subsequent Image()/FlushGPU results for this context receive a
// Bayer 4x4 ordered dither on the pixmap (reduces gradient banding).
func (c *Context) SetDither(enabled bool) {
	if c == nil {
		return
	}
	c.dither = enabled
}

// Dither reports whether ordered dithering is enabled.
func (c *Context) Dither() bool {
	if c == nil {
		return false
	}
	return c.dither
}

func (c *Context) applyDitherIfEnabled() {
	if c == nil || !c.dither || c.pixmap == nil {
		return
	}
	applyBayerDither4(c.pixmap)
	c.pixmap.NotifyPixelsChanged()
}

// Bayer 4x4 thresholds in 0..15, scaled into low-bit noise for 8-bit channels.
var bayer4 = [4][4]int{
	{0, 8, 2, 10},
	{12, 4, 14, 6},
	{3, 11, 1, 9},
	{15, 7, 13, 5},
}

func applyBayerDither4(pm *Pixmap) {
	if pm == nil {
		return
	}
	w, h := pm.Width(), pm.Height()
	data := pm.Data()
	stride := pm.Width() * 4
	if len(data) < h*stride {
		return
	}
	for y := 0; y < h; y++ {
		row := y * stride
		for x := 0; x < w; x++ {
			thr := bayer4[y&3][x&3] // 0..15
			// Bias each channel by ±2 based on threshold vs low bits.
			off := row + x*4
			for c := 0; c < 3; c++ {
				v := int(data[off+c])
				// ordered: if (v & 15) > thr then bump else leave; mild banding break
				if (v & 15) > thr {
					if v < 253 {
						v += 2
					}
				} else if v > 2 {
					v -= 1
				}
				if v < 0 {
					v = 0
				}
				if v > 255 {
					v = 255
				}
				data[off+c] = byte(v)
			}
		}
	}
}

// DrawImageQuad draws an image into a free-form destination quad (T.04 non-affine subset).
// corners are user-space points in order: top-left, top-right, bottom-right, bottom-left.
// GPU path uses QueueImageDraw with arbitrary corner mapping (perspective-like trapezoids).
func (c *Context) DrawImageQuad(img *ImageBuf, corners [4]Point) {
	if c == nil || img == nil {
		return
	}
	imgW, imgH := img.Bounds()
	if imgW <= 0 || imgH <= 0 {
		return
	}
	ctm := c.totalMatrix()
	dev := [4]Point{}
	for i := 0; i < 4; i++ {
		dev[i] = ctm.TransformPoint(corners[i])
	}
	// Damage bounds
	minX := math.Min(math.Min(dev[0].X, dev[1].X), math.Min(dev[2].X, dev[3].X))
	minY := math.Min(math.Min(dev[0].Y, dev[1].Y), math.Min(dev[2].Y, dev[3].Y))
	maxX := math.Max(math.Max(dev[0].X, dev[1].X), math.Max(dev[2].X, dev[3].X))
	maxY := math.Max(math.Max(dev[0].Y, dev[1].Y), math.Max(dev[2].Y, dev[3].Y))
	c.trackDamage(image.Rect(int(minX), int(minY), int(maxX)+1, int(maxY)+1))

	if rc := c.gpuCtxOps(); rc != nil {
		pixelData := img.PremultipliedData()
		if len(pixelData) == 0 {
			if c.gpuPathAvailable() {
				c.recordCPUFallbackReason("image:DrawImageQuad")
			}
			c.drawImageQuadCPU(img, corners)
			return
		}
		defer c.setGPUClipRect()()
		target := c.gpuRenderTarget()
		vpW := uint32(target.Width)  //nolint:gosec
		vpH := uint32(target.Height) //nolint:gosec
		rc.QueueImageDraw(target, pixelData, img.GenerationID(), imgW, imgH, img.Stride(),
			float32(dev[0].X), float32(dev[0].Y),
			float32(dev[1].X), float32(dev[1].Y),
			float32(dev[2].X), float32(dev[2].Y),
			float32(dev[3].X), float32(dev[3].Y),
			1.0, vpW, vpH,
			0, 0, 1, 1,
			false, false)
		c.recordGPUOp()
		return
	}
	if c.gpuPathAvailable() {
		c.recordCPUFallbackReason("image:DrawImageQuad")
	}
	c.drawImageQuadCPU(img, corners)
}

func (c *Context) drawImageQuadCPU(img *ImageBuf, corners [4]Point) {
	// Fallback: axis-aligned bounds draw (loses perspective; last-resort).
	minX := math.Min(math.Min(corners[0].X, corners[1].X), math.Min(corners[2].X, corners[3].X))
	minY := math.Min(math.Min(corners[0].Y, corners[1].Y), math.Min(corners[2].Y, corners[3].Y))
	maxX := math.Max(math.Max(corners[0].X, corners[1].X), math.Max(corners[2].X, corners[3].X))
	maxY := math.Max(math.Max(corners[0].Y, corners[1].Y), math.Max(corners[2].Y, corners[3].Y))
	c.DrawImageEx(img, DrawImageOptions{
		X: minX, Y: minY, DstWidth: maxX - minX, DstHeight: maxY - minY,
		Interpolation: InterpBilinear, Opacity: 1, BlendMode: BlendNormal,
	})
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
	// S6.4: push layer without Clear — full backdrop copy overwrites every pixel.
	c.pushLayerSurface(blendMode, opacity, false)
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
