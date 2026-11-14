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
	"os"
	"sync"
)

// Per-path shared font file bytes (Skia SkData::MakeFromFileName / Flutter
// fml::FileMapping model): the file is mapped read-only once per resolved
// path and every (path, ttcIndex, parser) cache key shares that mapping.
// Before this table, each cache key held its own full copy of the file bytes
// in the Go heap — the same 18.6MB CJK TTC loaded under two collection
// indexes pinned ~37MB of anonymous heap per process (O8, 2026-11-14).
//
// Mapped pages are file-backed: the OS can evict them under memory pressure
// and re-read on touch, they are shared with every other process mapping the
// same file, and they never consume Go heap. Platforms without mmap fall
// back to os.ReadFile with the same sharing semantics (one copy per path).
//
// Lifetime contract (must hold — the cache hands sources out that callers
// keep using after a purge, and lazy table parsing reads the bytes at any
// later time): the mapping lives as long as ANY FontSource referencing it.
// Each source registers a runtime.AddCleanup that drops its reference when
// the source becomes unreachable. Cache eviction and PurgeFontSourceCache
// only drop their bookkeeping — they never unmap bytes a live source can
// still read ("faces and sources already handed out stay valid").

type fontFileData struct {
	path    string
	data    []byte
	unmap   func() // nil for the heap fallback
	refs    int
	created bool // true when this acquire created the mapping (first ref)
}

var (
	fontFileMu    sync.Mutex
	fontFileTable map[string]*fontFileData
)

// fontFileAcquire returns the shared bytes for path, taking one reference.
// The path must already be normalized (cleaned/absolute) — the source cache
// passes its normalized key path so both tables agree on identity.
func fontFileAcquire(path string) (*fontFileData, error) {
	fontFileMu.Lock()
	defer fontFileMu.Unlock()
	if fontFileTable == nil {
		fontFileTable = make(map[string]*fontFileData)
	}
	if entry, ok := fontFileTable[path]; ok {
		entry.refs++
		return entry, nil
	}
	data, release, err := mmapFontFile(path)
	if err != nil {
		// Skia MakeFromFile semantics: mmap attempt first, plain read fallback.
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		release = nil
	}
	entry := &fontFileData{
		path:    path,
		data:    data,
		unmap:   release,
		refs:    1,
		created: true,
	}
	fontFileTable[path] = entry
	return entry, nil
}

// release drops one reference. The mapping is closed and dropped from the
// table when the last reference goes away. Safe to call while holding no
// other font locks.
func (f *fontFileData) release() {
	if f == nil {
		return
	}
	fontFileMu.Lock()
	defer fontFileMu.Unlock()
	f.refs--
	if f.refs > 0 {
		return
	}
	if f.unmap != nil {
		f.unmap()
	}
	if fontFileTable[f.path] == f {
		delete(fontFileTable, f.path)
	}
}

// fontFilePurgeUnreferenced drops mappings with no remaining FontSource.
// PurgeFontSourceCache calls this after clearing its entries: sources handed
// out before the purge keep their references (and their mappings) until they
// become unreachable, so a still-referenced mapping survives the purge.
func fontFilePurgeUnreferenced() {
	fontFileMu.Lock()
	defer fontFileMu.Unlock()
	for _, entry := range fontFileTable {
		if entry.refs <= 0 {
			if entry.unmap != nil {
				entry.unmap()
			}
			delete(fontFileTable, entry.path)
		}
	}
}

// fontFileTableSize reports the live mapping count (tests and diagnostics).
func fontFileTableSize() int {
	fontFileMu.Lock()
	defer fontFileMu.Unlock()
	return len(fontFileTable)
}
