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

// lruList is the intrusive LRU shared by the four geometry caches: the
// map owns keys, each entry carries its node, refresh/evict are O(1) pointer
// ops with no per-frame allocation on hit. Go generics keep one
// implementation for the four key types (pathTessKey, strokeCacheKey,
// dashGeomKey, uint64 convex hash); instantiated K appears only in the node
// key field, so codegen is four tiny pointer-shuffling copies, no
// interface boxing on the hot path.
type lruList[K comparable] struct {
	front, back *lruNode[K]
}

type lruNode[K comparable] struct {
	key        K
	prev, next *lruNode[K]
}

func (l *lruList[K]) pushFront(n *lruNode[K]) {
	n.prev, n.next = nil, l.front
	if l.front != nil {
		l.front.prev = n
	}
	l.front = n
	if l.back == nil {
		l.back = n
	}
}

func (l *lruList[K]) moveFront(n *lruNode[K]) {
	if l.front == n {
		return
	}
	if n.prev != nil {
		n.prev.next = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	}
	if l.back == n {
		l.back = n.prev
	}
	l.pushFront(n)
}

func (l *lruList[K]) popBack() *lruNode[K] {
	n := l.back
	if n == nil {
		return nil
	}
	if n.prev != nil {
		n.prev.next = nil
	}
	l.back = n.prev
	if l.back == nil {
		l.front = nil
	}
	n.prev, n.next = nil, nil
	return n
}

// pathTessLRU keeps the historical name; it is lruList[pathTessKey].
type pathTessLRU = lruList[pathTessKey]
type pathTessNode = lruNode[pathTessKey]

// pathTessKey identifies a tessellated fill path.
// scaleBits: tessellation tolerance/band widths are scale-relative, so
// the same user-space shape at different scales occupies distinct entries.
type pathTessKey struct {
	hash      uint64
	fillRule  render.FillRule
	aaOff     bool // anti-alias disabled → pixel-snapped geometry slot
	scaleBits uint64
}

// pathTessEntry holds fan-tessellated geometry for stencil-then-cover.
// vertices are immutable after insert (S6.6 zero-copy hit).
type pathTessEntry struct {
	vertices  []float32
	coverQuad [12]float32
	// Analytic-AA fringe bands (sampleCount==1): exterior + interior halves
	// as (x, y, signedEdgeDist) triples. Empty until an AA request misses.
	bandAA      []float32
	innerBandAA []float32
	gen         uint64
	lru         *pathTessNode // owned by PathGeometryCache.lru while in entries
}

// PathGeometryCache reuses path tessellation across draws/frames (S4.3/S6.6).
// Eviction is LRU via an intrusive list — insert/refresh/evict are O(1).
// The map still owns keys; each entry carries its list element so refresh on
// hit and removal on evict never scan the table (the old full-table oldest-gen
// scan stalled animation frames that churn unique paths every frame).
type PathGeometryCache struct {
	mu      sync.Mutex
	entries map[pathTessKey]*pathTessEntry
	lru     pathTessLRU
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
	if path == nil || path.NumVerbs() == 0 {
		return nil, cover, false
	}
	return c.GetOrTessellateKeyed(hashPathContent(path), fillRule, aaOff, 1, func() *render.Path { return path })
}

// GetOrTessellateKeyed is GetOrTessellate with a caller-supplied content hash
// and user scale. F2: preHash may be the unbaked (user-space) hash of a
// device-space path, in which case raw materializes the user-space path and
// is only invoked on miss (deferred closure — hits never allocate).
// userScale scales the flatten tolerance (device px → user units); 1 = device.
func (c *PathGeometryCache) GetOrTessellateKeyed(preHash uint64, fillRule render.FillRule, aaOff bool, userScale float64, raw func() *render.Path) (verts []float32, cover [12]float32, ok bool) {
	v, cq, _, _, ok := c.GetOrTessellateAAKeyed(preHash, fillRule, aaOff, false, userScale, raw)
	return v, cq, ok
}

// GetOrTessellateAA returns fan vertices plus the analytic-AA cover meshes
// (bandAA, innerBandAA — empty when wantAA is false or the path does not
// need AA). The AA fringe bands are tessellated lazily on the first AA miss
// and then cached; the base fan/cover are shared with GetOrTessellate.
func (c *PathGeometryCache) GetOrTessellateAA(path *render.Path, fillRule render.FillRule, aaOff, wantAA bool) (verts []float32, cover [12]float32, bandAA, innerBandAA []float32, ok bool) {
	if path == nil || path.NumVerbs() == 0 {
		return nil, cover, nil, nil, false
	}
	return c.GetOrTessellateAAKeyed(hashPathContent(path), fillRule, aaOff, wantAA, 1, func() *render.Path { return path })
}

// GetOrTessellateAAKeyed is the keyed variant (see GetOrTessellateKeyed).
// A nil cache, nil raw result, or empty path misses (returns ok=false);
// raw is only invoked when the entry (or its lazy AA bands) is absent.
func (c *PathGeometryCache) GetOrTessellateAAKeyed(preHash uint64, fillRule render.FillRule, aaOff, wantAA bool, userScale float64, raw func() *render.Path) (verts []float32, cover [12]float32, bandAA, innerBandAA []float32, ok bool) {
	if c == nil || raw == nil {
		return nil, cover, nil, nil, false
	}
	// Quantize the scale for keying AND tessellation: per-frame rotation
	// preserves norms only up to float64 rounding — unquantized bits would
	// miss every frame with zero visual difference.
	userScale = quantizeScale(userScale)
	if !(userScale > 1e-9) {
		userScale = 1
	}
	key := pathTessKey{
		hash:      preHash,
		fillRule:  fillRule,
		aaOff:     aaOff,
		scaleBits: math.Float64bits(userScale),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, found := c.entries[key]; found {
		c.gen++
		e.gen = c.gen
		c.lru.moveFront(e.lru)
		c.hits++
		if wantAA && len(e.bandAA) == 0 {
			r := raw()
			if r == nil || r.NumVerbs() == 0 {
				return nil, cover, nil, nil, false
			}
			c.tessellateAALocked(e, r, userScale)
		}
		return e.vertices, e.coverQuad, e.bandAA, e.innerBandAA, true
	}

	c.misses++
	r := raw()
	if r == nil || r.NumVerbs() == 0 {
		return nil, cover, nil, nil, false
	}
	tess := NewFanTessellator()
	tess.SetUserScale(userScale)
	tess.TessellatePath(r)
	fv := tess.Vertices()
	if len(fv) == 0 {
		return nil, cover, nil, nil, false
	}
	stored := make([]float32, len(fv))
	copy(stored, fv)
	cq := tess.CoverQuad()

	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	c.gen++
	e := &pathTessEntry{vertices: stored, coverQuad: cq, gen: c.gen}
	e.lru = &pathTessNode{key: key}
	c.lru.pushFront(e.lru)
	c.entries[key] = e
	if wantAA {
		c.tessellateAALocked(e, r, userScale)
	}
	return e.vertices, e.coverQuad, e.bandAA, e.innerBandAA, true
}

// tessellateAALocked generates the analytic-AA fringe bands into an existing
// entry. Caller holds c.mu.
func (c *PathGeometryCache) tessellateAALocked(e *pathTessEntry, path *render.Path, userScale float64) {
	tess := NewFanTessellator()
	tess.SetUserScale(userScale)
	tess.TessellateAA(path)
	if len(tess.bandVerts) == 0 {
		return
	}
	e.bandAA = append([]float32(nil), tess.bandVerts...)
	e.innerBandAA = append([]float32(nil), tess.innerBandVerts...)
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
	c.lru = pathTessLRU{}
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
	// F1: O(1) LRU evict — the back of the list is the oldest.
	// The node may alias a newer key only if the same node were reinserted,
	// which never happens (one node per entry, removed with its entry).
	if n := c.lru.popBack(); n != nil {
		delete(c.entries, n.key)
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
	lru  *lruNode[strokeCacheKey] // owned by StrokeGeometryCache.lru while in entries
}

// StrokeGeometryCache caches stroke expansion results (S4.3/S6.6).
// Eviction is LRU via an intrusive list — Get/Put/evict are O(1).
type StrokeGeometryCache struct {
	mu      sync.Mutex
	entries map[strokeCacheKey]*strokeCacheEntry
	lru     lruList[strokeCacheKey]
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
	c.lru.moveFront(e.lru)
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
	c.entries[key] = &strokeCacheEntry{path: p.Clone(), gen: c.gen, lru: &lruNode[strokeCacheKey]{key: key}}
	c.lru.pushFront(c.entries[key].lru)
}

// Clear drops all entries and stats.
func (c *StrokeGeometryCache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[strokeCacheKey]*strokeCacheEntry, 32)
	c.lru = lruList[strokeCacheKey]{}
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
	// O(1) LRU evict.
	if n := c.lru.popBack(); n != nil {
		delete(c.entries, n.key)
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
	lru  *lruNode[dashGeomKey] // owned by DashGeometryCache.lru while in entries
}

// DashGeometryCache caches dashed path expansions (S6.6).
// Eviction is LRU via an intrusive list — GetOrApply/evict are O(1).
type DashGeometryCache struct {
	mu      sync.Mutex
	entries map[dashGeomKey]*dashGeomEntry
	lru     lruList[dashGeomKey]
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
		c.lru.moveFront(e.lru)
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
		c.lru.moveFront(e.lru)
		c.hits++
		return e.path
	}
	c.misses++
	if len(c.entries) >= c.budget {
		c.evictOldestLocked()
	}
	c.gen++
	stored := dashed.Clone()
	e := &dashGeomEntry{path: stored, gen: c.gen, lru: &lruNode[dashGeomKey]{key: key}}
	c.lru.pushFront(e.lru)
	c.entries[key] = e
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
	c.lru = lruList[dashGeomKey]{}
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
	// O(1) LRU evict.
	if n := c.lru.popBack(); n != nil {
		delete(c.entries, n.key)
	}
}

// ---------------------------------------------------------------------------
// S6.6 Convex classification cache — skip re-walk + IsConvex on hot paths
// ---------------------------------------------------------------------------

type convexClassEntry struct {
	ok     bool
	points []render.Point // immutable after insert when ok
	gen    uint64
	lru    *lruNode[uint64] // owned by ConvexPathCache.lru while in entries
}

// ConvexPathCache caches extractConvexPolygon results by path content hash.
// Eviction is LRU via an intrusive list — GetOrClassify/evict are O(1).
type ConvexPathCache struct {
	mu      sync.Mutex
	entries map[uint64]*convexClassEntry
	lru     lruList[uint64]
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
		c.lru.moveFront(e.lru)
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
		c.lru.moveFront(e.lru)
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
	e := &convexClassEntry{ok: isConvex, points: stored, gen: c.gen, lru: &lruNode[uint64]{key: key}}
	c.lru.pushFront(e.lru)
	c.entries[key] = e
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
	c.lru = lruList[uint64]{}
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
	// O(1) LRU evict.
	if n := c.lru.popBack(); n != nil {
		delete(c.entries, n.key)
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
	return makeStrokeCacheKeyHashed(hashPathContent(path), w, cap, join, miter, dashHash, aaOff)
}

// makeStrokeCacheKeyHashed builds a stroke key from a precomputed path hash
// (F2: unbaked user-space hash — hits across pure-transform animation).
func makeStrokeCacheKeyHashed(preHash uint64, width float64, cap, join int, miter float64, dashHash uint64, aaOff bool) strokeCacheKey {
	return strokeCacheKey{
		pathHash:  preHash,
		widthBits: math.Float64bits(width),
		cap:       cap,
		join:      join,
		miterBits: math.Float64bits(miter),
		dashHash:  dashHash,
		aaOff:     aaOff,
	}
}
