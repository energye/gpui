package text

import (
	"fmt"
	"hash/fnv"
	"math"
	"sync"
	"sync/atomic"
)

// shapeResultMode selects which layout pipeline produced the cached glyphs.
type shapeResultMode uint8

const (
	// shapeModeLayout is Face.Glyphs → ShapedGlyph (cmap + advance; GPU LayoutText path).
	shapeModeLayout shapeResultMode = 0
	// shapeModeOT is full OpenType Shape() (GSUB/GPOS via global Shaper).
	shapeModeOT shapeResultMode = 1
)

// shapeResultKey identifies a shaped/layout glyph run for caching.
// All parameters that affect positions/glyph IDs must be part of the key.
type shapeResultKey struct {
	textHash  uint64
	fontID    uint64
	sizeBits  uint32
	direction uint8
	features  uint64
	langHash  uint64
	varHash   uint64
	mode      shapeResultMode
}

// ShapeResultStats reports global shape/layout result cache counters (S6.5).
type ShapeResultStats struct {
	Hits      uint64
	Misses    uint64
	Entries   int
	SoftLimit int
	Evictions uint64
}

// shapeResultCache is a process-wide soft-LRU for layout/shape glyph runs.
// Cached slices are immutable after insertion — callers must not mutate them.
type shapeResultCache struct {
	mu        sync.Mutex
	entries   map[shapeResultKey]*shapeResultEntry
	softLimit int
	tick      int64
	// missSinceSweep counts inserts since the last never-hit sweep (see
	// evictChurnLocked): bounds the sweep frequency so entries get a real
	// hit window while churn stays short-lived.
	missSinceSweep int
	hits           atomic.Uint64
	misses         atomic.Uint64
	evictions      atomic.Uint64
}

type shapeResultEntry struct {
	glyphs []ShapedGlyph
	atime  int64
}

const defaultShapeResultSoftLimit = 4096

// missSweepWindow bounds how many miss inserts happen between never-hit
// sweeps. At 60fps this keeps per-frame-unique churn alive ~1s (≤ ~64
// entries) while giving alternating layout/shape reuse a real hit window.
const missSweepWindow = 64

var globalShapeResultCache = newShapeResultCache(defaultShapeResultSoftLimit)

func newShapeResultCache(softLimit int) *shapeResultCache {
	if softLimit <= 0 {
		softLimit = defaultShapeResultSoftLimit
	}
	return &shapeResultCache{
		entries:   make(map[shapeResultKey]*shapeResultEntry),
		softLimit: softLimit,
	}
}

// ShapeResultCacheStats returns counters for the global shape/layout result cache.
func ShapeResultCacheStats() ShapeResultStats {
	return globalShapeResultCache.stats()
}

// ResetShapeResultCacheStats clears hit/miss/eviction counters (entries retained).
func ResetShapeResultCacheStats() {
	globalShapeResultCache.resetStats()
}

// ClearShapeResultCache drops all cached shape/layout runs and resets stats.
func ClearShapeResultCache() {
	globalShapeResultCache.clear()
}

func (c *shapeResultCache) stats() ShapeResultStats {
	c.mu.Lock()
	n := len(c.entries)
	lim := c.softLimit
	c.mu.Unlock()
	return ShapeResultStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Entries:   n,
		SoftLimit: lim,
		Evictions: c.evictions.Load(),
	}
}

func (c *shapeResultCache) resetStats() {
	c.hits.Store(0)
	c.misses.Store(0)
	c.evictions.Store(0)
}

func (c *shapeResultCache) clear() {
	c.mu.Lock()
	c.entries = make(map[shapeResultKey]*shapeResultEntry)
	c.tick = 0
	c.mu.Unlock()
	c.resetStats()
}

func (c *shapeResultCache) get(key shapeResultKey) ([]ShapedGlyph, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	c.tick++
	e.atime = c.tick
	return e.glyphs, true
}

func (c *shapeResultCache) set(key shapeResultKey, glyphs []ShapedGlyph) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tick++
	// Miss-inserted entries start at atime 0 so never-hit entries
	// (per-frame-unique HUD telemetry) become the first eviction candidates
	// — LRU with a never-hit fast lane (Skia/Flutter caches are LRU +
	// budget; this biases the same LRU toward entries proven to be reused).
	// A later hit upgrades atime and makes the entry resident.
	c.entries[key] = &shapeResultEntry{glyphs: glyphs, atime: 0}
	c.missSinceSweep++
	if c.missSinceSweep >= missSweepWindow {
		c.missSinceSweep = 0
		c.evictChurnLocked(key)
	}
	if c.softLimit > 0 && len(c.entries) > c.softLimit {
		c.evictOldestLocked()
	}
}

func (c *shapeResultCache) getOrCreate(key shapeResultKey, create func() []ShapedGlyph) []ShapedGlyph {
	// Fast path: hit under lock
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.tick++
		e.atime = c.tick
		g := e.glyphs
		c.mu.Unlock()
		c.hits.Add(1)
		return g
	}
	c.mu.Unlock()

	// Create outside lock (shaping can be expensive).
	glyphs := create()
	if glyphs == nil {
		// Distinguish empty result from nil: store empty non-nil slice for hits.
		glyphs = []ShapedGlyph{}
	}

	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		// Race: another goroutine filled it.
		c.tick++
		e.atime = c.tick
		g := e.glyphs
		c.mu.Unlock()
		c.hits.Add(1)
		return g
	}
	c.tick++
	// Miss insert: atime 0 so never-hit entries (per-frame unique keys) are
	// the first eviction candidates; a later hit upgrades them to resident.
	c.entries[key] = &shapeResultEntry{glyphs: glyphs, atime: 0}
	// Sweep stale never-hit entries on a miss counter: the sweep skips the
	// current key and runs at most every missSweepWindow inserts — so an
	// entry gets a real hit window (a layout/shape alternating text reuses
	// its key within that window) while per-frame-unique HUD telemetry still
	// lives only ~1s instead of pinning a slot until a batch eviction.
	c.missSinceSweep++
	if c.missSinceSweep >= missSweepWindow {
		c.missSinceSweep = 0
		c.evictChurnLocked(key)
	}
	if c.softLimit > 0 && len(c.entries) > c.softLimit {
		c.evictOldestLocked()
	}
	c.mu.Unlock()
	c.misses.Add(1)
	return glyphs
}

// evictChurnLocked drops every never-hit entry (atime 0) except skip — the
// entry inserted by the current call is given a hit window before it
// qualifies as garbage. Caller holds c.mu.
func (c *shapeResultCache) evictChurnLocked(skip shapeResultKey) {
	for k, e := range c.entries {
		if e.atime == 0 && k != skip {
			delete(c.entries, k)
			c.evictions.Add(1)
		}
	}
}

func (c *shapeResultCache) evictOldestLocked() {
	target := c.softLimit * 3 / 4
	if target < 1 {
		target = 1
	}
	// Drop every never-hit entry first (atime 0 — per-frame-unique churn like
	// HUD telemetry): they are garbage by definition and clearing them in one
	// pass collapses the cache back to the resident (hit) set.
	c.evictChurnLocked(shapeResultKey{})
	if len(c.entries) <= target {
		return
	}
	toEvict := len(c.entries) - target
	type pair struct {
		key   shapeResultKey
		atime int64
	}
	// Linear scan oldest: fine for occasional eviction batches.
	for i := 0; i < toEvict; i++ {
		var oldestKey shapeResultKey
		var oldestAtime int64 = math.MaxInt64
		found := false
		for k, e := range c.entries {
			if e.atime < oldestAtime {
				oldestAtime = e.atime
				oldestKey = k
				found = true
			}
		}
		if !found {
			break
		}
		delete(c.entries, oldestKey)
		c.evictions.Add(1)
	}
}

// FontSourceID returns a stable hash identity for a FontSource (S6.5 cache keys).
func FontSourceID(source *FontSource) uint64 {
	if source == nil {
		return 0
	}
	h := fnv.New64a()
	parsed := source.Parsed()
	if parsed != nil {
		fullName := parsed.FullName()
		if fullName == "" {
			fullName = source.Name()
		}
		_, _ = fmt.Fprintf(h, "%s:%d", fullName, parsed.NumGlyphs())
		return h.Sum64()
	}
	_, _ = h.Write([]byte(source.Name()))
	return h.Sum64()
}

// HashFontFeatures hashes OpenType feature settings for cache keys.
func HashFontFeatures(features []FontFeature) uint64 {
	if len(features) == 0 {
		return 0
	}
	// Order-independent XOR of per-feature hashes.
	var result uint64
	h := fnv.New64a()
	for _, f := range features {
		h.Reset()
		_, _ = h.Write(f.Tag[:])
		var vb [4]byte
		vb[0] = byte(f.Value)
		vb[1] = byte(f.Value >> 8)
		vb[2] = byte(f.Value >> 16)
		vb[3] = byte(f.Value >> 24)
		_, _ = h.Write(vb[:])
		result ^= h.Sum64()
	}
	return result
}

// HashFontVariations hashes variation axis settings for cache keys.
func HashFontVariations(vars []FontVariation) uint64 {
	if len(vars) == 0 {
		return 0
	}
	var result uint64
	h := fnv.New64a()
	for _, v := range vars {
		h.Reset()
		_, _ = h.Write(v.Tag[:])
		bits := math.Float32bits(v.Value)
		var vb [4]byte
		vb[0] = byte(bits)
		vb[1] = byte(bits >> 8)
		vb[2] = byte(bits >> 16)
		vb[3] = byte(bits >> 24)
		_, _ = h.Write(vb[:])
		result ^= h.Sum64()
	}
	return result
}

func hashStringFNV(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func faceShapeKey(face Face, s string, mode shapeResultMode) (shapeResultKey, bool) {
	if face == nil {
		return shapeResultKey{}, false
	}
	source := face.Source()
	if source == nil {
		// MultiFace etc. — caller should split runs; no single key.
		return shapeResultKey{}, false
	}
	return shapeResultKey{
		textHash:  hashStringFNV(s),
		fontID:    FontSourceID(source),
		sizeBits:  math.Float32bits(float32(face.Size())),
		direction: uint8(face.Direction() & 0xFF),
		features:  HashFontFeatures(face.Features()),
		langHash:  hashStringFNV(face.Language()),
		varHash:   HashFontVariations(face.Variations()),
		mode:      mode,
	}, true
}

// IsHighChurnLabel reports live telemetry-style strings (FPS / RSS / frame
// counters) that typically change every frame. Skipping soft-LRU result caches
// for these keeps capacity for stable UI labels and avoids multi-MB RSS growth
// under dynamic HUD load (unique full-string keys never hit).
//
// Heuristic: moderately long strings with a high digit ratio.
func IsHighChurnLabel(s string) bool {
	n := len(s)
	if n < 12 {
		return false
	}
	digits := 0
	for i := 0; i < n; i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			digits++
		}
	}
	// ≥20% digits with a meaningful digit count → ephemeral telemetry.
	// (dynamic HUD: "FPS … RSS … frame N"; static labels rarely hit this.)
	if digits < 6 {
		return false
	}
	return digits*5 >= n
}

// LayoutGlyphs converts text into positioned glyphs using Face.Glyphs
// (cmap + advance; no GSUB/GPOS). Results are cached for the GPU LayoutText
// hot path (S6.5). The returned slice must not be modified by callers.
// High-churn telemetry labels bypass the cache (see IsHighChurnLabel).
func LayoutGlyphs(face Face, s string) []ShapedGlyph {
	if face == nil || s == "" {
		return nil
	}
	if IsHighChurnLabel(s) {
		return layoutGlyphsUncached(face, s)
	}
	key, ok := faceShapeKey(face, s, shapeModeLayout)
	if !ok {
		return layoutGlyphsUncached(face, s)
	}
	return globalShapeResultCache.getOrCreate(key, func() []ShapedGlyph {
		return layoutGlyphsUncached(face, s)
	})
}

func layoutGlyphsUncached(face Face, s string) []ShapedGlyph {
	var shaped []ShapedGlyph
	for glyph := range face.Glyphs(s) {
		shaped = append(shaped, ShapedGlyph{
			GID:   glyph.GID,
			X:     glyph.X,
			Y:     glyph.Y,
			IsCJK: IsCJKRune(glyph.Rune),
		})
	}
	if shaped == nil {
		return []ShapedGlyph{}
	}
	return shaped
}
