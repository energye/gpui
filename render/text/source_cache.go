//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package text

import (
	"container/list"
	"path/filepath"
	"sync"
)

// Process-wide font file cache (Skia SkTypefaceCache / Flutter font manager
// model): one font file is read and parsed once, and every loader — UI chain,
// system resolver, weight family, missing-glyph fallback — shares the same
// FontSource. Without this, per-rune fallbacks re-read whole files (a 75-load
// / 4-file burst was measured in the M5 any-script window).
//
// Entries live for the process; call PurgeFontSourceCache to release them
// (file updates on disk are ignored until then). Bounded by entry count and
// by the summed file bytes so many huge fonts cannot pin unbounded memory.

const (
	fontSourceCacheMaxEntries = 32
	fontSourceCacheMaxBytes   = 256 << 20
)

type fontSourceKey struct {
	path     string // absolute, cleaned
	ttcIndex int    // WithCollectionIndex
	parser   string // WithParser
}

type fontSourceEntry struct {
	key   fontSourceKey
	src   *FontSource
	bytes int64
}

type fontSourceLoad struct {
	done chan struct{}
	src  *FontSource
	err  error
}

type fontSourceCache struct {
	mu       sync.Mutex
	maxItems int
	maxBytes int64
	bytes    int64
	entries  map[fontSourceKey]*list.Element // value *fontSourceEntry, front = MRU
	lru      *list.List
	inflight map[fontSourceKey]*fontSourceLoad
	stats    FontSourceCacheStats
}

var globalFontSourceCache = newFontSourceCache(fontSourceCacheMaxEntries, fontSourceCacheMaxBytes)

func newFontSourceCache(maxItems int, maxBytes int64) *fontSourceCache {
	return &fontSourceCache{
		maxItems: maxItems,
		maxBytes: maxBytes,
		entries:  make(map[fontSourceKey]*list.Element, maxItems),
		lru:      list.New(),
		inflight: make(map[fontSourceKey]*fontSourceLoad),
	}
}

// FontSourceCacheStats reports global font source cache counters.
type FontSourceCacheStats struct {
	Entries   int    // live entries
	Bytes     int64  // file bytes held by live entries
	Loads     uint64 // file reads + parses performed
	Hits      uint64 // lookups served from a live entry
	Misses    uint64 // lookups that started a load
	Waits     uint64 // lookups that waited for an in-flight load
	Evictions uint64 // entries dropped by the size limits
	Purges    uint64 // PurgeFontSourceCache calls that dropped entries
}

// FontSourceCacheSnapshot returns counters for the global font source cache.
func FontSourceCacheSnapshot() FontSourceCacheStats {
	c := globalFontSourceCache
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.stats
	s.Entries = len(c.entries)
	s.Bytes = c.bytes
	return s
}

// ResetFontSourceCacheStats clears hit/miss/load/eviction counters
// (entries are retained).
func ResetFontSourceCacheStats() {
	c := globalFontSourceCache
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats = FontSourceCacheStats{}
}

// PurgeFontSourceCache drops every cached FontSource and the global caches
// that would otherwise keep the dropped sources reachable. Faces and sources
// already handed out stay valid; files are re-read on the next load.
//
// Purge is meant for tests and explicit font reloads, not for use while a
// frame is being painted.
func PurgeFontSourceCache() {
	c := globalFontSourceCache
	c.mu.Lock()
	if len(c.entries) > 0 {
		c.stats.Purges++
	}
	c.entries = make(map[fontSourceKey]*list.Element, c.maxItems)
	c.lru.Init()
	c.bytes = 0
	c.mu.Unlock()

	// Drop the global caches that pin sources (or their parsed font) alive.
	ClearFontScanFallbackCache()
	ClearMultiFaceRunsCache()
	switch s := GetShaper().(type) {
	case *HbShaper:
		s.ClearCache()
	case *OwnShaper:
		s.ClearCache()
	}
	fontDigestCache.Range(func(k, _ any) bool {
		fontDigestCache.Delete(k)
		return true
	})
}

func (c *fontSourceCache) getOrLoad(key fontSourceKey, load func() (*FontSource, error)) (*FontSource, error) {
	for {
		c.mu.Lock()
		if el, ok := c.entries[key]; ok {
			c.lru.MoveToFront(el)
			c.stats.Hits++
			src := el.Value.(*fontSourceEntry).src
			c.mu.Unlock()
			return src, nil
		}
		if call, ok := c.inflight[key]; ok {
			c.stats.Waits++
			c.mu.Unlock()
			<-call.done
			return call.src, call.err
		}
		call := &fontSourceLoad{done: make(chan struct{})}
		c.inflight[key] = call
		c.stats.Misses++
		c.mu.Unlock()

		src, err := load()

		c.mu.Lock()
		delete(c.inflight, key)
		if err == nil && src != nil {
			src.cached.Store(true)
			c.stats.Loads++
			c.insertLocked(key, src, int64(len(src.data)))
		}
		call.src = src
		call.err = err
		close(call.done)
		c.mu.Unlock()
		return src, err
	}
}

func (c *fontSourceCache) insertLocked(key fontSourceKey, src *FontSource, size int64) {
	if el, ok := c.entries[key]; ok {
		el.Value.(*fontSourceEntry).src = src
		c.lru.MoveToFront(el)
		return
	}
	c.entries[key] = c.lru.PushFront(&fontSourceEntry{key: key, src: src, bytes: size})
	c.bytes += size
	for (c.maxItems > 0 && c.lru.Len() > c.maxItems) ||
		(c.maxBytes > 0 && c.bytes > c.maxBytes && c.lru.Len() > 1) {
		back := c.lru.Back()
		if back == nil {
			break
		}
		ent := back.Value.(*fontSourceEntry)
		c.lru.Remove(back)
		delete(c.entries, ent.key)
		c.bytes -= ent.bytes
		c.stats.Evictions++
	}
}

// fontSourceKeyFor builds the share key for a file. WithCacheLimit is
// deliberately not part of it: it is not read by any parser, so sharing the
// source built with the first caller's options is safe.
func fontSourceKeyFor(path string, config sourceConfig) fontSourceKey {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return fontSourceKey{
		path:     filepath.Clean(path),
		ttcIndex: config.collectionIndex,
		parser:   config.parserName,
	}
}
