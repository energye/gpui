//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build !nogpu

package gpu

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// textureSet holds the MSAA color, depth/stencil, and resolve textures an
// owner renders into. Since P2.4d the trio itself lives in the device-level
// surfaceCache (surface_cache.go): the set keeps the owner-facing fields that
// the render paths read directly, and holds one strong Ref for as long as it
// owns the trio.
//
// The texture set supports stencil-then-cover and SDF rendering:
//   - MSAA color: 1x default (BGRA8Unorm, RenderAttachment; 4x explicit opt-in)
//   - Depth/stencil: 1x default (Depth24PlusStencil8, RenderAttachment)
//   - Resolve: 1x sample, BGRA8Unorm, RenderAttachment | CopySrc (offscreen only)
type textureSet struct {
	msaaTex     hal.Texture
	msaaView    hal.TextureView
	stencilTex  hal.Texture
	stencilView hal.TextureView
	resolveTex  hal.Texture
	resolveView hal.TextureView
	width       uint32
	height      uint32

	// surfaceCache is the device-level trio cache this set acquires from;
	// surfaceRef is its strong reference. A nil cache keeps the direct
	// create/destroy path (standalone tests, renderers built without a
	// registry).
	surfaceCache *surfaceCache
	surfaceRef   res.Ref
	surfaceKey   res.CacheKey

	// deferRelease hands a released trio back to the caller instead of the
	// cache directly. The session installs it to track the ref as in-flight
	// for the current submission: on rebuild the old views may still be
	// referenced by commands queued earlier in the frame, so releasing them
	// right away leaves dangling handles (the resize-crash root cause).
	deferRelease func(ref res.Ref, retire bool)

	// retireOnRelease destroys the trio instead of pooling it. Owners whose
	// command buffers are submitted outside res.Submission (shared stencil
	// renderer, textured stencil caches) keep the pre-pooling
	// "destroy on rebuild" contract; degraded 1x1 trios always retire.
	retireOnRelease bool
	degradedTrio    bool

	// directTrio owns the trio when surfaceCache is nil.
	directTrio *surfaceTrio

	// failedErr latches a terminal double-failure OOM (full size + 1x1
	// fallback both failed) for failedW/failedH: later draws fail fast with
	// the stored error instead of retrying native allocation every frame
	// (no retry storm, no log flood, no half-built state for later draws to
	// trip on — the multiwindow 1.3 black-screen/SIGSEGV root). A different
	// size clears the latch for one re-probe; device loss clears it.
	failedErr        error
	failedW, failedH uint32

	// degradedW/H latch a single-failure OOM (full size failed, 1x1 fallback
	// serving): the set keeps serving the 1x1 depth and re-probes full size
	// at most once per degradedReprobeFrames ensure calls instead of every
	// frame. Every-frame full-size re-probe after a fallback turned one
	// transient OOM into a per-frame alloc-fail storm that pinned driver
	// heap blocks until a 1GB card died (~540MB in 3s measured 2026-09-17).
	degradedW, degradedH uint32
	degradedFrames       int
}

// degradedReprobeFrames bounds full-size re-probes while a 1x1 depth
// fallback is serving: one re-probe per ~2s at 60fps. A recovered heap
// clears the latch on success; a still-tight heap costs one failed attempt
// per window instead of one per frame.
const degradedReprobeFrames = 120

// degradedServe fast-paths a latched 1x1 fallback: same size within the
// window keeps serving, expired window clears the latch for one re-probe.
// Reports served=true when the caller must return nil (keep serving).
func (ts *textureSet) degradedServe(w, h uint32) (served bool) {
	if ts.degradedW != w || ts.degradedH != h {
		return false
	}
	ts.degradedFrames++
	if ts.degradedFrames < degradedReprobeFrames {
		return true
	}
	ts.degradedW, ts.degradedH, ts.degradedFrames = 0, 0, 0
	return false
}

// latchDegraded records a serving 1x1 fallback for w,h so later frames stop
// re-probing full size every frame. ts.width/height stay at w,h so the
// serving state passes the same-size early-return checks.
func (ts *textureSet) latchDegraded(w, h uint32) {
	ts.degradedW, ts.degradedH, ts.degradedFrames = w, h, 0
	ts.width, ts.height = w, h
}

// clearDegraded drops the 1x1 latch after a successful full-size re-probe.
func (ts *textureSet) clearDegraded() {
	ts.degradedW, ts.degradedH, ts.degradedFrames = 0, 0, 0
}

// ensureTextures ensures an offscreen trio: MSAA (sc>1), depth and resolve.
// The resolve target is required for CPU readback.
func (ts *textureSet) ensureTextures(device hal.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	return ts.ensure(device, w, h, labelPrefix, true, samples...)
}

// ensureSurfaceTextures ensures a surface trio: MSAA (sc>1) and depth only.
// The surface view is the resolve target (positive VRAM save on low-GPU
// hosts).
func (ts *textureSet) ensureSurfaceTextures(device hal.Device, w, h uint32, labelPrefix string, samples ...uint32) error {
	return ts.ensure(device, w, h, labelPrefix, false, samples...)
}

// ensure acquires (or reuses) the trio for size + mode. A same-size request in
// the other mode swaps the whole trio (surface and offscreen are distinct
// cache keys): the old same-size path dropped resolve in place, which under
// pooling would hand the surface trio to a readback pass with resolveTex nil.
func (ts *textureSet) ensure(device hal.Device, w, h uint32, labelPrefix string, withResolve bool, samples ...uint32) error {
	if device == nil {
		return fmt.Errorf("ensureTextures: device is nil")
	}
	if err := ts.failedFast(w, h); err != nil {
		return err
	}
	if ts.degradedServe(w, h) {
		return nil
	}
	sc := uint32(1) // engine default is 1x; 4x is an explicit opt-in
	if len(samples) > 0 && samples[0] > 0 {
		sc = samples[0]
	}
	needMSAA := sc > 1

	key := surfaceKey(w, h, sc, withResolve, false)
	if ts.width == w && ts.height == h && ts.surfaceKey == key &&
		ts.stencilView != nil && (!needMSAA || ts.msaaView != nil) {
		return nil
	}
	ts.destroyTextures()

	if err := ts.acquireSurface(device, key, labelPrefix); err != nil {
		var depthOOM *surfaceDepthOOMError
		if errors.As(err, &depthOOM) {
			// Post-TDR / AutoRecover: full-size depth may OOM while device
			// heap is still reclaiming. Fall back to 1x1 depth (stencil
			// Always/Keep still valid) and latch the degraded window.
			log.Printf("depth %dx%d samples=%d OOM, falling back to 1x1: %v", w, h, sc, depthOOM.err)
			degradedKey := surfaceKey(w, h, sc, withResolve, true)
			if err := ts.acquireSurface(device, degradedKey, labelPrefix); err != nil {
				if term := surfaceTerminalError(err); term != nil {
					return ts.failTerminal(w, h, term.tried, term.label, term.err)
				}
				return err
			}
			ts.latchDegraded(w, h)
		} else if term := surfaceTerminalError(err); term != nil {
			ts.destroyTextures()
			return ts.failTerminal(w, h, term.tried, term.label, term.err)
		} else {
			return err
		}
	} else {
		ts.clearDegraded()
	}

	if withResolve {
		slogger().Info("created offscreen textures",
			"label", labelPrefix,
			"width", w, "height", h,
			"msaa_samples", sc,
			"msaa_color", needMSAA,
			"degraded", ts.degradedTrio,
		)
	} else {
		slogger().Info("created surface textures",
			"label", labelPrefix,
			"width", w, "height", h,
			"msaa_samples", sc,
			"msaa_color", needMSAA,
			"degraded", ts.degradedTrio,
		)
	}
	return nil
}

// acquireSurface takes the trio for key from the cache (or creates it
// directly when no cache is bound) and publishes its fields.
func (ts *textureSet) acquireSurface(device hal.Device, key res.CacheKey, labelPrefix string) error {
	if ts.surfaceCache != nil {
		ref, trio, err := ts.surfaceCache.acquire(device, key)
		if err != nil {
			return err
		}
		ts.surfaceRef = ref
		ts.directTrio = nil
		ts.surfaceKey = key
		ts.degradedTrio = trio.degraded
		ts.publishTrio(trio)
		return nil
	}
	trio, err := createSurfaceTrio(device, key, labelPrefix)
	if err != nil {
		return err
	}
	ts.directTrio = trio
	ts.surfaceKey = key
	ts.degradedTrio = trio.degraded
	ts.publishTrio(trio)
	return nil
}

// publishTrio copies the trio handles into the owner-facing fields. The
// render paths read these fields directly (20+ sites), so the set keeps the
// historical shape while the cache owns lifetime.
func (ts *textureSet) publishTrio(trio *surfaceTrio) {
	if trio == nil {
		return
	}
	ts.msaaTex = trio.msaaTex
	ts.msaaView = trio.msaaView
	ts.stencilTex = trio.stencilTex
	ts.stencilView = trio.stencilView
	ts.resolveTex = trio.resolveTex
	ts.resolveView = trio.resolveView
	ts.width = trio.width
	ts.height = trio.height
}

// surfaceTerminalError unwraps the terminal OOM marker, if present.
func surfaceTerminalError(err error) *surfaceTerminalOOMError {
	var term *surfaceTerminalOOMError
	if errors.As(err, &term) {
		return term
	}
	return nil
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

// destroyTextures releases this owner's trio. Cache-bound sets hand the ref
// back (pooled or retired); direct sets destroy it now. Fields always clear so
// a failed re-acquire cannot be mistaken for a live set.
func (ts *textureSet) destroyTextures() {
	ref := ts.surfaceRef
	trio := ts.directTrio
	ts.surfaceRef = res.Ref{}
	ts.directTrio = nil
	degraded := ts.degradedTrio
	ts.degradedTrio = false

	switch {
	case !ref.IsNil():
		retire := degraded || ts.retireOnRelease
		if ts.deferRelease != nil {
			ts.deferRelease(ref, retire)
		} else if c := ts.surfaceCache; c != nil {
			if retire {
				c.releaseRetire(ref)
			} else {
				c.release(ref)
			}
		}
		ts.surfaceKey = res.CacheKey{}
	case trio != nil:
		trio.Release()
		ts.surfaceKey = res.CacheKey{}
	default:
		// Manually populated set (tests / legacy direct handles): release the
		// individual handles inline, matching the pre-cache contract.
		ts.releaseLooseTextures()
	}

	ts.msaaTex = nil
	ts.msaaView = nil
	ts.stencilTex = nil
	ts.stencilView = nil
	ts.resolveTex = nil
	ts.resolveView = nil
	ts.width = 0
	ts.height = 0
}

// releaseLooseTextures destroys handles assigned directly to the set fields
// (no trio owns them). Views go first, matching surfaceTrio.Release.
func (ts *textureSet) releaseLooseTextures() {
	if ts.msaaView != nil {
		ts.msaaView.Destroy()
	}
	if ts.stencilView != nil {
		ts.stencilView.Destroy()
	}
	if ts.resolveView != nil {
		ts.resolveView.Destroy()
	}
	if ts.msaaTex != nil {
		ts.msaaTex.Destroy()
	}
	if ts.stencilTex != nil {
		ts.stencilTex.Destroy()
	}
	if ts.resolveTex != nil {
		ts.resolveTex.Destroy()
	}
}

// createTextureRetryOOM creates a texture; on OOM-like errors flushes and retries.
// Second try forces SampleCount=1 when the failed desc used MSAA (post-TDR reclaim).
func createTextureRetryOOM(device hal.Device, desc *hal.TextureDescriptor) (hal.Texture, error) {
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
	// Still failing: drop surface-sized attachments process-wide (Skia
	// freeGpuResources: session MSAA/depth, surface trio cache, offscreen
	// pools, texture pool; includes pending drains), then retry once. The
	// current ensure may lose just-built siblings to this purge; all error
	// paths below destroy the partial trio (nil-safe), and the render pass
	// guards nil msaaView (falls back to direct-to-view), so the worst case
	// is one degraded frame, never a dangling handle.
	render.PurgeAcceleratorSurfaceResources()
	if tex, err3 := device.CreateTexture(desc); err3 == nil {
		oomLogThrottled("CreateTexture OOM surface-purge retry ok label=%s", desc.Label)
		return tex, nil
	} else if render.IsGPUOutOfMemory(err3) {
		err2 = err3
	} else {
		return nil, err3
	}
	// Last resort: drop MSAA for this allocation.
	if desc.SampleCount > 1 {
		d2 := *desc
		d2.SampleCount = 1
		return device.CreateTexture(&d2)
	}
	return nil, err2
}
