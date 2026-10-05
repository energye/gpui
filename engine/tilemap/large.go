//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package tilemap

import (
	"sync"

	"github.com/energye/gpui/engine/core"
)

// Large map streaming (S84, V3): index plus two-phase merge.
//
// A 1024x1024 map holds a million tiles; no frame parses it whole.
// Large keeps a chunk directory (32x32 tiles per chunk, 1024 chunks for
// the full map) and streams through it: Request queues the chunks a view
// needs (background parse), MergeBudget moves at most maxPerFrame into
// loaded (foreground merge under the per-frame budget), Update drops the
// chunks the view left behind in the same step so a jumping lens never
// leaves stale chunks and never grows. Unloaded chunks draw the magenta
// placeholder (see tex.PlaceholderImage), never a guessed tile.
//
// Geometry only comes from Chunk (Needed/ChunkBounds), so the culling
// math cannot drift between the small and large paths. This file owns
// the loaded/pending sets and the budgets; tilemap.go/chunk.go/lod.go
// semantics are untouched. Objects ride the Tilemap object list
// (ObjectsIn); spawn markers follow the scene, no second index here.
//
// Errors: bad sizes are InvalidArg, grids beyond MaxLargeTiles or
// MaxChunks are OutOfMemory, bad views change nothing. Nil receivers
// never panic.

// LargeChunkW/H is the frozen V3 partition: 32x32 tiles per chunk.
const (
	LargeChunkW = 32
	LargeChunkH = 32
)

// MaxLargeTiles caps one large map: 1024x1024 = a million tiles.
// Beyond it NewLarge reports OutOfMemory, never a half index.
const MaxLargeTiles = 1024 * 1024

// DefaultMergePerFrame caps one foreground merge so walking stays under
// the per-frame budget (S84: streaming a frame costs at most 2ms).
const DefaultMergePerFrame = 4

// Large is one streaming map index over a Chunk grid. The grid is held
// by pointer and never copied after construction: Chunk carries a lock.
type Large struct {
	grid       *Chunk
	chunkW     int
	chunkH     int
	mu         sync.RWMutex
	loaded     map[ChunkID]struct{}
	pending    []ChunkID
	pendingSet map[ChunkID]struct{}
	requests   int64
	merges     int64
}

// NewLarge builds a 1024-capable index over a mapW x mapH tile map with
// the frozen 32x32 partition and 32x32 world tiles. For other tile or
// chunk sizes use NewLargeSized.
func NewLarge(mapW, mapH int) (Large, error) {
	return NewLargeSized(OrientOrthogonal, mapW, mapH, 32, 32, LargeChunkW, LargeChunkH)
}

// NewLargeSized builds a streaming index with explicit geometry. Tile and
// chunk sizes must be finite and > 0; the tile total must fit
// MaxLargeTiles and the chunk grid must fit MaxChunks.
func NewLargeSized(orient Orientation, mapW, mapH int, tileW, tileH float64, chunkW, chunkH int) (Large, error) {
	const op = "tilemap.NewLarge"
	if orient != OrientOrthogonal && orient != OrientIsometric {
		return Large{}, core.InvalidArg(op, "orient")
	}
	if mapW <= 0 || mapH <= 0 {
		return Large{}, core.InvalidArg(op, "size")
	}
	if !finite(tileW) || !finite(tileH) || tileW <= 0 || tileH <= 0 {
		return Large{}, core.InvalidArg(op, "tile")
	}
	if chunkW <= 0 || chunkH <= 0 {
		return Large{}, core.InvalidArg(op, "chunk")
	}
	if int64(mapW)*int64(mapH) > MaxLargeTiles {
		return Large{}, core.OutOfMemory(op, "tiles")
	}
	grid, err := NewChunker(orient, mapW, mapH, tileW, tileH, chunkW, chunkH)
	if err != nil {
		return Large{}, err
	}
	return Large{grid: &grid, chunkW: chunkW, chunkH: chunkH}, nil
}

// W returns the map width in tiles, or 0 on a nil index.
func (l *Large) W() int {
	if l == nil || l.grid == nil {
		return 0
	}
	return l.grid.W()
}

// H returns the map height in tiles, or 0 on a nil index.
func (l *Large) H() int {
	if l == nil || l.grid == nil {
		return 0
	}
	return l.grid.H()
}

// ChunkW returns the chunk width in tiles, or 0 on a nil index.
func (l *Large) ChunkW() int {
	if l == nil {
		return 0
	}
	return l.chunkW
}

// ChunkH returns the chunk height in tiles, or 0 on a nil index.
func (l *Large) ChunkH() int {
	if l == nil {
		return 0
	}
	return l.chunkH
}

// Cols returns how many chunks span the map width, or 0 on a nil index.
func (l *Large) Cols() int {
	if l == nil || l.grid == nil {
		return 0
	}
	return l.grid.ChunkCols()
}

// Rows returns how many chunks span the map height, or 0 on a nil index.
func (l *Large) Rows() int {
	if l == nil || l.grid == nil {
		return 0
	}
	return l.grid.ChunkRows()
}

// ChunkCount returns Cols*Rows: 1024 for the full 1024x1024 map.
func (l *Large) ChunkCount() int {
	if l == nil || l.grid == nil {
		return 0
	}
	return l.grid.ChunkCols() * l.grid.ChunkRows()
}

// LoadedCount returns how many chunks are loaded now.
func (l *Large) LoadedCount() int {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.loaded)
}

// PendingCount returns how many chunks wait for the foreground merge.
func (l *Large) PendingCount() int {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.pending)
}

// Requests returns how many chunks were ever queued.
func (l *Large) Requests() int64 {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.requests
}

// Merges returns how many chunks ever reached loaded.
func (l *Large) Merges() int64 {
	if l == nil {
		return 0
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.merges
}

// ChunkOf maps one tile to its chunk. OOB tiles report ok=false.
func (l *Large) ChunkOf(col, row int) (ChunkID, bool) {
	if l == nil || l.grid == nil {
		return ChunkID{}, false
	}
	return l.grid.ChunkOf(col, row)
}

// ChunkBounds returns the world box a chunk covers. Unknown ids report
// ok=false with zero.
func (l *Large) ChunkBounds(id ChunkID) (core.Rect, bool) {
	if l == nil || l.grid == nil {
		return core.Rect{}, false
	}
	return l.grid.ChunkBounds(id)
}

// Needed lists chunks touching view, sorted by (CY,CX). Bad or empty
// views need nothing, never panic.
func (l *Large) Needed(view core.Rect) []ChunkID {
	if l == nil || l.grid == nil {
		return nil
	}
	return l.grid.Needed(view)
}

// IsLoaded reports whether id is loaded. Unknown ids are false.
func (l *Large) IsLoaded(id ChunkID) bool {
	if l == nil {
		return false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, ok := l.loaded[id]
	return ok
}

// IsMissing reports whether id is a valid chunk still showing the magenta
// placeholder: known to the grid but not loaded. Unknown ids are false.
func (l *Large) IsMissing(id ChunkID) bool {
	if l == nil {
		return false
	}
	if _, ok := l.grid.ChunkBounds(id); !ok {
		return false
	}
	return !l.IsLoaded(id)
}

// Loaded returns the loaded chunk ids, sorted. A fresh slice every call.
func (l *Large) Loaded() []ChunkID {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.loaded) == 0 {
		return nil
	}
	out := make([]ChunkID, 0, len(l.loaded))
	for id := range l.loaded {
		out = append(out, id)
	}
	sortChunkIDs(out)
	return out
}

// Pending returns the queued chunk ids in queue order. A fresh slice
// every call: the window paints them amber so the budget stays visible.
func (l *Large) Pending() []ChunkID {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.pending) == 0 {
		return nil
	}
	out := make([]ChunkID, len(l.pending))
	copy(out, l.pending)
	return out
}

// Visible lists loaded chunks touching view: the draw list. Sorted, nil
// when nothing to draw.
func (l *Large) Visible(view core.Rect) []ChunkID {
	if l == nil {
		return nil
	}
	need := l.grid.Needed(view)
	if len(need) == 0 {
		return nil
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if len(l.loaded) == 0 {
		return nil
	}
	var out []ChunkID
	for _, id := range need {
		if _, ok := l.loaded[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// Request queues the chunks view needs that are neither loaded nor queued
// (background parse). It returns the newly queued chunks, sorted, nil when
// nothing new. Empty or invalid views queue nothing.
func (l *Large) Request(view core.Rect) []ChunkID {
	if l == nil || !finiteRect(view) || view.IsEmpty() {
		return nil
	}
	need := l.grid.Needed(view)
	if len(need) == 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pendingSet == nil {
		l.pendingSet = make(map[ChunkID]struct{})
	}
	var queued []ChunkID
	for _, id := range need {
		if _, ok := l.loaded[id]; ok {
			continue
		}
		if _, ok := l.pendingSet[id]; ok {
			continue
		}
		l.pendingSet[id] = struct{}{}
		l.pending = append(l.pending, id)
		queued = append(queued, id)
		l.requests++
	}
	sortChunkIDs(queued)
	return queued
}

// MergeBudget moves at most maxPerFrame queued chunks into loaded
// (foreground merge under the per-frame budget) and returns the merged
// chunks in queue order. maxPerFrame <= 0 merges nothing; an empty queue
// returns nil.
func (l *Large) MergeBudget(maxPerFrame int) []ChunkID {
	if l == nil || maxPerFrame <= 0 {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.pending) == 0 {
		return nil
	}
	n := maxPerFrame
	if n > len(l.pending) {
		n = len(l.pending)
	}
	merged := make([]ChunkID, n)
	copy(merged, l.pending[:n])
	l.pending = append([]ChunkID(nil), l.pending[n:]...)
	if len(l.pending) == 0 {
		l.pendingSet = nil
	} else {
		for _, id := range merged {
			delete(l.pendingSet, id)
		}
	}
	if l.loaded == nil {
		l.loaded = make(map[ChunkID]struct{}, len(merged))
	}
	for _, id := range merged {
		l.loaded[id] = struct{}{}
	}
	l.merges += int64(len(merged))
	return merged
}

// dropLocked removes loaded chunks outside need and returns the dropped
// ones, sorted. The caller holds the write lock.
func (l *Large) dropLocked(need []ChunkID) []ChunkID {
	if len(l.loaded) == 0 {
		return nil
	}
	keep := make(map[ChunkID]struct{}, len(need))
	for _, id := range need {
		keep[id] = struct{}{}
	}
	var gone []ChunkID
	for id := range l.loaded {
		if _, ok := keep[id]; !ok {
			gone = append(gone, id)
		}
	}
	for _, id := range gone {
		delete(l.loaded, id)
	}
	// Queued chunks outside the view stay queued: the lens may swing back
	// within a frame and re-requesting would only churn the counters.
	sortChunkIDs(gone)
	return gone
}

// Update streams one frame: queue what view needs, merge at most
// maxPerFrame, drop what view left. It returns (merged, dropped): merged
// is queue order, dropped is sorted. A jumping lens never leaves stale
// chunks and never grows: afterwards Loaded is a subset of Needed(view).
// Empty views keep everything (resize safety): unlike Chunk.Update, a
// transient zero-size view never unloads the map. Invalid views change
// nothing and return nils.
func (l *Large) Update(view core.Rect, maxPerFrame int) (merged, dropped []ChunkID) {
	if l == nil || !finiteRect(view) || view.IsEmpty() {
		return nil, nil
	}
	need := l.grid.Needed(view)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.pendingSet == nil {
		l.pendingSet = make(map[ChunkID]struct{})
	}
	for _, id := range need {
		if _, ok := l.loaded[id]; ok {
			continue
		}
		if _, ok := l.pendingSet[id]; ok {
			continue
		}
		l.pendingSet[id] = struct{}{}
		l.pending = append(l.pending, id)
		l.requests++
	}
	if maxPerFrame > 0 && len(l.pending) > 0 {
		n := maxPerFrame
		if n > len(l.pending) {
			n = len(l.pending)
		}
		merged = make([]ChunkID, n)
		copy(merged, l.pending[:n])
		l.pending = append([]ChunkID(nil), l.pending[n:]...)
		if len(l.pending) == 0 {
			l.pendingSet = nil
		} else {
			for _, id := range merged {
				delete(l.pendingSet, id)
			}
		}
		if l.loaded == nil {
			l.loaded = make(map[ChunkID]struct{}, len(merged))
		}
		for _, id := range merged {
			l.loaded[id] = struct{}{}
		}
		l.merges += int64(len(merged))
	}
	dropped = l.dropLocked(need)
	return merged, dropped
}

// Reset unloads everything and clears the queue. Nil grids stay silent.
func (l *Large) Reset() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.loaded = nil
	l.pending = nil
	l.pendingSet = nil
}
