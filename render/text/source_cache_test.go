package text

import (
	"os"
	"sync"
	"testing"
)

func cacheTestLoad(t *testing.T, path string) *FontSource {
	t.Helper()
	src, err := NewFontSourceFromFile(path)
	if err != nil {
		t.Fatalf("NewFontSourceFromFile(%q): %v", path, err)
	}
	return src
}

func TestFontSourceCacheSharesPerFile(t *testing.T) {
	PurgeFontSourceCache()
	ResetFontSourceCacheStats()

	path := "testdata/wqy-microhei.ttf"
	a := cacheTestLoad(t, path)
	b := cacheTestLoad(t, path)
	c := cacheTestLoad(t, path)

	if a != b || b != c {
		t.Fatalf("expected one shared FontSource for %q, got %p %p %p", path, a, b, c)
	}

	stats := FontSourceCacheSnapshot()
	if stats.Loads != 1 {
		t.Errorf("Loads = %d, want 1", stats.Loads)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Hits != 2 {
		t.Errorf("Hits = %d, want 2", stats.Hits)
	}
	if stats.Entries != 1 {
		t.Errorf("Entries = %d, want 1", stats.Entries)
	}

	PurgeFontSourceCache()
}

func TestFontSourceCacheTTCIndexesAreDistinct(t *testing.T) {
	PurgeFontSourceCache()

	src0 := cacheTestLoad(t, "testdata/wqy-microhei.ttc")
	src1, err := NewFontSourceFromFile("testdata/wqy-microhei.ttc", WithCollectionIndex(1))
	if err != nil {
		t.Fatalf("TTC index 1: %v", err)
	}
	if src0 == src1 {
		t.Fatal("expected distinct sources for TTC indexes 0 and 1")
	}
	if src1.Name() == "" {
		t.Error("expected TTC index 1 to parse a named face")
	}
	stats := FontSourceCacheSnapshot()
	if stats.Entries != 2 {
		t.Errorf("Entries = %d, want 2 (one per TTC index)", stats.Entries)
	}

	PurgeFontSourceCache()
}

func TestFontSourceCacheConcurrentLoadOnce(t *testing.T) {
	PurgeFontSourceCache()
	ResetFontSourceCacheStats()

	path := "testdata/wqy-microhei.ttf"
	const goroutines = 16
	results := make([]*FontSource, goroutines)
	errs := make([]error, goroutines)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results[i], errs[i] = NewFontSourceFromFile(path)
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range results {
		if errs[i] != nil {
			t.Fatalf("goroutine %d: %v", i, errs[i])
		}
		if results[i] != results[0] {
			t.Fatalf("goroutine %d got a different FontSource (%p vs %p)", i, results[i], results[0])
		}
	}
	stats := FontSourceCacheSnapshot()
	if stats.Loads != 1 {
		t.Errorf("Loads = %d, want 1 (singleflight)", stats.Loads)
	}
	if stats.Waits == 0 {
		t.Error("expected at least one waiter to join the in-flight load")
	}

	PurgeFontSourceCache()
}

func TestFontSourceCacheEviction(t *testing.T) {
	c := newFontSourceCache(2, 0)
	load := func(path string) func() (*FontSource, error) {
		return func() (*FontSource, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			return newFontSourceOwned(data, defaultSourceConfig())
		}
	}

	paths := []string{
		"testdata/wqy-microhei.ttf",
		"testdata/wqy-microhei-nohint.ttf",
		"testdata/goregular.ttf",
	}
	first, err := c.getOrLoad(fontSourceKey{path: paths[0]}, load(paths[0]))
	if err != nil {
		t.Fatalf("load %s: %v", paths[0], err)
	}
	for _, p := range paths[1:] {
		if _, err := c.getOrLoad(fontSourceKey{path: p}, load(p)); err != nil {
			t.Fatalf("load %s: %v", p, err)
		}
	}

	if c.stats.Loads != 3 {
		t.Errorf("Loads = %d, want 3", c.stats.Loads)
	}
	if c.stats.Evictions != 1 {
		t.Errorf("Evictions = %d, want 1", c.stats.Evictions)
	}
	if len(c.entries) != 2 {
		t.Errorf("entries = %d, want 2 (maxItems)", len(c.entries))
	}
	// An evicted source stays usable: holders keep a valid pointer.
	if first.data == nil {
		t.Error("evicted source data was released")
	}
	if face := first.Face(12); face == nil || !face.HasGlyph('A') {
		t.Error("evicted source must keep working for existing holders")
	}
}

func TestPurgeFontSourceCache(t *testing.T) {
	PurgeFontSourceCache()
	ResetFontSourceCacheStats()

	src := cacheTestLoad(t, "testdata/wqy-microhei.ttf")
	face := src.Face(14)
	before := FontSourceCacheSnapshot()

	PurgeFontSourceCache()
	after := FontSourceCacheSnapshot()
	if after.Entries != 0 || after.Bytes != 0 {
		t.Errorf("after purge: entries=%d bytes=%d, want 0/0", after.Entries, after.Bytes)
	}
	if after.Purges != before.Purges+1 {
		t.Errorf("Purges = %d, want %d", after.Purges, before.Purges+1)
	}
	// Faces handed out before the purge stay valid.
	if face == nil || !face.HasGlyph('A') {
		t.Error("pre-purge face must stay usable")
	}

	reloaded := cacheTestLoad(t, "testdata/wqy-microhei.ttf")
	if got := FontSourceCacheSnapshot().Loads; got != before.Loads+1 {
		t.Errorf("Loads after reload = %d, want %d (file re-read after purge)", got, before.Loads+1)
	}
	if reloaded.data == nil {
		t.Error("reloaded source has no data")
	}

	PurgeFontSourceCache()
}
