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
//	img, _ := render.LoadImage("photo.png")
//	dc.DrawImage(img, 100, 100)
//
// DrawImage draws an image at the specified position.
// The current transformation matrix is applied to the position and size.
//
// Example:
//
//	img, _ := render.LoadImage("photo.png")
//	dc.DrawImage(img, 100, 100)
func (c *Context) DrawImage(img *ImageBuf, x, y float64) {
	c.syncPublishedFilterBeforeDraw()
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
//	dc.DrawImageEx(img, render.DrawImageOptions{
//	    X: 100,
//	    Y: 100,
//	    DstWidth: 200,
//	    DstHeight: 150,
//	    Interpolation: render.InterpBicubic,
//	    Opacity: 0.8,
//	    BlendMode: render.BlendNormal,
//	})
//
// DrawImageEx draws an image with advanced options.
// The current transformation matrix is applied to the position and size.
// The image is drawn through the Fill() pipeline, which means it respects
// the current clip region. This follows the enterprise pattern used by
// Skia, Cairo, tiny-skia, and Vello: image drawing = fillRect + image shader.
//
// Example:
//
//	dc.DrawImageEx(img, render.DrawImageOptions{
//	    X: 100,
//	    Y: 100,
//	    DstWidth: 200,
//	    DstHeight: 150,
//	    Interpolation: render.InterpBicubic,
//	    Opacity: 0.8,
//	    BlendMode: render.BlendNormal,
//	})
func (c *Context) DrawImageEx(img *ImageBuf, opts DrawImageOptions) {
	c.syncPublishedFilterBeforeDraw()
	if img == nil || img.Disposed() {
		return
	}
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

	// I.04 R1: Prefer GPU textured quads. Bicubic uses a GPU 4x4 convolution variant
	// (textured_quad_bicubic.wgsl, Catmull-Rom kernel matching the CPU path);
	// GPU failure falls back to the CPU bicubic samplers below.
	if c.tryGPUDrawImage(img, opts, srcX, srcY, srcW, srcH, dstWidth, dstHeight) {
		// The textured-quad path queues the draw directly and returns without
		// going through Fill() — Fill() is where CPU image draws register their
		// damage (context_draw.go: c.trackDamage(c.path.Bounds())). Register the
		// equivalent CTM-mapped AABB here, otherwise incremental present never
		// marks the GPU-drawn region dirty (video bridge / direct frame draws
		// reported "left no damage" with a GPU session active).
		ctm := c.totalMatrix()
		corners := [4]Point{
			ctm.TransformPoint(Pt(opts.X, opts.Y)),
			ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y)),
			ctm.TransformPoint(Pt(opts.X+dstWidth, opts.Y+dstHeight)),
			ctm.TransformPoint(Pt(opts.X, opts.Y+dstHeight)),
		}
		minX, minY := corners[0].X, corners[0].Y
		maxX, maxY := minX, minY
		for _, p := range corners[1:] {
			minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
			minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
		}
		c.trackDamage(image.Rect(
			int(math.Floor(minX)), int(math.Floor(minY)),
			int(math.Ceil(maxX)), int(math.Ceil(maxY)),
		))
		return
	}
	// P3.7 批 3b：材质无 CPU 求值器（P3.2 契约）——GPU 路没接走的材质绘制在此
	// 统一拒绝（覆盖投影层/blend/bicubic/CPU 模式全部分支）：跳过 + MaterialSkips
	// 计数，绝不落到任何 CPU 腿裸画无材质画面（skip ≠ fallback，不记 CPUFallbackOps）。
	if opts.Material != nil {
		c.pathStats.MaterialSkips++
		return
	}
	// P3.7 批 1d：投影层生效 ⇒ CPU 兜底也走透视 CPU 腿（drawImageQuadCPUSplit
	// 已有任意四角+权重的 1/w 有理插值），模式=ImagePattern 路在投影下无法表达
	//（pattern 逆变换是 2×3）。blend 非 Normal 走老 pattern 路 + 计数（投影下
	// 相位近似，登记）；bicubic 与 GPU 透视槽同口径降级 bilinear。
	if c.projActive() {
		dev, w, ok := c.projMapQuad([4]Point{{X: opts.X, Y: opts.Y}, {X: opts.X + dstWidth, Y: opts.Y},
			{X: opts.X + dstWidth, Y: opts.Y + dstHeight}, {X: opts.X, Y: opts.Y + dstHeight}})
		if !ok {
			// 眼后 ⇒ 整块不画（与 quad 口 ErrQuadBehindEye、图集/网腿「眼后跳过」
			// 同一条规则）。实测教训：落到下面 pattern 路会把未投影画面画出来
			// （探针 P2：眼后矩形画出 1762 像素）。
			return
		}
		{
			interp := opts.Interpolation
			if interp == InterpBicubic {
				// 透视 CPU 腿无 bicubic 采样核 ⇒ 降 bilinear（与 GPU 透视槽同口径）
				// + 具名计数。
				interp = InterpBilinear
				c.projInterpDegrades++
			}
			if opts.BlendMode == BlendNormal {
				c.drawImageQuadCPUSplit(img, dev, interp, opts.Opacity, true, w, RGBA{})
				return
			}
			c.projPatternDegrades++
		}
		// blend 非 Normal 的近似：继续走既有 pattern 路（几何仍按投影映射，
		// 只有图样采样是 2×3）+ 具名计数。
	}
	if opts.Interpolation == InterpBicubic {
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
					SrcRect:     srcRect,
					DstRect:     intImage.Rect{X: dx, Y: dy, Width: dw, Height: dh},
					Interp:      opts.Interpolation,
					Opacity:     opts.Opacity,
					BlendMode:   opts.BlendMode,
					BlendLinear: opts.BlendSpace.linear(),
					Mipmap:      opts.Mipmap,
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

	// Compose the pattern's forward transform in drawing space: image -> user.
	//
	//   1. Scale source pixels to destination size: Scale(scaleX, scaleY)
	//   2. Position at destination: Translate(opts.X, opts.Y)
	//
	// The canvas CTM is NOT composed here. Patterns are user-space brushes like
	// gradients; the fill pipeline maps device samples back through the total
	// matrix (Context.applyDeviceSpaceBrush). Baking it here too would apply the
	// transform twice once the CTM is non-identity.
	patternTransform := Translate(opts.X, opts.Y).
		Multiply(Scale(scaleX, scaleY))
	inverse := patternTransform.Invert()

	// Create image pattern with pre-computed inverse transform.
	pattern := &ImagePattern{
		image:    img,
		x:        srcX,
		y:        srcY,
		w:        srcW,
		h:        srcH,
		inverse:  inverse,
		opacity:  opts.Opacity,
		clamp:    true,                                // Don't tile — clamp to image bounds
		bilinear: opts.Interpolation != InterpNearest, // batch②：bilinear/默认真正双线性（nearest 才走整数取样）
	}

	// Save current state (paint, path, transform). Push/Pop covers path and
	// transform; paint must be frozen explicitly (深审 D6 / L1-P1-3): the
	// one-shot BlendMode/BlendSpace/image-pattern used to leak to the caller.
	c.Push()
	c.withPaintFrozen(func() {
		// Honor the requested compositing operator: SetFillPattern + Fill applies
		// c.blendMode through the software/GPU brush paths (B.02 fixed-function
		// Porter-Duff modes included). Previously the option was dropped here, so
		// e.g. DestinationIn image draws composited as SourceOver.
		c.SetBlendMode(opts.BlendMode)
		c.SetBlendSpace(opts.BlendSpace)

		// Set image pattern as fill source.
		c.SetFillPattern(pattern)

		// Draw rectangle at destination using the current transform.
		// DrawRectangle applies the transform, which is what we want.
		c.DrawRectangle(opts.X, opts.Y, dstWidth, dstHeight)

		// Fill the rectangle — the Fill() pipeline handles clipping automatically.
		_ = c.Fill()
	})

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
