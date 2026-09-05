package render

import (
	"strings"
	"sync"
)

// PurgeEvictable is implemented by caches holding rebuildable resources
// (glyph atlas pages, image texture caches, layer pools). PurgeEvictable
// drops only entries the cache can rebuild on next use and reports how many
// entries were freed. Called from the texture-OOM recovery path.
type PurgeEvictable interface {
	PurgeEvictable() (freedEntries int64)
}

var purgeRegistry = struct {
	sync.Mutex
	entries []namedPurge
}{}

type namedPurge struct {
	name  string
	cache PurgeEvictable
}

// RegisterPurgeEvictable adds a rebuildable cache to the texture-OOM purge
// chain. Registration is additive; packages with evictable caches call this
// once at construction. A nil cache is ignored.
func RegisterPurgeEvictable(name string, cache PurgeEvictable) {
	if cache == nil {
		return
	}
	purgeRegistry.Lock()
	defer purgeRegistry.Unlock()
	purgeRegistry.entries = append(purgeRegistry.entries, namedPurge{name: name, cache: cache})
}

// UnregisterPurgeEvictable removes a cache from the purge chain. Call at
// teardown so post-close purges never touch released native resources.
func UnregisterPurgeEvictable(cache PurgeEvictable) {
	if cache == nil {
		return
	}
	purgeRegistry.Lock()
	defer purgeRegistry.Unlock()
	kept := purgeRegistry.entries[:0]
	for _, e := range purgeRegistry.entries {
		if e.cache != cache {
			kept = append(kept, e)
		}
	}
	purgeRegistry.entries = kept
}

// PurgeEvictables runs every registered purge once and reports per-cache
// freed entry counts plus the total. A nil cache entry is skipped.
func PurgeEvictables() (total int64, byName map[string]int64) {
	purgeRegistry.Lock()
	entries := make([]namedPurge, len(purgeRegistry.entries))
	copy(entries, purgeRegistry.entries)
	purgeRegistry.Unlock()
	byName = make(map[string]int64, len(entries))
	for _, e := range entries {
		if e.cache == nil {
			continue
		}
		freed := e.cache.PurgeEvictable()
		byName[e.name] += freed
		total += freed
	}
	return total, byName
}

// IsGPUOutOfMemory reports whether err is a GPU-memory exhaustion error.
// Matches wgpu-native phrasing case-insensitively. Shared by the present
// downgrade chain, the texture purge path, and embedder exit decisions.
func IsGPUOutOfMemory(err error) bool {
	if err == nil {
		return false
	}
	low := strings.ToLower(err.Error())
	return strings.Contains(low, "not enough memory") ||
		strings.Contains(low, "out of memory") ||
		strings.Contains(low, "out of device memory")
}
