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
	"fmt"
	"sync"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
	"github.com/energye/gpui/render/internal/gpu/res"
)

// Surface trio cache (P2.4d): every owner that needs a same-size set of
// MSAA color + depth/stencil + resolve targets (session, shared stencil
// renderer, textured stencil caches, offscreen) now acquires one trio from a
// single device-level res.Cache instead of keeping its own fixed textureSet.
// Key = size + samples + surface mode (resolve present or not); an owner holds
// a strong Ref for as long as it renders into the trio, so two owners can
// never be handed the same native at once (D1: strong refs, not shared
// ownership per frame).
const (
	// surfaceUsageResolve marks a trio that also carries the single-sample
	// resolve target (offscreen/readback mode). Surface mode draws straight to
	// the surface view and skips it (positive VRAM save on low-GPU hosts).
	surfaceUsageResolve uint64 = 1 << 0
	// surfaceUsageDegraded marks the 1x1-depth fallback trio (post-TDR / tight
	// heap). It is never pooled: a successful full-size re-probe must not be
	// served a 1x1 depth from the pool.
	surfaceUsageDegraded uint64 = 1 << 1
)

// surfaceTrio is one cached MSAA/depth/resolve set. It implements res.Native:
// the registry calls Release exactly once, when the entry is retired and no
// Ref/submission still references it.
type surfaceTrio struct {
	msaaTex     hal.Texture
	msaaView    hal.TextureView
	stencilTex  hal.Texture
	stencilView hal.TextureView
	resolveTex  hal.Texture
	resolveView hal.TextureView
	width       uint32
	height      uint32
	samples     uint32
	// degraded reports the 1x1-depth fallback. Such owners release through
	// Cache.ReleaseRetire so the fallback never enters the pool.
	degraded bool
}

// Release destroys views before their textures (the old destroyTextures
// order) and tolerates nil members for partially created trios.
func (t *surfaceTrio) Release() {
	if t == nil {
		return
	}
	if t.msaaView != nil {
		t.msaaView.Destroy()
		t.msaaView = nil
	}
	if t.stencilView != nil {
		t.stencilView.Destroy()
		t.stencilView = nil
	}
	if t.resolveView != nil {
		t.resolveView.Destroy()
		t.resolveView = nil
	}
	if t.msaaTex != nil {
		t.msaaTex.Destroy()
		t.msaaTex = nil
	}
	if t.stencilTex != nil {
		t.stencilTex.Destroy()
		t.stencilTex = nil
	}
	if t.resolveTex != nil {
		t.resolveTex.Destroy()
		t.resolveTex = nil
	}
}

// surfaceDepthOOMError reports a failed full-size depth allocation. It is not
// terminal: the owner retries once with the degraded (1x1 depth) key, exactly
// like the old in-place fallback.
type surfaceDepthOOMError struct {
	label string
	err   error
}

func (e *surfaceDepthOOMError) Error() string {
	return fmt.Sprintf("create %s: %v", e.label, e.err)
}

func (e *surfaceDepthOOMError) Unwrap() error { return e.err }

// surfaceTerminalOOMError is the double failure (full size + 1x1 fallback, or
// an exhausted resolve allocation). The owner latches it (failTerminal) so the
// same size fails fast instead of re-probing native allocation every frame.
type surfaceTerminalOOMError struct {
	label string
	tried string
	err   error
}

func (e *surfaceTerminalOOMError) Error() string {
	return fmt.Sprintf("create %s (%s): %v", e.label, e.tried, e.err)
}

func (e *surfaceTerminalOOMError) Unwrap() error { return e.err }

// surfaceCache wraps the device-level res.Cache for surface trios.
type surfaceCache struct {
	mu     sync.Mutex
	device hal.Device // device of the in-flight/next create call

	owner res.Owner // device owner: trios outlive any single window
	cache *res.Cache
	name  string
}

// newSurfaceCache builds the device-level trio cache with the declared
// BudgetSurfacePool bytes and BudgetStencilScratch per-key cap.
func newSurfaceCache(owner res.Owner, name string) *surfaceCache {
	budget := render.DefaultBudget(render.BudgetSurfacePool)
	maxPerKey := render.DefaultBudget(render.BudgetStencilScratch).MaxEntries
	return newSurfaceCacheWithLimits(owner, name, uint64(budget.MaxBytes), maxPerKey)
}

// newSurfaceCacheWithLimits is the test seam for budget/eviction assertions.
func newSurfaceCacheWithLimits(owner res.Owner, name string, budget uint64, maxPerKey int) *surfaceCache {
	c := &surfaceCache{owner: owner, name: name}
	if owner.IsNil() || owner.Registry() == nil {
		return c
	}
	c.cache = res.NewCache(owner, c.create, res.CacheOptions{
		Name:      name,
		Budget:    budget,
		SizeFn:    surfaceTrioBytes,
		MaxPerKey: maxPerKey,
	})
	return c
}

// surfaceTrioBytes is the real byte estimate for one trio key: MSAA color
// (sc>1 only), Depth24PlusStencil8 depth (4B/texel/sample; 1 texel when
// degraded) and the single-sample resolve target when present. Matches the
// hal.VramTextureBytes accounting used elsewhere.
func surfaceTrioBytes(k res.CacheKey) uint64 {
	px := uint64(k.W) * uint64(k.H)
	sc := uint64(k.Samples)
	if sc < 1 {
		sc = 1
	}
	var bytes uint64
	if sc > 1 {
		bytes += px * 4 * sc
	}
	depthPx := px
	if k.Usage&surfaceUsageDegraded != 0 {
		depthPx = 1
	}
	bytes += depthPx * 4 * sc
	if k.Usage&surfaceUsageResolve != 0 {
		bytes += px * 4
	}
	return bytes
}

// surfaceKey builds the cache key for one trio request.
func surfaceKey(w, h, samples uint32, withResolve, degraded bool) res.CacheKey {
	if samples == 0 {
		samples = 1
	}
	var usage uint64
	if withResolve {
		usage |= surfaceUsageResolve
	}
	if degraded {
		usage |= surfaceUsageDegraded
	}
	return res.CacheKey{W: w, H: h, Samples: samples, Usage: usage}
}

// setDevice records the device the next create call must use. The cache is
// shared by every window on one device; GPUShared abandons it before a device
// switch, so a create never straddles two devices.
func (c *surfaceCache) setDevice(device hal.Device) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.device = device
	c.mu.Unlock()
}

// create is the res.Cache CreateFn. It runs without the cache lock (the OOM
// purge chain re-enters the cache through PurgeIdle).
func (c *surfaceCache) create(key res.CacheKey) (res.Native, error) {
	c.mu.Lock()
	device := c.device
	c.mu.Unlock()
	if device == nil {
		return nil, fmt.Errorf("surface cache %q: device is nil", c.name)
	}
	trio, err := createSurfaceTrio(device, key, "surface")
	if err != nil {
		return nil, err
	}
	return trio, nil
}

// acquire returns a strong Ref plus its trio for key.
func (c *surfaceCache) acquire(device hal.Device, key res.CacheKey) (res.Ref, *surfaceTrio, error) {
	if c == nil || c.cache == nil {
		return res.Ref{}, nil, fmt.Errorf("surface cache: not bound to a registry")
	}
	c.setDevice(device)
	ref, err := c.cache.Acquire(key)
	if err != nil {
		return res.Ref{}, nil, err
	}
	native := c.owner.Registry().NativeOf(ref)
	trio, ok := native.(*surfaceTrio)
	if !ok || trio == nil {
		c.cache.ReleaseRetire(ref)
		return res.Ref{}, nil, fmt.Errorf("surface cache: unexpected native %T for %dx%d", native, key.W, key.H)
	}
	return ref, trio, nil
}

// release returns a trio Ref to the pool.
func (c *surfaceCache) release(ref res.Ref) {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.Release(ref)
}

// releaseRetire destroys the trio instead of pooling it (degraded fallback, or
// owners whose command buffers are submitted outside res.Submission).
func (c *surfaceCache) releaseRetire(ref res.Ref) {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.ReleaseRetire(ref)
}

// purgeIdle retires every pooled trio (OOM / unpresentable surface outlet).
func (c *surfaceCache) purgeIdle() (int, uint64) {
	if c == nil || c.cache == nil {
		return 0, 0
	}
	return c.cache.PurgeIdle()
}

// releaseAll force-releases pooled and checked-out trios (device teardown on a
// healthy device).
func (c *surfaceCache) releaseAll() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.ReleaseAll()
}

// invalidateAll drops bookkeeping without touching natives (device-loss path:
// the abandon flow owns native teardown).
func (c *surfaceCache) invalidateAll() {
	if c == nil || c.cache == nil {
		return
	}
	c.cache.InvalidateAll()
}

// stats exposes the per-domain probe (hits/misses/entries/bytes/peak).
func (c *surfaceCache) stats() res.CacheStats {
	if c == nil || c.cache == nil {
		return res.CacheStats{}
	}
	return c.cache.Stats()
}

// createSurfaceTrio allocates one MSAA/depth/resolve set for key.
//
// Errors:
//   - full-size depth failed  → *surfaceDepthOOMError (owner retries degraded)
//   - 1x1 depth failed too    → *surfaceTerminalOOMError (owner latches)
//   - resolve failed          → *surfaceTerminalOOMError (owner latches)
//   - other view/create errors → plain error (retried next frame, as before)
func createSurfaceTrio(device hal.Device, key res.CacheKey, labelPrefix string) (*surfaceTrio, error) {
	if device == nil {
		return nil, fmt.Errorf("createSurfaceTrio: device is nil")
	}
	w, h := key.W, key.H
	sc := key.Samples
	if sc == 0 {
		sc = 1
	}
	size := hal.Extent3D{Width: w, Height: h, DepthOrArrayLayers: 1}
	degraded := key.Usage&surfaceUsageDegraded != 0
	trio := &surfaceTrio{width: w, height: h, samples: sc, degraded: degraded}
	fail := func(err error) (*surfaceTrio, error) {
		trio.Release()
		return nil, err
	}

	if sc > 1 {
		msaaTex, err := createTextureRetryOOM(device, &hal.TextureDescriptor{
			Label:         labelPrefix + "_msaa_color",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   sc,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment,
		})
		if err != nil {
			return nil, fmt.Errorf("create MSAA color texture: %w", err)
		}
		trio.msaaTex = msaaTex
		msaaView, err := device.CreateTextureView(msaaTex, &hal.TextureViewDescriptor{
			Label:         labelPrefix + "_msaa_color_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			return fail(fmt.Errorf("create MSAA color view: %w", err))
		}
		trio.msaaView = msaaView
	}

	depthW, depthH, depthSamples := w, h, sc
	depthLabel := labelPrefix + "_depth_stencil"
	if degraded {
		depthW, depthH, depthSamples = 1, 1, 1
		depthLabel = labelPrefix + "_depth_stencil_1x1"
	}
	stencilTex, err := createTextureRetryOOM(device, &hal.TextureDescriptor{
		Label:         depthLabel,
		Size:          hal.Extent3D{Width: depthW, Height: depthH, DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   depthSamples,
		Dimension:     types.TextureDimension2D,
		Format:        types.TextureFormatDepth24PlusStencil8,
		Usage:         types.TextureUsageRenderAttachment,
	})
	if err != nil {
		if degraded {
			return fail(&surfaceTerminalOOMError{
				label: labelPrefix + "_depth_stencil",
				tried: "full size and 1x1 fallback",
				err:   err,
			})
		}
		return nil, &surfaceDepthOOMError{label: labelPrefix + "_depth_stencil", err: err}
	}
	trio.stencilTex = stencilTex
	stencilView, err := device.CreateTextureView(stencilTex, &hal.TextureViewDescriptor{
		Label:         labelPrefix + "_depth_stencil_view",
		Format:        types.TextureFormatDepth24PlusStencil8,
		Dimension:     types.TextureViewDimension2D,
		Aspect:        types.TextureAspectAll,
		MipLevelCount: 1,
	})
	if err != nil {
		return fail(fmt.Errorf("create depth/stencil view: %w", err))
	}
	trio.stencilView = stencilView

	if key.Usage&surfaceUsageResolve != 0 {
		resolveTex, err := createTextureRetryOOM(device, &hal.TextureDescriptor{
			Label:         labelPrefix + "_resolve",
			Size:          size,
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     types.TextureDimension2D,
			Format:        types.TextureFormatBGRA8Unorm,
			Usage:         types.TextureUsageRenderAttachment | types.TextureUsageCopySrc,
		})
		if err != nil {
			return fail(&surfaceTerminalOOMError{
				label: labelPrefix + "_resolve",
				tried: "flush + cache-purge retries exhausted",
				err:   err,
			})
		}
		trio.resolveTex = resolveTex
		resolveView, err := device.CreateTextureView(resolveTex, &hal.TextureViewDescriptor{
			Label:         labelPrefix + "_resolve_view",
			Format:        types.TextureFormatBGRA8Unorm,
			Dimension:     types.TextureViewDimension2D,
			Aspect:        types.TextureAspectAll,
			MipLevelCount: 1,
		})
		if err != nil {
			return fail(fmt.Errorf("create resolve view: %w", err))
		}
		trio.resolveView = resolveView
	}
	return trio, nil
}
