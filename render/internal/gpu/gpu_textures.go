//go:build !nogpu

package gpu

import (
	"fmt"
	"log"
	"os"

	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
)

// textureSet holds a set of MSAA color, depth/stencil, and resolve textures
// for offscreen rendering. This is shared by GPURenderSession and
// StencilRenderer to avoid code duplication.
//
// The texture set supports stencil-then-cover and SDF rendering:
//   - MSAA color: 4x samples, BGRA8Unorm, RenderAttachment
//   - Depth/stencil: 4x samples, Depth24PlusStencil8, RenderAttachment
//   - Resolve: 1x sample, BGRA8Unorm, RenderAttachment | CopySrc
type textureSet struct {
	msaaTex     *webgpu.Texture
	msaaView    *webgpu.TextureView
	stencilTex  *webgpu.Texture
	stencilView *webgpu.TextureView
	resolveTex  *webgpu.Texture
	resolveView *webgpu.TextureView
	width       uint32
	height      uint32

	// retireFn decides how retired textures are handled (P4): nil = release
	// immediately (default); the session injects a deferred-release queue —
	// on rebuild the old views may still be referenced by commands queued
	// earlier in the frame, so releasing them right away leaves dangling
	// handles (the resize-crash root cause).
	retireFn func(tex *webgpu.Texture, view *webgpu.TextureView)

	// stencilPool caches depth/stencil textures by size for sc==1 surface
	// passes (R8 engine hole). Within one retained frame, per-layer offscreen
	// records alternate the pass target size (main band → body panel → hot
	// spot), and without a pool each flip destroyed+recreated the shared
	// stencil texture (~3 GPU alloc cycles per frame; 15s log showed 4293
	// "created surface textures"). Pooled entries rotate instead. Capped and
	// LRU-stamped; evicted entries go through retireFn like any other retired
	// texture (in-flight CBs keep them alive until the GPU is done).
	stencilPool  map[stencilPoolKey]*pooledStencil
	stencilStamp uint64

	// failedErr latches a terminal double-failure OOM (full size + 1x1
	// fallback both failed) for failedW/failedH: later draws fail fast with
	// the stored error instead of retrying native allocation every frame
	// (no retry storm, no log flood, no half-built state for later draws to
	// trip on — the multiwindow 1.3 black-screen/SIGSEGV root). A different
	// size clears the latch for one re-probe; device loss clears it.
	failedErr        error
	failedW, failedH uint32
}

// stencilPoolKey identifies a pooled depth/stencil texture.
type stencilPoolKey struct {
	w, h uint32
}

// pooledStencil is one cached depth/stencil texture + its last-use stamp.
type pooledStencil struct {
	tex   *webgpu.Texture
	view  *webgpu.TextureView
	stamp uint64
}

// stencilPoolCap bounds the size-bucketed stencil cache. Retained frames
// alternate a handful of layer-record sizes; 8 covers far more than any
// observed working set while bounding VRAM (each entry is Depth24PlusStencil8,
// RenderAttachment-only — cheap relative to color textures).
const stencilPoolCap = 8

// takePooledStencil fetches (or creates) a depth/stencil texture for (w,h).
// Returns nil when the caller should fall back to direct creation (pool
// disabled / OOM fallback path). sc must be 1 — MSAA textures are not pooled.
func (ts *textureSet) takePooledStencil(device *webgpu.Device, w, h uint32, labelPrefix string) *webgpu.TextureView {
	if ts.stencilPool == nil {
		ts.stencilPool = make(map[stencilPoolKey]*pooledStencil)
	}
	key := stencilPoolKey{w: w, h: h}
	ts.stencilStamp++
	if e := ts.stencilPool[key]; e != nil {
		e.stamp = ts.stencilStamp
		return e.view
	}
	tex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		return nil
	}
	view, err := device.CreateTextureView(tex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		tex.Release()
		return nil
	}
	// Evict LRU when at capacity (never the entry we are about to add).
	for len(ts.stencilPool) >= stencilPoolCap {
		var victimKey stencilPoolKey
		var oldest uint64 = ^uint64(0)
		for k, e := range ts.stencilPool {
			if e.stamp < oldest {
				oldest = e.stamp
				victimKey = k
			}
		}
		if v := ts.stencilPool[victimKey]; v != nil {
			ts.releaseOrRetire(v.tex, v.view)
		}
		delete(ts.stencilPool, victimKey)
	}
	ts.stencilPool[key] = &pooledStencil{tex: tex, view: view, stamp: ts.stencilStamp}
	return view
}

// ensureTextures creates or recreates textures if the requested dimensions
// differ from the current size. If dimensions match and textures exist,
// this is a no-op. The labelPrefix parameter distinguishes GPU debug labels
// between different owners (e.g., "session" vs "stencil").
//
// The samples parameter sets the MSAA sample count for color and depth/stencil
// textures (typically 4 for MSAA, 1 for non-MSAA fallback).
func (ts *textureSet) ensureTextures(device *webgpu.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	if device == nil {
		return fmt.Errorf("ensureTextures: device is nil")
	}
	if err := ts.failedFast(w, h); err != nil {
		return err
	}
	sc := uint32(4) // default MSAA sample count
	if len(samples) > 0 && samples[0] > 0 {
		sc = samples[0]
	}

	// Offscreen mode requires a resolve texture for readback. After a surface-mode
	// pass (ensureSurfaceTextures), msaa/stencil may exist at the same size with
	// resolveTex == nil. Do not early-return in that state or encodeSubmitReadback
	// fails with "offscreen textures destroyed (concurrent resize?)".
	// sc==1 does not allocate msaa color (colorAttachment draws to resolve directly).
	needMSAA := sc > 1
	haveMSAA := ts.msaaTex != nil && ts.msaaView != nil
	if ts.width == w && ts.height == h && ts.resolveTex != nil && ts.resolveView != nil &&
		ts.stencilView != nil && (!needMSAA || haveMSAA) {
		return nil
	}
	ts.destroyTextures()

	size := webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}

	if needMSAA {
		msaaTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_msaa_color",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   sc,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			return fmt.Errorf("create MSAA color texture: %w", err)
		}
		ts.msaaTex = msaaTex

		msaaView, err := device.CreateTextureView(msaaTex, &webgpu.TextureViewDescriptor{
			Label:         labelPrefix + "_msaa_color_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create MSAA color view: %w", err)
		}
		ts.msaaView = msaaView
	}

	// Depth/stencil texture (sc samples, Depth24PlusStencil8).
	stencilTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		// Post-TDR / AutoRecover: full-size depth may OOM while device heap is
		// still reclaiming. Fall back to 1x1 depth (stencil Always/Keep still valid).
		log.Printf("depth %dx%d samples=%d OOM, falling back to 1x1: %v", size.Width, size.Height, sc, err)
		stencilTex, err = createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_depth_stencil_1x1",
			Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatDepth24PlusStencil8,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			ts.destroyTextures()
			return ts.failTerminal(w, h, "full size and 1x1 fallback", labelPrefix+"_depth_stencil", err)
		}
		// Force recreate next frame when heap has reclaimed (size mismatch path).
		ts.width, ts.height = 0, 0
	}
	ts.stencilTex = stencilTex

	stencilView, err := device.CreateTextureView(stencilTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil view: %w", err)
	}
	ts.stencilView = stencilView

	// Single-sample resolve target (CopySrc for readback). For sc==1 this is
	// also the color attachment (no MSAA resolve).
	resolveTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_resolve",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatBGRA8Unorm,
		Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
	})
	if err != nil {
		ts.destroyTextures()
		return ts.failTerminal(w, h, "flush + cache-purge retries exhausted", labelPrefix+"_resolve", err)
	}
	ts.resolveTex = resolveTex

	resolveView, err := device.CreateTextureView(resolveTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_resolve_view",
		Format:        types.TextureFormatBGRA8Unorm,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create resolve view: %w", err)
	}
	ts.resolveView = resolveView

	ts.width = w
	ts.height = h
	slogger().Info("created offscreen textures",
		"label", labelPrefix,
		"width", w, "height", h,
		"msaa_samples", sc,
		"msaa_color", needMSAA,
	)
	return nil
}

func (ts *textureSet) ensureSurfaceTextures(device *webgpu.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	if device == nil {
		return fmt.Errorf("ensureSurfaceTextures: device is nil")
	}
	if err := ts.failedFast(w, h); err != nil {
		return err
	}
	sc := uint32(4) // default MSAA sample count
	if len(samples) > 0 && samples[0] > 0 {
		sc = samples[0]
	}
	needMSAA := sc > 1
	haveMSAA := ts.msaaTex != nil && ts.msaaView != nil
	// Surface mode does not use resolveTex. sc==1 draws directly to the surface
	// view, so only depth/stencil is required (positive VRAM save on low-GPU hosts).
	if ts.width == w && ts.height == h && ts.stencilView != nil && (!needMSAA || haveMSAA) {
		if ts.resolveView != nil {
			ts.resolveView.Release()
			ts.resolveView = nil
		}
		if ts.resolveTex != nil {
			ts.resolveTex.Release()
			ts.resolveTex = nil
		}
		// Drop leftover MSAA when switching into sc==1.
		if !needMSAA && haveMSAA {
			if ts.msaaView != nil {
				ts.msaaView.Release()
				ts.msaaView = nil
			}
			if ts.msaaTex != nil {
				ts.msaaTex.Release()
				ts.msaaTex = nil
			}
		}
		return nil
	}
	ts.destroyTextures()

	size := webgpu.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}

	// sc==1 surface passes take depth/stencil from the size-keyed pool:
	// retained frames alternate the pass target size per layer record (main
	// band → body panel → hot spot) and a fresh stencil alloc per flip cost
	// ~3 destroy/create cycles per frame (R8 hole). MSAA keeps its own path.
	if !needMSAA {
		if view := ts.takePooledStencil(device, w, h, labelPrefix); view != nil {
			ts.stencilView = view
			ts.stencilTex = nil // owned by the pool; never released here
			ts.width = w
			ts.height = h
			return nil
		}
	}

	if needMSAA {
		msaaTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_msaa_color",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   sc,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			return fmt.Errorf("create MSAA color texture: %w", err)
		}
		ts.msaaTex = msaaTex

		msaaView, err := device.CreateTextureView(msaaTex, &webgpu.TextureViewDescriptor{
			Label:         labelPrefix + "_msaa_color_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			ts.destroyTextures()
			return fmt.Errorf("create MSAA color view: %w", err)
		}
		ts.msaaView = msaaView
	}

	stencilTex, err := createTextureRetryOOM(device, &webgpu.TextureDescriptor{
		Label:         labelPrefix + "_depth_stencil",
		Size:          size,
		MipLevelCount: 1,
		SampleCount:   sc,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		// Post-TDR / AutoRecover: full-size depth may OOM while device heap is
		// still reclaiming. Fall back to 1x1 depth (stencil Always/Keep still valid).
		log.Printf("depth %dx%d samples=%d OOM, falling back to 1x1: %v", size.Width, size.Height, sc, err)
		stencilTex, err = createTextureRetryOOM(device, &webgpu.TextureDescriptor{
			Label:         labelPrefix + "_depth_stencil_1x1",
			Size:          webgpu.Extent3D{Width: 1, Height: 1, DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatDepth24PlusStencil8,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			ts.destroyTextures()
			return ts.failTerminal(w, h, "full size and 1x1 fallback", labelPrefix+"_depth_stencil", err)
		}
		// Force recreate next frame when heap has reclaimed (size mismatch path).
		ts.width, ts.height = 0, 0
	}
	ts.stencilTex = stencilTex

	stencilView, err := device.CreateTextureView(stencilTex, &webgpu.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		ts.destroyTextures()
		return fmt.Errorf("create depth/stencil view: %w", err)
	}
	ts.stencilView = stencilView

	// No resolve texture -- surface view is the resolve target.
	ts.width = w
	ts.height = h
	slogger().Info("created surface textures",
		"label", labelPrefix,
		"width", w, "height", h,
		"msaa_samples", sc,
		"msaa_color", needMSAA,
	)
	return nil
}

// ClearStencilPool releases every pooled depth/stencil texture (session
// teardown). Call only when the GPU is idle/drained.
func (ts *textureSet) ClearStencilPool() {
	if ts.stencilPool == nil {
		return
	}
	for k, e := range ts.stencilPool {
		if e != nil {
			ts.releaseOrRetire(e.tex, e.view)
		}
		delete(ts.stencilPool, k)
	}
}

// failedFast returns the latched terminal OOM error when the same size is
// requested again. A different size clears the latch for one re-probe.
func (ts *textureSet) failedFast(w, h uint32) error {
	if ts == nil || ts.failedErr == nil {
		return nil
	}
	if w == ts.failedW && h == ts.failedH {
		return ts.failedErr
	}
	ts.failedErr = nil
	return nil
}

// failTerminal latches a double-failure OOM with the texture role, size, and
// suggested action. Later draws at the same size fail fast with this error.
func (ts *textureSet) failTerminal(w, h uint32, tried, label string, err error) error {
	e := fmt.Errorf("render: GPU out of memory allocating %s %dx%d (%s): %v; close other GPU windows or set GPUI_POWER to a less-loaded GPU",
		label, w, h, tried, err)
	ts.failedErr = e
	ts.failedW, ts.failedH = w, h
	return e
}

// clearFailed drops the terminal-OOM latch (device-loss path: a fresh device
// gets a fresh probe).
func (ts *textureSet) clearFailed() {
	if ts == nil {
		return
	}
	ts.failedErr = nil
}

func (ts *textureSet) destroyTextures() {
	// Pool entries survive destroyTextures: their whole purpose is to ride
	// out per-frame size flips of the pass target (R8 hole). Flushing here
	// would evict every pooled stencil on each flip — the exact churn the
	// pool exists to prevent. The pool drains via takePooledStencil LRU
	// eviction and via ClearPool on session teardown.
	if ts.resolveView != nil {
		ts.releaseOrRetire(nil, ts.resolveView)
		ts.resolveView = nil
	}
	if ts.resolveTex != nil {
		ts.releaseOrRetire(ts.resolveTex, nil)
		ts.resolveTex = nil
	}
	if ts.stencilView != nil {
		// Pooled stencils are owned by the pool (destroyTextures flushes the
		// pool separately above); releasing the view here would leave the
		// pool entry dangling. Only release session-owned stencils.
		if ts.stencilTex != nil {
			ts.releaseOrRetire(nil, ts.stencilView)
			ts.releaseOrRetire(ts.stencilTex, nil)
			ts.stencilTex = nil
		}
		ts.stencilView = nil
	}
	if ts.msaaView != nil {
		ts.releaseOrRetire(nil, ts.msaaView)
		ts.msaaView = nil
	}
	if ts.msaaTex != nil {
		ts.releaseOrRetire(ts.msaaTex, nil)
		ts.msaaTex = nil
	}
	ts.width = 0
	ts.height = 0
}

// releaseOrRetire releases the resource immediately, or hands it to the
// retire callback when one is installed (deferred release on rebuild).
func (ts *textureSet) releaseOrRetire(tex *webgpu.Texture, view *webgpu.TextureView) {
	if ts.retireFn != nil {
		ts.retireFn(tex, view)
		return
	}
	if view != nil {
		view.Release()
	}
	if tex != nil {
		tex.Release()
	}
}

// createTextureRetryOOM creates a texture; on OOM-like errors flushes and retries.
// Second try forces SampleCount=1 when the failed desc used MSAA (post-TDR reclaim).
func createTextureRetryOOM(device *webgpu.Device, desc *webgpu.TextureDescriptor) (*webgpu.Texture, error) {
	if device == nil || desc == nil {
		return nil, fmt.Errorf("createTextureRetryOOM: nil device/desc")
	}
	if os.Getenv("GPUI_LOG_TEXTURE") == "1" {
		sc := desc.SampleCount
		if sc == 0 {
			sc = 1
		}
		// Rough VRAM estimate: BGRA8=4B, depth24+stencil≈4B per sample.
		bpp := uint64(4)
		est := uint64(desc.Size.Width) * uint64(desc.Size.Height) * uint64(sc) * bpp
		log.Printf("TEX_CREATE label=%q %dx%d samples=%d est_mib=%.2f usage=%d",
			desc.Label, desc.Size.Width, desc.Size.Height, sc, float64(est)/(1024*1024), desc.Usage)
	}
	tex, err := device.CreateTexture(desc)
	if err == nil {
		return tex, nil
	}
	if !render.IsGPUOutOfMemory(err) {
		return nil, err
	}
	oomLogThrottled("CreateTexture OOM label=%s %dx%d samples=%d", desc.Label, desc.Size.Width, desc.Size.Height, desc.SampleCount)
	noteTextureOOM()
	device.FlushCallbacks()
	_ = device.WaitIdle()
	// Retry original once after flush.
	tex, err2 := device.CreateTexture(desc)
	if err2 == nil {
		return tex, nil
	}
	// Still failing: purge rebuildable caches once (glyph/image/layer pools
	// free their GPU textures; misses rebuild on next use), then retry once.
	if freed, byName := render.PurgeEvictables(); freed > 0 {
		oomLogThrottled("CreateTexture OOM purge freed %d entries %v, retrying label=%s", freed, byName, desc.Label)
		if tex, err3 := device.CreateTexture(desc); err3 == nil {
			return tex, nil
		} else {
			err2 = err3
		}
	}
	// Last resort: drop MSAA for this allocation.
	if desc.SampleCount > 1 {
		d2 := *desc
		d2.SampleCount = 1
		return device.CreateTexture(&d2)
	}
	return nil, err2
}
