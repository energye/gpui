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
	"encoding/binary"
	"hash/fnv"
	"image"
	"math"
	"os"
	"sync"
	"sync/atomic"

	gpucontext "github.com/energye/gpui/gpu/context"
)

// P4.3 ②③ shadow tile cache: a static DrawShadow is recorded once and the
// blurred silhouette is reused across frames. The cache lives in the render
// package (process-wide) because the Recorder (UI thread) must know a hit
// *before* it records CmdDrawShadow; the tile is published by the Context on a
// miss. Keys are content-based (path verbs/coords + user CTM + elevation +
// color + device size + dpr), never pointers: the recorder clones paths every
// frame and pointer keys would never hit.
//
// Resource discipline: the tile is an offscreen texture owned by the cache;
// RegisterTextureRef publishes the logical ref, ReleaseTextureRef + the
// CreateOffscreenTexture release drop it. Context.Close releases the entries it
// owns. Eviction is conservative (only entries untouched for two store/lookup
// generations) so a tile still sampled by the in-flight present is never
// returned to the pool early; when the cap is reached, new tiles are simply not
// stored (the shadow keeps rendering through the normal chain).

// ShadowTile is one cached device-space shadow tile: a recordable ref plus the
// device-space destination rect (the tight silhouette + lift + 3σ bounds,
// surface-cropped). It is returned by LookupShadowCache.
type ShadowTile struct {
	Ref  TextureRef
	X, Y int
	W, H int
}

type shadowCacheKey struct {
	pathHash  uint64
	matrix    [6]float64
	col       [4]float64
	elevation float64
	deviceW   int
	deviceH   int
	scale     float64
}

type shadowCacheEntry struct {
	ref     TextureRef
	bounds  image.Rectangle
	release func()
	owner   uint64
	last    uint64
}

var (
	shadowCacheMu      sync.Mutex
	shadowCacheEntries = make(map[shadowCacheKey]*shadowCacheEntry)
	shadowCacheSeq     uint64
	shadowCacheBytes   int64
	shadowCacheOwnerID atomic.Uint64
)

const (
	shadowCacheMaxEntries = 128
	shadowCacheMaxBytes   = 128 << 20 // 128 MiB
	// shadowCacheEvictAge is how many store/lookup generations an entry must be
	// untouched before its texture may go back to the pool. Every shadow hit or
	// publish bumps the generation, so a handful of shadows per frame makes a
	// low value sub-frame. 64 keeps an evicted tile at least several frames away
	// from an in-flight present that may still sample it.
	shadowCacheEvictAge = 64
)

// shadowCacheDisabled is the negative-control switch (mutation): with
// GPUI_P43_BREAK=no-cache the lookup and publish both turn into no-ops.
func shadowCacheDisabled() bool {
	return os.Getenv("GPUI_P43_BREAK") == "no-cache"
}

func shadowMatrix6(m Matrix) [6]float64 {
	return [6]float64{m.A, m.B, m.C, m.D, m.E, m.F}
}

// hashShadowPath hashes the path content (verbs + coords), never its pointer.
func hashShadowPath(p *Path) uint64 {
	h := fnv.New64a()
	var b [8]byte
	verbs := p.Verbs()
	binary.LittleEndian.PutUint32(b[:4], uint32(len(verbs))) //nolint:gosec // bounded
	h.Write(b[:4])
	for _, v := range verbs {
		b[0] = byte(v)
		h.Write(b[:1])
	}
	coords := p.Coords()
	binary.LittleEndian.PutUint32(b[:4], uint32(len(coords))) //nolint:gosec // bounded
	h.Write(b[:4])
	for _, c := range coords {
		binary.LittleEndian.PutUint64(b[:], math.Float64bits(c))
		h.Write(b[:])
	}
	return h.Sum64()
}

func shadowKeyFor(path *Path, ctm Matrix, col RGBA, elevation float64, deviceW, deviceH int, scale float64) shadowCacheKey {
	return shadowCacheKey{
		pathHash:  hashShadowPath(path),
		matrix:    shadowMatrix6(ctm),
		col:       [4]float64{col.R, col.G, col.B, col.A},
		elevation: elevation,
		deviceW:   deviceW,
		deviceH:   deviceH,
		scale:     scale,
	}
}

func shadowTileBytes(r image.Rectangle) int64 {
	if r.Dx() <= 0 || r.Dy() <= 0 {
		return 0
	}
	return int64(r.Dx()) * int64(r.Dy()) * 4
}

// lookupShadowTile is the key-based lookup shared by the Context hit path and
// the public recorder bridge.
func lookupShadowTile(key shadowCacheKey) (ShadowTile, bool) {
	if shadowCacheDisabled() {
		return ShadowTile{}, false
	}
	shadowCacheMu.Lock()
	e := shadowCacheEntries[key]
	if e != nil {
		shadowCacheSeq++
		e.last = shadowCacheSeq
	}
	shadowCacheMu.Unlock()
	if e == nil {
		return ShadowTile{}, false
	}
	if !e.ref.Valid() {
		dropShadowCacheEntry(key)
		return ShadowTile{}, false
	}
	r := e.bounds
	return ShadowTile{Ref: e.ref, X: r.Min.X, Y: r.Min.Y, W: r.Dx(), H: r.Dy()}, true
}

// LookupShadowCache returns a cached device-space shadow tile for the given
// shadow inputs. The Recorder calls it at record time; deviceW/deviceH are the
// recording's device surface size (record logical size * scale). ok=false when
// the cache is disabled, the tile was released, or nothing matches.
func LookupShadowCache(path *Path, ctm Matrix, col RGBA, elevation float64, deviceW, deviceH int, scale float64) (ShadowTile, bool) {
	if path == nil || path.NumVerbs() == 0 || elevation <= 0 || col.A <= 0 || deviceW <= 0 || deviceH <= 0 {
		return ShadowTile{}, false
	}
	if scale <= 0 {
		scale = 1
	}
	key := shadowKeyFor(path, ctm, col, elevation, deviceW, deviceH, scale)
	return lookupShadowTile(key)
}

// storeShadowTile publishes a tile for key. When the cache is full and no entry
// is old enough to evict safely, the tile is released immediately (the shadow
// keeps rendering through the normal chain; no correctness change).
func storeShadowTile(key shadowCacheKey, ref TextureRef, bounds image.Rectangle, release func(), owner uint64) {
	if shadowCacheDisabled() || !ref.Valid() || release == nil || bounds.Empty() {
		if release != nil {
			release()
		}
		ReleaseTextureRef(ref)
		return
	}
	add := shadowTileBytes(bounds)
	var evicted []*shadowCacheEntry
	shadowCacheMu.Lock()
	shadowCacheSeq++
	// Same key = same content. Keep the existing tile rather than releasing a
	// ref an in-flight present may still sample; only replace a stale entry.
	if old := shadowCacheEntries[key]; old != nil && old.ref.Valid() {
		shadowCacheMu.Unlock()
		ReleaseTextureRef(ref)
		release()
		return
	}
	if old := shadowCacheEntries[key]; old != nil {
		evicted = append(evicted, old)
		shadowCacheBytes -= shadowTileBytes(old.bounds)
		delete(shadowCacheEntries, key)
	}
	// Evict only entries untouched for shadowCacheEvictAge generations.
	for len(shadowCacheEntries) >= shadowCacheMaxEntries || shadowCacheBytes+add > shadowCacheMaxBytes {
		var oldestKey shadowCacheKey
		var oldest *shadowCacheEntry
		for k, e := range shadowCacheEntries {
			if shadowCacheSeq-e.last < shadowCacheEvictAge {
				continue
			}
			if oldest == nil || e.last < oldest.last {
				oldestKey, oldest = k, e
			}
		}
		if oldest == nil {
			break
		}
		evicted = append(evicted, oldest)
		shadowCacheBytes -= shadowTileBytes(oldest.bounds)
		delete(shadowCacheEntries, oldestKey)
	}
	stored := false
	if len(shadowCacheEntries) < shadowCacheMaxEntries && shadowCacheBytes+add <= shadowCacheMaxBytes {
		shadowCacheEntries[key] = &shadowCacheEntry{
			ref: ref, bounds: bounds, release: release, owner: owner, last: shadowCacheSeq,
		}
		shadowCacheBytes += add
		stored = true
	}
	shadowCacheMu.Unlock()
	if !stored {
		// Full: do not store this tile; release it now.
		ReleaseTextureRef(ref)
		release()
	}
	for _, e := range evicted {
		ReleaseTextureRef(e.ref)
		e.release()
	}
}

func dropShadowCacheEntry(key shadowCacheKey) {
	shadowCacheMu.Lock()
	e := shadowCacheEntries[key]
	if e != nil {
		delete(shadowCacheEntries, key)
		shadowCacheBytes -= shadowTileBytes(e.bounds)
	}
	shadowCacheMu.Unlock()
	if e != nil {
		ReleaseTextureRef(e.ref)
		e.release()
	}
}

// releaseShadowCacheOwner drops every tile published by one Context (Close).
func releaseShadowCacheOwner(owner uint64) {
	if owner == 0 {
		return
	}
	var drop []*shadowCacheEntry
	shadowCacheMu.Lock()
	for k, e := range shadowCacheEntries {
		if e.owner == owner {
			drop = append(drop, e)
			shadowCacheBytes -= shadowTileBytes(e.bounds)
			delete(shadowCacheEntries, k)
		}
	}
	shadowCacheMu.Unlock()
	for _, e := range drop {
		ReleaseTextureRef(e.ref)
		e.release()
	}
}

// shadowCacheOwner returns (lazily assigning) the per-Context owner token used
// by Close to release the tiles that Context published.
func (c *Context) shadowCacheOwner() uint64 {
	if c == nil {
		return 0
	}
	if c.shadowCacheOwnerID == 0 {
		c.shadowCacheOwnerID = shadowCacheOwnerID.Add(1)
	}
	return c.shadowCacheOwnerID
}

// shadowTileBounds returns the tight device-space tile rect: path device
// bounds lifted by elevation*dpr, padded by 3σ of the blur, cropped to the
// surface. The clip is intentionally NOT baked in: the blit applies the current
// clip, so a tile stays valid when the clip changes (and the key needs no clip).
func (c *Context) shadowTileBounds(path *Path, elevation float64) image.Rectangle {
	if c == nil || c.pixmap == nil || path == nil || elevation <= 0 {
		return image.Rectangle{}
	}
	dpr := c.DeviceScale()
	if dpr <= 0 {
		dpr = 1
	}
	b := path.Bounds()
	if b.Empty() {
		return image.Rectangle{}
	}
	m := c.totalMatrix()
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range [4]Point{
		{X: float64(b.Min.X), Y: float64(b.Min.Y)},
		{X: float64(b.Max.X), Y: float64(b.Min.Y)},
		{X: float64(b.Max.X), Y: float64(b.Max.Y)},
		{X: float64(b.Min.X), Y: float64(b.Max.Y)},
	} {
		q := m.TransformPoint(p)
		minX = math.Min(minX, q.X)
		maxX = math.Max(maxX, q.X)
		minY = math.Min(minY, q.Y)
		maxY = math.Max(maxY, q.Y)
	}
	lift := elevation * dpr
	pad := 3.0 * (800.0 / 600.0) * elevation * dpr
	r := image.Rect(
		int(math.Floor(minX-pad)),
		int(math.Floor(minY+lift-pad)),
		int(math.Ceil(maxX+pad)),
		int(math.Ceil(maxY+lift+pad)),
	)
	return r.Intersect(image.Rect(0, 0, c.pixmap.Width(), c.pixmap.Height()))
}

// publishShadowTile crops the materialized blurred layer pixmap into a new
// offscreen tile and stores it under key. Called from Context.DrawShadow after
// ApplyBlur (the layer pixmap holds the clean filter output at that point).
func (c *Context) publishShadowTile(key shadowCacheKey, bounds image.Rectangle) {
	if c == nil || c.pixmap == nil || bounds.Empty() || c.pixmapFilterStale {
		return
	}
	if shadowCacheDisabled() || key.elevation <= 0 {
		return
	}
	src := c.pixmap.Data()
	sw := c.pixmap.Width()
	tile := make([]byte, bounds.Dx()*bounds.Dy()*4)
	rowBytes := bounds.Dx() * 4
	for y := 0; y < bounds.Dy(); y++ {
		srcOff := (bounds.Min.Y+y)*sw*4 + bounds.Min.X*4
		copy(tile[y*rowBytes:(y+1)*rowBytes], src[srcOff:srcOff+rowBytes])
	}
	view, release := c.CreateOffscreenTexture(bounds.Dx(), bounds.Dy())
	if view.IsNil() || release == nil {
		if release != nil {
			release()
		}
		return
	}
	type viewUploader interface {
		UploadRGBAToView(view gpucontext.TextureView, data []byte, w, h int) error
	}
	raw := c.GPURenderContext()
	vu, ok := raw.(viewUploader)
	if !ok || vu == nil {
		release()
		return
	}
	if err := vu.UploadRGBAToView(view, tile, bounds.Dx(), bounds.Dy()); err != nil {
		release()
		return
	}
	ref := RegisterTextureRef(func() gpucontext.TextureView {
		if view.IsNil() {
			return gpucontext.TextureView{}
		}
		return view
	})
	if !ref.Valid() {
		release()
		return
	}
	storeShadowTile(key, ref, bounds, release, c.shadowCacheOwner())
}
