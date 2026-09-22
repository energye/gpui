package render

import (
	"bytes"
	"fmt"
	"image"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// Retained-record CPU scratch swap.
//
// Background: retained texture sub-passes (ui/scene PictureTextureCache
// recordWith/recordLocalWith) replay a display list plus a RenderBox OnPaint
// callback into a small offscreen GPU view, then FlushGPUWithView resolves
// the queued GPU commands into that view. Any draw that takes the CPU
// fallback inside the pass (tinted atlas sprites, which have no GPU tint
// shader, and friends) writes into the Context pixmap instead — the MAIN
// surface pixmap, at pass-local coordinates. Two things then go wrong:
// the texture misses the CPU pixels (black cards), and the main pixmap is
// corrupted at its top-left (later covered by compositing, but wrong).
// Worse, the CPU fallback first calls flushGPUAccelerator, which flushes
// the already-queued GPU commands to the MAIN surface as well, so even the
// GPU-drawn siblings drawn before the CPU draw never reach the texture.
// Observed: game_particle fire/smoke cards fully black, game_sprite ROT
// card showing only the GPU-stroked red line over black.
//
// Native alignment: Skia resolves every flush to the CURRENT render target
// (GrRenderTargetContext ownership per surface); a mid-pass flush never
// escapes to the parent surface. This file gives the pass the same
// property without changing flush routing:
//
//  1. The record site swaps the Context pixmap with a pooled full-surface
//     transparent scratch (BeginPassScratch). CPU fallbacks then land in
//     the scratch at pass coordinates; mid-pass GPU flushes read back
//     into the scratch too, so the scratch accumulates the pass content
//     in exact z-order (GPU-via-readback plus CPU-direct).
//  2. Before the pass-closing FlushGPUWithView, the record site commits
//     the scratch region into the view (CommitPassScratchToView): raw
//     bytes go via WriteTexture (no render pass, no blending subtleties)
//     and the view is marked rendered so the closing flush uses LoadOpLoad
//     instead of clearing the upload away.
//  3. The swap is restored right after the flush; the main pixmap is never
//     touched, so no save/restore copy is needed.
//
// The swap is a pointer exchange on full-surface pixmaps, so every size,
// stride and coordinate assumption in drawing code keeps holding. It only
// arms when a GPU session is active; CPU-only contexts keep the legacy
// behavior bit-identically (their present path uploads the pixmap).
// Serialized raster-thread use only, same discipline as BeginOffscreenPass.

// passScratchBackend is the GPU-side half of the commit, implemented by
// internal/gpu (same narrow-interface pattern as the offscreenCreator
// assertion in CreateOffscreenTexture).
type passScratchBackend interface {
	CommitScratchRegion(view gpucontext.TextureView, payload []byte, bytesPerRow, rows, w, h int) error
}

// BeginPassScratch arms the CPU scratch swap for an offscreen record pass.
// r is the pass area in device pixels (pass-local origin); it is clamped
// to the pixmap and cleared to transparent. While armed, c.pixmap aliases
// a pooled scratch buffer; restore swaps the live pixmap back.
// ok=false leaves everything untouched (CPU-only, no GPU, bad geometry,
// or a swap already active): callers then run the legacy path verbatim.
func (c *Context) BeginPassScratch(r image.Rectangle) (restore func(), ok bool) {
	noop := func() {}
	if c == nil || c.pixmap == nil || c.passMain != nil {
		return noop, false
	}
	if CPUOnlyMode() || c.gpuCtxOps() == nil {
		return noop, false
	}
	pw, ph := c.pixmap.Width(), c.pixmap.Height()
	if pw <= 0 || ph <= 0 {
		return noop, false
	}
	r = r.Intersect(image.Rect(0, 0, pw, ph))
	if r.Empty() {
		return noop, false
	}
	if c.passScratch == nil || c.passScratch.Width() != pw || c.passScratch.Height() != ph {
		c.passScratch = NewPixmap(pw, ph)
		// Fresh pixmap is zeroed by construction; matches cleared state.
		c.passScratchDirty = false
	}
	// E6 lazy scratch: clear only when a previous pass left content behind.
	// A clean-reused scratch (the common all-GPU pass) skips the fullscreen
	// memset here and the zero-scan/upload at commit.
	if c.passScratchDirty {
		clearPixmapRect(c.passScratch, r)
		c.passScratchDirty = false
	}
	c.passMain = c.pixmap
	c.pixmap = c.passScratch
	c.passRect = r
	return func() {
		if c.passMain != nil {
			c.pixmap = c.passMain
			c.passMain = nil
		}
	}, true
}

// CommitPassScratchToView uploads a dirty scratch region into the pass view.
// It also discharges the scratch debt: after an upload the region holds
// content that a later pass must clear before reuse, so the dirty flag stays
// set (Begin pre-clears while set). A clean skip leaves the flag unset.
// Call only between BeginPassScratch and its restore; without an active
// swap it returns committed=false, nil.
func (c *Context) CommitPassScratchToView(view gpucontext.TextureView) (committed bool, err error) {
	if c == nil || c.passMain == nil || c.passScratch == nil {
		return false, nil
	}
	r := c.passRect.Intersect(image.Rect(0, 0, c.passScratch.Width(), c.passScratch.Height()))
	if r.Empty() || view.IsNil() {
		return false, nil
	}
	// E6 lazy scratch: no CPU fallback landed (flag unset) → scratch holds
	// nothing new; skip the zero-scan and upload entirely. Stale content is
	// impossible: any previous content forced a clear at Begin (flag was set).
	if !c.passScratchDirty {
		return false, nil
	}
	clean := scratchRectClean(c.passScratch, r)
	if clean {
		return false, nil
	}
	payload, bpr := extractSwizzledRows(c.passScratch, r)
	rc, ok := c.gpuCtxOps().(passScratchBackend)
	if !ok {
		return false, fmt.Errorf("render: GPU backend cannot commit pass scratch (no CommitScratchRegion)")
	}
	if err := rc.CommitScratchRegion(view, payload, bpr, r.Dy(), r.Dx(), r.Dy()); err != nil {
		return false, err
	}
	return true, nil
}

// clearPixmapRect fills r with transparent black (premultiplied zeros).
func clearPixmapRect(p *Pixmap, r image.Rectangle) {
	if p == nil || len(p.data) == 0 {
		return
	}
	r = r.Intersect(image.Rect(0, 0, p.width, p.height))
	if r.Empty() {
		return
	}
	stride := p.width * 4
	n := r.Dx() * 4
	for y := r.Min.Y; y < r.Max.Y; y++ {
		off := y*stride + r.Min.X*4
		clear(p.data[off : off+n])
	}
}

// scratchZeroChunk is the reusable all-zero comparator for scratchRectClean.
var scratchZeroChunk = make([]byte, 64*1024)

// scratchRectClean reports whether every byte in r is zero (untouched
// transparent scratch), using runtime memcmp rows for an early-out scan.
func scratchRectClean(p *Pixmap, r image.Rectangle) bool {
	if p == nil || len(p.data) == 0 {
		return true
	}
	r = r.Intersect(image.Rect(0, 0, p.width, p.height))
	if r.Empty() {
		return true
	}
	stride := p.width * 4
	n := r.Dx() * 4
	for y := r.Min.Y; y < r.Max.Y; y++ {
		off := y*stride + r.Min.X*4
		row := p.data[off : off+n]
		for len(row) > 0 {
			k := len(scratchZeroChunk)
			if k > len(row) {
				k = len(row)
			}
			if !bytes.Equal(row[:k], scratchZeroChunk[:k]) {
				return false
			}
			row = row[k:]
		}
	}
	return true
}

// extractSwizzledRows copies r out of a premultiplied-RGBA pixmap into a
// BGRA payload (offscreen cache texture format) with 256-byte aligned row
// pitch for WriteTexture. Returns the payload and its bytes-per-row.
func extractSwizzledRows(p *Pixmap, r image.Rectangle) ([]byte, int) {
	const copyPitchAlignment = 256
	w, h := r.Dx(), r.Dy()
	bpr := w * 4
	aligned := (bpr + copyPitchAlignment - 1) &^ (copyPitchAlignment - 1)
	out := make([]byte, aligned*h)
	stride := p.width * 4
	for y := 0; y < h; y++ {
		src := (r.Min.Y+y)*stride + r.Min.X*4
		dst := y * aligned
		for x := 0; x < w; x++ {
			// Pixmap is premultiplied RGBA, offscreen texture is BGRA8Unorm.
			out[dst+0] = p.data[src+2]
			out[dst+1] = p.data[src+1]
			out[dst+2] = p.data[src+0]
			out[dst+3] = p.data[src+3]
			src += 4
			dst += 4
		}
	}
	return out, aligned
}
