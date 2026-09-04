package textinput

import "github.com/energye/gpui/ui/textbuffer"

// bufferThreshold is the document size where the Editor activates its
// textbuffer mirror. Deactivation happens below half (hysteresis).
const bufferThreshold = 64 * 1024

func (e *Editor) mirrorOn() bool { return e != nil && e.bufOn && e.buf != nil }

// mirrorRebuild syncs the mirror from e.text, activating or deactivating by
// size. SetText and cross-threshold edits come here; small docs never pay.
func (e *Editor) mirrorRebuild() {
	if e == nil {
		return
	}
	if len(e.text) >= bufferThreshold {
		b := textbuffer.NewFromString(e.text)
		if e.bufForceString {
			b.ForceString(true)
		}
		e.buf = b
		e.bufOn = true
		return
	}
	if e.bufOn && len(e.text) < bufferThreshold/2 {
		e.buf = nil
		e.bufOn = false
		return
	}
	if e.bufOn {
		b := textbuffer.NewFromString(e.text)
		if e.bufForceString {
			b.ForceString(true)
		}
		e.buf = b
	}
}

func (e *Editor) mirrorInsert(off int, s string) {
	if s == "" {
		return
	}
	if !e.mirrorOn() {
		e.mirrorRebuild()
		return
	}
	e.buf.Insert(off, s)
	e.mirrorMaybeRelease()
}

func (e *Editor) mirrorDelete(a, c int) {
	if c <= a {
		return
	}
	if !e.mirrorOn() {
		e.mirrorRebuild()
		return
	}
	e.buf.Delete(a, c)
	e.mirrorMaybeRelease()
}

// mirrorMaybeRelease drops a mirror that degraded to a small plain string,
// matching mirrorRebuild's release rule (< threshold/2). Incremental edits
// (undo/redo/deletes) shrink through the buffer's own hysteresis, which
// leaves a redundant second copy of e.text; the whole-doc paths
// (SetText/ApplyDelta/Restore) release it, so this keeps both paths equal.
func (e *Editor) mirrorMaybeRelease() {
	if e.bufOn && e.buf != nil && e.buf.Mode() != "tree" && len(e.text) < bufferThreshold/2 {
		e.buf = nil
		e.bufOn = false
	}
}

// SetBufferFallback pins the large-document mirror to plain-string mode
// (true) or releases it back to automatic mode (false). It makes the M3.5
// degradation path reachable from production: without it only tests could
// flip bufForceString. Applied to the active mirror immediately; both
// directions pay one O(n) conversion on a large doc (materialize on enable,
// tree rebuild on disable). Small docs are unaffected (mirror stays off).
func (e *Editor) SetBufferFallback(v bool) {
	if e == nil {
		return
	}
	e.bufForceString = v
	if e.bufOn && e.buf != nil {
		e.buf.ForceString(v)
	}
}

// BufferSnapshot captures the document in O(1) for large docs (string header
// or tree root copy). Small docs return an equivalent string snapshot.
func (e *Editor) BufferSnapshot() textbuffer.Snapshot {
	if e == nil {
		return textbuffer.Snapshot{}
	}
	if !e.mirrorOn() {
		e.mirrorRebuild()
	}
	if e.mirrorOn() {
		return e.buf.Snapshot()
	}
	return textbuffer.SnapshotFromString(e.text)
}

// BufferRestore replaces the document with a snapshot. It records no undo
// history; selection collapses into the restored text. Cost note: the
// Editor still owns a plain string, so restoring a tree snapshot
// materializes once (O(n)) plus a mirror rebuild — only Buffer.Snapshot and
// Buffer.Restore themselves are O(1).
func (e *Editor) BufferRestore(s textbuffer.Snapshot) bool {
	if e == nil {
		return false
	}
	t := s.String()
	e.text = t
	e.selection.Base = clampUtf16(t, e.selection.Base)
	e.selection.Extent = clampUtf16(t, e.selection.Extent)
	e.composing = false
	e.composingRange = TextRange{}
	e.mirrorRebuild()
	e.caretColValid = false
	e.changed()
	return true
}
