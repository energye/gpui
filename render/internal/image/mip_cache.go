//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package image

import "sync"

// Mip chain cache (P3.6 batch 2).
//
// One chain per content generation. Before this cache every minified draw
// rebuilt the whole chain (≈1.7ms for a 512x512 source, even after the row-wise
// downsample fix) and never returned its levels to the pool: the CPU fallback
// paid that per draw and leaked the buffers, while the GPU path had no chain at
// all.
//
// Keying by content generation is sound because batch 1c made the generation the
// content version, handed out by one process-wide allocator: an ID is never
// reissued, so a stale entry can never be served for new pixels — it can only be
// retired. Retirement rides the same watcher the GPU image cache uses
// (AddGenIDWatcher), so a moved or disposed buffer gives its levels back to the
// pool immediately instead of waiting for LRU.
//
// Two more properties, both pinned by mip_cache_test.go:
//   - single flight: N threads asking for the same unseen generation cause
//     exactly one build;
//   - references: an entry being read is never freed. A retired or evicted entry
//     with live references is marked dead and freed by the last release — the
//     same discipline the GPU resource registry applies to in-flight textures.
//
// Caps are declared here, the way defaultPool is, since this package owns CPU
// pixel buffers; render's budget table surfaces them as BudgetMipChain so each
// domain keeps one visible declaration.
const (
	MipChainMaxEntries = 64
	MipChainMaxBytes   = int64(32 << 20)
)

type mipChainEntry struct {
	chain *MipmapChain
	gen   uint64
	bytes int64
	stamp uint64
	unsub func()
	refs  int
	dead  bool
}

var (
	mipMu      sync.Mutex
	mipEntries = map[uint64]*mipChainEntry{}
	mipBuild   = map[uint64]chan struct{}{}
	mipStamp   uint64
	mipBuilds  uint64
	mipHits    uint64
	mipEvicts  uint64
	mipWaits   uint64
	mipRaces   uint64
)

// MipChainBudget reports the cache caps (render/budget.go surfaces these).
func MipChainBudget() (entries int, bytes int64) { return MipChainMaxEntries, MipChainMaxBytes }

// MipChainStats is the cache snapshot for diagnostics and gates. Entries and
// Bytes cover live entries only; Dead counts retired-but-still-read ones, so a
// gate can tell resident from about-to-be-freed.
type MipChainStats struct {
	Entries    int
	Bytes      int64
	Dead       int
	Builds     uint64
	Hits       uint64
	Evicts     uint64
	Waits      uint64
	Races      uint64
	MaxEntries int
	MaxBytes   int64
}

// MipChainCacheStats snapshots the cache state.
func MipChainCacheStats() MipChainStats {
	mipMu.Lock()
	defer mipMu.Unlock()
	st := MipChainStats{Builds: mipBuilds, Hits: mipHits, Evicts: mipEvicts,
		Waits: mipWaits, Races: mipRaces,
		MaxEntries: MipChainMaxEntries, MaxBytes: MipChainMaxBytes}
	for _, e := range mipEntries {
		if e.dead {
			st.Dead++
			continue
		}
		st.Entries++
		st.Bytes += e.bytes
	}
	return st
}

// MipChainFor returns the full mip chain of src, built once per content
// generation and cached until those pixels move or the buffer is disposed.
//
// The second return is the caller's release; call it when done reading the
// chain. The chain stays owned by the cache — never call Release on it.
//
// ok is false for a nil, disposed, ephemeral (generation 0) or empty source, and
// for a generation whose pixels moved while its chain was being built (that
// chain describes pixels nobody draws any more, so it is dropped, not cached).
func MipChainFor(src *ImageBuf) (chain *MipmapChain, release func(), ok bool) {
	if src == nil {
		return nil, nil, false
	}
	// Two passes: the first may wait on another builder; the second then either
	// hits the entry it published or builds it ourselves.
	for pass := 0; pass < 2; pass++ {
		gen := src.GenerationID()
		if gen == 0 {
			return nil, nil, false
		}
		if c, rel, hit := mipChainAcquire(gen); hit {
			return c, rel, true
		}
		ch, mine := mipClaimBuild(gen)
		if !mine {
			mipMu.Lock()
			mipWaits++
			mipMu.Unlock()
			<-ch
			continue
		}
		built := GenerateMipmaps(src)
		if built == nil {
			mipEndBuild(gen, nil)
			return nil, nil, false
		}
		if src.GenerationID() != gen {
			mipEndBuild(gen, nil)
			mipMu.Lock()
			mipRaces++
			mipMu.Unlock()
			built.Release()
			return nil, nil, false
		}
		entry := &mipChainEntry{chain: built, gen: gen, bytes: mipChainBytes(built.levels)}
		entry.unsub = src.AddGenIDWatcher(mipChainRetire)
		existing, evicted := mipChainPublish(gen, entry)
		mipReleaseChains(evicted)
		if existing != nil {
			// Guard for the impossible-but-cheap case: an entry for this
			// generation appeared while we were building. Keep theirs, drop ours.
			entry.unsub()
			built.Release()
			return existing.chain, mipChainAddRef(existing), true
		}
		return built, func() { mipChainDrop(entry) }, true
	}
	return nil, nil, false
}

// mipClaimBuild takes the builder role for gen and hands back the channel to be
// closed when the build finishes, or the channel of whoever already has it.
func mipClaimBuild(gen uint64) (ch chan struct{}, mine bool) {
	mipMu.Lock()
	defer mipMu.Unlock()
	if busy, ok := mipBuild[gen]; ok {
		return busy, false
	}
	ch = make(chan struct{})
	mipBuild[gen] = ch
	return ch, true
}

// mipChainPublish installs the finished entry, clears the builder claim and
// wakes waiters — all under one lock, so a waiter can never observe the claim
// gone while the entry is still missing. It returns an existing entry (caller
// drops its own copy) plus entries that fell out of the caps.
func mipChainPublish(gen uint64, entry *mipChainEntry) (existing *mipChainEntry, evicted []*mipChainEntry) {
	mipMu.Lock()
	ch := mipBuild[gen]
	delete(mipBuild, gen)
	if cur, dup := mipEntries[gen]; dup {
		mipStamp++
		cur.stamp = mipStamp
		cur.refs++
		existing = cur
	} else {
		entry.refs = 1 // held by the thread that just built it
		mipEntries[gen] = entry
		mipBuilds++
		evicted = mipTakeEvicteesLocked()
	}
	mipMu.Unlock()
	if ch != nil {
		close(ch)
	}
	return existing, evicted
}

// mipEndBuild releases the builder claim without publishing (failure paths).
func mipEndBuild(gen uint64, entry *mipChainEntry) {
	mipMu.Lock()
	ch := mipBuild[gen]
	delete(mipBuild, gen)
	mipMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

// mipChainAcquire returns a live entry with the caller's reference already taken.
func mipChainAcquire(gen uint64) (*MipmapChain, func(), bool) {
	mipMu.Lock()
	e, ok := mipEntries[gen]
	if !ok || e.dead {
		mipMu.Unlock()
		return nil, nil, false
	}
	mipStamp++
	e.stamp = mipStamp
	e.refs++
	mipHits++
	c := e.chain
	mipMu.Unlock()
	return c, func() { mipChainDrop(e) }, true
}

func mipChainAddRef(e *mipChainEntry) func() {
	mipMu.Lock()
	if !e.dead {
		e.refs++
	}
	mipMu.Unlock()
	return func() { mipChainDrop(e) }
}

func mipChainDrop(e *mipChainEntry) {
	mipMu.Lock()
	e.refs--
	free := e.refs <= 0 && e.dead
	if free {
		delete(mipEntries, e.gen)
	}
	mipMu.Unlock()
	if !free {
		return
	}
	if e.unsub != nil {
		e.unsub()
	}
	e.chain.Release()
}

// mipChainRetire is the generation watcher: the pixels behind a cached chain are
// gone, so its levels go back to the pool — once nothing is reading them.
func mipChainRetire(gen uint64) {
	mipMu.Lock()
	e, ok := mipEntries[gen]
	if !ok {
		mipMu.Unlock()
		return
	}
	e.dead = true
	freed := e.refs <= 0
	if freed {
		delete(mipEntries, gen)
	}
	mipMu.Unlock()
	if !freed {
		return
	}
	if e.unsub != nil {
		e.unsub()
	}
	e.chain.Release()
}

// mipTakeEvicteesLocked marks least-recently-used live entries dead until both
// caps hold. Entries still being read are skipped rather than spun on: briefly
// exceeding the cap beats a livelock. The caller frees them off the lock.
func mipTakeEvicteesLocked() []*mipChainEntry {
	var victims []*mipChainEntry
	for {
		live, bytes := 0, int64(0)
		for _, e := range mipEntries {
			if e.dead {
				continue
			}
			live++
			bytes += e.bytes
		}
		if live <= MipChainMaxEntries && bytes <= MipChainMaxBytes {
			return victims
		}
		var key uint64
		var oldest *mipChainEntry
		for gen, e := range mipEntries {
			if e.dead || e.refs > 0 {
				continue
			}
			if oldest == nil || e.stamp < oldest.stamp {
				oldest, key = e, gen
			}
		}
		if oldest == nil {
			return victims // everything live is being read right now
		}
		oldest.dead = true
		delete(mipEntries, key)
		mipEvicts++
		victims = append(victims, oldest)
	}
}

func mipReleaseChains(victims []*mipChainEntry) {
	for _, e := range victims {
		if e.unsub != nil {
			e.unsub()
		}
		e.chain.Release()
	}
}

// MipChainFlush releases every cached chain that is not being read (gate hygiene:
// a cache claimed to be bounded must also show it empties).
func MipChainFlush() {
	mipMu.Lock()
	victims := make([]*mipChainEntry, 0, len(mipEntries))
	for gen, e := range mipEntries {
		if e.refs > 0 {
			e.dead = true
			continue
		}
		victims = append(victims, e)
		delete(mipEntries, gen)
	}
	mipMu.Unlock()
	mipReleaseChains(victims)
}

// mipChainBytes counts a chain's own storage: levels 1..N. Level 0 is the source
// buffer itself, owned elsewhere.
func mipChainBytes(levels []*ImageBuf) int64 {
	var n int64
	for i := 1; i < len(levels); i++ {
		if levels[i] == nil {
			continue
		}
		w, h := levels[i].Bounds()
		n += int64(w) * int64(h) * int64(levels[i].Format().BytesPerPixel())
	}
	return n
}
