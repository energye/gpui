package render

import (
	gpucontext "github.com/energye/gpui/gpu/context"
)

// TextureView is an owned GPU texture handle for picture-texture caching
// (B1, Flutter RasterCache). It hides the gpu/context dependency from ui/
// layers (G1: ui → render → gpu, never ui → gpu).
//
// Call Release to return the texture resources to the GPU. The handle is
// invalid after Release.
type TextureView struct {
	view    gpucontext.TextureView
	release func()
}

// Release returns the texture resources to the GPU. Idempotent.
func (t *TextureView) Release() {
	if t == nil || t.release == nil {
		return
	}
	t.release()
	t.release = nil
}

// Raw returns the underlying GPU texture view (render-internal use).
func (t *TextureView) Raw() gpucontext.TextureView {
	if t == nil {
		return gpucontext.TextureView{}
	}
	return t.view
}

// picCacheState holds the active picture-texture recording session
// (Flutter RasterCache pattern: rasterize a display list once, blit after).
type picCacheState struct {
	view    gpucontext.TextureView
	release func()
	w, h    int
	// ox, oy is the recording origin (the Translate(-ox,-oy) applied before
	// Replay). Live clip bounds are expressed in main-surface coordinates, so
	// the recording clip must be translated by (-ox,-oy) and clamped to the
	// texture rect — otherwise the scissor overflows the offscreen target and
	// the wgpu submit fails (B1 C3 discovery).
	ox, oy float64
	// prevPixmap is the drawing target to restore on End. Recorded CPU
	// fallback draws go into a pooled scratch pixmap that is discarded.
	prevPixmap *Pixmap
	scratch    *Pixmap
}

// PictureCacheEnabled reports whether a picture-texture recording session is
// active on this Context.
func (c *Context) PictureCacheEnabled() bool {
	return c != nil && c.picCache != nil
}

// BeginPictureCache opens a picture-texture recording session (Flutter
// RasterCache): subsequent draws are captured into a fresh offscreen texture
// of the given device-pixel size instead of the main target. Coordinates are
// relative to the current CTM origin (callers Translate(-ox,-oy) first);
// ox/oy is that recording origin, used to localize the active clip.
//
// Returns false when the GPU path is unavailable — callers must fall back to
// a normal Replay. Pending main-target commands are flushed first so the
// recording texture contains only the recorded content.
func (c *Context) BeginPictureCache(w, h int, ox, oy float64) bool {
	if c == nil || w <= 0 || h <= 0 || c.picCache != nil {
		return false
	}
	c.ensureGPUCtx()
	rc := c.gpuCtxOps()
	if rc == nil {
		return false
	}
	// Main-target pending commands must not leak into the recording texture.
	if rc.PendingCount() > 0 {
		_ = c.FlushGPU()
	}
	view, release := c.CreateOffscreenTexture(w, h)
	if view.IsNil() || release == nil {
		return false
	}
	// Route CPU-fallback draws into a discarded scratch pixmap (mirrors the
	// PushLayer isolation surface; GPU draws go to the recording texture).
	// The scratch MUST match the recording size: FlushGPUWithView renders
	// with target.Width/Height from the pixmap, and MSAA render sessions
	// fail the wgpu submit when those differ from the view size (B1 C3
	// submit-failed discovery — pure-blit sessions tolerated the mismatch,
	// vector/text paths did not).
	if c.layerStack == nil {
		c.layerStack = newLayerStack()
	}
	scratch := c.layerStack.pool.GetForOverwrite(w, h)
	prev := c.pixmap
	c.pixmap = scratch
	c.picCache = &picCacheState{
		view: view, release: release, w: w, h: h,
		ox: ox, oy: oy,
		prevPixmap: prev, scratch: scratch,
	}
	return true
}

// EndPictureCache finishes the recording session: flushes recorded draws into
// the offscreen texture, restores the main drawing target, and returns the
// texture handle. The caller owns the texture (must call Release() when the
// cached picture is invalidated or dropped).
//
// Returns nil when no session was active or nothing was recorded.
func (c *Context) EndPictureCache() *TextureView {
	if c == nil || c.picCache == nil {
		return nil
	}
	st := c.picCache
	c.picCache = nil
	// Restore the main drawing target before flushing so the recording
	// commands resolve into the recording texture only.
	if c.pixmap == st.scratch {
		c.pixmap = st.prevPixmap
	}
	c.layerStack.pool.Put(st.scratch)
	rc := c.gpuCtxOps()
	if rc == nil || rc.PendingCount() == 0 {
		// Nothing recorded — release the texture and report failure.
		st.release()
		return nil
	}
	if err := c.FlushGPUWithView(st.view, uint32(st.w), uint32(st.h)); err != nil { //nolint:gosec // bounded by caller size
		st.release()
		return nil
	}
	return &TextureView{view: st.view, release: st.release}
}
