//go:build !nogpu

package gpu

import (
	"hash/fnv"
	"math"
	"sync"

	"github.com/energye/gpui/render"
)

// defaultPathGeomBudget is max tessellation entries (S4.3).
const defaultPathGeomBudget = 256

// defaultStrokeGeomBudget is max stroke-expansion entries (S4.3).
const defaultStrokeGeomBudget = 128

// pathTessKey identifies a tessellated fill path.
type pathTessKey struct {
	hash     uint64
	fillRule render.FillRule
	aaOff    bool // anti-alias disabled → pixel-snapped geometry
}

// pathTessEntry holds fan-tessellated geometry for stencil-then-cover.
type pathTessEntry struct {
	vertices  []float32
	coverQuad [12]float32
	gen       uint64
}

// PathGeometryCache reuses path tessellation across draws/frames (S4.3).
// Not shared across threads; one instance per GPUShared (or per session).
type PathGeometryCache struct {
	mu      sync.Mutex
	entries map[pathTessKey]*pathTessEntry
	budget  int
	gen     uint64
	hits    uint64
	misses  uint64
}

// NewPathGeometryCache creates an empty tessellation cache.
func NewPathGeometryCache() *PathGeometryCache {
	return &PathGeometryCache{
		entries: make(map[pathTessKey]*pathTessEntry, 64),
		budget:  defaultPathGeomBudget,
	}
}

// GetOrTessellate returns fan vertices for path, computing on miss.
func (c *PathGeometryCache) GetOrTessellate(path *render.Path, fillRule render.FillRule, aaOff bool) (verts []float32, cover [12]float32, ok bool) {
	if c == nil || path == nil || path.NumVerbs() == 0 {
		return nil, cover, false
	}
	key := pathTessKey{
		hash:     hashPathContent(path),
		fillRule: fillRule,
		aaOff:    aaOff,
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, found := c.entries[key]; found {
		c.gen++
		e.gen = c.gen
		c.hits++
		// Return copies so callers can own/mutate safely.
		out := make([]float32, len(e.vertices))
		copy(out, e.vertices)
		return out, e.coverQuad, true
	}

	c.misses++
	tess := NewFanTessellator()
	tess.TessellatePath(path)
	fv := tess.Vertices()
	if len(fv) == 0 {
		return nil, cover, false
	}
	stored := make([]float32, len(fv))
	copy(stored, fv)
	cq := tess.CoverQuad()

	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	c.gen++
	c.entries[key] = &pathTessEntry{vertices: stored, coverQuad: cq, gen: c.gen}

	out := make([]float32, len(stored))
	copy(out, stored)
	return out, cq, true
}

// Stats returns hit/miss/entry counts.
func (c *PathGeometryCache) Stats() (hits, misses uint64, entries int) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, len(c.entries)
}

// Clear drops all entries.
func (c *PathGeometryCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[pathTessKey]*pathTessEntry, 64)
}

func (c *PathGeometryCache) evictOldestLocked() {
	var oldest pathTessKey
	var oldestGen uint64 = ^uint64(0)
	found := false
	for k, e := range c.entries {
		if !found || e.gen < oldestGen {
			oldest, oldestGen, found = k, e.gen, true
		}
	}
	if found {
		delete(c.entries, oldest)
	}
}

// strokeCacheKey identifies a stroke expansion.
type strokeCacheKey struct {
	pathHash  uint64
	widthBits uint64
	cap       int
	join      int
	miterBits uint64
	dashHash  uint64
	aaOff     bool
}

type strokeCacheEntry struct {
	// Expanded path as verbs/coords for FillPath.
	verbs  []render.PathVerb
	coords []float64
	gen    uint64
}

// StrokeGeometryCache caches stroke expansion results (S4.3).
type StrokeGeometryCache struct {
	mu      sync.Mutex
	entries map[strokeCacheKey]*strokeCacheEntry
	budget  int
	gen     uint64
	hits    uint64
	misses  uint64
}

// NewStrokeGeometryCache creates an empty stroke expansion cache.
func NewStrokeGeometryCache() *StrokeGeometryCache {
	return &StrokeGeometryCache{
		entries: make(map[strokeCacheKey]*strokeCacheEntry, 32),
		budget:  defaultStrokeGeomBudget,
	}
}

// Stats returns hit/miss/entry counts.
func (c *StrokeGeometryCache) Stats() (hits, misses uint64, entries int) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, len(c.entries)
}

// Get returns a cloned expanded path if present.
func (c *StrokeGeometryCache) Get(key strokeCacheKey) (*render.Path, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		c.misses++
		return nil, false
	}
	c.gen++
	e.gen = c.gen
	c.hits++
	return pathFromVerbsCoords(e.verbs, e.coords), true
}

// Put stores an expanded path under key.
func (c *StrokeGeometryCache) Put(key strokeCacheKey, p *render.Path) {
	if c == nil || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	verbs := append([]render.PathVerb(nil), p.Verbs()...)
	coords := append([]float64(nil), p.Coords()...)
	c.gen++
	c.entries[key] = &strokeCacheEntry{verbs: verbs, coords: coords, gen: c.gen}
}

func (c *StrokeGeometryCache) evictOldestLocked() {
	var oldest strokeCacheKey
	var oldestGen uint64 = ^uint64(0)
	found := false
	for k, e := range c.entries {
		if !found || e.gen < oldestGen {
			oldest, oldestGen, found = k, e.gen, true
		}
	}
	if found {
		delete(c.entries, oldest)
	}
}

func pathFromVerbsCoords(verbs []render.PathVerb, coords []float64) *render.Path {
	p := render.NewPath()
	// Replay via Append from a temporary path built with low-level if available.
	// Path has no public SetVerbs; use NewPath + MoveTo/LineTo/etc via replay.
	ci := 0
	for _, v := range verbs {
		switch v {
		case render.MoveTo:
			if ci+1 < len(coords) {
				p.MoveTo(coords[ci], coords[ci+1])
				ci += 2
			}
		case render.LineTo:
			if ci+1 < len(coords) {
				p.LineTo(coords[ci], coords[ci+1])
				ci += 2
			}
		case render.QuadTo:
			if ci+3 < len(coords) {
				p.QuadraticTo(coords[ci], coords[ci+1], coords[ci+2], coords[ci+3])
				ci += 4
			}
		case render.CubicTo:
			if ci+5 < len(coords) {
				p.CubicTo(coords[ci], coords[ci+1], coords[ci+2], coords[ci+3], coords[ci+4], coords[ci+5])
				ci += 6
			}
		case render.Close:
			p.Close()
		}
	}
	return p
}

func hashPathContent(path *render.Path) uint64 {
	h := fnv.New64a()
	verbs := path.Verbs()
	coords := path.Coords()
	var vb [1]byte
	for _, v := range verbs {
		vb[0] = byte(v)
		_, _ = h.Write(vb[:])
	}
	var buf [8]byte
	for _, c := range coords {
		u := math.Float64bits(c)
		buf[0] = byte(u)
		buf[1] = byte(u >> 8)
		buf[2] = byte(u >> 16)
		buf[3] = byte(u >> 24)
		buf[4] = byte(u >> 32)
		buf[5] = byte(u >> 40)
		buf[6] = byte(u >> 48)
		buf[7] = byte(u >> 56)
		_, _ = h.Write(buf[:])
	}
	return h.Sum64()
}

func hashDash(dash *render.Dash) uint64 {
	if dash == nil || !dash.IsDashed() {
		return 0
	}
	h := fnv.New64a()
	var buf [8]byte
	writeF64 := func(v float64) {
		u := math.Float64bits(v)
		for i := 0; i < 8; i++ {
			buf[i] = byte(u >> (8 * i))
		}
		_, _ = h.Write(buf[:])
	}
	writeF64(dash.Offset)
	for _, v := range dash.Array {
		writeF64(v)
	}
	return h.Sum64()
}

func makeStrokeCacheKey(path *render.Path, paint *render.Paint, aaOff bool, dashHash uint64) strokeCacheKey {
	w := 1.0
	cap := 0
	join := 0
	miter := 4.0
	if paint != nil {
		w = effectiveStrokeWidth(paint)
		cap = int(paint.EffectiveLineCap())
		join = int(paint.EffectiveLineJoin())
		miter = paint.EffectiveMiterLimit()
	}
	return strokeCacheKey{
		pathHash:  hashPathContent(path),
		widthBits: math.Float64bits(w),
		cap:       cap,
		join:      join,
		miterBits: math.Float64bits(miter),
		dashHash:  dashHash,
		aaOff:     aaOff,
	}
}
