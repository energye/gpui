//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package textinput

import "github.com/energye/gpui/ui/textbuffer"

// bufferThreshold keeps the legacy mirror-active flag in sync for
// observability/tests; the buffer itself is always the storage engine.
const bufferThreshold = 64 * 1024

// updateBufOn mirrors the old mirror hysteresis (activate at 64KB, release
// below 32KB) without gating the buffer.
func (e *Editor) updateBufOn() {
	if e == nil || e.buf == nil {
		return
	}
	n := e.buf.Len()
	if !e.bufOn && n >= bufferThreshold {
		e.bufOn = true
	}
	if e.bufOn && n < bufferThreshold/2 {
		e.bufOn = false
	}
}

// ensureBuf returns the document buffer, lazily seeding it from e.text.
func (e *Editor) ensureBuf() *textbuffer.Buffer {
	if e.buf == nil {
		e.buf = textbuffer.NewFromString(e.text)
		if e.bufForceString {
			e.buf.ForceString(true)
		}
		e.updateBufOn()
	}
	return e.buf
}

// Text materialization and the lazy cache live on Editor.Text in editor.go.

// setDoc replaces the document (buffer + cache) in one step.
func (e *Editor) setDoc(s string) {
	e.buf = textbuffer.NewFromString(s)
	if e.bufForceString {
		e.buf.ForceString(true)
	}
	if !e.bufForceString && e.buf.Mode() == "tree" {
		// 大文档:整串只作为按需缓存,不再常驻第二份引用.
		e.text = ""
		e.textDirty = true
	} else {
		e.text = s
		e.textDirty = false
	}
	e.docVersion++
	e.updateBufOn()
}

// editBuffer applies [a,c) -> ins on the buffer. Tree mode keeps e.text lazy;
// plain-string mode keeps the cache in sync so direct readers and tests see
// the same value without a materialization step.
func (e *Editor) editBuffer(a, c int, ins string) {
	b := e.ensureBuf()
	if c > a {
		b.Delete(a, c)
	}
	if ins != "" {
		b.Insert(a, ins)
	}
	if c > a || ins != "" {
		e.docVersion++
	}
	if b.Mode() == "tree" && !e.bufForceString {
		e.textDirty = true
		e.updateBufOn()
		return
	}
	e.text = b.String()
	e.textDirty = false
	e.updateBufOn()
}

// SetBufferFallback pins the buffer to plain-string mode (true) or releases
// it back to automatic mode (false). Both directions pay one O(n) conversion
// on a large doc; small docs are unaffected.
func (e *Editor) SetBufferFallback(v bool) {
	if e == nil {
		return
	}
	e.bufForceString = v
	if e.buf == nil {
		return
	}
	e.buf.ForceString(v)
	if v {
		e.text = e.buf.String()
		e.textDirty = false
	} else {
		e.textDirty = true
	}
}

// BufferSnapshot captures the document in O(1) (root pointer / string header).
func (e *Editor) BufferSnapshot() textbuffer.Snapshot {
	if e == nil {
		return textbuffer.Snapshot{}
	}
	return e.ensureBuf().Snapshot()
}

// BufferRestore replaces the document with a snapshot. It records no undo
// history; selection collapses into the restored text. The snapshot restore
// itself is O(1); the selection clamp materializes once (O(n)) as before.
func (e *Editor) BufferRestore(s textbuffer.Snapshot) bool {
	if e == nil {
		return false
	}
	e.ensureBuf().Restore(s)
	e.textDirty = true
	e.docVersion++
	t := e.Text()
	e.selection.Base = clampUtf16(t, e.selection.Base)
	e.selection.Extent = clampUtf16(t, e.selection.Extent)
	e.composing = false
	e.composingRange = TextRange{}
	e.caretColValid = false
	e.changed()
	return true
}
