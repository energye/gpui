//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// pendingWrite stages one Queue.WriteBuffer call. Data aliases the caller's
// buffer when the caller guarantees it is not mutated before the next drain
// (all flushes happen-before the next WriteBuffer returns: Submit/Present
// drain inline, overflow drains before stage returns), otherwise the caller
// must pass a copy. Zero-copy staging (Rust wgpu staging-belt parity —
// Queue.writeBuffer takes a view, no CPU copy): the hot upload path passes
// slab-owned bytes that are never mutated after staging, so no copy is made.
type pendingWrite struct {
	buf    *Buffer
	offset uint64
	data   []byte
	seq    uint64
}

// writeBatch coalesces buffer uploads so N WriteBuffers flush under one
// Lock with one bind per buffer and merged contiguous ranges. WriteBuffer
// only appends (no GL, no MakeCurrent); Submit/Present drain the batch.
type writeBatch struct {
	mu   sync.Mutex
	list []pendingWrite
	seq  uint64
	// bytes caps staged data; overflow flushes inline under a Lock.
	bytes uint64
}

// maxStagedBytes bounds deferred upload memory before inline flush.
const maxStagedBytes = 32 << 20

// stagedWriteOverlaps counts staged writes that cover bytes an earlier staged
// write in the SAME drain already covered. Uploaders here reuse one buffer
// region per flush and stay correct only because the queue drains once per
// command buffer: a drain that coalesces two overlapping writes uploads just
// the later bytes, so every command buffer before the last one reads the wrong
// data (measured on the merged-present regression: 46 sdf fills collapsed into
// one upload and every layer texture got the hot colour). Cumulative per
// process; a healthy run keeps it at zero — gate on that (see
// docs/RENDER_CPU_1PCT_REPLAY_PLAN.md §4). Shared across queues on purpose: the
// guard answers "did any submit window reuse an upload region", which is a
// property of the process, not of one device.
var stagedWriteOverlaps atomic.Int64

// stagedWriteOverlapLogs rate-limits the guard's diagnostic line, so a broken
// frame cannot flood stderr once it trips.
var stagedWriteOverlapLogs atomic.Int64

// stagedWriteOverlapLogCap bounds the diagnostic lines per process.
const stagedWriteOverlapLogCap = 200

// countStagedWriteOverlaps reports how many of one buffer's staged writes (in
// offset order, as flushWriteBatch sorts them) land on bytes an earlier staged
// write of the same drain already occupies. Touching ranges do not count: the
// later bytes simply upload next to the earlier ones. Mirrors the merge loop's
// segment tracking, so the count is exactly the number of uploads whose earlier
// writers would read the later bytes.
func countStagedWriteOverlaps(sorted []pendingWrite) int {
	overlaps := 0
	var curEnd uint64
	for i, w := range sorted {
		if i == 0 {
			curEnd = w.offset + uint64(len(w.data))
			continue
		}
		if w.offset <= curEnd {
			if w.offset < curEnd {
				overlaps++
			}
			if end := w.offset + uint64(len(w.data)); end > curEnd {
				curEnd = end
			}
			continue
		}
		curEnd = w.offset + uint64(len(w.data))
	}
	return overlaps
}

// noteStagedWriteOverlaps records overlaps of one buffer's drain and logs the
// first few. Callers pass the byte end of that buffer's last staged write, so
// the line shows how far the collapsed upload reaches.
func noteStagedWriteOverlaps(buf *Buffer, writes, overlaps int, lastEnd uint64) {
	stagedWriteOverlaps.Add(int64(overlaps))
	if stagedWriteOverlapLogs.Add(1) > stagedWriteOverlapLogCap {
		return
	}
	label := ""
	if buf != nil {
		label = buf.label
	}
	fmt.Fprintf(os.Stderr,
		"gles: staged write overlap buf=%d label=%s writes=%d overlaps=%d last-end=%d\n",
		bufID(buf), label, writes, overlaps, lastEnd)
}

// bufID is a nil-safe buffer id read for the diagnostic line.
func bufID(buf *Buffer) uint32 {
	if buf == nil {
		return 0
	}
	return buf.id
}

// stagedWriteOverlapCount returns the cumulative guard value.
func stagedWriteOverlapCount() int64 { return stagedWriteOverlaps.Load() }

func (b *writeBatch) stage(buf *Buffer, offset uint64, data []byte) (overflow bool) {
	// Alias, don't copy: the type contract above guarantees the caller
	// won't mutate data before the next drain consumes it.
	b.mu.Lock()
	b.list = append(b.list, pendingWrite{buf: buf, offset: offset, data: data, seq: b.seq})
	b.seq++
	b.bytes += uint64(len(data))
	overflow = b.bytes > maxStagedBytes
	b.mu.Unlock()
	return overflow
}

// drain hands the staged list to the caller for flushing under its Lock
// and keeps the backing for the next stage: the list is reset in place so
// the per-submit header array is reused instead of regrown every frame.
// The caller must finish with the returned headers before the next drain
// on the same batch (all drains here run serialized under a GL Lock).
func (b *writeBatch) drain() []pendingWrite {
	b.mu.Lock()
	list := b.list
	b.list = b.list[:0]
	b.bytes = 0
	b.mu.Unlock()
	return list
}

// flushWriteBatch uploads staged writes: one bind per buffer, contiguous
// ranges merged, later writes winning overlaps (call order preserved).
func flushWriteBatch(glCtx *gl.Context, st *glExecState, list []pendingWrite) {
	// Group by buffer, preserving first-seen order.
	order := make([]*Buffer, 0, 8)
	groups := make(map[*Buffer][]pendingWrite, 8)
	for _, w := range list {
		if w.buf == nil || w.buf.id == 0 || len(w.data) == 0 {
			continue
		}
		if _, ok := groups[w.buf]; !ok {
			order = append(order, w.buf)
		}
		groups[w.buf] = append(groups[w.buf], w)
	}
	for _, buf := range order {
		ws := groups[buf]
		if buf.id == 0 {
			continue
		}
		sort.SliceStable(ws, func(i, j int) bool {
			if ws[i].offset != ws[j].offset {
				return ws[i].offset < ws[j].offset
			}
			return ws[i].seq < ws[j].seq
		})
		// Guard: a drain that collapses overlapping writes of one buffer makes
		// every earlier writer read the later bytes. Observed, never repaired —
		// the merge below is unchanged (see stagedWriteOverlaps doc).
		if overlaps := countStagedWriteOverlaps(ws); overlaps > 0 {
			last := ws[len(ws)-1]
			noteStagedWriteOverlaps(buf, len(ws), overlaps, last.offset+uint64(len(last.data)))
		}
		if st.bindGeneric(buf.target, buf.id) {
			glCtx.BindBuffer(buf.target, buf.id)
		}
		// Sweep overlapping/contiguous ranges into single uploads.
		// Copy-on-merge: a staged slice aliases its caller, so merging must
		// never rewrite it in place — take ownership on first merge.
		cur := ws[0]
		curOwned := false
		emit := func(seg pendingWrite) {
			if len(seg.data) == 0 {
				return
			}
			glCtx.BufferSubData(buf.target, int(seg.offset), len(seg.data), unsafe.Pointer(&seg.data[0]))
		}
		for _, nxt := range ws[1:] {
			curEnd := cur.offset + uint64(len(cur.data))
			if nxt.offset <= curEnd {
				// Overlap or touching: extend, later bytes winning.
				need := nxt.offset + uint64(len(nxt.data)) - cur.offset
				if !curOwned || uint64(len(cur.data)) < need {
					ext := make([]byte, need)
					copy(ext, cur.data)
					cur.data = ext
					curOwned = true
				}
				copy(cur.data[nxt.offset-cur.offset:], nxt.data)
				continue
			}
			emit(cur)
			cur = nxt
		}
		emit(cur)
	}
}
