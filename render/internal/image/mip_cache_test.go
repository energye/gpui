package image

import (
	"sync"
	"testing"
	"time"
)

func newMipSrc(t *testing.T, w, h int) *ImageBuf {
	t.Helper()
	img, err := NewImageBuf(w, h, FormatRGBAPremul)
	if err != nil {
		t.Fatal(err)
	}
	d := img.Data()
	st := img.Stride()
	for y := 0; y < h; y++ {
		for x := 0; x < w*4; x++ {
			d[y*st+x] = byte((x + y*7) % 251)
		}
	}
	img.NotifyPixelsChanged()
	return img
}

// TestMipChainBuiltOncePerGeneration pins the point of the cache: one build per
// content generation, a fresh one after the pixels move, and the retired chain
// reclaimed without waiting for LRU.
func TestMipChainBuiltOncePerGeneration(t *testing.T) {
	MipChainFlush()
	base := MipChainCacheStats().Builds
	src := newMipSrc(t, 64, 64)
	defer src.Dispose()

	c1, rel1, ok := MipChainFor(src)
	if !ok || c1 == nil {
		t.Fatal("no chain for a 64x64 source")
	}
	defer rel1()
	st := MipChainCacheStats()
	if st.Builds-base != 1 || st.Entries != 1 {
		t.Fatalf("after first take: builds=+%d entries=%d, want +1/1", st.Builds-base, st.Entries)
	}
	levels := c1.NumLevels()

	c2, rel2, ok := MipChainFor(src)
	defer rel2()
	if !ok || c2 != c1 {
		t.Fatal("second call returned a different chain: cache miss")
	}
	st = MipChainCacheStats()
	if st.Builds-base != 1 {
		t.Fatalf("builds=+%d after a hit, want +1 (chain rebuilt per draw = the old bug)", st.Builds-base)
	}
	if st.Hits < 1 {
		t.Fatalf("hits=%d, want >=1", st.Hits)
	}

	// Pixels move: the previous generation's chain must be reclaimed now.
	src.Fill(10, 20, 30, 255)
	src.NotifyPixelsChanged()
	st = MipChainCacheStats()
	if st.Entries != 0 {
		t.Fatalf("entries=%d after NotifyPixelsChanged, want 0 (retired chain still cached)", st.Entries)
	}
	c3, rel3, ok := MipChainFor(src)
	if !ok {
		t.Fatal("no chain after a new generation")
	}
	defer rel3()
	if c3 == c1 {
		t.Fatal("new generation served the old chain")
	}
	if c3.NumLevels() != levels {
		t.Fatalf("levels %d != %d for the same size", c3.NumLevels(), levels)
	}
	st = MipChainCacheStats()
	if st.Builds-base != 2 {
		t.Fatalf("builds=+%d, want +2", st.Builds-base)
	}
}

// TestMipChainDisposeReclaims pins the Dispose path: a dead buffer must not keep
// its chain (and its pool buffers) resident.
func TestMipChainDisposeReclaims(t *testing.T) {
	MipChainFlush()
	src := newMipSrc(t, 32, 32)
	_, rel, ok := MipChainFor(src)
	if !ok {
		t.Fatal("no chain")
	}
	defer rel()
	if MipChainCacheStats().Entries != 1 {
		t.Fatalf("entries=%d, want 1", MipChainCacheStats().Entries)
	}
	src.Dispose()
	if st := MipChainCacheStats(); st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("after Dispose: entries=%d bytes=%d, want 0/0", st.Entries, st.Bytes)
	}
}

// TestMipChainHoldsAcrossFlush proves the handle is what makes eviction safe: a
// draw still reading a level must not have its buffers returned to the pool.
func TestMipChainHoldsAcrossFlush(t *testing.T) {
	MipChainFlush()
	src := newMipSrc(t, 32, 32)
	defer src.Dispose()
	chain, rel, ok := MipChainFor(src)
	if !ok {
		t.Fatal("no chain")
	}
	lvl := chain.Level(2)
	if lvl == nil {
		t.Fatal("no level 2")
	}
	want := append([]byte(nil), lvl.Data()...)

	// Somebody else empties the cache while this draw holds its reference.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			other, err := NewImageBuf(16, 16, FormatRGBAPremul)
			if err != nil {
				t.Errorf("new buf: %v", err)
				continue
			}
			if c, r, ok := MipChainFor(other); ok {
				_ = c
				r()
			}
			other.Dispose()
			MipChainFlush()
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("flush churn hung")
	}

	got := lvl.Data()
	if len(got) != len(want) {
		t.Fatalf("level shrunk under a live reader: %d -> %d", len(want), len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("held level mutated at byte %d (%d -> %d): eviction freed a buffer still being read", i, want[i], got[i])
		}
	}
	rel()
}

// TestMipChainRespectsCaps drives more live generations than the entry cap and
// checks both caps hold, evictions happened, and nothing was freed twice.
func TestMipChainRespectsCaps(t *testing.T) {
	MipChainFlush()
	const n = MipChainMaxEntries + 12
	srcs := make([]*ImageBuf, 0, n)
	defer func() {
		for _, s := range srcs {
			s.Dispose()
		}
	}()
	for i := 0; i < n; i++ {
		src := newMipSrc(t, 16, 16)
		srcs = append(srcs, src)
		chain, rel, ok := MipChainFor(src)
		if !ok {
			t.Fatalf("generation %d: no chain", i)
		}
		if lvl := chain.Level(1); lvl != nil {
			_ = lvl.Data()[0]
		}
		rel()
	}
	st := MipChainCacheStats()
	if st.Entries > st.MaxEntries {
		t.Fatalf("live entries=%d exceeds cap %d", st.Entries, st.MaxEntries)
	}
	if st.Bytes > st.MaxBytes {
		t.Fatalf("live bytes=%d exceeds cap %d", st.Bytes, st.MaxBytes)
	}
	if st.Evicts == 0 {
		t.Fatalf("no evictions after %d generations (cap %d)", n, st.MaxEntries)
	}
}

// TestMipChainConcurrentSameGeneration races many readers of one generation:
// exactly one build may land, and no level may be freed while another holds it.
func TestMipChainConcurrentSameGeneration(t *testing.T) {
	MipChainFlush()
	base := MipChainCacheStats().Builds
	src := newMipSrc(t, 128, 128)
	defer src.Dispose()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				chain, rel, ok := MipChainFor(src)
				if !ok {
					continue
				}
				if lvl := chain.Level(3); lvl != nil {
					_ = lvl.Data()[0]
				}
				rel()
			}
		}()
	}
	wg.Wait()
	st := MipChainCacheStats()
	if st.Builds-base != 1 {
		t.Fatalf("builds=+%d for one generation, want +1 (duplicate builds = wasted work and double free pressure)", st.Builds-base)
	}
	if st.Entries != 1 {
		t.Fatalf("entries=%d, want 1", st.Entries)
	}
}
