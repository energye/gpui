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

// ImageFilterKind identifies a node in an image-filter graph (F.03).
type ImageFilterKind int

const (
	// ImageFilterBlur is isotropic Gaussian blur.
	ImageFilterBlur ImageFilterKind = iota
	// ImageFilterBlurXY is anisotropic Gaussian blur.
	ImageFilterBlurXY
	// ImageFilterDropShadow composites a drop shadow under content.
	ImageFilterDropShadow
	// ImageFilterColorMatrix applies a 4x5 color matrix.
	ImageFilterColorMatrix
	// ImageFilterGrayscale converts to grayscale.
	ImageFilterGrayscale
	// ImageFilterInvert inverts RGB channels.
	ImageFilterInvert
)

// ImageFilterNode is one node in a multi-pass image filter graph (F.03).
// Unused fields for a given Kind are ignored.
type ImageFilterNode struct {
	Kind ImageFilterKind

	// Blur / BlurXY
	Radius  float64
	RadiusX float64
	RadiusY float64

	// DropShadow
	OffsetX, OffsetY float64
	ShadowBlur       float64
	ShadowColor      RGBA

	// ColorMatrix
	Matrix [20]float32
}

// ApplyImageFilterGraph runs a multi-node image filter graph with intermediate
// surface ping-pong (F.03 / Skia-style ImageFilter DAG chain).
//
// Pipeline:
//  1. FlushGPU so the graph operates on real GPU-produced pixels when content
//     was drawn on the native path.
//  2. Apply nodes in order using two full-size intermediate pixmaps (ping-pong),
//     matching multi-RT filter graphs without in-place clobber.
//  3. Write the final result back to the context surface.
//
// Requires render/filters registration (blank-import).
func (c *Context) ApplyImageFilterGraph(nodes ...ImageFilterNode) {
	if c == nil || c.pixmap == nil || len(nodes) == 0 {
		return
	}
	runnable := 0
	for i := range nodes {
		if imageFilterNodeRunnable(nodes[i]) {
			runnable++
		}
	}
	if runnable == 0 {
		return
	}

	_ = c.FlushGPU()
	src := c.pixmap
	w, h := src.Width(), src.Height()
	if w <= 0 || h <= 0 {
		return
	}

	// Ping-pong intermediate surfaces (multi-pass RT analogue).
	bufA := NewPixmap(w, h)
	bufB := NewPixmap(w, h)
	copy(bufA.Data(), src.Data())
	bufA.NotifyPixelsChanged()

	cur, nxt := bufA, bufB
	for i := range nodes {
		n := nodes[i]
		if !imageFilterNodeRunnable(n) {
			continue
		}
		// Seed next buffer with current so partial writes keep prior content.
		copy(nxt.Data(), cur.Data())
		nxt.NotifyPixelsChanged()
		applyImageFilterNode(n, cur, nxt)
		nxt.NotifyPixelsChanged()
		cur, nxt = nxt, cur
	}

	copy(src.Data(), cur.Data())
	src.NotifyPixelsChanged()
}

func imageFilterNodeRunnable(n ImageFilterNode) bool {
	switch n.Kind {
	case ImageFilterBlur:
		return n.Radius > 0 && blurApply != nil
	case ImageFilterBlurXY:
		return (n.RadiusX > 0 || n.RadiusY > 0) && blurXYApply != nil
	case ImageFilterDropShadow:
		return dropShadowApply != nil
	case ImageFilterColorMatrix:
		return colorMatrixApply != nil
	case ImageFilterGrayscale:
		return grayscaleApply != nil
	case ImageFilterInvert:
		return invertApply != nil
	default:
		return false
	}
}

func applyImageFilterNode(n ImageFilterNode, src, dst *Pixmap) {
	switch n.Kind {
	case ImageFilterBlur:
		blurApply(src, dst, n.Radius)
	case ImageFilterBlurXY:
		blurXYApply(src, dst, n.RadiusX, n.RadiusY)
	case ImageFilterDropShadow:
		dropShadowApply(src, dst, n.OffsetX, n.OffsetY, n.ShadowBlur, n.ShadowColor)
	case ImageFilterColorMatrix:
		colorMatrixApply(src, dst, n.Matrix)
	case ImageFilterGrayscale:
		grayscaleApply(src, dst)
	case ImageFilterInvert:
		invertApply(src, dst)
	}
}
