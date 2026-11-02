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

import (
	"sync"
	"sync/atomic"
)

// contentGenID is the single process-global allocator for pixel content
// generations.
//
// One allocator is a correctness requirement, not tidiness: Pixmap and ImageBuf
// used to own separate counters, both starting at 1, and both feed the same
// GPU image-cache key space (measured: 200 IDs from each side overlapped 200
// for 200). Two different buffers then carry the same number, and a cache
// keyed by that number serves one source's texture for the other's draw.
var contentGenID atomic.Uint64

// NextContentGenID hands out the next generation ID. Every pixel source whose
// ID keys a cache takes its number from here.
func NextContentGenID() uint64 { return contentGenID.Add(1) }

// GenWatch is a pixel source's list of generation-change listeners.
//
// This is the Skia SkPixelRef::addGenIDChangeListener role: caches keyed by the
// generation ID need to hear that the ID was retired, or the texture they
// uploaded for the previous content lives on as a budget-consuming zombie.
//
// Shape note against upstream (deliberate, not an oversight): Skia listeners are
// one-shot per generation change ("you must add a new listener for each
// generation ID change"), while ours stay registered until removed. Callers that
// cache by ID take the registration when they insert an entry and drop it when
// they remove the entry, which covers exactly the same window.
//
// Callbacks run with no lock held, so a listener may touch its own cache but
// must not call back into the pixel source it was notified by.
type GenWatch struct {
	mu       sync.Mutex
	watchers map[uint64]func(oldGenID uint64)
	seq      uint64
}

// Add registers fn and returns a remove closure; calling it twice is safe.
func (w *GenWatch) Add(fn func(oldGenID uint64)) func() {
	if w == nil || fn == nil {
		return func() {}
	}
	w.mu.Lock()
	if w.watchers == nil {
		w.watchers = make(map[uint64]func(uint64), 2)
	}
	w.seq++
	key := w.seq
	w.watchers[key] = fn
	w.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			w.mu.Lock()
			delete(w.watchers, key)
			w.mu.Unlock()
		})
	}
}

// Fire runs every listener with the retired generation ID.
func (w *GenWatch) Fire(oldGenID uint64) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if len(w.watchers) == 0 {
		w.mu.Unlock()
		return
	}
	fns := make([]func(uint64), 0, len(w.watchers))
	for _, fn := range w.watchers {
		fns = append(fns, fn)
	}
	w.mu.Unlock()
	for _, fn := range fns {
		fn(oldGenID)
	}
}

// GenWatched is a pixel source whose generation ID can change while the object
// itself stays: the GPU caches key on that ID, so they must be able to follow it.
// Both ImageBuf and render.Pixmap satisfy it.
type GenWatched interface {
	// GenerationID is the current content generation: it identifies the pixels,
	// not the buffer, and it changes whenever the pixels are rewritten.
	GenerationID() uint64
	// AddGenIDWatcher registers fn to run with the retired generation ID after
	// the pixels get a new one (and once more when the source is released).
	// The returned closure drops the registration; calling it twice is safe.
	AddGenIDWatcher(fn func(oldGenID uint64)) (remove func())
}

var _ GenWatched = (*ImageBuf)(nil)
