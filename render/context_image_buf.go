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

	intImage "github.com/energye/gpui/render/internal/image"
)

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
	// only FlushGPU when there are pending GPU draws. When the surface
	// is already GPU-filter-published (pixmapFilterStale), FlushGPU is a no-op
	// for content and only adds queue overhead.
	pending := 0
	if rc := c.gpuCtxOps(); rc != nil {
		type pcounter interface{ PendingCount() int }
		if pc, ok := rc.(pcounter); ok {
			pending = pc.PendingCount()
		}
	}
	if pending > 0 {
		_ = c.FlushGPU()
	}
	_ = c.syncViewFlushIntoPixmap()

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
	out := (*dst).Data()
	n := len(out)

	// when filter result lives only on GPU, readback once into ImageBuf
	// and refresh pixmap in the same pass (avoid Flush+materialize+copy triple).
	if c.pixmapFilterStale && !c.filterGPUView.IsNil() && c.filterGPUW == w && c.filterGPUH == h {
		if c.materializeFilterGPUTo(out, c.pixmap.Data()) {
			(*dst).MarkPixelsDirty()
			return true
		}
	} else {
		_ = c.materializeFilterGPU()
	}

	src := c.pixmap.Data()
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
// LoadImage loads an image from a file and returns an ImageBuf.
// Supported formats: PNG, JPEG, WebP.
func LoadImage(path string) (*ImageBuf, error) {
	return intImage.LoadImage(path)
}

// LoadWebP loads a WebP image from the given file path.
// LoadWebP loads a WebP image from the given file path.
func LoadWebP(path string) (*ImageBuf, error) {
	return intImage.LoadWebP(path)
}

// NewImageBuf creates a new image buffer with the given dimensions and format.
// NewImageBuf creates a new image buffer with the given dimensions and format.
func NewImageBuf(width, height int, format ImageFormat) (*ImageBuf, error) {
	return intImage.NewImageBuf(width, height, format)
}

// ImageBufFromImage creates an ImageBuf from a standard image.Image.
// ImageBufFromImage creates an ImageBuf from a standard image.Image.
func ImageBufFromImage(img image.Image) *ImageBuf {
	return intImage.FromStdImage(img)
}

// gpuTextureDrawGeom holds CTM-mapped destination rect and viewport for
// DrawGPUTexture* entry points (shared setup, no behavior change).
type gpuTextureDrawGeom struct {
	rc         gpuContextOps
	target     GPURenderTarget
	dstX, dstY float32
	dstW, dstH float32
	tlX, tlY   float32
	trX, trY   float32
	brX, brY   float32
	blX, blY   float32
	vpW, vpH   uint32
}

func min4(a, b, c, d float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	if d < m {
		m = d
	}
	return m
}

func max4(a, b, c, d float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	if d > m {
		m = d
	}
	return m
}

// prepareGPUTextureDraw shares sync/validate/CTM/viewport setup for DrawGPUTexture*.
// Returns ok=false when GPU is unavailable or view is nil.
