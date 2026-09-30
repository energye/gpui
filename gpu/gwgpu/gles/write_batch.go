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
	"sort"
	"sync"
	"unsafe"

	"github.com/energye/gpui/gpu/gwgpu/gles/gl"
)

// pendingWrite stages one Queue.WriteBuffer call.
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

func (b *writeBatch) stage(buf *Buffer, offset uint64, data []byte) (overflow bool) {
	cp := append([]byte(nil), data...)
	b.mu.Lock()
	b.list = append(b.list, pendingWrite{buf: buf, offset: offset, data: cp, seq: b.seq})
	b.seq++
	b.bytes += uint64(len(cp))
	overflow = b.bytes > maxStagedBytes
	b.mu.Unlock()
	return overflow
}

// drain swaps out the staged list for flushing under the caller's Lock.
func (b *writeBatch) drain() []pendingWrite {
	b.mu.Lock()
	list := b.list
	b.list = nil
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
		if st.bindGeneric(buf.target, buf.id) {
			glCtx.BindBuffer(buf.target, buf.id)
		}
		// Sweep overlapping/contiguous ranges into single uploads.
		cur := ws[0]
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
				if uint64(len(cur.data)) < need {
					ext := make([]byte, need)
					copy(ext, cur.data)
					cur.data = ext
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
