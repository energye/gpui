package render

// filterApplyFunc applies a filter from src into dst over the full pixmap.
type filterApplyFunc func(src, dst *Pixmap)

var (
	blurApply        func(src, dst *Pixmap, radius float64)
	blurXYApply      func(src, dst *Pixmap, radiusX, radiusY float64)
	dropShadowApply  func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA)
	colorMatrixApply func(src, dst *Pixmap, matrix [20]float32)
	grayscaleApply   func(src, dst *Pixmap)
	invertApply      func(src, dst *Pixmap)
)

// RegisterFilterOps wires image-filter implementations (blur/shadow/color matrix).
// Called from render/filters init to avoid import cycles with internal/filter.
func RegisterFilterOps(
	blur func(src, dst *Pixmap, radius float64),
	blurXY func(src, dst *Pixmap, radiusX, radiusY float64),
	shadow func(src, dst *Pixmap, offsetX, offsetY, blur float64, color RGBA),
	colorMatrix func(src, dst *Pixmap, matrix [20]float32),
	grayscale func(src, dst *Pixmap),
	invert func(src, dst *Pixmap),
) {
	blurApply = blur
	blurXYApply = blurXY
	dropShadowApply = shadow
	colorMatrixApply = colorMatrix
	grayscaleApply = grayscale
	invertApply = invert
}

func (c *Context) applyFilterInPlace(fn filterApplyFunc) {
	if c == nil || c.pixmap == nil || fn == nil {
		return
	}
	_ = c.FlushGPU()
	src := c.pixmap
	dst := NewPixmap(src.Width(), src.Height())
	dst.Clear(Transparent)
	copy(dst.Data(), src.Data())
	fn(src, dst)
	copy(src.Data(), dst.Data())
	src.NotifyPixelsChanged()
}

// ApplyBlur applies a Gaussian blur to the current surface contents (F.01).
// Requires render/filters registration (blank-import).
func (c *Context) ApplyBlur(radius float64) {
	if radius <= 0 || blurApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurApply(src, dst, radius)
	})
}

// ApplyBlurXY applies an anisotropic Gaussian blur.
func (c *Context) ApplyBlurXY(radiusX, radiusY float64) {
	if (radiusX <= 0 && radiusY <= 0) || blurXYApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		blurXYApply(src, dst, radiusX, radiusY)
	})
}

// ApplyDropShadow composites a drop shadow under current surface contents (F.02).
func (c *Context) ApplyDropShadow(offsetX, offsetY, blurRadius float64, color RGBA) {
	if dropShadowApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		dropShadowApply(src, dst, offsetX, offsetY, blurRadius, color)
	})
}

// ApplyColorMatrix applies a 4x5 color transformation matrix (F.04).
func (c *Context) ApplyColorMatrix(matrix [20]float32) {
	if colorMatrixApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		colorMatrixApply(src, dst, matrix)
	})
}

// ApplyGrayscale converts the surface to grayscale via color matrix (F.04).
func (c *Context) ApplyGrayscale() {
	if grayscaleApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		grayscaleApply(src, dst)
	})
}

// ApplyInvert inverts RGB channels via color matrix (F.04).
func (c *Context) ApplyInvert() {
	if invertApply == nil {
		return
	}
	c.applyFilterInPlace(func(src, dst *Pixmap) {
		invertApply(src, dst)
	})
}

// FiltersRegistered reports whether image filter ops were registered.
func FiltersRegistered() bool {
	return blurApply != nil && dropShadowApply != nil && grayscaleApply != nil
}
