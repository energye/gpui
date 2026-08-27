package textinput

import (
	"testing"

	"github.com/energye/gpui/ui/input"
)

func mustSetText(t *testing.T, e *Editor, s string) {
	t.Helper()
	n := utf16Len(s)
	if !e.SetText(s, TextRange{Base: n, Extent: n}, TextRange{}, 0) {
		t.Fatalf("SetText failed")
	}
}

func TestR1_FourTuple(t *testing.T) {
	e := New()
	mustSetText(t, e, "hi")
	if e.GetText() != "hi" {
		t.Fatalf("GetText %q", e.GetText())
	}
	if e.TextRange().Extent != 2 {
		t.Fatalf("TextRange %v", e.TextRange())
	}
}

func TestR1_SetTextSentinel(t *testing.T) {
	e := New()
	e.SetText("hello", TextRange{Base: -1, Extent: -1}, TextRange{Base: -1, Extent: -1}, 0)
	if e.GetText() != "hello" || e.GetCursorOffset() != 0 {
		t.Fatalf("sentinel failed text=%q cur=%d", e.GetText(), e.GetCursorOffset())
	}
}

func TestR1_EditableRange(t *testing.T) {
	e := New()
	mustSetText(t, e, "abcd")
	e.BeginComposing()
	e.UpdateComposingText("xy", TextRange{Base: 4, Extent: 6})
	er := e.EditableRange()
	if er.Start() != 4 || er.End() != 6 {
		t.Fatalf("EditableRange %v", er)
	}
	if !e.SetSelection(TextRange{Base: 4, Extent: 4}) {
		t.Fatalf("SetSelection in composing failed")
	}
	if e.SetSelection(TextRange{Base: 0, Extent: 1}) {
		t.Fatalf("SetSelection outside composing should fail")
	}
}

func TestR1_UpdateComposingText(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.BeginComposing()
	e.UpdateComposingText("ni", TextRange{Base: 2, Extent: 4})
	if e.GetText() != "abni" || !e.ComposeActive() {
		t.Fatalf("UpdateComposingText text=%q active=%v", e.GetText(), e.ComposeActive())
	}
	e.UpdateComposingText("nihao", TextRange{Base: 2, Extent: 7})
	if e.GetText() != "abnihao" {
		t.Fatalf("second update %q", e.GetText())
	}
	e.EndComposing()
	if e.GetText() != "ab" {
		t.Fatalf("EndComposing should delete preedit, got %q", e.GetText())
	}
}

func TestR1_AddTextReplacesComposing(t *testing.T) {
	e := New()
	mustSetText(t, e, "ab")
	e.SetSelection(TextRange{Base: 2, Extent: 2})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECompose, Text: "nihao"})
	e.ApplyIME(input.IMEEvent{Kind: input.IMECommit, Text: "你好"})
	if e.GetText() != "ab你好" || e.ComposeActive() {
		t.Fatalf("commit %q active=%v", e.GetText(), e.ComposeActive())
	}
}

func TestR1_Batch(t *testing.T) {
	e := New()
	n := 0
	e.OnChange = func() { n++ }
	e.BeginBatchEdit()
	e.AddText("a")
	e.AddText("b")
	if n != 0 {
		t.Fatalf("batch should suppress, n=%d", n)
	}
	e.EndBatchEdit()
	if n != 1 {
		t.Fatalf("batch end should fire once, n=%d", n)
	}
}

func TestR1_Epoch(t *testing.T) {
	e := New()
	e0 := e.Epoch()
	e.AddText("x")
	if e.Epoch() <= e0 {
		t.Fatalf("epoch not monotonic")
	}
}

func TestR1_BackspaceSurrogate(t *testing.T) {
	e := New()
	mustSetText(t, e, "a𐐷b")
	// 𐐷 is U+10437 >0xFFFF, 2 units
	e.SetSelection(TextRange{Base: 3, Extent: 3}) // after 𐐷 (a=1, 𐐷=2, so 3)
	e.Backspace()
	if e.GetText() != "ab" {
		t.Fatalf("surrogate backspace %q", e.GetText())
	}
}

func TestR1_DeleteSurrounding(t *testing.T) {
	e := New()
	mustSetText(t, e, "abcdef")
	e.SetSelection(TextRange{Base: 3, Extent: 3})
	e.DeleteSurrounding(-1, 1)
	if e.GetText() != "abdef" {
		t.Fatalf("DeleteSurrounding %q", e.GetText())
	}
}
