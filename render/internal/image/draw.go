// Package image provides image buffer management for gogpu/gg.
package image

import "math"

// Rect represents a rectangular region in pixel coordinates.
type Rect struct {
	X, Y          int // Top-left corner
	Width, Height int // Dimensions
}

// BlendMode defines how source pixels are blended with destination pixels.
type BlendMode uint8

const (
	// BlendNormal performs standard alpha blending (source over destination).
	BlendNormal BlendMode = iota

	// BlendMultiply multiplies source and destination colors.
	// Result is always darker or equal. Formula: dst * src
	BlendMultiply

	// BlendScreen performs inverse multiply for lighter results.
	// Formula: 1 - (1-dst) * (1-src)
	BlendScreen

	// BlendOverlay combines multiply and screen based on destination brightness.
	// Dark areas are multiplied, bright areas are screened.
	BlendOverlay

	// BlendHue (non-separable): hue of source, saturation/luminosity of destination.
	BlendHue
	// BlendSaturation (non-separable): saturation of source, hue/luminosity of destination.
	BlendSaturation
	// BlendColor (non-separable): hue+saturation of source, luminosity of destination.
	BlendColor
	// BlendLuminosity (non-separable): luminosity of source, hue+saturation of destination.
	BlendLuminosity

	// Porter-Duff fixed-function modes (B.02). Values continue after HSL modes.
	// BlendClear: result is transparent black (0).
	BlendClear
	// BlendCopy: result is source (Src / Replace).
	BlendCopy
	// BlendPlus: result is clamped source+destination (additive).
	BlendPlus
	// BlendDestinationOut: dst * (1 - srcA) — eraser / cutout.
	BlendDestinationOut
	// BlendSourceAtop: src*dstA + dst*(1-srcA).
	BlendSourceAtop
	// BlendXor: src*(1-dstA) + dst*(1-srcA).
	BlendXor
	// BlendDestinationOver: src*(1-dstA) + dst (DstOver).
	BlendDestinationOver
	// BlendSourceIn: src * dstA (SrcIn).
	BlendSourceIn
	// BlendSourceOut: src * (1-dstA) (SrcOut).
	BlendSourceOut
	// BlendDestinationIn: dst * srcA (DstIn).
	BlendDestinationIn
	// BlendDestinationAtop: src*(1-dstA) + dst*srcA (DstAtop).
	BlendDestinationAtop

	// Separable advanced modes (B.03 extended). Appended after Porter-Duff so
	// existing iota values stay stable.
	// BlendDarken selects the darker of source and destination per channel.
	BlendDarken
	// BlendLighten selects the lighter of source and destination per channel.
	BlendLighten
	// BlendColorDodge brightens destination to reflect source.
	BlendColorDodge
	// BlendColorBurn darkens destination to reflect source.
	BlendColorBurn
	// BlendHardLight is Multiply or Screen depending on source.
	BlendHardLight
	// BlendSoftLight is a softer HardLight variant.
	BlendSoftLight
	// BlendDifference is absolute difference of channels.
	BlendDifference
	// BlendExclusion is a lower-contrast difference.
	BlendExclusion
)

const unknownBlendMode = "Unknown"

// String returns a string representation of the blend mode.
func (b BlendMode) String() string {
	switch b {
	case BlendNormal:
		return "Normal"
	case BlendMultiply:
		return "Multiply"
	case BlendScreen:
		return "Screen"
	case BlendOverlay:
		return "Overlay"
	case BlendHue:
		return "Hue"
	case BlendSaturation:
		return "Saturation"
	case BlendColor:
		return "Color"
	case BlendLuminosity:
		return "Luminosity"
	case BlendClear:
		return "Clear"
	case BlendCopy:
		return "Copy"
	case BlendPlus:
		return "Plus"
	case BlendDestinationOut:
		return "DestinationOut"
	case BlendSourceAtop:
		return "SourceAtop"
	case BlendXor:
		return "Xor"
	case BlendDestinationOver:
		return "DestinationOver"
	case BlendSourceIn:
		return "SourceIn"
	case BlendSourceOut:
		return "SourceOut"
	case BlendDestinationIn:
		return "DestinationIn"
	case BlendDestinationAtop:
		return "DestinationAtop"
	case BlendDarken:
		return "Darken"
	case BlendLighten:
		return "Lighten"
	case BlendColorDodge:
		return "ColorDodge"
	case BlendColorBurn:
		return "ColorBurn"
	case BlendHardLight:
		return "HardLight"
	case BlendSoftLight:
		return "SoftLight"
	case BlendDifference:
		return "Difference"
	case BlendExclusion:
		return "Exclusion"
	default:
		return unknownBlendMode
	}
}

// DrawParams specifies parameters for the DrawImage operation.
type DrawParams struct {
	// SrcRect defines the source rectangle to sample from.
	// If nil, the entire source image is used.
	SrcRect *Rect

	// DstRect defines the destination rectangle to draw into.
	DstRect Rect

	// Transform is an optional affine transformation applied to source coordinates.
	// If nil, identity transform is used.
	Transform *Affine

	// Interp specifies the interpolation mode for sampling.
	Interp InterpolationMode

	// Opacity controls the overall transparency of the source image (0.0 to 1.0).
	// 1.0 means fully opaque, 0.0 means fully transparent.
	Opacity float64

	// BlendMode specifies how to blend source and destination pixels.
	BlendMode BlendMode

	// UseMipmaps selects a prefiltered mip level when drawing smaller than 1:1 (I.04).
	UseMipmaps bool
}

// DrawImage draws the source image onto the destination image using the specified parameters.
//
// The operation performs the following steps:
//  1. For each pixel in the destination rectangle
//  2. Apply inverse transformation to find source coordinates
//  3. Sample source image using specified interpolation
//  4. Apply opacity to the sampled color
//  5. Blend with destination using specified blend mode
//
// The destination image is modified in place.
func DrawImage(dst, src *ImageBuf, params DrawParams) {
	// Use entire source if no source rect specified
	srcRect := params.SrcRect
	if srcRect == nil {
		w, h := src.Bounds()
		srcRect = &Rect{X: 0, Y: 0, Width: w, Height: h}
	}

	// Use identity transform if none specified
	transform := params.Transform
	if transform == nil {
		identity := Identity()
		transform = &identity
	}

	// Compute inverse transform for mapping dst -> src
	invTransform, ok := transform.Invert()
	if !ok {
		// Singular matrix, cannot draw
		return
	}

	// Optional mipmap level selection when downscaling.
	if params.UseMipmaps && srcRect.Width > 0 && srcRect.Height > 0 {
		scaleX := float64(params.DstRect.Width) / float64(srcRect.Width)
		scaleY := float64(params.DstRect.Height) / float64(srcRect.Height)
		scale := scaleX
		if scaleY < scale {
			scale = scaleY
		}
		if scale > 0 && scale < 0.99 {
			if chain := GenerateMipmaps(src); chain != nil {
				if lvl := chain.LevelForScale(scale); lvl != nil && lvl != src {
					// Map srcRect into the mip level coordinate space.
					sw0, sh0 := src.Bounds()
					lw, lh := lvl.Bounds()
					sx := float64(lw) / float64(sw0)
					sy := float64(lh) / float64(sh0)
					src = lvl
					srcRect = &Rect{
						X:      int(float64(srcRect.X) * sx),
						Y:      int(float64(srcRect.Y) * sy),
						Width:  max(1, int(float64(srcRect.Width)*sx+0.5)),
						Height: max(1, int(float64(srcRect.Height)*sy+0.5)),
					}
				}
			}
		}
	}

	// Clamp opacity to valid range
	opacity := math.Max(0.0, math.Min(1.0, params.Opacity))

	// Get destination bounds
	dstWidth, dstHeight := dst.Bounds()

	// Clamp destination rectangle to image bounds
	dstRect := params.DstRect
	if dstRect.X < 0 {
		dstRect.Width += dstRect.X
		dstRect.X = 0
	}
	if dstRect.Y < 0 {
		dstRect.Height += dstRect.Y
		dstRect.Y = 0
	}
	if dstRect.X+dstRect.Width > dstWidth {
		dstRect.Width = dstWidth - dstRect.X
	}
	if dstRect.Y+dstRect.Height > dstHeight {
		dstRect.Height = dstHeight - dstRect.Y
	}

	// Nothing to draw if clamped rectangle is empty
	if dstRect.Width <= 0 || dstRect.Height <= 0 {
		return
	}

	// Draw each pixel in the destination rectangle
	for dy := 0; dy < dstRect.Height; dy++ {
		for dx := 0; dx < dstRect.Width; dx++ {
			// Destination pixel coordinates (absolute in destination image)
			dstX := dstRect.X + dx
			dstY := dstRect.Y + dy

			// Normalized position within destination rectangle [0, 1]
			// Add 0.5 to sample from pixel centers
			u := (float64(dx) + 0.5) / float64(dstRect.Width)
			v := (float64(dy) + 0.5) / float64(dstRect.Height)

			// Apply inverse transform to find where this maps in source space
			// The transform is meant to map from destination rect coords to source rect coords
			srcRelX, srcRelY := invTransform.TransformPoint(u*float64(dstRect.Width), v*float64(dstRect.Height))

			// Map to source rect space
			srcX := float64(srcRect.X) + srcRelX
			srcY := float64(srcRect.Y) + srcRelY

			// Check if we're outside the source rectangle
			if srcX < float64(srcRect.X) || srcX > float64(srcRect.X+srcRect.Width) ||
				srcY < float64(srcRect.Y) || srcY > float64(srcRect.Y+srcRect.Height) {
				continue
			}

			// Convert to normalized coordinates for the entire source image [0, 1]
			srcWidth, srcHeight := src.Bounds()
			sampleU := srcX / float64(srcWidth)
			sampleV := srcY / float64(srcHeight)

			// Sample source image
			srcR, srcG, srcB, srcA := Sample(src, sampleU, sampleV, params.Interp)

			// Apply opacity
			if opacity < 1.0 {
				srcA = uint8(float64(srcA) * opacity)
			}

			// Get destination pixel
			dstR, dstG, dstB, dstA := dst.GetRGBA(dstX, dstY)

			// Blend and write result
			r, g, b, a := blend(srcR, srcG, srcB, srcA, dstR, dstG, dstB, dstA, params.BlendMode)
			_ = dst.SetRGBA(dstX, dstY, r, g, b, a)
		}
	}
}

// blend blends source and destination colors using the specified blend mode.
func blend(srcR, srcG, srcB, srcA, dstR, dstG, dstB, dstA uint8, mode BlendMode) (r, g, b, a byte) {
	if srcA == 0 {
		// Fully transparent source, return destination unchanged
		return dstR, dstG, dstB, dstA
	}

	if mode == BlendNormal {
		// Standard alpha blending (source over destination)
		return blendNormal(srcR, srcG, srcB, srcA, dstR, dstG, dstB, dstA)
	}

	// For other blend modes, first blend the colors, then apply alpha
	var blendedR, blendedG, blendedB uint8

	switch mode {
	case BlendMultiply:
		blendedR, blendedG, blendedB = blendMultiply(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendScreen:
		blendedR, blendedG, blendedB = blendScreen(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendOverlay:
		blendedR, blendedG, blendedB = blendOverlay(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendHue:
		blendedR, blendedG, blendedB = blendHue(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendSaturation:
		blendedR, blendedG, blendedB = blendSaturation(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendColor:
		blendedR, blendedG, blendedB = blendColor(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendLuminosity:
		blendedR, blendedG, blendedB = blendLuminosity(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendDarken:
		blendedR, blendedG, blendedB = blendDarken(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendLighten:
		blendedR, blendedG, blendedB = blendLighten(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendColorDodge:
		blendedR, blendedG, blendedB = blendColorDodge(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendColorBurn:
		blendedR, blendedG, blendedB = blendColorBurn(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendHardLight:
		blendedR, blendedG, blendedB = blendHardLight(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendSoftLight:
		blendedR, blendedG, blendedB = blendSoftLight(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendDifference:
		blendedR, blendedG, blendedB = blendDifference(srcR, srcG, srcB, dstR, dstG, dstB)
	case BlendExclusion:
		blendedR, blendedG, blendedB = blendExclusion(srcR, srcG, srcB, dstR, dstG, dstB)
	default:
		blendedR, blendedG, blendedB = srcR, srcG, srcB
	}

	// Apply alpha blending to the blended result
	return blendNormal(blendedR, blendedG, blendedB, srcA, dstR, dstG, dstB, dstA)
}

// blendNormal performs standard alpha blending (source over destination).
func blendNormal(srcR, srcG, srcB, srcA, dstR, dstG, dstB, dstA uint8) (r, g, b, a byte) {
	if srcA == 255 {
		// Fully opaque source, just return source
		return srcR, srcG, srcB, 255
	}

	if dstA == 0 {
		// Transparent destination, just return source
		return srcR, srcG, srcB, srcA
	}

	// Porter-Duff "source over" formula
	// out_a = src_a + dst_a * (1 - src_a)
	// out_c = (src_c * src_a + dst_c * dst_a * (1 - src_a)) / out_a

	srcAlpha := float64(srcA) / 255.0
	dstAlpha := float64(dstA) / 255.0

	outAlpha := srcAlpha + dstAlpha*(1-srcAlpha)

	if outAlpha == 0 {
		return 0, 0, 0, 0
	}

	r = uint8((float64(srcR)*srcAlpha + float64(dstR)*dstAlpha*(1-srcAlpha)) / outAlpha)
	g = uint8((float64(srcG)*srcAlpha + float64(dstG)*dstAlpha*(1-srcAlpha)) / outAlpha)
	b = uint8((float64(srcB)*srcAlpha + float64(dstB)*dstAlpha*(1-srcAlpha)) / outAlpha)
	a = uint8(outAlpha * 255.0)

	return r, g, b, a
}

// blendMultiply multiplies source and destination colors.
func blendMultiply(srcR, srcG, srcB, dstR, dstG, dstB uint8) (r, g, b byte) {
	r = uint8((int(srcR) * int(dstR)) / 255)
	g = uint8((int(srcG) * int(dstG)) / 255)
	b = uint8((int(srcB) * int(dstB)) / 255)
	return r, g, b
}

// blendScreen performs screen blending for lighter results.
func blendScreen(srcR, srcG, srcB, dstR, dstG, dstB uint8) (r, g, b byte) {
	// Formula: 1 - (1-src) * (1-dst) = src + dst - src*dst
	r = uint8(255 - (255-int(srcR))*(255-int(dstR))/255)
	g = uint8(255 - (255-int(srcG))*(255-int(dstG))/255)
	b = uint8(255 - (255-int(srcB))*(255-int(dstB))/255)
	return r, g, b
}

// blendOverlay combines multiply and screen based on destination brightness.
func blendOverlay(srcR, srcG, srcB, dstR, dstG, dstB uint8) (r, g, b byte) {
	r = overlayChannel(srcR, dstR)
	g = overlayChannel(srcG, dstG)
	b = overlayChannel(srcB, dstB)
	return r, g, b
}

// overlayChannel applies overlay blending to a single channel.
func overlayChannel(src, dst uint8) uint8 {
	// If dst < 0.5: 2 * src * dst
	// Else: 1 - 2 * (1-src) * (1-dst)
	if dst < 128 {
		return uint8((2 * int(src) * int(dst)) / 255)
	}
	return uint8(255 - (2*(255-int(src))*(255-int(dst)))/255)
}

// --- Non-separable HSL-style blends (CSS Compositing / PDF) ---

func lumU8(r, g, b uint8) float64 {
	return 0.3*float64(r) + 0.59*float64(g) + 0.11*float64(b)
}

func satU8(r, g, b uint8) float64 {
	maxv := float64(r)
	if float64(g) > maxv {
		maxv = float64(g)
	}
	if float64(b) > maxv {
		maxv = float64(b)
	}
	minv := float64(r)
	if float64(g) < minv {
		minv = float64(g)
	}
	if float64(b) < minv {
		minv = float64(b)
	}
	return maxv - minv
}

func clipColor(r, g, b float64) (uint8, uint8, uint8) {
	l := 0.3*r + 0.59*g + 0.11*b
	n := r
	if g < n {
		n = g
	}
	if b < n {
		n = b
	}
	x := r
	if g > x {
		x = g
	}
	if b > x {
		x = b
	}
	if n < 0 {
		r = l + (r-l)*l/(l-n)
		g = l + (g-l)*l/(l-n)
		b = l + (b-l)*l/(l-n)
	}
	if x > 255 {
		r = l + (r-l)*(255-l)/(x-l)
		g = l + (g-l)*(255-l)/(x-l)
		b = l + (b-l)*(255-l)/(x-l)
	}
	return clampU8(r), clampU8(g), clampU8(b)
}

func clampU8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v + 0.5)
}

func setLum(r, g, b, l float64) (uint8, uint8, uint8) {
	d := l - (0.3*r + 0.59*g + 0.11*b)
	return clipColor(r+d, g+d, b+d)
}

func setSat(r, g, b, s float64) (float64, float64, float64) {
	// Sort channels
	type ch struct {
		v float64
		i int
	}
	cs := [3]ch{{r, 0}, {g, 1}, {b, 2}}
	// bubble sort by value
	if cs[0].v > cs[1].v {
		cs[0], cs[1] = cs[1], cs[0]
	}
	if cs[1].v > cs[2].v {
		cs[1], cs[2] = cs[2], cs[1]
	}
	if cs[0].v > cs[1].v {
		cs[0], cs[1] = cs[1], cs[0]
	}
	minc, midc, maxc := cs[0], cs[1], cs[2]
	if maxc.v > minc.v {
		midc.v = ((midc.v - minc.v) * s) / (maxc.v - minc.v)
		maxc.v = s
	} else {
		midc.v = 0
		maxc.v = 0
	}
	minc.v = 0
	out := [3]float64{}
	out[minc.i] = minc.v
	out[midc.i] = midc.v
	out[maxc.i] = maxc.v
	return out[0], out[1], out[2]
}

// blendHue: hue of source, sat/lum of destination.
func blendHue(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	r, g, b := setSat(float64(sr), float64(sg), float64(sb), satU8(dr, dg, db))
	return setLum(r, g, b, lumU8(dr, dg, db))
}

// blendSaturation: sat of source, hue/lum of destination.
func blendSaturation(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	r, g, b := setSat(float64(dr), float64(dg), float64(db), satU8(sr, sg, sb))
	return setLum(r, g, b, lumU8(dr, dg, db))
}

// blendColor: hue+sat of source, lum of destination.
func blendColor(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return setLum(float64(sr), float64(sg), float64(sb), lumU8(dr, dg, db))
}

// blendLuminosity: lum of source, hue+sat of destination.
func blendLuminosity(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return setLum(float64(dr), float64(dg), float64(db), lumU8(sr, sg, sb))
}

// --- Separable advanced helpers (straight 0-255 channels) ---

func minU8(a, b uint8) uint8 {
	if a < b {
		return a
	}
	return b
}
func maxU8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}
func absDiffU8(a, b uint8) uint8 {
	if a >= b {
		return a - b
	}
	return b - a
}

func blendDarken(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return minU8(sr, dr), minU8(sg, dg), minU8(sb, db)
}
func blendLighten(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return maxU8(sr, dr), maxU8(sg, dg), maxU8(sb, db)
}
func blendColorDodgeChan(s, d uint8) uint8 {
	if s >= 255 {
		return 255
	}
	// min(1, d/(1-s))
	inv := 255 - int(s)
	v := (int(d) * 255) / inv
	if v > 255 {
		return 255
	}
	return uint8(v)
}
func blendColorDodge(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return blendColorDodgeChan(sr, dr), blendColorDodgeChan(sg, dg), blendColorDodgeChan(sb, db)
}
func blendColorBurnChan(s, d uint8) uint8 {
	if s == 0 {
		return 0
	}
	// max(0, 1 - (1-d)/s)
	v := 255 - ((255-int(d))*255)/int(s)
	if v < 0 {
		return 0
	}
	return uint8(v)
}
func blendColorBurn(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return blendColorBurnChan(sr, dr), blendColorBurnChan(sg, dg), blendColorBurnChan(sb, db)
}
func blendHardLightChan(s, d uint8) uint8 {
	// HardLight = Overlay with src/dst swapped roles (decision on source)
	if s <= 127 {
		return uint8((2 * int(s) * int(d)) / 255)
	}
	return uint8(255 - (2*(255-int(s))*(255-int(d)))/255)
}
func blendHardLight(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return blendHardLightChan(sr, dr), blendHardLightChan(sg, dg), blendHardLightChan(sb, db)
}
func blendSoftLightChan(s, d uint8) uint8 {
	// W3C soft-light (approx with 0-255 ints)
	sf := float64(s) / 255.0
	df := float64(d) / 255.0
	var r float64
	if sf <= 0.5 {
		r = df - (1.0-2.0*sf)*df*(1.0-df)
	} else {
		var d2 float64
		if df <= 0.25 {
			d2 = ((16.0*df-12.0)*df + 4.0) * df
		} else {
			d2 = math.Sqrt(df)
		}
		r = df + (2.0*sf-1.0)*(d2-df)
	}
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	return uint8(r * 255.0)
}
func blendSoftLight(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return blendSoftLightChan(sr, dr), blendSoftLightChan(sg, dg), blendSoftLightChan(sb, db)
}
func blendDifference(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	return absDiffU8(sr, dr), absDiffU8(sg, dg), absDiffU8(sb, db)
}
func blendExclusion(sr, sg, sb, dr, dg, db uint8) (uint8, uint8, uint8) {
	// s+d-2*s*d
	ex := func(s, d uint8) uint8 {
		return uint8(int(s) + int(d) - (2*int(s)*int(d))/255)
	}
	return ex(sr, dr), ex(sg, dg), ex(sb, db)
}
