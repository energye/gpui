//go:build !nogpu

package gpu

import (
	"hash/fnv"
	"math"
	"sync"

	"github.com/energye/gpui/render"
)

// S4.3 budgets raised in S6.6 for retained UI frames with many unique paths.
const (
	defaultPathGeomBudget    = 512
	defaultStrokeGeomBudget  = 256
	defaultDashGeomBudget    = 256
	defaultConvexClassBudget = 512
)

// pathTessKey identifies a tessellated fill path.
type pathTessKey struct {
	hash     uint64
	fillRule render.FillRule
	aaOff    bool // anti-alias disabled → pixel-snapped geometry slot
}

// pathTessEntry holds fan-tessellated geometry for stencil-then-cover.
// vertices are immutable after insert (S6.6 zero-copy hit).
type pathTessEntry struct {
	vertices  []float32
	coverQuad [12]float32
	gen       uint64
}

// PathGeometryCache reuses path tessellation across draws/frames (S4.3/S6.6).
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
//
// S6.6: on hit, returns the cached slice directly (zero-copy). Callers must
// treat the returned vertices as immutable. StencilPathCommand / flush only
// read vertices into GPU buffers.
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
		return e.vertices, e.coverQuad, true
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
	// Return the stored slice (same immutability contract as hit).
	return stored, cq, true
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
	c.hits = 0
	c.misses = 0
}

// ResetStats clears hit/miss counters without dropping entries.
func (c *PathGeometryCache) ResetStats() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
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
	// path is an immutable clone of the expanded outline (S6.6 shared hit).
	path *render.Path
	gen  uint64
}

// StrokeGeometryCache caches stroke expansion results (S4.3/S6.6).
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

// Get returns the shared expanded path if present.
// S6.6: no clone on hit — callers must not mutate the returned path.
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
	return e.path, true
}

// Put stores an expanded path under key (cloned once).
func (c *StrokeGeometryCache) Put(key strokeCacheKey, p *render.Path) {
	if c == nil || p == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	c.gen++
	c.entries[key] = &strokeCacheEntry{path: p.Clone(), gen: c.gen}
}

// Clear drops all entries and stats.
func (c *StrokeGeometryCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[strokeCacheKey]*strokeCacheEntry, 32)
	c.hits = 0
	c.misses = 0
}

// ResetStats clears hit/miss counters.
func (c *StrokeGeometryCache) ResetStats() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
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

// ---------------------------------------------------------------------------
// S6.6 Dash geometry cache — avoid re-running ApplyDash on retained frames
// ---------------------------------------------------------------------------

type dashGeomKey struct {
	pathHash  uint64
	dashHash  uint64
	scaleBits uint64
}

type dashGeomEntry struct {
	path *render.Path
	gen  uint64
}

// DashGeometryCache caches dashed path expansions (S6.6).
type DashGeometryCache struct {
	mu      sync.Mutex
	entries map[dashGeomKey]*dashGeomEntry
	budget  int
	gen     uint64
	hits    uint64
	misses  uint64
}

// NewDashGeometryCache creates an empty dash geometry cache.
func NewDashGeometryCache() *DashGeometryCache {
	return &DashGeometryCache{
		entries: make(map[dashGeomKey]*dashGeomEntry, 32),
		budget:  defaultDashGeomBudget,
	}
}

// GetOrApply returns a dashed path for (path, dash, scale), computing on miss.
// Returned path is immutable shared storage.
func (c *DashGeometryCache) GetOrApply(path *render.Path, dash *render.Dash, transformScale float64) *render.Path {
	if c == nil || path == nil || dash == nil || !dash.IsDashed() {
		return nil
	}
	if transformScale <= 0 {
		transformScale = 1
	}
	key := dashGeomKey{
		pathHash:  hashPathContent(path),
		dashHash:  hashDash(dash),
		scaleBits: math.Float64bits(transformScale),
	}

	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.gen++
		e.gen = c.gen
		c.hits++
		out := e.path
		c.mu.Unlock()
		return out
	}
	c.mu.Unlock()

	// Apply outside lock (ApplyDash can be heavy).
	d := dash
	if transformScale > 1.0 {
		d = dash.Scale(transformScale)
	}
	dashed := render.ApplyDash(path, d)
	if dashed == nil || dashed.NumVerbs() == 0 {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return dashed
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		c.gen++
		e.gen = c.gen
		c.hits++
		return e.path
	}
	c.misses++
	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	c.gen++
	stored := dashed.Clone()
	c.entries[key] = &dashGeomEntry{path: stored, gen: c.gen}
	return stored
}

// Stats returns hit/miss/entry counts.
func (c *DashGeometryCache) Stats() (hits, misses uint64, entries int) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, len(c.entries)
}

// Clear drops all entries.
func (c *DashGeometryCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[dashGeomKey]*dashGeomEntry, 32)
	c.hits = 0
	c.misses = 0
}

// ResetStats clears hit/miss counters.
func (c *DashGeometryCache) ResetStats() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
}

func (c *DashGeometryCache) evictOldestLocked() {
	var oldest dashGeomKey
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

// ---------------------------------------------------------------------------
// S6.6 Convex classification cache — skip re-walk + IsConvex on hot paths
// ---------------------------------------------------------------------------

type convexClassEntry struct {
	ok     bool
	points []render.Point // immutable after insert when ok
	gen    uint64
}

// ConvexPathCache caches extractConvexPolygon results by path content hash.
type ConvexPathCache struct {
	mu      sync.Mutex
	entries map[uint64]*convexClassEntry
	budget  int
	gen     uint64
	hits    uint64
	misses  uint64
}

// NewConvexPathCache creates an empty convex classification cache.
func NewConvexPathCache() *ConvexPathCache {
	return &ConvexPathCache{
		entries: make(map[uint64]*convexClassEntry, 64),
		budget:  defaultConvexClassBudget,
	}
}

// GetOrClassify returns convex polygon points when path is a simple convex
// closed polyline. Negative results (non-convex / curves / multi-contour) are
// also cached to avoid repeated walks.
func (c *ConvexPathCache) GetOrClassify(path *render.Path) ([]render.Point, bool) {
	if c == nil || path == nil || path.NumVerbs() == 0 {
		return nil, false
	}
	key := hashPathContent(path)

	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.gen++
		e.gen = c.gen
		c.hits++
		pts, isConvex := e.points, e.ok
		c.mu.Unlock()
		if !isConvex {
			return nil, false
		}
		return pts, true
	}
	c.mu.Unlock()

	pts, isConvex := extractConvexPolygon(path)

	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[key]; ok {
		c.gen++
		e.gen = c.gen
		c.hits++
		if !e.ok {
			return nil, false
		}
		return e.points, true
	}
	c.misses++
	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	var stored []render.Point
	if isConvex && len(pts) > 0 {
		stored = make([]render.Point, len(pts))
		copy(stored, pts)
	}
	c.gen++
	c.entries[key] = &convexClassEntry{ok: isConvex, points: stored, gen: c.gen}
	if !isConvex {
		return nil, false
	}
	return stored, true
}

// Stats returns hit/miss/entry counts.
func (c *ConvexPathCache) Stats() (hits, misses uint64, entries int) {
	if c == nil {
		return 0, 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.misses, len(c.entries)
}

// Clear drops all entries.
func (c *ConvexPathCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[uint64]*convexClassEntry, 64)
	c.hits = 0
	c.misses = 0
}

// ResetStats clears hit/miss counters.
func (c *ConvexPathCache) ResetStats() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits = 0
	c.misses = 0
}

func (c *ConvexPathCache) evictOldestLocked() {
	var oldest uint64
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

// GeometryCacheStats aggregates S4.3/S6.6 path/stroke/dash/convex cache stats.
type GeometryCacheStats struct {
	PathHits, PathMisses     uint64
	PathEntries              int
	StrokeHits, StrokeMisses uint64
	StrokeEntries            int
	DashHits, DashMisses     uint64
	DashEntries              int
	ConvexHits, ConvexMisses uint64
	ConvexEntries            int
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
