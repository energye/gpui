package render

import (
	"image"
	"math"

	gpucontext "github.com/energye/gpui/gpu/context"
	intImage "github.com/energye/gpui/render/internal/image"
)

// ImageBuf is a public alias for internal ImageBuf.
// It represents a memory-efficient image buffer with support for multiple
// pixel formats and lazy premultiplication.
type ImageBuf = intImage.ImageBuf

// InterpolationMode defines how texture sampling is performed when drawing images.
type InterpolationMode = intImage.InterpolationMode

// Image interpolation modes.
const (
	// InterpNearest selects the closest pixel (no interpolation).
	// Fast but produces blocky results when scaling.
	InterpNearest = intImage.InterpNearest

	// InterpBilinear performs linear interpolation between 4 neighboring pixels.
	// Good balance between quality and performance.
	InterpBilinear = intImage.InterpBilinear

	// InterpBicubic performs cubic interpolation using a 4x4 pixel neighborhood.
	// Highest quality but slower than bilinear.
	InterpBicubic = intImage.InterpBicubic
)

// ImageFormat represents a pixel storage format.
type ImageFormat = intImage.Format

// Pixel formats.
const (
	// FormatGray8 is 8-bit grayscale (1 byte per pixel).
	FormatGray8 = intImage.FormatGray8

	// FormatGray16 is 16-bit grayscale (2 bytes per pixel).
	FormatGray16 = intImage.FormatGray16

	// FormatRGB8 is 24-bit RGB (3 bytes per pixel, no alpha).
	FormatRGB8 = intImage.FormatRGB8

	// FormatRGBA8 is 32-bit RGBA in sRGB color space (4 bytes per pixel).
	// This is the standard format for most operations.
	FormatRGBA8 = intImage.FormatRGBA8

	// FormatRGBAPremul is 32-bit RGBA with premultiplied alpha (4 bytes per pixel).
	// Used for correct alpha blending operations.
	FormatRGBAPremul = intImage.FormatRGBAPremul

	// FormatBGRA8 is 32-bit BGRA in sRGB color space (4 bytes per pixel).
	// Common on Windows and some GPU formats.
	FormatBGRA8 = intImage.FormatBGRA8

	// FormatBGRAPremul is 32-bit BGRA with premultiplied alpha (4 bytes per pixel).
	FormatBGRAPremul = intImage.FormatBGRAPremul
)

// BlendMode defines how source pixels are blended with destination pixels.
type BlendMode = intImage.BlendMode

// Blend modes.
const (
	// BlendNormal performs standard alpha blending (source over destination).
	BlendNormal = intImage.BlendNormal

	// BlendMultiply multiplies source and destination colors.
	// Result is always darker or equal. Formula: dst * src
	BlendMultiply = intImage.BlendMultiply

	// BlendScreen performs inverse multiply for lighter results.
	// Formula: 1 - (1-dst) * (1-src)
	BlendScreen = intImage.BlendScreen

	// BlendOverlay combines multiply and screen based on destination brightness.
	// Dark areas are multiplied, bright areas are screened.
	BlendOverlay = intImage.BlendOverlay

	// BlendHue is a non-separable HSL-style blend (B.04).
	BlendHue = intImage.BlendHue
	// BlendSaturation is a non-separable HSL-style blend (B.04).
	BlendSaturation = intImage.BlendSaturation
	// BlendColor is a non-separable HSL-style blend (B.04).
	BlendColor = intImage.BlendColor
	// BlendLuminosity is a non-separable HSL-style blend (B.04).
	BlendLuminosity = intImage.BlendLuminosity

	// BlendClear is Porter-Duff Clear (B.02): result is transparent black.
	BlendClear = intImage.BlendClear
	// BlendCopy is Porter-Duff Src/Copy (B.02): result is source.
	BlendCopy = intImage.BlendCopy
	// BlendPlus is Porter-Duff Plus (B.02 / B.07): clamped source+destination.
	BlendPlus = intImage.BlendPlus
	// BlendModulate multiplies source*destination (Skia kModulate / B.07).
	BlendModulate = intImage.BlendModulate
	// BlendDestinationOut is Porter-Duff DstOut (B.02).
	BlendDestinationOut = intImage.BlendDestinationOut
	// BlendSourceAtop is Porter-Duff SrcAtop (B.02).
	BlendSourceAtop = intImage.BlendSourceAtop
	// BlendXor is Porter-Duff Xor (B.02).
	BlendXor = intImage.BlendXor
	// BlendDestinationOver is Porter-Duff DstOver (B.02).
	BlendDestinationOver = intImage.BlendDestinationOver
	// BlendSourceIn is Porter-Duff SrcIn (B.02).
	BlendSourceIn = intImage.BlendSourceIn
	// BlendSourceOut is Porter-Duff SrcOut (B.02).
	BlendSourceOut = intImage.BlendSourceOut
	// BlendDestinationIn is Porter-Duff DstIn (B.02).
	BlendDestinationIn = intImage.BlendDestinationIn
	// BlendDestinationAtop is Porter-Duff DstAtop (B.02).
	BlendDestinationAtop = intImage.BlendDestinationAtop

	// Separable advanced modes (B.03 extended).
	BlendDarken     = intImage.BlendDarken
	BlendLighten    = intImage.BlendLighten
	BlendColorDodge = intImage.BlendColorDodge
	BlendColorBurn  = intImage.BlendColorBurn
	BlendHardLight  = intImage.BlendHardLight
	BlendSoftLight  = intImage.BlendSoftLight
	BlendDifference = intImage.BlendDifference
	BlendExclusion  = intImage.BlendExclusion
)

// DrawImageOptions specifies parameters for drawing an image.
type DrawImageOptions struct {
	// X, Y specify the top-left corner where the image will be drawn.
	X, Y float64

	// DstWidth and DstHeight specify the dimensions to scale the image to.
	// If zero, the source dimensions are used (possibly from SrcRect).
	DstWidth  float64
	DstHeight float64

	// SrcRect defines the source rectangle to sample from.
	// If nil, the entire source image is used.
	SrcRect *image.Rectangle

	// Interpolation specifies the interpolation mode for sampling.
	// Default is InterpBilinear.
	Interpolation InterpolationMode

	// Opacity controls the overall transparency of the source image (0.0 to 1.0).
	// 1.0 means fully opaque, 0.0 means fully transparent.
	// Default is 1.0.
	Opacity float64

	// UseMipmaps enables mipmap sampling when the image is drawn smaller than
	// its native size (I.04). Currently applied on the CPU image path.
	UseMipmaps bool

	// BlendMode specifies how to blend source and destination pixels.
	// Default is BlendNormal.
	BlendMode BlendMode
}

// IsAdvancedBlendMode reports whether mode requires destination sampling
// (separable/non-separable advanced blends) rather than fixed-function factors.
// Used by layer Pop dual-tex (P0-3 / G.06).
func IsAdvancedBlendMode(mode BlendMode) bool {
	switch mode {
	case BlendMultiply, BlendScreen, BlendOverlay,
		BlendDarken, BlendLighten, BlendColorDodge, BlendColorBurn,
		BlendHardLight, BlendSoftLight, BlendDifference, BlendExclusion,
		BlendHue, BlendSaturation, BlendColor, BlendLuminosity:
		return true
	default:
		return false
	}
}

// DrawImage draws an image at the specified position.
// The current transformation matrix is applied to the position and size.
//
// Example:
//
//	img, _ := gg.LoadImage("photo.png")
//	dc.DrawImage(img, 100, 100)
func (c *Context) DrawImage(img *ImageBuf, x, y float64) {
	c.DrawImageEx(img, DrawImageOptions{
		X:             x,
		Y:             y,
		Interpolation: InterpBilinear,
		Opacity:       1.0,
		BlendMode:     BlendNormal,
	})
}

// DrawImageEx draws an image with advanced options.
// The current transformation matrix is applied to the position and size.
// The image is drawn through the Fill() pipeline, which means it respects
// the current clip region. This follows the enterprise pattern used by
// Skia, Cairo, tiny-skia, and Vello: image drawing = fillRect + image shader.
//
// Example:
//
//	dc.DrawImageEx(img, gg.DrawImageOptions{
//	    X:             100,
//	    Y:             100,
//	    DstWidth:      200,
//	    DstHeight:     150,
//	    Interpolation: gg.InterpBicubic,
//	    Opacity:       0.8,
//	    BlendMode:     gg.BlendNormal,
//	})
func (c *Context) DrawImageEx(img *ImageBuf, opts DrawImageOptions) {
	// Default values. InterpNearest starts at 1 so zero means unspecified, not nearest.
	if opts.Interpolation == 0 {
		opts.Interpolation = InterpBilinear
	}
	if opts.Opacity == 0 {
		opts.Opacity = 1.0
	}

	// Get source dimensions
	srcWidth, srcHeight := img.Bounds()
	srcX, srcY := 0, 0
	srcW, srcH := srcWidth, srcHeight
	if opts.SrcRect != nil {
		srcX = opts.SrcRect.Min.X
		srcY = opts.SrcRect.Min.Y
		srcW = opts.SrcRect.Dx()
		srcH = opts.SrcRect.Dy()
	}

	// Determine destination size in user coordinates (before transform).
	dstWidth := opts.DstWidth
	dstHeight := opts.DstHeight
	if dstWidth == 0 {
		dstWidth = float64(srcW)
	}
	if dstHeight == 0 {
		dstHeight = float64(srcH)
	}

	// I.04 R1: Prefer GPU textured quads. UseMipmaps uses GPU bilinear (approx
	// vs true mip chain). Bicubic remains CPU† for filter correctness.
	if opts.Interpolation != InterpBicubic {
		if c.tryGPUDrawImage(img, opts, srcX, srcY, srcW, srcH, dstWidth, dstHeight) {
			return
		}
	} else {
		// Direct CPU DrawImage with bicubic sampling for correctness.
		dstImg := c.pixmapToImageBuf(c.pixmap)
		if dstImg != nil {
			srcRect := &intImage.Rect{X: srcX, Y: srcY, Width: srcW, Height: srcH}
			// Destination in device pixels via CTM translation+scale approx for axis-aligned.
			ctm := c.totalMatrix()
			tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
			br := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y+dstHeight))
			dx := int(math.Min(tl.X, br.X))
			dy := int(math.Min(tl.Y, br.Y))
			dw := int(math.Abs(br.X-tl.X) + 0.5)
			dh := int(math.Abs(br.Y-tl.Y) + 0.5)
			if dw > 0 && dh > 0 {
				intImage.DrawImage(dstImg, img, intImage.DrawParams{
					SrcRect:    srcRect,
					DstRect:    intImage.Rect{X: dx, Y: dy, Width: dw, Height: dh},
					Interp:     opts.Interpolation,
					Opacity:    opts.Opacity,
					BlendMode:  opts.BlendMode,
					UseMipmaps: opts.UseMipmaps,
				})
				c.recordCPUFallbackReason("image:bicubic")
				return
			}
		}
	}

	// Compute scale factors for the image pattern.
	// The pattern maps source pixels to destination pixels.
	scaleX := dstWidth / float64(srcW)
	scaleY := dstHeight / float64(srcH)

	// Compose the full forward transform: image-space -> device-space.
	//
	// The transform chain is:
	//   1. Scale source pixels to destination size: Scale(scaleX, scaleY)
	//   2. Position at destination: Translate(opts.X, opts.Y)
	//   3. Apply current CTM to device space: totalMatrix()
	//
	// Forward: device = totalMatrix * Translate(x,y) * Scale(sx,sy) * imageCoord
	// Inverse: imageCoord = inverse(totalMatrix * Translate(x,y) * Scale(sx,sy)) * device
	//
	// This follows the Cairo/Skia/tiny-skia pattern (see IMAGE-PATTERN-TRANSFORM-RESEARCH.md).
	patternTransform := c.totalMatrix().
		Multiply(Translate(opts.X, opts.Y)).
		Multiply(Scale(scaleX, scaleY))
	inverse := patternTransform.Invert()

	// Create image pattern with pre-computed inverse transform.
	pattern := &ImagePattern{
		image:   img,
		x:       srcX,
		y:       srcY,
		w:       srcW,
		h:       srcH,
		inverse: inverse,
		opacity: opts.Opacity,
		clamp:   true, // Don't tile — clamp to image bounds
	}

	// Save current state (paint, path, transform).
	c.Push()

	// Set image pattern as fill source.
	c.SetFillPattern(pattern)

	// Draw rectangle at destination using the current transform.
	// DrawRectangle applies the transform, which is what we want.
	c.DrawRectangle(opts.X, opts.Y, dstWidth, dstHeight)

	// Fill the rectangle — the Fill() pipeline handles clipping automatically.
	_ = c.Fill()

	c.Pop()
}

// tryGPUDrawImage attempts to render the image via GPU Tier 3 (textured quad).
// Returns true if the image was queued for GPU rendering, false if the caller
// should fall back to the CPU SetFillPattern→Fill() path.
//
// The destination is always a full CTM-transformed parallelogram (TL/TR/BR/BL).
// Rotation and skew are handled by emitting non-axis-aligned quad vertices;
// the previous axis-aligned-only gate incorrectly fell back to ImagePattern
// Fill, which GPU solid-path rendering cannot texture.
func (c *Context) tryGPUDrawImage(img *ImageBuf, opts DrawImageOptions, srcX, srcY, srcW, srcH int, dstWidth, dstHeight float64) bool {
	rc := c.gpuCtxOps()
	if rc == nil {
		if c.gpuPathAvailable() {
			c.recordCPUFallbackReason("image:tryGPUDrawImage")
		}
		return false
	}

	defer c.setGPUClipRect()()

	// Transform all four user-space corners through the full CTM so rotation,
	// scale, shear, and translation are preserved in device pixels.
	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(opts.X, opts.Y))
	tr := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y))
	br := ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y+dstHeight))
	bl := ctm.TransformPoint(Pt(opts.X, opts.Y+dstHeight))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
	vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32

	// Compute source UV rectangle (normalized 0..1 within the image).
	imgW, imgH := img.Bounds()
	u0 := float32(srcX) / float32(imgW)
	v0 := float32(srcY) / float32(imgH)
	u1 := float32(srcX+srcW) / float32(imgW)
	v1 := float32(srcY+srcH) / float32(imgH)

	// Get premultiplied pixel data for GPU upload.
	pixelData := img.PremultipliedData()
	if len(pixelData) == 0 {
		c.recordCPUFallbackReason("image:tryGPUDrawImage")
		return false
	}

	nearest := opts.Interpolation == InterpNearest
	contentDirty := img.TakeGPUDirty()
	rc.QueueImageDraw(target, pixelData, img.GenerationID(), imgW, imgH, img.Stride(),
		float32(tl.X), float32(tl.Y),
		float32(tr.X), float32(tr.Y),
		float32(br.X), float32(br.Y),
		float32(bl.X), float32(bl.Y),
		float32(opts.Opacity), vpW, vpH, u0, v0, u1, v1, nearest, contentDirty)
	c.recordGPUOp()
	return true
}

// CreateImagePattern creates an image pattern from a rectangular region of an image.
// The pattern can be used with SetFillPattern or SetStrokePattern.
//
// Example:
//
//	img, _ := gg.LoadImage("texture.png")
//	pattern := dc.CreateImagePattern(img, 0, 0, 100, 100)
//	dc.SetFillPattern(pattern)
//	dc.DrawRectangle(0, 0, 400, 300)
//	dc.Fill()
func (c *Context) CreateImagePattern(img *ImageBuf, x, y, w, h int) Pattern {
	return &ImagePattern{
		image:   img,
		x:       x,
		y:       y,
		w:       w,
		h:       h,
		inverse: Identity(), // identity transform: device coords = image coords (tiling from origin)
	}
}

// SetFillPattern sets the fill pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
func (c *Context) SetFillPattern(pattern Pattern) {
	if sp, ok := pattern.(*SolidPattern); ok {
		c.paint.solidColor = sp.Color
		c.paint.isSolid = true
		c.paint.Brush = nil
		c.paint.Pattern = nil
		return
	}
	c.paint.Pattern = pattern
	c.paint.Brush = BrushFromPattern(pattern)
	c.paint.isSolid = false
}

// SetStrokePattern sets the stroke pattern.
// It also updates the Brush field for consistency with ColorAt precedence.
// For solid patterns, stores the color inline (zero allocations).
func (c *Context) SetStrokePattern(pattern Pattern) {
	if sp, ok := pattern.(*SolidPattern); ok {
		c.paint.solidColor = sp.Color
		c.paint.isSolid = true
		c.paint.Brush = nil
		c.paint.Pattern = nil
		return
	}
	c.paint.Pattern = pattern
	c.paint.Brush = BrushFromPattern(pattern)
	c.paint.isSolid = false
}

// ImagePattern represents an image-based pattern with full affine transform support.
// The pattern stores a pre-computed inverse matrix that maps device-space coordinates
// back to image-space for sampling. This enables correct rendering under rotation,
// scale, skew, and any combination of affine transforms.
//
// For backward compatibility, SetAnchor/SetScale convenience methods rebuild
// the inverse from anchor+scale parameters. For full affine control, the inverse
// is set directly by DrawImageEx or via SetTransform.
type ImagePattern struct {
	image   *ImageBuf
	x, y    int     // source region offset within the image
	w, h    int     // source region size (0 = full image dimension)
	inverse Matrix  // maps device-space -> image-space (pre-computed)
	opacity float64 // opacity multiplier (0=transparent, 1=opaque; 0 means default=1)
	clamp   bool    // when true, out-of-bounds returns transparent instead of tiling

	// Legacy fields for SetAnchor/SetScale backward compatibility.
	// When these are used, rebuildInverse() computes the inverse from them.
	anchorX float64
	anchorY float64
	scaleX  float64 // horizontal scale factor (0 means 1.0)
	scaleY  float64 // vertical scale factor (0 means 1.0)
}

// SetAnchor sets the canvas position where the pattern is anchored.
// This offsets all coordinate lookups so the image appears at (x, y)
// on the canvas rather than tiled from the origin.
// The inverse transform is rebuilt to reflect the new anchor.
func (p *ImagePattern) SetAnchor(x, y float64) {
	p.anchorX = x
	p.anchorY = y
	p.rebuildInverse()
}

// SetOpacity sets the opacity multiplier for the pattern (0.0 to 1.0).
func (p *ImagePattern) SetOpacity(opacity float64) {
	p.opacity = opacity
}

// SetClamp enables clamp mode. When true, coordinates outside the image region
// return transparent black instead of tiling/wrapping.
func (p *ImagePattern) SetClamp(clamp bool) {
	p.clamp = clamp
}

// SetScale sets the scale factors for the pattern. The source image is scaled
// by these factors before being sampled. For example, SetScale(2, 2) makes
// each source pixel cover 2x2 destination pixels.
// The inverse transform is rebuilt to reflect the new scale.
func (p *ImagePattern) SetScale(sx, sy float64) {
	p.scaleX = sx
	p.scaleY = sy
	p.rebuildInverse()
}

// SetTransform sets the full forward transform (image-space to device-space)
// for the pattern. The inverse is computed and cached for sampling.
// This overrides any anchor/scale settings.
func (p *ImagePattern) SetTransform(m Matrix) {
	p.inverse = m.Invert()
}

// GPUPatternSource exposes image pattern fields for GPU-native fill (G.03).
// Returns nil image when the pattern is empty/invalid.
func (p *ImagePattern) GPUPatternSource() (img *ImageBuf, srcX, srcY, srcW, srcH int, inverse Matrix, opacity float64, clamp bool) {
	if p == nil || p.image == nil {
		return nil, 0, 0, 0, 0, Identity(), 1, false
	}
	imgW, imgH := p.image.Bounds()
	srcW, srcH = p.w, p.h
	if srcW <= 0 {
		srcW = imgW
	}
	if srcH <= 0 {
		srcH = imgH
	}
	op := p.opacity
	if op <= 0 {
		op = 1
	}
	return p.image, p.x, p.y, srcW, srcH, p.inverse, op, p.clamp
}

// rebuildInverse computes the inverse transform from the legacy anchor+scale fields.
// The forward transform is: Translate(anchorX, anchorY) * Scale(scaleX, scaleY),
// mapping image coordinates to device coordinates.
func (p *ImagePattern) rebuildInverse() {
	sx := p.scaleX
	if sx == 0 {
		sx = 1
	}
	sy := p.scaleY
	if sy == 0 {
		sy = 1
	}
	// Forward: device = Translate(anchor) * Scale(s) * imageCoord
	// Inverse: imageCoord = Scale(1/s) * Translate(-anchor) * device
	//        = (device - anchor) / s
	forward := Translate(p.anchorX, p.anchorY).Multiply(Scale(sx, sy))
	p.inverse = forward.Invert()
}

// ColorAt implements the Pattern interface.
// It samples the image at the given device-space coordinates by applying the
// pre-computed inverse transform to map back to image-space. In clamp mode,
// out-of-bounds coordinates return transparent black; otherwise the pattern tiles.
func (p *ImagePattern) ColorAt(x, y float64) RGBA {
	// Apply inverse transform: device-space -> image-space.
	imgPt := p.inverse.TransformPoint(Pt(x, y))
	lx := imgPt.X
	ly := imgPt.Y

	// Get image bounds.
	imgW, imgH := p.image.Bounds()

	// Determine pattern region.
	patternW := p.w
	patternH := p.h
	if patternW == 0 {
		patternW = imgW
	}
	if patternH == 0 {
		patternH = imgH
	}

	if p.clamp {
		// Clamp mode: out-of-bounds returns transparent.
		ix := int(lx)
		iy := int(ly)
		if ix < 0 || ix >= patternW || iy < 0 || iy >= patternH {
			return RGBA{}
		}
		ix += p.x
		iy += p.y
		r, g, b, a := p.image.GetRGBA(ix, iy)
		col := RGBA{
			R: float64(r) / 255.0,
			G: float64(g) / 255.0,
			B: float64(b) / 255.0,
			A: float64(a) / 255.0,
		}
		if p.opacity > 0 && p.opacity < 1.0 {
			col.A *= p.opacity
		}
		return col
	}

	// Wrap coordinates to pattern region (tiling).
	px := int(lx) % patternW
	py := int(ly) % patternH
	if px < 0 {
		px += patternW
	}
	if py < 0 {
		py += patternH
	}

	// Add source region offset.
	px += p.x
	py += p.y

	// Sample the image.
	r, g, b, a := p.image.GetRGBA(px, py)
	col := RGBA{
		R: float64(r) / 255.0,
		G: float64(g) / 255.0,
		B: float64(b) / 255.0,
		A: float64(a) / 255.0,
	}
	if p.opacity > 0 && p.opacity < 1.0 {
		col.A *= p.opacity
	}
	return col
}

// DrawImageRounded draws an image at (x, y) clipped to a rounded rectangle.
// The image is drawn at its natural size, clipped by a rounded rectangle with
// the given corner radius. This is a convenience method equivalent to:
//
//	dc.Push()
//	dc.DrawRoundedRectangle(x, y, w, h, radius)
//	dc.Clip()
//	dc.DrawImage(img, x, y)
//	dc.Pop()
func (c *Context) DrawImageRounded(img *ImageBuf, x, y, radius float64) {
	w, h := img.Bounds()
	fw := float64(w)
	fh := float64(h)

	c.Push()
	c.DrawRoundedRectangle(x, y, fw, fh, radius)
	c.Clip()
	c.DrawImage(img, x, y)
	c.Pop()
}

// DrawImageCircular draws an image at the specified center, clipped to a circle.
// The image is drawn centered at (cx, cy) and clipped by a circle with the given
// radius. The image is scaled to fit within the circle's bounding box.
func (c *Context) DrawImageCircular(img *ImageBuf, cx, cy, radius float64) {
	c.Push()
	c.DrawCircle(cx, cy, radius)
	c.Clip()

	// Draw image centered, scaled to fit the circle diameter.
	diameter := radius * 2
	c.DrawImageEx(img, DrawImageOptions{
		X:         cx - radius,
		Y:         cy - radius,
		DstWidth:  diameter,
		DstHeight: diameter,
		Opacity:   1.0,
		BlendMode: BlendNormal,
	})

	c.Pop()
}

// ExportImageBuf copies the current surface pixels into dst for DrawImage.
//
// Skia-class pattern for continuous effects: render+filter on a small offscreen
// Context, ExportImageBuf every frame, then DrawImage on the present path.
// Pixmap storage is premultiplied; dst is created/updated as FormatRGBAPremul
// so PremultipliedData does not double-multiply.
//
// Reuses *dst when size matches; always bumps generation id after copy.
func (c *Context) ExportImageBuf(dst **ImageBuf) bool {
	if c == nil || c.pixmap == nil || dst == nil {
		return false
	}
	_ = c.FlushGPU()
	_ = c.materializeFilterGPU()
	w, h := c.pixmap.Width(), c.pixmap.Height()
	if w <= 0 || h <= 0 {
		return false
	}
	needNew := *dst == nil
	if !needNew {
		dw, dh := (*dst).Bounds()
		needNew = dw != w || dh != h || (*dst).Format() != FormatRGBAPremul
	}
	if needNew {
		img, err := NewImageBuf(w, h, FormatRGBAPremul)
		if err != nil || img == nil {
			return false
		}
		*dst = img
	}
	src := c.pixmap.Data()
	out := (*dst).Data()
	n := len(out)
	if len(src) < n {
		n = len(src)
	}
	copy(out[:n], src[:n])
	// Reused ImageBuf: keep GenerationID and mark GPU dirty for in-place
	// reupload (continuous effect RT). Fresh buffers already have a gen from
	// NewImageBuf; MarkPixelsDirty still sets dirty so first Draw uploads.
	// NotifyPixelsChanged would allocate a new cache entry every export and
	// grow VRAM under particle/glow long soak.
	(*dst).MarkPixelsDirty()
	return true
}

// pixmapToImageBuf converts a Pixmap to an ImageBuf.
// This is a zero-copy operation that wraps the pixmap data.
func (c *Context) pixmapToImageBuf(pm *Pixmap) *ImageBuf {
	// Pixmap uses RGBA8 format
	stride := pm.Width() * 4
	img, _ := intImage.FromRaw(
		pm.Data(),
		pm.Width(),
		pm.Height(),
		intImage.FormatRGBA8,
		stride,
	)
	return img
}

// LoadImage loads an image from a file and returns an ImageBuf.
// Supported formats: PNG, JPEG, WebP.
func LoadImage(path string) (*ImageBuf, error) {
	return intImage.LoadImage(path)
}

// LoadWebP loads a WebP image from the given file path.
func LoadWebP(path string) (*ImageBuf, error) {
	return intImage.LoadWebP(path)
}

// NewImageBuf creates a new image buffer with the given dimensions and format.
func NewImageBuf(width, height int, format ImageFormat) (*ImageBuf, error) {
	return intImage.NewImageBuf(width, height, format)
}

// ImageBufFromImage creates an ImageBuf from a standard image.Image.
func ImageBufFromImage(img image.Image) *ImageBuf {
	return intImage.FromStdImage(img)
}

// DrawGPUTexture composites an existing GPU texture view as a textured quad
// at (x, y) with the given dimensions. No CPU readback or upload — pure
// GPU-to-GPU compositing. The view must be from the same device (e.g.,
// FlushGPUWithView output). CTM and scissor clip are applied.
//
// This is the Skia GrSurfaceProxyView direct-bind pattern for cached
// offscreen rendering (RepaintBoundary, layer compositing).
func (c *Context) DrawGPUTexture(view gpucontext.TextureView, x, y float64, width, height int) {
	rc := c.gpuCtxOps()
	if rc == nil || view.IsNil() {
		return
	}
	defer c.setGPUClipRect()()

	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	br := ctm.TransformPoint(Pt(x+float64(width), y+float64(height)))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
	vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32

	rc.QueueGPUTextureDraw(target, view,
		float32(tl.X), float32(tl.Y), float32(br.X-tl.X), float32(br.Y-tl.Y),
		1.0, vpW, vpH)
	c.recordGPUOp()
}

// DrawGPUTextureWithOpacity composites a GPU texture view as an overlay with
// the specified opacity (0.0 = fully transparent, 1.0 = fully opaque).
// Same as DrawGPUTexture but with alpha blending for fade transitions
// and OpacityLayer compositing (Flutter pattern).
func (c *Context) DrawGPUTextureWithOpacity(view gpucontext.TextureView, x, y float64, width, height int, opacity float32) {
	rc := c.gpuCtxOps()
	if rc == nil || view.IsNil() {
		return
	}
	defer c.setGPUClipRect()()

	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	br := ctm.TransformPoint(Pt(x+float64(width), y+float64(height)))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
	vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32

	rc.QueueGPUTextureDraw(target, view,
		float32(tl.X), float32(tl.Y), float32(br.X-tl.X), float32(br.Y-tl.Y),
		opacity, vpW, vpH)
	c.recordGPUOp()
}

// DrawGPUTextureWithOpacityUV composites a sub-rectangle of a GPU texture with
// opacity. u0..v1 are normalized source UVs (F1 damage-tight layer composite).
func (c *Context) DrawGPUTextureWithOpacityUV(view gpucontext.TextureView, x, y float64, width, height int, opacity float32, u0, v0, u1, v1 float32) {
	rc := c.gpuCtxOps()
	if rc == nil || view.IsNil() {
		return
	}
	defer c.setGPUClipRect()()

	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	br := ctm.TransformPoint(Pt(x+float64(width), y+float64(height)))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec
	vpH := uint32(target.Height) //nolint:gosec

	type uvDrawer interface {
		QueueGPUTextureDrawUV(target GPURenderTarget, view gpucontext.TextureView,
			dstX, dstY, dstW, dstH, opacity float32, vpW, vpH uint32,
			u0, v0, u1, v1 float32)
	}
	if ud, ok := rc.(uvDrawer); ok {
		ud.QueueGPUTextureDrawUV(target, view,
			float32(tl.X), float32(tl.Y), float32(br.X-tl.X), float32(br.Y-tl.Y),
			opacity, vpW, vpH, u0, v0, u1, v1)
	} else {
		rc.QueueGPUTextureDraw(target, view,
			float32(tl.X), float32(tl.Y), float32(br.X-tl.X), float32(br.Y-tl.Y),
			opacity, vpW, vpH)
	}
	c.recordGPUOp()
}

// DrawGPUTextureBase composites a GPU texture view as the compositor base layer.
// The base layer is drawn BEFORE all GPU tiers (SDF, convex, stencil, text) in
// the render pass, making it the background for zero-readback rendering.
//
// Use this to composite a CPU pixmap texture as the background, with GPU shapes
// rendered on top in the same render pass. Last call per frame wins.
//
// See ADR-015 (Compositor Base Layer), Flutter OffsetLayer pattern.
func (c *Context) DrawGPUTextureBase(view gpucontext.TextureView, x, y float64, width, height int) {
	rc := c.gpuCtxOps()
	if rc == nil || view.IsNil() {
		return
	}

	ctm := c.totalMatrix()
	tl := ctm.TransformPoint(Pt(x, y))
	br := ctm.TransformPoint(Pt(x+float64(width), y+float64(height)))

	target := c.gpuRenderTarget()
	vpW := uint32(target.Width)  //nolint:gosec // viewport fits uint32
	vpH := uint32(target.Height) //nolint:gosec // viewport fits uint32

	rc.QueueBaseLayer(target, view,
		float32(tl.X), float32(tl.Y), float32(br.X-tl.X), float32(br.Y-tl.Y),
		1.0, vpW, vpH)
	c.recordGPUOp()
}

// CreateOffscreenTexture allocates a GPU texture for offscreen rendering.
// The texture can be rendered to via FlushGPUWithView and composited via
// DrawGPUTexture. Returns (nil, nil) if GPU is not available.
//
// The returned TextureView is valid until release() is called.
// Call release() to return the texture resources to the GPU.
//
// Usage flags: RenderAttachment | CopySrc | TextureBinding.
func (c *Context) CreateOffscreenTexture(width, height int) (gpucontext.TextureView, func()) {
	rc := c.gpuCtxOps()
	if rc == nil {
		return gpucontext.TextureView{}, nil
	}
	type offscreenCreator interface {
		CreateOffscreenTexture(w, h int) (gpucontext.TextureView, func())
	}
	if oc, ok := rc.(offscreenCreator); ok {
		return oc.CreateOffscreenTexture(width, height)
	}
	return gpucontext.TextureView{}, nil
}
