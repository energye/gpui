package textinput

import (
	"math/rand"
	"strings"
	"testing"
)

// TestEditorBufferMirror_M35 locks Editor<->textbuffer consistency: random
// public-API edits must keep the buffer mirror byte-identical, in both auto
// and forced-string (fallback) modes.
func TestEditorBufferMirror_M35(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		e := New()
		if fallback {
			e.bufForceString = true
		}
		e.SetSingleLine(false)
		e.SetTextSimple(strings.Repeat("a世界b\n", 20000)) // ~160KB, above threshold
		if !e.bufOn {
			t.Fatalf("fallback=%v: mirror not active on 160KB doc", fallback)
		}
		if mode := e.buf.Mode(); fallback && mode != "string" {
			t.Fatalf("fallback mirror Mode=%q want string", mode)
		}
		if !fallback && e.buf.Mode() != "tree" {
			t.Fatalf("mirror Mode=%q want tree", e.buf.Mode())
		}
		rng := rand.New(rand.NewSource(777))
		ops := []string{"add", "back", "del", "code", "paste", "move"}
		for i := 0; i < 2000; i++ {
			switch ops[rng.Intn(len(ops))] {
			case "add":
				e.AddText([]string{"x", "世", "ab", "\n"}[rng.Intn(4)])
			case "back":
				e.Backspace()
			case "del":
				e.Delete()
			case "code":
				e.AddCodePoint('界')
			case "paste":
				e.Paste("pq")
			case "move":
				e.MoveCursorToEnd()
			}
			if i%100 == 0 {
				if got := e.buf.String(); got != e.GetText() {
					t.Fatalf("fallback=%v op %d: mirror divergence", fallback, i)
				}
			}
		}
		if got := e.buf.String(); got != e.GetText() {
			t.Fatalf("fallback=%v: final mirror divergence", fallback)
		}
		snap := e.BufferSnapshot()
		before := e.GetText()
		e.AddText("TAIL")
		if !e.BufferRestore(snap) {
			t.Fatalf("fallback=%v: BufferRestore failed", fallback)
		}
		if got := e.GetText(); got != before {
			t.Fatalf("fallback=%v: restore mismatch", fallback)
		}
	}
}

// TestEditorBufferSmall_NoMirror keeps small docs on the plain-string path.
func TestEditorBufferSmall_NoMirror(t *testing.T) {
	e := New()
	e.SetTextSimple("hello")
	if e.bufOn {
		t.Fatalf("small doc must not activate buffer mirror")
	}
	e.AddText(" world")
	if e.bufOn {
		t.Fatalf("small doc must stay off mirror after edit")
	}
	if got := e.GetText(); got != "hello world" {
		t.Fatalf("small doc text=%q", got)
	}
}

func mirrorMustMatch_M35(t *testing.T, e *Editor, where string) {
	t.Helper()
	// Documented hysteresis exception: below 32KB the mirror releases
	// (same rule as the SetText path), so bufOn may drop only there.
	if !e.bufOn {
		if len(e.GetText()) < 32*1024 {
			return
		}
		t.Fatalf("%s: mirror went inactive on large doc", where)
	}
	if got := e.buf.String(); got != e.GetText() {
		t.Fatalf("%s: mirror divergence", where)
	}
}

// TestEditorBufferMirrorUndoRedo_M35 locks the Undo/Redo mirror wiring:
// every step must keep the buffer byte-identical, including astral
// characters that cross the UTF-16/byte conversion.
func TestEditorBufferMirrorUndoRedo_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("a世界b\n", 20000))
	e.AddText("hello")
	e.AddText("😀👨\u200d👩\u200d👧")
	e.AddCodePoint('😀')
	mirrorMustMatch_M35(t, e, "after edits")
	for i := 0; i < 10 && e.Undo(); i++ {
		mirrorMustMatch_M35(t, e, "after undo")
	}
	for i := 0; i < 10 && e.Redo(); i++ {
		mirrorMustMatch_M35(t, e, "after redo")
	}
	mirrorMustMatch_M35(t, e, "final")
}

// TestEditorBufferMirrorComposing_M35 locks Update/EndComposing mirror
// wiring on a large doc.
func TestEditorBufferMirrorComposing_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("preedit行\n", 20000))
	e.MoveCursorToEnd()
	e.BeginComposing()
	if !e.UpdateComposingText("你好ni", TextRange{Base: 0, Extent: 0}) {
		t.Fatalf("UpdateComposingText rejected")
	}
	mirrorMustMatch_M35(t, e, "after update composing")
	e.EndComposing()
	mirrorMustMatch_M35(t, e, "after end composing")
}

// TestEditorBufferMirrorDeleteSurrounding_M35 locks DeleteSurrounding and
// DeleteSelected mirror wiring on a large doc.
func TestEditorBufferMirrorDeleteSurrounding_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("0123456789abcdef\n", 10000))
	e.MoveCursorToEnd()
	if !e.DeleteSurrounding(-3, 3) {
		t.Fatalf("DeleteSurrounding rejected")
	}
	mirrorMustMatch_M35(t, e, "after delete surrounding")
	n := utf16Len(e.GetText())
	e.SetSelection(TextRange{Base: n - 5, Extent: n - 1})
	if !e.DeleteSelected() {
		t.Fatalf("DeleteSelected rejected")
	}
	mirrorMustMatch_M35(t, e, "after delete selected")
}

// TestEditorBufferMirrorApplyDelta_M35 locks the ApplyDelta full-rebuild
// path: a remote append on a large doc must leave an active identical mirror.
func TestEditorBufferMirrorApplyDelta_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("delta行\n", 20000))
	cur := e.GetText()
	n := utf16Len(cur)
	d := TextEditingDelta{
		OldText:    cur,
		DeltaText:  "尾巴TAIL",
		DeltaStart: n,
		DeltaEnd:   n,
		Selection:  TextRange{Base: n + 6, Extent: n + 6},
	}
	if !e.ApplyDelta(d) {
		t.Fatalf("ApplyDelta rejected")
	}
	mirrorMustMatch_M35(t, e, "after apply delta")
	if got := e.GetText(); got != cur+"尾巴TAIL" {
		t.Fatalf("apply delta text mismatch len=%d", len(got))
	}
}

// TestEditorBufferFallback_M35 locks the production degradation switch:
// SetBufferFallback must flip an active large-doc mirror between tree and
// plain-string modes without losing a byte, and stay a no-op on small docs.
func TestEditorBufferFallback_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("a世界b\n", 20000)) // ~160KB, above threshold
	if !e.bufOn || e.buf.Mode() != "tree" {
		t.Fatalf("setup: mirror not active in tree mode")
	}
	e.SetBufferFallback(true)
	if got := e.buf.Mode(); got != "string" {
		t.Fatalf("fallback on: Mode=%q want string", got)
	}
	mirrorMustMatch_M35(t, e, "fallback on")
	e.AddText("x")
	e.Backspace()
	mirrorMustMatch_M35(t, e, "edits under fallback")
	e.SetBufferFallback(false)
	if got := e.buf.Mode(); got != "tree" {
		t.Fatalf("fallback off: Mode=%q want tree", got)
	}
	mirrorMustMatch_M35(t, e, "fallback off")

	s := New()
	s.SetTextSimple("small")
	s.SetBufferFallback(true)
	if s.bufOn {
		t.Fatalf("small doc must not activate mirror via fallback switch")
	}
	if got := s.GetText(); got != "small" {
		t.Fatalf("small doc text=%q", got)
	}
}

// TestEditorBufferUndoShrinkReleasesMirror_M35 locks that incremental
// edits release the mirror under the same rule as the whole-doc paths:
// a doc shrunk below 32KB via Undo must drop the mirror (SetText parity),
// while a mid-band shrink keeps it (hysteresis).
func TestEditorBufferUndoShrinkReleasesMirror_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("z", 40*1024))
	e.SetTextSimple(strings.Repeat("y", 70*1024))
	if !e.bufOn {
		t.Fatalf("setup: 70KB must activate mirror")
	}
	if !e.Undo() {
		t.Fatalf("setup: undo rejected")
	}
	if got := e.GetText(); got != strings.Repeat("z", 40*1024) {
		t.Fatalf("undo text mismatch len=%d", len(got))
	}
	if !e.bufOn {
		t.Fatalf("40KB undo shrink must keep mirror (hysteresis)")
	}
	mirrorMustMatch_M35(t, e, "mid-band after undo")

	// Shrink the same large doc below 32KB with incremental deletes: the
	// mirror must release exactly like the SetText path does.
	d := New()
	d.SetSingleLine(false)
	d.SetTextSimple(strings.Repeat("y", 70*1024))
	if !d.bufOn {
		t.Fatalf("setup: 70KB must activate mirror")
	}
	n := utf16Len(d.GetText())
	d.SetSelection(TextRange{Base: n, Extent: n})
	for len(d.GetText()) >= 32*1024 {
		if !d.DeleteSurrounding(-512, 513) {
			break
		}
	}
	if len(d.GetText()) >= 32*1024 {
		t.Fatalf("setup: deletes did not shrink below 32KB len=%d", len(d.GetText()))
	}
	if d.bufOn {
		t.Fatalf("sub-32KB incremental shrink must release mirror, mode=%s", d.buf.Mode())
	}
	if got := d.buf; got != nil {
		t.Fatalf("released mirror must be nil")
	}
}

// TestEditorBufferHysteresis_M35 locks the 64KB activate / 32KB release
// band: mid-band shrinks must keep the mirror instead of flapping.
func TestEditorBufferHysteresis_M35(t *testing.T) {
	e := New()
	e.SetSingleLine(false)
	e.SetTextSimple(strings.Repeat("x", 40*1024))
	if e.bufOn {
		t.Fatalf("40KB must not activate mirror")
	}
	e.SetTextSimple(strings.Repeat("y", 70*1024))
	if !e.bufOn {
		t.Fatalf("70KB must activate mirror")
	}
	e.SetTextSimple(strings.Repeat("z", 40*1024))
	if !e.bufOn {
		t.Fatalf("40KB shrink must keep mirror (hysteresis)")
	}
	mirrorMustMatch_M35(t, e, "mid-band")
	e.SetTextSimple(strings.Repeat("w", 10*1024))
	if e.bufOn {
		t.Fatalf("10KB must release mirror")
	}
	if got := e.GetText(); got != strings.Repeat("w", 10*1024) {
		t.Fatalf("release text mismatch")
	}
}
